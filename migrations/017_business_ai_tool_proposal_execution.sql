-- platformdb:accept-checksum-drift 004_ai_capability_baseline.sql

BEGIN;

ALTER TABLE business_stage_ai_runs
    ADD COLUMN IF NOT EXISTS proposal_status TEXT NOT NULL DEFAULT 'not_submitted',
    ADD COLUMN IF NOT EXISTS tool_execution_id UUID,
    ADD COLUMN IF NOT EXISTS tool_approval_id UUID,
    ADD COLUMN IF NOT EXISTS proposal_result JSONB NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS proposal_error TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS proposal_submitted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS proposal_completed_at TIMESTAMPTZ;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'business_stage_ai_runs'::regclass
          AND conname = 'chk_business_stage_ai_proposal_status'
    ) THEN
        ALTER TABLE business_stage_ai_runs
            ADD CONSTRAINT chk_business_stage_ai_proposal_status
            CHECK (proposal_status IN ('not_submitted', 'submitting', 'approval_required', 'completed', 'rejected', 'failed', 'denied'));
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'business_stage_ai_runs'::regclass
          AND conname = 'chk_business_stage_ai_proposal_result_object'
    ) THEN
        ALTER TABLE business_stage_ai_runs
            ADD CONSTRAINT chk_business_stage_ai_proposal_result_object
            CHECK (jsonb_typeof(proposal_result) = 'object');
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'business_stage_ai_runs'::regclass
          AND conname = 'fk_business_stage_ai_tool_execution'
    ) THEN
        ALTER TABLE business_stage_ai_runs
            ADD CONSTRAINT fk_business_stage_ai_tool_execution
            FOREIGN KEY (tool_execution_id) REFERENCES tool_executions(id) ON DELETE SET NULL;
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'business_stage_ai_runs'::regclass
          AND conname = 'fk_business_stage_ai_tool_approval'
    ) THEN
        ALTER TABLE business_stage_ai_runs
            ADD CONSTRAINT fk_business_stage_ai_tool_approval
            FOREIGN KEY (tool_approval_id) REFERENCES tool_approvals(id) ON DELETE SET NULL;
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS uq_business_stage_ai_tool_execution
    ON business_stage_ai_runs(tool_execution_id) WHERE tool_execution_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_business_stage_ai_proposal_status
    ON business_stage_ai_runs(organization_id, proposal_status, created_at DESC);

COMMIT;
