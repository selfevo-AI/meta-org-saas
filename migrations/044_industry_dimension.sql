CREATE SCHEMA IF NOT EXISTS platform;

CREATE TABLE IF NOT EXISTS platform.industries (
    industry_key TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'active', 'archived')),
    metadata     JSONB NOT NULL DEFAULT '{}'
        CHECK (jsonb_typeof(metadata) = 'object'),
    created_by   UUID REFERENCES public.users(id) ON DELETE SET NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS platform.custom_packages (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    industry_key TEXT NOT NULL REFERENCES platform.industries(industry_key) ON DELETE CASCADE,
    package_key  TEXT NOT NULL,
    version      INT NOT NULL DEFAULT 1,
    name         TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'active', 'archived')),
    metadata     JSONB NOT NULL DEFAULT '{}'
        CHECK (jsonb_typeof(metadata) = 'object'),
    created_by   UUID REFERENCES public.users(id) ON DELETE SET NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (industry_key, package_key, version)
);

CREATE INDEX IF NOT EXISTS idx_custom_packages_industry
    ON platform.custom_packages(industry_key, status, version DESC);

CREATE TABLE IF NOT EXISTS platform.custom_package_assets (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    package_id UUID NOT NULL REFERENCES platform.custom_packages(id) ON DELETE CASCADE,
    asset_key  TEXT NOT NULL,
    asset_type TEXT NOT NULL
        CHECK (asset_type IN (
            'schema_package', 'module', 'runtime_entity', 'runtime_operation',
            'skill_structure', 'skill', 'knowledge_source', 'model_policy', 'i18n'
        )),
    payload    JSONB NOT NULL DEFAULT '{}',
    metadata   JSONB NOT NULL DEFAULT '{}'
        CHECK (jsonb_typeof(metadata) = 'object'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (package_id, asset_key)
);

CREATE INDEX IF NOT EXISTS idx_custom_package_assets_type
    ON platform.custom_package_assets(package_id, asset_type, asset_key);

CREATE TABLE IF NOT EXISTS platform.organization_industry_adoptions (
    organization_id UUID PRIMARY KEY REFERENCES public.organizations(id) ON DELETE CASCADE,
    industry_key    TEXT NOT NULL REFERENCES platform.industries(industry_key) ON DELETE RESTRICT,
    package_id      UUID NOT NULL REFERENCES platform.custom_packages(id) ON DELETE RESTRICT,
    is_primary      BOOLEAN NOT NULL DEFAULT TRUE,
    enabled_modules JSONB NOT NULL DEFAULT '[]'
        CHECK (jsonb_typeof(enabled_modules) = 'array'),
    status          TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'archived')),
    metadata        JSONB NOT NULL DEFAULT '{}'
        CHECK (jsonb_typeof(metadata) = 'object'),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_organization_industry_adoptions_industry
    ON platform.organization_industry_adoptions(industry_key, status);

CREATE TABLE IF NOT EXISTS platform.organization_industry_extensions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES public.organizations(id) ON DELETE CASCADE,
    industry_key    TEXT NOT NULL REFERENCES platform.industries(industry_key) ON DELETE RESTRICT,
    package_id      UUID REFERENCES platform.custom_packages(id) ON DELETE SET NULL,
    extension_key   TEXT NOT NULL,
    name            TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'active', 'archived')),
    assets          JSONB NOT NULL DEFAULT '[]'
        CHECK (jsonb_typeof(assets) = 'array'),
    metadata        JSONB NOT NULL DEFAULT '{}'
        CHECK (jsonb_typeof(metadata) = 'object'),
    created_by      UUID REFERENCES public.users(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, extension_key)
);

CREATE INDEX IF NOT EXISTS idx_organization_industry_extensions_lookup
    ON platform.organization_industry_extensions(organization_id, industry_key, status);

CREATE TABLE IF NOT EXISTS platform.custom_package_publication_requests (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    extension_id           UUID NOT NULL REFERENCES platform.organization_industry_extensions(id) ON DELETE CASCADE,
    source_organization_id UUID NOT NULL REFERENCES public.organizations(id) ON DELETE CASCADE,
    industry_key            TEXT NOT NULL REFERENCES platform.industries(industry_key) ON DELETE RESTRICT,
    status                  TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'approved', 'rejected')),
    reason                  TEXT NOT NULL DEFAULT '',
    review_reason           TEXT NOT NULL DEFAULT '',
    requested_by            UUID REFERENCES public.users(id) ON DELETE SET NULL,
    reviewed_by             UUID REFERENCES public.users(id) ON DELETE SET NULL,
    reviewed_at             TIMESTAMPTZ,
    metadata                JSONB NOT NULL DEFAULT '{}'
        CHECK (jsonb_typeof(metadata) = 'object'),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_custom_package_publication_requests_status
    ON platform.custom_package_publication_requests(status, created_at DESC);

