package assistant

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestVerifiedContextEngineBuildsAttentionCoreFromWorkRecords(t *testing.T) {
	sessionID := uuid.New()
	resolver := &fakeContextResolver{
		result: WorkRecordContext{
			ModuleKey: "project",
			Records: []WorkRecord{
				{ID: uuid.New().String(), Type: "project", Title: "Launch", Status: "active"},
			},
		},
	}
	engine := NewVerifiedContextEngine(VerifiedContextEngineConfig{
		Resolver:  resolver,
		Evaluator: NewContextRuleEvaluator(ContextRuleEvaluatorConfig{AttentionCoreRatio: 0.5}),
	})

	pkg, err := engine.BuildContextPackage(context.Background(), ContextRequest{
		SessionID:   sessionID,
		ActorID:     uuid.New(),
		ActorType:   "internal_human",
		ModuleKey:   "project",
		TargetType:  "project",
		TokenBudget: 200,
	})
	if err != nil {
		t.Fatalf("BuildContextPackage returned error: %v", err)
	}
	if pkg.ID == uuid.Nil {
		t.Fatalf("context package id is nil")
	}
	if len(pkg.AttentionCore) == 0 {
		t.Fatalf("attention core is empty")
	}
	if pkg.AttentionCore[0].EntityKey != "project" {
		t.Fatalf("entity = %s, want project", pkg.AttentionCore[0].EntityKey)
	}
	if pkg.Provenance["source"] != "compatibility_resolver" {
		t.Fatalf("source = %v, want compatibility_resolver", pkg.Provenance["source"])
	}
}

func TestVerifiedContextEngineAppliesActiveContextRules(t *testing.T) {
	sessionID := uuid.New()
	dictionaryID := uuid.New()
	ruleID := uuid.New()
	resolver := &fakeContextResolver{
		result: WorkRecordContext{
			ModuleKey: "erp",
			Records: []WorkRecord{
				{
					ID:     "REQ-1",
					Type:   "requirement",
					Title:  "Approve launch requirement",
					Status: "approved",
					Data: map[string]any{
						"status":     "approved",
						"risk_level": "medium",
					},
				},
			},
		},
	}
	engine := NewVerifiedContextEngine(VerifiedContextEngineConfig{
		Resolver:  resolver,
		Evaluator: NewContextRuleEvaluator(ContextRuleEvaluatorConfig{AttentionCoreRatio: 0.8}),
		RuleSource: &fakeContextRuleSource{rules: []ContextRuleRecord{
			{
				ID:                  ruleID,
				DictionaryVersionID: dictionaryID,
				ModuleKey:           "erp",
				EntityKey:           "requirement",
				FieldKey:            "status",
				RuleType:            "attention",
				Rule:                map[string]any{"base_weight": float64(9), "attention_core": true},
				Status:              DictionaryStatusActive,
			},
		}},
	})

	pkg, err := engine.BuildContextPackage(context.Background(), ContextRequest{
		SessionID:   sessionID,
		ActorID:     uuid.New(),
		ActorType:   "internal_human",
		ModuleKey:   "erp",
		TargetType:  "requirement",
		TokenBudget: 400,
	})
	if err != nil {
		t.Fatalf("BuildContextPackage returned error: %v", err)
	}
	if pkg.DictionaryVersionID == nil || *pkg.DictionaryVersionID != dictionaryID {
		t.Fatalf("dictionary version = %v, want %s", pkg.DictionaryVersionID, dictionaryID)
	}
	if len(pkg.AttentionCore) != 1 {
		t.Fatalf("attention core len = %d, want 1: %#v", len(pkg.AttentionCore), pkg.AttentionCore)
	}
	item := pkg.AttentionCore[0]
	if item.EntityKey != "requirement" || item.FieldKey != "status" || item.Source != "context_dictionary" {
		t.Fatalf("item = %#v, want dictionary-backed requirement.status", item)
	}
	if item.Metadata["rule_id"] != ruleID.String() {
		t.Fatalf("rule metadata = %#v, want rule_id %s", item.Metadata, ruleID)
	}
	if pkg.Provenance["source"] != "context_dictionary" {
		t.Fatalf("provenance = %#v, want context_dictionary", pkg.Provenance)
	}
}

