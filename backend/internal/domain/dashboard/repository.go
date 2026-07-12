package dashboard

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Identity(ctx context.Context) (IdentitySummary, error) {
	var s IdentitySummary
	if orgID := currentTenantOrganizationID(ctx); orgID != nil {
		if err := r.db.QueryRow(ctx, `
			SELECT
				(SELECT COUNT(DISTINCT user_id) FROM organization_memberships WHERE organization_id = $1 AND user_id IS NOT NULL AND status = 'active'),
				(SELECT COUNT(DISTINCT a.id)
				 FROM ai_agents a
				 JOIN organization_memberships om ON om.agent_id = a.id
				 WHERE om.organization_id = $1 AND om.member_type = 'agent' AND om.status = 'active' AND a.is_active),
				(SELECT COUNT(DISTINCT a.id)
				 FROM ai_agents a
				 JOIN organization_memberships om ON om.agent_id = a.id
				 WHERE om.organization_id = $1 AND om.member_type = 'agent' AND om.status = 'active'),
				(SELECT COUNT(*) FROM roles)
		`, *orgID).Scan(&s.Users, &s.ActiveAgents, &s.TotalAgents, &s.Roles); err != nil {
			return s, fmt.Errorf("query scoped identity summary: %w", err)
		}
		return s, nil
	}
	if err := r.db.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM users),
			(SELECT COUNT(*) FROM ai_agents WHERE is_active),
			(SELECT COUNT(*) FROM ai_agents),
			(SELECT COUNT(*) FROM roles)
	`).Scan(&s.Users, &s.ActiveAgents, &s.TotalAgents, &s.Roles); err != nil {
		return s, fmt.Errorf("query identity summary: %w", err)
	}
	return s, nil
}

func (r *Repository) Organization(ctx context.Context) (OrganizationSummary, error) {
	var s OrganizationSummary
	if orgID := currentTenantOrganizationID(ctx); orgID != nil {
		if err := r.db.QueryRow(ctx, `
			SELECT
				(SELECT COUNT(*) FROM organizations WHERE id = $1),
				(SELECT COUNT(*) FROM muvrs WHERE organization_id = $1),
				(SELECT COUNT(*)
				 FROM mvru_members mm
				 JOIN muvrs m ON m.id = mm.mvru_id
				 WHERE m.organization_id = $1),
				(SELECT COUNT(*)
				 FROM mvru_relationships rel
				 JOIN muvrs source ON source.id = rel.source_mvru_id
				 JOIN muvrs target ON target.id = rel.target_mvru_id
				 WHERE source.organization_id = $1 AND target.organization_id = $1)
		`, *orgID).Scan(&s.Organizations, &s.MVRUs, &s.Members, &s.Relationships); err != nil {
			return s, fmt.Errorf("query scoped organization summary: %w", err)
		}

		counts, err := r.countBy(ctx, `SELECT status::text, COUNT(*) FROM muvrs WHERE organization_id = $1 GROUP BY status`, *orgID)
		if err != nil {
			return s, fmt.Errorf("query scoped mvru status counts: %w", err)
		}
		s.MVRUsByStatus = withKnownKeys(counts, "designing", "active", "evaluating", "evolving", "dissolved")
		return s, nil
	}
	if err := r.db.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM organizations),
			(SELECT COUNT(*) FROM muvrs),
			(SELECT COUNT(*) FROM mvru_members),
			(SELECT COUNT(*) FROM mvru_relationships)
	`).Scan(&s.Organizations, &s.MVRUs, &s.Members, &s.Relationships); err != nil {
		return s, fmt.Errorf("query organization summary: %w", err)
	}

	counts, err := r.countBy(ctx, `SELECT status::text, COUNT(*) FROM muvrs GROUP BY status`)
	if err != nil {
		return s, fmt.Errorf("query mvru status counts: %w", err)
	}
	s.MVRUsByStatus = withKnownKeys(counts, "designing", "active", "evaluating", "evolving", "dissolved")
	return s, nil
}

