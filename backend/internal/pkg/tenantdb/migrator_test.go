package tenantdb

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestLoadTenantMigrationFilesReturnsSortedSQLFilesWithChecksums(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "002_extra.sql", "CREATE TABLE second(id UUID PRIMARY KEY);\n")
	writeTestFile(t, dir, "README.md", "ignore me")
	writeTestFile(t, dir, "001_tenant_business_baseline.sql", "CREATE TABLE first(id UUID PRIMARY KEY);\n")

	files, err := LoadTenantMigrationFiles(dir)
	if err != nil {
		t.Fatalf("LoadTenantMigrationFiles() error = %v", err)
	}

	if len(files) != 2 {
		t.Fatalf("len(files) = %d, want 2", len(files))
	}
	if files[0].Filename != "001_tenant_business_baseline.sql" || files[1].Filename != "002_extra.sql" {
		t.Fatalf("files sorted by filename = %#v", files)
	}
	if files[0].Stage.Name != "001_tenant_business_baseline" {
		t.Fatalf("stage name = %q", files[0].Stage.Name)
	}
	if files[0].Stage.Scope != MigrationScopeTenantBusiness {
		t.Fatalf("stage scope = %q", files[0].Stage.Scope)
	}
	if files[0].Checksum == "" || files[0].Checksum == files[1].Checksum {
		t.Fatalf("checksums = %q, %q; want non-empty distinct values", files[0].Checksum, files[1].Checksum)
	}
}

func TestLoadTenantMigrationFilesExpandsRelativeIncludeDirectives(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "shared.inc", "CREATE TABLE included(id UUID PRIMARY KEY);\n")
	writeTestFile(t, dir, "001_tenant_business_baseline.sql", "CREATE EXTENSION IF NOT EXISTS pgcrypto;\n-- tenantdb:include shared.inc\n")

	files, err := LoadTenantMigrationFiles(dir)
	if err != nil {
		t.Fatalf("LoadTenantMigrationFiles() error = %v", err)
	}

	if !strings.Contains(files[0].SQL, "CREATE TABLE included") {
		t.Fatalf("expanded SQL = %q, want included file content", files[0].SQL)
	}
}

func TestTenantMigrationVersionUsesLastSQLStage(t *testing.T) {
	files := []TenantMigrationFile{
		{Filename: "001_tenant_business_baseline.sql"},
		{Filename: "010_finance_extension.sql"},
	}

	got := TenantMigrationVersion(files)

	if got != "010_finance_extension" {
		t.Fatalf("TenantMigrationVersion() = %q, want 010_finance_extension", got)
	}
}

func TestLoadTenantMigrationFilesRejectsDirectoryWithoutSQL(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "README.md", "no sql")

	_, err := LoadTenantMigrationFiles(dir)

	if err == nil {
		t.Fatalf("LoadTenantMigrationFiles() succeeded, want error for empty tenant migration directory")
	}
}

