package saas

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/platformauth"
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
	if kernel.lastRequest.Resource.ModuleKey != "general" {
		t.Fatalf("security module key = %q, want general for owner attestation feature gate", kernel.lastRequest.Resource.ModuleKey)
	}
	if !containsString(kernel.lastRequest.EnabledFeatures, "owner_attestation") {
		t.Fatalf("enabled features = %#v, want owner_attestation", kernel.lastRequest.EnabledFeatures)
	}
}

func TestCompleteOnboardingReturnsProvisioningTenantTargetWithoutInlineDatabaseWork(t *testing.T) {
	userID := uuid.New()
	orgID := uuid.New()
	target := tenantdb.NewDedicatedDatabaseTarget(orgID, "meta_org_", "local-primary", "local")
	repo := &fakeRepository{onboardingOrgID: orgID, tenantTarget: &target}
	svc := NewService(repo, ModeSaaS)

	_, err := svc.CompleteOnboarding(context.Background(), userID, OnboardingOrganizationInput{OrganizationName: "Acme"})

	if err != nil {
		t.Fatalf("CompleteOnboarding error = %v", err)
	}
	if target.Status != tenantdb.TargetStatusProvisioning {
		t.Fatalf("tenant target status = %q, want provisioning", target.Status)
	}
}

func TestCreateBusinessClosureSampleTenantEnablesAllModulesAndReturnsProvisioningTarget(t *testing.T) {
	actorID := uuid.New()
	orgID := uuid.New()
	target := tenantdb.NewDedicatedDatabaseTarget(orgID, "meta_org_", "local-primary", "local")
	repo := &fakeRepository{
		platformRole: "owner",
		tenantTarget: &target,
		sampleTenant: &CreatedSampleTenant{
			Organization: OrganizationAccount{ID: orgID, Name: "ERPNext Manufacturing Demo"},
			OwnerUserID:  uuid.New(),
			OwnerEmail:   "demo@local.com",
		},
		modules: []Module{
			{ModuleKey: "organization"},
			{ModuleKey: "project"},
			{ModuleKey: "workflow"},
			{ModuleKey: "finance"},
			{ModuleKey: "costing"},
			{ModuleKey: "erp"},
			{ModuleKey: "inventory"},
			{ModuleKey: "procurement"},
			{ModuleKey: "sales"},
			{ModuleKey: "manufacturing"},
		},
	}
	svc := NewService(repo, ModeSaaS)

	result, err := svc.CreateBusinessClosureSampleTenant(context.Background(), actorID)
	if err != nil {
		t.Fatalf("CreateBusinessClosureSampleTenant error = %v", err)
	}

	if result.Organization.ID != orgID {
		t.Fatalf("sample org = %s, want %s", result.Organization.ID, orgID)
	}
	if result.OwnerEmail != "demo@local.com" {
		t.Fatalf("owner email = %q, want demo@local.com", result.OwnerEmail)
	}
	if repo.sampleRecord.OwnerEmail != "demo@local.com" || repo.sampleRecord.OrganizationName != "ERPNext Manufacturing Demo" {
		t.Fatalf("sample record = %#v, want ERPNext manufacturing demo owner/org", repo.sampleRecord)
	}
	for _, key := range []string{"organization", "project", "workflow", "finance", "costing", "erp", "inventory", "procurement", "sales", "manufacturing"} {
		if !containsString(repo.sampleRecord.EnabledModules, key) {
			t.Fatalf("sample enabled modules missing %q: %#v", key, repo.sampleRecord.EnabledModules)
		}
	}
	if result.IndustrySolutionPackage != "erpnext_manufacturing_demo" {
		t.Fatalf("IndustrySolutionPackage = %q, want erpnext_manufacturing_demo", result.IndustrySolutionPackage)
	}
	if result.TenantDatabaseStatus != tenantdb.TargetStatusProvisioning {
		t.Fatalf("tenant database status = %q, want provisioning", result.TenantDatabaseStatus)
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

func TestPlatformAdminCanUpdateTenantAccountAndResetPassword(t *testing.T) {
	actorID := uuid.New()
	orgID := uuid.New()
	targetUserID := uuid.New()
	repo := &fakeRepository{platformRole: "owner"}
	svc := NewService(repo, ModeSaaS)

	account, err := svc.UpdateOrganizationAccount(context.Background(), actorID, orgID, targetUserID, UpdateOrganizationAccountInput{
		Name:          "New Owner",
		Email:         "new-owner@example.test",
		AccountStatus: "active",
		AuthorityTier: AuthorityOwner,
	})
	if err != nil {
		t.Fatalf("UpdateOrganizationAccount error = %v", err)
	}
	if account.UserID != targetUserID || account.AuthorityTier != AuthorityOwner || !account.IsOwner {
		t.Fatalf("account = %#v, want promoted owner account", account)
	}
	if repo.updatedAccount == nil || repo.updatedAccount.Email != "new-owner@example.test" {
		t.Fatalf("updatedAccount = %#v, want repository update record", repo.updatedAccount)
	}

	reset, err := svc.ResetOrganizationAccountPassword(context.Background(), actorID, orgID, targetUserID, ResetOrganizationAccountPasswordInput{})
	if err != nil {
		t.Fatalf("ResetOrganizationAccountPassword error = %v", err)
	}
	if reset.TemporaryPassword == "" || reset.UserID != targetUserID {
		t.Fatalf("reset = %#v, want temporary password for target user", reset)
	}
	if repo.resetPasswordHash == "" || repo.resetPasswordHash == reset.TemporaryPassword {
		t.Fatalf("resetPasswordHash = %q, want bcrypt hash", repo.resetPasswordHash)
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

func TestResolveTenantRejectsInvalidRequestedOrganizationID(t *testing.T) {
	userID := uuid.New()
	repo := &fakeRepository{
		profile: &UserProfile{
			ID:               userID,
			OnboardingStatus: OnboardingComplete,
		},
	}
	svc := NewService(repo, ModeSaaS)

	_, err := svc.ResolveTenant(context.Background(), middleware.AuthenticatedUser{ID: userID.String(), Type: "human"}, "null")

	if !errors.Is(err, middleware.ErrTenantInvalid) {
		t.Fatalf("ResolveTenant error = %v, want ErrTenantInvalid", err)
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
			DatabaseName:   "meta_org_" + strings.ReplaceAll(orgID.String(), "-", "")[:4],
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

func TestTenantMigrationRecordsFromResultIncludesAppliedAndSkippedFiles(t *testing.T) {
	target := tenantdb.Target{Status: tenantdb.TargetStatusProvisioned}
	result := tenantdb.MigrationResult{
		Version: "002_finance_extension",
		AppliedStages: []tenantdb.MigrationStage{
			{Name: "002_finance_extension", Scope: tenantdb.MigrationScopeTenantBusiness},
		},
		Metadata: map[string]any{
			"migration_files_applied": []string{"002_finance_extension.sql"},
			"migration_files_skipped": []string{"001_tenant_business_baseline.sql"},
			"migration_checksums": map[string]string{
				"001_tenant_business_baseline.sql": "checksum-001",
				"002_finance_extension.sql":        "checksum-002",
			},
		},
	}

	records := tenantMigrationRecordsFromResult(target, result)

	if len(records) != 2 {
		t.Fatalf("len(records) = %d, want 2", len(records))
	}
	byKey := map[string]tenantMigrationRecord{}
	for _, record := range records {
		byKey[record.Key] = record
	}
	if byKey["002_finance_extension"].Status != "applied" {
		t.Fatalf("applied status = %q", byKey["002_finance_extension"].Status)
	}
	if byKey["002_finance_extension"].Checksum != "checksum-002" {
		t.Fatalf("applied checksum = %q", byKey["002_finance_extension"].Checksum)
	}
	if byKey["001_tenant_business_baseline"].Status != "skipped" {
		t.Fatalf("skipped status = %q", byKey["001_tenant_business_baseline"].Status)
	}
	if byKey["001_tenant_business_baseline"].Checksum != "checksum-001" {
		t.Fatalf("skipped checksum = %q", byKey["001_tenant_business_baseline"].Checksum)
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
	platformPermissions []string
	tenantTarget        *tenantdb.Target
	onboardingOrgID     uuid.UUID
	modules             []Module
	sampleTenant        *CreatedSampleTenant
	sampleRecord        CreateSampleTenantRecord
	updatedAccount      *UpdateOrganizationAccountRecord
	resetPasswordHash   string
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
	if f.modules != nil {
		return f.modules, nil
	}
	return []Module{}, nil
}

func (f *fakeRepository) ListDefaultModuleKeys(context.Context) ([]string, error) {
	return []string{"organization", "assistant"}, nil
}

func (f *fakeRepository) CompleteOnboarding(context.Context, uuid.UUID, OnboardingOrganizationInput, []string) (*OrganizationAccount, error) {
	f.completedOnboarding = true
	orgID := f.onboardingOrgID
	if orgID == uuid.Nil {
		orgID = uuid.New()
	}
	return &OrganizationAccount{ID: orgID, AuthorityTier: AuthorityOwner, IsOwner: true}, nil
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

func (f *fakeRepository) ListOrganizationAccounts(context.Context, uuid.UUID, int) ([]OrganizationUserAccount, error) {
	return []OrganizationUserAccount{}, nil
}

func (f *fakeRepository) UpdateOrganizationAccount(_ context.Context, record UpdateOrganizationAccountRecord) (*OrganizationUserAccount, error) {
	f.updatedAccount = &record
	return &OrganizationUserAccount{
		UserID:         record.UserID,
		OrganizationID: record.OrganizationID,
		Name:           record.Name,
		Email:          record.Email,
		AccountStatus:  record.AccountStatus,
		AuthorityTier:  record.AuthorityTier,
		IsOwner:        record.AuthorityTier == AuthorityOwner,
	}, nil
}

func (f *fakeRepository) ResetOrganizationAccountPassword(_ context.Context, orgID uuid.UUID, userID uuid.UUID, passwordHash string, _ uuid.UUID) (*OrganizationUserAccount, error) {
	f.resetPasswordHash = passwordHash
	return &OrganizationUserAccount{OrganizationID: orgID, UserID: userID, AccountStatus: "active"}, nil
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

func TestResolveTenantIncludesPlatformTenantPermissions(t *testing.T) {
	userID := uuid.New()
	orgID := uuid.New()
	repo := &fakeRepository{
		profile: &UserProfile{
			ID:                    userID,
			PlatformRole:          platformauth.RoleAuditor,
			OnboardingStatus:      OnboardingComplete,
			DefaultOrganizationID: &orgID,
		},
		platformRole:        platformauth.RoleAuditor,
		platformPermissions: []string{platformauth.PermissionPlatformRead, platformauth.PermissionTenantDataRead},
		organization:        &OrganizationAccount{ID: orgID, Status: OrganizationStatusActive},
	}
	svc := NewService(repo, ModeSaaS)

	tenant, err := svc.ResolveTenant(context.Background(), middleware.AuthenticatedUser{ID: userID.String(), Type: "human"}, orgID.String())

	if err != nil {
		t.Fatalf("ResolveTenant() error = %v", err)
	}
	if !tenant.PlatformPermissions[platformauth.PermissionTenantDataRead] {
		t.Fatalf("tenant read permission = false, want true")
	}
	if tenant.PlatformPermissions[platformauth.PermissionTenantDataManage] {
		t.Fatalf("tenant manage permission = true, want false")
	}
}

func (f *fakeRepository) ListPlatformRolePermissions(context.Context, string) ([]string, error) {
	if f.platformPermissions != nil {
		return f.platformPermissions, nil
	}
	permissions := platformauth.PermissionsForRole(f.platformRole)
	items := make([]string, 0, len(permissions))
	for permission, allowed := range permissions {
		if allowed {
			items = append(items, permission)
		}
	}
	return items, nil
}

func (f *fakeRepository) GetTenantDatabaseTarget(context.Context, uuid.UUID) (*tenantdb.Target, error) {
	if f.tenantTarget == nil {
		return nil, ErrValidation
	}
	return f.tenantTarget, nil
}

func (f *fakeRepository) CreateBusinessClosureSampleTenant(_ context.Context, record CreateSampleTenantRecord) (*CreatedSampleTenant, error) {
	f.sampleRecord = record
	if f.sampleTenant != nil {
		f.sampleTenant.Modules = record.EnabledModules
		return f.sampleTenant, nil
	}
	return &CreatedSampleTenant{
		Organization: OrganizationAccount{ID: uuid.New(), Name: record.OrganizationName},
		OwnerUserID:  uuid.New(),
		OwnerEmail:   record.OwnerEmail,
		Modules:      record.EnabledModules,
	}, nil
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

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
