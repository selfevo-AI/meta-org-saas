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
		"runtime_operations",
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
	runtimeOperations := mapSliceFromAny(result.SchemaPackage.Metadata["runtime_operations"])
	if len(runtimeOperations) == 0 {
		t.Fatalf("schema package runtime_operations is empty in %#v", result.SchemaPackage.Metadata)
	}
	if !hasWorkspaceRuntimeOperation(runtimeOperations, "project", "requirement", "MREQ", "convert-to-project") {
		t.Fatalf("runtime_operations missing project requirement convert workspace metadata: %#v", runtimeOperations)
	}
	if !hasWorkspaceRuntimeOperation(runtimeOperations, "finance", "trial_balance", "MGLR", "run") {
		t.Fatalf("runtime_operations missing finance trial balance workspace metadata: %#v", runtimeOperations)
	}
	if !hasRuntimeOperationPath(runtimeOperations, "/finance/gl/trial-balance") {
		t.Fatalf("runtime_operations missing GL trial balance API path: %#v", runtimeOperations)
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

func hasWorkspaceRuntimeOperation(operations []map[string]any, module string, documentID string, tableCode string, action string) bool {
	for _, operation := range operations {
		workspace, _ := operation["workspace"].(map[string]any)
		if workspace["module"] == module && workspace["document_id"] == documentID && workspace["table_code"] == tableCode && workspace["action"] == action {
			return true
		}
	}
	return false
}

func hasRuntimeOperationPath(operations []map[string]any, path string) bool {
	for _, operation := range operations {
		if operation["path"] == path {
			return true
		}
	}
	return false
}
