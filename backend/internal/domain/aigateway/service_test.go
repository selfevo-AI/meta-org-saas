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
	accessToken     AccessTokenContext
	reserved        bool
	settled         bool
	refunded        bool
	reservation     BalanceReservation
	lastReserve     ReserveBalanceInput
	lastSettle      SettleBalanceInput
	lastTokenStore  CreateAccessTokenStoreInput
	lastAdjustment  AdjustGatewayBalanceInput
	catalogModels   []Model
	abilities       []ModelChannelAbility
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

func TestServiceInvokeWithAccessTokenReservesAndSettlesBalance(t *testing.T) {
	repo := newFakeGatewayRepo()
	orgID := uuid.New()
	tokenID := uuid.New()
	groupID := uuid.New()
	repo.accessToken = AccessTokenContext{
		ID:             tokenID,
		OrganizationID: orgID,
		ModelGroupID:   &groupID,
		AllowedModels:  []string{"gpt-*"},
		Status:         "active",
	}
	repo.reservation = BalanceReservation{ID: uuid.New(), ReservedAmount: 0.0013, Currency: "CNY"}
	adapter := fakeAdapter{resp: ProviderResponse{Content: "ok", Usage: TokenUsage{InputTokens: 10, OutputTokens: 20}}}
	svc := NewService(repo, AdapterRegistry{ProviderOpenAI: adapter})

	resp, err := svc.InvokeWithAccessToken(context.Background(), "ak-test", InvokeInput{
		ProviderType: ProviderOpenAI,
		Model:        "gpt-test",
		Messages:     []Message{{Role: "user", Content: "hi"}},
		MaxTokens:    32,
	})
	if err != nil {
		t.Fatalf("InvokeWithAccessToken returned error: %v", err)
	}
	if resp.Content != "ok" {
		t.Fatalf("content = %q, want ok", resp.Content)
	}
	if !repo.reserved {
		t.Fatalf("balance was not reserved")
	}
	if !repo.settled {
		t.Fatalf("balance was not settled")
	}
	if repo.refunded {
		t.Fatalf("balance was refunded for successful invocation")
	}
	if repo.lastReserve.AccessTokenID != tokenID || repo.lastReserve.OrganizationID != orgID {
		t.Fatalf("reserve attribution = %#v, want token/org ids", repo.lastReserve)
	}
	if repo.lastSettle.ActualAmount != repo.lastLedger.Amount {
		t.Fatalf("settled amount = %.8f, want ledger amount %.8f", repo.lastSettle.ActualAmount, repo.lastLedger.Amount)
	}
	if repo.lastLedger.Metadata["access_token_id"] != tokenID.String() {
		t.Fatalf("ledger access_token_id metadata = %#v, want %s", repo.lastLedger.Metadata["access_token_id"], tokenID)
	}
}

func TestServiceInvokeWithAccessTokenRejectsDisallowedModelBeforeReserve(t *testing.T) {
	repo := newFakeGatewayRepo()
	repo.accessToken = AccessTokenContext{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		AllowedModels:  []string{"claude-*"},
		Status:         "active",
	}
	svc := NewService(repo, AdapterRegistry{ProviderOpenAI: fakeAdapter{}})

	_, err := svc.InvokeWithAccessToken(context.Background(), "ak-test", InvokeInput{
		ProviderType: ProviderOpenAI,
		Model:        "gpt-test",
		Messages:     []Message{{Role: "user", Content: "hi"}},
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("InvokeWithAccessToken error = %v, want ErrForbidden", err)
	}
	if repo.reserved {
		t.Fatalf("balance was reserved for disallowed model")
	}
	if repo.recorded {
		t.Fatalf("invocation was recorded for disallowed model")
	}
}

func TestServiceAuthenticateAccessTokenRejectsInactiveToken(t *testing.T) {
	repo := newFakeGatewayRepo()
	repo.accessToken = AccessTokenContext{ID: uuid.New(), OrganizationID: uuid.New(), Status: "revoked"}
	service := NewService(repo, nil)

	_, err := service.AuthenticateAccessToken(context.Background(), "ak-revoked")
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("AuthenticateAccessToken() error = %v, want ErrForbidden", err)
	}
}

