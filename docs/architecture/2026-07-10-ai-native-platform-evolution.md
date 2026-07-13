# Meta-Org AI-Native Platform Evolution

## Purpose

This document defines the target architecture and staged evolution path for
Meta-Org across backend framework, database topology, service construction,
business logic, AI runtime, governance, observability, and frontend UI.

The objective is not to split the repository into services prematurely. The
objective is to establish stable ownership, contracts, and operational
boundaries so the platform can evolve from a modular monolith into a distributed
system only where scale or organizational ownership requires it.

## Current Architectural Truth

Meta-Org currently combines four major systems:

1. A SaaS control plane for platform users, tenant organizations, subscriptions,
   entitlements, industry solutions, model access, and governance.
2. Dedicated tenant databases for organization and ERP business execution.
3. An AI control and execution plane containing model routing, verified context,
   tools, assistants, skills, approvals, and usage accounting.
4. A single-page operational frontend containing platform and tenant workspaces.

The immediate risk is contract drift between those systems. Future work must
therefore optimize boundaries before adding more features.

## Target Architecture

```text
Browser / Agent / External API
            |
      API and Event Gateway
            |
  +---------+----------+
  | SaaS Control Plane |
  | Identity / RBAC    |
  | Tenant Catalog     |
  | Package Factory    |
  | AI Control Plane   |
  +---------+----------+
            |
    Commands / Events / Projections
            |
  +---------+----------+
  | Tenant Runtime     |
  | ERP Action Engine  |
  | Workflow / Finance |
  | Organization Data  |
  +---------+----------+
            |
  Security Kernel / Policy Decision Point
```

The platform database remains the control-plane source of truth. Tenant
databases remain the business-data source of truth. Cross-boundary reads use
versioned projections or explicit service calls. Cross-database joins and
cross-database transactions are prohibited.

## Framework Evolution

### Modular Monolith First

Keep one deployable Go backend until a module demonstrates an independent need
for scaling, security isolation, deployment cadence, or ownership. Each domain
must expose a module constructor with explicit ports instead of adding more
assembly logic directly to `cmd/server/main.go`.

Target module shape:

```text
internal/domain/<domain>/
  module.go        dependency declaration and route mounting
  command.go       state-changing use cases
  query.go         read models
  policy.go        authorization and invariants
  repository.go    persistence ports and PostgreSQL adapter
  events.go        domain and integration events
  handler.go       HTTP transport only
```

Existing `service.go` files can be split incrementally when they exceed a clear
business boundary. File size alone is not a reason to create a new service.

### Command and Query Contracts

All mutations should become named commands with:

- actor and tenant context;
- idempotency key;
- expected version or preconditions;
- policy decision;
- transaction boundary;
- emitted domain events;
- audit and provenance metadata.

Queries should return purpose-built read models instead of leaking database
rows. Long-running operations return job resources rather than holding an HTTP
request open.

### Contract-First APIs

Introduce a generated API contract after route ownership is stabilized.
OpenAPI is the external HTTP contract; JSON Schema remains the contract for
tools, operations, industry manifests, assistant proposals, and event payloads.
Frontend types and agent-facing operation metadata should be generated from the
same source where practical.

MCP, A2A, and future agent protocols are adapters over the internal command and
tool contracts. They must not become the core domain model.

The first HTTP contract stage standardizes internal `/api/v1` failures as
`error`, stable `code`, and `request_id`, with matching response headers and
structured frontend errors. OpenAI-compatible `/v1` remains an independent
adapter contract. Generated OpenAPI schemas should build on this envelope.

## Database Evolution

### Control Plane and Tenant Plane

Platform-owned data:

- users, platform roles, tenant organizations, subscriptions and entitlements;
- database targets, migrations, packages and marketplace metadata;
- model providers, routing policy, access tokens and usage settlement;
- global tools, platform skills, governance policy and audit projections.

Tenant-owned data:

- departments, positions, memberships and tenant-local actor projections;
- ERP documents, finance, costing, inventory, procurement and sales;
- tenant workflows, projects, execution ledgers and business events;
- tenant-installed solution assets and tenant-private AI configuration.

### Projection and Event Model

Add tenant-local outbox and platform inbox tables. Business transactions append
events in the same tenant transaction. A projection worker publishes events to
the platform control plane and maintains dashboard, Meta-Org, monitoring, cost,
and search projections.

Every event must contain:

- `event_id`, `event_type`, `event_version`;
- `organization_id`, actor identity and authority tier;
- aggregate type, aggregate ID and aggregate version;
- trace ID, causation ID and correlation ID;
- occurred time and schema version;
- payload and redacted metadata.

