package tenantdb

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestCreateDatabaseSQLQuotesDedicatedDatabaseName(t *testing.T) {
	target := Target{
		DeploymentMode: DeploymentModeDedicatedDatabase,
		DatabaseName:   "meta_org_123e",
	}

	got, err := CreateDatabaseSQL(target)
	if err != nil {
		t.Fatalf("CreateDatabaseSQL() error = %v", err)
	}
	want := `CREATE DATABASE "meta_org_123e"`
	if got != want {
		t.Fatalf("CreateDatabaseSQL() = %q, want %q", got, want)
	}
}

func TestCreateDatabaseSQLNoopsForSharedSchemaTarget(t *testing.T) {
	target := Target{
		DeploymentMode: DeploymentModeSharedSchema,
		SchemaName:     "org_123",
	}

	got, err := CreateDatabaseSQL(target)
	if err != nil {
		t.Fatalf("CreateDatabaseSQL() error = %v", err)
	}
	if got != "" {
		t.Fatalf("CreateDatabaseSQL() = %q, want empty SQL for shared schema", got)
	}
}

func TestCreateDatabaseSQLRejectsUnsafeDatabaseName(t *testing.T) {
	target := Target{
		DeploymentMode: DeploymentModeDedicatedDatabase,
		DatabaseName:   `tenant_db"; DROP DATABASE postgres; --`,
	}

	_, err := CreateDatabaseSQL(target)

	if err == nil {
		t.Fatalf("CreateDatabaseSQL() succeeded, want unsafe database name error")
	}
}

func TestCreateDatabaseWithCatalogSkipsExistingDatabase(t *testing.T) {
	catalog := &fakeDatabaseCatalog{exists: true}

	creation, err := CreateDatabaseWithCatalog(context.Background(), catalog, "meta_org_123e")
	if err != nil {
		t.Fatalf("CreateDatabaseWithCatalog() error = %v", err)
	}

	if creation.Created {
		t.Fatalf("Created = true, want false for existing database")
	}
	if len(catalog.execSQL) != 0 {
		t.Fatalf("Exec called for existing database: %#v", catalog.execSQL)
	}
}

func TestCreateDatabaseWithCatalogExecutesCreateDatabaseStatement(t *testing.T) {
	catalog := &fakeDatabaseCatalog{}

	creation, err := CreateDatabaseWithCatalog(context.Background(), catalog, "meta_org_123e")
	if err != nil {
		t.Fatalf("CreateDatabaseWithCatalog() error = %v", err)
	}

	if !creation.Created {
		t.Fatalf("Created = false, want true")
	}
	if creation.SQL != `CREATE DATABASE "meta_org_123e"` {
		t.Fatalf("SQL = %q", creation.SQL)
	}
	if len(catalog.execSQL) != 1 || catalog.execSQL[0] != creation.SQL {
		t.Fatalf("execSQL = %#v, want one create statement", catalog.execSQL)
	}
}

func TestProvisionerCreatesDedicatedDatabaseAndReportsProvisionedStatus(t *testing.T) {
	orgID := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")
	target := NewDedicatedDatabaseTarget(orgID, "meta_org_", "local-primary", "local")
	creator := &fakeDatabaseCreator{}
	migrator := &fakeTenantMigrator{
		result: MigrationResult{
			Version: "tenant-business-v1",
			AppliedStages: []MigrationStage{
				{Name: "tenant_business_baseline", Scope: MigrationScopeTenantBusiness},
			},
			Metadata: map[string]any{"migration_mode": "tenant_business"},
		},
	}
	provisioner := NewProvisioner(ProvisionerConfig{
		AdminURL: "postgres://user:pass@localhost:5432/postgres?sslmode=disable",
		Creator:  creator,
		Migrator: migrator,
	})

	result, err := provisioner.Provision(context.Background(), target)
	if err != nil {
		t.Fatalf("Provision() error = %v", err)
	}

	if len(creator.names) != 1 || creator.names[0] != target.DatabaseName {
		t.Fatalf("created databases = %#v, want %q", creator.names, target.DatabaseName)
	}
	if migrator.url != "postgres://user:pass@localhost:5432/meta_org_123e?sslmode=disable" {
		t.Fatalf("migrator URL = %q", migrator.url)
	}
	if result.Target.Status != TargetStatusProvisioned {
		t.Fatalf("Status = %q, want %q", result.Target.Status, TargetStatusProvisioned)
	}
	if result.Target.MigrationVersion != "tenant-business-v1" {
		t.Fatalf("MigrationVersion = %q", result.Target.MigrationVersion)
	}
	if result.Target.Metadata["provisioning_mode"] != DeploymentModeDedicatedDatabase {
		t.Fatalf("provisioning_mode metadata = %#v", result.Target.Metadata["provisioning_mode"])
	}
	if result.Target.Metadata["database_created"] != true {
		t.Fatalf("database_created metadata = %#v", result.Target.Metadata["database_created"])
	}
}

