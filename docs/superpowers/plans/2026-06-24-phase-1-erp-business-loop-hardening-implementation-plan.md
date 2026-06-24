# Phase 1 ERP Business Loop Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Harden ERP table-code actions into an auditable, idempotent, transaction-safe business execution kernel for tenant runtime, Tool Runtime, Assistant Runtime, and future monitoring loops.

**Architecture:** The ERP service remains the only business-rule owner for table-code actions. Action execution is wrapped by a tenant ERP ledger, explicit precondition checks, transaction-aware repository methods, deterministic idempotency, and generated-record provenance. Tool Runtime continues to call `erp.action.execute`, but now forwards actor, source, idempotency, and assistant/tool correlation metadata into ERP `ActionInput`.

**Tech Stack:** Go, pgx, PostgreSQL JSONB, existing `erp`, `toolruntime`, `assistant`, smoke command, staged baseline migrations in `migrations/`.

---

## File Structure

- Modify `backend/internal/domain/erp/model.go`
  - Extend `ActionInput` and `ActionResult`.
  - Add `ActionExecution`, `ActionGeneratedRecord`, `ActionFailure`, `ActionPrecondition`, and action status constants.
- Modify `backend/internal/domain/erp/service.go`
  - Extend `Repository` with action ledger methods.
  - Add optional transaction interface.
  - Wrap `RunAction` with execution tracking, idempotency, and failure result metadata.
- Modify `backend/internal/domain/erp/business_actions.go`
  - Add explicit precondition checks.
  - Add transaction usage for multi-write actions.
  - Add provenance helpers to generated documents and lines.
- Modify `backend/internal/domain/erp/repository.go`
  - Implement action ledger persistence.
  - Add pgx transaction-bound repository support.
- Modify `backend/internal/domain/erp/business_actions_test.go`
  - Extend fake repository with action ledger, generated rows, transaction snapshots, and targeted behavior tests.
- Modify `backend/internal/domain/erp/handler.go`
  - Keep decoding compatible; `ActionInput` additions work through JSON tags.
- Modify `backend/internal/domain/toolruntime/internal_tools.go`
  - Pass Tool Runtime metadata into `erp.ActionInput`.
- Modify `backend/internal/domain/toolruntime/internal_tools_test.go`
  - Assert ERP adapter forwards idempotency/context metadata.
- Modify `backend/cmd/smoke/main.go`
  - Strengthen ERP smoke assertions for provenance, idempotent replay, and final feedback.
- Modify `migrations/001_erp_code_baseline.sql`
  - Add `MACT` and `ACT1` tenant ERP tables.
- Modify `migrations/BASELINE_RESTRUCTURE.md`
  - Document ERP action ledger ownership in stage `001`.

## Task 1: Extend ERP Action Contract Types

**Files:**
- Modify: `backend/internal/domain/erp/model.go`
- Test: `backend/internal/domain/erp/business_actions_test.go`

- [ ] **Step 1: Write failing action contract test**

Append this test to `backend/internal/domain/erp/business_actions_test.go`:

```go
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
```

Add `github.com/google/uuid` to the test import block:

```go
import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
)
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd backend
go test ./internal/domain/erp -run TestActionResultIncludesExecutionContract -count=1
```

Expected: FAIL with compile errors for missing `ActionInput.ActorID`, `ActionInput.ActorType`, `ActionInput.IdempotencyKey`, `ActionInput.Source`, `ActionResult.ExecutionID`, `ActionResult.IdempotencyKey`, `ActionResult.Provenance`, and `ActionResult.PreconditionsChecked`.

- [ ] **Step 3: Extend action model types**

Modify the import block in `backend/internal/domain/erp/model.go`:

```go
import (
	"errors"
	"time"

	"github.com/google/uuid"
)
```

Replace `ActionInput` and `ActionResult` with:

```go
type ActionInput struct {
	Data               map[string]any `json:"data"`
	ActorID            *uuid.UUID     `json:"actor_id,omitempty"`
	ActorType          string         `json:"actor_type,omitempty"`
	IdempotencyKey     string         `json:"idempotency_key,omitempty"`
	Source             string         `json:"source,omitempty"`
	ToolExecutionID    *uuid.UUID     `json:"tool_execution_id,omitempty"`
	AssistantSessionID *uuid.UUID     `json:"assistant_session_id,omitempty"`
}

type ActionResult struct {
	TableCode            string                `json:"table_code"`
	Key                  string                `json:"key"`
	Action               string                `json:"action"`
	Status               string                `json:"status"`
	Record               *Record               `json:"record,omitempty"`
	GeneratedRecords     []Record              `json:"generated_records,omitempty"`
	Effects              map[string]any        `json:"effects,omitempty"`
	ExecutionID          uuid.UUID             `json:"execution_id,omitempty"`
	IdempotencyKey        string                `json:"idempotency_key,omitempty"`
	PreconditionsChecked []ActionPrecondition  `json:"preconditions_checked,omitempty"`
	Provenance           map[string]any        `json:"provenance,omitempty"`
	FailureReason        *ActionFailure        `json:"failure_reason,omitempty"`
}
```

Add these types below `ActionResult`:

```go
const (
	ActionExecutionRunning          = "running"
	ActionExecutionCompleted        = "completed"
	ActionExecutionFailed           = "failed"
	ActionExecutionIdempotentReplay = "idempotent_replay"
)

type ActionFailure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ActionPrecondition struct {
	Key     string `json:"key"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type ActionExecution struct {
	ID                 uuid.UUID      `json:"id"`
	TableCode          string         `json:"table_code"`
	RecordKey          string         `json:"record_key"`
	Action             string         `json:"action"`
	Status             string         `json:"status"`
	IdempotencyKey     string         `json:"idempotency_key"`
	ActorID            *uuid.UUID     `json:"actor_id,omitempty"`
	ActorType          string         `json:"actor_type,omitempty"`
	ToolExecutionID    *uuid.UUID     `json:"tool_execution_id,omitempty"`
	AssistantSessionID *uuid.UUID     `json:"assistant_session_id,omitempty"`
	Source             string         `json:"source,omitempty"`
	FailureCode        string         `json:"failure_code,omitempty"`
	FailureMessage     string         `json:"failure_message,omitempty"`
	Payload            map[string]any `json:"payload"`
	StartedAt          time.Time      `json:"started_at,omitempty"`
	CompletedAt        *time.Time     `json:"completed_at,omitempty"`
}

