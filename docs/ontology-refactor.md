# Operational Ontology Refactor

## Scope and evidence

Meta-Org combines a SaaS control plane, tenant ERP, organization/project
management, and an AI gateway. The useful core is an organization in which
people and agents operate on the same business objects under explicit
permissions. Palantir's Ontology provides the architectural reference:
object types and properties describe business state, links describe business
relationships, and actions implement governed changes to that state.

Reference: https://www.palantir.com/docs/foundry/ontology/overview/
This implementation is independent of Palantir Foundry and requires no
Palantir services or SDK license.

The September 2026 code inspection found these concrete gaps:

| Area | Existing behavior | Required behavior |
| --- | --- | --- |
| Storage | ERP code tables coexist with obsolete semantic supply-chain repositories whose tables are absent from fresh baselines | ERP code tables are authoritative; remove obsolete routes from the running application |
| Accounting | Business actions write MJDT/JDT1 while the finance workbench reads gl_journal_entries | Finance APIs and ERP actions use one ledger |
| Procurement | Receipt creates an A/P invoice, but no invoice posting or outgoing payment action exists | Purchase order, receipt, liability, payment, and reconciliation form one traceable flow |
| Sales | Incoming payment accepts arbitrary targets and can exceed invoice balance; a zero payment balance resets to the original amount | Restrict targets, validate balances/currency/partner, and record each allocation exactly once |
| Actions | Business transaction commits before audit completion; failed retries collide with a unique key; unsupported registered actions report success | State and successful audit commit together; retries and unsupported actions have explicit outcomes |
| Inventory | Read-modify-write balances have no serialization and generated movement documents can be posted twice | Serialize ledger mutations, maintain valuation, and mark generated movements as posted |
| AI | OpenAI base paths duplicate /v1; streamed tool JSON is treated as complete fragments; Anthropic discards system prompts; Gemini places secrets in URLs | Protocol-correct adapters with contract tests and bounded tool execution |
| Navigation | Framework administration and overlapping representations compete with business operations | Expose core business objects and workflows first; keep platform administration in its existing scope |

The pre-change `go test ./...` suite passes. PostgreSQL integration tests are
opt-in, so that result alone does not establish a working business loop.

## Boundaries

- `meta_org_saas` owns organizations, memberships, entitlements, AI provider
  configuration, tool approval, and platform governance.
- A provisioned `meta_org_xxxx` database owns the single organization's ERP
  objects, inventory, journal, allocations, and action provenance.
- The ontology is a semantic view of authoritative ERP records. It does not
  introduce a second graph database or duplicate business object store.
- Object queries, links, actions, the human workbench, and AI tools share the
  ERP application service and its transaction/permission rules.
- AI may query accessible objects. Business changes go through the existing
  Tool Runtime approval mechanism before invoking the same business actions.
- Existing project/workflow/governance behavior remains available. Unused
  semantic supply-chain services are disconnected from runtime rather than
  deleting historical customer data or unrelated working-tree changes.

## Verification contract

Validate purchase-to-pay and order-to-cash against a fresh, provisioned tenant
database. Assert stock and valuation, balanced journals, receivable/payable
settlement, provenance, repeat requests, over-allocation rejection, and rollback
after a failed action. Test model adapters against protocol fixtures including
fragmented streaming tool calls. Live provider verification requires locally
configured credentials and is reported separately from contract tests.

Migration ownership and upgrade instructions are maintained in
`migrations/BASELINE_RESTRUCTURE.md` alongside the staged SQL.

## Implemented architecture

```text
Human document workbench                 AI assistant / Business AI
        |                                        |
        |                              Provider protocol adapter
        |                                        |
        |                              Tool Runtime + reviewer gate
        |                                        |
        +-------------- Ontology API -------------+
                              |
                  ERP application service
                  permissions + state validation
                              |
             tenant-local PostgreSQL transaction
             objects + stock + journal + provenance
```

The catalog exposes 19 object types. Master data, supply-chain documents,
settlements, accounting, requirements, and projects retain their existing ERP
identities. The tenant database now stores normalized object, property, link,
and action definitions, loaded through `ontology/repository.go`;
`ontology/service.go` resolves links from source keys
and payment allocations. There is no eventually consistent graph copy to repair.

