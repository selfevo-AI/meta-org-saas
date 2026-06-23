package organization

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/evolution"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/governance"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/metaresource"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrValidation = errors.New("validation error")
	ErrForbidden  = errors.New("forbidden")
)

type Repository interface {
	CreateOrganization(ctx context.Context, input CreateOrganizationInput) (*Organization, error)
	GetOrganizationByID(ctx context.Context, id uuid.UUID) (*Organization, error)
	ListOrganizations(ctx context.Context, limit int) ([]Organization, error)
	UpdateOrganization(ctx context.Context, id uuid.UUID, input UpdateOrganizationInput) (*Organization, error)
	CreateMVRU(ctx context.Context, input CreateMVRUInput) (*MVRU, error)
	GetMVRUByID(ctx context.Context, id uuid.UUID) (*MVRU, error)
	ListMVRUs(ctx context.Context, orgID uuid.UUID) ([]MVRU, error)
	UpdateMVRUStatus(ctx context.Context, id uuid.UUID, status MVRUStatus) error
	AddMember(ctx context.Context, member MVRUMember) error
	RemoveMember(ctx context.Context, mvruID, userID, agentID *uuid.UUID) error
	CreateRelationship(ctx context.Context, rel MVRURelationship) (*MVRURelationship, error)
	GetOrgChart(ctx context.Context, orgID uuid.UUID) ([]MVRU, error)
	CreateDepartment(ctx context.Context, input CreateDepartmentInput) (*Department, error)
	GetDepartmentByID(ctx context.Context, id uuid.UUID) (*Department, error)
	ListDepartments(ctx context.Context, orgID uuid.UUID) ([]Department, error)
	GetDepartmentTree(ctx context.Context, orgID uuid.UUID) ([]Department, error)
	UpdateDepartment(ctx context.Context, id uuid.UUID, input UpdateDepartmentInput) (*Department, error)
	CreatePosition(ctx context.Context, input CreatePositionInput) (*Position, error)
	GetPositionByID(ctx context.Context, id uuid.UUID) (*Position, error)
	ListPositions(ctx context.Context, orgID uuid.UUID, departmentID *uuid.UUID) ([]Position, error)
	UpdatePosition(ctx context.Context, id uuid.UUID, input UpdatePositionInput) (*Position, error)
	CreatePositionAssignment(ctx context.Context, input CreatePositionAssignmentInput) (*PositionAssignment, error)
	ListPositionAssignments(ctx context.Context, positionID uuid.UUID) ([]PositionAssignment, error)
	GetPositionAssignmentByID(ctx context.Context, id uuid.UUID) (*PositionAssignment, error)
	UpdatePositionAssignment(ctx context.Context, id uuid.UUID, input UpdatePositionAssignmentInput) (*PositionAssignment, error)
	RemovePositionAssignment(ctx context.Context, id uuid.UUID) error
	CreateExternalMember(ctx context.Context, input CreateExternalMemberInput) (*ExternalMember, error)
	GetExternalMemberByID(ctx context.Context, id uuid.UUID) (*ExternalMember, error)
	ListExternalMembers(ctx context.Context, limit int) ([]ExternalMember, error)
	UpdateExternalMember(ctx context.Context, id uuid.UUID, input UpdateExternalMemberInput) (*ExternalMember, error)
	AddOrganizationMember(ctx context.Context, input AddOrganizationMemberInput) (*OrganizationMembership, error)
	GetOrganizationMembershipByID(ctx context.Context, id uuid.UUID) (*OrganizationMembership, error)
	ListOrganizationMemberships(ctx context.Context, orgID uuid.UUID, departmentID *uuid.UUID, memberTypes []string) ([]OrganizationMembership, error)
	UpdateOrganizationMembership(ctx context.Context, id uuid.UUID, input UpdateOrganizationMembershipInput) (*OrganizationMembership, error)
	RemoveOrganizationMembership(ctx context.Context, id uuid.UUID) error
	CountActiveOrganizationOwners(ctx context.Context, orgID uuid.UUID, excludeMembershipID *uuid.UUID) (int, error)
	CreatePermissionChangeRequest(ctx context.Context, input CreatePermissionChangeRequestRecord) (*PermissionChangeRequest, error)
	GetPermissionChangeRequestByID(ctx context.Context, id uuid.UUID) (*PermissionChangeRequest, error)
	ListPermissionChangeRequests(ctx context.Context, orgID uuid.UUID, status string, limit int) ([]PermissionChangeRequest, error)
	UpdatePermissionChangeRequestStatus(ctx context.Context, id uuid.UUID, input UpdatePermissionChangeRequestStatusInput) (*PermissionChangeRequest, error)
	CreateAccessRule(ctx context.Context, input CreateAccessRuleRecord) (*OrganizationAccessRule, error)
	ListAccessRules(ctx context.Context, orgID uuid.UUID, scopeType string, resourceType string, limit int) ([]OrganizationAccessRule, error)
	LinkDepartmentMVRU(ctx context.Context, input LinkDepartmentMVRUInput) (*DepartmentMVRULink, error)
	ListDepartmentMVRULinks(ctx context.Context, departmentID uuid.UUID) ([]DepartmentMVRULink, error)
}

type Service struct {
	repo       Repository
	governance *governance.Service
	evolution  *evolution.Service
	meta       MetaResourceService
}

type ServiceOption func(*Service)

type MetaResourceService interface {
	GetResource(ctx context.Context, id uuid.UUID) (*metaresource.MetaResource, error)
}

func WithGovernanceService(gov *governance.Service) ServiceOption {
	return func(s *Service) {
		s.governance = gov
	}
}

func WithEvolutionService(evo *evolution.Service) ServiceOption {
	return func(s *Service) {
		s.evolution = evo
	}
}

func WithMetaResourceService(meta MetaResourceService) ServiceOption {
	return func(s *Service) {
		s.meta = meta
	}
}

func NewService(repo Repository, opts ...ServiceOption) *Service {
	s := &Service{repo: repo}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Service) CreateOrganization(ctx context.Context, input CreateOrganizationInput) (*Organization, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrValidation)
	}
	if tenant, ok := middleware.TenantFromContext(ctx); ok && tenant.Mode == "saas" && !tenant.IsPlatformAdmin {
		return nil, fmt.Errorf("%w: only platform admins can create organizations outside onboarding", ErrForbidden)
	}
	return s.repo.CreateOrganization(ctx, input)
}

