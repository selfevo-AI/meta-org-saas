-- 042_configurable_runtime_kernel.sql
-- Configurable SaaS runtime kernel.
--
-- This is a new-baseline foundation for configurable runtime metadata and
-- per-organization record storage. It is intentionally non-destructive.

CREATE SCHEMA IF NOT EXISTS platform;

CREATE TABLE IF NOT EXISTS platform.runtime_entities (
    entity_key    TEXT PRIMARY KEY,
    module_key    TEXT NOT NULL,
    storage_table TEXT NOT NULL,
    entity_type   TEXT NOT NULL,
    title_key     TEXT NOT NULL DEFAULT '',
    status        TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'disabled', 'archived')),
    fields        JSONB NOT NULL DEFAULT '[]'
        CHECK (jsonb_typeof(fields) = 'array'),
    metadata      JSONB NOT NULL DEFAULT '{}'
        CHECK (jsonb_typeof(metadata) = 'object'),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_runtime_entities_module
    ON platform.runtime_entities(module_key, status, entity_type);

CREATE TABLE IF NOT EXISTS platform.runtime_views (
    view_key      TEXT PRIMARY KEY,
    entity_key    TEXT NOT NULL REFERENCES platform.runtime_entities(entity_key) ON DELETE CASCADE,
    view_type     TEXT NOT NULL DEFAULT 'list'
        CHECK (view_type IN ('list', 'detail', 'form', 'kanban', 'tree')),
    title_key     TEXT NOT NULL DEFAULT '',
    config        JSONB NOT NULL DEFAULT '{}'
        CHECK (jsonb_typeof(config) = 'object'),
    status        TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'disabled', 'archived')),
    metadata      JSONB NOT NULL DEFAULT '{}'
        CHECK (jsonb_typeof(metadata) = 'object'),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_runtime_views_entity
    ON platform.runtime_views(entity_key, view_type, status);

CREATE TABLE IF NOT EXISTS platform.runtime_operations (
    operation_key           TEXT PRIMARY KEY,
    domain                  TEXT NOT NULL,
    title                   TEXT NOT NULL,
    method                  TEXT NOT NULL
        CHECK (method IN ('GET', 'POST', 'PUT', 'PATCH', 'DELETE')),
    path                    TEXT NOT NULL,
    auth                    BOOLEAN NOT NULL DEFAULT TRUE,
    path_params             JSONB NOT NULL DEFAULT '[]'
        CHECK (jsonb_typeof(path_params) = 'array'),
    query_params            JSONB NOT NULL DEFAULT '[]'
        CHECK (jsonb_typeof(query_params) = 'array'),
    body_template           JSONB,
    operation_kind          TEXT NOT NULL DEFAULT 'direct'
        CHECK (operation_kind IN ('direct', 'contextual', 'agent_assisted', 'admin')),
    danger_level            TEXT NOT NULL DEFAULT 'low'
        CHECK (danger_level IN ('low', 'medium', 'high')),
    result_view             TEXT NOT NULL DEFAULT 'summary'
        CHECK (result_view IN ('summary', 'list', 'detail', 'audit')),
    assistant_eligible      BOOLEAN NOT NULL DEFAULT FALSE,
    requires_entity_context BOOLEAN NOT NULL DEFAULT FALSE,
    status                  TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'disabled', 'archived')),
    action_type             TEXT NOT NULL DEFAULT 'crud.list',
    entity_key              TEXT REFERENCES platform.runtime_entities(entity_key) ON DELETE SET NULL,
    adapter_key             TEXT NOT NULL DEFAULT '',
    metadata                JSONB NOT NULL DEFAULT '{}'
        CHECK (jsonb_typeof(metadata) = 'object'),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_runtime_operations_domain
    ON platform.runtime_operations(domain, status, operation_key);

CREATE TABLE IF NOT EXISTS platform.runtime_i18n (
    locale     TEXT NOT NULL CHECK (locale IN ('zh', 'en')),
    label_key  TEXT NOT NULL,
    value      TEXT NOT NULL,
    metadata   JSONB NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (locale, label_key)
);

