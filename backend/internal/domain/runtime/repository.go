package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/tenantdb"
)

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) ListOperations(ctx context.Context) ([]OperationDefinition, error) {
	rows, err := r.db.Query(ctx, `
		SELECT operation_key, domain, title, method, path, auth, path_params, query_params,
		       body_template, operation_kind, danger_level, result_view, assistant_eligible,
		       requires_entity_context, status, action_type, COALESCE(entity_key, ''), adapter_key, metadata,
		       created_at, updated_at
		FROM platform.runtime_operations
		ORDER BY domain, operation_key
	`)
	if err != nil {
		return nil, fmt.Errorf("list runtime operations: %w", err)
	}
	defer rows.Close()

	items := []OperationDefinition{}
	for rows.Next() {
		item, err := scanOperation(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list runtime operations iteration: %w", err)
	}
	return items, nil
}

func (r *PostgresRepository) GetOperation(ctx context.Context, operationID string) (*OperationDefinition, error) {
	item, err := scanOperation(r.db.QueryRow(ctx, `
		SELECT operation_key, domain, title, method, path, auth, path_params, query_params,
		       body_template, operation_kind, danger_level, result_view, assistant_eligible,
		       requires_entity_context, status, action_type, COALESCE(entity_key, ''), adapter_key, metadata,
		       created_at, updated_at
		FROM platform.runtime_operations
		WHERE operation_key = $1
	`, operationID))
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (r *PostgresRepository) GetEntity(ctx context.Context, entityKey string) (*EntityDefinition, error) {
	item, err := scanEntity(r.db.QueryRow(ctx, `
		SELECT entity_key, module_key, storage_table, entity_type, title_key, status, fields, metadata, created_at, updated_at
		FROM platform.runtime_entities
		WHERE entity_key = $1
	`, entityKey))
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (r *PostgresRepository) ListRecords(ctx context.Context, entity EntityDefinition, limit int) ([]RuntimeRecord, error) {
	schema, table, err := recordTarget(ctx, entity)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT master_key, entity_type, title, status, core_data, metadata, created_at, updated_at
		FROM %s.%s
		WHERE entity_type = $1 AND status <> 'archived'
		ORDER BY updated_at DESC, master_key
		LIMIT $2
	`, schema, table), entity.EntityType, normalizeLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("list runtime records: %w", err)
	}
	defer rows.Close()

	records := []RuntimeRecord{}
	for rows.Next() {
		record, err := scanRecord(entity.EntityKey, rows)
		if err != nil {
			return nil, err
		}
		records = append(records, *record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list runtime records iteration: %w", err)
	}
	return records, nil
}

func (r *PostgresRepository) GetRecord(ctx context.Context, entity EntityDefinition, recordKey string) (*RuntimeRecord, error) {
	schema, table, err := recordTarget(ctx, entity)
	if err != nil {
		return nil, err
	}
	return scanRecord(entity.EntityKey, r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT master_key, entity_type, title, status, core_data, metadata, created_at, updated_at
		FROM %s.%s
		WHERE master_key = $1 AND entity_type = $2 AND status <> 'archived'
	`, schema, table), recordKey, entity.EntityType))
}

