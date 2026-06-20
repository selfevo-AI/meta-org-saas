-- 038_supply_chain_core.sql
-- Procurement, sales, and inventory MVP foundation.
-- Strongly typed ERP-style tables remain the source of truth; the existing
-- catalog/master-detail/context systems index them for workbench and AI use.

CREATE TABLE IF NOT EXISTS business_partners (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    master_key TEXT NOT NULL DEFAULT next_business_key('business_partners', 'BPT'),
    partner_code TEXT NOT NULL DEFAULT '',
    partner_type TEXT NOT NULL
        CHECK (partner_type IN ('supplier', 'customer', 'both', 'carrier', 'other')),
    name TEXT NOT NULL,
    email TEXT NOT NULL DEFAULT '',
    phone TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'disabled', 'archived')),
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    metadata JSONB NOT NULL DEFAULT '{}',
    legacy_id UUID,
    parent_master_table TEXT,
    parent_master_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_business_partners_master_key ON business_partners(master_key);
CREATE UNIQUE INDEX IF NOT EXISTS uq_business_partners_org_code
    ON business_partners(organization_id, partner_code)
    WHERE partner_code <> '';
CREATE INDEX IF NOT EXISTS idx_business_partners_type_status
    ON business_partners(partner_type, status, name);

CREATE TABLE IF NOT EXISTS items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    master_key TEXT NOT NULL DEFAULT next_business_key('items', 'ITM'),
    item_code TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL,
    item_type TEXT NOT NULL DEFAULT 'material'
        CHECK (item_type IN ('material', 'service', 'asset', 'expense')),
    base_uom TEXT NOT NULL DEFAULT 'EA',
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'disabled', 'archived')),
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    metadata JSONB NOT NULL DEFAULT '{}',
    legacy_id UUID,
    parent_master_table TEXT,
    parent_master_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_items_master_key ON items(master_key);
CREATE UNIQUE INDEX IF NOT EXISTS uq_items_org_code
    ON items(organization_id, item_code)
    WHERE item_code <> '';
CREATE INDEX IF NOT EXISTS idx_items_type_status ON items(item_type, status, name);

CREATE TABLE IF NOT EXISTS item_uoms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    master_key TEXT NOT NULL DEFAULT next_business_key('item_uoms', 'IUM'),
    item_id UUID NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    uom TEXT NOT NULL,
    factor NUMERIC(18,8) NOT NULL DEFAULT 1 CHECK (factor > 0),
    is_base BOOLEAN NOT NULL DEFAULT FALSE,
    metadata JSONB NOT NULL DEFAULT '{}',
    legacy_id UUID,
    parent_master_table TEXT,
    parent_master_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (item_id, uom)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_item_uoms_master_key ON item_uoms(master_key);

CREATE TABLE IF NOT EXISTS warehouses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    master_key TEXT NOT NULL DEFAULT next_business_key('warehouses', 'WHS'),
    warehouse_code TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'disabled', 'archived')),
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    department_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    legacy_id UUID,
    parent_master_table TEXT,
    parent_master_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_warehouses_master_key ON warehouses(master_key);
CREATE UNIQUE INDEX IF NOT EXISTS uq_warehouses_org_code
    ON warehouses(organization_id, warehouse_code)
    WHERE warehouse_code <> '';
CREATE INDEX IF NOT EXISTS idx_warehouses_org_status
    ON warehouses(organization_id, status, name);

CREATE TABLE IF NOT EXISTS warehouse_locations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    master_key TEXT NOT NULL DEFAULT next_business_key('warehouse_locations', 'WLC'),
    warehouse_id UUID NOT NULL REFERENCES warehouses(id) ON DELETE CASCADE,
    location_code TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'disabled', 'archived')),
    metadata JSONB NOT NULL DEFAULT '{}',
    legacy_id UUID,
    parent_master_table TEXT,
    parent_master_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_warehouse_locations_master_key ON warehouse_locations(master_key);
CREATE UNIQUE INDEX IF NOT EXISTS uq_warehouse_locations_code
    ON warehouse_locations(warehouse_id, location_code)
    WHERE location_code <> '';

CREATE TABLE IF NOT EXISTS inventory_balances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    master_key TEXT NOT NULL DEFAULT next_business_key('inventory_balances', 'IVB'),
    item_id UUID NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    warehouse_id UUID NOT NULL REFERENCES warehouses(id) ON DELETE CASCADE,
    location_id UUID REFERENCES warehouse_locations(id) ON DELETE SET NULL,
    quantity NUMERIC(18,8) NOT NULL DEFAULT 0,
    reserved_qty NUMERIC(18,8) NOT NULL DEFAULT 0,
    average_cost NUMERIC(18,8) NOT NULL DEFAULT 0,
    value_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'CNY' REFERENCES currencies(code) ON DELETE RESTRICT,
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    metadata JSONB NOT NULL DEFAULT '{}',
    legacy_id UUID,
    parent_master_table TEXT,
    parent_master_key TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (quantity >= 0),
    CHECK (reserved_qty >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_inventory_balances_master_key ON inventory_balances(master_key);
CREATE UNIQUE INDEX IF NOT EXISTS uq_inventory_balances_scope
    ON inventory_balances(item_id, warehouse_id, COALESCE(location_id, '00000000-0000-0000-0000-000000000000'::uuid));
CREATE INDEX IF NOT EXISTS idx_inventory_balances_warehouse
    ON inventory_balances(warehouse_id, item_id);

CREATE TABLE IF NOT EXISTS inventory_movements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    master_key TEXT NOT NULL DEFAULT next_business_key('inventory_movements', 'IVM'),
    movement_type TEXT NOT NULL
        CHECK (movement_type IN (
            'purchase_receipt', 'purchase_return', 'sales_shipment', 'sales_return',
            'transfer_in', 'transfer_out', 'adjustment_in', 'adjustment_out',
            'count_gain', 'count_loss'
        )),
    source_type TEXT NOT NULL DEFAULT '',
    source_id UUID,
    item_id UUID NOT NULL REFERENCES items(id) ON DELETE RESTRICT,
    warehouse_id UUID NOT NULL REFERENCES warehouses(id) ON DELETE RESTRICT,
    location_id UUID REFERENCES warehouse_locations(id) ON DELETE SET NULL,
    quantity NUMERIC(18,8) NOT NULL CHECK (quantity > 0),
    unit_cost NUMERIC(18,8) NOT NULL DEFAULT 0,
    amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'CNY' REFERENCES currencies(code) ON DELETE RESTRICT,
    balance_after NUMERIC(18,8) NOT NULL DEFAULT 0,
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    department_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata JSONB NOT NULL DEFAULT '{}',
    legacy_id UUID,
    parent_master_table TEXT,
    parent_master_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_inventory_movements_master_key ON inventory_movements(master_key);
CREATE INDEX IF NOT EXISTS idx_inventory_movements_item_time
    ON inventory_movements(item_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_inventory_movements_source
    ON inventory_movements(source_type, source_id);

CREATE TABLE IF NOT EXISTS inventory_reservations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    master_key TEXT NOT NULL DEFAULT next_business_key('inventory_reservations', 'IVR'),
    source_type TEXT NOT NULL DEFAULT '',
    source_id UUID,
    item_id UUID NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    warehouse_id UUID NOT NULL REFERENCES warehouses(id) ON DELETE CASCADE,
    quantity NUMERIC(18,8) NOT NULL CHECK (quantity > 0),
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'released', 'consumed', 'cancelled')),
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    metadata JSONB NOT NULL DEFAULT '{}',
    legacy_id UUID,
    parent_master_table TEXT,
    parent_master_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_inventory_reservations_master_key ON inventory_reservations(master_key);
