-- 037_smt_edge_platform.sql
-- Platform-owned artifacts for the SMT MES industry solution:
--   1) platform.smt_edge_instances - registry of SMT edge (SMT-MES) instances per tenant.
--   2) Industry solution catalog registration (category, solution, assets,
--      capability package + marketplace listing) for smt_mes_v1.
--   3) platform.runtime_operations seeds for the SMT workspace / agent surface.

CREATE SCHEMA IF NOT EXISTS platform;

-- ---------------------------------------------------------------------------
-- 1) SMT edge instance registry.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS platform.smt_edge_instances (
    instance_key          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id       UUID NOT NULL REFERENCES public.organizations(id) ON DELETE CASCADE,
    name                  TEXT NOT NULL,
    base_url              TEXT NOT NULL DEFAULT '',
    api_key_cipher        TEXT NOT NULL DEFAULT '',
    webhook_secret_cipher TEXT NOT NULL DEFAULT '',
    subscribed_events     TEXT NOT NULL DEFAULT '*',
    status                TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'disabled')),
    auto_push             BOOLEAN NOT NULL DEFAULT TRUE,
    last_poll_id          BIGINT NOT NULL DEFAULT 0,
    last_sync_at          TIMESTAMPTZ,
    last_event_at         TIMESTAMPTZ,
    metadata              JSONB NOT NULL DEFAULT '{}'
        CHECK (jsonb_typeof(metadata) = 'object'),
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, name)
);

CREATE INDEX IF NOT EXISTS idx_smt_edge_instances_org
    ON platform.smt_edge_instances(organization_id, status, updated_at DESC);

COMMENT ON TABLE platform.smt_edge_instances IS
    'SMT edge (shop-floor SMT-MES) integration instances. API key and webhook secret are secretbox-encrypted.';

-- ---------------------------------------------------------------------------
-- 2) Industry solution catalog registration.
-- ---------------------------------------------------------------------------
INSERT INTO platform.industry_solution_categories(industry_key, name, description, status, metadata)
VALUES (
    'smt_manufacturing',
    'SMT Electronics Manufacturing',
    'Electronics assembly (SMT) shop-floor execution baseline: feeder verification, reel inventory, pass-station foolproofing, quality, andon, and downtime',
    'active',
    '{"seed":true,"source":"037_smt_edge_platform"}'::jsonb
)
ON CONFLICT (industry_key) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    status = EXCLUDED.status,
    metadata = platform.industry_solution_categories.metadata || EXCLUDED.metadata,
    updated_at = NOW();