func (s *Service) GetCurrentOrganization(ctx context.Context) (*Organization, error) {
	if orgID := currentTenantOrganizationID(ctx); orgID != nil {
		return s.repo.GetOrganizationByID(ctx, *orgID)
	}
	organizations, err := s.repo.ListOrganizations(ctx, 1)
	if err != nil {
		return nil, err
	}
	if len(organizations) > 0 {
		return &organizations[0], nil
	}
	return s.repo.CreateOrganization(ctx, CreateOrganizationInput{
		Name:        "Default Organization",
		Description: "Single organization workspace",
	})
}

func (s *Service) GetOrganization(ctx context.Context, id uuid.UUID) (*Organization, error) {
	if err := ensureTenantAccess(ctx, id); err != nil {
		return nil, err
	}
	return s.repo.GetOrganizationByID(ctx, id)
}

func (s *Service) ListOrganizations(ctx context.Context, limit int) ([]Organization, error) {
	if orgID := currentTenantOrganizationID(ctx); orgID != nil {
		org, err := s.repo.GetOrganizationByID(ctx, *orgID)
		if err != nil {
			return nil, err
		}
		return []Organization{*org}, nil
	}
	organizations, err := s.repo.ListOrganizations(ctx, limit)
	if organizations == nil {
		organizations = []Organization{}
	}
	return organizations, err
}

func (s *Service) UpdateOrganization(ctx context.Context, id uuid.UUID, input UpdateOrganizationInput) (*Organization, error) {
	if err := ensureTenantAccess(ctx, id); err != nil {
		return nil, err
	}
	if input.Name == "" && input.Description == "" {
		return nil, fmt.Errorf("%w: name or description is required", ErrValidation)
	}
	return s.repo.UpdateOrganization(ctx, id, input)
}

func (s *Service) GetOrgChart(ctx context.Context, orgID uuid.UUID) ([]MVRU, error) {
	if err := ensureTenantAccess(ctx, orgID); err != nil {
		return nil, err
	}
	return s.repo.GetOrgChart(ctx, orgID)
}

func (s *Service) CreateMVRU(ctx context.Context, input CreateMVRUInput) (*MVRU, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrValidation)
	}
	if input.Boundary == nil {
		input.Boundary = map[string]any{}
	}
	if input.Config == nil {
		input.Config = map[string]any{}
	}
	if err := ensureTenantAccess(ctx, input.OrganizationID); err != nil {
		return nil, err
	}
	return s.repo.CreateMVRU(ctx, input)
}

func (s *Service) GetMVRU(ctx context.Context, id uuid.UUID) (*MVRU, error) {
	mvru, err := s.repo.GetMVRUByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := ensureTenantAccess(ctx, mvru.OrganizationID); err != nil {
		return nil, err
	}
	return mvru, nil
}

func (s *Service) ActivateMVRU(ctx context.Context, id uuid.UUID) error {
	mvru, err := s.repo.GetMVRUByID(ctx, id)
	if err != nil {
		return err
	}
	if err := ensureTenantAccess(ctx, mvru.OrganizationID); err != nil {
		return err
	}
	return s.repo.UpdateMVRUStatus(ctx, id, MVRUActive)
}

func (s *Service) EvaluateMVRU(ctx context.Context, id uuid.UUID) error {
	mvru, err := s.repo.GetMVRUByID(ctx, id)
	if err != nil {
		return err
	}
	if err := ensureTenantAccess(ctx, mvru.OrganizationID); err != nil {
		return err
	}
	return s.repo.UpdateMVRUStatus(ctx, id, MVRUEvaluating)
}

func (s *Service) AddMember(ctx context.Context, mvruID, roleID uuid.UUID, userID, agentID *uuid.UUID) error {
	if userID == nil && agentID == nil {
		return fmt.Errorf("%w: user_id or agent_id is required", ErrValidation)
	}
	mvru, err := s.repo.GetMVRUByID(ctx, mvruID)
	if err != nil {
		return err
	}
	if err := ensureTenantAccess(ctx, mvru.OrganizationID); err != nil {
		return err
	}
	return s.repo.AddMember(ctx, MVRUMember{
		MVRUID:  mvruID,
		UserID:  userID,
		AgentID: agentID,
		RoleID:  roleID,
	})
}

func (s *Service) RemoveMember(ctx context.Context, mvruID uuid.UUID, userID, agentID *uuid.UUID) error {
	mvru, err := s.repo.GetMVRUByID(ctx, mvruID)
	if err != nil {
		return err
	}
	if err := ensureTenantAccess(ctx, mvru.OrganizationID); err != nil {
		return err
	}
	return s.repo.RemoveMember(ctx, &mvruID, userID, agentID)
}

func (s *Service) CreateRelationship(ctx context.Context, sourceID, targetID uuid.UUID, relType string, config map[string]any) (*MVRURelationship, error) {
	source, err := s.repo.GetMVRUByID(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	target, err := s.repo.GetMVRUByID(ctx, targetID)
	if err != nil {
		return nil, err
	}
	if source.OrganizationID != target.OrganizationID {
		return nil, fmt.Errorf("%w: mvru relationship must stay inside one organization", ErrValidation)
	}
	if err := ensureTenantAccess(ctx, source.OrganizationID); err != nil {
		return nil, err
	}
	return s.repo.CreateRelationship(ctx, MVRURelationship{
		SourceMVRUID: sourceID,
		TargetMVRUID: targetID,
		RelType:      relType,
		Config:       config,
	})
}

func (s *Service) CreateDepartment(ctx context.Context, orgID uuid.UUID, input CreateDepartmentInput) (*Department, error) {
	if err := ensureTenantAccess(ctx, orgID); err != nil {
		return nil, err
	}
	if input.Name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrValidation)
	}
	input.OrganizationID = orgID
	normalizeDepartmentInput(&input)
	if !isValidOrgStatus(input.Status) {
		return nil, fmt.Errorf("%w: invalid department status", ErrValidation)
	}
	return s.repo.CreateDepartment(ctx, input)
}

func (s *Service) GetDepartment(ctx context.Context, id uuid.UUID) (*Department, error) {
	dept, err := s.repo.GetDepartmentByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := ensureTenantAccess(ctx, dept.OrganizationID); err != nil {
		return nil, err
	}
	return dept, nil
}

func (s *Service) ListDepartments(ctx context.Context, orgID uuid.UUID) ([]Department, error) {
	if err := ensureTenantAccess(ctx, orgID); err != nil {
		return nil, err
	}
	departments, err := s.repo.ListDepartments(ctx, orgID)
	if departments == nil {
		departments = []Department{}
	}
	return departments, err
}

