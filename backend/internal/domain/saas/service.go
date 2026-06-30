package saas

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/identity"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/passwordhash"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/platformauth"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/securitykernel"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/tenantdb"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrValidation = errors.New("validation error")
	ErrForbidden  = errors.New("forbidden")
	ErrConflict   = errors.New("conflict")
)

type Service struct {
	repo                       repository
	mode                       string
	securityKernel             securitykernel.Client
	industryPolicy             industryPolicy
	tenantDatabaseProvisioner  tenantDatabaseProvisioner
	tenantDatabaseBootstrapper tenantDatabaseBootstrapper
}

type ServiceOption func(*Service)

type industryPolicy interface {
	ValidateOrganizationModules(context.Context, uuid.UUID, []string) error
}

type repository interface {
	BootstrapPlatformAdmin(context.Context, string, string) error
	GetUserProfile(context.Context, uuid.UUID) (*UserProfile, error)
	ListModules(context.Context) ([]Module, error)
	ListDefaultModuleKeys(context.Context) ([]string, error)
	CompleteOnboarding(context.Context, uuid.UUID, OnboardingOrganizationInput, []string) (*OrganizationAccount, error)
	ListOrganizationsForPlatform(context.Context, int) ([]OrganizationAccount, error)
	CloseOrganization(context.Context, uuid.UUID, uuid.UUID, string) (*OrganizationAccount, error)
	GetSubscription(context.Context, uuid.UUID) (*OrganizationSubscription, error)
	ListEnabledModules(context.Context, uuid.UUID) (map[string]bool, error)
	UpdateOrganizationModules(context.Context, uuid.UUID, []string) (map[string]bool, error)
	ListOrganizationAccounts(context.Context, uuid.UUID, int) ([]OrganizationUserAccount, error)
	UpdateOrganizationAccount(context.Context, UpdateOrganizationAccountRecord) (*OrganizationUserAccount, error)
	ResetOrganizationAccountPassword(context.Context, uuid.UUID, uuid.UUID, string, uuid.UUID) (*OrganizationUserAccount, error)
	CreateInvitation(context.Context, uuid.UUID, uuid.UUID, CreateInvitationInput, string) (*Invitation, error)
	ListInvitations(context.Context, uuid.UUID, int) ([]Invitation, error)
	AcceptInvitationWithNewUser(context.Context, string, AcceptInvitationInput, string) (*OrganizationAccount, uuid.UUID, error)
	EnsureSingleOrgForUser(context.Context, uuid.UUID) (*OrganizationAccount, error)
	GetHumanMembership(context.Context, uuid.UUID, uuid.UUID) (*membershipRecord, error)
	GetAgentMembership(context.Context, uuid.UUID, uuid.UUID) (*membershipRecord, error)
	GetOrganizationAccount(context.Context, uuid.UUID) (*OrganizationAccount, error)
	GetPlatformRole(context.Context, uuid.UUID) (string, error)
}

type tenantDatabaseTargetProvider interface {
	GetTenantDatabaseTarget(context.Context, uuid.UUID) (*tenantdb.Target, error)
}

type tenantDatabaseTargetUpdater interface {
	UpdateTenantDatabaseTarget(context.Context, tenantdb.Target) error
}

type tenantDatabaseMigrationRecorder interface {
	RecordTenantDatabaseMigrationResult(context.Context, tenantdb.Target, tenantdb.MigrationResult) error
}

type sampleTenantCreator interface {
	CreateBusinessClosureSampleTenant(context.Context, CreateSampleTenantRecord) (*CreatedSampleTenant, error)
}

type tenantDatabaseProvisioner interface {
	Provision(context.Context, tenantdb.Target) (tenantdb.ProvisionResult, error)
}

type tenantDatabaseBootstrapper interface {
	BootstrapTenant(context.Context, tenantdb.Target, tenantdb.TenantBootstrapInput) error
}

func WithSecurityKernel(client securitykernel.Client) ServiceOption {
	return func(s *Service) {
		s.securityKernel = client
	}
}

func WithIndustryPolicy(policy industryPolicy) ServiceOption {
	return func(s *Service) {
		s.industryPolicy = policy
	}
}

