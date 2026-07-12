package metaorg

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
)

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Overview(ctx context.Context) (Overview, error) {
	orgID := currentTenantOrganizationID(ctx)
	health, err := r.projectedHealth(ctx, orgID)
	if err != nil {
		return Overview{}, err
	}
	projects, err := r.projectedProjectSummary(ctx, orgID)
	if err != nil {
		return Overview{}, err
	}
	agents, err := r.agentSummary(ctx, orgID)
	if err != nil {
		return Overview{}, err
	}
	cost, err := r.projectedCostSummary(ctx, orgID)
	if err != nil {
		return Overview{}, err
	}
	risks, err := r.risks(ctx, orgID, 10)
	if err != nil {
		return Overview{}, err
	}
	activity, err := r.projectedActivity(ctx, orgID, 10)
	if err != nil {
		return Overview{}, err
	}
	freshness, err := r.projectionFreshness(ctx, orgID)
	if err != nil {
		return Overview{}, err
	}
	return Overview{
		GeneratedAt: time.Now().UTC(),
		Health:      health,
		Projects:    projects,
		Agents:      agents,
		Cost:        cost,
		Risks:       risks,
		Activity:    activity,
		Projection:  freshness,
	}, nil
}

func (r *PostgresRepository) Inbox(ctx context.Context, filter InboxFilter) ([]InboxItem, error) {
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 50
	}

	orgID := currentTenantOrganizationID(ctx)
	items := make([]InboxItem, 0, filter.Limit)
	if r.tableExists(ctx, "tool_approvals") {
		toolItems, err := r.toolApprovalInbox(ctx, orgID, filter.Limit)
		if err != nil {
			return nil, err
		}
		items = append(items, toolItems...)
	}
	signalItems, err := r.signalInbox(ctx, orgID, filter.Limit)
	if err != nil {
		return nil, err
	}
	items = append(items, signalItems...)

	reviewItems, err := r.reviewInbox(ctx, orgID, filter.Limit)
	if err != nil {
		return nil, err
	}
	items = append(items, reviewItems...)

	decisionItems, err := r.accessDecisionInbox(ctx, orgID, filter.Limit)
	if err != nil {
		return nil, err
	}
	items = append(items, decisionItems...)

	if r.tableExists(ctx, "finance_export_batches") {
		financeItems, err := r.financeInbox(ctx, orgID, filter.Limit)
		if err != nil {
			return nil, err
		}
		items = append(items, financeItems...)
	}

	items = filterInboxItems(items, filter.Type)
	sortInboxItems(items)
	if len(items) > filter.Limit {
		items = items[:filter.Limit]
	}
	return items, nil
}

