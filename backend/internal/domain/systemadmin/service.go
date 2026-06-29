package systemadmin

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/erp"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/passwordhash"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/platformauth"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/tenantdb"
)

var (
	ErrForbidden         = errors.New("forbidden")
	ErrValidation        = errors.New("validation error")
	ErrInvalidTransition = errors.New("invalid industry solution change status transition")
)

type Service struct {
	repo repository
}

type repository interface {
	GetPlatformRole(context.Context, uuid.UUID) (string, error)
	ListPlatformMasters(context.Context, string, int) ([]PlatformMaster, error)
	ListPlatformDetails(context.Context, string) ([]PlatformDetail, error)
	ListPlatformFeatures(context.Context, string, int) ([]PlatformFeature, error)
	CreatePlatformFeature(context.Context, CreatePlatformFeatureRecord) (*PlatformFeature, error)
	UpdatePlatformFeatureStatus(context.Context, string, string, uuid.UUID) (*PlatformFeature, error)
	ListPlatformMenuItems(context.Context, int) ([]PlatformMenuItem, error)
	ListPlatformPermissions(context.Context) ([]PlatformPermission, error)
	ListPlatformRoles(context.Context) ([]PlatformRole, error)
	ListPlatformRolePermissions(context.Context, string) ([]string, error)
	SetPlatformRolePermissions(context.Context, string, []string, uuid.UUID) (*PlatformRole, error)
	ListPlatformUsers(context.Context, int) ([]PlatformUser, error)
	CreatePlatformUser(context.Context, CreatePlatformUserRecord) (*PlatformUser, error)
	SetPlatformUserRoles(context.Context, uuid.UUID, []string, uuid.UUID) (*PlatformUser, error)
	ResetPlatformUserPassword(context.Context, uuid.UUID, string, uuid.UUID) (*PlatformUser, error)
	DisablePlatformUser(context.Context, uuid.UUID, uuid.UUID) (*PlatformUser, error)
	ListDatabaseMaintenanceJobs(context.Context, int) ([]DatabaseMaintenanceJob, error)
	CreateDatabaseMaintenanceJob(context.Context, CreateDatabaseMaintenanceJobRecord) (*DatabaseMaintenanceJob, error)
	ReviewDatabaseMaintenanceJob(context.Context, ReviewDatabaseMaintenanceJobRecord) (*DatabaseMaintenanceJob, error)
	ListIndustrySolutionTargets(context.Context, int) ([]OrganizationIndustrySolutionTarget, error)
	GetIndustrySolutionTarget(context.Context, uuid.UUID) (*OrganizationIndustrySolutionTarget, error)
	CreateIndustrySolutionChangeRequest(context.Context, CreateIndustrySolutionChangeRequestRecord) (*IndustrySolutionChangeRequest, error)
	GetIndustrySolutionChangeRequest(context.Context, uuid.UUID) (*IndustrySolutionChangeRequest, error)
	UpdateIndustrySolutionChangeRequestStatus(context.Context, uuid.UUID, string, uuid.UUID, string) (*IndustrySolutionChangeRequest, error)
	ApplyIndustrySolutionChange(context.Context, *IndustrySolutionChangeRequest, []string, []IndustrySolutionApplyAssetResult) (*IndustrySolutionApplyJob, error)
}

func NewService(repo repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetPermissionProfile(ctx context.Context, actorID uuid.UUID) (*PlatformPermissionProfile, error) {
	normalized, permissions, err := s.permissionsForActor(ctx, actorID)
	if err != nil {
		return nil, err
	}
	if len(permissions) == 0 {
		return nil, ErrForbidden
	}
	menuItems, _ := s.repo.ListPlatformMenuItems(ctx, 500)
	return &PlatformPermissionProfile{
		Role:        normalized,
		Permissions: permissions,
		MenuItems:   menuItemsForPermissions(permissions, menuItems),
	}, nil
}

func (s *Service) ListPlatformMasters(ctx context.Context, actorID uuid.UUID, moduleKey string, limit int) ([]PlatformMaster, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionPlatformRead); err != nil {
		return nil, err
	}
	return s.repo.ListPlatformMasters(ctx, moduleKey, limit)
}

func menuItemsForPermissions(permissions map[string]bool, catalog []PlatformMenuItem) []string {
	if len(catalog) > 0 {
		items := make([]string, 0, len(catalog))
		for _, item := range catalog {
			if item.Status != "" && item.Status != "active" {
				continue
			}
			if platformMenuItemAllowed(item, permissions) {
				items = append(items, item.MenuKey)
			}
		}
		return items
	}
	items := []string{}
	if permissions[platformauth.PermissionPlatformRead] {
		items = append(items, "saas", "catalog", "targets", "assistant")
	}
	if permissions[platformauth.PermissionPlatformFeatureManage] {
		items = append(items, "platform.features")
	}
	if permissions[platformauth.PermissionPlatformUserManage] {
		items = append(items, "platform.users")
	}
	if permissions[platformauth.PermissionPlatformRBACManage] {
		items = append(items, "platform.rbac")
	}
	if permissions[platformauth.PermissionDatabaseMaintenanceManage] {
		items = append(items, "platform.database")
	}
	if permissions[platformauth.PermissionOrganizationManage] || permissions[platformauth.PermissionOrganizationClose] {
		items = append(items, "organizations")
	}
	if permissions[platformauth.PermissionModelManage] {
		items = append(items, "models")
	}
	if permissions[platformauth.PermissionRuntimeManage] {
		items = append(items, "runtime")
	}
	if permissions[platformauth.PermissionIndustrySolutionManage] {
		items = append(items, "schema")
	}
	return items
}

func platformMenuItemAllowed(item PlatformMenuItem, permissions map[string]bool) bool {
	if len(item.RequiredPermissions) == 0 {
		return true
	}
	for _, permission := range item.RequiredPermissions {
		if permissions[permission] {
			return true
		}
	}
	return false
}

func (s *Service) ListPlatformDetails(ctx context.Context, actorID uuid.UUID, masterKey string) ([]PlatformDetail, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionPlatformRead); err != nil {
		return nil, err
	}
	if masterKey == "" {
		return nil, fmt.Errorf("%w: master_key is required", ErrValidation)
	}
	return s.repo.ListPlatformDetails(ctx, masterKey)
}

func (s *Service) ListPlatformFeatures(ctx context.Context, actorID uuid.UUID, status string, limit int) ([]PlatformFeature, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionPlatformRead); err != nil {
		return nil, err
	}
	return s.repo.ListPlatformFeatures(ctx, strings.TrimSpace(status), limit)
}

func (s *Service) CreatePlatformFeature(ctx context.Context, actorID uuid.UUID, input CreatePlatformFeatureInput) (*PlatformFeature, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionPlatformFeatureManage); err != nil {
		return nil, err
	}
	input.FeatureKey = strings.TrimSpace(input.FeatureKey)
	input.ParentKey = strings.TrimSpace(input.ParentKey)
	input.ModuleKey = strings.TrimSpace(input.ModuleKey)
	input.Category = strings.TrimSpace(input.Category)
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)
	if input.FeatureKey == "" || input.ModuleKey == "" || input.Title == "" {
		return nil, fmt.Errorf("%w: feature_key, module_key, and title are required", ErrValidation)
	}
	if input.Category == "" {
		input.Category = "platform_admin"
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
	if err := validateMetadataOnlyExtension(input.Metadata); err != nil {
		return nil, err
	}
	if _, ok := input.Metadata["extension_mode"]; !ok {
		input.Metadata["extension_mode"] = "metadata_only"
	}
	return s.repo.CreatePlatformFeature(ctx, CreatePlatformFeatureRecord{
		CreatePlatformFeatureInput: input,
		Status:                     "draft",
		ActorID:                    actorID,
	})
}

func (s *Service) PublishPlatformFeature(ctx context.Context, actorID uuid.UUID, featureKey string) (*PlatformFeature, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionPlatformFeatureManage); err != nil {
		return nil, err
	}
	featureKey = strings.TrimSpace(featureKey)
	if featureKey == "" {
		return nil, fmt.Errorf("%w: feature_key is required", ErrValidation)
	}
	return s.repo.UpdatePlatformFeatureStatus(ctx, featureKey, "active", actorID)
}

func (s *Service) ListPlatformPermissions(ctx context.Context, actorID uuid.UUID) ([]PlatformPermission, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionPlatformRead); err != nil {
		return nil, err
	}
	return s.repo.ListPlatformPermissions(ctx)
}

func (s *Service) ListPlatformRoles(ctx context.Context, actorID uuid.UUID) ([]PlatformRole, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionPlatformRead); err != nil {
		return nil, err
	}
	return s.repo.ListPlatformRoles(ctx)
}