func (s *Service) GetDepartmentTree(ctx context.Context, orgID uuid.UUID) ([]Department, error) {
	if err := ensureTenantAccess(ctx, orgID); err != nil {
		return nil, err
	}
	tree, err := s.repo.GetDepartmentTree(ctx, orgID)
	if tree == nil {
		tree = []Department{}
	}
	return tree, err
}

func (s *Service) UpdateDepartment(ctx context.Context, id uuid.UUID, input UpdateDepartmentInput) (*Department, error) {
	current, err := s.repo.GetDepartmentByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := ensureTenantAccess(ctx, current.OrganizationID); err != nil {
		return nil, err
	}
	if input.Status != "" && !isValidOrgStatus(input.Status) {
		return nil, fmt.Errorf("%w: invalid department status", ErrValidation)
	}
	if input.ParentID != nil && *input.ParentID == id {
		return nil, fmt.Errorf("%w: department cannot be its own parent", ErrValidation)
	}
	return s.repo.UpdateDepartment(ctx, id, input)
}

func (s *Service) CreatePosition(ctx context.Context, departmentID uuid.UUID, input CreatePositionInput) (*Position, error) {
	dept, err := s.repo.GetDepartmentByID(ctx, departmentID)
	if err != nil {
		return nil, err
	}
	if err := ensureTenantAccess(ctx, dept.OrganizationID); err != nil {
		return nil, err
	}
	input.OrganizationID = dept.OrganizationID
	input.DepartmentID = departmentID
	normalizePositionInput(&input)
	if input.Name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrValidation)
	}
	return s.repo.CreatePosition(ctx, input)
}

func (s *Service) GetPosition(ctx context.Context, id uuid.UUID) (*Position, error) {
	position, err := s.repo.GetPositionByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := ensureTenantAccess(ctx, position.OrganizationID); err != nil {
		return nil, err
	}
	return position, nil
}

func (s *Service) ListPositions(ctx context.Context, orgID uuid.UUID, departmentID *uuid.UUID) ([]Position, error) {
	if err := ensureTenantAccess(ctx, orgID); err != nil {
		return nil, err
	}
	positions, err := s.repo.ListPositions(ctx, orgID, departmentID)
	if positions == nil {
		positions = []Position{}
	}
	return positions, err
}

func (s *Service) UpdatePosition(ctx context.Context, id uuid.UUID, input UpdatePositionInput) (*Position, error) {
	current, err := s.repo.GetPositionByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := ensureTenantAccess(ctx, current.OrganizationID); err != nil {
		return nil, err
	}
	return s.repo.UpdatePosition(ctx, id, input)
}

func (s *Service) CreatePositionAssignment(ctx context.Context, positionID uuid.UUID, input CreatePositionAssignmentInput) (*PositionAssignment, error) {
	position, err := s.repo.GetPositionByID(ctx, positionID)
	if err != nil {
		return nil, err
	}
	if err := ensureTenantAccess(ctx, position.OrganizationID); err != nil {
		return nil, err
	}
	input.PositionID = positionID
	normalizePositionAssignmentInput(&input)
	if input.ActorID == uuid.Nil {
		return nil, fmt.Errorf("%w: actor_id is required", ErrValidation)
	}
	if !isValidPositionActorType(input.ActorType) {
		return nil, fmt.Errorf("%w: invalid actor_type", ErrValidation)
	}
	if err := s.validateMetaAssignment(ctx, input.MetaResourceID, input.ActorID, input.ActorType); err != nil {
		return nil, err
	}
	input.Metadata = mergeMetadata(input.Metadata, map[string]any{
		"organization_id": position.OrganizationID.String(),
		"department_id":   position.DepartmentID.String(),
	})
	return s.repo.CreatePositionAssignment(ctx, input)
}

func (s *Service) ListPositionAssignments(ctx context.Context, positionID uuid.UUID) ([]PositionAssignment, error) {
	position, err := s.repo.GetPositionByID(ctx, positionID)
	if err != nil {
		return nil, err
	}
	if err := ensureTenantAccess(ctx, position.OrganizationID); err != nil {
		return nil, err
	}
	assignments, err := s.repo.ListPositionAssignments(ctx, positionID)
	if assignments == nil {
		assignments = []PositionAssignment{}
	}
	return assignments, err
}

func (s *Service) UpdatePositionAssignment(ctx context.Context, id uuid.UUID, input UpdatePositionAssignmentInput) (*PositionAssignment, error) {
	current, err := s.repo.GetPositionAssignmentByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := ensureTenantAccess(ctx, current.OrganizationID); err != nil {
		return nil, err
	}
	if input.MetaResourceID != nil {
		if err := s.validateMetaAssignment(ctx, input.MetaResourceID, current.ActorID, current.ActorType); err != nil {
			return nil, err
		}
	}
	return s.repo.UpdatePositionAssignment(ctx, id, input)
}

func (s *Service) RemovePositionAssignment(ctx context.Context, id uuid.UUID) error {
	current, err := s.repo.GetPositionAssignmentByID(ctx, id)
	if err != nil {
		return err
	}
	if err := ensureTenantAccess(ctx, current.OrganizationID); err != nil {
		return err
	}
	return s.repo.RemovePositionAssignment(ctx, id)
}

func (s *Service) CreateExternalMember(ctx context.Context, input CreateExternalMemberInput) (*ExternalMember, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrValidation)
	}
	if input.Status == "" {
		input.Status = "active"
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
	if !isValidOrgStatus(input.Status) {
		return nil, fmt.Errorf("%w: invalid external member status", ErrValidation)
	}
	return s.repo.CreateExternalMember(ctx, input)
}

func (s *Service) GetExternalMember(ctx context.Context, id uuid.UUID) (*ExternalMember, error) {
	return s.repo.GetExternalMemberByID(ctx, id)
}

func (s *Service) ListExternalMembers(ctx context.Context, limit int) ([]ExternalMember, error) {
	members, err := s.repo.ListExternalMembers(ctx, limit)
	if members == nil {
		members = []ExternalMember{}
	}
	return members, err
}

func (s *Service) UpdateExternalMember(ctx context.Context, id uuid.UUID, input UpdateExternalMemberInput) (*ExternalMember, error) {
	if input.Status != "" && !isValidOrgStatus(input.Status) {
		return nil, fmt.Errorf("%w: invalid external member status", ErrValidation)
	}
	return s.repo.UpdateExternalMember(ctx, id, input)
}

