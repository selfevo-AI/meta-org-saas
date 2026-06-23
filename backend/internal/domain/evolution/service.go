package evolution

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrValidation = errors.New("validation error")
	ErrForbidden  = errors.New("forbidden")
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ComputeWeight(ctx context.Context, input WeightInput) (*DecisionWeight, error) {
	w, err := s.repo.GetWeight(ctx, input.ActorID, input.ActorType)
	if err != nil {
		w = &DecisionWeight{
			ActorID:          input.ActorID,
			ActorType:        input.ActorType,
			OverallScore:     1.0,
			ExpertiseScore:   0.5,
			TrackRecordScore: 0.5,
			ReliabilityScore: 0.5,
			RecencyScore:     1.0,
			ContextFitScore:  0.5,
			PrincipleScore:   0.5,
			DecisionCount:    0,
		}
	}

	a, err := s.repo.GetAlpha(ctx)
	if err != nil {
		a = &AlphaConfig{
			Expertise:   0.25,
			TrackRecord: 0.20,
			Reliability: 0.15,
			Recency:     0.10,
			ContextFit:  0.10,
			Principle:   0.20,
		}
	}

	overall := a.Expertise*w.ExpertiseScore +
		a.TrackRecord*w.TrackRecordScore +
		a.Reliability*w.ReliabilityScore +
		a.Recency*w.RecencyScore +
		a.ContextFit*w.ContextFitScore +
		a.Principle*w.PrincipleScore

	w.OverallScore = math.Min(overall, 1.0)

	if err := s.repo.UpsertWeight(ctx, w); err != nil {
		return nil, err
	}
	return w, nil
}

func (s *Service) RecordOutcome(ctx context.Context, input OutcomeInput) (*DecisionWeight, error) {
	w, err := s.repo.GetWeight(ctx, input.ActorID, input.ActorType)
	if err != nil {
		return nil, ErrNotFound
	}

	n := float64(w.DecisionCount + 1)
	w.TrackRecordScore = ((w.TrackRecordScore * (n - 1)) + input.OutcomeScore) / n
	w.RecencyScore = 1.0
	w.ContextFitScore = (w.ContextFitScore + 0.5) / 2
	w.DecisionCount = int(n)

	a, err := s.repo.GetAlpha(ctx)
	if err != nil {
		a = &AlphaConfig{
			Expertise:   0.25,
			TrackRecord: 0.20,
			Reliability: 0.15,
			Recency:     0.10,
			ContextFit:  0.10,
			Principle:   0.20,
		}
	}

	overall := a.Expertise*w.ExpertiseScore +
		a.TrackRecord*w.TrackRecordScore +
		a.Reliability*w.ReliabilityScore +
		a.Recency*w.RecencyScore +
		a.ContextFit*w.ContextFitScore +
		a.Principle*w.PrincipleScore

	w.OverallScore = math.Min(overall, 1.0)

	if err := s.repo.UpsertWeight(ctx, w); err != nil {
		return nil, err
	}
	return w, nil
}

func (s *Service) GetWeight(ctx context.Context, actorID uuid.UUID, actorType string) (*DecisionWeight, error) {
	return s.repo.GetWeight(ctx, actorID, actorType)
}

func (s *Service) ListWeights(ctx context.Context, limit int) ([]DecisionWeight, error) {
	return s.repo.ListWeights(ctx, limit)
}

func (s *Service) ComputeContextWeight(ctx context.Context, input ContextWeightInput) (*ContextDecisionWeight, error) {
	if input.ActorID == uuid.Nil || input.ActorType == "" {
		return nil, ErrValidation
	}
	if err := applyTenantContextWeightScope(ctx, &input.Scope); err != nil {
		return nil, err
	}
	normalizeScope(&input.Scope)
	scopeHash := ScopeHash(input.Scope)
	w, err := s.repo.GetContextWeight(ctx, input.ActorID, input.ActorType, scopeHash)
	if err != nil {
		global, globalErr := s.repo.GetWeight(ctx, input.ActorID, input.ActorType)
		if globalErr != nil {
			global = defaultDecisionWeight(input.ActorID, input.ActorType)
		}
		w = &ContextDecisionWeight{
			DecisionWeight:     *global,
			ScopeHash:          scopeHash,
			OrganizationID:     input.Scope.OrganizationID,
			DepartmentID:       input.Scope.DepartmentID,
			WorkflowTemplateID: input.Scope.WorkflowTemplateID,
			WorkflowStage:      input.Scope.WorkflowStage,
			TaskType:           input.Scope.TaskType,
			CapabilityID:       input.Scope.CapabilityID,
			RiskLevel:          input.Scope.RiskLevel,
			Context:            input.Scope.Context,
		}
		w.ID = uuid.Nil
		w.ContextFitScore = contextFitScore(input.Scope)
	}

	w.OverallScore = computeOverall(w.DecisionWeight, alphaOrDefault(s, ctx))
	w.OverallScore = applyRiskAdjustment(w.OverallScore, w.RiskLevel)
	if err := s.repo.UpsertContextWeight(ctx, w); err != nil {
		return nil, err
	}
	return w, nil
}

