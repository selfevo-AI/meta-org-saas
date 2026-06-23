# Local Non-Docker SaaS Deployment

This document records the local Windows deployment path for `meta-org-saas`
without Docker.

## Environment

- Repository: `D:\project\meta-org-saas`
- PostgreSQL: local service on `localhost:5432`
- Database: `meta_org`
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

This resets the local development database. Do not use against production.

```powershell
$env:PGPASSWORD = 'postgres'
& 'C:\Program Files\PostgreSQL\18\bin\psql.exe' -h localhost -U postgres -d postgres -c "DROP DATABASE IF EXISTS meta_org WITH (FORCE);"
& 'C:\Program Files\PostgreSQL\18\bin\psql.exe' -h localhost -U postgres -d postgres -c "CREATE DATABASE meta_org;"
```

## Start Services

Build and start the security kernel:

```powershell
cd D:\project\meta-org-saas\security-kernel
cargo build
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
    'DATABASE_URL'='postgres://postgres:postgres@localhost:5432/meta_org?sslmode=disable'
    'MIGRATIONS_PATH'='../migrations'
    'JWT_SECRET'='dev-secret-change-in-production'
    'MODEL_SECRET_KEY'='0123456789abcdef0123456789abcdef'
    'META_ORG_MODE'='saas'
    'META_ORG_DISTRIBUTION_MODE'='saas'
    'META_ORG_LICENSE_MODE'='commercial'
    'META_ORG_PLATFORM_ADMIN_EMAIL'='platform-admin@local.test'
    'META_ORG_PLATFORM_ADMIN_PASSWORD_HASH'='$2a$10$0Opm7t.xbz9Q8watzOLv6eKsV8.boNDPDd4gdli5vi6EN224ZcO2G'
    'SECURITY_KERNEL_URL'='http://127.0.0.1:8090'
    'SECURITY_KERNEL_SHARED_SECRET'='local-dev-shared-secret'
    'SECURITY_KERNEL_ENFORCEMENT_MODE'='audit'
    'CORS_ORIGINS'='http://localhost:3000,http://127.0.0.1:3000'
  }
```

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
npm run test:supply-chain
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
& 'C:\Program Files\PostgreSQL\18\bin\psql.exe' -h localhost -U postgres -d meta_org -c "SELECT filename FROM schema_migrations ORDER BY filename;"
& 'C:\Program Files\PostgreSQL\18\bin\psql.exe' -h localhost -U postgres -d meta_org -c "SELECT COUNT(*) AS not_valid_constraints FROM pg_constraint WHERE NOT convalidated;"
& 'C:\Program Files\PostgreSQL\18\bin\psql.exe' -h localhost -U postgres -d meta_org -c "SELECT u.email, u.account_status, pa.role FROM users u JOIN platform_admins pa ON pa.user_id = u.id WHERE u.email = 'platform-admin@local.test';"
```

Expected results:

- `schema_migrations` contains only `000_saas_platform_management_baseline.sql`,
  `001_erp_code_baseline.sql`, `002_erp_platform_integration_baseline.sql`, and
  `004_ai_capability_baseline.sql`.
- `not_valid_constraints` is `0`.
- Platform admin row exists with `account_status=active` and
  `role=system_owner`.
- Health endpoints return `{"status":"ok"}`.

## Current Verification Result

Verified on 2026-06-23:

- `go test ./...`: passed
- `go build ./cmd/server`: passed
- `npm run build`: passed
- `npm run lint`: passed
- `npm run test:erp-operations`: passed
- `npm run test:operations`: passed
- `npm run test:system-admin`: passed
- `npm run test:runtime-workbench`: passed
- `npm run test:supply-chain`: passed
- Local ports listening: `3000`, `8080`, `8090`
- `schema_migrations`: four staged baseline files only
- Cross-stage AI/ERP foreign keys rebuilt in `004`
- Unvalidated constraints: `0`