func TestVerifiedContextEnginePermissionRuleCreatesOmission(t *testing.T) {
	engine := NewVerifiedContextEngine(VerifiedContextEngineConfig{
		Resolver: &fakeContextResolver{result: WorkRecordContext{
			ModuleKey: "governance",
			Records:   []WorkRecord{{ID: "DEC-1", Type: "access_decision", Data: map[string]any{"decision": "deny"}}},
		}},
		RuleSource: &fakeContextRuleSource{rules: []ContextRuleRecord{
			{
				ID:        uuid.New(),
				ModuleKey: "governance",
				EntityKey: "access_decision",
				FieldKey:  "decision",
				RuleType:  "permission",
				Rule:      map[string]any{"allowed_actor_types": []any{"platform_admin"}},
				Status:    DictionaryStatusActive,
			},
		}},
	})

	pkg, err := engine.BuildContextPackage(context.Background(), ContextRequest{
		SessionID: uuid.New(),
		ActorID:   uuid.New(),
		ActorType: "internal_human",
		ModuleKey: "governance",
	})
	if err != nil {
		t.Fatalf("BuildContextPackage returned error: %v", err)
	}
	if len(pkg.Omissions) != 1 || pkg.Omissions[0].Reason != "permission_denied" {
		t.Fatalf("omissions = %#v, want permission_denied", pkg.Omissions)
	}
	if len(pkg.AttentionCore) != 0 {
		t.Fatalf("attention core = %#v, want denied field omitted", pkg.AttentionCore)
	}
}

func TestVerifiedContextEngineFinanceRuleCreatesRiskSignal(t *testing.T) {
	engine := NewVerifiedContextEngine(VerifiedContextEngineConfig{
		Resolver: &fakeContextResolver{result: WorkRecordContext{
			ModuleKey: "finance",
			Records:   []WorkRecord{{ID: "COST-1", Type: "cost_ledger_entry", Data: map[string]any{"amount": float64(120)}}},
		}},
		RuleSource: &fakeContextRuleSource{rules: []ContextRuleRecord{
			{
				ID:        uuid.New(),
				ModuleKey: "finance",
				EntityKey: "cost_ledger_entry",
				FieldKey:  "amount",
				RuleType:  "finance",
				Rule:      map[string]any{"requires_validation": true, "validation_status": "unverified", "unverified_as_signal": true},
				Status:    DictionaryStatusActive,
			},
		}},
	})

	pkg, err := engine.BuildContextPackage(context.Background(), ContextRequest{
		SessionID: uuid.New(),
		ActorID:   uuid.New(),
		ActorType: "internal_human",
		ModuleKey: "finance",
	})
	if err != nil {
		t.Fatalf("BuildContextPackage returned error: %v", err)
	}
	if len(pkg.RiskAndSignals) != 1 {
		t.Fatalf("risk signals = %#v, want one finance signal", pkg.RiskAndSignals)
	}
	if pkg.RiskAndSignals[0].ValidationState != ValidationFinanceConflict {
		t.Fatalf("validation = %q, want finance conflict", pkg.RiskAndSignals[0].ValidationState)
	}
}

func TestVerifiedContextEngineBlocksStrictModuleWithoutDictionaryRules(t *testing.T) {
	engine := NewVerifiedContextEngine(VerifiedContextEngineConfig{
		Resolver: &fakeContextResolver{result: WorkRecordContext{
			ModuleKey: "erp",
			Records: []WorkRecord{
				{ID: "REQ-1", Type: "requirement", Title: "Launch", Status: "approved"},
			},
		}},
		RuleSource: &fakeContextRuleSource{},
	})

	_, err := engine.BuildContextPackage(context.Background(), ContextRequest{
		SessionID:  uuid.New(),
		ActorID:    uuid.New(),
		ActorType:  "internal_human",
		ModuleKey:  "erp",
		TargetType: "requirement",
	})
	if err == nil {
		t.Fatalf("BuildContextPackage returned nil error, want strict dictionary coverage failure")
	}
	if !strings.Contains(err.Error(), "active context rule is required") {
		t.Fatalf("error = %v, want active context rule requirement", err)
	}
}

type fakeContextRuleSource struct {
	rules []ContextRuleRecord
	err   error
}

func (f *fakeContextRuleSource) ListActiveContextRules(_ context.Context, _ ContextRequest) ([]ContextRuleRecord, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.rules, nil
}