func (s *Service) RecordContextOutcome(ctx context.Context, input ContextOutcomeInput) (*ContextDecisionWeight, error) {
	if input.ActorID == uuid.Nil || input.ActorType == "" {
		return nil, ErrValidation
	}
	if input.OutcomeScore < 0 {
		input.OutcomeScore = 0
	}
	if input.OutcomeScore > 1 {
		input.OutcomeScore = 1
	}
	if err := applyTenantContextWeightScope(ctx, &input.Scope); err != nil {
		return nil, err
	}
	normalizeScope(&input.Scope)
	scopeHash := ScopeHash(input.Scope)
	w, err := s.repo.GetContextWeight(ctx, input.ActorID, input.ActorType, scopeHash)
	if err != nil {
		w, err = s.ComputeContextWeight(ctx, ContextWeightInput{
			ActorID:   input.ActorID,
			ActorType: input.ActorType,
			Scope:     input.Scope,
		})
		if err != nil {
			return nil, err
		}
	}

	n := float64(w.DecisionCount + 1)
	w.TrackRecordScore = ((w.TrackRecordScore * (n - 1)) + input.OutcomeScore) / n
	w.ReliabilityScore = ((w.ReliabilityScore * (n - 1)) + input.OutcomeScore) / n
	w.RecencyScore = 1.0
	w.ContextFitScore = (w.ContextFitScore + contextFitScore(input.Scope)) / 2
	w.DecisionCount = int(n)
	w.Context = input.Scope.Context
	w.OverallScore = applyRiskAdjustment(computeOverall(w.DecisionWeight, alphaOrDefault(s, ctx)), w.RiskLevel)

	if err := s.repo.UpsertContextWeight(ctx, w); err != nil {
		return nil, err
	}

	if _, err := s.RecordOutcome(ctx, OutcomeInput{
		ActorID:      input.ActorID,
		ActorType:    input.ActorType,
		OutcomeScore: input.OutcomeScore,
		TaskContext:  input.Scope.Context,
	}); err != nil && errors.Is(err, ErrNotFound) {
		if _, computeErr := s.ComputeWeight(ctx, WeightInput{ActorID: input.ActorID, ActorType: input.ActorType}); computeErr == nil {
			_, _ = s.RecordOutcome(ctx, OutcomeInput{
				ActorID:      input.ActorID,
				ActorType:    input.ActorType,
				OutcomeScore: input.OutcomeScore,
				TaskContext:  input.Scope.Context,
			})
		}
	}
	return w, nil
}

func (s *Service) ListContextWeights(ctx context.Context, limit int) ([]ContextDecisionWeight, error) {
	return s.repo.ListContextWeights(ctx, limit)
}

func (s *Service) GetAlpha(ctx context.Context) (*AlphaConfig, error) {
	return s.repo.GetAlpha(ctx)
}

func (s *Service) UpdateAlpha(ctx context.Context, a *AlphaConfig) error {
	sum := a.Expertise + a.TrackRecord + a.Reliability + a.Recency + a.ContextFit + a.Principle
	if sum < 0.95 || sum > 1.05 {
		return ErrValidation
	}
	return s.repo.UpdateAlpha(ctx, a)
}

func (s *Service) CreateExperiment(ctx context.Context, input CreateExperimentInput) (*Experiment, error) {
	if input.Name == "" || input.Hypothesis == "" {
		return nil, ErrValidation
	}
	if input.SuccessCriteria == nil {
		input.SuccessCriteria = map[string]any{}
	}
	return s.repo.CreateExperiment(ctx, input)
}

func (s *Service) ListExperiments(ctx context.Context) ([]Experiment, error) {
	return s.repo.ListExperiments(ctx)
}

