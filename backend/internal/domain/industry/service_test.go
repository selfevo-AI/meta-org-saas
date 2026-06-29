package industry

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/systemadmin"
)

func TestValidatePackageRejectsDuplicateAssets(t *testing.T) {
	pkg := validPackage()
	pkg.Assets = append(pkg.Assets, pkg.Assets[0])

	err := ValidatePackage(pkg)
	if err == nil {
		t.Fatal("ValidatePackage() succeeded, want duplicate asset error")
	}
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("ValidatePackage() error = %v, want ErrValidation", err)
	}
}

func TestValidatePackageValidatesSchemaAndSkillAssets(t *testing.T) {
	pkg := validPackage()
	pkg.Assets = append(pkg.Assets,
		PackageAsset{
			AssetKey:  "invalid-schema",
			AssetType: AssetTypeSolutionManifest,
			Payload: map[string]any{
				"format_version": "wrong",
				"module_key":     "organization",
			},
		},
	)

	err := ValidatePackage(pkg)
	if err == nil {
		t.Fatal("ValidatePackage() succeeded, want schema validation error")
	}

	pkg = validPackage()
	pkg.Assets = append(pkg.Assets,
		PackageAsset{
			AssetKey:  "invalid-skill",
			AssetType: AssetTypeSkill,
			Payload: map[string]any{
				"skill_key":        "sales_advisor",
				"module_key":       "sales",
				"name":             "Sales Advisor",
				"prompt_template":  "Help with sales",
				"skill_components": []any{map[string]any{"key": "intent"}},
			},
		},
	)
	err = ValidatePackage(pkg)
	if err == nil {
		t.Fatal("ValidatePackage() succeeded, want skill component validation error")
	}
}

func TestAdoptIndustryCreatesSinglePrimaryAdoption(t *testing.T) {
	repo := &fakeRepository{
		role: "system_owner",
		packageByID: map[uuid.UUID]*Package{
			packageID: validPackagePointer(packageID),
		},
	}
	service := NewService(repo)

	adoption, err := service.ApplyPackageToOrganization(context.Background(), actorID, ApplyPackageInput{
		PackageID:      packageID,
		OrganizationID: organizationID,
		ModuleKeys:     []string{"organization", "sales"},
	})
	if err != nil {
		t.Fatalf("ApplyPackageToOrganization() error = %v", err)
	}
	if adoption.IndustryKey != "manufacturing" {
		t.Fatalf("IndustryKey = %q, want manufacturing", adoption.IndustryKey)
	}
	if !adoption.Primary {
		t.Fatal("Primary = false, want true")
	}
	if repo.adoptionWrites != 1 {
		t.Fatalf("adoption writes = %d, want 1", repo.adoptionWrites)
	}
}

func TestUpdatePackageAllowsHumanEditingDraftSolutionAssets(t *testing.T) {
	repo := &fakeRepository{
		role: "system_owner",
		packageByID: map[uuid.UUID]*Package{
			packageID: validPackagePointer(packageID),
		},
	}
	repo.packageByID[packageID].Status = StatusDraft
	service := NewService(repo)

	updated, err := service.UpdatePackage(context.Background(), actorID, packageID, UpdatePackageInput{
		Name:        "Retail Distribution v1",
		Description: "ERP code-table retail distribution package",
		Assets: []PackageAsset{
			{AssetKey: "retail-module", AssetType: AssetTypeModule, Payload: map[string]any{"module_key": "retail", "display_name": "Retail"}},
			{AssetKey: "retail-pos-operation", AssetType: AssetTypeRuntimeOperation, Payload: map[string]any{"operation_key": "erp.mrps.close", "path": "/erp/MRPS/{key}/actions/close"}},
		},
		Metadata: map[string]any{"code_table_only": true},
	})
	if err != nil {
		t.Fatalf("UpdatePackage() error = %v", err)
	}
	if updated.Name != "Retail Distribution v1" || len(updated.Assets) != 2 {
		t.Fatalf("updated package = %#v, want edited draft assets", updated)
	}
	if repo.updatedPackage == nil || repo.updatedPackage.PackageID != packageID {
		t.Fatalf("updatedPackage = %#v, want package id recorded", repo.updatedPackage)
	}
}

