package tenantdb

import (
	"testing"

	"github.com/google/uuid"
)

func TestSchemaNameForOrganizationUsesStableSafeIdentifier(t *testing.T) {
	id := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")

	got := SchemaNameForOrganization(id)
	want := "org_123e4567e89b12d3a456426614174000"

	if got != want {
		t.Fatalf("SchemaNameForOrganization() = %q, want %q", got, want)
	}
}

func TestQuoteIdentifierRejectsUnsafeNames(t *testing.T) {
	unsafe := []string{
		"",
		"123table",
		"table-name",
		"table;drop",
		"table name",
		"table.name",
	}

	for _, name := range unsafe {
		if _, err := QuoteIdentifier(name); err == nil {
			t.Fatalf("QuoteIdentifier(%q) succeeded, want error", name)
		}
	}
}

func TestSearchPathSQLIncludesOrganizationThenPlatformAndPublic(t *testing.T) {
	got, err := SearchPathSQL("org_123e4567e89b12d3a456426614174000")
	if err != nil {
		t.Fatalf("SearchPathSQL() error = %v", err)
	}
	want := `SET LOCAL search_path = "org_123e4567e89b12d3a456426614174000", "platform", "public"`
	if got != want {
		t.Fatalf("SearchPathSQL() = %q, want %q", got, want)
	}
}

func TestDatabaseNameForOrganizationUsesStablePhysicalTenantName(t *testing.T) {
	id := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")

	got := DatabaseNameForOrganization("meta_org_", id)
	want := "meta_org_123e"

	if got != want {
		t.Fatalf("DatabaseNameForOrganization() = %q, want %q", got, want)
	}
}

func TestDatabaseNameForOrganizationNormalizesUnsafePrefix(t *testing.T) {
	id := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")

	got := DatabaseNameForOrganization("Tenant DB-", id)
	want := "tenant_db_123e"

	if got != want {
		t.Fatalf("DatabaseNameForOrganization() = %q, want %q", got, want)
	}
}

func TestDatabaseURLForNameReplacesDatabasePath(t *testing.T) {
	got, err := DatabaseURLForName("postgres://user:pass@localhost:5432/postgres?sslmode=disable", "meta_org_123e")
	if err != nil {
		t.Fatalf("DatabaseURLForName() error = %v", err)
	}
	want := "postgres://user:pass@localhost:5432/meta_org_123e?sslmode=disable"
	if got != want {
		t.Fatalf("DatabaseURLForName() = %q, want %q", got, want)
	}
}

func TestNewDedicatedDatabaseTargetUsesPublicTenantSchemaAndProvisioningStatus(t *testing.T) {
	id := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")

	target := NewDedicatedDatabaseTarget(id, "meta_org_", "local-primary", "local")

	if target.OrganizationID != id {
		t.Fatalf("OrganizationID = %s, want %s", target.OrganizationID, id)
	}
	if target.DeploymentMode != DeploymentModeDedicatedDatabase {
		t.Fatalf("DeploymentMode = %q, want %q", target.DeploymentMode, DeploymentModeDedicatedDatabase)
	}
	if target.DatabaseName != "meta_org_123e" {
		t.Fatalf("DatabaseName = %q", target.DatabaseName)
	}
	if target.SchemaName != "public" {
		t.Fatalf("SchemaName = %q, want public", target.SchemaName)
	}
	if target.Status != TargetStatusProvisioning {
		t.Fatalf("Status = %q, want %q", target.Status, TargetStatusProvisioning)
	}
}

func TestDefaultsBuildDedicatedDatabaseTarget(t *testing.T) {
	id := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")

	target := Defaults{
		DeploymentMode:     DeploymentModeDedicatedDatabase,
		DatabaseNamePrefix: "meta_org_",
		ClusterKey:         "cluster-a",
		Region:             "cn-east-1",
	}.TargetForOrganization(id)

	if target.DeploymentMode != DeploymentModeDedicatedDatabase {
		t.Fatalf("DeploymentMode = %q", target.DeploymentMode)
	}
	if target.DatabaseName != "meta_org_123e" {
		t.Fatalf("DatabaseName = %q", target.DatabaseName)
	}
	if target.SchemaName != "public" {
		t.Fatalf("SchemaName = %q, want public", target.SchemaName)
	}
	if target.ClusterKey != "cluster-a" || target.Region != "cn-east-1" {
		t.Fatalf("cluster/region = %q/%q", target.ClusterKey, target.Region)
	}
}

func TestDefaultsBuildSharedSchemaCompatibilityTarget(t *testing.T) {
	id := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")

	target := Defaults{DeploymentMode: DeploymentModeSharedSchema}.TargetForOrganization(id)

	if target.DeploymentMode != DeploymentModeSharedSchema {
		t.Fatalf("DeploymentMode = %q", target.DeploymentMode)
	}
	if target.DatabaseName != "" {
		t.Fatalf("DatabaseName = %q, want empty for shared schema", target.DatabaseName)
	}
	if target.SchemaName != "org_123e4567e89b12d3a456426614174000" {
		t.Fatalf("SchemaName = %q", target.SchemaName)
	}
	if target.Status != TargetStatusProvisioned {
		t.Fatalf("Status = %q, want provisioned shared-schema compatibility target", target.Status)
	}
}

func TestDataSourceResolverBuildsDedicatedTenantDatabaseURL(t *testing.T) {
	resolver := NewDataSourceResolver("postgres://user:pass@localhost:5432/postgres?sslmode=disable")
	target := Target{
		DeploymentMode: DeploymentModeDedicatedDatabase,
		DatabaseName:   "meta_org_123e",
	}

	got, err := resolver.URLForTarget(target)
	if err != nil {
		t.Fatalf("URLForTarget() error = %v", err)
	}
	want := "postgres://user:pass@localhost:5432/meta_org_123e?sslmode=disable"
	if got != want {
		t.Fatalf("URLForTarget() = %q, want %q", got, want)
	}
}

func TestDataSourceResolverUsesAdminURLForSharedSchemaCompatibilityTarget(t *testing.T) {
	adminURL := "postgres://user:pass@localhost:5432/meta_org_saas?sslmode=disable"
	resolver := NewDataSourceResolver(adminURL)
	target := Target{DeploymentMode: DeploymentModeSharedSchema, SchemaName: "org_123"}

	got, err := resolver.URLForTarget(target)
	if err != nil {
		t.Fatalf("URLForTarget() error = %v", err)
	}
	if got != adminURL {
		t.Fatalf("URLForTarget() = %q, want shared admin URL %q", got, adminURL)
	}
}
