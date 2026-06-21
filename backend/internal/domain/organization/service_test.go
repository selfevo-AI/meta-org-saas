package organization

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org/backend/internal/pkg/middleware"
)

func TestPlatformAdminCreatesPermissionChangeRequestWithoutUpdatingMembership(t *testing.T) {
	orgID := uuid.New()
	membershipID := uuid.New()
	actorID := uuid.New()
	repo := &fakeOrgRepository{
		membership: &OrganizationMembership{
			ID:             membershipID,
			OrganizationID: orgID,
			AuthorityTier:  AuthorityExecutor,
			Status:         "active",
		},
	}
	svc := NewService(repo)
	ctx := tenantContext(actorID, orgID, true, "")

	request, err := svc.CreatePermissionChangeRequest(ctx, orgID, CreatePermissionChangeRequestInput{
		MembershipID: membershipID,
		RequestedChange: UpdateOrganizationMembershipInput{
			AuthorityTier: AuthorityOrganizationAdmin,
		},
		Reason: "support escalation",
	})

	if err != nil {
		t.Fatalf("CreatePermissionChangeRequest error = %v", err)
	}
	if request.Status != PermissionChangePending {
		t.Fatalf("request status = %q, want pending", request.Status)
	}
	if repo.updatedMembership {
		t.Fatalf("membership was updated before organization review")
	}
}

func TestReviewPermissionChangeRequestAppliesApprovedMembershipChange(t *testing.T) {
	orgID := uuid.New()
	membershipID := uuid.New()
	reviewerID := uuid.New()
	requestID := uuid.New()
	repo := &fakeOrgRepository{
		membership: &OrganizationMembership{
			ID:             membershipID,
			OrganizationID: orgID,
			AuthorityTier:  AuthorityExecutor,
			Status:         "active",
		},
		permissionRequest: &PermissionChangeRequest{
			ID:             requestID,
			OrganizationID: orgID,
			MembershipID:   membershipID,
			Status:         PermissionChangePending,
			RequestedChange: UpdateOrganizationMembershipInput{
				AuthorityTier: AuthorityOrganizationAdmin,
			},
		},
		ownerCount: 2,
	}
	svc := NewService(repo)
	ctx := tenantContext(reviewerID, orgID, false, string(AuthorityOrganizationCreator))

	reviewed, err := svc.ReviewPermissionChangeRequest(ctx, requestID, ReviewPermissionChangeRequestInput{
		Decision: PermissionChangeApproved,
		Reason:   "owner reviewed",
	})

	if err != nil {
		t.Fatalf("ReviewPermissionChangeRequest error = %v", err)
	}
	if reviewed.Status != PermissionChangeApplied {
		t.Fatalf("reviewed status = %q, want applied", reviewed.Status)
	}
	if !repo.updatedMembership {
		t.Fatalf("membership was not updated after approval")
	}
	if repo.lastMembershipUpdate.AuthorityTier != AuthorityOrganizationAdmin {
		t.Fatalf("authority tier = %q, want organization_admin", repo.lastMembershipUpdate.AuthorityTier)
	}
}

func TestUpdateOrganizationMembershipRejectsLastOwnerDowngrade(t *testing.T) {
	orgID := uuid.New()
	membershipID := uuid.New()
	actorID := uuid.New()
	repo := &fakeOrgRepository{
		membership: &OrganizationMembership{
			ID:             membershipID,
			OrganizationID: orgID,
			AuthorityTier:  AuthorityOrganizationCreator,
			Status:         "active",
		},
		ownerCount: 1,
	}
	svc := NewService(repo)
	ctx := tenantContext(actorID, orgID, false, string(AuthorityOrganizationCreator))

	_, err := svc.UpdateOrganizationMembership(ctx, membershipID, UpdateOrganizationMembershipInput{
		AuthorityTier: AuthorityExecutor,
	})

	if !errors.Is(err, ErrValidation) {
		t.Fatalf("UpdateOrganizationMembership error = %v, want ErrValidation", err)
	}
	if repo.updatedMembership {
		t.Fatalf("last owner was downgraded")
	}
}

