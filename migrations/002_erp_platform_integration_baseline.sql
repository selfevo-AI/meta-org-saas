-- 002_erp_platform_integration_baseline.sql
-- ERP platform-integration baseline.
--
-- This file runs after 000_saas_platform_management_baseline.sql and
-- 001_erp_code_baseline.sql. It owns platform/runtime integration that depends
-- on ERP baseline metadata such as module_master_source_catalog. Keep SaaS
-- platform primitives in 000 and ERP industry code-table/business baseline in
-- 001; put cross-stage ERP runtime/workbench projections here.
--
-- 002 负责 ERP 基线完成之后的平台联动：基于 ERP/master-detail 注册生成
-- runtime entity/view/operation、工作台操作和组织运行时 schema 投影。
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

INSERT INTO platform.runtime_operations(
    operation_key, domain, title, method, path, auth, query_params,
    operation_kind, danger_level, result_view, assistant_eligible,
    action_type, entity_key, metadata
)
VALUES (
    'erp.finance.trial_balance.run',
    'ERP',
    'operation.erp.MGLR.run',
    'GET',
    '/finance/gl/trial-balance',
    TRUE,
    '[
        {"name":"period_start","label":"operation.param.periodStart","placeholder":"YYYY-MM-DD"},
        {"name":"period_end","label":"operation.param.periodEnd","placeholder":"YYYY-MM-DD"},
        {"name":"currency","label":"operation.param.currency","placeholder":"CNY"}
    ]'::jsonb,
    'direct',
    'low',
    'list',
    TRUE,
    'finance.gl.trial_balance',
    NULL,
    '{
        "source":"erp_platform_integration_seed",
        "workspace":{
            "module":"finance",
            "document_id":"trial_balance",
            "document_label_key":"erp.document.trialBalance",
            "submodule_key":"erp.submodule.trialBalance",
            "table_code":"MGLR",
            "primary_key":"ReportCode",
            "child_code":"",
            "kind":"report",
            "action":"run",
            "state_gate":"MGLR.run",
            "action_params":{},
            "sort_order":41
        }
    }'::jsonb
)
ON CONFLICT (operation_key) DO UPDATE SET
    domain = EXCLUDED.domain,
    title = EXCLUDED.title,
    method = EXCLUDED.method,
    path = EXCLUDED.path,
    query_params = EXCLUDED.query_params,
    operation_kind = EXCLUDED.operation_kind,
    danger_level = EXCLUDED.danger_level,
    result_view = EXCLUDED.result_view,
    assistant_eligible = EXCLUDED.assistant_eligible,
    action_type = EXCLUDED.action_type,
    entity_key = EXCLUDED.entity_key,
    metadata = platform.runtime_operations.metadata || EXCLUDED.metadata,
    updated_at = NOW();


SELECT platform.provision_runtime_organization(id)
FROM public.organizations;
