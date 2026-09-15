-- platformdb:accept-checksum-drift 002_erp_platform_integration_baseline.sql
-- External-document operations are human-only tenant workflows.
INSERT INTO platform.runtime_operations (
    operation_key, domain, title, method, path, auth, path_params, query_params, body_template,
    operation_kind, danger_level, result_view, assistant_eligible, action_type, metadata
)
SELECT 'document_import.' || a.key, 'ERP', 'import.operation.' || a.key,
    a.method, '/document-imports' || a.suffix, TRUE,
    CASE WHEN a.key='list' THEN '[]'::jsonb ELSE '[{"name":"id","label":"import.field.id"}]'::jsonb END,
    CASE WHEN a.key='list' THEN '[{"name":"object_type","label":"import.field.object_type"},{"name":"status","label":"import.field.status"},{"name":"cursor","label":"import.field.cursor"}]'::jsonb ELSE '[]'::jsonb END,
    a.body::jsonb, 'contextual',
    CASE WHEN a.method='GET' THEN 'low' ELSE 'high' END,
    'detail', FALSE, 'document_import.' || a.key,
    jsonb_build_object('source','ontology_document_import','human_only',TRUE,
        'title_i18n',jsonb_build_object('zh',a.zh,'en',a.en),
        'parameter_labels',jsonb_build_object('object_type','import.field.object_type',
            'id','import.field.id','version','import.field.version','draft','import.field.draft',
            'confirmed','import.field.confirmed'))
FROM (VALUES
 ('list','GET','','{}','查询单据导入','List document imports'),
 ('get','GET','/{id}','{}','读取单据导入','Get document import'),
 ('recognize','POST','/{id}/recognize','{"version":1}','识别外部单据','Recognize external document'),
 ('review','PATCH','/{id}/review','{"version":1,"draft":{"key":"","properties":{},"lines":[]}}','保存单据复核','Save document review'),
 ('confirm','POST','/{id}/confirm','{"version":1,"draft":{"key":"","properties":{},"lines":[]},"confirmed":false}','确认外部单据','Confirm external document'),
 ('reject','POST','/{id}/reject','{"version":1}','拒绝外部单据','Reject external document')
) a(key,method,suffix,body,zh,en)
ON CONFLICT (operation_key) DO NOTHING;