type ActionGeneratedRecord struct {
	ActionID           uuid.UUID      `json:"action_id"`
	LineNum            int            `json:"line_num"`
	GeneratedTableCode string         `json:"generated_table_code"`
	GeneratedKey       string         `json:"generated_key"`
	RelationType       string         `json:"relation_type"`
	Payload            map[string]any `json:"payload"`
}
```

- [ ] **Step 4: Run test to verify current fake still lacks ledger behavior**

Run:

```bash
cd backend
go test ./internal/domain/erp -run TestActionResultIncludesExecutionContract -count=1
```

Expected: FAIL because `ExecutionID`, `IdempotencyKey`, `Provenance`, or `PreconditionsChecked` is still empty.

- [ ] **Step 5: Commit contract model changes**

Run:

```bash
cd backend
gofmt -w internal/domain/erp/model.go internal/domain/erp/business_actions_test.go
go test ./internal/domain/erp -run TestActionResultIncludesExecutionContract -count=1
```

Expected: FAIL for missing behavior, not compile errors.

Commit:

```bash
git add backend/internal/domain/erp/model.go backend/internal/domain/erp/business_actions_test.go
git commit -m "Extend ERP action contract types"
```

## Task 2: Add ERP Action Ledger Repository Contract

**Files:**
- Modify: `backend/internal/domain/erp/service.go`
- Modify: `backend/internal/domain/erp/business_actions_test.go`
- Modify: `backend/internal/domain/erp/repository.go`
- Modify: `migrations/001_erp_code_baseline.sql`
- Modify: `migrations/BASELINE_RESTRUCTURE.md`

- [ ] **Step 1: Write failing ledger persistence test**

Append this test to `backend/internal/domain/erp/business_actions_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd backend
go test ./internal/domain/erp -run TestActionExecutionLedgerRecordsCompletedAction -count=1
```

Expected: FAIL with compile error for missing `repo.executions`.

- [ ] **Step 3: Extend ERP repository interfaces**

Modify `backend/internal/domain/erp/service.go` and extend `Repository`:

```go
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
```

Add `github.com/google/uuid` to `service.go` imports.

- [ ] **Step 4: Implement fake ledger methods**

Modify `newBusinessFakeRepository` in `backend/internal/domain/erp/business_actions_test.go`:

```go
func newBusinessFakeRepository() *businessFakeRepository {
	return &businessFakeRepository{
		records:          map[string]map[string]Record{},
		children:         map[string][]Record{},
		executions:       map[uuid.UUID]ActionExecution{},
		executionsByKey:  map[string]uuid.UUID{},
		generatedRecords: map[uuid.UUID][]ActionGeneratedRecord{},
	}
}
```

Extend `businessFakeRepository`:

```go
type businessFakeRepository struct {
	records          map[string]map[string]Record
	children         map[string][]Record
	executions       map[uuid.UUID]ActionExecution
	executionsByKey  map[string]uuid.UUID
	generatedRecords map[uuid.UUID][]ActionGeneratedRecord
}
```

Add methods to the fake:

```go
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
```

Add `time` to the test import block.

- [ ] **Step 5: Add PostgreSQL ledger methods**

Modify `backend/internal/domain/erp/repository.go`.

Add imports:

```go
	"time"

	"github.com/google/uuid"
```

Add methods:

```go
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
	payload, _ := json.Marshal(execution.Payload)
	query := `INSERT INTO "MACT" ("ActionID", "TableCode", "RecordKey", "Action", "Status", "IdempotencyKey", "ActorID", "ActorType", "ToolExecutionID", "AssistantSessionID", "Source", "Payload")
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12::jsonb)
		RETURNING "ActionID", "TableCode", "RecordKey", "Action", "Status", "IdempotencyKey", "ActorID", "ActorType", "ToolExecutionID", "AssistantSessionID", "Source", "FailureCode", "FailureMessage", "Payload", "StartedAt", "CompletedAt"`
	return scanActionExecution(r.db.QueryRow(ctx, query, execution.ID, execution.TableCode, execution.RecordKey, execution.Action, execution.Status, execution.IdempotencyKey, execution.ActorID, execution.ActorType, execution.ToolExecutionID, execution.AssistantSessionID, execution.Source, string(payload)))
}

func (r *PostgresRepository) FindActionExecutionByIdempotencyKey(ctx context.Context, key string) (*ActionExecution, error) {
	query := `SELECT "ActionID", "TableCode", "RecordKey", "Action", "Status", "IdempotencyKey", "ActorID", "ActorType", "ToolExecutionID", "AssistantSessionID", "Source", "FailureCode", "FailureMessage", "Payload", "StartedAt", "CompletedAt"
		FROM "MACT" WHERE "IdempotencyKey" = $1 ORDER BY "StartedAt" DESC LIMIT 1`
	return scanActionExecution(r.db.QueryRow(ctx, query, key))
}

func (r *PostgresRepository) CompleteActionExecution(ctx context.Context, id uuid.UUID, status string, payload map[string]any, failure *ActionFailure) (*ActionExecution, error) {
	if payload == nil {
		payload = map[string]any{}
	}
	payloadBytes, _ := json.Marshal(payload)
	failureCode := ""
	failureMessage := ""
	if failure != nil {
		failureCode = failure.Code
		failureMessage = failure.Message
	}
	query := `UPDATE "MACT" SET "Status" = $2, "Payload" = $3::jsonb, "FailureCode" = $4, "FailureMessage" = $5, "CompletedAt" = NOW()
		WHERE "ActionID" = $1
		RETURNING "ActionID", "TableCode", "RecordKey", "Action", "Status", "IdempotencyKey", "ActorID", "ActorType", "ToolExecutionID", "AssistantSessionID", "Source", "FailureCode", "FailureMessage", "Payload", "StartedAt", "CompletedAt"`
	return scanActionExecution(r.db.QueryRow(ctx, query, id, status, string(payloadBytes), failureCode, failureMessage))
}

func (r *PostgresRepository) CreateActionGeneratedRecord(ctx context.Context, record ActionGeneratedRecord) error {
	payload, _ := json.Marshal(record.Payload)
	_, err := r.db.Exec(ctx, `INSERT INTO "ACT1" ("ActionID", "LineNum", "GeneratedTableCode", "GeneratedKey", "RelationType", "Payload")
		VALUES ($1,$2,$3,$4,$5,$6::jsonb)`, record.ActionID, record.LineNum, record.GeneratedTableCode, record.GeneratedKey, record.RelationType, string(payload))
	if err != nil {
		return fmt.Errorf("create ERP action generated record: %w", err)
	}
	return nil
}

