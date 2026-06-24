# Phase 2 Industry Solution Factory Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the existing SystemAdmin ERP standard flow into a platform industry solution factory with structured package manifests, package-level diff, verify gates, apply asset results, publication gates, and SystemAdmin visibility.

**Architecture:** Keep the existing schema change lifecycle and ERP standard flow endpoints. Add a typed industry solution manifest as a stable substructure in `SchemaPackage.Metadata`, compute package diff from current and desired manifests, make `VerifySchemaChange` validate real manifest gates, and apply manifest assets into existing platform control-plane tables. Phase 2 reuses existing tables (`platform.runtime_operations`, `tool_definitions`, `platform.platform_masters`, `platform.platform_details`, and `platform.schema_apply_jobs.metadata`) instead of adding new tables.

**Tech Stack:** Go, pgx, PostgreSQL JSONB, Next.js App Router, TypeScript, Tailwind CSS, existing bilingual i18n, staged baseline migrations.

---

## File Structure

- Create `backend/internal/domain/systemadmin/industry_manifest.go`
  - Own typed `IndustrySolutionManifest`, manifest assets, package diff V2, manifest parsing, manifest building, manifest validation, and apply-result helpers.
- Create `backend/internal/domain/systemadmin/industry_manifest_test.go`
  - Unit tests for manifest shape, diff V2, verify gate fixtures, and apply-result helpers.
- Modify `backend/internal/domain/systemadmin/model.go`
  - Add package diff and apply-result API models.
- Modify `backend/internal/domain/systemadmin/service.go`
  - Replace ad hoc metadata coverage with manifest-aware build, diff, verify, and apply checks.
- Modify `backend/internal/domain/systemadmin/handler.go`
  - Add `GET /platform/admin/schema-change-requests/{id}/package-diff`.
- Modify `backend/internal/domain/systemadmin/repository.go`
  - Apply DDL and manifest metadata assets in one apply job path, storing per-asset results in `schema_apply_jobs.metadata`.
- Modify `backend/internal/domain/systemadmin/service_test.go`
  - Cover verify blocking logic and apply gating.
- Modify `backend/internal/domain/industry/model.go`
  - Add publication gate result model.
- Modify `backend/internal/domain/industry/service.go`
  - Evaluate anonymization, knowledge-source permission, and verification-scenario publication gates.
- Modify `backend/internal/domain/industry/repository.go`
  - Persist gate metadata on publication requests and expose request lookup for review gating.
- Modify `backend/internal/domain/industry/service_test.go`
  - Cover failed publication gates and successful warning-tolerant approval.
- Modify `frontend/src/lib/api.ts`
  - Add manifest, diff V2, apply asset result, publication gate result types and API helper for package diff.
- Modify `frontend/src/app/system-admin-workspace.tsx`
  - Show manifest asset groups, package diff V2, verify blocking metadata, apply asset results, and publication gates.
- Modify `frontend/src/lib/i18n.tsx`
  - Add English and Chinese keys for all new UI text.
- Modify `frontend/verify-system-admin-workspace.mjs`
  - Extend static checks for Phase 2 factory UI and i18n coverage.
- Modify `migrations/000_saas_platform_management_baseline.sql`
  - Add SQL comments documenting that Phase 2 factory assets use existing runtime operations, platform master/detail, and apply job metadata.
- Modify `migrations/BASELINE_RESTRUCTURE.md`
  - Document Phase 2 table ownership and the no-new-table decision.

## Scope Decisions

- Do not create new Phase 2 tables in the first implementation. The existing baseline already contains stable JSONB metadata storage for schema packages, apply jobs, runtime operations, tool definitions, and platform master/detail records.
- Do not auto-activate assistant context rules. Context-rule assets are written as draft platform metadata and must be activated by a later human-approved context workflow.
- Do not add tenant business workbench pages. Frontend work stays inside the SystemAdmin control plane.
- Do not restore legacy tenant business routes. All factory metadata continues to target ERP table-code APIs and platform admin APIs.

---

## Task 1: Add Structured Industry Solution Manifest

**Files:**
- Create: `backend/internal/domain/systemadmin/industry_manifest.go`
- Create: `backend/internal/domain/systemadmin/industry_manifest_test.go`
- Modify: `backend/internal/domain/systemadmin/service.go`
- Modify: `backend/internal/domain/systemadmin/model.go`

- [ ] **Step 1: Write the failing manifest test**

Create `backend/internal/domain/systemadmin/industry_manifest_test.go`:

```go
package systemadmin

import (
	"testing"
)

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
```

- [ ] **Step 2: Run the test to verify RED**

Run:

```powershell
cd backend
go test ./internal/domain/systemadmin -run TestBuildERPSolutionSchemaPackageIncludesStructuredManifest -count=1
```

Expected: FAIL with compile errors for `ManifestFromSchemaPackage`, `IndustryManifestVersion`, `IndustrySolutionManifest`, and `AssetType...` constants.

- [ ] **Step 3: Add manifest types and parsing helpers**

Create `backend/internal/domain/systemadmin/industry_manifest.go`:

```go
package systemadmin

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const IndustryManifestVersion = "meta-org.industry-solution.v1"

const (
	AssetTypeDatabaseAsset         = "database_asset"
	AssetTypeBusinessFunction      = "business_function"
	AssetTypeProcessLoop           = "process_loop"
	AssetTypeRuntimeOperation      = "runtime_operation"
	AssetTypeUIWorkspace           = "ui_workspace"
	AssetTypePermission            = "permission"
	AssetTypeToolPolicy            = "tool_policy"
	AssetTypeToolDefinition        = "tool_definition"
	AssetTypeAssistantTarget       = "assistant_target"
	AssetTypeContextRule           = "context_rule"
	AssetTypeAssistantSkill        = "assistant_skill"
	AssetTypeQualityGate           = "quality_gate"
	AssetTypeVerificationScenario  = "verification_scenario"
)

type IndustrySolutionManifest struct {
	ManifestVersion      string                  `json:"manifest_version"`
	IndustryKey          string                  `json:"industry_key"`
	PackageKey           string                  `json:"package_key"`
	PackageVersion       string                  `json:"package_version"`
	Assets               []IndustrySolutionAsset `json:"assets"`
	Dependencies         []string                `json:"dependencies,omitempty"`
	QualityGates         []string                `json:"quality_gates,omitempty"`
	VerificationScenarios []string               `json:"verification_scenarios,omitempty"`
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
```

- [ ] **Step 4: Build manifest from existing ERP package metadata**

In `backend/internal/domain/systemadmin/service.go`, keep the existing legacy metadata keys, but construct a manifest before returning the package. Add this helper near `BuildERPSolutionSchemaPackage`:

```go
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
		assets = append(assets, IndustrySolutionAsset{AssetKey: manifestAssetKey(AssetTypeRuntimeOperation, operation), AssetType: AssetTypeRuntimeOperation, Version: "v1", Source: "erp_standard_factory", Owner: "platform", RiskLevel: "medium", Payload: map[string]any{"path": operation}})
	}
	for _, workspace := range stringSliceFromAny(metadata["ui_workspaces"]) {
		assets = append(assets, IndustrySolutionAsset{AssetKey: manifestAssetKey(AssetTypeUIWorkspace, workspace), AssetType: AssetTypeUIWorkspace, Version: "v1", Source: "erp_standard_factory", Owner: "platform", RiskLevel: "low", Payload: map[string]any{"workspace_key": workspace}})
	}
	for _, permission := range stringSliceFromAny(metadata["permissions"]) {
		assets = append(assets, IndustrySolutionAsset{AssetKey: manifestAssetKey(AssetTypePermission, permission), AssetType: AssetTypePermission, Version: "v1", Source: "erp_standard_factory", Owner: "platform", RiskLevel: "medium", Payload: map[string]any{"permission": permission}})
	}
	for _, target := range stringSliceFromAny(metadata["assistant_targets"]) {
		assets = append(assets, IndustrySolutionAsset{AssetKey: manifestAssetKey(AssetTypeAssistantTarget, target), AssetType: AssetTypeAssistantTarget, Version: "v1", Source: "erp_standard_factory", Owner: "platform", RiskLevel: "medium", Payload: map[string]any{"target": target}})
	}

	sortManifestAssets(assets)
	return IndustrySolutionManifest{
		ManifestVersion:      IndustryManifestVersion,
		IndustryKey:          input.IndustryKey,
		PackageKey:           input.PackageKey,
		PackageVersion:       "v1",
		Assets:               assets,
		Dependencies:         []string{"erp.catalog", "erp.action_registry", "platform.schema_change"},
		QualityGates:         stringSliceFromMaps(mapSliceFromAny(metadata["quality_gates"]), "gate_key"),
		VerificationScenarios: stringSliceFromMaps(mapSliceFromAny(metadata["verification_scenarios"]), "scenario_key"),
	}
}
```

