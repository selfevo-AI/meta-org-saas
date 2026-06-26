package project

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/tenantdb"
)

type Repository struct {
	db tenantdb.DB
}

type scanner interface {
	Scan(dest ...any) error
}

func NewRepository(db tenantdb.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ResolveLegacyUUID(ctx context.Context, sourceTable string, key string) (uuid.UUID, error) {
	if id, err := uuid.Parse(key); err == nil {
		return id, nil
	}

	var resolved *string
	err := r.db.QueryRow(ctx, `SELECT resolve_legacy_uuid($1, $2)::TEXT`, sourceTable, key).Scan(&resolved)
	if err != nil {
		return uuid.Nil, fmt.Errorf("resolve %s business key: %w", sourceTable, err)
	}
	if resolved == nil || *resolved == "" {
		return uuid.Nil, fmt.Errorf("%w: %s %q", ErrNotFound, sourceTable, key)
	}
	id, err := uuid.Parse(*resolved)
	if err != nil {
		return uuid.Nil, fmt.Errorf("resolve %s business key returned invalid uuid: %w", sourceTable, err)
	}
	return id, nil
}

func (r *Repository) CreateRequirement(ctx context.Context, input CreateRequirementInput) (*Requirement, error) {
	analysisJSON := marshalMap(input.Analysis)
	metadataJSON := marshalMap(input.Metadata)
	req := &Requirement{}
	err := scanRequirement(r.db.QueryRow(ctx,
		`INSERT INTO requirements (
		    title, description, source, status, priority, risk_level, required_level,
		    organization_id, department_id, created_by_id, created_by_type, budget_amount,
		    budget_currency, analysis, metadata
		 )
		 VALUES ($1, $2, $3, 'draft', $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		 RETURNING id, master_key, title, description, source, status, priority, risk_level, required_level,
		           organization_id, department_id, created_by_id, created_by_type, budget_amount,
		           budget_currency, analysis, metadata,
		           created_at, updated_at`,
		input.Title, input.Description, input.Source, input.Priority, input.RiskLevel, input.RequiredLevel,
		input.OrganizationID, input.DepartmentID, input.CreatedByID, input.CreatedByType, input.BudgetAmount,
		input.BudgetCurrency, analysisJSON, metadataJSON,
	), req)
	if err != nil {
		return nil, fmt.Errorf("create requirement: %w", err)
	}
	return req, nil
}

func (r *Repository) ListRequirements(ctx context.Context, limit int) ([]Requirement, error) {
	limit = normalizeLimit(limit)
	rows, err := r.db.Query(ctx,
		`SELECT id, master_key, title, description, source, status, priority, risk_level, required_level,
		        organization_id, department_id, created_by_id, created_by_type, budget_amount,
		        budget_currency, analysis, metadata,
		        created_at, updated_at
		 FROM requirements ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list requirements: %w", err)
	}
	defer rows.Close()

	requirements := []Requirement{}
	for rows.Next() {
		var req Requirement
		if err := scanRequirement(rows, &req); err != nil {
			return nil, fmt.Errorf("scan requirement: %w", err)
		}
		requirements = append(requirements, req)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list requirements iteration: %w", err)
	}
	return requirements, nil
}

func (r *Repository) ListRequirementsByOrganization(ctx context.Context, orgID uuid.UUID, limit int) ([]Requirement, error) {
	limit = normalizeLimit(limit)
	rows, err := r.db.Query(ctx,
		`SELECT id, master_key, title, description, source, status, priority, risk_level, required_level,
		        organization_id, department_id, created_by_id, created_by_type, budget_amount,
		        budget_currency, analysis, metadata,
		        created_at, updated_at
		 FROM requirements
		 WHERE organization_id = $1
		 ORDER BY created_at DESC LIMIT $2`, orgID, limit)
	if err != nil {
		return nil, fmt.Errorf("list requirements by organization: %w", err)
	}
	defer rows.Close()

	requirements := []Requirement{}
	for rows.Next() {
		var req Requirement
		if err := scanRequirement(rows, &req); err != nil {
			return nil, fmt.Errorf("scan requirement: %w", err)
		}
		requirements = append(requirements, req)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list requirements by organization iteration: %w", err)
	}
	return requirements, nil
}

func (r *Repository) GetRequirement(ctx context.Context, id uuid.UUID) (*Requirement, error) {
	req := &Requirement{}
	err := scanRequirement(r.db.QueryRow(ctx,
		`SELECT id, master_key, title, description, source, status, priority, risk_level, required_level,
		        organization_id, department_id, created_by_id, created_by_type, budget_amount,
		        budget_currency, analysis, metadata,
		        created_at, updated_at
		 FROM requirements WHERE id = $1`, id), req)
	if err != nil {
		return nil, fmt.Errorf("get requirement: %w", err)
	}
	return req, nil
}

func (r *Repository) CreateRequirementDocument(ctx context.Context, requirementID uuid.UUID, input UploadRequirementDocumentInput, uploadedByID *uuid.UUID, uploadedByType string) (*RequirementDocument, error) {
	metadataJSON := marshalMap(input.Metadata)
	doc := &RequirementDocument{}
	err := scanRequirementDocument(r.db.QueryRow(ctx,
		`INSERT INTO requirement_documents (
		    requirement_id, file_name, content_type, size_bytes, uploaded_by_id,
		    uploaded_by_type, content, metadata
		 )
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, master_key, requirement_id, file_name, content_type, size_bytes,
		           uploaded_by_id, uploaded_by_type, metadata, created_at`,
		requirementID, input.FileName, input.ContentType, input.SizeBytes, uploadedByID, uploadedByType, input.Content, metadataJSON,
	), doc)
	if err != nil {
		return nil, fmt.Errorf("create requirement document: %w", err)
	}
	return doc, nil
}

func (r *Repository) ListRequirementDocuments(ctx context.Context, requirementID uuid.UUID) ([]RequirementDocument, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, master_key, requirement_id, file_name, content_type, size_bytes,
		        uploaded_by_id, uploaded_by_type, metadata, created_at
		 FROM requirement_documents WHERE requirement_id = $1 ORDER BY created_at DESC`, requirementID)
	if err != nil {
		return nil, fmt.Errorf("list requirement documents: %w", err)
	}
	defer rows.Close()

	documents := []RequirementDocument{}
	for rows.Next() {
		var doc RequirementDocument
		if err := scanRequirementDocument(rows, &doc); err != nil {
			return nil, fmt.Errorf("scan requirement document: %w", err)
		}
		documents = append(documents, doc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list requirement documents iteration: %w", err)
	}
	return documents, nil
}

func (r *Repository) GetRequirementDocument(ctx context.Context, id uuid.UUID) (*RequirementDocumentContent, error) {
	doc := &RequirementDocumentContent{}
	var metadataJSON []byte
	err := r.db.QueryRow(ctx,
		`SELECT id, master_key, requirement_id, file_name, content_type, size_bytes,
		        uploaded_by_id, uploaded_by_type, metadata, created_at, content
		 FROM requirement_documents WHERE id = $1`, id,
	).Scan(&doc.ID, &doc.MasterKey, &doc.RequirementID, &doc.FileName, &doc.ContentType, &doc.SizeBytes, &doc.UploadedByID,
		&doc.UploadedByType, &metadataJSON, &doc.CreatedAt, &doc.Content)
	if err != nil {
		return nil, fmt.Errorf("get requirement document: %w", err)
	}
	doc.Metadata = unmarshalMap(metadataJSON)
	return doc, nil
}

func (r *Repository) CreateRequirementAnalysisWorkflow(ctx context.Context, requirementID uuid.UUID, workflowID uuid.UUID, input StartRequirementAnalysisWorkflowInput) (*RequirementAnalysisWorkflow, error) {
	metadataJSON := marshalMap(input.Metadata)
	resultJSON := marshalMap(nil)
	analysis := &RequirementAnalysisWorkflow{}
	err := scanRequirementAnalysisWorkflow(r.db.QueryRow(ctx,
		`INSERT INTO requirement_analysis_workflows (
		    requirement_id, workflow_id, workflow_template_id, status, analysis_result, metadata
		 )
		 VALUES ($1, $2, $3, 'active', $4, $5)
		 RETURNING id, master_key, requirement_id, workflow_id, workflow_template_id, status,
		           analysis_result, metadata, created_at, updated_at`,
		requirementID, workflowID, input.WorkflowTemplateID, resultJSON, metadataJSON,
	), analysis)
	if err != nil {
		return nil, fmt.Errorf("create requirement analysis workflow: %w", err)
	}
	return analysis, nil
}

func (r *Repository) ListRequirementAnalysisWorkflows(ctx context.Context, requirementID uuid.UUID) ([]RequirementAnalysisWorkflow, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, master_key, requirement_id, workflow_id, workflow_template_id, status,
		        analysis_result, metadata, created_at, updated_at
		 FROM requirement_analysis_workflows WHERE requirement_id = $1 ORDER BY created_at DESC`, requirementID)
	if err != nil {
		return nil, fmt.Errorf("list requirement analysis workflows: %w", err)
	}
	defer rows.Close()

	workflows := []RequirementAnalysisWorkflow{}
	for rows.Next() {
		var analysis RequirementAnalysisWorkflow
		if err := scanRequirementAnalysisWorkflow(rows, &analysis); err != nil {
			return nil, fmt.Errorf("scan requirement analysis workflow: %w", err)
		}
		workflows = append(workflows, analysis)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list requirement analysis workflows iteration: %w", err)
	}
	return workflows, nil
}

func (r *Repository) GetRequirementAnalysisWorkflow(ctx context.Context, requirementID uuid.UUID, workflowID uuid.UUID) (*RequirementAnalysisWorkflow, error) {
	analysis := &RequirementAnalysisWorkflow{}
	err := scanRequirementAnalysisWorkflow(r.db.QueryRow(ctx,
		`SELECT id, master_key, requirement_id, workflow_id, workflow_template_id, status,
		        analysis_result, metadata, created_at, updated_at
		 FROM requirement_analysis_workflows WHERE requirement_id = $1 AND workflow_id = $2`,
		requirementID, workflowID,
	), analysis)
	if err != nil {
		return nil, fmt.Errorf("get requirement analysis workflow: %w", err)
	}
	return analysis, nil
}

func (r *Repository) UpdateRequirementAnalysisWorkflow(ctx context.Context, id uuid.UUID, status string, result map[string]any, metadata map[string]any) (*RequirementAnalysisWorkflow, error) {
	analysis := &RequirementAnalysisWorkflow{}
	err := scanRequirementAnalysisWorkflow(r.db.QueryRow(ctx,
		`UPDATE requirement_analysis_workflows SET
		    status = COALESCE(NULLIF($2, ''), status),
		    analysis_result = COALESCE($3::jsonb, analysis_result),
		    metadata = metadata || COALESCE($4::jsonb, '{}'::jsonb),
		    updated_at = NOW()
		 WHERE id = $1
		 RETURNING id, master_key, requirement_id, workflow_id, workflow_template_id, status,
		           analysis_result, metadata, created_at, updated_at`,
		id, status, marshalMapOrNil(result), marshalMapOrNil(metadata),
	), analysis)
	if err != nil {
		return nil, fmt.Errorf("update requirement analysis workflow: %w", err)
	}
	return analysis, nil
}

func (r *Repository) UpdateRequirement(ctx context.Context, id uuid.UUID, input UpdateRequirementInput) (*Requirement, error) {
	req := &Requirement{}
	err := scanRequirement(r.db.QueryRow(ctx,
		`UPDATE requirements SET
		    title = COALESCE(NULLIF($2, ''), title),
		    description = COALESCE(NULLIF($3, ''), description),
		    source = COALESCE(NULLIF($4, ''), source),
		    status = COALESCE(NULLIF($5, ''), status),
		    priority = COALESCE(NULLIF($6, ''), priority),
		    risk_level = COALESCE(NULLIF($7, ''), risk_level),
		    required_level = COALESCE(NULLIF($8, ''), required_level),
		    budget_amount = COALESCE($9, budget_amount),
		    budget_currency = COALESCE(NULLIF($10, ''), budget_currency),
		    analysis = COALESCE($11::jsonb, analysis),
		    metadata = COALESCE($12::jsonb, metadata),
		    updated_at = NOW()
		 WHERE id = $1
		 RETURNING id, master_key, title, description, source, status, priority, risk_level, required_level,
		           organization_id, department_id, created_by_id, created_by_type, budget_amount,
		           budget_currency, analysis, metadata,
		           created_at, updated_at`,
		id, input.Title, input.Description, input.Source, input.Status, input.Priority, input.RiskLevel,
		input.RequiredLevel, input.BudgetAmount, input.BudgetCurrency, marshalMapOrNil(input.Analysis),
		marshalMapOrNil(input.Metadata),
	), req)
	if err != nil {
		return nil, fmt.Errorf("update requirement: %w", err)
	}
	return req, nil
}

func (r *Repository) CreateProject(ctx context.Context, input CreateProjectInput) (*Project, error) {
	metadataJSON := marshalMap(input.Metadata)
	proj := &Project{}
	err := scanProject(r.db.QueryRow(ctx,
		`INSERT INTO projects (
		    requirement_id, organization_id, department_id, name, description, status,
		    priority, risk_level, required_level, budget_amount, budget_currency, metadata
		 )
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		 RETURNING id, master_key, requirement_id, organization_id, department_id, name, description, status,
		           priority, risk_level, required_level, budget_amount, budget_currency, metadata, created_at, updated_at`,
		input.RequirementID, input.OrganizationID, input.DepartmentID, input.Name, input.Description, input.Status,
		input.Priority, input.RiskLevel, input.RequiredLevel, input.BudgetAmount, input.BudgetCurrency, metadataJSON,
	), proj)
	if err != nil {
		return nil, fmt.Errorf("create project: %w", err)
	}
	return proj, nil
}

func (r *Repository) ListProjects(ctx context.Context, limit int) ([]Project, error) {
	limit = normalizeLimit(limit)
	rows, err := r.db.Query(ctx,
		`SELECT id, master_key, requirement_id, organization_id, department_id, name, description, status,
		        priority, risk_level, required_level, budget_amount, budget_currency, metadata, created_at, updated_at
		 FROM projects ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	projects := []Project{}
	for rows.Next() {
		var proj Project
		if err := scanProject(rows, &proj); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, proj)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list projects iteration: %w", err)
	}
	return projects, nil
}

func (r *Repository) ListProjectsByOrganization(ctx context.Context, orgID uuid.UUID, limit int) ([]Project, error) {
	limit = normalizeLimit(limit)
	rows, err := r.db.Query(ctx,
		`SELECT id, master_key, requirement_id, organization_id, department_id, name, description, status,
		        priority, risk_level, required_level, budget_amount, budget_currency, metadata, created_at, updated_at
		 FROM projects
		 WHERE organization_id = $1
		 ORDER BY created_at DESC LIMIT $2`, orgID, limit)
	if err != nil {
		return nil, fmt.Errorf("list projects by organization: %w", err)
	}
	defer rows.Close()

	projects := []Project{}
	for rows.Next() {
		var proj Project
		if err := scanProject(rows, &proj); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, proj)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list projects by organization iteration: %w", err)
	}
	return projects, nil
}