func (s *Service) AddOrganizationMember(ctx context.Context, departmentID uuid.UUID, input AddOrganizationMemberInput) (*OrganizationMembership, error) {
	dept, err := s.repo.GetDepartmentByID(ctx, departmentID)
	if err != nil {
		return nil, err
	}
	if err := ensureTenantAccess(ctx, dept.OrganizationID); err != nil {
		return nil, err
	}
	input.DepartmentID = departmentID
	if input.Status == "" {
		input.Status = "active"
	}
	if input.AuthorityTier == "" {
		input.AuthorityTier = AuthorityExecutor
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
	if !isValidOrgStatus(input.Status) {
		return nil, fmt.Errorf("%w: invalid membership status", ErrValidation)
	}
	if !isValidAuthorityTier(input.AuthorityTier) {
		return nil, fmt.Errorf("%w: invalid authority tier", ErrValidation)
	}
	if err := validateMembershipActor(input); err != nil {
		return nil, err
	}
	return s.repo.AddOrganizationMember(ctx, input)
}

func (s *Service) ListOrganizationMemberships(ctx context.Context, orgID uuid.UUID, departmentID *uuid.UUID, memberTypes []string) ([]OrganizationMembership, error) {
	if err := ensureTenantAccess(ctx, orgID); err != nil {
		return nil, err
	}
	for _, memberType := range memberTypes {
		if !isValidMemberType(memberType) {
			return nil, fmt.Errorf("%w: invalid member type", ErrValidation)
		}
	}
	if memberTypes == nil {
		memberTypes = []string{}
	}
	memberships, err := s.repo.ListOrganizationMemberships(ctx, orgID, departmentID, memberTypes)
	if memberships == nil {
		memberships = []OrganizationMembership{}
	}
	return memberships, err
}

func (s *Service) UpdateOrganizationMembership(ctx context.Context, id uuid.UUID, input UpdateOrganizationMembershipInput) (*OrganizationMembership, error) {
	current, err := s.repo.GetOrganizationMembershipByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := ensureTenantAccess(ctx, current.OrganizationID); err != nil {
		return nil, err
	}
	if err := requireOrganizationPermissionManager(ctx); err != nil {
		return nil, err
	}
	if input.Status != "" && !isValidOrgStatus(input.Status) {
		return nil, fmt.Errorf("%w: invalid membership status", ErrValidation)
	}
	if input.AuthorityTier != "" && !isValidAuthorityTier(input.AuthorityTier) {
		return nil, fmt.Errorf("%w: invalid authority tier", ErrValidation)
	}
	if err := s.preventLastOwnerLoss(ctx, current, input.AuthorityTier, input.Status); err != nil {
		return nil, err
	}
	return s.repo.UpdateOrganizationMembership(ctx, id, input)
}

func (s *Service) RemoveOrganizationMembership(ctx context.Context, id uuid.UUID) error {
	current, err := s.repo.GetOrganizationMembershipByID(ctx, id)
	if err != nil {
		return err
	}
	if err := ensureTenantAccess(ctx, current.OrganizationID); err != nil {
		return err
	}
	if err := requireOrganizationPermissionManager(ctx); err != nil {
		return err
	}
	if err := s.preventLastOwnerLoss(ctx, current, AuthorityExecutor, "archived"); err != nil {
		return err
	}
	return s.repo.RemoveOrganizationMembership(ctx, id)
}

func (s *Service) CreatePermissionChangeRequest(ctx context.Context, orgID uuid.UUID, input CreatePermissionChangeRequestInput) (*PermissionChangeRequest, error) {
	if err := ensureTenantAccess(ctx, orgID); err != nil {
		return nil, err
	}
	if input.MembershipID == uuid.Nil {
		return nil, fmt.Errorf("%w: membership_id is required", ErrValidation)
	}
	if input.RequestedChange.AuthorityTier == "" && input.RequestedChange.Status == "" && input.RequestedChange.RoleID == nil && input.RequestedChange.Title == "" {
		return nil, fmt.Errorf("%w: requested_change is required", ErrValidation)
	}
	if input.RequestedChange.AuthorityTier != "" && !isValidAuthorityTier(input.RequestedChange.AuthorityTier) {
		return nil, fmt.Errorf("%w: invalid authority tier", ErrValidation)
	}
	if input.RequestedChange.Status != "" && !isValidOrgStatus(input.RequestedChange.Status) {
		return nil, fmt.Errorf("%w: invalid membership status", ErrValidation)
	}
	current, err := s.repo.GetOrganizationMembershipByID(ctx, input.MembershipID)
	if err != nil {
		return nil, err
	}
	if current.OrganizationID != orgID {
		return nil, fmt.Errorf("%w: membership is outside organization", ErrForbidden)
	}
	user, ok := middleware.UserFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("%w: authenticated user is required", ErrForbidden)
	}
	requestedBy, err := uuid.Parse(user.ID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid authenticated user", ErrValidation)
	}
	return s.repo.CreatePermissionChangeRequest(ctx, CreatePermissionChangeRequestRecord{
		OrganizationID:  orgID,
		MembershipID:    input.MembershipID,
		RequestedBy:     requestedBy,
		RequestedByType: user.Type,
		RequestedChange: input.RequestedChange,
		Reason:          input.Reason,
	})
}

func (s *Service) ListPermissionChangeRequests(ctx context.Context, orgID uuid.UUID, status string, limit int) ([]PermissionChangeRequest, error) {
	if err := ensureTenantAccess(ctx, orgID); err != nil {
		return nil, err
	}
	if status != "" && !isValidPermissionChangeStatus(status) {
		return nil, fmt.Errorf("%w: invalid permission change status", ErrValidation)
	}
	return s.repo.ListPermissionChangeRequests(ctx, orgID, status, limit)
}

