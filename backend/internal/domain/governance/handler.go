package governance

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

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/governance/permissions", h.createPermission)
	r.Get("/governance/permissions", h.listPermissions)
	r.Post("/governance/principles", h.createPrinciple)
	r.Get("/governance/principles", h.listPrinciples)
	r.Get("/governance/principles/{id}", h.getPrinciple)
	r.Post("/governance/control-rules", h.createControlRule)
	r.Get("/governance/control-rules", h.listControlRules)
	r.Post("/governance/check", h.checkPermission)
	r.Post("/governance/access/decide", h.decideAccess)
	r.Get("/governance/access/decisions", h.listAccessDecisions)
	r.Get("/governance/data/tables", h.listDataTables)
	r.Get("/governance/data/tables/{table}/fields", h.listDataFields)
	r.Get("/governance/data/field-preferences/{table}", h.getUserFieldPreference)
	r.Put("/governance/data/field-preferences/{table}", h.upsertUserFieldPreference)
	r.Post("/governance/data/field-permissions", h.createFieldPermissionRule)
	r.Get("/governance/data/field-permissions", h.listFieldPermissionRules)
	r.Post("/governance/data/field-access/check", h.checkFieldAccess)
	r.Get("/preferences/{key}", h.getUserUIPreference)
	r.Put("/preferences/{key}", h.upsertUserUIPreference)
}

func (h *Handler) createPermission(w http.ResponseWriter, r *http.Request) {
	var input Permission
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	perm, err := h.service.CreatePermission(r.Context(), &input)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, perm)
}

func (h *Handler) listPermissions(w http.ResponseWriter, r *http.Request) {
	perms, err := h.service.ListPermissions(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, perms)
}

func (h *Handler) createPrinciple(w http.ResponseWriter, r *http.Request) {
	var input CreatePrincipleInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	prin, err := h.service.CreatePrinciple(r.Context(), input)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, prin)
}

func (h *Handler) listPrinciples(w http.ResponseWriter, r *http.Request) {
	principles, err := h.service.ListPrinciples(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, principles)
}

func (h *Handler) getPrinciple(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	prin, err := h.service.GetPrinciple(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "principle not found"})
		return
	}
	writeJSON(w, http.StatusOK, prin)
}

func (h *Handler) createControlRule(w http.ResponseWriter, r *http.Request) {
	var input CreateControlRuleInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	rule, err := h.service.CreateControlRule(r.Context(), input)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, rule)
}

func (h *Handler) listControlRules(w http.ResponseWriter, r *http.Request) {
	rules, err := h.service.ListControlRules(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, rules)
}

func (h *Handler) checkPermission(w http.ResponseWriter, r *http.Request) {
	var input PermissionCheckInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	result, err := h.service.CheckPermission(r.Context(), input)
	if err != nil {
		if errors.Is(err, ErrForbidden) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
			return
		}
		if errors.Is(err, ErrValidation) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) decideAccess(w http.ResponseWriter, r *http.Request) {
	var input AccessDecisionInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	result, err := h.service.DecideAccess(r.Context(), input)
	if err != nil {
		if errors.Is(err, ErrForbidden) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) listAccessDecisions(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	results, err := h.service.ListAccessDecisions(r.Context(), limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func (h *Handler) listDataTables(w http.ResponseWriter, r *http.Request) {
	tables, err := h.service.ListDataTables(r.Context(), r.URL.Query().Get("category"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, tables)
}

func (h *Handler) listDataFields(w http.ResponseWriter, r *http.Request) {
	fields, err := h.service.ListDataFields(r.Context(), chi.URLParam(r, "table"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, fields)
}

func (h *Handler) getUserFieldPreference(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	pref, err := h.service.GetUserFieldPreference(r.Context(), user.ID, chi.URLParam(r, "table"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, pref)
}

func (h *Handler) upsertUserFieldPreference(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	var input UpsertUserFieldPreferenceInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	pref, err := h.service.UpsertUserFieldPreference(r.Context(), user.ID, chi.URLParam(r, "table"), input)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, pref)
}

func (h *Handler) getUserUIPreference(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	pref, err := h.service.GetUserUIPreference(r.Context(), user.ID, chi.URLParam(r, "key"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, pref)
}

func (h *Handler) upsertUserUIPreference(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	var input UpsertUserUIPreferenceInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	pref, err := h.service.UpsertUserUIPreference(r.Context(), user.ID, chi.URLParam(r, "key"), input)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, pref)
}

func (h *Handler) createFieldPermissionRule(w http.ResponseWriter, r *http.Request) {
	var input CreateFieldPermissionRuleInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	rule, err := h.service.CreateFieldPermissionRule(r.Context(), input)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, rule)
}

func (h *Handler) listFieldPermissionRules(w http.ResponseWriter, r *http.Request) {
	rules, err := h.service.ListFieldPermissionRules(r.Context(), r.URL.Query().Get("table"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, rules)
}

func (h *Handler) checkFieldAccess(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	var input FieldAccessCheckInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if input.ActorID == "" {
		input.ActorID = user.ID
	}
	if input.ActorType == "" {
		input.ActorType = user.Type
	}
	result, err := h.service.CheckFieldAccess(r.Context(), input)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON error: %v", err)
	}
}