func (s *Service) SetPlatformRolePermissions(ctx context.Context, actorID uuid.UUID, roleKey string, input SetPlatformRolePermissionsInput) (*PlatformRole, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionPlatformRBACManage); err != nil {
		return nil, err
	}
	roleKey = platformauth.NormalizeRole(roleKey)
	if roleKey == "" {
		return nil, fmt.Errorf("%w: role_key is required", ErrValidation)
	}
	permissions := normalizeStringList(input.PermissionKeys)
	if len(permissions) == 0 {
		return nil, fmt.Errorf("%w: permission_keys are required", ErrValidation)
	}
	return s.repo.SetPlatformRolePermissions(ctx, roleKey, permissions, actorID)
}

func (s *Service) ListPlatformUsers(ctx context.Context, actorID uuid.UUID, limit int) ([]PlatformUser, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionPlatformUserManage); err != nil {
		return nil, err
	}
	return s.repo.ListPlatformUsers(ctx, limit)
}

func (s *Service) CreatePlatformUser(ctx context.Context, actorID uuid.UUID, input CreatePlatformUserInput) (*CreatePlatformUserResponse, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionPlatformUserManage); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(input.Name)
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if name == "" || email == "" {
		return nil, fmt.Errorf("%w: name and email are required", ErrValidation)
	}
	roles := normalizePlatformRoles(input.Roles)
	if len(roles) == 0 {
		roles = []string{platformauth.RoleAuditor}
	}
	temporaryPassword, err := generateTemporaryPassword()
	if err != nil {
		return nil, err
	}
	hash, err := passwordhash.GenerateBcryptHash(temporaryPassword, 0)
	if err != nil {
		return nil, err
	}
	user, err := s.repo.CreatePlatformUser(ctx, CreatePlatformUserRecord{
		Name:         name,
		Email:        email,
		PasswordHash: hash,
		Roles:        roles,
		Metadata:     input.Metadata,
		ActorID:      actorID,
	})
	if err != nil {
		return nil, err
	}
	return &CreatePlatformUserResponse{User: *user, TemporaryPassword: temporaryPassword}, nil
}

func (s *Service) SetPlatformUserRoles(ctx context.Context, actorID uuid.UUID, userID uuid.UUID, roles []string) (*PlatformUser, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionPlatformUserManage); err != nil {
		return nil, err
	}
	if userID == uuid.Nil {
		return nil, fmt.Errorf("%w: user_id is required", ErrValidation)
	}
	normalizedRoles := normalizePlatformRoles(roles)
	if len(normalizedRoles) == 0 {
		return nil, fmt.Errorf("%w: roles are required", ErrValidation)
	}
	return s.repo.SetPlatformUserRoles(ctx, userID, normalizedRoles, actorID)
}

func (s *Service) ResetPlatformUserPassword(ctx context.Context, actorID uuid.UUID, userID uuid.UUID) (*ResetPlatformUserPasswordResponse, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionPlatformUserManage); err != nil {
		return nil, err
	}
	if userID == uuid.Nil {
		return nil, fmt.Errorf("%w: user_id is required", ErrValidation)
	}
	temporaryPassword, err := generateTemporaryPassword()
	if err != nil {
		return nil, err
	}
	hash, err := passwordhash.GenerateBcryptHash(temporaryPassword, 0)
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.ResetPlatformUserPassword(ctx, userID, hash, actorID); err != nil {
		return nil, err
	}
	return &ResetPlatformUserPasswordResponse{UserID: userID, TemporaryPassword: temporaryPassword}, nil
}

func (s *Service) DisablePlatformUser(ctx context.Context, actorID uuid.UUID, userID uuid.UUID) (*PlatformUser, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionPlatformUserManage); err != nil {
		return nil, err
	}
	if userID == uuid.Nil {
		return nil, fmt.Errorf("%w: user_id is required", ErrValidation)
	}
	return s.repo.DisablePlatformUser(ctx, userID, actorID)
}

func (s *Service) ListDatabaseMaintenanceJobs(ctx context.Context, actorID uuid.UUID, limit int) ([]DatabaseMaintenanceJob, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionDatabaseMaintenanceManage); err != nil {
		return nil, err
	}
	return s.repo.ListDatabaseMaintenanceJobs(ctx, limit)
}

func (s *Service) CreateDatabaseMaintenanceJob(ctx context.Context, actorID uuid.UUID, input CreateDatabaseMaintenanceJobInput) (*DatabaseMaintenanceJob, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionDatabaseMaintenanceManage); err != nil {
		return nil, err
	}
	input.JobType = strings.ToLower(strings.TrimSpace(input.JobType))
	input.Scope = strings.ToLower(strings.TrimSpace(input.Scope))
	input.Reason = strings.TrimSpace(input.Reason)
	input.BackupRef = strings.TrimSpace(input.BackupRef)
	if input.Scope == "" {
		input.Scope = "platform"
	}
	if input.JobType != "backup" && input.JobType != "restore" {
		return nil, fmt.Errorf("%w: job_type must be backup or restore", ErrValidation)
	}
	if input.JobType == "restore" && input.BackupRef == "" {
		return nil, fmt.Errorf("%w: backup_ref is required for restore", ErrValidation)
	}
	if input.Reason == "" {
		return nil, fmt.Errorf("%w: reason is required", ErrValidation)
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
	if err := applyDatabaseMaintenanceScopeMetadata(input.Scope, input.Metadata); err != nil {
		return nil, err
	}
	return s.repo.CreateDatabaseMaintenanceJob(ctx, CreateDatabaseMaintenanceJobRecord{
		CreateDatabaseMaintenanceJobInput: input,
		Status:                            DatabaseMaintenancePendingApproval,
		RequestedBy:                       actorID,
	})
}

func (s *Service) CreatePrivateDeploymentExportJob(ctx context.Context, actorID uuid.UUID, orgID uuid.UUID) (*DatabaseMaintenanceJob, error) {
	if orgID == uuid.Nil {
		return nil, fmt.Errorf("%w: organization_id is required", ErrValidation)
	}
	return s.CreateDatabaseMaintenanceJob(ctx, actorID, CreateDatabaseMaintenanceJobInput{
		JobType: "backup",
		Scope:   "tenant_database:" + orgID.String(),
		Reason:  "Reserve tenant private deployment export package generation",
		Metadata: map[string]any{
			"export_purpose":   "private_deployment",
			"execution_mode":   "metadata_only_reserved",
			"package_format":   "tenant_private_deployment_bundle",
			"includes":         []string{"tenant_business_database", "module_entitlements", "industry_solution_manifest", "tenant_database_migration_state"},
			"deferred_actions": []string{"pg_dump_generation", "package_signing", "private_runtime_import"},
		},
	})
}

func applyDatabaseMaintenanceScopeMetadata(scope string, metadata map[string]any) error {
	switch {
	case scope == "platform" || scope == "platform_control":
		metadata["target_scope"] = "platform_control"
	case scope == "all_tenants" || scope == "tenant_databases":
		metadata["target_scope"] = "tenant_databases"
	case strings.HasPrefix(scope, "tenant_database:"):
		rawID := strings.TrimPrefix(scope, "tenant_database:")
		orgID, err := uuid.Parse(rawID)
		if err != nil {
			return fmt.Errorf("%w: tenant_database scope must be tenant_database:<organization_id>", ErrValidation)
		}
		metadata["target_scope"] = "tenant_database"
		metadata["organization_id"] = orgID.String()
	default:
		return fmt.Errorf("%w: scope must be platform, tenant_databases, or tenant_database:<organization_id>", ErrValidation)
	}
	return nil
}

func (s *Service) ReviewDatabaseMaintenanceJob(ctx context.Context, actorID uuid.UUID, jobID uuid.UUID, input ReviewDatabaseMaintenanceJobInput) (*DatabaseMaintenanceJob, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionDatabaseMaintenanceApprove); err != nil {
		return nil, err
	}
	if jobID == uuid.Nil {
		return nil, fmt.Errorf("%w: job_id is required", ErrValidation)
	}
	decision := strings.ToLower(strings.TrimSpace(input.Decision))
	status := ""
	switch decision {
	case "approve", "approved":
		status = DatabaseMaintenanceApproved
	case "reject", "rejected":
		status = DatabaseMaintenanceRejected
	default:
		return nil, fmt.Errorf("%w: decision must be approve or reject", ErrValidation)
	}
	return s.repo.ReviewDatabaseMaintenanceJob(ctx, ReviewDatabaseMaintenanceJobRecord{
		JobID:        jobID,
		Status:       status,
		ReviewedBy:   actorID,
		ReviewReason: strings.TrimSpace(input.Reason),
	})
}

