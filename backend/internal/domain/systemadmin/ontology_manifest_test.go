package systemadmin

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/erp"
)

func TestSolutionManifestsReferenceExecutableActionsAndTools(t *testing.T) {
	for _, build := range []func(ERPSolutionFlowRequest) IndustrySolutionManifest{BuildERPSolutionManifest, BuildRetailDistributionSolutionManifest, BuildERPNextManufacturingSolutionManifest} {
		manifest := build(ERPSolutionFlowRequest{})
		tools := map[string]bool{}
		for _, tool := range mapSliceFromAny(manifest.Metadata["tool_definitions"]) {
			key := stringValue(tool["tool_key"])
			if strings.HasPrefix(key, "erp.m") {
				t.Fatalf("unimplemented tool alias %s", key)
			}
			tools[key] = true
		}
		for _, skill := range mapSliceFromAny(manifest.Metadata["assistant_skills"]) {
			for _, tool := range stringSliceFromAny(skill["allowed_tools"]) {
				if !tools[tool] {
					t.Errorf("skill %s references unavailable tool %s", skill["skill_key"], tool)
				}
			}
		}
		registry := erp.DefaultActionRegistry()
		for _, kind := range []string{"process_loops", "verification_scenarios"} {
			for _, loop := range mapSliceFromAny(manifest.Metadata[kind]) {
				for _, step := range stringSliceFromAny(loop["steps"]) {
					table, action, isAction := strings.Cut(step, ".")
					if _, ok := registry.Lookup(table, action); isAction && !ok {
						t.Errorf("active workflow references retired action %s", step)
					}
				}
			}
		}
	}
}

type toolReferenceTx struct {
	pgx.Tx
	sql  string
	args []any
	rows string
}

func (tx *toolReferenceTx) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	tx.sql, tx.args = sql, args
	return pgconn.NewCommandTag(tx.rows), nil
}

func TestSolutionToolAssetsReferenceRegistryWithoutOverwritingPolicy(t *testing.T) {
	for _, kind := range []string{AssetTypeToolDefinition, AssetTypeToolPolicy} {
		tx := &toolReferenceTx{rows: "UPDATE 1"}
		repo := &Repository{}
		request := &IndustrySolutionChangeRequest{ID: uuid.New()}
		result := &IndustrySolutionApplyAssetResult{AssetType: kind, AssetKey: kind + ".ontology.action.execute", Metadata: map[string]any{
			"payload": map[string]any{"tool_key": "ontology.action.execute", "policy": "auto", "required_level": "L1"},
		}}
		if err := repo.applyIndustrySolutionAsset(context.Background(), tx, request, result); err != nil {
			t.Fatal(err)
		}
		if tx.args[0] != "ontology.action.execute" || !strings.Contains(tx.sql, "UPDATE tool_definitions") ||
			strings.Contains(tx.sql, "default_policy") || strings.Contains(tx.sql, "required_level") {
			t.Fatalf("tool registry contract changed: %s %#v", tx.sql, tx.args)
		}
		tx.rows = "UPDATE 0"
		if err := repo.applyIndustrySolutionAsset(context.Background(), tx, request, result); !errors.Is(err, ErrValidation) {
			t.Fatalf("missing tool was accepted: %v", err)
		}
	}
}
