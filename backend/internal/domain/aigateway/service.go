package aigateway

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/costing"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/observability"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/securitykernel"
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
	AuthenticateAccessToken(ctx context.Context, token string) (AccessTokenContext, error)
	ReserveAccessTokenBalance(ctx context.Context, input ReserveBalanceInput) (BalanceReservation, error)
	AttachBalanceReservation(ctx context.Context, reservationID uuid.UUID, invocationID uuid.UUID) error
	SettleAccessTokenBalance(ctx context.Context, input SettleBalanceInput) error
	RefundAccessTokenBalance(ctx context.Context, reservationID uuid.UUID, reason string) error
}

type CatalogRepository interface {
	CreateProvider(ctx context.Context, input CreateProviderInput) (*ModelProvider, error)
	ListProviders(ctx context.Context, limit int) ([]ModelProvider, error)
	ListActiveProviders(ctx context.Context, limit int) ([]ModelProvider, error)
	UpdateProvider(ctx context.Context, id uuid.UUID, input UpdateProviderInput) (*ModelProvider, error)
	RotateProviderKey(ctx context.Context, id uuid.UUID, apiKey string) (*ModelProvider, error)
	UpdateProviderTestResult(ctx context.Context, id uuid.UUID, status string, message string) error
	GetProviderSecret(ctx context.Context, id uuid.UUID) (ProviderSecret, error)
	CreateModel(ctx context.Context, input CreateModelInput) (*Model, error)
	ListModels(ctx context.Context, providerID *uuid.UUID, limit int) ([]Model, error)
	ListActiveModels(ctx context.Context) ([]Model, error)
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
	CreateAccessToken(ctx context.Context, input CreateAccessTokenStoreInput) (*AccessToken, error)
	ListAccessTokens(ctx context.Context, organizationID *uuid.UUID, limit int) ([]AccessToken, error)
	CreateModelGroup(ctx context.Context, input CreateModelGroupInput) (*ModelGroup, error)
	ListModelGroups(ctx context.Context, organizationID *uuid.UUID, limit int) ([]ModelGroup, error)
	CreateModelChannelAbility(ctx context.Context, input CreateModelChannelAbilityInput) (*ModelChannelAbility, error)
	ListModelChannelAbilities(ctx context.Context, modelGroupID *uuid.UUID, limit int) ([]ModelChannelAbility, error)
	GetGatewayBalance(ctx context.Context, organizationID uuid.UUID, currency string) (*GatewayBalance, error)
	ListBalanceTransactions(ctx context.Context, organizationID uuid.UUID, limit int) ([]BalanceTransaction, error)
	AdjustGatewayBalance(ctx context.Context, input AdjustGatewayBalanceInput) (*GatewayBalance, error)
}