func (r *PostgresRepository) health(ctx context.Context, orgID *uuid.UUID) (HealthSummary, error) {
	var summary HealthSummary
	if orgID != nil {
		if err := r.db.QueryRow(ctx, `
			SELECT
				(SELECT COUNT(*) FROM requirements WHERE organization_id = $1 AND status IN ('draft', 'analyzed', 'approved')),
				(SELECT COUNT(*) FROM projects WHERE organization_id = $1 AND status IN ('planning', 'active', 'paused', 'delivering')),
				(
					SELECT COUNT(DISTINCT a.id)
					FROM ai_agents a
					JOIN organization_memberships om ON om.agent_id = a.id
					WHERE om.organization_id = $1
					  AND om.member_type = 'agent'
					  AND om.status = 'active'
					  AND a.is_active
				)
		`, *orgID).Scan(&summary.OpenRequirements, &summary.ActiveProjects, &summary.ActiveAgents); err != nil {
			return summary, fmt.Errorf("query scoped meta-org health: %w", err)
		}
		summary.Currency = "CNY"
		if r.tableExists(ctx, "tool_approvals") {
			if err := r.db.QueryRow(ctx, `
				SELECT COUNT(*)
				FROM tool_approvals a
				JOIN tool_executions e ON e.id = a.execution_id
				WHERE a.status = 'pending' AND e.organization_id = $1
			`, *orgID).Scan(&summary.PendingApprovals); err != nil {
				return summary, fmt.Errorf("query scoped pending tool approvals: %w", err)
			}
		}
		if r.tableExists(ctx, "ai_usage_ledger") {
			if err := r.db.QueryRow(ctx, `
				SELECT COALESCE(SUM(l.amount), 0)::float8
				FROM ai_usage_ledger l
				JOIN ai_invocations i ON i.id = l.invocation_id
				WHERE l.finance_export_line_id IS NULL AND i.organization_id = $1
			`, *orgID).Scan(&summary.UnexportedCost); err != nil {
				return summary, fmt.Errorf("query scoped unexported ai cost: %w", err)
			}
		}
		return summary, nil
	}
	if err := r.db.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM requirements WHERE status IN ('draft', 'analyzed', 'approved')),
			(SELECT COUNT(*) FROM projects WHERE status IN ('planning', 'active', 'paused', 'delivering')),
			(SELECT COUNT(*) FROM ai_agents WHERE is_active)
	`).Scan(&summary.OpenRequirements, &summary.ActiveProjects, &summary.ActiveAgents); err != nil {
		return summary, fmt.Errorf("query meta-org health: %w", err)
	}
	summary.Currency = "CNY"
	summary.PendingApprovals = 0
	if r.tableExists(ctx, "tool_approvals") {
		if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM tool_approvals WHERE status = 'pending'`).Scan(&summary.PendingApprovals); err != nil {
			return summary, fmt.Errorf("query pending tool approvals: %w", err)
		}
	}
	if r.tableExists(ctx, "ai_usage_ledger") {
		if err := r.db.QueryRow(ctx, `
			SELECT COALESCE(SUM(amount), 0)::float8
			FROM ai_usage_ledger
			WHERE finance_export_line_id IS NULL
		`).Scan(&summary.UnexportedCost); err != nil {
			return summary, fmt.Errorf("query unexported ai cost: %w", err)
		}
	}
	return summary, nil
}

func (r *PostgresRepository) projectSummary(ctx context.Context, orgID *uuid.UUID) (ProjectSummary, error) {
	if orgID != nil {
		counts, err := r.countBy(ctx, `SELECT status::text, COUNT(*) FROM projects WHERE organization_id = $1 GROUP BY status`, *orgID)
		if err != nil {
			return ProjectSummary{}, fmt.Errorf("query scoped project status counts: %w", err)
		}
		var overBudget int64
		if err := r.db.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM projects p
			WHERE p.organization_id = $1
			  AND p.budget_amount > 0
			  AND COALESCE((SELECT SUM(amount) FROM project_cost_entries c WHERE c.project_id = p.id), 0) > p.budget_amount
		`, *orgID).Scan(&overBudget); err != nil {
			return ProjectSummary{}, fmt.Errorf("query scoped over-budget projects: %w", err)
		}
		return ProjectSummary{
			ByStatus:   withKnownKeys(counts, "planning", "active", "paused", "delivering", "completed", "closed", "cancelled"),
			OverBudget: overBudget,
		}, nil
	}
	counts, err := r.countBy(ctx, `SELECT status::text, COUNT(*) FROM projects GROUP BY status`)
	if err != nil {
		return ProjectSummary{}, fmt.Errorf("query project status counts: %w", err)
	}
	var overBudget int64
	if err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM projects p
		WHERE p.budget_amount > 0
		  AND COALESCE((SELECT SUM(amount) FROM project_cost_entries c WHERE c.project_id = p.id), 0) > p.budget_amount
	`).Scan(&overBudget); err != nil {
		return ProjectSummary{}, fmt.Errorf("query over-budget projects: %w", err)
	}
	return ProjectSummary{
		ByStatus:   withKnownKeys(counts, "planning", "active", "paused", "delivering", "completed", "closed", "cancelled"),
		OverBudget: overBudget,
	}, nil
}

