# ERP Business Migration and Industry Solution Standard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate the existing project, procurement, sales, inventory, finance, costing, and feedback capabilities into ERP table-code APIs, rebuild missing SAP-style business loops, restore typed business UI, expose AI-assistant-callable operations, and turn the reconstruction method into a reusable SaaS platform management industry-solution workflow.

**Architecture:** Keep the ERP table-code schema as the physical and API naming baseline. Add a typed ERP business action layer above generic table CRUD, then reconnect frontend workspaces, API operation metadata, assistant context targets, smoke tests, and SaaS industry package/schema workflows to that layer.

**Tech Stack:** Go backend with chi routes and pgx repositories, PostgreSQL migrations, Next.js/React/TypeScript frontend, existing i18n, existing assistant and SaaS platform management domains.

---

## Scope And Verification Contract

This plan must not be treated as complete when tables exist but business behavior is missing. Completion requires all of these to pass:

- Backend tests prove action state transitions and cross-table writes.
- ERP catalog tests prove module, submodule, business document, table, child
  table, and action hierarchy exists.
- Smoke test proves an end-to-end flow creates records and follow-on documents.
- Frontend build proves typed workspaces compile.
- API operation metadata exposes ERP action APIs.
- Assistant context APIs expose ERP business objects and proposal targets.
- SaaS platform management can create a reusable industry solution change package that includes database, function, workflow, permission, UI, API, and assistant metadata.

Required final commands:

```powershell
cd backend
go test ./...
go build ./cmd/server
cd ..\frontend
npm run lint
npm run build
cd ..
git diff --check
```

Expected final result:

- All commands exit 0.
- `git diff --check` may print CRLF warnings on Windows, but must not report whitespace errors.

---

## File Responsibility Map

### Backend ERP Core

- Modify `backend/internal/domain/erp/model.go`
  - Add ERP module, submodule, business document, action request/response
    types, action registry metadata, process step metadata, and typed status
    constants.
- Modify `backend/internal/domain/erp/catalog.go`
  - Ensure all ERP and project extension table codes exist in catalog.
  - Expose module -> submodule -> business document -> master table -> child
    table -> action hierarchy.
  - Ensure tables required by action logic expose needed typed fields and `Payload`.
- Modify `backend/internal/domain/erp/repository.go`
  - Add transactional helpers for reading/updating master records, appending child records, upserting balances, and creating generated documents.
- Modify `backend/internal/domain/erp/service.go`
  - Keep generic CRUD.
  - Add action dispatch and typed business action services.
- Modify `backend/internal/domain/erp/handler.go`
  - Add `/api/v1/erp/{tableCode}/{key}/actions/{action}` endpoint.
  - Add action catalog endpoint if API workbench needs discoverability.
- Create `backend/internal/domain/erp/actions.go`
  - Define action registry, validation rules, transition rules, and table-code/action mapping.
- Create `backend/internal/domain/erp/project_actions.go`
  - Requirement approval, requirement-to-project conversion, project members, deliverables, cost refresh, feedback close.
- Create `backend/internal/domain/erp/procurement_actions.go`
  - Purchase submit, approve, goods receipt post, A/P invoice generation.
- Create `backend/internal/domain/erp/sales_actions.go`
  - Sales quote/order confirmation, approval, delivery post, A/R invoice generation, incoming payment allocation.
- Create `backend/internal/domain/erp/inventory_actions.go`
  - Goods receipt, goods issue, transfer, adjustment, count, balance mutation.
- Create `backend/internal/domain/erp/finance_actions.go`
  - Journal posting, invoice posting, incoming/outgoing payment allocation, receivable/payable status updates.
- Create `backend/internal/domain/erp/industry_solution.go`
  - Build reusable industry solution package assets from ERP tables, actions, process definitions, permissions, UI metadata, API metadata, and assistant metadata.

### Backend Tests And Smoke

- Modify `backend/internal/domain/erp/service_test.go`
  - Add action dispatch tests and registry tests.
- Create `backend/internal/domain/erp/actions_test.go`
  - State transition and validation tests.
- Create `backend/internal/domain/erp/hierarchy_test.go`
  - Verify ERP module, submodule, business document, table, child table, and
    action hierarchy.
- Create `backend/internal/domain/erp/business_flow_test.go`
  - In-memory end-to-end flow from requirement to feedback close.
- Create `backend/internal/domain/erp/industry_solution_test.go`
  - Verify generated SaaS industry solution assets are complete.
- Modify `backend/cmd/smoke/main.go`
  - Replace table-only smoke with complete ERP business loop smoke.
- Modify `backend/cmd/smoke/main_test.go`
  - Assert smoke expected-created paths include ERP action endpoints.

### Backend Gateway And SaaS Platform

- Modify `backend/cmd/server/main.go`
  - Wire ERP service dependencies for action layer and industry solution generator.
- Modify `backend/internal/domain/systemadmin/handler.go`
  - Add platform management endpoints for ERP industry solution standard flow if not covered by existing schema package endpoints.
- Modify `backend/internal/domain/systemadmin/service.go`
  - Add orchestration for solution package generation and schema change request creation.
- Modify `backend/internal/domain/systemadmin/model.go`
  - Add request/response structs for solution creation steps.
- Modify `backend/internal/domain/industry/model.go`
  - Add metadata shape for database assets, business functions, process loops, UI metadata, API metadata, assistant metadata.
- Modify `backend/internal/domain/industry/service.go`
  - Validate industry solution packages contain the required ERP reconstruction assets.
- Modify `backend/internal/domain/assistant/context.go`
  - Add ERP business object context queries.
- Modify `backend/internal/domain/assistant/proposal_applicator.go`
  - Allow approved proposals to target ERP records or ERP actions.

### Frontend

- Modify `frontend/src/lib/api.ts`
  - Add typed ERP record, ERP action, ERP process, and industry solution APIs.
- Modify `frontend/src/lib/operations.ts`
  - Add ERP action operations for all migrated business flows.
  - Ensure old semantic business paths are not exported.
- Modify `frontend/src/lib/i18n.tsx`
  - Add bilingual keys for all forms, actions, statuses, validation messages, and industry solution flow labels.
- Modify `frontend/src/app/page.tsx`
  - Restore business workspaces as main screens and keep generic ERP table workspace as admin/developer tool.
- Create or replace `frontend/src/app/project-lifecycle-workspace.tsx`
  - Use `MREQ`, `MPRJ`, `MDLN`, `MCST`, `MFDB` ERP APIs and actions.
