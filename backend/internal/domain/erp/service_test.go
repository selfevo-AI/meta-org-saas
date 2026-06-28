package erp

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeRepository struct {
	records                 map[string][]Record
	childRecords            map[string][]Record
	created                 Record
	createdChild            Record
	updatedChild            Record
	deletedChildTable       string
	deletedChildParentKey   string
	deletedChildKey         string
	listTableCode           string
	listExecutionsTableCode string
	listExecutionsRecordKey string
	listExecutionsLimit     int
	executions              map[uuid.UUID]ActionExecution
	executionsByKey         map[string]uuid.UUID
	generatedRecords        map[uuid.UUID][]ActionGeneratedRecord
}

func (r *fakeRepository) ListRecords(ctx context.Context, table TableDefinition, limit int) ([]Record, error) {
	r.listTableCode = table.Code
	return r.records[table.Code], nil
}

func (r *fakeRepository) CreateRecord(ctx context.Context, table TableDefinition, input RecordInput) (*Record, error) {
	r.created = Record{TableCode: table.Code, Key: input.Key, Data: input.Data}
	return &r.created, nil
}

func (r *fakeRepository) GetRecord(ctx context.Context, table TableDefinition, key string) (*Record, error) {
	for _, record := range r.records[table.Code] {
		if record.Key == key {
			return &record, nil
		}
	}
	return nil, ErrNotFound
}

func (r *fakeRepository) UpdateRecord(ctx context.Context, table TableDefinition, key string, input RecordInput) (*Record, error) {
	record := Record{TableCode: table.Code, Key: key, Data: input.Data}
	return &record, nil
}

func (r *fakeRepository) DeleteRecord(ctx context.Context, table TableDefinition, key string) error {
	return nil
}

func (r *fakeRepository) ListChildRecords(ctx context.Context, parent TableDefinition, child ChildTableDefinition, parentKey string, limit int) ([]Record, error) {
	return r.childRecords[child.Code], nil
}

func (r *fakeRepository) CreateChildRecord(ctx context.Context, parent TableDefinition, child ChildTableDefinition, parentKey string, input RecordInput) (*Record, error) {
	r.createdChild = Record{TableCode: child.Code, ParentTableCode: parent.Code, ParentKey: parentKey, Key: input.Key, Data: input.Data}
	return &r.createdChild, nil
}

func (r *fakeRepository) UpdateChildRecord(ctx context.Context, parent TableDefinition, child ChildTableDefinition, parentKey string, lineKey string, input RecordInput) (*Record, error) {
	r.updatedChild = Record{TableCode: child.Code, ParentTableCode: parent.Code, ParentKey: parentKey, Key: lineKey, Data: input.Data}
	return &r.updatedChild, nil
}

func (r *fakeRepository) DeleteChildRecord(ctx context.Context, parent TableDefinition, child ChildTableDefinition, parentKey string, lineKey string) error {
	r.deletedChildTable = child.Code
	r.deletedChildParentKey = parentKey
	r.deletedChildKey = lineKey
	return nil
}

func (r *fakeRepository) CreateActionExecution(_ context.Context, execution ActionExecution) (*ActionExecution, error) {
	r.ensureExecutionLedger()
	if execution.ID == uuid.Nil {
		execution.ID = uuid.New()
	}
	if execution.Status == "" {
		execution.Status = ActionExecutionRunning
	}
	if execution.Payload == nil {
		execution.Payload = map[string]any{}
	}
	r.executions[execution.ID] = execution
	if execution.IdempotencyKey != "" {
		r.executionsByKey[execution.IdempotencyKey] = execution.ID
	}
	return &execution, nil
}

func (r *fakeRepository) FindActionExecutionByIdempotencyKey(_ context.Context, key string) (*ActionExecution, error) {
	r.ensureExecutionLedger()
	id, ok := r.executionsByKey[key]
	if !ok {
		return nil, ErrNotFound
	}
	execution := r.executions[id]
	return &execution, nil
}

func (r *fakeRepository) CompleteActionExecution(_ context.Context, id uuid.UUID, status string, payload map[string]any, failure *ActionFailure) (*ActionExecution, error) {
	r.ensureExecutionLedger()
	execution, ok := r.executions[id]
	if !ok {
		return nil, ErrNotFound
	}
	execution.Status = status
	execution.Payload = payload
	if failure != nil {
		execution.FailureCode = failure.Code
		execution.FailureMessage = failure.Message
	}
	now := time.Now()
	execution.CompletedAt = &now
	r.executions[id] = execution
	return &execution, nil
}

func (r *fakeRepository) CreateActionGeneratedRecord(_ context.Context, record ActionGeneratedRecord) error {
	r.ensureExecutionLedger()
	r.generatedRecords[record.ActionID] = append(r.generatedRecords[record.ActionID], record)
	return nil
}

func (r *fakeRepository) ListActionGeneratedRecords(_ context.Context, actionID uuid.UUID) ([]ActionGeneratedRecord, error) {
	r.ensureExecutionLedger()
	return append([]ActionGeneratedRecord{}, r.generatedRecords[actionID]...), nil
}

