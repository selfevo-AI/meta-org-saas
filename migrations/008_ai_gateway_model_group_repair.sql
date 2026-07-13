-- 008_ai_gateway_model_group_repair.sql
-- Repair databases that applied an older 004_ai_capability_baseline.sql before
-- AI Gateway model groups, access tokens, and balance tables were folded into it.

CREATE TABLE IF NOT EXISTS ai_model_groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    department_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    project_id UUID,
    agent_id UUID REFERENCES ai_agents(id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    group_key TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'disabled')),
    rate_multiplier NUMERIC(12,6) NOT NULL DEFAULT 1,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_ai_model_groups_org_key_ci
    ON ai_model_groups (organization_id, lower(btrim(group_key)))
    WHERE group_key <> '';
CREATE INDEX IF NOT EXISTS idx_ai_model_groups_scope
    ON ai_model_groups(organization_id, department_id, project_id, status);

CREATE TABLE IF NOT EXISTS ai_model_channel_abilities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_group_id UUID REFERENCES ai_model_groups(id) ON DELETE CASCADE,
    channel_id UUID NOT NULL REFERENCES model_provider_channels(id) ON DELETE CASCADE,
    requested_model TEXT NOT NULL DEFAULT '',
    model_pattern TEXT NOT NULL DEFAULT '*',
    upstream_model TEXT NOT NULL DEFAULT '',
    priority INT NOT NULL DEFAULT 100,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ai_model_channel_abilities_lookup
    ON ai_model_channel_abilities(model_group_id, enabled, priority);
CREATE INDEX IF NOT EXISTS idx_ai_model_channel_abilities_channel
    ON ai_model_channel_abilities(channel_id, enabled);

CREATE TABLE IF NOT EXISTS ai_access_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    model_group_id UUID REFERENCES ai_model_groups(id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    masked_token TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'disabled', 'expired', 'revoked')),
    allowed_models JSONB NOT NULL DEFAULT '[]',
    allowed_model_patterns JSONB NOT NULL DEFAULT '[]',
    allowed_ip_ranges JSONB NOT NULL DEFAULT '[]',
    allow_channel_override BOOLEAN NOT NULL DEFAULT FALSE,
    quota_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    quota_used NUMERIC(18,8) NOT NULL DEFAULT 0,
    quota_currency TEXT NOT NULL DEFAULT 'CNY',
    expires_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ai_access_tokens_org_status
    ON ai_access_tokens(organization_id, status, created_at DESC);

CREATE TABLE IF NOT EXISTS ai_gateway_balances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    balance_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    reserved_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'CNY',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(organization_id, currency)
);

CREATE TABLE IF NOT EXISTS ai_gateway_balance_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    access_token_id UUID REFERENCES ai_access_tokens(id) ON DELETE SET NULL,
    model_group_id UUID REFERENCES ai_model_groups(id) ON DELETE SET NULL,
    invocation_id UUID REFERENCES ai_invocations(id) ON DELETE SET NULL,
    reservation_id UUID REFERENCES ai_gateway_balance_transactions(id) ON DELETE SET NULL,
    transaction_type TEXT NOT NULL
        CHECK (transaction_type IN ('reserve', 'settle', 'refund', 'adjustment')),
    amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'CNY',
    reason TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}',
    reconcile_lease_owner TEXT NOT NULL DEFAULT '',
    reconcile_lease_expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ai_gateway_balance_transactions_org
    ON ai_gateway_balance_transactions(organization_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ai_gateway_balance_transactions_reservation
    ON ai_gateway_balance_transactions(reservation_id);
CREATE INDEX IF NOT EXISTS idx_ai_gateway_open_reservations_reconcile
    ON ai_gateway_balance_transactions(created_at, reconcile_lease_expires_at)
    WHERE transaction_type = 'reserve';

ALTER TABLE ai_invocations
    ADD COLUMN IF NOT EXISTS access_token_id UUID REFERENCES ai_access_tokens(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS model_group_id UUID REFERENCES ai_model_groups(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS retry_count INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS failover_chain JSONB NOT NULL DEFAULT '[]',
    ADD COLUMN IF NOT EXISTS reserved_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS settled_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS upstream_status_code INT NOT NULL DEFAULT 0;

ALTER TABLE ai_usage_ledger
    ADD COLUMN IF NOT EXISTS access_token_id UUID REFERENCES ai_access_tokens(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS model_group_id UUID REFERENCES ai_model_groups(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_ai_invocations_access_token
    ON ai_invocations(access_token_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ai_usage_ledger_access_token
    ON ai_usage_ledger(access_token_id, created_at DESC);
