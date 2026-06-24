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
