package tenantprojection

import (
	"time"

	"github.com/google/uuid"
)

type OutboxEvent struct {
	EventID          uuid.UUID
	EventType        string
	EventVersion     int
	OrganizationID   uuid.UUID
	ActorID          *uuid.UUID
	ActorType        string
	AuthorityTier    string
	AggregateType    string
	AggregateID      string
	AggregateVersion int64
	TraceID          uuid.UUID
	CausationID      uuid.UUID
	CorrelationID    uuid.UUID
	OccurredAt       time.Time
	SchemaVersion    string
	Payload          []byte
	Metadata         []byte
	AttemptCount     int
}

type OperationalSnapshot struct {
	OrganizationID            uuid.UUID
	OpenRequirements          int64
	ActiveProjects            int64
	ProjectsByStatus          map[string]int64
	OverBudgetProjects        int64
	ProjectCostToday          float64
	ProjectCostMonthToDate    float64
	ProjectCostCurrency       string
	WorkflowTemplates         int64
	ActiveWorkflowTemplates   int64
	WorkflowInstances         int64
	WorkflowInstancesByStatus map[string]int64
	WorkflowTasksByStatus     map[string]int64
	WorkflowDecisions7d       int64
}

type ActivityItem struct {
	OrganizationID uuid.UUID
	ItemType       string
	ItemID         string
	Title          string
	Status         string
	OccurredAt     time.Time
}

type WorkflowEntity struct {
	OrganizationID uuid.UUID
	WorkflowID     uuid.UUID
	ProjectID      *uuid.UUID
	Status         string
	CurrentStage   int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ProjectionFreshness struct {
	SourceEventID    uuid.UUID `json:"source_event_id"`
	SourceOccurredAt time.Time `json:"source_occurred_at"`
	ProjectedAt      time.Time `json:"projected_at"`
	LagMilliseconds  int64     `json:"lag_ms"`
	SnapshotVersion  int64     `json:"snapshot_version"`
}

type WorkerStats struct {
	WorkerID            string    `json:"worker_id"`
	TargetsDiscovered   uint64    `json:"targets_discovered_total"`
	TargetsMigrated     uint64    `json:"targets_migrated_total"`
	EventsClaimed       uint64    `json:"events_claimed_total"`
	EventsProjected     uint64    `json:"events_projected_total"`
	EventsFailed        uint64    `json:"events_failed_total"`
	ProjectionRuns      uint64    `json:"projection_runs_total"`
	ProjectionFailures  uint64    `json:"projection_failures_total"`
	LastProjectionAt    time.Time `json:"last_projection_at,omitempty"`
	LastProjectionLagMs int64     `json:"last_projection_lag_ms"`
	MaxObservedLagMs    int64     `json:"max_observed_lag_ms"`
	LastError           string    `json:"last_error,omitempty"`
	Running             bool      `json:"running"`
}
