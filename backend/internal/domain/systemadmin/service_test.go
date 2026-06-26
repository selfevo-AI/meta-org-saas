package systemadmin

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/platformauth"
	"golang.org/x/crypto/bcrypt"
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

func TestVerifySchemaChangeReportsChecksWithoutApplying(t *testing.T) {
	repo := &fakeRepository{
		role: "system_owner",
		request: &SchemaChangeRequest{
			ID:             uuid.New(),
			OrganizationID: uuid.New(),
			SchemaName:     "org_123e4567e89b12d3a456426614174000",
			Status:         SchemaChangeApproved,
			SchemaPackage:  DefaultOrganizationSchemaPackage(),
			RiskLevel:      SchemaRiskSafe,
		},
	}
	service := NewService(repo)

	report, err := service.VerifySchemaChange(context.Background(), uuid.New(), repo.request.ID)
	if err != nil {
		t.Fatalf("VerifySchemaChange() error = %v", err)
	}
	if report.ChangeRequestID != repo.request.ID {
		t.Fatalf("ChangeRequestID = %s, want %s", report.ChangeRequestID, repo.request.ID)
	}
	if report.Status != "passed" {
		t.Fatalf("Status = %q, want passed", report.Status)
	}
	if report.StatementCount == 0 {
		t.Fatal("StatementCount = 0, want generated DDL statements")
	}
	if report.BlockingIssues != 0 || !report.CanApply {
		t.Fatalf("report blocking/can_apply = %d/%v, want 0/true", report.BlockingIssues, report.CanApply)
	}
	if repo.applied {
		t.Fatal("VerifySchemaChange() applied schema change")
	}
}

func TestVerifySchemaChangeReportsIndustryFactoryCoverage(t *testing.T) {
	pkg := BuildERPSolutionSchemaPackage(ERPSolutionFlowRequest{
		IndustryKey:    "professional_services",
		PackageKey:     "erp_standard",
		Name:           "ERP Standard",
		EnabledModules: []string{"project", "procurement", "inventory", "sales", "finance"},
	})
	repo := &fakeRepository{
		role: "system_owner",
		request: &SchemaChangeRequest{
			ID:             uuid.New(),
			OrganizationID: uuid.New(),
			SchemaName:     "org_123e4567e89b12d3a456426614174000",
			RequestType:    "erp_solution_flow",
			Status:         SchemaChangeApproved,
			SchemaPackage:  pkg,
			RiskLevel:      SchemaRiskSafe,
		},
	}
	service := NewService(repo)

	report, err := service.VerifySchemaChange(context.Background(), uuid.New(), repo.request.ID)
	if err != nil {
		t.Fatalf("VerifySchemaChange() error = %v", err)
	}

	for _, key := range []string{
		"permissions_impact",
		"runtime_operations",
		"assistant_context",
		"tool_policy",
		"assistant_skills",
		"quality_gates",
		"verification_scenarios",
		"rollback_risk",
	} {
		check := verificationCheckByKey(report, key)
		if check == nil {
			t.Fatalf("missing verification check %s in %#v", key, report.Checks)
		}
		if check.Status != "passed" {
			t.Fatalf("check %s status = %q, want passed", key, check.Status)
		}
	}
	if report.Status != "passed" || report.BlockingIssues != 0 || !report.CanApply {
		t.Fatalf("report status/blocking/can_apply = %s/%d/%v, want passed/0/true", report.Status, report.BlockingIssues, report.CanApply)
	}
	if repo.applied {
		t.Fatal("VerifySchemaChange() applied schema change")
	}
}

