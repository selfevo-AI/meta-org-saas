-- 041_system_admin_master_detail.sql
-- SaaS system administration foundation.
--
-- This migration is intentionally non-destructive. Existing platform tables stay
-- in public for compatibility while platform-managed metadata is mirrored into
-- module master/detail records under the platform schema.

CREATE SCHEMA IF NOT EXISTS platform;

CREATE TABLE IF NOT EXISTS platform.platform_masters (
    master_key      TEXT PRIMARY KEY DEFAULT ('PLT-' || UPPER(SUBSTR(REPLACE(gen_random_uuid()::TEXT, '-', ''), 1, 24))),
    module_key      TEXT NOT NULL,
    entity_type     TEXT NOT NULL,
    source_table    TEXT NOT NULL DEFAULT '',
    source_pk       TEXT NOT NULL DEFAULT '',
    title           TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'active',
    organization_id UUID REFERENCES public.organizations(id) ON DELETE SET NULL,
    payload         JSONB NOT NULL DEFAULT '{}',
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_platform_masters_source
    ON platform.platform_masters(source_table, source_pk)
    WHERE source_table <> '' AND source_pk <> '';

CREATE INDEX IF NOT EXISTS idx_platform_masters_module
    ON platform.platform_masters(module_key, entity_type, status, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_platform_masters_org
    ON platform.platform_masters(organization_id, module_key, entity_type);

CREATE TABLE IF NOT EXISTS platform.platform_details (
    detail_key  TEXT PRIMARY KEY DEFAULT ('PLTD-' || UPPER(SUBSTR(REPLACE(gen_random_uuid()::TEXT, '-', ''), 1, 24))),
    master_key  TEXT NOT NULL REFERENCES platform.platform_masters(master_key) ON DELETE CASCADE,
    detail_type TEXT NOT NULL,
    field_key   TEXT NOT NULL DEFAULT '',
    line_no     INT NOT NULL DEFAULT 0,
    payload     JSONB NOT NULL DEFAULT '{}',
    metadata    JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_platform_details_master
    ON platform.platform_details(master_key, detail_type, line_no);

CREATE UNIQUE INDEX IF NOT EXISTS uq_platform_details_natural
    ON platform.platform_details(master_key, detail_type, field_key, line_no);

CREATE TABLE IF NOT EXISTS platform.schema_template_masters (
    master_key     TEXT PRIMARY KEY DEFAULT ('STM-' || UPPER(SUBSTR(REPLACE(gen_random_uuid()::TEXT, '-', ''), 1, 24))),
    template_key   TEXT NOT NULL,
    module_key     TEXT NOT NULL,
    template_scope TEXT NOT NULL DEFAULT 'platform'
        CHECK (template_scope IN ('platform', 'organization', 'deployment')),
    version_key    TEXT NOT NULL DEFAULT 'meta-org.schema.v1',
    status         TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('draft', 'active', 'archived')),
    title          TEXT NOT NULL DEFAULT '',
    package_json   JSONB NOT NULL DEFAULT '{}',
    metadata       JSONB NOT NULL DEFAULT '{}',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_schema_template_masters_key
    ON platform.schema_template_masters(template_key, version_key);

CREATE TABLE IF NOT EXISTS platform.schema_template_details (
    detail_key  TEXT PRIMARY KEY DEFAULT ('STD-' || UPPER(SUBSTR(REPLACE(gen_random_uuid()::TEXT, '-', ''), 1, 24))),
    master_key  TEXT NOT NULL REFERENCES platform.schema_template_masters(master_key) ON DELETE CASCADE,
    detail_type TEXT NOT NULL,
    field_key   TEXT NOT NULL DEFAULT '',
    line_no     INT NOT NULL DEFAULT 0,
    payload     JSONB NOT NULL DEFAULT '{}',
    metadata    JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_schema_template_details_master
    ON platform.schema_template_details(master_key, detail_type, line_no);

CREATE UNIQUE INDEX IF NOT EXISTS uq_schema_template_details_natural
    ON platform.schema_template_details(master_key, detail_type, field_key, line_no);

CREATE TABLE IF NOT EXISTS platform.organization_schema_targets (
    organization_id        UUID PRIMARY KEY REFERENCES public.organizations(id) ON DELETE CASCADE,
    schema_name            TEXT NOT NULL UNIQUE,
    template_version       TEXT NOT NULL DEFAULT 'meta-org.schema.v1',
    status                 TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'provisioned', 'applying', 'error', 'archived')),
    last_change_request_id UUID,
    metadata               JSONB NOT NULL DEFAULT '{}',
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS platform.schema_change_requests (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES public.organizations(id) ON DELETE CASCADE,
    schema_name     TEXT NOT NULL,
    request_type    TEXT NOT NULL DEFAULT 'import_schema_package',
    status          TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'approved', 'rejected', 'applied', 'failed', 'cancelled')),
    reason          TEXT NOT NULL DEFAULT '',
    schema_package  JSONB NOT NULL DEFAULT '{}',
    statements      JSONB NOT NULL DEFAULT '[]',
    requested_by    UUID REFERENCES public.users(id) ON DELETE SET NULL,
    reviewed_by     UUID REFERENCES public.users(id) ON DELETE SET NULL,
    applied_by      UUID REFERENCES public.users(id) ON DELETE SET NULL,
    review_reason   TEXT NOT NULL DEFAULT '',
    reviewed_at     TIMESTAMPTZ,
    applied_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (jsonb_typeof(schema_package) = 'object'),
    CHECK (jsonb_typeof(statements) = 'array')
);

CREATE INDEX IF NOT EXISTS idx_schema_change_requests_org
    ON platform.schema_change_requests(organization_id, status, created_at DESC);

CREATE TABLE IF NOT EXISTS platform.schema_apply_jobs (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    change_request_id UUID NOT NULL REFERENCES platform.schema_change_requests(id) ON DELETE CASCADE,
    organization_id   UUID NOT NULL REFERENCES public.organizations(id) ON DELETE CASCADE,
    schema_name       TEXT NOT NULL,
    status            TEXT NOT NULL DEFAULT 'running'
        CHECK (status IN ('running', 'applied', 'failed')),
    statements        JSONB NOT NULL DEFAULT '[]',
    error_message     TEXT NOT NULL DEFAULT '',
    metadata          JSONB NOT NULL DEFAULT '{}',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (jsonb_typeof(statements) = 'array')
);

CREATE INDEX IF NOT EXISTS idx_schema_apply_jobs_request
    ON platform.schema_apply_jobs(change_request_id, status, created_at DESC);

CREATE OR REPLACE FUNCTION platform.organization_schema_name(p_organization_id UUID)
RETURNS TEXT
LANGUAGE sql
IMMUTABLE
AS $$
    SELECT 'org_' || REPLACE(p_organization_id::TEXT, '-', '');
$$;

CREATE OR REPLACE FUNCTION platform.upsert_platform_master(
    p_module_key TEXT,
    p_entity_type TEXT,
    p_source_table TEXT,
    p_source_pk TEXT,
    p_title TEXT,
    p_status TEXT,
    p_organization_id UUID,
    p_payload JSONB,
    p_metadata JSONB
)
RETURNS TEXT
LANGUAGE plpgsql
AS $$
DECLARE
    v_master_key TEXT;
BEGIN
    INSERT INTO platform.platform_masters(
        module_key, entity_type, source_table, source_pk, title, status,
        organization_id, payload, metadata
    )
    VALUES (
        p_module_key, p_entity_type, p_source_table, p_source_pk,
        COALESCE(p_title, ''), COALESCE(NULLIF(p_status, ''), 'active'),
        p_organization_id, COALESCE(p_payload, '{}'::JSONB), COALESCE(p_metadata, '{}'::JSONB)
    )
    ON CONFLICT (source_table, source_pk) WHERE source_table <> '' AND source_pk <> ''
    DO UPDATE SET
        module_key = EXCLUDED.module_key,
        entity_type = EXCLUDED.entity_type,
        title = EXCLUDED.title,
        status = EXCLUDED.status,
        organization_id = EXCLUDED.organization_id,
        payload = EXCLUDED.payload,
        metadata = platform.platform_masters.metadata || EXCLUDED.metadata,
        updated_at = NOW()
    RETURNING master_key INTO v_master_key;

    RETURN v_master_key;
END;
$$;

CREATE OR REPLACE FUNCTION platform.ensure_organization_schema(p_organization_id UUID)
RETURNS TEXT
LANGUAGE plpgsql
AS $$
DECLARE
    v_schema_name TEXT := platform.organization_schema_name(p_organization_id);
BEGIN
    EXECUTE FORMAT('CREATE SCHEMA IF NOT EXISTS %I', v_schema_name);

    EXECUTE FORMAT(
        'CREATE TABLE IF NOT EXISTS %I.%I (
            master_key TEXT PRIMARY KEY DEFAULT (''ORGM-'' || UPPER(SUBSTR(REPLACE(gen_random_uuid()::TEXT, ''-'', ''''), 1, 24))),
            entity_type TEXT NOT NULL,
            legacy_table TEXT NOT NULL DEFAULT '''',
            legacy_pk TEXT NOT NULL DEFAULT '''',
            legacy_id UUID,
            title TEXT NOT NULL DEFAULT '''',
            status TEXT NOT NULL DEFAULT ''active'',
            parent_master_key TEXT NOT NULL DEFAULT '''',
            core_data JSONB NOT NULL DEFAULT ''{}''::JSONB,
            metadata JSONB NOT NULL DEFAULT ''{}''::JSONB,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        )',
        v_schema_name,
        'organization_masters'
    );

    EXECUTE FORMAT(
        'CREATE TABLE IF NOT EXISTS %I.%I (
            detail_key TEXT PRIMARY KEY DEFAULT (''ORGD-'' || UPPER(SUBSTR(REPLACE(gen_random_uuid()::TEXT, ''-'', ''''), 1, 24))),
            master_key TEXT NOT NULL REFERENCES %I.%I(master_key) ON DELETE CASCADE,
            detail_type TEXT NOT NULL,
            field_key TEXT NOT NULL DEFAULT '''',
            line_no INT NOT NULL DEFAULT 0,
            payload JSONB NOT NULL DEFAULT ''{}''::JSONB,
            metadata JSONB NOT NULL DEFAULT ''{}''::JSONB,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        )',
        v_schema_name,
        'organization_details',
        v_schema_name,
        'organization_masters'
    );

    EXECUTE FORMAT('CREATE INDEX IF NOT EXISTS %I ON %I.%I(entity_type, status)', 'idx_' || v_schema_name || '_org_masters_entity', v_schema_name, 'organization_masters');
    EXECUTE FORMAT('CREATE INDEX IF NOT EXISTS %I ON %I.%I(master_key, detail_type, line_no)', 'idx_' || v_schema_name || '_org_details_master', v_schema_name, 'organization_details');

    INSERT INTO platform.organization_schema_targets(
        organization_id, schema_name, template_version, status, metadata
    )
    VALUES (
        p_organization_id, v_schema_name, 'meta-org.schema.v1', 'provisioned',
        '{"source":"migration_041_system_admin_master_detail"}'::JSONB
    )
    ON CONFLICT (organization_id) DO UPDATE SET
        schema_name = EXCLUDED.schema_name,
        status = CASE
            WHEN platform.organization_schema_targets.status = 'archived' THEN 'archived'
            ELSE 'provisioned'
        END,
        metadata = platform.organization_schema_targets.metadata || EXCLUDED.metadata,
        updated_at = NOW();

    RETURN v_schema_name;
END;
$$;

INSERT INTO platform.organization_schema_targets(organization_id, schema_name, template_version, status, metadata)
SELECT id, platform.organization_schema_name(id), 'meta-org.schema.v1', 'pending', '{"source":"existing_organizations"}'::JSONB
FROM public.organizations
ON CONFLICT (organization_id) DO NOTHING;

DO $$
DECLARE
    rec RECORD;
BEGIN
    FOR rec IN SELECT id FROM public.organizations ORDER BY created_at LOOP
        PERFORM platform.ensure_organization_schema(rec.id);
    END LOOP;
END;
$$;

INSERT INTO platform.schema_template_masters(template_key, module_key, template_scope, version_key, status, title, package_json, metadata)
VALUES (
    'organization.default',
    'organization',
    'platform',
    'meta-org.schema.v1',
    'active',
    'Default organization master/detail schema',
    '{
        "format_version": "meta-org.schema.v1",
        "module_key": "organization",
        "tables": [
            {"name":"organization_masters","fields":[
                {"name":"master_key","data_type":"text","primary_key":true},
                {"name":"entity_type","data_type":"text","nullable":false},
                {"name":"legacy_table","data_type":"text","nullable":false,"default":"''''"},
                {"name":"legacy_pk","data_type":"text","nullable":false,"default":"''''"},
                {"name":"legacy_id","data_type":"uuid","nullable":true},
                {"name":"title","data_type":"text","nullable":false,"default":"''''"},
                {"name":"status","data_type":"text","nullable":false,"default":"''active''"},
                {"name":"parent_master_key","data_type":"text","nullable":false,"default":"''''"},
                {"name":"core_data","data_type":"jsonb","nullable":false,"default":"''{}''::jsonb"},
                {"name":"metadata","data_type":"jsonb","nullable":false,"default":"''{}''::jsonb"},
                {"name":"created_at","data_type":"timestamptz","nullable":false,"default":"now()"},
                {"name":"updated_at","data_type":"timestamptz","nullable":false,"default":"now()"}
            ]},
            {"name":"organization_details","fields":[
                {"name":"detail_key","data_type":"text","primary_key":true},
                {"name":"master_key","data_type":"text","nullable":false},
                {"name":"detail_type","data_type":"text","nullable":false},
                {"name":"field_key","data_type":"text","nullable":false,"default":"''''"},
                {"name":"line_no","data_type":"integer","nullable":false,"default":"0"},
                {"name":"payload","data_type":"jsonb","nullable":false,"default":"''{}''::jsonb"},
                {"name":"metadata","data_type":"jsonb","nullable":false,"default":"''{}''::jsonb"},
                {"name":"created_at","data_type":"timestamptz","nullable":false,"default":"now()"},
                {"name":"updated_at","data_type":"timestamptz","nullable":false,"default":"now()"}
            ]}
        ]
    }'::JSONB,
    '{"source":"migration_041_system_admin_master_detail"}'::JSONB
)
ON CONFLICT (template_key, version_key) DO UPDATE SET
    package_json = EXCLUDED.package_json,
    status = EXCLUDED.status,
    metadata = platform.schema_template_masters.metadata || EXCLUDED.metadata,
    updated_at = NOW();

WITH template_master AS (
    SELECT master_key
    FROM platform.schema_template_masters
    WHERE template_key = 'organization.default'
      AND version_key = 'meta-org.schema.v1'
)
INSERT INTO platform.schema_template_details(master_key, detail_type, field_key, line_no, payload, metadata)
SELECT
    template_master.master_key,
    'source_table',
    c.source_table,
    ROW_NUMBER() OVER (ORDER BY c.module_name, c.source_table),
    to_jsonb(c),
    '{"source":"module_master_source_catalog"}'::JSONB
FROM template_master
JOIN public.module_master_source_catalog c ON c.module_name = 'organization';

INSERT INTO platform.platform_masters(module_key, entity_type, source_table, source_pk, title, status, organization_id, payload, metadata)
SELECT 'saas', 'module', 'saas_modules', module_key, display_name, CASE WHEN enabled_default THEN 'active' ELSE 'disabled' END, NULL, to_jsonb(m), '{"migrated_to_master_detail":true}'::JSONB
FROM public.saas_modules m
ON CONFLICT (source_table, source_pk) WHERE source_table <> '' AND source_pk <> ''
DO UPDATE SET payload = EXCLUDED.payload, title = EXCLUDED.title, status = EXCLUDED.status, updated_at = NOW();

INSERT INTO platform.platform_masters(module_key, entity_type, source_table, source_pk, title, status, organization_id, payload, metadata)
SELECT 'saas', 'plan', 'saas_plans', id::TEXT, code, status, NULL, to_jsonb(p), '{"migrated_to_master_detail":true}'::JSONB
FROM public.saas_plans p
ON CONFLICT (source_table, source_pk) WHERE source_table <> '' AND source_pk <> ''
DO UPDATE SET payload = EXCLUDED.payload, title = EXCLUDED.title, status = EXCLUDED.status, updated_at = NOW();

INSERT INTO platform.platform_masters(module_key, entity_type, source_table, source_pk, title, status, organization_id, payload, metadata)
SELECT 'saas', 'platform_admin', 'platform_admins', user_id::TEXT, user_id::TEXT, 'active', NULL, to_jsonb(pa), '{"migrated_to_master_detail":true}'::JSONB
FROM public.platform_admins pa
ON CONFLICT (source_table, source_pk) WHERE source_table <> '' AND source_pk <> ''
DO UPDATE SET payload = EXCLUDED.payload, title = EXCLUDED.title, status = EXCLUDED.status, updated_at = NOW();

INSERT INTO platform.platform_masters(module_key, entity_type, source_table, source_pk, title, status, organization_id, payload, metadata)
SELECT 'saas', 'organization_subscription', 'organization_subscriptions', id::TEXT, organization_id::TEXT, status, organization_id, to_jsonb(s), '{"migrated_to_master_detail":true}'::JSONB
FROM public.organization_subscriptions s
ON CONFLICT (source_table, source_pk) WHERE source_table <> '' AND source_pk <> ''
DO UPDATE SET payload = EXCLUDED.payload, title = EXCLUDED.title, status = EXCLUDED.status, organization_id = EXCLUDED.organization_id, updated_at = NOW();

INSERT INTO platform.platform_masters(module_key, entity_type, source_table, source_pk, title, status, organization_id, payload, metadata)
SELECT 'saas', 'organization_module_entitlement', 'organization_module_entitlements', organization_id::TEXT || ':' || module_key, module_key, status, organization_id, to_jsonb(e), '{"migrated_to_master_detail":true}'::JSONB
FROM public.organization_module_entitlements e
ON CONFLICT (source_table, source_pk) WHERE source_table <> '' AND source_pk <> ''
DO UPDATE SET payload = EXCLUDED.payload, title = EXCLUDED.title, status = EXCLUDED.status, organization_id = EXCLUDED.organization_id, updated_at = NOW();

INSERT INTO platform.platform_masters(module_key, entity_type, source_table, source_pk, title, status, organization_id, payload, metadata)
SELECT 'data_catalog', 'data_table', 'data_table_catalog', table_name, table_name, 'active', NULL, to_jsonb(t), '{"migrated_to_master_detail":true}'::JSONB
FROM public.data_table_catalog t
ON CONFLICT (source_table, source_pk) WHERE source_table <> '' AND source_pk <> ''
DO UPDATE SET payload = EXCLUDED.payload, title = EXCLUDED.title, updated_at = NOW();

INSERT INTO platform.platform_masters(module_key, entity_type, source_table, source_pk, title, status, organization_id, payload, metadata)
SELECT 'data_catalog', 'data_field', 'data_field_catalog', table_name || '.' || field_name, table_name || '.' || field_name, 'active', NULL, to_jsonb(f), '{"migrated_to_master_detail":true}'::JSONB
FROM public.data_field_catalog f
ON CONFLICT (source_table, source_pk) WHERE source_table <> '' AND source_pk <> ''
DO UPDATE SET payload = EXCLUDED.payload, title = EXCLUDED.title, updated_at = NOW();

INSERT INTO platform.platform_masters(module_key, entity_type, source_table, source_pk, title, status, organization_id, payload, metadata)
SELECT 'security', 'security_policy', 'security_policies', id::TEXT, policy_key, status, organization_id, to_jsonb(p), '{"migrated_to_master_detail":true}'::JSONB
FROM public.security_policies p
ON CONFLICT (source_table, source_pk) WHERE source_table <> '' AND source_pk <> ''
DO UPDATE SET payload = EXCLUDED.payload, title = EXCLUDED.title, status = EXCLUDED.status, organization_id = EXCLUDED.organization_id, updated_at = NOW();

INSERT INTO platform.platform_masters(module_key, entity_type, source_table, source_pk, title, status, organization_id, payload, metadata)
SELECT 'security', 'field_permission_rule', 'field_permission_rules', id::TEXT, table_name || '.' || field_name || ':' || action, status, organization_id, to_jsonb(r), '{"migrated_to_master_detail":true}'::JSONB
FROM public.field_permission_rules r
ON CONFLICT (source_table, source_pk) WHERE source_table <> '' AND source_pk <> ''
DO UPDATE SET payload = EXCLUDED.payload, title = EXCLUDED.title, status = EXCLUDED.status, organization_id = EXCLUDED.organization_id, updated_at = NOW();

INSERT INTO platform.platform_masters(module_key, entity_type, source_table, source_pk, title, status, organization_id, payload, metadata)
SELECT 'assistant', 'skill', 'skill', id::TEXT, name, status, organization_id, to_jsonb(s), '{"migrated_to_master_detail":true}'::JSONB
FROM public.skill s
ON CONFLICT (source_table, source_pk) WHERE source_table <> '' AND source_pk <> ''
DO UPDATE SET payload = EXCLUDED.payload, title = EXCLUDED.title, status = EXCLUDED.status, organization_id = EXCLUDED.organization_id, updated_at = NOW();

INSERT INTO platform.platform_masters(module_key, entity_type, source_table, source_pk, title, status, organization_id, payload, metadata)
SELECT 'assistant', 'skill_publication_request', 'skill_publication_requests', id::TEXT, source_skill_id::TEXT, status, source_organization_id, to_jsonb(r), '{"migrated_to_master_detail":true}'::JSONB
FROM public.skill_publication_requests r
ON CONFLICT (source_table, source_pk) WHERE source_table <> '' AND source_pk <> ''
DO UPDATE SET payload = EXCLUDED.payload, title = EXCLUDED.title, status = EXCLUDED.status, organization_id = EXCLUDED.organization_id, updated_at = NOW();

INSERT INTO platform.platform_details(master_key, detail_type, field_key, line_no, payload, metadata)
SELECT
    pm.master_key,
    'field',
    f.field_name,
    f.display_order,
    to_jsonb(f),
    '{"source":"data_field_catalog"}'::JSONB
FROM platform.platform_masters pm
JOIN public.data_field_catalog f ON pm.source_table = 'data_table_catalog' AND pm.source_pk = f.table_name
ON CONFLICT DO NOTHING;

INSERT INTO platform.platform_details(master_key, detail_type, field_key, line_no, payload, metadata)
SELECT
    pm.master_key,
    'skill_component',
    component->>'key',
    ordinality::INT,
    component,
    '{"source":"skill.skill_components"}'::JSONB
FROM platform.platform_masters pm
JOIN public.skill s ON pm.source_table = 'skill' AND pm.source_pk = s.id::TEXT
CROSS JOIN LATERAL jsonb_array_elements(s.skill_components) WITH ORDINALITY AS components(component, ordinality)
ON CONFLICT DO NOTHING;
