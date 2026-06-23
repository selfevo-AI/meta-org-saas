package assistant

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/toolruntime"
)

func TestToolRunnerBlocksToolOutsideAllowlist(t *testing.T) {
	runner := NewToolRunner(&fakeToolExecutor{}, ToolRunnerConfig{AllowedTools: []string{"project.match_members"}})

	_, err := runner.ExecuteTool(context.Background(), ToolRunRequest{
		Session: &Session{ID: uuid.New(), ActorID: uuid.New(), ActorType: "internal_human", ModuleKey: "project"},
		Call:    aigatewayToolCall("project.create_cost_entry"),
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("error = %v, want ErrForbidden", err)
	}
}

func TestToolRunnerPassesAllowedTool(t *testing.T) {
	executor := &fakeToolExecutor{output: &toolruntime.ExecuteToolOutput{Execution: &toolruntime.ToolExecution{ID: uuid.New(), Status: toolruntime.ExecutionCompleted}}}
	runner := NewToolRunner(executor, ToolRunnerConfig{AllowedTools: []string{"project.match_members"}})

	output, err := runner.ExecuteTool(context.Background(), ToolRunRequest{
		Session: &Session{ID: uuid.New(), ActorID: uuid.New(), ActorType: "internal_human", ModuleKey: "project"},
		Call:    aigatewayToolCall("project.match_members"),
	})
	if err != nil {
		t.Fatalf("ExecuteTool returned error: %v", err)
	}
	if output.Execution.Status != toolruntime.ExecutionCompleted {
		t.Fatalf("status = %s, want completed", output.Execution.Status)
	}
}

func aigatewayToolCall(name string) ToolCallRequest {
	return ToolCallRequest{ID: "call-1", Name: name, Arguments: map[string]any{"project_id": uuid.New().String()}}
}

type fakeToolExecutor struct {
	output    *toolruntime.ExecuteToolOutput
	approval  *toolruntime.ToolApproval
	execution *toolruntime.ToolExecution
}

func (f *fakeToolExecutor) ExecuteTool(context.Context, toolruntime.ExecuteToolInput) (*toolruntime.ExecuteToolOutput, error) {
	if f.output == nil {
		return &toolruntime.ExecuteToolOutput{Execution: &toolruntime.ToolExecution{ID: uuid.New(), Status: toolruntime.ExecutionCompleted}}, nil
	}
	return f.output, nil
}

func (f *fakeToolExecutor) ListTools(context.Context, int) ([]toolruntime.ToolDefinition, error) {
	return []toolruntime.ToolDefinition{}, nil
}

func (f *fakeToolExecutor) GetApproval(context.Context, uuid.UUID) (*toolruntime.ToolApproval, error) {
	if f.approval != nil {
		return f.approval, nil
	}
	return nil, ErrNotFound
}

func (f *fakeToolExecutor) GetExecution(context.Context, uuid.UUID) (*toolruntime.ToolExecution, error) {
	if f.execution != nil {
		return f.execution, nil
	}
	return nil, ErrNotFound
}
