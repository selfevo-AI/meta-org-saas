package evolution

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
)

func ptr[T any](value T) *T { return &value }

func TestContextFitScoreAccumulatesEvidence(t *testing.T) {
	cases := []struct {
		name  string
		scope ContextWeightScope
		want  float64
	}{
		{"bare scope", ContextWeightScope{}, 0.45},
		{"department only", ContextWeightScope{DepartmentID: ptr(uuid.New())}, 0.55},
		{"capability only", ContextWeightScope{CapabilityID: ptr(uuid.New())}, 0.60},
		{"stage or task type", ContextWeightScope{TaskType: "review"}, 0.55},
		{"context payload", ContextWeightScope{Context: map[string]any{"k": "v"}}, 0.55},
		{"everything", ContextWeightScope{
			DepartmentID:       ptr(uuid.New()),
			CapabilityID:       ptr(uuid.New()),
			WorkflowTemplateID: ptr(uuid.New()),
			WorkflowStage:      "plan",
			Context:            map[string]any{"k": "v"},
		}, 1.0},
	}
	for _, tc := range cases {
		if got := contextFitScore(tc.scope); math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("%s: contextFitScore = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestApplyRiskAdjustmentPenalizesHighRisk(t *testing.T) {
	cases := []struct {
		score, want float64
		risk        string
	}{
		{0.9, 0.75, "critical"},
		{0.1, 0, "critical"},
		{0.9, 0.82, "high"},
		{0.05, 0, "high"},
		{0.9, 0.9, "low"},
		{0.9, 0.9, ""},
		{0.9, 0.9, "medium"},
	}
	for _, tc := range cases {
		if got := applyRiskAdjustment(tc.score, tc.risk); math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("applyRiskAdjustment(%v, %q) = %v, want %v", tc.score, tc.risk, got, tc.want)
		}
	}
}

func TestComputeOverallUsesAlphaWeightsAndClamps(t *testing.T) {
	defaultAlpha := &AlphaConfig{Expertise: 0.25, TrackRecord: 0.20, Reliability: 0.15, Recency: 0.10, ContextFit: 0.10, Principle: 0.20}

	perfect := DecisionWeight{ExpertiseScore: 1, TrackRecordScore: 1, ReliabilityScore: 1, RecencyScore: 1, ContextFitScore: 1, PrincipleScore: 1}
	if got := computeOverall(perfect, defaultAlpha); math.Abs(got-1) > 1e-9 {
		t.Fatalf("computeOverall(perfect) = %v, want 1", got)
	}

	if got := computeOverall(DecisionWeight{}, defaultAlpha); got != 0 {
		t.Fatalf("computeOverall(zero) = %v, want 0", got)
	}

	expertiseOnly := DecisionWeight{ExpertiseScore: 1}
	if got := computeOverall(expertiseOnly, defaultAlpha); math.Abs(got-0.25) > 1e-9 {
		t.Fatalf("computeOverall(expertise only) = %v, want the 0.25 weight", got)
	}

	heavy := &AlphaConfig{Expertise: 2, TrackRecord: 2, Reliability: 2, Recency: 2, ContextFit: 2, Principle: 2}
	if got := computeOverall(perfect, heavy); got != 1 {
		t.Fatalf("computeOverall(overweighted) = %v, want clamped to 1", got)
	}
}

func TestNormalizeScopeFillsDefaults(t *testing.T) {
	scope := ContextWeightScope{}
	normalizeScope(&scope)
	if scope.RiskLevel != "low" {
		t.Fatalf("RiskLevel = %q, want low", scope.RiskLevel)
	}
	if scope.Context == nil {
		t.Fatal("Context = nil, want an initialized map")
	}

	custom := ContextWeightScope{RiskLevel: "critical", Context: map[string]any{"k": 1}}
	normalizeScope(&custom)
	if custom.RiskLevel != "critical" || len(custom.Context) != 1 {
		t.Fatalf("normalizeScope overwrote explicit values: %+v", custom)
	}
}

func TestScopeHashIsStableAndScopeSensitive(t *testing.T) {
	base := ContextWeightScope{DepartmentID: ptr(uuid.New()), RiskLevel: "high", Context: map[string]any{"a": "b"}}
	first := ScopeHash(base)
	if first != ScopeHash(base) {
		t.Fatal("ScopeHash is not deterministic for equal scopes")
	}

	// Normalized defaults must not change the hash: an empty risk level and
	// nil context normalize to the same values inside ScopeHash.
	withDefaults := ContextWeightScope{DepartmentID: base.DepartmentID}
	if ScopeHash(withDefaults) != ScopeHash(ContextWeightScope{DepartmentID: base.DepartmentID, RiskLevel: "low", Context: map[string]any{}}) {
		t.Fatal("ScopeHash should normalize risk level and context before hashing")
	}
	if first == ScopeHash(ContextWeightScope{DepartmentID: ptr(uuid.New()), RiskLevel: "high", Context: map[string]any{"a": "b"}}) {
		t.Fatal("ScopeHash should differ for different departments")
	}
	if first == ScopeHash(ContextWeightScope{DepartmentID: base.DepartmentID, RiskLevel: "low", Context: map[string]any{"a": "b"}}) {
		t.Fatal("ScopeHash should differ for different risk levels")
	}
}

func TestApplyTenantContextWeightScopeEnforcesTenantBoundary(t *testing.T) {
	orgID := uuid.New()
	tenantCtx := context.WithValue(context.Background(), middleware.TenantContextKey, &middleware.TenantContext{OrganizationID: &orgID})

	// Without tenant context the scope passes through untouched.
	scope := ContextWeightScope{}
	if err := applyTenantContextWeightScope(context.Background(), &scope); err != nil || scope.OrganizationID != nil {
		t.Fatalf("no-tenant scope = (%v, %v), want untouched", scope.OrganizationID, err)
	}

	// A missing organization is filled with the tenant's.
	if err := applyTenantContextWeightScope(tenantCtx, &scope); err != nil || scope.OrganizationID == nil || *scope.OrganizationID != orgID {
		t.Fatalf("scope organization = %v (err %v), want tenant %v", scope.OrganizationID, err, orgID)
	}

	// A matching organization passes.
	matching := ContextWeightScope{OrganizationID: &orgID}
	if err := applyTenantContextWeightScope(tenantCtx, &matching); err != nil {
		t.Fatalf("matching scope rejected: %v", err)
	}

	// A different organization is forbidden.
	other := uuid.New()
	mismatch := ContextWeightScope{OrganizationID: &other}
	if err := applyTenantContextWeightScope(tenantCtx, &mismatch); !errors.Is(err, ErrForbidden) {
		t.Fatalf("cross-tenant scope error = %v, want ErrForbidden", err)
	}
}