CREATE INDEX IF NOT EXISTS idx_inventory_reservations_source
    ON inventory_reservations(source_type, source_id, status);

CREATE TABLE IF NOT EXISTS inventory_transfers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    master_key TEXT NOT NULL DEFAULT next_business_key('inventory_transfers', 'IVT'),
    transfer_number TEXT NOT NULL DEFAULT '',
    from_warehouse_id UUID NOT NULL REFERENCES warehouses(id) ON DELETE RESTRICT,
    to_warehouse_id UUID NOT NULL REFERENCES warehouses(id) ON DELETE RESTRICT,
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'submitted', 'approved', 'posted', 'void')),
    approval_status TEXT NOT NULL DEFAULT 'not_required'
        CHECK (approval_status IN ('not_required', 'pending', 'approved', 'rejected')),
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    department_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    workflow_instance_id UUID REFERENCES workflow_instances(id) ON DELETE SET NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    legacy_id UUID,
    parent_master_table TEXT,
    parent_master_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (from_warehouse_id <> to_warehouse_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_inventory_transfers_master_key ON inventory_transfers(master_key);

CREATE TABLE IF NOT EXISTS inventory_transfer_lines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    master_key TEXT NOT NULL DEFAULT next_business_key('inventory_transfer_lines', 'ITL'),
    transfer_id UUID NOT NULL REFERENCES inventory_transfers(id) ON DELETE CASCADE,
    item_id UUID NOT NULL REFERENCES items(id) ON DELETE RESTRICT,
    quantity NUMERIC(18,8) NOT NULL CHECK (quantity > 0),
    unit_cost NUMERIC(18,8) NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}',
    legacy_id UUID,
    parent_master_table TEXT,
    parent_master_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_inventory_transfer_lines_master_key ON inventory_transfer_lines(master_key);
CREATE INDEX IF NOT EXISTS idx_inventory_transfer_lines_transfer
    ON inventory_transfer_lines(transfer_id);

CREATE TABLE IF NOT EXISTS inventory_adjustments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    master_key TEXT NOT NULL DEFAULT next_business_key('inventory_adjustments', 'IVA'),
    adjustment_number TEXT NOT NULL DEFAULT '',
    warehouse_id UUID NOT NULL REFERENCES warehouses(id) ON DELETE RESTRICT,
    reason TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'submitted', 'approved', 'posted', 'void')),
    approval_status TEXT NOT NULL DEFAULT 'not_required'
        CHECK (approval_status IN ('not_required', 'pending', 'approved', 'rejected')),
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    department_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    workflow_instance_id UUID REFERENCES workflow_instances(id) ON DELETE SET NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    legacy_id UUID,
    parent_master_table TEXT,
    parent_master_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_inventory_adjustments_master_key ON inventory_adjustments(master_key);

CREATE TABLE IF NOT EXISTS inventory_adjustment_lines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    master_key TEXT NOT NULL DEFAULT next_business_key('inventory_adjustment_lines', 'IAL'),
    adjustment_id UUID NOT NULL REFERENCES inventory_adjustments(id) ON DELETE CASCADE,
    item_id UUID NOT NULL REFERENCES items(id) ON DELETE RESTRICT,
    quantity_delta NUMERIC(18,8) NOT NULL,
    unit_cost NUMERIC(18,8) NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}',
    legacy_id UUID,
    parent_master_table TEXT,
    parent_master_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (quantity_delta <> 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_inventory_adjustment_lines_master_key ON inventory_adjustment_lines(master_key);
CREATE INDEX IF NOT EXISTS idx_inventory_adjustment_lines_adjustment
    ON inventory_adjustment_lines(adjustment_id);

CREATE TABLE IF NOT EXISTS inventory_counts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    master_key TEXT NOT NULL DEFAULT next_business_key('inventory_counts', 'IVC'),
    count_number TEXT NOT NULL DEFAULT '',
    warehouse_id UUID NOT NULL REFERENCES warehouses(id) ON DELETE RESTRICT,
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'submitted', 'approved', 'posted', 'void')),
    approval_status TEXT NOT NULL DEFAULT 'not_required'
        CHECK (approval_status IN ('not_required', 'pending', 'approved', 'rejected')),
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    department_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    workflow_instance_id UUID REFERENCES workflow_instances(id) ON DELETE SET NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    legacy_id UUID,
    parent_master_table TEXT,
    parent_master_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_inventory_counts_master_key ON inventory_counts(master_key);

CREATE TABLE IF NOT EXISTS inventory_count_lines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    master_key TEXT NOT NULL DEFAULT next_business_key('inventory_count_lines', 'ICL'),
    count_id UUID NOT NULL REFERENCES inventory_counts(id) ON DELETE CASCADE,
    item_id UUID NOT NULL REFERENCES items(id) ON DELETE RESTRICT,
    book_qty NUMERIC(18,8) NOT NULL DEFAULT 0,
    counted_qty NUMERIC(18,8) NOT NULL DEFAULT 0,
    variance_qty NUMERIC(18,8) NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}',
    legacy_id UUID,
    parent_master_table TEXT,
    parent_master_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_inventory_count_lines_master_key ON inventory_count_lines(master_key);
CREATE INDEX IF NOT EXISTS idx_inventory_count_lines_count ON inventory_count_lines(count_id);

