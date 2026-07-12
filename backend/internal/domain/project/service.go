package project

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/aigateway"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/businessai"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/costing"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/evolution"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/governance"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/metaresource"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/organization"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/workflow"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
)

var (
	ErrNotFound    = errors.New("not found")
	ErrValidation  = errors.New("validation error")
	ErrForbidden   = errors.New("forbidden")
	ErrConflict    = errors.New("conflict")
	ErrUnavailable = errors.New("unavailable")
)

type Service struct {
	repo         *Repository
	governance   *governance.Service
	evolution    *evolution.Service
	organization *organization.Service
	workflow     *workflow.Service
	metaResource *metaresource.Service
	costRecorder CostRecorder
	businessAI   *businessai.Service
}

type ServiceOption func(*Service)

type CostRecorder interface {
	RecordActual(ctx context.Context, input costing.CreateLedgerEntryInput) (*costing.CostLedgerEntry, error)
}

func WithGovernanceService(gov *governance.Service) ServiceOption {
	return func(s *Service) {
		s.governance = gov
	}
}

func WithEvolutionService(evo *evolution.Service) ServiceOption {
	return func(s *Service) {
		s.evolution = evo
	}
}

func WithOrganizationService(org *organization.Service) ServiceOption {
	return func(s *Service) {
		s.organization = org
	}
}

func WithWorkflowService(wf *workflow.Service) ServiceOption {
	return func(s *Service) {
		s.workflow = wf
	}
}

func WithMetaResourceService(meta *metaresource.Service) ServiceOption {
	return func(s *Service) {
		s.metaResource = meta
	}
}

func WithCostRecorder(recorder CostRecorder) ServiceOption {
	return func(s *Service) {
		s.costRecorder = recorder
	}
}

func WithBusinessAI(service *businessai.Service) ServiceOption {
	return func(s *Service) {
		s.businessAI = service
	}
}

func NewService(repo *Repository, opts ...ServiceOption) *Service {
	s := &Service{repo: repo}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Service) ResolveLegacyUUID(ctx context.Context, sourceTable string, key string) (uuid.UUID, error) {
	return s.repo.ResolveLegacyUUID(ctx, sourceTable, key)
}

func (s *Service) CreateRequirement(ctx context.Context, input CreateRequirementInput) (*Requirement, error) {
	if input.Title == "" {
		return nil, fmt.Errorf("%w: title is required", ErrValidation)
	}
	normalizeRequirementInput(&input)
	if input.OrganizationID == nil {
		input.OrganizationID = currentTenantOrganizationID(ctx)
	}
	actorID, actorType, err := s.resolveActor(ctx, ActorInput{ActorID: input.CreatedByID, ActorType: input.CreatedByType})
	if err != nil {
		return nil, err
	}
	input.CreatedByID = &actorID
	input.CreatedByType = actorType
	if err := s.requireAccess(ctx, actorID, actorType, "requirement.create", "requirement", nil, input.OrganizationID, input.DepartmentID, nil, input.RequiredLevel, input.RiskLevel, nil); err != nil {
		return nil, err
	}
	req, err := s.repo.CreateRequirement(ctx, input)
	if err != nil {
		return nil, err
	}
	req = s.ensureRequirementPDCA(ctx, req, actorID, actorType)
	s.recordRequirementPDCA(ctx, req, metaresource.StagePlan, "requirement_created", &req.ID, actorID, actorType, map[string]any{
		"title":          req.Title,
		"priority":       req.Priority,
		"risk_level":     req.RiskLevel,
		"required_level": req.RequiredLevel,
	}, "Requirement captured", "Analyze requirement")
	return req, nil
}

func (s *Service) ListRequirements(ctx context.Context, limit int) ([]Requirement, error) {
	var requirements []Requirement
	var err error
	if orgID := currentTenantOrganizationID(ctx); orgID != nil {
		requirements, err = s.repo.ListRequirementsByOrganization(ctx, *orgID, limit)
	} else {
		requirements, err = s.repo.ListRequirements(ctx, limit)
	}
	if requirements == nil {
		requirements = []Requirement{}
	}
	return requirements, err
}

