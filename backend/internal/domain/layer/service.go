package layer

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/google/uuid"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrValidation = errors.New("validation error")
)

type ClassifierService struct {
	repo *Repository
}

func NewClassifierService(repo *Repository) *ClassifierService {
	return &ClassifierService{repo: repo}
}

type ClassifyInput struct {
	TaskDescription string         `json:"task_description"`
	MVRUID          uuid.UUID      `json:"mvru_id,omitempty"`
	ComplexityScore float64        `json:"complexity_score"`
	RiskScore       float64        `json:"risk_score"`
	StrategicScore  float64        `json:"strategic_score"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

type ClassifyOutput struct {
	Layer           LayerType `json:"layer"`
	ComplexityScore float64   `json:"complexity_score"`
	RiskScore       float64   `json:"risk_score"`
	StrategicScore  float64   `json:"strategic_score"`
	Explanation     string    `json:"explanation"`
}

func (s *ClassifierService) Classify(ctx context.Context, input ClassifyInput) (*ClassifyOutput, error) {
	if input.ComplexityScore == 0 && input.RiskScore == 0 && input.StrategicScore == 0 {
		input.ComplexityScore = estimateComplexity(input.TaskDescription)
		input.RiskScore = estimateRisk(input.TaskDescription)
		input.StrategicScore = estimateStrategic(input.TaskDescription)
	}

	layer, explanation := determineLayer(input.ComplexityScore, input.RiskScore, input.StrategicScore)

	return &ClassifyOutput{
		Layer:           layer,
		ComplexityScore: input.ComplexityScore,
		RiskScore:       input.RiskScore,
		StrategicScore:  input.StrategicScore,
		Explanation:     explanation,
	}, nil
}

func (s *ClassifierService) SetLayerConfig(ctx context.Context, mvruID uuid.UUID, layer LayerType, config map[string]any) error {
	if config == nil {
		config = map[string]any{}
	}
	return s.repo.SetLayerConfig(ctx, mvruID, layer, config)
}

func (s *ClassifierService) GetLayerConfig(ctx context.Context, mvruID uuid.UUID, layer LayerType) (*LayerConfig, error) {
	return s.repo.GetLayerConfig(ctx, mvruID, layer)
}

func (s *ClassifierService) ListRoutingRules(ctx context.Context) ([]LayerRoutingRule, error) {
	return s.repo.ListRoutingRules(ctx)
}

func estimateComplexity(desc string) float64 {
	length := len(desc)
	switch {
	case length > 500:
		return 0.9
	case length > 200:
		return 0.6
	case length > 50:
		return 0.3
	default:
		return 0.1
	}
}

func estimateRisk(desc string) float64 {
	riskWords := []string{"critical", "urgent", "compliance", "regulatory", "financial", "security", "legal", "risk"}
	count := 0
	for _, word := range riskWords {
		if contains(desc, word) {
			count++
		}
	}
	return math.Min(float64(count)*0.2, 1.0)
}

func estimateStrategic(desc string) float64 {
	stratWords := []string{"strategy", "vision", "transform", "initiative", "direction", "future", "growth", "innovation"}
	count := 0
	for _, word := range stratWords {
		if contains(desc, word) {
			count++
		}
	}
	return math.Min(float64(count)*0.2, 1.0)
}

func determineLayer(complexity, risk, strategic float64) (LayerType, string) {
	combined := complexity*0.3 + risk*0.4 + strategic*0.3

	switch {
	case combined >= 0.7:
		return LayerStrategic, fmt.Sprintf("Strategic layer (score: %.2f): high complexity/risk/strategic impact", combined)
	case combined >= 0.4:
		return LayerTactical, fmt.Sprintf("Tactical layer (score: %.2f): moderate complexity requiring planning", combined)
	default:
		return LayerOperational, fmt.Sprintf("Operational layer (score: %.2f): routine execution task", combined)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			sc := s[i+j]
			tc := substr[j]
			if sc >= 'A' && sc <= 'Z' {
				sc += 32
			}
			if tc >= 'A' && tc <= 'Z' {
				tc += 32
			}
			if sc != tc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
