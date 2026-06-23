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
	required := []string{
		"database_assets",
		"business_functions",
		"process_loops",
		"permissions",
		"api_operations",
		"ui_workspaces",
		"assistant_targets",
		"context_rules",
		"tool_definitions",
		"assistant_skills",
		"quality_gates",
		"verification_scenarios",
	}
	for _, key := range required {
		if !result.SchemaPackageHas(key) {
			t.Fatalf("schema package missing %s in %#v", key, result.SchemaPackage.Metadata)
		}
	}
	requiredTables := []string{
		"erp_solution_context_rules",
		"erp_solution_tool_definitions",
		"erp_solution_assistant_skills",
		"erp_solution_quality_gates",
		"erp_solution_verification_scenarios",
	}
	for _, name := range requiredTables {
		if !schemaPackageHasTable(result.SchemaPackage, name) {
			t.Fatalf("schema package missing table %s in %#v", name, result.SchemaPackage.Tables)
		}
	}
}

func schemaPackageHasTable(pkg SchemaPackage, name string) bool {
	for _, table := range pkg.Tables {
		if table.Name == name {
			return true
		}
	}
	return false
}