func TestGetPermissionProfileUsesMetadataCatalog(t *testing.T) {
	repo := &fakeRepository{
		role: "platform_architect",
		rolePermissions: map[string][]string{
			"platform_architect": {
				platformauth.PermissionPlatformRead,
				platformauth.PermissionPlatformFeatureManage,
				platformauth.PermissionPlatformUserManage,
			},
		},
		menuItems: []PlatformMenuItem{
			{MenuKey: "platform.features", FeatureKey: "platform.feature.catalog", LabelKey: "systemAdmin.platformFeatures", RequiredPermissions: []string{platformauth.PermissionPlatformFeatureManage}, SortOrder: 20},
			{MenuKey: "platform.users", FeatureKey: "platform.user.management", LabelKey: "systemAdmin.platformUsers", RequiredPermissions: []string{platformauth.PermissionPlatformUserManage}, SortOrder: 30},
			{MenuKey: "platform.database", FeatureKey: "platform.database.maintenance", LabelKey: "systemAdmin.databaseMaintenance", RequiredPermissions: []string{platformauth.PermissionDatabaseMaintenanceManage}, SortOrder: 40},
		},
	}
	service := NewService(repo)

	profile, err := service.GetPermissionProfile(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("GetPermissionProfile() error = %v", err)
	}
	if profile.Role != "platform_architect" {
		t.Fatalf("Role = %q, want platform_architect", profile.Role)
	}
	if !profile.Permissions[platformauth.PermissionPlatformFeatureManage] || !profile.Permissions[platformauth.PermissionPlatformUserManage] {
		t.Fatalf("permissions = %#v, want DB-backed feature and user permissions", profile.Permissions)
	}
	if strings.Join(profile.MenuItems, ",") != "platform.features,platform.users" {
		t.Fatalf("MenuItems = %#v, want permitted metadata menu items only", profile.MenuItems)
	}
}

func TestCreatePlatformFeatureRequiresMetadataOnlyFeaturePermission(t *testing.T) {
	actorID := uuid.New()
	repo := &fakeRepository{
		role: "feature_admin",
		rolePermissions: map[string][]string{
			"feature_admin": {platformauth.PermissionPlatformRead, platformauth.PermissionPlatformFeatureManage},
		},
	}
	service := NewService(repo)

	feature, err := service.CreatePlatformFeature(context.Background(), actorID, CreatePlatformFeatureInput{
		FeatureKey:  "platform.future.audit",
		ModuleKey:   "platform_admin",
		Title:       "Future audit center",
		Description: "Metadata registered future platform capability",
		PermissionKeys: []string{
			platformauth.PermissionPlatformRead,
		},
		Metadata: map[string]any{"extension_mode": "metadata_only"},
	})
	if err != nil {
		t.Fatalf("CreatePlatformFeature() error = %v", err)
	}
	if feature.FeatureKey != "platform.future.audit" || feature.Status != "draft" {
		t.Fatalf("feature = %#v, want draft platform.future.audit", feature)
	}
	if repo.createdFeature == nil || repo.createdFeature.ActorID != actorID {
		t.Fatalf("createdFeature = %#v, want actor recorded", repo.createdFeature)
	}

	_, err = service.CreatePlatformFeature(context.Background(), actorID, CreatePlatformFeatureInput{
		FeatureKey: "platform.invalid",
		ModuleKey:  "platform_admin",
		Title:      "Invalid",
		Metadata:   map[string]any{"script": "drop all"},
	})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("CreatePlatformFeature() script metadata error = %v, want ErrValidation", err)
	}
}

func TestCreatePlatformUserReturnsTemporaryPasswordAndHashesIt(t *testing.T) {
	actorID := uuid.New()
	repo := &fakeRepository{
		role: "user_admin",
		rolePermissions: map[string][]string{
			"user_admin": {platformauth.PermissionPlatformRead, platformauth.PermissionPlatformUserManage},
		},
	}
	service := NewService(repo)

	result, err := service.CreatePlatformUser(context.Background(), actorID, CreatePlatformUserInput{
		Name:  "Platform Operator",
		Email: " Operator@Example.COM ",
		Roles: []string{"operator"},
	})
	if err != nil {
		t.Fatalf("CreatePlatformUser() error = %v", err)
	}
	if result.TemporaryPassword == "" {
		t.Fatal("TemporaryPassword is empty")
	}
	if result.User.Email != "operator@example.com" {
		t.Fatalf("Email = %q, want normalized operator@example.com", result.User.Email)
	}
	if repo.createdPlatformUser == nil {
		t.Fatal("CreatePlatformUser() did not call repository")
	}
	if repo.createdPlatformUser.PasswordHash == result.TemporaryPassword {
		t.Fatal("repository received plaintext temporary password")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(repo.createdPlatformUser.PasswordHash), []byte(result.TemporaryPassword)); err != nil {
		t.Fatalf("PasswordHash does not match temporary password: %v", err)
	}
	if strings.Join(repo.createdPlatformUser.Roles, ",") != "operator" {
		t.Fatalf("Roles = %#v, want operator", repo.createdPlatformUser.Roles)
	}
}

