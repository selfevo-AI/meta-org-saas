package industry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org/backend/internal/domain/systemadmin"
	"github.com/selfevo-AI/meta-org/backend/internal/pkg/platformauth"
)

var (
	ErrValidation = errors.New("validation error")
	ErrForbidden  = errors.New("forbidden")
	ErrNotFound   = errors.New("not found")
)

type repository interface {
	GetPlatformRole(context.Context, uuid.UUID) (string, error)
	GetOrganizationAuthority(context.Context, uuid.UUID, uuid.UUID) (string, error)
	ListIndustries(context.Context, int) ([]Industry, error)
	GetIndustry(context.Context, string) (*Industry, error)
	CreateIndustry(context.Context, CreateIndustryInput, uuid.UUID) (*Industry, error)
	ListPackages(context.Context, string, int) ([]Package, error)
	GetPackage(context.Context, uuid.UUID) (*Package, error)
	GetPackageByID(context.Context, uuid.UUID) (*Package, error)
	CreatePackage(context.Context, CreatePackageInput, uuid.UUID) (*Package, error)
	ActivatePackage(context.Context, uuid.UUID, uuid.UUID) (*Package, error)
	UpsertAdoption(context.Context, ApplyPackageInput, Package) (*OrganizationAdoption, error)
	ListOrganizationExtensionModules(context.Context, uuid.UUID, string) ([]string, error)
	GetAdoption(context.Context, uuid.UUID) (*OrganizationAdoption, error)
	CreateExtension(context.Context, CreateExtensionInput, uuid.UUID) (*Extension, error)
	ListExtensions(context.Context, uuid.UUID, int) ([]Extension, error)
	GetExtension(context.Context, uuid.UUID) (*Extension, error)
	CreatePublicationRequest(context.Context, Extension, uuid.UUID, string) (*PublicationRequest, error)
	ListPublicationRequests(context.Context, int) ([]PublicationRequest, error)
	ReviewPublicationRequest(context.Context, uuid.UUID, uuid.UUID, string, string) (*PublicationRequest, error)
	ListKnowledgeSources(context.Context, string, uuid.UUID, int) ([]KnowledgeSource, error)
}

type Service struct {
	repo repository
}

func NewService(repo repository) *Service {
	return &Service{repo: repo}
}

func ValidatePackage(pkg Package) error {
	if strings.TrimSpace(pkg.IndustryKey) == "" {
		return fmt.Errorf("%w: industry_key is required", ErrValidation)
	}
	if strings.TrimSpace(pkg.PackageKey) == "" {
		return fmt.Errorf("%w: package_key is required", ErrValidation)
	}
	seen := map[string]bool{}
	for _, asset := range pkg.Assets {
		key := strings.TrimSpace(asset.AssetKey)
		if key == "" {
			return fmt.Errorf("%w: asset_key is required", ErrValidation)
		}
		if seen[key] {
			return fmt.Errorf("%w: duplicate asset %s", ErrValidation, key)
		}
		seen[key] = true
		if err := validateAsset(asset); err != nil {
			return err
		}
	}
	return nil
}

func validateAsset(asset PackageAsset) error {
	switch asset.AssetType {
	case AssetTypeSchemaPackage:
		var pkg systemadmin.SchemaPackage
		if err := decodePayload(asset.Payload, &pkg); err != nil {
			return fmt.Errorf("%w: invalid schema package payload", ErrValidation)
		}
		if err := systemadmin.ValidateSchemaPackage(pkg); err != nil {
			return fmt.Errorf("%w: %v", ErrValidation, err)
		}
	case AssetTypeModule:
		if strings.TrimSpace(stringFromPayload(asset.Payload, "module_key")) == "" {
			return fmt.Errorf("%w: module asset %s requires module_key", ErrValidation, asset.AssetKey)
		}
	case AssetTypeSkill, AssetTypeSkillStructure:
		if strings.TrimSpace(stringFromPayload(asset.Payload, "skill_key")) == "" && asset.AssetType == AssetTypeSkill {
			return fmt.Errorf("%w: skill asset %s requires skill_key", ErrValidation, asset.AssetKey)
		}
		components, _ := asset.Payload["skill_components"].([]any)
		if len(components) < 3 || len(components) > 9 {
			return fmt.Errorf("%w: skill asset %s requires 3 to 9 skill_components", ErrValidation, asset.AssetKey)
		}
	case AssetTypeRuntimeEntity, AssetTypeRuntimeOperation, AssetTypeKnowledgeSource, AssetTypeModelPolicy, AssetTypeI18n:
		if len(asset.Payload) == 0 {
			return fmt.Errorf("%w: asset %s payload is required", ErrValidation, asset.AssetKey)
		}
	default:
		return fmt.Errorf("%w: unsupported asset_type %q", ErrValidation, asset.AssetType)
	}
	return nil
}

func (s *Service) ListIndustries(ctx context.Context, limit int) ([]Industry, error) {
	return s.repo.ListIndustries(ctx, normalizeLimit(limit))
}