func WithTenantDatabaseProvisioner(provisioner tenantDatabaseProvisioner) ServiceOption {
	return func(s *Service) {
		s.tenantDatabaseProvisioner = provisioner
	}
}

func WithTenantDatabaseBootstrapper(bootstrapper tenantDatabaseBootstrapper) ServiceOption {
	return func(s *Service) {
		s.tenantDatabaseBootstrapper = bootstrapper
	}
}

func NewService(repo repository, mode string, opts ...ServiceOption) *Service {
	if mode != ModeSaaS {
		mode = ModeSingleOrg
	}
	s := &Service{repo: repo, mode: mode}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Service) Mode() string {
	return s.mode
}

func (s *Service) BootstrapPlatformAdmin(ctx context.Context, email string, passwordHash string) error {
	if s.mode != ModeSaaS {
		return nil
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || strings.TrimSpace(passwordHash) == "" {
		return fmt.Errorf("%w: META_ORG_PLATFORM_ADMIN_EMAIL and META_ORG_PLATFORM_ADMIN_PASSWORD_HASH are required in saas mode", ErrValidation)
	}
	if _, err := bcrypt.Cost([]byte(passwordHash)); err != nil {
		return fmt.Errorf("%w: platform admin password hash must be a bcrypt hash", ErrValidation)
	}
	return s.repo.BootstrapPlatformAdmin(ctx, email, passwordHash)
}

func (s *Service) GetProfile(ctx context.Context, userID uuid.UUID) (*UserProfile, error) {
	profile, err := s.repo.GetUserProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	profile.OnboardingRequired = s.mode == ModeSaaS && profile.PlatformRole == "" && profile.OnboardingStatus != OnboardingComplete
	return profile, nil
}

func (s *Service) IdentitySessionProfile(ctx context.Context, userID uuid.UUID) (*identity.SessionProfile, error) {
	profile, err := s.GetProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	orgs := make([]identity.AuthOrganization, 0, len(profile.Organizations))
	for _, org := range profile.Organizations {
		orgs = append(orgs, identity.AuthOrganization{
			ID:            org.ID,
			Name:          org.Name,
			Description:   org.Description,
			MembershipID:  org.MembershipID,
			AuthorityTier: org.AuthorityTier,
			IsOwner:       org.IsOwner,
		})
	}
	return &identity.SessionProfile{
		OnboardingRequired:    profile.OnboardingRequired,
		DefaultOrganizationID: profile.DefaultOrganizationID,
		PlatformRole:          profile.PlatformRole,
		Organizations:         orgs,
		EnabledModules:        profile.EnabledModules,
	}, nil
}

func (s *Service) ListModules(ctx context.Context) ([]Module, error) {
	return s.repo.ListModules(ctx)
}

func (s *Service) CompleteOnboarding(ctx context.Context, userID uuid.UUID, input OnboardingOrganizationInput) (*OnboardingOrganizationResponse, error) {
	input.OrganizationName = strings.TrimSpace(input.OrganizationName)
	if input.OrganizationName == "" {
		return nil, fmt.Errorf("%w: organization_name is required", ErrValidation)
	}
	modules := normalizeModuleKeys(input.EnabledModules)
	if len(modules) == 0 {
		defaults, err := s.repo.ListDefaultModuleKeys(ctx)
		if err != nil {
			return nil, err
		}
		modules = defaults
	}
	if err := s.authorizeWithKernel(ctx, securitykernel.Request{
		Actor: securitykernel.Actor{
			ActorID:       userID,
			ActorType:     "human",
			AuthorityTier: AuthorityOwner,
		},
		EnabledModules:  modules,
		EnabledFeatures: []string{"owner_attestation"},
		Resource: securitykernel.Resource{
			ModuleKey:             "general",
			ResourceType:          "owner_attestation",
			Action:                "verify",
			ScopeLevel:            "organization",
			RequiredAuthorityTier: AuthorityOwner,
			RequiredLicenseMode:   "commercial",
		},
		Metadata: map[string]any{"organization_name": input.OrganizationName},
	}); err != nil {
		return nil, err
	}
	org, err := s.repo.CompleteOnboarding(ctx, userID, input, modules)
	if err != nil {
		return nil, err
	}
	ownerProfile, _ := s.repo.GetUserProfile(ctx, userID)
	s.provisionTenantDatabase(ctx, org.ID, tenantdb.TenantBootstrapInput{
		OrganizationID:   org.ID,
		OrganizationName: org.Name,
		Description:      org.Description,
		OwnerUserID:      userID,
		OwnerName:        ownerProfileName(ownerProfile),
		OwnerEmail:       ownerProfileEmail(ownerProfile),
		EnabledModules:   modules,
	})
	profile, err := s.GetProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &OnboardingOrganizationResponse{Profile: *profile, Organization: *org}, nil
}

func (s *Service) CreateBusinessClosureSampleTenant(ctx context.Context, actorID uuid.UUID) (*SampleTenantResponse, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionOrganizationManage); err != nil {
		return nil, err
	}
	creator, ok := s.repo.(sampleTenantCreator)
	if !ok {
		return nil, fmt.Errorf("%w: sample tenant creator is not configured", ErrValidation)
	}
	modules, err := s.allModuleKeys(ctx)
	if err != nil {
		return nil, err
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("MetaOrgSampleTenant!2026"), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	created, err := creator.CreateBusinessClosureSampleTenant(ctx, CreateSampleTenantRecord{
		ActorID:          actorID,
		OwnerEmail:       "demo@local.com",
		OwnerName:        "ERPNext Manufacturing Demo Owner",
		PasswordHash:     string(passwordHash),
		OrganizationName: "ERPNext Manufacturing Demo",
		Description:      "Demo tenant for validating the ERPNext manufacturing industry solution loop.",
		EnabledModules:   modules,
		SampleKey:        "erpnext_manufacturing_demo",
	})
	if err != nil {
		return nil, err
	}
	s.provisionTenantDatabase(ctx, created.Organization.ID, tenantdb.TenantBootstrapInput{
		OrganizationID:               created.Organization.ID,
		OrganizationName:             created.Organization.Name,
		Description:                  created.Organization.Description,
		OwnerUserID:                  created.OwnerUserID,
		OwnerName:                    "ERPNext Manufacturing Demo Owner",
		OwnerEmail:                   created.OwnerEmail,
		EnabledModules:               modules,
		SampleKey:                    "erpnext_manufacturing_demo",
		IncludeBusinessClosureSample: true,
	})
	target := s.resolveTenantDatabaseTarget(ctx, created.Organization.ID)
	return &SampleTenantResponse{
		Organization:            created.Organization,
		OwnerUserID:             created.OwnerUserID,
		OwnerEmail:              created.OwnerEmail,
		EnabledModules:          modules,
		TenantDatabaseStatus:    target.Status,
		TenantDatabaseName:      target.DatabaseName,
		IndustrySolutionPackage: "erpnext_manufacturing_demo",
	}, nil
}

func (s *Service) ListPlatformOrganizations(ctx context.Context, actorID uuid.UUID, limit int) ([]OrganizationAccount, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionPlatformRead); err != nil {
		return nil, err
	}
	return s.repo.ListOrganizationsForPlatform(ctx, limit)
}

