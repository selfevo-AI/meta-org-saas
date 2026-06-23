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
			"tool_key":    "schema.change.preview",
			"entrypoint":  "/platform/admin/schema-change-requests/{id}/verify",
			"policy":      "schema.manage",
			"permissions": []string{"schema.manage"},
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
		toolDefinitions = append(toolDefinitions, map[string]any{
			"tool_key":    fmt.Sprintf("erp.%s.%s", strings.ToLower(action.TableCode), action.Action),
			"table_code":  action.TableCode,
			"action":      action.Action,
			"entrypoint":  fmt.Sprintf("/erp/%s/{key}/actions/%s", action.TableCode, action.Action),
			"policy":      "erp_action_state_gate",
			"permissions": []string{"erp:action"},
			"idempotency": fmt.Sprintf("%s:key:%s", action.TableCode, action.Action),
			"next_tables": action.NextTables,
			"risk_level":  "medium",
		})
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
			erpSolutionAssetTable("erp_solution_context_rules"),
			erpSolutionAssetTable("erp_solution_tool_definitions"),
			erpSolutionAssetTable("erp_solution_assistant_skills"),
			erpSolutionAssetTable("erp_solution_quality_gates"),
			erpSolutionAssetTable("erp_solution_verification_scenarios"),
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
			"context_rules": []map[string]any{
				{
					"key":                  "erp_document_state_context",
					"scope":                "erp",
					"source_tables":        []string{"MREQ", "MPRJ", "MPOR", "MPDN", "MRDR", "MDLN", "MINV", "MRCT", "MIGN", "MIGE", "MJDT"},
					"required_permissions": []string{"erp:read"},
					"workflow_stages":      []string{"draft", "submitted", "approved", "posted", "closed"},
					"attention_budget":     "document_timeline",
				},
				{
					"key":                  "erp_finance_validation_context",
					"scope":                "finance",
					"source_tables":        []string{"MCST", "MINV", "MPCH", "MRCT", "MJDT"},
					"required_permissions": []string{"erp:read", "assistant:erp"},
					"workflow_stages":      []string{"cost_refresh", "invoice_posting", "payment_allocation"},
					"attention_budget":     "cost_and_settlement",
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
					"skill_key":     "schema_change_reviewer",
					"targets":       []string{"schema_change", "industry_package"},
					"context_rules": []string{"erp_governance_approval_context"},
					"allowed_tools": []string{"schema.change.preview"},
				},
			},
			"quality_gates": []map[string]any{
				{
					"gate_key":        "schema_verify_before_apply",
					"stage":           "schema_change",
					"required_checks": []string{"schema_package", "ddl_plan", "permissions_impact", "runtime_operations", "assistant_context", "verification_scenarios"},
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
			},
		},
	}
}

func (s *Service) VerifySchemaChange(ctx context.Context, actorID uuid.UUID, requestID uuid.UUID) (*SchemaVerificationReport, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionSchemaManage); err != nil {
		return nil, err
	}
	request, err := s.repo.GetSchemaChangeRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}
	schemaName := request.SchemaName
	if schemaName == "" {
		schemaName = tenantdb.SchemaNameForOrganization(request.OrganizationID)
	}
	report := &SchemaVerificationReport{
		ChangeRequestID: request.ID,
		OrganizationID:  request.OrganizationID,
		SchemaName:      schemaName,
		RequestStatus:   request.Status,
		Status:          "passed",
		RiskLevel:       firstNonEmptyString(request.RiskLevel, SchemaRiskSafe),
	}
	if err := ValidateSchemaPackage(request.SchemaPackage); err != nil {
		report.addCheck("schema_package", "failed", err.Error(), nil)
	} else {
		report.addCheck("schema_package", "passed", "schema package is valid", map[string]any{
			"module_key": request.SchemaPackage.ModuleKey,
			"tables":     len(request.SchemaPackage.Tables),
		})
	}
	statements := request.Statements
	if len(statements) == 0 && report.BlockingIssues == 0 {
		generated, err := BuildCreateTableStatements(schemaName, request.SchemaPackage)
		if err != nil {
			report.addCheck("ddl_plan", "failed", err.Error(), nil)
		} else {
			statements = generated
		}
	}
	report.StatementCount = len(statements)
	if report.StatementCount == 0 {
		report.addCheck("ddl_plan", "failed", "schema change has no executable statements", nil)
	} else {
		report.addCheck("ddl_plan", "passed", "DDL statements are available", map[string]any{"statement_count": report.StatementCount})
	}
	if report.RiskLevel == SchemaRiskDestructive {
		report.addCheck("risk_level", "warning", "destructive schema changes require explicit review before apply", nil)
	} else {
		report.addCheck("risk_level", "passed", "risk level is safe", nil)
	}
	addIndustryFactoryCoverageChecks(report, request)
	switch request.Status {
	case SchemaChangeApproved:
		report.addCheck("lifecycle_status", "passed", "change request is approved for apply", nil)
	case SchemaChangePending:
		report.addCheck("lifecycle_status", "warning", "change request must be approved before apply", nil)
	default:
		report.addCheck("lifecycle_status", "failed", "change request is not in an applicable state", map[string]any{"status": request.Status})
	}
	report.Status = reportStatus(report.Checks)
	report.CanApply = report.BlockingIssues == 0 && request.Status == SchemaChangeApproved
	return report, nil
}