- Create or replace `frontend/src/app/procurement-workspace.tsx`
  - Use `MPOR`, `MPDN`, `MPCH` ERP APIs and actions.
- Create or replace `frontend/src/app/sales-workspace.tsx`
  - Use `MQUT`, `MRDR`, `MDLN`, `MINV`, `MRCT` ERP APIs and actions.
- Create or replace `frontend/src/app/inventory-workspace.tsx`
  - Use `MCRD`, `MITM`, `MWHS`, `MITW`, `MIGN`, `MIGE` ERP APIs and actions.
- Create or replace `frontend/src/app/finance-workspace.tsx`
  - Use `MACT`, `MPRC`, `MJDT`, `MINV`, `MPCH`, `MRCT` ERP APIs and actions.
- Modify `frontend/src/app/system-admin-workspace.tsx`
  - Add or extend industry solution standard flow UI.
- Keep `frontend/src/app/erp-workspace.tsx`
  - Limit it to generic admin/developer table operations.

### Migrations

- Modify `migrations/000_erp_code_baseline.sql`
  - Add any missing code tables needed by the action layer.
  - Add SaaS platform tables/columns for industry solution workflow metadata if not already present.

---

## Task 1: Lock Current Failure With Tests

**Files:**
- Modify: `backend/internal/domain/erp/service_test.go`
- Create: `backend/internal/domain/erp/actions_test.go`
- Modify: `frontend/src/lib/operations.ts`

- [ ] **Step 1: Add failing backend test for missing ERP actions**

Add this test to `backend/internal/domain/erp/actions_test.go`:

```go
package erp

import "testing"

func TestDefaultActionRegistryIncludesBusinessActions(t *testing.T) {
	registry := DefaultActionRegistry()
	cases := []struct {
		table  string
		action string
	}{
		{"MREQ", "approve"},
		{"MREQ", "convert-to-project"},
		{"MPOR", "submit"},
		{"MPOR", "approve"},
		{"MPDN", "post"},
		{"MRDR", "confirm"},
		{"MRDR", "approve"},
		{"MDLN", "post"},
		{"MINV", "post"},
		{"MRCT", "allocate"},
		{"MIGN", "post"},
		{"MIGE", "post"},
		{"MJDT", "post"},
	}
	for _, tc := range cases {
		if _, ok := registry.Lookup(tc.table, tc.action); !ok {
			t.Fatalf("registry missing %s/%s", tc.table, tc.action)
		}
	}
}
```

- [ ] **Step 2: Run the failing test**

Run:

```powershell
cd backend
go test ./internal/domain/erp -run TestDefaultActionRegistryIncludesBusinessActions
```

Expected: FAIL because `DefaultActionRegistry` does not exist.

- [ ] **Step 3: Add failing frontend metadata test by static check**

Add a local verification script later in Task 10, but for this task run:

```powershell
rg -n "erp-.+-action|actions/" frontend\src\lib\operations.ts
```

Expected: no matches or incomplete matches, proving API workbench action metadata is missing.

- [ ] **Step 4: Commit the failing tests**

Run:

```powershell
git add backend/internal/domain/erp/actions_test.go
git commit -m "test: capture missing ERP business actions"
```

Expected: commit succeeds with only the test file staged.

---

## Task 1A: ERP Module/Submodule/Business Document Hierarchy

**Files:**
- Modify: `backend/internal/domain/erp/model.go`
- Modify: `backend/internal/domain/erp/catalog.go`
- Create: `backend/internal/domain/erp/hierarchy_test.go`
- Modify: `backend/internal/domain/erp/handler_test.go`

- [ ] **Step 1: Write failing catalog hierarchy test**

Create `backend/internal/domain/erp/hierarchy_test.go`:

```go
package erp

import "testing"

func TestDefaultCatalogIncludesERPBusinessHierarchy(t *testing.T) {
	catalog := DefaultCatalog()
	cases := []struct {
		module    string
		submodule string
		document  string
		table     string
		child     string
		action    string
	}{
		{"Project", "Requirements", "Requirement", "MREQ", "REQ1", "approve"},
		{"Project", "Projects", "Project", "MPRJ", "APRJ", "refresh-cost"},
		{"Purchasing", "Purchase Orders", "Purchase Order", "MPOR", "POR1", "approve"},
		{"Purchasing", "Goods Receipt PO", "Goods Receipt PO", "MPDN", "PDN1", "post"},
		{"Sales", "Sales Orders", "Sales Order", "MRDR", "RDR1", "confirm"},
		{"Sales", "A/R Invoices", "A/R Invoice", "MINV", "INV1", "post"},
		{"Inventory", "Goods Issues", "Goods Issue", "MIGE", "IGE1", "post"},
		{"Finance", "Journal Entries", "Journal Entry", "MJDT", "JDT1", "post"},
		{"Master Data", "Items", "Item", "MITM", "ITM1", ""},
	}

	for _, tc := range cases {
		if !catalog.HasBusinessDocument(tc.module, tc.submodule, tc.document, tc.table, tc.child, tc.action) {
			t.Fatalf("catalog missing hierarchy %#v", tc)
		}
	}
}
```

- [ ] **Step 2: Run test to verify RED**

```powershell
cd backend
go test ./internal/domain/erp -run TestDefaultCatalogIncludesERPBusinessHierarchy
```

Expected: FAIL because hierarchy types and `HasBusinessDocument` do not exist.

- [ ] **Step 3: Add hierarchy model types**

Add `ModuleDefinition`, `SubmoduleDefinition`, and `BusinessDocumentType` to
`backend/internal/domain/erp/model.go`, and add `Modules []ModuleDefinition`
to `Catalog`.

- [ ] **Step 4: Build default ERP hierarchy**

Add `defaultModules()` in `backend/internal/domain/erp/catalog.go` with Project,
Master Data, Purchasing, Sales, Inventory, Finance, and Platform modules. Each
document must include table code, child table codes, and available action names.

- [ ] **Step 5: Implement hierarchy lookup helper**

Implement:

```go
func (c Catalog) HasBusinessDocument(moduleName, submoduleName, documentName, tableCode, childCode, actionName string) bool
```

The helper must walk the full hierarchy and verify optional child/action values
when they are non-empty.

- [ ] **Step 6: Assert `/erp/catalog` exposes modules**

Extend `TestHandlerReturnsCatalog` to require `"modules"` and `"Purchase Order"`
in the response body.

- [ ] **Step 7: Verify GREEN**

