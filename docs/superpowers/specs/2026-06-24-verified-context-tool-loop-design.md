# Verified Context and Tool Loop Design

Date: 2026-06-24

## Purpose

Phase 4 promotes the existing Verified Context Engine and Tool Runtime from
framework pieces into the assistant execution path.

The target is a governed loop:

```text
business signal -> verified context -> model decision -> tool runtime
-> approval or execution -> observability -> refreshed context -> learning
```

The assistant must not reason from ungoverned records or execute business
actions directly. Every model-visible business fact must be packaged through
verified context, and every AI-executable operation must pass through Tool
Runtime policy, governance, approval, idempotency, and observability.

## Current State

The repository already contains most of the required structure:

- `VerifiedContextEngine` builds `ContextPackage` values, but today it mainly
  wraps records from the compatibility context resolver.
- `ContextRuleEvaluator` already splits items into attention core, supporting
  context, risk signals, and omissions.
- `context_dictionary_versions`, `context_rules`, `context_packages`, and
  `context_change_proposals` already exist in the AI capability baseline.
- `AssistantRuntime.Run` can build a context package before delegating to the
  legacy loop, but `Run`, `Resume`, and turn continuation still use legacy
  `WorkRecordContext` as the main prompt context.
- Tool Runtime already records tools, executions, approvals, governance
  decisions, idempotency keys, observability traces, spans, and metrics.
- Some platform solution package metadata already declares tools such as
  `erp.action.execute`, `schema.change.preview`, and
  `runtime.operation.execute`, but not all of them are registered as executable
  Tool Runtime adapters.

Phase 4 should therefore connect and harden existing systems. It should avoid
large new schema surfaces unless a small metadata field is required to preserve
auditability.

## Goals

- Make active dictionary rules the primary source of assistant context for ERP,
  finance, and governance domains.
- Keep the compatibility resolver as a controlled fallback while dictionary
  coverage is being expanded.
- Build a `ContextPackage` before every assistant run and every approval resume.
- Attach context package metadata to AI Gateway invocations, assistant steps,
  tool calls, approval waits, and resumed tool results.
- Route all AI-executable actions through Tool Runtime.
- Register missing Tool Runtime adapters for ERP actions, schema verification,
  runtime operation execution, and context proposal application.
- Ensure AI can suggest context changes but cannot activate context rules.
- Preserve the existing assistant API and frontend event stream shape as much
  as practical.
- Produce enough observability and cost metadata to connect token usage to
  business flow progress and tool outcomes.

## Non-Goals

- Do not let AI execute DDL or apply schema changes directly.
- Do not let AI activate context dictionary versions or context rules.
- Do not remove the compatibility resolver until dictionary-backed coverage is
  proven for the main ERP, finance, and governance paths.
- Do not redesign the ERP workbench UI in this phase. Phase 4 can expose
  context and approval metadata, but broad document-view UX work belongs to
  the tenant workbench phase.
- Do not introduce vector search, graph storage, or another retrieval backend
  as a dependency for this phase.
- Do not reintroduce old semantic tenant APIs as assistant tool targets.

## Architecture

Phase 4 adds one primary runtime contract:

```text
AssistantService
-> AssistantRuntime
-> VerifiedContextEngine
-> AI Gateway
-> ToolRunner
-> Tool Runtime
-> approval or adapter execution
-> AssistantRuntime resume with refreshed context
```

`AssistantService` remains the HTTP-facing application service. It validates
input, loads sessions, and delegates execution.

`AssistantRuntime` becomes the main owner of run and resume orchestration. It
builds the verified context package, prepares model messages, invokes the AI
Gateway, records assistant steps, calls tools, pauses for approvals, and
continues after approved tool execution.

`VerifiedContextEngine` becomes responsible for resolving active dictionary
rules and producing a deterministic `ContextPackage`.

`ToolRunner` is a thin adapter between assistant tool calls and Tool Runtime.
It should pass session scope, invocation ID, context package ID, idempotency
key, and arguments into Tool Runtime.

