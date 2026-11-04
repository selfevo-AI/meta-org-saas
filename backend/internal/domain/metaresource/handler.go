package metaresource

import (
	"encoding/json"
	"errors"
	"log/slog"
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
	r.Get("/meta-resources", h.listResources)
	r.Post("/meta-resources", h.createResource)
	r.Get("/meta-resources/{id}", h.getResource)
	// Bulk re-sync rewrites the whole resource registry.
	r.With(middleware.RequireAuthority(middleware.AuthorityReviewer)).Post("/meta-resources/sync-existing", h.syncExistingResources)
	r.Get("/meta-resources/summary", h.resourceSummary)
	r.Get("/demand-profiles", h.listDemandProfiles)
	r.Post("/demand-profiles", h.createDemandProfile)
	r.Get("/pdca-cycles", h.listCycles)
	r.Post("/pdca-cycles", h.createCycle)
	r.Get("/pdca-events", h.listEvents)
	r.Post("/pdca-events", h.createEvent)
}

func (h *Handler) getResource(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.writeError(w, err)
		return
	}
	item, err := h.service.GetResource(r.Context(), id)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) listResources(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ListResources(r.Context(), ListFilter{
		Limit:        queryLimit(r),
		ResourceType: r.URL.Query().Get("resource_type"),
		Status:       r.URL.Query().Get("status"),
	})
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) createResource(w http.ResponseWriter, r *http.Request) {
	var input CreateMetaResourceInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.writeError(w, err)
		return
	}
	item, err := h.service.CreateResource(r.Context(), input)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h *Handler) syncExistingResources(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.SyncExistingResources(r.Context())
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) resourceSummary(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ResourceSummary(r.Context(), queryLimit(r))
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) listDemandProfiles(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ListDemandProfiles(r.Context(), queryLimit(r))
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) createDemandProfile(w http.ResponseWriter, r *http.Request) {
	var input CreateDemandProfileInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.writeError(w, err)
		return
	}
	item, err := h.service.CreateDemandProfile(r.Context(), input)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h *Handler) listCycles(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ListCycles(r.Context(), queryLimit(r))
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) createCycle(w http.ResponseWriter, r *http.Request) {
	var input CreatePDCACycleInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.writeError(w, err)
		return
	}
	item, err := h.service.CreateCycle(r.Context(), input)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h *Handler) listEvents(w http.ResponseWriter, r *http.Request) {
	filter := ListFilter{Limit: queryLimit(r)}
	if raw := r.URL.Query().Get("cycle_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			h.writeError(w, err)
			return
		}
		filter.CycleID = &id
	}
	items, err := h.service.ListEvents(r.Context(), filter)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) createEvent(w http.ResponseWriter, r *http.Request) {
	var input CreatePDCAEventInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.writeError(w, err)
		return
	}
	item, err := h.service.CreateEvent(r.Context(), input)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h *Handler) writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if errors.Is(err, ErrValidation) || errors.Is(err, strconv.ErrSyntax) {
		status = http.StatusBadRequest
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func queryLimit(r *http.Request) int {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		return 100
	}
	if limit > 500 {
		return 500
	}
	return limit
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		slog.Error("metaresource writeJSON error", "error", err)
	}
}
