BEGIN;

CREATE TABLE IF NOT EXISTS platform.tenant_database_provisioning_jobs (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id       UUID NOT NULL REFERENCES public.organizations(id) ON DELETE CASCADE,
    tenant_database_id    UUID NOT NULL REFERENCES platform.tenant_database_targets(id) ON DELETE CASCADE,
    idempotency_key       TEXT NOT NULL UNIQUE,
    status                TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'running', 'retry_scheduled', 'succeeded', 'failed', 'cancelled')),
    attempt_count         INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    max_attempts          INTEGER NOT NULL DEFAULT 8 CHECK (max_attempts BETWEEN 1 AND 100),
    available_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    lease_owner           TEXT NOT NULL DEFAULT '',
    lease_expires_at      TIMESTAMPTZ,
    bootstrap_payload     JSONB NOT NULL DEFAULT '{}'
        CHECK (jsonb_typeof(bootstrap_payload) = 'object'),
    last_error            TEXT NOT NULL DEFAULT '',
    started_at            TIMESTAMPTZ,
    completed_at          TIMESTAMPTZ,
    metadata              JSONB NOT NULL DEFAULT '{}'
        CHECK (jsonb_typeof(metadata) = 'object'),
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tenant_database_provisioning_jobs_claim
    ON platform.tenant_database_provisioning_jobs(status, available_at, created_at)
    WHERE status IN ('pending', 'running', 'retry_scheduled');

CREATE INDEX IF NOT EXISTS idx_tenant_database_provisioning_jobs_organization
    ON platform.tenant_database_provisioning_jobs(organization_id, created_at DESC);

INSERT INTO platform.tenant_database_provisioning_jobs(
    organization_id,
    tenant_database_id,
    idempotency_key,
    status,
    bootstrap_payload,
    metadata
)
SELECT
    t.organization_id,
    t.id,
    'tenant-database-provision:' || t.organization_id::text || ':v1',
    'pending',
    jsonb_build_object(
        'organization_id', o.id,
        'organization_name', o.name,
        'description', COALESCE(o.description, ''),
        'owner_user_id', COALESCE(o.created_by, '00000000-0000-0000-0000-000000000000'::uuid),
        'owner_name', COALESCE(u.name, ''),
        'owner_email', COALESCE(u.email, ''),
        'enabled_modules', COALESCE((
            SELECT jsonb_agg(e.module_key ORDER BY e.module_key)
            FROM public.organization_module_entitlements e
            WHERE e.organization_id = o.id AND e.status = 'enabled'
        ), '[]'::jsonb)
    ),
    '{"source":"010_existing_unprovisioned_targets"}'::jsonb
FROM platform.tenant_database_targets t
JOIN public.organizations o ON o.id = t.organization_id
LEFT JOIN public.users u ON u.id = o.created_by
WHERE t.deployment_mode = 'dedicated_database'
  AND t.status IN ('provisioning', 'failed')
  AND o.status = 'active'
ON CONFLICT (idempotency_key) DO NOTHING;

COMMIT;
