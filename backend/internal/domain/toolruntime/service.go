package toolruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/governance"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/observability"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/securitykernel"
)

var (
	ErrValidation = errors.New("validation error")
	ErrNotFound   = errors.New("not found")
	ErrForbidden  = errors.New("forbidden")
)

type ToolAdapter func(context.Context, ExecuteToolInput) (ToolResult, error)

type Repository interface {
	CreateTool(ctx context.Context, input CreateToolInput) (*ToolDefinition, error)
	ListTools(ctx context.Context, limit int) ([]ToolDefinition, error)
	UpdateTool(ctx context.Context, id uuid.UUID, input UpdateToolInput) (*ToolDefinition, error)
	GetToolByID(ctx context.Context, id uuid.UUID) (*ToolDefinition, error)
	GetToolByName(ctx context.Context, name string) (*ToolDefinition, error)
	CreateInterfaceFile(ctx context.Context, input CreateInterfaceFileInput, createdBy *uuid.UUID) (*InterfaceFile, error)
	ListInterfaceFiles(ctx context.Context, limit int) ([]InterfaceFile, error)
	GetInterfaceFile(ctx context.Context, id uuid.UUID) (*InterfaceFile, error)
	UpdateInterfaceFile(ctx context.Context, id uuid.UUID, input UpdateInterfaceFileInput) (*InterfaceFile, error)
	CreateExecution(ctx context.Context, input CreateExecutionInput) (*ToolExecution, error)
	CompleteExecution(ctx context.Context, id uuid.UUID, input CompleteExecutionInput) (*ToolExecution, error)
	CreateApproval(ctx context.Context, executionID uuid.UUID, requestedBy *uuid.UUID, reason string) (*ToolApproval, error)
	ListExecutions(ctx context.Context, limit int) ([]ToolExecution, error)
	GetExecution(ctx context.Context, id uuid.UUID) (*ToolExecution, error)
	GetApproval(ctx context.Context, id uuid.UUID) (*ToolApproval, error)
	GetHumanAuthorityTier(ctx context.Context, userID uuid.UUID, organizationID *uuid.UUID) (string, error)
	UpdateApproval(ctx context.Context, id uuid.UUID, status string, reviewedBy *uuid.UUID, reason string) (*ToolApproval, error)
}

type GovernanceService interface {
	DecideAccess(context.Context, governance.AccessDecisionInput) (*governance.AccessDecision, error)
}

type ObservabilityRecorder interface {
	StartTrace(ctx context.Context, workflowID *uuid.UUID, metadata map[string]any) (*observability.Trace, error)
	RecordSpan(ctx context.Context, input observability.RecordSpanInput) (*observability.Span, error)
	RecordMetric(ctx context.Context, input observability.RecordMetricInput) (*observability.Metric, error)
	CompleteTrace(ctx context.Context, id uuid.UUID, status string) error
}

type Service struct {
	repo           Repository
	governance     GovernanceService
	adapters       map[string]ToolAdapter
	observability  ObservabilityRecorder
	securityKernel securitykernel.Client
}

type ServiceOption func(*Service)

func WithObservability(recorder ObservabilityRecorder) ServiceOption {
	return func(s *Service) {
		s.observability = recorder
	}
}

func WithSecurityKernel(client securitykernel.Client) ServiceOption {
	return func(s *Service) {
		s.securityKernel = client
	}
}

func NewService(repo Repository, governanceSvc GovernanceService, adapters map[string]ToolAdapter, opts ...ServiceOption) *Service {
	if adapters == nil {
		adapters = map[string]ToolAdapter{}
	}
	s := &Service{repo: repo, governance: governanceSvc, adapters: adapters}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Service) RegisterAdapters(adapters map[string]ToolAdapter) {
	if s.adapters == nil {
		s.adapters = map[string]ToolAdapter{}
	}
	for name, adapter := range adapters {
		if name == "" || adapter == nil {
			continue
		}
		s.adapters[name] = adapter
	}
}

type CreateExecutionInput struct {
	ToolID             uuid.UUID
	InvocationID       *uuid.UUID
	ActorID            uuid.UUID
	ActorType          string
	OrganizationID     *uuid.UUID
	DepartmentID       *uuid.UUID
	ProjectID          *uuid.UUID
	WorkflowID         *uuid.UUID
	TaskID             *uuid.UUID
	IdempotencyKey     string
	Policy             string
	GovernanceDecision string
	RequestedByHumanID *uuid.UUID
	Status             string
	Arguments          map[string]any
}