CREATE TABLE IF NOT EXISTS purchase_requisitions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    master_key TEXT NOT NULL DEFAULT next_business_key('purchase_requisitions', 'PRQ'),
    title TEXT NOT NULL,
    supplier_partner_id UUID REFERENCES business_partners(id) ON DELETE SET NULL,
    supplier_id TEXT NOT NULL DEFAULT '',
    supplier_name TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'submitted', 'approved', 'ordered', 'void')),
    approval_status TEXT NOT NULL DEFAULT 'not_required'
        CHECK (approval_status IN ('not_required', 'pending', 'approved', 'rejected')),
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    department_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    workflow_instance_id UUID REFERENCES workflow_instances(id) ON DELETE SET NULL,
    currency TEXT NOT NULL DEFAULT 'CNY' REFERENCES currencies(code) ON DELETE RESTRICT,
    total_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}',
    legacy_id UUID,
    parent_master_table TEXT,
    parent_master_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_purchase_requisitions_master_key ON purchase_requisitions(master_key);
CREATE INDEX IF NOT EXISTS idx_purchase_requisitions_org_status
    ON purchase_requisitions(organization_id, status, created_at DESC);

CREATE TABLE IF NOT EXISTS purchase_requisition_lines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    master_key TEXT NOT NULL DEFAULT next_business_key('purchase_requisition_lines', 'PRL'),
    requisition_id UUID NOT NULL REFERENCES purchase_requisitions(id) ON DELETE CASCADE,
    item_id UUID NOT NULL REFERENCES items(id) ON DELETE RESTRICT,
    warehouse_id UUID NOT NULL REFERENCES warehouses(id) ON DELETE RESTRICT,
    quantity NUMERIC(18,8) NOT NULL CHECK (quantity > 0),
    unit_cost NUMERIC(18,8) NOT NULL DEFAULT 0,
    amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}',
    legacy_id UUID,
    parent_master_table TEXT,
    parent_master_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_purchase_requisition_lines_master_key ON purchase_requisition_lines(master_key);
CREATE INDEX IF NOT EXISTS idx_purchase_requisition_lines_req ON purchase_requisition_lines(requisition_id);

CREATE TABLE IF NOT EXISTS purchase_orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    master_key TEXT NOT NULL DEFAULT next_business_key('purchase_orders', 'POR'),
    order_number TEXT NOT NULL DEFAULT '',
    requisition_id UUID REFERENCES purchase_requisitions(id) ON DELETE SET NULL,
    supplier_partner_id UUID REFERENCES business_partners(id) ON DELETE SET NULL,
    supplier_id TEXT NOT NULL DEFAULT '',
    supplier_name TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'submitted', 'approved', 'partially_received', 'received', 'closed', 'void')),
    approval_status TEXT NOT NULL DEFAULT 'not_required'
        CHECK (approval_status IN ('not_required', 'pending', 'approved', 'rejected')),
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    department_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    workflow_instance_id UUID REFERENCES workflow_instances(id) ON DELETE SET NULL,
    currency TEXT NOT NULL DEFAULT 'CNY' REFERENCES currencies(code) ON DELETE RESTRICT,
    subtotal NUMERIC(18,8) NOT NULL DEFAULT 0,
    tax_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    total_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}',
    legacy_id UUID,
    parent_master_table TEXT,
    parent_master_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_purchase_orders_master_key ON purchase_orders(master_key);
CREATE INDEX IF NOT EXISTS idx_purchase_orders_org_status
    ON purchase_orders(organization_id, status, created_at DESC);

CREATE TABLE IF NOT EXISTS purchase_order_lines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    master_key TEXT NOT NULL DEFAULT next_business_key('purchase_order_lines', 'POL'),
    order_id UUID NOT NULL REFERENCES purchase_orders(id) ON DELETE CASCADE,
    item_id UUID NOT NULL REFERENCES items(id) ON DELETE RESTRICT,
    warehouse_id UUID NOT NULL REFERENCES warehouses(id) ON DELETE RESTRICT,
    quantity NUMERIC(18,8) NOT NULL CHECK (quantity > 0),
    unit_cost NUMERIC(18,8) NOT NULL DEFAULT 0,
    tax_rate NUMERIC(10,6) NOT NULL DEFAULT 0,
    amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    tax_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    total_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}',
    legacy_id UUID,
    parent_master_table TEXT,
    parent_master_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_purchase_order_lines_master_key ON purchase_order_lines(master_key);
CREATE INDEX IF NOT EXISTS idx_purchase_order_lines_order ON purchase_order_lines(order_id);

CREATE TABLE IF NOT EXISTS purchase_receipts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    master_key TEXT NOT NULL DEFAULT next_business_key('purchase_receipts', 'PRC'),
    receipt_number TEXT NOT NULL DEFAULT '',
    order_id UUID REFERENCES purchase_orders(id) ON DELETE SET NULL,
    supplier_partner_id UUID REFERENCES business_partners(id) ON DELETE SET NULL,
    supplier_id TEXT NOT NULL DEFAULT '',
    supplier_name TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'submitted', 'approved', 'posted', 'void')),
    approval_status TEXT NOT NULL DEFAULT 'not_required'
        CHECK (approval_status IN ('not_required', 'pending', 'approved', 'rejected')),
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    department_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    workflow_instance_id UUID REFERENCES workflow_instances(id) ON DELETE SET NULL,
    warehouse_id UUID REFERENCES warehouses(id) ON DELETE SET NULL,
    payable_id UUID REFERENCES finance_payables(id) ON DELETE SET NULL,
    currency TEXT NOT NULL DEFAULT 'CNY' REFERENCES currencies(code) ON DELETE RESTRICT,
    subtotal NUMERIC(18,8) NOT NULL DEFAULT 0,
    tax_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    total_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}',
    legacy_id UUID,
    parent_master_table TEXT,
    parent_master_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_purchase_receipts_master_key ON purchase_receipts(master_key);
CREATE INDEX IF NOT EXISTS idx_purchase_receipts_org_status
    ON purchase_receipts(organization_id, status, created_at DESC);

