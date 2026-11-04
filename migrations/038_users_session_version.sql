-- platformdb:accept-checksum-drift 000_saas_platform_management_baseline.sql

-- JWT session revocation: `users.session_version` is embedded in issued tokens as
-- the `ver` claim and compared per request. Password changes, admin resets and
-- account disables increment it, which invalidates every previously issued token
-- for that account. Tokens without the claim are treated as version 0 so the
-- rollout does not log existing sessions out.

BEGIN;

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS session_version BIGINT NOT NULL DEFAULT 0;

COMMIT;
