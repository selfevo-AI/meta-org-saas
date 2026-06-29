package toolruntime

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/erp"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/evolution"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/finance"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/organization"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/project"
	domainruntime "github.com/selfevo-AI/meta-org-saas/backend/internal/domain/runtime"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/systemadmin"
)

type ProjectService interface {
	AnalyzeRequirement(context.Context, uuid.UUID, project.AnalyzeRequirementInput) (*project.Requirement, error)
	MatchProjectActors(context.Context, uuid.UUID, project.MatchProjectActorsInput) ([]organization.MemberMatchCandidate, error)
	BindProjectWorkflow(context.Context, uuid.UUID, project.BindProjectWorkflowInput) (*project.ProjectWorkflow, error)
	GetCostSummary(context.Context, uuid.UUID) (*project.CostSummary, error)
	CreateCostEntry(context.Context, uuid.UUID, project.CreateCostEntryInput) (*project.CostEntry, error)
}

type FinanceService interface {
	CreateExportBatch(context.Context, finance.CreateExportBatchInput) (*finance.ExportBatch, error)
}

type EvolutionService interface {
	CreateKnowledge(context.Context, evolution.CreateKnowledgeInput) (*evolution.KnowledgeEntry, error)
	CreateSignal(context.Context, evolution.CreateSignalInput) (*evolution.Signal, error)
	CreateExperiment(context.Context, evolution.CreateExperimentInput) (*evolution.Experiment, error)
}

type ERPActionService interface {
	RunAction(context.Context, string, string, string, erp.ActionInput) (*erp.ActionResult, error)
}

type IndustrySolutionChangeVerifier interface {
	VerifyIndustrySolutionChange(context.Context, uuid.UUID, uuid.UUID) (*systemadmin.IndustrySolutionVerificationReport, error)
}

type RuntimeOperationService interface {
	ExecuteOperation(context.Context, string, domainruntime.RuntimeExecutionRequest) (*domainruntime.RuntimeExecutionResult, error)
}

type ContextProposalService interface {
	ApplyContextProposal(context.Context, uuid.UUID, uuid.UUID, string) (map[string]any, error)
}

type PlatformToolServices struct {
	ERP                      ERPActionService
	IndustrySolutionVerifier IndustrySolutionChangeVerifier
	Runtime                  RuntimeOperationService
	ContextProposal          ContextProposalService
}

func InternalTools(projectSvc ProjectService, financeSvc FinanceService, evolutionSvc EvolutionService) map[string]ToolAdapter {
	return InternalToolsWithPlatform(projectSvc, financeSvc, evolutionSvc, PlatformToolServices{})
}

func InternalToolsWithPlatform(projectSvc ProjectService, financeSvc FinanceService, evolutionSvc EvolutionService, platform PlatformToolServices) map[string]ToolAdapter {
	tools := map[string]ToolAdapter{
		"governance.explain_decision": explainGovernanceDecision,
	}
	if financeSvc == nil {
		tools["finance.prepare_export_batch"] = notConfiguredTool("finance module is not available until finance integration is enabled")
	} else {
		tools["finance.prepare_export_batch"] = prepareFinanceExportBatchTool(financeSvc)
	}
	if evolutionSvc == nil {
		tools["evolution.create_knowledge"] = notConfiguredTool("evolution module is not configured")
		tools["evolution.create_signal"] = notConfiguredTool("evolution module is not configured")
		tools["evolution.propose_experiment"] = notConfiguredTool("evolution module is not configured")
	} else {
		tools["evolution.create_knowledge"] = createKnowledgeTool(evolutionSvc)
		tools["evolution.create_signal"] = createSignalTool(evolutionSvc)
		tools["evolution.propose_experiment"] = proposeExperimentTool(evolutionSvc)
	}
	if projectSvc == nil {
		tools["requirement.analyze"] = notConfiguredTool("project service is not configured")
		tools["project.match_members"] = notConfiguredTool("project service is not configured")
		tools["project.bind_workflow"] = notConfiguredTool("project service is not configured")
		tools["project.estimate_cost"] = notConfiguredTool("project service is not configured")
		tools["project.create_cost_entry"] = notConfiguredTool("project service is not configured")
	} else {
		tools["requirement.analyze"] = analyzeRequirementTool(projectSvc)
		tools["project.match_members"] = matchMembersTool(projectSvc)
		tools["project.bind_workflow"] = bindWorkflowTool(projectSvc)
		tools["project.estimate_cost"] = estimateCostTool(projectSvc)
		tools["project.create_cost_entry"] = createCostEntryTool(projectSvc)
	}
	if platform.ERP == nil {
		tools["erp.action.execute"] = notConfiguredTool("ERP action service is not configured")
	} else {
		tools["erp.action.execute"] = erpActionExecuteTool(platform.ERP)
	}
	if platform.IndustrySolutionVerifier == nil {
		tools["industry.solution.change.preview"] = notConfiguredTool("industry solution verifier is not configured")
	} else {
		tools["industry.solution.change.preview"] = industrySolutionChangePreviewTool(platform.IndustrySolutionVerifier)
	}
	if platform.Runtime == nil {
		tools["runtime.operation.execute"] = notConfiguredTool("runtime operation service is not configured")
	} else {
		tools["runtime.operation.execute"] = runtimeOperationExecuteTool(platform.Runtime)
	}
	if platform.ContextProposal == nil {
		tools["context.proposal.apply"] = notConfiguredTool("context proposal service is not configured")
	} else {
		tools["context.proposal.apply"] = contextProposalApplyTool(platform.ContextProposal)
	}
	return tools
}