func (r *Repository) legacyWorkflow(ctx context.Context) (WorkflowSummary, error) {
	var s WorkflowSummary
	if orgID := currentTenantOrganizationID(ctx); orgID != nil {
		if err := r.db.QueryRow(ctx, `
			SELECT
				(SELECT COUNT(*) FROM workflow_templates WHERE organization_id = $1),
				(SELECT COUNT(*) FROM workflow_templates WHERE organization_id = $1 AND is_active),
				(SELECT COUNT(*) FROM workflow_instances WHERE organization_id = $1),
				(SELECT COUNT(*)
				 FROM decisions d
				 JOIN tasks t ON t.id = d.task_id
				 JOIN workflow_instances wi ON wi.id = t.workflow_id
				 WHERE wi.organization_id = $1 AND d.created_at >= NOW() - INTERVAL '7 days')
		`, *orgID).Scan(&s.Templates, &s.ActiveTemplates, &s.Instances, &s.Decisions7d); err != nil {
			return s, fmt.Errorf("query scoped workflow summary: %w", err)
		}

		instanceCounts, err := r.countBy(ctx, `SELECT status::text, COUNT(*) FROM workflow_instances WHERE organization_id = $1 GROUP BY status`, *orgID)
		if err != nil {
			return s, fmt.Errorf("query scoped workflow status counts: %w", err)
		}
		s.InstancesByStatus = withKnownKeys(instanceCounts, "active", "paused", "completed", "failed")

		taskCounts, err := r.countBy(ctx, `
			SELECT t.status::text, COUNT(*)
			FROM tasks t
			JOIN workflow_instances wi ON wi.id = t.workflow_id
			WHERE wi.organization_id = $1
			GROUP BY t.status
		`, *orgID)
		if err != nil {
			return s, fmt.Errorf("query scoped task status counts: %w", err)
		}
		s.TasksByStatus = withKnownKeys(taskCounts, "pending", "assigned", "in_progress", "completed", "rejected")

		return s, nil
	}
	if err := r.db.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM workflow_templates),
			(SELECT COUNT(*) FROM workflow_templates WHERE is_active),
			(SELECT COUNT(*) FROM workflow_instances),
			(SELECT COUNT(*) FROM decisions WHERE created_at >= NOW() - INTERVAL '7 days')
	`).Scan(&s.Templates, &s.ActiveTemplates, &s.Instances, &s.Decisions7d); err != nil {
		return s, fmt.Errorf("query workflow summary: %w", err)
	}

	instanceCounts, err := r.countBy(ctx, `SELECT status::text, COUNT(*) FROM workflow_instances GROUP BY status`)
	if err != nil {
		return s, fmt.Errorf("query workflow status counts: %w", err)
	}
	s.InstancesByStatus = withKnownKeys(instanceCounts, "active", "paused", "completed", "failed")

	taskCounts, err := r.countBy(ctx, `SELECT status::text, COUNT(*) FROM tasks GROUP BY status`)
	if err != nil {
		return s, fmt.Errorf("query task status counts: %w", err)
	}
	s.TasksByStatus = withKnownKeys(taskCounts, "pending", "assigned", "in_progress", "completed", "rejected")

	return s, nil
}

func (r *Repository) Capability(ctx context.Context) (CapabilitySummary, error) {
	var s CapabilitySummary
	if orgID := currentTenantOrganizationID(ctx); orgID != nil {
		if err := r.db.QueryRow(ctx, `
			SELECT
				(SELECT COUNT(DISTINCT c.id)
				 FROM capabilities c
				 JOIN capability_bindings b ON b.capability_id = c.id
				 JOIN muvrs m ON m.id = b.mvru_id
				 WHERE m.organization_id = $1),
				(SELECT COUNT(DISTINCT c.id)
				 FROM capabilities c
				 JOIN capability_bindings b ON b.capability_id = c.id
				 JOIN muvrs m ON m.id = b.mvru_id
				 WHERE m.organization_id = $1 AND c.is_active),
				(SELECT COUNT(*)
				 FROM capability_bindings b
				 JOIN muvrs m ON m.id = b.mvru_id
				 WHERE m.organization_id = $1),
				(SELECT COUNT(*)
				 FROM capability_invocations ci
				 JOIN traces tr ON tr.id = ci.trace_id
				 JOIN workflow_instances wi ON wi.id = tr.workflow_id
				 WHERE wi.organization_id = $1 AND ci.created_at >= NOW() - INTERVAL '24 hours'),
				(SELECT COUNT(*)
				 FROM capability_invocations ci
				 JOIN traces tr ON tr.id = ci.trace_id
				 JOIN workflow_instances wi ON wi.id = tr.workflow_id
				 WHERE wi.organization_id = $1 AND ci.created_at >= NOW() - INTERVAL '24 hours' AND ci.outcome IN ('failed', 'error', 'rejected')),
				(SELECT COALESCE(AVG(ci.duration_ms), 0)::float8
				 FROM capability_invocations ci
				 JOIN traces tr ON tr.id = ci.trace_id
				 JOIN workflow_instances wi ON wi.id = tr.workflow_id
				 WHERE wi.organization_id = $1 AND ci.created_at >= NOW() - INTERVAL '24 hours'),
				(SELECT COALESCE(SUM(ci.cost), 0)::float8
				 FROM capability_invocations ci
				 JOIN traces tr ON tr.id = ci.trace_id
				 JOIN workflow_instances wi ON wi.id = tr.workflow_id
				 WHERE wi.organization_id = $1 AND ci.created_at >= NOW() - INTERVAL '24 hours')
		`, *orgID).Scan(
			&s.Capabilities,
			&s.ActiveCapabilities,
			&s.Bindings,
			&s.Invocations24h,
			&s.FailedInvocations24h,
			&s.AverageDurationMs,
			&s.Cost24h,
		); err != nil {
			return s, fmt.Errorf("query scoped capability summary: %w", err)
		}
		return s, nil
	}
	if err := r.db.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM capabilities),
			(SELECT COUNT(*) FROM capabilities WHERE is_active),
			(SELECT COUNT(*) FROM capability_bindings),
			(SELECT COUNT(*) FROM capability_invocations WHERE created_at >= NOW() - INTERVAL '24 hours'),
			(SELECT COUNT(*) FROM capability_invocations WHERE created_at >= NOW() - INTERVAL '24 hours' AND outcome IN ('failed', 'error', 'rejected')),
			(SELECT COALESCE(AVG(duration_ms), 0)::float8 FROM capability_invocations WHERE created_at >= NOW() - INTERVAL '24 hours'),
			(SELECT COALESCE(SUM(cost), 0)::float8 FROM capability_invocations WHERE created_at >= NOW() - INTERVAL '24 hours')
	`).Scan(
		&s.Capabilities,
		&s.ActiveCapabilities,
		&s.Bindings,
		&s.Invocations24h,
		&s.FailedInvocations24h,
		&s.AverageDurationMs,
		&s.Cost24h,
	); err != nil {
		return s, fmt.Errorf("query capability summary: %w", err)
	}
	return s, nil
}

