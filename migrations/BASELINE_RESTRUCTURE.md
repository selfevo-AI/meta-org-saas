# Migration Baseline Restructure

This directory now uses staged baseline migrations instead of the historical
`001` through `044` incremental files. The active migration order is:

1. `000_saas_platform_management_baseline.sql`
2. `001_erp_code_baseline.sql`
3. `002_erp_platform_integration_baseline.sql`
4. `004_ai_capability_baseline.sql`

Physical tenant databases use their own tenant migration directory:

1. `tenant/001_tenant_business_baseline.sql`

The tenant baseline creates tenant-local projections for platform-owned actors,
organizations, departments, memberships, workflow metadata, module snapshots,
and sample validation data, then includes `001_erp_code_baseline.sql` so each
dedicated tenant database owns its ERP, project, workflow, costing, finance,
inventory, procurement, sales, and retail runtime tables without cross-database
foreign keys. Procurement, sales, inventory, warehouse, and retail distribution
business objects are represented by ERP code-table master/detail pairs rather
than by a second set of semantic supply-chain tables.

## Database Naming And Provisioning Rules

The SaaS split-database contract is:

- The platform management database is `meta_org_saas`.
- Physical tenant business databases are named `meta_org_xxxx`.
- `xxxx` is the first four lowercase hexadecimal characters of the tenant
  organization UUID after removing hyphens.
- `TENANT_DATABASE_NAME_PREFIX` defaults to `meta_org_`.
- `TENANT_DATABASE_ADMIN_URL` points at an administrative database such as
  `postgres` on the target PostgreSQL instance. It must not rely on connecting
  to the platform database before creating a tenant database.

Tenant database names must be generated through the backend tenant database
helper (`tenantdb.DatabaseNameForOrganization`) or through SQL that exactly
matches the baseline expression:

```sql
'meta_org_' || LEFT(REPLACE(organization_id::TEXT, '-', ''), 4)
```

Fresh setup and tenant onboarding must create `meta_org_saas` first, run the
root staged migrations there, insert the `platform.tenant_database_targets`
catalog row, then create and migrate the physical tenant database. Do not use
the historical single database `meta_org` as an active runtime database after
the split; it may only be used as an explicitly named backup or migration
source.

The SaaS management database also owns tenant account governance. Platform
administrators may list tenant organization accounts, change the organization
owner/authority tier, update account status and email/name, and reset tenant
account passwords. Human users and other authenticated account holders may only
change their own password through the identity self-service endpoint. These
flows are platform-governed operations and must not be implemented as direct
tenant database writes.

Tenant migrations must be executed by the backend tenant migrator or another
tool that understands `-- tenantdb:include`. Running
`migrations/tenant/001_tenant_business_baseline.sql` directly with `psql -f`
does not expand the include of `../001_erp_code_baseline.sql`; that creates an
incomplete tenant database where ERP/finance tables such as
`gl_journal_entries`, `gl_journal_entry_lines`, and `gl_accounts` are missing.
If manual recovery is unavoidable, run the included ERP baseline explicitly and
record the checksum that the tenant migrator calculates for the expanded tenant
file.

Industry module policy must stay aligned with the staged seeds. The `general`
industry baseline is allowed to enable core organization/governance modules and
the ERP operating loop modules (`erp`, `finance`, `costing`, `inventory`,
`procurement`, `sales`, `retail`) so a tenant can adopt the standard ERP and
retail distribution industry solutions without a policy denial. Any future
industry module addition must update both `000_saas_platform_management_baseline.sql`
seed assets and the backend policy defaults before the UI exposes it.

The June 2026 split-database recovery surfaced these concrete failure modes:

- `relation platform.database_maintenance_jobs does not exist`
- `relation platform.tenant_database_targets does not exist`
- `security_kernel_unavailable` during owner attestation for tenant creation
- `relation gl_journal_entries does not exist` in tenant finance APIs
- browser `Failed to fetch` when the frontend reached an unavailable or
  incorrectly configured backend

