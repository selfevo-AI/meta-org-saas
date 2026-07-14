package toolruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) CreateTool(ctx context.Context, input CreateToolInput) (*ToolDefinition, error) {
	tool := &ToolDefinition{}
	err := scanToolRow(r.db.QueryRow(ctx, `
		INSERT INTO tool_definitions (
			name, description, source_type, default_policy, risk_level, required_level,
			tool_category, approval_tier_required, input_schema, output_schema, metadata, is_active
		)
		VALUES ($1, $2, COALESCE(NULLIF($3, ''), 'internal_api'), COALESCE(NULLIF($4, ''), 'approve'),
			COALESCE(NULLIF($5, ''), 'medium'), COALESCE(NULLIF($6, ''), 'L1'),
			COALESCE(NULLIF($7, ''), 'execution_operation'), COALESCE(NULLIF($8, ''), 'executor'), $9, $10, $11, COALESCE($12, TRUE))
		RETURNING id, name, description, source_type, default_policy, risk_level, required_level,
			tool_category, approval_tier_required, input_schema, output_schema, metadata, is_active, created_at, updated_at
	`, input.Name, input.Description, input.SourceType, input.DefaultPolicy, input.RiskLevel, input.RequiredLevel,
		input.ToolCategory, input.ApprovalTierRequired, mustJSON(input.InputSchema), mustJSON(input.OutputSchema), mustJSON(input.Metadata), input.IsActive), tool)
	if err != nil {
		return nil, fmt.Errorf("create tool definition: %w", err)
	}
	return tool, nil
}

func (r *PostgresRepository) ListTools(ctx context.Context, limit int) ([]ToolDefinition, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, description, source_type, default_policy, risk_level, required_level,
			tool_category, approval_tier_required, input_schema, output_schema, metadata, is_active, created_at, updated_at
		FROM tool_definitions
		ORDER BY name
		LIMIT $1
	`, normalizeLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("list tool definitions: %w", err)
	}
	defer rows.Close()
	return scanTools(rows)
}

func (r *PostgresRepository) UpdateTool(ctx context.Context, id uuid.UUID, input UpdateToolInput) (*ToolDefinition, error) {
	tool := &ToolDefinition{}
	err := scanToolRow(r.db.QueryRow(ctx, `
		UPDATE tool_definitions
		SET description = COALESCE($2, description),
			source_type = COALESCE($3, source_type),
			default_policy = COALESCE($4, default_policy),
			risk_level = COALESCE($5, risk_level),
			required_level = COALESCE($6, required_level),
			tool_category = COALESCE($7, tool_category),
			approval_tier_required = COALESCE($8, approval_tier_required),
			input_schema = CASE WHEN $9::jsonb IS NULL THEN input_schema ELSE $9::jsonb END,
			output_schema = CASE WHEN $10::jsonb IS NULL THEN output_schema ELSE $10::jsonb END,
			metadata = CASE WHEN $11::jsonb IS NULL THEN metadata ELSE $11::jsonb END,
			is_active = COALESCE($12, is_active),
			updated_at = NOW()
		WHERE id = $1
		RETURNING id, name, description, source_type, default_policy, risk_level, required_level,
			tool_category, approval_tier_required, input_schema, output_schema, metadata, is_active, created_at, updated_at
	`, id, input.Description, input.SourceType, input.DefaultPolicy, input.RiskLevel, input.RequiredLevel,
		input.ToolCategory, input.ApprovalTierRequired, nullableJSON(input.InputSchema), nullableJSON(input.OutputSchema), nullableJSON(input.Metadata), input.IsActive), tool)
	if err != nil {
		return nil, fmt.Errorf("update tool definition: %w", err)
	}
	return tool, nil
}

func (r *PostgresRepository) GetToolByID(ctx context.Context, id uuid.UUID) (*ToolDefinition, error) {
	tool := &ToolDefinition{}
	err := scanToolRow(r.db.QueryRow(ctx, `
		SELECT id, name, description, source_type, default_policy, risk_level, required_level,
			tool_category, approval_tier_required, input_schema, output_schema, metadata, is_active, created_at, updated_at
		FROM tool_definitions WHERE id = $1
	`, id), tool)
	if err != nil {
		return nil, fmt.Errorf("get tool definition: %w", err)
	}
	return tool, nil
}

func (r *PostgresRepository) GetToolByName(ctx context.Context, name string) (*ToolDefinition, error) {
	tool := &ToolDefinition{}
	err := scanToolRow(r.db.QueryRow(ctx, `
		SELECT id, name, description, source_type, default_policy, risk_level, required_level,
			tool_category, approval_tier_required, input_schema, output_schema, metadata, is_active, created_at, updated_at
		FROM tool_definitions WHERE name = $1 AND is_active
	`, name), tool)
	if err != nil {
		return nil, fmt.Errorf("get tool definition by name: %w", err)
	}
	return tool, nil
}

func (r *PostgresRepository) CreateInterfaceFile(ctx context.Context, input CreateInterfaceFileInput, createdBy *uuid.UUID) (*InterfaceFile, error) {
	file := &InterfaceFile{}
	err := scanInterfaceFileRow(r.db.QueryRow(ctx, `
		INSERT INTO interface_files (name, file_type, content, metadata, created_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, name, file_type, content, metadata, created_by, created_at, updated_at
	`, input.Name, input.FileType, input.Content, mustJSON(input.Metadata), createdBy), file)
	if err != nil {
		return nil, fmt.Errorf("create interface file: %w", err)
	}
	return file, nil
}

func (r *PostgresRepository) ListInterfaceFiles(ctx context.Context, limit int) ([]InterfaceFile, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, file_type, content, metadata, created_by, created_at, updated_at
		FROM interface_files
		ORDER BY updated_at DESC, created_at DESC
		LIMIT $1
	`, normalizeLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("list interface files: %w", err)
	}
	defer rows.Close()
	return scanInterfaceFiles(rows)
}