CREATE TABLE IF NOT EXISTS purchase_receipt_lines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    master_key TEXT NOT NULL DEFAULT next_business_key('purchase_receipt_lines', 'PRL2'),
    receipt_id UUID NOT NULL REFERENCES purchase_receipts(id) ON DELETE CASCADE,
    order_line_id UUID REFERENCES purchase_order_lines(id) ON DELETE SET NULL,
    item_id UUID NOT NULL REFERENCES items(id) ON DELETE RESTRICT,
    warehouse_id UUID NOT NULL REFERENCES warehouses(id) ON DELETE RESTRICT,
    location_id UUID REFERENCES warehouse_locations(id) ON DELETE SET NULL,
    quantity NUMERIC(18,8) NOT NULL CHECK (quantity > 0),
    unit_cost NUMERIC(18,8) NOT NULL DEFAULT 0,
    tax_rate NUMERIC(10,6) NOT NULL DEFAULT 0,
    amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    tax_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    total_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}',
    legacy_id UUID,
    parent_master_table TEXT,
    parent_master_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_purchase_receipt_lines_master_key ON purchase_receipt_lines(master_key);
CREATE INDEX IF NOT EXISTS idx_purchase_receipt_lines_receipt ON purchase_receipt_lines(receipt_id);

CREATE TABLE IF NOT EXISTS purchase_returns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    master_key TEXT NOT NULL DEFAULT next_business_key('purchase_returns', 'PRT'),
    return_number TEXT NOT NULL DEFAULT '',
    receipt_id UUID REFERENCES purchase_receipts(id) ON DELETE SET NULL,
    supplier_partner_id UUID REFERENCES business_partners(id) ON DELETE SET NULL,
    supplier_id TEXT NOT NULL DEFAULT '',
    supplier_name TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'submitted', 'approved', 'posted', 'void')),
    approval_status TEXT NOT NULL DEFAULT 'not_required'
        CHECK (approval_status IN ('not_required', 'pending', 'approved', 'rejected')),
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    department_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    workflow_instance_id UUID REFERENCES workflow_instances(id) ON DELETE SET NULL,
    currency TEXT NOT NULL DEFAULT 'CNY' REFERENCES currencies(code) ON DELETE RESTRICT,
    subtotal NUMERIC(18,8) NOT NULL DEFAULT 0,
    tax_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    total_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}',
    legacy_id UUID,
    parent_master_table TEXT,
    parent_master_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_purchase_returns_master_key ON purchase_returns(master_key);

CREATE TABLE IF NOT EXISTS purchase_return_lines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    master_key TEXT NOT NULL DEFAULT next_business_key('purchase_return_lines', 'PRTL'),
    return_id UUID NOT NULL REFERENCES purchase_returns(id) ON DELETE CASCADE,
    item_id UUID NOT NULL REFERENCES items(id) ON DELETE RESTRICT,
    warehouse_id UUID NOT NULL REFERENCES warehouses(id) ON DELETE RESTRICT,
    location_id UUID REFERENCES warehouse_locations(id) ON DELETE SET NULL,
    quantity NUMERIC(18,8) NOT NULL CHECK (quantity > 0),
    unit_cost NUMERIC(18,8) NOT NULL DEFAULT 0,
    amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    tax_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    total_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}',
    legacy_id UUID,
    parent_master_table TEXT,
    parent_master_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_purchase_return_lines_master_key ON purchase_return_lines(master_key);
CREATE INDEX IF NOT EXISTS idx_purchase_return_lines_return ON purchase_return_lines(return_id);

CREATE TABLE IF NOT EXISTS sales_quotations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    master_key TEXT NOT NULL DEFAULT next_business_key('sales_quotations', 'SQT'),
    quotation_number TEXT NOT NULL DEFAULT '',
    customer_partner_id UUID REFERENCES business_partners(id) ON DELETE SET NULL,
    customer_id TEXT NOT NULL DEFAULT '',
    customer_name TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'submitted', 'approved', 'converted', 'expired', 'void')),
    approval_status TEXT NOT NULL DEFAULT 'not_required'
        CHECK (approval_status IN ('not_required', 'pending', 'approved', 'rejected')),
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    department_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    workflow_instance_id UUID REFERENCES workflow_instances(id) ON DELETE SET NULL,
    currency TEXT NOT NULL DEFAULT 'CNY' REFERENCES currencies(code) ON DELETE RESTRICT,
    subtotal NUMERIC(18,8) NOT NULL DEFAULT 0,
    tax_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    total_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}',
    legacy_id UUID,
    parent_master_table TEXT,
    parent_master_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_sales_quotations_master_key ON sales_quotations(master_key);
CREATE INDEX IF NOT EXISTS idx_sales_quotations_org_status
    ON sales_quotations(organization_id, status, created_at DESC);

CREATE TABLE IF NOT EXISTS sales_quotation_lines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    master_key TEXT NOT NULL DEFAULT next_business_key('sales_quotation_lines', 'SQL'),
    quotation_id UUID NOT NULL REFERENCES sales_quotations(id) ON DELETE CASCADE,
    item_id UUID NOT NULL REFERENCES items(id) ON DELETE RESTRICT,
    warehouse_id UUID NOT NULL REFERENCES warehouses(id) ON DELETE RESTRICT,
    quantity NUMERIC(18,8) NOT NULL CHECK (quantity > 0),
    unit_price NUMERIC(18,8) NOT NULL DEFAULT 0,
    tax_rate NUMERIC(10,6) NOT NULL DEFAULT 0,
    amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    tax_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    total_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}',
    legacy_id UUID,
    parent_master_table TEXT,
    parent_master_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_sales_quotation_lines_master_key ON sales_quotation_lines(master_key);
CREATE INDEX IF NOT EXISTS idx_sales_quotation_lines_quote ON sales_quotation_lines(quotation_id);

CREATE TABLE IF NOT EXISTS sales_orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    master_key TEXT NOT NULL DEFAULT next_business_key('sales_orders', 'SOR'),
    order_number TEXT NOT NULL DEFAULT '',
    quotation_id UUID REFERENCES sales_quotations(id) ON DELETE SET NULL,
    customer_partner_id UUID REFERENCES business_partners(id) ON DELETE SET NULL,
    customer_id TEXT NOT NULL DEFAULT '',
    customer_name TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'submitted', 'approved', 'confirmed', 'partially_shipped', 'shipped', 'closed', 'void')),
    approval_status TEXT NOT NULL DEFAULT 'not_required'
        CHECK (approval_status IN ('not_required', 'pending', 'approved', 'rejected')),
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    department_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    workflow_instance_id UUID REFERENCES workflow_instances(id) ON DELETE SET NULL,
    currency TEXT NOT NULL DEFAULT 'CNY' REFERENCES currencies(code) ON DELETE RESTRICT,
    subtotal NUMERIC(18,8) NOT NULL DEFAULT 0,
    tax_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    total_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}',
    legacy_id UUID,
    parent_master_table TEXT,
    parent_master_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_sales_orders_master_key ON sales_orders(master_key);
