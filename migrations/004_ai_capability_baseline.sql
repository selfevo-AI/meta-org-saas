-- 004_ai_capability_baseline.sql
-- AI capability baseline.
--
-- This stage runs after SaaS platform management, ERP baseline, and ERP platform
-- integration. It owns model/provider management, agents, AI invocation ledger,
-- tool runtime, assistant runtime/context, unified skills, skill publication,
-- and AI capability platform projections.
--
-- Cross-stage foreign keys are rebuilt at the end of this file after both the
-- SaaS/ERP tables and AI capability tables exist.
--
-- 004 归属：模型、provider/channel、agent、AI 调用与用量、工具运行时、
-- 助手运行时/上下文、统一 skill、skill 发布，以及这些 AI 能力相关的
-- 后置外键重建和平台主数据投影。
CREATE TABLE IF NOT EXISTS ai_agents (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(255) NOT NULL,
    model_type      VARCHAR(100) NOT NULL,
    api_key_hash    VARCHAR(255) NOT NULL,
    capabilities    JSONB NOT NULL DEFAULT '[]',
    permission_level permission_level NOT NULL DEFAULT 'L2',
    metadata        JSONB DEFAULT '{}',
    is_active       BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);


CREATE TABLE IF NOT EXISTS agent_roles (
    agent_id        UUID NOT NULL REFERENCES ai_agents(id) ON DELETE CASCADE,
    role_id         UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    PRIMARY KEY (agent_id, role_id)
);


-- Folded from historical migration: 016_ai_gateway.sql
-- -----------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS model_providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    provider_type TEXT NOT NULL CHECK (provider_type IN ('openai', 'anthropic', 'gemini')),
    base_url TEXT NOT NULL DEFAULT '',
    encrypted_api_key TEXT NOT NULL,
    masked_api_key TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled', 'error')),
    timeout_ms INT NOT NULL DEFAULT 60000,
    retry_count INT NOT NULL DEFAULT 1,
    risk_level TEXT NOT NULL DEFAULT 'medium' CHECK (risk_level IN ('low', 'medium', 'high', 'critical')),
    tags JSONB NOT NULL DEFAULT '[]',
    metadata JSONB NOT NULL DEFAULT '{}',
    last_test_status TEXT NOT NULL DEFAULT '',
    last_test_error TEXT NOT NULL DEFAULT '',
    last_tested_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS models (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id UUID NOT NULL REFERENCES model_providers(id) ON DELETE CASCADE,
    model_key TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    context_window INT NOT NULL DEFAULT 0,
    max_output_tokens INT NOT NULL DEFAULT 0,
    capabilities JSONB NOT NULL DEFAULT '[]',
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(provider_id, model_key)
);

