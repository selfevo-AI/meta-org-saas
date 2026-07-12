package project

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/dberrors"
)

type Handler struct {
	service *Service
}

const maxRequirementDocumentBytes = 10 << 20

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/requirements", h.createRequirement)
	r.Get("/requirements", h.listRequirements)
	r.Get("/requirements/{id}", h.getRequirement)
	r.Patch("/requirements/{id}", h.updateRequirement)
	r.Post("/requirements/{id}/documents", h.uploadRequirementDocument)
	r.Get("/requirements/{id}/documents", h.listRequirementDocuments)
	r.Get("/requirement-documents/{id}/download", h.downloadRequirementDocument)
	r.Post("/requirements/{id}/analyze", h.analyzeRequirement)
	r.Post("/requirements/{id}/approve", h.approveRequirement)
	r.Post("/requirements/{id}/convert-to-project", h.convertRequirement)
	r.Post("/requirements/{id}/analysis-workflows", h.startRequirementAnalysisWorkflow)
	r.Get("/requirements/{id}/analysis-workflows", h.listRequirementAnalysisWorkflows)
	r.Post("/requirements/{id}/analysis-workflows/sync", h.syncLatestRequirementAnalysisWorkflow)
	r.Post("/requirements/{id}/analysis-workflows/{workflowID}/sync", h.syncRequirementAnalysisWorkflow)

	r.Post("/projects", h.createProject)
	r.Get("/projects", h.listProjects)
	r.Get("/projects/{id}", h.getProject)
	r.Patch("/projects/{id}", h.updateProject)
	r.Post("/projects/{id}/members", h.addProjectMember)
	r.Get("/projects/{id}/members", h.listProjectMembers)
	r.Post("/projects/{id}/workflows", h.bindProjectWorkflow)
	r.Get("/projects/{id}/workflows", h.listProjectWorkflows)
	r.Post("/projects/{id}/match-actors", h.matchProjectActors)
	r.Post("/projects/{id}/status", h.updateProjectStatus)
	r.Get("/projects/{id}/overview", h.getProjectOverview)
	r.Post("/projects/{id}/ai-analyses", h.analyzeProjectStage)
	r.Get("/projects/{id}/ai-analyses", h.listProjectStageAIRuns)
	r.Post("/projects/{id}/ai-analyses/{runID}/submit-proposal", h.submitProjectAIProposal)

	r.Post("/projects/{id}/deliverables", h.createDeliverable)
	r.Get("/projects/{id}/deliverables", h.listDeliverables)
	r.Patch("/deliverables/{id}", h.updateDeliverable)
	r.Post("/deliverables/{id}/submit", h.submitDeliverable)
	r.Post("/deliverables/{id}/accept", h.acceptDeliverable)
	r.Post("/deliverables/{id}/reject", h.rejectDeliverable)

	r.Post("/projects/{id}/cost-entries", h.createCostEntry)
	r.Get("/projects/{id}/cost-entries", h.listCostEntries)
	r.Get("/projects/{id}/cost-summary", h.getCostSummary)
	r.Post("/projects/{id}/cost-refresh", h.refreshCost)

	r.Post("/projects/{id}/evaluations", h.createProjectEvaluation)
	r.Get("/projects/{id}/evaluations", h.listProjectEvaluations)
	r.Post("/projects/{id}/close-feedback", h.closeFeedback)
}

func (h *Handler) createRequirement(w http.ResponseWriter, r *http.Request) {
	var input CreateRequirementInput
	if !decodeJSON(w, r, &input) {
		return
	}
	req, err := h.service.CreateRequirement(r.Context(), input)
	writeResult(w, http.StatusCreated, req, err)
}

func (h *Handler) listRequirements(w http.ResponseWriter, r *http.Request) {
	requirements, err := h.service.ListRequirements(r.Context(), queryLimit(r))
	writeResult(w, http.StatusOK, requirements, err)
}

func (h *Handler) getRequirement(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseURLID(w, r, "id", "requirements")
	if !ok {
		return
	}
	req, err := h.service.GetRequirement(r.Context(), id)
	writeResult(w, http.StatusOK, req, err)
}

func (h *Handler) updateRequirement(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseURLID(w, r, "id", "requirements")
	if !ok {
		return
	}
	var input UpdateRequirementInput
	if !decodeJSON(w, r, &input) {
		return
	}
	req, err := h.service.UpdateRequirement(r.Context(), id, input)
	writeResult(w, http.StatusOK, req, err)
}

