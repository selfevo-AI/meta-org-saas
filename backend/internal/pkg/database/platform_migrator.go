package database

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type platformMigrationFile struct {
	Filename             string
	SQL                  string
	Checksum             string
	AcceptsChecksumDrift []string
}

type platformChecksumReconciliation struct {
	Filename         string
	PreviousChecksum string
	AcceptedChecksum string
	RepairFilename   string
}

func RunMigrations(ctx context.Context, pool *pgxpool.Pool, migrationsDir string) error {
	if _, err := pool.Exec(ctx, platformMigrationTrackingSQL()); err != nil {
		return fmt.Errorf("create migration tracking table: %w", err)
	}
	if _, err := pool.Exec(ctx, migrateLegacyPlatformMigrationRunsSQL()); err != nil {
		return fmt.Errorf("migrate legacy migration tracking table: %w", err)
	}

	files, err := loadPlatformMigrationFiles(migrationsDir)
	if err != nil {
		return err
	}
	applied, err := loadAppliedPlatformMigrations(ctx, pool)
	if err != nil {
		return err
	}
	if err := backfillUnknownPlatformChecksums(ctx, pool, files, applied); err != nil {
		return err
	}

	reconciliationsByRepair := map[string][]platformChecksumReconciliation{}
	for index, file := range files {
		appliedChecksum, ok := applied[file.Filename]
		if !ok || appliedChecksum == file.Checksum {
			continue
		}
		repairFilename := pendingPlatformChecksumRepairFor(files, applied, index, file.Filename)
		if repairFilename == "" {
			return fmt.Errorf("platform migration checksum drift for %s", file.Filename)
		}
		reconciliationsByRepair[repairFilename] = append(reconciliationsByRepair[repairFilename], platformChecksumReconciliation{
			Filename:         file.Filename,
			PreviousChecksum: appliedChecksum,
			AcceptedChecksum: file.Checksum,
			RepairFilename:   repairFilename,
		})
	}

	for _, file := range files {
		if appliedChecksum, ok := applied[file.Filename]; ok {
			if appliedChecksum == file.Checksum {
				fmt.Printf("migration skipped (already applied): %s\n", file.Filename)
			} else {
				fmt.Printf("migration checksum reconciliation pending: %s via %s\n", file.Filename, pendingPlatformChecksumRepairFor(files, applied, platformMigrationIndex(files, file.Filename), file.Filename))
			}
			continue
		}
		if err := applyPlatformMigration(ctx, pool, file, reconciliationsByRepair[file.Filename]); err != nil {
			return err
		}
		fmt.Printf("migration applied: %s\n", file.Filename)
	}
	return nil
}

func loadPlatformMigrationFiles(migrationsDir string) ([]platformMigrationFile, error) {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}
	filenames := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		filenames = append(filenames, entry.Name())
	}
	sort.Strings(filenames)

	files := make([]platformMigrationFile, 0, len(filenames))
	for _, filename := range filenames {
		content, err := os.ReadFile(filepath.Join(migrationsDir, filename))
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", filename, err)
		}
		accepted, err := acceptedPlatformChecksumDriftFilenames(string(content))
		if err != nil {
			return nil, fmt.Errorf("read migration %s directives: %w", filename, err)
		}
		sum := sha256.Sum256(content)
		files = append(files, platformMigrationFile{
			Filename:             filename,
			SQL:                  string(content),
			Checksum:             hex.EncodeToString(sum[:]),
			AcceptsChecksumDrift: accepted,
		})
	}
	return files, nil
}

func acceptedPlatformChecksumDriftFilenames(sql string) ([]string, error) {
	const directive = "-- platformdb:accept-checksum-drift "
	accepted := make([]string, 0)
	seen := map[string]bool{}
	for _, line := range strings.Split(sql, "\n") {
		value, ok := strings.CutPrefix(strings.TrimSpace(line), directive)
		if !ok {
			continue
		}
		filename := strings.TrimSpace(value)
		if filename == "" || filename != filepath.Base(filename) || filepath.Ext(filename) != ".sql" {
			return nil, fmt.Errorf("invalid checksum drift filename %q", filename)
		}
		if seen[filename] {
			continue
		}
		seen[filename] = true
		accepted = append(accepted, filename)
	}
	sort.Strings(accepted)
	return accepted, nil
}