WITH smt_package AS (
    INSERT INTO platform.industry_solutions(industry_key, solution_key, version, name, description, status, metadata)
    VALUES (
        'smt_manufacturing',
        'smt_mes_v1',
        1,
        'SMT MES',
        'SMT shop-floor execution solution: feeder verification, reel-level inventory, SN pass-station, FAI, repair, andon, downtime, panel flow, and edge integration',
        'active',
        '{"seed":true,"code_table_only":true,"module_key":"smt_mes"}'::jsonb
    )
    ON CONFLICT (industry_key, solution_key, version) DO UPDATE SET
        name = EXCLUDED.name,
        description = EXCLUDED.description,
        status = EXCLUDED.status,
        metadata = platform.industry_solutions.metadata || EXCLUDED.metadata,
        updated_at = NOW()
    RETURNING id
)
INSERT INTO platform.industry_solution_assets(solution_id, asset_key, asset_type, payload, metadata)
SELECT id, 'table-' || lower(table_code), 'database_table', payload, '{"seed":true,"solution_key":"smt_mes_v1"}'::jsonb
FROM smt_package
CROSS JOIN (VALUES
    ('MLIN', '{"table_code":"MLIN","display_name":"SMT Lines","module":"smt_mes","primary_key":"LineCode","child_tables":["LIN1"]}'::jsonb),
    ('LIN1', '{"table_code":"LIN1","display_name":"SMT Line - Machines","module":"smt_mes","parent_code":"MLIN","parent_key":"LineCode"}'::jsonb),
    ('MMAC', '{"table_code":"MMAC","display_name":"SMT Machines","module":"smt_mes","primary_key":"MachineCode","child_tables":["MAC1"]}'::jsonb),
    ('MAC1', '{"table_code":"MAC1","display_name":"SMT Machine - Slots","module":"smt_mes","parent_code":"MMAC","parent_key":"MachineCode"}'::jsonb),
    ('MFSB', '{"table_code":"MFSB","display_name":"Feeder Setups","module":"smt_mes","primary_key":"SetupCode","child_tables":["FSB1"]}'::jsonb),
    ('FSB1', '{"table_code":"FSB1","display_name":"Feeder Setup - Rows","module":"smt_mes","parent_code":"MFSB","parent_key":"SetupCode"}'::jsonb),
    ('MOPR', '{"table_code":"MOPR","display_name":"Operations","module":"smt_mes","primary_key":"OpCode"}'::jsonb),
    ('MROU', '{"table_code":"MROU","display_name":"Routings","module":"smt_mes","primary_key":"RouteCode","child_tables":["ROU1"]}'::jsonb),
    ('ROU1', '{"table_code":"ROU1","display_name":"Routing - Steps","module":"smt_mes","parent_code":"MROU","parent_key":"RouteCode"}'::jsonb),
    ('MPRD', '{"table_code":"MPRD","display_name":"SMT Products","module":"smt_mes","primary_key":"ProductCode","child_tables":["PRD1"]}'::jsonb),
    ('PRD1', '{"table_code":"PRD1","display_name":"SMT Product - BOM Rows","module":"smt_mes","parent_code":"MPRD","parent_key":"ProductCode"}'::jsonb),
    ('MCPM', '{"table_code":"MCPM","display_name":"Customer Part Maps","module":"smt_mes","primary_key":"MapCode"}'::jsonb),
    ('MREL', '{"table_code":"MREL","display_name":"Reels","module":"smt_mes","primary_key":"ReelCode","child_tables":["REL1"]}'::jsonb),
    ('REL1', '{"table_code":"REL1","display_name":"Reel - Transactions","module":"smt_mes","parent_code":"MREL","parent_key":"ReelCode"}'::jsonb),
    ('MSWO', '{"table_code":"MSWO","display_name":"SMT Work Orders","module":"smt_mes","primary_key":"WorkOrderCode","child_tables":["SWO1"]}'::jsonb),
    ('SWO1', '{"table_code":"SWO1","display_name":"SMT Work Order - Material Summary","module":"smt_mes","parent_code":"MSWO","parent_key":"WorkOrderCode"}'::jsonb),
    ('MRQN', '{"table_code":"MRQN","display_name":"Requisitions","module":"smt_mes","primary_key":"ReqNo","child_tables":["RQN1"]}'::jsonb),
    ('RQN1', '{"table_code":"RQN1","display_name":"Requisition - Items","module":"smt_mes","parent_code":"MRQN","parent_key":"ReqNo"}'::jsonb),
    ('MLOD', '{"table_code":"MLOD","display_name":"Loading Records","module":"smt_mes","primary_key":"LoadCode","child_tables":["LOD1"]}'::jsonb),
    ('LOD1', '{"table_code":"LOD1","display_name":"Loading Record - Slices","module":"smt_mes","parent_code":"MLOD","parent_key":"LoadCode"}'::jsonb),
    ('MVER', '{"table_code":"MVER","display_name":"Verification Scans","module":"smt_mes","primary_key":"ScanCode"}'::jsonb),
    ('MLWA', '{"table_code":"MLWA","display_name":"Low Material Alerts","module":"smt_mes","primary_key":"AlertCode"}'::jsonb),
    ('MUNT', '{"table_code":"MUNT","display_name":"Production Units","module":"smt_mes","primary_key":"SN","child_tables":["UNT1"]}'::jsonb),
    ('UNT1', '{"table_code":"UNT1","display_name":"Unit - Events","module":"smt_mes","parent_code":"MUNT","parent_key":"SN"}'::jsonb),
    ('MPNL', '{"table_code":"MPNL","display_name":"Panels","module":"smt_mes","primary_key":"PanelCode","child_tables":["PNL1"]}'::jsonb),
    ('PNL1', '{"table_code":"PNL1","display_name":"Panel - Boards","module":"smt_mes","parent_code":"MPNL","parent_key":"PanelCode"}'::jsonb),
    ('MDFC', '{"table_code":"MDFC","display_name":"Defect Codes","module":"smt_mes","primary_key":"DefectCode"}'::jsonb),
    ('MUDF', '{"table_code":"MUDF","display_name":"Unit Defects","module":"smt_mes","primary_key":"DefectID"}'::jsonb),
    ('MINS', '{"table_code":"MINS","display_name":"Inspections","module":"smt_mes","primary_key":"InspNo","child_tables":["INS1"]}'::jsonb),
    ('INS1', '{"table_code":"INS1","display_name":"Inspection - Items","module":"smt_mes","parent_code":"MINS","parent_key":"InspNo"}'::jsonb),
    ('MAND', '{"table_code":"MAND","display_name":"Andon Calls","module":"smt_mes","primary_key":"CallNo","child_tables":["AND1"]}'::jsonb),
    ('AND1', '{"table_code":"AND1","display_name":"Andon Call - Timeline","module":"smt_mes","parent_code":"MAND","parent_key":"CallNo"}'::jsonb),
    ('MDWN', '{"table_code":"MDWN","display_name":"Downtimes","module":"smt_mes","primary_key":"DowntimeCode"}'::jsonb),
    ('MSET', '{"table_code":"MSET","display_name":"SMT Settings","module":"smt_mes","primary_key":"SettingKey"}'::jsonb)
) AS tables(table_code, payload)
ON CONFLICT (solution_id, asset_key) DO UPDATE SET
    asset_type = EXCLUDED.asset_type,
    payload = EXCLUDED.payload,
    metadata = platform.industry_solution_assets.metadata || EXCLUDED.metadata,
    updated_at = NOW();

