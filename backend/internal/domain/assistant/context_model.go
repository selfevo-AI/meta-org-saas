package assistant

import (
	"context"

	"github.com/google/uuid"
)

const (
	ContextScopeSaaS         = "saas"
	ContextScopeOrganization = "organization"
	ContextScopeModule       = "module"

	ContextSourceJSON = "json"
	ContextSourceYAML = "yaml"
	ContextSourceCSV  = "csv"
	ContextSourceXLSX = "xlsx"

	DictionaryStatusDraft      = "draft"
	DictionaryStatusAIReviewed = "ai_reviewed"
	DictionaryStatusApproved   = "approved"
	DictionaryStatusActive     = "active"
	DictionaryStatusRejected   = "rejected"
	DictionaryStatusArchived   = "archived"
)

type DictionaryImportModel struct {
	VersionKey                string                          `json:"version_key" yaml:"version_key"`
	SourceType                string                          `json:"source_type" yaml:"source_type"`
	SourceName                string                          `json:"source_name" yaml:"source_name"`
	ScopeLevel                string                          `json:"scope_level" yaml:"scope_level"`
	OrganizationID            *uuid.UUID                      `json:"organization_id,omitempty" yaml:"organization_id,omitempty"`
	ModuleKey                 string                          `json:"module_key" yaml:"module_key"`
	Domains                   []ContextBusinessDomainInput    `json:"domains" yaml:"domains"`
	Entities                  []ContextEntityInput            `json:"entities" yaml:"entities"`
	Fields                    []ContextFieldInput             `json:"fields" yaml:"fields"`
	Relationships             []ContextRelationshipInput      `json:"relationships" yaml:"relationships"`
	WorkflowContextProfiles   []WorkflowContextProfileInput   `json:"workflow_context_profiles" yaml:"workflow_context_profiles"`
	FinanceValidationProfiles []FinanceValidationProfileInput `json:"finance_validation_profiles" yaml:"finance_validation_profiles"`
	Permissions               []ContextPermissionInput        `json:"permissions" yaml:"permissions"`
	MigrationIntents          []ContextMigrationIntentInput   `json:"migration_intents" yaml:"migration_intents"`
}