func (r *PostgresRepository) ListActionGeneratedRecords(ctx context.Context, actionID uuid.UUID) ([]ActionGeneratedRecord, error) {
	rows, err := r.db.Query(ctx, `SELECT "ActionID", "LineNum", "GeneratedTableCode", "GeneratedKey", "RelationType", "Payload" FROM "ACT1" WHERE "ActionID" = $1 ORDER BY "LineNum"`, actionID)
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
	return items, rows.Err()
}
```

Add scanner helper:

```go
func scanActionExecution(row interface{ Scan(dest ...any) error }) (*ActionExecution, error) {
	var execution ActionExecution
	var payload []byte
	if err := row.Scan(&execution.ID, &execution.TableCode, &execution.RecordKey, &execution.Action, &execution.Status, &execution.IdempotencyKey, &execution.ActorID, &execution.ActorType, &execution.ToolExecutionID, &execution.AssistantSessionID, &execution.Source, &execution.FailureCode, &execution.FailureMessage, &payload, &execution.StartedAt, &execution.CompletedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	execution.Payload = map[string]any{}
	_ = json.Unmarshal(payload, &execution.Payload)
	return &execution, nil
}
```

- [ ] **Step 6: Add migration tables**

Append to `migrations/001_erp_code_baseline.sql` near other ERP extension tables:

```sql
CREATE TABLE IF NOT EXISTS "MACT" (
    "ActionID" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "TableCode" TEXT NOT NULL,
    "RecordKey" TEXT NOT NULL,
    "Action" TEXT NOT NULL,
    "Status" TEXT NOT NULL DEFAULT 'running',
    "IdempotencyKey" TEXT NOT NULL,
    "ActorID" UUID,
    "ActorType" TEXT NOT NULL DEFAULT '',
    "ToolExecutionID" UUID,
    "AssistantSessionID" UUID,
    "Source" TEXT NOT NULL DEFAULT '',
    "FailureCode" TEXT NOT NULL DEFAULT '',
    "FailureMessage" TEXT NOT NULL DEFAULT '',
    "Payload" JSONB NOT NULL DEFAULT '{}'::jsonb,
    "StartedAt" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    "CompletedAt" TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_mact_idempotency_key
    ON "MACT" ("IdempotencyKey");

CREATE INDEX IF NOT EXISTS idx_mact_source_record
    ON "MACT" ("TableCode", "RecordKey", "Action", "Status");

CREATE TABLE IF NOT EXISTS "ACT1" (
    "ActionID" UUID NOT NULL REFERENCES "MACT"("ActionID") ON DELETE CASCADE,
    "LineNum" BIGINT NOT NULL,
    "GeneratedTableCode" TEXT NOT NULL,
    "GeneratedKey" TEXT NOT NULL,
    "RelationType" TEXT NOT NULL DEFAULT 'created',
    "Payload" JSONB NOT NULL DEFAULT '{}'::jsonb,
    "CreatedAt" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY ("ActionID", "LineNum")
);

CREATE INDEX IF NOT EXISTS idx_act1_generated_record
    ON "ACT1" ("GeneratedTableCode", "GeneratedKey");
```

- [ ] **Step 7: Update baseline ownership documentation**

Modify `migrations/BASELINE_RESTRUCTURE.md` in Stage Ownership, after the `001_erp_code_baseline.sql` paragraph:

```markdown
The ERP stage also owns tenant business execution history such as `MACT` and
`ACT1`, because those tables record ERP action attempts and generated business
records inside the tenant runtime. They are not SaaS platform administration
objects and must remain available anywhere the tenant ERP code-table baseline is
installed.
```

- [ ] **Step 8: Run focused tests and commit**

Run:

```bash
cd backend
gofmt -w internal/domain/erp/model.go internal/domain/erp/service.go internal/domain/erp/repository.go internal/domain/erp/business_actions_test.go
go test ./internal/domain/erp -run 'TestActionResultIncludesExecutionContract|TestActionExecutionLedgerRecordsCompletedAction' -count=1
```

Expected: tests still FAIL until Task 3 wraps `RunAction`, but all repository interface compile errors are resolved.

Commit:

```bash
git add backend/internal/domain/erp/model.go backend/internal/domain/erp/service.go backend/internal/domain/erp/repository.go backend/internal/domain/erp/business_actions_test.go migrations/001_erp_code_baseline.sql migrations/BASELINE_RESTRUCTURE.md
git commit -m "Add ERP action ledger contract"
```

## Task 3: Wrap RunAction With Execution Tracking

**Files:**
- Modify: `backend/internal/domain/erp/service.go`
- Modify: `backend/internal/domain/erp/business_actions.go`
- Test: `backend/internal/domain/erp/business_actions_test.go`

- [ ] **Step 1: Write failing failure-ledger test**

Append this test to `backend/internal/domain/erp/business_actions_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
cd backend
go test ./internal/domain/erp -run 'TestActionResultIncludesExecutionContract|TestActionExecutionLedgerRecordsCompletedAction|TestActionExecutionLedgerRecordsValidationFailure' -count=1
```

Expected: FAIL because `RunAction` does not create or complete ledger entries.

- [ ] **Step 3: Add execution context helpers**

Add to `backend/internal/domain/erp/service.go` below `NewService`:

```go
type actionRunState struct {
	execution     *ActionExecution
	idempotencyKey string
	preconditions []ActionPrecondition
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
```

Add `strings` to the import block if not already present:

```go
import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)
```

- [ ] **Step 4: Wrap RunAction**

Replace `RunAction` in `backend/internal/domain/erp/service.go` with:

```go
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
	if existing, err := s.repo.FindActionExecutionByIdempotencyKey(ctx, idempotencyKey); err == nil && existing.Status == ActionExecutionCompleted {
		return s.idempotentReplayResult(ctx, existing)
	} else if err == nil && existing.Status == ActionExecutionFailed && input.Data["retry_failed"] != true {
		return nil, fmt.Errorf("%w: previous ERP action execution failed; pass retry_failed=true to retry", ErrValidation)
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
			Effects:   map[string]any{"definition": def.Label},
		}
		actionErr = nil
	}
	if actionErr != nil {
		failure := actionFailureFromError(actionErr)
		_, _ = s.repo.CompleteActionExecution(ctx, execution.ID, ActionExecutionFailed, map[string]any{"error": actionErr.Error()}, failure)
		return nil, actionErr
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
	if _, err := s.repo.CompleteActionExecution(ctx, execution.ID, ActionExecutionCompleted, actionResultPayload(result), nil); err != nil {
		return nil, err
	}
	return result, nil
}
```

Add replay helper:

```go
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
		IdempotencyKey:        execution.IdempotencyKey,
		PreconditionsChecked: []ActionPrecondition{{Key: "idempotency", Status: "passed", Message: "completed execution replayed"}},
		Provenance: map[string]any{
			"source":              execution.Source,
			"action_execution_id": execution.ID.String(),
			"idempotency_key":     execution.IdempotencyKey,
		},
	}, nil
}
```

- [ ] **Step 5: Add firstNonEmptyString to ERP service**

Add to `backend/internal/domain/erp/business_actions.go` or `service.go`:

```go
func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
```

- [ ] **Step 6: Run tests and commit**

Run:

```bash
cd backend
gofmt -w internal/domain/erp/service.go internal/domain/erp/business_actions.go internal/domain/erp/business_actions_test.go
go test ./internal/domain/erp -run 'TestActionResultIncludesExecutionContract|TestActionExecutionLedgerRecordsCompletedAction|TestActionExecutionLedgerRecordsValidationFailure' -count=1
go test ./internal/domain/erp -count=1
```

Expected: PASS.

Commit:

```bash
git add backend/internal/domain/erp/service.go backend/internal/domain/erp/business_actions.go backend/internal/domain/erp/business_actions_test.go
git commit -m "Track ERP action executions"
```

## Task 4: Add Explicit Preconditions

**Files:**
- Modify: `backend/internal/domain/erp/business_actions.go`
- Test: `backend/internal/domain/erp/business_actions_test.go`

- [ ] **Step 1: Write failing precondition tests**

Append these tests to `backend/internal/domain/erp/business_actions_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
cd backend
go test ./internal/domain/erp -run 'TestRequirementApproveRequiresAnalyzedStatus|TestSalesOrderApproveRequiresConfirmedOrder|TestCloseProjectFeedbackRequiresCostRefresh' -count=1
```

Expected: FAIL because current actions allow at least `MREQ.approve`, `MRDR.approve`, or `MPRJ.close-feedback` without these explicit preconditions.

- [ ] **Step 3: Add precondition helpers**

Add to `backend/internal/domain/erp/business_actions.go`:

```go
func passedPrecondition(key string, message string) ActionPrecondition {
	return ActionPrecondition{Key: key, Status: "passed", Message: message}
}

