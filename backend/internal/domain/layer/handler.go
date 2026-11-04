package layer

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	classifier *ClassifierService
}

func NewHandler(classifier *ClassifierService) *Handler {
	return &Handler{classifier: classifier}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/layers/classify", h.classify)
	r.Get("/layers/config/{mvruId}", h.getLayerConfig)
	r.Put("/layers/config/{mvruId}", h.setLayerConfig)
	r.Get("/layers/rules", h.listRules)
}

func (h *Handler) classify(w http.ResponseWriter, r *http.Request) {
	var input ClassifyInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	output, err := h.classifier.Classify(r.Context(), input)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, output)
}

func (h *Handler) getLayerConfig(w http.ResponseWriter, r *http.Request) {
	mvruID, err := uuid.Parse(chi.URLParam(r, "mvruId"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid mvru id"})
		return
	}
	layerStr := r.URL.Query().Get("layer")
	if layerStr == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "layer query param required"})
		return
	}
	layer := LayerType(layerStr)
	if layer != LayerStrategic && layer != LayerTactical && layer != LayerOperational {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid layer"})
		return
	}
	config, err := h.classifier.GetLayerConfig(r.Context(), mvruID, layer)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "config not found"})
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (h *Handler) setLayerConfig(w http.ResponseWriter, r *http.Request) {
	mvruID, err := uuid.Parse(chi.URLParam(r, "mvruId"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid mvru id"})
		return
	}
	var req struct {
		Layer  string         `json:"layer"`
		Config map[string]any `json:"config"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	layer := LayerType(req.Layer)
	if layer != LayerStrategic && layer != LayerTactical && layer != LayerOperational {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid layer"})
		return
	}
	if err := h.classifier.SetLayerConfig(r.Context(), mvruID, layer, req.Config); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "config saved"})
}

func (h *Handler) listRules(w http.ResponseWriter, r *http.Request) {
	rules, err := h.classifier.ListRoutingRules(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, rules)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("writeJSON error", "error", err)
	}
}