func (r *PostgresRepository) agentSummary(ctx context.Context, orgID *uuid.UUID) (AgentSummary, error) {
	var summary AgentSummary
	if orgID != nil {
		if err := r.db.QueryRow(ctx, `
			SELECT
				(SELECT COUNT(DISTINCT a.id)
				 FROM ai_agents a
				 JOIN organization_memberships om ON om.agent_id = a.id
				 WHERE om.organization_id = $1 AND om.member_type = 'agent' AND om.status = 'active'),
				(SELECT COUNT(DISTINCT a.id)
				 FROM ai_agents a
				 JOIN organization_memberships om ON om.agent_id = a.id
				 WHERE om.organization_id = $1 AND om.member_type = 'agent' AND om.status = 'active' AND a.is_active)
		`, *orgID).Scan(&summary.Total, &summary.Active); err != nil {
			return summary, fmt.Errorf("query scoped agent summary: %w", err)
		}
		counts, err := r.countBy(ctx, `
			SELECT a.risk_level::text, COUNT(DISTINCT a.id)
			FROM ai_agents a
			JOIN organization_memberships om ON om.agent_id = a.id
			WHERE om.organization_id = $1 AND om.member_type = 'agent' AND om.status = 'active'
			GROUP BY a.risk_level
		`, *orgID)
		if err != nil {
			return summary, fmt.Errorf("query scoped agent risk counts: %w", err)
		}
		summary.ByRiskLevel = withKnownKeys(counts, "low", "medium", "high", "critical")
		return summary, nil
	}
	if err := r.db.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM ai_agents),
			(SELECT COUNT(*) FROM ai_agents WHERE is_active)
	`).Scan(&summary.Total, &summary.Active); err != nil {
		return summary, fmt.Errorf("query agent summary: %w", err)
	}
	counts, err := r.countBy(ctx, `SELECT risk_level::text, COUNT(*) FROM ai_agents GROUP BY risk_level`)
	if err != nil {
		return summary, fmt.Errorf("query agent risk counts: %w", err)
	}
	summary.ByRiskLevel = withKnownKeys(counts, "low", "medium", "high", "critical")
	return summary, nil
}

func (r *PostgresRepository) costSummary(ctx context.Context, orgID *uuid.UUID) (CostSummary, error) {
	summary := CostSummary{
		Currency:   "CNY",
		ByProvider: map[string]float64{},
	}
	if orgID != nil {
		if err := r.db.QueryRow(ctx, `
			SELECT
				COALESCE(SUM(c.amount) FILTER (WHERE c.occurred_at >= date_trunc('day', NOW())), 0)::float8,
				COALESCE(SUM(c.amount) FILTER (WHERE c.occurred_at >= date_trunc('month', NOW())), 0)::float8
			FROM project_cost_entries c
			JOIN projects p ON p.id = c.project_id
			WHERE p.organization_id = $1
		`, *orgID).Scan(&summary.Today, &summary.MonthToDate); err != nil {
			return summary, fmt.Errorf("query scoped project cost summary: %w", err)
		}
		if r.tableExists(ctx, "ai_usage_ledger") {
			if err := r.db.QueryRow(ctx, `
				SELECT COALESCE(SUM(l.amount), 0)::float8
				FROM ai_usage_ledger l
				JOIN ai_invocations i ON i.id = l.invocation_id
				WHERE l.finance_export_line_id IS NULL AND i.organization_id = $1
			`, *orgID).Scan(&summary.Unexported); err != nil {
				return summary, fmt.Errorf("query scoped unexported usage summary: %w", err)
			}
			rows, err := r.db.Query(ctx, `
				SELECT mp.provider_type, COALESCE(SUM(l.amount), 0)::float8
				FROM ai_usage_ledger l
				JOIN ai_invocations i ON i.id = l.invocation_id
				JOIN model_providers mp ON mp.id = i.provider_id
				WHERE i.organization_id = $1
				GROUP BY mp.provider_type
			`, *orgID)
			if err != nil {
				return summary, fmt.Errorf("query scoped provider usage summary: %w", err)
			}
			defer rows.Close()
			for rows.Next() {
				var provider string
				var amount float64
				if err := rows.Scan(&provider, &amount); err != nil {
					return summary, fmt.Errorf("scan scoped provider usage summary: %w", err)
				}
				summary.ByProvider[provider] = amount
			}
			if err := rows.Err(); err != nil {
				return summary, fmt.Errorf("iterate scoped provider usage summary: %w", err)
			}
		}
		return summary, nil
	}
	if err := r.db.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(amount) FILTER (WHERE occurred_at >= date_trunc('day', NOW())), 0)::float8,
			COALESCE(SUM(amount) FILTER (WHERE occurred_at >= date_trunc('month', NOW())), 0)::float8
		FROM project_cost_entries
	`).Scan(&summary.Today, &summary.MonthToDate); err != nil {
		return summary, fmt.Errorf("query project cost summary: %w", err)
	}
	if r.tableExists(ctx, "ai_usage_ledger") {
		if err := r.db.QueryRow(ctx, `
			SELECT COALESCE(SUM(amount), 0)::float8
			FROM ai_usage_ledger
			WHERE finance_export_line_id IS NULL
		`).Scan(&summary.Unexported); err != nil {
			return summary, fmt.Errorf("query unexported usage summary: %w", err)
		}
		rows, err := r.db.Query(ctx, `
			SELECT mp.provider_type, COALESCE(SUM(l.amount), 0)::float8
			FROM ai_usage_ledger l
			JOIN ai_invocations i ON i.id = l.invocation_id
			JOIN model_providers mp ON mp.id = i.provider_id
			GROUP BY mp.provider_type
		`)
		if err != nil {
			return summary, fmt.Errorf("query provider usage summary: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var provider string
			var amount float64
			if err := rows.Scan(&provider, &amount); err != nil {
				return summary, fmt.Errorf("scan provider usage summary: %w", err)
			}
			summary.ByProvider[provider] = amount
		}
		if err := rows.Err(); err != nil {
			return summary, fmt.Errorf("iterate provider usage summary: %w", err)
		}
	}
	return summary, nil
}