Add the small conversion helpers below it:

```go
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
```

In `BuildERPSolutionSchemaPackage`, assign the metadata map to a local variable, build the `SchemaPackage`, then call:

```go
manifest := buildIndustryManifest(input, metadata)
pkg.Metadata = metadata
setIndustryManifest(&pkg, manifest)
return pkg
```

- [ ] **Step 5: Run focused tests**

Run:

```powershell
cd backend
gofmt -w internal/domain/systemadmin/industry_manifest.go internal/domain/systemadmin/industry_manifest_test.go internal/domain/systemadmin/service.go internal/domain/systemadmin/model.go
go test ./internal/domain/systemadmin -run 'TestBuildERPSolutionSchemaPackageIncludesStructuredManifest|TestBuildERPSolutionFlowBuildsCompleteChangePackage' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add backend/internal/domain/systemadmin/industry_manifest.go backend/internal/domain/systemadmin/industry_manifest_test.go backend/internal/domain/systemadmin/service.go backend/internal/domain/systemadmin/model.go
git commit -m "Add structured industry solution manifest"
```

---

## Task 2: Add Package Diff V2 And Query API

**Files:**
- Modify: `backend/internal/domain/systemadmin/industry_manifest.go`
- Modify: `backend/internal/domain/systemadmin/model.go`
- Modify: `backend/internal/domain/systemadmin/service.go`
- Modify: `backend/internal/domain/systemadmin/handler.go`
- Modify: `backend/internal/domain/systemadmin/industry_manifest_test.go`

- [ ] **Step 1: Write the failing diff test**

Append to `backend/internal/domain/systemadmin/industry_manifest_test.go`:

```go
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
```

- [ ] **Step 2: Run the diff test to verify RED**

Run:

```powershell
cd backend
go test ./internal/domain/systemadmin -run TestBuildPackageAssetDiffDetectsManifestAssetChanges -count=1
```

Expected: FAIL with compile errors for `PackageAssetDiff` and `BuildPackageAssetDiff`.

- [ ] **Step 3: Add diff models and builder**

Add to `backend/internal/domain/systemadmin/model.go`:

```go
type PackageAssetDiff struct {
	AssetType      string   `json:"asset_type"`
	AssetKey       string   `json:"asset_key"`
	Action         string   `json:"action"`
	RiskLevel      string   `json:"risk_level"`
	CurrentVersion string   `json:"current_version,omitempty"`
	DesiredVersion string   `json:"desired_version,omitempty"`
	Summary        string   `json:"summary"`
	BlockingReason string   `json:"blocking_reason,omitempty"`
	DependsOn      []string `json:"depends_on,omitempty"`
}
```

Add to `backend/internal/domain/systemadmin/industry_manifest.go`:

```go
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

func mapsEqual(left map[string]any, right map[string]any) bool {
	leftJSON, _ := json.Marshal(left)
	rightJSON, _ := json.Marshal(right)
	return string(leftJSON) == string(rightJSON)
}
```

- [ ] **Step 4: Store package diff when building the ERP solution flow**

In `BuildERPSolutionFlow`, after `pkg := BuildERPSolutionSchemaPackage(input)`, add:

```go
desiredManifest, err := ManifestFromSchemaPackage(pkg)
if err != nil {
	return nil, fmt.Errorf("%w: %v", ErrValidation, err)
}
currentManifest := IndustrySolutionManifest{ManifestVersion: IndustryManifestVersion, IndustryKey: input.IndustryKey, PackageKey: input.PackageKey}
if input.CurrentTemplate != nil {
	if parsed, err := ManifestFromSchemaPackage(*input.CurrentTemplate); err == nil {
		currentManifest = parsed
	}
}
pkg.Metadata["package_diff"] = BuildPackageAssetDiff(currentManifest, desiredManifest)
```

- [ ] **Step 5: Add service method and route**

Add to `backend/internal/domain/systemadmin/service.go`:

```go
func (s *Service) GetSchemaChangePackageDiff(ctx context.Context, actorID uuid.UUID, requestID uuid.UUID) ([]PackageAssetDiff, error) {
	if err := s.requirePlatformPermission(ctx, actorID, platformauth.PermissionSchemaManage); err != nil {
		return nil, err
	}
	request, err := s.repo.GetSchemaChangeRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if diff := PackageDiffFromSchemaPackage(request.SchemaPackage); len(diff) > 0 {
		return diff, nil
	}
	manifest, err := ManifestFromSchemaPackage(request.SchemaPackage)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	return BuildPackageAssetDiff(IndustrySolutionManifest{ManifestVersion: IndustryManifestVersion}, manifest), nil
}
```

In `backend/internal/domain/systemadmin/handler.go`, add the route:

```go
r.Get("/platform/admin/schema-change-requests/{id}/package-diff", h.getSchemaChangePackageDiff)
```

Add the handler:

```go
func (h *Handler) getSchemaChangePackageDiff(w http.ResponseWriter, r *http.Request) {
	actorID, ok := actorIDFromRequest(w, r)
	if !ok {
		return
	}
	requestID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := h.service.GetSchemaChangePackageDiff(r.Context(), actorID, requestID)
	if err != nil {
		writeSystemAdminError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"diff": result})
}
```

- [ ] **Step 6: Run focused tests**

Run:

```powershell
cd backend
gofmt -w internal/domain/systemadmin/industry_manifest.go internal/domain/systemadmin/industry_manifest_test.go internal/domain/systemadmin/model.go internal/domain/systemadmin/service.go internal/domain/systemadmin/handler.go
go test ./internal/domain/systemadmin -run 'TestBuildPackageAssetDiffDetectsManifestAssetChanges|TestBuildERPSolutionFlowBuildsCompleteChangePackage' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```powershell
git add backend/internal/domain/systemadmin
git commit -m "Add industry solution package diff"
```

---

## Task 3: Upgrade Schema Verify To Manifest Gates

**Files:**
- Modify: `backend/internal/domain/systemadmin/industry_manifest.go`
- Modify: `backend/internal/domain/systemadmin/service.go`
- Modify: `backend/internal/domain/systemadmin/service_test.go`

- [ ] **Step 1: Write failing blocking-gate tests**

Append to `backend/internal/domain/systemadmin/service_test.go`:

```go
func TestVerifySchemaChangeBlocksApplyForDuplicateRuntimeOperations(t *testing.T) {
	pkg := BuildERPSolutionSchemaPackage(ERPSolutionFlowRequest{IndustryKey: "professional_services", PackageKey: "erp_standard", Name: "ERP Standard"})
	manifest, err := ManifestFromSchemaPackage(pkg)
	if err != nil {
		t.Fatalf("ManifestFromSchemaPackage error = %v", err)
	}
	manifest.Assets = append(manifest.Assets, IndustrySolutionAsset{
		AssetKey:  "runtime_operation.duplicate",
		AssetType: AssetTypeRuntimeOperation,
		Version:   "v1",
		RiskLevel: "medium",
		Payload:   map[string]any{"path": "/erp/catalog"},
	})
	setIndustryManifest(&pkg, manifest)

	repo := &fakeRepository{role: "system_owner", request: &SchemaChangeRequest{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		SchemaName:     "org_123e4567e89b12d3a456426614174000",
		RequestType:    "erp_solution_flow",
		Status:         SchemaChangeApproved,
		SchemaPackage:  pkg,
		RiskLevel:      SchemaRiskSafe,
	}}
	service := NewService(repo)

	report, err := service.VerifySchemaChange(context.Background(), uuid.New(), repo.request.ID)
	if err != nil {
		t.Fatalf("VerifySchemaChange error = %v", err)
	}
	check := verificationCheckByKey(report, "runtime_operations")
	if check == nil || check.Status != "failed" {
		t.Fatalf("runtime_operations check = %#v, want failed", check)
	}
	if report.CanApply || report.BlockingIssues == 0 {
		t.Fatalf("report can_apply/blocking = %v/%d, want blocked", report.CanApply, report.BlockingIssues)
	}
}

