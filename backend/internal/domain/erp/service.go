package erp

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Repository interface {
	ListRecords(ctx context.Context, table TableDefinition, limit int) ([]Record, error)
	CreateRecord(ctx context.Context, table TableDefinition, input RecordInput) (*Record, error)
	GetRecord(ctx context.Context, table TableDefinition, key string) (*Record, error)
	UpdateRecord(ctx context.Context, table TableDefinition, key string, input RecordInput) (*Record, error)
	DeleteRecord(ctx context.Context, table TableDefinition, key string) error
	ListChildRecords(ctx context.Context, parent TableDefinition, child ChildTableDefinition, parentKey string, limit int) ([]Record, error)
	CreateChildRecord(ctx context.Context, parent TableDefinition, child ChildTableDefinition, parentKey string, input RecordInput) (*Record, error)
	UpdateChildRecord(ctx context.Context, parent TableDefinition, child ChildTableDefinition, parentKey string, lineKey string, input RecordInput) (*Record, error)
	DeleteChildRecord(ctx context.Context, parent TableDefinition, child ChildTableDefinition, parentKey string, lineKey string) error
	CreateActionExecution(ctx context.Context, execution ActionExecution) (*ActionExecution, error)
	FindActionExecutionByIdempotencyKey(ctx context.Context, key string) (*ActionExecution, error)
	CompleteActionExecution(ctx context.Context, id uuid.UUID, status string, payload map[string]any, failure *ActionFailure) (*ActionExecution, error)
	CreateActionGeneratedRecord(ctx context.Context, record ActionGeneratedRecord) error
	ListActionGeneratedRecords(ctx context.Context, actionID uuid.UUID) ([]ActionGeneratedRecord, error)
	ListActionExecutions(ctx context.Context, tableCode string, recordKey string, limit int) ([]ActionExecution, error)
}

type TransactionalRepository interface {
	RunInTx(context.Context, func(Repository) error) error
}

type actionExecutionContextKey struct{}

type actionExecutionContext struct {
	ExecutionID        uuid.UUID
	IdempotencyKey     string
	ActorType          string
	ToolExecutionID    *uuid.UUID
	AssistantSessionID *uuid.UUID
}

func contextWithActionExecution(ctx context.Context, meta actionExecutionContext) context.Context {
	return context.WithValue(ctx, actionExecutionContextKey{}, meta)
}

func actionExecutionFromContext(ctx context.Context) actionExecutionContext {
	meta, _ := ctx.Value(actionExecutionContextKey{}).(actionExecutionContext)
	return meta
}

type Service struct {
	repo    Repository
	catalog Catalog
	actions ActionRegistry
}

func NewService(repo Repository, catalog Catalog) *Service {
	if catalog.byCode == nil {
		catalog = DefaultCatalog()
	}
	return &Service{repo: repo, catalog: catalog, actions: DefaultActionRegistry()}
}

func (s *Service) Catalog(ctx context.Context) Catalog {
	return s.catalog
}

func (s *Service) Actions(ctx context.Context) []ActionDefinition {
	return s.actions.List()
}

func (s *Service) ListRecords(ctx context.Context, tableCode string, limit int) ([]Record, error) {
	if err := s.authorize(ctx, tableCode, "read"); err != nil {
		return nil, err
	}
	table, err := s.table(tableCode)
	if err != nil {
		return nil, err
	}
	return s.repo.ListRecords(ctx, table, limit)
}

func (s *Service) CreateRecord(ctx context.Context, tableCode string, input RecordInput) (*Record, error) {
	if err := s.authorize(ctx, tableCode, "create"); err != nil {
		return nil, err
	}
	table, err := s.table(tableCode)
	if err != nil {
		return nil, err
	}
	input = flattenPayloadInput(input)
	if err := validateRecordInput(table, input); err != nil {
		return nil, err
	}
	if err := validateDraftInput(table, input); err != nil {
		return nil, err
	}
	return s.recordInTx(ctx, func(tx *Service) (*Record, error) {
		return tx.repo.CreateRecord(ctx, table, input)
	})
}