CREATE TABLE IF NOT EXISTS platform.runtime_publish_history (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    version_key  TEXT NOT NULL,
    status       TEXT NOT NULL DEFAULT 'published'
        CHECK (status IN ('draft', 'published', 'rolled_back')),
    published_by UUID REFERENCES public.users(id) ON DELETE SET NULL,
    summary      TEXT NOT NULL DEFAULT '',
    snapshot     JSONB NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS platform.runtime_audit_logs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID REFERENCES public.organizations(id) ON DELETE SET NULL,
    actor_id        UUID REFERENCES public.users(id) ON DELETE SET NULL,
    actor_type      TEXT NOT NULL DEFAULT '',
    operation_key   TEXT NOT NULL DEFAULT '',
    entity_key      TEXT NOT NULL DEFAULT '',
    record_key      TEXT NOT NULL DEFAULT '',
    action_type     TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'ok',
    payload         JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_runtime_audit_logs_org
    ON platform.runtime_audit_logs(organization_id, created_at DESC);

CREATE OR REPLACE FUNCTION platform.runtime_module_key(p_source_module TEXT)
RETURNS TEXT
LANGUAGE sql
IMMUTABLE
AS $$
    SELECT CASE p_source_module
        WHEN 'project_lifecycle' THEN 'project'
        WHEN 'aigateway' THEN 'ai_gateway'
        WHEN 'metaresource' THEN 'meta_resource'
        ELSE p_source_module
    END;
$$;

INSERT INTO platform.runtime_entities(entity_key, module_key, storage_table, entity_type, title_key, status, fields, metadata)
SELECT
    platform.runtime_module_key(c.module_name) || '.' || c.entity_type,
    platform.runtime_module_key(c.module_name),
    c.module_name || '_masters',
    c.entity_type,
    'runtime.entity.' || platform.runtime_module_key(c.module_name) || '.' || c.entity_type,
    'active',
    jsonb_build_array(
        jsonb_build_object('field_key', 'title', 'label_key', 'runtime.field.title', 'data_type', 'text', 'required', true, 'display_order', 10),
        jsonb_build_object('field_key', 'status', 'label_key', 'runtime.field.status', 'data_type', 'text', 'required', false, 'display_order', 20),
        jsonb_build_object('field_key', 'core_data', 'label_key', 'runtime.field.coreData', 'data_type', 'json', 'required', false, 'display_order', 30)
    ),
    jsonb_build_object(
        'source', 'module_master_source_catalog',
        'source_module', c.module_name,
        'source_table', c.source_table,
        'relation_mode', c.relation_mode,
        'parent_table', COALESCE(c.parent_table, ''),
        'parent_fk', COALESCE(c.parent_fk, ''),
        'key_prefix', c.key_prefix
    )
FROM public.module_master_source_catalog c
ON CONFLICT (entity_key) DO UPDATE SET
    module_key = EXCLUDED.module_key,
    storage_table = EXCLUDED.storage_table,
    entity_type = EXCLUDED.entity_type,
    title_key = EXCLUDED.title_key,
    status = EXCLUDED.status,
    fields = EXCLUDED.fields,
    metadata = platform.runtime_entities.metadata || EXCLUDED.metadata,
    updated_at = NOW();

INSERT INTO platform.runtime_views(view_key, entity_key, view_type, title_key, config, metadata)
SELECT
    entity_key || '.list',
    entity_key,
    'list',
    title_key || '.list',
    jsonb_build_object(
        'columns', jsonb_build_array('master_key', 'title', 'status', 'updated_at'),
        'default_sort', jsonb_build_array(jsonb_build_object('field', 'updated_at', 'direction', 'desc'))
    ),
    '{"source":"runtime_seed"}'::jsonb
FROM platform.runtime_entities
ON CONFLICT (view_key) DO UPDATE SET
    title_key = EXCLUDED.title_key,
    config = EXCLUDED.config,
    metadata = platform.runtime_views.metadata || EXCLUDED.metadata,
    updated_at = NOW();

INSERT INTO platform.runtime_operations(
    operation_key, domain, title, method, path, auth, query_params, body_template,
    operation_kind, danger_level, result_view, action_type, entity_key, metadata
)
SELECT
    entity_key || '.list',
    module_key,
    'operation.runtime.' || entity_key || '.list',
    'GET',
    '/runtime/entities/' || entity_key || '/records',
    TRUE,
    '[{"name":"limit","label":"operation.param.limit","placeholder":"100"}]'::jsonb,
    NULL,
    'direct',
    'low',
    'list',
    'crud.list',
    entity_key,
    '{"source":"runtime_seed"}'::jsonb
FROM platform.runtime_entities
ON CONFLICT (operation_key) DO UPDATE SET
    domain = EXCLUDED.domain,
    title = EXCLUDED.title,
    method = EXCLUDED.method,
    path = EXCLUDED.path,
    query_params = EXCLUDED.query_params,
    result_view = EXCLUDED.result_view,
    action_type = EXCLUDED.action_type,
    entity_key = EXCLUDED.entity_key,
    updated_at = NOW();

INSERT INTO platform.runtime_operations(
    operation_key, domain, title, method, path, auth, body_template,
    operation_kind, danger_level, result_view, action_type, entity_key, metadata
)
SELECT
    entity_key || '.create',
    module_key,
    'operation.runtime.' || entity_key || '.create',
    'POST',
    '/runtime/entities/' || entity_key || '/records',
    TRUE,
    '{"title":"","status":"active","data":{},"metadata":{}}'::jsonb,
    'direct',
    'medium',
    'detail',
    'crud.create',
    entity_key,
    '{"source":"runtime_seed"}'::jsonb
FROM platform.runtime_entities
ON CONFLICT (operation_key) DO UPDATE SET
    domain = EXCLUDED.domain,
    title = EXCLUDED.title,
    method = EXCLUDED.method,
    path = EXCLUDED.path,
    body_template = EXCLUDED.body_template,
    danger_level = EXCLUDED.danger_level,
    result_view = EXCLUDED.result_view,
    action_type = EXCLUDED.action_type,
    entity_key = EXCLUDED.entity_key,
    updated_at = NOW();

INSERT INTO platform.runtime_operations(
    operation_key, domain, title, method, path, auth, path_params, body_template,
    operation_kind, danger_level, result_view, requires_entity_context, action_type, entity_key, metadata
)
SELECT
    entity_key || '.update',
    module_key,
    'operation.runtime.' || entity_key || '.update',
    'PATCH',
    '/runtime/entities/' || entity_key || '/records/{recordKey}',
    TRUE,
    '[{"name":"recordKey","label":"operation.param.recordKey"}]'::jsonb,
    '{"title":"","status":"active","data":{},"metadata":{}}'::jsonb,
    'contextual',
    'medium',
    'detail',
    TRUE,
    'crud.update',
    entity_key,
    '{"source":"runtime_seed"}'::jsonb
FROM platform.runtime_entities
ON CONFLICT (operation_key) DO UPDATE SET
    domain = EXCLUDED.domain,
    title = EXCLUDED.title,
    method = EXCLUDED.method,
    path = EXCLUDED.path,
    path_params = EXCLUDED.path_params,
    body_template = EXCLUDED.body_template,
    operation_kind = EXCLUDED.operation_kind,
    danger_level = EXCLUDED.danger_level,
    requires_entity_context = EXCLUDED.requires_entity_context,
    action_type = EXCLUDED.action_type,
    entity_key = EXCLUDED.entity_key,
    updated_at = NOW();

INSERT INTO platform.runtime_operations(
    operation_key, domain, title, method, path, auth, path_params,
    operation_kind, danger_level, result_view, requires_entity_context, action_type, entity_key, metadata
)
SELECT
    entity_key || '.delete',
    module_key,
    'operation.runtime.' || entity_key || '.delete',
    'DELETE',
    '/runtime/entities/' || entity_key || '/records/{recordKey}',
    TRUE,
    '[{"name":"recordKey","label":"operation.param.recordKey"}]'::jsonb,
    'contextual',
    'high',
    'audit',
    TRUE,
    'crud.delete',
    entity_key,
    '{"source":"runtime_seed"}'::jsonb
FROM platform.runtime_entities
ON CONFLICT (operation_key) DO UPDATE SET
    domain = EXCLUDED.domain,
    title = EXCLUDED.title,
    method = EXCLUDED.method,
    path = EXCLUDED.path,
    path_params = EXCLUDED.path_params,
    operation_kind = EXCLUDED.operation_kind,
    danger_level = EXCLUDED.danger_level,
    result_view = EXCLUDED.result_view,
    requires_entity_context = EXCLUDED.requires_entity_context,
    action_type = EXCLUDED.action_type,
    entity_key = EXCLUDED.entity_key,
    updated_at = NOW();

INSERT INTO platform.runtime_i18n(locale, label_key, value)
VALUES
    ('en', 'operation.param.limit', 'Limit'),
    ('zh', 'operation.param.limit', '数量'),
    ('en', 'operation.param.recordKey', 'Record key'),
    ('zh', 'operation.param.recordKey', '记录编号'),
    ('en', 'runtime.field.title', 'Title'),
    ('zh', 'runtime.field.title', '标题'),
    ('en', 'runtime.field.status', 'Status'),
    ('zh', 'runtime.field.status', '状态'),
    ('en', 'runtime.field.coreData', 'Core data'),
    ('zh', 'runtime.field.coreData', '核心数据')
ON CONFLICT (locale, label_key) DO UPDATE SET
    value = EXCLUDED.value,
    updated_at = NOW();

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

    INSERT INTO platform.organization_schema_targets(
        organization_id, schema_name, template_version, status, metadata
    )
    VALUES (
        p_organization_id,
        v_schema_name,
        'meta-org.runtime.v1',
        'provisioned',
        '{"source":"runtime_kernel"}'::jsonb
    )
    ON CONFLICT (organization_id) DO UPDATE SET
        schema_name = EXCLUDED.schema_name,
        template_version = EXCLUDED.template_version,
        status = 'provisioned',
        metadata = platform.organization_schema_targets.metadata || EXCLUDED.metadata,
        updated_at = NOW();
END;
$$;

SELECT platform.provision_runtime_organization(id)
FROM public.organizations;
