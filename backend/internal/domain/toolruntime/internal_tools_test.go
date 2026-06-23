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

func TestSchemaChangePreviewToolRunsVerifier(t *testing.T) {
	requestID := uuid.New()
	verifier := &fakeSchemaVerifier{}
	tools := InternalToolsWithPlatform(nil, nil, nil, PlatformToolServices{SchemaVerifier: verifier})

	result, err := tools["schema.change.preview"](context.Background(), ExecuteToolInput{
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
}

func (f *fakeERPActionService) RunAction(_ context.Context, tableCode string, key string, action string, input erp.ActionInput) (*erp.ActionResult, error) {
	f.tableCode = tableCode
	f.key = key
	f.action = action
	return &erp.ActionResult{TableCode: tableCode, Key: key, Action: action, Status: "approved", Record: &erp.Record{TableCode: tableCode, Key: key, Data: input.Data}}, nil
}

type fakeSchemaVerifier struct {
	requestID uuid.UUID
}

func (f *fakeSchemaVerifier) VerifySchemaChange(_ context.Context, _ uuid.UUID, requestID uuid.UUID) (*systemadmin.SchemaVerificationReport, error) {
	f.requestID = requestID
	return &systemadmin.SchemaVerificationReport{ChangeRequestID: requestID, Status: "passed", CanApply: true}, nil
}

type fakeRuntimeOperationService struct {
	operationID string
}

func (f *fakeRuntimeOperationService) ExecuteOperation(_ context.Context, operationID string, input domainruntime.RuntimeExecutionRequest) (*domainruntime.RuntimeExecutionResult, error) {
	f.operationID = operationID
	return &domainruntime.RuntimeExecutionResult{Status: "ok", Data: input.Body}, nil
}
