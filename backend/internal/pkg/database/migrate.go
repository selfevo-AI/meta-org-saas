package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

const platformMigrationTrackingTable = "platform.platform_migration_runs"

func platformMigrationTrackingSQL() string {
	return `
		CREATE SCHEMA IF NOT EXISTS platform;
		CREATE TABLE IF NOT EXISTS platform.platform_migration_runs (
			filename    VARCHAR(255) PRIMARY KEY,
			applied_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`
}

func migrateLegacyPlatformMigrationRunsSQL() string {
	return `
		DO $$
		BEGIN
			IF to_regclass('public.schema_migrations') IS NOT NULL THEN
				INSERT INTO platform.platform_migration_runs(filename, applied_at)
				SELECT filename, applied_at FROM public.schema_migrations
				ON CONFLICT (filename) DO NOTHING;
				DROP TABLE public.schema_migrations;
			END IF;
		END;
		$$;
	`
}

func RunMigrations(ctx context.Context, pool *pgxpool.Pool, migrationsDir string) error {
	if _, err := pool.Exec(ctx, platformMigrationTrackingSQL()); err != nil {
		return fmt.Errorf("create migration tracking table: %w", err)
	}
	if _, err := pool.Exec(ctx, migrateLegacyPlatformMigrationRunsSQL()); err != nil {
		return fmt.Errorf("migrate legacy migration tracking table: %w", err)
	}

	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	var sqlFiles []string
	for _, f := range files {
		if !f.IsDir() && filepath.Ext(f.Name()) == ".sql" {
			sqlFiles = append(sqlFiles, f.Name())
		}
	}
	sort.Strings(sqlFiles)

	for _, f := range sqlFiles {
		var applied bool
		err := pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM platform.platform_migration_runs WHERE filename = $1)", f).Scan(&applied)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", f, err)
		}
		if applied {
			fmt.Printf("migration skipped (already applied): %s\n", f)
			continue
		}

		content, err := os.ReadFile(filepath.Join(migrationsDir, f))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", f, err)
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin transaction for %s: %w", f, err)
		}

		if _, err := tx.Exec(ctx, string(content)); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("execute migration %s: %w", f, err)
		}

		if _, err := tx.Exec(ctx, "INSERT INTO platform.platform_migration_runs (filename) VALUES ($1)", f); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("record migration %s: %w", f, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", f, err)
		}

		fmt.Printf("migration applied: %s\n", f)
	}
	return nil
}