func TestVerifySchemaChangeBlocksActiveContextRules(t *testing.T) {
	pkg := BuildERPSolutionSchemaPackage(ERPSolutionFlowRequest{IndustryKey: "professional_services", PackageKey: "erp_standard", Name: "ERP Standard"})
	manifest, err := ManifestFromSchemaPackage(pkg)
	if err != nil {
		t.Fatalf("ManifestFromSchemaPackage error = %v", err)
	}
	for i := range manifest.Assets {
		if manifest.Assets[i].AssetType == AssetTypeContextRule {
			manifest.Assets[i].Payload["status"] = "active"
			break
		}
	}
	setIndustryManifest(&pkg, manifest)

	repo := &fakeRepository{role: "system_owner", request: &SchemaChangeRequest{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		SchemaName:     "org_123e4567e89b12d3a456426614174000",
		RequestType:    "erp_solution_flow",
		Status:         SchemaChangeApproved,
		SchemaPackage:  pkg,
		RiskLevel:      SchemaRiskSafe,
	}}
	service := NewService(repo)

	report, err := service.VerifySchemaChange(context.Background(), uuid.New(), repo.request.ID)
	if err != nil {
		t.Fatalf("VerifySchemaChange error = %v", err)
	}
	check := verificationCheckByKey(report, "assistant_context")
	if check == nil || check.Status != "failed" {
		t.Fatalf("assistant_context check = %#v, want failed", check)
	}
	if report.CanApply || report.BlockingIssues == 0 {
		t.Fatalf("report can_apply/blocking = %v/%d, want blocked", report.CanApply, report.BlockingIssues)
	}
}
```

- [ ] **Step 2: Run the tests to verify RED**

Run:

```powershell
cd backend
go test ./internal/domain/systemadmin -run 'DuplicateRuntimeOperations|ActiveContextRules' -count=1
```

Expected: FAIL because coverage checks only warn for missing keys and do not inspect manifest contents.

- [ ] **Step 3: Add manifest gate validators**

Add to `backend/internal/domain/systemadmin/industry_manifest.go`:

```go
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
	for _, asset := range manifest.Assets {
		if asset.AssetType != AssetTypeAssistantSkill {
			continue
		}
		if len(stringSliceFromAny(asset.Payload["targets"])) == 0 || len(stringSliceFromAny(asset.Payload["context_rules"])) == 0 || len(stringSliceFromAny(asset.Payload["allowed_tools"])) == 0 {
			return SchemaVerificationCheck{Key: "assistant_skills", Status: "failed", Message: "assistant skill must declare targets, context rules, and allowed tools", Metadata: map[string]any{"asset_key": asset.AssetKey}}
		}
	}
	return SchemaVerificationCheck{Key: "assistant_skills", Status: "passed", Message: "assistant skills declare targets, context rules, and allowed tools", nil}
}

func qualityGateCheck(manifest IndustrySolutionManifest) SchemaVerificationCheck {
	if len(manifest.QualityGates) == 0 {
		return SchemaVerificationCheck{Key: "quality_gates", Status: "warning", Message: "industry package should declare quality gates", nil}
	}
	return SchemaVerificationCheck{Key: "quality_gates", Status: "passed", Message: "quality gates are declared", map[string]any{"count": len(manifest.QualityGates)}}
}

func verificationScenarioCheck(manifest IndustrySolutionManifest) SchemaVerificationCheck {
	if len(manifest.VerificationScenarios) == 0 {
		return SchemaVerificationCheck{Key: "verification_scenarios", Status: "warning", Message: "industry package should declare verification scenarios", nil}
	}
	return SchemaVerificationCheck{Key: "verification_scenarios", Status: "passed", Message: "verification scenarios are declared", map[string]any{"count": len(manifest.VerificationScenarios)}}
}

func rollbackRiskCheck(manifest IndustrySolutionManifest, riskLevel string) SchemaVerificationCheck {
	if riskLevel == SchemaRiskDestructive {
		for _, asset := range manifest.Assets {
			if stringValue(asset.Payload["rollback_plan"]) != "" {
				return SchemaVerificationCheck{Key: "rollback_risk", Status: "warning", Message: "destructive change includes a rollback plan for manual review", nil}
			}
		}
		return SchemaVerificationCheck{Key: "rollback_risk", Status: "failed", Message: "destructive industry package requires rollback_plan metadata", nil}
	}
	return SchemaVerificationCheck{Key: "rollback_risk", Status: "passed", Message: "rollback risk is low for additive factory package", map[string]any{"risk_level": riskLevel}}
}
```

- [ ] **Step 4: Replace coverage checks with manifest checks**

In `addIndustryFactoryCoverageChecks`, parse the manifest and add validator checks:

```go
manifest, err := ManifestFromSchemaPackage(request.SchemaPackage)
if err != nil {
	report.addCheck("industry_manifest", "failed", err.Error(), nil)
	return
}
for _, check := range ManifestVerificationChecks(manifest, report.RiskLevel) {
	report.addCheck(check.Key, check.Status, check.Message, check.Metadata)
}
```

Keep `permissions_impact` as a coverage check before manifest checks:

```go
addMetadataCoverageCheck(report, request, "permissions_impact", []string{"permissions"}, "package declares permission impact", "industry package should declare permission impact")
```

- [ ] **Step 5: Run verify tests**

Run:

```powershell
cd backend
gofmt -w internal/domain/systemadmin/industry_manifest.go internal/domain/systemadmin/service.go internal/domain/systemadmin/service_test.go
go test ./internal/domain/systemadmin -run 'VerifySchemaChangeReportsIndustryFactoryCoverage|WarnsWhenIndustryFactoryCoverageIsIncomplete|DuplicateRuntimeOperations|ActiveContextRules' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add backend/internal/domain/systemadmin
git commit -m "Add manifest-backed schema verification gates"
```

---

## Task 4: Apply Manifest Assets And Persist Asset Results

**Files:**
- Modify: `backend/internal/domain/systemadmin/model.go`
- Modify: `backend/internal/domain/systemadmin/industry_manifest.go`
- Modify: `backend/internal/domain/systemadmin/service.go`
- Modify: `backend/internal/domain/systemadmin/repository.go`
- Modify: `backend/internal/domain/systemadmin/service_test.go`
- Modify: `backend/internal/domain/systemadmin/industry_manifest_test.go`

- [ ] **Step 1: Write failing apply gating test**

Append to `backend/internal/domain/systemadmin/service_test.go`:

```go
func TestApplySchemaChangeRejectsManifestBlockingIssues(t *testing.T) {
	pkg := BuildERPSolutionSchemaPackage(ERPSolutionFlowRequest{IndustryKey: "professional_services", PackageKey: "erp_standard", Name: "ERP Standard"})
	manifest, err := ManifestFromSchemaPackage(pkg)
	if err != nil {
		t.Fatalf("ManifestFromSchemaPackage error = %v", err)
	}
	for i := range manifest.Assets {
		if manifest.Assets[i].AssetType == AssetTypeContextRule {
			manifest.Assets[i].Payload["status"] = "active"
			break
		}
	}
	setIndustryManifest(&pkg, manifest)

	repo := &fakeRepository{role: "system_owner", request: &SchemaChangeRequest{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		SchemaName:     "org_123e4567e89b12d3a456426614174000",
		RequestType:    "erp_solution_flow",
		Status:         SchemaChangeApproved,
		SchemaPackage:  pkg,
		RiskLevel:      SchemaRiskSafe,
	}}
	service := NewService(repo)

	_, err = service.ApplySchemaChange(context.Background(), uuid.New(), repo.request.ID)
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("ApplySchemaChange error = %v, want ErrValidation", err)
	}
	if repo.applied {
		t.Fatal("ApplySchemaChange applied request with blocking manifest issue")
	}
}
```

- [ ] **Step 2: Write failing apply result helper test**

Append to `backend/internal/domain/systemadmin/industry_manifest_test.go`:

```go
func TestBuildSchemaApplyAssetResultsIncludesManifestAssets(t *testing.T) {
	pkg := BuildERPSolutionSchemaPackage(ERPSolutionFlowRequest{IndustryKey: "professional_services", PackageKey: "erp_standard", Name: "ERP Standard"})
	manifest, err := ManifestFromSchemaPackage(pkg)
	if err != nil {
		t.Fatalf("ManifestFromSchemaPackage error = %v", err)
	}

	results := BuildSchemaApplyAssetResults(manifest)

	if len(results) == 0 {
		t.Fatal("BuildSchemaApplyAssetResults returned no results")
	}
	if !hasAssetResult(results, AssetTypeRuntimeOperation) {
		t.Fatalf("results missing runtime operation asset in %#v", results)
	}
	if !hasAssetResult(results, AssetTypeContextRule) {
		t.Fatalf("results missing context rule asset in %#v", results)
	}
}

