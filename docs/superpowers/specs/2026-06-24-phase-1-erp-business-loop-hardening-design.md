# Phase 1 ERP Business Loop Hardening Design

## Summary

Phase 1 hardens the existing ERP code-table action layer into a reliable business execution kernel. The goal is not to add another set of semantic APIs or broader CRUD screens. The goal is to make current ERP actions observable, auditable, idempotent, transaction-safe, and suitable as the shared backend for the tenant workbench, Tool Runtime, Assistant Runtime, and later monitoring agents.

The current codebase already has strong ERP foundations:

- `backend/internal/domain/erp/business_actions.go` implements action behavior for requirement, project, purchasing, sales, inventory, and finance tables.
- `backend/internal/domain/erp/business_actions_test.go` covers many core status transitions and generated records.
- `backend/cmd/smoke/main.go` runs a broad ERP flow through HTTP.
- Tool Runtime now exposes `erp.action.execute`, so assistant-requested ERP actions can already enter the governed tool path.

The missing piece is a first-class action execution contract. Today, multi-record actions rely on implicit status fields, fixed generated keys, and direct errors. That is enough for a demo path, but not enough for a self-evolving company loop where failures must feed learning, human reviewers need provenance, and repeated tool calls must not duplicate inventory, invoices, costs, or feedback records.

## Goals

Phase 1 will produce a hardened ERP action layer with these properties:

- Every ERP action has explicit preconditions and a stable action result shape.
- Every action execution is recorded in an ERP action ledger.
- Multi-write actions run in a transaction boundary.
- Repeated calls use a clear idempotency key and return the existing execution result instead of repeating side effects.
- Generated records carry provenance linking them back to source table, source key, action, and action execution.
- Failures are structured enough for Tool Runtime, workbench timelines, and future monitoring signals.
- End-to-end smoke coverage proves the three business loops work against the real backend surface.

## Non-Goals

Phase 1 will not rebuild the tenant ERP workbench UI. Status-driven document views remain Phase 3 work.

Phase 1 will not expand the platform industry solution factory. Package publication, package diff UI, and schema verify screens remain Phase 2 work.

Phase 1 will not introduce a monitoring agent that generates improvement proposals. That remains Phase 5 work.

Phase 1 will not restore legacy semantic business API paths such as `/projects`, `/sales`, or `/procurement` as primary tenant interfaces. ERP table-code APIs remain the tenant business runtime surface.

Phase 1 will not let AI bypass approval. Assistant-initiated ERP actions still enter through Tool Runtime and governance policy.

## Design Principles

The ERP service remains the business-rule owner for table-code actions. Handlers continue to parse HTTP inputs and return service results. Repositories handle persistence and transaction mechanics. Tool Runtime calls the same ERP service method as human/API clients, so there is one action path.

Action hardening must be incremental and compatible with the existing ERP catalog. Existing tables such as `MREQ`, `MPRJ`, `MPOR`, `MPDN`, `MRDR`, `MDLN`, `MINV`, `MRCT`, `MIGN`, `MIGE`, `MJDT`, `MCST`, and `MFDB` remain the business records. The new ledger records execution facts; it does not replace business documents.

Generated records must be deterministic. If a receipt post creates `IGN-{DocEntry}` and `AP-{DocEntry}`, a retry must find those records or the existing action execution and report an idempotent success.

Failures must be business-readable. Returning only a Go error is insufficient for user timelines and monitoring loops. The service should still return typed errors, but the action result and ledger should carry machine-readable failure metadata.

## Action Contract

`ActionInput` should carry:

- `Data`: existing action-specific payload.
- `ActorID`: optional actor UUID for human or assistant initiated actions.
- `ActorType`: optional actor type such as `internal_human`, `external_human`, or `internal_agent`.
- `IdempotencyKey`: optional client key provided by Tool Runtime, API Workbench, or other clients.
- `Source`: optional source such as `tenant_api`, `toolruntime`, `assistant`, or `smoke`.
- `ToolExecutionID`: optional Tool Runtime execution reference.
- `AssistantSessionID`: optional assistant session reference.

`ActionResult` should carry:

