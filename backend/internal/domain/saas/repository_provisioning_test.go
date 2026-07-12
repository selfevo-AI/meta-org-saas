package saas

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/tenantdb"
)

func TestTenantDatabaseProvisioningIdempotencyKeyIsStableForSameTopology(t *testing.T) {
	target := tenantdb.NewDedicatedDatabaseTarget(
		uuid.MustParse("31a859fa-c572-42fa-ae78-7af5f0a8edea"),
		"meta_org_",
		"local-primary",
		"local",
	)

	first := tenantDatabaseProvisioningIdempotencyKey(target)
	second := tenantDatabaseProvisioningIdempotencyKey(target)

	if first != second {
		t.Fatalf("idempotency keys differ for same topology: %q != %q", first, second)
	}
	if !strings.HasPrefix(first, "tenant-database-provision:"+target.OrganizationID.String()+":v2:") {
		t.Fatalf("idempotency key = %q, want organization-scoped v2 prefix", first)
	}
}

func TestTenantDatabaseProvisioningIdempotencyKeyChangesWithTopology(t *testing.T) {
	target := tenantdb.NewDedicatedDatabaseTarget(uuid.New(), "meta_org_", "local-primary", "local")
	original := tenantDatabaseProvisioningIdempotencyKey(target)

	changedRegion := target
	changedRegion.Region = "cn-east-1"
	changedDatabase := target
	changedDatabase.DatabaseName = "meta_org_recovery"
	changedSchema := target
	changedSchema.SchemaName = "tenant_runtime"

	for name, changed := range map[string]tenantdb.Target{
		"region":        changedRegion,
		"database_name": changedDatabase,
		"schema_name":   changedSchema,
	} {
		t.Run(name, func(t *testing.T) {
			if got := tenantDatabaseProvisioningIdempotencyKey(changed); got == original {
				t.Fatalf("idempotency key did not change for %s topology change", name)
			}
		})
	}
}
