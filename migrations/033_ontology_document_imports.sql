-- platformdb:accept-checksum-drift 001_erp_code_baseline.sql
-- Ontology contract and external-document evidence. ERP rows remain authoritative.
CREATE TABLE IF NOT EXISTS ontology_object_types (
    key TEXT PRIMARY KEY,
    table_code TEXT NOT NULL UNIQUE REFERENCES "MREG"("Code"),
    primary_key TEXT NOT NULL,
    module TEXT NOT NULL,
    label_zh TEXT NOT NULL,
    label_en TEXT NOT NULL,
    importable BOOLEAN NOT NULL DEFAULT FALSE,
    schema_version INTEGER NOT NULL DEFAULT 1 CHECK (schema_version > 0)
);
CREATE TABLE IF NOT EXISTS ontology_properties (
    object_type TEXT NOT NULL REFERENCES ontology_object_types(key),
    key TEXT NOT NULL,
    source_field TEXT NOT NULL,
    data_type TEXT NOT NULL CHECK (data_type IN ('string', 'decimal', 'date', 'boolean')),
    label_zh TEXT NOT NULL,
    label_en TEXT NOT NULL,
    ordinal INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (object_type, key)
);
CREATE TABLE IF NOT EXISTS ontology_link_types (
    object_type TEXT NOT NULL REFERENCES ontology_object_types(key),
    key TEXT NOT NULL,
    target_type TEXT REFERENCES ontology_object_types(key),
    cardinality TEXT NOT NULL CHECK (cardinality IN ('one', 'many')),
    source_field TEXT NOT NULL,
    child_table TEXT NOT NULL DEFAULT '',
    inverse BOOLEAN NOT NULL DEFAULT FALSE,
    polymorphic BOOLEAN NOT NULL DEFAULT FALSE,
    label_zh TEXT NOT NULL,
    label_en TEXT NOT NULL,
    PRIMARY KEY (object_type, key),
    CHECK (target_type IS NOT NULL OR polymorphic)
);
CREATE TABLE IF NOT EXISTS ontology_action_types (
    object_type TEXT NOT NULL REFERENCES ontology_object_types(key),
    key TEXT NOT NULL,
    label_zh TEXT NOT NULL,
    label_en TEXT NOT NULL,
    requires_approval BOOLEAN NOT NULL DEFAULT TRUE CHECK (requires_approval),
    parameters JSONB NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(parameters) = 'array'),
    PRIMARY KEY (object_type, key)
);
INSERT INTO ontology_object_types (key, table_code, primary_key, module, label_zh, label_en, importable)
VALUES
 ('business_partner','MCRD','CardCode','inventory','业务伙伴','Business Partner',FALSE),
 ('item','MITM','ItemCode','inventory','物料','Item',FALSE),
 ('warehouse','MWHS','WhsCode','inventory','仓库','Warehouse',FALSE),
 ('stock_balance','MITW','ItemCode','inventory','库存余额','Stock Balance',FALSE),
 ('purchase_order','MPOR','DocEntry','procurement','采购订单','Purchase Order',TRUE),
 ('goods_receipt','MPDN','DocEntry','procurement','采购收货','Goods Receipt',TRUE),
 ('payable_invoice','MPCH','DocEntry','finance','应付发票','Payable Invoice',TRUE),
 ('outgoing_payment','MVPM','DocEntry','finance','付款单','Outgoing Payment',TRUE),
 ('sales_order','MRDR','DocEntry','sales','销售订单','Sales Order',TRUE),
 ('delivery','MDLN','DocEntry','sales','销售交货','Delivery',TRUE),
 ('receivable_invoice','MINV','DocEntry','finance','应收发票','Receivable Invoice',TRUE),
 ('incoming_payment','MRCT','DocEntry','finance','收款单','Incoming Payment',TRUE),
 ('inventory_receipt','MIGN','DocEntry','inventory','库存入库','Inventory Receipt',TRUE),
 ('inventory_issue','MIGE','DocEntry','inventory','库存出库','Inventory Issue',TRUE),
 ('account','MACT','AcctCode','finance','会计科目','Account',FALSE),
 ('journal_entry','MJDT','TransId','finance','会计分录','Journal Entry',FALSE),
 ('cost_center','MPRC','PrcCode','finance','成本中心','Cost Center',FALSE),
 ('project','MPRJ','PrjCode','project','项目','Project',FALSE),
 ('requirement','MREQ','ReqCode','project','需求','Requirement',FALSE)
