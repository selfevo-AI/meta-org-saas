package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/aigateway"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/toolruntime"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/securitykernel"
)

var (
	ErrValidation = errors.New("validation error")
	ErrNotFound   = errors.New("not found")
	ErrForbidden  = errors.New("forbidden")
)

type AIInvoker interface {
	Invoke(context.Context, aigateway.InvokeInput) (*aigateway.InvokeOutput, error)
}

type ToolExecutor interface {
	ExecuteTool(context.Context, toolruntime.ExecuteToolInput) (*toolruntime.ExecuteToolOutput, error)
	ListTools(context.Context, int) ([]toolruntime.ToolDefinition, error)
	GetApproval(context.Context, uuid.UUID) (*toolruntime.ToolApproval, error)
	GetExecution(context.Context, uuid.UUID) (*toolruntime.ToolExecution, error)
}

type ProposalApplicator interface {
	ApplyProposal(context.Context, *Proposal) (map[string]any, error)
}

type Service struct {
	repo               Repository
	ai                 AIInvoker
	tools              ToolExecutor
	contextResolver    ContextResolver
	proposalApplicator ProposalApplicator
	dictionary         *DictionaryService
	contextEngine      ContextPackageBuilder
	runtime            AssistantRuntimeRunner
	securityKernel     securitykernel.Client
	maxTurns           int
	maxHistory         int
}

type ServiceOption func(*Service)

func WithContextResolver(resolver ContextResolver) ServiceOption {
	return func(s *Service) {
		s.contextResolver = resolver
	}
}

func WithProposalApplicator(applicator ProposalApplicator) ServiceOption {
	return func(s *Service) {
		s.proposalApplicator = applicator
	}
}

func WithDictionaryService(dictionary *DictionaryService) ServiceOption {
	return func(s *Service) {
		s.dictionary = dictionary
	}
}

func WithVerifiedContextEngine(engine ContextPackageBuilder) ServiceOption {
	return func(s *Service) {
		s.contextEngine = engine
	}
}

func WithAssistantRuntime(runtime AssistantRuntimeRunner) ServiceOption {
	return func(s *Service) {
		s.runtime = runtime
	}
}

func WithSecurityKernel(client securitykernel.Client) ServiceOption {
	return func(s *Service) {
		s.securityKernel = client
	}
}

func NewService(repo Repository, ai AIInvoker, tools ToolExecutor, opts ...ServiceOption) *Service {
	s := &Service{repo: repo, ai: ai, tools: tools, maxTurns: 12, maxHistory: 40}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Service) SetRuntime(runtime AssistantRuntimeRunner) {
	s.runtime = runtime
}

func (s *Service) CreateSession(ctx context.Context, actorID uuid.UUID, actorType string, input CreateSessionInput) (*Session, error) {
	input.ModuleKey = normalizedModule(input.ModuleKey)
	input.TargetType = strings.TrimSpace(input.TargetType)
	if input.Mode == "" {
		input.Mode = ModeBusinessProcess
	}
	if input.Mode != ModeBusinessProcess && input.Mode != ModeSelfEvolution {
		return nil, fmt.Errorf("%w: unsupported assistant mode", ErrValidation)
	}
	if actorID == uuid.Nil || actorType == "" {
		return nil, fmt.Errorf("%w: actor is required", ErrValidation)
	}
	if err := applyTenantSessionScope(ctx, &input); err != nil {
		return nil, err
	}
	if input.AutoModel {
		if err := s.applyDefaultModel(ctx, &input); err != nil {
			return nil, err
		}
	}
	return s.repo.CreateSession(ctx, actorID, actorType, input)
}

func (s *Service) applyDefaultModel(ctx context.Context, input *CreateSessionInput) error {
	if input == nil {
		return fmt.Errorf("%w: session input is required", ErrValidation)
	}
	if input.AgentID != nil && (input.ProviderID != nil || input.ProviderType != "") && input.Model != "" {
		return nil
	}
	var selected *ModuleDefault
	if s.repo != nil {
		if item, err := s.repo.GetModuleDefault(ctx, input.ModuleKey, input.TargetType); err == nil {
			selected = item
		} else if !isNotFound(err) {
			return err
		}
		if selected == nil {
			if item, err := s.repo.FindDefaultModel(ctx); err == nil {
				selected = item
			} else if !isNotFound(err) {
				return err
			}
		}
	}
	if selected == nil {
		return fmt.Errorf("%w: no default assistant model configured", ErrValidation)
	}
	if input.AgentID == nil {
		input.AgentID = selected.AgentID
	}
	if input.ProviderID == nil {
		input.ProviderID = selected.ProviderID
	}
	if input.PreferredChannelID == nil {
		input.PreferredChannelID = selected.PreferredChannelID
	}
	if input.ProviderType == "" {
		input.ProviderType = selected.ProviderType
	}
	if input.Model == "" {
		input.Model = selected.Model
	}
	if input.ServiceTier == "" {
		input.ServiceTier = selected.ServiceTier
	}
	if input.ReasoningEffort == "" {
		input.ReasoningEffort = selected.ReasoningEffort
	}
	return nil
}

func (s *Service) ListSessions(ctx context.Context, actorID uuid.UUID, actorType string, moduleKey string, limit int) ([]Session, error) {
	return s.repo.ListSessions(ctx, actorID, actorType, normalizedOptionalModule(moduleKey), limit)
}

func (s *Service) GetSession(ctx context.Context, id uuid.UUID, actorID uuid.UUID, actorType string) (*Session, error) {
	return s.repo.GetSession(ctx, id, actorID, actorType)
}

func (s *Service) ListSteps(ctx context.Context, sessionID uuid.UUID, actorID uuid.UUID, actorType string, limit int) ([]Step, error) {
	if _, err := s.repo.GetSession(ctx, sessionID, actorID, actorType); err != nil {
		return nil, err
	}
	return s.repo.ListSteps(ctx, sessionID, limit)
}

func (s *Service) ListContextTargets(ctx context.Context, moduleKey string, targetType string, limit int) ([]WorkRecord, error) {
	moduleKey = normalizedModule(moduleKey)
	if s.contextResolver == nil {
		return []WorkRecord{}, nil
	}
	session := &Session{ModuleKey: moduleKey, TargetType: strings.TrimSpace(targetType), OrganizationID: currentTenantOrganizationID(ctx)}
	context := s.contextResolver.Resolve(ctx, session)
	if context.Error != "" {
		return nil, fmt.Errorf("%w: %s", ErrValidation, context.Error)
	}
	if limit <= 0 || limit > len(context.Records) {
		return context.Records, nil
	}
	return context.Records[:limit], nil
}

type ImportDictionaryInput struct {
	SourceType string `json:"source_type"`
	SourceName string `json:"source_name"`
	Content    string `json:"content"`
	ScopeLevel string `json:"scope_level"`
	ModuleKey  string `json:"module_key"`
	VersionKey string `json:"version_key"`
}