- `TableCode`, `Key`, `Action`, and `Status`.
- `Record`: the primary record after the action.
- `GeneratedRecords`: records created or reused by the action.
- `Effects`: domain effects such as inventory delta, allocated amount, target invoice, or cost summary.
- `ExecutionID`: ERP action ledger ID.
- `IdempotencyKey`: the effective key used by the action.
- `PreconditionsChecked`: explicit checks and their pass/fail result.
- `Provenance`: source and correlation data for workbench timelines and monitoring signals.
- `FailureReason`: structured failure data when the action fails.

The HTTP handler should continue returning action results. Tool Runtime can continue wrapping the ERP result in `erp_action` without needing a new public API path.

## ERP Action Ledger

Add tenant-side ERP baseline tables in `migrations/001_erp_code_baseline.sql`:

- `MACT`: ERP Action Execution
- `ACT1`: ERP Action Generated Records

`MACT` records the action attempt:

- `ActionID`: primary key.
- `TableCode`: source ERP table code.
- `RecordKey`: source business key.
- `Action`: action name.
- `Status`: `running`, `completed`, `failed`, or `idempotent_replay`.
- `IdempotencyKey`: unique action key within table/key/action scope.
- `ActorID`, `ActorType`.
- `ToolExecutionID`.
- `AssistantSessionID`.
- `Source`.
- `FailureCode`, `FailureMessage`.
- `Payload`: JSONB request/result details.
- `StartedAt`, `CompletedAt`.

`ACT1` records generated or reused records:

- `ActionID`.
- `LineNum`.
- `GeneratedTableCode`.
- `GeneratedKey`.
- `RelationType`: `created`, `reused`, `updated`, or `linked`.
- `Payload`: JSONB provenance snapshot.

The staged baseline documentation must describe why these tables belong to the tenant ERP baseline: they represent tenant business execution history, not platform-level SaaS administration.

## Transaction Boundary

The ERP repository needs a transaction-aware boundary. The preferred shape is:

```go
type TransactionalRepository interface {
	RunInTx(context.Context, func(Repository) error) error
}
```

The PostgreSQL implementation should begin a transaction, create a repository bound to that transaction, run the callback, and commit or rollback.

The in-memory test repository can implement `RunInTx` with a copy-on-write snapshot so tests can prove partial writes are rolled back.

These actions must use a transaction:

- `MREQ.convert-to-project`
- `MPRJ.refresh-cost`
- `MPRJ.close-feedback`
- `MPDN.post`
- `MDLN.post`
- `MINV.post`
- `MRCT.allocate`
- `MIGN.post`
- `MIGE.post`
- `MJDT.post`

Single-field status actions can still run through the same action execution wrapper for ledger consistency, even when they do not need a multi-record transaction.

## Idempotency

The effective idempotency key is:

```text
erp:{table_code}:{record_key}:{action}:{client_or_default_key}
```

If the caller provides `ActionInput.IdempotencyKey`, use that as `client_or_default_key`.

If no key is provided, derive a stable default:

- `MREQ.convert-to-project`: `PRJ-{ReqCode}`
- `MPRJ.refresh-cost`: `COST-{PrjCode}`
- `MPRJ.close-feedback`: `FDB-{PrjCode}`
- `MPDN.post`: `IGN-{DocEntry}|AP-{DocEntry}`
- `MDLN.post`: `IGE-{DocEntry}|INV-{DocEntry}`
- `MINV.post`: `JE-{DocEntry}`
- `MRCT.allocate`: `TargetTable|TargetKey|Amount`
- `MIGN.post`: `MIGN-{DocEntry}`
- `MIGE.post`: `MIGE-{DocEntry}`
- `MJDT.post`: `MJDT-{TransId}`

If an existing completed execution with the same effective key exists, return an action result with status `idempotent_replay` and the prior generated records. Do not write duplicate inventory balances, invoices, costs, journals, or feedback records.

If an existing failed execution with the same key exists, allow retry only when the new request includes `retry_failed: true` in `Data`. This keeps accidental repeated calls from hiding persistent validation failures.

## Preconditions

Preconditions should be explicit functions instead of ad hoc inline checks. Each action should report which checks were evaluated.