```powershell
cd backend
go test ./internal/domain/erp -run "Hierarchy|Catalog|Action"
```

Expected: PASS.

---

## Task 2: ERP Action Registry And Handler Foundation

**Files:**
- Create: `backend/internal/domain/erp/actions.go`
- Modify: `backend/internal/domain/erp/model.go`
- Modify: `backend/internal/domain/erp/service.go`
- Modify: `backend/internal/domain/erp/handler.go`
- Test: `backend/internal/domain/erp/actions_test.go`
- Test: `backend/internal/domain/erp/handler_test.go`

- [ ] **Step 1: Define action metadata and request/response types**

Add to `backend/internal/domain/erp/model.go`:

```go
type ActionDefinition struct {
	TableCode   string   `json:"table_code"`
	Action      string   `json:"action"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	NextTables  []string `json:"next_tables,omitempty"`
}

type ActionInput struct {
	Data map[string]any `json:"data"`
}

type ActionResult struct {
	TableCode        string         `json:"table_code"`
	Key              string         `json:"key"`
	Action           string         `json:"action"`
	Status           string         `json:"status"`
	Record           *Record        `json:"record,omitempty"`
	GeneratedRecords []Record       `json:"generated_records,omitempty"`
	Effects          map[string]any `json:"effects,omitempty"`
}
```

- [ ] **Step 2: Create action registry**

Create `backend/internal/domain/erp/actions.go`:

```go
package erp

type ActionRegistry struct {
	actions map[string]ActionDefinition
}

func DefaultActionRegistry() ActionRegistry {
	defs := []ActionDefinition{
		{TableCode: "MREQ", Action: "analyze", Label: "Analyze requirement", Description: "Analyze requirement and prepare approval"},
		{TableCode: "MREQ", Action: "approve", Label: "Approve requirement", Description: "Approve requirement for conversion"},
		{TableCode: "MREQ", Action: "convert-to-project", Label: "Convert requirement to project", Description: "Create or link project from approved requirement", NextTables: []string{"MPRJ"}},
		{TableCode: "MPRJ", Action: "add-member", Label: "Add project member", Description: "Append project member row"},
		{TableCode: "MPRJ", Action: "add-deliverable", Label: "Add deliverable", Description: "Create delivery document for project", NextTables: []string{"MDLN"}},
		{TableCode: "MPRJ", Action: "refresh-cost", Label: "Refresh project cost", Description: "Aggregate ERP cost effects into project cost records", NextTables: []string{"MCST"}},
		{TableCode: "MPRJ", Action: "close-feedback", Label: "Close feedback", Description: "Close project feedback loop", NextTables: []string{"MFDB"}},
		{TableCode: "MPOR", Action: "submit", Label: "Submit purchase order", Description: "Submit purchase order for approval"},
		{TableCode: "MPOR", Action: "approve", Label: "Approve purchase order", Description: "Approve purchase order"},
		{TableCode: "MPDN", Action: "post", Label: "Post goods receipt PO", Description: "Post goods receipt and inventory receipt", NextTables: []string{"MIGN", "MPCH"}},
		{TableCode: "MRDR", Action: "confirm", Label: "Confirm sales order", Description: "Confirm sales order"},
		{TableCode: "MRDR", Action: "approve", Label: "Approve sales order", Description: "Approve sales order"},
		{TableCode: "MDLN", Action: "post", Label: "Post delivery", Description: "Post delivery and inventory issue", NextTables: []string{"MIGE", "MINV"}},
		{TableCode: "MINV", Action: "post", Label: "Post A/R invoice", Description: "Post receivable invoice", NextTables: []string{"MJDT"}},
		{TableCode: "MRCT", Action: "allocate", Label: "Allocate incoming payment", Description: "Allocate payment to receivable invoice"},
		{TableCode: "MIGN", Action: "post", Label: "Post goods receipt", Description: "Increase inventory balance"},
		{TableCode: "MIGE", Action: "post", Label: "Post goods issue", Description: "Decrease inventory balance"},
		{TableCode: "MJDT", Action: "post", Label: "Post journal entry", Description: "Post journal entry"},
	}
	registry := ActionRegistry{actions: map[string]ActionDefinition{}}
	for _, def := range defs {
		registry.actions[actionKey(def.TableCode, def.Action)] = def
	}
	return registry
}

func (r ActionRegistry) Lookup(tableCode string, action string) (ActionDefinition, bool) {
	def, ok := r.actions[actionKey(tableCode, action)]
	return def, ok
}

func (r ActionRegistry) List() []ActionDefinition {
	result := make([]ActionDefinition, 0, len(r.actions))
	for _, def := range r.actions {
		result = append(result, def)
	}
	return result
}

func actionKey(tableCode string, action string) string {
	return tableCode + ":" + action
}
```

- [ ] **Step 3: Add service dispatch skeleton**

Modify `Service` in `backend/internal/domain/erp/service.go` to include registry and action dispatch:

```go
type Service struct {
	repo     Repository
	catalog  Catalog
	actions  ActionRegistry
}

func NewService(repo Repository, catalog Catalog) *Service {
	if catalog.byCode == nil {
		catalog = DefaultCatalog()
	}
	return &Service{repo: repo, catalog: catalog, actions: DefaultActionRegistry()}
}

func (s *Service) Actions(ctx context.Context) []ActionDefinition {
	return s.actions.List()
}

func (s *Service) RunAction(ctx context.Context, tableCode string, key string, action string, input ActionInput) (*ActionResult, error) {
	if _, err := s.table(tableCode); err != nil {
		return nil, err
	}
	def, ok := s.actions.Lookup(tableCode, action)
	if !ok {
		return nil, fmt.Errorf("%w: unknown action %s for %s", ErrValidation, action, tableCode)
	}
	return &ActionResult{TableCode: tableCode, Key: key, Action: def.Action, Status: "accepted", Effects: map[string]any{"definition": def.Label}}, nil
}
```

- [ ] **Step 4: Add action routes**

Modify `backend/internal/domain/erp/handler.go`:

```go
r.Get("/erp/actions", h.listActions)
r.Post("/erp/{tableCode}/{key}/actions/{action}", h.runAction)
```

Implement handlers:

```go
func (h *Handler) listActions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"actions": h.service.Actions(r.Context())})
}

