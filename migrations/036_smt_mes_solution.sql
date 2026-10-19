-- 036_smt_mes_solution.sql
-- SMT MES industry solution baseline (shared by the platform database and tenant databases).
--
-- Creates the SMT shop-floor code tables (line, machine/slot, feeder setup, operation,
-- routing, reel, loading record, verification scan, low-material alert, unit, panel,
-- defect code, unit defect, inspection, andon call, downtime), plus the tenant-local
-- SMT settings table and the SMT edge integration inbox/sync tables.
--
-- The physical code tables follow the MREG registry + create_erp_master/create_erp_child
-- convention from 001_erp_code_baseline.sql. This file is included by
-- migrations/tenant/011_smt_mes_solution.sql so tenant databases receive the same tables.
--
-- IMPORTANT: keep this file idempotent (IF NOT EXISTS / ON CONFLICT); migration checksum
-- governance forbids editing already-applied migrations without an accept-checksum-drift
-- repair. Future SMT schema changes use a new numbered migration.

CREATE SCHEMA IF NOT EXISTS platform;

-- ---------------------------------------------------------------------------
-- 1) SMT code-table registration (MREG) + physical master/detail tables.
-- ---------------------------------------------------------------------------
DO $$
DECLARE
    rec RECORD;
BEGIN
    FOR rec IN
        SELECT * FROM (VALUES
            ('MLIN','SMT Lines','smt_mes','LineCode','master',''),
            ('LIN1','SMT Line - Machines','smt_mes','LineCode','child','MLIN'),
            ('MMAC','SMT Machines','smt_mes','MachineCode','master',''),
            ('MAC1','SMT Machine - Slots','smt_mes','MachineCode','child','MMAC'),
            ('MFSB','Feeder Setups','smt_mes','SetupCode','master',''),
            ('FSB1','Feeder Setup - Rows','smt_mes','SetupCode','child','MFSB'),
            ('MOPR','Operations','smt_mes','OpCode','master',''),
            ('MROU','Routings','smt_mes','RouteCode','master',''),
            ('ROU1','Routing - Steps','smt_mes','RouteCode','child','MROU'),
            ('MPRD','SMT Products','smt_mes','ProductCode','master',''),
            ('PRD1','SMT Product - BOM Rows','smt_mes','ProductCode','child','MPRD'),
            ('MCPM','Customer Part Maps','smt_mes','MapCode','master',''),
            ('MREL','Reels','smt_mes','ReelCode','master',''),
            ('REL1','Reel - Transactions','smt_mes','ReelCode','child','MREL'),
            ('MSWO','SMT Work Orders','smt_mes','WorkOrderCode','master',''),
            ('SWO1','SMT Work Order - Material Summary','smt_mes','WorkOrderCode','child','MSWO'),
            ('MRQN','Requisitions','smt_mes','ReqNo','master',''),
            ('RQN1','Requisition - Items','smt_mes','ReqNo','child','MRQN'),
            ('MLOD','Loading Records','smt_mes','LoadCode','master',''),
            ('LOD1','Loading Record - Slices','smt_mes','LoadCode','child','MLOD'),
            ('MVER','Verification Scans','smt_mes','ScanCode','master',''),
            ('MLWA','Low Material Alerts','smt_mes','AlertCode','master',''),
            ('MUNT','Production Units','smt_mes','SN','master',''),
            ('UNT1','Unit - Events','smt_mes','SN','child','MUNT'),
            ('MPNL','Panels','smt_mes','PanelCode','master',''),
            ('PNL1','Panel - Boards','smt_mes','PanelCode','child','MPNL'),
            ('MDFC','Defect Codes','smt_mes','DefectCode','master',''),
            ('MUDF','Unit Defects','smt_mes','DefectID','master',''),
            ('MINS','Inspections','smt_mes','InspNo','master',''),
            ('INS1','Inspection - Items','smt_mes','InspNo','child','MINS'),
            ('MAND','Andon Calls','smt_mes','CallNo','master',''),
            ('AND1','Andon Call - Timeline','smt_mes','CallNo','child','MAND'),
            ('MDWN','Downtimes','smt_mes','DowntimeCode','master',''),
            ('MSET','SMT Settings','smt_mes','SettingKey','master','')
        ) AS t(code, name, module, pk, kind, parent_code)
    LOOP
        INSERT INTO "MREG"("Code", "Name", "Module", "PrimaryKey", "Kind", "ParentCode")
        VALUES (rec.code, rec.name, rec.module, rec.pk, rec.kind, rec.parent_code)
        ON CONFLICT ("Code") DO UPDATE SET
            "Name" = EXCLUDED."Name",
            "Module" = EXCLUDED."Module",
            "PrimaryKey" = EXCLUDED."PrimaryKey",
            "Kind" = EXCLUDED."Kind",
            "ParentCode" = EXCLUDED."ParentCode",
            "UpdatedAt" = NOW();

        IF rec.kind = 'master' THEN
            PERFORM create_erp_master(rec.code, rec.pk);
        ELSE
            PERFORM create_erp_child(rec.code, rec.parent_code, rec.pk);
        END IF;
    END LOOP;
END $$;

