package aigateway

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org/backend/internal/domain/costing"
	"github.com/selfevo-AI/meta-org/backend/internal/domain/observability"
	"github.com/selfevo-AI/meta-org/backend/internal/pkg/middleware"
	"github.com/selfevo-AI/meta-org/backend/internal/pkg/securitykernel"
)

var (
	ErrValidation  = errors.New("validation error")
	ErrNotFound    = errors.New("not found")
	ErrForbidden   = errors.New("forbidden")
	ErrUnavailable = errors.New("unavailable")
)

type AdapterRegistry map[string]ProviderAdapter

type ObservabilityRecorder interface {
	StartTrace(ctx context.Context, workflowID *uuid.UUID, metadata map[string]any) (*observability.Trace, error)
	RecordSpan(ctx context.Context, input observability.RecordSpanInput) (*observability.Span, error)
	RecordMetric(ctx context.Context, input observability.RecordMetricInput) (*observability.Metric, error)
	CompleteTrace(ctx context.Context, id uuid.UUID, status string) error
}

type InvocationRepository interface {
	ResolveInvocationTarget(ctx context.Context, input InvokeInput) (ResolvedModel, error)
	CreateInvocation(ctx context.Context, input CreateInvocationInput) (*Invocation, error)
	CompleteInvocation(ctx context.Context, id uuid.UUID, input CompleteInvocationInput) error
	FailInvocation(ctx context.Context, id uuid.UUID, input FailInvocationInput) error
	CreateUsageLedger(ctx context.Context, input CreateUsageLedgerInput) error
	ReleaseChannel(ctx context.Context, id *uuid.UUID, amount float64) error
}

type CatalogRepository interface {
	CreateProvider(ctx context.Context, input CreateProviderInput) (*ModelProvider, error)
	ListProviders(ctx context.Context, limit int) ([]ModelProvider, error)
	UpdateProvider(ctx context.Context, id uuid.UUID, input UpdateProviderInput) (*ModelProvider, error)
	RotateProviderKey(ctx context.Context, id uuid.UUID, apiKey string) (*ModelProvider, error)
	UpdateProviderTestResult(ctx context.Context, id uuid.UUID, status string, message string) error
	GetProviderSecret(ctx context.Context, id uuid.UUID) (ProviderSecret, error)
	CreateModel(ctx context.Context, input CreateModelInput) (*Model, error)
	ListModels(ctx context.Context, providerID *uuid.UUID, limit int) ([]Model, error)
	UpdateModel(ctx context.Context, id uuid.UUID, input UpdateModelInput) (*Model, error)
	ListInvocations(ctx context.Context, limit int) ([]Invocation, error)
	GetInvocation(ctx context.Context, id uuid.UUID) (*Invocation, error)
	CostSummary(ctx context.Context) (*GatewayCostSummary, error)
	CreateChannel(ctx context.Context, input CreateChannelInput) (*ProviderChannel, error)
	ListChannels(ctx context.Context, providerID *uuid.UUID, limit int) ([]ProviderChannel, error)
	UpdateChannel(ctx context.Context, id uuid.UUID, input UpdateChannelInput) (*ProviderChannel, error)
	RotateChannelKey(ctx context.Context, id uuid.UUID, apiKey string) (*ProviderChannel, error)
	GetChannelSecret(ctx context.Context, id uuid.UUID) (ChannelSecret, error)
	UpdateChannelTestResult(ctx context.Context, id uuid.UUID, status string, message string) error
	ListRoutingRules(ctx context.Context, limit int) ([]RoutingRule, error)
	CreateRoutingRule(ctx context.Context, input CreateRoutingRuleInput) (*RoutingRule, error)
	UsageAnalysis(ctx context.Context, filter UsageAnalysisFilter) (*UsageAnalysis, error)
	ResolvePricingTarget(ctx context.Context, input EstimateCostInput) (ResolvedModel, error)
}

type Service struct {
	repo           InvocationRepository
	catalog        CatalogRepository
	adapters       AdapterRegistry
	client         *http.Client
	observability  ObservabilityRecorder
	costRecorder   CostRecorder
	securityKernel securitykernel.Client
}

type ServiceOption func(*Service)

type CostRecorder interface {
	RecordActual(ctx context.Context, input costing.CreateLedgerEntryInput) (*costing.CostLedgerEntry, error)
}

func WithObservability(recorder ObservabilityRecorder) ServiceOption {
	return func(s *Service) {
		s.observability = recorder
	}
}

func WithCostRecorder(recorder CostRecorder) ServiceOption {
	return func(s *Service) {
		s.costRecorder = recorder
	}
}

func WithSecurityKernel(client securitykernel.Client) ServiceOption {
	return func(s *Service) {
		s.securityKernel = client
	}
}