WITH smt_package AS (
    SELECT id FROM platform.industry_solutions
    WHERE industry_key = 'smt_manufacturing' AND solution_key = 'smt_mes_v1' AND version = 1
)
INSERT INTO platform.industry_solution_assets(solution_id, asset_key, asset_type, payload, metadata)
SELECT id, asset_key, asset_type, payload, '{"seed":true,"solution_key":"smt_mes_v1"}'::jsonb
FROM smt_package
CROSS JOIN (VALUES
    ('smt-mes-module', 'module', '{"module_key":"smt_mes","display_name":"SMT Manufacturing Execution"}'::jsonb),
    ('erp-module', 'module', '{"module_key":"erp","display_name":"ERP"}'::jsonb),
    ('inventory-module', 'module', '{"module_key":"inventory","display_name":"Inventory"}'::jsonb),
    ('manufacturing-module', 'module', '{"module_key":"manufacturing","display_name":"Manufacturing"}'::jsonb),
    ('smt-workspace', 'runtime_entity', '{"entity_kind":"ui_workspace","workspace_key":"smt","module":"smt_mes","documents":["smt_workstation","smt_panel_station","smt_pass_station","smt_repair_station","smt_andon","smt_line_board","smt_trace","smt_reports","smt_settings","smt_edge","smt_lines","smt_machines","smt_operations","smt_routings","smt_products","smt_customer_part_maps","smt_feeder_setups","smt_reels","smt_work_orders","smt_requisitions","smt_loading_records","smt_verification_scans","smt_low_material_alerts","smt_units","smt_panels","smt_defect_codes","smt_unit_defects","smt_inspections","smt_andon_calls","smt_downtimes"]}'::jsonb)
) AS assets(asset_key, asset_type, payload)
ON CONFLICT (solution_id, asset_key) DO UPDATE SET
    asset_type = EXCLUDED.asset_type,
    payload = EXCLUDED.payload,
    metadata = platform.industry_solution_assets.metadata || EXCLUDED.metadata,
    updated_at = NOW();

