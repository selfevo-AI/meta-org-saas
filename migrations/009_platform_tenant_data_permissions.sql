-- Platform-to-tenant data access permissions.
-- Existing databases need this repair because platform baseline files are tracked
-- by filename and may already have been applied before these permissions existed.

INSERT INTO platform.platform_permissions(permission_key, name, description, category, status, metadata)
VALUES
    ('tenant.data.read', 'Read tenant business data', 'Read tenant business workspaces through an explicitly selected organization context', 'tenant_data', 'active', '{"seed":true,"repair":"009"}'::jsonb),
    ('tenant.data.manage', 'Manage tenant business data', 'Create and mutate tenant business records through an explicitly selected organization context', 'tenant_data', 'active', '{"seed":true,"repair":"009"}'::jsonb)
ON CONFLICT (permission_key) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    category = EXCLUDED.category,
    status = EXCLUDED.status,
    metadata = platform.platform_permissions.metadata || EXCLUDED.metadata,
    updated_at = NOW();

WITH role_permission_matrix(role_key, permission_key) AS (
    VALUES
        ('owner', 'tenant.data.read'),
        ('owner', 'tenant.data.manage'),
        ('admin', 'tenant.data.read'),
        ('admin', 'tenant.data.manage'),
        ('operator', 'tenant.data.read'),
        ('operator', 'tenant.data.manage'),
        ('auditor', 'tenant.data.read')
)
INSERT INTO platform.platform_role_permissions(role_key, permission_key, status)
SELECT role_key, permission_key, 'active'
FROM role_permission_matrix
ON CONFLICT (role_key, permission_key) DO UPDATE SET
    status = EXCLUDED.status,
    updated_at = NOW();
