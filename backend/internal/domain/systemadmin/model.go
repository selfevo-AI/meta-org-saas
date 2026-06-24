package systemadmin

import (
	"time"

	"github.com/google/uuid"
)

const (
	SchemaChangePending  = "pending"
	SchemaChangeApproved = "approved"
	SchemaChangeRejected = "rejected"
	SchemaChangeApplied  = "applied"
	SchemaChangeFailed   = "failed"
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

type OrganizationSchemaTarget struct {
	OrganizationID      uuid.UUID      `json:"organization_id"`
	SchemaName          string         `json:"schema_name"`
	TemplateVersion     string         `json:"template_version"`
	Status              string         `json:"status"`
	LastChangeRequestID *uuid.UUID     `json:"last_change_request_id,omitempty"`
	Metadata            map[string]any `json:"metadata"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
}

type SchemaChangeRequest struct {
	ID             uuid.UUID     `json:"id"`
	OrganizationID uuid.UUID     `json:"organization_id"`
	SchemaName     string        `json:"schema_name"`
	RequestType    string        `json:"request_type"`
	Status         string        `json:"status"`
	Reason         string        `json:"reason"`
	SchemaPackage  SchemaPackage `json:"schema_package"`
	Statements     []string      `json:"statements"`
	RiskLevel      string        `json:"risk_level"`
	Diff           []SchemaDiff  `json:"diff"`
	RequestedBy    *uuid.UUID    `json:"requested_by,omitempty"`
	ReviewedBy     *uuid.UUID    `json:"reviewed_by,omitempty"`
	AppliedBy      *uuid.UUID    `json:"applied_by,omitempty"`
	ReviewReason   string        `json:"review_reason,omitempty"`
	CreatedAt      time.Time     `json:"created_at"`
	ReviewedAt     *time.Time    `json:"reviewed_at,omitempty"`
	AppliedAt      *time.Time    `json:"applied_at,omitempty"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

func (r *SchemaChangeRequest) SchemaPackageHas(key string) bool {
	if r == nil || r.SchemaPackage.Metadata == nil {
		return false
	}
	value, ok := r.SchemaPackage.Metadata[key]
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

type SchemaApplyJob struct {
	ID              uuid.UUID      `json:"id"`
	ChangeRequestID uuid.UUID      `json:"change_request_id"`
	OrganizationID  uuid.UUID      `json:"organization_id"`
	SchemaName      string         `json:"schema_name"`
	Status          string         `json:"status"`
	Statements      []string       `json:"statements"`
	ErrorMessage    string         `json:"error_message,omitempty"`
	Metadata        map[string]any `json:"metadata"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

type SchemaVerificationReport struct {
	ChangeRequestID uuid.UUID                 `json:"change_request_id"`
	OrganizationID  uuid.UUID                 `json:"organization_id"`
	SchemaName      string                    `json:"schema_name"`
	RequestStatus   string                    `json:"request_status"`
	Status          string                    `json:"status"`
	RiskLevel       string                    `json:"risk_level"`
	StatementCount  int                       `json:"statement_count"`
	BlockingIssues  int                       `json:"blocking_issues"`
	CanApply        bool                      `json:"can_apply"`
	Checks          []SchemaVerificationCheck `json:"checks"`
}

type SchemaVerificationCheck struct {
	Key      string         `json:"key"`
	Status   string         `json:"status"`
	Message  string         `json:"message"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type PackageAssetDiff struct {
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

type CreateSchemaChangeRequestInput struct {
	OrganizationID       uuid.UUID      `json:"organization_id"`
	RequestType          string         `json:"request_type"`
	Reason               string         `json:"reason,omitempty"`
	SchemaPackage        SchemaPackage  `json:"schema_package"`
	CurrentSchemaPackage *SchemaPackage `json:"current_schema_package,omitempty"`
}

type ERPSolutionFlowRequest struct {
	OrganizationID  uuid.UUID      `json:"organization_id"`
	IndustryKey     string         `json:"industry_key"`
	PackageKey      string         `json:"package_key"`
	Name            string         `json:"name"`
	EnabledModules  []string       `json:"enabled_modules"`
	CurrentTemplate *SchemaPackage `json:"current_template,omitempty"`
}

type CreateSchemaChangeRequestRecord struct {
	OrganizationID uuid.UUID
	SchemaName     string
	RequestType    string
	Reason         string
	SchemaPackage  SchemaPackage
	Statements     []string
	RiskLevel      string
	Diff           []SchemaDiff
	RequestedBy    uuid.UUID
}
