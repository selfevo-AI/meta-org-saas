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
			AssetType: AssetTypeSchemaPackage,
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
				AssetType: AssetTypeSchemaPackage,
				Payload:   mustMap(systemadmin.DefaultOrganizationSchemaPackage()),
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

func (f *fakeRepository) GetPackageByID(context.Context, uuid.UUID) (*Package, error) {
	return f.packageByID[packageID], nil
}

func (f *fakeRepository) CreatePackage(context.Context, CreatePackageInput, uuid.UUID) (*Package, error) {
	return nil, nil
}

func (f *fakeRepository) ActivatePackage(context.Context, uuid.UUID, uuid.UUID) (*Package, error) {
	return nil, nil
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
	return nil, nil
}

func (f *fakeRepository) CreateExtension(context.Context, CreateExtensionInput, uuid.UUID) (*Extension, error) {
	return nil, nil
}

func (f *fakeRepository) ListExtensions(context.Context, uuid.UUID, int) ([]Extension, error) {
	return nil, nil
}

func (f *fakeRepository) GetExtension(context.Context, uuid.UUID) (*Extension, error) {
	return f.extensionByID[extensionID], nil
}

func (f *fakeRepository) CreatePublicationRequest(context.Context, Extension, uuid.UUID, string) (*PublicationRequest, error) {
	return &PublicationRequest{
		ID:                   uuid.New(),
		ExtensionID:          extensionID,
		SourceOrganizationID: organizationID,
		IndustryKey:          "manufacturing",
		Status:               PublicationPending,
		Reason:               "share with partners",
		RequestedBy:          &actorID,
	}, nil
}

func (f *fakeRepository) ListPublicationRequests(context.Context, int) ([]PublicationRequest, error) {
	return nil, nil
}

func (f *fakeRepository) ReviewPublicationRequest(context.Context, uuid.UUID, uuid.UUID, string, string) (*PublicationRequest, error) {
	return nil, nil
}

func (f *fakeRepository) ListKnowledgeSources(context.Context, string, uuid.UUID, int) ([]KnowledgeSource, error) {
	return nil, nil
}
