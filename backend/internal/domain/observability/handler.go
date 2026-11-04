package observability

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
	r.Post("/traces", h.startTrace)
	r.Get("/traces", h.listTraces)
	r.Get("/traces/{id}", h.getTrace)
	r.Post("/traces/{id}/spans", h.recordSpan)
	r.Post("/traces/{id}/complete", h.completeTrace)
	r.Post("/metrics", h.recordMetric)
	r.Get("/metrics", h.queryMetrics)
}

func (h *Handler) startTrace(w http.ResponseWriter, r *http.Request) {
	var input CreateTraceInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	t, err := h.service.StartTrace(r.Context(), input.WorkflowID, input.Metadata)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (h *Handler) listTraces(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	traces, err := h.service.ListTraces(r.Context(), limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, traces)
}

func (h *Handler) getTrace(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid trace id"})
		return
	}
	t, err := h.service.GetTrace(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "trace not found"})
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h *Handler) recordSpan(w http.ResponseWriter, r *http.Request) {
	traceID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid trace id"})
		return
	}
	var input RecordSpanInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	input.TraceID = traceID
	span, err := h.service.RecordSpan(r.Context(), input)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, span)
}

func (h *Handler) completeTrace(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid trace id"})
		return
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if body.Status == "" {
		body.Status = "completed"
	}
	if err := h.service.CompleteTrace(r.Context(), id, body.Status); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "completed"})
}

func (h *Handler) recordMetric(w http.ResponseWriter, r *http.Request) {
	var input RecordMetricInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	m, err := h.service.RecordMetric(r.Context(), input)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

func (h *Handler) queryMetrics(w http.ResponseWriter, r *http.Request) {
	q := MetricsQuery{}
	if mt := r.URL.Query().Get("metric_type"); mt != "" {
		q.MetricType = MetricType(mt)
	}
	if mn := r.URL.Query().Get("metric_name"); mn != "" {
		q.MetricName = mn
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			q.Limit = parsed
		}
	}
	metrics, err := h.service.QueryMetrics(r.Context(), q)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, metrics)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("writeJSON error", "error", err)
	}
}
