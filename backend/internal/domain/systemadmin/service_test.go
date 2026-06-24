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
	role          string
	request       *SchemaChangeRequest
	createdRecord *CreateSchemaChangeRequestRecord
	applied       bool
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