The object API lives below `/api/v1`:

| API | Purpose |
| --- | --- |
| `GET /ontology/types` | Discover accessible types, properties, links, and actions |
| `GET /ontology/types/{type}` | Read a type's bilingual operation metadata |
| `POST /ontology/objects/{type}/query` | Filter properties with bounded keyset pagination |
| `GET /ontology/objects/{type}/{key}` | Read one authoritative object |
| `GET /ontology/objects/{type}/{key}/links` | Traverse authorized business relationships |
| `GET /ontology/objects/{type}/{key}/history` | Inspect action and generated-document history |
| `POST /ontology/objects/{type}/{key}/actions/{action}` | Execute a governed state transition |

Example query body for `purchase_order`:

```json
{"filters":{"partner":"SUP-001"},"limit":50}
```

An action accepts `{"data":{...}}` and an `Idempotency-Key` header. Reusing a
completed key with changed arguments is rejected. Explicit retries of failed
actions use `data.retry_failed = true`; completed effects are never repeated.
Actor identity comes from authentication, not caller-supplied provenance fields.
Draft forms continue to use `/erp` CRUD through the same application service.

### Accounting and supply chain

```text
Purchase order -> approved receipt -> inventory + draft payable invoice
                                      |                 |
                                inventory / GRNI   posted liability
                                                        |
                                              outgoing allocations

Sales order -> approved delivery -> inventory issue + draft receivable invoice
                                      |                      |
                              cost of sales / stock      posted receivable
                                                             |
                                                    incoming allocations
```

Receipt posting debits inventory and credits goods received not invoiced.
Payable posting clears that accrual, recognizes input tax, and credits A/P.
Outgoing allocations debit A/P and credit cash. Delivery posting recognizes
inventory cost at the item/warehouse weighted average. Receivable posting debits
A/R and credits revenue and output tax; incoming allocations clear A/R to cash.
Default posting accounts are seeded by `001` and its `030` upgrade.

Every generated journal must balance before commit. Invalid numeric values,
missing accounts, inactive/non-postable accounts, currency mismatches, negative
stock, and over-allocation reject the whole transaction. Allocation validates
both the invoice balance and the payment balance, along with partner identity.
Successful audit completion is part of the transaction, not a later best-effort
write. Failed diagnostic logging is serialized against concurrent retries.

The compatibility `/finance/gl` APIs now query `erp_gl_*` views over this same
ledger. Trial balance aggregates all posted entries in the selected period and
currency; it is not a sum of the current paginated document list. Manual journal
creation/posting and the legacy UUID-based lookup use these same records.

### Removed duplication

- Disconnected legacy semantic inventory/procurement/sales handlers from the
  server. The maintained workflow uses ERP objects exclusively.
- Retired legacy receivable/payable/payment endpoints with HTTP 410 instead of
  retaining a second writable accounting model.
- Replaced duplicated document editors with one bilingual workbench containing
  draft editing, line editing, action confirmation, links, and action history.
- Removed the unused status sidebar from document workspaces. Historical
  retail/manufacturing screens live in a collapsed, read-only archive group.
- Removed quantity-only industry mutation implementations from the action
  registry. New industry packages archive their unsupported workflows and
  skills rather than advertising nonexistent actions.
- Industry tool assets now reference existing platform tools. Applying a package
  cannot manufacture an executable adapter, overwrite its input schema, or
  downgrade its approval policy. Old generated aliases are disabled by `031`.

Historical business rows, old financial tables, and unrelated user changes are
not deleted. Project/workflow/governance capabilities remain available, including
the existing five-stage Business AI review and proposal flow.

## External Document Intake

The document workbench includes a table with server-side status filters,
semantic property sorting, stable keyset pagination, matching record counts,
and a collapsible detail area. Its external-document inbox accepts original
images, PDFs, DOCX, UTF-8 text, and CSV with supporting files. A review workspace
keeps original previews beside editable semantic fields and line items, with
source excerpts, estimated confidence, and a correction review before confirmation.

Recognition creates a proposal only. Standard CSV can be parsed locally; other
documents use the existing governed AI Gateway, including image/PDF payloads
for its OpenAI-compatible, Anthropic, and Gemini adapters. Provider credentials
stay in the control plane. Human confirmation creates a draft through the same
ERP application service and records an immutable source trail. Inventory,
approval, journal, and payment actions still require their normal transitions.

