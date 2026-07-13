-- Repair existing platform databases with leased recovery for unfinished AI
-- Gateway balance reservations.
-- platformdb:accept-checksum-drift 004_ai_capability_baseline.sql
-- platformdb:accept-checksum-drift 008_ai_gateway_model_group_repair.sql

ALTER TABLE ai_gateway_balance_transactions
    ADD COLUMN IF NOT EXISTS reconcile_lease_owner TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS reconcile_lease_expires_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_ai_gateway_open_reservations_reconcile
    ON ai_gateway_balance_transactions(created_at, reconcile_lease_expires_at)
    WHERE transaction_type = 'reserve';
