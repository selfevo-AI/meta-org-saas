package organization

import (
	"time"

	"github.com/google/uuid"
)

type MVRUStatus string
type AuthorityTier string

const (
	MVRUDesigning  MVRUStatus = "designing"
	MVRUActive     MVRUStatus = "active"
	MVRUEvaluating MVRUStatus = "evaluating"
	MVRUEvolving   MVRUStatus = "evolving"
	MVRUDissolved  MVRUStatus = "dissolved"

	AuthorityOrganizationCreator AuthorityTier = "organization_creator"
	AuthorityOrganizationAdmin   AuthorityTier = "organization_admin"
	AuthorityReviewer            AuthorityTier = "reviewer"
	AuthorityExecutor            AuthorityTier = "executor"
)

type Organization struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	CreatedBy   *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type MVRU struct {
	ID             uuid.UUID          `json:"id"`
	OrganizationID uuid.UUID          `json:"organization_id"`
	Name           string             `json:"name"`
	Description    string             `json:"description,omitempty"`
	Status         MVRUStatus         `json:"status"`
	Boundary       map[string]any     `json:"boundary"`
	Config         map[string]any     `json:"config"`
	ParentID       *uuid.UUID         `json:"parent_id,omitempty"`
	Children       []MVRU             `json:"children,omitempty"`
	Members        []MVRUMember       `json:"members,omitempty"`
	Relationships  []MVRURelationship `json:"relationships,omitempty"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
}

type Team struct {
	ID          uuid.UUID `json:"id"`
	MVRUID      uuid.UUID `json:"mvru_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type MVRUMember struct {
	MVRUID  uuid.UUID  `json:"mvru_id"`
	UserID  *uuid.UUID `json:"user_id,omitempty"`
	AgentID *uuid.UUID `json:"agent_id,omitempty"`
	RoleID  uuid.UUID  `json:"role_id"`
}