Requirement to project:

- `MREQ.analyze`: requirement must exist and not be converted.
- `MREQ.approve`: status must be `analyzed`.
- `MREQ.convert-to-project`: status must be `approved`; target project key must be available or already linked to the same requirement.
- `MPRJ.refresh-cost`: project must exist and be active.
- `MPRJ.close-feedback`: project must have a refreshed cost reference before closing feedback.

Source-to-pay:

- `MPOR.submit`: purchase order must be open.
- `MPOR.approve`: purchase order must be submitted.
- `MPDN.post`: goods receipt PO must be approved and have at least one line with positive `ItemCode`, `WhsCode`, `Quantity`, and non-negative price.
- Inventory receipt must not create duplicate `MIGN` or payable records on retry.

Order-to-cash:

- `MRDR.confirm`: sales order must be open.
- `MRDR.approve`: sales order must be confirmed.
- `MDLN.post`: delivery must be approved and have at least one shippable line.
- Inventory issue must reject insufficient stock before creating invoice or issue documents.
- `MINV.post`: invoice must be open and not already journal-posted.
- `MRCT.allocate`: target invoice must exist, amount must be positive, amount must not exceed payment open balance, and allocation must not overpay the invoice.

Inventory and finance:

- `MIGN.post`: document must not already be posted and lines must be valid.
- `MIGE.post`: document must not already be posted and stock must be sufficient.
- `MJDT.post`: journal entry must not already be posted.

## Provenance

Every generated record should include a `Payload.provenance` object:

```json
{
  "source_table_code": "MDLN",
  "source_key": "DLV-1",
  "source_action": "post",
  "action_execution_id": "uuid",
  "idempotency_key": "erp:MDLN:DLV-1:post:IGE-DLV-1|INV-DLV-1",
  "created_by_actor_type": "internal_human",
  "tool_execution_id": "uuid-or-empty",
  "assistant_session_id": "uuid-or-empty"
}
```

The action ledger is the authoritative execution history. Generated-record provenance is a denormalized reference that lets document views, cost refresh, and monitoring jobs explain where downstream records came from without joining every ledger row.

## Business Loops

### Requirement -> Project -> Cost -> Feedback

The flow starts with `MREQ`, moves to `MPRJ`, then generates `MCST` and `MFDB`.

`MREQ.convert-to-project` should create or reuse a project linked to the requirement. `MPRJ.refresh-cost` should aggregate known project cost inputs and record a cost snapshot in `MCST`. `MPRJ.close-feedback` should generate `MFDB` after cost refresh and mark the project feedback state.

This loop becomes the backbone for connecting Meta Resource / PDCA profiles to ERP records in later phases.

### Source-to-Pay

The flow starts with `MPOR`, posts `MPDN`, generates `MIGN` and `MPCH`, and updates inventory.

The key reliability rule is that receipt posting is all-or-nothing. If inventory adjustment or payable creation fails, the receipt must remain unposted and no partial generated documents should remain.

### Order-to-Cash

The flow starts with `MRDR`, posts `MDLN`, generates `MIGE` and `MINV`, posts `MINV` to `MJDT`, and allocates `MRCT`.

The key reliability rule is that delivery posting must check stock before producing invoice side effects. Payment allocation must keep payment and invoice balances consistent.

## Error Handling

Validation failures should return `ErrValidation` with a clear message and should also create a failed `MACT` ledger entry when the action reached execution evaluation.

Unsupported actions should continue returning validation errors before ledger creation, because no known business action was attempted.

Persistence failures should roll back the transaction and mark the action execution failed when possible. If the ledger itself cannot be written, return the repository error and let observability capture the failure outside ERP.

Idempotent replays should not be treated as errors. They should return a successful result with status `idempotent_replay`, the original execution ID, and the known generated records.

## Integration With Tool Runtime and Assistant Runtime

Tool Runtime remains the only AI execution entry point. `erp.action.execute` should pass actor, source, idempotency, tool execution, and assistant session metadata into `ActionInput`.

Assistant proposals that target ERP actions should keep payload fields:

- `table_code`
- `key`
- `action`
- `data`

