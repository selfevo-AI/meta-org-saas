# Meta-Org - AI-Native Organization Operating Platform

English | [简体中文](README.md)

Meta-Org is an AI-native organization operating platform for hybrid human and AI-agent teams. It brings human employees, AI agents, external collaborators, organization structure, project delivery, governance rules, and continuous learning into one operating system. The current product flow covers requirement intake, project formation, workflow execution, deliverable acceptance, cost tracking, and feedback capture.

The project is built around the **ETCLOVG** framework: Execution, Tooling, Context, Lifecycle, Observability, Verification, and Governance. This repository currently includes a Go backend, a Next.js frontend, PostgreSQL migrations, Docker Compose orchestration, JWT authentication, Meta-Org Home, Meta Resource / PDCA workspace, organization and project workspaces, Model Settings, AI Gateway, tool runtime, cost accounting, generic finance, SaaS module gating, security-kernel integration, and inventory/procurement/sales supply-chain workflows.

## Product Goal

Meta-Org is not a simple task tracker. It focuses on how an organization can keep operating reliably after AI agents become active participants:

- **Humans and AI agents in one management model**: users, agents, and external members share identity, role, position, permission, and project assignment concepts.
- **Executable organization structure**: departments, positions, position assignments, MVRUs, workflow templates, and project members are used for scheduling, authorization, and evaluation.
- **Requirement-to-feedback lifecycle**: requirements can include uploaded documents, enter analysis workflows, be approved, convert into projects, bind members and workflows, track deliverables, record costs, and close feedback.
- **Governance embedded in workflows**: permissions, principles, control rules, risk levels, required permission levels, and decision weights participate in key project actions.
- **Self-evolution through records**: weight calculations, outcomes, experiments, knowledge entries, and signals preserve operational learning for future decisions.

## Core Concepts

| Concept | Description |
|---|---|
| ETCLOVG | Execution, Tooling, Context, Lifecycle, Observability, Verification, and Governance as seven organizational capabilities. |
| AI agents as first-class actors | Agents have identity, permission level, capabilities, origin, provider, risk level, and metadata, and can participate in projects and workflows. |
| MVRU | Minimal Viable Reconfigurable Unit, a small reconfigurable organization unit used to model adjustable structure, members, and relationships. The current API path uses `/muvrs`. |
| P-E-R workflow | Workflow templates and instances composed of Planner, Executor, and Reviewer stages, with tasks, decisions, and context. |
| Meta Resource | A unified meta-level index for humans, external humans, agents, model channels, tools, materials, time, capabilities, budgets, and generic resources, with capability, cost, capacity, and risk profiles. |
| Demand Profile | A demand-side profile containing goals, acceptance criteria, required capabilities, budget/time/risk constraints, and resource-fit candidates. |
| PDCA Cycle | Explicit Plan, Do, Change, and Accept records around a demand profile, including decisions, evidence, and next actions. |
| Decision weight | A score for a human or agent based on capability, history, risk, and organization context. |
| Governance access decision | Access decisions are based on permissions, principles, control rules, risk level, required permission level, and weight snapshots. |
| Capability matching | Matching humans, agents, or capability resources by capability needs, risk, context, and candidates. |
| Self-evolution loop | Signals, experiments, verification, knowledge capture, and weight updates form a continuous improvement cycle. |

## Current Capabilities

### Business Lifecycle

The system currently supports a full project lifecycle:

1. **Requirement intake**: create requirements with organization, department, submitter, priority, risk level, budget, and metadata.
2. **Documents and analysis**: upload requirement documents, start requirement-analysis workflows, sync workflow output, and store analysis results.
3. **Requirement approval**: a human or agent actor approves requirements; approval can trigger governance checks and outcome recording.
4. **Project conversion**: convert approved requirements into projects while preserving organization, department, budget, risk, and context.
5. **Project staffing**: add project members, connect organization positions or position assignments, and match participants by capability and risk.
6. **Workflow binding**: bind workflow templates to projects, create workflow instances, and track tasks, decisions, and context.
7. **Deliverable management**: create, update, submit, accept, or reject deliverables.
8. **Cost management**: record cost entries, refresh cost summaries, and aggregate budget consumption by source type.
9. **Feedback evaluation**: create project evaluations, close feedback, and record outcomes and learnable signals in the evolution domain.

