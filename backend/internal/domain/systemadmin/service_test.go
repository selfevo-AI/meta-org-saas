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

func TestDefaultOrganizationIndustrySolutionManifestIsValid(t *testing.T) {
	pkg := DefaultOrganizationIndustrySolutionManifest()

	if err := ValidateIndustrySolutionManifest(pkg); err != nil {
		t.Fatalf("DefaultOrganizationIndustrySolutionManifest() invalid: %v", err)
	}
}

func TestApplyIndustrySolutionChangeRejectsPendingRequest(t *testing.T) {
	repo := &fakeRepository{
		role: "system_owner",
		request: &IndustrySolutionChangeRequest{
			ID:               uuid.New(),
			OrganizationID:   uuid.New(),
			Status:           IndustrySolutionChangePending,
			SolutionManifest: DefaultOrganizationIndustrySolutionManifest(),
		},
	}
	service := NewService(repo)

	_, err := service.ApplyIndustrySolutionChange(context.Background(), uuid.New(), repo.request.ID)
	if err == nil {
		t.Fatal("ApplyIndustrySolutionChange() succeeded, want error")
	}
	if err != ErrInvalidTransition {
		t.Fatalf("ApplyIndustrySolutionChange() error = %v, want ErrInvalidTransition", err)
	}
	if repo.applied {
		t.Fatal("ApplyIndustrySolutionChange() applied pending request")
	}
}

func TestApplyIndustrySolutionChangeRejectsAuditorRole(t *testing.T) {
	repo := &fakeRepository{
		role: "auditor",
		request: &IndustrySolutionChangeRequest{
			ID:               uuid.New(),
			OrganizationID:   uuid.New(),
			Status:           IndustrySolutionChangeApproved,
			SolutionManifest: DefaultOrganizationIndustrySolutionManifest(),
		},
	}
	service := NewService(repo)

	_, err := service.ApplyIndustrySolutionChange(context.Background(), uuid.New(), repo.request.ID)

	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("ApplyIndustrySolutionChange error = %v, want ErrForbidden", err)
	}
	if repo.applied {
		t.Fatal("ApplyIndustrySolutionChange applied schema for auditor role")
	}
}

func TestVerifyIndustrySolutionChangeReportsChecksWithoutApplying(t *testing.T) {
	repo := &fakeRepository{
		role: "system_owner",
		request: &IndustrySolutionChangeRequest{
			ID:               uuid.New(),
			OrganizationID:   uuid.New(),
			TargetSchemaName: "org_123e4567e89b12d3a456426614174000",
			Status:           IndustrySolutionChangeApproved,
			SolutionManifest: DefaultOrganizationIndustrySolutionManifest(),
			RiskLevel:        IndustrySolutionRiskSafe,
		},
	}
	service := NewService(repo)

	report, err := service.VerifyIndustrySolutionChange(context.Background(), uuid.New(), repo.request.ID)
	if err != nil {
		t.Fatalf("VerifyIndustrySolutionChange() error = %v", err)
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
		t.Fatal("VerifyIndustrySolutionChange() applied industry solution change")
	}
}

