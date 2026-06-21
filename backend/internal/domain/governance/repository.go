package governance

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreatePermission(ctx context.Context, p *Permission) (*Permission, error) {
	perm := &Permission{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO permissions (level, name, description, behavior)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, level, name, description, behavior, created_at`,
		p.Level, p.Name, p.Description, p.Behavior,
	).Scan(&perm.ID, &perm.Level, &perm.Name, &perm.Description, &perm.Behavior, &perm.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create permission: %w", err)
	}
	return perm, nil
}

func (r *Repository) ListPermissions(ctx context.Context) ([]Permission, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, level, name, description, behavior, created_at
		 FROM permissions ORDER BY level, name`)
	if err != nil {
		return nil, fmt.Errorf("list permissions: %w", err)
	}
	defer rows.Close()

	perms := make([]Permission, 0)
	for rows.Next() {
		var p Permission
		if err := rows.Scan(&p.ID, &p.Level, &p.Name, &p.Description, &p.Behavior, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan permission: %w", err)
		}
		perms = append(perms, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list permissions iteration: %w", err)
	}
	return perms, nil
}

func (r *Repository) GetPermissionByLevel(ctx context.Context, level int) (*Permission, error) {
	p := &Permission{}
	err := r.db.QueryRow(ctx,
		`SELECT id, level, name, description, behavior, created_at
		 FROM permissions WHERE level = $1`, level,
	).Scan(&p.ID, &p.Level, &p.Name, &p.Description, &p.Behavior, &p.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get permission by level: %w", err)
	}
	return p, nil
}

func (r *Repository) CreatePrinciple(ctx context.Context, input CreatePrincipleInput) (*Principle, error) {
	evalJSON, _ := json.Marshal(input.EvaluationLogic)
	prin := &Principle{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO principles (name, description, evaluation_logic, priority)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, name, description, evaluation_logic, priority, is_active, created_at, updated_at`,
		input.Name, input.Description, evalJSON, input.Priority,
	).Scan(&prin.ID, &prin.Name, &prin.Description, &evalJSON, &prin.Priority, &prin.IsActive, &prin.CreatedAt, &prin.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create principle: %w", err)
	}
	json.Unmarshal(evalJSON, &prin.EvaluationLogic)
	return prin, nil
}

func (r *Repository) ListPrinciples(ctx context.Context) ([]Principle, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, name, description, evaluation_logic, priority, is_active, created_at, updated_at
		 FROM principles ORDER BY priority DESC, name`)
	if err != nil {
		return nil, fmt.Errorf("list principles: %w", err)
	}
	defer rows.Close()

	principles := make([]Principle, 0)
	for rows.Next() {
		var p Principle
		var evalJSON []byte
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &evalJSON, &p.Priority, &p.IsActive, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan principle: %w", err)
		}
		json.Unmarshal(evalJSON, &p.EvaluationLogic)
		principles = append(principles, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list principles iteration: %w", err)
	}
	return principles, nil
}

func (r *Repository) GetPrinciple(ctx context.Context, id uuid.UUID) (*Principle, error) {
	p := &Principle{}
	var evalJSON []byte
	err := r.db.QueryRow(ctx,
		`SELECT id, name, description, evaluation_logic, priority, is_active, created_at, updated_at
		 FROM principles WHERE id = $1`, id,
	).Scan(&p.ID, &p.Name, &p.Description, &evalJSON, &p.Priority, &p.IsActive, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get principle: %w", err)
	}
	json.Unmarshal(evalJSON, &p.EvaluationLogic)
	return p, nil
}

func (r *Repository) CreateControlRule(ctx context.Context, input CreateControlRuleInput) (*ControlRule, error) {
	condJSON, _ := json.Marshal(input.Condition)
	rule := &ControlRule{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO control_rules (principle_id, target_entity_type, target_entity_id, condition, action, priority)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, principle_id, target_entity_type, target_entity_id, condition, action, priority, is_active, created_at`,
		input.PrincipleID, input.TargetEntityType, input.TargetEntityID, condJSON, input.Action, input.Priority,
	).Scan(&rule.ID, &rule.PrincipleID, &rule.TargetEntityType, &rule.TargetEntityID, &condJSON, &rule.Action, &rule.Priority, &rule.IsActive, &rule.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create control rule: %w", err)
	}
	json.Unmarshal(condJSON, &rule.Condition)
	return rule, nil
}

func (r *Repository) ListControlRules(ctx context.Context) ([]ControlRule, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, principle_id, target_entity_type, target_entity_id, condition, action, priority, is_active, created_at
		 FROM control_rules ORDER BY priority DESC`)
	if err != nil {
		return nil, fmt.Errorf("list control rules: %w", err)
	}
	defer rows.Close()

	rules := make([]ControlRule, 0)
	for rows.Next() {
		var rule ControlRule
		var condJSON []byte
		if err := rows.Scan(&rule.ID, &rule.PrincipleID, &rule.TargetEntityType, &rule.TargetEntityID, &condJSON, &rule.Action, &rule.Priority, &rule.IsActive, &rule.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan control rule: %w", err)
		}
		json.Unmarshal(condJSON, &rule.Condition)
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list control rules iteration: %w", err)
	}
	return rules, nil
}

func (r *Repository) GetControlRulesByTarget(ctx context.Context, entityType string, entityID *uuid.UUID) ([]ControlRule, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, principle_id, target_entity_type, target_entity_id, condition, action, priority, is_active, created_at
		 FROM control_rules
		 WHERE target_entity_type = $1 AND (target_entity_id = $2 OR target_entity_id IS NULL)
		 ORDER BY priority DESC`, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("get control rules by target: %w", err)
	}
	defer rows.Close()

	rules := make([]ControlRule, 0)
	for rows.Next() {
		var rule ControlRule
		var condJSON []byte
		if err := rows.Scan(&rule.ID, &rule.PrincipleID, &rule.TargetEntityType, &rule.TargetEntityID, &condJSON, &rule.Action, &rule.Priority, &rule.IsActive, &rule.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan control rule: %w", err)
		}
		json.Unmarshal(condJSON, &rule.Condition)
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get control rules by target iteration: %w", err)
	}
	return rules, nil
}

func (r *Repository) CreateAccessDecision(ctx context.Context, input AccessDecisionInput, decision, behavior, reason string, allowed bool, matchedRules []string) (*AccessDecision, error) {
	if input.Context == nil {
		input.Context = map[string]any{}
	}
	rulesJSON, _ := json.Marshal(matchedRules)
	contextJSON, _ := json.Marshal(input.Context)

	access := &AccessDecision{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO access_decisions (
		    actor_id, actor_type, action, resource, resource_id, organization_id, department_id,
		    workflow_id, task_id, capability_id, required_level, risk_level, decision, allowed,
		    behavior, reason, matched_rules, weight_snapshot, context
		 )
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
		 RETURNING id, master_key, actor_id, actor_type, action, resource, resource_id, organization_id, department_id,
		           workflow_id, task_id, capability_id, required_level, risk_level, decision, allowed,
		           behavior, reason, matched_rules, weight_snapshot, context, created_at`,
		input.ActorID, input.ActorType, input.Action, input.Resource, input.ResourceID, input.OrganizationID, input.DepartmentID,
		input.WorkflowID, input.TaskID, input.CapabilityID, input.RequiredLevel, input.RiskLevel, decision, allowed,
		behavior, reason, rulesJSON, input.WeightSnapshot, contextJSON,
	).Scan(&access.ID, &access.MasterKey, &access.ActorID, &access.ActorType, &access.Action, &access.Resource, &access.ResourceID, &access.OrganizationID, &access.DepartmentID,
		&access.WorkflowID, &access.TaskID, &access.CapabilityID, &access.RequiredLevel, &access.RiskLevel, &access.Decision, &access.Allowed,
		&access.Behavior, &access.Reason, &rulesJSON, &access.WeightSnapshot, &contextJSON, &access.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create access decision: %w", err)
	}
	json.Unmarshal(rulesJSON, &access.MatchedRules)
	json.Unmarshal(contextJSON, &access.Context)
	return access, nil
}

