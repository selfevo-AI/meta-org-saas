-- platformdb:accept-checksum-drift 001_erp_code_baseline.sql
-- ERP is the authoritative business object store. These views contain no
-- duplicate journal data; legacy finance rows are retained for reconciliation.
SELECT create_erp_master('MVPM', 'DocEntry');
SELECT create_erp_child('VPM1', 'MVPM', 'DocEntry');
INSERT INTO "MREG" ("Code", "Name", "Module", "PrimaryKey", "Kind", "ParentCode") VALUES
    ('MVPM', 'Outgoing Payments', 'finance', 'DocEntry', 'master', ''),
    ('VPM1', 'Outgoing Payment Allocations', 'finance', 'DocEntry', 'child', 'MVPM')
ON CONFLICT ("Code") DO NOTHING;

-- Canonical tenant codes are unique. Ambiguous legacy ownership must be
-- reconciled explicitly instead of silently merging distinct accounts.
DO $$
BEGIN
    IF EXISTS (SELECT account_code FROM gl_accounts GROUP BY account_code HAVING COUNT(*) > 1)
        OR EXISTS (SELECT cost_center_code FROM gl_cost_centers GROUP BY cost_center_code HAVING COUNT(*) > 1) THEN
        RAISE EXCEPTION 'Ontology ledger upgrade requires reconciliation of duplicate legacy account or cost-center codes across organizations';
    END IF;
    IF EXISTS (
        SELECT 1 FROM gl_accounts g JOIN "MACT" a ON a."AcctCode" = g.account_code
        WHERE (NULLIF(a."Payload"->>'LegacyID','') IS NOT NULL AND a."Payload"->>'LegacyID' <> g.id::text)
           OR (NULLIF(a."Payload"->>'OrganizationID','') IS NOT NULL AND a."Payload"->>'OrganizationID' IS DISTINCT FROM g.organization_id::text)
    ) OR EXISTS (
        SELECT 1 FROM gl_cost_centers g JOIN "MPRC" c ON c."PrcCode" = g.cost_center_code
        WHERE (NULLIF(c."Payload"->>'LegacyID','') IS NOT NULL AND c."Payload"->>'LegacyID' <> g.id::text)
           OR (NULLIF(c."Payload"->>'OrganizationID','') IS NOT NULL AND c."Payload"->>'OrganizationID' IS DISTINCT FROM g.organization_id::text)
    ) OR EXISTS (
        SELECT 1 FROM gl_journal_entries g JOIN "MJDT" e ON e."TransId" = 'GL-' || g.id::text
        WHERE e."Payload"->>'LegacyID' IS DISTINCT FROM g.id::text
    ) THEN
        RAISE EXCEPTION 'Ontology ledger upgrade found conflicting canonical and legacy identities; reconcile ownership before retrying';
    END IF;
END $$;

INSERT INTO "MACT" ("AcctCode", "Payload", "CreatedAt", "UpdatedAt")
SELECT account_code, jsonb_build_object('LegacyID', id, 'Name', name, 'AccountType', account_type,
    'Currency', currency, 'ParentAcctCode', parent_account_code,
    'Postable', CASE WHEN postable THEN 'Y' ELSE 'N' END, 'Active', CASE WHEN active THEN 'Y' ELSE 'N' END,
    'OrganizationID', organization_id, 'DepartmentID', department_id, 'Metadata', metadata), created_at, updated_at
FROM gl_accounts ON CONFLICT ("AcctCode") DO UPDATE
SET "Payload" = EXCLUDED."Payload" || "MACT"."Payload"
    || jsonb_build_object('LegacyID', EXCLUDED."Payload"->'LegacyID');

INSERT INTO "MPRC" ("PrcCode", "Payload", "CreatedAt", "UpdatedAt")
SELECT cost_center_code, jsonb_build_object('LegacyID', id, 'Name', name,
    'Active', CASE WHEN active THEN 'Y' ELSE 'N' END, 'OrganizationID', organization_id,
    'DepartmentID', department_id, 'Metadata', metadata), created_at, updated_at
FROM gl_cost_centers ON CONFLICT ("PrcCode") DO UPDATE
SET "Payload" = EXCLUDED."Payload" || "MPRC"."Payload"
    || jsonb_build_object('LegacyID', EXCLUDED."Payload"->'LegacyID');

INSERT INTO "MJDT" ("TransId", "Payload", "CreatedAt", "UpdatedAt")
SELECT 'GL-' || id::text, jsonb_build_object('LegacyID', id, 'LegacyMasterKey', master_key,
    'EntryNumber', entry_number, 'RefDate', reference_date, 'Memo', memo,
    'BtfStatus', CASE status WHEN 'posted' THEN 'P' WHEN 'void' THEN 'V' ELSE 'O' END,
    'Posted', CASE WHEN status = 'posted' THEN 'Y' ELSE 'N' END, 'PostedAt', posted_at,
    'Currency', currency, 'SourceType', source_type, 'SourceID', source_id,
    'OrganizationID', organization_id, 'DepartmentID', department_id, 'Metadata', metadata), created_at, updated_at