func (h *Handler) uploadRequirementDocument(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseURLID(w, r, "id", "requirements")
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequirementDocumentBytes+(1<<20))
	if err := r.ParseMultipartForm(maxRequirementDocumentBytes); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid multipart form"})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file field is required"})
		return
	}
	defer file.Close()

	content, err := io.ReadAll(io.LimitReader(file, maxRequirementDocumentBytes+1))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "read uploaded file failed"})
		return
	}
	if len(content) > maxRequirementDocumentBytes {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "file exceeds 10MB"})
		return
	}
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	doc, err := h.service.UploadRequirementDocument(r.Context(), id, UploadRequirementDocumentInput{
		FileName:    header.Filename,
		ContentType: contentType,
		SizeBytes:   int64(len(content)),
		Content:     content,
		Metadata:    parseMetadataField(r.FormValue("metadata")),
	})
	writeResult(w, http.StatusCreated, doc, err)
}

func (h *Handler) listRequirementDocuments(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseURLID(w, r, "id", "requirements")
	if !ok {
		return
	}
	documents, err := h.service.ListRequirementDocuments(r.Context(), id)
	writeResult(w, http.StatusOK, documents, err)
}

func (h *Handler) downloadRequirementDocument(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseURLID(w, r, "id", "requirement_documents")
	if !ok {
		return
	}
	doc, err := h.service.GetRequirementDocument(r.Context(), id)
	if err != nil {
		writeResult(w, http.StatusOK, nil, err)
		return
	}
	w.Header().Set("Content-Type", doc.ContentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(doc.Content)))
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", strings.ReplaceAll(doc.FileName, `"`, "")))
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(doc.Content); err != nil {
		log.Printf("download requirement document error: %v", err)
	}
}

func (h *Handler) analyzeRequirement(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseURLID(w, r, "id", "requirements")
	if !ok {
		return
	}
	var input AnalyzeRequirementInput
	if !decodeJSON(w, r, &input) {
		return
	}
	req, err := h.service.AnalyzeRequirement(r.Context(), id, input)
	writeResult(w, http.StatusOK, req, err)
}

func (h *Handler) approveRequirement(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseURLID(w, r, "id", "requirements")
	if !ok {
		return
	}
	var input ActorInput
	if !decodeJSON(w, r, &input) {
		return
	}
	req, err := h.service.ApproveRequirement(r.Context(), id, input)
	writeResult(w, http.StatusOK, req, err)
}

func (h *Handler) convertRequirement(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseURLID(w, r, "id", "requirements")
	if !ok {
		return
	}
	var input ConvertRequirementInput
	if !decodeJSON(w, r, &input) {
		return
	}
	proj, err := h.service.ConvertRequirementToProject(r.Context(), id, input)
	writeResult(w, http.StatusCreated, proj, err)
}

func (h *Handler) startRequirementAnalysisWorkflow(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseURLID(w, r, "id", "requirements")
	if !ok {
		return
	}
	var input StartRequirementAnalysisWorkflowInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.StartRequirementAnalysisWorkflow(r.Context(), id, input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) listRequirementAnalysisWorkflows(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseURLID(w, r, "id", "requirements")
	if !ok {
		return
	}
	workflows, err := h.service.ListRequirementAnalysisWorkflows(r.Context(), id)
	writeResult(w, http.StatusOK, workflows, err)
}

func (h *Handler) syncLatestRequirementAnalysisWorkflow(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseURLID(w, r, "id", "requirements")
	if !ok {
		return
	}
	var input SyncRequirementAnalysisWorkflowInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.SyncRequirementAnalysisWorkflow(r.Context(), id, input)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) syncRequirementAnalysisWorkflow(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseURLID(w, r, "id", "requirements")
	if !ok {
		return
	}
	workflowID, ok := h.parseURLID(w, r, "workflowID", "workflow_instances")
	if !ok {
		return
	}
	var input SyncRequirementAnalysisWorkflowInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.WorkflowID = workflowID
	result, err := h.service.SyncRequirementAnalysisWorkflow(r.Context(), id, input)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) createProject(w http.ResponseWriter, r *http.Request) {
	var input CreateProjectInput
	if !decodeJSON(w, r, &input) {
		return
	}
	proj, err := h.service.CreateProject(r.Context(), input)
	writeResult(w, http.StatusCreated, proj, err)
}

func (h *Handler) listProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := h.service.ListProjects(r.Context(), queryLimit(r))
	writeResult(w, http.StatusOK, projects, err)
}

func (h *Handler) getProject(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseURLID(w, r, "id", "projects")
	if !ok {
		return
	}
	proj, err := h.service.GetProject(r.Context(), id)
	writeResult(w, http.StatusOK, proj, err)
}

func (h *Handler) updateProject(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseURLID(w, r, "id", "projects")
	if !ok {
		return
	}
	var input UpdateProjectInput
	if !decodeJSON(w, r, &input) {
		return
	}
	proj, err := h.service.UpdateProject(r.Context(), id, input)
	writeResult(w, http.StatusOK, proj, err)
}

func (h *Handler) addProjectMember(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseURLID(w, r, "id", "projects")
	if !ok {
		return
	}
	var input AddProjectMemberInput
	if !decodeJSON(w, r, &input) {
		return
	}
	member, err := h.service.AddProjectMember(r.Context(), id, input)
	writeResult(w, http.StatusCreated, member, err)
}

func (h *Handler) listProjectMembers(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseURLID(w, r, "id", "projects")
	if !ok {
		return
	}
	members, err := h.service.ListProjectMembers(r.Context(), id)
	writeResult(w, http.StatusOK, members, err)
}

func (h *Handler) bindProjectWorkflow(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseURLID(w, r, "id", "projects")
	if !ok {
		return
	}
	var input BindProjectWorkflowInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.BindProjectWorkflow(r.Context(), id, input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) listProjectWorkflows(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseURLID(w, r, "id", "projects")
	if !ok {
		return
	}
	workflows, err := h.service.ListProjectWorkflows(r.Context(), id)
	writeResult(w, http.StatusOK, workflows, err)
}

func (h *Handler) matchProjectActors(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseURLID(w, r, "id", "projects")
	if !ok {
		return
	}
	var input MatchProjectActorsInput
	if !decodeJSON(w, r, &input) {
		return
	}
	candidates, err := h.service.MatchProjectActors(r.Context(), id, input)
	writeResult(w, http.StatusOK, candidates, err)
}

func (h *Handler) updateProjectStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseURLID(w, r, "id", "projects")
	if !ok {
		return
	}
	var input UpdateProjectStatusInput
	if !decodeJSON(w, r, &input) {
		return
	}
	proj, err := h.service.UpdateProjectStatus(r.Context(), id, input)
	writeResult(w, http.StatusOK, proj, err)
}

func (h *Handler) getProjectOverview(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseURLID(w, r, "id", "projects")
	if !ok {
		return
	}
	overview, err := h.service.GetProjectOverview(r.Context(), id)
	writeResult(w, http.StatusOK, overview, err)
}

func (h *Handler) createDeliverable(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseURLID(w, r, "id", "projects")
	if !ok {
		return
	}
	var input CreateDeliverableInput
	if !decodeJSON(w, r, &input) {
		return
	}
	deliverable, err := h.service.CreateDeliverable(r.Context(), id, input)
	writeResult(w, http.StatusCreated, deliverable, err)
}

func (h *Handler) listDeliverables(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseURLID(w, r, "id", "projects")
	if !ok {
		return
	}
	deliverables, err := h.service.ListDeliverables(r.Context(), id)
	writeResult(w, http.StatusOK, deliverables, err)
}

func (h *Handler) updateDeliverable(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseURLID(w, r, "id", "deliverables")
	if !ok {
		return
	}
	var input UpdateDeliverableInput
	if !decodeJSON(w, r, &input) {
		return
	}
	deliverable, err := h.service.UpdateDeliverable(r.Context(), id, input)
	writeResult(w, http.StatusOK, deliverable, err)
}

func (h *Handler) submitDeliverable(w http.ResponseWriter, r *http.Request) {
	h.deliverableAction(w, r, h.service.SubmitDeliverable)
}

func (h *Handler) acceptDeliverable(w http.ResponseWriter, r *http.Request) {
	h.deliverableAction(w, r, h.service.AcceptDeliverable)
}

func (h *Handler) rejectDeliverable(w http.ResponseWriter, r *http.Request) {
	h.deliverableAction(w, r, h.service.RejectDeliverable)
}

func (h *Handler) deliverableAction(w http.ResponseWriter, r *http.Request, action func(context.Context, uuid.UUID, DeliverableActionInput) (*Deliverable, error)) {
	id, ok := h.parseURLID(w, r, "id", "deliverables")
	if !ok {
		return
	}
	var input DeliverableActionInput
	if !decodeJSON(w, r, &input) {
		return
	}
	deliverable, err := action(r.Context(), id, input)
	writeResult(w, http.StatusOK, deliverable, err)
}

func (h *Handler) createCostEntry(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseURLID(w, r, "id", "projects")
	if !ok {
		return
	}
	var input CreateCostEntryInput
	if !decodeJSON(w, r, &input) {
		return
	}
	entry, err := h.service.CreateCostEntry(r.Context(), id, input)
	writeResult(w, http.StatusCreated, entry, err)
}

func (h *Handler) listCostEntries(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseURLID(w, r, "id", "projects")
	if !ok {
		return
	}
	entries, err := h.service.ListCostEntries(r.Context(), id)
	writeResult(w, http.StatusOK, entries, err)
}

func (h *Handler) getCostSummary(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseURLID(w, r, "id", "projects")
	if !ok {
		return
	}
	summary, err := h.service.GetCostSummary(r.Context(), id)
	writeResult(w, http.StatusOK, summary, err)
}

func (h *Handler) refreshCost(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseURLID(w, r, "id", "projects")
	if !ok {
		return
	}
	var input ActorInput
	if !decodeJSON(w, r, &input) {
		return
	}
	entries, err := h.service.RefreshCost(r.Context(), id, input)
	writeResult(w, http.StatusCreated, entries, err)
}

func (h *Handler) createProjectEvaluation(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseURLID(w, r, "id", "projects")
	if !ok {
		return
	}
	var input CreateProjectEvaluationInput
	if !decodeJSON(w, r, &input) {
		return
	}
	eval, err := h.service.CreateProjectEvaluation(r.Context(), id, input)
	writeResult(w, http.StatusCreated, eval, err)
}

func (h *Handler) listProjectEvaluations(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseURLID(w, r, "id", "projects")
	if !ok {
		return
	}
	evaluations, err := h.service.ListProjectEvaluations(r.Context(), id)
	writeResult(w, http.StatusOK, evaluations, err)
}

func (h *Handler) closeFeedback(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseURLID(w, r, "id", "projects")
	if !ok {
		return
	}
	var input CloseFeedbackInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CloseFeedback(r.Context(), id, input)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) analyzeProjectStage(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseURLID(w, r, "id", "projects")
	if !ok {
		return
	}
	var input AnalyzeProjectStageInput
	if !decodeJSON(w, r, &input) {
		return
	}
	run, err := h.service.AnalyzeProjectStage(r.Context(), id, input)
	writeResult(w, http.StatusCreated, run, err)
}

func (h *Handler) listProjectStageAIRuns(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseURLID(w, r, "id", "projects")
	if !ok {
		return
	}
	runs, err := h.service.ListProjectStageAIRuns(r.Context(), id, queryLimit(r))
	writeResult(w, http.StatusOK, runs, err)
}

func (h *Handler) submitProjectAIProposal(w http.ResponseWriter, r *http.Request) {
	projectID, ok := h.parseURLID(w, r, "id", "projects")
	if !ok {
		return
	}
	runID, err := uuid.Parse(chi.URLParam(r, "runID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid AI run id"})
		return
	}
	run, err := h.service.SubmitProjectAIProposal(r.Context(), projectID, runID)
	writeResult(w, http.StatusAccepted, run, err)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dest any) bool {
	err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(dest)
	if err == nil || errors.Is(err, io.EOF) {
		return true
	}
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	return false
}

func (h *Handler) parseURLID(w http.ResponseWriter, r *http.Request, param string, sourceTable string) (uuid.UUID, bool) {
	raw := chi.URLParam(r, param)
	id, err := h.service.ResolveLegacyUUID(r.Context(), sourceTable, raw)
	if err != nil {
		writeJSON(w, statusFromError(err), map[string]string{"error": err.Error()})
		return uuid.Nil, false
	}
	if id == uuid.Nil {
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

func parseMetadataField(raw string) map[string]any {
	if raw == "" {
		return map[string]any{}
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(raw), &metadata); err != nil || metadata == nil {
		return map[string]any{}
	}
	return metadata
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
	case errors.Is(err, ErrConflict):
		return http.StatusConflict
	case errors.Is(err, ErrUnavailable):
		return http.StatusServiceUnavailable
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
		log.Printf("writeJSON error: %v", err)
	}
}
