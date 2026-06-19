package toolruntime

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org/backend/internal/pkg/middleware"
	"github.com/selfevo-AI/meta-org/backend/internal/pkg/securitykernel"
)

func TestEffectivePolicyGovernanceOverridesAuto(t *testing.T) {
	policy := EffectivePolicy("auto", GovernanceResult{Decision: "approve", Allowed: false})
	if policy != "approve" {
		t.Fatalf("policy = %q, want approve", policy)
	}
}

func TestEffectivePolicyDenyWins(t *testing.T) {
	policy := EffectivePolicy("auto", GovernanceResult{Decision: "deny", Allowed: false})
	if policy != "deny" {
		t.Fatalf("policy = %q, want deny", policy)
	}
}

func TestEffectivePolicyNotifyOverridesAuto(t *testing.T) {
	policy := EffectivePolicy("auto", GovernanceResult{Decision: "notify", Allowed: true})
	if policy != "notify" {
		t.Fatalf("policy = %q, want notify", policy)
	}
}

func TestExecuteToolUsesCurrentTenantOrganization(t *testing.T) {
	orgID := uuid.New()
	toolID := uuid.New()
	actorID := uuid.New()
	ctx := context.WithValue(context.Background(), middleware.TenantContextKey, &middleware.TenantContext{OrganizationID: &orgID})
	repo := &fakeApprovalRepository{
		tool: ToolDefinition{
			ID:            toolID,
			Name:          "project.summarize",
			DefaultPolicy: PolicyAuto,
			RiskLevel:     "medium",
			RequiredLevel: "L1",
			IsActive:      true,
		},
	}
	svc := NewService(repo, nil, map[string]ToolAdapter{
		"project.summarize": func(_ context.Context, input ExecuteToolInput) (ToolResult, error) {
			if input.OrganizationID == nil || *input.OrganizationID != orgID {
				t.Fatalf("adapter organization = %v, want %s", input.OrganizationID, orgID)
			}
			return ToolResult{Summary: "summarized", Data: map[string]any{"ok": true}}, nil
		},
	})

	result, err := svc.ExecuteTool(ctx, ExecuteToolInput{
		ToolName:  "project.summarize",
		ActorID:   actorID,
		ActorType: "internal_human",
	})
	if err != nil {
		t.Fatalf("ExecuteTool returned error: %v", err)
	}
	if repo.createExecutionInput.OrganizationID == nil || *repo.createExecutionInput.OrganizationID != orgID {
		t.Fatalf("created execution organization = %v, want %s", repo.createExecutionInput.OrganizationID, orgID)
	}
	if result.Execution.OrganizationID == nil || *result.Execution.OrganizationID != orgID {
		t.Fatalf("execution organization = %v, want %s", result.Execution.OrganizationID, orgID)
	}
}

func TestExecuteToolRejectsCrossTenantOrganization(t *testing.T) {
	orgID := uuid.New()
	otherOrgID := uuid.New()
	ctx := context.WithValue(context.Background(), middleware.TenantContextKey, &middleware.TenantContext{OrganizationID: &orgID})
	svc := NewService(&fakeApprovalRepository{}, nil, nil)

	_, err := svc.ExecuteTool(ctx, ExecuteToolInput{
		ToolName:       "project.summarize",
		ActorID:        uuid.New(),
		ActorType:      "internal_human",
		OrganizationID: &otherOrgID,
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("ExecuteTool error = %v, want ErrForbidden", err)
	}
}

func TestExecuteToolRequiresSecurityKernelAuthorization(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()
	ctx := context.WithValue(context.Background(), middleware.TenantContextKey, &middleware.TenantContext{
		OrganizationID: &orgID,
		AuthorityTier:  "executor",
		EnabledModules: map[string]bool{"toolruntime": true},
	})
	repo := &fakeApprovalRepository{
		tool: ToolDefinition{
			ID:                   uuid.New(),
			Name:                 "project.summarize",
			DefaultPolicy:        PolicyNotify,
			RiskLevel:            "medium",
			RequiredLevel:        "L1",
			ToolCategory:         ToolCategoryExecutionOperation,
			ApprovalTierRequired: ApprovalTierExecutor,
			IsActive:             true,
		},
	}
	kernel := &fakeSecurityKernel{decision: securitykernel.Decision{Allowed: false, Reason: "tool_denied", DecisionType: "deny"}, err: securitykernel.ErrDenied}
	adapterCalled := false
	svc := NewService(repo, nil, map[string]ToolAdapter{
		"project.summarize": func(context.Context, ExecuteToolInput) (ToolResult, error) {
			adapterCalled = true
			return ToolResult{}, nil
		},
	}, WithSecurityKernel(kernel))

	_, err := svc.ExecuteTool(ctx, ExecuteToolInput{
		ToolName:  "project.summarize",
		ActorID:   actorID,
		ActorType: "internal_human",
	})

	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("ExecuteTool error = %v, want ErrForbidden", err)
	}
	if adapterCalled {
		t.Fatalf("adapter called after security denial")
	}
	if kernel.lastRequest.Resource.ResourceType != "tool" || kernel.lastRequest.Resource.Action != "execute" {
		t.Fatalf("security resource = %#v, want tool execute", kernel.lastRequest.Resource)
	}
}

