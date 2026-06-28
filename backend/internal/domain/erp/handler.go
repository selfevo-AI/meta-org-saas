package erp

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/erp/catalog", h.catalog)
	r.Get("/erp/actions", h.listActions)
	r.Get("/erp/{tableCode}", h.listRecords)
	r.Post("/erp/{tableCode}", h.createRecord)
	r.Post("/erp/{tableCode}/{key}/actions/{action}", h.runAction)
	r.Get("/erp/{tableCode}/{key}/action-executions", h.listActionExecutions)
	r.Get("/erp/{tableCode}/{key}", h.getRecord)
	r.Patch("/erp/{tableCode}/{key}", h.updateRecord)
	r.Delete("/erp/{tableCode}/{key}", h.deleteRecord)
	r.Get("/erp/{tableCode}/{key}/{childCode}", h.listChildRecords)
	r.Post("/erp/{tableCode}/{key}/{childCode}", h.createChildRecord)
	r.Patch("/erp/{tableCode}/{key}/{childCode}/{lineKey}", h.updateChildRecord)
	r.Delete("/erp/{tableCode}/{key}/{childCode}/{lineKey}", h.deleteChildRecord)
}

func (h *Handler) catalog(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.service.Catalog(r.Context()))
}

func (h *Handler) listActions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"actions": h.service.Actions(r.Context())})
}

func (h *Handler) listRecords(w http.ResponseWriter, r *http.Request) {
	records, err := h.service.ListRecords(r.Context(), chi.URLParam(r, "tableCode"), queryLimit(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"records": records})
}

func (h *Handler) createRecord(w http.ResponseWriter, r *http.Request) {
	input, ok := decodeRecordInput(w, r)
	if !ok {
		return
	}
	record, err := h.service.CreateRecord(r.Context(), chi.URLParam(r, "tableCode"), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, record)
}

func (h *Handler) runAction(w http.ResponseWriter, r *http.Request) {
	input, ok := decodeActionInput(w, r)
	if !ok {
		return
	}
	result, err := h.service.RunAction(
		r.Context(),
		chi.URLParam(r, "tableCode"),
		chi.URLParam(r, "key"),
		chi.URLParam(r, "action"),
		input,
	)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) listActionExecutions(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ListActionExecutions(
		r.Context(),
		chi.URLParam(r, "tableCode"),
		chi.URLParam(r, "key"),
		queryLimit(r),
	)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"action_executions": items})
}

func (h *Handler) getRecord(w http.ResponseWriter, r *http.Request) {
	record, err := h.service.GetRecord(r.Context(), chi.URLParam(r, "tableCode"), chi.URLParam(r, "key"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (h *Handler) updateRecord(w http.ResponseWriter, r *http.Request) {
	input, ok := decodeRecordInput(w, r)
	if !ok {
		return
	}
	record, err := h.service.UpdateRecord(r.Context(), chi.URLParam(r, "tableCode"), chi.URLParam(r, "key"), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (h *Handler) deleteRecord(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DeleteRecord(r.Context(), chi.URLParam(r, "tableCode"), chi.URLParam(r, "key")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listChildRecords(w http.ResponseWriter, r *http.Request) {
	records, err := h.service.ListChildRecords(
		r.Context(),
		chi.URLParam(r, "tableCode"),
		chi.URLParam(r, "key"),
		chi.URLParam(r, "childCode"),
		queryLimit(r),
	)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"records": records})
}

func (h *Handler) createChildRecord(w http.ResponseWriter, r *http.Request) {
	input, ok := decodeRecordInput(w, r)
	if !ok {
		return
	}
	record, err := h.service.CreateChildRecord(
		r.Context(),
		chi.URLParam(r, "tableCode"),
		chi.URLParam(r, "key"),
		chi.URLParam(r, "childCode"),
		input,
	)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, record)
}

func (h *Handler) updateChildRecord(w http.ResponseWriter, r *http.Request) {
	input, ok := decodeRecordInput(w, r)
	if !ok {
		return
	}
	record, err := h.service.UpdateChildRecord(
		r.Context(),
		chi.URLParam(r, "tableCode"),
		chi.URLParam(r, "key"),
		chi.URLParam(r, "childCode"),
		chi.URLParam(r, "lineKey"),
		input,
	)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (h *Handler) deleteChildRecord(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DeleteChildRecord(
		r.Context(),
		chi.URLParam(r, "tableCode"),
		chi.URLParam(r, "key"),
		chi.URLParam(r, "childCode"),
		chi.URLParam(r, "lineKey"),
	); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func decodeRecordInput(w http.ResponseWriter, r *http.Request) (RecordInput, bool) {
	defer r.Body.Close()
	var input RecordInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return RecordInput{}, false
	}
	if input.Data == nil {
		input.Data = map[string]any{}
	}
	return input, true
}

func decodeActionInput(w http.ResponseWriter, r *http.Request) (ActionInput, bool) {
	defer r.Body.Close()
	var input ActionInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil && !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return ActionInput{}, false
	}
	if input.Data == nil {
		input.Data = map[string]any{}
	}
	return input, true
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
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