func (r *fakeRepository) ListActionExecutions(_ context.Context, tableCode string, recordKey string, limit int) ([]ActionExecution, error) {
	r.ensureExecutionLedger()
	r.listExecutionsTableCode = tableCode
	r.listExecutionsRecordKey = recordKey
	r.listExecutionsLimit = limit
	items := make([]ActionExecution, 0, len(r.executions))
	for _, execution := range r.executions {
		if execution.TableCode == tableCode && execution.RecordKey == recordKey {
			items = append(items, execution)
		}
	}
	return items, nil
}

func (r *fakeRepository) ensureExecutionLedger() {
	if r.executions == nil {
		r.executions = map[uuid.UUID]ActionExecution{}
	}
	if r.executionsByKey == nil {
		r.executionsByKey = map[string]uuid.UUID{}
	}
	if r.generatedRecords == nil {
		r.generatedRecords = map[uuid.UUID][]ActionGeneratedRecord{}
	}
}

func TestCatalogIncludesDocumentTablesAndChildRows(t *testing.T) {
	catalog := DefaultCatalog()

	minv, ok := catalog.Table("MINV")
	if !ok {
		t.Fatal("DefaultCatalog().Table(\"MINV\") missing")
	}
	if minv.Name != "A/R Invoice" {
		t.Fatalf("MINV name = %q, want A/R Invoice", minv.Name)
	}
	if minv.PrimaryKey != "DocEntry" {
		t.Fatalf("MINV primary key = %q, want DocEntry", minv.PrimaryKey)
	}
	child, ok := minv.Child("INV1")
	if !ok {
		t.Fatal("MINV child INV1 missing")
	}
	if child.ParentKey != "DocEntry" || child.LineKey != "LineNum" {
		t.Fatalf("INV1 keys = %#v, want DocEntry/LineNum", child)
	}
}

func TestServiceListsActionExecutionsWithGeneratedRecords(t *testing.T) {
	repo := &fakeRepository{}
	repo.ensureExecutionLedger()
	executionID := uuid.New()
	repo.executions[executionID] = ActionExecution{
		ID:             executionID,
		TableCode:      "MREQ",
		RecordKey:      "REQ-1001",
		Action:         "convert-to-project",
		Status:         ActionExecutionCompleted,
		IdempotencyKey: "erp:MREQ:REQ-1001:convert-to-project:PRJ-1001",
		Payload:        map[string]any{"status": "converted"},
	}
	repo.generatedRecords[executionID] = []ActionGeneratedRecord{
		{
			ActionID:           executionID,
			LineNum:            1,
			GeneratedTableCode: "MPRJ",
			GeneratedKey:       "PRJ-1001",
			RelationType:       "created",
			Payload:            map[string]any{"PrjCode": "PRJ-1001"},
		},
	}

	items, err := NewService(repo, DefaultCatalog()).ListActionExecutions(context.Background(), "MREQ", "REQ-1001", 3)
	if err != nil {
		t.Fatalf("ListActionExecutions returned error: %v", err)
	}
	if repo.listExecutionsTableCode != "MREQ" || repo.listExecutionsRecordKey != "REQ-1001" || repo.listExecutionsLimit != 3 {
		t.Fatalf("repo list args = %s/%s/%d", repo.listExecutionsTableCode, repo.listExecutionsRecordKey, repo.listExecutionsLimit)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if len(items[0].GeneratedRecords) != 1 || items[0].GeneratedRecords[0].GeneratedTableCode != "MPRJ" {
		t.Fatalf("generated records = %#v, want MPRJ generated record", items[0].GeneratedRecords)
	}
}

func TestServiceRejectsActionExecutionTimelineForUnknownTable(t *testing.T) {
	_, err := NewService(&fakeRepository{}, DefaultCatalog()).ListActionExecutions(context.Background(), "NOPE", "1", 50)
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("ListActionExecutions error = %v, want validation", err)
	}
}

func TestCreateRecordRejectsUnknownFields(t *testing.T) {
	service := NewService(&fakeRepository{}, DefaultCatalog())

	_, err := service.CreateRecord(context.Background(), "MCRD", RecordInput{
		Key:  "C0001",
		Data: map[string]any{"CardType": "C", "NotAField": "bad"},
	})

	if !errors.Is(err, ErrValidation) {
		t.Fatalf("CreateRecord() error = %v, want ErrValidation", err)
	}
}

func TestCreateChildRecordUsesParentAndChildDefinitions(t *testing.T) {
	repo := &fakeRepository{}
	service := NewService(repo, DefaultCatalog())

	record, err := service.CreateChildRecord(context.Background(), "MINV", "1001", "INV1", RecordInput{
		Key: "1",
		Data: map[string]any{
			"LineNum":    float64(1),
			"Payload":    map[string]any{"ItemCode": "A-100"},
			"LineStatus": "O",
		},
	})

	if err != nil {
		t.Fatalf("CreateChildRecord() error = %v", err)
	}
	if record.TableCode != "INV1" || record.ParentTableCode != "MINV" || record.ParentKey != "1001" {
		t.Fatalf("child record = %#v, want INV1 under MINV/1001", record)
	}
}