func (s *Service) GetRecord(ctx context.Context, tableCode, key string) (*Record, error) {
	if err := s.authorize(ctx, tableCode, "read"); err != nil {
		return nil, err
	}
	table, err := s.table(tableCode)
	if err != nil {
		return nil, err
	}
	return s.repo.GetRecord(ctx, table, key)
}

func (s *Service) UpdateRecord(ctx context.Context, tableCode, key string, input RecordInput) (*Record, error) {
	if err := s.authorize(ctx, tableCode, "update"); err != nil {
		return nil, err
	}
	table, err := s.table(tableCode)
	if err != nil {
		return nil, err
	}
	input = flattenPayloadInput(input)
	if err := validateIdentityInput(input, key, table.PrimaryKey); err != nil {
		return nil, err
	}
	if err := validateRecordInput(table, input); err != nil {
		return nil, err
	}
	if err := validateDraftInput(table, input); err != nil {
		return nil, err
	}
	return s.recordInTx(ctx, func(tx *Service) (*Record, error) {
		if err := tx.ensureEditable(ctx, table, key); err != nil {
			return nil, err
		}
		return tx.repo.UpdateRecord(ctx, table, key, input)
	})
}

func (s *Service) DeleteRecord(ctx context.Context, tableCode, key string) error {
	if err := s.authorize(ctx, tableCode, "delete"); err != nil {
		return err
	}
	table, err := s.table(tableCode)
	if err != nil {
		return err
	}
	_, err = s.recordInTx(ctx, func(tx *Service) (*Record, error) {
		if err := tx.ensureEditable(ctx, table, key); err != nil {
			return nil, err
		}
		return nil, tx.repo.DeleteRecord(ctx, table, key)
	})
	return err
}

func (s *Service) ListChildRecords(ctx context.Context, tableCode, parentKey, childCode string, limit int) ([]Record, error) {
	if err := s.authorize(ctx, tableCode, "read"); err != nil {
		return nil, err
	}
	parent, child, err := s.child(tableCode, childCode)
	if err != nil {
		return nil, err
	}
	return s.repo.ListChildRecords(ctx, parent, child, parentKey, limit)
}

func (s *Service) CreateChildRecord(ctx context.Context, tableCode, parentKey, childCode string, input RecordInput) (*Record, error) {
	if tableCode == "MRCT" || tableCode == "MVPM" {
		return nil, fmt.Errorf("%w: allocation lines are maintained by payment actions", ErrValidation)
	}
	if err := s.authorize(ctx, tableCode, "update"); err != nil {
		return nil, err
	}
	parent, child, err := s.child(tableCode, childCode)
	if err != nil {
		return nil, err
	}
	input = flattenPayloadInput(input)
	if err := validateChildRecordInput(child, input); err != nil {
		return nil, err
	}
	return s.recordInTx(ctx, func(tx *Service) (*Record, error) {
		if err := tx.ensureEditable(ctx, parent, parentKey); err != nil {
			return nil, err
		}
		return tx.repo.CreateChildRecord(ctx, parent, child, parentKey, input)
	})
}

func (s *Service) UpdateChildRecord(ctx context.Context, tableCode, parentKey, childCode, lineKey string, input RecordInput) (*Record, error) {
	if tableCode == "MRCT" || tableCode == "MVPM" {
		return nil, fmt.Errorf("%w: allocation lines are maintained by payment actions", ErrValidation)
	}
	if err := s.authorize(ctx, tableCode, "update"); err != nil {
		return nil, err
	}
	parent, child, err := s.child(tableCode, childCode)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(parentKey) == "" || strings.TrimSpace(lineKey) == "" {
		return nil, fmt.Errorf("%w: child parent key and line key are required", ErrValidation)
	}
	input = flattenPayloadInput(input)
	if err := validateIdentityInput(input, lineKey, child.LineKey); err != nil {
		return nil, err
	}
	if value, ok := input.Data[child.ParentKey]; ok && fmt.Sprint(value) != parentKey {
		return nil, fmt.Errorf("%w: a line cannot be moved to another document", ErrValidation)
	}
	if err := validateChildRecordInput(child, input); err != nil {
		return nil, err
	}
	return s.recordInTx(ctx, func(tx *Service) (*Record, error) {
		if err := tx.ensureEditable(ctx, parent, parentKey); err != nil {
			return nil, err
		}
		return tx.repo.UpdateChildRecord(ctx, parent, child, parentKey, lineKey, input)
	})
}