func (r *Repository) ListAccessDecisions(ctx context.Context, limit int) ([]AccessDecision, error) {
	if limit <= 0 {
		limit = 50
	} else if limit > 100 {
		limit = 100
	}
	rows, err := r.db.Query(ctx,
		`SELECT id, master_key, actor_id, actor_type, action, resource, resource_id, organization_id, department_id,
		        workflow_id, task_id, capability_id, required_level, risk_level, decision, allowed,
		        behavior, reason, matched_rules, weight_snapshot, context, created_at
		 FROM access_decisions
		 WHERE ($1::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $1)
		 ORDER BY created_at DESC LIMIT $2`, nullableUUID(currentTenantOrganizationID(ctx)), limit)
	if err != nil {
		return nil, fmt.Errorf("list access decisions: %w", err)
	}
	defer rows.Close()

	decisions := make([]AccessDecision, 0)
	for rows.Next() {
		var decision AccessDecision
		var rulesJSON, contextJSON []byte
		if err := rows.Scan(&decision.ID, &decision.MasterKey, &decision.ActorID, &decision.ActorType, &decision.Action, &decision.Resource, &decision.ResourceID, &decision.OrganizationID, &decision.DepartmentID,
			&decision.WorkflowID, &decision.TaskID, &decision.CapabilityID, &decision.RequiredLevel, &decision.RiskLevel, &decision.Decision, &decision.Allowed,
			&decision.Behavior, &decision.Reason, &rulesJSON, &decision.WeightSnapshot, &contextJSON, &decision.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan access decision: %w", err)
		}
		json.Unmarshal(rulesJSON, &decision.MatchedRules)
		json.Unmarshal(contextJSON, &decision.Context)
		decisions = append(decisions, decision)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list access decisions iteration: %w", err)
	}
	return decisions, nil
}