CREATE INDEX IF NOT EXISTS idx_sales_orders_org_status
    ON sales_orders(organization_id, status, created_at DESC);

CREATE TABLE IF NOT EXISTS sales_order_lines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    master_key TEXT NOT NULL DEFAULT next_business_key('sales_order_lines', 'SOL'),
    order_id UUID NOT NULL REFERENCES sales_orders(id) ON DELETE CASCADE,
    item_id UUID NOT NULL REFERENCES items(id) ON DELETE RESTRICT,
    warehouse_id UUID NOT NULL REFERENCES warehouses(id) ON DELETE RESTRICT,
    quantity NUMERIC(18,8) NOT NULL CHECK (quantity > 0),
    unit_price NUMERIC(18,8) NOT NULL DEFAULT 0,
    tax_rate NUMERIC(10,6) NOT NULL DEFAULT 0,
    amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    tax_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    total_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}',
    legacy_id UUID,
    parent_master_table TEXT,
    parent_master_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_sales_order_lines_master_key ON sales_order_lines(master_key);
CREATE INDEX IF NOT EXISTS idx_sales_order_lines_order ON sales_order_lines(order_id);

CREATE TABLE IF NOT EXISTS sales_shipments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    master_key TEXT NOT NULL DEFAULT next_business_key('sales_shipments', 'SSP'),
    shipment_number TEXT NOT NULL DEFAULT '',
    order_id UUID REFERENCES sales_orders(id) ON DELETE SET NULL,
    customer_partner_id UUID REFERENCES business_partners(id) ON DELETE SET NULL,
    customer_id TEXT NOT NULL DEFAULT '',
    customer_name TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'submitted', 'approved', 'posted', 'void')),
    approval_status TEXT NOT NULL DEFAULT 'not_required'
        CHECK (approval_status IN ('not_required', 'pending', 'approved', 'rejected')),
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    department_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    workflow_instance_id UUID REFERENCES workflow_instances(id) ON DELETE SET NULL,
    receivable_id UUID REFERENCES finance_receivables(id) ON DELETE SET NULL,
    currency TEXT NOT NULL DEFAULT 'CNY' REFERENCES currencies(code) ON DELETE RESTRICT,
    subtotal NUMERIC(18,8) NOT NULL DEFAULT 0,
    tax_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    total_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}',
    legacy_id UUID,
    parent_master_table TEXT,
    parent_master_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_sales_shipments_master_key ON sales_shipments(master_key);
CREATE INDEX IF NOT EXISTS idx_sales_shipments_org_status
    ON sales_shipments(organization_id, status, created_at DESC);

CREATE TABLE IF NOT EXISTS sales_shipment_lines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    master_key TEXT NOT NULL DEFAULT next_business_key('sales_shipment_lines', 'SSL'),
    shipment_id UUID NOT NULL REFERENCES sales_shipments(id) ON DELETE CASCADE,
    order_line_id UUID REFERENCES sales_order_lines(id) ON DELETE SET NULL,
    item_id UUID NOT NULL REFERENCES items(id) ON DELETE RESTRICT,
    warehouse_id UUID NOT NULL REFERENCES warehouses(id) ON DELETE RESTRICT,
    location_id UUID REFERENCES warehouse_locations(id) ON DELETE SET NULL,
    quantity NUMERIC(18,8) NOT NULL CHECK (quantity > 0),
    unit_price NUMERIC(18,8) NOT NULL DEFAULT 0,
    tax_rate NUMERIC(10,6) NOT NULL DEFAULT 0,
    amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    tax_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    total_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}',
    legacy_id UUID,
    parent_master_table TEXT,
    parent_master_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_sales_shipment_lines_master_key ON sales_shipment_lines(master_key);
CREATE INDEX IF NOT EXISTS idx_sales_shipment_lines_shipment ON sales_shipment_lines(shipment_id);

CREATE TABLE IF NOT EXISTS sales_returns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    master_key TEXT NOT NULL DEFAULT next_business_key('sales_returns', 'SRT'),
    return_number TEXT NOT NULL DEFAULT '',
    shipment_id UUID REFERENCES sales_shipments(id) ON DELETE SET NULL,
    customer_partner_id UUID REFERENCES business_partners(id) ON DELETE SET NULL,
    customer_id TEXT NOT NULL DEFAULT '',
    customer_name TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'submitted', 'approved', 'posted', 'void')),
    approval_status TEXT NOT NULL DEFAULT 'not_required'
        CHECK (approval_status IN ('not_required', 'pending', 'approved', 'rejected')),
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    department_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    workflow_instance_id UUID REFERENCES workflow_instances(id) ON DELETE SET NULL,
    currency TEXT NOT NULL DEFAULT 'CNY' REFERENCES currencies(code) ON DELETE RESTRICT,
    subtotal NUMERIC(18,8) NOT NULL DEFAULT 0,
    tax_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    total_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}',
    legacy_id UUID,
    parent_master_table TEXT,
    parent_master_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_sales_returns_master_key ON sales_returns(master_key);

CREATE TABLE IF NOT EXISTS sales_return_lines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    master_key TEXT NOT NULL DEFAULT next_business_key('sales_return_lines', 'SRL'),
    return_id UUID NOT NULL REFERENCES sales_returns(id) ON DELETE CASCADE,
    item_id UUID NOT NULL REFERENCES items(id) ON DELETE RESTRICT,
    warehouse_id UUID NOT NULL REFERENCES warehouses(id) ON DELETE RESTRICT,
    location_id UUID REFERENCES warehouse_locations(id) ON DELETE SET NULL,
    quantity NUMERIC(18,8) NOT NULL CHECK (quantity > 0),
    unit_price NUMERIC(18,8) NOT NULL DEFAULT 0,
    amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    tax_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    total_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}',
    legacy_id UUID,
    parent_master_table TEXT,
    parent_master_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_sales_return_lines_master_key ON sales_return_lines(master_key);
CREATE INDEX IF NOT EXISTS idx_sales_return_lines_return ON sales_return_lines(return_id);

