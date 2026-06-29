-- Consolidate the former Schema target/package workflow into industry solutions.

CREATE SCHEMA IF NOT EXISTS platform;

CREATE TABLE IF NOT EXISTS platform.platform_migration_runs (
    filename   VARCHAR(255) PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

DO $$
BEGIN
    IF to_regclass('public.schema_migrations') IS NOT NULL THEN
        INSERT INTO platform.platform_migration_runs(filename, applied_at)
        SELECT filename, applied_at FROM public.schema_migrations
        ON CONFLICT (filename) DO NOTHING;
        DROP TABLE public.schema_migrations;
    END IF;
END;
$$;

DO $$
BEGIN
    IF to_regclass('platform.schema_template_masters') IS NOT NULL
       AND to_regclass('platform.industry_solution_template_masters') IS NULL THEN
        ALTER TABLE platform.schema_template_masters RENAME TO industry_solution_template_masters;
    END IF;

    IF to_regclass('platform.schema_template_details') IS NOT NULL
       AND to_regclass('platform.industry_solution_template_details') IS NULL THEN
        ALTER TABLE platform.schema_template_details RENAME TO industry_solution_template_details;
    END IF;

    IF to_regclass('platform.organization_schema_targets') IS NOT NULL
       AND to_regclass('platform.organization_industry_solution_targets') IS NULL THEN
        ALTER TABLE platform.organization_schema_targets RENAME TO organization_industry_solution_targets;
    END IF;

    IF to_regclass('platform.schema_change_requests') IS NOT NULL
       AND to_regclass('platform.industry_solution_change_requests') IS NULL THEN
        ALTER TABLE platform.schema_change_requests RENAME TO industry_solution_change_requests;
    END IF;

    IF to_regclass('platform.schema_apply_jobs') IS NOT NULL
       AND to_regclass('platform.industry_solution_apply_jobs') IS NULL THEN
        ALTER TABLE platform.schema_apply_jobs RENAME TO industry_solution_apply_jobs;
    END IF;

    IF to_regclass('platform.industries') IS NOT NULL
       AND to_regclass('platform.industry_solution_categories') IS NULL THEN
        ALTER TABLE platform.industries RENAME TO industry_solution_categories;
    END IF;

    IF to_regclass('platform.custom_packages') IS NOT NULL
       AND to_regclass('platform.industry_solutions') IS NULL THEN
        ALTER TABLE platform.custom_packages RENAME TO industry_solutions;
    END IF;

    IF to_regclass('platform.custom_package_assets') IS NOT NULL
       AND to_regclass('platform.industry_solution_assets') IS NULL THEN
        ALTER TABLE platform.custom_package_assets RENAME TO industry_solution_assets;
    END IF;

    IF to_regclass('platform.organization_industry_adoptions') IS NOT NULL
       AND to_regclass('platform.organization_industry_solution_adoptions') IS NULL THEN
        ALTER TABLE platform.organization_industry_adoptions RENAME TO organization_industry_solution_adoptions;
    END IF;

    IF to_regclass('platform.organization_industry_extensions') IS NOT NULL
       AND to_regclass('platform.organization_industry_solution_extensions') IS NULL THEN
        ALTER TABLE platform.organization_industry_extensions RENAME TO organization_industry_solution_extensions;
    END IF;

    IF to_regclass('platform.custom_package_publication_requests') IS NOT NULL
       AND to_regclass('platform.industry_solution_publication_requests') IS NULL THEN
        ALTER TABLE platform.custom_package_publication_requests RENAME TO industry_solution_publication_requests;
    END IF;

    IF to_regclass('platform.knowledge_sources') IS NOT NULL
       AND to_regclass('platform.industry_solution_knowledge_sources') IS NULL THEN
        ALTER TABLE platform.knowledge_sources RENAME TO industry_solution_knowledge_sources;
    END IF;
END;
$$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'platform'
          AND table_name = 'industry_solution_template_masters'
          AND column_name = 'package_json'
    ) THEN
        ALTER TABLE platform.industry_solution_template_masters RENAME COLUMN package_json TO manifest_json;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'platform'
          AND table_name = 'organization_industry_solution_targets'
          AND column_name = 'schema_name'
    ) THEN
        ALTER TABLE platform.organization_industry_solution_targets RENAME COLUMN schema_name TO target_schema_name;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'platform'
          AND table_name = 'industry_solution_change_requests'
          AND column_name = 'schema_name'
    ) THEN
        ALTER TABLE platform.industry_solution_change_requests RENAME COLUMN schema_name TO target_schema_name;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'platform'
          AND table_name = 'industry_solution_change_requests'
          AND column_name = 'schema_package'
    ) THEN
        ALTER TABLE platform.industry_solution_change_requests RENAME COLUMN schema_package TO solution_manifest;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'platform'
          AND table_name = 'industry_solution_apply_jobs'
          AND column_name = 'schema_name'
    ) THEN
        ALTER TABLE platform.industry_solution_apply_jobs RENAME COLUMN schema_name TO target_schema_name;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'platform'
          AND table_name = 'industry_solutions'
          AND column_name = 'package_key'
    ) THEN
        ALTER TABLE platform.industry_solutions RENAME COLUMN package_key TO solution_key;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'platform'
          AND table_name = 'industry_solution_assets'
          AND column_name = 'package_id'
    ) THEN
        ALTER TABLE platform.industry_solution_assets RENAME COLUMN package_id TO solution_id;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'platform'
          AND table_name = 'organization_industry_solution_adoptions'
          AND column_name = 'package_id'
    ) THEN
        ALTER TABLE platform.organization_industry_solution_adoptions RENAME COLUMN package_id TO solution_id;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'platform'
          AND table_name = 'organization_industry_solution_extensions'
          AND column_name = 'package_id'
    ) THEN
        ALTER TABLE platform.organization_industry_solution_extensions RENAME COLUMN package_id TO solution_id;
    END IF;
