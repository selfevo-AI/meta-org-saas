package organization

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org/backend/internal/pkg/dberrors"
	"github.com/selfevo-AI/meta-org/backend/internal/pkg/middleware"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/organization/current", h.getCurrentOrganization)
	r.Post("/organizations", h.createOrganization)
	r.Get("/organizations", h.listOrganizations)
	r.Get("/organizations/{id}", h.getOrganization)
	r.Patch("/organizations/{id}", h.updateOrganization)
	r.Post("/organizations/{id}/departments", h.createDepartment)
	r.Get("/organizations/{id}/departments", h.listDepartments)
	r.Get("/organizations/{id}/departments/tree", h.getDepartmentTree)
	r.Get("/organizations/{id}/members", h.listOrganizationMembers)
	r.Get("/organizations/{id}/permission-change-requests", h.listPermissionChangeRequests)
	r.Post("/organizations/{id}/permission-change-requests", h.createPermissionChangeRequest)
	r.Get("/organizations/{id}/permission-rules", h.listAccessRules)
	r.Post("/organizations/{id}/permission-rules", h.createAccessRule)
	r.Get("/departments/{id}", h.getDepartment)
	r.Patch("/departments/{id}", h.updateDepartment)
	r.Post("/departments/{id}/positions", h.createPosition)
	r.Get("/departments/{id}/positions", h.listDepartmentPositions)
	r.Get("/positions/{id}", h.getPosition)
	r.Patch("/positions/{id}", h.updatePosition)
	r.Post("/positions/{id}/assignments", h.createPositionAssignment)
	r.Get("/positions/{id}/assignments", h.listPositionAssignments)
	r.Patch("/position-assignments/{id}", h.updatePositionAssignment)
	r.Delete("/position-assignments/{id}", h.removePositionAssignment)
	r.Post("/departments/{id}/members", h.addOrganizationMember)
	r.Get("/departments/{id}/members", h.listDepartmentMembers)
	r.Post("/departments/{id}/mvru-links", h.linkDepartmentMVRU)
	r.Get("/departments/{id}/mvru-links", h.listDepartmentMVRULinks)
	r.Post("/external-members", h.createExternalMember)
	r.Get("/external-members", h.listExternalMembers)
	r.Get("/external-members/{id}", h.getExternalMember)
	r.Patch("/external-members/{id}", h.updateExternalMember)
	r.Patch("/memberships/{id}", h.updateOrganizationMembership)
	r.Delete("/memberships/{id}", h.removeOrganizationMembership)
	r.Post("/permission-change-requests/{id}/approve", h.approvePermissionChangeRequest)
	r.Post("/permission-change-requests/{id}/reject", h.rejectPermissionChangeRequest)
	r.Post("/organization/match-members", h.matchMembers)
	r.Post("/organization/match-capabilities", h.matchCapabilities)
	r.Post("/muvrs", h.createMVRU)
	r.Get("/muvrs/{id}", h.getMVRU)
	r.Patch("/muvrs/{id}/status", h.updateMVRUStatus)
	r.Post("/muvrs/{id}/members", h.addMember)
	r.Delete("/muvrs/{id}/members", h.removeMember)
	r.Post("/relationships", h.createRelationship)
}

func (h *Handler) createOrganization(w http.ResponseWriter, r *http.Request) {
	var input CreateOrganizationInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if user, ok := middleware.UserFromContext(r.Context()); ok && user.Type == "human" {
		if userID, err := uuid.Parse(user.ID); err == nil {
			input.CreatedBy = &userID
		}
	}
	org, err := h.service.CreateOrganization(r.Context(), input)
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
	writeJSON(w, http.StatusCreated, org)
}