func (r *PostgresRepository) GetInterfaceFile(ctx context.Context, id uuid.UUID) (*InterfaceFile, error) {
	file := &InterfaceFile{}
	err := scanInterfaceFileRow(r.db.QueryRow(ctx, `
		SELECT id, name, file_type, content, metadata, created_by, created_at, updated_at
		FROM interface_files WHERE id = $1
	`, id), file)
	if err != nil {
		return nil, fmt.Errorf("get interface file: %w", err)
	}
	return file, nil
}

func (r *PostgresRepository) UpdateInterfaceFile(ctx context.Context, id uuid.UUID, input UpdateInterfaceFileInput) (*InterfaceFile, error) {
	file := &InterfaceFile{}
	err := scanInterfaceFileRow(r.db.QueryRow(ctx, `
		UPDATE interface_files
		SET name = COALESCE($2, name),
			file_type = COALESCE($3, file_type),
			content = COALESCE($4, content),
			metadata = CASE WHEN $5::jsonb IS NULL THEN metadata ELSE $5::jsonb END,
			updated_at = NOW()
		WHERE id = $1
		RETURNING id, name, file_type, content, metadata, created_by, created_at, updated_at
	`, id, input.Name, input.FileType, input.Content, nullableJSON(input.Metadata)), file)
	if err != nil {
		return nil, fmt.Errorf("update interface file: %w", err)
	}
	return file, nil
}