The corrective contract is to rebuild `meta_org_saas` from the staged platform
baseline, create tenant databases through the backend provisioner, and verify
tenant migration expansion before using tenant ERP/finance APIs.

## Stage Ownership

`000_saas_platform_management_baseline.sql` owns the SaaS management platform:
platform accounts, SaaS organizations, subscriptions, platform modules,
tenant/module entitlement, permission governance, security-kernel policy
foundation, schema-change governance, package metadata, and platform
master/detail infrastructure.

It also owns the platform control foundation used by the SaaS management
console: platform feature registration, platform menu metadata, platform RBAC
permissions and roles, platform user role assignment, and database maintenance
job governance for backup/restore requests. These structures are metadata and
approval records only; they do not grant arbitrary code, SQL, or plugin
execution from the UI.

It also owns the physical database topology control plane. The target
architecture separates the SaaS platform control database from tenant business
databases:

- `platform.database_clusters` records logical PostgreSQL clusters, regions,
  deployment scope, capacity, and secret references.
- `platform.tenant_database_targets` maps each tenant organization to its
  physical business database or the compatibility shared-schema target.
- `platform.tenant_database_migrations` tracks migration state per tenant
  database target.
- `platform.capability_packages` and `platform.marketplace_listings` hold the
  platform-authoritative package and marketplace catalog for industry
  solutions, function modules, API capabilities, AI model profiles, and skills.

The staged baseline seeds a local-development industry solution sample,
`local_manufacturing_demo`, into `platform.capability_packages` and
`platform.marketplace_listings`. It is metadata only: the package manifest
declares a sample tenant database template, the `sample_work_orders` sample
table, and the `sample.work_order.create` sample function. Physical database
creation remains the responsibility of tenant database provisioning or approved
database maintenance jobs because `CREATE DATABASE` cannot be executed inside
the baseline transaction.

The business-closure sample tenant is not created by baseline startup. SaaS
administrators create it explicitly from the SaaS management console or
`POST /api/v1/platform/admin/sample-tenants/business-closure`. That flow creates
the platform organization, enables all currently registered SaaS modules,
provisions the dedicated tenant database, runs the tenant migration set, and
bootstraps tenant-local sample projections and ERP/inventory sample records.

Tenant private deployment export is reserved in this stage as database
maintenance metadata only through
`POST /api/v1/platform/admin/organizations/{id}/private-deployment-exports`.
The job scope is `tenant_database:<organization_id>` and records the intended
private-deployment package contents, but actual `pg_dump`, package signing, and
private runtime import remain deferred implementation steps.

The platform control plane owns package definitions, publication, review,
authorization, marketplace listing, settlement policy, tenant database routing,
and backup/restore orchestration. Tenant business databases own instantiated
business data, installed solution/module/runtime tables, tenant-specific API
configuration, private model channels, private skills, assistant context, and
tenant execution logs. Do not add cross-database foreign keys or assume
cross-database transactions; use UUID references, metadata projections, and
event/outbox style synchronization for future multi-service and cluster
deployments.

### Phase 2 Industry Solution Factory Storage

The industry solution factory remains a platform management capability and
belongs to `000_saas_platform_management_baseline.sql`.

Phase 2 intentionally reuses existing platform storage instead of creating a
table per asset type:

- `platform.industry_solution_change_requests.solution_manifest.metadata.industry_manifest`
  stores the desired manifest.
- `platform.industry_solution_change_requests.solution_manifest.metadata.package_diff` stores
  the solution-level asset diff computed at request creation.
- `platform.industry_solution_apply_jobs.metadata.asset_results` stores per-asset apply
  status and retry diagnostics.
- `platform.runtime_operations` stores runtime operation assets.
- `tool_definitions` stores Tool Runtime definition and policy assets from the
  AI capability baseline.