func (s *Service) CloseOrganization(ctx context.Context, actorID uuid.UUID, orgID uuid.UUID, input CloseOrganizationInput) (*OrganizationAccount, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionOrganizationClose); err != nil {
		return nil, err
	}
	if orgID == uuid.Nil {
		return nil, fmt.Errorf("%w: organization_id is required", ErrValidation)
	}
	reason := strings.TrimSpace(input.Reason)
	return s.repo.CloseOrganization(ctx, orgID, actorID, reason)
}

func (s *Service) GetSubscription(ctx context.Context, actorID uuid.UUID, orgID uuid.UUID) (*OrganizationSubscription, error) {
	if err := s.requireOrgAdmin(ctx, actorID, orgID); err != nil {
		return nil, err
	}
	return s.repo.GetSubscription(ctx, orgID)
}

func (s *Service) GetEntitlements(ctx context.Context, actorID uuid.UUID, orgID uuid.UUID) (map[string]bool, error) {
	if err := s.requireOrgAdmin(ctx, actorID, orgID); err != nil {
		return nil, err
	}
	return s.repo.ListEnabledModules(ctx, orgID)
}

func (s *Service) UpdateOrganizationModules(ctx context.Context, actorID uuid.UUID, orgID uuid.UUID, input UpdateOrganizationModulesInput) (map[string]bool, error) {
	if err := s.requireOrgAdmin(ctx, actorID, orgID); err != nil {
		return nil, err
	}
	modules := normalizeModuleKeys(input.EnabledModules)
	if s.industryPolicy != nil {
		if err := s.industryPolicy.ValidateOrganizationModules(ctx, orgID, modules); err != nil {
			return nil, fmt.Errorf("%w: industry module policy denied update: %v", ErrValidation, err)
		}
	}
	authorityTier, isPlatformAdmin := s.actorAuthority(ctx, actorID, orgID)
	if err := s.authorizeWithKernel(ctx, securitykernel.Request{
		Actor: securitykernel.Actor{
			ActorID:         actorID,
			ActorType:       "human",
			AuthorityTier:   authorityTier,
			IsPlatformAdmin: isPlatformAdmin,
		},
		OrganizationID:  &orgID,
		EnabledModules:  modules,
		EnabledFeatures: []string{"module_entitlements"},
		Resource: securitykernel.Resource{
			ModuleKey:             "organization",
			ResourceType:          "module_entitlement",
			Action:                "update",
			ScopeLevel:            "organization",
			OrganizationID:        &orgID,
			RequiredAuthorityTier: AuthorityAdmin,
			RequiredLicenseMode:   "commercial",
		},
	}); err != nil {
		return nil, err
	}
	return s.repo.UpdateOrganizationModules(ctx, orgID, modules)
}

