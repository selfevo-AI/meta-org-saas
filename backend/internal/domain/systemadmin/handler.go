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
	r.Get("/platform/admin/features", h.listPlatformFeatures)
	r.Post("/platform/admin/features", h.createPlatformFeature)
	r.Post("/platform/admin/features/{featureKey}/publish", h.publishPlatformFeature)
	r.Get("/platform/admin/permissions", h.listPlatformPermissions)
	r.Get("/platform/admin/roles", h.listPlatformRoles)
	r.Put("/platform/admin/roles/{roleKey}/permissions", h.setPlatformRolePermissions)
	r.Get("/platform/admin/users", h.listPlatformUsers)
	r.Post("/platform/admin/users", h.createPlatformUser)
	r.Put("/platform/admin/users/{userID}/roles", h.setPlatformUserRoles)
	r.Post("/platform/admin/users/{userID}/reset-password", h.resetPlatformUserPassword)
	r.Post("/platform/admin/users/{userID}/disable", h.disablePlatformUser)
	r.Get("/platform/admin/database-maintenance/jobs", h.listDatabaseMaintenanceJobs)
	r.Post("/platform/admin/database-maintenance/jobs", h.createDatabaseMaintenanceJob)
	r.Post("/platform/admin/database-maintenance/jobs/{id}/approve", h.approveDatabaseMaintenanceJob)
	r.Post("/platform/admin/database-maintenance/jobs/{id}/reject", h.rejectDatabaseMaintenanceJob)
	r.Get("/platform/admin/modules/{moduleKey}/masters", h.listPlatformMasters)
	r.Get("/platform/admin/masters/{masterKey}/details", h.listPlatformDetails)
	r.Get("/platform/admin/industry-solution-targets", h.listIndustrySolutionTargets)
	r.Post("/platform/admin/organizations/{id}/private-deployment-exports", h.createPrivateDeploymentExport)
	r.Get("/platform/admin/organizations/{id}/industry-solution-manifest/export", h.exportOrganizationIndustrySolutionManifest)
	r.Post("/platform/admin/organizations/{id}/industry-solution-change-requests", h.createIndustrySolutionChange)
	r.Post("/platform/admin/organizations/{id}/industry-solution-table-fields/change-requests", h.createIndustrySolutionTableFieldChange)
	r.Post("/platform/admin/organizations/{id}/industry-solution-flows/erp-standard", h.createERPSolutionFlow)
	r.Post("/platform/admin/organizations/{id}/industry-solution-flows/retail-distribution", h.createRetailDistributionSolutionFlow)
	r.Post("/platform/admin/industry-solution-change-requests/{id}/approve", h.approveIndustrySolutionChange)
	r.Post("/platform/admin/industry-solution-change-requests/{id}/verify", h.verifyIndustrySolutionChange)
	r.Post("/platform/admin/industry-solution-change-requests/{id}/apply", h.applyIndustrySolutionChange)
	r.Get("/platform/admin/industry-solution-change-requests/{id}/asset-diff", h.getIndustrySolutionChangeAssetDiff)
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