func (h *Handler) runAction(w http.ResponseWriter, r *http.Request) {
	var input ActionInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil && err != io.EOF {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := h.service.RunAction(
		r.Context(),
		chi.URLParam(r, "tableCode"),
		chi.URLParam(r, "key"),
		chi.URLParam(r, "action"),
		input,
	)
	if err != nil {
		writeERPError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
```

- [ ] **Step 5: Run registry and handler tests**

Run:

```powershell
cd backend
go test ./internal/domain/erp
```

Expected: PASS for registry and existing CRUD tests.

- [ ] **Step 6: Commit**

Run:

```powershell
git add backend/internal/domain/erp
git commit -m "feat: add ERP action registry"
```

---

## Task 3: Repository Transaction Helpers For Real Effects

**Files:**
- Modify: `backend/internal/domain/erp/repository.go`
- Modify: `backend/internal/domain/erp/service.go`
- Create: `backend/internal/domain/erp/repository_actions_test.go`

- [ ] **Step 1: Extend repository interface**

Add methods to `Repository` in `backend/internal/domain/erp/service.go`:

```go
RunInTx(ctx context.Context, fn func(ctx context.Context, tx Repository) error) error
AppendChildRecord(ctx context.Context, parent TableDefinition, child ChildTableDefinition, parentKey string, input RecordInput) (*Record, error)
MergeRecordPayload(ctx context.Context, table TableDefinition, key string, payload map[string]any) (*Record, error)
```

- [ ] **Step 2: Add failing test for transactional action effect**

Create `backend/internal/domain/erp/repository_actions_test.go` with a fake repository that records transaction calls:

```go
package erp

import (
	"context"
	"testing"
)

func TestRepositoryActionHelpersAreRequiredForActions(t *testing.T) {
	repo := &actionFakeRepository{}
	service := NewService(repo, DefaultCatalog())
	_, err := service.RunAction(context.Background(), "MREQ", "REQ-1", "approve", ActionInput{Data: map[string]any{"approver": "u1"}})
	if err != nil {
		t.Fatalf("RunAction returned error: %v", err)
	}
	if !repo.txCalled {
		t.Fatal("RunAction did not use transaction helper")
	}
	if repo.lastMergeTable != "MREQ" {
		t.Fatalf("last merge table = %q, want MREQ", repo.lastMergeTable)
	}
}
```

- [ ] **Step 3: Implement `RunInTx` for PostgresRepository**

Add to `backend/internal/domain/erp/repository.go`:

```go
func (r *PostgresRepository) RunInTx(ctx context.Context, fn func(ctx context.Context, tx Repository) error) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin ERP tx: %w", err)
	}
	defer tx.Rollback(ctx)
	txRepo := &TxRepository{tx: tx}
	if err := fn(ctx, txRepo); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit ERP tx: %w", err)
	}
	return nil
}
```

Create `TxRepository` in the same file and implement the same repository methods using `pgx.Tx`.

- [ ] **Step 4: Implement merge and append helpers**

Add helper behavior:

```go
func (r *PostgresRepository) MergeRecordPayload(ctx context.Context, table TableDefinition, key string, payload map[string]any) (*Record, error) {
	return mergeRecordPayload(ctx, r.db, table, key, payload)
}

func (r *PostgresRepository) AppendChildRecord(ctx context.Context, parent TableDefinition, child ChildTableDefinition, parentKey string, input RecordInput) (*Record, error) {
	return r.CreateChildRecord(ctx, parent, child, parentKey, input)
}
```

Use shared helper functions so `TxRepository` can call them.

- [ ] **Step 5: Run tests**

Run:

```powershell
cd backend
go test ./internal/domain/erp
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add backend/internal/domain/erp
git commit -m "feat: add ERP action transaction helpers"
```

---

## Task 4: Project And Requirement ERP Actions

**Files:**
- Create: `backend/internal/domain/erp/project_actions.go`
- Modify: `backend/internal/domain/erp/service.go`
- Modify: `backend/internal/domain/erp/catalog.go`
- Create: `backend/internal/domain/erp/project_actions_test.go`

- [ ] **Step 1: Write failing test for requirement approval**

Create test:

```go
func TestApproveRequirementUpdatesStatus(t *testing.T) {
	repo := newBusinessFakeRepository()
	repo.seed("MREQ", "REQ-1", map[string]any{"ReqCode": "REQ-1", "Status": "analyzed"})
	service := NewService(repo, DefaultCatalog())
	result, err := service.RunAction(context.Background(), "MREQ", "REQ-1", "approve", ActionInput{Data: map[string]any{"approver": "u1"}})
	if err != nil {
		t.Fatalf("approve returned error: %v", err)
	}
	if result.Record.Data["Status"] != "approved" {
		t.Fatalf("status = %v, want approved", result.Record.Data["Status"])
	}
}
```

- [ ] **Step 2: Write failing test for conversion**

Add:

```go
func TestConvertRequirementCreatesProject(t *testing.T) {
	repo := newBusinessFakeRepository()
	repo.seed("MREQ", "REQ-1", map[string]any{"ReqCode": "REQ-1", "Name": "Portal", "Status": "approved"})
	service := NewService(repo, DefaultCatalog())
	result, err := service.RunAction(context.Background(), "MREQ", "REQ-1", "convert-to-project", ActionInput{Data: map[string]any{"PrjCode": "PRJ-1"}})
	if err != nil {
		t.Fatalf("convert returned error: %v", err)
	}
	if len(result.GeneratedRecords) != 1 || result.GeneratedRecords[0].TableCode != "MPRJ" {
		t.Fatalf("generated = %#v, want one MPRJ record", result.GeneratedRecords)
	}
}
```

- [ ] **Step 3: Implement project action dispatcher**

Create `backend/internal/domain/erp/project_actions.go`:

```go
package erp

import (
	"context"
	"fmt"
)