### Organization Capabilities

- Multi-organization model and current-organization lookup.
- Department tree, department status, ordering, and metadata.
- Positions, position permission levels, required capabilities, and position assignments.
- Human users, AI agents, and external members as organization members.
- Department-to-MVRU links.
- Connections between organization members, project members, positions, and position assignments.
- Member matching and capability matching for tasks and projects.

### Governance and Evolution

- Permissions, principles, control rules, and access decision records.
- Governance fields for AI agents: origin, service class, provider, contract reference, and risk level.
- Employee profiles, context weights, capability evaluations, and access-decision data structures.
- Weight computation, context-weight computation, outcome recording, experiments, knowledge entries, and signal acknowledgement.

### AI Operations, Tools, and Finance

- Meta-Org Home aggregates organization health, project status, agent status, AI cost, risks, recent events, and inbox items.
- AI Gateway supports OpenAI, Anthropic, and Gemini provider configuration, encrypted secrets, model catalog, streaming calls, invocation logs, and cost summaries.
- Tool Runtime supports tool registry, governance decisions, approval policy, execution audit, and internal tool adapters.
- Meta Resource supports syncing existing humans, external members, agents, model channels, tools, and capabilities into one capability/cost/capacity/risk profile layer.
- Demand Profile and PDCA Cycle make demand constraints, resource fit, planning, execution, change, and acceptance events explicit queryable objects.
- Model Settings covers model providers, model catalog, tool registry, interface files, invocation logs, and cost summaries.
- Finance Exports support generic finance adapters, HMAC/Bearer auth, export batches, webhook callbacks, and reconciliation differences.

### Frontend Workspaces

The frontend is an operational single-page workspace:

- Login, registration, session persistence, and logout.
- Chinese and English language switching through `LanguageProvider` and `useI18n`.
- Dashboard overview for identity, organization, workflow, capability, observability, verification, governance, evolution, and recent events.
- Meta-Org Home for organization health, AI cost, risks, inbox items, and contextual AI assistance.
- Meta Resource Workspace for resource summary, existing-resource sync, demand profiles, PDCA cycles, and PDCA events.
- Draggable sidebar menu groups: business lifecycle, organization capabilities, governance evolution, and system tools.
- Organization workspace for organizations, departments, positions, members, external members, position assignments, MVRU links, and matching.
- Control workspaces for governance, weights, capability evaluations, workflow design, and workflow matching.
- Project lifecycle workspace for requirements, projects, delivery, costs, and feedback.
- Model Settings for providers, models, tools, interface files, invocation logs, and cost summaries.
- Finance Exports for adapters, export batches, reconciliation, and failed callbacks.
- Context AI Assistant for Meta-Org, organization, project, governance, and model-setting contexts with streaming and cost display.
- API Workbench for browsing and calling backend APIs by domain, with path parameters, query parameters, request templates, and auth token support.

## Technical Architecture

| Layer | Current Implementation |
|---|---|
| Frontend | Next.js 16 App Router, React 19, TypeScript, Tailwind CSS, lucide-react, @xyflow/react |
| Backend | Go 1.22, Chi Router v5, modular domain-oriented monolith, pgx PostgreSQL driver |
| Database | PostgreSQL 16, root-level SQL migrations, automatically applied by the backend on startup |
| Authentication | JWT Bearer Token with separate public and protected route groups |
| Deployment | Docker Compose starts PostgreSQL, backend, and frontend |

### Backend Structure

The backend entry point is `backend/cmd/server/main.go`. Startup flow:

1. Load environment configuration.
2. Connect to PostgreSQL.
3. Run SQL migrations from `migrations/`.
4. Initialize repositories, services, and handlers for each domain.
5. Register `/api/v1` routes in `backend/internal/gateway/router.go`.
6. Start the HTTP server with graceful shutdown.

Backend domains live under `backend/internal/domain/<domain>/` and typically contain:

- `model.go`: API and database models.
- `repository.go`: PostgreSQL persistence.
- `service.go`: business rules and cross-domain orchestration.
- `handler.go`: HTTP request parsing and responses.

Shared packages live under `backend/internal/pkg/` and cover configuration, database access, migrations, middleware, and server setup.

### Backend Domains

