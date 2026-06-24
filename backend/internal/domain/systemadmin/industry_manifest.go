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

func BuildPackageAssetDiff(current IndustrySolutionManifest, desired IndustrySolutionManifest) []PackageAssetDiff {
	currentByKey := map[string]IndustrySolutionAsset{}
	desiredByKey := map[string]IndustrySolutionAsset{}
	for _, asset := range current.Assets {
		currentByKey[asset.AssetKey] = asset
	}
	for _, asset := range desired.Assets {
		desiredByKey[asset.AssetKey] = asset
	}

	items := []PackageAssetDiff{}
	for key, desiredAsset := range desiredByKey {
		currentAsset, ok := currentByKey[key]
		action := "create"
		currentVersion := ""
		if ok {
			currentVersion = currentAsset.Version
			if currentAsset.Version == desiredAsset.Version && mapsEqual(currentAsset.Payload, desiredAsset.Payload) {
				action = "unchanged"
			} else {
				action = "update"
			}
		}
		items = append(items, PackageAssetDiff{
			AssetType:      desiredAsset.AssetType,
			AssetKey:       desiredAsset.AssetKey,
			Action:         action,
			RiskLevel:      firstNonEmptyString(desiredAsset.RiskLevel, currentAsset.RiskLevel, "medium"),
			CurrentVersion: currentVersion,
			DesiredVersion: desiredAsset.Version,
			Summary:        fmt.Sprintf("%s %s", action, desiredAsset.AssetKey),
			DependsOn:      desiredAsset.DependsOn,
		})
	}
	for key, currentAsset := range currentByKey {
		if _, ok := desiredByKey[key]; ok {
			continue
		}
		items = append(items, PackageAssetDiff{
			AssetType:      currentAsset.AssetType,
			AssetKey:       currentAsset.AssetKey,
			Action:         "remove",
			RiskLevel:      "high",
			CurrentVersion: currentAsset.Version,
			Summary:        fmt.Sprintf("remove %s", currentAsset.AssetKey),
			BlockingReason: "asset removal requires explicit platform review",
			DependsOn:      currentAsset.DependsOn,
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Action == items[j].Action {
			return items[i].AssetKey < items[j].AssetKey
		}
		return items[i].Action < items[j].Action
	})
	return items
}

func PackageDiffFromSchemaPackage(pkg SchemaPackage) []PackageAssetDiff {
	if pkg.Metadata == nil {
		return nil
	}
	data, err := json.Marshal(pkg.Metadata["package_diff"])
	if err != nil {
		return nil
	}
	var diff []PackageAssetDiff
	if err := json.Unmarshal(data, &diff); err != nil {
		return nil
	}
	return diff
}

func ManifestVerificationChecks(manifest IndustrySolutionManifest, riskLevel string) []SchemaVerificationCheck {
	return []SchemaVerificationCheck{
		runtimeOperationCheck(manifest),
		toolPolicyCheck(manifest),
		assistantContextCheck(manifest),
		assistantSkillCheck(manifest),
		qualityGateCheck(manifest),
		verificationScenarioCheck(manifest),
		rollbackRiskCheck(manifest, riskLevel),
	}
}

func runtimeOperationCheck(manifest IndustrySolutionManifest) SchemaVerificationCheck {
	paths := map[string]string{}
	duplicates := []string{}
	count := 0
	for _, asset := range manifest.Assets {
		if asset.AssetType != AssetTypeRuntimeOperation {
			continue
		}
		count++
		path := stringValue(asset.Payload["path"])
		if path == "" {
			return SchemaVerificationCheck{Key: "runtime_operations", Status: "failed", Message: "runtime operation asset is missing path", Metadata: map[string]any{"asset_key": asset.AssetKey}}
		}
		if previous := paths[path]; previous != "" {
			duplicates = append(duplicates, previous, asset.AssetKey)
		}
		paths[path] = asset.AssetKey
	}
	if len(duplicates) > 0 {
		return SchemaVerificationCheck{Key: "runtime_operations", Status: "failed", Message: "runtime operation paths must be unique", Metadata: map[string]any{"duplicates": duplicates}}
	}
	return SchemaVerificationCheck{Key: "runtime_operations", Status: "passed", Message: "runtime operations are unique and executable", Metadata: map[string]any{"count": count}}
}

func toolPolicyCheck(manifest IndustrySolutionManifest) SchemaVerificationCheck {
	count := 0
	for _, asset := range manifest.Assets {
		if asset.AssetType != AssetTypeToolPolicy && asset.AssetType != AssetTypeToolDefinition {
			continue
		}
		count++
		if stringValue(asset.Payload["tool_key"]) == "" {
			return SchemaVerificationCheck{Key: "tool_policy", Status: "failed", Message: "tool asset is missing tool_key", Metadata: map[string]any{"asset_key": asset.AssetKey}}
		}
		if stringValue(asset.Payload["policy"]) == "" {
			return SchemaVerificationCheck{Key: "tool_policy", Status: "failed", Message: "tool asset is missing policy", Metadata: map[string]any{"asset_key": asset.AssetKey}}
		}
		if len(stringSliceFromAny(asset.Payload["permissions"])) == 0 {
			return SchemaVerificationCheck{Key: "tool_policy", Status: "failed", Message: "tool asset is missing required permissions", Metadata: map[string]any{"asset_key": asset.AssetKey}}
		}
	}
	return SchemaVerificationCheck{Key: "tool_policy", Status: "passed", Message: "tool definitions declare policy, risk, and permissions", Metadata: map[string]any{"count": count}}
}

func assistantContextCheck(manifest IndustrySolutionManifest) SchemaVerificationCheck {
	count := 0
	for _, asset := range manifest.Assets {
		if asset.AssetType != AssetTypeContextRule {
			continue
		}
		count++
		status := firstNonEmptyString(stringValue(asset.Payload["status"]), "draft")
		if status == "active" {
			return SchemaVerificationCheck{Key: "assistant_context", Status: "failed", Message: "context rules from industry packages must stay draft before human activation", Metadata: map[string]any{"asset_key": asset.AssetKey}}
		}
		if len(stringSliceFromAny(asset.Payload["required_permissions"])) == 0 {
			return SchemaVerificationCheck{Key: "assistant_context", Status: "failed", Message: "context rule is missing required permissions", Metadata: map[string]any{"asset_key": asset.AssetKey}}
		}
	}
	return SchemaVerificationCheck{Key: "assistant_context", Status: "passed", Message: "assistant context assets are draft and permission scoped", Metadata: map[string]any{"count": count}}
}

func assistantSkillCheck(manifest IndustrySolutionManifest) SchemaVerificationCheck {
	count := 0
	for _, asset := range manifest.Assets {
		if asset.AssetType != AssetTypeAssistantSkill {
			continue
		}
		count++
		if len(stringSliceFromAny(asset.Payload["targets"])) == 0 || len(stringSliceFromAny(asset.Payload["context_rules"])) == 0 || len(stringSliceFromAny(asset.Payload["allowed_tools"])) == 0 {
			return SchemaVerificationCheck{Key: "assistant_skills", Status: "failed", Message: "assistant skill must declare targets, context rules, and allowed tools", Metadata: map[string]any{"asset_key": asset.AssetKey}}
		}
	}
	return SchemaVerificationCheck{Key: "assistant_skills", Status: "passed", Message: "assistant skills declare targets, context rules, and allowed tools", Metadata: map[string]any{"count": count}}
}

func qualityGateCheck(manifest IndustrySolutionManifest) SchemaVerificationCheck {
	if len(manifest.QualityGates) == 0 {
		return SchemaVerificationCheck{Key: "quality_gates", Status: "warning", Message: "industry package should declare quality gates"}
	}
	return SchemaVerificationCheck{Key: "quality_gates", Status: "passed", Message: "quality gates are declared", Metadata: map[string]any{"count": len(manifest.QualityGates)}}
}

func verificationScenarioCheck(manifest IndustrySolutionManifest) SchemaVerificationCheck {
	if len(manifest.VerificationScenarios) == 0 {
		return SchemaVerificationCheck{Key: "verification_scenarios", Status: "warning", Message: "industry package should declare verification scenarios"}
	}
	return SchemaVerificationCheck{Key: "verification_scenarios", Status: "passed", Message: "verification scenarios are declared", Metadata: map[string]any{"count": len(manifest.VerificationScenarios)}}
}

func rollbackRiskCheck(manifest IndustrySolutionManifest, riskLevel string) SchemaVerificationCheck {
	if riskLevel == SchemaRiskDestructive {
		for _, asset := range manifest.Assets {
			if stringValue(asset.Payload["rollback_plan"]) != "" {
				return SchemaVerificationCheck{Key: "rollback_risk", Status: "warning", Message: "destructive change includes a rollback plan for manual review"}
			}
		}
		return SchemaVerificationCheck{Key: "rollback_risk", Status: "failed", Message: "destructive industry package requires rollback_plan metadata"}
	}
	return SchemaVerificationCheck{Key: "rollback_risk", Status: "passed", Message: "rollback risk is low for additive factory package", Metadata: map[string]any{"risk_level": riskLevel}}
}

func BuildSchemaApplyAssetResults(manifest IndustrySolutionManifest) []SchemaApplyAssetResult {
	results := make([]SchemaApplyAssetResult, 0, len(manifest.Assets))
	for _, asset := range manifest.Assets {
		results = append(results, SchemaApplyAssetResult{
			AssetKey:  asset.AssetKey,
			AssetType: asset.AssetType,
			Status:    "pending",
			Target:    applyTargetForAsset(asset),
			Metadata: map[string]any{
				"payload":    asset.Payload,
				"risk_level": asset.RiskLevel,
				"version":    asset.Version,
			},
		})
	}
	return results
}

func applyTargetForAsset(asset IndustrySolutionAsset) string {
	switch asset.AssetType {
	case AssetTypeRuntimeOperation:
		return "platform.runtime_operations"
	case AssetTypeToolDefinition, AssetTypeToolPolicy:
		return "tool_definitions"
	case AssetTypeContextRule:
		return "platform.platform_masters:context_rule_draft"
	case AssetTypeAssistantSkill:
		return "platform.platform_masters:assistant_skill"
	case AssetTypeQualityGate:
		return "platform.platform_masters:quality_gate"
	case AssetTypeVerificationScenario:
		return "platform.platform_masters:verification_scenario"
	default:
		return "schema_package.metadata"
	}
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

	if runtimeOperations := mapSliceFromAny(metadata["runtime_operations"]); len(runtimeOperations) > 0 {
		for _, operation := range runtimeOperations {
			assetKey := firstNonEmptyString(stringValue(operation["operation_key"]), stringValue(operation["path"]))
			assets = append(assets, IndustrySolutionAsset{
				AssetKey:  manifestAssetKey(AssetTypeRuntimeOperation, assetKey),
				AssetType: AssetTypeRuntimeOperation,
				Version:   "v1",
				Source:    "erp_standard_factory",
				Owner:     "platform",
				RiskLevel: firstNonEmptyString(stringValue(operation["risk_level"]), stringValue(operation["danger_level"]), "medium"),
				Payload:   operation,
			})
		}
	} else {
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

func mapsEqual(left map[string]any, right map[string]any) bool {
	leftJSON, _ := json.Marshal(left)
	rightJSON, _ := json.Marshal(right)
	return string(leftJSON) == string(rightJSON)
}
