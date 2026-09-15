-- platformdb:accept-checksum-drift 002_erp_platform_integration_baseline.sql
-- Operational Ontology: retire duplicate and quantity-only mutation surfaces.
UPDATE platform.runtime_operations SET status = 'disabled', assistant_eligible = FALSE,
    metadata = metadata || '{"operational_status":"legacy_read_only","replacement":"/ontology/types"}'::jsonb,
    updated_at = NOW()
WHERE
    path ~ '^/finance/(receivables|receipts|payables|payments)(/|$)'
    OR (method <> 'GET' AND (
        metadata #>> '{workspace,module}' IN ('retail', 'manufacturing')
        OR path ~ '^/erp/(MBRN|MTER|MMBR|MPRM|MPUB|MRPS|MDRQ|MDSP|MDRC|MDIF|MSTP|MCNT|MSPR|MBOM|MWOR)(/|$)'
    ));

INSERT INTO platform.runtime_operations (
    operation_key, domain, title, method, path, auth, path_params, body_template,
    operation_kind, danger_level, result_view, assistant_eligible, action_type, metadata
)
SELECT 'erp.' || lower(code) || '.' || action, 'ERP', 'erp.action.' || action,
    'POST', '/erp/' || code || '/{key}/actions/' || action, TRUE,
    '[{"name":"key","label":"ontology.field.key"}]'::jsonb,
    CASE WHEN action = 'allocate' THEN '{"data":{"TargetKey":"","Amount":0}}'::jsonb ELSE '{"data":{}}'::jsonb END,
    'contextual', 'high', 'detail', FALSE, 'erp.action',
    jsonb_build_object('source', 'operational_ontology', 'ontology_type', object_type,
        'workspace', jsonb_build_object('module', module, 'document_id', document_id,
            'table_code', code, 'primary_key', 'DocEntry', 'action', action))
FROM (VALUES
    ('MPOR','receive','purchase_order','procurement','purchase_order'),
    ('MPDN','approve','goods_receipt','procurement','goods_receipt_po'),
    ('MRDR','deliver','sales_order','sales','sales_order'),
    ('MDLN','approve','delivery','sales','delivery'),
    ('MPCH','post','payable_invoice','finance','ap_invoice'),
    ('MVPM','allocate','outgoing_payment','finance','outgoing_payment')
) AS actions(code, action, object_type, module, document_id)
ON CONFLICT (operation_key) DO UPDATE SET
    title = EXCLUDED.title, path = EXCLUDED.path, path_params = EXCLUDED.path_params,
    body_template = EXCLUDED.body_template, action_type = EXCLUDED.action_type,
    metadata = platform.runtime_operations.metadata || EXCLUDED.metadata, updated_at = NOW();