type CompleteExecutionInput struct {
	Status        string
	ResultSummary string
	Result        map[string]any
	ErrorMessage  string
	DurationMS    int
}

func EffectivePolicy(defaultPolicy string, governance GovernanceResult) string {
	if governance.Decision == "deny" {
		return PolicyDeny
	}
	if governance.Decision == "approve" {
		return PolicyApprove
	}
	if governance.Decision == "notify" && defaultPolicy == PolicyAuto {
		return PolicyNotify
	}
	return defaultPolicy
}

func EffectivePolicyForExecution(tool ToolDefinition, input ExecuteToolInput, governance GovernanceResult) string {
	policy := EffectivePolicy(tool.DefaultPolicy, governance)
	if policy == PolicyDeny || policy == PolicyApprove {
		return policy
	}
	switch tool.Name {
	case "erp.action.execute", "context.proposal.apply", "finance.prepare_export_batch":
		return PolicyApprove
	case "industry.solution.change.preview":
		if policy == PolicyAuto || policy == "" {
			return PolicyNotify
		}
		return policy
	case "runtime.operation.execute":
		if runtimeOperationRequiresApproval(input.Arguments) {
			return PolicyApprove
		}
		if policy == PolicyAuto || policy == "" {
			return PolicyNotify
		}
		return policy
	default:
		return policy
	}
}

func runtimeOperationRequiresApproval(arguments map[string]any) bool {
	method := strings.ToUpper(firstArgumentString(arguments, "method", "http_method", "operation_method", "request_method"))
	switch method {
	case "GET", "HEAD", "OPTIONS":
		return false
	case "POST", "PUT", "PATCH", "DELETE":
		return true
	}
	kind := strings.ToLower(firstArgumentString(arguments, "operation_kind", "kind", "mode"))
	switch kind {
	case "read", "query", "preview", "describe", "list":
		return false
	case "write", "mutation", "command", "delete", "apply", "execute":
		return true
	}
	action := strings.ToLower(firstArgumentString(arguments, "action", "operation", "operation_code", "name"))
	if action == "" {
		return true
	}
	for _, prefix := range []string{"get", "list", "read", "preview", "describe", "search"} {
		if strings.HasPrefix(action, prefix) {
			return false
		}
	}
	for _, prefix := range []string{"create", "update", "delete", "apply", "approve", "reject", "post", "put", "patch", "run", "execute"} {
		if strings.HasPrefix(action, prefix) {
			return true
		}
	}
	return true
}

func firstArgumentString(arguments map[string]any, keys ...string) string {
	if arguments == nil {
		return ""
	}
	for _, key := range keys {
		value, ok := arguments[key]
		if !ok {
			continue
		}
		switch v := value.(type) {
		case string:
			if text := strings.TrimSpace(v); text != "" {
				return text
			}
		case fmt.Stringer:
			if text := strings.TrimSpace(v.String()); text != "" {
				return text
			}
		}
	}
	return ""
}

func (s *Service) CreateTool(ctx context.Context, input CreateToolInput) (*ToolDefinition, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrValidation)
	}
	input.ToolCategory = normalizeToolCategory(input.ToolCategory)
	input.ApprovalTierRequired = normalizeApprovalTier(input.ApprovalTierRequired, approvalTierForCategory(input.ToolCategory))
	return s.repo.CreateTool(ctx, input)
}

func (s *Service) ListTools(ctx context.Context, limit int) ([]ToolDefinition, error) {
	return s.repo.ListTools(ctx, limit)
}

func (s *Service) UpdateTool(ctx context.Context, id uuid.UUID, input UpdateToolInput) (*ToolDefinition, error) {
	if input.ToolCategory != nil {
		category := normalizeToolCategory(*input.ToolCategory)
		input.ToolCategory = &category
	}
	if input.ApprovalTierRequired != nil {
		tier := normalizeApprovalTier(*input.ApprovalTierRequired, "")
		if tier == "" {
			return nil, fmt.Errorf("%w: invalid approval tier", ErrValidation)
		}
		input.ApprovalTierRequired = &tier
	}
	return s.repo.UpdateTool(ctx, id, input)
}

