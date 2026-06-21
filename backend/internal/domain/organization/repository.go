package organization

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) CreateOrganization(ctx context.Context, input CreateOrganizationInput) (*Organization, error) {
	org := &Organization{}
	var createdBy pgtype.UUID
	err := r.db.QueryRow(ctx,
		`INSERT INTO organizations (name, description, created_by) VALUES ($1, $2, $3)
		 RETURNING id, name, COALESCE(description, ''), created_by, created_at, updated_at`,
		input.Name, input.Description, input.CreatedBy,
	).Scan(&org.ID, &org.Name, &org.Description, &createdBy, &org.CreatedAt, &org.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create organization: %w", err)
	}
	org.CreatedBy = uuidPointer(createdBy)
	return org, nil
}

func (r *PostgresRepository) GetOrganizationByID(ctx context.Context, id uuid.UUID) (*Organization, error) {
	org := &Organization{}
	var createdBy pgtype.UUID
	err := r.db.QueryRow(ctx,
		`SELECT id, name, COALESCE(description, ''), created_by, created_at, updated_at FROM organizations WHERE id = $1`, id,
	).Scan(&org.ID, &org.Name, &org.Description, &createdBy, &org.CreatedAt, &org.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get organization: %w", err)
	}
	org.CreatedBy = uuidPointer(createdBy)
	return org, nil
}

func (r *PostgresRepository) ListOrganizations(ctx context.Context, limit int) ([]Organization, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.Query(ctx,
		`SELECT id, name, COALESCE(description, ''), created_by, created_at, updated_at
		 FROM organizations ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list organizations: %w", err)
	}
	defer rows.Close()

	var organizations []Organization
	for rows.Next() {
		var org Organization
		var createdBy pgtype.UUID
		if err := rows.Scan(&org.ID, &org.Name, &org.Description, &createdBy, &org.CreatedAt, &org.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan organization: %w", err)
		}
		org.CreatedBy = uuidPointer(createdBy)
		organizations = append(organizations, org)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list organizations iteration: %w", err)
	}
	return organizations, nil
}

func (r *PostgresRepository) UpdateOrganization(ctx context.Context, id uuid.UUID, input UpdateOrganizationInput) (*Organization, error) {
	_, err := r.db.Exec(ctx,
		`UPDATE organizations SET
			name = COALESCE(NULLIF($2, ''), name),
			description = COALESCE(NULLIF($3, ''), description),
			updated_at = NOW()
		 WHERE id = $1`,
		id, input.Name, input.Description)
	if err != nil {
		return nil, fmt.Errorf("update organization: %w", err)
	}
	return r.GetOrganizationByID(ctx, id)
}

func (r *PostgresRepository) CreateMVRU(ctx context.Context, input CreateMVRUInput) (*MVRU, error) {
	boundaryJSON, _ := json.Marshal(input.Boundary)
	configJSON, _ := json.Marshal(input.Config)

	mvru := &MVRU{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO muvrs (organization_id, name, description, boundary, config, parent_id)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, organization_id, name, description, status, boundary, config, parent_id, created_at, updated_at`,
		input.OrganizationID, input.Name, input.Description, boundaryJSON, configJSON, input.ParentID,
	).Scan(&mvru.ID, &mvru.OrganizationID, &mvru.Name, &mvru.Description, &mvru.Status, &boundaryJSON, &configJSON, &mvru.ParentID, &mvru.CreatedAt, &mvru.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create mvru: %w", err)
	}
	json.Unmarshal(boundaryJSON, &mvru.Boundary)
	json.Unmarshal(configJSON, &mvru.Config)
	return mvru, nil
}

func (r *PostgresRepository) GetMVRUByID(ctx context.Context, id uuid.UUID) (*MVRU, error) {
	mvru := &MVRU{}
	var boundaryJSON, configJSON []byte
	err := r.db.QueryRow(ctx,
		`SELECT id, organization_id, name, description, status, boundary, config, parent_id, created_at, updated_at
		 FROM muvrs WHERE id = $1`, id,
	).Scan(&mvru.ID, &mvru.OrganizationID, &mvru.Name, &mvru.Description, &mvru.Status, &boundaryJSON, &configJSON, &mvru.ParentID, &mvru.CreatedAt, &mvru.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get mvru: %w", err)
	}
	json.Unmarshal(boundaryJSON, &mvru.Boundary)
	json.Unmarshal(configJSON, &mvru.Config)
	return mvru, nil
}

func (r *PostgresRepository) ListMVRUs(ctx context.Context, orgID uuid.UUID) ([]MVRU, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, organization_id, name, description, status, boundary, config, parent_id, created_at, updated_at
		 FROM muvrs WHERE organization_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return nil, fmt.Errorf("list muvrs: %w", err)
	}
	defer rows.Close()

	var muvrs []MVRU
	for rows.Next() {
		var mvru MVRU
		var boundaryJSON, configJSON []byte
		if err := rows.Scan(&mvru.ID, &mvru.OrganizationID, &mvru.Name, &mvru.Description, &mvru.Status, &boundaryJSON, &configJSON, &mvru.ParentID, &mvru.CreatedAt, &mvru.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan mvru: %w", err)
		}
		json.Unmarshal(boundaryJSON, &mvru.Boundary)
		json.Unmarshal(configJSON, &mvru.Config)
		muvrs = append(muvrs, mvru)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list muvrs iteration: %w", err)
	}
	return muvrs, nil
}

func (r *PostgresRepository) UpdateMVRUStatus(ctx context.Context, id uuid.UUID, status MVRUStatus) error {
	_, err := r.db.Exec(ctx, `UPDATE muvrs SET status = $1, updated_at = NOW() WHERE id = $2`, status, id)
	if err != nil {
		return fmt.Errorf("update mvru status: %w", err)
	}
	return nil
}