func TestCreateAccessRuleRequiresOrganizationPermissionManager(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()
	repo := &fakeOrgRepository{}
	svc := NewService(repo)
	ctx := tenantContext(actorID, orgID, false, string(AuthorityExecutor))

	_, err := svc.CreateAccessRule(ctx, orgID, CreateAccessRuleInput{
		ScopeType:    "project",
		ResourceType: "project",
		Action:       "read",
		Behavior:     "allow",
	})

	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("CreateAccessRule error = %v, want ErrForbidden", err)
	}
}

func TestCreateAccessRuleStoresCreatedByActor(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()
	repo := &fakeOrgRepository{}
	svc := NewService(repo)
	ctx := tenantContext(actorID, orgID, false, string(AuthorityOrganizationAdmin))

	rule, err := svc.CreateAccessRule(ctx, orgID, CreateAccessRuleInput{
		ScopeType:    "project",
		ScopeID:      "project-1",
		ResourceType: "project",
		Action:       "write",
		Behavior:     "deny",
		Reason:       "restricted project",
	})

	if err != nil {
		t.Fatalf("CreateAccessRule error = %v", err)
	}
	if rule.CreatedBy == nil || *rule.CreatedBy != actorID {
		t.Fatalf("CreatedBy = %#v, want %s", rule.CreatedBy, actorID)
	}
	if repo.accessRule == nil || repo.accessRule.OrganizationID != orgID {
		t.Fatalf("stored access rule = %#v, want org %s", repo.accessRule, orgID)
	}
}

func TestSingleOrgCanCreateAccessRuleWithoutSaaSOwnerTier(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()
	repo := &fakeOrgRepository{}
	svc := NewService(repo)
	ctx := tenantContextWithMode(actorID, orgID, "single_org", false, string(AuthorityExecutor))

	_, err := svc.CreateAccessRule(ctx, orgID, CreateAccessRuleInput{
		ScopeType:    "department",
		ResourceType: "organization_member",
		Action:       "read",
		Behavior:     "allow",
	})

	if err != nil {
		t.Fatalf("CreateAccessRule in single_org error = %v, want nil", err)
	}
}

func TestSingleOrgCanUpdateMembershipWithoutSaaSOwnerTier(t *testing.T) {
	orgID := uuid.New()
	membershipID := uuid.New()
	actorID := uuid.New()
	repo := &fakeOrgRepository{
		membership: &OrganizationMembership{
			ID:             membershipID,
			OrganizationID: orgID,
			AuthorityTier:  AuthorityExecutor,
			Status:         "active",
		},
	}
	svc := NewService(repo)
	ctx := tenantContextWithMode(actorID, orgID, "single_org", false, string(AuthorityExecutor))

	_, err := svc.UpdateOrganizationMembership(ctx, membershipID, UpdateOrganizationMembershipInput{
		Title: "Operations Lead",
	})

	if err != nil {
		t.Fatalf("UpdateOrganizationMembership in single_org error = %v, want nil", err)
	}
	if !repo.updatedMembership {
		t.Fatalf("membership was not updated in single_org")
	}
}

func tenantContext(userID, orgID uuid.UUID, platformAdmin bool, authorityTier string) context.Context {
	return tenantContextWithMode(userID, orgID, "saas", platformAdmin, authorityTier)
}

func tenantContextWithMode(userID, orgID uuid.UUID, mode string, platformAdmin bool, authorityTier string) context.Context {
	ctx := context.WithValue(context.Background(), middleware.UserContextKey, middleware.AuthenticatedUser{
		ID:   userID.String(),
		Type: "human",
		Name: "Tester",
	})
	return context.WithValue(ctx, middleware.TenantContextKey, &middleware.TenantContext{
		Mode:            mode,
		UserID:          userID,
		OrganizationID:  &orgID,
		IsPlatformAdmin: platformAdmin,
		AuthorityTier:   authorityTier,
		EnabledModules:  map[string]bool{"organization": true, "governance": true},
	})
}

type fakeOrgRepository struct {
	membership           *OrganizationMembership
	permissionRequest    *PermissionChangeRequest
	accessRule           *OrganizationAccessRule
	ownerCount           int
	updatedMembership    bool
	lastMembershipUpdate UpdateOrganizationMembershipInput
}