func (s *Service) CreateInterfaceFile(ctx context.Context, input CreateInterfaceFileInput, createdBy *uuid.UUID) (*InterfaceFile, error) {
	normalized, err := normalizeCreateInterfaceFileInput(input)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateInterfaceFile(ctx, normalized, createdBy)
}

func (s *Service) ListInterfaceFiles(ctx context.Context, limit int) ([]InterfaceFile, error) {
	return s.repo.ListInterfaceFiles(ctx, limit)
}

func (s *Service) GetInterfaceFile(ctx context.Context, id uuid.UUID) (*InterfaceFile, error) {
	return s.repo.GetInterfaceFile(ctx, id)
}

func (s *Service) UpdateInterfaceFile(ctx context.Context, id uuid.UUID, input UpdateInterfaceFileInput) (*InterfaceFile, error) {
	if (input.Content != nil && input.FileType == nil) || (input.Content == nil && input.FileType != nil) {
		current, err := s.repo.GetInterfaceFile(ctx, id)
		if err != nil {
			return nil, err
		}
		if input.Content != nil && input.FileType == nil {
			fileType := current.FileType
			input.FileType = &fileType
		}
		if input.Content == nil && input.FileType != nil {
			content := current.Content
			input.Content = &content
		}
	}
	normalized, err := normalizeUpdateInterfaceFileInput(input)
	if err != nil {
		return nil, err
	}
	return s.repo.UpdateInterfaceFile(ctx, id, normalized)
}

func (s *Service) ExecuteTool(ctx context.Context, input ExecuteToolInput) (*ExecuteToolOutput, error) {
	if input.ToolName == "" || input.ActorID == uuid.Nil || input.ActorType == "" {
		return nil, fmt.Errorf("%w: tool_name, actor_id, and actor_type are required", ErrValidation)
	}
	if input.Arguments == nil {
		input.Arguments = map[string]any{}
	}
	if err := applyTenantExecutionScope(ctx, &input); err != nil {
		return nil, err
	}
	tool, err := s.repo.GetToolByName(ctx, input.ToolName)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeToolWithKernel(ctx, tool, input); err != nil {
		return nil, err
	}
	governanceResult, err := s.decide(ctx, tool, input)
	if err != nil {
		return nil, err
	}
	policy := EffectivePolicyForExecution(*tool, input, governanceResult)
	requestedByHumanID := requestedByHuman(input)
	status := ExecutionRequested
	switch policy {
	case PolicyApprove:
		status = ExecutionApprovalRequired
	case PolicyDeny:
		status = ExecutionDenied
	case PolicyAuto, PolicyNotify:
		status = ExecutionRunning
	}
	execution, err := s.repo.CreateExecution(ctx, CreateExecutionInput{
		ToolID:             tool.ID,
		InvocationID:       input.InvocationID,
		ActorID:            input.ActorID,
		ActorType:          input.ActorType,
		OrganizationID:     input.OrganizationID,
		DepartmentID:       input.DepartmentID,
		ProjectID:          input.ProjectID,
		WorkflowID:         input.WorkflowID,
		TaskID:             input.TaskID,
		IdempotencyKey:     input.IdempotencyKey,
		Policy:             policy,
		GovernanceDecision: governanceResult.Decision,
		RequestedByHumanID: requestedByHumanID,
		Status:             status,
		Arguments:          input.Arguments,
	})
	if err != nil {
		return nil, err
	}
	trace := s.startToolTrace(ctx, tool, execution, input, policy, governanceResult)
	switch policy {
	case PolicyDeny:
		completed, err := s.repo.CompleteExecution(ctx, execution.ID, CompleteExecutionInput{
			Status:       ExecutionDenied,
			ErrorMessage: governanceResult.Reason,
		})
		if err != nil {
			return nil, err
		}
		s.recordToolMetric(ctx, "tool_governance_denied", execution.ID, 1, map[string]any{"tool_name": tool.Name, "risk_level": tool.RiskLevel})
		s.recordToolSpan(ctx, trace, tool, completed, input, ExecutionDenied, governanceResult.Reason, 0)
		s.completeObservationTrace(ctx, trace, observability.TraceFailed)
		return &ExecuteToolOutput{Execution: completed}, nil
	case PolicyApprove:
		approval, err := s.repo.CreateApproval(ctx, execution.ID, &input.ActorID, governanceResult.Reason)
		if err != nil {
			s.completeObservationTrace(ctx, trace, observability.TraceFailed)
			return nil, err
		}
		s.recordToolMetric(ctx, "tool_approval_required", execution.ID, 1, map[string]any{"tool_name": tool.Name, "risk_level": tool.RiskLevel})
		s.recordToolSpan(ctx, trace, tool, execution, input, ExecutionApprovalRequired, governanceResult.Reason, 0)
		s.completeObservationTrace(ctx, trace, observability.TraceComplete)
		return &ExecuteToolOutput{Execution: execution, Approval: approval}, nil
	default:
		return s.runAdapter(ctx, tool, execution, input, trace)
	}
}

