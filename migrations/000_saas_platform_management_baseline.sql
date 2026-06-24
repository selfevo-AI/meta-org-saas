-- SaaS platform management baseline.
-- Organizations are platform-managed tenant accounts. Tenant runtime is
-- single-organization: tenant-facing business hierarchy is represented by
-- departments scoped by organization_id, not by an organization tree.

-- 000_saas_platform_management_baseline.sql
-- SaaS management-platform baseline.
--
-- Execution principle:
-- 1. Build the SaaS management platform first.
-- 2. The management platform owns industry-solution creation and adjustment
--    capabilities: tables, modules, functions, runtime operations, governance,
--    permissions, security, schema-change workflow, and package metadata.
-- 3. ERP is created after that as an industry-solution baseline in
--    001_erp_code_baseline.sql.
-- 4. AI/model/agent/tool/assistant/skill capability tables are isolated in
--    004_ai_capability_baseline.sql after platform and ERP tables exist.
--
-- Future database-structure changes made while implementing platform-management
-- capabilities must update this file and BASELINE_RESTRUCTURE.md in the same
-- change. Do not insert later filenames into schema_migrations from this file.
-- Historical 001-044 migrations have been folded into the staged baselines and
-- removed from the active migration set.
--
-- 000 基线原则：先有 SaaS 管理平台；行业解决方案中的表、模块、功能、
-- 运行时、治理、权限、安全和 schema 调整能力都由管理平台承载；
-- ERP 基线在管理平台之后由 001_erp_code_baseline.sql 承载；
-- AI/模型/agent/工具/助手/skill 能力表归 004_ai_capability_baseline.sql。

-- -----------------------------------------------------------------------------
-- Folded from historical migration: 001_identity.sql
-- -----------------------------------------------------------------------------

-- 001_identity.sql