func TestRepositoryTenantBusinessBaselineDeclaresPhysicalTenantRuntime(t *testing.T) {
	files, err := LoadTenantMigrationFiles(repoTenantMigrationsDir(t))
	if err != nil {
		t.Fatalf("LoadTenantMigrationFiles(repo tenant migrations) error = %v", err)
	}

	if len(files) != 4 {
		t.Fatalf("tenant migration file count = %d, want 4", len(files))
	}
	sql := files[0].SQL
	for _, snippet := range []string{
		"CREATE TABLE IF NOT EXISTS sample_work_orders",
		"CREATE TABLE IF NOT EXISTS workflow_templates",
		"CREATE TABLE IF NOT EXISTS saas_modules",
		"-- tenantdb:include ../001_erp_code_baseline.sql",
		"('MPOR','Purchase Order','purchase','DocEntry','master','')",
		"('MRDR','Sales Order','sale','DocEntry','master','')",
		"('MITW','Items - Warehouse','product','ItemCode','master','')",
		"('MRPS','Retail POS Sale','retail','DocEntry','master','')",
		"('MBOM','Bill of Materials','manufacturing','BOMCode','master','')",
		"('MWOR','Work Order','manufacturing','WorkOrderCode','master','')",
		"('manufacturing', 'MBOM', 'bill_of_materials'",
		"('manufacturing', 'Manufacturing', 'industry', true, 'commercial'",
		"CREATE TABLE IF NOT EXISTS finance_payables",
	} {
		if !strings.Contains(sql, snippet) {
			t.Fatalf("tenant baseline SQL missing %q", snippet)
		}
	}
	for _, legacySnippet := range []string{
		"CREATE TABLE IF NOT EXISTS inventory_balances",
		"CREATE TABLE IF NOT EXISTS purchase_orders",
		"CREATE TABLE IF NOT EXISTS sales_orders",
		"CREATE TABLE IF NOT EXISTS inventory_counts",
		"CREATE TABLE IF NOT EXISTS inventory_transfers",
		"CREATE TABLE IF NOT EXISTS sales_shipments",
	} {
		if strings.Contains(sql, legacySnippet) {
			t.Fatalf("tenant baseline SQL still declares legacy semantic supply-chain table with %q", legacySnippet)
		}
	}
	projectionSQL := files[1].SQL
	for _, snippet := range []string{"tenantdb:accept-checksum-drift 001_tenant_business_baseline.sql", "CREATE TABLE IF NOT EXISTS tenant_integration_outbox", "emit_tenant_projection_outbox_event"} {
		if !strings.Contains(projectionSQL, snippet) {
			t.Fatalf("tenant projection migration SQL missing %q", snippet)
		}
	}
	projectProjectionSQL := files[2].SQL
	for _, snippet := range []string{
		"tenantdb:accept-checksum-drift 001_tenant_business_baseline.sql",
		`ALTER TABLE "MPRJ" RENAME TO "MPRJ_legacy"`,
		`CREATE OR REPLACE VIEW "MPRJ"`,
		`CREATE OR REPLACE VIEW "APRJ"`,
		"write_mprj_project_projection",
	} {
		if !strings.Contains(projectProjectionSQL, snippet) {
			t.Fatalf("tenant project projection migration SQL missing %q", snippet)
		}
	}
	organizationScopeSQL := files[3].SQL
	for _, snippet := range []string{
		"tenantdb:accept-checksum-drift 001_tenant_business_baseline.sql",
		"v_organization_id UUID",
		"SET organization_id = tenant_org.id",
	} {
		if !strings.Contains(organizationScopeSQL, snippet) {
			t.Fatalf("tenant project organization scope migration SQL missing %q", snippet)
		}
	}
}

func TestFileTenantMigratorAppliesUnappliedFilesAndSkipsMatchingChecksums(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "001_tenant_business_baseline.sql", "CREATE TABLE first(id UUID PRIMARY KEY);\n")
	writeTestFile(t, dir, "002_finance_extension.sql", "CREATE TABLE second(id UUID PRIMARY KEY);\n")
	files, err := LoadTenantMigrationFiles(dir)
	if err != nil {
		t.Fatalf("LoadTenantMigrationFiles() error = %v", err)
	}
	store := &fakeTenantMigrationStore{
		applied: map[string]string{
			files[0].Filename: files[0].Checksum,
		},
	}
	migrator := FileTenantMigrator{
		MigrationsDir: dir,
		OpenStore: func(context.Context, string) (TenantMigrationStore, error) {
			return store, nil
		},
	}

	result, err := migrator.Migrate(context.Background(), Target{}, "postgres://tenant")
	if err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	if !store.ensured {
		t.Fatalf("migration store was not initialized")
	}
	if !store.closed {
		t.Fatalf("migration store was not closed")
	}
	if got := result.Version; got != "002_finance_extension" {
		t.Fatalf("Version = %q, want 002_finance_extension", got)
	}
	if len(store.appliedFiles) != 1 || store.appliedFiles[0].Filename != files[1].Filename {
		t.Fatalf("applied files = %#v, want only second file", store.appliedFiles)
	}
	if !reflect.DeepEqual(result.AppliedStages, []MigrationStage{files[1].Stage}) {
		t.Fatalf("AppliedStages = %#v, want second stage", result.AppliedStages)
	}
	if got := stringSliceMetadata(result.Metadata, "migration_files_skipped"); !reflect.DeepEqual(got, []string{files[0].Filename}) {
		t.Fatalf("migration_files_skipped = %#v", got)
	}
	if got := stringSliceMetadata(result.Metadata, "migration_files_applied"); !reflect.DeepEqual(got, []string{files[1].Filename}) {
		t.Fatalf("migration_files_applied = %#v", got)
	}
	if result.Metadata["migration_mode"] != "tenant_business" {
		t.Fatalf("migration_mode = %#v", result.Metadata["migration_mode"])
	}
}