func TestDeletePackageArchivesAppliedSolutionAndDeletesDraft(t *testing.T) {
	repo := &fakeRepository{
		role: "system_owner",
		packageByID: map[uuid.UUID]*Package{
			packageID: validPackagePointer(packageID),
		},
	}
	repo.packageByID[packageID].Status = StatusDraft
	service := NewService(repo)

	deleted, err := service.DeletePackage(context.Background(), actorID, packageID)
	if err != nil {
		t.Fatalf("DeletePackage() draft error = %v", err)
	}
	if deleted.Status != StatusArchived || repo.deletedPackageID != packageID {
		t.Fatalf("draft delete result = %#v deleted=%s, want archived/deleted record", deleted, repo.deletedPackageID)
	}

	repo.deletedPackageID = uuid.Nil
	repo.packageByID[packageID] = validPackagePointer(packageID)
	repo.packageApplied = true
	archived, err := service.DeletePackage(context.Background(), actorID, packageID)
	if err != nil {
		t.Fatalf("DeletePackage() applied error = %v", err)
	}
	if archived.Status != StatusArchived || repo.deletedPackageID != uuid.Nil {
		t.Fatalf("applied delete result = %#v deleted=%s, want archived without physical delete", archived, repo.deletedPackageID)
	}
}

func TestApplyPackageRejectsModulesOutsideIndustryAndExtensions(t *testing.T) {
	repo := &fakeRepository{
		role: "system_owner",
		packageByID: map[uuid.UUID]*Package{
			packageID: validPackagePointer(packageID),
		},
	}
	service := NewService(repo)

	_, err := service.ApplyPackageToOrganization(context.Background(), actorID, ApplyPackageInput{
		PackageID:      packageID,
		OrganizationID: organizationID,
		ModuleKeys:     []string{"finance"},
	})
	if err == nil {
		t.Fatal("ApplyPackageToOrganization() succeeded, want module validation error")
	}

	repo.allowedExtensionModules = []string{"finance"}
	_, err = service.ApplyPackageToOrganization(context.Background(), actorID, ApplyPackageInput{
		PackageID:      packageID,
		OrganizationID: organizationID,
		ModuleKeys:     []string{"finance"},
	})
	if err != nil {
		t.Fatalf("ApplyPackageToOrganization() with extension module error = %v", err)
	}
}

func TestValidateOrganizationModulesAllowsERPForGeneralIndustry(t *testing.T) {
	generalPackageID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	generalPackage := validPackage()
	generalPackage.ID = generalPackageID
	generalPackage.IndustryKey = "general"
	generalPackage.PackageKey = "general-foundation"
	generalPackage.Assets = []PackageAsset{
		{AssetKey: "organization-module", AssetType: AssetTypeModule, Payload: map[string]any{"module_key": "organization"}},
	}
	repo := &fakeRepository{
		packageByID: map[uuid.UUID]*Package{
			generalPackageID: &generalPackage,
		},
		adoption: &OrganizationAdoption{
			OrganizationID: organizationID,
			IndustryKey:    "general",
			PackageID:      generalPackageID,
			Primary:        true,
			EnabledModules: []string{"organization"},
			Status:         StatusActive,
		},
	}
	service := NewService(repo)

	if err := service.ValidateOrganizationModules(context.Background(), organizationID, []string{"organization", "erp"}); err != nil {
		t.Fatalf("ValidateOrganizationModules() error = %v, want general industry to allow erp", err)
	}
}

