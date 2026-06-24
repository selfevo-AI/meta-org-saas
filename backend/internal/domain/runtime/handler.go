package runtime

import (
	"encoding/json"
	"errors"
	"log"
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
	r.Get("/runtime/operations", h.listOperations)
	r.Post("/runtime/operations/{operationID}/execute", h.executeOperation)
	r.Get("/runtime/entities/{entityKey}/records", h.listRecords)
	r.Get("/runtime/entities/{entityKey}/records/{recordKey}", h.getRecord)
	r.Post("/runtime/entities/{entityKey}/records", h.createRecord)
	r.Patch("/runtime/entities/{entityKey}/records/{recordKey}", h.updateRecord)
	r.Delete("/runtime/entities/{entityKey}/records/{recordKey}", h.deleteRecord)
}

func (h *Handler) RegisterTenantReadRoutes(r chi.Router) {
	r.Get("/runtime/operations", h.listOperations)
}

func (h *Handler) listOperations(w http.ResponseWriter, r *http.Request) {
	operations, err := h.service.ListOperations(r.Context())
	writeResult(w, http.StatusOK, operations, err)
}

func (h *Handler) executeOperation(w http.ResponseWriter, r *http.Request) {
	var input RuntimeExecutionRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.ExecuteOperation(r.Context(), chi.URLParam(r, "operationID"), input)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) listRecords(w http.ResponseWriter, r *http.Request) {
	records, err := h.service.ListRecords(r.Context(), chi.URLParam(r, "entityKey"), queryLimit(r, 100))
	writeResult(w, http.StatusOK, records, err)
}

func (h *Handler) getRecord(w http.ResponseWriter, r *http.Request) {
	record, err := h.service.GetRecord(r.Context(), chi.URLParam(r, "entityKey"), chi.URLParam(r, "recordKey"))
	writeResult(w, http.StatusOK, record, err)
}

func (h *Handler) createRecord(w http.ResponseWriter, r *http.Request) {
	var input RuntimeRecordInput
	if !decodeJSON(w, r, &input) {
		return
	}
	record, err := h.service.CreateRecord(r.Context(), chi.URLParam(r, "entityKey"), input)
	writeResult(w, http.StatusCreated, record, err)
}

func (h *Handler) updateRecord(w http.ResponseWriter, r *http.Request) {
	var input RuntimeRecordInput
	if !decodeJSON(w, r, &input) {
		return
	}
	record, err := h.service.UpdateRecord(r.Context(), chi.URLParam(r, "entityKey"), chi.URLParam(r, "recordKey"), input)
	writeResult(w, http.StatusOK, record, err)
}

func (h *Handler) deleteRecord(w http.ResponseWriter, r *http.Request) {
	err := h.service.DeleteRecord(r.Context(), chi.URLParam(r, "entityKey"), chi.URLParam(r, "recordKey"))
	writeResult(w, http.StatusOK, map[string]string{"status": "deleted"}, err)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	if r.Body == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return false
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(target); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return false
	}
	return true
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
	case errors.Is(err, ErrForbidden):
		status = http.StatusForbidden
	case errors.Is(err, ErrNotFound):
		status = http.StatusNotFound
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("runtime write json: %v", err)
	}
}

func queryLimit(r *http.Request, fallback int) int {
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			return parsed
		}
	}
	return fallback
}
