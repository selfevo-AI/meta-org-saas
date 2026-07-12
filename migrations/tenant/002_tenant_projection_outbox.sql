-- tenantdb:accept-checksum-drift 001_tenant_business_baseline.sql

CREATE TABLE IF NOT EXISTS tenant_integration_outbox (
    event_id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type        TEXT NOT NULL,
    event_version     INTEGER NOT NULL DEFAULT 1 CHECK (event_version > 0),
    organization_id   UUID NOT NULL,
    actor_id           UUID,
    actor_type         TEXT NOT NULL DEFAULT 'system',
    authority_tier     TEXT NOT NULL DEFAULT '',
    aggregate_type     TEXT NOT NULL,
    aggregate_id       TEXT NOT NULL,
    aggregate_version  BIGINT NOT NULL DEFAULT 1 CHECK (aggregate_version > 0),
    trace_id           UUID NOT NULL,
    causation_id       UUID NOT NULL,
    correlation_id     UUID NOT NULL,
    occurred_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    schema_version     TEXT NOT NULL DEFAULT 'meta-org.tenant-projection.v1',
    payload            JSONB NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(payload) = 'object'),
    metadata           JSONB NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(metadata) = 'object'),
    status             TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'running', 'published', 'failed')),
    attempt_count      INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    available_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    lease_owner        TEXT NOT NULL DEFAULT '',
    lease_expires_at   TIMESTAMPTZ,
    published_at       TIMESTAMPTZ,
    last_error         TEXT NOT NULL DEFAULT '',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tenant_integration_outbox_claim
    ON tenant_integration_outbox(status, available_at, occurred_at)
    WHERE status IN ('pending', 'running', 'failed');

CREATE INDEX IF NOT EXISTS idx_tenant_integration_outbox_organization
    ON tenant_integration_outbox(organization_id, occurred_at DESC);

CREATE OR REPLACE FUNCTION tenant_outbox_context_uuid(p_setting TEXT)
RETURNS UUID
LANGUAGE plpgsql
STABLE
AS $$
DECLARE
    value TEXT;
BEGIN
    value := NULLIF(current_setting(p_setting, TRUE), '');
    IF value IS NULL THEN
        RETURN NULL;
    END IF;
    BEGIN
        RETURN value::UUID;
    EXCEPTION WHEN invalid_text_representation THEN
        RETURN NULL;
    END;
END;
$$;

CREATE OR REPLACE FUNCTION emit_tenant_projection_outbox_event()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    row_data JSONB;
    organization_id_value UUID;
    actor_id_value UUID;
    trace_id_value UUID;
    occurred_at_value TIMESTAMPTZ;
    aggregate_version_value BIGINT;
BEGIN
    row_data := CASE WHEN TG_OP = 'DELETE' THEN to_jsonb(OLD) ELSE to_jsonb(NEW) END;
    organization_id_value := NULLIF(row_data->>'organization_id', '')::UUID;

    IF organization_id_value IS NULL AND TG_TABLE_NAME = 'project_cost_entries' THEN
        SELECT project.organization_id INTO organization_id_value
        FROM projects AS project
        WHERE project.id = NULLIF(row_data->>'project_id', '')::UUID;
    ELSIF organization_id_value IS NULL AND TG_TABLE_NAME = 'tasks' THEN
        SELECT workflow.organization_id INTO organization_id_value
        FROM workflow_instances AS workflow
        WHERE workflow.id = NULLIF(row_data->>'workflow_id', '')::UUID;
    ELSIF organization_id_value IS NULL AND TG_TABLE_NAME = 'decisions' THEN
        SELECT workflow.organization_id INTO organization_id_value
        FROM tasks AS task
        JOIN workflow_instances AS workflow ON workflow.id = task.workflow_id
        WHERE task.id = NULLIF(row_data->>'task_id', '')::UUID;
    END IF;

    IF organization_id_value IS NULL THEN
        IF TG_OP = 'DELETE' THEN
            RETURN OLD;
        END IF;
        RETURN NEW;
    END IF;

    actor_id_value := COALESCE(
        tenant_outbox_context_uuid('meta_org.actor_id'),
        NULLIF(row_data->>'created_by_id', '')::UUID,
        NULLIF(row_data->>'actor_id', '')::UUID
    );
    trace_id_value := COALESCE(tenant_outbox_context_uuid('meta_org.trace_id'), gen_random_uuid());
    occurred_at_value := COALESCE(
        NULLIF(row_data->>'updated_at', '')::TIMESTAMPTZ,
        NULLIF(row_data->>'occurred_at', '')::TIMESTAMPTZ,
        NULLIF(row_data->>'created_at', '')::TIMESTAMPTZ,
        NOW()
    );
    aggregate_version_value := GREATEST(
        1,
        FLOOR(EXTRACT(EPOCH FROM occurred_at_value) * 1000000)::BIGINT
    );

    INSERT INTO tenant_integration_outbox(
        event_type,
        organization_id,
        actor_id,
        actor_type,
        authority_tier,
        aggregate_type,
        aggregate_id,
        aggregate_version,
        trace_id,
        causation_id,
        correlation_id,
        occurred_at,
        payload,
        metadata
    )
    VALUES (
        'tenant.' || TG_TABLE_NAME || '.changed',
        organization_id_value,
        actor_id_value,
        COALESCE(NULLIF(current_setting('meta_org.actor_type', TRUE), ''), 'system'),
        COALESCE(NULLIF(current_setting('meta_org.authority_tier', TRUE), ''), ''),
        TG_TABLE_NAME,
        COALESCE(NULLIF(row_data->>'id', ''), TG_TABLE_NAME),
        aggregate_version_value,
        trace_id_value,
        COALESCE(tenant_outbox_context_uuid('meta_org.causation_id'), trace_id_value),
        COALESCE(tenant_outbox_context_uuid('meta_org.correlation_id'), trace_id_value),
        occurred_at_value,
        jsonb_build_object('operation', LOWER(TG_OP), 'table', TG_TABLE_NAME),
        jsonb_build_object('source', 'database_trigger')
    );

    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_requirements_projection_outbox ON requirements;
