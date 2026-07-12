-- Add governed payload retention without deleting financial or approval evidence.
-- platformdb:accept-checksum-drift 004_ai_capability_baseline.sql

ALTER TABLE ai_invocations ADD COLUMN IF NOT EXISTS retention_redacted_at TIMESTAMPTZ;
ALTER TABLE business_stage_ai_runs ADD COLUMN IF NOT EXISTS retention_redacted_at TIMESTAMPTZ;
ALTER TABLE tool_executions ADD COLUMN IF NOT EXISTS retention_redacted_at TIMESTAMPTZ;
ALTER TABLE assistant_sessions ADD COLUMN IF NOT EXISTS retention_redacted_at TIMESTAMPTZ;
ALTER TABLE assistant_messages ADD COLUMN IF NOT EXISTS retention_redacted_at TIMESTAMPTZ;
ALTER TABLE assistant_steps ADD COLUMN IF NOT EXISTS retention_redacted_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_ai_invocations_retention
    ON ai_invocations(created_at) WHERE retention_redacted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_business_stage_ai_runs_retention
    ON business_stage_ai_runs(created_at) WHERE retention_redacted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_tool_executions_retention
    ON tool_executions(created_at) WHERE retention_redacted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_assistant_sessions_retention
    ON assistant_sessions(updated_at) WHERE retention_redacted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_assistant_messages_retention
    ON assistant_messages(created_at) WHERE retention_redacted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_assistant_steps_retention
    ON assistant_steps(created_at) WHERE retention_redacted_at IS NULL;

CREATE TABLE IF NOT EXISTS platform.audit_retention_runs (
    id              BIGSERIAL PRIMARY KEY,
    started_at      TIMESTAMPTZ NOT NULL,
    completed_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    cutoff_at       TIMESTAMPTZ NOT NULL,
    retention_days  INT NOT NULL CHECK (retention_days > 0),
    batch_size      INT NOT NULL CHECK (batch_size > 0),
    status          TEXT NOT NULL CHECK (status IN ('succeeded', 'failed')),
    redaction_counts JSONB NOT NULL DEFAULT '{}',
    error_message   TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_audit_retention_runs_completed
    ON platform.audit_retention_runs(completed_at DESC);