func TestDatabaseMaintenanceJobLifecycleRequiresSeparateApproval(t *testing.T) {
	actorID := uuid.New()
	reviewerID := uuid.New()
	repo := &fakeRepository{
		role: "maintenance_admin",
		rolePermissions: map[string][]string{
			"maintenance_admin": {
				platformauth.PermissionPlatformRead,
				platformauth.PermissionDatabaseMaintenanceManage,
				platformauth.PermissionDatabaseMaintenanceApprove,
			},
		},
	}
	service := NewService(repo)

	job, err := service.CreateDatabaseMaintenanceJob(context.Background(), actorID, CreateDatabaseMaintenanceJobInput{
		JobType: "backup",
		Scope:   "platform",
		Reason:  "nightly baseline backup",
	})
	if err != nil {
		t.Fatalf("CreateDatabaseMaintenanceJob() error = %v", err)
	}
	if job.Status != DatabaseMaintenancePendingApproval {
		t.Fatalf("Status = %q, want %q", job.Status, DatabaseMaintenancePendingApproval)
	}
	if repo.createdMaintenanceJob == nil || repo.createdMaintenanceJob.RequestedBy != actorID {
		t.Fatalf("createdMaintenanceJob = %#v, want requested_by actor", repo.createdMaintenanceJob)
	}

	approved, err := service.ReviewDatabaseMaintenanceJob(context.Background(), reviewerID, job.ID, ReviewDatabaseMaintenanceJobInput{
		Decision: "approve",
		Reason:   "maintenance window approved",
	})
	if err != nil {
		t.Fatalf("ReviewDatabaseMaintenanceJob() error = %v", err)
	}
	if approved.Status != DatabaseMaintenanceApproved {
		t.Fatalf("Status = %q, want %q", approved.Status, DatabaseMaintenanceApproved)
	}
	if repo.reviewedMaintenanceJob == nil || repo.reviewedMaintenanceJob.ReviewedBy != reviewerID {
		t.Fatalf("reviewedMaintenanceJob = %#v, want reviewer recorded", repo.reviewedMaintenanceJob)
	}
}

func TestCreateIndustrySolutionTableFieldChangeBuildsPhysicalSchemaPackage(t *testing.T) {
	actorID := uuid.New()
	orgID := uuid.New()
	repo := &fakeRepository{
		role: "solution_admin",
		rolePermissions: map[string][]string{
			"solution_admin": {
				platformauth.PermissionPlatformRead,
				platformauth.PermissionIndustrySolutionManage,
				platformauth.PermissionSchemaManage,
			},
		},
	}
	service := NewService(repo)

	request, err := service.CreateIndustrySolutionSchemaChange(context.Background(), actorID, CreateIndustrySolutionSchemaChangeInput{
		OrganizationID: orgID,
		IndustryKey:    "manufacturing",
		PackageKey:     "manufacturing-supply-chain",
		Table: IndustrySolutionTableInput{
			Name:        "tenant_quality_inspections",
			DisplayName: "Quality inspections",
			Fields: []IndustrySolutionFieldInput{
				{Name: "inspection_no", DataType: "varchar(64)", Nullable: false},
				{Name: "result_status", DataType: "varchar(40)", Nullable: false, Default: "'pending'"},
			},
		},
		Reason: "add quality inspection table for selected tenant",
	})
	if err != nil {
		t.Fatalf("CreateIndustrySolutionSchemaChange() error = %v", err)
	}
	if request.RequestType != "industry_solution_table_field_change" {
		t.Fatalf("RequestType = %q, want industry_solution_table_field_change", request.RequestType)
	}
	if repo.createdRecord == nil {
		t.Fatal("schema change record was not created")
	}
	if len(repo.createdRecord.SchemaPackage.Tables) != 1 {
		t.Fatalf("tables = %#v, want one table", repo.createdRecord.SchemaPackage.Tables)
	}
	table := repo.createdRecord.SchemaPackage.Tables[0]
	if table.Name != "tenant_quality_inspections" || len(table.Fields) != 3 {
		t.Fatalf("table = %#v, want physical table with id plus two fields", table)
	}
	if repo.createdRecord.SchemaPackage.Metadata["industry_key"] != "manufacturing" {
		t.Fatalf("metadata = %#v, want industry_key", repo.createdRecord.SchemaPackage.Metadata)
	}
}

