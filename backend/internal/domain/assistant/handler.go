package assistant

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
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/dberrors"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/assistant/context-targets", h.listContextTargets)
	r.Post("/assistant/context-dictionaries/imports", h.importContextDictionary)
	r.Post("/assistant/context-preview", h.previewContext)
	r.Post("/assistant/sessions", h.createSession)
	r.Get("/assistant/sessions", h.listSessions)
	r.Get("/assistant/sessions/{id}", h.getSession)
	r.Get("/assistant/sessions/{id}/steps", h.listSteps)
	r.Get("/assistant/sessions/{id}/proposals", h.listProposals)
	r.Post("/assistant/sessions/{id}/runs", h.runSession)
	r.Post("/assistant/sessions/{id}/resume", h.resumeSession)
	r.Get("/assistant/skills", h.listBusinessSkills)
	r.Post("/assistant/skills", h.createBusinessSkill)
	r.Post("/assistant/skills/{id}/activate", h.activateBusinessSkill)
	r.Post("/assistant/skills/{id}/run", h.runBusinessSkill)
	r.Post("/assistant/proposals/{id}/confirm", h.confirmProposal)
	r.Post("/assistant/proposals/{id}/reject", h.rejectProposal)
}

func (h *Handler) RegisterPlatformRoutes(r chi.Router) {
	r.Get("/platform/admin/assistant/context-targets", h.listContextTargets)
	r.Post("/platform/admin/assistant/sessions", h.createSession)
	r.Get("/platform/admin/assistant/sessions", h.listSessions)
	r.Get("/platform/admin/assistant/sessions/{id}", h.getSession)
	r.Get("/platform/admin/assistant/sessions/{id}/steps", h.listSteps)
	r.Get("/platform/admin/assistant/sessions/{id}/proposals", h.listProposals)
	r.Post("/platform/admin/assistant/sessions/{id}/runs", h.runSession)
	r.Post("/platform/admin/assistant/sessions/{id}/resume", h.resumeSession)
	r.Get("/platform/admin/assistant/skills", h.listBusinessSkills)
	r.Post("/platform/admin/assistant/skills", h.createBusinessSkill)
	r.Post("/platform/admin/assistant/skills/{id}/activate", h.activateBusinessSkill)
	r.Post("/platform/admin/assistant/skills/{id}/run", h.runBusinessSkill)
	r.Post("/platform/admin/assistant/proposals/{id}/confirm", h.confirmProposal)
	r.Post("/platform/admin/assistant/proposals/{id}/reject", h.rejectProposal)
}

