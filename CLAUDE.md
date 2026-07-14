# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

Meta-Org is an AI-native organization operating platform: a multi-tenant SaaS where human employees, AI agents, and external collaborators are co-managed alongside org structure, project delivery, governance, and an ERP/industry-solution business core. Backend is a Go domain-modular monolith; frontend is a Next.js single-page workbench; a separate Rust security kernel handles cryptographic governance. Deeper prose references (in Chinese/English) live in `README.md` / `README_EN.md`; contributor conventions live in `AGENTS.md`.

## Commands

Backend (`cd backend`):
- `go run ./cmd/server` — run the API on `:8080`. Outside Docker you MUST set `MIGRATIONS_PATH=../migrations` and point `DATABASE_URL`/`PLATFORM_DATABASE_URL` at the `meta_org_saas` platform DB.
- `go build ./cmd/server` — compile.
- `go test ./...` — run unit tests. Files named `*_integration_test.go` (and some `service_test.go`) are **skipped unless a gating env var is set** — e.g. `RUN_RUNTIME_DB_TEST=1` with `RUNTIME_TEST_DATABASE_URL=...`. Grep the test file for its `os.Getenv(...)` guard before assuming a test ran.
- Single test: `go test ./internal/domain/runtime -run TestRuntimeOperationRepository`.
- `gofmt` before committing Go.

Frontend (`cd frontend`, run `npm install` first):
- `npm run dev` — Next.js dev server on `:3000` (`NEXT_PUBLIC_API_URL` defaults to `http://127.0.0.1:8080/api/v1`).
- `npm run build` / `npm run lint` (eslint).
- `npm run test:e2e` — Playwright; **requires backend `:8080` and frontend `:3000` already running**. `npm run test:e2e:ui` for the UI runner.
- `npm run test:*` (e.g. `test:erp-operations`, `test:unified-workbench`, `test:system-admin`) — standalone Node verification scripts (`verify-*.mjs`) that drive a running stack.

Full stack: `docker compose up --build` (PostgreSQL `:5432`, backend `:8080`, frontend `:3000`). On Windows without Docker, see the PowerShell `Start-Process` / `Set-Item Env:` recipe in `README.md` — do **not** use `$env:NAME="value"` inside nested PowerShell commands (it resolves too early and breaks `MIGRATIONS_PATH`/URLs).

Security kernel (`cd security-kernel`): Rust (`cargo build`, `cargo run`); the Go backend calls it over HTTP via `internal/pkg/securitykernel`.

Local dev SaaS admin login (local verification only): `platform-admin@local.test` / `MetaOrgSaasDev!2026` — on `http://127.0.0.1:3000` choose the **SaaS management** login entry, not the organization console entry.

## Architecture

### Split-database multi-tenancy (read this first)
This is the defining constraint of the system. There are two tiers of PostgreSQL databases:
- **Platform control DB** `meta_org_saas` — SaaS management, platform accounts/permissions, tenant-org catalog, subscriptions/modules, tenant-database provisioning targets, cross-tenant read projections. Reached via `PLATFORM_DATABASE_URL` (falls back to `DATABASE_URL`).
- **Per-tenant physical business DBs** `meta_org_xxxx` — one per tenant, where `xxxx` is the first 4 lowercase hex chars of the tenant org UUID (hyphens removed). Hold that tenant's ERP, project, workflow, finance, costing, inventory/procurement/sales/retail/manufacturing runtime with **no cross-database foreign keys**.

Tenant DBs are created and migrated **only** by the backend tenant provisioner/migrator (`internal/pkg/tenantdb`), never by hand. `tenantdb.PoolRouter` implements the `tenantdb.DB` interface and transparently routes every query to the correct tenant DB using the tenant resolved from the **request context**. Repositories accept a `tenantdb.DB` and are database-agnostic — they never know which physical DB they hit.

Request pipeline (`internal/gateway/router.go`, all under `/api/v1`): `APIErrorContract` → public routes (`/health`, `/auth/*`, `/agents/auth`, `/roles`) OR `AuthMiddleware` (JWT) → then either **platform-scoped** routes (`/platform/admin/*`, gated by `PlatformPermissionMiddleware`) or **tenant-scoped** routes wrapped in `TenantMiddleware`. `TenantMiddleware` resolves the `X-Organization-ID` header into a `TenantContext` (mode, org, platform role, enabled modules, tenant DB target), enforces module gating and platform-vs-tenant access rules, and stores it in context for the PoolRouter and services downstream.

### Domain-modular monolith
Backend entry point `backend/cmd/server/main.go` loads config, connects the platform DB, runs migrations, wires every domain's repository/service/handler, starts background workers, and registers routes. Each domain lives in `backend/internal/domain/<domain>/` with the standard four-file layering — **keep it**: `handler.go` parses HTTP, `service.go` holds business rules and cross-domain orchestration, `repository.go` does persistence, `model.go` defines API/DB shapes. Shared infra is in `backend/internal/pkg/` (`config`, `database`, `tenantdb`, `middleware`, `securitykernel`, `authlimit`, `authorization`, `platformauth`, `secretbox`, `server`, ...). Domains include `identity`, `organization`, `saas`, `systemadmin`, `metaorg`, `metaresource`, `aigateway`, `toolruntime`, `businessai`, `assistant`, `runtime`, `erp`, `finance`, `costing`, `inventory`, `procurement`, `sales`, `industry`, `project`, `workflow`, `governance`, `evolution`, `observability`, `verification`, `dashboard`, `tenantprojection`, `capability`, `layer`.