`ToolRuntime.Service` remains the single execution control plane for policy,
governance, approval, adapter dispatch, execution logs, and observability.

## Context Engine Design

The context engine should resolve context in this order:

1. Active organization-level dictionary rules for the current organization and
   module.
2. Active SaaS-level dictionary rules for the current module.
3. Compatibility resolver records, explicitly marked as fallback provenance.

The engine should convert active rules and allowed records into `ContextItem`
values. Each item must carry:

- logical entity and field keys
- record identifier
- value or redacted value
- source
- weight
- estimated token cost
- validation state
- metadata with rule IDs, dictionary version IDs, and physical mapping hints

Validation must be deterministic:

- Permission failures become omissions and do not leak sensitive values.
- Finance conflicts become risk signals, not confirmed facts.
- Workflow-stage relevance changes item weight.
- Governance-restricted fields require explicit active rules.
- Token budget controls attention core size.

The compatibility resolver remains useful for migration, but its output must
be lower trust than dictionary-backed context. The package provenance should
make this visible with values such as `source: dictionary` and
`fallback_source: compatibility_resolver`.

## Assistant Runtime Design

`Run` should build a context package before model invocation and use that
package to assemble model messages. The old `WorkRecordContext` should be
derived from the package only when a legacy helper still needs that shape.

Each LLM invocation should include metadata:

- `assistant_session_id`
- `context_package_id`
- `dictionary_version_ids`
- `attention_core_count`
- `supporting_context_count`
- `risk_signal_count`
- `omission_count`
- `module_key`
- `target_type`
- `target_id`
- `business_flow`

Each assistant LLM step should persist the same compact metadata. The full
context package stays in `context_packages`; assistant steps should store only
references and summaries.

`Resume` must rebuild context after an approved tool execution and before the
model continues. This prevents the assistant from continuing with stale facts
after a tool changed business state.

The resumed tool-result step should record:

- original tool execution ID
- approval ID
- refreshed context package ID
- prior context package ID when available
- tool status and summary
- result payload

## Tool Runtime Design

All AI-executable operations must be Tool Runtime tools. The missing adapters
should be registered with explicit defaults:

| Tool name | Adapter responsibility | Default policy | Risk |
|---|---|---|---|
| `erp.action.execute` | Run ERP action by `table_code`, `key`, and `action` | approve for high-risk actions, notify otherwise | medium/high |
| `schema.change.preview` | Run schema-change verification only | notify | low |
| `runtime.operation.execute` | Execute a platform runtime operation | approve for writes/deletes, notify for reads | medium |
| `context.proposal.apply` | Apply an already approved context change proposal | approve | high |
| `finance.prepare_export_batch` | Prepare finance export batch | approve | high |

The tool execution arguments should include context metadata:

- `context_package_id`
- `target_type`
- `target_id`
- `business_flow`
- `assistant_session_id`

Tool Runtime should continue to derive the final policy from tool defaults and
governance decisions. High-risk ERP actions, finance exports, schema-related
operations, and context proposal application must require human approval.

Idempotency keys should be stable:

```text
assistant:{session_id}:{context_package_id}:{tool_call_id}:{turn}
```

For ERP action execution, the adapter should also pass or derive:

```text
erp:{organization_id}:{table_code}:{key}:{action}
```

This prevents repeated model/tool retries from creating duplicate business
effects.

## Approval Resume

Approval resume is the most important safety boundary in this phase.

The sequence should be:

1. Human approves the Tool Runtime approval.
2. Tool Runtime runs the adapter and records execution completion.
3. Assistant `Resume` verifies approval and execution status.
4. Assistant runtime rebuilds a fresh context package for the same session.
5. Runtime records a tool-result step with the new context package ID.
6. Runtime appends the tool result message.
7. Runtime invokes the model with refreshed context and prior history.

If context rebuild fails after approval, the session should stop in a failed
state with a classified context error. The assistant should not continue with
the old package.

## Context Change Governance

AI may create proposals for:

- new context fields
- adjusted field weights
- workflow-stage multipliers
- finance validation hints
- governance visibility rules
- physical mapping suggestions