type MVRURelationship struct {
	ID           uuid.UUID      `json:"id"`
	SourceMVRUID uuid.UUID      `json:"source_mvru_id"`
	TargetMVRUID uuid.UUID      `json:"target_mvru_id"`
	RelType      string         `json:"rel_type"`
	Config       map[string]any `json:"config,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}

type Department struct {
	ID             uuid.UUID                `json:"id"`
	OrganizationID uuid.UUID                `json:"organization_id"`
	ParentID       *uuid.UUID               `json:"parent_id,omitempty"`
	Name           string                   `json:"name"`
	Code           string                   `json:"code,omitempty"`
	Description    string                   `json:"description,omitempty"`
	Status         string                   `json:"status"`
	SortOrder      int                      `json:"sort_order"`
	Metadata       map[string]any           `json:"metadata"`
	Children       []Department             `json:"children,omitempty"`
	Positions      []Position               `json:"positions,omitempty"`
	Members        []OrganizationMembership `json:"members,omitempty"`
	MVRULinks      []DepartmentMVRULink     `json:"mvru_links,omitempty"`
	CreatedAt      time.Time                `json:"created_at"`
	UpdatedAt      time.Time                `json:"updated_at"`
}

type Position struct {
	ID                   uuid.UUID            `json:"id"`
	OrganizationID       uuid.UUID            `json:"organization_id"`
	DepartmentID         uuid.UUID            `json:"department_id"`
	Name                 string               `json:"name"`
	Code                 string               `json:"code,omitempty"`
	Description          string               `json:"description,omitempty"`
	Status               string               `json:"status"`
	SortOrder            int                  `json:"sort_order"`
	PermissionLevel      string               `json:"permission_level"`
	RequiredCapabilities []string             `json:"required_capabilities"`
	Metadata             map[string]any       `json:"metadata"`
	Assignments          []PositionAssignment `json:"assignments,omitempty"`
	CreatedAt            time.Time            `json:"created_at"`
	UpdatedAt            time.Time            `json:"updated_at"`
}

type PositionAssignment struct {
	ID                uuid.UUID      `json:"id"`
	PositionID        uuid.UUID      `json:"position_id"`
	OrganizationID    uuid.UUID      `json:"organization_id"`
	DepartmentID      uuid.UUID      `json:"department_id"`
	MetaResourceID    *uuid.UUID     `json:"meta_resource_id,omitempty"`
	MetaResourceName  string         `json:"meta_resource_name,omitempty"`
	MetaResourceType  string         `json:"meta_resource_type,omitempty"`
	ActorID           uuid.UUID      `json:"actor_id"`
	ActorType         string         `json:"actor_type"`
	ActorName         string         `json:"actor_name,omitempty"`
	ActorEmail        string         `json:"actor_email,omitempty"`
	AssignmentType    string         `json:"assignment_type"`
	AllocationPercent float64        `json:"allocation_percent"`
	Status            string         `json:"status"`
	Metadata          map[string]any `json:"metadata"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

type ExternalMember struct {
	ID           uuid.UUID      `json:"id"`
	Name         string         `json:"name"`
	Email        string         `json:"email,omitempty"`
	Vendor       string         `json:"vendor,omitempty"`
	ContractType string         `json:"contract_type,omitempty"`
	Status       string         `json:"status"`
	Metadata     map[string]any `json:"metadata"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type OrganizationMembership struct {
	ID               uuid.UUID      `json:"id"`
	OrganizationID   uuid.UUID      `json:"organization_id"`
	DepartmentID     uuid.UUID      `json:"department_id"`
	MemberType       string         `json:"member_type"`
	UserID           *uuid.UUID     `json:"user_id,omitempty"`
	ExternalMemberID *uuid.UUID     `json:"external_member_id,omitempty"`
	AgentID          *uuid.UUID     `json:"agent_id,omitempty"`
	MemberName       string         `json:"member_name,omitempty"`
	MemberEmail      string         `json:"member_email,omitempty"`
	Title            string         `json:"title,omitempty"`
	RoleID           *uuid.UUID     `json:"role_id,omitempty"`
	RoleName         string         `json:"role_name,omitempty"`
	AuthorityTier    AuthorityTier  `json:"authority_tier"`
	Status           string         `json:"status"`
	JoinedAt         time.Time      `json:"joined_at"`
	Metadata         map[string]any `json:"metadata"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

type DepartmentMVRULink struct {
	DepartmentID uuid.UUID      `json:"department_id"`
	MVRUID       uuid.UUID      `json:"mvru_id"`
	MVRUName     string         `json:"mvru_name,omitempty"`
	LinkType     string         `json:"link_type"`
	Metadata     map[string]any `json:"metadata"`
	CreatedAt    time.Time      `json:"created_at"`
}

type MemberMatchCandidate struct {
	MembershipID         uuid.UUID  `json:"membership_id"`
	DepartmentID         uuid.UUID  `json:"department_id"`
	PositionID           *uuid.UUID `json:"position_id,omitempty"`
	PositionName         string     `json:"position_name,omitempty"`
	PositionAssignmentID *uuid.UUID `json:"position_assignment_id,omitempty"`
	MemberType           string     `json:"member_type"`
	MemberID             uuid.UUID  `json:"member_id"`
	MemberName           string     `json:"member_name"`
	Title                string     `json:"title,omitempty"`
	Score                float64    `json:"score"`
	WeightSnapshot       float64    `json:"weight_snapshot"`
	AccessDecision       string     `json:"access_decision"`
	AccessAllowed        bool       `json:"access_allowed"`
	RequiresApproval     bool       `json:"requires_approval"`
	Reason               string     `json:"reason"`
	CapabilityMatchPath  string     `json:"capability_match_path"`
	WorkflowAssignHint   string     `json:"workflow_assign_hint"`
}

type CapabilityMatchBridge struct {
	DepartmentID         *uuid.UUID     `json:"department_id,omitempty"`
	TaskDescription      string         `json:"task_description"`
	RequiredCapabilities []string       `json:"required_capabilities,omitempty"`
	RequiredLevel        string         `json:"required_level,omitempty"`
	RiskLevel            string         `json:"risk_level,omitempty"`
	CapabilityMatchPath  string         `json:"capability_match_path"`
	ContextWeightPath    string         `json:"context_weight_path"`
	AccessDecisionPath   string         `json:"access_decision_path"`
	WorkflowStartPath    string         `json:"workflow_start_path"`
	Context              map[string]any `json:"context"`
}

type CreateOrganizationInput struct {
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	CreatedBy   *uuid.UUID `json:"created_by,omitempty"`
}

type UpdateOrganizationInput struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

type CreateMVRUInput struct {
	OrganizationID uuid.UUID      `json:"organization_id"`
	Name           string         `json:"name"`
	Description    string         `json:"description,omitempty"`
	Boundary       map[string]any `json:"boundary,omitempty"`
	Config         map[string]any `json:"config,omitempty"`
	ParentID       *uuid.UUID     `json:"parent_id,omitempty"`
}

type CreateDepartmentInput struct {
	OrganizationID uuid.UUID      `json:"organization_id"`
	ParentID       *uuid.UUID     `json:"parent_id,omitempty"`
	Name           string         `json:"name"`
	Code           string         `json:"code,omitempty"`
	Description    string         `json:"description,omitempty"`
	Status         string         `json:"status,omitempty"`
	SortOrder      int            `json:"sort_order,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type UpdateDepartmentInput struct {
	ParentID    *uuid.UUID     `json:"parent_id,omitempty"`
	Name        string         `json:"name,omitempty"`
	Code        string         `json:"code,omitempty"`
	Description string         `json:"description,omitempty"`
	Status      string         `json:"status,omitempty"`
	SortOrder   *int           `json:"sort_order,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type CreatePositionInput struct {
	OrganizationID       uuid.UUID      `json:"organization_id"`
	DepartmentID         uuid.UUID      `json:"department_id"`
	Name                 string         `json:"name"`
	Code                 string         `json:"code,omitempty"`
	Description          string         `json:"description,omitempty"`
	Status               string         `json:"status,omitempty"`
	SortOrder            int            `json:"sort_order,omitempty"`
	PermissionLevel      string         `json:"permission_level,omitempty"`
	RequiredCapabilities []string       `json:"required_capabilities,omitempty"`
	Metadata             map[string]any `json:"metadata,omitempty"`
}

type UpdatePositionInput struct {
	DepartmentID         *uuid.UUID     `json:"department_id,omitempty"`
	Name                 string         `json:"name,omitempty"`
	Code                 string         `json:"code,omitempty"`
	Description          string         `json:"description,omitempty"`
	Status               string         `json:"status,omitempty"`
	SortOrder            *int           `json:"sort_order,omitempty"`
	PermissionLevel      string         `json:"permission_level,omitempty"`
	RequiredCapabilities []string       `json:"required_capabilities,omitempty"`
	Metadata             map[string]any `json:"metadata,omitempty"`
}

type CreatePositionAssignmentInput struct {
	PositionID        uuid.UUID      `json:"position_id"`
	MetaResourceID    *uuid.UUID     `json:"meta_resource_id,omitempty"`
	ActorID           uuid.UUID      `json:"actor_id"`
	ActorType         string         `json:"actor_type"`
	AssignmentType    string         `json:"assignment_type,omitempty"`
	AllocationPercent float64        `json:"allocation_percent,omitempty"`
	Status            string         `json:"status,omitempty"`
	Metadata          map[string]any `json:"metadata,omitempty"`
}

type UpdatePositionAssignmentInput struct {
	MetaResourceID    *uuid.UUID     `json:"meta_resource_id,omitempty"`
	AssignmentType    string         `json:"assignment_type,omitempty"`
	AllocationPercent *float64       `json:"allocation_percent,omitempty"`
	Status            string         `json:"status,omitempty"`
	Metadata          map[string]any `json:"metadata,omitempty"`
}

type CreateExternalMemberInput struct {
	Name         string         `json:"name"`
	Email        string         `json:"email,omitempty"`
	Vendor       string         `json:"vendor,omitempty"`
	ContractType string         `json:"contract_type,omitempty"`
	Status       string         `json:"status,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type UpdateExternalMemberInput struct {
	Name         string         `json:"name,omitempty"`
	Email        string         `json:"email,omitempty"`
	Vendor       string         `json:"vendor,omitempty"`
	ContractType string         `json:"contract_type,omitempty"`
	Status       string         `json:"status,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type AddOrganizationMemberInput struct {
	DepartmentID     uuid.UUID      `json:"department_id"`
	MemberType       string         `json:"member_type"`
	UserID           *uuid.UUID     `json:"user_id,omitempty"`
	ExternalMemberID *uuid.UUID     `json:"external_member_id,omitempty"`
	AgentID          *uuid.UUID     `json:"agent_id,omitempty"`
	Title            string         `json:"title,omitempty"`
	RoleID           *uuid.UUID     `json:"role_id,omitempty"`
	AuthorityTier    AuthorityTier  `json:"authority_tier,omitempty"`
	Status           string         `json:"status,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

type UpdateOrganizationMembershipInput struct {
	Title         string         `json:"title,omitempty"`
	RoleID        *uuid.UUID     `json:"role_id,omitempty"`
	AuthorityTier AuthorityTier  `json:"authority_tier,omitempty"`
	Status        string         `json:"status,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

const (
	PermissionChangePending   = "pending"
	PermissionChangeApproved  = "approved"
	PermissionChangeRejected  = "rejected"
	PermissionChangeApplied   = "applied"
	PermissionChangeCancelled = "cancelled"
)

type PermissionChangeRequest struct {
	ID              uuid.UUID                         `json:"id"`
	OrganizationID  uuid.UUID                         `json:"organization_id"`
	MembershipID    uuid.UUID                         `json:"membership_id"`
	RequestedBy     *uuid.UUID                        `json:"requested_by,omitempty"`
	RequestedByType string                            `json:"requested_by_type"`
	RequestedChange UpdateOrganizationMembershipInput `json:"requested_change"`
	Reason          string                            `json:"reason"`
	Status          string                            `json:"status"`
	ReviewedBy      *uuid.UUID                        `json:"reviewed_by,omitempty"`
	ReviewReason    string                            `json:"review_reason,omitempty"`
	ReviewedAt      *time.Time                        `json:"reviewed_at,omitempty"`
	AppliedAt       *time.Time                        `json:"applied_at,omitempty"`
	CreatedAt       time.Time                         `json:"created_at"`
	UpdatedAt       time.Time                         `json:"updated_at"`
}

type CreatePermissionChangeRequestInput struct {
	MembershipID    uuid.UUID                         `json:"membership_id"`
	RequestedChange UpdateOrganizationMembershipInput `json:"requested_change"`
	Reason          string                            `json:"reason,omitempty"`
}

type CreatePermissionChangeRequestRecord struct {
	OrganizationID  uuid.UUID
	MembershipID    uuid.UUID
	RequestedBy     uuid.UUID
	RequestedByType string
	RequestedChange UpdateOrganizationMembershipInput
	Reason          string
}

type ReviewPermissionChangeRequestInput struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason,omitempty"`
}

type UpdatePermissionChangeRequestStatusInput struct {
	Status       string
	ReviewedBy   *uuid.UUID
	ReviewReason string
}

type OrganizationAccessRule struct {
	ID             uuid.UUID      `json:"id"`
	OrganizationID uuid.UUID      `json:"organization_id"`
	ScopeType      string         `json:"scope_type"`
	ScopeID        string         `json:"scope_id"`
	ResourceType   string         `json:"resource_type"`
	ResourceKey    string         `json:"resource_key"`
	Action         string         `json:"action"`
	ActorType      string         `json:"actor_type"`
	ActorID        string         `json:"actor_id,omitempty"`
	RoleID         *uuid.UUID     `json:"role_id,omitempty"`
	AuthorityTier  AuthorityTier  `json:"authority_tier,omitempty"`
	Behavior       string         `json:"behavior"`
	RequiredLevel  string         `json:"required_level"`
	Priority       int            `json:"priority"`
	Status         string         `json:"status"`
	Reason         string         `json:"reason,omitempty"`
	Metadata       map[string]any `json:"metadata"`
	CreatedBy      *uuid.UUID     `json:"created_by,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type CreateAccessRuleInput struct {
	ScopeType     string         `json:"scope_type"`
	ScopeID       string         `json:"scope_id,omitempty"`
	ResourceType  string         `json:"resource_type"`
	ResourceKey   string         `json:"resource_key,omitempty"`
	Action        string         `json:"action"`
	ActorType     string         `json:"actor_type,omitempty"`
	ActorID       string         `json:"actor_id,omitempty"`
	RoleID        *uuid.UUID     `json:"role_id,omitempty"`
	AuthorityTier AuthorityTier  `json:"authority_tier,omitempty"`
	Behavior      string         `json:"behavior"`
	RequiredLevel string         `json:"required_level,omitempty"`
	Priority      int            `json:"priority,omitempty"`
	Reason        string         `json:"reason,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

type CreateAccessRuleRecord struct {
	OrganizationID uuid.UUID
	ScopeType      string
	ScopeID        string
	ResourceType   string
	ResourceKey    string
	Action         string
	ActorType      string
	ActorID        string
	RoleID         *uuid.UUID
	AuthorityTier  AuthorityTier
	Behavior       string
	RequiredLevel  string
	Priority       int
	Reason         string
	Metadata       map[string]any
	CreatedBy      *uuid.UUID
}

type LinkDepartmentMVRUInput struct {
	DepartmentID uuid.UUID      `json:"department_id"`
	MVRUID       uuid.UUID      `json:"mvru_id"`
	LinkType     string         `json:"link_type,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type MatchMembersInput struct {
	OrganizationID       uuid.UUID  `json:"organization_id"`
	DepartmentID         *uuid.UUID `json:"department_id,omitempty"`
	PositionID           *uuid.UUID `json:"position_id,omitempty"`
	TaskDescription      string     `json:"task_description"`
	WorkflowTemplateID   *uuid.UUID `json:"workflow_template_id,omitempty"`
	RequiredCapabilities []string   `json:"required_capabilities,omitempty"`
	RequiredLevel        string     `json:"required_level,omitempty"`
	RiskLevel            string     `json:"risk_level,omitempty"`
	MemberTypes          []string   `json:"member_types,omitempty"`
}

type MatchCapabilitiesInput struct {
	DepartmentID         *uuid.UUID     `json:"department_id,omitempty"`
	TaskDescription      string         `json:"task_description"`
	RequiredCapabilities []string       `json:"required_capabilities,omitempty"`
	RequiredLevel        string         `json:"required_level,omitempty"`
	RiskLevel            string         `json:"risk_level,omitempty"`
	Context              map[string]any `json:"context,omitempty"`
}