func (r *PostgresRepository) AddMember(ctx context.Context, member MVRUMember) error {
	var err error
	if member.UserID != nil {
		_, err = r.db.Exec(ctx,
			`INSERT INTO mvru_members (mvru_id, user_id, agent_id, role_id) VALUES ($1, $2, NULL, $3)
			 ON CONFLICT ON CONSTRAINT uq_mvru_user DO UPDATE SET role_id = EXCLUDED.role_id`,
			member.MVRUID, *member.UserID, member.RoleID)
	} else {
		_, err = r.db.Exec(ctx,
			`INSERT INTO mvru_members (mvru_id, user_id, agent_id, role_id) VALUES ($1, NULL, $2, $3)
			 ON CONFLICT ON CONSTRAINT uq_mvru_agent DO UPDATE SET role_id = EXCLUDED.role_id`,
			member.MVRUID, *member.AgentID, member.RoleID)
	}
	if err != nil {
		return fmt.Errorf("add member: %w", err)
	}
	return nil
}

func (r *PostgresRepository) RemoveMember(ctx context.Context, mvruID, userID, agentID *uuid.UUID) error {
	if userID != nil {
		_, err := r.db.Exec(ctx, `DELETE FROM mvru_members WHERE mvru_id = $1 AND user_id = $2`, mvruID, *userID)
		if err != nil {
			return fmt.Errorf("remove user member: %w", err)
		}
	} else if agentID != nil {
		_, err := r.db.Exec(ctx, `DELETE FROM mvru_members WHERE mvru_id = $1 AND agent_id = $2`, mvruID, *agentID)
		if err != nil {
			return fmt.Errorf("remove agent member: %w", err)
		}
	}
	return nil
}

func (r *PostgresRepository) CreateRelationship(ctx context.Context, rel MVRURelationship) (*MVRURelationship, error) {
	configJSON, _ := json.Marshal(rel.Config)
	err := r.db.QueryRow(ctx,
		`INSERT INTO mvru_relationships (source_mvru_id, target_mvru_id, rel_type, config)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, source_mvru_id, target_mvru_id, rel_type, config, created_at`,
		rel.SourceMVRUID, rel.TargetMVRUID, rel.RelType, configJSON,
	).Scan(&rel.ID, &rel.SourceMVRUID, &rel.TargetMVRUID, &rel.RelType, &configJSON, &rel.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create relationship: %w", err)
	}
	json.Unmarshal(configJSON, &rel.Config)
	return &rel, nil
}

func (r *PostgresRepository) GetOrgChart(ctx context.Context, orgID uuid.UUID) ([]MVRU, error) {
	all, err := r.ListMVRUs(ctx, orgID)
	if err != nil {
		return nil, err
	}

	childMap := make(map[uuid.UUID][]MVRU)
	for _, mv := range all {
		if mv.ParentID != nil {
			childMap[*mv.ParentID] = append(childMap[*mv.ParentID], mv)
		}
	}

	var roots []MVRU
	for _, mv := range all {
		if mv.ParentID == nil {
			mv.Children = childMap[mv.ID]
			roots = append(roots, mv)
		}
	}
	return roots, nil
}

func (r *PostgresRepository) CreateDepartment(ctx context.Context, input CreateDepartmentInput) (*Department, error) {
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
	if input.Status == "" {
		input.Status = "active"
	}
	metadataJSON, err := json.Marshal(input.Metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal department metadata: %w", err)
	}

	dept := &Department{}
	err = r.db.QueryRow(ctx,
		`INSERT INTO departments (organization_id, parent_id, name, code, description, status, sort_order, metadata)
		 VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, $7, $8)
		 RETURNING id, organization_id, parent_id, name, COALESCE(code, ''), description, status, sort_order, metadata, created_at, updated_at`,
		input.OrganizationID, input.ParentID, input.Name, input.Code, input.Description, input.Status, input.SortOrder, metadataJSON,
	).Scan(&dept.ID, &dept.OrganizationID, &dept.ParentID, &dept.Name, &dept.Code, &dept.Description, &dept.Status, &dept.SortOrder, &metadataJSON, &dept.CreatedAt, &dept.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create department: %w", err)
	}
	if err := json.Unmarshal(metadataJSON, &dept.Metadata); err != nil {
		return nil, fmt.Errorf("unmarshal department metadata: %w", err)
	}
	return dept, nil
}

func (r *PostgresRepository) GetDepartmentByID(ctx context.Context, id uuid.UUID) (*Department, error) {
	dept := &Department{}
	var metadataJSON []byte
	err := r.db.QueryRow(ctx,
		`SELECT id, organization_id, parent_id, name, COALESCE(code, ''), description, status, sort_order, metadata, created_at, updated_at
		 FROM departments WHERE id = $1`, id,
	).Scan(&dept.ID, &dept.OrganizationID, &dept.ParentID, &dept.Name, &dept.Code, &dept.Description, &dept.Status, &dept.SortOrder, &metadataJSON, &dept.CreatedAt, &dept.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get department: %w", err)
	}
	if err := json.Unmarshal(metadataJSON, &dept.Metadata); err != nil {
		return nil, fmt.Errorf("unmarshal department metadata: %w", err)
	}
	return dept, nil
}

