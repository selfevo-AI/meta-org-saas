package capability

import (
	"context"
	"errors"
	"math"
	"strings"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/evolution"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrValidation = errors.New("validation error")
)

// CapabilityLister is the subset of repository behavior the router needs; it
// keeps the matching rules unit-testable without a database.
type CapabilityLister interface {
	ListCapabilities(ctx context.Context) ([]Capability, error)
}

type Router struct {
	repo CapabilityLister
}

func NewRouter(repo *Repository) *Router {
	return &Router{repo: repo}
}

// Service owns the capability business rules and the evaluation write path;
// handlers only translate HTTP.
type Service struct {
	repo      *Repository
	router    *Router
	evolution *evolution.Service
}

func NewService(repo *Repository, router *Router, evolutionService *evolution.Service) *Service {
	return &Service{repo: repo, router: router, evolution: evolutionService}
}

func (s *Service) CreateCapability(ctx context.Context, input CreateCapabilityInput) (*Capability, error) {
	return s.repo.CreateCapability(ctx, input)
}

func (s *Service) ListCapabilities(ctx context.Context) ([]Capability, error) {
	return s.repo.ListCapabilities(ctx)
}

func (s *Service) GetCapabilityByID(ctx context.Context, id uuid.UUID) (*Capability, error) {
	return s.repo.GetCapabilityByID(ctx, id)
}

func (s *Service) MatchTask(ctx context.Context, req MatchRequest) ([]RankedCapability, error) {
	return s.router.MatchTask(ctx, req)
}

// CreateEvaluation normalizes the scores, stores the evaluation, and — when the
// evaluated actor is known — feeds the outcome back into the evolution
// context-weight model so capability quality influences future routing.
func (s *Service) CreateEvaluation(ctx context.Context, input CreateCapabilityEvaluationInput) (*CapabilityEvaluation, error) {
	normalizeEvaluationScores(&input)
	eval, err := s.repo.CreateCapabilityEvaluation(ctx, input, evaluationOverall(input))
	if err != nil {
		return nil, err
	}
	s.recordEvolutionOutcome(ctx, eval)
	return eval, nil
}

func (s *Service) recordEvolutionOutcome(ctx context.Context, eval *CapabilityEvaluation) {
	if s.evolution == nil || eval.ActorID == nil || eval.ActorType == "" {
		return
	}
	riskLevel := "medium"
	if raw, ok := eval.Evidence["risk_level"].(string); ok && raw != "" {
		riskLevel = raw
	}
	_, _ = s.evolution.RecordContextOutcome(ctx, evolution.ContextOutcomeInput{
		ActorID:      *eval.ActorID,
		ActorType:    eval.ActorType,
		OutcomeScore: eval.OverallScore,
		Scope: evolution.ContextWeightScope{
			CapabilityID: eval.CapabilityID,
			TaskType:     "capability_evaluation",
			RiskLevel:    riskLevel,
			Context: map[string]any{
				"evaluation_id":  eval.ID.String(),
				"workflow_id":    uuidString(eval.WorkflowID),
				"task_id":        uuidString(eval.TaskID),
				"evaluator_type": eval.EvaluatorType,
			},
		},
	})
}

func (s *Service) ListEvaluations(ctx context.Context, capabilityID *uuid.UUID, limit int) ([]CapabilityEvaluation, error) {
	return s.repo.ListCapabilityEvaluations(ctx, capabilityID, limit)
}

func (s *Service) BindCapability(ctx context.Context, input BindCapabilityInput) (*CapabilityBinding, error) {
	return s.repo.BindCapability(ctx, input)
}

func (s *Service) UnbindCapability(ctx context.Context, id uuid.UUID) error {
	return s.repo.UnbindCapability(ctx, id)
}

func (s *Service) ListBoundCapabilities(ctx context.Context, mvruID uuid.UUID) ([]CapabilityBinding, error) {
	return s.repo.ListBoundCapabilities(ctx, mvruID)
}

func normalizeEvaluationScores(input *CreateCapabilityEvaluationInput) {
	input.QualityScore = clampScore(input.QualityScore)
	input.ReliabilityScore = clampScore(input.ReliabilityScore)
	input.CostScore = clampScore(input.CostScore)
	input.LatencyScore = clampScore(input.LatencyScore)
	input.RiskScore = clampScore(input.RiskScore)
	input.ComplianceScore = clampScore(input.ComplianceScore)
}

func evaluationOverall(input CreateCapabilityEvaluationInput) float64 {
	return (input.QualityScore*0.25 +
		input.ReliabilityScore*0.20 +
		input.CostScore*0.10 +
		input.LatencyScore*0.10 +
		input.RiskScore*0.15 +
		input.ComplianceScore*0.20)
}

func clampScore(score float64) float64 {
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

func uuidString(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

type MatchRequest struct {
	TaskDescription string         `json:"task_description"`
	MVRUID          uuid.UUID      `json:"mvru_id,omitempty"`
	RequiredLevel   string         `json:"required_level,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

type RankedCapability struct {
	Capability Capability `json:"capability"`
	Score      float64    `json:"score"`
	Reason     string     `json:"reason"`
}

func (r *Router) MatchTask(ctx context.Context, req MatchRequest) ([]RankedCapability, error) {
	caps, err := r.repo.ListCapabilities(ctx)
	if err != nil {
		return nil, err
	}

	var ranked []RankedCapability
	desc := strings.ToLower(req.TaskDescription)

	for _, cap := range caps {
		if !cap.IsActive {
			continue
		}
		if req.RequiredLevel != "" && permissionLevelWeight(cap.PermissionLevel) < permissionLevelWeight(req.RequiredLevel) {
			continue
		}

		score := scoreCapability(cap, desc)
		if score > 0 {
			ranked = append(ranked, RankedCapability{
				Capability: cap,
				Score:      score,
				Reason:     scoreReason(score),
			})
		}
	}

	for i := 0; i < len(ranked); i++ {
		for j := i + 1; j < len(ranked); j++ {
			if ranked[j].Score > ranked[i].Score {
				ranked[i], ranked[j] = ranked[j], ranked[i]
			}
		}
	}

	if len(ranked) > 10 {
		ranked = ranked[:10]
	}
	return ranked, nil
}

func scoreCapability(cap Capability, desc string) float64 {
	nameScore := 0.0
	if strings.Contains(desc, strings.ToLower(cap.Name)) {
		nameScore = 0.5
	}

	descScore := 0.0
	capDesc := strings.ToLower(cap.Description)
	words := strings.Fields(desc)
	matchCount := 0
	for _, w := range words {
		if len(w) > 3 && strings.Contains(capDesc, w) {
			matchCount++
		}
	}
	if len(words) > 0 {
		descScore = float64(matchCount) / float64(len(words))
	}

	return math.Min(nameScore+descScore, 1.0)
}

func scoreReason(score float64) string {
	switch {
	case score >= 0.8:
		return "high match"
	case score >= 0.5:
		return "good match"
	case score >= 0.2:
		return "partial match"
	default:
		return "low match"
	}
}

func permissionLevelWeight(level string) int {
	switch level {
	case "L1":
		return 1
	case "L2":
		return 2
	case "L3":
		return 3
	case "L4":
		return 4
	default:
		return 0
	}
}
