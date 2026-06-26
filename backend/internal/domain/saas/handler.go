package saas

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

func (h *Handler) RegisterPublicRoutes(r chi.Router) {
	r.Post("/auth/invitations/{token}/accept", h.acceptInvitation)
}

func (h *Handler) RegisterAuthenticatedRoutes(r chi.Router) {
	r.Get("/auth/me", h.me)
	r.Get("/modules", h.listModules)
	r.Post("/onboarding/organization", h.completeOnboarding)
	r.Get("/platform/organizations", h.listPlatformOrganizations)
	r.Post("/platform/admin/sample-tenants/business-closure", h.createBusinessClosureSampleTenant)
	r.Post("/platform/admin/organizations/{id}/close", h.closePlatformOrganization)
}

func (h *Handler) RegisterTenantRoutes(r chi.Router) {
	r.Get("/organizations/{id}/subscription", h.getSubscription)
	r.Get("/organizations/{id}/entitlements", h.getEntitlements)
	r.Patch("/organizations/{id}/modules", h.updateOrganizationModules)
	r.Get("/organizations/{id}/invitations", h.listInvitations)
	r.Post("/organizations/{id}/invitations", h.createInvitation)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	userID, ok := authenticatedUserID(w, r)
	if !ok {
		return
	}
	profile, err := h.service.GetProfile(r.Context(), userID)
	writeResult(w, http.StatusOK, profile, err)
}

func (h *Handler) listModules(w http.ResponseWriter, r *http.Request) {
	modules, err := h.service.ListModules(r.Context())
	writeResult(w, http.StatusOK, modules, err)
}

func (h *Handler) completeOnboarding(w http.ResponseWriter, r *http.Request) {
	userID, ok := authenticatedUserID(w, r)
	if !ok {
		return
	}
	var input OnboardingOrganizationInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CompleteOnboarding(r.Context(), userID, input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) listPlatformOrganizations(w http.ResponseWriter, r *http.Request) {
	userID, ok := authenticatedUserID(w, r)
	if !ok {
		return
	}
	items, err := h.service.ListPlatformOrganizations(r.Context(), userID, queryLimit(r))
	writeResult(w, http.StatusOK, items, err)
}

func (h *Handler) closePlatformOrganization(w http.ResponseWriter, r *http.Request) {
	userID, ok := authenticatedUserID(w, r)
	if !ok {
		return
	}
	orgID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid organization id"})
		return
	}
	var input CloseOrganizationInput
	if r.Body != nil {
		_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input)
	}
	result, err := h.service.CloseOrganization(r.Context(), userID, orgID, input)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) createBusinessClosureSampleTenant(w http.ResponseWriter, r *http.Request) {
	userID, ok := authenticatedUserID(w, r)
	if !ok {
		return
	}
	result, err := h.service.CreateBusinessClosureSampleTenant(r.Context(), userID)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) getSubscription(w http.ResponseWriter, r *http.Request) {
	userID, orgID, ok := h.authenticatedOrg(w, r)
	if !ok {
		return
	}
	result, err := h.service.GetSubscription(r.Context(), userID, orgID)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) getEntitlements(w http.ResponseWriter, r *http.Request) {
	userID, orgID, ok := h.authenticatedOrg(w, r)
	if !ok {
		return
	}
	result, err := h.service.GetEntitlements(r.Context(), userID, orgID)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) updateOrganizationModules(w http.ResponseWriter, r *http.Request) {
	userID, orgID, ok := h.authenticatedOrg(w, r)
	if !ok {
		return
	}
	var input UpdateOrganizationModulesInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.UpdateOrganizationModules(r.Context(), userID, orgID, input)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) listInvitations(w http.ResponseWriter, r *http.Request) {
	userID, orgID, ok := h.authenticatedOrg(w, r)
	if !ok {
		return
	}
	result, err := h.service.ListInvitations(r.Context(), userID, orgID, queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) createInvitation(w http.ResponseWriter, r *http.Request) {
	userID, orgID, ok := h.authenticatedOrg(w, r)
	if !ok {
		return
	}
	var input CreateInvitationInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateInvitation(r.Context(), userID, orgID, input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) acceptInvitation(w http.ResponseWriter, r *http.Request) {
	var input AcceptInvitationInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.AcceptInvitation(r.Context(), chi.URLParam(r, "token"), input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) authenticatedOrg(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	userID, ok := authenticatedUserID(w, r)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	orgID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid organization id"})
		return uuid.Nil, uuid.Nil, false
	}
	return userID, orgID, true
}

func authenticatedUserID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok || user.Type != "human" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return uuid.Nil, false
	}
	userID, err := uuid.Parse(user.ID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid authenticated user"})
		return uuid.Nil, false
	}
	return userID, true
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
	case errors.Is(err, ErrConflict):
		status = http.StatusConflict
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("saas write json: %v", err)
	}
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
