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

	"github.com/jackc/pgx/v5/pgxpool"
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
		WHERE package_key = 'local_manufacturing_demo'
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
		WHERE pkg.package_key = 'local_manufacturing_demo'
		  AND listing.listing_type = 'industry_solution'
		  AND listing.status = 'published'
	`).Scan(&listingCount); err != nil {
		t.Fatalf("count sample industry solution listing: %v", err)
	}
	if listingCount != 1 {
		t.Fatalf("sample industry solution listing count = %d, want 1", listingCount)
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
