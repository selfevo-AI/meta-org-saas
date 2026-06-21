package authorization

import "testing"

func TestEvaluateRulesUsesDenyBeforeAllow(t *testing.T) {
	result := EvaluateRules(Request{
		Action:       "read",
		ResourceType: "project",
		ScopeType:    "project",
		ScopeID:      "project-1",
	}, []Rule{
		{Action: "read", ResourceType: "project", ScopeType: "project", ScopeID: "project-1", Behavior: "allow", Priority: 100},
		{Action: "read", ResourceType: "project", ScopeType: "project", ScopeID: "project-1", Behavior: "deny", Priority: 10},
	})

	if result.Allowed {
		t.Fatalf("EvaluateRules allowed = true, want false")
	}
	if result.Behavior != "deny" {
		t.Fatalf("EvaluateRules behavior = %q, want deny", result.Behavior)
	}
}

func TestEvaluateRulesPrefersSpecificScope(t *testing.T) {
	result := EvaluateRules(Request{
		Action:       "write",
		ResourceType: "form",
		ScopeType:    "form",
		ScopeID:      "expense-form",
	}, []Rule{
		{Action: "write", ResourceType: "form", ScopeType: "organization", ScopeID: "org-1", Behavior: "deny", Priority: 100},
		{Action: "write", ResourceType: "form", ScopeType: "form", ScopeID: "expense-form", Behavior: "allow", Priority: 1},
	})

	if !result.Allowed {
		t.Fatalf("EvaluateRules allowed = false, want true")
	}
	if result.MatchedRule == nil || result.MatchedRule.ScopeType != "form" {
		t.Fatalf("EvaluateRules matched = %#v, want form scoped rule", result.MatchedRule)
	}
}