func (h *Handler) getCurrentOrganization(w http.ResponseWriter, r *http.Request) {
	org, err := h.service.GetCurrentOrganization(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	tree, _ := h.service.GetDepartmentTree(r.Context(), org.ID)
	writeJSON(w, http.StatusOK, map[string]any{"organization": org, "departments": tree})
}

func (h *Handler) listOrganizations(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if value := r.URL.Query().Get("limit"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	organizations, err := h.service.ListOrganizations(r.Context(), limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, organizations)
}

func (h *Handler) getOrganization(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	org, err := h.service.GetOrganization(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "organization not found"})
		return
	}
	chart, err := h.service.GetOrgChart(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to fetch org chart"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"organization": org, "chart": chart})
}

func (h *Handler) updateOrganization(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid organization id"})
		return
	}
	var input UpdateOrganizationInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	org, err := h.service.UpdateOrganization(r.Context(), id, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, org)
}

func (h *Handler) createDepartment(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid organization id"})
		return
	}
	var input CreateDepartmentInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	dept, err := h.service.CreateDepartment(r.Context(), orgID, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, dept)
}

func (h *Handler) listDepartments(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid organization id"})
		return
	}
	departments, err := h.service.ListDepartments(r.Context(), orgID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, departments)
}

func (h *Handler) getDepartmentTree(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid organization id"})
		return
	}
	tree, err := h.service.GetDepartmentTree(r.Context(), orgID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, tree)
}

func (h *Handler) getDepartment(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid department id"})
		return
	}
	dept, err := h.service.GetDepartment(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "department not found"})
		return
	}
	writeJSON(w, http.StatusOK, dept)
}

func (h *Handler) updateDepartment(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid department id"})
		return
	}
	var input UpdateDepartmentInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	dept, err := h.service.UpdateDepartment(r.Context(), id, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dept)
}

func (h *Handler) createPosition(w http.ResponseWriter, r *http.Request) {
	departmentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid department id"})
		return
	}
	var input CreatePositionInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	position, err := h.service.CreatePosition(r.Context(), departmentID, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, position)
}

func (h *Handler) listDepartmentPositions(w http.ResponseWriter, r *http.Request) {
	departmentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid department id"})
		return
	}
	dept, err := h.service.GetDepartment(r.Context(), departmentID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "department not found"})
		return
	}
	positions, err := h.service.ListPositions(r.Context(), dept.OrganizationID, &departmentID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, positions)
}

func (h *Handler) getPosition(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid position id"})
		return
	}
	position, err := h.service.GetPosition(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "position not found"})
		return
	}
	writeJSON(w, http.StatusOK, position)
}

func (h *Handler) updatePosition(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid position id"})
		return
	}
	var input UpdatePositionInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	position, err := h.service.UpdatePosition(r.Context(), id, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, position)
}

func (h *Handler) createPositionAssignment(w http.ResponseWriter, r *http.Request) {
	positionID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid position id"})
		return
	}
	var input CreatePositionAssignmentInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	assignment, err := h.service.CreatePositionAssignment(r.Context(), positionID, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, assignment)
}

func (h *Handler) listPositionAssignments(w http.ResponseWriter, r *http.Request) {
	positionID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid position id"})
		return
	}
	assignments, err := h.service.ListPositionAssignments(r.Context(), positionID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, assignments)
}

func (h *Handler) updatePositionAssignment(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid position assignment id"})
		return
	}
	var input UpdatePositionAssignmentInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	assignment, err := h.service.UpdatePositionAssignment(r.Context(), id, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, assignment)
}

func (h *Handler) removePositionAssignment(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid position assignment id"})
		return
	}
	if err := h.service.RemovePositionAssignment(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "position assignment removed"})
}

func (h *Handler) createExternalMember(w http.ResponseWriter, r *http.Request) {
	var input CreateExternalMemberInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	member, err := h.service.CreateExternalMember(r.Context(), input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, member)
}

func (h *Handler) listExternalMembers(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if value := r.URL.Query().Get("limit"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	members, err := h.service.ListExternalMembers(r.Context(), limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, members)
}

func (h *Handler) getExternalMember(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid external member id"})
		return
	}
	member, err := h.service.GetExternalMember(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "external member not found"})
		return
	}
	writeJSON(w, http.StatusOK, member)
}