func TestFileTenantMigratorRejectsChecksumDriftForAlreadyAppliedMigration(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "001_tenant_business_baseline.sql", "CREATE TABLE first(id UUID PRIMARY KEY);\n")
	files, err := LoadTenantMigrationFiles(dir)
	if err != nil {
		t.Fatalf("LoadTenantMigrationFiles() error = %v", err)
	}
	store := &fakeTenantMigrationStore{
		applied: map[string]string{
			files[0].Filename: "different-checksum",
		},
	}
	migrator := FileTenantMigrator{
		MigrationsDir: dir,
		OpenStore: func(context.Context, string) (TenantMigrationStore, error) {
			return store, nil
		},
	}

	_, err = migrator.Migrate(context.Background(), Target{}, "postgres://tenant")

	if err == nil || !strings.Contains(err.Error(), "checksum drift") {
		t.Fatalf("Migrate() error = %v, want checksum drift", err)
	}
	if len(store.appliedFiles) != 0 {
		t.Fatalf("applied files after checksum drift = %#v, want none", store.appliedFiles)
	}
}

func TestFileTenantMigratorReconcilesDeclaredBaselineDriftWithPendingRepair(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "001_tenant_business_baseline.sql", "CREATE TABLE first(id UUID PRIMARY KEY, name TEXT);\n")
	writeTestFile(t, dir, "002_projection_repair.sql", `-- tenantdb:accept-checksum-drift 001_tenant_business_baseline.sql
CREATE TABLE projection_outbox(id UUID PRIMARY KEY);
`)
	files, err := LoadTenantMigrationFiles(dir)
	if err != nil {
		t.Fatalf("LoadTenantMigrationFiles() error = %v", err)
	}
	if !reflect.DeepEqual(files[1].AcceptsChecksumDrift, []string{"001_tenant_business_baseline.sql"}) {
		t.Fatalf("accepted drift = %#v", files[1].AcceptsChecksumDrift)
	}
	store := &fakeTenantMigrationStore{applied: map[string]string{
		files[0].Filename: "previous-baseline-checksum",
	}}
	migrator := FileTenantMigrator{
		MigrationsDir: dir,
		OpenStore: func(context.Context, string) (TenantMigrationStore, error) {
			return store, nil
		},
	}

	result, err := migrator.Migrate(context.Background(), Target{}, "postgres://tenant")
	if err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	if len(store.appliedFiles) != 1 || store.appliedFiles[0].Filename != files[1].Filename {
		t.Fatalf("applied files = %#v, want repair only", store.appliedFiles)
	}
	if len(store.reconciliations) != 1 {
		t.Fatalf("reconciliations = %#v, want one", store.reconciliations)
	}
	reconciliation := store.reconciliations[0]
	if reconciliation.Filename != files[0].Filename || reconciliation.PreviousChecksum != "previous-baseline-checksum" || reconciliation.AcceptedChecksum != files[0].Checksum || reconciliation.RepairFilename != files[1].Filename {
		t.Fatalf("reconciliation = %#v", reconciliation)
	}
	if store.applied[files[0].Filename] != files[0].Checksum {
		t.Fatalf("reconciled checksum = %q, want %q", store.applied[files[0].Filename], files[0].Checksum)
	}
	if got := stringSliceMetadata(result.Metadata, "migration_checksums_reconciled"); !reflect.DeepEqual(got, []string{files[0].Filename}) {
		t.Fatalf("migration_checksums_reconciled = %#v", got)
	}
}

