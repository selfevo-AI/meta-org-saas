package database

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/tenantdb"
)

var historicalWindowsMigrations = map[string]bool{
	"000_saas_platform_management_baseline.sql":       true,
	"001_erp_code_baseline.sql":                       true,
	"002_erp_platform_integration_baseline.sql":       true,
	"004_ai_capability_baseline.sql":                  true,
	"006_saas_manufacturing_module_seed.sql":          true,
	"007_saas_runtime_organization_target_repair.sql": true,
	"012_tenant_database_target_state_repair.sql":     true,
	"013_tenant_event_projection_infrastructure.sql":  true,
	"014_platform_migration_checksum_governance.sql":  true,
	"015_authentication_rate_limit_buckets.sql":       true,
	"tenant/001_tenant_business_baseline.sql":         true,
	"tenant/002_tenant_projection_outbox.sql":         true,
}

func TestRepositoryMigrationsUseLFLineEndings(t *testing.T) {
	err := filepath.WalkDir(repoMigrationsDir(t), func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Ext(path) != ".sql" {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(data), "\r") {
			t.Errorf("migration %s must use LF line endings", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestMigrationLineEndingRepairAgainstPostgres(t *testing.T) {
	if os.Getenv("RUN_FRESH_DB_MIGRATION_TEST") != "1" {
		t.Skip("set RUN_FRESH_DB_MIGRATION_TEST=1 to verify Windows checksum repair")
	}
	for _, scope := range []string{"platform", "tenant"} {
		t.Run(scope, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			pool, databaseURL := ontologyUpgradeDatabase(t, ctx)
			target := tenantdb.NewDedicatedDatabaseTarget(uuid.New(), "meta_org_", "test", "local")
			migrate := func(dir string) error {
				if scope == "platform" {
					return RunMigrations(ctx, pool, dir)
				}
				_, err := (tenantdb.FileTenantMigrator{MigrationsDir: filepath.Join(dir, "tenant")}).Migrate(ctx, target, databaseURL)
				return err
			}
			previous := migrationLineEndingSnapshot(t, true)
			current := migrationLineEndingSnapshot(t, false)
			if err := migrate(previous); err != nil {
				t.Fatalf("install mixed-line-ending baseline: %v", err)
			}
			if _, err := pool.Exec(ctx, `INSERT INTO "MCRD" ("CardCode", "Payload") VALUES ('EOL-KEEP', '{"CardName":"Preserved partner"}')`); err != nil {
				t.Fatal(err)
			}
			for attempt := 0; attempt < 2; attempt++ {
				if err := migrate(current); err != nil {
					t.Fatalf("repair/restart %d: %v", attempt, err)
				}
			}
			var name string
			if err := pool.QueryRow(ctx, `SELECT "Payload"->>'CardName' FROM "MCRD" WHERE "CardCode" = 'EOL-KEEP'`).Scan(&name); err != nil || name != "Preserved partner" {
				t.Fatalf("business row changed during checksum repair: %q, %v", name, err)
			}
			query := `SELECT COUNT(*) FROM platform.platform_migration_checksum_history WHERE repair_filename = '035_migration_line_endings.sql'`
			want := 10
			if scope == "tenant" {
				query = `SELECT COUNT(*) FROM tenant_migration_checksum_history WHERE repair_filename = '010_migration_line_endings.sql'`
				want = 2
			}
			var count int
			if err := pool.QueryRow(ctx, query).Scan(&count); err != nil || count != want {
				t.Fatalf("checksum audit rows = %d, want %d: %v", count, want, err)
			}

			// A consumed repair must not permit another change to the same baseline.
			path := filepath.Join(current, "001_erp_code_baseline.sql")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, append(data, []byte("\n-- unexpected later edit\n")...), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := migrate(current); err == nil || !strings.Contains(err.Error(), "checksum drift") {
				t.Fatalf("subsequent drift was not rejected: %v", err)
			}
		})
	}
}

func migrationLineEndingSnapshot(t *testing.T, legacy bool) string {
	t.Helper()
	source := repoMigrationsDir(t)
	dir := t.TempDir()
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Ext(path) != ".sql" {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		key := filepath.ToSlash(relative)
		if legacy && (key == "035_migration_line_endings.sql" || key == "tenant/010_migration_line_endings.sql") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if legacy && historicalWindowsMigrations[key] {
			lines := strings.SplitAfter(string(data), "\n")
			for i := range lines {
				if i%2 == 0 {
					lines[i] = strings.ReplaceAll(lines[i], "\n", "\r\n")
				}
			}
			data = []byte(strings.Join(lines, ""))
		}
		destination := filepath.Join(dir, relative)
		if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
			return err
		}
		return os.WriteFile(destination, data, 0o600)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dir
}