func TestVerifyIndustrySolutionChangeReportsIndustryFactoryCoverage(t *testing.T) {
	pkg := BuildERPSolutionManifest(ERPSolutionFlowRequest{
		IndustryKey:    "professional_services",
		PackageKey:     "erp_standard",
		Name:           "ERP Standard",
		EnabledModules: []string{"project", "procurement", "inventory", "sales", "finance"},
	})
	repo := &fakeRepository{
		role: "system_owner",
		request: &IndustrySolutionChangeRequest{
			ID:               uuid.New(),
			OrganizationID:   uuid.New(),
			TargetSchemaName: "org_123e4567e89b12d3a456426614174000",
			RequestType:      "erp_solution_flow",
			Status:           IndustrySolutionChangeApproved,
			SolutionManifest: pkg,
			RiskLevel:        IndustrySolutionRiskSafe,
		},
	}
	service := NewService(repo)

	report, err := service.VerifyIndustrySolutionChange(context.Background(), uuid.New(), repo.request.ID)
	if err != nil {
		t.Fatalf("VerifyIndustrySolutionChange() error = %v", err)
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
		t.Fatal("VerifyIndustrySolutionChange() applied industry solution change")
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

func TestCreateDatabaseMaintenanceJobSupportsTenantDatabaseScope(t *testing.T) {
	actorID := uuid.New()
	orgID := uuid.New()
	repo := &fakeRepository{
		role: "maintenance_admin",
		rolePermissions: map[string][]string{
			"maintenance_admin": {
				platformauth.PermissionPlatformRead,
				platformauth.PermissionDatabaseMaintenanceManage,
			},
		},
	}
	service := NewService(repo)

	job, err := service.CreateDatabaseMaintenanceJob(context.Background(), actorID, CreateDatabaseMaintenanceJobInput{
		JobType: "backup",
		Scope:   "tenant_database:" + orgID.String(),
		Reason:  "export tenant business database before solution upgrade",
	})

	if err != nil {
		t.Fatalf("CreateDatabaseMaintenanceJob() error = %v", err)
	}
	if job.Scope != "tenant_database:"+orgID.String() {
		t.Fatalf("Scope = %q, want tenant_database:<org_id>", job.Scope)
	}
	if repo.createdMaintenanceJob == nil {
		t.Fatal("repository did not receive maintenance job")
	}
	if repo.createdMaintenanceJob.Metadata["target_scope"] != "tenant_database" {
		t.Fatalf("metadata target_scope = %#v, want tenant_database", repo.createdMaintenanceJob.Metadata["target_scope"])
	}
	if repo.createdMaintenanceJob.Metadata["organization_id"] != orgID.String() {
		t.Fatalf("metadata organization_id = %#v, want %s", repo.createdMaintenanceJob.Metadata["organization_id"], orgID)
	}
}

func TestCreatePrivateDeploymentExportJobCreatesTenantBackupMetadataOnlyTask(t *testing.T) {
	actorID := uuid.New()
	orgID := uuid.New()
	repo := &fakeRepository{
		role: "maintenance_admin",
		rolePermissions: map[string][]string{
			"maintenance_admin": {
				platformauth.PermissionPlatformRead,
				platformauth.PermissionDatabaseMaintenanceManage,
			},
		},
	}
	service := NewService(repo)

	job, err := service.CreatePrivateDeploymentExportJob(context.Background(), actorID, orgID)

	if err != nil {
		t.Fatalf("CreatePrivateDeploymentExportJob() error = %v", err)
	}
	if job.JobType != "backup" {
		t.Fatalf("JobType = %q, want backup", job.JobType)
	}
	if job.Scope != "tenant_database:"+orgID.String() {
		t.Fatalf("Scope = %q, want tenant_database:<org_id>", job.Scope)
	}
	if repo.createdMaintenanceJob.Metadata["export_purpose"] != "private_deployment" {
		t.Fatalf("export_purpose = %#v", repo.createdMaintenanceJob.Metadata["export_purpose"])
	}
	if repo.createdMaintenanceJob.Metadata["execution_mode"] != "metadata_only_reserved" {
		t.Fatalf("execution_mode = %#v", repo.createdMaintenanceJob.Metadata["execution_mode"])
	}
}

func TestCreateIndustrySolutionTableFieldChangeBuildsPhysicalIndustrySolutionManifest(t *testing.T) {
	actorID := uuid.New()
	orgID := uuid.New()
	repo := &fakeRepository{
		role: "solution_admin",
		rolePermissions: map[string][]string{
			"solution_admin": {
				platformauth.PermissionPlatformRead,
				platformauth.PermissionIndustrySolutionManage,
				platformauth.PermissionIndustrySolutionExport,
			},
		},
	}
	service := NewService(repo)

	request, err := service.CreateIndustrySolutionTableFieldChange(context.Background(), actorID, CreateIndustrySolutionTableFieldChangeInput{
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
		t.Fatalf("CreateIndustrySolutionTableFieldChange() error = %v", err)
	}
	if request.RequestType != "industry_solution_table_field_change" {
		t.Fatalf("RequestType = %q, want industry_solution_table_field_change", request.RequestType)
	}
	if repo.createdRecord == nil {
		t.Fatal("industry solution change record was not created")
	}
	if len(repo.createdRecord.SolutionManifest.Tables) != 1 {
		t.Fatalf("tables = %#v, want one table", repo.createdRecord.SolutionManifest.Tables)
	}
	table := repo.createdRecord.SolutionManifest.Tables[0]
	if table.Name != "tenant_quality_inspections" || len(table.Fields) != 3 {
		t.Fatalf("table = %#v, want physical table with id plus two fields", table)
	}
	if repo.createdRecord.SolutionManifest.Metadata["industry_key"] != "manufacturing" {
		t.Fatalf("metadata = %#v, want industry_key", repo.createdRecord.SolutionManifest.Metadata)
	}
}

func TestVerifyIndustrySolutionChangeWarnsWhenIndustryFactoryCoverageIsIncomplete(t *testing.T) {
	pkg := BuildERPSolutionManifest(ERPSolutionFlowRequest{
		IndustryKey:    "professional_services",
		PackageKey:     "erp_standard",
		Name:           "ERP Standard",
		EnabledModules: []string{"project", "procurement"},
	})
	delete(pkg.Metadata, "verification_scenarios")
	repo := &fakeRepository{
		role: "system_owner",
		request: &IndustrySolutionChangeRequest{
			ID:               uuid.New(),
			OrganizationID:   uuid.New(),
			TargetSchemaName: "org_123e4567e89b12d3a456426614174000",
			RequestType:      "erp_solution_flow",
			Status:           IndustrySolutionChangeApproved,
			SolutionManifest: pkg,
			RiskLevel:        IndustrySolutionRiskSafe,
		},
	}
	service := NewService(repo)

	report, err := service.VerifyIndustrySolutionChange(context.Background(), uuid.New(), repo.request.ID)
	if err != nil {
		t.Fatalf("VerifyIndustrySolutionChange() error = %v", err)
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
		t.Fatal("VerifyIndustrySolutionChange() applied industry solution change")
	}
}

func TestVerifyIndustrySolutionChangeBlocksApplyForDuplicateRuntimeOperations(t *testing.T) {
	pkg := BuildERPSolutionManifest(ERPSolutionFlowRequest{IndustryKey: "professional_services", PackageKey: "erp_standard", Name: "ERP Standard"})
	manifest, err := AssetManifestFromSolutionManifest(pkg)
	if err != nil {
		t.Fatalf("AssetManifestFromSolutionManifest error = %v", err)
	}
	manifest.Assets = append(manifest.Assets, IndustrySolutionAsset{
		AssetKey:  "runtime_operation.duplicate",
		AssetType: AssetTypeRuntimeOperation,
		Version:   "v1",
		RiskLevel: "medium",
		Payload:   map[string]any{"path": "/erp/catalog"},
	})
	setIndustryAssetManifest(&pkg, manifest)

	repo := &fakeRepository{role: "system_owner", request: &IndustrySolutionChangeRequest{
		ID:               uuid.New(),
		OrganizationID:   uuid.New(),
		TargetSchemaName: "org_123e4567e89b12d3a456426614174000",
		RequestType:      "erp_solution_flow",
		Status:           IndustrySolutionChangeApproved,
		SolutionManifest: pkg,
		RiskLevel:        IndustrySolutionRiskSafe,
	}}
	service := NewService(repo)

	report, err := service.VerifyIndustrySolutionChange(context.Background(), uuid.New(), repo.request.ID)
	if err != nil {
		t.Fatalf("VerifyIndustrySolutionChange error = %v", err)
	}
	check := verificationCheckByKey(report, "runtime_operations")
	if check == nil || check.Status != "failed" {
		t.Fatalf("runtime_operations check = %#v, want failed", check)
	}
	if report.CanApply || report.BlockingIssues == 0 {
		t.Fatalf("report can_apply/blocking = %v/%d, want blocked", report.CanApply, report.BlockingIssues)
	}
}

func TestVerifyIndustrySolutionChangeBlocksActiveContextRules(t *testing.T) {
	pkg := BuildERPSolutionManifest(ERPSolutionFlowRequest{IndustryKey: "professional_services", PackageKey: "erp_standard", Name: "ERP Standard"})
	manifest, err := AssetManifestFromSolutionManifest(pkg)
	if err != nil {
		t.Fatalf("AssetManifestFromSolutionManifest error = %v", err)
	}
	for i := range manifest.Assets {
		if manifest.Assets[i].AssetType == AssetTypeContextRule {
			manifest.Assets[i].Payload["status"] = "active"
			break
		}
	}
	setIndustryAssetManifest(&pkg, manifest)

	repo := &fakeRepository{role: "system_owner", request: &IndustrySolutionChangeRequest{
		ID:               uuid.New(),
		OrganizationID:   uuid.New(),
		TargetSchemaName: "org_123e4567e89b12d3a456426614174000",
		RequestType:      "erp_solution_flow",
		Status:           IndustrySolutionChangeApproved,
		SolutionManifest: pkg,
		RiskLevel:        IndustrySolutionRiskSafe,
	}}
	service := NewService(repo)

	report, err := service.VerifyIndustrySolutionChange(context.Background(), uuid.New(), repo.request.ID)
	if err != nil {
		t.Fatalf("VerifyIndustrySolutionChange error = %v", err)
	}
	check := verificationCheckByKey(report, "assistant_context")
	if check == nil || check.Status != "failed" {
		t.Fatalf("assistant_context check = %#v, want failed", check)
	}
	if report.CanApply || report.BlockingIssues == 0 {
		t.Fatalf("report can_apply/blocking = %v/%d, want blocked", report.CanApply, report.BlockingIssues)
	}
}

func TestApplyIndustrySolutionChangeRejectsManifestBlockingIssues(t *testing.T) {
	pkg := BuildERPSolutionManifest(ERPSolutionFlowRequest{IndustryKey: "professional_services", PackageKey: "erp_standard", Name: "ERP Standard"})
	manifest, err := AssetManifestFromSolutionManifest(pkg)
	if err != nil {
		t.Fatalf("AssetManifestFromSolutionManifest error = %v", err)
	}
	for i := range manifest.Assets {
		if manifest.Assets[i].AssetType == AssetTypeContextRule {
			manifest.Assets[i].Payload["status"] = "active"
			break
		}
	}
	setIndustryAssetManifest(&pkg, manifest)

	repo := &fakeRepository{role: "system_owner", request: &IndustrySolutionChangeRequest{
		ID:               uuid.New(),
		OrganizationID:   uuid.New(),
		TargetSchemaName: "org_123e4567e89b12d3a456426614174000",
		RequestType:      "erp_solution_flow",
		Status:           IndustrySolutionChangeApproved,
		SolutionManifest: pkg,
		RiskLevel:        IndustrySolutionRiskSafe,
	}}
	service := NewService(repo)

	_, err = service.ApplyIndustrySolutionChange(context.Background(), uuid.New(), repo.request.ID)
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("ApplyIndustrySolutionChange error = %v, want ErrValidation", err)
	}
	if repo.applied {
		t.Fatal("ApplyIndustrySolutionChange applied request with blocking manifest issue")
	}
}

func verificationCheckByKey(report *IndustrySolutionVerificationReport, key string) *IndustrySolutionVerificationCheck {
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
	request                *IndustrySolutionChangeRequest
	createdRecord          *CreateIndustrySolutionChangeRequestRecord
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

func (f *fakeRepository) ListIndustrySolutionTargets(context.Context, int) ([]OrganizationIndustrySolutionTarget, error) {
	return nil, nil
}

func (f *fakeRepository) GetIndustrySolutionTarget(context.Context, uuid.UUID) (*OrganizationIndustrySolutionTarget, error) {
	return nil, nil
}

func (f *fakeRepository) CreateIndustrySolutionChangeRequest(_ context.Context, record CreateIndustrySolutionChangeRequestRecord) (*IndustrySolutionChangeRequest, error) {
	f.createdRecord = &record
	return &IndustrySolutionChangeRequest{
		ID:               uuid.New(),
		OrganizationID:   record.OrganizationID,
		TargetSchemaName: record.TargetSchemaName,
		RequestType:      record.RequestType,
		Status:           IndustrySolutionChangePending,
		Reason:           record.Reason,
		SolutionManifest: record.SolutionManifest,
		Statements:       record.Statements,
		RiskLevel:        record.RiskLevel,
		Diff:             record.Diff,
		RequestedBy:      &record.RequestedBy,
	}, nil
}

func (f *fakeRepository) GetIndustrySolutionChangeRequest(context.Context, uuid.UUID) (*IndustrySolutionChangeRequest, error) {
	return f.request, nil
}

func (f *fakeRepository) UpdateIndustrySolutionChangeRequestStatus(context.Context, uuid.UUID, string, uuid.UUID, string) (*IndustrySolutionChangeRequest, error) {
	return nil, nil
}

func (f *fakeRepository) ApplyIndustrySolutionChange(_ context.Context, _ *IndustrySolutionChangeRequest, _ []string, assetResults []IndustrySolutionApplyAssetResult) (*IndustrySolutionApplyJob, error) {
	f.applied = true
	return &IndustrySolutionApplyJob{Metadata: map[string]any{"asset_results": assetResults}}, nil
}
