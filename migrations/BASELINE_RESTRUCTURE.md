# Migration Baseline Restructure

This directory now uses staged baseline migrations instead of the historical
`001` through `044` incremental files. The active migration order is:

1. `000_saas_platform_management_baseline.sql`
2. `001_erp_code_baseline.sql`
3. `002_erp_platform_integration_baseline.sql`
4. `004_ai_capability_baseline.sql`

## Stage Ownership

`000_saas_platform_management_baseline.sql` owns the SaaS management platform:
platform accounts, SaaS organizations, subscriptions, platform modules,
tenant/module entitlement, permission governance, security-kernel policy
foundation, schema-change governance, package metadata, and platform
master/detail infrastructure.

`001_erp_code_baseline.sql` owns ERP and industry-solution business tables:
tenant departments, project lifecycle, workflow, finance, costing,
supply-chain, and other ERP-facing domain structures. This file can change when
the SaaS platform creates or adjusts an industry solution.

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

Future organization profile, subscription, entitlement, invitation, schema, AI,
assistant, governance, and platform administration schema changes belong in
`000_saas_platform_management_baseline.sql`. Future tenant department or
ERP/industry-solution structures belong in `001_erp_code_baseline.sql` unless
they are AI capability structures owned by `004_ai_capability_baseline.sql`.

`002_erp_platform_integration_baseline.sql` owns runtime integration between the
ERP baseline and the SaaS platform: platform projections, module synchronization,
and ERP/platform master-data integration.

`004_ai_capability_baseline.sql` owns AI capability structures: model providers,
models, model channels, agents, AI invocation and usage ledger, tool runtime,
assistant runtime, assistant context, unified skill, skill publication, and AI
capability platform projections.

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