func (s *Service) runProjectAction(ctx context.Context, tableCode string, key string, action string, input ActionInput) (*ActionResult, error) {
	switch tableCode + ":" + action {
	case "MREQ:approve":
		return s.approveRequirement(ctx, key, input)
	case "MREQ:convert-to-project":
		return s.convertRequirementToProject(ctx, key, input)
	case "MPRJ:refresh-cost":
		return s.refreshProjectCost(ctx, key, input)
	case "MPRJ:close-feedback":
		return s.closeProjectFeedback(ctx, key, input)
	default:
		return nil, fmt.Errorf("%w: unsupported project action %s for %s", ErrValidation, action, tableCode)
	}
}
```

- [ ] **Step 4: Implement approval and conversion**

Implement:

```go
func (s *Service) approveRequirement(ctx context.Context, key string, input ActionInput) (*ActionResult, error) {
	table, _ := s.table("MREQ")
	record, err := s.repo.MergeRecordPayload(ctx, table, key, map[string]any{
		"Status": "approved",
		"ApprovedBy": input.Data["approver"],
	})
	if err != nil {
		return nil, err
	}
	return &ActionResult{TableCode: "MREQ", Key: key, Action: "approve", Status: "approved", Record: record}, nil
}
```

Implement conversion by creating `MPRJ` with `PrjCode`, `Name`, `Active`, and `Payload.requirement_code`.

- [ ] **Step 5: Wire dispatcher in `RunAction`**

In `service.go`, dispatch project tables:

```go
if tableCode == "MREQ" || tableCode == "MPRJ" {
	return s.runProjectAction(ctx, tableCode, key, action, input)
}
```

- [ ] **Step 6: Run tests**

```powershell
cd backend
go test ./internal/domain/erp -run "Requirement|Project|Action"
```

Expected: PASS.

- [ ] **Step 7: Commit**

```powershell
git add backend/internal/domain/erp
git commit -m "feat: migrate project actions to ERP"
```

---

## Task 5: Procurement And Inventory Posting Actions

**Files:**
- Create: `backend/internal/domain/erp/procurement_actions.go`
- Create: `backend/internal/domain/erp/inventory_actions.go`
- Create: `backend/internal/domain/erp/procurement_actions_test.go`
- Create: `backend/internal/domain/erp/inventory_actions_test.go`

- [ ] **Step 1: Write purchase order state tests**

Test `MPOR submit` changes `DocStatus` to `S`, and `approve` changes `WddStatus` to `A`:

```go
func TestPurchaseOrderSubmitAndApprove(t *testing.T) {
	repo := newBusinessFakeRepository()
	repo.seed("MPOR", "PO-1", map[string]any{"DocEntry": "PO-1", "DocStatus": "O", "WddStatus": "W"})
	service := NewService(repo, DefaultCatalog())
	submitted, err := service.RunAction(context.Background(), "MPOR", "PO-1", "submit", ActionInput{})
	if err != nil {
		t.Fatalf("submit error: %v", err)
	}
	if submitted.Record.Data["DocStatus"] != "S" {
		t.Fatalf("DocStatus = %v, want S", submitted.Record.Data["DocStatus"])
	}
	approved, err := service.RunAction(context.Background(), "MPOR", "PO-1", "approve", ActionInput{})
	if err != nil {
		t.Fatalf("approve error: %v", err)
	}
	if approved.Record.Data["WddStatus"] != "A" {
		t.Fatalf("WddStatus = %v, want A", approved.Record.Data["WddStatus"])
	}
}
```

- [ ] **Step 2: Write goods receipt posting test**

Test `MPDN post` generates inventory receipt and payable invoice:

```go
func TestGoodsReceiptPostGeneratesInventoryAndPayable(t *testing.T) {
	repo := newBusinessFakeRepository()
	repo.seed("MPDN", "GR-1", map[string]any{"DocEntry": "GR-1", "DocStatus": "O", "CardCode": "S-1"})
	repo.seedChild("MPDN", "GR-1", "PDN1", map[string]any{"LineNum": "1", "Payload": map[string]any{"ItemCode": "I-1", "WhsCode": "W-1", "Quantity": 2, "Price": 10}})
	service := NewService(repo, DefaultCatalog())
	result, err := service.RunAction(context.Background(), "MPDN", "GR-1", "post", ActionInput{})
	if err != nil {
		t.Fatalf("post error: %v", err)
	}
	assertGeneratedTable(t, result, "MIGN")
	assertGeneratedTable(t, result, "MPCH")
	if repo.balance("I-1", "W-1") != 2 {
		t.Fatalf("balance = %v, want 2", repo.balance("I-1", "W-1"))
	}
}
```

- [ ] **Step 3: Implement procurement actions**

`MPOR:submit` updates `DocStatus=S`.
`MPOR:approve` updates `WddStatus=A`.
`MPDN:post` requires `DocStatus=O`, reads `PDN1`, creates `MIGN` and `MPCH`, updates `MITW`, then sets `MPDN.DocStatus=C`.

- [ ] **Step 4: Implement inventory balance mutation**

Use `MITW` key format `ItemCode|WhsCode` in `Payload` if the table has only `ItemCode` as primary key. Store `OnHand`, `Committed`, and `Available` in payload.

- [ ] **Step 5: Run tests**

```powershell
cd backend
go test ./internal/domain/erp -run "Purchase|Goods|Inventory"
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add backend/internal/domain/erp
git commit -m "feat: migrate procurement and inventory posting"
```

---

## Task 6: Sales, Delivery, Invoice, And Payment Actions

**Files:**
- Create: `backend/internal/domain/erp/sales_actions.go`
- Create: `backend/internal/domain/erp/finance_actions.go`
- Create: `backend/internal/domain/erp/sales_actions_test.go`
- Create: `backend/internal/domain/erp/finance_actions_test.go`

- [ ] **Step 1: Write sales order action test**

```go
func TestSalesOrderConfirmAndApprove(t *testing.T) {
	repo := newBusinessFakeRepository()
	repo.seed("MRDR", "SO-1", map[string]any{"DocEntry": "SO-1", "DocStatus": "O", "Confirmed": "N", "WddStatus": "W"})
	service := NewService(repo, DefaultCatalog())
	confirmed, err := service.RunAction(context.Background(), "MRDR", "SO-1", "confirm", ActionInput{})
	if err != nil {
		t.Fatalf("confirm error: %v", err)
	}
	if confirmed.Record.Data["Confirmed"] != "Y" {
		t.Fatalf("Confirmed = %v, want Y", confirmed.Record.Data["Confirmed"])
	}
	approved, err := service.RunAction(context.Background(), "MRDR", "SO-1", "approve", ActionInput{})
	if err != nil {
		t.Fatalf("approve error: %v", err)
	}
	if approved.Record.Data["WddStatus"] != "A" {
		t.Fatalf("WddStatus = %v, want A", approved.Record.Data["WddStatus"])
	}
}
```

- [ ] **Step 2: Write delivery posting test**

```go
func TestDeliveryPostGeneratesGoodsIssueAndInvoice(t *testing.T) {
	repo := newBusinessFakeRepository()
	repo.setBalance("I-1", "W-1", 5)
	repo.seed("MDLN", "DLV-1", map[string]any{"DocEntry": "DLV-1", "DocStatus": "O", "CardCode": "C-1"})
	repo.seedChild("MDLN", "DLV-1", "DLN1", map[string]any{"LineNum": "1", "Payload": map[string]any{"ItemCode": "I-1", "WhsCode": "W-1", "Quantity": 2, "Price": 15}})
	service := NewService(repo, DefaultCatalog())
	result, err := service.RunAction(context.Background(), "MDLN", "DLV-1", "post", ActionInput{})
	if err != nil {
		t.Fatalf("post error: %v", err)
	}
	assertGeneratedTable(t, result, "MIGE")
	assertGeneratedTable(t, result, "MINV")
	if repo.balance("I-1", "W-1") != 3 {
		t.Fatalf("balance = %v, want 3", repo.balance("I-1", "W-1"))
	}
}
```

- [ ] **Step 3: Write payment allocation test**

```go
func TestIncomingPaymentAllocateMarksInvoicePaid(t *testing.T) {
	repo := newBusinessFakeRepository()
	repo.seed("MINV", "INV-1", map[string]any{"DocEntry": "INV-1", "DocTotal": 100, "PaidToDate": 0, "DocStatus": "O"})
	repo.seed("MRCT", "PAY-1", map[string]any{"DocEntry": "PAY-1", "DocTotal": 100, "OpenBal": 100})
	service := NewService(repo, DefaultCatalog())
	result, err := service.RunAction(context.Background(), "MRCT", "PAY-1", "allocate", ActionInput{Data: map[string]any{"TargetTable": "MINV", "TargetKey": "INV-1", "Amount": 100}})
	if err != nil {
		t.Fatalf("allocate error: %v", err)
	}
	if result.Effects["allocated_amount"] != float64(100) {
		t.Fatalf("effects = %#v, want allocated_amount 100", result.Effects)
	}
}
```

- [ ] **Step 4: Implement sales and finance actions**

Implement:

- `MRDR:confirm` sets `Confirmed=Y`.
- `MRDR:approve` sets `WddStatus=A`.
- `MDLN:post` creates `MIGE`, decrements `MITW`, creates `MINV`, closes delivery.
- `MINV:post` creates `MJDT` journal entry and keeps invoice open until paid.
- `MRCT:allocate` appends `RCT1`, updates `MINV.PaidToDate`, closes `MINV` if paid, updates `MRCT.OpenBal`.

- [ ] **Step 5: Run tests**

```powershell
cd backend
go test ./internal/domain/erp -run "Sales|Delivery|Payment|Invoice"
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add backend/internal/domain/erp
git commit -m "feat: migrate sales and finance actions"
```

---

## Task 7: End-To-End ERP Business Smoke

**Files:**
- Modify: `backend/cmd/smoke/main.go`
- Modify: `backend/cmd/smoke/main_test.go`
- Test: `backend/cmd/smoke/main_test.go`

- [ ] **Step 1: Rewrite smoke to prove real loop**

`runERPSmoke` must do these calls in this order:

1. Create `MCRD` customer and supplier.
2. Create `MITM` item.
3. Create `MWHS` warehouse.
4. Create `MREQ`, analyze, approve, convert to `MPRJ`.
5. Create `MPOR/POR1`, submit, approve.
6. Create `MPDN/PDN1`, post and verify `MIGN`, `MPCH`, and `MITW` balance.
7. Create `MRDR/RDR1`, confirm, approve.
8. Create `MDLN/DLN1`, post and verify `MIGE`, `MINV`, and balance decrease.
9. Create `MRCT`, allocate to `MINV`.
10. Run `MPRJ refresh-cost`.
11. Run `MPRJ close-feedback` and verify `MFDB`.

- [ ] **Step 2: Add smoke path expectation test**

In `backend/cmd/smoke/main_test.go`, assert action paths expecting `200 OK`:

```go
func TestERPActionPathsDoNotExpectCreated(t *testing.T) {
	paths := []string{
		"/erp/MREQ/REQ-1/actions/approve",
		"/erp/MREQ/REQ-1/actions/convert-to-project",
		"/erp/MPOR/PO-1/actions/submit",
		"/erp/MPOR/PO-1/actions/approve",
		"/erp/MPDN/GR-1/actions/post",
		"/erp/MRDR/SO-1/actions/confirm",
		"/erp/MDLN/DLV-1/actions/post",
		"/erp/MRCT/PAY-1/actions/allocate",
	}
	for _, path := range paths {
		if expectsCreated(path) {
			t.Fatalf("%s expects created, want ok action response", path)
		}
	}
}
```

- [ ] **Step 3: Run smoke tests**

```powershell
cd backend
go test ./cmd/smoke
```

Expected: PASS.

- [ ] **Step 4: Commit**

```powershell
git add backend/cmd/smoke
git commit -m "test: verify end-to-end ERP business loop"
```

---

## Task 8: API Operation Catalog And AI-Callable Metadata

**Files:**
- Modify: `frontend/src/lib/operations.ts`
- Modify: `frontend/src/lib/i18n.tsx`
- Modify: `backend/internal/domain/assistant/context.go`
- Modify: `backend/internal/domain/assistant/proposal_applicator.go`
- Create: `backend/internal/domain/assistant/erp_context_test.go`
- Create: `frontend/verify-erp-operations.mjs`
- Modify: `frontend/package.json`

- [ ] **Step 1: Add ERP action operations**

In `frontend/src/lib/operations.ts`, add operation entries for each required action. Example:

```ts
{
  id: 'erp-mpor-submit-action',
  domain: 'Procurement',
  title: 'operation.erp.MPOR.submit',
  method: 'POST',
  path: '/erp/MPOR/{DocEntry}/actions/submit',
  pathParams: [{ name: 'DocEntry', label: 'operation.erp.DocEntry', placeholder: '1001' }],
  operationKind: 'contextual',
  dangerLevel: 'medium',
  resultView: 'summary',
  assistantEligible: true,
}
```

Add equivalent entries for `MREQ`, `MPRJ`, `MPDN`, `MRDR`, `MDLN`, `MINV`, `MRCT`, `MIGN`, `MIGE`, and `MJDT`.

- [ ] **Step 2: Add verification script**

Create `frontend/verify-erp-operations.mjs`:

```js
import { readFileSync } from 'node:fs'

