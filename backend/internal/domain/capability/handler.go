package capability

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
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
	cap, err := h.service.CreateCapability(r.Context(), input)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, cap)
}

func (h *Handler) listCapabilities(w http.ResponseWriter, r *http.Request) {
	caps, err := h.service.ListCapabilities(r.Context())
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
	cap, err := h.service.GetCapabilityByID(r.Context(), id)
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
	results, err := h.service.MatchTask(r.Context(), req)
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
	eval, err := h.service.CreateEvaluation(r.Context(), input)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, eval)
}

func (h *Handler) listEvaluations(w http.ResponseWriter, r *http.Request) {
	limit := evaluationLimit(r)
	evaluations, err := h.service.ListEvaluations(r.Context(), nil, limit)
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
	evaluations, err := h.service.ListEvaluations(r.Context(), &id, limit)
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
	binding, err := h.service.BindCapability(r.Context(), input)
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
	if err := h.service.UnbindCapability(r.Context(), id); err != nil {
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
	bindings, err := h.service.ListBoundCapabilities(r.Context(), mvruID)
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
		slog.Error("writeJSON error", "error", err)
	}
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
