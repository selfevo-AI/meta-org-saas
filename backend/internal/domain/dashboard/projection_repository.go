package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
)

func (r *Repository) Workflow(ctx context.Context) (WorkflowSummary, error) {
	orgID := currentTenantOrganizationID(ctx)
	var summary WorkflowSummary
	if orgID != nil {
		var instancesJSON, tasksJSON []byte
		if err := r.db.QueryRow(ctx, `
			SELECT workflow_templates, active_workflow_templates, workflow_instances,
			       workflow_instances_by_status, workflow_tasks_by_status, workflow_decisions_7d,
			       source_occurred_at, projected_at, projection_lag_ms, snapshot_version
			FROM platform.tenant_operational_projections
			WHERE organization_id = $1
		`, *orgID).Scan(
			&summary.Templates, &summary.ActiveTemplates, &summary.Instances,
			&instancesJSON, &tasksJSON, &summary.Decisions7d,
			&summary.SourceOccurredAt, &summary.ProjectedAt, &summary.ProjectionLagMs, &summary.SnapshotVersion,
		); err != nil {
			return summary, fmt.Errorf("query scoped workflow projection: %w", err)
		}
		if err := json.Unmarshal(instancesJSON, &summary.InstancesByStatus); err != nil {
			return summary, fmt.Errorf("decode scoped workflow status projection: %w", err)
		}
		if err := json.Unmarshal(tasksJSON, &summary.TasksByStatus); err != nil {
			return summary, fmt.Errorf("decode scoped task status projection: %w", err)
		}
	} else {
		if err := r.db.QueryRow(ctx, `
			SELECT COALESCE(SUM(workflow_templates), 0), COALESCE(SUM(active_workflow_templates), 0),
			       COALESCE(SUM(workflow_instances), 0), COALESCE(SUM(workflow_decisions_7d), 0),
			       COALESCE(MAX(source_occurred_at), 'epoch'::timestamptz),
			       COALESCE(MAX(projected_at), 'epoch'::timestamptz),
			       COALESCE(MAX(projection_lag_ms), 0), COALESCE(MAX(snapshot_version), 0)
			FROM platform.tenant_operational_projections
		`).Scan(
			&summary.Templates, &summary.ActiveTemplates, &summary.Instances, &summary.Decisions7d,
			&summary.SourceOccurredAt, &summary.ProjectedAt, &summary.ProjectionLagMs, &summary.SnapshotVersion,
		); err != nil {
			return summary, fmt.Errorf("query workflow projections: %w", err)
		}
		instanceCounts, err := r.projectionJSONCounts(ctx, "workflow_instances_by_status")
		if err != nil {
			return summary, err
		}
		taskCounts, err := r.projectionJSONCounts(ctx, "workflow_tasks_by_status")
		if err != nil {
			return summary, err
		}
		summary.InstancesByStatus = instanceCounts
		summary.TasksByStatus = taskCounts
	}
	summary.InstancesByStatus = withKnownKeys(summary.InstancesByStatus, "active", "paused", "completed", "failed")
	summary.TasksByStatus = withKnownKeys(summary.TasksByStatus, "pending", "assigned", "in_progress", "completed", "rejected")
	return summary, nil
}

func (r *Repository) projectionJSONCounts(ctx context.Context, column string) (map[string]int64, error) {
	if column != "workflow_instances_by_status" && column != "workflow_tasks_by_status" {
		return nil, fmt.Errorf("unsupported workflow projection column %q", column)
	}
	query := fmt.Sprintf(`
		SELECT entry.key, SUM(entry.value::bigint)
		FROM platform.tenant_operational_projections projection
		CROSS JOIN LATERAL jsonb_each_text(projection.%s) entry
		GROUP BY entry.key
	`, column)
	counts, err := r.countBy(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query %s projection counts: %w", column, err)
	}
	return counts, nil
}

func (r *Repository) RecentEvents(ctx context.Context, limit int) ([]RecentEvent, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	orgID := nullableUUID(currentTenantOrganizationID(ctx))
	rows, err := r.db.Query(ctx, projectedRecentEventsQuery(), limit, orgID)
	if err != nil {
		return nil, fmt.Errorf("query projected recent events: %w", err)
	}
	defer rows.Close()

	events := make([]RecentEvent, 0, limit)
	for rows.Next() {
		var event RecentEvent
		if err := rows.Scan(&event.ID, &event.Type, &event.Title, &event.Status, &event.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan projected recent event: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate projected recent events: %w", err)
	}
	return events, nil
}

func projectedRecentEventsQuery() string {
	return `
		SELECT id, type, title, status, created_at
		FROM (
			SELECT item_id AS id, item_type AS type, title, status, occurred_at AS created_at
			FROM platform.tenant_activity_projections
			WHERE $2::uuid IS NULL OR organization_id = $2
			UNION ALL
			SELECT id::text, 'signal', signal_type, CASE WHEN acknowledged THEN 'acknowledged' ELSE 'open' END, created_at
			FROM signals WHERE $2::uuid IS NULL OR data->>'organization_id' = $2::text
			UNION ALL
			SELECT tr.id::text, 'trace', 'Execution trace', tr.status, tr.started_at
			FROM traces tr
			WHERE $2::uuid IS NULL OR EXISTS (
				SELECT 1 FROM platform.tenant_workflow_projections workflow
				WHERE workflow.workflow_id = tr.workflow_id AND workflow.organization_id = $2
			)
			UNION ALL
			SELECT id::text, 'ai_invocation', COALESCE(NULLIF(source_surface, ''), 'AI') || ' model call', status, created_at
			FROM ai_invocations WHERE $2::uuid IS NULL OR organization_id = $2
			UNION ALL
			SELECT id::text, 'tool_execution', 'Tool execution', status, created_at
			FROM tool_executions WHERE $2::uuid IS NULL OR organization_id = $2
		) events
		ORDER BY created_at DESC
		LIMIT $1
	`
}