func (s *Service) TestTool(ctx context.Context, id uuid.UUID, input ExecuteToolInput) (*ExecuteToolOutput, error) {
	tool, err := s.repo.GetToolByID(ctx, id)
	if err != nil {
		return nil, err
	}
	input.ToolName = tool.Name
	return s.ExecuteTool(ctx, input)
}

func (s *Service) ListExecutions(ctx context.Context, limit int) ([]ToolExecution, error) {
	return s.repo.ListExecutions(ctx, limit)
}

func (s *Service) GetExecution(ctx context.Context, id uuid.UUID) (*ToolExecution, error) {
	return s.repo.GetExecution(ctx, id)
}

func (s *Service) GetApproval(ctx context.Context, id uuid.UUID) (*ToolApproval, error) {
	return s.repo.GetApproval(ctx, id)
}

func (s *Service) Approve(ctx context.Context, id uuid.UUID, reviewedBy *uuid.UUID, reason string) (*ApprovalReviewOutput, error) {
	approval, execution, tool, err := s.authorizeApprovalReview(ctx, id, reviewedBy)
	if err != nil {
		return nil, err
	}
	if approval.Status == ApprovalApproved {
		return s.approvedOutput(ctx, approval, execution, tool)
	}
	if approval.ExpiresAt != nil && time.Now().After(*approval.ExpiresAt) {
		expired, err := s.repo.UpdateApproval(ctx, id, ApprovalExpired, nil, firstNonEmpty(reason, "approval expired"))
		if err != nil {
			return nil, err
		}
		expiredExecution, err := s.repo.GetExecution(ctx, expired.ExecutionID)
		if err != nil {
			return nil, err
		}
		return &ApprovalReviewOutput{Approval: expired, Execution: expiredExecution}, fmt.Errorf("%w: approval expired", ErrValidation)
	}
	approved, err := s.repo.UpdateApproval(ctx, id, ApprovalApproved, reviewedBy, reason)
	if err != nil {
		return nil, err
	}
	execution, err = s.repo.GetExecution(ctx, approved.ExecutionID)
	if err != nil {
		return nil, err
	}
	return s.approvedOutput(ctx, approved, execution, tool)
}

func (s *Service) Reject(ctx context.Context, id uuid.UUID, reviewedBy *uuid.UUID, reason string) (*ApprovalReviewOutput, error) {
	_, _, _, err := s.authorizeApprovalReview(ctx, id, reviewedBy)
	if err != nil {
		return nil, err
	}
	approval, err := s.repo.UpdateApproval(ctx, id, ApprovalRejected, reviewedBy, reason)
	if err != nil {
		return nil, err
	}
	execution, err := s.repo.GetExecution(ctx, approval.ExecutionID)
	if err != nil {
		return nil, err
	}
	return &ApprovalReviewOutput{Approval: approval, Execution: execution}, nil
}

func (s *Service) approvedOutput(ctx context.Context, approval *ToolApproval, execution *ToolExecution, tool *ToolDefinition) (*ApprovalReviewOutput, error) {
	if execution.Status == ExecutionCompleted || execution.Status == ExecutionFailed || execution.Status == ExecutionDenied || execution.Status == ExecutionRejected {
		return &ApprovalReviewOutput{Approval: approval, Execution: execution}, nil
	}
	input := ExecuteToolInput{
		ToolName:       tool.Name,
		InvocationID:   execution.InvocationID,
		ActorID:        execution.ActorID,
		ActorType:      execution.ActorType,
		OrganizationID: execution.OrganizationID,
		DepartmentID:   execution.DepartmentID,
		ProjectID:      execution.ProjectID,
		WorkflowID:     execution.WorkflowID,
		TaskID:         execution.TaskID,
		IdempotencyKey: execution.IdempotencyKey,
		Arguments:      execution.Arguments,
	}
	trace := s.startToolTrace(ctx, tool, execution, input, execution.Policy, GovernanceResult{Decision: execution.GovernanceDecision, Allowed: true})
	result, err := s.runAdapter(ctx, tool, execution, input, trace)
	if result != nil && result.Execution != nil {
		return &ApprovalReviewOutput{Approval: approval, Execution: result.Execution}, err
	}
	if err != nil {
		return nil, err
	}
	return &ApprovalReviewOutput{Approval: approval, Execution: execution}, nil
}

