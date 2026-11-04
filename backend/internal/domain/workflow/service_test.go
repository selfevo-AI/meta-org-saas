package workflow

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
)

func TestNormalizeStageFillsDefaultsWithoutOverwriting(t *testing.T) {
	stage := Stage{Type: StageExecute}
	normalizeStage(&stage)
	if stage.Name != "execute" {
		t.Fatalf("Name = %q, want the type as fallback", stage.Name)
	}
	if stage.ID != "execute-execute" {
		t.Fatalf("ID = %q, want type-name composition", stage.ID)
	}
	if stage.AssigneeType != "either" || stage.RequiredPermissionLevel != "L1" || stage.RiskLevel != "low" {
		t.Fatalf("defaults not applied: %+v", stage)
	}
	if stage.EvaluationPolicy["primary_reviewer"] != "human" || stage.MatchingPolicy["ranking"] != "capability_weight_access" {
		t.Fatalf("policy defaults not applied: %+v", stage)
	}

	explicit := Stage{
		ID:                      "custom-id",
		Type:                    StageReview,
		Name:                    "Quality gate",
		AssigneeType:            "internal",
		RequiredPermissionLevel: "L3",
		RiskLevel:               "high",
		EvaluationPolicy:        map[string]any{"primary_reviewer": "machine"},
		MatchingPolicy:          map[string]any{"ranking": "manual"},
	}
	normalizeStage(&explicit)
	if explicit.ID != "custom-id" || explicit.Name != "Quality gate" || explicit.AssigneeType != "internal" ||
		explicit.RequiredPermissionLevel != "L3" || explicit.RiskLevel != "high" {
		t.Fatalf("normalizeStage overwrote explicit values: %+v", explicit)
	}
	if explicit.EvaluationPolicy["primary_reviewer"] != "machine" || explicit.MatchingPolicy["ranking"] != "manual" {
		t.Fatalf("normalizeStage overwrote explicit policies: %+v", explicit)
	}
}

func TestDefaultStagesCoverPlanExecuteReview(t *testing.T) {
	stages := defaultStages()
	if len(stages) != 3 {
		t.Fatalf("defaultStages = %d stages, want 3", len(stages))
	}
	if stages[0].Type != StagePlan || stages[1].Type != StageExecute || stages[2].Type != StageReview {
		t.Fatalf("stage order = [%s, %s, %s]", stages[0].Type, stages[1].Type, stages[2].Type)
	}
	if stages[2].AssigneeType != "internal" {
		t.Fatalf("review stage assignee = %q, want internal", stages[2].AssigneeType)
	}
	if stages[1].RequiredPermissionLevel != "L2" {
		t.Fatalf("execution stage permission = %q, want L2", stages[1].RequiredPermissionLevel)
	}
}

func TestEnumValidators(t *testing.T) {
	for _, status := range []WorkflowStatus{WorkflowActive, WorkflowPaused, WorkflowCompleted, WorkflowFailed} {
		if !isValidWorkflowStatus(status) {
			t.Errorf("isValidWorkflowStatus(%q) = false, want true", status)
		}
	}
	for _, status := range []WorkflowStatus{"", "running", "done", "ACTIVE"} {
		if isValidWorkflowStatus(status) {
			t.Errorf("isValidWorkflowStatus(%q) = true, want false", status)
		}
	}

	for _, role := range []string{"owner", "reviewer", "support", "observer"} {
		if !validTaskMatrixRole(role) {
			t.Errorf("validTaskMatrixRole(%q) = false, want true", role)
		}
	}
	if validTaskMatrixRole("manager") {
		t.Error(`validTaskMatrixRole("manager") = true, want false`)
	}

	for _, status := range []string{"active", "inactive", "archived"} {
		if !validTaskMatrixStatus(status) {
			t.Errorf("validTaskMatrixStatus(%q) = false, want true", status)
		}
	}
	if validTaskMatrixStatus("deleted") {
		t.Error(`validTaskMatrixStatus("deleted") = true, want false`)
	}
}

func TestEnsureTenantAccessRejectsCrossOrganizationResources(t *testing.T) {
	orgID := uuid.New()
	tenantCtx := context.WithValue(context.Background(), middleware.TenantContextKey, &middleware.TenantContext{OrganizationID: &orgID})

	if err := ensureTenantAccess(context.Background(), &orgID); err != nil {
		t.Fatalf("tenant-less context error = %v, want nil passthrough", err)
	}
	if err := ensureTenantAccess(tenantCtx, nil); err != nil {
		t.Fatalf("nil organization error = %v, want nil passthrough", err)
	}
	if err := ensureTenantAccess(tenantCtx, &orgID); err != nil {
		t.Fatalf("matching organization error = %v, want nil", err)
	}
	other := uuid.New()
	if err := ensureTenantAccess(tenantCtx, &other); !errors.Is(err, ErrValidation) {
		t.Fatalf("cross-organization error = %v, want ErrValidation", err)
	}
}

func TestCreateTemplateValidatesBeforePersistence(t *testing.T) {
	service := NewService(nil)

	if _, err := service.CreateTemplate(context.Background(), CreateWorkflowInput{}); !errors.Is(err, ErrValidation) {
		t.Fatalf("missing name error = %v, want ErrValidation", err)
	}

	orgID := uuid.New()
	tenantCtx := context.WithValue(context.Background(), middleware.TenantContextKey, &middleware.TenantContext{OrganizationID: &orgID})
	other := uuid.New()
	if _, err := service.CreateTemplate(tenantCtx, CreateWorkflowInput{Name: "Cross tenant", OrganizationID: &other}); !errors.Is(err, ErrValidation) {
		t.Fatalf("cross-organization template error = %v, want ErrValidation", err)
	}
}