Background workers started in `main.go` include audit retention, tenant projection (tenant outbox → platform read projections), and AI Gateway reservation recovery — all tuned by `TENANT_PROJECTION_*` / `AI_GATEWAY_RESERVATION_*` / `AUDIT_RETENTION_*` env vars (documented in `README.md`).

### Migrations are staged baselines, auto-run at startup
Platform migrations in `migrations/` run in filename order on backend boot. They are **restructured staged baselines**, not incremental history: `000` (SaaS platform) → `001` (ERP code baseline) → `002` (ERP-platform integration) → `004` (AI capability), followed by numbered repair/feature migrations. Tenant-DB migrations live separately in `migrations/tenant/` and use `tenantdb:include` directives that expand the ERP/finance baseline into each tenant DB — this is why running `psql -f migrations/tenant/001_*.sql` by hand is wrong (the include never expands). `migrations/BASELINE_RESTRUCTURE.md` is the source-of-truth governance doc.

**Any change to schema, table relationships, foreign keys, indexes, seed data, or schema-generation logic must update the corresponding staged SQL in the same change** — not just backend/frontend code — and be verified against a fresh migrated database. Startup errors like `relation model_provider_channels does not exist` or missing `MITW`/`MPOR` code-tables almost always mean `MIGRATIONS_PATH` is wrong, the backend is pointed at an old DB, or migrations haven't run.

### ERP code-table business model
Supply-chain and industry solutions are modeled as **ERP master/detail code-table pairs**, not bespoke semantic tables: purchase `MPOR/POR1`, sales `MRDR/RDR1`, warehouse balance `MITW/ITW1`, stock in/out `MIGN·IGN1`/`MIGE·IGE1`, retail distribution `MRPS/DRQ/DSP/...`. Fresh tenant baselines deliberately do **not** create legacy tables like `purchase_orders`/`sales_orders`/`inventory_balances`. New industry modules, UI workbenches, and agent-facing APIs must go through code-tables; legacy semantic APIs survive only as compatibility/migration layers.

### AI + governance stack
- **AI Gateway** (`aigateway`): multi-provider model routing (OpenAI/Anthropic/Gemini), channel/key pools with per-channel circuit breaking, encrypted secrets (`MODEL_SECRET_KEY`), multi-dimensional billing with balance reservations, and an OpenAI-compatible surface — `POST /v1/chat/completions` supports `stream: true` SSE.
- **Tool Runtime** (`toolruntime`, `runtime`): tool registration, governance decisions, approval policies, and audited execution; internal tools adapt backend operations (e.g. `runtime/finance_adapter.go`).
- **Business AI** (`businessai`, `businessaibridge`): project-stage AI over Plan/Do/Change/Accept/Learn, producing structured results and **tool proposals that must pass Tool Runtime approval before idempotent execution and result write-back**.
- **Security Kernel**: standalone Rust service (axum, ed25519/HMAC signatures, nonce replay ledger) invoked by the Go backend for cryptographic governance decisions; backend health surfaces `security_kernel.status`.

### API error contract
`/api/v1` responses carry a stable `code` and `request_id` (mirrored in `X-Request-ID` / `X-Error-Code`) alongside a legacy free-text `error` field. Clients (and any new frontend/agent code) must branch on `code` and correlate via `request_id` — never parse the free-text message. The OpenAI-compatible `/v1` surface follows its own protocol.

## Conventions

- **Frontend i18n is mandatory.** All user-facing text — labels, placeholders, validation/error fallbacks, buttons, badges, table headers, menu labels, empty states, panel titles, **and API operation names + parameter labels** — must go through `LanguageProvider`/`useI18n` (`frontend/src/lib/i18n.tsx`) with both `zh` and `en`. New modules use stable translation keys; never hardcode visible strings. This includes the API Workbench operation metadata in `frontend/src/lib/operations.ts`.
- **Frontend style:** TypeScript + React + Tailwind, two-space indent, single quotes, no trailing semicolons, `@/*` import alias for `frontend/src/`.
- **Tenant DB naming is a contract:** names are generated by the backend tenant-database helper (prefix `TENANT_DATABASE_NAME_PREFIX`, default `meta_org_`), never by ad-hoc string building in handlers or SQL. `TENANT_DATABASE_ADMIN_URL` must point at an admin DB (e.g. `postgres`), not the platform DB. Do not restore the old single `meta_org` DB as the active runtime DB.
- **`general`-industry orgs** may enable base governance modules plus the ERP/industry closed-loop modules (`erp`, `finance`, `costing`, `inventory`, `procurement`, `sales`, `retail`, `manufacturing`); a `module ... not allowed by industry general` error means the baseline seed + backend policy + SystemAdmin frontend entry are out of sync.
- **File deletion:** never batch-delete files or directories with scripts. Delete one file at a time (`Remove-Item`); if bulk deletion seems necessary, stop and ask the user.
- Config is environment-driven in `backend/internal/pkg/config/config.go`; override `DATABASE_URL`, `PLATFORM_DATABASE_URL`, `JWT_SECRET`, `MODEL_SECRET_KEY`, `CORS_ORIGINS`, `NEXT_PUBLIC_API_URL`, and the `TENANT_DATABASE_*` set per environment (full table in `README.md`).