func (r *Repository) ListDataTables(ctx context.Context, category string) ([]DataTable, error) {
	query := `SELECT table_name, master_table_name, detail_table_name, key_prefix, display_name, category,
	                 is_base_data, is_business_scenario, metadata, created_at, updated_at
	          FROM data_table_catalog`
	args := []any{}
	if category != "" {
		query += ` WHERE category = $1`
		args = append(args, category)
	}
	query += ` ORDER BY category, display_name, table_name`

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list data tables: %w", err)
	}
	defer rows.Close()

	tables := make([]DataTable, 0)
	for rows.Next() {
		var table DataTable
		var metadataJSON []byte
		if err := rows.Scan(
			&table.TableName,
			&table.MasterTableName,
			&table.DetailTableName,
			&table.KeyPrefix,
			&table.DisplayName,
			&table.Category,
			&table.IsBaseData,
			&table.IsBusinessScenario,
			&metadataJSON,
			&table.CreatedAt,
			&table.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan data table: %w", err)
		}
		_ = json.Unmarshal(metadataJSON, &table.Metadata)
		tables = append(tables, table)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list data tables iteration: %w", err)
	}
	return tables, nil
}

func (r *Repository) ListDataFields(ctx context.Context, tableName string) ([]DataField, error) {
	rows, err := r.db.Query(ctx,
		`SELECT table_name, field_name, data_type, display_name, is_master_key, is_sub_key,
		        is_visible_default, permission_level, display_order, metadata, created_at, updated_at
		 FROM data_field_catalog
		 WHERE table_name = $1
		 ORDER BY display_order, field_name`, tableName)
	if err != nil {
		return nil, fmt.Errorf("list data fields: %w", err)
	}
	defer rows.Close()

	fields := make([]DataField, 0)
	for rows.Next() {
		var field DataField
		var metadataJSON []byte
		if err := rows.Scan(
			&field.TableName,
			&field.FieldName,
			&field.DataType,
			&field.DisplayName,
			&field.IsMasterKey,
			&field.IsSubKey,
			&field.IsVisibleDefault,
			&field.PermissionLevel,
			&field.DisplayOrder,
			&metadataJSON,
			&field.CreatedAt,
			&field.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan data field: %w", err)
		}
		_ = json.Unmarshal(metadataJSON, &field.Metadata)
		fields = append(fields, field)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list data fields iteration: %w", err)
	}
	return fields, nil
}

func (r *Repository) GetUserFieldPreference(ctx context.Context, actorID, tableName string) (*UserFieldPreference, error) {
	pref := &UserFieldPreference{}
	var visibleJSON, orderJSON, widthsJSON []byte
	err := r.db.QueryRow(ctx,
		`SELECT actor_id, table_name, visible_fields, field_order, field_widths, created_at, updated_at
		 FROM user_field_preferences
		 WHERE actor_id = $1 AND table_name = $2`, actorID, tableName,
	).Scan(&pref.ActorID, &pref.TableName, &visibleJSON, &orderJSON, &widthsJSON, &pref.CreatedAt, &pref.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get user field preference: %w", err)
	}
	_ = json.Unmarshal(visibleJSON, &pref.VisibleFields)
	_ = json.Unmarshal(orderJSON, &pref.FieldOrder)
	_ = json.Unmarshal(widthsJSON, &pref.FieldWidths)
	return pref, nil
}