FROM gl_journal_entries ON CONFLICT ("TransId") DO NOTHING;

INSERT INTO "JDT1" ("TransId", "LineNum", "Payload", "CreatedAt", "UpdatedAt")
SELECT 'GL-' || entry_id::text, line_num, jsonb_build_object('LegacyID', id, 'AccountCode', account_code,
    'AccountName', account_name, 'CostCenterCode', cost_center_code, 'Debit', debit, 'Credit', credit,
    'Description', description, 'Metadata', metadata), created_at, created_at
FROM gl_journal_entry_lines ON CONFLICT ("TransId", "LineNum") DO NOTHING;

INSERT INTO "MACT" ("AcctCode", "Payload")
SELECT code, jsonb_build_object('Name', name_en, 'AccountType', account_type, 'Currency', '',
    'Postable', 'Y', 'Active', 'Y', 'Metadata', jsonb_build_object('label_en', name_en, 'label_zh', name_zh))
FROM (VALUES
    ('1002', 'Bank', '银行存款', 'asset'),
    ('1122', 'Accounts receivable', '应收账款', 'asset'),
    ('1401', 'Work in process', '在制品', 'asset'),
    ('1405', 'Inventory', '库存商品', 'asset'),
    ('2202-GRNI', 'Goods received not invoiced', '暂估应付', 'liability'),
    ('2202', 'Accounts payable', '应付账款', 'liability'),
    ('2221-IN', 'Input VAT', '进项税额', 'asset'),
    ('2221-OUT', 'Output VAT', '销项税额', 'liability'),
    ('6001', 'Sales revenue', '主营业务收入', 'revenue'),
    ('6401', 'Cost of sales', '主营业务成本', 'expense'),
    ('6602', 'Purchase expense', '采购费用', 'expense'),
    ('6901', 'Inventory adjustments', '存货调整', 'expense')
) AS accounts(code, name_en, name_zh, account_type)
ON CONFLICT ("AcctCode") DO NOTHING;

CREATE OR REPLACE VIEW erp_gl_accounts AS
SELECT COALESCE(NULLIF("Payload"->>'LegacyID','')::uuid, md5('MACT:' || "AcctCode")::uuid) AS id,
    "AcctCode"::text AS master_key, "AcctCode"::text AS account_code,
    COALESCE("Payload"->>'Name', "AcctCode") AS name,
    COALESCE("Payload"->>'AccountType', 'expense') AS account_type,
    COALESCE("Payload"->>'Currency', 'CNY') AS currency,
    COALESCE("Payload"->>'ParentAcctCode', '') AS parent_account_code,
    COALESCE("Payload"->>'Postable', 'Y') NOT IN ('N','false') AS postable,
    COALESCE("Payload"->>'Active', 'Y') NOT IN ('N','false') AS active,
    NULLIF("Payload"->>'OrganizationID','')::uuid AS organization_id,
    NULLIF("Payload"->>'DepartmentID','')::uuid AS department_id,
    COALESCE(NULLIF("Payload"->'Metadata', 'null'::jsonb), '{}'::jsonb) AS metadata, "CreatedAt" AS created_at, "UpdatedAt" AS updated_at
FROM "MACT";

CREATE OR REPLACE VIEW erp_gl_cost_centers AS
SELECT COALESCE(NULLIF("Payload"->>'LegacyID','')::uuid, md5('MPRC:' || "PrcCode")::uuid) AS id,
    "PrcCode"::text AS master_key, "PrcCode"::text AS cost_center_code,
    COALESCE("Payload"->>'Name', "PrcCode") AS name,
    COALESCE("Payload"->>'Active', 'Y') NOT IN ('N','false') AS active,
    NULLIF("Payload"->>'OrganizationID','')::uuid AS organization_id,
    NULLIF("Payload"->>'DepartmentID','')::uuid AS department_id,
    COALESCE(NULLIF("Payload"->'Metadata', 'null'::jsonb), '{}'::jsonb) AS metadata, "CreatedAt" AS created_at, "UpdatedAt" AS updated_at
FROM "MPRC";