func TestSubmitPublicationCreatesPendingRequestForTenantExtension(t *testing.T) {
	repo := &fakeRepository{
		membership: "organization_admin",
		extensionByID: map[uuid.UUID]*Extension{
			extensionID: {
				ID:             extensionID,
				OrganizationID: organizationID,
				IndustryKey:    "manufacturing",
				PackageID:      packageID,
				ExtensionKey:   "private-costing",
				Name:           "Private Costing",
				Status:         StatusActive,
			},
		},
	}
	service := NewService(repo)

	request, err := service.SubmitPublicationRequest(context.Background(), actorID, extensionID, "share with partners")
	if err != nil {
		t.Fatalf("SubmitPublicationRequest() error = %v", err)
	}
	if request.Status != PublicationPending {
		t.Fatalf("Status = %q, want pending", request.Status)
	}
	if request.SourceOrganizationID != organizationID {
		t.Fatalf("SourceOrganizationID = %s, want %s", request.SourceOrganizationID, organizationID)
	}
}

func TestReviewPublicationRequestRejectsFailedPublicationGate(t *testing.T) {
	repo := &fakeRepository{
		role: "system_owner",
		publicationRequest: &PublicationRequest{
			ID:          uuid.New(),
			ExtensionID: extensionID,
			Status:      PublicationPending,
			Metadata:    map[string]any{},
		},
		extensionByID: map[uuid.UUID]*Extension{
			extensionID: {
				ID:             extensionID,
				OrganizationID: organizationID,
				IndustryKey:    "manufacturing",
				ExtensionKey:   "customer-specific-extension",
				Name:           "Customer specific extension",
				Assets: []PackageAsset{
					{AssetKey: "customer_export", AssetType: AssetTypeRuntimeEntity, Payload: map[string]any{"customer_name": "Acme Corp"}},
				},
			},
		},
	}
	service := NewService(repo)

	_, err := service.ReviewPublicationRequest(context.Background(), actorID, repo.publicationRequest.ID, PublicationApproved, "approve")
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("ReviewPublicationRequest error = %v, want ErrValidation", err)
	}
	if repo.reviewedStatus == PublicationApproved {
		t.Fatal("publication request approved despite failed gate")
	}
}

func TestReviewPublicationRequestAllowsWarningsAndPersistsGateMetadata(t *testing.T) {
	repo := &fakeRepository{
		role: "system_owner",
		publicationRequest: &PublicationRequest{
			ID:          uuid.New(),
			ExtensionID: extensionID,
			Status:      PublicationPending,
			Metadata:    map[string]any{},
		},
		extensionByID: map[uuid.UUID]*Extension{
			extensionID: {
				ID:             extensionID,
				OrganizationID: organizationID,
				IndustryKey:    "manufacturing",
				ExtensionKey:   "verified-extension",
				Name:           "Verified extension",
				Metadata: map[string]any{
					"required_verification_scenarios": []any{"source_to_pay_smoke"},
					"verification_scenario_results": []any{
						map[string]any{"scenario_key": "source_to_pay_smoke", "status": "warning"},
					},
				},
				Assets: []PackageAsset{
					{AssetKey: "knowledge_source.safe", AssetType: AssetTypeKnowledgeSource, Payload: map[string]any{"permission": map[string]any{"allow_publication": true}}},
				},
			},
		},
	}
	service := NewService(repo)

	result, err := service.ReviewPublicationRequest(context.Background(), actorID, repo.publicationRequest.ID, PublicationApproved, "approve")
	if err != nil {
		t.Fatalf("ReviewPublicationRequest error = %v", err)
	}
	if result.Status != PublicationApproved {
		t.Fatalf("status = %q, want approved", result.Status)
	}
	if len(repo.publicationGateResults) == 0 {
		t.Fatal("gate results were not persisted")
	}
}

var (
	actorID        = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	organizationID = uuid.MustParse("22222222-2222-2222-2222-222222222222")
	packageID      = uuid.MustParse("33333333-3333-3333-3333-333333333333")
	extensionID    = uuid.MustParse("44444444-4444-4444-4444-444444444444")
)