func (s *Service) UpdateExperimentStatus(ctx context.Context, id uuid.UUID, status string, conclusion string) error {
	return s.repo.UpdateExperimentStatus(ctx, id, status, conclusion)
}

func (s *Service) CreateKnowledge(ctx context.Context, input CreateKnowledgeInput) (*KnowledgeEntry, error) {
	if input.Title == "" || input.Content == "" {
		return nil, ErrValidation
	}
	if input.Source == "" {
		input.Source = "manual"
	}
	if input.Tags == nil {
		input.Tags = []string{}
	}
	return s.repo.CreateKnowledge(ctx, input)
}

func (s *Service) ListKnowledge(ctx context.Context, limit int) ([]KnowledgeEntry, error) {
	return s.repo.ListKnowledge(ctx, limit)
}

func (s *Service) CreateSignal(ctx context.Context, input CreateSignalInput) (*Signal, error) {
	if input.SignalType == "" {
		return nil, ErrValidation
	}
	if input.Data == nil {
		input.Data = map[string]any{}
	}
	return s.repo.CreateSignal(ctx, input)
}

func (s *Service) ListSignals(ctx context.Context, acknowledged *bool, limit int) ([]Signal, error) {
	return s.repo.ListSignals(ctx, acknowledged, limit)
}

func (s *Service) AcknowledgeSignal(ctx context.Context, id uuid.UUID) error {
	return s.repo.AcknowledgeSignal(ctx, id)
}

func ScopeHash(scope ContextWeightScope) string {
	normalizeScope(&scope)
	payload, _ := json.Marshal(scope)
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func normalizeScope(scope *ContextWeightScope) {
	if scope.RiskLevel == "" {
		scope.RiskLevel = "low"
	}
	if scope.Context == nil {
		scope.Context = map[string]any{}
	}
}

func defaultDecisionWeight(actorID uuid.UUID, actorType string) *DecisionWeight {
	return &DecisionWeight{
		ActorID:          actorID,
		ActorType:        actorType,
		OverallScore:     1.0,
		ExpertiseScore:   0.5,
		TrackRecordScore: 0.5,
		ReliabilityScore: 0.5,
		RecencyScore:     1.0,
		ContextFitScore:  0.5,
		PrincipleScore:   0.5,
		DecisionCount:    0,
	}
}

func alphaOrDefault(s *Service, ctx context.Context) *AlphaConfig {
	a, err := s.repo.GetAlpha(ctx)
	if err == nil {
		return a
	}
	return &AlphaConfig{
		Expertise:   0.25,
		TrackRecord: 0.20,
		Reliability: 0.15,
		Recency:     0.10,
		ContextFit:  0.10,
		Principle:   0.20,
	}
}

func computeOverall(w DecisionWeight, a *AlphaConfig) float64 {
	overall := a.Expertise*w.ExpertiseScore +
		a.TrackRecord*w.TrackRecordScore +
		a.Reliability*w.ReliabilityScore +
		a.Recency*w.RecencyScore +
		a.ContextFit*w.ContextFitScore +
		a.Principle*w.PrincipleScore
	return math.Min(math.Max(overall, 0), 1)
}

func contextFitScore(scope ContextWeightScope) float64 {
	score := 0.45
	if scope.DepartmentID != nil {
		score += 0.10
	}
	if scope.CapabilityID != nil {
		score += 0.15
	}
	if scope.WorkflowTemplateID != nil {
		score += 0.10
	}
	if scope.WorkflowStage != "" || scope.TaskType != "" {
		score += 0.10
	}
	if len(scope.Context) > 0 {
		score += 0.10
	}
	return math.Min(score, 1)
}

func applyRiskAdjustment(score float64, riskLevel string) float64 {
	switch riskLevel {
	case "critical":
		return math.Max(score-0.15, 0)
	case "high":
		return math.Max(score-0.08, 0)
	default:
		return score
	}
}

func applyTenantContextWeightScope(ctx context.Context, scope *ContextWeightScope) error {
	orgID := currentTenantOrganizationID(ctx)
	if orgID == nil {
		return nil
	}
	if scope.OrganizationID != nil && *scope.OrganizationID != *orgID {
		return ErrForbidden
	}
	id := *orgID
	scope.OrganizationID = &id
	return nil
}

func currentTenantOrganizationID(ctx context.Context) *uuid.UUID {
	tenant, ok := middleware.TenantFromContext(ctx)
	if !ok || tenant.OrganizationID == nil {
		return nil
	}
	id := *tenant.OrganizationID
	return &id
}
