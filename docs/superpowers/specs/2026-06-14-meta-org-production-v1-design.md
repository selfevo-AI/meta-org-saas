# Meta-Org Production v1 Design

## 1. Summary

Meta-Org Production v1 upgrades the current HarnessCompany codebase into a production-ready AI-native organization operating platform.

The current system already has substantial foundations:

- Go backend with domain modules for identity, organization, layer, capability, dashboard, workflow, project, governance, evolution, observability, and verification.
- PostgreSQL migrations through `015`, including organization tree, positions, project lifecycle, project costs, governance decisions, context weights, and capability evaluations.
- Next.js frontend with dashboard overview, organization workspace, control workspaces, project lifecycle workspace, API Workbench, authentication, and bilingual UI.
- Cross-domain orchestration already present in project and organization services, including governance access decisions, evolution context weights, member matching, workflow binding, cost entries, and feedback closure.

Production v1 must not discard this framework. It will rename and reposition the product as Meta-Org, then extend the existing architecture with a production AI gateway, tool runtime, finance export integration, and user-facing Meta-Org entry experience.

Target initial production scale:

- 10-50 human users.
- 50-250+ AI agents.
- Real external model calls through OpenAI, Anthropic, and Gemini.
- Streaming model responses in all three provider adapters.
- Full tool-calling loop with governance-controlled execution.
- External finance system integration through a generic finance adapter and Webhook/API export.

## 2. Product Direction

Meta-Org is the single-enterprise top-level organization operating system entry point.

For v1, Meta-Org represents one enterprise or organization. The data model should continue to use the existing `organizations` table as the root enterprise node. Future multi-enterprise or super-admin capabilities must remain possible, but Production v1 does not implement a multi-enterprise federation layer.

The product must serve four real-user scenarios through one coherent workspace:

- Internal project collaboration: requirements, project formation, task/workflow execution, delivery, and review.
- AI agent operations: agent identity, model/provider configuration, permissions, capabilities, cost, and performance.
- Mixed human/external/AI delivery: internal members, external members, agents, project delivery, and cost attribution.
- Governance cockpit: risk, access decisions, tool policies, costs, signals, and organizational health.

Meta-Org v1 homepage:

- Operations dashboard: organization health, project status, agent status, model cost, risk, recent events.
- Inbox: approvals, tool execution requests, verification reviews, failed exports, high-risk events, and pending project actions.
- Secondary entry points: organization map, project operations, agent workforce, governance control, evolution memory, developer tools, finance exports.

## 3. Canonical Rename

Production v1 includes a full rename from HarnessCompany to Meta-Org.

Canonical identity:

- Product name: `Meta-Org`
- GitHub repository: `https://github.com/selfevo-AI/meta-org`
- Go module: `github.com/selfevo-AI/meta-org-saas/backend`
- Frontend package: `meta-org-frontend`
- PostgreSQL database: `meta_org`
- Default local database URL: `postgres://postgres:postgres@localhost:5432/meta_org?sslmode=disable`
- Docker database URL: `postgres://postgres:postgres@postgres:5432/meta_org?sslmode=disable`

Rename requirements:

- Update README, page metadata, product labels, sample organization names, and developer-facing docs.
- Update Go module path and all Go imports.
- Update frontend package name and lockfile package name.
- Update Docker Compose database name and backend default `DATABASE_URL`.
- Update frontend storage keys from `harness.*` to `meta_org.*`.
- Preserve a one-time frontend migration from old localStorage keys to new keys so existing local sessions and menu preferences are not silently lost.
- API base path remains `/api/v1`; do not introduce `/meta-org/api/v1` in v1.
- Existing deployments using `harness_org` must require explicit backup and database migration. Do not silently delete or overwrite the old database.

## 4. Architecture

Production v1 remains a modular monolith unless a specific integration requires an external component.

The backend should add these domains while reusing existing ones:

- `metaorg`: aggregate Meta-Org homepage, inbox, health, and navigation data.
- `aigateway`: model providers, models, invocations, streaming, usage ledger, and provider adapters.
- `toolruntime`: tool registry, tool policies, execution lifecycle, approvals, and audit records.
- `finance`: generic finance adapter, export batches, outbound payloads, webhook callbacks, and reconciliation state.

