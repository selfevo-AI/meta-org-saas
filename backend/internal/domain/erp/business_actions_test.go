package erp

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
)

type businessFakeRepository struct {
	records          map[string]map[string]Record
	children         map[string][]Record
	executions       map[uuid.UUID]ActionExecution
	executionsByKey  map[string]uuid.UUID
	generatedRecords map[uuid.UUID][]ActionGeneratedRecord
}

func newBusinessFakeRepository() *businessFakeRepository {
	return &businessFakeRepository{
		records:          map[string]map[string]Record{},
		children:         map[string][]Record{},
		executions:       map[uuid.UUID]ActionExecution{},
		executionsByKey:  map[string]uuid.UUID{},
		generatedRecords: map[uuid.UUID][]ActionGeneratedRecord{},
	}
}

func (r *businessFakeRepository) seed(tableCode string, key string, data map[string]any) {
	if r.records[tableCode] == nil {
		r.records[tableCode] = map[string]Record{}
	}
	r.records[tableCode][key] = Record{TableCode: tableCode, Key: key, Data: copyData(data)}
}

func (r *businessFakeRepository) seedChild(parentCode string, parentKey string, childCode string, data map[string]any) {
	key := fmt.Sprint(data["LineNum"])
	if key == "" || key == "<nil>" {
		key = fmt.Sprint(len(r.children[childBucket(parentCode, parentKey, childCode)]) + 1)
	}
	record := Record{TableCode: childCode, ParentTableCode: parentCode, ParentKey: parentKey, Key: key, Data: copyData(data)}
	bucket := childBucket(parentCode, parentKey, childCode)
	r.children[bucket] = append(r.children[bucket], record)
}

func (r *businessFakeRepository) ListRecords(ctx context.Context, table TableDefinition, limit int) ([]Record, error) {
	items := []Record{}
	for _, record := range r.records[table.Code] {
		items = append(items, record)
	}
	return items, nil
}

func (r *businessFakeRepository) CreateRecord(ctx context.Context, table TableDefinition, input RecordInput) (*Record, error) {
	key := input.Key
	if key == "" {
		key = fmt.Sprint(input.Data[table.PrimaryKey])
	}
	data := copyData(input.Data)
	data[table.PrimaryKey] = key
	r.seed(table.Code, key, data)
	record := r.records[table.Code][key]
	return &record, nil
}

func (r *businessFakeRepository) GetRecord(ctx context.Context, table TableDefinition, key string) (*Record, error) {
	record, ok := r.records[table.Code][key]
	if !ok {
		return nil, ErrNotFound
	}
	return &record, nil
}

func (r *businessFakeRepository) UpdateRecord(ctx context.Context, table TableDefinition, key string, input RecordInput) (*Record, error) {
	record, ok := r.records[table.Code][key]
	if !ok {
		return nil, ErrNotFound
	}
	if record.Data == nil {
		record.Data = map[string]any{}
	}
	for k, v := range input.Data {
		record.Data[k] = v
	}
	r.seed(table.Code, key, record.Data)
	updated := r.records[table.Code][key]
	return &updated, nil
}

func (r *businessFakeRepository) DeleteRecord(ctx context.Context, table TableDefinition, key string) error {
	delete(r.records[table.Code], key)
	return nil
}

func (r *businessFakeRepository) ListChildRecords(ctx context.Context, parent TableDefinition, child ChildTableDefinition, parentKey string, limit int) ([]Record, error) {
	return append([]Record{}, r.children[childBucket(parent.Code, parentKey, child.Code)]...), nil
}

func (r *businessFakeRepository) CreateChildRecord(ctx context.Context, parent TableDefinition, child ChildTableDefinition, parentKey string, input RecordInput) (*Record, error) {
	data := copyData(input.Data)
	data[child.ParentKey] = parentKey
	if input.Key != "" {
		data[child.LineKey] = input.Key
	}
	r.seedChild(parent.Code, parentKey, child.Code, data)
	items := r.children[childBucket(parent.Code, parentKey, child.Code)]
	record := items[len(items)-1]
	return &record, nil
}