func (r *PostgresRepository) CreateExecution(ctx context.Context, input CreateExecutionInput) (*ToolExecution, bool, error) {
	execution := &ToolExecution{}
	err := scanExecutionRow(r.db.QueryRow(ctx, `
		INSERT INTO tool_executions (
			tool_id, invocation_id, actor_id, actor_type, organization_id, department_id,
			project_id, workflow_id, task_id, idempotency_key, policy, governance_decision,
			requested_by_human_id, status, arguments
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT DO NOTHING
		RETURNING id, tool_id, invocation_id, actor_id, actor_type, organization_id, department_id,
			project_id, workflow_id, task_id, idempotency_key, policy, governance_decision,
			requested_by_human_id, status, arguments, result_summary, result, error_message, duration_ms, created_at, completed_at
	`, input.ToolID, input.InvocationID, input.ActorID, input.ActorType, input.OrganizationID, input.DepartmentID,
		input.ProjectID, input.WorkflowID, input.TaskID, input.IdempotencyKey, input.Policy, input.GovernanceDecision,
		input.RequestedByHumanID, input.Status, mustJSON(input.Arguments)), execution)
	if err == nil {
		return execution, false, nil
	}
	if err != pgx.ErrNoRows || input.IdempotencyKey == "" {
		return nil, false, fmt.Errorf("create tool execution: %w", err)
	}
	err = scanExecutionRow(r.db.QueryRow(ctx, `
		SELECT id, tool_id, invocation_id, actor_id, actor_type, organization_id, department_id,
			project_id, workflow_id, task_id, idempotency_key, policy, governance_decision,
			requested_by_human_id, status, arguments, result_summary, result, error_message, duration_ms, created_at, completed_at
		FROM tool_executions
		WHERE tool_id = $1 AND idempotency_key = $2
			AND ($3::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $3)
	`, input.ToolID, input.IdempotencyKey, nullableUUID(currentTenantOrganizationID(ctx))), execution)
	if err != nil {
		return nil, false, fmt.Errorf("get idempotent tool execution: %w", err)
	}
	return execution, true, nil
}

func (r *PostgresRepository) CompleteExecution(ctx context.Context, id uuid.UUID, input CompleteExecutionInput) (*ToolExecution, error) {
	execution := &ToolExecution{}
	err := scanExecutionRow(r.db.QueryRow(ctx, `
		UPDATE tool_executions
		SET status = $2, result_summary = $3, result = $4, error_message = $5, duration_ms = $6, completed_at = NOW()
		WHERE id = $1
		RETURNING id, tool_id, invocation_id, actor_id, actor_type, organization_id, department_id,
			project_id, workflow_id, task_id, idempotency_key, policy, governance_decision,
			requested_by_human_id, status, arguments, result_summary, result, error_message, duration_ms, created_at, completed_at
	`, id, input.Status, input.ResultSummary, mustJSON(input.Result), input.ErrorMessage, input.DurationMS), execution)
	if err != nil {
		return nil, fmt.Errorf("complete tool execution: %w", err)
	}
	return execution, nil
}

func (r *PostgresRepository) CreateApproval(ctx context.Context, executionID uuid.UUID, requestedBy *uuid.UUID, reason string) (*ToolApproval, error) {
	approval := &ToolApproval{}
	err := scanApprovalRow(r.db.QueryRow(ctx, `
		INSERT INTO tool_approvals (execution_id, requested_by, reason, expires_at)
		VALUES ($1, $2, $3, NOW() + INTERVAL '24 hours')
		RETURNING id, execution_id, status, requested_by, reviewed_by, approved_by_human_id, reason, expires_at, created_at, reviewed_at
	`, executionID, requestedBy, reason), approval)
	if err != nil {
		return nil, fmt.Errorf("create tool approval: %w", err)
	}
	return approval, nil
}