func (r *Repository) UpsertUserFieldPreference(ctx context.Context, actorID, tableName string, input UpsertUserFieldPreferenceInput) (*UserFieldPreference, error) {
	visibleJSON, _ := json.Marshal(input.VisibleFields)
	orderJSON, _ := json.Marshal(input.FieldOrder)
	widthsJSON, _ := json.Marshal(input.FieldWidths)

	pref := &UserFieldPreference{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO user_field_preferences(actor_id, table_name, visible_fields, field_order, field_widths)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (actor_id, table_name) DO UPDATE SET
		    visible_fields = EXCLUDED.visible_fields,
		    field_order = EXCLUDED.field_order,
		    field_widths = EXCLUDED.field_widths,
		    updated_at = NOW()
		 RETURNING actor_id, table_name, visible_fields, field_order, field_widths, created_at, updated_at`,
		actorID, tableName, visibleJSON, orderJSON, widthsJSON,
	).Scan(&pref.ActorID, &pref.TableName, &visibleJSON, &orderJSON, &widthsJSON, &pref.CreatedAt, &pref.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("upsert user field preference: %w", err)
	}
	_ = json.Unmarshal(visibleJSON, &pref.VisibleFields)
	_ = json.Unmarshal(orderJSON, &pref.FieldOrder)
	_ = json.Unmarshal(widthsJSON, &pref.FieldWidths)
	return pref, nil
}

func (r *Repository) GetUserUIPreference(ctx context.Context, actorID, preferenceKey string) (*UserUIPreference, error) {
	pref := &UserUIPreference{}
	var valueJSON []byte
	err := r.db.QueryRow(ctx,
		`SELECT actor_id, preference_key, value, created_at, updated_at
		 FROM user_ui_preferences
		 WHERE actor_id = $1 AND preference_key = $2`, actorID, preferenceKey,
	).Scan(&pref.ActorID, &pref.PreferenceKey, &valueJSON, &pref.CreatedAt, &pref.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get user ui preference: %w", err)
	}
	_ = json.Unmarshal(valueJSON, &pref.Value)
	return pref, nil
}

func (r *Repository) UpsertUserUIPreference(ctx context.Context, actorID, preferenceKey string, input UpsertUserUIPreferenceInput) (*UserUIPreference, error) {
	valueJSON, _ := json.Marshal(input.Value)

	pref := &UserUIPreference{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO user_ui_preferences(actor_id, preference_key, value)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (actor_id, preference_key) DO UPDATE SET
		    value = EXCLUDED.value,
		    updated_at = NOW()
		 RETURNING actor_id, preference_key, value, created_at, updated_at`,
		actorID, preferenceKey, valueJSON,
	).Scan(&pref.ActorID, &pref.PreferenceKey, &valueJSON, &pref.CreatedAt, &pref.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("upsert user ui preference: %w", err)
	}
	_ = json.Unmarshal(valueJSON, &pref.Value)
	return pref, nil
}