func (r *businessFakeRepository) CreateActionExecution(_ context.Context, execution ActionExecution) (*ActionExecution, error) {
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

func (r *businessFakeRepository) FindActionExecutionByIdempotencyKey(_ context.Context, key string) (*ActionExecution, error) {
	id, ok := r.executionsByKey[key]
	if !ok {
		return nil, ErrNotFound
	}
	execution := r.executions[id]
	return &execution, nil
}

func (r *businessFakeRepository) CompleteActionExecution(_ context.Context, id uuid.UUID, status string, payload map[string]any, failure *ActionFailure) (*ActionExecution, error) {
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

func (r *businessFakeRepository) CreateActionGeneratedRecord(_ context.Context, record ActionGeneratedRecord) error {
	r.generatedRecords[record.ActionID] = append(r.generatedRecords[record.ActionID], record)
	return nil
}

func (r *businessFakeRepository) ListActionGeneratedRecords(_ context.Context, actionID uuid.UUID) ([]ActionGeneratedRecord, error) {
	return append([]ActionGeneratedRecord{}, r.generatedRecords[actionID]...), nil
}

func (r *businessFakeRepository) ListActionExecutions(_ context.Context, tableCode string, recordKey string, limit int) ([]ActionExecution, error) {
	items := []ActionExecution{}
	for _, execution := range r.executions {
		if execution.TableCode == tableCode && execution.RecordKey == recordKey {
			items = append(items, execution)
		}
	}
	return items, nil
}

func (r *businessFakeRepository) RunInTx(_ context.Context, fn func(Repository) error) error {
	records := cloneRecords(r.records)
	children := cloneChildren(r.children)
	executions := cloneExecutions(r.executions)
	executionsByKey := cloneExecutionKeys(r.executionsByKey)
	generatedRecords := cloneGeneratedRecords(r.generatedRecords)
	if err := fn(r); err != nil {
		r.records = records
		r.children = children
		r.executions = executions
		r.executionsByKey = executionsByKey
		r.generatedRecords = generatedRecords
		return err
	}
	return nil
}

func cloneRecords(input map[string]map[string]Record) map[string]map[string]Record {
	output := map[string]map[string]Record{}
	for tableCode, rows := range input {
		output[tableCode] = map[string]Record{}
		for key, record := range rows {
			record.Data = copyData(record.Data)
			output[tableCode][key] = record
		}
	}
	return output
}

func cloneChildren(input map[string][]Record) map[string][]Record {
	output := map[string][]Record{}
	for bucket, rows := range input {
		output[bucket] = append([]Record{}, rows...)
		for i := range output[bucket] {
			output[bucket][i].Data = copyData(output[bucket][i].Data)
		}
	}
	return output
}

func cloneExecutions(input map[uuid.UUID]ActionExecution) map[uuid.UUID]ActionExecution {
	output := map[uuid.UUID]ActionExecution{}
	for id, execution := range input {
		output[id] = execution
	}
	return output
}

func cloneExecutionKeys(input map[string]uuid.UUID) map[string]uuid.UUID {
	output := map[string]uuid.UUID{}
	for key, id := range input {
		output[key] = id
	}
	return output
}

func cloneGeneratedRecords(input map[uuid.UUID][]ActionGeneratedRecord) map[uuid.UUID][]ActionGeneratedRecord {
	output := map[uuid.UUID][]ActionGeneratedRecord{}
	for id, rows := range input {
		output[id] = append([]ActionGeneratedRecord{}, rows...)
	}
	return output
}

func (r *businessFakeRepository) balance(itemCode string, whsCode string) float64 {
	record, ok := r.records["MITW"][itemCode+"|"+whsCode]
	if !ok {
		return 0
	}
	value, _ := numberFromAny(record.Data["OnHand"])
	return value
}

func (r *businessFakeRepository) childCount(parentCode string, parentKey string, childCode string) int {
	return len(r.children[childBucket(parentCode, parentKey, childCode)])
}

func assertGeneratedTable(t *testing.T, result *ActionResult, tableCode string) {
	t.Helper()
	for _, record := range result.GeneratedRecords {
		if record.TableCode == tableCode {
			return
		}
	}
	t.Fatalf("generated records = %#v, want table %s", result.GeneratedRecords, tableCode)
}

func childBucket(parentCode string, parentKey string, childCode string) string {
	return parentCode + ":" + parentKey + ":" + childCode
}

func numberFromAny(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	default:
		return 0, false
	}
}

func TestApproveRequirementUpdatesStatus(t *testing.T) {
	repo := newBusinessFakeRepository()
	repo.seed("MREQ", "REQ-1", map[string]any{"ReqCode": "REQ-1", "Status": "analyzed"})
	service := NewService(repo, DefaultCatalog())

	result, err := service.RunAction(context.Background(), "MREQ", "REQ-1", "approve", ActionInput{Data: map[string]any{"approver": "u1"}})
	if err != nil {
		t.Fatalf("approve returned error: %v", err)
	}
	if result.Record == nil {
		t.Fatal("approve returned nil record, want updated MREQ record")
	}
	if result.Record.Data["Status"] != "approved" {
		t.Fatalf("status = %v, want approved", result.Record.Data["Status"])
	}
}

func TestActionResultIncludesExecutionContract(t *testing.T) {
	repo := newBusinessFakeRepository()
	actorID := uuid.New()
	repo.seed("MREQ", "REQ-1", map[string]any{"ReqCode": "REQ-1", "Status": "analyzed"})
	service := NewService(repo, DefaultCatalog())

	result, err := service.RunAction(context.Background(), "MREQ", "REQ-1", "approve", ActionInput{
		ActorID:        &actorID,
		ActorType:      "internal_human",
		IdempotencyKey: "approve-REQ-1",
		Source:         "tenant_api",
		Data:           map[string]any{"approver": "u1"},
	})
	if err != nil {
		t.Fatalf("approve returned error: %v", err)
	}
	if result.ExecutionID == uuid.Nil {
		t.Fatalf("execution id = %s, want non-nil", result.ExecutionID)
	}
	if result.IdempotencyKey == "" {
		t.Fatalf("idempotency key is empty")
	}
	if result.Provenance["source"] != "tenant_api" {
		t.Fatalf("provenance = %#v, want source tenant_api", result.Provenance)
	}
	if len(result.PreconditionsChecked) == 0 {
		t.Fatalf("preconditions = %#v, want at least one check", result.PreconditionsChecked)
	}
}

func TestActionExecutionLedgerRecordsCompletedAction(t *testing.T) {
	repo := newBusinessFakeRepository()
	repo.seed("MREQ", "REQ-1", map[string]any{"ReqCode": "REQ-1", "Status": "analyzed"})
	service := NewService(repo, DefaultCatalog())

	result, err := service.RunAction(context.Background(), "MREQ", "REQ-1", "approve", ActionInput{
		IdempotencyKey: "approve-ledger",
		Source:         "tenant_api",
		Data:           map[string]any{"approver": "u1"},
	})
	if err != nil {
		t.Fatalf("approve returned error: %v", err)
	}
	execution, ok := repo.executions[result.ExecutionID]
	if !ok {
		t.Fatalf("missing execution %s in fake ledger", result.ExecutionID)
	}
	if execution.Status != ActionExecutionCompleted {
		t.Fatalf("execution status = %q, want completed", execution.Status)
	}
	if execution.TableCode != "MREQ" || execution.RecordKey != "REQ-1" || execution.Action != "approve" {
		t.Fatalf("execution = %#v, want MREQ/REQ-1/approve", execution)
	}
}

func TestActionExecutionLedgerRecordsValidationFailure(t *testing.T) {
	repo := newBusinessFakeRepository()
	repo.seed("MPOR", "PO-1", map[string]any{"DocEntry": "PO-1", "DocStatus": "O", "WddStatus": "W"})
	service := NewService(repo, DefaultCatalog())

	result, err := service.RunAction(context.Background(), "MPOR", "PO-1", "approve", ActionInput{IdempotencyKey: "bad-approve"})
	if err == nil {
		t.Fatalf("approve returned nil error and result %#v, want validation error", result)
	}
	var failed ActionExecution
	for _, execution := range repo.executions {
		if execution.IdempotencyKey == "erp:MPOR:PO-1:approve:bad-approve" {
			failed = execution
		}
	}
	if failed.ID == uuid.Nil {
		t.Fatalf("missing failed execution")
	}
	if failed.Status != ActionExecutionFailed || failed.FailureCode != "validation_failed" {
		t.Fatalf("failed execution = %#v, want validation_failed", failed)
	}
}

func TestRequirementApproveRequiresAnalyzedStatus(t *testing.T) {
	repo := newBusinessFakeRepository()
	repo.seed("MREQ", "REQ-1", map[string]any{"ReqCode": "REQ-1", "Status": "draft"})
	service := NewService(repo, DefaultCatalog())

	result, err := service.RunAction(context.Background(), "MREQ", "REQ-1", "approve", ActionInput{IdempotencyKey: "approve-draft"})
	if err == nil {
		t.Fatalf("approve returned nil error and result %#v, want validation error", result)
	}
	if repo.records["MREQ"]["REQ-1"].Data["Status"] != "draft" {
		t.Fatalf("status changed to %v, want draft", repo.records["MREQ"]["REQ-1"].Data["Status"])
	}
}

func TestSalesOrderApproveRequiresConfirmedOrder(t *testing.T) {
	repo := newBusinessFakeRepository()
	repo.seed("MRDR", "SO-1", map[string]any{"DocEntry": "SO-1", "DocStatus": "O", "Confirmed": "N", "WddStatus": "W"})
	service := NewService(repo, DefaultCatalog())

	result, err := service.RunAction(context.Background(), "MRDR", "SO-1", "approve", ActionInput{IdempotencyKey: "approve-unconfirmed"})
	if err == nil {
		t.Fatalf("approve returned nil error and result %#v, want validation error", result)
	}
	if repo.records["MRDR"]["SO-1"].Data["WddStatus"] != "W" {
		t.Fatalf("WddStatus changed to %v, want W", repo.records["MRDR"]["SO-1"].Data["WddStatus"])
	}
}

func TestCloseProjectFeedbackRequiresCostRefresh(t *testing.T) {
	repo := newBusinessFakeRepository()
	repo.seed("MPRJ", "PRJ-1", map[string]any{"PrjCode": "PRJ-1", "Active": "Y"})
	service := NewService(repo, DefaultCatalog())

	result, err := service.RunAction(context.Background(), "MPRJ", "PRJ-1", "close-feedback", ActionInput{IdempotencyKey: "close-before-cost"})
	if err == nil {
		t.Fatalf("close-feedback returned nil error and result %#v, want validation error", result)
	}
	if repo.records["MFDB"] != nil {
		t.Fatalf("feedback record generated before cost refresh: %#v", repo.records["MFDB"])
	}
}

func TestDeliveryPostRollsBackWhenInventoryFails(t *testing.T) {
	repo := newBusinessFakeRepository()
	repo.seed("MDLN", "DLV-1", map[string]any{"DocEntry": "DLV-1", "DocStatus": "O", "WddStatus": "A", "CardCode": "C-1"})
	repo.seedChild("MDLN", "DLV-1", "DLN1", map[string]any{"LineNum": "1", "Payload": map[string]any{"ItemCode": "I-1", "WhsCode": "W-1", "Quantity": 2, "Price": 15}})
	repo.seed("MITW", "I-1|W-1", map[string]any{"ItemCode": "I-1|W-1", "OnHand": 1})
	service := NewService(repo, DefaultCatalog())

	result, err := service.RunAction(context.Background(), "MDLN", "DLV-1", "post", ActionInput{IdempotencyKey: "rollback-delivery"})
	if err == nil {
		t.Fatalf("delivery post returned nil error and result %#v, want insufficient inventory error", result)
	}
	if repo.records["MIGE"] != nil || repo.records["MINV"] != nil {
		t.Fatalf("generated records were not rolled back: MIGE=%#v MINV=%#v", repo.records["MIGE"], repo.records["MINV"])
	}
	if repo.records["MDLN"]["DLV-1"].Data["Posted"] == "Y" {
		t.Fatalf("delivery marked posted after rollback")
	}
}

func TestGoodsReceiptPostIsIdempotent(t *testing.T) {
	repo := newBusinessFakeRepository()
	repo.seed("MPDN", "GR-1", map[string]any{"DocEntry": "GR-1", "DocStatus": "O", "WddStatus": "A", "CardCode": "S-1"})
	repo.seedChild("MPDN", "GR-1", "PDN1", map[string]any{"LineNum": "1", "Payload": map[string]any{"ItemCode": "I-1", "WhsCode": "W-1", "Quantity": 2, "Price": 10}})
	service := NewService(repo, DefaultCatalog())

	first, err := service.RunAction(context.Background(), "MPDN", "GR-1", "post", ActionInput{IdempotencyKey: "receipt-post"})
	if err != nil {
		t.Fatalf("first post error: %v", err)
	}
	second, err := service.RunAction(context.Background(), "MPDN", "GR-1", "post", ActionInput{IdempotencyKey: "receipt-post"})
	if err != nil {
		t.Fatalf("second post error: %v", err)
	}
	if second.Status != ActionExecutionIdempotentReplay {
		t.Fatalf("second status = %q, want idempotent_replay", second.Status)
	}
	if first.ExecutionID != second.ExecutionID {
		t.Fatalf("execution ids = %s and %s, want replay of first execution", first.ExecutionID, second.ExecutionID)
	}
	if len(repo.generatedRecords[first.ExecutionID]) != 2 {
		t.Fatalf("generated ledger rows = %d, want 2", len(repo.generatedRecords[first.ExecutionID]))
	}
	if len(second.GeneratedRecords) != 2 {
		t.Fatalf("replay generated records = %d, want 2", len(second.GeneratedRecords))
	}
	if repo.childCount("MIGN", "IGN-GR-1", "IGN1") != 1 {
		t.Fatalf("MIGN child rows = %d, want 1 after replay", repo.childCount("MIGN", "IGN-GR-1", "IGN1"))
	}
	if repo.balance("I-1", "W-1") != 2 {
		t.Fatalf("balance = %v, want 2 after replay", repo.balance("I-1", "W-1"))
	}
}

func TestDeliveryPostAddsGeneratedRecordProvenance(t *testing.T) {
	repo := newBusinessFakeRepository()
	toolExecutionID := uuid.New()
	sessionID := uuid.New()
	repo.seed("MDLN", "DLV-1", map[string]any{"DocEntry": "DLV-1", "DocStatus": "O", "WddStatus": "A", "CardCode": "C-1"})
	repo.seedChild("MDLN", "DLV-1", "DLN1", map[string]any{"LineNum": "1", "Payload": map[string]any{"ItemCode": "I-1", "WhsCode": "W-1", "Quantity": 2, "Price": 15}})
	repo.seed("MITW", "I-1|W-1", map[string]any{"ItemCode": "I-1|W-1", "OnHand": 5})
	service := NewService(repo, DefaultCatalog())

	result, err := service.RunAction(context.Background(), "MDLN", "DLV-1", "post", ActionInput{
		IdempotencyKey:     "delivery-provenance",
		Source:             "toolruntime",
		ToolExecutionID:    &toolExecutionID,
		AssistantSessionID: &sessionID,
	})
	if err != nil {
		t.Fatalf("delivery post error: %v", err)
	}
	if len(result.GeneratedRecords) == 0 {
		t.Fatalf("generated records empty")
	}
	for _, record := range result.GeneratedRecords {
		provenance, ok := record.Data["provenance"].(map[string]any)
		if !ok {
			t.Fatalf("record %#v missing provenance map", record)
		}
		if provenance["source_table_code"] != "MDLN" || provenance["source_key"] != "DLV-1" || provenance["source_action"] != "post" {
			t.Fatalf("provenance = %#v, want MDLN/DLV-1/post", provenance)
		}
		if provenance["tool_execution_id"] != toolExecutionID.String() || provenance["assistant_session_id"] != sessionID.String() {
			t.Fatalf("provenance = %#v, want tool/session correlation", provenance)
		}
	}
}

func TestConvertRequirementAddsGeneratedProjectProvenance(t *testing.T) {
	repo := newBusinessFakeRepository()
	repo.seed("MREQ", "REQ-1", map[string]any{"ReqCode": "REQ-1", "Name": "Portal", "Status": "approved"})
	service := NewService(repo, DefaultCatalog())

	result, err := service.RunAction(context.Background(), "MREQ", "REQ-1", "convert-to-project", ActionInput{
		IdempotencyKey: "convert-provenance",
		Data:           map[string]any{"PrjCode": "PRJ-1"},
	})
	if err != nil {
		t.Fatalf("convert returned error: %v", err)
	}
	if len(result.GeneratedRecords) != 1 {
		t.Fatalf("generated records = %d, want 1", len(result.GeneratedRecords))
	}
	provenance, ok := result.GeneratedRecords[0].Data["provenance"].(map[string]any)
	if !ok {
		t.Fatalf("generated project missing provenance: %#v", result.GeneratedRecords[0])
	}
	if provenance["source_table_code"] != "MREQ" || provenance["source_key"] != "REQ-1" || provenance["source_action"] != "convert-to-project" {
		t.Fatalf("provenance = %#v, want MREQ/REQ-1/convert-to-project", provenance)
	}
}

func TestConvertRequirementCreatesProject(t *testing.T) {
	repo := newBusinessFakeRepository()
	repo.seed("MREQ", "REQ-1", map[string]any{"ReqCode": "REQ-1", "Name": "Portal", "Status": "approved"})
	service := NewService(repo, DefaultCatalog())

	result, err := service.RunAction(context.Background(), "MREQ", "REQ-1", "convert-to-project", ActionInput{Data: map[string]any{"PrjCode": "PRJ-1"}})
	if err != nil {
		t.Fatalf("convert returned error: %v", err)
	}
	assertGeneratedTable(t, result, "MPRJ")
}

func TestConvertRequirementRejectsUnapprovedRequirement(t *testing.T) {
	repo := newBusinessFakeRepository()
	repo.seed("MREQ", "REQ-1", map[string]any{"ReqCode": "REQ-1", "Name": "Portal", "Status": "draft"})
	service := NewService(repo, DefaultCatalog())

	result, err := service.RunAction(context.Background(), "MREQ", "REQ-1", "convert-to-project", ActionInput{Data: map[string]any{"PrjCode": "PRJ-1"}})
	if err == nil {
		t.Fatalf("convert returned nil error and result %#v, want validation error", result)
	}
	if result != nil {
		t.Fatalf("convert result = %#v, want nil on validation error", result)
	}
}

func TestPurchaseOrderSubmitAndApprove(t *testing.T) {
	repo := newBusinessFakeRepository()
	repo.seed("MPOR", "PO-1", map[string]any{"DocEntry": "PO-1", "DocStatus": "O", "WddStatus": "W"})
	service := NewService(repo, DefaultCatalog())

	submitted, err := service.RunAction(context.Background(), "MPOR", "PO-1", "submit", ActionInput{})
	if err != nil {
		t.Fatalf("submit error: %v", err)
	}
	if submitted.Record == nil {
		t.Fatal("submit returned nil record, want updated MPOR record")
	}
	if submitted.Record.Data["DocStatus"] != "S" {
		t.Fatalf("DocStatus = %v, want S", submitted.Record.Data["DocStatus"])
	}
	approved, err := service.RunAction(context.Background(), "MPOR", "PO-1", "approve", ActionInput{})
	if err != nil {
		t.Fatalf("approve error: %v", err)
	}
	if approved.Record == nil {
		t.Fatal("approve returned nil record, want updated MPOR record")
	}
	if approved.Record.Data["WddStatus"] != "A" {
		t.Fatalf("WddStatus = %v, want A", approved.Record.Data["WddStatus"])
	}
}

func TestPurchaseOrderApproveRequiresSubmittedOrder(t *testing.T) {
	repo := newBusinessFakeRepository()
	repo.seed("MPOR", "PO-1", map[string]any{"DocEntry": "PO-1", "DocStatus": "O", "WddStatus": "W"})
	service := NewService(repo, DefaultCatalog())

	result, err := service.RunAction(context.Background(), "MPOR", "PO-1", "approve", ActionInput{})
	if err == nil {
		t.Fatalf("approve returned nil error and result %#v, want validation error", result)
	}
	if result != nil {
		t.Fatalf("approve result = %#v, want nil on validation error", result)
	}
	if repo.records["MPOR"]["PO-1"].Data["WddStatus"] != "W" {
		t.Fatalf("WddStatus changed to %v, want unchanged W", repo.records["MPOR"]["PO-1"].Data["WddStatus"])
	}
}

func TestGoodsReceiptPostGeneratesInventoryAndPayable(t *testing.T) {
	repo := newBusinessFakeRepository()
	repo.seed("MPDN", "GR-1", map[string]any{"DocEntry": "GR-1", "DocStatus": "O", "WddStatus": "A", "CardCode": "S-1"})
	repo.seedChild("MPDN", "GR-1", "PDN1", map[string]any{"LineNum": "1", "Payload": map[string]any{"ItemCode": "I-1", "WhsCode": "W-1", "Quantity": 2, "Price": 10}})
	service := NewService(repo, DefaultCatalog())

	result, err := service.RunAction(context.Background(), "MPDN", "GR-1", "post", ActionInput{})
	if err != nil {
		t.Fatalf("post error: %v", err)
	}
	assertGeneratedTable(t, result, "MIGN")
	assertGeneratedTable(t, result, "MPCH")
	if repo.childCount("MIGN", "IGN-GR-1", "IGN1") != 1 {
		t.Fatalf("MIGN child rows = %d, want 1", repo.childCount("MIGN", "IGN-GR-1", "IGN1"))
	}
	if repo.childCount("MPCH", "AP-GR-1", "PCH1") != 1 {
		t.Fatalf("MPCH child rows = %d, want 1", repo.childCount("MPCH", "AP-GR-1", "PCH1"))
	}
	if repo.balance("I-1", "W-1") != 2 {
		t.Fatalf("balance = %v, want 2", repo.balance("I-1", "W-1"))
	}
}

func TestGoodsReceiptPostRequiresApproval(t *testing.T) {
	repo := newBusinessFakeRepository()
	repo.seed("MPDN", "GR-1", map[string]any{"DocEntry": "GR-1", "DocStatus": "O", "WddStatus": "W", "CardCode": "S-1"})
	repo.seedChild("MPDN", "GR-1", "PDN1", map[string]any{"LineNum": "1", "Payload": map[string]any{"ItemCode": "I-1", "WhsCode": "W-1", "Quantity": 2, "Price": 10}})
	service := NewService(repo, DefaultCatalog())

	result, err := service.RunAction(context.Background(), "MPDN", "GR-1", "post", ActionInput{})
	if err == nil {
		t.Fatalf("post returned nil error and result %#v, want validation error", result)
	}
	if result != nil {
		t.Fatalf("post result = %#v, want nil on validation error", result)
	}
	if repo.records["MIGN"] != nil || repo.records["MPCH"] != nil {
		t.Fatalf("post generated records for unapproved receipt: %#v %#v", repo.records["MIGN"], repo.records["MPCH"])
	}
}

func TestSalesDeliveryPaymentLoop(t *testing.T) {
	repo := newBusinessFakeRepository()
	repo.seed("MRDR", "SO-1", map[string]any{"DocEntry": "SO-1", "DocStatus": "O", "Confirmed": "N", "WddStatus": "W"})
	repo.seed("MDLN", "DLV-1", map[string]any{"DocEntry": "DLV-1", "DocStatus": "O", "WddStatus": "A", "CardCode": "C-1"})
	repo.seedChild("MDLN", "DLV-1", "DLN1", map[string]any{"LineNum": "1", "Payload": map[string]any{"ItemCode": "I-1", "WhsCode": "W-1", "Quantity": 2, "Price": 15}})
	repo.seed("MITW", "I-1|W-1", map[string]any{"ItemCode": "I-1|W-1", "OnHand": 5})
	service := NewService(repo, DefaultCatalog())

	confirmed, err := service.RunAction(context.Background(), "MRDR", "SO-1", "confirm", ActionInput{})
	if err != nil {
		t.Fatalf("confirm error: %v", err)
	}
	if confirmed.Record == nil {
		t.Fatal("confirm returned nil record, want updated MRDR record")
	}
	if confirmed.Record.Data["Confirmed"] != "Y" {
		t.Fatalf("Confirmed = %v, want Y", confirmed.Record.Data["Confirmed"])
	}
	approved, err := service.RunAction(context.Background(), "MRDR", "SO-1", "approve", ActionInput{})
	if err != nil {
		t.Fatalf("approve error: %v", err)
	}
	if approved.Record == nil {
		t.Fatal("approve returned nil record, want updated MRDR record")
	}
	if approved.Record.Data["WddStatus"] != "A" {
		t.Fatalf("WddStatus = %v, want A", approved.Record.Data["WddStatus"])
	}
	delivery, err := service.RunAction(context.Background(), "MDLN", "DLV-1", "post", ActionInput{})
	if err != nil {
		t.Fatalf("delivery post error: %v", err)
	}
	assertGeneratedTable(t, delivery, "MIGE")
	assertGeneratedTable(t, delivery, "MINV")
	if repo.childCount("MIGE", "IGE-DLV-1", "IGE1") != 1 {
		t.Fatalf("MIGE child rows = %d, want 1", repo.childCount("MIGE", "IGE-DLV-1", "IGE1"))
	}
	if repo.childCount("MINV", "INV-DLV-1", "INV1") != 1 {
		t.Fatalf("MINV child rows = %d, want 1", repo.childCount("MINV", "INV-DLV-1", "INV1"))
	}
	if repo.balance("I-1", "W-1") != 3 {
		t.Fatalf("balance = %v, want 3", repo.balance("I-1", "W-1"))
	}
	repo.seed("MRCT", "PAY-1", map[string]any{"DocEntry": "PAY-1", "DocTotal": 30, "OpenBal": 30})
	payment, err := service.RunAction(context.Background(), "MRCT", "PAY-1", "allocate", ActionInput{Data: map[string]any{"TargetTable": "MINV", "TargetKey": "INV-DLV-1", "Amount": 20}})
	if err != nil {
		t.Fatalf("allocate error: %v", err)
	}
	if payment.Effects["allocated_amount"] != float64(20) {
		t.Fatalf("effects = %#v, want allocated_amount 20", payment.Effects)
	}
	if repo.records["MRCT"]["PAY-1"].Data["OpenBal"] != float64(10) {
		t.Fatalf("payment OpenBal = %v, want 10", repo.records["MRCT"]["PAY-1"].Data["OpenBal"])
	}
	if repo.records["MINV"]["INV-DLV-1"].Data["DocStatus"] != "O" {
		t.Fatalf("invoice DocStatus = %v, want O", repo.records["MINV"]["INV-DLV-1"].Data["DocStatus"])
	}
}

func TestInvoicePostCreatesJournalEntryWithJournalPrimaryKey(t *testing.T) {
	repo := newBusinessFakeRepository()
	repo.seed("MINV", "INV-1", map[string]any{"DocEntry": "INV-1", "DocTotal": 100, "PaidToDate": 0, "DocStatus": "O"})
	service := NewService(repo, DefaultCatalog())

	result, err := service.RunAction(context.Background(), "MINV", "INV-1", "post", ActionInput{})
	if err != nil {
		t.Fatalf("invoice post error: %v", err)
	}
	assertGeneratedTable(t, result, "MJDT")
	journal := repo.records["MJDT"]["JE-INV-1"]
	if journal.Data["TransId"] != "JE-INV-1" {
		t.Fatalf("journal TransId = %v, want JE-INV-1", journal.Data["TransId"])
	}
	if _, ok := journal.Data["DocEntry"]; ok {
		t.Fatalf("journal unexpectedly has DocEntry: %#v", journal.Data)
	}
}
