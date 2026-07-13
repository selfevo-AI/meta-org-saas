# Meta-Org Production Runbook

This runbook covers the first production deployment path for a single-enterprise Meta-Org instance.

## Required Environment

Backend:

| Variable | Required | Notes |
|---|---:|---|
| `META_ORG_ENVIRONMENT` | yes | Set to `production`. Startup rejects repository development secrets, malformed integer/boolean values, and invalid worker, pool, rate-limit, or scheduler bounds before connecting to PostgreSQL. |
| `DATABASE_URL` | yes | PostgreSQL URL for the SaaS platform database, for example `postgres://user:pass@host:5432/meta_org_saas?sslmode=require`. |
| `PLATFORM_DATABASE_URL` | SaaS deployments | Explicit SaaS platform control database URL. If unset, follows `DATABASE_URL`. |
| `TENANT_DATABASE_ADMIN_URL` | dedicated tenant DB deployments | Administrative PostgreSQL URL for creating and migrating tenant databases. Point it at an admin database such as `postgres`, not at `meta_org_saas`. |
| `TENANT_DATABASE_NAME_PREFIX` | no | Defaults to `meta_org_`; tenant databases are named `meta_org_xxxx` from the tenant organization UUID. |
| `TENANT_MIGRATIONS_PATH` | dedicated tenant DB deployments | Usually `migrations/tenant` in containers and `../migrations/tenant` when running from `backend/`. |
| `JWT_SECRET` | yes | Use a non-default high-entropy secret. |
| `MODEL_SECRET_KEY` | yes | Exactly 32 characters. Used for provider and finance adapter secret encryption. |
| `SERVER_PORT` | no | Defaults to `8080`. |
| `CORS_ORIGINS` | yes | Comma-separated frontend origins. |
| `MIGRATIONS_PATH` | yes | Usually `migrations` in containers and `../migrations` when running from `backend/`. |
| `META_ORG_MODE` | no | Defaults to `single_org`; set to `saas` for multi-tenant semantics. |
| `META_ORG_DISTRIBUTION_MODE` | no | Distribution mode for licensing/deployment policy. |
| `META_ORG_LICENSE_MODE` | no | Defaults to `commercial`; choose the deployment license mode deliberately. |
| `SECURITY_KERNEL_URL` | SaaS deployments | Absolute external authorization service URL. SaaS startup fails when it is missing. |
| `SECURITY_KERNEL_SHARED_SECRET` | if kernel enabled | Shared secret of at least 32 characters. Production rejects the Compose development key. |
| `SECURITY_KERNEL_ENFORCEMENT_MODE` | no | Defaults to `blocking`; SaaS deployments must use `blocking`. |
| `SECURITY_KERNEL_DATABASE_URL` | security-kernel service | PostgreSQL URL for the `meta_org_saas` platform database. All replicas use its shared nonce ledger for replay protection. |
| `SENSITIVE_RATE_LIMIT_WINDOW_SECONDS` | no | Shared window for invitation creation, access-token creation, model configuration, key rotation, account administration, balance adjustment, and database maintenance writes. Defaults to `60`. |
| `SENSITIVE_RATE_LIMIT_MAX_ATTEMPTS` | no | Maximum sensitive writes per actor and client IP in the window. Defaults to `20`. |
| `SENSITIVE_RATE_LIMIT_BLOCK_SECONDS` | no | Block duration after a sensitive-operation bucket exceeds its budget. Defaults to `300`. |
| `INVITATION_ACCEPT_RATE_LIMIT_MAX_ATTEMPTS` | no | Public invitation acceptance attempts per IP and invitation token per window. Defaults to `10`. |
| `AI_GATEWAY_RATE_LIMIT_WINDOW_SECONDS` | no | Window for OpenAI-compatible API traffic. Defaults to `60`. |
| `AI_GATEWAY_RATE_LIMIT_MAX_REQUESTS` | no | OpenAI-compatible requests per access token and client IP in `AI_GATEWAY_RATE_LIMIT_WINDOW_SECONDS`. Defaults to `120`. |
| `AI_GATEWAY_RATE_LIMIT_BLOCK_SECONDS` | no | Block duration after an AI Gateway token or IP bucket exceeds its budget. Defaults to `60`. |
| `AI_GATEWAY_INVOKE_TIMEOUT_SECONDS` | no | Deployment ceiling for non-streaming provider calls. Defaults to `60`; provider `timeout_ms` may impose a lower limit. |
| `AI_GATEWAY_STREAM_TIMEOUT_SECONDS` | no | Deployment ceiling for AI Gateway and Assistant SSE duration. Defaults to `600`; Gateway provider `timeout_ms` may impose a lower limit. |
| `AI_GATEWAY_MAX_RETRIES` | no | Deployment ceiling for provider retries. Defaults to `3`; effective retries are also limited by provider `retry_count`. |
| `AI_GATEWAY_RESERVATION_RECOVERY_ENABLED` | no | Enables unfinished balance-reservation recovery. Defaults to `true`. |
| `AI_GATEWAY_RESERVATION_STALE_SECONDS` | no | Age before a reservation is recoverable. Defaults to `1800` and must exceed the stream timeout. |
| `AI_GATEWAY_RESERVATION_POLL_SECONDS` | no | Recovery scan interval. Defaults to `300`. |
| `AI_GATEWAY_RESERVATION_LEASE_SECONDS` | no | Recovery lease duration for multi-replica workers. Defaults to `60`. |
| `AI_GATEWAY_RESERVATION_BATCH_SIZE` | no | Maximum reservations per recovery run. Defaults to `100`, maximum `1000`. |