func DefaultToolDefinitions() []CreateToolInput {
	return []CreateToolInput{
		{Name: "requirement.analyze", Description: "Analyze a requirement", SourceType: SourceInternalAPI, DefaultPolicy: PolicyNotify, RiskLevel: "medium", RequiredLevel: "L2", ToolCategory: ToolCategoryExecutionOperation, ApprovalTierRequired: ApprovalTierExecutor},
		{Name: "project.match_members", Description: "Recommend project members", SourceType: SourceInternalAPI, DefaultPolicy: PolicyNotify, RiskLevel: "medium", RequiredLevel: "L2", ToolCategory: ToolCategoryExecutionOperation, ApprovalTierRequired: ApprovalTierExecutor},
		{Name: "project.bind_workflow", Description: "Bind workflow to project", SourceType: SourceInternalAPI, DefaultPolicy: PolicyApprove, RiskLevel: "high", RequiredLevel: "L3", ToolCategory: ToolCategoryBusinessApproval, ApprovalTierRequired: ApprovalTierReviewer},
		{Name: "project.estimate_cost", Description: "Estimate project cost", SourceType: SourceInternalAPI, DefaultPolicy: PolicyNotify, RiskLevel: "medium", RequiredLevel: "L2", ToolCategory: ToolCategoryExecutionOperation, ApprovalTierRequired: ApprovalTierExecutor},
		{Name: "project.create_cost_entry", Description: "Create project cost entry", SourceType: SourceInternalAPI, DefaultPolicy: PolicyApprove, RiskLevel: "high", RequiredLevel: "L3", ToolCategory: ToolCategoryBusinessApproval, ApprovalTierRequired: ApprovalTierReviewer},
		{Name: "governance.explain_decision", Description: "Explain governance decision", SourceType: SourceInternalAPI, DefaultPolicy: PolicyNotify, RiskLevel: "low", RequiredLevel: "L1", ToolCategory: ToolCategoryCoreData, ApprovalTierRequired: ApprovalTierOrganizationCreator},
		{Name: "finance.prepare_export_batch", Description: "Prepare finance export batch", SourceType: SourceManualApproval, DefaultPolicy: PolicyApprove, RiskLevel: "high", RequiredLevel: "L3", ToolCategory: ToolCategoryBusinessApproval, ApprovalTierRequired: ApprovalTierReviewer},
		{Name: "evolution.create_knowledge", Description: "Create evolution knowledge entry", SourceType: SourceInternalAPI, DefaultPolicy: PolicyNotify, RiskLevel: "medium", RequiredLevel: "L2", ToolCategory: ToolCategoryExecutionOperation, ApprovalTierRequired: ApprovalTierExecutor},
		{Name: "evolution.create_signal", Description: "Create evolution signal", SourceType: SourceInternalAPI, DefaultPolicy: PolicyNotify, RiskLevel: "medium", RequiredLevel: "L2", ToolCategory: ToolCategoryExecutionOperation, ApprovalTierRequired: ApprovalTierExecutor},
		{Name: "evolution.propose_experiment", Description: "Propose evolution experiment", SourceType: SourceInternalAPI, DefaultPolicy: PolicyApprove, RiskLevel: "high", RequiredLevel: "L3", ToolCategory: ToolCategoryBusinessApproval, ApprovalTierRequired: ApprovalTierReviewer},
		{Name: "erp.action.execute", Description: "Execute an ERP business action", SourceType: SourceInternalAPI, DefaultPolicy: PolicyApprove, RiskLevel: "high", RequiredLevel: "L3", ToolCategory: ToolCategoryBusinessApproval, ApprovalTierRequired: ApprovalTierReviewer},
		{Name: "industry.solution.change.preview", Description: "Verify an industry solution change request without applying it", SourceType: SourceInternalAPI, DefaultPolicy: PolicyNotify, RiskLevel: "low", RequiredLevel: "L2", ToolCategory: ToolCategoryExecutionOperation, ApprovalTierRequired: ApprovalTierExecutor},
		{Name: "runtime.operation.execute", Description: "Execute a platform runtime operation", SourceType: SourceInternalAPI, DefaultPolicy: PolicyNotify, RiskLevel: "medium", RequiredLevel: "L2", ToolCategory: ToolCategoryExecutionOperation, ApprovalTierRequired: ApprovalTierExecutor},
		{Name: "context.proposal.apply", Description: "Apply an approved context change proposal", SourceType: SourceManualApproval, DefaultPolicy: PolicyApprove, RiskLevel: "high", RequiredLevel: "L3", ToolCategory: ToolCategoryBusinessApproval, ApprovalTierRequired: ApprovalTierReviewer},
	}
}

