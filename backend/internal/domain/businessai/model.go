package businessai

import (
	"time"

	"github.com/google/uuid"
)

const (
	StagePlan   = "plan"
	StageDo     = "do"
	StageChange = "change"
	StageAccept = "accept"
	StageLearn  = "learn"

	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"

	ProposalNotSubmitted     = "not_submitted"
	ProposalSubmitting       = "submitting"
	ProposalApprovalRequired = "approval_required"
	ProposalCompleted        = "completed"
	ProposalRejected         = "rejected"
	ProposalFailed           = "failed"
	ProposalDenied           = "denied"
)

var ValidStages = map[string]struct{}{
	StagePlan: {}, StageDo: {}, StageChange: {}, StageAccept: {}, StageLearn: {},
}

type Finding struct {
	Title    string `json:"title"`
	Evidence string `json:"evidence"`
	Impact   string `json:"impact"`
}

type Recommendation struct {
	Title     string `json:"title"`
	Rationale string `json:"rationale"`
	Priority  string `json:"priority"`
}

type Risk struct {
	Title       string `json:"title"`
	Probability string `json:"probability"`
	Impact      string `json:"impact"`
	Mitigation  string `json:"mitigation"`
}

type Proposal struct {
	Action           string         `json:"action"`
	ToolName         string         `json:"tool_name"`
	Arguments        map[string]any `json:"arguments"`
	RequiresApproval bool           `json:"requires_approval"`
}

type Analysis struct {
	Summary         string           `json:"summary"`
	Findings        []Finding        `json:"findings"`
	Recommendations []Recommendation `json:"recommendations"`
	Risks           []Risk           `json:"risks"`
	Proposal        Proposal         `json:"proposal"`
	Confidence      float64          `json:"confidence"`
	EvidenceRefs    []string         `json:"evidence_refs"`
}

type Run struct {
	ID                  uuid.UUID      `json:"id"`
	OrganizationID      uuid.UUID      `json:"organization_id"`
	ProjectID           uuid.UUID      `json:"project_id"`
	RequirementID       *uuid.UUID     `json:"requirement_id,omitempty"`
	Stage               string         `json:"stage"`
	Status              string         `json:"status"`
	RequestedByID       *uuid.UUID     `json:"requested_by_id,omitempty"`
	RequestedByType     string         `json:"requested_by_type"`
	ProviderType        string         `json:"provider_type"`
	RequestedModel      string         `json:"requested_model"`
	InvocationID        *uuid.UUID     `json:"invocation_id,omitempty"`
	ResolvedModel       string         `json:"resolved_model"`
	InputContext        map[string]any `json:"input_context"`
	ContextHash         string         `json:"authoritative_context_hash"`
	Analysis            *Analysis      `json:"analysis,omitempty"`
	CostAmount          float64        `json:"cost_amount"`
	Currency            string         `json:"currency"`
	InputTokens         int            `json:"input_tokens"`
	OutputTokens        int            `json:"output_tokens"`
	ErrorMessage        string         `json:"error_message,omitempty"`
	ProposalStatus      string         `json:"proposal_status"`
	ToolExecutionID     *uuid.UUID     `json:"tool_execution_id,omitempty"`
	ToolApprovalID      *uuid.UUID     `json:"tool_approval_id,omitempty"`
	ProposalResult      map[string]any `json:"proposal_result"`
	ProposalError       string         `json:"proposal_error,omitempty"`
	ProposalSubmittedAt *time.Time     `json:"proposal_submitted_at,omitempty"`
	ProposalCompletedAt *time.Time     `json:"proposal_completed_at,omitempty"`
	CreatedAt           time.Time      `json:"created_at"`
	CompletedAt         *time.Time     `json:"completed_at,omitempty"`
}

type AnalyzeInput struct {
	OrganizationID  uuid.UUID
	ProjectID       uuid.UUID
	RequirementID   *uuid.UUID
	Stage           string
	RequestedByID   *uuid.UUID
	RequestedByType string
	ProviderType    string
	Model           string
	Focus           string
	Context         map[string]any
	ContextHash     string
}

type SubmitProposalInput struct {
	OrganizationID uuid.UUID
	ProjectID      uuid.UUID
	RunID          uuid.UUID
	ActorID        uuid.UUID
	ActorType      string
	ContextHash    string
}

type ProposalExecutionRequest struct {
	ToolName        string
	InvocationID    *uuid.UUID
	ActorID         uuid.UUID
	ActorType       string
	OrganizationID  uuid.UUID
	ProjectID       uuid.UUID
	IdempotencyKey  string
	RequireApproval bool
	Arguments       map[string]any
}

type ProposalExecutionOutput struct {
	ExecutionID uuid.UUID
	ApprovalID  *uuid.UUID
	Status      string
	Result      map[string]any
	Error       string
}

type ProposalExecutionUpdate struct {
	ExecutionID uuid.UUID
	Status      string
	Result      map[string]any
	Error       string
}