func NewService(repo InvocationRepository, adapters AdapterRegistry, opts ...ServiceOption) *Service {
	catalog, _ := repo.(CatalogRepository)
	if adapters == nil {
		adapters = AdapterRegistry{}
	}
	s := &Service{
		repo:     repo,
		catalog:  catalog,
		adapters: adapters,
		client:   &http.Client{Timeout: 60 * time.Second},
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

type ResolvedModel struct {
	ProviderID          uuid.UUID
	ModelID             uuid.UUID
	ChannelID           *uuid.UUID
	ChannelName         string
	ProviderType        string
	BaseURL             string
	APIKey              string
	Model               string
	RequestedModel      string
	UpstreamModel       string
	ModelMappingChain   string
	PriceVersionID      *uuid.UUID
	Price               Price
	Currency            string
	MaxOutputTokens     int
	ProviderRequestHint string
	RateMultiplier      float64
}

type InvokeInput struct {
	ProviderID         *uuid.UUID       `json:"provider_id,omitempty"`
	PreferredChannelID *uuid.UUID       `json:"preferred_channel_id,omitempty"`
	ProviderType       string           `json:"provider_type,omitempty"`
	ModelID            *uuid.UUID       `json:"model_id,omitempty"`
	Model              string           `json:"model"`
	Messages           []Message        `json:"messages"`
	Temperature        *float64         `json:"temperature,omitempty"`
	MaxTokens          int              `json:"max_tokens,omitempty"`
	ServiceTier        string           `json:"service_tier,omitempty"`
	ReasoningEffort    string           `json:"reasoning_effort,omitempty"`
	Tools              []ToolDefinition `json:"tools,omitempty"`
	Attribution        Attribution      `json:"attribution"`
	Metadata           map[string]any   `json:"metadata,omitempty"`
}

type InvokeOutput struct {
	InvocationID      uuid.UUID     `json:"invocation_id"`
	ProviderRequestID string        `json:"provider_request_id,omitempty"`
	Content           string        `json:"content"`
	Usage             TokenUsage    `json:"usage"`
	CostAmount        float64       `json:"cost_amount"`
	CostBreakdown     CostBreakdown `json:"cost_breakdown"`
	Currency          string        `json:"currency"`
	ToolCalls         []ToolCall    `json:"tool_calls,omitempty"`
	CompletedAt       time.Time     `json:"completed_at"`
	ProviderType      string        `json:"provider_type"`
	Model             string        `json:"model"`
	RequestedModel    string        `json:"requested_model"`
	ChannelID         *uuid.UUID    `json:"channel_id,omitempty"`
}

type StreamResult struct {
	InvocationID uuid.UUID
	Events       <-chan StreamEvent
}

type CreateInvocationInput struct {
	ProviderID            uuid.UUID
	ModelID               uuid.UUID
	ChannelID             *uuid.UUID
	Mode                  string
	Status                string
	Attribution           Attribution
	RequestedModel        string
	UpstreamModel         string
	ModelMappingChain     string
	ServiceTier           string
	ReasoningEffort       string
	RequestHash           string
	EstimatedInputTokens  int
	EstimatedOutputTokens int
	Metadata              map[string]any
}

type CompleteInvocationInput struct {
	ProviderRequestID string
	Usage             TokenUsage
	CostAmount        float64
	CostBreakdown     CostBreakdown
	Currency          string
	DurationMS        int
}

type FailInvocationInput struct {
	ErrorType  string
	Message    string
	DurationMS int
	Cancelled  bool
}

type CreateUsageLedgerInput struct {
	InvocationID        uuid.UUID
	ChannelID           *uuid.UUID
	ModelPriceVersionID *uuid.UUID
	LedgerType          string
	Amount              float64
	ActualAmount        float64
	Currency            string
	Usage               TokenUsage
	CostBreakdown       CostBreakdown
	ServiceTier         string
	ReasoningEffort     string
	RequestedModel      string
	UpstreamModel       string
	Metadata            map[string]any
	Reason              string
}

type CreateProviderInput struct {
	Name         string         `json:"name"`
	ProviderType string         `json:"provider_type"`
	BaseURL      string         `json:"base_url,omitempty"`
	APIKey       string         `json:"api_key"`
	Status       string         `json:"status,omitempty"`
	TimeoutMS    int            `json:"timeout_ms,omitempty"`
	RetryCount   int            `json:"retry_count,omitempty"`
	RiskLevel    string         `json:"risk_level,omitempty"`
	Tags         []string       `json:"tags,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type UpdateProviderInput struct {
	Name       *string        `json:"name,omitempty"`
	BaseURL    *string        `json:"base_url,omitempty"`
	Status     *string        `json:"status,omitempty"`
	TimeoutMS  *int           `json:"timeout_ms,omitempty"`
	RetryCount *int           `json:"retry_count,omitempty"`
	RiskLevel  *string        `json:"risk_level,omitempty"`
	Tags       []string       `json:"tags,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type ProviderSecret struct {
	ID           uuid.UUID
	Name         string
	ProviderType string
	BaseURL      string
	APIKey       string
	Status       string
}

type RotateProviderKeyInput struct {
	APIKey string `json:"api_key"`
}

type TestProviderInput struct {
	Model string `json:"model,omitempty"`
}

type CreateModelInput struct {
	ProviderID                  uuid.UUID      `json:"provider_id"`
	ModelKey                    string         `json:"model_key"`
	DisplayName                 string         `json:"display_name,omitempty"`
	ContextWindow               int            `json:"context_window,omitempty"`
	MaxOutputTokens             int            `json:"max_output_tokens,omitempty"`
	Capabilities                []string       `json:"capabilities,omitempty"`
	Status                      string         `json:"status,omitempty"`
	Metadata                    map[string]any `json:"metadata,omitempty"`
	InputPricePer1K             float64        `json:"input_price_per_1k,omitempty"`
	OutputPricePer1K            float64        `json:"output_price_per_1k,omitempty"`
	CacheCreationPricePer1K     float64        `json:"cache_creation_price_per_1k,omitempty"`
	CacheReadPricePer1K         float64        `json:"cache_read_price_per_1k,omitempty"`
	CacheCreation5mPricePer1K   float64        `json:"cache_creation_5m_price_per_1k,omitempty"`
	CacheCreation1hPricePer1K   float64        `json:"cache_creation_1h_price_per_1k,omitempty"`
	ImageOutputPricePer1K       float64        `json:"image_output_price_per_1k,omitempty"`
	PriorityInputPricePer1K     float64        `json:"priority_input_price_per_1k,omitempty"`
	PriorityOutputPricePer1K    float64        `json:"priority_output_price_per_1k,omitempty"`
	PriorityCacheReadPricePer1K float64        `json:"priority_cache_read_price_per_1k,omitempty"`
	LongContextThreshold        int            `json:"long_context_threshold,omitempty"`
	LongContextInputMultiplier  float64        `json:"long_context_input_multiplier,omitempty"`
	LongContextOutputMultiplier float64        `json:"long_context_output_multiplier,omitempty"`
	BillingMode                 string         `json:"billing_mode,omitempty"`
	PricingSource               string         `json:"pricing_source,omitempty"`
	Currency                    string         `json:"currency,omitempty"`
}

type UpdateModelInput struct {
	DisplayName                 *string        `json:"display_name,omitempty"`
	ContextWindow               *int           `json:"context_window,omitempty"`
	MaxOutputTokens             *int           `json:"max_output_tokens,omitempty"`
	Capabilities                []string       `json:"capabilities,omitempty"`
	Status                      *string        `json:"status,omitempty"`
	Metadata                    map[string]any `json:"metadata,omitempty"`
	InputPricePer1K             *float64       `json:"input_price_per_1k,omitempty"`
	OutputPricePer1K            *float64       `json:"output_price_per_1k,omitempty"`
	CacheCreationPricePer1K     *float64       `json:"cache_creation_price_per_1k,omitempty"`
	CacheReadPricePer1K         *float64       `json:"cache_read_price_per_1k,omitempty"`
	CacheCreation5mPricePer1K   *float64       `json:"cache_creation_5m_price_per_1k,omitempty"`
	CacheCreation1hPricePer1K   *float64       `json:"cache_creation_1h_price_per_1k,omitempty"`
	ImageOutputPricePer1K       *float64       `json:"image_output_price_per_1k,omitempty"`
	PriorityInputPricePer1K     *float64       `json:"priority_input_price_per_1k,omitempty"`
	PriorityOutputPricePer1K    *float64       `json:"priority_output_price_per_1k,omitempty"`
	PriorityCacheReadPricePer1K *float64       `json:"priority_cache_read_price_per_1k,omitempty"`
	LongContextThreshold        *int           `json:"long_context_threshold,omitempty"`
	LongContextInputMultiplier  *float64       `json:"long_context_input_multiplier,omitempty"`
	LongContextOutputMultiplier *float64       `json:"long_context_output_multiplier,omitempty"`
	BillingMode                 *string        `json:"billing_mode,omitempty"`
	PricingSource               *string        `json:"pricing_source,omitempty"`
	Currency                    *string        `json:"currency,omitempty"`
}

type GatewayCostSummary struct {
	Total      float64            `json:"total"`
	Unexported float64            `json:"unexported"`
	Currency   string             `json:"currency"`
	ByProvider map[string]float64 `json:"by_provider"`
	ByChannel  map[string]float64 `json:"by_channel,omitempty"`
}

type CreateChannelInput struct {
	ProviderID             uuid.UUID         `json:"provider_id"`
	Name                   string            `json:"name"`
	BaseURL                string            `json:"base_url,omitempty"`
	APIKey                 string            `json:"api_key"`
	OwnerType              string            `json:"owner_type,omitempty"`
	UserID                 *uuid.UUID        `json:"user_id,omitempty"`
	AgentID                *uuid.UUID        `json:"agent_id,omitempty"`
	Status                 string            `json:"status,omitempty"`
	Priority               int               `json:"priority,omitempty"`
	ConcurrencyLimit       int               `json:"concurrency_limit,omitempty"`
	LoadFactor             int               `json:"load_factor,omitempty"`
	RateMultiplier         *float64          `json:"rate_multiplier,omitempty"`
	QuotaAmount            float64           `json:"quota_amount,omitempty"`
	QuotaCurrency          string            `json:"quota_currency,omitempty"`
	SupportedModelPatterns []string          `json:"supported_model_patterns,omitempty"`
	ModelMapping           map[string]string `json:"model_mapping,omitempty"`
	Metadata               map[string]any    `json:"metadata,omitempty"`
}

type UpdateChannelInput struct {
	Name                   *string           `json:"name,omitempty"`
	BaseURL                *string           `json:"base_url,omitempty"`
	OwnerType              *string           `json:"owner_type,omitempty"`
	UserID                 *uuid.UUID        `json:"user_id,omitempty"`
	AgentID                *uuid.UUID        `json:"agent_id,omitempty"`
	Status                 *string           `json:"status,omitempty"`
	Priority               *int              `json:"priority,omitempty"`
	ConcurrencyLimit       *int              `json:"concurrency_limit,omitempty"`
	LoadFactor             *int              `json:"load_factor,omitempty"`
	RateMultiplier         *float64          `json:"rate_multiplier,omitempty"`
	QuotaAmount            *float64          `json:"quota_amount,omitempty"`
	QuotaCurrency          *string           `json:"quota_currency,omitempty"`
	SupportedModelPatterns []string          `json:"supported_model_patterns,omitempty"`
	ModelMapping           map[string]string `json:"model_mapping,omitempty"`
	Metadata               map[string]any    `json:"metadata,omitempty"`
}

type ChannelSecret struct {
	ID           uuid.UUID
	ProviderID   uuid.UUID
	ProviderType string
	Name         string
	BaseURL      string
	APIKey       string
	Status       string
}

type RotateChannelKeyInput struct {
	APIKey string `json:"api_key"`
}

type TestChannelInput struct {
	Model string `json:"model,omitempty"`
}

type CreateRoutingRuleInput struct {
	Name         string         `json:"name"`
	ProviderID   *uuid.UUID     `json:"provider_id,omitempty"`
	ChannelID    *uuid.UUID     `json:"channel_id,omitempty"`
	MatchScope   string         `json:"match_scope,omitempty"`
	MatchValue   string         `json:"match_value,omitempty"`
	ModelPattern string         `json:"model_pattern,omitempty"`
	Priority     int            `json:"priority,omitempty"`
	Status       string         `json:"status,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type UsageAnalysisFilter struct {
	Limit int
}

type UsageAnalysis struct {
	Currency        string             `json:"currency"`
	TotalCost       float64            `json:"total_cost"`
	InvocationCount int                `json:"invocation_count"`
	ByProvider      map[string]float64 `json:"by_provider"`
	ByChannel       map[string]float64 `json:"by_channel"`
	ByModel         map[string]float64 `json:"by_model"`
	ByActor         map[string]float64 `json:"by_actor"`
	Recent          []Invocation       `json:"recent"`
}

type EstimateCostInput struct {
	Model          string     `json:"model"`
	ProviderID     *uuid.UUID `json:"provider_id,omitempty"`
	ProviderType   string     `json:"provider_type,omitempty"`
	Usage          TokenUsage `json:"usage"`
	ServiceTier    string     `json:"service_tier,omitempty"`
	RateMultiplier *float64   `json:"rate_multiplier,omitempty"`
}

type EstimateCostOutput struct {
	Model         string        `json:"model"`
	CostBreakdown CostBreakdown `json:"cost_breakdown"`
	Currency      string        `json:"currency"`
}

func (s *Service) Invoke(ctx context.Context, input InvokeInput) (*InvokeOutput, error) {
	if err := validateInvokeInput(input); err != nil {
		return nil, err
	}
	if err := applyTenantAttribution(ctx, &input); err != nil {
		return nil, err
	}
	target, err := s.repo.ResolveInvocationTarget(ctx, input)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeInvocationWithKernel(ctx, input, target); err != nil {
		return nil, err
	}
	adapter, err := s.adapterFor(target)
	if err != nil {
		return nil, err
	}

	started := time.Now()
	trace := s.startInvocationTrace(ctx, input, target, ModeSync)
	invocation, err := s.repo.CreateInvocation(ctx, CreateInvocationInput{
		ProviderID:        target.ProviderID,
		ModelID:           target.ModelID,
		ChannelID:         target.ChannelID,
		Mode:              ModeSync,
		Status:            StatusStarted,
		Attribution:       input.Attribution,
		RequestedModel:    target.RequestedModel,
		UpstreamModel:     target.UpstreamModel,
		ModelMappingChain: target.ModelMappingChain,
		ServiceTier:       normalizedServiceTier(input.ServiceTier),
		ReasoningEffort:   strings.TrimSpace(input.ReasoningEffort),
		Metadata:          input.Metadata,
	})
	if err != nil {
		s.completeObservationTrace(ctx, trace, observability.TraceFailed)
		return nil, err
	}
	releaseChannel := target.ChannelID != nil
	releaseAmount := 0.0
	defer func() {
		if releaseChannel {
			_ = s.repo.ReleaseChannel(context.Background(), target.ChannelID, releaseAmount)
		}
	}()

	resp, err := adapter.Invoke(ctx, ProviderRequest{
		Model:       target.Model,
		Messages:    input.Messages,
		Temperature: input.Temperature,
		MaxTokens:   maxTokens(input.MaxTokens, target.MaxOutputTokens),
		Tools:       input.Tools,
	})
	if err != nil {
		s.recordFailedInvocation(ctx, invocation.ID, target, started, err)
		releaseChannel = false
		s.recordInvocationSpan(ctx, trace, observability.SpanAIInvocation, invocation.ID, target, input.Attribution, StatusFailed, err.Error(), int(time.Since(started).Milliseconds()))
		s.completeObservationTrace(ctx, trace, observability.TraceFailed)
		return nil, err
	}

	breakdown := CalculateCostBreakdown(resp.Usage, target.Price, target.RateMultiplier, normalizedServiceTier(input.ServiceTier))
	cost := breakdown.ActualCost
	currency := currencyOrDefault(target.Currency)
	if err := s.repo.CreateUsageLedger(ctx, CreateUsageLedgerInput{
		InvocationID:        invocation.ID,
		ChannelID:           target.ChannelID,
		ModelPriceVersionID: target.PriceVersionID,
		LedgerType:          "usage",
		Amount:              cost,
		ActualAmount:        cost,
		Currency:            currency,
		Usage:               resp.Usage,
		CostBreakdown:       breakdown,
		ServiceTier:         normalizedServiceTier(input.ServiceTier),
		ReasoningEffort:     strings.TrimSpace(input.ReasoningEffort),
		RequestedModel:      target.RequestedModel,
		UpstreamModel:       target.UpstreamModel,
	}); err != nil {
		s.completeObservationTrace(ctx, trace, observability.TraceFailed)
		return nil, err
	}
	releaseAmount = cost
	_ = s.repo.ReleaseChannel(ctx, target.ChannelID, releaseAmount)
	releaseChannel = false
	s.recordCostLedger(ctx, invocation.ID, target, input.Attribution, resp.Usage, breakdown, currency, ModeSync, StatusCompleted, "")
	completedAt := time.Now()
	durationMS := int(completedAt.Sub(started).Milliseconds())
	if err := s.repo.CompleteInvocation(ctx, invocation.ID, CompleteInvocationInput{
		ProviderRequestID: resp.ProviderRequestID,
		Usage:             resp.Usage,
		CostAmount:        cost,
		CostBreakdown:     breakdown,
		Currency:          currency,
		DurationMS:        durationMS,
	}); err != nil {
		s.completeObservationTrace(ctx, trace, observability.TraceFailed)
		return nil, err
	}
	s.recordInvocationSpan(ctx, trace, observability.SpanAIInvocation, invocation.ID, target, input.Attribution, StatusCompleted, "", durationMS)
	s.recordAIMetrics(ctx, invocation.ID, resp.Usage, cost, currency, map[string]any{"mode": ModeSync, "provider_type": target.ProviderType, "model": target.Model})
	s.completeObservationTrace(ctx, trace, observability.TraceComplete)
	return &InvokeOutput{
		InvocationID:      invocation.ID,
		ProviderRequestID: resp.ProviderRequestID,
		Content:           resp.Content,
		Usage:             resp.Usage,
		CostAmount:        cost,
		CostBreakdown:     breakdown,
		Currency:          currency,
		ToolCalls:         resp.ToolCalls,
		CompletedAt:       completedAt,
		ProviderType:      target.ProviderType,
		Model:             target.Model,
		RequestedModel:    target.RequestedModel,
		ChannelID:         target.ChannelID,
	}, nil
}

func (s *Service) Stream(ctx context.Context, input InvokeInput) (*StreamResult, error) {
	if err := validateInvokeInput(input); err != nil {
		return nil, err
	}
	if err := applyTenantAttribution(ctx, &input); err != nil {
		return nil, err
	}
	target, err := s.repo.ResolveInvocationTarget(ctx, input)
	if err != nil {
		return nil, err
	}
	adapter, err := s.adapterFor(target)
	if err != nil {
		return nil, err
	}
	started := time.Now()
	trace := s.startInvocationTrace(ctx, input, target, ModeStream)
	invocation, err := s.repo.CreateInvocation(ctx, CreateInvocationInput{
		ProviderID:        target.ProviderID,
		ModelID:           target.ModelID,
		ChannelID:         target.ChannelID,
		Mode:              ModeStream,
		Status:            StatusStreaming,
		Attribution:       input.Attribution,
		RequestedModel:    target.RequestedModel,
		UpstreamModel:     target.UpstreamModel,
		ModelMappingChain: target.ModelMappingChain,
		ServiceTier:       normalizedServiceTier(input.ServiceTier),
		ReasoningEffort:   strings.TrimSpace(input.ReasoningEffort),
		Metadata:          input.Metadata,
	})
	if err != nil {
		s.completeObservationTrace(ctx, trace, observability.TraceFailed)
		return nil, err
	}
	events, err := adapter.Stream(ctx, ProviderRequest{
		Model:       target.Model,
		Messages:    input.Messages,
		Temperature: input.Temperature,
		MaxTokens:   maxTokens(input.MaxTokens, target.MaxOutputTokens),
		Tools:       input.Tools,
	})
	if err != nil {
		s.recordFailedInvocation(ctx, invocation.ID, target, started, err)
		s.recordInvocationSpan(ctx, trace, observability.SpanAIStream, invocation.ID, target, input.Attribution, StatusFailed, err.Error(), int(time.Since(started).Milliseconds()))
		s.completeObservationTrace(ctx, trace, observability.TraceFailed)
		return nil, err
	}
	return &StreamResult{InvocationID: invocation.ID, Events: s.recordingStream(ctx, invocation.ID, target, input.Attribution, normalizedServiceTier(input.ServiceTier), strings.TrimSpace(input.ReasoningEffort), started, events, trace)}, nil
}

func (s *Service) CreateProvider(ctx context.Context, input CreateProviderInput) (*ModelProvider, error) {
	if err := validateProviderInput(input); err != nil {
		return nil, err
	}
	return s.catalogRepo().CreateProvider(ctx, input)
}

func (s *Service) ListProviders(ctx context.Context, limit int) ([]ModelProvider, error) {
	return s.catalogRepo().ListProviders(ctx, limit)
}

func (s *Service) UpdateProvider(ctx context.Context, id uuid.UUID, input UpdateProviderInput) (*ModelProvider, error) {
	return s.catalogRepo().UpdateProvider(ctx, id, input)
}

func (s *Service) RotateProviderKey(ctx context.Context, id uuid.UUID, input RotateProviderKeyInput) (*ModelProvider, error) {
	if input.APIKey == "" {
		return nil, fmt.Errorf("%w: api_key is required", ErrValidation)
	}
	return s.catalogRepo().RotateProviderKey(ctx, id, input.APIKey)
}

func (s *Service) TestProvider(ctx context.Context, id uuid.UUID, input TestProviderInput) error {
	provider, err := s.catalogRepo().GetProviderSecret(ctx, id)
	if err != nil {
		return err
	}
	target := ResolvedModel{
		ProviderID:   provider.ID,
		ProviderType: provider.ProviderType,
		BaseURL:      provider.BaseURL,
		APIKey:       provider.APIKey,
		Model:        input.Model,
	}
	if target.Model == "" {
		target.Model = defaultTestModel(provider.ProviderType)
	}
	adapter, err := s.adapterFor(target)
	if err != nil {
		return err
	}
	_, err = adapter.Invoke(ctx, ProviderRequest{
		Model:     target.Model,
		Messages:  []Message{{Role: "user", Content: "ping"}},
		MaxTokens: 8,
	})
	if err != nil {
		_ = s.catalogRepo().UpdateProviderTestResult(ctx, id, "failed", err.Error())
		return err
	}
	return s.catalogRepo().UpdateProviderTestResult(ctx, id, "ok", "")
}

func (s *Service) CreateModel(ctx context.Context, input CreateModelInput) (*Model, error) {
	if input.ProviderID == uuid.Nil || input.ModelKey == "" {
		return nil, fmt.Errorf("%w: provider_id and model_key are required", ErrValidation)
	}
	return s.catalogRepo().CreateModel(ctx, input)
}

func (s *Service) ListModels(ctx context.Context, providerID *uuid.UUID, limit int) ([]Model, error) {
	return s.catalogRepo().ListModels(ctx, providerID, limit)
}

func (s *Service) UpdateModel(ctx context.Context, id uuid.UUID, input UpdateModelInput) (*Model, error) {
	return s.catalogRepo().UpdateModel(ctx, id, input)
}

func (s *Service) ListInvocations(ctx context.Context, limit int) ([]Invocation, error) {
	return s.catalogRepo().ListInvocations(ctx, limit)
}

func (s *Service) GetInvocation(ctx context.Context, id uuid.UUID) (*Invocation, error) {
	return s.catalogRepo().GetInvocation(ctx, id)
}

func (s *Service) CostSummary(ctx context.Context) (*GatewayCostSummary, error) {
	return s.catalogRepo().CostSummary(ctx)
}

func (s *Service) CreateChannel(ctx context.Context, input CreateChannelInput) (*ProviderChannel, error) {
	if input.ProviderID == uuid.Nil || strings.TrimSpace(input.Name) == "" || input.APIKey == "" {
		return nil, fmt.Errorf("%w: provider_id, name, and api_key are required", ErrValidation)
	}
	return s.catalogRepo().CreateChannel(ctx, input)
}

func (s *Service) ListChannels(ctx context.Context, providerID *uuid.UUID, limit int) ([]ProviderChannel, error) {
	return s.catalogRepo().ListChannels(ctx, providerID, limit)
}

func (s *Service) UpdateChannel(ctx context.Context, id uuid.UUID, input UpdateChannelInput) (*ProviderChannel, error) {
	return s.catalogRepo().UpdateChannel(ctx, id, input)
}

func (s *Service) RotateChannelKey(ctx context.Context, id uuid.UUID, input RotateChannelKeyInput) (*ProviderChannel, error) {
	if input.APIKey == "" {
		return nil, fmt.Errorf("%w: api_key is required", ErrValidation)
	}
	return s.catalogRepo().RotateChannelKey(ctx, id, input.APIKey)
}

func (s *Service) TestChannel(ctx context.Context, id uuid.UUID, input TestChannelInput) error {
	channel, err := s.catalogRepo().GetChannelSecret(ctx, id)
	if err != nil {
		return err
	}
	target := ResolvedModel{
		ProviderID:   channel.ProviderID,
		ChannelID:    &channel.ID,
		ChannelName:  channel.Name,
		ProviderType: channel.ProviderType,
		BaseURL:      channel.BaseURL,
		APIKey:       channel.APIKey,
		Model:        input.Model,
	}
	if target.Model == "" {
		target.Model = defaultTestModel(channel.ProviderType)
	}
	adapter, err := s.adapterFor(target)
	if err != nil {
		return err
	}
	_, err = adapter.Invoke(ctx, ProviderRequest{Model: target.Model, Messages: []Message{{Role: "user", Content: "ping"}}, MaxTokens: 8})
	if err != nil {
		_ = s.catalogRepo().UpdateChannelTestResult(ctx, id, "failed", err.Error())
		return err
	}
	return s.catalogRepo().UpdateChannelTestResult(ctx, id, "ok", "")
}

func (s *Service) ListRoutingRules(ctx context.Context, limit int) ([]RoutingRule, error) {
	return s.catalogRepo().ListRoutingRules(ctx, limit)
}

func (s *Service) CreateRoutingRule(ctx context.Context, input CreateRoutingRuleInput) (*RoutingRule, error) {
	if strings.TrimSpace(input.Name) == "" {
		return nil, fmt.Errorf("%w: name is required", ErrValidation)
	}
	return s.catalogRepo().CreateRoutingRule(ctx, input)
}

func (s *Service) UsageAnalysis(ctx context.Context, filter UsageAnalysisFilter) (*UsageAnalysis, error) {
	return s.catalogRepo().UsageAnalysis(ctx, filter)
}

func (s *Service) EstimateCost(ctx context.Context, input EstimateCostInput) (*EstimateCostOutput, error) {
	target, err := s.catalogRepo().ResolvePricingTarget(ctx, input)
	if err != nil {
		return nil, err
	}
	rate := target.RateMultiplier
	if input.RateMultiplier != nil {
		rate = *input.RateMultiplier
	}
	if rate < 0 {
		rate = target.RateMultiplier
	}
	breakdown := CalculateCostBreakdown(input.Usage, target.Price, rate, normalizedServiceTier(input.ServiceTier))
	return &EstimateCostOutput{Model: target.Model, CostBreakdown: breakdown, Currency: currencyOrDefault(target.Currency)}, nil
}

func (s *Service) recordingStream(ctx context.Context, invocationID uuid.UUID, target ResolvedModel, attribution Attribution, serviceTier string, reasoningEffort string, started time.Time, events <-chan StreamEvent, trace *observability.Trace) <-chan StreamEvent {
	out := make(chan StreamEvent)
	go func() {
		defer close(out)
		usage := TokenUsage{}
		failed := ""
		for event := range events {
			if event.Usage.InputTokens > 0 || event.Usage.OutputTokens > 0 {
				usage = event.Usage
			}
			if event.Error != "" {
				failed = event.Error
			}
			select {
			case <-ctx.Done():
				_ = s.repo.FailInvocation(context.Background(), invocationID, FailInvocationInput{ErrorType: "cancelled", Message: ctx.Err().Error(), DurationMS: int(time.Since(started).Milliseconds()), Cancelled: true})
				_ = s.repo.ReleaseChannel(context.Background(), target.ChannelID, 0)
				durationMS := int(time.Since(started).Milliseconds())
				s.recordInvocationSpan(context.Background(), trace, observability.SpanAIStream, invocationID, target, attribution, StatusCancelled, ctx.Err().Error(), durationMS)
				s.recordMetric(context.Background(), observability.MetricReliability, "ai_stream_disconnect", &invocationID, "ai_invocation", 1, map[string]any{"provider_type": target.ProviderType, "model": target.Model})
				s.completeObservationTrace(context.Background(), trace, observability.TraceFailed)
				return
			case out <- event:
			}
		}
		breakdown := CalculateCostBreakdown(usage, target.Price, target.RateMultiplier, serviceTier)
		cost := breakdown.ActualCost
		currency := currencyOrDefault(target.Currency)
		_ = s.repo.CreateUsageLedger(context.Background(), CreateUsageLedgerInput{
			InvocationID:        invocationID,
			ChannelID:           target.ChannelID,
			ModelPriceVersionID: target.PriceVersionID,
			LedgerType:          "usage",
			Amount:              cost,
			ActualAmount:        cost,
			Currency:            currency,
			Usage:               usage,
			CostBreakdown:       breakdown,
			ServiceTier:         serviceTier,
			ReasoningEffort:     reasoningEffort,
			RequestedModel:      target.RequestedModel,
			UpstreamModel:       target.UpstreamModel,
			Reason:              failed,
		})
		durationMS := int(time.Since(started).Milliseconds())
		if failed != "" {
			_ = s.repo.FailInvocation(context.Background(), invocationID, FailInvocationInput{ErrorType: "provider_error", Message: failed, DurationMS: durationMS})
			_ = s.repo.ReleaseChannel(context.Background(), target.ChannelID, cost)
			s.recordInvocationSpan(context.Background(), trace, observability.SpanAIStream, invocationID, target, attribution, StatusFailed, failed, durationMS)
			s.completeObservationTrace(context.Background(), trace, observability.TraceFailed)
			return
		}
		_ = s.repo.ReleaseChannel(context.Background(), target.ChannelID, cost)
		s.recordCostLedger(context.Background(), invocationID, target, attribution, usage, breakdown, currency, ModeStream, StatusCompleted, "")
		_ = s.repo.CompleteInvocation(context.Background(), invocationID, CompleteInvocationInput{
			Usage:         usage,
			CostAmount:    cost,
			CostBreakdown: breakdown,
			Currency:      currency,
			DurationMS:    durationMS,
		})
		s.recordInvocationSpan(context.Background(), trace, observability.SpanAIStream, invocationID, target, attribution, StatusCompleted, "", durationMS)
		s.recordAIMetrics(context.Background(), invocationID, usage, cost, currency, map[string]any{"mode": ModeStream, "provider_type": target.ProviderType, "model": target.Model})
		s.completeObservationTrace(context.Background(), trace, observability.TraceComplete)
	}()
	return out
}

func (s *Service) recordFailedInvocation(ctx context.Context, id uuid.UUID, target ResolvedModel, started time.Time, cause error) {
	_ = s.repo.CreateUsageLedger(ctx, CreateUsageLedgerInput{
		InvocationID:        id,
		ChannelID:           target.ChannelID,
		ModelPriceVersionID: target.PriceVersionID,
		LedgerType:          "usage",
		Amount:              0,
		Currency:            currencyOrDefault(target.Currency),
		Reason:              cause.Error(),
	})
	_ = s.repo.ReleaseChannel(ctx, target.ChannelID, 0)
	_ = s.repo.FailInvocation(ctx, id, FailInvocationInput{ErrorType: "provider_error", Message: cause.Error(), DurationMS: int(time.Since(started).Milliseconds())})
}

func (s *Service) adapterFor(target ResolvedModel) (ProviderAdapter, error) {
	if adapter, ok := s.adapters[target.ProviderType]; ok {
		return adapter, nil
	}
	switch target.ProviderType {
	case ProviderOpenAI:
		return NewOpenAIAdapter(target.BaseURL, target.APIKey, s.client), nil
	case ProviderAnthropic:
		return NewAnthropicAdapter(target.BaseURL, target.APIKey, s.client), nil
	case ProviderGemini:
		return NewGeminiAdapter(target.BaseURL, target.APIKey, s.client), nil
	default:
		return nil, fmt.Errorf("%w: unsupported provider type %q", ErrValidation, target.ProviderType)
	}
}

func (s *Service) authorizeInvocationWithKernel(ctx context.Context, input InvokeInput, target ResolvedModel) error {
	if s.securityKernel == nil {
		return nil
	}
	tenant, _ := middleware.TenantFromContext(ctx)
	actorID, actorType := actorFromAttribution(input.Attribution)
	if actorID == nil && tenant != nil && tenant.UserID != uuid.Nil {
		id := tenant.UserID
		actorID = &id
		actorType = "human"
	}
	request := securitykernel.Request{
		Actor: securitykernel.Actor{
			ActorID:         optionalActorID(actorID),
			ActorType:       actorType,
			AuthorityTier:   tenantAuthorityTier(tenant),
			IsPlatformAdmin: tenant != nil && tenant.IsPlatformAdmin,
		},
		OrganizationID:  input.Attribution.OrganizationID,
		EnabledModules:  tenantEnabledModules(tenant),
		EnabledFeatures: []string{"ai_model_use"},
		Resource: securitykernel.Resource{
			ModuleKey:             "ai_gateway",
			ResourceType:          "model_provider_channel",
			Action:                "use",
			ScopeLevel:            "organization",
			OrganizationID:        input.Attribution.OrganizationID,
			RequiredAuthorityTier: "executor",
			RequiredLicenseMode:   "commercial",
		},
		Metadata: map[string]any{
			"provider_id":   target.ProviderID.String(),
			"provider_type": target.ProviderType,
			"model_id":      target.ModelID.String(),
			"model":         target.Model,
			"channel_id":    optionalUUIDString(target.ChannelID),
		},
	}
	decision, err := s.securityKernel.Authorize(ctx, request)
	if err != nil || !decision.Allowed {
		reason := decision.Reason
		if reason == "" && err != nil {
			reason = err.Error()
		}
		return fmt.Errorf("%w: security kernel denied ai invocation: %s", ErrForbidden, reason)
	}
	return nil
}

func (s *Service) catalogRepo() CatalogRepository {
	if s.catalog == nil {
		panic("aigateway: catalog repository is not configured")
	}
	return s.catalog
}

func validateProviderInput(input CreateProviderInput) error {
	if input.Name == "" || input.ProviderType == "" || input.APIKey == "" {
		return fmt.Errorf("%w: name, provider_type, and api_key are required", ErrValidation)
	}
	switch input.ProviderType {
	case ProviderOpenAI, ProviderAnthropic, ProviderGemini:
		return nil
	default:
		return fmt.Errorf("%w: unsupported provider type %q", ErrValidation, input.ProviderType)
	}
}

func defaultTestModel(providerType string) string {
	switch providerType {
	case ProviderOpenAI:
		return "gpt-4o-mini"
	case ProviderAnthropic:
		return "claude-3-5-haiku-latest"
	case ProviderGemini:
		return "gemini-1.5-flash"
	default:
		return ""
	}
}

func validateInvokeInput(input InvokeInput) error {
	if (input.ProviderID == nil && input.ProviderType == "") || (input.ModelID == nil && input.Model == "") {
		return fmt.Errorf("%w: provider and model are required", ErrValidation)
	}
	if len(input.Messages) == 0 {
		return fmt.Errorf("%w: messages are required", ErrValidation)
	}
	return nil
}

func applyTenantAttribution(ctx context.Context, input *InvokeInput) error {
	orgID := currentTenantOrganizationID(ctx)
	if orgID == nil {
		return nil
	}
	if input.Attribution.OrganizationID != nil && *input.Attribution.OrganizationID != *orgID {
		return fmt.Errorf("%w: attribution.organization_id must match current organization", ErrValidation)
	}
	id := *orgID
	input.Attribution.OrganizationID = &id
	return nil
}

func maxTokens(requested int, modelDefault int) int {
	if requested > 0 {
		return requested
	}
	return modelDefault
}

func currencyOrDefault(currency string) string {
	if currency == "" {
		return "CNY"
	}
	return currency
}

func (s *Service) startInvocationTrace(ctx context.Context, input InvokeInput, target ResolvedModel, mode string) *observability.Trace {
	if s.observability == nil {
		return nil
	}
	trace, err := s.observability.StartTrace(ctx, input.Attribution.WorkflowID, map[string]any{
		"category":        "ai_invocation",
		"mode":            mode,
		"provider_id":     target.ProviderID.String(),
		"provider_type":   target.ProviderType,
		"model_id":        target.ModelID.String(),
		"model":           target.Model,
		"source_surface":  input.Attribution.SourceSurface,
		"organization_id": optionalUUIDString(input.Attribution.OrganizationID),
		"department_id":   optionalUUIDString(input.Attribution.DepartmentID),
		"project_id":      optionalUUIDString(input.Attribution.ProjectID),
		"requirement_id":  optionalUUIDString(input.Attribution.RequirementID),
		"task_id":         optionalUUIDString(input.Attribution.TaskID),
	})
	if err != nil {
		return nil
	}
	return trace
}

func (s *Service) recordInvocationSpan(ctx context.Context, trace *observability.Trace, spanType observability.SpanType, invocationID uuid.UUID, target ResolvedModel, attribution Attribution, status string, message string, durationMS int) {
	if s.observability == nil || trace == nil {
		return
	}
	actorID, actorType := actorFromAttribution(attribution)
	_, _ = s.observability.RecordSpan(ctx, observability.RecordSpanInput{
		TraceID:    trace.ID,
		SpanType:   spanType,
		EntityID:   &invocationID,
		EntityType: "ai_invocation",
		ActorID:    actorID,
		ActorType:  actorType,
		Input: map[string]any{
			"provider_id":   target.ProviderID.String(),
			"provider_type": target.ProviderType,
			"model_id":      target.ModelID.String(),
			"model":         target.Model,
		},
		Output: map[string]any{
			"status": status,
			"error":  message,
		},
		DurationMs: durationMS,
		Metadata: map[string]any{
			"source_surface": attribution.SourceSurface,
			"project_id":     optionalUUIDString(attribution.ProjectID),
		},
	})
}

func (s *Service) recordAIMetrics(ctx context.Context, invocationID uuid.UUID, usage TokenUsage, cost float64, currency string, metadata map[string]any) {
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["currency"] = currency
	s.recordMetric(ctx, observability.MetricUsage, "ai_tokens_input", &invocationID, "ai_invocation", float64(usage.InputTokens), metadata)
	s.recordMetric(ctx, observability.MetricUsage, "ai_tokens_output", &invocationID, "ai_invocation", float64(usage.OutputTokens), metadata)
	s.recordMetric(ctx, observability.MetricCost, "ai_cost_amount", &invocationID, "ai_invocation", cost, metadata)
}

func (s *Service) recordCostLedger(ctx context.Context, invocationID uuid.UUID, target ResolvedModel, attribution Attribution, usage TokenUsage, breakdown CostBreakdown, currency string, mode string, status string, reason string) {
	if s.costRecorder == nil || breakdown.ActualCost == 0 {
		return
	}
	actorID, actorType := actorFromAttribution(attribution)
	metadata := map[string]any{
		"ai_invocation_id":      invocationID.String(),
		"provider_id":           target.ProviderID.String(),
		"provider_type":         target.ProviderType,
		"model_id":              target.ModelID.String(),
		"model":                 target.Model,
		"requested_model":       target.RequestedModel,
		"upstream_model":        target.UpstreamModel,
		"channel_name":          target.ChannelName,
		"mode":                  mode,
		"status":                status,
		"input_tokens":          usage.InputTokens,
		"output_tokens":         usage.OutputTokens,
		"cache_creation_tokens": usage.CacheCreationTokens,
		"cache_read_tokens":     usage.CacheReadTokens,
		"source_surface":        attribution.SourceSurface,
		"cost_breakdown":        breakdown,
	}
	if target.ChannelID != nil {
		metadata["channel_id"] = target.ChannelID.String()
	}
	if target.PriceVersionID != nil {
		metadata["model_price_version_id"] = target.PriceVersionID.String()
	}
	if reason != "" {
		metadata["reason"] = reason
	}
	_, _ = s.costRecorder.RecordActual(ctx, costing.CreateLedgerEntryInput{
		CostCategory:   "model_token",
		SourceType:     "ai_invocation",
		SourceID:       &invocationID,
		OrganizationID: attribution.OrganizationID,
		DepartmentID:   attribution.DepartmentID,
		RequirementID:  attribution.RequirementID,
		ProjectID:      attribution.ProjectID,
		WorkflowID:     attribution.WorkflowID,
		TaskID:         attribution.TaskID,
		CapabilityID:   attribution.CapabilityID,
		ActorID:        actorID,
		ActorType:      actorType,
		ResourceType:   "model_token",
		Amount:         breakdown.ActualCost,
		Currency:       currency,
		Status:         "posted",
		Description:    "AI model token usage",
		Metadata:       metadata,
	})
}

func normalizedServiceTier(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func (s *Service) recordMetric(ctx context.Context, metricType observability.MetricType, name string, entityID *uuid.UUID, entityType string, value float64, metadata map[string]any) {
	if s.observability == nil {
		return
	}
	_, _ = s.observability.RecordMetric(ctx, observability.RecordMetricInput{
		MetricType: metricType,
		MetricName: name,
		EntityID:   entityID,
		EntityType: entityType,
		Value:      value,
		Metadata:   metadata,
	})
}

func (s *Service) completeObservationTrace(ctx context.Context, trace *observability.Trace, status observability.TraceStatus) {
	if s.observability == nil || trace == nil {
		return
	}
	_ = s.observability.CompleteTrace(ctx, trace.ID, string(status))
}

func actorFromAttribution(attribution Attribution) (*uuid.UUID, string) {
	if attribution.UserID != nil {
		return attribution.UserID, "human"
	}
	if attribution.AgentID != nil {
		return attribution.AgentID, "ai_agent"
	}
	return nil, ""
}

func optionalActorID(id *uuid.UUID) uuid.UUID {
	if id == nil {
		return uuid.Nil
	}
	return *id
}

func optionalUUIDString(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

func tenantAuthorityTier(tenant *middleware.TenantContext) string {
	if tenant == nil {
		return ""
	}
	return tenant.AuthorityTier
}

func tenantEnabledModules(tenant *middleware.TenantContext) []string {
	if tenant == nil || tenant.EnabledModules == nil {
		return []string{}
	}
	modules := []string{}
	for moduleKey, enabled := range tenant.EnabledModules {
		if enabled {
			modules = append(modules, moduleKey)
		}
	}
	return modules
}
