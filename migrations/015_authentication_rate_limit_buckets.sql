-- platformdb:accept-checksum-drift 000_saas_platform_management_baseline.sql

BEGIN;

CREATE TABLE IF NOT EXISTS platform.authentication_rate_limit_buckets (
    bucket_key        TEXT PRIMARY KEY,
    scope             TEXT NOT NULL,
    window_started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    attempt_count     INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    failure_count     INTEGER NOT NULL DEFAULT 0 CHECK (failure_count >= 0),
    blocked_until     TIMESTAMPTZ,
    last_attempt_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata          JSONB NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(metadata) = 'object')
);

CREATE INDEX IF NOT EXISTS idx_auth_rate_limit_blocked
    ON platform.authentication_rate_limit_buckets(blocked_until)
    WHERE blocked_until IS NOT NULL;

COMMIT;

