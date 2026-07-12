package metaorg

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

func (r *PostgresRepository) projectedHealth(ctx context.Context, orgID *uuid.UUID) (HealthSummary, error) {
	var summary HealthSummary
	if err := r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(open_requirements), 0), COALESCE(SUM(active_projects), 0)
		FROM platform.tenant_operational_projections
		WHERE $1::uuid IS NULL OR organization_id = $1
	`, nullableUUID(orgID)).Scan(&summary.OpenRequirements, &summary.ActiveProjects); err != nil {
		return summary, fmt.Errorf("query projected meta-org health: %w", err)
	}
	if orgID != nil {
		if err := r.db.QueryRow(ctx, `
			SELECT COUNT(DISTINCT agent.id)
			FROM ai_agents agent
			JOIN organization_memberships membership ON membership.agent_id = agent.id
			WHERE membership.organization_id = $1 AND membership.member_type = 'agent'
			  AND membership.status = 'active' AND agent.is_active
		`, *orgID).Scan(&summary.ActiveAgents); err != nil {
			return summary, fmt.Errorf("query scoped active agents: %w", err)
		}
	} else if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM ai_agents WHERE is_active`).Scan(&summary.ActiveAgents); err != nil {
		return summary, fmt.Errorf("query active agents: %w", err)
	}
	summary.Currency = "CNY"
	if r.tableExists(ctx, "tool_approvals") {
		if err := r.db.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM tool_approvals approval
			JOIN tool_executions execution ON execution.id = approval.execution_id
			WHERE approval.status = 'pending' AND ($1::uuid IS NULL OR execution.organization_id = $1)
		`, nullableUUID(orgID)).Scan(&summary.PendingApprovals); err != nil {
			return summary, fmt.Errorf("query projected pending approvals: %w", err)
		}
	}
	if r.tableExists(ctx, "ai_usage_ledger") {
		if err := r.db.QueryRow(ctx, `
			SELECT COALESCE(SUM(ledger.amount), 0)::float8
			FROM ai_usage_ledger ledger
			JOIN ai_invocations invocation ON invocation.id = ledger.invocation_id
			WHERE ledger.finance_export_line_id IS NULL
			  AND ($1::uuid IS NULL OR invocation.organization_id = $1)
		`, nullableUUID(orgID)).Scan(&summary.UnexportedCost); err != nil {
			return summary, fmt.Errorf("query projected unexported AI cost: %w", err)
		}
	}
	return summary, nil
}

func (r *PostgresRepository) projectedProjectSummary(ctx context.Context, orgID *uuid.UUID) (ProjectSummary, error) {
	counts, err := r.countBy(ctx, `
		SELECT entry.key, SUM(entry.value::bigint)
		FROM platform.tenant_operational_projections projection
		CROSS JOIN LATERAL jsonb_each_text(projection.projects_by_status) entry
		WHERE $1::uuid IS NULL OR projection.organization_id = $1
		GROUP BY entry.key
	`, nullableUUID(orgID))
	if err != nil {
		return ProjectSummary{}, fmt.Errorf("query projected project status counts: %w", err)
	}
	var overBudget int64
	if err := r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(over_budget_projects), 0)
		FROM platform.tenant_operational_projections
		WHERE $1::uuid IS NULL OR organization_id = $1
	`, nullableUUID(orgID)).Scan(&overBudget); err != nil {
		return ProjectSummary{}, fmt.Errorf("query projected over-budget projects: %w", err)
	}
	return ProjectSummary{
		ByStatus:   withKnownKeys(counts, "planning", "active", "paused", "delivering", "completed", "closed", "cancelled"),
		OverBudget: overBudget,
	}, nil
}

