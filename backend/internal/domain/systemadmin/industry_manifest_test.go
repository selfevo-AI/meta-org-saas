package systemadmin

import "testing"

func TestBuildERPSolutionSchemaPackageIncludesStructuredManifest(t *testing.T) {
	pkg := BuildERPSolutionSchemaPackage(ERPSolutionFlowRequest{
		IndustryKey:    "professional_services",
		PackageKey:     "erp_standard",
		Name:           "ERP Standard",
		EnabledModules: []string{"project", "procurement", "inventory", "sales", "finance"},
	})

	manifest, err := ManifestFromSchemaPackage(pkg)
	if err != nil {
		t.Fatalf("ManifestFromSchemaPackage error = %v", err)
	}
	if manifest.ManifestVersion != IndustryManifestVersion {
		t.Fatalf("manifest version = %q, want %q", manifest.ManifestVersion, IndustryManifestVersion)
	}
	if manifest.IndustryKey != "professional_services" || manifest.PackageKey != "erp_standard" {
		t.Fatalf("manifest package = %s/%s, want professional_services/erp_standard", manifest.IndustryKey, manifest.PackageKey)
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
		if countManifestAssets(manifest, assetType) == 0 {
			t.Fatalf("manifest missing asset type %s in %#v", assetType, manifest.Assets)
		}
	}
}

func countManifestAssets(manifest IndustrySolutionManifest, assetType string) int {
	count := 0
	for _, asset := range manifest.Assets {
		if asset.AssetType == assetType {
			count++
		}
	}
	return count
}

func TestBuildPackageAssetDiffDetectsManifestAssetChanges(t *testing.T) {
	current := IndustrySolutionManifest{
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
	desired := IndustrySolutionManifest{
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

	diff := BuildPackageAssetDiff(current, desired)

	assertPackageDiffAction(t, diff, "database_asset.mreq", "unchanged")
	assertPackageDiffAction(t, diff, "tool_definition.erp.mreq.approve", "update")
	assertPackageDiffAction(t, diff, "context_rule.erp_document_state_context", "create")
	assertPackageDiffAction(t, diff, "runtime_operation.old", "remove")
}

func assertPackageDiffAction(t *testing.T, diff []PackageAssetDiff, assetKey string, action string) {
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