func (r *PostgresRepository) ListDepartments(ctx context.Context, orgID uuid.UUID) ([]Department, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, organization_id, parent_id, name, COALESCE(code, ''), description, status, sort_order, metadata, created_at, updated_at
		 FROM departments WHERE organization_id = $1 ORDER BY parent_id NULLS FIRST, sort_order, name`, orgID)
	if err != nil {
		return nil, fmt.Errorf("list departments: %w", err)
	}
	defer rows.Close()

	var departments []Department
	for rows.Next() {
		dept, err := scanDepartment(rows.Scan)
		if err != nil {
			return nil, err
		}
		departments = append(departments, *dept)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list departments iteration: %w", err)
	}
	return departments, nil
}

func (r *PostgresRepository) GetDepartmentTree(ctx context.Context, orgID uuid.UUID) ([]Department, error) {
	all, err := r.ListDepartments(ctx, orgID)
	if err != nil {
		return nil, err
	}
	positions, err := r.ListPositions(ctx, orgID, nil)
	if err != nil {
		return nil, err
	}
	positionsByDepartment := make(map[uuid.UUID][]Position)
	for _, position := range positions {
		positionsByDepartment[position.DepartmentID] = append(positionsByDepartment[position.DepartmentID], position)
	}

	byParent := make(map[uuid.UUID][]Department)
	var roots []Department
	for _, dept := range all {
		dept.Positions = positionsByDepartment[dept.ID]
		if dept.ParentID == nil {
			roots = append(roots, dept)
			continue
		}
		byParent[*dept.ParentID] = append(byParent[*dept.ParentID], dept)
	}

	var attach func([]Department) []Department
	attach = func(nodes []Department) []Department {
		for i := range nodes {
			nodes[i].Children = attach(byParent[nodes[i].ID])
		}
		return nodes
	}
	return attach(roots), nil
}

func (r *PostgresRepository) UpdateDepartment(ctx context.Context, id uuid.UUID, input UpdateDepartmentInput) (*Department, error) {
	var metadataJSON []byte
	var err error
	if input.Metadata != nil {
		metadataJSON, err = json.Marshal(input.Metadata)
		if err != nil {
			return nil, fmt.Errorf("marshal department metadata: %w", err)
		}
	}

	_, err = r.db.Exec(ctx,
		`UPDATE departments SET
			parent_id = COALESCE($2, parent_id),
			name = COALESCE(NULLIF($3, ''), name),
			code = COALESCE(NULLIF($4, ''), code),
			description = COALESCE(NULLIF($5, ''), description),
			status = COALESCE(NULLIF($6, ''), status),
			sort_order = COALESCE($7, sort_order),
			metadata = COALESCE($8::jsonb, metadata),
			updated_at = NOW()
		 WHERE id = $1`,
		id, input.ParentID, input.Name, input.Code, input.Description, input.Status, input.SortOrder, metadataJSON)
	if err != nil {
		return nil, fmt.Errorf("update department: %w", err)
	}
	return r.GetDepartmentByID(ctx, id)
}

func (r *PostgresRepository) CreatePosition(ctx context.Context, input CreatePositionInput) (*Position, error) {
	capabilitiesJSON, err := json.Marshal(input.RequiredCapabilities)
	if err != nil {
		return nil, fmt.Errorf("marshal position capabilities: %w", err)
	}
	metadataJSON, err := json.Marshal(input.Metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal position metadata: %w", err)
	}
	position := &Position{}
	err = scanPosition(r.db.QueryRow(ctx,
		`INSERT INTO positions (
		    organization_id, department_id, name, code, description, status, sort_order,
		    permission_level, required_capabilities, metadata
		 )
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		 RETURNING id, organization_id, department_id, name, COALESCE(code, ''), description, status, sort_order,
		           permission_level, required_capabilities, metadata, created_at, updated_at`,
		input.OrganizationID, input.DepartmentID, input.Name, input.Code, input.Description, input.Status, input.SortOrder,
		input.PermissionLevel, capabilitiesJSON, metadataJSON,
	).Scan, position)
	if err != nil {
		return nil, fmt.Errorf("create position: %w", err)
	}
	return position, nil
}

func (r *PostgresRepository) GetPositionByID(ctx context.Context, id uuid.UUID) (*Position, error) {
	position := &Position{}
	err := scanPosition(r.db.QueryRow(ctx,
		`SELECT id, organization_id, department_id, name, COALESCE(code, ''), description, status, sort_order,
		        permission_level, required_capabilities, metadata, created_at, updated_at
		 FROM positions WHERE id = $1`, id,
	).Scan, position)
	if err != nil {
		return nil, fmt.Errorf("get position: %w", err)
	}
	assignments, err := r.ListPositionAssignments(ctx, id)
	if err == nil {
		position.Assignments = assignments
	}
	return position, nil
}

func (r *PostgresRepository) ListPositions(ctx context.Context, orgID uuid.UUID, departmentID *uuid.UUID) ([]Position, error) {
	query := `SELECT id, organization_id, department_id, name, COALESCE(code, ''), description, status, sort_order,
	                 permission_level, required_capabilities, metadata, created_at, updated_at
	          FROM positions WHERE organization_id = $1`
	args := []any{orgID}
	if departmentID != nil {
		query += ` AND department_id = $2`
		args = append(args, *departmentID)
	}
	query += ` ORDER BY department_id, sort_order, name`
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list positions: %w", err)
	}
	defer rows.Close()

	var positions []Position
	for rows.Next() {
		var position Position
		if err := scanPosition(rows.Scan, &position); err != nil {
			return nil, err
		}
		positions = append(positions, position)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list positions iteration: %w", err)
	}
	return positions, nil
}

func (r *PostgresRepository) UpdatePosition(ctx context.Context, id uuid.UUID, input UpdatePositionInput) (*Position, error) {
	var capabilitiesJSON []byte
	var err error
	if input.RequiredCapabilities != nil {
		capabilitiesJSON, err = json.Marshal(input.RequiredCapabilities)
		if err != nil {
			return nil, fmt.Errorf("marshal position capabilities: %w", err)
		}
	}
	var metadataJSON []byte
	if input.Metadata != nil {
		metadataJSON, err = json.Marshal(input.Metadata)
		if err != nil {
			return nil, fmt.Errorf("marshal position metadata: %w", err)
		}
	}
	_, err = r.db.Exec(ctx,
		`UPDATE positions SET
		 department_id = COALESCE($2, department_id),
		 name = COALESCE(NULLIF($3, ''), name),
		 code = COALESCE(NULLIF($4, ''), code),
		 description = COALESCE($5, description),
		 status = COALESCE(NULLIF($6, ''), status),
		 sort_order = COALESCE($7, sort_order),
		 permission_level = COALESCE(NULLIF($8, ''), permission_level),
		 required_capabilities = COALESCE($9::jsonb, required_capabilities),
		 metadata = COALESCE($10::jsonb, metadata),
		 updated_at = NOW()
		 WHERE id = $1`,
		id, input.DepartmentID, input.Name, input.Code, input.Description, input.Status, input.SortOrder,
		input.PermissionLevel, capabilitiesJSON, metadataJSON,
	)
	if err != nil {
		return nil, fmt.Errorf("update position: %w", err)
	}
	return r.GetPositionByID(ctx, id)
}

func (r *PostgresRepository) CreatePositionAssignment(ctx context.Context, input CreatePositionAssignmentInput) (*PositionAssignment, error) {
	position, err := r.GetPositionByID(ctx, input.PositionID)
	if err != nil {
		return nil, err
	}
	metadataJSON, err := json.Marshal(input.Metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal position assignment metadata: %w", err)
	}
	assignment := &PositionAssignment{}
	err = scanPositionAssignment(r.db.QueryRow(ctx,
		`WITH upserted AS (
		   INSERT INTO position_assignments (
		     position_id, organization_id, department_id, meta_resource_id, actor_id, actor_type,
		     assignment_type, allocation_percent, status, metadata
		   )
		   VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		   ON CONFLICT (position_id, actor_id, actor_type) WHERE status <> 'archived'
		   DO UPDATE SET meta_resource_id = EXCLUDED.meta_resource_id,
		                 assignment_type = EXCLUDED.assignment_type,
		                 allocation_percent = EXCLUDED.allocation_percent,
		                 status = EXCLUDED.status,
		                 metadata = EXCLUDED.metadata,
		                 updated_at = NOW()
		   RETURNING id
		 )
		 `+assignmentSelectSQL()+` WHERE pa.id = (SELECT id FROM upserted)`,
		input.PositionID, position.OrganizationID, position.DepartmentID, input.MetaResourceID, input.ActorID, input.ActorType,
		input.AssignmentType, input.AllocationPercent, input.Status, metadataJSON,
	).Scan, assignment)
	if err != nil {
		return nil, fmt.Errorf("create position assignment: %w", err)
	}
	return assignment, nil
}

func (r *PostgresRepository) GetPositionAssignmentByID(ctx context.Context, id uuid.UUID) (*PositionAssignment, error) {
	assignment := &PositionAssignment{}
	if err := scanPositionAssignment(r.db.QueryRow(ctx, assignmentSelectSQL()+` WHERE pa.id = $1`, id).Scan, assignment); err != nil {
		return nil, fmt.Errorf("get position assignment: %w", err)
	}
	return assignment, nil
}

func (r *PostgresRepository) ListPositionAssignments(ctx context.Context, positionID uuid.UUID) ([]PositionAssignment, error) {
	rows, err := r.db.Query(ctx, assignmentSelectSQL()+` WHERE pa.position_id = $1 ORDER BY pa.assignment_type, actor_name`, positionID)
	if err != nil {
		return nil, fmt.Errorf("list position assignments: %w", err)
	}
	defer rows.Close()

	var assignments []PositionAssignment
	for rows.Next() {
		var assignment PositionAssignment
		if err := scanPositionAssignment(rows.Scan, &assignment); err != nil {
			return nil, err
		}
		assignments = append(assignments, assignment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list position assignments iteration: %w", err)
	}
	return assignments, nil
}

func (r *PostgresRepository) UpdatePositionAssignment(ctx context.Context, id uuid.UUID, input UpdatePositionAssignmentInput) (*PositionAssignment, error) {
	var metadataJSON []byte
	var err error
	if input.Metadata != nil {
		metadataJSON, err = json.Marshal(input.Metadata)
		if err != nil {
			return nil, fmt.Errorf("marshal position assignment metadata: %w", err)
		}
	}
	_, err = r.db.Exec(ctx,
		`UPDATE position_assignments SET
		 meta_resource_id = COALESCE($2, meta_resource_id),
		 assignment_type = COALESCE(NULLIF($3, ''), assignment_type),
		 allocation_percent = COALESCE($4, allocation_percent),
		 status = COALESCE(NULLIF($5, ''), status),
		 metadata = COALESCE($6::jsonb, metadata),
		 updated_at = NOW()
		 WHERE id = $1`,
		id, input.MetaResourceID, input.AssignmentType, input.AllocationPercent, input.Status, metadataJSON,
	)
	if err != nil {
		return nil, fmt.Errorf("update position assignment: %w", err)
	}
	assignment := &PositionAssignment{}
	if err := scanPositionAssignment(r.db.QueryRow(ctx, assignmentSelectSQL()+` WHERE pa.id = $1`, id).Scan, assignment); err != nil {
		return nil, fmt.Errorf("get position assignment: %w", err)
	}
	return assignment, nil
}

func (r *PostgresRepository) RemovePositionAssignment(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE position_assignments SET status = 'archived', updated_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("remove position assignment: %w", err)
	}
	return nil
}

func (r *PostgresRepository) CreateExternalMember(ctx context.Context, input CreateExternalMemberInput) (*ExternalMember, error) {
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
	if input.Status == "" {
		input.Status = "active"
	}
	metadataJSON, err := json.Marshal(input.Metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal external member metadata: %w", err)
	}

	member := &ExternalMember{}
	err = r.db.QueryRow(ctx,
		`INSERT INTO external_members (name, email, vendor, contract_type, status, metadata)
		 VALUES ($1, NULLIF($2, ''), $3, $4, $5, $6)
		 RETURNING id, name, COALESCE(email, ''), vendor, contract_type, status, metadata, created_at, updated_at`,
		input.Name, input.Email, input.Vendor, input.ContractType, input.Status, metadataJSON,
	).Scan(&member.ID, &member.Name, &member.Email, &member.Vendor, &member.ContractType, &member.Status, &metadataJSON, &member.CreatedAt, &member.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create external member: %w", err)
	}
	if err := json.Unmarshal(metadataJSON, &member.Metadata); err != nil {
		return nil, fmt.Errorf("unmarshal external member metadata: %w", err)
	}
	return member, nil
}

func (r *PostgresRepository) GetExternalMemberByID(ctx context.Context, id uuid.UUID) (*ExternalMember, error) {
	member := &ExternalMember{}
	var metadataJSON []byte
	err := r.db.QueryRow(ctx,
		`SELECT id, name, COALESCE(email, ''), vendor, contract_type, status, metadata, created_at, updated_at
		 FROM external_members WHERE id = $1`, id,
	).Scan(&member.ID, &member.Name, &member.Email, &member.Vendor, &member.ContractType, &member.Status, &metadataJSON, &member.CreatedAt, &member.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get external member: %w", err)
	}
	if err := json.Unmarshal(metadataJSON, &member.Metadata); err != nil {
		return nil, fmt.Errorf("unmarshal external member metadata: %w", err)
	}
	return member, nil
}

func (r *PostgresRepository) ListExternalMembers(ctx context.Context, limit int) ([]ExternalMember, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.Query(ctx,
		`SELECT id, name, COALESCE(email, ''), vendor, contract_type, status, metadata, created_at, updated_at
		 FROM external_members ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list external members: %w", err)
	}
	defer rows.Close()

	var members []ExternalMember
	for rows.Next() {
		var member ExternalMember
		var metadataJSON []byte
		if err := rows.Scan(&member.ID, &member.Name, &member.Email, &member.Vendor, &member.ContractType, &member.Status, &metadataJSON, &member.CreatedAt, &member.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan external member: %w", err)
		}
		if err := json.Unmarshal(metadataJSON, &member.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal external member metadata: %w", err)
		}
		members = append(members, member)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list external members iteration: %w", err)
	}
	return members, nil
}