func (s *Service) ReviewPermissionChangeRequest(ctx context.Context, id uuid.UUID, input ReviewPermissionChangeRequestInput) (*PermissionChangeRequest, error) {
	if err := requireOrganizationPermissionManager(ctx); err != nil {
		return nil, err
	}
	request, err := s.repo.GetPermissionChangeRequestByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := ensureTenantAccess(ctx, request.OrganizationID); err != nil {
		return nil, err
	}
	if request.Status != PermissionChangePending {
		return nil, fmt.Errorf("%w: permission change request is not pending", ErrValidation)
	}
	if input.Decision != PermissionChangeApproved && input.Decision != PermissionChangeRejected {
		return nil, fmt.Errorf("%w: decision must be approved or rejected", ErrValidation)
	}
	user, ok := middleware.UserFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("%w: authenticated user is required", ErrForbidden)
	}
	reviewedBy, err := uuid.Parse(user.ID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid authenticated user", ErrValidation)
	}
	if input.Decision == PermissionChangeRejected {
		return s.repo.UpdatePermissionChangeRequestStatus(ctx, id, UpdatePermissionChangeRequestStatusInput{
			Status:       PermissionChangeRejected,
			ReviewedBy:   &reviewedBy,
			ReviewReason: input.Reason,
		})
	}
	current, err := s.repo.GetOrganizationMembershipByID(ctx, request.MembershipID)
	if err != nil {
		return nil, err
	}
	if current.OrganizationID != request.OrganizationID {
		return nil, fmt.Errorf("%w: membership is outside request organization", ErrForbidden)
	}
	if err := s.preventLastOwnerLoss(ctx, current, request.RequestedChange.AuthorityTier, request.RequestedChange.Status); err != nil {
		return nil, err
	}
	if _, err := s.repo.UpdateOrganizationMembership(ctx, request.MembershipID, request.RequestedChange); err != nil {
		return nil, err
	}
	return s.repo.UpdatePermissionChangeRequestStatus(ctx, id, UpdatePermissionChangeRequestStatusInput{
		Status:       PermissionChangeApplied,
		ReviewedBy:   &reviewedBy,
		ReviewReason: input.Reason,
	})
}

func (s *Service) CreateAccessRule(ctx context.Context, orgID uuid.UUID, input CreateAccessRuleInput) (*OrganizationAccessRule, error) {
	if err := ensureTenantAccess(ctx, orgID); err != nil {
		return nil, err
	}
	if err := requireOrganizationPermissionManager(ctx); err != nil {
		return nil, err
	}
	normalizeAccessRuleInput(&input)
	if !isValidAccessRuleScope(input.ScopeType) {
		return nil, fmt.Errorf("%w: invalid scope_type", ErrValidation)
	}
	if input.ResourceType == "" {
		return nil, fmt.Errorf("%w: resource_type is required", ErrValidation)
	}
	if !isValidAccessRuleAction(input.Action) {
		return nil, fmt.Errorf("%w: invalid action", ErrValidation)
	}
	if !isValidAccessRuleBehavior(input.Behavior) {
		return nil, fmt.Errorf("%w: invalid behavior", ErrValidation)
	}
	if !isValidPermissionLevel(input.RequiredLevel) {
		return nil, fmt.Errorf("%w: invalid required_level", ErrValidation)
	}
	if input.AuthorityTier != "" && !isValidAuthorityTier(input.AuthorityTier) {
		return nil, fmt.Errorf("%w: invalid authority_tier", ErrValidation)
	}
	var createdBy *uuid.UUID
	if user, ok := middleware.UserFromContext(ctx); ok {
		if parsed, err := uuid.Parse(user.ID); err == nil {
			createdBy = &parsed
		}
	}
	return s.repo.CreateAccessRule(ctx, CreateAccessRuleRecord{
		OrganizationID: orgID,
		ScopeType:      input.ScopeType,
		ScopeID:        input.ScopeID,
		ResourceType:   input.ResourceType,
		ResourceKey:    input.ResourceKey,
		Action:         input.Action,
		ActorType:      input.ActorType,
		ActorID:        input.ActorID,
		RoleID:         input.RoleID,
		AuthorityTier:  input.AuthorityTier,
		Behavior:       input.Behavior,
		RequiredLevel:  input.RequiredLevel,
		Priority:       input.Priority,
		Reason:         input.Reason,
		Metadata:       input.Metadata,
		CreatedBy:      createdBy,
	})
}

func (s *Service) ListAccessRules(ctx context.Context, orgID uuid.UUID, scopeType string, resourceType string, limit int) ([]OrganizationAccessRule, error) {
	if err := ensureTenantAccess(ctx, orgID); err != nil {
		return nil, err
	}
	if scopeType != "" && !isValidAccessRuleScope(scopeType) {
		return nil, fmt.Errorf("%w: invalid scope_type", ErrValidation)
	}
	return s.repo.ListAccessRules(ctx, orgID, scopeType, resourceType, limit)
}