func (s *Service) authorizeApprovalReview(ctx context.Context, approvalID uuid.UUID, reviewedBy *uuid.UUID) (*ToolApproval, *ToolExecution, *ToolDefinition, error) {
	if reviewedBy == nil || *reviewedBy == uuid.Nil {
		return nil, nil, nil, fmt.Errorf("%w: reviewed_by human is required", ErrValidation)
	}
	approval, err := s.repo.GetApproval(ctx, approvalID)
	if err != nil {
		return nil, nil, nil, err
	}
	if approval.Status != ApprovalPending && approval.Status != ApprovalApproved {
		return nil, nil, nil, fmt.Errorf("%w: approval is not pending", ErrValidation)
	}
	execution, err := s.repo.GetExecution(ctx, approval.ExecutionID)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := ensureTenantExecutionAccess(ctx, execution.OrganizationID); err != nil {
		return nil, nil, nil, err
	}
	tool, err := s.repo.GetToolByID(ctx, execution.ToolID)
	if err != nil {
		return nil, nil, nil, err
	}
	requiredTier := normalizeApprovalTier(tool.ApprovalTierRequired, approvalTierForCategory(tool.ToolCategory))
	actualTier, err := s.repo.GetHumanAuthorityTier(ctx, *reviewedBy, execution.OrganizationID)
	if err != nil {
		return nil, nil, nil, err
	}
	if !authorityTierAllows(actualTier, requiredTier) {
		return nil, nil, nil, fmt.Errorf("%w: %s approval requires %s authority", ErrForbidden, normalizeToolCategory(tool.ToolCategory), requiredTier)
	}
	if err := s.authorizeApprovalWithKernel(ctx, tool, execution, *reviewedBy, actualTier); err != nil {
		return nil, nil, nil, err
	}
	return approval, execution, tool, nil
}

func (s *Service) runAdapter(ctx context.Context, tool *ToolDefinition, execution *ToolExecution, input ExecuteToolInput, trace *observability.Trace) (*ExecuteToolOutput, error) {
	adapter, ok := s.adapters[tool.Name]
	if !ok {
		completed, err := s.repo.CompleteExecution(ctx, execution.ID, CompleteExecutionInput{
			Status:       ExecutionFailed,
			ErrorMessage: "tool adapter is not configured",
		})
		if err != nil {
			s.completeObservationTrace(ctx, trace, observability.TraceFailed)
			return nil, err
		}
		s.recordToolSpan(ctx, trace, tool, completed, input, ExecutionFailed, "tool adapter is not configured", 0)
		s.completeObservationTrace(ctx, trace, observability.TraceFailed)
		return &ExecuteToolOutput{Execution: completed}, fmt.Errorf("%w: tool adapter is not configured", ErrNotFound)
	}
	started := time.Now()
	result, err := adapter(ctx, input)
	if err != nil {
		completed, updateErr := s.repo.CompleteExecution(ctx, execution.ID, CompleteExecutionInput{
			Status:       ExecutionFailed,
			ErrorMessage: err.Error(),
			DurationMS:   int(time.Since(started).Milliseconds()),
		})
		if updateErr != nil {
			return nil, updateErr
		}
		durationMS := int(time.Since(started).Milliseconds())
		s.recordToolSpan(ctx, trace, tool, completed, input, ExecutionFailed, err.Error(), durationMS)
		s.completeObservationTrace(ctx, trace, observability.TraceFailed)
		return &ExecuteToolOutput{Execution: completed}, err
	}
	durationMS := int(time.Since(started).Milliseconds())
	completed, err := s.repo.CompleteExecution(ctx, execution.ID, CompleteExecutionInput{
		Status:        ExecutionCompleted,
		ResultSummary: result.Summary,
		Result:        result.Data,
		DurationMS:    durationMS,
	})
	if err != nil {
		s.completeObservationTrace(ctx, trace, observability.TraceFailed)
		return nil, err
	}
	s.recordToolSpan(ctx, trace, tool, completed, input, ExecutionCompleted, "", durationMS)
	s.completeObservationTrace(ctx, trace, observability.TraceComplete)
	return &ExecuteToolOutput{Execution: completed}, nil
}

