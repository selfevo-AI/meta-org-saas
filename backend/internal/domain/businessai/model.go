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
	ID              uuid.UUID      `json:"id"`
	OrganizationID  uuid.UUID      `json:"organization_id"`
	ProjectID       uuid.UUID      `json:"project_id"`
	RequirementID   *uuid.UUID     `json:"requirement_id,omitempty"`
	Stage           string         `json:"stage"`
	Status          string         `json:"status"`
	RequestedByID   *uuid.UUID     `json:"requested_by_id,omitempty"`
	RequestedByType string         `json:"requested_by_type"`
	ProviderType    string         `json:"provider_type"`
	RequestedModel  string         `json:"requested_model"`
	InvocationID    *uuid.UUID     `json:"invocation_id,omitempty"`
	ResolvedModel   string         `json:"resolved_model"`
	InputContext    map[string]any `json:"input_context"`
	Analysis        *Analysis      `json:"analysis,omitempty"`
	CostAmount      float64        `json:"cost_amount"`
	Currency        string         `json:"currency"`
	InputTokens     int            `json:"input_tokens"`
	OutputTokens    int            `json:"output_tokens"`
	ErrorMessage    string         `json:"error_message,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	CompletedAt     *time.Time     `json:"completed_at,omitempty"`
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
}