-- Capability package manifest + marketplace listing (mirrors the erpnext sample shape).
WITH smt_capability_package AS (
    INSERT INTO platform.capability_packages(
        package_key, package_type, version_key, display_name, status, artifact_hash, manifest, metadata
    )
    VALUES (
        'smt_mes_v1',
        'industry_solution',
        'v1',
        'SMT MES',
        'published',
        'smt-mes-v1',
        '{
            "manifest_version": "meta-org.industry-solution.v1",
            "industry_key": "smt_manufacturing",
            "package_key": "smt_mes_v1",
            "package_version": "v1",
            "dependencies": ["tenant_database.provisioning", "platform.runtime_operations", "erp.smt_mes_code_tables"],
            "quality_gates": ["smt_mes_closed_loop_smoke"],
            "verification_scenarios": ["smt_mes_work_order_to_finish_smoke"],
            "assets": [
                {"asset_key": "database_asset.smt_code_tables", "asset_type": "database_asset", "version": "v1", "source": "smt_mes_solution", "owner": "platform", "risk_level": "low", "payload": {"tables": ["MLIN","LIN1","MMAC","MAC1","MFSB","FSB1","MOPR","MROU","ROU1","MPRD","PRD1","MCPM","MREL","REL1","MSWO","SWO1","MRQN","RQN1","MLOD","LOD1","MVER","MLWA","MUNT","UNT1","MPNL","PNL1","MDFC","MUDF","MINS","INS1","MAND","AND1","MDWN"], "module": "smt_mes"}},
                {"asset_key": "module.smt_mes", "asset_type": "module", "version": "v1", "source": "smt_mes_solution", "owner": "platform", "risk_level": "low", "payload": {"module_key": "smt_mes", "display_name": "SMT Manufacturing Execution"}},
                {"asset_key": "runtime_operation.smt_loading", "asset_type": "runtime_operation", "version": "v1", "source": "smt_mes_solution", "owner": "platform", "risk_level": "medium", "payload": {"operation_key": "smt.mlod.load", "domain": "SMT MES", "title": "smt.action.load", "method": "POST", "path": "/erp/MLOD/{key}/actions/load", "operation_kind": "contextual", "danger_level": "medium", "result_view": "detail", "assistant_eligible": false, "action_type": "erp.action", "workspace": {"module": "smt_mes", "document_id": "loading_record", "kind": "document", "table_code": "MLOD", "action": "load"}}},
                {"asset_key": "runtime_operation.smt_pass", "asset_type": "runtime_operation", "version": "v1", "source": "smt_mes_solution", "owner": "platform", "risk_level": "low", "payload": {"operation_key": "smt.munt.pass", "domain": "SMT MES", "title": "smt.action.pass", "method": "POST", "path": "/erp/MUNT/{key}/actions/pass", "operation_kind": "contextual", "danger_level": "low", "result_view": "detail", "assistant_eligible": true, "action_type": "erp.action", "workspace": {"module": "smt_mes", "document_id": "unit", "kind": "document", "table_code": "MUNT", "action": "pass"}}},
                {"asset_key": "ui_workspace.smt", "asset_type": "ui_workspace", "version": "v1", "source": "smt_mes_solution", "owner": "platform", "risk_level": "low", "payload": {"workspace_key": "smt", "module": "smt_mes", "documents": ["smt_workstation","smt_pass_station","smt_repair_station","smt_andon","smt_line_board","smt_trace","smt_reports"], "primary_operation": "smt.mlod.load"}}
            ]
        }'::jsonb,
        '{
            "seed": true,
            "sample_industry_solution": true,
            "sample_scope": "local_development",
            "tenant_database_template": {
                "deployment_mode": "dedicated_database",
                "schema_name": "public",
                "database_name_prefix": "meta_org_"
            },
            "notes": "SMT MES industry solution for electronics assembly shop-floor execution. Physical code tables are created by migration 036 in every tenant database."
        }'::jsonb
    )
    ON CONFLICT (package_key, package_type, version_key) DO UPDATE SET
        display_name = EXCLUDED.display_name,
        status = EXCLUDED.status,
        artifact_hash = EXCLUDED.artifact_hash,
        manifest = EXCLUDED.manifest,
        metadata = platform.capability_packages.metadata || EXCLUDED.metadata,
        updated_at = NOW()
    RETURNING id
)
INSERT INTO platform.marketplace_listings(
    package_id, listing_type, status, visibility,
    pricing_policy, license_policy, settlement_policy, metadata
)
SELECT
    id,
    'industry_solution',
    'published',
    'platform',
    '{"billing_mode":"commercial"}'::jsonb,
    '{"license_scope":"commercial","private_deployment_allowed":true}'::jsonb,
    '{"settlement_mode":"none"}'::jsonb,
    '{"seed":true,"sample_industry_solution":true,"catalog":"saas_management_samples","package_key":"smt_mes_v1"}'::jsonb
FROM smt_capability_package
ON CONFLICT (package_id, listing_type) DO UPDATE SET
    status = EXCLUDED.status,
    visibility = EXCLUDED.visibility,
    pricing_policy = EXCLUDED.pricing_policy,
    license_policy = EXCLUDED.license_policy,
    settlement_policy = EXCLUDED.settlement_policy,
    metadata = platform.marketplace_listings.metadata || EXCLUDED.metadata,
    updated_at = NOW();