func (r *Repository) GetProject(ctx context.Context, id uuid.UUID) (*Project, error) {
	proj := &Project{}
	err := scanProject(r.db.QueryRow(ctx,
		`SELECT id, master_key, requirement_id, organization_id, department_id, name, description, status,
		        priority, risk_level, required_level, budget_amount, budget_currency, metadata, created_at, updated_at
		 FROM projects WHERE id = $1`, id), proj)
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}
	return proj, nil
}

func (r *Repository) GetProjectByRequirement(ctx context.Context, requirementID uuid.UUID) (*Project, error) {
	proj := &Project{}
	err := scanProject(r.db.QueryRow(ctx,
		`SELECT id, master_key, requirement_id, organization_id, department_id, name, description, status,
		        priority, risk_level, required_level, budget_amount, budget_currency, metadata, created_at, updated_at
		 FROM projects
		 WHERE requirement_id = $1 AND status <> 'cancelled'
		 ORDER BY created_at ASC
		 LIMIT 1`, requirementID), proj)
	if err != nil {
		return nil, fmt.Errorf("get project by requirement: %w", err)
	}
	return proj, nil
}

func (r *Repository) UpdateProject(ctx context.Context, id uuid.UUID, input UpdateProjectInput) (*Project, error) {
	proj := &Project{}
	err := scanProject(r.db.QueryRow(ctx,
		`UPDATE projects SET
		    name = COALESCE(NULLIF($2, ''), name),
		    description = COALESCE(NULLIF($3, ''), description),
		    status = COALESCE(NULLIF($4, ''), status),
		    priority = COALESCE(NULLIF($5, ''), priority),
		    risk_level = COALESCE(NULLIF($6, ''), risk_level),
		    required_level = COALESCE(NULLIF($7, ''), required_level),
		    budget_amount = COALESCE($8, budget_amount),
		    budget_currency = COALESCE(NULLIF($9, ''), budget_currency),
		    metadata = COALESCE($10::jsonb, metadata),
		    updated_at = NOW()
		 WHERE id = $1
		 RETURNING id, master_key, requirement_id, organization_id, department_id, name, description, status,
		           priority, risk_level, required_level, budget_amount, budget_currency, metadata, created_at, updated_at`,
		id, input.Name, input.Description, input.Status, input.Priority, input.RiskLevel, input.RequiredLevel,
		input.BudgetAmount, input.BudgetCurrency, marshalMapOrNil(input.Metadata),
	), proj)
	if err != nil {
		return nil, fmt.Errorf("update project: %w", err)
	}
	return proj, nil
}