const source = readFileSync('src/lib/operations.ts', 'utf8')
const required = [
  '/erp/MREQ/{ReqCode}/actions/approve',
  '/erp/MREQ/{ReqCode}/actions/convert-to-project',
  '/erp/MPOR/{DocEntry}/actions/submit',
  '/erp/MPOR/{DocEntry}/actions/approve',
  '/erp/MPDN/{DocEntry}/actions/post',
  '/erp/MRDR/{DocEntry}/actions/confirm',
  '/erp/MRDR/{DocEntry}/actions/approve',
  '/erp/MDLN/{DocEntry}/actions/post',
  '/erp/MINV/{DocEntry}/actions/post',
  '/erp/MRCT/{DocEntry}/actions/allocate',
]

for (const path of required) {
  if (!source.includes(path)) {
    console.error(`missing ERP operation ${path}`)
    process.exit(1)
  }
}

const forbidden = [
  "path: '/projects'",
  "path: '/requirements'",
  "path: '/procurement/orders'",
  "path: '/sales/orders'",
  "path: '/inventory/items'",
  "path: '/finance/receivables'",
]

for (const path of forbidden) {
  if (source.includes(path) && !source.includes('legacyBusinessPathPrefixes')) {
    console.error(`exported legacy business path ${path}`)
    process.exit(1)
  }
}
```

- [ ] **Step 3: Add package script**

Modify `frontend/package.json`:

```json
"test:erp-operations": "node verify-erp-operations.mjs"
```

- [ ] **Step 4: Add assistant context test**

Create `backend/internal/domain/assistant/erp_context_test.go`:

```go
func TestERPContextTargetsIncludeBusinessObjects(t *testing.T) {
	targets := erpContextTargetsForCatalog([]string{"MREQ", "MPRJ", "MPOR", "MRDR", "MINV"})
	want := []string{"requirement", "project", "purchase_order", "sales_order", "ar_invoice"}
	for _, key := range want {
		if !containsContextTarget(targets, key) {
			t.Fatalf("missing ERP context target %s in %#v", key, targets)
		}
	}
}
```

- [ ] **Step 5: Implement assistant ERP context and proposal target support**

Add ERP target mapping in `assistant/context.go`:

```go
var erpContextTargetTypes = map[string]string{
	"MREQ": "requirement",
	"MPRJ": "project",
	"MPOR": "purchase_order",
	"MRDR": "sales_order",
	"MINV": "ar_invoice",
}
```

Allow proposal applicator targets with `moduleKey == "erp"` to resolve to ERP action or record application.

- [ ] **Step 6: Run verification**

```powershell
cd backend
go test ./internal/domain/assistant -run ERP
cd ..\frontend
npm run test:erp-operations
```

Expected: both exit 0.

- [ ] **Step 7: Commit**

```powershell
git add backend/internal/domain/assistant frontend/src/lib/operations.ts frontend/src/lib/i18n.tsx frontend/verify-erp-operations.mjs frontend/package.json
git commit -m "feat: expose ERP actions to API workbench and assistant"
```

---

## Task 9: Restore Typed Frontend Business Workspaces

**Files:**
- Modify: `frontend/src/lib/api.ts`
- Modify: `frontend/src/app/page.tsx`
- Modify: `frontend/src/app/project-lifecycle-workspace.tsx`
- Modify: `frontend/src/app/procurement-workspace.tsx`
- Modify: `frontend/src/app/sales-workspace.tsx`
- Modify: `frontend/src/app/inventory-workspace.tsx`
- Modify: `frontend/src/app/finance-workspace.tsx`
- Modify: `frontend/src/lib/i18n.tsx`

- [ ] **Step 1: Add typed ERP API helpers**

In `frontend/src/lib/api.ts`, add helpers:

```ts
export async function runERPAction<T>(
  token: string,
  tableCode: string,
  key: string,
  action: string,
  data: Record<string, unknown> = {},
): Promise<T> {
  return apiRequest<T>(`/erp/${encodeURIComponent(tableCode)}/${encodeURIComponent(key)}/actions/${encodeURIComponent(action)}`, {
    method: 'POST',
    token,
    body: { data },
  })
}

