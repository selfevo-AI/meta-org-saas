package businessai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/aigateway"
)

var (
	ErrValidation    = errors.New("validation error")
	ErrInvalidOutput = errors.New("invalid ai output")
	ErrNotConfigured = errors.New("business ai is not configured")
	ErrConflict      = errors.New("conflict")
)

type AIInvoker interface {
	Invoke(context.Context, aigateway.InvokeInput) (*aigateway.InvokeOutput, error)
}

type ProposalExecutor interface {
	ExecuteProposal(context.Context, ProposalExecutionRequest) (*ProposalExecutionOutput, error)
}

type Config struct {
	ProviderType string
	Model        string
	MaxTokens    int
}

type RunRepository interface {
	CreateRun(context.Context, AnalyzeInput) (*Run, error)
	CompleteRun(context.Context, uuid.UUID, CompleteRunInput) (*Run, error)
	FailRun(context.Context, uuid.UUID, string) error
	ListRuns(context.Context, uuid.UUID, uuid.UUID, int) ([]Run, error)
	GetRun(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*Run, error)
	BeginProposalSubmission(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*Run, error)
	LinkProposalExecution(context.Context, uuid.UUID, ProposalExecutionOutput) (*Run, error)
	FailProposalSubmission(context.Context, uuid.UUID, string) error
	UpdateProposalExecution(context.Context, ProposalExecutionUpdate) error
}

type Service struct {
	repo             RunRepository
	ai               AIInvoker
	cfg              Config
	proposalExecutor ProposalExecutor
}

func (s *Service) SetProposalExecutor(executor ProposalExecutor) {
	s.proposalExecutor = executor
}

func NewService(repo RunRepository, ai AIInvoker, cfg Config) *Service {
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = 1800
	}
	return &Service{repo: repo, ai: ai, cfg: cfg}
}

func (s *Service) Analyze(ctx context.Context, input AnalyzeInput) (*Run, error) {
	input.Stage = strings.ToLower(strings.TrimSpace(input.Stage))
	if _, ok := ValidStages[input.Stage]; !ok {
		return nil, fmt.Errorf("%w: stage must be one of plan, do, change, accept, learn", ErrValidation)
	}
	input.ProviderType = firstNonEmpty(strings.TrimSpace(input.ProviderType), s.cfg.ProviderType)
	input.Model = firstNonEmpty(strings.TrimSpace(input.Model), s.cfg.Model)
	if input.Context == nil {
		input.Context = map[string]any{}
	}
	input.Context["stage"] = input.Stage
	input.Context["focus"] = strings.TrimSpace(input.Focus)
	run, err := s.repo.CreateRun(ctx, input)
	if err != nil {
		return nil, err
	}
	if s.ai == nil || input.ProviderType == "" || input.Model == "" {
		err := fmt.Errorf("%w: provider type and model are required", ErrNotConfigured)
		_ = s.repo.FailRun(ctx, run.ID, err.Error())
		return nil, err
	}

	contextJSON, err := json.Marshal(input.Context)
	if err != nil {
		_ = s.repo.FailRun(ctx, run.ID, err.Error())
		return nil, fmt.Errorf("marshal business ai context: %w", err)
	}
	userID, agentID := attributionActor(input.RequestedByID, input.RequestedByType)
	output, err := s.ai.Invoke(ctx, aigateway.InvokeInput{
		ProviderType: input.ProviderType,
		Model:        input.Model,
		Messages: []aigateway.Message{
			{Role: "system", Content: stageSystemPrompt(input.Stage)},
			{Role: "user", Content: string(contextJSON)},
		},
		MaxTokens: s.cfg.MaxTokens,
		Attribution: aigateway.Attribution{
			OrganizationID: &input.OrganizationID,
			ProjectID:      &input.ProjectID,
			RequirementID:  input.RequirementID,
			UserID:         userID,
			AgentID:        agentID,
			SourceSurface:  "business_stage_ai",
		},
		Metadata: map[string]any{"business_stage": input.Stage, "run_id": run.ID.String()},
	})
	if err != nil {
		_ = s.repo.FailRun(ctx, run.ID, err.Error())
		return nil, err
	}
	analysis, err := parseAnalysis(output.Content, input.Stage)
	if err != nil {
		_ = s.repo.FailRun(ctx, run.ID, err.Error())
		return nil, err
	}
	return s.repo.CompleteRun(ctx, run.ID, CompleteRunInput{
		InvocationID: output.InvocationID, ResolvedModel: output.Model, Analysis: analysis,
		CostAmount: output.CostAmount, Currency: output.Currency,
		InputTokens: output.Usage.InputTokens, OutputTokens: output.Usage.OutputTokens,
	})
}