func (r *Repository) AddProjectMember(ctx context.Context, input AddProjectMemberInput) (*ProjectMember, error) {
	capabilitiesJSON := marshalStrings(input.Capabilities)
	metadataJSON := marshalMap(input.Metadata)
	member := &ProjectMember{}
	err := scanProjectMember(r.db.QueryRow(ctx,
		`INSERT INTO project_members (
		    project_id, actor_id, actor_type, position_id, position_assignment_id, role, title, allocation_percent, cost_rate,
		    permission_level, capabilities, status, metadata
		 )
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		 RETURNING id, master_key, project_id, actor_id, actor_type, position_id, position_assignment_id, role, title, allocation_percent, cost_rate,
		           permission_level, capabilities, status, metadata, created_at, updated_at`,
		input.ProjectID, input.MemberActorID, input.MemberActorType, input.PositionID, input.PositionAssignmentID, input.Role, input.Title,
		input.AllocationPercent, input.CostRate, input.PermissionLevel, capabilitiesJSON, input.Status, metadataJSON,
	), member)
	if err != nil {
		return nil, fmt.Errorf("add project member: %w", err)
	}
	return member, nil
}

func (r *Repository) ListProjectMembers(ctx context.Context, projectID uuid.UUID) ([]ProjectMember, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, master_key, project_id, actor_id, actor_type, position_id, position_assignment_id, role, title, allocation_percent, cost_rate,
		        permission_level, capabilities, status, metadata, created_at, updated_at
		 FROM project_members WHERE project_id = $1 ORDER BY created_at DESC`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list project members: %w", err)
	}
	defer rows.Close()

	members := []ProjectMember{}
	for rows.Next() {
		var member ProjectMember
		if err := scanProjectMember(rows, &member); err != nil {
			return nil, fmt.Errorf("scan project member: %w", err)
		}
		members = append(members, member)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list project members iteration: %w", err)
	}
	return members, nil
}

func (r *Repository) BindProjectWorkflow(ctx context.Context, input BindProjectWorkflowInput, projectID uuid.UUID, workflowID uuid.UUID) (*ProjectWorkflow, error) {
	metadataJSON := marshalMap(input.Metadata)
	pw := &ProjectWorkflow{}
	err := scanProjectWorkflow(r.db.QueryRow(ctx,
		`INSERT INTO project_workflows (project_id, workflow_id, workflow_template_id, purpose, status, metadata)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, master_key, project_id, workflow_id, workflow_template_id, purpose, status, metadata, created_at`,
		projectID, workflowID, input.WorkflowTemplateID, input.Purpose, input.Status, metadataJSON,
	), pw)
	if err != nil {
		return nil, fmt.Errorf("bind project workflow: %w", err)
	}
	return pw, nil
}

func (r *Repository) ListProjectWorkflows(ctx context.Context, projectID uuid.UUID) ([]ProjectWorkflow, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, master_key, project_id, workflow_id, workflow_template_id, purpose, status, metadata, created_at
		 FROM project_workflows WHERE project_id = $1 ORDER BY created_at DESC`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list project workflows: %w", err)
	}
	defer rows.Close()

	workflows := []ProjectWorkflow{}
	for rows.Next() {
		var wf ProjectWorkflow
		if err := scanProjectWorkflow(rows, &wf); err != nil {
			return nil, fmt.Errorf("scan project workflow: %w", err)
		}
		workflows = append(workflows, wf)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list project workflows iteration: %w", err)
	}
	return workflows, nil
}