func TestApproveRequiresSecurityKernelAuthorization(t *testing.T) {
	orgID := uuid.New()
	toolID := uuid.New()
	executionID := uuid.New()
	approvalID := uuid.New()
	reviewerID := uuid.New()
	repo := &fakeApprovalRepository{
		tool: ToolDefinition{
			ID:                   toolID,
			Name:                 "project.summarize",
			DefaultPolicy:        PolicyApprove,
			RiskLevel:            "high",
			RequiredLevel:        "L3",
			ToolCategory:         ToolCategoryBusinessApproval,
			ApprovalTierRequired: ApprovalTierReviewer,
			IsActive:             true,
		},
		execution: ToolExecution{
			ID:             executionID,
			ToolID:         toolID,
			ActorID:        uuid.New(),
			ActorType:      "internal_human",
			OrganizationID: &orgID,
			Status:         ExecutionApprovalRequired,
			Arguments:      map[string]any{},
		},
		approval: ToolApproval{ID: approvalID, ExecutionID: executionID, Status: ApprovalPending},
		tier:     ApprovalTierReviewer,
	}
	kernel := &fakeSecurityKernel{decision: securitykernel.Decision{Allowed: false, Reason: "approval_denied", DecisionType: "deny"}, err: securitykernel.ErrDenied}
	svc := NewService(repo, nil, nil, WithSecurityKernel(kernel))

	if _, err := svc.Approve(context.Background(), approvalID, &reviewerID, "review"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Approve error = %v, want ErrForbidden", err)
	}
	if repo.approval.Status != ApprovalPending {
		t.Fatalf("approval status = %q, want pending", repo.approval.Status)
	}
	if kernel.lastRequest.Resource.ResourceType != "tool_approval" || kernel.lastRequest.Resource.Action != "review" {
		t.Fatalf("security resource = %#v, want tool approval review", kernel.lastRequest.Resource)
	}
}

