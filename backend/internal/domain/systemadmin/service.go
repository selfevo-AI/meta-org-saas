package systemadmin

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/erp"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/platformauth"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/tenantdb"
)

var (
	ErrForbidden         = errors.New("forbidden")
	ErrValidation        = errors.New("validation error")
	ErrInvalidTransition = errors.New("invalid schema change status transition")
)

type Service struct {
	repo repository
}

type repository interface {
	GetPlatformRole(context.Context, uuid.UUID) (string, error)
	ListPlatformMasters(context.Context, string, int) ([]PlatformMaster, error)
	ListPlatformDetails(context.Context, string) ([]PlatformDetail, error)
	ListSchemaTargets(context.Context, int) ([]OrganizationSchemaTarget, error)
	GetSchemaTarget(context.Context, uuid.UUID) (*OrganizationSchemaTarget, error)
	CreateSchemaChangeRequest(context.Context, CreateSchemaChangeRequestRecord) (*SchemaChangeRequest, error)
	GetSchemaChangeRequest(context.Context, uuid.UUID) (*SchemaChangeRequest, error)
	UpdateSchemaChangeRequestStatus(context.Context, uuid.UUID, string, uuid.UUID, string) (*SchemaChangeRequest, error)
	ApplySchemaChange(context.Context, *SchemaChangeRequest, []string) (*SchemaApplyJob, error)
}

func NewService(repo repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetPermissionProfile(ctx context.Context, actorID uuid.UUID) (*PlatformPermissionProfile, error) {
	if actorID == uuid.Nil {
		return nil, ErrForbidden
	}
	role, err := s.repo.GetPlatformRole(ctx, actorID)
	if err != nil {
		return nil, ErrForbidden
	}
	normalized := platformauth.NormalizeRole(role)
	permissions := platformauth.PermissionsForRole(normalized)
	if len(permissions) == 0 {
		return nil, ErrForbidden
	}
	return &PlatformPermissionProfile{
		Role:        normalized,
		Permissions: permissions,
		MenuItems:   menuItemsForPermissions(permissions),
	}, nil
}

func (s *Service) ListPlatformMasters(ctx context.Context, actorID uuid.UUID, moduleKey string, limit int) ([]PlatformMaster, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionPlatformRead); err != nil {
		return nil, err
	}
	return s.repo.ListPlatformMasters(ctx, moduleKey, limit)
}

func menuItemsForPermissions(permissions map[string]bool) []string {
	items := []string{}
	if permissions[platformauth.PermissionPlatformRead] {
		items = append(items, "saas", "catalog", "targets", "assistant")
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
	if permissions[platformauth.PermissionSchemaManage] {
		items = append(items, "schema")
	}
	return items
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

func (s *Service) ListSchemaTargets(ctx context.Context, actorID uuid.UUID, limit int) ([]OrganizationSchemaTarget, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionPlatformRead); err != nil {
		return nil, err
	}
	return s.repo.ListSchemaTargets(ctx, limit)
}

func (s *Service) ExportOrganizationSchema(ctx context.Context, actorID uuid.UUID, orgID uuid.UUID) (*SchemaPackage, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionSchemaManage); err != nil {
		return nil, err
	}
	if orgID == uuid.Nil {
		return nil, fmt.Errorf("%w: organization_id is required", ErrValidation)
	}
	pkg := DefaultOrganizationSchemaPackage()
	return &pkg, nil
}

func (s *Service) CreateSchemaChangeRequest(ctx context.Context, actorID uuid.UUID, input CreateSchemaChangeRequestInput) (*SchemaChangeRequest, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionSchemaManage); err != nil {
		return nil, err
	}
	if input.OrganizationID == uuid.Nil {
		return nil, fmt.Errorf("%w: organization_id is required", ErrValidation)
	}
	if input.RequestType == "" {
		input.RequestType = "import_schema_package"
	}
	if err := ValidateSchemaPackage(input.SchemaPackage); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	schemaName := tenantdb.SchemaNameForOrganization(input.OrganizationID)
	riskLevel := SchemaRiskSafe
	diff := []SchemaDiff{{Action: "create_or_ensure_tables", Risk: SchemaRiskSafe}}
	var statements []string
	if input.CurrentSchemaPackage != nil {
		plan, err := BuildSchemaMigrationPlan(schemaName, *input.CurrentSchemaPackage, input.SchemaPackage)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrValidation, err)
		}
		statements = plan.Statements
		riskLevel = plan.RiskLevel
		diff = plan.Diff
	} else {
		var err error
		statements, err = BuildCreateTableStatements(schemaName, input.SchemaPackage)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrValidation, err)
		}
	}
	return s.repo.CreateSchemaChangeRequest(ctx, CreateSchemaChangeRequestRecord{
		OrganizationID: input.OrganizationID,
		SchemaName:     schemaName,
		RequestType:    input.RequestType,
		Reason:         input.Reason,
		SchemaPackage:  input.SchemaPackage,
		Statements:     statements,
		RiskLevel:      riskLevel,
		Diff:           diff,
		RequestedBy:    actorID,
	})
}