func (s *Service) ListIndustrySolutionTargets(ctx context.Context, actorID uuid.UUID, limit int) ([]OrganizationIndustrySolutionTarget, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionPlatformRead); err != nil {
		return nil, err
	}
	return s.repo.ListIndustrySolutionTargets(ctx, limit)
}

func (s *Service) ExportOrganizationIndustrySolutionManifest(ctx context.Context, actorID uuid.UUID, orgID uuid.UUID) (*IndustrySolutionManifest, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionIndustrySolutionExport); err != nil {
		return nil, err
	}
	if orgID == uuid.Nil {
		return nil, fmt.Errorf("%w: organization_id is required", ErrValidation)
	}
	manifest := DefaultOrganizationIndustrySolutionManifest()
	return &manifest, nil
}

func (s *Service) CreateIndustrySolutionChangeRequest(ctx context.Context, actorID uuid.UUID, input CreateIndustrySolutionChangeRequestInput) (*IndustrySolutionChangeRequest, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionIndustrySolutionManage); err != nil {
		return nil, err
	}
	if input.OrganizationID == uuid.Nil {
		return nil, fmt.Errorf("%w: organization_id is required", ErrValidation)
	}
	if input.RequestType == "" {
		input.RequestType = "industry_solution_manifest_import"
	}
	if err := ValidateIndustrySolutionManifest(input.SolutionManifest); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	schemaName := tenantdb.SchemaNameForOrganization(input.OrganizationID)
	riskLevel := IndustrySolutionRiskSafe
	diff := []IndustrySolutionDiff{{Action: "create_or_ensure_tables", Risk: IndustrySolutionRiskSafe}}
	var statements []string
	if input.CurrentSolutionManifest != nil {
		plan, err := BuildIndustrySolutionMigrationPlan(schemaName, *input.CurrentSolutionManifest, input.SolutionManifest)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrValidation, err)
		}
		statements = plan.Statements
		riskLevel = plan.RiskLevel
		diff = plan.Diff
	} else {
		var err error
		statements, err = BuildIndustrySolutionTableStatements(schemaName, input.SolutionManifest)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrValidation, err)
		}
	}
	return s.repo.CreateIndustrySolutionChangeRequest(ctx, CreateIndustrySolutionChangeRequestRecord{
		OrganizationID:   input.OrganizationID,
		TargetSchemaName: schemaName,
		RequestType:      input.RequestType,
		Reason:           input.Reason,
		SolutionManifest: input.SolutionManifest,
		Statements:       statements,
		RiskLevel:        riskLevel,
		Diff:             diff,
		RequestedBy:      actorID,
	})
}

func (s *Service) CreateIndustrySolutionTableFieldChange(ctx context.Context, actorID uuid.UUID, input CreateIndustrySolutionTableFieldChangeInput) (*IndustrySolutionChangeRequest, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionIndustrySolutionManage); err != nil {
		return nil, err
	}
	if input.OrganizationID == uuid.Nil {
		return nil, fmt.Errorf("%w: organization_id is required", ErrValidation)
	}
	input.IndustryKey = strings.TrimSpace(input.IndustryKey)
	input.PackageKey = strings.TrimSpace(input.PackageKey)
	if input.IndustryKey == "" || input.PackageKey == "" {
		return nil, fmt.Errorf("%w: industry_key and package_key are required", ErrValidation)
	}
	manifest, err := BuildIndustrySolutionTableFieldManifest(input)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	reason := strings.TrimSpace(input.Reason)
	if reason == "" {
		reason = "Update industry solution table and field definition"
	}
	return s.CreateIndustrySolutionChangeRequest(ctx, actorID, CreateIndustrySolutionChangeRequestInput{
		OrganizationID:          input.OrganizationID,
		RequestType:             "industry_solution_table_field_change",
		Reason:                  reason,
		SolutionManifest:        manifest,
		CurrentSolutionManifest: input.CurrentSolutionManifest,
	})
}

func BuildIndustrySolutionTableFieldManifest(input CreateIndustrySolutionTableFieldChangeInput) (IndustrySolutionManifest, error) {
	table := input.Table
	table.Name = strings.TrimSpace(table.Name)
	table.PreviousName = strings.TrimSpace(table.PreviousName)
	table.DisplayName = strings.TrimSpace(table.DisplayName)
	if table.Name == "" {
		return IndustrySolutionManifest{}, fmt.Errorf("table.name is required")
	}
	fields := []IndustrySolutionFieldDefinition{
		{Name: "id", DataType: "uuid", PrimaryKey: true, Default: "gen_random_uuid()"},
	}
	for _, fieldInput := range table.Fields {
		fieldInput.Name = strings.TrimSpace(fieldInput.Name)
		fieldInput.PreviousName = strings.TrimSpace(fieldInput.PreviousName)
		fieldInput.DataType = strings.TrimSpace(fieldInput.DataType)
		fieldInput.Default = strings.TrimSpace(fieldInput.Default)
		if fieldInput.Name == "" || fieldInput.DataType == "" {
			return IndustrySolutionManifest{}, fmt.Errorf("field name and data_type are required")
		}
		if fieldInput.Name == "id" {
			continue
		}
		fields = append(fields, IndustrySolutionFieldDefinition{
			Name:         fieldInput.Name,
			PreviousName: fieldInput.PreviousName,
			DataType:     fieldInput.DataType,
			Nullable:     fieldInput.Nullable,
			Default:      fieldInput.Default,
		})
	}
	if len(fields) == 1 {
		return IndustrySolutionManifest{}, fmt.Errorf("at least one non-id field is required")
	}
	metadata := copyMap(table.Metadata)
	if table.DisplayName != "" {
		metadata["display_name"] = table.DisplayName
	}
	manifest := IndustrySolutionManifest{
		FormatVersion: IndustrySolutionManifestFormatVersion,
		ModuleKey:     "industry_solution",
		Tables: []IndustrySolutionTableDefinition{
			{
				Name:         table.Name,
				PreviousName: table.PreviousName,
				Fields:       fields,
				Metadata:     metadata,
			},
		},
		Metadata: map[string]any{
			"industry_key": input.IndustryKey,
			"package_key":  input.PackageKey,
			"source":       "platform_industry_solution_table_field_editor",
		},
	}
	if err := ValidateIndustrySolutionManifest(manifest); err != nil {
		return IndustrySolutionManifest{}, err
	}
	return manifest, nil
}

func (s *Service) BuildERPSolutionFlow(ctx context.Context, actorID uuid.UUID, input ERPSolutionFlowRequest) (*IndustrySolutionChangeRequest, error) {
	if input.IndustryKey == "" {
		input.IndustryKey = "standard_erp"
	}
	if input.PackageKey == "" {
		input.PackageKey = "erp_standard"
	}
	if input.Name == "" {
		input.Name = "ERP Standard"
	}
	if len(input.EnabledModules) == 0 {
		input.EnabledModules = []string{"project", "procurement", "inventory", "sales", "finance"}
	}
	solutionManifest := BuildERPSolutionManifest(input)
	desiredAssetManifest, err := AssetManifestFromSolutionManifest(solutionManifest)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	currentAssetManifest := IndustrySolutionAssetManifest{ManifestVersion: IndustryManifestVersion, IndustryKey: input.IndustryKey, PackageKey: input.PackageKey}
	if input.CurrentTemplate != nil {
		if parsed, err := AssetManifestFromSolutionManifest(*input.CurrentTemplate); err == nil {
			currentAssetManifest = parsed
		}
	}
	solutionManifest.Metadata["package_diff"] = BuildIndustrySolutionAssetDiff(currentAssetManifest, desiredAssetManifest)
	return s.CreateIndustrySolutionChangeRequest(ctx, actorID, CreateIndustrySolutionChangeRequestInput{
		OrganizationID:          input.OrganizationID,
		RequestType:             "erp_solution_flow",
		Reason:                  "Create ERP standard industry solution flow",
		SolutionManifest:        solutionManifest,
		CurrentSolutionManifest: input.CurrentTemplate,
	})
}