| Domain | Responsibility |
|---|---|
| `identity` | Users, AI agents, roles, login, registration, and agent authentication. |
| `organization` | Organizations, department tree, positions, position assignments, external members, organization members, MVRUs, relationships, and matching. |
| `layer` | Strategic, tactical, and execution layer classification and MVRU layer configuration. |
| `capability` | Capability catalog, capability bindings, capability matching, and capability evaluations. |
| `dashboard` | Aggregated statistics and recent events for the system overview. |
| `metaorg` | Meta-Org Home, organization health, risks, activity, and inbox aggregation. |
| `metaresource` | Unified resource profiles, demand profiles, PDCA cycles, and PDCA event records. |
| `aigateway` | Model providers, model catalog, streaming calls, invocation logs, and AI usage cost. |
| `toolruntime` | Tool registry, governance policy, approvals, execution audit, and internal tool adapters. |
| `finance` | Generic finance adapters, export batches, webhook callbacks, and reconciliation. |
| `workflow` | Workflow templates, instances, tasks, decisions, and context. |
| `project` | Requirements, documents, requirement-analysis workflows, projects, members, project workflows, delivery, costs, and feedback. |
| `governance` | Permissions, governance principles, control rules, permission checks, and access decisions. |
| `evolution` | Decision weights, context weights, experiments, knowledge entries, signals, and outcome recording. |
| `observability` | Traces, spans, metrics, and execution telemetry. |
| `verification` | Verification reports, review assignments, review completion, and scoring. |

### Frontend Structure

| Path | Description |
|---|---|
| `frontend/src/app/page.tsx` | Main app entry, login/register, layout, overview, menu, and workspace switching. |
| `frontend/src/app/organization-workspace.tsx` | Organization, department, position, member, external member, and MVRU operations. |
| `frontend/src/app/control-workspaces.tsx` | Governance, weights, capability evaluations, workflow design, and workflow matching. |
| `frontend/src/app/project-lifecycle-workspace.tsx` | Requirement, project, delivery, cost, and feedback workspace. |
| `frontend/src/app/meta-resource-workspace.tsx` | Meta Resource, Demand Profile, and PDCA cycle workspace. |
| `frontend/src/app/api-workbench.tsx` | Generic API calling panel. |
| `frontend/src/app/ai-assistant.tsx` | Context AI Assistant and SSE streaming response panel. |
| `frontend/src/app/developer-tools-workspace.tsx` | Model, tool, interface file, invocation log, and cost views. |
| `frontend/src/app/finance-workspace.tsx` | Finance adapter, export batch, reconciliation, and failed callback views. |
| `frontend/src/lib/api.ts` | API request wrapper, base types, and dashboard data shapes. |
| `frontend/src/lib/operations.ts` | API Workbench domain, path, parameter, and body-template metadata. |
| `frontend/src/lib/i18n.tsx` | Chinese and English language packs and i18n provider. |
| `frontend/src/lib/auth.ts` | Token and session storage. |

## Database Migrations

The backend applies SQL files from the root `migrations/` directory at startup in filename order. The current release keeps only the restructured staged baselines:

| Migration | Topic |
|---|---|
| `000_saas_platform_management_baseline.sql` | SaaS management platform, platform accounts, permission governance, subscriptions, modules, tenants, and platform master/detail foundation. |
| `001_erp_code_baseline.sql` | ERP and industry business baseline, including organization, project, workflow, finance, costing, and supply-chain solution tables. |
| `002_erp_platform_integration_baseline.sql` | Runtime projection, module integration, and platform master-data synchronization between ERP and platform management. |
| `004_ai_capability_baseline.sql` | Models, providers/channels, agents, tool runtime, AI Assistant, context, skills, AI usage, and cross-stage foreign-key rebuilds. |

The stage principle is SaaS management platform first, then platform-created or platform-adjusted industry solutions, then the ERP baseline and AI capability baseline. Future database structure, relationship, foreign-key, index, seed-data, or schema-generation changes must update the matching stage SQL and `migrations/BASELINE_RESTRUCTURE.md`.

The `000` baseline seeds a `local_manufacturing_demo` industry-solution sample in the SaaS management catalog. It includes a sample tenant database template, the `sample_work_orders` sample table, and `sample.work_order.create` sample function metadata. The physical sample database is still created by tenant database provisioning or a later maintenance job, not directly inside the SQL baseline transaction.