- `platform.platform_masters` stores draft context rule, assistant skill,
  quality gate, and verification scenario metadata assets.

Context-rule assets generated from industry solutions are stored as draft
metadata and must not be activated automatically by solution apply.

Phase 3 keeps the same tables and extends the runtime metadata contract:
industry solution `runtime_operation` assets preserve their full payload in
`platform.runtime_operations.metadata`, including `metadata.workspace` for
tenant ERP business workbench document/action configuration.

Phase 4 adds structured `database_table` and `database_field` solution asset
types. Tenant-specific table and field edits are converted into
`platform.industry_solution_change_requests` so every selected tenant still follows the
preview, approve, verify, and apply lifecycle before physical database changes
are applied.

`001_erp_code_baseline.sql` owns ERP and industry-solution business tables:
tenant departments, project lifecycle, workflow, finance, costing,
ERP code-table supply-chain, ERP action execution ledger tables, and other
ERP-facing domain structures. This file can change when the SaaS platform
creates or adjusts an industry solution. The ERP action ledger uses `MAEX` and
`AEX1`; `MACT` remains the ERP G/L account table. Semantic GL runtime tables
such as `gl_accounts`, `gl_cost_centers`, `gl_journal_entries`, and
`gl_journal_entry_lines` also belong here because they are tenant ERP/finance
structures behind the compatible ERP table codes (`MACT`, `MPRC`, `MJDT`,
`JDT1`, `MGLR`).

Supply-chain domain ownership is now code-table-only for fresh tenant
databases:

- Purchase orders: `MPOR/POR1`
- Sales orders: `MRDR/RDR1`
- Warehouse balances: `MITW/ITW1`
- Goods receipts and issues: `MIGN/IGN1`, `MIGE/IGE1`
- Retail POS and distribution: `MRPS/RPS1`, `MDRQ/DRQ1`, `MDSP/DSP1`,
  `MDRC/DRC1`, `MDIF/DIF1`
- Retail stock policy, count, and special procurement: `MSTP/STP1`,
  `MCNT/CNT1`, `MSPR/SPR1`

Fresh tenant databases must not create the old semantic supply-chain physical
tables such as `inventory_counts`, `inventory_transfers`, `purchase_orders`,
`sales_shipments`, `inventory_balances`, or `sales_orders`. Historical data
from backup databases should be transformed into ERP code-table payloads during
an explicit migration step instead of restoring those tables as the primary
runtime model.

## Organization And Department Boundary

SaaS platform management owns organizations. An organization represents the
tenant account created and governed by the platform, including subscription,
module entitlement, invitation, platform permissions, schema target, and
industry-solution adoption.

Tenant runtime is single-organization. Tenant-side base data must not expose an
organization tree, organization switching, or tenant organization CRUD. The
tenant business tree for the historical `Organization` domain is a department
tree under the current tenant organization. Tenant-side department, position,
member, authority, and department-MVRU structures remain scoped by
`organization_id` for isolation, but the user-facing concept is department.

Future organization profile, subscription, entitlement, invitation, industry solution, AI,
assistant, governance, and platform administration schema changes belong in
`000_saas_platform_management_baseline.sql`. Future tenant department or
ERP/industry-solution structures belong in `001_erp_code_baseline.sql` unless
they are AI capability structures owned by `004_ai_capability_baseline.sql`.

Tenant runtime is moving from compatibility shared schemas toward physical
tenant business databases. `organization_industry_solution_targets` records the
tenant solution target, while `tenant_database_targets` is the source of truth for physical database routing.
New platform code must read or write tenant database placement through the
platform control-plane target table, not derive deployment topology only from
`org_<uuid>` schema names. Private deployments run a local platform control
plane copy and synchronize only solution, license, authorization, and settlement
summaries with the central SaaS platform.