func (s *Service) ImportDictionary(ctx context.Context, actorID uuid.UUID, input ImportDictionaryInput) (*DictionaryImportResult, error) {
	if s.dictionary == nil {
		return nil, fmt.Errorf("%w: dictionary service is not configured", ErrValidation)
	}
	model, err := NormalizeDictionaryImport(DictionaryImportSource{
		SourceType: input.SourceType,
		SourceName: input.SourceName,
		Content:    []byte(input.Content),
		ScopeLevel: input.ScopeLevel,
		ModuleKey:  input.ModuleKey,
		VersionKey: input.VersionKey,
	})
	if err != nil {
		return nil, err
	}
	return s.dictionary.Import(ctx, DictionaryImportRequest{Model: model, ImportedBy: &actorID})
}

func (s *Service) PreviewContext(ctx context.Context, actorID uuid.UUID, actorType string, request ContextRequest) (*ContextPackage, error) {
	if s.contextEngine == nil {
		return nil, fmt.Errorf("%w: context engine is not configured", ErrValidation)
	}
	request.ActorID = actorID
	request.ActorType = actorType
	return s.contextEngine.BuildContextPackage(ctx, request)
}

func (s *Service) ListProposals(ctx context.Context, sessionID uuid.UUID, actorID uuid.UUID, actorType string, limit int) ([]Proposal, error) {
	if _, err := s.repo.GetSession(ctx, sessionID, actorID, actorType); err != nil {
		return nil, err
	}
	return s.repo.ListProposals(ctx, sessionID, limit)
}

func (s *Service) ConfirmProposal(ctx context.Context, proposalID uuid.UUID, reviewerID uuid.UUID, reviewerType string) (*Proposal, error) {
	if proposalID == uuid.Nil || reviewerID == uuid.Nil {
		return nil, fmt.Errorf("%w: proposal and reviewer are required", ErrValidation)
	}
	if !isHumanActor(reviewerType) {
		return nil, fmt.Errorf("%w: only human users can confirm assistant proposals", ErrValidation)
	}
	proposal, err := s.repo.GetProposal(ctx, proposalID)
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.GetSession(ctx, proposal.SessionID, reviewerID, reviewerType); err != nil {
		return nil, err
	}
	switch proposal.Status {
	case ProposalApplied:
		return proposal, nil
	case ProposalRejected:
		return nil, fmt.Errorf("%w: rejected proposal cannot be confirmed", ErrValidation)
	case ProposalPending, "":
	default:
		return nil, fmt.Errorf("%w: proposal is not confirmable", ErrValidation)
	}
	if s.proposalApplicator == nil {
		return nil, fmt.Errorf("%w: proposal applicator is not configured", ErrValidation)
	}
	result, err := s.proposalApplicator.ApplyProposal(ctx, proposal)
	if err != nil {
		return nil, err
	}
	return s.repo.MarkProposalApplied(ctx, proposal.ID, reviewerID, result)
}

func (s *Service) RejectProposal(ctx context.Context, proposalID uuid.UUID, reviewerID uuid.UUID, reviewerType string, reason string) (*Proposal, error) {
	if proposalID == uuid.Nil || reviewerID == uuid.Nil {
		return nil, fmt.Errorf("%w: proposal and reviewer are required", ErrValidation)
	}
	if !isHumanActor(reviewerType) {
		return nil, fmt.Errorf("%w: only human users can reject assistant proposals", ErrValidation)
	}
	proposal, err := s.repo.GetProposal(ctx, proposalID)
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.GetSession(ctx, proposal.SessionID, reviewerID, reviewerType); err != nil {
		return nil, err
	}
	if proposal.Status == ProposalApplied {
		return nil, fmt.Errorf("%w: applied proposal cannot be rejected", ErrValidation)
	}
	if proposal.Status == ProposalRejected {
		return proposal, nil
	}
	return s.repo.MarkProposalRejected(ctx, proposal.ID, reviewerID, strings.TrimSpace(reason))
}