func (r *Repository) Observability(ctx context.Context) (ObservabilitySummary, error) {
	var s ObservabilitySummary
	if orgID := currentTenantOrganizationID(ctx); orgID != nil {
		if err := r.db.QueryRow(ctx, scopedObservabilityQuery(), *orgID).Scan(
			&s.ActiveTraces,
			&s.CompletedTraces,
			&s.FailedTraces,
			&s.Spans24h,
			&s.Metrics24h,
			&s.AIInvocations24h,
			&s.ToolExecutions24h,
			&s.FinanceEvents24h,
		); err != nil {
			return s, fmt.Errorf("query scoped observability summary: %w", err)
		}
		return s, nil
	}
	if err := r.db.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM traces WHERE status = 'active'),
			(SELECT COUNT(*) FROM traces WHERE status = 'completed'),
			(SELECT COUNT(*) FROM traces WHERE status = 'failed'),
			(SELECT COUNT(*) FROM spans WHERE started_at >= NOW() - INTERVAL '24 hours'),
			(SELECT COUNT(*) FROM metrics WHERE recorded_at >= NOW() - INTERVAL '24 hours'),
			(SELECT COUNT(*) FROM ai_invocations WHERE created_at >= NOW() - INTERVAL '24 hours'),
			(SELECT COUNT(*) FROM tool_executions WHERE created_at >= NOW() - INTERVAL '24 hours'),
			(SELECT COUNT(*) FROM finance_export_batches WHERE updated_at >= NOW() - INTERVAL '24 hours')
				+ (SELECT COUNT(*) FROM finance_webhook_events WHERE created_at >= NOW() - INTERVAL '24 hours')
	`).Scan(
		&s.ActiveTraces,
		&s.CompletedTraces,
		&s.FailedTraces,
		&s.Spans24h,
		&s.Metrics24h,
		&s.AIInvocations24h,
		&s.ToolExecutions24h,
		&s.FinanceEvents24h,
	); err != nil {
		return s, fmt.Errorf("query observability summary: %w", err)
	}
	return s, nil
}

func scopedObservabilityQuery() string {
	return `
		SELECT
			(SELECT COUNT(*) FROM traces tr JOIN workflow_instances wi ON wi.id = tr.workflow_id WHERE wi.organization_id = $1 AND tr.status = 'active'),
			(SELECT COUNT(*) FROM traces tr JOIN workflow_instances wi ON wi.id = tr.workflow_id WHERE wi.organization_id = $1 AND tr.status = 'completed'),
			(SELECT COUNT(*) FROM traces tr JOIN workflow_instances wi ON wi.id = tr.workflow_id WHERE wi.organization_id = $1 AND tr.status = 'failed'),
			(SELECT COUNT(*)
			 FROM spans sp
			 JOIN traces tr ON tr.id = sp.trace_id
			 JOIN workflow_instances wi ON wi.id = tr.workflow_id
			 WHERE wi.organization_id = $1 AND sp.started_at >= NOW() - INTERVAL '24 hours'),
			(SELECT COUNT(*) FROM metrics WHERE metadata->>'organization_id' = $1::text AND recorded_at >= NOW() - INTERVAL '24 hours'),
			(SELECT COUNT(*) FROM ai_invocations WHERE organization_id = $1 AND created_at >= NOW() - INTERVAL '24 hours'),
			(SELECT COUNT(*) FROM tool_executions WHERE organization_id = $1 AND created_at >= NOW() - INTERVAL '24 hours'),
			(SELECT COUNT(*)
			 FROM finance_export_batches b
			 WHERE b.updated_at >= NOW() - INTERVAL '24 hours'
			   AND EXISTS (SELECT 1 FROM finance_export_lines l WHERE l.batch_id = b.id AND l.organization_id = $1))
	`
}

func (r *Repository) Verification(ctx context.Context) (VerificationSummary, error) {
	var s VerificationSummary
	if orgID := currentTenantOrganizationID(ctx); orgID != nil {
		if err := r.db.QueryRow(ctx, `
			SELECT
				(SELECT COUNT(*)
				 FROM verification_reports vr
				 JOIN workflow_instances wi ON wi.id = vr.workflow_id
				 WHERE wi.organization_id = $1),
				(SELECT COALESCE(AVG(vr.overall_score), 0)::float8
				 FROM verification_reports vr
				 JOIN workflow_instances wi ON wi.id = vr.workflow_id
				 WHERE wi.organization_id = $1),
				(SELECT COUNT(*)
				 FROM review_assignments ra
				 JOIN verification_reports vr ON vr.id = ra.report_id
				 JOIN workflow_instances wi ON wi.id = vr.workflow_id
				 WHERE wi.organization_id = $1 AND ra.status = 'pending')
		`, *orgID).Scan(&s.Reports, &s.AverageScore, &s.PendingReviews); err != nil {
			return s, fmt.Errorf("query scoped verification summary: %w", err)
		}
		return s, nil
	}
	if err := r.db.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM verification_reports),
			(SELECT COALESCE(AVG(overall_score), 0)::float8 FROM verification_reports),
			(SELECT COUNT(*) FROM review_assignments WHERE status = 'pending')
	`).Scan(&s.Reports, &s.AverageScore, &s.PendingReviews); err != nil {
		return s, fmt.Errorf("query verification summary: %w", err)
	}
	return s, nil
}