func (r *PostgresRepository) ListExecutions(ctx context.Context, limit int) ([]ToolExecution, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, tool_id, invocation_id, actor_id, actor_type, organization_id, department_id,
			project_id, workflow_id, task_id, idempotency_key, policy, governance_decision,
			requested_by_human_id, status, arguments, result_summary, result, error_message, duration_ms, created_at, completed_at
		FROM tool_executions
		WHERE ($2::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $2)
		ORDER BY created_at DESC
		LIMIT $1
	`, normalizeLimit(limit), nullableUUID(currentTenantOrganizationID(ctx)))
	if err != nil {
		return nil, fmt.Errorf("list tool executions: %w", err)
	}
	defer rows.Close()
	return scanExecutions(rows)
}

func (r *PostgresRepository) GetExecution(ctx context.Context, id uuid.UUID) (*ToolExecution, error) {
	execution := &ToolExecution{}
	err := scanExecutionRow(r.db.QueryRow(ctx, `
		SELECT id, tool_id, invocation_id, actor_id, actor_type, organization_id, department_id,
			project_id, workflow_id, task_id, idempotency_key, policy, governance_decision,
			requested_by_human_id, status, arguments, result_summary, result, error_message, duration_ms, created_at, completed_at
		FROM tool_executions
		WHERE id = $1 AND ($2::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $2)
	`, id, nullableUUID(currentTenantOrganizationID(ctx))), execution)
	if err != nil {
		return nil, fmt.Errorf("get tool execution: %w", err)
	}
	return execution, nil
}

func (r *PostgresRepository) GetApproval(ctx context.Context, id uuid.UUID) (*ToolApproval, error) {
	approval := &ToolApproval{}
	err := scanApprovalRow(r.db.QueryRow(ctx, `
		SELECT a.id, a.execution_id, a.status, a.requested_by, a.reviewed_by, a.approved_by_human_id, a.reason, a.expires_at, a.created_at, a.reviewed_at
		FROM tool_approvals a
		JOIN tool_executions e ON e.id = a.execution_id
		WHERE a.id = $1 AND ($2::uuid IS NULL OR e.organization_id IS NOT DISTINCT FROM $2)
	`, id, nullableUUID(currentTenantOrganizationID(ctx))), approval)
	if err != nil {
		return nil, fmt.Errorf("get tool approval: %w", err)
	}
	return approval, nil
}

func (r *PostgresRepository) GetLatestApprovalByExecution(ctx context.Context, executionID uuid.UUID) (*ToolApproval, error) {
	approval := &ToolApproval{}
	err := scanApprovalRow(r.db.QueryRow(ctx, `
		SELECT a.id, a.execution_id, a.status, a.requested_by, a.reviewed_by, a.approved_by_human_id, a.reason, a.expires_at, a.created_at, a.reviewed_at
		FROM tool_approvals a
		JOIN tool_executions e ON e.id = a.execution_id
		WHERE a.execution_id = $1 AND ($2::uuid IS NULL OR e.organization_id IS NOT DISTINCT FROM $2)
		ORDER BY a.created_at DESC
		LIMIT 1
	`, executionID, nullableUUID(currentTenantOrganizationID(ctx))), approval)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest tool approval: %w", err)
	}
	return approval, nil
}

func (r *PostgresRepository) GetHumanAuthorityTier(ctx context.Context, userID uuid.UUID, organizationID *uuid.UUID) (string, error) {
	var tier string
	if organizationID != nil {
		err := r.db.QueryRow(ctx, `
			SELECT COALESCE(
				(SELECT 'organization_creator'
				 WHERE EXISTS (
					SELECT 1 FROM organizations WHERE id = $1 AND created_by = $2
				 )),
				(SELECT authority_tier
				 FROM organization_memberships
				 WHERE organization_id = $1 AND user_id = $2 AND status = 'active'
				 ORDER BY CASE authority_tier
					WHEN 'organization_creator' THEN 3
					WHEN 'reviewer' THEN 2
					WHEN 'executor' THEN 1
					ELSE 0
				 END DESC
				 LIMIT 1),
				''
			)
		`, *organizationID, userID).Scan(&tier)
		if err != nil {
			return "", fmt.Errorf("get human authority tier: %w", err)
		}
		return tier, nil
	}

	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(
			(SELECT 'organization_creator'
			 WHERE EXISTS (
				SELECT 1 FROM organizations WHERE created_by = $1
			 )),
			(SELECT authority_tier
			 FROM organization_memberships
			 WHERE user_id = $1 AND status = 'active'
			 ORDER BY CASE authority_tier
				WHEN 'organization_creator' THEN 3
				WHEN 'reviewer' THEN 2
				WHEN 'executor' THEN 1
				ELSE 0
			 END DESC
			 LIMIT 1),
			''
		)
	`, userID).Scan(&tier)
	if err != nil {
		return "", fmt.Errorf("get human authority tier: %w", err)
	}
	return tier, nil
}

