package systemadmin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/platformauth"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/tenantdb"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetPlatformRole(ctx context.Context, userID uuid.UUID) (string, error) {
	var role string
	err := r.db.QueryRow(ctx, `SELECT role FROM public.platform_admins WHERE user_id = $1`, userID).Scan(&role)
	if err != nil {
		return "", fmt.Errorf("get platform role: %w", err)
	}
	return role, nil
}

func (r *Repository) ListPlatformMasters(ctx context.Context, moduleKey string, limit int) ([]PlatformMaster, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	query := `SELECT master_key, module_key, entity_type, source_table, source_pk, title, status,
	                 organization_id, payload, metadata, created_at, updated_at
	          FROM platform.platform_masters`
	args := []any{}
	if moduleKey != "" {
		query += ` WHERE module_key = $1`
		args = append(args, moduleKey)
	}
	query += fmt.Sprintf(` ORDER BY updated_at DESC, master_key LIMIT $%d`, len(args)+1)
	args = append(args, limit)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list platform masters: %w", err)
	}
	defer rows.Close()

	items := []PlatformMaster{}
	for rows.Next() {
		item, err := scanPlatformMaster(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list platform masters iteration: %w", err)
	}
	return items, nil
}

func (r *Repository) ListPlatformDetails(ctx context.Context, masterKey string) ([]PlatformDetail, error) {
	rows, err := r.db.Query(ctx, `
		SELECT detail_key, master_key, detail_type, field_key, line_no, payload, metadata, created_at, updated_at
		FROM platform.platform_details
		WHERE master_key = $1
		ORDER BY detail_type, line_no, field_key
	`, masterKey)
	if err != nil {
		return nil, fmt.Errorf("list platform details: %w", err)
	}
	defer rows.Close()

	items := []PlatformDetail{}
	for rows.Next() {
		item, err := scanPlatformDetail(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list platform details iteration: %w", err)
	}
	return items, nil
}

func (r *Repository) ListPlatformFeatures(ctx context.Context, status string, limit int) ([]PlatformFeature, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	query := `
		SELECT feature_key, parent_key, module_key, category, title, description, status, sort_order,
		       permission_keys, metadata, created_at, updated_at
		FROM platform.platform_features`
	args := []any{}
	if status != "" {
		query += ` WHERE status = $1`
		args = append(args, status)
	}
	query += fmt.Sprintf(` ORDER BY sort_order, feature_key LIMIT $%d`, len(args)+1)
	args = append(args, limit)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list platform features: %w", err)
	}
	defer rows.Close()

	items := []PlatformFeature{}
	for rows.Next() {
		item, err := scanPlatformFeature(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list platform features iteration: %w", err)
	}
	return items, nil
}

func (r *Repository) CreatePlatformFeature(ctx context.Context, record CreatePlatformFeatureRecord) (*PlatformFeature, error) {
	permissionKeysJSON, _ := json.Marshal(record.PermissionKeys)
	metadataJSON := jsonBytes(record.Metadata)
	return scanPlatformFeature(r.db.QueryRow(ctx, `
		INSERT INTO platform.platform_features(
		    feature_key, parent_key, module_key, category, title, description, status, sort_order,
		    permission_keys, metadata, created_by, updated_by
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10::jsonb, $11, $11)
		RETURNING feature_key, parent_key, module_key, category, title, description, status, sort_order,
		          permission_keys, metadata, created_at, updated_at
	`, record.FeatureKey, record.ParentKey, record.ModuleKey, record.Category, record.Title, record.Description,
		record.Status, record.SortOrder, permissionKeysJSON, metadataJSON, record.ActorID))
}

func (r *Repository) UpdatePlatformFeatureStatus(ctx context.Context, featureKey string, status string, actorID uuid.UUID) (*PlatformFeature, error) {
	return scanPlatformFeature(r.db.QueryRow(ctx, `
		UPDATE platform.platform_features
		SET status = $2, updated_by = $3, updated_at = NOW()
		WHERE feature_key = $1
		RETURNING feature_key, parent_key, module_key, category, title, description, status, sort_order,
		          permission_keys, metadata, created_at, updated_at
	`, featureKey, status, actorID))
}

func (r *Repository) ListPlatformMenuItems(ctx context.Context, limit int) ([]PlatformMenuItem, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	rows, err := r.db.Query(ctx, `
		SELECT menu_key, parent_key, feature_key, label_key, icon, route, required_permissions,
		       status, sort_order, metadata, created_at, updated_at
		FROM platform.platform_menu_items
		ORDER BY sort_order, menu_key
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list platform menu items: %w", err)
	}
	defer rows.Close()

	items := []PlatformMenuItem{}
	for rows.Next() {
		item, err := scanPlatformMenuItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list platform menu items iteration: %w", err)
	}
	return items, nil
}

func (r *Repository) ListPlatformPermissions(ctx context.Context) ([]PlatformPermission, error) {
	rows, err := r.db.Query(ctx, `
		SELECT permission_key, name, description, category, status, metadata, created_at, updated_at
		FROM platform.platform_permissions
		ORDER BY category, permission_key
	`)
	if err != nil {
		return nil, fmt.Errorf("list platform permissions: %w", err)
	}
	defer rows.Close()

	items := []PlatformPermission{}
	for rows.Next() {
		item, err := scanPlatformPermission(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list platform permissions iteration: %w", err)
	}
	return items, nil
}

func (r *Repository) ListPlatformRoles(ctx context.Context) ([]PlatformRole, error) {
	rows, err := r.db.Query(ctx, `
		SELECT pr.role_key, pr.name, pr.description, pr.status, pr.is_system,
		       COALESCE(jsonb_agg(rp.permission_key ORDER BY rp.permission_key)
		         FILTER (WHERE rp.permission_key IS NOT NULL AND rp.status = 'active'), '[]'::jsonb) AS permissions,
		       pr.metadata, pr.created_at, pr.updated_at
		FROM platform.platform_roles pr
		LEFT JOIN platform.platform_role_permissions rp ON rp.role_key = pr.role_key
		GROUP BY pr.role_key, pr.name, pr.description, pr.status, pr.is_system, pr.metadata, pr.created_at, pr.updated_at
		ORDER BY pr.is_system DESC, pr.role_key
	`)
	if err != nil {
		return nil, fmt.Errorf("list platform roles: %w", err)
	}
	defer rows.Close()

	items := []PlatformRole{}
	for rows.Next() {
		item, err := scanPlatformRole(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list platform roles iteration: %w", err)
	}
	return items, nil
}

func (r *Repository) ListPlatformRolePermissions(ctx context.Context, roleKey string) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT permission_key
		FROM platform.platform_role_permissions
		WHERE role_key = $1 AND status = 'active'
		ORDER BY permission_key
	`, roleKey)
	if err != nil {
		return nil, fmt.Errorf("list platform role permissions: %w", err)
	}
	defer rows.Close()
	items := []string{}
	for rows.Next() {
		var item string
		if err := rows.Scan(&item); err != nil {
			return nil, fmt.Errorf("scan platform role permission: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list platform role permissions iteration: %w", err)
	}
	return items, nil
}

func (r *Repository) SetPlatformRolePermissions(ctx context.Context, roleKey string, permissions []string, actorID uuid.UUID) (*PlatformRole, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin set platform role permissions: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO platform.platform_roles(role_key, name, description, status, is_system, metadata, created_by, updated_by)
		VALUES ($1, $1, '', 'active', false, '{}'::jsonb, $2, $2)
		ON CONFLICT (role_key) DO UPDATE SET updated_by = EXCLUDED.updated_by, updated_at = NOW()
	`, roleKey, actorID); err != nil {
		return nil, fmt.Errorf("upsert platform role: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM platform.platform_role_permissions WHERE role_key = $1`, roleKey); err != nil {
		return nil, fmt.Errorf("clear platform role permissions: %w", err)
	}
	for _, permission := range permissions {
		if _, err := tx.Exec(ctx, `
			INSERT INTO platform.platform_role_permissions(role_key, permission_key, status, granted_by)
			VALUES ($1, $2, 'active', $3)
		`, roleKey, permission, actorID); err != nil {
			return nil, fmt.Errorf("insert platform role permission: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit set platform role permissions: %w", err)
	}
	return r.getPlatformRoleRecord(ctx, roleKey)
}

func (r *Repository) ListPlatformUsers(ctx context.Context, limit int) ([]PlatformUser, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.db.Query(ctx, `
		SELECT u.id, u.name, u.email, COALESCE(u.account_status, 'active'),
		       COALESCE(
		         jsonb_agg(DISTINCT pur.role_key) FILTER (WHERE pur.role_key IS NOT NULL AND pur.status = 'active'),
		         CASE WHEN pa.role IS NOT NULL THEN to_jsonb(ARRAY[pa.role]) ELSE '[]'::jsonb END
		       ) AS roles,
		       jsonb_build_object('platform_admin_role', COALESCE(pa.role, '')) AS metadata,
		       u.created_at, u.updated_at
		FROM public.users u
		LEFT JOIN public.platform_admins pa ON pa.user_id = u.id
		LEFT JOIN platform.platform_user_roles pur ON pur.user_id = u.id
		WHERE pa.user_id IS NOT NULL OR pur.user_id IS NOT NULL
		GROUP BY u.id, u.name, u.email, u.account_status, pa.role, u.created_at, u.updated_at
		ORDER BY u.updated_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list platform users: %w", err)
	}
	defer rows.Close()

	items := []PlatformUser{}
	for rows.Next() {
		item, err := scanPlatformUser(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list platform users iteration: %w", err)
	}
	return items, nil
}

func (r *Repository) CreatePlatformUser(ctx context.Context, record CreatePlatformUserRecord) (*PlatformUser, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin create platform user: %w", err)
	}
	defer tx.Rollback(ctx)

	var userID uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO public.users(name, email, password_hash, account_status, onboarding_status)
		VALUES ($1, lower($2), $3, 'active', 'complete')
		RETURNING id
	`, record.Name, record.Email, record.PasswordHash).Scan(&userID); err != nil {
		return nil, fmt.Errorf("insert platform user: %w", err)
	}
	if err := upsertPlatformAdminRole(ctx, tx, userID, primaryPlatformRole(record.Roles)); err != nil {
		return nil, err
	}
	if err := replacePlatformUserRoles(ctx, tx, userID, record.Roles, record.ActorID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit create platform user: %w", err)
	}
	return r.getPlatformUser(ctx, userID)
}

func (r *Repository) SetPlatformUserRoles(ctx context.Context, userID uuid.UUID, roles []string, actorID uuid.UUID) (*PlatformUser, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin set platform user roles: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := upsertPlatformAdminRole(ctx, tx, userID, primaryPlatformRole(roles)); err != nil {
		return nil, err
	}
	if err := replacePlatformUserRoles(ctx, tx, userID, roles, actorID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit set platform user roles: %w", err)
	}
	return r.getPlatformUser(ctx, userID)
}

func (r *Repository) ResetPlatformUserPassword(ctx context.Context, userID uuid.UUID, passwordHash string, actorID uuid.UUID) (*PlatformUser, error) {
	if _, err := r.db.Exec(ctx, `
		UPDATE public.users
		SET password_hash = $2, updated_at = NOW()
		WHERE id = $1
	`, userID, passwordHash); err != nil {
		return nil, fmt.Errorf("reset platform user password: %w", err)
	}
	return r.getPlatformUser(ctx, userID)
}

func (r *Repository) DisablePlatformUser(ctx context.Context, userID uuid.UUID, actorID uuid.UUID) (*PlatformUser, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin disable platform user: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		UPDATE public.users
		SET account_status = 'disabled', updated_at = NOW()
		WHERE id = $1
	`, userID); err != nil {
		return nil, fmt.Errorf("disable user account: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE platform.platform_user_roles
		SET status = 'disabled', updated_at = NOW()
		WHERE user_id = $1
	`, userID); err != nil {
		return nil, fmt.Errorf("disable platform user roles: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit disable platform user: %w", err)
	}
	return r.getPlatformUser(ctx, userID)
}

func (r *Repository) ListDatabaseMaintenanceJobs(ctx context.Context, limit int) ([]DatabaseMaintenanceJob, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, job_type, scope, status, reason, backup_ref, requested_by, reviewed_by,
		       review_reason, result, metadata, created_at, reviewed_at, completed_at, updated_at
		FROM platform.database_maintenance_jobs
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list database maintenance jobs: %w", err)
	}
	defer rows.Close()

	items := []DatabaseMaintenanceJob{}
	for rows.Next() {
		item, err := scanDatabaseMaintenanceJob(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list database maintenance jobs iteration: %w", err)
	}
	return items, nil
}

func (r *Repository) CreateDatabaseMaintenanceJob(ctx context.Context, record CreateDatabaseMaintenanceJobRecord) (*DatabaseMaintenanceJob, error) {
	metadataJSON := jsonBytes(record.Metadata)
	return scanDatabaseMaintenanceJob(r.db.QueryRow(ctx, `
		INSERT INTO platform.database_maintenance_jobs(
		    job_type, scope, status, reason, backup_ref, requested_by, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb)
		RETURNING id, job_type, scope, status, reason, backup_ref, requested_by, reviewed_by,
		          review_reason, result, metadata, created_at, reviewed_at, completed_at, updated_at
	`, record.JobType, record.Scope, record.Status, record.Reason, record.BackupRef, record.RequestedBy, metadataJSON))
}

func (r *Repository) ReviewDatabaseMaintenanceJob(ctx context.Context, record ReviewDatabaseMaintenanceJobRecord) (*DatabaseMaintenanceJob, error) {
	return scanDatabaseMaintenanceJob(r.db.QueryRow(ctx, `
		UPDATE platform.database_maintenance_jobs
		SET status = $2,
		    reviewed_by = $3,
		    review_reason = $4,
		    reviewed_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1 AND status = 'pending_approval'
		RETURNING id, job_type, scope, status, reason, backup_ref, requested_by, reviewed_by,
		          review_reason, result, metadata, created_at, reviewed_at, completed_at, updated_at
	`, record.JobID, record.Status, record.ReviewedBy, record.ReviewReason))
}

func (r *Repository) ListIndustrySolutionTargets(ctx context.Context, limit int) ([]OrganizationIndustrySolutionTarget, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := r.db.Query(ctx, `
		SELECT st.organization_id, st.target_schema_name, st.template_version, st.status, st.last_change_request_id,
		       st.metadata, st.created_at, st.updated_at,
		       COALESCE(tdt.deployment_mode, ''), COALESCE(tdt.cluster_key, ''), COALESCE(tdt.region, ''),
		       COALESCE(tdt.database_name, ''), COALESCE(tdt.status, '')
		FROM platform.organization_industry_solution_targets st
		LEFT JOIN platform.tenant_database_targets tdt ON tdt.organization_id = st.organization_id
		ORDER BY st.updated_at DESC, st.target_schema_name
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list industry solution targets: %w", err)
	}
	defer rows.Close()

	items := []OrganizationIndustrySolutionTarget{}
	for rows.Next() {
		item, err := scanIndustrySolutionTarget(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list industry solution targets iteration: %w", err)
	}
	return items, nil
}

func (r *Repository) GetIndustrySolutionTarget(ctx context.Context, orgID uuid.UUID) (*OrganizationIndustrySolutionTarget, error) {
	return scanIndustrySolutionTarget(r.db.QueryRow(ctx, `
		SELECT st.organization_id, st.target_schema_name, st.template_version, st.status, st.last_change_request_id,
		       st.metadata, st.created_at, st.updated_at,
		       COALESCE(tdt.deployment_mode, ''), COALESCE(tdt.cluster_key, ''), COALESCE(tdt.region, ''),
		       COALESCE(tdt.database_name, ''), COALESCE(tdt.status, '')
		FROM platform.organization_industry_solution_targets st
		LEFT JOIN platform.tenant_database_targets tdt ON tdt.organization_id = st.organization_id
		WHERE st.organization_id = $1
	`, orgID))
}

func (r *Repository) CreateIndustrySolutionChangeRequest(ctx context.Context, record CreateIndustrySolutionChangeRequestRecord) (*IndustrySolutionChangeRequest, error) {
	manifestJSON, _ := json.Marshal(record.SolutionManifest)
	statementsJSON, _ := json.Marshal(record.Statements)
	diffJSON, _ := json.Marshal(record.Diff)
	return scanIndustrySolutionChangeRequest(r.db.QueryRow(ctx, `
		INSERT INTO platform.industry_solution_change_requests(
		    organization_id, target_schema_name, request_type, status, reason, solution_manifest, statements, risk_level, diff, requested_by
		)
		VALUES ($1, $2, $3, 'pending', $4, $5, $6, $7, $8, $9)
		RETURNING id, organization_id, target_schema_name, request_type, status, reason, solution_manifest, statements, risk_level, diff,
		          requested_by, reviewed_by, applied_by, review_reason, created_at, reviewed_at, applied_at, updated_at
	`, record.OrganizationID, record.TargetSchemaName, record.RequestType, record.Reason, manifestJSON, statementsJSON, record.RiskLevel, diffJSON, record.RequestedBy))
}

func (r *Repository) GetIndustrySolutionChangeRequest(ctx context.Context, id uuid.UUID) (*IndustrySolutionChangeRequest, error) {
	return scanIndustrySolutionChangeRequest(r.db.QueryRow(ctx, `
		SELECT id, organization_id, target_schema_name, request_type, status, reason, solution_manifest, statements, risk_level, diff,
		       requested_by, reviewed_by, applied_by, review_reason, created_at, reviewed_at, applied_at, updated_at
		FROM platform.industry_solution_change_requests
		WHERE id = $1
	`, id))
}

func (r *Repository) UpdateIndustrySolutionChangeRequestStatus(ctx context.Context, id uuid.UUID, status string, reviewerID uuid.UUID, reason string) (*IndustrySolutionChangeRequest, error) {
	return scanIndustrySolutionChangeRequest(r.db.QueryRow(ctx, `
		UPDATE platform.industry_solution_change_requests
		SET status = $2,
		    reviewed_by = $3,
		    review_reason = $4,
		    reviewed_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id, organization_id, target_schema_name, request_type, status, reason, solution_manifest, statements, risk_level, diff,
		          requested_by, reviewed_by, applied_by, review_reason, created_at, reviewed_at, applied_at, updated_at
	`, id, status, reviewerID, reason))
}

func (r *Repository) ApplyIndustrySolutionChange(ctx context.Context, request *IndustrySolutionChangeRequest, statements []string, assetResults []IndustrySolutionApplyAssetResult) (*IndustrySolutionApplyJob, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin apply industry solution change: %w", err)
	}
	defer tx.Rollback(ctx)

	quotedSchema, err := tenantdb.QuoteIdentifier(request.TargetSchemaName)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS "+quotedSchema); err != nil {
		return nil, fmt.Errorf("create organization schema: %w", err)
	}
	for _, statement := range statements {
		if _, err := tx.Exec(ctx, statement); err != nil {
			return nil, fmt.Errorf("execute schema statement: %w", err)
		}
	}

	statementsJSON, _ := json.Marshal(statements)
	assetResults = r.applyIndustrySolutionAssets(ctx, tx, request, assetResults)
	metadataJSON, _ := json.Marshal(map[string]any{
		"source":        "systemadmin",
		"asset_results": assetResults,
	})
	job, err := scanIndustrySolutionApplyJob(tx.QueryRow(ctx, `
		INSERT INTO platform.industry_solution_apply_jobs(
		    change_request_id, organization_id, target_schema_name, status, statements, metadata
		)
		VALUES ($1, $2, $3, 'applied', $4, $5::jsonb)
		RETURNING id, change_request_id, organization_id, target_schema_name, status, statements,
		          error_message, metadata, created_at, updated_at
	`, request.ID, request.OrganizationID, request.TargetSchemaName, statementsJSON, metadataJSON))
	if err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE platform.industry_solution_change_requests
		SET status = 'applied', applied_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, request.ID); err != nil {
		return nil, fmt.Errorf("mark industry solution request applied: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO platform.organization_industry_solution_targets(
		    organization_id, target_schema_name, template_version, status, last_change_request_id, metadata
		)
		VALUES ($1, $2, $3, 'provisioned', $4, '{"source":"industry_solution_apply_job"}'::jsonb)
		ON CONFLICT (organization_id) DO UPDATE SET
		    target_schema_name = EXCLUDED.target_schema_name,
		    template_version = EXCLUDED.template_version,
		    status = 'provisioned',
		    last_change_request_id = EXCLUDED.last_change_request_id,
		    metadata = platform.organization_industry_solution_targets.metadata || EXCLUDED.metadata,
		    updated_at = NOW()
	`, request.OrganizationID, request.TargetSchemaName, request.SolutionManifest.FormatVersion, request.ID); err != nil {
		return nil, fmt.Errorf("upsert industry solution target: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit apply industry solution change: %w", err)
	}
	return job, nil
}

func (r *Repository) applyIndustrySolutionAssets(ctx context.Context, tx pgx.Tx, request *IndustrySolutionChangeRequest, results []IndustrySolutionApplyAssetResult) []IndustrySolutionApplyAssetResult {
	for i := range results {
		err := r.applyIndustrySolutionAsset(ctx, tx, request, &results[i])
		if err != nil {
			results[i].Status = "failed"
			results[i].ErrorMessage = err.Error()
			continue
		}
		results[i].Status = "applied"
	}
	return results
}

func (r *Repository) applyIndustrySolutionAsset(ctx context.Context, tx pgx.Tx, request *IndustrySolutionChangeRequest, result *IndustrySolutionApplyAssetResult) error {
	payload, _ := result.Metadata["payload"].(map[string]any)
	switch result.AssetType {
	case AssetTypeRuntimeOperation:
		operationKey := firstNonEmptyString(stringValue(payload["operation_key"]), result.AssetKey)
		path := firstNonEmptyString(stringValue(payload["path"]), result.AssetKey)
		metadata := copyApplyMetadata(payload)
		metadata["source_change_request_id"] = request.ID.String()
		metadata["asset_key"] = result.AssetKey
		metadata["risk_level"] = result.Metadata["risk_level"]
		_, err := tx.Exec(ctx, `
			INSERT INTO platform.runtime_operations(operation_key, domain, title, method, path, operation_kind, danger_level, result_view, assistant_eligible, status, action_type, metadata)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'active', $10, $11::jsonb)
			ON CONFLICT (operation_key) DO UPDATE SET
				domain = EXCLUDED.domain,
				title = EXCLUDED.title,
				method = EXCLUDED.method,
				path = EXCLUDED.path,
				operation_kind = EXCLUDED.operation_kind,
				danger_level = EXCLUDED.danger_level,
				result_view = EXCLUDED.result_view,
				assistant_eligible = EXCLUDED.assistant_eligible,
				action_type = EXCLUDED.action_type,
				metadata = platform.runtime_operations.metadata || EXCLUDED.metadata,
				updated_at = NOW()
		`, operationKey,
			firstNonEmptyString(stringValue(payload["domain"]), "ERP"),
			firstNonEmptyString(stringValue(payload["title"]), operationKey),
			firstNonEmptyString(stringValue(payload["method"]), "POST"),
			path,
			firstNonEmptyString(stringValue(payload["operation_kind"]), "contextual"),
			firstNonEmptyString(stringValue(payload["danger_level"]), "medium"),
			firstNonEmptyString(stringValue(payload["result_view"]), "summary"),
			boolValue(payload["assistant_eligible"], true),
			firstNonEmptyString(stringValue(payload["action_type"]), "erp.action"),
			jsonBytes(metadata),
		)
		return err
	case AssetTypeToolDefinition, AssetTypeToolPolicy:
		_, err := tx.Exec(ctx, `
			INSERT INTO tool_definitions(name, description, source_type, default_policy, risk_level, required_level, metadata)
			VALUES ($1, 'Generated from ERP industry solution', 'internal_api', 'approve', 'medium', 'L2', $2::jsonb)
			ON CONFLICT (name) DO UPDATE SET
				metadata = tool_definitions.metadata || EXCLUDED.metadata,
				updated_at = NOW()
		`, result.AssetKey, jsonBytes(map[string]any{"source_change_request_id": request.ID.String(), "asset_key": result.AssetKey}))
		return err
	default:
		_, err := tx.Exec(ctx, `
			INSERT INTO platform.platform_masters(module_key, entity_type, source_table, source_pk, title, status, organization_id, payload, metadata)
			VALUES ('industry_solution_factory', $1, 'industry_solution_asset', $2, $3, 'draft', $4, $5::jsonb, $6::jsonb)
			ON CONFLICT (source_table, source_pk) WHERE source_table <> '' AND source_pk <> ''
			DO UPDATE SET
				title = EXCLUDED.title,
				status = EXCLUDED.status,
				payload = EXCLUDED.payload,
				metadata = platform.platform_masters.metadata || EXCLUDED.metadata,
				updated_at = NOW()
		`, result.AssetType, request.ID.String()+":"+result.AssetKey, result.AssetKey, request.OrganizationID, jsonBytes(payload), jsonBytes(map[string]any{"source_change_request_id": request.ID.String(), "target": result.Target}))
		return err
	}
}

func scanPlatformMaster(row interface{ Scan(dest ...any) error }) (*PlatformMaster, error) {
	var item PlatformMaster
	var payloadJSON, metadataJSON []byte
	if err := row.Scan(
		&item.MasterKey,
		&item.ModuleKey,
		&item.EntityType,
		&item.SourceTable,
		&item.SourcePK,
		&item.Title,
		&item.Status,
		&item.OrganizationID,
		&payloadJSON,
		&metadataJSON,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan platform master: %w", err)
	}
	item.Payload = unmarshalMap(payloadJSON)
	item.Metadata = unmarshalMap(metadataJSON)
	return &item, nil
}

func scanPlatformDetail(row interface{ Scan(dest ...any) error }) (*PlatformDetail, error) {
	var item PlatformDetail
	var payloadJSON, metadataJSON []byte
	if err := row.Scan(
		&item.DetailKey,
		&item.MasterKey,
		&item.DetailType,
		&item.FieldKey,
		&item.LineNo,
		&payloadJSON,
		&metadataJSON,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan platform detail: %w", err)
	}
	item.Payload = unmarshalMap(payloadJSON)
	item.Metadata = unmarshalMap(metadataJSON)
	return &item, nil
}

func scanIndustrySolutionTarget(row interface{ Scan(dest ...any) error }) (*OrganizationIndustrySolutionTarget, error) {
	var item OrganizationIndustrySolutionTarget
	var metadataJSON []byte
	if err := row.Scan(
		&item.OrganizationID,
		&item.TargetSchemaName,
		&item.TemplateVersion,
		&item.Status,
		&item.LastChangeRequestID,
		&metadataJSON,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.TenantDatabaseDeploymentMode,
		&item.TenantDatabaseClusterKey,
		&item.TenantDatabaseRegion,
		&item.TenantDatabaseName,
		&item.TenantDatabaseStatus,
	); err != nil {
		return nil, fmt.Errorf("scan industry solution target: %w", err)
	}
	item.Metadata = unmarshalMap(metadataJSON)
	return &item, nil
}

func scanIndustrySolutionChangeRequest(row interface{ Scan(dest ...any) error }) (*IndustrySolutionChangeRequest, error) {
	var item IndustrySolutionChangeRequest
	var manifestJSON, statementsJSON, diffJSON []byte
	if err := row.Scan(
		&item.ID,
		&item.OrganizationID,
		&item.TargetSchemaName,
		&item.RequestType,
		&item.Status,
		&item.Reason,
		&manifestJSON,
		&statementsJSON,
		&item.RiskLevel,
		&diffJSON,
		&item.RequestedBy,
		&item.ReviewedBy,
		&item.AppliedBy,
		&item.ReviewReason,
		&item.CreatedAt,
		&item.ReviewedAt,
		&item.AppliedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan industry solution change request: %w", err)
	}
	_ = json.Unmarshal(manifestJSON, &item.SolutionManifest)
	_ = json.Unmarshal(statementsJSON, &item.Statements)
	_ = json.Unmarshal(diffJSON, &item.Diff)
	if item.RiskLevel == "" {
		item.RiskLevel = IndustrySolutionRiskSafe
	}
	return &item, nil
}

func scanIndustrySolutionApplyJob(row interface{ Scan(dest ...any) error }) (*IndustrySolutionApplyJob, error) {
	var item IndustrySolutionApplyJob
	var statementsJSON, metadataJSON []byte
	if err := row.Scan(
		&item.ID,
		&item.ChangeRequestID,
		&item.OrganizationID,
		&item.TargetSchemaName,
		&item.Status,
		&statementsJSON,
		&item.ErrorMessage,
		&metadataJSON,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan industry solution apply job: %w", err)
	}
	_ = json.Unmarshal(statementsJSON, &item.Statements)
	item.Metadata = unmarshalMap(metadataJSON)
	return &item, nil
}

func scanPlatformFeature(row interface{ Scan(dest ...any) error }) (*PlatformFeature, error) {
	var item PlatformFeature
	var permissionKeysJSON, metadataJSON []byte
	if err := row.Scan(
		&item.FeatureKey,
		&item.ParentKey,
		&item.ModuleKey,
		&item.Category,
		&item.Title,
		&item.Description,
		&item.Status,
		&item.SortOrder,
		&permissionKeysJSON,
		&metadataJSON,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan platform feature: %w", err)
	}
	item.PermissionKeys = unmarshalStringSlice(permissionKeysJSON)
	item.Metadata = unmarshalMap(metadataJSON)
	return &item, nil
}

func scanPlatformMenuItem(row interface{ Scan(dest ...any) error }) (*PlatformMenuItem, error) {
	var item PlatformMenuItem
	var requiredPermissionsJSON, metadataJSON []byte
	if err := row.Scan(
		&item.MenuKey,
		&item.ParentKey,
		&item.FeatureKey,
		&item.LabelKey,
		&item.Icon,
		&item.Route,
		&requiredPermissionsJSON,
		&item.Status,
		&item.SortOrder,
		&metadataJSON,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan platform menu item: %w", err)
	}
	item.RequiredPermissions = unmarshalStringSlice(requiredPermissionsJSON)
	item.Metadata = unmarshalMap(metadataJSON)
	return &item, nil
}

func scanPlatformPermission(row interface{ Scan(dest ...any) error }) (*PlatformPermission, error) {
	var item PlatformPermission
	var metadataJSON []byte
	if err := row.Scan(
		&item.PermissionKey,
		&item.Name,
		&item.Description,
		&item.Category,
		&item.Status,
		&metadataJSON,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan platform permission: %w", err)
	}
	item.Metadata = unmarshalMap(metadataJSON)
	return &item, nil
}

func scanPlatformRole(row interface{ Scan(dest ...any) error }) (*PlatformRole, error) {
	var item PlatformRole
	var permissionsJSON, metadataJSON []byte
	if err := row.Scan(
		&item.RoleKey,
		&item.Name,
		&item.Description,
		&item.Status,
		&item.IsSystem,
		&permissionsJSON,
		&metadataJSON,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan platform role: %w", err)
	}
	item.Permissions = unmarshalStringSlice(permissionsJSON)
	item.Metadata = unmarshalMap(metadataJSON)
	return &item, nil
}

func scanPlatformUser(row interface{ Scan(dest ...any) error }) (*PlatformUser, error) {
	var item PlatformUser
	var rolesJSON, metadataJSON []byte
	if err := row.Scan(
		&item.UserID,
		&item.Name,
		&item.Email,
		&item.AccountStatus,
		&rolesJSON,
		&metadataJSON,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan platform user: %w", err)
	}
	item.Roles = normalizePlatformRoles(unmarshalStringSlice(rolesJSON))
	item.Metadata = unmarshalMap(metadataJSON)
	return &item, nil
}

func scanDatabaseMaintenanceJob(row interface{ Scan(dest ...any) error }) (*DatabaseMaintenanceJob, error) {
	var item DatabaseMaintenanceJob
	var resultJSON, metadataJSON []byte
	if err := row.Scan(
		&item.ID,
		&item.JobType,
		&item.Scope,
		&item.Status,
		&item.Reason,
		&item.BackupRef,
		&item.RequestedBy,
		&item.ReviewedBy,
		&item.ReviewReason,
		&resultJSON,
		&metadataJSON,
		&item.CreatedAt,
		&item.ReviewedAt,
		&item.CompletedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan database maintenance job: %w", err)
	}
	item.Result = unmarshalMap(resultJSON)
	item.Metadata = unmarshalMap(metadataJSON)
	return &item, nil
}

func unmarshalMap(data []byte) map[string]any {
	if len(data) == 0 {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil || out == nil {
		return map[string]any{}
	}
	return out
}

func unmarshalStringSlice(data []byte) []string {
	if len(data) == 0 {
		return []string{}
	}
	var out []string
	if err := json.Unmarshal(data, &out); err != nil || out == nil {
		return []string{}
	}
	return out
}

func jsonBytes(value any) []byte {
	if value == nil {
		return []byte("{}")
	}
	data, err := json.Marshal(value)
	if err != nil || len(data) == 0 {
		return []byte("{}")
	}
	return data
}

func copyApplyMetadata(input map[string]any) map[string]any {
	output := make(map[string]any, len(input)+3)
	for key, value := range input {
		output[key] = value
	}
	return output
}

func boolValue(value any, fallback bool) bool {
	typed, ok := value.(bool)
	if !ok {
		return fallback
	}
	return typed
}

func (r *Repository) getPlatformRoleRecord(ctx context.Context, roleKey string) (*PlatformRole, error) {
	return scanPlatformRole(r.db.QueryRow(ctx, `
		SELECT pr.role_key, pr.name, pr.description, pr.status, pr.is_system,
		       COALESCE(jsonb_agg(rp.permission_key ORDER BY rp.permission_key)
		         FILTER (WHERE rp.permission_key IS NOT NULL AND rp.status = 'active'), '[]'::jsonb) AS permissions,
		       pr.metadata, pr.created_at, pr.updated_at
		FROM platform.platform_roles pr
		LEFT JOIN platform.platform_role_permissions rp ON rp.role_key = pr.role_key
		WHERE pr.role_key = $1
		GROUP BY pr.role_key, pr.name, pr.description, pr.status, pr.is_system, pr.metadata, pr.created_at, pr.updated_at
	`, roleKey))
}

func (r *Repository) getPlatformUser(ctx context.Context, userID uuid.UUID) (*PlatformUser, error) {
	return scanPlatformUser(r.db.QueryRow(ctx, `
		SELECT u.id, u.name, u.email, COALESCE(u.account_status, 'active'),
		       COALESCE(
		         jsonb_agg(DISTINCT pur.role_key) FILTER (WHERE pur.role_key IS NOT NULL AND pur.status = 'active'),
		         CASE WHEN pa.role IS NOT NULL THEN to_jsonb(ARRAY[pa.role]) ELSE '[]'::jsonb END
		       ) AS roles,
		       jsonb_build_object('platform_admin_role', COALESCE(pa.role, '')) AS metadata,
		       u.created_at, u.updated_at
		FROM public.users u
		LEFT JOIN public.platform_admins pa ON pa.user_id = u.id
		LEFT JOIN platform.platform_user_roles pur ON pur.user_id = u.id
		WHERE u.id = $1
		GROUP BY u.id, u.name, u.email, u.account_status, pa.role, u.created_at, u.updated_at
	`, userID))
}

func replacePlatformUserRoles(ctx context.Context, tx pgx.Tx, userID uuid.UUID, roles []string, actorID uuid.UUID) error {
	if _, err := tx.Exec(ctx, `DELETE FROM platform.platform_user_roles WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("clear platform user roles: %w", err)
	}
	for _, role := range roles {
		if _, err := tx.Exec(ctx, `
			INSERT INTO platform.platform_user_roles(user_id, role_key, status, granted_by)
			VALUES ($1, $2, 'active', $3)
		`, userID, role, actorID); err != nil {
			return fmt.Errorf("insert platform user role: %w", err)
		}
	}
	return nil
}

func upsertPlatformAdminRole(ctx context.Context, tx pgx.Tx, userID uuid.UUID, role string) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO public.platform_admins(user_id, role)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE SET role = EXCLUDED.role, updated_at = NOW()
	`, userID, legacyPlatformAdminRole(role)); err != nil {
		return fmt.Errorf("upsert platform admin compatibility role: %w", err)
	}
	return nil
}

func primaryPlatformRole(roles []string) string {
	if len(roles) == 0 {
		return "auditor"
	}
	for _, preferred := range []string{"owner", "admin", "operator", "auditor"} {
		for _, role := range roles {
			if role == preferred {
				return role
			}
		}
	}
	return roles[0]
}

func legacyPlatformAdminRole(role string) string {
	switch platformauth.NormalizeRole(strings.TrimSpace(role)) {
	case platformauth.RoleOwner:
		return "system_owner"
	case platformauth.RoleAdmin:
		return "system_admin"
	default:
		return "support"
	}
}

var _ repository = (*Repository)(nil)