AI may not:

- activate dictionary versions
- activate rules
- raise permissions
- execute DDL
- apply context proposals without a human-approved Tool Runtime execution

The `context.proposal.apply` tool is intentionally high risk. It should only
apply proposals that are already approved by a human reviewer and should record
the reviewer, execution ID, and resulting active rule IDs.

## Observability and Cost Attribution

The system should connect these entities:

```text
ai_invocation -> context_package -> tool_execution -> approval -> ERP action
```

AI Gateway invocation metadata should include context package and business flow
identifiers. Tool Runtime observability spans should include assistant session,
context package, approval, and business target metadata.

This lets later monitoring answer practical questions:

- Which context omissions caused repeated tool failures?
- Which tools required approval most often?
- Which business flows consumed tokens without completing a state transition?
- Which dictionary rules reduced failed tool calls?
- Which finance or governance validations blocked unsafe actions?

## Frontend Design

Frontend work in this phase should be minimal and diagnostic:

- Show context package ID and summary on assistant run details.
- Show omissions and risk signals in assistant/debug panels.
- Show refreshed context metadata when an approval resume completes.
- Link assistant approval events to Tool Runtime execution status.
- Keep all visible text bilingual through the existing i18n layer.

The UI should not become a broad context editor in this phase. Context rule
editing and approval can remain in platform/admin flows unless a focused review
panel already exists.

## Database and Migration Notes

The current staged baseline already includes the main context dictionary and
package tables. Phase 4 should prefer repository queries and metadata usage
over new tables.

If implementation requires new columns, the same change must update the
relevant migration stage and baseline documentation. The likely minimal
schema addition would be a context package reference on assistant steps or tool
executions, but this should be avoided if existing JSON metadata can carry the
reference reliably.

No migration should make AI-created rules active by default.

## Error Handling

Runtime failures should be classified instead of collapsing into generic
provider errors:

- `context_error`
- `permission_denied`
- `finance_validation_failed`
- `governance_denied`
- `tool_approval_required`
- `tool_execution_failed`
- `approval_rejected`
- `stale_context`
- `runtime_error`

Context errors during resume are blocking. Tool execution failures may be
returned to the model only if the tool completed through Tool Runtime and the
failure is part of normal business handling.

## Acceptance Criteria

- Assistant run builds and persists a context package before invoking the AI
  Gateway.
- Assistant resume rebuilds context after an approved tool execution and before
  invoking the model again.
- AI Gateway invocation metadata includes `context_package_id` and compact
  context summary counts.
- Assistant LLM, tool call, approval, and tool result steps include context
  package references or summaries.
- ERP action, schema preview, runtime operation, and context proposal apply
  are registered as Tool Runtime tools.
- High-risk AI tool actions require Tool Runtime approval.
- AI-created context suggestions remain proposals until human approval.
- Permission-denied fields are omissions, not prompt-visible facts.
- Finance-conflicted fields are risk signals, not confirmed facts.
- Legacy context resolver fallback is visible in provenance.
- Existing assistant APIs remain compatible enough for the current frontend.

## Testing Strategy

Backend tests should cover:

- Context package building from active dictionary rules.
- Permission-denied fields becoming omissions.
- Finance validation conflicts becoming risk signals.
- Workflow-stage rules changing context weights.
- Attention core budget enforcement.
- Assistant run attaching context package metadata to AI invocation and steps.
- Assistant resume rebuilding context after approved tool execution.
- Tool Runtime adapter registration for the Phase 4 tool set.
- ERP action execution through Tool Runtime with idempotency.
- Context proposal application requiring approved Tool Runtime execution.
- AI suggestions creating proposals without activating rules.

Verification commands:

```bash
cd backend
go test ./internal/domain/assistant ./internal/domain/toolruntime ./internal/domain/aigateway
go test ./...
```

If frontend diagnostic UI is touched:

```bash
cd frontend
npm run lint
npm run build
```

Always finish with:

```bash
git diff --check
```