func loadAppliedPlatformMigrations(ctx context.Context, pool *pgxpool.Pool) (map[string]string, error) {
	rows, err := pool.Query(ctx, `SELECT filename, checksum FROM platform.platform_migration_runs`)
	if err != nil {
		return nil, fmt.Errorf("list applied platform migrations: %w", err)
	}
	defer rows.Close()
	applied := map[string]string{}
	for rows.Next() {
		var filename, checksum string
		if err := rows.Scan(&filename, &checksum); err != nil {
			return nil, fmt.Errorf("scan applied platform migration: %w", err)
		}
		applied[filename] = checksum
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applied platform migrations: %w", err)
	}
	return applied, nil
}

func backfillUnknownPlatformChecksums(ctx context.Context, pool *pgxpool.Pool, files []platformMigrationFile, applied map[string]string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin platform checksum backfill: %w", err)
	}
	defer tx.Rollback(ctx)
	for _, file := range files {
		checksum, ok := applied[file.Filename]
		if !ok || checksum != "" {
			continue
		}
		tag, err := tx.Exec(ctx, `
			UPDATE platform.platform_migration_runs
			SET checksum = $2
			WHERE filename = $1 AND checksum = ''
		`, file.Filename, file.Checksum)
		if err != nil {
			return fmt.Errorf("backfill platform migration checksum %s: %w", file.Filename, err)
		}
		if tag.RowsAffected() != 1 {
			return fmt.Errorf("backfill platform migration checksum %s: updated %d rows", file.Filename, tag.RowsAffected())
		}
		applied[file.Filename] = file.Checksum
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit platform checksum backfill: %w", err)
	}
	return nil
}

func pendingPlatformChecksumRepairFor(files []platformMigrationFile, applied map[string]string, driftIndex int, filename string) string {
	if driftIndex < 0 {
		return ""
	}
	for _, candidate := range files[driftIndex+1:] {
		if _, alreadyApplied := applied[candidate.Filename]; alreadyApplied {
			continue
		}
		for _, accepted := range candidate.AcceptsChecksumDrift {
			if accepted == filename {
				return candidate.Filename
			}
		}
	}
	return ""
}

func platformMigrationIndex(files []platformMigrationFile, filename string) int {
	for index, file := range files {
		if file.Filename == filename {
			return index
		}
	}
	return -1
}

func applyPlatformMigration(ctx context.Context, pool *pgxpool.Pool, file platformMigrationFile, reconciliations []platformChecksumReconciliation) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction for %s: %w", file.Filename, err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, file.SQL); err != nil {
		return fmt.Errorf("execute migration %s: %w", file.Filename, err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO platform.platform_migration_runs(filename, checksum)
		VALUES ($1, $2)
	`, file.Filename, file.Checksum); err != nil {
		return fmt.Errorf("record migration %s: %w", file.Filename, err)
	}
	for _, reconciliation := range reconciliations {
		if _, err := tx.Exec(ctx, `
			INSERT INTO platform.platform_migration_checksum_history(
			    filename, previous_checksum, accepted_checksum, repair_filename
			)
			VALUES ($1, $2, $3, $4)
		`, reconciliation.Filename, reconciliation.PreviousChecksum, reconciliation.AcceptedChecksum, reconciliation.RepairFilename); err != nil {
			return fmt.Errorf("record platform checksum reconciliation for %s: %w", reconciliation.Filename, err)
		}
		tag, err := tx.Exec(ctx, `
			UPDATE platform.platform_migration_runs
			SET checksum = $2
			WHERE filename = $1 AND checksum = $3
		`, reconciliation.Filename, reconciliation.AcceptedChecksum, reconciliation.PreviousChecksum)
		if err != nil {
			return fmt.Errorf("update platform checksum reconciliation for %s: %w", reconciliation.Filename, err)
		}
		if tag.RowsAffected() != 1 {
			return fmt.Errorf("update platform checksum reconciliation for %s: updated %d rows", reconciliation.Filename, tag.RowsAffected())
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration %s: %w", file.Filename, err)
	}
	return nil
}