func (s *Service) BuildRetailDistributionSolutionFlow(ctx context.Context, actorID uuid.UUID, input ERPSolutionFlowRequest) (*IndustrySolutionChangeRequest, error) {
	if input.IndustryKey == "" {
		input.IndustryKey = "retail_chain_distribution"
	}
	if input.PackageKey == "" {
		input.PackageKey = "retail_distribution_v1"
	}
	if input.Name == "" {
		input.Name = "Retail Distribution v1"
	}
	input.EnabledModules = []string{"master_data", "procurement", "inventory", "sales", "finance", "retail"}
	solutionManifest := BuildRetailDistributionSolutionManifest(input)
	desiredAssetManifest, err := AssetManifestFromSolutionManifest(solutionManifest)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	currentAssetManifest := IndustrySolutionAssetManifest{ManifestVersion: IndustryManifestVersion, IndustryKey: input.IndustryKey, PackageKey: input.PackageKey}
	if input.CurrentTemplate != nil {
		if parsed, err := AssetManifestFromSolutionManifest(*input.CurrentTemplate); err == nil {
			currentAssetManifest = parsed
		}
	}
	solutionManifest.Metadata["package_diff"] = BuildIndustrySolutionAssetDiff(currentAssetManifest, desiredAssetManifest)
	return s.CreateIndustrySolutionChangeRequest(ctx, actorID, CreateIndustrySolutionChangeRequestInput{
		OrganizationID:          input.OrganizationID,
		RequestType:             "retail_distribution_solution_flow",
		Reason:                  "Create retail distribution industry solution flow",
		SolutionManifest:        solutionManifest,
		CurrentSolutionManifest: input.CurrentTemplate,
	})
}

func BuildERPSolutionManifest(input ERPSolutionFlowRequest) IndustrySolutionManifest {
	catalog := erp.DefaultCatalog()
	actions := erp.DefaultActionRegistry().List()
	enabled := make([]string, 0, len(input.EnabledModules))
	for _, module := range input.EnabledModules {
		trimmed := strings.TrimSpace(module)
		if trimmed != "" {
			enabled = append(enabled, trimmed)
		}
	}
	databaseAssets := make([]map[string]any, 0, len(catalog.Tables))
	for _, table := range catalog.Tables {
		childTables := make([]string, 0, len(table.Children))
		for _, child := range table.Children {
			childTables = append(childTables, child.Code)
		}
		databaseAssets = append(databaseAssets, map[string]any{
			"table_code":   table.Code,
			"name":         table.Name,
			"module":       table.Module,
			"primary_key":  table.PrimaryKey,
			"child_tables": childTables,
		})
	}
	businessFunctions := make([]map[string]any, 0, len(actions))
	for _, action := range actions {
		businessFunctions = append(businessFunctions, map[string]any{
			"table_code":  action.TableCode,
			"action":      action.Action,
			"label":       action.Label,
			"next_tables": action.NextTables,
		})
	}
	runtimeOperations := buildERPStandardRuntimeOperations(catalog, actions, enabled)
	apiOperations := make([]string, 0, len(runtimeOperations)+3)
	apiOperations = append(apiOperations, "/erp/catalog", "/erp/actions", "/erp/{tableCode}")
	seenAPIOperations := map[string]bool{}
	for _, operation := range apiOperations {
		seenAPIOperations[operation] = true
	}
	for _, operation := range runtimeOperations {
		path := stringValue(operation["path"])
		if path != "" && !seenAPIOperations[path] {
			apiOperations = append(apiOperations, path)
			seenAPIOperations[path] = true
		}
	}
	toolDefinitions := []map[string]any{
		{
			"tool_key":    "erp.action.execute",
			"entrypoint":  "/erp/{tableCode}/{key}/actions/{action}",
			"policy":      "toolruntime.approval_required_for_high_risk",
			"permissions": []string{"erp:action"},
			"observed_by": []string{"tool_execution", "erp_action", "audit_log"},
			"idempotency": "table_code:key:action",
			"risk_level":  "medium",
		},
		{
			"tool_key":    "industry.solution.change.preview",
			"entrypoint":  "/platform/admin/industry-solution-change-requests/{id}/verify",
			"policy":      "industry.solution.manage",
			"permissions": []string{"industry.solution.manage"},
			"risk_level":  "low",
		},
		{
			"tool_key":    "runtime.operation.execute",
			"entrypoint":  "/platform/runtime/operations/{operation_id}",
			"policy":      "runtime.manage",
			"permissions": []string{"runtime.manage"},
			"risk_level":  "medium",
		},
		{
			"tool_key":    "context.proposal.apply",
			"entrypoint":  "/platform/context-change-proposals/{id}/apply",
			"policy":      "context_rule_human_approval",
			"permissions": []string{"assistant:erp"},
			"risk_level":  "high",
		},
	}
	for _, action := range actions {
		entrypoint := fmt.Sprintf("/erp/%s/{key}/actions/%s", action.TableCode, action.Action)
		policy := "erp_action_state_gate"
		riskLevel := "medium"
		if action.TableCode == "MGLR" && action.Action == "run" {
			entrypoint = "/finance/gl/trial-balance"
			policy = "erp_report_read"
			riskLevel = "low"
		}
		toolDefinitions = append(toolDefinitions, map[string]any{
			"tool_key":    fmt.Sprintf("erp.%s.%s", strings.ToLower(action.TableCode), action.Action),
			"table_code":  action.TableCode,
			"action":      action.Action,
			"entrypoint":  entrypoint,
			"policy":      policy,
			"permissions": []string{"erp:action"},
			"idempotency": fmt.Sprintf("%s:key:%s", action.TableCode, action.Action),
			"next_tables": action.NextTables,
			"risk_level":  riskLevel,
		})
	}
	return IndustrySolutionManifest{
		FormatVersion: IndustrySolutionManifestFormatVersion,
		ModuleKey:     "erp_standard",
		Tables: []IndustrySolutionTableDefinition{
			erpSolutionAssetTable("erp_solution_database_assets"),
			erpSolutionAssetTable("erp_solution_business_functions"),
			erpSolutionAssetTable("erp_solution_process_loops"),
			erpSolutionAssetTable("erp_solution_ui_metadata"),
			erpSolutionAssetTable("erp_solution_assistant_targets"),
			erpSolutionAssetTable("erp_solution_context_rules"),
			erpSolutionAssetTable("erp_solution_tool_definitions"),
			erpSolutionAssetTable("erp_solution_assistant_skills"),
			erpSolutionAssetTable("erp_solution_quality_gates"),
			erpSolutionAssetTable("erp_solution_verification_scenarios"),
		},
		Metadata: func() map[string]any {
			metadata := map[string]any{
				"industry_key":       input.IndustryKey,
				"package_key":        input.PackageKey,
				"name":               input.Name,
				"enabled_modules":    enabled,
				"database_assets":    databaseAssets,
				"business_functions": businessFunctions,
				"process_loops": []map[string]any{
					{"key": "requirement_to_project", "steps": []string{"MREQ.analyze", "MREQ.approve", "MREQ.convert-to-project", "MPRJ.refresh-cost", "MPRJ.close-feedback"}},
					{"key": "procure_to_pay", "steps": []string{"MPOR.submit", "MPOR.approve", "MPDN.post", "MPCH"}},
					{"key": "order_to_cash", "steps": []string{"MRDR.confirm", "MRDR.approve", "MDLN.post", "MINV.post", "MRCT.allocate"}},
					{"key": "inventory_to_finance", "steps": []string{"MIGN.post", "MIGE.post", "MJDT.post"}},
					{"key": "finance_close", "steps": []string{"MACT", "MPRC", "MJDT.post", "MGLR.run"}},
				},
				"permissions":        []string{"erp:read", "erp:write", "erp:action", "erp:admin", "assistant:erp"},
				"api_operations":     apiOperations,
				"runtime_operations": runtimeOperations,
				"ui_workspaces":      []string{"project", "procurement", "sales", "inventory", "finance", "developer_erp_code"},
				"assistant_targets":  []string{"requirement", "project", "purchase_order", "sales_order", "ar_invoice", "ap_invoice", "gl_account", "cost_center", "journal_entry", "trial_balance"},
				"context_rules": []map[string]any{
					{
						"key":                  "erp_document_state_context",
						"scope":                "erp",
						"source_tables":        []string{"MREQ", "MPRJ", "MPOR", "MPDN", "MRDR", "MDLN", "MINV", "MRCT", "MIGN", "MIGE", "MACT", "MPRC", "MJDT", "MGLR"},
						"required_permissions": []string{"erp:read"},
						"workflow_stages":      []string{"draft", "submitted", "approved", "posted", "closed"},
						"attention_budget":     "document_timeline",
					},
					{
						"key":                  "erp_finance_validation_context",
						"scope":                "finance",
						"source_tables":        []string{"MCST", "MINV", "MPCH", "MRCT", "MACT", "MPRC", "MJDT", "MGLR"},
						"required_permissions": []string{"erp:read", "assistant:erp"},
						"workflow_stages":      []string{"cost_refresh", "invoice_posting", "payment_allocation", "journal_posting", "trial_balance"},
						"attention_budget":     "finance_close",
					},
					{
						"key":                  "erp_governance_approval_context",
						"scope":                "governance",
						"source_tables":        []string{"MPOR", "MRDR", "MDLN", "MPDN"},
						"required_permissions": []string{"erp:action"},
						"workflow_stages":      []string{"submit", "approve", "post"},
						"attention_budget":     "approval_risk",
					},
				},
				"tool_definitions": toolDefinitions,
				"assistant_skills": []map[string]any{
					{
						"skill_key":     "erp_requirement_to_project",
						"targets":       []string{"requirement", "project"},
						"context_rules": []string{"erp_document_state_context", "erp_finance_validation_context"},
						"allowed_tools": []string{"erp.mreq.analyze", "erp.mreq.approve", "erp.mreq.convert-to-project", "erp.mprj.refresh-cost"},
					},
					{
						"skill_key":     "erp_source_to_pay",
						"targets":       []string{"purchase_order", "ap_invoice"},
						"context_rules": []string{"erp_document_state_context", "erp_governance_approval_context"},
						"allowed_tools": []string{"erp.mpor.submit", "erp.mpor.approve", "erp.mpdn.post"},
					},
					{
						"skill_key":     "erp_order_to_cash",
						"targets":       []string{"sales_order", "ar_invoice"},
						"context_rules": []string{"erp_document_state_context", "erp_finance_validation_context"},
						"allowed_tools": []string{"erp.mrdr.confirm", "erp.mrdr.approve", "erp.mdln.post", "erp.minv.post", "erp.mrct.allocate"},
					},
					{
						"skill_key":     "erp_finance_close",
						"targets":       []string{"gl_account", "cost_center", "journal_entry", "trial_balance"},
						"context_rules": []string{"erp_finance_validation_context"},
						"allowed_tools": []string{"erp.mjdt.post", "erp.mglr.run"},
					},
					{
						"skill_key":     "industry_solution_change_reviewer",
						"targets":       []string{"industry_solution_change", "industry_solution"},
						"context_rules": []string{"erp_governance_approval_context"},
						"allowed_tools": []string{"industry.solution.change.preview"},
					},
				},
				"quality_gates": []map[string]any{
					{
						"gate_key":        "solution_verify_before_apply",
						"stage":           "industry_solution_change",
						"required_checks": []string{"solution_manifest", "ddl_plan", "permissions_impact", "runtime_operations", "assistant_context", "verification_scenarios"},
					},
					{
						"gate_key":        "tool_policy_before_execution",
						"stage":           "tool_runtime",
						"required_checks": []string{"state_precondition", "policy", "approval", "idempotency"},
					},
					{
						"gate_key":        "context_rule_human_activation",
						"stage":           "context_change",
						"required_checks": []string{"permission_scope", "workflow_stage", "finance_validation", "attention_budget"},
					},
				},
				"verification_scenarios": []map[string]any{
					{
						"scenario_key": "requirement_to_project_smoke",
						"steps":        []string{"MREQ.analyze", "MREQ.approve", "MREQ.convert-to-project", "MPRJ.refresh-cost", "MPRJ.close-feedback"},
						"expected":     []string{"MPRJ", "MCST", "MFDB"},
					},
					{
						"scenario_key": "source_to_pay_smoke",
						"steps":        []string{"MPOR.submit", "MPOR.approve", "MPDN.post"},
						"expected":     []string{"MIGN", "MPCH"},
					},
					{
						"scenario_key": "order_to_cash_smoke",
						"steps":        []string{"MRDR.confirm", "MRDR.approve", "MDLN.post", "MINV.post", "MRCT.allocate"},
						"expected":     []string{"MIGE", "MINV", "MRCT", "MJDT"},
					},
					{
						"scenario_key": "inventory_to_finance_smoke",
						"steps":        []string{"MIGN.post", "MIGE.post", "MJDT.post"},
						"expected":     []string{"inventory_movement", "journal_entry"},
					},
					{
						"scenario_key": "finance_trial_balance_smoke",
						"steps":        []string{"MACT", "MPRC", "MJDT.post", "MGLR.run"},
						"expected":     []string{"balanced_debits_credits", "trial_balance"},
					},
				},
			}
			metadata["industry_manifest"] = buildIndustryAssetManifest(input, metadata)
			return metadata
		}(),
	}
}

