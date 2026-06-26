# Repository Guidelines

## Project Structure & Module Organization

This repository contains a self-evolving organization management platform. The Go backend lives in `backend/`; its entry point is `backend/cmd/server/main.go`. Domain code is under `backend/internal/domain/<domain>/` with `model.go`, `repository.go`, `service.go`, and `handler.go`. Shared packages are in `backend/internal/pkg/`, and route registration is in `backend/internal/gateway/`.

The Next.js frontend lives in `frontend/`, with App Router files in `frontend/src/app/` and shared client utilities in `frontend/src/lib/`. SQL migrations are stored in root-level `migrations/` and are numbered in execution order.

## Build, Test, and Development Commands

- `docker compose up --build`: start PostgreSQL, backend, and frontend from `docker-compose.yml`.
- `cd backend && go run ./cmd/server`: run the API on port `8080`; outside Docker, set `MIGRATIONS_PATH=../migrations`.
- `cd backend && go build ./cmd/server`: compile the backend server.
- `cd backend && go test ./...`: run all Go tests.
- `cd frontend && npm run dev`: start the Next.js development server on port `3000`.
- `cd frontend && npm run build`: create a production frontend build.
- `cd frontend && npm run lint`: run Next.js linting.

## Local Development SaaS Login

This is a local development and verification rule only, not a system deployment rule or production credential policy.

For local non-Docker SaaS management testing, use the platform administrator account `platform-admin@local.test` with password `MetaOrgSaasDev!2026`.

When logging in through `http://127.0.0.1:3000`, first select the SaaS management login entry on the login page. Do not use the organization console login entry for platform management verification.

## Coding Style & Naming Conventions

Format Go code with `gofmt`; keep package names short and lowercase. Preserve the existing layering: handlers parse HTTP, services hold business rules, repositories handle persistence, and models define API/database shapes.

Frontend code uses TypeScript, React, Tailwind CSS, two-space indentation, single quotes, and no trailing semicolons. Prefer the `@/*` path alias for imports from `frontend/src/`.

## Frontend Internationalization

All user-facing frontend text must support Chinese and English. Use `LanguageProvider` and `useI18n` from `frontend/src/lib/i18n.tsx`; do not hardcode visible strings in new UI without adding both `zh` and `en` translations.

Field-level text is included in this requirement: form labels, placeholders, validation and error fallback text, button text, status badges, table headers, menu labels, empty states, panel titles, API operation names, and API operation parameter labels must all go through the i18n layer. New modules should use stable translation keys. Chinese literal keys are allowed only when migrating existing UI incrementally.

Any future frontend module or API-facing operation metadata must be designed with this same bilingual contract from the start, so human UI and agent-facing API workbench screens stay consistent.

## Testing Guidelines

No test files are currently present. Add Go tests as `*_test.go` beside the code they cover, and prefer table-driven tests for services and repositories. For frontend additions, add focused component or integration tests if a framework is introduced, and document the command in `frontend/package.json`.

## Commit & Pull Request Guidelines

Local history does not show an established commit convention beyond the initial clone. Use concise, imperative commit subjects such as `Add workflow approval tests` or `Fix identity token expiry`.

Pull requests should include a short description, affected backend/frontend domains, migration notes, configuration changes, and screenshots for visible UI changes. Mention any manual verification commands run.

## Security & Configuration Tips

Configuration is environment-driven in `backend/internal/pkg/config/config.go`. Do not commit real secrets; override `DATABASE_URL`, `JWT_SECRET`, `SERVER_PORT`, `CORS_ORIGINS`, and `NEXT_PUBLIC_API_URL` per environment.

## Database Baseline Governance

Any feature or refactor that changes database structure, table relationships, foreign keys, indexes, seed metadata, schema-generation logic, or table ownership must update the corresponding file in `migrations/` in the same change.

Use the current staged baseline as the source of truth: SaaS management platform first, ERP/industry baseline next, ERP-platform integration after that, and AI capability stages separately when applicable. Do not place schema changes only in backend/frontend code or informal notes. The relevant migration stage SQL and the migration-stage documentation must be updated together, and fresh-database migration verification must be run before claiming the change is complete.

SaaS platform management owns tenant organizations. Tenant runtime is single-organization, so tenant-facing base data must model the business hierarchy as departments, not an organization tree. Organization profile, subscription, entitlement, invitation, schema, AI/assistant, governance, and platform administration schema changes belong in `000_saas_platform_management_baseline.sql`; tenant department and ERP/industry-solution structures belong in `001_erp_code_baseline.sql` unless the staged baseline document explicitly assigns them elsewhere.

## Agent-Specific Instructions

Do not use scripts to batch delete files or directories. Delete files only one at a time with `Remove-Item`. If bulk deletion is required, stop and ask the user to confirm manually.