func (r *PostgresRepository) UpdateExternalMember(ctx context.Context, id uuid.UUID, input UpdateExternalMemberInput) (*ExternalMember, error) {
	var metadataJSON []byte
	var err error
	if input.Metadata != nil {
		metadataJSON, err = json.Marshal(input.Metadata)
		if err != nil {
			return nil, fmt.Errorf("marshal external member metadata: %w", err)
		}
	}

	_, err = r.db.Exec(ctx,
		`UPDATE external_members SET
			name = COALESCE(NULLIF($2, ''), name),
			email = COALESCE(NULLIF($3, ''), email),
			vendor = COALESCE(NULLIF($4, ''), vendor),
			contract_type = COALESCE(NULLIF($5, ''), contract_type),
			status = COALESCE(NULLIF($6, ''), status),
			metadata = COALESCE($7::jsonb, metadata),
			updated_at = NOW()
		 WHERE id = $1`,
		id, input.Name, input.Email, input.Vendor, input.ContractType, input.Status, metadataJSON)
	if err != nil {
		return nil, fmt.Errorf("update external member: %w", err)
	}
	return r.GetExternalMemberByID(ctx, id)
}

func (r *PostgresRepository) AddOrganizationMember(ctx context.Context, input AddOrganizationMemberInput) (*OrganizationMembership, error) {
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
	if input.Status == "" {
		input.Status = "active"
	}
	if input.AuthorityTier == "" {
		input.AuthorityTier = AuthorityExecutor
	}
	metadataJSON, err := json.Marshal(input.Metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal membership metadata: %w", err)
	}

	var membershipID uuid.UUID
	switch input.MemberType {
	case "internal":
		userID := *input.UserID
		err = r.db.QueryRow(ctx,
			`INSERT INTO organization_memberships (organization_id, department_id, member_type, user_id, title, role_id, authority_tier, status, metadata)
			 SELECT organization_id, id, $2, $3, $4, $5, $6, $7, $8 FROM departments WHERE id = $1
			 ON CONFLICT (department_id, user_id) WHERE member_type = 'internal'
			 DO UPDATE SET title = EXCLUDED.title, role_id = EXCLUDED.role_id, authority_tier = EXCLUDED.authority_tier, status = EXCLUDED.status, metadata = EXCLUDED.metadata, updated_at = NOW()
			 RETURNING id`,
			input.DepartmentID, input.MemberType, userID, input.Title, input.RoleID, input.AuthorityTier, input.Status, metadataJSON,
		).Scan(&membershipID)
	case "external":
		externalMemberID := *input.ExternalMemberID
		err = r.db.QueryRow(ctx,
			`INSERT INTO organization_memberships (organization_id, department_id, member_type, external_member_id, title, role_id, authority_tier, status, metadata)
			 SELECT organization_id, id, $2, $3, $4, $5, $6, $7, $8 FROM departments WHERE id = $1
			 ON CONFLICT (department_id, external_member_id) WHERE member_type = 'external'
			 DO UPDATE SET title = EXCLUDED.title, role_id = EXCLUDED.role_id, authority_tier = EXCLUDED.authority_tier, status = EXCLUDED.status, metadata = EXCLUDED.metadata, updated_at = NOW()
			 RETURNING id`,
			input.DepartmentID, input.MemberType, externalMemberID, input.Title, input.RoleID, input.AuthorityTier, input.Status, metadataJSON,
		).Scan(&membershipID)
	case "agent":
		agentID := *input.AgentID
		err = r.db.QueryRow(ctx,
			`INSERT INTO organization_memberships (organization_id, department_id, member_type, agent_id, title, role_id, authority_tier, status, metadata)
			 SELECT organization_id, id, $2, $3, $4, $5, $6, $7, $8 FROM departments WHERE id = $1
			 ON CONFLICT (department_id, agent_id) WHERE member_type = 'agent'
			 DO UPDATE SET title = EXCLUDED.title, role_id = EXCLUDED.role_id, authority_tier = EXCLUDED.authority_tier, status = EXCLUDED.status, metadata = EXCLUDED.metadata, updated_at = NOW()
			 RETURNING id`,
			input.DepartmentID, input.MemberType, agentID, input.Title, input.RoleID, input.AuthorityTier, input.Status, metadataJSON,
		).Scan(&membershipID)
	default:
		return nil, fmt.Errorf("unsupported member type: %s", input.MemberType)
	}
	if err != nil {
		return nil, fmt.Errorf("add organization member: %w", err)
	}
	return r.GetOrganizationMembershipByID(ctx, membershipID)
}

