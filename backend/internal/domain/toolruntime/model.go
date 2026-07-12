package toolruntime

import (
	"time"

	"github.com/google/uuid"
)

const (
	SourceInternalAPI    = "internal_api"
	SourceInterfaceFile  = "interface_file"
	SourceManualApproval = "manual_approval"

	PolicyAuto    = "auto"
	PolicyNotify  = "notify"
	PolicyApprove = "approve"
	PolicyDeny    = "deny"

	ExecutionRequested        = "requested"
	ExecutionApprovalRequired = "approval_required"
	ExecutionApproved         = "approved"
	ExecutionRunning          = "running"
	ExecutionCompleted        = "completed"
	ExecutionRejected         = "rejected"
	ExecutionDenied           = "denied"
	ExecutionFailed           = "failed"

	ApprovalPending  = "pending"
	ApprovalApproved = "approved"
	ApprovalRejected = "rejected"
	ApprovalExpired  = "expired"

	ToolCategoryCoreData           = "core_data"
	ToolCategoryBusinessApproval   = "business_approval"
	ToolCategoryExecutionOperation = "execution_operation"

	ApprovalTierOrganizationCreator = "organization_creator"
	ApprovalTierReviewer            = "reviewer"
	ApprovalTierExecutor            = "executor"
)

type ToolDefinition struct {
	ID                   uuid.UUID      `json:"id"`
	Name                 string         `json:"name"`
	Description          string         `json:"description"`
	SourceType           string         `json:"source_type"`
	DefaultPolicy        string         `json:"default_policy"`
	RiskLevel            string         `json:"risk_level"`
	RequiredLevel        string         `json:"required_level"`
	ToolCategory         string         `json:"tool_category"`
	ApprovalTierRequired string         `json:"approval_tier_required"`
	InputSchema          map[string]any `json:"input_schema"`
	OutputSchema         map[string]any `json:"output_schema"`
	Metadata             map[string]any `json:"metadata"`
	IsActive             bool           `json:"is_active"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
}

type ToolExecution struct {
	ID                 uuid.UUID      `json:"id"`
	ToolID             uuid.UUID      `json:"tool_id"`
	InvocationID       *uuid.UUID     `json:"invocation_id,omitempty"`
	ActorID            uuid.UUID      `json:"actor_id"`
	ActorType          string         `json:"actor_type"`
	OrganizationID     *uuid.UUID     `json:"organization_id,omitempty"`
	DepartmentID       *uuid.UUID     `json:"department_id,omitempty"`
	ProjectID          *uuid.UUID     `json:"project_id,omitempty"`
	WorkflowID         *uuid.UUID     `json:"workflow_id,omitempty"`
	TaskID             *uuid.UUID     `json:"task_id,omitempty"`
	IdempotencyKey     string         `json:"idempotency_key,omitempty"`
	Policy             string         `json:"policy"`
	GovernanceDecision string         `json:"governance_decision,omitempty"`
	RequestedByHumanID *uuid.UUID     `json:"requested_by_human_id,omitempty"`
	Status             string         `json:"status"`
	Arguments          map[string]any `json:"arguments"`
	ResultSummary      string         `json:"result_summary"`
	Result             map[string]any `json:"result"`
	ErrorMessage       string         `json:"error_message,omitempty"`
	DurationMS         int            `json:"duration_ms"`
	CreatedAt          time.Time      `json:"created_at"`
	CompletedAt        *time.Time     `json:"completed_at,omitempty"`
}

type ToolApproval struct {
	ID                uuid.UUID  `json:"id"`
	ExecutionID       uuid.UUID  `json:"execution_id"`
	Status            string     `json:"status"`
	RequestedBy       *uuid.UUID `json:"requested_by,omitempty"`
	ReviewedBy        *uuid.UUID `json:"reviewed_by,omitempty"`
	ApprovedByHumanID *uuid.UUID `json:"approved_by_human_id,omitempty"`
	Reason            string     `json:"reason,omitempty"`
	ExpiresAt         *time.Time `json:"expires_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	ReviewedAt        *time.Time `json:"reviewed_at,omitempty"`
}