func (s *Service) decide(ctx context.Context, tool *ToolDefinition, input ExecuteToolInput) (GovernanceResult, error) {
	if s.governance == nil {
		return GovernanceResult{Decision: "notify", Allowed: true, Reason: "governance service not configured"}, nil
	}
	decision, err := s.governance.DecideAccess(ctx, governance.AccessDecisionInput{
		ActorID:        input.ActorID,
		ActorType:      input.ActorType,
		Action:         tool.Name,
		Resource:       "tool",
		ResourceID:     &tool.ID,
		OrganizationID: input.OrganizationID,
		DepartmentID:   input.DepartmentID,
		WorkflowID:     input.WorkflowID,
		TaskID:         input.TaskID,
		RequiredLevel:  tool.RequiredLevel,
		RiskLevel:      tool.RiskLevel,
		Context: map[string]any{
			"tool_source_type": tool.SourceType,
			"project_id":       input.ProjectID,
			"idempotency_key":  input.IdempotencyKey,
		},
	})
	if err != nil {
		return GovernanceResult{}, err
	}
	return GovernanceResult{Decision: decision.Decision, Allowed: decision.Allowed, Reason: decision.Reason}, nil
}

func (s *Service) authorizeToolWithKernel(ctx context.Context, tool *ToolDefinition, input ExecuteToolInput) error {
	if s.securityKernel == nil {
		return nil
	}
	tenant, _ := middleware.TenantFromContext(ctx)
	request := securitykernel.Request{
		Actor: securitykernel.Actor{
			ActorID:         input.ActorID,
			ActorType:       input.ActorType,
			AuthorityTier:   tenantAuthorityTier(tenant),
			IsPlatformAdmin: tenant != nil && tenant.IsPlatformAdmin,
		},
		OrganizationID:  input.OrganizationID,
		EnabledModules:  tenantEnabledModules(tenant),
		EnabledFeatures: []string{"tool_execution"},
		Resource: securitykernel.Resource{
			ModuleKey:             "toolruntime",
			ResourceType:          "tool",
			Action:                "execute",
			ScopeLevel:            "organization",
			OrganizationID:        input.OrganizationID,
			RequiredAuthorityTier: normalizeApprovalTier(tool.ApprovalTierRequired, approvalTierForCategory(tool.ToolCategory)),
			RequiredLicenseMode:   "commercial",
		},
		Metadata: map[string]any{
			"tool_name":      tool.Name,
			"risk_level":     tool.RiskLevel,
			"required_level": tool.RequiredLevel,
		},
	}
	decision, err := s.securityKernel.Authorize(ctx, request)
	if err != nil || !decision.Allowed {
		reason := decision.Reason
		if reason == "" && err != nil {
			reason = err.Error()
		}
		return fmt.Errorf("%w: security kernel denied tool execution: %s", ErrForbidden, reason)
	}
	return nil
}

func (s *Service) authorizeApprovalWithKernel(ctx context.Context, tool *ToolDefinition, execution *ToolExecution, reviewerID uuid.UUID, reviewerTier string) error {
	if s.securityKernel == nil {
		return nil
	}
	tenant, _ := middleware.TenantFromContext(ctx)
	request := securitykernel.Request{
		Actor: securitykernel.Actor{
			ActorID:         reviewerID,
			ActorType:       "human",
			AuthorityTier:   reviewerTier,
			IsPlatformAdmin: tenant != nil && tenant.IsPlatformAdmin,
		},
		OrganizationID:  execution.OrganizationID,
		EnabledModules:  tenantEnabledModules(tenant),
		EnabledFeatures: []string{"tool_approval"},
		Resource: securitykernel.Resource{
			ModuleKey:             "toolruntime",
			ResourceType:          "tool_approval",
			Action:                "review",
			ScopeLevel:            "organization",
			OrganizationID:        execution.OrganizationID,
			RequiredAuthorityTier: normalizeApprovalTier(tool.ApprovalTierRequired, approvalTierForCategory(tool.ToolCategory)),
			RequiredLicenseMode:   "commercial",
		},
		Metadata: map[string]any{
			"tool_name":    tool.Name,
			"execution_id": execution.ID.String(),
		},
	}
	decision, err := s.securityKernel.Authorize(ctx, request)
	if err != nil || !decision.Allowed {
		reason := decision.Reason
		if reason == "" && err != nil {
			reason = err.Error()
		}
		return fmt.Errorf("%w: security kernel denied tool approval: %s", ErrForbidden, reason)
	}
	return nil
}

