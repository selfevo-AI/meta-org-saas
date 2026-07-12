package auditretention

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RedactionCounts struct {
	AIInvocations     int64 `json:"ai_invocations"`
	BusinessAIRuns    int64 `json:"business_ai_runs"`
	ToolExecutions    int64 `json:"tool_executions"`
	AssistantSessions int64 `json:"assistant_sessions"`
	AssistantMessages int64 `json:"assistant_messages"`
	AssistantSteps    int64 `json:"assistant_steps"`
}

func (c RedactionCounts) Total() int64 {
	return c.AIInvocations + c.BusinessAIRuns + c.ToolExecutions + c.AssistantSessions + c.AssistantMessages + c.AssistantSteps
}

func (c *RedactionCounts) Add(other RedactionCounts) {
	c.AIInvocations += other.AIInvocations
	c.BusinessAIRuns += other.BusinessAIRuns
	c.ToolExecutions += other.ToolExecutions
	c.AssistantSessions += other.AssistantSessions
	c.AssistantMessages += other.AssistantMessages
	c.AssistantSteps += other.AssistantSteps
}

type RunRecord struct {
	StartedAt     time.Time
	CutoffAt      time.Time
	RetentionDays int
	BatchSize     int
	Status        string
	Counts        RedactionCounts
	ErrorMessage  string
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) RedactBatch(ctx context.Context, cutoff time.Time, batchSize int) (RedactionCounts, error) {
	if r == nil || r.db == nil {
		return RedactionCounts{}, fmt.Errorf("audit retention database is not configured")
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return RedactionCounts{}, fmt.Errorf("begin audit retention batch: %w", err)
	}
	defer tx.Rollback(ctx)

	var counts RedactionCounts
	if counts.AIInvocations, err = redactRows(ctx, tx, `
		WITH candidates AS (
			SELECT id FROM ai_invocations
			WHERE retention_redacted_at IS NULL AND created_at < $1
			  AND status IN ('completed', 'failed', 'cancelled')
			ORDER BY created_at LIMIT $2 FOR UPDATE SKIP LOCKED
		)
		UPDATE ai_invocations target
		SET provider_request_id = '', error_type = '', error_message = '',
		    metadata = jsonb_build_object('retention_redacted', true), retention_redacted_at = NOW()
		FROM candidates WHERE target.id = candidates.id
	`, cutoff, batchSize); err != nil {
		return RedactionCounts{}, fmt.Errorf("redact ai invocations: %w", err)
	}
	if counts.BusinessAIRuns, err = redactRows(ctx, tx, `
		WITH candidates AS (
			SELECT id FROM business_stage_ai_runs
			WHERE retention_redacted_at IS NULL AND created_at < $1
			  AND status IN ('completed', 'failed')
			ORDER BY created_at LIMIT $2 FOR UPDATE SKIP LOCKED
		)
		UPDATE business_stage_ai_runs target
		SET input_context = '{}'::jsonb,
		    result = jsonb_build_object('retention_redacted', true),
		    proposal_result = '{}'::jsonb, error_message = '', proposal_error = '', retention_redacted_at = NOW()
		FROM candidates WHERE target.id = candidates.id
	`, cutoff, batchSize); err != nil {
		return RedactionCounts{}, fmt.Errorf("redact business ai runs: %w", err)
	}
	if counts.ToolExecutions, err = redactRows(ctx, tx, `
		WITH candidates AS (
			SELECT id FROM tool_executions
			WHERE retention_redacted_at IS NULL AND created_at < $1
			  AND status IN ('completed', 'rejected', 'denied', 'failed')
			ORDER BY created_at LIMIT $2 FOR UPDATE SKIP LOCKED
		)
		UPDATE tool_executions target
		SET arguments = '{}'::jsonb, result_summary = '', result = '{}'::jsonb,
		    error_message = '', retention_redacted_at = NOW()
		FROM candidates WHERE target.id = candidates.id
	`, cutoff, batchSize); err != nil {
		return RedactionCounts{}, fmt.Errorf("redact tool executions: %w", err)
	}
	if counts.AssistantMessages, err = redactRows(ctx, tx, `
		WITH candidates AS (
			SELECT message.id
			FROM assistant_messages message
			JOIN assistant_sessions session ON session.id = message.session_id
			WHERE message.retention_redacted_at IS NULL AND message.created_at < $1
			  AND session.status IN ('completed', 'failed', 'cancelled')
			ORDER BY message.created_at LIMIT $2 FOR UPDATE OF message SKIP LOCKED
		)
		UPDATE assistant_messages target
		SET content = '', tool_call_id = '', metadata = jsonb_build_object('retention_redacted', true),
		    retention_redacted_at = NOW()
		FROM candidates WHERE target.id = candidates.id
	`, cutoff, batchSize); err != nil {
		return RedactionCounts{}, fmt.Errorf("redact assistant messages: %w", err)
	}
	if counts.AssistantSteps, err = redactRows(ctx, tx, `
		WITH candidates AS (
			SELECT step.id
			FROM assistant_steps step
			JOIN assistant_sessions session ON session.id = step.session_id
			WHERE step.retention_redacted_at IS NULL AND step.created_at < $1
			  AND session.status IN ('completed', 'failed', 'cancelled')
			ORDER BY step.created_at LIMIT $2 FOR UPDATE OF step SKIP LOCKED
		)
		UPDATE assistant_steps target
		SET summary = '', data = '{}'::jsonb, retention_redacted_at = NOW()
		FROM candidates WHERE target.id = candidates.id
	`, cutoff, batchSize); err != nil {
		return RedactionCounts{}, fmt.Errorf("redact assistant steps: %w", err)
	}
	if counts.AssistantSessions, err = redactRows(ctx, tx, `
		WITH candidates AS (
			SELECT id FROM assistant_sessions
			WHERE retention_redacted_at IS NULL AND updated_at < $1
			  AND status IN ('completed', 'failed', 'cancelled')
			ORDER BY updated_at LIMIT $2 FOR UPDATE SKIP LOCKED
		)
		UPDATE assistant_sessions target
		SET title = '', working_memory = '{}'::jsonb, metadata = jsonb_build_object('retention_redacted', true),
		    last_error = '', retention_redacted_at = NOW()
		FROM candidates WHERE target.id = candidates.id
	`, cutoff, batchSize); err != nil {
		return RedactionCounts{}, fmt.Errorf("redact assistant sessions: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return RedactionCounts{}, fmt.Errorf("commit audit retention batch: %w", err)
	}
	return counts, nil
}

func (r *Repository) RecordRun(ctx context.Context, record RunRecord) error {
	counts, err := json.Marshal(record.Counts)
	if err != nil {
		return fmt.Errorf("marshal audit retention counts: %w", err)
	}
	if _, err := r.db.Exec(ctx, `
		INSERT INTO platform.audit_retention_runs(
		    started_at, cutoff_at, retention_days, batch_size, status, redaction_counts, error_message
		)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7)
	`, record.StartedAt, record.CutoffAt, record.RetentionDays, record.BatchSize, record.Status, counts, record.ErrorMessage); err != nil {
		return fmt.Errorf("record audit retention run: %w", err)
	}
	return nil
}

func redactRows(ctx context.Context, tx pgx.Tx, sql string, cutoff time.Time, batchSize int) (int64, error) {
	tag, err := tx.Exec(ctx, sql, cutoff, batchSize)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
