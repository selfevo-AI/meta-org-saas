-- platformdb:accept-checksum-drift 000_saas_platform_management_baseline.sql

BEGIN;

CREATE SCHEMA IF NOT EXISTS platform;

ALTER TABLE platform.platform_migration_runs
    ADD COLUMN IF NOT EXISTS checksum TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS platform.platform_migration_checksum_history (
    id                BIGSERIAL PRIMARY KEY,
    filename          VARCHAR(255) NOT NULL,
    previous_checksum TEXT NOT NULL,
    accepted_checksum TEXT NOT NULL,
    repair_filename   VARCHAR(255) NOT NULL,
    reconciled_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (filename, previous_checksum, accepted_checksum, repair_filename)
);

COMMIT;