func analyzeRequirementTool(projectSvc ProjectService) ToolAdapter {
	return func(ctx context.Context, input ExecuteToolInput) (ToolResult, error) {
		requirementID, err := uuidArg(input.Arguments, "requirement_id")
		if err != nil {
			return ToolResult{}, err
		}
		req, err := projectSvc.AnalyzeRequirement(ctx, requirementID, project.AnalyzeRequirementInput{
			ActorInput: project.ActorInput{ActorID: &input.ActorID, ActorType: input.ActorType},
			Notes:      stringArg(input.Arguments, "notes"),
		})
		if err != nil {
			return ToolResult{}, err
		}
		return ToolResult{Summary: "Requirement analyzed", Data: map[string]any{"requirement": req}}, nil
	}
}

func matchMembersTool(projectSvc ProjectService) ToolAdapter {
	return func(ctx context.Context, input ExecuteToolInput) (ToolResult, error) {
		projectID, err := uuidArg(input.Arguments, "project_id")
		if err != nil {
			return ToolResult{}, err
		}
		result, err := projectSvc.MatchProjectActors(ctx, projectID, project.MatchProjectActorsInput{
			TaskDescription:      stringArg(input.Arguments, "task_description"),
			RequiredCapabilities: stringSliceArg(input.Arguments, "required_capabilities"),
			RequiredLevel:        stringArg(input.Arguments, "required_level"),
			RiskLevel:            stringArg(input.Arguments, "risk_level"),
			MemberTypes:          stringSliceArg(input.Arguments, "member_types"),
		})
		if err != nil {
			return ToolResult{}, err
		}
		return ToolResult{Summary: "Project members matched", Data: map[string]any{"candidates": result}}, nil
	}
}

