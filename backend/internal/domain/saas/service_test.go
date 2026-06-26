package saas

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/securitykernel"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/tenantdb"
)

func TestCompleteOnboardingRequiresSecurityKernelAuthorization(t *testing.T) {
	userID := uuid.New()
	kernel := &fakeSecurityKernel{decision: securitykernel.Decision{Allowed: false, Reason: "owner_attestation_required", DecisionType: "deny"}, err: securitykernel.ErrDenied}
	repo := &fakeRepository{}
	svc := NewService(repo, ModeSaaS, WithSecurityKernel(kernel))

	_, err := svc.CompleteOnboarding(context.Background(), userID, OnboardingOrganizationInput{OrganizationName: "Acme"})

	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("CompleteOnboarding error = %v, want ErrForbidden", err)
	}
	if repo.completedOnboarding {
		t.Fatalf("onboarding completed after security denial")
	}
	if kernel.lastRequest.Resource.ResourceType != "owner_attestation" || kernel.lastRequest.Resource.Action != "verify" {
		t.Fatalf("security resource = %#v, want owner attestation verify", kernel.lastRequest.Resource)
	}
}

func TestUpdateOrganizationModulesRequiresSecurityKernelAuthorization(t *testing.T) {
	userID := uuid.New()
	orgID := uuid.New()
	kernel := &fakeSecurityKernel{decision: securitykernel.Decision{Allowed: false, Reason: "license_denied", DecisionType: "deny"}, err: securitykernel.ErrDenied}
	repo := &fakeRepository{membership: &membershipRecord{ID: uuid.New(), OrganizationID: orgID, AuthorityTier: AuthorityOwner}}
	svc := NewService(repo, ModeSaaS, WithSecurityKernel(kernel))

	_, err := svc.UpdateOrganizationModules(context.Background(), userID, orgID, UpdateOrganizationModulesInput{EnabledModules: []string{"assistant"}})

	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("UpdateOrganizationModules error = %v, want ErrForbidden", err)
	}
	if repo.updatedModules {
		t.Fatalf("modules updated after security denial")
	}
	if kernel.lastRequest.Resource.ResourceType != "module_entitlement" || kernel.lastRequest.Resource.Action != "update" {
		t.Fatalf("security resource = %#v, want module entitlement update", kernel.lastRequest.Resource)
	}
}

func TestUpdateOrganizationModulesRequiresIndustryPolicy(t *testing.T) {
	userID := uuid.New()
	orgID := uuid.New()
	repo := &fakeRepository{membership: &membershipRecord{ID: uuid.New(), OrganizationID: orgID, AuthorityTier: AuthorityOwner}}
	policy := &fakeIndustryPolicy{err: ErrValidation}
	svc := NewService(repo, ModeSaaS, WithIndustryPolicy(policy))

	_, err := svc.UpdateOrganizationModules(context.Background(), userID, orgID, UpdateOrganizationModulesInput{EnabledModules: []string{"finance"}})

	if !errors.Is(err, ErrValidation) {
		t.Fatalf("UpdateOrganizationModules error = %v, want ErrValidation", err)
	}
	if repo.updatedModules {
		t.Fatalf("modules updated after industry policy denial")
	}
	if len(policy.modules) != 1 || policy.modules[0] != "finance" {
		t.Fatalf("industry policy modules = %#v, want [finance]", policy.modules)
	}
}

func TestCloseOrganizationRequiresPlatformAdmin(t *testing.T) {
	actorID := uuid.New()
	orgID := uuid.New()
	repo := &fakeRepository{platformRole: ""}
	svc := NewService(repo, ModeSaaS)

	_, err := svc.CloseOrganization(context.Background(), actorID, orgID, CloseOrganizationInput{Reason: "expired contract"})

	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("CloseOrganization error = %v, want ErrForbidden", err)
	}
	if repo.closedOrganization {
		t.Fatalf("organization closed without platform permission")
	}
}

