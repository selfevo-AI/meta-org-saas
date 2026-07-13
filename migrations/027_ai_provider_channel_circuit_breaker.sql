-- Add runtime circuit state for provider-channel failure isolation and atomic
-- cooldown probes on databases that applied the AI capability baseline earlier.
-- platformdb:accept-checksum-drift 004_ai_capability_baseline.sql

ALTER TABLE model_provider_channels
    ADD COLUMN IF NOT EXISTS consecutive_failure_count INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS circuit_open_until TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_model_provider_channels_circuit_routing
    ON model_provider_channels(provider_id, status, circuit_open_until, priority)
    WHERE status = 'active';
