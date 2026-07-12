package businessai

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/aigateway"
)

func TestAnalyzeSupportsEveryBusinessStage(t *testing.T) {
	stages := []string{StagePlan, StageDo, StageChange, StageAccept, StageLearn}
	for _, stage := range stages {
		t.Run(stage, func(t *testing.T) {
			repo := &fakeRunRepository{}
			invoker := &fakeAIInvoker{content: validAnalysisJSON(t)}
			svc := NewService(repo, invoker, Config{ProviderType: "openai", Model: "test-model"})
			orgID, projectID := uuid.New(), uuid.New()
			run, err := svc.Analyze(context.Background(), AnalyzeInput{
				OrganizationID: orgID, ProjectID: projectID, Stage: stage,
				Context: map[string]any{"project": map[string]any{"status": "active"}},
			})
			if err != nil {
				t.Fatalf("Analyze() error = %v", err)
			}
			if run.Status != StatusCompleted || run.Analysis == nil || run.Analysis.Summary == "" {
				t.Fatalf("run = %#v, want completed analysis", run)
			}
			if !run.Analysis.Proposal.RequiresApproval {
				t.Fatal("tool-backed proposal must require approval")
			}
			if invoker.input.Attribution.OrganizationID == nil || *invoker.input.Attribution.OrganizationID != orgID {
				t.Fatalf("organization attribution = %v", invoker.input.Attribution.OrganizationID)
			}
			if invoker.input.Metadata["business_stage"] != stage {
				t.Fatalf("business_stage metadata = %v", invoker.input.Metadata["business_stage"])
			}
		})
	}
}

func TestAnalyzeRejectsInvalidModelOutputAndPersistsFailure(t *testing.T) {
	repo := &fakeRunRepository{}
	svc := NewService(repo, &fakeAIInvoker{content: `{"summary":"missing contract"}`}, Config{
		ProviderType: "openai", Model: "test-model",
	})
	_, err := svc.Analyze(context.Background(), AnalyzeInput{
		OrganizationID: uuid.New(), ProjectID: uuid.New(), Stage: StagePlan,
	})
	if !errors.Is(err, ErrInvalidOutput) {
		t.Fatalf("Analyze() error = %v, want ErrInvalidOutput", err)
	}
	if repo.failedMessage == "" {
		t.Fatal("invalid model output was not persisted as a failed run")
	}
}

func TestParseAnalysisRejectsTrailingContent(t *testing.T) {
	_, err := parseAnalysis(validAnalysisJSON(t) + " trailing")
	if !errors.Is(err, ErrInvalidOutput) {
		t.Fatalf("parseAnalysis() error = %v, want ErrInvalidOutput", err)
	}
}

func TestAttributionActorSeparatesHumanAndAgentForeignKeys(t *testing.T) {
	id := uuid.New()
	userID, agentID := attributionActor(&id, "internal_agent")
	if userID != nil || agentID == nil || *agentID != id {
		t.Fatalf("agent attribution = user %v agent %v", userID, agentID)
	}
	userID, agentID = attributionActor(&id, "human")
	if userID == nil || *userID != id || agentID != nil {
		t.Fatalf("human attribution = user %v agent %v", userID, agentID)
	}
}

func validAnalysisJSON(t *testing.T) string {
	t.Helper()
	data, err := json.Marshal(Analysis{
		Summary:         "Stage analysis",
		Findings:        []Finding{{Title: "Budget", Evidence: "project.cost_summary", Impact: "medium"}},
		Recommendations: []Recommendation{{Title: "Review", Rationale: "variance", Priority: "high"}},
		Risks:           []Risk{{Title: "Delay", Probability: "medium", Impact: "delivery", Mitigation: "replan"}},
		Proposal:        Proposal{Action: "Review plan", ToolName: "project.bind_workflow", Arguments: map[string]any{}, RequiresApproval: false},
		Confidence:      0.82, EvidenceRefs: []string{"project.cost_summary"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

type fakeAIInvoker struct {
	input   aigateway.InvokeInput
	content string
}

func (f *fakeAIInvoker) Invoke(_ context.Context, input aigateway.InvokeInput) (*aigateway.InvokeOutput, error) {
	f.input = input
	return &aigateway.InvokeOutput{
		InvocationID: uuid.New(), Content: f.content, Model: "resolved-model", Currency: "CNY",
		CostAmount: 0.12, Usage: aigateway.TokenUsage{InputTokens: 100, OutputTokens: 80}, CompletedAt: time.Now(),
	}, nil
}

type fakeRunRepository struct {
	run           Run
	failedMessage string
}

func (f *fakeRunRepository) CreateRun(_ context.Context, input AnalyzeInput) (*Run, error) {
	f.run = Run{ID: uuid.New(), OrganizationID: input.OrganizationID, ProjectID: input.ProjectID, Stage: input.Stage, Status: StatusRunning}
	return &f.run, nil
}

func (f *fakeRunRepository) CompleteRun(_ context.Context, _ uuid.UUID, input CompleteRunInput) (*Run, error) {
	f.run.Status = StatusCompleted
	f.run.InvocationID = &input.InvocationID
	f.run.ResolvedModel = input.ResolvedModel
	f.run.Analysis = &input.Analysis
	f.run.CostAmount = input.CostAmount
	return &f.run, nil
}

func (f *fakeRunRepository) FailRun(_ context.Context, _ uuid.UUID, message string) error {
	f.failedMessage = message
	f.run.Status = StatusFailed
	return nil
}

func (f *fakeRunRepository) ListRuns(context.Context, uuid.UUID, uuid.UUID, int) ([]Run, error) {
	return []Run{f.run}, nil
}