func hasAssetResult(results []SchemaApplyAssetResult, assetType string) bool {
	for _, result := range results {
		if result.AssetType == assetType && result.Status == "pending" {
			return true
		}
	}
	return false
}
```

- [ ] **Step 3: Run tests to verify RED**

Run:

```powershell
cd backend
go test ./internal/domain/systemadmin -run 'ApplySchemaChangeRejectsManifestBlockingIssues|BuildSchemaApplyAssetResultsIncludesManifestAssets' -count=1
```

Expected: FAIL because apply does not call verify and `SchemaApplyAssetResult` does not exist.

- [ ] **Step 4: Add apply result model and helper**

Add to `backend/internal/domain/systemadmin/model.go`:

```go
type SchemaApplyAssetResult struct {
	AssetKey     string         `json:"asset_key"`
	AssetType    string         `json:"asset_type"`
	Status       string         `json:"status"`
	Target       string         `json:"target"`
	ErrorMessage string         `json:"error_message,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}
```

Add to `backend/internal/domain/systemadmin/industry_manifest.go`:

```go
func BuildSchemaApplyAssetResults(manifest IndustrySolutionManifest) []SchemaApplyAssetResult {
	results := make([]SchemaApplyAssetResult, 0, len(manifest.Assets))
	for _, asset := range manifest.Assets {
		results = append(results, SchemaApplyAssetResult{
			AssetKey:  asset.AssetKey,
			AssetType: asset.AssetType,
			Status:    "pending",
			Target:    applyTargetForAsset(asset),
			Metadata:  map[string]any{"risk_level": asset.RiskLevel, "version": asset.Version},
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
```

- [ ] **Step 5: Make apply enforce verify gates**

In `ApplySchemaChange`, call `VerifySchemaChange` before applying:

```go
report, err := s.VerifySchemaChange(ctx, actorID, requestID)
if err != nil {
	return nil, err
}
if !report.CanApply {
	return nil, fmt.Errorf("%w: schema change verification has %d blocking issues", ErrValidation, report.BlockingIssues)
}
```

Build asset results and pass them to the repository:

```go
assetResults := []SchemaApplyAssetResult{}
if manifest, err := ManifestFromSchemaPackage(request.SchemaPackage); err == nil {
	assetResults = BuildSchemaApplyAssetResults(manifest)
}
return s.repo.ApplySchemaChange(ctx, request, statements, assetResults)
```

Update the repository interface in `service.go`:

```go
ApplySchemaChange(context.Context, *SchemaChangeRequest, []string, []SchemaApplyAssetResult) (*SchemaApplyJob, error)
```

Update `fakeRepository.ApplySchemaChange` in `service_test.go`:

```go
func (f *fakeRepository) ApplySchemaChange(_ context.Context, _ *SchemaChangeRequest, _ []string, assetResults []SchemaApplyAssetResult) (*SchemaApplyJob, error) {
	f.applied = true
	return &SchemaApplyJob{Metadata: map[string]any{"asset_results": assetResults}}, nil
}
```

- [ ] **Step 6: Persist asset results in PostgreSQL apply job metadata**

Modify `backend/internal/domain/systemadmin/repository.go` signature:

```go
func (r *Repository) ApplySchemaChange(ctx context.Context, request *SchemaChangeRequest, statements []string, assetResults []SchemaApplyAssetResult) (*SchemaApplyJob, error)
```

Before inserting the apply job, call a new helper:

```go
assetResults = r.applyIndustrySolutionAssets(ctx, tx, request, assetResults)
metadataJSON, _ := json.Marshal(map[string]any{
	"source":        "systemadmin",
	"asset_results": assetResults,
})
```

Use `metadataJSON` in the `INSERT INTO platform.schema_apply_jobs` statement:

```go
VALUES ($1, $2, $3, 'applied', $4, $5::jsonb)
```

Add the helper:

```go
func (r *Repository) applyIndustrySolutionAssets(ctx context.Context, tx pgx.Tx, request *SchemaChangeRequest, results []SchemaApplyAssetResult) []SchemaApplyAssetResult {
	for i := range results {
		err := r.applyIndustrySolutionAsset(ctx, tx, request, &results[i])
		if err != nil {
			results[i].Status = "failed"
			results[i].ErrorMessage = err.Error()
			continue
		}
		results[i].Status = "applied"
	}
	return results
}

func (r *Repository) applyIndustrySolutionAsset(ctx context.Context, tx pgx.Tx, request *SchemaChangeRequest, result *SchemaApplyAssetResult) error {
	switch result.AssetType {
	case AssetTypeRuntimeOperation:
		_, err := tx.Exec(ctx, `
			INSERT INTO platform.runtime_operations(operation_key, domain, title, method, path, operation_kind, danger_level, result_view, assistant_eligible, status, action_type, metadata)
			VALUES ($1, 'ERP', $2, 'POST', $3, 'contextual', 'medium', 'summary', true, 'active', 'erp.action', $4::jsonb)
			ON CONFLICT (operation_key) DO UPDATE SET
				title = EXCLUDED.title,
				path = EXCLUDED.path,
				metadata = platform.runtime_operations.metadata || EXCLUDED.metadata,
				updated_at = NOW()
		`, result.AssetKey, result.AssetKey, stringValue(result.Metadata["path"]), jsonBytes(map[string]any{"source_change_request_id": request.ID.String()}))
		return err
	case AssetTypeToolDefinition, AssetTypeToolPolicy:
		_, err := tx.Exec(ctx, `
			INSERT INTO tool_definitions(name, description, source_type, default_policy, risk_level, required_level, metadata)
			VALUES ($1, 'Generated from ERP industry solution package', 'internal_api', 'approve', 'medium', 'L2', $2::jsonb)
			ON CONFLICT (name) DO UPDATE SET
				metadata = tool_definitions.metadata || EXCLUDED.metadata,
				updated_at = NOW()
		`, result.AssetKey, jsonBytes(map[string]any{"source_change_request_id": request.ID.String()}))
		return err
	default:
		_, err := tx.Exec(ctx, `
			INSERT INTO platform.platform_masters(module_key, entity_type, source_table, source_pk, title, status, organization_id, payload, metadata)
			VALUES ('industry_solution_factory', $1, 'industry_solution_asset', $2, $3, 'draft', $4, '{}'::jsonb, $5::jsonb)
			ON CONFLICT (source_table, source_pk) WHERE source_table <> '' AND source_pk <> ''
			DO UPDATE SET
				title = EXCLUDED.title,
				status = EXCLUDED.status,
				metadata = platform.platform_masters.metadata || EXCLUDED.metadata,
				updated_at = NOW()
		`, result.AssetType, request.ID.String()+":"+result.AssetKey, result.AssetKey, request.OrganizationID, jsonBytes(map[string]any{"source_change_request_id": request.ID.String(), "target": result.Target}))
		return err
	}
}
```

Add imports for `github.com/jackc/pgx/v5` if it is not already imported by `repository.go`.

- [ ] **Step 7: Run apply tests**

Run:

```powershell
cd backend
gofmt -w internal/domain/systemadmin/model.go internal/domain/systemadmin/industry_manifest.go internal/domain/systemadmin/service.go internal/domain/systemadmin/repository.go internal/domain/systemadmin/service_test.go internal/domain/systemadmin/industry_manifest_test.go
go test ./internal/domain/systemadmin -run 'ApplySchemaChangeRejectsManifestBlockingIssues|BuildSchemaApplyAssetResultsIncludesManifestAssets|ApplySchemaChangeRejectsPendingRequest|VerifySchemaChangeReportsIndustryFactoryCoverage' -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit**

```powershell
git add backend/internal/domain/systemadmin
git commit -m "Apply industry solution manifest assets"
```

---

## Task 5: Add Custom Package Publication Gates

**Files:**
- Modify: `backend/internal/domain/industry/model.go`
- Modify: `backend/internal/domain/industry/service.go`
- Modify: `backend/internal/domain/industry/repository.go`
- Modify: `backend/internal/domain/industry/service_test.go`

- [ ] **Step 1: Write failing publication gate tests**

Append to `backend/internal/domain/industry/service_test.go`:

```go
func TestReviewPublicationRequestRejectsFailedPublicationGate(t *testing.T) {
	actorID := uuid.New()
	requestID := uuid.New()
	extensionID := uuid.New()
	repo := &fakeRepository{
		role: "system_owner",
		publicationRequest: &PublicationRequest{
			ID:          requestID,
			ExtensionID: extensionID,
			Status:      PublicationPending,
			Metadata:    map[string]any{},
		},
		extension: &Extension{
			ID:             extensionID,
			OrganizationID: uuid.New(),
			IndustryKey:    "professional_services",
			ExtensionKey:   "customer_specific_extension",
			Name:           "Customer specific extension",
			Assets: []PackageAsset{
				{AssetKey: "customer_export", AssetType: AssetTypeRuntimeEntity, Payload: map[string]any{"customer_name": "Acme Corp"}},
			},
		},
	}
	service := NewService(repo)

	_, err := service.ReviewPublicationRequest(context.Background(), actorID, requestID, PublicationApproved, "approve")
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("ReviewPublicationRequest error = %v, want ErrValidation", err)
	}
	if repo.reviewedStatus == PublicationApproved {
		t.Fatal("publication request approved despite failed gate")
	}
}

