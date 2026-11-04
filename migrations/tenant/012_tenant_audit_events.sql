-- Tenant-side audit trail for governance-relevant mutations.
-- Written by internal/pkg/auditlog.Record; the platform database keeps no copy
-- because the mutated rows live in each tenant's physical database.

CREATE TABLE IF NOT EXISTS audit_events (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    occurred_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    actor_id          UUID,
    actor_type        TEXT NOT NULL DEFAULT 'system',
    action            TEXT NOT NULL,
    entity_type       TEXT NOT NULL,
    entity_key        TEXT NOT NULL DEFAULT '',
    organization_id   UUID,
    detail            JSONB NOT NULL DEFAULT '{}'::JSONB
);

CREATE INDEX IF NOT EXISTS audit_events_occurred_at_idx
    ON audit_events (occurred_at DESC);

CREATE INDEX IF NOT EXISTS audit_events_entity_idx
    ON audit_events (entity_type, entity_key);