func (r *PostgresRepository) UpdateApproval(ctx context.Context, id uuid.UUID, status string, reviewedBy *uuid.UUID, reason string) (*ToolApproval, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin update tool approval: %w", err)
	}
	defer tx.Rollback(ctx)

	approval := &ToolApproval{}
	err = scanApprovalRow(tx.QueryRow(ctx, `
		UPDATE tool_approvals
		SET status = $2,
			reviewed_by = $3,
			approved_by_human_id = CASE WHEN $2 = 'approved' THEN $3 ELSE approved_by_human_id END,
			reason = COALESCE(NULLIF($4, ''), reason),
			reviewed_at = NOW()
		WHERE id = $1
		RETURNING id, execution_id, status, requested_by, reviewed_by, approved_by_human_id, reason, expires_at, created_at, reviewed_at
	`, id, status, reviewedBy, reason), approval)
	if err != nil {
		return nil, fmt.Errorf("update tool approval: %w", err)
	}
	executionStatus := ExecutionApproved
	switch status {
	case ApprovalRejected:
		executionStatus = ExecutionRejected
	case ApprovalExpired:
		executionStatus = ExecutionFailed
	}
	if _, err := tx.Exec(ctx, `UPDATE tool_executions SET status = $2, completed_at = NOW() WHERE id = $1`, approval.ExecutionID, executionStatus); err != nil {
		return nil, fmt.Errorf("update approval execution status: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit update tool approval: %w", err)
	}
	return approval, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanToolRow(row scanner, tool *ToolDefinition) error {
	var inputJSON, outputJSON, metaJSON []byte
	if err := row.Scan(&tool.ID, &tool.Name, &tool.Description, &tool.SourceType, &tool.DefaultPolicy,
		&tool.RiskLevel, &tool.RequiredLevel, &tool.ToolCategory, &tool.ApprovalTierRequired, &inputJSON, &outputJSON, &metaJSON,
		&tool.IsActive, &tool.CreatedAt, &tool.UpdatedAt); err != nil {
		return err
	}
	if err := json.Unmarshal(inputJSON, &tool.InputSchema); err != nil {
		return fmt.Errorf("unmarshal input schema: %w", err)
	}
	if err := json.Unmarshal(outputJSON, &tool.OutputSchema); err != nil {
		return fmt.Errorf("unmarshal output schema: %w", err)
	}
	if err := json.Unmarshal(metaJSON, &tool.Metadata); err != nil {
		return fmt.Errorf("unmarshal tool metadata: %w", err)
	}
	return nil
}

