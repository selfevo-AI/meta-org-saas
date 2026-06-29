package tenantdb

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

type TenantMigrationFile struct {
	Filename string
	Path     string
	SQL      string
	Checksum string
	Stage    MigrationStage
}

type TenantMigrationStore interface {
	EnsureMigrationTable(context.Context) error
	AppliedMigrations(context.Context) (map[string]string, error)
	ApplyMigration(context.Context, TenantMigrationFile) error
	Close()
}

type TenantMigrationStoreOpener func(context.Context, string) (TenantMigrationStore, error)

type FileTenantMigrator struct {
	MigrationsDir string
	OpenStore     TenantMigrationStoreOpener
}

const tenantMigrationTrackingTable = "tenant_migration_runs"

func tenantMigrationTrackingSQL() string {
	return `
		CREATE TABLE IF NOT EXISTS tenant_migration_runs (
			filename   TEXT PRIMARY KEY,
			checksum   TEXT NOT NULL,
			scope      TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`
}

func migrateLegacyTenantMigrationRunsSQL() string {
	return `
		DO $$
		BEGIN
			IF to_regclass('tenant_schema_migrations') IS NOT NULL THEN
				INSERT INTO tenant_migration_runs(filename, checksum, scope, applied_at)
				SELECT filename, checksum, scope, applied_at FROM tenant_schema_migrations
				ON CONFLICT (filename) DO NOTHING;
				DROP TABLE tenant_schema_migrations;
			END IF;
		END;
		$$;
	`
}

func (m FileTenantMigrator) Migrate(ctx context.Context, _ Target, tenantDatabaseURL string) (MigrationResult, error) {
	files, err := LoadTenantMigrationFiles(m.migrationsDir())
	if err != nil {
		return MigrationResult{}, err
	}
	openStore := m.OpenStore
	if openStore == nil {
		openStore = OpenPGTenantMigrationStore
	}
	store, err := openStore(ctx, tenantDatabaseURL)
	if err != nil {
		return MigrationResult{}, fmt.Errorf("open tenant migration store: %w", err)
	}
	defer store.Close()

	if err := store.EnsureMigrationTable(ctx); err != nil {
		return MigrationResult{}, err
	}
	appliedChecksums, err := store.AppliedMigrations(ctx)
	if err != nil {
		return MigrationResult{}, err
	}

	appliedNames := []string{}
	skippedNames := []string{}
	appliedStages := []MigrationStage{}
	checksums := map[string]string{}
	for _, file := range files {
		checksums[file.Filename] = file.Checksum
		if appliedChecksum, ok := appliedChecksums[file.Filename]; ok {
			if appliedChecksum != file.Checksum {
				return MigrationResult{}, fmt.Errorf("tenant migration checksum drift for %s", file.Filename)
			}
			skippedNames = append(skippedNames, file.Filename)
			continue
		}
		if err := store.ApplyMigration(ctx, file); err != nil {
			return MigrationResult{}, fmt.Errorf("apply tenant migration %s: %w", file.Filename, err)
		}
		appliedNames = append(appliedNames, file.Filename)
		appliedStages = append(appliedStages, file.Stage)
	}

	return MigrationResult{
		Version:       TenantMigrationVersion(files),
		AppliedStages: appliedStages,
		Metadata: map[string]any{
			"migration_mode":          "tenant_business",
			"migration_files_applied": appliedNames,
			"migration_files_skipped": skippedNames,
			"migration_checksums":     checksums,
			"migration_total_files":   len(files),
		},
	}, nil
}

func (m FileTenantMigrator) migrationsDir() string {
	if strings.TrimSpace(m.MigrationsDir) == "" {
		return "migrations/tenant"
	}
	return m.MigrationsDir
}

