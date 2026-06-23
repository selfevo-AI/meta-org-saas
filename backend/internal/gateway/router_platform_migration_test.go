package gateway

import (
	"os"
	"strings"
	"testing"
)

func TestPlatformOnlyDomainsAreNotMountedInTenantRoutes(t *testing.T) {
	sourceBytes, err := os.ReadFile("router.go")
	if err != nil {
		t.Fatalf("read router.go: %v", err)
	}
	source := string(sourceBytes)
	tenantBlock := functionBlock(t, source, "registerTenantRoutes")
	platformBlock := functionBlock(t, source, "registerPlatformAdminRoutes")

	movedHandlers := []string{
		"deps.IdentityHandler.RegisterProtectedRoutes",
		"deps.LayerHandler.RegisterRoutes",
		"deps.CapabilityHandler.RegisterRoutes",
		"deps.GovernanceHandler.RegisterRoutes",
		"deps.EvolutionHandler.RegisterRoutes",
		"deps.VerificationHandler.RegisterRoutes",
		"deps.ObservabilityHandler.RegisterRoutes",
	}

	for _, handlerCall := range movedHandlers {
		if strings.Contains(tenantBlock, handlerCall) {
			t.Fatalf("%s is still mounted in tenant routes", handlerCall)
		}
		if !strings.Contains(platformBlock, handlerCall) {
			t.Fatalf("%s is not mounted in platform admin routes", handlerCall)
		}
	}

	if strings.Contains(tenantBlock, "AIGatewayHandler.RegisterTenantRoutes") {
		t.Fatalf("tenant routes should not expose AI gateway tenant routes after ERP API replacement")
	}
	if !strings.Contains(tenantBlock, "ErpHandler.RegisterRoutes") {
		t.Fatalf("tenant routes should expose ERP code-table routes")
	}
	if !strings.Contains(platformBlock, "AIGatewayHandler.RegisterRoutes") {
		t.Fatalf("platform routes should expose AI gateway administration routes")
	}
}

func TestERPRoutesAreMountedForTenantWorkbench(t *testing.T) {
	sourceBytes, err := os.ReadFile("router.go")
	if err != nil {
		t.Fatalf("read router.go: %v", err)
	}
	tenantBlock := functionBlock(t, string(sourceBytes), "registerTenantRoutes")
	if !strings.Contains(tenantBlock, "deps.ErpHandler.RegisterRoutes") {
		t.Fatalf("tenant routes should expose ERP code-table operations used by the frontend workbench")
	}
	if strings.Contains(tenantBlock, "deps.RuntimeHandler.RegisterRoutes") {
		t.Fatalf("runtime entity routes should not be mounted in tenant routes after ERP API replacement")
	}
}

func TestTenantHomeRoutesAreMounted(t *testing.T) {
	sourceBytes, err := os.ReadFile("router.go")
	if err != nil {
		t.Fatalf("read router.go: %v", err)
	}
	tenantBlock := functionBlock(t, string(sourceBytes), "registerTenantRoutes")
	for _, snippet := range []string{
		"deps.DashboardHandler.RegisterRoutes",
		"deps.MetaOrgHandler.RegisterRoutes",
	} {
		if !strings.Contains(tenantBlock, snippet) {
			t.Fatalf("tenant routes should mount homepage API %q", snippet)
		}
	}
}

func TestOrganizationManagementIsMountedOnlyInPlatformAdmin(t *testing.T) {
	sourceBytes, err := os.ReadFile("router.go")
	if err != nil {
		t.Fatalf("read router.go: %v", err)
	}
	source := string(sourceBytes)
	tenantBlock := functionBlock(t, source, "registerTenantRoutes")
	platformBlock := functionBlock(t, source, "registerPlatformAdminRoutes")

	if !strings.Contains(platformBlock, "deps.OrganizationHandler.RegisterPlatformManagementRoutes") {
		t.Fatalf("platform admin routes should mount SaaS organization management routes")
	}
	if !strings.Contains(platformBlock, "platformauth.PermissionOrganizationManage") {
		t.Fatalf("platform organization management routes should require organization.manage")
	}
	if strings.Contains(tenantBlock, "deps.OrganizationHandler.RegisterPlatformManagementRoutes") {
		t.Fatalf("tenant routes should not mount platform organization management routes")
	}
	if strings.Contains(tenantBlock, "deps.OrganizationHandler.RegisterRoutes") {
		t.Fatalf("tenant routes should not mount full organization routes")
	}
}

func TestTenantRoutesExposeDepartmentWorkspaceOnly(t *testing.T) {
	sourceBytes, err := os.ReadFile("router.go")
	if err != nil {
		t.Fatalf("read router.go: %v", err)
	}
	tenantBlock := functionBlock(t, string(sourceBytes), "registerTenantRoutes")
	if !strings.Contains(tenantBlock, "deps.OrganizationHandler.RegisterTenantDepartmentRoutes") {
		t.Fatalf("tenant routes should mount current-tenant department workspace routes")
	}
	for _, forbidden := range []string{
		"deps.OrganizationHandler.RegisterRoutes",
		"deps.OrganizationHandler.RegisterPlatformManagementRoutes",
	} {
		if strings.Contains(tenantBlock, forbidden) {
			t.Fatalf("tenant routes should not mount %q", forbidden)
		}
	}
}

func TestLegacyTenantBusinessRoutesAreNotMounted(t *testing.T) {
	sourceBytes, err := os.ReadFile("router.go")
	if err != nil {
		t.Fatalf("read router.go: %v", err)
	}
	tenantBlock := functionBlock(t, string(sourceBytes), "registerTenantRoutes")
	for _, snippet := range []string{
		"OrganizationHandler.RegisterRoutes",
		"CostingHandler.RegisterRoutes",
		"MetaResourceHandler.RegisterRoutes",
		"AssistantHandler.RegisterRoutes",
		"AIGatewayHandler.RegisterTenantRoutes",
		"WorkflowHandler.RegisterRoutes",
		"ProjectHandler.RegisterRoutes",
		"FinanceHandler.RegisterRoutes",
		"InventoryHandler.RegisterRoutes",
		"ProcurementHandler.RegisterRoutes",
		"SalesHandler.RegisterRoutes",
		"ToolRuntimeHandler.RegisterRoutes",
	} {
		if strings.Contains(tenantBlock, snippet) {
			t.Fatalf("tenant routes still mount legacy API %q", snippet)
		}
	}
}

func functionBlock(t *testing.T, source string, name string) string {
	t.Helper()
	start := strings.Index(source, "func "+name)
	if start == -1 {
		t.Fatalf("missing %s", name)
	}
	next := strings.Index(source[start+1:], "\nfunc ")
	if next == -1 {
		return source[start:]
	}
	return source[start : start+1+next]
}