CREATE TRIGGER trg_requirements_projection_outbox
AFTER INSERT OR UPDATE OR DELETE ON requirements
FOR EACH ROW EXECUTE FUNCTION emit_tenant_projection_outbox_event();

DROP TRIGGER IF EXISTS trg_projects_projection_outbox ON projects;
CREATE TRIGGER trg_projects_projection_outbox
AFTER INSERT OR UPDATE OR DELETE ON projects
FOR EACH ROW EXECUTE FUNCTION emit_tenant_projection_outbox_event();

DROP TRIGGER IF EXISTS trg_workflow_templates_projection_outbox ON workflow_templates;
CREATE TRIGGER trg_workflow_templates_projection_outbox
AFTER INSERT OR UPDATE OR DELETE ON workflow_templates
FOR EACH ROW EXECUTE FUNCTION emit_tenant_projection_outbox_event();

DROP TRIGGER IF EXISTS trg_workflow_instances_projection_outbox ON workflow_instances;
CREATE TRIGGER trg_workflow_instances_projection_outbox
AFTER INSERT OR UPDATE OR DELETE ON workflow_instances
FOR EACH ROW EXECUTE FUNCTION emit_tenant_projection_outbox_event();

DROP TRIGGER IF EXISTS trg_tasks_projection_outbox ON tasks;
CREATE TRIGGER trg_tasks_projection_outbox
AFTER INSERT OR UPDATE OR DELETE ON tasks
FOR EACH ROW EXECUTE FUNCTION emit_tenant_projection_outbox_event();

DROP TRIGGER IF EXISTS trg_decisions_projection_outbox ON decisions;
CREATE TRIGGER trg_decisions_projection_outbox
AFTER INSERT OR UPDATE OR DELETE ON decisions
FOR EACH ROW EXECUTE FUNCTION emit_tenant_projection_outbox_event();

DROP TRIGGER IF EXISTS trg_project_cost_entries_projection_outbox ON project_cost_entries;
CREATE TRIGGER trg_project_cost_entries_projection_outbox
AFTER INSERT OR UPDATE OR DELETE ON project_cost_entries
FOR EACH ROW EXECUTE FUNCTION emit_tenant_projection_outbox_event();

INSERT INTO tenant_integration_outbox(
    event_type,
    organization_id,
    aggregate_type,
    aggregate_id,
    trace_id,
    causation_id,
    correlation_id,
    payload,
    metadata
)
SELECT
    'tenant.snapshot.bootstrap',
    organization.id,
    'organization',
    organization.id::TEXT,
    seed.trace_id,
    seed.trace_id,
    seed.trace_id,
    '{"operation":"bootstrap","table":"organization"}'::JSONB,
    '{"source":"002_tenant_projection_outbox"}'::JSONB
FROM organizations AS organization
CROSS JOIN LATERAL (SELECT gen_random_uuid() AS trace_id) AS seed
WHERE organization.status = 'active';
