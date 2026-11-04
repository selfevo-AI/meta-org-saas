package capability

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

type fakeLister struct {
	caps []Capability
	err  error
}

func (f *fakeLister) ListCapabilities(context.Context) ([]Capability, error) {
	return f.caps, f.err
}

func capability(name, description, level string, active bool) Capability {
	return Capability{
		ID:              uuid.New(),
		Name:            name,
		Description:     description,
		PermissionLevel: level,
		IsActive:        active,
	}
}

func TestScoreCapabilityRewardsNameAndWordOverlap(t *testing.T) {
	cases := []struct {
		name string
		cap  Capability
		desc string
		want float64
	}{
		{"empty description", capability("Billing", "billing", "L1", true), "", 0},
		{"name match only", capability("Billing", "", "L1", true), "billing review", 0.5},
		{"word overlap only", capability("Forecast Engine", "quarterly demand for warehouses", "L1", true), "demand layout", 0.5},
		{"name plus overlap caps at one", capability("Billing", "billing", "L1", true), "billing billing billing", 1.0},
		{"short words ignored", capability("Anything", "and the for", "L1", true), "and the for", 0},
	}
	for _, tc := range cases {
		if got := scoreCapability(tc.cap, tc.desc); got != tc.want {
			t.Errorf("%s: scoreCapability = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestScoreReasonThresholds(t *testing.T) {
	cases := []struct {
		score float64
		want  string
	}{
		{1.0, "high match"},
		{0.8, "high match"},
		{0.79, "good match"},
		{0.5, "good match"},
		{0.49, "partial match"},
		{0.2, "partial match"},
		{0.19, "low match"},
		{0, "low match"},
	}
	for _, tc := range cases {
		if got := scoreReason(tc.score); got != tc.want {
			t.Errorf("scoreReason(%v) = %q, want %q", tc.score, got, tc.want)
		}
	}
}

func TestPermissionLevelWeight(t *testing.T) {
	cases := map[string]int{"L1": 1, "L2": 2, "L3": 3, "L4": 4, "": 0, "L5": 0, "l2": 0}
	for level, want := range cases {
		if got := permissionLevelWeight(level); got != want {
			t.Errorf("permissionLevelWeight(%q) = %d, want %d", level, got, want)
		}
	}
}

func TestMatchTaskFiltersRanksAndCapsResults(t *testing.T) {
	router := NewRouter(nil)
	router.repo = &fakeLister{caps: []Capability{
		capability("Demand Forecast", "demand forecasting for retail", "L2", true),
		capability("Weekly Review", "weekly demand checks", "L2", true),
		capability("Retired Planner", "demand planning legacy", "L2", false),
		capability("Junior Planner", "demand planning", "L1", true),
		capability("Unrelated", "nothing to see", "L2", true),
	}}

	ranked, err := router.MatchTask(context.Background(), MatchRequest{
		TaskDescription: "demand forecast weekly",
		RequiredLevel:   "L2",
	})
	if err != nil {
		t.Fatalf("MatchTask error = %v", err)
	}
	if len(ranked) != 2 {
		t.Fatalf("ranked = %d entries (%+v), want 2: inactive, under-leveled and zero-score caps excluded", len(ranked), ranked)
	}
	if ranked[0].Capability.Name != "Demand Forecast" || ranked[1].Capability.Name != "Weekly Review" {
		t.Fatalf("ranking order = [%s, %s], want [Demand Forecast, Weekly Review]", ranked[0].Capability.Name, ranked[1].Capability.Name)
	}
	if ranked[0].Score <= ranked[1].Score {
		t.Fatalf("scores not descending: %v then %v", ranked[0].Score, ranked[1].Score)
	}
	if ranked[0].Reason != "high match" || ranked[1].Reason != "good match" {
		t.Fatalf("reasons = [%q, %q], want [high match, good match]", ranked[0].Reason, ranked[1].Reason)
	}
}

func TestMatchTaskCapsResultsAtTen(t *testing.T) {
	var caps []Capability
	for i := 0; i < 12; i++ {
		caps = append(caps, capability("Common", "common handling", "L1", true))
	}
	router := NewRouter(nil)
	router.repo = &fakeLister{caps: caps}
	ranked, err := router.MatchTask(context.Background(), MatchRequest{TaskDescription: "common"})
	if err != nil {
		t.Fatalf("MatchTask error = %v", err)
	}
	if len(ranked) != 10 {
		t.Fatalf("ranked = %d entries, want the top 10", len(ranked))
	}
}

func TestEvaluationHelpersClampNormalizeAndWeigh(t *testing.T) {
	input := CreateCapabilityEvaluationInput{
		QualityScore:     1.4,
		ReliabilityScore: -0.5,
		CostScore:        0.5,
		LatencyScore:     2,
		RiskScore:        -1,
		ComplianceScore:  1,
	}
	normalizeEvaluationScores(&input)
	if input.QualityScore != 1 || input.ReliabilityScore != 0 || input.CostScore != 0.5 || input.LatencyScore != 1 || input.RiskScore != 0 || input.ComplianceScore != 1 {
		t.Fatalf("normalized scores = %+v", input)
	}
	if got := clampScore(0.42); got != 0.42 {
		t.Fatalf("clampScore(0.42) = %v", got)
	}

	perfect := CreateCapabilityEvaluationInput{QualityScore: 1, ReliabilityScore: 1, CostScore: 1, LatencyScore: 1, RiskScore: 1, ComplianceScore: 1}
	if got := evaluationOverall(perfect); got != 1 {
		t.Fatalf("overall(perfect) = %v, want 1", got)
	}
	if got := evaluationOverall(CreateCapabilityEvaluationInput{}); got != 0 {
		t.Fatalf("overall(zero) = %v, want 0", got)
	}
	if got := evaluationOverall(CreateCapabilityEvaluationInput{QualityScore: 1}); got != 0.25 {
		t.Fatalf("overall(quality only) = %v, want the 0.25 quality weight", got)
	}
}

func TestEvaluationLimitParsesQuery(t *testing.T) {
	cases := []struct {
		query string
		want  int
	}{
		{"", 50},
		{"?limit=5", 5},
		{"?limit=0", 50},
		{"?limit=-3", 50},
		{"?limit=abc", 50},
	}
	for i, tc := range cases {
		request := httptest.NewRequest("GET", fmt.Sprintf("/evaluations%s", tc.query), nil)
		if got := evaluationLimit(request); got != tc.want {
			t.Errorf("case %d (%q): evaluationLimit = %d, want %d", i, tc.query, got, tc.want)
		}
	}
}