func (r *Repository) CreateFieldPermissionRule(ctx context.Context, input CreateFieldPermissionRuleInput) (*FieldPermissionRule, error) {
	metadataJSON, _ := json.Marshal(input.Metadata)
	rule := &FieldPermissionRule{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO field_permission_rules(
		    organization_id, scope_type, scope_id, table_name, field_name, actor_type, actor_id,
		    role_id, action, behavior, required_level, reason, priority, metadata
		 )
		 VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), $8, $9, $10, $11, $12, $13, $14)
		 RETURNING id, organization_id, scope_type, scope_id, table_name, field_name, actor_type, COALESCE(actor_id, ''),
		           role_id, action, behavior, required_level, reason, priority, status, metadata, created_at, updated_at`,
		input.OrganizationID, input.ScopeType, input.ScopeID, input.TableName, input.FieldName, input.ActorType, input.ActorID,
		input.RoleID, input.Action, input.Behavior, input.RequiredLevel, input.Reason, input.Priority, metadataJSON,
	).Scan(
		&rule.ID,
		&rule.OrganizationID,
		&rule.ScopeType,
		&rule.ScopeID,
		&rule.TableName,
		&rule.FieldName,
		&rule.ActorType,
		&rule.ActorID,
		&rule.RoleID,
		&rule.Action,
		&rule.Behavior,
		&rule.RequiredLevel,
		&rule.Reason,
		&rule.Priority,
		&rule.Status,
		&metadataJSON,
		&rule.CreatedAt,
		&rule.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create field permission rule: %w", err)
	}
	_ = json.Unmarshal(metadataJSON, &rule.Metadata)
	return rule, nil
}

func (r *Repository) ListFieldPermissionRules(ctx context.Context, tableName string) ([]FieldPermissionRule, error) {
	query := `SELECT id, organization_id, scope_type, scope_id, table_name, field_name, actor_type, COALESCE(actor_id, ''), role_id, action, behavior,
	                 required_level, reason, priority, status, metadata, created_at, updated_at
	          FROM field_permission_rules`
	args := []any{}
	if tableName != "" {
		query += ` WHERE table_name = $1`
		args = append(args, tableName)
	}
	query += ` ORDER BY table_name, field_name, action, actor_type, priority DESC, created_at DESC`

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list field permission rules: %w", err)
	}
	defer rows.Close()

	rules := make([]FieldPermissionRule, 0)
	for rows.Next() {
		var rule FieldPermissionRule
		var metadataJSON []byte
		if err := rows.Scan(
			&rule.ID,
			&rule.OrganizationID,
			&rule.ScopeType,
			&rule.ScopeID,
			&rule.TableName,
			&rule.FieldName,
			&rule.ActorType,
			&rule.ActorID,
			&rule.RoleID,
			&rule.Action,
			&rule.Behavior,
			&rule.RequiredLevel,
			&rule.Reason,
			&rule.Priority,
			&rule.Status,
			&metadataJSON,
			&rule.CreatedAt,
			&rule.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan field permission rule: %w", err)
		}
		_ = json.Unmarshal(metadataJSON, &rule.Metadata)
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list field permission rules iteration: %w", err)
	}
	return rules, nil
}

func (r *Repository) CheckFieldAccess(ctx context.Context, input FieldAccessCheckInput) (*FieldAccessCheckResult, error) {
	if input.FieldName == "" {
		input.FieldName = "*"
	}
	result := &FieldAccessCheckResult{
		Allowed:       true,
		Behavior:      "allow",
		RequiredLevel: "L1",
		Reason:        "no matching field rule",
	}
	err := r.db.QueryRow(ctx,
		`SELECT behavior, required_level, reason
		 FROM field_permission_rules
		 WHERE table_name = $1
		   AND action = $2
		   AND (field_name = $3 OR field_name = '*')
		   AND (actor_type = $4 OR actor_type = '*')
		   AND (actor_id = $5 OR actor_id IS NULL)
		   AND (organization_id IS NOT DISTINCT FROM $6 OR organization_id IS NULL)
		   AND (scope_type = $7 OR scope_type = 'organization')
		   AND (scope_id = $8 OR scope_id = '')
		   AND status = 'active'
		 ORDER BY
		   CASE WHEN organization_id IS NOT DISTINCT FROM $6 THEN 0 ELSE 1 END,
		   CASE WHEN scope_type = $7 THEN 0 ELSE 1 END,
		   CASE WHEN scope_id = $8 THEN 0 ELSE 1 END,
		   CASE WHEN actor_id = $5 THEN 0 ELSE 1 END,
		   CASE WHEN actor_type = $4 THEN 0 ELSE 1 END,
		   CASE WHEN field_name = $3 THEN 0 ELSE 1 END,
		   CASE WHEN behavior = 'deny' THEN 0 ELSE 1 END,
		   priority DESC,
		   created_at DESC
		 LIMIT 1`,
		input.TableName, input.Action, input.FieldName, input.ActorType, input.ActorID, input.OrganizationID, input.ScopeType, input.ScopeID,
	).Scan(&result.Behavior, &result.RequiredLevel, &result.Reason)
	if err != nil {
		return result, nil
	}
	result.Allowed = result.Behavior == "allow" || result.Behavior == "notify"
	return result, nil
}

func nullableUUID(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return *id
}
