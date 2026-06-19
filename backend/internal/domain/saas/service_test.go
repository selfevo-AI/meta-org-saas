package saas

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org/backend/internal/pkg/securitykernel"
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

type fakeRepository struct {
	completedOnboarding bool
	updatedModules      bool
	profile             *UserProfile
	membership          *membershipRecord
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
	return &OrganizationAccount{ID: uuid.New()}, nil
}

func (f *fakeRepository) GetPlatformRole(context.Context, uuid.UUID) (string, error) {
	return "", ErrForbidden
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