func (s *Service) GetRequirement(ctx context.Context, id uuid.UUID) (*Requirement, error) {
	req, err := s.repo.GetRequirement(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := ensureTenantAccess(ctx, req.OrganizationID); err != nil {
		return nil, err
	}
	return req, nil
}

func (s *Service) UploadRequirementDocument(ctx context.Context, requirementID uuid.UUID, input UploadRequirementDocumentInput) (*RequirementDocument, error) {
	req, err := s.repo.GetRequirement(ctx, requirementID)
	if err != nil {
		return nil, err
	}
	actorID, actorType, err := s.resolveActor(ctx, input.ActorInput)
	if err != nil {
		return nil, err
	}
	if !isHumanActor(actorType) {
		return nil, fmt.Errorf("%w: only human employees can upload requirement documents", ErrForbidden)
	}
	if input.FileName == "" || len(input.Content) == 0 {
		return nil, fmt.Errorf("%w: file is required", ErrValidation)
	}
	if input.ContentType == "" {
		input.ContentType = "application/octet-stream"
	}
	input.SizeBytes = int64(len(input.Content))
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
	if err := s.requireAccess(ctx, actorID, actorType, "requirement.document.upload", "requirement", &requirementID, req.OrganizationID, req.DepartmentID, nil, req.RequiredLevel, req.RiskLevel, nil); err != nil {
		return nil, err
	}
	return s.repo.CreateRequirementDocument(ctx, requirementID, input, &actorID, actorType)
}

func (s *Service) ListRequirementDocuments(ctx context.Context, requirementID uuid.UUID) ([]RequirementDocument, error) {
	req, err := s.repo.GetRequirement(ctx, requirementID)
	if err != nil {
		return nil, err
	}
	if err := ensureTenantAccess(ctx, req.OrganizationID); err != nil {
		return nil, err
	}
	documents, err := s.repo.ListRequirementDocuments(ctx, requirementID)
	if documents == nil {
		documents = []RequirementDocument{}
	}
	return documents, err
}

func (s *Service) GetRequirementDocument(ctx context.Context, id uuid.UUID) (*RequirementDocumentContent, error) {
	doc, err := s.repo.GetRequirementDocument(ctx, id)
	if err != nil {
		return nil, err
	}
	req, err := s.repo.GetRequirement(ctx, doc.RequirementID)
	if err != nil {
		return nil, err
	}
	if err := ensureTenantAccess(ctx, req.OrganizationID); err != nil {
		return nil, err
	}
	return doc, nil
}

func (s *Service) StartRequirementAnalysisWorkflow(ctx context.Context, requirementID uuid.UUID, input StartRequirementAnalysisWorkflowInput) (*RequirementAnalysisWorkflow, error) {
	req, err := s.repo.GetRequirement(ctx, requirementID)
	if err != nil {
		return nil, err
	}
	if s.workflow == nil {
		return nil, fmt.Errorf("%w: workflow service is unavailable", ErrValidation)
	}
	if input.WorkflowTemplateID == uuid.Nil {
		return nil, fmt.Errorf("%w: workflow_template_id is required", ErrValidation)
	}
	actorID, actorType, err := s.resolveActor(ctx, input.ActorInput)
	if err != nil {
		return nil, err
	}
	if !isHumanActor(actorType) {
		return nil, fmt.Errorf("%w: only human employees can start requirement analysis workflows", ErrForbidden)
	}
	if err := s.requireAccess(ctx, actorID, actorType, "requirement.workflow.start", "requirement", &requirementID, req.OrganizationID, req.DepartmentID, nil, minLevel(req.RequiredLevel, "L2"), req.RiskLevel, nil); err != nil {
		return nil, err
	}
	if input.Purpose == "" {
		input.Purpose = "requirement_analysis"
	}
	if input.Context == nil {
		input.Context = map[string]any{}
	}
	documents, _ := s.ListRequirementDocuments(ctx, requirementID)
	input.Context["requirement_id"] = req.ID.String()
	input.Context["requirement_title"] = req.Title
	input.Context["requirement_description"] = req.Description
	input.Context["requirement_priority"] = req.Priority
	input.Context["requirement_risk_level"] = req.RiskLevel
	input.Context["requirement_required_level"] = req.RequiredLevel
	input.Context["requirement_documents"] = documents
	input.Context["analysis_result_contract"] = map[string]any{
		"generated_requirement": map[string]any{
			"title":          "string",
			"description":    "string",
			"priority":       "low|medium|high|critical",
			"risk_level":     "low|medium|high|critical",
			"required_level": "L1|L2|L3|L4",
			"analysis":       "object",
		},
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
	input.Metadata["started_by_id"] = actorID.String()
	input.Metadata["started_by_type"] = actorType

	inst, err := s.workflow.StartWorkflow(ctx, workflow.StartWorkflowInput{
		TemplateID:     input.WorkflowTemplateID,
		OrganizationID: req.OrganizationID,
		DepartmentID:   req.DepartmentID,
		Context:        input.Context,
	})
	if err != nil {
		return nil, err
	}
	analysis, err := s.repo.CreateRequirementAnalysisWorkflow(ctx, requirementID, inst.ID, input)
	if err != nil {
		return nil, err
	}
	_, _ = s.repo.UpdateRequirement(ctx, requirementID, UpdateRequirementInput{
		Metadata: mergeMetadata(req.Metadata, map[string]any{
			"active_analysis_workflow_id": inst.ID.String(),
			"analysis_workflow_status":    "active",
		}),
	})
	return analysis, nil
}

func (s *Service) ListRequirementAnalysisWorkflows(ctx context.Context, requirementID uuid.UUID) ([]RequirementAnalysisWorkflow, error) {
	workflows, err := s.repo.ListRequirementAnalysisWorkflows(ctx, requirementID)
	if workflows == nil {
		workflows = []RequirementAnalysisWorkflow{}
	}
	return workflows, err
}

func (s *Service) SyncRequirementAnalysisWorkflow(ctx context.Context, requirementID uuid.UUID, input SyncRequirementAnalysisWorkflowInput) (map[string]any, error) {
	req, err := s.repo.GetRequirement(ctx, requirementID)
	if err != nil {
		return nil, err
	}
	if s.workflow == nil {
		return nil, fmt.Errorf("%w: workflow service is unavailable", ErrValidation)
	}
	actorID, actorType, err := s.resolveActor(ctx, input.ActorInput)
	if err != nil {
		return nil, err
	}
	if err := s.requireAccess(ctx, actorID, actorType, "requirement.workflow.sync", "requirement", &requirementID, req.OrganizationID, req.DepartmentID, nil, req.RequiredLevel, req.RiskLevel, nil); err != nil {
		return nil, err
	}
	if err := validateRequirementStatus(req.Status, "sync_analysis"); err != nil {
		return nil, err
	}

	workflowID := input.WorkflowID
	if workflowID == uuid.Nil {
		workflows, err := s.ListRequirementAnalysisWorkflows(ctx, requirementID)
		if err != nil {
			return nil, err
		}
		if len(workflows) == 0 {
			return nil, fmt.Errorf("%w: no analysis workflow found", ErrNotFound)
		}
		workflowID = workflows[0].WorkflowID
	}
	link, err := s.repo.GetRequirementAnalysisWorkflow(ctx, requirementID, workflowID)
	if err != nil {
		return nil, err
	}
	inst, err := s.workflow.GetWorkflow(ctx, workflowID)
	if err != nil {
		return nil, err
	}
	if inst.Status != workflow.WorkflowCompleted {
		_, _ = s.repo.UpdateRequirementAnalysisWorkflow(ctx, link.ID, string(inst.Status), nil, map[string]any{
			"last_sync_attempt": "workflow_not_completed",
		})
		return nil, fmt.Errorf("%w: workflow is %s", ErrConflict, inst.Status)
	}

	result := s.requirementAnalysisResult(ctx, inst)
	generated := generatedRequirementFromResult(result)
	update := UpdateRequirementInput{
		Status:   "analyzed",
		Analysis: mergeMetadata(req.Analysis, result),
		Metadata: mergeMetadata(req.Metadata, map[string]any{
			"active_analysis_workflow_id": workflowID.String(),
			"analysis_workflow_status":    "completed",
		}),
	}
	if title := stringFromMap(generated, "title"); title != "" {
		update.Title = title
	}
	if description := stringFromMap(generated, "description"); description != "" {
		update.Description = description
	}
	if priority := stringFromMap(generated, "priority"); priority != "" {
		update.Priority = normalizePriority(priority)
	}
	if riskLevel := stringFromMap(generated, "risk_level"); riskLevel != "" {
		update.RiskLevel = normalizeRisk(riskLevel)
	}
	if requiredLevel := stringFromMap(generated, "required_level"); requiredLevel != "" {
		update.RequiredLevel = normalizeLevel(requiredLevel)
	}
	if analysis, ok := mapFromAny(generated["analysis"]); ok {
		update.Analysis = mergeMetadata(update.Analysis, analysis)
	}

	updatedRequirement, err := s.repo.UpdateRequirement(ctx, requirementID, update)
	if err != nil {
		return nil, err
	}
	analysisWorkflow, err := s.repo.UpdateRequirementAnalysisWorkflow(ctx, link.ID, "completed", result, map[string]any{
		"synced_requirement_id": updatedRequirement.ID.String(),
	})
	if err != nil {
		return nil, err
	}
	s.recordRequirementPDCA(ctx, updatedRequirement, metaresource.StagePlan, "requirement_analyzed", &updatedRequirement.ID, actorID, actorType, result, "Requirement analysis workflow completed", "Approve requirement")
	return map[string]any{
		"requirement":       updatedRequirement,
		"analysis_workflow": analysisWorkflow,
		"workflow":          inst,
	}, nil
}

func (s *Service) UpdateRequirement(ctx context.Context, id uuid.UUID, input UpdateRequirementInput) (*Requirement, error) {
	current, err := s.repo.GetRequirement(ctx, id)
	if err != nil {
		return nil, err
	}
	actorID, actorType, err := s.resolveActor(ctx, ActorInput{})
	if err != nil {
		return nil, err
	}
	if err := s.requireAccess(ctx, actorID, actorType, "requirement.update", "requirement", &id, current.OrganizationID, current.DepartmentID, nil, current.RequiredLevel, current.RiskLevel, nil); err != nil {
		return nil, err
	}
	normalizeRequirementUpdate(&input)
	return s.repo.UpdateRequirement(ctx, id, input)
}

func (s *Service) AnalyzeRequirement(ctx context.Context, id uuid.UUID, input AnalyzeRequirementInput) (*Requirement, error) {
	req, err := s.repo.GetRequirement(ctx, id)
	if err != nil {
		return nil, err
	}
	actorID, actorType, err := s.resolveActor(ctx, input.ActorInput)
	if err != nil {
		return nil, err
	}
	if err := s.requireAccess(ctx, actorID, actorType, "requirement.analyze", "requirement", &id, req.OrganizationID, req.DepartmentID, nil, req.RequiredLevel, req.RiskLevel, nil); err != nil {
		return nil, err
	}
	if err := validateRequirementStatus(req.Status, "analyze"); err != nil {
		return nil, err
	}
	analysis := buildRequirementAnalysis(req, input.Notes)
	updated, err := s.repo.UpdateRequirement(ctx, id, UpdateRequirementInput{
		Status:   "analyzed",
		Analysis: analysis,
	})
	if err != nil {
		return nil, err
	}
	s.recordRequirementPDCA(ctx, updated, metaresource.StagePlan, "requirement_analyzed", &updated.ID, actorID, actorType, analysis, "Requirement analyzed", "Approve requirement")
	return updated, nil
}

func (s *Service) ApproveRequirement(ctx context.Context, id uuid.UUID, input ActorInput) (*Requirement, error) {
	req, err := s.repo.GetRequirement(ctx, id)
	if err != nil {
		return nil, err
	}
	actorID, actorType, err := s.resolveActor(ctx, input)
	if err != nil {
		return nil, err
	}
	if err := s.requireAccess(ctx, actorID, actorType, "requirement.approve", "requirement", &id, req.OrganizationID, req.DepartmentID, nil, minLevel(req.RequiredLevel, "L2"), req.RiskLevel, nil); err != nil {
		return nil, err
	}
	if err := validateRequirementStatus(req.Status, "approve"); err != nil {
		return nil, err
	}
	updated, err := s.repo.UpdateRequirement(ctx, id, UpdateRequirementInput{Status: "approved"})
	if err != nil {
		return nil, err
	}
	s.recordRequirementPDCA(ctx, updated, metaresource.StagePlan, "requirement_approved", &updated.ID, actorID, actorType, map[string]any{
		"approved_by": actorID.String(),
	}, "Requirement approved", "Convert requirement to project")
	return updated, nil
}

func (s *Service) ConvertRequirementToProject(ctx context.Context, id uuid.UUID, input ConvertRequirementInput) (*Project, error) {
	req, err := s.repo.GetRequirement(ctx, id)
	if err != nil {
		return nil, err
	}
	actorID, actorType, err := s.resolveActor(ctx, input.ActorInput)
	if err != nil {
		return nil, err
	}
	if err := s.requireAccess(ctx, actorID, actorType, "project.create", "project", nil, req.OrganizationID, req.DepartmentID, nil, req.RequiredLevel, req.RiskLevel, nil); err != nil {
		return nil, err
	}
	if existing, err := s.repo.GetProjectByRequirement(ctx, id); err == nil && existing != nil {
		return existing, nil
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	name := input.Name
	if name == "" {
		name = req.Title
	}
	description := input.Description
	if description == "" {
		description = req.Description
	}
	metadata := input.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["converted_from_requirement"] = req.ID.String()
	if demandID := stringMetadata(req.Metadata, "demand_profile_id"); demandID != "" {
		metadata["demand_profile_id"] = demandID
	}
	if cycleID := stringMetadata(req.Metadata, "pdca_cycle_id"); cycleID != "" {
		metadata["pdca_cycle_id"] = cycleID
	}
	budgetAmount := input.BudgetAmount
	if budgetAmount == 0 {
		budgetAmount = req.BudgetAmount
	}
	budgetCurrency := input.BudgetCurrency
	if budgetCurrency == "" {
		budgetCurrency = req.BudgetCurrency
	}
	proj, err := s.repo.CreateProject(ctx, CreateProjectInput{
		RequirementID:  &req.ID,
		OrganizationID: req.OrganizationID,
		DepartmentID:   req.DepartmentID,
		Name:           name,
		Description:    description,
		Status:         "planning",
		Priority:       req.Priority,
		RiskLevel:      req.RiskLevel,
		RequiredLevel:  req.RequiredLevel,
		BudgetAmount:   budgetAmount,
		BudgetCurrency: budgetCurrency,
		Metadata:       metadata,
	})
	if err != nil {
		return nil, err
	}
	updatedReq, _ := s.repo.UpdateRequirement(ctx, id, UpdateRequirementInput{
		Status: "converted",
		Metadata: mergeMetadata(req.Metadata, map[string]any{
			"project_id": proj.ID.String(),
		}),
	})
	if updatedReq == nil {
		updatedReq = req
	}
	s.recordRequirementPDCA(ctx, updatedReq, metaresource.StagePlan, "requirement_converted", &proj.ID, actorID, actorType, map[string]any{
		"project_id": proj.ID.String(),
	}, "Requirement converted to project", "Plan project delivery")
	return proj, nil
}

func (s *Service) CreateProject(ctx context.Context, input CreateProjectInput) (*Project, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrValidation)
	}
	normalizeProjectInput(&input)
	actorID, actorType, err := s.resolveActor(ctx, input.ActorInput)
	if err != nil {
		return nil, err
	}
	if input.RequirementID != nil {
		if req, err := s.repo.GetRequirement(ctx, *input.RequirementID); err == nil {
			if input.OrganizationID == nil {
				input.OrganizationID = req.OrganizationID
			}
			if input.DepartmentID == nil {
				input.DepartmentID = req.DepartmentID
			}
			if input.Description == "" {
				input.Description = req.Description
			}
			if input.BudgetAmount == 0 {
				input.BudgetAmount = req.BudgetAmount
			}
			if input.BudgetCurrency == "" {
				input.BudgetCurrency = req.BudgetCurrency
			}
			if input.Metadata == nil {
				input.Metadata = map[string]any{}
			}
			if demandID := stringMetadata(req.Metadata, "demand_profile_id"); demandID != "" {
				input.Metadata["demand_profile_id"] = demandID
			}
			if cycleID := stringMetadata(req.Metadata, "pdca_cycle_id"); cycleID != "" {
				input.Metadata["pdca_cycle_id"] = cycleID
			}
		}
	}
	if input.OrganizationID == nil {
		input.OrganizationID = currentTenantOrganizationID(ctx)
	}
	if err := s.requireAccess(ctx, actorID, actorType, "project.create", "project", nil, input.OrganizationID, input.DepartmentID, nil, input.RequiredLevel, input.RiskLevel, nil); err != nil {
		return nil, err
	}
	proj, err := s.repo.CreateProject(ctx, input)
	if err != nil {
		return nil, err
	}
	s.recordProjectPDCA(ctx, proj, metaresource.StagePlan, "project_created", &proj.ID, actorID, actorType, map[string]any{
		"status": proj.Status,
	}, "Project created", "Assign members and bind workflow")
	return proj, nil
}

func (s *Service) ListProjects(ctx context.Context, limit int) ([]Project, error) {
	var projects []Project
	var err error
	if orgID := currentTenantOrganizationID(ctx); orgID != nil {
		projects, err = s.repo.ListProjectsByOrganization(ctx, *orgID, limit)
	} else {
		projects, err = s.repo.ListProjects(ctx, limit)
	}
	if projects == nil {
		projects = []Project{}
	}
	return projects, err
}

func (s *Service) GetProject(ctx context.Context, id uuid.UUID) (*Project, error) {
	proj, err := s.repo.GetProject(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := ensureTenantAccess(ctx, proj.OrganizationID); err != nil {
		return nil, err
	}
	return proj, nil
}

func (s *Service) UpdateProject(ctx context.Context, id uuid.UUID, input UpdateProjectInput) (*Project, error) {
	current, err := s.repo.GetProject(ctx, id)
	if err != nil {
		return nil, err
	}
	actorID, actorType, err := s.resolveActor(ctx, ActorInput{})
	if err != nil {
		return nil, err
	}
	if err := s.requireAccess(ctx, actorID, actorType, "project.update", "project", &id, current.OrganizationID, current.DepartmentID, nil, current.RequiredLevel, current.RiskLevel, nil); err != nil {
		return nil, err
	}
	normalizeProjectUpdate(&input)
	return s.repo.UpdateProject(ctx, id, input)
}

func (s *Service) AddProjectMember(ctx context.Context, projectID uuid.UUID, input AddProjectMemberInput) (*ProjectMember, error) {
	proj, err := s.repo.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	actorID, actorType, err := s.resolveActor(ctx, input.ActorInput)
	if err != nil {
		return nil, err
	}
	if input.MemberActorID == uuid.Nil || input.MemberActorType == "" {
		return nil, fmt.Errorf("%w: member_actor_id and member_actor_type are required", ErrValidation)
	}
	input.ProjectID = projectID
	normalizeProjectMemberInput(&input)
	if err := s.requireAccess(ctx, actorID, actorType, "project.assign", "project", &projectID, proj.OrganizationID, proj.DepartmentID, nil, minLevel(input.PermissionLevel, proj.RequiredLevel), proj.RiskLevel, nil); err != nil {
		return nil, err
	}
	member, err := s.repo.AddProjectMember(ctx, input)
	if err != nil {
		return nil, err
	}
	s.recordProjectPDCA(ctx, proj, metaresource.StageDo, "project_member_added", &member.ID, actorID, actorType, map[string]any{
		"member_actor_id":   member.ActorID.String(),
		"member_actor_type": member.ActorType,
		"role":              member.Role,
	}, "Project member assigned", "Bind or execute workflow")
	return member, nil
}

func (s *Service) ListProjectMembers(ctx context.Context, projectID uuid.UUID) ([]ProjectMember, error) {
	proj, err := s.repo.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if err := ensureTenantAccess(ctx, proj.OrganizationID); err != nil {
		return nil, err
	}
	members, err := s.repo.ListProjectMembers(ctx, projectID)
	if members == nil {
		members = []ProjectMember{}
	}
	return members, err
}

func (s *Service) BindProjectWorkflow(ctx context.Context, projectID uuid.UUID, input BindProjectWorkflowInput) (*ProjectWorkflow, error) {
	proj, err := s.repo.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	actorID, actorType, err := s.resolveActor(ctx, input.ActorInput)
	if err != nil {
		return nil, err
	}
	if input.WorkflowID == nil && input.WorkflowTemplateID == nil {
		return nil, fmt.Errorf("%w: workflow_id or workflow_template_id is required", ErrValidation)
	}
	if err := s.requireAccess(ctx, actorID, actorType, "project.workflow.bind", "project", &projectID, proj.OrganizationID, proj.DepartmentID, nil, proj.RequiredLevel, proj.RiskLevel, nil); err != nil {
		return nil, err
	}
	normalizeProjectWorkflowInput(&input)
	workflowID := uuid.Nil
	if input.WorkflowID != nil {
		workflowID = *input.WorkflowID
	} else if s.workflow != nil && input.WorkflowTemplateID != nil {
		instance, err := s.workflow.StartWorkflow(ctx, workflow.StartWorkflowInput{
			TemplateID:     *input.WorkflowTemplateID,
			OrganizationID: proj.OrganizationID,
			DepartmentID:   proj.DepartmentID,
			ProjectID:      &projectID,
			Context: map[string]any{
				"project_id":        projectID.String(),
				"project_name":      proj.Name,
				"requirement_id":    uuidString(proj.RequirementID),
				"organization_id":   uuidString(proj.OrganizationID),
				"department_id":     uuidString(proj.DepartmentID),
				"required_level":    proj.RequiredLevel,
				"risk_level":        proj.RiskLevel,
				"lifecycle_purpose": input.Purpose,
			},
		})
		if err != nil {
			return nil, err
		}
		workflowID = instance.ID
	}
	if workflowID == uuid.Nil {
		return nil, fmt.Errorf("%w: workflow_id is required when workflow service is unavailable", ErrValidation)
	}
	projectWorkflow, err := s.repo.BindProjectWorkflow(ctx, input, projectID, workflowID)
	if err != nil {
		return nil, err
	}
	s.recordProjectPDCA(ctx, proj, metaresource.StageDo, "project_workflow_bound", &projectWorkflow.ID, actorID, actorType, map[string]any{
		"workflow_id": workflowID.String(),
		"purpose":     projectWorkflow.Purpose,
	}, "Project workflow bound", "Start delivery execution")
	return projectWorkflow, nil
}

func (s *Service) ListProjectWorkflows(ctx context.Context, projectID uuid.UUID) ([]ProjectWorkflow, error) {
	proj, err := s.repo.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if err := ensureTenantAccess(ctx, proj.OrganizationID); err != nil {
		return nil, err
	}
	workflows, err := s.repo.ListProjectWorkflows(ctx, projectID)
	if workflows == nil {
		workflows = []ProjectWorkflow{}
	}
	return workflows, err
}

func (s *Service) MatchProjectActors(ctx context.Context, projectID uuid.UUID, input MatchProjectActorsInput) ([]organization.MemberMatchCandidate, error) {
	proj, err := s.repo.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if err := ensureTenantAccess(ctx, proj.OrganizationID); err != nil {
		return nil, err
	}
	if s.organization == nil {
		return []organization.MemberMatchCandidate{}, nil
	}
	if proj.OrganizationID == nil {
		return nil, fmt.Errorf("%w: project has no organization_id", ErrValidation)
	}
	if input.TaskDescription == "" {
		input.TaskDescription = proj.Description
	}
	if input.RequiredLevel == "" {
		input.RequiredLevel = proj.RequiredLevel
	}
	if input.RiskLevel == "" {
		input.RiskLevel = proj.RiskLevel
	}
	return s.organization.MatchMembers(ctx, organization.MatchMembersInput{
		OrganizationID:       *proj.OrganizationID,
		DepartmentID:         proj.DepartmentID,
		TaskDescription:      input.TaskDescription,
		WorkflowTemplateID:   input.WorkflowTemplateID,
		RequiredCapabilities: input.RequiredCapabilities,
		RequiredLevel:        input.RequiredLevel,
		RiskLevel:            input.RiskLevel,
		MemberTypes:          input.MemberTypes,
	})
}

func (s *Service) UpdateProjectStatus(ctx context.Context, projectID uuid.UUID, input UpdateProjectStatusInput) (*Project, error) {
	proj, err := s.repo.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	actorID, actorType, err := s.resolveActor(ctx, input.ActorInput)
	if err != nil {
		return nil, err
	}
	if !isValidProjectStatus(input.Status) {
		return nil, fmt.Errorf("%w: invalid project status", ErrValidation)
	}
	if err := s.validateProjectStatusChange(ctx, proj, input.Status); err != nil {
		return nil, err
	}
	if err := s.requireAccess(ctx, actorID, actorType, "project.status", "project", &projectID, proj.OrganizationID, proj.DepartmentID, nil, proj.RequiredLevel, proj.RiskLevel, nil); err != nil {
		return nil, err
	}
	metadata := mergeMetadata(proj.Metadata, map[string]any{"last_status_note": input.Note})
	updated, err := s.repo.UpdateProject(ctx, projectID, UpdateProjectInput{Status: input.Status, Metadata: metadata})
	if err != nil {
		return nil, err
	}
	stage, eventType, nextAction := projectStatusPDCA(input.Status)
	s.recordProjectPDCA(ctx, updated, stage, eventType, &updated.ID, actorID, actorType, map[string]any{
		"from_status": proj.Status,
		"to_status":   input.Status,
		"note":        input.Note,
	}, "Project status updated", nextAction)
	return updated, nil
}

func (s *Service) GetProjectOverview(ctx context.Context, projectID uuid.UUID) (*ProjectOverview, error) {
	proj, err := s.repo.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if err := ensureTenantAccess(ctx, proj.OrganizationID); err != nil {
		return nil, err
	}
	var req *Requirement
	if proj.RequirementID != nil {
		req, _ = s.repo.GetRequirement(ctx, *proj.RequirementID)
	}
	members, err := s.ListProjectMembers(ctx, projectID)
	if err != nil {
		return nil, err
	}
	workflows, err := s.ListProjectWorkflows(ctx, projectID)
	if err != nil {
		return nil, err
	}
	deliverables, err := s.ListDeliverables(ctx, projectID)
	if err != nil {
		return nil, err
	}
	costSummary, err := s.GetCostSummary(ctx, projectID)
	if err != nil {
		return nil, err
	}
	evaluations, err := s.ListProjectEvaluations(ctx, projectID)
	if err != nil {
		return nil, err
	}
	s.syncProjectWorkflowStatuses(ctx, workflows)
	return &ProjectOverview{
		Project:      proj,
		Requirement:  req,
		Members:      members,
		Workflows:    workflows,
		Deliverables: deliverables,
		CostSummary:  costSummary,
		Evaluations:  evaluations,
		Lifecycle:    buildProjectLifecycle(proj, req, members, workflows, deliverables, costSummary, evaluations),
	}, nil
}

func (s *Service) AnalyzeProjectStage(ctx context.Context, projectID uuid.UUID, input AnalyzeProjectStageInput) (*businessai.Run, error) {
	if s.businessAI == nil {
		return nil, fmt.Errorf("business stage ai is not configured")
	}
	proj, err := s.repo.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if proj.OrganizationID == nil {
		return nil, fmt.Errorf("%w: project organization is required", ErrValidation)
	}
	actorID, actorType, err := s.resolveActor(ctx, input.ActorInput)
	if err != nil {
		return nil, err
	}
	if err := s.requireAccess(ctx, actorID, actorType, "project.ai.analyze", "project", &projectID,
		proj.OrganizationID, proj.DepartmentID, nil, proj.RequiredLevel, proj.RiskLevel, nil); err != nil {
		return nil, err
	}
	overview, err := s.GetProjectOverview(ctx, projectID)
	if err != nil {
		return nil, err
	}
	verifiedContext := map[string]any{"project_overview": overview}
	if input.Context != nil {
		verifiedContext["request_context"] = input.Context
	}
	run, err := s.businessAI.Analyze(ctx, businessai.AnalyzeInput{
		OrganizationID:  *proj.OrganizationID,
		ProjectID:       projectID,
		RequirementID:   proj.RequirementID,
		Stage:           input.Stage,
		RequestedByID:   &actorID,
		RequestedByType: actorType,
		ProviderType:    input.ProviderType,
		Model:           input.Model,
		Focus:           input.Focus,
		Context:         verifiedContext,
	})
	if errors.Is(err, businessai.ErrValidation) {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	if errors.Is(err, businessai.ErrNotConfigured) || errors.Is(err, aigateway.ErrUnavailable) {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	return run, err
}

func (s *Service) ListProjectStageAIRuns(ctx context.Context, projectID uuid.UUID, limit int) ([]businessai.Run, error) {
	if s.businessAI == nil {
		return nil, fmt.Errorf("business stage ai is not configured")
	}
	proj, err := s.repo.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if proj.OrganizationID == nil {
		return nil, fmt.Errorf("%w: project organization is required", ErrValidation)
	}
	if err := ensureTenantAccess(ctx, proj.OrganizationID); err != nil {
		return nil, err
	}
	return s.businessAI.ListRuns(ctx, *proj.OrganizationID, projectID, limit)
}

func (s *Service) CreateDeliverable(ctx context.Context, projectID uuid.UUID, input CreateDeliverableInput) (*Deliverable, error) {
	proj, err := s.repo.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	actorID, actorType, err := s.resolveActor(ctx, input.ActorInput)
	if err != nil {
		return nil, err
	}
	normalizeDeliverableInput(&input)
	if err := validateProjectCanWriteDeliverable(proj.Status); err != nil {
		return nil, err
	}
	if input.Status != "draft" && input.Status != "submitted" {
		return nil, fmt.Errorf("%w: deliverable can only be created as draft or submitted", ErrValidation)
	}
	action := "deliverable.write"
	if input.Status == "submitted" {
		action = "deliverable.submit"
	}
	if err := s.requireAccess(ctx, actorID, actorType, action, "project", &projectID, proj.OrganizationID, proj.DepartmentID, nil, proj.RequiredLevel, proj.RiskLevel, nil); err != nil {
		return nil, err
	}
	deliverable, err := s.repo.CreateDeliverable(ctx, projectID, input, &actorID, actorType)
	if err != nil {
		return nil, err
	}
	stage := metaresource.StageDo
	eventType := "deliverable_created"
	nextAction := "Submit deliverable"
	if deliverable.Status == "submitted" {
		eventType = "deliverable_submitted"
		nextAction = "Accept or reject deliverable"
	}
	s.recordProjectPDCA(ctx, proj, stage, eventType, &deliverable.ID, actorID, actorType, map[string]any{
		"deliverable_status": deliverable.Status,
		"deliverable_type":   deliverable.DeliverableType,
	}, "Deliverable updated", nextAction)
	return deliverable, nil
}

func (s *Service) ListDeliverables(ctx context.Context, projectID uuid.UUID) ([]Deliverable, error) {
	proj, err := s.repo.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if err := ensureTenantAccess(ctx, proj.OrganizationID); err != nil {
		return nil, err
	}
	deliverables, err := s.repo.ListDeliverables(ctx, projectID)
	if deliverables == nil {
		deliverables = []Deliverable{}
	}
	return deliverables, err
}

func (s *Service) UpdateDeliverable(ctx context.Context, id uuid.UUID, input UpdateDeliverableInput) (*Deliverable, error) {
	deliverable, err := s.repo.GetDeliverable(ctx, id)
	if err != nil {
		return nil, err
	}
	proj, err := s.repo.GetProject(ctx, deliverable.ProjectID)
	if err != nil {
		return nil, err
	}
	actorID, actorType, err := s.resolveActor(ctx, ActorInput{})
	if err != nil {
		return nil, err
	}
	if err := s.requireAccess(ctx, actorID, actorType, "deliverable.write", "deliverable", &id, proj.OrganizationID, proj.DepartmentID, nil, proj.RequiredLevel, proj.RiskLevel, nil); err != nil {
		return nil, err
	}
	return s.repo.UpdateDeliverable(ctx, id, input)
}

func (s *Service) SubmitDeliverable(ctx context.Context, id uuid.UUID, input DeliverableActionInput) (*Deliverable, error) {
	return s.changeDeliverableStatus(ctx, id, input, "submitted", "deliverable.submit")
}

func (s *Service) AcceptDeliverable(ctx context.Context, id uuid.UUID, input DeliverableActionInput) (*Deliverable, error) {
	return s.changeDeliverableStatus(ctx, id, input, "accepted", "deliverable.accept")
}

func (s *Service) RejectDeliverable(ctx context.Context, id uuid.UUID, input DeliverableActionInput) (*Deliverable, error) {
	if input.Evidence == nil {
		input.Evidence = map[string]any{}
	}
	input.Evidence["reject_reason"] = input.Reason
	return s.changeDeliverableStatus(ctx, id, input, "rejected", "deliverable.accept")
}

func (s *Service) CreateCostEntry(ctx context.Context, projectID uuid.UUID, input CreateCostEntryInput) (*CostEntry, error) {
	proj, err := s.repo.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	actorID, actorType, err := s.resolveActor(ctx, input.ActorInput)
	if err != nil {
		return nil, err
	}
	if input.Currency == "" {
		input.Currency = proj.BudgetCurrency
	}
	normalizeCostEntryInput(&input)
	if input.Amount <= 0 {
		return nil, fmt.Errorf("%w: amount must be greater than zero", ErrValidation)
	}
	if err := s.requireAccess(ctx, actorID, actorType, "cost.write", "project", &projectID, proj.OrganizationID, proj.DepartmentID, nil, proj.RequiredLevel, proj.RiskLevel, nil); err != nil {
		return nil, err
	}
	entry, err := s.repo.CreateCostEntry(ctx, projectID, input)
	if err != nil {
		return nil, err
	}
	s.recordCostEntry(ctx, proj, entry)
	s.recordProjectPDCA(ctx, proj, metaresource.StageDo, "cost_recorded", &entry.ID, actorID, actorType, map[string]any{
		"source_type": entry.SourceType,
		"amount":      entry.Amount,
		"currency":    entry.Currency,
	}, "Project cost recorded", "Review cost and delivery evidence")
	return entry, nil
}

func (s *Service) CreateCostEntryFromAIUsage(ctx context.Context, projectID uuid.UUID, input CreateCostEntryInput) (*CostEntry, error) {
	if err := prepareAIUsageCostEntryInput(&input); err != nil {
		return nil, err
	}
	existing, err := s.repo.GetCostEntryBySource(ctx, "ai_usage", *input.SourceID)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	entry, err := s.CreateCostEntry(ctx, projectID, input)
	if err == nil {
		return entry, nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return s.repo.GetCostEntryBySource(ctx, "ai_usage", *input.SourceID)
	}
	return nil, err
}

func (s *Service) ListCostEntries(ctx context.Context, projectID uuid.UUID) ([]CostEntry, error) {
	proj, err := s.repo.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if err := ensureTenantAccess(ctx, proj.OrganizationID); err != nil {
		return nil, err
	}
	entries, err := s.repo.ListCostEntries(ctx, projectID)
	if entries == nil {
		entries = []CostEntry{}
	}
	return entries, err
}

func (s *Service) GetCostSummary(ctx context.Context, projectID uuid.UUID) (*CostSummary, error) {
	proj, err := s.repo.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if err := ensureTenantAccess(ctx, proj.OrganizationID); err != nil {
		return nil, err
	}
	return s.repo.GetCostSummary(ctx, projectID)
}

func (s *Service) RefreshCost(ctx context.Context, projectID uuid.UUID, actorInput ActorInput) ([]CostEntry, error) {
	proj, err := s.repo.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	actorID, actorType, err := s.resolveActor(ctx, actorInput)
	if err != nil {
		return nil, err
	}
	if err := s.requireAccess(ctx, actorID, actorType, "cost.write", "project", &projectID, proj.OrganizationID, proj.DepartmentID, nil, proj.RequiredLevel, proj.RiskLevel, nil); err != nil {
		return nil, err
	}
	members, err := s.ListProjectMembers(ctx, projectID)
	if err != nil {
		return nil, err
	}
	entries := []CostEntry{}
	refreshPeriod := time.Now().UTC().Format("2006-01-02")
	for _, member := range members {
		if member.CostRate <= 0 || member.AllocationPercent <= 0 || member.Status == "archived" {
			continue
		}
		if existing, err := s.repo.GetMemberAllocationCostEntry(ctx, projectID, member.ID, refreshPeriod); err == nil && existing != nil {
			entries = append(entries, *existing)
			continue
		} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		amount := member.CostRate * (member.AllocationPercent / 100)
		entry, err := s.repo.CreateCostEntry(ctx, projectID, CreateCostEntryInput{
			SourceType:     "member_allocation",
			EntryActorID:   &member.ActorID,
			EntryActorType: member.ActorType,
			Amount:         amount,
			Currency:       proj.BudgetCurrency,
			Description:    "member allocation cost snapshot",
			Metadata: map[string]any{
				"project_member_id":  member.ID.String(),
				"allocation_percent": member.AllocationPercent,
				"cost_rate":          member.CostRate,
				"refresh_period":     refreshPeriod,
				"refresh_actor_id":   actorID.String(),
				"refresh_actor_type": actorType,
			},
		})
		if err != nil {
			return nil, err
		}
		s.recordCostEntry(ctx, proj, entry)
		s.recordProjectPDCA(ctx, proj, metaresource.StageDo, "cost_refreshed", &entry.ID, actorID, actorType, map[string]any{
			"project_member_id": member.ID.String(),
			"amount":            entry.Amount,
			"refresh_period":    refreshPeriod,
		}, "Member allocation cost refreshed", "Review project cost")
		entries = append(entries, *entry)
	}
	return entries, nil
}

func (s *Service) recordCostEntry(ctx context.Context, project *Project, entry *CostEntry) {
	if s.costRecorder == nil || project == nil || entry == nil || entry.Amount == 0 {
		return
	}
	category := "manual"
	switch entry.SourceType {
	case "member_allocation":
		category = "human"
	case "ai_usage":
		category = "model_token"
	case "resource":
		category = "resource"
	case "capability":
		category = "capability"
	}
	metadata := map[string]any{
		"project_cost_entry_id": entry.ID.String(),
		"source_type":           entry.SourceType,
	}
	for key, value := range entry.Metadata {
		metadata[key] = value
	}
	_, _ = s.costRecorder.RecordActual(ctx, costing.CreateLedgerEntryInput{
		CostCategory:   category,
		SourceType:     "project_cost_entry",
		SourceID:       &entry.ID,
		OrganizationID: project.OrganizationID,
		DepartmentID:   project.DepartmentID,
		ProjectID:      &project.ID,
		ActorID:        entry.ActorID,
		ActorType:      entry.ActorType,
		Amount:         entry.Amount,
		Currency:       entry.Currency,
		OccurredAt:     &entry.OccurredAt,
		Description:    entry.Description,
		Metadata:       metadata,
	})
}

func (s *Service) CreateProjectEvaluation(ctx context.Context, projectID uuid.UUID, input CreateProjectEvaluationInput) (*ProjectEvaluation, error) {
	proj, err := s.repo.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	actorID, actorType, err := s.resolveActor(ctx, input.ActorInput)
	if err != nil {
		return nil, err
	}
	if err := s.requireAccess(ctx, actorID, actorType, "evaluation.create", "project", &projectID, proj.OrganizationID, proj.DepartmentID, input.CapabilityID, proj.RequiredLevel, proj.RiskLevel, nil); err != nil {
		return nil, err
	}
	normalizeProjectEvaluationInput(&input)
	overall := projectEvaluationOverall(input)
	eval, err := s.repo.CreateProjectEvaluation(ctx, projectID, input, &actorID, actorType, overall)
	if err != nil {
		return nil, err
	}
	s.recordEvaluationOutcome(ctx, proj, eval)
	s.recordProjectPDCA(ctx, proj, metaresource.StageAccept, "project_evaluated", &eval.ID, actorID, actorType, map[string]any{
		"overall_score": eval.OverallScore,
		"conclusion":    eval.Conclusion,
	}, "Project evaluated", "Close feedback")
	return eval, nil
}

func (s *Service) ListProjectEvaluations(ctx context.Context, projectID uuid.UUID) ([]ProjectEvaluation, error) {
	proj, err := s.repo.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if err := ensureTenantAccess(ctx, proj.OrganizationID); err != nil {
		return nil, err
	}
	evaluations, err := s.repo.ListProjectEvaluations(ctx, projectID)
	if evaluations == nil {
		evaluations = []ProjectEvaluation{}
	}
	return evaluations, err
}

func (s *Service) CloseFeedback(ctx context.Context, projectID uuid.UUID, input CloseFeedbackInput) (map[string]any, error) {
	proj, err := s.repo.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	actorID, actorType, err := s.resolveActor(ctx, input.ActorInput)
	if err != nil {
		return nil, err
	}
	if err := s.requireAccess(ctx, actorID, actorType, "evaluation.create", "project", &projectID, proj.OrganizationID, proj.DepartmentID, nil, proj.RequiredLevel, proj.RiskLevel, nil); err != nil {
		return nil, err
	}
	if proj.Status != "completed" && proj.Status != "closed" {
		return nil, fmt.Errorf("%w: project must be completed before closing feedback", ErrConflict)
	}
	outcomeScore := clampScore(input.OutcomeScore)
	if input.OutcomeScore == 0 {
		outcomeScore = 0.75
	}
	updated := 0
	evaluations, err := s.ListProjectEvaluations(ctx, projectID)
	if err != nil {
		return nil, err
	}
	for _, eval := range evaluations {
		if eval.ActorID == nil || eval.ActorType == "" {
			continue
		}
		score := eval.OverallScore
		if input.OutcomeScore > 0 {
			score = outcomeScore
		}
		s.recordOutcome(ctx, proj, *eval.ActorID, eval.ActorType, eval.CapabilityID, score, map[string]any{
			"project_evaluation_id": eval.ID.String(),
			"source":                "project_close_feedback",
		})
		updated++
	}
	if updated == 0 {
		members, _ := s.ListProjectMembers(ctx, projectID)
		for _, member := range members {
			s.recordOutcome(ctx, proj, member.ActorID, member.ActorType, nil, outcomeScore, map[string]any{
				"project_member_id": member.ID.String(),
				"source":            "project_close_feedback",
			})
			updated++
		}
	}
	if updated == 0 {
		return nil, fmt.Errorf("%w: feedback requires at least one project evaluation or member", ErrConflict)
	}
	updatedProject, err := s.repo.UpdateProject(ctx, projectID, UpdateProjectInput{
		Status:   "closed",
		Metadata: mergeMetadata(proj.Metadata, map[string]any{"close_feedback": input.Conclusion}),
	})
	if err != nil {
		return nil, err
	}
	s.recordProjectPDCA(ctx, updatedProject, metaresource.StageAccept, "project_feedback_closed", &updatedProject.ID, actorID, actorType, map[string]any{
		"outcome_score": outcomeScore,
		"conclusion":    input.Conclusion,
	}, "Project feedback closed", "Use learning in the next PDCA cycle")
	return map[string]any{
		"project":        updatedProject,
		"outcome_score":  outcomeScore,
		"updated_actors": updated,
	}, nil
}

func (s *Service) changeDeliverableStatus(ctx context.Context, id uuid.UUID, input DeliverableActionInput, status string, action string) (*Deliverable, error) {
	deliverable, err := s.repo.GetDeliverable(ctx, id)
	if err != nil {
		return nil, err
	}
	proj, err := s.repo.GetProject(ctx, deliverable.ProjectID)
	if err != nil {
		return nil, err
	}
	if err := validateDeliverableStatusChange(deliverable.Status, status); err != nil {
		return nil, err
	}
	actorID, actorType, err := s.resolveActor(ctx, input.ActorInput)
	if err != nil {
		return nil, err
	}
	if err := s.requireAccess(ctx, actorID, actorType, action, "deliverable", &id, proj.OrganizationID, proj.DepartmentID, nil, proj.RequiredLevel, proj.RiskLevel, nil); err != nil {
		return nil, err
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
	if input.Reason != "" {
		input.Metadata["reason"] = input.Reason
	}
	updated, err := s.repo.UpdateDeliverableStatus(ctx, id, status, &actorID, actorType, input.Evidence, input.Metadata)
	if err != nil {
		return nil, err
	}
	stage := metaresource.StageDo
	eventType := "deliverable_submitted"
	nextAction := "Accept or reject deliverable"
	if status == "accepted" {
		stage = metaresource.StageAccept
		eventType = "deliverable_accepted"
		nextAction = "Complete project"
	} else if status == "rejected" {
		stage = metaresource.StageChange
		eventType = "deliverable_rejected"
		nextAction = "Revise and resubmit deliverable"
	}
	s.recordProjectPDCA(ctx, proj, stage, eventType, &updated.ID, actorID, actorType, map[string]any{
		"from_status": deliverable.Status,
		"to_status":   status,
		"reason":      input.Reason,
	}, "Deliverable status changed", nextAction)
	return updated, nil
}

func (s *Service) resolveActor(ctx context.Context, input ActorInput) (uuid.UUID, string, error) {
	if input.ActorID != nil && *input.ActorID != uuid.Nil {
		actorType := input.ActorType
		if actorType == "" {
			actorType = "internal_human"
		}
		return *input.ActorID, normalizeActorType(actorType), nil
	}
	user, ok := middleware.UserFromContext(ctx)
	if !ok {
		return uuid.Nil, "", fmt.Errorf("%w: actor_id is required", ErrValidation)
	}
	actorID, err := uuid.Parse(user.ID)
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("%w: invalid authenticated actor", ErrValidation)
	}
	return actorID, normalizeAuthActorType(user.Type), nil
}

func (s *Service) requireAccess(ctx context.Context, actorID uuid.UUID, actorType string, action string, resource string, resourceID *uuid.UUID, organizationID *uuid.UUID, departmentID *uuid.UUID, capabilityID *uuid.UUID, requiredLevel string, riskLevel string, weightSnapshot *float64) error {
	if err := ensureTenantAccess(ctx, organizationID); err != nil {
		return err
	}
	if s.governance == nil {
		return nil
	}
	decision, err := s.governance.DecideAccess(ctx, governance.AccessDecisionInput{
		ActorID:        actorID,
		ActorType:      actorType,
		Action:         action,
		Resource:       resource,
		ResourceID:     resourceID,
		OrganizationID: organizationID,
		DepartmentID:   departmentID,
		CapabilityID:   capabilityID,
		RequiredLevel:  normalizeLevel(requiredLevel),
		RiskLevel:      normalizeRisk(riskLevel),
		WeightSnapshot: weightSnapshot,
		Context: map[string]any{
			"domain": "project_lifecycle",
		},
	})
	if err != nil {
		return err
	}
	if !decision.Allowed {
		return fmt.Errorf("%w: %s", ErrForbidden, decision.Reason)
	}
	return nil
}

func currentTenantOrganizationID(ctx context.Context) *uuid.UUID {
	tenant, ok := middleware.TenantFromContext(ctx)
	if !ok || tenant.OrganizationID == nil {
		return nil
	}
	id := *tenant.OrganizationID
	return &id
}

func ensureTenantAccess(ctx context.Context, organizationID *uuid.UUID) error {
	tenant, ok := middleware.TenantFromContext(ctx)
	if !ok || tenant.OrganizationID == nil || organizationID == nil {
		return nil
	}
	if *tenant.OrganizationID != *organizationID {
		return fmt.Errorf("%w: resource is outside current organization", ErrForbidden)
	}
	return nil
}

func (s *Service) recordEvaluationOutcome(ctx context.Context, proj *Project, eval *ProjectEvaluation) {
	if eval.ActorID == nil || eval.ActorType == "" {
		return
	}
	s.recordOutcome(ctx, proj, *eval.ActorID, eval.ActorType, eval.CapabilityID, eval.OverallScore, map[string]any{
		"project_evaluation_id": eval.ID.String(),
		"source":                "project_evaluation",
	})
}

func (s *Service) recordOutcome(ctx context.Context, proj *Project, actorID uuid.UUID, actorType string, capabilityID *uuid.UUID, outcomeScore float64, extra map[string]any) {
	if s.evolution == nil {
		return
	}
	context := map[string]any{
		"project_id":      proj.ID.String(),
		"project_name":    proj.Name,
		"requirement_id":  uuidString(proj.RequirementID),
		"required_level":  proj.RequiredLevel,
		"lifecycle_stage": "feedback",
	}
	for key, value := range extra {
		context[key] = value
	}
	_, _ = s.evolution.RecordContextOutcome(ctx, evolution.ContextOutcomeInput{
		ActorID:      actorID,
		ActorType:    actorType,
		OutcomeScore: clampScore(outcomeScore),
		Scope: evolution.ContextWeightScope{
			OrganizationID: proj.OrganizationID,
			DepartmentID:   proj.DepartmentID,
			TaskType:       "project_delivery",
			CapabilityID:   capabilityID,
			RiskLevel:      normalizeRisk(proj.RiskLevel),
			Context:        context,
		},
	})
}

func normalizeRequirementInput(input *CreateRequirementInput) {
	if input.Source == "" {
		input.Source = "manual"
	}
	input.Priority = normalizePriority(input.Priority)
	input.RiskLevel = normalizeRisk(input.RiskLevel)
	input.RequiredLevel = normalizeLevel(input.RequiredLevel)
	if input.BudgetCurrency == "" {
		input.BudgetCurrency = "CNY"
	}
	if input.Analysis == nil {
		input.Analysis = map[string]any{}
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
}

func normalizeRequirementUpdate(input *UpdateRequirementInput) {
	if input.Priority != "" {
		input.Priority = normalizePriority(input.Priority)
	}
	if input.RiskLevel != "" {
		input.RiskLevel = normalizeRisk(input.RiskLevel)
	}
	if input.RequiredLevel != "" {
		input.RequiredLevel = normalizeLevel(input.RequiredLevel)
	}
	if input.BudgetCurrency != nil && *input.BudgetCurrency == "" {
		input.BudgetCurrency = nil
	}
}

func normalizeProjectInput(input *CreateProjectInput) {
	if input.Status == "" {
		input.Status = "planning"
	}
	if input.Priority == "" {
		input.Priority = "medium"
	}
	input.Priority = normalizePriority(input.Priority)
	input.RiskLevel = normalizeRisk(input.RiskLevel)
	input.RequiredLevel = normalizeLevel(input.RequiredLevel)
	if input.BudgetCurrency == "" {
		input.BudgetCurrency = "CNY"
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
}

func normalizeProjectUpdate(input *UpdateProjectInput) {
	if input.Priority != "" {
		input.Priority = normalizePriority(input.Priority)
	}
	if input.RiskLevel != "" {
		input.RiskLevel = normalizeRisk(input.RiskLevel)
	}
	if input.RequiredLevel != "" {
		input.RequiredLevel = normalizeLevel(input.RequiredLevel)
	}
}

func normalizeProjectMemberInput(input *AddProjectMemberInput) {
	if input.Role == "" {
		input.Role = "contributor"
	}
	if input.AllocationPercent <= 0 {
		input.AllocationPercent = 100
	}
	if input.AllocationPercent > 100 {
		input.AllocationPercent = 100
	}
	input.PermissionLevel = normalizeLevel(input.PermissionLevel)
	if input.Status == "" {
		input.Status = "active"
	}
	if input.Capabilities == nil {
		input.Capabilities = []string{}
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
	input.MemberActorType = normalizeActorType(input.MemberActorType)
}

func normalizeProjectWorkflowInput(input *BindProjectWorkflowInput) {
	if input.Purpose == "" {
		input.Purpose = "delivery"
	}
	if input.Status == "" {
		input.Status = "active"
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
}

func normalizeDeliverableInput(input *CreateDeliverableInput) {
	if input.DeliverableType == "" {
		input.DeliverableType = "artifact"
	}
	if input.Version == "" {
		input.Version = "1.0"
	}
	if input.Status == "" {
		input.Status = "draft"
	}
	if input.Evidence == nil {
		input.Evidence = map[string]any{}
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
}

func normalizeCostEntryInput(input *CreateCostEntryInput) {
	if input.SourceType == "" {
		input.SourceType = "manual"
	}
	if input.Currency == "" {
		input.Currency = "CNY"
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
	input.EntryActorType = normalizeActorType(input.EntryActorType)
}

func prepareAIUsageCostEntryInput(input *CreateCostEntryInput) error {
	if input.SourceID == nil || *input.SourceID == uuid.Nil {
		return fmt.Errorf("%w: source_id is required for ai_usage cost entries", ErrValidation)
	}
	input.SourceType = "ai_usage"
	if input.Currency == "" {
		input.Currency = "CNY"
	}
	if input.Description == "" {
		input.Description = "AI usage cost"
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
	input.Metadata["source_type"] = "ai_usage"
	input.Metadata["ai_usage_ledger_id"] = input.SourceID.String()
	return nil
}

func normalizeProjectEvaluationInput(input *CreateProjectEvaluationInput) {
	input.EvaluatedActorType = normalizeActorType(input.EvaluatedActorType)
	input.QualityScore = clampScore(input.QualityScore)
	input.DeliveryScore = clampScore(input.DeliveryScore)
	input.CostScore = clampScore(input.CostScore)
	input.CollaborationScore = clampScore(input.CollaborationScore)
	if input.Evidence == nil {
		input.Evidence = map[string]any{}
	}
}

func buildRequirementAnalysis(req *Requirement, notes string) map[string]any {
	text := strings.ToLower(req.Title + " " + req.Description + " " + notes)
	capabilities := []string{}
	for _, keyword := range []string{"analysis", "review", "delivery", "integration", "compliance", "finance", "data", "workflow"} {
		if strings.Contains(text, keyword) {
			capabilities = append(capabilities, keyword)
		}
	}
	if len(capabilities) == 0 {
		capabilities = append(capabilities, "planning", "delivery", "review")
	}
	return map[string]any{
		"suggested_capabilities": capabilities,
		"suggested_stages": []string{
			"需求拆解",
			"方案设计",
			"执行交付",
			"人工验收",
			"反馈评估",
		},
		"risk_level":     req.RiskLevel,
		"required_level": req.RequiredLevel,
		"priority":       req.Priority,
		"notes":          notes,
	}
}

func (s *Service) requirementAnalysisResult(ctx context.Context, inst *workflow.WorkflowInstance) map[string]any {
	result := map[string]any{
		"workflow_id":   inst.ID.String(),
		"template_id":   inst.TemplateID.String(),
		"status":        string(inst.Status),
		"current_stage": inst.CurrentStage,
		"context":       inst.Context,
	}
	taskOutputs := []map[string]any{}
	for _, task := range inst.Tasks {
		if task.Output == nil {
			continue
		}
		taskOutputs = append(taskOutputs, map[string]any{
			"task_id":    task.ID.String(),
			"stage":      task.Stage,
			"stage_type": string(task.StageType),
			"output":     task.Output,
		})
	}
	result["task_outputs"] = taskOutputs

	if wc, err := s.workflow.GetContext(ctx, inst.ID); err == nil {
		result["workflow_working_memory"] = wc.WorkingMemory
		result["workflow_principle_notes"] = wc.PrincipleNotes
		if generated, ok := candidateRequirement(wc.WorkingMemory); ok {
			result["generated_requirement"] = generated
		}
	}
	if _, ok := result["generated_requirement"]; !ok {
		if generated, ok := candidateRequirement(inst.Context); ok {
			result["generated_requirement"] = generated
		}
	}
	if _, ok := result["generated_requirement"]; !ok {
		for i := len(inst.Tasks) - 1; i >= 0; i-- {
			if generated, ok := candidateRequirement(inst.Tasks[i].Output); ok {
				result["generated_requirement"] = generated
				break
			}
		}
	}
	return result
}

func generatedRequirementFromResult(result map[string]any) map[string]any {
	if generated, ok := mapFromAny(result["generated_requirement"]); ok {
		return generated
	}
	return map[string]any{}
}

func candidateRequirement(payload map[string]any) (map[string]any, bool) {
	if payload == nil {
		return nil, false
	}
	for _, key := range []string{"generated_requirement", "requirement", "requirement_result"} {
		if generated, ok := mapFromAny(payload[key]); ok {
			return generated, true
		}
	}
	if _, ok := payload["title"].(string); ok {
		return payload, true
	}
	if _, ok := payload["description"].(string); ok {
		return payload, true
	}
	if _, ok := payload["analysis"]; ok {
		return payload, true
	}
	return nil, false
}

func mapFromAny(value any) (map[string]any, bool) {
	if value == nil {
		return nil, false
	}
	if mapped, ok := value.(map[string]any); ok {
		return mapped, true
	}
	if mapped, ok := value.(map[string]string); ok {
		result := map[string]any{}
		for key, item := range mapped {
			result[key] = item
		}
		return result, true
	}
	return nil, false
}

func stringFromMap(value map[string]any, key string) string {
	raw, _ := value[key].(string)
	return strings.TrimSpace(raw)
}

func projectEvaluationOverall(input CreateProjectEvaluationInput) float64 {
	return clampScore(input.QualityScore*0.35 + input.DeliveryScore*0.30 + input.CostScore*0.15 + input.CollaborationScore*0.20)
}

func clampScore(score float64) float64 {
	return math.Min(math.Max(score, 0), 1)
}

func normalizePriority(priority string) string {
	switch priority {
	case "low", "medium", "high", "critical":
		return priority
	default:
		return "medium"
	}
}

func normalizeRisk(risk string) string {
	switch risk {
	case "low", "medium", "high", "critical":
		return risk
	default:
		return "low"
	}
}

func normalizeLevel(level string) string {
	switch level {
	case "L1", "L2", "L3", "L4":
		return level
	default:
		return "L1"
	}
}

func minLevel(primary string, fallback string) string {
	if primary != "" {
		return normalizeLevel(primary)
	}
	return normalizeLevel(fallback)
}

func normalizeActorType(actorType string) string {
	switch actorType {
	case "human":
		return "internal_human"
	case "ai":
		return "internal_agent"
	case "internal", "internal_human":
		return "internal_human"
	case "external", "external_human":
		return "external_human"
	case "agent", "internal_agent":
		return "internal_agent"
	case "external_agent":
		return "external_agent"
	default:
		return actorType
	}
}

func normalizeAuthActorType(userType string) string {
	if userType == "ai" {
		return "internal_agent"
	}
	return "internal_human"
}

func isHumanActor(actorType string) bool {
	return actorType == "internal_human" || actorType == "external_human"
}

func isValidProjectStatus(status string) bool {
	switch status {
	case "planning", "active", "paused", "delivering", "completed", "closed", "cancelled":
		return true
	default:
		return false
	}
}

func validateRequirementStatus(status string, action string) error {
	switch action {
	case "analyze", "sync_analysis":
		if status == "draft" || status == "analyzed" || status == "converted" {
			return nil
		}
	case "approve":
		if status == "analyzed" {
			return nil
		}
	case "convert":
		return nil
	}
	return fmt.Errorf("%w: requirement status %q cannot %s", ErrConflict, status, action)
}

func validateProjectCanWriteDeliverable(status string) error {
	switch status {
	case "active", "delivering":
		return nil
	default:
		return fmt.Errorf("%w: project status %q cannot write deliverables", ErrConflict, status)
	}
}

func validateDeliverableStatusChange(current string, next string) error {
	switch next {
	case "submitted":
		if current == "draft" || current == "rejected" {
			return nil
		}
	case "accepted", "rejected":
		if current == "submitted" {
			return nil
		}
	}
	return fmt.Errorf("%w: deliverable status %q cannot transition to %q", ErrConflict, current, next)
}

func (s *Service) validateProjectStatusChange(ctx context.Context, proj *Project, next string) error {
	if proj.Status == next {
		return nil
	}
	if proj.Status == "closed" || proj.Status == "cancelled" {
		return fmt.Errorf("%w: terminal project status %q cannot change", ErrConflict, proj.Status)
	}
	switch next {
	case "paused":
		if proj.Status == "planning" || proj.Status == "active" || proj.Status == "delivering" {
			return nil
		}
	case "active":
		if proj.Status != "planning" && proj.Status != "paused" {
			break
		}
		members, err := s.ListProjectMembers(ctx, proj.ID)
		if err != nil {
			return err
		}
		workflows, err := s.ListProjectWorkflows(ctx, proj.ID)
		if err != nil {
			return err
		}
		blockers := projectReadinessBlockers(members, workflows)
		if len(blockers) > 0 {
			return fmt.Errorf("%w: %s", ErrConflict, strings.Join(blockers, "; "))
		}
		return nil
	case "delivering":
		if proj.Status != "active" && proj.Status != "paused" {
			break
		}
		deliverables, err := s.ListDeliverables(ctx, proj.ID)
		if err != nil {
			return err
		}
		if len(deliverables) == 0 {
			return fmt.Errorf("%w: at least one deliverable is required before delivering", ErrConflict)
		}
		return nil
	case "completed":
		if proj.Status != "delivering" && proj.Status != "active" {
			break
		}
		deliverables, err := s.ListDeliverables(ctx, proj.ID)
		if err != nil {
			return err
		}
		if !hasDeliverableStatus(deliverables, "accepted") {
			return fmt.Errorf("%w: at least one accepted deliverable is required before completion", ErrConflict)
		}
		return nil
	case "closed":
		if proj.Status == "completed" {
			return nil
		}
	case "cancelled":
		return nil
	}
	return fmt.Errorf("%w: project status %q cannot transition to %q", ErrConflict, proj.Status, next)
}

func projectReadinessBlockers(members []ProjectMember, workflows []ProjectWorkflow) []string {
	blockers := []string{}
	if len(activeProjectMembers(members)) == 0 {
		blockers = append(blockers, "at least one active project member is required")
	}
	if len(activeProjectWorkflows(workflows)) == 0 {
		blockers = append(blockers, "at least one active project workflow is required")
	}
	return blockers
}

func activeProjectMembers(members []ProjectMember) []ProjectMember {
	active := []ProjectMember{}
	for _, member := range members {
		if member.Status == "active" {
			active = append(active, member)
		}
	}
	return active
}

func activeProjectWorkflows(workflows []ProjectWorkflow) []ProjectWorkflow {
	active := []ProjectWorkflow{}
	for _, item := range workflows {
		if item.Status == "active" {
			active = append(active, item)
		}
	}
	return active
}

func hasDeliverableStatus(deliverables []Deliverable, status string) bool {
	for _, deliverable := range deliverables {
		if deliverable.Status == status {
			return true
		}
	}
	return false
}

func buildProjectLifecycle(proj *Project, req *Requirement, members []ProjectMember, workflows []ProjectWorkflow, deliverables []Deliverable, costSummary *CostSummary, evaluations []ProjectEvaluation) ProjectLifecycle {
	lifecycle := ProjectLifecycle{
		Stage:          proj.Status,
		PDCAStage:      projectPDCAStage(proj.Status, deliverables),
		AllowedActions: []string{},
		Blockers:       []string{},
	}
	if proj.RequirementID != nil {
		lifecycle.RequirementID = proj.RequirementID.String()
	}
	if demandID := stringMetadata(proj.Metadata, "demand_profile_id"); demandID != "" {
		lifecycle.DemandProfileID = demandID
	} else if req != nil {
		lifecycle.DemandProfileID = stringMetadata(req.Metadata, "demand_profile_id")
	}
	if cycleID := stringMetadata(proj.Metadata, "pdca_cycle_id"); cycleID != "" {
		lifecycle.PDCACycleID = cycleID
	} else if req != nil {
		lifecycle.PDCACycleID = stringMetadata(req.Metadata, "pdca_cycle_id")
	}

	switch proj.Status {
	case "planning":
		lifecycle.Blockers = projectReadinessBlockers(members, workflows)
		lifecycle.AllowedActions = append(lifecycle.AllowedActions, "add_member", "bind_workflow")
		if len(lifecycle.Blockers) == 0 {
			lifecycle.AllowedActions = append(lifecycle.AllowedActions, "activate_project")
			lifecycle.NextAction = "activate_project"
		} else {
			lifecycle.NextAction = "prepare_project"
		}
	case "active":
		lifecycle.AllowedActions = append(lifecycle.AllowedActions, "create_deliverable", "submit_deliverable", "refresh_cost", "pause_project")
		if len(deliverables) > 0 {
			lifecycle.AllowedActions = append(lifecycle.AllowedActions, "start_delivery")
			lifecycle.NextAction = "start_delivery"
		} else {
			lifecycle.Blockers = append(lifecycle.Blockers, "at least one deliverable is required before delivering")
			lifecycle.NextAction = "create_deliverable"
		}
	case "delivering":
		lifecycle.AllowedActions = append(lifecycle.AllowedActions, "create_deliverable", "submit_deliverable", "accept_deliverable", "reject_deliverable", "refresh_cost")
		if hasDeliverableStatus(deliverables, "accepted") {
			lifecycle.AllowedActions = append(lifecycle.AllowedActions, "complete_project")
			lifecycle.NextAction = "complete_project"
		} else {
			lifecycle.Blockers = append(lifecycle.Blockers, "accepted deliverable is required before completion")
			lifecycle.NextAction = "accept_deliverable"
		}
	case "completed":
		lifecycle.AllowedActions = append(lifecycle.AllowedActions, "create_evaluation")
		if len(evaluations) > 0 || len(activeProjectMembers(members)) > 0 {
			lifecycle.AllowedActions = append(lifecycle.AllowedActions, "close_feedback")
			lifecycle.NextAction = "close_feedback"
		} else {
			lifecycle.Blockers = append(lifecycle.Blockers, "evaluation or active member is required before closing feedback")
			lifecycle.NextAction = "create_evaluation"
		}
	case "paused":
		lifecycle.AllowedActions = append(lifecycle.AllowedActions, "activate_project", "cancel_project")
		lifecycle.NextAction = "activate_project"
	case "closed":
		lifecycle.NextAction = "review_pdca_outcome"
	case "cancelled":
		lifecycle.NextAction = "archive_or_replan"
	}
	if costSummary != nil && costSummary.BudgetAmount > 0 && costSummary.BudgetVariance < 0 {
		lifecycle.Blockers = append(lifecycle.Blockers, "project is over budget")
	}
	return lifecycle
}

func projectPDCAStage(status string, deliverables []Deliverable) string {
	switch status {
	case "planning":
		return metaresource.StagePlan
	case "active", "delivering", "paused":
		if hasDeliverableStatus(deliverables, "rejected") {
			return metaresource.StageChange
		}
		return metaresource.StageDo
	case "completed", "closed":
		return metaresource.StageAccept
	default:
		return metaresource.StageChange
	}
}

func projectStatusPDCA(status string) (string, string, string) {
	switch status {
	case "planning":
		return metaresource.StagePlan, "project_planned", "Assign members and bind workflow"
	case "active":
		return metaresource.StageDo, "project_activated", "Create and submit deliverables"
	case "delivering":
		return metaresource.StageDo, "project_delivery_started", "Accept or reject deliverables"
	case "completed":
		return metaresource.StageAccept, "project_completed", "Create evaluation and close feedback"
	case "closed":
		return metaresource.StageAccept, "project_closed", "Review PDCA outcome"
	case "paused":
		return metaresource.StageChange, "project_paused", "Resume or replan project"
	case "cancelled":
		return metaresource.StageChange, "project_cancelled", "Archive or replan"
	default:
		return metaresource.StageDo, "project_status_changed", "Continue project"
	}
}

func (s *Service) syncProjectWorkflowStatuses(ctx context.Context, workflows []ProjectWorkflow) {
	if s.workflow == nil {
		return
	}
	for i := range workflows {
		if workflows[i].Status != "active" {
			continue
		}
		inst, err := s.workflow.GetWorkflow(ctx, workflows[i].WorkflowID)
		if err != nil || inst.Status != workflow.WorkflowCompleted {
			continue
		}
		workflows[i].Status = "completed"
		_ = s.repo.UpdateProjectWorkflowStatus(ctx, workflows[i].ID, "completed")
	}
}

func (s *Service) ensureRequirementPDCA(ctx context.Context, req *Requirement, actorID uuid.UUID, actorType string) *Requirement {
	if s.metaResource == nil || req == nil {
		return req
	}
	if stringMetadata(req.Metadata, "demand_profile_id") != "" && stringMetadata(req.Metadata, "pdca_cycle_id") != "" {
		return req
	}
	demand, err := s.metaResource.CreateDemandProfile(ctx, metaresource.CreateDemandProfileInput{
		RequirementID: &req.ID,
		Title:         req.Title,
		Goal:          req.Description,
		Status:        "planned",
		BudgetConstraints: map[string]any{
			"amount":   req.BudgetAmount,
			"currency": req.BudgetCurrency,
		},
		RiskConstraints: map[string]any{
			"risk_level":     req.RiskLevel,
			"required_level": req.RequiredLevel,
		},
		Metadata: map[string]any{
			"source":     "project_lifecycle",
			"created_by": actorID.String(),
			"actor_type": actorType,
		},
	})
	if err != nil {
		return req
	}
	cycle, err := s.metaResource.CreateCycle(ctx, metaresource.CreatePDCACycleInput{
		DemandProfileID: &demand.ID,
		RequirementID:   &req.ID,
		CurrentStage:    metaresource.StagePlan,
		Summary:         req.Title,
		Metadata: map[string]any{
			"source": "project_lifecycle",
		},
	})
	if err != nil {
		return req
	}
	updated, err := s.repo.UpdateRequirement(ctx, req.ID, UpdateRequirementInput{
		Metadata: mergeMetadata(req.Metadata, map[string]any{
			"demand_profile_id": demand.ID.String(),
			"pdca_cycle_id":     cycle.ID.String(),
		}),
	})
	if err != nil {
		return req
	}
	return updated
}

func (s *Service) recordRequirementPDCA(ctx context.Context, req *Requirement, stage string, eventType string, sourceID *uuid.UUID, actorID uuid.UUID, actorType string, evidence map[string]any, decision string, nextAction string) {
	if s.metaResource == nil || req == nil {
		return
	}
	cycleID, err := uuid.Parse(stringMetadata(req.Metadata, "pdca_cycle_id"))
	if err != nil || cycleID == uuid.Nil {
		return
	}
	_, _ = s.metaResource.CreateEvent(ctx, metaresource.CreatePDCAEventInput{
		CycleID:    cycleID,
		Stage:      stage,
		EventType:  eventType,
		SourceType: "requirement",
		SourceID:   sourceID,
		ActorID:    &actorID,
		ActorType:  actorType,
		Evidence:   evidence,
		Decision:   decision,
		NextAction: nextAction,
		Metadata: map[string]any{
			"requirement_id": req.ID.String(),
		},
	})
}

func (s *Service) recordProjectPDCA(ctx context.Context, proj *Project, stage string, eventType string, sourceID *uuid.UUID, actorID uuid.UUID, actorType string, evidence map[string]any, decision string, nextAction string) {
	if s.metaResource == nil || proj == nil {
		return
	}
	cycleIDString := stringMetadata(proj.Metadata, "pdca_cycle_id")
	if cycleIDString == "" && proj.RequirementID != nil {
		if req, err := s.repo.GetRequirement(ctx, *proj.RequirementID); err == nil {
			cycleIDString = stringMetadata(req.Metadata, "pdca_cycle_id")
		}
	}
	cycleID, err := uuid.Parse(cycleIDString)
	if err != nil || cycleID == uuid.Nil {
		return
	}
	_, _ = s.metaResource.CreateEvent(ctx, metaresource.CreatePDCAEventInput{
		CycleID:    cycleID,
		Stage:      stage,
		EventType:  eventType,
		SourceType: "project",
		SourceID:   sourceID,
		ActorID:    &actorID,
		ActorType:  actorType,
		Evidence:   evidence,
		Decision:   decision,
		NextAction: nextAction,
		Metadata: map[string]any{
			"project_id":     proj.ID.String(),
			"requirement_id": uuidString(proj.RequirementID),
		},
	})
	if eventType == "project_feedback_closed" {
		if score, ok := evidence["outcome_score"].(float64); ok {
			_ = s.metaResource.CompleteCycle(ctx, cycleID, score, proj.Name)
		}
	}
}

func stringMetadata(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	switch value := metadata[key].(type) {
	case string:
		return strings.TrimSpace(value)
	case fmt.Stringer:
		return strings.TrimSpace(value.String())
	default:
		return ""
	}
}

func mergeMetadata(base map[string]any, patch map[string]any) map[string]any {
	merged := map[string]any{}
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range patch {
		if value != "" && value != nil {
			merged[key] = value
		}
	}
	return merged
}

func uuidString(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}
