package database

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/tenantdb"
)

func TestOperationalOntologyUpgradeAgainstPostgres(t *testing.T) {
	if os.Getenv("RUN_FRESH_DB_MIGRATION_TEST") != "1" {
		t.Skip("set RUN_FRESH_DB_MIGRATION_TEST=1 to verify the operational ontology upgrade")
	}
	current := repoMigrationsDir(t)
	previous := preOntologyMigrationSnapshot(t, current)
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
			if err := migrate(previous); err != nil {
				t.Fatalf("install pre-ontology baseline: %v", err)
			}
			entryID := uuid.New()
			voidID, accountID, centerID := uuid.New(), uuid.New(), uuid.New()
			if _, err := pool.Exec(ctx, `INSERT INTO "MACT" ("AcctCode", "Payload") VALUES ('UPGRADE-CASH', '{"Name":"Canonical cash","Currency":"CNY"}');
				INSERT INTO "MPRC" ("PrcCode", "Payload") VALUES ('UPGRADE-CENTER', '{"Name":"Canonical center"}');`); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, `INSERT INTO gl_accounts(account_code, name, account_type) VALUES ('UPGRADE-CASH', 'Legacy cash', 'asset');
				INSERT INTO gl_accounts(account_code, name, account_type) VALUES ('UPGRADE-EQUITY', 'Legacy equity', 'equity');`); err != nil {
				t.Fatal(err)
			}
			if err := pool.QueryRow(ctx, `SELECT id FROM gl_accounts WHERE account_code = 'UPGRADE-CASH'`).Scan(&accountID); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, `INSERT INTO gl_cost_centers(id, cost_center_code, name) VALUES ($1, 'UPGRADE-CENTER', 'Legacy center')`, centerID); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, `INSERT INTO gl_journal_entries(id, reference_date, status) VALUES ($1, CURRENT_DATE, 'void')`, voidID); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, `INSERT INTO gl_journal_entries(id, reference_date, status, memo, posted_at) VALUES ($1, CURRENT_DATE, 'posted', 'Upgrade reconciliation', NOW())`, entryID); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, `INSERT INTO gl_journal_entry_lines(entry_id, line_num, account_code, debit, credit) VALUES ($1, 1, 'UPGRADE-CASH', 123.456789, 0), ($1, 2, 'UPGRADE-EQUITY', 0, 123.456789)`, entryID); err != nil {
				t.Fatal(err)
			}
			for attempt := 0; attempt < 2; attempt++ {
				if err := migrate(current); err != nil {
					t.Fatalf("upgrade/restart %d: %v", attempt, err)
				}
			}
			var count int
			var debit, credit float64
			if err := pool.QueryRow(ctx, `SELECT COUNT(*), SUM(debit), SUM(credit) FROM erp_gl_journal_entry_lines WHERE entry_id = $1`, entryID).Scan(&count, &debit, &credit); err != nil {
				t.Fatal(err)
			}
			if count != 2 || debit != 123.456789 || credit != debit {
				t.Fatalf("migrated journal changed or duplicated: lines=%d debit=%v credit=%v", count, debit, credit)
			}
			if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM gl_journal_entries WHERE id = $1`, entryID).Scan(&count); err != nil || count != 1 {
				t.Fatalf("legacy reconciliation row was not preserved: %d, %v", count, err)
			}
			var name, status string
			if err := pool.QueryRow(ctx, `SELECT name FROM erp_gl_accounts WHERE id = $1`, accountID).Scan(&name); err != nil || name != "Canonical cash" {
				t.Fatalf("account identity or canonical value was lost: %q, %v", name, err)
			}
			if err := pool.QueryRow(ctx, `SELECT name FROM erp_gl_cost_centers WHERE id = $1`, centerID).Scan(&name); err != nil || name != "Canonical center" {
				t.Fatalf("cost-center identity or canonical value was lost: %q, %v", name, err)
			}
			if err := pool.QueryRow(ctx, `SELECT status FROM erp_gl_journal_entries WHERE id = $1`, voidID).Scan(&status); err != nil || status != "void" {
				t.Fatalf("void journal status changed: %q, %v", status, err)
			}
			query, want := `SELECT COUNT(*) FROM platform.platform_migration_checksum_history WHERE repair_filename IN ('030_operational_ontology_ledger.sql','031_ontology_tools.sql','032_core_business_boundary.sql')`, 3
			if scope == "tenant" {
				query, want = `SELECT COUNT(*) FROM tenant_migration_checksum_history WHERE repair_filename = '008_operational_ontology_ledger.sql'`, 1
			}
			if err := pool.QueryRow(ctx, query).Scan(&count); err != nil || count != want {
				t.Fatalf("checksum reconciliation history = %d, want %d: %v", count, want, err)
			}
			if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM pg_constraint WHERE NOT convalidated`).Scan(&count); err != nil || count != 0 {
				t.Fatalf("unvalidated constraints = %d: %v", count, err)
			}
			upgradeSQL, err := os.ReadFile(filepath.Join(current, "030_operational_ontology_ledger.sql"))
			if err != nil {
				t.Fatal(err)
			}
			for _, tc := range []struct{ name, setup, message string }{
				{"duplicate account", `WITH org AS (INSERT INTO organizations(name) VALUES ('Conflicting upgrade organization') RETURNING id)
				 INSERT INTO gl_accounts(account_code, name, organization_id) SELECT 'UPGRADE-CASH', 'Conflicting cash', id FROM org`, "duplicate legacy"},
				{"duplicate cost center", `WITH org AS (INSERT INTO organizations(name) VALUES ('Conflicting upgrade organization') RETURNING id)
				 INSERT INTO gl_cost_centers(cost_center_code, name, organization_id) SELECT 'UPGRADE-CENTER', 'Conflicting center', id FROM org`, "duplicate legacy"},
				{"conflicting identity", `UPDATE "MACT" SET "Payload" = "Payload" || jsonb_build_object('LegacyID', gen_random_uuid()) WHERE "AcctCode" = 'UPGRADE-CASH'`, "conflicting canonical"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					tx, err := pool.Begin(ctx)
					if err != nil {
						t.Fatal(err)
					}
					defer tx.Rollback(ctx)
					if _, err := tx.Exec(ctx, tc.setup); err != nil {
						t.Fatal(err)
					}
					if _, err := tx.Exec(ctx, string(upgradeSQL)); err == nil || !strings.Contains(err.Error(), tc.message) {
						t.Fatalf("ambiguous legacy import was not rejected: %v", err)
					}
				})
			}
		})
	}
}