Detailed contracts, limits, and configuration are in `docs/document-imports.md`.

## Model API integration

Three protocol adapters implement non-streaming responses, streaming responses,
and function/tool calls:

| Protocol | Presets |
| --- | --- |
| OpenAI Chat Completions compatible | OpenAI, DeepSeek, Qwen/DashScope, Kimi/Moonshot, Grok/xAI, OpenRouter, Doubao/Ark, SiliconFlow, Ollama |
| Anthropic Messages | Anthropic |
| Gemini generateContent | Gemini |

The configured base URL is an API root, including any vendor-specific prefix.
For example, `/compatible-mode/v1` and `/api/v3` are preserved instead of receiving
an extra `/v1`. Provider configuration and key rotation remain platform-owned;
tenant catalog responses exclude provider credentials and internal routing data.

Tool names are mapped to wire-safe identifiers and restored after the provider
response. Streaming adapters assemble fragmented arguments before execution,
preserve tool-call identity, reject truncated streams, and retain provider
continuation fields where needed. Anthropic keeps system instructions separate
from message history. Gemini uses a credential header, not an API key URL query.

The shared AI business tools are `ontology.types.list`, `ontology.objects.query`,
`ontology.objects.get`, `ontology.objects.links`, and `ontology.action.execute`.
Mutations require reviewer approval even if stored tool metadata is weakened.
The approved adapter then enters the same ERP transaction path as the human UI.

### Configure a real provider

1. Enter the SaaS management login, then open AI models and API access.
2. Select a preset or custom API root and supply a vendor key. Do not put secrets
   into source files or frontend environment variables.
3. Create the provider and test it with an explicit model ID that the account can
   access. Configure the model catalog entry, capabilities, and prices as needed.
4. Select that model in the tenant assistant or Business AI workbench. Review
   proposed business changes through Tool Runtime approval.

For a backend running in Docker, host-local Ollama needs a reachable host URL
such as `http://host.docker.internal:11434/v1`; `localhost` refers to the backend
container. The current provider form requires a nonempty key, so an unauthenticated
local-compatible server can use a local placeholder accepted by that server.
Use separate strong `MODEL_SECRET_KEY` and `JWT_SECRET` values for deployment.

## Deliberate limits

- This is an independent Ontology-inspired implementation, not a Palantir
  Foundry integration or an implementation of every Foundry capability.
- Order fulfillment currently creates a full-order receipt/delivery. Partial
  payments and multiple allocations are supported; partial shipments, returns,
  credit notes, and reversal workflows are not part of this completed slice.
- Payment allocation records accounting settlement. It does not send an external
  bank transfer, collect money, or implement bank statement reconciliation.
- The ledger does not yet implement exchange-rate conversion, fiscal-period
  locking, or jurisdiction-specific tax compliance. Amounts use six decimal
  places; inventory valuation is per item/warehouse/currency.
- Existing unvalued inventory requires a reviewed carrying-value adjustment.
  The migration deliberately does not invent historical cost.
- Duplicate legacy account codes across organizations and conflicting UUID
  ownership stop migration for explicit reconciliation. Void journals remain
  immutable and cannot be reposted.
- Provider tests use protocol fixtures and a local HTTP model fixture. No real
  vendor credentials, account entitlements, live model availability, or external
  billing were verified. Compatibility depends on the selected model supporting
  the advertised protocol and tools; Responses-only and multimodal endpoints are
  not included in these adapters.

## Verification results (2026-09-10)

All checks below passed against this implementation. Go and PostgreSQL checks
used Docker (`golang:1.22` and PostgreSQL 16); frontend checks used the local
Node.js toolchain. Integration databases were isolated from the running demo.