-- ---------------------------------------------------------------------------
-- 3) Runtime operations (API workbench / assistant-eligible surface).
-- ---------------------------------------------------------------------------
INSERT INTO platform.runtime_operations (
    operation_key, domain, title, method, path, auth, path_params, body_template,
    operation_kind, danger_level, result_view, assistant_eligible, action_type, metadata
)
SELECT 'smt.' || lower(code) || '.' || action, 'SMT MES', 'smt.action.' || action,
    'POST', '/erp/' || code || '/{key}/actions/' || action, TRUE,
    '[{"name":"key","label":"smt.field.key"}]'::jsonb,
    body, 'contextual', danger, 'detail', eligible, 'erp.action',
    jsonb_build_object('source', 'smt_mes_solution', 'workspace',
        jsonb_build_object('module', 'smt_mes', 'document_id', document_id,
            'table_code', code, 'action', action),
        'title_i18n', title_i18n)
FROM (VALUES
    ('MREL','receive','reel','medium',FALSE,'{"data":{"part_no":"","qty":0,"reel_count":1,"lot_no":"","supplier":"","owner_type":"SELF","customer_code":""}}'::jsonb,'{"zh":"收料入库","en":"Receive reels"}'),
    ('MREL','issue','reel','medium',FALSE,'{"data":{"work_order":"","ref":""}}'::jsonb,'{"zh":"发料","en":"Issue reel"}'),
    ('MREL','return','reel','medium',FALSE,'{"data":{"qty":0,"remark":""}}'::jsonb,'{"zh":"退库","en":"Return reel"}'),
    ('MREL','empty','reel','low',FALSE,'{"data":{"qty":0}}'::jsonb,'{"zh":"料盘用完","en":"Reel empty"}'),
    ('MREL','scrap','reel','high',FALSE,'{"data":{"reason":""}}'::jsonb,'{"zh":"料盘报废","en":"Scrap reel"}'),
    ('MREL','lost','reel','high',FALSE,'{"data":{"reason":""}}'::jsonb,'{"zh":"盘亏","en":"Mark reel lost"}'),
    ('MREL','return-to-customer','reel','medium',FALSE,'{"data":{}}'::jsonb,'{"zh":"客供料退回客户","en":"Return reel to customer"}'),
    ('MREL','adjust','reel','high',FALSE,'{"data":{"qty":0,"reason":""}}'::jsonb,'{"zh":"库存调整","en":"Adjust reel quantity"}'),
    ('MLOD','load','loading_record','medium',FALSE,'{"data":{"work_order":"","machine_code":"","slot_no":"","reel_code":"","replace":false,"splice":false}}'::jsonb,'{"zh":"上料","en":"Load reel"}'),
    ('MLOD','unload','loading_record','medium',FALSE,'{"data":{"reason":"OTHER","qty_at_unload":0}}'::jsonb,'{"zh":"卸料","en":"Unload reel"}'),
    ('MMAC','bind','machine','medium',FALSE,'{"data":{"work_order":""}}'::jsonb,'{"zh":"绑定工单（换线）","en":"Bind work order (changeover)"}'),
    ('MMAC','verify','machine','low',FALSE,'{"data":{}}'::jsonb,'{"zh":"整机核对","en":"Verify machine"}'),
    ('MMAC','sign-off','machine','medium',FALSE,'{"data":{}}'::jsonb,'{"zh":"第二人签核","en":"Sign off machine"}'),
    ('MMAC','lock-slot','machine','medium',FALSE,'{"data":{"slot_no":"","reason":""}}'::jsonb,'{"zh":"锁定站位","en":"Lock slot"}'),
    ('MMAC','unlock-slot','machine','medium',FALSE,'{"data":{"slot_no":""}}'::jsonb,'{"zh":"站位解锁","en":"Unlock slot"}'),
    ('MSWO','start','work_order','low',FALSE,'{"data":{}}'::jsonb,'{"zh":"工单开工","en":"Start work order"}'),
    ('MSWO','progress','work_order','low',FALSE,'{"data":{"done_qty":0}}'::jsonb,'{"zh":"报工","en":"Report progress"}'),
    ('MSWO','finish','work_order','low',FALSE,'{"data":{}}'::jsonb,'{"zh":"工单完工","en":"Finish work order"}'),
    ('MSWO','close','work_order','medium',FALSE,'{"data":{}}'::jsonb,'{"zh":"工单关闭","en":"Close work order"}'),
    ('MRQN','issue','requisition','medium',FALSE,'{"data":{"part_no":"","reel_code":""}}'::jsonb,'{"zh":"领料发料（扫料盘）","en":"Issue reel to requisition"}'),
    ('MRQN','close','requisition','medium',FALSE,'{"data":{}}'::jsonb,'{"zh":"领料单完成","en":"Close requisition"}'),
    ('MRQN','cancel','requisition','medium',FALSE,'{"data":{}}'::jsonb,'{"zh":"领料单取消","en":"Cancel requisition"}'),
    ('MUNT','pass','unit','low',TRUE,'{"data":{"sn":"","operation_code":"","machine_code":"","source":"MANUAL"}}'::jsonb,'{"zh":"过站（良品）","en":"Pass unit"}'),
    ('MUNT','fail','unit','medium',FALSE,'{"data":{"sn":"","operation_code":"","defects":[{"code":"","location":""}]}}'::jsonb,'{"zh":"过站判不良","en":"Fail unit"}'),
    ('MUNT','repair','unit','medium',FALSE,'{"data":{"defect_key":"","action":"REPAIRED","root_cause":"","remark":""}}'::jsonb,'{"zh":"维修完成","en":"Complete repair"}'),
    ('MUNT','scrap','unit','high',FALSE,'{"data":{"reason":""}}'::jsonb,'{"zh":"报废","en":"Scrap unit"}'),
    ('MPNL','bind','panel','medium',FALSE,'{"data":{"work_order":"","panel_code":"","sns":[]}}'::jsonb,'{"zh":"拼板绑定","en":"Bind panel"}'),
    ('MPNL','split','panel','medium',FALSE,'{"data":{}}'::jsonb,'{"zh":"拼板分板","en":"Split panel"}'),
    ('MINS','record','inspection','medium',FALSE,'{"data":{"insp_type":"FAI","work_order":"","sample_sn":"","result":"PASS","items":[]}}'::jsonb,'{"zh":"首件 / 巡检登记","en":"Record inspection"}'),
    ('MAND','raise','andon_call','low',TRUE,'{"data":{"line_code":"","machine_code":"","work_order":"","call_type":"QUALITY","description":""}}'::jsonb,'{"zh":"Andon 呼叫","en":"Raise andon call"}'),
    ('MAND','ack','andon_call','low',TRUE,'{"data":{}}'::jsonb,'{"zh":"Andon 响应","en":"Acknowledge andon"}'),
    ('MAND','close','andon_call','low',TRUE,'{"data":{"cause":"","solution":""}}'::jsonb,'{"zh":"Andon 关闭","en":"Close andon"}'),
    ('MDWN','start','downtime','low',TRUE,'{"data":{"machine_code":"","line_code":"","reason_type":"OTHER","reason":""}}'::jsonb,'{"zh":"开始停机","en":"Start downtime"}'),
    ('MDWN','end','downtime','low',TRUE,'{"data":{}}'::jsonb,'{"zh":"结束停机","en":"End downtime"}')
) AS actions(code, action, document_id, danger, eligible, body, title_i18n)
ON CONFLICT (operation_key) DO UPDATE SET
    domain = EXCLUDED.domain,
    title = EXCLUDED.title,
    method = EXCLUDED.method,
    path = EXCLUDED.path,
    path_params = EXCLUDED.path_params,
    body_template = EXCLUDED.body_template,
    operation_kind = EXCLUDED.operation_kind,
    danger_level = EXCLUDED.danger_level,
    result_view = EXCLUDED.result_view,
    assistant_eligible = EXCLUDED.assistant_eligible,
    action_type = EXCLUDED.action_type,
    metadata = platform.runtime_operations.metadata || EXCLUDED.metadata,
    updated_at = NOW();