func (r *PostgresRepository) projectedCostSummary(ctx context.Context, orgID *uuid.UUID) (CostSummary, error) {
	summary := CostSummary{Currency: "CNY", ByProvider: map[string]float64{}}
	if err := r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(project_cost_today), 0)::float8,
		       COALESCE(SUM(project_cost_month_to_date), 0)::float8,
		       COALESCE((array_agg(project_cost_currency ORDER BY projected_at DESC))[1], 'CNY')
		FROM platform.tenant_operational_projections
		WHERE $1::uuid IS NULL OR organization_id = $1
	`, nullableUUID(orgID)).Scan(&summary.Today, &summary.MonthToDate, &summary.Currency); err != nil {
		return summary, fmt.Errorf("query projected project cost summary: %w", err)
	}
	if !r.tableExists(ctx, "ai_usage_ledger") {
		return summary, nil
	}
	if err := r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(ledger.amount), 0)::float8
		FROM ai_usage_ledger ledger
		JOIN ai_invocations invocation ON invocation.id = ledger.invocation_id
		WHERE ledger.finance_export_line_id IS NULL
		  AND ($1::uuid IS NULL OR invocation.organization_id = $1)
	`, nullableUUID(orgID)).Scan(&summary.Unexported); err != nil {
		return summary, fmt.Errorf("query projected unexported usage: %w", err)
	}
	rows, err := r.db.Query(ctx, `
		SELECT provider.provider_type, COALESCE(SUM(ledger.amount), 0)::float8
		FROM ai_usage_ledger ledger
		JOIN ai_invocations invocation ON invocation.id = ledger.invocation_id
		JOIN model_providers provider ON provider.id = invocation.provider_id
		WHERE $1::uuid IS NULL OR invocation.organization_id = $1
		GROUP BY provider.provider_type
	`, nullableUUID(orgID))
	if err != nil {
		return summary, fmt.Errorf("query projected provider usage: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var provider string
		var amount float64
		if err := rows.Scan(&provider, &amount); err != nil {
			return summary, fmt.Errorf("scan projected provider usage: %w", err)
		}
		summary.ByProvider[provider] = amount
	}
	if err := rows.Err(); err != nil {
		return summary, fmt.Errorf("iterate projected provider usage: %w", err)
	}
	return summary, nil
}

func (r *PostgresRepository) projectedActivity(ctx context.Context, orgID *uuid.UUID, limit int) ([]ActivityItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, type, title, status, created_at
		FROM (
			SELECT item_id AS id, item_type AS type, title, status, occurred_at AS created_at
			FROM platform.tenant_activity_projections
			WHERE $2::uuid IS NULL OR organization_id = $2
			UNION ALL
			SELECT id::text, 'signal', signal_type, CASE WHEN acknowledged THEN 'acknowledged' ELSE 'open' END, created_at
			FROM signals WHERE $2::uuid IS NULL OR data->>'organization_id' = $2::text
			UNION ALL
			SELECT id::text, 'ai_invocation', COALESCE(NULLIF(source_surface, ''), 'AI') || ' model call', status, created_at
			FROM ai_invocations WHERE $2::uuid IS NULL OR organization_id = $2
			UNION ALL
			SELECT id::text, 'tool_execution', 'Tool execution', status, created_at
			FROM tool_executions WHERE $2::uuid IS NULL OR organization_id = $2
		) activity
		ORDER BY created_at DESC
		LIMIT $1
	`, limit, nullableUUID(orgID))
	if err != nil {
		return nil, fmt.Errorf("query projected meta-org activity: %w", err)
	}
	defer rows.Close()
	items := make([]ActivityItem, 0, limit)
	for rows.Next() {
		var item ActivityItem
		if err := rows.Scan(&item.ID, &item.Type, &item.Title, &item.Status, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan projected meta-org activity: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate projected meta-org activity: %w", err)
	}
	return items, nil
}

func (r *PostgresRepository) projectionFreshness(ctx context.Context, orgID *uuid.UUID) (ProjectionFreshness, error) {
	var freshness ProjectionFreshness
	if err := r.db.QueryRow(ctx, `
		SELECT COALESCE(MAX(source_occurred_at), 'epoch'::timestamptz),
		       COALESCE(MAX(projected_at), 'epoch'::timestamptz),
		       COALESCE(MAX(projection_lag_ms), 0),
		       COALESCE(MAX(snapshot_version), 0),
		       COUNT(*)
		FROM platform.tenant_operational_projections
		WHERE $1::uuid IS NULL OR organization_id = $1
	`, nullableUUID(orgID)).Scan(
		&freshness.SourceOccurredAt, &freshness.ProjectedAt, &freshness.LagMilliseconds,
		&freshness.SnapshotVersion, &freshness.TenantCount,
	); err != nil {
		return freshness, fmt.Errorf("query tenant projection freshness: %w", err)
	}
	return freshness, nil
}
