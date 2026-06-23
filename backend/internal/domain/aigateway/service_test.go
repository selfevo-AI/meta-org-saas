package aigateway

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/securitykernel"
)

type fakeGatewayRepo struct {
	target          ResolvedModel
	recorded        bool
	completed       bool
	failed          bool
	pricingResolved bool
	ledgerCount     int
	lastLedger      CreateUsageLedgerInput
	lastComplete    CompleteInvocationInput
}

func newFakeGatewayRepo() *fakeGatewayRepo {
	return &fakeGatewayRepo{target: ResolvedModel{
		ProviderID:     uuid.New(),
		ModelID:        uuid.New(),
		ProviderType:   ProviderOpenAI,
		Model:          "gpt-test",
		Price:          Price{InputPer1K: 0.01, OutputPer1K: 0.03},
		Currency:       "CNY",
		RateMultiplier: 1,
	}}
}

func TestServiceInvokeRecordsUsage(t *testing.T) {
	repo := newFakeGatewayRepo()
	adapter := fakeAdapter{resp: ProviderResponse{Content: "ok", Usage: TokenUsage{InputTokens: 10, OutputTokens: 20}}}
	svc := NewService(repo, AdapterRegistry{ProviderOpenAI: adapter})

	resp, err := svc.Invoke(context.Background(), InvokeInput{
		ProviderType: ProviderOpenAI,
		Model:        "gpt-test",
		Messages:     []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}
	if resp.Content != "ok" {
		t.Fatalf("content = %q, want ok", resp.Content)
	}
	if !repo.recorded {
		t.Fatalf("invocation was not recorded")
	}
	if !repo.completed {
		t.Fatalf("invocation was not completed")
	}
	if repo.ledgerCount != 1 {
		t.Fatalf("ledger count = %d, want 1", repo.ledgerCount)
	}
	if repo.lastLedger.Amount != 0.0007 {
		t.Fatalf("ledger amount = %.8f, want 0.0007", repo.lastLedger.Amount)
	}
}

func TestServiceInvokeRecordsFailedUsage(t *testing.T) {
	repo := newFakeGatewayRepo()
	adapter := fakeAdapter{err: errors.New("provider down")}
	svc := NewService(repo, AdapterRegistry{ProviderOpenAI: adapter})

	_, err := svc.Invoke(context.Background(), InvokeInput{
		ProviderType: ProviderOpenAI,
		Model:        "gpt-test",
		Messages:     []Message{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Fatalf("Invoke returned nil error")
	}
	if !repo.recorded {
		t.Fatalf("invocation was not recorded")
	}
	if !repo.failed {
		t.Fatalf("invocation was not marked failed")
	}
	if repo.ledgerCount != 1 {
		t.Fatalf("ledger count = %d, want 1", repo.ledgerCount)
	}
	if repo.lastLedger.Amount != 0 {
		t.Fatalf("failed ledger amount = %.8f, want 0", repo.lastLedger.Amount)
	}
}

func TestServiceInvokeRequiresSecurityKernelAuthorization(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	repo := newFakeGatewayRepo()
	kernel := &fakeSecurityKernel{decision: securitykernel.Decision{Allowed: false, Reason: "license_denied", DecisionType: "deny"}, err: securitykernel.ErrDenied}
	ctx := context.WithValue(context.Background(), middleware.TenantContextKey, &middleware.TenantContext{
		OrganizationID: &orgID,
		UserID:         userID,
		AuthorityTier:  "executor",
		EnabledModules: map[string]bool{"ai_gateway": true},
	})
	svc := NewService(repo, AdapterRegistry{ProviderOpenAI: fakeAdapter{}}, WithSecurityKernel(kernel))

	_, err := svc.Invoke(ctx, InvokeInput{
		ProviderType: ProviderOpenAI,
		Model:        "gpt-test",
		Messages:     []Message{{Role: "user", Content: "hi"}},
		Attribution:  Attribution{UserID: &userID},
	})

	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("Invoke error = %v, want ErrForbidden", err)
	}
	if repo.recorded {
		t.Fatalf("invocation was recorded after security denial")
	}
	if kernel.lastRequest.Resource.ResourceType != "model_provider_channel" || kernel.lastRequest.Resource.Action != "use" {
		t.Fatalf("security resource = %#v, want model_provider_channel use", kernel.lastRequest.Resource)
	}
}

func TestServiceInvokeUsesResolvedChannelKeyForProviderRequest(t *testing.T) {
	const channelKey = "sk-channel"
	channelID := uuid.New()
	repo := newFakeGatewayRepo()
	repo.target.APIKey = channelKey
	repo.target.ChannelID = &channelID

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("request path = %q, want /v1/chat/completions", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer "+channelKey {
			t.Fatalf("authorization header = %q, want channel key", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"req-1","choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":1,"completion_tokens":2}}`))
	}))
	defer server.Close()
	repo.target.BaseURL = server.URL

	svc := NewService(repo, nil)
	resp, err := svc.Invoke(context.Background(), InvokeInput{
		ProviderType: ProviderOpenAI,
		Model:        "gpt-test",
		Messages:     []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}
	if resp.Content != "ok" {
		t.Fatalf("content = %q, want ok", resp.Content)
	}
	if resp.ChannelID == nil || *resp.ChannelID != channelID {
		t.Fatalf("channel id = %v, want %s", resp.ChannelID, channelID)
	}
	if repo.lastLedger.ChannelID == nil || *repo.lastLedger.ChannelID != channelID {
		t.Fatalf("ledger channel id = %v, want %s", repo.lastLedger.ChannelID, channelID)
	}
}

func TestServiceEstimateCostUsesPricingTarget(t *testing.T) {
	repo := newFakeGatewayRepo()
	repo.target.RateMultiplier = 1.25
	svc := NewService(repo, nil)
	rate := 2.0

	result, err := svc.EstimateCost(context.Background(), EstimateCostInput{
		Model:          "gpt-test",
		Usage:          TokenUsage{InputTokens: 1000, OutputTokens: 500},
		RateMultiplier: &rate,
	})
	if err != nil {
		t.Fatalf("EstimateCost returned error: %v", err)
	}
	if !repo.pricingResolved {
		t.Fatalf("pricing target was not resolved")
	}
	if result.Model != "gpt-test" {
		t.Fatalf("model = %q, want gpt-test", result.Model)
	}
	if result.CostBreakdown.TotalCost != 0.025 {
		t.Fatalf("total cost = %.8f, want 0.025", result.CostBreakdown.TotalCost)
	}
	if result.CostBreakdown.ActualCost != 0.05 {
		t.Fatalf("actual cost = %.8f, want 0.05", result.CostBreakdown.ActualCost)
	}
	if result.Currency != "CNY" {
		t.Fatalf("currency = %q, want CNY", result.Currency)
	}
}

func (f *fakeGatewayRepo) ResolveInvocationTarget(context.Context, InvokeInput) (ResolvedModel, error) {
	return f.target, nil
}

func (f *fakeGatewayRepo) ResolvePricingTarget(context.Context, EstimateCostInput) (ResolvedModel, error) {
	f.pricingResolved = true
	return f.target, nil
}

func (f *fakeGatewayRepo) CreateInvocation(context.Context, CreateInvocationInput) (*Invocation, error) {
	f.recorded = true
	return &Invocation{ID: uuid.New(), ProviderID: f.target.ProviderID, ModelID: f.target.ModelID, Status: StatusStarted}, nil
}

func (f *fakeGatewayRepo) CompleteInvocation(_ context.Context, id uuid.UUID, input CompleteInvocationInput) error {
	f.completed = true
	f.lastComplete = input
	return nil
}

func (f *fakeGatewayRepo) FailInvocation(context.Context, uuid.UUID, FailInvocationInput) error {
	f.failed = true
	return nil
}

func (f *fakeGatewayRepo) CreateUsageLedger(_ context.Context, input CreateUsageLedgerInput) error {
	f.ledgerCount++
	f.lastLedger = input
	return nil
}

func (f *fakeGatewayRepo) ReleaseChannel(context.Context, *uuid.UUID, float64) error {
	return nil
}

func (f *fakeGatewayRepo) CreateProvider(context.Context, CreateProviderInput) (*ModelProvider, error) {
	return nil, errors.New("unexpected catalog call")
}

func (f *fakeGatewayRepo) ListProviders(context.Context, int) ([]ModelProvider, error) {
	return nil, errors.New("unexpected catalog call")
}

func (f *fakeGatewayRepo) UpdateProvider(context.Context, uuid.UUID, UpdateProviderInput) (*ModelProvider, error) {
	return nil, errors.New("unexpected catalog call")
}

func (f *fakeGatewayRepo) RotateProviderKey(context.Context, uuid.UUID, string) (*ModelProvider, error) {
	return nil, errors.New("unexpected catalog call")
}

func (f *fakeGatewayRepo) UpdateProviderTestResult(context.Context, uuid.UUID, string, string) error {
	return errors.New("unexpected catalog call")
}

func (f *fakeGatewayRepo) GetProviderSecret(context.Context, uuid.UUID) (ProviderSecret, error) {
	return ProviderSecret{}, errors.New("unexpected catalog call")
}

func (f *fakeGatewayRepo) CreateModel(context.Context, CreateModelInput) (*Model, error) {
	return nil, errors.New("unexpected catalog call")
}

func (f *fakeGatewayRepo) ListModels(context.Context, *uuid.UUID, int) ([]Model, error) {
	return nil, errors.New("unexpected catalog call")
}

func (f *fakeGatewayRepo) UpdateModel(context.Context, uuid.UUID, UpdateModelInput) (*Model, error) {
	return nil, errors.New("unexpected catalog call")
}

func (f *fakeGatewayRepo) ListInvocations(context.Context, int) ([]Invocation, error) {
	return nil, errors.New("unexpected catalog call")
}

func (f *fakeGatewayRepo) GetInvocation(context.Context, uuid.UUID) (*Invocation, error) {
	return nil, errors.New("unexpected catalog call")
}

func (f *fakeGatewayRepo) CostSummary(context.Context) (*GatewayCostSummary, error) {
	return nil, errors.New("unexpected catalog call")
}

func (f *fakeGatewayRepo) CreateChannel(context.Context, CreateChannelInput) (*ProviderChannel, error) {
	return nil, errors.New("unexpected catalog call")
}

func (f *fakeGatewayRepo) ListChannels(context.Context, *uuid.UUID, int) ([]ProviderChannel, error) {
	return nil, errors.New("unexpected catalog call")
}

func (f *fakeGatewayRepo) UpdateChannel(context.Context, uuid.UUID, UpdateChannelInput) (*ProviderChannel, error) {
	return nil, errors.New("unexpected catalog call")
}

func (f *fakeGatewayRepo) RotateChannelKey(context.Context, uuid.UUID, string) (*ProviderChannel, error) {
	return nil, errors.New("unexpected catalog call")
}

func (f *fakeGatewayRepo) GetChannelSecret(context.Context, uuid.UUID) (ChannelSecret, error) {
	return ChannelSecret{}, errors.New("unexpected catalog call")
}

func (f *fakeGatewayRepo) UpdateChannelTestResult(context.Context, uuid.UUID, string, string) error {
	return errors.New("unexpected catalog call")
}

func (f *fakeGatewayRepo) ListRoutingRules(context.Context, int) ([]RoutingRule, error) {
	return nil, errors.New("unexpected catalog call")
}

func (f *fakeGatewayRepo) CreateRoutingRule(context.Context, CreateRoutingRuleInput) (*RoutingRule, error) {
	return nil, errors.New("unexpected catalog call")
}

func (f *fakeGatewayRepo) UsageAnalysis(context.Context, UsageAnalysisFilter) (*UsageAnalysis, error) {
	return nil, errors.New("unexpected catalog call")
}

type fakeAdapter struct {
	resp ProviderResponse
	err  error
}

func (a fakeAdapter) Invoke(context.Context, ProviderRequest) (*ProviderResponse, error) {
	if a.err != nil {
		return nil, a.err
	}
	return &a.resp, nil
}

func (a fakeAdapter) Stream(context.Context, ProviderRequest) (<-chan StreamEvent, error) {
	ch := make(chan StreamEvent)
	close(ch)
	return ch, a.err
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
