-- Add hot-path indexes for tenant finance/costing list and allocation queries.
-- cost_ledger_entries is only indexed by (project_id, occurred_at) in the ERP
-- baseline, so organization-scoped ledger lists/summaries and the payment
-- allocation join on finance_payable_id degrade to sequential scans as the
-- ledger grows; the payable/receivable workbench lists sort org rows by
-- created_at with no supporting index.

CREATE INDEX IF NOT EXISTS idx_cost_ledger_org_occurred
    ON cost_ledger_entries(organization_id, occurred_at DESC);

CREATE INDEX IF NOT EXISTS idx_cost_ledger_finance_payable
    ON cost_ledger_entries(finance_payable_id)
    WHERE finance_payable_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_finance_payables_org_created
    ON finance_payables(organization_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_finance_receivables_org_created
    ON finance_receivables(organization_id, created_at DESC);