func (r *PostgresRepository) risks(ctx context.Context, orgID *uuid.UUID, limit int) ([]RiskItem, error) {
	query := `
		SELECT id, title, severity, source
		FROM (
			SELECT id::text, action || ' ' || resource AS title, risk_level AS severity, 'governance' AS source, created_at
			FROM access_decisions
			WHERE ((risk_level IN ('high', 'critical') OR NOT allowed) AND ($2::uuid IS NULL OR organization_id = $2))
			UNION ALL
			SELECT id::text, signal_type AS title, CASE WHEN priority >= 9 THEN 'critical' ELSE 'high' END AS severity, 'evolution' AS source, created_at
			FROM signals
			WHERE NOT acknowledged AND priority >= 7 AND ($2::uuid IS NULL OR data->>'organization_id' = $2::text)
		) risks
		ORDER BY
			CASE severity WHEN 'critical' THEN 0 WHEN 'high' THEN 1 WHEN 'medium' THEN 2 ELSE 3 END,
			created_at DESC
		LIMIT $1
	`
	rows, err := r.db.Query(ctx, query, limit, nullableUUID(orgID))
	if err != nil {
		return nil, fmt.Errorf("query risks: %w", err)
	}
	defer rows.Close()

	risks := []RiskItem{}
	for rows.Next() {
		var item RiskItem
		if err := rows.Scan(&item.ID, &item.Title, &item.Severity, &item.Source); err != nil {
			return nil, fmt.Errorf("scan risk: %w", err)
		}
		risks = append(risks, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate risks: %w", err)
	}
	return risks, nil
}

func (r *PostgresRepository) activity(ctx context.Context, orgID *uuid.UUID, limit int) ([]ActivityItem, error) {
	query := activityQuery()
	rows, err := r.db.Query(ctx, query, limit, nullableUUID(orgID))
	if err != nil {
		return nil, fmt.Errorf("query activity: %w", err)
	}
	defer rows.Close()

	activity := []ActivityItem{}
	for rows.Next() {
		var item ActivityItem
		if err := rows.Scan(&item.ID, &item.Type, &item.Title, &item.Status, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan activity: %w", err)
		}
		activity = append(activity, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate activity: %w", err)
	}
	return activity, nil
}

func activityQuery() string {
	return `
		SELECT id, type, title, status, created_at
		FROM (
			SELECT id::text, 'requirement' AS type, title, status::text, created_at FROM requirements WHERE $2::uuid IS NULL OR organization_id = $2
			UNION ALL
			SELECT id::text, 'project' AS type, name AS title, status::text, created_at FROM projects WHERE $2::uuid IS NULL OR organization_id = $2
			UNION ALL
			SELECT id::text, 'workflow' AS type, 'Workflow instance' AS title, status::text, created_at FROM workflow_instances WHERE $2::uuid IS NULL OR organization_id = $2
			UNION ALL
			SELECT id::text, 'signal' AS type, signal_type AS title, CASE WHEN acknowledged THEN 'acknowledged' ELSE 'open' END AS status, created_at FROM signals WHERE $2::uuid IS NULL OR data->>'organization_id' = $2::text
			UNION ALL
			SELECT id::text, 'ai_invocation' AS type, COALESCE(NULLIF(source_surface, ''), 'AI') || ' model call' AS title, status::text, created_at FROM ai_invocations WHERE $2::uuid IS NULL OR organization_id = $2
			UNION ALL
			SELECT id::text, 'tool_execution' AS type, 'Tool execution' AS title, status::text, created_at FROM tool_executions WHERE $2::uuid IS NULL OR organization_id = $2
			UNION ALL
			SELECT b.id::text, 'finance_export' AS type, 'Finance export batch' AS title, b.status::text, b.updated_at AS created_at
			FROM finance_export_batches b
			WHERE $2::uuid IS NULL OR EXISTS (SELECT 1 FROM finance_export_lines l WHERE l.batch_id = b.id AND l.organization_id = $2)
			UNION ALL
			SELECT id::text, 'finance_webhook' AS type, event_type AS title, CASE WHEN processed THEN 'processed' ELSE 'failed' END AS status, created_at FROM finance_webhook_events WHERE $2::uuid IS NULL
		) events
		ORDER BY created_at DESC
		LIMIT $1
	`
}

func (r *PostgresRepository) signalInbox(ctx context.Context, orgID *uuid.UUID, limit int) ([]InboxItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, 'evolution_signal', signal_type, 'pending', CASE WHEN priority >= 9 THEN 'critical' WHEN priority >= 7 THEN 'high' ELSE 'medium' END, 'evolution', created_at
		FROM signals
		WHERE NOT acknowledged AND priority >= 7 AND ($2::uuid IS NULL OR data->>'organization_id' = $2::text)
		ORDER BY priority DESC, created_at DESC
		LIMIT $1
	`, limit, nullableUUID(orgID))
	if err != nil {
		return nil, fmt.Errorf("query signal inbox: %w", err)
	}
	return scanInboxRows(rows)
}

func (r *PostgresRepository) reviewInbox(ctx context.Context, orgID *uuid.UUID, limit int) ([]InboxItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, 'verification_review', 'Verification review', status, 'medium', 'verification', created_at
		FROM (
			SELECT ra.id, ra.status, ra.created_at, wi.organization_id
			FROM review_assignments ra
			JOIN verification_reports vr ON vr.id = ra.report_id
			LEFT JOIN workflow_instances wi ON wi.id = vr.workflow_id
		) reviews
		WHERE status = 'pending' AND ($2::uuid IS NULL OR organization_id = $2)
		ORDER BY created_at DESC
		LIMIT $1
	`, limit, nullableUUID(orgID))
	if err != nil {
		return nil, fmt.Errorf("query review inbox: %w", err)
	}
	return scanInboxRows(rows)
}

func (r *PostgresRepository) accessDecisionInbox(ctx context.Context, orgID *uuid.UUID, limit int) ([]InboxItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, 'governance_decision', action || ' ' || resource, decision, CASE WHEN risk_level = 'critical' THEN 'critical' ELSE 'high' END, 'governance', created_at
		FROM access_decisions
		WHERE risk_level IN ('high', 'critical') AND (NOT allowed OR decision = 'deny') AND ($2::uuid IS NULL OR organization_id = $2)
		ORDER BY created_at DESC
		LIMIT $1
	`, limit, nullableUUID(orgID))
	if err != nil {
		return nil, fmt.Errorf("query access decision inbox: %w", err)
	}
	return scanInboxRows(rows)
}

func (r *PostgresRepository) toolApprovalInbox(ctx context.Context, orgID *uuid.UUID, limit int) ([]InboxItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT a.id::text, 'tool_approval', 'Tool approval required', a.status, 'high', 'toolruntime', a.created_at
		FROM tool_approvals a
		JOIN tool_executions e ON e.id = a.execution_id
		WHERE a.status = 'pending' AND ($2::uuid IS NULL OR e.organization_id = $2)
		ORDER BY a.created_at DESC
		LIMIT $1
	`, limit, nullableUUID(orgID))
	if err != nil {
		return nil, fmt.Errorf("query tool approval inbox: %w", err)
	}
	return scanInboxRows(rows)
}

