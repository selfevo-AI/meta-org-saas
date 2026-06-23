package workflow

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrValidation = errors.New("validation error")
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateTemplate(ctx context.Context, input CreateWorkflowInput) (*WorkflowTemplate, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrValidation)
	}
	if input.OrganizationID == nil {
		input.OrganizationID = currentTenantOrganizationID(ctx)
	}
	if err := ensureTenantAccess(ctx, input.OrganizationID); err != nil {
		return nil, err
	}
	if len(input.Stages) == 0 {
		input.Stages = defaultStages()
	}
	if input.AssigneeType == "" {
		input.AssigneeType = "either"
	}
	if input.RoutingRules == nil {
		input.RoutingRules = map[string]any{}
	}
	if input.VisualGraph == nil {
		input.VisualGraph = map[string]any{}
	}
	for i := range input.Stages {
		normalizeStage(&input.Stages[i])
	}
	return s.repo.CreateTemplate(ctx, input)
}

func (s *Service) GetTemplate(ctx context.Context, id uuid.UUID) (*WorkflowTemplate, error) {
	template, err := s.repo.GetTemplate(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := ensureTenantAccess(ctx, template.OrganizationID); err != nil {
		return nil, err
	}
	return template, nil
}

func (s *Service) ListTemplates(ctx context.Context) ([]WorkflowTemplate, error) {
	if orgID := currentTenantOrganizationID(ctx); orgID != nil {
		return s.repo.ListTemplatesByOrganization(ctx, *orgID)
	}
	return s.repo.ListTemplates(ctx)
}

func (s *Service) StartWorkflow(ctx context.Context, input StartWorkflowInput) (*WorkflowInstance, error) {
	tmpl, err := s.repo.GetTemplate(ctx, input.TemplateID)
	if err != nil {
		return nil, fmt.Errorf("template not found: %w", err)
	}
	if input.Context == nil {
		input.Context = map[string]any{}
	}
	if input.OrganizationID == nil {
		if tmpl.OrganizationID != nil {
			input.OrganizationID = tmpl.OrganizationID
		} else {
			input.OrganizationID = currentTenantOrganizationID(ctx)
		}
	}
	if err := ensureTenantAccess(ctx, input.OrganizationID); err != nil {
		return nil, err
	}

	return s.repo.CreateInstanceWithTasks(ctx, input, tmpl)
}

func (s *Service) GetWorkflow(ctx context.Context, id uuid.UUID) (*WorkflowInstance, error) {
	inst, err := s.repo.GetInstance(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := ensureTenantAccess(ctx, inst.OrganizationID); err != nil {
		return nil, err
	}
	tasks, err := s.repo.GetTasksByWorkflow(ctx, id)
	if err != nil {
		return nil, err
	}
	inst.Tasks = tasks
	return inst, nil
}

func (s *Service) UpdateWorkflowStatus(ctx context.Context, id uuid.UUID, status WorkflowStatus) error {
	if !isValidWorkflowStatus(status) {
		return fmt.Errorf("%w: invalid workflow status", ErrValidation)
	}
	if _, err := s.ensureWorkflowAccess(ctx, id); err != nil {
		return err
	}
	return s.repo.UpdateInstanceStatus(ctx, id, status)
}

func (s *Service) CompleteTask(ctx context.Context, taskID uuid.UUID, output map[string]any) error {
	if output == nil {
		output = map[string]any{}
	}
	if _, _, err := s.ensureTaskAccess(ctx, taskID); err != nil {
		return err
	}
	return s.repo.CompleteTaskWithWorkflowProgress(ctx, taskID, output)
}

func (s *Service) CreateTaskMatrixAssignment(ctx context.Context, taskID uuid.UUID, input CreateTaskMatrixAssignmentInput) (*TaskMatrixAssignment, error) {
	input.TaskID = taskID
	if _, _, err := s.ensureTaskAccess(ctx, taskID); err != nil {
		return nil, err
	}
	if input.PositionID == uuid.Nil || input.PositionAssignmentID == nil || *input.PositionAssignmentID == uuid.Nil || input.MetaResourceID == uuid.Nil {
		return nil, fmt.Errorf("%w: position_id, position_assignment_id, and meta_resource_id are required", ErrValidation)
	}
	if input.RoleInTask == "" {
		input.RoleInTask = "owner"
	}
	if !validTaskMatrixRole(input.RoleInTask) {
		return nil, fmt.Errorf("%w: invalid role_in_task", ErrValidation)
	}
	if input.AllocationPercent <= 0 {
		input.AllocationPercent = 100
	}
	if input.Status == "" {
		input.Status = "active"
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
	return s.repo.CreateTaskMatrixAssignment(ctx, input)
}

func (s *Service) ListTaskMatrixAssignments(ctx context.Context, taskID uuid.UUID) ([]TaskMatrixAssignment, error) {
	if _, _, err := s.ensureTaskAccess(ctx, taskID); err != nil {
		return nil, err
	}
	items, err := s.repo.ListTaskMatrixAssignments(ctx, taskID)
	if items == nil {
		items = []TaskMatrixAssignment{}
	}
	return items, err
}

func (s *Service) UpdateTaskMatrixAssignment(ctx context.Context, id uuid.UUID, input UpdateTaskMatrixAssignmentInput) (*TaskMatrixAssignment, error) {
	current, err := s.repo.GetTaskMatrixAssignmentByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := ensureTenantAccess(ctx, current.OrganizationID); err != nil {
		return nil, err
	}
	if input.RoleInTask != "" && !validTaskMatrixRole(input.RoleInTask) {
		return nil, fmt.Errorf("%w: invalid role_in_task", ErrValidation)
	}
	if input.Status != "" && !validTaskMatrixStatus(input.Status) {
		return nil, fmt.Errorf("%w: invalid status", ErrValidation)
	}
	return s.repo.UpdateTaskMatrixAssignment(ctx, id, input)
}

func (s *Service) RemoveTaskMatrixAssignment(ctx context.Context, id uuid.UUID) error {
	current, err := s.repo.GetTaskMatrixAssignmentByID(ctx, id)
	if err != nil {
		return err
	}
	if err := ensureTenantAccess(ctx, current.OrganizationID); err != nil {
		return err
	}
	return s.repo.RemoveTaskMatrixAssignment(ctx, id)
}

func (s *Service) RecordDecision(ctx context.Context, taskID uuid.UUID, decisionMakerID uuid.UUID, makerType string, reasoning string, outcome string, input, output map[string]any) (*Decision, error) {
	if _, _, err := s.ensureTaskAccess(ctx, taskID); err != nil {
		return nil, err
	}
	d := &Decision{
		TaskID:          taskID,
		DecisionMakerID: decisionMakerID,
		MakerType:       makerType,
		Weight:          1.0,
		Input:           input,
		Output:          output,
		Reasoning:       reasoning,
		Outcome:         outcome,
	}
	return s.repo.RecordDecision(ctx, d)
}

func (s *Service) GetContext(ctx context.Context, workflowID uuid.UUID) (*WorkflowContext, error) {
	if _, err := s.ensureWorkflowAccess(ctx, workflowID); err != nil {
		return nil, err
	}
	return s.repo.GetWorkflowContext(ctx, workflowID)
}

func (s *Service) UpdateContext(ctx context.Context, wc *WorkflowContext) error {
	if _, err := s.ensureWorkflowAccess(ctx, wc.WorkflowID); err != nil {
		return err
	}
	return s.repo.UpsertWorkflowContext(ctx, wc)
}

func defaultStages() []Stage {
	return []Stage{
		{Type: StagePlan, Name: "Planning", AssigneeType: "either", RequiredPermissionLevel: "L1", RiskLevel: "low"},
		{Type: StageExecute, Name: "Execution", AssigneeType: "either", RequiredPermissionLevel: "L2", RiskLevel: "medium"},
		{Type: StageReview, Name: "Review", AssigneeType: "internal", RequiredPermissionLevel: "L2", RiskLevel: "medium", PreferredActorTypes: []string{"internal_human"}},
	}
}

func normalizeStage(stage *Stage) {
	if stage.Name == "" {
		stage.Name = string(stage.Type)
	}
	if stage.ID == "" {
		stage.ID = string(stage.Type) + "-" + stage.Name
	}
	if stage.AssigneeType == "" {
		stage.AssigneeType = "either"
	}
	if stage.RequiredPermissionLevel == "" {
		stage.RequiredPermissionLevel = "L1"
	}
	if stage.RiskLevel == "" {
		stage.RiskLevel = "low"
	}
	if stage.EvaluationPolicy == nil {
		stage.EvaluationPolicy = map[string]any{"primary_reviewer": "human"}
	}
	if stage.MatchingPolicy == nil {
		stage.MatchingPolicy = map[string]any{"ranking": "capability_weight_access"}
	}
}

func isValidWorkflowStatus(status WorkflowStatus) bool {
	switch status {
	case WorkflowActive, WorkflowPaused, WorkflowCompleted, WorkflowFailed:
		return true
	default:
		return false
	}
}

func validTaskMatrixRole(value string) bool {
	switch value {
	case "owner", "reviewer", "support", "observer":
		return true
	default:
		return false
	}
}

func validTaskMatrixStatus(value string) bool {
	switch value {
	case "active", "inactive", "archived":
		return true
	default:
		return false
	}
}

func currentTenantOrganizationID(ctx context.Context) *uuid.UUID {
	tenant, ok := middleware.TenantFromContext(ctx)
	if !ok || tenant.OrganizationID == nil {
		return nil
	}
	id := *tenant.OrganizationID
	return &id
}

func (s *Service) ensureWorkflowAccess(ctx context.Context, workflowID uuid.UUID) (*WorkflowInstance, error) {
	inst, err := s.repo.GetInstance(ctx, workflowID)
	if err != nil {
		return nil, err
	}
	if err := ensureTenantAccess(ctx, inst.OrganizationID); err != nil {
		return nil, err
	}
	return inst, nil
}

func (s *Service) ensureTaskAccess(ctx context.Context, taskID uuid.UUID) (*Task, *WorkflowInstance, error) {
	task, err := s.repo.GetTaskByID(ctx, taskID)
	if err != nil {
		return nil, nil, err
	}
	inst, err := s.ensureWorkflowAccess(ctx, task.WorkflowID)
	if err != nil {
		return nil, nil, err
	}
	return task, inst, nil
}

func ensureTenantAccess(ctx context.Context, organizationID *uuid.UUID) error {
	tenant, ok := middleware.TenantFromContext(ctx)
	if !ok || tenant.OrganizationID == nil || organizationID == nil {
		return nil
	}
	if *tenant.OrganizationID != *organizationID {
		return fmt.Errorf("%w: resource is outside current organization", ErrValidation)
	}
	return nil
}
