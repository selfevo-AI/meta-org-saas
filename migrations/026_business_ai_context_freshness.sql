-- Prevent governed Business AI proposals from executing against stale project
-- facts captured before concurrent business changes.
-- platformdb:accept-checksum-drift 004_ai_capability_baseline.sql
-- platformdb:accept-checksum-drift 016_business_stage_ai_runs.sql

ALTER TABLE business_stage_ai_runs
    ADD COLUMN IF NOT EXISTS authoritative_context_hash TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_business_stage_ai_context_hash
    ON business_stage_ai_runs(organization_id, project_id, authoritative_context_hash)
    WHERE authoritative_context_hash <> '';
