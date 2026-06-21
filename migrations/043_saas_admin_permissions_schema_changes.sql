ALTER TABLE organizations
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'closed')),
    ADD COLUMN IF NOT EXISTS closed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS closed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS closed_reason TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_organizations_status
    ON organizations(status, updated_at DESC);

ALTER TABLE platform.schema_change_requests
    ADD COLUMN IF NOT EXISTS risk_level TEXT NOT NULL DEFAULT 'safe'
        CHECK (risk_level IN ('safe', 'destructive')),
    ADD COLUMN IF NOT EXISTS diff JSONB NOT NULL DEFAULT '[]'::jsonb;
