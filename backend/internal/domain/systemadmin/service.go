package systemadmin

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org/backend/internal/pkg/tenantdb"
)

var (
	ErrForbidden         = errors.New("forbidden")
	ErrValidation        = errors.New("validation error")
	ErrInvalidTransition = errors.New("invalid schema change status transition")
)

type Service struct {
	repo repository
}

type repository interface {
	GetPlatformRole(context.Context, uuid.UUID) (string, error)
	ListPlatformMasters(context.Context, string, int) ([]PlatformMaster, error)
	ListPlatformDetails(context.Context, string) ([]PlatformDetail, error)
	ListSchemaTargets(context.Context, int) ([]OrganizationSchemaTarget, error)
	GetSchemaTarget(context.Context, uuid.UUID) (*OrganizationSchemaTarget, error)
	CreateSchemaChangeRequest(context.Context, CreateSchemaChangeRequestRecord) (*SchemaChangeRequest, error)
	GetSchemaChangeRequest(context.Context, uuid.UUID) (*SchemaChangeRequest, error)
	UpdateSchemaChangeRequestStatus(context.Context, uuid.UUID, string, uuid.UUID, string) (*SchemaChangeRequest, error)
	ApplySchemaChange(context.Context, *SchemaChangeRequest, []string) (*SchemaApplyJob, error)
}

func NewService(repo repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListPlatformMasters(ctx context.Context, actorID uuid.UUID, moduleKey string, limit int) ([]PlatformMaster, error) {
	if err := s.requirePlatformAdmin(ctx, actorID); err != nil {
		return nil, err
	}
	return s.repo.ListPlatformMasters(ctx, moduleKey, limit)
}

func (s *Service) ListPlatformDetails(ctx context.Context, actorID uuid.UUID, masterKey string) ([]PlatformDetail, error) {
	if err := s.requirePlatformAdmin(ctx, actorID); err != nil {
		return nil, err
	}
	if masterKey == "" {
		return nil, fmt.Errorf("%w: master_key is required", ErrValidation)
	}
	return s.repo.ListPlatformDetails(ctx, masterKey)
}

func (s *Service) ListSchemaTargets(ctx context.Context, actorID uuid.UUID, limit int) ([]OrganizationSchemaTarget, error) {
	if err := s.requirePlatformAdmin(ctx, actorID); err != nil {
		return nil, err
	}
	return s.repo.ListSchemaTargets(ctx, limit)
}

func (s *Service) ExportOrganizationSchema(ctx context.Context, actorID uuid.UUID, orgID uuid.UUID) (*SchemaPackage, error) {
	if err := s.requirePlatformAdmin(ctx, actorID); err != nil {
		return nil, err
	}
	if orgID == uuid.Nil {
		return nil, fmt.Errorf("%w: organization_id is required", ErrValidation)
	}
	pkg := DefaultOrganizationSchemaPackage()
	return &pkg, nil
}

func (s *Service) CreateSchemaChangeRequest(ctx context.Context, actorID uuid.UUID, input CreateSchemaChangeRequestInput) (*SchemaChangeRequest, error) {
	if err := s.requirePlatformAdmin(ctx, actorID); err != nil {
		return nil, err
	}
	if input.OrganizationID == uuid.Nil {
		return nil, fmt.Errorf("%w: organization_id is required", ErrValidation)
	}
	if input.RequestType == "" {
		input.RequestType = "import_schema_package"
	}
	if err := ValidateSchemaPackage(input.SchemaPackage); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	schemaName := tenantdb.SchemaNameForOrganization(input.OrganizationID)
	statements, err := BuildCreateTableStatements(schemaName, input.SchemaPackage)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	return s.repo.CreateSchemaChangeRequest(ctx, CreateSchemaChangeRequestRecord{
		OrganizationID: input.OrganizationID,
		SchemaName:     schemaName,
		RequestType:    input.RequestType,
		Reason:         input.Reason,
		SchemaPackage:  input.SchemaPackage,
		Statements:     statements,
		RequestedBy:    actorID,
	})
}

func (s *Service) ApproveSchemaChange(ctx context.Context, actorID uuid.UUID, requestID uuid.UUID, reason string) (*SchemaChangeRequest, error) {
	if err := s.requirePlatformAdmin(ctx, actorID); err != nil {
		return nil, err
	}
	return s.repo.UpdateSchemaChangeRequestStatus(ctx, requestID, SchemaChangeApproved, actorID, reason)
}

func (s *Service) ApplySchemaChange(ctx context.Context, actorID uuid.UUID, requestID uuid.UUID) (*SchemaApplyJob, error) {
	if err := s.requirePlatformAdmin(ctx, actorID); err != nil {
		return nil, err
	}
	request, err := s.repo.GetSchemaChangeRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if request.Status != SchemaChangeApproved {
		return nil, ErrInvalidTransition
	}
	schemaName := request.SchemaName
	if schemaName == "" {
		schemaName = tenantdb.SchemaNameForOrganization(request.OrganizationID)
	}
	statements := request.Statements
	if len(statements) == 0 {
		statements, err = BuildCreateTableStatements(schemaName, request.SchemaPackage)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrValidation, err)
		}
	}
	return s.repo.ApplySchemaChange(ctx, request, statements)
}

func (s *Service) requirePlatformAdmin(ctx context.Context, actorID uuid.UUID) error {
	if actorID == uuid.Nil {
		return ErrForbidden
	}
	role, err := s.repo.GetPlatformRole(ctx, actorID)
	if err != nil || role == "" {
		return ErrForbidden
	}
	return nil
}