func (r *Repository) Governance(ctx context.Context) (GovernanceSummary, error) {
	var s GovernanceSummary
	if err := r.db.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM permissions),
			(SELECT COUNT(*) FROM principles WHERE is_active),
			(SELECT COUNT(*) FROM control_rules),
			(SELECT COUNT(*) FROM control_rules WHERE is_active)
	`).Scan(&s.Permissions, &s.ActivePrinciples, &s.ControlRules, &s.ActiveControlRules); err != nil {
		return s, fmt.Errorf("query governance summary: %w", err)
	}
	return s, nil
}

func (r *Repository) Evolution(ctx context.Context) (EvolutionSummary, error) {
	var s EvolutionSummary
	if orgID := currentTenantOrganizationID(ctx); orgID != nil {
		if err := r.db.QueryRow(ctx, `
			SELECT
				(SELECT COUNT(*) FROM weight_scores),
				(SELECT COUNT(*)
				 FROM knowledge_entries ke
				 JOIN workflow_instances wi ON wi.id = ke.workflow_id
				 WHERE wi.organization_id = $1),
				(SELECT COUNT(*) FROM signals WHERE NOT acknowledged AND data->>'organization_id' = $1::text),
				(SELECT COUNT(*) FROM signals WHERE NOT acknowledged AND priority >= 7 AND data->>'organization_id' = $1::text)
		`, *orgID).Scan(&s.WeightedActors, &s.KnowledgeEntries, &s.UnacknowledgedSignals, &s.HighPrioritySignals); err != nil {
			return s, fmt.Errorf("query scoped evolution summary: %w", err)
		}

		counts, err := r.countBy(ctx, `
			SELECT e.status, COUNT(*)
			FROM experiments e
			JOIN muvrs m ON m.id = e.mvru_id
			WHERE m.organization_id = $1
			GROUP BY e.status
		`, *orgID)
		if err != nil {
			return s, fmt.Errorf("query scoped experiment status counts: %w", err)
		}
		s.ExperimentsByStatus = withKnownKeys(counts, "proposed", "running", "completed", "failed")
		return s, nil
	}
	if err := r.db.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM weight_scores),
			(SELECT COUNT(*) FROM knowledge_entries),
			(SELECT COUNT(*) FROM signals WHERE NOT acknowledged),
			(SELECT COUNT(*) FROM signals WHERE NOT acknowledged AND priority >= 7)
	`).Scan(&s.WeightedActors, &s.KnowledgeEntries, &s.UnacknowledgedSignals, &s.HighPrioritySignals); err != nil {
		return s, fmt.Errorf("query evolution summary: %w", err)
	}

	counts, err := r.countBy(ctx, `SELECT status, COUNT(*) FROM experiments GROUP BY status`)
	if err != nil {
		return s, fmt.Errorf("query experiment status counts: %w", err)
	}
	s.ExperimentsByStatus = withKnownKeys(counts, "proposed", "running", "completed", "failed")
	return s, nil
}

