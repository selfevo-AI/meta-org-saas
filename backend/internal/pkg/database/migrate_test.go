package database

import (
	"strings"
	"testing"
)

func TestPlatformMigrationTrackingUsesPlatformMigrationRuns(t *testing.T) {
	if platformMigrationTrackingTable != "platform.platform_migration_runs" {
		t.Fatalf("platform migration tracking table = %q, want platform.platform_migration_runs", platformMigrationTrackingTable)
	}
	sql := platformMigrationTrackingSQL()
	if !strings.Contains(sql, "CREATE SCHEMA IF NOT EXISTS platform") {
		t.Fatalf("tracking SQL = %q, want platform schema creation", sql)
	}
	if !strings.Contains(sql, "CREATE TABLE IF NOT EXISTS platform.platform_migration_runs") {
		t.Fatalf("tracking SQL = %q, want platform.platform_migration_runs table", sql)
	}
	if strings.Contains(sql, "CREATE TABLE IF NOT EXISTS public.schema_migrations") {
		t.Fatalf("tracking SQL = %q, must not create old schema_migrations table", sql)
	}
	legacySQL := migrateLegacyPlatformMigrationRunsSQL()
	if !strings.Contains(legacySQL, "to_regclass('public.schema_migrations')") {
		t.Fatalf("legacy tracking SQL = %q, want legacy schema_migrations copy guard", legacySQL)
	}
	if !strings.Contains(legacySQL, "INSERT INTO platform.platform_migration_runs") {
		t.Fatalf("legacy tracking SQL = %q, want copy into platform.platform_migration_runs", legacySQL)
	}
}
