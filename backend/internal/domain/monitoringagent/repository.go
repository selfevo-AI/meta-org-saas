package monitoringagent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) CreateRun(ctx context.Context, run *MonitoringAgentRun) error {
	if run.ID == uuid.Nil {
		run.ID = uuid.New()
	}
	if run.Summary == nil {
		run.Summary = map[string]any{}
	}
	return r.db.QueryRow(ctx, `
		INSERT INTO monitoring_agent_runs (
			id, trigger_type, organization_id, status, lookback_started_at, lookback_ended_at,
			signals_created, duplicates_suppressed, summary, error_message, started_at, completed_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id
	`, run.ID, run.TriggerType, run.OrganizationID, run.Status, run.LookbackStartedAt, run.LookbackEndedAt,
		run.SignalsCreated, run.DuplicatesSuppressed, mustJSON(run.Summary), run.ErrorMessage, run.StartedAt, run.CompletedAt).Scan(&run.ID)
}

func (r *PostgresRepository) CompleteRun(ctx context.Context, run *MonitoringAgentRun) error {
	if run.Summary == nil {
		run.Summary = map[string]any{}
	}
	_, err := r.db.Exec(ctx, `
		UPDATE monitoring_agent_runs
		SET status = $2,
			signals_created = $3,
			duplicates_suppressed = $4,
			summary = $5,
			error_message = $6,
			completed_at = $7
		WHERE id = $1
	`, run.ID, run.Status, run.SignalsCreated, run.DuplicatesSuppressed, mustJSON(run.Summary), run.ErrorMessage, run.CompletedAt)
	if err != nil {
		return fmt.Errorf("complete monitoring agent run: %w", err)
	}
	return nil
}

func (r *PostgresRepository) ListRuns(ctx context.Context, filter ListRunsFilter) ([]MonitoringAgentRun, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, trigger_type, organization_id, status, lookback_started_at, lookback_ended_at,
			signals_created, duplicates_suppressed, summary, error_message, started_at, completed_at
		FROM monitoring_agent_runs
		WHERE ($1::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $1)
		ORDER BY started_at DESC
		LIMIT $2
	`, nullableUUID(filter.OrganizationID), normalizeLimit(filter.Limit))
	if err != nil {
		return nil, fmt.Errorf("list monitoring agent runs: %w", err)
	}
	defer rows.Close()
	return scanRuns(rows)
}

func (r *PostgresRepository) GetRun(ctx context.Context, id uuid.UUID) (*MonitoringAgentRun, error) {
	run := &MonitoringAgentRun{}
	if err := scanRun(r.db.QueryRow(ctx, `
		SELECT id, trigger_type, organization_id, status, lookback_started_at, lookback_ended_at,
			signals_created, duplicates_suppressed, summary, error_message, started_at, completed_at
		FROM monitoring_agent_runs
		WHERE id = $1
	`, id), run); err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get monitoring agent run: %w", err)
	}
	return run, nil
}

func (r *PostgresRepository) HasOpenSignal(ctx context.Context, fingerprint string) (bool, error) {
	var exists bool
	if err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM signals
			WHERE acknowledged = false
				AND source = 'monitoring_agent'
				AND data->>'fingerprint' = $1
		)
	`, fingerprint).Scan(&exists); err != nil {
		return false, fmt.Errorf("check monitoring signal fingerprint: %w", err)
	}
	return exists, nil
}

func (r *PostgresRepository) CreateSignal(ctx context.Context, signal SignalWrite) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO signals (signal_type, source, priority, data)
		VALUES ($1, $2, $3, $4)
	`, signal.SignalType, signal.Source, signal.Priority, mustJSON(signal.Data))
	if err != nil {
		return fmt.Errorf("create monitoring signal: %w", err)
	}
	return nil
}

func (r *PostgresRepository) CreateContextProposal(ctx context.Context, proposal ContextProposalWrite) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO context_change_proposals (dictionary_version_id, proposal_type, title, summary, payload, status)
		VALUES ($1, $2, $3, $4, $5, COALESCE(NULLIF($6, ''), 'pending'))
	`, proposal.DictionaryVersionID, proposal.ProposalType, proposal.Title, proposal.Summary, mustJSON(proposal.Payload), proposal.Status)
	if err != nil {
		return fmt.Errorf("create monitoring context proposal: %w", err)
	}
	return nil
}