func (h *Handler) updateExternalMember(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid external member id"})
		return
	}
	var input UpdateExternalMemberInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	member, err := h.service.UpdateExternalMember(r.Context(), id, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, member)
}

func (h *Handler) addOrganizationMember(w http.ResponseWriter, r *http.Request) {
	departmentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid department id"})
		return
	}
	var input AddOrganizationMemberInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	membership, err := h.service.AddOrganizationMember(r.Context(), departmentID, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, membership)
}

func (h *Handler) listDepartmentMembers(w http.ResponseWriter, r *http.Request) {
	departmentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid department id"})
		return
	}
	dept, err := h.service.GetDepartment(r.Context(), departmentID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "department not found"})
		return
	}
	members, err := h.service.ListOrganizationMemberships(r.Context(), dept.OrganizationID, &departmentID, memberTypesFromQuery(r))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, members)
}

func (h *Handler) listOrganizationMembers(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid organization id"})
		return
	}
	var departmentID *uuid.UUID
	if value := r.URL.Query().Get("department_id"); value != "" {
		parsed, err := uuid.Parse(value)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid department_id"})
			return
		}
		departmentID = &parsed
	}
	members, err := h.service.ListOrganizationMemberships(r.Context(), orgID, departmentID, memberTypesFromQuery(r))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, members)
}

func (h *Handler) updateOrganizationMembership(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid membership id"})
		return
	}
	var input UpdateOrganizationMembershipInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	membership, err := h.service.UpdateOrganizationMembership(r.Context(), id, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, membership)
}

func (h *Handler) removeOrganizationMembership(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid membership id"})
		return
	}
	if err := h.service.RemoveOrganizationMembership(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "member removed"})
}

func (h *Handler) createPermissionChangeRequest(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid organization id"})
		return
	}
	var input CreatePermissionChangeRequestInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	request, err := h.service.CreatePermissionChangeRequest(r.Context(), orgID, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, request)
}

func (h *Handler) listPermissionChangeRequests(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid organization id"})
		return
	}
	requests, err := h.service.ListPermissionChangeRequests(r.Context(), orgID, r.URL.Query().Get("status"), queryLimit(r, 50))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, requests)
}

func (h *Handler) approvePermissionChangeRequest(w http.ResponseWriter, r *http.Request) {
	h.reviewPermissionChangeRequest(w, r, PermissionChangeApproved)
}

func (h *Handler) rejectPermissionChangeRequest(w http.ResponseWriter, r *http.Request) {
	h.reviewPermissionChangeRequest(w, r, PermissionChangeRejected)
}

func (h *Handler) reviewPermissionChangeRequest(w http.ResponseWriter, r *http.Request, decision string) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid permission change request id"})
		return
	}
	var input ReviewPermissionChangeRequestInput
	if r.Body != nil {
		_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input)
	}
	input.Decision = decision
	request, err := h.service.ReviewPermissionChangeRequest(r.Context(), id, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, request)
}

func (h *Handler) createAccessRule(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid organization id"})
		return
	}
	var input CreateAccessRuleInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	rule, err := h.service.CreateAccessRule(r.Context(), orgID, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, rule)
}

func (h *Handler) listAccessRules(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid organization id"})
		return
	}
	rules, err := h.service.ListAccessRules(r.Context(), orgID, r.URL.Query().Get("scope_type"), r.URL.Query().Get("resource_type"), queryLimit(r, 50))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rules)
}

func (h *Handler) linkDepartmentMVRU(w http.ResponseWriter, r *http.Request) {
	departmentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid department id"})
		return
	}
	var input LinkDepartmentMVRUInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	link, err := h.service.LinkDepartmentMVRU(r.Context(), departmentID, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, link)
}

func (h *Handler) listDepartmentMVRULinks(w http.ResponseWriter, r *http.Request) {
	departmentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid department id"})
		return
	}
	links, err := h.service.ListDepartmentMVRULinks(r.Context(), departmentID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, links)
}

func (h *Handler) matchMembers(w http.ResponseWriter, r *http.Request) {
	var input MatchMembersInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	candidates, err := h.service.MatchMembers(r.Context(), input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, candidates)
}