func (f *fakeOrgRepository) CreateOrganization(context.Context, CreateOrganizationInput) (*Organization, error) {
	return nil, nil
}
func (f *fakeOrgRepository) GetOrganizationByID(context.Context, uuid.UUID) (*Organization, error) {
	return nil, nil
}
func (f *fakeOrgRepository) ListOrganizations(context.Context, int) ([]Organization, error) {
	return nil, nil
}
func (f *fakeOrgRepository) UpdateOrganization(context.Context, uuid.UUID, UpdateOrganizationInput) (*Organization, error) {
	return nil, nil
}
func (f *fakeOrgRepository) CreateMVRU(context.Context, CreateMVRUInput) (*MVRU, error) {
	return nil, nil
}
func (f *fakeOrgRepository) GetMVRUByID(context.Context, uuid.UUID) (*MVRU, error) {
	return nil, nil
}
func (f *fakeOrgRepository) ListMVRUs(context.Context, uuid.UUID) ([]MVRU, error) {
	return nil, nil
}
func (f *fakeOrgRepository) UpdateMVRUStatus(context.Context, uuid.UUID, MVRUStatus) error {
	return nil
}
func (f *fakeOrgRepository) AddMember(context.Context, MVRUMember) error { return nil }
func (f *fakeOrgRepository) RemoveMember(context.Context, *uuid.UUID, *uuid.UUID, *uuid.UUID) error {
	return nil
}
func (f *fakeOrgRepository) CreateRelationship(context.Context, MVRURelationship) (*MVRURelationship, error) {
	return nil, nil
}
func (f *fakeOrgRepository) GetOrgChart(context.Context, uuid.UUID) ([]MVRU, error) {
	return nil, nil
}
func (f *fakeOrgRepository) CreateDepartment(context.Context, CreateDepartmentInput) (*Department, error) {
	return nil, nil
}
func (f *fakeOrgRepository) GetDepartmentByID(context.Context, uuid.UUID) (*Department, error) {
	return nil, nil
}
func (f *fakeOrgRepository) ListDepartments(context.Context, uuid.UUID) ([]Department, error) {
	return nil, nil
}
func (f *fakeOrgRepository) GetDepartmentTree(context.Context, uuid.UUID) ([]Department, error) {
	return nil, nil
}
func (f *fakeOrgRepository) UpdateDepartment(context.Context, uuid.UUID, UpdateDepartmentInput) (*Department, error) {
	return nil, nil
}
func (f *fakeOrgRepository) CreatePosition(context.Context, CreatePositionInput) (*Position, error) {
	return nil, nil
}
func (f *fakeOrgRepository) GetPositionByID(context.Context, uuid.UUID) (*Position, error) {
	return nil, nil
}
func (f *fakeOrgRepository) ListPositions(context.Context, uuid.UUID, *uuid.UUID) ([]Position, error) {
	return nil, nil
}
func (f *fakeOrgRepository) UpdatePosition(context.Context, uuid.UUID, UpdatePositionInput) (*Position, error) {
	return nil, nil
}
func (f *fakeOrgRepository) CreatePositionAssignment(context.Context, CreatePositionAssignmentInput) (*PositionAssignment, error) {
	return nil, nil
}
func (f *fakeOrgRepository) ListPositionAssignments(context.Context, uuid.UUID) ([]PositionAssignment, error) {
	return nil, nil
}
func (f *fakeOrgRepository) GetPositionAssignmentByID(context.Context, uuid.UUID) (*PositionAssignment, error) {
	return nil, nil
}
func (f *fakeOrgRepository) UpdatePositionAssignment(context.Context, uuid.UUID, UpdatePositionAssignmentInput) (*PositionAssignment, error) {
	return nil, nil
}
func (f *fakeOrgRepository) RemovePositionAssignment(context.Context, uuid.UUID) error { return nil }
func (f *fakeOrgRepository) CreateExternalMember(context.Context, CreateExternalMemberInput) (*ExternalMember, error) {
	return nil, nil
}
func (f *fakeOrgRepository) GetExternalMemberByID(context.Context, uuid.UUID) (*ExternalMember, error) {
	return nil, nil
}
func (f *fakeOrgRepository) ListExternalMembers(context.Context, int) ([]ExternalMember, error) {
	return nil, nil
}
func (f *fakeOrgRepository) UpdateExternalMember(context.Context, uuid.UUID, UpdateExternalMemberInput) (*ExternalMember, error) {
	return nil, nil
}
func (f *fakeOrgRepository) AddOrganizationMember(context.Context, AddOrganizationMemberInput) (*OrganizationMembership, error) {
	return nil, nil
}
func (f *fakeOrgRepository) GetOrganizationMembershipByID(context.Context, uuid.UUID) (*OrganizationMembership, error) {
	return f.membership, nil
}
func (f *fakeOrgRepository) ListOrganizationMemberships(context.Context, uuid.UUID, *uuid.UUID, []string) ([]OrganizationMembership, error) {
	if f.membership == nil {
		return []OrganizationMembership{}, nil
	}
	return []OrganizationMembership{*f.membership}, nil
}
func (f *fakeOrgRepository) UpdateOrganizationMembership(_ context.Context, _ uuid.UUID, input UpdateOrganizationMembershipInput) (*OrganizationMembership, error) {
	f.updatedMembership = true
	f.lastMembershipUpdate = input
	updated := *f.membership
	if input.AuthorityTier != "" {
		updated.AuthorityTier = input.AuthorityTier
	}
	if input.Status != "" {
		updated.Status = input.Status
	}
	f.membership = &updated
	return f.membership, nil
}
func (f *fakeOrgRepository) RemoveOrganizationMembership(context.Context, uuid.UUID) error {
	f.updatedMembership = true
	return nil
}
func (f *fakeOrgRepository) LinkDepartmentMVRU(context.Context, LinkDepartmentMVRUInput) (*DepartmentMVRULink, error) {
	return nil, nil
}
func (f *fakeOrgRepository) ListDepartmentMVRULinks(context.Context, uuid.UUID) ([]DepartmentMVRULink, error) {
	return nil, nil
}
func (f *fakeOrgRepository) CountActiveOrganizationOwners(context.Context, uuid.UUID, *uuid.UUID) (int, error) {
	return f.ownerCount, nil
}
func (f *fakeOrgRepository) CreatePermissionChangeRequest(_ context.Context, input CreatePermissionChangeRequestRecord) (*PermissionChangeRequest, error) {
	requestedBy := input.RequestedBy
	f.permissionRequest = &PermissionChangeRequest{
		ID:              uuid.New(),
		OrganizationID:  input.OrganizationID,
		MembershipID:    input.MembershipID,
		RequestedBy:     &requestedBy,
		RequestedByType: input.RequestedByType,
		RequestedChange: input.RequestedChange,
		Reason:          input.Reason,
		Status:          PermissionChangePending,
	}
	return f.permissionRequest, nil
}
func (f *fakeOrgRepository) GetPermissionChangeRequestByID(context.Context, uuid.UUID) (*PermissionChangeRequest, error) {
	return f.permissionRequest, nil
}
func (f *fakeOrgRepository) ListPermissionChangeRequests(context.Context, uuid.UUID, string, int) ([]PermissionChangeRequest, error) {
	if f.permissionRequest == nil {
		return []PermissionChangeRequest{}, nil
	}
	return []PermissionChangeRequest{*f.permissionRequest}, nil
}
func (f *fakeOrgRepository) UpdatePermissionChangeRequestStatus(_ context.Context, _ uuid.UUID, input UpdatePermissionChangeRequestStatusInput) (*PermissionChangeRequest, error) {
	updated := *f.permissionRequest
	updated.Status = input.Status
	updated.ReviewReason = input.ReviewReason
	f.permissionRequest = &updated
	return f.permissionRequest, nil
}
func (f *fakeOrgRepository) CreateAccessRule(_ context.Context, input CreateAccessRuleRecord) (*OrganizationAccessRule, error) {
	f.accessRule = &OrganizationAccessRule{
		ID:             uuid.New(),
		OrganizationID: input.OrganizationID,
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
		Status:         "active",
		Reason:         input.Reason,
		Metadata:       input.Metadata,
		CreatedBy:      input.CreatedBy,
	}
	return f.accessRule, nil
}
func (f *fakeOrgRepository) ListAccessRules(context.Context, uuid.UUID, string, string, int) ([]OrganizationAccessRule, error) {
	if f.accessRule == nil {
		return []OrganizationAccessRule{}, nil
	}
	return []OrganizationAccessRule{*f.accessRule}, nil
}
