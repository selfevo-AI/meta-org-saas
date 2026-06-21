package platformauth

import "strings"

const (
	RoleOwner    = "owner"
	RoleAdmin    = "admin"
	RoleOperator = "operator"
	RoleAuditor  = "auditor"

	PermissionPlatformRead       = "platform.read"
	PermissionOrganizationManage = "organization.manage"
	PermissionOrganizationClose  = "organization.close"
	PermissionSchemaManage       = "schema.manage"
	PermissionSchemaApprove      = "schema.approve"
	PermissionSchemaApply        = "schema.apply"
	PermissionModelManage        = "model.manage"
	PermissionRuntimeManage      = "runtime.manage"
	PermissionAssistantRun       = "assistant.platform.run"
)

var rolePermissions = map[string][]string{
	RoleOwner: {
		PermissionPlatformRead,
		PermissionOrganizationManage,
		PermissionOrganizationClose,
		PermissionSchemaManage,
		PermissionSchemaApprove,
		PermissionSchemaApply,
		PermissionModelManage,
		PermissionRuntimeManage,
		PermissionAssistantRun,
	},
	RoleAdmin: {
		PermissionPlatformRead,
		PermissionOrganizationManage,
		PermissionSchemaManage,
		PermissionSchemaApprove,
		PermissionSchemaApply,
		PermissionModelManage,
		PermissionRuntimeManage,
		PermissionAssistantRun,
	},
	RoleOperator: {
		PermissionPlatformRead,
		PermissionOrganizationManage,
		PermissionSchemaManage,
		PermissionModelManage,
		PermissionRuntimeManage,
		PermissionAssistantRun,
	},
	RoleAuditor: {
		PermissionPlatformRead,
	},
}

func NormalizeRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "system_owner", "platform_owner", "super_admin", "owner":
		return RoleOwner
	case "system_admin", "platform_admin", "admin":
		return RoleAdmin
	case "operator", "ops":
		return RoleOperator
	case "auditor", "viewer", "readonly", "read_only":
		return RoleAuditor
	default:
		return strings.ToLower(strings.TrimSpace(role))
	}
}

func PermissionsForRole(role string) map[string]bool {
	normalized := NormalizeRole(role)
	result := map[string]bool{}
	for _, permission := range rolePermissions[normalized] {
		result[permission] = true
	}
	return result
}

func HasPermission(role string, permission string) bool {
	return PermissionsForRole(role)[permission]
}