func (r *PostgresRepository) financeInbox(ctx context.Context, orgID *uuid.UUID, limit int) ([]InboxItem, error) {
	rows, err := r.db.Query(ctx, financeInboxQuery(), limit, nullableUUID(orgID))
	if err != nil {
		return nil, fmt.Errorf("query finance inbox: %w", err)
	}
	return scanInboxRows(rows)
}

func financeInboxQuery() string {
	return `
		SELECT b.id::text, 'finance_export', 'Finance export failed', b.status, 'high', 'finance', b.updated_at
		FROM finance_export_batches b
		WHERE b.status = 'failed'
		  AND ($2::uuid IS NULL OR EXISTS (SELECT 1 FROM finance_export_lines l WHERE l.batch_id = b.id AND l.organization_id = $2))
		ORDER BY b.updated_at DESC
		LIMIT $1
	`
}

func (r *PostgresRepository) tableExists(ctx context.Context, tableName string) bool {
	var exists bool
	if err := r.db.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, tableName).Scan(&exists); err != nil {
		return false
	}
	return exists
}

func (r *PostgresRepository) countBy(ctx context.Context, query string, args ...any) (map[string]int64, error) {
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := map[string]int64{}
	for rows.Next() {
		var key string
		var count int64
		if err := rows.Scan(&key, &count); err != nil {
			return nil, err
		}
		counts[key] = count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return counts, nil
}

func withKnownKeys(counts map[string]int64, keys ...string) map[string]int64 {
	result := make(map[string]int64, len(keys)+len(counts))
	for _, key := range keys {
		result[key] = counts[key]
	}
	for key, count := range counts {
		result[key] = count
	}
	return result
}

func currentTenantOrganizationID(ctx context.Context) *uuid.UUID {
	tenant, ok := middleware.TenantFromContext(ctx)
	if !ok || tenant.OrganizationID == nil {
		return nil
	}
	id := *tenant.OrganizationID
	return &id
}

func nullableUUID(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return *id
}

func scanInboxRows(rows pgx.Rows) ([]InboxItem, error) {
	defer rows.Close()

	items := []InboxItem{}
	for rows.Next() {
		var item InboxItem
		if err := rows.Scan(&item.ID, &item.Type, &item.Title, &item.Status, &item.Priority, &item.Source, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan inbox item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate inbox items: %w", err)
	}
	return items, nil
}

func filterInboxItems(items []InboxItem, itemType string) []InboxItem {
	if itemType == "" {
		return items
	}
	filtered := make([]InboxItem, 0, len(items))
	for _, item := range items {
		if item.Type == itemType {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func sortInboxItems(items []InboxItem) {
	sort.SliceStable(items, func(i, j int) bool {
		leftRank := inboxPriorityRank(items[i].Priority)
		rightRank := inboxPriorityRank(items[j].Priority)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
}

func inboxPriorityRank(priority string) int {
	switch priority {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	case "low":
		return 3
	default:
		return 4
	}
}