High-risk execution still requires Tool Runtime approval. Human-confirmed assistant proposals can record proposal confirmation metadata, but actual ERP side effects must go through the governed ERP action path when risk is high.

## Migration Governance

Because `MACT` and `ACT1` are new tenant ERP tables, Phase 1 must update:

- `migrations/001_erp_code_baseline.sql`
- `migrations/BASELINE_RESTRUCTURE.md`

Fresh database verification must apply staged migrations in order and confirm there are no invalid constraints.

No schema-only change should be left only in Go structs or informal documentation.

## Testing Strategy

Backend tests should focus on behavior rather than implementation details.

ERP unit tests:

- Preconditions reject illegal state transitions.
- Failed multi-write actions roll back all generated side effects.
- Idempotent retry returns existing generated records.
- Generated records include provenance.
- `MRCT.allocate` prevents payment and invoice over-allocation.
- `MDLN.post` rejects insufficient inventory before creating issue or invoice records.

Tool Runtime tests:

- `erp.action.execute` passes idempotency and context metadata into ERP action input.
- Approved Tool Runtime resume does not duplicate ERP side effects when the same idempotency key is reused.

Smoke tests:

- Create tenant organization.
- Apply ERP industry solution.
- Create requirement.
- Analyze, approve, and convert to project.
- Submit and approve purchase order.
- Post goods receipt and verify inventory/payable generation.
- Confirm and approve sales order.
- Post delivery and verify issue/invoice generation.
- Post invoice and allocate payment.
- Refresh project cost.
- Close feedback.

Verification commands:

```bash
cd backend
go test ./internal/domain/erp -count=1
go test ./internal/domain/toolruntime -count=1
go test ./cmd/smoke -count=1
go test ./...
```

Migration verification should run the current staged baseline against a fresh database and update the baseline documentation with the result.

## Phased Execution

### Phase 1.1: Action Contract and Ledger

Add `ActionInput` metadata fields, extend `ActionResult`, introduce `MACT` and `ACT1`, and record execution success/failure for existing actions.

### Phase 1.2: Preconditions

Move status checks into explicit action precondition helpers and cover the three business loops with tests.

### Phase 1.3: Transactional Actions

Introduce repository transaction support and wrap multi-write actions in transactions.

### Phase 1.4: Idempotency and Provenance

Add effective idempotency key derivation, replay handling, generated-record provenance, and generated-record ledger rows.

### Phase 1.5: End-to-End Smoke

Strengthen the existing smoke flow to assert generated records, statuses, balances, provenance, and final project feedback.

## Acceptance Criteria

Phase 1 is complete when:

- All listed ERP actions have explicit preconditions.
- All ERP action attempts create or reuse ledger entries.
- Multi-write actions are transaction-safe.
- Repeated post/convert/allocate/refresh/feedback calls are idempotent.
- Generated records carry source provenance.
- Tool Runtime passes context/idempotency metadata through `erp.action.execute`.
- ERP smoke covers the three end-to-end loops.
- Backend tests pass.
- Migration baseline SQL and baseline documentation are updated together.

## Risks and Mitigations

Risk: Transaction support may require widening the repository interface.
Mitigation: Add a small optional transactional interface and keep non-transaction tests supported with an in-memory implementation.

Risk: Idempotency can hide real failures if failed executions are replayed as success.
Mitigation: Replay only completed executions by default; require explicit `retry_failed: true` for failed attempts.

Risk: Adding ledger tables to the ERP baseline can drift from existing fresh DB assumptions.
Mitigation: Update `001_erp_code_baseline.sql` and `BASELINE_RESTRUCTURE.md` in the same change and run fresh migration verification.

Risk: Generated-record provenance can become inconsistent if each action writes it manually.
Mitigation: Centralize provenance construction in ERP service helpers and use those helpers for every generated document and line.

## Open Decisions Resolved

The action ledger belongs in the tenant ERP baseline, not the SaaS platform baseline, because it records tenant business execution history.

ERP table-code APIs remain the main tenant business runtime surface.

Tool Runtime remains the AI execution boundary; Phase 1 does not create an AI bypass path.

UI work is deferred until Phase 3 because Phase 1 is a backend reliability slice.
