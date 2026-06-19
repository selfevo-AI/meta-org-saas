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
    agent_id UUID REFERENCES ai_agents(id) ON DELETE CASCADE,
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