func TestReviewPublicationRequestAllowsWarningsAndPersistsGateMetadata(t *testing.T) {
	actorID := uuid.New()
	requestID := uuid.New()
	extensionID := uuid.New()
	repo := &fakeRepository{
		role: "system_owner",
		publicationRequest: &PublicationRequest{
			ID:          requestID,
			ExtensionID: extensionID,
			Status:      PublicationPending,
			Metadata:    map[string]any{},
		},
		extension: &Extension{
			ID:             extensionID,
			OrganizationID: uuid.New(),
			IndustryKey:    "professional_services",
			ExtensionKey:   "verified_extension",
			Name:           "Verified extension",
			Metadata: map[string]any{
				"required_verification_scenarios": []any{"source_to_pay_smoke"},
				"verification_scenario_results": []any{
					map[string]any{"scenario_key": "source_to_pay_smoke", "status": "warning"},
				},
			},
			Assets: []PackageAsset{
				{AssetKey: "knowledge_source.safe", AssetType: AssetTypeKnowledgeSource, Payload: map[string]any{"permission": map[string]any{"allow_publication": true}}},
			},
		},
	}
	service := NewService(repo)

	result, err := service.ReviewPublicationRequest(context.Background(), actorID, requestID, PublicationApproved, "approve")
	if err != nil {
		t.Fatalf("ReviewPublicationRequest error = %v", err)
	}
	if result.Status != PublicationApproved {
		t.Fatalf("status = %q, want approved", result.Status)
	}
	if len(repo.publicationGateResults) == 0 {
		t.Fatal("gate results were not persisted")
	}
}
```

- [ ] **Step 2: Run tests to verify RED**

Run:

```powershell
cd backend
go test ./internal/domain/industry -run 'PublicationRequestRejectsFailedPublicationGate|PublicationRequestAllowsWarnings' -count=1
```

Expected: FAIL because the fake repository and service do not support publication gate metadata.

- [ ] **Step 3: Add publication gate model**

Add to `backend/internal/domain/industry/model.go`:

```go
type PublicationGateResult struct {
	Key      string         `json:"key"`
	Status   string         `json:"status"`
	Message  string         `json:"message"`
	Metadata map[string]any `json:"metadata,omitempty"`
}
```

- [ ] **Step 4: Extend repository interface and fake repository**

In `backend/internal/domain/industry/service.go`, change repository interface:

```go
CreatePublicationRequest(context.Context, Extension, uuid.UUID, string, map[string]any) (*PublicationRequest, error)
GetPublicationRequest(context.Context, uuid.UUID) (*PublicationRequest, error)
UpdatePublicationRequestMetadata(context.Context, uuid.UUID, map[string]any) error
```

Extend the fake repository in `service_test.go` with:

```go
publicationRequest    *PublicationRequest
publicationGateResults []PublicationGateResult
reviewedStatus        string
```

Add fake methods:

```go
func (f *fakeRepository) GetPublicationRequest(context.Context, uuid.UUID) (*PublicationRequest, error) {
	return f.publicationRequest, nil
}

func (f *fakeRepository) UpdatePublicationRequestMetadata(_ context.Context, _ uuid.UUID, metadata map[string]any) error {
	if gates, ok := metadata["publication_gates"].([]PublicationGateResult); ok {
		f.publicationGateResults = gates
	}
	if f.publicationRequest != nil {
		f.publicationRequest.Metadata = metadata
	}
	return nil
}
```

Update existing fake `ReviewPublicationRequest` to set `f.reviewedStatus = status`.

- [ ] **Step 5: Implement gate evaluation**

Add to `backend/internal/domain/industry/service.go`:

```go
func EvaluatePublicationGates(extension Extension) []PublicationGateResult {
	return []PublicationGateResult{
		anonymizationGate(extension),
		knowledgeSourcePermissionGate(extension),
		verificationScenarioGate(extension),
	}
}

func anonymizationGate(extension Extension) PublicationGateResult {
	for _, asset := range extension.Assets {
		if containsSensitivePublicationData(asset.Payload) {
			return PublicationGateResult{Key: "anonymization_check", Status: "failed", Message: "extension assets contain customer, user, order, or payment identifiers", Metadata: map[string]any{"asset_key": asset.AssetKey}}
		}
	}
	return PublicationGateResult{Key: "anonymization_check", Status: "passed", Message: "extension assets contain no blocked identifiers", nil}
}

func knowledgeSourcePermissionGate(extension Extension) PublicationGateResult {
	count := 0
	for _, asset := range extension.Assets {
		if asset.AssetType != AssetTypeKnowledgeSource {
			continue
		}
		count++
		permission, _ := asset.Payload["permission"].(map[string]any)
		if permission["allow_publication"] != true {
			return PublicationGateResult{Key: "knowledge_source_permission_check", Status: "failed", Message: "knowledge source asset is missing publication permission", Metadata: map[string]any{"asset_key": asset.AssetKey}}
		}
	}
	return PublicationGateResult{Key: "knowledge_source_permission_check", Status: "passed", Message: "knowledge source permissions allow publication", map[string]any{"count": count}}
}

func verificationScenarioGate(extension Extension) PublicationGateResult {
	required := stringSliceFromAny(extension.Metadata["required_verification_scenarios"])
	if len(required) == 0 {
		return PublicationGateResult{Key: "verification_scenario_check", Status: "warning", Message: "extension declares no required verification scenarios", nil}
	}
	results := map[string]string{}
	for _, item := range mapSliceFromAny(extension.Metadata["verification_scenario_results"]) {
		results[stringValue(item["scenario_key"])] = stringValue(item["status"])
	}
	missing := []string{}
	for _, scenario := range required {
		status := results[scenario]
		if status != "passed" && status != "warning" {
			missing = append(missing, scenario)
		}
	}
	if len(missing) > 0 {
		return PublicationGateResult{Key: "verification_scenario_check", Status: "failed", Message: "required verification scenarios must pass or be explicitly warning", Metadata: map[string]any{"missing": missing}}
	}
	return PublicationGateResult{Key: "verification_scenario_check", Status: "passed", Message: "required verification scenarios passed or warned", map[string]any{"count": len(required)}}
}