func (s *Service) ListRuns(ctx context.Context, organizationID, projectID uuid.UUID, limit int) ([]Run, error) {
	return s.repo.ListRuns(ctx, organizationID, projectID, limit)
}

func (s *Service) SubmitProposal(ctx context.Context, input SubmitProposalInput) (*Run, error) {
	run, err := s.repo.GetRun(ctx, input.OrganizationID, input.ProjectID, input.RunID)
	if err != nil {
		return nil, err
	}
	if run.ToolExecutionID != nil {
		return run, nil
	}
	if run.Status != StatusCompleted || run.Analysis == nil {
		return nil, fmt.Errorf("%w: AI analysis must be completed before submitting its proposal", ErrConflict)
	}
	toolName := strings.TrimSpace(run.Analysis.Proposal.ToolName)
	if toolName == "" {
		return nil, fmt.Errorf("%w: AI analysis did not propose an executable tool", ErrValidation)
	}
	if err := validateStageProposal(run.Stage, run.Analysis.Proposal); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	if s.proposalExecutor == nil {
		return nil, fmt.Errorf("%w: proposal executor is not configured", ErrNotConfigured)
	}
	run, err = s.repo.BeginProposalSubmission(ctx, input.OrganizationID, input.ProjectID, input.RunID)
	if err != nil {
		current, getErr := s.repo.GetRun(ctx, input.OrganizationID, input.ProjectID, input.RunID)
		if getErr == nil && current.ToolExecutionID != nil {
			return current, nil
		}
		return nil, fmt.Errorf("%w: proposal is already being submitted or is terminal", ErrConflict)
	}
	arguments := copyArguments(run.Analysis.Proposal.Arguments)
	arguments["project_id"] = input.ProjectID.String()
	output, err := s.proposalExecutor.ExecuteProposal(ctx, ProposalExecutionRequest{
		ToolName: toolName, InvocationID: run.InvocationID, ActorID: input.ActorID, ActorType: input.ActorType,
		OrganizationID: input.OrganizationID, ProjectID: input.ProjectID,
		IdempotencyKey: "business-ai-proposal:" + input.RunID.String(), RequireApproval: true, Arguments: arguments,
	})
	if err != nil {
		_ = s.repo.FailProposalSubmission(ctx, run.ID, err.Error())
		return nil, err
	}
	return s.repo.LinkProposalExecution(ctx, run.ID, *output)
}

func (s *Service) UpdateProposalExecution(ctx context.Context, update ProposalExecutionUpdate) error {
	return s.repo.UpdateProposalExecution(ctx, update)
}

func copyArguments(input map[string]any) map[string]any {
	result := make(map[string]any, len(input)+1)
	for key, value := range input {
		result[key] = value
	}
	return result
}

func parseAnalysis(content string, stage string) (Analysis, error) {
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSuffix(strings.TrimSpace(content), "```")
	}
	var analysis Analysis
	decoder := json.NewDecoder(strings.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&analysis); err != nil {
		return Analysis{}, fmt.Errorf("%w: %v", ErrInvalidOutput, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return Analysis{}, fmt.Errorf("%w: trailing content", ErrInvalidOutput)
	}
	if strings.TrimSpace(analysis.Summary) == "" || analysis.Confidence < 0 || analysis.Confidence > 1 {
		return Analysis{}, fmt.Errorf("%w: summary is required and confidence must be between 0 and 1", ErrInvalidOutput)
	}
	if analysis.Findings == nil || analysis.Recommendations == nil || analysis.Risks == nil || analysis.EvidenceRefs == nil {
		return Analysis{}, fmt.Errorf("%w: findings, recommendations, risks and evidence_refs must be arrays", ErrInvalidOutput)
	}
	if analysis.Proposal.Arguments == nil {
		analysis.Proposal.Arguments = map[string]any{}
	}
	if strings.TrimSpace(analysis.Proposal.ToolName) != "" {
		analysis.Proposal.RequiresApproval = true
	}
	if err := validateStageProposal(stage, analysis.Proposal); err != nil {
		return Analysis{}, fmt.Errorf("%w: %v", ErrInvalidOutput, err)
	}
	return analysis, nil
}