func (s *Service) CreateBusinessSkill(ctx context.Context, actorID uuid.UUID, actorType string, input CreateBusinessSkillInput) (*BusinessSkill, error) {
	if actorID == uuid.Nil || !isHumanActor(actorType) {
		return nil, fmt.Errorf("%w: only human users can create business skills", ErrValidation)
	}
	input.ModuleKey = normalizedModule(input.ModuleKey)
	input.TargetType = strings.TrimSpace(input.TargetType)
	input.BusinessFunctionKey = strings.TrimSpace(input.BusinessFunctionKey)
	input.Name = strings.TrimSpace(input.Name)
	input.PromptTemplate = strings.TrimSpace(input.PromptTemplate)
	if input.Name == "" || input.PromptTemplate == "" {
		return nil, fmt.Errorf("%w: name and prompt_template are required", ErrValidation)
	}
	if err := applySkillScopeDefaults(ctx, actorID, &input); err != nil {
		return nil, err
	}
	if err := validateSkillComponents(input.SkillComponents); err != nil {
		return nil, err
	}
	if input.InputSchema == nil {
		input.InputSchema = map[string]any{}
	}
	if input.OutputSchema == nil {
		input.OutputSchema = map[string]any{}
	}
	if input.PermissionPolicy == nil {
		input.PermissionPolicy = map[string]any{}
	}
	if input.ContextPolicy == nil {
		input.ContextPolicy = map[string]any{}
	}
	if input.PricingPolicy == nil {
		input.PricingPolicy = map[string]any{}
	}
	if input.ActivationPolicy == nil {
		input.ActivationPolicy = map[string]any{}
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
	if err := s.authorizeSkillWithKernel(ctx, actorID, actorType, "create", &BusinessSkill{
		ScopeLevel:          input.ScopeLevel,
		DeploymentMode:      input.DeploymentMode,
		OrganizationID:      input.OrganizationID,
		OwnerUserID:         input.OwnerUserID,
		ModuleKey:           input.ModuleKey,
		TargetType:          input.TargetType,
		BusinessFunctionKey: input.BusinessFunctionKey,
	}); err != nil {
		return nil, err
	}
	return s.repo.CreateBusinessSkill(ctx, input, actorID, actorType)
}

func (s *Service) ListBusinessSkills(ctx context.Context, moduleKey string, targetType string) ([]BusinessSkill, error) {
	return s.repo.ListBusinessSkills(ctx, normalizedOptionalModule(moduleKey), strings.TrimSpace(targetType), 100)
}

func (s *Service) ActivateBusinessSkill(ctx context.Context, id uuid.UUID, reviewerID uuid.UUID, reviewerType string) (*BusinessSkill, error) {
	if id == uuid.Nil || reviewerID == uuid.Nil {
		return nil, fmt.Errorf("%w: skill and reviewer are required", ErrValidation)
	}
	if !isHumanActor(reviewerType) {
		return nil, fmt.Errorf("%w: only human users can activate business skills", ErrValidation)
	}
	skill, err := s.repo.GetBusinessSkill(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := authorizeSkillActivation(ctx, skill); err != nil {
		return nil, err
	}
	if err := s.authorizeSkillWithKernel(ctx, reviewerID, reviewerType, "activate", skill); err != nil {
		return nil, err
	}
	return s.repo.ActivateBusinessSkill(ctx, id, reviewerID)
}

func (s *Service) RunBusinessSkill(ctx context.Context, id uuid.UUID, actorID uuid.UUID, actorType string, input map[string]any) (*SkillRun, error) {
	if id == uuid.Nil || actorID == uuid.Nil {
		return nil, fmt.Errorf("%w: skill and actor are required", ErrValidation)
	}
	skill, err := s.repo.GetBusinessSkill(ctx, id)
	if err != nil {
		return nil, err
	}
	if skill.Status != SkillActive {
		return nil, fmt.Errorf("%w: skill is not active", ErrValidation)
	}
	if err := authorizeSkillUse(ctx, skill); err != nil {
		return nil, err
	}
	if err := s.authorizeSkillWithKernel(ctx, actorID, actorType, "run", skill); err != nil {
		return nil, err
	}
	if input == nil {
		input = map[string]any{}
	}
	sessionID := uuidFromInput(input, "session_id")
	targetID := uuidFromInput(input, "target_id")
	targetType := firstNonEmpty(stringFromInput(input, "target_type"), skill.TargetType)
	output := map[string]any{
		"prompt_template":   skill.PromptTemplate,
		"trigger_intent":    skill.TriggerIntent,
		"tool_allowlist":    skill.ToolAllowlist,
		"skill_components":  skill.SkillComponents,
		"permission_policy": skill.PermissionPolicy,
		"context_policy":    skill.ContextPolicy,
		"pricing_policy":    skill.PricingPolicy,
		"activation_policy": skill.ActivationPolicy,
		"scope_level":       skill.ScopeLevel,
		"deployment_mode":   skill.DeploymentMode,
		"organization_id":   uuidString(skill.OrganizationID),
		"business_function": skill.BusinessFunctionKey,
	}
	return s.repo.CreateSkillRun(ctx, CreateSkillRunInput{
		SkillID:       skill.ID,
		SessionID:     sessionID,
		ModuleKey:     skill.ModuleKey,
		TargetType:    targetType,
		TargetID:      targetID,
		Input:         input,
		Output:        output,
		Status:        StatusCompleted,
		CreatedBy:     &actorID,
		CreatedByType: actorType,
	})
}

func (s *Service) Run(ctx context.Context, sessionID uuid.UUID, actorID uuid.UUID, actorType string, input RunInput) (<-chan RunEvent, error) {
	if s.runtime != nil {
		return s.runtime.Run(ctx, AssistantRunRequest{SessionID: sessionID, ActorID: actorID, ActorType: actorType, Input: input})
	}
	return s.runLegacy(ctx, sessionID, actorID, actorType, input)
}

func (s *Service) runLegacy(ctx context.Context, sessionID uuid.UUID, actorID uuid.UUID, actorType string, input RunInput) (<-chan RunEvent, error) {
	return s.runWithContextEngine(ctx, sessionID, actorID, actorType, input, s.contextEngine)
}

func (s *Service) runWithContextEngine(ctx context.Context, sessionID uuid.UUID, actorID uuid.UUID, actorType string, input RunInput, engine ContextPackageBuilder) (<-chan RunEvent, error) {
	if strings.TrimSpace(input.Message) == "" {
		return nil, fmt.Errorf("%w: message is required", ErrValidation)
	}
	if s.ai == nil {
		return nil, fmt.Errorf("%w: ai gateway is not configured", ErrValidation)
	}
	if s.tools == nil {
		return nil, fmt.Errorf("%w: tool runtime is not configured", ErrValidation)
	}
	session, err := s.repo.GetSession(ctx, sessionID, actorID, actorType)
	if err != nil {
		return nil, err
	}
	if err := s.repo.UpdateSessionStatus(ctx, session.ID, StatusRunning, ""); err != nil {
		return nil, err
	}

	events := make(chan RunEvent)
	go s.runLoop(ctx, events, session, input, engine)
	return events, nil
}

func (s *Service) Resume(ctx context.Context, sessionID uuid.UUID, actorID uuid.UUID, actorType string, input ResumeInput) (<-chan RunEvent, error) {
	if s.runtime != nil {
		return s.runtime.Resume(ctx, AssistantResumeRequest{SessionID: sessionID, ActorID: actorID, ActorType: actorType, Input: input})
	}
	return s.resumeLegacy(ctx, sessionID, actorID, actorType, input)
}

func (s *Service) resumeLegacy(ctx context.Context, sessionID uuid.UUID, actorID uuid.UUID, actorType string, input ResumeInput) (<-chan RunEvent, error) {
	return s.resumeWithContextEngine(ctx, sessionID, actorID, actorType, input, s.contextEngine)
}

func (s *Service) resumeWithContextEngine(ctx context.Context, sessionID uuid.UUID, actorID uuid.UUID, actorType string, input ResumeInput, engine ContextPackageBuilder) (<-chan RunEvent, error) {
	if input.ToolApprovalID == uuid.Nil {
		return nil, fmt.Errorf("%w: tool_approval_id is required", ErrValidation)
	}
	if s.ai == nil {
		return nil, fmt.Errorf("%w: ai gateway is not configured", ErrValidation)
	}
	if s.tools == nil {
		return nil, fmt.Errorf("%w: tool runtime is not configured", ErrValidation)
	}
	session, err := s.repo.GetSession(ctx, sessionID, actorID, actorType)
	if err != nil {
		return nil, err
	}
	if err := s.repo.UpdateSessionStatus(ctx, session.ID, StatusRunning, ""); err != nil {
		return nil, err
	}
	events := make(chan RunEvent)
	go s.resumeLoop(ctx, events, session, input, engine)
	return events, nil
}

type assistantTurnContext struct {
	Package     *ContextPackage
	WorkContext WorkRecordContext
	Metadata    map[string]any
}

func (s *Service) buildAssistantTurnContext(ctx context.Context, session *Session, intent string, tokenBudget int, engine ContextPackageBuilder) (assistantTurnContext, error) {
	if engine == nil {
		engine = s.contextEngine
	}
	if engine != nil {
		pkg, err := engine.BuildContextPackage(ctx, ContextRequest{
			SessionID:      session.ID,
			ActorID:        session.ActorID,
			ActorType:      session.ActorType,
			OrganizationID: session.OrganizationID,
			ModuleKey:      session.ModuleKey,
			WorkflowID:     session.WorkflowID,
			TaskID:         session.TaskID,
			TargetType:     session.TargetType,
			TargetID:       session.TargetID,
			Intent:         intent,
			Mode:           session.Mode,
			TokenBudget:    firstPositive(tokenBudget, 4096),
		})
		if err != nil {
			return assistantTurnContext{}, err
		}
		return assistantTurnContext{
			Package:     pkg,
			WorkContext: contextPackageToWorkRecordContext(session.ModuleKey, pkg),
			Metadata:    contextPackageMetadata(pkg),
		}, nil
	}
	workContext := WorkRecordContext{ModuleKey: session.ModuleKey}
	if s.contextResolver != nil {
		workContext = s.contextResolver.Resolve(ctx, session)
	}
	return assistantTurnContext{
		WorkContext: workContext,
		Metadata: map[string]any{
			"context_record_count": len(workContext.Records),
			"context_error":        workContext.Error,
			"context_source":       "compatibility_resolver",
		},
	}, nil
}

func (s *Service) runLoop(ctx context.Context, events chan<- RunEvent, session *Session, input RunInput, engine ContextPackageBuilder) {
	defer close(events)
	send := func(event RunEvent) bool {
		select {
		case <-ctx.Done():
			return false
		case events <- event:
			return true
		}
	}
	fail := func(err error, turn int) {
		_ = s.repo.UpdateSessionStatus(context.Background(), session.ID, StatusFailed, err.Error())
		step, _ := s.repo.AddStep(context.Background(), session, AddStepInput{StepType: StepError, Status: StatusFailed, Summary: err.Error(), Turn: turn})
		send(RunEvent{Type: "error", Step: step, Error: err.Error(), Done: true})
	}

	userMessage, err := s.repo.AddMessage(ctx, session.ID, "user", input.Message, "", "", nil)
	if err != nil {
		fail(err, 0)
		return
	}
	if !send(RunEvent{Type: "message", Message: userMessage}) {
		return
	}

	scope := sessionScope(session)
	memories, err := s.repo.ListScopedMemories(ctx, scope, session.ActorID, session.ActorType, 12)
	if err != nil {
		fail(err, 0)
		return
	}
	turnContext, err := s.buildAssistantTurnContext(ctx, session, input.Message, 4096, engine)
	if err != nil {
		fail(err, 0)
		return
	}
	history, err := s.repo.ListMessages(ctx, session.ID, s.maxHistory)
	if err != nil {
		fail(err, 0)
		return
	}
	toolDefs, err := s.tools.ListTools(ctx, 100)
	if err != nil {
		fail(err, 0)
		return
	}

	messages := buildAIMessages(session, memories, history, turnContext.WorkContext)
	tools := gatewayTools(toolDefs)
	providerID, channelID, providerType, model, serviceTier, effort := runModelConfig(session, input)
	if (providerID == nil && providerType == "") || model == "" {
		fail(fmt.Errorf("%w: model provider and model are required", ErrValidation), 0)
		return
	}

	s.continueAssistantTurns(ctx, send, fail, session, messages, turnContext, tools, providerID, channelID, providerType, model, serviceTier, effort, 1)
}

func (s *Service) resumeLoop(ctx context.Context, events chan<- RunEvent, session *Session, input ResumeInput, engine ContextPackageBuilder) {
	defer close(events)
	send := func(event RunEvent) bool {
		select {
		case <-ctx.Done():
			return false
		case events <- event:
			return true
		}
	}
	fail := func(err error, turn int) {
		_ = s.repo.UpdateSessionStatus(context.Background(), session.ID, StatusFailed, err.Error())
		step, _ := s.repo.AddStep(context.Background(), session, AddStepInput{StepType: StepError, Status: StatusFailed, Summary: err.Error(), Turn: turn})
		send(RunEvent{Type: "error", Step: step, Error: err.Error(), Done: true})
	}

	approval, err := s.tools.GetApproval(ctx, input.ToolApprovalID)
	if err != nil {
		fail(err, 0)
		return
	}
	if approval.Status != toolruntime.ApprovalApproved {
		fail(fmt.Errorf("%w: tool approval is not approved", ErrValidation), 0)
		return
	}
	execution, err := s.tools.GetExecution(ctx, approval.ExecutionID)
	if err != nil {
		fail(err, 0)
		return
	}
	if execution.Status != toolruntime.ExecutionCompleted {
		fail(fmt.Errorf("%w: approved tool execution is %s", ErrValidation, execution.Status), 0)
		return
	}
	steps, err := s.repo.ListSteps(ctx, session.ID, s.maxHistory)
	if err != nil {
		fail(err, 0)
		return
	}
	callID, toolName, turn := approvalStepContext(steps, input.ToolApprovalID, execution.ID)
	turnContext, err := s.buildAssistantTurnContext(ctx, session, "resume_after_tool_approval", 4096, engine)
	if err != nil {
		fail(err, turn)
		return
	}
	payload := map[string]any{"status": execution.Status, "summary": execution.ResultSummary, "result": execution.Result, "error": execution.ErrorMessage}
	for key, value := range turnContext.Metadata {
		payload[key] = value
	}
	resultStep, err := s.repo.AddStep(ctx, session, AddStepInput{
		ToolExecutionID: &execution.ID,
		ToolApprovalID:  &approval.ID,
		StepType:        StepToolResult,
		Status:          execution.Status,
		Summary:         execution.ResultSummary,
		Data:            payload,
		Turn:            turn,
	})
	if err != nil {
		fail(err, turn)
		return
	}
	if !send(RunEvent{Type: "tool_result", Step: resultStep, Data: payload}) {
		return
	}
	toolContent := marshalToolContent(payload)
	if _, err := s.repo.AddMessage(ctx, session.ID, "tool", toolContent, callID, toolName, payload); err != nil {
		fail(err, turn)
		return
	}

	scope := sessionScope(session)
	memories, err := s.repo.ListScopedMemories(ctx, scope, session.ActorID, session.ActorType, 12)
	if err != nil {
		fail(err, turn)
		return
	}
	history, err := s.repo.ListMessages(ctx, session.ID, s.maxHistory)
	if err != nil {
		fail(err, turn)
		return
	}
	toolDefs, err := s.tools.ListTools(ctx, 100)
	if err != nil {
		fail(err, turn)
		return
	}
	messages := buildAIMessages(session, memories, history, turnContext.WorkContext)
	tools := gatewayTools(toolDefs)
	providerID, channelID, providerType, model, serviceTier, effort := runModelConfig(session, RunInput{})
	if (providerID == nil && providerType == "") || model == "" {
		fail(fmt.Errorf("%w: model provider and model are required", ErrValidation), turn)
		return
	}
	s.continueAssistantTurns(ctx, send, fail, session, messages, turnContext, tools, providerID, channelID, providerType, model, serviceTier, effort, turn+1)
}

func (s *Service) continueAssistantTurns(
	ctx context.Context,
	send func(RunEvent) bool,
	fail func(error, int),
	session *Session,
	messages []aigateway.Message,
	turnContext assistantTurnContext,
	tools []aigateway.ToolDefinition,
	providerID *uuid.UUID,
	channelID *uuid.UUID,
	providerType string,
	model string,
	serviceTier string,
	effort string,
	startTurn int,
) {
	for turn := startTurn; turn <= s.maxTurns; turn++ {
		invocationMetadata := map[string]any{
			"assistant_session_id":   session.ID.String(),
			"module_key":             session.ModuleKey,
			"position_id":            uuidString(session.PositionID),
			"position_assignment_id": uuidString(session.PositionAssignmentID),
		}
		for key, value := range turnContext.Metadata {
			invocationMetadata[key] = value
		}
		output, err := s.ai.Invoke(ctx, aigateway.InvokeInput{
			ProviderID:         providerID,
			PreferredChannelID: channelID,
			ProviderType:       providerType,
			Model:              model,
			Messages:           messages,
			MaxTokens:          2048,
			ServiceTier:        serviceTier,
			ReasoningEffort:    effort,
			Tools:              tools,
			Attribution: aigateway.Attribution{
				OrganizationID: session.OrganizationID,
				DepartmentID:   session.DepartmentID,
				ProjectID:      session.ProjectID,
				WorkflowID:     session.WorkflowID,
				TaskID:         session.TaskID,
				UserID:         actorIDForAttribution(session),
				SourceSurface:  "assistant:" + session.ModuleKey,
			},
			Metadata: invocationMetadata,
		})
		if err != nil {
			fail(err, turn)
			return
		}

		assistantMessage := aigateway.Message{Role: "assistant", Content: output.Content, ToolCalls: output.ToolCalls}
		messages = append(messages, assistantMessage)
		stored, err := s.repo.AddMessage(ctx, session.ID, "assistant", output.Content, "", "", map[string]any{"tool_calls": output.ToolCalls})
		if err != nil {
			fail(err, turn)
			return
		}
		stepData := map[string]any{"tool_call_count": len(output.ToolCalls), "model": output.Model, "provider_type": output.ProviderType}
		for key, value := range turnContext.Metadata {
			stepData[key] = value
		}
		llmStep, err := s.repo.AddStep(ctx, session, AddStepInput{
			InvocationID: &output.InvocationID,
			StepType:     StepLLM,
			Status:       StatusCompleted,
			Summary:      trimSummary(output.Content),
			Data:         stepData,
			Turn:         turn,
		})
		if err != nil {
			fail(err, turn)
			return
		}
		if output.Content != "" && !send(RunEvent{Type: "message", Message: stored, Step: llmStep, Delta: output.Content}) {
			return
		}
		if len(output.ToolCalls) == 0 {
			s.finishRun(ctx, send, session, output.Content, llmStep)
			return
		}

		for index, call := range output.ToolCalls {
			callID := toolCallID(call, index)
			callData := map[string]any{"tool_call_id": callID, "tool_name": call.Name, "arguments": call.Arguments}
			for key, value := range turnContext.Metadata {
				callData[key] = value
			}
			callStep, err := s.repo.AddStep(ctx, session, AddStepInput{
				InvocationID: &output.InvocationID,
				StepType:     StepToolCall,
				Status:       "requested",
				Summary:      call.Name,
				Data:         callData,
				Turn:         turn,
			})
			if err != nil {
				fail(err, turn)
				return
			}
			if !send(RunEvent{Type: "tool_call", Step: callStep, Data: callStep.Data}) {
				return
			}
			runner := NewToolRunner(s.tools, ToolRunnerConfig{})
			result, err := runner.ExecuteTool(ctx, ToolRunRequest{
				Session:        session,
				ContextPackage: turnContext.Package,
				Call: ToolCallRequest{
					ID:        callID,
					Name:      call.Name,
					Arguments: call.Arguments,
				},
				InvocationID: &output.InvocationID,
			})
			if err != nil {
				fail(err, turn)
				return
			}
			status := result.Execution.Status
			if result.Approval != nil {
				approvalData := map[string]any{"tool_call_id": callID, "tool_name": call.Name, "approval_id": result.Approval.ID.String()}
				for key, value := range turnContext.Metadata {
					approvalData[key] = value
				}
				step, _ := s.repo.AddStep(ctx, session, AddStepInput{
					InvocationID:    &output.InvocationID,
					ToolExecutionID: &result.Execution.ID,
					ToolApprovalID:  &result.Approval.ID,
					StepType:        StepApproval,
					Status:          StatusApprovalRequired,
					Summary:         result.Execution.ResultSummary,
					Data:            approvalData,
					Turn:            turn,
				})
				_ = s.repo.UpdateSessionStatus(ctx, session.ID, StatusApprovalRequired, "")
				send(RunEvent{Type: "approval_required", Step: step, Data: step.Data})
				return
			}
			payload := map[string]any{"status": status, "summary": result.Execution.ResultSummary, "result": result.Execution.Result, "error": result.Execution.ErrorMessage}
			for key, value := range turnContext.Metadata {
				payload[key] = value
			}
			resultStep, err := s.repo.AddStep(ctx, session, AddStepInput{
				InvocationID:    &output.InvocationID,
				ToolExecutionID: &result.Execution.ID,
				StepType:        StepToolResult,
				Status:          status,
				Summary:         result.Execution.ResultSummary,
				Data:            payload,
				Turn:            turn,
			})
			if err != nil {
				fail(err, turn)
				return
			}
			if !send(RunEvent{Type: "tool_result", Step: resultStep, Data: payload}) {
				return
			}
			toolContent := marshalToolContent(payload)
			messages = append(messages, aigateway.Message{Role: "tool", Content: toolContent, ToolCallID: callID, ToolName: call.Name})
			if _, err := s.repo.AddMessage(ctx, session.ID, "tool", toolContent, callID, call.Name, payload); err != nil {
				fail(err, turn)
				return
			}
		}
	}
	fail(fmt.Errorf("%w: assistant reached max turns", ErrValidation), s.maxTurns)
}

func (s *Service) finishRun(ctx context.Context, send func(RunEvent) bool, session *Session, summary string, sourceStep *Step) {
	memory := copyMap(session.WorkingMemory)
	memory["last_summary"] = summary
	memory["last_module_key"] = session.ModuleKey
	memory["last_position_id"] = uuidString(session.PositionID)
	_ = s.repo.UpdateWorkingMemory(ctx, session.ID, memory)
	sourceStepID := sourceStep.ID
	created, err := s.repo.CreateMemory(ctx, CreateMemoryInput{
		Scope:           sessionScope(session),
		ActorID:         &session.ActorID,
		ActorType:       session.ActorType,
		MemoryType:      "lesson",
		Title:           "Assistant run summary",
		Content:         trimSummary(summary),
		Data:            map[string]any{"session_id": session.ID.String(), "mode": session.Mode},
		SourceSessionID: &session.ID,
		SourceStepID:    &sourceStepID,
		Confidence:      1,
	})
	if err == nil {
		step, _ := s.repo.AddStep(ctx, session, AddStepInput{
			StepType: StepMemory,
			Status:   StatusCompleted,
			Summary:  created.Title,
			Data:     map[string]any{"memory_id": created.ID.String(), "module_key": session.ModuleKey, "position_id": uuidString(session.PositionID)},
		})
		send(RunEvent{Type: "memory_updated", Step: step, Data: step.Data})
	}
	if session.TargetID != nil || session.TargetType != "" {
		proposal, err := s.repo.CreateProposal(ctx, CreateProposalInput{
			SessionID:    session.ID,
			ModuleKey:    session.ModuleKey,
			TargetType:   firstNonEmpty(session.TargetType, session.ModuleKey),
			TargetID:     session.TargetID,
			ProposalType: "metadata_patch",
			Title:        "Assistant suggested result",
			Summary:      trimSummary(summary),
			Payload: map[string]any{
				"summary":        summary,
				"module_key":     session.ModuleKey,
				"target_type":    session.TargetType,
				"target_id":      uuidString(session.TargetID),
				"source":         "assistant",
				"requires_human": true,
			},
			SourceStepID: &sourceStepID,
		})
		if err == nil {
			send(RunEvent{Type: "proposal_created", Data: map[string]any{"proposal_id": proposal.ID.String(), "status": proposal.Status}})
		}
	}
	_ = s.repo.UpdateSessionStatus(ctx, session.ID, StatusCompleted, "")
	send(RunEvent{Type: "done", Done: true, Data: map[string]any{"status": StatusCompleted}})
}

func buildAIMessages(session *Session, memories []Memory, history []Message, workContext WorkRecordContext) []aigateway.Message {
	messages := []aigateway.Message{{Role: "system", Content: systemPrompt(session, memories, workContext)}}
	for _, item := range history {
		msg := aigateway.Message{Role: item.Role, Content: item.Content, ToolCallID: item.ToolCallID, ToolName: item.ToolName}
		if item.Role == "assistant" {
			if calls, ok := item.Metadata["tool_calls"]; ok {
				data, _ := json.Marshal(calls)
				_ = json.Unmarshal(data, &msg.ToolCalls)
			}
		}
		messages = append(messages, msg)
	}
	return messages
}

func contextPackageMetadata(pkg *ContextPackage) map[string]any {
	if pkg == nil {
		return map[string]any{}
	}
	metadata := map[string]any{
		"context_package_id":       pkg.ID.String(),
		"attention_core_count":     len(pkg.AttentionCore),
		"supporting_context_count": len(pkg.SupportingContext),
		"risk_signal_count":        len(pkg.RiskAndSignals),
		"omission_count":           len(pkg.Omissions),
		"context_token_budget":     pkg.TokenBudget,
		"context_estimated_tokens": pkg.TotalEstimatedTokens(),
	}
	if pkg.DictionaryVersionID != nil {
		metadata["dictionary_version_id"] = pkg.DictionaryVersionID.String()
	}
	if source, ok := pkg.Provenance["source"]; ok {
		metadata["context_source"] = source
	}
	return metadata
}

func contextPackageToWorkRecordContext(moduleKey string, pkg *ContextPackage) WorkRecordContext {
	workContext := WorkRecordContext{ModuleKey: moduleKey}
	if pkg == nil {
		return workContext
	}
	records := make([]WorkRecord, 0, len(pkg.AttentionCore)+len(pkg.SupportingContext))
	items := append([]ContextItem{}, pkg.AttentionCore...)
	items = append(items, pkg.SupportingContext...)
	for _, item := range items {
		records = append(records, WorkRecord{
			ID:     item.RecordID,
			Type:   item.EntityKey,
			Title:  item.EntityKey + "." + item.FieldKey,
			Status: item.ValidationState,
			Data: map[string]any{
				"field_key":          item.FieldKey,
				"value":              item.Value,
				"source":             item.Source,
				"validation_state":   item.ValidationState,
				"context_package_id": pkg.ID.String(),
			},
		})
	}
	workContext.Records = records
	workContext.Metadata = map[string]any{
		"verified_context_package": true,
		"context_package_id":       pkg.ID.String(),
		"risk_signals":             len(pkg.RiskAndSignals),
		"omissions":                len(pkg.Omissions),
	}
	return workContext
}

func systemPrompt(session *Session, memories []Memory, workContext WorkRecordContext) string {
	var b strings.Builder
	b.WriteString("You are a business process and system self-evolution assistant for this organization.\n")
	b.WriteString("Authorized humans may operate the workspace directly or call you as the execution Agent from the current module.\n")
	b.WriteString("When a human delegates work to you, perform operational changes by Agent tool calls. Do not instruct the human to call APIs, edit JSON payloads, or execute delegated write operations manually.\n")
	b.WriteString("Humans provide intent, query records, and approve or reject decisions; you inspect context, call tools, summarize results, and stop for approval when required.\n")
	b.WriteString("Use tools for delegated create, update, delete, workflow, governance, finance, model-configuration, cost, organization, and project operations.\n")
	b.WriteString("For high-risk, financial, governance, external, model-configuration, or destructive actions, rely on the tool runtime approval policy and stop when approval_required is returned.\n")
	b.WriteString("Before using tools, inspect the scoped work records below and infer the safest current entity context. If the target entity remains ambiguous, ask a short review question instead of guessing.\n")
	b.WriteString("Memory isolation rule: only use memories and work records from the exact module_key and organization position scope shown here. Do not infer or import context from other modules or positions.\n")
	b.WriteString(fmt.Sprintf("mode=%s module_key=%s target_type=%s target_id=%s organization_id=%s department_id=%s position_id=%s position_assignment_id=%s\n",
		session.Mode, session.ModuleKey, session.TargetType, uuidString(session.TargetID), uuidString(session.OrganizationID), uuidString(session.DepartmentID),
		uuidString(session.PositionID), uuidString(session.PositionAssignmentID)))
	if len(memories) > 0 {
		b.WriteString("Scoped memories:\n")
		for _, memory := range memories {
			b.WriteString("- ")
			b.WriteString(memory.Title)
			if memory.Content != "" {
				b.WriteString(": ")
				b.WriteString(memory.Content)
			}
			b.WriteByte('\n')
		}
	}
	if workContext.Error != "" {
		b.WriteString("Work record context unavailable: ")
		b.WriteString(workContext.Error)
		b.WriteByte('\n')
	} else if workContext.Metadata != nil && workContext.Metadata["verified_context_package"] == true {
		b.WriteString("Verified context package: ")
		b.WriteString(fmt.Sprintf("id=%s risk_signals=%v omissions=%v\n",
			fmt.Sprint(workContext.Metadata["context_package_id"]),
			workContext.Metadata["risk_signals"],
			workContext.Metadata["omissions"],
		))
	}
	if workContext.Error == "" && len(workContext.Records) > 0 {
		b.WriteString("Recent work records from this module:\n")
		for _, record := range workContext.Records {
			b.WriteString("- ")
			b.WriteString(record.Type)
			b.WriteString(" ")
			b.WriteString(record.ID)
			b.WriteString(" status=")
			b.WriteString(record.Status)
			if record.Title != "" {
				b.WriteString(" title=")
				b.WriteString(record.Title)
			}
			if record.CreatedAt != "" {
				b.WriteString(" created_at=")
				b.WriteString(record.CreatedAt)
			}
			if len(record.Data) > 0 {
				if data, err := json.Marshal(record.Data); err == nil {
					b.WriteString(" data=")
					b.Write(data)
				}
			}
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func gatewayTools(tools []toolruntime.ToolDefinition) []aigateway.ToolDefinition {
	result := []aigateway.ToolDefinition{}
	for _, tool := range tools {
		if !tool.IsActive {
			continue
		}
		result = append(result, aigateway.ToolDefinition{Name: tool.Name, Description: tool.Description, Schema: tool.InputSchema})
	}
	return result
}

func runModelConfig(session *Session, input RunInput) (*uuid.UUID, *uuid.UUID, string, string, string, string) {
	providerID := session.ProviderID
	if input.ProviderID != nil {
		providerID = input.ProviderID
	}
	channelID := session.PreferredChannelID
	if input.PreferredChannelID != nil {
		channelID = input.PreferredChannelID
	}
	providerType := firstNonEmpty(input.ProviderType, session.ProviderType)
	model := firstNonEmpty(input.Model, session.Model)
	serviceTier := firstNonEmpty(input.ServiceTier, session.ServiceTier)
	effort := firstNonEmpty(input.ReasoningEffort, session.ReasoningEffort)
	return providerID, channelID, providerType, model, serviceTier, effort
}

func sessionScope(session *Session) Scope {
	return Scope{
		ModuleKey:            session.ModuleKey,
		OrganizationID:       session.OrganizationID,
		DepartmentID:         session.DepartmentID,
		PositionID:           session.PositionID,
		PositionAssignmentID: session.PositionAssignmentID,
		ProjectID:            session.ProjectID,
		WorkflowID:           session.WorkflowID,
		TaskID:               session.TaskID,
	}
}

func normalizedModule(value string) string {
	if strings.TrimSpace(value) == "" {
		return "general"
	}
	return strings.TrimSpace(value)
}

func normalizedOptionalModule(value string) string {
	return strings.TrimSpace(value)
}

func applySkillScopeDefaults(ctx context.Context, actorID uuid.UUID, input *CreateBusinessSkillInput) error {
	input.SkillKey = strings.TrimSpace(input.SkillKey)
	input.ScopeLevel = strings.TrimSpace(input.ScopeLevel)
	if input.ScopeLevel == "" {
		if currentTenantOrganizationID(ctx) != nil {
			input.ScopeLevel = SkillScopeOrganization
		} else {
			input.ScopeLevel = SkillScopeSaaSGlobal
		}
	}
	switch input.ScopeLevel {
	case SkillScopeSaaSGlobal, SkillScopeOrganization, SkillScopeDeployment:
	default:
		return fmt.Errorf("%w: unsupported skill scope_level", ErrValidation)
	}
	input.DeploymentMode = strings.TrimSpace(input.DeploymentMode)
	if input.DeploymentMode == "" {
		input.DeploymentMode = SkillDeploymentSaaS
	}
	switch input.DeploymentMode {
	case SkillDeploymentSaaS, SkillDeploymentOrgPrivate, SkillDeploymentPrivate:
	default:
		return fmt.Errorf("%w: unsupported skill deployment_mode", ErrValidation)
	}
	if input.ScopeLevel == SkillScopeOrganization {
		orgID := currentTenantOrganizationID(ctx)
		if input.OrganizationID != nil && orgID != nil && *input.OrganizationID != *orgID {
			return fmt.Errorf("%w: skill organization must match current organization", ErrForbidden)
		}
		if input.OrganizationID == nil {
			if orgID == nil {
				return fmt.Errorf("%w: organization skill requires organization context", ErrValidation)
			}
			id := *orgID
			input.OrganizationID = &id
		}
		if input.OwnerUserID == nil {
			id := actorID
			input.OwnerUserID = &id
		}
	}
	if input.ScopeLevel == SkillScopeSaaSGlobal {
		input.OrganizationID = nil
	}
	return nil
}

func validateSkillComponents(components []SkillComponent) error {
	if len(components) < 3 || len(components) > 9 {
		return fmt.Errorf("%w: skill_components must contain 3 to 9 components", ErrValidation)
	}
	for _, component := range components {
		if strings.TrimSpace(component.Key) == "" {
			return fmt.Errorf("%w: skill component key is required", ErrValidation)
		}
		if component.Weight <= 0 {
			return fmt.Errorf("%w: skill component weight must be positive", ErrValidation)
		}
	}
	return nil
}

func authorizeSkillActivation(ctx context.Context, skill *BusinessSkill) error {
	if skill == nil {
		return ErrNotFound
	}
	tenant, ok := middleware.TenantFromContext(ctx)
	switch firstNonEmpty(skill.ScopeLevel, SkillScopeSaaSGlobal) {
	case SkillScopeSaaSGlobal:
		if !ok || !tenant.IsPlatformAdmin {
			return fmt.Errorf("%w: platform admin is required to activate saas global skill", ErrForbidden)
		}
	case SkillScopeOrganization:
		if !tenantMatchesOrganization(tenant, ok, skill.OrganizationID) || !tenantCanAdminOrganization(tenant) {
			return fmt.Errorf("%w: organization admin is required to activate organization skill", ErrForbidden)
		}
	case SkillScopeDeployment:
		if ok && (tenant.IsPlatformAdmin || tenantCanAdminOrganization(tenant)) {
			return nil
		}
		return fmt.Errorf("%w: deployment skill activation requires admin authority", ErrForbidden)
	default:
		return fmt.Errorf("%w: unsupported skill scope_level", ErrValidation)
	}
	return nil
}

func authorizeSkillUse(ctx context.Context, skill *BusinessSkill) error {
	if skill == nil {
		return ErrNotFound
	}
	tenant, ok := middleware.TenantFromContext(ctx)
	if ok && tenant.EnabledModules != nil && skill.ModuleKey != "" && skill.ModuleKey != "general" && !tenant.EnabledModules[skill.ModuleKey] {
		return fmt.Errorf("%w: skill module is disabled", ErrForbidden)
	}
	switch firstNonEmpty(skill.ScopeLevel, SkillScopeSaaSGlobal) {
	case SkillScopeSaaSGlobal:
		return nil
	case SkillScopeOrganization:
		if !tenantMatchesOrganization(tenant, ok, skill.OrganizationID) {
			return fmt.Errorf("%w: skill organization does not match current organization", ErrForbidden)
		}
	case SkillScopeDeployment:
		if skill.OrganizationID != nil && !tenantMatchesOrganization(tenant, ok, skill.OrganizationID) {
			return fmt.Errorf("%w: deployment skill organization does not match current organization", ErrForbidden)
		}
	default:
		return fmt.Errorf("%w: unsupported skill scope_level", ErrValidation)
	}
	return nil
}

func (s *Service) authorizeSkillWithKernel(ctx context.Context, actorID uuid.UUID, actorType string, action string, skill *BusinessSkill) error {
	if s.securityKernel == nil || skill == nil {
		return nil
	}
	tenant, _ := middleware.TenantFromContext(ctx)
	request := securitykernel.Request{
		Actor: securitykernel.Actor{
			ActorID:         actorID,
			ActorType:       actorType,
			AuthorityTier:   tenantAuthorityTier(tenant),
			IsPlatformAdmin: tenant != nil && tenant.IsPlatformAdmin,
		},
		OrganizationID:  tenantOrganizationID(tenant),
		EnabledModules:  tenantEnabledModules(tenant),
		EnabledFeatures: []string{"skill_governance"},
		Resource: securitykernel.Resource{
			ModuleKey:             firstNonEmpty(skill.ModuleKey, "assistant"),
			ResourceType:          "skill",
			Action:                action,
			ScopeLevel:            firstNonEmpty(skill.ScopeLevel, SkillScopeSaaSGlobal),
			OrganizationID:        skill.OrganizationID,
			RequiredAuthorityTier: requiredSkillAuthority(action),
			RequiredLicenseMode:   "commercial",
		},
		Metadata: map[string]any{
			"target_type":           skill.TargetType,
			"business_function_key": skill.BusinessFunctionKey,
			"deployment_mode":       skill.DeploymentMode,
		},
	}
	decision, err := s.securityKernel.Authorize(ctx, request)
	if err != nil || !decision.Allowed {
		reason := decision.Reason
		if reason == "" && err != nil {
			reason = err.Error()
		}
		return fmt.Errorf("%w: security kernel denied skill %s: %s", ErrForbidden, action, reason)
	}
	return nil
}

func requiredSkillAuthority(action string) string {
	switch action {
	case "create", "activate":
		return "organization_admin"
	default:
		return "executor"
	}
}

func tenantAuthorityTier(tenant *middleware.TenantContext) string {
	if tenant == nil {
		return ""
	}
	return tenant.AuthorityTier
}

func tenantOrganizationID(tenant *middleware.TenantContext) *uuid.UUID {
	if tenant == nil {
		return nil
	}
	return tenant.OrganizationID
}

func tenantEnabledModules(tenant *middleware.TenantContext) []string {
	if tenant == nil || tenant.EnabledModules == nil {
		return []string{}
	}
	modules := []string{}
	for moduleKey, enabled := range tenant.EnabledModules {
		if enabled {
			modules = append(modules, moduleKey)
		}
	}
	return modules
}

func tenantMatchesOrganization(tenant *middleware.TenantContext, ok bool, orgID *uuid.UUID) bool {
	if orgID == nil {
		return ok && tenant != nil && tenant.IsPlatformAdmin
	}
	return ok && tenant != nil && tenant.OrganizationID != nil && *tenant.OrganizationID == *orgID
}

func tenantCanAdminOrganization(tenant *middleware.TenantContext) bool {
	if tenant == nil {
		return false
	}
	switch tenant.AuthorityTier {
	case "organization_creator", "organization_admin":
		return true
	default:
		return tenant.IsPlatformAdmin
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func trimSummary(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 600 {
		return value[:600]
	}
	return value
}

func marshalToolContent(input map[string]any) string {
	data, err := json.Marshal(input)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func toolCallID(call aigateway.ToolCall, index int) string {
	if call.ID != "" {
		return call.ID
	}
	return fmt.Sprintf("tool_call_%d", index+1)
}

func approvalStepContext(steps []Step, approvalID uuid.UUID, executionID uuid.UUID) (string, string, int) {
	for i := len(steps) - 1; i >= 0; i-- {
		step := steps[i]
		if (step.ToolApprovalID != nil && *step.ToolApprovalID == approvalID) ||
			(step.ToolExecutionID != nil && *step.ToolExecutionID == executionID) {
			callID := stringFromMap(step.Data, "tool_call_id")
			if callID == "" {
				callID = approvalID.String()
			}
			toolName := stringFromMap(step.Data, "tool_name")
			if toolName == "" {
				toolName = executionID.String()
			}
			return callID, toolName, step.Turn
		}
	}
	return approvalID.String(), executionID.String(), 1
}

func actorIDForAttribution(session *Session) *uuid.UUID {
	if session.ActorType == "human" || session.ActorType == "internal_human" || session.ActorType == "external_human" {
		return &session.ActorID
	}
	return nil
}

func uuidString(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

func stringFromInput(input map[string]any, key string) string {
	value, ok := input[key]
	if !ok {
		return ""
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
}

func stringFromMap(input map[string]any, key string) string {
	value, ok := input[key]
	if !ok {
		return ""
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
}

func uuidFromInput(input map[string]any, key string) *uuid.UUID {
	raw := stringFromInput(input, key)
	if raw == "" {
		return nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil
	}
	return &id
}

func isHumanActor(actorType string) bool {
	switch strings.TrimSpace(actorType) {
	case "human", "internal", "internal_human", "external_human":
		return true
	default:
		return false
	}
}

func isNotFound(err error) bool {
	return errors.Is(err, ErrNotFound) || errors.Is(err, pgx.ErrNoRows)
}

func applyTenantSessionScope(ctx context.Context, input *CreateSessionInput) error {
	orgID := currentTenantOrganizationID(ctx)
	if orgID == nil {
		return nil
	}
	if input.OrganizationID != nil && *input.OrganizationID != *orgID {
		return fmt.Errorf("%w: session organization must match current organization", ErrForbidden)
	}
	id := *orgID
	input.OrganizationID = &id
	return nil
}

func currentTenantOrganizationID(ctx context.Context) *uuid.UUID {
	tenant, ok := middleware.TenantFromContext(ctx)
	if !ok || tenant.OrganizationID == nil {
		return nil
	}
	id := *tenant.OrganizationID
	return &id
}

func currentTenantIsPlatformAdmin(ctx context.Context) bool {
	tenant, ok := middleware.TenantFromContext(ctx)
	return ok && tenant.IsPlatformAdmin
}

func copyMap(input map[string]any) map[string]any {
	output := map[string]any{}
	for key, value := range input {
		output[key] = value
	}
	return output
}