func preOntologyMigrationSnapshot(t *testing.T, current string) string {
	t.Helper()
	dir := t.TempDir()
	// The refactor only appends these sections to the staged baselines.
	markers := map[string]string{
		"001_erp_code_baseline.sql":                 "-- ERP is the authoritative business object store.",
		"002_erp_platform_integration_baseline.sql": "-- Operational Ontology: retire duplicate",
		"004_ai_capability_baseline.sql":            "-- Ontology tools are platform AI capability metadata",
	}
	err := filepath.WalkDir(current, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".sql" {
			return nil
		}
		relative, err := filepath.Rel(current, path)
		if err != nil {
			return err
		}
		isTenant := filepath.Dir(relative) == "tenant"
		if (!isTenant && entry.Name() >= "030") || (isTenant && entry.Name() >= "008") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if marker, ok := markers[relative]; ok {
			prefix, _, found := strings.Cut(string(data), marker)
			if !found {
				return fmt.Errorf("missing baseline boundary %s", relative)
			}
			data = []byte(prefix)
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

func ontologyUpgradeDatabase(t *testing.T, ctx context.Context) (*pgxpool.Pool, string) {
	t.Helper()
	adminURL := os.Getenv("MIGRATION_TEST_ADMIN_URL")
	if adminURL == "" {
		adminURL = "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
	}
	admin, err := pgxpool.New(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	name := "meta_org_ontology_upgrade_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	quoted := pgx.Identifier{name}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+quoted); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	databaseURL, err := tenantdb.DatabaseURLForName(adminURL, name)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, _ = admin.Exec(cleanupCtx, "DROP DATABASE "+quoted+" WITH (FORCE)")
		admin.Close()
	})
	return pool, databaseURL
}