func BuildRetailDistributionSolutionManifest(input ERPSolutionFlowRequest) IndustrySolutionManifest {
	if input.IndustryKey == "" {
		input.IndustryKey = "retail_chain_distribution"
	}
	if input.PackageKey == "" {
		input.PackageKey = "retail_distribution_v1"
	}
	if input.Name == "" {
		input.Name = "Retail Distribution v1"
	}
	input.EnabledModules = []string{"master_data", "procurement", "inventory", "sales", "finance", "retail"}
	manifest := BuildERPSolutionManifest(input)
	manifest.ModuleKey = "retail_distribution"
	manifest.Metadata["solution_key"] = "retail_distribution_v1"
	manifest.Metadata["code_table_only"] = true
	manifest.Metadata["semantic_supply_chain_policy"] = "removed_from_primary_model"
	manifest.Metadata["ui_workspaces"] = []string{"master_data", "procurement", "sales", "inventory", "finance", "retail", "developer_erp_code"}
	manifest.Metadata["assistant_targets"] = []string{"store", "pos_terminal", "member", "promotion", "item_publication", "pos_sale", "distribution_request", "distribution_shipment", "distribution_receipt", "distribution_difference", "stock_policy", "store_count", "special_purchase_request", "purchase_order", "delivery", "ar_invoice", "incoming_payment", "trial_balance"}
	manifest.Metadata["process_loops"] = append(mapSliceFromAny(manifest.Metadata["process_loops"]),
		map[string]any{"key": "retail_replenishment_to_distribution", "steps": []string{"MSTP.replenish", "MDRQ.submit", "MDRQ.approve", "MDRQ.auto-allocate", "MDSP.ship", "MDRC.receive", "MDIF.resolve"}},
		map[string]any{"key": "retail_pos_to_cash", "steps": []string{"MRPS.close", "MIGE", "MINV", "MRCT"}},
		map[string]any{"key": "retail_count_to_adjustment", "steps": []string{"MCNT.submit", "MCNT.approve", "MCNT.post-adjustment", "MIGN", "MIGE"}},
		map[string]any{"key": "retail_special_procurement", "steps": []string{"MSPR.submit", "MSPR.approve", "MSPR.convert-to-purchase-order", "MPOR.submit", "MPOR.approve", "MPDN.post"}},
	)
	manifest.Metadata["verification_scenarios"] = append(mapSliceFromAny(manifest.Metadata["verification_scenarios"]),
		map[string]any{"scenario_key": "retail_replenishment_to_distribution_smoke", "steps": []string{"MSTP.replenish", "MDRQ.submit", "MDRQ.approve", "MDRQ.auto-allocate", "MDSP.ship", "MDRC.receive"}, "expected": []string{"MDRQ", "MDSP", "MIGE", "MDRC", "MIGN"}},
		map[string]any{"scenario_key": "retail_pos_to_cash_smoke", "steps": []string{"MRPS.close"}, "expected": []string{"MIGE", "MINV", "MRCT"}},
		map[string]any{"scenario_key": "retail_count_to_adjustment_smoke", "steps": []string{"MCNT.submit", "MCNT.approve", "MCNT.post-adjustment"}, "expected": []string{"MIGN", "MIGE"}},
		map[string]any{"scenario_key": "retail_special_procurement_smoke", "steps": []string{"MSPR.submit", "MSPR.approve", "MSPR.convert-to-purchase-order"}, "expected": []string{"MPOR"}},
	)
	manifest.Metadata["context_rules"] = append(mapSliceFromAny(manifest.Metadata["context_rules"]),
		map[string]any{
			"key":                  "retail_distribution_state_context",
			"scope":                "retail",
			"source_tables":        []string{"MBRN", "MTER", "MMBR", "MPRM", "MPUB", "MRPS", "MDRQ", "MDSP", "MDRC", "MDIF", "MSTP", "MCNT", "MSPR", "MITW", "MIGN", "MIGE", "MINV", "MRCT"},
			"required_permissions": []string{"erp:read", "assistant:erp"},
			"workflow_stages":      []string{"draft", "submitted", "approved", "allocated", "shipped", "received", "posted", "closed"},
			"attention_budget":     "retail_distribution_loop",
		},
	)
	manifest.Metadata["assistant_skills"] = append(mapSliceFromAny(manifest.Metadata["assistant_skills"]),
		map[string]any{
			"skill_key":     "retail_distribution_operator",
			"targets":       []string{"stock_policy", "distribution_request", "distribution_shipment", "distribution_receipt", "pos_sale", "store_count", "special_purchase_request"},
			"context_rules": []string{"retail_distribution_state_context"},
			"allowed_tools": []string{"erp.mstp.replenish", "erp.mdrq.submit", "erp.mdrq.approve", "erp.mdrq.auto-allocate", "erp.mdsp.ship", "erp.mdrc.receive", "erp.mrps.close", "erp.mcnt.post-adjustment", "erp.mspr.convert-to-purchase-order"},
		},
	)
	assetManifest := buildIndustryAssetManifest(input, manifest.Metadata)
	assetManifest.Dependencies = append(assetManifest.Dependencies, "erp.retail_code_tables")
	setIndustryAssetManifest(&manifest, assetManifest)
	return manifest
}

