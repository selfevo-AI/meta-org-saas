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
		if !result.SolutionManifestHas(key) {
			t.Fatalf("solution manifest missing %s in %#v", key, result.SolutionManifest.Metadata)
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
		if !solutionManifestHasTable(result.SolutionManifest, name) {
			t.Fatalf("solution manifest missing table %s in %#v", name, result.SolutionManifest.Tables)
		}
	}
	runtimeOperations := mapSliceFromAny(result.SolutionManifest.Metadata["runtime_operations"])
	if len(runtimeOperations) == 0 {
		t.Fatalf("solution manifest runtime_operations is empty in %#v", result.SolutionManifest.Metadata)
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

func TestBuildRetailDistributionSolutionFlowBuildsCodeTablePackage(t *testing.T) {
	repo := &fakeRepository{role: "system_owner"}
	service := NewService(repo)
	organizationID := uuid.New()

	result, err := service.BuildRetailDistributionSolutionFlow(context.Background(), uuid.New(), ERPSolutionFlowRequest{
		OrganizationID: organizationID,
		IndustryKey:    "retail_chain_distribution",
		PackageKey:     "retail_distribution_v1",
		Name:           "Retail Distribution v1",
	})
	if err != nil {
		t.Fatalf("BuildRetailDistributionSolutionFlow error: %v", err)
	}
	if result.RequestType != "retail_distribution_solution_flow" {
		t.Fatalf("RequestType = %q, want retail_distribution_solution_flow", result.RequestType)
	}
	manifest, err := AssetManifestFromSolutionManifest(result.SolutionManifest)
	if err != nil {
		t.Fatalf("AssetManifestFromSolutionManifest error = %v", err)
	}
	if manifest.IndustryKey != "retail_chain_distribution" || manifest.PackageKey != "retail_distribution_v1" {
		t.Fatalf("manifest package = %s/%s, want retail_chain_distribution/retail_distribution_v1", manifest.IndustryKey, manifest.PackageKey)
	}
	for _, tableCode := range []string{"MRPS", "MDRQ", "MDSP", "MDRC", "MDIF", "MCNT", "MSPR"} {
		if !manifestHasDatabaseAsset(manifest, tableCode) {
			t.Fatalf("manifest missing retail ERP code-table %s", tableCode)
		}
	}
	for _, loopKey := range []string{"retail_replenishment_to_distribution", "retail_pos_to_cash", "retail_count_to_adjustment", "retail_special_procurement"} {
		if !manifestHasProcessLoop(manifest, loopKey) {
			t.Fatalf("manifest missing process loop %s", loopKey)
		}
	}
	if !manifestHasRuntimeWorkspace(manifest, "retail", "pos_sale", "MRPS", "close") {
		t.Fatalf("manifest missing POS close runtime workspace")
	}
}

func solutionManifestHasTable(pkg IndustrySolutionManifest, name string) bool {
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

func manifestHasDatabaseAsset(manifest IndustrySolutionAssetManifest, tableCode string) bool {
	for _, asset := range manifest.Assets {
		if asset.AssetType == AssetTypeDatabaseAsset && asset.Payload["table_code"] == tableCode {
			return true
		}
	}
	return false
}

func manifestHasProcessLoop(manifest IndustrySolutionAssetManifest, key string) bool {
	for _, asset := range manifest.Assets {
		if asset.AssetType == AssetTypeProcessLoop && asset.Payload["key"] == key {
			return true
		}
	}
	return false
}
