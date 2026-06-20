-- 039_supply_chain_posting_idempotency.sql
-- Idempotency guards for supply-chain posting side effects.

CREATE UNIQUE INDEX IF NOT EXISTS uq_inventory_movements_purchase_receipt_line
    ON inventory_movements(source_type, source_id, (metadata ->> 'receipt_line_id'))
    WHERE source_type = 'purchase_receipt'
      AND source_id IS NOT NULL
      AND metadata ? 'receipt_line_id';

CREATE UNIQUE INDEX IF NOT EXISTS uq_inventory_movements_sales_shipment_line
    ON inventory_movements(source_type, source_id, (metadata ->> 'shipment_line_id'))
    WHERE source_type = 'sales_shipment'
      AND source_id IS NOT NULL
      AND metadata ? 'shipment_line_id';

CREATE UNIQUE INDEX IF NOT EXISTS uq_finance_payables_purchase_receipt_source
    ON finance_payables(source_type, source_id)
    WHERE source_type = 'purchase_receipt'
      AND source_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_finance_receivables_sales_shipment_source
    ON finance_receivables(source_type, source_id)
    WHERE source_type = 'sales_shipment'
      AND source_id IS NOT NULL;
