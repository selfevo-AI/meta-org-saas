package industry

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
	r.Get("/industries/catalog", h.listIndustries)
	r.Get("/platform/admin/industries", h.listIndustries)
	r.Post("/platform/admin/industries", h.createIndustry)
	r.Get("/platform/admin/industries/{industryKey}/packages", h.listPackages)
	r.Post("/platform/admin/industries/{industryKey}/packages", h.createPackage)
	r.Patch("/platform/admin/industry-packages/{packageID}", h.updatePackage)
	r.Delete("/platform/admin/industry-packages/{packageID}", h.deletePackage)
	r.Post("/platform/admin/industry-packages/{packageID}/activate", h.activatePackage)
	r.Post("/platform/admin/industry-packages/{packageID}/apply-to-organization/{organizationID}", h.applyPackage)
	r.Get("/platform/admin/industry-publication-requests", h.listPublicationRequests)
	r.Post("/platform/admin/industry-publication-requests/{requestID}/approve", h.approvePublicationRequest)
	r.Post("/platform/admin/industry-publication-requests/{requestID}/reject", h.rejectPublicationRequest)
	r.Get("/organizations/{organizationID}/industry", h.getAdoption)
	r.Put("/organizations/{organizationID}/industry", h.applyPackage)
	r.Get("/organizations/{organizationID}/industry/modules", h.getAdoption)
	r.Put("/organizations/{organizationID}/industry/modules", h.applyPackage)
	r.Get("/industry/extensions", h.listExtensions)
	r.Post("/industry/extensions", h.createExtension)
	r.Post("/industry/extensions/{extensionID}/submit-publication", h.submitPublicationRequest)
	r.Get("/industry/knowledge-sources", h.listKnowledgeSources)
}

func (h *Handler) listIndustries(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListIndustries(r.Context(), queryLimit(r, 100))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) createIndustry(w http.ResponseWriter, r *http.Request) {
	actorID, ok := authenticatedHumanID(w, r)
	if !ok {
		return
	}
	var input CreateIndustryInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateIndustry(r.Context(), actorID, input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) listPackages(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListPackages(r.Context(), chi.URLParam(r, "industryKey"), queryLimit(r, 100))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) createPackage(w http.ResponseWriter, r *http.Request) {
	actorID, ok := authenticatedHumanID(w, r)
	if !ok {
		return
	}
	var input CreatePackageInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.IndustryKey = chi.URLParam(r, "industryKey")
	result, err := h.service.CreatePackage(r.Context(), actorID, input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) updatePackage(w http.ResponseWriter, r *http.Request) {
	actorID, packageID, ok := h.actorAndID(w, r, "packageID", "invalid package id")
	if !ok {
		return
	}
	var input UpdatePackageInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.UpdatePackage(r.Context(), actorID, packageID, input)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) deletePackage(w http.ResponseWriter, r *http.Request) {
	actorID, packageID, ok := h.actorAndID(w, r, "packageID", "invalid package id")
	if !ok {
		return
	}
	result, err := h.service.DeletePackage(r.Context(), actorID, packageID)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) activatePackage(w http.ResponseWriter, r *http.Request) {
	actorID, packageID, ok := h.actorAndID(w, r, "packageID", "invalid package id")
	if !ok {
		return
	}
	result, err := h.service.ActivatePackage(r.Context(), actorID, packageID)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) applyPackage(w http.ResponseWriter, r *http.Request) {
	actorID, ok := authenticatedHumanID(w, r)
	if !ok {
		return
	}
	var input ApplyPackageInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if raw := chi.URLParam(r, "packageID"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid package id"})
			return
		}
		input.PackageID = id
	}
	if raw := chi.URLParam(r, "organizationID"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid organization id"})
			return
		}
		input.OrganizationID = id
	}
	result, err := h.service.ApplyPackageToOrganization(r.Context(), actorID, input)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) getAdoption(w http.ResponseWriter, r *http.Request) {
	actorID, ok := authenticatedHumanID(w, r)
	if !ok {
		return
	}
	orgID, ok := pathUUID(w, r, "organizationID", "invalid organization id")
	if !ok {
		return
	}
	result, err := h.service.GetAdoption(r.Context(), actorID, orgID)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) createExtension(w http.ResponseWriter, r *http.Request) {
	actorID, ok := authenticatedHumanID(w, r)
	if !ok {
		return
	}
	var input CreateExtensionInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateExtension(r.Context(), actorID, input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) listExtensions(w http.ResponseWriter, r *http.Request) {
	actorID, ok := authenticatedHumanID(w, r)
	if !ok {
		return
	}
	orgID, err := uuid.Parse(r.URL.Query().Get("organization_id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization_id is required"})
		return
	}
	result, err := h.service.ListExtensions(r.Context(), actorID, orgID, queryLimit(r, 100))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) submitPublicationRequest(w http.ResponseWriter, r *http.Request) {
	actorID, extensionID, ok := h.actorAndID(w, r, "extensionID", "invalid extension id")
	if !ok {
		return
	}
	var input struct {
		Reason string `json:"reason"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input)
	}
	result, err := h.service.SubmitPublicationRequest(r.Context(), actorID, extensionID, input.Reason)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) listPublicationRequests(w http.ResponseWriter, r *http.Request) {
	actorID, ok := authenticatedHumanID(w, r)
	if !ok {
		return
	}
	result, err := h.service.ListPublicationRequests(r.Context(), actorID, queryLimit(r, 100))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) approvePublicationRequest(w http.ResponseWriter, r *http.Request) {
	h.reviewPublicationRequest(w, r, PublicationApproved)
}

func (h *Handler) rejectPublicationRequest(w http.ResponseWriter, r *http.Request) {
	h.reviewPublicationRequest(w, r, PublicationRejected)
}

func (h *Handler) reviewPublicationRequest(w http.ResponseWriter, r *http.Request, status string) {
	actorID, requestID, ok := h.actorAndID(w, r, "requestID", "invalid publication request id")
	if !ok {
		return
	}
	var input struct {
		Reason string `json:"reason"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input)
	}
	result, err := h.service.ReviewPublicationRequest(r.Context(), actorID, requestID, status, input.Reason)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) listKnowledgeSources(w http.ResponseWriter, r *http.Request) {
	actorID, ok := authenticatedHumanID(w, r)
	if !ok {
		return
	}
	var orgID uuid.UUID
	if raw := r.URL.Query().Get("organization_id"); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid organization id"})
			return
		}
		orgID = parsed
	}
	result, err := h.service.ListKnowledgeSources(r.Context(), actorID, r.URL.Query().Get("industry_key"), orgID, queryLimit(r, 100))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) actorAndID(w http.ResponseWriter, r *http.Request, param string, message string) (uuid.UUID, uuid.UUID, bool) {
	actorID, ok := authenticatedHumanID(w, r)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	id, ok := pathUUID(w, r, param, message)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	return actorID, id, true
}

func pathUUID(w http.ResponseWriter, r *http.Request, param string, message string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, param))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": message})
		return uuid.Nil, false
	}
	return id, true
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
		log.Printf("industry write json: %v", err)
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
