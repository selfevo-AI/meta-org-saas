package monitoringagent

import (
	"encoding/json"
	"errors"
	"log"
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
	r.Get("/monitoring-agent/status", h.getStatus)
	r.Post("/monitoring-agent/runs", h.createRun)
	r.Get("/monitoring-agent/runs", h.listRuns)
	r.Get("/monitoring-agent/runs/{id}", h.getRun)
}

func (h *Handler) getStatus(w http.ResponseWriter, r *http.Request) {
	var orgID *uuid.UUID
	if raw := r.URL.Query().Get("organization_id"); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid organization_id"})
			return
		}
		orgID = &parsed
	}
	status, err := h.service.Status(r.Context(), orgID)
	writeResult(w, http.StatusOK, status, err)
}

func (h *Handler) createRun(w http.ResponseWriter, r *http.Request) {
	var body struct {
		OrganizationID string `json:"organization_id"`
		LookbackHours  int    `json:"lookback_hours"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	var orgID *uuid.UUID
	if body.OrganizationID != "" {
		parsed, err := uuid.Parse(body.OrganizationID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid organization_id"})
			return
		}
		orgID = &parsed
	}
	run, err := h.service.Run(r.Context(), RunInput{
		TriggerType:    TriggerManual,
		OrganizationID: orgID,
		LookbackHours:  body.LookbackHours,
	})
	writeResult(w, http.StatusCreated, run, err)
}

func (h *Handler) listRuns(w http.ResponseWriter, r *http.Request) {
	var orgID *uuid.UUID
	if raw := r.URL.Query().Get("organization_id"); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid organization_id"})
			return
		}
		orgID = &parsed
	}
	runs, err := h.service.ListRuns(r.Context(), ListRunsFilter{OrganizationID: orgID, Limit: queryLimit(r, 20)})
	writeResult(w, http.StatusOK, map[string]any{"runs": runs}, err)
}

func (h *Handler) getRun(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid run id"})
		return
	}
	run, err := h.service.GetRun(r.Context(), id)
	writeResult(w, http.StatusOK, run, err)
}

func writeResult(w http.ResponseWriter, successStatus int, value any, err error) {
	if err == nil {
		writeJSON(w, successStatus, value)
		return
	}
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, ErrValidation):
		status = http.StatusBadRequest
	case errors.Is(err, ErrNotFound):
		status = http.StatusNotFound
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("monitoringagent write json: %v", err)
	}
}

func queryLimit(r *http.Request, fallback int) int {
	limit := fallback
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	return limit
}