func (r *Repository) UpdateProjectWorkflowStatus(ctx context.Context, id uuid.UUID, status string) error {
	_, err := r.db.Exec(ctx, `UPDATE project_workflows SET status = $2 WHERE id = $1`, id, status)
	if err != nil {
		return fmt.Errorf("update project workflow status: %w", err)
	}
	return nil
}

func (r *Repository) CreateDeliverable(ctx context.Context, projectID uuid.UUID, input CreateDeliverableInput, submittedByID *uuid.UUID, submittedByType string) (*Deliverable, error) {
	evidenceJSON := marshalMap(input.Evidence)
	metadataJSON := marshalMap(input.Metadata)
	deliverable := &Deliverable{}
	err := scanDeliverable(r.db.QueryRow(ctx,
		`INSERT INTO deliverables (
		    project_id, name, deliverable_type, uri, version, status, submitted_by_id,
		    submitted_by_type, submitted_at, evidence, metadata
		 )
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, CASE WHEN $6 = 'submitted' THEN NOW() ELSE NULL END, $9, $10)
		 RETURNING id, master_key, project_id, name, deliverable_type, uri, version, status, submitted_by_id,
		           submitted_by_type, accepted_by_id, accepted_by_type, evidence, metadata, submitted_at,
		           accepted_at, created_at, updated_at`,
		projectID, input.Name, input.DeliverableType, input.URI, input.Version, input.Status,
		submittedByID, submittedByType, evidenceJSON, metadataJSON,
	), deliverable)
	if err != nil {
		return nil, fmt.Errorf("create deliverable: %w", err)
	}
	return deliverable, nil
}

