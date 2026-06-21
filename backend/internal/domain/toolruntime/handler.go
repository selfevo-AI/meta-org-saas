package toolruntime

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/selfevo-AI/meta-org/backend/internal/pkg/dberrors"
	"github.com/selfevo-AI/meta-org/backend/internal/pkg/middleware"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/interface-files", h.createInterfaceFile)
	r.Get("/interface-files", h.listInterfaceFiles)
	r.Get("/interface-files/{id}", h.getInterfaceFile)
	r.Patch("/interface-files/{id}", h.updateInterfaceFile)
	r.Post("/tools", h.createTool)
	r.Get("/tools", h.listTools)
	r.Patch("/tools/{id}", h.updateTool)
	r.Post("/tools/{id}/test", h.testTool)
	r.Get("/tool-executions", h.listExecutions)
	r.Get("/tool-executions/{id}", h.getExecution)
	r.Post("/tool-approvals/{id}/approve", h.approve)
	r.Post("/tool-approvals/{id}/reject", h.reject)
}

func (h *Handler) RegisterPlatformRoutes(r chi.Router) {
	r.Post("/platform/admin/tool-approvals/{id}/approve", h.approve)
	r.Post("/platform/admin/tool-approvals/{id}/reject", h.reject)
}

func (h *Handler) createTool(w http.ResponseWriter, r *http.Request) {
	var input CreateToolInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateTool(r.Context(), input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) listTools(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListTools(r.Context(), queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) updateTool(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var input UpdateToolInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.UpdateTool(r.Context(), id, input)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) createInterfaceFile(w http.ResponseWriter, r *http.Request) {
	var input CreateInterfaceFileInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateInterfaceFile(r.Context(), input, humanUserIDFromContext(r))
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) listInterfaceFiles(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListInterfaceFiles(r.Context(), queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) getInterfaceFile(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	result, err := h.service.GetInterfaceFile(r.Context(), id)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) updateInterfaceFile(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var input UpdateInterfaceFileInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.UpdateInterfaceFile(r.Context(), id, input)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) testTool(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var input ExecuteToolInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.TestTool(r.Context(), id, input)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) listExecutions(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListExecutions(r.Context(), queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) getExecution(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	result, err := h.service.GetExecution(r.Context(), id)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) approve(w http.ResponseWriter, r *http.Request) {
	h.reviewApproval(w, r, ApprovalApproved)
}

func (h *Handler) reject(w http.ResponseWriter, r *http.Request) {
	h.reviewApproval(w, r, ApprovalRejected)
}

func (h *Handler) reviewApproval(w http.ResponseWriter, r *http.Request, status string) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var input struct {
		Reason string `json:"reason,omitempty"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	user, ok := middleware.UserFromContext(r.Context())
	if !ok || user.Type != "human" {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "human reviewer is required"})
		return
	}
	reviewedBy, err := uuid.Parse(user.ID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid reviewer id"})
		return
	}
	var (
		result    *ApprovalReviewOutput
		reviewErr error
	)
	if status == ApprovalApproved {
		result, reviewErr = h.service.Approve(r.Context(), id, &reviewedBy, input.Reason)
	} else {
		result, reviewErr = h.service.Reject(r.Context(), id, &reviewedBy, input.Reason)
	}
	writeResult(w, http.StatusOK, result, reviewErr)
}

func humanUserIDFromContext(r *http.Request) *uuid.UUID {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok || user.Type != "human" {
		return nil
	}
	id, err := uuid.Parse(user.ID)
	if err != nil {
		return nil
	}
	return &id
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dest any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(dest); err != nil {
		if errors.Is(err, io.EOF) {
			return true
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return false
	}
	return true
}

func parseID(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return uuid.Nil, false
	}
	return id, true
}

func queryLimit(r *http.Request) int {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	return limit
}

func writeResult(w http.ResponseWriter, successStatus int, payload any, err error) {
	if err != nil {
		writeJSON(w, statusFromError(err), map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, successStatus, payload)
}

func statusFromError(err error) int {
	switch {
	case errors.Is(err, ErrValidation):
		return http.StatusBadRequest
	case dberrors.IsUniqueViolation(err):
		return http.StatusConflict
	case errors.Is(err, ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, ErrNotFound), errors.Is(err, pgx.ErrNoRows):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("toolruntime writeJSON error: %v", err)
	}
}
