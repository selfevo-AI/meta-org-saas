package assistant

import (
	"time"

	"github.com/google/uuid"
)

const (
	ModeBusinessProcess = "business_process"
	ModeSelfEvolution   = "self_evolution"

	StatusIdle             = "idle"
	StatusRunning          = "running"
	StatusApprovalRequired = "approval_required"
	StatusCompleted        = "completed"
	StatusFailed           = "failed"

	StepLLM        = "llm"
	StepToolCall   = "tool_call"
	StepToolResult = "tool_result"
	StepMemory     = "memory"
	StepApproval   = "approval"
	StepContext    = "context"
	StepError      = "error"

	ProposalPending  = "pending"
	ProposalApplied  = "applied"
	ProposalRejected = "rejected"

	SkillDraft    = "draft"
	SkillActive   = "active"
	SkillArchived = "archived"

	SkillScopeSaaSGlobal   = "saas_global"
	SkillScopeOrganization = "organization"
	SkillScopeDeployment   = "deployment"

	SkillDeploymentSaaS       = "saas"
	SkillDeploymentOrgPrivate = "org_private"
	SkillDeploymentPrivate    = "private"
)

type Scope struct {
	ModuleKey            string     `json:"module_key"`
	OrganizationID       *uuid.UUID `json:"organization_id,omitempty"`
	DepartmentID         *uuid.UUID `json:"department_id,omitempty"`
	PositionID           *uuid.UUID `json:"position_id,omitempty"`
	PositionAssignmentID *uuid.UUID `json:"position_assignment_id,omitempty"`
	ProjectID            *uuid.UUID `json:"project_id,omitempty"`
	WorkflowID           *uuid.UUID `json:"workflow_id,omitempty"`
	TaskID               *uuid.UUID `json:"task_id,omitempty"`
}

type Session struct {
	ID                   uuid.UUID      `json:"id"`
	Title                string         `json:"title"`
	Mode                 string         `json:"mode"`
	ModuleKey            string         `json:"module_key"`
	Status               string         `json:"status"`
	ActorID              uuid.UUID      `json:"actor_id"`
	ActorType            string         `json:"actor_type"`
	AgentID              *uuid.UUID     `json:"agent_id,omitempty"`
	ProviderID           *uuid.UUID     `json:"provider_id,omitempty"`
	PreferredChannelID   *uuid.UUID     `json:"preferred_channel_id,omitempty"`
	ProviderType         string         `json:"provider_type"`
	Model                string         `json:"model"`
	ServiceTier          string         `json:"service_tier,omitempty"`
	ReasoningEffort      string         `json:"reasoning_effort,omitempty"`
	OrganizationID       *uuid.UUID     `json:"organization_id,omitempty"`
	DepartmentID         *uuid.UUID     `json:"department_id,omitempty"`
	PositionID           *uuid.UUID     `json:"position_id,omitempty"`
	PositionAssignmentID *uuid.UUID     `json:"position_assignment_id,omitempty"`
	ProjectID            *uuid.UUID     `json:"project_id,omitempty"`
	WorkflowID           *uuid.UUID     `json:"workflow_id,omitempty"`
	TaskID               *uuid.UUID     `json:"task_id,omitempty"`
	TargetType           string         `json:"target_type,omitempty"`
	TargetID             *uuid.UUID     `json:"target_id,omitempty"`
	WorkingMemory        map[string]any `json:"working_memory"`
	Metadata             map[string]any `json:"metadata"`
	LastError            string         `json:"last_error,omitempty"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
}

type Message struct {
	ID         uuid.UUID      `json:"id"`
	SessionID  uuid.UUID      `json:"session_id"`
	Role       string         `json:"role"`
	Content    string         `json:"content"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	ToolName   string         `json:"tool_name,omitempty"`
	Metadata   map[string]any `json:"metadata"`
	CreatedAt  time.Time      `json:"created_at"`
}

type Step struct {
	ID                   uuid.UUID      `json:"id"`
	SessionID            uuid.UUID      `json:"session_id"`
	ModuleKey            string         `json:"module_key"`
	OrganizationID       *uuid.UUID     `json:"organization_id,omitempty"`
	DepartmentID         *uuid.UUID     `json:"department_id,omitempty"`
	PositionID           *uuid.UUID     `json:"position_id,omitempty"`
	PositionAssignmentID *uuid.UUID     `json:"position_assignment_id,omitempty"`
	InvocationID         *uuid.UUID     `json:"invocation_id,omitempty"`
	ToolExecutionID      *uuid.UUID     `json:"tool_execution_id,omitempty"`
	ToolApprovalID       *uuid.UUID     `json:"tool_approval_id,omitempty"`
	StepType             string         `json:"step_type"`
	Status               string         `json:"status"`
	Summary              string         `json:"summary"`
	Data                 map[string]any `json:"data"`
	Turn                 int            `json:"turn"`
	CreatedAt            time.Time      `json:"created_at"`
}

