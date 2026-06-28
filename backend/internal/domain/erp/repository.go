package erp

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/tenantdb"
)

const (
	actionExecutionTable       = "MAEX"
	actionGeneratedRecordTable = "AEX1"
)

type PostgresRepository struct {
	db      tenantdb.DB
	tx      pgx.Tx
	querier erpQuerier
}

type erpQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func NewRepository(db tenantdb.DB) *PostgresRepository {
	return &PostgresRepository{db: db, querier: db}
}

func (r *PostgresRepository) RunInTx(ctx context.Context, fn func(Repository) error) error {
	if r.tx != nil {
		return fn(r)
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	txRepo := &PostgresRepository{db: r.db, tx: tx, querier: tx}
	if err := fn(txRepo); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) ListRecords(ctx context.Context, table TableDefinition, limit int) ([]Record, error) {
	query := fmt.Sprintf(`SELECT %s::TEXT, row_to_json(t)::jsonb, "CreatedAt", "UpdatedAt" FROM %s t ORDER BY "UpdatedAt" DESC LIMIT $1`,
		quoteIdent(table.PrimaryKey), quoteIdent(table.Code))
	rows, err := r.querier.Query(ctx, query, normalizeLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", table.Code, err)
	}
	defer rows.Close()
	return scanRecords(rows, table.Code, "", "")
}

func (r *PostgresRepository) CreateRecord(ctx context.Context, table TableDefinition, input RecordInput) (*Record, error) {
	key := input.Key
	if key == "" {
		key = fmt.Sprint(input.Data[table.PrimaryKey])
	}
	if key == "" || key == "<nil>" {
		return nil, fmt.Errorf("%w: %s key is required", ErrValidation, table.PrimaryKey)
	}
	payload := copyData(input.Data)
	payload[table.PrimaryKey] = key
	record, err := r.insertRecord(ctx, table.Code, table.PrimaryKey, map[string]any{
		table.PrimaryKey: key,
		"Payload":        payload,
	})
	if err != nil {
		return nil, err
	}
	record.TableCode = table.Code
	return record, nil
}

func (r *PostgresRepository) GetRecord(ctx context.Context, table TableDefinition, key string) (*Record, error) {
	query := fmt.Sprintf(`SELECT %s::TEXT, row_to_json(t)::jsonb, "CreatedAt", "UpdatedAt" FROM %s t WHERE %s::TEXT = $1`,
		quoteIdent(table.PrimaryKey), quoteIdent(table.Code), quoteIdent(table.PrimaryKey))
	record, err := scanRecord(table.Code, "", "", r.querier.QueryRow(ctx, query, key))
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (r *PostgresRepository) UpdateRecord(ctx context.Context, table TableDefinition, key string, input RecordInput) (*Record, error) {
	if len(input.Data) == 0 {
		return r.GetRecord(ctx, table, key)
	}
	payload, _ := json.Marshal(input.Data)
	query := fmt.Sprintf(`UPDATE %s SET "Payload" = "Payload" || $1::jsonb, "UpdatedAt" = NOW() WHERE %s::TEXT = $2 RETURNING %s::TEXT, row_to_json(%s)::jsonb, "CreatedAt", "UpdatedAt"`,
		quoteIdent(table.Code), quoteIdent(table.PrimaryKey), quoteIdent(table.PrimaryKey), quoteIdent(table.Code))
	record, err := scanRecord(table.Code, "", "", r.querier.QueryRow(ctx, query, payload, key))
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (r *PostgresRepository) DeleteRecord(ctx context.Context, table TableDefinition, key string) error {
	tag, err := r.querier.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE %s::TEXT = $1`, quoteIdent(table.Code), quoteIdent(table.PrimaryKey)), key)
	if err != nil {
		return fmt.Errorf("delete %s: %w", table.Code, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) ListChildRecords(ctx context.Context, parent TableDefinition, child ChildTableDefinition, parentKey string, limit int) ([]Record, error) {
	query := fmt.Sprintf(`SELECT %s::TEXT, row_to_json(t)::jsonb, "CreatedAt", "UpdatedAt" FROM %s t WHERE %s::TEXT = $1 ORDER BY %s LIMIT $2`,
		quoteIdent(child.LineKey), quoteIdent(child.Code), quoteIdent(child.ParentKey), quoteIdent(child.LineKey))
	rows, err := r.querier.Query(ctx, query, parentKey, normalizeLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", child.Code, err)
	}
	defer rows.Close()
	return scanRecords(rows, child.Code, parent.Code, parentKey)
}

func (r *PostgresRepository) CreateChildRecord(ctx context.Context, parent TableDefinition, child ChildTableDefinition, parentKey string, input RecordInput) (*Record, error) {
	key := input.Key
	if key == "" {
		key = fmt.Sprint(input.Data[child.LineKey])
	}
	if key == "" || key == "<nil>" {
		return nil, fmt.Errorf("%w: %s key is required", ErrValidation, child.LineKey)
	}
	payload := copyData(input.Data)
	payload[child.ParentKey] = parentKey
	payload[child.LineKey] = key
	values := map[string]any{
		child.ParentKey: parentKey,
		child.LineKey:   key,
		"Payload":       payload,
	}
	if lineStatus, ok := input.Data["LineStatus"]; ok {
		values["LineStatus"] = lineStatus
	}
	record, err := r.insertRecord(ctx, child.Code, child.LineKey, values)
	if err != nil {
		return nil, err
	}
	record.TableCode = child.Code
	record.ParentTableCode = parent.Code
	record.ParentKey = parentKey
	return record, nil
}

func (r *PostgresRepository) UpdateChildRecord(ctx context.Context, parent TableDefinition, child ChildTableDefinition, parentKey string, lineKey string, input RecordInput) (*Record, error) {
	if len(input.Data) == 0 {
		query := fmt.Sprintf(`SELECT %s::TEXT, row_to_json(t)::jsonb, "CreatedAt", "UpdatedAt" FROM %s t WHERE %s::TEXT = $1 AND %s::TEXT = $2`,
			quoteIdent(child.LineKey), quoteIdent(child.Code), quoteIdent(child.ParentKey), quoteIdent(child.LineKey))
		return scanRecord(child.Code, parent.Code, parentKey, r.querier.QueryRow(ctx, query, parentKey, lineKey))
	}
	payload := copyData(input.Data)
	payload[child.ParentKey] = parentKey
	payload[child.LineKey] = lineKey
	payloadBytes, _ := json.Marshal(payload)
	query := fmt.Sprintf(`UPDATE %s SET "Payload" = "Payload" || $1::jsonb, "UpdatedAt" = NOW() WHERE %s::TEXT = $2 AND %s::TEXT = $3 RETURNING %s::TEXT, row_to_json(%s)::jsonb, "CreatedAt", "UpdatedAt"`,
		quoteIdent(child.Code), quoteIdent(child.ParentKey), quoteIdent(child.LineKey), quoteIdent(child.LineKey), quoteIdent(child.Code))
	record, err := scanRecord(child.Code, parent.Code, parentKey, r.querier.QueryRow(ctx, query, string(payloadBytes), parentKey, lineKey))
	if err != nil {
		return nil, err
	}
	record.ParentTableCode = parent.Code
	record.ParentKey = parentKey
	return record, nil
}

func (r *PostgresRepository) DeleteChildRecord(ctx context.Context, parent TableDefinition, child ChildTableDefinition, parentKey string, lineKey string) error {
	query := fmt.Sprintf(`DELETE FROM %s WHERE %s::TEXT = $1 AND %s::TEXT = $2`,
		quoteIdent(child.Code), quoteIdent(child.ParentKey), quoteIdent(child.LineKey))
	tag, err := r.querier.Exec(ctx, query, parentKey, lineKey)
	if err != nil {
		return fmt.Errorf("delete %s: %w", child.Code, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) CreateActionExecution(ctx context.Context, execution ActionExecution) (*ActionExecution, error) {
	if execution.ID == uuid.Nil {
		execution.ID = uuid.New()
	}
	if execution.Status == "" {
		execution.Status = ActionExecutionRunning
	}
	if execution.Payload == nil {
		execution.Payload = map[string]any{}
	}
	payload, err := json.Marshal(execution.Payload)
	if err != nil {
		return nil, fmt.Errorf("marshal ERP action execution payload: %w", err)
	}
	query := fmt.Sprintf(`INSERT INTO %s ("ActionID", "TableCode", "RecordKey", "Action", "Status", "IdempotencyKey", "ActorID", "ActorType", "ToolExecutionID", "AssistantSessionID", "Source", "Payload")
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12::jsonb)
		RETURNING "ActionID", "TableCode", "RecordKey", "Action", "Status", "IdempotencyKey", "ActorID", "ActorType", "ToolExecutionID", "AssistantSessionID", "Source", "FailureCode", "FailureMessage", "Payload", "StartedAt", "CompletedAt"`, quoteIdent(actionExecutionTable))
	return scanActionExecution(r.querier.QueryRow(ctx, query, execution.ID, execution.TableCode, execution.RecordKey, execution.Action, execution.Status, execution.IdempotencyKey, execution.ActorID, execution.ActorType, execution.ToolExecutionID, execution.AssistantSessionID, execution.Source, string(payload)))
}

func (r *PostgresRepository) FindActionExecutionByIdempotencyKey(ctx context.Context, key string) (*ActionExecution, error) {
	query := fmt.Sprintf(`SELECT "ActionID", "TableCode", "RecordKey", "Action", "Status", "IdempotencyKey", "ActorID", "ActorType", "ToolExecutionID", "AssistantSessionID", "Source", "FailureCode", "FailureMessage", "Payload", "StartedAt", "CompletedAt"
		FROM %s WHERE "IdempotencyKey" = $1 ORDER BY "StartedAt" DESC LIMIT 1`, quoteIdent(actionExecutionTable))
	return scanActionExecution(r.querier.QueryRow(ctx, query, key))
}

func (r *PostgresRepository) CompleteActionExecution(ctx context.Context, id uuid.UUID, status string, payload map[string]any, failure *ActionFailure) (*ActionExecution, error) {
	if payload == nil {
		payload = map[string]any{}
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal ERP action execution completion payload: %w", err)
	}
	failureCode := ""
	failureMessage := ""
	if failure != nil {
		failureCode = failure.Code
		failureMessage = failure.Message
	}
	query := fmt.Sprintf(`UPDATE %s SET "Status" = $2, "Payload" = $3::jsonb, "FailureCode" = $4, "FailureMessage" = $5, "CompletedAt" = NOW()
		WHERE "ActionID" = $1
		RETURNING "ActionID", "TableCode", "RecordKey", "Action", "Status", "IdempotencyKey", "ActorID", "ActorType", "ToolExecutionID", "AssistantSessionID", "Source", "FailureCode", "FailureMessage", "Payload", "StartedAt", "CompletedAt"`, quoteIdent(actionExecutionTable))
	return scanActionExecution(r.querier.QueryRow(ctx, query, id, status, string(payloadBytes), failureCode, failureMessage))
}

func (r *PostgresRepository) CreateActionGeneratedRecord(ctx context.Context, record ActionGeneratedRecord) error {
	if record.Payload == nil {
		record.Payload = map[string]any{}
	}
	if record.RelationType == "" {
		record.RelationType = "created"
	}
	payload, err := json.Marshal(record.Payload)
	if err != nil {
		return fmt.Errorf("marshal ERP action generated record payload: %w", err)
	}
	query := fmt.Sprintf(`INSERT INTO %s ("ActionID", "LineNum", "GeneratedTableCode", "GeneratedKey", "RelationType", "Payload")
		VALUES ($1,$2,$3,$4,$5,$6::jsonb)`, quoteIdent(actionGeneratedRecordTable))
	if _, err := r.querier.Exec(ctx, query, record.ActionID, record.LineNum, record.GeneratedTableCode, record.GeneratedKey, record.RelationType, string(payload)); err != nil {
		return fmt.Errorf("create ERP action generated record: %w", err)
	}
	return nil
}

func (r *PostgresRepository) ListActionGeneratedRecords(ctx context.Context, actionID uuid.UUID) ([]ActionGeneratedRecord, error) {
	query := fmt.Sprintf(`SELECT "ActionID", "LineNum", "GeneratedTableCode", "GeneratedKey", "RelationType", "Payload" FROM %s WHERE "ActionID" = $1 ORDER BY "LineNum"`, quoteIdent(actionGeneratedRecordTable))
	rows, err := r.querier.Query(ctx, query, actionID)
	if err != nil {
		return nil, fmt.Errorf("list ERP action generated records: %w", err)
	}
	defer rows.Close()
	items := []ActionGeneratedRecord{}
	for rows.Next() {
		var item ActionGeneratedRecord
		var payload []byte
		if err := rows.Scan(&item.ActionID, &item.LineNum, &item.GeneratedTableCode, &item.GeneratedKey, &item.RelationType, &payload); err != nil {
			return nil, err
		}
		item.Payload = map[string]any{}
		_ = json.Unmarshal(payload, &item.Payload)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *PostgresRepository) ListActionExecutions(ctx context.Context, tableCode string, recordKey string, limit int) ([]ActionExecution, error) {
	query := fmt.Sprintf(`SELECT "ActionID", "TableCode", "RecordKey", "Action", "Status", "IdempotencyKey", "ActorID", "ActorType", "ToolExecutionID", "AssistantSessionID", "Source", "FailureCode", "FailureMessage", "Payload", "StartedAt", "CompletedAt"
		FROM %s WHERE "TableCode" = $1 AND "RecordKey" = $2 ORDER BY "StartedAt" DESC LIMIT $3`, quoteIdent(actionExecutionTable))
	rows, err := r.querier.Query(ctx, query, tableCode, recordKey, normalizeLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("list ERP action executions: %w", err)
	}
	defer rows.Close()
	items := []ActionExecution{}
	for rows.Next() {
		item, err := scanActionExecution(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *PostgresRepository) insertRecord(ctx context.Context, tableCode, keyColumn string, values map[string]any) (*Record, error) {
	names := sortedKeys(values)
	columns := make([]string, 0, len(names))
	placeholders := make([]string, 0, len(names))
	args := make([]any, 0, len(names))
	for i, name := range names {
		columns = append(columns, quoteIdent(name))
		placeholders = append(placeholders, "$"+strconv.Itoa(i+1))
		args = append(args, normalizeValue(values[name]))
	}
	query := fmt.Sprintf(`INSERT INTO %s (%s) VALUES (%s) RETURNING %s::TEXT, row_to_json(%s)::jsonb, "CreatedAt", "UpdatedAt"`,
		quoteIdent(tableCode), strings.Join(columns, ", "), strings.Join(placeholders, ", "), quoteIdent(keyColumn), quoteIdent(tableCode))
	return scanRecord(tableCode, "", "", r.querier.QueryRow(ctx, query, args...))
}

func scanRecords(rows pgx.Rows, tableCode, parentTableCode, parentKey string) ([]Record, error) {
	records := []Record{}
	for rows.Next() {
		record, err := scanRecord(tableCode, parentTableCode, parentKey, rows)
		if err != nil {
			return nil, err
		}
		records = append(records, *record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

func scanRecord(tableCode, parentTableCode, parentKey string, row interface{ Scan(dest ...any) error }) (*Record, error) {
	var key string
	var dataJSON []byte
	var record Record
	if err := row.Scan(&key, &dataJSON, &record.CreatedAt, &record.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	record.TableCode = tableCode
	record.ParentTableCode = parentTableCode
	record.ParentKey = parentKey
	record.Key = key
	record.Data = map[string]any{}
	_ = json.Unmarshal(dataJSON, &record.Data)
	if payload, ok := record.Data["Payload"].(map[string]any); ok {
		for k, v := range payload {
			record.Data[k] = v
		}
		delete(record.Data, "Payload")
	}
	return &record, nil
}

func scanActionExecution(row interface{ Scan(dest ...any) error }) (*ActionExecution, error) {
	var execution ActionExecution
	var actorID pgtype.UUID
	var toolExecutionID pgtype.UUID
	var assistantSessionID pgtype.UUID
	var payload []byte
	var completedAt pgtype.Timestamptz
	if err := row.Scan(
		&execution.ID,
		&execution.TableCode,
		&execution.RecordKey,
		&execution.Action,
		&execution.Status,
		&execution.IdempotencyKey,
		&actorID,
		&execution.ActorType,
		&toolExecutionID,
		&assistantSessionID,
		&execution.Source,
		&execution.FailureCode,
		&execution.FailureMessage,
		&payload,
		&execution.StartedAt,
		&completedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	execution.ActorID = uuidPointer(actorID)
	execution.ToolExecutionID = uuidPointer(toolExecutionID)
	execution.AssistantSessionID = uuidPointer(assistantSessionID)
	if completedAt.Valid {
		completed := completedAt.Time
		execution.CompletedAt = &completed
	}
	execution.Payload = map[string]any{}
	_ = json.Unmarshal(payload, &execution.Payload)
	return &execution, nil
}

var identPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func quoteIdent(value string) string {
	if !identPattern.MatchString(value) {
		return `""`
	}
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func sortedKeys(values map[string]any) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 100
	}
	if limit > 500 {
		return 500
	}
	return limit
}

func normalizeValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		b, _ := json.Marshal(typed)
		return string(b)
	case []any:
		b, _ := json.Marshal(typed)
		return string(b)
	default:
		return typed
	}
}

func copyData(values map[string]any) map[string]any {
	copied := map[string]any{}
	for k, v := range values {
		copied[k] = v
	}
	return copied
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

func postgresColumnType(field FieldDefinition) string {
	switch strings.ToLower(field.DataType) {
	case "int":
		return "BIGINT"
	case "numeric", "decimal":
		if field.Size != "" {
			return "NUMERIC(" + field.Size + ")"
		}
		return "NUMERIC"
	case "date":
		return "DATE"
	case "jsonb":
		return "JSONB"
	case "varchar", "nvarchar":
		if field.Size != "" {
			return "VARCHAR(" + field.Size + ")"
		}
		return "TEXT"
	default:
		return "TEXT"
	}
}