func TestServiceInvokeWithAccessTokenRefundsReservationWhenProviderFails(t *testing.T) {
	repo := newFakeGatewayRepo()
	repo.accessToken = AccessTokenContext{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		AllowedModels:  []string{"gpt-test"},
		Status:         "active",
	}
	repo.reservation = BalanceReservation{ID: uuid.New(), ReservedAmount: 0.0013, Currency: "CNY"}
	svc := NewService(repo, AdapterRegistry{ProviderOpenAI: fakeAdapter{err: errors.New("provider down")}})

	_, err := svc.InvokeWithAccessToken(context.Background(), "ak-test", InvokeInput{
		ProviderType: ProviderOpenAI,
		Model:        "gpt-test",
		Messages:     []Message{{Role: "user", Content: "hi"}},
		MaxTokens:    32,
	})
	if err == nil {
		t.Fatalf("InvokeWithAccessToken returned nil error")
	}
	if !repo.reserved {
		t.Fatalf("balance was not reserved")
	}
	if !repo.refunded {
		t.Fatalf("balance was not refunded after provider failure")
	}
	if repo.settled {
		t.Fatalf("balance was settled after provider failure")
	}
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

func TestServiceCreateAccessTokenGeneratesHashAndOneTimePlainToken(t *testing.T) {
	repo := newFakeGatewayRepo()
	orgID := uuid.New()
	svc := NewService(repo, nil)

	result, err := svc.CreateAccessToken(context.Background(), CreateAccessTokenInput{
		OrganizationID: orgID,
		Name:           "ERP integration",
		AllowedModels:  []string{"gpt-*"},
	})
	if err != nil {
		t.Fatalf("CreateAccessToken returned error: %v", err)
	}
	if result.PlainToken == "" {
		t.Fatalf("PlainToken is empty")
	}
	if result.TokenHash == "" || result.TokenHash == result.PlainToken {
		t.Fatalf("TokenHash = %q, PlainToken = %q, want non-empty hash distinct from token", result.TokenHash, result.PlainToken)
	}
	if repo.lastTokenStore.TokenHash != result.TokenHash {
		t.Fatalf("stored token hash = %q, want %q", repo.lastTokenStore.TokenHash, result.TokenHash)
	}
	if repo.lastTokenStore.OrganizationID != orgID {
		t.Fatalf("stored organization id = %s, want %s", repo.lastTokenStore.OrganizationID, orgID)
	}
	if repo.lastTokenStore.MaskedToken == "" || repo.lastTokenStore.MaskedToken == result.PlainToken {
		t.Fatalf("masked token = %q, want masked value", repo.lastTokenStore.MaskedToken)
	}
}

func TestServiceAdjustGatewayBalanceRequiresOrganizationAndRecordsAdjustment(t *testing.T) {
	repo := newFakeGatewayRepo()
	orgID := uuid.New()
	svc := NewService(repo, nil)

	result, err := svc.AdjustGatewayBalance(context.Background(), AdjustGatewayBalanceInput{
		OrganizationID: orgID,
		Amount:         120.5,
		Currency:       "CNY",
		Reason:         "manual_top_up",
	})
	if err != nil {
		t.Fatalf("AdjustGatewayBalance returned error: %v", err)
	}
	if result.BalanceAmount != 120.5 {
		t.Fatalf("balance amount = %.4f, want 120.5", result.BalanceAmount)
	}
	if repo.lastAdjustment.OrganizationID != orgID || repo.lastAdjustment.Reason != "manual_top_up" {
		t.Fatalf("adjustment input = %#v, want org and reason", repo.lastAdjustment)
	}
}

func TestServiceAdjustGatewayBalanceRejectsMissingReason(t *testing.T) {
	svc := NewService(newFakeGatewayRepo(), nil)

	_, err := svc.AdjustGatewayBalance(context.Background(), AdjustGatewayBalanceInput{
		OrganizationID: uuid.New(),
		Amount:         10,
	})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("AdjustGatewayBalance error = %v, want ErrValidation", err)
	}
}

func TestServiceAdapterCatalogIncludesOpenAICompatibleMatrix(t *testing.T) {
	svc := NewService(newFakeGatewayRepo(), nil)

	adapters := svc.AdapterCatalog()

	if len(adapters) < 4 {
		t.Fatalf("adapter count = %d, want OpenAI-compatible matrix and native adapters", len(adapters))
	}
	if !adapterCatalogHas(adapters, "deepseek", "openai_compatible") {
		t.Fatalf("adapter catalog = %#v, want deepseek as openai_compatible", adapters)
	}
	if !adapterCatalogHas(adapters, "anthropic", "native") {
		t.Fatalf("adapter catalog = %#v, want anthropic native adapter", adapters)
	}
}

