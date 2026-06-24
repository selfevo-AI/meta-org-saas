package systemadmin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

func (r *Repository) ListSchemaTargets(ctx context.Context, limit int) ([]OrganizationSchemaTarget, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := r.db.Query(ctx, `
		SELECT organization_id, schema_name, template_version, status, last_change_request_id,
		       metadata, created_at, updated_at
		FROM platform.organization_schema_targets
		ORDER BY updated_at DESC, schema_name
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list schema targets: %w", err)
	}
	defer rows.Close()

	items := []OrganizationSchemaTarget{}
	for rows.Next() {
		item, err := scanSchemaTarget(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list schema targets iteration: %w", err)
	}
	return items, nil
}

func (r *Repository) GetSchemaTarget(ctx context.Context, orgID uuid.UUID) (*OrganizationSchemaTarget, error) {
	return scanSchemaTarget(r.db.QueryRow(ctx, `
		SELECT organization_id, schema_name, template_version, status, last_change_request_id,
		       metadata, created_at, updated_at
		FROM platform.organization_schema_targets
		WHERE organization_id = $1
	`, orgID))
}

func (r *Repository) CreateSchemaChangeRequest(ctx context.Context, record CreateSchemaChangeRequestRecord) (*SchemaChangeRequest, error) {
	pkgJSON, _ := json.Marshal(record.SchemaPackage)
	statementsJSON, _ := json.Marshal(record.Statements)
	diffJSON, _ := json.Marshal(record.Diff)
	return scanSchemaChangeRequest(r.db.QueryRow(ctx, `
		INSERT INTO platform.schema_change_requests(
		    organization_id, schema_name, request_type, status, reason, schema_package, statements, risk_level, diff, requested_by
		)
		VALUES ($1, $2, $3, 'pending', $4, $5, $6, $7, $8, $9)
		RETURNING id, organization_id, schema_name, request_type, status, reason, schema_package, statements, risk_level, diff,
		          requested_by, reviewed_by, applied_by, review_reason, created_at, reviewed_at, applied_at, updated_at
	`, record.OrganizationID, record.SchemaName, record.RequestType, record.Reason, pkgJSON, statementsJSON, record.RiskLevel, diffJSON, record.RequestedBy))
}

func (r *Repository) GetSchemaChangeRequest(ctx context.Context, id uuid.UUID) (*SchemaChangeRequest, error) {
	return scanSchemaChangeRequest(r.db.QueryRow(ctx, `
		SELECT id, organization_id, schema_name, request_type, status, reason, schema_package, statements, risk_level, diff,
		       requested_by, reviewed_by, applied_by, review_reason, created_at, reviewed_at, applied_at, updated_at
		FROM platform.schema_change_requests
		WHERE id = $1
	`, id))
}

func (r *Repository) UpdateSchemaChangeRequestStatus(ctx context.Context, id uuid.UUID, status string, reviewerID uuid.UUID, reason string) (*SchemaChangeRequest, error) {
	return scanSchemaChangeRequest(r.db.QueryRow(ctx, `
		UPDATE platform.schema_change_requests
		SET status = $2,
		    reviewed_by = $3,
		    review_reason = $4,
		    reviewed_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id, organization_id, schema_name, request_type, status, reason, schema_package, statements, risk_level, diff,
		          requested_by, reviewed_by, applied_by, review_reason, created_at, reviewed_at, applied_at, updated_at
	`, id, status, reviewerID, reason))
}

func (r *Repository) ApplySchemaChange(ctx context.Context, request *SchemaChangeRequest, statements []string, assetResults []SchemaApplyAssetResult) (*SchemaApplyJob, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin apply schema change: %w", err)
	}
	defer tx.Rollback(ctx)

	quotedSchema, err := tenantdb.QuoteIdentifier(request.SchemaName)
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
	job, err := scanSchemaApplyJob(tx.QueryRow(ctx, `
		INSERT INTO platform.schema_apply_jobs(
		    change_request_id, organization_id, schema_name, status, statements, metadata
		)
		VALUES ($1, $2, $3, 'applied', $4, $5::jsonb)
		RETURNING id, change_request_id, organization_id, schema_name, status, statements,
		          error_message, metadata, created_at, updated_at
	`, request.ID, request.OrganizationID, request.SchemaName, statementsJSON, metadataJSON))
	if err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE platform.schema_change_requests
		SET status = 'applied', applied_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, request.ID); err != nil {
		return nil, fmt.Errorf("mark schema request applied: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO platform.organization_schema_targets(
		    organization_id, schema_name, template_version, status, last_change_request_id, metadata
		)
		VALUES ($1, $2, $3, 'provisioned', $4, '{"source":"schema_apply_job"}'::jsonb)
		ON CONFLICT (organization_id) DO UPDATE SET
		    schema_name = EXCLUDED.schema_name,
		    template_version = EXCLUDED.template_version,
		    status = 'provisioned',
		    last_change_request_id = EXCLUDED.last_change_request_id,
		    metadata = platform.organization_schema_targets.metadata || EXCLUDED.metadata,
		    updated_at = NOW()
	`, request.OrganizationID, request.SchemaName, request.SchemaPackage.FormatVersion, request.ID); err != nil {
		return nil, fmt.Errorf("upsert schema target: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit apply schema change: %w", err)
	}
	return job, nil
}

