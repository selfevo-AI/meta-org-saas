package database

const platformMigrationTrackingTable = "platform.platform_migration_runs"

func platformMigrationTrackingSQL() string {
	return `
		CREATE SCHEMA IF NOT EXISTS platform;
		CREATE TABLE IF NOT EXISTS platform.platform_migration_runs (
			filename    VARCHAR(255) PRIMARY KEY,
			checksum    TEXT NOT NULL DEFAULT '',
			applied_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
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
	`
}

func migrateLegacyPlatformMigrationRunsSQL() string {
	return `
		DO $$
		BEGIN
			IF to_regclass('public.schema_migrations') IS NOT NULL THEN
				INSERT INTO platform.platform_migration_runs(filename, checksum, applied_at)
				SELECT filename, '', applied_at FROM public.schema_migrations
				ON CONFLICT (filename) DO NOTHING;
				DROP TABLE public.schema_migrations;
			END IF;
		END;
		$$;
	`
}