func (s *Service) startToolTrace(ctx context.Context, tool *ToolDefinition, execution *ToolExecution, input ExecuteToolInput, policy string, governanceResult GovernanceResult) *observability.Trace {
	if s.observability == nil {
		return nil
	}
	trace, err := s.observability.StartTrace(ctx, input.WorkflowID, map[string]any{
		"category":            "tool_execution",
		"tool_id":             tool.ID.String(),
		"tool_name":           tool.Name,
		"execution_id":        execution.ID.String(),
		"policy":              policy,
		"governance_decision": governanceResult.Decision,
		"risk_level":          tool.RiskLevel,
		"organization_id":     optionalUUIDString(input.OrganizationID),
		"department_id":       optionalUUIDString(input.DepartmentID),
		"project_id":          optionalUUIDString(input.ProjectID),
		"task_id":             optionalUUIDString(input.TaskID),
	})
	if err != nil {
		return nil
	}
	return trace
}

func (s *Service) recordToolSpan(ctx context.Context, trace *observability.Trace, tool *ToolDefinition, execution *ToolExecution, input ExecuteToolInput, status string, message string, durationMS int) {
	if s.observability == nil || trace == nil {
		return
	}
	_, _ = s.observability.RecordSpan(ctx, observability.RecordSpanInput{
		TraceID:    trace.ID,
		SpanType:   observability.SpanToolExecution,
		EntityID:   &execution.ID,
		EntityType: "tool_execution",
		ActorID:    &input.ActorID,
		ActorType:  input.ActorType,
		Input: map[string]any{
			"tool_id":         tool.ID.String(),
			"tool_name":       tool.Name,
			"arguments":       redactedMap(input.Arguments),
			"idempotency_key": input.IdempotencyKey,
		},
		Output: map[string]any{
			"status":         status,
			"error":          message,
			"result_summary": execution.ResultSummary,
		},
		DurationMs: durationMS,
		Metadata: map[string]any{
			"policy":              execution.Policy,
			"governance_decision": execution.GovernanceDecision,
			"risk_level":          tool.RiskLevel,
		},
	})
}

func (s *Service) recordToolMetric(ctx context.Context, name string, executionID uuid.UUID, value float64, metadata map[string]any) {
	if s.observability == nil {
		return
	}
	_, _ = s.observability.RecordMetric(ctx, observability.RecordMetricInput{
		MetricType: observability.MetricHealth,
		MetricName: name,
		EntityID:   &executionID,
		EntityType: "tool_execution",
		Value:      value,
		Metadata:   metadata,
	})
}

func (s *Service) completeObservationTrace(ctx context.Context, trace *observability.Trace, status observability.TraceStatus) {
	if s.observability == nil || trace == nil {
		return
	}
	_ = s.observability.CompleteTrace(ctx, trace.ID, string(status))
}

func optionalUUIDString(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

func redactedMap(input map[string]any) map[string]any {
	if input == nil {
		return map[string]any{}
	}
	output := make(map[string]any, len(input))
	for key, value := range input {
		if sensitiveKey(key) {
			output[key] = "[redacted]"
			continue
		}
		if nested, ok := value.(map[string]any); ok {
			output[key] = redactedMap(nested)
			continue
		}
		output[key] = value
	}
	return output
}

func sensitiveKey(key string) bool {
	normalized := strings.ToLower(key)
	return strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "token") ||
		strings.Contains(normalized, "password") ||
		strings.Contains(normalized, "api_key") ||
		strings.Contains(normalized, "authorization")
}

func requestedByHuman(input ExecuteToolInput) *uuid.UUID {
	if strings.EqualFold(input.ActorType, "human") || strings.EqualFold(input.ActorType, "internal_human") {
		return &input.ActorID
	}
	return nil
}

