# Local Non-Docker SaaS Deployment

This document records the local Windows deployment path for `meta-org-saas`
without Docker.

## Environment

- Repository: `D:\project\meta-org-saas`
- PostgreSQL: local service on `localhost:5432`
- Platform database: `meta_org_saas`
- Tenant databases: `meta_org_xxxx`, where `xxxx` is the first four lowercase
  hexadecimal characters of the tenant organization UUID without hyphens
- Backend: `http://127.0.0.1:8080`
- Frontend: `http://127.0.0.1:3000`
- Security kernel: `http://127.0.0.1:8090`
- Platform admin: `platform-admin@local.test`

Required local tools:

- Go toolchain
- Node.js and npm
- Rust toolchain
- PostgreSQL client tools, including `psql.exe`
- Visual Studio Build Tools for native Rust/Go dependency builds on Windows

## Migration Baseline

Active migration files, in execution order:

1. `000_saas_platform_management_baseline.sql`
2. `001_erp_code_baseline.sql`
3. `002_erp_platform_integration_baseline.sql`
4. `004_ai_capability_baseline.sql`

The baseline rule is SaaS management platform first, ERP/industry solution
baseline second, ERP-platform integration third, and AI/model/agent/tool/
assistant/skill capability structures in `004` after both platform and ERP
tables exist.

Future database changes must update the owning stage SQL and
`migrations/BASELINE_RESTRUCTURE.md` in the same change.

## Reset Local Database

This resets the local development platform database. Do not use against
production. Keep any explicitly named backup database, for example
`meta_org_backup_20260626074450`, unless the operator deliberately chooses a
different restore point.

```powershell
$env:PGPASSWORD = 'postgres'
& 'C:\Program Files\PostgreSQL\18\bin\psql.exe' -h localhost -U postgres -d postgres -c "DROP DATABASE IF EXISTS meta_org_saas WITH (FORCE);"
& 'C:\Program Files\PostgreSQL\18\bin\psql.exe' -h localhost -U postgres -d postgres -c "CREATE DATABASE meta_org_saas;"
```

Tenant databases must be created through SaaS onboarding or the backend tenant
database provisioner so tenant migration includes are expanded. If a local reset
requires removing old tenant databases, list them first and drop each
`meta_org_xxxx` database explicitly:

```powershell
$env:PGPASSWORD = 'postgres'
& 'C:\Program Files\PostgreSQL\18\bin\psql.exe' -h localhost -U postgres -d postgres -tAc "SELECT datname FROM pg_database WHERE datname ~ '^meta_org_[0-9a-f]{4}$' ORDER BY datname;"
& 'C:\Program Files\PostgreSQL\18\bin\psql.exe' -h localhost -U postgres -d postgres -c "DROP DATABASE IF EXISTS meta_org_4b28 WITH (FORCE);"
```

Do not run `migrations/tenant/001_tenant_business_baseline.sql` directly with
`psql -f` for a fresh tenant. That file contains `-- tenantdb:include`; direct
`psql` execution skips the included `001_erp_code_baseline.sql` and leaves
ERP/finance tables such as `gl_journal_entries` missing.

## Start Services

Build and start the security kernel:

```powershell
cd D:\project\meta-org-saas\security-kernel
cargo build
$env:SECURITY_KERNEL_PORT = '8090'
$env:SECURITY_KERNEL_SHARED_SECRET = 'dev-security-kernel-shared-secret'
$env:SECURITY_KERNEL_MAX_CLOCK_SKEW_SECONDS = '60'
$env:SECURITY_KERNEL_DATABASE_URL = 'postgres://postgres:postgres@127.0.0.1:5432/meta_org_saas?sslmode=disable'
Start-Process -FilePath 'D:\project\meta-org-saas\target\debug\security-kernel.exe' `
  -WorkingDirectory 'D:\project\meta-org-saas\security-kernel' `
  -WindowStyle Hidden `
  -RedirectStandardOutput 'D:\project\meta-org-saas\security-kernel\security-kernel-dev.out.log' `
  -RedirectStandardError 'D:\project\meta-org-saas\security-kernel\security-kernel-dev.err.log'
```