func (s *Service) LinkDepartmentMVRU(ctx context.Context, departmentID uuid.UUID, input LinkDepartmentMVRUInput) (*DepartmentMVRULink, error) {
	dept, err := s.repo.GetDepartmentByID(ctx, departmentID)
	if err != nil {
		return nil, err
	}
	if err := ensureTenantAccess(ctx, dept.OrganizationID); err != nil {
		return nil, err
	}
	input.DepartmentID = departmentID
	if input.MVRUID == uuid.Nil {
		return nil, fmt.Errorf("%w: mvru_id is required", ErrValidation)
	}
	if input.LinkType == "" {
		input.LinkType = "execution"
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
	return s.repo.LinkDepartmentMVRU(ctx, input)
}

func (s *Service) ListDepartmentMVRULinks(ctx context.Context, departmentID uuid.UUID) ([]DepartmentMVRULink, error) {
	dept, err := s.repo.GetDepartmentByID(ctx, departmentID)
	if err != nil {
		return nil, err
	}
	if err := ensureTenantAccess(ctx, dept.OrganizationID); err != nil {
		return nil, err
	}
	return s.repo.ListDepartmentMVRULinks(ctx, departmentID)
}

func (s *Service) MatchMembers(ctx context.Context, input MatchMembersInput) ([]MemberMatchCandidate, error) {
	if input.OrganizationID == uuid.Nil {
		return nil, fmt.Errorf("%w: organization_id is required", ErrValidation)
	}
	if err := ensureTenantAccess(ctx, input.OrganizationID); err != nil {
		return nil, err
	}
	if input.TaskDescription == "" {
		return nil, fmt.Errorf("%w: task_description is required", ErrValidation)
	}
	if input.PositionID != nil {
		position, err := s.repo.GetPositionByID(ctx, *input.PositionID)
		if err != nil {
			return nil, err
		}
		assignments, err := s.repo.ListPositionAssignments(ctx, *input.PositionID)
		if err != nil {
			return nil, err
		}
		candidates := make([]MemberMatchCandidate, 0, len(assignments))
		for _, assignment := range assignments {
			if assignment.Status == "archived" || !matchesMemberTypes(assignment.ActorType, input.MemberTypes) {
				continue
			}
			score := 0.72
			reason := "position assignment"
			if assignment.AssignmentType == "primary" {
				score += 0.12
				reason += ", primary"
			}
			if len(input.RequiredCapabilities) > 0 {
				score += capabilityOverlapScore(position.RequiredCapabilities, input.RequiredCapabilities)
				reason += ", capability fit"
			}
			weightSnapshot := 0.5
			if s.evolution != nil {
				weight, err := s.evolution.ComputeContextWeight(ctx, evolution.ContextWeightInput{
					ActorID:       assignment.ActorID,
					ActorType:     assignment.ActorType,
					RequiredLevel: firstNonEmpty(input.RequiredLevel, position.PermissionLevel),
					Scope: evolution.ContextWeightScope{
						OrganizationID:     &assignment.OrganizationID,
						DepartmentID:       &assignment.DepartmentID,
						WorkflowTemplateID: input.WorkflowTemplateID,
						TaskType:           input.TaskDescription,
						RiskLevel:          normalizeRiskLevel(input.RiskLevel),
						Context: map[string]any{
							"position_id":           position.ID.String(),
							"position_name":         position.Name,
							"required_capabilities": input.RequiredCapabilities,
						},
					},
				})
				if err == nil {
					weightSnapshot = weight.OverallScore
					reason += ", contextual weight"
				}
			}
			accessDecision := "notify"
			accessAllowed := true
			requiresApproval := false
			if s.governance != nil {
				access, err := s.governance.DecideAccess(ctx, governance.AccessDecisionInput{
					ActorID:        assignment.ActorID,
					ActorType:      assignment.ActorType,
					Action:         "workflow.assign",
					Resource:       "position",
					ResourceID:     &position.ID,
					OrganizationID: &assignment.OrganizationID,
					DepartmentID:   &assignment.DepartmentID,
					RequiredLevel:  firstNonEmpty(input.RequiredLevel, position.PermissionLevel),
					RiskLevel:      normalizeRiskLevel(input.RiskLevel),
					WeightSnapshot: &weightSnapshot,
					Context: map[string]any{
						"task_description":       input.TaskDescription,
						"position_assignment_id": assignment.ID.String(),
						"workflow_template_id":   uuidString(input.WorkflowTemplateID),
					},
				})
				if err == nil {
					accessDecision = access.Decision
					accessAllowed = access.Allowed
					requiresApproval = access.Decision == "approve"
					reason += ", access " + access.Decision
				}
			}
			if accessDecision == "deny" {
				continue
			}
			score = (score * 0.6) + (weightSnapshot * 0.3)
			if accessAllowed {
				score += 0.1
			}
			if requiresApproval {
				score -= 0.05
			}
			candidates = append(candidates, MemberMatchCandidate{
				MembershipID:         assignment.ID,
				DepartmentID:         assignment.DepartmentID,
				PositionID:           &position.ID,
				PositionName:         position.Name,
				PositionAssignmentID: &assignment.ID,
				MemberType:           memberTypeFromActorType(assignment.ActorType),
				MemberID:             assignment.ActorID,
				MemberName:           assignment.ActorName,
				Title:                position.Name,
				Score:                clampScore(score),
				WeightSnapshot:       weightSnapshot,
				AccessDecision:       accessDecision,
				AccessAllowed:        accessAllowed,
				RequiresApproval:     requiresApproval,
				Reason:               reason,
				CapabilityMatchPath:  "/api/v1/capabilities/match",
				WorkflowAssignHint:   "Use member_id as task assignee_id and actor_type as assignee_type.",
			})
		}
		sort.SliceStable(candidates, func(i, j int) bool {
			return candidates[i].Score > candidates[j].Score
		})
		if len(candidates) > 10 {
			candidates = candidates[:10]
		}
		return candidates, nil
	}
	memberships, err := s.ListOrganizationMemberships(ctx, input.OrganizationID, input.DepartmentID, input.MemberTypes)
	if err != nil {
		return nil, err
	}

	candidates := make([]MemberMatchCandidate, 0, len(memberships))
	for _, membership := range memberships {
		if membership.Status == "archived" {
			continue
		}
		memberID, ok := membershipActorID(membership)
		if !ok {
			continue
		}
		baseScore, reason := scoreMembership(membership, input)
		actorType := membershipActorType(membership)
		weightSnapshot := 0.5
		if s.evolution != nil {
			weight, err := s.evolution.ComputeContextWeight(ctx, evolution.ContextWeightInput{
				ActorID:       memberID,
				ActorType:     actorType,
				RequiredLevel: input.RequiredLevel,
				Scope: evolution.ContextWeightScope{
					OrganizationID:     &input.OrganizationID,
					DepartmentID:       &membership.DepartmentID,
					WorkflowTemplateID: input.WorkflowTemplateID,
					TaskType:           input.TaskDescription,
					RiskLevel:          normalizeRiskLevel(input.RiskLevel),
					Context: map[string]any{
						"required_capabilities": input.RequiredCapabilities,
						"required_level":        input.RequiredLevel,
						"member_type":           membership.MemberType,
						"membership_id":         membership.ID.String(),
					},
				},
			})
			if err == nil {
				weightSnapshot = weight.OverallScore
				reason += ", contextual weight"
			}
		}

		accessDecision := "notify"
		accessAllowed := true
		requiresApproval := false
		if s.governance != nil {
			access, err := s.governance.DecideAccess(ctx, governance.AccessDecisionInput{
				ActorID:        memberID,
				ActorType:      actorType,
				Action:         "workflow.assign",
				Resource:       "organization_member",
				ResourceID:     &membership.ID,
				OrganizationID: &input.OrganizationID,
				DepartmentID:   &membership.DepartmentID,
				RequiredLevel:  input.RequiredLevel,
				RiskLevel:      normalizeRiskLevel(input.RiskLevel),
				WeightSnapshot: &weightSnapshot,
				Context: map[string]any{
					"task_description":      input.TaskDescription,
					"required_capabilities": input.RequiredCapabilities,
					"workflow_template_id":  uuidString(input.WorkflowTemplateID),
				},
			})
			if err == nil {
				accessDecision = access.Decision
				accessAllowed = access.Allowed
				requiresApproval = access.Decision == "approve"
				reason += ", access " + access.Decision
			}
		}
		if accessDecision == "deny" {
			continue
		}
		score := (baseScore * 0.55) + (weightSnapshot * 0.35)
		if accessAllowed {
			score += 0.10
		}
		if requiresApproval {
			score -= 0.05
		}
		if score > 1 {
			score = 1
		}
		if score < 0 {
			score = 0
		}
		candidates = append(candidates, MemberMatchCandidate{
			MembershipID:        membership.ID,
			DepartmentID:        membership.DepartmentID,
			MemberType:          membership.MemberType,
			MemberID:            memberID,
			MemberName:          membership.MemberName,
			Title:               membership.Title,
			Score:               score,
			WeightSnapshot:      weightSnapshot,
			AccessDecision:      accessDecision,
			AccessAllowed:       accessAllowed,
			RequiresApproval:    requiresApproval,
			Reason:              reason,
			CapabilityMatchPath: "/api/v1/capabilities/match",
			WorkflowAssignHint:  "Use member_id as task assignee_id and member_type as assignee_type.",
		})
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})
	if len(candidates) > 10 {
		candidates = candidates[:10]
	}
	return candidates, nil
}

