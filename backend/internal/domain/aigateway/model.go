package aigateway

import (
	"time"

	"github.com/google/uuid"
)

const (
	ProviderOpenAI    = "openai"
	ProviderAnthropic = "anthropic"
	ProviderGemini    = "gemini"
	ModeSync          = "sync"
	ModeStream        = "stream"
	StatusStarted     = "started"
	StatusStreaming   = "streaming"
	StatusCompleted   = "completed"
	StatusFailed      = "failed"
	StatusCancelled   = "cancelled"
)

type ModelProvider struct {
	ID             uuid.UUID      `json:"id"`
	Name           string         `json:"name"`
	ProviderType   string         `json:"provider_type"`
	BaseURL        string         `json:"base_url"`
	MaskedAPIKey   string         `json:"masked_api_key"`
	Status         string         `json:"status"`
	TimeoutMS      int            `json:"timeout_ms"`
	RetryCount     int            `json:"retry_count"`
	RiskLevel      string         `json:"risk_level"`
	Tags           []string       `json:"tags"`
	Metadata       map[string]any `json:"metadata"`
	LastTestStatus string         `json:"last_test_status"`
	LastTestError  string         `json:"last_test_error,omitempty"`
	LastTestedAt   *time.Time     `json:"last_tested_at,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type TenantModelProvider struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	ProviderType string    `json:"provider_type"`
	Status       string    `json:"status"`
}

type Model struct {
	ID              uuid.UUID      `json:"id"`
	ProviderID      uuid.UUID      `json:"provider_id"`
	ModelKey        string         `json:"model_key"`
	DisplayName     string         `json:"display_name"`
	ContextWindow   int            `json:"context_window"`
	MaxOutputTokens int            `json:"max_output_tokens"`
	Capabilities    []string       `json:"capabilities"`
	Status          string         `json:"status"`
	Metadata        map[string]any `json:"metadata"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

type TenantModel struct {
	ID              uuid.UUID `json:"id"`
	ProviderID      uuid.UUID `json:"provider_id"`
	ModelKey        string    `json:"model_key"`
	DisplayName     string    `json:"display_name"`
	ContextWindow   int       `json:"context_window"`
	MaxOutputTokens int       `json:"max_output_tokens"`
	Capabilities    []string  `json:"capabilities"`
	Status          string    `json:"status"`
}

type PriceVersion struct {
	ID                          uuid.UUID      `json:"id"`
	ModelID                     uuid.UUID      `json:"model_id"`
	InputPricePer1K             float64        `json:"input_price_per_1k"`
	OutputPricePer1K            float64        `json:"output_price_per_1k"`
	CacheCreationPricePer1K     float64        `json:"cache_creation_price_per_1k"`
	CacheReadPricePer1K         float64        `json:"cache_read_price_per_1k"`
	CacheCreation5mPricePer1K   float64        `json:"cache_creation_5m_price_per_1k"`
	CacheCreation1hPricePer1K   float64        `json:"cache_creation_1h_price_per_1k"`
	ImageOutputPricePer1K       float64        `json:"image_output_price_per_1k"`
	PriorityInputPricePer1K     float64        `json:"priority_input_price_per_1k"`
	PriorityOutputPricePer1K    float64        `json:"priority_output_price_per_1k"`
	PriorityCacheReadPricePer1K float64        `json:"priority_cache_read_price_per_1k"`
	LongContextThreshold        int            `json:"long_context_threshold"`
	LongContextInputMultiplier  float64        `json:"long_context_input_multiplier"`
	LongContextOutputMultiplier float64        `json:"long_context_output_multiplier"`
	BillingMode                 string         `json:"billing_mode"`
	PricingSource               string         `json:"pricing_source"`
	Currency                    string         `json:"currency"`
	Metadata                    map[string]any `json:"metadata"`
	EffectiveFrom               time.Time      `json:"effective_from"`
	EffectiveTo                 *time.Time     `json:"effective_to,omitempty"`
	CreatedAt                   time.Time      `json:"created_at"`
}

type ProviderChannel struct {
	ID                      uuid.UUID         `json:"id"`
	ProviderID              uuid.UUID         `json:"provider_id"`
	Name                    string            `json:"name"`
	BaseURL                 string            `json:"base_url"`
	MaskedAPIKey            string            `json:"masked_api_key"`
	OwnerType               string            `json:"owner_type,omitempty"`
	UserID                  *uuid.UUID        `json:"user_id,omitempty"`
	AgentID                 *uuid.UUID        `json:"agent_id,omitempty"`
	Status                  string            `json:"status"`
	Priority                int               `json:"priority"`
	ConcurrencyLimit        int               `json:"concurrency_limit"`
	InflightRequests        int               `json:"inflight_requests"`
	LoadFactor              int               `json:"load_factor"`
	RateMultiplier          float64           `json:"rate_multiplier"`
	QuotaAmount             float64           `json:"quota_amount"`
	QuotaUsed               float64           `json:"quota_used"`
	QuotaCurrency           string            `json:"quota_currency"`
	SupportedModelPatterns  []string          `json:"supported_model_patterns"`
	ModelMapping            map[string]string `json:"model_mapping"`
	HealthStatus            string            `json:"health_status"`
	LastError               string            `json:"last_error,omitempty"`
	LastResponseMS          int               `json:"last_response_ms"`
	SuccessCount            int64             `json:"success_count"`
	FailureCount            int64             `json:"failure_count"`
	ConsecutiveFailureCount int               `json:"consecutive_failure_count"`
	CircuitOpenUntil        *time.Time        `json:"circuit_open_until,omitempty"`
	AutoDisabledAt          *time.Time        `json:"auto_disabled_at,omitempty"`
	LastTestedAt            *time.Time        `json:"last_tested_at,omitempty"`
	LastUsedAt              *time.Time        `json:"last_used_at,omitempty"`
	Metadata                map[string]any    `json:"metadata"`
	CreatedAt               time.Time         `json:"created_at"`
	UpdatedAt               time.Time         `json:"updated_at"`
}

type RoutingRule struct {
	ID           uuid.UUID      `json:"id"`
	Name         string         `json:"name"`
	ProviderID   *uuid.UUID     `json:"provider_id,omitempty"`
	ChannelID    *uuid.UUID     `json:"channel_id,omitempty"`
	MatchScope   string         `json:"match_scope"`
	MatchValue   string         `json:"match_value"`
	ModelPattern string         `json:"model_pattern"`
	Priority     int            `json:"priority"`
	Status       string         `json:"status"`
	Metadata     map[string]any `json:"metadata"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type Attribution struct {
	OrganizationID *uuid.UUID `json:"organization_id,omitempty"`
	DepartmentID   *uuid.UUID `json:"department_id,omitempty"`
	ProjectID      *uuid.UUID `json:"project_id,omitempty"`
	RequirementID  *uuid.UUID `json:"requirement_id,omitempty"`
	WorkflowID     *uuid.UUID `json:"workflow_id,omitempty"`
	TaskID         *uuid.UUID `json:"task_id,omitempty"`
	AgentID        *uuid.UUID `json:"agent_id,omitempty"`
	UserID         *uuid.UUID `json:"user_id,omitempty"`
	CapabilityID   *uuid.UUID `json:"capability_id,omitempty"`
	SourceSurface  string     `json:"source_surface,omitempty"`
}

type Invocation struct {
	ID                    uuid.UUID      `json:"id"`
	ProviderID            uuid.UUID      `json:"provider_id"`
	ModelID               uuid.UUID      `json:"model_id"`
	ChannelID             *uuid.UUID     `json:"channel_id,omitempty"`
	Mode                  string         `json:"mode"`
	Status                string         `json:"status"`
	Attribution           Attribution    `json:"attribution"`
	RequestedModel        string         `json:"requested_model,omitempty"`
	UpstreamModel         string         `json:"upstream_model,omitempty"`
	ModelMappingChain     string         `json:"model_mapping_chain,omitempty"`
	ServiceTier           string         `json:"service_tier,omitempty"`
	ReasoningEffort       string         `json:"reasoning_effort,omitempty"`
	RequestHash           string         `json:"request_hash,omitempty"`
	ProviderRequestID     string         `json:"provider_request_id,omitempty"`
	InputTokens           int            `json:"input_tokens"`
	OutputTokens          int            `json:"output_tokens"`
	CacheCreationTokens   int            `json:"cache_creation_tokens"`
	CacheReadTokens       int            `json:"cache_read_tokens"`
	CacheCreation5mTokens int            `json:"cache_creation_5m_tokens"`
	CacheCreation1hTokens int            `json:"cache_creation_1h_tokens"`
	ImageOutputTokens     int            `json:"image_output_tokens"`
	EstimatedInputTokens  int            `json:"estimated_input_tokens"`
	EstimatedOutputTokens int            `json:"estimated_output_tokens"`
	CostAmount            float64        `json:"cost_amount"`
	CostBreakdown         CostBreakdown  `json:"cost_breakdown"`
	Currency              string         `json:"currency"`
	FirstTokenMS          int            `json:"first_token_ms"`
	DurationMS            int            `json:"duration_ms"`
	ErrorType             string         `json:"error_type,omitempty"`
	ErrorMessage          string         `json:"error_message,omitempty"`
	Metadata              map[string]any `json:"metadata"`
	CreatedAt             time.Time      `json:"created_at"`
	CompletedAt           *time.Time     `json:"completed_at,omitempty"`
}

type UsageLedgerEntry struct {
	ID                  uuid.UUID  `json:"id"`
	InvocationID        uuid.UUID  `json:"invocation_id"`
	ChannelID           *uuid.UUID `json:"channel_id,omitempty"`
	ModelPriceVersionID *uuid.UUID `json:"model_price_version_id,omitempty"`
	LedgerType          string     `json:"ledger_type"`
	Amount              float64    `json:"amount"`
	ActualAmount        float64    `json:"actual_amount"`
	Currency            string     `json:"currency"`
	InputTokens         int        `json:"input_tokens"`
	OutputTokens        int        `json:"output_tokens"`
	CacheCreationTokens int        `json:"cache_creation_tokens"`
	CacheReadTokens     int        `json:"cache_read_tokens"`
	PostedToProjectCost bool       `json:"posted_to_project_cost"`
	ProjectCostEntryID  *uuid.UUID `json:"project_cost_entry_id,omitempty"`
	FinanceExportLineID *uuid.UUID `json:"finance_export_line_id,omitempty"`
	Reason              string     `json:"reason,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
}

type Message struct {
	Role       string         `json:"role"`
	Content    string         `json:"content,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	ToolName   string         `json:"tool_name,omitempty"`
	ToolCalls  []ToolCall     `json:"tool_calls,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type ToolSpec struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Schema      map[string]any `json:"schema"`
}

type InvokeRequest struct {
	ProviderID         *uuid.UUID     `json:"provider_id,omitempty"`
	PreferredChannelID *uuid.UUID     `json:"preferred_channel_id,omitempty"`
	ModelID            *uuid.UUID     `json:"model_id,omitempty"`
	ModelKey           string         `json:"model_key,omitempty"`
	Messages           []Message      `json:"messages"`
	Temperature        *float64       `json:"temperature,omitempty"`
	TopP               *float64       `json:"top_p,omitempty"`
	MaxTokens          int            `json:"max_tokens,omitempty"`
	ServiceTier        string         `json:"service_tier,omitempty"`
	ReasoningEffort    string         `json:"reasoning_effort,omitempty"`
	Stream             bool           `json:"stream"`
	Tools              []ToolSpec     `json:"tools,omitempty"`
	Attribution        Attribution    `json:"attribution"`
	Metadata           map[string]any `json:"metadata,omitempty"`
}

type InvokeResponse struct {
	InvocationID  uuid.UUID        `json:"invocation_id"`
	Status        string           `json:"status"`
	Message       Message          `json:"message"`
	Usage         TokenUsage       `json:"usage"`
	CostAmount    float64          `json:"cost_amount"`
	CostBreakdown CostBreakdown    `json:"cost_breakdown"`
	Currency      string           `json:"currency"`
	ToolCalls     []ToolCallResult `json:"tool_calls,omitempty"`
}

type ToolCallResult struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Status    string         `json:"status"`
	Arguments map[string]any `json:"arguments,omitempty"`
	Result    map[string]any `json:"result,omitempty"`
	Error     string         `json:"error,omitempty"`
}