WITH new_tables(table_name, master_table_name, detail_table_name, key_prefix, display_name, category, is_base_data, is_business_scenario, metadata) AS (
    VALUES
        ('business_partners', 'business_partners', 'business_partner_details', 'BPT', 'Business Partner', 'base_data', true, false, '{"supply_chain":true}'::jsonb),
        ('items', 'items', 'item_details', 'ITM', 'Item', 'base_data', true, false, '{"supply_chain":true}'::jsonb),
        ('item_uoms', 'items', 'item_uom_details', 'IUM', 'Item UOM', 'base_data', true, false, '{"supply_chain":true}'::jsonb),
        ('warehouses', 'warehouses', 'warehouse_details', 'WHS', 'Warehouse', 'base_data', true, false, '{"supply_chain":true}'::jsonb),
        ('warehouse_locations', 'warehouses', 'warehouse_location_details', 'WLC', 'Warehouse Location', 'base_data', true, false, '{"supply_chain":true}'::jsonb),
        ('inventory_balances', 'inventory_balances', 'inventory_balance_details', 'IVB', 'Inventory Balance', 'inventory', false, true, '{"supply_chain":true}'::jsonb),
        ('inventory_movements', 'inventory_movements', 'inventory_movement_details', 'IVM', 'Inventory Movement', 'inventory', false, true, '{"supply_chain":true}'::jsonb),
        ('inventory_reservations', 'inventory_reservations', 'inventory_reservation_details', 'IVR', 'Inventory Reservation', 'inventory', false, true, '{"supply_chain":true}'::jsonb),
        ('inventory_transfers', 'inventory_transfers', 'inventory_transfer_details', 'IVT', 'Inventory Transfer', 'inventory', false, true, '{"supply_chain":true}'::jsonb),
        ('inventory_transfer_lines', 'inventory_transfers', 'inventory_transfer_line_details', 'ITL', 'Inventory Transfer Line', 'inventory', false, true, '{"supply_chain":true}'::jsonb),
        ('inventory_adjustments', 'inventory_adjustments', 'inventory_adjustment_details', 'IVA', 'Inventory Adjustment', 'inventory', false, true, '{"supply_chain":true}'::jsonb),
        ('inventory_adjustment_lines', 'inventory_adjustments', 'inventory_adjustment_line_details', 'IAL', 'Inventory Adjustment Line', 'inventory', false, true, '{"supply_chain":true}'::jsonb),
        ('inventory_counts', 'inventory_counts', 'inventory_count_details', 'IVC', 'Inventory Count', 'inventory', false, true, '{"supply_chain":true}'::jsonb),
        ('inventory_count_lines', 'inventory_counts', 'inventory_count_line_details', 'ICL', 'Inventory Count Line', 'inventory', false, true, '{"supply_chain":true}'::jsonb),
        ('purchase_requisitions', 'purchase_requisitions', 'purchase_requisition_details', 'PRQ', 'Purchase Requisition', 'procurement', false, true, '{"supply_chain":true}'::jsonb),
        ('purchase_requisition_lines', 'purchase_requisitions', 'purchase_requisition_line_details', 'PRL', 'Purchase Requisition Line', 'procurement', false, true, '{"supply_chain":true}'::jsonb),
        ('purchase_orders', 'purchase_orders', 'purchase_order_details', 'POR', 'Purchase Order', 'procurement', false, true, '{"supply_chain":true}'::jsonb),
        ('purchase_order_lines', 'purchase_orders', 'purchase_order_line_details', 'POL', 'Purchase Order Line', 'procurement', false, true, '{"supply_chain":true}'::jsonb),
        ('purchase_receipts', 'purchase_receipts', 'purchase_receipt_details', 'PRC', 'Purchase Receipt', 'procurement', false, true, '{"supply_chain":true}'::jsonb),
        ('purchase_receipt_lines', 'purchase_receipts', 'purchase_receipt_line_details', 'PRL2', 'Purchase Receipt Line', 'procurement', false, true, '{"supply_chain":true}'::jsonb),
        ('purchase_returns', 'purchase_returns', 'purchase_return_details', 'PRT', 'Purchase Return', 'procurement', false, true, '{"supply_chain":true}'::jsonb),
        ('purchase_return_lines', 'purchase_returns', 'purchase_return_line_details', 'PRTL', 'Purchase Return Line', 'procurement', false, true, '{"supply_chain":true}'::jsonb),
        ('sales_quotations', 'sales_quotations', 'sales_quotation_details', 'SQT', 'Sales Quotation', 'sales', false, true, '{"supply_chain":true}'::jsonb),
        ('sales_quotation_lines', 'sales_quotations', 'sales_quotation_line_details', 'SQL', 'Sales Quotation Line', 'sales', false, true, '{"supply_chain":true}'::jsonb),
        ('sales_orders', 'sales_orders', 'sales_order_details', 'SOR', 'Sales Order', 'sales', false, true, '{"supply_chain":true}'::jsonb),
        ('sales_order_lines', 'sales_orders', 'sales_order_line_details', 'SOL', 'Sales Order Line', 'sales', false, true, '{"supply_chain":true}'::jsonb),
        ('sales_shipments', 'sales_shipments', 'sales_shipment_details', 'SSP', 'Sales Shipment', 'sales', false, true, '{"supply_chain":true}'::jsonb),
        ('sales_shipment_lines', 'sales_shipments', 'sales_shipment_line_details', 'SSL', 'Sales Shipment Line', 'sales', false, true, '{"supply_chain":true}'::jsonb),
        ('sales_returns', 'sales_returns', 'sales_return_details', 'SRT', 'Sales Return', 'sales', false, true, '{"supply_chain":true}'::jsonb),
        ('sales_return_lines', 'sales_returns', 'sales_return_line_details', 'SRL', 'Sales Return Line', 'sales', false, true, '{"supply_chain":true}'::jsonb)
)
INSERT INTO data_table_catalog(table_name, master_table_name, detail_table_name, key_prefix, display_name, category, is_base_data, is_business_scenario, metadata)
SELECT table_name, master_table_name, detail_table_name, key_prefix, display_name, category, is_base_data, is_business_scenario, metadata
FROM new_tables
ON CONFLICT (table_name) DO UPDATE SET
    master_table_name = EXCLUDED.master_table_name,
    detail_table_name = EXCLUDED.detail_table_name,
    key_prefix = EXCLUDED.key_prefix,
    display_name = EXCLUDED.display_name,
    category = EXCLUDED.category,
    is_base_data = EXCLUDED.is_base_data,
    is_business_scenario = EXCLUDED.is_business_scenario,
    metadata = data_table_catalog.metadata || EXCLUDED.metadata,
    updated_at = NOW();