END;
$$;

ALTER TABLE platform.industry_solution_template_masters
    ALTER COLUMN version_key SET DEFAULT 'meta-org.industry-solution-manifest.v1';

ALTER TABLE platform.organization_industry_solution_targets
    ALTER COLUMN template_version SET DEFAULT 'meta-org.industry-solution-manifest.v1';

ALTER TABLE platform.industry_solution_change_requests
    ALTER COLUMN request_type SET DEFAULT 'industry_solution_manifest_import',
    ADD COLUMN IF NOT EXISTS risk_level TEXT NOT NULL DEFAULT 'safe'
        CHECK (risk_level IN ('safe', 'destructive')),
    ADD COLUMN IF NOT EXISTS diff JSONB NOT NULL DEFAULT '[]'::jsonb;

UPDATE platform.industry_solution_template_masters
SET version_key = 'meta-org.industry-solution-manifest.v1'
WHERE version_key = 'meta-org.schema.v1';

UPDATE platform.organization_industry_solution_targets
SET template_version = 'meta-org.industry-solution-manifest.v1'
WHERE template_version = 'meta-org.schema.v1';

UPDATE platform.industry_solution_template_masters
SET manifest_json = jsonb_set(manifest_json, '{format_version}', '"meta-org.industry-solution-manifest.v1"', true)
WHERE manifest_json->>'format_version' = 'meta-org.schema.v1';

UPDATE platform.industry_solution_change_requests
SET solution_manifest = jsonb_set(solution_manifest, '{format_version}', '"meta-org.industry-solution-manifest.v1"', true)
WHERE solution_manifest->>'format_version' = 'meta-org.schema.v1';

UPDATE platform.industry_solution_change_requests
SET request_type = CASE request_type
    WHEN 'import_schema_package' THEN 'industry_solution_manifest_import'
    WHEN 'schema_package_update' THEN 'industry_solution_manifest_import'
    ELSE request_type