export async function listERPRecords<T>(token: string, tableCode: string, limit = 100): Promise<T[]> {
  const result = await apiRequest<{ records: Array<{ key: string; data: T }> }>(`/erp/${encodeURIComponent(tableCode)}?limit=${limit}`, { token })
  return result.records.map((record) => ({ ...record.data, key: record.key }))
}
```

- [ ] **Step 2: Restore project workspace on ERP endpoints**

Use:

- `MREQ` for requirements.
- `MPRJ` for projects.
- `MDLN` for deliverables.
- `MCST` for costs.
- `MFDB` for feedback.

Buttons must call:

- `MREQ analyze`
- `MREQ approve`
- `MREQ convert-to-project`
- `MPRJ refresh-cost`
- `MPRJ close-feedback`

- [ ] **Step 3: Restore procurement workspace**

Use:

- `MPOR/POR1`
- `MPDN/PDN1`
- `MPCH/PCH1`

Buttons must call:

- `MPOR submit`
- `MPOR approve`
- `MPDN post`

- [ ] **Step 4: Restore sales workspace**

Use:

- `MQUT/QUT1`
- `MRDR/RDR1`
- `MDLN/DLN1`
- `MINV/INV1`
- `MRCT/RCT1`

Buttons must call:

- `MRDR confirm`
- `MRDR approve`
- `MDLN post`
- `MINV post`
- `MRCT allocate`

- [ ] **Step 5: Restore inventory workspace**

Use:

- `MCRD`
- `MITM`
- `MWHS`
- `MITW`
- `MIGN`
- `MIGE`

Buttons must call:

- `MIGN post`
- `MIGE post`

- [ ] **Step 6: Restore finance workspace**

Use:

- `MACT`
- `MPRC`
- `MJDT/JDT1`
- `MINV`
- `MPCH`
- `MRCT`

Buttons must call:

- `MJDT post`
- `MINV post`
- `MRCT allocate`

- [ ] **Step 7: Keep generic ERP workspace out of business primary flow**

In `page.tsx`, business domains must import and render typed workspaces. Generic `ERPCodeWorkspace` should be reachable only from developer/admin surface.

- [ ] **Step 8: Verify frontend**

```powershell
cd frontend
npm run lint
npm run build
npm run test:erp-operations
```

Expected: all exit 0.

- [ ] **Step 9: Commit**

```powershell
git add frontend/src/lib/api.ts frontend/src/app frontend/src/lib/i18n.tsx
git commit -m "feat: restore ERP-backed business workspaces"
```

---

## Task 10: SaaS Industry Solution Standard Flow

**Files:**
- Modify: `backend/internal/domain/systemadmin/model.go`
- Modify: `backend/internal/domain/systemadmin/service.go`
- Modify: `backend/internal/domain/systemadmin/handler.go`
- Modify: `backend/internal/domain/industry/model.go`
- Modify: `backend/internal/domain/industry/service.go`
- Create: `backend/internal/domain/systemadmin/erp_solution_flow_test.go`
- Modify: `frontend/src/app/system-admin-workspace.tsx`
- Modify: `frontend/src/lib/api.ts`
- Modify: `frontend/src/lib/i18n.tsx`

- [ ] **Step 1: Add solution flow request/response models**

Add model:

```go
type ERPSolutionFlowRequest struct {
	IndustryKey       string              `json:"industry_key"`
	PackageKey        string              `json:"package_key"`
	Name              string              `json:"name"`
	EnabledModules    []string            `json:"enabled_modules"`
	DatabaseAssets    []ERPDatabaseAsset  `json:"database_assets"`
	BusinessFunctions []ERPFunctionAsset  `json:"business_functions"`
	ProcessLoops      []ERPProcessLoop     `json:"process_loops"`
	Permissions       []string            `json:"permissions"`
	APIOperations     []string            `json:"api_operations"`
	UIWorkspaces      []string            `json:"ui_workspaces"`
	AssistantTargets  []string            `json:"assistant_targets"`
}
```

- [ ] **Step 2: Add system admin test**

Create test:

```go
func TestERPSolutionFlowBuildsCompleteChangePackage(t *testing.T) {
	service := newSystemAdminServiceWithFakeRepositories()
	result, err := service.BuildERPSolutionFlow(context.Background(), ERPSolutionFlowRequest{
		IndustryKey: "professional_services",
		PackageKey: "erp_standard",
		Name: "ERP Standard",
		EnabledModules: []string{"project", "procurement", "inventory", "sales", "finance"},
	})
	if err != nil {
		t.Fatalf("BuildERPSolutionFlow error: %v", err)
	}
	required := []string{"database_assets", "business_functions", "process_loops", "permissions", "api_operations", "ui_workspaces", "assistant_targets"}
	for _, key := range required {
		if !result.SchemaPackageHas(key) {
			t.Fatalf("schema package missing %s", key)
		}
	}
}
```

- [ ] **Step 3: Implement backend flow**

Add endpoint:

```go
r.Post("/platform/admin/industry-solution-flows/erp-standard", h.createERPSolutionFlow)
```

The handler builds a schema change request payload using:

- ERP catalog table assets.
- ERP action registry function assets.
- End-to-end process loop definitions.
- Permission keys.
- API operation paths.
- UI workspace metadata.
- Assistant target metadata.

- [ ] **Step 4: Add frontend flow UI**

In `system-admin-workspace.tsx`, add an industry tab section with:

- industry package selector
- enabled modules checklist
- database assets preview
- business functions preview
- process loops preview
- API/UI/assistant metadata preview
- create change request button
- approval/apply status display

- [ ] **Step 5: Run tests**

```powershell
cd backend
go test ./internal/domain/systemadmin ./internal/domain/industry
cd ..\frontend
npm run lint
npm run build
```

Expected: all exit 0.

- [ ] **Step 6: Commit**

```powershell
git add backend/internal/domain/systemadmin backend/internal/domain/industry frontend/src/app/system-admin-workspace.tsx frontend/src/lib/api.ts frontend/src/lib/i18n.tsx
git commit -m "feat: add ERP industry solution standard flow"
```

---

## Task 11: Final Integration And Non-Table-Shell Verification

**Files:**
- Modify: `backend/internal/gateway/router_erp_test.go`
- Modify: `backend/internal/gateway/router_platform_migration_test.go`
- Modify: `backend/cmd/smoke/main.go`
- Create: `docs/operations/erp-business-verification.md`

- [ ] **Step 1: Add route surface tests**

Assert:

- `/api/v1/erp/actions` is mounted.
- `/api/v1/erp/{tableCode}/{key}/actions/{action}` is mounted.
- old semantic business tenant routes are not mounted.
- platform admin and assistant routes remain mounted.

- [ ] **Step 2: Add verification document**

Create `docs/operations/erp-business-verification.md` with:

```markdown
# ERP Business Verification