func (r *Repository) legacyRecentEvents(ctx context.Context, limit int) ([]RecentEvent, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	orgID := nullableUUID(currentTenantOrganizationID(ctx))
	rows, err := r.db.Query(ctx, recentEventsQuery(), limit, orgID)
	if err != nil {
		return nil, fmt.Errorf("query recent events: %w", err)
	}
	defer rows.Close()

	events := make([]RecentEvent, 0, limit)
	for rows.Next() {
		var event RecentEvent
		if err := rows.Scan(&event.ID, &event.Type, &event.Title, &event.Status, &event.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan recent event: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent events: %w", err)
	}
	return events, nil
}

func recentEventsQuery() string {
	return `
		SELECT id, type, title, status, created_at
		FROM (
			SELECT id::text, 'workflow' AS type, 'Workflow instance' AS title, status::text, created_at
			FROM workflow_instances
			WHERE $2::uuid IS NULL OR organization_id = $2
			UNION ALL
			SELECT id::text, 'signal' AS type, signal_type AS title, CASE WHEN acknowledged THEN 'acknowledged' ELSE 'open' END AS status, created_at
			FROM signals
			WHERE $2::uuid IS NULL OR data->>'organization_id' = $2::text
			UNION ALL
			SELECT id::text, 'verification' AS type, 'Verification report' AS title, 'reported' AS status, created_at
			FROM verification_reports vr
			WHERE $2::uuid IS NULL OR EXISTS (
				SELECT 1 FROM workflow_instances wi WHERE wi.id = vr.workflow_id AND wi.organization_id = $2
			)
			UNION ALL
			SELECT e.id::text, 'experiment' AS type, e.name AS title, e.status, e.created_at
			FROM experiments e
			LEFT JOIN muvrs m ON m.id = e.mvru_id
			WHERE $2::uuid IS NULL OR m.organization_id = $2
			UNION ALL
			SELECT tr.id::text, 'trace' AS type, 'Execution trace' AS title, tr.status, tr.started_at AS created_at
			FROM traces tr
			WHERE $2::uuid IS NULL OR EXISTS (
				SELECT 1 FROM workflow_instances wi WHERE wi.id = tr.workflow_id AND wi.organization_id = $2
			)
			UNION ALL
			SELECT id::text, 'ai_invocation' AS type, COALESCE(NULLIF(source_surface, ''), 'AI') || ' model call' AS title, status, created_at
			FROM ai_invocations
			WHERE $2::uuid IS NULL OR organization_id = $2
			UNION ALL
			SELECT id::text, 'tool_execution' AS type, 'Tool execution' AS title, status, created_at
			FROM tool_executions
			WHERE $2::uuid IS NULL OR organization_id = $2
			UNION ALL
			SELECT b.id::text, 'finance_export' AS type, 'Finance export batch' AS title, b.status, b.updated_at AS created_at
			FROM finance_export_batches b
			WHERE $2::uuid IS NULL
			   OR EXISTS (SELECT 1 FROM finance_export_lines l WHERE l.batch_id = b.id AND l.organization_id = $2)
			UNION ALL
			SELECT id::text, 'finance_webhook' AS type, event_type AS title, CASE WHEN processed THEN 'processed' ELSE 'failed' END AS status, created_at
			FROM finance_webhook_events
			WHERE $2::uuid IS NULL
		) events
		ORDER BY created_at DESC
		LIMIT $1
	`
}

func (r *Repository) countBy(ctx context.Context, query string, args ...any) (map[string]int64, error) {
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