func (r *PostgresRepository) CreateRecord(ctx context.Context, entity EntityDefinition, input RuntimeRecordInput) (*RuntimeRecord, error) {
	schema, table, err := recordTarget(ctx, entity)
	if err != nil {
		return nil, err
	}
	dataJSON, _ := json.Marshal(input.Data)
	metadataJSON, _ := json.Marshal(input.Metadata)
	masterKey := newRuntimeKey(entity)
	return scanRecord(entity.EntityKey, r.db.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.%s(master_key, entity_type, title, status, core_data, metadata)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING master_key, entity_type, title, status, core_data, metadata, created_at, updated_at
	`, schema, table), masterKey, entity.EntityType, input.Title, input.Status, dataJSON, metadataJSON))
}

func (r *PostgresRepository) UpdateRecord(ctx context.Context, entity EntityDefinition, recordKey string, input RuntimeRecordInput) (*RuntimeRecord, error) {
	schema, table, err := recordTarget(ctx, entity)
	if err != nil {
		return nil, err
	}
	dataJSON, _ := json.Marshal(input.Data)
	metadataJSON, _ := json.Marshal(input.Metadata)
	return scanRecord(entity.EntityKey, r.db.QueryRow(ctx, fmt.Sprintf(`
		UPDATE %s.%s
		SET title = COALESCE(NULLIF($2, ''), title),
		    status = COALESCE(NULLIF($3, ''), status),
		    core_data = core_data || $4::jsonb,
		    metadata = metadata || $5::jsonb,
		    updated_at = NOW()
		WHERE master_key = $1 AND entity_type = $6 AND status <> 'archived'
		RETURNING master_key, entity_type, title, status, core_data, metadata, created_at, updated_at
	`, schema, table), recordKey, input.Title, input.Status, dataJSON, metadataJSON, entity.EntityType))
}

func (r *PostgresRepository) DeleteRecord(ctx context.Context, entity EntityDefinition, recordKey string) error {
	schema, table, err := recordTarget(ctx, entity)
	if err != nil {
		return err
	}
	tag, err := r.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.%s
		SET status = 'archived', updated_at = NOW()
		WHERE master_key = $1 AND entity_type = $2 AND status <> 'archived'
	`, schema, table), recordKey, entity.EntityType)
	if err != nil {
		return fmt.Errorf("delete runtime record: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func recordTarget(ctx context.Context, entity EntityDefinition) (string, string, error) {
	schemaName, err := schemaNameFromContext(ctx)
	if err != nil {
		return "", "", err
	}
	quotedSchema, err := tenantdb.QuoteIdentifier(schemaName)
	if err != nil {
		return "", "", err
	}
	quotedTable, err := tenantdb.QuoteIdentifier(entity.StorageTable)
	if err != nil {
		return "", "", fmt.Errorf("%w: invalid storage_table", ErrValidation)
	}
	return quotedSchema, quotedTable, nil
}

func schemaNameFromContext(ctx context.Context) (string, error) {
	tenant, ok := middleware.TenantFromContext(ctx)
	if !ok || tenant.OrganizationID == nil {
		return "", fmt.Errorf("%w: organization context is required", ErrValidation)
	}
	return tenantdb.SchemaNameForOrganization(*tenant.OrganizationID), nil
}

func scanEntity(row interface{ Scan(dest ...any) error }) (*EntityDefinition, error) {
	var item EntityDefinition
	var fieldsJSON, metadataJSON []byte
	if err := row.Scan(
		&item.EntityKey,
		&item.ModuleKey,
		&item.StorageTable,
		&item.EntityType,
		&item.TitleKey,
		&item.Status,
		&fieldsJSON,
		&metadataJSON,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, mapNotFound(err, "get runtime entity")
	}
	_ = json.Unmarshal(fieldsJSON, &item.Fields)
	item.Metadata = unmarshalMap(metadataJSON)
	return &item, nil
}

func scanOperation(row interface{ Scan(dest ...any) error }) (*OperationDefinition, error) {
	var item OperationDefinition
	var pathParamsJSON, queryParamsJSON, bodyTemplateJSON, metadataJSON []byte
	if err := row.Scan(
		&item.ID,
		&item.Domain,
		&item.Title,
		&item.Method,
		&item.Path,
		&item.Auth,
		&pathParamsJSON,
		&queryParamsJSON,
		&bodyTemplateJSON,
		&item.OperationKind,
		&item.DangerLevel,
		&item.ResultView,
		&item.AssistantEligible,
		&item.RequiresEntityContext,
		&item.Status,
		&item.ActionType,
		&item.EntityKey,
		&item.AdapterKey,
		&metadataJSON,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, mapNotFound(err, "get runtime operation")
	}
	_ = json.Unmarshal(pathParamsJSON, &item.PathParams)
	_ = json.Unmarshal(queryParamsJSON, &item.QueryParams)
	if len(bodyTemplateJSON) > 0 && string(bodyTemplateJSON) != "null" {
		_ = json.Unmarshal(bodyTemplateJSON, &item.BodyTemplate)
	}
	item.Metadata = unmarshalMap(metadataJSON)
	return &item, nil
}

func scanRecord(entityKey string, row interface{ Scan(dest ...any) error }) (*RuntimeRecord, error) {
	var item RuntimeRecord
	var dataJSON, metadataJSON []byte
	if err := row.Scan(
		&item.MasterKey,
		&item.EntityType,
		&item.Title,
		&item.Status,
		&dataJSON,
		&metadataJSON,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, mapNotFound(err, "scan runtime record")
	}
	item.EntityKey = entityKey
	item.Data = unmarshalMap(dataJSON)
	item.Metadata = unmarshalMap(metadataJSON)
	return &item, nil
}

func mapNotFound(err error, prefix string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return fmt.Errorf("%s: %w", prefix, err)
}

func unmarshalMap(data []byte) map[string]any {
	out := map[string]any{}
	if len(data) == 0 {
		return out
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func newRuntimeKey(entity EntityDefinition) string {
	prefix := strings.ToUpper(strings.ReplaceAll(entity.ModuleKey, "_", ""))
	if len(prefix) > 6 {
		prefix = prefix[:6]
	}
	if prefix == "" {
		prefix = "RT"
	}
	return fmt.Sprintf("%s-%s-%s", prefix, time.Now().UTC().Format("20060102"), strings.ToUpper(strings.ReplaceAll(uuid.NewString(), "-", ""))[:8])
}