| Check | Result |
| --- | --- |
| Backend `go test ./...` | Passed, including Ontology, ledger validation, tool approval, and provider protocol contracts |
| Race detector | Passed for `erp`, `aigateway`, and `toolruntime` |
| Fresh platform migration | Passed, including foreign-key validation and tracked migration checksums |
| Fresh provisioned tenant | Passed with expanded `tenantdb:include` directives and tenant-local ERP/finance tables |
| Pre-Ontology upgrade | Passed for platform and tenant databases, preserving balances, UUID lookups, void journals, and checksum history; ambiguous ownership is rejected |
| PostgreSQL commerce integration | Passed for stock valuation, purchase/payment and sales/receipt accounting, idempotency, authorization, concurrent mutations, and transaction rollback |
| Frontend lint and production build | Passed after the final form-state and archive-boundary fixes |
| Frontend source contracts | All 10 scripts passed; the ERP business contract was rerun after the archive fix |
| Playwright | **14 passed** across desktop and mobile Chromium |
| Working-tree whitespace check | `git diff --check` passed |

Playwright covers both business loops, partial payment allocation, locked posted
documents, linked objects, bilingual action history, balanced trial balance and
CSV export. It also covers the five-stage project AI workflow, stale-context
rejection, proposal approval/execution, provider creation/connectivity, login
scope isolation, and route reload/back/forward behavior. The archive regression
opens the same inventory and journal drafts through both archive and core
navigation: archive forms/actions are read-only, while core entry points remain
editable. Desktop/mobile screenshots and traces are generated in
`frontend/test-results/`; document layouts and provider configuration were
visually inspected for clipping and overlap.

For the database suite, the existing isolated test instance is
`meta-org-ontology-postgres` (host port `55432`). Run from the repository root:

```powershell
docker run --rm --network container:meta-org-ontology-postgres `
  -e RUN_FRESH_DB_MIGRATION_TEST=1 `
  -e RUN_FRESH_TENANT_DB_MIGRATION_TEST=1 `
  -e RUN_COMMERCE_DB_TEST=1 `
  -e MIGRATION_TEST_ADMIN_URL=postgres://postgres:postgres@127.0.0.1:5432/postgres?sslmode=disable `
  -v D:/project/meta-org-saas:/workspace `
  -v meta-org-ontology-gomod:/go/pkg/mod `
  -v meta-org-ontology-gocache:/root/.cache/go-build `
  -w /workspace/backend golang:1.22 `
  go test -p 1 ./internal/pkg/database ./internal/pkg/tenantdb ./internal/domain/erp -count=1
```

The integration tests create and drop temporary databases. Use a disposable
PostgreSQL instance with database-creation permissions, never a production
administrative connection. Without the three opt-in environment flags, ordinary
`go test ./...` skips these PostgreSQL tests.

## Local runtime

The local runtime uses Compose project `meta-org-ontology-final` for PostgreSQL
(`5432`), backend (`8080`), security kernel (`8090`), frontend, and Caddy. Open
**http://127.0.0.1:3000**; Caddy forwards `/api/v1` to the backend. PostgreSQL,
backend, and security-kernel health checks pass. Older verification containers
and images have been removed; their database volumes were retained. No old
`meta_org` database was restored as an active runtime.

To start the same stack from the repository root when it is not already running:

```powershell
docker compose -p meta-org-ontology-final up -d --build
```

For frontend hot reload alongside that stack, use a separate local port:

```powershell
$env:API_PROXY_TARGET = 'http://127.0.0.1:8080'
npm --prefix frontend run dev -- --hostname 127.0.0.1 --port 3100
```

The browser now uses the same-origin `/api/v1` path. The development server
forwards it to `API_PROXY_TARGET`; container deployments use Caddy. See the
[deployment guide](../deploy/README_EN.md) for the complete container runtime and
production HTTPS configuration.

Do not start a second frontend on an occupied port. Cross-origin API clients
still require their origins in backend `CORS_ORIGINS`. Use the same
Compose project to reuse this demo, and use a separate volume when validating a
fresh installation. Do not delete migration history or edit checksums manually.

Select **SaaS management** for `platform-admin@local.test` with the local-only
password documented in `AGENTS.md`. Select **Organization console** for the
provisioned sample tenant `demo@local.com` / `MetaOrgSampleTenant!2026`. These are
development fixtures, not production credential defaults.

Run `npm --prefix frontend run test:e2e` against the running services for the
complete UI suite, or `npm --prefix frontend run test:ontology` for the business,
archive, and provider scenarios. The suite starts a local model fixture on
`18081`, provisions/reuses the sample tenant, and disables its test providers on
completion. A real provider must be configured separately before live AI use.
