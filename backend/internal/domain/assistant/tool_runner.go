package assistant

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/toolruntime"
)

type ToolCallRequest struct {
	ID        string
	Name      string
	Arguments map[string]any
}

type ToolRunRequest struct {
	Session        *Session
	ContextPackage *ContextPackage
	Call           ToolCallRequest
	InvocationID   *uuid.UUID
}

type ToolRunnerConfig struct {
	AllowedTools []string
}

type ToolRunner struct {
	executor ToolExecutor
	allowed  map[string]bool
}

func NewToolRunner(executor ToolExecutor, config ToolRunnerConfig) *ToolRunner {
	allowed := map[string]bool{}
	for _, name := range config.AllowedTools {
		if name != "" {
			allowed[name] = true
		}
	}
	return &ToolRunner{executor: executor, allowed: allowed}
}

func (r *ToolRunner) ExecuteTool(ctx context.Context, request ToolRunRequest) (*toolruntime.ExecuteToolOutput, error) {
	if r == nil || r.executor == nil {
		return nil, fmt.Errorf("%w: tool runner executor is not configured", ErrValidation)
	}
	if request.Session == nil {
		return nil, fmt.Errorf("%w: tool runner session is required", ErrValidation)
	}
	if len(r.allowed) > 0 && !r.allowed[request.Call.Name] {
		return nil, fmt.Errorf("%w: tool %s is not allowed in this assistant context", ErrForbidden, request.Call.Name)
	}
	args := request.Call.Arguments
	if args == nil {
		args = map[string]any{}
	}
	return r.executor.ExecuteTool(ctx, toolruntime.ExecuteToolInput{
		ToolName:       request.Call.Name,
		InvocationID:   request.InvocationID,
		ActorID:        request.Session.ActorID,
		ActorType:      request.Session.ActorType,
		OrganizationID: request.Session.OrganizationID,
		DepartmentID:   request.Session.DepartmentID,
		ProjectID:      request.Session.ProjectID,
		WorkflowID:     request.Session.WorkflowID,
		TaskID:         request.Session.TaskID,
		IdempotencyKey: fmt.Sprintf("assistant:%s:%s", request.Session.ID, request.Call.ID),
		Arguments:      args,
	})
}
