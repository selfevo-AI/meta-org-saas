package systemadmin

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestDefaultOrganizationSchemaPackageIsValid(t *testing.T) {
	pkg := DefaultOrganizationSchemaPackage()

	if err := ValidateSchemaPackage(pkg); err != nil {
		t.Fatalf("DefaultOrganizationSchemaPackage() invalid: %v", err)
	}
}

func TestApplySchemaChangeRejectsPendingRequest(t *testing.T) {
	repo := &fakeRepository{
		role: "system_owner",
		request: &SchemaChangeRequest{
			ID:             uuid.New(),
			OrganizationID: uuid.New(),
			Status:         SchemaChangePending,
			SchemaPackage:  DefaultOrganizationSchemaPackage(),
		},
	}
	service := NewService(repo)

	_, err := service.ApplySchemaChange(context.Background(), uuid.New(), repo.request.ID)
	if err == nil {
		t.Fatal("ApplySchemaChange() succeeded, want error")
	}
	if err != ErrInvalidTransition {
		t.Fatalf("ApplySchemaChange() error = %v, want ErrInvalidTransition", err)
	}
	if repo.applied {
		t.Fatal("ApplySchemaChange() applied pending request")
	}
}

func TestApplySchemaChangeRejectsAuditorRole(t *testing.T) {
	repo := &fakeRepository{
		role: "auditor",
		request: &SchemaChangeRequest{
			ID:             uuid.New(),
			OrganizationID: uuid.New(),
			Status:         SchemaChangeApproved,
			SchemaPackage:  DefaultOrganizationSchemaPackage(),
		},
	}
	service := NewService(repo)

	_, err := service.ApplySchemaChange(context.Background(), uuid.New(), repo.request.ID)

	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("ApplySchemaChange error = %v, want ErrForbidden", err)
	}
	if repo.applied {
		t.Fatal("ApplySchemaChange applied schema for auditor role")
	}
}

type fakeRepository struct {
	role    string
	request *SchemaChangeRequest
	applied bool
}

func (f *fakeRepository) GetPlatformRole(context.Context, uuid.UUID) (string, error) {
	return f.role, nil
}

func (f *fakeRepository) ListPlatformMasters(context.Context, string, int) ([]PlatformMaster, error) {
	return nil, nil
}

func (f *fakeRepository) ListPlatformDetails(context.Context, string) ([]PlatformDetail, error) {
	return nil, nil
}

func (f *fakeRepository) ListSchemaTargets(context.Context, int) ([]OrganizationSchemaTarget, error) {
	return nil, nil
}

func (f *fakeRepository) GetSchemaTarget(context.Context, uuid.UUID) (*OrganizationSchemaTarget, error) {
	return nil, nil
}

func (f *fakeRepository) CreateSchemaChangeRequest(context.Context, CreateSchemaChangeRequestRecord) (*SchemaChangeRequest, error) {
	return nil, nil
}

func (f *fakeRepository) GetSchemaChangeRequest(context.Context, uuid.UUID) (*SchemaChangeRequest, error) {
	return f.request, nil
}

func (f *fakeRepository) UpdateSchemaChangeRequestStatus(context.Context, uuid.UUID, string, uuid.UUID, string) (*SchemaChangeRequest, error) {
	return nil, nil
}

func (f *fakeRepository) ApplySchemaChange(context.Context, *SchemaChangeRequest, []string) (*SchemaApplyJob, error) {
	f.applied = true
	return &SchemaApplyJob{}, nil
}
