package saas

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/tenantdb"
)

func TestAllocateOrganizationIDUsesPhysicalNameUniqueIndex(t *testing.T) {
	databaseURL := os.Getenv("SAAS_REPOSITORY_INTEGRATION_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set SAAS_REPOSITORY_INTEGRATION_DATABASE_URL to run repository integration tests")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect integration database: %v", err)
	}
	t.Cleanup(pool.Close)

	first := uuid.New()
	collisionOwner := first
	collisionOwner[15] ^= 0xff
	second := uuid.New()
	for tenantdb.DatabaseNameForOrganization("meta_org_", second) == tenantdb.DatabaseNameForOrganization("meta_org_", first) {
		second = uuid.New()
	}

	defaults := tenantdb.Defaults{
		DeploymentMode:     tenantdb.DeploymentModeDedicatedDatabase,
		DatabaseNamePrefix: "meta_org_",
		ClusterKey:         "local-primary",
		Region:             "local",
	}
	repository := NewRepository(pool, WithTenantDatabaseDefaults(defaults))

	if _, err := pool.Exec(ctx, `INSERT INTO organizations (id, name) VALUES ($1, $2)`, collisionOwner, fmt.Sprintf("Allocation collision owner %s", collisionOwner)); err != nil {
		t.Fatalf("create collision owner: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM platform.tenant_database_targets WHERE organization_id IN ($1, $2, $3)`, collisionOwner, first, second)
		_, _ = pool.Exec(ctx, `DELETE FROM organizations WHERE id IN ($1, $2, $3)`, collisionOwner, first, second)
	})

	// Create the existing physical-name reservation in its own committed transaction.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin collision setup: %v", err)
	}
	if err := repository.upsertTenantDatabaseTarget(ctx, tx, defaults.TargetForOrganization(collisionOwner)); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("reserve collision target: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit collision target: %v", err)
	}

	ids := []uuid.UUID{first, second}
	repository.newOrganizationID = func() uuid.UUID {
		id := ids[0]
		ids = ids[1:]
		return id
	}
	createdID, err := allocateOrganizationID(repository, func(organizationID uuid.UUID) (uuid.UUID, error) {
		tx, err := pool.Begin(ctx)
		if err != nil {
			return uuid.Nil, err
		}
		defer tx.Rollback(ctx)
		if _, err := tx.Exec(ctx, `INSERT INTO organizations (id, name) VALUES ($1, $2)`, organizationID, fmt.Sprintf("Allocation candidate %s", organizationID)); err != nil {
			return uuid.Nil, err
		}
		if err := repository.upsertTenantDatabaseTarget(ctx, tx, defaults.TargetForOrganization(organizationID)); err != nil {
			return uuid.Nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return uuid.Nil, err
		}
		return organizationID, nil
	})
	if err != nil {
		t.Fatalf("allocate organization ID: %v", err)
	}
	if createdID != second {
		t.Fatalf("created organization ID = %s, want %s", createdID, second)
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM organizations WHERE id IN ($1, $2)`, first, second).Scan(&count); err != nil {
		t.Fatalf("count allocation candidates: %v", err)
	}
	if count != 1 {
		t.Fatalf("committed allocation candidates = %d, want 1", count)
	}
}