Frontend:

| Variable | Required | Notes |
|---|---:|---|
| `NEXT_PUBLIC_API_URL` | yes | Browser-visible API base, for example `https://api.example.com/api/v1`. |

## Secret Generation

PowerShell examples:

```powershell
$bytes = [byte[]]::new(48)
[System.Security.Cryptography.RandomNumberGenerator]::Fill($bytes)
[Convert]::ToBase64String($bytes)
```

Use the full output for `JWT_SECRET`.

For `MODEL_SECRET_KEY`, generate a 32-character value:

```powershell
$bytes = [byte[]]::new(24)
[System.Security.Cryptography.RandomNumberGenerator]::Fill($bytes)
[Convert]::ToBase64String($bytes).Substring(0, 32)
```

Keep both values in the deployment secret manager. Do not commit them.

## Fresh Setup

1. Create an empty PostgreSQL database named `meta_org_saas`.
2. Set `META_ORG_ENVIRONMENT=production` and backend environment variables including `DATABASE_URL`, `PLATFORM_DATABASE_URL`, `TENANT_DATABASE_ADMIN_URL`, `JWT_SECRET`, `MODEL_SECRET_KEY`, `MIGRATIONS_PATH`, and `TENANT_MIGRATIONS_PATH`.
3. If using an external security kernel, set `SECURITY_KERNEL_URL`, `SECURITY_KERNEL_SHARED_SECRET`, and choose `SECURITY_KERNEL_ENFORCEMENT_MODE`. Configure every security-kernel replica with the same `SECURITY_KERNEL_DATABASE_URL` for `meta_org_saas`.
4. Start the backend. It applies the staged SQL baselines automatically in filename order: `000_saas_platform_management_baseline.sql`, `001_erp_code_baseline.sql`, `002_erp_platform_integration_baseline.sql`, and `004_ai_capability_baseline.sql`.
5. For SaaS mode, bootstrap the platform administrator with `META_ORG_PLATFORM_ADMIN_EMAIL` and `META_ORG_PLATFORM_ADMIN_PASSWORD_HASH`.
6. Start the frontend with `NEXT_PUBLIC_API_URL` pointing at `/api/v1`.
7. Create or onboard tenant organizations through SaaS onboarding. Each tenant physical database must be created by the backend tenant database provisioner and named `meta_org_xxxx`, where `xxxx` is the first four lowercase hex characters of the tenant organization UUID.
8. Open Meta-Org Home and confirm overview and inbox data load.
9. Open Meta Resource and run Sync Existing Resources once so humans, agents, external members, model channels, tools, and capabilities are indexed into the meta resource layer.
10. Open Inventory, Procurement, Sales, and Finance if those SaaS modules are enabled for the organization, then confirm list endpoints load before running posting actions.

## Database Naming Rules

SaaS deployments use split databases:

- Platform control database: `meta_org_saas`
- Tenant business database: `meta_org_xxxx`
- `xxxx`: first four lowercase hexadecimal characters of the tenant
  organization UUID after removing hyphens
- Backup databases must be explicitly named, for example
  `meta_org_backup_YYYYMMDDHHMMSS`

