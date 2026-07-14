package toolruntime

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/erp"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/organization"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/project"
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

func TestProjectLifecycleToolsExecuteEffectiveActions(t *testing.T) {
	projectID := uuid.New()
	deliverableID := uuid.New()
	service := &fakeProjectService{}
	tools := InternalTools(service, nil, nil)
	input := ExecuteToolInput{ActorID: uuid.New(), ActorType: "internal_human"}

	input.Arguments = map[string]any{"project_id": projectID.String(), "status": "active", "note": "AI approved"}
	if _, err := tools["project.update_status"](context.Background(), input); err != nil {
		t.Fatalf("update status tool error = %v", err)
	}
	input.Arguments = map[string]any{"project_id": projectID.String(), "name": "Acceptance package", "status": "draft"}
	if _, err := tools["project.create_deliverable"](context.Background(), input); err != nil {
		t.Fatalf("create deliverable tool error = %v", err)
	}
	input.Arguments = map[string]any{"deliverable_id": deliverableID.String(), "reason": "Evidence verified"}
	if _, err := tools["project.accept_deliverable"](context.Background(), input); err != nil {
		t.Fatalf("accept deliverable tool error = %v", err)
	}
	input.Arguments = map[string]any{"project_id": projectID.String(), "outcome_score": 92, "conclusion": "Objectives achieved"}
	if _, err := tools["project.close_feedback"](context.Background(), input); err != nil {
		t.Fatalf("close feedback tool error = %v", err)
	}

	if service.statusProjectID != projectID || service.statusInput.Status != "active" {
		t.Fatalf("status call = %s %#v", service.statusProjectID, service.statusInput)
	}
	if service.deliverableProjectID != projectID || service.deliverableInput.Name != "Acceptance package" {
		t.Fatalf("deliverable call = %s %#v", service.deliverableProjectID, service.deliverableInput)
	}
	if service.acceptDeliverableID != deliverableID || service.acceptInput.Reason != "Evidence verified" {
		t.Fatalf("accept call = %s %#v", service.acceptDeliverableID, service.acceptInput)
	}
	if service.feedbackProjectID != projectID || service.feedbackInput.Conclusion != "Objectives achieved" {
		t.Fatalf("feedback call = %s %#v", service.feedbackProjectID, service.feedbackInput)
	}
}

func TestNewProjectToolDefinitionsCarryBilingualMetadata(t *testing.T) {
	wanted := map[string]bool{
		"project.update_status": false, "project.create_deliverable": false,
		"project.accept_deliverable": false, "project.close_feedback": false,
	}
	for _, definition := range DefaultToolDefinitions() {
		if _, ok := wanted[definition.Name]; !ok {
			continue
		}
		wanted[definition.Name] = definition.Metadata["label_zh"] != "" && definition.Metadata["label_en"] != ""
	}
	for name, valid := range wanted {
		if !valid {
			t.Fatalf("tool %s missing bilingual metadata", name)
		}
	}
}

type fakeERPActionService struct {
	tableCode string
	key       string
	action    string
	input     erp.ActionInput
}

type fakeProjectService struct {
	statusProjectID      uuid.UUID
	statusInput          project.UpdateProjectStatusInput
	deliverableProjectID uuid.UUID
	deliverableInput     project.CreateDeliverableInput
	acceptDeliverableID  uuid.UUID
	acceptInput          project.DeliverableActionInput
	feedbackProjectID    uuid.UUID
	feedbackInput        project.CloseFeedbackInput
}

func (f *fakeProjectService) AnalyzeRequirement(context.Context, uuid.UUID, project.AnalyzeRequirementInput) (*project.Requirement, error) {
	return &project.Requirement{}, nil
}

func (f *fakeProjectService) MatchProjectActors(context.Context, uuid.UUID, project.MatchProjectActorsInput) ([]organization.MemberMatchCandidate, error) {
	return []organization.MemberMatchCandidate{}, nil
}

func (f *fakeProjectService) BindProjectWorkflow(context.Context, uuid.UUID, project.BindProjectWorkflowInput) (*project.ProjectWorkflow, error) {
	return &project.ProjectWorkflow{}, nil
}

func (f *fakeProjectService) GetCostSummary(context.Context, uuid.UUID) (*project.CostSummary, error) {
	return &project.CostSummary{}, nil
}

func (f *fakeProjectService) CreateCostEntry(context.Context, uuid.UUID, project.CreateCostEntryInput) (*project.CostEntry, error) {
	return &project.CostEntry{}, nil
}

func (f *fakeProjectService) UpdateProjectStatus(_ context.Context, id uuid.UUID, input project.UpdateProjectStatusInput) (*project.Project, error) {
	f.statusProjectID, f.statusInput = id, input
	return &project.Project{ID: id, Status: input.Status}, nil
}

func (f *fakeProjectService) CreateDeliverable(_ context.Context, id uuid.UUID, input project.CreateDeliverableInput) (*project.Deliverable, error) {
	f.deliverableProjectID, f.deliverableInput = id, input
	return &project.Deliverable{ID: uuid.New(), ProjectID: id, Name: input.Name}, nil
}

func (f *fakeProjectService) AcceptDeliverable(_ context.Context, id uuid.UUID, input project.DeliverableActionInput) (*project.Deliverable, error) {
	f.acceptDeliverableID, f.acceptInput = id, input
	return &project.Deliverable{ID: id, Status: "accepted"}, nil
}

func (f *fakeProjectService) CloseFeedback(_ context.Context, id uuid.UUID, input project.CloseFeedbackInput) (map[string]any, error) {
	f.feedbackProjectID, f.feedbackInput = id, input
	return map[string]any{"status": "closed"}, nil
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

func (f *fakeRuntimeOperationService) ExecuteAssistantOperation(_ context.Context, operationID string, input domainruntime.RuntimeExecutionRequest) (*domainruntime.RuntimeExecutionResult, error) {
	f.operationID = operationID
	return &domainruntime.RuntimeExecutionResult{Status: "ok", Data: input.Body}, nil
}