func containsSensitivePublicationData(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			normalized := strings.ToLower(key)
			if strings.Contains(normalized, "customer") || strings.Contains(normalized, "user_email") || strings.Contains(normalized, "order_number") || strings.Contains(normalized, "payment") || strings.Contains(normalized, "phone") || strings.Contains(normalized, "id_card") {
				return true
			}
			if containsSensitivePublicationData(item) {
				return true
			}
		}
	case []any:
		for _, item := range typed {
			if containsSensitivePublicationData(item) {
				return true
			}
		}
	}
	return false
}

func publicationGatesBlock(gates []PublicationGateResult) bool {
	for _, gate := range gates {
		if gate.Status == "failed" {
			return true
		}
	}
	return false
}
```

- [ ] **Step 6: Enforce gates on submit and approve**

In `SubmitPublicationRequest`, evaluate gates and pass metadata:

```go
gates := EvaluatePublicationGates(*extension)
metadata := map[string]any{"extension_key": extension.ExtensionKey, "publication_gates": gates}
return s.repo.CreatePublicationRequest(ctx, *extension, actorID, strings.TrimSpace(reason), metadata)
```

In `ReviewPublicationRequest`, before approving:

```go
request, err := s.repo.GetPublicationRequest(ctx, requestID)
if err != nil {
	return nil, err
}
extension, err := s.repo.GetExtension(ctx, request.ExtensionID)
if err != nil {
	return nil, err
}
gates := EvaluatePublicationGates(*extension)
metadata := request.Metadata
if metadata == nil {
	metadata = map[string]any{}
}
metadata["publication_gates"] = gates
if err := s.repo.UpdatePublicationRequestMetadata(ctx, requestID, metadata); err != nil {
	return nil, err
}
if status == PublicationApproved && publicationGatesBlock(gates) {
	return nil, fmt.Errorf("%w: publication gates failed", ErrValidation)
}
```

- [ ] **Step 7: Update PostgreSQL repository**

In `backend/internal/domain/industry/repository.go`, update `CreatePublicationRequest` to accept metadata:

```go
func (r *Repository) CreatePublicationRequest(ctx context.Context, extension Extension, actorID uuid.UUID, reason string, metadata map[string]any) (*PublicationRequest, error)
```

Use `jsonBytes(metadata)` in the insert. Add:

```go
func (r *Repository) GetPublicationRequest(ctx context.Context, requestID uuid.UUID) (*PublicationRequest, error) {
	item := &PublicationRequest{}
	err := scanPublicationRequest(r.db.QueryRow(ctx, `
		SELECT id, extension_id, source_organization_id, industry_key, status, reason, review_reason,
			requested_by, reviewed_by, metadata, created_at, updated_at, reviewed_at
		FROM platform.custom_package_publication_requests
		WHERE id = $1
	`, requestID), item)
	if err != nil {
		return nil, fmt.Errorf("get publication request: %w", err)
	}
	return item, nil
}

func (r *Repository) UpdatePublicationRequestMetadata(ctx context.Context, requestID uuid.UUID, metadata map[string]any) error {
	_, err := r.db.Exec(ctx, `
		UPDATE platform.custom_package_publication_requests
		SET metadata = $2::jsonb, updated_at = NOW()
		WHERE id = $1
	`, requestID, jsonBytes(metadata))
	if err != nil {
		return fmt.Errorf("update publication request metadata: %w", err)
	}
	return nil
}
```

- [ ] **Step 8: Run industry tests**

Run:

```powershell
cd backend
gofmt -w internal/domain/industry/model.go internal/domain/industry/service.go internal/domain/industry/repository.go internal/domain/industry/service_test.go
go test ./internal/domain/industry -run 'PublicationRequest|ValidatePackage' -count=1
```

Expected: PASS.

- [ ] **Step 9: Commit**

```powershell
git add backend/internal/domain/industry
git commit -m "Add industry publication gates"
```

---

## Task 6: Surface Phase 2 Factory Data In SystemAdmin UI

**Files:**
- Modify: `frontend/src/lib/api.ts`
- Modify: `frontend/src/app/system-admin-workspace.tsx`
- Modify: `frontend/src/lib/i18n.tsx`
- Modify: `frontend/verify-system-admin-workspace.mjs`
- Modify: `frontend/package.json` only if the existing verify script is not already wired to a package script.

- [ ] **Step 1: Extend the static verification script first**

Modify `frontend/verify-system-admin-workspace.mjs` and append these snippets to the existing arrays:

```js
requiredApiExports.push('getSchemaChangePackageDiff')

requiredApiTypes.push(
  'IndustrySolutionManifest',
  'IndustrySolutionAsset',
  'PackageAssetDiff',
  'SchemaApplyAssetResult',
  'PublicationGateResult',
)

requiredI18nKeys.push(
  'systemAdmin.packageAssets',
  'systemAdmin.assetResults',
  'systemAdmin.publicationGates',
  'systemAdmin.blockingReason',
  'systemAdmin.metadataAssets',
  'systemAdmin.assetType.database_asset',
  'systemAdmin.assetType.business_function',
  'systemAdmin.assetType.process_loop',
  'systemAdmin.assetType.runtime_operation',
  'systemAdmin.assetType.tool_policy',
  'systemAdmin.assetType.tool_definition',
  'systemAdmin.assetType.context_rule',
  'systemAdmin.assetType.assistant_skill',
  'systemAdmin.assetType.quality_gate',
  'systemAdmin.assetType.verification_scenario',
  'systemAdmin.check.industry_manifest',
  'systemAdmin.gate.anonymization_check',
  'systemAdmin.gate.knowledge_source_permission_check',
  'systemAdmin.gate.verification_scenario_check',
)

requiredWorkspaceSnippets.push(
  'getSchemaChangePackageDiff',
  'packageAssetDiff',
  'packageAssetsByType',
  'assetResults',
  'publication_gates',
  "t('systemAdmin.packageAssets')",
  "t('systemAdmin.assetResults')",
  "t('systemAdmin.publicationGates')",
)
```

- [ ] **Step 2: Run the static check to verify RED**

Run:

```powershell
cd frontend
node verify-system-admin-workspace.mjs
```

Expected: FAIL listing missing API types, workspace snippets, and i18n keys.

- [ ] **Step 3: Add frontend API types and helper**

In `frontend/src/lib/api.ts`, add:

```ts
export interface IndustrySolutionManifest {
  manifest_version: string
  industry_key: string
  package_key: string
  package_version: string
  assets: IndustrySolutionAsset[]
  dependencies?: string[]
  quality_gates?: string[]
  verification_scenarios?: string[]
}

export interface IndustrySolutionAsset {
  asset_key: string
  asset_type: string
  version: string
  source: string
  owner: string
  risk_level: string
  depends_on?: string[]
  payload: Record<string, unknown>
}

export interface PackageAssetDiff {
  asset_type: string
  asset_key: string
  action: string
  risk_level: string
  current_version?: string
  desired_version?: string
  summary: string
  blocking_reason?: string
  depends_on?: string[]
}

export interface SchemaApplyAssetResult {
  asset_key: string
  asset_type: string
  status: string
  target: string
  error_message?: string
  metadata?: Record<string, unknown>
}

export interface PublicationGateResult {
  key: string
  status: string
  message: string
  metadata?: Record<string, unknown>
}
```

Extend `SchemaApplyJob`:

```ts
metadata: {
  asset_results?: SchemaApplyAssetResult[]
  [key: string]: unknown
}
```

Add the API helper near schema change helpers:

```ts
export async function getSchemaChangePackageDiff(token: string, requestID: string): Promise<PackageAssetDiff[]> {
  const result = await apiRequest<{ diff: PackageAssetDiff[] }>(
    `/platform/admin/schema-change-requests/${encodeURIComponent(requestID)}/package-diff`,
    { token },
  )
  return result.diff
}
```

- [ ] **Step 4: Add SystemAdmin workspace state and derived data**

In `frontend/src/app/system-admin-workspace.tsx`, import the new helper and types:

```ts
  getSchemaChangePackageDiff,
  type IndustrySolutionManifest,
  type PackageAssetDiff,
  type PublicationGateResult,
  type SchemaApplyAssetResult,
