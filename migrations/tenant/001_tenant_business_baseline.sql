-- Tenant physical database baseline.
-- This file creates tenant-local runtime projections and then includes the
-- ERP/project/workflow/costing/finance/inventory/procurement/sales baseline.

CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE SCHEMA IF NOT EXISTS platform;

DO $$ BEGIN
    CREATE TYPE role_type AS ENUM ('planner', 'executor', 'reviewer');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL DEFAULT '',
    avatar_url TEXT,
    account_status TEXT NOT NULL DEFAULT 'active',
    onboarding_status TEXT NOT NULL DEFAULT 'complete',
    default_organization_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL UNIQUE,
    role_type role_type NOT NULL DEFAULT 'executor',
    description TEXT,
    permissions JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE users
    ADD CONSTRAINT fk_users_default_organization
    FOREIGN KEY (default_organization_id) REFERENCES organizations(id) ON DELETE SET NULL;

CREATE TABLE IF NOT EXISTS muvrs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    boundary JSONB NOT NULL DEFAULT '{"data_permissions":[],"resource_quota":{},"network_policies":[]}',
    config JSONB NOT NULL DEFAULT '{}',
    parent_id UUID REFERENCES muvrs(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS departments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    parent_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    code TEXT,
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    sort_order INT NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_department_not_self_parent CHECK (parent_id IS NULL OR parent_id <> id)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_departments_org_code
    ON departments(organization_id, code)
    WHERE code IS NOT NULL AND code <> '';
CREATE INDEX IF NOT EXISTS idx_departments_org_parent ON departments(organization_id, parent_id, sort_order);

CREATE TABLE IF NOT EXISTS external_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    email TEXT,
    vendor TEXT NOT NULL DEFAULT '',
    contract_type TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_external_members_email
    ON external_members(lower(email))
    WHERE email IS NOT NULL AND email <> '';

CREATE TABLE IF NOT EXISTS organization_memberships (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    department_id UUID NOT NULL REFERENCES departments(id) ON DELETE CASCADE,
    member_type TEXT NOT NULL CHECK (member_type IN ('internal', 'external', 'agent')),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    external_member_id UUID REFERENCES external_members(id) ON DELETE CASCADE,
    agent_id UUID,
    title TEXT NOT NULL DEFAULT '',
    role_id UUID REFERENCES roles(id) ON DELETE SET NULL,
    authority_tier TEXT NOT NULL DEFAULT 'executor',
    status TEXT NOT NULL DEFAULT 'active',
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_org_membership_one_actor CHECK (
        (member_type = 'internal' AND user_id IS NOT NULL AND external_member_id IS NULL AND agent_id IS NULL) OR
        (member_type = 'external' AND user_id IS NULL AND external_member_id IS NOT NULL AND agent_id IS NULL) OR
        (member_type = 'agent' AND user_id IS NULL AND external_member_id IS NULL AND agent_id IS NOT NULL)
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_org_membership_internal
    ON organization_memberships(department_id, user_id)
    WHERE user_id IS NOT NULL AND status <> 'archived';
CREATE UNIQUE INDEX IF NOT EXISTS uq_org_membership_external
    ON organization_memberships(department_id, external_member_id)
    WHERE external_member_id IS NOT NULL AND status <> 'archived';
CREATE UNIQUE INDEX IF NOT EXISTS uq_org_membership_agent
    ON organization_memberships(department_id, agent_id)
    WHERE agent_id IS NOT NULL AND status <> 'archived';
CREATE INDEX IF NOT EXISTS idx_org_memberships_org ON organization_memberships(organization_id);
CREATE INDEX IF NOT EXISTS idx_org_memberships_department ON organization_memberships(department_id);

CREATE TABLE IF NOT EXISTS department_mvru_links (
    department_id UUID NOT NULL REFERENCES departments(id) ON DELETE CASCADE,
    mvru_id UUID NOT NULL REFERENCES muvrs(id) ON DELETE CASCADE,
    link_type TEXT NOT NULL DEFAULT 'execution',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (department_id, mvru_id, link_type)
);

CREATE TABLE IF NOT EXISTS capabilities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    version VARCHAR(50) NOT NULL DEFAULT '1.0',
    description TEXT,
    input_schema JSONB NOT NULL DEFAULT '{}',
    output_schema JSONB NOT NULL DEFAULT '{}',
    preconditions JSONB NOT NULL DEFAULT '[]',
    error_handling JSONB NOT NULL DEFAULT '{}',
    permission_level TEXT NOT NULL DEFAULT 'L2',
    cost_estimate JSONB NOT NULL DEFAULT '{}',
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (name, version)
);

CREATE TABLE IF NOT EXISTS workflow_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
    department_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    stages JSONB NOT NULL DEFAULT '[]',
    assignee_type VARCHAR(10) NOT NULL DEFAULT 'either',
    required_weight NUMERIC(5,2) DEFAULT 0,
    routing_rules JSONB NOT NULL DEFAULT '{}',
    visual_graph JSONB NOT NULL DEFAULT '{}',
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS workflow_instances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id UUID NOT NULL REFERENCES workflow_templates(id),
    organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
    department_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    project_id UUID,
    status TEXT NOT NULL DEFAULT 'active',
    current_stage INT NOT NULL DEFAULT 0,
    context JSONB NOT NULL DEFAULT '{}',
    trace_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id UUID NOT NULL REFERENCES workflow_instances(id) ON DELETE CASCADE,
    stage INT NOT NULL,
    stage_type TEXT NOT NULL,
    assignee_id UUID,
    assignee_type VARCHAR(32),
    input JSONB,
    output JSONB,
    weight_snapshot NUMERIC(5,2),
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS decisions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    decision_maker_id UUID NOT NULL,
    maker_type VARCHAR(32) NOT NULL,
    weight NUMERIC(5,2) DEFAULT 0,
    input JSONB,
    output JSONB,
    reasoning TEXT,
    outcome VARCHAR(50),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS workflow_contexts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id UUID NOT NULL REFERENCES workflow_instances(id) ON DELETE CASCADE,
    working_memory JSONB NOT NULL DEFAULT '{}',
    injected_experience JSONB NOT NULL DEFAULT '[]',
    principle_notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (workflow_id)
);

CREATE TABLE IF NOT EXISTS meta_resources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    department_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    resource_type TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    source_type TEXT NOT NULL DEFAULT '',
    source_id UUID,
    status TEXT NOT NULL DEFAULT 'active',
    capabilities JSONB NOT NULL DEFAULT '[]',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS positions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    department_id UUID NOT NULL REFERENCES departments(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    code TEXT,
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    sort_order INT NOT NULL DEFAULT 0,
    permission_level TEXT NOT NULL DEFAULT 'L1',
    required_capabilities JSONB NOT NULL DEFAULT '[]',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_positions_department_code
    ON positions(department_id, code)
    WHERE code IS NOT NULL AND code <> '';
CREATE INDEX IF NOT EXISTS idx_positions_org_department ON positions(organization_id, department_id, status, sort_order);

CREATE TABLE IF NOT EXISTS position_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    position_id UUID NOT NULL REFERENCES positions(id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    department_id UUID NOT NULL REFERENCES departments(id) ON DELETE CASCADE,
    meta_resource_id UUID REFERENCES meta_resources(id) ON DELETE SET NULL,
    actor_id UUID NOT NULL,
    actor_type TEXT NOT NULL,
    assignment_type TEXT NOT NULL DEFAULT 'candidate',
    allocation_percent NUMERIC(5,2) NOT NULL DEFAULT 100,
    status TEXT NOT NULL DEFAULT 'active',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_position_assignment_actor
    ON position_assignments(position_id, actor_id, actor_type)
    WHERE status <> 'archived';
CREATE INDEX IF NOT EXISTS idx_position_assignments_position ON position_assignments(position_id, status);
CREATE INDEX IF NOT EXISTS idx_position_assignments_actor ON position_assignments(actor_id, actor_type, status);

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
    actor_type TEXT NOT NULL,
    role_in_task TEXT NOT NULL DEFAULT 'responsible',
    allocation_percent NUMERIC(5,2) NOT NULL DEFAULT 100,
    status TEXT NOT NULL DEFAULT 'active',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_task_matrix_assignment_role
    ON task_matrix_assignments(task_id, position_id, meta_resource_id, role_in_task)
    WHERE status <> 'archived';

CREATE TABLE IF NOT EXISTS access_decisions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id UUID NOT NULL,
    actor_type TEXT NOT NULL,
    action TEXT NOT NULL,
    resource TEXT NOT NULL,
    resource_id UUID,
    organization_id UUID,
    department_id UUID,
    workflow_id UUID REFERENCES workflow_instances(id) ON DELETE SET NULL,
    task_id UUID REFERENCES tasks(id) ON DELETE SET NULL,
    capability_id UUID REFERENCES capabilities(id) ON DELETE SET NULL,
    required_level TEXT NOT NULL DEFAULT 'L1',
    risk_level TEXT NOT NULL DEFAULT 'low',
    decision TEXT NOT NULL DEFAULT 'allow',
    allowed BOOLEAN NOT NULL DEFAULT true,
    behavior TEXT NOT NULL DEFAULT 'allow',
    reason TEXT NOT NULL DEFAULT '',
    matched_rules JSONB NOT NULL DEFAULT '[]',
    weight_snapshot DOUBLE PRECISION,
    context JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS context_weight_scores (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id UUID NOT NULL,
    actor_type TEXT NOT NULL,
    scope_hash TEXT NOT NULL,
    organization_id UUID,
    department_id UUID,
    workflow_template_id UUID REFERENCES workflow_templates(id) ON DELETE SET NULL,
    workflow_stage TEXT,
    task_type TEXT,
    capability_id UUID REFERENCES capabilities(id) ON DELETE SET NULL,
    risk_level TEXT NOT NULL DEFAULT 'low',
    quality_score DOUBLE PRECISION NOT NULL DEFAULT 0.5,
    latency_score DOUBLE PRECISION NOT NULL DEFAULT 0.5,
    risk_score DOUBLE PRECISION NOT NULL DEFAULT 0.5,
    compliance_score DOUBLE PRECISION NOT NULL DEFAULT 0.5,
    overall_score DOUBLE PRECISION NOT NULL DEFAULT 0.5,
    evidence JSONB NOT NULL DEFAULT '{}',
    conclusion TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS permission_change_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    membership_id UUID NOT NULL REFERENCES organization_memberships(id) ON DELETE CASCADE,
    requested_by UUID REFERENCES users(id) ON DELETE SET NULL,
    requested_by_type TEXT NOT NULL DEFAULT 'human',
    requested_change JSONB NOT NULL DEFAULT '{}',
    reason TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    review_reason TEXT NOT NULL DEFAULT '',
    reviewed_at TIMESTAMPTZ,
    applied_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS organization_access_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    scope_type TEXT NOT NULL DEFAULT 'organization',
    scope_id TEXT NOT NULL DEFAULT '',
    resource_type TEXT NOT NULL,
    resource_key TEXT NOT NULL DEFAULT '',
    action TEXT NOT NULL,
    actor_type TEXT NOT NULL DEFAULT 'any',
    actor_id TEXT NOT NULL DEFAULT '',
    role_id UUID REFERENCES roles(id) ON DELETE SET NULL,
    authority_tier TEXT NOT NULL DEFAULT '',
    behavior TEXT NOT NULL DEFAULT 'allow',
    required_level TEXT NOT NULL DEFAULT 'L1',
    priority INT NOT NULL DEFAULT 100,
    status TEXT NOT NULL DEFAULT 'active',
    reason TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS data_table_catalog (
    table_name TEXT PRIMARY KEY,
    master_table_name TEXT NOT NULL,
    detail_table_name TEXT NOT NULL,
    key_prefix TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    category TEXT NOT NULL DEFAULT 'system',
    is_base_data BOOLEAN NOT NULL DEFAULT false,
    is_business_scenario BOOLEAN NOT NULL DEFAULT false,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS data_field_catalog (
    table_name TEXT NOT NULL REFERENCES data_table_catalog(table_name) ON DELETE CASCADE,
    field_name TEXT NOT NULL,
    data_type TEXT NOT NULL DEFAULT '',
    display_name TEXT NOT NULL DEFAULT '',
    is_master_key BOOLEAN NOT NULL DEFAULT false,
    is_sub_key BOOLEAN NOT NULL DEFAULT false,
    is_visible_default BOOLEAN NOT NULL DEFAULT true,
    permission_level TEXT NOT NULL DEFAULT 'L1',
    display_order INT NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (table_name, field_name)
);

CREATE TABLE IF NOT EXISTS business_key_sequences (
    entity_name TEXT NOT NULL,
    date_key TEXT NOT NULL,
    next_value BIGINT NOT NULL DEFAULT 1,
    PRIMARY KEY (entity_name, date_key)
);

CREATE OR REPLACE FUNCTION next_business_key(p_entity_name TEXT, p_prefix TEXT DEFAULT NULL)
RETURNS TEXT
LANGUAGE plpgsql
AS $$
DECLARE
    v_date_key TEXT := TO_CHAR(NOW(), 'YYYYMMDD');
    v_next BIGINT;
    v_prefix TEXT := COALESCE(NULLIF(p_prefix, ''), upper(substr(p_entity_name, 1, 3)));
BEGIN
    INSERT INTO business_key_sequences(entity_name, date_key, next_value)
    VALUES (p_entity_name, v_date_key, 2)
    ON CONFLICT (entity_name, date_key)
    DO UPDATE SET next_value = business_key_sequences.next_value + 1
    RETURNING next_value - 1 INTO v_next;
    RETURN v_prefix || '-' || v_date_key || '-' || LPAD(v_next::TEXT, 6, '0');
END;
$$;

CREATE TABLE IF NOT EXISTS saas_modules (
    module_key TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    category TEXT NOT NULL DEFAULT 'business',
    enabled_default BOOLEAN NOT NULL DEFAULT true,
    license_scope TEXT NOT NULL DEFAULT 'commercial',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sample_work_orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    work_order_no TEXT NOT NULL UNIQUE,
    product_name TEXT NOT NULL,
    quantity NUMERIC(18,4) NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'planned',
    planned_start_at TIMESTAMPTZ,
    planned_finish_at TIMESTAMPTZ,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- tenantdb:include ../001_erp_code_baseline.sql