func TestVerifySchemaChangeWarnsWhenIndustryFactoryCoverageIsIncomplete(t *testing.T) {
	pkg := BuildERPSolutionSchemaPackage(ERPSolutionFlowRequest{
		IndustryKey:    "professional_services",
		PackageKey:     "erp_standard",
		Name:           "ERP Standard",
		EnabledModules: []string{"project", "procurement"},
	})
	delete(pkg.Metadata, "verification_scenarios")
	repo := &fakeRepository{
		role: "system_owner",
		request: &SchemaChangeRequest{
			ID:             uuid.New(),
			OrganizationID: uuid.New(),
			SchemaName:     "org_123e4567e89b12d3a456426614174000",
			RequestType:    "erp_solution_flow",
			Status:         SchemaChangeApproved,
			SchemaPackage:  pkg,
			RiskLevel:      SchemaRiskSafe,
		},
	}
	service := NewService(repo)

	report, err := service.VerifySchemaChange(context.Background(), uuid.New(), repo.request.ID)
	if err != nil {
		t.Fatalf("VerifySchemaChange() error = %v", err)
	}

	check := verificationCheckByKey(report, "verification_scenarios")
	if check == nil {
		t.Fatalf("missing verification_scenarios check in %#v", report.Checks)
	}
	if check.Status != "warning" {
		t.Fatalf("verification_scenarios status = %q, want warning", check.Status)
	}
	if report.Status != "warning" || report.BlockingIssues != 0 || !report.CanApply {
		t.Fatalf("report status/blocking/can_apply = %s/%d/%v, want warning/0/true", report.Status, report.BlockingIssues, report.CanApply)
	}
	if repo.applied {
		t.Fatal("VerifySchemaChange() applied schema change")
	}
}

func TestVerifySchemaChangeBlocksApplyForDuplicateRuntimeOperations(t *testing.T) {
	pkg := BuildERPSolutionSchemaPackage(ERPSolutionFlowRequest{IndustryKey: "professional_services", PackageKey: "erp_standard", Name: "ERP Standard"})
	manifest, err := ManifestFromSchemaPackage(pkg)
	if err != nil {
		t.Fatalf("ManifestFromSchemaPackage error = %v", err)
	}
	manifest.Assets = append(manifest.Assets, IndustrySolutionAsset{
		AssetKey:  "runtime_operation.duplicate",
		AssetType: AssetTypeRuntimeOperation,
		Version:   "v1",
		RiskLevel: "medium",
		Payload:   map[string]any{"path": "/erp/catalog"},
	})
	setIndustryManifest(&pkg, manifest)

	repo := &fakeRepository{role: "system_owner", request: &SchemaChangeRequest{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		SchemaName:     "org_123e4567e89b12d3a456426614174000",
		RequestType:    "erp_solution_flow",
		Status:         SchemaChangeApproved,
		SchemaPackage:  pkg,
		RiskLevel:      SchemaRiskSafe,
	}}
	service := NewService(repo)

	report, err := service.VerifySchemaChange(context.Background(), uuid.New(), repo.request.ID)
	if err != nil {
		t.Fatalf("VerifySchemaChange error = %v", err)
	}
	check := verificationCheckByKey(report, "runtime_operations")
	if check == nil || check.Status != "failed" {
		t.Fatalf("runtime_operations check = %#v, want failed", check)
	}
	if report.CanApply || report.BlockingIssues == 0 {
		t.Fatalf("report can_apply/blocking = %v/%d, want blocked", report.CanApply, report.BlockingIssues)
	}
}

func TestVerifySchemaChangeBlocksActiveContextRules(t *testing.T) {
	pkg := BuildERPSolutionSchemaPackage(ERPSolutionFlowRequest{IndustryKey: "professional_services", PackageKey: "erp_standard", Name: "ERP Standard"})
	manifest, err := ManifestFromSchemaPackage(pkg)
	if err != nil {
		t.Fatalf("ManifestFromSchemaPackage error = %v", err)
	}
	for i := range manifest.Assets {
		if manifest.Assets[i].AssetType == AssetTypeContextRule {
			manifest.Assets[i].Payload["status"] = "active"
			break
		}
	}
	setIndustryManifest(&pkg, manifest)

	repo := &fakeRepository{role: "system_owner", request: &SchemaChangeRequest{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		SchemaName:     "org_123e4567e89b12d3a456426614174000",
		RequestType:    "erp_solution_flow",
		Status:         SchemaChangeApproved,
		SchemaPackage:  pkg,
		RiskLevel:      SchemaRiskSafe,
	}}
	service := NewService(repo)

	report, err := service.VerifySchemaChange(context.Background(), uuid.New(), repo.request.ID)
	if err != nil {
		t.Fatalf("VerifySchemaChange error = %v", err)
	}
	check := verificationCheckByKey(report, "assistant_context")
	if check == nil || check.Status != "failed" {
		t.Fatalf("assistant_context check = %#v, want failed", check)
	}
	if report.CanApply || report.BlockingIssues == 0 {
		t.Fatalf("report can_apply/blocking = %v/%d, want blocked", report.CanApply, report.BlockingIssues)
	}
}

