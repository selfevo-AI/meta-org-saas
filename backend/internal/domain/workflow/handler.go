package workflow

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

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
	// Templates define who approves what, and forcing an instance status
	// bypasses that routing; both stay with reviewers.
	reviewer := r.With(middleware.RequireAuthority(middleware.AuthorityReviewer))
	reviewer.Post("/workflows/templates", h.createTemplate)
	r.Get("/workflows/templates", h.listTemplates)
	r.Get("/workflows/templates/{id}", h.getTemplate)
	r.Post("/workflows/instances", h.startWorkflow)
	r.Get("/workflows/instances/{id}", h.getWorkflow)
	reviewer.Patch("/workflows/instances/{id}/status", h.updateStatus)
	r.Patch("/tasks/{id}/status", h.completeTask)
	r.Post("/tasks/{id}/decisions", h.recordDecision)
	r.Get("/tasks/{id}/matrix-assignments", h.listTaskMatrixAssignments)
	r.Post("/tasks/{id}/matrix-assignments", h.createTaskMatrixAssignment)
	r.Patch("/task-matrix-assignments/{id}", h.updateTaskMatrixAssignment)
	r.Delete("/task-matrix-assignments/{id}", h.removeTaskMatrixAssignment)
	r.Get("/workflows/instances/{id}/context", h.getContext)
	r.Put("/workflows/instances/{id}/context", h.updateContext)
}

func (h *Handler) createTemplate(w http.ResponseWriter, r *http.Request) {
	var input CreateWorkflowInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	t, err := h.service.CreateTemplate(r.Context(), input)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrValidation) {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (h *Handler) listTemplates(w http.ResponseWriter, r *http.Request) {
	templates, err := h.service.ListTemplates(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, templates)
}

func (h *Handler) getTemplate(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	t, err := h.service.GetTemplate(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "template not found"})
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h *Handler) startWorkflow(w http.ResponseWriter, r *http.Request) {
	var input StartWorkflowInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	inst, err := h.service.StartWorkflow(r.Context(), input)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, inst)
}

func (h *Handler) getWorkflow(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	inst, err := h.service.GetWorkflow(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "workflow not found"})
		return
	}
	writeJSON(w, http.StatusOK, inst)
}

func (h *Handler) updateStatus(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	status := WorkflowStatus(req.Status)
	if err := h.service.UpdateWorkflowStatus(r.Context(), id, status); err != nil {
		statusCode := http.StatusInternalServerError
		if errors.Is(err, ErrValidation) {
			statusCode = http.StatusBadRequest
		}
		writeJSON(w, statusCode, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *Handler) completeTask(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	var req struct {
		Output map[string]any `json:"output"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if err := h.service.CompleteTask(r.Context(), id, req.Output); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrValidation) {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "task completed"})
}

func (h *Handler) listTaskMatrixAssignments(w http.ResponseWriter, r *http.Request) {
	taskID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid task id"})
		return
	}
	items, err := h.service.ListTaskMatrixAssignments(r.Context(), taskID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) createTaskMatrixAssignment(w http.ResponseWriter, r *http.Request) {
	taskID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid task id"})
		return
	}
	var input CreateTaskMatrixAssignmentInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	item, err := h.service.CreateTaskMatrixAssignment(r.Context(), taskID, input)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrValidation) {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h *Handler) updateTaskMatrixAssignment(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid matrix assignment id"})
		return
	}
	var input UpdateTaskMatrixAssignmentInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	item, err := h.service.UpdateTaskMatrixAssignment(r.Context(), id, input)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrValidation) {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) removeTaskMatrixAssignment(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid matrix assignment id"})
		return
	}
	if err := h.service.RemoveTaskMatrixAssignment(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

func (h *Handler) recordDecision(w http.ResponseWriter, r *http.Request) {
	taskID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid task id"})
		return
	}
	var req struct {
		DecisionMakerID string         `json:"decision_maker_id"`
		MakerType       string         `json:"maker_type"`
		Reasoning       string         `json:"reasoning"`
		Outcome         string         `json:"outcome"`
		Input           map[string]any `json:"input"`
		Output          map[string]any `json:"output"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	makerID, err := uuid.Parse(req.DecisionMakerID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid decision maker id"})
		return
	}
	d, err := h.service.RecordDecision(r.Context(), taskID, makerID, req.MakerType, req.Reasoning, req.Outcome, req.Input, req.Output)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, d)
}

func (h *Handler) getContext(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	wc, err := h.service.GetContext(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "context not found"})
		return
	}
	writeJSON(w, http.StatusOK, wc)
}

func (h *Handler) updateContext(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	var wc WorkflowContext
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&wc); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	wc.WorkflowID = id
	if err := h.service.UpdateContext(r.Context(), &wc); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "context updated"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON error: %v", err)
	}
}