func (s *Service) ListOrganizationAccounts(ctx context.Context, actorID uuid.UUID, orgID uuid.UUID, limit int) ([]OrganizationUserAccount, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionOrganizationManage); err != nil {
		return nil, err
	}
	if orgID == uuid.Nil {
		return nil, fmt.Errorf("%w: organization_id is required", ErrValidation)
	}
	return s.repo.ListOrganizationAccounts(ctx, orgID, normalizeAccountLimit(limit))
}

func (s *Service) UpdateOrganizationAccount(ctx context.Context, actorID uuid.UUID, orgID uuid.UUID, userID uuid.UUID, input UpdateOrganizationAccountInput) (*OrganizationUserAccount, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionOrganizationManage); err != nil {
		return nil, err
	}
	if orgID == uuid.Nil || userID == uuid.Nil {
		return nil, fmt.Errorf("%w: organization_id and user_id are required", ErrValidation)
	}
	name := strings.TrimSpace(input.Name)
	email := strings.ToLower(strings.TrimSpace(input.Email))
	accountStatus := normalizeAccountKey(input.AccountStatus)
	authorityTier := normalizeAccountKey(input.AuthorityTier)
	if accountStatus != "" && accountStatus != "active" && accountStatus != "disabled" {
		return nil, fmt.Errorf("%w: account_status must be active or disabled", ErrValidation)
	}
	if authorityTier != "" && !validAuthorityTier(authorityTier) {
		return nil, fmt.Errorf("%w: invalid authority_tier", ErrValidation)
	}
	if name == "" && email == "" && accountStatus == "" && authorityTier == "" {
		return nil, fmt.Errorf("%w: at least one account field is required", ErrValidation)
	}
	return s.repo.UpdateOrganizationAccount(ctx, UpdateOrganizationAccountRecord{
		OrganizationID: orgID,
		UserID:         userID,
		ActorID:        actorID,
		Name:           name,
		Email:          email,
		AccountStatus:  accountStatus,
		AuthorityTier:  authorityTier,
	})
}

func (s *Service) ResetOrganizationAccountPassword(ctx context.Context, actorID uuid.UUID, orgID uuid.UUID, userID uuid.UUID, input ResetOrganizationAccountPasswordInput) (*ResetOrganizationAccountPasswordResponse, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionOrganizationManage); err != nil {
		return nil, err
	}
	if orgID == uuid.Nil || userID == uuid.Nil {
		return nil, fmt.Errorf("%w: organization_id and user_id are required", ErrValidation)
	}
	temporaryPassword := strings.TrimSpace(input.Password)
	if temporaryPassword == "" {
		var err error
		temporaryPassword, err = newInviteToken()
		if err != nil {
			return nil, err
		}
	}
	hash, err := passwordhash.GenerateBcryptHash(temporaryPassword, 0)
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.ResetOrganizationAccountPassword(ctx, orgID, userID, hash, actorID); err != nil {
		return nil, err
	}
	return &ResetOrganizationAccountPasswordResponse{UserID: userID, TemporaryPassword: temporaryPassword}, nil
}

