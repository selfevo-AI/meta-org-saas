package systemadmin

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestBuildERPSolutionFlowBuildsCompleteChangePackage(t *testing.T) {
	repo := &fakeRepository{role: "system_owner"}
	service := NewService(repo)
	organizationID := uuid.New()

	result, err := service.BuildERPSolutionFlow(context.Background(), uuid.New(), ERPSolutionFlowRequest{
		OrganizationID:  organizationID,
		IndustryKey:     "professional_services",
		PackageKey:      "erp_standard",
		Name:            "ERP Standard",
		EnabledModules:  []string{"project", "procurement", "inventory", "sales", "finance"},
		CurrentTemplate: nil,
	})
	if err != nil {
		t.Fatalf("BuildERPSolutionFlow error: %v", err)
	}
	if result == nil {
		t.Fatal("BuildERPSolutionFlow returned nil result")
	}
	if result.OrganizationID != organizationID {
		t.Fatalf("organization id = %s, want %s", result.OrganizationID, organizationID)
	}
	required := []string{"database_assets", "business_functions", "process_loops", "permissions", "api_operations", "ui_workspaces", "assistant_targets"}
	for _, key := range required {
		if !result.SchemaPackageHas(key) {
			t.Fatalf("schema package missing %s in %#v", key, result.SchemaPackage.Metadata)
		}
	}
}