func (s *Service) MatchCapabilities(ctx context.Context, input MatchCapabilitiesInput) (*CapabilityMatchBridge, error) {
	if input.TaskDescription == "" {
		return nil, fmt.Errorf("%w: task_description is required", ErrValidation)
	}
	if input.Context == nil {
		input.Context = map[string]any{}
	}
	if input.DepartmentID != nil {
		input.Context["department_id"] = input.DepartmentID.String()
	}
	if len(input.RequiredCapabilities) > 0 {
		input.Context["required_capabilities"] = input.RequiredCapabilities
	}
	if input.RiskLevel == "" {
		input.RiskLevel = "low"
	}
	return &CapabilityMatchBridge{
		DepartmentID:         input.DepartmentID,
		TaskDescription:      input.TaskDescription,
		RequiredCapabilities: input.RequiredCapabilities,
		RequiredLevel:        input.RequiredLevel,
		RiskLevel:            input.RiskLevel,
		CapabilityMatchPath:  "/api/v1/capabilities/match",
		ContextWeightPath:    "/api/v1/evolution/context-weights/compute",
		AccessDecisionPath:   "/api/v1/governance/access/decide",
		WorkflowStartPath:    "/api/v1/workflows/instances",
		Context:              input.Context,
	}, nil
}

func normalizeDepartmentInput(input *CreateDepartmentInput) {
	if input.Status == "" {
		input.Status = "active"
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
}

func normalizePositionInput(input *CreatePositionInput) {
	if input.Status == "" {
		input.Status = "active"
	}
	if input.PermissionLevel == "" {
		input.PermissionLevel = "L1"
	}
	if input.RequiredCapabilities == nil {
		input.RequiredCapabilities = []string{}
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
}

func normalizePositionAssignmentInput(input *CreatePositionAssignmentInput) {
	if input.AssignmentType == "" {
		input.AssignmentType = "candidate"
	}
	if input.AllocationPercent <= 0 {
		input.AllocationPercent = 100
	}
	if input.Status == "" {
		input.Status = "active"
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
}

func isValidPositionActorType(actorType string) bool {
	switch actorType {
	case "internal_human", "external_human", "internal_agent", "external_agent":
		return true
	default:
		return false
	}
}

func (s *Service) validateMetaAssignment(ctx context.Context, metaResourceID *uuid.UUID, actorID uuid.UUID, actorType string) error {
	if metaResourceID == nil || *metaResourceID == uuid.Nil {
		return fmt.Errorf("%w: meta_resource_id is required", ErrValidation)
	}
	if s.meta == nil {
		return nil
	}
	resource, err := s.meta.GetResource(ctx, *metaResourceID)
	if err != nil {
		return err
	}
	if !metaResourceTypeMatchesActor(resource.ResourceType, actorType) {
		return fmt.Errorf("%w: meta_resource type %q does not match actor_type %q", ErrValidation, resource.ResourceType, actorType)
	}
	if resource.OwnerActorID != nil && *resource.OwnerActorID != actorID {
		return fmt.Errorf("%w: meta_resource owner does not match actor_id", ErrValidation)
	}
	if resource.SourceID != nil && *resource.SourceID != actorID {
		return fmt.Errorf("%w: meta_resource source does not match actor_id", ErrValidation)
	}
	return nil
}

func metaResourceTypeMatchesActor(resourceType string, actorType string) bool {
	switch actorType {
	case "internal_human":
		return resourceType == "internal_human" || resourceType == "human"
	case "external_human":
		return resourceType == "external_human"
	case "internal_agent":
		return resourceType == "internal_agent" || resourceType == "agent"
	case "external_agent":
		return resourceType == "external_agent"
	default:
		return false
	}
}

func matchesMemberTypes(actorType string, memberTypes []string) bool {
	if len(memberTypes) == 0 {
		return true
	}
	memberType := memberTypeFromActorType(actorType)
	for _, allowed := range memberTypes {
		if allowed == memberType || allowed == actorType {
			return true
		}
	}
	return false
}

func memberTypeFromActorType(actorType string) string {
	switch actorType {
	case "internal_human":
		return "internal"
	case "external_human":
		return "external"
	case "internal_agent", "external_agent":
		return "agent"
	default:
		return actorType
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func capabilityOverlapScore(positionCapabilities, requiredCapabilities []string) float64 {
	if len(positionCapabilities) == 0 || len(requiredCapabilities) == 0 {
		return 0
	}
	lookup := make(map[string]struct{}, len(positionCapabilities))
	for _, capability := range positionCapabilities {
		lookup[strings.ToLower(strings.TrimSpace(capability))] = struct{}{}
	}
	matches := 0
	for _, capability := range requiredCapabilities {
		if _, ok := lookup[strings.ToLower(strings.TrimSpace(capability))]; ok {
			matches++
		}
	}
	return float64(matches) / float64(len(requiredCapabilities)) * 0.12
}

func clampScore(score float64) float64 {
	if score > 1 {
		return 1
	}
	if score < 0 {
		return 0
	}
	return score
}

func mergeMetadata(base map[string]any, extra map[string]any) map[string]any {
	if base == nil {
		base = map[string]any{}
	}
	for key, value := range extra {
		base[key] = value
	}
	return base
}

func isValidOrgStatus(status string) bool {
	switch status {
	case "active", "inactive", "archived":
		return true
	default:
		return false
	}
}

func isValidMemberType(memberType string) bool {
	switch memberType {
	case "internal", "external", "agent":
		return true
	default:
		return false
	}
}

func isValidAuthorityTier(tier AuthorityTier) bool {
	switch tier {
	case AuthorityOrganizationCreator, AuthorityOrganizationAdmin, AuthorityReviewer, AuthorityExecutor:
		return true
	default:
		return false
	}
}

func isValidPermissionChangeStatus(status string) bool {
	switch status {
	case PermissionChangePending, PermissionChangeApproved, PermissionChangeRejected, PermissionChangeApplied, PermissionChangeCancelled:
		return true
	default:
		return false
	}
}

func normalizeAccessRuleInput(input *CreateAccessRuleInput) {
	if input.ScopeType == "" {
		input.ScopeType = "organization"
	}
	if input.Action == "" {
		input.Action = "read"
	}
	if input.ActorType == "" {
		input.ActorType = "*"
	}
	if input.Behavior == "" {
		input.Behavior = "allow"
	}
	if input.RequiredLevel == "" {
		input.RequiredLevel = "L1"
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
}

func isValidAccessRuleScope(scopeType string) bool {
	switch scopeType {
	case "organization", "department", "project", "function", "form", "field":
		return true
	default:
		return false
	}
}

func isValidAccessRuleAction(action string) bool {
	switch action {
	case "read", "write", "delete", "admin", "execute":
		return true
	default:
		return false
	}
}

func isValidAccessRuleBehavior(behavior string) bool {
	switch behavior {
	case "allow", "notify", "approve", "deny":
		return true
	default:
		return false
	}
}

func isValidPermissionLevel(level string) bool {
	switch level {
	case "L1", "L2", "L3", "L4":
		return true
	default:
		return false
	}
}

func requireOrganizationPermissionManager(ctx context.Context) error {
	tenant, ok := middleware.TenantFromContext(ctx)
	if !ok {
		return nil
	}
	if tenant.Mode == "single_org" {
		return nil
	}
	if tenant.IsPlatformAdmin {
		return fmt.Errorf("%w: platform admins must use organization review requests", ErrForbidden)
	}
	switch tenant.AuthorityTier {
	case string(AuthorityOrganizationCreator), string(AuthorityOrganizationAdmin):
		return nil
	default:
		return fmt.Errorf("%w: organization owner or admin is required", ErrForbidden)
	}
}

func (s *Service) preventLastOwnerLoss(ctx context.Context, current *OrganizationMembership, nextTier AuthorityTier, nextStatus string) error {
	if current == nil || current.AuthorityTier != AuthorityOrganizationCreator || current.Status != "active" {
		return nil
	}
	losesOwnerTier := nextTier != "" && nextTier != AuthorityOrganizationCreator
	losesActiveStatus := nextStatus != "" && nextStatus != "active"
	if !losesOwnerTier && !losesActiveStatus {
		return nil
	}
	count, err := s.repo.CountActiveOrganizationOwners(ctx, current.OrganizationID, nil)
	if err != nil {
		return err
	}
	if count <= 1 {
		return fmt.Errorf("%w: organization must keep at least one active owner", ErrValidation)
	}
	return nil
}

func validateMembershipActor(input AddOrganizationMemberInput) error {
	if !isValidMemberType(input.MemberType) {
		return fmt.Errorf("%w: invalid member type", ErrValidation)
	}
	switch input.MemberType {
	case "internal":
		if input.UserID == nil || input.ExternalMemberID != nil || input.AgentID != nil {
			return fmt.Errorf("%w: internal membership requires only user_id", ErrValidation)
		}
	case "external":
		if input.ExternalMemberID == nil || input.UserID != nil || input.AgentID != nil {
			return fmt.Errorf("%w: external membership requires only external_member_id", ErrValidation)
		}
	case "agent":
		if input.AgentID == nil || input.UserID != nil || input.ExternalMemberID != nil {
			return fmt.Errorf("%w: agent membership requires only agent_id", ErrValidation)
		}
	}
	return nil
}

func membershipActorID(membership OrganizationMembership) (uuid.UUID, bool) {
	switch membership.MemberType {
	case "internal":
		if membership.UserID != nil {
			return *membership.UserID, true
		}
	case "external":
		if membership.ExternalMemberID != nil {
			return *membership.ExternalMemberID, true
		}
	case "agent":
		if membership.AgentID != nil {
			return *membership.AgentID, true
		}
	}
	return uuid.Nil, false
}

func membershipActorType(membership OrganizationMembership) string {
	switch membership.MemberType {
	case "internal":
		return "internal_human"
	case "external":
		return "external_human"
	case "agent":
		if origin, ok := membership.Metadata["agent_origin"].(string); ok && origin == "external" {
			return "external_agent"
		}
		return "internal_agent"
	default:
		return membership.MemberType
	}
}

func scoreMembership(membership OrganizationMembership, input MatchMembersInput) (float64, string) {
	score := 0.45
	reasons := []string{"department member"}
	if membership.Status == "active" {
		score += 0.25
		reasons = append(reasons, "active")
	}
	if titleMatchesTask(membership.Title, input.TaskDescription) {
		score += 0.15
		reasons = append(reasons, "title matches task")
	}
	if len(input.RequiredCapabilities) > 0 && membership.MemberType == "agent" {
		score += 0.1
		reasons = append(reasons, "agent capability candidate")
	}
	if input.WorkflowTemplateID != nil {
		score += 0.05
		reasons = append(reasons, "workflow context available")
	}
	if score > 1 {
		score = 1
	}
	return score, strings.Join(reasons, ", ")
}

func titleMatchesTask(title, task string) bool {
	title = strings.ToLower(title)
	task = strings.ToLower(task)
	for _, word := range strings.Fields(task) {
		if len(word) > 3 && strings.Contains(title, word) {
			return true
		}
	}
	return false
}

func normalizeRiskLevel(riskLevel string) string {
	switch riskLevel {
	case "low", "medium", "high", "critical":
		return riskLevel
	default:
		return "low"
	}
}

func uuidString(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

func currentTenantOrganizationID(ctx context.Context) *uuid.UUID {
	tenant, ok := middleware.TenantFromContext(ctx)
	if !ok || tenant.OrganizationID == nil {
		return nil
	}
	id := *tenant.OrganizationID
	return &id
}

func ensureTenantAccess(ctx context.Context, organizationID uuid.UUID) error {
	tenant, ok := middleware.TenantFromContext(ctx)
	if !ok || tenant.OrganizationID == nil {
		return nil
	}
	if *tenant.OrganizationID != organizationID {
		return fmt.Errorf("%w: resource is outside current organization", ErrForbidden)
	}
	return nil
}
