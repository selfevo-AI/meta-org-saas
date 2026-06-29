package systemadmin

import "testing"

func TestBuildERPSolutionManifestIncludesStructuredAssetManifest(t *testing.T) {
	manifest := BuildERPSolutionManifest(ERPSolutionFlowRequest{
		IndustryKey:    "professional_services",
		PackageKey:     "erp_standard",
		Name:           "ERP Standard",
		EnabledModules: []string{"project", "procurement", "inventory", "sales", "finance"},
	})

	assetManifest, err := AssetManifestFromSolutionManifest(manifest)
	if err != nil {
		t.Fatalf("AssetManifestFromSolutionManifest error = %v", err)
	}
	if assetManifest.ManifestVersion != IndustryManifestVersion {
		t.Fatalf("manifest version = %q, want %q", assetManifest.ManifestVersion, IndustryManifestVersion)
	}
	if assetManifest.IndustryKey != "professional_services" || assetManifest.PackageKey != "erp_standard" {
		t.Fatalf("asset manifest package = %s/%s, want professional_services/erp_standard", assetManifest.IndustryKey, assetManifest.PackageKey)
	}
	requiredTypes := []string{
		AssetTypeDatabaseAsset,
		AssetTypeBusinessFunction,
		AssetTypeProcessLoop,
		AssetTypeRuntimeOperation,
		AssetTypeUIWorkspace,
		AssetTypePermission,
		AssetTypeToolPolicy,
		AssetTypeToolDefinition,
		AssetTypeAssistantTarget,
		AssetTypeContextRule,
		AssetTypeAssistantSkill,
		AssetTypeQualityGate,
		AssetTypeVerificationScenario,
	}
	for _, assetType := range requiredTypes {
		if countManifestAssets(assetManifest, assetType) == 0 {
			t.Fatalf("asset manifest missing asset type %s in %#v", assetType, assetManifest.Assets)
		}
	}
	if !manifestHasRuntimeWorkspace(assetManifest, "project", "requirement", "MREQ", "convert-to-project") {
		t.Fatalf("asset manifest missing runtime workspace payload for MREQ convert action: %#v", assetManifest.Assets)
	}
}

func countManifestAssets(manifest IndustrySolutionAssetManifest, assetType string) int {
	count := 0
	for _, asset := range manifest.Assets {
		if asset.AssetType == assetType {
			count++
		}
	}
	return count
}

func manifestHasRuntimeWorkspace(manifest IndustrySolutionAssetManifest, module string, documentID string, tableCode string, action string) bool {
	for _, asset := range manifest.Assets {
		if asset.AssetType != AssetTypeRuntimeOperation {
			continue
		}
		workspace, _ := asset.Payload["workspace"].(map[string]any)
		if workspace["module"] == module && workspace["document_id"] == documentID && workspace["table_code"] == tableCode && workspace["action"] == action {
			return true
		}
	}
	return false
}

func TestBuildIndustrySolutionAssetDiffDetectsManifestAssetChanges(t *testing.T) {
	current := IndustrySolutionAssetManifest{
		ManifestVersion: IndustryManifestVersion,
		IndustryKey:     "professional_services",
		PackageKey:      "erp_standard",
		PackageVersion:  "v1",
		Assets: []IndustrySolutionAsset{
			{AssetKey: "database_asset.mreq", AssetType: AssetTypeDatabaseAsset, Version: "v1", RiskLevel: "low", Payload: map[string]any{"table_code": "MREQ"}},
			{AssetKey: "tool_definition.erp.mreq.approve", AssetType: AssetTypeToolDefinition, Version: "v1", RiskLevel: "medium", Payload: map[string]any{"tool_key": "erp.mreq.approve", "policy": "old"}},
			{AssetKey: "runtime_operation.old", AssetType: AssetTypeRuntimeOperation, Version: "v1", RiskLevel: "low", Payload: map[string]any{"path": "/old"}},
		},
	}
	desired := IndustrySolutionAssetManifest{
		ManifestVersion: IndustryManifestVersion,
		IndustryKey:     "professional_services",
		PackageKey:      "erp_standard",
		PackageVersion:  "v2",
		Assets: []IndustrySolutionAsset{
			{AssetKey: "database_asset.mreq", AssetType: AssetTypeDatabaseAsset, Version: "v1", RiskLevel: "low", Payload: map[string]any{"table_code": "MREQ"}},
			{AssetKey: "tool_definition.erp.mreq.approve", AssetType: AssetTypeToolDefinition, Version: "v2", RiskLevel: "high", Payload: map[string]any{"tool_key": "erp.mreq.approve", "policy": "erp_action_state_gate"}},
			{AssetKey: "context_rule.erp_document_state_context", AssetType: AssetTypeContextRule, Version: "v1", RiskLevel: "medium", Payload: map[string]any{"key": "erp_document_state_context"}},
		},
	}

	diff := BuildIndustrySolutionAssetDiff(current, desired)

	assertPackageDiffAction(t, diff, "database_asset.mreq", "unchanged")
	assertPackageDiffAction(t, diff, "tool_definition.erp.mreq.approve", "update")
	assertPackageDiffAction(t, diff, "context_rule.erp_document_state_context", "create")
	assertPackageDiffAction(t, diff, "runtime_operation.old", "remove")
}

func assertPackageDiffAction(t *testing.T, diff []IndustrySolutionAssetDiff, assetKey string, action string) {
	t.Helper()
	for _, item := range diff {
		if item.AssetKey == assetKey {
			if item.Action != action {
				t.Fatalf("diff action for %s = %q, want %q", assetKey, item.Action, action)
			}
			return
		}
	}
	t.Fatalf("missing diff item %s in %#v", assetKey, diff)
}

func TestBuildIndustrySolutionApplyAssetResultsIncludesManifestAssets(t *testing.T) {
	manifest := BuildERPSolutionManifest(ERPSolutionFlowRequest{IndustryKey: "professional_services", PackageKey: "erp_standard", Name: "ERP Standard"})
	assetManifest, err := AssetManifestFromSolutionManifest(manifest)
	if err != nil {
		t.Fatalf("AssetManifestFromSolutionManifest error = %v", err)
	}

	results := BuildIndustrySolutionApplyAssetResults(assetManifest)

	if len(results) == 0 {
		t.Fatal("BuildIndustrySolutionApplyAssetResults returned no results")
	}
	if !hasAssetResult(results, AssetTypeRuntimeOperation) {
		t.Fatalf("results missing runtime operation asset in %#v", results)
	}
	if !hasAssetResult(results, AssetTypeContextRule) {
		t.Fatalf("results missing context rule asset in %#v", results)
	}
}

func hasAssetResult(results []IndustrySolutionApplyAssetResult, assetType string) bool {
	for _, result := range results {
		if result.AssetType == assetType && result.Status == "pending" {
			return true
		}
	}
	return false
}