Start the backend from `backend/` so `MIGRATIONS_PATH=../migrations` points to
the root migration directory:

```powershell
Start-Process -FilePath 'go' `
  -ArgumentList 'run','./cmd/server' `
  -WorkingDirectory 'D:\project\meta-org-saas\backend' `
  -WindowStyle Hidden `
  -RedirectStandardOutput 'D:\project\meta-org-saas\backend\backend-dev.out.log' `
  -RedirectStandardError 'D:\project\meta-org-saas\backend\backend-dev.err.log' `
  -Environment @{
    'DATABASE_URL'='postgres://postgres:postgres@localhost:5432/meta_org_saas?sslmode=disable'
    'PLATFORM_DATABASE_URL'='postgres://postgres:postgres@localhost:5432/meta_org_saas?sslmode=disable'
    'TENANT_DATABASE_ADMIN_URL'='postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable'
    'TENANT_DATABASE_NAME_PREFIX'='meta_org_'
    'TENANT_MIGRATIONS_PATH'='../migrations/tenant'
    'MIGRATIONS_PATH'='../migrations'
    'JWT_SECRET'='dev-secret-change-in-production'
    'MODEL_SECRET_KEY'='0123456789abcdef0123456789abcdef'
    'META_ORG_MODE'='saas'
    'META_ORG_DISTRIBUTION_MODE'='saas'
    'META_ORG_LICENSE_MODE'='commercial'
    'META_ORG_PLATFORM_ADMIN_EMAIL'='platform-admin@local.test'
    'META_ORG_PLATFORM_ADMIN_PASSWORD_HASH'='$2a$10$/Dou0gOhCVFNGMitu8IUu.92HzEaG6iYWGxTTVUrSA1pkFvogvj22'
    'SECURITY_KERNEL_URL'='http://127.0.0.1:8090'
    'SECURITY_KERNEL_SHARED_SECRET'='dev-security-kernel-shared-secret'
    'SECURITY_KERNEL_ENFORCEMENT_MODE'='blocking'
    'CORS_ORIGINS'='http://localhost:3000,http://127.0.0.1:3000,http://172.16.0.2:3000'
  }
```

When using `Start-Process -ArgumentList` instead of `-Environment`, quote the
comma-separated CORS value inside the child command:

```powershell
Set-Item Env:CORS_ORIGINS 'http://localhost:3000,http://127.0.0.1:3000,http://172.16.0.2:3000'
```

If the value is not quoted, browser requests can fail with `Failed to fetch`
because the backend starts without a matching `Access-Control-Allow-Origin`
response header.

Start the frontend:

```powershell
Start-Process -FilePath 'cmd.exe' `
  -ArgumentList '/c','npm run dev -- --hostname 127.0.0.1' `
  -WorkingDirectory 'D:\project\meta-org-saas\frontend' `
  -WindowStyle Hidden `
  -RedirectStandardOutput 'D:\project\meta-org-saas\frontend\frontend-dev.out.log' `
  -RedirectStandardError 'D:\project\meta-org-saas\frontend\frontend-dev.err.log' `
  -Environment @{ 'NEXT_PUBLIC_API_URL'='http://127.0.0.1:8080/api/v1' }
```

## Verification

Build and test:

```powershell
cd D:\project\meta-org-saas\backend
go test ./...
go build ./cmd/server

cd D:\project\meta-org-saas\frontend
npm run build
npm run lint
npm run test:erp-operations
npm run test:operations
npm run test:system-admin
npm run test:runtime-workbench
# Then run the retail closed-loop checklist in docs/operations/retail-distribution-industry-solution.md.
```

Service checks:

```powershell
netstat -ano | Select-String ":3000|:8080|:8090"
curl.exe -s -i http://127.0.0.1:8090/healthz
curl.exe -s -i http://127.0.0.1:8080/api/v1/health
curl.exe -s -i http://127.0.0.1:3000
```

Database checks:

