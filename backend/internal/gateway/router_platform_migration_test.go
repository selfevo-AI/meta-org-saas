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

	if !strings.Contains(tenantBlock, "AIGatewayHandler.RegisterTenantRoutes") {
		t.Fatalf("tenant routes should expose only AI gateway tenant routes")
	}
	if !strings.Contains(platformBlock, "AIGatewayHandler.RegisterRoutes") {
		t.Fatalf("platform routes should expose AI gateway administration routes")
	}
}

func TestRuntimeRoutesAreMountedForTenantWorkbench(t *testing.T) {
	sourceBytes, err := os.ReadFile("router.go")
	if err != nil {
		t.Fatalf("read router.go: %v", err)
	}
	tenantBlock := functionBlock(t, string(sourceBytes), "registerTenantRoutes")
	if !strings.Contains(tenantBlock, "deps.RuntimeHandler.RegisterRoutes") {
		t.Fatalf("tenant routes should expose runtime operations used by the frontend workbench")
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