func (s *Service) requireRecordField(ctx context.Context, tableCode string, key string, field string, expected string, message string) (*Record, ActionPrecondition, error) {
	table, err := s.table(tableCode)
	if err != nil {
		return nil, ActionPrecondition{}, err
	}
	record, err := s.repo.GetRecord(ctx, table, key)
	if err != nil {
		return nil, ActionPrecondition{}, err
	}
	check := ActionPrecondition{Key: tableCode + "." + field, Status: "passed", Message: message}
	if !documentFieldEquals(record, field, expected) {
		check.Status = "failed"
		return nil, check, fmt.Errorf("%w: %s", ErrValidation, message)
	}
	return record, check, nil
}

func attachPreconditions(result *ActionResult, checks ...ActionPrecondition) *ActionResult {
	if result == nil {
		return result
	}
	result.PreconditionsChecked = append(result.PreconditionsChecked, checks...)
	return result
}
```

- [ ] **Step 4: Apply preconditions to actions**

Modify `runBusinessAction` cases in `backend/internal/domain/erp/business_actions.go`:

```go
case "MREQ:approve":
	_, check, err := s.requireRecordField(ctx, tableCode, key, "Status", "analyzed", "requirement must be analyzed before approval")
	if err != nil {
		return nil, err
	}
	return attachPreconditions(s.mergeStatusAction(ctx, tableCode, key, action, map[string]any{"Status": "approved", "ApprovedBy": input.Data["approver"]}), check)
case "MRDR:approve":
	_, check, err := s.requireRecordField(ctx, tableCode, key, "Confirmed", "Y", "sales order must be confirmed before approval")
	if err != nil {
		return nil, err
	}
	return attachPreconditions(s.mergeStatusAction(ctx, tableCode, key, action, map[string]any{"WddStatus": "A"}), check)
```

Because Go cannot pass `(*ActionResult, error)` directly into `attachPreconditions`, use this exact pattern:

```go
result, err := s.mergeStatusAction(ctx, tableCode, key, action, map[string]any{"Status": "approved", "ApprovedBy": input.Data["approver"]})
if err != nil {
	return nil, err
}
return attachPreconditions(result, check), nil
```

Apply the same pattern for sales order approval.

Modify `closeProjectFeedback`:

```go
project, check, err := s.requireRecordField(ctx, "MPRJ", key, "LastCostCode", stringValue(input.Data, "CostCode", stringValue(input.Data, "LastCostCode", "")), "project cost must be refreshed before feedback closes")
if err != nil {
	return nil, err
}
if stringValue(project.Data, "LastCostCode", "") == "" {
	return nil, fmt.Errorf("%w: project cost must be refreshed before feedback closes", ErrValidation)
}
```

Then include `check` in the returned result:

```go
return attachPreconditions(&ActionResult{TableCode: "MPRJ", Key: key, Action: "close-feedback", Status: "closed", Record: project, GeneratedRecords: []Record{*feedback}}, check), nil
```

- [ ] **Step 5: Run tests and commit**

Run:

```bash
cd backend
gofmt -w internal/domain/erp/business_actions.go internal/domain/erp/business_actions_test.go
go test ./internal/domain/erp -run 'TestRequirementApproveRequiresAnalyzedStatus|TestSalesOrderApproveRequiresConfirmedOrder|TestCloseProjectFeedbackRequiresCostRefresh|TestPurchaseOrderSubmitAndApprove|TestSalesDeliveryPaymentLoop' -count=1
go test ./internal/domain/erp -count=1
```

Expected: PASS.

Commit:

```bash
git add backend/internal/domain/erp/business_actions.go backend/internal/domain/erp/business_actions_test.go
git commit -m "Add ERP action preconditions"
```

## Task 5: Add Transactional Multi-Write Actions

**Files:**
- Modify: `backend/internal/domain/erp/repository.go`
- Modify: `backend/internal/domain/erp/service.go`
- Modify: `backend/internal/domain/erp/business_actions.go`
- Test: `backend/internal/domain/erp/business_actions_test.go`

- [ ] **Step 1: Write failing rollback test**

Append this test to `backend/internal/domain/erp/business_actions_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd backend
go test ./internal/domain/erp -run TestDeliveryPostRollsBackWhenInventoryFails -count=1
```

Expected: FAIL because generated records may remain after inventory failure.

- [ ] **Step 3: Implement fake transaction snapshot**

Add this method to `businessFakeRepository`:

```go
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
```

Add clone helpers:

```go
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
```

- [ ] **Step 4: Implement PostgreSQL RunInTx**

Modify `backend/internal/domain/erp/repository.go`.

Change `PostgresRepository`:

```go
type PostgresRepository struct {
	db      *pgxpool.Pool
	tx      pgx.Tx
	querier erpQuerier
}

type erpQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}
```

Add import:

```go
	"github.com/jackc/pgx/v5/pgconn"
```

Update constructor:

```go
func NewRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db, querier: db}
}
```

Replace all `r.db.Query`, `r.db.QueryRow`, and `r.db.Exec` calls with `r.querier.Query`, `r.querier.QueryRow`, and `r.querier.Exec`.

Add:

```go
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
```

- [ ] **Step 5: Add service transaction helper**

Add to `backend/internal/domain/erp/service.go`:

```go
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
```

- [ ] **Step 6: Wrap multi-write actions**

Modify `runBusinessAction` in `backend/internal/domain/erp/business_actions.go`:

```go
case "MREQ:convert-to-project":
	return s.runInTx(ctx, func(tx *Service) (*ActionResult, error) {
		return tx.convertRequirementToProject(ctx, key, input)
	})
case "MPRJ:refresh-cost":
	return s.runInTx(ctx, func(tx *Service) (*ActionResult, error) {
		return tx.refreshProjectCost(ctx, key, input)
	})
case "MPRJ:close-feedback":
	return s.runInTx(ctx, func(tx *Service) (*ActionResult, error) {
		return tx.closeProjectFeedback(ctx, key, input)
	})
case "MPDN:post":
	return s.runInTx(ctx, func(tx *Service) (*ActionResult, error) {
		return tx.postGoodsReceiptPO(ctx, key)
	})
case "MDLN:post":
	return s.runInTx(ctx, func(tx *Service) (*ActionResult, error) {
		return tx.postDelivery(ctx, key)
	})
case "MINV:post":
	return s.runInTx(ctx, func(tx *Service) (*ActionResult, error) {
		return tx.postInvoice(ctx, key)
	})
case "MRCT:allocate":
	return s.runInTx(ctx, func(tx *Service) (*ActionResult, error) {
		return tx.allocateIncomingPayment(ctx, key, input)
	})
case "MIGN:post":
	return s.runInTx(ctx, func(tx *Service) (*ActionResult, error) {
		return tx.postInventoryDocument(ctx, tableCode, key, 1)
	})
case "MIGE:post":
	return s.runInTx(ctx, func(tx *Service) (*ActionResult, error) {
		return tx.postInventoryDocument(ctx, tableCode, key, -1)
	})
case "MJDT:post":
	return s.runInTx(ctx, func(tx *Service) (*ActionResult, error) {
		return tx.mergeStatusAction(ctx, tableCode, key, action, map[string]any{"BtfStatus": "P"})
	})
