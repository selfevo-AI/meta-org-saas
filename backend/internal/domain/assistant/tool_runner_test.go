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

func TestToolRunnerInjectsContextMetadata(t *testing.T) {
	sessionID := uuid.New()
	pkgID := uuid.New()
	executor := &fakeToolExecutor{}
	runner := NewToolRunner(executor, ToolRunnerConfig{})
	session := &Session{
		ID:        sessionID,
		ActorID:   uuid.New(),
		ActorType: "internal_human",
		ModuleKey: "erp",
	}

	_, err := runner.ExecuteTool(context.Background(), ToolRunRequest{
		Session:        session,
		ContextPackage: &ContextPackage{ID: pkgID, Provenance: map[string]any{"source": "context_dictionary"}},
		Call: ToolCallRequest{
			ID:        "call-1",
			Name:      "erp.action.execute",
			Arguments: map[string]any{"table_code": "MREQ", "key": "REQ-1", "action": "approve"},
		},
	})
	if err != nil {
		t.Fatalf("ExecuteTool returned error: %v", err)
	}
	if executor.input.Arguments["context_package_id"] != pkgID.String() {
		t.Fatalf("tool args = %#v, want context package id", executor.input.Arguments)
	}
	if executor.input.IdempotencyKey != "assistant:"+sessionID.String()+":"+pkgID.String()+":call-1" {
		t.Fatalf("idempotency = %q", executor.input.IdempotencyKey)
	}
}

func aigatewayToolCall(name string) ToolCallRequest {
	return ToolCallRequest{ID: "call-1", Name: name, Arguments: map[string]any{"project_id": uuid.New().String()}}
}

type fakeToolExecutor struct {
	output    *toolruntime.ExecuteToolOutput
	approval  *toolruntime.ToolApproval
	execution *toolruntime.ToolExecution
	input     toolruntime.ExecuteToolInput
}

func (f *fakeToolExecutor) ExecuteTool(_ context.Context, input toolruntime.ExecuteToolInput) (*toolruntime.ExecuteToolOutput, error) {
	f.input = input
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