ON CONFLICT (key) DO NOTHING;
INSERT INTO ontology_properties (object_type, key, source_field, data_type, label_zh, label_en, ordinal)
SELECT key, 'key', primary_key, 'string', '编号', 'Key', 0 FROM ontology_object_types
ON CONFLICT DO NOTHING;
INSERT INTO ontology_properties (object_type, key, source_field, data_type, label_zh, label_en, ordinal)
SELECT t.key, p.key, p.field, p.kind, p.zh, p.en, p.n FROM ontology_object_types t
CROSS JOIN (VALUES
 ('partner','CardCode','string','业务伙伴','Business Partner',1),
 ('date','DocDate','date','单据日期','Document Date',2),
 ('due_date','DocDueDate','date','到期日期','Due Date',3),
 ('currency','DocCur','string','币种','Currency',4),
 ('total','DocTotal','decimal','含税金额','Total',5),
 ('tax','VatSum','decimal','税额','Tax',6),
 ('status','DocStatus','string','状态','Status',7),
 ('approval_status','WddStatus','string','审批状态','Approval Status',8),
 ('posted','Posted','string','已过账','Posted',9),
 ('external_number','NumAtCard','string','外部单号','External Reference',10),
 ('note','Comments','string','备注','Notes',11)
) p(key,field,kind,zh,en,n) WHERE t.primary_key = 'DocEntry'
ON CONFLICT DO NOTHING;
INSERT INTO ontology_properties (object_type, key, source_field, data_type, label_zh, label_en, ordinal)
SELECT p.typ,p.key,p.field,p.kind,p.zh,p.en,20 + p.n FROM (VALUES
 ('business_partner','name','CardName','string','名称','Name',1),
 ('business_partner','partner_type','CardType','string','伙伴类型','Partner Type',2),
 ('item','name','ItemName','string','名称','Name',1),
 ('item','unit','InvntryUom','string','计量单位','Unit',2),
 ('warehouse','name','WhsName','string','名称','Name',1),
 ('stock_balance','item','BaseItemCode','string','物料','Item',1),
 ('stock_balance','warehouse','WhsCode','string','仓库','Warehouse',2),
 ('stock_balance','quantity','OnHand','decimal','现存量','On Hand',3),
 ('stock_balance','value','InventoryValue','decimal','存货价值','Inventory Value',4),
 ('stock_balance','average_cost','AvgPrice','decimal','平均成本','Average Cost',5),
 ('stock_balance','currency','Currency','string','币种','Currency',6),
 ('receivable_invoice','paid','PaidToDate','decimal','已结算','Settled',1),
 ('payable_invoice','paid','PaidToDate','decimal','已结算','Settled',1),
 ('incoming_payment','allocated','AllocatedAmount','decimal','已核销','Allocated',1),
 ('incoming_payment','open_balance','OpenBal','decimal','未核销金额','Open Balance',2),
 ('outgoing_payment','allocated','AllocatedAmount','decimal','已核销','Allocated',1),
 ('outgoing_payment','open_balance','OpenBal','decimal','未核销金额','Open Balance',2),
 ('journal_entry','name','Memo','string','摘要','Memo',1),
 ('journal_entry','date','RefDate','date','记账日期','Posting Date',2),
 ('journal_entry','currency','Currency','string','币种','Currency',3),
 ('journal_entry','status','BtfStatus','string','状态','Status',4),
 ('account','name','Name','string','名称','Name',1),
 ('cost_center','name','Name','string','名称','Name',1),
 ('project','name','Name','string','名称','Name',1),
 ('requirement','name','Name','string','名称','Name',1)
) p(typ,key,field,kind,zh,en,n) ON CONFLICT DO NOTHING;
INSERT INTO ontology_link_types (object_type,key,target_type,cardinality,source_field,label_zh,label_en)
SELECT key,'partner','business_partner','one','CardCode','业务伙伴','Business Partner'
FROM ontology_object_types WHERE primary_key = 'DocEntry' ON CONFLICT DO NOTHING;
INSERT INTO ontology_link_types (object_type,key,target_type,cardinality,source_field,inverse,polymorphic,label_zh,label_en)
SELECT key,'source',NULL,'one','BaseEntry',FALSE,TRUE,'来源单据','Source Document'
FROM ontology_object_types WHERE primary_key = 'DocEntry' OR key = 'journal_entry' ON CONFLICT DO NOTHING;
INSERT INTO ontology_link_types (object_type,key,target_type,cardinality,source_field,inverse,polymorphic,label_zh,label_en)
SELECT key,'journals','journal_entry','many','BaseEntry',TRUE,TRUE,'会计分录','Journal Entries'
FROM ontology_object_types WHERE primary_key = 'DocEntry' ON CONFLICT DO NOTHING;
INSERT INTO ontology_link_types (object_type,key,target_type,cardinality,source_field,child_table,label_zh,label_en)
SELECT p.typ,l.key,l.target,'many',l.field,p.child,l.zh,l.en
FROM (VALUES ('purchase_order','POR1'),('goods_receipt','PDN1'),('payable_invoice','PCH1'),
 ('sales_order','RDR1'),('delivery','DLN1'),('receivable_invoice','INV1'),
 ('inventory_receipt','IGN1'),('inventory_issue','IGE1')) p(typ,child)
