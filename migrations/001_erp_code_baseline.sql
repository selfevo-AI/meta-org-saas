-- ERP and industry-solution baseline.
-- Tenant organization is a SaaS platform management object owned by 000.
-- ERP-facing hierarchy starts from tenant departments scoped by organization_id.

-- 001_erp_code_baseline.sql
-- ERP code-table baseline for fresh databases.
--
-- This migration runs after 000_saas_platform_management_baseline.sql and
-- creates the concrete ERP code-table baseline. It is intentionally reserved
-- for industry-solution-specific ERP master/detail table setup.
--
-- IMPORTANT / 重要:
-- Do not insert other migrations into platform.platform_migration_runs from this file. SaaS
-- platform management, AI Gateway, assistant/runtime workbench, governance, and
-- security-kernel platform metadata belong to 000_saas_platform_management_baseline.sql.
-- This ERP baseline may be adjusted for concrete industry solutions by changing
-- ERP code-table registrations and their seeded master/detail data.
--
-- 不要在本文件中写入其他迁移到 platform.platform_migration_runs。SaaS 平台管理、
-- AI Gateway、助手/运行时工作台、治理、安全内核等平台管理元数据归属
-- 000_saas_platform_management_baseline.sql。本 ERP 基线只承载行业解决方案
-- 可以调整的 ERP code-table 注册和初始化数据。

CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE SCHEMA IF NOT EXISTS platform;

