package monitoringagent

import (
	"time"

	"github.com/google/uuid"
)

const (
	TriggerManual    = "manual"
	TriggerScheduled = "scheduled"

	RunRunning   = "running"
	RunCompleted = "completed"
	RunFailed    = "failed"

	SeverityCritical = "critical"
	SeverityHigh     = "high"
	SeverityMedium   = "medium"
	SeverityLow      = "low"

	SignalAIFailure                   = "ai_invocation_failure"
	SignalAIStreamDisconnect          = "ai_stream_disconnect"
	SignalToolFailure                 = "tool_execution_failure"
	SignalToolApprovalBacklog         = "tool_approval_backlog"
	SignalContextBuildFailure         = "context_build_failure"
	SignalContextRuleGap              = "context_rule_gap"
	SignalSchemaChangeFailure         = "schema_change_failure"
	SignalFinanceCallbackFailure      = "finance_callback_failure"
	SignalERPActionFailure            = "erp_action_failure"
	SignalCostWithoutBusinessProgress = "cost_without_business_progress"
	SignalStaleProposal               = "stale_proposal"
)

type MonitoringAgentRun struct {
	ID                   uuid.UUID      `json:"id"`
	TriggerType          string         `json:"trigger_type"`
	OrganizationID       *uuid.UUID     `json:"organization_id,omitempty"`
	Status               string         `json:"status"`
	LookbackStartedAt    time.Time      `json:"lookback_started_at"`
	LookbackEndedAt      time.Time      `json:"lookback_ended_at"`
	SignalsCreated       int            `json:"signals_created"`
	DuplicatesSuppressed int            `json:"duplicates_suppressed"`
	Summary              map[string]any `json:"summary"`
	ErrorMessage         string         `json:"error_message,omitempty"`
	StartedAt            time.Time      `json:"started_at"`
	CompletedAt          *time.Time     `json:"completed_at,omitempty"`
}

type MonitoringAgentStatus struct {
	SchedulerEnabled bool                `json:"scheduler_enabled"`
	DailyTime        string              `json:"daily_time"`
	LookbackHours    int                 `json:"lookback_hours"`
	MaxSignalsPerRun int                 `json:"max_signals_per_run"`
	LatestRun        *MonitoringAgentRun `json:"latest_run,omitempty"`
}

type RunInput struct {
	TriggerType    string     `json:"trigger_type,omitempty"`
	OrganizationID *uuid.UUID `json:"organization_id,omitempty"`
	LookbackHours  int        `json:"lookback_hours,omitempty"`
}

type ListRunsFilter struct {
	OrganizationID *uuid.UUID
	Limit          int
}

type ScanWindow struct {
	OrganizationID *uuid.UUID
	StartedAt      time.Time
	EndedAt        time.Time
	Limit          int
}

type OperationalFinding struct {
	Category        string
	OrganizationID  *uuid.UUID
	EntityType      string
	EntityID        string
	Reason          string
	Severity        string
	OccurredAt      time.Time
	Data            map[string]any
	ProposalPayload map[string]any
}

type SignalWrite struct {
	SignalType string
	Source     string
	Priority   int
	Data       map[string]any
}

type ContextProposalWrite struct {
	DictionaryVersionID uuid.UUID
	ProposalType        string
	Title               string
	Summary             string
	Payload             map[string]any
	Status              string
}

type ServiceConfig struct {
	LookbackHours    int
	MaxSignalsPerRun int
	SchedulerEnabled bool
	DailyTime        string
	Now              func() time.Time
}