func (s *Service) CreateInvitation(ctx context.Context, actorID uuid.UUID, orgID uuid.UUID, input CreateInvitationInput) (*Invitation, error) {
	if err := s.requireOrgAdmin(ctx, actorID, orgID); err != nil {
		return nil, err
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Name = strings.TrimSpace(input.Name)
	if input.Email == "" {
		return nil, fmt.Errorf("%w: email is required", ErrValidation)
	}
	if input.AuthorityTier == "" {
		input.AuthorityTier = "executor"
	}
	if !validAuthorityTier(input.AuthorityTier) {
		return nil, fmt.Errorf("%w: invalid authority_tier", ErrValidation)
	}
	if input.ExpiresInDays <= 0 || input.ExpiresInDays > 90 {
		input.ExpiresInDays = 7
	}
	token, err := newInviteToken()
	if err != nil {
		return nil, err
	}
	return s.repo.CreateInvitation(ctx, orgID, actorID, input, token)
}

func (s *Service) ListInvitations(ctx context.Context, actorID uuid.UUID, orgID uuid.UUID, limit int) ([]Invitation, error) {
	if err := s.requireOrgAdmin(ctx, actorID, orgID); err != nil {
		return nil, err
	}
	return s.repo.ListInvitations(ctx, orgID, limit)
}

func (s *Service) AcceptInvitation(ctx context.Context, token string, input AcceptInvitationInput) (*AcceptInvitationResponse, error) {
	token = strings.TrimSpace(token)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Name = strings.TrimSpace(input.Name)
	if token == "" || input.Email == "" || input.Password == "" {
		return nil, fmt.Errorf("%w: token, email, and password are required", ErrValidation)
	}
	if input.Name == "" {
		input.Name = strings.Split(input.Email, "@")[0]
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash invited password: %w", err)
	}
	org, userID, err := s.repo.AcceptInvitationWithNewUser(ctx, token, input, string(hash))
	if err != nil {
		return nil, err
	}
	profile, err := s.GetProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &AcceptInvitationResponse{Profile: *profile, Organization: *org}, nil
}

func (s *Service) ResolveTenant(ctx context.Context, user middleware.AuthenticatedUser, requestedOrganizationID string) (*middleware.TenantContext, error) {
	actorID, err := uuid.Parse(user.ID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid actor id", ErrValidation)
	}

	var requested *uuid.UUID
	if strings.TrimSpace(requestedOrganizationID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(requestedOrganizationID))
		if err != nil {
			return nil, middleware.ErrTenantInvalid
		}
		requested = &parsed
	}

	if user.Type == "ai" {
		if requested == nil {
			return nil, middleware.ErrTenantRequired
		}
		org, err := s.repo.GetOrganizationAccount(ctx, *requested)
		if err != nil || org.Status == OrganizationStatusClosed {
			return nil, middleware.ErrTenantForbidden
		}
		membership, err := s.repo.GetAgentMembership(ctx, actorID, *requested)
		if err != nil {
			return nil, middleware.ErrTenantForbidden
		}
		enabled, _ := s.repo.ListEnabledModules(ctx, *requested)
		membershipID := membership.ID
		orgID := membership.OrganizationID
		tenant := &middleware.TenantContext{
			Mode:           s.mode,
			UserID:         actorID,
			OrganizationID: &orgID,
			MembershipID:   &membershipID,
			AuthorityTier:  membership.AuthorityTier,
			EnabledModules: enabled,
		}
		s.applyTenantDatabaseTarget(ctx, tenant, orgID)
		return tenant, nil
	}

	profile, err := s.GetProfile(ctx, actorID)
	if err != nil {
		return nil, err
	}
	if s.mode == ModeSaaS && profile.OnboardingRequired {
		return nil, middleware.ErrOnboardingRequired
	}

	isPlatform := profile.PlatformRole != ""
	var orgID *uuid.UUID
	if requested != nil {
		orgID = requested
	} else if profile.DefaultOrganizationID != nil {
		orgID = profile.DefaultOrganizationID
	}
	if orgID == nil && s.mode == ModeSingleOrg {
		org, err := s.repo.EnsureSingleOrgForUser(ctx, actorID)
		if err != nil {
			return nil, err
		}
		orgID = &org.ID
		profile, _ = s.GetProfile(ctx, actorID)
	}
	if orgID == nil {
		return nil, middleware.ErrTenantRequired
	}
	org, err := s.repo.GetOrganizationAccount(ctx, *orgID)
	if err != nil || org.Status == OrganizationStatusClosed {
		return nil, middleware.ErrTenantForbidden
	}

	var membershipID *uuid.UUID
	authorityTier := ""
	if isPlatform {
		// Platform users may resolve active organizations without membership.
	} else {
		membership, err := s.repo.GetHumanMembership(ctx, actorID, *orgID)
		if err != nil {
			return nil, middleware.ErrTenantForbidden
		}
		id := membership.ID
		membershipID = &id
		authorityTier = membership.AuthorityTier
	}
	enabled, _ := s.repo.ListEnabledModules(ctx, *orgID)
	tenant := &middleware.TenantContext{
		Mode:             s.mode,
		UserID:           actorID,
		OrganizationID:   orgID,
		IsPlatformAdmin:  isPlatform,
		PlatformRole:     profile.PlatformRole,
		MembershipID:     membershipID,
		AuthorityTier:    authorityTier,
		EnabledModules:   enabled,
		OnboardingStatus: profile.OnboardingStatus,
	}
	s.applyTenantDatabaseTarget(ctx, tenant, *orgID)
	return tenant, nil
}

