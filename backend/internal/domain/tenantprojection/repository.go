package tenantprojection

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/tenantdb"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListProvisionedTargets(ctx context.Context, limit int) ([]tenantdb.Target, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := r.db.Query(ctx, `
		SELECT organization_id, deployment_mode, cluster_key, region, database_name,
		       schema_name, connection_secret_ref, migration_version, status, metadata
		FROM platform.tenant_database_targets
		WHERE deployment_mode = 'dedicated_database' AND status = 'provisioned'
		ORDER BY updated_at, organization_id
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list projection tenant targets: %w", err)
	}
	defer rows.Close()

	targets := make([]tenantdb.Target, 0, limit)
	for rows.Next() {
		var target tenantdb.Target
		var metadata []byte
		if err := rows.Scan(
			&target.OrganizationID,
			&target.DeploymentMode,
			&target.ClusterKey,
			&target.Region,
			&target.DatabaseName,
			&target.SchemaName,
			&target.ConnectionSecretRef,
			&target.MigrationVersion,
			&target.Status,
			&metadata,
		); err != nil {
			return nil, fmt.Errorf("scan projection tenant target: %w", err)
		}
		target.Metadata = map[string]any{}
		if err := json.Unmarshal(metadata, &target.Metadata); err != nil {
			return nil, fmt.Errorf("decode projection tenant target metadata: %w", err)
		}
		targets = append(targets, target)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate projection tenant targets: %w", err)
	}
	return targets, nil
}