CROSS JOIN (VALUES ('items','item','ItemCode','物料','Items'),
 ('warehouses','warehouse','WhsCode','仓库','Warehouses')) l(key,target,field,zh,en)
ON CONFLICT DO NOTHING;
INSERT INTO ontology_link_types (object_type,key,target_type,cardinality,source_field,child_table,inverse,label_zh,label_en)
VALUES
 ('purchase_order','fulfillment','goods_receipt','one','FulfillmentEntry','',FALSE,'收货单','Receipt'),
 ('sales_order','fulfillment','delivery','one','FulfillmentEntry','',FALSE,'交货单','Delivery'),
 ('goods_receipt','invoice','payable_invoice','one','InvoiceEntry','',FALSE,'应付发票','Payable Invoice'),
 ('delivery','invoice','receivable_invoice','one','InvoiceEntry','',FALSE,'应收发票','Receivable Invoice'),
 ('payable_invoice','payments','outgoing_payment','many','TargetKey','VPM1',TRUE,'收付款','Payments'),
 ('receivable_invoice','payments','incoming_payment','many','TargetKey','RCT1',TRUE,'收付款','Payments'),
 ('outgoing_payment','invoices','payable_invoice','many','TargetKey','VPM1',FALSE,'核销发票','Allocated Invoices'),
 ('incoming_payment','invoices','receivable_invoice','many','TargetKey','RCT1',FALSE,'核销发票','Allocated Invoices'),
 ('stock_balance','item','item','one','BaseItemCode','',FALSE,'物料','Item'),
 ('stock_balance','warehouse','warehouse','one','WhsCode','',FALSE,'仓库','Warehouse'),
 ('item','stock','stock_balance','many','BaseItemCode','',TRUE,'仓库库存','Warehouse Stock'),
 ('journal_entry','accounts','account','many','AccountCode','JDT1',FALSE,'会计科目','Accounts')
ON CONFLICT DO NOTHING;
INSERT INTO ontology_action_types (object_type,key,label_zh,label_en,parameters)
SELECT a.typ,a.key,a.zh,a.en,
 CASE WHEN a.key = 'allocate' THEN
 '[{"key":"TargetKey","source_field":"TargetKey","data_type":"string","label":{"zh":"发票编号","en":"Invoice Key"}},{"key":"Amount","source_field":"Amount","data_type":"decimal","label":{"zh":"核销金额","en":"Allocation Amount"}}]'::jsonb
 ELSE '[]'::jsonb END
