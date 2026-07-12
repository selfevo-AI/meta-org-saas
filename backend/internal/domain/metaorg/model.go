package metaorg

import "time"

type Overview struct {
	GeneratedAt time.Time           `json:"generated_at"`
	Health      HealthSummary       `json:"health"`
	Projects    ProjectSummary      `json:"projects"`
	Agents      AgentSummary        `json:"agents"`
	Cost        CostSummary         `json:"cost"`
	Risks       []RiskItem          `json:"risks"`
	Activity    []ActivityItem      `json:"activity"`
	Projection  ProjectionFreshness `json:"projection"`
}

type ProjectionFreshness struct {
	SourceOccurredAt time.Time `json:"source_occurred_at"`
	ProjectedAt      time.Time `json:"projected_at"`
	LagMilliseconds  int64     `json:"lag_ms"`
	SnapshotVersion  int64     `json:"snapshot_version"`
	TenantCount      int64     `json:"tenant_count"`
}

type HealthSummary struct {
	OpenRequirements int64   `json:"open_requirements"`
	ActiveProjects   int64   `json:"active_projects"`
	ActiveAgents     int64   `json:"active_agents"`
	PendingApprovals int64   `json:"pending_approvals"`
	UnexportedCost   float64 `json:"unexported_cost"`
	Currency         string  `json:"currency"`
}

type ProjectSummary struct {
	ByStatus   map[string]int64 `json:"by_status"`
	OverBudget int64            `json:"over_budget"`
}

type AgentSummary struct {
	Total       int64            `json:"total"`
	Active      int64            `json:"active"`
	ByRiskLevel map[string]int64 `json:"by_risk_level"`
}

type CostSummary struct {
	Today       float64            `json:"today"`
	MonthToDate float64            `json:"month_to_date"`
	Unexported  float64            `json:"unexported"`
	Currency    string             `json:"currency"`
	ByProvider  map[string]float64 `json:"by_provider"`
}

type RiskItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Severity string `json:"severity"`
	Source   string `json:"source"`
}

type ActivityItem struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Status    string    `json:"status,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type InboxFilter struct {
	Limit int
	Type  string
}

type InboxItem struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	Priority  string    `json:"priority"`
	Source    string    `json:"source,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