## API Overview

All API routes are mounted under `/api/v1`.

Public routes:

- `GET /health`
- `POST /auth/login`
- `POST /auth/register`
- `POST /agents/auth`
- `GET /roles`

All other business routes require a JWT Bearer Token.

| Domain | Main Routes |
|---|---|
| Dashboard | `GET /dashboard/overview` |
| Meta-Org | `GET /meta-org/overview`, `GET /meta-org/inbox` |
| Meta Resource | `GET/POST /meta-resources`, `POST /meta-resources/sync-existing`, `GET /meta-resources/summary`, `GET/POST /demand-profiles`, `GET/POST /pdca-cycles`, `GET/POST /pdca-events` |
| Identity | `POST /agents/register`, `GET /agents` |
| AI Gateway | Model providers, channel/key pool, model catalog, multidimensional pricing, routing rules, `POST /ai-gateway/invoke`, `GET/POST /ai-gateway/stream`, invocation logs, usage analysis, and cost summary routes |
| Tool Runtime | Tool definition, tool test, tool execution log, and tool approval routes |
| Finance | Finance adapter, export batch, submit export, webhook callback, and reconciliation routes |
| Organization | `GET/POST/PATCH /organizations`, `GET /organization/current`, plus department, department tree, position, position assignment, organization member, external member, MVRU, relationship, member-matching, and capability-matching routes |
| Layer | `POST /layers/classify`, `GET/PUT /layers/config/{mvruId}`, `GET /layers/rules` |
| Capability | `GET/POST /capabilities`, `GET /capabilities/{id}`, `POST /capabilities/match`, capability evaluation, binding, and unbinding routes |
| Workflow | Workflow template, instance, status, task completion, decision recording, and context read/write routes |
| Project Lifecycle | Requirement, requirement document, requirement-analysis workflow, approval, project conversion, project member, project workflow, project overview, deliverable, cost, and feedback routes |
| Governance | Permission, principle, control rule, permission check, access decision, and access decision list routes |
| Evolution | Weight computation, outcome recording, context weights, alpha config, experiment, knowledge, signal, and signal acknowledgement routes |
| Observability | Trace, span, trace completion, metric recording, and metric query routes |
| Verification | Verification report, report query, review assignment, and review completion routes |

Frontend API Workbench metadata lives in `frontend/src/lib/operations.ts`. It groups operations by MetaOrg, MetaResource, DeveloperTools (Model Settings), Finance, Dashboard, Identity, Organization, Layer, Capability, Workflow, Observability, Verification, Governance, Evolution, Requirement, Project, Delivery, Cost, and Feedback.

## Quick Start

Start the full local environment with Docker Compose:

```bash
docker compose up --build
```

Service addresses:

- PostgreSQL: `localhost:5432`
- Go API: `http://localhost:8080`
- API health: `http://localhost:8080/api/v1/health`
- Next.js frontend: `http://localhost:3000`

Default Docker environment values are defined in `docker-compose.yml`:

- Database: `postgres://postgres:postgres@postgres:5432/meta_org?sslmode=disable`
- Backend port: `8080`
- Model and finance secret encryption: `MODEL_SECRET_KEY=0123456789abcdef0123456789abcdef`
- Frontend API URL: `http://localhost:8080/api/v1`

## Local Development

Backend:

```bash
cd backend
go run ./cmd/server
go test ./...
go build ./cmd/server
```

When running the backend outside Docker, provide PostgreSQL and set:

```bash
MIGRATIONS_PATH=../migrations
```

PowerShell:

```powershell
$env:MIGRATIONS_PATH = '../migrations'
go run ./cmd/server
```

Frontend:

```bash
cd frontend
npm install
npm run dev
npm run lint
npm run build
```

The frontend defaults to:

```bash
NEXT_PUBLIC_API_URL=http://127.0.0.1:8080/api/v1
```

### Windows Local Restart Notes

If `docker compose up --build` fails because the `docker` command is unavailable, run the development services with local PostgreSQL, Go, and Node. Confirm PostgreSQL is reachable, then start the backend and frontend separately.

When starting background processes with `Start-Process -ArgumentList` in Windows PowerShell, avoid `$env:NAME="value"` inside the nested command. The outer PowerShell process can expand `$env:` too early, so the child process receives `=value` or unquoted URL/path fragments. Common symptoms are:

- `migrations failed: read migrations dir: open migrations: The system cannot find the file specified.`
- `../migrations` or `http://localhost:8080/api/v1` is treated as a command.

Use `Set-Item Env:` instead:

```powershell
Start-Process -FilePath "powershell" -ArgumentList @(
  '-NoProfile',
  '-Command',
  'Set-Item Env:MIGRATIONS_PATH ../migrations; Set-Item Env:SERVER_PORT 8080; Set-Item Env:DATABASE_URL postgres://postgres:postgres@localhost:5432/meta_org?sslmode=disable; go run ./cmd/server'
) -WorkingDirectory "D:\project\meta-org\backend" -WindowStyle Hidden -RedirectStandardOutput "D:\project\meta-org\backend-dev.log" -RedirectStandardError "D:\project\meta-org\backend-dev-err.log"

Start-Process -FilePath "powershell" -ArgumentList @(
  '-NoProfile',
  '-Command',
  'Set-Item Env:NEXT_PUBLIC_API_URL http://localhost:8080/api/v1; npm run dev'
) -WorkingDirectory "D:\project\meta-org\frontend" -WindowStyle Hidden -RedirectStandardOutput "D:\project\meta-org\frontend-dev.log" -RedirectStandardError "D:\project\meta-org\frontend-dev-err.log"
```

Verification:

```powershell
Get-NetTCPConnection -LocalPort 3000,8080 -ErrorAction SilentlyContinue |
  Select-Object LocalAddress,LocalPort,State,OwningProcess

Invoke-WebRequest -Uri http://127.0.0.1:3000 -UseBasicParsing -TimeoutSec 8 |
  Select-Object StatusCode

Invoke-WebRequest -Uri http://127.0.0.1:8080/api/v1/health -UseBasicParsing -TimeoutSec 8 |
  Select-Object StatusCode,Content
```

The expected state is frontend `3000` and backend `8080` both in `Listen`, frontend HTTP `200`, and backend health returning `{"status":"ok"}`. To stop an old process, first confirm the `OwningProcess` from the port query, then run `Stop-Process -Id <PID> -Force` for one PID at a time.

After the AI Gateway, Meta Resource, SaaS, security-kernel, and supply-chain refactors, startup must apply all four staged baselines: `000/001/002/004`. If backend startup, Model Settings, Meta Resource, SaaS module pages, or supply-chain pages fail with `column ... does not exist`, `relation model_provider_channels does not exist`, `relation ai_routing_rules does not exist`, `relation meta_resources does not exist`, `relation tenant_modules does not exist`, `relation security_policies does not exist`, or `relation inventory_items does not exist`, the usual cause is a wrong `MIGRATIONS_PATH`, an old database in `DATABASE_URL`, or pending migrations. Confirm `DATABASE_URL`, use `MIGRATIONS_PATH=../migrations` when running from `backend/`, restart the backend so the migration runner applies `000_saas_platform_management_baseline.sql`, `001_erp_code_baseline.sql`, `002_erp_platform_integration_baseline.sql`, and `004_ai_capability_baseline.sql`, verify Model Settings pages for Channels / Keys, Routing, and Usage Analysis, then run Sync Existing Resources in the Meta Resource workspace.

## Configuration

Backend configuration is loaded in `backend/internal/pkg/config/config.go`:

| Environment Variable | Default | Description |
|---|---|---|
| `SERVER_PORT` | `8080` | Backend listen port. |
| `DATABASE_URL` | `postgres://postgres:postgres@localhost:5432/meta_org?sslmode=disable` | Backward-compatible PostgreSQL connection string; also used as the platform control database when `PLATFORM_DATABASE_URL` is unset. |
| `PLATFORM_DATABASE_URL` | Follows `DATABASE_URL` | SaaS platform control database connection string for platform administration, tenant organizations, capability packages, marketplace catalog, and tenant database routing metadata. |
| `TENANT_DATABASE_ADMIN_URL` | Follows `PLATFORM_DATABASE_URL` | Administrative connection string for physical tenant business database creation and maintenance; locally it can point to the same PostgreSQL instance. SaaS onboarding uses it to try creating tenant databases in `dedicated_database` mode. |
| `TENANT_DATABASE_NAME_PREFIX` | `meta_org_tenant_` | Prefix used when deriving physical tenant business database names from tenant organization IDs. |
| `TENANT_DATABASE_MODE` | `dedicated_database` | Tenant database target mode; `dedicated_database` records one physical database per tenant, `shared_schema` preserves the compatibility single-database schema mode. |
| `TENANT_DATABASE_DEFAULT_CLUSTER` | `local-primary` | Default tenant database cluster key recorded in the platform catalog. |
| `TENANT_DATABASE_DEFAULT_REGION` | `local` | Default tenant database region recorded in the platform catalog. |
| `JWT_SECRET` | `dev-secret-change-in-production` | JWT signing secret. Replace in production. |
| `MODEL_SECRET_KEY` | `0123456789abcdef0123456789abcdef` | 32-character key for model provider and finance adapter secret encryption. Replace in production. |
| `CORS_ORIGINS` | `http://localhost:3000,http://127.0.0.1:3000` | Frontend origins allowed to call the API. |
| `MIGRATIONS_PATH` | `migrations` | SQL migration directory; when running from `backend/`, usually set it to `../migrations`. |
| `META_ORG_MODE` | `single_org` | Runtime mode; set to `saas` for multi-tenant/SaaS semantics. |
| `META_ORG_DISTRIBUTION_MODE` | Follows `META_ORG_MODE` | Distribution mode: `saas`, `saas_org_private`, `single_org_commercial`, or `private_deployment`. |
| `META_ORG_LICENSE_MODE` | `commercial` | License mode: `community`, `commercial`, `enterprise`, or `private_contract`. |
| `SECURITY_KERNEL_URL` | empty | External security-kernel authorization service URL; when empty, the client treats it as not configured and allows requests. |
| `SECURITY_KERNEL_SHARED_SECRET` | empty | Shared secret for calls to the external security kernel. |
| `SECURITY_KERNEL_ENFORCEMENT_MODE` | `blocking` | Security-kernel enforcement mode; supports `blocking` and `audit`. |

In SaaS mode, initialize the platform administrator with `META_ORG_PLATFORM_ADMIN_EMAIL` and `META_ORG_PLATFORM_ADMIN_PASSWORD_HASH`. Generate the bcrypt password hash from `backend/`:

```powershell
$env:BCRYPT_PASSWORD = '<your-admin-password>'
go run ./cmd/bcrypt-hash
```

You can also pipe the password: `'<your-admin-password>' | go run ./cmd/bcrypt-hash -stdin`. Do not commit plaintext passwords or production hashes.

Frontend configuration:

| Environment Variable | Default | Description |
|---|---|---|
| `NEXT_PUBLIC_API_URL` | `http://127.0.0.1:8080/api/v1` | Browser-side API base URL. |

## Project Structure

```text
backend/
  cmd/server/                 Backend entry point
  internal/domain/            Domain modules
  internal/gateway/           Route registration
  internal/pkg/               Config, database, migrations, middleware, server
frontend/
  src/app/                    Next.js App Router pages and workspaces
  src/lib/                    API, auth, i18n, API Workbench metadata
docker-compose.yml            Full local environment orchestration
migrations/                   PostgreSQL staged SQL baselines 000, 001, 002, 004
docs/operations/              Production operations and finance adapter protocol docs
.github/workflows/            GitHub Actions CI
```

## Current Status and Boundaries

The codebase now provides a single-enterprise Meta-Org entry, Meta Resource / PDCA resource framework, organization management, project lifecycle, AI Gateway, tool runtime loop, cost accounting, generic finance, SaaS module gating, security-kernel integration, inventory/procurement/sales supply-chain workflows, governance, evolution, observability, and verification foundation. It is suitable as a production v1 base for 10-50 humans and 50-250+ agents.

When upgrading from the old `harness_org` database to `meta_org`, explicitly back up and migrate data first. The system does not automatically delete or overwrite the old database.

Important next steps:

- Expand model capabilities, agent executors, and external tool runtimes.
- Expand the MVRU sandbox concept from data model to isolated execution environment.
- Add automated frontend-state and end-to-end tests for critical workflows.
- Improve production-grade secret management, audit reports, alerts, and permission-policy visualization.
- Extend multi-organization tenant boundaries, approval-flow templates, and finer-grained operation audit trails.
