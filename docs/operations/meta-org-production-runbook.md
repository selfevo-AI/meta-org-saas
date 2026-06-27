# Meta-Org Production Runbook

This runbook covers the first production deployment path for a single-enterprise Meta-Org instance.

## Required Environment

Backend:

| Variable | Required | Notes |
|---|---:|---|
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
| `SECURITY_KERNEL_URL` | no | External authorization service URL. Empty means the client is not configured and allows requests. |
| `SECURITY_KERNEL_SHARED_SECRET` | if kernel enabled | Shared secret for security-kernel calls. |
| `SECURITY_KERNEL_ENFORCEMENT_MODE` | no | Defaults to `blocking`; use `audit` only for controlled rollout. |

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
2. Set backend environment variables, including `DATABASE_URL`, `PLATFORM_DATABASE_URL`, `TENANT_DATABASE_ADMIN_URL`, `JWT_SECRET`, `MODEL_SECRET_KEY`, `MIGRATIONS_PATH`, and `TENANT_MIGRATIONS_PATH`.
3. If using an external security kernel, set `SECURITY_KERNEL_URL`, `SECURITY_KERNEL_SHARED_SECRET`, and choose `SECURITY_KERNEL_ENFORCEMENT_MODE`.
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

If startup, Developer Tools, Meta Resource, SaaS module pages, supply-chain pages, or AI Assistant calls fail with errors such as `relation model_provider_channels does not exist`, `relation ai_routing_rules does not exist`, `column cost_breakdown does not exist`, `relation meta_resources does not exist`, `relation demand_profiles does not exist`, `relation tenant_modules does not exist`, `relation security_policies does not exist`, or `relation inventory_items does not exist`:

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

For proxy deployments, disable response buffering for `/api/v1/ai-gateway/stream`.

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

Expected result: all commands exit successfully, and Docker Compose renders `meta_org`.