CREATE TABLE IF NOT EXISTS platform.knowledge_sources (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    industry_key      TEXT NOT NULL REFERENCES platform.industries(industry_key) ON DELETE CASCADE,
    organization_id   UUID REFERENCES public.organizations(id) ON DELETE CASCADE,
    source_key        TEXT NOT NULL,
    name              TEXT NOT NULL,
    source_type       TEXT NOT NULL DEFAULT 'external_reference',
    adapter_key       TEXT NOT NULL DEFAULT '',
    reference_uri     TEXT NOT NULL DEFAULT '',
    sync_status       TEXT NOT NULL DEFAULT 'not_configured'
        CHECK (sync_status IN ('not_configured', 'ready', 'syncing', 'error', 'disabled')),
    permission        JSONB NOT NULL DEFAULT '{}'
        CHECK (jsonb_typeof(permission) = 'object'),
    retrieval_config  JSONB NOT NULL DEFAULT '{}'
        CHECK (jsonb_typeof(retrieval_config) = 'object'),
    metadata          JSONB NOT NULL DEFAULT '{}'
        CHECK (jsonb_typeof(metadata) = 'object'),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (industry_key, organization_id, source_key)
);

CREATE INDEX IF NOT EXISTS idx_knowledge_sources_scope
    ON platform.knowledge_sources(industry_key, organization_id, sync_status);

INSERT INTO platform.industries(industry_key, name, description, status, metadata)
VALUES
    ('general', 'General Business', 'Default cross-industry SaaS management baseline', 'active', '{"seed":true}'::jsonb),
    ('manufacturing', 'Manufacturing', 'Manufacturing and supply-chain operating baseline', 'active', '{"seed":true}'::jsonb)
ON CONFLICT (industry_key) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    status = EXCLUDED.status,
    metadata = platform.industries.metadata || EXCLUDED.metadata,
    updated_at = NOW();

WITH foundation_package AS (
    INSERT INTO platform.custom_packages(industry_key, package_key, version, name, description, status, metadata)
    VALUES (
        'general',
        'general-foundation',
        1,
        'General Foundation',
        'Default package for organization, project, workflow, governance, assistant, finance, and runtime foundations',
        'active',
        '{"seed":true}'::jsonb
    )
    ON CONFLICT (industry_key, package_key, version) DO UPDATE SET
        name = EXCLUDED.name,
        description = EXCLUDED.description,
        status = EXCLUDED.status,
        metadata = platform.custom_packages.metadata || EXCLUDED.metadata,
        updated_at = NOW()
    RETURNING id
)
INSERT INTO platform.custom_package_assets(package_id, asset_key, asset_type, payload, metadata)
SELECT id, asset_key, 'module', jsonb_build_object('module_key', module_key, 'display_name', display_name), '{"seed":true}'::jsonb
FROM foundation_package
CROSS JOIN (VALUES
    ('organization', 'Organization'),
    ('project', 'Project Operations'),
    ('workflow', 'Workflow'),
    ('governance', 'Governance'),
    ('evolution', 'Evolution'),
    ('capability', 'Capability'),
    ('meta_resource', 'Meta Resource'),
    ('assistant', 'AI Assistant'),
    ('ai_gateway', 'AI Gateway'),
    ('toolruntime', 'Tool Runtime'),
    ('finance', 'Finance'),
    ('costing', 'Costing'),
    ('developer_tools', 'Developer Tools')
) AS modules(module_key, display_name)
CROSS JOIN LATERAL (SELECT module_key || '-module' AS asset_key) asset_keys
ON CONFLICT (package_id, asset_key) DO UPDATE SET
    payload = EXCLUDED.payload,
    metadata = platform.custom_package_assets.metadata || EXCLUDED.metadata,
    updated_at = NOW();

WITH manufacturing_package AS (
    INSERT INTO platform.custom_packages(industry_key, package_key, version, name, description, status, metadata)
    VALUES (
        'manufacturing',
        'manufacturing-supply-chain',
        1,
        'Manufacturing Supply Chain',
        'Manufacturing package with inventory, procurement, sales, finance, costing, assistant, and runtime capabilities',
        'active',
        '{"seed":true}'::jsonb
    )
    ON CONFLICT (industry_key, package_key, version) DO UPDATE SET
        name = EXCLUDED.name,
        description = EXCLUDED.description,
        status = EXCLUDED.status,
        metadata = platform.custom_packages.metadata || EXCLUDED.metadata,
        updated_at = NOW()
    RETURNING id
)
INSERT INTO platform.custom_package_assets(package_id, asset_key, asset_type, payload, metadata)
SELECT id, asset_key, 'module', jsonb_build_object('module_key', module_key, 'display_name', display_name), '{"seed":true}'::jsonb
FROM manufacturing_package
CROSS JOIN (VALUES
    ('organization', 'Organization'),
    ('inventory', 'Inventory'),
    ('procurement', 'Procurement'),
    ('sales', 'Sales'),
    ('finance', 'Finance'),
    ('costing', 'Costing'),
    ('assistant', 'AI Assistant'),
    ('ai_gateway', 'AI Gateway'),
    ('toolruntime', 'Tool Runtime')
) AS modules(module_key, display_name)
CROSS JOIN LATERAL (SELECT module_key || '-module' AS asset_key) asset_keys
ON CONFLICT (package_id, asset_key) DO UPDATE SET
    payload = EXCLUDED.payload,
    metadata = platform.custom_package_assets.metadata || EXCLUDED.metadata,
    updated_at = NOW();