Do not restore the historical single database `meta_org` as the active runtime
database after the split. Restore it only as an explicitly named backup or
migration source, then migrate compatible data into a freshly migrated
`meta_org_saas`.

Tenant databases must be migrated with the backend tenant migrator. The tenant
baseline contains `-- tenantdb:include`; direct `psql -f` execution skips the
included ERP baseline and can leave Finance/ERP tables missing.

## Migrating From `harness_org` Or `meta_org`

The application does not delete or overwrite the old database. Back up first.

```bash
pg_dump "$OLD_DATABASE_URL" > harness_org_backup.sql
createdb meta_org_saas
```

Create `meta_org_saas` from current migrations first. Then copy only compatible
data from the backup source into the new platform database, preserving the new
baseline seed IDs where they intentionally changed. Create tenant databases
through SaaS provisioning or a controlled maintenance worker.

After migration, start the new backend with `DATABASE_URL` and
`PLATFORM_DATABASE_URL` pointing to `meta_org_saas`. Run a smoke test before
allowing users back in:

```bash
cd backend
go test ./...
go build ./cmd/server
```

## Restore Procedure

1. Stop backend writers.
2. Create a fresh restore database.
3. Restore the selected dump with `psql`.
4. Point `DATABASE_URL` at the restored database.
5. Start the backend and confirm `/api/v1/health`.
6. Verify login, Meta-Org Home, Developer Tools, and Finance Exports.

## Provider Setup

1. Open Developer Tools.
2. Create providers for OpenAI, Anthropic, and Gemini.
3. Store only real provider keys in the provider form; keys are encrypted at rest and returned as masked values.
4. Create provider channels for production keys, agent-owned keys, or fallback keys. Configure priority, concurrency, quota, rate multiplier, supported model patterns, and model mapping.
5. Create or confirm model catalog entries with input, output, cache, image, priority, long-context pricing, and currency.
6. Create routing rules when a source surface, user, agent, project, or model pattern must prefer a provider/channel.
7. Run provider and channel connection tests.
8. Use the AI Assistant to run a streaming call per provider or channel.
9. Confirm invocation logs, usage analysis, channel cost breakdown, and cost ledger entries update.

Provider key rotation:

1. Open Developer Tools, Providers.
2. Select the provider.
3. Enter the new key in Key Rotation.
4. Run Test Provider.
5. Confirm only the masked key is visible.

Channel key rotation:

1. Open Developer Tools, Channels / Keys.
2. Select the channel.
3. Enter the new key in Rotate Channel Key.
4. Run Test Channel.
5. Confirm the channel health and masked key update.

## Startup and Migration Troubleshooting

The current production schema baseline is staged as SaaS platform management first, ERP/industry business baseline second, ERP-platform integration third, and AI capability baseline after both platform and ERP tables exist. The active migration files are `000_saas_platform_management_baseline.sql`, `001_erp_code_baseline.sql`, `002_erp_platform_integration_baseline.sql`, and `004_ai_capability_baseline.sql`.

If startup, Developer Tools, Meta Resource, SaaS module pages, ERP code-table workspaces, or AI Assistant calls fail with errors such as `relation model_provider_channels does not exist`, `relation ai_routing_rules does not exist`, `column cost_breakdown does not exist`, `relation meta_resources does not exist`, `relation demand_profiles does not exist`, `relation tenant_modules does not exist`, `relation security_policies does not exist`, or missing ERP code-table relations such as `MITW`, `MPOR`, `MRDR`, `MRPS`, or `MDRQ`:

1. Confirm `DATABASE_URL` and `PLATFORM_DATABASE_URL` point to the intended `meta_org_saas` platform database.
2. Confirm `MIGRATIONS_PATH` points to the root `migrations/` directory. When running from `backend/`, use `../migrations`.
3. Restart the backend so the migration runner applies `000/001/002/004` in order.
4. Re-run `cd backend && go test ./...` and `cd frontend && npm run build`.
5. Open Developer Tools and verify Providers, Channels / Keys, Routing, Invocations, and Usage Analysis all load.
6. Open Meta Resource, run Sync Existing Resources, and verify the summary includes at least the existing human, agent, tool, and capability counts.

If SaaS management APIs fail with `relation platform.database_maintenance_jobs does not exist` or `relation platform.tenant_database_targets does not exist`, the backend is connected to an old or partially migrated platform database. Rebuild or migrate `meta_org_saas` from the current staged baseline before retrying.