func (r *Repository) ApplyProjection(ctx context.Context, events []OutboxEvent, snapshot OperationalSnapshot, activities []ActivityItem, workflows []WorkflowEntity, projectedAt time.Time) (ProjectionFreshness, error) {
	if len(events) == 0 {
		return ProjectionFreshness{}, fmt.Errorf("projection requires at least one event")
	}
	latest := events[0]
	for _, event := range events[1:] {
		if event.OccurredAt.After(latest.OccurredAt) {
			latest = event
		}
		if event.OrganizationID != snapshot.OrganizationID {
			return ProjectionFreshness{}, fmt.Errorf("projection batch mixes organizations %s and %s", snapshot.OrganizationID, event.OrganizationID)
		}
	}
	projectedAt = projectedAt.UTC()
	lag := projectedAt.Sub(latest.OccurredAt)
	if lag < 0 {
		lag = 0
	}
	projectJSON, err := json.Marshal(snapshot.ProjectsByStatus)
	if err != nil {
		return ProjectionFreshness{}, fmt.Errorf("encode project status projection: %w", err)
	}
	workflowJSON, err := json.Marshal(snapshot.WorkflowInstancesByStatus)
	if err != nil {
		return ProjectionFreshness{}, fmt.Errorf("encode workflow status projection: %w", err)
	}
	taskJSON, err := json.Marshal(snapshot.WorkflowTasksByStatus)
	if err != nil {
		return ProjectionFreshness{}, fmt.Errorf("encode task status projection: %w", err)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return ProjectionFreshness{}, fmt.Errorf("begin tenant projection: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, event := range events {
		if _, err := tx.Exec(ctx, `
			INSERT INTO platform.tenant_integration_inbox(
			    event_id, event_type, event_version, organization_id, actor_id, actor_type,
			    authority_tier, aggregate_type, aggregate_id, aggregate_version,
			    trace_id, causation_id, correlation_id, occurred_at, schema_version,
			    payload, metadata, status
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16::jsonb,$17::jsonb,'received')
			ON CONFLICT (event_id) DO UPDATE SET
			    metadata = platform.tenant_integration_inbox.metadata || EXCLUDED.metadata,
			    updated_at = NOW()
		`, event.EventID, event.EventType, event.EventVersion, event.OrganizationID, event.ActorID, event.ActorType,
			event.AuthorityTier, event.AggregateType, event.AggregateID, event.AggregateVersion,
			event.TraceID, event.CausationID, event.CorrelationID, event.OccurredAt, event.SchemaVersion,
			event.Payload, event.Metadata); err != nil {
			return ProjectionFreshness{}, fmt.Errorf("upsert tenant integration inbox event %s: %w", event.EventID, err)
		}
	}

	var snapshotVersion int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO platform.tenant_operational_projections(
		    organization_id, open_requirements, active_projects, projects_by_status,
		    over_budget_projects, project_cost_today, project_cost_month_to_date,
		    project_cost_currency, workflow_templates, active_workflow_templates,
		    workflow_instances, workflow_instances_by_status, workflow_tasks_by_status,
		    workflow_decisions_7d, source_event_id, source_occurred_at, projected_at,
		    projection_lag_ms, snapshot_version, metadata
		)
		VALUES ($1,$2,$3,$4::jsonb,$5,$6,$7,$8,$9,$10,$11,$12::jsonb,$13::jsonb,$14,$15,$16,$17,$18,1,
		        jsonb_build_object('projection_contract','meta-org.tenant-operational.v1'))
		ON CONFLICT (organization_id) DO UPDATE SET
		    open_requirements = EXCLUDED.open_requirements,
		    active_projects = EXCLUDED.active_projects,
		    projects_by_status = EXCLUDED.projects_by_status,
		    over_budget_projects = EXCLUDED.over_budget_projects,
		    project_cost_today = EXCLUDED.project_cost_today,
		    project_cost_month_to_date = EXCLUDED.project_cost_month_to_date,
		    project_cost_currency = EXCLUDED.project_cost_currency,
		    workflow_templates = EXCLUDED.workflow_templates,
		    active_workflow_templates = EXCLUDED.active_workflow_templates,
		    workflow_instances = EXCLUDED.workflow_instances,
		    workflow_instances_by_status = EXCLUDED.workflow_instances_by_status,
		    workflow_tasks_by_status = EXCLUDED.workflow_tasks_by_status,
		    workflow_decisions_7d = EXCLUDED.workflow_decisions_7d,
		    source_event_id = EXCLUDED.source_event_id,
		    source_occurred_at = EXCLUDED.source_occurred_at,
		    projected_at = EXCLUDED.projected_at,
		    projection_lag_ms = EXCLUDED.projection_lag_ms,
		    snapshot_version = platform.tenant_operational_projections.snapshot_version + 1,
		    metadata = platform.tenant_operational_projections.metadata || EXCLUDED.metadata
		RETURNING snapshot_version
	`, snapshot.OrganizationID, snapshot.OpenRequirements, snapshot.ActiveProjects, projectJSON,
		snapshot.OverBudgetProjects, snapshot.ProjectCostToday, snapshot.ProjectCostMonthToDate,
		snapshot.ProjectCostCurrency, snapshot.WorkflowTemplates, snapshot.ActiveWorkflowTemplates,
		snapshot.WorkflowInstances, workflowJSON, taskJSON, snapshot.WorkflowDecisions7d,
		latest.EventID, latest.OccurredAt, projectedAt, lag.Milliseconds()).Scan(&snapshotVersion); err != nil {
		return ProjectionFreshness{}, fmt.Errorf("upsert tenant operational projection: %w", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM platform.tenant_workflow_projections WHERE organization_id = $1`, snapshot.OrganizationID); err != nil {
		return ProjectionFreshness{}, fmt.Errorf("replace tenant workflow projections: %w", err)
	}
	for _, workflow := range workflows {
		if workflow.OrganizationID != snapshot.OrganizationID {
			return ProjectionFreshness{}, fmt.Errorf("workflow projection %s belongs to organization %s, expected %s", workflow.WorkflowID, workflow.OrganizationID, snapshot.OrganizationID)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO platform.tenant_workflow_projections(
			    organization_id, workflow_id, project_id, status, current_stage,
			    created_at, updated_at, source_event_id, projected_at
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		`, workflow.OrganizationID, workflow.WorkflowID, workflow.ProjectID, workflow.Status,
			workflow.CurrentStage, workflow.CreatedAt, workflow.UpdatedAt, latest.EventID, projectedAt); err != nil {
			return ProjectionFreshness{}, fmt.Errorf("insert tenant workflow projection %s: %w", workflow.WorkflowID, err)
		}
	}

	if _, err := tx.Exec(ctx, `DELETE FROM platform.tenant_activity_projections WHERE organization_id = $1`, snapshot.OrganizationID); err != nil {
		return ProjectionFreshness{}, fmt.Errorf("replace tenant activity projection: %w", err)
	}
	for _, activity := range activities {
		if _, err := tx.Exec(ctx, `
			INSERT INTO platform.tenant_activity_projections(
			    organization_id, item_type, item_id, title, status, occurred_at,
			    source_event_id, projected_at, metadata
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,
			        jsonb_build_object('projection_contract','meta-org.tenant-activity.v1'))
		`, activity.OrganizationID, activity.ItemType, activity.ItemID, activity.Title,
			activity.Status, activity.OccurredAt, latest.EventID, projectedAt); err != nil {
			return ProjectionFreshness{}, fmt.Errorf("insert tenant activity projection: %w", err)
		}
	}

	eventIDs := make([]uuid.UUID, 0, len(events))
	for _, event := range events {
		eventIDs = append(eventIDs, event.EventID)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE platform.tenant_integration_inbox
		SET status = 'projected', projected_at = $2, last_error = '', updated_at = NOW()
		WHERE event_id = ANY($1)
	`, eventIDs, projectedAt); err != nil {
		return ProjectionFreshness{}, fmt.Errorf("complete tenant integration inbox: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return ProjectionFreshness{}, fmt.Errorf("commit tenant projection: %w", err)
	}
	return ProjectionFreshness{
		SourceEventID:    latest.EventID,
		SourceOccurredAt: latest.OccurredAt,
		ProjectedAt:      projectedAt,
		LagMilliseconds:  lag.Milliseconds(),
		SnapshotVersion:  snapshotVersion,
	}, nil
}

func ClaimEvents(ctx context.Context, db tenantdb.DB, workerID string, lease time.Duration, batchSize int, maxAttempts int) ([]OutboxEvent, error) {
	if batchSize <= 0 || batchSize > 1000 {
		batchSize = 100
	}
	if maxAttempts <= 0 {
		maxAttempts = 20
	}
	if lease <= 0 {
		lease = time.Minute
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tenant outbox claim: %w", err)
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `
		WITH candidates AS (
		    SELECT event_id
		    FROM tenant_integration_outbox
		    WHERE attempt_count < $4
		      AND available_at <= NOW()
		      AND (
		        status IN ('pending', 'failed')
		        OR (status = 'running' AND lease_expires_at <= NOW())
		      )
		    ORDER BY occurred_at, event_id
		    FOR UPDATE SKIP LOCKED
		    LIMIT $3
		)
		UPDATE tenant_integration_outbox AS event
		SET status = 'running', attempt_count = event.attempt_count + 1,
		    lease_owner = $1, lease_expires_at = NOW() + make_interval(secs => $2),
		    last_error = '', updated_at = NOW()
		FROM candidates
		WHERE event.event_id = candidates.event_id
		RETURNING event.event_id, event.event_type, event.event_version, event.organization_id,
		          event.actor_id, event.actor_type, event.authority_tier, event.aggregate_type,
		          event.aggregate_id, event.aggregate_version, event.trace_id, event.causation_id,
		          event.correlation_id, event.occurred_at, event.schema_version,
		          event.payload, event.metadata, event.attempt_count
	`, workerID, int64(lease/time.Second), batchSize, maxAttempts)
	if err != nil {
		return nil, fmt.Errorf("claim tenant outbox events: %w", err)
	}
	defer rows.Close()
	events := make([]OutboxEvent, 0, batchSize)
	for rows.Next() {
		var event OutboxEvent
		if err := rows.Scan(
			&event.EventID, &event.EventType, &event.EventVersion, &event.OrganizationID,
			&event.ActorID, &event.ActorType, &event.AuthorityTier, &event.AggregateType,
			&event.AggregateID, &event.AggregateVersion, &event.TraceID, &event.CausationID,
			&event.CorrelationID, &event.OccurredAt, &event.SchemaVersion,
			&event.Payload, &event.Metadata, &event.AttemptCount,
		); err != nil {
			return nil, fmt.Errorf("scan tenant outbox event: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tenant outbox events: %w", err)
	}
	rows.Close()
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tenant outbox claim: %w", err)
	}
	return events, nil
}

func LoadOperationalSnapshot(ctx context.Context, db tenantdb.DB, organizationID uuid.UUID) (OperationalSnapshot, error) {
	var snapshot OperationalSnapshot
	snapshot.OrganizationID = organizationID
	var projectsJSON, workflowJSON, tasksJSON []byte
	err := db.QueryRow(ctx, `
		WITH project_status AS (
		    SELECT status, COUNT(*) AS count FROM projects WHERE organization_id = $1 GROUP BY status
		), workflow_status AS (
		    SELECT status, COUNT(*) AS count FROM workflow_instances WHERE organization_id = $1 GROUP BY status
		), task_status AS (
		    SELECT task.status, COUNT(*) AS count
		    FROM tasks AS task
		    JOIN workflow_instances AS workflow ON workflow.id = task.workflow_id
		    WHERE workflow.organization_id = $1
		    GROUP BY task.status
		)
		SELECT
		    (SELECT COUNT(*) FROM requirements WHERE organization_id = $1 AND status IN ('draft','analyzed','approved')),
		    (SELECT COUNT(*) FROM projects WHERE organization_id = $1 AND status IN ('planning','active','paused','delivering')),
		    COALESCE((SELECT jsonb_object_agg(status, count) FROM project_status), '{}'::jsonb),
		    (SELECT COUNT(*) FROM projects AS project WHERE project.organization_id = $1 AND project.budget_amount > 0
		       AND COALESCE((SELECT SUM(cost.amount) FROM project_cost_entries AS cost WHERE cost.project_id = project.id), 0) > project.budget_amount),
		    COALESCE((SELECT SUM(cost.amount) FROM project_cost_entries AS cost JOIN projects AS project ON project.id = cost.project_id
		              WHERE project.organization_id = $1 AND cost.occurred_at >= date_trunc('day', NOW())), 0)::float8,
		    COALESCE((SELECT SUM(cost.amount) FROM project_cost_entries AS cost JOIN projects AS project ON project.id = cost.project_id
		              WHERE project.organization_id = $1 AND cost.occurred_at >= date_trunc('month', NOW())), 0)::float8,
		    COALESCE((SELECT cost.currency FROM project_cost_entries AS cost JOIN projects AS project ON project.id = cost.project_id
		              WHERE project.organization_id = $1 GROUP BY cost.currency ORDER BY SUM(cost.amount) DESC LIMIT 1), 'CNY'),
		    (SELECT COUNT(*) FROM workflow_templates WHERE organization_id = $1),
		    (SELECT COUNT(*) FROM workflow_templates WHERE organization_id = $1 AND is_active),
		    (SELECT COUNT(*) FROM workflow_instances WHERE organization_id = $1),
		    COALESCE((SELECT jsonb_object_agg(status, count) FROM workflow_status), '{}'::jsonb),
		    COALESCE((SELECT jsonb_object_agg(status, count) FROM task_status), '{}'::jsonb),
		    (SELECT COUNT(*) FROM decisions AS decision
		       JOIN tasks AS task ON task.id = decision.task_id
		       JOIN workflow_instances AS workflow ON workflow.id = task.workflow_id
		     WHERE workflow.organization_id = $1 AND decision.created_at >= NOW() - INTERVAL '7 days')
	`, organizationID).Scan(
		&snapshot.OpenRequirements, &snapshot.ActiveProjects, &projectsJSON,
		&snapshot.OverBudgetProjects, &snapshot.ProjectCostToday, &snapshot.ProjectCostMonthToDate,
		&snapshot.ProjectCostCurrency, &snapshot.WorkflowTemplates, &snapshot.ActiveWorkflowTemplates,
		&snapshot.WorkflowInstances, &workflowJSON, &tasksJSON, &snapshot.WorkflowDecisions7d,
	)
	if err != nil {
		return snapshot, fmt.Errorf("load tenant operational snapshot: %w", err)
	}
	if err := json.Unmarshal(projectsJSON, &snapshot.ProjectsByStatus); err != nil {
		return snapshot, fmt.Errorf("decode project status snapshot: %w", err)
	}
	if err := json.Unmarshal(workflowJSON, &snapshot.WorkflowInstancesByStatus); err != nil {
		return snapshot, fmt.Errorf("decode workflow status snapshot: %w", err)
	}
	if err := json.Unmarshal(tasksJSON, &snapshot.WorkflowTasksByStatus); err != nil {
		return snapshot, fmt.Errorf("decode task status snapshot: %w", err)
	}
	return snapshot, nil
}

func LoadRecentActivity(ctx context.Context, db tenantdb.DB, organizationID uuid.UUID, limit int) ([]ActivityItem, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := db.Query(ctx, `
		SELECT item_type, item_id, title, status, occurred_at
		FROM (
		    SELECT 'requirement' AS item_type, id::text AS item_id, title, status, updated_at AS occurred_at
		    FROM requirements WHERE organization_id = $1
		    UNION ALL
		    SELECT 'project', id::text, name, status, updated_at FROM projects WHERE organization_id = $1
		    UNION ALL
		    SELECT 'workflow', id::text, 'Workflow instance', status, updated_at FROM workflow_instances WHERE organization_id = $1
		) AS activity
		ORDER BY occurred_at DESC, item_type, item_id
		LIMIT $2
	`, organizationID, limit)
	if err != nil {
		return nil, fmt.Errorf("load tenant activity snapshot: %w", err)
	}
	defer rows.Close()
	items := make([]ActivityItem, 0, limit)
	for rows.Next() {
		item := ActivityItem{OrganizationID: organizationID}
		if err := rows.Scan(&item.ItemType, &item.ItemID, &item.Title, &item.Status, &item.OccurredAt); err != nil {
			return nil, fmt.Errorf("scan tenant activity snapshot: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tenant activity snapshot: %w", err)
	}
	return items, nil
}

func LoadWorkflowEntities(ctx context.Context, db tenantdb.DB, organizationID uuid.UUID) ([]WorkflowEntity, error) {
	rows, err := db.Query(ctx, `
		SELECT id, project_id, status, current_stage, created_at, updated_at
		FROM workflow_instances
		WHERE organization_id = $1
		ORDER BY created_at, id
	`, organizationID)
	if err != nil {
		return nil, fmt.Errorf("load tenant workflow entities: %w", err)
	}
	defer rows.Close()

	entities := make([]WorkflowEntity, 0)
	for rows.Next() {
		entity := WorkflowEntity{OrganizationID: organizationID}
		if err := rows.Scan(&entity.WorkflowID, &entity.ProjectID, &entity.Status, &entity.CurrentStage, &entity.CreatedAt, &entity.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan tenant workflow entity: %w", err)
		}
		entities = append(entities, entity)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tenant workflow entities: %w", err)
	}
	return entities, nil
}

func MarkEventsPublished(ctx context.Context, db tenantdb.DB, workerID string, eventIDs []uuid.UUID, publishedAt time.Time) error {
	if len(eventIDs) == 0 {
		return nil
	}
	tag, err := db.Exec(ctx, `
		UPDATE tenant_integration_outbox
		SET status = 'published', published_at = $3, lease_owner = '', lease_expires_at = NULL,
		    last_error = '', updated_at = NOW()
		WHERE event_id = ANY($1) AND status = 'running' AND lease_owner = $2
	`, eventIDs, workerID, publishedAt)
	if err != nil {
		return fmt.Errorf("mark tenant outbox events published: %w", err)
	}
	if tag.RowsAffected() != int64(len(eventIDs)) {
		return fmt.Errorf("mark tenant outbox events published: updated %d of %d leased events", tag.RowsAffected(), len(eventIDs))
	}
	return nil
}

func MarkEventsFailed(ctx context.Context, db tenantdb.DB, workerID string, eventIDs []uuid.UUID, retryAt time.Time, message string) error {
	if len(eventIDs) == 0 {
		return nil
	}
	_, err := db.Exec(ctx, `
		UPDATE tenant_integration_outbox
		SET status = 'failed', available_at = $3, lease_owner = '', lease_expires_at = NULL,
		    last_error = $4, updated_at = NOW()
		WHERE event_id = ANY($1) AND status = 'running' AND lease_owner = $2
	`, eventIDs, workerID, retryAt, message)
	if err != nil {
		return fmt.Errorf("mark tenant outbox events failed: %w", err)
	}
	return nil
}

func IsMissingOutbox(err error) bool {
	var pgErr interface{ SQLState() string }
	return errors.As(err, &pgErr) && pgErr.SQLState() == "42P01"
}