func (s *Service) applyTenantDatabaseTarget(ctx context.Context, tenant *middleware.TenantContext, orgID uuid.UUID) {
	target := s.resolveTenantDatabaseTarget(ctx, orgID)
	tenant.TenantDatabaseDeploymentMode = target.DeploymentMode
	tenant.TenantDatabaseClusterKey = target.ClusterKey
	tenant.TenantDatabaseRegion = target.Region
	tenant.TenantDatabaseName = target.DatabaseName
	tenant.TenantDatabaseStatus = target.Status
	tenant.TenantSchemaName = target.SchemaName
}

func (s *Service) resolveTenantDatabaseTarget(ctx context.Context, orgID uuid.UUID) tenantdb.Target {
	if provider, ok := s.repo.(tenantDatabaseTargetProvider); ok {
		if target, err := provider.GetTenantDatabaseTarget(ctx, orgID); err == nil && target != nil {
			if target.SchemaName == "" {
				target.SchemaName = tenantdb.SchemaNameForOrganization(orgID)
			}
			return *target
		}
	}
	return tenantdb.Target{
		OrganizationID: orgID,
		DeploymentMode: tenantdb.DeploymentModeSharedSchema,
		ClusterKey:     "local-primary",
		Region:         "local",
		SchemaName:     tenantdb.SchemaNameForOrganization(orgID),
		Status:         tenantdb.TargetStatusProvisioned,
	}
}

func (s *Service) provisionTenantDatabase(ctx context.Context, orgID uuid.UUID, bootstrapInput tenantdb.TenantBootstrapInput) {
	if s.tenantDatabaseProvisioner == nil {
		return
	}
	provider, ok := s.repo.(tenantDatabaseTargetProvider)
	if !ok {
		return
	}
	updater, ok := s.repo.(tenantDatabaseTargetUpdater)
	if !ok {
		return
	}
	target, err := provider.GetTenantDatabaseTarget(ctx, orgID)
	if err != nil || target == nil || target.Status == tenantdb.TargetStatusArchived {
		return
	}
	result, err := s.tenantDatabaseProvisioner.Provision(ctx, *target)
	updated := result.Target
	if updated.OrganizationID == uuid.Nil {
		updated = *target
	}
	if err != nil && updated.Status == "" {
		updated.Status = tenantdb.TargetStatusFailed
		updated.Metadata = mergeTenantDatabaseMetadata(updated.Metadata, map[string]any{"error": err.Error()})
	}
	_ = updater.UpdateTenantDatabaseTarget(ctx, updated)
	if s.tenantDatabaseBootstrapper != nil && updated.Status == tenantdb.TargetStatusProvisioned {
		bootstrapInput.OrganizationID = orgID
		if err := s.tenantDatabaseBootstrapper.BootstrapTenant(ctx, updated, bootstrapInput); err != nil {
			updated.Status = tenantdb.TargetStatusFailed
			updated.Metadata = mergeTenantDatabaseMetadata(updated.Metadata, map[string]any{"bootstrap_error": err.Error()})
			_ = updater.UpdateTenantDatabaseTarget(ctx, updated)
		}
	}
	if recorder, ok := s.repo.(tenantDatabaseMigrationRecorder); ok {
		_ = recorder.RecordTenantDatabaseMigrationResult(ctx, updated, result.Migration)
	}
}

