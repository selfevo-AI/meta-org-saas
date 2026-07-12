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
	if result.Version != "004_project_erp_organization_scope" {
		t.Fatalf("tenant migration version = %q, want 004_project_erp_organization_scope", result.Version)
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
		`public."MPRJ"`,
		`public."APRJ"`,
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

	assertProjectERPProjection(t, ctx, targetPool, orgID)
}

func assertProjectERPProjection(t *testing.T, ctx context.Context, db *pgxpool.Pool, orgID uuid.UUID) {
	t.Helper()
	projectID := uuid.New()
	if _, err := db.Exec(ctx, `
		INSERT INTO projects(id, organization_id, name, status, metadata)
		VALUES ($1, $2, 'Lifecycle Project', 'active', '{}')`, projectID, orgID); err != nil {
		t.Fatalf("create lifecycle project: %v", err)
	}

	var projectedName string
	if err := db.QueryRow(ctx, `SELECT "Payload"->>'Name' FROM "MPRJ" WHERE "PrjCode" = $1`, projectID.String()).Scan(&projectedName); err != nil {
		t.Fatalf("read lifecycle project through MPRJ: %v", err)
	}
	if projectedName != "Lifecycle Project" {
		t.Fatalf("MPRJ lifecycle project name = %q, want Lifecycle Project", projectedName)
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO "MPRJ"("PrjCode", "Payload")
		VALUES ('PRJ-COMPAT', '{"Active":"Y","Payload":{"Name":"ERP Compatibility Project"}}')`); err != nil {
		t.Fatalf("create project through MPRJ: %v", err)
	}
	var compatibilityProjectID uuid.UUID
	var compatibilityOrganizationID uuid.UUID
	if err := db.QueryRow(ctx, `SELECT id, organization_id FROM projects WHERE metadata->>'erp_project_code' = 'PRJ-COMPAT'`).Scan(&compatibilityProjectID, &compatibilityOrganizationID); err != nil {
		t.Fatalf("read MPRJ-created authoritative project: %v", err)
	}
	if compatibilityOrganizationID != orgID {
		t.Fatalf("MPRJ project organization = %s, want %s", compatibilityOrganizationID, orgID)
	}

	if _, err := db.Exec(ctx, `
		UPDATE "MPRJ"
		SET "Payload" = "Payload" || '{"Name":"Updated Compatibility Project","LastCostCode":"COST-PRJ-COMPAT"}'::JSONB
		WHERE "PrjCode" = 'PRJ-COMPAT'`); err != nil {
		t.Fatalf("update project through MPRJ: %v", err)
	}
	var updatedName, lastCostCode string
	if err := db.QueryRow(ctx, `SELECT name, metadata->>'LastCostCode' FROM projects WHERE id = $1`, compatibilityProjectID).Scan(&updatedName, &lastCostCode); err != nil {
		t.Fatalf("read MPRJ-updated authoritative project: %v", err)
	}
	if updatedName != "Updated Compatibility Project" || lastCostCode != "COST-PRJ-COMPAT" {
		t.Fatalf("MPRJ update = (%q, %q), want updated name and cost code", updatedName, lastCostCode)
	}

	memberID := uuid.New()
	actorID := uuid.MustParse("123e4567-e89b-12d3-a456-426614174001")
	if _, err := db.Exec(ctx, `
		INSERT INTO project_members(id, project_id, actor_id, actor_type, role)
		VALUES ($1, $2, $3, 'internal_human', 'owner')`, memberID, compatibilityProjectID, actorID); err != nil {
		t.Fatalf("create lifecycle project member: %v", err)
	}
	var projectedMemberID string
	if err := db.QueryRow(ctx, `SELECT "Payload"->>'ProjectMemberID' FROM "APRJ" WHERE "PrjCode" = 'PRJ-COMPAT'`).Scan(&projectedMemberID); err != nil {
		t.Fatalf("read project member through APRJ: %v", err)
	}
	if projectedMemberID != memberID.String() {
		t.Fatalf("APRJ member id = %q, want %q", projectedMemberID, memberID)
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