Existing domains stay authoritative for their business concepts:

- `identity`: users, AI agents, roles, auth.
- `organization`: departments, positions, members, external members, MVRU, matching.
- `project`: requirements, projects, members, workflows, deliverables, project cost entries, feedback.
- `workflow`: templates, instances, tasks, decisions, contexts.
- `governance`: permissions, principles, control rules, access decisions.
- `evolution`: weights, context weights, experiments, knowledge, signals.
- `observability`: traces, spans, metrics.
- `capability`: capabilities, bindings, evaluations.
- `dashboard`: existing overview aggregation, reused or folded into `metaorg` gradually.

Cross-domain dependency direction:

- `aigateway` records usage and emits observability events; it does not directly mutate projects except through explicit cost posting services.
- `toolruntime` invokes existing domain services through registered internal adapters, not raw HTTP calls to itself.
- `finance` exports cost and usage facts; external finance status flows back into `finance` and linked cost records.
- `metaorg` reads summaries and inbox items; it should not own underlying business state.
- `governance` remains the final authority for execution permission and risk escalation.

## 5. Meta-Org Entry Experience

The current frontend already has a main workspace in `frontend/src/app/page.tsx` with dashboard, grouped navigation, organization workspace, control workspaces, project lifecycle workspace, and API Workbench. Production v1 should upgrade this rather than rebuild it.

Homepage sections:

- Executive strip: active projects, open requirements, active agents, daily model cost, unresolved risks, pending approvals.
- Operational status: project statuses, deliverables, workflow tasks, agent usage, failed model calls, finance export failures.
- Inbox: tool approvals, governance approvals, verification reviews, failed finance exports, high-priority evolution signals, pending project feedback.
- Cost panel: today/month-to-date AI cost, project cost variance, provider cost split, unexported finance amount.
- Risk panel: denied/approved access decisions, high-risk tools, critical agents, failed streaming calls.
- Recent activity: reuse existing recent event pattern and extend it with model calls, tool executions, and finance events.

Navigation:

- Meta-Org Home
- Organization Map
- Project Operations
- Agent Workforce
- Governance Control
- Evolution Memory
- Developer Tools
- Finance Exports

## 6. AI Gateway

The AI gateway is production critical.

Supported providers in v1:

- OpenAI native adapter.
- Anthropic native adapter.
- Gemini native adapter.

Supported call modes:

- Non-streaming chat/text generation.
- Streaming chat/text generation for all three provider adapters.
- Tool calling loop through `toolruntime`.

Explicit v1 exclusions:

- Fine-tuning.
- Batch jobs.
- Image/audio/video generation.
- Embeddings, unless required for a specific internal tool later.
- Arbitrary external MCP servers or arbitrary webhooks as auto-executed tools.

Provider configuration:

- Provider name, type, base URL, auth type, encrypted API key, status, timeout, retry policy, rate limits, risk level, tags.
- API keys must be encrypted at rest and never returned in plaintext after creation.
- Frontend only receives masked key state, rotation status, and last validation result.
- Provider config changes must be audited.

Model catalog:

- Provider ID, model key, display name, context window, max output tokens, input token price, output token price, currency, price effective date, capabilities, status.
- Price versions must be preserved for historical cost accuracy.
- Model disablement must prevent new calls but not break historical records.

Invocation requirements:

- Unified request schema includes model, messages, temperature/top_p/max_tokens, stream flag, tools, tool policy context, metadata, and attribution fields.
- Attribution fields include organization, department, project, requirement, workflow, task, agent, user, capability, and source surface where available.
- Every invocation creates an immutable usage ledger record.
- Streaming responses must record start, first-token time, completion time, status, provider request ID, token counts when available, estimated token counts when final usage is absent, and error details.
- Client disconnects must be recorded distinctly from provider errors.
- Retries must be bounded and idempotent for non-streaming calls; streaming retries must not duplicate tool execution.

Streaming protocol:

- Backend exposes a stable streaming endpoint using SSE for browser compatibility.
- Events include lifecycle, delta, tool_call_requested, tool_call_started, tool_call_result, usage_update, error, and done.
- Events include a gateway invocation ID so the frontend can reconnect or show a durable audit reference.
- Frontend must handle partial output, cancellation, network interruption, provider timeout, and final cost update.