If tenant finance or ERP APIs fail with `relation gl_journal_entries does not exist`, inspect the selected tenant database. A tenant created by raw `psql -f migrations/tenant/001_tenant_business_baseline.sql` is incomplete because `tenantdb:include` is not expanded. Re-run the tenant migration through the backend tenant migrator, or explicitly apply the included ERP baseline during recovery and align the tenant migration checksum with the migrator output.

If browser calls fail with `Failed to fetch`, verify both network reachability and CORS response headers. Windows PowerShell startup commands must quote comma-separated `CORS_ORIGINS`; otherwise the backend can start without matching `Access-Control-Allow-Origin`.

If onboarding fails with `security kernel denied owner_attestation verify`, verify the security kernel process is reachable, `SECURITY_KERNEL_SHARED_SECRET` matches on both sides, the security kernel binary accepts the JSON distribution mode value `saas`, and the backend sends owner attestation as a `general` security feature gate rather than as a tenant business module entitlement.

If security-kernel `/healthz` reports `security_nonce_table_unavailable`, start
the backend migration process and confirm migration `023_security_kernel_replay_ledger.sql`
has created `platform.security_request_nonces`. A database error or missing
ledger intentionally fails authorization closed so a horizontally scaled
deployment cannot silently fall back to per-process replay protection.

Backend `/api/v1/health` checks both the platform PostgreSQL pool and the
security kernel. A database outage returns HTTP 503 with
`platform_database_unavailable`; the security kernel also returns unavailable
because replay protection is database-backed. Both services reconnect without
a process restart after PostgreSQL is reachable again.

For failed `/api/v1` requests, record the `code` and `request_id` response
fields or the matching `X-Error-Code` and `X-Request-ID` headers. Search backend
logs by request ID for internal 5xx details. Public 5xx responses intentionally
hide database, provider, and infrastructure error text. OpenAI-compatible `/v1`
responses are outside this internal API envelope.

Sensitive management writes, public invitation acceptance, and OpenAI-compatible
AI Gateway calls use the shared PostgreSQL rate-limit buckets. Limits apply to
both the authenticated actor or presented token and the resolved client IP.
`X-Forwarded-For` and `X-Real-IP` are considered only when the direct peer is in
`TRUSTED_PROXY_CIDRS`. A blocked request returns HTTP 429 with `Retry-After`; a
bucket-store error returns HTTP 503 so replicas cannot silently bypass limits.
Aggregate checks, blocks, and storage failures are exposed as
`request_rate_limits` in `/api/v1/health`; the legacy
`authentication_rate_limits` field remains during the compatibility window.

Every OpenAI-compatible endpoint authenticates the presented AI access token,
including `/v1/models` and endpoints that currently return 501. `/v1/models`
returns only active models allowed by the token's exact/pattern allowlist and,
when assigned, its model-group channel abilities. A non-empty arbitrary Bearer
value is not sufficient authentication. `POST /v1/chat/completions` honors
`stream: true` with OpenAI-compatible SSE chunks and `[DONE]`; streaming calls
use the same model policy, security-kernel authorization, balance reservation,
actual-usage settlement, channel accounting, and invocation audit as synchronous
calls.

Business AI proposals are freshness-bound to the authoritative project overview
captured during analysis. HTTP 409 with a project-context-changed message means
members, workflows, deliverables, costs, evaluations, lifecycle state, or core
project/requirement data changed before submission. Re-run the selected business
stage analysis; do not bypass the check or reuse the old tool arguments.
Analyses created before migration `026` have no trusted fingerprint and also
return HTTP 409; re-run them rather than backfilling or copying a hash.

Meta Resource sync intentionally reads existing source tables without owning them. If sync fails after a schema change, check these source assumptions first:

- `ai_agents` uses `is_active`, `service_class`, `risk_level`, `capabilities`, and `metadata`; the governance fields come from migration `012`.
- `tool_definitions` uses `is_active` for status; it does not have a `status` column.
- `model_provider_channels` comes from migration `020` and requires `model_providers` rows for joined provider type metadata.
- `capabilities.cost_estimate` is used as the initial cost profile.

PDCA and operations smoke-test notes:

