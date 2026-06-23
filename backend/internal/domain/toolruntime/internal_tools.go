package toolruntime

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/evolution"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/finance"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/organization"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/project"
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

func InternalTools(projectSvc ProjectService, financeSvc FinanceService, evolutionSvc EvolutionService) map[string]ToolAdapter {
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
		return tools
	}
	tools["requirement.analyze"] = analyzeRequirementTool(projectSvc)
	tools["project.match_members"] = matchMembersTool(projectSvc)
	tools["project.bind_workflow"] = bindWorkflowTool(projectSvc)
	tools["project.estimate_cost"] = estimateCostTool(projectSvc)
	tools["project.create_cost_entry"] = createCostEntryTool(projectSvc)
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