func (r *PostgresRepository) GetOrganizationMembershipByID(ctx context.Context, id uuid.UUID) (*OrganizationMembership, error) {
	rows, err := r.db.Query(ctx, membershipSelectSQL()+` WHERE om.id = $1`, id)
	if err != nil {
		return nil, fmt.Errorf("get organization membership: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, fmt.Errorf("get organization membership: not found")
	}
	membership, err := scanOrganizationMembership(rows.Scan)
	if err != nil {
		return nil, err
	}
	return membership, rows.Err()
}

func (r *PostgresRepository) ListOrganizationMemberships(ctx context.Context, orgID uuid.UUID, departmentID *uuid.UUID, memberTypes []string) ([]OrganizationMembership, error) {
	query := membershipSelectSQL() + `
		WHERE om.organization_id = $1
		  AND ($2::uuid IS NULL OR om.department_id = $2)
		  AND (cardinality($3::text[]) = 0 OR om.member_type = ANY($3))
		ORDER BY om.department_id, om.member_type, member_name`
	rows, err := r.db.Query(ctx, query, orgID, departmentID, memberTypes)
	if err != nil {
		return nil, fmt.Errorf("list organization memberships: %w", err)
	}
	defer rows.Close()

	var memberships []OrganizationMembership
	for rows.Next() {
		membership, err := scanOrganizationMembership(rows.Scan)
		if err != nil {
			return nil, err
		}
		memberships = append(memberships, *membership)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list organization memberships iteration: %w", err)
	}
	return memberships, nil
}

func (r *PostgresRepository) UpdateOrganizationMembership(ctx context.Context, id uuid.UUID, input UpdateOrganizationMembershipInput) (*OrganizationMembership, error) {
	var metadataJSON []byte
	var err error
	if input.Metadata != nil {
		metadataJSON, err = json.Marshal(input.Metadata)
		if err != nil {
			return nil, fmt.Errorf("marshal membership metadata: %w", err)
		}
	}
	_, err = r.db.Exec(ctx,
		`UPDATE organization_memberships SET
			title = COALESCE(NULLIF($2, ''), title),
			role_id = COALESCE($3, role_id),
			authority_tier = COALESCE(NULLIF($4::text, ''), authority_tier),
			status = COALESCE(NULLIF($5, ''), status),
			metadata = COALESCE($6::jsonb, metadata),
			updated_at = NOW()
		 WHERE id = $1`,
		id, input.Title, input.RoleID, input.AuthorityTier, input.Status, metadataJSON)
	if err != nil {
		return nil, fmt.Errorf("update organization membership: %w", err)
	}
	return r.GetOrganizationMembershipByID(ctx, id)
}

func (r *PostgresRepository) RemoveOrganizationMembership(ctx context.Context, id uuid.UUID) error {
	if _, err := r.db.Exec(ctx, `DELETE FROM organization_memberships WHERE id = $1`, id); err != nil {
		return fmt.Errorf("remove organization membership: %w", err)
	}
	return nil
}

func (r *PostgresRepository) CountActiveOrganizationOwners(ctx context.Context, orgID uuid.UUID, excludeMembershipID *uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*)
		 FROM organization_memberships
		 WHERE organization_id = $1
		   AND authority_tier = $2
		   AND status = 'active'
		   AND ($3::uuid IS NULL OR id <> $3)`,
		orgID, AuthorityOrganizationCreator, excludeMembershipID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count active organization owners: %w", err)
	}
	return count, nil
}

func (r *PostgresRepository) CreatePermissionChangeRequest(ctx context.Context, input CreatePermissionChangeRequestRecord) (*PermissionChangeRequest, error) {
	changeJSON, err := json.Marshal(input.RequestedChange)
	if err != nil {
		return nil, fmt.Errorf("marshal permission change: %w", err)
	}
	req := &PermissionChangeRequest{}
	err = scanPermissionChangeRequest(r.db.QueryRow(ctx,
		`INSERT INTO permission_change_requests (
		    organization_id, membership_id, requested_by, requested_by_type, requested_change, reason
		 )
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, organization_id, membership_id, requested_by, requested_by_type,
		           requested_change, reason, status, reviewed_by, review_reason,
		           reviewed_at, applied_at, created_at, updated_at`,
		input.OrganizationID, input.MembershipID, input.RequestedBy, input.RequestedByType, changeJSON, input.Reason,
	).Scan, req)
	if err != nil {
		return nil, fmt.Errorf("create permission change request: %w", err)
	}
	return req, nil
}

func (r *PostgresRepository) GetPermissionChangeRequestByID(ctx context.Context, id uuid.UUID) (*PermissionChangeRequest, error) {
	req := &PermissionChangeRequest{}
	if err := scanPermissionChangeRequest(r.db.QueryRow(ctx,
		`SELECT id, organization_id, membership_id, requested_by, requested_by_type,
		        requested_change, reason, status, reviewed_by, review_reason,
		        reviewed_at, applied_at, created_at, updated_at
		 FROM permission_change_requests
		 WHERE id = $1`, id,
	).Scan, req); err != nil {
		return nil, fmt.Errorf("get permission change request: %w", err)
	}
	return req, nil
}

func (r *PostgresRepository) ListPermissionChangeRequests(ctx context.Context, orgID uuid.UUID, status string, limit int) ([]PermissionChangeRequest, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.Query(ctx,
		`SELECT id, organization_id, membership_id, requested_by, requested_by_type,
		        requested_change, reason, status, reviewed_by, review_reason,
		        reviewed_at, applied_at, created_at, updated_at
		 FROM permission_change_requests
		 WHERE organization_id = $1
		   AND ($2 = '' OR status = $2)
		 ORDER BY created_at DESC
		 LIMIT $3`, orgID, status, limit)
	if err != nil {
		return nil, fmt.Errorf("list permission change requests: %w", err)
	}
	defer rows.Close()
	requests := []PermissionChangeRequest{}
	for rows.Next() {
		var req PermissionChangeRequest
		if err := scanPermissionChangeRequest(rows.Scan, &req); err != nil {
			return nil, err
		}
		requests = append(requests, req)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list permission change requests iteration: %w", err)
	}
	return requests, nil
}

func (r *PostgresRepository) UpdatePermissionChangeRequestStatus(ctx context.Context, id uuid.UUID, input UpdatePermissionChangeRequestStatusInput) (*PermissionChangeRequest, error) {
	req := &PermissionChangeRequest{}
	err := scanPermissionChangeRequest(r.db.QueryRow(ctx,
		`UPDATE permission_change_requests
		 SET status = $2,
		     reviewed_by = COALESCE($3, reviewed_by),
		     review_reason = COALESCE(NULLIF($4, ''), review_reason),
		     reviewed_at = CASE WHEN $2 IN ('rejected', 'applied', 'approved') THEN COALESCE(reviewed_at, NOW()) ELSE reviewed_at END,
		     applied_at = CASE WHEN $2 = 'applied' THEN COALESCE(applied_at, NOW()) ELSE applied_at END,
		     updated_at = NOW()
		 WHERE id = $1
		 RETURNING id, organization_id, membership_id, requested_by, requested_by_type,
		           requested_change, reason, status, reviewed_by, review_reason,
		           reviewed_at, applied_at, created_at, updated_at`,
		id, input.Status, input.ReviewedBy, input.ReviewReason,
	).Scan, req)
	if err != nil {
		return nil, fmt.Errorf("update permission change request: %w", err)
	}
	return req, nil
}

func (r *PostgresRepository) CreateAccessRule(ctx context.Context, input CreateAccessRuleRecord) (*OrganizationAccessRule, error) {
	metadataJSON, err := json.Marshal(input.Metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal organization access rule metadata: %w", err)
	}
	rule := &OrganizationAccessRule{}
	err = scanOrganizationAccessRule(r.db.QueryRow(ctx,
		`INSERT INTO organization_access_rules (
		    organization_id, scope_type, scope_id, resource_type, resource_key, action,
		    actor_type, actor_id, role_id, authority_tier, behavior, required_level,
		    priority, reason, metadata, created_by
		 )
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		 RETURNING id, organization_id, scope_type, scope_id, resource_type, resource_key, action,
		           actor_type, actor_id, role_id, authority_tier, behavior, required_level,
		           priority, status, reason, metadata, created_by, created_at, updated_at`,
		input.OrganizationID, input.ScopeType, input.ScopeID, input.ResourceType, input.ResourceKey, input.Action,
		input.ActorType, input.ActorID, input.RoleID, input.AuthorityTier, input.Behavior, input.RequiredLevel,
		input.Priority, input.Reason, metadataJSON, input.CreatedBy,
	).Scan, rule)
	if err != nil {
		return nil, fmt.Errorf("create organization access rule: %w", err)
	}
	return rule, nil
}

func (r *PostgresRepository) ListAccessRules(ctx context.Context, orgID uuid.UUID, scopeType string, resourceType string, limit int) ([]OrganizationAccessRule, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.Query(ctx,
		`SELECT id, organization_id, scope_type, scope_id, resource_type, resource_key, action,
		        actor_type, actor_id, role_id, authority_tier, behavior, required_level,
		        priority, status, reason, metadata, created_by, created_at, updated_at
		 FROM organization_access_rules
		 WHERE organization_id = $1
		   AND ($2 = '' OR scope_type = $2)
		   AND ($3 = '' OR resource_type = $3)
		 ORDER BY priority DESC, created_at DESC
		 LIMIT $4`, orgID, scopeType, resourceType, limit)
	if err != nil {
		return nil, fmt.Errorf("list organization access rules: %w", err)
	}
	defer rows.Close()
	rules := []OrganizationAccessRule{}
	for rows.Next() {
		var rule OrganizationAccessRule
		if err := scanOrganizationAccessRule(rows.Scan, &rule); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list organization access rules iteration: %w", err)
	}
	return rules, nil
}

func (r *PostgresRepository) LinkDepartmentMVRU(ctx context.Context, input LinkDepartmentMVRUInput) (*DepartmentMVRULink, error) {
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
	if input.LinkType == "" {
		input.LinkType = "execution"
	}
	metadataJSON, err := json.Marshal(input.Metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal department mvru link metadata: %w", err)
	}
	_, err = r.db.Exec(ctx,
		`INSERT INTO department_mvru_links (department_id, mvru_id, link_type, metadata)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (department_id, mvru_id, link_type) DO UPDATE SET metadata = EXCLUDED.metadata`,
		input.DepartmentID, input.MVRUID, input.LinkType, metadataJSON)
	if err != nil {
		return nil, fmt.Errorf("link department mvru: %w", err)
	}
	links, err := r.ListDepartmentMVRULinks(ctx, input.DepartmentID)
	if err != nil {
		return nil, err
	}
	for _, link := range links {
		if link.MVRUID == input.MVRUID && link.LinkType == input.LinkType {
			return &link, nil
		}
	}
	return nil, fmt.Errorf("link department mvru: not found after upsert")
}

func (r *PostgresRepository) ListDepartmentMVRULinks(ctx context.Context, departmentID uuid.UUID) ([]DepartmentMVRULink, error) {
	rows, err := r.db.Query(ctx,
		`SELECT l.department_id, l.mvru_id, m.name, l.link_type, l.metadata, l.created_at
		 FROM department_mvru_links l
		 JOIN muvrs m ON m.id = l.mvru_id
		 WHERE l.department_id = $1
		 ORDER BY l.created_at DESC`, departmentID)
	if err != nil {
		return nil, fmt.Errorf("list department mvru links: %w", err)
	}
	defer rows.Close()

	var links []DepartmentMVRULink
	for rows.Next() {
		var link DepartmentMVRULink
		var metadataJSON []byte
		if err := rows.Scan(&link.DepartmentID, &link.MVRUID, &link.MVRUName, &link.LinkType, &metadataJSON, &link.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan department mvru link: %w", err)
		}
		if err := json.Unmarshal(metadataJSON, &link.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal department mvru link metadata: %w", err)
		}
		links = append(links, link)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list department mvru links iteration: %w", err)
	}
	return links, nil
}

type scanFunc func(dest ...any) error

func scanPosition(scan scanFunc, position *Position) error {
	var capabilitiesJSON, metadataJSON []byte
	if err := scan(
		&position.ID,
		&position.OrganizationID,
		&position.DepartmentID,
		&position.Name,
		&position.Code,
		&position.Description,
		&position.Status,
		&position.SortOrder,
		&position.PermissionLevel,
		&capabilitiesJSON,
		&metadataJSON,
		&position.CreatedAt,
		&position.UpdatedAt,
	); err != nil {
		return fmt.Errorf("scan position: %w", err)
	}
	if err := json.Unmarshal(capabilitiesJSON, &position.RequiredCapabilities); err != nil {
		return fmt.Errorf("unmarshal position capabilities: %w", err)
	}
	if err := json.Unmarshal(metadataJSON, &position.Metadata); err != nil {
		return fmt.Errorf("unmarshal position metadata: %w", err)
	}
	return nil
}

func assignmentSelectSQL() string {
	return `SELECT
		pa.id,
		pa.position_id,
		pa.organization_id,
		pa.department_id,
		pa.meta_resource_id,
		COALESCE(mr.name, '') AS meta_resource_name,
		COALESCE(mr.resource_type, '') AS meta_resource_type,
		pa.actor_id,
		pa.actor_type,
		COALESCE(u.name, em.name, aa.name, '') AS actor_name,
		COALESCE(u.email, em.email, '') AS actor_email,
		pa.assignment_type,
		pa.allocation_percent,
		pa.status,
		pa.metadata,
		pa.created_at,
		pa.updated_at
	 FROM position_assignments pa
	 LEFT JOIN meta_resources mr ON mr.id = pa.meta_resource_id
	 LEFT JOIN users u ON pa.actor_type = 'internal_human' AND u.id = pa.actor_id
	 LEFT JOIN external_members em ON pa.actor_type = 'external_human' AND em.id = pa.actor_id
	 LEFT JOIN ai_agents aa ON pa.actor_type IN ('internal_agent', 'external_agent') AND aa.id = pa.actor_id`
}

func scanPositionAssignment(scan scanFunc, assignment *PositionAssignment) error {
	var metadataJSON []byte
	if err := scan(
		&assignment.ID,
		&assignment.PositionID,
		&assignment.OrganizationID,
		&assignment.DepartmentID,
		&assignment.MetaResourceID,
		&assignment.MetaResourceName,
		&assignment.MetaResourceType,
		&assignment.ActorID,
		&assignment.ActorType,
		&assignment.ActorName,
		&assignment.ActorEmail,
		&assignment.AssignmentType,
		&assignment.AllocationPercent,
		&assignment.Status,
		&metadataJSON,
		&assignment.CreatedAt,
		&assignment.UpdatedAt,
	); err != nil {
		return fmt.Errorf("scan position assignment: %w", err)
	}
	if err := json.Unmarshal(metadataJSON, &assignment.Metadata); err != nil {
		return fmt.Errorf("unmarshal position assignment metadata: %w", err)
	}
	return nil
}

func scanDepartment(scan scanFunc) (*Department, error) {
	dept := &Department{}
	var metadataJSON []byte
	if err := scan(&dept.ID, &dept.OrganizationID, &dept.ParentID, &dept.Name, &dept.Code, &dept.Description, &dept.Status, &dept.SortOrder, &metadataJSON, &dept.CreatedAt, &dept.UpdatedAt); err != nil {
		return nil, fmt.Errorf("scan department: %w", err)
	}
	if err := json.Unmarshal(metadataJSON, &dept.Metadata); err != nil {
		return nil, fmt.Errorf("unmarshal department metadata: %w", err)
	}
	return dept, nil
}

func membershipSelectSQL() string {
	return `SELECT
		om.id,
		om.organization_id,
		om.department_id,
		om.member_type,
		om.user_id,
		om.external_member_id,
		om.agent_id,
		COALESCE(u.name, em.name, aa.name, '') AS member_name,
		COALESCE(u.email, em.email, '') AS member_email,
		om.title,
		om.role_id,
		COALESCE(r.name, '') AS role_name,
		om.authority_tier,
		om.status,
		om.joined_at,
		om.metadata,
		om.created_at,
		om.updated_at
	 FROM organization_memberships om
	 LEFT JOIN users u ON u.id = om.user_id
	 LEFT JOIN external_members em ON em.id = om.external_member_id
	 LEFT JOIN ai_agents aa ON aa.id = om.agent_id
	 LEFT JOIN roles r ON r.id = om.role_id`
}

func scanOrganizationMembership(scan scanFunc) (*OrganizationMembership, error) {
	membership := &OrganizationMembership{}
	var metadataJSON []byte
	if err := scan(
		&membership.ID,
		&membership.OrganizationID,
		&membership.DepartmentID,
		&membership.MemberType,
		&membership.UserID,
		&membership.ExternalMemberID,
		&membership.AgentID,
		&membership.MemberName,
		&membership.MemberEmail,
		&membership.Title,
		&membership.RoleID,
		&membership.RoleName,
		&membership.AuthorityTier,
		&membership.Status,
		&membership.JoinedAt,
		&metadataJSON,
		&membership.CreatedAt,
		&membership.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan organization membership: %w", err)
	}
	if err := json.Unmarshal(metadataJSON, &membership.Metadata); err != nil {
		return nil, fmt.Errorf("unmarshal organization membership metadata: %w", err)
	}
	return membership, nil
}

func scanPermissionChangeRequest(scan scanFunc, req *PermissionChangeRequest) error {
	var changeJSON []byte
	var requestedBy *uuid.UUID
	if err := scan(
		&req.ID,
		&req.OrganizationID,
		&req.MembershipID,
		&requestedBy,
		&req.RequestedByType,
		&changeJSON,
		&req.Reason,
		&req.Status,
		&req.ReviewedBy,
		&req.ReviewReason,
		&req.ReviewedAt,
		&req.AppliedAt,
		&req.CreatedAt,
		&req.UpdatedAt,
	); err != nil {
		return fmt.Errorf("scan permission change request: %w", err)
	}
	if err := json.Unmarshal(changeJSON, &req.RequestedChange); err != nil {
		return fmt.Errorf("unmarshal permission change request: %w", err)
	}
	req.RequestedBy = requestedBy
	return nil
}

func scanOrganizationAccessRule(scan scanFunc, rule *OrganizationAccessRule) error {
	var metadataJSON []byte
	if err := scan(
		&rule.ID,
		&rule.OrganizationID,
		&rule.ScopeType,
		&rule.ScopeID,
		&rule.ResourceType,
		&rule.ResourceKey,
		&rule.Action,
		&rule.ActorType,
		&rule.ActorID,
		&rule.RoleID,
		&rule.AuthorityTier,
		&rule.Behavior,
		&rule.RequiredLevel,
		&rule.Priority,
		&rule.Status,
		&rule.Reason,
		&metadataJSON,
		&rule.CreatedBy,
		&rule.CreatedAt,
		&rule.UpdatedAt,
	); err != nil {
		return fmt.Errorf("scan organization access rule: %w", err)
	}
	if err := json.Unmarshal(metadataJSON, &rule.Metadata); err != nil {
		return fmt.Errorf("unmarshal organization access rule metadata: %w", err)
	}
	return nil
}

func uuidPointer(value pgtype.UUID) *uuid.UUID {
	if !value.Valid {
		return nil
	}
	id := uuid.UUID(value.Bytes)
	return &id
}
