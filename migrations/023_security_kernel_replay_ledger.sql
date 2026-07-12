-- platformdb:accept-checksum-drift 000_saas_platform_management_baseline.sql

BEGIN;

CREATE TABLE IF NOT EXISTS platform.security_request_nonces (
    nonce             UUID PRIMARY KEY,
    request_timestamp TIMESTAMPTZ NOT NULL,
    claimed_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at        TIMESTAMPTZ NOT NULL,
    metadata          JSONB NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(metadata) = 'object')
);

CREATE INDEX IF NOT EXISTS idx_security_request_nonces_expires
    ON platform.security_request_nonces(expires_at);

COMMIT;