func stageSystemPrompt(stage string) string {
	return `You are the business-stage decision analyst for an ERP project. Analyze only the supplied verified context for stage "` + stage + `". Never invent facts. Return exactly one JSON object with this schema and no markdown: {"summary":"string","findings":[{"title":"string","evidence":"string","impact":"string"}],"recommendations":[{"title":"string","rationale":"string","priority":"high|medium|low"}],"risks":[{"title":"string","probability":"high|medium|low","impact":"string","mitigation":"string"}],"proposal":{"action":"string","tool_name":"string","arguments":{},"requires_approval":true},"confidence":0.0,"evidence_refs":["JSON path in supplied context"]}. The proposal is advisory only and every executable proposal requires approval. Use an empty tool_name when no executable action is justified. Allowed tools and required arguments for this stage: ` + stageToolPrompt(stage)
}

var stageToolContracts = map[string]map[string][]string{
	StagePlan: {
		"project.match_members": {"task_description"},
		"project.bind_workflow": {"workflow_reference"},
		"project.estimate_cost": {},
	},
	StageDo: {
		"project.create_deliverable": {"name"},
		"project.create_cost_entry":  {"amount"},
		"project.estimate_cost":      {},
		"project.update_status":      {"status"},
	},
	StageChange: {
		"project.update_status": {"status"},
		"project.bind_workflow": {"workflow_reference"},
		"project.match_members": {"task_description"},
	},
	StageAccept: {
		"project.accept_deliverable": {"deliverable_id"},
		"project.close_feedback":     {"conclusion"},
		"project.estimate_cost":      {},
	},
	StageLearn: {
		"evolution.create_knowledge":   {"title", "content"},
		"evolution.create_signal":      {"signal_type"},
		"evolution.propose_experiment": {"name", "hypothesis"},
	},
}

func validateStageProposal(stage string, proposal Proposal) error {
	toolName := strings.TrimSpace(proposal.ToolName)
	if toolName == "" {
		return nil
	}
	contract, ok := stageToolContracts[stage][toolName]
	if !ok {
		return fmt.Errorf("tool %q is not allowed for stage %q", toolName, stage)
	}
	if strings.TrimSpace(proposal.Action) == "" {
		return fmt.Errorf("proposal action is required when tool_name is set")
	}
	for _, required := range contract {
		if required == "workflow_reference" {
			if argumentText(proposal.Arguments, "workflow_id") == "" && argumentText(proposal.Arguments, "workflow_template_id") == "" {
				return fmt.Errorf("tool %q requires workflow_id or workflow_template_id", toolName)
			}
			continue
		}
		if required == "amount" {
			if value, ok := proposal.Arguments[required]; !ok || numericArgument(value) <= 0 {
				return fmt.Errorf("tool %q requires a positive amount", toolName)
			}
			continue
		}
		if argumentText(proposal.Arguments, required) == "" {
			return fmt.Errorf("tool %q requires %s", toolName, required)
		}
	}
	return nil
}

func argumentText(arguments map[string]any, key string) string {
	if arguments == nil {
		return ""
	}
	value, ok := arguments[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func numericArgument(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		parsed, _ := typed.Float64()
		return parsed
	default:
		var parsed float64
		_, _ = fmt.Sscan(fmt.Sprint(value), &parsed)
		return parsed
	}
}

func stageToolPrompt(stage string) string {
	switch stage {
	case StagePlan:
		return `project.match_members(task_description, required_capabilities, required_level, risk_level, member_types); project.bind_workflow(workflow_id or workflow_template_id, purpose, status); project.estimate_cost().`
	case StageDo:
		return `project.create_deliverable(name, deliverable_type, uri, version, status, evidence, metadata); project.create_cost_entry(amount, source_type, currency, description, metadata); project.estimate_cost(); project.update_status(status, note).`
	case StageChange:
		return `project.update_status(status, note); project.bind_workflow(workflow_id or workflow_template_id, purpose, status); project.match_members(task_description, required_capabilities, required_level, risk_level, member_types).`
	case StageAccept:
		return `project.accept_deliverable(deliverable_id, evidence, metadata, reason); project.close_feedback(conclusion, outcome_score, metadata); project.estimate_cost().`
	case StageLearn:
		return `evolution.create_knowledge(title, content, workflow_id, tags); evolution.create_signal(signal_type, source, priority, data); evolution.propose_experiment(name, hypothesis, success_criteria).`
	default:
		return `no executable tools.`
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func attributionActor(actorID *uuid.UUID, actorType string) (*uuid.UUID, *uuid.UUID) {
	if actorID == nil {
		return nil, nil
	}
	if strings.Contains(strings.ToLower(actorType), "agent") {
		return nil, actorID
	}
	return actorID, nil
}
