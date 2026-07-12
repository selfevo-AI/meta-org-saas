package identity

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
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
	h.RegisterPublicRoutes(r)
	h.RegisterProtectedRoutes(r)
}

func (h *Handler) RegisterPublicRoutes(r chi.Router) {
	r.Post("/auth/login", h.login)
	r.Post("/auth/register", h.register)
	r.Post("/agents/auth", h.authenticateAgent)
	r.Get("/roles", h.listRoles)
}

func (h *Handler) RegisterProtectedRoutes(r chi.Router) {
	h.RegisterSelfServiceRoutes(r)
	h.RegisterPlatformManagementRoutes(r)
}

func (h *Handler) RegisterSelfServiceRoutes(r chi.Router) {
	r.Post("/auth/me/password", h.changeOwnPassword)
}

func (h *Handler) RegisterPlatformManagementRoutes(r chi.Router) {
	r.Post("/agents/register", h.registerAgent)
	r.Get("/agents", h.listAgents)
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	resp, err := h.service.AuthenticateUser(r.Context(), req.Email, req.Password)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	user, err := h.service.RegisterUser(r.Context(), CreateUserInput{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		status := http.StatusInternalServerError
		message := err.Error()
		if errors.Is(err, ErrValidation) {
			status = http.StatusBadRequest
		} else if errors.Is(err, ErrEmailAlreadyRegistered) || dberrors.IsUniqueViolation(err) {
			status = http.StatusConflict
			message = ErrEmailAlreadyRegistered.Error()
		}
		writeJSON(w, status, map[string]string{"error": message})
		return
	}

	writeJSON(w, http.StatusCreated, user)
}

func (h *Handler) changeOwnPassword(w http.ResponseWriter, r *http.Request) {
	user, ok := currentHumanUser(w, r)
	if !ok {
		return
	}
	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if err := h.service.ChangeOwnPassword(r.Context(), user, req.CurrentPassword, req.NewPassword); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrValidation) {
			status = http.StatusBadRequest
		} else if errors.Is(err, ErrInvalidCredentials) {
			status = http.StatusUnauthorized
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func currentHumanUser(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
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

func (h *Handler) registerAgent(w http.ResponseWriter, r *http.Request) {
	var input CreateAgentInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	resp, err := h.service.RegisterAgent(r.Context(), input)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrValidation) {
			status = http.StatusBadRequest
		} else if dberrors.IsUniqueViolation(err) {
			status = http.StatusConflict
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) authenticateAgent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AgentID string `json:"agent_id"`
		APIKey  string `json:"api_key"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	agentID, err := uuid.Parse(req.AgentID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid agent id"})
		return
	}

	resp, err := h.service.AuthenticateAgent(r.Context(), agentID, req.APIKey)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication failed"})
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) listAgents(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	agents, err := h.service.ListAgents(r.Context(), limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list agents"})
		return
	}
	writeJSON(w, http.StatusOK, agents)
}

func (h *Handler) listRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := h.service.ListRoles(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list roles"})
		return
	}

	writeJSON(w, http.StatusOK, roles)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON error: %v", err)
	}
}