- Backend health check is `GET /api/v1/health`; `GET /health` is not registered.
- Demand Profile JSONB list fields accept arrays such as `["accepted"]` or object arrays such as `[{"name":"accepted"}]`; use arrays for `acceptance_criteria`, `required_capabilities`, and `resource_fit_candidates`.
- `POST /ai-gateway/estimate-cost` uses a nested `usage` object and reads the model price catalog directly. It does not require an active provider channel, so disabled draft providers can still be priced.
- Exchange-rate `source` must be `manual` or `external`.
- Cost rate cards require `cost_category`, `subject_type`, `rate_type`, `amount`, and `currency`.
- Finance adapters support `hmac` and `bearer`; both require a secret because outgoing calls are signed or authenticated.
- A compact accept-stage smoke test should register and log in a temporary user, read the main workbench endpoints, create Demand Profile -> PDCA Cycle -> PDCA Event, create a draft model catalog entry, and call `POST /ai-gateway/estimate-cost`. Expected estimate for 1000 input tokens and 500 output tokens at 0.01/0.03 CNY per 1K is `0.025 CNY`.

When running services in Windows PowerShell with `Start-Process`, set nested environment variables with `Set-Item Env:NAME value`; avoid `$env:NAME="value"` inside `-ArgumentList` because the outer shell can expand it before the child process starts.

## Finance Adapter Setup

1. Open Finance Exports, Adapters.
2. Create a Generic Finance Adapter with endpoint URL, auth type, secret, timeout, and retry count.
3. Use HMAC unless the downstream system requires bearer auth.
4. Run Test Adapter.
5. Create an export batch for the desired period.
6. Submit the batch.
7. Confirm external acknowledgement through webhook callback.
8. Review reconciliation differences.

## Streaming Troubleshooting

Check:

- Provider is active and tested.
- Model catalog entry is active.
- Browser can reach `NEXT_PUBLIC_API_URL`.
- Reverse proxy does not buffer `text/event-stream`.
- Timeouts allow long-running streaming responses.
- Invocation logs distinguish provider errors from cancelled streams.

For proxy deployments, disable response buffering for both
`/api/v1/ai-gateway/stream` and `/v1/chat/completions` when `stream: true` is used.
The backend clears its per-response write deadline only for AI Gateway and
Assistant SSE handlers; other routes retain the server-wide 15-second write
timeout. The effective Gateway timeout is the smaller positive value of the
provider `timeout_ms` and the matching deployment ceiling. Stream timeout is
governed by `AI_GATEWAY_STREAM_TIMEOUT_SECONDS` and
is audited as `ai_stream_timeout`, while client disconnects remain
`ai_stream_disconnect`.

The health response exposes `ai_gateway_reservation_recovery`. A rising
`failures_total` or non-empty `last_error` indicates that stale reservations may
still hold organization balance or token quota. The worker refunds reservations
that never reached invocation creation, settles attached terminal invocations,
and cancels abandoned `started` or `streaming` invocations while releasing their
provider channel. Keep `AI_GATEWAY_RESERVATION_STALE_SECONDS` above every allowed
stream lifetime; startup rejects an unsafe lower value.

Provider `retry_count` applies to synchronous calls, stream establishment, and
provider/channel connectivity tests. All attempts share the original operation
timeout. Automatic retry is deliberately limited to explicit HTTP 429, 502, 503,
and 504 responses. Authentication, validation, quota, ordinary 4xx, HTTP 500,
context cancellation, and ambiguous network failures are not retried because
the upstream may already have accepted and billed the request. Increase retry
limits only after checking provider billing and idempotency behavior.
Observability records `ai_provider_retry` for each scheduled retry and
`ai_provider_retry_exhausted` when a configured retry budget ends without
success; metadata contains only HTTP status and attempt counts.

## Finance Export Retry

Failed exports remain visible in Finance Exports and Meta-Org Inbox.

1. Inspect the batch failure reason.
2. Fix adapter endpoint, auth, or downstream validation.
3. Re-submit the same batch when the failure is transient.
4. Create an adjustment batch for accounting corrections after external posting.

Do not mutate exported source usage rows. Corrections should be additive.

## Operational Checks

Run before a release:

```bash
cd backend
go test ./...
go build ./cmd/server
cd ../frontend
npm run lint
npm run build
cd ..
docker compose config
```

Expected result: all commands exit successfully, Docker Compose renders the
`meta_org_saas` platform database topology, and `/api/v1/health` returns HTTP
200 with `platform_database.status` and `security_kernel.status` set to `ok`.
