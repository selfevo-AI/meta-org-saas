package gateway

import (
	"os"
	"strings"
	"testing"
)

func TestPlatformHomeRoutesAreMountedWithExpectedPermissions(t *testing.T) {
	sourceBytes, err := os.ReadFile("router.go")
	if err != nil {
		t.Fatalf("read router.go: %v", err)
	}
	source := string(sourceBytes)
	platformBlock := functionBlock(t, source, "registerPlatformAdminRoutes")
	assistantBlock := functionBlock(t, source, "registerPlatformAssistantRoutes")

	for _, snippet := range []string{
		"deps.MetaOrgHandler.RegisterPlatformRoutes",
		"platformauth.PermissionPlatformRead",
	} {
		if !strings.Contains(platformBlock, snippet) {
			t.Fatalf("platform admin routes should include %q", snippet)
		}
	}

	if !strings.Contains(assistantBlock, "deps.AssistantHandler.RegisterPlatformRoutes") {
		t.Fatalf("platform assistant routes should register platform assistant handlers")
	}
}