Consumers use inbox deduplication. Projection lag must be observable.

### Migration Governance

Platform and tenant migrations need checksums, immutable history, fresh-database
verification, upgrade-path verification, and downgrade documentation. Baseline
files remain the fresh-install truth, while numbered repair migrations carry
changes for already-applied databases.

Schema changes require:

1. owner stage update;
2. repair migration when an existing database can already contain the stage;
3. migration documentation update;
4. fresh platform and tenant migration tests;
5. compatibility verification against the previous release.

### Future Data Capabilities

- Use PostgreSQL JSONB only for extensible metadata, not hidden core fields.
- Add optimistic versions to mutable business aggregates.
- Partition high-volume invocation, event, audit, and usage ledgers by time.
- Introduce `pgvector` only for derived embeddings and retrieval indexes; the
  canonical business record remains relational and verified.
- Add temporal history for permissions, subscriptions, prices, policies, and
  business-document status changes.
- Maintain a semantic data catalog connecting logical fields, physical columns,
  sensitivity, permissions, lineage, and assistant context rules.

The four-hex tenant database naming rule has a limited collision space. Until
the naming contract is formally changed, provisioning must detect collisions
before database creation and return a governed remediation state instead of
silently attaching to an existing database.

## Service Construction

### Module Registry

Replace the growing manual constructor block in `cmd/server/main.go` with a
module registry. A module declares configuration validation, repositories,
services, background workers, routes, health checks, and shutdown hooks.

The registry must preserve explicit dependencies. It is not a reflection-based
dependency injection container.

### Application Lifecycle

Service startup becomes:

1. load and validate configuration;
2. connect platform infrastructure;
3. verify security kernel and required dependencies;
4. apply platform migrations;
5. construct modules;
6. start job workers and projection consumers;
7. expose readiness only after mandatory dependencies are ready;
8. drain HTTP, workers and tenant pools during shutdown.

Tenant provisioning, package application, backup, restore, model tests, large
imports, and assistant evaluation suites are persistent jobs with retry,
progress, cancellation, and audit state.

### Reliability

- Apply timeouts per dependency and operation, not one generic timeout.
- Add bounded tenant-pool caching with idle eviction and connection budgets.
- Add retries only for classified transient errors.
- Use circuit breakers for model providers, finance adapters, and remote tools.
- Define readiness, liveness, startup, and dependency health separately.
- Standardize structured errors with stable codes and bilingual UI mappings.

## Business Logic Evolution

### ERP Action Engine

The existing ERP action ledger becomes the canonical mutation engine for tenant
business documents. All important state transitions should use registered
actions rather than arbitrary record patches.

Each action definition contains:

- allowed source states and resulting state;
- field and document preconditions;
- required tenant permission and authority tier;
- risk and approval policy;
- transactional effects and generated records;
- reversal or compensation behavior;
- accounting, inventory and cost posting rules;
- assistant/tool exposure metadata;
- verification scenarios and invariants.

### Workflow and Saga Model

Use local database transactions inside one tenant database. Use sagas for
platform-to-tenant and external integrations. Saga state must be persistent,
idempotent and observable. Compensation is a named business action, not an
implicit rollback across services.

### Industry Solution Factory

Industry packages should evolve into signed, versioned capability bundles:

- data definitions and migration assets;
- document and action metadata;
- UI workspace definitions;
- assistant context rules and skills;
- policies, approval gates and verification scenarios;
- upgrade compatibility and dependency constraints.

Generated capabilities remain declarative. Arbitrary SQL, code, or plugins are
not executed without an isolated build and approval pipeline.

## AI Capability Evolution

### AI Control Plane

The AI Gateway owns model groups, provider channels, policy, routing, quotas,
budgets, pricing, evaluation scores and fallback rules. Business modules request
a capability such as `reasoning.high`, `vision.document`, or
`embedding.multilingual`; they should not hardcode a provider model name.

Routing decisions consider:

- capability and context-window requirements;
- tenant policy and data residency;
- quality and task-specific evaluation scores;
- latency, availability and rate limits;
- estimated and remaining budget;
- safety classification and tool requirements.

### Verified Context

The verified context engine remains the only path for business-grounded
assistant context. Retrieval becomes a pipeline:

1. authorize logical entities and fields;
2. resolve current physical mappings;
3. query canonical relational data;
4. redact or mask sensitive values;
5. rank verified facts and derived retrieval results;
6. attach lineage, freshness and omission diagnostics;
7. persist a reproducible context package.

