BEGIN;

CREATE TABLE IF NOT EXISTS platform.tenant_integration_inbox (
    event_id           UUID PRIMARY KEY,
    event_type         TEXT NOT NULL,
    event_version      INTEGER NOT NULL CHECK (event_version > 0),
    organization_id    UUID NOT NULL REFERENCES public.organizations(id) ON DELETE CASCADE,
    actor_id           UUID,
    actor_type         TEXT NOT NULL DEFAULT 'system',
    authority_tier     TEXT NOT NULL DEFAULT '',
    aggregate_type     TEXT NOT NULL,
    aggregate_id       TEXT NOT NULL,
    aggregate_version  BIGINT NOT NULL CHECK (aggregate_version > 0),
    trace_id            UUID NOT NULL,
    causation_id        UUID NOT NULL,
    correlation_id      UUID NOT NULL,
    occurred_at         TIMESTAMPTZ NOT NULL,
    schema_version      TEXT NOT NULL,
    payload             JSONB NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(payload) = 'object'),
    metadata            JSONB NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(metadata) = 'object'),
    status              TEXT NOT NULL DEFAULT 'received'
        CHECK (status IN ('received', 'projected', 'failed')),
    received_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    projected_at        TIMESTAMPTZ,
    last_error          TEXT NOT NULL DEFAULT '',
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tenant_integration_inbox_projection
    ON platform.tenant_integration_inbox(status, received_at, occurred_at);
CREATE INDEX IF NOT EXISTS idx_tenant_integration_inbox_organization
    ON platform.tenant_integration_inbox(organization_id, occurred_at DESC);

CREATE TABLE IF NOT EXISTS platform.tenant_operational_projections (
    organization_id              UUID PRIMARY KEY REFERENCES public.organizations(id) ON DELETE CASCADE,
    open_requirements            BIGINT NOT NULL DEFAULT 0,
    active_projects              BIGINT NOT NULL DEFAULT 0,
    projects_by_status           JSONB NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(projects_by_status) = 'object'),
    over_budget_projects         BIGINT NOT NULL DEFAULT 0,
    project_cost_today           NUMERIC(20,6) NOT NULL DEFAULT 0,
    project_cost_month_to_date   NUMERIC(20,6) NOT NULL DEFAULT 0,
    project_cost_currency        TEXT NOT NULL DEFAULT 'CNY',
    workflow_templates           BIGINT NOT NULL DEFAULT 0,
    active_workflow_templates    BIGINT NOT NULL DEFAULT 0,
    workflow_instances           BIGINT NOT NULL DEFAULT 0,
    workflow_instances_by_status JSONB NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(workflow_instances_by_status) = 'object'),
    workflow_tasks_by_status     JSONB NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(workflow_tasks_by_status) = 'object'),
    workflow_decisions_7d        BIGINT NOT NULL DEFAULT 0,
    source_event_id              UUID NOT NULL,
    source_occurred_at           TIMESTAMPTZ NOT NULL,
    projected_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    projection_lag_ms            BIGINT NOT NULL DEFAULT 0 CHECK (projection_lag_ms >= 0),
    snapshot_version             BIGINT NOT NULL DEFAULT 1 CHECK (snapshot_version > 0),
    metadata                     JSONB NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(metadata) = 'object')
);

CREATE INDEX IF NOT EXISTS idx_tenant_operational_projections_freshness
    ON platform.tenant_operational_projections(projected_at, projection_lag_ms DESC);

CREATE TABLE IF NOT EXISTS platform.tenant_workflow_projections (
    organization_id UUID NOT NULL REFERENCES public.organizations(id) ON DELETE CASCADE,
    workflow_id     UUID NOT NULL,
    project_id      UUID,
    status          TEXT NOT NULL,
    current_stage   INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL,
    updated_at      TIMESTAMPTZ NOT NULL,
    source_event_id UUID NOT NULL,
    projected_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (organization_id, workflow_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_tenant_workflow_projections_workflow
    ON platform.tenant_workflow_projections(workflow_id);
CREATE INDEX IF NOT EXISTS idx_tenant_workflow_projections_status
    ON platform.tenant_workflow_projections(organization_id, status, updated_at DESC);

CREATE TABLE IF NOT EXISTS platform.tenant_activity_projections (
    organization_id UUID NOT NULL REFERENCES public.organizations(id) ON DELETE CASCADE,
    item_type       TEXT NOT NULL,
    item_id         TEXT NOT NULL,
    title           TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT '',
    occurred_at     TIMESTAMPTZ NOT NULL,
    source_event_id UUID NOT NULL,
    projected_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata        JSONB NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(metadata) = 'object'),
    PRIMARY KEY (organization_id, item_type, item_id)
);

CREATE INDEX IF NOT EXISTS idx_tenant_activity_projections_recent
    ON platform.tenant_activity_projections(organization_id, occurred_at DESC);

COMMIT;
