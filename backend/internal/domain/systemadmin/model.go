package systemadmin

import (
	"time"

	"github.com/google/uuid"
)

const (
	IndustrySolutionChangePending  = "pending"
	IndustrySolutionChangeApproved = "approved"
	IndustrySolutionChangeRejected = "rejected"
	IndustrySolutionChangeApplied  = "applied"
	IndustrySolutionChangeFailed   = "failed"
)

type PlatformMaster struct {
	MasterKey      string         `json:"master_key"`
	ModuleKey      string         `json:"module_key"`
	EntityType     string         `json:"entity_type"`
	SourceTable    string         `json:"source_table"`
	SourcePK       string         `json:"source_pk"`
	Title          string         `json:"title"`
	Status         string         `json:"status"`
	OrganizationID *uuid.UUID     `json:"organization_id,omitempty"`
	Payload        map[string]any `json:"payload"`
	Metadata       map[string]any `json:"metadata"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type PlatformDetail struct {
	DetailKey  string         `json:"detail_key"`
	MasterKey  string         `json:"master_key"`
	DetailType string         `json:"detail_type"`
	FieldKey   string         `json:"field_key"`
	LineNo     int            `json:"line_no"`
	Payload    map[string]any `json:"payload"`
	Metadata   map[string]any `json:"metadata"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

type PlatformPermissionProfile struct {
	Role        string          `json:"role"`
	Permissions map[string]bool `json:"permissions"`
	MenuItems   []string        `json:"menu_items"`
}

type PlatformFeature struct {
	FeatureKey     string         `json:"feature_key"`
	ParentKey      string         `json:"parent_key,omitempty"`
	ModuleKey      string         `json:"module_key"`
	Category       string         `json:"category"`
	Title          string         `json:"title"`
	Description    string         `json:"description,omitempty"`
	Status         string         `json:"status"`
	SortOrder      int            `json:"sort_order"`
	PermissionKeys []string       `json:"permission_keys"`
	Metadata       map[string]any `json:"metadata"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type PlatformMenuItem struct {
	MenuKey             string         `json:"menu_key"`
	ParentKey           string         `json:"parent_key,omitempty"`
	FeatureKey          string         `json:"feature_key,omitempty"`
	LabelKey            string         `json:"label_key"`
	Icon                string         `json:"icon,omitempty"`
	Route               string         `json:"route,omitempty"`
	RequiredPermissions []string       `json:"required_permissions"`
	Status              string         `json:"status"`
	SortOrder           int            `json:"sort_order"`
	Metadata            map[string]any `json:"metadata"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
}

type PlatformPermission struct {
	PermissionKey string         `json:"permission_key"`
	Name          string         `json:"name"`
	Description   string         `json:"description,omitempty"`
	Category      string         `json:"category"`
	Status        string         `json:"status"`
	Metadata      map[string]any `json:"metadata"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

type PlatformRole struct {
	RoleKey     string         `json:"role_key"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Status      string         `json:"status"`
	IsSystem    bool           `json:"is_system"`
	Permissions []string       `json:"permissions,omitempty"`
	Metadata    map[string]any `json:"metadata"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type PlatformUser struct {
	UserID        uuid.UUID      `json:"user_id"`
	Name          string         `json:"name"`
	Email         string         `json:"email"`
	AccountStatus string         `json:"account_status"`
	Roles         []string       `json:"roles"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

type CreatePlatformFeatureInput struct {
	FeatureKey     string         `json:"feature_key"`
	ParentKey      string         `json:"parent_key,omitempty"`
	ModuleKey      string         `json:"module_key"`
	Category       string         `json:"category,omitempty"`
	Title          string         `json:"title"`
	Description    string         `json:"description,omitempty"`
	SortOrder      int            `json:"sort_order,omitempty"`
	PermissionKeys []string       `json:"permission_keys,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type CreatePlatformFeatureRecord struct {
	CreatePlatformFeatureInput
	Status  string
	ActorID uuid.UUID
}

type SetPlatformRolePermissionsInput struct {
	PermissionKeys []string `json:"permission_keys"`
}

type CreatePlatformUserInput struct {
	Name     string         `json:"name"`
	Email    string         `json:"email"`
	Roles    []string       `json:"roles,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type CreatePlatformUserResponse struct {
	User              PlatformUser `json:"user"`
	TemporaryPassword string       `json:"temporary_password"`
}

type CreatePlatformUserRecord struct {
	Name         string
	Email        string
	PasswordHash string
	Roles        []string
	Metadata     map[string]any
	ActorID      uuid.UUID
}

type ResetPlatformUserPasswordResponse struct {
	UserID            uuid.UUID `json:"user_id"`
	TemporaryPassword string    `json:"temporary_password"`
}

const (
	DatabaseMaintenancePendingApproval = "pending_approval"
	DatabaseMaintenanceApproved        = "approved"
	DatabaseMaintenanceRejected        = "rejected"
	DatabaseMaintenanceCancelled       = "cancelled"
	DatabaseMaintenanceCompleted       = "completed"
	DatabaseMaintenanceFailed          = "failed"
)

type DatabaseMaintenanceJob struct {
	ID           uuid.UUID      `json:"id"`
	JobType      string         `json:"job_type"`
	Scope        string         `json:"scope"`
	Status       string         `json:"status"`
	Reason       string         `json:"reason,omitempty"`
	BackupRef    string         `json:"backup_ref,omitempty"`
	RequestedBy  *uuid.UUID     `json:"requested_by,omitempty"`
	ReviewedBy   *uuid.UUID     `json:"reviewed_by,omitempty"`
	ReviewReason string         `json:"review_reason,omitempty"`
	Result       map[string]any `json:"result,omitempty"`
	Metadata     map[string]any `json:"metadata"`
	CreatedAt    time.Time      `json:"created_at"`
	ReviewedAt   *time.Time     `json:"reviewed_at,omitempty"`
	CompletedAt  *time.Time     `json:"completed_at,omitempty"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type CreateDatabaseMaintenanceJobInput struct {
	JobType   string         `json:"job_type"`
	Scope     string         `json:"scope,omitempty"`
	Reason    string         `json:"reason,omitempty"`
	BackupRef string         `json:"backup_ref,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type CreateDatabaseMaintenanceJobRecord struct {
	CreateDatabaseMaintenanceJobInput
	Status      string
	RequestedBy uuid.UUID
}

type ReviewDatabaseMaintenanceJobInput struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason,omitempty"`
}

type ReviewDatabaseMaintenanceJobRecord struct {
	JobID        uuid.UUID
	Status       string
	ReviewedBy   uuid.UUID
	ReviewReason string
}

type IndustrySolutionTableInput struct {
	Name         string                       `json:"name"`
	PreviousName string                       `json:"previous_name,omitempty"`
	DisplayName  string                       `json:"display_name,omitempty"`
	Fields       []IndustrySolutionFieldInput `json:"fields"`
	Metadata     map[string]any               `json:"metadata,omitempty"`
}

type IndustrySolutionFieldInput struct {
	Name         string `json:"name"`
	PreviousName string `json:"previous_name,omitempty"`
	DataType     string `json:"data_type"`
	Nullable     bool   `json:"nullable"`
	Default      string `json:"default,omitempty"`
}

type CreateIndustrySolutionTableFieldChangeInput struct {
	OrganizationID          uuid.UUID                  `json:"organization_id"`
	IndustryKey             string                     `json:"industry_key"`
	PackageKey              string                     `json:"package_key"`
	Table                   IndustrySolutionTableInput `json:"table"`
	CurrentSolutionManifest *IndustrySolutionManifest  `json:"current_solution_manifest,omitempty"`
	Reason                  string                     `json:"reason,omitempty"`
}

type OrganizationIndustrySolutionTarget struct {
	OrganizationID               uuid.UUID      `json:"organization_id"`
	TargetSchemaName             string         `json:"target_schema_name"`
	TemplateVersion              string         `json:"template_version"`
	Status                       string         `json:"status"`
	LastChangeRequestID          *uuid.UUID     `json:"last_change_request_id,omitempty"`
	TenantDatabaseDeploymentMode string         `json:"tenant_database_deployment_mode,omitempty"`
	TenantDatabaseClusterKey     string         `json:"tenant_database_cluster_key,omitempty"`
	TenantDatabaseRegion         string         `json:"tenant_database_region,omitempty"`
	TenantDatabaseName           string         `json:"tenant_database_name,omitempty"`
	TenantDatabaseStatus         string         `json:"tenant_database_status,omitempty"`
	Metadata                     map[string]any `json:"metadata"`
	CreatedAt                    time.Time      `json:"created_at"`
	UpdatedAt                    time.Time      `json:"updated_at"`
}

type IndustrySolutionChangeRequest struct {
	ID               uuid.UUID                `json:"id"`
	OrganizationID   uuid.UUID                `json:"organization_id"`
	TargetSchemaName string                   `json:"target_schema_name"`
	RequestType      string                   `json:"request_type"`
	Status           string                   `json:"status"`
	Reason           string                   `json:"reason"`
	SolutionManifest IndustrySolutionManifest `json:"solution_manifest"`
	Statements       []string                 `json:"statements"`
	RiskLevel        string                   `json:"risk_level"`
	Diff             []IndustrySolutionDiff   `json:"diff"`
	RequestedBy      *uuid.UUID               `json:"requested_by,omitempty"`
	ReviewedBy       *uuid.UUID               `json:"reviewed_by,omitempty"`
	AppliedBy        *uuid.UUID               `json:"applied_by,omitempty"`
	ReviewReason     string                   `json:"review_reason,omitempty"`
	CreatedAt        time.Time                `json:"created_at"`
	ReviewedAt       *time.Time               `json:"reviewed_at,omitempty"`
	AppliedAt        *time.Time               `json:"applied_at,omitempty"`
	UpdatedAt        time.Time                `json:"updated_at"`
}

func (r *IndustrySolutionChangeRequest) SolutionManifestHas(key string) bool {
	if r == nil || r.SolutionManifest.Metadata == nil {
		return false
	}
	value, ok := r.SolutionManifest.Metadata[key]
	if !ok || value == nil {
		return false
	}
	switch typed := value.(type) {
	case []map[string]any:
		return len(typed) > 0
	case []string:
		return len(typed) > 0
	case []any:
		return len(typed) > 0
	default:
		return true
	}
}

type IndustrySolutionApplyJob struct {
	ID               uuid.UUID      `json:"id"`
	ChangeRequestID  uuid.UUID      `json:"change_request_id"`
	OrganizationID   uuid.UUID      `json:"organization_id"`
	TargetSchemaName string         `json:"target_schema_name"`
	Status           string         `json:"status"`
	Statements       []string       `json:"statements"`
	ErrorMessage     string         `json:"error_message,omitempty"`
	Metadata         map[string]any `json:"metadata"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

type IndustrySolutionVerificationReport struct {
	ChangeRequestID  uuid.UUID                           `json:"change_request_id"`
	OrganizationID   uuid.UUID                           `json:"organization_id"`
	TargetSchemaName string                              `json:"target_schema_name"`
	RequestStatus    string                              `json:"request_status"`
	Status           string                              `json:"status"`
	RiskLevel        string                              `json:"risk_level"`
	StatementCount   int                                 `json:"statement_count"`
	BlockingIssues   int                                 `json:"blocking_issues"`
	CanApply         bool                                `json:"can_apply"`
	Checks           []IndustrySolutionVerificationCheck `json:"checks"`
}

type IndustrySolutionVerificationCheck struct {
	Key      string         `json:"key"`
	Status   string         `json:"status"`
	Message  string         `json:"message"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type IndustrySolutionAssetDiff struct {
	AssetType      string   `json:"asset_type"`
	AssetKey       string   `json:"asset_key"`
	Action         string   `json:"action"`
	RiskLevel      string   `json:"risk_level"`
	CurrentVersion string   `json:"current_version,omitempty"`
	DesiredVersion string   `json:"desired_version,omitempty"`
	Summary        string   `json:"summary"`
	BlockingReason string   `json:"blocking_reason,omitempty"`
	DependsOn      []string `json:"depends_on,omitempty"`
}

type IndustrySolutionApplyAssetResult struct {
	AssetKey     string         `json:"asset_key"`
	AssetType    string         `json:"asset_type"`
	Status       string         `json:"status"`
	Target       string         `json:"target"`
	ErrorMessage string         `json:"error_message,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type CreateIndustrySolutionChangeRequestInput struct {
	OrganizationID          uuid.UUID                 `json:"organization_id"`
	RequestType             string                    `json:"request_type"`
	Reason                  string                    `json:"reason,omitempty"`
	SolutionManifest        IndustrySolutionManifest  `json:"solution_manifest"`
	CurrentSolutionManifest *IndustrySolutionManifest `json:"current_solution_manifest,omitempty"`
}

type ERPSolutionFlowRequest struct {
	OrganizationID  uuid.UUID                 `json:"organization_id"`
	IndustryKey     string                    `json:"industry_key"`
	PackageKey      string                    `json:"package_key"`
	Name            string                    `json:"name"`
	EnabledModules  []string                  `json:"enabled_modules"`
	CurrentTemplate *IndustrySolutionManifest `json:"current_template,omitempty"`
}

type CreateIndustrySolutionChangeRequestRecord struct {
	OrganizationID   uuid.UUID
	TargetSchemaName string
	RequestType      string
	Reason           string
	SolutionManifest IndustrySolutionManifest
	Statements       []string
	RiskLevel        string
	Diff             []IndustrySolutionDiff
	RequestedBy      uuid.UUID
}
