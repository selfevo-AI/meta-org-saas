package layer

import (
	"context"
	"strings"
	"testing"
)

func TestEstimateComplexityByDescriptionLength(t *testing.T) {
	cases := []struct {
		name string
		desc string
		want float64
	}{
		{"empty", "", 0.1},
		{"short", strings.Repeat("a", 50), 0.1},
		{"boundary 51", strings.Repeat("a", 51), 0.3},
		{"medium", strings.Repeat("a", 200), 0.3},
		{"boundary 201", strings.Repeat("a", 201), 0.6},
		{"long", strings.Repeat("a", 500), 0.6},
		{"boundary 501", strings.Repeat("a", 501), 0.9},
		{"very long", strings.Repeat("a", 5000), 0.9},
	}
	for _, tc := range cases {
		if got := estimateComplexity(tc.desc); got != tc.want {
			t.Errorf("%s: estimateComplexity = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestEstimateRiskAndStrategicCountKeywordsCaseInsensitively(t *testing.T) {
	riskCases := []struct {
		desc string
		want float64
	}{
		{"routine task", 0},
		{"security review", 0.2},
		{"SECURITY and LEGAL check", 0.4},
		{"critical urgent compliance regulatory financial security legal risk", 1.0},
		{"critical urgent compliance regulatory financial security legal", 1.0},
	}
	for _, tc := range riskCases {
		if got := estimateRisk(tc.desc); got != tc.want {
			t.Errorf("estimateRisk(%q) = %v, want %v", tc.desc, got, tc.want)
		}
	}

	if got := estimateStrategic("strategy vision transform initiative direction future growth innovation"); got != 1.0 {
		t.Errorf("estimateStrategic with all keywords = %v, want 1.0", got)
	}
	if got := estimateStrategic("Growth initiative"); got != 0.4 {
		t.Errorf("estimateStrategic case-insensitive = %v, want 0.4", got)
	}
	if got := estimateStrategic("bugfix"); got != 0 {
		t.Errorf("estimateStrategic without keywords = %v, want 0", got)
	}
}

func TestDetermineLayerThresholds(t *testing.T) {
	cases := []struct {
		name                        string
		complexity, risk, strategic float64
		wantLayer                   LayerType
		wantExplanationContains     string
	}{
		{"zero scores", 0, 0, 0, LayerOperational, "Operational layer"},
		{"strategic at threshold", 1, 1, 0, LayerStrategic, "Strategic layer"},
		{"high risk alone reaches tactical", 0, 1, 0, LayerTactical, "Tactical layer"},
		{"moderate blend", 0.5, 0.5, 0.5, LayerTactical, "Tactical layer"},
		{"just below strategic", 0.66, 0.66, 0.66, LayerTactical, "Tactical layer"},
		{"just below tactical", 0.3, 0.3, 0.3, LayerOperational, "Operational layer"},
	}
	for _, tc := range cases {
		gotLayer, explanation := determineLayer(tc.complexity, tc.risk, tc.strategic)
		if gotLayer != tc.wantLayer {
			t.Errorf("%s: determineLayer = %v, want %v", tc.name, gotLayer, tc.wantLayer)
		}
		if !strings.Contains(explanation, tc.wantExplanationContains) {
			t.Errorf("%s: explanation %q does not contain %q", tc.name, explanation, tc.wantExplanationContains)
		}
		if !strings.Contains(explanation, "score:") {
			t.Errorf("%s: explanation %q should surface the combined score", tc.name, explanation)
		}
	}
}

func TestClassifyUsesProvidedScoresOrEstimatesFromDescription(t *testing.T) {
	service := &ClassifierService{}

	estimated, err := service.Classify(context.Background(), ClassifyInput{
		TaskDescription: "critical security compliance review for the regulatory financial platform with a long tail of urgent legal risk notes",
	})
	if err != nil {
		t.Fatalf("Classify estimate error = %v", err)
	}
	if estimated.RiskScore != 1.0 {
		t.Fatalf("estimated risk = %v, want 1.0 from keyword density", estimated.RiskScore)
	}
	if estimated.Layer != LayerTactical {
		t.Fatalf("estimated layer = %v, want tactical (0.4 risk weight plus low complexity/strategic)", estimated.Layer)
	}

	provided, err := service.Classify(context.Background(), ClassifyInput{
		TaskDescription: "ignored because explicit scores win",
		ComplexityScore: 0.1,
		RiskScore:       0.1,
		StrategicScore:  0.1,
	})
	if err != nil {
		t.Fatalf("Classify provided error = %v", err)
	}
	if provided.Layer != LayerOperational {
		t.Fatalf("provided layer = %v, want operational", provided.Layer)
	}
	if provided.ComplexityScore != 0.1 || provided.RiskScore != 0.1 || provided.StrategicScore != 0.1 {
		t.Fatalf("provided scores echoed back as %+v", provided)
	}
}