func TestApproveRunsApprovedToolOnce(t *testing.T) {
	ctx := context.Background()
	toolID := uuid.New()
	executionID := uuid.New()
	approvalID := uuid.New()
	actorID := uuid.New()
	reviewerID := uuid.New()
	expiresAt := time.Now().Add(time.Hour)
	repo := &fakeApprovalRepository{
		tool: ToolDefinition{
			ID:                   toolID,
			Name:                 "project.summarize",
			DefaultPolicy:        PolicyApprove,
			RiskLevel:            "high",
			RequiredLevel:        "L3",
			ToolCategory:         ToolCategoryBusinessApproval,
			ApprovalTierRequired: ApprovalTierReviewer,
			IsActive:             true,
		},
		execution: ToolExecution{
			ID:                 executionID,
			ToolID:             toolID,
			ActorID:            actorID,
			ActorType:          "internal_human",
			IdempotencyKey:     "idem-1",
			Policy:             PolicyApprove,
			GovernanceDecision: "approve",
			Status:             ExecutionApprovalRequired,
			Arguments:          map[string]any{"project_id": "p-1"},
			Result:             map[string]any{},
		},
		approval: ToolApproval{
			ID:          approvalID,
			ExecutionID: executionID,
			Status:      ApprovalPending,
			ExpiresAt:   &expiresAt,
		},
		tier: ApprovalTierReviewer,
	}
	calls := 0
	svc := NewService(repo, nil, map[string]ToolAdapter{
		"project.summarize": func(_ context.Context, input ExecuteToolInput) (ToolResult, error) {
			calls++
			if input.Arguments["project_id"] != "p-1" {
				t.Fatalf("project_id = %v, want p-1", input.Arguments["project_id"])
			}
			return ToolResult{Summary: "summarized", Data: map[string]any{"ok": true}}, nil
		},
	})

	first, err := svc.Approve(ctx, approvalID, &reviewerID, "looks good")
	if err != nil {
		t.Fatalf("Approve returned error: %v", err)
	}
	if first.Approval.Status != ApprovalApproved {
		t.Fatalf("approval status = %q, want approved", first.Approval.Status)
	}
	if first.Execution.Status != ExecutionCompleted {
		t.Fatalf("execution status = %q, want completed", first.Execution.Status)
	}
	if calls != 1 {
		t.Fatalf("adapter calls = %d, want 1", calls)
	}

	second, err := svc.Approve(ctx, approvalID, &reviewerID, "duplicate")
	if err != nil {
		t.Fatalf("duplicate Approve returned error: %v", err)
	}
	if second.Execution.Status != ExecutionCompleted {
		t.Fatalf("duplicate execution status = %q, want completed", second.Execution.Status)
	}
	if calls != 1 {
		t.Fatalf("adapter calls after duplicate = %d, want 1", calls)
	}
}