func TestFileTenantMigratorRejectsDriftWhenAcceptingRepairIsAlreadyApplied(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "001_tenant_business_baseline.sql", "CREATE TABLE first(id UUID PRIMARY KEY, name TEXT);\n")
	writeTestFile(t, dir, "002_projection_repair.sql", `-- tenantdb:accept-checksum-drift 001_tenant_business_baseline.sql
CREATE TABLE projection_outbox(id UUID PRIMARY KEY);
`)
	files, err := LoadTenantMigrationFiles(dir)
	if err != nil {
		t.Fatalf("LoadTenantMigrationFiles() error = %v", err)
	}
	store := &fakeTenantMigrationStore{applied: map[string]string{
		files[0].Filename: "unexpected-new-drift",
		files[1].Filename: files[1].Checksum,
	}}
	migrator := FileTenantMigrator{
		MigrationsDir: dir,
		OpenStore: func(context.Context, string) (TenantMigrationStore, error) {
			return store, nil
		},
	}

	_, err = migrator.Migrate(context.Background(), Target{}, "postgres://tenant")
	if err == nil || !strings.Contains(err.Error(), "checksum drift") {
		t.Fatalf("Migrate() error = %v, want checksum drift after repair already applied", err)
	}
	if len(store.reconciliations) != 0 || len(store.appliedFiles) != 0 {
		t.Fatalf("store changed after rejected drift: files=%#v reconciliations=%#v", store.appliedFiles, store.reconciliations)
	}
}

func TestTenantMigrationTrackingUsesTenantMigrationRuns(t *testing.T) {
	if tenantMigrationTrackingTable != "tenant_migration_runs" {
		t.Fatalf("tenant migration tracking table = %q, want tenant_migration_runs", tenantMigrationTrackingTable)
	}
	sql := tenantMigrationTrackingSQL()
	if !strings.Contains(sql, "CREATE TABLE IF NOT EXISTS tenant_migration_runs") {
		t.Fatalf("tracking SQL = %q, want tenant_migration_runs table", sql)
	}
	if strings.Contains(sql, "CREATE TABLE IF NOT EXISTS tenant_schema_migrations") {
		t.Fatalf("tracking SQL = %q, must not create old tenant_schema_migrations table", sql)
	}
	if !strings.Contains(sql, "CREATE TABLE IF NOT EXISTS tenant_migration_checksum_history") {
		t.Fatalf("tracking SQL = %q, want checksum reconciliation history", sql)
	}
	if strings.Count(sql, ");") < 2 {
		t.Fatalf("tracking SQL = %q, want both CREATE TABLE statements terminated", sql)
	}
	legacySQL := migrateLegacyTenantMigrationRunsSQL()
	if !strings.Contains(legacySQL, "to_regclass('tenant_schema_migrations')") {
		t.Fatalf("legacy tracking SQL = %q, want legacy tenant_schema_migrations copy guard", legacySQL)
	}
	if !strings.Contains(legacySQL, "INSERT INTO tenant_migration_runs") {
		t.Fatalf("legacy tracking SQL = %q, want copy into tenant_migration_runs", legacySQL)
	}
}

func writeTestFile(t *testing.T, dir string, name string, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func stringSliceMetadata(metadata map[string]any, key string) []string {
	value, _ := metadata[key].([]string)
	return value
}

func repoTenantMigrationsDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "migrations", "tenant")
}

type fakeTenantMigrationStore struct {
	ensured         bool
	closed          bool
	applied         map[string]string
	appliedFiles    []TenantMigrationFile
	reconciliations []ChecksumReconciliation
}

func (f *fakeTenantMigrationStore) EnsureMigrationTable(context.Context) error {
	f.ensured = true
	return nil
}

func (f *fakeTenantMigrationStore) AppliedMigrations(context.Context) (map[string]string, error) {
	out := map[string]string{}
	for key, value := range f.applied {
		out[key] = value
	}
	return out, nil
}

func (f *fakeTenantMigrationStore) ApplyMigration(_ context.Context, file TenantMigrationFile, reconciliations []ChecksumReconciliation) error {
	if f.applied == nil {
		f.applied = make(map[string]string)
	}
	f.appliedFiles = append(f.appliedFiles, file)
	f.applied[file.Filename] = file.Checksum
	for _, reconciliation := range reconciliations {
		f.reconciliations = append(f.reconciliations, reconciliation)
		if f.applied[reconciliation.Filename] == reconciliation.PreviousChecksum {
			f.applied[reconciliation.Filename] = reconciliation.AcceptedChecksum
		}
	}
	return nil
}

func (f *fakeTenantMigrationStore) Close() {
	f.closed = true
}