CREATE TABLE IF NOT EXISTS schema_migrations (
    filename    VARCHAR(255) PRIMARY KEY,
    applied_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

DO $$ BEGIN
    CREATE TYPE role_type AS ENUM ('planner', 'executor', 'reviewer');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE permission_level AS ENUM ('L1', 'L2', 'L3', 'L4');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(255) NOT NULL,
    email           VARCHAR(255) UNIQUE NOT NULL,
    password_hash   VARCHAR(255) NOT NULL,
    avatar_url      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS roles (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(100) NOT NULL UNIQUE,
    role_type       role_type NOT NULL,
    description     TEXT,
    permissions     JSONB NOT NULL DEFAULT '[]',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS user_roles (
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id         UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, role_id)
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);



-- -----------------------------------------------------------------------------
-- Folded from historical migration: 002_seed_roles.sql
-- -----------------------------------------------------------------------------

-- 002_seed_roles.sql

INSERT INTO roles (name, role_type, description, permissions) VALUES
  ('Strategic Planner', 'planner', 'C-suite and strategic decision makers', '["org:read","org:write","strategy:full","governance:full"]'),
  ('Tactical Planner', 'planner', 'MVRU leads and product managers', '["mvru:read","mvru:write","workflow:full","capability:read"]'),
  ('AI Planner', 'planner', 'AI agents responsible for planning', '["mvru:read","workflow:read","capability:read"]'),
  ('Human Executor', 'executor', 'Human team members executing tasks', '["task:read","task:write","capability:use"]'),
  ('AI Executor', 'executor', 'AI agents executing defined tasks', '["task:read","task:write","capability:use"]'),
  ('Independent Reviewer', 'reviewer', 'Independent reviewers (human)', '["review:full","verification:read"]'),
  ('AI Reviewer', 'reviewer', 'AI agents performing automated review', '["review:limited","verification:read"]')
ON CONFLICT (name) DO NOTHING;


-- -----------------------------------------------------------------------------
-- Folded from historical migration: 003_organization.sql
-- -----------------------------------------------------------------------------

-- 003_organization.sql

CREATE TABLE IF NOT EXISTS organizations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

DO $$ BEGIN
    CREATE TYPE mvru_status AS ENUM ('designing', 'active', 'evaluating', 'evolving', 'dissolved');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS muvrs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name            VARCHAR(255) NOT NULL,
    description     TEXT,
    status          mvru_status NOT NULL DEFAULT 'designing',
    boundary        JSONB NOT NULL DEFAULT '{"data_permissions":[],"resource_quota":{},"network_policies":[]}',
    config          JSONB NOT NULL DEFAULT '{}',
    parent_id       UUID REFERENCES muvrs(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS teams (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    mvru_id     UUID NOT NULL REFERENCES muvrs(id) ON DELETE CASCADE,
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS mvru_members (
    mvru_id     UUID NOT NULL REFERENCES muvrs(id) ON DELETE CASCADE,
    user_id     UUID REFERENCES users(id) ON DELETE CASCADE,
    agent_id    UUID,
    role_id     UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    CONSTRAINT chk_one_actor CHECK (
        (user_id IS NOT NULL AND agent_id IS NULL) OR
        (user_id IS NULL AND agent_id IS NOT NULL)
    ),
    CONSTRAINT uq_mvru_user UNIQUE (mvru_id, user_id),
    CONSTRAINT uq_mvru_agent UNIQUE (mvru_id, agent_id)
);

CREATE TABLE IF NOT EXISTS mvru_relationships (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_mvru_id  UUID NOT NULL REFERENCES muvrs(id) ON DELETE CASCADE,
    target_mvru_id  UUID NOT NULL REFERENCES muvrs(id) ON DELETE CASCADE,
    rel_type        VARCHAR(50) NOT NULL DEFAULT 'collaborate',
    config          JSONB DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_no_self_ref CHECK (source_mvru_id != target_mvru_id)
);

CREATE INDEX IF NOT EXISTS idx_muvrs_org ON muvrs(organization_id);
CREATE INDEX IF NOT EXISTS idx_teams_mvru ON teams(mvru_id);


-- -----------------------------------------------------------------------------
-- Folded from historical migration: 004_layer.sql
-- -----------------------------------------------------------------------------

-- 004_layer.sql

DO $$ BEGIN
    CREATE TYPE layer_type AS ENUM ('strategic', 'tactical', 'operational');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS layer_configs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    mvru_id         UUID NOT NULL REFERENCES muvrs(id) ON DELETE CASCADE,
    layer           layer_type NOT NULL,
    config          JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (mvru_id, layer)
);

CREATE TABLE IF NOT EXISTS layer_routing_rules (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_layer    layer_type NOT NULL,
    target_layer    layer_type NOT NULL,
    condition       JSONB NOT NULL DEFAULT '{}',
    priority        INT NOT NULL DEFAULT 0,
    is_active       BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_layer_config_mvru ON layer_configs(mvru_id);


-- -----------------------------------------------------------------------------
-- Folded from historical migration: 005_capability.sql
-- -----------------------------------------------------------------------------

-- 005_capability.sql

CREATE TABLE IF NOT EXISTS capabilities (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(255) NOT NULL,
    version         VARCHAR(50) NOT NULL DEFAULT '1.0',
    description     TEXT,
    input_schema    JSONB NOT NULL DEFAULT '{}',
    output_schema   JSONB NOT NULL DEFAULT '{}',
    preconditions   JSONB NOT NULL DEFAULT '[]',
    error_handling  JSONB NOT NULL DEFAULT '{}',
    permission_level permission_level NOT NULL DEFAULT 'L2',
    cost_estimate   JSONB NOT NULL DEFAULT '{}',
    is_active       BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (name, version)
);

CREATE TABLE IF NOT EXISTS capability_bindings (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    capability_id   UUID NOT NULL REFERENCES capabilities(id) ON DELETE CASCADE,
    mvru_id         UUID NOT NULL REFERENCES muvrs(id) ON DELETE CASCADE,
    config          JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (capability_id, mvru_id)
);

CREATE TABLE IF NOT EXISTS capability_invocations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    capability_id   UUID NOT NULL REFERENCES capabilities(id) ON DELETE CASCADE,
    caller_id       UUID NOT NULL,
    caller_type     VARCHAR(10) NOT NULL,
    input           JSONB,
    output          JSONB,
    duration_ms     INT,
    cost            NUMERIC(12,4),
    outcome         VARCHAR(20),
    trace_id        UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_cap_name ON capabilities(name);
CREATE INDEX IF NOT EXISTS idx_cap_bind_mvru ON capability_bindings(mvru_id);
CREATE INDEX IF NOT EXISTS idx_cap_inv_caller ON capability_invocations(caller_id);


-- -----------------------------------------------------------------------------
-- Folded from historical migration: 006_workflow.sql
-- -----------------------------------------------------------------------------

-- 006_workflow.sql

DO $$ BEGIN
    CREATE TYPE workflow_status AS ENUM ('active', 'paused', 'completed', 'failed');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE task_status AS ENUM ('pending', 'assigned', 'in_progress', 'completed', 'rejected');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE stage_type AS ENUM ('plan', 'execute', 'review');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS workflow_templates (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(255) NOT NULL,
    description     TEXT,
    stages          JSONB NOT NULL DEFAULT '[]',
    assignee_type   VARCHAR(10) NOT NULL DEFAULT 'either',
    required_weight NUMERIC(5,2) DEFAULT 0,
    routing_rules   JSONB NOT NULL DEFAULT '{}',
    is_active       BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS workflow_instances (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id     UUID NOT NULL REFERENCES workflow_templates(id),
    status          workflow_status NOT NULL DEFAULT 'active',
    current_stage   INT NOT NULL DEFAULT 0,
    context         JSONB NOT NULL DEFAULT '{}',
    trace_id        UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tasks (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id     UUID NOT NULL REFERENCES workflow_instances(id) ON DELETE CASCADE,
    stage           INT NOT NULL,
    stage_type      stage_type NOT NULL,
    assignee_id     UUID,
    assignee_type   VARCHAR(10),
    input           JSONB,
    output          JSONB,
    weight_snapshot NUMERIC(5,2),
    status          task_status NOT NULL DEFAULT 'pending',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS decisions (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id           UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    decision_maker_id UUID NOT NULL,
    maker_type        VARCHAR(10) NOT NULL,
    weight            NUMERIC(5,2) DEFAULT 0,
    input             JSONB,
    output            JSONB,
    reasoning         TEXT,
    outcome           VARCHAR(50),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS workflow_contexts (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id       UUID NOT NULL REFERENCES workflow_instances(id) ON DELETE CASCADE,
    working_memory    JSONB NOT NULL DEFAULT '{}',
    injected_experience JSONB NOT NULL DEFAULT '[]',
    principle_notes   TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (workflow_id)
);

CREATE INDEX IF NOT EXISTS idx_tasks_workflow ON tasks(workflow_id);
CREATE INDEX IF NOT EXISTS idx_tasks_assignee ON tasks(assignee_id);
CREATE INDEX IF NOT EXISTS idx_decisions_task ON decisions(task_id);


-- -----------------------------------------------------------------------------
-- Folded from historical migration: 007_observability.sql
-- -----------------------------------------------------------------------------

-- 007_observability.sql

CREATE TABLE IF NOT EXISTS traces (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id     UUID REFERENCES workflow_instances(id) ON DELETE SET NULL,
    status          TEXT NOT NULL DEFAULT 'active',
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ,
    metadata        JSONB NOT NULL DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS spans (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trace_id        UUID NOT NULL REFERENCES traces(id) ON DELETE CASCADE,
    parent_span_id  UUID REFERENCES spans(id) ON DELETE SET NULL,
    span_type       TEXT NOT NULL,
    entity_id       UUID,
    entity_type     TEXT,
    actor_id        UUID,
    actor_type      TEXT,
    input           JSONB,
    output          JSONB,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ,
    duration_ms     INT,
    metadata        JSONB NOT NULL DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS metrics (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    metric_type     TEXT NOT NULL,
    metric_name     TEXT NOT NULL,
    entity_id       UUID,
    entity_type     TEXT,
    value           DOUBLE PRECISION NOT NULL,
    recorded_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata        JSONB NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_spans_trace_id ON spans(trace_id);
CREATE INDEX IF NOT EXISTS idx_spans_actor ON spans(actor_id);
CREATE INDEX IF NOT EXISTS idx_spans_type ON spans(span_type);
CREATE INDEX IF NOT EXISTS idx_metrics_type ON metrics(metric_type, metric_name);
CREATE INDEX IF NOT EXISTS idx_metrics_recorded_at ON metrics(recorded_at);


-- -----------------------------------------------------------------------------
-- Folded from historical migration: 008_verification.sql
-- -----------------------------------------------------------------------------

-- 008_verification.sql

CREATE TABLE IF NOT EXISTS verification_reports (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id         UUID REFERENCES workflow_instances(id) ON DELETE SET NULL,
    task_id             UUID REFERENCES tasks(id) ON DELETE SET NULL,
    result_score        DOUBLE PRECISION,
    path_score          DOUBLE PRECISION,
    environment_score   DOUBLE PRECISION,
    overall_score       DOUBLE PRECISION,
    conclusion          TEXT NOT NULL DEFAULT '',
    suggestions         JSONB NOT NULL DEFAULT '[]',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS review_assignments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    report_id       UUID NOT NULL REFERENCES verification_reports(id) ON DELETE CASCADE,
    level           TEXT NOT NULL,
    reviewer_id     UUID,
    reviewer_type   TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending',
    result          JSONB,
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_verif_workflow ON verification_reports(workflow_id);
CREATE INDEX IF NOT EXISTS idx_verif_task ON verification_reports(task_id);
CREATE INDEX IF NOT EXISTS idx_review_report ON review_assignments(report_id);
CREATE INDEX IF NOT EXISTS idx_review_reviewer ON review_assignments(reviewer_id);


-- -----------------------------------------------------------------------------
-- Folded from historical migration: 009_governance.sql
-- -----------------------------------------------------------------------------

-- 009_governance.sql

CREATE TABLE IF NOT EXISTS permissions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    level       INT NOT NULL CHECK (level BETWEEN 1 AND 4),
    name        TEXT NOT NULL UNIQUE,
    description TEXT,
    behavior    TEXT NOT NULL DEFAULT 'notify',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS principles (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name             TEXT NOT NULL UNIQUE,
    description      TEXT NOT NULL,
    evaluation_logic JSONB NOT NULL DEFAULT '{}',
    priority         INT NOT NULL DEFAULT 0,
    is_active        BOOLEAN NOT NULL DEFAULT true,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS control_rules (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    principle_id        UUID REFERENCES principles(id) ON DELETE CASCADE,
    target_entity_type  TEXT NOT NULL,
    target_entity_id    UUID,
    condition           JSONB NOT NULL DEFAULT '{}',
    action              TEXT NOT NULL,
    priority            INT NOT NULL DEFAULT 0,
    is_active           BOOLEAN NOT NULL DEFAULT true,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_principles_active ON principles(is_active);
CREATE INDEX IF NOT EXISTS idx_control_principle ON control_rules(principle_id);
CREATE INDEX IF NOT EXISTS idx_control_target ON control_rules(target_entity_type, target_entity_id);


-- -----------------------------------------------------------------------------
-- Folded from historical migration: 010_evolution.sql
-- -----------------------------------------------------------------------------

-- 010_evolution.sql

CREATE TABLE IF NOT EXISTS weight_scores (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id              UUID NOT NULL,
    actor_type            TEXT NOT NULL,
    overall_score         DOUBLE PRECISION NOT NULL DEFAULT 1.0,
    expertise_score       DOUBLE PRECISION DEFAULT 0,
    track_record_score    DOUBLE PRECISION DEFAULT 0,
    reliability_score     DOUBLE PRECISION DEFAULT 0,
    recency_score         DOUBLE PRECISION DEFAULT 0,
    context_fit_score     DOUBLE PRECISION DEFAULT 0,
    principle_score       DOUBLE PRECISION DEFAULT 0,
    decision_count        INT NOT NULL DEFAULT 0,
    last_updated          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (actor_id, actor_type)
);

CREATE TABLE IF NOT EXISTS weight_alphas (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    alpha_expertise     DOUBLE PRECISION NOT NULL DEFAULT 0.25,
    alpha_track_record  DOUBLE PRECISION NOT NULL DEFAULT 0.20,
    alpha_reliability   DOUBLE PRECISION NOT NULL DEFAULT 0.15,
    alpha_recency       DOUBLE PRECISION NOT NULL DEFAULT 0.10,
    alpha_context_fit   DOUBLE PRECISION NOT NULL DEFAULT 0.10,
    alpha_principle     DOUBLE PRECISION NOT NULL DEFAULT 0.20,
    version             INT NOT NULL DEFAULT 1,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS experiments (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                TEXT NOT NULL,
    hypothesis          TEXT NOT NULL,
    status              TEXT NOT NULL DEFAULT 'proposed',
    mvru_id             UUID,
    alpha_overrides     JSONB,
    success_criteria    JSONB NOT NULL DEFAULT '{}',
    started_at          TIMESTAMPTZ,
    completed_at        TIMESTAMPTZ,
    conclusion          TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS knowledge_entries (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id         UUID REFERENCES workflow_instances(id) ON DELETE SET NULL,
    title               TEXT NOT NULL,
    content             TEXT NOT NULL,
    tags                TEXT[] DEFAULT '{}',
    source              TEXT NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS signals (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    signal_type         TEXT NOT NULL,
    source              TEXT NOT NULL,
    priority            INT NOT NULL DEFAULT 0,
    data                JSONB NOT NULL DEFAULT '{}',
    acknowledged        BOOLEAN NOT NULL DEFAULT false,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_weight_actor ON weight_scores(actor_id, actor_type);
CREATE INDEX IF NOT EXISTS idx_weight_overall ON weight_scores(overall_score DESC);
CREATE INDEX IF NOT EXISTS idx_experiment_status ON experiments(status);
CREATE INDEX IF NOT EXISTS idx_knowledge_tags ON knowledge_entries USING GIN(tags);
CREATE INDEX IF NOT EXISTS idx_knowledge_source ON knowledge_entries(source);
CREATE INDEX IF NOT EXISTS idx_signals_priority ON signals(priority DESC, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_signals_acknowledged ON signals(acknowledged);


-- -----------------------------------------------------------------------------
-- Folded from historical migration: 011_organization_tree.sql
-- -----------------------------------------------------------------------------

-- 011_organization_tree.sql

CREATE TABLE IF NOT EXISTS departments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    parent_id       UUID REFERENCES departments(id) ON DELETE SET NULL,
    name            TEXT NOT NULL,
    code            TEXT,
    description     TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'archived')),
    sort_order      INT NOT NULL DEFAULT 0,
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_department_not_self_parent CHECK (parent_id IS NULL OR parent_id <> id)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_departments_org_code
    ON departments(organization_id, code)
    WHERE code IS NOT NULL AND code <> '';

CREATE INDEX IF NOT EXISTS idx_departments_org_parent ON departments(organization_id, parent_id, sort_order);
CREATE INDEX IF NOT EXISTS idx_departments_status ON departments(status);

CREATE TABLE IF NOT EXISTS external_members (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL,
    email           TEXT,
    vendor          TEXT NOT NULL DEFAULT '',
    contract_type   TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'archived')),
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_external_members_email
    ON external_members(email)
    WHERE email IS NOT NULL AND email <> '';

CREATE INDEX IF NOT EXISTS idx_external_members_status ON external_members(status);

CREATE TABLE IF NOT EXISTS organization_memberships (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id     UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    department_id       UUID NOT NULL REFERENCES departments(id) ON DELETE CASCADE,
    member_type         TEXT NOT NULL CHECK (member_type IN ('internal', 'external', 'agent')),
    user_id             UUID REFERENCES users(id) ON DELETE CASCADE,
    external_member_id  UUID REFERENCES external_members(id) ON DELETE CASCADE,
    agent_id    UUID,
    title               TEXT NOT NULL DEFAULT '',
    role_id             UUID REFERENCES roles(id) ON DELETE SET NULL,
    status              TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'archived')),
    joined_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata            JSONB NOT NULL DEFAULT '{}',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_organization_membership_actor CHECK (
        (member_type = 'internal' AND user_id IS NOT NULL AND external_member_id IS NULL AND agent_id IS NULL) OR
        (member_type = 'external' AND user_id IS NULL AND external_member_id IS NOT NULL AND agent_id IS NULL) OR
        (member_type = 'agent' AND user_id IS NULL AND external_member_id IS NULL AND agent_id IS NOT NULL)
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_org_membership_internal
    ON organization_memberships(department_id, user_id)
    WHERE member_type = 'internal';

CREATE UNIQUE INDEX IF NOT EXISTS uq_org_membership_external
    ON organization_memberships(department_id, external_member_id)
    WHERE member_type = 'external';

CREATE UNIQUE INDEX IF NOT EXISTS uq_org_membership_agent
    ON organization_memberships(department_id, agent_id)
    WHERE member_type = 'agent';

CREATE INDEX IF NOT EXISTS idx_org_memberships_org ON organization_memberships(organization_id);
CREATE INDEX IF NOT EXISTS idx_org_memberships_department ON organization_memberships(department_id);
CREATE INDEX IF NOT EXISTS idx_org_memberships_type_status ON organization_memberships(member_type, status);

CREATE TABLE IF NOT EXISTS department_mvru_links (
    department_id   UUID NOT NULL REFERENCES departments(id) ON DELETE CASCADE,
    mvru_id         UUID NOT NULL REFERENCES muvrs(id) ON DELETE CASCADE,
    link_type       TEXT NOT NULL DEFAULT 'execution',
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (department_id, mvru_id, link_type)
);

CREATE INDEX IF NOT EXISTS idx_department_mvru_links_mvru ON department_mvru_links(mvru_id);


-- -----------------------------------------------------------------------------
-- Folded from historical migration: 012_policy_weight_evaluation.sql
-- -----------------------------------------------------------------------------

-- 012_policy_weight_evaluation.sql

CREATE TABLE IF NOT EXISTS employee_profiles (
    user_id             UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    employee_no         TEXT UNIQUE,
    employment_type     TEXT NOT NULL DEFAULT 'internal'
        CHECK (employment_type IN ('internal', 'contractor', 'partner')),
    status              TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'inactive', 'archived')),
    title               TEXT,
    metadata            JSONB NOT NULL DEFAULT '{}',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS access_decisions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id            UUID NOT NULL,
    actor_type          TEXT NOT NULL,
    action              TEXT NOT NULL,
    resource            TEXT NOT NULL,
    resource_id         UUID,
    organization_id     UUID,
    department_id       UUID,
    workflow_id         UUID REFERENCES workflow_instances(id) ON DELETE SET NULL,
    task_id             UUID REFERENCES tasks(id) ON DELETE SET NULL,
    capability_id       UUID REFERENCES capabilities(id) ON DELETE SET NULL,
    required_level      TEXT NOT NULL DEFAULT 'L1',
    risk_level          TEXT NOT NULL DEFAULT 'low'
        CHECK (risk_level IN ('low', 'medium', 'high', 'critical')),
    decision            TEXT NOT NULL
        CHECK (decision IN ('allow', 'notify', 'approve', 'deny')),
    allowed             BOOLEAN NOT NULL,
    behavior            TEXT NOT NULL,
    reason              TEXT NOT NULL,
    matched_rules       JSONB NOT NULL DEFAULT '[]',
    weight_snapshot     DOUBLE PRECISION,
    context             JSONB NOT NULL DEFAULT '{}',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS context_weight_scores (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id              UUID NOT NULL,
    actor_type            TEXT NOT NULL,
    scope_hash            TEXT NOT NULL,
    organization_id       UUID,
    department_id         UUID,
    workflow_template_id  UUID REFERENCES workflow_templates(id) ON DELETE SET NULL,
    workflow_stage        TEXT,
    task_type             TEXT,
    capability_id         UUID REFERENCES capabilities(id) ON DELETE SET NULL,
    risk_level            TEXT NOT NULL DEFAULT 'low'
        CHECK (risk_level IN ('low', 'medium', 'high', 'critical')),
    overall_score         DOUBLE PRECISION NOT NULL DEFAULT 1.0,
    expertise_score       DOUBLE PRECISION NOT NULL DEFAULT 0.5,
    track_record_score    DOUBLE PRECISION NOT NULL DEFAULT 0.5,
    reliability_score     DOUBLE PRECISION NOT NULL DEFAULT 0.5,
    recency_score         DOUBLE PRECISION NOT NULL DEFAULT 1.0,
    context_fit_score     DOUBLE PRECISION NOT NULL DEFAULT 0.5,
    principle_score       DOUBLE PRECISION NOT NULL DEFAULT 0.5,
    decision_count        INT NOT NULL DEFAULT 0,
    context               JSONB NOT NULL DEFAULT '{}',
    last_updated          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (actor_id, actor_type, scope_hash)
);

CREATE TABLE IF NOT EXISTS capability_evaluations (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    capability_id         UUID REFERENCES capabilities(id) ON DELETE SET NULL,
    actor_id              UUID,
    actor_type            TEXT,
    workflow_id           UUID REFERENCES workflow_instances(id) ON DELETE SET NULL,
    task_id               UUID REFERENCES tasks(id) ON DELETE SET NULL,
    evaluator_id          UUID,
    evaluator_type        TEXT NOT NULL DEFAULT 'human',
    quality_score         DOUBLE PRECISION NOT NULL DEFAULT 0.5,
    reliability_score     DOUBLE PRECISION NOT NULL DEFAULT 0.5,
    cost_score            DOUBLE PRECISION NOT NULL DEFAULT 0.5,
    latency_score         DOUBLE PRECISION NOT NULL DEFAULT 0.5,
    risk_score            DOUBLE PRECISION NOT NULL DEFAULT 0.5,
    compliance_score      DOUBLE PRECISION NOT NULL DEFAULT 0.5,
    overall_score         DOUBLE PRECISION NOT NULL DEFAULT 0.5,
    evidence              JSONB NOT NULL DEFAULT '{}',
    conclusion            TEXT,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (capability_id IS NOT NULL OR actor_id IS NOT NULL)
);


CREATE INDEX IF NOT EXISTS idx_employee_profiles_status ON employee_profiles(status, employment_type);
CREATE INDEX IF NOT EXISTS idx_access_decisions_actor ON access_decisions(actor_id, actor_type, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_access_decisions_resource ON access_decisions(resource, action, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_access_decisions_context ON access_decisions(organization_id, department_id, risk_level);
CREATE INDEX IF NOT EXISTS idx_context_weights_actor ON context_weight_scores(actor_id, actor_type, overall_score DESC);
CREATE INDEX IF NOT EXISTS idx_context_weights_scope ON context_weight_scores(scope_hash, risk_level);
CREATE INDEX IF NOT EXISTS idx_capability_evaluations_capability ON capability_evaluations(capability_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_capability_evaluations_actor ON capability_evaluations(actor_id, actor_type, created_at DESC);


-- -----------------------------------------------------------------------------
-- Folded from historical migration: 015_single_org_positions_workflow_graph.sql
-- -----------------------------------------------------------------------------

-- 015_single_org_positions_workflow_graph.sql

CREATE TABLE IF NOT EXISTS positions (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id       UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    department_id         UUID NOT NULL REFERENCES departments(id) ON DELETE CASCADE,
    name                  TEXT NOT NULL,
    code                  TEXT,
    description           TEXT NOT NULL DEFAULT '',
    status                TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'inactive', 'archived')),
    sort_order            INT NOT NULL DEFAULT 0,
    permission_level      TEXT NOT NULL DEFAULT 'L1'
        CHECK (permission_level IN ('L1', 'L2', 'L3', 'L4')),
    required_capabilities JSONB NOT NULL DEFAULT '[]',
    metadata              JSONB NOT NULL DEFAULT '{}',
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_positions_department_code
    ON positions(department_id, code)
    WHERE code IS NOT NULL AND code <> '';

CREATE INDEX IF NOT EXISTS idx_positions_org_department ON positions(organization_id, department_id, status, sort_order);
CREATE INDEX IF NOT EXISTS idx_positions_permission ON positions(permission_level, status);

CREATE TABLE IF NOT EXISTS position_assignments (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    position_id        UUID NOT NULL REFERENCES positions(id) ON DELETE CASCADE,
    organization_id    UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    department_id      UUID NOT NULL REFERENCES departments(id) ON DELETE CASCADE,
    actor_id           UUID NOT NULL,
    actor_type         TEXT NOT NULL
        CHECK (actor_type IN ('internal_human', 'external_human', 'internal_agent', 'external_agent')),
    assignment_type    TEXT NOT NULL DEFAULT 'candidate'
        CHECK (assignment_type IN ('primary', 'backup', 'candidate')),
    allocation_percent NUMERIC(5,2) NOT NULL DEFAULT 100,
    status             TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'inactive', 'archived')),
    metadata           JSONB NOT NULL DEFAULT '{}',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_position_assignment_actor
    ON position_assignments(position_id, actor_id, actor_type)
    WHERE status <> 'archived';

CREATE INDEX IF NOT EXISTS idx_position_assignments_position ON position_assignments(position_id, status);
CREATE INDEX IF NOT EXISTS idx_position_assignments_actor ON position_assignments(actor_id, actor_type, status);

ALTER TABLE workflow_templates
    ADD COLUMN IF NOT EXISTS organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS department_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS visual_graph JSONB NOT NULL DEFAULT '{}';

ALTER TABLE workflow_instances
    ADD COLUMN IF NOT EXISTS organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS department_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS project_id UUID;

CREATE INDEX IF NOT EXISTS idx_workflow_templates_org_department ON workflow_templates(organization_id, department_id, is_active);
CREATE INDEX IF NOT EXISTS idx_workflow_instances_org_project ON workflow_instances(organization_id, project_id, status);


-- -----------------------------------------------------------------------------
-- Folded from historical migration: 021_meta_resource_pdca.sql
-- -----------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS meta_resources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    resource_type TEXT NOT NULL
        CHECK (resource_type IN ('human', 'external_human', 'agent', 'model_channel', 'tool', 'material', 'time', 'capability', 'budget', 'resource')),
    source_type TEXT NOT NULL DEFAULT '',
    source_id UUID,
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'inactive', 'reserved', 'exhausted', 'archived')),
    organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
    department_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    owner_actor_id UUID,
    owner_actor_type TEXT NOT NULL DEFAULT '',
    capability_profile JSONB NOT NULL DEFAULT '{}',
    cost_profile JSONB NOT NULL DEFAULT '{}',
    capacity_profile JSONB NOT NULL DEFAULT '{}',
    risk_profile JSONB NOT NULL DEFAULT '{}',
    performance_profile JSONB NOT NULL DEFAULT '{}',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_meta_resources_unique_source
    ON meta_resources(resource_type, source_type, source_id)
    WHERE source_type <> '' AND source_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_meta_resources_type_status
    ON meta_resources(resource_type, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_meta_resources_org_department
    ON meta_resources(organization_id, department_id);

CREATE TABLE IF NOT EXISTS demand_profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    requirement_id UUID,
    project_id UUID,
    title TEXT NOT NULL,
    goal TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'planned', 'active', 'accepted', 'closed', 'archived')),
    acceptance_criteria JSONB NOT NULL DEFAULT '[]',
    required_capabilities JSONB NOT NULL DEFAULT '[]',
    budget_constraints JSONB NOT NULL DEFAULT '{}',
    time_constraints JSONB NOT NULL DEFAULT '{}',
    risk_constraints JSONB NOT NULL DEFAULT '{}',
    resource_fit_candidates JSONB NOT NULL DEFAULT '[]',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_demand_profiles_requirement
    ON demand_profiles(requirement_id);

CREATE INDEX IF NOT EXISTS idx_demand_profiles_project
    ON demand_profiles(project_id);

CREATE INDEX IF NOT EXISTS idx_demand_profiles_status
    ON demand_profiles(status, created_at DESC);

CREATE TABLE IF NOT EXISTS pdca_cycles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    demand_profile_id UUID REFERENCES demand_profiles(id) ON DELETE SET NULL,
    requirement_id UUID,
    project_id UUID,
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'completed', 'cancelled', 'archived')),
    current_stage TEXT NOT NULL DEFAULT 'plan'
        CHECK (current_stage IN ('plan', 'do', 'change', 'accept')),
    outcome_score DOUBLE PRECISION NOT NULL DEFAULT 0,
    summary TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_pdca_cycles_demand
    ON pdca_cycles(demand_profile_id);

CREATE INDEX IF NOT EXISTS idx_pdca_cycles_status_stage
    ON pdca_cycles(status, current_stage, created_at DESC);

CREATE TABLE IF NOT EXISTS pdca_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cycle_id UUID NOT NULL REFERENCES pdca_cycles(id) ON DELETE CASCADE,
    stage TEXT NOT NULL CHECK (stage IN ('plan', 'do', 'change', 'accept')),
    event_type TEXT NOT NULL DEFAULT 'note',
    source_type TEXT NOT NULL DEFAULT '',
    source_id UUID,
    actor_id UUID,
    actor_type TEXT NOT NULL DEFAULT '',
    resource_refs JSONB NOT NULL DEFAULT '[]',
    cost_refs JSONB NOT NULL DEFAULT '[]',
    evidence JSONB NOT NULL DEFAULT '{}',
    decision TEXT NOT NULL DEFAULT '',
    next_action TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_pdca_events_cycle_stage
    ON pdca_events(cycle_id, stage, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_pdca_events_source
    ON pdca_events(source_type, source_id);


-- -----------------------------------------------------------------------------
-- Folded from historical migration: 023_org_meta_task_matrix.sql
-- -----------------------------------------------------------------------------

ALTER TABLE position_assignments
    ADD COLUMN IF NOT EXISTS meta_resource_id UUID REFERENCES meta_resources(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_position_assignments_meta_resource
    ON position_assignments(meta_resource_id, status);

CREATE TABLE IF NOT EXISTS task_matrix_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    workflow_id UUID NOT NULL REFERENCES workflow_instances(id) ON DELETE CASCADE,
    project_id UUID,
    organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
    department_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    position_id UUID NOT NULL REFERENCES positions(id) ON DELETE CASCADE,
    position_assignment_id UUID REFERENCES position_assignments(id) ON DELETE SET NULL,
    meta_resource_id UUID NOT NULL REFERENCES meta_resources(id) ON DELETE CASCADE,
    actor_id UUID NOT NULL,
    actor_type TEXT NOT NULL
        CHECK (actor_type IN ('internal_human', 'external_human', 'internal_agent', 'external_agent')),
    role_in_task TEXT NOT NULL DEFAULT 'owner'
        CHECK (role_in_task IN ('owner', 'reviewer', 'support', 'observer')),
    allocation_percent NUMERIC(5,2) NOT NULL DEFAULT 100,
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'inactive', 'archived')),
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_task_matrix_assignment_role
    ON task_matrix_assignments(task_id, position_id, meta_resource_id, role_in_task)
    WHERE status <> 'archived';

CREATE INDEX IF NOT EXISTS idx_task_matrix_assignments_task
    ON task_matrix_assignments(task_id, role_in_task, status);

CREATE INDEX IF NOT EXISTS idx_task_matrix_assignments_meta_resource
    ON task_matrix_assignments(meta_resource_id, status);

CREATE INDEX IF NOT EXISTS idx_task_matrix_assignments_position
    ON task_matrix_assignments(position_id, position_assignment_id, status);


-- -----------------------------------------------------------------------------
-- Folded from historical migration: 024_strong_base_data_uniqueness.sql
-- -----------------------------------------------------------------------------

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

SELECT assert_no_duplicate_key('organizations.name', $$
    SELECT string_agg(key, '; ') FROM (
        SELECT lower(btrim(name)) AS key
        FROM organizations
        GROUP BY lower(btrim(name))
        HAVING COUNT(*) > 1
    ) d
$$);

SELECT assert_no_duplicate_key('muvrs.organization_id.name', $$
    SELECT string_agg(key, '; ') FROM (
        SELECT organization_id::text || ':' || lower(btrim(name)) AS key
        FROM muvrs
        GROUP BY organization_id, lower(btrim(name))
        HAVING COUNT(*) > 1
    ) d
$$);

SELECT assert_no_duplicate_key('teams.mvru_id.name', $$
    SELECT string_agg(key, '; ') FROM (
        SELECT mvru_id::text || ':' || lower(btrim(name)) AS key
        FROM teams
        GROUP BY mvru_id, lower(btrim(name))
        HAVING COUNT(*) > 1
    ) d
$$);

SELECT assert_no_duplicate_key('departments.organization_id.code', $$
    SELECT string_agg(key, '; ') FROM (
        SELECT organization_id::text || ':' || lower(btrim(code)) AS key
        FROM departments
        WHERE code IS NOT NULL AND btrim(code) <> ''
        GROUP BY organization_id, lower(btrim(code))
        HAVING COUNT(*) > 1
    ) d
$$);

SELECT assert_no_duplicate_key('departments.organization_id.name', $$
    SELECT string_agg(key, '; ') FROM (
        SELECT organization_id::text || ':' || lower(btrim(name)) AS key
        FROM departments
        GROUP BY organization_id, lower(btrim(name))
        HAVING COUNT(*) > 1
    ) d
$$);

SELECT assert_no_duplicate_key('positions.department_id.code', $$
    SELECT string_agg(key, '; ') FROM (
        SELECT department_id::text || ':' || lower(btrim(code)) AS key
        FROM positions
        WHERE code IS NOT NULL AND btrim(code) <> ''
        GROUP BY department_id, lower(btrim(code))
        HAVING COUNT(*) > 1
    ) d
$$);

SELECT assert_no_duplicate_key('positions.department_id.name', $$
    SELECT string_agg(key, '; ') FROM (
        SELECT department_id::text || ':' || lower(btrim(name)) AS key
        FROM positions
        GROUP BY department_id, lower(btrim(name))
        HAVING COUNT(*) > 1
    ) d
$$);

SELECT assert_no_duplicate_key('external_members.email', $$
    SELECT string_agg(key, '; ') FROM (
        SELECT lower(btrim(email)) AS key
        FROM external_members
        WHERE email IS NOT NULL AND btrim(email) <> ''
        GROUP BY lower(btrim(email))
        HAVING COUNT(*) > 1
    ) d
$$);

SELECT assert_no_duplicate_key('capabilities.name.version', $$
    SELECT string_agg(key, '; ') FROM (
        SELECT lower(btrim(name)) || ':' || version AS key
        FROM capabilities
        GROUP BY lower(btrim(name)), version
        HAVING COUNT(*) > 1
    ) d
$$);

SELECT assert_no_duplicate_key('workflow_templates.organization.department.name', $$
    SELECT string_agg(key, '; ') FROM (
        SELECT COALESCE(organization_id::text, '') || ':' || COALESCE(department_id::text, '') || ':' || lower(btrim(name)) AS key
        FROM workflow_templates
        GROUP BY organization_id, department_id, lower(btrim(name))
        HAVING COUNT(*) > 1
    ) d
$$);

CREATE UNIQUE INDEX IF NOT EXISTS uq_organizations_name_ci
    ON organizations (lower(btrim(name)));

CREATE UNIQUE INDEX IF NOT EXISTS uq_muvrs_org_name_ci
    ON muvrs (organization_id, lower(btrim(name)));

CREATE UNIQUE INDEX IF NOT EXISTS uq_teams_mvru_name_ci
    ON teams (mvru_id, lower(btrim(name)));

CREATE UNIQUE INDEX IF NOT EXISTS uq_departments_org_code_ci
    ON departments (organization_id, lower(btrim(code)))
    WHERE code IS NOT NULL AND btrim(code) <> '';

CREATE UNIQUE INDEX IF NOT EXISTS uq_departments_org_name_ci
    ON departments (organization_id, lower(btrim(name)));

CREATE UNIQUE INDEX IF NOT EXISTS uq_positions_department_code_ci
    ON positions (department_id, lower(btrim(code)))
    WHERE code IS NOT NULL AND btrim(code) <> '';

CREATE UNIQUE INDEX IF NOT EXISTS uq_positions_department_name_ci
    ON positions (department_id, lower(btrim(name)));

CREATE UNIQUE INDEX IF NOT EXISTS uq_external_members_email_ci
    ON external_members (lower(btrim(email)))
    WHERE email IS NOT NULL AND btrim(email) <> '';

CREATE UNIQUE INDEX IF NOT EXISTS uq_capabilities_name_version_ci
    ON capabilities (lower(btrim(name)), version);

CREATE UNIQUE INDEX IF NOT EXISTS uq_workflow_templates_scope_name_ci
    ON workflow_templates (COALESCE(organization_id, '00000000-0000-0000-0000-000000000000'::uuid), COALESCE(department_id, '00000000-0000-0000-0000-000000000000'::uuid), lower(btrim(name)));

DROP FUNCTION assert_no_duplicate_key(TEXT, TEXT);


-- -----------------------------------------------------------------------------
-- Folded from historical migration: 025_authority_tiers_agent_governance.sql
-- -----------------------------------------------------------------------------

ALTER TABLE organizations
    ADD COLUMN IF NOT EXISTS created_by UUID REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE organization_memberships
    ADD COLUMN IF NOT EXISTS authority_tier TEXT NOT NULL DEFAULT 'executor'
        CHECK (authority_tier IN ('organization_creator', 'reviewer', 'executor'));

CREATE INDEX IF NOT EXISTS idx_organizations_created_by
    ON organizations(created_by)
    WHERE created_by IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_org_memberships_authority
    ON organization_memberships(organization_id, user_id, authority_tier, status)
    WHERE user_id IS NOT NULL;

-- 026_master_detail_permissions.sql
-- Compatible foundation for physical master/detail records, user field preferences,
-- and table/field level authorization.

CREATE TABLE IF NOT EXISTS data_table_catalog (
    table_name           TEXT PRIMARY KEY,
    master_table_name    TEXT NOT NULL,
    detail_table_name    TEXT NOT NULL,
    key_prefix           TEXT NOT NULL,
    display_name         TEXT NOT NULL DEFAULT '',
    category             TEXT NOT NULL DEFAULT 'system',
    is_base_data         BOOLEAN NOT NULL DEFAULT false,
    is_business_scenario BOOLEAN NOT NULL DEFAULT false,
    metadata             JSONB NOT NULL DEFAULT '{}',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS data_field_catalog (
    table_name         TEXT NOT NULL REFERENCES data_table_catalog(table_name) ON DELETE CASCADE,
    field_name         TEXT NOT NULL,
    data_type          TEXT NOT NULL DEFAULT '',
    display_name       TEXT NOT NULL DEFAULT '',
    is_master_key      BOOLEAN NOT NULL DEFAULT false,
    is_sub_key         BOOLEAN NOT NULL DEFAULT false,
    is_visible_default BOOLEAN NOT NULL DEFAULT true,
    permission_level   TEXT NOT NULL DEFAULT 'L1'
        CHECK (permission_level IN ('L1', 'L2', 'L3', 'L4')),
    display_order      INT NOT NULL DEFAULT 0,
    metadata           JSONB NOT NULL DEFAULT '{}',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (table_name, field_name)
);

CREATE TABLE IF NOT EXISTS user_field_preferences (
    actor_id       TEXT NOT NULL,
    table_name     TEXT NOT NULL REFERENCES data_table_catalog(table_name) ON DELETE CASCADE,
    visible_fields JSONB NOT NULL DEFAULT '[]',
    field_order    JSONB NOT NULL DEFAULT '[]',
    field_widths   JSONB NOT NULL DEFAULT '{}',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (actor_id, table_name)
);

CREATE TABLE IF NOT EXISTS field_permission_rules (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    table_name       TEXT NOT NULL REFERENCES data_table_catalog(table_name) ON DELETE CASCADE,
    field_name       TEXT NOT NULL DEFAULT '*',
    actor_type       TEXT NOT NULL DEFAULT '*',
    actor_id         TEXT,
    role_id          UUID REFERENCES roles(id) ON DELETE CASCADE,
    action           TEXT NOT NULL CHECK (action IN ('read', 'write', 'delete', 'admin')),
    behavior         TEXT NOT NULL DEFAULT 'allow'
        CHECK (behavior IN ('allow', 'notify', 'approve', 'deny')),
    required_level   TEXT NOT NULL DEFAULT 'L1'
        CHECK (required_level IN ('L1', 'L2', 'L3', 'L4')),
    reason           TEXT NOT NULL DEFAULT '',
    metadata         JSONB NOT NULL DEFAULT '{}',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_field_permission_rules_lookup
    ON field_permission_rules(table_name, field_name, action, actor_type, actor_id);

CREATE TABLE IF NOT EXISTS business_key_sequences (
    entity_name TEXT NOT NULL,
    date_key    DATE NOT NULL,
    next_value  BIGINT NOT NULL DEFAULT 1,
    PRIMARY KEY (entity_name, date_key)
);

CREATE OR REPLACE FUNCTION next_business_key(p_entity_name TEXT, p_prefix TEXT DEFAULT NULL)
RETURNS TEXT
LANGUAGE plpgsql
AS $$
DECLARE
    v_date DATE := CURRENT_DATE;
    v_value BIGINT;
    v_prefix TEXT;
BEGIN
    SELECT key_prefix INTO v_prefix
    FROM data_table_catalog
    WHERE table_name = p_entity_name;

    v_prefix := COALESCE(
        NULLIF(p_prefix, ''),
        NULLIF(v_prefix, ''),
        UPPER(LEFT(REGEXP_REPLACE(p_entity_name, '[^A-Za-z0-9]', '', 'g'), 6))
    );

    INSERT INTO business_key_sequences(entity_name, date_key, next_value)
    VALUES (p_entity_name, v_date, 2)
    ON CONFLICT (entity_name, date_key)
    DO UPDATE SET next_value = business_key_sequences.next_value + 1
    RETURNING next_value - 1 INTO v_value;

    RETURN v_prefix || '-' || TO_CHAR(v_date, 'YYYYMMDD') || '-' || LPAD(v_value::TEXT, 6, '0');
END;
$$;

DO $$
DECLARE
    rec RECORD;
    v_detail_table TEXT;
    v_prefix TEXT;
    v_detail_prefix TEXT;
    v_has_uuid_id BOOLEAN;
BEGIN
    FOR rec IN
        SELECT table_name
        FROM information_schema.tables
        WHERE table_schema = 'public'
          AND table_type = 'BASE TABLE'
          AND table_name NOT IN (
              'schema_migrations',
              'data_table_catalog',
              'data_field_catalog',
              'user_field_preferences',
              'field_permission_rules',
              'business_key_sequences'
          )
          AND table_name NOT LIKE '%\_details' ESCAPE '\'
        ORDER BY table_name
    LOOP
        v_detail_table := rec.table_name || '_details';
        v_prefix := UPPER(LEFT(REGEXP_REPLACE(rec.table_name, '[^A-Za-z0-9]', '', 'g'), 6));
        v_detail_prefix := UPPER(LEFT(REGEXP_REPLACE(v_detail_table, '[^A-Za-z0-9]', '', 'g'), 6));

        INSERT INTO data_table_catalog(
            table_name,
            master_table_name,
            detail_table_name,
            key_prefix,
            display_name,
            category,
            is_base_data,
            is_business_scenario
        )
        VALUES (
            rec.table_name,
            rec.table_name || '_masters',
            v_detail_table,
            v_prefix,
            rec.table_name,
            CASE
                WHEN rec.table_name IN (
                    'requirements', 'requirement_documents', 'requirement_analysis_workflows',
                    'projects', 'project_members', 'project_workflows', 'deliverables',
                    'project_cost_entries', 'project_evaluations', 'meta_resources',
                    'demand_profiles', 'pdca_cycles', 'pdca_events', 'finance_export_batches',
                    'finance_export_lines', 'tool_executions', 'tool_approvals'
                ) THEN 'business'
                ELSE 'base_data'
            END,
            rec.table_name NOT IN (
                'requirements', 'requirement_documents', 'requirement_analysis_workflows',
                'projects', 'project_members', 'project_workflows', 'deliverables',
                'project_cost_entries', 'project_evaluations', 'meta_resources',
                'demand_profiles', 'pdca_cycles', 'pdca_events', 'finance_export_batches',
                'finance_export_lines', 'tool_executions', 'tool_approvals',
                'traces', 'spans', 'metrics', 'assistant_sessions', 'assistant_messages',
                'assistant_steps', 'assistant_memories', 'ai_invocations', 'ai_usage_ledger',
                'access_decisions', 'capability_invocations', 'finance_webhook_events'
            ),
            rec.table_name IN (
                'requirements', 'requirement_documents', 'requirement_analysis_workflows',
                'projects', 'project_members', 'project_workflows', 'deliverables',
                'project_cost_entries', 'project_evaluations', 'meta_resources',
                'demand_profiles', 'pdca_cycles', 'pdca_events', 'finance_export_batches',
                'finance_export_lines', 'tool_executions', 'tool_approvals'
            )
        )
        ON CONFLICT (table_name) DO UPDATE SET
            master_table_name = EXCLUDED.master_table_name,
            detail_table_name = EXCLUDED.detail_table_name,
            key_prefix = EXCLUDED.key_prefix,
            category = EXCLUDED.category,
            is_base_data = EXCLUDED.is_base_data,
            is_business_scenario = EXCLUDED.is_business_scenario,
            updated_at = NOW();

        SELECT EXISTS (
            SELECT 1
            FROM information_schema.columns
            WHERE table_schema = 'public'
              AND table_name = rec.table_name
              AND column_name = 'id'
              AND data_type = 'uuid'
        ) INTO v_has_uuid_id;

        EXECUTE FORMAT('ALTER TABLE %I ADD COLUMN IF NOT EXISTS legacy_id UUID', rec.table_name);
        IF v_has_uuid_id THEN
            EXECUTE FORMAT('UPDATE %I SET legacy_id = id WHERE legacy_id IS NULL AND id IS NOT NULL', rec.table_name);
        END IF;
        EXECUTE FORMAT('ALTER TABLE %I ADD COLUMN IF NOT EXISTS master_key TEXT', rec.table_name);
        EXECUTE FORMAT('ALTER TABLE %I ADD COLUMN IF NOT EXISTS parent_master_table TEXT', rec.table_name);
        EXECUTE FORMAT('ALTER TABLE %I ADD COLUMN IF NOT EXISTS parent_master_key TEXT', rec.table_name);
        EXECUTE FORMAT('UPDATE %I SET master_key = next_business_key(%L, %L) WHERE master_key IS NULL', rec.table_name, rec.table_name, v_prefix);
        EXECUTE FORMAT('ALTER TABLE %I ALTER COLUMN master_key SET NOT NULL', rec.table_name);
        EXECUTE FORMAT('CREATE UNIQUE INDEX IF NOT EXISTS %I ON %I(master_key)', 'uq_' || rec.table_name || '_master_key', rec.table_name);
        EXECUTE FORMAT('CREATE INDEX IF NOT EXISTS %I ON %I(parent_master_table, parent_master_key)', 'idx_' || rec.table_name || '_parent_master', rec.table_name);

        EXECUTE FORMAT(
            'CREATE TABLE IF NOT EXISTS %I (
                sub_key TEXT PRIMARY KEY DEFAULT next_business_key(%L, %L),
                master_key TEXT NOT NULL REFERENCES %I(master_key) ON DELETE CASCADE,
                parent_master_table TEXT,
                parent_master_key TEXT,
                detail_type TEXT NOT NULL DEFAULT ''field'',
                line_no INT NOT NULL DEFAULT 0,
                field_key TEXT NOT NULL DEFAULT '''',
                field_value JSONB NOT NULL DEFAULT ''null''::jsonb,
                payload JSONB NOT NULL DEFAULT ''{}''::jsonb,
                metadata JSONB NOT NULL DEFAULT ''{}''::jsonb,
                created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
            )',
            v_detail_table, v_detail_table, v_detail_prefix, rec.table_name
        );
        EXECUTE FORMAT('CREATE INDEX IF NOT EXISTS %I ON %I(master_key, line_no)', 'idx_' || v_detail_table || '_master', v_detail_table);
    END LOOP;
END;
$$;

INSERT INTO data_field_catalog(table_name, field_name, data_type, display_name, is_master_key, is_visible_default, display_order)
SELECT
    c.table_name,
    c.column_name,
    c.data_type,
    c.column_name,
    c.column_name = 'master_key',
    c.column_name NOT IN ('api_key_hash', 'password_hash', 'secret_ciphertext', 'content', 'metadata'),
    c.ordinal_position
FROM information_schema.columns c
JOIN data_table_catalog t ON t.table_name = c.table_name
WHERE c.table_schema = 'public'
ON CONFLICT (table_name, field_name) DO UPDATE SET
    data_type = EXCLUDED.data_type,
    is_master_key = EXCLUDED.is_master_key,
    is_visible_default = EXCLUDED.is_visible_default,
    display_order = EXCLUDED.display_order,
    updated_at = NOW();

INSERT INTO field_permission_rules(table_name, field_name, action, behavior, required_level, reason)
SELECT table_name, field_name, 'read', 'deny', 'L4', 'sensitive field is hidden by default'
FROM data_field_catalog
WHERE field_name IN ('api_key_hash', 'password_hash', 'secret_ciphertext', 'content')
ON CONFLICT DO NOTHING;


-- -----------------------------------------------------------------------------
-- Folded from historical migration: 033_user_ui_preferences.sql
-- -----------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS user_ui_preferences (
    actor_id       TEXT NOT NULL,
    preference_key TEXT NOT NULL,
    value          JSONB NOT NULL DEFAULT '{}',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (actor_id, preference_key)
);

CREATE INDEX IF NOT EXISTS idx_user_ui_preferences_key
    ON user_ui_preferences(preference_key);


-- -----------------------------------------------------------------------------
-- Folded from historical migration: 034_saas_foundation.sql
-- -----------------------------------------------------------------------------

-- 034_saas_foundation.sql
-- SaaS mode foundation: platform administration, onboarding, invitations,
-- module entitlements, subscription metadata, and organization-scoped roles.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS account_status TEXT NOT NULL DEFAULT 'active'
        CHECK (account_status IN ('active', 'disabled')),
    ADD COLUMN IF NOT EXISTS onboarding_status TEXT NOT NULL DEFAULT 'required'
        CHECK (onboarding_status IN ('required', 'complete')),
    ADD COLUMN IF NOT EXISTS default_organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS last_login_at TIMESTAMPTZ;

UPDATE users u
SET onboarding_status = 'complete',
    default_organization_id = COALESCE(u.default_organization_id, first_org.organization_id)
FROM (
    SELECT DISTINCT ON (user_id) user_id, organization_id
    FROM organization_memberships
    WHERE user_id IS NOT NULL AND status = 'active'
    ORDER BY user_id, joined_at ASC
) first_org
WHERE u.id = first_org.user_id
  AND u.onboarding_status = 'required';

CREATE INDEX IF NOT EXISTS idx_users_onboarding
    ON users(onboarding_status, default_organization_id);

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM users
        GROUP BY lower(email)
        HAVING COUNT(*) > 1
    ) THEN
        RAISE EXCEPTION 'duplicate user emails found when compared case-insensitively; resolve duplicates before applying SaaS foundation migration';
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS uq_users_email_lower
    ON users(lower(email));

CREATE TABLE IF NOT EXISTS platform_admins (
    user_id     UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    role        TEXT NOT NULL DEFAULT 'system_owner'
        CHECK (role IN ('system_owner', 'system_admin', 'support')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS saas_modules (
    module_key      TEXT PRIMARY KEY,
    display_name    TEXT NOT NULL,
    category        TEXT NOT NULL DEFAULT 'business',
    enabled_default BOOLEAN NOT NULL DEFAULT true,
    license_scope   TEXT NOT NULL DEFAULT 'mit'
        CHECK (license_scope IN ('mit', 'commercial')),
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS saas_plans (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code           TEXT NOT NULL UNIQUE,
    name           TEXT NOT NULL,
    description    TEXT NOT NULL DEFAULT '',
    status         TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'archived')),
    price_amount   NUMERIC(14,2) NOT NULL DEFAULT 0,
    currency       TEXT NOT NULL DEFAULT 'CNY',
    billing_cycle  TEXT NOT NULL DEFAULT 'monthly'
        CHECK (billing_cycle IN ('monthly', 'yearly', 'manual')),
    metadata       JSONB NOT NULL DEFAULT '{}',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS saas_plan_modules (
    plan_id    UUID NOT NULL REFERENCES saas_plans(id) ON DELETE CASCADE,
    module_key TEXT NOT NULL REFERENCES saas_modules(module_key) ON DELETE CASCADE,
    limit_json JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (plan_id, module_key)
);

CREATE TABLE IF NOT EXISTS organization_subscriptions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL UNIQUE REFERENCES organizations(id) ON DELETE CASCADE,
    plan_id         UUID REFERENCES saas_plans(id) ON DELETE SET NULL,
    status          TEXT NOT NULL DEFAULT 'trialing'
        CHECK (status IN ('trialing', 'active', 'past_due', 'cancelled', 'expired')),
    trial_ends_at   TIMESTAMPTZ,
    current_period_start TIMESTAMPTZ,
    current_period_end   TIMESTAMPTZ,
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS organization_module_entitlements (
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    module_key      TEXT NOT NULL REFERENCES saas_modules(module_key) ON DELETE CASCADE,
    status          TEXT NOT NULL DEFAULT 'enabled'
        CHECK (status IN ('enabled', 'disabled')),
    source          TEXT NOT NULL DEFAULT 'plan'
        CHECK (source IN ('plan', 'manual', 'trial')),
    limit_json      JSONB NOT NULL DEFAULT '{}',
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (organization_id, module_key)
);

CREATE INDEX IF NOT EXISTS idx_org_entitlements_status
    ON organization_module_entitlements(module_key, status);

CREATE TABLE IF NOT EXISTS organization_usage_events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    module_key      TEXT NOT NULL REFERENCES saas_modules(module_key) ON DELETE CASCADE,
    usage_key       TEXT NOT NULL,
    quantity        NUMERIC(18,6) NOT NULL DEFAULT 1,
    occurred_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata        JSONB NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_org_usage_events_lookup
    ON organization_usage_events(organization_id, module_key, usage_key, occurred_at DESC);

CREATE TABLE IF NOT EXISTS organization_invitations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email           TEXT NOT NULL,
    name            TEXT NOT NULL DEFAULT '',
    role_id         UUID REFERENCES roles(id) ON DELETE SET NULL,
    authority_tier  TEXT NOT NULL DEFAULT 'executor'
        CHECK (authority_tier IN ('organization_creator', 'organization_admin', 'reviewer', 'executor')),
    token_hash      TEXT NOT NULL UNIQUE,
    status          TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'accepted', 'revoked', 'expired')),
    invited_by      UUID REFERENCES users(id) ON DELETE SET NULL,
    accepted_by     UUID REFERENCES users(id) ON DELETE SET NULL,
    expires_at      TIMESTAMPTZ NOT NULL,
    metadata        JSONB NOT NULL DEFAULT '{}',
    accepted_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_org_invitations_org_status
    ON organization_invitations(organization_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_org_invitations_email_status
    ON organization_invitations(lower(email), status);

DO $$
DECLARE
    constraint_name TEXT;
BEGIN
    SELECT conname INTO constraint_name
    FROM pg_constraint
    WHERE conrelid = 'organization_memberships'::regclass
      AND contype = 'c'
      AND pg_get_constraintdef(oid) LIKE '%authority_tier%';

    IF constraint_name IS NOT NULL THEN
        EXECUTE FORMAT('ALTER TABLE organization_memberships DROP CONSTRAINT %I', constraint_name);
    END IF;
END $$;

ALTER TABLE organization_memberships
    ADD CONSTRAINT chk_organization_memberships_authority_tier
    CHECK (authority_tier IN ('organization_creator', 'organization_admin', 'reviewer', 'executor'));

INSERT INTO saas_modules(module_key, display_name, category, enabled_default, license_scope) VALUES
    ('organization', 'Organization', 'base_data', true, 'mit'),
    ('project', 'Project Operations', 'business', true, 'mit'),
    ('workflow', 'Workflow', 'business', true, 'mit'),
    ('governance', 'Governance', 'governance', true, 'mit'),
    ('evolution', 'Evolution', 'governance', true, 'mit'),
    ('capability', 'Capability', 'base_data', true, 'mit'),
    ('meta_resource', 'Meta Resource', 'base_data', true, 'mit'),
    ('assistant', 'AI Assistant', 'ai', true, 'commercial'),
    ('ai_gateway', 'AI Gateway', 'ai', true, 'commercial'),
    ('toolruntime', 'Tool Runtime', 'ai', true, 'commercial'),
    ('finance', 'Finance', 'finance', true, 'commercial'),
    ('costing', 'Costing', 'finance', true, 'mit'),
    ('developer_tools', 'Developer Tools', 'system', true, 'commercial')
ON CONFLICT (module_key) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    category = EXCLUDED.category,
    enabled_default = EXCLUDED.enabled_default,
    license_scope = EXCLUDED.license_scope,
    updated_at = NOW();

INSERT INTO saas_plans(code, name, description, status, price_amount, currency, billing_cycle, metadata)
VALUES ('foundation', 'Foundation', 'Default SaaS foundation plan with all current modules enabled.', 'active', 0, 'CNY', 'manual', '{"default":true}'::jsonb)
ON CONFLICT (code) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    status = EXCLUDED.status,
    metadata = EXCLUDED.metadata,
    updated_at = NOW();

INSERT INTO saas_plan_modules(plan_id, module_key)
SELECT p.id, m.module_key
FROM saas_plans p
CROSS JOIN saas_modules m
WHERE p.code = 'foundation'
ON CONFLICT (plan_id, module_key) DO NOTHING;


-- -----------------------------------------------------------------------------
-- Folded from historical migration: 037_security_kernel.sql
-- -----------------------------------------------------------------------------

-- 037_security_kernel.sql
-- Security kernel policy, verifiable identity, commercial entitlement, and audit evidence.

CREATE TABLE IF NOT EXISTS security_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    policy_key TEXT NOT NULL,
    distribution_mode TEXT NOT NULL DEFAULT 'saas'
        CHECK (distribution_mode IN ('saas', 'saas_org_private', 'single_org_commercial', 'private_deployment')),
    license_mode TEXT NOT NULL DEFAULT 'community'
        CHECK (license_mode IN ('community', 'commercial', 'enterprise', 'private_contract')),
    scope_level TEXT NOT NULL DEFAULT 'organization'
        CHECK (scope_level IN ('saas_global', 'organization', 'deployment')),
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    module_key TEXT NOT NULL DEFAULT 'general',
    resource_type TEXT NOT NULL DEFAULT '',
    action TEXT NOT NULL DEFAULT '',
    required_authority_tier TEXT NOT NULL DEFAULT 'executor'
        CHECK (required_authority_tier IN ('organization_creator', 'organization_admin', 'reviewer', 'executor')),
    required_permission_level TEXT NOT NULL DEFAULT 'L1'
        CHECK (required_permission_level IN ('L1', 'L2', 'L3', 'L4')),
    effect TEXT NOT NULL DEFAULT 'allow'
        CHECK (effect IN ('allow', 'deny', 'approval_required', 'context_filter')),
    conditions JSONB NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'disabled', 'archived')),
    version INT NOT NULL DEFAULT 1,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_security_policies_key_version
    ON security_policies(policy_key, version);
CREATE INDEX IF NOT EXISTS idx_security_policies_lookup
    ON security_policies(distribution_mode, scope_level, organization_id, module_key, resource_type, action, status);

CREATE TABLE IF NOT EXISTS security_decisions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    decision_type TEXT NOT NULL DEFAULT 'deny'
        CHECK (decision_type IN ('allow', 'deny', 'approval_required', 'context_filter')),
    allowed BOOLEAN NOT NULL DEFAULT false,
    reason TEXT NOT NULL DEFAULT '',
    actor_id UUID,
    actor_type TEXT NOT NULL DEFAULT '',
    authority_tier TEXT NOT NULL DEFAULT '',
    is_platform_admin BOOLEAN NOT NULL DEFAULT false,
    organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
    distribution_mode TEXT NOT NULL DEFAULT 'saas',
    license_mode TEXT NOT NULL DEFAULT 'community',
    module_key TEXT NOT NULL DEFAULT '',
    resource_type TEXT NOT NULL DEFAULT '',
    action TEXT NOT NULL DEFAULT '',
    resource_id UUID,
    request JSONB NOT NULL DEFAULT '{}',
    response JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_security_decisions_org_time
    ON security_decisions(organization_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_security_decisions_actor_time
    ON security_decisions(actor_id, actor_type, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_security_decisions_resource
    ON security_decisions(module_key, resource_type, action, created_at DESC);

CREATE TABLE IF NOT EXISTS deployment_license_grants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    deployment_key TEXT NOT NULL,
    distribution_mode TEXT NOT NULL DEFAULT 'private_deployment'
        CHECK (distribution_mode IN ('saas', 'saas_org_private', 'single_org_commercial', 'private_deployment')),
    license_mode TEXT NOT NULL DEFAULT 'community'
        CHECK (license_mode IN ('community', 'commercial', 'enterprise', 'private_contract')),
    license_subject TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'expired', 'revoked')),
    starts_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ,
    public_key_fingerprint TEXT NOT NULL DEFAULT '',
    signed_grant JSONB NOT NULL DEFAULT '{}',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_deployment_license_grants_key
    ON deployment_license_grants(deployment_key, license_subject)
    WHERE status = 'active';

CREATE TABLE IF NOT EXISTS commercial_feature_entitlements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    deployment_key TEXT NOT NULL DEFAULT '',
    module_key TEXT NOT NULL DEFAULT 'general',
    feature_key TEXT NOT NULL,
    license_mode TEXT NOT NULL DEFAULT 'commercial'
        CHECK (license_mode IN ('community', 'commercial', 'enterprise', 'private_contract')),
    status TEXT NOT NULL DEFAULT 'enabled'
        CHECK (status IN ('enabled', 'disabled', 'expired')),
    source TEXT NOT NULL DEFAULT 'license'
        CHECK (source IN ('plan', 'license', 'manual', 'private_contract')),
    limit_json JSONB NOT NULL DEFAULT '{}',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_commercial_feature_entitlements_scope
    ON commercial_feature_entitlements(
        COALESCE(organization_id, '00000000-0000-0000-0000-000000000000'::uuid),
        deployment_key,
        module_key,
        feature_key
    );

CREATE TABLE IF NOT EXISTS verifiable_identities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    agent_id    UUID,
    algorithm TEXT NOT NULL DEFAULT 'ed25519'
        CHECK (algorithm IN ('ed25519', 'secp256k1')),
    public_key TEXT NOT NULL,
    key_fingerprint TEXT NOT NULL UNIQUE,
    verification_status TEXT NOT NULL DEFAULT 'pending'
        CHECK (verification_status IN ('pending', 'verified', 'revoked')),
    signed_registration_challenge_hash TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}',
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (
        (user_id IS NOT NULL AND agent_id IS NULL)
        OR (user_id IS NULL AND agent_id IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_verifiable_identities_user
    ON verifiable_identities(user_id, verification_status);

CREATE TABLE IF NOT EXISTS identity_challenges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_id UUID NOT NULL,
    subject_type TEXT NOT NULL DEFAULT 'user'
        CHECK (subject_type IN ('user', 'agent', 'organization_owner', 'deployment')),
    purpose TEXT NOT NULL,
    nonce TEXT NOT NULL UNIQUE,
    payload TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'verified', 'expired', 'revoked')),
    expires_at TIMESTAMPTZ NOT NULL,
    verified_at TIMESTAMPTZ,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_identity_challenges_subject
    ON identity_challenges(subject_id, subject_type, status, created_at DESC);

CREATE TABLE IF NOT EXISTS ownership_attestations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    owner_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    membership_id UUID REFERENCES organization_memberships(id) ON DELETE SET NULL,
    authority_tier TEXT NOT NULL DEFAULT 'organization_creator'
        CHECK (authority_tier IN ('organization_creator', 'organization_admin')),
    key_fingerprint TEXT NOT NULL,
    challenge_id UUID REFERENCES identity_challenges(id) ON DELETE SET NULL,
    signed_attestation_hash TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'verified'
        CHECK (status IN ('pending', 'verified', 'revoked')),
    metadata JSONB NOT NULL DEFAULT '{}',
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_ownership_attestation_active_owner
    ON ownership_attestations(organization_id, owner_user_id)
    WHERE status = 'verified';

CREATE TABLE IF NOT EXISTS audit_hash_chain (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    previous_hash TEXT,
    event_hash TEXT NOT NULL UNIQUE,
    event_type TEXT NOT NULL,
    subject_id UUID,
    organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
    payload_hash TEXT NOT NULL,
    anchor_network TEXT,
    anchor_tx_id TEXT,
    anchor_status TEXT NOT NULL DEFAULT 'not_anchored'
        CHECK (anchor_status IN ('not_anchored', 'pending', 'anchored', 'failed')),
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_hash_chain_org_time
    ON audit_hash_chain(organization_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_hash_chain_event
    ON audit_hash_chain(event_type, created_at DESC);

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


-- -----------------------------------------------------------------------------
-- Folded from historical migration: 041_system_admin_master_detail.sql
-- -----------------------------------------------------------------------------

-- 041_system_admin_master_detail.sql
-- SaaS system administration foundation.
--
-- This migration is intentionally non-destructive. Existing platform tables stay
-- in public for compatibility while platform-managed metadata is mirrored into
-- module master/detail records under the platform schema.

CREATE SCHEMA IF NOT EXISTS platform;

CREATE TABLE IF NOT EXISTS platform.platform_masters (
    master_key      TEXT PRIMARY KEY DEFAULT ('PLT-' || UPPER(SUBSTR(REPLACE(gen_random_uuid()::TEXT, '-', ''), 1, 24))),
    module_key      TEXT NOT NULL,
    entity_type     TEXT NOT NULL,
    source_table    TEXT NOT NULL DEFAULT '',
    source_pk       TEXT NOT NULL DEFAULT '',
    title           TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'active',
    organization_id UUID REFERENCES public.organizations(id) ON DELETE SET NULL,
    payload         JSONB NOT NULL DEFAULT '{}',
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE platform.platform_masters IS
    'System administration master records. Phase 2 industry solution factory stores draft context rule, assistant skill, quality gate, and verification scenario metadata assets here.';

CREATE UNIQUE INDEX IF NOT EXISTS uq_platform_masters_source
    ON platform.platform_masters(source_table, source_pk)
    WHERE source_table <> '' AND source_pk <> '';

CREATE INDEX IF NOT EXISTS idx_platform_masters_module
    ON platform.platform_masters(module_key, entity_type, status, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_platform_masters_org
    ON platform.platform_masters(organization_id, module_key, entity_type);

CREATE TABLE IF NOT EXISTS platform.platform_details (
    detail_key  TEXT PRIMARY KEY DEFAULT ('PLTD-' || UPPER(SUBSTR(REPLACE(gen_random_uuid()::TEXT, '-', ''), 1, 24))),
    master_key  TEXT NOT NULL REFERENCES platform.platform_masters(master_key) ON DELETE CASCADE,
    detail_type TEXT NOT NULL,
    field_key   TEXT NOT NULL DEFAULT '',
    line_no     INT NOT NULL DEFAULT 0,
    payload     JSONB NOT NULL DEFAULT '{}',
    metadata    JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_platform_details_master
    ON platform.platform_details(master_key, detail_type, line_no);

CREATE UNIQUE INDEX IF NOT EXISTS uq_platform_details_natural
    ON platform.platform_details(master_key, detail_type, field_key, line_no);

CREATE TABLE IF NOT EXISTS platform.schema_template_masters (
    master_key     TEXT PRIMARY KEY DEFAULT ('STM-' || UPPER(SUBSTR(REPLACE(gen_random_uuid()::TEXT, '-', ''), 1, 24))),
    template_key   TEXT NOT NULL,
    module_key     TEXT NOT NULL,
    template_scope TEXT NOT NULL DEFAULT 'platform'
        CHECK (template_scope IN ('platform', 'organization', 'deployment')),
    version_key    TEXT NOT NULL DEFAULT 'meta-org.schema.v1',
    status         TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('draft', 'active', 'archived')),
    title          TEXT NOT NULL DEFAULT '',
    package_json   JSONB NOT NULL DEFAULT '{}',
    metadata       JSONB NOT NULL DEFAULT '{}',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_schema_template_masters_key
    ON platform.schema_template_masters(template_key, version_key);

CREATE TABLE IF NOT EXISTS platform.schema_template_details (
    detail_key  TEXT PRIMARY KEY DEFAULT ('STD-' || UPPER(SUBSTR(REPLACE(gen_random_uuid()::TEXT, '-', ''), 1, 24))),
    master_key  TEXT NOT NULL REFERENCES platform.schema_template_masters(master_key) ON DELETE CASCADE,
    detail_type TEXT NOT NULL,
    field_key   TEXT NOT NULL DEFAULT '',
    line_no     INT NOT NULL DEFAULT 0,
    payload     JSONB NOT NULL DEFAULT '{}',
    metadata    JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_schema_template_details_master
    ON platform.schema_template_details(master_key, detail_type, line_no);

CREATE UNIQUE INDEX IF NOT EXISTS uq_schema_template_details_natural
    ON platform.schema_template_details(master_key, detail_type, field_key, line_no);

CREATE TABLE IF NOT EXISTS platform.organization_schema_targets (
    organization_id        UUID PRIMARY KEY REFERENCES public.organizations(id) ON DELETE CASCADE,
    schema_name            TEXT NOT NULL UNIQUE,
    template_version       TEXT NOT NULL DEFAULT 'meta-org.schema.v1',
    status                 TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'provisioned', 'applying', 'error', 'archived')),
    last_change_request_id UUID,
    metadata               JSONB NOT NULL DEFAULT '{}',
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS platform.schema_change_requests (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES public.organizations(id) ON DELETE CASCADE,
    schema_name     TEXT NOT NULL,
    request_type    TEXT NOT NULL DEFAULT 'import_schema_package',
    status          TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'approved', 'rejected', 'applied', 'failed', 'cancelled')),
    reason          TEXT NOT NULL DEFAULT '',
    schema_package  JSONB NOT NULL DEFAULT '{}',
    statements      JSONB NOT NULL DEFAULT '[]',
    requested_by    UUID REFERENCES public.users(id) ON DELETE SET NULL,
    reviewed_by     UUID REFERENCES public.users(id) ON DELETE SET NULL,
    applied_by      UUID REFERENCES public.users(id) ON DELETE SET NULL,
    review_reason   TEXT NOT NULL DEFAULT '',
    reviewed_at     TIMESTAMPTZ,
    applied_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (jsonb_typeof(schema_package) = 'object'),
    CHECK (jsonb_typeof(statements) = 'array')
);

CREATE INDEX IF NOT EXISTS idx_schema_change_requests_org
    ON platform.schema_change_requests(organization_id, status, created_at DESC);

CREATE TABLE IF NOT EXISTS platform.schema_apply_jobs (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    change_request_id UUID NOT NULL REFERENCES platform.schema_change_requests(id) ON DELETE CASCADE,
    organization_id   UUID NOT NULL REFERENCES public.organizations(id) ON DELETE CASCADE,
    schema_name       TEXT NOT NULL,
    status            TEXT NOT NULL DEFAULT 'running'
        CHECK (status IN ('running', 'applied', 'failed')),
    statements        JSONB NOT NULL DEFAULT '[]',
    error_message     TEXT NOT NULL DEFAULT '',
    metadata          JSONB NOT NULL DEFAULT '{}',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (jsonb_typeof(statements) = 'array')
);

COMMENT ON TABLE platform.schema_apply_jobs IS
    'Schema apply execution log. Phase 2 industry solution factory stores per-asset apply results in metadata.asset_results.';

CREATE INDEX IF NOT EXISTS idx_schema_apply_jobs_request
    ON platform.schema_apply_jobs(change_request_id, status, created_at DESC);

CREATE OR REPLACE FUNCTION platform.organization_schema_name(p_organization_id UUID)
RETURNS TEXT
LANGUAGE sql
IMMUTABLE
AS $$
    SELECT 'org_' || REPLACE(p_organization_id::TEXT, '-', '');
$$;

CREATE OR REPLACE FUNCTION platform.upsert_platform_master(
    p_module_key TEXT,
    p_entity_type TEXT,
    p_source_table TEXT,
    p_source_pk TEXT,
    p_title TEXT,
    p_status TEXT,
    p_organization_id UUID,
    p_payload JSONB,
    p_metadata JSONB
)
RETURNS TEXT
LANGUAGE plpgsql
AS $$
DECLARE
    v_master_key TEXT;
BEGIN
    INSERT INTO platform.platform_masters(
        module_key, entity_type, source_table, source_pk, title, status,
        organization_id, payload, metadata
    )
    VALUES (
        p_module_key, p_entity_type, p_source_table, p_source_pk,
        COALESCE(p_title, ''), COALESCE(NULLIF(p_status, ''), 'active'),
        p_organization_id, COALESCE(p_payload, '{}'::JSONB), COALESCE(p_metadata, '{}'::JSONB)
    )
    ON CONFLICT (source_table, source_pk) WHERE source_table <> '' AND source_pk <> ''
    DO UPDATE SET
        module_key = EXCLUDED.module_key,
        entity_type = EXCLUDED.entity_type,
        title = EXCLUDED.title,
        status = EXCLUDED.status,
        organization_id = EXCLUDED.organization_id,
        payload = EXCLUDED.payload,
        metadata = platform.platform_masters.metadata || EXCLUDED.metadata,
        updated_at = NOW()
    RETURNING master_key INTO v_master_key;

    RETURN v_master_key;
END;
$$;

CREATE OR REPLACE FUNCTION platform.ensure_organization_schema(p_organization_id UUID)
RETURNS TEXT
LANGUAGE plpgsql
AS $$
DECLARE
    v_schema_name TEXT := platform.organization_schema_name(p_organization_id);
BEGIN
    EXECUTE FORMAT('CREATE SCHEMA IF NOT EXISTS %I', v_schema_name);

    EXECUTE FORMAT(
        'CREATE TABLE IF NOT EXISTS %I.%I (
            master_key TEXT PRIMARY KEY DEFAULT (''ORGM-'' || UPPER(SUBSTR(REPLACE(gen_random_uuid()::TEXT, ''-'', ''''), 1, 24))),
            entity_type TEXT NOT NULL,
            legacy_table TEXT NOT NULL DEFAULT '''',
            legacy_pk TEXT NOT NULL DEFAULT '''',
            legacy_id UUID,
            title TEXT NOT NULL DEFAULT '''',
            status TEXT NOT NULL DEFAULT ''active'',
            parent_master_key TEXT NOT NULL DEFAULT '''',
            core_data JSONB NOT NULL DEFAULT ''{}''::JSONB,
            metadata JSONB NOT NULL DEFAULT ''{}''::JSONB,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        )',
        v_schema_name,
        'organization_masters'
    );

    EXECUTE FORMAT(
        'CREATE TABLE IF NOT EXISTS %I.%I (
            detail_key TEXT PRIMARY KEY DEFAULT (''ORGD-'' || UPPER(SUBSTR(REPLACE(gen_random_uuid()::TEXT, ''-'', ''''), 1, 24))),
            master_key TEXT NOT NULL REFERENCES %I.%I(master_key) ON DELETE CASCADE,
            detail_type TEXT NOT NULL,
            field_key TEXT NOT NULL DEFAULT '''',
            line_no INT NOT NULL DEFAULT 0,
            payload JSONB NOT NULL DEFAULT ''{}''::JSONB,
            metadata JSONB NOT NULL DEFAULT ''{}''::JSONB,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        )',
        v_schema_name,
        'organization_details',
        v_schema_name,
        'organization_masters'
    );

    EXECUTE FORMAT('CREATE INDEX IF NOT EXISTS %I ON %I.%I(entity_type, status)', 'idx_' || v_schema_name || '_org_masters_entity', v_schema_name, 'organization_masters');
    EXECUTE FORMAT('CREATE INDEX IF NOT EXISTS %I ON %I.%I(master_key, detail_type, line_no)', 'idx_' || v_schema_name || '_org_details_master', v_schema_name, 'organization_details');

    INSERT INTO platform.organization_schema_targets(
        organization_id, schema_name, template_version, status, metadata
    )
    VALUES (
        p_organization_id, v_schema_name, 'meta-org.schema.v1', 'provisioned',
        '{"source":"migration_041_system_admin_master_detail"}'::JSONB
    )
    ON CONFLICT (organization_id) DO UPDATE SET
        schema_name = EXCLUDED.schema_name,
        status = CASE
            WHEN platform.organization_schema_targets.status = 'archived' THEN 'archived'
            ELSE 'provisioned'
        END,
        metadata = platform.organization_schema_targets.metadata || EXCLUDED.metadata,
        updated_at = NOW();

    RETURN v_schema_name;
END;
$$;

INSERT INTO platform.organization_schema_targets(organization_id, schema_name, template_version, status, metadata)
SELECT id, platform.organization_schema_name(id), 'meta-org.schema.v1', 'pending', '{"source":"existing_organizations"}'::JSONB
FROM public.organizations
ON CONFLICT (organization_id) DO NOTHING;

DO $$
DECLARE
    rec RECORD;
BEGIN
    FOR rec IN SELECT id FROM public.organizations ORDER BY created_at LOOP
        PERFORM platform.ensure_organization_schema(rec.id);
    END LOOP;
END;
$$;

INSERT INTO platform.schema_template_masters(template_key, module_key, template_scope, version_key, status, title, package_json, metadata)
VALUES (
    'organization.default',
    'organization',
    'platform',
    'meta-org.schema.v1',
    'active',
    'Default organization master/detail schema',
    '{
        "format_version": "meta-org.schema.v1",
        "module_key": "organization",
        "tables": [
            {"name":"organization_masters","fields":[
                {"name":"master_key","data_type":"text","primary_key":true},
                {"name":"entity_type","data_type":"text","nullable":false},
                {"name":"legacy_table","data_type":"text","nullable":false,"default":"''''"},
                {"name":"legacy_pk","data_type":"text","nullable":false,"default":"''''"},
                {"name":"legacy_id","data_type":"uuid","nullable":true},
                {"name":"title","data_type":"text","nullable":false,"default":"''''"},
                {"name":"status","data_type":"text","nullable":false,"default":"''active''"},
                {"name":"parent_master_key","data_type":"text","nullable":false,"default":"''''"},
                {"name":"core_data","data_type":"jsonb","nullable":false,"default":"''{}''::jsonb"},
                {"name":"metadata","data_type":"jsonb","nullable":false,"default":"''{}''::jsonb"},
                {"name":"created_at","data_type":"timestamptz","nullable":false,"default":"now()"},
                {"name":"updated_at","data_type":"timestamptz","nullable":false,"default":"now()"}
            ]},
            {"name":"organization_details","fields":[
                {"name":"detail_key","data_type":"text","primary_key":true},
                {"name":"master_key","data_type":"text","nullable":false},
                {"name":"detail_type","data_type":"text","nullable":false},
                {"name":"field_key","data_type":"text","nullable":false,"default":"''''"},
                {"name":"line_no","data_type":"integer","nullable":false,"default":"0"},
                {"name":"payload","data_type":"jsonb","nullable":false,"default":"''{}''::jsonb"},
                {"name":"metadata","data_type":"jsonb","nullable":false,"default":"''{}''::jsonb"},
                {"name":"created_at","data_type":"timestamptz","nullable":false,"default":"now()"},
                {"name":"updated_at","data_type":"timestamptz","nullable":false,"default":"now()"}
            ]}
        ]
    }'::JSONB,
    '{"source":"migration_041_system_admin_master_detail"}'::JSONB
)
ON CONFLICT (template_key, version_key) DO UPDATE SET
    package_json = EXCLUDED.package_json,
    status = EXCLUDED.status,
    metadata = platform.schema_template_masters.metadata || EXCLUDED.metadata,
    updated_at = NOW();

INSERT INTO platform.platform_masters(module_key, entity_type, source_table, source_pk, title, status, organization_id, payload, metadata)
SELECT 'saas', 'module', 'saas_modules', module_key, display_name, CASE WHEN enabled_default THEN 'active' ELSE 'disabled' END, NULL, to_jsonb(m), '{"migrated_to_master_detail":true}'::JSONB
FROM public.saas_modules m
ON CONFLICT (source_table, source_pk) WHERE source_table <> '' AND source_pk <> ''
DO UPDATE SET payload = EXCLUDED.payload, title = EXCLUDED.title, status = EXCLUDED.status, updated_at = NOW();

INSERT INTO platform.platform_masters(module_key, entity_type, source_table, source_pk, title, status, organization_id, payload, metadata)
SELECT 'saas', 'plan', 'saas_plans', id::TEXT, code, status, NULL, to_jsonb(p), '{"migrated_to_master_detail":true}'::JSONB
FROM public.saas_plans p
ON CONFLICT (source_table, source_pk) WHERE source_table <> '' AND source_pk <> ''
DO UPDATE SET payload = EXCLUDED.payload, title = EXCLUDED.title, status = EXCLUDED.status, updated_at = NOW();

INSERT INTO platform.platform_masters(module_key, entity_type, source_table, source_pk, title, status, organization_id, payload, metadata)
SELECT 'saas', 'platform_admin', 'platform_admins', user_id::TEXT, user_id::TEXT, 'active', NULL, to_jsonb(pa), '{"migrated_to_master_detail":true}'::JSONB
FROM public.platform_admins pa
ON CONFLICT (source_table, source_pk) WHERE source_table <> '' AND source_pk <> ''
DO UPDATE SET payload = EXCLUDED.payload, title = EXCLUDED.title, status = EXCLUDED.status, updated_at = NOW();

INSERT INTO platform.platform_masters(module_key, entity_type, source_table, source_pk, title, status, organization_id, payload, metadata)
SELECT 'saas', 'organization_subscription', 'organization_subscriptions', id::TEXT, organization_id::TEXT, status, organization_id, to_jsonb(s), '{"migrated_to_master_detail":true}'::JSONB
FROM public.organization_subscriptions s
ON CONFLICT (source_table, source_pk) WHERE source_table <> '' AND source_pk <> ''
DO UPDATE SET payload = EXCLUDED.payload, title = EXCLUDED.title, status = EXCLUDED.status, organization_id = EXCLUDED.organization_id, updated_at = NOW();

INSERT INTO platform.platform_masters(module_key, entity_type, source_table, source_pk, title, status, organization_id, payload, metadata)
SELECT 'saas', 'organization_module_entitlement', 'organization_module_entitlements', organization_id::TEXT || ':' || module_key, module_key, status, organization_id, to_jsonb(e), '{"migrated_to_master_detail":true}'::JSONB
FROM public.organization_module_entitlements e
ON CONFLICT (source_table, source_pk) WHERE source_table <> '' AND source_pk <> ''
DO UPDATE SET payload = EXCLUDED.payload, title = EXCLUDED.title, status = EXCLUDED.status, organization_id = EXCLUDED.organization_id, updated_at = NOW();

INSERT INTO platform.platform_masters(module_key, entity_type, source_table, source_pk, title, status, organization_id, payload, metadata)
SELECT 'data_catalog', 'data_table', 'data_table_catalog', table_name, table_name, 'active', NULL, to_jsonb(t), '{"migrated_to_master_detail":true}'::JSONB
FROM public.data_table_catalog t
ON CONFLICT (source_table, source_pk) WHERE source_table <> '' AND source_pk <> ''
DO UPDATE SET payload = EXCLUDED.payload, title = EXCLUDED.title, updated_at = NOW();

INSERT INTO platform.platform_masters(module_key, entity_type, source_table, source_pk, title, status, organization_id, payload, metadata)
SELECT 'data_catalog', 'data_field', 'data_field_catalog', table_name || '.' || field_name, table_name || '.' || field_name, 'active', NULL, to_jsonb(f), '{"migrated_to_master_detail":true}'::JSONB
FROM public.data_field_catalog f
ON CONFLICT (source_table, source_pk) WHERE source_table <> '' AND source_pk <> ''
DO UPDATE SET payload = EXCLUDED.payload, title = EXCLUDED.title, updated_at = NOW();

INSERT INTO platform.platform_masters(module_key, entity_type, source_table, source_pk, title, status, organization_id, payload, metadata)
SELECT 'security', 'security_policy', 'security_policies', id::TEXT, policy_key, status, organization_id, to_jsonb(p), '{"migrated_to_master_detail":true}'::JSONB
FROM public.security_policies p
ON CONFLICT (source_table, source_pk) WHERE source_table <> '' AND source_pk <> ''
DO UPDATE SET payload = EXCLUDED.payload, title = EXCLUDED.title, status = EXCLUDED.status, organization_id = EXCLUDED.organization_id, updated_at = NOW();

INSERT INTO platform.platform_masters(module_key, entity_type, source_table, source_pk, title, status, organization_id, payload, metadata)
SELECT 'security', 'field_permission_rule', 'field_permission_rules', id::TEXT, table_name || '.' || field_name || ':' || action, status, organization_id, to_jsonb(r), '{"migrated_to_master_detail":true}'::JSONB
FROM public.field_permission_rules r
ON CONFLICT (source_table, source_pk) WHERE source_table <> '' AND source_pk <> ''
DO UPDATE SET payload = EXCLUDED.payload, title = EXCLUDED.title, status = EXCLUDED.status, organization_id = EXCLUDED.organization_id, updated_at = NOW();

-- -----------------------------------------------------------------------------
-- Folded from historical migration: 042_configurable_runtime_kernel.sql
-- -----------------------------------------------------------------------------

-- 042_configurable_runtime_kernel.sql
-- Configurable SaaS runtime kernel.
--
-- This is a new-baseline foundation for configurable runtime metadata and
-- per-organization record storage. It is intentionally non-destructive.

CREATE SCHEMA IF NOT EXISTS platform;

CREATE TABLE IF NOT EXISTS platform.runtime_entities (
    entity_key    TEXT PRIMARY KEY,
    module_key    TEXT NOT NULL,
    storage_table TEXT NOT NULL,
    entity_type   TEXT NOT NULL,
    title_key     TEXT NOT NULL DEFAULT '',
    status        TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'disabled', 'archived')),
    fields        JSONB NOT NULL DEFAULT '[]'
        CHECK (jsonb_typeof(fields) = 'array'),
    metadata      JSONB NOT NULL DEFAULT '{}'
        CHECK (jsonb_typeof(metadata) = 'object'),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_runtime_entities_module
    ON platform.runtime_entities(module_key, status, entity_type);

CREATE TABLE IF NOT EXISTS platform.runtime_views (
    view_key      TEXT PRIMARY KEY,
    entity_key    TEXT NOT NULL REFERENCES platform.runtime_entities(entity_key) ON DELETE CASCADE,
    view_type     TEXT NOT NULL DEFAULT 'list'
        CHECK (view_type IN ('list', 'detail', 'form', 'kanban', 'tree')),
    title_key     TEXT NOT NULL DEFAULT '',
    config        JSONB NOT NULL DEFAULT '{}'
        CHECK (jsonb_typeof(config) = 'object'),
    status        TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'disabled', 'archived')),
    metadata      JSONB NOT NULL DEFAULT '{}'
        CHECK (jsonb_typeof(metadata) = 'object'),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_runtime_views_entity
    ON platform.runtime_views(entity_key, view_type, status);

CREATE TABLE IF NOT EXISTS platform.runtime_operations (
    operation_key           TEXT PRIMARY KEY,
    domain                  TEXT NOT NULL,
    title                   TEXT NOT NULL,
    method                  TEXT NOT NULL
        CHECK (method IN ('GET', 'POST', 'PUT', 'PATCH', 'DELETE')),
    path                    TEXT NOT NULL,
    auth                    BOOLEAN NOT NULL DEFAULT TRUE,
    path_params             JSONB NOT NULL DEFAULT '[]'
        CHECK (jsonb_typeof(path_params) = 'array'),
    query_params            JSONB NOT NULL DEFAULT '[]'
        CHECK (jsonb_typeof(query_params) = 'array'),
    body_template           JSONB,
    operation_kind          TEXT NOT NULL DEFAULT 'direct'
        CHECK (operation_kind IN ('direct', 'contextual', 'agent_assisted', 'admin')),
    danger_level            TEXT NOT NULL DEFAULT 'low'
        CHECK (danger_level IN ('low', 'medium', 'high')),
    result_view             TEXT NOT NULL DEFAULT 'summary'
        CHECK (result_view IN ('summary', 'list', 'detail', 'audit')),
    assistant_eligible      BOOLEAN NOT NULL DEFAULT FALSE,
    requires_entity_context BOOLEAN NOT NULL DEFAULT FALSE,
    status                  TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'disabled', 'archived')),
    action_type             TEXT NOT NULL DEFAULT 'crud.list',
    entity_key              TEXT REFERENCES platform.runtime_entities(entity_key) ON DELETE SET NULL,
    adapter_key             TEXT NOT NULL DEFAULT '',
    metadata                JSONB NOT NULL DEFAULT '{}'
        CHECK (jsonb_typeof(metadata) = 'object'),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE platform.runtime_operations IS
    'Platform runtime operation catalog. Phase 2 industry solution factory upserts manifest runtime_operation assets here.';

CREATE INDEX IF NOT EXISTS idx_runtime_operations_domain
    ON platform.runtime_operations(domain, status, operation_key);

CREATE TABLE IF NOT EXISTS platform.runtime_i18n (
    locale     TEXT NOT NULL CHECK (locale IN ('zh', 'en')),
    label_key  TEXT NOT NULL,
    value      TEXT NOT NULL,
    metadata   JSONB NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (locale, label_key)
);

CREATE TABLE IF NOT EXISTS platform.runtime_publish_history (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    version_key  TEXT NOT NULL,
    status       TEXT NOT NULL DEFAULT 'published'
        CHECK (status IN ('draft', 'published', 'rolled_back')),
    published_by UUID REFERENCES public.users(id) ON DELETE SET NULL,
    summary      TEXT NOT NULL DEFAULT '',
    snapshot     JSONB NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS platform.runtime_audit_logs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID REFERENCES public.organizations(id) ON DELETE SET NULL,
    actor_id        UUID REFERENCES public.users(id) ON DELETE SET NULL,
    actor_type      TEXT NOT NULL DEFAULT '',
    operation_key   TEXT NOT NULL DEFAULT '',
    entity_key      TEXT NOT NULL DEFAULT '',
    record_key      TEXT NOT NULL DEFAULT '',
    action_type     TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'ok',
    payload         JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_runtime_audit_logs_org
    ON platform.runtime_audit_logs(organization_id, created_at DESC);

CREATE OR REPLACE FUNCTION platform.runtime_module_key(p_source_module TEXT)
RETURNS TEXT
LANGUAGE sql
IMMUTABLE
AS $$
    SELECT CASE p_source_module
        WHEN 'project_lifecycle' THEN 'project'
        WHEN 'aigateway' THEN 'ai_gateway'
        WHEN 'metaresource' THEN 'meta_resource'
        ELSE p_source_module
    END;
$$;

INSERT INTO platform.runtime_i18n(locale, label_key, value)
VALUES
    ('en', 'operation.param.limit', 'Limit'),
    ('zh', 'operation.param.limit', '数量'),
    ('en', 'operation.param.recordKey', 'Record key'),
    ('zh', 'operation.param.recordKey', '记录编号'),
    ('en', 'runtime.field.title', 'Title'),
    ('zh', 'runtime.field.title', '标题'),
    ('en', 'runtime.field.status', 'Status'),
    ('zh', 'runtime.field.status', '状态'),
    ('en', 'runtime.field.coreData', 'Core data'),
    ('zh', 'runtime.field.coreData', '核心数据')
ON CONFLICT (locale, label_key) DO UPDATE SET
    value = EXCLUDED.value,
    updated_at = NOW();

CREATE OR REPLACE FUNCTION platform.provision_runtime_organization(p_organization_id UUID)
RETURNS VOID
LANGUAGE plpgsql
AS $$
DECLARE
    v_schema_name TEXT := platform.organization_schema_name(p_organization_id);
    rec RECORD;
BEGIN
    EXECUTE FORMAT('CREATE SCHEMA IF NOT EXISTS %I', v_schema_name);

    FOR rec IN
        SELECT DISTINCT module_key, storage_table
        FROM platform.runtime_entities
        WHERE status = 'active'
        ORDER BY module_key, storage_table
    LOOP
        EXECUTE FORMAT(
            'CREATE TABLE IF NOT EXISTS %I.%I (
                master_key TEXT PRIMARY KEY,
                entity_type TEXT NOT NULL,
                title TEXT NOT NULL DEFAULT '''',
                status TEXT NOT NULL DEFAULT ''active'',
                parent_master_key TEXT NOT NULL DEFAULT '''',
                core_data JSONB NOT NULL DEFAULT ''{}''::jsonb,
                metadata JSONB NOT NULL DEFAULT ''{}''::jsonb,
                created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
            )',
            v_schema_name,
            rec.storage_table
        );
        EXECUTE FORMAT('CREATE INDEX IF NOT EXISTS %I ON %I.%I(entity_type, status, updated_at DESC)',
            'idx_' || rec.storage_table || '_entity_status',
            v_schema_name,
            rec.storage_table
        );
    END LOOP;

    INSERT INTO platform.organization_schema_targets(
        organization_id, schema_name, template_version, status, metadata
    )
    VALUES (
        p_organization_id,
        v_schema_name,
        'meta-org.runtime.v1',
        'provisioned',
        '{"source":"runtime_kernel"}'::jsonb
    )
    ON CONFLICT (organization_id) DO UPDATE SET
        schema_name = EXCLUDED.schema_name,
        template_version = EXCLUDED.template_version,
        status = 'provisioned',
        metadata = platform.organization_schema_targets.metadata || EXCLUDED.metadata,
        updated_at = NOW();
END;
$$;

SELECT platform.provision_runtime_organization(id)
FROM public.organizations;


-- -----------------------------------------------------------------------------
-- Folded from historical migration: 043_saas_admin_permissions_schema_changes.sql
-- -----------------------------------------------------------------------------

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


-- -----------------------------------------------------------------------------
-- Folded from historical migration: 044_industry_dimension.sql
-- -----------------------------------------------------------------------------

CREATE SCHEMA IF NOT EXISTS platform;

CREATE TABLE IF NOT EXISTS platform.industries (
    industry_key TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'active', 'archived')),
    metadata     JSONB NOT NULL DEFAULT '{}'
        CHECK (jsonb_typeof(metadata) = 'object'),
    created_by   UUID REFERENCES public.users(id) ON DELETE SET NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS platform.custom_packages (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    industry_key TEXT NOT NULL REFERENCES platform.industries(industry_key) ON DELETE CASCADE,
    package_key  TEXT NOT NULL,
    version      INT NOT NULL DEFAULT 1,
    name         TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'active', 'archived')),
    metadata     JSONB NOT NULL DEFAULT '{}'
        CHECK (jsonb_typeof(metadata) = 'object'),
    created_by   UUID REFERENCES public.users(id) ON DELETE SET NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (industry_key, package_key, version)
);

CREATE INDEX IF NOT EXISTS idx_custom_packages_industry
    ON platform.custom_packages(industry_key, status, version DESC);

CREATE TABLE IF NOT EXISTS platform.custom_package_assets (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    package_id UUID NOT NULL REFERENCES platform.custom_packages(id) ON DELETE CASCADE,
    asset_key  TEXT NOT NULL,
    asset_type TEXT NOT NULL
        CHECK (asset_type IN (
            'schema_package', 'module', 'runtime_entity', 'runtime_operation',
            'skill_structure', 'skill', 'knowledge_source', 'model_policy', 'i18n'
        )),
    payload    JSONB NOT NULL DEFAULT '{}',
    metadata   JSONB NOT NULL DEFAULT '{}'
        CHECK (jsonb_typeof(metadata) = 'object'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (package_id, asset_key)
);

CREATE INDEX IF NOT EXISTS idx_custom_package_assets_type
    ON platform.custom_package_assets(package_id, asset_type, asset_key);

CREATE TABLE IF NOT EXISTS platform.organization_industry_adoptions (
    organization_id UUID PRIMARY KEY REFERENCES public.organizations(id) ON DELETE CASCADE,
    industry_key    TEXT NOT NULL REFERENCES platform.industries(industry_key) ON DELETE RESTRICT,
    package_id      UUID NOT NULL REFERENCES platform.custom_packages(id) ON DELETE RESTRICT,
    is_primary      BOOLEAN NOT NULL DEFAULT TRUE,
    enabled_modules JSONB NOT NULL DEFAULT '[]'
        CHECK (jsonb_typeof(enabled_modules) = 'array'),
    status          TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'archived')),
    metadata        JSONB NOT NULL DEFAULT '{}'
        CHECK (jsonb_typeof(metadata) = 'object'),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_organization_industry_adoptions_industry
    ON platform.organization_industry_adoptions(industry_key, status);

CREATE TABLE IF NOT EXISTS platform.organization_industry_extensions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES public.organizations(id) ON DELETE CASCADE,
    industry_key    TEXT NOT NULL REFERENCES platform.industries(industry_key) ON DELETE RESTRICT,
    package_id      UUID REFERENCES platform.custom_packages(id) ON DELETE SET NULL,
    extension_key   TEXT NOT NULL,
    name            TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'active', 'archived')),
    assets          JSONB NOT NULL DEFAULT '[]'
        CHECK (jsonb_typeof(assets) = 'array'),
    metadata        JSONB NOT NULL DEFAULT '{}'
        CHECK (jsonb_typeof(metadata) = 'object'),
    created_by      UUID REFERENCES public.users(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, extension_key)
);

CREATE INDEX IF NOT EXISTS idx_organization_industry_extensions_lookup
    ON platform.organization_industry_extensions(organization_id, industry_key, status);

CREATE TABLE IF NOT EXISTS platform.custom_package_publication_requests (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    extension_id           UUID NOT NULL REFERENCES platform.organization_industry_extensions(id) ON DELETE CASCADE,
    source_organization_id UUID NOT NULL REFERENCES public.organizations(id) ON DELETE CASCADE,
    industry_key            TEXT NOT NULL REFERENCES platform.industries(industry_key) ON DELETE RESTRICT,
    status                  TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'approved', 'rejected')),
    reason                  TEXT NOT NULL DEFAULT '',
    review_reason           TEXT NOT NULL DEFAULT '',
    requested_by            UUID REFERENCES public.users(id) ON DELETE SET NULL,
    reviewed_by             UUID REFERENCES public.users(id) ON DELETE SET NULL,
    reviewed_at             TIMESTAMPTZ,
    metadata                JSONB NOT NULL DEFAULT '{}'
        CHECK (jsonb_typeof(metadata) = 'object'),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_custom_package_publication_requests_status
    ON platform.custom_package_publication_requests(status, created_at DESC);

CREATE TABLE IF NOT EXISTS platform.knowledge_sources (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    industry_key      TEXT NOT NULL REFERENCES platform.industries(industry_key) ON DELETE CASCADE,
    organization_id   UUID REFERENCES public.organizations(id) ON DELETE CASCADE,
    source_key        TEXT NOT NULL,
    name              TEXT NOT NULL,
    source_type       TEXT NOT NULL DEFAULT 'external_reference',
    adapter_key       TEXT NOT NULL DEFAULT '',
    reference_uri     TEXT NOT NULL DEFAULT '',
    sync_status       TEXT NOT NULL DEFAULT 'not_configured'
        CHECK (sync_status IN ('not_configured', 'ready', 'syncing', 'error', 'disabled')),
    permission        JSONB NOT NULL DEFAULT '{}'
        CHECK (jsonb_typeof(permission) = 'object'),
    retrieval_config  JSONB NOT NULL DEFAULT '{}'
        CHECK (jsonb_typeof(retrieval_config) = 'object'),
    metadata          JSONB NOT NULL DEFAULT '{}'
        CHECK (jsonb_typeof(metadata) = 'object'),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (industry_key, organization_id, source_key)
);

CREATE INDEX IF NOT EXISTS idx_knowledge_sources_scope
    ON platform.knowledge_sources(industry_key, organization_id, sync_status);

INSERT INTO platform.industries(industry_key, name, description, status, metadata)
VALUES
    ('general', 'General Business', 'Default cross-industry SaaS management baseline', 'active', '{"seed":true}'::jsonb),
    ('manufacturing', 'Manufacturing', 'Manufacturing and supply-chain operating baseline', 'active', '{"seed":true}'::jsonb)
ON CONFLICT (industry_key) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    status = EXCLUDED.status,
    metadata = platform.industries.metadata || EXCLUDED.metadata,
    updated_at = NOW();

WITH foundation_package AS (
    INSERT INTO platform.custom_packages(industry_key, package_key, version, name, description, status, metadata)
    VALUES (
        'general',
        'general-foundation',
        1,
        'General Foundation',
        'Default package for organization, project, workflow, governance, assistant, finance, and runtime foundations',
        'active',
        '{"seed":true}'::jsonb
    )
    ON CONFLICT (industry_key, package_key, version) DO UPDATE SET
        name = EXCLUDED.name,
        description = EXCLUDED.description,
        status = EXCLUDED.status,
        metadata = platform.custom_packages.metadata || EXCLUDED.metadata,
        updated_at = NOW()
    RETURNING id
)
INSERT INTO platform.custom_package_assets(package_id, asset_key, asset_type, payload, metadata)
SELECT id, asset_key, 'module', jsonb_build_object('module_key', module_key, 'display_name', display_name), '{"seed":true}'::jsonb
FROM foundation_package
CROSS JOIN (VALUES
    ('organization', 'Organization'),
    ('project', 'Project Operations'),
    ('workflow', 'Workflow'),
    ('governance', 'Governance'),
    ('evolution', 'Evolution'),
    ('capability', 'Capability'),
    ('meta_resource', 'Meta Resource'),
    ('assistant', 'AI Assistant'),
    ('ai_gateway', 'AI Gateway'),
    ('toolruntime', 'Tool Runtime'),
    ('finance', 'Finance'),
    ('costing', 'Costing'),
    ('developer_tools', 'Developer Tools')
) AS modules(module_key, display_name)
CROSS JOIN LATERAL (SELECT module_key || '-module' AS asset_key) asset_keys
ON CONFLICT (package_id, asset_key) DO UPDATE SET
    payload = EXCLUDED.payload,
    metadata = platform.custom_package_assets.metadata || EXCLUDED.metadata,
    updated_at = NOW();

WITH manufacturing_package AS (
    INSERT INTO platform.custom_packages(industry_key, package_key, version, name, description, status, metadata)
    VALUES (
        'manufacturing',
        'manufacturing-supply-chain',
        1,
        'Manufacturing Supply Chain',
        'Manufacturing package with inventory, procurement, sales, finance, costing, assistant, and runtime capabilities',
        'active',
        '{"seed":true}'::jsonb
    )
    ON CONFLICT (industry_key, package_key, version) DO UPDATE SET
        name = EXCLUDED.name,
        description = EXCLUDED.description,
        status = EXCLUDED.status,
        metadata = platform.custom_packages.metadata || EXCLUDED.metadata,
        updated_at = NOW()
    RETURNING id
)
INSERT INTO platform.custom_package_assets(package_id, asset_key, asset_type, payload, metadata)
SELECT id, asset_key, 'module', jsonb_build_object('module_key', module_key, 'display_name', display_name), '{"seed":true}'::jsonb
FROM manufacturing_package
CROSS JOIN (VALUES
    ('organization', 'Organization'),
    ('inventory', 'Inventory'),
    ('procurement', 'Procurement'),
    ('sales', 'Sales'),
    ('finance', 'Finance'),
    ('costing', 'Costing'),
    ('assistant', 'AI Assistant'),
    ('ai_gateway', 'AI Gateway'),
    ('toolruntime', 'Tool Runtime')
) AS modules(module_key, display_name)
CROSS JOIN LATERAL (SELECT module_key || '-module' AS asset_key) asset_keys
ON CONFLICT (package_id, asset_key) DO UPDATE SET
    payload = EXCLUDED.payload,
    metadata = platform.custom_package_assets.metadata || EXCLUDED.metadata,
    updated_at = NOW();