func (s *Service) CreateIndustry(ctx context.Context, actorID uuid.UUID, input CreateIndustryInput) (*Industry, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionPlatformRead); err != nil {
		return nil, err
	}
	input.IndustryKey = normalizeKey(input.IndustryKey)
	input.Name = strings.TrimSpace(input.Name)
	if input.IndustryKey == "" || input.Name == "" {
		return nil, fmt.Errorf("%w: industry_key and name are required", ErrValidation)
	}
	if input.Status == "" {
		input.Status = StatusDraft
	}
	return s.repo.CreateIndustry(ctx, input, actorID)
}

func (s *Service) ListPackages(ctx context.Context, industryKey string, limit int) ([]Package, error) {
	return s.repo.ListPackages(ctx, normalizeKey(industryKey), normalizeLimit(limit))
}

func (s *Service) CreatePackage(ctx context.Context, actorID uuid.UUID, input CreatePackageInput) (*Package, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionSchemaManage); err != nil {
		return nil, err
	}
	input.IndustryKey = normalizeKey(input.IndustryKey)
	input.PackageKey = normalizeKey(input.PackageKey)
	if input.Version <= 0 {
		input.Version = 1
	}
	if input.Status == "" {
		input.Status = StatusDraft
	}
	pkg := Package{
		IndustryKey: input.IndustryKey,
		PackageKey:  input.PackageKey,
		Version:     input.Version,
		Name:        input.Name,
		Status:      input.Status,
		Assets:      input.Assets,
	}
	if err := ValidatePackage(pkg); err != nil {
		return nil, err
	}
	return s.repo.CreatePackage(ctx, input, actorID)
}

func (s *Service) ActivatePackage(ctx context.Context, actorID uuid.UUID, packageID uuid.UUID) (*Package, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionSchemaApprove); err != nil {
		return nil, err
	}
	return s.repo.ActivatePackage(ctx, packageID, actorID)
}

func (s *Service) ApplyPackageToOrganization(ctx context.Context, actorID uuid.UUID, input ApplyPackageInput) (*OrganizationAdoption, error) {
	if input.PackageID == uuid.Nil || input.OrganizationID == uuid.Nil {
		return nil, fmt.Errorf("%w: package_id and organization_id are required", ErrValidation)
	}
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionOrganizationManage); err != nil {
		if orgErr := s.requireOrganizationAdmin(ctx, actorID, input.OrganizationID); orgErr != nil {
			return nil, err
		}
	}
	pkg, err := s.repo.GetPackageByID(ctx, input.PackageID)
	if err != nil {
		return nil, err
	}
	if pkg == nil {
		return nil, ErrNotFound
	}
	if pkg.Status != StatusActive {
		return nil, fmt.Errorf("%w: only active industry packages can be applied", ErrValidation)
	}
	if err := ValidatePackage(*pkg); err != nil {
		return nil, err
	}
	modules := normalizeKeys(input.ModuleKeys)
	if len(modules) == 0 {
		modules = PackageModuleKeys(*pkg)
	}
	if err := s.validateModules(ctx, input.OrganizationID, pkg.IndustryKey, *pkg, modules); err != nil {
		return nil, err
	}
	input.ModuleKeys = modules
	return s.repo.UpsertAdoption(ctx, input, *pkg)
}

func (s *Service) GetAdoption(ctx context.Context, actorID uuid.UUID, orgID uuid.UUID) (*OrganizationAdoption, error) {
	if err := s.requireOrganizationAdmin(ctx, actorID, orgID); err != nil {
		return nil, err
	}
	return s.repo.GetAdoption(ctx, orgID)
}

func (s *Service) ValidateOrganizationModules(ctx context.Context, orgID uuid.UUID, modules []string) error {
	adoption, err := s.repo.GetAdoption(ctx, orgID)
	if err != nil || adoption == nil || adoption.PackageID == uuid.Nil {
		return nil
	}
	pkg, err := s.repo.GetPackageByID(ctx, adoption.PackageID)
	if err != nil {
		return err
	}
	return s.validateModules(ctx, orgID, adoption.IndustryKey, *pkg, normalizeKeys(modules))
}

func (s *Service) CreateExtension(ctx context.Context, actorID uuid.UUID, input CreateExtensionInput) (*Extension, error) {
	if err := s.requireOrganizationAdmin(ctx, actorID, input.OrganizationID); err != nil {
		return nil, err
	}
	input.ExtensionKey = normalizeKey(input.ExtensionKey)
	if input.OrganizationID == uuid.Nil || input.IndustryKey == "" || input.ExtensionKey == "" || input.Name == "" {
		return nil, fmt.Errorf("%w: organization_id, industry_key, extension_key, and name are required", ErrValidation)
	}
	if err := ValidatePackage(Package{IndustryKey: input.IndustryKey, PackageKey: input.ExtensionKey, Version: 1, Assets: input.Assets}); err != nil {
		return nil, err
	}
	return s.repo.CreateExtension(ctx, input, actorID)
}