func TestApplySchemaChangeRejectsManifestBlockingIssues(t *testing.T) {
	pkg := BuildERPSolutionSchemaPackage(ERPSolutionFlowRequest{IndustryKey: "professional_services", PackageKey: "erp_standard", Name: "ERP Standard"})
	manifest, err := ManifestFromSchemaPackage(pkg)
	if err != nil {
		t.Fatalf("ManifestFromSchemaPackage error = %v", err)
	}
	for i := range manifest.Assets {
		if manifest.Assets[i].AssetType == AssetTypeContextRule {
			manifest.Assets[i].Payload["status"] = "active"
			break
		}
	}
	setIndustryManifest(&pkg, manifest)

	repo := &fakeRepository{role: "system_owner", request: &SchemaChangeRequest{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		SchemaName:     "org_123e4567e89b12d3a456426614174000",
		RequestType:    "erp_solution_flow",
		Status:         SchemaChangeApproved,
		SchemaPackage:  pkg,
		RiskLevel:      SchemaRiskSafe,
	}}
	service := NewService(repo)

	_, err = service.ApplySchemaChange(context.Background(), uuid.New(), repo.request.ID)
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("ApplySchemaChange error = %v, want ErrValidation", err)
	}
	if repo.applied {
		t.Fatal("ApplySchemaChange applied request with blocking manifest issue")
	}
}

func verificationCheckByKey(report *SchemaVerificationReport, key string) *SchemaVerificationCheck {
	if report == nil {
		return nil
	}
	for i := range report.Checks {
		if report.Checks[i].Key == key {
			return &report.Checks[i]
		}
	}
	return nil
}