INSERT INTO platform.runtime_operations (
    operation_key, domain, title, method, path, auth, path_params, query_params,
    operation_kind, danger_level, result_view, assistant_eligible, action_type, metadata
)
SELECT 'smt.' || key, 'SMT MES', 'smt.operation.' || key,
    method, path, TRUE,
    path_params, query_params,
    'direct', danger, result_view, eligible, 'smt.read',
    jsonb_build_object('source', 'smt_mes_solution', 'title_i18n', title_i18n)
FROM (VALUES
    ('machine.context','GET','/smt/machines/{code}/context','[{"name":"code","label":"smt.field.machineCode"}]'::jsonb,'[]'::jsonb,'low','detail',TRUE,'{"zh":"机台站位状态","en":"Machine station context"}'),
    ('board','GET','/smt/board','[]'::jsonb,'[{"name":"line_code","label":"smt.field.lineCode"},{"name":"date","label":"smt.field.date"}]'::jsonb,'low','summary',TRUE,'{"zh":"产线看板","en":"Line board"}'),
    ('work_order.progress','GET','/smt/work-orders/{key}/progress','[{"name":"key","label":"smt.field.workOrder"}]'::jsonb,'[]'::jsonb,'low','detail',TRUE,'{"zh":"工单工序进度","en":"Work order step progress"}'),
    ('trace.sn','GET','/smt/trace/sn/{sn}','[{"name":"sn","label":"smt.field.sn"}]'::jsonb,'[]'::jsonb,'low','detail',TRUE,'{"zh":"SN 追溯","en":"Trace by SN"}'),
    ('trace.reel_units','GET','/smt/trace/reels/{uid}/units','[{"name":"uid","label":"smt.field.reelCode"}]'::jsonb,'[]'::jsonb,'low','list',TRUE,'{"zh":"料盘受影响 SN","en":"Units by reel"}'),
    ('reports.production','GET','/smt/reports/production','[]'::jsonb,'[{"name":"from","label":"smt.field.from"},{"name":"to","label":"smt.field.to"},{"name":"line_code","label":"smt.field.lineCode"}]'::jsonb,'low','summary',TRUE,'{"zh":"生产质量报表","en":"Production quality report"}'),
    ('reports.andon','GET','/smt/reports/andon','[]'::jsonb,'[{"name":"from","label":"smt.field.from"},{"name":"to","label":"smt.field.to"}]'::jsonb,'low','summary',TRUE,'{"zh":"Andon 报表","en":"Andon report"}'),
    ('reports.downtime','GET','/smt/reports/downtime','[]'::jsonb,'[{"name":"from","label":"smt.field.from"},{"name":"to","label":"smt.field.to"}]'::jsonb,'low','summary',TRUE,'{"zh":"停机与可用率","en":"Downtime and availability"}'),
    ('settings.list','GET','/smt/settings','[]'::jsonb,'[]'::jsonb,'low','detail',FALSE,'{"zh":"SMT 参数","en":"SMT settings"}'),
    ('settings.update','PATCH','/smt/settings','[]'::jsonb,'[]'::jsonb,'medium','detail',FALSE,'{"zh":"更新 SMT 参数","en":"Update SMT settings"}'),
    ('edge.list','GET','/smt/edges','[]'::jsonb,'[]'::jsonb,'low','list',FALSE,'{"zh":"边缘实例","en":"Edge instances"}'),
    ('edge.create','POST','/smt/edges','[]'::jsonb,'[]'::jsonb,'medium','detail',FALSE,'{"zh":"新建边缘实例","en":"Create edge instance"}'),
    ('edge.sync','POST','/smt/edges/{key}/sync','[{"name":"key","label":"smt.field.instanceKey"}]'::jsonb,'[]'::jsonb,'medium','detail',FALSE,'{"zh":"下发主数据 / 工单","en":"Push master data and work orders"}'),
    ('edge.events','GET','/smt/edges/{key}/events','[{"name":"key","label":"smt.field.instanceKey"}]'::jsonb,'[{"name":"status","label":"smt.field.status"},{"name":"limit","label":"smt.field.limit"}]'::jsonb,'low','list',FALSE,'{"zh":"边缘事件日志","en":"Edge event log"}')
) AS operations(key, method, path, path_params, query_params, danger, result_view, eligible, title_i18n)
ON CONFLICT (operation_key) DO UPDATE SET
    domain = EXCLUDED.domain,
    title = EXCLUDED.title,
    method = EXCLUDED.method,
    path = EXCLUDED.path,
    path_params = EXCLUDED.path_params,
    query_params = EXCLUDED.query_params,
    danger_level = EXCLUDED.danger_level,
    result_view = EXCLUDED.result_view,
    assistant_eligible = EXCLUDED.assistant_eligible,
    action_type = EXCLUDED.action_type,
    metadata = platform.runtime_operations.metadata || EXCLUDED.metadata,
    updated_at = NOW();
