-- 040_saas_permission_system.sql
-- Organization-reviewed permission changes and scoped organization access rules.

CREATE TABLE IF NOT EXISTS permission_change_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    membership_id UUID NOT NULL REFERENCES organization_memberships(id) ON DELETE CASCADE,
    requested_by UUID REFERENCES users(id) ON DELETE SET NULL,
    requested_by_type TEXT NOT NULL DEFAULT 'human',
    requested_change JSONB NOT NULL DEFAULT '{}',
    reason TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'approved', 'rejected', 'applied', 'cancelled')),
    reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    review_reason TEXT NOT NULL DEFAULT '',
    reviewed_at TIMESTAMPTZ,
    applied_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_permission_change_requests_org_status
    ON permission_change_requests(organization_id, status, created_at DESC);

CREATE TABLE IF NOT EXISTS organization_access_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    scope_type TEXT NOT NULL DEFAULT 'organization'
        CHECK (scope_type IN ('organization', 'department', 'project', 'function', 'form', 'field')),
    scope_id TEXT NOT NULL DEFAULT '',
    resource_type TEXT NOT NULL DEFAULT '',
    resource_key TEXT NOT NULL DEFAULT '',
    action TEXT NOT NULL DEFAULT 'read',
    actor_type TEXT NOT NULL DEFAULT '*',
    actor_id TEXT NOT NULL DEFAULT '',
    role_id UUID REFERENCES roles(id) ON DELETE CASCADE,
    authority_tier TEXT NOT NULL DEFAULT ''
        CHECK (authority_tier IN ('', 'organization_creator', 'organization_admin', 'reviewer', 'executor')),
    behavior TEXT NOT NULL DEFAULT 'allow'
        CHECK (behavior IN ('allow', 'notify', 'approve', 'deny')),
    required_level TEXT NOT NULL DEFAULT 'L1'
        CHECK (required_level IN ('L1', 'L2', 'L3', 'L4')),
    priority INT NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'disabled', 'archived')),
    reason TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_org_access_rules_lookup
    ON organization_access_rules(organization_id, scope_type, scope_id, resource_type, action, status, priority DESC);

ALTER TABLE field_permission_rules
    ADD COLUMN IF NOT EXISTS organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    ADD COLUMN IF NOT EXISTS scope_type TEXT NOT NULL DEFAULT 'organization',
    ADD COLUMN IF NOT EXISTS scope_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS priority INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'disabled', 'archived'));

CREATE INDEX IF NOT EXISTS idx_field_permission_rules_scope
    ON field_permission_rules(organization_id, scope_type, scope_id, table_name, field_name, action, status, priority DESC);