func TestCloseOrganizationMarksOrganizationClosed(t *testing.T) {
	actorID := uuid.New()
	orgID := uuid.New()
	repo := &fakeRepository{platformRole: "owner"}
	svc := NewService(repo, ModeSaaS)

	closed, err := svc.CloseOrganization(context.Background(), actorID, orgID, CloseOrganizationInput{Reason: "expired contract"})

	if err != nil {
		t.Fatalf("CloseOrganization error = %v", err)
	}
	if !repo.closedOrganization {
		t.Fatalf("CloseOrganization did not call repository")
	}
	if closed.Status != OrganizationStatusClosed {
		t.Fatalf("closed status = %q, want %q", closed.Status, OrganizationStatusClosed)
	}
	if repo.closeReason != "expired contract" {
		t.Fatalf("close reason = %q, want expired contract", repo.closeReason)
	}
}

func TestResolveTenantRejectsClosedOrganization(t *testing.T) {
	userID := uuid.New()
	orgID := uuid.New()
	repo := &fakeRepository{
		profile: &UserProfile{
			ID:                    userID,
			OnboardingStatus:      OnboardingComplete,
			DefaultOrganizationID: &orgID,
		},
		membership:   &membershipRecord{ID: uuid.New(), OrganizationID: orgID, AuthorityTier: AuthorityOwner},
		organization: &OrganizationAccount{ID: orgID, Status: OrganizationStatusClosed},
	}
	svc := NewService(repo, ModeSaaS)

	_, err := svc.ResolveTenant(context.Background(), middleware.AuthenticatedUser{ID: userID.String(), Type: "human"}, orgID.String())

	if !errors.Is(err, middleware.ErrTenantForbidden) {
		t.Fatalf("ResolveTenant error = %v, want ErrTenantForbidden for closed organization", err)
	}
}

func TestResolveTenantIncludesTenantDatabaseTarget(t *testing.T) {
	userID := uuid.New()
	orgID := uuid.New()
	repo := &fakeRepository{
		profile: &UserProfile{
			ID:                    userID,
			OnboardingStatus:      OnboardingComplete,
			DefaultOrganizationID: &orgID,
		},
		membership: &membershipRecord{ID: uuid.New(), OrganizationID: orgID, AuthorityTier: AuthorityOwner},
		organization: &OrganizationAccount{
			ID:     orgID,
			Status: OrganizationStatusActive,
		},
		tenantTarget: &tenantdb.Target{
			OrganizationID: orgID,
			DeploymentMode: tenantdb.DeploymentModeDedicatedDatabase,
			ClusterKey:     "local-primary",
			Region:         "local",
			DatabaseName:   "meta_org_tenant_" + strings.ReplaceAll(orgID.String(), "-", ""),
			SchemaName:     "public",
			Status:         tenantdb.TargetStatusProvisioned,
		},
	}
	svc := NewService(repo, ModeSaaS)

	tenant, err := svc.ResolveTenant(context.Background(), middleware.AuthenticatedUser{ID: userID.String(), Type: "human"}, orgID.String())

	if err != nil {
		t.Fatalf("ResolveTenant() error = %v", err)
	}
	if tenant.TenantDatabaseName != repo.tenantTarget.DatabaseName {
		t.Fatalf("TenantDatabaseName = %q, want %q", tenant.TenantDatabaseName, repo.tenantTarget.DatabaseName)
	}
	if tenant.TenantDatabaseDeploymentMode != tenantdb.DeploymentModeDedicatedDatabase {
		t.Fatalf("TenantDatabaseDeploymentMode = %q", tenant.TenantDatabaseDeploymentMode)
	}
	if tenant.TenantDatabaseClusterKey != "local-primary" {
		t.Fatalf("TenantDatabaseClusterKey = %q", tenant.TenantDatabaseClusterKey)
	}
	if tenant.TenantSchemaName != "public" {
		t.Fatalf("TenantSchemaName = %q, want public", tenant.TenantSchemaName)
	}
}

type fakeRepository struct {
	completedOnboarding bool
	updatedModules      bool
	closedOrganization  bool
	closeReason         string
	profile             *UserProfile
	membership          *membershipRecord
	organization        *OrganizationAccount
	platformRole        string
	tenantTarget        *tenantdb.Target
}

func (f *fakeRepository) BootstrapPlatformAdmin(context.Context, string, string) error {
	return nil
}

func (f *fakeRepository) GetUserProfile(context.Context, uuid.UUID) (*UserProfile, error) {
	if f.profile != nil {
		return f.profile, nil
	}
	return &UserProfile{ID: uuid.New(), OnboardingStatus: OnboardingComplete, EnabledModules: map[string]bool{"assistant": true}}, nil
}

