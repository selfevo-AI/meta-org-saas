package smtmes

import (
	"time"

	"github.com/google/uuid"
)

// SMT MES edge integration and shop-floor read surfaces for a tenant.

const (
	EdgeInstanceStatusActive   = "active"
	EdgeInstanceStatusDisabled = "disabled"

	EdgeEventStatusPending   = "pending"
	EdgeEventStatusProcessed = "processed"
	EdgeEventStatusSkipped   = "skipped"
	EdgeEventStatusFailed    = "failed"
)

// EdgeInstance is a registered SMT-MES shop-floor instance.
type EdgeInstance struct {
	InstanceKey      uuid.UUID  `json:"instance_key"`
	OrganizationID   uuid.UUID  `json:"organization_id"`
	Name             string     `json:"name"`
	BaseURL          string     `json:"base_url"`
	APIKeyMasked     string     `json:"api_key_masked,omitempty"`
	SubscribedEvents string     `json:"subscribed_events"`
	Status           string     `json:"status"`
	AutoPush         bool       `json:"auto_push"`
	LastPollID       int64      `json:"last_poll_id"`
	LastSyncAt       *time.Time `json:"last_sync_at,omitempty"`
	LastEventAt      *time.Time `json:"last_event_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	WebhookPath      string     `json:"webhook_path,omitempty"`
}

// EdgeInstanceRecord carries the stored instance row including secrets.
type EdgeInstanceRecord struct {
	EdgeInstance
	APIKeyCipher        string
	WebhookSecretCipher string
}

// ResolvedInstance is the platform-side resolution of a webhook/pull target.
type ResolvedInstance struct {
	EdgeInstanceRecord
	TenantDatabaseDeploymentMode string
	TenantDatabaseName           string
	TenantDatabaseStatus         string
}

// CreateEdgeInstanceInput registers a new SMT-MES instance.
type CreateEdgeInstanceInput struct {
	Name             string `json:"name"`
	BaseURL          string `json:"base_url"`
	APIKey           string `json:"api_key"`
	WebhookSecret    string `json:"webhook_secret"`
	SubscribedEvents string `json:"subscribed_events,omitempty"`
	AutoPush         bool   `json:"auto_push"`
}

// UpdateEdgeInstanceInput patches an instance.
type UpdateEdgeInstanceInput struct {
	Name             string `json:"name,omitempty"`
	BaseURL          string `json:"base_url,omitempty"`
	APIKey           string `json:"api_key,omitempty"`
	WebhookSecret    string `json:"webhook_secret,omitempty"`
	SubscribedEvents string `json:"subscribed_events,omitempty"`
	Status           string `json:"status,omitempty"`
	AutoPush         *bool  `json:"auto_push,omitempty"`
}

// EdgeEventRecord is one inbound event row (smt_edge_events).
type EdgeEventRecord struct {
	ID          int64      `json:"id"`
	InstanceKey uuid.UUID  `json:"instance_key"`
	EventID     int64      `json:"event_id"`
	EventType   string     `json:"event_type"`
	Ref         string     `json:"ref"`
	Status      string     `json:"status"`
	Error       string     `json:"error,omitempty"`
	ReceivedAt  time.Time  `json:"received_at"`
	ProcessedAt *time.Time `json:"processed_at,omitempty"`
}

// SyncLogRecord is one outbound push log row (smt_edge_sync_log).
type SyncLogRecord struct {
	ID          int64          `json:"id"`
	InstanceKey uuid.UUID      `json:"instance_key"`
	Scope       string         `json:"scope"`
	Status      string         `json:"status"`
	Message     string         `json:"message"`
	Payload     map[string]any `json:"payload,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

// SettingsView merges defaults with stored tenant values.
type SettingsView struct {
	Values  map[string]string `json:"values"`
	Stored  map[string]string `json:"stored"`
	Updated []string          `json:"updated"`
}

// UpdateSettingsInput patches SMT settings.
type UpdateSettingsInput struct {
	Values map[string]string `json:"values"`
}

// PushInput selects outbound sync scopes.
type PushInput struct {
	Scopes []string `json:"scopes,omitempty"`
}

// PushResult summarizes one outbound push.
type PushResult struct {
	InstanceKey uuid.UUID        `json:"instance_key"`
	Results     []map[string]any `json:"results"`
}