END
WHERE request_type IN ('import_schema_package', 'schema_package_update');

UPDATE platform.industry_solution_assets
SET asset_type = CASE asset_type
    WHEN 'schema_package' THEN 'solution_manifest'
    WHEN 'solution_table' THEN 'database_table'
    WHEN 'solution_field' THEN 'database_field'
    ELSE asset_type
END
WHERE asset_type IN ('schema_package', 'solution_table', 'solution_field');

ALTER INDEX IF EXISTS platform.uq_schema_template_masters_key RENAME TO uq_industry_solution_template_masters_key;
ALTER INDEX IF EXISTS platform.idx_schema_template_details_master RENAME TO idx_industry_solution_template_details_master;
ALTER INDEX IF EXISTS platform.uq_schema_template_details_natural RENAME TO uq_industry_solution_template_details_natural;
ALTER INDEX IF EXISTS platform.idx_schema_change_requests_org RENAME TO idx_industry_solution_change_requests_org;
ALTER INDEX IF EXISTS platform.idx_schema_apply_jobs_request RENAME TO idx_industry_solution_apply_jobs_request;
ALTER INDEX IF EXISTS platform.idx_custom_packages_industry RENAME TO idx_industry_solutions_industry;
ALTER INDEX IF EXISTS platform.idx_custom_package_assets_type RENAME TO idx_industry_solution_assets_type;
ALTER INDEX IF EXISTS platform.idx_organization_industry_adoptions_industry RENAME TO idx_organization_industry_solution_adoptions_industry;
ALTER INDEX IF EXISTS platform.idx_organization_industry_extensions_lookup RENAME TO idx_organization_industry_solution_extensions_lookup;
ALTER INDEX IF EXISTS platform.idx_custom_package_publication_requests_status RENAME TO idx_industry_solution_publication_requests_status;
ALTER INDEX IF EXISTS platform.idx_knowledge_sources_scope RENAME TO idx_industry_solution_knowledge_sources_scope;

CREATE UNIQUE INDEX IF NOT EXISTS uq_industry_solution_template_masters_key
    ON platform.industry_solution_template_masters(template_key, version_key);
CREATE INDEX IF NOT EXISTS idx_industry_solution_template_details_master
    ON platform.industry_solution_template_details(master_key, detail_type, line_no);
CREATE UNIQUE INDEX IF NOT EXISTS uq_industry_solution_template_details_natural
    ON platform.industry_solution_template_details(master_key, detail_type, field_key, line_no);
