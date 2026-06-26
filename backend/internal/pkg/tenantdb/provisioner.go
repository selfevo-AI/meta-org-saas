package tenantdb

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	MigrationScopeTenantBusiness  = "tenant_business"
	defaultTenantMigrationVersion = "tenant-business-baseline-pending"
)

type MigrationStage struct {
	Name  string
	Scope string
}

type MigrationResult struct {
	Version       string
	AppliedStages []MigrationStage
	Metadata      map[string]any
}

type ProvisionResult struct {
	Target    Target
	Database  DatabaseCreation
	Migration MigrationResult
}

type TenantMigrator interface {
	Migrate(context.Context, Target, string) (MigrationResult, error)
}

type DatabaseCreation struct {
	SQL     string
	Created bool
	Existed bool
}

type DatabaseCatalog interface {
	DatabaseExists(context.Context, string) (bool, error)
	Exec(context.Context, string) error
}

type DatabaseCreator interface {
	CreateDatabase(context.Context, string) (DatabaseCreation, error)
}

type ProvisionerConfig struct {
	AdminURL string
	Creator  DatabaseCreator
	Migrator TenantMigrator
	Now      func() time.Time
}

type Provisioner struct {
	resolver DataSourceResolver
	creator  DatabaseCreator
	migrator TenantMigrator
	now      func() time.Time
}

func NewProvisioner(config ProvisionerConfig) Provisioner {
	migrator := config.Migrator
	if migrator == nil {
		migrator = NoopTenantMigrator{}
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	return Provisioner{
		resolver: NewDataSourceResolver(config.AdminURL),
		creator:  config.Creator,
		migrator: migrator,
		now:      now,
	}
}

func (p Provisioner) Provision(ctx context.Context, target Target) (ProvisionResult, error) {
	result := ProvisionResult{Target: target}
	metadata := cloneMetadata(target.Metadata)
	metadata["provisioner"] = "tenant_database_provisioner"
	metadata["provisioning_mode"] = target.DeploymentMode

	switch target.DeploymentMode {
	case DeploymentModeDedicatedDatabase:
		if _, err := CreateDatabaseSQL(target); err != nil {
			return p.failed(result, metadata, err), err
		}
		if p.creator == nil {
			err := fmt.Errorf("tenant database creator is required for %s", DeploymentModeDedicatedDatabase)
			return p.failed(result, metadata, err), err
		}
		creation, err := p.creator.CreateDatabase(ctx, target.DatabaseName)
		result.Database = creation
		metadata["database_created"] = creation.Created
		metadata["database_existed"] = creation.Existed
		if err != nil {
			return p.failed(result, metadata, fmt.Errorf("create tenant database: %w", err)), err
		}
	case DeploymentModeSharedSchema:
		metadata["database_created"] = false
		metadata["database_existed"] = true
	default:
		err := fmt.Errorf("unsupported tenant database deployment mode %q", target.DeploymentMode)
		return p.failed(result, metadata, err), err
	}

	targetURL, err := p.resolver.URLForTarget(target)
	if err != nil {
		return p.failed(result, metadata, err), err
	}
	migration, err := p.migrator.Migrate(ctx, target, targetURL)
	result.Migration = migration
	if err != nil {
		wrapped := fmt.Errorf("migrate tenant database: %w", err)
		return p.failed(result, metadata, wrapped), wrapped
	}
	mergeMetadata(metadata, migration.Metadata)
	metadata["migration_stage_count"] = len(migration.AppliedStages)
	metadata["provisioned_at"] = p.now().UTC().Format(time.RFC3339)

	result.Target = target
	result.Target.Status = TargetStatusProvisioned
	if migration.Version != "" {
		result.Target.MigrationVersion = migration.Version
	}
	result.Target.Metadata = metadata
	return result, nil
}

func (p Provisioner) failed(result ProvisionResult, metadata map[string]any, err error) ProvisionResult {
	metadata["error"] = err.Error()
	metadata["failed_at"] = p.now().UTC().Format(time.RFC3339)
	result.Target.Status = TargetStatusFailed
	result.Target.Metadata = metadata
	return result
}

type NoopTenantMigrator struct {
	Version string
}

func (m NoopTenantMigrator) Migrate(context.Context, Target, string) (MigrationResult, error) {
	version := m.Version
	if version == "" {
		version = defaultTenantMigrationVersion
	}
	return MigrationResult{
		Version: version,
		Metadata: map[string]any{
			"migration_mode":          "deferred_until_tenant_baseline_split",
			"root_migrations_applied": false,
		},
	}, nil
}

func CreateDatabaseSQL(target Target) (string, error) {
	switch target.DeploymentMode {
	case DeploymentModeSharedSchema:
		return "", nil
	case DeploymentModeDedicatedDatabase:
		quoted, err := QuoteIdentifier(target.DatabaseName)
		if err != nil {
			return "", fmt.Errorf("invalid tenant database name: %w", err)
		}
		return fmt.Sprintf("CREATE DATABASE %s", quoted), nil
	default:
		return "", fmt.Errorf("unsupported tenant database deployment mode %q", target.DeploymentMode)
	}
}

func CreateDatabaseWithCatalog(ctx context.Context, catalog DatabaseCatalog, databaseName string) (DatabaseCreation, error) {
	target := Target{DeploymentMode: DeploymentModeDedicatedDatabase, DatabaseName: databaseName}
	sql, err := CreateDatabaseSQL(target)
	if err != nil {
		return DatabaseCreation{}, err
	}
	exists, err := catalog.DatabaseExists(ctx, databaseName)
	if err != nil {
		return DatabaseCreation{SQL: sql}, fmt.Errorf("check tenant database existence: %w", err)
	}
	if exists {
		return DatabaseCreation{SQL: sql, Existed: true}, nil
	}
	if err := catalog.Exec(ctx, sql); err != nil {
		return DatabaseCreation{SQL: sql}, err
	}
	return DatabaseCreation{SQL: sql, Created: true}, nil
}

type CatalogDatabaseCreator struct {
	catalog DatabaseCatalog
}

func NewCatalogDatabaseCreator(catalog DatabaseCatalog) CatalogDatabaseCreator {
	return CatalogDatabaseCreator{catalog: catalog}
}

func (c CatalogDatabaseCreator) CreateDatabase(ctx context.Context, databaseName string) (DatabaseCreation, error) {
	if c.catalog == nil {
		return DatabaseCreation{}, fmt.Errorf("tenant database catalog is required")
	}
	return CreateDatabaseWithCatalog(ctx, c.catalog, databaseName)
}

type PGDatabaseCatalog struct {
	db *pgxpool.Pool
}

func NewPGDatabaseCatalog(db *pgxpool.Pool) PGDatabaseCatalog {
	return PGDatabaseCatalog{db: db}
}

func (c PGDatabaseCatalog) DatabaseExists(ctx context.Context, databaseName string) (bool, error) {
	var exists bool
	if err := c.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)`, databaseName).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (c PGDatabaseCatalog) Exec(ctx context.Context, sql string) error {
	_, err := c.db.Exec(ctx, sql)
	return err
}

func cloneMetadata(metadata map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range metadata {
		out[key] = value
	}
	return out
}

func mergeMetadata(dst map[string]any, src map[string]any) {
	for key, value := range src {
		dst[key] = value
	}
}
