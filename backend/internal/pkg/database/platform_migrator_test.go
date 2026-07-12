package database

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadPlatformMigrationFilesComputesChecksumsAndDirectives(t *testing.T) {
	dir := t.TempDir()
	writePlatformMigrationTestFile(t, dir, "001_baseline.sql", "CREATE TABLE example(id integer);\n")
	writePlatformMigrationTestFile(t, dir, "002_repair.sql", "-- platformdb:accept-checksum-drift 001_baseline.sql\nSELECT 1;\n")

	files, err := loadPlatformMigrationFiles(dir)
	if err != nil {
		t.Fatalf("load platform migrations: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("file count = %d, want 2", len(files))
	}
	if files[0].Filename != "001_baseline.sql" || files[0].Checksum == "" {
		t.Fatalf("baseline file = %#v", files[0])
	}
	if len(files[1].AcceptsChecksumDrift) != 1 || files[1].AcceptsChecksumDrift[0] != "001_baseline.sql" {
		t.Fatalf("repair directives = %#v", files[1].AcceptsChecksumDrift)
	}
}

func TestAcceptedPlatformChecksumDriftFilenamesRejectsPaths(t *testing.T) {
	_, err := acceptedPlatformChecksumDriftFilenames("-- platformdb:accept-checksum-drift ../001.sql")
	if err == nil {
		t.Fatal("expected invalid platform checksum drift path to fail")
	}
}

func TestPendingPlatformChecksumRepairRequiresLaterUnappliedMigration(t *testing.T) {
	files := []platformMigrationFile{
		{Filename: "001.sql"},
		{Filename: "002.sql", AcceptsChecksumDrift: []string{"001.sql"}},
	}
	if got := pendingPlatformChecksumRepairFor(files, map[string]string{"001.sql": "old"}, 0, "001.sql"); got != "002.sql" {
		t.Fatalf("pending repair = %q, want 002.sql", got)
	}
	if got := pendingPlatformChecksumRepairFor(files, map[string]string{"001.sql": "old", "002.sql": "done"}, 0, "001.sql"); got != "" {
		t.Fatalf("applied repair accepted new drift through %q", got)
	}
}

func TestPlatformMigrationTrackingSQLIncludesChecksumAudit(t *testing.T) {
	sql := platformMigrationTrackingSQL()
	for _, fragment := range []string{
		"checksum    TEXT NOT NULL DEFAULT ''",
		"ADD COLUMN IF NOT EXISTS checksum",
		"platform.platform_migration_checksum_history",
	} {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("tracking SQL missing %q:\n%s", fragment, sql)
		}
	}
	if strings.Count(sql, ");") < 2 {
		t.Fatalf("tracking SQL statements are not terminated:\n%s", sql)
	}
}

func writePlatformMigrationTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