CREATE OR REPLACE VIEW erp_gl_journal_entries AS
SELECT COALESCE(NULLIF("Payload"->>'LegacyID','')::uuid, md5('MJDT:' || "TransId")::uuid) AS id,
    "TransId"::text AS master_key, COALESCE(NULLIF("Payload"->>'EntryNumber',''), "TransId") AS entry_number,
    COALESCE(NULLIF("Payload"->>'RefDate','')::date, "CreatedAt"::date) AS reference_date,
    COALESCE("Payload"->>'Memo', '') AS memo,
    CASE WHEN "Payload"->>'BtfStatus' = 'V' THEN 'void'
         WHEN "Payload"->>'BtfStatus' = 'P' OR "Payload"->>'Posted' = 'Y' THEN 'posted' ELSE 'draft' END AS status,
    COALESCE(NULLIF("Payload"->>'Currency',''), NULLIF("Payload"->>'DocCur',''), 'CNY') AS currency,
    COALESCE("Payload"->>'SourceType', "Payload"->>'BaseTable', '') AS source_type,
    NULLIF("Payload"->>'SourceID','')::uuid AS source_id,
    NULLIF("Payload"->>'OrganizationID','')::uuid AS organization_id,
    NULLIF("Payload"->>'DepartmentID','')::uuid AS department_id,
    COALESCE(NULLIF("Payload"->'Metadata', 'null'::jsonb), '{}'::jsonb) || jsonb_build_object('base_table', "Payload"->>'BaseTable', 'base_key', "Payload"->>'BaseEntry') AS metadata,
    NULLIF("Payload"->>'PostedAt','')::timestamptz AS posted_at,
    "CreatedAt" AS created_at, "UpdatedAt" AS updated_at
FROM "MJDT";

CREATE OR REPLACE VIEW erp_gl_journal_entry_lines AS
SELECT COALESCE(NULLIF(l."Payload"->>'LegacyID','')::uuid, md5('JDT1:' || l."TransId" || ':' || l."LineNum")::uuid) AS id,
    e.id AS entry_id, l."LineNum" AS line_num,
    COALESCE(l."Payload"->>'AccountCode', l."Payload"->>'Account', '') AS account_code,
    COALESCE(l."Payload"->>'AccountName', a."Payload"->>'Name', '') AS account_name,
    COALESCE(l."Payload"->>'CostCenterCode', '') AS cost_center_code,
    COALESCE((l."Payload"->>'Debit')::numeric,0) AS debit,
    COALESCE((l."Payload"->>'Credit')::numeric,0) AS credit,
    COALESCE(l."Payload"->>'Description', '') AS description,
    COALESCE(NULLIF(l."Payload"->'Metadata', 'null'::jsonb), '{}'::jsonb) AS metadata, l."CreatedAt" AS created_at
FROM "JDT1" l JOIN erp_gl_journal_entries e ON e.master_key = l."TransId"
LEFT JOIN "MACT" a ON a."AcctCode" = COALESCE(l."Payload"->>'AccountCode', l."Payload"->>'Account');

CREATE INDEX IF NOT EXISTS idx_mjdt_posted_currency_date ON "MJDT"
    (("Payload"->>'Currency'), ("Payload"->>'RefDate'))
    WHERE "Payload"->>'BtfStatus' = 'P' OR "Payload"->>'Posted' = 'Y';
CREATE INDEX IF NOT EXISTS idx_mjdt_object_id ON "MJDT" ((md5('MJDT:' || "TransId")::uuid));
CREATE INDEX IF NOT EXISTS idx_mjdt_legacy_id ON "MJDT" (("Payload"->>'LegacyID'));

UPDATE "MREG" r SET "Metadata" = r."Metadata" || jsonb_build_object(
    'ontology', jsonb_build_object('object_type', objects.object_type, 'authoritative', true, 'version', 1))
FROM (VALUES
    ('MCRD','business_partner'), ('MITM','item'), ('MWHS','warehouse'), ('MITW','stock_balance'),
    ('MPOR','purchase_order'), ('MPDN','goods_receipt'), ('MPCH','payable_invoice'), ('MVPM','outgoing_payment'),
    ('MRDR','sales_order'), ('MDLN','delivery'), ('MINV','receivable_invoice'), ('MRCT','incoming_payment'),
    ('MIGN','inventory_receipt'), ('MIGE','inventory_issue'), ('MACT','account'), ('MJDT','journal_entry'),
    ('MPRC','cost_center'), ('MPRJ','project'), ('MREQ','requirement')
) AS objects(code, object_type) WHERE r."Code" = objects.code;

-- Historical industry extensions have no ledger-safe mutation adapter.
UPDATE "MREG" SET "Metadata" = "Metadata" || jsonb_build_object(
    'read_only', true, 'operational_status', 'legacy_read_only')
WHERE "Module" IN ('retail', 'manufacturing');
UPDATE "MREG" SET "Metadata" = "Metadata" || jsonb_build_object('read_only', true)
WHERE "Code" = 'MITW';
