package systemadmin

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const IndustryManifestVersion = "meta-org.industry-solution.v1"

const (
	AssetTypeDatabaseAsset        = "database_asset"
	AssetTypeBusinessFunction     = "business_function"
	AssetTypeProcessLoop          = "process_loop"
	AssetTypeRuntimeOperation     = "runtime_operation"
	AssetTypeUIWorkspace          = "ui_workspace"
	AssetTypePermission           = "permission"
	AssetTypeToolPolicy           = "tool_policy"
	AssetTypeToolDefinition       = "tool_definition"
	AssetTypeAssistantTarget      = "assistant_target"
	AssetTypeContextRule          = "context_rule"
	AssetTypeAssistantSkill       = "assistant_skill"
	AssetTypeQualityGate          = "quality_gate"
	AssetTypeVerificationScenario = "verification_scenario"
)

type IndustrySolutionManifest struct {
	ManifestVersion       string                  `json:"manifest_version"`
	IndustryKey           string                  `json:"industry_key"`
	PackageKey            string                  `json:"package_key"`
	PackageVersion        string                  `json:"package_version"`
	Assets                []IndustrySolutionAsset `json:"assets"`
	Dependencies          []string                `json:"dependencies,omitempty"`
	QualityGates          []string                `json:"quality_gates,omitempty"`
	VerificationScenarios []string                `json:"verification_scenarios,omitempty"`
}

type IndustrySolutionAsset struct {
	AssetKey  string         `json:"asset_key"`
	AssetType string         `json:"asset_type"`
	Version   string         `json:"version"`
	Source    string         `json:"source"`
	Owner     string         `json:"owner"`
	RiskLevel string         `json:"risk_level"`
	DependsOn []string       `json:"depends_on,omitempty"`
	Payload   map[string]any `json:"payload"`
}