type fakeRepository struct {
	role                   string
	rolePermissions        map[string][]string
	menuItems              []PlatformMenuItem
	request                *SchemaChangeRequest
	createdRecord          *CreateSchemaChangeRequestRecord
	createdFeature         *CreatePlatformFeatureRecord
	createdPlatformUser    *CreatePlatformUserRecord
	createdMaintenanceJob  *CreateDatabaseMaintenanceJobRecord
	reviewedMaintenanceJob *ReviewDatabaseMaintenanceJobRecord
	applied                bool
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

func (f *fakeRepository) ListPlatformFeatures(context.Context, string, int) ([]PlatformFeature, error) {
	return nil, nil
}

func (f *fakeRepository) CreatePlatformFeature(_ context.Context, record CreatePlatformFeatureRecord) (*PlatformFeature, error) {
	f.createdFeature = &record
	return &PlatformFeature{
		FeatureKey:     record.FeatureKey,
		ParentKey:      record.ParentKey,
		ModuleKey:      record.ModuleKey,
		Category:       record.Category,
		Title:          record.Title,
		Description:    record.Description,
		Status:         record.Status,
		SortOrder:      record.SortOrder,
		PermissionKeys: record.PermissionKeys,
		Metadata:       record.Metadata,
	}, nil
}

func (f *fakeRepository) UpdatePlatformFeatureStatus(context.Context, string, string, uuid.UUID) (*PlatformFeature, error) {
	return nil, nil
}

func (f *fakeRepository) ListPlatformMenuItems(context.Context, int) ([]PlatformMenuItem, error) {
	return f.menuItems, nil
}

func (f *fakeRepository) ListPlatformPermissions(context.Context) ([]PlatformPermission, error) {
	return nil, nil
}

func (f *fakeRepository) ListPlatformRoles(context.Context) ([]PlatformRole, error) {
	return nil, nil
}

func (f *fakeRepository) ListPlatformRolePermissions(_ context.Context, roleKey string) ([]string, error) {
	if f.rolePermissions == nil {
		return nil, nil
	}
	return f.rolePermissions[roleKey], nil
}

func (f *fakeRepository) SetPlatformRolePermissions(context.Context, string, []string, uuid.UUID) (*PlatformRole, error) {
	return nil, nil
}

func (f *fakeRepository) ListPlatformUsers(context.Context, int) ([]PlatformUser, error) {
	return nil, nil
}

func (f *fakeRepository) CreatePlatformUser(_ context.Context, record CreatePlatformUserRecord) (*PlatformUser, error) {
	f.createdPlatformUser = &record
	return &PlatformUser{
		UserID:        uuid.New(),
		Name:          record.Name,
		Email:         record.Email,
		AccountStatus: "active",
		Roles:         record.Roles,
		Metadata:      record.Metadata,
	}, nil
}

func (f *fakeRepository) SetPlatformUserRoles(context.Context, uuid.UUID, []string, uuid.UUID) (*PlatformUser, error) {
	return nil, nil
}

func (f *fakeRepository) ResetPlatformUserPassword(context.Context, uuid.UUID, string, uuid.UUID) (*PlatformUser, error) {
	return nil, nil
}

func (f *fakeRepository) DisablePlatformUser(context.Context, uuid.UUID, uuid.UUID) (*PlatformUser, error) {
	return nil, nil
}

func (f *fakeRepository) ListDatabaseMaintenanceJobs(context.Context, int) ([]DatabaseMaintenanceJob, error) {
	return nil, nil
}

func (f *fakeRepository) CreateDatabaseMaintenanceJob(_ context.Context, record CreateDatabaseMaintenanceJobRecord) (*DatabaseMaintenanceJob, error) {
	f.createdMaintenanceJob = &record
	return &DatabaseMaintenanceJob{
		ID:          uuid.New(),
		JobType:     record.JobType,
		Scope:       record.Scope,
		Status:      record.Status,
		Reason:      record.Reason,
		BackupRef:   record.BackupRef,
		RequestedBy: &record.RequestedBy,
		Metadata:    record.Metadata,
	}, nil
}

func (f *fakeRepository) ReviewDatabaseMaintenanceJob(_ context.Context, record ReviewDatabaseMaintenanceJobRecord) (*DatabaseMaintenanceJob, error) {
	f.reviewedMaintenanceJob = &record
	return &DatabaseMaintenanceJob{
		ID:           record.JobID,
		Status:       record.Status,
		ReviewedBy:   &record.ReviewedBy,
		ReviewReason: record.ReviewReason,
	}, nil
}

func (f *fakeRepository) ListSchemaTargets(context.Context, int) ([]OrganizationSchemaTarget, error) {
	return nil, nil
}

func (f *fakeRepository) GetSchemaTarget(context.Context, uuid.UUID) (*OrganizationSchemaTarget, error) {
	return nil, nil
}

func (f *fakeRepository) CreateSchemaChangeRequest(_ context.Context, record CreateSchemaChangeRequestRecord) (*SchemaChangeRequest, error) {
	f.createdRecord = &record
	return &SchemaChangeRequest{
		ID:             uuid.New(),
		OrganizationID: record.OrganizationID,
		SchemaName:     record.SchemaName,
		RequestType:    record.RequestType,
		Status:         SchemaChangePending,
		Reason:         record.Reason,
		SchemaPackage:  record.SchemaPackage,
		Statements:     record.Statements,
		RiskLevel:      record.RiskLevel,
		Diff:           record.Diff,
		RequestedBy:    &record.RequestedBy,
	}, nil
}

func (f *fakeRepository) GetSchemaChangeRequest(context.Context, uuid.UUID) (*SchemaChangeRequest, error) {
	return f.request, nil
}

func (f *fakeRepository) UpdateSchemaChangeRequestStatus(context.Context, uuid.UUID, string, uuid.UUID, string) (*SchemaChangeRequest, error) {
	return nil, nil
}

func (f *fakeRepository) ApplySchemaChange(_ context.Context, _ *SchemaChangeRequest, _ []string, assetResults []SchemaApplyAssetResult) (*SchemaApplyJob, error) {
	f.applied = true
	return &SchemaApplyJob{Metadata: map[string]any{"asset_results": assetResults}}, nil
}