func mergeTenantDatabaseMetadata(base map[string]any, extra map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range base {
		out[key] = value
	}
	for key, value := range extra {
		out[key] = value
	}
	return out
}

func (s *Service) requirePlatformAdmin(ctx context.Context, userID uuid.UUID) error {
	return s.requirePlatformPermission(ctx, userID, platformauth.PermissionPlatformRead)
}

func (s *Service) requirePlatformPermission(ctx context.Context, userID uuid.UUID, permission string) error {
	role, err := s.repo.GetPlatformRole(ctx, userID)
	if err != nil || !platformauth.HasPermission(role, permission) {
		return ErrForbidden
	}
	return nil
}

func (s *Service) requireOrgAdmin(ctx context.Context, userID uuid.UUID, orgID uuid.UUID) error {
	if role, err := s.repo.GetPlatformRole(ctx, userID); err == nil {
		if platformauth.HasPermission(role, platformauth.PermissionOrganizationManage) {
			return nil
		}
		return ErrForbidden
	}
	membership, err := s.repo.GetHumanMembership(ctx, userID, orgID)
	if err != nil {
		return ErrForbidden
	}
	switch membership.AuthorityTier {
	case AuthorityOwner, AuthorityAdmin:
		return nil
	default:
		return ErrForbidden
	}
}

func (s *Service) authorizeWithKernel(ctx context.Context, request securitykernel.Request) error {
	if s.securityKernel == nil {
		return nil
	}
	decision, err := s.securityKernel.Authorize(ctx, request)
	if err != nil || !decision.Allowed {
		reason := decision.Reason
		if reason == "" && err != nil {
			reason = err.Error()
		}
		return fmt.Errorf("%w: security kernel denied %s %s: %s", ErrForbidden, request.Resource.ResourceType, request.Resource.Action, reason)
	}
	return nil
}

func (s *Service) actorAuthority(ctx context.Context, userID uuid.UUID, orgID uuid.UUID) (string, bool) {
	if _, err := s.repo.GetPlatformRole(ctx, userID); err == nil {
		return "", true
	}
	membership, err := s.repo.GetHumanMembership(ctx, userID, orgID)
	if err != nil || membership == nil {
		return "", false
	}
	return membership.AuthorityTier, false
}

func normalizeModuleKeys(values []string) []string {
	seen := map[string]struct{}{}
	result := []string{}
	for _, value := range values {
		key := strings.ToLower(strings.TrimSpace(value))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, key)
	}
	return result
}

func (s *Service) allModuleKeys(ctx context.Context) ([]string, error) {
	modules, err := s.repo.ListModules(ctx)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(modules)+9)
	for _, module := range modules {
		keys = append(keys, module.ModuleKey)
	}
	keys = append(keys, "organization", "project", "workflow", "finance", "costing", "erp", "inventory", "procurement", "sales", "manufacturing")
	return normalizeModuleKeys(keys), nil
}

func ownerProfileName(profile *UserProfile) string {
	if profile == nil || strings.TrimSpace(profile.Name) == "" {
		return "Tenant Owner"
	}
	return profile.Name
}

func ownerProfileEmail(profile *UserProfile) string {
	if profile == nil {
		return ""
	}
	return profile.Email
}

func validAuthorityTier(value string) bool {
	switch value {
	case "organization_creator", "organization_admin", "reviewer", "executor":
		return true
	default:
		return false
	}
}

func newInviteToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate invite token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func normalizeAccountLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 200 {
		return 200
	}
	return limit
}

func normalizeAccountKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