func ManifestFromSchemaPackage(pkg SchemaPackage) (IndustrySolutionManifest, error) {
	var manifest IndustrySolutionManifest
	if pkg.Metadata == nil || pkg.Metadata["industry_manifest"] == nil {
		return manifest, fmt.Errorf("industry_manifest metadata is required")
	}
	data, err := json.Marshal(pkg.Metadata["industry_manifest"])
	if err != nil {
		return manifest, fmt.Errorf("marshal industry manifest: %w", err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, fmt.Errorf("decode industry manifest: %w", err)
	}
	if manifest.ManifestVersion != IndustryManifestVersion {
		return manifest, fmt.Errorf("unsupported industry manifest version %q", manifest.ManifestVersion)
	}
	return manifest, nil
}

func setIndustryManifest(pkg *SchemaPackage, manifest IndustrySolutionManifest) {
	if pkg.Metadata == nil {
		pkg.Metadata = map[string]any{}
	}
	pkg.Metadata["industry_manifest"] = manifest
}

func buildIndustryManifest(input ERPSolutionFlowRequest, metadata map[string]any) IndustrySolutionManifest {
	assets := []IndustrySolutionAsset{}
	appendAssets := func(assetType string, values []map[string]any, keyField string) {
		for _, value := range values {
			key := firstNonEmptyString(stringValue(value[keyField]), stringValue(value["key"]), stringValue(value["tool_key"]), stringValue(value["scenario_key"]), stringValue(value["gate_key"]))
			if key == "" {
				continue
			}
			assets = append(assets, IndustrySolutionAsset{
				AssetKey:  manifestAssetKey(assetType, key),
				AssetType: assetType,
				Version:   "v1",
				Source:    "erp_standard_factory",
				Owner:     "platform",
				RiskLevel: firstNonEmptyString(stringValue(value["risk_level"]), "medium"),
				DependsOn: stringSliceFromAny(value["depends_on"]),
				Payload:   value,
			})
		}
	}

	appendAssets(AssetTypeDatabaseAsset, mapSliceFromAny(metadata["database_assets"]), "table_code")
	appendAssets(AssetTypeBusinessFunction, mapSliceFromAny(metadata["business_functions"]), "action")
	appendAssets(AssetTypeProcessLoop, mapSliceFromAny(metadata["process_loops"]), "key")
	appendAssets(AssetTypeToolDefinition, mapSliceFromAny(metadata["tool_definitions"]), "tool_key")
	appendAssets(AssetTypeToolPolicy, mapSliceFromAny(metadata["tool_definitions"]), "tool_key")
	appendAssets(AssetTypeContextRule, mapSliceFromAny(metadata["context_rules"]), "key")
	appendAssets(AssetTypeAssistantSkill, mapSliceFromAny(metadata["assistant_skills"]), "skill_key")
	appendAssets(AssetTypeQualityGate, mapSliceFromAny(metadata["quality_gates"]), "gate_key")
	appendAssets(AssetTypeVerificationScenario, mapSliceFromAny(metadata["verification_scenarios"]), "scenario_key")

	for _, operation := range stringSliceFromAny(metadata["api_operations"]) {
		assets = append(assets, IndustrySolutionAsset{
			AssetKey:  manifestAssetKey(AssetTypeRuntimeOperation, operation),
			AssetType: AssetTypeRuntimeOperation,
			Version:   "v1",
			Source:    "erp_standard_factory",
			Owner:     "platform",
			RiskLevel: "medium",
			Payload:   map[string]any{"path": operation},
		})
	}
	for _, workspace := range stringSliceFromAny(metadata["ui_workspaces"]) {
		assets = append(assets, IndustrySolutionAsset{
			AssetKey:  manifestAssetKey(AssetTypeUIWorkspace, workspace),
			AssetType: AssetTypeUIWorkspace,
			Version:   "v1",
			Source:    "erp_standard_factory",
			Owner:     "platform",
			RiskLevel: "low",
			Payload:   map[string]any{"workspace_key": workspace},
		})
	}
	for _, permission := range stringSliceFromAny(metadata["permissions"]) {
		assets = append(assets, IndustrySolutionAsset{
			AssetKey:  manifestAssetKey(AssetTypePermission, permission),
			AssetType: AssetTypePermission,
			Version:   "v1",
			Source:    "erp_standard_factory",
			Owner:     "platform",
			RiskLevel: "medium",
			Payload:   map[string]any{"permission": permission},
		})
	}
	for _, target := range stringSliceFromAny(metadata["assistant_targets"]) {
		assets = append(assets, IndustrySolutionAsset{
			AssetKey:  manifestAssetKey(AssetTypeAssistantTarget, target),
			AssetType: AssetTypeAssistantTarget,
			Version:   "v1",
			Source:    "erp_standard_factory",
			Owner:     "platform",
			RiskLevel: "medium",
			Payload:   map[string]any{"target": target},
		})
	}

	sortManifestAssets(assets)
	return IndustrySolutionManifest{
		ManifestVersion:       IndustryManifestVersion,
		IndustryKey:           input.IndustryKey,
		PackageKey:            input.PackageKey,
		PackageVersion:        "v1",
		Assets:                assets,
		Dependencies:          []string{"erp.catalog", "erp.action_registry", "platform.schema_change"},
		QualityGates:          stringSliceFromMaps(mapSliceFromAny(metadata["quality_gates"]), "gate_key"),
		VerificationScenarios: stringSliceFromMaps(mapSliceFromAny(metadata["verification_scenarios"]), "scenario_key"),
	}
}

func manifestAssetKey(assetType string, parts ...string) string {
	clean := make([]string, 0, len(parts)+1)
	clean = append(clean, assetType)
	for _, part := range parts {
		trimmed := strings.TrimSpace(strings.ToLower(part))
		if trimmed != "" {
			clean = append(clean, strings.ReplaceAll(trimmed, " ", "_"))
		}
	}
	return strings.Join(clean, ".")
}

func sortManifestAssets(assets []IndustrySolutionAsset) {
	sort.SliceStable(assets, func(i, j int) bool {
		if assets[i].AssetType == assets[j].AssetType {
			return assets[i].AssetKey < assets[j].AssetKey
		}
		return assets[i].AssetType < assets[j].AssetType
	})
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func mapSliceFromAny(value any) []map[string]any {
	switch typed := value.(type) {
	case []map[string]any:
		return typed
	case []any:
		items := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if mapped, ok := item.(map[string]any); ok {
				items = append(items, mapped)
			}
		}
		return items
	default:
		return nil
	}
}

func stringSliceFromAny(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		items := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := stringValue(item); text != "" {
				items = append(items, text)
			}
		}
		return items
	default:
		return nil
	}
}

func stringSliceFromMaps(values []map[string]any, key string) []string {
	items := make([]string, 0, len(values))
	for _, value := range values {
		if text := stringValue(value[key]); text != "" {
			items = append(items, text)
		}
	}
	return items
}
