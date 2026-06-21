package governance

import (
	"time"

	"github.com/google/uuid"
)

type Permission struct {
	ID          uuid.UUID `json:"id"`
	Level       int       `json:"level"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Behavior    string    `json:"behavior"`
	CreatedAt   time.Time `json:"created_at"`
}

type Principle struct {
	ID              uuid.UUID      `json:"id"`
	Name            string         `json:"name"`
	Description     string         `json:"description"`
	EvaluationLogic map[string]any `json:"evaluation_logic"`
	Priority        int            `json:"priority"`
	IsActive        bool           `json:"is_active"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

type ControlRule struct {
	ID               uuid.UUID      `json:"id"`
	PrincipleID      *uuid.UUID     `json:"principle_id,omitempty"`
	TargetEntityType string         `json:"target_entity_type"`
	TargetEntityID   *uuid.UUID     `json:"target_entity_id,omitempty"`
	Condition        map[string]any `json:"condition"`
	Action           string         `json:"action"`
	Priority         int            `json:"priority"`
	IsActive         bool           `json:"is_active"`
	CreatedAt        time.Time      `json:"created_at"`
}

type CreatePrincipleInput struct {
	Name            string         `json:"name"`
	Description     string         `json:"description"`
	EvaluationLogic map[string]any `json:"evaluation_logic,omitempty"`
	Priority        int            `json:"priority,omitempty"`
}

type CreateControlRuleInput struct {
	PrincipleID      *uuid.UUID     `json:"principle_id,omitempty"`
	TargetEntityType string         `json:"target_entity_type"`
	TargetEntityID   *uuid.UUID     `json:"target_entity_id,omitempty"`
	Condition        map[string]any `json:"condition,omitempty"`
	Action           string         `json:"action"`
	Priority         int            `json:"priority,omitempty"`
}

type PermissionCheckInput struct {
	UserID     uuid.UUID  `json:"user_id"`
	Action     string     `json:"action"`
	Resource   string     `json:"resource"`
	ResourceID *uuid.UUID `json:"resource_id,omitempty"`
}

type PermissionCheckResult struct {
	Allowed  bool   `json:"allowed"`
	Level    int    `json:"level"`
	Behavior string `json:"behavior"`
	Reason   string `json:"reason"`
}

type AccessDecisionInput struct {
	ActorID        uuid.UUID      `json:"actor_id"`
	ActorType      string         `json:"actor_type"`
	Action         string         `json:"action"`
	Resource       string         `json:"resource"`
	ResourceID     *uuid.UUID     `json:"resource_id,omitempty"`
	OrganizationID *uuid.UUID     `json:"organization_id,omitempty"`
	DepartmentID   *uuid.UUID     `json:"department_id,omitempty"`
	WorkflowID     *uuid.UUID     `json:"workflow_id,omitempty"`
	TaskID         *uuid.UUID     `json:"task_id,omitempty"`
	CapabilityID   *uuid.UUID     `json:"capability_id,omitempty"`
	RequiredLevel  string         `json:"required_level,omitempty"`
	RiskLevel      string         `json:"risk_level,omitempty"`
	WeightSnapshot *float64       `json:"weight_snapshot,omitempty"`
	Context        map[string]any `json:"context,omitempty"`
}

type AccessDecision struct {
	ID             uuid.UUID      `json:"id"`
	MasterKey      string         `json:"master_key"`
	ActorID        uuid.UUID      `json:"actor_id"`
	ActorType      string         `json:"actor_type"`
	Action         string         `json:"action"`
	Resource       string         `json:"resource"`
	ResourceID     *uuid.UUID     `json:"resource_id,omitempty"`
	OrganizationID *uuid.UUID     `json:"organization_id,omitempty"`
	DepartmentID   *uuid.UUID     `json:"department_id,omitempty"`
	WorkflowID     *uuid.UUID     `json:"workflow_id,omitempty"`
	TaskID         *uuid.UUID     `json:"task_id,omitempty"`
	CapabilityID   *uuid.UUID     `json:"capability_id,omitempty"`
	RequiredLevel  string         `json:"required_level"`
	RiskLevel      string         `json:"risk_level"`
	Decision       string         `json:"decision"`
	Allowed        bool           `json:"allowed"`
	Behavior       string         `json:"behavior"`
	Reason         string         `json:"reason"`
	MatchedRules   []string       `json:"matched_rules"`
	WeightSnapshot *float64       `json:"weight_snapshot,omitempty"`
	Context        map[string]any `json:"context"`
	CreatedAt      time.Time      `json:"created_at"`
}

type DataTable struct {
	TableName          string         `json:"table_name"`
	MasterTableName    string         `json:"master_table_name"`
	DetailTableName    string         `json:"detail_table_name"`
	KeyPrefix          string         `json:"key_prefix"`
	DisplayName        string         `json:"display_name"`
	Category           string         `json:"category"`
	IsBaseData         bool           `json:"is_base_data"`
	IsBusinessScenario bool           `json:"is_business_scenario"`
	Metadata           map[string]any `json:"metadata"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

type DataField struct {
	TableName        string         `json:"table_name"`
	FieldName        string         `json:"field_name"`
	DataType         string         `json:"data_type"`
	DisplayName      string         `json:"display_name"`
	IsMasterKey      bool           `json:"is_master_key"`
	IsSubKey         bool           `json:"is_sub_key"`
	IsVisibleDefault bool           `json:"is_visible_default"`
	PermissionLevel  string         `json:"permission_level"`
	DisplayOrder     int            `json:"display_order"`
	Metadata         map[string]any `json:"metadata"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

type UserFieldPreference struct {
	ActorID       string         `json:"actor_id"`
	TableName     string         `json:"table_name"`
	VisibleFields []string       `json:"visible_fields"`
	FieldOrder    []string       `json:"field_order"`
	FieldWidths   map[string]int `json:"field_widths"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

type UpsertUserFieldPreferenceInput struct {
	VisibleFields []string       `json:"visible_fields"`
	FieldOrder    []string       `json:"field_order"`
	FieldWidths   map[string]int `json:"field_widths"`
}

type UserUIPreference struct {
	ActorID       string         `json:"actor_id"`
	PreferenceKey string         `json:"preference_key"`
	Value         map[string]any `json:"value"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

type UpsertUserUIPreferenceInput struct {
	Value map[string]any `json:"value"`
}

type FieldPermissionRule struct {
	ID             uuid.UUID      `json:"id"`
	OrganizationID *uuid.UUID     `json:"organization_id,omitempty"`
	ScopeType      string         `json:"scope_type"`
	ScopeID        string         `json:"scope_id,omitempty"`
	TableName      string         `json:"table_name"`
	FieldName      string         `json:"field_name"`
	ActorType      string         `json:"actor_type"`
	ActorID        string         `json:"actor_id,omitempty"`
	RoleID         *uuid.UUID     `json:"role_id,omitempty"`
	Action         string         `json:"action"`
	Behavior       string         `json:"behavior"`
	RequiredLevel  string         `json:"required_level"`
	Reason         string         `json:"reason"`
	Priority       int            `json:"priority"`
	Status         string         `json:"status"`
	Metadata       map[string]any `json:"metadata"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type CreateFieldPermissionRuleInput struct {
	OrganizationID *uuid.UUID     `json:"organization_id,omitempty"`
	ScopeType      string         `json:"scope_type,omitempty"`
	ScopeID        string         `json:"scope_id,omitempty"`
	TableName      string         `json:"table_name"`
	FieldName      string         `json:"field_name,omitempty"`
	ActorType      string         `json:"actor_type,omitempty"`
	ActorID        string         `json:"actor_id,omitempty"`
	RoleID         *uuid.UUID     `json:"role_id,omitempty"`
	Action         string         `json:"action"`
	Behavior       string         `json:"behavior,omitempty"`
	RequiredLevel  string         `json:"required_level,omitempty"`
	Reason         string         `json:"reason,omitempty"`
	Priority       int            `json:"priority,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type FieldAccessCheckInput struct {
	ActorID        string     `json:"actor_id"`
	ActorType      string     `json:"actor_type"`
	OrganizationID *uuid.UUID `json:"organization_id,omitempty"`
	ScopeType      string     `json:"scope_type,omitempty"`
	ScopeID        string     `json:"scope_id,omitempty"`
	TableName      string     `json:"table_name"`
	FieldName      string     `json:"field_name,omitempty"`
	Action         string     `json:"action"`
}

type FieldAccessCheckResult struct {
	Allowed       bool   `json:"allowed"`
	Behavior      string `json:"behavior"`
	RequiredLevel string `json:"required_level"`
	Reason        string `json:"reason"`
}