func (r *Repository) applyIndustrySolutionAssets(ctx context.Context, tx pgx.Tx, request *SchemaChangeRequest, results []SchemaApplyAssetResult) []SchemaApplyAssetResult {
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

func (r *Repository) applyIndustrySolutionAsset(ctx context.Context, tx pgx.Tx, request *SchemaChangeRequest, result *SchemaApplyAssetResult) error {
	payload, _ := result.Metadata["payload"].(map[string]any)
	switch result.AssetType {
	case AssetTypeRuntimeOperation:
		path := firstNonEmptyString(stringValue(payload["path"]), result.AssetKey)
		_, err := tx.Exec(ctx, `
			INSERT INTO platform.runtime_operations(operation_key, domain, title, method, path, operation_kind, danger_level, result_view, assistant_eligible, status, action_type, metadata)
			VALUES ($1, 'ERP', $2, 'POST', $3, 'contextual', 'medium', 'summary', true, 'active', 'erp.action', $4::jsonb)
			ON CONFLICT (operation_key) DO UPDATE SET
				title = EXCLUDED.title,
				path = EXCLUDED.path,
				metadata = platform.runtime_operations.metadata || EXCLUDED.metadata,
				updated_at = NOW()
		`, result.AssetKey, result.AssetKey, path, jsonBytes(map[string]any{"source_change_request_id": request.ID.String(), "asset_key": result.AssetKey}))
		return err
	case AssetTypeToolDefinition, AssetTypeToolPolicy:
		_, err := tx.Exec(ctx, `
			INSERT INTO tool_definitions(name, description, source_type, default_policy, risk_level, required_level, metadata)
			VALUES ($1, 'Generated from ERP industry solution package', 'internal_api', 'approve', 'medium', 'L2', $2::jsonb)
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

func scanSchemaTarget(row interface{ Scan(dest ...any) error }) (*OrganizationSchemaTarget, error) {
	var item OrganizationSchemaTarget
	var metadataJSON []byte
	if err := row.Scan(
		&item.OrganizationID,
		&item.SchemaName,
		&item.TemplateVersion,
		&item.Status,
		&item.LastChangeRequestID,
		&metadataJSON,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan schema target: %w", err)
	}
	item.Metadata = unmarshalMap(metadataJSON)
	return &item, nil
}

func scanSchemaChangeRequest(row interface{ Scan(dest ...any) error }) (*SchemaChangeRequest, error) {
	var item SchemaChangeRequest
	var pkgJSON, statementsJSON, diffJSON []byte
	if err := row.Scan(
		&item.ID,
		&item.OrganizationID,
		&item.SchemaName,
		&item.RequestType,
		&item.Status,
		&item.Reason,
		&pkgJSON,
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
		return nil, fmt.Errorf("scan schema change request: %w", err)
	}
	_ = json.Unmarshal(pkgJSON, &item.SchemaPackage)
	_ = json.Unmarshal(statementsJSON, &item.Statements)
	_ = json.Unmarshal(diffJSON, &item.Diff)
	if item.RiskLevel == "" {
		item.RiskLevel = SchemaRiskSafe
	}
	return &item, nil
}

func scanSchemaApplyJob(row interface{ Scan(dest ...any) error }) (*SchemaApplyJob, error) {
	var item SchemaApplyJob
	var statementsJSON, metadataJSON []byte
	if err := row.Scan(
		&item.ID,
		&item.ChangeRequestID,
		&item.OrganizationID,
		&item.SchemaName,
		&item.Status,
		&statementsJSON,
		&item.ErrorMessage,
		&metadataJSON,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan schema apply job: %w", err)
	}
	_ = json.Unmarshal(statementsJSON, &item.Statements)
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

var _ repository = (*Repository)(nil)