```

Add state:

```ts
const [packageAssetDiff, setPackageAssetDiff] = useState<PackageAssetDiff[]>([])
```

Add derived values:

```ts
const industryManifest = useMemo(() => {
  const manifest = changeRequest?.schema_package.metadata?.industry_manifest
  return manifest && typeof manifest === 'object' ? (manifest as IndustrySolutionManifest) : null
}, [changeRequest])

const packageAssetsByType = useMemo(() => {
  const groups = new Map<string, IndustrySolutionManifest['assets']>()
  for (const asset of industryManifest?.assets ?? []) {
    groups.set(asset.asset_type, [...(groups.get(asset.asset_type) ?? []), asset])
  }
  return Array.from(groups.entries()).map(([assetType, assets]) => ({ assetType, assets }))
}, [industryManifest])

const assetResults = useMemo(
  () => ((applyJob?.metadata?.asset_results as SchemaApplyAssetResult[] | undefined) ?? []),
  [applyJob],
)

const publicationGates = (item: IndustryPublicationRequest): PublicationGateResult[] =>
  ((item.metadata?.publication_gates as PublicationGateResult[] | undefined) ?? [])
```

After creating a flow or verifying a change, fetch package diff:

```ts
setPackageAssetDiff(await getSchemaChangePackageDiff(token, request.id))
```

- [ ] **Step 5: Render assets, package diff, apply results, and publication gates**

Add a package assets section inside the schema side panel when `changeRequest` exists:

```tsx
{packageAssetsByType.length > 0 && (
  <div className="mt-4">
    <p className="text-xs font-semibold text-slate-500">{t('systemAdmin.packageAssets')}</p>
    <div className="mt-2 grid gap-2 sm:grid-cols-2">
      {packageAssetsByType.map((group) => (
        <div key={group.assetType} className="rounded-lg border border-slate-200 bg-slate-50 p-3">
          <div className="flex items-center justify-between gap-2">
            <p className="min-w-0 truncate text-sm font-semibold text-slate-900">{t(`systemAdmin.assetType.${group.assetType}`)}</p>
            <StatusBadge label={String(group.assets.length)} />
          </div>
          <p className="mt-1 truncate text-xs text-slate-500">{group.assets[0]?.asset_key || t('common.empty')}</p>
        </div>
      ))}
    </div>
  </div>
)}
```

Render package diff V2:

```tsx
{packageAssetDiff.length > 0 && (
  <div className="mt-4">
    <p className="text-xs font-semibold text-slate-500">{t('systemAdmin.packageDiff')}</p>
    <div className="mt-2 space-y-2">
      {packageAssetDiff.map((item) => (
        <div key={`${item.asset_type}-${item.asset_key}`} className="rounded-lg border border-slate-200 bg-slate-50 p-3">
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0">
              <p className="truncate text-sm font-semibold text-slate-900">{item.asset_key}</p>
              <p className="mt-1 text-xs text-slate-500">{t(`systemAdmin.assetType.${item.asset_type}`)} / {item.risk_level}</p>
            </div>
            <StatusBadge label={item.action} />
          </div>
          {item.blocking_reason && <p className="mt-2 rounded-md bg-red-50 p-2 text-xs text-red-700">{t('systemAdmin.blockingReason')}: {item.blocking_reason}</p>}
        </div>
      ))}
    </div>
  </div>
)}
```

Render apply asset results under `applyJob`:

```tsx
{assetResults.length > 0 && (
  <div className="mt-4 space-y-2">
    <p className="text-xs font-semibold text-slate-500">{t('systemAdmin.assetResults')}</p>
    {assetResults.map((item) => (
      <div key={`${item.asset_type}-${item.asset_key}`} className="rounded-lg border border-slate-200 bg-slate-50 p-3">
        <div className="flex items-start justify-between gap-3">
          <p className="min-w-0 truncate text-sm font-semibold text-slate-900">{item.asset_key}</p>
          <StatusBadge label={item.status} />
        </div>
        <p className="mt-1 truncate text-xs text-slate-500">{item.target}</p>
        {item.error_message && <p className="mt-2 rounded-md bg-red-50 p-2 text-xs text-red-700">{item.error_message}</p>}
      </div>
    ))}
  </div>
)}
```

Inside the publication request card, render gates:

```tsx
{publicationGates(item).length > 0 && (
  <div className="mt-3 space-y-2">
    <p className="text-xs font-semibold text-slate-500">{t('systemAdmin.publicationGates')}</p>
    {publicationGates(item).map((gate) => (
      <div key={gate.key} className="rounded-md border border-slate-200 bg-slate-50 p-2">
        <div className="flex items-start justify-between gap-2">
          <p className="text-xs font-semibold text-slate-700">{t(`systemAdmin.gate.${gate.key}`)}</p>
          <StatusBadge label={gate.status} />
        </div>
        <p className="mt-1 text-xs text-slate-500">{gate.message}</p>
      </div>
    ))}
  </div>
)}
```

- [ ] **Step 6: Add bilingual i18n keys**

Add English keys in `frontend/src/lib/i18n.tsx`:

```ts
'systemAdmin.packageAssets': 'Package assets',
'systemAdmin.assetResults': 'Asset apply results',
'systemAdmin.publicationGates': 'Publication gates',
'systemAdmin.blockingReason': 'Blocking reason',
'systemAdmin.metadataAssets': 'Metadata assets',
'systemAdmin.assetType.database_asset': 'Database asset',
'systemAdmin.assetType.business_function': 'Business function',
'systemAdmin.assetType.process_loop': 'Process loop',
'systemAdmin.assetType.runtime_operation': 'Runtime operation',
'systemAdmin.assetType.ui_workspace': 'UI workspace',
'systemAdmin.assetType.permission': 'Permission',
'systemAdmin.assetType.tool_policy': 'Tool policy',
'systemAdmin.assetType.tool_definition': 'Tool definition',
'systemAdmin.assetType.assistant_target': 'Assistant target',
'systemAdmin.assetType.context_rule': 'Context rule',
'systemAdmin.assetType.assistant_skill': 'Assistant skill',
'systemAdmin.assetType.quality_gate': 'Quality gate',
'systemAdmin.assetType.verification_scenario': 'Verification scenario',
'systemAdmin.check.industry_manifest': 'Industry manifest',
'systemAdmin.gate.anonymization_check': 'Anonymization',
'systemAdmin.gate.knowledge_source_permission_check': 'Knowledge source permission',
'systemAdmin.gate.verification_scenario_check': 'Verification scenario',
```

Add Chinese keys:

```ts
'systemAdmin.packageAssets': '方案资产',
'systemAdmin.assetResults': '资产应用结果',
'systemAdmin.publicationGates': '发布门禁',
'systemAdmin.blockingReason': '阻塞原因',
'systemAdmin.metadataAssets': '元数据资产',
'systemAdmin.assetType.database_asset': '数据库资产',
'systemAdmin.assetType.business_function': '业务功能',
'systemAdmin.assetType.process_loop': '流程闭环',
'systemAdmin.assetType.runtime_operation': '运行时操作',
'systemAdmin.assetType.ui_workspace': '界面工作区',
'systemAdmin.assetType.permission': '权限',
'systemAdmin.assetType.tool_policy': '工具策略',
'systemAdmin.assetType.tool_definition': '工具定义',
'systemAdmin.assetType.assistant_target': '助手目标',
'systemAdmin.assetType.context_rule': '上下文规则',
'systemAdmin.assetType.assistant_skill': '助手技能',
'systemAdmin.assetType.quality_gate': '质量门禁',
'systemAdmin.assetType.verification_scenario': '验证场景',
'systemAdmin.check.industry_manifest': '行业方案清单',
'systemAdmin.gate.anonymization_check': '匿名化',
'systemAdmin.gate.knowledge_source_permission_check': '知识来源权限',
'systemAdmin.gate.verification_scenario_check': '验证场景',
```

- [ ] **Step 7: Run frontend verification**

Run:

```powershell
cd frontend
npm run lint
npm run build
node verify-system-admin-workspace.mjs
```

Expected: all exit 0.

- [ ] **Step 8: Commit**

```powershell
git add frontend/src/lib/api.ts frontend/src/app/system-admin-workspace.tsx frontend/src/lib/i18n.tsx frontend/verify-system-admin-workspace.mjs frontend/package.json
git commit -m "Show industry solution factory gates in SystemAdmin"
```

---

## Task 7: Document Baseline Governance And Run Migration Verification

**Files:**
- Modify: `migrations/000_saas_platform_management_baseline.sql`
- Modify: `migrations/BASELINE_RESTRUCTURE.md`

- [ ] **Step 1: Add baseline comments for reused Phase 2 storage**

In `migrations/000_saas_platform_management_baseline.sql`, add comments after `platform.schema_apply_jobs` and `platform.runtime_operations` definitions:

```sql
COMMENT ON TABLE platform.schema_apply_jobs IS
    'Schema apply execution log. Phase 2 industry solution factory stores per-asset apply results in metadata.asset_results.';