func applyTenantExecutionScope(ctx context.Context, input *ExecuteToolInput) error {
	orgID := currentTenantOrganizationID(ctx)
	if orgID == nil {
		return nil
	}
	if input.OrganizationID != nil && *input.OrganizationID != *orgID {
		return fmt.Errorf("%w: tool execution organization must match current organization", ErrForbidden)
	}
	id := *orgID
	input.OrganizationID = &id
	return nil
}

func ensureTenantExecutionAccess(ctx context.Context, organizationID *uuid.UUID) error {
	orgID := currentTenantOrganizationID(ctx)
	if orgID == nil || organizationID == nil {
		return nil
	}
	if *orgID != *organizationID {
		return fmt.Errorf("%w: tool execution is outside current organization", ErrForbidden)
	}
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

func tenantAuthorityTier(tenant *middleware.TenantContext) string {
	if tenant == nil {
		return ""
	}
	return tenant.AuthorityTier
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

func normalizeToolCategory(category string) string {
	switch strings.TrimSpace(category) {
	case ToolCategoryCoreData:
		return ToolCategoryCoreData
	case ToolCategoryBusinessApproval:
		return ToolCategoryBusinessApproval
	case ToolCategoryExecutionOperation, "":
		return ToolCategoryExecutionOperation
	default:
		return ToolCategoryExecutionOperation
	}
}

func approvalTierForCategory(category string) string {
	switch normalizeToolCategory(category) {
	case ToolCategoryCoreData:
		return ApprovalTierOrganizationCreator
	case ToolCategoryBusinessApproval:
		return ApprovalTierReviewer
	default:
		return ApprovalTierExecutor
	}
}

func normalizeApprovalTier(tier string, fallback string) string {
	switch strings.TrimSpace(tier) {
	case ApprovalTierOrganizationCreator:
		return ApprovalTierOrganizationCreator
	case ApprovalTierReviewer:
		return ApprovalTierReviewer
	case ApprovalTierExecutor:
		return ApprovalTierExecutor
	case "":
		return fallback
	default:
		return ""
	}
}

func normalizeCreateInterfaceFileInput(input CreateInterfaceFileInput) (CreateInterfaceFileInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.FileType = normalizeInterfaceFileType(input.FileType)
	if input.Name == "" {
		return input, fmt.Errorf("%w: name is required", ErrValidation)
	}
	if input.FileType == "" {
		return input, fmt.Errorf("%w: file_type must be json, yaml, or markdown", ErrValidation)
	}
	if strings.TrimSpace(input.Content) == "" {
		return input, fmt.Errorf("%w: content is required", ErrValidation)
	}
	if err := validateInterfaceContent(input.FileType, input.Content); err != nil {
		return input, err
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
	return input, nil
}

func normalizeUpdateInterfaceFileInput(input UpdateInterfaceFileInput) (UpdateInterfaceFileInput, error) {
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return input, fmt.Errorf("%w: name is required", ErrValidation)
		}
		input.Name = &name
	}
	if input.FileType != nil {
		fileType := normalizeInterfaceFileType(*input.FileType)
		if fileType == "" {
			return input, fmt.Errorf("%w: file_type must be json, yaml, or markdown", ErrValidation)
		}
		input.FileType = &fileType
	}
	if input.Content != nil {
		if strings.TrimSpace(*input.Content) == "" {
			return input, fmt.Errorf("%w: content is required", ErrValidation)
		}
		if input.FileType != nil && *input.FileType == "json" {
			if err := validateInterfaceContent(*input.FileType, *input.Content); err != nil {
				return input, err
			}
		}
	}
	return input, nil
}

func normalizeInterfaceFileType(fileType string) string {
	switch strings.ToLower(strings.TrimSpace(fileType)) {
	case "json":
		return "json"
	case "yaml", "yml":
		return "yaml"
	case "markdown", "md":
		return "markdown"
	default:
		return ""
	}
}

func validateInterfaceContent(fileType string, content string) error {
	if fileType != "json" {
		return nil
	}
	var parsed any
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return fmt.Errorf("%w: content must be valid JSON", ErrValidation)
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func authorityTierAllows(actual string, required string) bool {
	return authorityTierWeight(actual) >= authorityTierWeight(required) && authorityTierWeight(required) > 0
}

func authorityTierWeight(tier string) int {
	switch tier {
	case ApprovalTierOrganizationCreator:
		return 3
	case ApprovalTierReviewer:
		return 2
	case ApprovalTierExecutor:
		return 1
	default:
		return 0
	}
}