CREATE INDEX IF NOT EXISTS idx_industry_solution_change_requests_org
    ON platform.industry_solution_change_requests(organization_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_industry_solution_apply_jobs_request
    ON platform.industry_solution_apply_jobs(change_request_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_industry_solutions_industry
    ON platform.industry_solutions(industry_key, status, version DESC);
CREATE INDEX IF NOT EXISTS idx_industry_solution_assets_type
    ON platform.industry_solution_assets(solution_id, asset_type, asset_key);
CREATE INDEX IF NOT EXISTS idx_organization_industry_solution_adoptions_industry
    ON platform.organization_industry_solution_adoptions(industry_key, status);
CREATE INDEX IF NOT EXISTS idx_organization_industry_solution_extensions_lookup
    ON platform.organization_industry_solution_extensions(organization_id, industry_key, status);
CREATE INDEX IF NOT EXISTS idx_industry_solution_publication_requests_status
    ON platform.industry_solution_publication_requests(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_industry_solution_knowledge_sources_scope
    ON platform.industry_solution_knowledge_sources(industry_key, organization_id, sync_status);

COMMENT ON TABLE platform.industry_solution_apply_jobs IS
    'Industry solution apply execution log. Stores per-asset apply results in metadata.asset_results.';

INSERT INTO platform.platform_permissions(permission_key, name, description, category, status, metadata)
VALUES
    ('industry.solution.verify', 'Verify industry solutions', 'Verify tenant-scoped industry solution change requests', 'industry_solution', 'active', '{"seed":true}'::jsonb),
    ('industry.solution.approve', 'Approve industry solutions', 'Approve tenant-scoped industry solution change requests', 'industry_solution', 'active', '{"seed":true}'::jsonb),
    ('industry.solution.apply', 'Apply industry solutions', 'Apply approved tenant-scoped industry solution change requests', 'industry_solution', 'active', '{"seed":true}'::jsonb)
ON CONFLICT (permission_key) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    category = EXCLUDED.category,
    status = EXCLUDED.status,
    metadata = platform.platform_permissions.metadata || EXCLUDED.metadata,
    updated_at = NOW();

UPDATE platform.platform_permissions
SET name = 'Manage industry solutions',
    description = 'Manage industry solutions, tables, fields, modules, and tenant adaptation changes',
    category = 'industry_solution',
    updated_at = NOW()
WHERE permission_key = 'industry.solution.manage';

UPDATE platform.platform_permissions
SET description = 'Import structured industry solution manifest JSON', updated_at = NOW()
WHERE permission_key = 'industry.solution.import';

UPDATE platform.platform_permissions
SET description = 'Export structured industry solution manifest JSON', updated_at = NOW()
WHERE permission_key = 'industry.solution.export';

WITH role_permission_matrix(role_key, permission_key) AS (
    SELECT role_key, permission_key
    FROM (VALUES ('owner'), ('admin')) AS roles(role_key)
    CROSS JOIN (VALUES
        ('industry.solution.verify'),
        ('industry.solution.approve'),
        ('industry.solution.apply')
    ) AS permissions(permission_key)
    UNION ALL
    SELECT 'operator', permission_key
    FROM (VALUES
        ('industry.solution.manage'),
        ('industry.solution.import'),
        ('industry.solution.export'),
        ('industry.solution.verify')
    ) AS permissions(permission_key)
)
INSERT INTO platform.platform_role_permissions(role_key, permission_key, status)
SELECT role_key, permission_key, 'active'
FROM role_permission_matrix
ON CONFLICT (role_key, permission_key) DO UPDATE SET
    status = 'active',
    updated_at = NOW();

DELETE FROM platform.platform_role_permissions
WHERE permission_key IN ('schema.manage', 'schema.approve', 'schema.apply');

DELETE FROM platform.platform_permissions
WHERE permission_key IN ('schema.manage', 'schema.approve', 'schema.apply');

DELETE FROM platform.platform_menu_items
WHERE menu_key IN ('targets', 'schema');

DELETE FROM platform.platform_features
WHERE feature_key IN ('platform.schema.targets', 'platform.schema.package');

UPDATE platform.platform_features
SET description = 'Industry solution manifests, table and field changes, import, export, verification, approval, apply, and tenant adaptation',
    permission_keys = '["industry.solution.manage"]'::jsonb,
    updated_at = NOW()
WHERE feature_key = 'platform.industry.solutions';

UPDATE platform.platform_roles
SET description = 'Platform operator for daily organization, runtime, and industry solution operations',
    updated_at = NOW()
WHERE role_key = 'operator';

DO $$
BEGIN
    IF to_regclass('public.tool_definitions') IS NOT NULL THEN
        UPDATE public.tool_definitions
        SET name = 'industry.solution.change.preview',
            description = 'Verify an industry solution change request without applying it',
            input_schema = '{"type":"object","properties":{"industry_solution_change_request_id":{"type":"string"},"context_package_id":{"type":"string"}},"required":["industry_solution_change_request_id"]}'::jsonb,
            metadata = metadata || '{"quality_gate":"solution_verify"}'::jsonb,
            updated_at = NOW()
        WHERE name = 'schema.change.preview'
          AND NOT EXISTS (
              SELECT 1 FROM public.tool_definitions WHERE name = 'industry.solution.change.preview'
          );

        DELETE FROM public.tool_definitions
        WHERE name = 'schema.change.preview';
    END IF;
END;
$$;
