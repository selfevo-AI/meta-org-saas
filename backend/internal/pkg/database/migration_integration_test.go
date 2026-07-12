package database

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/saas"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/tenantdb"
)

func TestFreshBaselineMigrationsAgainstPostgres(t *testing.T) {
	if os.Getenv("RUN_FRESH_DB_MIGRATION_TEST") != "1" {
		t.Skip("set RUN_FRESH_DB_MIGRATION_TEST=1 to run fresh PostgreSQL migration verification")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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
		t.Fatalf("create fresh migration database: %v", err)
	}

	targetURL, err := databaseURLForTestName(adminURL, dbName)
	if err != nil {
		t.Fatalf("build target database url: %v", err)
	}
	targetPool, err = pgxpool.New(ctx, targetURL)
	if err != nil {
		t.Fatalf("connect fresh migration database: %v", err)
	}

	migrationsDir := repoMigrationsDir(t)
	if err := RunMigrations(ctx, targetPool, migrationsDir); err != nil {
		t.Fatalf("run fresh migrations: %v", err)
	}

	var notValid int
	if err := targetPool.QueryRow(ctx, `SELECT COUNT(*) FROM pg_constraint WHERE NOT convalidated`).Scan(&notValid); err != nil {
		t.Fatalf("count unvalidated constraints: %v", err)
	}
	if notValid != 0 {
		t.Fatalf("not valid constraints = %d, want 0", notValid)
	}

	var sampleCount int
	if err := targetPool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM platform.capability_packages
		WHERE package_key = 'erpnext_manufacturing_demo'
		  AND package_type = 'industry_solution'
		  AND status = 'published'
	`).Scan(&sampleCount); err != nil {
		t.Fatalf("count sample industry solution package: %v", err)
	}
	if sampleCount != 1 {
		t.Fatalf("sample industry solution package count = %d, want 1", sampleCount)
	}

	var listingCount int
	if err := targetPool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM platform.marketplace_listings listing
		JOIN platform.capability_packages pkg ON pkg.id = listing.package_id
		WHERE pkg.package_key = 'erpnext_manufacturing_demo'
		  AND listing.listing_type = 'industry_solution'
		  AND listing.status = 'published'
	`).Scan(&listingCount); err != nil {
		t.Fatalf("count sample industry solution listing: %v", err)
	}
	if listingCount != 1 {
		t.Fatalf("sample industry solution listing count = %d, want 1", listingCount)
	}

	verifyTenantProvisioningJobClaim(t, ctx, targetPool)
}

func verifyTenantProvisioningJobClaim(t *testing.T, ctx context.Context, db *pgxpool.Pool) {
	t.Helper()
	orgID := uuid.New()
	if _, err := db.Exec(ctx, `
		INSERT INTO organizations(id, name, status)
		VALUES ($1, 'Migration provisioning verification', 'active')
	`, orgID); err != nil {
		t.Fatalf("insert provisioning verification organization: %v", err)
	}
	target := tenantdb.NewDedicatedDatabaseTarget(orgID, "meta_org_", "local-primary", "local")
	var targetID uuid.UUID
	if err := db.QueryRow(ctx, `
		INSERT INTO platform.tenant_database_targets(
		    organization_id, deployment_mode, cluster_key, region, database_name, schema_name, status, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, '{}')
		RETURNING id
	`, orgID, target.DeploymentMode, target.ClusterKey, target.Region, target.DatabaseName, target.SchemaName, target.Status).Scan(&targetID); err != nil {
		t.Fatalf("insert provisioning verification target: %v", err)
	}
	jobID := uuid.New()
	if _, err := db.Exec(ctx, `
		INSERT INTO platform.tenant_database_provisioning_jobs(
		    id, organization_id, tenant_database_id, idempotency_key, available_at, bootstrap_payload
		)
		VALUES ($1, $2, $3, $4, '2000-01-01', jsonb_build_object('organization_id', $2::uuid, 'organization_name', 'Migration provisioning verification'))
	`, jobID, orgID, targetID, "migration-verification:"+orgID.String()); err != nil {
		t.Fatalf("insert provisioning verification job: %v", err)
	}

	repo := saas.NewRepository(db)
	job, err := repo.ClaimTenantDatabaseProvisioningJob(ctx, "migration-test-worker", time.Minute)
	if err != nil {
		t.Fatalf("claim provisioning verification job: %v", err)
	}
	if job == nil || job.ID != jobID || job.LeaseOwner != "migration-test-worker" {
		t.Fatalf("claimed provisioning job = %#v", job)
	}
	target.Status = tenantdb.TargetStatusProvisioned
	if err := repo.CompleteTenantDatabaseProvisioningJob(ctx, *job, target, tenantdb.MigrationResult{Version: "001_tenant_business_baseline"}); err != nil {
		t.Fatalf("complete provisioning verification job: %v", err)
	}
	var jobStatus string
	var targetStatus string
	if err := db.QueryRow(ctx, `
		SELECT j.status, t.status
		FROM platform.tenant_database_provisioning_jobs j
		JOIN platform.tenant_database_targets t ON t.id = j.tenant_database_id
		WHERE j.id = $1
	`, jobID).Scan(&jobStatus, &targetStatus); err != nil {
		t.Fatalf("query completed provisioning verification job: %v", err)
	}
	if jobStatus != "succeeded" || targetStatus != tenantdb.TargetStatusProvisioned {
		t.Fatalf("job/target status = %q/%q", jobStatus, targetStatus)
	}
}

func repoMigrationsDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "migrations")
}

func databaseURLForTestName(adminURL string, databaseName string) (string, error) {
	parsed, err := url.Parse(adminURL)
	if err != nil {
		return "", err
	}
	parsed.Path = "/" + databaseName
	return parsed.String(), nil
}