```

- [ ] **Step 7: Run tests and commit**

Run:

```bash
cd backend
gofmt -w internal/domain/erp/repository.go internal/domain/erp/service.go internal/domain/erp/business_actions.go internal/domain/erp/business_actions_test.go
go test ./internal/domain/erp -run 'TestDeliveryPostRollsBackWhenInventoryFails|TestSalesDeliveryPaymentLoop|TestGoodsReceiptPostGeneratesInventoryAndPayable' -count=1
go test ./internal/domain/erp -count=1
```

Expected: PASS.

Commit:

```bash
git add backend/internal/domain/erp/repository.go backend/internal/domain/erp/service.go backend/internal/domain/erp/business_actions.go backend/internal/domain/erp/business_actions_test.go
git commit -m "Add ERP action transaction boundaries"
```

## Task 6: Add Idempotency Replay and Generated Record Ledger Rows

**Files:**
- Modify: `backend/internal/domain/erp/business_actions.go`
- Modify: `backend/internal/domain/erp/service.go`
- Test: `backend/internal/domain/erp/business_actions_test.go`

- [ ] **Step 1: Write failing idempotency replay test**

Append to `backend/internal/domain/erp/business_actions_test.go`:

```go
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
	if repo.childCount("MIGN", "IGN-GR-1", "IGN1") != 1 {
		t.Fatalf("MIGN child rows = %d, want 1 after replay", repo.childCount("MIGN", "IGN-GR-1", "IGN1"))
	}
	if repo.balance("I-1", "W-1") != 2 {
		t.Fatalf("balance = %v, want 2 after replay", repo.balance("I-1", "W-1"))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd backend
go test ./internal/domain/erp -run TestGoodsReceiptPostIsIdempotent -count=1
```

Expected: FAIL because generated record ledger rows are not written for replay.

- [ ] **Step 3: Add generated record ledger helper**

Add to `backend/internal/domain/erp/service.go`:

```go
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
```

In `RunAction`, before completing the execution, add:

```go
if err := s.recordGeneratedRecords(ctx, execution.ID, result.GeneratedRecords); err != nil {
	return nil, err
}
```

- [ ] **Step 4: Ensure replay returns generated records**

Modify `idempotentReplayResult` if needed so `GeneratedRecords` is built from `ListActionGeneratedRecords`:

```go
generated := make([]Record, 0, len(generatedRows))
for _, row := range generatedRows {
	generated = append(generated, Record{
		TableCode: row.GeneratedTableCode,
		Key:       row.GeneratedKey,
		Data:      row.Payload,
	})
}
```

- [ ] **Step 5: Run tests and commit**

Run:

```bash
cd backend
gofmt -w internal/domain/erp/service.go internal/domain/erp/business_actions_test.go
go test ./internal/domain/erp -run 'TestGoodsReceiptPostIsIdempotent|TestGoodsReceiptPostGeneratesInventoryAndPayable' -count=1
go test ./internal/domain/erp -count=1
```

Expected: PASS.

Commit:

```bash
git add backend/internal/domain/erp/service.go backend/internal/domain/erp/business_actions_test.go
git commit -m "Add ERP action idempotency replay"
```

## Task 7: Add Generated Record Provenance

**Files:**
- Modify: `backend/internal/domain/erp/business_actions.go`
- Test: `backend/internal/domain/erp/business_actions_test.go`

- [ ] **Step 1: Write failing provenance test**

Append to `backend/internal/domain/erp/business_actions_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd backend
go test ./internal/domain/erp -run TestDeliveryPostAddsGeneratedRecordProvenance -count=1
```

Expected: FAIL because generated records do not include provenance.

- [ ] **Step 3: Add action provenance helpers**

Add to `backend/internal/domain/erp/business_actions.go`:

```go
type actionProvenanceInput struct {
	TableCode          string
	Key                string
	Action             string
	ExecutionID        uuid.UUID
	IdempotencyKey     string
	ActorType          string
	ToolExecutionID    *uuid.UUID
	AssistantSessionID *uuid.UUID
}

func actionProvenance(input actionProvenanceInput) map[string]any {
	return map[string]any{
		"source_table_code":   input.TableCode,
		"source_key":          input.Key,
		"source_action":       input.Action,
		"action_execution_id": input.ExecutionID.String(),
		"idempotency_key":     input.IdempotencyKey,
		"created_by_actor_type": input.ActorType,
		"tool_execution_id":      uuidString(input.ToolExecutionID),
		"assistant_session_id":   uuidString(input.AssistantSessionID),
	}
}

func withProvenance(payload map[string]any, provenance map[string]any) map[string]any {
	next := copyData(payload)
	next["provenance"] = provenance
	return next
}

func uuidString(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}
```

Add `github.com/google/uuid` to `business_actions.go` imports.

- [ ] **Step 4: Pass execution metadata into business actions**

Create an internal context key in `backend/internal/domain/erp/service.go`:

```go
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
```

In `RunAction`, before calling `runBusinessAction`, wrap the context:

```go
ctx = contextWithActionExecution(ctx, actionExecutionContext{
	ExecutionID:        execution.ID,
	IdempotencyKey:     idempotencyKey,
	ActorType:          input.ActorType,
	ToolExecutionID:    input.ToolExecutionID,
	AssistantSessionID: input.AssistantSessionID,
})
```

- [ ] **Step 5: Add provenance to generated documents**

Modify `createDocument` signature:

```go
func (s *Service) createDocument(ctx context.Context, sourceTableCode string, sourceKey string, sourceAction string, tableCode string, key string, payload map[string]any) (*Record, error)
```

Inside it:

```go
meta := actionExecutionFromContext(ctx)
payload = withProvenance(payload, actionProvenance(actionProvenanceInput{
	TableCode:          sourceTableCode,
	Key:                sourceKey,
	Action:             sourceAction,
	ExecutionID:        meta.ExecutionID,
	IdempotencyKey:     meta.IdempotencyKey,
	ActorType:          meta.ActorType,
	ToolExecutionID:    meta.ToolExecutionID,
	AssistantSessionID: meta.AssistantSessionID,
}))
```

Update call sites:

```go
goodsReceipt, err := s.createDocument(ctx, "MPDN", key, "post", "MIGN", "IGN-"+key, map[string]any{...})
payable, err := s.createDocument(ctx, "MPDN", key, "post", "MPCH", "AP-"+key, map[string]any{...})
goodsIssue, err := s.createDocument(ctx, "MDLN", key, "post", "MIGE", "IGE-"+key, map[string]any{...})
invoice, err := s.createDocument(ctx, "MDLN", key, "post", "MINV", "INV-"+key, map[string]any{...})
journal, err := s.createDocument(ctx, "MINV", key, "post", "MJDT", "JE-"+key, map[string]any{"BaseEntry": key})
```

For `refreshProjectCost` and `closeProjectFeedback`, create `MCST` and `MFDB` with provenance by passing source table `MPRJ`.

- [ ] **Step 6: Run tests and commit**

Run:

```bash
cd backend
gofmt -w internal/domain/erp/service.go internal/domain/erp/business_actions.go internal/domain/erp/business_actions_test.go
go test ./internal/domain/erp -run 'TestDeliveryPostAddsGeneratedRecordProvenance|TestGoodsReceiptPostIsIdempotent|TestInvoicePostCreatesJournalEntryWithJournalPrimaryKey' -count=1
go test ./internal/domain/erp -count=1
```

Expected: PASS.

Commit:

```bash
git add backend/internal/domain/erp/service.go backend/internal/domain/erp/business_actions.go backend/internal/domain/erp/business_actions_test.go
git commit -m "Add ERP generated record provenance"
```

## Task 8: Pass Tool Runtime Metadata Into ERP Actions

**Files:**
- Modify: `backend/internal/domain/toolruntime/internal_tools.go`
- Modify: `backend/internal/domain/toolruntime/internal_tools_test.go`

- [ ] **Step 1: Write failing Tool Runtime metadata test**

Modify `fakeERPActionService` in `backend/internal/domain/toolruntime/internal_tools_test.go`:

```go
type fakeERPActionService struct {
	tableCode string
	key       string
	action    string
	input     erp.ActionInput
}

func (f *fakeERPActionService) RunAction(_ context.Context, tableCode string, key string, action string, input erp.ActionInput) (*erp.ActionResult, error) {
	f.tableCode = tableCode
	f.key = key
	f.action = action
	f.input = input
	return &erp.ActionResult{TableCode: tableCode, Key: key, Action: action, Status: "approved", Record: &erp.Record{TableCode: tableCode, Key: key, Data: input.Data}}, nil
}
```

Append this test:

```go
func TestERPActionExecuteToolForwardsExecutionMetadata(t *testing.T) {
	erpSvc := &fakeERPActionService{}
	sessionID := uuid.New()
	toolExecutionID := uuid.New()
	tools := InternalToolsWithPlatform(nil, nil, nil, PlatformToolServices{ERP: erpSvc})

	_, err := tools["erp.action.execute"](context.Background(), ExecuteToolInput{
		ActorID:        uuid.New(),
		ActorType:      "internal_human",
		IdempotencyKey: "assistant-session-tool-call",
		Arguments: map[string]any{
			"table_code":           "MREQ",
			"key":                  "REQ-1",
			"action":               "approve",
			"assistant_session_id": sessionID.String(),
			"tool_execution_id":    toolExecutionID.String(),
			"context_package_id":   uuid.New().String(),
			"data":                 map[string]any{"approver": "u1"},
		},
	})
	if err != nil {
		t.Fatalf("tool returned error: %v", err)
	}
	if erpSvc.input.ActorType != "internal_human" || erpSvc.input.ActorID == nil {
		t.Fatalf("input actor = %#v/%#v, want forwarded actor", erpSvc.input.ActorID, erpSvc.input.ActorType)
	}
	if erpSvc.input.Source != "toolruntime" || erpSvc.input.IdempotencyKey != "assistant-session-tool-call" {
		t.Fatalf("input source/idempotency = %q/%q, want toolruntime/assistant-session-tool-call", erpSvc.input.Source, erpSvc.input.IdempotencyKey)
	}
	if erpSvc.input.AssistantSessionID == nil || *erpSvc.input.AssistantSessionID != sessionID {
		t.Fatalf("assistant session id = %#v, want %s", erpSvc.input.AssistantSessionID, sessionID)
	}
	if erpSvc.input.ToolExecutionID == nil || *erpSvc.input.ToolExecutionID != toolExecutionID {
		t.Fatalf("tool execution id = %#v, want %s", erpSvc.input.ToolExecutionID, toolExecutionID)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd backend
go test ./internal/domain/toolruntime -run TestERPActionExecuteToolForwardsExecutionMetadata -count=1
```

Expected: FAIL because `erpActionExecuteTool` only passes `Data`.

- [ ] **Step 3: Forward metadata in adapter**

Modify `erpActionExecuteTool` in `backend/internal/domain/toolruntime/internal_tools.go`:

```go
toolExecutionID, err := optionalUUIDArg(input.Arguments, "tool_execution_id")
if err != nil {
	return ToolResult{}, err
}
assistantSessionID, err := optionalUUIDArg(input.Arguments, "assistant_session_id")
if err != nil {
	return ToolResult{}, err
}
actorID := input.ActorID
result, err := erpSvc.RunAction(ctx, tableCode, key, action, erp.ActionInput{
	Data:               mapArg(input.Arguments, "data"),
	ActorID:            &actorID,
	ActorType:          input.ActorType,
	IdempotencyKey:     input.IdempotencyKey,
	Source:             "toolruntime",
	ToolExecutionID:    toolExecutionID,
	AssistantSessionID: assistantSessionID,
})
```

- [ ] **Step 4: Run tests and commit**

Run:

```bash
cd backend
gofmt -w internal/domain/toolruntime/internal_tools.go internal/domain/toolruntime/internal_tools_test.go
go test ./internal/domain/toolruntime -run 'TestERPActionExecuteToolRunsERPAction|TestERPActionExecuteToolForwardsExecutionMetadata' -count=1
go test ./internal/domain/toolruntime -count=1
```

Expected: PASS.

Commit:

```bash
git add backend/internal/domain/toolruntime/internal_tools.go backend/internal/domain/toolruntime/internal_tools_test.go
git commit -m "Forward Tool Runtime metadata to ERP actions"
```

## Task 9: Strengthen ERP Smoke Coverage

**Files:**
- Modify: `backend/cmd/smoke/main.go`
- Test: `backend/cmd/smoke/main_test.go`

- [ ] **Step 1: Add smoke assertions for action contract**

In `backend/cmd/smoke/main.go`, add helper functions near existing smoke helpers:

```go
func requireActionContract(result responseMap, label string) {
	must(fmt.Sprint(result["execution_id"]) != "" && fmt.Sprint(result["execution_id"]) != "<nil>", label+" missing execution_id")
	must(fmt.Sprint(result["idempotency_key"]) != "" && fmt.Sprint(result["idempotency_key"]) != "<nil>", label+" missing idempotency_key")
	preconditions := asList(result["preconditions_checked"])
	must(len(preconditions) > 0, label+" missing preconditions_checked")
}

func requireGeneratedProvenance(result responseMap, tableCode string) {
	generated := asList(result["generated_records"])
	for _, item := range generated {
		record, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if fmt.Sprint(record["table_code"]) != tableCode {
			continue
		}
		data, _ := record["data"].(map[string]any)
		provenance, _ := data["provenance"].(map[string]any)
		must(fmt.Sprint(provenance["source_table_code"]) != "", "missing provenance source_table_code for "+tableCode)
		return
	}
	must(false, "missing generated table "+tableCode)
}
```

- [ ] **Step 2: Use helpers in smoke flow**

After action calls in `backend/cmd/smoke/main.go`, add:

```go
approvedRequirement := c.post("/erp/MREQ/"+requirementKey+"/actions/approve", responseMap{"data": responseMap{"approver": userID}, "idempotency_key": "smoke-approve-"+requirementKey})
requireActionContract(approvedRequirement, "requirement approve")
converted := c.post("/erp/MREQ/"+requirementKey+"/actions/convert-to-project", responseMap{"data": responseMap{"PrjCode": projectKey}, "idempotency_key": "smoke-convert-"+requirementKey})
requireActionContract(converted, "requirement convert")
requireGeneratedProvenance(converted, "MPRJ")
```

Replace existing `converted :=` line with the snippet above so the variable remains available.

For receipt and delivery post actions:

```go
receiptPost := c.post("/erp/MPDN/"+goodsReceiptKey+"/actions/post", responseMap{"idempotency_key": "smoke-receipt-"+goodsReceiptKey})
requireActionContract(receiptPost, "goods receipt post")
requireGeneratedProvenance(receiptPost, "MIGN")
```

```go
deliveryPost := c.post("/erp/MDLN/"+deliveryKey+"/actions/post", responseMap{"idempotency_key": "smoke-delivery-"+deliveryKey})
requireActionContract(deliveryPost, "delivery post")
requireGeneratedProvenance(deliveryPost, "MIGE")
requireGeneratedProvenance(deliveryPost, "MINV")
```

For final project actions:

```go
costRefresh := c.post("/erp/MPRJ/"+projectKey+"/actions/refresh-cost", responseMap{"idempotency_key": "smoke-cost-"+projectKey})
requireActionContract(costRefresh, "project cost refresh")
feedback := c.post("/erp/MPRJ/"+projectKey+"/actions/close-feedback", responseMap{"data": responseMap{"result": "accepted"}, "idempotency_key": "smoke-feedback-"+projectKey})
requireActionContract(feedback, "project feedback close")
```

- [ ] **Step 3: Run smoke package tests**

Run:

```bash
cd backend
gofmt -w cmd/smoke/main.go cmd/smoke/main_test.go
go test ./cmd/smoke -count=1
```

Expected: PASS for compile-level smoke tests. Runtime smoke still depends on the local service stack and is covered by existing smoke invocation workflow.

- [ ] **Step 4: Commit**

```bash
git add backend/cmd/smoke/main.go backend/cmd/smoke/main_test.go
git commit -m "Strengthen ERP smoke action assertions"
```

## Task 10: Final Verification

**Files:**
- Review all modified backend and migration files.

- [ ] **Step 1: Run focused ERP tests**

Run:

```bash
cd backend
go test ./internal/domain/erp -count=1
```

Expected: PASS.

- [ ] **Step 2: Run Tool Runtime tests**

Run:

```bash
cd backend
go test ./internal/domain/toolruntime -count=1
```

Expected: PASS.

- [ ] **Step 3: Run smoke package tests**

Run:

```bash
cd backend
go test ./cmd/smoke -count=1
```

Expected: PASS.

- [ ] **Step 4: Run full backend tests**

Run:

```bash
cd backend
go test ./...
```

Expected: PASS.

- [ ] **Step 5: Build backend server**

Run:

```bash
cd backend
go build ./cmd/server
```

Expected: PASS with no output.

- [ ] **Step 6: Check diff hygiene**

Run:

```bash
git diff --check
git status --short
```

Expected: no whitespace errors. Only intended Phase 1 files should be modified or staged. Existing untracked user files such as the YC markdown, `docs/investor.rar`, `docs/investor/`, and `tools/` must remain untouched unless the user explicitly asks to include them.

- [ ] **Step 7: Fresh migration verification**

Start PostgreSQL if it is not already running:

```bash
docker compose up -d postgres
```

Create a separate verification database so the developer database is not modified:

```powershell
$env:PGPASSWORD='postgres'
psql -h localhost -U postgres -d postgres -c "CREATE DATABASE meta_org_phase1_verify;"
```

Apply the active staged baselines in order:

```powershell
$env:PGPASSWORD='postgres'
psql -h localhost -U postgres -d meta_org_phase1_verify -f migrations/000_saas_platform_management_baseline.sql
psql -h localhost -U postgres -d meta_org_phase1_verify -f migrations/001_erp_code_baseline.sql
psql -h localhost -U postgres -d meta_org_phase1_verify -f migrations/002_erp_platform_integration_baseline.sql
psql -h localhost -U postgres -d meta_org_phase1_verify -f migrations/004_ai_capability_baseline.sql
psql -h localhost -U postgres -d meta_org_phase1_verify -c "SELECT COUNT(*) AS not_valid_constraints FROM pg_constraint WHERE NOT convalidated;"
```

Expected: the final query returns `0`.

Clean up only the verification database:

```powershell
$env:PGPASSWORD='postgres'
psql -h localhost -U postgres -d postgres -c "DROP DATABASE meta_org_phase1_verify WITH (FORCE);"
```

If `psql` is not available, use the explicit Windows client path from `docs/operations/local-non-docker-saas-deployment.md`:

```powershell
$env:PGPASSWORD='postgres'
& 'C:\Program Files\PostgreSQL\18\bin\psql.exe' -h localhost -U postgres -d postgres -c "CREATE DATABASE meta_org_phase1_verify;"
& 'C:\Program Files\PostgreSQL\18\bin\psql.exe' -h localhost -U postgres -d meta_org_phase1_verify -f migrations/000_saas_platform_management_baseline.sql
& 'C:\Program Files\PostgreSQL\18\bin\psql.exe' -h localhost -U postgres -d meta_org_phase1_verify -f migrations/001_erp_code_baseline.sql
& 'C:\Program Files\PostgreSQL\18\bin\psql.exe' -h localhost -U postgres -d meta_org_phase1_verify -f migrations/002_erp_platform_integration_baseline.sql
& 'C:\Program Files\PostgreSQL\18\bin\psql.exe' -h localhost -U postgres -d meta_org_phase1_verify -f migrations/004_ai_capability_baseline.sql
& 'C:\Program Files\PostgreSQL\18\bin\psql.exe' -h localhost -U postgres -d meta_org_phase1_verify -c "SELECT COUNT(*) AS not_valid_constraints FROM pg_constraint WHERE NOT convalidated;"
& 'C:\Program Files\PostgreSQL\18\bin\psql.exe' -h localhost -U postgres -d postgres -c "DROP DATABASE meta_org_phase1_verify WITH (FORCE);"
```

If neither `psql` command is available, report that fresh migration verification could not be run and do not claim migration verification passed.

- [ ] **Step 8: Commit final cleanup if needed**

If final formatting or migration documentation changed after the last task, commit:

```bash
git add backend/internal/domain/erp backend/internal/domain/toolruntime backend/cmd/smoke migrations/001_erp_code_baseline.sql migrations/BASELINE_RESTRUCTURE.md
git commit -m "Finalize ERP business loop hardening"
```

If there are no additional changes, do not create an empty commit.

## Plan Self-Review

Spec coverage:

- Action contract is covered by Tasks 1 and 3.
- ERP action ledger is covered by Tasks 2 and 3.
- Transaction boundary is covered by Task 5.
- Idempotency and replay are covered by Task 6.
- Generated record provenance is covered by Task 7.
- Tool Runtime metadata passthrough is covered by Task 8.
- Smoke coverage is covered by Task 9.
- Migration baseline governance is covered by Tasks 2 and 10.

Scope check:

- This plan is backend-only except for migration documentation. It does not include tenant workbench UI, platform solution factory UI, or monitoring agent automation.
- The plan preserves ERP table-code APIs as the tenant business runtime surface.

Type consistency:

- `ActionInput`, `ActionResult`, `ActionExecution`, `ActionGeneratedRecord`, `ActionFailure`, and `ActionPrecondition` are introduced before later tasks use them.
- Repository method names in tests match the repository interface additions.
- Tool Runtime adapter changes use the existing `ExecuteToolInput` and `erp.ActionInput` boundaries.

Migration check:

- New `MACT` and `ACT1` tables belong to `001_erp_code_baseline.sql`.
- `BASELINE_RESTRUCTURE.md` is updated in the same task as the migration.
