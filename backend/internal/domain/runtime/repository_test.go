package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org/backend/internal/pkg/middleware"
)

func TestSchemaNameFromContextUsesTenantOrganization(t *testing.T) {
	orgID := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")
	ctx := context.WithValue(context.Background(), middleware.TenantContextKey, &middleware.TenantContext{
		OrganizationID: &orgID,
	})

	got, err := schemaNameFromContext(ctx)

	if err != nil {
		t.Fatalf("schemaNameFromContext() error = %v", err)
	}
	want := "org_123e4567e89b12d3a456426614174000"
	if got != want {
		t.Fatalf("schemaNameFromContext() = %q, want %q", got, want)
	}
}

func TestSchemaNameFromContextRequiresTenantOrganization(t *testing.T) {
	_, err := schemaNameFromContext(context.Background())

	if err == nil {
		t.Fatal("schemaNameFromContext() succeeded, want error")
	}
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("schemaNameFromContext() error = %v, want ErrValidation", err)
	}
}