func bindWorkflowTool(projectSvc ProjectService) ToolAdapter {
	return func(ctx context.Context, input ExecuteToolInput) (ToolResult, error) {
		projectID, err := uuidArg(input.Arguments, "project_id")
		if err != nil {
			return ToolResult{}, err
		}
		workflowTemplateID, err := optionalUUIDArg(input.Arguments, "workflow_template_id")
		if err != nil {
			return ToolResult{}, err
		}
		workflowID, err := optionalUUIDArg(input.Arguments, "workflow_id")
		if err != nil {
			return ToolResult{}, err
		}
		result, err := projectSvc.BindProjectWorkflow(ctx, projectID, project.BindProjectWorkflowInput{
			ActorInput:         project.ActorInput{ActorID: &input.ActorID, ActorType: input.ActorType},
			WorkflowID:         workflowID,
			WorkflowTemplateID: workflowTemplateID,
			Purpose:            stringArg(input.Arguments, "purpose"),
			Status:             stringArg(input.Arguments, "status"),
		})
		if err != nil {
			return ToolResult{}, err
		}
		return ToolResult{Summary: "Workflow bound", Data: map[string]any{"project_workflow": result}}, nil
	}
}

func estimateCostTool(projectSvc ProjectService) ToolAdapter {
	return func(ctx context.Context, input ExecuteToolInput) (ToolResult, error) {
		projectID, err := uuidArg(input.Arguments, "project_id")
		if err != nil {
			return ToolResult{}, err
		}
		summary, err := projectSvc.GetCostSummary(ctx, projectID)
		if err != nil {
			return ToolResult{}, err
		}
		return ToolResult{Summary: "Project cost estimated", Data: map[string]any{"cost_summary": summary}}, nil
	}
}

func createCostEntryTool(projectSvc ProjectService) ToolAdapter {
	return func(ctx context.Context, input ExecuteToolInput) (ToolResult, error) {
		projectID, err := uuidArg(input.Arguments, "project_id")
		if err != nil {
			return ToolResult{}, err
		}
		entry, err := projectSvc.CreateCostEntry(ctx, projectID, project.CreateCostEntryInput{
			ActorInput:  project.ActorInput{ActorID: &input.ActorID, ActorType: input.ActorType},
			SourceType:  stringArg(input.Arguments, "source_type"),
			Amount:      floatArg(input.Arguments, "amount"),
			Currency:    stringArg(input.Arguments, "currency"),
			Description: stringArg(input.Arguments, "description"),
			Metadata:    mapArg(input.Arguments, "metadata"),
		})
		if err != nil {
			return ToolResult{}, err
		}
		return ToolResult{Summary: "Project cost entry created", Data: map[string]any{"cost_entry": entry}}, nil
	}
}

func prepareFinanceExportBatchTool(financeSvc FinanceService) ToolAdapter {
	return func(ctx context.Context, input ExecuteToolInput) (ToolResult, error) {
		adapterID, err := uuidArg(input.Arguments, "adapter_id")
		if err != nil {
			return ToolResult{}, err
		}
		batch, err := financeSvc.CreateExportBatch(ctx, finance.CreateExportBatchInput{
			AdapterID:   adapterID,
			PeriodStart: stringArg(input.Arguments, "period_start"),
			PeriodEnd:   stringArg(input.Arguments, "period_end"),
			Currency:    stringArg(input.Arguments, "currency"),
			ActorID:     &input.ActorID,
			ActorType:   input.ActorType,
			Metadata:    mapArg(input.Arguments, "metadata"),
		})
		if err != nil {
			return ToolResult{}, err
		}
		return ToolResult{Summary: "Finance export batch prepared", Data: map[string]any{"export_batch": batch}}, nil
	}
}

func createKnowledgeTool(evolutionSvc EvolutionService) ToolAdapter {
	return func(ctx context.Context, input ExecuteToolInput) (ToolResult, error) {
		workflowID, err := optionalUUIDArg(input.Arguments, "workflow_id")
		if err != nil {
			return ToolResult{}, err
		}
		entry, err := evolutionSvc.CreateKnowledge(ctx, evolution.CreateKnowledgeInput{
			WorkflowID: workflowID,
			Title:      stringArg(input.Arguments, "title"),
			Content:    stringArg(input.Arguments, "content"),
			Tags:       stringSliceArg(input.Arguments, "tags"),
			Source:     "assistant",
		})
		if err != nil {
			return ToolResult{}, err
		}
		return ToolResult{Summary: "Evolution knowledge created", Data: map[string]any{"knowledge": entry}}, nil
	}
}