type erpRuntimeWorkspaceDocument struct {
	Module       string
	DocumentID   string
	LabelKey     string
	SubmoduleKey string
	TableCode    string
	Actions      []string
	SortOrder    int
}

func buildERPStandardRuntimeOperations(catalog erp.Catalog, actions []erp.ActionDefinition, enabledModules []string) []map[string]any {
	enabled := map[string]bool{}
	for _, module := range enabledModules {
		enabled[module] = true
	}
	if len(enabled) == 0 {
		for _, module := range []string{"project", "procurement", "inventory", "sales", "finance"} {
			enabled[module] = true
		}
	}
	actionDefs := map[string]erp.ActionDefinition{}
	for _, action := range actions {
		actionDefs[action.TableCode+":"+action.Action] = action
	}
	documents := []erpRuntimeWorkspaceDocument{
		{Module: "project", DocumentID: "requirement", LabelKey: "erp.document.requirement", SubmoduleKey: "erp.submodule.requirements", TableCode: "MREQ", Actions: []string{"analyze", "approve", "convert-to-project"}, SortOrder: 10},
		{Module: "project", DocumentID: "project", LabelKey: "erp.document.project", SubmoduleKey: "erp.submodule.projects", TableCode: "MPRJ", Actions: []string{"refresh-cost", "close-feedback"}, SortOrder: 20},
		{Module: "project", DocumentID: "deliverable", LabelKey: "erp.document.delivery", SubmoduleKey: "erp.submodule.deliveries", TableCode: "MDLN", Actions: []string{"post"}, SortOrder: 30},
		{Module: "project", DocumentID: "cost", LabelKey: "erp.document.cost", SubmoduleKey: "erp.submodule.costs", TableCode: "MCST", SortOrder: 40},
		{Module: "project", DocumentID: "feedback", LabelKey: "erp.document.feedback", SubmoduleKey: "erp.submodule.feedback", TableCode: "MFDB", SortOrder: 50},
		{Module: "procurement", DocumentID: "purchase_order", LabelKey: "erp.document.purchaseOrder", SubmoduleKey: "erp.submodule.purchaseOrders", TableCode: "MPOR", Actions: []string{"submit", "approve"}, SortOrder: 10},
		{Module: "procurement", DocumentID: "goods_receipt_po", LabelKey: "erp.document.goodsReceiptPO", SubmoduleKey: "erp.submodule.goodsReceiptPO", TableCode: "MPDN", Actions: []string{"post"}, SortOrder: 20},
		{Module: "procurement", DocumentID: "ap_invoice", LabelKey: "erp.document.apInvoice", SubmoduleKey: "erp.submodule.apInvoices", TableCode: "MPCH", SortOrder: 30},
		{Module: "sales", DocumentID: "sales_order", LabelKey: "erp.document.salesOrder", SubmoduleKey: "erp.submodule.salesOrders", TableCode: "MRDR", Actions: []string{"confirm", "approve"}, SortOrder: 10},
		{Module: "sales", DocumentID: "delivery", LabelKey: "erp.document.delivery", SubmoduleKey: "erp.submodule.deliveries", TableCode: "MDLN", Actions: []string{"post"}, SortOrder: 20},
		{Module: "sales", DocumentID: "ar_invoice", LabelKey: "erp.document.arInvoice", SubmoduleKey: "erp.submodule.arInvoices", TableCode: "MINV", Actions: []string{"post"}, SortOrder: 30},
		{Module: "sales", DocumentID: "incoming_payment", LabelKey: "erp.document.incomingPayment", SubmoduleKey: "erp.submodule.incomingPayments", TableCode: "MRCT", Actions: []string{"allocate"}, SortOrder: 40},
		{Module: "inventory", DocumentID: "business_partner", LabelKey: "erp.document.businessPartner", SubmoduleKey: "erp.submodule.partners", TableCode: "MCRD", SortOrder: 10},
		{Module: "inventory", DocumentID: "item", LabelKey: "erp.document.item", SubmoduleKey: "erp.submodule.items", TableCode: "MITM", SortOrder: 20},
		{Module: "inventory", DocumentID: "warehouse", LabelKey: "erp.document.warehouse", SubmoduleKey: "erp.submodule.warehouses", TableCode: "MWHS", SortOrder: 30},
		{Module: "inventory", DocumentID: "warehouse_balance", LabelKey: "erp.document.warehouseBalance", SubmoduleKey: "erp.submodule.warehouseBalances", TableCode: "MITW", SortOrder: 40},
		{Module: "inventory", DocumentID: "goods_receipt", LabelKey: "erp.document.goodsReceipt", SubmoduleKey: "erp.submodule.goodsReceipts", TableCode: "MIGN", Actions: []string{"post"}, SortOrder: 50},
		{Module: "inventory", DocumentID: "goods_issue", LabelKey: "erp.document.goodsIssue", SubmoduleKey: "erp.submodule.goodsIssues", TableCode: "MIGE", Actions: []string{"post"}, SortOrder: 60},
		{Module: "finance", DocumentID: "gl_account", LabelKey: "erp.document.glAccount", SubmoduleKey: "erp.submodule.chartOfAccounts", TableCode: "MACT", SortOrder: 10},
		{Module: "finance", DocumentID: "cost_center", LabelKey: "erp.document.costCenter", SubmoduleKey: "erp.submodule.costCenters", TableCode: "MPRC", SortOrder: 20},
		{Module: "finance", DocumentID: "journal_entry", LabelKey: "erp.document.journalEntry", SubmoduleKey: "erp.submodule.journalEntries", TableCode: "MJDT", Actions: []string{"post"}, SortOrder: 30},
		{Module: "finance", DocumentID: "trial_balance", LabelKey: "erp.document.trialBalance", SubmoduleKey: "erp.submodule.trialBalance", TableCode: "MGLR", Actions: []string{"run"}, SortOrder: 40},
		{Module: "finance", DocumentID: "ar_invoice", LabelKey: "erp.document.arInvoice", SubmoduleKey: "erp.submodule.arInvoices", TableCode: "MINV", Actions: []string{"post"}, SortOrder: 50},
		{Module: "finance", DocumentID: "ap_invoice", LabelKey: "erp.document.apInvoice", SubmoduleKey: "erp.submodule.apInvoices", TableCode: "MPCH", SortOrder: 60},
		{Module: "finance", DocumentID: "incoming_payment", LabelKey: "erp.document.incomingPayment", SubmoduleKey: "erp.submodule.incomingPayments", TableCode: "MRCT", Actions: []string{"allocate"}, SortOrder: 70},
		{Module: "retail", DocumentID: "store", LabelKey: "erp.document.store", SubmoduleKey: "erp.submodule.stores", TableCode: "MBRN", SortOrder: 10},
		{Module: "retail", DocumentID: "pos_terminal", LabelKey: "erp.document.posTerminal", SubmoduleKey: "erp.submodule.pos", TableCode: "MTER", SortOrder: 20},
		{Module: "retail", DocumentID: "member", LabelKey: "erp.document.member", SubmoduleKey: "erp.submodule.membersPromotions", TableCode: "MMBR", SortOrder: 30},
		{Module: "retail", DocumentID: "promotion", LabelKey: "erp.document.promotion", SubmoduleKey: "erp.submodule.membersPromotions", TableCode: "MPRM", SortOrder: 40},
		{Module: "retail", DocumentID: "item_publication", LabelKey: "erp.document.itemPublication", SubmoduleKey: "erp.submodule.membersPromotions", TableCode: "MPUB", Actions: []string{"publish"}, SortOrder: 50},
		{Module: "retail", DocumentID: "pos_sale", LabelKey: "erp.document.posSale", SubmoduleKey: "erp.submodule.pos", TableCode: "MRPS", Actions: []string{"close"}, SortOrder: 60},
		{Module: "retail", DocumentID: "distribution_request", LabelKey: "erp.document.distributionRequest", SubmoduleKey: "erp.submodule.distribution", TableCode: "MDRQ", Actions: []string{"submit", "approve", "auto-allocate"}, SortOrder: 70},
		{Module: "retail", DocumentID: "distribution_shipment", LabelKey: "erp.document.distributionShipment", SubmoduleKey: "erp.submodule.distribution", TableCode: "MDSP", Actions: []string{"ship"}, SortOrder: 80},
		{Module: "retail", DocumentID: "distribution_receipt", LabelKey: "erp.document.distributionReceipt", SubmoduleKey: "erp.submodule.distribution", TableCode: "MDRC", Actions: []string{"receive"}, SortOrder: 90},
		{Module: "retail", DocumentID: "distribution_difference", LabelKey: "erp.document.distributionDifference", SubmoduleKey: "erp.submodule.distribution", TableCode: "MDIF", Actions: []string{"resolve"}, SortOrder: 100},
		{Module: "retail", DocumentID: "stock_policy", LabelKey: "erp.document.stockPolicy", SubmoduleKey: "erp.submodule.inventoryControl", TableCode: "MSTP", Actions: []string{"replenish"}, SortOrder: 110},
		{Module: "retail", DocumentID: "store_count", LabelKey: "erp.document.storeCount", SubmoduleKey: "erp.submodule.inventoryControl", TableCode: "MCNT", Actions: []string{"submit", "approve", "post-adjustment"}, SortOrder: 120},
		{Module: "retail", DocumentID: "special_purchase_request", LabelKey: "erp.document.specialPurchaseRequest", SubmoduleKey: "erp.submodule.specialProcurement", TableCode: "MSPR", Actions: []string{"submit", "approve", "convert-to-purchase-order"}, SortOrder: 130},
	}
	operations := []map[string]any{}
	seenPaths := map[string]bool{}
	for _, operation := range []map[string]any{
		{
			"operation_key":      "erp.catalog",
			"domain":             "ERP",
			"title":              "operation.erp.catalog",
			"method":             "GET",
			"path":               "/erp/catalog",
			"operation_kind":     "direct",
			"danger_level":       "low",
			"result_view":        "list",
			"assistant_eligible": false,
			"action_type":        "erp.catalog",
		},
		{
			"operation_key":      "erp.actions",
			"domain":             "ERP",
			"title":              "operation.erp.actions",
			"method":             "GET",
			"path":               "/erp/actions",
			"operation_kind":     "direct",
			"danger_level":       "low",
			"result_view":        "list",
			"assistant_eligible": true,
			"action_type":        "erp.action.catalog",
		},
		{
			"operation_key":      "erp.records.list",
			"domain":             "ERP",
			"title":              "operation.erp.recordList",
			"method":             "GET",
			"path":               "/erp/{tableCode}",
			"operation_kind":     "direct",
			"danger_level":       "low",
			"result_view":        "list",
			"assistant_eligible": false,
			"action_type":        "erp.record.list",
		},
	} {
		operations = append(operations, operation)
		seenPaths[stringValue(operation["path"])] = true
	}
	for _, document := range documents {
		if !enabled[document.Module] {
			continue
		}
		table, ok := catalog.Table(document.TableCode)
		if !ok {
			continue
		}
		childCode := ""
		if len(table.Children) > 0 {
			childCode = table.Children[0].Code
		}
		workspaceKind := "document"
		if table.Code == "MGLR" {
			workspaceKind = "report"
		}
		workspace := map[string]any{
			"module":             document.Module,
			"document_id":        document.DocumentID,
			"document_label_key": document.LabelKey,
			"submodule_key":      document.SubmoduleKey,
			"table_code":         table.Code,
			"primary_key":        table.PrimaryKey,
			"child_code":         childCode,
			"kind":               workspaceKind,
			"sort_order":         document.SortOrder,
		}
		path := fmt.Sprintf("/erp/%s", table.Code)
		if table.Code != "MGLR" && !seenPaths[path] {
			operations = append(operations, map[string]any{
				"operation_key":      fmt.Sprintf("erp.workspace.%s.%s.list", document.Module, document.DocumentID),
				"domain":             "ERP",
				"title":              document.LabelKey,
				"method":             "GET",
				"path":               path,
				"operation_kind":     "direct",
				"danger_level":       "low",
				"result_view":        "list",
				"assistant_eligible": false,
				"action_type":        "erp.document.list",
				"workspace":          workspace,
			})
			seenPaths[path] = true
		}
		for index, actionName := range document.Actions {
			action, ok := actionDefs[table.Code+":"+actionName]
			if !ok {
				continue
			}
			if table.Code == "MGLR" && action.Action == "run" {
				actionPath := "/finance/gl/trial-balance"
				if seenPaths[actionPath] {
					continue
				}
				actionWorkspace := copyMap(workspace)
				actionWorkspace["action"] = action.Action
				actionWorkspace["state_gate"] = table.Code + "." + action.Action
				actionWorkspace["action_params"] = map[string]any{}
				actionWorkspace["sort_order"] = document.SortOrder + index + 1
				operations = append(operations, map[string]any{
					"operation_key":      "erp.finance.trial_balance.run",
					"domain":             "ERP",
					"title":              "operation.erp.MGLR.run",
					"method":             "GET",
					"path":               actionPath,
					"operation_kind":     "direct",
					"danger_level":       "low",
					"result_view":        "list",
					"assistant_eligible": true,
					"action_type":        "finance.gl.trial_balance",
					"workspace":          actionWorkspace,
					"next_tables":        action.NextTables,
				})
				seenPaths[actionPath] = true
				continue
			}
			actionPath := fmt.Sprintf("/erp/%s/{key}/actions/%s", table.Code, action.Action)
			if seenPaths[actionPath] {
				continue
			}
			actionWorkspace := copyMap(workspace)
			actionWorkspace["kind"] = "action"
			actionWorkspace["action"] = action.Action
			actionWorkspace["state_gate"] = table.Code + "." + action.Action
			actionWorkspace["action_params"] = map[string]any{}
			actionWorkspace["sort_order"] = document.SortOrder + index + 1
			operations = append(operations, map[string]any{
				"operation_key":      fmt.Sprintf("erp.%s.%s", strings.ToLower(table.Code), action.Action),
				"domain":             "ERP",
				"title":              fmt.Sprintf("operation.erp.%s.%s", table.Code, action.Action),
				"method":             "POST",
				"path":               actionPath,
				"operation_kind":     "contextual",
				"danger_level":       "medium",
				"result_view":        "summary",
				"assistant_eligible": true,
				"action_type":        "erp.action",
				"workspace":          actionWorkspace,
				"next_tables":        action.NextTables,
			})
			seenPaths[actionPath] = true
		}
	}
	return operations
}