INSERT INTO data_field_catalog(table_name, field_name, data_type, display_name, is_master_key, is_visible_default, permission_level, display_order, metadata)
SELECT
    c.table_name,
    c.column_name,
    c.data_type,
    c.column_name,
    c.column_name = 'master_key',
    c.column_name NOT IN ('metadata'),
    CASE WHEN c.column_name IN ('metadata') THEN 'L3' ELSE 'L1' END,
    c.ordinal_position,
    '{"supply_chain":true}'::jsonb
FROM information_schema.columns c
JOIN data_table_catalog t ON t.table_name = c.table_name
WHERE c.table_schema = 'public'
  AND t.metadata ? 'supply_chain'
ON CONFLICT (table_name, field_name) DO UPDATE SET
    data_type = EXCLUDED.data_type,
    display_name = EXCLUDED.display_name,
    is_master_key = EXCLUDED.is_master_key,
    is_visible_default = EXCLUDED.is_visible_default,
    permission_level = EXCLUDED.permission_level,
    display_order = EXCLUDED.display_order,
    metadata = data_field_catalog.metadata || EXCLUDED.metadata,
    updated_at = NOW();

INSERT INTO module_master_source_catalog(module_name, source_table, entity_type, relation_mode, parent_table, parent_fk, key_prefix, metadata)
VALUES
    ('inventory', 'business_partners', 'business_partner', 'master', NULL, NULL, 'BPT', '{"supply_chain":true}'::jsonb),
    ('inventory', 'items', 'item', 'master', NULL, NULL, 'ITM', '{"supply_chain":true}'::jsonb),
    ('inventory', 'item_uoms', 'item_uom', 'detail', 'items', 'item_id', 'IUM', '{"supply_chain":true}'::jsonb),
    ('inventory', 'warehouses', 'warehouse', 'master', NULL, NULL, 'WHS', '{"supply_chain":true}'::jsonb),
    ('inventory', 'warehouse_locations', 'warehouse_location', 'detail', 'warehouses', 'warehouse_id', 'WLC', '{"supply_chain":true}'::jsonb),
    ('inventory', 'inventory_balances', 'inventory_balance', 'master', NULL, NULL, 'IVB', '{"supply_chain":true}'::jsonb),
    ('inventory', 'inventory_movements', 'inventory_movement', 'detail', 'inventory_balances', 'item_id', 'IVM', '{"supply_chain":true}'::jsonb),
    ('inventory', 'inventory_reservations', 'inventory_reservation', 'detail', 'inventory_balances', 'item_id', 'IVR', '{"supply_chain":true}'::jsonb),
    ('inventory', 'inventory_transfers', 'inventory_transfer', 'master', NULL, NULL, 'IVT', '{"supply_chain":true}'::jsonb),
    ('inventory', 'inventory_transfer_lines', 'inventory_transfer_line', 'detail', 'inventory_transfers', 'transfer_id', 'ITL', '{"supply_chain":true}'::jsonb),
    ('inventory', 'inventory_adjustments', 'inventory_adjustment', 'master', NULL, NULL, 'IVA', '{"supply_chain":true}'::jsonb),
    ('inventory', 'inventory_adjustment_lines', 'inventory_adjustment_line', 'detail', 'inventory_adjustments', 'adjustment_id', 'IAL', '{"supply_chain":true}'::jsonb),
    ('inventory', 'inventory_counts', 'inventory_count', 'master', NULL, NULL, 'IVC', '{"supply_chain":true}'::jsonb),
    ('inventory', 'inventory_count_lines', 'inventory_count_line', 'detail', 'inventory_counts', 'count_id', 'ICL', '{"supply_chain":true}'::jsonb),
    ('procurement', 'purchase_requisitions', 'purchase_requisition', 'master', NULL, NULL, 'PRQ', '{"supply_chain":true}'::jsonb),
    ('procurement', 'purchase_requisition_lines', 'purchase_requisition_line', 'detail', 'purchase_requisitions', 'requisition_id', 'PRL', '{"supply_chain":true}'::jsonb),
    ('procurement', 'purchase_orders', 'purchase_order', 'master', NULL, NULL, 'POR', '{"supply_chain":true}'::jsonb),
    ('procurement', 'purchase_order_lines', 'purchase_order_line', 'detail', 'purchase_orders', 'order_id', 'POL', '{"supply_chain":true}'::jsonb),
    ('procurement', 'purchase_receipts', 'purchase_receipt', 'master', NULL, NULL, 'PRC', '{"supply_chain":true}'::jsonb),
    ('procurement', 'purchase_receipt_lines', 'purchase_receipt_line', 'detail', 'purchase_receipts', 'receipt_id', 'PRL2', '{"supply_chain":true}'::jsonb),
    ('procurement', 'purchase_returns', 'purchase_return', 'master', NULL, NULL, 'PRT', '{"supply_chain":true}'::jsonb),
    ('procurement', 'purchase_return_lines', 'purchase_return_line', 'detail', 'purchase_returns', 'return_id', 'PRTL', '{"supply_chain":true}'::jsonb),
    ('sales', 'sales_quotations', 'sales_quotation', 'master', NULL, NULL, 'SQT', '{"supply_chain":true}'::jsonb),
    ('sales', 'sales_quotation_lines', 'sales_quotation_line', 'detail', 'sales_quotations', 'quotation_id', 'SQL', '{"supply_chain":true}'::jsonb),
    ('sales', 'sales_orders', 'sales_order', 'master', NULL, NULL, 'SOR', '{"supply_chain":true}'::jsonb),
    ('sales', 'sales_order_lines', 'sales_order_line', 'detail', 'sales_orders', 'order_id', 'SOL', '{"supply_chain":true}'::jsonb),
    ('sales', 'sales_shipments', 'sales_shipment', 'master', NULL, NULL, 'SSP', '{"supply_chain":true}'::jsonb),
    ('sales', 'sales_shipment_lines', 'sales_shipment_line', 'detail', 'sales_shipments', 'shipment_id', 'SSL', '{"supply_chain":true}'::jsonb),
    ('sales', 'sales_returns', 'sales_return', 'master', NULL, NULL, 'SRT', '{"supply_chain":true}'::jsonb),
    ('sales', 'sales_return_lines', 'sales_return_line', 'detail', 'sales_returns', 'return_id', 'SRL', '{"supply_chain":true}'::jsonb)
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
    ('procurement', 'Procurement', 'business', true, 'commercial', '{"supply_chain":true}'::jsonb),
    ('sales', 'Sales', 'business', true, 'commercial', '{"supply_chain":true}'::jsonb),
    ('inventory', 'Inventory', 'business', true, 'commercial', '{"supply_chain":true}'::jsonb)
