package project

import (
	"time"

	"github.com/google/uuid"
)

type Requirement struct {
	ID             uuid.UUID      `json:"id"`
	MasterKey      string         `json:"master_key,omitempty"`
	Title          string         `json:"title"`
	Description    string         `json:"description"`
	Source         string         `json:"source"`
	Status         string         `json:"status"`
	Priority       string         `json:"priority"`
	RiskLevel      string         `json:"risk_level"`
	RequiredLevel  string         `json:"required_level"`
	BudgetAmount   float64        `json:"budget_amount"`
	BudgetCurrency string         `json:"budget_currency"`
	OrganizationID *uuid.UUID     `json:"organization_id,omitempty"`
	DepartmentID   *uuid.UUID     `json:"department_id,omitempty"`
	CreatedByID    *uuid.UUID     `json:"created_by_id,omitempty"`
	CreatedByType  string         `json:"created_by_type,omitempty"`
	Analysis       map[string]any `json:"analysis"`
	Metadata       map[string]any `json:"metadata"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type RequirementDocument struct {
	ID             uuid.UUID      `json:"id"`
	MasterKey      string         `json:"master_key,omitempty"`
	RequirementID  uuid.UUID      `json:"requirement_id"`
	FileName       string         `json:"file_name"`
	ContentType    string         `json:"content_type"`
	SizeBytes      int64          `json:"size_bytes"`
	UploadedByID   *uuid.UUID     `json:"uploaded_by_id,omitempty"`
	UploadedByType string         `json:"uploaded_by_type,omitempty"`
	Metadata       map[string]any `json:"metadata"`
	CreatedAt      time.Time      `json:"created_at"`
}

type RequirementDocumentContent struct {
	RequirementDocument
	Content []byte `json:"-"`
}

type RequirementAnalysisWorkflow struct {
	ID                 uuid.UUID      `json:"id"`
	MasterKey          string         `json:"master_key,omitempty"`
	RequirementID      uuid.UUID      `json:"requirement_id"`
	WorkflowID         uuid.UUID      `json:"workflow_id"`
	WorkflowTemplateID uuid.UUID      `json:"workflow_template_id"`
	Status             string         `json:"status"`
	AnalysisResult     map[string]any `json:"analysis_result"`
	Metadata           map[string]any `json:"metadata"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

type Project struct {
	ID             uuid.UUID      `json:"id"`
	MasterKey      string         `json:"master_key,omitempty"`
	RequirementID  *uuid.UUID     `json:"requirement_id,omitempty"`
	OrganizationID *uuid.UUID     `json:"organization_id,omitempty"`
	DepartmentID   *uuid.UUID     `json:"department_id,omitempty"`
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	Status         string         `json:"status"`
	Priority       string         `json:"priority"`
	RiskLevel      string         `json:"risk_level"`
	RequiredLevel  string         `json:"required_level"`
	BudgetAmount   float64        `json:"budget_amount"`
	BudgetCurrency string         `json:"budget_currency"`
	Metadata       map[string]any `json:"metadata"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type ProjectMember struct {
	ID                   uuid.UUID      `json:"id"`
	MasterKey            string         `json:"master_key,omitempty"`
	ProjectID            uuid.UUID      `json:"project_id"`
	ActorID              uuid.UUID      `json:"actor_id"`
	ActorType            string         `json:"actor_type"`
	PositionID           *uuid.UUID     `json:"position_id,omitempty"`
	PositionAssignmentID *uuid.UUID     `json:"position_assignment_id,omitempty"`
	Role                 string         `json:"role"`
	Title                string         `json:"title"`
	AllocationPercent    float64        `json:"allocation_percent"`
	CostRate             float64        `json:"cost_rate"`
	PermissionLevel      string         `json:"permission_level"`
	Capabilities         []string       `json:"capabilities"`
	Status               string         `json:"status"`
	Metadata             map[string]any `json:"metadata"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
}

type ProjectWorkflow struct {
	ID                 uuid.UUID      `json:"id"`
	MasterKey          string         `json:"master_key,omitempty"`
	ProjectID          uuid.UUID      `json:"project_id"`
	WorkflowID         uuid.UUID      `json:"workflow_id"`
	WorkflowTemplateID *uuid.UUID     `json:"workflow_template_id,omitempty"`
	Purpose            string         `json:"purpose"`
	Status             string         `json:"status"`
	Metadata           map[string]any `json:"metadata"`
	CreatedAt          time.Time      `json:"created_at"`
}

type Deliverable struct {
	ID              uuid.UUID      `json:"id"`
	MasterKey       string         `json:"master_key,omitempty"`
	ProjectID       uuid.UUID      `json:"project_id"`
	Name            string         `json:"name"`
	DeliverableType string         `json:"deliverable_type"`
	URI             string         `json:"uri"`
	Version         string         `json:"version"`
	Status          string         `json:"status"`
	SubmittedByID   *uuid.UUID     `json:"submitted_by_id,omitempty"`
	SubmittedByType string         `json:"submitted_by_type,omitempty"`
	AcceptedByID    *uuid.UUID     `json:"accepted_by_id,omitempty"`
	AcceptedByType  string         `json:"accepted_by_type,omitempty"`
	Evidence        map[string]any `json:"evidence"`
	Metadata        map[string]any `json:"metadata"`
	SubmittedAt     *time.Time     `json:"submitted_at,omitempty"`
	AcceptedAt      *time.Time     `json:"accepted_at,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

type CostEntry struct {
	ID          uuid.UUID      `json:"id"`
	MasterKey   string         `json:"master_key,omitempty"`
	ProjectID   uuid.UUID      `json:"project_id"`
	SourceType  string         `json:"source_type"`
	SourceID    *uuid.UUID     `json:"source_id,omitempty"`
	ActorID     *uuid.UUID     `json:"actor_id,omitempty"`
	ActorType   string         `json:"actor_type,omitempty"`
	Amount      float64        `json:"amount"`
	Currency    string         `json:"currency"`
	OccurredAt  time.Time      `json:"occurred_at"`
	Description string         `json:"description"`
	Metadata    map[string]any `json:"metadata"`
	CreatedAt   time.Time      `json:"created_at"`
}

type ProjectEvaluation struct {
	ID                 uuid.UUID      `json:"id"`
	MasterKey          string         `json:"master_key,omitempty"`
	ProjectID          uuid.UUID      `json:"project_id"`
	ActorID            *uuid.UUID     `json:"actor_id,omitempty"`
	ActorType          string         `json:"actor_type,omitempty"`
	CapabilityID       *uuid.UUID     `json:"capability_id,omitempty"`
	EvaluatorID        *uuid.UUID     `json:"evaluator_id,omitempty"`
	EvaluatorType      string         `json:"evaluator_type"`
	QualityScore       float64        `json:"quality_score"`
	DeliveryScore      float64        `json:"delivery_score"`
	CostScore          float64        `json:"cost_score"`
	CollaborationScore float64        `json:"collaboration_score"`
	OverallScore       float64        `json:"overall_score"`
	Conclusion         string         `json:"conclusion"`
	Evidence           map[string]any `json:"evidence"`
	CreatedAt          time.Time      `json:"created_at"`
}

type CostSummary struct {
	ProjectID       uuid.UUID          `json:"project_id"`
	Currency        string             `json:"currency"`
	BaseCurrency    string             `json:"base_currency,omitempty"`
	EntryCount      int                `json:"entry_count"`
	TotalAmount     float64            `json:"total_amount"`
	BaseTotalAmount float64            `json:"base_total_amount,omitempty"`
	BudgetAmount    float64            `json:"budget_amount"`
	BudgetVariance  float64            `json:"budget_variance"`
	BySource        []CostSummaryItem  `json:"by_source"`
	Metadata        map[string]float64 `json:"metadata,omitempty"`
}

type CostSummaryItem struct {
	SourceType string  `json:"source_type"`
	Amount     float64 `json:"amount"`
	Count      int     `json:"count"`
}

type ProjectOverview struct {
	Project      *Project            `json:"project"`
	Requirement  *Requirement        `json:"requirement,omitempty"`
	Members      []ProjectMember     `json:"members"`
	Workflows    []ProjectWorkflow   `json:"workflows"`
	Deliverables []Deliverable       `json:"deliverables"`
	CostSummary  *CostSummary        `json:"cost_summary"`
	Evaluations  []ProjectEvaluation `json:"evaluations"`
	Lifecycle    ProjectLifecycle    `json:"lifecycle"`
}

type ProjectLifecycle struct {
	Stage           string   `json:"stage"`
	PDCAStage       string   `json:"pdca_stage"`
	RequirementID   string   `json:"requirement_id,omitempty"`
	DemandProfileID string   `json:"demand_profile_id,omitempty"`
	PDCACycleID     string   `json:"pdca_cycle_id,omitempty"`
	AllowedActions  []string `json:"allowed_actions"`
	Blockers        []string `json:"blockers"`
	NextAction      string   `json:"next_action"`
}

type ActorInput struct {
	ActorID   *uuid.UUID `json:"actor_id,omitempty"`
	ActorType string     `json:"actor_type,omitempty"`
}

type CreateRequirementInput struct {
	Title          string         `json:"title"`
	Description    string         `json:"description"`
	Source         string         `json:"source,omitempty"`
	Priority       string         `json:"priority,omitempty"`
	RiskLevel      string         `json:"risk_level,omitempty"`
	RequiredLevel  string         `json:"required_level,omitempty"`
	OrganizationID *uuid.UUID     `json:"organization_id,omitempty"`
	DepartmentID   *uuid.UUID     `json:"department_id,omitempty"`
	CreatedByID    *uuid.UUID     `json:"created_by_id,omitempty"`
	CreatedByType  string         `json:"created_by_type,omitempty"`
	BudgetAmount   float64        `json:"budget_amount,omitempty"`
	BudgetCurrency string         `json:"budget_currency,omitempty"`
	Analysis       map[string]any `json:"analysis,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type UploadRequirementDocumentInput struct {
	ActorInput
	FileName    string         `json:"file_name"`
	ContentType string         `json:"content_type"`
	SizeBytes   int64          `json:"size_bytes"`
	Content     []byte         `json:"-"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type StartRequirementAnalysisWorkflowInput struct {
	ActorInput
	WorkflowTemplateID uuid.UUID      `json:"workflow_template_id"`
	Purpose            string         `json:"purpose,omitempty"`
	Context            map[string]any `json:"context,omitempty"`
	Metadata           map[string]any `json:"metadata,omitempty"`
}

type SyncRequirementAnalysisWorkflowInput struct {
	ActorInput
	WorkflowID uuid.UUID `json:"workflow_id,omitempty"`
}

type UpdateRequirementInput struct {
	Title          string         `json:"title,omitempty"`
	Description    string         `json:"description,omitempty"`
	Source         string         `json:"source,omitempty"`
	Status         string         `json:"status,omitempty"`
	Priority       string         `json:"priority,omitempty"`
	RiskLevel      string         `json:"risk_level,omitempty"`
	RequiredLevel  string         `json:"required_level,omitempty"`
	BudgetAmount   *float64       `json:"budget_amount,omitempty"`
	BudgetCurrency *string        `json:"budget_currency,omitempty"`
	Analysis       map[string]any `json:"analysis,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type AnalyzeRequirementInput struct {
	ActorInput
	Notes string `json:"notes,omitempty"`
}

type ConvertRequirementInput struct {
	ActorInput
	Name           string         `json:"name,omitempty"`
	Description    string         `json:"description,omitempty"`
	BudgetAmount   float64        `json:"budget_amount,omitempty"`
	BudgetCurrency string         `json:"budget_currency,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type CreateProjectInput struct {
	ActorInput
	RequirementID  *uuid.UUID     `json:"requirement_id,omitempty"`
	OrganizationID *uuid.UUID     `json:"organization_id,omitempty"`
	DepartmentID   *uuid.UUID     `json:"department_id,omitempty"`
	Name           string         `json:"name"`
	Description    string         `json:"description,omitempty"`
	Status         string         `json:"status,omitempty"`
	Priority       string         `json:"priority,omitempty"`
	RiskLevel      string         `json:"risk_level,omitempty"`
	RequiredLevel  string         `json:"required_level,omitempty"`
	BudgetAmount   float64        `json:"budget_amount,omitempty"`
	BudgetCurrency string         `json:"budget_currency,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type UpdateProjectInput struct {
	Name           string         `json:"name,omitempty"`
	Description    string         `json:"description,omitempty"`
	Status         string         `json:"status,omitempty"`
	Priority       string         `json:"priority,omitempty"`
	RiskLevel      string         `json:"risk_level,omitempty"`
	RequiredLevel  string         `json:"required_level,omitempty"`
	BudgetAmount   *float64       `json:"budget_amount,omitempty"`
	BudgetCurrency *string        `json:"budget_currency,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type AddProjectMemberInput struct {
	ActorInput
	ProjectID            uuid.UUID      `json:"project_id,omitempty"`
	MemberActorID        uuid.UUID      `json:"member_actor_id"`
	MemberActorType      string         `json:"member_actor_type"`
	PositionID           *uuid.UUID     `json:"position_id,omitempty"`
	PositionAssignmentID *uuid.UUID     `json:"position_assignment_id,omitempty"`
	Role                 string         `json:"role,omitempty"`
	Title                string         `json:"title,omitempty"`
	AllocationPercent    float64        `json:"allocation_percent,omitempty"`
	CostRate             float64        `json:"cost_rate,omitempty"`
	PermissionLevel      string         `json:"permission_level,omitempty"`
	Capabilities         []string       `json:"capabilities,omitempty"`
	Status               string         `json:"status,omitempty"`
	Metadata             map[string]any `json:"metadata,omitempty"`
}

type BindProjectWorkflowInput struct {
	ActorInput
	WorkflowID         *uuid.UUID     `json:"workflow_id,omitempty"`
	WorkflowTemplateID *uuid.UUID     `json:"workflow_template_id,omitempty"`
	Purpose            string         `json:"purpose,omitempty"`
	Status             string         `json:"status,omitempty"`
	Metadata           map[string]any `json:"metadata,omitempty"`
}

type MatchProjectActorsInput struct {
	TaskDescription      string     `json:"task_description"`
	WorkflowTemplateID   *uuid.UUID `json:"workflow_template_id,omitempty"`
	RequiredCapabilities []string   `json:"required_capabilities,omitempty"`
	RequiredLevel        string     `json:"required_level,omitempty"`
	RiskLevel            string     `json:"risk_level,omitempty"`
	MemberTypes          []string   `json:"member_types,omitempty"`
}

type UpdateProjectStatusInput struct {
	ActorInput
	Status string `json:"status"`
	Note   string `json:"note,omitempty"`
}

type CreateDeliverableInput struct {
	ActorInput
	Name            string         `json:"name"`
	DeliverableType string         `json:"deliverable_type,omitempty"`
	URI             string         `json:"uri,omitempty"`
	Version         string         `json:"version,omitempty"`
	Status          string         `json:"status,omitempty"`
	Evidence        map[string]any `json:"evidence,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

type UpdateDeliverableInput struct {
	Name            string         `json:"name,omitempty"`
	DeliverableType string         `json:"deliverable_type,omitempty"`
	URI             string         `json:"uri,omitempty"`
	Version         string         `json:"version,omitempty"`
	Status          string         `json:"status,omitempty"`
	Evidence        map[string]any `json:"evidence,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

type DeliverableActionInput struct {
	ActorInput
	Evidence map[string]any `json:"evidence,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
	Reason   string         `json:"reason,omitempty"`
}

type CreateCostEntryInput struct {
	ActorInput
	SourceType     string         `json:"source_type,omitempty"`
	SourceID       *uuid.UUID     `json:"source_id,omitempty"`
	EntryActorID   *uuid.UUID     `json:"entry_actor_id,omitempty"`
	EntryActorType string         `json:"entry_actor_type,omitempty"`
	Amount         float64        `json:"amount"`
	Currency       string         `json:"currency,omitempty"`
	OccurredAt     *time.Time     `json:"occurred_at,omitempty"`
	Description    string         `json:"description,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type CreateProjectEvaluationInput struct {
	ActorInput
	EvaluatedActorID   *uuid.UUID     `json:"evaluated_actor_id,omitempty"`
	EvaluatedActorType string         `json:"evaluated_actor_type,omitempty"`
	CapabilityID       *uuid.UUID     `json:"capability_id,omitempty"`
	QualityScore       float64        `json:"quality_score"`
	DeliveryScore      float64        `json:"delivery_score"`
	CostScore          float64        `json:"cost_score"`
	CollaborationScore float64        `json:"collaboration_score"`
	Conclusion         string         `json:"conclusion,omitempty"`
	Evidence           map[string]any `json:"evidence,omitempty"`
}

type CloseFeedbackInput struct {
	ActorInput
	OutcomeScore float64        `json:"outcome_score,omitempty"`
	Conclusion   string         `json:"conclusion,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type AnalyzeProjectStageInput struct {
	ActorInput
	Stage        string         `json:"stage"`
	Focus        string         `json:"focus,omitempty"`
	ProviderType string         `json:"provider_type,omitempty"`
	Model        string         `json:"model,omitempty"`
	Context      map[string]any `json:"context,omitempty"`
}
