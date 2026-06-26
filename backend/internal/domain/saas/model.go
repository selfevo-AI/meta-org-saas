package saas

import (
	"time"

	"github.com/google/uuid"
)

const (
	ModeSingleOrg = "single_org"
	ModeSaaS      = "saas"

	OnboardingRequired = "required"
	OnboardingComplete = "complete"

	AuthorityOwner = "organization_creator"
	AuthorityAdmin = "organization_admin"

	OrganizationStatusActive = "active"
	OrganizationStatusClosed = "closed"
)

type UserProfile struct {
	ID                    uuid.UUID             `json:"id"`
	Name                  string                `json:"name"`
	Email                 string                `json:"email"`
	AccountStatus         string                `json:"account_status"`
	OnboardingStatus      string                `json:"onboarding_status"`
	OnboardingRequired    bool                  `json:"onboarding_required"`
	DefaultOrganizationID *uuid.UUID            `json:"default_organization_id,omitempty"`
	PlatformRole          string                `json:"platform_role,omitempty"`
	Organizations         []OrganizationAccount `json:"organizations"`
	EnabledModules        map[string]bool       `json:"enabled_modules,omitempty"`
}

type OrganizationAccount struct {
	ID           uuid.UUID  `json:"id"`
	Name         string     `json:"name"`
	Description  string     `json:"description,omitempty"`
	Status       string     `json:"status,omitempty"`
	ClosedAt     *time.Time `json:"closed_at,omitempty"`
	ClosedBy     *uuid.UUID `json:"closed_by,omitempty"`
	ClosedReason string     `json:"closed_reason,omitempty"`

	MembershipID  *uuid.UUID `json:"membership_id,omitempty"`
	AuthorityTier string     `json:"authority_tier,omitempty"`
	IsOwner       bool       `json:"is_owner"`
}

type Module struct {
	ModuleKey      string         `json:"module_key"`
	DisplayName    string         `json:"display_name"`
	Category       string         `json:"category"`
	EnabledDefault bool           `json:"enabled_default"`
	LicenseScope   string         `json:"license_scope"`
	Metadata       map[string]any `json:"metadata"`
}

type OrganizationSubscription struct {
	ID                 uuid.UUID      `json:"id"`
	OrganizationID     uuid.UUID      `json:"organization_id"`
	PlanID             *uuid.UUID     `json:"plan_id,omitempty"`
	PlanCode           string         `json:"plan_code,omitempty"`
	PlanName           string         `json:"plan_name,omitempty"`
	Status             string         `json:"status"`
	TrialEndsAt        *time.Time     `json:"trial_ends_at,omitempty"`
	CurrentPeriodStart *time.Time     `json:"current_period_start,omitempty"`
	CurrentPeriodEnd   *time.Time     `json:"current_period_end,omitempty"`
	Metadata           map[string]any `json:"metadata"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

type OnboardingOrganizationInput struct {
	OrganizationName string   `json:"organization_name"`
	Description      string   `json:"description,omitempty"`
	EnabledModules   []string `json:"enabled_modules,omitempty"`
}

type OnboardingOrganizationResponse struct {
	Profile      UserProfile         `json:"profile"`
	Organization OrganizationAccount `json:"organization"`
}

type SampleTenantResponse struct {
	Organization            OrganizationAccount `json:"organization"`
	OwnerUserID             uuid.UUID           `json:"owner_user_id"`
	OwnerEmail              string              `json:"owner_email"`
	EnabledModules          []string            `json:"enabled_modules"`
	TenantDatabaseStatus    string              `json:"tenant_database_status"`
	TenantDatabaseName      string              `json:"tenant_database_name,omitempty"`
	IndustrySolutionPackage string              `json:"industry_solution_package"`
}

type CreateSampleTenantRecord struct {
	ActorID          uuid.UUID
	OwnerEmail       string
	OwnerName        string
	PasswordHash     string
	OrganizationName string
	Description      string
	EnabledModules   []string
	SampleKey        string
}

type CreatedSampleTenant struct {
	Organization OrganizationAccount
	OwnerUserID  uuid.UUID
	OwnerEmail   string
	Modules      []string
}

type UpdateOrganizationModulesInput struct {
	EnabledModules []string `json:"enabled_modules"`
}

type CloseOrganizationInput struct {
	Reason string `json:"reason,omitempty"`
}

type CreateInvitationInput struct {
	Email         string         `json:"email"`
	Name          string         `json:"name,omitempty"`
	RoleID        *uuid.UUID     `json:"role_id,omitempty"`
	AuthorityTier string         `json:"authority_tier,omitempty"`
	ExpiresInDays int            `json:"expires_in_days,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

type Invitation struct {
	ID             uuid.UUID      `json:"id"`
	OrganizationID uuid.UUID      `json:"organization_id"`
	Email          string         `json:"email"`
	Name           string         `json:"name,omitempty"`
	RoleID         *uuid.UUID     `json:"role_id,omitempty"`
	AuthorityTier  string         `json:"authority_tier"`
	Status         string         `json:"status"`
	InvitedBy      *uuid.UUID     `json:"invited_by,omitempty"`
	AcceptedBy     *uuid.UUID     `json:"accepted_by,omitempty"`
	ExpiresAt      time.Time      `json:"expires_at"`
	Metadata       map[string]any `json:"metadata"`
	AcceptedAt     *time.Time     `json:"accepted_at,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	Token          string         `json:"token,omitempty"`
}

type AcceptInvitationInput struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AcceptInvitationResponse struct {
	Profile      UserProfile         `json:"profile"`
	Organization OrganizationAccount `json:"organization"`
}