```powershell
$env:PGPASSWORD = 'postgres'
& 'C:\Program Files\PostgreSQL\18\bin\psql.exe' -h localhost -U postgres -d meta_org_saas -c "SELECT filename FROM platform.platform_migration_runs ORDER BY filename;"
& 'C:\Program Files\PostgreSQL\18\bin\psql.exe' -h localhost -U postgres -d meta_org_saas -c "SELECT COUNT(*) AS not_valid_constraints FROM pg_constraint WHERE NOT convalidated;"
& 'C:\Program Files\PostgreSQL\18\bin\psql.exe' -h localhost -U postgres -d meta_org_saas -c "SELECT u.email, u.account_status, pa.role FROM users u JOIN platform_admins pa ON pa.user_id = u.id WHERE u.email = 'platform-admin@local.test';"
& 'C:\Program Files\PostgreSQL\18\bin\psql.exe' -h localhost -U postgres -d postgres -c "SELECT datname FROM pg_database WHERE datname='meta_org_saas' OR datname ~ '^meta_org_[0-9a-f]{4}$' ORDER BY datname;"
```

Expected results:

- `platform.platform_migration_runs` contains only the staged baseline files and
  later consolidation migrations.
- `not_valid_constraints` is `0`.
- Platform admin row exists with `account_status=active` and
  `role=system_owner`.
- Health endpoints return `{"status":"ok"}`.

## 2026-06-28 Split-Database Rebuild Notes

Local rebuild target:

- Platform database: `meta_org_saas`
- Tenant database rule: `meta_org_xxxx`, where `xxxx` is the first four
  lowercase hex characters of the tenant organization UUID
- Trusted source backup: `meta_org_backup_20260626074450`
- Historical active database `meta_org` was removed after verification

Issues found and fixes applied:

- `relation platform.database_maintenance_jobs does not exist` and
  `relation platform.tenant_database_targets does not exist`: caused by using
  the old `meta_org` database whose migration history skipped new baseline
  structures. Rebuilt `meta_org_saas` from current staged migrations.
- `security_kernel_unavailable`: security kernel process/config mismatch during
  onboarding verification. Rebuilt and restarted the kernel with shared secret
  `dev-security-kernel-shared-secret`.
- `distribution_mode=saas` deserialization: the security kernel must accept the
  literal JSON value `saas`. If an older binary rejects it, rebuild the Rust
  `security-kernel` package before retrying onboarding.
- `owner_attestation verify: module_disabled`: owner attestation is a security
  feature gate, not a tenant business module. The backend now sends
  `module_key=general` with `enabled_features=["owner_attestation"]`.
- `Failed to fetch`: backend CORS startup value was not quoted when passed
  through PowerShell, so no `Access-Control-Allow-Origin` header was emitted.
  Restart with a quoted `CORS_ORIGINS` value.
- `relation gl_journal_entries does not exist`: manually created tenant
  databases had run the tenant SQL without expanding `tenantdb:include`.
  Recovered those tenants by applying `001_erp_code_baseline.sql` and aligning
  `tenant_migration_runs` to the tenant migrator checksum.

Verification after the rebuild:

- `go test ./...`
- `go build ./cmd/server`
- `cargo test -p security-kernel`
- `npm run build`
- `GET /api/v1/health`
- `GET /healthz` on security kernel
- SaaS platform admin login and `/auth/me`
- database maintenance job list
- new user registration plus `/onboarding/organization`, producing a physical
  tenant database named with the `meta_org_xxxx` rule
- `GET /finance/gl/trial-balance` against a finance-enabled tenant

## Current Verification Result

Verified on 2026-06-28:

- `go test ./...`: passed
- `go build ./cmd/server`: passed
- `npm run build`: passed
- `cargo test -p security-kernel`: passed
- Local ports listening: `3000`, `8080`, `8090`
- `platform.platform_migration_runs`: staged baseline and consolidation files only
- Platform database: `meta_org_saas`
- Tenant databases verified with `meta_org_xxxx` naming
- Cross-stage AI/ERP foreign keys rebuilt in `004`
- Unvalidated constraints: `0`
