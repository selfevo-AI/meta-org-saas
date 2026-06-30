CREATE SCHEMA IF NOT EXISTS platform;

CREATE TABLE IF NOT EXISTS platform.organization_industry_solution_targets (
    organization_id        UUID PRIMARY KEY REFERENCES public.organizations(id) ON DELETE CASCADE,
    target_schema_name     TEXT NOT NULL UNIQUE,
    template_version       TEXT NOT NULL DEFAULT 'meta-org.industry-solution-manifest.v1',
    status                 TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'provisioned', 'applying', 'error', 'archived')),
    last_change_request_id UUID,
    metadata               JSONB NOT NULL DEFAULT '{}',
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO platform.organization_industry_solution_targets(organization_id, target_schema_name, template_version, status, metadata)
SELECT id, platform.organization_schema_name(id), 'meta-org.industry-solution-manifest.v1', 'pending', '{"source":"007_saas_runtime_organization_target_repair"}'::jsonb
FROM public.organizations
ON CONFLICT (organization_id) DO NOTHING;

CREATE OR REPLACE FUNCTION platform.provision_runtime_organization(p_organization_id UUID)
RETURNS VOID
LANGUAGE plpgsql
AS $$
DECLARE
    v_schema_name TEXT := platform.organization_schema_name(p_organization_id);
    rec RECORD;
BEGIN
    EXECUTE FORMAT('CREATE SCHEMA IF NOT EXISTS %I', v_schema_name);

    FOR rec IN
        SELECT DISTINCT module_key, storage_table
        FROM platform.runtime_entities
        WHERE status = 'active'
        ORDER BY module_key, storage_table
    LOOP
        EXECUTE FORMAT(
            'CREATE TABLE IF NOT EXISTS %I.%I (
                master_key TEXT PRIMARY KEY,
                entity_type TEXT NOT NULL,
                title TEXT NOT NULL DEFAULT '''',
                status TEXT NOT NULL DEFAULT ''active'',
                parent_master_key TEXT NOT NULL DEFAULT '''',
                core_data JSONB NOT NULL DEFAULT ''{}''::jsonb,
                metadata JSONB NOT NULL DEFAULT ''{}''::jsonb,
                created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
            )',
            v_schema_name,
            rec.storage_table
        );
        EXECUTE FORMAT('CREATE INDEX IF NOT EXISTS %I ON %I.%I(entity_type, status, updated_at DESC)',
            'idx_' || rec.storage_table || '_entity_status',
            v_schema_name,
            rec.storage_table
        );
    END LOOP;

    INSERT INTO platform.organization_industry_solution_targets(
        organization_id, target_schema_name, template_version, status, metadata
    )
    VALUES (
        p_organization_id,
        v_schema_name,
        'meta-org.runtime.v1',
        'provisioned',
        '{"source":"runtime_kernel"}'::jsonb
    )
    ON CONFLICT (organization_id) DO UPDATE SET
        target_schema_name = EXCLUDED.target_schema_name,
        template_version = EXCLUDED.template_version,
        status = 'provisioned',
        metadata = platform.organization_industry_solution_targets.metadata || EXCLUDED.metadata,
        updated_at = NOW();
END;
$$;