func (r *Repository) GetDeliverable(ctx context.Context, id uuid.UUID) (*Deliverable, error) {
	deliverable := &Deliverable{}
	err := scanDeliverable(r.db.QueryRow(ctx,
		`SELECT id, master_key, project_id, name, deliverable_type, uri, version, status, submitted_by_id,
		        submitted_by_type, accepted_by_id, accepted_by_type, evidence, metadata, submitted_at,
		        accepted_at, created_at, updated_at
		 FROM deliverables WHERE id = $1`, id), deliverable)
	if err != nil {
		return nil, fmt.Errorf("get deliverable: %w", err)
	}
	return deliverable, nil
}

func (r *Repository) ListDeliverables(ctx context.Context, projectID uuid.UUID) ([]Deliverable, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, master_key, project_id, name, deliverable_type, uri, version, status, submitted_by_id,
		        submitted_by_type, accepted_by_id, accepted_by_type, evidence, metadata, submitted_at,
		        accepted_at, created_at, updated_at
		 FROM deliverables WHERE project_id = $1 ORDER BY created_at DESC`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list deliverables: %w", err)
	}
	defer rows.Close()

	deliverables := []Deliverable{}
	for rows.Next() {
		var deliverable Deliverable
		if err := scanDeliverable(rows, &deliverable); err != nil {
			return nil, fmt.Errorf("scan deliverable: %w", err)
		}
		deliverables = append(deliverables, deliverable)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list deliverables iteration: %w", err)
	}
	return deliverables, nil
}

func (r *Repository) UpdateDeliverable(ctx context.Context, id uuid.UUID, input UpdateDeliverableInput) (*Deliverable, error) {
	deliverable := &Deliverable{}
	err := scanDeliverable(r.db.QueryRow(ctx,
		`UPDATE deliverables SET
		    name = COALESCE(NULLIF($2, ''), name),
		    deliverable_type = COALESCE(NULLIF($3, ''), deliverable_type),
		    uri = COALESCE(NULLIF($4, ''), uri),
		    version = COALESCE(NULLIF($5, ''), version),
		    status = COALESCE(NULLIF($6, ''), status),
		    evidence = COALESCE($7::jsonb, evidence),
		    metadata = COALESCE($8::jsonb, metadata),
		    updated_at = NOW()
		 WHERE id = $1
		 RETURNING id, master_key, project_id, name, deliverable_type, uri, version, status, submitted_by_id,
		           submitted_by_type, accepted_by_id, accepted_by_type, evidence, metadata, submitted_at,
		           accepted_at, created_at, updated_at`,
		id, input.Name, input.DeliverableType, input.URI, input.Version, input.Status,
		marshalMapOrNil(input.Evidence), marshalMapOrNil(input.Metadata),
	), deliverable)
	if err != nil {
		return nil, fmt.Errorf("update deliverable: %w", err)
	}
	return deliverable, nil
}

func (r *Repository) UpdateDeliverableStatus(ctx context.Context, id uuid.UUID, status string, actorID *uuid.UUID, actorType string, evidence map[string]any, metadata map[string]any) (*Deliverable, error) {
	deliverable := &Deliverable{}
	err := scanDeliverable(r.db.QueryRow(ctx,
		`UPDATE deliverables SET
		    status = $2,
		    submitted_by_id = CASE WHEN $2 = 'submitted' THEN $3 ELSE submitted_by_id END,
		    submitted_by_type = CASE WHEN $2 = 'submitted' THEN $4 ELSE submitted_by_type END,
		    submitted_at = CASE WHEN $2 = 'submitted' THEN NOW() ELSE submitted_at END,
		    accepted_by_id = CASE WHEN $2 = 'accepted' THEN $3 ELSE accepted_by_id END,
		    accepted_by_type = CASE WHEN $2 = 'accepted' THEN $4 ELSE accepted_by_type END,
		    accepted_at = CASE WHEN $2 = 'accepted' THEN NOW() ELSE accepted_at END,
		    evidence = evidence || COALESCE($5::jsonb, '{}'::jsonb),
		    metadata = metadata || COALESCE($6::jsonb, '{}'::jsonb),
		    updated_at = NOW()
		 WHERE id = $1
		 RETURNING id, master_key, project_id, name, deliverable_type, uri, version, status, submitted_by_id,
		           submitted_by_type, accepted_by_id, accepted_by_type, evidence, metadata, submitted_at,
		           accepted_at, created_at, updated_at`,
		id, status, actorID, actorType, marshalMapOrNil(evidence), marshalMapOrNil(metadata),
	), deliverable)
	if err != nil {
		return nil, fmt.Errorf("update deliverable status: %w", err)
	}
	return deliverable, nil
}

func (r *Repository) CreateCostEntry(ctx context.Context, projectID uuid.UUID, input CreateCostEntryInput) (*CostEntry, error) {
	metadataJSON := marshalMap(input.Metadata)
	occurredAt := time.Now().UTC()
	if input.OccurredAt != nil {
		occurredAt = *input.OccurredAt
	}
	entry := &CostEntry{}
	err := scanCostEntry(r.db.QueryRow(ctx,
		`INSERT INTO project_cost_entries (
		    project_id, source_type, source_id, actor_id, actor_type, amount,
		    currency, occurred_at, description, metadata
		 )
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 RETURNING id, master_key, project_id, source_type, source_id, actor_id, actor_type, amount,
		           currency, occurred_at, description, metadata, created_at`,
		projectID, input.SourceType, input.SourceID, input.EntryActorID, input.EntryActorType,
		input.Amount, input.Currency, occurredAt, input.Description, metadataJSON,
	), entry)
	if err != nil {
		return nil, fmt.Errorf("create cost entry: %w", err)
	}
	return entry, nil
}

func (r *Repository) GetCostEntryBySource(ctx context.Context, sourceType string, sourceID uuid.UUID) (*CostEntry, error) {
	entry := &CostEntry{}
	err := scanCostEntry(r.db.QueryRow(ctx,
		`SELECT id, master_key, project_id, source_type, source_id, actor_id, actor_type, amount,
		        currency, occurred_at, description, metadata, created_at
		 FROM project_cost_entries
		 WHERE source_type = $1 AND source_id = $2
		 ORDER BY created_at ASC
		 LIMIT 1`, sourceType, sourceID), entry)
	if err != nil {
		return nil, fmt.Errorf("get cost entry by source: %w", err)
	}
	return entry, nil
}

func (r *Repository) GetMemberAllocationCostEntry(ctx context.Context, projectID uuid.UUID, memberID uuid.UUID, refreshPeriod string) (*CostEntry, error) {
	entry := &CostEntry{}
	err := scanCostEntry(r.db.QueryRow(ctx,
		`SELECT id, master_key, project_id, source_type, source_id, actor_id, actor_type, amount,
		        currency, occurred_at, description, metadata, created_at
		 FROM project_cost_entries
		 WHERE project_id = $1
		   AND source_type = 'member_allocation'
		   AND metadata->>'project_member_id' = $2
		   AND metadata->>'refresh_period' = $3
		 ORDER BY created_at ASC
		 LIMIT 1`, projectID, memberID.String(), refreshPeriod), entry)
	if err != nil {
		return nil, fmt.Errorf("get member allocation cost entry: %w", err)
	}
	return entry, nil
}

func (r *Repository) ListCostEntries(ctx context.Context, projectID uuid.UUID) ([]CostEntry, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, master_key, project_id, source_type, source_id, actor_id, actor_type, amount,
		        currency, occurred_at, description, metadata, created_at
		 FROM project_cost_entries WHERE project_id = $1 ORDER BY occurred_at DESC, created_at DESC`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list cost entries: %w", err)
	}
	defer rows.Close()

	entries := []CostEntry{}
	for rows.Next() {
		var entry CostEntry
		if err := scanCostEntry(rows, &entry); err != nil {
			return nil, fmt.Errorf("scan cost entry: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list cost entries iteration: %w", err)
	}
	return entries, nil
}

