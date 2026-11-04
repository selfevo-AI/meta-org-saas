package evolution

import (
	"encoding/json"
	"errors"
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
	r.Post("/evolution/weights/compute", h.computeWeight)
	r.Post("/evolution/weights/outcome", h.recordOutcome)
	r.Post("/evolution/context-weights/compute", h.computeContextWeight)
	r.Post("/evolution/context-weights/outcome", h.recordContextOutcome)
	r.Get("/evolution/context-weights", h.listContextWeights)
	r.Get("/evolution/weights/{actorType}/{actorID}", h.getWeight)
	r.Get("/evolution/weights", h.listWeights)
	r.Get("/evolution/alphas", h.getAlpha)
	r.Put("/evolution/alphas", h.updateAlpha)
	r.Post("/evolution/experiments", h.createExperiment)
	r.Get("/evolution/experiments", h.listExperiments)
	r.Patch("/evolution/experiments/{id}", h.updateExperimentStatus)
	r.Post("/evolution/knowledge", h.createKnowledge)
	r.Get("/evolution/knowledge", h.listKnowledge)
	r.Post("/evolution/signals", h.createSignal)
	r.Get("/evolution/signals", h.listSignals)
	r.Post("/evolution/signals/{id}/acknowledge", h.acknowledgeSignal)
}

func (h *Handler) computeWeight(w http.ResponseWriter, r *http.Request) {
	var input WeightInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	wResult, err := h.service.ComputeWeight(r.Context(), input)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, wResult)
}

func (h *Handler) recordOutcome(w http.ResponseWriter, r *http.Request) {
	var input OutcomeInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	wResult, err := h.service.RecordOutcome(r.Context(), input)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, wResult)
}

func (h *Handler) computeContextWeight(w http.ResponseWriter, r *http.Request) {
	var input ContextWeightInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	wResult, err := h.service.ComputeContextWeight(r.Context(), input)
	if err != nil {
		if errors.Is(err, ErrForbidden) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, wResult)
}

func (h *Handler) recordContextOutcome(w http.ResponseWriter, r *http.Request) {
	var input ContextOutcomeInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	wResult, err := h.service.RecordContextOutcome(r.Context(), input)
	if err != nil {
		if errors.Is(err, ErrForbidden) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, wResult)
}

func (h *Handler) listContextWeights(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	weights, err := h.service.ListContextWeights(r.Context(), limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, weights)
}

func (h *Handler) getWeight(w http.ResponseWriter, r *http.Request) {
	actorType := chi.URLParam(r, "actorType")
	actorID, err := uuid.Parse(chi.URLParam(r, "actorID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid actor id"})
		return
	}
	wResult, err := h.service.GetWeight(r.Context(), actorID, actorType)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "weight not found"})
		return
	}
	writeJSON(w, http.StatusOK, wResult)
}

func (h *Handler) listWeights(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	weights, err := h.service.ListWeights(r.Context(), limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, weights)
}

func (h *Handler) getAlpha(w http.ResponseWriter, r *http.Request) {
	a, err := h.service.GetAlpha(r.Context())
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "alpha config not found"})
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func (h *Handler) updateAlpha(w http.ResponseWriter, r *http.Request) {
	var input AlphaConfig
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if err := h.service.UpdateAlpha(r.Context(), &input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, input)
}

func (h *Handler) createExperiment(w http.ResponseWriter, r *http.Request) {
	var input CreateExperimentInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	e, err := h.service.CreateExperiment(r.Context(), input)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, e)
}

func (h *Handler) listExperiments(w http.ResponseWriter, r *http.Request) {
	experiments, err := h.service.ListExperiments(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, experiments)
}

func (h *Handler) updateExperimentStatus(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid experiment id"})
		return
	}
	var body struct {
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if err := h.service.UpdateExperimentStatus(r.Context(), id, body.Status, body.Conclusion); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": body.Status})
}

func (h *Handler) createKnowledge(w http.ResponseWriter, r *http.Request) {
	var input CreateKnowledgeInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	e, err := h.service.CreateKnowledge(r.Context(), input)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, e)
}

func (h *Handler) listKnowledge(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	entries, err := h.service.ListKnowledge(r.Context(), limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

func (h *Handler) createSignal(w http.ResponseWriter, r *http.Request) {
	var input CreateSignalInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	s, err := h.service.CreateSignal(r.Context(), input)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, s)
}

func (h *Handler) listSignals(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	var acknowledged *bool
	if a := r.URL.Query().Get("acknowledged"); a != "" {
		parsed, err := strconv.ParseBool(a)
		if err == nil {
			acknowledged = &parsed
		}
	}
	signals, err := h.service.ListSignals(r.Context(), acknowledged, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, signals)
}

func (h *Handler) acknowledgeSignal(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid signal id"})
		return
	}
	if err := h.service.AcknowledgeSignal(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "acknowledged"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("writeJSON error", "error", err)
	}
}