-- Hot-path expression indexes over the JSONB payload (tenant-local tables).
CREATE INDEX IF NOT EXISTS "idx_mrel_part_status" ON "MREL" (("Payload" ->> 'part_no'), ("Payload" ->> 'status'));
CREATE INDEX IF NOT EXISTS "idx_mrel_issued_wo" ON "MREL" (("Payload" ->> 'issued_wo'));
CREATE INDEX IF NOT EXISTS "idx_mrel_lot" ON "MREL" (("Payload" ->> 'lot_no'));
CREATE INDEX IF NOT EXISTS "idx_mmac_line" ON "MMAC" (("Payload" ->> 'line_code'));
CREATE INDEX IF NOT EXISTS "idx_mfsb_product_machine" ON "MFSB" (("Payload" ->> 'product_code'), ("Payload" ->> 'machine_code'));
CREATE INDEX IF NOT EXISTS "idx_mcpm_customer" ON "MCPM" (("Payload" ->> 'customer_code'), ("Payload" ->> 'customer_part_no'));
CREATE INDEX IF NOT EXISTS "idx_mswo_status" ON "MSWO" (("Payload" ->> 'status'));
CREATE INDEX IF NOT EXISTS "idx_mswo_product" ON "MSWO" (("Payload" ->> 'product_code'));
CREATE INDEX IF NOT EXISTS "idx_mrqn_wo" ON "MRQN" (("Payload" ->> 'work_order'));
CREATE INDEX IF NOT EXISTS "idx_mlod_wo" ON "MLOD" (("Payload" ->> 'work_order'));
CREATE INDEX IF NOT EXISTS "idx_mlod_machine_status" ON "MLOD" (("Payload" ->> 'machine_code'), ("Payload" ->> 'status'));
CREATE INDEX IF NOT EXISTS "idx_mlod_reel" ON "MLOD" (("Payload" ->> 'reel_code'));
CREATE INDEX IF NOT EXISTS "idx_mver_created" ON "MVER" ("CreatedAt" DESC);
CREATE INDEX IF NOT EXISTS "idx_mver_result" ON "MVER" (("Payload" ->> 'result'));
CREATE INDEX IF NOT EXISTS "idx_mver_wo" ON "MVER" (("Payload" ->> 'work_order'));
CREATE INDEX IF NOT EXISTS "idx_mlwa_status" ON "MLWA" (("Payload" ->> 'status'));
CREATE INDEX IF NOT EXISTS "idx_munt_wo" ON "MUNT" (("Payload" ->> 'work_order'));
CREATE INDEX IF NOT EXISTS "idx_munt_status" ON "MUNT" (("Payload" ->> 'status'));
CREATE INDEX IF NOT EXISTS "idx_munt_created" ON "MUNT" ("CreatedAt");
CREATE INDEX IF NOT EXISTS "idx_mudf_status_sn" ON "MUDF" (("Payload" ->> 'status'), ("Payload" ->> 'sn'));
CREATE INDEX IF NOT EXISTS "idx_mand_status" ON "MAND" (("Payload" ->> 'status'));
CREATE INDEX IF NOT EXISTS "idx_mand_line" ON "MAND" (("Payload" ->> 'line_code'), ("Payload" ->> 'status'));
CREATE INDEX IF NOT EXISTS "idx_mdwn_machine" ON "MDWN" (("Payload" ->> 'machine_code'), ("Payload" ->> 'started_at'));
CREATE INDEX IF NOT EXISTS "idx_mpnl_wo" ON "MPNL" (("Payload" ->> 'work_order'));

-- ---------------------------------------------------------------------------
-- 2) Tenant-local SMT runtime settings (key/value, mirrors SettingsService).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS smt_settings (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ---------------------------------------------------------------------------
-- 3) SMT edge integration inbox + outbound sync log (tenant-local).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS smt_edge_events (
    id           BIGSERIAL PRIMARY KEY,
    instance_key UUID NOT NULL,
    event_id     BIGINT NOT NULL,
    event_type   TEXT NOT NULL,
    ref          TEXT NOT NULL DEFAULT '',
    payload      JSONB NOT NULL DEFAULT '{}'::jsonb,
    status       TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processed', 'skipped', 'failed')),
    error        TEXT NOT NULL DEFAULT '',
    received_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    UNIQUE (instance_key, event_id)
);

CREATE INDEX IF NOT EXISTS idx_smt_edge_events_status
    ON smt_edge_events(status, received_at);

CREATE TABLE IF NOT EXISTS smt_edge_sync_log (
    id           BIGSERIAL PRIMARY KEY,
    instance_key UUID NOT NULL,
    scope        TEXT NOT NULL,
    status       TEXT NOT NULL DEFAULT 'ok',
    message      TEXT NOT NULL DEFAULT '',
    payload      JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_smt_edge_sync_log_instance
    ON smt_edge_sync_log(instance_key, created_at DESC);

-- ---------------------------------------------------------------------------
-- 4) SaaS module registration for the SMT MES industry solution.
-- ---------------------------------------------------------------------------
INSERT INTO saas_modules(module_key, display_name, category, enabled_default, license_scope, metadata)
VALUES (
    'smt_mes',
    'SMT Manufacturing Execution',
    'industry',
    true,
    'commercial',
    '{"industry_solution":"smt_mes_v1","code_table_only":true}'::jsonb
)
ON CONFLICT (module_key) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    category = EXCLUDED.category,
    enabled_default = EXCLUDED.enabled_default,
    license_scope = EXCLUDED.license_scope,
    metadata = saas_modules.metadata || EXCLUDED.metadata,
    updated_at = NOW();

DO $$
BEGIN
    IF to_regclass('public.saas_plans') IS NOT NULL
       AND to_regclass('public.saas_plan_modules') IS NOT NULL THEN
        INSERT INTO saas_plan_modules(plan_id, module_key)
        SELECT p.id, 'smt_mes'
        FROM saas_plans p
        WHERE p.code = 'foundation'
        ON CONFLICT (plan_id, module_key) DO NOTHING;
    END IF;
END $$;