func (r *Repository) GetCostSummary(ctx context.Context, projectID uuid.UUID) (*CostSummary, error) {
	project, err := r.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	summary := &CostSummary{
		ProjectID:    projectID,
		Currency:     "CNY",
		BudgetAmount: project.BudgetAmount,
		BySource:     []CostSummaryItem{},
	}
	err = r.db.QueryRow(ctx,
		`SELECT COALESCE(currency, 'CNY'), COUNT(*), COALESCE(SUM(amount), 0)
		 FROM project_cost_entries WHERE project_id = $1
		 GROUP BY currency ORDER BY SUM(amount) DESC LIMIT 1`, projectID,
	).Scan(&summary.Currency, &summary.EntryCount, &summary.TotalAmount)
	if err != nil {
		summary.BudgetVariance = project.BudgetAmount
		return summary, nil
	}
	summary.BudgetVariance = project.BudgetAmount - summary.TotalAmount

	rows, err := r.db.Query(ctx,
		`SELECT source_type, COUNT(*), COALESCE(SUM(amount), 0)
		 FROM project_cost_entries WHERE project_id = $1
		 GROUP BY source_type ORDER BY SUM(amount) DESC`, projectID)
	if err != nil {
		return nil, fmt.Errorf("project cost source summary: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var item CostSummaryItem
		if err := rows.Scan(&item.SourceType, &item.Count, &item.Amount); err != nil {
			return nil, fmt.Errorf("scan cost summary item: %w", err)
		}
		summary.BySource = append(summary.BySource, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("cost summary item iteration: %w", err)
	}
	return summary, nil
}

func (r *Repository) CreateProjectEvaluation(ctx context.Context, projectID uuid.UUID, input CreateProjectEvaluationInput, evaluatorID *uuid.UUID, evaluatorType string, overall float64) (*ProjectEvaluation, error) {
	evidenceJSON := marshalMap(input.Evidence)
	eval := &ProjectEvaluation{}
	err := scanProjectEvaluation(r.db.QueryRow(ctx,
		`INSERT INTO project_evaluations (
		    project_id, actor_id, actor_type, capability_id, evaluator_id, evaluator_type,
		    quality_score, delivery_score, cost_score, collaboration_score, overall_score,
		    conclusion, evidence
		 )
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		 RETURNING id, master_key, project_id, actor_id, actor_type, capability_id, evaluator_id, evaluator_type,
		           quality_score, delivery_score, cost_score, collaboration_score, overall_score,
		           conclusion, evidence, created_at`,
		projectID, input.EvaluatedActorID, input.EvaluatedActorType, input.CapabilityID, evaluatorID, evaluatorType,
		input.QualityScore, input.DeliveryScore, input.CostScore, input.CollaborationScore, overall,
		input.Conclusion, evidenceJSON,
	), eval)
	if err != nil {
		return nil, fmt.Errorf("create project evaluation: %w", err)
	}
	return eval, nil
}

func (r *Repository) ListProjectEvaluations(ctx context.Context, projectID uuid.UUID) ([]ProjectEvaluation, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, master_key, project_id, actor_id, actor_type, capability_id, evaluator_id, evaluator_type,
		        quality_score, delivery_score, cost_score, collaboration_score, overall_score,
		        conclusion, evidence, created_at
		 FROM project_evaluations WHERE project_id = $1 ORDER BY created_at DESC`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list project evaluations: %w", err)
	}
	defer rows.Close()

	evaluations := []ProjectEvaluation{}
	for rows.Next() {
		var eval ProjectEvaluation
		if err := scanProjectEvaluation(rows, &eval); err != nil {
			return nil, fmt.Errorf("scan project evaluation: %w", err)
		}
		evaluations = append(evaluations, eval)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list project evaluations iteration: %w", err)
	}
	return evaluations, nil
}

func scanRequirement(row scanner, req *Requirement) error {
	var analysisJSON, metadataJSON []byte
	if err := row.Scan(&req.ID, &req.MasterKey, &req.Title, &req.Description, &req.Source, &req.Status, &req.Priority,
		&req.RiskLevel, &req.RequiredLevel, &req.OrganizationID, &req.DepartmentID, &req.CreatedByID,
		&req.CreatedByType, &req.BudgetAmount, &req.BudgetCurrency, &analysisJSON, &metadataJSON,
		&req.CreatedAt, &req.UpdatedAt); err != nil {
		return err
	}
	req.Analysis = unmarshalMap(analysisJSON)
	req.Metadata = unmarshalMap(metadataJSON)
	return nil
}

func scanRequirementDocument(row scanner, doc *RequirementDocument) error {
	var metadataJSON []byte
	if err := row.Scan(&doc.ID, &doc.MasterKey, &doc.RequirementID, &doc.FileName, &doc.ContentType, &doc.SizeBytes,
		&doc.UploadedByID, &doc.UploadedByType, &metadataJSON, &doc.CreatedAt); err != nil {
		return err
	}
	doc.Metadata = unmarshalMap(metadataJSON)
	return nil
}

func scanRequirementAnalysisWorkflow(row scanner, analysis *RequirementAnalysisWorkflow) error {
	var resultJSON, metadataJSON []byte
	if err := row.Scan(&analysis.ID, &analysis.MasterKey, &analysis.RequirementID, &analysis.WorkflowID, &analysis.WorkflowTemplateID,
		&analysis.Status, &resultJSON, &metadataJSON, &analysis.CreatedAt, &analysis.UpdatedAt); err != nil {
		return err
	}
	analysis.AnalysisResult = unmarshalMap(resultJSON)
	analysis.Metadata = unmarshalMap(metadataJSON)
	return nil
}

func scanProject(row scanner, proj *Project) error {
	var metadataJSON []byte
	if err := row.Scan(&proj.ID, &proj.MasterKey, &proj.RequirementID, &proj.OrganizationID, &proj.DepartmentID, &proj.Name,
		&proj.Description, &proj.Status, &proj.Priority, &proj.RiskLevel, &proj.RequiredLevel,
		&proj.BudgetAmount, &proj.BudgetCurrency, &metadataJSON, &proj.CreatedAt, &proj.UpdatedAt); err != nil {
		return err
	}
	proj.Metadata = unmarshalMap(metadataJSON)
	return nil
}

func scanProjectMember(row scanner, member *ProjectMember) error {
	var capabilitiesJSON, metadataJSON []byte
	if err := row.Scan(&member.ID, &member.MasterKey, &member.ProjectID, &member.ActorID, &member.ActorType, &member.PositionID, &member.PositionAssignmentID, &member.Role,
		&member.Title, &member.AllocationPercent, &member.CostRate, &member.PermissionLevel,
		&capabilitiesJSON, &member.Status, &metadataJSON, &member.CreatedAt, &member.UpdatedAt); err != nil {
		return err
	}
	member.Capabilities = unmarshalStrings(capabilitiesJSON)
	member.Metadata = unmarshalMap(metadataJSON)
	return nil
}

func scanProjectWorkflow(row scanner, wf *ProjectWorkflow) error {
	var metadataJSON []byte
	if err := row.Scan(&wf.ID, &wf.MasterKey, &wf.ProjectID, &wf.WorkflowID, &wf.WorkflowTemplateID, &wf.Purpose,
		&wf.Status, &metadataJSON, &wf.CreatedAt); err != nil {
		return err
	}
	wf.Metadata = unmarshalMap(metadataJSON)
	return nil
}

func scanDeliverable(row scanner, deliverable *Deliverable) error {
	var evidenceJSON, metadataJSON []byte
	if err := row.Scan(&deliverable.ID, &deliverable.MasterKey, &deliverable.ProjectID, &deliverable.Name, &deliverable.DeliverableType,
		&deliverable.URI, &deliverable.Version, &deliverable.Status, &deliverable.SubmittedByID,
		&deliverable.SubmittedByType, &deliverable.AcceptedByID, &deliverable.AcceptedByType,
		&evidenceJSON, &metadataJSON, &deliverable.SubmittedAt, &deliverable.AcceptedAt,
		&deliverable.CreatedAt, &deliverable.UpdatedAt); err != nil {
		return err
	}
	deliverable.Evidence = unmarshalMap(evidenceJSON)
	deliverable.Metadata = unmarshalMap(metadataJSON)
	return nil
}

func scanCostEntry(row scanner, entry *CostEntry) error {
	var metadataJSON []byte
	if err := row.Scan(&entry.ID, &entry.MasterKey, &entry.ProjectID, &entry.SourceType, &entry.SourceID, &entry.ActorID,
		&entry.ActorType, &entry.Amount, &entry.Currency, &entry.OccurredAt, &entry.Description,
		&metadataJSON, &entry.CreatedAt); err != nil {
		return err
	}
	entry.Metadata = unmarshalMap(metadataJSON)
	return nil
}

func scanProjectEvaluation(row scanner, eval *ProjectEvaluation) error {
	var evidenceJSON []byte
	if err := row.Scan(&eval.ID, &eval.MasterKey, &eval.ProjectID, &eval.ActorID, &eval.ActorType, &eval.CapabilityID,
		&eval.EvaluatorID, &eval.EvaluatorType, &eval.QualityScore, &eval.DeliveryScore,
		&eval.CostScore, &eval.CollaborationScore, &eval.OverallScore, &eval.Conclusion,
		&evidenceJSON, &eval.CreatedAt); err != nil {
		return err
	}
	eval.Evidence = unmarshalMap(evidenceJSON)
	return nil
}

func marshalMap(value map[string]any) []byte {
	if value == nil {
		value = map[string]any{}
	}
	data, _ := json.Marshal(value)
	return data
}

func marshalMapOrNil(value map[string]any) any {
	if value == nil {
		return nil
	}
	return marshalMap(value)
}

func marshalStrings(value []string) []byte {
	if value == nil {
		value = []string{}
	}
	data, _ := json.Marshal(value)
	return data
}

func unmarshalMap(data []byte) map[string]any {
	value := map[string]any{}
	if len(data) == 0 {
		return value
	}
	_ = json.Unmarshal(data, &value)
	if value == nil {
		return map[string]any{}
	}
	return value
}

func unmarshalStrings(data []byte) []string {
	value := []string{}
	if len(data) == 0 {
		return value
	}
	_ = json.Unmarshal(data, &value)
	if value == nil {
		return []string{}
	}
	return value
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 100 {
		return 100
	}
	return limit
}