func (s *Service) BuildERPSolutionFlow(ctx context.Context, actorID uuid.UUID, input ERPSolutionFlowRequest) (*SchemaChangeRequest, error) {
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
	pkg := BuildERPSolutionSchemaPackage(input)
	return s.CreateSchemaChangeRequest(ctx, actorID, CreateSchemaChangeRequestInput{
		OrganizationID:       input.OrganizationID,
		RequestType:          "erp_solution_flow",
		Reason:               "Create ERP standard industry solution flow",
		SchemaPackage:        pkg,
		CurrentSchemaPackage: input.CurrentTemplate,
	})
}

func BuildERPSolutionSchemaPackage(input ERPSolutionFlowRequest) SchemaPackage {
	catalog := erp.DefaultCatalog()
	actions := erp.DefaultActionRegistry().List()
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
	apiOperations := make([]string, 0, len(actions)+3)
	apiOperations = append(apiOperations, "/erp/catalog", "/erp/actions", "/erp/{tableCode}")
	for _, action := range actions {
		businessFunctions = append(businessFunctions, map[string]any{
			"table_code":  action.TableCode,
			"action":      action.Action,
			"label":       action.Label,
			"next_tables": action.NextTables,
		})
		apiOperations = append(apiOperations, fmt.Sprintf("/erp/%s/{key}/actions/%s", action.TableCode, action.Action))
	}
	enabled := make([]string, 0, len(input.EnabledModules))
	for _, module := range input.EnabledModules {
		trimmed := strings.TrimSpace(module)
		if trimmed != "" {
			enabled = append(enabled, trimmed)
		}
	}
	return SchemaPackage{
		FormatVersion: SchemaPackageFormatVersion,
		ModuleKey:     "erp_standard",
		Tables: []SchemaTableDefinition{
			erpSolutionAssetTable("erp_solution_database_assets"),
			erpSolutionAssetTable("erp_solution_business_functions"),
			erpSolutionAssetTable("erp_solution_process_loops"),
			erpSolutionAssetTable("erp_solution_ui_metadata"),
			erpSolutionAssetTable("erp_solution_assistant_targets"),
		},
		Metadata: map[string]any{
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
			},
			"permissions":       []string{"erp:read", "erp:write", "erp:action", "erp:admin", "assistant:erp"},
			"api_operations":    apiOperations,
			"ui_workspaces":     []string{"project", "procurement", "sales", "inventory", "finance", "developer_erp_code"},
			"assistant_targets": []string{"requirement", "project", "purchase_order", "sales_order", "ar_invoice", "ap_invoice", "journal_entry"},
		},
	}
}

func erpSolutionAssetTable(name string) SchemaTableDefinition {
	return SchemaTableDefinition{
		Name: name,
		Fields: []SchemaFieldDefinition{
			{Name: "id", DataType: "uuid", PrimaryKey: true, Default: "gen_random_uuid()"},
			{Name: "asset_key", DataType: "varchar(120)", Nullable: false},
			{Name: "payload", DataType: "jsonb", Nullable: false, Default: "'{}'::jsonb"},
			{Name: "created_at", DataType: "timestamptz", Nullable: false, Default: "now()"},
		},
		Indexes: []SchemaIndexDefinition{{Name: name + "_asset_key_idx", Fields: []string{"asset_key"}, Unique: true}},
	}
}

func (s *Service) ApproveSchemaChange(ctx context.Context, actorID uuid.UUID, requestID uuid.UUID, reason string) (*SchemaChangeRequest, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionSchemaApprove); err != nil {
		return nil, err
	}
	return s.repo.UpdateSchemaChangeRequestStatus(ctx, requestID, SchemaChangeApproved, actorID, reason)
}

func (s *Service) ApplySchemaChange(ctx context.Context, actorID uuid.UUID, requestID uuid.UUID) (*SchemaApplyJob, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionSchemaApply); err != nil {
		return nil, err
	}
	request, err := s.repo.GetSchemaChangeRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if request.Status != SchemaChangeApproved {
		return nil, ErrInvalidTransition
	}
	schemaName := request.SchemaName
	if schemaName == "" {
		schemaName = tenantdb.SchemaNameForOrganization(request.OrganizationID)
	}
	statements := request.Statements
	if len(statements) == 0 {
		statements, err = BuildCreateTableStatements(schemaName, request.SchemaPackage)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrValidation, err)
		}
	}
	return s.repo.ApplySchemaChange(ctx, request, statements)
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