type InterfaceFile struct {
	ID        uuid.UUID      `json:"id"`
	Name      string         `json:"name"`
	FileType  string         `json:"file_type"`
	Content   string         `json:"content"`
	Metadata  map[string]any `json:"metadata"`
	CreatedBy *uuid.UUID     `json:"created_by,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type ExecuteToolInput struct {
	ToolName        string         `json:"tool_name"`
	InvocationID    *uuid.UUID     `json:"invocation_id,omitempty"`
	ActorID         uuid.UUID      `json:"actor_id"`
	ActorType       string         `json:"actor_type"`
	OrganizationID  *uuid.UUID     `json:"organization_id,omitempty"`
	DepartmentID    *uuid.UUID     `json:"department_id,omitempty"`
	ProjectID       *uuid.UUID     `json:"project_id,omitempty"`
	WorkflowID      *uuid.UUID     `json:"workflow_id,omitempty"`
	TaskID          *uuid.UUID     `json:"task_id,omitempty"`
	IdempotencyKey  string         `json:"idempotency_key,omitempty"`
	RequireApproval bool           `json:"require_approval,omitempty"`
	Arguments       map[string]any `json:"arguments"`
}

type ExecuteToolOutput struct {
	Execution *ToolExecution `json:"execution"`
	Approval  *ToolApproval  `json:"approval,omitempty"`
}

type ApprovalReviewOutput struct {
	Approval  *ToolApproval  `json:"approval"`
	Execution *ToolExecution `json:"execution"`
}

type GovernanceResult struct {
	Decision string `json:"decision"`
	Allowed  bool   `json:"allowed"`
	Reason   string `json:"reason,omitempty"`
}

type ToolResult struct {
	Summary string         `json:"summary"`
	Data    map[string]any `json:"data"`
}

type CreateToolInput struct {
	Name                 string         `json:"name"`
	Description          string         `json:"description,omitempty"`
	SourceType           string         `json:"source_type,omitempty"`
	DefaultPolicy        string         `json:"default_policy,omitempty"`
	RiskLevel            string         `json:"risk_level,omitempty"`
	RequiredLevel        string         `json:"required_level,omitempty"`
	ToolCategory         string         `json:"tool_category,omitempty"`
	ApprovalTierRequired string         `json:"approval_tier_required,omitempty"`
	InputSchema          map[string]any `json:"input_schema,omitempty"`
	OutputSchema         map[string]any `json:"output_schema,omitempty"`
	Metadata             map[string]any `json:"metadata,omitempty"`
	IsActive             *bool          `json:"is_active,omitempty"`
}

type CreateInterfaceFileInput struct {
	Name     string         `json:"name"`
	FileType string         `json:"file_type"`
	Content  string         `json:"content"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type UpdateInterfaceFileInput struct {
	Name     *string        `json:"name,omitempty"`
	FileType *string        `json:"file_type,omitempty"`
	Content  *string        `json:"content,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type UpdateToolInput struct {
	Description          *string        `json:"description,omitempty"`
	SourceType           *string        `json:"source_type,omitempty"`
	DefaultPolicy        *string        `json:"default_policy,omitempty"`
	RiskLevel            *string        `json:"risk_level,omitempty"`
	RequiredLevel        *string        `json:"required_level,omitempty"`
	ToolCategory         *string        `json:"tool_category,omitempty"`
	ApprovalTierRequired *string        `json:"approval_tier_required,omitempty"`
	InputSchema          map[string]any `json:"input_schema,omitempty"`
	OutputSchema         map[string]any `json:"output_schema,omitempty"`
	Metadata             map[string]any `json:"metadata,omitempty"`
	IsActive             *bool          `json:"is_active,omitempty"`
}