## 7. Tool Runtime

Tool runtime v1 implements a complete tool-calling loop while limiting tool sources to production-controllable surfaces.

Allowed tool sources:

- Internal API/service tools wrapping existing domain services.
- Interface files maintained in Developer Tools.
- Manual approval tools for high-risk or externally sensitive operations.

Initial high-value tools:

- Analyze requirement.
- Create or update requirement.
- Recommend project members using existing organization matching.
- Generate or bind workflow template.
- Estimate project cost.
- Create project cost entry from approved AI usage or estimate.
- Explain governance decision.
- Propose governance control rule.
- Summarize project status.
- Prepare finance export batch.

Tool policy:

- Each tool has a default execution policy: `auto`, `notify`, `approve`, or `deny`.
- Governance control rules can override the tool policy.
- Tool execution must include actor, actor type, organization, department, project/workflow/task context, risk level, and required permission level.
- High-risk or critical actions must be escalated to approval or denied even if the tool default is `auto`.
- All mutating tools require idempotency keys.
- Tool inputs and outputs must be validated against schema.
- Tool output must be summarized and redacted before being sent back to the model if it contains sensitive fields.

Approval flow:

- Pending approvals appear in the Meta-Org inbox.
- Approval records include requested tool, actor, model invocation, arguments summary, full arguments for authorized reviewers, risk, cost estimate, and expiry.
- Approval actions are approve, reject, request changes, and expire.
- Approved tool calls continue the model loop with tool result context.

Audit:

- Every tool call stores request, normalized arguments, execution policy, governance decision, status, result summary, error, duration, and linked model invocation.
- Audit records are immutable except for status transitions.

## 8. Context AI Assistants

AI capability should be embedded into existing business pages.

Frontend pattern:

- Right-side context AI assistant for multi-turn work.
- Inline action buttons for frequent tasks.
- All assistants use the AI gateway and tool runtime.
- Assistant state is scoped to the current page context and can cite created records or failed tool calls.

Initial placements:

- Meta-Org Home: summarize health, risks, costs, and pending actions.
- Requirement area: analyze requirement, suggest priority/risk, propose acceptance criteria, start analysis workflow.
- Project area: recommend members/agents, generate workflow, summarize project, estimate cost, prepare feedback.
- Organization area: suggest positions, identify capability gaps, recommend agents/members.
- Governance area: explain decisions, propose control rules, review high-risk tools.
- Developer Tools: test providers, inspect tool schemas, replay safe model calls.

UX requirements:

- Streaming output must be cancellable.
- Tool calls must be visible, not hidden behind opaque text.
- Mutating actions must show confirmation or approval status.
- Cost estimate and final cost must be visible for model calls where available.
- Errors must distinguish provider failure, validation failure, governance denial, approval required, finance export failure, and network interruption.

## 9. Cost Accounting

Existing `project_cost_entries` and `CostSummary` are useful for project cost reporting, but they are not enough for financial integration.

New AI usage ledger:

- Immutable per invocation.
- Stores provider, model, request mode, token counts, estimated/final costs, currency, latency, status, attribution, and price version.
- Records streaming partial/final state.
- Links tool executions and observability traces.
- Supports cost correction records instead of updating historical facts destructively.

Cost attribution:

- Organization and department are required when available.
- Project attribution is required for project-context assistants and project tools.
- Agent attribution is required when an AI agent initiates or owns a call.
- Workflow/task attribution is required when the call occurs inside workflow execution.
- Unattributed cost is allowed only with explicit reason and appears in finance review.

Project cost integration:

- AI usage can be posted into `project_cost_entries` after cost finalization or finance export confirmation, depending on policy.
- Generated project cost entries use source type such as `ai_usage` and link back to usage ledger IDs.
- Duplicate posting is prevented by idempotency key and unique source linkage.

Cost summaries:

- By provider, model, project, department, agent, workflow, task, day, month, currency, and export status.
- Budget variance should include AI cost when posted to project costs.

## 10. Finance Integration

The external finance system is the accounting source of truth.

Meta-Org responsibilities:

- Maintain immutable AI usage and internal cost attribution.
- Prepare finance export batches.
- Push standard voucher/detail payloads through Generic Finance Adapter.
- Receive webhook/API callbacks for accepted, rejected, posted, reconciled, or failed statuses.
- Track reconciliation differences and expose them in Meta-Org inbox.

Generic Finance Adapter:

- Configurable endpoint, auth method, signing secret, retry policy, timeout, enabled status.
- Payload format versioning.
- Webhook callback endpoint with signature verification.
- Idempotency key per exported cost line and per batch.
- Dead-letter state for repeated failures.

Finance export batch:

- Period start/end.
- Organization, department, project, provider, model, currency, amount, tax fields if configured, and line details.
- Status: draft, ready, exporting, exported, acknowledged, posted, reconciled, failed, cancelled.
- Locking: once exported, source lines cannot be changed; corrections must be new adjustment records.

Webhook callback:

- External voucher ID.
- External line IDs.
- Posted/reconciled status.
- Failure reason and retryability.
- Reconciliation amount and difference if provided.

## 11. Developer Tools

Developer Tools extend the existing API Workbench.

Required sections:

- API Workbench: existing operation browser and caller.
- Model Providers: create/update provider, rotate keys, test connection, view health.
- Model Catalog: models, pricing, capabilities, status.
- Interface Files: JSON/YAML/Markdown definitions for tools, agent contracts, workflow adapters, and provider notes.
- Tool Registry: internal tools, schemas, policies, risk, governance requirements, test execution.
- Invocation Logs: model calls, streaming events, tool calls, traces, errors.
- Cost and Finance: usage summaries, unexported cost, export batches, callback records.

Access control:

- Only authorized users can view or change provider secrets, tool policies, finance adapters, and export batches.
- Secret values are never shown after creation.

## 12. Security and Governance

Security requirements:

- API keys encrypted at rest.
- No provider secret in frontend localStorage.
- Sensitive fields redacted in logs, tool outputs, and assistant visible traces.
- All mutating AI/tool actions require authenticated user or agent identity.
- Governance `DecideAccess` is invoked before mutating tool execution.
- Model-generated tool arguments must be treated as untrusted input.
- Finance callbacks must be signature verified.
- Rate limits by user, agent, provider, model, and organization.
- Audit trail for provider changes, tool policy changes, approvals, model invocations, tool executions, and finance exports.

JWT/local auth remains acceptable for v1 scale, but production deployment must require non-default `JWT_SECRET` and documented secret management.

## 13. Observability and Reliability

Reuse and extend existing observability domain:

- Trace per assistant interaction.
- Span per model call, tool call, finance export, and webhook callback.
- Metrics for latency, token usage, cost, error rates, streaming disconnects, tool approvals, finance failures.
- Recent events include AI gateway, tool runtime, and finance events.

Reliability requirements:

- Streaming cancellation and disconnect handling.
- Idempotent mutating tools.
- Idempotent finance exports.
- Retry policies for provider calls and finance exports.
- Dead-letter queue or persisted failed-export state.
- Backpressure for provider rate limits.
- Graceful degradation when one provider is unavailable.

## 14. Public API Additions

API base remains `/api/v1`.

Meta-Org:

- `GET /meta-org/overview`
- `GET /meta-org/inbox`
- `GET /meta-org/activity`

AI Gateway:

- `POST /model-providers`
- `GET /model-providers`
- `PATCH /model-providers/{id}`
- `POST /model-providers/{id}/rotate-key`
- `POST /model-providers/{id}/test`
- `POST /models`
- `GET /models`
- `PATCH /models/{id}`
- `POST /ai-gateway/invoke`
- `GET /ai-gateway/stream`
- `GET /ai-gateway/invocations`
- `GET /ai-gateway/invocations/{id}`
- `GET /ai-gateway/cost-summary`

Tool Runtime:

- `POST /tools`
- `GET /tools`
- `PATCH /tools/{id}`
- `POST /tools/{id}/test`
- `GET /tool-executions`
- `GET /tool-executions/{id}`
- `POST /tool-approvals/{id}/approve`
- `POST /tool-approvals/{id}/reject`

Finance:

- `POST /finance/adapters`
- `GET /finance/adapters`
- `PATCH /finance/adapters/{id}`
- `POST /finance/adapters/{id}/test`
- `POST /finance/export-batches`
- `GET /finance/export-batches`
- `GET /finance/export-batches/{id}`
- `POST /finance/export-batches/{id}/submit`
- `POST /finance/webhooks/{adapterID}`
- `GET /finance/reconciliation`

Developer tools can consume these APIs; they do not need separate backend-only paths unless admin authorization requires grouping.

## 15. Data Model Additions

Migration additions should be append-only and numbered after current `015`.

Likely new tables:

- `model_providers`
- `model_provider_secrets` or encrypted secret fields in provider table, depending on local secret strategy.
- `models`
- `model_price_versions`
- `ai_invocations`
- `ai_stream_events` if durable event replay is required.
- `ai_usage_ledger`
- `tool_definitions`
- `tool_policy_versions`
- `tool_executions`
- `tool_approvals`
- `interface_files`
- `finance_adapters`
- `finance_export_batches`
- `finance_export_lines`
- `finance_webhook_events`
- `finance_reconciliation_results`

All tables that represent audit or finance facts need immutable created timestamps and explicit status transitions. Corrections should be additive.

## 16. Testing and CI

Production v1 requires automated tests.

Backend tests:

- Unit tests for model pricing, token/cost calculation, provider request normalization, provider response parsing.
- Unit tests for tool policy decisions, governance override, idempotency, approval state transitions.
- Unit tests for finance export batching, idempotency keys, callback signature verification, reconciliation state transitions.
- Integration tests for project cost posting from AI usage ledger.
- Handler tests for auth, validation, and error formats.

Frontend tests:

- Component tests or integration tests for AI assistant streaming states.
- Developer Tools provider config and masked key behavior.
- Finance export status UI.
- Meta-Org inbox rendering.

End-to-end scenarios:

- Configure provider, test connection, stream model response, record usage and cost.
- AI assistant recommends project members through tool runtime and governance approval.
- Approved AI usage posts to project cost and finance export batch.
- Finance webhook marks export as posted/reconciled.
- Provider streaming disconnect is visible and auditable.

CI:

- Go test.
- Go build.
- Frontend lint.
- Frontend build.
- Migration validation against fresh PostgreSQL.

## 17. Rollout

Recommended production rollout:

1. Commit current README documentation state or explicitly separate it before implementation.
2. Rename to Meta-Org and update canonical repository/module/database naming.
3. Add migrations for AI gateway, tool runtime, and finance.
4. Implement AI Gateway with provider tests and mock provider adapters first, then real adapters.
5. Implement Developer Tools for provider/model/tool/finance setup.
6. Implement Meta-Org overview/inbox using existing dashboard/project/governance/evolution data.
7. Implement context AI assistants and selected tool wrappers.
8. Implement finance adapter export/callback loop.
9. Add production verification, CI, deployment docs, and rollback docs.

## 18. Acceptance Criteria

Production v1 is complete only when:

- Product and repository identity are fully Meta-Org.
- A fresh Docker Compose deployment starts with `meta_org`.
- The frontend Meta-Org homepage is the primary authenticated entry.
- A user can configure OpenAI, Anthropic, and Gemini providers.
- A user can stream responses from all three providers.
- Every model invocation creates usage and cost records.
- A page-level AI assistant can call at least one read tool and one mutating tool through governance-controlled tool runtime.
- Mutating tool calls support approval and audit.
- AI usage can be attributed to project/department/agent/workflow context.
- AI cost can appear in project cost reporting when posted.
- A finance export batch can be generated, submitted to a generic webhook/API adapter, and reconciled from callback.
- Provider secrets are encrypted/masked and not exposed in frontend storage.
- Tests and CI cover core gateway, tool, cost, finance, and UI states.
- Operational docs explain setup, secrets, provider config, finance adapter config, backup, and rollback.

## 19. Assumptions

- The first production deployment serves one enterprise Meta-Org.
- External finance system is generic and will integrate through API/Webhook export, not direct vendor-specific SDKs.
- The platform remains a Go modular monolith for v1.
- No automatic deletion or destructive migration of existing `harness_org` data occurs.
- Existing project and organization functionality remains available throughout the migration.