Vector retrieval can improve discovery but cannot override relational truth,
permissions, or business validation.

### Agent Runtime

Agents use planner, executor and reviewer roles with explicit budgets and stop
conditions. High-risk actions always pass through Tool Runtime and human or
policy approval. Agent identity, delegated authority, tool calls, model calls,
context packages, proposals, approvals, outputs and costs share one trace.

Streaming model and assistant responses use endpoint-specific write-deadline
control plus bounded invocation contexts. Non-streaming calls and SSE streams
have separate deployment ceilings, provider `timeout_ms` can only reduce those
ceilings, and timeout versus client disconnect is recorded as a
different reliability outcome.

Long-term memory is separated into:

- immutable business facts referenced from source records;
- session working memory;
- user or organization preferences;
- proposed knowledge awaiting review;
- evaluated reusable skills and procedures.

### Evaluation and Evolution

Every model route, prompt, context rule, tool, skill and agent workflow should be
versioned and evaluated. Evaluation includes correctness, business invariants,
permission leakage, hallucination, tool selection, latency and cost.

Production feedback may propose new rules or skills, but promotion requires a
quality gate, regression suite, approval, canary release and rollback plan.
Self-evolution means governed proposal and selection, not autonomous production
code or schema mutation.

## Security and Governance

- Platform role and tenant authority are independent dimensions.
- Platform users require explicit `tenant.data.read` or `tenant.data.manage`
  permission before entering a tenant business context.
- Tenant modules and entitlements are checked after tenant access permission.
- Business actions enforce resource-level policy and authority.
- Security-kernel requests require freshness and replay protection.
- Secrets move from environment defaults to a secret manager in production.
- Login, access-token, invitation and model endpoints require rate limits.
- Sensitive management writes use shared actor and client-IP buckets; public
  invitation acceptance uses IP and invitation-token buckets; compatible AI
  Gateway traffic uses access-token and IP buckets. Storage failure fails closed.
- OpenAI-compatible discovery and unsupported-operation responses authenticate
  the same access-token lifecycle used for invocation and filter model discovery
  by token policy plus model-group channel abilities.
- OpenAI-compatible streaming uses protocol-native SSE chunks while retaining
  security-kernel authorization, quota reservation, actual-usage settlement,
  channel release, and timeout/disconnect audit semantics.
- Balance reservations are attached to invocation records. A leased,
  multi-replica recovery worker refunds pre-invocation orphans and settles or
  cancels stale attached invocations after backend interruption, preventing
  permanent balance, token-quota, and provider-channel occupancy leaks.
- Provider retry policy is operational rather than catalog-only: provider
  `retry_count` is capped by deployment policy, shares one total timeout across
  attempts, and retries only explicit throttle or gateway-unavailable responses.
  Ambiguous transport failures remain terminal to avoid duplicate inference and
  billing.
- Five-stage Business AI hashes the authoritative project overview at analysis
  time and recomputes it before proposal submission. Changed project facts fail
  closed before tool execution or approval creation, forcing a fresh analysis
  instead of applying an otherwise valid proposal to stale business state.
- Audit records use structured events, tamper evidence and retention policy.
- Sensitive context and logs use field classification and redaction.

## Frontend UI Evolution

### Information Architecture

Split the current single page into two explicit applications within one Next.js
deployment:

```text
/tenant/[organizationId]/overview
/tenant/[organizationId]/organization
/tenant/[organizationId]/erp/[module]/[document]
/tenant/[organizationId]/assistant
/platform/overview
/platform/organizations
/platform/industry-solutions
/platform/models
/platform/runtime
/platform/governance
```

The login surface selects the application boundary. Platform and tenant tokens,
navigation, permissions and current context remain visually distinct.

The first routing stage now publishes canonical `/platform/<workspace>` and
`/tenant/<organizationId>/<workspace>` deep links while reusing the established
workbench component. Login, navigation, refresh, organization validation, and
browser history synchronize through these URLs. This is a compatibility step;
platform and tenant layouts, server read models, and nested ERP document routes
still need to move out of the shared client page in later stages.

### Operational Design System

The target UI is a dense, quiet operational workspace inspired by professional
ERP systems, without reproducing legacy desktop limitations.

- Persistent left navigation for modules and document types.
- Stable top context bar for organization, environment, role and global search.
- Master/detail document workbench with fixed actions and status transitions.
- Tables optimized for scanning, keyboard navigation and repeated actions.
- Drawers for contextual editing and operation execution.
- A right-side AI assistant that inherits the current route and selected record.
- Approval, risk and provenance indicators close to the affected action.
- No decorative dashboard cards where a table, timeline or status band is more
  useful.

