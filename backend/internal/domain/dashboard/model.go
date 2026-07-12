package dashboard

import "time"

type Overview struct {
	GeneratedAt   time.Time            `json:"generated_at"`
	Identity      IdentitySummary      `json:"identity"`
	Organization  OrganizationSummary  `json:"organization"`
	Workflow      WorkflowSummary      `json:"workflow"`
	Capability    CapabilitySummary    `json:"capability"`
	Observability ObservabilitySummary `json:"observability"`
	Verification  VerificationSummary  `json:"verification"`
	Governance    GovernanceSummary    `json:"governance"`
	Evolution     EvolutionSummary     `json:"evolution"`
	RecentEvents  []RecentEvent        `json:"recent_events"`
}

type IdentitySummary struct {
	Users        int64 `json:"users"`
	ActiveAgents int64 `json:"active_agents"`
	TotalAgents  int64 `json:"total_agents"`
	Roles        int64 `json:"roles"`
}

type OrganizationSummary struct {
	Organizations int64            `json:"organizations"`
	MVRUs         int64            `json:"mvrus"`
	MVRUsByStatus map[string]int64 `json:"mvrus_by_status"`
	Members       int64            `json:"members"`
	Relationships int64            `json:"relationships"`
}

type WorkflowSummary struct {
	Templates         int64            `json:"templates"`
	ActiveTemplates   int64            `json:"active_templates"`
	Instances         int64            `json:"instances"`
	InstancesByStatus map[string]int64 `json:"instances_by_status"`
	TasksByStatus     map[string]int64 `json:"tasks_by_status"`
	Decisions7d       int64            `json:"decisions_7d"`
	SourceOccurredAt  time.Time        `json:"source_occurred_at,omitempty"`
	ProjectedAt       time.Time        `json:"projected_at,omitempty"`
	ProjectionLagMs   int64            `json:"projection_lag_ms"`
	SnapshotVersion   int64            `json:"snapshot_version"`
}

type CapabilitySummary struct {
	Capabilities         int64   `json:"capabilities"`
	ActiveCapabilities   int64   `json:"active_capabilities"`
	Bindings             int64   `json:"bindings"`
	Invocations24h       int64   `json:"invocations_24h"`
	FailedInvocations24h int64   `json:"failed_invocations_24h"`
	AverageDurationMs    float64 `json:"average_duration_ms"`
	Cost24h              float64 `json:"cost_24h"`
}

type ObservabilitySummary struct {
	ActiveTraces      int64 `json:"active_traces"`
	CompletedTraces   int64 `json:"completed_traces"`
	FailedTraces      int64 `json:"failed_traces"`
	Spans24h          int64 `json:"spans_24h"`
	Metrics24h        int64 `json:"metrics_24h"`
	AIInvocations24h  int64 `json:"ai_invocations_24h"`
	ToolExecutions24h int64 `json:"tool_executions_24h"`
	FinanceEvents24h  int64 `json:"finance_events_24h"`
}

type VerificationSummary struct {
	Reports        int64   `json:"reports"`
	AverageScore   float64 `json:"average_score"`
	PendingReviews int64   `json:"pending_reviews"`
}

type GovernanceSummary struct {
	Permissions        int64 `json:"permissions"`
	ActivePrinciples   int64 `json:"active_principles"`
	ControlRules       int64 `json:"control_rules"`
	ActiveControlRules int64 `json:"active_control_rules"`
}

type EvolutionSummary struct {
	WeightedActors        int64            `json:"weighted_actors"`
	ExperimentsByStatus   map[string]int64 `json:"experiments_by_status"`
	KnowledgeEntries      int64            `json:"knowledge_entries"`
	UnacknowledgedSignals int64            `json:"unacknowledged_signals"`
	HighPrioritySignals   int64            `json:"high_priority_signals"`
}

type RecentEvent struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Status    string    `json:"status,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
