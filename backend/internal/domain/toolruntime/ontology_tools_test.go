package toolruntime

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
)

func TestOntologyMutationCannotBypassReviewerApproval(t *testing.T) {
	orgID, actorID, reviewerID := uuid.New(), uuid.New(), uuid.New()
	ctx := context.WithValue(context.Background(), middleware.TenantContextKey, &middleware.TenantContext{OrganizationID: &orgID})
	repo := &fakeApprovalRepository{tool: ToolDefinition{
		ID: uuid.New(), Name: "ontology.action.execute", DefaultPolicy: PolicyAuto,
		ToolCategory: ToolCategoryExecutionOperation, ApprovalTierRequired: ApprovalTierExecutor, IsActive: true,
	}, tier: ApprovalTierExecutor}
	calls := 0
	service := NewService(repo, nil, map[string]ToolAdapter{
		"ontology.action.execute": func(_ context.Context, input ExecuteToolInput) (ToolResult, error) {
			calls++
			if input.Arguments["tool_execution_id"] != repo.execution.ID.String() || input.IdempotencyKey != "tool:"+repo.execution.ID.String() {
				t.Fatalf("untrusted execution identity: %#v", input)
			}
			return ToolResult{Data: map[string]any{"posted": true}}, nil
		},
	})
	result, err := service.ExecuteTool(ctx, ExecuteToolInput{
		ToolName: "ontology.action.execute", ActorID: actorID, ActorType: "internal_human", IdempotencyKey: "business-action",
		Arguments: map[string]any{"object_type": "payable_invoice", "key": "invoice", "action": "post", "tool_execution_id": "untrusted"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Approval == nil || calls != 0 || result.Execution.Policy != PolicyApprove {
		t.Fatalf("mutation ran without approval: %#v, calls=%d", result, calls)
	}
	if _, err := service.Approve(ctx, result.Approval.ID, &reviewerID, "review"); !errors.Is(err, ErrForbidden) || calls != 0 {
		t.Fatalf("executor downgraded reviewer gate: %v", err)
	}
	repo.tier = ApprovalTierReviewer
	if _, err := service.Approve(ctx, result.Approval.ID, &reviewerID, "review"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Approve(ctx, result.Approval.ID, &reviewerID, "retry"); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("adapter calls = %d, want 1", calls)
	}
}

func TestOntologyDenyWinsOverMandatoryApproval(t *testing.T) {
	for _, policy := range []string{PolicyAuto, PolicyNotify, PolicyApprove, PolicyDeny} {
		got := EffectivePolicyForExecution(ToolDefinition{Name: "ontology.action.execute", DefaultPolicy: policy}, ExecuteToolInput{}, GovernanceResult{Allowed: true})
		want := PolicyApprove
		if policy == PolicyDeny {
			want = PolicyDeny
		}
		if got != want {
			t.Fatalf("%s -> %s, want %s", policy, got, want)
		}
		if got := EffectivePolicyForExecution(ToolDefinition{Name: "ontology.action.execute", DefaultPolicy: policy}, ExecuteToolInput{RequireApproval: true}, GovernanceResult{Decision: "deny"}); got != PolicyDeny {
			t.Fatal("governance denial bypassed")
		}
	}
}