### Frontend Architecture

- Use App Router routes and layouts to isolate platform and tenant bundles.
- Keep server components for shells and initial read models where useful.
- Keep interactive workbenches as focused client components.
- Introduce a query cache for server state and invalidate it by command result or
  event, instead of maintaining large duplicated local state graphs.
- Generate API types and stable error codes from the backend contract.
- Split `api.ts`, `i18n.tsx`, `operations.ts`, and `page.tsx` by domain.
- Load heavy workspaces lazily to reduce the initial JavaScript bundle.
- Store filters and selected records in URLs so views are shareable and
  recoverable.

### AI-Native Interaction

The assistant is not a generic chat window. It should expose:

- current organization, module, document, selection and permission context;
- verified facts and omitted fields;
- proposed commands with impact preview;
- approval requirements and expected generated records;
- tool and action timeline;
- cost, model route and evaluation diagnostics when permitted.

Users can invoke the same business command from a form, command palette, API,
assistant proposal, or agent. All paths converge on the same backend action and
policy contract.

### Accessibility and Internationalization

- All visible strings and error fallbacks use stable Chinese and English keys.
- Status values, document labels, operation metadata and generated workspaces
  carry bilingual display metadata.
- Keyboard navigation, focus order, contrast, table semantics and screen-reader
  labels are release requirements.
- Locale formatting is applied to dates, numbers, currency and relative time.

### Frontend Verification

Source-string checks remain useful contract guards but are not UI tests. Add:

- component tests for forms, document actions and permission states;
- API integration tests for tenant headers and platform paths;
- Playwright flows for login, onboarding, ERP loops and platform management;
- visual regression at desktop and mobile operational viewports;
- bundle budgets and accessibility checks in CI.

## Observability

Adopt OpenTelemetry-compatible traces, metrics and logs across HTTP, database,
jobs, model calls, tool calls and business actions. The existing observability
domain can remain the business-facing ledger while exporters provide operational
telemetry.

Minimum service-level indicators:

- API availability and latency by route and tenant;
- tenant provisioning success and duration;
- projection lag and failed events;
- ERP action success, replay and compensation rates;
- assistant completion, approval and tool failure rates;
- model quality, latency, fallback and cost by capability;
- database connection utilization by platform and tenant pool.

## Staged Delivery

### Phase 0: Correctness Foundation

- Make local SaaS Compose startup deterministic.
- Separate platform tenant read and manage permissions.
- Restore frontend/backend route contract for tenant AI and Meta Resource.
- Run fresh platform and tenant migration verification in CI.
- Document target architecture and ownership.

### Phase 1: Application Boundaries

- Add module registry and configuration validation.
- Split identity self-service from platform identity management.
- Split frontend platform and tenant route layouts.
- Add stable error codes and generated API contracts.

### Phase 2: Reliable Tenant Runtime

- Move tenant provisioning to persistent jobs.
- Add bounded tenant connection-pool management.
- Add collision detection and governed remediation.
- Add tenant outbox and platform projection workers.

### Phase 3: Business Action Platform

- Move critical ERP and project mutations to the action registry.
- Add optimistic concurrency, compensation and policy metadata.
- Generate workbench actions and agent operations from action contracts.

### Phase 4: AI Runtime and Evaluation

- Route by model capability rather than model name.
- Add reproducible evaluation datasets and quality gates.
- Add prompt, context, tool and skill version promotion.
- Add trace-linked cost, safety and business outcome evaluation.

### Phase 5: Industry Capability Marketplace

- Sign and version capability bundles.
- Add dependency resolution, upgrade diff, canary apply and rollback.
- Generate tenant UI, actions, context and verification from approved manifests.

### Phase 6: Selective Service Extraction

Extract only modules that demonstrate a measurable need. Likely candidates are
AI Gateway, asynchronous job execution, projection/event processing, and tenant
database provisioning. Identity and policy may require stronger isolation later.

## Success Criteria

- No platform role grants implicit unrestricted tenant access.
- Platform and tenant data ownership is explicit and testable.
- Tenant provisioning is resumable and does not depend on one HTTP request.
- Dashboard and Meta-Org data is projection-backed with measured freshness.
- Every critical mutation has idempotency, policy, audit and verification.
- Frontend platform and tenant bundles have separate routes and permissions.
- AI actions use verified context and the same business command contracts as
  human UI and public APIs.
- Fresh database, upgrade, backend, frontend, Rust and end-to-end verification
  run in CI before release.