func TestCreateInterfaceFileValidation(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name    string
		input   CreateInterfaceFileInput
		wantErr string
	}{
		{
			name:    "name required",
			input:   CreateInterfaceFileInput{FileType: "json", Content: "{}"},
			wantErr: "name is required",
		},
		{
			name:    "type required",
			input:   CreateInterfaceFileInput{Name: "contract", FileType: "txt", Content: "{}"},
			wantErr: "file_type must be json, yaml, or markdown",
		},
		{
			name:    "content required",
			input:   CreateInterfaceFileInput{Name: "contract", FileType: "markdown", Content: " "},
			wantErr: "content is required",
		},
		{
			name:    "json validated",
			input:   CreateInterfaceFileInput{Name: "contract", FileType: "json", Content: "{invalid"},
			wantErr: "content must be valid JSON",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(&fakeApprovalRepository{}, nil, nil)
			_, err := svc.CreateInterfaceFile(ctx, tt.input, nil)
			if err == nil {
				t.Fatalf("CreateInterfaceFile returned nil error")
			}
			if !errors.Is(err, ErrValidation) || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("CreateInterfaceFile error = %v, want validation containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestCreateInterfaceFileNormalizesInput(t *testing.T) {
	ctx := context.Background()
	createdBy := uuid.New()
	repo := &fakeApprovalRepository{}
	svc := NewService(repo, nil, nil)

	file, err := svc.CreateInterfaceFile(ctx, CreateInterfaceFileInput{
		Name:     "  Provider Contract  ",
		FileType: "yml",
		Content:  "provider: openai",
	}, &createdBy)
	if err != nil {
		t.Fatalf("CreateInterfaceFile returned error: %v", err)
	}
	if file.Name != "Provider Contract" {
		t.Fatalf("file name = %q, want trimmed name", file.Name)
	}
	if file.FileType != "yaml" {
		t.Fatalf("file type = %q, want yaml", file.FileType)
	}
	if file.Metadata == nil {
		t.Fatalf("metadata is nil, want empty map")
	}
	if file.CreatedBy == nil || *file.CreatedBy != createdBy {
		t.Fatalf("created_by = %v, want %s", file.CreatedBy, createdBy)
	}
}

func TestUpdateInterfaceFileValidatesExistingJSON(t *testing.T) {
	ctx := context.Background()
	fileID := uuid.New()
	repo := &fakeApprovalRepository{
		interfaceFiles: []InterfaceFile{
			{
				ID:       fileID,
				Name:     "Tool Contract",
				FileType: "json",
				Content:  `{"ok": true}`,
				Metadata: map[string]any{},
			},
		},
	}
	svc := NewService(repo, nil, nil)
	invalid := "{invalid"

	_, err := svc.UpdateInterfaceFile(ctx, fileID, UpdateInterfaceFileInput{Content: &invalid})
	if err == nil {
		t.Fatalf("UpdateInterfaceFile returned nil error")
	}
	if !errors.Is(err, ErrValidation) || !strings.Contains(err.Error(), "content must be valid JSON") {
		t.Fatalf("UpdateInterfaceFile error = %v, want JSON validation", err)
	}
}

func TestUpdateInterfaceFile(t *testing.T) {
	ctx := context.Background()
	fileID := uuid.New()
	repo := &fakeApprovalRepository{
		interfaceFiles: []InterfaceFile{
			{
				ID:       fileID,
				Name:     "Tool Contract",
				FileType: "json",
				Content:  `{"ok": true}`,
				Metadata: map[string]any{"kind": "tool"},
			},
		},
	}
	svc := NewService(repo, nil, nil)
	name := "Updated Contract"
	content := "# Updated"
	fileType := "md"

	file, err := svc.UpdateInterfaceFile(ctx, fileID, UpdateInterfaceFileInput{
		Name:     &name,
		FileType: &fileType,
		Content:  &content,
		Metadata: map[string]any{"kind": "agent"},
	})
	if err != nil {
		t.Fatalf("UpdateInterfaceFile returned error: %v", err)
	}
	if file.Name != name || file.FileType != "markdown" || file.Content != content {
		t.Fatalf("updated file = %#v", file)
	}
	if file.Metadata["kind"] != "agent" {
		t.Fatalf("metadata kind = %v, want agent", file.Metadata["kind"])
	}
}

type fakeApprovalRepository struct {
	tool                 ToolDefinition
	execution            ToolExecution
	approval             ToolApproval
	tier                 string
	interfaceFiles       []InterfaceFile
	createExecutionInput CreateExecutionInput
}

func (f *fakeApprovalRepository) CreateTool(context.Context, CreateToolInput) (*ToolDefinition, error) {
	return &f.tool, nil
}

func (f *fakeApprovalRepository) ListTools(context.Context, int) ([]ToolDefinition, error) {
	return []ToolDefinition{f.tool}, nil
}

func (f *fakeApprovalRepository) UpdateTool(context.Context, uuid.UUID, UpdateToolInput) (*ToolDefinition, error) {
	return &f.tool, nil
}

func (f *fakeApprovalRepository) GetToolByID(context.Context, uuid.UUID) (*ToolDefinition, error) {
	return &f.tool, nil
}

func (f *fakeApprovalRepository) GetToolByName(context.Context, string) (*ToolDefinition, error) {
	return &f.tool, nil
}

func (f *fakeApprovalRepository) CreateInterfaceFile(_ context.Context, input CreateInterfaceFileInput, createdBy *uuid.UUID) (*InterfaceFile, error) {
	file := InterfaceFile{
		ID:        uuid.New(),
		Name:      input.Name,
		FileType:  input.FileType,
		Content:   input.Content,
		Metadata:  input.Metadata,
		CreatedBy: createdBy,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	f.interfaceFiles = append(f.interfaceFiles, file)
	return &f.interfaceFiles[len(f.interfaceFiles)-1], nil
}

func (f *fakeApprovalRepository) ListInterfaceFiles(context.Context, int) ([]InterfaceFile, error) {
	return f.interfaceFiles, nil
}

func (f *fakeApprovalRepository) GetInterfaceFile(_ context.Context, id uuid.UUID) (*InterfaceFile, error) {
	for index := range f.interfaceFiles {
		if f.interfaceFiles[index].ID == id {
			return &f.interfaceFiles[index], nil
		}
	}
	return nil, ErrNotFound
}

func (f *fakeApprovalRepository) UpdateInterfaceFile(_ context.Context, id uuid.UUID, input UpdateInterfaceFileInput) (*InterfaceFile, error) {
	file, err := f.GetInterfaceFile(context.Background(), id)
	if err != nil {
		return nil, err
	}
	if input.Name != nil {
		file.Name = *input.Name
	}
	if input.FileType != nil {
		file.FileType = *input.FileType
	}
	if input.Content != nil {
		file.Content = *input.Content
	}
	if input.Metadata != nil {
		file.Metadata = input.Metadata
	}
	file.UpdatedAt = time.Now()
	return file, nil
}

func (f *fakeApprovalRepository) CreateExecution(_ context.Context, input CreateExecutionInput) (*ToolExecution, error) {
	f.createExecutionInput = input
	if f.execution.ID == uuid.Nil {
		f.execution = ToolExecution{
			ID:                 uuid.New(),
			ToolID:             input.ToolID,
			InvocationID:       input.InvocationID,
			ActorID:            input.ActorID,
			ActorType:          input.ActorType,
			OrganizationID:     input.OrganizationID,
			DepartmentID:       input.DepartmentID,
			ProjectID:          input.ProjectID,
			WorkflowID:         input.WorkflowID,
			TaskID:             input.TaskID,
			IdempotencyKey:     input.IdempotencyKey,
			Policy:             input.Policy,
			GovernanceDecision: input.GovernanceDecision,
			RequestedByHumanID: input.RequestedByHumanID,
			Status:             input.Status,
			Arguments:          input.Arguments,
			Result:             map[string]any{},
			CreatedAt:          time.Now(),
		}
	}
	return &f.execution, nil
}

func (f *fakeApprovalRepository) CompleteExecution(_ context.Context, _ uuid.UUID, input CompleteExecutionInput) (*ToolExecution, error) {
	f.execution.Status = input.Status
	f.execution.ResultSummary = input.ResultSummary
	f.execution.Result = input.Result
	f.execution.ErrorMessage = input.ErrorMessage
	now := time.Now()
	f.execution.CompletedAt = &now
	return &f.execution, nil
}

func (f *fakeApprovalRepository) CreateApproval(context.Context, uuid.UUID, *uuid.UUID, string) (*ToolApproval, error) {
	return &f.approval, nil
}

func (f *fakeApprovalRepository) ListExecutions(context.Context, int) ([]ToolExecution, error) {
	return []ToolExecution{f.execution}, nil
}

func (f *fakeApprovalRepository) GetExecution(context.Context, uuid.UUID) (*ToolExecution, error) {
	return &f.execution, nil
}

func (f *fakeApprovalRepository) GetApproval(context.Context, uuid.UUID) (*ToolApproval, error) {
	return &f.approval, nil
}

func (f *fakeApprovalRepository) GetHumanAuthorityTier(context.Context, uuid.UUID, *uuid.UUID) (string, error) {
	return f.tier, nil
}

func (f *fakeApprovalRepository) UpdateApproval(_ context.Context, _ uuid.UUID, status string, reviewedBy *uuid.UUID, reason string) (*ToolApproval, error) {
	f.approval.Status = status
	f.approval.ReviewedBy = reviewedBy
	if status == ApprovalApproved {
		f.approval.ApprovedByHumanID = reviewedBy
	}
	if reason != "" {
		f.approval.Reason = reason
	}
	now := time.Now()
	f.approval.ReviewedAt = &now
	switch status {
	case ApprovalApproved:
		f.execution.Status = ExecutionApproved
	case ApprovalRejected:
		f.execution.Status = ExecutionRejected
	case ApprovalExpired:
		f.execution.Status = ExecutionFailed
	}
	return &f.approval, nil
}

type fakeSecurityKernel struct {
	lastRequest securitykernel.Request
	decision    securitykernel.Decision
	err         error
}

func (f *fakeSecurityKernel) Authorize(_ context.Context, request securitykernel.Request) (securitykernel.Decision, error) {
	f.lastRequest = request
	return f.decision, f.err
}