func (s *Service) DeleteChildRecord(ctx context.Context, tableCode, parentKey, childCode, lineKey string) error {
	if tableCode == "MRCT" || tableCode == "MVPM" {
		return fmt.Errorf("%w: allocation lines are maintained by payment actions", ErrValidation)
	}
	if err := s.authorize(ctx, tableCode, "delete"); err != nil {
		return err
	}
	parent, child, err := s.child(tableCode, childCode)
	if err != nil {
		return err
	}
	if strings.TrimSpace(parentKey) == "" || strings.TrimSpace(lineKey) == "" {
		return fmt.Errorf("%w: child parent key and line key are required", ErrValidation)
	}
	_, err = s.recordInTx(ctx, func(tx *Service) (*Record, error) {
		if err := tx.ensureEditable(ctx, parent, parentKey); err != nil {
			return nil, err
		}
		return nil, tx.repo.DeleteChildRecord(ctx, parent, child, parentKey, lineKey)
	})
	return err
}

func (s *Service) ListActionExecutions(ctx context.Context, tableCode string, recordKey string, limit int) ([]ActionExecution, error) {
	if err := s.authorize(ctx, tableCode, "read"); err != nil {
		return nil, err
	}
	if _, err := s.table(tableCode); err != nil {
		return nil, err
	}
	if strings.TrimSpace(recordKey) == "" {
		return nil, fmt.Errorf("%w: record key is required", ErrValidation)
	}
	items, err := s.repo.ListActionExecutions(ctx, tableCode, recordKey, limit)
	if err != nil {
		return nil, err
	}
	for i := range items {
		generated, err := s.repo.ListActionGeneratedRecords(ctx, items[i].ID)
		if err != nil {
			return nil, err
		}
		items[i].GeneratedRecords = generated
	}
	return items, nil
}

func (s *Service) RunAction(ctx context.Context, tableCode string, key string, action string, input ActionInput) (*ActionResult, error) {
	if _, err := json.Marshal(input.Data); err != nil {
		return nil, fmt.Errorf("%w: invalid action parameters: %v", ErrValidation, err)
	}
	if tableCode == "MGLR" && action == "run" && input.IdempotencyKey == "" {
		input.IdempotencyKey = uuid.NewString()
	}
	if strings.TrimSpace(key) == "" {
		return nil, fmt.Errorf("%w: record key is required", ErrValidation)
	}
	if err := s.authorize(ctx, tableCode, action); err != nil {
		return nil, err
	}
	if err := s.authorizeActionDependencies(ctx, tableCode, action); err != nil {
		return nil, err
	}
	if _, ok := s.actions.Lookup(tableCode, action); !ok {
		return nil, fmt.Errorf("%w: unknown action %s for %s", ErrValidation, action, tableCode)
	}
	result, err := s.runInTx(ctx, func(tx *Service) (*ActionResult, error) {
		return tx.runAction(ctx, tableCode, key, action, input)
	})
	if err != nil && !errors.Is(err, ErrConflict) {
		// A failed transaction has no business effects. Keep its diagnostic audit
		// separately; successful effects and their audit commit atomically above.
		auditCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		failure := actionFailureFromError(err)
		_, _ = s.runInTx(auditCtx, func(tx *Service) (*ActionResult, error) {
			// A retry may have committed after the failed transaction released its
			// lock. Recheck under the same lock before recording its diagnostic.
			idempotencyKey := s.effectiveIdempotencyKey(tableCode, key, action, input)
			existing, findErr := tx.repo.FindActionExecutionByIdempotencyKey(auditCtx, idempotencyKey)
			if findErr == nil && existing.Status != ActionExecutionFailed {
				return nil, nil
			}
			if findErr != nil && !errors.Is(findErr, ErrNotFound) {
				return nil, findErr
			}
			execution, auditErr := tx.repo.CreateActionExecution(auditCtx, ActionExecution{
				TableCode: tableCode, RecordKey: key, Action: action,
				Status: ActionExecutionFailed, IdempotencyKey: idempotencyKey,
				ActorID: input.ActorID, ActorType: input.ActorType, ToolExecutionID: input.ToolExecutionID,
				AssistantSessionID: input.AssistantSessionID, Source: firstNonEmptyString(input.Source, "tenant_api"),
				Payload: map[string]any{"data": input.Data, "request_hash": actionRequestHash(input)},
			})
			if auditErr != nil {
				return nil, auditErr
			}
			_, auditErr = tx.repo.CompleteActionExecution(auditCtx, execution.ID, ActionExecutionFailed, execution.Payload, failure)
			return nil, auditErr
		})
	}
	return result, err
}

