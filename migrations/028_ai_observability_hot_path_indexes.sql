-- Add organization-scoped hot-path indexes for the platform AI observability
-- tables. ListInvocations / ListExecutions and the per-organization cost and
-- usage aggregations filter by organization_id and sort by created_at, which
-- previously required a full scan on these append-heavy shared tables.

CREATE INDEX IF NOT EXISTS idx_ai_invocations_org_created
    ON ai_invocations(organization_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_tool_executions_org_created
    ON tool_executions(organization_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_tool_executions_created
    ON tool_executions(created_at DESC);