func adapterCatalogHas(items []AdapterDescriptor, key string, mode string) bool {
	for _, item := range items {
		if item.AdapterKey == key && item.AdapterMode == mode {
			return true
		}
	}
	return false
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

func (f *fakeGatewayRepo) AuthenticateAccessToken(context.Context, string) (AccessTokenContext, error) {
	if f.accessToken.ID == uuid.Nil {
		return AccessTokenContext{}, ErrForbidden
	}
	return f.accessToken, nil
}

func (f *fakeGatewayRepo) ReserveAccessTokenBalance(_ context.Context, input ReserveBalanceInput) (BalanceReservation, error) {
	f.reserved = true
	f.lastReserve = input
	if f.reservation.ID == uuid.Nil {
		f.reservation = BalanceReservation{ID: uuid.New(), ReservedAmount: input.EstimatedAmount, Currency: input.Currency}
	}
	return f.reservation, nil
}

func (f *fakeGatewayRepo) SettleAccessTokenBalance(_ context.Context, input SettleBalanceInput) error {
	f.settled = true
	f.lastSettle = input
	return nil
}

func (f *fakeGatewayRepo) RefundAccessTokenBalance(context.Context, uuid.UUID, string) error {
	f.refunded = true
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
	if f.catalogModels == nil {
		return nil, errors.New("unexpected catalog call")
	}
	return f.catalogModels, nil
}

func (f *fakeGatewayRepo) ListActiveModels(context.Context) ([]Model, error) {
	if f.catalogModels == nil {
		return nil, errors.New("unexpected catalog call")
	}
	return f.catalogModels, nil
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

func (f *fakeGatewayRepo) CreateAccessToken(_ context.Context, input CreateAccessTokenStoreInput) (*AccessToken, error) {
	f.lastTokenStore = input
	return &AccessToken{
		ID:             uuid.New(),
		OrganizationID: input.OrganizationID,
		ModelGroupID:   input.ModelGroupID,
		Name:           input.Name,
		TokenHash:      input.TokenHash,
		MaskedToken:    input.MaskedToken,
		Status:         "active",
		AllowedModels:  input.AllowedModels,
	}, nil
}

func (f *fakeGatewayRepo) ListAccessTokens(context.Context, *uuid.UUID, int) ([]AccessToken, error) {
	return nil, errors.New("unexpected catalog call")
}

func (f *fakeGatewayRepo) ListModelGroups(context.Context, *uuid.UUID, int) ([]ModelGroup, error) {
	return nil, errors.New("unexpected catalog call")
}

func (f *fakeGatewayRepo) CreateModelGroup(context.Context, CreateModelGroupInput) (*ModelGroup, error) {
	return nil, errors.New("unexpected catalog call")
}

func (f *fakeGatewayRepo) CreateModelChannelAbility(context.Context, CreateModelChannelAbilityInput) (*ModelChannelAbility, error) {
	return nil, errors.New("unexpected catalog call")
}

func (f *fakeGatewayRepo) ListModelChannelAbilities(context.Context, *uuid.UUID, int) ([]ModelChannelAbility, error) {
	if f.abilities == nil {
		return nil, errors.New("unexpected catalog call")
	}
	return f.abilities, nil
}

func (f *fakeGatewayRepo) GetGatewayBalance(context.Context, uuid.UUID, string) (*GatewayBalance, error) {
	return nil, errors.New("unexpected catalog call")
}

func (f *fakeGatewayRepo) ListBalanceTransactions(context.Context, uuid.UUID, int) ([]BalanceTransaction, error) {
	return nil, errors.New("unexpected catalog call")
}

func (f *fakeGatewayRepo) AdjustGatewayBalance(_ context.Context, input AdjustGatewayBalanceInput) (*GatewayBalance, error) {
	f.lastAdjustment = input
	return &GatewayBalance{
		ID:             uuid.New(),
		OrganizationID: input.OrganizationID,
		BalanceAmount:  input.Amount,
		Currency:       currencyOrDefault(input.Currency),
		Metadata:       map[string]any{},
	}, nil
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
