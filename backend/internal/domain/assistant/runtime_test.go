package assistant

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/aigateway"
)

func TestNewAssistantHarnessFreezesRunScope(t *testing.T) {
	actorID := uuid.New()
	orgID := uuid.New()
	sessionID := uuid.New()
	contextPackageID := uuid.New()

	harness := NewAssistantHarness(AssistantHarnessInput{
		SessionID:        sessionID,
		ActorID:          actorID,
		ActorType:        "internal_human",
		OrganizationID:   &orgID,
		ModuleKey:        "project",
		TargetType:       "project",
		ContextPackageID: contextPackageID,
		Model:            "gpt-4o-mini",
		ProviderType:     "openai",
	})

	if harness.SessionID != sessionID {
		t.Fatalf("session id = %s, want %s", harness.SessionID, sessionID)
	}
	if harness.ActorID != actorID || harness.ActorType != "internal_human" {
		t.Fatalf("actor = %s/%s, want %s/internal_human", harness.ActorID, harness.ActorType, actorID)
	}
	if harness.OrganizationID == nil || *harness.OrganizationID != orgID {
		t.Fatalf("organization id = %v, want %s", harness.OrganizationID, orgID)
	}
	if harness.ModuleKey != "project" || harness.TargetType != "project" {
		t.Fatalf("scope = %s/%s, want project/project", harness.ModuleKey, harness.TargetType)
	}
	if harness.ContextPackageID != contextPackageID {
		t.Fatalf("context package = %s, want %s", harness.ContextPackageID, contextPackageID)
	}
}

func TestRuntimeEventNamesAreStable(t *testing.T) {
	if RuntimeEventContextBuilt != "context_built" {
		t.Fatalf("context event = %q, want context_built", RuntimeEventContextBuilt)
	}
	if RuntimeEventToolBlocked != "tool_blocked" {
		t.Fatalf("tool blocked event = %q, want tool_blocked", RuntimeEventToolBlocked)
	}
	if RuntimeErrorFinanceValidationFailed != "finance_validation_failed" {
		t.Fatalf("finance error = %q, want finance_validation_failed", RuntimeErrorFinanceValidationFailed)
	}
}

func TestMemoryEventSinkAddsStep(t *testing.T) {
	repo := &fakeRepository{}
	sink := NewMemoryEventSink(repo)
	session := &Session{ID: uuid.New(), ModuleKey: "project", WorkingMemory: map[string]any{}}

	event, err := sink.Emit(context.Background(), session, AssistantRuntimeEvent{
		Type:    RuntimeEventContextBuilt,
		Status:  StatusCompleted,
		Summary: "context built",
		Data:    map[string]any{"context_package_id": uuid.New().String()},
	})
	if err != nil {
		t.Fatalf("Emit returned error: %v", err)
	}
	if event.Type != RuntimeEventContextBuilt {
		t.Fatalf("event type = %s, want context_built", event.Type)
	}
	if repo.lastStep.StepType != StepContext {
		t.Fatalf("step type = %s, want context", repo.lastStep.StepType)
	}
}

func TestServiceRunDelegatesToRuntime(t *testing.T) {
	sessionID := uuid.New()
	actorID := uuid.New()
	runtime := &fakeAssistantRuntime{events: make(chan RunEvent, 1)}
	runtime.events <- RunEvent{Type: RuntimeEventRunCompleted, Done: true}
	close(runtime.events)
	svc := NewService(&fakeRepository{}, nil, nil, WithAssistantRuntime(runtime))

	events, err := svc.Run(context.Background(), sessionID, actorID, "internal_human", RunInput{Message: "summarize"})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	received := <-events
	if received.Type != RuntimeEventRunCompleted {
		t.Fatalf("event = %s, want run_completed", received.Type)
	}
	if runtime.request.SessionID != sessionID || runtime.request.ActorID != actorID {
		t.Fatalf("runtime request = %#v", runtime.request)
	}
}

func TestAssistantRuntimeBuildsContextBeforeLegacyRun(t *testing.T) {
	sessionID := uuid.New()
	actorID := uuid.New()
	repo := &fakeRepository{
		session: &Session{
			ID:            sessionID,
			ActorID:       actorID,
			ActorType:     "internal_human",
			ModuleKey:     "project",
			TargetType:    "project",
			WorkingMemory: map[string]any{},
		},
	}
	service := NewService(repo, &fakeAIInvoker{}, &fakeToolExecutor{})
	engine := &fakeContextEngine{pkg: &ContextPackage{ID: uuid.New(), AttentionCore: []ContextItem{{EntityKey: "project", FieldKey: "status", Value: "active"}}}}
	runtime := NewAssistantRuntime(service, engine, nil, NewMemoryEventSink(repo))

	events, err := runtime.Run(context.Background(), AssistantRunRequest{
		SessionID: sessionID,
		ActorID:   actorID,
		ActorType: "internal_human",
		Input:     RunInput{Message: "status"},
	})
	if err == nil {
		for range events {
		}
	}
	if engine.request.SessionID != sessionID || engine.request.ActorID != actorID {
		t.Fatalf("context request = %#v, want session and actor", engine.request)
	}
}

type fakeAssistantRuntime struct {
	request AssistantRunRequest
	events  chan RunEvent
}

func (f *fakeAssistantRuntime) Run(_ context.Context, request AssistantRunRequest) (<-chan RunEvent, error) {
	f.request = request
	return f.events, nil
}

func (f *fakeAssistantRuntime) Resume(_ context.Context, _ AssistantResumeRequest) (<-chan RunEvent, error) {
	return f.events, nil
}

type fakeContextEngine struct {
	request ContextRequest
	pkg     *ContextPackage
}

func (f *fakeContextEngine) BuildContextPackage(_ context.Context, request ContextRequest) (*ContextPackage, error) {
	f.request = request
	if f.pkg == nil {
		f.pkg = &ContextPackage{ID: uuid.New()}
	}
	return f.pkg, nil
}

type fakeAIInvoker struct{}

func (f *fakeAIInvoker) Invoke(context.Context, aigateway.InvokeInput) (*aigateway.InvokeOutput, error) {
	return &aigateway.InvokeOutput{
		InvocationID: uuid.New(),
		Content:      "ok",
		ProviderType: "openai",
		Model:        "gpt-4o-mini",
		CompletedAt:  time.Now(),
	}, nil
}
