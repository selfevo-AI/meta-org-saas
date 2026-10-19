package smtmes

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

// RegisterRoutes registers tenant-scoped SMT MES routes (inside TenantMiddleware).
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/smt/settings", h.settings)
	r.Patch("/smt/settings", h.updateSettings)
	r.Get("/smt/machines/{code}/context", h.machineContext)
	r.Get("/smt/board", h.board)
	r.Get("/smt/work-orders/{key}/progress", h.workOrderProgress)
	r.Get("/smt/trace/sn/{sn}", h.traceSN)
	r.Get("/smt/trace/reels/{uid}/units", h.reelUnits)
	r.Get("/smt/reports/production", h.productionReport)
	r.Get("/smt/reports/andon", h.andonReport)
	r.Get("/smt/reports/downtime", h.downtimeReport)
	r.Get("/smt/reports/materials", h.materialReport)
	r.Get("/smt/edges", h.listEdges)
	r.Post("/smt/edges", h.createEdge)
	r.Patch("/smt/edges/{key}", h.updateEdge)
	r.Delete("/smt/edges/{key}", h.deleteEdge)
	r.Get("/smt/edges/{key}/events", h.edgeEvents)
	r.Get("/smt/edges/{key}/sync-log", h.edgeSyncLog)
	r.Post("/smt/edges/{key}/pull", h.pullEdge)
	r.Post("/smt/edges/{key}/push", h.pushEdge)
	r.Post("/smt/edges/{key}/work-orders/{wo}/actions/{action}", h.pushWorkOrderAction)
}

// RegisterHookRoutes registers the public webhook receiver (no JWT; HMAC-authenticated).
func (h *Handler) RegisterHookRoutes(r chi.Router) {
	r.Post("/smt/hooks/{instance}", h.webhook)
}

func (h *Handler) settings(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.Settings(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) updateSettings(w http.ResponseWriter, r *http.Request) {
	var input UpdateSettingsInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.UpdateSettings(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) machineContext(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.MachineContext(r.Context(), chi.URLParam(r, "code"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) board(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.Board(r.Context(), r.URL.Query().Get("line_code"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) workOrderProgress(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.WorkOrderProgress(r.Context(), chi.URLParam(r, "key"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) traceSN(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.TraceSN(r.Context(), chi.URLParam(r, "sn"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) reelUnits(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ReelUnits(r.Context(), chi.URLParam(r, "uid"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) productionReport(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ProductionReport(r.Context(), r.URL.Query().Get("from"), r.URL.Query().Get("to"), r.URL.Query().Get("line_code"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) andonReport(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.AndonReport(r.Context(), r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) downtimeReport(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.DowntimeReport(r.Context(), r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) materialReport(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.MaterialReport(r.Context(), r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) listEdges(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListEdges(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"instances": result})
}

func (h *Handler) createEdge(w http.ResponseWriter, r *http.Request) {
	var input CreateEdgeInstanceInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateEdge(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) updateEdge(w http.ResponseWriter, r *http.Request) {
	key, ok := instanceKey(w, r)
	if !ok {
		return
	}
	var input UpdateEdgeInstanceInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.UpdateEdge(r.Context(), key, input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) deleteEdge(w http.ResponseWriter, r *http.Request) {
	key, ok := instanceKey(w, r)
	if !ok {
		return
	}
	if err := h.service.DeleteEdge(r.Context(), key); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) edgeEvents(w http.ResponseWriter, r *http.Request) {
	key, ok := instanceKey(w, r)
	if !ok {
		return
	}
	result, err := h.service.EdgeEvents(r.Context(), key, r.URL.Query().Get("status"), queryLimit(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": result})
}

func (h *Handler) edgeSyncLog(w http.ResponseWriter, r *http.Request) {
	key, ok := instanceKey(w, r)
	if !ok {
		return
	}
	result, err := h.service.EdgeSyncLog(r.Context(), key, queryLimit(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": result})
}

func (h *Handler) pullEdge(w http.ResponseWriter, r *http.Request) {
	key, ok := instanceKey(w, r)
	if !ok {
		return
	}
	organizationID, ok := organizationFromRequest(w, r)
	if !ok {
		return
	}
	result, err := h.service.Pull(r.Context(), organizationID, key, queryLimit(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) pushEdge(w http.ResponseWriter, r *http.Request) {
	key, ok := instanceKey(w, r)
	if !ok {
		return
	}
	organizationID, ok := organizationFromRequest(w, r)
	if !ok {
		return
	}
	var input PushInput
	if r.Body != nil && r.ContentLength != 0 {
		if !decodeJSON(w, r, &input) {
			return
		}
	}
	result, err := h.service.Push(r.Context(), organizationID, key, input.Scopes)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) pushWorkOrderAction(w http.ResponseWriter, r *http.Request) {
	key, ok := instanceKey(w, r)
	if !ok {
		return
	}
	organizationID, ok := organizationFromRequest(w, r)
	if !ok {
		return
	}
	if err := h.service.PushWorkOrderAction(r.Context(), organizationID, key, chi.URLParam(r, "wo"), chi.URLParam(r, "action")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (h *Handler) webhook(w http.ResponseWriter, r *http.Request) {
	key, err := uuid.Parse(chi.URLParam(r, "instance"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "instance not found"})
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cannot read request body"})
		return
	}
	signature := r.Header.Get("X-SMT-Signature")
	status, err := h.service.IngestWebhook(r.Context(), key, body, signature)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": status})
}

func instanceKey(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	key, err := uuid.Parse(chi.URLParam(r, "key"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid instance key"})
		return uuid.Nil, false
	}
	return key, true
}

func organizationFromRequest(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	tenant, ok := tenantFrom(r.Context())
	if !ok {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "organization context is required"})
		return uuid.Nil, false
	}
	return *tenant.OrganizationID, true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(target); err != nil && !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return false
	}
	return true
}

func queryLimit(r *http.Request) int {
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit <= 0 {
		return 100
	}
	if limit > 500 {
		return 500
	}
	return limit
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrValidation):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	case errors.Is(err, ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
	case errors.Is(err, ErrForbidden):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