func scanExecutionRow(row scanner, execution *ToolExecution) error {
	var invocationID, organizationID, departmentID, projectID, workflowID, taskID, requestedByHumanID pgtype.UUID
	var argsJSON, resultJSON []byte
	var completedAt pgtype.Timestamptz
	if err := row.Scan(&execution.ID, &execution.ToolID, &invocationID, &execution.ActorID, &execution.ActorType,
		&organizationID, &departmentID, &projectID, &workflowID, &taskID, &execution.IdempotencyKey,
		&execution.Policy, &execution.GovernanceDecision, &requestedByHumanID, &execution.Status, &argsJSON, &execution.ResultSummary,
		&resultJSON, &execution.ErrorMessage, &execution.DurationMS, &execution.CreatedAt, &completedAt); err != nil {
		return err
	}
	execution.InvocationID = uuidPointer(invocationID)
	execution.OrganizationID = uuidPointer(organizationID)
	execution.DepartmentID = uuidPointer(departmentID)
	execution.ProjectID = uuidPointer(projectID)
	execution.WorkflowID = uuidPointer(workflowID)
	execution.TaskID = uuidPointer(taskID)
	execution.RequestedByHumanID = uuidPointer(requestedByHumanID)
	if completedAt.Valid {
		t := completedAt.Time
		execution.CompletedAt = &t
	}
	if err := json.Unmarshal(argsJSON, &execution.Arguments); err != nil {
		return fmt.Errorf("unmarshal execution arguments: %w", err)
	}
	if err := json.Unmarshal(resultJSON, &execution.Result); err != nil {
		return fmt.Errorf("unmarshal execution result: %w", err)
	}
	return nil
}

func scanApprovalRow(row scanner, approval *ToolApproval) error {
	var requestedBy, reviewedBy, approvedByHumanID pgtype.UUID
	var expiresAt, reviewedAt pgtype.Timestamptz
	if err := row.Scan(&approval.ID, &approval.ExecutionID, &approval.Status, &requestedBy, &reviewedBy,
		&approvedByHumanID, &approval.Reason, &expiresAt, &approval.CreatedAt, &reviewedAt); err != nil {
		return err
	}
	approval.RequestedBy = uuidPointer(requestedBy)
	approval.ReviewedBy = uuidPointer(reviewedBy)
	approval.ApprovedByHumanID = uuidPointer(approvedByHumanID)
	if expiresAt.Valid {
		t := expiresAt.Time
		approval.ExpiresAt = &t
	}
	if reviewedAt.Valid {
		t := reviewedAt.Time
		approval.ReviewedAt = &t
	}
	return nil
}

func scanInterfaceFileRow(row scanner, file *InterfaceFile) error {
	var metaJSON []byte
	var createdBy pgtype.UUID
	if err := row.Scan(&file.ID, &file.Name, &file.FileType, &file.Content, &metaJSON, &createdBy, &file.CreatedAt, &file.UpdatedAt); err != nil {
		return err
	}
	file.CreatedBy = uuidPointer(createdBy)
	if err := json.Unmarshal(metaJSON, &file.Metadata); err != nil {
		return fmt.Errorf("unmarshal interface file metadata: %w", err)
	}
	return nil
}

func scanTools(rows pgx.Rows) ([]ToolDefinition, error) {
	items := []ToolDefinition{}
	for rows.Next() {
		var item ToolDefinition
		if err := scanToolRow(rows, &item); err != nil {
			return nil, fmt.Errorf("scan tool definition: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanExecutions(rows pgx.Rows) ([]ToolExecution, error) {
	items := []ToolExecution{}
	for rows.Next() {
		var item ToolExecution
		if err := scanExecutionRow(rows, &item); err != nil {
			return nil, fmt.Errorf("scan tool execution: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanInterfaceFiles(rows pgx.Rows) ([]InterfaceFile, error) {
	items := []InterfaceFile{}
	for rows.Next() {
		var item InterfaceFile
		if err := scanInterfaceFileRow(rows, &item); err != nil {
			return nil, fmt.Errorf("scan interface file: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func mustJSON(input map[string]any) []byte {
	if input == nil {
		input = map[string]any{}
	}
	data, err := json.Marshal(input)
	if err != nil {
		return []byte("{}")
	}
	return data
}

func nullableJSON(input map[string]any) any {
	if input == nil {
		return nil
	}
	return mustJSON(input)
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

func uuidPointer(value pgtype.UUID) *uuid.UUID {
	if !value.Valid {
		return nil
	}
	id, err := uuid.FromBytes(value.Bytes[:])
	if err != nil {
		return nil
	}
	return &id
}

func nullableUUID(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return *id
}
