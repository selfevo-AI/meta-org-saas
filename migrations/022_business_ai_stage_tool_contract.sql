-- Enforce executable five-stage AI through governed project lifecycle tools.
-- platformdb:accept-checksum-drift 004_ai_capability_baseline.sql

INSERT INTO tool_definitions(
    name, description, source_type, default_policy, risk_level, required_level,
    tool_category, approval_tier_required, metadata
)
VALUES
    ('project.update_status', 'Update project lifecycle status', 'internal_api', 'approve', 'high', 'L3', 'business_approval', 'reviewer',
     '{"label_zh":"更新项目生命周期状态","label_en":"Update project lifecycle status","description_zh":"更新项目生命周期状态","description_en":"Update project lifecycle status","business_ai_stages":["do","change"]}'::JSONB),
    ('project.create_deliverable', 'Create a project deliverable', 'internal_api', 'approve', 'high', 'L3', 'business_approval', 'reviewer',
     '{"label_zh":"创建项目交付物","label_en":"Create a project deliverable","description_zh":"创建项目交付物","description_en":"Create a project deliverable","business_ai_stages":["do"]}'::JSONB),
    ('project.accept_deliverable', 'Accept a submitted project deliverable', 'internal_api', 'approve', 'high', 'L3', 'business_approval', 'reviewer',
     '{"label_zh":"验收已提交的项目交付物","label_en":"Accept a submitted project deliverable","description_zh":"验收已提交的项目交付物","description_en":"Accept a submitted project deliverable","business_ai_stages":["accept"]}'::JSONB),
    ('project.close_feedback', 'Close the project feedback loop', 'internal_api', 'approve', 'high', 'L3', 'business_approval', 'reviewer',
     '{"label_zh":"关闭项目反馈闭环","label_en":"Close the project feedback loop","description_zh":"关闭项目反馈闭环","description_en":"Close the project feedback loop","business_ai_stages":["accept"]}'::JSONB)
ON CONFLICT (name) DO UPDATE SET
    description = EXCLUDED.description,
    source_type = EXCLUDED.source_type,
    default_policy = EXCLUDED.default_policy,
    risk_level = EXCLUDED.risk_level,
    required_level = EXCLUDED.required_level,
    tool_category = EXCLUDED.tool_category,
    approval_tier_required = EXCLUDED.approval_tier_required,
    metadata = tool_definitions.metadata || EXCLUDED.metadata,
    is_active = true,
    updated_at = NOW();

UPDATE tool_definitions
SET metadata = metadata || CASE name
        WHEN 'project.match_members' THEN '{"business_ai_stages":["plan","change"]}'::JSONB
        WHEN 'project.bind_workflow' THEN '{"business_ai_stages":["plan","change"]}'::JSONB
        WHEN 'project.estimate_cost' THEN '{"business_ai_stages":["plan","do","accept"]}'::JSONB
        WHEN 'project.create_cost_entry' THEN '{"business_ai_stages":["do"]}'::JSONB
        WHEN 'evolution.create_knowledge' THEN '{"business_ai_stages":["learn"]}'::JSONB
        WHEN 'evolution.create_signal' THEN '{"business_ai_stages":["learn"]}'::JSONB
        WHEN 'evolution.propose_experiment' THEN '{"business_ai_stages":["learn"]}'::JSONB
        ELSE '{}'::JSONB
    END,
    updated_at = NOW()
WHERE name IN (
    'project.match_members', 'project.bind_workflow', 'project.estimate_cost', 'project.create_cost_entry',
    'evolution.create_knowledge', 'evolution.create_signal', 'evolution.propose_experiment'
);