func validPackagePointer(id uuid.UUID) *Package {
	pkg := validPackage()
	pkg.ID = id
	return &pkg
}

func validPackage() Package {
	return Package{
		IndustryKey: "manufacturing",
		PackageKey:  "manufacturing-foundation",
		Version:     1,
		Status:      StatusActive,
		Assets: []PackageAsset{
			{
				AssetKey:  "organization-schema",
				AssetType: AssetTypeSolutionManifest,
				Payload:   mustMap(systemadmin.DefaultOrganizationIndustrySolutionManifest()),
			},
			{
				AssetKey:  "organization-module",
				AssetType: AssetTypeModule,
				Payload: map[string]any{
					"module_key":   "organization",
					"display_name": "Organization",
				},
			},
			{
				AssetKey:  "sales-module",
				AssetType: AssetTypeModule,
				Payload: map[string]any{
					"module_key":   "sales",
					"display_name": "Sales",
				},
			},
			{
				AssetKey:  "sales-skill",
				AssetType: AssetTypeSkill,
				Payload: map[string]any{
					"skill_key":       "sales_advisor",
					"module_key":      "sales",
					"name":            "Sales Advisor",
					"prompt_template": "Help with sales",
					"skill_components": []any{
						map[string]any{"key": "intent", "instruction": "Clarify intent"},
						map[string]any{"key": "context", "instruction": "Collect context"},
						map[string]any{"key": "action", "instruction": "Recommend action"},
					},
				},
			},
		},
	}
}

type fakeRepository struct {
	role                    string
	membership              string
	packageByID             map[uuid.UUID]*Package
	extensionByID           map[uuid.UUID]*Extension
	allowedExtensionModules []string
	adoption                *OrganizationAdoption
	publicationRequest      *PublicationRequest
	publicationGateResults  []PublicationGateResult
	reviewedStatus          string
	updatedPackage          *UpdatePackageRecord
	deletedPackageID        uuid.UUID
	packageApplied          bool
	adoptionWrites          int
}

func (f *fakeRepository) GetPlatformRole(context.Context, uuid.UUID) (string, error) {
	if f.role == "" {
		return "", errors.New("no platform role")
	}
	return f.role, nil
}

func (f *fakeRepository) GetOrganizationAuthority(context.Context, uuid.UUID, uuid.UUID) (string, error) {
	if f.membership == "" {
		return "", errors.New("no membership")
	}
	return f.membership, nil
}

func (f *fakeRepository) ListIndustries(context.Context, int) ([]Industry, error) {
	return nil, nil
}

func (f *fakeRepository) GetIndustry(context.Context, string) (*Industry, error) {
	return nil, nil
}

func (f *fakeRepository) CreateIndustry(context.Context, CreateIndustryInput, uuid.UUID) (*Industry, error) {
	return nil, nil
}

func (f *fakeRepository) ListPackages(context.Context, string, int) ([]Package, error) {
	return nil, nil
}

func (f *fakeRepository) GetPackage(context.Context, uuid.UUID) (*Package, error) {
	return f.packageByID[uuid.Nil], nil
}

func (f *fakeRepository) GetPackageByID(_ context.Context, id uuid.UUID) (*Package, error) {
	return f.packageByID[id], nil
}

func (f *fakeRepository) CreatePackage(context.Context, CreatePackageInput, uuid.UUID) (*Package, error) {
	return nil, nil
}

func (f *fakeRepository) UpdatePackage(_ context.Context, record UpdatePackageRecord) (*Package, error) {
	f.updatedPackage = &record
	current := *f.packageByID[record.PackageID]
	current.Name = record.Name
	current.Description = record.Description
	current.Status = record.Status
	current.Assets = record.Assets
	current.Metadata = record.Metadata
	f.packageByID[record.PackageID] = &current
	return &current, nil
}

