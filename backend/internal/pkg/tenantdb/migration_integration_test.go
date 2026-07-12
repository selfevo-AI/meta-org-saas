package tenantdb

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestFreshTenantBusinessMigrationAgainstPostgres(t *testing.T) {
	if os.Getenv("RUN_FRESH_TENANT_DB_MIGRATION_TEST") != "1" {
		t.Skip("set RUN_FRESH_TENANT_DB_MIGRATION_TEST=1 to run fresh tenant PostgreSQL migration verification")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	adminURL := strings.TrimSpace(os.Getenv("MIGRATION_TEST_ADMIN_URL"))
	if adminURL == "" {
		adminURL = "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
	}
	dbName := fmt.Sprintf("meta_org_migration_check_%d", time.Now().UnixNano())

	adminPool, err := pgxpool.New(ctx, adminURL)
	if err != nil {
		t.Fatalf("connect admin database: %v", err)
	}
	var targetPool *pgxpool.Pool
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if targetPool != nil {
			targetPool.Close()
		}
		_, _ = adminPool.Exec(cleanupCtx, `SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1`, dbName)
		_, _ = adminPool.Exec(cleanupCtx, `DROP DATABASE IF EXISTS "`+dbName+`"`)
		adminPool.Close()
	})

	if _, err := adminPool.Exec(ctx, `CREATE DATABASE "`+dbName+`"`); err != nil {
		t.Fatalf("create fresh tenant migration database: %v", err)
	}
	targetURL, err := DatabaseURLForName(adminURL, dbName)
	if err != nil {
		t.Fatalf("build tenant database url: %v", err)
	}

	orgID := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")
	target := NewDedicatedDatabaseTarget(orgID, "meta_org_", "local-primary", "local")
	target.DatabaseName = dbName
	result, err := FileTenantMigrator{MigrationsDir: repoTenantMigrationsDirForIntegration(t)}.Migrate(ctx, target, targetURL)
	if err != nil {
		t.Fatalf("run fresh tenant migrations: %v", err)
	}
	if result.Version != "002_tenant_projection_outbox" {
		t.Fatalf("tenant migration version = %q, want 002_tenant_projection_outbox", result.Version)
	}

	targetPool, err = pgxpool.New(ctx, targetURL)
	if err != nil {
		t.Fatalf("connect migrated tenant database: %v", err)
	}

	for _, tableRef := range []string{
		"public.sample_work_orders",
		"public.projects",
		"public.workflow_templates",
		"public.tenant_integration_outbox",
		"public.finance_payables",
		`public."MREG"`,
		`public."MITW"`,
		`public."MPOR"`,
		`public."MRDR"`,
		`public."MRPS"`,
		`public."MDRQ"`,
		`public."MCNT"`,
	} {
		assertTenantTableExists(t, ctx, targetPool, tableRef)
	}

	var notValid int
	if err := targetPool.QueryRow(ctx, `SELECT COUNT(*) FROM pg_constraint WHERE NOT convalidated`).Scan(&notValid); err != nil {
		t.Fatalf("count tenant unvalidated constraints: %v", err)
	}
	if notValid != 0 {
		t.Fatalf("tenant not valid constraints = %d, want 0", notValid)
	}

	if err := BootstrapTenantData(ctx, targetPool, TenantBootstrapInput{
		OrganizationID:               orgID,
		OrganizationName:             "Local Manufacturing Demo Tenant",
		Description:                  "Fresh tenant migration integration sample",
		OwnerUserID:                  uuid.MustParse("123e4567-e89b-12d3-a456-426614174001"),
		OwnerName:                    "Sample Owner",
		OwnerEmail:                   "sample-owner@local.test",
		EnabledModules:               []string{"organization", "project", "workflow", "finance", "costing", "erp", "inventory", "procurement", "sales"},
		SampleKey:                    "business_closure_sample",
		IncludeBusinessClosureSample: true,
	}); err != nil {
		t.Fatalf("bootstrap fresh tenant sample data: %v", err)
	}

	var sampleWorkOrders int
	if err := targetPool.QueryRow(ctx, `SELECT COUNT(*) FROM sample_work_orders WHERE organization_id = $1`, orgID).Scan(&sampleWorkOrders); err != nil {
		t.Fatalf("count sample work orders: %v", err)
	}
	if sampleWorkOrders != 1 {
		t.Fatalf("sample work order count = %d, want 1", sampleWorkOrders)
	}
}

func assertTenantTableExists(t *testing.T, ctx context.Context, db *pgxpool.Pool, tableRef string) {
	t.Helper()
	var exists bool
	if err := db.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, tableRef).Scan(&exists); err != nil {
		t.Fatalf("check tenant table %s: %v", tableRef, err)
	}
	if !exists {
		t.Fatalf("tenant table %s does not exist", tableRef)
	}
}

func repoTenantMigrationsDirForIntegration(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "migrations", "tenant")
}
