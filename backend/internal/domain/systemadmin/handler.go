package systemadmin

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterAuthenticatedRoutes(r chi.Router) {
	r.Get("/platform/admin/me/permissions", h.getPermissionProfile)
	r.Get("/platform/admin/modules/{moduleKey}/masters", h.listPlatformMasters)
	r.Get("/platform/admin/masters/{masterKey}/details", h.listPlatformDetails)
	r.Get("/platform/admin/schema-targets", h.listSchemaTargets)
	r.Get("/platform/admin/organizations/{id}/schema/export", h.exportOrganizationSchema)
	r.Post("/platform/admin/organizations/{id}/schema/import", h.createOrganizationSchemaChange)
	r.Post("/platform/admin/organizations/{id}/schema/change-requests", h.createOrganizationSchemaChange)
	r.Post("/platform/admin/organizations/{id}/industry-solution-flows/erp-standard", h.createERPSolutionFlow)
	r.Post("/platform/admin/schema-change-requests/{id}/approve", h.approveSchemaChange)
	r.Post("/platform/admin/schema-change-requests/{id}/apply", h.applySchemaChange)
}

func (h *Handler) getPermissionProfile(w http.ResponseWriter, r *http.Request) {
	actorID, ok := authenticatedHumanID(w, r)
	if !ok {
		return
	}
	result, err := h.service.GetPermissionProfile(r.Context(), actorID)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) listPlatformMasters(w http.ResponseWriter, r *http.Request) {
	actorID, ok := authenticatedHumanID(w, r)
	if !ok {
		return
	}
	result, err := h.service.ListPlatformMasters(r.Context(), actorID, chi.URLParam(r, "moduleKey"), queryLimit(r, 100))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) listPlatformDetails(w http.ResponseWriter, r *http.Request) {
	actorID, ok := authenticatedHumanID(w, r)
	if !ok {
		return
	}
	result, err := h.service.ListPlatformDetails(r.Context(), actorID, chi.URLParam(r, "masterKey"))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) listSchemaTargets(w http.ResponseWriter, r *http.Request) {
	actorID, ok := authenticatedHumanID(w, r)
	if !ok {
		return
	}
	result, err := h.service.ListSchemaTargets(r.Context(), actorID, queryLimit(r, 100))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) exportOrganizationSchema(w http.ResponseWriter, r *http.Request) {
	actorID, orgID, ok := h.actorAndOrganization(w, r)
	if !ok {
		return
	}
	result, err := h.service.ExportOrganizationSchema(r.Context(), actorID, orgID)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) createOrganizationSchemaChange(w http.ResponseWriter, r *http.Request) {
	actorID, orgID, ok := h.actorAndOrganization(w, r)
	if !ok {
		return
	}
	var input CreateSchemaChangeRequestInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.OrganizationID = orgID
	if input.SchemaPackage.FormatVersion == "" {
		input.SchemaPackage = DefaultOrganizationSchemaPackage()
	}
	result, err := h.service.CreateSchemaChangeRequest(r.Context(), actorID, input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) createERPSolutionFlow(w http.ResponseWriter, r *http.Request) {
	actorID, orgID, ok := h.actorAndOrganization(w, r)
	if !ok {
		return
	}
	var input ERPSolutionFlowRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	input.OrganizationID = orgID
	result, err := h.service.BuildERPSolutionFlow(r.Context(), actorID, input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) approveSchemaChange(w http.ResponseWriter, r *http.Request) {
	actorID, requestID, ok := h.actorAndRequest(w, r)
	if !ok {
		return
	}
	var input struct {
		Reason string `json:"reason"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input)
	}
	result, err := h.service.ApproveSchemaChange(r.Context(), actorID, requestID, input.Reason)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) applySchemaChange(w http.ResponseWriter, r *http.Request) {
	actorID, requestID, ok := h.actorAndRequest(w, r)
	if !ok {
		return
	}
	result, err := h.service.ApplySchemaChange(r.Context(), actorID, requestID)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) actorAndOrganization(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	actorID, ok := authenticatedHumanID(w, r)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	orgID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid organization id"})
		return uuid.Nil, uuid.Nil, false
	}
	return actorID, orgID, true
}

func (h *Handler) actorAndRequest(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	actorID, ok := authenticatedHumanID(w, r)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	requestID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid schema change request id"})
		return uuid.Nil, uuid.Nil, false
	}
	return actorID, requestID, true
}

func authenticatedHumanID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok || user.Type != "human" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return uuid.Nil, false
	}
	actorID, err := uuid.Parse(user.ID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid authenticated user"})
		return uuid.Nil, false
	}
	return actorID, true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
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
	case errors.Is(err, ErrInvalidTransition):
		status = http.StatusConflict
	case errors.Is(err, ErrForbidden):
		status = http.StatusForbidden
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("systemadmin write json: %v", err)
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
