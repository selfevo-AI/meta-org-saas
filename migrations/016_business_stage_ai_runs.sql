-- platformdb:accept-checksum-drift 004_ai_capability_baseline.sql

BEGIN;

CREATE TABLE IF NOT EXISTS business_stage_ai_runs (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id   UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    project_id        UUID NOT NULL,
    requirement_id    UUID,
    stage             TEXT NOT NULL CHECK (stage IN ('plan', 'do', 'change', 'accept', 'learn')),
    status            TEXT NOT NULL DEFAULT 'running' CHECK (status IN ('running', 'completed', 'failed')),
    requested_by_id   UUID,
    requested_by_type TEXT NOT NULL DEFAULT 'human',
    provider_type     TEXT NOT NULL DEFAULT '',
    requested_model   TEXT NOT NULL DEFAULT '',
    invocation_id     UUID REFERENCES ai_invocations(id) ON DELETE SET NULL,
    resolved_model    TEXT NOT NULL DEFAULT '',
    input_context     JSONB NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(input_context) = 'object'),
    authoritative_context_hash TEXT NOT NULL DEFAULT '',
    result            JSONB NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(result) = 'object'),
    cost_amount       NUMERIC(18,8) NOT NULL DEFAULT 0,
    currency          TEXT NOT NULL DEFAULT 'CNY',
    input_tokens      INT NOT NULL DEFAULT 0,
    output_tokens     INT NOT NULL DEFAULT 0,
    error_message     TEXT NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at      TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_business_stage_ai_runs_project
    ON business_stage_ai_runs(organization_id, project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_business_stage_ai_runs_stage
    ON business_stage_ai_runs(organization_id, stage, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_business_stage_ai_context_hash
    ON business_stage_ai_runs(organization_id, project_id, authoritative_context_hash)
    WHERE authoritative_context_hash <> '';

COMMIT;