func (h *Handler) listContextTargets(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := authenticatedActor(w, r); !ok {
		return
	}
	result, err := h.service.ListContextTargets(r.Context(), r.URL.Query().Get("module_key"), r.URL.Query().Get("target_type"), queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) importContextDictionary(w http.ResponseWriter, r *http.Request) {
	actorID, _, ok := authenticatedActor(w, r)
	if !ok {
		return
	}
	var input ImportDictionaryInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.ImportDictionary(r.Context(), actorID, input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) previewContext(w http.ResponseWriter, r *http.Request) {
	actorID, actorType, ok := authenticatedActor(w, r)
	if !ok {
		return
	}
	var input ContextRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.PreviewContext(r.Context(), actorID, actorType, input)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) createSession(w http.ResponseWriter, r *http.Request) {
	actorID, actorType, ok := authenticatedActor(w, r)
	if !ok {
		return
	}
	var input CreateSessionInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateSession(r.Context(), actorID, actorType, input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) listSessions(w http.ResponseWriter, r *http.Request) {
	actorID, actorType, ok := authenticatedActor(w, r)
	if !ok {
		return
	}
	result, err := h.service.ListSessions(r.Context(), actorID, actorType, r.URL.Query().Get("module_key"), queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) getSession(w http.ResponseWriter, r *http.Request) {
	actorID, actorType, ok := authenticatedActor(w, r)
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	result, err := h.service.GetSession(r.Context(), id, actorID, actorType)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) listSteps(w http.ResponseWriter, r *http.Request) {
	actorID, actorType, ok := authenticatedActor(w, r)
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	result, err := h.service.ListSteps(r.Context(), id, actorID, actorType, queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) listProposals(w http.ResponseWriter, r *http.Request) {
	actorID, actorType, ok := authenticatedActor(w, r)
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	result, err := h.service.ListProposals(r.Context(), id, actorID, actorType, queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) runSession(w http.ResponseWriter, r *http.Request) {
	actorID, actorType, ok := authenticatedActor(w, r)
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var input RunInput
	if !decodeJSON(w, r, &input) {
		return
	}
	events, err := h.service.Run(r.Context(), id, actorID, actorType, input)
	if err != nil {
		writeJSON(w, statusFromError(err), map[string]string{"error": err.Error()})
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming unsupported"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	writeSSE(w, "lifecycle", map[string]any{"status": StatusRunning, "session_id": id.String()})
	flusher.Flush()
	for event := range events {
		name := event.Type
		if name == "" {
			name = "message"
		}
		writeSSE(w, name, event)
		flusher.Flush()
	}
}

func (h *Handler) resumeSession(w http.ResponseWriter, r *http.Request) {
	actorID, actorType, ok := authenticatedActor(w, r)
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var input ResumeInput
	if !decodeJSON(w, r, &input) {
		return
	}
	events, err := h.service.Resume(r.Context(), id, actorID, actorType, input)
	if err != nil {
		writeJSON(w, statusFromError(err), map[string]string{"error": err.Error()})
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming unsupported"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	writeSSE(w, "lifecycle", map[string]any{"status": StatusRunning, "session_id": id.String(), "resume": true})
	flusher.Flush()
	for event := range events {
		name := event.Type
		if name == "" {
			name = "message"
		}
		writeSSE(w, name, event)
		flusher.Flush()
	}
}

func (h *Handler) confirmProposal(w http.ResponseWriter, r *http.Request) {
	actorID, actorType, ok := authenticatedActor(w, r)
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	result, err := h.service.ConfirmProposal(r.Context(), id, actorID, actorType)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) rejectProposal(w http.ResponseWriter, r *http.Request) {
	actorID, actorType, ok := authenticatedActor(w, r)
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var input struct {
		Reason string `json:"reason"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.RejectProposal(r.Context(), id, actorID, actorType, input.Reason)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) listBusinessSkills(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := authenticatedActor(w, r); !ok {
		return
	}
	result, err := h.service.ListBusinessSkills(r.Context(), r.URL.Query().Get("module_key"), r.URL.Query().Get("target_type"))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) createBusinessSkill(w http.ResponseWriter, r *http.Request) {
	actorID, actorType, ok := authenticatedActor(w, r)
	if !ok {
		return
	}
	var input CreateBusinessSkillInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateBusinessSkill(r.Context(), actorID, actorType, input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) activateBusinessSkill(w http.ResponseWriter, r *http.Request) {
	actorID, actorType, ok := authenticatedActor(w, r)
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	result, err := h.service.ActivateBusinessSkill(r.Context(), id, actorID, actorType)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) runBusinessSkill(w http.ResponseWriter, r *http.Request) {
	actorID, actorType, ok := authenticatedActor(w, r)
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var input map[string]any
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.RunBusinessSkill(r.Context(), id, actorID, actorType, input)
	writeResult(w, http.StatusCreated, result, err)
}

func authenticatedActor(w http.ResponseWriter, r *http.Request) (uuid.UUID, string, bool) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return uuid.Nil, "", false
	}
	id, err := uuid.Parse(user.ID)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid authenticated user"})
		return uuid.Nil, "", false
	}
	return id, normalizeAuthenticatedActorType(user.Type), true
}

func normalizeAuthenticatedActorType(actorType string) string {
	switch actorType {
	case "human", "internal", "internal_human":
		return "internal_human"
	case "external_human":
		return "external_human"
	case "ai", "ai_agent", "agent", "internal_agent":
		return "internal_agent"
	case "external_agent":
		return "external_agent"
	default:
		return actorType
	}
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
	case errors.Is(err, ErrForbidden):
		return http.StatusForbidden
	case dberrors.IsUniqueViolation(err):
		return http.StatusConflict
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
		log.Printf("assistant writeJSON error: %v", err)
	}
}

func writeSSE(w http.ResponseWriter, event string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		data = []byte(`{"error":"encode stream event failed"}`)
		event = "error"
	}
	if _, err := w.Write([]byte("event: " + event + "\n")); err != nil {
		log.Printf("assistant stream write error: %v", err)
		return
	}
	if _, err := w.Write([]byte("data: " + string(data) + "\n\n")); err != nil {
		log.Printf("assistant stream write error: %v", err)
	}
}
