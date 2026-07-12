package saas

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/tenantdb"
)

func TestAllocateOrganizationIDRetriesDedicatedDatabaseNameCollision(t *testing.T) {
	first := uuid.MustParse("aaaa0000-0000-0000-0000-000000000001")
	second := uuid.MustParse("bbbb0000-0000-0000-0000-000000000002")
	ids := []uuid.UUID{first, second}
	repository := NewRepository(nil,
		WithTenantDatabaseDefaults(tenantdb.Defaults{DeploymentMode: tenantdb.DeploymentModeDedicatedDatabase}),
		withOrganizationIDGenerator(func() uuid.UUID {
			id := ids[0]
			ids = ids[1:]
			return id
		}),
	)

	result, err := allocateOrganizationID(repository, func(id uuid.UUID) (uuid.UUID, error) {
		if id == first {
			return uuid.Nil, tenantDatabaseNameConflictError()
		}
		return id, nil
	})
	if err != nil {
		t.Fatalf("allocateOrganizationID() error = %v", err)
	}
	if result != second {
		t.Fatalf("allocateOrganizationID() = %s, want %s", result, second)
	}
}

func TestAllocateOrganizationIDDoesNotRetrySharedSchemaConflict(t *testing.T) {
	calls := 0
	repository := NewRepository(nil,
		WithTenantDatabaseDefaults(tenantdb.Defaults{DeploymentMode: tenantdb.DeploymentModeSharedSchema}),
		withOrganizationIDGenerator(func() uuid.UUID {
			calls++
			return uuid.New()
		}),
	)

	_, err := allocateOrganizationID(repository, func(uuid.UUID) (uuid.UUID, error) {
		return uuid.Nil, tenantDatabaseNameConflictError()
	})
	if err == nil {
		t.Fatal("allocateOrganizationID() error = nil, want physical-name conflict")
	}
	if calls != 1 {
		t.Fatalf("organization ID generator calls = %d, want 1", calls)
	}
}

func TestAllocateOrganizationIDReturnsConflictAfterRetryExhaustion(t *testing.T) {
	calls := 0
	repository := NewRepository(nil,
		WithTenantDatabaseDefaults(tenantdb.Defaults{DeploymentMode: tenantdb.DeploymentModeDedicatedDatabase}),
		withOrganizationIDGenerator(func() uuid.UUID {
			calls++
			return uuid.New()
		}),
	)

	_, err := allocateOrganizationID(repository, func(uuid.UUID) (uuid.UUID, error) {
		return uuid.Nil, tenantDatabaseNameConflictError()
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("allocateOrganizationID() error = %v, want ErrConflict", err)
	}
	if calls != tenantDatabaseNameAllocationAttempts {
		t.Fatalf("organization ID generator calls = %d, want %d", calls, tenantDatabaseNameAllocationAttempts)
	}
}

func TestIsTenantDatabaseNameConflictRejectsOtherUniqueConstraints(t *testing.T) {
	err := &pgconn.PgError{Code: "23505", ConstraintName: "users_email_key"}
	if isTenantDatabaseNameConflict(err) {
		t.Fatal("isTenantDatabaseNameConflict() = true for unrelated unique constraint")
	}
}

func tenantDatabaseNameConflictError() error {
	return &pgconn.PgError{
		Code:           "23505",
		ConstraintName: "uq_tenant_database_targets_physical_name",
	}
}