COMMENT ON TABLE platform.runtime_operations IS
    'Platform runtime operation catalog. Phase 2 industry solution factory upserts manifest runtime_operation assets here.';

COMMENT ON TABLE platform.platform_masters IS
    'System administration master records. Phase 2 industry solution factory stores draft context rule, assistant skill, quality gate, and verification scenario metadata assets here.';
```

- [ ] **Step 2: Update baseline restructure document**

Add this section to `migrations/BASELINE_RESTRUCTURE.md` under the SaaS platform management baseline notes:

```markdown
### Phase 2 Industry Solution Factory Storage

The industry solution factory remains a platform management capability and belongs to `000_saas_platform_management_baseline.sql`.

Phase 2 intentionally reuses existing platform storage instead of creating a table per asset type:

- `platform.schema_change_requests.schema_package.metadata.industry_manifest` stores the desired manifest.
- `platform.schema_change_requests.schema_package.metadata.package_diff` stores the package-level asset diff computed at request creation.
- `platform.schema_apply_jobs.metadata.asset_results` stores per-asset apply status and retry diagnostics.
- `platform.runtime_operations` stores runtime operation assets.
- `tool_definitions` stores Tool Runtime definition and policy assets from the AI capability baseline.
- `platform.platform_masters` stores draft context rule, assistant skill, quality gate, and verification scenario metadata assets.

Context-rule assets generated from industry packages are stored as draft metadata and must not be activated automatically by schema apply.
```

- [ ] **Step 3: Run backend tests affected by migration docs**

Run:

```powershell
cd backend
go test ./internal/domain/systemadmin ./internal/domain/industry -count=1
```

Expected: PASS.

- [ ] **Step 4: Run fresh migration verification**

Start PostgreSQL if needed:

```powershell
docker compose up -d postgres
```

Create and apply a verification database:

```powershell
$env:PGPASSWORD='postgres'
psql -h localhost -U postgres -d postgres -c "CREATE DATABASE meta_org_phase2_verify;"
psql -h localhost -U postgres -d meta_org_phase2_verify -f migrations/000_saas_platform_management_baseline.sql
psql -h localhost -U postgres -d meta_org_phase2_verify -f migrations/001_erp_code_baseline.sql
psql -h localhost -U postgres -d meta_org_phase2_verify -f migrations/002_erp_platform_integration_baseline.sql
psql -h localhost -U postgres -d meta_org_phase2_verify -f migrations/004_ai_capability_baseline.sql
psql -h localhost -U postgres -d meta_org_phase2_verify -c "SELECT COUNT(*) AS not_valid_constraints FROM pg_constraint WHERE NOT convalidated;"
psql -h localhost -U postgres -d postgres -c "DROP DATABASE meta_org_phase2_verify WITH (FORCE);"
```

Expected: final count is `0`.

If `psql` is not on PATH, use:

```powershell
$env:PGPASSWORD='postgres'
& 'C:\Program Files\PostgreSQL\18\bin\psql.exe' -h localhost -U postgres -d postgres -c "CREATE DATABASE meta_org_phase2_verify;"
& 'C:\Program Files\PostgreSQL\18\bin\psql.exe' -h localhost -U postgres -d meta_org_phase2_verify -f migrations/000_saas_platform_management_baseline.sql
& 'C:\Program Files\PostgreSQL\18\bin\psql.exe' -h localhost -U postgres -d meta_org_phase2_verify -f migrations/001_erp_code_baseline.sql
& 'C:\Program Files\PostgreSQL\18\bin\psql.exe' -h localhost -U postgres -d meta_org_phase2_verify -f migrations/002_erp_platform_integration_baseline.sql
& 'C:\Program Files\PostgreSQL\18\bin\psql.exe' -h localhost -U postgres -d meta_org_phase2_verify -f migrations/004_ai_capability_baseline.sql
& 'C:\Program Files\PostgreSQL\18\bin\psql.exe' -h localhost -U postgres -d meta_org_phase2_verify -c "SELECT COUNT(*) AS not_valid_constraints FROM pg_constraint WHERE NOT convalidated;"
& 'C:\Program Files\PostgreSQL\18\bin\psql.exe' -h localhost -U postgres -d postgres -c "DROP DATABASE meta_org_phase2_verify WITH (FORCE);"
```

- [ ] **Step 5: Commit**

```powershell
git add migrations/000_saas_platform_management_baseline.sql migrations/BASELINE_RESTRUCTURE.md
git commit -m "Document industry solution factory storage"
```

---

## Task 8: Final Verification

**Files:**
- Review modified backend, frontend, and migration files.

- [ ] **Step 1: Run focused backend tests**

Run:

```powershell
cd backend
go test ./internal/domain/systemadmin -count=1
go test ./internal/domain/industry -count=1
```

Expected: PASS.

- [ ] **Step 2: Run full backend verification**

Run:

```powershell
cd backend
go test ./...
go build ./cmd/server
```

Expected: both exit 0.

- [ ] **Step 3: Run frontend verification**

Run:

```powershell
cd frontend
npm run lint
npm run build
node verify-system-admin-workspace.mjs
```

Expected: all exit 0.

- [ ] **Step 4: Run diff hygiene checks**

Run:

```powershell
cd ..
git diff --check
git status --short
```

Expected: no whitespace errors. Existing unrelated untracked files, including the YC markdown, `docs/investor.rar`, `docs/investor/`, and `tools/`, remain untouched unless the user explicitly asks to include them.

- [ ] **Step 5: Commit final cleanup if needed**

If formatting or verification fixes changed files after previous commits:

```powershell
git add backend/internal/domain/systemadmin backend/internal/domain/industry frontend/src/lib/api.ts frontend/src/app/system-admin-workspace.tsx frontend/src/lib/i18n.tsx frontend/verify-system-admin-workspace.mjs migrations/000_saas_platform_management_baseline.sql migrations/BASELINE_RESTRUCTURE.md
git commit -m "Finalize industry solution factory phase 2"
```

If there are no additional changes, do not create an empty commit.

---

## Plan Self-Review

Spec coverage:

- Manifest structure is covered by Task 1.
- Package diff V2 is covered by Task 2.
- Schema verify V2 gates are covered by Task 3.
- Apply orchestration and asset result persistence are covered by Task 4.
- Custom package publication gates are covered by Task 5.
- SystemAdmin UI display and bilingual text are covered by Task 6.
- Baseline governance and fresh migration verification are covered by Task 7.
- Backend, frontend, migration, and diff hygiene verification are covered by Task 8.

Storage consistency:

- Platform industry solution factory data stays in the SaaS platform baseline.
- Tenant ERP business data remains in the ERP baseline.
- AI capability tables are reused for Tool Runtime definitions, while context-rule activation remains outside schema apply.

Type consistency:

- `IndustrySolutionManifest`, `IndustrySolutionAsset`, `PackageAssetDiff`, `SchemaApplyAssetResult`, and `PublicationGateResult` are introduced before later tasks consume them.
- Backend JSON names match frontend TypeScript field names.
- Existing endpoint names are reused, with one query endpoint added for package diff V2.

Verification coverage:

- Every backend behavior change starts with a failing Go test.
- Frontend changes start with a failing static verification script.
- Final verification includes focused tests, full backend tests, backend build, frontend lint/build, static UI verification, migration verification, and diff hygiene.