func TestProvisionerDoesNotCreateDatabaseForSharedSchemaTarget(t *testing.T) {
	orgID := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")
	target := Defaults{DeploymentMode: DeploymentModeSharedSchema}.TargetForOrganization(orgID)
	creator := &fakeDatabaseCreator{}
	migrator := &fakeTenantMigrator{
		result: MigrationResult{Version: "shared-schema-compatibility"},
	}
	provisioner := NewProvisioner(ProvisionerConfig{
		AdminURL: "postgres://user:pass@localhost:5432/meta_org_saas?sslmode=disable",
		Creator:  creator,
		Migrator: migrator,
	})

	result, err := provisioner.Provision(context.Background(), target)
	if err != nil {
		t.Fatalf("Provision() error = %v", err)
	}

	if len(creator.names) != 0 {
		t.Fatalf("created databases = %#v, want none", creator.names)
	}
	if migrator.url != "postgres://user:pass@localhost:5432/meta_org_saas?sslmode=disable" {
		t.Fatalf("migrator URL = %q", migrator.url)
	}
	if result.Target.Status != TargetStatusProvisioned {
		t.Fatalf("Status = %q, want %q", result.Target.Status, TargetStatusProvisioned)
	}
}

func TestProvisionerReturnsFailedResultWhenMigrationFails(t *testing.T) {
	orgID := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")
	target := NewDedicatedDatabaseTarget(orgID, "meta_org_", "local-primary", "local")
	migrationErr := errors.New("migration failed")
	provisioner := NewProvisioner(ProvisionerConfig{
		AdminURL: "postgres://user:pass@localhost:5432/postgres?sslmode=disable",
		Creator:  &fakeDatabaseCreator{},
		Migrator: &fakeTenantMigrator{err: migrationErr},
	})

	result, err := provisioner.Provision(context.Background(), target)

	if !errors.Is(err, migrationErr) {
		t.Fatalf("Provision() error = %v, want migration error", err)
	}
	if result.Target.Status != TargetStatusFailed {
		t.Fatalf("Status = %q, want %q", result.Target.Status, TargetStatusFailed)
	}
	errorText, _ := result.Target.Metadata["error"].(string)
	if !strings.Contains(errorText, "migration failed") {
		t.Fatalf("error metadata = %#v, want migration failure", result.Target.Metadata["error"])
	}
}

type fakeDatabaseCatalog struct {
	exists  bool
	execSQL []string
}

func (f *fakeDatabaseCatalog) DatabaseExists(context.Context, string) (bool, error) {
	return f.exists, nil
}

func (f *fakeDatabaseCatalog) Exec(ctx context.Context, sql string) error {
	f.execSQL = append(f.execSQL, sql)
	return nil
}

type fakeDatabaseCreator struct {
	names []string
}

func (f *fakeDatabaseCreator) CreateDatabase(ctx context.Context, databaseName string) (DatabaseCreation, error) {
	f.names = append(f.names, databaseName)
	return DatabaseCreation{SQL: `CREATE DATABASE "` + databaseName + `"`, Created: true}, nil
}

type fakeTenantMigrator struct {
	url    string
	result MigrationResult
	err    error
}

func (f *fakeTenantMigrator) Migrate(ctx context.Context, target Target, tenantDatabaseURL string) (MigrationResult, error) {
	f.url = tenantDatabaseURL
	return f.result, f.err
}