func addIndustryFactoryCoverageChecks(report *SchemaVerificationReport, request *SchemaChangeRequest) {
	if report == nil || request == nil || !isIndustryFactoryPackage(request) {
		return
	}
	addMetadataCoverageCheck(report, request, "permissions_impact", []string{"permissions"}, "package declares permission impact", "industry package should declare permission impact")
	addMetadataCoverageCheck(report, request, "runtime_operations", []string{"api_operations"}, "runtime operation coverage is declared", "industry package should declare runtime operation coverage")
	addMetadataCoverageCheck(report, request, "assistant_context", []string{"assistant_targets", "context_rules"}, "assistant context coverage is declared", "industry package should declare assistant targets and context rules")
	addMetadataCoverageCheck(report, request, "tool_policy", []string{"tool_definitions"}, "tool policy coverage is declared", "industry package should declare Tool Runtime definitions")
	addMetadataCoverageCheck(report, request, "assistant_skills", []string{"assistant_skills"}, "assistant skill coverage is declared", "industry package should declare assistant skills")
	addMetadataCoverageCheck(report, request, "quality_gates", []string{"quality_gates"}, "quality gates are declared", "industry package should declare quality gates")
	addMetadataCoverageCheck(report, request, "verification_scenarios", []string{"verification_scenarios"}, "verification scenarios are declared", "industry package should declare verification scenarios")
	if report.RiskLevel == SchemaRiskDestructive {
		report.addCheck("rollback_risk", "warning", "destructive schema changes need a rollback plan before apply", nil)
		return
	}
	report.addCheck("rollback_risk", "passed", "rollback risk is low for additive factory package", map[string]any{"risk_level": report.RiskLevel})
}

func isIndustryFactoryPackage(request *SchemaChangeRequest) bool {
	if request == nil {
		return false
	}
	if request.RequestType == "erp_solution_flow" {
		return true
	}
	metadata := request.SchemaPackage.Metadata
	if metadata == nil {
		return false
	}
	return metadata["industry_key"] != nil || metadata["package_key"] != nil
}

func addMetadataCoverageCheck(report *SchemaVerificationReport, request *SchemaChangeRequest, key string, required []string, passedMessage string, warningMessage string) {
	missing := make([]string, 0)
	counts := make(map[string]any, len(required))
	for _, metadataKey := range required {
		if !request.SchemaPackageHas(metadataKey) {
			missing = append(missing, metadataKey)
			continue
		}
		counts[metadataKey] = metadataValueCount(request.SchemaPackage.Metadata[metadataKey])
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

func (r *SchemaVerificationReport) addCheck(key string, status string, message string, metadata map[string]any) {
	r.Checks = append(r.Checks, SchemaVerificationCheck{Key: key, Status: status, Message: message, Metadata: metadata})
	if status == "failed" {
		r.BlockingIssues++
	}
}

func reportStatus(checks []SchemaVerificationCheck) string {
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