type ContextBusinessDomainInput struct {
	ModuleKey  string         `json:"module_key" yaml:"module_key"`
	Name       string         `json:"name" yaml:"name"`
	ScopeLevel string         `json:"scope_level" yaml:"scope_level"`
	Metadata   map[string]any `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

type ContextEntityInput struct {
	EntityKey   string         `json:"entity_key" yaml:"entity_key"`
	ModuleKey   string         `json:"module_key" yaml:"module_key"`
	DisplayName string         `json:"display_name" yaml:"display_name"`
	Description string         `json:"description" yaml:"description"`
	Metadata    map[string]any `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

type ContextFieldInput struct {
	EntityKey         string         `json:"entity_key" yaml:"entity_key"`
	FieldKey          string         `json:"field_key" yaml:"field_key"`
	DisplayName       string         `json:"display_name" yaml:"display_name"`
	DataType          string         `json:"data_type" yaml:"data_type"`
	SemanticType      string         `json:"semantic_type" yaml:"semantic_type"`
	SensitivityLevel  string         `json:"sensitivity_level" yaml:"sensitivity_level"`
	BaseWeight        float64        `json:"base_weight" yaml:"base_weight"`
	IsFinanceField    bool           `json:"is_finance_field" yaml:"is_finance_field"`
	IsWorkflowField   bool           `json:"is_workflow_field" yaml:"is_workflow_field"`
	IsGovernanceField bool           `json:"is_governance_field" yaml:"is_governance_field"`
	MaskStrategy      string         `json:"mask_strategy" yaml:"mask_strategy"`
	TableName         string         `json:"table_name" yaml:"table_name"`
	ColumnName        string         `json:"column_name" yaml:"column_name"`
	Metadata          map[string]any `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

type ContextRelationshipInput struct {
	FromEntityKey string         `json:"from_entity_key" yaml:"from_entity_key"`
	ToEntityKey   string         `json:"to_entity_key" yaml:"to_entity_key"`
	RelationKey   string         `json:"relation_key" yaml:"relation_key"`
	JoinPath      []string       `json:"join_path" yaml:"join_path"`
	Metadata      map[string]any `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

type WorkflowContextProfileInput struct {
	ModuleKey        string             `json:"module_key" yaml:"module_key"`
	WorkflowStage    string             `json:"workflow_stage" yaml:"workflow_stage"`
	FieldMultipliers map[string]float64 `json:"field_multipliers" yaml:"field_multipliers"`
}

type FinanceValidationProfileInput struct {
	ModuleKey          string   `json:"module_key" yaml:"module_key"`
	FieldKey           string   `json:"field_key" yaml:"field_key"`
	RequiredStatus     []string `json:"required_status" yaml:"required_status"`
	UnverifiedAsSignal bool     `json:"unverified_as_signal" yaml:"unverified_as_signal"`
}

type ContextPermissionInput struct {
	ModuleKey         string   `json:"module_key" yaml:"module_key"`
	EntityKey         string   `json:"entity_key" yaml:"entity_key"`
	FieldKey          string   `json:"field_key" yaml:"field_key"`
	AllowedActorTypes []string `json:"allowed_actor_types" yaml:"allowed_actor_types"`
	MinimumTier       string   `json:"minimum_tier" yaml:"minimum_tier"`
}

type ContextMigrationIntentInput struct {
	IntentType string `json:"intent_type" yaml:"intent_type"`
	EntityKey  string `json:"entity_key" yaml:"entity_key"`
	FieldKey   string `json:"field_key" yaml:"field_key"`
	Reason     string `json:"reason" yaml:"reason"`
}

type ContextRuleRecord struct {
	ID                  uuid.UUID      `json:"id"`
	DictionaryVersionID uuid.UUID      `json:"dictionary_version_id"`
	ModuleKey           string         `json:"module_key"`
	EntityKey           string         `json:"entity_key"`
	FieldKey            string         `json:"field_key"`
	RuleType            string         `json:"rule_type"`
	Rule                map[string]any `json:"rule"`
	Status              string         `json:"status"`
}

type ContextRuleSource interface {
	ListActiveContextRules(context.Context, ContextRequest) ([]ContextRuleRecord, error)
}

type ContextDiagnosticsRepository interface {
	GetContextPackage(context.Context, uuid.UUID, *uuid.UUID) (*ContextPackage, error)
	GetContextHealth(context.Context, *uuid.UUID) (*ContextHealthSummary, error)
}

type ContextRequest struct {
	SessionID      uuid.UUID  `json:"session_id,omitempty"`
	ActorID        uuid.UUID  `json:"actor_id,omitempty"`
	ActorType      string     `json:"actor_type,omitempty"`
	OrganizationID *uuid.UUID `json:"organization_id,omitempty"`
	ModuleKey      string     `json:"module_key,omitempty"`
	WorkflowID     *uuid.UUID `json:"workflow_id,omitempty"`
	TaskID         *uuid.UUID `json:"task_id,omitempty"`
	TargetType     string     `json:"target_type,omitempty"`
	TargetID       *uuid.UUID `json:"target_id,omitempty"`
	Intent         string     `json:"intent,omitempty"`
	Mode           string     `json:"mode,omitempty"`
	RiskLevel      string     `json:"risk_level,omitempty"`
	TokenBudget    int        `json:"token_budget,omitempty"`
}

type ContextItem struct {
	EntityKey       string         `json:"entity_key"`
	FieldKey        string         `json:"field_key"`
	RecordID        string         `json:"record_id"`
	Value           any            `json:"value"`
	Weight          float64        `json:"weight"`
	EstimatedTokens int            `json:"estimated_tokens"`
	ValidationState string         `json:"validation_state"`
	Source          string         `json:"source"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

type ContextOmission struct {
	EntityKey string `json:"entity_key"`
	FieldKey  string `json:"field_key"`
	Reason    string `json:"reason"`
}

type ContextPackage struct {
	ID                  uuid.UUID          `json:"id"`
	SessionID           uuid.UUID          `json:"session_id"`
	DictionaryVersionID *uuid.UUID         `json:"dictionary_version_id,omitempty"`
	AttentionCore       []ContextItem      `json:"attention_core"`
	SupportingContext   []ContextItem      `json:"supporting_context"`
	RiskAndSignals      []ContextItem      `json:"risk_and_signals"`
	Omissions           []ContextOmission  `json:"omissions"`
	Weights             map[string]float64 `json:"weights"`
	Validations         map[string]any     `json:"validations"`
	Provenance          map[string]any     `json:"provenance"`
	TokenBudget         int                `json:"token_budget"`
}

type ContextPackageSummary struct {
	AttentionCoreCount     int    `json:"attention_core_count"`
	SupportingContextCount int    `json:"supporting_context_count"`
	RiskSignalCount        int    `json:"risk_signal_count"`
	OmissionCount          int    `json:"omission_count"`
	TokenBudget            int    `json:"token_budget"`
	EstimatedTokens        int    `json:"estimated_tokens"`
	Source                 string `json:"source"`
}

type ContextPackageDiagnostic struct {
	ID                  uuid.UUID             `json:"id"`
	SessionID           uuid.UUID             `json:"session_id"`
	DictionaryVersionID *uuid.UUID            `json:"dictionary_version_id,omitempty"`
	Summary             ContextPackageSummary `json:"summary"`
	AttentionCore       []ContextItem         `json:"attention_core"`
	SupportingContext   []ContextItem         `json:"supporting_context"`
	RiskAndSignals      []ContextItem         `json:"risk_and_signals"`
	Omissions           []ContextOmission     `json:"omissions"`
	Weights             map[string]float64    `json:"weights"`
	Validations         map[string]any        `json:"validations"`
	Provenance          map[string]any        `json:"provenance"`
}

type ContextHealthSummary struct {
	OrganizationID           *uuid.UUID     `json:"organization_id,omitempty"`
	ActiveRuleCount          int            `json:"active_rule_count"`
	StrictModules            []string       `json:"strict_modules"`
	StrictModuleCoverage     map[string]int `json:"strict_module_coverage"`
	MissingStrictModules     []string       `json:"missing_strict_modules"`
	RecentPackageCount       int            `json:"recent_package_count"`
	FallbackPackageCount     int            `json:"fallback_package_count"`
	ContextBuildFailureCount int            `json:"context_build_failure_count"`
	PendingProposalCount     int            `json:"pending_proposal_count"`
	ApprovedProposalCount    int            `json:"approved_proposal_count"`
	AppliedProposalCount     int            `json:"applied_proposal_count"`
	ToolApprovalBacklog      int            `json:"tool_approval_backlog"`
}

func (p ContextPackage) AttentionCoreTokens() int {
	total := 0
	for _, item := range p.AttentionCore {
		total += item.EstimatedTokens
	}
	return total
}

func (p ContextPackage) TotalEstimatedTokens() int {
	total := p.AttentionCoreTokens()
	for _, item := range p.SupportingContext {
		total += item.EstimatedTokens
	}
	for _, item := range p.RiskAndSignals {
		total += item.EstimatedTokens
	}
	return total
}
