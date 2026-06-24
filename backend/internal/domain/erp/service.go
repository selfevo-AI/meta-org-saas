package erp

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
	CreateActionExecution(ctx context.Context, execution ActionExecution) (*ActionExecution, error)
	FindActionExecutionByIdempotencyKey(ctx context.Context, key string) (*ActionExecution, error)
	CompleteActionExecution(ctx context.Context, id uuid.UUID, status string, payload map[string]any, failure *ActionFailure) (*ActionExecution, error)
	CreateActionGeneratedRecord(ctx context.Context, record ActionGeneratedRecord) error
	ListActionGeneratedRecords(ctx context.Context, actionID uuid.UUID) ([]ActionGeneratedRecord, error)
}

type TransactionalRepository interface {
	RunInTx(context.Context, func(Repository) error) error
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
	table, err := s.table(tableCode)
	if err != nil {
		return nil, err
	}
	return s.repo.ListRecords(ctx, table, limit)
}

func (s *Service) CreateRecord(ctx context.Context, tableCode string, input RecordInput) (*Record, error) {
	table, err := s.table(tableCode)
	if err != nil {
		return nil, err
	}
	if err := validateRecordInput(table, input); err != nil {
		return nil, err
	}
	return s.repo.CreateRecord(ctx, table, input)
}

func (s *Service) GetRecord(ctx context.Context, tableCode, key string) (*Record, error) {
	table, err := s.table(tableCode)
	if err != nil {
		return nil, err
	}
	return s.repo.GetRecord(ctx, table, key)
}

func (s *Service) UpdateRecord(ctx context.Context, tableCode, key string, input RecordInput) (*Record, error) {
	table, err := s.table(tableCode)
	if err != nil {
		return nil, err
	}
	if err := validateRecordInput(table, input); err != nil {
		return nil, err
	}
	return s.repo.UpdateRecord(ctx, table, key, input)
}

func (s *Service) DeleteRecord(ctx context.Context, tableCode, key string) error {
	table, err := s.table(tableCode)
	if err != nil {
		return err
	}
	return s.repo.DeleteRecord(ctx, table, key)
}

func (s *Service) ListChildRecords(ctx context.Context, tableCode, parentKey, childCode string, limit int) ([]Record, error) {
	parent, child, err := s.child(tableCode, childCode)
	if err != nil {
		return nil, err
	}
	return s.repo.ListChildRecords(ctx, parent, child, parentKey, limit)
}

func (s *Service) CreateChildRecord(ctx context.Context, tableCode, parentKey, childCode string, input RecordInput) (*Record, error) {
	parent, child, err := s.child(tableCode, childCode)
	if err != nil {
		return nil, err
	}
	if err := validateChildRecordInput(child, input); err != nil {
		return nil, err
	}
	return s.repo.CreateChildRecord(ctx, parent, child, parentKey, input)
}

func (s *Service) RunAction(ctx context.Context, tableCode string, key string, action string, input ActionInput) (*ActionResult, error) {
	if input.Data == nil {
		input.Data = map[string]any{}
	}
	if _, err := s.table(tableCode); err != nil {
		return nil, err
	}
	def, ok := s.actions.Lookup(tableCode, action)
	if !ok {
		return nil, fmt.Errorf("%w: unknown action %s for %s", ErrValidation, action, tableCode)
	}
	idempotencyKey := s.effectiveIdempotencyKey(tableCode, key, action, input)
	if existing, err := s.repo.FindActionExecutionByIdempotencyKey(ctx, idempotencyKey); err == nil {
		if existing.Status == ActionExecutionCompleted {
			return s.idempotentReplayResult(ctx, existing)
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
		Payload:            map[string]any{"data": input.Data},
	})
	if err != nil {
		return nil, err
	}
	result, actionErr := s.runBusinessAction(ctx, tableCode, key, action, input)
	if actionErr != nil && errors.Is(actionErr, errUnsupportedERPAction) {
		result = &ActionResult{
			TableCode: tableCode,
			Key:       key,
			Action:    def.Action,
			Status:    "accepted",
			Effects: map[string]any{
				"definition": def.Label,
			},
		}
		actionErr = nil
	}
	if actionErr != nil {
		failure := actionFailureFromError(actionErr)
		_, _ = s.repo.CompleteActionExecution(ctx, execution.ID, ActionExecutionFailed, map[string]any{"error": actionErr.Error()}, failure)
		return nil, actionErr
	}
	if result == nil {
		result = &ActionResult{TableCode: tableCode, Key: key, Action: action, Status: "accepted"}
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
	if _, err := s.repo.CompleteActionExecution(ctx, execution.ID, ActionExecutionCompleted, actionResultPayload(result), nil); err != nil {
		return nil, err
	}
	return result, nil
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
	case "MINV:post":
		return "JE-" + key
	case "MRCT:allocate":
		return stringValue(input.Data, "TargetTable", "MINV") + "|" + stringValue(input.Data, "TargetKey", "") + "|" + fmt.Sprint(numericValue(input.Data, "Amount"))
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
	for name := range input.Data {
		if _, ok := table.Field(name); !ok {
			return fmt.Errorf("%w: unknown field %s for %s", ErrValidation, name, table.Code)
		}
	}
	return nil
}

func validateChildRecordInput(child ChildTableDefinition, input RecordInput) error {
	for name := range input.Data {
		if _, ok := child.Field(name); !ok {
			return fmt.Errorf("%w: unknown field %s for %s", ErrValidation, name, child.Code)
		}
	}
	return nil
}
