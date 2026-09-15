package ontology

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/erp"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/ontology/types", h.types)
	r.Get("/ontology/types/{type}", h.objectType)
	r.Get("/ontology/objects/{type}", h.query)
	r.Post("/ontology/objects/{type}/query", h.query)
	r.Get("/ontology/objects/{type}/{key}", h.object)
	r.Get("/ontology/objects/{type}/{key}/links", h.links)
	r.Get("/ontology/objects/{type}/{key}/history", h.history)
	r.Post("/ontology/objects/{type}/{key}/actions/{action}", h.execute)
}

func (h *Handler) types(w http.ResponseWriter, r *http.Request) {
	types, err := h.service.Types(r.Context())
	respond(w, map[string]any{"object_types": types}, err)
}

func (h *Handler) objectType(w http.ResponseWriter, r *http.Request) {
	typ, err := h.service.Type(r.Context(), chi.URLParam(r, "type"))
	respond(w, typ, err)
}

func (h *Handler) query(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	input := QueryInput{Search: r.URL.Query().Get("search"), Cursor: r.URL.Query().Get("cursor"), Limit: limit,
		Status: r.URL.Query().Get("status"), Sort: r.URL.Query().Get("sort"), Direction: r.URL.Query().Get("direction")}
	if r.Method == http.MethodPost && !decode(w, r, &input) {
		return
	}
	result, err := h.service.Query(r.Context(), chi.URLParam(r, "type"), input)
	respond(w, result, err)
}

func (h *Handler) object(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.Get(r.Context(), chi.URLParam(r, "type"), chi.URLParam(r, "key"))
	respond(w, result, err)
}

func (h *Handler) links(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.Links(r.Context(), chi.URLParam(r, "type"), chi.URLParam(r, "key"))
	respond(w, result, err)
}

func (h *Handler) history(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.History(r.Context(), chi.URLParam(r, "type"), chi.URLParam(r, "key"), 100)
	respond(w, map[string]any{"executions": result}, err)
}

func (h *Handler) execute(w http.ResponseWriter, r *http.Request) {
	var input erp.ActionInput
	if !decode(w, r, &input) {
		return
	}
	input.ActorID, input.ToolExecutionID, input.AssistantSessionID = nil, nil, nil
	input.ActorType, input.Source = "", "ontology_api"
	if user, ok := middleware.UserFromContext(r.Context()); ok {
		if id, err := uuid.Parse(user.ID); err == nil {
			input.ActorID = &id
		}
		input.ActorType = user.Type
	}
	if key := r.Header.Get("Idempotency-Key"); key != "" {
		input.IdempotencyKey = key
	}
	result, err := h.service.Execute(r.Context(), chi.URLParam(r, "type"), chi.URLParam(r, "key"), chi.URLParam(r, "action"), input)
	respond(w, result, err)
}

func decode(w http.ResponseWriter, r *http.Request, input any) bool {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(input); err != nil && !errors.Is(err, io.EOF) {
		respond(w, nil, erp.ErrValidation)
		return false
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		respond(w, nil, erp.ErrValidation)
		return false
	}
	return true
}

func respond(w http.ResponseWriter, value any, err error) {
	status := http.StatusOK
	if err != nil {
		message := err.Error()
		switch {
		case errors.Is(err, erp.ErrValidation):
			status = http.StatusBadRequest
		case errors.Is(err, erp.ErrForbidden):
			status = http.StatusForbidden
		case errors.Is(err, erp.ErrNotFound):
			status = http.StatusNotFound
		case errors.Is(err, erp.ErrConflict):
			status = http.StatusConflict
		default:
			status, message = http.StatusInternalServerError, "ontology operation failed"
		}
		value = map[string]string{"error": message}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