type Service struct {
	repo           InvocationRepository
	catalog        CatalogRepository
	adapters       AdapterRegistry
	client         *http.Client
	observability  ObservabilityRecorder
	costRecorder   CostRecorder
	securityKernel securitykernel.Client
	invokeTimeout  time.Duration
	streamTimeout  time.Duration
	maxRetries     int
	retryBaseDelay time.Duration
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

func WithInvocationTimeouts(invokeTimeout, streamTimeout time.Duration) ServiceOption {
	return func(s *Service) {
		if invokeTimeout > 0 {
			s.invokeTimeout = invokeTimeout
		}
		if streamTimeout > 0 {
			s.streamTimeout = streamTimeout
		}
	}
}

func WithProviderRetryPolicy(maxRetries int, baseDelay time.Duration) ServiceOption {
	return func(s *Service) {
		if maxRetries >= 0 {
			s.maxRetries = maxRetries
		}
		if baseDelay > 0 {
			s.retryBaseDelay = baseDelay
		}
	}
}

func NewService(repo InvocationRepository, adapters AdapterRegistry, opts ...ServiceOption) *Service {
	catalog, _ := repo.(CatalogRepository)
	if adapters == nil {
		adapters = AdapterRegistry{}
	}
	s := &Service{
		repo:           repo,
		catalog:        catalog,
		adapters:       adapters,
		client:         &http.Client{},
		invokeTimeout:  60 * time.Second,
		streamTimeout:  10 * time.Minute,
		maxRetries:     3,
		retryBaseDelay: 100 * time.Millisecond,
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
	TimeoutMS           int
	RetryCount          int
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
	AccessTokenID      *uuid.UUID       `json:"access_token_id,omitempty"`
	ModelGroupID       *uuid.UUID       `json:"model_group_id,omitempty"`
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
	Model        string
	Events       <-chan StreamEvent
}

type CreateInvocationInput struct {
	ProviderID            uuid.UUID
	ModelID               uuid.UUID
	ChannelID             *uuid.UUID
	AccessTokenID         *uuid.UUID
	ModelGroupID          *uuid.UUID
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
	ReservedAmount        float64
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
	AccessTokenID       *uuid.UUID
	ModelGroupID        *uuid.UUID
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

type AccessTokenContext struct {
	ID                   uuid.UUID      `json:"id"`
	OrganizationID       uuid.UUID      `json:"organization_id"`
	ModelGroupID         *uuid.UUID     `json:"model_group_id,omitempty"`
	AllowedModels        []string       `json:"allowed_models"`
	AllowedModelPatterns []string       `json:"allowed_model_patterns,omitempty"`
	AllowChannelOverride bool           `json:"allow_channel_override"`
	Status               string         `json:"status"`
	Metadata             map[string]any `json:"metadata,omitempty"`
}

type AccessToken struct {
	ID                   uuid.UUID      `json:"id"`
	OrganizationID       uuid.UUID      `json:"organization_id"`
	ModelGroupID         *uuid.UUID     `json:"model_group_id,omitempty"`
	Name                 string         `json:"name"`
	TokenHash            string         `json:"-"`
	MaskedToken          string         `json:"masked_token"`
	PlainToken           string         `json:"plain_token,omitempty"`
	Status               string         `json:"status"`
	AllowedModels        []string       `json:"allowed_models"`
	AllowedModelPatterns []string       `json:"allowed_model_patterns"`
	AllowChannelOverride bool           `json:"allow_channel_override"`
	QuotaAmount          float64        `json:"quota_amount"`
	QuotaUsed            float64        `json:"quota_used"`
	QuotaCurrency        string         `json:"quota_currency"`
	ExpiresAt            *time.Time     `json:"expires_at,omitempty"`
	LastUsedAt           *time.Time     `json:"last_used_at,omitempty"`
	Metadata             map[string]any `json:"metadata"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
}

type CreateAccessTokenInput struct {
	OrganizationID       uuid.UUID      `json:"organization_id"`
	ModelGroupID         *uuid.UUID     `json:"model_group_id,omitempty"`
	Name                 string         `json:"name"`
	AllowedModels        []string       `json:"allowed_models,omitempty"`
	AllowedModelPatterns []string       `json:"allowed_model_patterns,omitempty"`
	AllowChannelOverride bool           `json:"allow_channel_override,omitempty"`
	QuotaAmount          float64        `json:"quota_amount,omitempty"`
	QuotaCurrency        string         `json:"quota_currency,omitempty"`
	ExpiresAt            *time.Time     `json:"expires_at,omitempty"`
	Metadata             map[string]any `json:"metadata,omitempty"`
}

type CreateAccessTokenStoreInput struct {
	OrganizationID       uuid.UUID
	ModelGroupID         *uuid.UUID
	Name                 string
	TokenHash            string
	MaskedToken          string
	AllowedModels        []string
	AllowedModelPatterns []string
	AllowChannelOverride bool
	QuotaAmount          float64
	QuotaCurrency        string
	ExpiresAt            *time.Time
	Metadata             map[string]any
}

type ModelGroup struct {
	ID             uuid.UUID      `json:"id"`
	OrganizationID *uuid.UUID     `json:"organization_id,omitempty"`
	DepartmentID   *uuid.UUID     `json:"department_id,omitempty"`
	ProjectID      *uuid.UUID     `json:"project_id,omitempty"`
	AgentID        *uuid.UUID     `json:"agent_id,omitempty"`
	Name           string         `json:"name"`
	GroupKey       string         `json:"group_key"`
	Status         string         `json:"status"`
	RateMultiplier float64        `json:"rate_multiplier"`
	Metadata       map[string]any `json:"metadata"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type ModelChannelAbility struct {
	ID             uuid.UUID      `json:"id"`
	ModelGroupID   *uuid.UUID     `json:"model_group_id,omitempty"`
	ChannelID      uuid.UUID      `json:"channel_id"`
	RequestedModel string         `json:"requested_model"`
	ModelPattern   string         `json:"model_pattern"`
	UpstreamModel  string         `json:"upstream_model"`
	Priority       int            `json:"priority"`
	Enabled        bool           `json:"enabled"`
	Metadata       map[string]any `json:"metadata"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type CreateModelChannelAbilityInput struct {
	ModelGroupID   *uuid.UUID     `json:"model_group_id,omitempty"`
	ChannelID      uuid.UUID      `json:"channel_id"`
	RequestedModel string         `json:"requested_model,omitempty"`
	ModelPattern   string         `json:"model_pattern,omitempty"`
	UpstreamModel  string         `json:"upstream_model,omitempty"`
	Priority       int            `json:"priority,omitempty"`
	Enabled        *bool          `json:"enabled,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type CreateModelGroupInput struct {
	OrganizationID *uuid.UUID     `json:"organization_id,omitempty"`
	DepartmentID   *uuid.UUID     `json:"department_id,omitempty"`
	ProjectID      *uuid.UUID     `json:"project_id,omitempty"`
	AgentID        *uuid.UUID     `json:"agent_id,omitempty"`
	Name           string         `json:"name"`
	GroupKey       string         `json:"group_key,omitempty"`
	Status         string         `json:"status,omitempty"`
	RateMultiplier float64        `json:"rate_multiplier,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type GatewayBalance struct {
	ID             uuid.UUID      `json:"id"`
	OrganizationID uuid.UUID      `json:"organization_id"`
	BalanceAmount  float64        `json:"balance_amount"`
	ReservedAmount float64        `json:"reserved_amount"`
	Currency       string         `json:"currency"`
	Metadata       map[string]any `json:"metadata"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type BalanceTransaction struct {
	ID              uuid.UUID      `json:"id"`
	OrganizationID  uuid.UUID      `json:"organization_id"`
	AccessTokenID   *uuid.UUID     `json:"access_token_id,omitempty"`
	ModelGroupID    *uuid.UUID     `json:"model_group_id,omitempty"`
	InvocationID    *uuid.UUID     `json:"invocation_id,omitempty"`
	ReservationID   *uuid.UUID     `json:"reservation_id,omitempty"`
	TransactionType string         `json:"transaction_type"`
	Amount          float64        `json:"amount"`
	Currency        string         `json:"currency"`
	Reason          string         `json:"reason"`
	Metadata        map[string]any `json:"metadata"`
	CreatedAt       time.Time      `json:"created_at"`
}

type AdapterDescriptor struct {
	AdapterKey       string   `json:"adapter_key"`
	DisplayName      string   `json:"display_name"`
	ProviderType     string   `json:"provider_type"`
	AdapterMode      string   `json:"adapter_mode"`
	DefaultBaseURL   string   `json:"default_base_url"`
	SupportedModels  []string `json:"supported_models"`
	SupportedModes   []string `json:"supported_modes"`
	RequiresNativeIO bool     `json:"requires_native_io"`
}

type ReserveBalanceInput struct {
	AccessTokenID   uuid.UUID
	OrganizationID  uuid.UUID
	ModelGroupID    *uuid.UUID
	EstimatedAmount float64
	Currency        string
	Reason          string
}

type BalanceReservation struct {
	ID             uuid.UUID `json:"id"`
	ReservedAmount float64   `json:"reserved_amount"`
	Currency       string    `json:"currency"`
}

type SettleBalanceInput struct {
	ReservationID uuid.UUID
	AccessTokenID uuid.UUID
	ActualAmount  float64
	Currency      string
	Reason        string
}

type AdjustGatewayBalanceInput struct {
	OrganizationID uuid.UUID      `json:"organization_id"`
	Amount         float64        `json:"amount"`
	Currency       string         `json:"currency,omitempty"`
	Reason         string         `json:"reason"`
	Metadata       map[string]any `json:"metadata,omitempty"`
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
	TimeoutMS    int
	RetryCount   int
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
	TimeoutMS    int
	RetryCount   int
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
	return s.invokeSync(ctx, input, nil)
}

func (s *Service) AuthenticateAccessToken(ctx context.Context, token string) (AccessTokenContext, error) {
	accessToken, err := s.repo.AuthenticateAccessToken(ctx, token)
	if err != nil {
		return AccessTokenContext{}, err
	}
	if accessToken.Status != "" && accessToken.Status != "active" {
		return AccessTokenContext{}, fmt.Errorf("%w: ai access token is not active", ErrForbidden)
	}
	return accessToken, nil
}

func (s *Service) ListModelsWithAccessToken(ctx context.Context, token string, limit int) ([]Model, error) {
	accessToken, err := s.AuthenticateAccessToken(ctx, token)
	if err != nil {
		return nil, err
	}
	models, err := s.catalogRepo().ListActiveModels(ctx)
	if err != nil {
		return nil, err
	}
	var abilities []ModelChannelAbility
	if accessToken.ModelGroupID != nil {
		abilities, err = s.catalogRepo().ListModelChannelAbilities(ctx, accessToken.ModelGroupID, 100)
		if err != nil {
			return nil, err
		}
	}
	limit = normalizeLimit(limit)
	allowed := make([]Model, 0, min(limit, len(models)))
	for _, model := range models {
		if model.Status != "" && model.Status != "active" {
			continue
		}
		if !accessTokenAllowsModel(accessToken, model.ModelKey) {
			continue
		}
		if accessToken.ModelGroupID != nil && !modelAllowedByAbilities(abilities, model.ModelKey) {
			continue
		}
		allowed = append(allowed, model)
		if len(allowed) >= limit {
			break
		}
	}
	return allowed, nil
}

func (s *Service) InvokeWithAccessToken(ctx context.Context, token string, input InvokeInput) (*InvokeOutput, error) {
	accessToken, input, err := s.prepareAccessTokenInvocation(ctx, token, input)
	if err != nil {
		return nil, err
	}
	return s.invokeSync(ctx, input, &accessToken)
}

func (s *Service) invokeSync(ctx context.Context, input InvokeInput, accessToken *AccessTokenContext) (*InvokeOutput, error) {
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
	releaseChannel := target.ChannelID != nil
	releaseAmount := 0.0
	defer func() {
		if releaseChannel {
			_ = s.repo.ReleaseChannel(context.Background(), target.ChannelID, releaseAmount)
		}
	}()
	var reservation *BalanceReservation
	if accessToken != nil {
		estimatedUsage := estimateInvocationUsage(input, target)
		estimatedBreakdown := CalculateCostBreakdown(estimatedUsage, target.Price, target.RateMultiplier, normalizedServiceTier(input.ServiceTier))
		reserved, err := s.repo.ReserveAccessTokenBalance(ctx, ReserveBalanceInput{
			AccessTokenID:   accessToken.ID,
			OrganizationID:  accessToken.OrganizationID,
			ModelGroupID:    accessToken.ModelGroupID,
			EstimatedAmount: estimatedBreakdown.ActualCost,
			Currency:        currencyOrDefault(target.Currency),
			Reason:          "ai_invocation_reserve",
		})
		if err != nil {
			return nil, err
		}
		reservation = &reserved
	}
	if err := s.authorizeInvocationWithKernel(ctx, input, target); err != nil {
		if reservation != nil {
			_ = s.repo.RefundAccessTokenBalance(context.Background(), reservation.ID, err.Error())
		}
		return nil, err
	}
	adapter, err := s.adapterFor(target)
	if err != nil {
		if reservation != nil {
			_ = s.repo.RefundAccessTokenBalance(context.Background(), reservation.ID, err.Error())
		}
		return nil, err
	}

	started := time.Now()
	trace := s.startInvocationTrace(ctx, input, target, ModeSync)
	invocation, err := s.repo.CreateInvocation(ctx, CreateInvocationInput{
		ProviderID:        target.ProviderID,
		ModelID:           target.ModelID,
		ChannelID:         target.ChannelID,
		AccessTokenID:     input.AccessTokenID,
		ModelGroupID:      input.ModelGroupID,
		Mode:              ModeSync,
		Status:            StatusStarted,
		Attribution:       input.Attribution,
		RequestedModel:    target.RequestedModel,
		UpstreamModel:     target.UpstreamModel,
		ModelMappingChain: target.ModelMappingChain,
		ServiceTier:       normalizedServiceTier(input.ServiceTier),
		ReasoningEffort:   strings.TrimSpace(input.ReasoningEffort),
		ReservedAmount:    reservedAmount(reservation),
		Metadata:          input.Metadata,
	})
	if err != nil {
		if reservation != nil {
			_ = s.repo.RefundAccessTokenBalance(context.Background(), reservation.ID, err.Error())
		}
		s.completeObservationTrace(ctx, trace, observability.TraceFailed)
		return nil, err
	}
	if reservation != nil {
		if err := s.repo.AttachBalanceReservation(ctx, reservation.ID, invocation.ID); err != nil {
			_ = s.repo.RefundAccessTokenBalance(context.Background(), reservation.ID, err.Error())
			_ = s.repo.FailInvocation(context.Background(), invocation.ID, FailInvocationInput{ErrorType: "reservation_attach_error", Message: err.Error(), DurationMS: int(time.Since(started).Milliseconds())})
			s.completeObservationTrace(ctx, trace, observability.TraceFailed)
			return nil, err
		}
	}
	resp, err := s.invokeAdapter(ctx, adapter, ProviderRequest{
		Model:       target.Model,
		Messages:    input.Messages,
		Temperature: input.Temperature,
		MaxTokens:   maxTokens(input.MaxTokens, target.MaxOutputTokens),
		Tools:       input.Tools,
	}, target.TimeoutMS, target.RetryCount)
	if err != nil {
		s.recordFailedInvocation(ctx, invocation.ID, target, started, err)
		if reservation != nil {
			_ = s.repo.RefundAccessTokenBalance(context.Background(), reservation.ID, err.Error())
		}
		releaseChannel = false
		s.recordInvocationSpan(ctx, trace, observability.SpanAIInvocation, invocation.ID, target, input.Attribution, StatusFailed, err.Error(), int(time.Since(started).Milliseconds()))
		s.completeObservationTrace(ctx, trace, observability.TraceFailed)
		return nil, err
	}

	breakdown := CalculateCostBreakdown(resp.Usage, target.Price, target.RateMultiplier, normalizedServiceTier(input.ServiceTier))
	cost := breakdown.ActualCost
	releaseAmount = cost
	currency := currencyOrDefault(target.Currency)
	if err := s.repo.CreateUsageLedger(ctx, CreateUsageLedgerInput{
		InvocationID:        invocation.ID,
		AccessTokenID:       input.AccessTokenID,
		ModelGroupID:        input.ModelGroupID,
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
		Metadata:            accessTokenLedgerMetadata(input),
	}); err != nil {
		if reservation != nil {
			_ = s.repo.RefundAccessTokenBalance(context.Background(), reservation.ID, err.Error())
		}
		s.completeObservationTrace(ctx, trace, observability.TraceFailed)
		return nil, err
	}
	if reservation != nil {
		if err := s.repo.SettleAccessTokenBalance(ctx, SettleBalanceInput{
			ReservationID: reservation.ID,
			AccessTokenID: accessToken.ID,
			ActualAmount:  cost,
			Currency:      currency,
			Reason:        "ai_invocation_settle",
		}); err != nil {
			s.completeObservationTrace(ctx, trace, observability.TraceFailed)
			return nil, err
		}
	}
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
	return s.stream(ctx, input, nil)
}

func (s *Service) StreamWithAccessToken(ctx context.Context, token string, input InvokeInput) (*StreamResult, error) {
	accessToken, input, err := s.prepareAccessTokenInvocation(ctx, token, input)
	if err != nil {
		return nil, err
	}
	return s.stream(ctx, input, &accessToken)
}

func (s *Service) stream(ctx context.Context, input InvokeInput, accessToken *AccessTokenContext) (*StreamResult, error) {
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
	releaseChannel := target.ChannelID != nil
	defer func() {
		if releaseChannel {
			_ = s.repo.ReleaseChannel(context.Background(), target.ChannelID, 0)
		}
	}()
	var reservation *BalanceReservation
	if accessToken != nil {
		estimatedUsage := estimateInvocationUsage(input, target)
		estimatedBreakdown := CalculateCostBreakdown(estimatedUsage, target.Price, target.RateMultiplier, normalizedServiceTier(input.ServiceTier))
		reserved, err := s.repo.ReserveAccessTokenBalance(ctx, ReserveBalanceInput{
			AccessTokenID:   accessToken.ID,
			OrganizationID:  accessToken.OrganizationID,
			ModelGroupID:    accessToken.ModelGroupID,
			EstimatedAmount: estimatedBreakdown.ActualCost,
			Currency:        currencyOrDefault(target.Currency),
			Reason:          "ai_stream_reserve",
		})
		if err != nil {
			return nil, err
		}
		reservation = &reserved
	}
	if err := s.authorizeInvocationWithKernel(ctx, input, target); err != nil {
		if reservation != nil {
			_ = s.repo.RefundAccessTokenBalance(context.Background(), reservation.ID, err.Error())
		}
		return nil, err
	}
	adapter, err := s.adapterFor(target)
	if err != nil {
		if reservation != nil {
			_ = s.repo.RefundAccessTokenBalance(context.Background(), reservation.ID, err.Error())
		}
		return nil, err
	}
	started := time.Now()
	trace := s.startInvocationTrace(ctx, input, target, ModeStream)
	invocation, err := s.repo.CreateInvocation(ctx, CreateInvocationInput{
		ProviderID:        target.ProviderID,
		ModelID:           target.ModelID,
		ChannelID:         target.ChannelID,
		AccessTokenID:     input.AccessTokenID,
		ModelGroupID:      input.ModelGroupID,
		Mode:              ModeStream,
		Status:            StatusStreaming,
		Attribution:       input.Attribution,
		RequestedModel:    target.RequestedModel,
		UpstreamModel:     target.UpstreamModel,
		ModelMappingChain: target.ModelMappingChain,
		ServiceTier:       normalizedServiceTier(input.ServiceTier),
		ReasoningEffort:   strings.TrimSpace(input.ReasoningEffort),
		ReservedAmount:    reservedAmount(reservation),
		Metadata:          input.Metadata,
	})
	if err != nil {
		if reservation != nil {
			_ = s.repo.RefundAccessTokenBalance(context.Background(), reservation.ID, err.Error())
		}
		s.completeObservationTrace(ctx, trace, observability.TraceFailed)
		return nil, err
	}
	if reservation != nil {
		if err := s.repo.AttachBalanceReservation(ctx, reservation.ID, invocation.ID); err != nil {
			_ = s.repo.RefundAccessTokenBalance(context.Background(), reservation.ID, err.Error())
			_ = s.repo.FailInvocation(context.Background(), invocation.ID, FailInvocationInput{ErrorType: "reservation_attach_error", Message: err.Error(), DurationMS: int(time.Since(started).Milliseconds())})
			s.completeObservationTrace(ctx, trace, observability.TraceFailed)
			return nil, err
		}
	}
	streamCtx, streamCancel := context.WithTimeout(ctx, effectiveTimeout(s.streamTimeout, target.TimeoutMS))
	events, err := s.streamAdapter(streamCtx, adapter, ProviderRequest{
		Model:       target.Model,
		Messages:    input.Messages,
		Temperature: input.Temperature,
		MaxTokens:   maxTokens(input.MaxTokens, target.MaxOutputTokens),
		Tools:       input.Tools,
	}, target.RetryCount)
	if err != nil {
		streamCancel()
		s.recordFailedInvocation(ctx, invocation.ID, target, started, err)
		releaseChannel = false
		if reservation != nil {
			_ = s.repo.RefundAccessTokenBalance(context.Background(), reservation.ID, err.Error())
		}
		s.recordInvocationSpan(ctx, trace, observability.SpanAIStream, invocation.ID, target, input.Attribution, StatusFailed, err.Error(), int(time.Since(started).Milliseconds()))
		s.completeObservationTrace(ctx, trace, observability.TraceFailed)
		return nil, err
	}
	releaseChannel = false
	return &StreamResult{InvocationID: invocation.ID, Model: target.Model, Events: s.recordingStream(streamCtx, streamCancel, invocation.ID, target, input, normalizedServiceTier(input.ServiceTier), strings.TrimSpace(input.ReasoningEffort), started, events, trace, reservation, accessToken)}, nil
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

func (s *Service) ListTenantProviders(ctx context.Context, limit int) ([]TenantModelProvider, error) {
	providers, err := s.catalogRepo().ListActiveProviders(ctx, limit)
	if err != nil {
		return nil, err
	}
	result := make([]TenantModelProvider, 0, len(providers))
	for _, provider := range providers {
		result = append(result, TenantModelProvider{
			ID: provider.ID, Name: provider.Name, ProviderType: provider.ProviderType, Status: provider.Status,
		})
	}
	return result, nil
}

func (s *Service) UpdateProvider(ctx context.Context, id uuid.UUID, input UpdateProviderInput) (*ModelProvider, error) {
	if input.RetryCount != nil && (*input.RetryCount < 0 || *input.RetryCount > 100) {
		return nil, fmt.Errorf("%w: retry_count must be between 0 and 100", ErrValidation)
	}
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
		TimeoutMS:    provider.TimeoutMS,
		RetryCount:   provider.RetryCount,
	}
	if target.Model == "" {
		target.Model = defaultTestModel(provider.ProviderType)
	}
	adapter, err := s.adapterFor(target)
	if err != nil {
		return err
	}
	_, err = s.invokeAdapter(ctx, adapter, ProviderRequest{
		Model:     target.Model,
		Messages:  []Message{{Role: "user", Content: "ping"}},
		MaxTokens: 8,
	}, target.TimeoutMS, target.RetryCount)
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

func (s *Service) ListTenantModels(ctx context.Context, providerID *uuid.UUID, limit int) ([]TenantModel, error) {
	models, err := s.catalogRepo().ListActiveModels(ctx)
	if err != nil {
		return nil, err
	}
	limit = normalizeLimit(limit)
	result := make([]TenantModel, 0, min(limit, len(models)))
	for _, model := range models {
		if providerID != nil && model.ProviderID != *providerID {
			continue
		}
		result = append(result, TenantModel{
			ID: model.ID, ProviderID: model.ProviderID, ModelKey: model.ModelKey, DisplayName: model.DisplayName,
			ContextWindow: model.ContextWindow, MaxOutputTokens: model.MaxOutputTokens,
			Capabilities: append([]string(nil), model.Capabilities...), Status: model.Status,
		})
		if len(result) >= limit {
			break
		}
	}
	return result, nil
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
		TimeoutMS:    channel.TimeoutMS,
		RetryCount:   channel.RetryCount,
	}
	if target.Model == "" {
		target.Model = defaultTestModel(channel.ProviderType)
	}
	adapter, err := s.adapterFor(target)
	if err != nil {
		return err
	}
	_, err = s.invokeAdapter(ctx, adapter, ProviderRequest{Model: target.Model, Messages: []Message{{Role: "user", Content: "ping"}}, MaxTokens: 8}, target.TimeoutMS, target.RetryCount)
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

func (s *Service) CreateAccessToken(ctx context.Context, input CreateAccessTokenInput) (*AccessToken, error) {
	if input.OrganizationID == uuid.Nil || strings.TrimSpace(input.Name) == "" {
		return nil, fmt.Errorf("%w: organization_id and name are required", ErrValidation)
	}
	plainToken, err := generateAccessToken()
	if err != nil {
		return nil, err
	}
	result, err := s.catalogRepo().CreateAccessToken(ctx, CreateAccessTokenStoreInput{
		OrganizationID:       input.OrganizationID,
		ModelGroupID:         input.ModelGroupID,
		Name:                 strings.TrimSpace(input.Name),
		TokenHash:            hashAccessToken(plainToken),
		MaskedToken:          maskSecret(plainToken),
		AllowedModels:        input.AllowedModels,
		AllowedModelPatterns: input.AllowedModelPatterns,
		AllowChannelOverride: input.AllowChannelOverride,
		QuotaAmount:          input.QuotaAmount,
		QuotaCurrency:        currencyOrDefault(input.QuotaCurrency),
		ExpiresAt:            input.ExpiresAt,
		Metadata:             input.Metadata,
	})
	if err != nil {
		return nil, err
	}
	result.PlainToken = plainToken
	return result, nil
}

func (s *Service) ListAccessTokens(ctx context.Context, organizationID *uuid.UUID, limit int) ([]AccessToken, error) {
	return s.catalogRepo().ListAccessTokens(ctx, organizationID, limit)
}

func (s *Service) CreateModelGroup(ctx context.Context, input CreateModelGroupInput) (*ModelGroup, error) {
	if strings.TrimSpace(input.Name) == "" {
		return nil, fmt.Errorf("%w: name is required", ErrValidation)
	}
	return s.catalogRepo().CreateModelGroup(ctx, input)
}

func (s *Service) ListModelGroups(ctx context.Context, organizationID *uuid.UUID, limit int) ([]ModelGroup, error) {
	return s.catalogRepo().ListModelGroups(ctx, organizationID, limit)
}

func (s *Service) CreateModelChannelAbility(ctx context.Context, input CreateModelChannelAbilityInput) (*ModelChannelAbility, error) {
	if input.ChannelID == uuid.Nil {
		return nil, fmt.Errorf("%w: channel_id is required", ErrValidation)
	}
	return s.catalogRepo().CreateModelChannelAbility(ctx, input)
}

func (s *Service) ListModelChannelAbilities(ctx context.Context, modelGroupID *uuid.UUID, limit int) ([]ModelChannelAbility, error) {
	return s.catalogRepo().ListModelChannelAbilities(ctx, modelGroupID, limit)
}

func (s *Service) GetGatewayBalance(ctx context.Context, organizationID uuid.UUID, currency string) (*GatewayBalance, error) {
	if organizationID == uuid.Nil {
		return nil, fmt.Errorf("%w: organization_id is required", ErrValidation)
	}
	return s.catalogRepo().GetGatewayBalance(ctx, organizationID, currencyOrDefault(currency))
}

func (s *Service) ListBalanceTransactions(ctx context.Context, organizationID uuid.UUID, limit int) ([]BalanceTransaction, error) {
	if organizationID == uuid.Nil {
		return nil, fmt.Errorf("%w: organization_id is required", ErrValidation)
	}
	return s.catalogRepo().ListBalanceTransactions(ctx, organizationID, limit)
}

func (s *Service) AdjustGatewayBalance(ctx context.Context, input AdjustGatewayBalanceInput) (*GatewayBalance, error) {
	if input.OrganizationID == uuid.Nil {
		return nil, fmt.Errorf("%w: organization_id is required", ErrValidation)
	}
	if strings.TrimSpace(input.Reason) == "" {
		return nil, fmt.Errorf("%w: reason is required", ErrValidation)
	}
	input.Currency = currencyOrDefault(input.Currency)
	input.Reason = strings.TrimSpace(input.Reason)
	return s.catalogRepo().AdjustGatewayBalance(ctx, input)
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

func (s *Service) AdapterCatalog() []AdapterDescriptor {
	return []AdapterDescriptor{
		{AdapterKey: ProviderOpenAI, DisplayName: "OpenAI", ProviderType: ProviderOpenAI, AdapterMode: "native", DefaultBaseURL: defaultOpenAIBaseURL, SupportedModes: []string{"chat", "stream", "tools"}},
		{AdapterKey: ProviderAnthropic, DisplayName: "Anthropic", ProviderType: ProviderAnthropic, AdapterMode: "native", DefaultBaseURL: defaultAnthropicBaseURL, SupportedModes: []string{"chat", "stream", "tools"}, RequiresNativeIO: true},
		{AdapterKey: ProviderGemini, DisplayName: "Gemini", ProviderType: ProviderGemini, AdapterMode: "native", DefaultBaseURL: defaultGeminiBaseURL, SupportedModes: []string{"chat", "stream", "tools"}, RequiresNativeIO: true},
		{AdapterKey: "deepseek", DisplayName: "DeepSeek", ProviderType: ProviderOpenAI, AdapterMode: "openai_compatible", DefaultBaseURL: "https://api.deepseek.com", SupportedModels: []string{"deepseek-chat", "deepseek-reasoner"}, SupportedModes: []string{"chat", "stream"}},
		{AdapterKey: "moonshot", DisplayName: "Moonshot", ProviderType: ProviderOpenAI, AdapterMode: "openai_compatible", DefaultBaseURL: "https://api.moonshot.cn/v1", SupportedModes: []string{"chat", "stream"}},
		{AdapterKey: "openrouter", DisplayName: "OpenRouter", ProviderType: ProviderOpenAI, AdapterMode: "openai_compatible", DefaultBaseURL: "https://openrouter.ai/api/v1", SupportedModes: []string{"chat", "stream"}},
		{AdapterKey: "doubao", DisplayName: "Doubao", ProviderType: ProviderOpenAI, AdapterMode: "openai_compatible", SupportedModes: []string{"chat", "stream"}},
		{AdapterKey: "siliconflow", DisplayName: "SiliconFlow", ProviderType: ProviderOpenAI, AdapterMode: "openai_compatible", DefaultBaseURL: "https://api.siliconflow.cn/v1", SupportedModes: []string{"chat", "stream"}},
		{AdapterKey: "ollama", DisplayName: "Ollama", ProviderType: ProviderOpenAI, AdapterMode: "openai_compatible", DefaultBaseURL: "http://localhost:11434/v1", SupportedModes: []string{"chat", "stream", "embeddings"}},
	}
}

func (s *Service) invokeAdapter(ctx context.Context, adapter ProviderAdapter, request ProviderRequest, providerTimeoutMS int, providerRetryCount int) (*ProviderResponse, error) {
	providerCtx, cancel := context.WithTimeout(ctx, effectiveTimeout(s.invokeTimeout, providerTimeoutMS))
	defer cancel()
	var response *ProviderResponse
	err := s.retryProviderCall(providerCtx, providerRetryCount, func() error {
		var err error
		response, err = adapter.Invoke(providerCtx, request)
		return err
	})
	return response, err
}

func (s *Service) streamAdapter(ctx context.Context, adapter ProviderAdapter, request ProviderRequest, providerRetryCount int) (<-chan StreamEvent, error) {
	var events <-chan StreamEvent
	err := s.retryProviderCall(ctx, providerRetryCount, func() error {
		var err error
		events, err = adapter.Stream(ctx, request)
		return err
	})
	return events, err
}

func (s *Service) retryProviderCall(ctx context.Context, providerRetryCount int, call func() error) error {
	retries := effectiveRetryCount(providerRetryCount, s.maxRetries)
	for attempt := 0; ; attempt++ {
		err := call()
		if err == nil {
			return err
		}
		statusCode, retryable := retryableProviderStatus(err)
		if !retryable || attempt >= retries {
			if retryable && retries > 0 {
				s.recordMetric(ctx, observability.MetricReliability, "ai_provider_retry_exhausted", nil, "ai_provider_call", 1, map[string]any{
					"status_code": statusCode, "attempts": attempt + 1,
				})
			}
			return err
		}
		s.recordMetric(ctx, observability.MetricReliability, "ai_provider_retry", nil, "ai_provider_call", 1, map[string]any{
			"status_code": statusCode, "retry_attempt": attempt + 1,
		})
		delay := s.retryBaseDelay * time.Duration(1<<min(attempt, 4))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
}

func effectiveRetryCount(providerRetryCount int, deploymentLimit int) int {
	if providerRetryCount <= 0 || deploymentLimit <= 0 {
		return 0
	}
	return min(providerRetryCount, deploymentLimit)
}

func retryableProviderStatus(err error) (int, bool) {
	var providerErr *ProviderError
	if !errors.As(err, &providerErr) {
		return 0, false
	}
	switch providerErr.StatusCode {
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return providerErr.StatusCode, true
	default:
		return providerErr.StatusCode, false
	}
}

func effectiveTimeout(deploymentLimit time.Duration, providerTimeoutMS int) time.Duration {
	providerLimit := time.Duration(providerTimeoutMS) * time.Millisecond
	if providerLimit <= 0 {
		return deploymentLimit
	}
	if deploymentLimit <= 0 || providerLimit < deploymentLimit {
		return providerLimit
	}
	return deploymentLimit
}

func (s *Service) recordingStream(ctx context.Context, cancel context.CancelFunc, invocationID uuid.UUID, target ResolvedModel, input InvokeInput, serviceTier string, reasoningEffort string, started time.Time, events <-chan StreamEvent, trace *observability.Trace, reservation *BalanceReservation, accessToken *AccessTokenContext) <-chan StreamEvent {
	out := make(chan StreamEvent, 1)
	go func() {
		defer cancel()
		defer close(out)
		usage := TokenUsage{}
		failed := ""
		for {
			select {
			case <-ctx.Done():
				s.recordCancelledStream(ctx.Err(), invocationID, target, input, serviceTier, reasoningEffort, usage, started, trace, reservation, accessToken)
				select {
				case out <- StreamEvent{Type: "error", Error: ctx.Err().Error(), Done: true}:
				default:
				}
				return
			case event, ok := <-events:
				if !ok {
					goto complete
				}
				if event.Usage.InputTokens > 0 || event.Usage.OutputTokens > 0 {
					usage = event.Usage
				}
				if event.Error != "" {
					failed = event.Error
				}
				select {
				case <-ctx.Done():
					s.recordCancelledStream(ctx.Err(), invocationID, target, input, serviceTier, reasoningEffort, usage, started, trace, reservation, accessToken)
					select {
					case out <- StreamEvent{Type: "error", Error: ctx.Err().Error(), Done: true}:
					default:
					}
					return
				case out <- event:
				}
			}
		}
	complete:
		breakdown := CalculateCostBreakdown(usage, target.Price, target.RateMultiplier, serviceTier)
		cost := breakdown.ActualCost
		currency := currencyOrDefault(target.Currency)
		_ = s.repo.CreateUsageLedger(context.Background(), CreateUsageLedgerInput{
			InvocationID:        invocationID,
			AccessTokenID:       input.AccessTokenID,
			ModelGroupID:        input.ModelGroupID,
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
			Metadata:            accessTokenLedgerMetadata(input),
		})
		durationMS := int(time.Since(started).Milliseconds())
		if err := s.settleStreamBalance(reservation, accessToken, cost, currency, firstNonEmpty(failed, "ai_stream_settle")); err != nil {
			failed = err.Error()
		}
		if failed != "" {
			_ = s.repo.FailInvocation(context.Background(), invocationID, FailInvocationInput{ErrorType: "provider_error", Message: failed, DurationMS: durationMS})
			_ = s.repo.ReleaseChannel(context.Background(), target.ChannelID, cost)
			s.recordInvocationSpan(context.Background(), trace, observability.SpanAIStream, invocationID, target, input.Attribution, StatusFailed, failed, durationMS)
			s.completeObservationTrace(context.Background(), trace, observability.TraceFailed)
			return
		}
		_ = s.repo.ReleaseChannel(context.Background(), target.ChannelID, cost)
		s.recordCostLedger(context.Background(), invocationID, target, input.Attribution, usage, breakdown, currency, ModeStream, StatusCompleted, "")
		_ = s.repo.CompleteInvocation(context.Background(), invocationID, CompleteInvocationInput{
			Usage:         usage,
			CostAmount:    cost,
			CostBreakdown: breakdown,
			Currency:      currency,
			DurationMS:    durationMS,
		})
		s.recordInvocationSpan(context.Background(), trace, observability.SpanAIStream, invocationID, target, input.Attribution, StatusCompleted, "", durationMS)
		s.recordAIMetrics(context.Background(), invocationID, usage, cost, currency, map[string]any{"mode": ModeStream, "provider_type": target.ProviderType, "model": target.Model})
		s.completeObservationTrace(context.Background(), trace, observability.TraceComplete)
	}()
	return out
}

func (s *Service) recordCancelledStream(cause error, invocationID uuid.UUID, target ResolvedModel, input InvokeInput, serviceTier string, reasoningEffort string, usage TokenUsage, started time.Time, trace *observability.Trace, reservation *BalanceReservation, accessToken *AccessTokenContext) {
	errorType := "cancelled"
	metricName := "ai_stream_disconnect"
	if errors.Is(cause, context.DeadlineExceeded) {
		errorType = "timeout"
		metricName = "ai_stream_timeout"
	}
	durationMS := int(time.Since(started).Milliseconds())
	breakdown := CalculateCostBreakdown(usage, target.Price, target.RateMultiplier, serviceTier)
	cost := breakdown.ActualCost
	currency := currencyOrDefault(target.Currency)
	_ = s.repo.CreateUsageLedger(context.Background(), CreateUsageLedgerInput{
		InvocationID:        invocationID,
		AccessTokenID:       input.AccessTokenID,
		ModelGroupID:        input.ModelGroupID,
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
		Reason:              cause.Error(),
		Metadata:            accessTokenLedgerMetadata(input),
	})
	if err := s.settleStreamBalance(reservation, accessToken, cost, currency, cause.Error()); err != nil {
		cause = err
	}
	_ = s.repo.FailInvocation(context.Background(), invocationID, FailInvocationInput{ErrorType: errorType, Message: cause.Error(), DurationMS: durationMS, Cancelled: true})
	_ = s.repo.ReleaseChannel(context.Background(), target.ChannelID, cost)
	if cost > 0 {
		s.recordCostLedger(context.Background(), invocationID, target, input.Attribution, usage, breakdown, currency, ModeStream, StatusCancelled, cause.Error())
	}
	s.recordInvocationSpan(context.Background(), trace, observability.SpanAIStream, invocationID, target, input.Attribution, StatusCancelled, cause.Error(), durationMS)
	s.recordMetric(context.Background(), observability.MetricReliability, metricName, &invocationID, "ai_invocation", 1, map[string]any{"provider_type": target.ProviderType, "model": target.Model})
	s.completeObservationTrace(context.Background(), trace, observability.TraceFailed)
}

func (s *Service) settleStreamBalance(reservation *BalanceReservation, accessToken *AccessTokenContext, cost float64, currency string, reason string) error {
	if reservation == nil || accessToken == nil {
		return nil
	}
	return s.repo.SettleAccessTokenBalance(context.Background(), SettleBalanceInput{
		ReservationID: reservation.ID,
		AccessTokenID: accessToken.ID,
		ActualAmount:  cost,
		Currency:      currency,
		Reason:        reason,
	})
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

func (s *Service) prepareAccessTokenInvocation(ctx context.Context, token string, input InvokeInput) (AccessTokenContext, InvokeInput, error) {
	accessToken, err := s.AuthenticateAccessToken(ctx, token)
	if err != nil {
		return AccessTokenContext{}, input, err
	}
	if !accessTokenAllowsModel(accessToken, input.Model) {
		return AccessTokenContext{}, input, fmt.Errorf("%w: model %q is not allowed for this ai access token", ErrForbidden, input.Model)
	}
	orgID := accessToken.OrganizationID
	input.Attribution.OrganizationID = &orgID
	input.ProviderType = firstNonEmpty(input.ProviderType, ProviderOpenAI)
	input.AccessTokenID = &accessToken.ID
	input.ModelGroupID = accessToken.ModelGroupID
	input.Attribution.SourceSurface = firstNonEmpty(input.Attribution.SourceSurface, "organization_api")
	if !accessToken.AllowChannelOverride {
		input.PreferredChannelID = nil
	}
	return accessToken, input, nil
}

func accessTokenAllowsModel(token AccessTokenContext, model string) bool {
	model = strings.TrimSpace(model)
	if model == "" {
		return true
	}
	patterns := append([]string{}, token.AllowedModels...)
	patterns = append(patterns, token.AllowedModelPatterns...)
	if len(patterns) == 0 {
		return true
	}
	for _, pattern := range patterns {
		if wildcardMatch(pattern, model) {
			return true
		}
	}
	return false
}

func modelAllowedByAbilities(abilities []ModelChannelAbility, model string) bool {
	for _, ability := range abilities {
		if !ability.Enabled {
			continue
		}
		if requested := strings.TrimSpace(ability.RequestedModel); requested != "" && !strings.EqualFold(requested, model) {
			continue
		}
		if wildcardMatch(firstNonEmpty(ability.ModelPattern, "*"), model) {
			return true
		}
	}
	return false
}

func estimateInvocationUsage(input InvokeInput, target ResolvedModel) TokenUsage {
	inputTokens := 0
	for _, message := range input.Messages {
		inputTokens += estimateTextTokens(message.Content)
	}
	outputTokens := maxTokens(input.MaxTokens, target.MaxOutputTokens)
	if outputTokens == 0 {
		outputTokens = 1024
	}
	return TokenUsage{InputTokens: inputTokens, OutputTokens: outputTokens}
}

func estimateTextTokens(value string) int {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0
	}
	runes := len([]rune(trimmed))
	tokens := runes / 4
	if runes%4 != 0 {
		tokens++
	}
	if tokens == 0 {
		return 1
	}
	return tokens
}

func reservedAmount(reservation *BalanceReservation) float64 {
	if reservation == nil {
		return 0
	}
	return reservation.ReservedAmount
}

func accessTokenLedgerMetadata(input InvokeInput) map[string]any {
	metadata := copyMap(input.Metadata)
	if input.AccessTokenID != nil {
		metadata["access_token_id"] = input.AccessTokenID.String()
	}
	if input.ModelGroupID != nil {
		metadata["model_group_id"] = input.ModelGroupID.String()
	}
	return metadata
}

func generateAccessToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate ai access token: %w", err)
	}
	return "ak-meta-" + base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func validateProviderInput(input CreateProviderInput) error {
	if input.Name == "" || input.ProviderType == "" || input.APIKey == "" {
		return fmt.Errorf("%w: name, provider_type, and api_key are required", ErrValidation)
	}
	if input.RetryCount < 0 || input.RetryCount > 100 {
		return fmt.Errorf("%w: retry_count must be between 0 and 100", ErrValidation)
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