func copyMap(input map[string]any) map[string]any {
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func (s *Service) VerifyIndustrySolutionChange(ctx context.Context, actorID uuid.UUID, requestID uuid.UUID) (*IndustrySolutionVerificationReport, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionIndustrySolutionVerify); err != nil {
		return nil, err
	}
	request, err := s.repo.GetIndustrySolutionChangeRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}
	schemaName := request.TargetSchemaName
	if schemaName == "" {
		schemaName = tenantdb.SchemaNameForOrganization(request.OrganizationID)
	}
	report := &IndustrySolutionVerificationReport{
		ChangeRequestID:  request.ID,
		OrganizationID:   request.OrganizationID,
		TargetSchemaName: schemaName,
		RequestStatus:    request.Status,
		Status:           "passed",
		RiskLevel:        firstNonEmptyString(request.RiskLevel, IndustrySolutionRiskSafe),
	}
	if err := ValidateIndustrySolutionManifest(request.SolutionManifest); err != nil {
		report.addCheck("solution_manifest", "failed", err.Error(), nil)
	} else {
		report.addCheck("solution_manifest", "passed", "industry solution manifest is valid", map[string]any{
			"module_key": request.SolutionManifest.ModuleKey,
			"tables":     len(request.SolutionManifest.Tables),
		})
	}
	statements := request.Statements
	if len(statements) == 0 && report.BlockingIssues == 0 {
		generated, err := BuildIndustrySolutionTableStatements(schemaName, request.SolutionManifest)
		if err != nil {
			report.addCheck("ddl_plan", "failed", err.Error(), nil)
		} else {
			statements = generated
		}
	}
	report.StatementCount = len(statements)
	if report.StatementCount == 0 {
		report.addCheck("ddl_plan", "failed", "industry solution change has no executable statements", nil)
	} else {
		report.addCheck("ddl_plan", "passed", "DDL statements are available", map[string]any{"statement_count": report.StatementCount})
	}
	if report.RiskLevel == IndustrySolutionRiskDestructive {
		report.addCheck("risk_level", "warning", "destructive industry solution changes require explicit review before apply", nil)
	} else {
		report.addCheck("risk_level", "passed", "risk level is safe", nil)
	}
	addIndustryFactoryCoverageChecks(report, request)
	switch request.Status {
	case IndustrySolutionChangeApproved:
		report.addCheck("lifecycle_status", "passed", "change request is approved for apply", nil)
	case IndustrySolutionChangePending:
		report.addCheck("lifecycle_status", "warning", "change request must be approved before apply", nil)
	default:
		report.addCheck("lifecycle_status", "failed", "change request is not in an applicable state", map[string]any{"status": request.Status})
	}
	report.Status = reportStatus(report.Checks)
	report.CanApply = report.BlockingIssues == 0 && request.Status == IndustrySolutionChangeApproved
	return report, nil
}