func createSignalTool(evolutionSvc EvolutionService) ToolAdapter {
	return func(ctx context.Context, input ExecuteToolInput) (ToolResult, error) {
		signalType := stringArg(input.Arguments, "signal_type")
		if signalType == "" {
			return ToolResult{}, fmt.Errorf("%w: signal_type is required", ErrValidation)
		}
		priority := intArg(input.Arguments, "priority")
		signal, err := evolutionSvc.CreateSignal(ctx, evolution.CreateSignalInput{
			SignalType: signalType,
			Source:     firstNonEmptyString(stringArg(input.Arguments, "source"), "assistant"),
			Priority:   priority,
			Data:       mapArg(input.Arguments, "data"),
		})
		if err != nil {
			return ToolResult{}, err
		}
		return ToolResult{Summary: "Evolution signal created", Data: map[string]any{"signal": signal}}, nil
	}
}

func proposeExperimentTool(evolutionSvc EvolutionService) ToolAdapter {
	return func(ctx context.Context, input ExecuteToolInput) (ToolResult, error) {
		experiment, err := evolutionSvc.CreateExperiment(ctx, evolution.CreateExperimentInput{
			Name:            stringArg(input.Arguments, "name"),
			Hypothesis:      stringArg(input.Arguments, "hypothesis"),
			SuccessCriteria: mapArg(input.Arguments, "success_criteria"),
		})
		if err != nil {
			return ToolResult{}, err
		}
		return ToolResult{Summary: "Evolution experiment proposed", Data: map[string]any{"experiment": experiment}}, nil
	}
}

func erpActionExecuteTool(erpSvc ERPActionService) ToolAdapter {
	return func(ctx context.Context, input ExecuteToolInput) (ToolResult, error) {
		tableCode := stringArg(input.Arguments, "table_code")
		key := firstNonEmptyString(stringArg(input.Arguments, "key"), stringArg(input.Arguments, "record_key"), stringArg(input.Arguments, "target_key"))
		action := stringArg(input.Arguments, "action")
		if tableCode == "" || key == "" || action == "" {
			return ToolResult{}, fmt.Errorf("%w: table_code, key, and action are required", ErrValidation)
		}
		toolExecutionID, err := optionalUUIDArg(input.Arguments, "tool_execution_id")
		if err != nil {
			return ToolResult{}, err
		}
		assistantSessionID, err := optionalUUIDArg(input.Arguments, "assistant_session_id")
		if err != nil {
			return ToolResult{}, err
		}
		actorID := input.ActorID
		result, err := erpSvc.RunAction(ctx, tableCode, key, action, erp.ActionInput{
			Data:               mapArg(input.Arguments, "data"),
			ActorID:            &actorID,
			ActorType:          input.ActorType,
			IdempotencyKey:     input.IdempotencyKey,
			Source:             "toolruntime",
			ToolExecutionID:    toolExecutionID,
			AssistantSessionID: assistantSessionID,
		})
		if err != nil {
			return ToolResult{}, err
		}
		return ToolResult{Summary: "ERP action executed", Data: map[string]any{"erp_action": result}}, nil
	}
}

func industrySolutionChangePreviewTool(verifier IndustrySolutionChangeVerifier) ToolAdapter {
	return func(ctx context.Context, input ExecuteToolInput) (ToolResult, error) {
		requestID, err := firstUUIDArg(input.Arguments, "request_id", "industry_solution_change_request_id", "id")
		if err != nil {
			return ToolResult{}, err
		}
		report, err := verifier.VerifyIndustrySolutionChange(ctx, input.ActorID, requestID)
		if err != nil {
			return ToolResult{}, err
		}
		return ToolResult{Summary: "Industry solution change verified", Data: map[string]any{"verification": report}}, nil
	}
}