func (s *Service) ListExtensions(ctx context.Context, actorID uuid.UUID, orgID uuid.UUID, limit int) ([]Extension, error) {
	if err := s.requireOrganizationAdmin(ctx, actorID, orgID); err != nil {
		return nil, err
	}
	return s.repo.ListExtensions(ctx, orgID, normalizeLimit(limit))
}

func (s *Service) SubmitPublicationRequest(ctx context.Context, actorID uuid.UUID, extensionID uuid.UUID, reason string) (*PublicationRequest, error) {
	extension, err := s.repo.GetExtension(ctx, extensionID)
	if err != nil {
		return nil, err
	}
	if extension == nil {
		return nil, ErrNotFound
	}
	if err := s.requireOrganizationAdmin(ctx, actorID, extension.OrganizationID); err != nil {
		return nil, err
	}
	return s.repo.CreatePublicationRequest(ctx, *extension, actorID, strings.TrimSpace(reason))
}

func (s *Service) ListPublicationRequests(ctx context.Context, actorID uuid.UUID, limit int) ([]PublicationRequest, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionPlatformRead); err != nil {
		return nil, err
	}
	return s.repo.ListPublicationRequests(ctx, normalizeLimit(limit))
}

func (s *Service) ReviewPublicationRequest(ctx context.Context, actorID uuid.UUID, requestID uuid.UUID, status string, reason string) (*PublicationRequest, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionSchemaApprove); err != nil {
		return nil, err
	}
	if status != PublicationApproved && status != PublicationRejected {
		return nil, fmt.Errorf("%w: review status must be approved or rejected", ErrValidation)
	}
	return s.repo.ReviewPublicationRequest(ctx, requestID, actorID, status, reason)
}

func (s *Service) ListKnowledgeSources(ctx context.Context, actorID uuid.UUID, industryKey string, orgID uuid.UUID, limit int) ([]KnowledgeSource, error) {
	if orgID != uuid.Nil {
		if err := s.requireOrganizationAdmin(ctx, actorID, orgID); err != nil {
			return nil, err
		}
	}
	return s.repo.ListKnowledgeSources(ctx, normalizeKey(industryKey), orgID, normalizeLimit(limit))
}

func (s *Service) validateModules(ctx context.Context, orgID uuid.UUID, industryKey string, pkg Package, modules []string) error {
	allowed := map[string]bool{}
	for _, key := range PackageModuleKeys(pkg) {
		allowed[key] = true
	}
	extensionModules, err := s.repo.ListOrganizationExtensionModules(ctx, orgID, industryKey)
	if err != nil {
		return err
	}
	for _, key := range extensionModules {
		allowed[normalizeKey(key)] = true
	}
	for _, key := range modules {
		if !allowed[key] {
			return fmt.Errorf("%w: module %s is not allowed by industry %s", ErrValidation, key, industryKey)
		}
	}
	return nil
}

func PackageModuleKeys(pkg Package) []string {
	seen := map[string]bool{}
	for _, asset := range pkg.Assets {
		if asset.AssetType != AssetTypeModule {
			continue
		}
		key := normalizeKey(stringFromPayload(asset.Payload, "module_key"))
		if key != "" {
			seen[key] = true
		}
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (s *Service) requirePlatformPermission(ctx context.Context, actorID uuid.UUID, permission string) error {
	if actorID == uuid.Nil {
		return ErrForbidden
	}
	role, err := s.repo.GetPlatformRole(ctx, actorID)
	if err != nil || !platformauth.HasPermission(role, permission) {
		return ErrForbidden
	}
	return nil
}

func (s *Service) requireOrganizationAdmin(ctx context.Context, actorID uuid.UUID, orgID uuid.UUID) error {
	if actorID == uuid.Nil || orgID == uuid.Nil {
		return ErrForbidden
	}
	if role, err := s.repo.GetPlatformRole(ctx, actorID); err == nil && platformauth.HasPermission(role, platformauth.PermissionOrganizationManage) {
		return nil
	}
	authority, err := s.repo.GetOrganizationAuthority(ctx, actorID, orgID)
	if err != nil {
		return ErrForbidden
	}
	switch authority {
	case "organization_creator", "organization_admin":
		return nil
	default:
		return ErrForbidden
	}
}

func decodePayload(payload map[string]any, target any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func mustMap(value any) map[string]any {
	data, _ := json.Marshal(value)
	out := map[string]any{}
	_ = json.Unmarshal(data, &out)
	return out
}

func stringFromPayload(payload map[string]any, key string) string {
	value, _ := payload[key].(string)
	return strings.TrimSpace(value)
}

func normalizeKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeKeys(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		key := normalizeKey(value)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

func normalizeLimit(limit int) int {
	if limit <= 0 || limit > 200 {
		return 100
	}
	return limit
}