CREATE TABLE IF NOT EXISTS model_price_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_id UUID NOT NULL REFERENCES models(id) ON DELETE CASCADE,
    input_price_per_1k NUMERIC(18,8) NOT NULL DEFAULT 0,
    output_price_per_1k NUMERIC(18,8) NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'CNY',
    effective_from TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    effective_to TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS ai_invocations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id UUID NOT NULL REFERENCES model_providers(id) ON DELETE RESTRICT,
    model_id UUID NOT NULL REFERENCES models(id) ON DELETE RESTRICT,
    mode TEXT NOT NULL CHECK (mode IN ('sync', 'stream')),
    status TEXT NOT NULL DEFAULT 'started' CHECK (status IN ('started', 'streaming', 'completed', 'failed', 'cancelled')),
    organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
    department_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    project_id UUID,
    requirement_id UUID,
    workflow_id UUID REFERENCES workflow_instances(id) ON DELETE SET NULL,
    task_id UUID REFERENCES tasks(id) ON DELETE SET NULL,
    agent_id UUID REFERENCES ai_agents(id) ON DELETE SET NULL,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    capability_id UUID REFERENCES capabilities(id) ON DELETE SET NULL,
    source_surface TEXT NOT NULL DEFAULT '',
    request_hash TEXT NOT NULL DEFAULT '',
    provider_request_id TEXT NOT NULL DEFAULT '',
    input_tokens INT NOT NULL DEFAULT 0,
    output_tokens INT NOT NULL DEFAULT 0,
    estimated_input_tokens INT NOT NULL DEFAULT 0,
    estimated_output_tokens INT NOT NULL DEFAULT 0,
    cost_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'CNY',
    first_token_ms INT NOT NULL DEFAULT 0,
    duration_ms INT NOT NULL DEFAULT 0,
    error_type TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    retention_redacted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS ai_usage_ledger (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invocation_id UUID NOT NULL REFERENCES ai_invocations(id) ON DELETE RESTRICT,
    model_price_version_id UUID REFERENCES model_price_versions(id) ON DELETE SET NULL,
    ledger_type TEXT NOT NULL DEFAULT 'usage' CHECK (ledger_type IN ('usage', 'adjustment')),
    amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'CNY',
    input_tokens INT NOT NULL DEFAULT 0,
    output_tokens INT NOT NULL DEFAULT 0,
    posted_to_project_cost BOOLEAN NOT NULL DEFAULT FALSE,
    project_cost_entry_id UUID,
    finance_export_line_id UUID,
    reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_model_providers_type_status ON model_providers(provider_type, status);
CREATE INDEX IF NOT EXISTS idx_models_provider_status ON models(provider_id, status);
CREATE INDEX IF NOT EXISTS idx_model_price_versions_model ON model_price_versions(model_id, effective_from DESC);
CREATE INDEX IF NOT EXISTS idx_ai_invocations_project ON ai_invocations(project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ai_invocations_agent ON ai_invocations(agent_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ai_invocations_status ON ai_invocations(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ai_usage_ledger_invocation ON ai_usage_ledger(invocation_id);
CREATE INDEX IF NOT EXISTS idx_ai_usage_ledger_export ON ai_usage_ledger(finance_export_line_id);

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
    result            JSONB NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(result) = 'object'),
    cost_amount       NUMERIC(18,8) NOT NULL DEFAULT 0,
    currency          TEXT NOT NULL DEFAULT 'CNY',
    input_tokens      INT NOT NULL DEFAULT 0,
    output_tokens     INT NOT NULL DEFAULT 0,
    error_message     TEXT NOT NULL DEFAULT '',
    proposal_status   TEXT NOT NULL DEFAULT 'not_submitted'
        CHECK (proposal_status IN ('not_submitted', 'submitting', 'approval_required', 'completed', 'rejected', 'failed', 'denied')),
    tool_execution_id UUID,
    tool_approval_id  UUID,
    proposal_result   JSONB NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(proposal_result) = 'object'),
    proposal_error    TEXT NOT NULL DEFAULT '',
    proposal_submitted_at TIMESTAMPTZ,
    proposal_completed_at TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at      TIMESTAMPTZ,
    retention_redacted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_business_stage_ai_runs_project
    ON business_stage_ai_runs(organization_id, project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_business_stage_ai_runs_stage
    ON business_stage_ai_runs(organization_id, stage, status, created_at DESC);


-- -----------------------------------------------------------------------------

-- Folded from historical migration: 017_tool_runtime.sql
-- -----------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS tool_definitions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    source_type TEXT NOT NULL CHECK (source_type IN ('internal_api', 'interface_file', 'manual_approval')),
    default_policy TEXT NOT NULL DEFAULT 'approve' CHECK (default_policy IN ('auto', 'notify', 'approve', 'deny')),
    risk_level TEXT NOT NULL DEFAULT 'medium' CHECK (risk_level IN ('low', 'medium', 'high', 'critical')),
    required_level TEXT NOT NULL DEFAULT 'L1',
    input_schema JSONB NOT NULL DEFAULT '{}',
    output_schema JSONB NOT NULL DEFAULT '{}',
    metadata JSONB NOT NULL DEFAULT '{}',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS interface_files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    file_type TEXT NOT NULL CHECK (file_type IN ('json', 'yaml', 'markdown')),
    content TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tool_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tool_id UUID NOT NULL REFERENCES tool_definitions(id) ON DELETE RESTRICT,
    invocation_id UUID REFERENCES ai_invocations(id) ON DELETE SET NULL,
    actor_id UUID NOT NULL,
    actor_type TEXT NOT NULL,
    organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
    department_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    project_id UUID,
    workflow_id UUID REFERENCES workflow_instances(id) ON DELETE SET NULL,
    task_id UUID REFERENCES tasks(id) ON DELETE SET NULL,
    idempotency_key TEXT NOT NULL DEFAULT '',
    policy TEXT NOT NULL,
    governance_decision TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'requested' CHECK (status IN ('requested', 'approval_required', 'approved', 'running', 'completed', 'rejected', 'denied', 'failed')),
    arguments JSONB NOT NULL DEFAULT '{}',
    result_summary TEXT NOT NULL DEFAULT '',
    result JSONB NOT NULL DEFAULT '{}',
    error_message TEXT NOT NULL DEFAULT '',
    duration_ms INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    retention_redacted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_tool_execution_idempotency
    ON tool_executions(tool_id, idempotency_key)
    WHERE idempotency_key <> '';

CREATE TABLE IF NOT EXISTS tool_approvals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id UUID NOT NULL REFERENCES tool_executions(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected', 'expired')),
    requested_by UUID REFERENCES users(id) ON DELETE SET NULL,
    reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    reason TEXT NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reviewed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_tool_definitions_active ON tool_definitions(is_active, source_type, name);
CREATE INDEX IF NOT EXISTS idx_tool_executions_status ON tool_executions(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_tool_approvals_status ON tool_approvals(status, created_at DESC);

ALTER TABLE business_stage_ai_runs
    ADD CONSTRAINT fk_business_stage_ai_tool_execution
    FOREIGN KEY (tool_execution_id) REFERENCES tool_executions(id) ON DELETE SET NULL;
ALTER TABLE business_stage_ai_runs
    ADD CONSTRAINT fk_business_stage_ai_tool_approval
    FOREIGN KEY (tool_approval_id) REFERENCES tool_approvals(id) ON DELETE SET NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_business_stage_ai_tool_execution
    ON business_stage_ai_runs(tool_execution_id) WHERE tool_execution_id IS NOT NULL;

INSERT INTO tool_definitions (name, description, source_type, default_policy, risk_level, required_level)
VALUES
    ('requirement.analyze', 'Analyze a requirement', 'internal_api', 'notify', 'medium', 'L2'),
    ('project.match_members', 'Recommend project members', 'internal_api', 'notify', 'medium', 'L2'),
    ('project.bind_workflow', 'Bind workflow to project', 'internal_api', 'approve', 'high', 'L3'),
    ('project.estimate_cost', 'Estimate project cost', 'internal_api', 'notify', 'medium', 'L2'),
    ('project.create_cost_entry', 'Create project cost entry', 'internal_api', 'approve', 'high', 'L3'),
    ('project.update_status', 'Update project lifecycle status', 'internal_api', 'approve', 'high', 'L3'),
    ('project.create_deliverable', 'Create a project deliverable', 'internal_api', 'approve', 'high', 'L3'),
    ('project.accept_deliverable', 'Accept a submitted project deliverable', 'internal_api', 'approve', 'high', 'L3'),
    ('project.close_feedback', 'Close the project feedback loop', 'internal_api', 'approve', 'high', 'L3'),
    ('governance.explain_decision', 'Explain governance decision', 'internal_api', 'notify', 'low', 'L1'),
    ('finance.prepare_export_batch', 'Prepare finance export batch', 'manual_approval', 'approve', 'high', 'L3')
ON CONFLICT (name) DO NOTHING;


-- -----------------------------------------------------------------------------

-- Folded from historical migration: 020_ai_gateway_internal_ops.sql
-- -----------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS model_provider_channels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id UUID NOT NULL REFERENCES model_providers(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    base_url TEXT NOT NULL DEFAULT '',
    encrypted_api_key TEXT NOT NULL,
    masked_api_key TEXT NOT NULL DEFAULT '',
    owner_type TEXT NOT NULL DEFAULT ''
        CHECK (owner_type IN ('', 'human', 'agent', 'team', 'system')),
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    agent_id UUID REFERENCES ai_agents(id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'disabled', 'error', 'quota_exhausted')),
    priority INT NOT NULL DEFAULT 50,
    concurrency_limit INT NOT NULL DEFAULT 0,
    inflight_requests INT NOT NULL DEFAULT 0,
    load_factor INT NOT NULL DEFAULT 1,
    rate_multiplier NUMERIC(12,6) NOT NULL DEFAULT 1,
    quota_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    quota_used NUMERIC(18,8) NOT NULL DEFAULT 0,
    quota_currency TEXT NOT NULL DEFAULT 'CNY',
    supported_model_patterns JSONB NOT NULL DEFAULT '[]',
    model_mapping JSONB NOT NULL DEFAULT '{}',
    health_status TEXT NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT '',
    last_tested_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_model_provider_channels_provider_status
    ON model_provider_channels(provider_id, status, priority);
CREATE INDEX IF NOT EXISTS idx_model_provider_channels_owner
    ON model_provider_channels(owner_type, user_id, agent_id);

ALTER TABLE model_price_versions
    ADD COLUMN IF NOT EXISTS cache_creation_price_per_1k NUMERIC(18,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS cache_read_price_per_1k NUMERIC(18,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS cache_creation_5m_price_per_1k NUMERIC(18,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS cache_creation_1h_price_per_1k NUMERIC(18,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS image_output_price_per_1k NUMERIC(18,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS priority_input_price_per_1k NUMERIC(18,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS priority_output_price_per_1k NUMERIC(18,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS priority_cache_read_price_per_1k NUMERIC(18,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS long_context_threshold INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS long_context_input_multiplier NUMERIC(12,6) NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS long_context_output_multiplier NUMERIC(12,6) NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS billing_mode TEXT NOT NULL DEFAULT 'token',
    ADD COLUMN IF NOT EXISTS pricing_source TEXT NOT NULL DEFAULT 'manual',
    ADD COLUMN IF NOT EXISTS metadata JSONB NOT NULL DEFAULT '{}';

ALTER TABLE ai_invocations
    ADD COLUMN IF NOT EXISTS channel_id UUID REFERENCES model_provider_channels(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS requested_model TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS upstream_model TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS model_mapping_chain TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS service_tier TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS reasoning_effort TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS cache_creation_tokens INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS cache_read_tokens INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS cache_creation_5m_tokens INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS cache_creation_1h_tokens INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS image_output_tokens INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS cost_breakdown JSONB NOT NULL DEFAULT '{}';

ALTER TABLE ai_usage_ledger
    ADD COLUMN IF NOT EXISTS channel_id UUID REFERENCES model_provider_channels(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS cache_creation_tokens INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS cache_read_tokens INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS cache_creation_5m_tokens INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS cache_creation_1h_tokens INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS image_output_tokens INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS input_cost NUMERIC(18,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS output_cost NUMERIC(18,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS cache_creation_cost NUMERIC(18,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS cache_read_cost NUMERIC(18,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS image_output_cost NUMERIC(18,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS rate_multiplier NUMERIC(12,6) NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS actual_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS service_tier TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS reasoning_effort TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS requested_model TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS upstream_model TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS metadata JSONB NOT NULL DEFAULT '{}';

CREATE INDEX IF NOT EXISTS idx_ai_invocations_channel
    ON ai_invocations(channel_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ai_usage_ledger_channel
    ON ai_usage_ledger(channel_id, created_at DESC);

CREATE TABLE IF NOT EXISTS ai_routing_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    provider_id UUID REFERENCES model_providers(id) ON DELETE CASCADE,
    channel_id UUID REFERENCES model_provider_channels(id) ON DELETE CASCADE,
    match_scope TEXT NOT NULL DEFAULT 'global',
    match_value TEXT NOT NULL DEFAULT '',
    model_pattern TEXT NOT NULL DEFAULT '',
    priority INT NOT NULL DEFAULT 100,
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'disabled')),
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ai_routing_rules_status_priority
    ON ai_routing_rules(status, priority);

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
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ai_gateway_balance_transactions_org
    ON ai_gateway_balance_transactions(organization_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ai_gateway_balance_transactions_reservation
    ON ai_gateway_balance_transactions(reservation_id);

ALTER TABLE model_provider_channels
    ADD COLUMN IF NOT EXISTS adapter_key TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS channel_type TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS config JSONB NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS system_prompt TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS balance_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS balance_updated_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_response_ms INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS success_count BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS failure_count BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS auto_disabled_at TIMESTAMPTZ;

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


-- -----------------------------------------------------------------------------

-- Folded from historical migration: 022_assistant_runtime.sql
-- -----------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS assistant_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title TEXT NOT NULL DEFAULT '',
    mode TEXT NOT NULL DEFAULT 'business_process'
        CHECK (mode IN ('business_process', 'self_evolution')),
    module_key TEXT NOT NULL DEFAULT 'general',
    status TEXT NOT NULL DEFAULT 'idle'
        CHECK (status IN ('idle', 'running', 'approval_required', 'completed', 'failed', 'cancelled')),
    actor_id UUID NOT NULL,
    actor_type TEXT NOT NULL,
    provider_id UUID REFERENCES model_providers(id) ON DELETE SET NULL,
    preferred_channel_id UUID REFERENCES model_provider_channels(id) ON DELETE SET NULL,
    provider_type TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    service_tier TEXT NOT NULL DEFAULT '',
    reasoning_effort TEXT NOT NULL DEFAULT '',
    organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
    department_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    position_id UUID REFERENCES positions(id) ON DELETE SET NULL,
    position_assignment_id UUID REFERENCES position_assignments(id) ON DELETE SET NULL,
    project_id UUID,
    workflow_id UUID REFERENCES workflow_instances(id) ON DELETE SET NULL,
    task_id UUID REFERENCES tasks(id) ON DELETE SET NULL,
    working_memory JSONB NOT NULL DEFAULT '{}',
    metadata JSONB NOT NULL DEFAULT '{}',
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    retention_redacted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS assistant_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES assistant_sessions(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('system', 'user', 'assistant', 'tool')),
    content TEXT NOT NULL DEFAULT '',
    tool_call_id TEXT NOT NULL DEFAULT '',
    tool_name TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    retention_redacted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS assistant_steps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES assistant_sessions(id) ON DELETE CASCADE,
    module_key TEXT NOT NULL DEFAULT 'general',
    organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
    department_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    position_id UUID REFERENCES positions(id) ON DELETE SET NULL,
    position_assignment_id UUID REFERENCES position_assignments(id) ON DELETE SET NULL,
    invocation_id UUID REFERENCES ai_invocations(id) ON DELETE SET NULL,
    tool_execution_id UUID REFERENCES tool_executions(id) ON DELETE SET NULL,
    tool_approval_id UUID REFERENCES tool_approvals(id) ON DELETE SET NULL,
    step_type TEXT NOT NULL CHECK (step_type IN ('llm', 'tool_call', 'tool_result', 'memory', 'approval', 'error')),
    status TEXT NOT NULL DEFAULT 'completed',
    summary TEXT NOT NULL DEFAULT '',
    data JSONB NOT NULL DEFAULT '{}',
    turn INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    retention_redacted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS assistant_memories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    module_key TEXT NOT NULL DEFAULT 'general',
    organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
    department_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    position_id UUID REFERENCES positions(id) ON DELETE SET NULL,
    position_assignment_id UUID REFERENCES position_assignments(id) ON DELETE SET NULL,
    actor_id UUID,
    actor_type TEXT NOT NULL DEFAULT '',
    memory_type TEXT NOT NULL DEFAULT 'lesson'
        CHECK (memory_type IN ('working', 'knowledge', 'preference', 'lesson')),
    title TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL DEFAULT '',
    data JSONB NOT NULL DEFAULT '{}',
    source_session_id UUID REFERENCES assistant_sessions(id) ON DELETE SET NULL,
    source_step_id UUID REFERENCES assistant_steps(id) ON DELETE SET NULL,
    confidence DOUBLE PRECISION NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_assistant_sessions_actor
    ON assistant_sessions(actor_id, actor_type, module_key, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_assistant_sessions_scope
    ON assistant_sessions(module_key, organization_id, department_id, position_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_assistant_sessions_status
    ON assistant_sessions(status, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_assistant_messages_session
    ON assistant_messages(session_id, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_assistant_steps_session
    ON assistant_steps(session_id, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_assistant_steps_scope
    ON assistant_steps(module_key, organization_id, department_id, position_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_assistant_memories_scope
    ON assistant_memories(module_key, organization_id, department_id, position_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_assistant_memories_actor
    ON assistant_memories(actor_id, actor_type, module_key, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_ai_invocations_retention
    ON ai_invocations(created_at) WHERE retention_redacted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_business_stage_ai_runs_retention
    ON business_stage_ai_runs(created_at) WHERE retention_redacted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_tool_executions_retention
    ON tool_executions(created_at) WHERE retention_redacted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_assistant_sessions_retention
    ON assistant_sessions(updated_at) WHERE retention_redacted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_assistant_messages_retention
    ON assistant_messages(created_at) WHERE retention_redacted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_assistant_steps_retention
    ON assistant_steps(created_at) WHERE retention_redacted_at IS NULL;

CREATE TABLE IF NOT EXISTS platform.audit_retention_runs (
    id              BIGSERIAL PRIMARY KEY,
    started_at      TIMESTAMPTZ NOT NULL,
    completed_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    cutoff_at       TIMESTAMPTZ NOT NULL,
    retention_days  INT NOT NULL CHECK (retention_days > 0),
    batch_size      INT NOT NULL CHECK (batch_size > 0),
    status          TEXT NOT NULL CHECK (status IN ('succeeded', 'failed')),
    redaction_counts JSONB NOT NULL DEFAULT '{}',
    error_message   TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_audit_retention_runs_completed
    ON platform.audit_retention_runs(completed_at DESC);

INSERT INTO tool_definitions (name, description, source_type, default_policy, risk_level, required_level, input_schema)
VALUES
    (
        'evolution.create_knowledge',
        'Create an evolution knowledge entry from verified assistant work',
        'internal_api',
        'notify',
        'medium',
        'L2',
        '{"type":"object","properties":{"title":{"type":"string"},"content":{"type":"string"},"tags":{"type":"array","items":{"type":"string"}},"workflow_id":{"type":"string"}},"required":["title","content"]}'::jsonb
    ),
    (
        'evolution.create_signal',
        'Create an evolution signal for follow-up review',
        'internal_api',
        'notify',
        'medium',
        'L2',
        '{"type":"object","properties":{"signal_type":{"type":"string"},"source":{"type":"string"},"priority":{"type":"integer"},"data":{"type":"object"}},"required":["signal_type"]}'::jsonb
    ),
    (
        'evolution.propose_experiment',
        'Propose a system evolution experiment',
        'internal_api',
        'approve',
        'high',
        'L3',
        '{"type":"object","properties":{"name":{"type":"string"},"hypothesis":{"type":"string"},"success_criteria":{"type":"object"}},"required":["name","hypothesis"]}'::jsonb
    )
ON CONFLICT (name) DO UPDATE
SET description = EXCLUDED.description,
    source_type = EXCLUDED.source_type,
    default_policy = EXCLUDED.default_policy,
    risk_level = EXCLUDED.risk_level,
    required_level = EXCLUDED.required_level,
    input_schema = EXCLUDED.input_schema,
    updated_at = NOW();


-- -----------------------------------------------------------------------------

-- Folded from historical migration: 029_assistant_business_interaction.sql
-- -----------------------------------------------------------------------------

-- Global business interaction runtime: default model matching, proposals, and internal business skills.

ALTER TABLE assistant_sessions
    ADD COLUMN IF NOT EXISTS agent_id UUID REFERENCES ai_agents(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS target_type TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS target_id UUID;

ALTER TABLE workflow_instances
    ADD COLUMN IF NOT EXISTS metadata JSONB NOT NULL DEFAULT '{}';

ALTER TABLE tasks
    ADD COLUMN IF NOT EXISTS metadata JSONB NOT NULL DEFAULT '{}';

CREATE TABLE IF NOT EXISTS assistant_module_defaults (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    module_key TEXT NOT NULL DEFAULT 'general',
    target_type TEXT NOT NULL DEFAULT '',
    agent_id UUID REFERENCES ai_agents(id) ON DELETE SET NULL,
    provider_id UUID REFERENCES model_providers(id) ON DELETE SET NULL,
    preferred_channel_id UUID REFERENCES model_provider_channels(id) ON DELETE SET NULL,
    provider_type TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    service_tier TEXT NOT NULL DEFAULT '',
    reasoning_effort TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (module_key, target_type)
);

CREATE INDEX IF NOT EXISTS idx_assistant_module_defaults_lookup
    ON assistant_module_defaults(module_key, target_type, updated_at DESC);

CREATE TABLE IF NOT EXISTS assistant_proposals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES assistant_sessions(id) ON DELETE CASCADE,
    module_key TEXT NOT NULL DEFAULT 'general',
    target_type TEXT NOT NULL DEFAULT '',
    target_id UUID,
    proposal_type TEXT NOT NULL DEFAULT 'metadata_patch',
    title TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    payload JSONB NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'applied', 'rejected')),
    reviewer_id UUID REFERENCES users(id) ON DELETE SET NULL,
    review_reason TEXT NOT NULL DEFAULT '',
    apply_result JSONB NOT NULL DEFAULT '{}',
    error_message TEXT NOT NULL DEFAULT '',
    source_step_id UUID REFERENCES assistant_steps(id) ON DELETE SET NULL,
    applied_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_assistant_proposals_session
    ON assistant_proposals(session_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_assistant_proposals_target
    ON assistant_proposals(target_type, target_id, status, created_at DESC);

CREATE TABLE IF NOT EXISTS assistant_business_skills (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    module_key TEXT NOT NULL DEFAULT 'general',
    target_type TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    trigger_intent TEXT NOT NULL DEFAULT '',
    prompt_template TEXT NOT NULL DEFAULT '',
    tool_allowlist JSONB NOT NULL DEFAULT '[]',
    input_schema JSONB NOT NULL DEFAULT '{}',
    output_schema JSONB NOT NULL DEFAULT '{}',
    version INT NOT NULL DEFAULT 1,
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'active', 'archived')),
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_by_type TEXT NOT NULL DEFAULT '',
    reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    source_session_id UUID REFERENCES assistant_sessions(id) ON DELETE SET NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_assistant_business_skills_scope
    ON assistant_business_skills(module_key, target_type, status, updated_at DESC);

CREATE TABLE IF NOT EXISTS assistant_skill_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    skill_id UUID NOT NULL REFERENCES assistant_business_skills(id) ON DELETE CASCADE,
    session_id UUID REFERENCES assistant_sessions(id) ON DELETE SET NULL,
    module_key TEXT NOT NULL DEFAULT 'general',
    target_type TEXT NOT NULL DEFAULT '',
    target_id UUID,
    input JSONB NOT NULL DEFAULT '{}',
    output JSONB NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'completed',
    error_message TEXT NOT NULL DEFAULT '',
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_by_type TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_assistant_skill_runs_skill
    ON assistant_skill_runs(skill_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_assistant_skill_runs_target
    ON assistant_skill_runs(target_type, target_id, created_at DESC);

INSERT INTO assistant_module_defaults (module_key, target_type, agent_id, provider_id, provider_type, model, metadata)
SELECT module_key, target_type, agent_id, provider_id, provider_type, model_key,
    jsonb_build_object('source', 'auto_seed_first_active_model')
FROM (
    SELECT *
    FROM (VALUES
        ('meta_org', ''),
        ('requirement', 'requirement'),
        ('project', 'project'),
        ('delivery', 'deliverable'),
        ('project_cost', 'project_cost'),
        ('feedback', 'project_evaluation'),
        ('workflow', 'workflow_instance'),
        ('finance', 'finance_settlement'),
        ('finance', 'finance_receivable'),
        ('finance', 'finance_payable'),
        ('costing', 'cost_ledger_entry')
    ) AS defaults(module_key, target_type)
) defaults
CROSS JOIN LATERAL (
    SELECT m.provider_id, mp.provider_type, m.model_key
    FROM models m
    JOIN model_providers mp ON mp.id = m.provider_id
    WHERE m.status = 'active' AND mp.status = 'active'
    ORDER BY m.updated_at DESC, m.created_at DESC
    LIMIT 1
) model_defaults
LEFT JOIN LATERAL (
    SELECT id AS agent_id
    FROM ai_agents
    WHERE is_active
    ORDER BY updated_at DESC, created_at DESC
    LIMIT 1
) agent_defaults ON TRUE
ON CONFLICT (module_key, target_type) DO NOTHING;


-- -----------------------------------------------------------------------------

-- Folded from historical migration: 030_assistant_business_interaction_patch.sql
-- -----------------------------------------------------------------------------

-- Backfill fixes for environments that applied an earlier assistant business interaction migration.

WITH agent_defaults AS (
    SELECT id AS agent_id
    FROM ai_agents
    WHERE is_active
    ORDER BY updated_at DESC, created_at DESC
    LIMIT 1
)
UPDATE assistant_module_defaults
SET agent_id = agent_defaults.agent_id,
    updated_at = NOW()
FROM agent_defaults
WHERE assistant_module_defaults.agent_id IS NULL;

INSERT INTO assistant_module_defaults (module_key, target_type, agent_id, provider_id, provider_type, model, metadata)
SELECT module_key, target_type, agent_id, provider_id, provider_type, model_key,
    jsonb_build_object('source', 'patch_seed_first_active_model')
FROM (
    SELECT *
    FROM (VALUES
        ('meta_org', ''),
        ('project_cost', 'project_cost')
    ) AS defaults(module_key, target_type)
) defaults
CROSS JOIN LATERAL (
    SELECT m.provider_id, mp.provider_type, m.model_key
    FROM models m
    JOIN model_providers mp ON mp.id = m.provider_id
    WHERE m.status = 'active' AND mp.status = 'active'
    ORDER BY m.updated_at DESC, m.created_at DESC
    LIMIT 1
) model_defaults
LEFT JOIN LATERAL (
    SELECT id AS agent_id
    FROM ai_agents
    WHERE is_active
    ORDER BY updated_at DESC, created_at DESC
    LIMIT 1
) agent_defaults ON TRUE
ON CONFLICT (module_key, target_type) DO UPDATE
SET agent_id = COALESCE(assistant_module_defaults.agent_id, EXCLUDED.agent_id),
    provider_id = COALESCE(assistant_module_defaults.provider_id, EXCLUDED.provider_id),
    provider_type = COALESCE(NULLIF(assistant_module_defaults.provider_type, ''), EXCLUDED.provider_type),
    model = COALESCE(NULLIF(assistant_module_defaults.model, ''), EXCLUDED.model),
    updated_at = NOW();


-- -----------------------------------------------------------------------------

-- Folded from historical migration: 035_assistant_context_engine.sql
-- -----------------------------------------------------------------------------

ALTER TABLE assistant_steps DROP CONSTRAINT IF EXISTS assistant_steps_step_type_check;
ALTER TABLE assistant_steps
    ADD CONSTRAINT assistant_steps_step_type_check
    CHECK (step_type IN ('llm', 'tool_call', 'tool_result', 'memory', 'approval', 'error', 'context'));

CREATE TABLE IF NOT EXISTS context_dictionary_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_level TEXT NOT NULL CHECK (scope_level IN ('saas', 'organization', 'module')),
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    module_key TEXT NOT NULL DEFAULT '',
    version_key TEXT NOT NULL,
    source_type TEXT NOT NULL CHECK (source_type IN ('json', 'yaml', 'csv', 'xlsx')),
    source_name TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'ai_reviewed', 'approved', 'active', 'rejected', 'archived')),
    checksum TEXT NOT NULL DEFAULT '',
    imported_by UUID REFERENCES users(id) ON DELETE SET NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (scope_level, organization_id, module_key, version_key)
);

CREATE TABLE IF NOT EXISTS context_business_domains (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dictionary_version_id UUID NOT NULL REFERENCES context_dictionary_versions(id) ON DELETE CASCADE,
    module_key TEXT NOT NULL,
    name TEXT NOT NULL,
    scope_level TEXT NOT NULL CHECK (scope_level IN ('saas', 'organization', 'module')),
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'draft',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (dictionary_version_id, module_key)
);

CREATE TABLE IF NOT EXISTS context_entities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dictionary_version_id UUID NOT NULL REFERENCES context_dictionary_versions(id) ON DELETE CASCADE,
    domain_id UUID REFERENCES context_business_domains(id) ON DELETE CASCADE,
    entity_key TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (dictionary_version_id, entity_key)
);

CREATE TABLE IF NOT EXISTS context_fields (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dictionary_version_id UUID NOT NULL REFERENCES context_dictionary_versions(id) ON DELETE CASCADE,
    entity_id UUID NOT NULL REFERENCES context_entities(id) ON DELETE CASCADE,
    field_key TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    data_type TEXT NOT NULL DEFAULT 'string',
    semantic_type TEXT NOT NULL DEFAULT '',
    sensitivity_level TEXT NOT NULL DEFAULT 'normal'
        CHECK (sensitivity_level IN ('public', 'normal', 'sensitive', 'restricted')),
    base_weight DOUBLE PRECISION NOT NULL DEFAULT 1,
    is_finance_field BOOLEAN NOT NULL DEFAULT FALSE,
    is_workflow_field BOOLEAN NOT NULL DEFAULT FALSE,
    is_governance_field BOOLEAN NOT NULL DEFAULT FALSE,
    mask_strategy TEXT NOT NULL DEFAULT 'none',
    status TEXT NOT NULL DEFAULT 'draft',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (entity_id, field_key)
);

CREATE TABLE IF NOT EXISTS context_physical_mappings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dictionary_version_id UUID NOT NULL REFERENCES context_dictionary_versions(id) ON DELETE CASCADE,
    entity_id UUID NOT NULL REFERENCES context_entities(id) ON DELETE CASCADE,
    field_id UUID REFERENCES context_fields(id) ON DELETE CASCADE,
    table_name TEXT NOT NULL,
    column_name TEXT NOT NULL DEFAULT '',
    join_path JSONB NOT NULL DEFAULT '[]',
    tenant_column TEXT NOT NULL DEFAULT 'organization_id',
    status TEXT NOT NULL DEFAULT 'draft',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS context_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dictionary_version_id UUID NOT NULL REFERENCES context_dictionary_versions(id) ON DELETE CASCADE,
    module_key TEXT NOT NULL DEFAULT '',
    entity_key TEXT NOT NULL DEFAULT '',
    field_key TEXT NOT NULL DEFAULT '',
    rule_type TEXT NOT NULL CHECK (rule_type IN ('permission', 'workflow', 'finance', 'governance', 'weight', 'attention')),
    rule JSONB NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'approved', 'active', 'rejected', 'archived')),
    approved_by UUID REFERENCES users(id) ON DELETE SET NULL,
    approved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS context_change_proposals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dictionary_version_id UUID NOT NULL REFERENCES context_dictionary_versions(id) ON DELETE CASCADE,
    proposal_type TEXT NOT NULL DEFAULT 'dictionary_change',
    title TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    payload JSONB NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'approved', 'applied', 'rejected', 'blocked')),
    reviewer_id UUID REFERENCES users(id) ON DELETE SET NULL,
    review_reason TEXT NOT NULL DEFAULT '',
    apply_result JSONB NOT NULL DEFAULT '{}',
    applied_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS context_migration_drafts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dictionary_version_id UUID NOT NULL REFERENCES context_dictionary_versions(id) ON DELETE CASCADE,
    title TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    sql_up TEXT NOT NULL DEFAULT '',
    sql_down TEXT NOT NULL DEFAULT '',
    risk_level TEXT NOT NULL DEFAULT 'medium',
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'reviewed', 'executed', 'rejected')),
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS context_packages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID REFERENCES assistant_sessions(id) ON DELETE SET NULL,
    dictionary_version_id UUID REFERENCES context_dictionary_versions(id) ON DELETE SET NULL,
    actor_id UUID NOT NULL,
    actor_type TEXT NOT NULL DEFAULT '',
    organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
    module_key TEXT NOT NULL DEFAULT '',
    target_type TEXT NOT NULL DEFAULT '',
    target_id UUID,
    workflow_id UUID REFERENCES workflow_instances(id) ON DELETE SET NULL,
    task_id UUID REFERENCES tasks(id) ON DELETE SET NULL,
    attention_core JSONB NOT NULL DEFAULT '[]',
    supporting_context JSONB NOT NULL DEFAULT '[]',
    risk_and_signals JSONB NOT NULL DEFAULT '[]',
    omissions JSONB NOT NULL DEFAULT '[]',
    weights JSONB NOT NULL DEFAULT '{}',
    validations JSONB NOT NULL DEFAULT '{}',
    provenance JSONB NOT NULL DEFAULT '{}',
    token_budget INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_context_dictionary_versions_scope
    ON context_dictionary_versions(scope_level, organization_id, module_key, status, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_context_rules_lookup
    ON context_rules(module_key, entity_key, field_key, rule_type, status);
CREATE INDEX IF NOT EXISTS idx_context_packages_session
    ON context_packages(session_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_context_packages_target
    ON context_packages(module_key, target_type, target_id, created_at DESC);

CREATE TABLE IF NOT EXISTS monitoring_agent_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trigger_type TEXT NOT NULL DEFAULT 'manual'
        CHECK (trigger_type IN ('manual', 'scheduled')),
    organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'running'
        CHECK (status IN ('running', 'completed', 'failed')),
    lookback_started_at TIMESTAMPTZ NOT NULL,
    lookback_ended_at TIMESTAMPTZ NOT NULL,
    signals_created INT NOT NULL DEFAULT 0,
    duplicates_suppressed INT NOT NULL DEFAULT 0,
    summary JSONB NOT NULL DEFAULT '{}',
    error_message TEXT NOT NULL DEFAULT '',
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_monitoring_agent_runs_org
    ON monitoring_agent_runs(organization_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_monitoring_agent_runs_status
    ON monitoring_agent_runs(status, started_at DESC);

WITH seed_version AS (
    INSERT INTO context_dictionary_versions (scope_level, module_key, version_key, source_type, source_name, status, metadata)
    VALUES ('saas', 'assistant_seed', 'assistant-context-seed-v1', 'json', 'migration_035_seed', 'active',
        '{"seed":true,"domains":["project","finance","governance"]}'::jsonb)
    ON CONFLICT (scope_level, organization_id, module_key, version_key) DO UPDATE
    SET status = 'active', updated_at = NOW()
    RETURNING id
),
domains AS (
    INSERT INTO context_business_domains (dictionary_version_id, module_key, name, scope_level, status)
    SELECT id, 'project', 'Project', 'saas', 'active' FROM seed_version
    UNION ALL SELECT id, 'finance', 'Finance', 'saas', 'active' FROM seed_version
    UNION ALL SELECT id, 'governance', 'Governance', 'saas', 'active' FROM seed_version
    ON CONFLICT (dictionary_version_id, module_key) DO NOTHING
    RETURNING id, module_key
),
entities AS (
    INSERT INTO context_entities (dictionary_version_id, domain_id, entity_key, display_name, status)
    SELECT seed_version.id, domains.id, 'project', 'Project', 'active' FROM seed_version, domains WHERE domains.module_key = 'project'
    UNION ALL SELECT seed_version.id, domains.id, 'requirement', 'Requirement', 'active' FROM seed_version, domains WHERE domains.module_key = 'project'
    UNION ALL SELECT seed_version.id, domains.id, 'cost_ledger_entry', 'Cost Ledger Entry', 'active' FROM seed_version, domains WHERE domains.module_key = 'finance'
    UNION ALL SELECT seed_version.id, domains.id, 'access_decision', 'Access Decision', 'active' FROM seed_version, domains WHERE domains.module_key = 'governance'
    ON CONFLICT (dictionary_version_id, entity_key) DO NOTHING
    RETURNING id, entity_key
)
INSERT INTO context_rules (dictionary_version_id, module_key, entity_key, field_key, rule_type, rule, status)
SELECT id, 'project', 'project', 'status', 'attention', '{"base_weight":8,"attention_core":true}'::jsonb, 'active' FROM seed_version
UNION ALL SELECT id, 'project', 'requirement', 'risk_level', 'workflow', '{"stage_multiplier":{"analysis":1.5,"execution":0.8}}'::jsonb, 'active' FROM seed_version
UNION ALL SELECT id, 'erp', 'requirement', 'status', 'workflow', '{"attention_core":true,"table_code":"MREQ","strict_module":true}'::jsonb, 'active' FROM seed_version
UNION ALL SELECT id, 'erp', 'project', 'status', 'workflow', '{"attention_core":true,"table_code":"MPRJ","strict_module":true}'::jsonb, 'active' FROM seed_version
UNION ALL SELECT id, 'erp', 'purchase_order', 'status', 'workflow', '{"attention_core":true,"table_code":"MPOR","strict_module":true}'::jsonb, 'active' FROM seed_version
UNION ALL SELECT id, 'erp', 'goods_receipt_po', 'status', 'workflow', '{"attention_core":true,"table_code":"MPDN","strict_module":true}'::jsonb, 'active' FROM seed_version
UNION ALL SELECT id, 'erp', 'sales_order', 'status', 'workflow', '{"attention_core":true,"table_code":"MRDR","strict_module":true}'::jsonb, 'active' FROM seed_version
UNION ALL SELECT id, 'erp', 'delivery', 'status', 'workflow', '{"attention_core":true,"table_code":"MDLN","strict_module":true}'::jsonb, 'active' FROM seed_version
UNION ALL SELECT id, 'erp', 'ar_invoice', 'status', 'finance', '{"requires_validation":true,"table_code":"MINV","strict_module":true}'::jsonb, 'active' FROM seed_version
UNION ALL SELECT id, 'erp', 'ap_invoice', 'status', 'finance', '{"requires_validation":true,"table_code":"MRCT","strict_module":true}'::jsonb, 'active' FROM seed_version
UNION ALL SELECT id, 'erp', 'goods_receipt', 'status', 'workflow', '{"attention_core":true,"table_code":"MIGN","strict_module":true}'::jsonb, 'active' FROM seed_version
UNION ALL SELECT id, 'erp', 'goods_issue', 'status', 'workflow', '{"attention_core":true,"table_code":"MIGE","strict_module":true}'::jsonb, 'active' FROM seed_version
UNION ALL SELECT id, 'erp', 'journal_entry', 'status', 'finance', '{"requires_validation":true,"table_code":"MJDT","strict_module":true}'::jsonb, 'active' FROM seed_version
UNION ALL SELECT id, 'finance', 'cost_ledger_entry', 'amount', 'finance', '{"requires_validation":true,"unverified_as_signal":true}'::jsonb, 'active' FROM seed_version
UNION ALL SELECT id, 'governance', 'access_decision', 'decision', 'permission', '{"sensitivity":"restricted","explicit_rule_required":true}'::jsonb, 'active' FROM seed_version;

WITH seed_version AS (
    INSERT INTO context_dictionary_versions(scope_level, module_key, version_key, source_type, source_name, status, metadata)
    VALUES ('saas', 'supply_chain', 'supply-chain-v1', 'json', 'migration_038_supply_chain_core', 'active', '{"modules":["procurement","sales","inventory"]}'::jsonb)
    ON CONFLICT (scope_level, organization_id, module_key, version_key) DO UPDATE
    SET status = 'active', updated_at = NOW()
    RETURNING id
),
domains AS (
    INSERT INTO context_business_domains(dictionary_version_id, module_key, name, scope_level, status, metadata)
    SELECT id, 'procurement', 'Procurement', 'saas', 'active', '{"supply_chain":true}'::jsonb FROM seed_version
    UNION ALL SELECT id, 'sales', 'Sales', 'saas', 'active', '{"supply_chain":true}'::jsonb FROM seed_version
    UNION ALL SELECT id, 'inventory', 'Inventory', 'saas', 'active', '{"supply_chain":true}'::jsonb FROM seed_version
    ON CONFLICT (dictionary_version_id, module_key) DO UPDATE SET status = 'active'
    RETURNING id, dictionary_version_id, module_key
),
entities AS (
    INSERT INTO context_entities(dictionary_version_id, domain_id, entity_key, display_name, description, status, metadata)
    SELECT dictionary_version_id, id, 'item', 'Item', 'Tradable material or service master data', 'active', '{"table_name":"items"}'::jsonb FROM domains WHERE module_key = 'inventory'
    UNION ALL SELECT dictionary_version_id, id, 'warehouse', 'Warehouse', 'Inventory warehouse', 'active', '{"table_name":"warehouses"}'::jsonb FROM domains WHERE module_key = 'inventory'
    UNION ALL SELECT dictionary_version_id, id, 'inventory_balance', 'Inventory Balance', 'On-hand inventory balance and valuation', 'active', '{"table_name":"inventory_balances"}'::jsonb FROM domains WHERE module_key = 'inventory'
    UNION ALL SELECT dictionary_version_id, id, 'purchase_receipt', 'Purchase Receipt', 'Inbound receipt that can create inventory and payable entries', 'active', '{"table_name":"purchase_receipts"}'::jsonb FROM domains WHERE module_key = 'procurement'
    UNION ALL SELECT dictionary_version_id, id, 'sales_shipment', 'Sales Shipment', 'Outbound shipment that can create inventory and receivable entries', 'active', '{"table_name":"sales_shipments"}'::jsonb FROM domains WHERE module_key = 'sales'
    ON CONFLICT (dictionary_version_id, entity_key) DO UPDATE
    SET status = 'active', display_name = EXCLUDED.display_name, description = EXCLUDED.description
    RETURNING id, dictionary_version_id, entity_key
),
fields AS (
    INSERT INTO context_fields(dictionary_version_id, entity_id, field_key, display_name, data_type, semantic_type, sensitivity_level, base_weight, is_finance_field, is_workflow_field, is_governance_field, mask_strategy, status, metadata)
    SELECT dictionary_version_id, id, 'quantity', 'Quantity', 'number', 'inventory_quantity', 'normal', 8, false, false, false, 'none', 'active', '{"supply_chain":true}'::jsonb FROM entities WHERE entity_key = 'inventory_balance'
    UNION ALL SELECT dictionary_version_id, id, 'average_cost', 'Average Cost', 'number', 'valuation', 'sensitive', 7, true, false, false, 'summary', 'active', '{"supply_chain":true}'::jsonb FROM entities WHERE entity_key = 'inventory_balance'
    UNION ALL SELECT dictionary_version_id, id, 'status', 'Status', 'string', 'document_status', 'normal', 8, false, true, true, 'none', 'active', '{"supply_chain":true}'::jsonb FROM entities WHERE entity_key IN ('purchase_receipt', 'sales_shipment')
    UNION ALL SELECT dictionary_version_id, id, 'total_amount', 'Total Amount', 'number', 'document_amount', 'sensitive', 8, true, false, false, 'summary', 'active', '{"supply_chain":true}'::jsonb FROM entities WHERE entity_key IN ('purchase_receipt', 'sales_shipment')
    ON CONFLICT (entity_id, field_key) DO UPDATE
    SET status = 'active', display_name = EXCLUDED.display_name, sensitivity_level = EXCLUDED.sensitivity_level, base_weight = EXCLUDED.base_weight
    RETURNING id, dictionary_version_id, entity_id, field_key
)
INSERT INTO context_physical_mappings(dictionary_version_id, entity_id, field_id, table_name, column_name, tenant_column, status, metadata)
SELECT f.dictionary_version_id, f.entity_id, f.id,
       CASE e.entity_key
           WHEN 'inventory_balance' THEN 'inventory_balances'
           WHEN 'purchase_receipt' THEN 'purchase_receipts'
           WHEN 'sales_shipment' THEN 'sales_shipments'
           ELSE e.entity_key || 's'
       END,
       f.field_key,
       'organization_id',
       'active',
       '{"supply_chain":true}'::jsonb
FROM fields f
JOIN entities e ON e.id = f.entity_id
WHERE NOT EXISTS (
    SELECT 1
    FROM context_physical_mappings existing
    WHERE existing.field_id = f.id
      AND existing.table_name = CASE e.entity_key
          WHEN 'inventory_balance' THEN 'inventory_balances'
          WHEN 'purchase_receipt' THEN 'purchase_receipts'
          WHEN 'sales_shipment' THEN 'sales_shipments'
          ELSE e.entity_key || 's'
      END
      AND existing.column_name = f.field_key
);


-- -----------------------------------------------------------------------------

-- Folded from historical migration: 036_unified_skill.sql
-- -----------------------------------------------------------------------------

-- 036_unified_skill.sql
-- Unified skill table: first-class skill governance while preserving existing assistant skill APIs.

CREATE TABLE IF NOT EXISTS skill (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    skill_key TEXT NOT NULL,
    scope_level TEXT NOT NULL DEFAULT 'saas_global'
        CHECK (scope_level IN ('saas_global', 'organization', 'deployment')),
    deployment_mode TEXT NOT NULL DEFAULT 'saas'
        CHECK (deployment_mode IN ('saas', 'org_private', 'private')),
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    owner_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    module_key TEXT NOT NULL DEFAULT 'general',
    target_type TEXT NOT NULL DEFAULT '',
    business_function_key TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    trigger_intent TEXT NOT NULL DEFAULT '',
    prompt_template TEXT NOT NULL DEFAULT '',
    tool_allowlist JSONB NOT NULL DEFAULT '[]',
    input_schema JSONB NOT NULL DEFAULT '{}',
    output_schema JSONB NOT NULL DEFAULT '{}',
    skill_components JSONB NOT NULL DEFAULT '[]',
    permission_policy JSONB NOT NULL DEFAULT '{}',
    context_policy JSONB NOT NULL DEFAULT '{}',
    pricing_policy JSONB NOT NULL DEFAULT '{}',
    activation_policy JSONB NOT NULL DEFAULT '{}',
    version INT NOT NULL DEFAULT 1,
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'active', 'archived')),
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_by_type TEXT NOT NULL DEFAULT '',
    reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    source_session_id UUID REFERENCES assistant_sessions(id) ON DELETE SET NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (jsonb_typeof(skill_components) = 'array'),
    CHECK (jsonb_array_length(skill_components) BETWEEN 3 AND 9)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_skill_scope_key_version
    ON skill(scope_level, COALESCE(organization_id, '00000000-0000-0000-0000-000000000000'::uuid), skill_key, version);
CREATE INDEX IF NOT EXISTS idx_skill_scope_lookup
    ON skill(scope_level, organization_id, module_key, target_type, status, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_skill_business_function
    ON skill(module_key, business_function_key, status);

INSERT INTO skill (
    id, skill_key, scope_level, deployment_mode, organization_id, owner_user_id, module_key, target_type,
    business_function_key, name, description, trigger_intent, prompt_template, tool_allowlist,
    input_schema, output_schema, skill_components, permission_policy, context_policy, pricing_policy,
    activation_policy, version, status, created_by, created_by_type, reviewed_by, source_session_id,
    metadata, created_at, updated_at
)
SELECT
    abs.id,
    lower(regexp_replace(NULLIF(abs.name, ''), '[^a-zA-Z0-9]+', '_', 'g')) || '_' || left(abs.id::text, 8),
    'saas_global',
    'saas',
    NULL,
    abs.created_by,
    COALESCE(NULLIF(abs.module_key, ''), 'general'),
    COALESCE(abs.target_type, ''),
    COALESCE(NULLIF(abs.target_type, ''), COALESCE(NULLIF(abs.module_key, ''), 'general')),
    abs.name,
    COALESCE(abs.description, ''),
    COALESCE(abs.trigger_intent, ''),
    COALESCE(abs.prompt_template, ''),
    COALESCE(abs.tool_allowlist, '[]'::jsonb),
    COALESCE(abs.input_schema, '{}'::jsonb),
    COALESCE(abs.output_schema, '{}'::jsonb),
    jsonb_build_array(
        jsonb_build_object('key', 'intent', 'label', jsonb_build_object('zh', '意图', 'en', 'Intent'), 'weight', 0.3, 'instruction', COALESCE(abs.trigger_intent, 'Clarify the requested skill intent'), 'required_context', '[]'::jsonb, 'permission_tags', '[]'::jsonb),
        jsonb_build_object('key', 'context', 'label', jsonb_build_object('zh', '上下文', 'en', 'Context'), 'weight', 0.4, 'instruction', 'Collect governed business context through context rules', 'required_context', jsonb_build_array(COALESCE(NULLIF(abs.target_type, ''), 'target')), 'permission_tags', '[]'::jsonb),
        jsonb_build_object('key', 'action', 'label', jsonb_build_object('zh', '动作', 'en', 'Action'), 'weight', 0.3, 'instruction', COALESCE(abs.prompt_template, 'Execute the skill prompt'), 'required_context', '[]'::jsonb, 'permission_tags', COALESCE(abs.tool_allowlist, '[]'::jsonb))
    ),
    jsonb_build_object('source', 'legacy_assistant_business_skills', 'field_permission_catalog', true),
    jsonb_build_object('source', 'legacy_assistant_business_skills', 'context_engine_required', true),
    '{}'::jsonb,
    jsonb_build_object('saas_global', 'platform_admin', 'organization', 'organization_admin', 'deployment', 'deployment_admin'),
    COALESCE(abs.version, 1),
    COALESCE(NULLIF(abs.status, ''), 'draft'),
    abs.created_by,
    COALESCE(abs.created_by_type, ''),
    abs.reviewed_by,
    abs.source_session_id,
    COALESCE(abs.metadata, '{}'::jsonb) || jsonb_build_object('migrated_from', 'assistant_business_skills'),
    abs.created_at,
    abs.updated_at
FROM assistant_business_skills abs
ON CONFLICT (id) DO NOTHING;

DO $$
DECLARE
    constraint_name TEXT;
BEGIN
    SELECT conname INTO constraint_name
    FROM pg_constraint
    WHERE conrelid = 'assistant_skill_runs'::regclass
      AND contype = 'f'
      AND pg_get_constraintdef(oid) LIKE '%skill_id%';

    IF constraint_name IS NOT NULL THEN
        EXECUTE FORMAT('ALTER TABLE assistant_skill_runs DROP CONSTRAINT %I', constraint_name);
    END IF;
END $$;

ALTER TABLE assistant_skill_runs
    ADD CONSTRAINT assistant_skill_runs_skill_id_fkey
    FOREIGN KEY (skill_id) REFERENCES skill(id) ON DELETE CASCADE;

INSERT INTO data_table_catalog(
    table_name, master_table_name, detail_table_name, key_prefix, display_name,
    category, is_base_data, is_business_scenario, metadata
)
VALUES (
    'skill', 'skill_masters', 'skill_details', 'SKL', 'Skill',
    'ai', false, true, '{"unified_skill_table":true}'::jsonb
)
ON CONFLICT (table_name) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    category = EXCLUDED.category,
    is_base_data = EXCLUDED.is_base_data,
    is_business_scenario = EXCLUDED.is_business_scenario,
    metadata = data_table_catalog.metadata || EXCLUDED.metadata,
    updated_at = NOW();

ALTER TABLE skill ADD COLUMN IF NOT EXISTS legacy_id UUID;
UPDATE skill SET legacy_id = id WHERE legacy_id IS NULL;
ALTER TABLE skill ADD COLUMN IF NOT EXISTS master_key TEXT;
ALTER TABLE skill ADD COLUMN IF NOT EXISTS parent_master_table TEXT;
ALTER TABLE skill ADD COLUMN IF NOT EXISTS parent_master_key TEXT;
UPDATE skill SET master_key = next_business_key('skill', 'SKL') WHERE master_key IS NULL;
ALTER TABLE skill ALTER COLUMN master_key SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_skill_master_key ON skill(master_key);
CREATE INDEX IF NOT EXISTS idx_skill_parent_master ON skill(parent_master_table, parent_master_key);

CREATE TABLE IF NOT EXISTS skill_details (
    sub_key TEXT PRIMARY KEY DEFAULT next_business_key('skill_details', 'SKLD'),
    master_key TEXT NOT NULL REFERENCES skill(master_key) ON DELETE CASCADE,
    parent_master_table TEXT,
    parent_master_key TEXT,
    detail_type TEXT NOT NULL DEFAULT 'field',
    line_no INT NOT NULL DEFAULT 0,
    field_key TEXT NOT NULL DEFAULT '',
    field_value JSONB NOT NULL DEFAULT 'null'::jsonb,
    payload JSONB NOT NULL DEFAULT '{}',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_skill_details_master ON skill_details(master_key, line_no);

INSERT INTO data_field_catalog(table_name, field_name, data_type, display_name, is_master_key, is_visible_default, permission_level, display_order, metadata)
VALUES
    ('skill', 'id', 'uuid', 'ID', false, true, 'L1', 10, '{}'),
    ('skill', 'legacy_id', 'uuid', 'Legacy ID', false, false, 'L2', 15, '{}'),
    ('skill', 'master_key', 'text', 'Master Key', true, true, 'L1', 18, '{}'),
    ('skill', 'parent_master_table', 'text', 'Parent Master Table', false, false, 'L2', 19, '{}'),
    ('skill', 'parent_master_key', 'text', 'Parent Master Key', false, false, 'L2', 20, '{}'),
    ('skill', 'skill_key', 'text', 'Skill Key', false, true, 'L1', 30, '{}'),
    ('skill', 'scope_level', 'text', 'Scope Level', false, true, 'L2', 40, '{}'),
    ('skill', 'deployment_mode', 'text', 'Deployment Mode', false, true, 'L2', 50, '{}'),
    ('skill', 'organization_id', 'uuid', 'Organization ID', false, true, 'L2', 60, '{}'),
    ('skill', 'owner_user_id', 'uuid', 'Owner User ID', false, true, 'L2', 70, '{}'),
    ('skill', 'module_key', 'text', 'Module Key', false, true, 'L1', 80, '{}'),
    ('skill', 'target_type', 'text', 'Target Type', false, true, 'L1', 90, '{}'),
    ('skill', 'business_function_key', 'text', 'Business Function Key', false, true, 'L1', 100, '{}'),
    ('skill', 'name', 'text', 'Name', false, true, 'L1', 110, '{}'),
    ('skill', 'description', 'text', 'Description', false, true, 'L1', 120, '{}'),
    ('skill', 'trigger_intent', 'text', 'Trigger Intent', false, true, 'L2', 130, '{}'),
    ('skill', 'prompt_template', 'text', 'Prompt Template', false, false, 'L3', 140, '{"sensitive":true}'),
    ('skill', 'tool_allowlist', 'jsonb', 'Tool Allowlist', false, true, 'L3', 150, '{}'),
    ('skill', 'input_schema', 'jsonb', 'Input Schema', false, true, 'L2', 160, '{}'),
    ('skill', 'output_schema', 'jsonb', 'Output Schema', false, true, 'L2', 170, '{}'),
    ('skill', 'skill_components', 'jsonb', 'Skill Components', false, true, 'L2', 180, '{"component_count":"3-9"}'),
    ('skill', 'permission_policy', 'jsonb', 'Permission Policy', false, false, 'L3', 190, '{"sensitive":true}'),
    ('skill', 'context_policy', 'jsonb', 'Context Policy', false, false, 'L3', 200, '{"sensitive":true}'),
    ('skill', 'pricing_policy', 'jsonb', 'Pricing Policy', false, false, 'L3', 210, '{"sensitive":true}'),
    ('skill', 'activation_policy', 'jsonb', 'Activation Policy', false, false, 'L3', 220, '{"sensitive":true}'),
    ('skill', 'version', 'integer', 'Version', false, true, 'L1', 230, '{}'),
    ('skill', 'status', 'text', 'Status', false, true, 'L1', 240, '{}'),
    ('skill', 'metadata', 'jsonb', 'Metadata', false, false, 'L3', 250, '{"sensitive":true}')
ON CONFLICT (table_name, field_name) DO UPDATE SET
    data_type = EXCLUDED.data_type,
    display_name = EXCLUDED.display_name,
    is_master_key = EXCLUDED.is_master_key,
    is_visible_default = EXCLUDED.is_visible_default,
    permission_level = EXCLUDED.permission_level,
    display_order = EXCLUDED.display_order,
    metadata = data_field_catalog.metadata || EXCLUDED.metadata,
    updated_at = NOW();

INSERT INTO field_permission_rules(table_name, field_name, action, behavior, required_level, reason, metadata)
VALUES
    ('skill', 'prompt_template', 'read', 'approve', 'L3', 'skill prompt controls model behavior and requires governed access', '{"unified_skill":true}'::jsonb),
    ('skill', 'permission_policy', 'read', 'approve', 'L3', 'skill permission policy is sensitive governance data', '{"unified_skill":true}'::jsonb),
    ('skill', 'context_policy', 'read', 'approve', 'L3', 'skill context policy controls business context admission', '{"unified_skill":true}'::jsonb),
    ('skill', 'pricing_policy', 'read', 'approve', 'L3', 'skill pricing policy affects commercial behavior', '{"unified_skill":true}'::jsonb),
    ('skill', 'activation_policy', 'write', 'approve', 'L3', 'skill activation policy changes approval authority', '{"unified_skill":true}'::jsonb),
    ('skill', 'metadata', 'read', 'approve', 'L3', 'skill metadata can contain imported governance details', '{"unified_skill":true}'::jsonb);

WITH skill_dictionary AS (
    INSERT INTO context_dictionary_versions(scope_level, module_key, version_key, source_type, source_name, status, metadata)
    VALUES ('saas', 'assistant', 'unified-skill-v1', 'json', 'migration_036_unified_skill', 'active', '{"entity":"skill"}'::jsonb)
    ON CONFLICT (scope_level, organization_id, module_key, version_key) DO UPDATE
    SET status = 'active', updated_at = NOW()
    RETURNING id
),
skill_domain AS (
    INSERT INTO context_business_domains(dictionary_version_id, module_key, name, scope_level, status, metadata)
    SELECT id, 'assistant', 'Assistant Skill', 'saas', 'active', '{"unified_skill":true}'::jsonb
    FROM skill_dictionary
    ON CONFLICT (dictionary_version_id, module_key) DO UPDATE
    SET status = 'active'
    RETURNING id, dictionary_version_id
),
skill_entity AS (
    INSERT INTO context_entities(dictionary_version_id, domain_id, entity_key, display_name, description, status, metadata)
    SELECT dictionary_version_id, id, 'skill', 'Skill', 'Unified governed skill table', 'active', '{"table_name":"skill"}'::jsonb
    FROM skill_domain
    ON CONFLICT (dictionary_version_id, entity_key) DO UPDATE
    SET status = 'active', display_name = EXCLUDED.display_name, description = EXCLUDED.description
    RETURNING id, dictionary_version_id
),
skill_fields AS (
    INSERT INTO context_fields(dictionary_version_id, entity_id, field_key, display_name, data_type, semantic_type, sensitivity_level, base_weight, is_governance_field, mask_strategy, status, metadata)
    SELECT dictionary_version_id, id, 'skill_components', 'Skill Components', 'jsonb', 'skill_component_weights', 'normal', 9, true, 'none', 'active', '{"component_count":"3-9"}'::jsonb FROM skill_entity
    UNION ALL SELECT dictionary_version_id, id, 'permission_policy', 'Permission Policy', 'jsonb', 'permission_policy', 'restricted', 8, true, 'summary', 'active', '{}'::jsonb FROM skill_entity
    UNION ALL SELECT dictionary_version_id, id, 'context_policy', 'Context Policy', 'jsonb', 'context_policy', 'restricted', 8, true, 'summary', 'active', '{}'::jsonb FROM skill_entity
    UNION ALL SELECT dictionary_version_id, id, 'pricing_policy', 'Pricing Policy', 'jsonb', 'pricing_policy', 'sensitive', 6, false, 'summary', 'active', '{}'::jsonb FROM skill_entity
    ON CONFLICT (entity_id, field_key) DO UPDATE
    SET status = 'active', display_name = EXCLUDED.display_name, sensitivity_level = EXCLUDED.sensitivity_level, base_weight = EXCLUDED.base_weight
    RETURNING id, dictionary_version_id, entity_id, field_key
)
INSERT INTO context_physical_mappings(dictionary_version_id, entity_id, field_id, table_name, column_name, tenant_column, status, metadata)
SELECT dictionary_version_id, entity_id, id, 'skill', field_key, 'organization_id', 'active', '{"unified_skill":true}'::jsonb
FROM skill_fields;


-- -----------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS skill_publication_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_skill_id UUID NOT NULL REFERENCES skill(id) ON DELETE CASCADE,
    source_organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
    requested_by UUID REFERENCES users(id) ON DELETE SET NULL,
    reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    target_scope_level TEXT NOT NULL DEFAULT 'saas_global'
        CHECK (target_scope_level IN ('saas_global', 'organization', 'deployment')),
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'approved', 'rejected', 'published', 'cancelled')),
    request_payload JSONB NOT NULL DEFAULT '{}',
    review_payload JSONB NOT NULL DEFAULT '{}',
    published_skill_id UUID REFERENCES skill(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reviewed_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_skill_publication_requests_org
    ON skill_publication_requests(source_organization_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_skill_publication_requests_skill
    ON skill_publication_requests(source_skill_id, status);

INSERT INTO module_master_source_catalog(module_name, source_table, entity_type, relation_mode, parent_table, parent_fk, key_prefix)
VALUES
    ('identity', 'ai_agents', 'ai_agent', 'master', NULL, NULL, 'AGT'),
    ('identity', 'agent_roles', 'agent_role', 'detail', 'ai_agents', 'agent_id', 'ARO'),
    ('aigateway', 'model_providers', 'model_provider', 'master', NULL, NULL, 'AIP'),
    ('aigateway', 'models', 'model', 'detail', 'model_providers', 'provider_id', 'AIM'),
    ('aigateway', 'model_price_versions', 'model_price_version', 'detail', 'models', 'model_id', 'MPV'),
    ('aigateway', 'ai_invocations', 'ai_invocation', 'detail', 'models', 'model_id', 'AIN'),
    ('aigateway', 'ai_usage_ledger', 'ai_usage_ledger', 'detail', 'ai_invocations', 'invocation_id', 'AUL'),
    ('aigateway', 'model_provider_channels', 'model_provider_channel', 'detail', 'model_providers', 'provider_id', 'MPC'),
    ('aigateway', 'ai_routing_rules', 'ai_routing_rule', 'detail', 'model_providers', 'provider_id', 'ARR'),
    ('aigateway', 'ai_model_groups', 'ai_model_group', 'master', NULL, NULL, 'AMG'),
    ('aigateway', 'ai_model_channel_abilities', 'ai_model_channel_ability', 'detail', 'ai_model_groups', 'model_group_id', 'AMA'),
    ('aigateway', 'ai_access_tokens', 'ai_access_token', 'detail', 'organizations', 'organization_id', 'AAT'),
    ('aigateway', 'ai_gateway_balances', 'ai_gateway_balance', 'detail', 'organizations', 'organization_id', 'AGB'),
    ('aigateway', 'ai_gateway_balance_transactions', 'ai_gateway_balance_transaction', 'detail', 'ai_gateway_balances', 'organization_id', 'GBT'),
    ('toolruntime', 'tool_definitions', 'tool_definition', 'master', NULL, NULL, 'TLD'),
    ('toolruntime', 'interface_files', 'interface_file', 'master', NULL, NULL, 'IFL'),
    ('toolruntime', 'tool_executions', 'tool_execution', 'detail', 'tool_definitions', 'tool_id', 'TEX'),
    ('toolruntime', 'tool_approvals', 'tool_approval', 'detail', 'tool_executions', 'execution_id', 'TAP'),
    ('assistant', 'assistant_sessions', 'assistant_session', 'master', NULL, NULL, 'ASN'),
    ('assistant', 'assistant_messages', 'assistant_message', 'detail', 'assistant_sessions', 'session_id', 'AMG'),
    ('assistant', 'assistant_steps', 'assistant_step', 'detail', 'assistant_sessions', 'session_id', 'AST'),
    ('assistant', 'assistant_memories', 'assistant_memory', 'master', NULL, NULL, 'AMR'),
    ('assistant', 'assistant_module_defaults', 'assistant_module_default', 'master', NULL, NULL, 'AMD'),
    ('assistant', 'assistant_proposals', 'assistant_proposal', 'detail', 'assistant_sessions', 'session_id', 'APR'),
    ('assistant', 'assistant_business_skills', 'assistant_business_skill', 'master', NULL, NULL, 'ABS'),
    ('assistant', 'assistant_skill_runs', 'assistant_skill_run', 'detail', 'assistant_business_skills', 'skill_id', 'ASR'),
    ('assistant', 'skill', 'skill', 'master', NULL, NULL, 'SKL'),
    ('assistant', 'skill_publication_requests', 'skill_publication_request', 'detail', 'skill', 'source_skill_id', 'SPR')
ON CONFLICT (source_table) DO UPDATE SET
    module_name = EXCLUDED.module_name,
    entity_type = EXCLUDED.entity_type,
    relation_mode = EXCLUDED.relation_mode,
    parent_table = EXCLUDED.parent_table,
    parent_fk = EXCLUDED.parent_fk,
    key_prefix = EXCLUDED.key_prefix,
    updated_at = NOW();

DO $$
DECLARE
    rec RECORD;
BEGIN
    FOR rec IN
        SELECT module_name, MIN(key_prefix) AS key_prefix
        FROM module_master_source_catalog
        WHERE module_name IN ('identity', 'aigateway', 'toolruntime', 'assistant')
        GROUP BY module_name
        ORDER BY module_name
    LOOP
        PERFORM ensure_module_master_detail_tables(rec.module_name, rec.key_prefix);
    END LOOP;

    FOR rec IN
        SELECT source_table, key_prefix
        FROM module_master_source_catalog
        WHERE module_name IN ('identity', 'aigateway', 'toolruntime', 'assistant')
          AND source_table <> 'skill_details'
          AND module_table_exists(source_table)
        ORDER BY source_table
    LOOP
        PERFORM ensure_source_master_key(rec.source_table, rec.key_prefix);
    END LOOP;
END;
$$;

DO $$
DECLARE
    rec RECORD;
BEGIN
    FOR rec IN
        SELECT source_table
        FROM module_master_source_catalog
        WHERE module_name IN ('identity', 'aigateway', 'toolruntime', 'assistant')
          AND source_table <> 'skill_details'
          AND module_table_exists(source_table)
        ORDER BY relation_mode, source_table
    LOOP
        PERFORM refresh_module_source(rec.source_table);
    END LOOP;
END;
$$;

DO $$
DECLARE
    rec RECORD;
    v_trigger_name TEXT;
BEGIN
    FOR rec IN
        SELECT source_table
        FROM module_master_source_catalog
        WHERE module_name IN ('identity', 'aigateway', 'toolruntime', 'assistant')
          AND source_table <> 'skill_details'
          AND module_table_exists(source_table)
        ORDER BY source_table
    LOOP
        v_trigger_name := 'trg_refresh_' || rec.source_table || '_module_master';
        EXECUTE FORMAT('DROP TRIGGER IF EXISTS %I ON %I', v_trigger_name, rec.source_table);
        EXECUTE FORMAT(
            'CREATE TRIGGER %I
             AFTER INSERT OR UPDATE OR DELETE ON %I
             FOR EACH STATEMENT EXECUTE FUNCTION refresh_module_source_trigger()',
            v_trigger_name,
            rec.source_table
        );
    END LOOP;
END;
$$;

-- AI capability fragments moved from earlier platform/governance stages.
ALTER TABLE ai_agents
    ADD COLUMN IF NOT EXISTS agent_origin TEXT NOT NULL DEFAULT 'internal'
        CHECK (agent_origin IN ('internal', 'external')),
    ADD COLUMN IF NOT EXISTS provider TEXT,
    ADD COLUMN IF NOT EXISTS service_class TEXT NOT NULL DEFAULT 'model',
    ADD COLUMN IF NOT EXISTS vendor TEXT,
    ADD COLUMN IF NOT EXISTS contract_ref TEXT,
    ADD COLUMN IF NOT EXISTS risk_level TEXT NOT NULL DEFAULT 'medium'
        CHECK (risk_level IN ('low', 'medium', 'high', 'critical'));

CREATE OR REPLACE FUNCTION assert_no_duplicate_key(check_name TEXT, duplicate_query TEXT)
RETURNS VOID AS $$
DECLARE
    duplicate_keys TEXT;
BEGIN
    EXECUTE duplicate_query INTO duplicate_keys;
    IF duplicate_keys IS NOT NULL AND duplicate_keys <> '' THEN
        RAISE EXCEPTION 'duplicate base data found for %: %', check_name, duplicate_keys;
    END IF;
END;
$$ LANGUAGE plpgsql;

SELECT assert_no_duplicate_key('model_providers.provider_type.name', $$
    SELECT string_agg(key, '; ') FROM (
        SELECT provider_type || ':' || lower(btrim(name)) AS key
        FROM model_providers
        GROUP BY provider_type, lower(btrim(name))
        HAVING COUNT(*) > 1
    ) d
$$);

SELECT assert_no_duplicate_key('models.provider_id.model_key', $$
    SELECT string_agg(key, '; ') FROM (
        SELECT provider_id::text || ':' || lower(btrim(model_key)) AS key
        FROM models
        GROUP BY provider_id, lower(btrim(model_key))
        HAVING COUNT(*) > 1
    ) d
$$);

SELECT assert_no_duplicate_key('model_provider_channels.provider_id.name', $$
    SELECT string_agg(key, '; ') FROM (
        SELECT provider_id::text || ':' || lower(btrim(name)) AS key
        FROM model_provider_channels
        GROUP BY provider_id, lower(btrim(name))
        HAVING COUNT(*) > 1
    ) d
$$);

DROP FUNCTION assert_no_duplicate_key(TEXT, TEXT);

CREATE UNIQUE INDEX IF NOT EXISTS uq_model_providers_type_name_ci
    ON model_providers (provider_type, lower(btrim(name)));

CREATE UNIQUE INDEX IF NOT EXISTS uq_models_provider_key_ci
    ON models (provider_id, lower(btrim(model_key)));

CREATE UNIQUE INDEX IF NOT EXISTS uq_model_provider_channels_provider_name_ci
    ON model_provider_channels (provider_id, lower(btrim(name)));

ALTER TABLE tool_definitions
    ADD COLUMN IF NOT EXISTS tool_category TEXT NOT NULL DEFAULT 'execution_operation'
        CHECK (tool_category IN ('core_data', 'business_approval', 'execution_operation')),
    ADD COLUMN IF NOT EXISTS approval_tier_required TEXT NOT NULL DEFAULT 'executor'
        CHECK (approval_tier_required IN ('organization_creator', 'reviewer', 'executor'));

ALTER TABLE tool_executions
    ADD COLUMN IF NOT EXISTS requested_by_human_id UUID REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE tool_approvals
    ADD COLUMN IF NOT EXISTS approved_by_human_id UUID REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_tool_definitions_authority
    ON tool_definitions(tool_category, approval_tier_required, required_level);

INSERT INTO tool_definitions (
    name, description, source_type, default_policy, risk_level, required_level,
    tool_category, approval_tier_required, input_schema, metadata
)
VALUES
    (
        'erp.action.execute',
        'Execute an ERP business action through the governed tool runtime',
        'internal_api',
        'approve',
        'high',
        'L3',
        'business_approval',
        'reviewer',
        '{"type":"object","properties":{"table_code":{"type":"string"},"key":{"type":"string"},"action":{"type":"string"},"context_package_id":{"type":"string"}},"required":["table_code","key","action"]}'::jsonb,
        '{"phase":"verified_context_tool_loop","quality_gate":"tool_runtime_approval"}'::jsonb
    ),
    (
        'industry.solution.change.preview',
        'Verify an industry solution change request without applying it',
        'internal_api',
        'notify',
        'low',
        'L2',
        'execution_operation',
        'executor',
        '{"type":"object","properties":{"industry_solution_change_request_id":{"type":"string"},"context_package_id":{"type":"string"}},"required":["industry_solution_change_request_id"]}'::jsonb,
        '{"phase":"verified_context_tool_loop","quality_gate":"solution_verify"}'::jsonb
    ),
    (
        'runtime.operation.execute',
        'Execute a platform runtime operation with dynamic approval policy',
        'internal_api',
        'notify',
        'medium',
        'L2',
        'execution_operation',
        'executor',
        '{"type":"object","properties":{"operation_id":{"type":"string"},"method":{"type":"string"},"arguments":{"type":"object"},"context_package_id":{"type":"string"}},"required":["operation_id"]}'::jsonb,
        '{"phase":"verified_context_tool_loop","dynamic_policy":true}'::jsonb
    ),
    (
        'context.proposal.apply',
        'Apply an approved context change proposal',
        'manual_approval',
        'approve',
        'high',
        'L3',
        'business_approval',
        'reviewer',
        '{"type":"object","properties":{"proposal_id":{"type":"string"},"reviewer_id":{"type":"string"},"reviewer_type":{"type":"string"},"context_package_id":{"type":"string"}},"required":["proposal_id"]}'::jsonb,
        '{"phase":"verified_context_tool_loop","quality_gate":"human_reviewed_context_change"}'::jsonb
    )
ON CONFLICT (name) DO UPDATE SET
    description = EXCLUDED.description,
    source_type = EXCLUDED.source_type,
    default_policy = EXCLUDED.default_policy,
    risk_level = EXCLUDED.risk_level,
    required_level = EXCLUDED.required_level,
    tool_category = EXCLUDED.tool_category,
    approval_tier_required = EXCLUDED.approval_tier_required,
    input_schema = EXCLUDED.input_schema,
    metadata = tool_definitions.metadata || EXCLUDED.metadata,
    is_active = TRUE,
    updated_at = NOW();

UPDATE tool_definitions
SET tool_category = 'core_data',
    approval_tier_required = 'organization_creator',
    required_level = CASE
        WHEN required_level IN ('', 'L1', 'L2', 'L3') THEN 'L4'
        ELSE required_level
    END,
    default_policy = 'approve',
    updated_at = NOW()
WHERE name LIKE 'model.%'
   OR name LIKE 'organization.%'
   OR name LIKE 'department.%'
   OR name LIKE 'position.%'
   OR name LIKE 'governance.%'
   OR name LIKE 'tool.%'
   OR name LIKE 'costing.%';

UPDATE tool_definitions
SET tool_category = 'business_approval',
    approval_tier_required = 'reviewer',
    required_level = CASE
        WHEN required_level IN ('', 'L1', 'L2') THEN 'L3'
        ELSE required_level
    END,
    default_policy = 'approve',
    updated_at = NOW()
WHERE name LIKE '%.approve%'
   OR name LIKE 'workflow.%'
   OR name LIKE 'stage.%'
   OR name LIKE 'finance.%'
   OR name IN ('project.bind_workflow', 'project.create_cost_entry', 'project.update_status',
               'project.create_deliverable', 'project.accept_deliverable', 'project.close_feedback',
               'evolution.propose_experiment');

UPDATE tool_definitions
SET metadata = metadata || CASE name
        WHEN 'project.update_status' THEN '{"label_zh":"更新项目生命周期状态","label_en":"Update project lifecycle status","description_zh":"更新项目生命周期状态","description_en":"Update project lifecycle status"}'::JSONB
        WHEN 'project.create_deliverable' THEN '{"label_zh":"创建项目交付物","label_en":"Create a project deliverable","description_zh":"创建项目交付物","description_en":"Create a project deliverable"}'::JSONB
        WHEN 'project.accept_deliverable' THEN '{"label_zh":"验收已提交的项目交付物","label_en":"Accept a submitted project deliverable","description_zh":"验收已提交的项目交付物","description_en":"Accept a submitted project deliverable"}'::JSONB
        WHEN 'project.close_feedback' THEN '{"label_zh":"关闭项目反馈闭环","label_en":"Close the project feedback loop","description_zh":"关闭项目反馈闭环","description_en":"Close the project feedback loop"}'::JSONB
        ELSE '{}'::JSONB
    END,
    updated_at = NOW()
WHERE name IN ('project.update_status', 'project.create_deliverable', 'project.accept_deliverable', 'project.close_feedback');

UPDATE tool_definitions
SET metadata = metadata || CASE name
        WHEN 'project.match_members' THEN '{"business_ai_stages":["plan","change"]}'::JSONB
        WHEN 'project.bind_workflow' THEN '{"business_ai_stages":["plan","change"]}'::JSONB
        WHEN 'project.estimate_cost' THEN '{"business_ai_stages":["plan","do","accept"]}'::JSONB
        WHEN 'project.create_cost_entry' THEN '{"business_ai_stages":["do"]}'::JSONB
        WHEN 'project.update_status' THEN '{"business_ai_stages":["do","change"]}'::JSONB
        WHEN 'project.create_deliverable' THEN '{"business_ai_stages":["do"]}'::JSONB
        WHEN 'project.accept_deliverable' THEN '{"business_ai_stages":["accept"]}'::JSONB
        WHEN 'project.close_feedback' THEN '{"business_ai_stages":["accept"]}'::JSONB
        WHEN 'evolution.create_knowledge' THEN '{"business_ai_stages":["learn"]}'::JSONB
        WHEN 'evolution.create_signal' THEN '{"business_ai_stages":["learn"]}'::JSONB
        WHEN 'evolution.propose_experiment' THEN '{"business_ai_stages":["learn"]}'::JSONB
        ELSE '{}'::JSONB
    END,
    updated_at = NOW()
WHERE name IN (
    'project.match_members', 'project.bind_workflow', 'project.estimate_cost', 'project.create_cost_entry',
    'project.update_status', 'project.create_deliverable', 'project.accept_deliverable', 'project.close_feedback',
    'evolution.create_knowledge', 'evolution.create_signal', 'evolution.propose_experiment'
);

UPDATE tool_definitions
SET tool_category = 'execution_operation',
    approval_tier_required = 'executor',
    updated_at = NOW()
WHERE tool_category NOT IN ('core_data', 'business_approval');

INSERT INTO security_policies(
    policy_key, distribution_mode, license_mode, scope_level, module_key,
    resource_type, action, required_authority_tier, required_permission_level, effect, conditions, metadata
)
VALUES
    ('skill.organization.activate', 'saas', 'commercial', 'organization', 'assistant', 'skill', 'activate', 'organization_admin', 'L3', 'allow', '{"same_organization":true}'::jsonb, '{"seed":true}'::jsonb),
    ('skill.saas_global.activate', 'saas', 'commercial', 'saas_global', 'assistant', 'skill', 'activate', 'organization_creator', 'L4', 'allow', '{"platform_admin":true}'::jsonb, '{"seed":true}'::jsonb),
    ('ai_gateway.model.use', 'saas', 'commercial', 'organization', 'ai_gateway', 'model_provider_channel', 'use', 'executor', 'L2', 'allow', '{"module_enabled":true}'::jsonb, '{"seed":true}'::jsonb),
    ('tool.approval.review', 'saas', 'commercial', 'organization', 'toolruntime', 'tool_approval', 'review', 'reviewer', 'L3', 'approval_required', '{"no_self_approval":true}'::jsonb, '{"seed":true}'::jsonb),
    ('owner.attest', 'saas', 'commercial', 'organization', 'organization', 'owner_attestation', 'verify', 'organization_creator', 'L4', 'allow', '{"asymmetric_signature":true}'::jsonb, '{"seed":true}'::jsonb)
ON CONFLICT (policy_key, version) DO UPDATE SET
    conditions = EXCLUDED.conditions,
    metadata = security_policies.metadata || EXCLUDED.metadata,
    updated_at = NOW();


-- -----------------------------------------------------------------------------
-- Folded from historical migration: 040_saas_permission_system.sql
-- -----------------------------------------------------------------------------

-- 040_saas_permission_system.sql
-- Organization-reviewed permission changes and scoped organization access rules.


INSERT INTO platform.platform_masters(module_key, entity_type, source_table, source_pk, title, status, organization_id, payload, metadata)
SELECT 'assistant', 'skill', 'skill', id::TEXT, name, status, organization_id, to_jsonb(s), '{"migrated_to_master_detail":true}'::JSONB
FROM public.skill s
ON CONFLICT (source_table, source_pk) WHERE source_table <> '' AND source_pk <> ''
DO UPDATE SET payload = EXCLUDED.payload, title = EXCLUDED.title, status = EXCLUDED.status, organization_id = EXCLUDED.organization_id, updated_at = NOW();

INSERT INTO platform.platform_masters(module_key, entity_type, source_table, source_pk, title, status, organization_id, payload, metadata)
SELECT 'assistant', 'skill_publication_request', 'skill_publication_requests', id::TEXT, source_skill_id::TEXT, status, source_organization_id, to_jsonb(r), '{"migrated_to_master_detail":true}'::JSONB
FROM public.skill_publication_requests r
ON CONFLICT (source_table, source_pk) WHERE source_table <> '' AND source_pk <> ''
DO UPDATE SET payload = EXCLUDED.payload, title = EXCLUDED.title, status = EXCLUDED.status, organization_id = EXCLUDED.organization_id, updated_at = NOW();

INSERT INTO platform.platform_details(master_key, detail_type, field_key, line_no, payload, metadata)
SELECT
    pm.master_key,
    'field',
    f.field_name,
    f.display_order,
    to_jsonb(f),
    '{"source":"data_field_catalog"}'::JSONB
FROM platform.platform_masters pm
JOIN public.data_field_catalog f ON pm.source_table = 'data_table_catalog' AND pm.source_pk = f.table_name
ON CONFLICT DO NOTHING;

INSERT INTO platform.platform_details(master_key, detail_type, field_key, line_no, payload, metadata)
SELECT
    pm.master_key,
    'skill_component',
    component->>'key',
    ordinality::INT,
    component,
    '{"source":"skill.skill_components"}'::JSONB
FROM platform.platform_masters pm
JOIN public.skill s ON pm.source_table = 'skill' AND pm.source_pk = s.id::TEXT
CROSS JOIN LATERAL jsonb_array_elements(s.skill_components) WITH ORDINALITY AS components(component, ordinality)
ON CONFLICT DO NOTHING;



-- Cross-stage foreign keys rebuilt after AI capability tables exist.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_mvru_members_agent') THEN
        ALTER TABLE mvru_members
            ADD CONSTRAINT fk_mvru_members_agent
            FOREIGN KEY (agent_id) REFERENCES ai_agents(id) ON DELETE CASCADE;
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_organization_memberships_agent') THEN
        ALTER TABLE organization_memberships
            ADD CONSTRAINT fk_organization_memberships_agent
            FOREIGN KEY (agent_id) REFERENCES ai_agents(id) ON DELETE CASCADE;
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_finance_export_lines_usage_ledger') THEN
        ALTER TABLE finance_export_lines
            ADD CONSTRAINT fk_finance_export_lines_usage_ledger
            FOREIGN KEY (usage_ledger_id) REFERENCES ai_usage_ledger(id) ON DELETE RESTRICT;
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_finance_export_lines_provider') THEN
        ALTER TABLE finance_export_lines
            ADD CONSTRAINT fk_finance_export_lines_provider
            FOREIGN KEY (provider_id) REFERENCES model_providers(id) ON DELETE SET NULL;
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_finance_export_lines_model') THEN
        ALTER TABLE finance_export_lines
            ADD CONSTRAINT fk_finance_export_lines_model
            FOREIGN KEY (model_id) REFERENCES models(id) ON DELETE SET NULL;
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_ai_usage_finance_export_line') THEN
        ALTER TABLE ai_usage_ledger
            ADD CONSTRAINT fk_ai_usage_finance_export_line
            FOREIGN KEY (finance_export_line_id) REFERENCES finance_export_lines(id) ON DELETE SET NULL;
    END IF;
END $$;