func runtimeOperationExecuteTool(runtimeSvc RuntimeOperationService) ToolAdapter {
	return func(ctx context.Context, input ExecuteToolInput) (ToolResult, error) {
		operationID := firstNonEmptyString(stringArg(input.Arguments, "operation_id"), stringArg(input.Arguments, "id"))
		if operationID == "" {
			return ToolResult{}, fmt.Errorf("%w: operation_id is required", ErrValidation)
		}
		result, err := runtimeSvc.ExecuteOperation(ctx, operationID, domainruntime.RuntimeExecutionRequest{
			Path:  stringMapArg(input.Arguments, "path"),
			Query: stringMapArg(input.Arguments, "query"),
			Body:  mapArg(input.Arguments, "body"),
		})
		if err != nil {
			return ToolResult{}, err
		}
		return ToolResult{Summary: "Runtime operation executed", Data: map[string]any{"runtime_result": result}}, nil
	}
}

func contextProposalApplyTool(proposals ContextProposalService) ToolAdapter {
	return func(ctx context.Context, input ExecuteToolInput) (ToolResult, error) {
		proposalID, err := firstUUIDArg(input.Arguments, "proposal_id", "context_proposal_id", "id")
		if err != nil {
			return ToolResult{}, err
		}
		result, err := proposals.ApplyContextProposal(ctx, proposalID, input.ActorID, input.ActorType)
		if err != nil {
			return ToolResult{}, err
		}
		return ToolResult{Summary: "Context proposal applied", Data: map[string]any{"context_proposal": result}}, nil
	}
}

func explainGovernanceDecision(ctx context.Context, input ExecuteToolInput) (ToolResult, error) {
	return ToolResult{
		Summary: "Governance decision context prepared",
		Data: map[string]any{
			"decision_id": input.Arguments["decision_id"],
			"reason":      input.Arguments["reason"],
		},
	}, nil
}

func notConfiguredTool(message string) ToolAdapter {
	return func(context.Context, ExecuteToolInput) (ToolResult, error) {
		return ToolResult{}, fmt.Errorf("%w: %s", ErrNotFound, message)
	}
}

func uuidArg(args map[string]any, key string) (uuid.UUID, error) {
	raw := stringArg(args, key)
	if raw == "" {
		return uuid.Nil, fmt.Errorf("%w: %s is required", ErrValidation, key)
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: invalid %s", ErrValidation, key)
	}
	return id, nil
}

func optionalUUIDArg(args map[string]any, key string) (*uuid.UUID, error) {
	raw := stringArg(args, key)
	if raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid %s", ErrValidation, key)
	}
	return &id, nil
}

func firstUUIDArg(args map[string]any, keys ...string) (uuid.UUID, error) {
	for _, key := range keys {
		raw := stringArg(args, key)
		if raw == "" {
			continue
		}
		id, err := uuid.Parse(raw)
		if err != nil {
			return uuid.Nil, fmt.Errorf("%w: invalid %s", ErrValidation, key)
		}
		return id, nil
	}
	return uuid.Nil, fmt.Errorf("%w: %s is required", ErrValidation, strings.Join(keys, " or "))
}

func stringArg(args map[string]any, key string) string {
	if value, ok := args[key].(string); ok {
		return value
	}
	return ""
}

func stringSliceArg(args map[string]any, key string) []string {
	raw, ok := args[key].([]any)
	if !ok {
		if values, ok := args[key].([]string); ok {
			return values
		}
		return nil
	}
	values := make([]string, 0, len(raw))
	for _, item := range raw {
		if value, ok := item.(string); ok {
			values = append(values, value)
		}
	}
	return values
}

func floatArg(args map[string]any, key string) float64 {
	switch value := args[key].(type) {
	case float64:
		return value
	case int:
		return float64(value)
	default:
		return 0
	}
}

func intArg(args map[string]any, key string) int {
	switch value := args[key].(type) {
	case float64:
		return int(value)
	case int:
		return value
	default:
		return 0
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func mapArg(args map[string]any, key string) map[string]any {
	if value, ok := args[key].(map[string]any); ok {
		return value
	}
	return map[string]any{}
}

func stringMapArg(args map[string]any, key string) map[string]string {
	raw, ok := args[key].(map[string]any)
	if !ok {
		if values, ok := args[key].(map[string]string); ok {
			return values
		}
		return map[string]string{}
	}
	result := map[string]string{}
	for itemKey, value := range raw {
		result[itemKey] = fmt.Sprint(value)
	}
	return result
}