This checklist proves the implementation is not only ERP table CRUD.

1. Create requirement MREQ.
2. Analyze and approve MREQ.
3. Convert MREQ to MPRJ.
4. Create and approve MPOR.
5. Post MPDN and verify MIGN, MPCH, MITW.
6. Confirm and approve MRDR.
7. Post MDLN and verify MIGE, MINV, MITW.
8. Post MINV and verify MJDT.
9. Allocate MRCT and verify MINV closes.
10. Refresh MPRJ cost and verify MCST.
11. Close feedback and verify MFDB.
12. Open API workbench and verify ERP action APIs exist.
13. Open project/procurement/sales/inventory/finance workspaces and verify typed forms and action buttons.
14. Query assistant context targets and verify ERP objects exist.
15. Create SaaS industry solution change package and verify database, function, process, permission, API, UI, and assistant metadata.
```

- [ ] **Step 3: Run all backend checks**

```powershell
cd backend
go test ./...
go build ./cmd/server
```

Expected: both exit 0.

- [ ] **Step 4: Run all frontend checks**

```powershell
cd frontend
npm run lint
npm run build
npm run test:erp-operations
```

Expected: all exit 0.

- [ ] **Step 5: Run diff check**

```powershell
cd ..
git diff --check
```

Expected: no whitespace errors.

- [ ] **Step 6: Commit**

```powershell
git add backend frontend docs
git commit -m "test: verify ERP business migration end to end"
```

---

## Self-Review Checklist

- Spec requirement "old functions migrate, not remove": covered by Tasks 4-9.
- Spec requirement "SAP-style missing loops": covered by Tasks 5-7.
- Spec requirement "project management integrated": covered by Task 4 and Task 7.
- Spec requirement "AI assistant unchanged but integrated": covered by Task 8.
- Spec requirement "SaaS platform management standard reusable flow": covered by Task 10.
- Spec requirement "not table shell": covered by Task 7 and Task 11.
- Spec requirement "typed UI, not raw JSON": covered by Task 9.
- Spec requirement "old API not restored": covered by Task 8 and Task 11.

No task should be marked complete until its listed command exits 0 and the expected behavior is observed.
