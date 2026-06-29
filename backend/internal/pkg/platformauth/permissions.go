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
	PermissionModelManage        = "model.manage"
	PermissionRuntimeManage      = "runtime.manage"
	PermissionAssistantRun       = "assistant.platform.run"

	PermissionPlatformFeatureManage       = "platform.feature.manage"
	PermissionPlatformUserManage          = "platform.user.manage"
	PermissionPlatformRBACManage          = "platform.rbac.manage"
	PermissionDatabaseMaintenanceManage   = "database.maintenance.manage"
	PermissionDatabaseMaintenanceApprove  = "database.maintenance.approve"
	PermissionAPIManage                   = "api.manage"
	PermissionIndustrySolutionManage      = "industry.solution.manage"
	PermissionIndustrySolutionImport      = "industry.solution.import"
	PermissionIndustrySolutionExport      = "industry.solution.export"
	PermissionIndustrySolutionVerify      = "industry.solution.verify"
	PermissionIndustrySolutionApprove     = "industry.solution.approve"
	PermissionIndustrySolutionApply       = "industry.solution.apply"
	PermissionTenantIndustrySolutionApply = "tenant.industry_solution.apply"
)

var rolePermissions = map[string][]string{
	RoleOwner: {
		PermissionPlatformRead,
		PermissionOrganizationManage,
		PermissionOrganizationClose,
		PermissionModelManage,
		PermissionRuntimeManage,
		PermissionAssistantRun,
		PermissionPlatformFeatureManage,
		PermissionPlatformUserManage,
		PermissionPlatformRBACManage,
		PermissionDatabaseMaintenanceManage,
		PermissionDatabaseMaintenanceApprove,
		PermissionAPIManage,
		PermissionIndustrySolutionManage,
		PermissionIndustrySolutionImport,
		PermissionIndustrySolutionExport,
		PermissionIndustrySolutionVerify,
		PermissionIndustrySolutionApprove,
		PermissionIndustrySolutionApply,
		PermissionTenantIndustrySolutionApply,
	},
	RoleAdmin: {
		PermissionPlatformRead,
		PermissionOrganizationManage,
		PermissionModelManage,
		PermissionRuntimeManage,
		PermissionAssistantRun,
		PermissionPlatformFeatureManage,
		PermissionPlatformUserManage,
		PermissionPlatformRBACManage,
		PermissionDatabaseMaintenanceManage,
		PermissionDatabaseMaintenanceApprove,
		PermissionAPIManage,
		PermissionIndustrySolutionManage,
		PermissionIndustrySolutionImport,
		PermissionIndustrySolutionExport,
		PermissionIndustrySolutionVerify,
		PermissionIndustrySolutionApprove,
		PermissionIndustrySolutionApply,
		PermissionTenantIndustrySolutionApply,
	},
	RoleOperator: {
		PermissionPlatformRead,
		PermissionOrganizationManage,
		PermissionModelManage,
		PermissionRuntimeManage,
		PermissionAssistantRun,
		PermissionDatabaseMaintenanceManage,
		PermissionIndustrySolutionManage,
		PermissionIndustrySolutionImport,
		PermissionIndustrySolutionExport,
		PermissionIndustrySolutionVerify,
		PermissionTenantIndustrySolutionApply,
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
	case "operator", "ops", "support", "system_support":
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