func (r *PostgresRepository) CollectFindings(ctx context.Context, window ScanWindow) ([]OperationalFinding, error) {
	findings := make([]OperationalFinding, 0)
	queries := []string{
		aiFailureQuery,
		aiStreamDisconnectQuery,
		toolFailureQuery,
		toolApprovalBacklogQuery,
		contextRuleGapQuery,
		contextBuildFailureQuery,
		schemaChangeFailureQuery,
		financeExportFailureQuery,
		financeWebhookFailureQuery,
		financeImportFailureQuery,
		erpActionFailureQuery,
		costWithoutProgressQuery,
		contextProposalStaleQuery,
		assistantProposalStaleQuery,
	}
	for _, query := range queries {
		if err := r.appendFindings(ctx, &findings, query, window); err != nil {
			return nil, err
		}
		if window.Limit > 0 && len(findings) >= window.Limit {
			break
		}
	}
	if window.Limit > 0 && len(findings) > window.Limit {
		findings = findings[:window.Limit]
	}
	return findings, nil
}

func (r *PostgresRepository) appendFindings(ctx context.Context, target *[]OperationalFinding, query string, window ScanWindow) error {
	rows, err := r.db.Query(ctx, query, nullableUUID(window.OrganizationID), window.StartedAt, window.EndedAt)
	if err != nil {
		return fmt.Errorf("query monitoring findings: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var finding OperationalFinding
		var orgID pgtype.UUID
		var dataJSON, proposalJSON []byte
		if err := rows.Scan(&finding.Category, &orgID, &finding.EntityType, &finding.EntityID, &finding.Reason,
			&finding.Severity, &finding.OccurredAt, &dataJSON, &proposalJSON); err != nil {
			return fmt.Errorf("scan monitoring finding: %w", err)
		}
		finding.OrganizationID = uuidPointer(orgID)
		finding.Data = unmarshalMap(dataJSON)
		finding.ProposalPayload = unmarshalMap(proposalJSON)
		*target = append(*target, finding)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate monitoring findings: %w", err)
	}
	return nil
}

const commonFindingColumns = `
	category,
	organization_id,
	entity_type,
	entity_id,
	reason,
	severity,
	occurred_at,
	data,
	proposal_payload
`

const aiFailureQuery = `
SELECT ` + commonFindingColumns + `
FROM (
	SELECT '` + SignalAIFailure + `' AS category,
		organization_id,
		'ai_invocation' AS entity_type,
		id::text AS entity_id,
		COALESCE(NULLIF(error_message, ''), status) AS reason,
		CASE WHEN status = 'failed' THEN '` + SeverityHigh + `' ELSE '` + SeverityMedium + `' END AS severity,
		COALESCE(completed_at, created_at) AS occurred_at,
		jsonb_build_object('status', status, 'error_type', error_type, 'requested_model', requested_model, 'cost_amount', cost_amount::text) AS data,
		'{}'::jsonb AS proposal_payload
	FROM ai_invocations
	WHERE status IN ('failed', 'cancelled')
		AND created_at >= $2 AND created_at <= $3
		AND ($1::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $1)
) findings`

const aiStreamDisconnectQuery = `
SELECT ` + commonFindingColumns + `
FROM (
	SELECT '` + SignalAIStreamDisconnect + `' AS category,
		organization_id,
		'ai_invocation' AS entity_type,
		id::text AS entity_id,
		'streaming invocation has no completion record' AS reason,
		'` + SeverityMedium + `' AS severity,
		created_at AS occurred_at,
		jsonb_build_object('status', status, 'requested_model', requested_model, 'duration_minutes', EXTRACT(EPOCH FROM ($3 - created_at)) / 60) AS data,
		'{}'::jsonb AS proposal_payload
	FROM ai_invocations
	WHERE status = 'streaming'
		AND completed_at IS NULL
		AND created_at >= $2 AND created_at <= $3 - INTERVAL '10 minutes'
		AND ($1::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $1)
) findings`

const toolFailureQuery = `
SELECT ` + commonFindingColumns + `
FROM (
	SELECT '` + SignalToolFailure + `' AS category,
		e.organization_id,
		'tool_execution' AS entity_type,
		e.id::text AS entity_id,
		COALESCE(NULLIF(e.error_message, ''), e.status) AS reason,
		CASE WHEN td.risk_level IN ('high', 'critical') THEN '` + SeverityHigh + `' ELSE '` + SeverityMedium + `' END AS severity,
		COALESCE(e.completed_at, e.created_at) AS occurred_at,
		jsonb_build_object('tool_name', td.name, 'status', e.status, 'policy', e.policy, 'risk_level', td.risk_level) AS data,
		'{}'::jsonb AS proposal_payload
	FROM tool_executions e
	JOIN tool_definitions td ON td.id = e.tool_id
	WHERE e.status IN ('failed', 'denied', 'rejected')
		AND e.created_at >= $2 AND e.created_at <= $3
		AND ($1::uuid IS NULL OR e.organization_id IS NOT DISTINCT FROM $1)
) findings`

const toolApprovalBacklogQuery = `
SELECT ` + commonFindingColumns + `
FROM (
	SELECT '` + SignalToolApprovalBacklog + `' AS category,
		e.organization_id,
		'tool_approval' AS entity_type,
		a.id::text AS entity_id,
		'tool approval pending longer than four hours' AS reason,
		CASE WHEN td.risk_level IN ('high', 'critical') THEN '` + SeverityHigh + `' ELSE '` + SeverityMedium + `' END AS severity,
		a.created_at AS occurred_at,
		jsonb_build_object('tool_name', td.name, 'execution_id', e.id::text, 'risk_level', td.risk_level, 'approval_status', a.status) AS data,
		'{}'::jsonb AS proposal_payload
	FROM tool_approvals a
	JOIN tool_executions e ON e.id = a.execution_id
	JOIN tool_definitions td ON td.id = e.tool_id
	WHERE a.status = 'pending'
		AND a.created_at <= $3 - INTERVAL '4 hours'
		AND ($1::uuid IS NULL OR e.organization_id IS NOT DISTINCT FROM $1)
) findings`

const contextRuleGapQuery = `
SELECT ` + commonFindingColumns + `
FROM (
	SELECT '` + SignalContextRuleGap + `' AS category,
		cp.organization_id,
		'context_package' AS entity_type,
		cp.id::text AS entity_id,
		'context package omitted reviewable fields' AS reason,
		'` + SeverityMedium + `' AS severity,
		cp.created_at AS occurred_at,
		jsonb_build_object('module_key', cp.module_key, 'target_type', cp.target_type, 'omissions', cp.omissions, 'dictionary_version_id', cp.dictionary_version_id::text) AS data,
		jsonb_build_object('dictionary_version_id', cp.dictionary_version_id::text, 'context_package_id', cp.id::text, 'module_key', cp.module_key, 'target_type', cp.target_type, 'omissions', cp.omissions) AS proposal_payload
	FROM context_packages cp
	WHERE cp.dictionary_version_id IS NOT NULL
		AND jsonb_array_length(cp.omissions) > 0
		AND cp.created_at >= $2 AND cp.created_at <= $3
		AND ($1::uuid IS NULL OR cp.organization_id IS NOT DISTINCT FROM $1)
) findings`

const contextBuildFailureQuery = `
SELECT ` + commonFindingColumns + `
FROM (
	SELECT '` + SignalContextBuildFailure + `' AS category,
		cp.organization_id,
		'context_package' AS entity_type,
		cp.id::text AS entity_id,
		COALESCE(NULLIF(cp.provenance->>'fallback_reason', ''), 'context package used fallback or lacked dictionary version') AS reason,
		'` + SeverityMedium + `' AS severity,
		cp.created_at AS occurred_at,
		jsonb_build_object('module_key', cp.module_key, 'target_type', cp.target_type, 'provenance', cp.provenance) AS data,
		'{}'::jsonb AS proposal_payload
	FROM context_packages cp
	WHERE (cp.dictionary_version_id IS NULL OR cp.provenance ? 'fallback_reason' OR COALESCE(cp.provenance->>'source', '') = 'compatibility_resolver')
		AND cp.created_at >= $2 AND cp.created_at <= $3
		AND ($1::uuid IS NULL OR cp.organization_id IS NOT DISTINCT FROM $1)
) findings`

const schemaChangeFailureQuery = `
SELECT ` + commonFindingColumns + `
FROM (
	SELECT '` + SignalSchemaChangeFailure + `' AS category,
		organization_id,
		'schema_change_request' AS entity_type,
		id::text AS entity_id,
		COALESCE(NULLIF(review_reason, ''), status) AS reason,
		'` + SeverityHigh + `' AS severity,
		COALESCE(applied_at, reviewed_at, created_at) AS occurred_at,
		jsonb_build_object('schema_name', schema_name, 'request_type', request_type, 'status', status) AS data,
		'{}'::jsonb AS proposal_payload
	FROM platform.schema_change_requests
	WHERE status = 'failed'
		AND created_at >= $2 AND created_at <= $3
		AND ($1::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $1)
) findings`

const financeExportFailureQuery = `
SELECT ` + commonFindingColumns + `
FROM (
	SELECT '` + SignalFinanceCallbackFailure + `' AS category,
		(SELECT l.organization_id FROM finance_export_lines l WHERE l.batch_id = b.id AND l.organization_id IS NOT NULL LIMIT 1) AS organization_id,
		'finance_export_batch' AS entity_type,
		b.id::text AS entity_id,
		COALESCE(NULLIF(b.error_message, ''), b.status) AS reason,
		'` + SeverityHigh + `' AS severity,
		b.updated_at AS occurred_at,
		jsonb_build_object('status', b.status, 'adapter_id', b.adapter_id::text, 'total_amount', b.total_amount::text) AS data,
		'{}'::jsonb AS proposal_payload
	FROM finance_export_batches b
	WHERE b.status = 'failed'
		AND b.updated_at >= $2 AND b.updated_at <= $3
		AND ($1::uuid IS NULL OR EXISTS (SELECT 1 FROM finance_export_lines l WHERE l.batch_id = b.id AND l.organization_id IS NOT DISTINCT FROM $1))
) findings`

const financeWebhookFailureQuery = `
SELECT ` + commonFindingColumns + `
FROM (
	SELECT '` + SignalFinanceCallbackFailure + `' AS category,
		(SELECT l.organization_id FROM finance_export_lines l WHERE l.batch_id = e.batch_id AND l.organization_id IS NOT NULL LIMIT 1) AS organization_id,
		'finance_webhook_event' AS entity_type,
		e.id::text AS entity_id,
		COALESCE(NULLIF(e.error_message, ''), 'finance webhook was not processed') AS reason,
		'` + SeverityMedium + `' AS severity,
		e.created_at AS occurred_at,
		jsonb_build_object('adapter_id', e.adapter_id::text, 'batch_id', e.batch_id::text, 'signature_valid', e.signature_valid, 'processed', e.processed) AS data,
		'{}'::jsonb AS proposal_payload
	FROM finance_webhook_events e
	WHERE (e.signature_valid = false OR e.processed = false OR e.error_message <> '')
		AND e.created_at >= $2 AND e.created_at <= $3
		AND ($1::uuid IS NULL OR EXISTS (SELECT 1 FROM finance_export_lines l WHERE l.batch_id = e.batch_id AND l.organization_id IS NOT DISTINCT FROM $1))
) findings`

const financeImportFailureQuery = `
SELECT ` + commonFindingColumns + `
FROM (
	SELECT '` + SignalFinanceCallbackFailure + `' AS category,
		NULL::uuid AS organization_id,
		'finance_import_batch' AS entity_type,
		id::text AS entity_id,
		status AS reason,
		CASE WHEN status = 'failed' THEN '` + SeverityHigh + `' ELSE '` + SeverityMedium + `' END AS severity,
		COALESCE(completed_at, created_at) AS occurred_at,
		jsonb_build_object('status', status, 'source_type', source_type, 'failed_records', failed_records, 'total_records', total_records) AS data,
		'{}'::jsonb AS proposal_payload
	FROM finance_import_batches
	WHERE status IN ('failed', 'completed_with_errors')
		AND created_at >= $2 AND created_at <= $3
		AND $1::uuid IS NULL
) findings`

const erpActionFailureQuery = `
SELECT ` + commonFindingColumns + `
FROM (
	SELECT '` + SignalERPActionFailure + `' AS category,
		CASE
			WHEN COALESCE("Payload"->>'organization_id', '') ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
			THEN ("Payload"->>'organization_id')::uuid
			ELSE NULL::uuid
		END AS organization_id,
		'erp_action_execution' AS entity_type,
		"ActionID"::text AS entity_id,
		COALESCE(NULLIF("FailureMessage", ''), "Status") AS reason,
		'` + SeverityHigh + `' AS severity,
		COALESCE("CompletedAt", "StartedAt") AS occurred_at,
		jsonb_build_object('table_code', "TableCode", 'record_key', "RecordKey", 'action', "Action", 'status', "Status", 'failure_code', "FailureCode", 'source', "Source") AS data,
		'{}'::jsonb AS proposal_payload
	FROM "MAEX"
	WHERE "Status" = 'failed'
		AND "StartedAt" >= $2 AND "StartedAt" <= $3
		AND (
			$1::uuid IS NULL OR
			CASE
				WHEN COALESCE("Payload"->>'organization_id', '') ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
				THEN ("Payload"->>'organization_id')::uuid
				ELSE NULL::uuid
			END IS NOT DISTINCT FROM $1
		)
) findings`

const costWithoutProgressQuery = `
SELECT ` + commonFindingColumns + `
FROM (
	SELECT '` + SignalCostWithoutBusinessProgress + `' AS category,
		i.organization_id,
		'ai_usage_ledger' AS entity_type,
		l.id::text AS entity_id,
		'AI cost has not been posted to project cost ledger' AS reason,
		'` + SeverityLow + `' AS severity,
		l.created_at AS occurred_at,
		jsonb_build_object('invocation_id', i.id::text, 'project_id', i.project_id::text, 'amount', l.amount::text, 'currency', l.currency, 'posted_to_project_cost', l.posted_to_project_cost) AS data,
		'{}'::jsonb AS proposal_payload
	FROM ai_usage_ledger l
	JOIN ai_invocations i ON i.id = l.invocation_id
	WHERE l.amount > 0
		AND i.project_id IS NOT NULL
		AND l.posted_to_project_cost = false
		AND l.created_at >= $2 AND l.created_at <= $3
		AND ($1::uuid IS NULL OR i.organization_id IS NOT DISTINCT FROM $1)
) findings`

const contextProposalStaleQuery = `
SELECT ` + commonFindingColumns + `
FROM (
	SELECT '` + SignalStaleProposal + `' AS category,
		dv.organization_id,
		'context_change_proposal' AS entity_type,
		p.id::text AS entity_id,
		'context change proposal pending longer than three days' AS reason,
		'` + SeverityMedium + `' AS severity,
		p.created_at AS occurred_at,
		jsonb_build_object('proposal_type', p.proposal_type, 'status', p.status, 'dictionary_version_id', p.dictionary_version_id::text) AS data,
		'{}'::jsonb AS proposal_payload
	FROM context_change_proposals p
	JOIN context_dictionary_versions dv ON dv.id = p.dictionary_version_id
	WHERE p.status IN ('pending', 'approved')
		AND p.created_at <= $3 - INTERVAL '3 days'
		AND ($1::uuid IS NULL OR dv.organization_id IS NOT DISTINCT FROM $1)
) findings`

const assistantProposalStaleQuery = `
SELECT ` + commonFindingColumns + `
FROM (
	SELECT '` + SignalStaleProposal + `' AS category,
		s.organization_id,
		'assistant_proposal' AS entity_type,
		p.id::text AS entity_id,
		'assistant proposal pending longer than three days' AS reason,
		'` + SeverityLow + `' AS severity,
		p.created_at AS occurred_at,
		jsonb_build_object('module_key', p.module_key, 'target_type', p.target_type, 'proposal_type', p.proposal_type, 'session_id', p.session_id::text) AS data,
		'{}'::jsonb AS proposal_payload
	FROM assistant_proposals p
	JOIN assistant_sessions s ON s.id = p.session_id
	WHERE p.status = 'pending'
		AND p.created_at <= $3 - INTERVAL '3 days'
		AND ($1::uuid IS NULL OR s.organization_id IS NOT DISTINCT FROM $1)
) findings`

type scanner interface {
	Scan(dest ...any) error
}

func scanRuns(rows pgx.Rows) ([]MonitoringAgentRun, error) {
	runs := make([]MonitoringAgentRun, 0)
	for rows.Next() {
		var run MonitoringAgentRun
		if err := scanRun(rows, &run); err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return runs, nil
}

func scanRun(row scanner, run *MonitoringAgentRun) error {
	var orgID pgtype.UUID
	var summaryJSON []byte
	var completedAt pgtype.Timestamptz
	if err := row.Scan(&run.ID, &run.TriggerType, &orgID, &run.Status, &run.LookbackStartedAt, &run.LookbackEndedAt,
		&run.SignalsCreated, &run.DuplicatesSuppressed, &summaryJSON, &run.ErrorMessage, &run.StartedAt, &completedAt); err != nil {
		return err
	}
	run.OrganizationID = uuidPointer(orgID)
	run.Summary = unmarshalMap(summaryJSON)
	if completedAt.Valid {
		t := completedAt.Time
		run.CompletedAt = &t
	}
	return nil
}

func mustJSON(value any) []byte {
	if value == nil {
		value = map[string]any{}
	}
	data, err := json.Marshal(value)
	if err != nil {
		return []byte(`{}`)
	}
	return data
}

func unmarshalMap(data []byte) map[string]any {
	result := map[string]any{}
	if len(data) == 0 {
		return result
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return map[string]any{}
	}
	return result
}

func nullableUUID(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return *id
}

func uuidPointer(value pgtype.UUID) *uuid.UUID {
	if !value.Valid {
		return nil
	}
	id := uuid.UUID(value.Bytes)
	return &id
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 200 {
		return 200
	}
	return limit
}

var _ = time.Time{}
