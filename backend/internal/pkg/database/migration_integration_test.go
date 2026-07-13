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

	var missingChecksums int
	if err := targetPool.QueryRow(ctx, `
		SELECT COUNT(*) FROM platform.platform_migration_runs WHERE checksum = ''
	`).Scan(&missingChecksums); err != nil {
		t.Fatalf("count platform migrations without checksum: %v", err)
	}
	if missingChecksums != 0 {
		t.Fatalf("platform migrations without checksum = %d, want 0", missingChecksums)
	}
	var checksumHistoryExists bool
	if err := targetPool.QueryRow(ctx, `
		SELECT to_regclass('platform.platform_migration_checksum_history') IS NOT NULL
	`).Scan(&checksumHistoryExists); err != nil {
		t.Fatalf("check platform checksum history table: %v", err)
	}
	if !checksumHistoryExists {
		t.Fatal("platform migration checksum history table missing")
	}
	var authRateLimitTableExists bool
	if err := targetPool.QueryRow(ctx, `
		SELECT to_regclass('platform.authentication_rate_limit_buckets') IS NOT NULL
	`).Scan(&authRateLimitTableExists); err != nil {
		t.Fatalf("check authentication rate limit table: %v", err)
	}
	if !authRateLimitTableExists {
		t.Fatal("authentication rate limit bucket table missing")
	}
	var securityNonceTableExists bool
	if err := targetPool.QueryRow(ctx, `
		SELECT to_regclass('platform.security_request_nonces') IS NOT NULL
	`).Scan(&securityNonceTableExists); err != nil {
		t.Fatalf("check security request nonce table: %v", err)
	}
	if !securityNonceTableExists {
		t.Fatal("security request nonce table missing")
	}
	var businessAIStageTableExists bool
	if err := targetPool.QueryRow(ctx, `
		SELECT to_regclass('public.business_stage_ai_runs') IS NOT NULL
	`).Scan(&businessAIStageTableExists); err != nil {
		t.Fatalf("check business stage ai table: %v", err)
	}
	if !businessAIStageTableExists {
		t.Fatal("business stage ai run table missing")
	}
	var retentionRunTableExists bool
	if err := targetPool.QueryRow(ctx, `
		SELECT to_regclass('platform.audit_retention_runs') IS NOT NULL
	`).Scan(&retentionRunTableExists); err != nil {
		t.Fatalf("check audit retention run table: %v", err)
	}
	if !retentionRunTableExists {
		t.Fatal("audit retention run table missing")
	}
	var retentionColumns int
	if err := targetPool.QueryRow(ctx, `
		SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema = 'public' AND column_name = 'retention_redacted_at'
		  AND table_name IN (
		    'ai_invocations', 'business_stage_ai_runs', 'tool_executions',
		    'assistant_sessions', 'assistant_messages', 'assistant_steps'
		  )
	`).Scan(&retentionColumns); err != nil {
		t.Fatalf("check audit retention marker columns: %v", err)
	}
	if retentionColumns != 6 {
		t.Fatalf("audit retention marker columns = %d, want 6", retentionColumns)
	}
	var channelCircuitColumns int
	if err := targetPool.QueryRow(ctx, `
		SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'model_provider_channels'
		  AND column_name IN ('consecutive_failure_count', 'circuit_open_until')
	`).Scan(&channelCircuitColumns); err != nil {
		t.Fatalf("check provider channel circuit columns: %v", err)
	}
	if channelCircuitColumns != 2 {
		t.Fatalf("provider channel circuit columns = %d, want 2", channelCircuitColumns)
	}
	var proposalColumns int
	if err := targetPool.QueryRow(ctx, `
		SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'business_stage_ai_runs'
		  AND column_name IN ('proposal_status', 'tool_execution_id', 'tool_approval_id', 'proposal_result')
	`).Scan(&proposalColumns); err != nil {
		t.Fatalf("check business AI proposal columns: %v", err)
	}
	if proposalColumns != 4 {
		t.Fatalf("business AI proposal columns = %d, want 4", proposalColumns)
	}
	var effectiveStageTools int
	if err := targetPool.QueryRow(ctx, `
		SELECT COUNT(*) FROM tool_definitions
		WHERE name IN ('project.update_status', 'project.create_deliverable', 'project.accept_deliverable', 'project.close_feedback')
		  AND is_active
		  AND default_policy = 'approve'
		  AND tool_category = 'business_approval'
		  AND approval_tier_required = 'reviewer'
		  AND metadata ? 'label_zh'
		  AND metadata ? 'label_en'
		  AND metadata ? 'business_ai_stages'
	`).Scan(&effectiveStageTools); err != nil {
		t.Fatalf("check effective business AI stage tools: %v", err)
	}
	if effectiveStageTools != 4 {
		t.Fatalf("effective business AI stage tools = %d, want 4", effectiveStageTools)
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

func TestPlatformMigrationChecksumReconciliationAgainstPostgres(t *testing.T) {
	if os.Getenv("RUN_FRESH_DB_MIGRATION_TEST") != "1" {
		t.Skip("set RUN_FRESH_DB_MIGRATION_TEST=1 to run platform checksum reconciliation verification")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	adminURL := strings.TrimSpace(os.Getenv("MIGRATION_TEST_ADMIN_URL"))
	if adminURL == "" {
		adminURL = "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
	}
	dbName := fmt.Sprintf("meta_org_checksum_check_%d", time.Now().UnixNano())
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
		t.Fatalf("create checksum database: %v", err)
	}
	targetURL, err := databaseURLForTestName(adminURL, dbName)
	if err != nil {
		t.Fatalf("build checksum database URL: %v", err)
	}
	targetPool, err = pgxpool.New(ctx, targetURL)
	if err != nil {
		t.Fatalf("connect checksum database: %v", err)
	}

	dir := t.TempDir()
	baselinePath := filepath.Join(dir, "001_baseline.sql")
	if err := os.WriteFile(baselinePath, []byte("CREATE TABLE checksum_probe(id integer PRIMARY KEY);\n"), 0o600); err != nil {
		t.Fatalf("write initial baseline: %v", err)
	}
	if err := RunMigrations(ctx, targetPool, dir); err != nil {
		t.Fatalf("run initial platform migration: %v", err)
	}

	if err := os.WriteFile(baselinePath, []byte("CREATE TABLE checksum_probe(id integer PRIMARY KEY, label text);\n"), 0o600); err != nil {
		t.Fatalf("write drifted baseline: %v", err)
	}
	repairPath := filepath.Join(dir, "002_checksum_repair.sql")
	repairSQL := "-- platformdb:accept-checksum-drift 001_baseline.sql\nALTER TABLE checksum_probe ADD COLUMN label text;\n"
	if err := os.WriteFile(repairPath, []byte(repairSQL), 0o600); err != nil {
		t.Fatalf("write checksum repair: %v", err)
	}
	if err := RunMigrations(ctx, targetPool, dir); err != nil {
		t.Fatalf("run checksum reconciliation: %v", err)
	}

	files, err := loadPlatformMigrationFiles(dir)
	if err != nil {
		t.Fatalf("reload platform migrations: %v", err)
	}
	var trackedChecksum string
	if err := targetPool.QueryRow(ctx, `
		SELECT checksum FROM platform.platform_migration_runs WHERE filename = '001_baseline.sql'
	`).Scan(&trackedChecksum); err != nil {
		t.Fatalf("read reconciled checksum: %v", err)
	}
	if trackedChecksum != files[0].Checksum {
		t.Fatalf("tracked checksum = %q, want %q", trackedChecksum, files[0].Checksum)
	}
	var historyCount int
	if err := targetPool.QueryRow(ctx, `
		SELECT COUNT(*) FROM platform.platform_migration_checksum_history
		WHERE filename = '001_baseline.sql' AND repair_filename = '002_checksum_repair.sql'
	`).Scan(&historyCount); err != nil {
		t.Fatalf("count checksum history: %v", err)
	}
	if historyCount != 1 {
		t.Fatalf("checksum history count = %d, want 1", historyCount)
	}

	if err := os.WriteFile(baselinePath, []byte("CREATE TABLE checksum_probe(id integer PRIMARY KEY, label text, note text);\n"), 0o600); err != nil {
		t.Fatalf("write second drift: %v", err)
	}
	if err := RunMigrations(ctx, targetPool, dir); err == nil || !strings.Contains(err.Error(), "checksum drift") {
		t.Fatalf("second drift error = %v, want checksum drift rejection", err)
	}
}