ON CONFLICT (module_key) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    category = EXCLUDED.category,
    enabled_default = EXCLUDED.enabled_default,
    license_scope = EXCLUDED.license_scope,
    metadata = saas_modules.metadata || EXCLUDED.metadata,
    updated_at = NOW();

INSERT INTO saas_plan_modules(plan_id, module_key)
SELECT p.id, m.module_key
FROM saas_plans p
JOIN saas_modules m ON m.module_key IN ('procurement', 'sales', 'inventory')
WHERE p.code = 'foundation'
ON CONFLICT (plan_id, module_key) DO NOTHING;

WITH seed_version AS (
    INSERT INTO context_dictionary_versions(scope_level, module_key, version_key, source_type, source_name, status, metadata)
    VALUES ('saas', 'supply_chain', 'supply-chain-v1', 'json', 'migration_038_supply_chain_core', 'active', '{"modules":["procurement","sales","inventory"]}'::jsonb)
    ON CONFLICT (scope_level, organization_id, module_key, version_key) DO UPDATE
    SET status = 'active', updated_at = NOW()
    RETURNING id
),
domains AS (
    INSERT INTO context_business_domains(dictionary_version_id, module_key, name, scope_level, status, metadata)
    SELECT id, 'procurement', 'Procurement', 'saas', 'active', '{"supply_chain":true}'::jsonb FROM seed_version
    UNION ALL SELECT id, 'sales', 'Sales', 'saas', 'active', '{"supply_chain":true}'::jsonb FROM seed_version
    UNION ALL SELECT id, 'inventory', 'Inventory', 'saas', 'active', '{"supply_chain":true}'::jsonb FROM seed_version
    ON CONFLICT (dictionary_version_id, module_key) DO UPDATE SET status = 'active'
    RETURNING id, dictionary_version_id, module_key
),
entities AS (
    INSERT INTO context_entities(dictionary_version_id, domain_id, entity_key, display_name, description, status, metadata)
    SELECT dictionary_version_id, id, 'item', 'Item', 'Tradable material or service master data', 'active', '{"table_name":"items"}'::jsonb FROM domains WHERE module_key = 'inventory'
    UNION ALL SELECT dictionary_version_id, id, 'warehouse', 'Warehouse', 'Inventory warehouse', 'active', '{"table_name":"warehouses"}'::jsonb FROM domains WHERE module_key = 'inventory'
    UNION ALL SELECT dictionary_version_id, id, 'inventory_balance', 'Inventory Balance', 'On-hand inventory balance and valuation', 'active', '{"table_name":"inventory_balances"}'::jsonb FROM domains WHERE module_key = 'inventory'
    UNION ALL SELECT dictionary_version_id, id, 'purchase_receipt', 'Purchase Receipt', 'Inbound receipt that can create inventory and payable entries', 'active', '{"table_name":"purchase_receipts"}'::jsonb FROM domains WHERE module_key = 'procurement'
    UNION ALL SELECT dictionary_version_id, id, 'sales_shipment', 'Sales Shipment', 'Outbound shipment that can create inventory and receivable entries', 'active', '{"table_name":"sales_shipments"}'::jsonb FROM domains WHERE module_key = 'sales'
    ON CONFLICT (dictionary_version_id, entity_key) DO UPDATE
    SET status = 'active', display_name = EXCLUDED.display_name, description = EXCLUDED.description
    RETURNING id, dictionary_version_id, entity_key
),
fields AS (
    INSERT INTO context_fields(dictionary_version_id, entity_id, field_key, display_name, data_type, semantic_type, sensitivity_level, base_weight, is_finance_field, is_workflow_field, is_governance_field, mask_strategy, status, metadata)
    SELECT dictionary_version_id, id, 'quantity', 'Quantity', 'number', 'inventory_quantity', 'normal', 8, false, false, false, 'none', 'active', '{"supply_chain":true}'::jsonb FROM entities WHERE entity_key = 'inventory_balance'
    UNION ALL SELECT dictionary_version_id, id, 'average_cost', 'Average Cost', 'number', 'valuation', 'sensitive', 7, true, false, false, 'summary', 'active', '{"supply_chain":true}'::jsonb FROM entities WHERE entity_key = 'inventory_balance'
    UNION ALL SELECT dictionary_version_id, id, 'status', 'Status', 'string', 'document_status', 'normal', 8, false, true, true, 'none', 'active', '{"supply_chain":true}'::jsonb FROM entities WHERE entity_key IN ('purchase_receipt', 'sales_shipment')
    UNION ALL SELECT dictionary_version_id, id, 'total_amount', 'Total Amount', 'number', 'document_amount', 'sensitive', 8, true, false, false, 'summary', 'active', '{"supply_chain":true}'::jsonb FROM entities WHERE entity_key IN ('purchase_receipt', 'sales_shipment')
    ON CONFLICT (entity_id, field_key) DO UPDATE
    SET status = 'active', display_name = EXCLUDED.display_name, sensitivity_level = EXCLUDED.sensitivity_level, base_weight = EXCLUDED.base_weight
    RETURNING id, dictionary_version_id, entity_id, field_key
)
INSERT INTO context_physical_mappings(dictionary_version_id, entity_id, field_id, table_name, column_name, tenant_column, status, metadata)
SELECT f.dictionary_version_id, f.entity_id, f.id,
       CASE e.entity_key
           WHEN 'inventory_balance' THEN 'inventory_balances'
           WHEN 'purchase_receipt' THEN 'purchase_receipts'
           WHEN 'sales_shipment' THEN 'sales_shipments'
           ELSE e.entity_key || 's'
       END,
       f.field_key,
       'organization_id',
       'active',
       '{"supply_chain":true}'::jsonb
FROM fields f
JOIN entities e ON e.id = f.entity_id
WHERE NOT EXISTS (
    SELECT 1
    FROM context_physical_mappings existing
    WHERE existing.field_id = f.id
      AND existing.table_name = CASE e.entity_key
          WHEN 'inventory_balance' THEN 'inventory_balances'
          WHEN 'purchase_receipt' THEN 'purchase_receipts'
          WHEN 'sales_shipment' THEN 'sales_shipments'
          ELSE e.entity_key || 's'
      END
      AND existing.column_name = f.field_key
);
