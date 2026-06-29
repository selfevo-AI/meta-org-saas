package industry

import (
	"time"

	"github.com/google/uuid"
)

const (
	StatusDraft    = "draft"
	StatusActive   = "active"
	StatusArchived = "archived"

	AssetTypeSolutionManifest = "solution_manifest"
	AssetTypeModule           = "module"
	AssetTypeRuntimeEntity    = "runtime_entity"
	AssetTypeRuntimeOperation = "runtime_operation"
	AssetTypeSkillStructure   = "skill_structure"
	AssetTypeSkill            = "skill"
	AssetTypeKnowledgeSource  = "knowledge_source"
	AssetTypeModelPolicy      = "model_policy"
	AssetTypeI18n             = "i18n"

	PublicationPending  = "pending"
	PublicationApproved = "approved"
	PublicationRejected = "rejected"
)

type Industry struct {
	IndustryKey string         `json:"industry_key"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Status      string         `json:"status"`
	Metadata    map[string]any `json:"metadata"`
	CreatedBy   *uuid.UUID     `json:"created_by,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type Package struct {
	ID          uuid.UUID      `json:"id"`
	IndustryKey string         `json:"industry_key"`
	PackageKey  string         `json:"package_key"`
	Version     int            `json:"version"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Status      string         `json:"status"`
	Assets      []PackageAsset `json:"assets"`
	Metadata    map[string]any `json:"metadata"`
	CreatedBy   *uuid.UUID     `json:"created_by,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type PackageAsset struct {
	ID        uuid.UUID      `json:"id,omitempty"`
	PackageID uuid.UUID      `json:"package_id,omitempty"`
	AssetKey  string         `json:"asset_key"`
	AssetType string         `json:"asset_type"`
	Payload   map[string]any `json:"payload"`
	Metadata  map[string]any `json:"metadata"`
	CreatedAt time.Time      `json:"created_at,omitempty"`
	UpdatedAt time.Time      `json:"updated_at,omitempty"`
}

type OrganizationAdoption struct {
	OrganizationID uuid.UUID      `json:"organization_id"`
	IndustryKey    string         `json:"industry_key"`
	PackageID      uuid.UUID      `json:"package_id"`
	Primary        bool           `json:"primary"`
	EnabledModules []string       `json:"enabled_modules"`
	Status         string         `json:"status"`
	Metadata       map[string]any `json:"metadata"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type Extension struct {
	ID             uuid.UUID      `json:"id"`
	OrganizationID uuid.UUID      `json:"organization_id"`
	IndustryKey    string         `json:"industry_key"`
	PackageID      uuid.UUID      `json:"package_id"`
	ExtensionKey   string         `json:"extension_key"`
	Name           string         `json:"name"`
	Description    string         `json:"description,omitempty"`
	Status         string         `json:"status"`
	Assets         []PackageAsset `json:"assets"`
	Metadata       map[string]any `json:"metadata"`
	CreatedBy      *uuid.UUID     `json:"created_by,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type PublicationRequest struct {
	ID                   uuid.UUID      `json:"id"`
	ExtensionID          uuid.UUID      `json:"extension_id"`
	SourceOrganizationID uuid.UUID      `json:"source_organization_id"`
	IndustryKey          string         `json:"industry_key"`
	Status               string         `json:"status"`
	Reason               string         `json:"reason,omitempty"`
	ReviewReason         string         `json:"review_reason,omitempty"`
	RequestedBy          *uuid.UUID     `json:"requested_by,omitempty"`
	ReviewedBy           *uuid.UUID     `json:"reviewed_by,omitempty"`
	Metadata             map[string]any `json:"metadata"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
	ReviewedAt           *time.Time     `json:"reviewed_at,omitempty"`
}

type PublicationGateResult struct {
	Key      string         `json:"key"`
	Status   string         `json:"status"`
	Message  string         `json:"message"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type KnowledgeSource struct {
	ID              uuid.UUID      `json:"id"`
	IndustryKey     string         `json:"industry_key"`
	OrganizationID  *uuid.UUID     `json:"organization_id,omitempty"`
	SourceKey       string         `json:"source_key"`
	Name            string         `json:"name"`
	SourceType      string         `json:"source_type"`
	AdapterKey      string         `json:"adapter_key"`
	ReferenceURI    string         `json:"reference_uri,omitempty"`
	SyncStatus      string         `json:"sync_status"`
	Permission      map[string]any `json:"permission"`
	RetrievalConfig map[string]any `json:"retrieval_config"`
	Metadata        map[string]any `json:"metadata"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

type CreateIndustryInput struct {
	IndustryKey string         `json:"industry_key"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Status      string         `json:"status,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type CreatePackageInput struct {
	IndustryKey string         `json:"industry_key"`
	PackageKey  string         `json:"package_key"`
	Version     int            `json:"version,omitempty"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Status      string         `json:"status,omitempty"`
	Assets      []PackageAsset `json:"assets"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type UpdatePackageInput struct {
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Status      string         `json:"status,omitempty"`
	Assets      []PackageAsset `json:"assets"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type UpdatePackageRecord struct {
	PackageID   uuid.UUID
	Name        string
	Description string
	Status      string
	Assets      []PackageAsset
	Metadata    map[string]any
	ActorID     uuid.UUID
}

type ApplyPackageInput struct {
	PackageID      uuid.UUID      `json:"package_id"`
	OrganizationID uuid.UUID      `json:"organization_id"`
	ModuleKeys     []string       `json:"module_keys"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type CreateExtensionInput struct {
	OrganizationID uuid.UUID      `json:"organization_id"`
	IndustryKey    string         `json:"industry_key"`
	PackageID      uuid.UUID      `json:"package_id"`
	ExtensionKey   string         `json:"extension_key"`
	Name           string         `json:"name"`
	Description    string         `json:"description,omitempty"`
	Assets         []PackageAsset `json:"assets"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}
