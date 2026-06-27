package tenantdb

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
)

func TestTenantDatabaseURLFromContextUsesProvisionedDedicatedTenant(t *testing.T) {
	orgID := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")
	ctx := context.WithValue(context.Background(), middleware.TenantContextKey, &middleware.TenantContext{
		OrganizationID:               &orgID,
		TenantDatabaseDeploymentMode: DeploymentModeDedicatedDatabase,
		TenantDatabaseStatus:         TargetStatusProvisioned,
		TenantDatabaseName:           "meta_org_123e",
		TenantDatabaseClusterKey:     "local-primary",
		TenantDatabaseRegion:         "local",
	})

	url, ok, err := TenantDatabaseURLFromContext(ctx, "postgres://user:pass@localhost:5432/postgres?sslmode=disable")
	if err != nil {
		t.Fatalf("TenantDatabaseURLFromContext() error = %v", err)
	}
	if !ok {
		t.Fatalf("TenantDatabaseURLFromContext() ok = false, want true")
	}
	want := "postgres://user:pass@localhost:5432/meta_org_123e?sslmode=disable"
	if url != want {
		t.Fatalf("tenant URL = %q, want %q", url, want)
	}
}

func TestTenantDatabaseURLFromContextSkipsSharedSchemaTenant(t *testing.T) {
	orgID := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")
	ctx := context.WithValue(context.Background(), middleware.TenantContextKey, &middleware.TenantContext{
		OrganizationID:               &orgID,
		TenantDatabaseDeploymentMode: DeploymentModeSharedSchema,
		TenantDatabaseStatus:         TargetStatusProvisioned,
		TenantDatabaseName:           "",
		TenantSchemaName:             "org_123",
	})

	_, ok, err := TenantDatabaseURLFromContext(ctx, "postgres://user:pass@localhost:5432/postgres?sslmode=disable")
	if err != nil {
		t.Fatalf("TenantDatabaseURLFromContext() error = %v", err)
	}
	if ok {
		t.Fatalf("TenantDatabaseURLFromContext() ok = true, want false for shared schema")
	}
}