func (h *Handler) matchCapabilities(w http.ResponseWriter, r *http.Request) {
	var input MatchCapabilitiesInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	bridge, err := h.service.MatchCapabilities(r.Context(), input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, bridge)
}

func (h *Handler) createMVRU(w http.ResponseWriter, r *http.Request) {
	var input CreateMVRUInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	mvru, err := h.service.CreateMVRU(r.Context(), input)
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
	writeJSON(w, http.StatusCreated, mvru)
}

func (h *Handler) getMVRU(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	mvru, err := h.service.GetMVRU(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "mvru not found"})
		return
	}
	writeJSON(w, http.StatusOK, mvru)
}

func (h *Handler) updateMVRUStatus(w http.ResponseWriter, r *http.Request) {
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
	switch req.Status {
	case "active":
		err = h.service.ActivateMVRU(r.Context(), id)
	case "evaluating":
		err = h.service.EvaluateMVRU(r.Context(), id)
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid status"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *Handler) addMember(w http.ResponseWriter, r *http.Request) {
	mvruID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid mvru id"})
		return
	}
	var req struct {
		UserID  *string `json:"user_id"`
		AgentID *string `json:"agent_id"`
		RoleID  string  `json:"role_id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	roleUUID, err := uuid.Parse(req.RoleID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid role id"})
		return
	}
	var userUUID, agentUUID *uuid.UUID
	if req.UserID != nil {
		u, err := uuid.Parse(*req.UserID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user id"})
			return
		}
		userUUID = &u
	}
	if req.AgentID != nil {
		a, err := uuid.Parse(*req.AgentID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid agent id"})
			return
		}
		agentUUID = &a
	}
	if err := h.service.AddMember(r.Context(), mvruID, roleUUID, userUUID, agentUUID); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrValidation) {
			status = http.StatusBadRequest
		} else if dberrors.IsUniqueViolation(err) {
			status = http.StatusConflict
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "member added"})
}

func (h *Handler) removeMember(w http.ResponseWriter, r *http.Request) {
	mvruID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid mvru id"})
		return
	}
	var req struct {
		UserID  *string `json:"user_id"`
		AgentID *string `json:"agent_id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	var userUUID, agentUUID *uuid.UUID
	if req.UserID != nil {
		u, err := uuid.Parse(*req.UserID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user id"})
			return
		}
		userUUID = &u
	}
	if req.AgentID != nil {
		a, err := uuid.Parse(*req.AgentID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid agent id"})
			return
		}
		agentUUID = &a
	}
	if err := h.service.RemoveMember(r.Context(), mvruID, userUUID, agentUUID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "member removed"})
}

func (h *Handler) createRelationship(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SourceMVRUID string         `json:"source_mvru_id"`
		TargetMVRUID string         `json:"target_mvru_id"`
		RelType      string         `json:"rel_type"`
		Config       map[string]any `json:"config,omitempty"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	src, err := uuid.Parse(req.SourceMVRUID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid source mvru id"})
		return
	}
	tgt, err := uuid.Parse(req.TargetMVRUID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid target mvru id"})
		return
	}
	if req.Config == nil {
		req.Config = map[string]any{}
	}
	rel, err := h.service.CreateRelationship(r.Context(), src, tgt, req.RelType, req.Config)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, rel)
}

func memberTypesFromQuery(r *http.Request) []string {
	values := r.URL.Query()["member_type"]
	var memberTypes []string
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				memberTypes = append(memberTypes, part)
			}
		}
	}
	return memberTypes
}

func queryLimit(r *http.Request, fallback int) int {
	limit := fallback
	if value := r.URL.Query().Get("limit"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	return limit
}

func writeServiceError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if errors.Is(err, ErrValidation) {
		status = http.StatusBadRequest
	} else if errors.Is(err, ErrForbidden) {
		status = http.StatusForbidden
	} else if dberrors.IsUniqueViolation(err) {
		status = http.StatusConflict
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON error: %v", err)
	}
}