func (s *Service) runAction(ctx context.Context, tableCode string, key string, action string, input ActionInput) (*ActionResult, error) {
	if input.Data == nil {
		input.Data = map[string]any{}
	}
	if _, err := s.table(tableCode); err != nil {
		return nil, err
	}
	_, ok := s.actions.Lookup(tableCode, action)
	if !ok {
		return nil, fmt.Errorf("%w: unknown action %s for %s", ErrValidation, action, tableCode)
	}
	idempotencyKey := s.effectiveIdempotencyKey(tableCode, key, action, input)
	if existing, err := s.repo.FindActionExecutionByIdempotencyKey(ctx, idempotencyKey); err == nil {
		if existing.Status == ActionExecutionCompleted {
			if hash, _ := existing.Payload["request_hash"].(string); hash != "" && hash != actionRequestHash(input) {
				return nil, fmt.Errorf("%w: idempotency key was used with different action parameters", ErrConflict)
			}
			return s.idempotentReplayResult(ctx, existing)
		}
		if existing.Status == ActionExecutionRunning {
			return nil, fmt.Errorf("%w: action is already running", ErrConflict)
		}
		if existing.Status == ActionExecutionFailed && input.Data["retry_failed"] != true {
			return nil, fmt.Errorf("%w: previous ERP action execution failed; pass retry_failed=true to retry", ErrValidation)
		}
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	execution, err := s.repo.CreateActionExecution(ctx, ActionExecution{
		TableCode:          tableCode,
		RecordKey:          key,
		Action:             action,
		Status:             ActionExecutionRunning,
		IdempotencyKey:     idempotencyKey,
		ActorID:            input.ActorID,
		ActorType:          input.ActorType,
		ToolExecutionID:    input.ToolExecutionID,
		AssistantSessionID: input.AssistantSessionID,
		Source:             firstNonEmptyString(input.Source, "tenant_api"),
		Payload:            map[string]any{"data": input.Data, "request_hash": actionRequestHash(input)},
	})
	if err != nil {
		return nil, err
	}
	ctx = contextWithActionExecution(ctx, actionExecutionContext{
		ExecutionID:        execution.ID,
		IdempotencyKey:     idempotencyKey,
		ActorType:          input.ActorType,
		ToolExecutionID:    input.ToolExecutionID,
		AssistantSessionID: input.AssistantSessionID,
	})
	result, actionErr := s.runBusinessAction(ctx, tableCode, key, action, input)
	if actionErr != nil {
		return nil, actionErr
	}
	if result == nil {
		return nil, fmt.Errorf("%w: action produced no result", ErrValidation)
	}
	result.ExecutionID = execution.ID
	result.IdempotencyKey = idempotencyKey
	if result.Provenance == nil {
		result.Provenance = map[string]any{}
	}
	result.Provenance["source"] = firstNonEmptyString(input.Source, "tenant_api")
	result.Provenance["action_execution_id"] = execution.ID.String()
	result.Provenance["idempotency_key"] = idempotencyKey
	if len(result.PreconditionsChecked) == 0 {
		result.PreconditionsChecked = []ActionPrecondition{{Key: tableCode + "." + action, Status: "passed"}}
	}
	if err := s.recordGeneratedRecords(ctx, execution.ID, result.GeneratedRecords); err != nil {
		return nil, err
	}
	payload := actionResultPayload(result)
	payload["request_hash"] = actionRequestHash(input)
	if _, err := s.repo.CompleteActionExecution(ctx, execution.ID, ActionExecutionCompleted, payload, nil); err != nil {
		return nil, err
	}
	return result, nil
}

func actionRequestHash(input ActionInput) string {
	data := copyData(input.Data)
	delete(data, "retry_failed")
	encoded, _ := json.Marshal(data)
	return fmt.Sprintf("%x", sha256.Sum256(encoded))
}

func (s *Service) effectiveIdempotencyKey(tableCode string, key string, action string, input ActionInput) string {
	suffix := strings.TrimSpace(input.IdempotencyKey)
	if suffix == "" {
		suffix = defaultActionIdempotencySuffix(tableCode, key, action, input)
	}
	return "erp:" + tableCode + ":" + key + ":" + action + ":" + suffix
}

func defaultActionIdempotencySuffix(tableCode string, key string, action string, input ActionInput) string {
	switch tableCode + ":" + action {
	case "MREQ:convert-to-project":
		return stringValue(input.Data, "PrjCode", "PRJ-"+key)
	case "MPRJ:refresh-cost":
		return stringValue(input.Data, "CostCode", "COST-"+key)
	case "MPRJ:close-feedback":
		return stringValue(input.Data, "FeedbackCode", "FDB-"+key)
	case "MPDN:post":
		return "IGN-" + key + "|AP-" + key
	case "MDLN:post":
		return "IGE-" + key + "|INV-" + key
	case "MINV:post", "MPCH:post":
		return "JE-" + key
	case "MRCT:allocate":
		return stringValue(input.Data, "TargetTable", "MINV") + "|" + stringValue(input.Data, "TargetKey", "") + "|" + fmt.Sprint(numericValue(input.Data, "Amount"))
	case "MVPM:allocate":
		return "MPCH|" + stringValue(input.Data, "TargetKey", "") + "|" + fmt.Sprint(numericValue(input.Data, "Amount"))
	case "MGLR:run":
		return uuid.NewString()
	case "MIGN:post":
		return "MIGN-" + key
	case "MIGE:post":
		return "MIGE-" + key
	case "MJDT:post":
		return "MJDT-" + key
	default:
		return "default"
	}
}

func actionFailureFromError(err error) *ActionFailure {
	if err == nil {
		return nil
	}
	code := "action_failed"
	if errors.Is(err, ErrValidation) {
		code = "validation_failed"
	}
	return &ActionFailure{Code: code, Message: err.Error()}
}

func actionResultPayload(result *ActionResult) map[string]any {
	if result == nil {
		return map[string]any{}
	}
	return map[string]any{
		"status":            result.Status,
		"generated_records": len(result.GeneratedRecords),
		"effects":           result.Effects,
	}
}

func (s *Service) idempotentReplayResult(ctx context.Context, execution *ActionExecution) (*ActionResult, error) {
	generatedRows, err := s.repo.ListActionGeneratedRecords(ctx, execution.ID)
	if err != nil {
		return nil, err
	}
	generated := make([]Record, 0, len(generatedRows))
	for _, row := range generatedRows {
		generated = append(generated, Record{TableCode: row.GeneratedTableCode, Key: row.GeneratedKey, Data: row.Payload})
	}
	return &ActionResult{
		TableCode:            execution.TableCode,
		Key:                  execution.RecordKey,
		Action:               execution.Action,
		Status:               ActionExecutionIdempotentReplay,
		GeneratedRecords:     generated,
		ExecutionID:          execution.ID,
		IdempotencyKey:       execution.IdempotencyKey,
		PreconditionsChecked: []ActionPrecondition{{Key: "idempotency", Status: "passed", Message: "completed execution replayed"}},
		Provenance: map[string]any{
			"source":              execution.Source,
			"action_execution_id": execution.ID.String(),
			"idempotency_key":     execution.IdempotencyKey,
		},
	}, nil
}

func (s *Service) recordGeneratedRecords(ctx context.Context, executionID uuid.UUID, records []Record) error {
	for i, record := range records {
		payload := copyData(record.Data)
		payload["table_code"] = record.TableCode
		payload["key"] = record.Key
		if err := s.repo.CreateActionGeneratedRecord(ctx, ActionGeneratedRecord{
			ActionID:           executionID,
			LineNum:            i + 1,
			GeneratedTableCode: record.TableCode,
			GeneratedKey:       record.Key,
			RelationType:       "created",
			Payload:            payload,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) runInTx(ctx context.Context, fn func(*Service) (*ActionResult, error)) (*ActionResult, error) {
	txRepo, ok := s.repo.(TransactionalRepository)
	if !ok {
		return fn(s)
	}
	var result *ActionResult
	err := txRepo.RunInTx(ctx, func(repo Repository) error {
		txService := &Service{repo: repo, catalog: s.catalog, actions: s.actions}
		var actionErr error
		result, actionErr = fn(txService)
		return actionErr
	})
	return result, err
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (s *Service) table(tableCode string) (TableDefinition, error) {
	table, ok := s.catalog.Table(tableCode)
	if !ok {
		return TableDefinition{}, fmt.Errorf("%w: unknown table %s", ErrValidation, tableCode)
	}
	return table, nil
}

func (s *Service) child(tableCode, childCode string) (TableDefinition, ChildTableDefinition, error) {
	parent, err := s.table(tableCode)
	if err != nil {
		return TableDefinition{}, ChildTableDefinition{}, err
	}
	child, ok := parent.Child(childCode)
	if !ok {
		return TableDefinition{}, ChildTableDefinition{}, fmt.Errorf("%w: unknown child table %s for %s", ErrValidation, childCode, tableCode)
	}
	return parent, child, nil
}

func validateRecordInput(table TableDefinition, input RecordInput) error {
	if err := validateLedgerPayload(table.Code, input.Data); err != nil {
		return err
	}
	_, acceptsPayload := table.Field("Payload")
	for name := range input.Data {
		if _, ok := table.Field(name); !ok {
			if acceptsPayload {
				continue
			}
			return fmt.Errorf("%w: unknown field %s for %s", ErrValidation, name, table.Code)
		}
	}
	return nil
}

func validateIdentityInput(input RecordInput, key, field string) error {
	if input.Key != "" && input.Key != key {
		return fmt.Errorf("%w: record identity is immutable", ErrValidation)
	}
	if value, ok := input.Data[field]; ok && fmt.Sprint(value) != key {
		return fmt.Errorf("%w: %s is immutable", ErrValidation, field)
	}
	return nil
}

func validateChildRecordInput(child ChildTableDefinition, input RecordInput) error {
	if err := validateLedgerPayload(child.Code, input.Data); err != nil {
		return err
	}
	_, acceptsPayload := child.Field("Payload")
	for name := range input.Data {
		if _, ok := child.Field(name); !ok {
			if acceptsPayload {
				continue
			}
			return fmt.Errorf("%w: unknown field %s for %s", ErrValidation, name, child.Code)
		}
	}
	return nil
}

func flattenPayloadInput(input RecordInput) RecordInput {
	nested, ok := input.Data["Payload"].(map[string]any)
	if !ok {
		return input
	}
	flattened := copyData(nested)
	for name, value := range input.Data {
		if name == "Payload" {
			continue
		}
		flattened[name] = value
	}
	input.Data = flattened
	return input
}