func addIndustryFactoryCoverageChecks(report *IndustrySolutionVerificationReport, request *IndustrySolutionChangeRequest) {
	if report == nil || request == nil || !isIndustryFactoryPackage(request) {
		return
	}
	addMetadataCoverageCheck(report, request, "permissions_impact", []string{"permissions"}, "solution declares permission impact", "industry solution should declare permission impact")
	manifest, err := AssetManifestFromSolutionManifest(request.SolutionManifest)
	if err != nil {
		report.addCheck("industry_manifest", "failed", err.Error(), nil)
		return
	}
	for _, check := range ManifestVerificationChecks(manifest, report.RiskLevel) {
		if check.Key == "verification_scenarios" && !request.SolutionManifestHas("verification_scenarios") {
			check.Status = "warning"
			check.Message = "industry solution should declare verification scenarios"
			check.Metadata = map[string]any{"missing": []string{"verification_scenarios"}}
		}
		report.addCheck(check.Key, check.Status, check.Message, check.Metadata)
	}
}

func (s *Service) GetIndustrySolutionChangeAssetDiff(ctx context.Context, actorID uuid.UUID, requestID uuid.UUID) ([]IndustrySolutionAssetDiff, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionIndustrySolutionVerify); err != nil {
		return nil, err
	}
	request, err := s.repo.GetIndustrySolutionChangeRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if diff := AssetDiffFromSolutionManifest(request.SolutionManifest); len(diff) > 0 {
		return diff, nil
	}
	manifest, err := AssetManifestFromSolutionManifest(request.SolutionManifest)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	return BuildIndustrySolutionAssetDiff(IndustrySolutionAssetManifest{ManifestVersion: IndustryManifestVersion}, manifest), nil
}

func isIndustryFactoryPackage(request *IndustrySolutionChangeRequest) bool {
	if request == nil {
		return false
	}
	if request.RequestType == "erp_solution_flow" {
		return true
	}
	metadata := request.SolutionManifest.Metadata
	if metadata == nil {
		return false
	}
	return metadata["industry_key"] != nil || metadata["package_key"] != nil
}

func addMetadataCoverageCheck(report *IndustrySolutionVerificationReport, request *IndustrySolutionChangeRequest, key string, required []string, passedMessage string, warningMessage string) {
	missing := make([]string, 0)
	counts := make(map[string]any, len(required))
	for _, metadataKey := range required {
		if !request.SolutionManifestHas(metadataKey) {
			missing = append(missing, metadataKey)
			continue
		}
		counts[metadataKey] = metadataValueCount(request.SolutionManifest.Metadata[metadataKey])
	}
	if len(missing) > 0 {
		report.addCheck(key, "warning", warningMessage, map[string]any{"missing": missing})
		return
	}
	report.addCheck(key, "passed", passedMessage, counts)
}

func metadataValueCount(value any) int {
	switch typed := value.(type) {
	case []map[string]any:
		return len(typed)
	case []string:
		return len(typed)
	case []any:
		return len(typed)
	default:
		if value == nil {
			return 0
		}
		return 1
	}
}

func (r *IndustrySolutionVerificationReport) addCheck(key string, status string, message string, metadata map[string]any) {
	r.Checks = append(r.Checks, IndustrySolutionVerificationCheck{Key: key, Status: status, Message: message, Metadata: metadata})
	if status == "failed" {
		r.BlockingIssues++
	}
}

func reportStatus(checks []IndustrySolutionVerificationCheck) string {
	hasWarning := false
	for _, check := range checks {
		switch check.Status {
		case "failed":
			return "failed"
		case "warning":
			hasWarning = true
		}
	}
	if hasWarning {
		return "warning"
	}
	return "passed"
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func erpSolutionAssetTable(name string) IndustrySolutionTableDefinition {
	return IndustrySolutionTableDefinition{
		Name: name,
		Fields: []IndustrySolutionFieldDefinition{
			{Name: "id", DataType: "uuid", PrimaryKey: true, Default: "gen_random_uuid()"},
			{Name: "asset_key", DataType: "varchar(120)", Nullable: false},
			{Name: "payload", DataType: "jsonb", Nullable: false, Default: "'{}'::jsonb"},
			{Name: "created_at", DataType: "timestamptz", Nullable: false, Default: "now()"},
		},
		Indexes: []IndustrySolutionIndexDefinition{{Name: name + "_asset_key_idx", Fields: []string{"asset_key"}, Unique: true}},
	}
}

func (s *Service) ApproveIndustrySolutionChange(ctx context.Context, actorID uuid.UUID, requestID uuid.UUID, reason string) (*IndustrySolutionChangeRequest, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionIndustrySolutionApprove); err != nil {
		return nil, err
	}
	return s.repo.UpdateIndustrySolutionChangeRequestStatus(ctx, requestID, IndustrySolutionChangeApproved, actorID, reason)
}

func (s *Service) ApplyIndustrySolutionChange(ctx context.Context, actorID uuid.UUID, requestID uuid.UUID) (*IndustrySolutionApplyJob, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionIndustrySolutionApply); err != nil {
		return nil, err
	}
	request, err := s.repo.GetIndustrySolutionChangeRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if request.Status != IndustrySolutionChangeApproved {
		return nil, ErrInvalidTransition
	}
	report, err := s.VerifyIndustrySolutionChange(ctx, actorID, requestID)
	if err != nil {
		return nil, err
	}
	if !report.CanApply {
		return nil, fmt.Errorf("%w: industry solution change verification has %d blocking issues", ErrValidation, report.BlockingIssues)
	}
	schemaName := request.TargetSchemaName
	if schemaName == "" {
		schemaName = tenantdb.SchemaNameForOrganization(request.OrganizationID)
	}
	statements := request.Statements
	if len(statements) == 0 {
		statements, err = BuildIndustrySolutionTableStatements(schemaName, request.SolutionManifest)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrValidation, err)
		}
	}
	assetResults := []IndustrySolutionApplyAssetResult{}
	if manifest, err := AssetManifestFromSolutionManifest(request.SolutionManifest); err == nil {
		assetResults = BuildIndustrySolutionApplyAssetResults(manifest)
	}
	return s.repo.ApplyIndustrySolutionChange(ctx, request, statements, assetResults)
}

func (s *Service) requirePlatformPermission(ctx context.Context, actorID uuid.UUID, permission string) error {
	_, permissions, err := s.permissionsForActor(ctx, actorID)
	if err != nil || !permissions[permission] {
		return ErrForbidden
	}
	return nil
}

func (s *Service) permissionsForActor(ctx context.Context, actorID uuid.UUID) (string, map[string]bool, error) {
	if actorID == uuid.Nil {
		return "", nil, ErrForbidden
	}
	role, err := s.repo.GetPlatformRole(ctx, actorID)
	if err != nil {
		return "", nil, ErrForbidden
	}
	normalized := platformauth.NormalizeRole(role)
	permissions := platformauth.PermissionsForRole(normalized)
	if dbPermissions, err := s.repo.ListPlatformRolePermissions(ctx, normalized); err == nil && len(dbPermissions) > 0 {
		permissions = permissionsMap(dbPermissions)
	}
	if len(permissions) == 0 {
		return normalized, nil, ErrForbidden
	}
	return normalized, permissions, nil
}

func permissionsMap(values []string) map[string]bool {
	result := map[string]bool{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			result[trimmed] = true
		}
	}
	return result
}

func normalizeStringList(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		result = append(result, trimmed)
	}
	return result
}

func normalizePlatformRoles(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		normalized := platformauth.NormalizeRole(value)
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		result = append(result, normalized)
	}
	return result
}

func generateTemporaryPassword() (string, error) {
	buffer := make([]byte, 18)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate temporary password: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func validateMetadataOnlyExtension(metadata map[string]any) error {
	for key := range metadata {
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "script", "scripts", "sql", "command", "commands", "code", "handler", "plugin", "plugin_url", "webhook":
			return fmt.Errorf("%w: platform feature metadata cannot contain executable key %q", ErrValidation, key)
		}
	}
	return nil
}
