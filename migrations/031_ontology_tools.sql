-- Ontology tools are platform AI capability metadata, not tenant business data.
-- Older industry generators registered asset keys and per-table aliases even
-- though no internal adapter existed for those names. Keep their audit identity.
UPDATE tool_definitions SET is_active = FALSE, default_policy = 'deny',
    metadata = metadata || '{"operational_status":"retired","replacement":"ontology.action.execute"}'::jsonb,
    updated_at = NOW()
WHERE source_type = 'internal_api'
  AND (name ~ '^(tool_definition|tool_policy)\.' OR name ~ '^erp\.m[a-z0-9]+\.');

INSERT INTO tool_definitions (
    name, description, source_type, default_policy, risk_level, required_level,
    tool_category, approval_tier_required, input_schema, metadata
)
SELECT name, label_en, 'internal_api',
    CASE WHEN name = 'ontology.action.execute' THEN 'approve' ELSE 'notify' END,
    CASE WHEN name = 'ontology.action.execute' THEN 'high' ELSE 'low' END,
    CASE WHEN name = 'ontology.action.execute' THEN 'L3' ELSE 'L1' END,
    CASE WHEN name = 'ontology.action.execute' THEN 'business_approval' ELSE 'execution_operation' END,
    CASE WHEN name = 'ontology.action.execute' THEN 'reviewer' ELSE 'executor' END,
    schema,
    jsonb_build_object('label_zh', label_zh, 'label_en', label_en,
        'description_zh', label_zh, 'description_en', label_en,
        'ontology_version', 1, 'quality_gate', 'tenant_permissions_and_tool_approval')
FROM (VALUES
    ('ontology.types.list', '查询业务对象类型', 'List accessible business object types and their properties, links and actions',
        '{"type":"object","properties":{}}'::jsonb),
    ('ontology.objects.query', '查询业务对象', 'Query authoritative business objects using semantic property filters; follow next_cursor for more results',
        '{"type":"object","properties":{"object_type":{"type":"string","x-label-zh":"对象类型","x-label-en":"Object Type"},"filters":{"type":"object","x-label-zh":"属性筛选","x-label-en":"Property Filters"},"search":{"type":"string","x-label-zh":"搜索","x-label-en":"Search"},"cursor":{"type":"string","x-label-zh":"分页游标","x-label-en":"Cursor"}},"required":["object_type"]}'::jsonb),
    ('ontology.objects.get', '读取业务对象', 'Get an authoritative business object',
        '{"type":"object","properties":{"object_type":{"type":"string","x-label-zh":"对象类型","x-label-en":"Object Type"},"key":{"type":"string","x-label-zh":"对象编号","x-label-en":"Object Key"}},"required":["object_type","key"]}'::jsonb),
    ('ontology.objects.links', '查询业务对象关系', 'Read the upstream and downstream relationships of a business object',
        '{"type":"object","properties":{"object_type":{"type":"string","x-label-zh":"对象类型","x-label-en":"Object Type"},"key":{"type":"string","x-label-zh":"对象编号","x-label-en":"Object Key"}},"required":["object_type","key"]}'::jsonb),
    ('ontology.action.execute', '执行已批准的业务动作', 'Request approval and execute a business action on an existing object; use action parameters from ontology.types.list',
        '{"type":"object","properties":{"object_type":{"type":"string","x-label-zh":"对象类型","x-label-en":"Object Type"},"key":{"type":"string","x-label-zh":"对象编号","x-label-en":"Object Key"},"action":{"type":"string","x-label-zh":"业务动作","x-label-en":"Business Action"},"data":{"type":"object","x-label-zh":"动作参数","x-label-en":"Action Parameters"}},"required":["object_type","key","action"]}'::jsonb)
) AS tools(name, label_zh, label_en, schema)
ON CONFLICT (name) DO UPDATE SET
    description = EXCLUDED.description,
    default_policy = EXCLUDED.default_policy,
    risk_level = EXCLUDED.risk_level,
    required_level = EXCLUDED.required_level,
    tool_category = EXCLUDED.tool_category,
    approval_tier_required = EXCLUDED.approval_tier_required,
    input_schema = EXCLUDED.input_schema,
    metadata = tool_definitions.metadata || EXCLUDED.metadata,
    updated_at = NOW();
-- platformdb:accept-checksum-drift 004_ai_capability_baseline.sql
