package platformauth

import "testing"

func TestRolePermissionMatrix(t *testing.T) {
	tests := []struct {
		name       string
		role       string
		permission string
		want       bool
	}{
		{name: "owner can close organizations", role: RoleOwner, permission: PermissionOrganizationClose, want: true},
		{name: "admin can manage schema", role: RoleAdmin, permission: PermissionSchemaApply, want: true},
		{name: "operator cannot close organizations", role: RoleOperator, permission: PermissionOrganizationClose, want: false},
		{name: "auditor can read platform", role: RoleAuditor, permission: PermissionPlatformRead, want: true},
		{name: "auditor cannot apply schema", role: RoleAuditor, permission: PermissionSchemaApply, want: false},
		{name: "legacy system owner maps to owner", role: "system_owner", permission: PermissionOrganizationClose, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasPermission(tt.role, tt.permission); got != tt.want {
				t.Fatalf("HasPermission(%q, %q) = %v, want %v", tt.role, tt.permission, got, tt.want)
			}
		})
	}
}

func TestPermissionsForRoleReturnsCopy(t *testing.T) {
	permissions := PermissionsForRole(RoleOwner)
	permissions[PermissionOrganizationClose] = false

	if !HasPermission(RoleOwner, PermissionOrganizationClose) {
		t.Fatalf("mutating PermissionsForRole result changed owner permission matrix")
	}
}
