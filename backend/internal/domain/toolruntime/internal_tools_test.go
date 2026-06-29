package toolruntime

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/erp"
	domainruntime "github.com/selfevo-AI/meta-org-saas/backend/internal/domain/runtime"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/systemadmin"
)

func TestERPActionExecuteToolRunsERPAction(t *testing.T) {
	erpSvc := &fakeERPActionService{}
	tools := InternalToolsWithPlatform(nil, nil, nil, PlatformToolServices{ERP: erpSvc})

	result, err := tools["erp.action.execute"](context.Background(), ExecuteToolInput{
		ActorID:   uuid.New(),
		ActorType: "internal_human",
		Arguments: map[string]any{
			"table_code": "MREQ",
			"key":        "REQ-1",
			"action":     "approve",
			"data":       map[string]any{"approver": "u1"},
		},
	})
	if err != nil {
		t.Fatalf("tool returned error: %v", err)
	}
	if erpSvc.tableCode != "MREQ" || erpSvc.key != "REQ-1" || erpSvc.action != "approve" {
		t.Fatalf("erp call = %s/%s/%s", erpSvc.tableCode, erpSvc.key, erpSvc.action)
	}
	if result.Data["erp_action"] == nil {
		t.Fatalf("result data = %#v, want erp_action", result.Data)
	}
}

func TestERPActionExecuteToolForwardsExecutionMetadata(t *testing.T) {
	erpSvc := &fakeERPActionService{}
	sessionID := uuid.New()
	toolExecutionID := uuid.New()
	tools := InternalToolsWithPlatform(nil, nil, nil, PlatformToolServices{ERP: erpSvc})

	_, err := tools["erp.action.execute"](context.Background(), ExecuteToolInput{
		ActorID:        uuid.New(),
		ActorType:      "internal_human",
		IdempotencyKey: "assistant-session-tool-call",
		Arguments: map[string]any{
			"table_code":           "MREQ",
			"key":                  "REQ-1",
			"action":               "approve",
			"assistant_session_id": sessionID.String(),
			"tool_execution_id":    toolExecutionID.String(),
			"context_package_id":   uuid.New().String(),
			"data":                 map[string]any{"approver": "u1"},
		},
	})
	if err != nil {
		t.Fatalf("tool returned error: %v", err)
	}
	if erpSvc.input.ActorType != "internal_human" || erpSvc.input.ActorID == nil {
		t.Fatalf("input actor = %#v/%#v, want forwarded actor", erpSvc.input.ActorID, erpSvc.input.ActorType)
	}
	if erpSvc.input.Source != "toolruntime" || erpSvc.input.IdempotencyKey != "assistant-session-tool-call" {
		t.Fatalf("input source/idempotency = %q/%q, want toolruntime/assistant-session-tool-call", erpSvc.input.Source, erpSvc.input.IdempotencyKey)
	}
	if erpSvc.input.AssistantSessionID == nil || *erpSvc.input.AssistantSessionID != sessionID {
		t.Fatalf("assistant session id = %#v, want %s", erpSvc.input.AssistantSessionID, sessionID)
	}
	if erpSvc.input.ToolExecutionID == nil || *erpSvc.input.ToolExecutionID != toolExecutionID {
		t.Fatalf("tool execution id = %#v, want %s", erpSvc.input.ToolExecutionID, toolExecutionID)
	}
}

func TestIndustrySolutionChangePreviewToolRunsVerifier(t *testing.T) {
	requestID := uuid.New()
	verifier := &fakeIndustrySolutionChangeVerifier{}
	tools := InternalToolsWithPlatform(nil, nil, nil, PlatformToolServices{IndustrySolutionVerifier: verifier})

	result, err := tools["industry.solution.change.preview"](context.Background(), ExecuteToolInput{
		ActorID:   uuid.New(),
		ActorType: "internal_human",
		Arguments: map[string]any{"request_id": requestID.String()},
	})
	if err != nil {
		t.Fatalf("tool returned error: %v", err)
	}
	if verifier.requestID != requestID {
		t.Fatalf("request id = %s, want %s", verifier.requestID, requestID)
	}
	if result.Data["verification"] == nil {
		t.Fatalf("result data = %#v, want verification", result.Data)
	}
}

func TestRuntimeOperationExecuteToolRunsRuntimeService(t *testing.T) {
	runtimeSvc := &fakeRuntimeOperationService{}
	tools := InternalToolsWithPlatform(nil, nil, nil, PlatformToolServices{Runtime: runtimeSvc})

	result, err := tools["runtime.operation.execute"](context.Background(), ExecuteToolInput{
		ActorID:   uuid.New(),
		ActorType: "internal_human",
		Arguments: map[string]any{
			"operation_id": "op-1",
			"path":         map[string]any{"recordKey": "R-1"},
			"query":        map[string]any{"limit": "10"},
			"body":         map[string]any{"title": "Runtime row"},
		},
	})
	if err != nil {
		t.Fatalf("tool returned error: %v", err)
	}
	if runtimeSvc.operationID != "op-1" {
		t.Fatalf("operation id = %s, want op-1", runtimeSvc.operationID)
	}
	if result.Data["runtime_result"] == nil {
		t.Fatalf("result data = %#v, want runtime_result", result.Data)
	}
}

func TestContextProposalApplyToolRequiresProposalService(t *testing.T) {
	tools := InternalToolsWithPlatform(nil, nil, nil, PlatformToolServices{})

	_, err := tools["context.proposal.apply"](context.Background(), ExecuteToolInput{
		ActorID:   uuid.New(),
		ActorType: "internal_human",
		Arguments: map[string]any{"proposal_id": uuid.New().String()},
	})
	if err == nil {
		t.Fatalf("context proposal tool returned nil error without service")
	}
}

type fakeERPActionService struct {
	tableCode string
	key       string
	action    string
	input     erp.ActionInput
}

func (f *fakeERPActionService) RunAction(_ context.Context, tableCode string, key string, action string, input erp.ActionInput) (*erp.ActionResult, error) {
	f.tableCode = tableCode
	f.key = key
	f.action = action
	f.input = input
	return &erp.ActionResult{TableCode: tableCode, Key: key, Action: action, Status: "approved", Record: &erp.Record{TableCode: tableCode, Key: key, Data: input.Data}}, nil
}

type fakeIndustrySolutionChangeVerifier struct {
	requestID uuid.UUID
}

func (f *fakeIndustrySolutionChangeVerifier) VerifyIndustrySolutionChange(_ context.Context, _ uuid.UUID, requestID uuid.UUID) (*systemadmin.IndustrySolutionVerificationReport, error) {
	f.requestID = requestID
	return &systemadmin.IndustrySolutionVerificationReport{ChangeRequestID: requestID, Status: "passed", CanApply: true}, nil
}

type fakeRuntimeOperationService struct {
	operationID string
}

func (f *fakeRuntimeOperationService) ExecuteOperation(_ context.Context, operationID string, input domainruntime.RuntimeExecutionRequest) (*domainruntime.RuntimeExecutionResult, error) {
	f.operationID = operationID
	return &domainruntime.RuntimeExecutionResult{Status: "ok", Data: input.Body}, nil
}