ERP, project, workflow, costing, finance, inventory, procurement, sales, and
tenant organization runtime repositories route through the tenant database
router. Provisioned `dedicated_database` tenants use their physical tenant
database; shared-schema or not-yet-provisioned tenants continue to use the
platform pool as the compatibility fallback. Tenant-facing routes for those
business domains remain mounted in the tenant closure so new tenants are created
through the same permission, module entitlement, and database provisioning flow.

SaaS onboarding now records the target first in the platform transaction and
then attempts physical tenant database provisioning after the transaction has
committed. A successful provisioning attempt marks the target `provisioned` and
records the migration version. A failed attempt marks the target `failed` with
diagnostic metadata while preserving the already-created platform organization,
so a later maintenance/retry worker can continue from the catalog state.

`002_erp_platform_integration_baseline.sql` owns runtime integration between the
ERP baseline and the SaaS platform: platform projections, module synchronization,
ERP/platform master-data integration, and cross-stage runtime operations that
depend on ERP assets, such as the Finance trial-balance operation
`erp.finance.trial_balance.run` mapped to `/finance/gl/trial-balance`.

`004_ai_capability_baseline.sql` owns AI capability structures: model providers,
models, model channels, agents, AI invocation and usage ledger, tool runtime,
assistant runtime, assistant context, unified skill, skill publication, and AI
capability platform projections.

Phase 4 Verified Context + Tool Loop changes also belong to `004`: context
change proposals include the `applied` lifecycle state plus `apply_result` and
`applied_at`; the baseline seeds the governed tools `erp.action.execute`,
`industry.solution.change.preview`, `runtime.operation.execute`, and
`context.proposal.apply`; active ERP/finance/governance context rules are seeded
so strict modules do not fall back to compatibility context without dictionary
coverage.

Phase 5 Monitoring Agent storage is split by ownership: `004` owns
`monitoring_agent_runs` because scan execution and summaries are AI/evolution
capability metadata; `000` owns the `signals` table and adds the monitoring
fingerprint index used to suppress duplicate unacknowledged findings. The agent
may write signals and pending context proposals, but it must not apply database changes,
activate context rules, bypass tool approvals, or execute repository code
changes.

## Foreign-Key Rule

A migration stage must not create a foreign key to a table that belongs to a
later stage. Use plain UUID columns in the earlier stage, then rebuild the
foreign key in the later owning stage after both sides exist.

Current cross-stage foreign keys rebuilt in `004`:

- `mvru_members.agent_id -> ai_agents(id)`
- `organization_memberships.agent_id -> ai_agents(id)`
- `finance_export_lines.usage_ledger_id -> ai_usage_ledger(id)`
- `finance_export_lines.provider_id -> model_providers(id)`
- `finance_export_lines.model_id -> models(id)`
- `ai_usage_ledger.finance_export_line_id -> finance_export_lines(id)`

All baseline verification must confirm there are no unvalidated constraints:

```sql
SELECT COUNT(*) AS not_valid_constraints
FROM pg_constraint
WHERE NOT convalidated;
```

The expected result is `0`.

## Change Governance

Every future change that touches database structure, table ownership,
relationships, foreign keys, indexes, seed data, schema-generation logic, or
module/table catalogs must update both:

- the matching staged SQL file in `migrations/`
- this baseline restructure document

Do not put schema changes only in backend code, frontend assumptions, ad hoc SQL,
or informal notes. Fresh-database validation must run the active stage order
before the change is considered complete.

Fresh tenant-database validation is available with:

```bash
RUN_FRESH_TENANT_DB_MIGRATION_TEST=1 go test ./internal/pkg/tenantdb -run TestFreshTenantBusinessMigrationAgainstPostgres -count=1 -v
```

It creates a temporary PostgreSQL database from `MIGRATION_TEST_ADMIN_URL`, runs
`migrations/tenant`, checks core ERP/finance/project/workflow/supply-chain
tables, and bootstraps the business-closure sample data.