func (f *fakeRepository) ListModules(context.Context) ([]Module, error) {
	return []Module{}, nil
}

func (f *fakeRepository) ListDefaultModuleKeys(context.Context) ([]string, error) {
	return []string{"organization", "assistant"}, nil
}

func (f *fakeRepository) CompleteOnboarding(context.Context, uuid.UUID, OnboardingOrganizationInput, []string) (*OrganizationAccount, error) {
	f.completedOnboarding = true
	return &OrganizationAccount{ID: uuid.New(), AuthorityTier: AuthorityOwner, IsOwner: true}, nil
}

func (f *fakeRepository) ListOrganizationsForPlatform(context.Context, int) ([]OrganizationAccount, error) {
	return []OrganizationAccount{}, nil
}

func (f *fakeRepository) CloseOrganization(_ context.Context, orgID uuid.UUID, actorID uuid.UUID, reason string) (*OrganizationAccount, error) {
	f.closedOrganization = true
	f.closeReason = reason
	return &OrganizationAccount{ID: orgID, Status: OrganizationStatusClosed}, nil
}

func (f *fakeRepository) GetSubscription(context.Context, uuid.UUID) (*OrganizationSubscription, error) {
	return &OrganizationSubscription{}, nil
}

func (f *fakeRepository) ListEnabledModules(context.Context, uuid.UUID) (map[string]bool, error) {
	return map[string]bool{"assistant": true}, nil
}

func (f *fakeRepository) UpdateOrganizationModules(context.Context, uuid.UUID, []string) (map[string]bool, error) {
	f.updatedModules = true
	return map[string]bool{"assistant": true}, nil
}

func (f *fakeRepository) CreateInvitation(context.Context, uuid.UUID, uuid.UUID, CreateInvitationInput, string) (*Invitation, error) {
	return &Invitation{}, nil
}

func (f *fakeRepository) ListInvitations(context.Context, uuid.UUID, int) ([]Invitation, error) {
	return []Invitation{}, nil
}

func (f *fakeRepository) AcceptInvitationWithNewUser(context.Context, string, AcceptInvitationInput, string) (*OrganizationAccount, uuid.UUID, error) {
	return &OrganizationAccount{}, uuid.New(), nil
}

func (f *fakeRepository) EnsureSingleOrgForUser(context.Context, uuid.UUID) (*OrganizationAccount, error) {
	return &OrganizationAccount{ID: uuid.New()}, nil
}

func (f *fakeRepository) GetHumanMembership(context.Context, uuid.UUID, uuid.UUID) (*membershipRecord, error) {
	if f.membership != nil {
		return f.membership, nil
	}
	return nil, ErrForbidden
}

func (f *fakeRepository) GetAgentMembership(context.Context, uuid.UUID, uuid.UUID) (*membershipRecord, error) {
	return &membershipRecord{ID: uuid.New(), OrganizationID: uuid.New(), AuthorityTier: "executor"}, nil
}

func (f *fakeRepository) GetOrganizationAccount(context.Context, uuid.UUID) (*OrganizationAccount, error) {
	if f.organization != nil {
		return f.organization, nil
	}
	return &OrganizationAccount{ID: uuid.New()}, nil
}

func (f *fakeRepository) GetPlatformRole(context.Context, uuid.UUID) (string, error) {
	if f.platformRole == "" {
		return "", ErrForbidden
	}
	return f.platformRole, nil
}

func (f *fakeRepository) GetTenantDatabaseTarget(context.Context, uuid.UUID) (*tenantdb.Target, error) {
	if f.tenantTarget == nil {
		return nil, ErrValidation
	}
	return f.tenantTarget, nil
}

type fakeSecurityKernel struct {
	lastRequest securitykernel.Request
	decision    securitykernel.Decision
	err         error
}

func (f *fakeSecurityKernel) Authorize(_ context.Context, request securitykernel.Request) (securitykernel.Decision, error) {
	f.lastRequest = request
	return f.decision, f.err
}

type fakeIndustryPolicy struct {
	modules []string
	err     error
}

func (f *fakeIndustryPolicy) ValidateOrganizationModules(_ context.Context, _ uuid.UUID, modules []string) error {
	f.modules = modules
	return f.err
}