func (h *Handler) listPlatformFeatures(w http.ResponseWriter, r *http.Request) {
	actorID, ok := authenticatedHumanID(w, r)
	if !ok {
		return
	}
	result, err := h.service.ListPlatformFeatures(r.Context(), actorID, r.URL.Query().Get("status"), queryLimit(r, 200))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) createPlatformFeature(w http.ResponseWriter, r *http.Request) {
	actorID, ok := authenticatedHumanID(w, r)
	if !ok {
		return
	}
	var input CreatePlatformFeatureInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreatePlatformFeature(r.Context(), actorID, input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) publishPlatformFeature(w http.ResponseWriter, r *http.Request) {
	actorID, ok := authenticatedHumanID(w, r)
	if !ok {
		return
	}
	result, err := h.service.PublishPlatformFeature(r.Context(), actorID, chi.URLParam(r, "featureKey"))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) listPlatformPermissions(w http.ResponseWriter, r *http.Request) {
	actorID, ok := authenticatedHumanID(w, r)
	if !ok {
		return
	}
	result, err := h.service.ListPlatformPermissions(r.Context(), actorID)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) listPlatformRoles(w http.ResponseWriter, r *http.Request) {
	actorID, ok := authenticatedHumanID(w, r)
	if !ok {
		return
	}
	result, err := h.service.ListPlatformRoles(r.Context(), actorID)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) setPlatformRolePermissions(w http.ResponseWriter, r *http.Request) {
	actorID, ok := authenticatedHumanID(w, r)
	if !ok {
		return
	}
	var input SetPlatformRolePermissionsInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.SetPlatformRolePermissions(r.Context(), actorID, chi.URLParam(r, "roleKey"), input)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) listPlatformUsers(w http.ResponseWriter, r *http.Request) {
	actorID, ok := authenticatedHumanID(w, r)
	if !ok {
		return
	}
	result, err := h.service.ListPlatformUsers(r.Context(), actorID, queryLimit(r, 100))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) createPlatformUser(w http.ResponseWriter, r *http.Request) {
	actorID, ok := authenticatedHumanID(w, r)
	if !ok {
		return
	}
	var input CreatePlatformUserInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreatePlatformUser(r.Context(), actorID, input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) setPlatformUserRoles(w http.ResponseWriter, r *http.Request) {
	actorID, userID, ok := h.actorAndUser(w, r)
	if !ok {
		return
	}
	var input struct {
		Roles []string `json:"roles"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.SetPlatformUserRoles(r.Context(), actorID, userID, input.Roles)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) resetPlatformUserPassword(w http.ResponseWriter, r *http.Request) {
	actorID, userID, ok := h.actorAndUser(w, r)
	if !ok {
		return
	}
	result, err := h.service.ResetPlatformUserPassword(r.Context(), actorID, userID)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) disablePlatformUser(w http.ResponseWriter, r *http.Request) {
	actorID, userID, ok := h.actorAndUser(w, r)
	if !ok {
		return
	}
	result, err := h.service.DisablePlatformUser(r.Context(), actorID, userID)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) listDatabaseMaintenanceJobs(w http.ResponseWriter, r *http.Request) {
	actorID, ok := authenticatedHumanID(w, r)
	if !ok {
		return
	}
	result, err := h.service.ListDatabaseMaintenanceJobs(r.Context(), actorID, queryLimit(r, 100))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) createDatabaseMaintenanceJob(w http.ResponseWriter, r *http.Request) {
	actorID, ok := authenticatedHumanID(w, r)
	if !ok {
		return
	}
	var input CreateDatabaseMaintenanceJobInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateDatabaseMaintenanceJob(r.Context(), actorID, input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) approveDatabaseMaintenanceJob(w http.ResponseWriter, r *http.Request) {
	h.reviewDatabaseMaintenanceJob(w, r, "approve")
}

func (h *Handler) rejectDatabaseMaintenanceJob(w http.ResponseWriter, r *http.Request) {
	h.reviewDatabaseMaintenanceJob(w, r, "reject")
}

func (h *Handler) reviewDatabaseMaintenanceJob(w http.ResponseWriter, r *http.Request, decision string) {
	actorID, requestID, ok := h.actorAndRequest(w, r)
	if !ok {
		return
	}
	var input ReviewDatabaseMaintenanceJobInput
	if r.Body != nil {
		_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input)
	}
	input.Decision = decision
	result, err := h.service.ReviewDatabaseMaintenanceJob(r.Context(), actorID, requestID, input)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) listIndustrySolutionTargets(w http.ResponseWriter, r *http.Request) {
	actorID, ok := authenticatedHumanID(w, r)
	if !ok {
		return
	}
	result, err := h.service.ListIndustrySolutionTargets(r.Context(), actorID, queryLimit(r, 100))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) exportOrganizationIndustrySolutionManifest(w http.ResponseWriter, r *http.Request) {
	actorID, orgID, ok := h.actorAndOrganization(w, r)
	if !ok {
		return
	}
	result, err := h.service.ExportOrganizationIndustrySolutionManifest(r.Context(), actorID, orgID)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) createPrivateDeploymentExport(w http.ResponseWriter, r *http.Request) {
	actorID, orgID, ok := h.actorAndOrganization(w, r)
	if !ok {
		return
	}
	result, err := h.service.CreatePrivateDeploymentExportJob(r.Context(), actorID, orgID)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) createIndustrySolutionChange(w http.ResponseWriter, r *http.Request) {
	actorID, orgID, ok := h.actorAndOrganization(w, r)
	if !ok {
		return
	}
	var input CreateIndustrySolutionChangeRequestInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.OrganizationID = orgID
	if input.SolutionManifest.FormatVersion == "" {
		input.SolutionManifest = DefaultOrganizationIndustrySolutionManifest()
	}
	result, err := h.service.CreateIndustrySolutionChangeRequest(r.Context(), actorID, input)
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

func (h *Handler) createRetailDistributionSolutionFlow(w http.ResponseWriter, r *http.Request) {
	actorID, orgID, ok := h.actorAndOrganization(w, r)
	if !ok {
		return
	}
	var input ERPSolutionFlowRequest
	if r.Body != nil && r.ContentLength != 0 {
		if !decodeJSON(w, r, &input) {
			return
		}
	}
	input.OrganizationID = orgID
	result, err := h.service.BuildRetailDistributionSolutionFlow(r.Context(), actorID, input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) createIndustrySolutionTableFieldChange(w http.ResponseWriter, r *http.Request) {
	actorID, orgID, ok := h.actorAndOrganization(w, r)
	if !ok {
		return
	}
	var input CreateIndustrySolutionTableFieldChangeInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.OrganizationID = orgID
	result, err := h.service.CreateIndustrySolutionTableFieldChange(r.Context(), actorID, input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) approveIndustrySolutionChange(w http.ResponseWriter, r *http.Request) {
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
	result, err := h.service.ApproveIndustrySolutionChange(r.Context(), actorID, requestID, input.Reason)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) verifyIndustrySolutionChange(w http.ResponseWriter, r *http.Request) {
	actorID, requestID, ok := h.actorAndRequest(w, r)
	if !ok {
		return
	}
	result, err := h.service.VerifyIndustrySolutionChange(r.Context(), actorID, requestID)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) applyIndustrySolutionChange(w http.ResponseWriter, r *http.Request) {
	actorID, requestID, ok := h.actorAndRequest(w, r)
	if !ok {
		return
	}
	result, err := h.service.ApplyIndustrySolutionChange(r.Context(), actorID, requestID)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) getIndustrySolutionChangeAssetDiff(w http.ResponseWriter, r *http.Request) {
	actorID, requestID, ok := h.actorAndRequest(w, r)
	if !ok {
		return
	}
	result, err := h.service.GetIndustrySolutionChangeAssetDiff(r.Context(), actorID, requestID)
	writeResult(w, http.StatusOK, map[string]any{"diff": result}, err)
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
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid industry solution change request id"})
		return uuid.Nil, uuid.Nil, false
	}
	return actorID, requestID, true
}

func (h *Handler) actorAndUser(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	actorID, ok := authenticatedHumanID(w, r)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	userID, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user id"})
		return uuid.Nil, uuid.Nil, false
	}
	return actorID, userID, true
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