func LoadTenantMigrationFiles(migrationsDir string) ([]TenantMigrationFile, error) {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return nil, fmt.Errorf("read tenant migrations dir: %w", err)
	}

	filenames := []string{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		filenames = append(filenames, entry.Name())
	}
	sort.Strings(filenames)
	if len(filenames) == 0 {
		return nil, fmt.Errorf("tenant migrations dir %q contains no sql files", migrationsDir)
	}

	files := make([]TenantMigrationFile, 0, len(filenames))
	for _, filename := range filenames {
		path := filepath.Join(migrationsDir, filename)
		sql, err := readTenantMigrationSQL(path, map[string]bool{})
		if err != nil {
			return nil, fmt.Errorf("read tenant migration %s: %w", filename, err)
		}
		sum := sha256.Sum256([]byte(sql))
		files = append(files, TenantMigrationFile{
			Filename: filename,
			Path:     path,
			SQL:      sql,
			Checksum: hex.EncodeToString(sum[:]),
			Stage: MigrationStage{
				Name:  migrationStageName(filename),
				Scope: MigrationScopeTenantBusiness,
			},
		})
	}
	return files, nil
}

func readTenantMigrationSQL(path string, seen map[string]bool) (string, error) {
	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if seen[cleanPath] {
		return "", fmt.Errorf("tenant migration include cycle at %s", path)
	}
	seen[cleanPath] = true
	defer delete(seen, cleanPath)

	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	var out strings.Builder
	for _, line := range strings.SplitAfter(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		includePath, ok := strings.CutPrefix(trimmed, "-- tenantdb:include ")
		if !ok {
			out.WriteString(line)
			continue
		}
		out.WriteString(line)
		includePath = strings.TrimSpace(includePath)
		if filepath.IsAbs(includePath) {
			return "", fmt.Errorf("tenant migration include path must be relative: %s", includePath)
		}
		expanded, err := readTenantMigrationSQL(filepath.Join(filepath.Dir(path), includePath), seen)
		if err != nil {
			return "", err
		}
		out.WriteString(expanded)
		if !strings.HasSuffix(expanded, "\n") {
			out.WriteByte('\n')
		}
	}
	return out.String(), nil
}

func TenantMigrationVersion(files []TenantMigrationFile) string {
	if len(files) == 0 {
		return ""
	}
	return migrationStageName(files[len(files)-1].Filename)
}

func migrationStageName(filename string) string {
	return strings.TrimSuffix(filename, filepath.Ext(filename))
}

type PGTenantMigrationStore struct {
	pool *pgxpool.Pool
}

func OpenPGTenantMigrationStore(ctx context.Context, tenantDatabaseURL string) (TenantMigrationStore, error) {
	pool, err := pgxpool.New(ctx, tenantDatabaseURL)
	if err != nil {
		return nil, err
	}
	return &PGTenantMigrationStore{pool: pool}, nil
}

func (s *PGTenantMigrationStore) EnsureMigrationTable(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, tenantMigrationTrackingSQL())
	if err != nil {
		return fmt.Errorf("ensure tenant migration tracking table: %w", err)
	}
	if _, err := s.pool.Exec(ctx, migrateLegacyTenantMigrationRunsSQL()); err != nil {
		return fmt.Errorf("migrate legacy tenant migration tracking table: %w", err)
	}
	return nil
}

func (s *PGTenantMigrationStore) AppliedMigrations(ctx context.Context) (map[string]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT filename, checksum FROM tenant_migration_runs`)
	if err != nil {
		return nil, fmt.Errorf("list applied tenant migrations: %w", err)
	}
	defer rows.Close()

	items := map[string]string{}
	for rows.Next() {
		var filename string
		var checksum string
		if err := rows.Scan(&filename, &checksum); err != nil {
			return nil, fmt.Errorf("scan applied tenant migration: %w", err)
		}
		items[filename] = checksum
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applied tenant migrations: %w", err)
	}
	return items, nil
}

func (s *PGTenantMigrationStore) ApplyMigration(ctx context.Context, file TenantMigrationFile) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tenant migration transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, file.SQL); err != nil {
		return fmt.Errorf("execute tenant migration SQL: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO tenant_migration_runs(filename, checksum, scope)
		VALUES ($1, $2, $3)
	`, file.Filename, file.Checksum, file.Stage.Scope); err != nil {
		return fmt.Errorf("record tenant migration: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tenant migration: %w", err)
	}
	return nil
}

func (s *PGTenantMigrationStore) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}