func (f *fakeRepository) ActivatePackage(context.Context, uuid.UUID, uuid.UUID) (*Package, error) {
	return nil, nil
}

func (f *fakeRepository) ArchivePackage(_ context.Context, packageID uuid.UUID, _ uuid.UUID) (*Package, error) {
	current := *f.packageByID[packageID]
	current.Status = StatusArchived
	f.packageByID[packageID] = &current
	return &current, nil
}

func (f *fakeRepository) DeletePackage(_ context.Context, packageID uuid.UUID) (*Package, error) {
	current := *f.packageByID[packageID]
	current.Status = StatusArchived
	f.deletedPackageID = packageID
	delete(f.packageByID, packageID)
	return &current, nil
}

func (f *fakeRepository) PackageHasAdoptions(context.Context, uuid.UUID) (bool, error) {
	return f.packageApplied, nil
}

func (f *fakeRepository) UpsertAdoption(context.Context, ApplyPackageInput, Package) (*OrganizationAdoption, error) {
	f.adoptionWrites++
	return &OrganizationAdoption{
		OrganizationID: organizationID,
		IndustryKey:    "manufacturing",
		PackageID:      packageID,
		Primary:        true,
		EnabledModules: []string{"organization", "sales"},
		Status:         StatusActive,
	}, nil
}

func (f *fakeRepository) ListOrganizationExtensionModules(context.Context, uuid.UUID, string) ([]string, error) {
	return f.allowedExtensionModules, nil
}

func (f *fakeRepository) GetAdoption(context.Context, uuid.UUID) (*OrganizationAdoption, error) {
	return f.adoption, nil
}

func (f *fakeRepository) CreateExtension(context.Context, CreateExtensionInput, uuid.UUID) (*Extension, error) {
	return nil, nil
}

func (f *fakeRepository) ListExtensions(context.Context, uuid.UUID, int) ([]Extension, error) {
	return nil, nil
}

func (f *fakeRepository) GetExtension(_ context.Context, id uuid.UUID) (*Extension, error) {
	return f.extensionByID[id], nil
}

func (f *fakeRepository) CreatePublicationRequest(_ context.Context, extension Extension, _ uuid.UUID, reason string, metadata map[string]any) (*PublicationRequest, error) {
	return &PublicationRequest{
		ID:                   uuid.New(),
		ExtensionID:          extension.ID,
		SourceOrganizationID: extension.OrganizationID,
		IndustryKey:          extension.IndustryKey,
		Status:               PublicationPending,
		Reason:               reason,
		RequestedBy:          &actorID,
		Metadata:             metadata,
	}, nil
}

func (f *fakeRepository) GetPublicationRequest(context.Context, uuid.UUID) (*PublicationRequest, error) {
	return f.publicationRequest, nil
}

func (f *fakeRepository) UpdatePublicationRequestMetadata(_ context.Context, _ uuid.UUID, metadata map[string]any) error {
	if gates, ok := metadata["publication_gates"].([]PublicationGateResult); ok {
		f.publicationGateResults = gates
	}
	if f.publicationRequest != nil {
		f.publicationRequest.Metadata = metadata
	}
	return nil
}

func (f *fakeRepository) ListPublicationRequests(context.Context, int) ([]PublicationRequest, error) {
	return nil, nil
}

func (f *fakeRepository) ReviewPublicationRequest(_ context.Context, _ uuid.UUID, _ uuid.UUID, status string, reason string) (*PublicationRequest, error) {
	f.reviewedStatus = status
	return &PublicationRequest{
		ID:           f.publicationRequest.ID,
		ExtensionID:  f.publicationRequest.ExtensionID,
		Status:       status,
		ReviewReason: reason,
		Metadata:     f.publicationRequest.Metadata,
	}, nil
}

func (f *fakeRepository) ListKnowledgeSources(context.Context, string, uuid.UUID, int) ([]KnowledgeSource, error) {
	return nil, nil
}