CREATE TABLE IF NOT EXISTS "MREG" (
    "Code" VARCHAR(8) PRIMARY KEY,
    "Name" TEXT NOT NULL,
    "Module" TEXT NOT NULL,
    "PrimaryKey" TEXT NOT NULL,
    "Kind" TEXT NOT NULL DEFAULT 'master',
    "ParentCode" VARCHAR(8) NOT NULL DEFAULT '',
    "Metadata" JSONB NOT NULL DEFAULT '{}',
    "CreatedAt" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    "UpdatedAt" TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE OR REPLACE FUNCTION create_erp_master(p_code TEXT, p_pk TEXT)
RETURNS VOID
LANGUAGE plpgsql
AS $$
BEGIN
    EXECUTE FORMAT('CREATE TABLE IF NOT EXISTS %I (
        %I TEXT PRIMARY KEY,
        "Payload" JSONB NOT NULL DEFAULT ''{}'',
        "CreatedAt" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        "UpdatedAt" TIMESTAMPTZ NOT NULL DEFAULT NOW()
    )', p_code, p_pk);
END $$;

CREATE OR REPLACE FUNCTION create_erp_child(p_code TEXT, p_parent_code TEXT, p_parent_key TEXT)
RETURNS VOID
LANGUAGE plpgsql
AS $$
BEGIN
    EXECUTE FORMAT('CREATE TABLE IF NOT EXISTS %I (
        %I TEXT NOT NULL,
        "LineNum" BIGINT NOT NULL,
        "LineStatus" VARCHAR(1) NOT NULL DEFAULT ''O'',
        "Payload" JSONB NOT NULL DEFAULT ''{}'',
        "CreatedAt" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        "UpdatedAt" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        PRIMARY KEY (%I, "LineNum")
    )', p_code, p_parent_key, p_parent_key);
    EXECUTE FORMAT('CREATE INDEX IF NOT EXISTS %I ON %I(%I, "LineNum")',
        'idx_' || lower(p_code) || '_parent', p_code, p_parent_key);
END $$;

DO $$
DECLARE
    rec RECORD;
BEGIN
    FOR rec IN
        SELECT * FROM (VALUES
            ('MACT','G/L Accounts','finance','AcctCode','master',''),
            ('AACT','G/L Account - History','finance','AcctCode','child','MACT'),
            ('MBTD','Journal Vouchers List','finance','BatchNum','master',''),
            ('MBTF','Journal Voucher Entry','finance','BatchNum','master',''),
            ('BTF1','Journal Voucher - Rows','finance','BatchNum','child','MBTF'),
            ('MJDT','Journal Entry','finance','TransId','master',''),
            ('JDT1','Journal Entry - Rows','finance','TransId','child','MJDT'),
            ('MPRC','Cost Center','finance','PrcCode','master',''),
            ('APRC','Cost Center','finance','PrcCode','child','MPRC'),
            ('MGLR','Trial Balance','finance','ReportCode','master',''),
            ('MCRD','Business Partners','partner','CardCode','master',''),
            ('CRD1','Business Partners - Addresses','partner','CardCode','child','MCRD'),
            ('MITM','Items','product','ItemCode','master',''),
            ('ITM1','Items - Prices','product','ItemCode','child','MITM'),
            ('MITW','Items - Warehouse','product','ItemCode','master',''),
            ('ITW1','Item Count Alert','product','ItemCode','child','MITW'),
            ('MBRN','Retail Branches','retail','StoreCode','master',''),
            ('BRN1','Retail Branch - Rows','retail','StoreCode','child','MBRN'),
            ('MTER','Retail Terminals','retail','TerminalCode','master',''),
            ('TER1','Retail Terminal - Rows','retail','TerminalCode','child','MTER'),
            ('MMBR','Retail Members','retail','MemberCode','master',''),
            ('MBR1','Retail Member - Rows','retail','MemberCode','child','MMBR'),
            ('MPRM','Retail Promotions','retail','PromotionCode','master',''),
            ('PRM1','Retail Promotion - Rows','retail','PromotionCode','child','MPRM'),
            ('MPUB','Retail Publishing','retail','PublicationCode','master',''),
            ('PUB1','Retail Publishing - Rows','retail','PublicationCode','child','MPUB'),
            ('MPRJ','Project Codes','product','PrjCode','master',''),
            ('APRJ','Project Codes','product','PrjCode','child','MPRJ'),
            ('MDLN','Delivery','sale','DocEntry','master',''),
            ('DLN1','Delivery - Rows','sale','DocEntry','child','MDLN'),
            ('MDPS','Deposit','sale','DocEntry','master',''),
            ('DPS1','Deposit - Rows','sale','DocEntry','child','MDPS'),
            ('MINV','A/R Invoice','sale','DocEntry','master',''),
            ('INV1','A/R Invoice - Rows','sale','DocEntry','child','MINV'),
            ('MQUT','Sales Quotation','sale','DocEntry','master',''),
            ('QUT1','Sales Quotation - Rows','sale','DocEntry','child','MQUT'),
            ('MRCT','Incoming Payments','sale','DocEntry','master',''),
            ('RCT1','Incoming Payment - Checks','sale','DocEntry','child','MRCT'),
            ('MRDN','Returns','sale','DocEntry','master',''),
            ('RDN1','Returns - Rows','sale','DocEntry','child','MRDN'),
            ('MRDR','Sales Order','sale','DocEntry','master',''),
            ('RDR1','Sales Order - Rows','sale','DocEntry','child','MRDR'),
            ('MRIN','A/R Credit Memo','sale','DocEntry','master',''),
            ('RIN1','A/R Credit Memo - Rows','sale','DocEntry','child','MRIN'),
            ('MPCH','A/P Invoice','purchase','DocEntry','master',''),
            ('PCH1','A/P Invoice - Rows','purchase','DocEntry','child','MPCH'),
            ('MPDN','Goods Receipt PO','purchase','DocEntry','master',''),
            ('PDN1','Goods Receipt PO - Rows','purchase','DocEntry','child','MPDN'),
            ('MPOR','Purchase Order','purchase','DocEntry','master',''),
            ('POR1','Purchase Order - Rows','purchase','DocEntry','child','MPOR'),
            ('MRPC','A/P Credit Memo','purchase','DocEntry','master',''),
            ('RPC1','A/P Credit Memo - Rows','purchase','DocEntry','child','MRPC'),
            ('MRPD','Goods Return','purchase','DocEntry','master',''),
            ('RPD1','Goods Return - Rows','purchase','DocEntry','child','MRPD'),
            ('MIGE','Goods Issue','warehouse','DocEntry','master',''),
            ('IGE1','Goods Issue - Rows','warehouse','DocEntry','child','MIGE'),
            ('MIGN','Goods Receipt','warehouse','DocEntry','master',''),
            ('IGN1','Goods Receipt - Rows','warehouse','DocEntry','child','MIGN'),
            ('MWHS','Warehouses','warehouse','WhsCode','master',''),
            ('AWHS','Warehouses - History','warehouse','WhsCode','child','MWHS'),
            ('MRPS','Retail POS Sale','retail','DocEntry','master',''),
            ('RPS1','Retail POS Sale - Rows','retail','DocEntry','child','MRPS'),
            ('MDRQ','Distribution Request','retail','DocEntry','master',''),
            ('DRQ1','Distribution Request - Rows','retail','DocEntry','child','MDRQ'),
            ('MDSP','Distribution Shipment','retail','DocEntry','master',''),
            ('DSP1','Distribution Shipment - Rows','retail','DocEntry','child','MDSP'),
            ('MDRC','Distribution Receipt','retail','DocEntry','master',''),
            ('DRC1','Distribution Receipt - Rows','retail','DocEntry','child','MDRC'),
            ('MDIF','Distribution Difference','retail','DocEntry','master',''),
            ('DIF1','Distribution Difference - Rows','retail','DocEntry','child','MDIF'),
            ('MSTP','Store Stock Policy','retail','PolicyCode','master',''),
            ('STP1','Store Stock Policy - Rows','retail','PolicyCode','child','MSTP'),
            ('MCNT','Store Inventory Count','retail','DocEntry','master',''),
            ('CNT1','Store Inventory Count - Rows','retail','DocEntry','child','MCNT'),
            ('MSPR','Special Purchase Request','retail','DocEntry','master',''),
            ('SPR1','Special Purchase Request - Rows','retail','DocEntry','child','MSPR'),
            ('MUSR','Users','user','USERID','master',''),
            ('AUSR','Users - History','user','USERID','child','MUSR'),
            ('MREQ','Requirements','project','ReqCode','master',''),
            ('REQ1','Requirement Rows','project','ReqCode','child','MREQ'),
            ('MCST','Cost Records','finance','CostCode','master',''),
            ('CST1','Cost Rows','finance','CostCode','child','MCST'),
            ('MFDB','Feedback Records','project','FeedbackCode','master',''),
            ('FDB1','Feedback Rows','project','FeedbackCode','child','MFDB'),
            ('MORG','Organizations','platform','OrgCode','master',''),
            ('AORG','Organizations - History','platform','OrgCode','child','MORG'),
            ('MDEP','Departments','platform','DeptCode','master',''),
            ('ADEP','Departments - History','platform','DeptCode','child','MDEP'),
            ('MPOS','Positions','platform','PosCode','master',''),
            ('APOS','Positions - History','platform','PosCode','child','MPOS'),
            ('MROL','Roles','platform','RoleCode','master',''),
            ('AROL','Role Permissions','platform','RoleCode','child','MROL'),
            ('MSAS','SaaS Modules','platform','ModuleCode','master',''),
            ('ASAS','SaaS Module Details','platform','ModuleCode','child','MSAS'),
            ('MWFL','Workflows','platform','WorkflowCode','master',''),
            ('WFL1','Workflow Rows','platform','WorkflowCode','child','MWFL'),
            ('MGOV','Governance','platform','GovCode','master',''),
            ('GOV1','Governance Rows','platform','GovCode','child','MGOV'),
            ('MAIG','AI Gateway','platform','AIGCode','master',''),
            ('AIG1','AI Gateway Rows','platform','AIGCode','child','MAIG'),
            ('MAST','Assistant','platform','AssistantCode','master',''),
            ('AST1','Assistant Rows','platform','AssistantCode','child','MAST'),
            ('MTOL','Tool Runtime','platform','ToolCode','master',''),
            ('TOL1','Tool Runtime Rows','platform','ToolCode','child','MTOL'),
            ('MOBS','Observability','platform','ObsCode','master',''),
            ('OBS1','Observability Rows','platform','ObsCode','child','MOBS'),
            ('MRTM','Runtime Configuration','platform','RuntimeCode','master',''),
            ('RTM1','Runtime Configuration Rows','platform','RuntimeCode','child','MRTM')
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

-- ERP action execution ledger. "MACT" is already the G/L account code table,
-- so the action ledger uses "MAEX" / "AEX1" to avoid colliding with accounts.
CREATE TABLE IF NOT EXISTS "MAEX" (
    "ActionID" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "TableCode" TEXT NOT NULL,
    "RecordKey" TEXT NOT NULL,
    "Action" TEXT NOT NULL,
    "Status" TEXT NOT NULL DEFAULT 'running',
    "IdempotencyKey" TEXT NOT NULL,
    "ActorID" UUID,
    "ActorType" TEXT NOT NULL DEFAULT '',
    "ToolExecutionID" UUID,
    "AssistantSessionID" UUID,
    "Source" TEXT NOT NULL DEFAULT '',
    "FailureCode" TEXT NOT NULL DEFAULT '',
    "FailureMessage" TEXT NOT NULL DEFAULT '',
    "Payload" JSONB NOT NULL DEFAULT '{}'::jsonb,
    "StartedAt" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    "CompletedAt" TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_maex_idempotency_key
    ON "MAEX" ("IdempotencyKey");

CREATE INDEX IF NOT EXISTS idx_maex_source_record
    ON "MAEX" ("TableCode", "RecordKey", "Action", "Status");

CREATE TABLE IF NOT EXISTS "AEX1" (
    "ActionID" UUID NOT NULL REFERENCES "MAEX"("ActionID") ON DELETE CASCADE,
    "LineNum" BIGINT NOT NULL,
    "GeneratedTableCode" TEXT NOT NULL,
    "GeneratedKey" TEXT NOT NULL,
    "RelationType" TEXT NOT NULL DEFAULT 'created',
    "Payload" JSONB NOT NULL DEFAULT '{}'::jsonb,
    "CreatedAt" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY ("ActionID", "LineNum")
);

CREATE INDEX IF NOT EXISTS idx_aex1_generated_record
    ON "AEX1" ("GeneratedTableCode", "GeneratedKey");

-- -----------------------------------------------------------------------------
-- Folded ERP-strong historical migrations
-- -----------------------------------------------------------------------------
-- ERP-strong means industry business baseline tables/functions for project,
-- finance, costing, inventory, procurement, sales, ERP master/detail registry,
-- and posting/idempotency behavior. SaaS management-platform capabilities that
-- create and govern industry solutions remain in 000.

-- -----------------------------------------------------------------------------
-- Folded from ERP-strong historical migration: 013_project_lifecycle.sql
-- -----------------------------------------------------------------------------

-- 013_project_lifecycle.sql

CREATE TABLE IF NOT EXISTS requirements (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title            TEXT NOT NULL,
    description      TEXT NOT NULL DEFAULT '',
    source           TEXT NOT NULL DEFAULT 'manual',
    status           TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'analyzed', 'approved', 'converted', 'rejected', 'archived')),
    priority         TEXT NOT NULL DEFAULT 'medium'
        CHECK (priority IN ('low', 'medium', 'high', 'critical')),
    risk_level       TEXT NOT NULL DEFAULT 'low'
        CHECK (risk_level IN ('low', 'medium', 'high', 'critical')),
    required_level   TEXT NOT NULL DEFAULT 'L1'
        CHECK (required_level IN ('L1', 'L2', 'L3', 'L4')),
    organization_id  UUID REFERENCES organizations(id) ON DELETE SET NULL,
    department_id    UUID REFERENCES departments(id) ON DELETE SET NULL,
    created_by_id    UUID,
    created_by_type  TEXT NOT NULL DEFAULT '',
    analysis         JSONB NOT NULL DEFAULT '{}',
    metadata         JSONB NOT NULL DEFAULT '{}',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS projects (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    requirement_id   UUID REFERENCES requirements(id) ON DELETE SET NULL,
    organization_id  UUID REFERENCES organizations(id) ON DELETE SET NULL,
    department_id    UUID REFERENCES departments(id) ON DELETE SET NULL,
    name             TEXT NOT NULL,
    description      TEXT NOT NULL DEFAULT '',
    status           TEXT NOT NULL DEFAULT 'planning'
        CHECK (status IN ('planning', 'active', 'paused', 'delivering', 'completed', 'closed', 'cancelled')),
    priority         TEXT NOT NULL DEFAULT 'medium'
        CHECK (priority IN ('low', 'medium', 'high', 'critical')),
    risk_level       TEXT NOT NULL DEFAULT 'low'
        CHECK (risk_level IN ('low', 'medium', 'high', 'critical')),
    required_level   TEXT NOT NULL DEFAULT 'L1'
        CHECK (required_level IN ('L1', 'L2', 'L3', 'L4')),
    budget_amount    NUMERIC(14,2) NOT NULL DEFAULT 0,
    metadata         JSONB NOT NULL DEFAULT '{}',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS project_members (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id         UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    actor_id           UUID NOT NULL,
    actor_type         TEXT NOT NULL
        CHECK (actor_type IN ('internal_human', 'external_human', 'internal_agent', 'external_agent')),
    role               TEXT NOT NULL DEFAULT 'contributor',
    title              TEXT NOT NULL DEFAULT '',
    allocation_percent NUMERIC(5,2) NOT NULL DEFAULT 100,
    cost_rate          NUMERIC(14,2) NOT NULL DEFAULT 0,
    permission_level   TEXT NOT NULL DEFAULT 'L1'
        CHECK (permission_level IN ('L1', 'L2', 'L3', 'L4')),
    capabilities       JSONB NOT NULL DEFAULT '[]',
    status             TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'inactive', 'archived')),
    metadata           JSONB NOT NULL DEFAULT '{}',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS project_workflows (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id           UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    workflow_id          UUID NOT NULL REFERENCES workflow_instances(id) ON DELETE CASCADE,
    workflow_template_id UUID REFERENCES workflow_templates(id) ON DELETE SET NULL,
    purpose              TEXT NOT NULL DEFAULT 'delivery',
    status               TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'paused', 'completed', 'cancelled')),
    metadata             JSONB NOT NULL DEFAULT '{}',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS deliverables (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id          UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name                TEXT NOT NULL,
    deliverable_type    TEXT NOT NULL DEFAULT 'artifact',
    uri                 TEXT NOT NULL DEFAULT '',
    version             TEXT NOT NULL DEFAULT '1.0',
    status              TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'submitted', 'accepted', 'rejected', 'archived')),
    submitted_by_id     UUID,
    submitted_by_type   TEXT NOT NULL DEFAULT '',
    accepted_by_id      UUID,
    accepted_by_type    TEXT NOT NULL DEFAULT '',
    evidence            JSONB NOT NULL DEFAULT '{}',
    metadata            JSONB NOT NULL DEFAULT '{}',
    submitted_at        TIMESTAMPTZ,
    accepted_at         TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS project_cost_entries (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id   UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    source_type  TEXT NOT NULL DEFAULT 'manual',
    source_id    UUID,
    actor_id     UUID,
    actor_type   TEXT NOT NULL DEFAULT '',
    amount       NUMERIC(14,2) NOT NULL DEFAULT 0,
    currency     TEXT NOT NULL DEFAULT 'CNY',
    occurred_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    description  TEXT NOT NULL DEFAULT '',
    metadata     JSONB NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS project_evaluations (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id          UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    actor_id            UUID,
    actor_type          TEXT NOT NULL DEFAULT '',
    capability_id       UUID REFERENCES capabilities(id) ON DELETE SET NULL,
    evaluator_id        UUID,
    evaluator_type      TEXT NOT NULL DEFAULT 'internal_human',
    quality_score       DOUBLE PRECISION NOT NULL DEFAULT 0.5,
    delivery_score      DOUBLE PRECISION NOT NULL DEFAULT 0.5,
    cost_score          DOUBLE PRECISION NOT NULL DEFAULT 0.5,
    collaboration_score DOUBLE PRECISION NOT NULL DEFAULT 0.5,
    overall_score       DOUBLE PRECISION NOT NULL DEFAULT 0.5,
    conclusion          TEXT NOT NULL DEFAULT '',
    evidence            JSONB NOT NULL DEFAULT '{}',
    metadata            JSONB NOT NULL DEFAULT '{}',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE project_members
    ADD COLUMN IF NOT EXISTS position_id UUID REFERENCES positions(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS position_assignment_id UUID REFERENCES position_assignments(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_project_members_position ON project_members(position_id, position_assignment_id);

CREATE INDEX IF NOT EXISTS idx_requirements_status ON requirements(status, priority, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_requirements_org_department ON requirements(organization_id, department_id);
CREATE INDEX IF NOT EXISTS idx_projects_status ON projects(status, priority, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_projects_requirement ON projects(requirement_id);
CREATE INDEX IF NOT EXISTS idx_projects_org_department ON projects(organization_id, department_id);
CREATE INDEX IF NOT EXISTS idx_project_members_project ON project_members(project_id, status);
CREATE INDEX IF NOT EXISTS idx_project_members_actor ON project_members(actor_id, actor_type);
CREATE INDEX IF NOT EXISTS idx_project_workflows_project ON project_workflows(project_id, status);
CREATE INDEX IF NOT EXISTS idx_deliverables_project ON deliverables(project_id, status);
CREATE INDEX IF NOT EXISTS idx_project_cost_entries_project ON project_cost_entries(project_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_project_evaluations_project ON project_evaluations(project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_project_evaluations_actor ON project_evaluations(actor_id, actor_type, created_at DESC);


-- -----------------------------------------------------------------------------
-- Folded from ERP-strong historical migration: 014_requirement_documents_workflow_analysis.sql
-- -----------------------------------------------------------------------------

-- 014_requirement_documents_workflow_analysis.sql

CREATE TABLE IF NOT EXISTS requirement_documents (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    requirement_id    UUID NOT NULL REFERENCES requirements(id) ON DELETE CASCADE,
    file_name         TEXT NOT NULL,
    content_type      TEXT NOT NULL DEFAULT 'application/octet-stream',
    size_bytes        BIGINT NOT NULL DEFAULT 0,
    uploaded_by_id    UUID,
    uploaded_by_type  TEXT NOT NULL DEFAULT '',
    content           BYTEA NOT NULL,
    metadata          JSONB NOT NULL DEFAULT '{}',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS requirement_analysis_workflows (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    requirement_id        UUID NOT NULL REFERENCES requirements(id) ON DELETE CASCADE,
    workflow_id           UUID NOT NULL REFERENCES workflow_instances(id) ON DELETE CASCADE,
    workflow_template_id  UUID NOT NULL REFERENCES workflow_templates(id) ON DELETE CASCADE,
    status               TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'paused', 'completed', 'failed', 'cancelled')),
    analysis_result      JSONB NOT NULL DEFAULT '{}',
    metadata             JSONB NOT NULL DEFAULT '{}',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (requirement_id, workflow_id)
);

CREATE INDEX IF NOT EXISTS idx_requirement_documents_requirement ON requirement_documents(requirement_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_requirement_analysis_workflows_requirement ON requirement_analysis_workflows(requirement_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_requirement_analysis_workflows_workflow ON requirement_analysis_workflows(workflow_id);


-- -----------------------------------------------------------------------------
-- Folded from ERP-strong historical migration: 018_finance_exports.sql
-- -----------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS finance_adapters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    endpoint_url TEXT NOT NULL,
    auth_type TEXT NOT NULL DEFAULT 'hmac' CHECK (auth_type IN ('hmac', 'bearer')),
    encrypted_secret TEXT NOT NULL,
    masked_secret TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled', 'error')),
    timeout_ms INT NOT NULL DEFAULT 30000,
    retry_count INT NOT NULL DEFAULT 3,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS finance_export_batches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    adapter_id UUID NOT NULL REFERENCES finance_adapters(id) ON DELETE RESTRICT,
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'ready', 'exporting', 'exported', 'acknowledged', 'posted', 'reconciled', 'failed', 'cancelled')),
    currency TEXT NOT NULL DEFAULT 'CNY',
    total_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    external_batch_id TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    idempotency_key TEXT NOT NULL UNIQUE,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    submitted_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS finance_export_lines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id UUID NOT NULL REFERENCES finance_export_batches(id) ON DELETE CASCADE,
    usage_ledger_id UUID,
    project_cost_entry_id UUID REFERENCES project_cost_entries(id) ON DELETE SET NULL,
    organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
    department_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    project_id UUID REFERENCES projects(id) ON DELETE SET NULL,
    provider_id UUID,
    model_id UUID,
    amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'CNY',
    external_line_id TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'ready',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS finance_webhook_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    adapter_id UUID NOT NULL REFERENCES finance_adapters(id) ON DELETE RESTRICT,
    batch_id UUID REFERENCES finance_export_batches(id) ON DELETE SET NULL,
    event_type TEXT NOT NULL,
    signature_valid BOOLEAN NOT NULL DEFAULT FALSE,
    payload JSONB NOT NULL DEFAULT '{}',
    processed BOOLEAN NOT NULL DEFAULT FALSE,
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_finance_batches_status ON finance_export_batches(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_finance_lines_batch ON finance_export_lines(batch_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_finance_lines_usage_ledger ON finance_export_lines(usage_ledger_id) WHERE usage_ledger_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_finance_webhooks_adapter ON finance_webhook_events(adapter_id, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_project_cost_entries_ai_usage_source
    ON project_cost_entries(source_type, source_id)
    WHERE source_type = 'ai_usage' AND source_id IS NOT NULL;

-- -----------------------------------------------------------------------------
-- Folded from ERP-strong historical migration: 019_costing_framework.sql
-- -----------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS currencies (
    code TEXT PRIMARY KEY,
    name TEXT NOT NULL DEFAULT '',
    currency_type TEXT NOT NULL DEFAULT 'fiat'
        CHECK (currency_type IN ('fiat', 'virtual')),
    symbol TEXT NOT NULL DEFAULT '',
    precision_digits INT NOT NULL DEFAULT 2,
    chain_id TEXT NOT NULL DEFAULT '',
    contract_address TEXT NOT NULL DEFAULT '',
    external_source TEXT NOT NULL DEFAULT '',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO currencies (code, name, currency_type, symbol, precision_digits)
VALUES
    ('CNY', 'Chinese Yuan', 'fiat', '¥', 2),
    ('USD', 'US Dollar', 'fiat', '$', 2)
ON CONFLICT (code) DO NOTHING;

CREATE TABLE IF NOT EXISTS exchange_rate_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    from_currency TEXT NOT NULL REFERENCES currencies(code) ON DELETE RESTRICT,
    to_currency TEXT NOT NULL REFERENCES currencies(code) ON DELETE RESTRICT,
    rate NUMERIC(24,12) NOT NULL CHECK (rate > 0),
    source TEXT NOT NULL DEFAULT 'manual'
        CHECK (source IN ('manual', 'external')),
    provider TEXT NOT NULL DEFAULT '',
    external_rate_id TEXT NOT NULL DEFAULT '',
    effective_from TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    effective_to TIMESTAMPTZ,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (from_currency <> to_currency)
);

CREATE INDEX IF NOT EXISTS idx_exchange_rates_pair_effective
    ON exchange_rate_versions(from_currency, to_currency, effective_from DESC);

CREATE TABLE IF NOT EXISTS cost_rate_cards (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_type TEXT NOT NULL
        CHECK (subject_type IN ('human', 'external_human', 'agent', 'resource', 'capability', 'tool')),
    subject_id UUID,
    scope_type TEXT NOT NULL DEFAULT '',
    scope_id UUID,
    rate_type TEXT NOT NULL DEFAULT 'fixed'
        CHECK (rate_type IN ('hourly', 'daily', 'monthly', 'token', 'unit', 'fixed')),
    amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    currency TEXT NOT NULL REFERENCES currencies(code) ON DELETE RESTRICT,
    base_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    base_currency TEXT NOT NULL DEFAULT 'CNY' REFERENCES currencies(code) ON DELETE RESTRICT,
    exchange_rate_version_id UUID REFERENCES exchange_rate_versions(id) ON DELETE SET NULL,
    effective_from TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    effective_to TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'disabled')),
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_cost_rate_cards_subject
    ON cost_rate_cards(subject_type, subject_id, effective_from DESC);

CREATE TABLE IF NOT EXISTS cost_budgets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_type TEXT NOT NULL
        CHECK (scope_type IN ('organization', 'department', 'requirement', 'project', 'capability', 'workflow', 'task')),
    scope_id UUID,
    amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    currency TEXT NOT NULL REFERENCES currencies(code) ON DELETE RESTRICT,
    base_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    base_currency TEXT NOT NULL DEFAULT 'CNY' REFERENCES currencies(code) ON DELETE RESTRICT,
    exchange_rate_version_id UUID REFERENCES exchange_rate_versions(id) ON DELETE SET NULL,
    period_start TIMESTAMPTZ,
    period_end TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'closed', 'cancelled')),
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_cost_budgets_scope
    ON cost_budgets(scope_type, scope_id, status);

CREATE TABLE IF NOT EXISTS cost_ledger_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ledger_type TEXT NOT NULL DEFAULT 'actual'
        CHECK (ledger_type IN ('actual', 'estimate', 'budget', 'adjustment')),
    cost_category TEXT NOT NULL
        CHECK (cost_category IN ('human', 'resource', 'agent', 'model_token', 'capability', 'tool', 'finance', 'adjustment', 'manual')),
    source_type TEXT NOT NULL DEFAULT 'manual',
    source_id UUID,
    organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
    department_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    requirement_id UUID REFERENCES requirements(id) ON DELETE SET NULL,
    project_id UUID REFERENCES projects(id) ON DELETE SET NULL,
    workflow_id UUID REFERENCES workflow_instances(id) ON DELETE SET NULL,
    task_id UUID REFERENCES tasks(id) ON DELETE SET NULL,
    capability_id UUID REFERENCES capabilities(id) ON DELETE SET NULL,
    actor_id UUID,
    actor_type TEXT NOT NULL DEFAULT '',
    resource_type TEXT NOT NULL DEFAULT '',
    amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    currency TEXT NOT NULL REFERENCES currencies(code) ON DELETE RESTRICT,
    base_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    base_currency TEXT NOT NULL DEFAULT 'CNY' REFERENCES currencies(code) ON DELETE RESTRICT,
    exchange_rate_version_id UUID REFERENCES exchange_rate_versions(id) ON DELETE SET NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status TEXT NOT NULL DEFAULT 'posted'
        CHECK (status IN ('draft', 'posted', 'exported', 'reconciled', 'void')),
    finance_export_line_id UUID REFERENCES finance_export_lines(id) ON DELETE SET NULL,
    description TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_cost_ledger_scope
    ON cost_ledger_entries(project_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_cost_ledger_source
    ON cost_ledger_entries(source_type, source_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_cost_ledger_unique_source_actual
    ON cost_ledger_entries(source_type, source_id, ledger_type)
    WHERE source_id IS NOT NULL AND ledger_type = 'actual';
CREATE INDEX IF NOT EXISTS idx_cost_ledger_export
    ON cost_ledger_entries(finance_export_line_id);

ALTER TABLE requirements
    ADD COLUMN IF NOT EXISTS budget_amount NUMERIC(14,2) NOT NULL DEFAULT 0;

ALTER TABLE requirements
    ADD COLUMN IF NOT EXISTS budget_currency TEXT NOT NULL DEFAULT 'CNY';

ALTER TABLE projects
    ADD COLUMN IF NOT EXISTS budget_currency TEXT NOT NULL DEFAULT 'CNY';

ALTER TABLE finance_export_lines
    ADD COLUMN IF NOT EXISTS cost_ledger_entry_id UUID REFERENCES cost_ledger_entries(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_finance_lines_cost_ledger
    ON finance_export_lines(cost_ledger_entry_id)
    WHERE cost_ledger_entry_id IS NOT NULL;

INSERT INTO cost_budgets (scope_type, scope_id, amount, currency, base_amount, base_currency, metadata)
SELECT 'requirement', id, budget_amount, budget_currency, budget_amount, 'CNY',
       jsonb_build_object('backfilled_from', 'requirements.budget_amount')
FROM requirements
WHERE budget_amount > 0
ON CONFLICT DO NOTHING;

INSERT INTO cost_budgets (scope_type, scope_id, amount, currency, base_amount, base_currency, metadata)
SELECT 'project', id, budget_amount, budget_currency, budget_amount, 'CNY',
       jsonb_build_object('backfilled_from', 'projects.budget_amount')
FROM projects
WHERE budget_amount > 0
ON CONFLICT DO NOTHING;


-- -----------------------------------------------------------------------------
-- Folded from ERP-strong historical migration: 027_finance_expense_ingestion.sql
-- -----------------------------------------------------------------------------

-- 027_finance_expense_ingestion.sql
-- Finance expense ingestion, payables, payments, and richer financial dimensions.

ALTER TABLE finance_adapters
    ADD COLUMN IF NOT EXISTS adapter_type TEXT NOT NULL DEFAULT 'generic'
        CHECK (adapter_type IN ('generic', 'expense_api', 'file_import', 'scheduled_pull', 'payroll', 'model_billing', 'agent_billing'));

ALTER TABLE finance_adapters
    ADD COLUMN IF NOT EXISTS direction TEXT NOT NULL DEFAULT 'export'
        CHECK (direction IN ('export', 'import', 'bidirectional'));

ALTER TABLE finance_adapters
    ADD COLUMN IF NOT EXISTS field_mapping JSONB NOT NULL DEFAULT '{}';

ALTER TABLE finance_adapters
    ADD COLUMN IF NOT EXISTS pull_config JSONB NOT NULL DEFAULT '{}';

ALTER TABLE finance_adapters
    ADD COLUMN IF NOT EXISTS last_sync_at TIMESTAMPTZ;

ALTER TABLE finance_adapters
    ADD COLUMN IF NOT EXISTS last_sync_status TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS finance_import_batches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    adapter_id UUID REFERENCES finance_adapters(id) ON DELETE SET NULL,
    source_type TEXT NOT NULL CHECK (source_type IN ('api', 'webhook', 'file', 'pull')),
    file_name TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'processing'
        CHECK (status IN ('processing', 'completed', 'completed_with_errors', 'failed')),
    total_records INT NOT NULL DEFAULT 0,
    processed_records INT NOT NULL DEFAULT 0,
    failed_records INT NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS finance_payables (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payable_type TEXT NOT NULL DEFAULT 'expense'
        CHECK (payable_type IN ('expense', 'salary', 'project', 'model', 'agent', 'vendor')),
    source_type TEXT NOT NULL DEFAULT 'manual',
    source_id UUID,
    external_payable_id TEXT NOT NULL DEFAULT '',
    invoice_number TEXT NOT NULL DEFAULT '',
    vendor_id TEXT NOT NULL DEFAULT '',
    vendor_name TEXT NOT NULL DEFAULT '',
    employee_id TEXT NOT NULL DEFAULT '',
    employee_name TEXT NOT NULL DEFAULT '',
    agent_id UUID,
    project_id UUID REFERENCES projects(id) ON DELETE SET NULL,
    organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
    department_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    account_code TEXT NOT NULL DEFAULT '',
    account_name TEXT NOT NULL DEFAULT '',
    cost_center_code TEXT NOT NULL DEFAULT '',
    cost_center_name TEXT NOT NULL DEFAULT '',
    amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    tax_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'CNY' REFERENCES currencies(code) ON DELETE RESTRICT,
    period_start DATE,
    period_end DATE,
    invoice_date DATE,
    due_date DATE,
    status TEXT NOT NULL DEFAULT 'open'
        CHECK (status IN ('open', 'partially_paid', 'paid', 'void')),
    paid_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_finance_payables_external
    ON finance_payables(source_type, external_payable_id)
    WHERE external_payable_id <> '';

CREATE TABLE IF NOT EXISTS finance_payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_number TEXT NOT NULL DEFAULT '',
    external_payment_id TEXT NOT NULL DEFAULT '',
    payment_method TEXT NOT NULL DEFAULT '',
    payer_account TEXT NOT NULL DEFAULT '',
    payee_account TEXT NOT NULL DEFAULT '',
    vendor_id TEXT NOT NULL DEFAULT '',
    vendor_name TEXT NOT NULL DEFAULT '',
    employee_id TEXT NOT NULL DEFAULT '',
    employee_name TEXT NOT NULL DEFAULT '',
    amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'CNY' REFERENCES currencies(code) ON DELETE RESTRICT,
    paid_at TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'submitted', 'paid', 'failed', 'void')),
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_finance_payments_external
    ON finance_payments(external_payment_id)
    WHERE external_payment_id <> '';

CREATE TABLE IF NOT EXISTS finance_payment_allocations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_id UUID NOT NULL REFERENCES finance_payments(id) ON DELETE CASCADE,
    payable_id UUID NOT NULL REFERENCES finance_payables(id) ON DELETE CASCADE,
    amount NUMERIC(18,8) NOT NULL CHECK (amount > 0),
    currency TEXT NOT NULL DEFAULT 'CNY' REFERENCES currencies(code) ON DELETE RESTRICT,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_finance_payment_allocations_payment
    ON finance_payment_allocations(payment_id);

CREATE INDEX IF NOT EXISTS idx_finance_payment_allocations_payable
    ON finance_payment_allocations(payable_id);

ALTER TABLE cost_ledger_entries
    ADD COLUMN IF NOT EXISTS expense_type TEXT NOT NULL DEFAULT '';

ALTER TABLE cost_ledger_entries
    ADD COLUMN IF NOT EXISTS account_code TEXT NOT NULL DEFAULT '';

ALTER TABLE cost_ledger_entries
    ADD COLUMN IF NOT EXISTS account_name TEXT NOT NULL DEFAULT '';

ALTER TABLE cost_ledger_entries
    ADD COLUMN IF NOT EXISTS cost_center_code TEXT NOT NULL DEFAULT '';

ALTER TABLE cost_ledger_entries
    ADD COLUMN IF NOT EXISTS cost_center_name TEXT NOT NULL DEFAULT '';

ALTER TABLE cost_ledger_entries
    ADD COLUMN IF NOT EXISTS vendor_id TEXT NOT NULL DEFAULT '';

ALTER TABLE cost_ledger_entries
    ADD COLUMN IF NOT EXISTS vendor_name TEXT NOT NULL DEFAULT '';

ALTER TABLE cost_ledger_entries
    ADD COLUMN IF NOT EXISTS employee_id TEXT NOT NULL DEFAULT '';

ALTER TABLE cost_ledger_entries
    ADD COLUMN IF NOT EXISTS employee_name TEXT NOT NULL DEFAULT '';

ALTER TABLE cost_ledger_entries
    ADD COLUMN IF NOT EXISTS agent_id UUID;

ALTER TABLE cost_ledger_entries
    ADD COLUMN IF NOT EXISTS agent_name TEXT NOT NULL DEFAULT '';

ALTER TABLE cost_ledger_entries
    ADD COLUMN IF NOT EXISTS tax_amount NUMERIC(18,8) NOT NULL DEFAULT 0;

ALTER TABLE cost_ledger_entries
    ADD COLUMN IF NOT EXISTS tax_rate NUMERIC(10,6) NOT NULL DEFAULT 0;

ALTER TABLE cost_ledger_entries
    ADD COLUMN IF NOT EXISTS invoice_number TEXT NOT NULL DEFAULT '';

ALTER TABLE cost_ledger_entries
    ADD COLUMN IF NOT EXISTS invoice_date DATE;

ALTER TABLE cost_ledger_entries
    ADD COLUMN IF NOT EXISTS payment_status TEXT NOT NULL DEFAULT ''
        CHECK (payment_status IN ('', 'unpaid', 'partially_paid', 'paid', 'failed', 'void'));

ALTER TABLE cost_ledger_entries
    ADD COLUMN IF NOT EXISTS payment_due_date DATE;

ALTER TABLE cost_ledger_entries
    ADD COLUMN IF NOT EXISTS paid_at TIMESTAMPTZ;

ALTER TABLE cost_ledger_entries
    ADD COLUMN IF NOT EXISTS period_start DATE;

ALTER TABLE cost_ledger_entries
    ADD COLUMN IF NOT EXISTS period_end DATE;

ALTER TABLE cost_ledger_entries
    ADD COLUMN IF NOT EXISTS finance_import_record_id UUID;

ALTER TABLE cost_ledger_entries
    ADD COLUMN IF NOT EXISTS finance_payable_id UUID REFERENCES finance_payables(id) ON DELETE SET NULL;

ALTER TABLE cost_ledger_entries
    ADD COLUMN IF NOT EXISTS finance_payment_id UUID REFERENCES finance_payments(id) ON DELETE SET NULL;

CREATE TABLE IF NOT EXISTS finance_import_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id UUID NOT NULL REFERENCES finance_import_batches(id) ON DELETE CASCADE,
    adapter_id UUID REFERENCES finance_adapters(id) ON DELETE SET NULL,
    external_record_id TEXT NOT NULL,
    expense_type TEXT NOT NULL DEFAULT '',
    raw_payload JSONB NOT NULL DEFAULT '{}',
    normalized_payload JSONB NOT NULL DEFAULT '{}',
    cost_ledger_entry_id UUID REFERENCES cost_ledger_entries(id) ON DELETE SET NULL,
    payable_id UUID REFERENCES finance_payables(id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'posted'
        CHECK (status IN ('posted', 'duplicate', 'failed')),
    error_message TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_finance_import_records_external
    ON finance_import_records(adapter_id, external_record_id)
    WHERE adapter_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_finance_import_records_batch
    ON finance_import_records(batch_id, status);

DO $$ BEGIN
    ALTER TABLE cost_ledger_entries
        ADD CONSTRAINT fk_cost_ledger_finance_import_record
        FOREIGN KEY (finance_import_record_id) REFERENCES finance_import_records(id) ON DELETE SET NULL;
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

CREATE INDEX IF NOT EXISTS idx_cost_ledger_finance_dimensions
    ON cost_ledger_entries(expense_type, account_code, cost_center_code, occurred_at DESC);

CREATE INDEX IF NOT EXISTS idx_cost_ledger_salary_privacy
    ON cost_ledger_entries(project_id, employee_id, expense_type)
    WHERE expense_type = 'salary';


-- -----------------------------------------------------------------------------
-- Folded from ERP-strong historical migration: 028_finance_receivables_accounting_crud.sql
-- -----------------------------------------------------------------------------

-- Finance accounting aliases, project settlements, receivables, receipts, and CRUD status fields.

CREATE TABLE IF NOT EXISTS finance_settlement_orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    settlement_number TEXT NOT NULL DEFAULT '',
    project_id UUID REFERENCES projects(id) ON DELETE SET NULL,
    requirement_id UUID REFERENCES requirements(id) ON DELETE SET NULL,
    deliverable_id UUID REFERENCES deliverables(id) ON DELETE SET NULL,
    customer_id TEXT NOT NULL DEFAULT '',
    customer_name TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    subtotal NUMERIC(18,8) NOT NULL DEFAULT 0,
    tax_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    total_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'CNY' REFERENCES currencies(code) ON DELETE RESTRICT,
    settlement_date DATE,
    due_date DATE,
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'posted', 'void')),
    receivable_id UUID,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_finance_settlement_orders_number
    ON finance_settlement_orders(settlement_number)
    WHERE settlement_number <> '';

CREATE INDEX IF NOT EXISTS idx_finance_settlement_orders_project
    ON finance_settlement_orders(project_id, status, created_at DESC);

CREATE TABLE IF NOT EXISTS finance_settlement_lines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    settlement_order_id UUID NOT NULL REFERENCES finance_settlement_orders(id) ON DELETE CASCADE,
    line_type TEXT NOT NULL DEFAULT 'manual',
    source_type TEXT NOT NULL DEFAULT '',
    source_id UUID,
    deliverable_id UUID REFERENCES deliverables(id) ON DELETE SET NULL,
    description TEXT NOT NULL DEFAULT '',
    quantity NUMERIC(18,8) NOT NULL DEFAULT 1,
    unit_price NUMERIC(18,8) NOT NULL DEFAULT 0,
    amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    tax_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    total_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_finance_settlement_lines_order
    ON finance_settlement_lines(settlement_order_id);

CREATE TABLE IF NOT EXISTS finance_receivables (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    receivable_type TEXT NOT NULL DEFAULT 'project',
    settlement_order_id UUID REFERENCES finance_settlement_orders(id) ON DELETE SET NULL,
    source_type TEXT NOT NULL DEFAULT 'manual',
    source_id UUID,
    external_receivable_id TEXT NOT NULL DEFAULT '',
    invoice_number TEXT NOT NULL DEFAULT '',
    customer_id TEXT NOT NULL DEFAULT '',
    customer_name TEXT NOT NULL DEFAULT '',
    project_id UUID REFERENCES projects(id) ON DELETE SET NULL,
    requirement_id UUID REFERENCES requirements(id) ON DELETE SET NULL,
    organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
    department_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    account_code TEXT NOT NULL DEFAULT '',
    account_name TEXT NOT NULL DEFAULT '',
    amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    tax_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'CNY' REFERENCES currencies(code) ON DELETE RESTRICT,
    period_start DATE,
    period_end DATE,
    invoice_date DATE,
    due_date DATE,
    status TEXT NOT NULL DEFAULT 'unpaid'
        CHECK (status IN ('unpaid', 'partially_received', 'paid', 'void')),
    received_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_finance_receivables_external
    ON finance_receivables(source_type, external_receivable_id)
    WHERE external_receivable_id <> '';

CREATE INDEX IF NOT EXISTS idx_finance_receivables_project
    ON finance_receivables(project_id, status, created_at DESC);

CREATE TABLE IF NOT EXISTS finance_receivable_lines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    receivable_id UUID NOT NULL REFERENCES finance_receivables(id) ON DELETE CASCADE,
    settlement_line_id UUID REFERENCES finance_settlement_lines(id) ON DELETE SET NULL,
    line_type TEXT NOT NULL DEFAULT 'manual',
    description TEXT NOT NULL DEFAULT '',
    amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    tax_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    total_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_finance_receivable_lines_receivable
    ON finance_receivable_lines(receivable_id);

CREATE TABLE IF NOT EXISTS finance_receipts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    receipt_number TEXT NOT NULL DEFAULT '',
    external_receipt_id TEXT NOT NULL DEFAULT '',
    payment_method TEXT NOT NULL DEFAULT '',
    payer_account TEXT NOT NULL DEFAULT '',
    receiver_account TEXT NOT NULL DEFAULT '',
    customer_id TEXT NOT NULL DEFAULT '',
    customer_name TEXT NOT NULL DEFAULT '',
    amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'CNY' REFERENCES currencies(code) ON DELETE RESTRICT,
    received_at TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'received'
        CHECK (status IN ('draft', 'received', 'allocated', 'void')),
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_finance_receipts_external
    ON finance_receipts(external_receipt_id)
    WHERE external_receipt_id <> '';

CREATE TABLE IF NOT EXISTS finance_receipt_allocations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    receipt_id UUID NOT NULL REFERENCES finance_receipts(id) ON DELETE CASCADE,
    receivable_id UUID NOT NULL REFERENCES finance_receivables(id) ON DELETE CASCADE,
    amount NUMERIC(18,8) NOT NULL CHECK (amount > 0),
    currency TEXT NOT NULL DEFAULT 'CNY' REFERENCES currencies(code) ON DELETE RESTRICT,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_finance_receipt_allocations_receipt
    ON finance_receipt_allocations(receipt_id);

CREATE INDEX IF NOT EXISTS idx_finance_receipt_allocations_receivable
    ON finance_receipt_allocations(receivable_id);

CREATE TABLE IF NOT EXISTS gl_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    master_key TEXT NOT NULL DEFAULT next_business_key('gl_accounts', 'GLA'),
    account_code TEXT NOT NULL,
    name TEXT NOT NULL,
    account_type TEXT NOT NULL DEFAULT 'expense'
        CHECK (account_type IN ('asset', 'liability', 'equity', 'revenue', 'expense', 'other')),
    currency TEXT NOT NULL DEFAULT 'CNY' REFERENCES currencies(code) ON DELETE RESTRICT,
    parent_account_code TEXT NOT NULL DEFAULT '',
    postable BOOLEAN NOT NULL DEFAULT TRUE,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    department_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    legacy_id UUID,
    parent_master_table TEXT,
    parent_master_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_gl_accounts_master_key ON gl_accounts(master_key);
CREATE UNIQUE INDEX IF NOT EXISTS uq_gl_accounts_org_code
    ON gl_accounts(COALESCE(organization_id, '00000000-0000-0000-0000-000000000000'::uuid), account_code);
CREATE INDEX IF NOT EXISTS idx_gl_accounts_active ON gl_accounts(active, account_type, account_code);

CREATE TABLE IF NOT EXISTS gl_cost_centers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    master_key TEXT NOT NULL DEFAULT next_business_key('gl_cost_centers', 'GLC'),
    cost_center_code TEXT NOT NULL,
    name TEXT NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    department_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    legacy_id UUID,
    parent_master_table TEXT,
    parent_master_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_gl_cost_centers_master_key ON gl_cost_centers(master_key);
CREATE UNIQUE INDEX IF NOT EXISTS uq_gl_cost_centers_org_code
    ON gl_cost_centers(COALESCE(organization_id, '00000000-0000-0000-0000-000000000000'::uuid), cost_center_code);
CREATE INDEX IF NOT EXISTS idx_gl_cost_centers_active ON gl_cost_centers(active, cost_center_code);

CREATE TABLE IF NOT EXISTS gl_journal_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    master_key TEXT NOT NULL DEFAULT next_business_key('gl_journal_entries', 'GLJ'),
    entry_number TEXT NOT NULL DEFAULT '',
    reference_date DATE NOT NULL,
    memo TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'posted', 'void')),
    currency TEXT NOT NULL DEFAULT 'CNY' REFERENCES currencies(code) ON DELETE RESTRICT,
    source_type TEXT NOT NULL DEFAULT '',
    source_id UUID,
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    department_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    posted_at TIMESTAMPTZ,
    legacy_id UUID,
    parent_master_table TEXT,
    parent_master_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_gl_journal_entries_master_key ON gl_journal_entries(master_key);
CREATE INDEX IF NOT EXISTS idx_gl_journal_entries_org_status
    ON gl_journal_entries(organization_id, status, reference_date DESC);
CREATE INDEX IF NOT EXISTS idx_gl_journal_entries_source
    ON gl_journal_entries(source_type, source_id)
    WHERE source_type <> '' AND source_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS gl_journal_entry_lines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    master_key TEXT NOT NULL DEFAULT next_business_key('gl_journal_entry_lines', 'GLL'),
    entry_id UUID NOT NULL REFERENCES gl_journal_entries(id) ON DELETE CASCADE,
    line_num INT NOT NULL,
    account_code TEXT NOT NULL,
    account_name TEXT NOT NULL DEFAULT '',
    cost_center_code TEXT NOT NULL DEFAULT '',
    debit NUMERIC(18,8) NOT NULL DEFAULT 0 CHECK (debit >= 0),
    credit NUMERIC(18,8) NOT NULL DEFAULT 0 CHECK (credit >= 0),
    description TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}',
    legacy_id UUID,
    parent_master_table TEXT,
    parent_master_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_gl_journal_entry_lines_side CHECK (
        (debit > 0 AND credit = 0) OR (credit > 0 AND debit = 0)
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_gl_journal_entry_lines_master_key ON gl_journal_entry_lines(master_key);
CREATE UNIQUE INDEX IF NOT EXISTS uq_gl_journal_entry_lines_entry_line
    ON gl_journal_entry_lines(entry_id, line_num);
CREATE INDEX IF NOT EXISTS idx_gl_journal_entry_lines_account
    ON gl_journal_entry_lines(account_code, cost_center_code);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'fk_finance_settlement_receivable'
    ) THEN
        ALTER TABLE finance_settlement_orders
            ADD CONSTRAINT fk_finance_settlement_receivable
            FOREIGN KEY (receivable_id) REFERENCES finance_receivables(id) ON DELETE SET NULL
            NOT VALID;
    END IF;
END $$;

ALTER TABLE finance_settlement_orders
    VALIDATE CONSTRAINT fk_finance_settlement_receivable;

ALTER TABLE finance_payables
    ADD COLUMN IF NOT EXISTS void_reason TEXT NOT NULL DEFAULT '';

ALTER TABLE finance_payments
    ADD COLUMN IF NOT EXISTS void_reason TEXT NOT NULL DEFAULT '';


-- -----------------------------------------------------------------------------
-- Folded from ERP-strong historical migration: 031_master_key_defaults.sql
-- -----------------------------------------------------------------------------

-- 031_master_key_defaults.sql
-- Ensure records inserted after the master/detail migration receive business keys.

DO $$
DECLARE
    rec RECORD;
BEGIN
    FOR rec IN
        SELECT table_name, key_prefix
        FROM data_table_catalog
        ORDER BY table_name
    LOOP
        IF EXISTS (
            SELECT 1
            FROM information_schema.columns
            WHERE table_schema = 'public'
              AND table_name = rec.table_name
              AND column_name = 'master_key'
        ) THEN
            EXECUTE FORMAT(
                'ALTER TABLE %I ALTER COLUMN master_key SET DEFAULT next_business_key(%L, %L)',
                rec.table_name,
                rec.table_name,
                rec.key_prefix
            );
            EXECUTE FORMAT(
                'UPDATE %I SET master_key = next_business_key(%L, %L) WHERE master_key IS NULL',
                rec.table_name,
                rec.table_name,
                rec.key_prefix
            );
            EXECUTE FORMAT('ALTER TABLE %I ALTER COLUMN master_key SET NOT NULL', rec.table_name);
        END IF;
    END LOOP;
END;
$$;


-- -----------------------------------------------------------------------------
-- Folded from ERP-strong historical migration: 032_module_master_detail_unification.sql
-- -----------------------------------------------------------------------------

-- 032_module_master_detail_unification.sql
-- Canonical module-level master/detail tables.
--
-- This migration is intentionally non-destructive. Legacy tables remain in place
-- until migration validation passes and a separate, explicit drop migration is
-- approved.

CREATE TABLE IF NOT EXISTS module_master_source_catalog (
    module_name     TEXT NOT NULL,
    source_table    TEXT PRIMARY KEY,
    entity_type     TEXT NOT NULL,
    relation_mode   TEXT NOT NULL DEFAULT 'master'
        CHECK (relation_mode IN ('master', 'detail')),
    parent_table    TEXT,
    parent_fk       TEXT,
    key_prefix      TEXT NOT NULL,
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO module_master_source_catalog(module_name, source_table, entity_type, relation_mode, parent_table, parent_fk, key_prefix)
VALUES
    ('identity', 'users', 'user', 'master', NULL, NULL, 'USR'),
    ('identity', 'roles', 'role', 'master', NULL, NULL, 'ROL'),
    ('identity', 'user_roles', 'user_role', 'detail', 'users', 'user_id', 'URO'),

    ('organization', 'organizations', 'organization', 'master', NULL, NULL, 'ORG'),
    ('organization', 'muvrs', 'mvru', 'detail', 'organizations', 'organization_id', 'MVR'),
    ('organization', 'teams', 'team', 'detail', 'muvrs', 'mvru_id', 'TEM'),
    ('organization', 'mvru_members', 'mvru_member', 'detail', 'muvrs', 'mvru_id', 'MMM'),
    ('organization', 'mvru_relationships', 'mvru_relationship', 'detail', 'muvrs', 'source_mvru_id', 'MRL'),
    ('organization', 'departments', 'department', 'detail', 'organizations', 'organization_id', 'DEP'),
    ('organization', 'external_members', 'external_member', 'master', NULL, NULL, 'EXT'),
    ('organization', 'organization_memberships', 'organization_membership', 'detail', 'departments', 'department_id', 'OMB'),
    ('organization', 'department_mvru_links', 'department_mvru_link', 'detail', 'departments', 'department_id', 'DML'),
    ('organization', 'positions', 'position', 'detail', 'departments', 'department_id', 'POS'),
    ('organization', 'position_assignments', 'position_assignment', 'detail', 'positions', 'position_id', 'PAS'),
    ('organization', 'employee_profiles', 'employee_profile', 'detail', 'users', 'user_id', 'EMP'),

    ('layer', 'layer_configs', 'layer_config', 'detail', 'muvrs', 'mvru_id', 'LAY'),
    ('layer', 'layer_routing_rules', 'layer_routing_rule', 'detail', 'muvrs', 'mvru_id', 'LRR'),

    ('capability', 'capabilities', 'capability', 'master', NULL, NULL, 'CAP'),
    ('capability', 'capability_bindings', 'capability_binding', 'detail', 'capabilities', 'capability_id', 'CBN'),
    ('capability', 'capability_invocations', 'capability_invocation', 'detail', 'capabilities', 'capability_id', 'CIN'),
    ('capability', 'capability_evaluations', 'capability_evaluation', 'detail', 'capabilities', 'capability_id', 'CEV'),

    ('workflow', 'workflow_templates', 'workflow_template', 'master', NULL, NULL, 'WFT'),
    ('workflow', 'workflow_instances', 'workflow_instance', 'detail', 'workflow_templates', 'template_id', 'WFI'),
    ('workflow', 'tasks', 'task', 'detail', 'workflow_instances', 'workflow_id', 'TSK'),
    ('workflow', 'decisions', 'decision', 'detail', 'tasks', 'task_id', 'DEC'),
    ('workflow', 'workflow_contexts', 'workflow_context', 'detail', 'workflow_instances', 'workflow_id', 'WFC'),
    ('workflow', 'task_matrix_assignments', 'task_matrix_assignment', 'detail', 'tasks', 'task_id', 'TMA'),

    ('project_lifecycle', 'requirements', 'requirement', 'master', NULL, NULL, 'REQ'),
    ('project_lifecycle', 'requirement_documents', 'requirement_document', 'detail', 'requirements', 'requirement_id', 'RDOC'),
    ('project_lifecycle', 'requirement_analysis_workflows', 'requirement_analysis_workflow', 'detail', 'requirements', 'requirement_id', 'RAW'),
    ('project_lifecycle', 'projects', 'project', 'detail', 'requirements', 'requirement_id', 'PRJ'),
    ('project_lifecycle', 'project_members', 'project_member', 'detail', 'projects', 'project_id', 'PMB'),
    ('project_lifecycle', 'project_workflows', 'project_workflow', 'detail', 'projects', 'project_id', 'PWF'),
    ('project_lifecycle', 'deliverables', 'deliverable', 'detail', 'projects', 'project_id', 'DEL'),
    ('project_lifecycle', 'project_cost_entries', 'project_cost_entry', 'detail', 'projects', 'project_id', 'PCE'),
    ('project_lifecycle', 'project_evaluations', 'project_evaluation', 'detail', 'projects', 'project_id', 'PEV'),

    ('finance', 'finance_adapters', 'finance_adapter', 'master', NULL, NULL, 'FAD'),
    ('finance', 'finance_export_batches', 'finance_export_batch', 'detail', 'finance_adapters', 'adapter_id', 'FEB'),
    ('finance', 'finance_export_lines', 'finance_export_line', 'detail', 'finance_export_batches', 'batch_id', 'FEL'),
    ('finance', 'finance_webhook_events', 'finance_webhook_event', 'detail', 'finance_adapters', 'adapter_id', 'FWE'),
    ('finance', 'finance_import_batches', 'finance_import_batch', 'detail', 'finance_adapters', 'adapter_id', 'FIB'),
    ('finance', 'finance_import_records', 'finance_import_record', 'detail', 'finance_import_batches', 'batch_id', 'FIR'),
    ('finance', 'gl_accounts', 'gl_account', 'master', NULL, NULL, 'GLA'),
    ('finance', 'gl_cost_centers', 'gl_cost_center', 'master', NULL, NULL, 'GLC'),
    ('finance', 'gl_journal_entries', 'gl_journal_entry', 'master', NULL, NULL, 'GLJ'),
    ('finance', 'gl_journal_entry_lines', 'gl_journal_entry_line', 'detail', 'gl_journal_entries', 'entry_id', 'GLL'),
    ('finance', 'finance_payables', 'finance_payable', 'master', NULL, NULL, 'FPY'),
    ('finance', 'finance_payments', 'finance_payment', 'master', NULL, NULL, 'FPM'),
    ('finance', 'finance_payment_allocations', 'finance_payment_allocation', 'detail', 'finance_payments', 'payment_id', 'FPA'),
    ('finance', 'finance_settlement_orders', 'finance_settlement_order', 'master', NULL, NULL, 'FSO'),
    ('finance', 'finance_settlement_lines', 'finance_settlement_line', 'detail', 'finance_settlement_orders', 'settlement_order_id', 'FSL'),
    ('finance', 'finance_receivables', 'finance_receivable', 'detail', 'finance_settlement_orders', 'settlement_order_id', 'FRC'),
    ('finance', 'finance_receivable_lines', 'finance_receivable_line', 'detail', 'finance_receivables', 'receivable_id', 'FRL'),
    ('finance', 'finance_receipts', 'finance_receipt', 'master', NULL, NULL, 'FRP'),
    ('finance', 'finance_receipt_allocations', 'finance_receipt_allocation', 'detail', 'finance_receipts', 'receipt_id', 'FRA'),

    ('costing', 'currencies', 'currency', 'master', NULL, NULL, 'CUR'),
    ('costing', 'exchange_rate_versions', 'exchange_rate_version', 'detail', 'currencies', 'from_currency', 'ERV'),
    ('costing', 'cost_rate_cards', 'cost_rate_card', 'detail', 'currencies', 'currency', 'CRC'),
    ('costing', 'cost_budgets', 'cost_budget', 'detail', 'currencies', 'currency', 'CBU'),
    ('costing', 'cost_ledger_entries', 'cost_ledger_entry', 'detail', 'currencies', 'currency', 'CLE'),

    ('governance', 'permissions', 'permission', 'master', NULL, NULL, 'PER'),
    ('governance', 'principles', 'principle', 'master', NULL, NULL, 'PRI'),
    ('governance', 'control_rules', 'control_rule', 'detail', 'principles', 'principle_id', 'CRL'),
    ('governance', 'access_decisions', 'access_decision', 'master', NULL, NULL, 'ACD'),
    ('governance', 'context_weight_scores', 'context_weight_score', 'detail', 'workflow_templates', 'workflow_template_id', 'CWS'),
    ('governance', 'field_permission_rules', 'field_permission_rule', 'master', NULL, NULL, 'FPR'),
    ('governance', 'user_field_preferences', 'user_field_preference', 'master', NULL, NULL, 'UFP'),

    ('verification', 'verification_reports', 'verification_report', 'master', NULL, NULL, 'VRP'),
    ('verification', 'review_assignments', 'review_assignment', 'detail', 'verification_reports', 'report_id', 'RVA'),

    ('evolution', 'weight_scores', 'weight_score', 'master', NULL, NULL, 'WSC'),
    ('evolution', 'weight_alphas', 'weight_alpha', 'master', NULL, NULL, 'WAL'),
    ('evolution', 'experiments', 'experiment', 'master', NULL, NULL, 'EXP'),
    ('evolution', 'knowledge_entries', 'knowledge_entry', 'master', NULL, NULL, 'KNE'),
    ('evolution', 'signals', 'signal', 'master', NULL, NULL, 'SIG'),

    ('observability', 'traces', 'trace', 'master', NULL, NULL, 'TRC'),
    ('observability', 'spans', 'span', 'detail', 'traces', 'trace_id', 'SPN'),
    ('observability', 'metrics', 'metric', 'master', NULL, NULL, 'MET'),

    ('metaresource', 'meta_resources', 'meta_resource', 'master', NULL, NULL, 'MRS'),
    ('metaresource', 'demand_profiles', 'demand_profile', 'detail', 'requirements', 'requirement_id', 'DPR'),
    ('metaresource', 'pdca_cycles', 'pdca_cycle', 'detail', 'demand_profiles', 'demand_profile_id', 'PDC'),
    ('metaresource', 'pdca_events', 'pdca_event', 'detail', 'pdca_cycles', 'cycle_id', 'PDE')
ON CONFLICT (source_table) DO UPDATE SET
    module_name = EXCLUDED.module_name,
    entity_type = EXCLUDED.entity_type,
    relation_mode = EXCLUDED.relation_mode,
    parent_table = EXCLUDED.parent_table,
    parent_fk = EXCLUDED.parent_fk,
    key_prefix = EXCLUDED.key_prefix,
    updated_at = NOW();

CREATE OR REPLACE FUNCTION module_table_exists(p_table TEXT)
RETURNS BOOLEAN
LANGUAGE sql
STABLE
AS $$
    SELECT EXISTS (
        SELECT 1
        FROM information_schema.tables
        WHERE table_schema = 'public'
          AND table_name = p_table
          AND table_type = 'BASE TABLE'
    );
$$;

CREATE OR REPLACE FUNCTION module_column_exists(p_table TEXT, p_column TEXT)
RETURNS BOOLEAN
LANGUAGE sql
STABLE
AS $$
    SELECT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = p_table
          AND column_name = p_column
    );
$$;

CREATE OR REPLACE FUNCTION module_column_udt(p_table TEXT, p_column TEXT)
RETURNS TEXT
LANGUAGE sql
STABLE
AS $$
    SELECT udt_name
    FROM information_schema.columns
    WHERE table_schema = 'public'
      AND table_name = p_table
      AND column_name = p_column
    LIMIT 1;
$$;

CREATE OR REPLACE FUNCTION module_first_text_expr(p_table TEXT, p_columns TEXT[], p_alias TEXT)
RETURNS TEXT
LANGUAGE plpgsql
STABLE
AS $$
DECLARE
    v_column TEXT;
BEGIN
    FOREACH v_column IN ARRAY p_columns LOOP
        IF module_column_exists(p_table, v_column) THEN
            RETURN FORMAT('COALESCE(%s.%I::TEXT, '''')', p_alias, v_column);
        END IF;
    END LOOP;
    RETURN '''''';
END;
$$;

CREATE OR REPLACE FUNCTION module_uuid_expr(p_table TEXT, p_column TEXT, p_alias TEXT)
RETURNS TEXT
LANGUAGE plpgsql
STABLE
AS $$
BEGIN
    IF module_column_exists(p_table, p_column) AND module_column_udt(p_table, p_column) = 'uuid' THEN
        RETURN FORMAT('%s.%I', p_alias, p_column);
    END IF;
    RETURN 'NULL::UUID';
END;
$$;

CREATE OR REPLACE FUNCTION module_jsonb_expr(p_table TEXT, p_column TEXT, p_alias TEXT)
RETURNS TEXT
LANGUAGE plpgsql
STABLE
AS $$
BEGIN
    IF module_column_exists(p_table, p_column) AND module_column_udt(p_table, p_column) IN ('json', 'jsonb') THEN
        RETURN FORMAT('COALESCE(%s.%I::JSONB, ''{}''::JSONB)', p_alias, p_column);
    END IF;
    RETURN '''{}''::JSONB';
END;
$$;

CREATE OR REPLACE FUNCTION module_timestamp_expr(p_table TEXT, p_column TEXT, p_alias TEXT)
RETURNS TEXT
LANGUAGE plpgsql
STABLE
AS $$
BEGIN
    IF module_column_exists(p_table, p_column) THEN
        RETURN FORMAT('COALESCE(%s.%I::TIMESTAMPTZ, NOW())', p_alias, p_column);
    END IF;
    IF p_column = 'updated_at' AND module_column_exists(p_table, 'created_at') THEN
        RETURN FORMAT('COALESCE(%s.created_at::TIMESTAMPTZ, NOW())', p_alias);
    END IF;
    RETURN 'NOW()';
END;
$$;

CREATE OR REPLACE FUNCTION module_legacy_pk_expr(p_table TEXT, p_alias TEXT)
RETURNS TEXT
LANGUAGE plpgsql
STABLE
AS $$
BEGIN
    IF module_column_exists(p_table, 'id') THEN
        RETURN FORMAT('%s.id::TEXT', p_alias);
    ELSIF module_column_exists(p_table, 'code') THEN
        RETURN FORMAT('%s.code::TEXT', p_alias);
    ELSIF module_column_exists(p_table, 'user_id') THEN
        RETURN FORMAT('%s.user_id::TEXT', p_alias);
    ELSIF module_column_exists(p_table, 'entity_name') THEN
        RETURN FORMAT('%s.entity_name::TEXT', p_alias);
    END IF;
    RETURN FORMAT('%s.master_key::TEXT', p_alias);
END;
$$;

CREATE OR REPLACE FUNCTION ensure_module_master_detail_tables(p_module_name TEXT, p_key_prefix TEXT)
RETURNS VOID
LANGUAGE plpgsql
AS $$
DECLARE
    v_master_table TEXT := p_module_name || '_masters';
    v_detail_table TEXT := p_module_name || '_details';
    v_master_prefix TEXT := UPPER(LEFT(REGEXP_REPLACE(p_module_name, '[^A-Za-z0-9]', '', 'g'), 5)) || 'M';
    v_detail_prefix TEXT := UPPER(LEFT(REGEXP_REPLACE(p_module_name, '[^A-Za-z0-9]', '', 'g'), 5)) || 'D';
BEGIN
    EXECUTE FORMAT(
        'CREATE TABLE IF NOT EXISTS %I (
            master_key TEXT PRIMARY KEY DEFAULT next_business_key(%L, %L),
            entity_type TEXT NOT NULL,
            legacy_table TEXT NOT NULL,
            legacy_pk TEXT NOT NULL,
            legacy_id UUID,
            title TEXT NOT NULL DEFAULT '''',
            name TEXT NOT NULL DEFAULT '''',
            code TEXT NOT NULL DEFAULT '''',
            status TEXT NOT NULL DEFAULT '''',
            organization_id UUID,
            department_id UUID,
            project_id UUID,
            requirement_id UUID,
            workflow_id UUID,
            task_id UUID,
            actor_id UUID,
            actor_type TEXT NOT NULL DEFAULT '''',
            core_data JSONB NOT NULL DEFAULT ''{}''::JSONB,
            metadata JSONB NOT NULL DEFAULT ''{}''::JSONB,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            UNIQUE (legacy_table, legacy_pk)
        )',
        v_master_table,
        v_master_table,
        COALESCE(NULLIF(p_key_prefix, ''), v_master_prefix)
    );

    EXECUTE FORMAT('CREATE INDEX IF NOT EXISTS %I ON %I(entity_type, status)', 'idx_' || v_master_table || '_entity_status', v_master_table);
    EXECUTE FORMAT('CREATE INDEX IF NOT EXISTS %I ON %I(legacy_table, legacy_id)', 'idx_' || v_master_table || '_legacy_id', v_master_table);
    EXECUTE FORMAT('CREATE INDEX IF NOT EXISTS %I ON %I(organization_id, department_id)', 'idx_' || v_master_table || '_org', v_master_table);

    EXECUTE FORMAT(
        'CREATE TABLE IF NOT EXISTS %I (
            sub_key TEXT PRIMARY KEY DEFAULT next_business_key(%L, %L),
            master_key TEXT NOT NULL REFERENCES %I(master_key) ON DELETE CASCADE,
            source_master_key TEXT NOT NULL DEFAULT '''',
            detail_type TEXT NOT NULL,
            legacy_table TEXT NOT NULL,
            legacy_pk TEXT NOT NULL,
            legacy_id UUID,
            parent_legacy_table TEXT NOT NULL DEFAULT '''',
            parent_legacy_pk TEXT NOT NULL DEFAULT '''',
            parent_legacy_id UUID,
            line_no INT NOT NULL DEFAULT 0,
            field_key TEXT NOT NULL DEFAULT '''',
            payload JSONB NOT NULL DEFAULT ''{}''::JSONB,
            metadata JSONB NOT NULL DEFAULT ''{}''::JSONB,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            UNIQUE (legacy_table, legacy_pk)
        )',
        v_detail_table,
        v_detail_table,
        COALESCE(NULLIF(p_key_prefix, ''), v_detail_prefix),
        v_master_table
    );

    EXECUTE FORMAT('CREATE INDEX IF NOT EXISTS %I ON %I(master_key, line_no)', 'idx_' || v_detail_table || '_master_line', v_detail_table);
    EXECUTE FORMAT('CREATE INDEX IF NOT EXISTS %I ON %I(detail_type)', 'idx_' || v_detail_table || '_detail_type', v_detail_table);

    INSERT INTO data_table_catalog(table_name, master_table_name, detail_table_name, key_prefix, display_name, category, is_base_data, is_business_scenario, metadata)
    VALUES
        (v_master_table, v_master_table, v_detail_table, COALESCE(NULLIF(p_key_prefix, ''), v_master_prefix), v_master_table, 'canonical', false, true,
         jsonb_build_object('module_name', p_module_name, 'canonical_role', 'master')),
        (v_detail_table, v_master_table, v_detail_table, COALESCE(NULLIF(p_key_prefix, ''), v_detail_prefix), v_detail_table, 'canonical', false, true,
         jsonb_build_object('module_name', p_module_name, 'canonical_role', 'detail'))
    ON CONFLICT (table_name) DO UPDATE SET
        master_table_name = EXCLUDED.master_table_name,
        detail_table_name = EXCLUDED.detail_table_name,
        key_prefix = EXCLUDED.key_prefix,
        category = EXCLUDED.category,
        is_base_data = EXCLUDED.is_base_data,
        is_business_scenario = EXCLUDED.is_business_scenario,
        metadata = data_table_catalog.metadata || EXCLUDED.metadata,
        updated_at = NOW();
END;
$$;

DO $$
DECLARE
    rec RECORD;
BEGIN
    FOR rec IN
        SELECT module_name, MIN(key_prefix) AS key_prefix
        FROM module_master_source_catalog
        GROUP BY module_name
        ORDER BY module_name
    LOOP
        PERFORM ensure_module_master_detail_tables(rec.module_name, rec.key_prefix);
    END LOOP;
END;
$$;

CREATE OR REPLACE FUNCTION ensure_source_master_key(p_source_table TEXT, p_key_prefix TEXT)
RETURNS VOID
LANGUAGE plpgsql
AS $$
DECLARE
    v_has_uuid_id BOOLEAN;
BEGIN
    IF NOT module_table_exists(p_source_table) THEN
        RETURN;
    END IF;

    INSERT INTO data_table_catalog(table_name, master_table_name, detail_table_name, key_prefix, display_name, category, is_base_data, is_business_scenario, metadata)
    VALUES (
        p_source_table,
        p_source_table || '_masters',
        p_source_table || '_details',
        p_key_prefix,
        p_source_table,
        'legacy',
        false,
        false,
        jsonb_build_object('deprecated', true, 'canonicalized_by', '032_module_master_detail_unification.sql')
    )
    ON CONFLICT (table_name) DO UPDATE SET
        key_prefix = COALESCE(NULLIF(data_table_catalog.key_prefix, ''), EXCLUDED.key_prefix),
        category = 'legacy',
        metadata = data_table_catalog.metadata || EXCLUDED.metadata,
        updated_at = NOW();

    SELECT module_column_exists(p_source_table, 'id') AND module_column_udt(p_source_table, 'id') = 'uuid'
    INTO v_has_uuid_id;

    EXECUTE FORMAT('ALTER TABLE %I ADD COLUMN IF NOT EXISTS legacy_id UUID', p_source_table);
    IF v_has_uuid_id THEN
        EXECUTE FORMAT('UPDATE %I SET legacy_id = id WHERE legacy_id IS NULL AND id IS NOT NULL', p_source_table);
    END IF;

    EXECUTE FORMAT('ALTER TABLE %I ADD COLUMN IF NOT EXISTS master_key TEXT', p_source_table);
    EXECUTE FORMAT(
        'UPDATE %I
         SET master_key = next_business_key(%L, %L)
         WHERE master_key IS NULL
            OR (COALESCE($1, '''') <> '''' AND master_key NOT LIKE COALESCE($1, '''') || ''-%%'')',
        p_source_table,
        p_source_table,
        p_key_prefix
    )
    USING p_key_prefix;
    EXECUTE FORMAT(
        'ALTER TABLE %I ALTER COLUMN master_key SET DEFAULT next_business_key(%L, %L)',
        p_source_table,
        p_source_table,
        p_key_prefix
    );
    EXECUTE FORMAT('ALTER TABLE %I ALTER COLUMN master_key SET NOT NULL', p_source_table);
    EXECUTE FORMAT('CREATE UNIQUE INDEX IF NOT EXISTS %I ON %I(master_key)', 'uq_' || p_source_table || '_master_key', p_source_table);
END;
$$;

DO $$
DECLARE
    rec RECORD;
BEGIN
    FOR rec IN SELECT source_table, key_prefix FROM module_master_source_catalog ORDER BY source_table LOOP
        PERFORM ensure_source_master_key(rec.source_table, rec.key_prefix);
    END LOOP;
END;
$$;

CREATE OR REPLACE FUNCTION upsert_module_masters_for_source(p_source_table TEXT)
RETURNS VOID
LANGUAGE plpgsql
AS $$
DECLARE
    rec RECORD;
    v_master_table TEXT;
    v_sql TEXT;
BEGIN
    SELECT *
    INTO rec
    FROM module_master_source_catalog
    WHERE source_table = p_source_table;

    IF rec.source_table IS NULL OR NOT module_table_exists(rec.source_table) THEN
        RETURN;
    END IF;

    v_master_table := rec.module_name || '_masters';

    EXECUTE FORMAT('DELETE FROM %I WHERE legacy_table = $1', v_master_table)
    USING rec.source_table;

    v_sql := FORMAT(
        'INSERT INTO %I (
            master_key, entity_type, legacy_table, legacy_pk, legacy_id,
            title, name, code, status,
            organization_id, department_id, project_id, requirement_id, workflow_id, task_id,
            actor_id, actor_type, core_data, metadata, created_at, updated_at
         )
         SELECT
            t.master_key,
            %L,
            %L,
            %s,
            t.legacy_id,
            %s,
            %s,
            %s,
            %s,
            %s,
            %s,
            %s,
            %s,
            %s,
            %s,
            %s,
            %s,
            to_jsonb(t),
            %s,
            %s,
            %s
         FROM %I t
         ON CONFLICT (legacy_table, legacy_pk) DO UPDATE SET
            master_key = EXCLUDED.master_key,
            entity_type = EXCLUDED.entity_type,
            legacy_id = EXCLUDED.legacy_id,
            title = EXCLUDED.title,
            name = EXCLUDED.name,
            code = EXCLUDED.code,
            status = EXCLUDED.status,
            organization_id = EXCLUDED.organization_id,
            department_id = EXCLUDED.department_id,
            project_id = EXCLUDED.project_id,
            requirement_id = EXCLUDED.requirement_id,
            workflow_id = EXCLUDED.workflow_id,
            task_id = EXCLUDED.task_id,
            actor_id = EXCLUDED.actor_id,
            actor_type = EXCLUDED.actor_type,
            core_data = EXCLUDED.core_data,
            metadata = EXCLUDED.metadata,
            updated_at = EXCLUDED.updated_at',
        v_master_table,
        rec.entity_type,
        rec.source_table,
        module_legacy_pk_expr(rec.source_table, 't'),
        module_first_text_expr(rec.source_table, ARRAY['title', 'name', 'display_name', 'email', 'username', 'code', 'key', 'action', 'resource', 'file_name'], 't'),
        module_first_text_expr(rec.source_table, ARRAY['name', 'title', 'display_name', 'email', 'username', 'code', 'key', 'file_name'], 't'),
        module_first_text_expr(rec.source_table, ARRAY['code', 'key', 'slug', 'name', 'email'], 't'),
        module_first_text_expr(rec.source_table, ARRAY['status', 'state', 'decision', 'result'], 't'),
        module_uuid_expr(rec.source_table, 'organization_id', 't'),
        module_uuid_expr(rec.source_table, 'department_id', 't'),
        module_uuid_expr(rec.source_table, 'project_id', 't'),
        module_uuid_expr(rec.source_table, 'requirement_id', 't'),
        module_uuid_expr(rec.source_table, 'workflow_id', 't'),
        module_uuid_expr(rec.source_table, 'task_id', 't'),
        module_uuid_expr(rec.source_table, 'actor_id', 't'),
        module_first_text_expr(rec.source_table, ARRAY['actor_type', 'created_by_type', 'uploaded_by_type', 'submitted_by_type', 'evaluator_type', 'member_type'], 't'),
        module_jsonb_expr(rec.source_table, 'metadata', 't'),
        module_timestamp_expr(rec.source_table, 'created_at', 't'),
        module_timestamp_expr(rec.source_table, 'updated_at', 't'),
        rec.source_table
    );

    EXECUTE v_sql;
END;
$$;

CREATE OR REPLACE FUNCTION upsert_module_details_for_source(p_source_table TEXT)
RETURNS VOID
LANGUAGE plpgsql
AS $$
DECLARE
    rec RECORD;
    v_detail_table TEXT;
    v_master_table TEXT;
    v_sql TEXT;
BEGIN
    SELECT *
    INTO rec
    FROM module_master_source_catalog
    WHERE source_table = p_source_table;

    IF rec.source_table IS NULL
       OR rec.relation_mode <> 'detail'
       OR rec.parent_table IS NULL
       OR rec.parent_fk IS NULL
       OR NOT module_table_exists(rec.source_table)
       OR NOT module_table_exists(rec.parent_table)
       OR NOT module_column_exists(rec.source_table, rec.parent_fk)
    THEN
        RETURN;
    END IF;

    v_detail_table := rec.module_name || '_details';
    v_master_table := rec.module_name || '_masters';

    EXECUTE FORMAT('DELETE FROM %I WHERE legacy_table = $1', v_detail_table)
    USING rec.source_table;

    v_sql := FORMAT(
        'INSERT INTO %I (
            sub_key, master_key, source_master_key, detail_type, legacy_table, legacy_pk, legacy_id,
            parent_legacy_table, parent_legacy_pk, parent_legacy_id, line_no,
            payload, metadata, created_at, updated_at
         )
         SELECT
            child.master_key,
            COALESCE(parent_master.master_key, child_master.master_key),
            child.master_key,
            %L,
            %L,
            %s,
            child.legacy_id,
            %L,
            COALESCE(%s, ''''),
            parent_old.legacy_id,
            ROW_NUMBER() OVER (PARTITION BY COALESCE(parent_master.master_key, child_master.master_key) ORDER BY %s),
            to_jsonb(child),
            %s,
            %s,
            %s
         FROM %I child
         JOIN %I child_master ON child_master.legacy_table = %L AND child_master.legacy_pk = %s
         LEFT JOIN %I parent_old ON child.%I::TEXT = %s
         LEFT JOIN %I parent_master ON parent_master.legacy_table = %L AND parent_master.legacy_pk = %s
         ON CONFLICT (legacy_table, legacy_pk) DO UPDATE SET
            master_key = EXCLUDED.master_key,
            source_master_key = EXCLUDED.source_master_key,
            detail_type = EXCLUDED.detail_type,
            legacy_id = EXCLUDED.legacy_id,
            parent_legacy_table = EXCLUDED.parent_legacy_table,
            parent_legacy_pk = EXCLUDED.parent_legacy_pk,
            parent_legacy_id = EXCLUDED.parent_legacy_id,
            line_no = EXCLUDED.line_no,
            payload = EXCLUDED.payload,
            metadata = EXCLUDED.metadata,
            updated_at = EXCLUDED.updated_at',
        v_detail_table,
        rec.entity_type,
        rec.source_table,
        module_legacy_pk_expr(rec.source_table, 'child'),
        rec.parent_table,
        module_legacy_pk_expr(rec.parent_table, 'parent_old'),
        module_legacy_pk_expr(rec.source_table, 'child'),
        module_jsonb_expr(rec.source_table, 'metadata', 'child'),
        module_timestamp_expr(rec.source_table, 'created_at', 'child'),
        module_timestamp_expr(rec.source_table, 'updated_at', 'child'),
        rec.source_table,
        v_master_table,
        rec.source_table,
        module_legacy_pk_expr(rec.source_table, 'child'),
        rec.parent_table,
        rec.parent_fk,
        module_legacy_pk_expr(rec.parent_table, 'parent_old'),
        v_master_table,
        rec.parent_table,
        module_legacy_pk_expr(rec.parent_table, 'parent_old')
    );

    EXECUTE v_sql;
END;
$$;

CREATE OR REPLACE FUNCTION refresh_module_source(p_source_table TEXT)
RETURNS VOID
LANGUAGE plpgsql
AS $$
DECLARE
    child_rec RECORD;
BEGIN
    PERFORM upsert_module_masters_for_source(p_source_table);
    PERFORM upsert_module_details_for_source(p_source_table);

    FOR child_rec IN
        SELECT source_table
        FROM module_master_source_catalog
        WHERE parent_table = p_source_table
          AND module_table_exists(source_table)
        ORDER BY source_table
    LOOP
        PERFORM upsert_module_details_for_source(child_rec.source_table);
    END LOOP;
END;
$$;

CREATE OR REPLACE FUNCTION refresh_all_module_master_detail()
RETURNS VOID
LANGUAGE plpgsql
AS $$
DECLARE
    rec RECORD;
BEGIN
    FOR rec IN
        SELECT source_table
        FROM module_master_source_catalog
        WHERE module_table_exists(source_table)
        ORDER BY relation_mode, source_table
    LOOP
        PERFORM refresh_module_source(rec.source_table);
    END LOOP;
END;
$$;

SELECT refresh_all_module_master_detail();

CREATE OR REPLACE FUNCTION refresh_module_source_trigger()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    PERFORM refresh_module_source(TG_TABLE_NAME);
    RETURN NULL;
END;
$$;

DO $$
DECLARE
    rec RECORD;
    v_trigger_name TEXT;
BEGIN
    FOR rec IN
        SELECT source_table
        FROM module_master_source_catalog
        WHERE module_table_exists(source_table)
        ORDER BY source_table
    LOOP
        v_trigger_name := 'trg_refresh_' || rec.source_table || '_module_master';
        EXECUTE FORMAT('DROP TRIGGER IF EXISTS %I ON %I', v_trigger_name, rec.source_table);
        EXECUTE FORMAT(
            'CREATE TRIGGER %I
             AFTER INSERT OR UPDATE OR DELETE ON %I
             FOR EACH STATEMENT EXECUTE FUNCTION refresh_module_source_trigger()',
            v_trigger_name,
            rec.source_table
        );
    END LOOP;
END;
$$;

CREATE TABLE IF NOT EXISTS module_master_migration_audit (
    module_name            TEXT NOT NULL,
    source_table           TEXT NOT NULL PRIMARY KEY,
    entity_type            TEXT NOT NULL,
    source_count           BIGINT NOT NULL DEFAULT 0,
    master_count           BIGINT NOT NULL DEFAULT 0,
    detail_count           BIGINT NOT NULL DEFAULT 0,
    status                 TEXT NOT NULL DEFAULT 'pending',
    checked_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE OR REPLACE FUNCTION refresh_module_master_migration_audit()
RETURNS VOID
LANGUAGE plpgsql
AS $$
DECLARE
    rec RECORD;
    v_source_count BIGINT;
    v_master_count BIGINT;
    v_detail_count BIGINT;
BEGIN
    FOR rec IN
        SELECT *
        FROM module_master_source_catalog
        WHERE module_table_exists(source_table)
        ORDER BY source_table
    LOOP
        EXECUTE FORMAT('SELECT COUNT(*) FROM %I', rec.source_table) INTO v_source_count;
        EXECUTE FORMAT('SELECT COUNT(*) FROM %I WHERE legacy_table = $1', rec.module_name || '_masters')
            INTO v_master_count
            USING rec.source_table;

        IF rec.relation_mode = 'detail' THEN
            EXECUTE FORMAT('SELECT COUNT(*) FROM %I WHERE legacy_table = $1', rec.module_name || '_details')
                INTO v_detail_count
                USING rec.source_table;
        ELSE
            v_detail_count := 0;
        END IF;

        INSERT INTO module_master_migration_audit(
            module_name, source_table, entity_type, source_count, master_count, detail_count, status, checked_at
        )
        VALUES (
            rec.module_name,
            rec.source_table,
            rec.entity_type,
            v_source_count,
            v_master_count,
            v_detail_count,
            CASE
                WHEN v_source_count = v_master_count
                 AND (rec.relation_mode = 'master' OR v_source_count = v_detail_count)
                THEN 'ok'
                ELSE 'mismatch'
            END,
            NOW()
        )
        ON CONFLICT (source_table) DO UPDATE SET
            module_name = EXCLUDED.module_name,
            entity_type = EXCLUDED.entity_type,
            source_count = EXCLUDED.source_count,
            master_count = EXCLUDED.master_count,
            detail_count = EXCLUDED.detail_count,
            status = EXCLUDED.status,
            checked_at = EXCLUDED.checked_at;
    END LOOP;
END;
$$;

SELECT refresh_module_master_migration_audit();

CREATE OR REPLACE FUNCTION resolve_legacy_uuid(p_source_table TEXT, p_key TEXT)
RETURNS UUID
LANGUAGE plpgsql
STABLE
AS $$
DECLARE
    v_id UUID;
BEGIN
    IF p_key IS NULL OR p_key = '' THEN
        RETURN NULL;
    END IF;

    IF p_key ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$' THEN
        RETURN p_key::UUID;
    END IF;

    IF NOT module_table_exists(p_source_table) OR NOT module_column_exists(p_source_table, 'master_key') THEN
        RETURN NULL;
    END IF;

    EXECUTE FORMAT('SELECT legacy_id FROM %I WHERE master_key = $1 LIMIT 1', p_source_table)
    INTO v_id
    USING p_key;

    RETURN v_id;
END;
$$;

INSERT INTO data_field_catalog(table_name, field_name, data_type, display_name, is_master_key, is_sub_key, is_visible_default, display_order, metadata)
SELECT
    c.table_name,
    c.column_name,
    c.data_type,
    c.column_name,
    c.column_name = 'master_key',
    c.column_name = 'sub_key',
    c.column_name NOT IN ('core_data', 'payload', 'metadata'),
    c.ordinal_position,
    jsonb_build_object('canonical', true)
FROM information_schema.columns c
JOIN data_table_catalog t ON t.table_name = c.table_name
WHERE c.table_schema = 'public'
  AND t.category = 'canonical'
ON CONFLICT (table_name, field_name) DO UPDATE SET
    data_type = EXCLUDED.data_type,
    display_name = EXCLUDED.display_name,
    is_master_key = EXCLUDED.is_master_key,
    is_sub_key = EXCLUDED.is_sub_key,
    is_visible_default = EXCLUDED.is_visible_default,
    display_order = EXCLUDED.display_order,
    metadata = data_field_catalog.metadata || EXCLUDED.metadata,
    updated_at = NOW();


-- -----------------------------------------------------------------------------
-- ERP code-table supply-chain governance
-- -----------------------------------------------------------------------------
-- Fresh tenant databases no longer create the old semantic supply-chain tables
-- such as inventory_counts, inventory_transfers, purchase_orders, or
-- sales_shipments. Procurement, sales, inventory, warehouse, and retail industry
-- flows use ERP code-table master/detail pairs registered in MREG, for example
-- MPOR/POR1, MRDR/RDR1, MITW/ITW1, MRPS/RPS1, MDRQ/DRQ1, MDSP/DSP1,
-- MDRC/DRC1, MCNT/CNT1, and MSPR/SPR1.

INSERT INTO module_master_source_catalog(module_name, source_table, entity_type, relation_mode, parent_table, parent_fk, key_prefix, metadata)
VALUES
    ('inventory', 'MITM', 'item', 'master', NULL, NULL, 'ITM', '{"code_table":true,"supply_chain":true}'::jsonb),
    ('inventory', 'MITW', 'item_warehouse', 'master', NULL, NULL, 'ITW', '{"code_table":true,"supply_chain":true}'::jsonb),
    ('inventory', 'MWHS', 'warehouse', 'master', NULL, NULL, 'WHS', '{"code_table":true,"supply_chain":true}'::jsonb),
    ('procurement', 'MPOR', 'purchase_order', 'master', NULL, NULL, 'POR', '{"code_table":true,"supply_chain":true}'::jsonb),
    ('procurement', 'POR1', 'purchase_order_line', 'detail', 'MPOR', 'DocEntry', 'POR', '{"code_table":true,"supply_chain":true}'::jsonb),
    ('sales', 'MRDR', 'sales_order', 'master', NULL, NULL, 'RDR', '{"code_table":true,"supply_chain":true}'::jsonb),
    ('sales', 'RDR1', 'sales_order_line', 'detail', 'MRDR', 'DocEntry', 'RDR', '{"code_table":true,"supply_chain":true}'::jsonb),
    ('retail', 'MRPS', 'pos_sale', 'master', NULL, NULL, 'RPS', '{"code_table":true,"industry_solution":"retail_distribution_v1"}'::jsonb),
    ('retail', 'RPS1', 'pos_sale_line', 'detail', 'MRPS', 'DocEntry', 'RPS', '{"code_table":true,"industry_solution":"retail_distribution_v1"}'::jsonb),
    ('retail', 'MDRQ', 'distribution_request', 'master', NULL, NULL, 'DRQ', '{"code_table":true,"industry_solution":"retail_distribution_v1"}'::jsonb),
    ('retail', 'DRQ1', 'distribution_request_line', 'detail', 'MDRQ', 'DocEntry', 'DRQ', '{"code_table":true,"industry_solution":"retail_distribution_v1"}'::jsonb),
    ('retail', 'MDSP', 'distribution_shipment', 'master', NULL, NULL, 'DSP', '{"code_table":true,"industry_solution":"retail_distribution_v1"}'::jsonb),
    ('retail', 'DSP1', 'distribution_shipment_line', 'detail', 'MDSP', 'DocEntry', 'DSP', '{"code_table":true,"industry_solution":"retail_distribution_v1"}'::jsonb),
    ('retail', 'MDRC', 'distribution_receipt', 'master', NULL, NULL, 'DRC', '{"code_table":true,"industry_solution":"retail_distribution_v1"}'::jsonb),
    ('retail', 'DRC1', 'distribution_receipt_line', 'detail', 'MDRC', 'DocEntry', 'DRC', '{"code_table":true,"industry_solution":"retail_distribution_v1"}'::jsonb),
    ('retail', 'MCNT', 'store_inventory_count', 'master', NULL, NULL, 'CNT', '{"code_table":true,"industry_solution":"retail_distribution_v1"}'::jsonb),
    ('retail', 'CNT1', 'store_inventory_count_line', 'detail', 'MCNT', 'DocEntry', 'CNT', '{"code_table":true,"industry_solution":"retail_distribution_v1"}'::jsonb),
    ('retail', 'MSPR', 'special_purchase_request', 'master', NULL, NULL, 'SPR', '{"code_table":true,"industry_solution":"retail_distribution_v1"}'::jsonb),
    ('retail', 'SPR1', 'special_purchase_request_line', 'detail', 'MSPR', 'DocEntry', 'SPR', '{"code_table":true,"industry_solution":"retail_distribution_v1"}'::jsonb)
ON CONFLICT (source_table) DO UPDATE SET
    module_name = EXCLUDED.module_name,
    entity_type = EXCLUDED.entity_type,
    relation_mode = EXCLUDED.relation_mode,
    parent_table = EXCLUDED.parent_table,
    parent_fk = EXCLUDED.parent_fk,
    key_prefix = EXCLUDED.key_prefix,
    metadata = module_master_source_catalog.metadata || EXCLUDED.metadata,
    updated_at = NOW();

INSERT INTO saas_modules(module_key, display_name, category, enabled_default, license_scope, metadata)
VALUES
    ('procurement', 'Procurement', 'business', true, 'commercial', '{"code_table_supply_chain":true}'::jsonb),
    ('sales', 'Sales', 'business', true, 'commercial', '{"code_table_supply_chain":true}'::jsonb),
    ('inventory', 'Inventory', 'business', true, 'commercial', '{"code_table_supply_chain":true}'::jsonb),
    ('retail', 'Retail Distribution', 'industry', true, 'commercial', '{"industry_solution":"retail_distribution_v1","code_table_only":true}'::jsonb)
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
        SELECT p.id, m.module_key
        FROM saas_plans p
        JOIN saas_modules m ON m.module_key IN ('procurement', 'sales', 'inventory', 'retail')
        WHERE p.code = 'foundation'
        ON CONFLICT (plan_id, module_key) DO NOTHING;
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS uq_finance_receivables_sales_shipment_source
    ON finance_receivables(source_type, source_id)
    WHERE source_type = 'sales_shipment'
      AND source_id IS NOT NULL;