type Memory struct {
	ID              uuid.UUID      `json:"id"`
	Scope           Scope          `json:"scope"`
	ActorID         *uuid.UUID     `json:"actor_id,omitempty"`
	ActorType       string         `json:"actor_type,omitempty"`
	MemoryType      string         `json:"memory_type"`
	Title           string         `json:"title"`
	Content         string         `json:"content"`
	Data            map[string]any `json:"data"`
	SourceSessionID *uuid.UUID     `json:"source_session_id,omitempty"`
	SourceStepID    *uuid.UUID     `json:"source_step_id,omitempty"`
	Confidence      float64        `json:"confidence"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

type CreateSessionInput struct {
	Title                string         `json:"title"`
	Mode                 string         `json:"mode"`
	ModuleKey            string         `json:"module_key"`
	AgentID              *uuid.UUID     `json:"agent_id,omitempty"`
	ProviderID           *uuid.UUID     `json:"provider_id,omitempty"`
	PreferredChannelID   *uuid.UUID     `json:"preferred_channel_id,omitempty"`
	ProviderType         string         `json:"provider_type,omitempty"`
	Model                string         `json:"model,omitempty"`
	ServiceTier          string         `json:"service_tier,omitempty"`
	ReasoningEffort      string         `json:"reasoning_effort,omitempty"`
	OrganizationID       *uuid.UUID     `json:"organization_id,omitempty"`
	DepartmentID         *uuid.UUID     `json:"department_id,omitempty"`
	PositionID           *uuid.UUID     `json:"position_id,omitempty"`
	PositionAssignmentID *uuid.UUID     `json:"position_assignment_id,omitempty"`
	ProjectID            *uuid.UUID     `json:"project_id,omitempty"`
	WorkflowID           *uuid.UUID     `json:"workflow_id,omitempty"`
	TaskID               *uuid.UUID     `json:"task_id,omitempty"`
	TargetType           string         `json:"target_type,omitempty"`
	TargetID             *uuid.UUID     `json:"target_id,omitempty"`
	AutoModel            bool           `json:"auto_model,omitempty"`
	Metadata             map[string]any `json:"metadata,omitempty"`
}

type RunInput struct {
	Message            string     `json:"message"`
	ProviderID         *uuid.UUID `json:"provider_id,omitempty"`
	PreferredChannelID *uuid.UUID `json:"preferred_channel_id,omitempty"`
	ProviderType       string     `json:"provider_type,omitempty"`
	Model              string     `json:"model,omitempty"`
	ServiceTier        string     `json:"service_tier,omitempty"`
	ReasoningEffort    string     `json:"reasoning_effort,omitempty"`
}

type ResumeInput struct {
	ToolApprovalID uuid.UUID `json:"tool_approval_id"`
}

type RunEvent struct {
	Type    string         `json:"type"`
	Session *Session       `json:"session,omitempty"`
	Step    *Step          `json:"step,omitempty"`
	Message *Message       `json:"message,omitempty"`
	Delta   string         `json:"delta,omitempty"`
	Error   string         `json:"error,omitempty"`
	Code    string         `json:"code,omitempty"`
	Done    bool           `json:"done,omitempty"`
	Data    map[string]any `json:"data,omitempty"`
}

type CreateMemoryInput struct {
	Scope           Scope
	ActorID         *uuid.UUID
	ActorType       string
	MemoryType      string
	Title           string
	Content         string
	Data            map[string]any
	SourceSessionID *uuid.UUID
	SourceStepID    *uuid.UUID
	Confidence      float64
}

type ModuleDefault struct {
	ID                 uuid.UUID      `json:"id"`
	ModuleKey          string         `json:"module_key"`
	TargetType         string         `json:"target_type"`
	AgentID            *uuid.UUID     `json:"agent_id,omitempty"`
	ProviderID         *uuid.UUID     `json:"provider_id,omitempty"`
	PreferredChannelID *uuid.UUID     `json:"preferred_channel_id,omitempty"`
	ProviderType       string         `json:"provider_type"`
	Model              string         `json:"model"`
	ServiceTier        string         `json:"service_tier,omitempty"`
	ReasoningEffort    string         `json:"reasoning_effort,omitempty"`
	Metadata           map[string]any `json:"metadata"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

type Proposal struct {
	ID           uuid.UUID      `json:"id"`
	SessionID    uuid.UUID      `json:"session_id"`
	ModuleKey    string         `json:"module_key"`
	TargetType   string         `json:"target_type"`
	TargetID     *uuid.UUID     `json:"target_id,omitempty"`
	ProposalType string         `json:"proposal_type"`
	Title        string         `json:"title"`
	Summary      string         `json:"summary"`
	Payload      map[string]any `json:"payload"`
	Status       string         `json:"status"`
	ReviewerID   *uuid.UUID     `json:"reviewer_id,omitempty"`
	ReviewReason string         `json:"review_reason,omitempty"`
	ApplyResult  map[string]any `json:"apply_result"`
	ErrorMessage string         `json:"error_message,omitempty"`
	SourceStepID *uuid.UUID     `json:"source_step_id,omitempty"`
	AppliedAt    *time.Time     `json:"applied_at,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type CreateProposalInput struct {
	SessionID    uuid.UUID
	ModuleKey    string
	TargetType   string
	TargetID     *uuid.UUID
	ProposalType string
	Title        string
	Summary      string
	Payload      map[string]any
	SourceStepID *uuid.UUID
}

type BusinessSkill struct {
	ID                  uuid.UUID        `json:"id"`
	SkillKey            string           `json:"skill_key"`
	ScopeLevel          string           `json:"scope_level"`
	DeploymentMode      string           `json:"deployment_mode"`
	OrganizationID      *uuid.UUID       `json:"organization_id,omitempty"`
	OwnerUserID         *uuid.UUID       `json:"owner_user_id,omitempty"`
	ModuleKey           string           `json:"module_key"`
	TargetType          string           `json:"target_type"`
	BusinessFunctionKey string           `json:"business_function_key"`
	Name                string           `json:"name"`
	Description         string           `json:"description"`
	TriggerIntent       string           `json:"trigger_intent"`
	PromptTemplate      string           `json:"prompt_template"`
	ToolAllowlist       []string         `json:"tool_allowlist"`
	InputSchema         map[string]any   `json:"input_schema"`
	OutputSchema        map[string]any   `json:"output_schema"`
	SkillComponents     []SkillComponent `json:"skill_components"`
	PermissionPolicy    map[string]any   `json:"permission_policy"`
	ContextPolicy       map[string]any   `json:"context_policy"`
	PricingPolicy       map[string]any   `json:"pricing_policy"`
	ActivationPolicy    map[string]any   `json:"activation_policy"`
	Version             int              `json:"version"`
	Status              string           `json:"status"`
	CreatedBy           *uuid.UUID       `json:"created_by,omitempty"`
	CreatedByType       string           `json:"created_by_type,omitempty"`
	ReviewedBy          *uuid.UUID       `json:"reviewed_by,omitempty"`
	SourceSessionID     *uuid.UUID       `json:"source_session_id,omitempty"`
	Metadata            map[string]any   `json:"metadata"`
	CreatedAt           time.Time        `json:"created_at"`
	UpdatedAt           time.Time        `json:"updated_at"`
}

type SkillComponent struct {
	Key             string            `json:"key"`
	Label           map[string]string `json:"label,omitempty"`
	Weight          float64           `json:"weight"`
	Instruction     string            `json:"instruction"`
	RequiredContext []string          `json:"required_context,omitempty"`
	PermissionTags  []string          `json:"permission_tags,omitempty"`
}

type CreateBusinessSkillInput struct {
	SkillKey            string           `json:"skill_key,omitempty"`
	ScopeLevel          string           `json:"scope_level,omitempty"`
	DeploymentMode      string           `json:"deployment_mode,omitempty"`
	OrganizationID      *uuid.UUID       `json:"organization_id,omitempty"`
	OwnerUserID         *uuid.UUID       `json:"owner_user_id,omitempty"`
	ModuleKey           string           `json:"module_key"`
	TargetType          string           `json:"target_type,omitempty"`
	BusinessFunctionKey string           `json:"business_function_key,omitempty"`
	Name                string           `json:"name"`
	Description         string           `json:"description,omitempty"`
	TriggerIntent       string           `json:"trigger_intent,omitempty"`
	PromptTemplate      string           `json:"prompt_template"`
	ToolAllowlist       []string         `json:"tool_allowlist,omitempty"`
	InputSchema         map[string]any   `json:"input_schema,omitempty"`
	OutputSchema        map[string]any   `json:"output_schema,omitempty"`
	SkillComponents     []SkillComponent `json:"skill_components,omitempty"`
	PermissionPolicy    map[string]any   `json:"permission_policy,omitempty"`
	ContextPolicy       map[string]any   `json:"context_policy,omitempty"`
	PricingPolicy       map[string]any   `json:"pricing_policy,omitempty"`
	ActivationPolicy    map[string]any   `json:"activation_policy,omitempty"`
	SourceSessionID     *uuid.UUID       `json:"source_session_id,omitempty"`
	Metadata            map[string]any   `json:"metadata,omitempty"`
}

type SkillRun struct {
	ID            uuid.UUID      `json:"id"`
	SkillID       uuid.UUID      `json:"skill_id"`
	SessionID     *uuid.UUID     `json:"session_id,omitempty"`
	ModuleKey     string         `json:"module_key"`
	TargetType    string         `json:"target_type"`
	TargetID      *uuid.UUID     `json:"target_id,omitempty"`
	Input         map[string]any `json:"input"`
	Output        map[string]any `json:"output"`
	Status        string         `json:"status"`
	ErrorMessage  string         `json:"error_message,omitempty"`
	CreatedBy     *uuid.UUID     `json:"created_by,omitempty"`
	CreatedByType string         `json:"created_by_type,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	CompletedAt   *time.Time     `json:"completed_at,omitempty"`
}

type CreateSkillRunInput struct {
	SkillID       uuid.UUID
	SessionID     *uuid.UUID
	ModuleKey     string
	TargetType    string
	TargetID      *uuid.UUID
	Input         map[string]any
	Output        map[string]any
	Status        string
	ErrorMessage  string
	CreatedBy     *uuid.UUID
	CreatedByType string
}