FROM (VALUES
 ('purchase_order','submit','提交审批','Submit'),('purchase_order','approve','批准','Approve'),
 ('purchase_order','receive','生成收货单','Create Receipt'),
 ('goods_receipt','approve','批准','Approve'),('goods_receipt','post','过账','Post'),
 ('payable_invoice','post','过账','Post'),('outgoing_payment','allocate','核销','Allocate'),
 ('sales_order','confirm','确认','Confirm'),('sales_order','approve','批准','Approve'),
 ('sales_order','deliver','生成交货单','Create Delivery'),
 ('delivery','approve','批准','Approve'),('delivery','post','过账','Post'),
 ('receivable_invoice','post','过账','Post'),('incoming_payment','allocate','核销','Allocate'),
 ('inventory_receipt','post','过账','Post'),('inventory_issue','post','过账','Post'),
 ('journal_entry','post','过账','Post'),
 ('requirement','analyze','分析','Analyze'),('requirement','approve','批准','Approve'),
 ('requirement','convert-to-project','创建项目','Create Project'),
 ('project','refresh-cost','刷新项目成本','Refresh Project Cost'),
 ('project','close-feedback','关闭反馈','Close Feedback')
) a(typ,key,zh,en) ON CONFLICT DO NOTHING;

CREATE TABLE IF NOT EXISTS ontology_document_imports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    object_type TEXT NOT NULL REFERENCES ontology_object_types(key),
    source_hash TEXT NOT NULL CHECK (source_hash ~ '^[a-f0-9]{64}$'),
    status TEXT NOT NULL DEFAULT 'uploaded'
        CHECK (status IN ('uploaded','recognizing','needs_review','confirmed','rejected','failed')),
    version INTEGER NOT NULL DEFAULT 1 CHECK (version > 0),
    draft JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(draft) = 'object'),
    extraction JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(extraction) = 'object'),
    recognition_method TEXT NOT NULL DEFAULT '',
    recognition_token UUID,
    recognition_started_at TIMESTAMPTZ,
    invocation_id UUID,
    error_code TEXT NOT NULL DEFAULT '',
    created_by UUID NOT NULL,
    reviewed_by UUID,
    reviewed_at TIMESTAMPTZ,
    confirmed_key TEXT,
    confirmed_version INTEGER,
    confirmation_hash TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (object_type, source_hash),
    CHECK ((status = 'confirmed') = (confirmed_key IS NOT NULL)),
    CHECK (status <> 'confirmed' OR (reviewed_by IS NOT NULL AND reviewed_at IS NOT NULL
        AND confirmed_version IS NOT NULL AND confirmation_hash IS NOT NULL))
);
CREATE INDEX IF NOT EXISTS idx_ontology_import_inbox
    ON ontology_document_imports (object_type, status, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_ontology_import_object
    ON ontology_document_imports (object_type, confirmed_key) WHERE confirmed_key IS NOT NULL;
CREATE TABLE IF NOT EXISTS ontology_source_files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    import_id UUID NOT NULL REFERENCES ontology_document_imports(id),
    name TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 240),
    media_type TEXT NOT NULL CHECK (media_type IN ('image/png','image/jpeg','image/webp','application/pdf',
        'application/vnd.openxmlformats-officedocument.wordprocessingml.document','text/plain','text/csv')),
    byte_size INTEGER NOT NULL CHECK (byte_size BETWEEN 1 AND 10485760),
    sha256 TEXT NOT NULL CHECK (sha256 ~ '^[a-f0-9]{64}$'),
    content BYTEA NOT NULL,
    ordinal INTEGER NOT NULL CHECK (ordinal BETWEEN 0 AND 4),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (import_id, ordinal),
    UNIQUE (import_id, sha256),
    CHECK (octet_length(content) = byte_size)
);
CREATE TABLE IF NOT EXISTS ontology_import_events (
    id BIGSERIAL PRIMARY KEY,
    import_id UUID NOT NULL REFERENCES ontology_document_imports(id),
    version INTEGER NOT NULL,
    event TEXT NOT NULL CHECK (event IN ('uploaded','recognizing','recognized','recognition_failed','reviewed','confirmed','rejected')),
    actor_id UUID NOT NULL,
    snapshot JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(snapshot) = 'object'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (import_id, version)
);
CREATE INDEX IF NOT EXISTS idx_ontology_import_events ON ontology_import_events (import_id,id);

-- A lightweight source edge is resolved from the confirmed import, without copying business state.
CREATE OR REPLACE VIEW ontology_document_sources AS
SELECT i.object_type, i.confirmed_key AS object_key, f.id AS source_id, i.id AS import_id,
       f.name, f.media_type, f.sha256, i.reviewed_by, i.reviewed_at
FROM ontology_document_imports i JOIN ontology_source_files f ON f.import_id = i.id
WHERE i.status = 'confirmed';
