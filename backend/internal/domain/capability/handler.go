package capability

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/evolution"
)

type Handler struct {
	repo      *Repository
	router    *Router
	evolution *evolution.Service
}

func NewHandler(repo *Repository, router *Router, evo ...*evolution.Service) *Handler {
	h := &Handler{repo: repo, router: router}
	if len(evo) > 0 {
		h.evolution = evo[0]
	}
	return h
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/capabilities", h.createCapability)
	r.Get("/capabilities", h.listCapabilities)
	r.Get("/capabilities/{id}", h.getCapability)
	r.Post("/capabilities/match", h.matchCapability)
	r.Post("/capabilities/evaluations", h.createEvaluation)
	r.Get("/capabilities/evaluations", h.listEvaluations)
	r.Get("/capabilities/{id}/evaluations", h.listCapabilityEvaluations)
	r.Post("/bindings", h.bindCapability)
	r.Delete("/bindings/{id}", h.unbindCapability)
	r.Get("/bindings", h.listBindings)
}

func (h *Handler) createCapability(w http.ResponseWriter, r *http.Request) {
	var input CreateCapabilityInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	cap, err := h.repo.CreateCapability(r.Context(), input)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, cap)
}

func (h *Handler) listCapabilities(w http.ResponseWriter, r *http.Request) {
	caps, err := h.repo.ListCapabilities(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, caps)
}

func (h *Handler) getCapability(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	cap, err := h.repo.GetCapabilityByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "capability not found"})
		return
	}
	writeJSON(w, http.StatusOK, cap)
}

func (h *Handler) matchCapability(w http.ResponseWriter, r *http.Request) {
	var req MatchRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	results, err := h.router.MatchTask(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func (h *Handler) createEvaluation(w http.ResponseWriter, r *http.Request) {
	var input CreateCapabilityEvaluationInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if input.CapabilityID == nil && input.ActorID == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "capability_id or actor_id is required"})
		return
	}
	if input.EvaluatorType == "" {
		input.EvaluatorType = "human"
	}
	if input.Evidence == nil {
		input.Evidence = map[string]any{}
	}
	normalizeEvaluationScores(&input)
	overall := evaluationOverall(input)
	eval, err := h.repo.CreateCapabilityEvaluation(r.Context(), input, overall)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if h.evolution != nil && eval.ActorID != nil && eval.ActorType != "" {
		riskLevel := "medium"
		if raw, ok := eval.Evidence["risk_level"].(string); ok && raw != "" {
			riskLevel = raw
		}
		_, _ = h.evolution.RecordContextOutcome(r.Context(), evolution.ContextOutcomeInput{
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
	writeJSON(w, http.StatusCreated, eval)
}

func (h *Handler) listEvaluations(w http.ResponseWriter, r *http.Request) {
	limit := evaluationLimit(r)
	evaluations, err := h.repo.ListCapabilityEvaluations(r.Context(), nil, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, evaluations)
}

func (h *Handler) listCapabilityEvaluations(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	limit := evaluationLimit(r)
	evaluations, err := h.repo.ListCapabilityEvaluations(r.Context(), &id, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, evaluations)
}

func (h *Handler) bindCapability(w http.ResponseWriter, r *http.Request) {
	var input BindCapabilityInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	binding, err := h.repo.BindCapability(r.Context(), input)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, binding)
}

func (h *Handler) unbindCapability(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	if err := h.repo.UnbindCapability(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "unbound"})
}

func (h *Handler) listBindings(w http.ResponseWriter, r *http.Request) {
	mvruStr := r.URL.Query().Get("mvru_id")
	if mvruStr == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "mvru_id query param required"})
		return
	}
	mvruID, err := uuid.Parse(mvruStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid mvru_id"})
		return
	}
	bindings, err := h.repo.ListBoundCapabilities(r.Context(), mvruID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, bindings)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON error: %v", err)
	}
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

func evaluationLimit(r *http.Request) int {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	return limit
}

func uuidString(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}
