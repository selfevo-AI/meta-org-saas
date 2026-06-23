# Meta-Org Production v1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the production-ready Meta-Org v1 platform from the existing HarnessCompany codebase, including full rename, Meta-Org entry, AI Gateway, streaming model adapters, tool runtime, cost accounting, generic finance export, context AI assistants, and production readiness.

**Architecture:** Keep the Go backend as a modular monolith and extend the existing domain pattern with `metaorg`, `aigateway`, `toolruntime`, and `finance`. Reuse existing `organization`, `project`, `workflow`, `governance`, `evolution`, `observability`, and `dashboard` services instead of duplicating business state. Upgrade the current Next.js single-page workspace with Meta-Org navigation, Developer Tools, finance views, and context AI assistants.

**Tech Stack:** Go 1.22, Chi v5, pgx v5, PostgreSQL 16, Next.js 16, React 19, TypeScript, Tailwind CSS, SSE streaming, Docker Compose.

---

## Scope Check

This is one production v1 release made of several tightly connected subsystems. Each task below must be implemented and committed independently. Do not begin a later task until the previous task has passing tests and a clean staged diff for that task.

Current uncommitted README changes exist in the working tree. Do not stage them unless the active task explicitly updates README content as part of the Meta-Org rename.

## File Structure Map

Create or modify these areas:

- Rename baseline:
  - Modify: `backend/go.mod`
  - Modify: Go imports under `backend/cmd/` and `backend/internal/`
  - Modify: `frontend/package.json`
  - Modify: `frontend/package-lock.json`
  - Modify: `docker-compose.yml`
  - Modify: `backend/internal/pkg/config/config.go`
  - Modify: `frontend/src/app/layout.tsx`
  - Modify: `frontend/src/lib/i18n.tsx`
  - Modify: `frontend/src/lib/auth.ts`
  - Modify: `frontend/src/app/page.tsx`
  - Modify: `frontend/src/lib/operations.ts`
  - Modify: `README.md`, `README_EN.md`
- Meta-Org backend:
  - Create: `backend/internal/domain/metaorg/model.go`
  - Create: `backend/internal/domain/metaorg/repository.go`
  - Create: `backend/internal/domain/metaorg/service.go`
  - Create: `backend/internal/domain/metaorg/handler.go`
  - Create: `backend/internal/domain/metaorg/service_test.go`
  - Modify: `backend/cmd/server/main.go`
  - Modify: `backend/internal/gateway/router.go`
- AI Gateway:
  - Create: `migrations/016_ai_gateway.sql`
  - Create: `backend/internal/domain/aigateway/model.go`
  - Create: `backend/internal/domain/aigateway/repository.go`
  - Create: `backend/internal/domain/aigateway/service.go`
  - Create: `backend/internal/domain/aigateway/handler.go`
  - Create: `backend/internal/domain/aigateway/provider.go`
  - Create: `backend/internal/domain/aigateway/provider_openai.go`
  - Create: `backend/internal/domain/aigateway/provider_anthropic.go`
  - Create: `backend/internal/domain/aigateway/provider_gemini.go`
  - Create: `backend/internal/domain/aigateway/pricing.go`
  - Create: `backend/internal/domain/aigateway/stream.go`
  - Create: `backend/internal/domain/aigateway/*_test.go`
  - Create: `backend/internal/pkg/secretbox/secretbox.go`
  - Create: `backend/internal/pkg/secretbox/secretbox_test.go`
  - Modify: `backend/internal/pkg/config/config.go`
  - Modify: `backend/cmd/server/main.go`
  - Modify: `backend/internal/gateway/router.go`
- Tool Runtime:
  - Create: `migrations/017_tool_runtime.sql`
  - Create: `backend/internal/domain/toolruntime/model.go`
  - Create: `backend/internal/domain/toolruntime/repository.go`
  - Create: `backend/internal/domain/toolruntime/service.go`
  - Create: `backend/internal/domain/toolruntime/handler.go`
  - Create: `backend/internal/domain/toolruntime/internal_tools.go`
  - Create: `backend/internal/domain/toolruntime/service_test.go`
  - Modify: `backend/cmd/server/main.go`
  - Modify: `backend/internal/gateway/router.go`
- Finance:
  - Create: `migrations/018_finance_exports.sql`
  - Create: `backend/internal/domain/finance/model.go`
  - Create: `backend/internal/domain/finance/repository.go`
  - Create: `backend/internal/domain/finance/service.go`
  - Create: `backend/internal/domain/finance/handler.go`
  - Create: `backend/internal/domain/finance/signature.go`
  - Create: `backend/internal/domain/finance/service_test.go`
  - Modify: `backend/cmd/server/main.go`
  - Modify: `backend/internal/gateway/router.go`
  - Modify: `backend/internal/domain/project/model.go`
  - Modify: `backend/internal/domain/project/repository.go`
  - Modify: `backend/internal/domain/project/service.go`
- Frontend:
  - Modify: `frontend/src/lib/api.ts`
  - Modify: `frontend/src/lib/operations.ts`
  - Modify: `frontend/src/lib/i18n.tsx`
  - Create: `frontend/src/lib/stream.ts`
  - Create: `frontend/src/app/developer-tools-workspace.tsx`
  - Create: `frontend/src/app/finance-workspace.tsx`
  - Create: `frontend/src/app/ai-assistant.tsx`
  - Modify: `frontend/src/app/page.tsx`
  - Modify: `frontend/src/app/organization-workspace.tsx`
  - Modify: `frontend/src/app/project-lifecycle-workspace.tsx`
  - Modify: `frontend/src/app/control-workspaces.tsx`
- CI and docs:
  - Create: `.github/workflows/ci.yml`
  - Create: `docs/operations/meta-org-production-runbook.md`
  - Create: `docs/operations/finance-adapter.md`

---

### Task 1: Full Meta-Org Rename

**Files:**
- Modify: `backend/go.mod`
- Modify: all Go files importing `github.com/harness-org/backend`
- Modify: `frontend/package.json`
- Modify: `frontend/package-lock.json`
- Modify: `docker-compose.yml`
- Modify: `backend/internal/pkg/config/config.go`
- Modify: `frontend/src/app/layout.tsx`
- Modify: `frontend/src/lib/i18n.tsx`
- Modify: `frontend/src/lib/auth.ts`
- Modify: `frontend/src/app/page.tsx`
- Modify: `frontend/src/lib/operations.ts`
- Modify: `README.md`
- Modify: `README_EN.md`

- [ ] **Step 1: Verify current rename targets**

Run:

```powershell
rg -n "HarnessCompany|Harness Company|Harness Organization|harness-org|harness_org|harness_token|harness_user|harness\.|harness-" README.md README_EN.md backend frontend docker-compose.yml
```

Expected: matches in README files, Go imports, frontend package metadata, localStorage keys, Docker database config, and sample organization names.

- [ ] **Step 2: Update Go module path**

Edit `backend/go.mod`:

```go
module github.com/selfevo-AI/meta-org-saas/backend
```

Replace every Go import prefix:

```go
github.com/harness-org/backend/
```

with:

```go
github.com/selfevo-AI/meta-org-saas/backend/
```

Run:

```powershell
gofmt -w backend/cmd/server/main.go backend/internal/gateway/router.go backend/internal/domain/capability/handler.go backend/internal/domain/organization/service.go backend/internal/domain/project/service.go
go test ./...
```

Expected: all Go packages compile and tests pass.

- [ ] **Step 3: Update database defaults**

Modify `backend/internal/pkg/config/config.go` default database URL:

```go
DatabaseURL: getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/meta_org?sslmode=disable"),
```

Modify `docker-compose.yml`:

```yaml
POSTGRES_DB: meta_org
DATABASE_URL: "postgres://postgres:postgres@postgres:5432/meta_org?sslmode=disable"
```

Run:

```powershell
go test ./...
```

Expected: backend tests still pass.

- [ ] **Step 4: Update frontend package and app identity**

Modify `frontend/package.json`:

```json
{
  "name": "meta-org-frontend"
}
```

Modify matching package names in `frontend/package-lock.json`.

Modify `frontend/src/app/layout.tsx` metadata:

```ts
export const metadata = {
  title: 'Meta-Org',
  description: 'AI-native organization operating platform',
}
```

Modify `frontend/src/lib/i18n.tsx` product labels:

```ts
'app.product': 'Meta-Org'
```

for both `en` and `zh`.

Run:

```powershell
npm run lint
npm run build
```

Expected: lint and build pass.

- [ ] **Step 5: Migrate frontend storage keys**

Modify `frontend/src/lib/auth.ts` so it reads old keys once and writes new keys:

```ts
const TOKEN_KEY = 'meta_org.token'
const USER_KEY = 'meta_org.user'
const LEGACY_TOKEN_KEY = 'harness_token'
const LEGACY_USER_KEY = 'harness_user'

function migrateLegacySession() {
  if (typeof localStorage === 'undefined') return
  const token = localStorage.getItem(TOKEN_KEY) || localStorage.getItem(LEGACY_TOKEN_KEY)
  const user = localStorage.getItem(USER_KEY) || localStorage.getItem(LEGACY_USER_KEY)
  if (token) localStorage.setItem(TOKEN_KEY, token)
  if (user) localStorage.setItem(USER_KEY, user)
  localStorage.removeItem(LEGACY_TOKEN_KEY)
  localStorage.removeItem(LEGACY_USER_KEY)
}
```

Call `migrateLegacySession()` at the start of `getToken()` and `getSessionUser()`.

Modify menu storage keys in `frontend/src/app/page.tsx`:

```ts
const menuStorageKey = 'meta_org.menu.groups.v1'
const expandedMenuStorageKey = 'meta_org.menu.expanded.v1'
const legacyMenuStorageKey = 'harness.menu.groups.v1'
const legacyExpandedMenuStorageKey = 'harness.menu.expanded.v1'
```

Implement the same read-once migration in `loadMenuGroups()` and `loadExpandedGroups()`.

Run:

```powershell
npm run lint
npm run build
```

Expected: lint and build pass.

- [ ] **Step 6: Update product docs and examples**

Update README files to use:

```text
Meta-Org
https://github.com/selfevo-AI/meta-org
github.com/selfevo-AI/meta-org-saas/backend
meta_org
meta-org-frontend
```

Update API Workbench sample organization names in `frontend/src/lib/operations.ts`:

```ts
name: 'Meta-Org'
description: 'AI-native organization operating platform'
```

Run:

```powershell
rg -n "HarnessCompany|Harness Organization|harness-org|harness_org|harness_token|harness_user|harness\.menu" README.md README_EN.md backend frontend docker-compose.yml
```

Expected: no matches except deliberate migration notes for old database/session names.

- [ ] **Step 7: Commit rename**

Run:

```powershell
git status --short
git diff --check
git add README.md README_EN.md backend frontend docker-compose.yml
git commit -m "Rename platform to Meta-Org"
```

Expected: commit succeeds; no unrelated files are staged.

---

### Task 2: Meta-Org Overview and Inbox Backend

**Files:**
- Create: `backend/internal/domain/metaorg/model.go`
- Create: `backend/internal/domain/metaorg/repository.go`
- Create: `backend/internal/domain/metaorg/service.go`
- Create: `backend/internal/domain/metaorg/handler.go`
- Create: `backend/internal/domain/metaorg/service_test.go`
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/internal/gateway/router.go`

- [ ] **Step 1: Write service tests**

Create `backend/internal/domain/metaorg/service_test.go`:

```go
package metaorg

import (
	"context"
	"testing"
	"time"
)

type fakeRepository struct{}

func (fakeRepository) Overview(context.Context) (Overview, error) {
	return Overview{
		GeneratedAt: time.Unix(100, 0).UTC(),
		Health: HealthSummary{
			OpenRequirements: 3,
			ActiveProjects:  2,
			ActiveAgents:    9,
			PendingApprovals: 4,
			UnexportedCost:  12.5,
			Currency:        "CNY",
		},
		Risks: []RiskItem{{ID: "risk-1", Title: "High-risk tool pending approval", Severity: "high", Source: "toolruntime"}},
	}, nil
}

func (fakeRepository) Inbox(context.Context, InboxFilter) ([]InboxItem, error) {
	return []InboxItem{{ID: "approval-1", Type: "tool_approval", Title: "Approve member matching", Status: "pending", Priority: "high"}}, nil
}

func TestServiceOverviewReturnsRepositoryData(t *testing.T) {
	svc := NewService(fakeRepository{})
	overview, err := svc.GetOverview(context.Background())
	if err != nil {
		t.Fatalf("GetOverview returned error: %v", err)
	}
	if overview.Health.ActiveAgents != 9 {
		t.Fatalf("ActiveAgents = %d, want 9", overview.Health.ActiveAgents)
	}
	if len(overview.Risks) != 1 {
		t.Fatalf("Risks length = %d, want 1", len(overview.Risks))
	}
}

func TestServiceInboxNormalizesLimit(t *testing.T) {
	svc := NewService(fakeRepository{})
	items, err := svc.GetInbox(context.Background(), InboxFilter{Limit: 0})
	if err != nil {
		t.Fatalf("GetInbox returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items length = %d, want 1", len(items))
	}
}
```

- [ ] **Step 2: Run failing tests**

Run:

```powershell
go test ./internal/domain/metaorg
```

Expected: FAIL because package files and types are not implemented.

- [ ] **Step 3: Implement models**

Create `backend/internal/domain/metaorg/model.go`:

```go
package metaorg

import "time"

type Overview struct {
	GeneratedAt time.Time      `json:"generated_at"`
	Health      HealthSummary  `json:"health"`
	Projects    ProjectSummary `json:"projects"`
	Agents      AgentSummary   `json:"agents"`
	Cost        CostSummary    `json:"cost"`
	Risks       []RiskItem     `json:"risks"`
	Activity    []ActivityItem `json:"activity"`
}

type HealthSummary struct {
	OpenRequirements int64   `json:"open_requirements"`
	ActiveProjects   int64   `json:"active_projects"`
	ActiveAgents     int64   `json:"active_agents"`
	PendingApprovals int64   `json:"pending_approvals"`
	UnexportedCost   float64 `json:"unexported_cost"`
	Currency         string  `json:"currency"`
}

type ProjectSummary struct {
	ByStatus map[string]int64 `json:"by_status"`
	OverBudget int64         `json:"over_budget"`
}

type AgentSummary struct {
	Total       int64            `json:"total"`
	Active      int64            `json:"active"`
	ByRiskLevel map[string]int64 `json:"by_risk_level"`
}

type CostSummary struct {
	Today        float64            `json:"today"`
	MonthToDate  float64            `json:"month_to_date"`
	Unexported   float64            `json:"unexported"`
	Currency     string             `json:"currency"`
	ByProvider   map[string]float64 `json:"by_provider"`
}

type RiskItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Severity string `json:"severity"`
	Source   string `json:"source"`
}

type ActivityItem struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Status    string    `json:"status,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type InboxFilter struct {
	Limit int
	Type  string
}

type InboxItem struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	Priority  string    `json:"priority"`
	Source    string    `json:"source,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
```

- [ ] **Step 4: Implement service**

Create `backend/internal/domain/metaorg/service.go`:

```go
package metaorg

import "context"

type Repository interface {
	Overview(ctx context.Context) (Overview, error)
	Inbox(ctx context.Context, filter InboxFilter) ([]InboxItem, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetOverview(ctx context.Context) (*Overview, error) {
	overview, err := s.repo.Overview(ctx)
	if err != nil {
		return nil, err
	}
	return &overview, nil
}

func (s *Service) GetInbox(ctx context.Context, filter InboxFilter) ([]InboxItem, error) {
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 50
	}
	return s.repo.Inbox(ctx, filter)
}
```

- [ ] **Step 5: Run service tests**

Run:

```powershell
go test ./internal/domain/metaorg
```

Expected: PASS.

- [ ] **Step 6: Implement repository and handler**

Create repository queries that aggregate from existing tables:

- `requirements` for open requirements.
- `projects` for active projects and status counts.
- `ai_agents` for agent counts and risk levels.
- `access_decisions` for high-risk governance signals.
- `signals` for high-priority evolution signals.
- `finance_export_batches` and `ai_usage_ledger` after Task 4/7 exist; before then return zero cost using `to_regclass()` guard queries.
- `tool_approvals` after Task 6 exists; before then return no tool approvals using `to_regclass()` guard queries.

Create `backend/internal/domain/metaorg/handler.go` with:

```go
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/meta-org/overview", h.getOverview)
	r.Get("/meta-org/inbox", h.getInbox)
}
```

The handler must return JSON and use the same error response shape used by existing handlers: `{"error":"message"}`.

- [ ] **Step 7: Register routes**

Modify `backend/cmd/server/main.go` to construct `metaorg.NewRepository(db)`, `metaorg.NewService(repo)`, and `metaorg.NewHandler(service)`.

Modify `backend/internal/gateway/router.go` dependency struct and protected route registration:

```go
MetaOrgHandler *metaorg.Handler
```

and:

```go
if deps.MetaOrgHandler != nil {
	deps.MetaOrgHandler.RegisterRoutes(r)
}
```

- [ ] **Step 8: Verify backend**

Run:

```powershell
gofmt -w backend/internal/domain/metaorg backend/cmd/server/main.go backend/internal/gateway/router.go
go test ./...
go build ./cmd/server
```

Expected: all tests pass and server builds.

- [ ] **Step 9: Commit Meta-Org backend**

Run:

```powershell
git add backend/internal/domain/metaorg backend/cmd/server/main.go backend/internal/gateway/router.go
git commit -m "Add Meta-Org overview backend"
```

Expected: commit succeeds.

---

### Task 3: Secret Storage and AI Gateway Database

**Files:**
- Create: `backend/internal/pkg/secretbox/secretbox.go`
- Create: `backend/internal/pkg/secretbox/secretbox_test.go`
- Create: `migrations/016_ai_gateway.sql`
- Create: `backend/internal/domain/aigateway/model.go`
- Create: `backend/internal/domain/aigateway/pricing.go`
- Create: `backend/internal/domain/aigateway/pricing_test.go`
- Modify: `backend/internal/pkg/config/config.go`

- [ ] **Step 1: Write secretbox tests**

Create `backend/internal/pkg/secretbox/secretbox_test.go`:

```go
package secretbox

import "testing"

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef"
	box, err := New(key)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	ciphertext, err := box.Encrypt("sk-test")
	if err != nil {
		t.Fatalf("Encrypt returned error: %v", err)
	}
	if ciphertext == "sk-test" {
		t.Fatalf("ciphertext should not equal plaintext")
	}
	plaintext, err := box.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt returned error: %v", err)
	}
	if plaintext != "sk-test" {
		t.Fatalf("plaintext = %q, want sk-test", plaintext)
	}
}

func TestNewRejectsShortKey(t *testing.T) {
	if _, err := New("short"); err == nil {
		t.Fatalf("New accepted short key")
	}
}
```

- [ ] **Step 2: Implement secretbox**

Create `backend/internal/pkg/secretbox/secretbox.go`:

```go
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

type Box struct {
	aead cipher.AEAD
}

func New(key string) (*Box, error) {
	if len(key) != 32 {
		return nil, errors.New("secretbox key must be 32 bytes")
	}
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	return &Box{aead: aead}, nil
}

func (b *Box) Encrypt(plaintext string) (string, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	sealed := b.aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.RawStdEncoding.EncodeToString(sealed), nil
}

func (b *Box) Decrypt(ciphertext string) (string, error) {
	raw, err := base64.RawStdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}
	nonceSize := b.aead.NonceSize()
	if len(raw) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	nonce := raw[:nonceSize]
	body := raw[nonceSize:]
	plaintext, err := b.aead.Open(nil, nonce, body, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt ciphertext: %w", err)
	}
	return string(plaintext), nil
}

func Mask(secret string) string {
	if len(secret) <= 8 {
		return "****"
	}
	return secret[:4] + "****" + secret[len(secret)-4:]
}
```

- [ ] **Step 3: Add config**

Modify `backend/internal/pkg/config/config.go`:

```go
ModelSecretKey string
```

and:

```go
ModelSecretKey: getEnv("MODEL_SECRET_KEY", "dev-model-secret-key-32-bytes!!"),
```

Ensure the default value is exactly 32 bytes. If using a different default string, count bytes before committing.

- [ ] **Step 4: Create AI gateway migration**

Create `migrations/016_ai_gateway.sql` with:

```sql
CREATE TABLE IF NOT EXISTS model_providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    provider_type TEXT NOT NULL CHECK (provider_type IN ('openai', 'anthropic', 'gemini')),
    base_url TEXT NOT NULL DEFAULT '',
    encrypted_api_key TEXT NOT NULL,
    masked_api_key TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled', 'error')),
    timeout_ms INT NOT NULL DEFAULT 60000,
    retry_count INT NOT NULL DEFAULT 1,
    risk_level TEXT NOT NULL DEFAULT 'medium' CHECK (risk_level IN ('low', 'medium', 'high', 'critical')),
    tags JSONB NOT NULL DEFAULT '[]',
    metadata JSONB NOT NULL DEFAULT '{}',
    last_test_status TEXT NOT NULL DEFAULT '',
    last_test_error TEXT NOT NULL DEFAULT '',
    last_tested_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS models (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id UUID NOT NULL REFERENCES model_providers(id) ON DELETE CASCADE,
    model_key TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    context_window INT NOT NULL DEFAULT 0,
    max_output_tokens INT NOT NULL DEFAULT 0,
    capabilities JSONB NOT NULL DEFAULT '[]',
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(provider_id, model_key)
);

CREATE TABLE IF NOT EXISTS model_price_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_id UUID NOT NULL REFERENCES models(id) ON DELETE CASCADE,
    input_price_per_1k NUMERIC(18,8) NOT NULL DEFAULT 0,
    output_price_per_1k NUMERIC(18,8) NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'CNY',
    effective_from TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    effective_to TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS ai_invocations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id UUID NOT NULL REFERENCES model_providers(id) ON DELETE RESTRICT,
    model_id UUID NOT NULL REFERENCES models(id) ON DELETE RESTRICT,
    mode TEXT NOT NULL CHECK (mode IN ('sync', 'stream')),
    status TEXT NOT NULL DEFAULT 'started' CHECK (status IN ('started', 'streaming', 'completed', 'failed', 'cancelled')),
    organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
    department_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    project_id UUID REFERENCES projects(id) ON DELETE SET NULL,
    requirement_id UUID REFERENCES requirements(id) ON DELETE SET NULL,
    workflow_id UUID REFERENCES workflow_instances(id) ON DELETE SET NULL,
    task_id UUID REFERENCES tasks(id) ON DELETE SET NULL,
    agent_id UUID REFERENCES ai_agents(id) ON DELETE SET NULL,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    capability_id UUID REFERENCES capabilities(id) ON DELETE SET NULL,
    source_surface TEXT NOT NULL DEFAULT '',
    request_hash TEXT NOT NULL DEFAULT '',
    provider_request_id TEXT NOT NULL DEFAULT '',
    input_tokens INT NOT NULL DEFAULT 0,
    output_tokens INT NOT NULL DEFAULT 0,
    estimated_input_tokens INT NOT NULL DEFAULT 0,
    estimated_output_tokens INT NOT NULL DEFAULT 0,
    cost_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'CNY',
    first_token_ms INT NOT NULL DEFAULT 0,
    duration_ms INT NOT NULL DEFAULT 0,
    error_type TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS ai_usage_ledger (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invocation_id UUID NOT NULL REFERENCES ai_invocations(id) ON DELETE RESTRICT,
    model_price_version_id UUID REFERENCES model_price_versions(id) ON DELETE SET NULL,
    ledger_type TEXT NOT NULL DEFAULT 'usage' CHECK (ledger_type IN ('usage', 'adjustment')),
    amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'CNY',
    input_tokens INT NOT NULL DEFAULT 0,
    output_tokens INT NOT NULL DEFAULT 0,
    posted_to_project_cost BOOLEAN NOT NULL DEFAULT FALSE,
    project_cost_entry_id UUID REFERENCES project_cost_entries(id) ON DELETE SET NULL,
    finance_export_line_id UUID,
    reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ai_invocations_project ON ai_invocations(project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ai_invocations_agent ON ai_invocations(agent_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ai_invocations_status ON ai_invocations(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ai_usage_ledger_invocation ON ai_usage_ledger(invocation_id);
CREATE INDEX IF NOT EXISTS idx_ai_usage_ledger_export ON ai_usage_ledger(finance_export_line_id);
```

- [ ] **Step 5: Add pricing tests**

Create `backend/internal/domain/aigateway/pricing_test.go`:

```go
package aigateway

import "testing"

func TestCalculateCost(t *testing.T) {
	cost := CalculateCost(TokenUsage{InputTokens: 1200, OutputTokens: 300}, Price{InputPer1K: 0.01, OutputPer1K: 0.03})
	if cost != 0.021 {
		t.Fatalf("cost = %.6f, want 0.021", cost)
	}
}
```

- [ ] **Step 6: Implement minimal AI gateway model and pricing**

Create `backend/internal/domain/aigateway/model.go` with provider, model, price, attribution, invocation, usage ledger, and request/response structs. Include these exact enum string constants:

```go
const (
	ProviderOpenAI    = "openai"
	ProviderAnthropic = "anthropic"
	ProviderGemini    = "gemini"
	ModeSync          = "sync"
	ModeStream        = "stream"
	StatusStarted     = "started"
	StatusStreaming   = "streaming"
	StatusCompleted   = "completed"
	StatusFailed      = "failed"
	StatusCancelled   = "cancelled"
)
```

Create `backend/internal/domain/aigateway/pricing.go`:

```go
package aigateway

type TokenUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type Price struct {
	InputPer1K  float64 `json:"input_price_per_1k"`
	OutputPer1K float64 `json:"output_price_per_1k"`
}

func CalculateCost(usage TokenUsage, price Price) float64 {
	return (float64(usage.InputTokens)/1000.0)*price.InputPer1K + (float64(usage.OutputTokens)/1000.0)*price.OutputPer1K
}
```

- [ ] **Step 7: Verify**

Run:

```powershell
gofmt -w backend/internal/pkg/secretbox backend/internal/domain/aigateway backend/internal/pkg/config/config.go
go test ./internal/pkg/secretbox ./internal/domain/aigateway ./internal/pkg/config
go test ./...
```

Expected: all tests pass.

- [ ] **Step 8: Commit**

Run:

```powershell
git add migrations/016_ai_gateway.sql backend/internal/pkg/secretbox backend/internal/domain/aigateway backend/internal/pkg/config/config.go
git commit -m "Add AI gateway storage foundation"
```

Expected: commit succeeds.

---

### Task 4: Provider Adapters and Streaming Parser

**Files:**
- Create: `backend/internal/domain/aigateway/provider.go`
- Create: `backend/internal/domain/aigateway/provider_openai.go`
- Create: `backend/internal/domain/aigateway/provider_anthropic.go`
- Create: `backend/internal/domain/aigateway/provider_gemini.go`
- Create: `backend/internal/domain/aigateway/stream.go`
- Create: `backend/internal/domain/aigateway/provider_test.go`

- [ ] **Step 1: Write provider contract tests**

Create `backend/internal/domain/aigateway/provider_test.go` with httptest servers for each provider shape:

```go
package aigateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIAdapterInvokeParsesContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"req_1","choices":[{"message":{"content":"hello"}}],"usage":{"prompt_tokens":2,"completion_tokens":3}}`))
	}))
	defer server.Close()

	adapter := NewOpenAIAdapter(server.URL, "sk-test", server.Client())
	resp, err := adapter.Invoke(context.Background(), ProviderRequest{Model: "gpt-test", Messages: []Message{{Role: "user", Content: "hi"}}})
	if err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}
	if resp.Content != "hello" {
		t.Fatalf("Content = %q, want hello", resp.Content)
	}
	if resp.Usage.InputTokens != 2 || resp.Usage.OutputTokens != 3 {
		t.Fatalf("usage = %+v, want 2/3", resp.Usage)
	}
}

func TestStreamParserEmitsDeltaAndDone(t *testing.T) {
	raw := strings.NewReader("data: {\"type\":\"delta\",\"text\":\"he\"}\n\ndata: {\"type\":\"delta\",\"text\":\"llo\"}\n\ndata: [DONE]\n\n")
	events, err := ParseSSE(raw)
	if err != nil {
		t.Fatalf("ParseSSE returned error: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("events length = %d, want 3", len(events))
	}
	if events[0].Data == "" || !events[2].Done {
		t.Fatalf("unexpected events: %+v", events)
	}
}
```

- [ ] **Step 2: Run failing tests**

Run:

```powershell
go test ./internal/domain/aigateway
```

Expected: FAIL because adapter contracts are missing.

- [ ] **Step 3: Implement provider contract**

Create `backend/internal/domain/aigateway/provider.go`:

```go
package aigateway

import "context"

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Schema      map[string]any `json:"schema"`
}

type ProviderRequest struct {
	Model       string           `json:"model"`
	Messages    []Message        `json:"messages"`
	Temperature *float64         `json:"temperature,omitempty"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
}

type ProviderResponse struct {
	ProviderRequestID string     `json:"provider_request_id"`
	Content           string     `json:"content"`
	Usage             TokenUsage `json:"usage"`
	ToolCalls         []ToolCall `json:"tool_calls,omitempty"`
}

type ToolCall struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type StreamEvent struct {
	Type      string     `json:"type"`
	Delta     string     `json:"delta,omitempty"`
	Usage     TokenUsage `json:"usage,omitempty"`
	ToolCall  *ToolCall  `json:"tool_call,omitempty"`
	Error     string     `json:"error,omitempty"`
	Done      bool       `json:"done,omitempty"`
}

type ProviderAdapter interface {
	Invoke(ctx context.Context, req ProviderRequest) (*ProviderResponse, error)
	Stream(ctx context.Context, req ProviderRequest) (<-chan StreamEvent, error)
}
```

- [ ] **Step 4: Implement OpenAI, Anthropic, and Gemini adapters**

Each adapter must:

- Use provider-native endpoint and payload.
- Accept a custom `*http.Client` for tests.
- Convert provider-specific response into `ProviderResponse`.
- Convert provider-specific stream events into `StreamEvent`.
- Return typed provider errors with status code and body summary.

Implement files:

```text
backend/internal/domain/aigateway/provider_openai.go
backend/internal/domain/aigateway/provider_anthropic.go
backend/internal/domain/aigateway/provider_gemini.go
```

Do not log request bodies because they may contain sensitive data.

- [ ] **Step 5: Implement SSE parser**

Create `backend/internal/domain/aigateway/stream.go` with `ParseSSE(io.Reader) ([]RawSSEEvent, error)` and streaming scanner helpers. The parser must:

- Accept `data: [DONE]`.
- Ignore empty comment lines.
- Preserve event data.
- Return scanner errors.

- [ ] **Step 6: Verify adapters**

Run:

```powershell
gofmt -w backend/internal/domain/aigateway
go test ./internal/domain/aigateway
go test ./...
```

Expected: all tests pass.

- [ ] **Step 7: Commit**

Run:

```powershell
git add backend/internal/domain/aigateway
git commit -m "Add model provider adapters"
```

Expected: commit succeeds.

---

### Task 5: AI Gateway Service, API, Usage Ledger, and SSE

**Files:**
- Create/modify: `backend/internal/domain/aigateway/repository.go`
- Create/modify: `backend/internal/domain/aigateway/service.go`
- Create/modify: `backend/internal/domain/aigateway/handler.go`
- Create: `backend/internal/domain/aigateway/service_test.go`
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/internal/gateway/router.go`

- [ ] **Step 1: Write service tests**

Create `backend/internal/domain/aigateway/service_test.go` with fake repository and fake adapter:

```go
package aigateway

import (
	"context"
	"testing"
)

type fakeGatewayRepo struct {
	recorded bool
}

func TestServiceInvokeRecordsUsage(t *testing.T) {
	repo := &fakeGatewayRepo{}
	adapter := fakeAdapter{resp: ProviderResponse{Content: "ok", Usage: TokenUsage{InputTokens: 10, OutputTokens: 20}}}
	svc := NewService(repo, AdapterRegistry{"openai": adapter})

	resp, err := svc.Invoke(context.Background(), InvokeInput{
		ProviderType: "openai",
		Model:        "gpt-test",
		Messages:     []Message{{Role: "user", Content: "hi"}},
		Price:        Price{InputPer1K: 0.01, OutputPer1K: 0.03},
	})
	if err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}
	if resp.Content != "ok" {
		t.Fatalf("content = %q, want ok", resp.Content)
	}
	if !repo.recorded {
		t.Fatalf("usage was not recorded")
	}
}
```

Implement fake repository methods as required by the service interface. The test must assert that invocation and usage ledger records are created for successful calls and failed calls.

- [ ] **Step 2: Implement service contract**

`Service` responsibilities:

- Create invocation record before provider call.
- Select provider adapter by provider type.
- Call sync or streaming adapter.
- Compute cost using active price version.
- Write `ai_usage_ledger` for final usage.
- Mark invocation completed, failed, or cancelled.
- Never execute tools directly; tool calls are delegated to `toolruntime` after Task 6.

- [ ] **Step 3: Implement handlers**

Routes:

```go
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/model-providers", h.createProvider)
	r.Get("/model-providers", h.listProviders)
	r.Patch("/model-providers/{id}", h.updateProvider)
	r.Post("/model-providers/{id}/rotate-key", h.rotateProviderKey)
	r.Post("/model-providers/{id}/test", h.testProvider)
	r.Post("/models", h.createModel)
	r.Get("/models", h.listModels)
	r.Patch("/models/{id}", h.updateModel)
	r.Post("/ai-gateway/invoke", h.invoke)
	r.Get("/ai-gateway/stream", h.stream)
	r.Get("/ai-gateway/invocations", h.listInvocations)
	r.Get("/ai-gateway/invocations/{id}", h.getInvocation)
	r.Get("/ai-gateway/cost-summary", h.costSummary)
}
```

SSE endpoint response headers:

```go
w.Header().Set("Content-Type", "text/event-stream")
w.Header().Set("Cache-Control", "no-cache")
w.Header().Set("Connection", "keep-alive")
```

SSE event format:

```text
event: delta
data: {"invocation_id":"...","delta":"text"}
```

- [ ] **Step 4: Register domain**

Modify `main.go` to create AI gateway repository/service/handler. Pass secret box from `MODEL_SECRET_KEY`.

Modify gateway dependencies and protected route registration.

- [ ] **Step 5: Verify**

Run:

```powershell
gofmt -w backend/internal/domain/aigateway backend/cmd/server/main.go backend/internal/gateway/router.go
go test ./internal/domain/aigateway
go test ./...
go build ./cmd/server
```

Expected: all tests pass and build succeeds.

- [ ] **Step 6: Commit**

Run:

```powershell
git add backend/internal/domain/aigateway backend/cmd/server/main.go backend/internal/gateway/router.go
git commit -m "Add AI gateway API and usage ledger"
```

Expected: commit succeeds.

---

### Task 6: Tool Runtime and Governance-Controlled Execution

**Files:**
- Create: `migrations/017_tool_runtime.sql`
- Create: `backend/internal/domain/toolruntime/model.go`
- Create: `backend/internal/domain/toolruntime/repository.go`
- Create: `backend/internal/domain/toolruntime/service.go`
- Create: `backend/internal/domain/toolruntime/handler.go`
- Create: `backend/internal/domain/toolruntime/internal_tools.go`
- Create: `backend/internal/domain/toolruntime/service_test.go`
- Modify: `backend/internal/domain/aigateway/service.go`
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/internal/gateway/router.go`

- [ ] **Step 1: Create migration**

Create `migrations/017_tool_runtime.sql`:

```sql
CREATE TABLE IF NOT EXISTS tool_definitions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    source_type TEXT NOT NULL CHECK (source_type IN ('internal_api', 'interface_file', 'manual_approval')),
    default_policy TEXT NOT NULL DEFAULT 'approve' CHECK (default_policy IN ('auto', 'notify', 'approve', 'deny')),
    risk_level TEXT NOT NULL DEFAULT 'medium' CHECK (risk_level IN ('low', 'medium', 'high', 'critical')),
    required_level TEXT NOT NULL DEFAULT 'L1',
    input_schema JSONB NOT NULL DEFAULT '{}',
    output_schema JSONB NOT NULL DEFAULT '{}',
    metadata JSONB NOT NULL DEFAULT '{}',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS interface_files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    file_type TEXT NOT NULL CHECK (file_type IN ('json', 'yaml', 'markdown')),
    content TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tool_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tool_id UUID NOT NULL REFERENCES tool_definitions(id) ON DELETE RESTRICT,
    invocation_id UUID REFERENCES ai_invocations(id) ON DELETE SET NULL,
    actor_id UUID NOT NULL,
    actor_type TEXT NOT NULL,
    organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
    department_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    project_id UUID REFERENCES projects(id) ON DELETE SET NULL,
    workflow_id UUID REFERENCES workflow_instances(id) ON DELETE SET NULL,
    task_id UUID REFERENCES tasks(id) ON DELETE SET NULL,
    idempotency_key TEXT NOT NULL DEFAULT '',
    policy TEXT NOT NULL,
    governance_decision TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'requested' CHECK (status IN ('requested', 'approval_required', 'approved', 'running', 'completed', 'rejected', 'denied', 'failed')),
    arguments JSONB NOT NULL DEFAULT '{}',
    result_summary TEXT NOT NULL DEFAULT '',
    result JSONB NOT NULL DEFAULT '{}',
    error_message TEXT NOT NULL DEFAULT '',
    duration_ms INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_tool_execution_idempotency
    ON tool_executions(tool_id, idempotency_key)
    WHERE idempotency_key <> '';

CREATE TABLE IF NOT EXISTS tool_approvals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id UUID NOT NULL REFERENCES tool_executions(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected', 'expired')),
    requested_by UUID REFERENCES users(id) ON DELETE SET NULL,
    reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    reason TEXT NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reviewed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_tool_executions_status ON tool_executions(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_tool_approvals_status ON tool_approvals(status, created_at DESC);
```

- [ ] **Step 2: Write policy tests**

Create `backend/internal/domain/toolruntime/service_test.go`:

```go
package toolruntime

import "testing"

func TestEffectivePolicyGovernanceOverridesAuto(t *testing.T) {
	policy := EffectivePolicy("auto", GovernanceResult{Decision: "approve", Allowed: false})
	if policy != "approve" {
		t.Fatalf("policy = %q, want approve", policy)
	}
}

func TestEffectivePolicyDenyWins(t *testing.T) {
	policy := EffectivePolicy("auto", GovernanceResult{Decision: "deny", Allowed: false})
	if policy != "deny" {
		t.Fatalf("policy = %q, want deny", policy)
	}
}
```

- [ ] **Step 3: Implement tool runtime models and policy**

Create `model.go` with `ToolDefinition`, `ToolExecution`, `ToolApproval`, `ExecuteToolInput`, `GovernanceResult`.

Create `service.go` policy function:

```go
func EffectivePolicy(defaultPolicy string, governance GovernanceResult) string {
	if governance.Decision == "deny" {
		return "deny"
	}
	if governance.Decision == "approve" {
		return "approve"
	}
	if governance.Decision == "notify" && defaultPolicy == "auto" {
		return "notify"
	}
	return defaultPolicy
}
```

- [ ] **Step 4: Implement internal tools**

Create `internal_tools.go` registering these names:

```text
requirement.analyze
project.match_members
project.bind_workflow
project.estimate_cost
project.create_cost_entry
governance.explain_decision
finance.prepare_export_batch
```

Each tool adapter must call existing domain services, not HTTP endpoints.

- [ ] **Step 5: Integrate governance**

Before any mutating tool executes, call existing `governance.DecideAccess` with:

```go
Action: tool.Name
Resource: "tool"
RequiredLevel: tool.RequiredLevel
RiskLevel: tool.RiskLevel
```

If effective policy is:

- `auto`: execute immediately.
- `notify`: execute and record notification-needed state.
- `approve`: create `tool_approvals` and return approval required.
- `deny`: record denied execution and do not run adapter.

- [ ] **Step 6: Register handlers**

Routes:

```go
r.Post("/tools", h.createTool)
r.Get("/tools", h.listTools)
r.Patch("/tools/{id}", h.updateTool)
r.Post("/tools/{id}/test", h.testTool)
r.Get("/tool-executions", h.listExecutions)
r.Get("/tool-executions/{id}", h.getExecution)
r.Post("/tool-approvals/{id}/approve", h.approve)
r.Post("/tool-approvals/{id}/reject", h.reject)
```

- [ ] **Step 7: Verify**

Run:

```powershell
gofmt -w backend/internal/domain/toolruntime backend/internal/domain/aigateway backend/cmd/server/main.go backend/internal/gateway/router.go
go test ./internal/domain/toolruntime
go test ./...
go build ./cmd/server
```

Expected: all tests pass and build succeeds.

- [ ] **Step 8: Commit**

Run:

```powershell
git add migrations/017_tool_runtime.sql backend/internal/domain/toolruntime backend/internal/domain/aigateway backend/cmd/server/main.go backend/internal/gateway/router.go
git commit -m "Add governance-controlled tool runtime"
```

Expected: commit succeeds.

---

### Task 7: Finance Adapter and Cost Export

**Files:**
- Create: `migrations/018_finance_exports.sql`
- Create: `backend/internal/domain/finance/model.go`
- Create: `backend/internal/domain/finance/repository.go`
- Create: `backend/internal/domain/finance/service.go`
- Create: `backend/internal/domain/finance/handler.go`
- Create: `backend/internal/domain/finance/signature.go`
- Create: `backend/internal/domain/finance/service_test.go`
- Modify: `backend/internal/domain/project/model.go`
- Modify: `backend/internal/domain/project/repository.go`
- Modify: `backend/internal/domain/project/service.go`
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/internal/gateway/router.go`

- [ ] **Step 1: Create migration**

Create `migrations/018_finance_exports.sql`:

```sql
CREATE TABLE IF NOT EXISTS finance_adapters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    endpoint_url TEXT NOT NULL,
    auth_type TEXT NOT NULL DEFAULT 'hmac' CHECK (auth_type IN ('hmac', 'bearer')),
    encrypted_secret TEXT NOT NULL,
    masked_secret TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled', 'error')),
    timeout_ms INT NOT NULL DEFAULT 30000,
    retry_count INT NOT NULL DEFAULT 3,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS finance_export_batches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    adapter_id UUID NOT NULL REFERENCES finance_adapters(id) ON DELETE RESTRICT,
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'ready', 'exporting', 'exported', 'acknowledged', 'posted', 'reconciled', 'failed', 'cancelled')),
    currency TEXT NOT NULL DEFAULT 'CNY',
    total_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    external_batch_id TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    idempotency_key TEXT NOT NULL UNIQUE,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    submitted_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS finance_export_lines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id UUID NOT NULL REFERENCES finance_export_batches(id) ON DELETE CASCADE,
    usage_ledger_id UUID REFERENCES ai_usage_ledger(id) ON DELETE RESTRICT,
    project_cost_entry_id UUID REFERENCES project_cost_entries(id) ON DELETE SET NULL,
    organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
    department_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    project_id UUID REFERENCES projects(id) ON DELETE SET NULL,
    provider_id UUID REFERENCES model_providers(id) ON DELETE SET NULL,
    model_id UUID REFERENCES models(id) ON DELETE SET NULL,
    amount NUMERIC(18,8) NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'CNY',
    external_line_id TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'ready',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS finance_webhook_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    adapter_id UUID NOT NULL REFERENCES finance_adapters(id) ON DELETE RESTRICT,
    batch_id UUID REFERENCES finance_export_batches(id) ON DELETE SET NULL,
    event_type TEXT NOT NULL,
    signature_valid BOOLEAN NOT NULL DEFAULT FALSE,
    payload JSONB NOT NULL DEFAULT '{}',
    processed BOOLEAN NOT NULL DEFAULT FALSE,
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_finance_batches_status ON finance_export_batches(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_finance_lines_batch ON finance_export_lines(batch_id);
CREATE INDEX IF NOT EXISTS idx_finance_webhooks_adapter ON finance_webhook_events(adapter_id, created_at DESC);

ALTER TABLE ai_usage_ledger
    ADD CONSTRAINT fk_ai_usage_finance_export_line
    FOREIGN KEY (finance_export_line_id) REFERENCES finance_export_lines(id) ON DELETE SET NULL;
```

- [ ] **Step 2: Write finance tests**

Create `backend/internal/domain/finance/service_test.go`:

```go
package finance

import "testing"

func TestSignAndVerifyPayload(t *testing.T) {
	body := []byte(`{"batch_id":"b1"}`)
	secret := "secret"
	signature := SignPayload(body, secret)
	if !VerifyPayload(body, signature, secret) {
		t.Fatalf("VerifyPayload returned false")
	}
	if VerifyPayload(body, signature, "other") {
		t.Fatalf("VerifyPayload accepted wrong secret")
	}
}

func TestBatchIdempotencyKeyStable(t *testing.T) {
	key1 := BatchIdempotencyKey("adapter-1", "2026-06-01", "2026-06-30", "CNY")
	key2 := BatchIdempotencyKey("adapter-1", "2026-06-01", "2026-06-30", "CNY")
	if key1 != key2 {
		t.Fatalf("keys differ: %q %q", key1, key2)
	}
}
```

- [ ] **Step 3: Implement signatures**

Create `backend/internal/domain/finance/signature.go`:

```go
package finance

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

func SignPayload(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func VerifyPayload(body []byte, signature string, secret string) bool {
	expected := SignPayload(body, secret)
	return hmac.Equal([]byte(expected), []byte(signature))
}
```

- [ ] **Step 4: Implement finance service**

Service responsibilities:

- Create and update finance adapters using encrypted secrets.
- Build export batches from unexported `ai_usage_ledger` rows.
- Submit batches to configured endpoint with idempotency key and HMAC signature.
- Process webhook callbacks and update batch/line statuses.
- Mark failed callbacks and export failures visible to Meta-Org inbox.

- [ ] **Step 5: Integrate project cost posting**

Add a project service method:

```go
func (s *Service) CreateCostEntryFromAIUsage(ctx context.Context, projectID uuid.UUID, input CreateCostEntryInput) (*CostEntry, error)
```

It must:

- Use source type `ai_usage`.
- Require `SourceID` to point to `ai_usage_ledger.id`.
- Use idempotency by checking existing `project_cost_entries` with same `source_type` and `source_id`.
- Return existing entry instead of creating duplicates.

- [ ] **Step 6: Register finance routes**

Routes:

```go
r.Post("/finance/adapters", h.createAdapter)
r.Get("/finance/adapters", h.listAdapters)
r.Patch("/finance/adapters/{id}", h.updateAdapter)
r.Post("/finance/adapters/{id}/test", h.testAdapter)
r.Post("/finance/export-batches", h.createExportBatch)
r.Get("/finance/export-batches", h.listExportBatches)
r.Get("/finance/export-batches/{id}", h.getExportBatch)
r.Post("/finance/export-batches/{id}/submit", h.submitExportBatch)
r.Post("/finance/webhooks/{adapterID}", h.receiveWebhook)
r.Get("/finance/reconciliation", h.listReconciliation)
```

- [ ] **Step 7: Verify**

Run:

```powershell
gofmt -w backend/internal/domain/finance backend/internal/domain/project backend/cmd/server/main.go backend/internal/gateway/router.go
go test ./internal/domain/finance ./internal/domain/project
go test ./...
go build ./cmd/server
```

Expected: all tests pass and build succeeds.

- [ ] **Step 8: Commit**

Run:

```powershell
git add migrations/018_finance_exports.sql backend/internal/domain/finance backend/internal/domain/project backend/cmd/server/main.go backend/internal/gateway/router.go
git commit -m "Add generic finance export integration"
```

Expected: commit succeeds.

---

### Task 8: Frontend Meta-Org Navigation, Developer Tools, Finance, and AI Assistant

**Files:**
- Modify: `frontend/src/lib/api.ts`
- Modify: `frontend/src/lib/operations.ts`
- Modify: `frontend/src/lib/i18n.tsx`
- Create: `frontend/src/lib/stream.ts`
- Create: `frontend/src/app/developer-tools-workspace.tsx`
- Create: `frontend/src/app/finance-workspace.tsx`
- Create: `frontend/src/app/ai-assistant.tsx`
- Modify: `frontend/src/app/page.tsx`
- Modify: `frontend/src/app/organization-workspace.tsx`
- Modify: `frontend/src/app/project-lifecycle-workspace.tsx`
- Modify: `frontend/src/app/control-workspaces.tsx`

- [ ] **Step 1: Add API types**

Modify `frontend/src/lib/api.ts` with exported types:

```ts
export interface MetaOrgOverview {
  generated_at: string
  health: {
    open_requirements: number
    active_projects: number
    active_agents: number
    pending_approvals: number
    unexported_cost: number
    currency: string
  }
  risks: Array<{ id: string; title: string; severity: string; source: string }>
  activity: RecentEvent[]
}

export interface InboxItem {
  id: string
  type: string
  title: string
  status: string
  priority: string
  source?: string
  created_at: string
}

export interface ModelProvider {
  id: string
  name: string
  provider_type: 'openai' | 'anthropic' | 'gemini'
  masked_api_key: string
  status: string
  risk_level: string
  last_test_status: string
  updated_at: string
}
```

Add functions:

```ts
export function getMetaOrgOverview(token: string) {
  return apiRequest<MetaOrgOverview>('/meta-org/overview', { token })
}

export function getMetaOrgInbox(token: string) {
  return apiRequest<InboxItem[]>('/meta-org/inbox', { token })
}
```

- [ ] **Step 2: Add SSE helper**

Create `frontend/src/lib/stream.ts`:

```ts
export interface StreamEvent<T = unknown> {
  event: string
  data: T
}

export async function streamSSE<T>(
  url: string,
  token: string,
  onEvent: (event: StreamEvent<T>) => void,
  signal?: AbortSignal,
) {
  const response = await fetch(url, {
    headers: { Authorization: `Bearer ${token}` },
    signal,
  })
  if (!response.ok || !response.body) {
    throw new Error(`HTTP ${response.status}`)
  }

  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''

  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })
    const frames = buffer.split('\n\n')
    buffer = frames.pop() ?? ''
    for (const frame of frames) {
      const eventLine = frame.split('\n').find((line) => line.startsWith('event:'))
      const dataLine = frame.split('\n').find((line) => line.startsWith('data:'))
      if (!dataLine) continue
      onEvent({
        event: eventLine ? eventLine.replace('event:', '').trim() : 'message',
        data: JSON.parse(dataLine.replace('data:', '').trim()) as T,
      })
    }
  }
}
```

- [ ] **Step 3: Create AI assistant component**

Create `frontend/src/app/ai-assistant.tsx`.

Component props:

```ts
interface AIAssistantProps {
  token: string
  contextType: 'meta_org' | 'requirement' | 'project' | 'organization' | 'governance' | 'developer_tools'
  contextID?: string
}
```

Required UI states:

- idle
- streaming
- tool approval required
- completed
- provider error
- governance denied
- cancelled

Use `streamSSE` for streaming and show event rows for tool calls.

- [ ] **Step 4: Create Developer Tools workspace**

Create `frontend/src/app/developer-tools-workspace.tsx` with tabs:

- Providers
- Models
- Tools
- Interface Files
- Invocation Logs
- Cost Summary

Providers tab must support:

- Create provider.
- Rotate key.
- Test connection.
- Show masked key only.

- [ ] **Step 5: Create Finance workspace**

Create `frontend/src/app/finance-workspace.tsx` with tabs:

- Adapters
- Export Batches
- Reconciliation
- Failed Webhooks

Export batch list must show:

- period
- status
- total amount
- currency
- external batch ID
- failure reason

- [ ] **Step 6: Update main navigation**

Modify `frontend/src/app/page.tsx`:

- Rename app mental model to Meta-Org.
- Add domains or workspace views for Developer Tools and Finance.
- Use `getMetaOrgOverview` and `getMetaOrgInbox` for homepage.
- Keep existing Organization, Governance, Evolution, Capability, Workflow, Requirement, Project, Delivery, Cost, Feedback entry points.

- [ ] **Step 7: Add assistants to existing workspaces**

Add right-side `AIAssistant` to:

- `project-lifecycle-workspace.tsx`
- `organization-workspace.tsx`
- `control-workspaces.tsx` governance section

Add inline buttons:

- Requirement: Analyze with AI.
- Project: Recommend members.
- Project: Generate workflow.
- Project: Estimate AI cost.
- Organization: Suggest positions.
- Governance: Explain decision.

Each button sends current context to AI Gateway through assistant request state, not direct provider calls.

- [ ] **Step 8: Add i18n keys**

Add English and Chinese keys for:

```text
Meta-Org
Developer Tools
Model Providers
Model Catalog
Tool Registry
Interface Files
Invocation Logs
Finance Exports
Export Batches
Reconciliation
AI Assistant
Streaming
Approval required
Governance denied
Estimated cost
Final cost
```

- [ ] **Step 9: Verify frontend**

Run:

```powershell
npm run lint
npm run build
```

Expected: lint and build pass.

- [ ] **Step 10: Commit**

Run:

```powershell
git add frontend/src
git commit -m "Add Meta-Org developer tools and AI assistant UI"
```

Expected: commit succeeds.

---

### Task 9: Observability, Dashboard Integration, and Meta-Org Inbox

**Files:**
- Modify: `backend/internal/domain/observability/model.go`
- Modify: `backend/internal/domain/observability/repository.go`
- Modify: `backend/internal/domain/dashboard/model.go`
- Modify: `backend/internal/domain/dashboard/repository.go`
- Modify: `backend/internal/domain/metaorg/repository.go`
- Modify: `backend/internal/domain/aigateway/service.go`
- Modify: `backend/internal/domain/toolruntime/service.go`
- Modify: `backend/internal/domain/finance/service.go`

- [ ] **Step 1: Add event categories**

Extend recent activity and metrics to include:

```text
ai_invocation
ai_stream
tool_execution
tool_approval
finance_export
finance_webhook
```

Use existing `traces`, `spans`, and `metrics` tables when possible.

- [ ] **Step 2: Emit traces**

AI Gateway emits:

- trace for each assistant interaction
- span for provider call
- metric for token usage
- metric for cost amount
- metric for streaming disconnect

Tool Runtime emits:

- span for tool execution
- metric for approval required
- metric for governance denied

Finance emits:

- span for export submit
- span for webhook callback
- metric for export failed
- metric for reconciliation difference

- [ ] **Step 3: Update Meta-Org inbox repository**

Inbox query must union:

- pending `tool_approvals`
- failed `finance_export_batches`
- high-priority unacknowledged `signals`
- pending `review_assignments`
- denied high-risk `access_decisions`

Return a normalized `InboxItem` list ordered by priority and created time.

- [ ] **Step 4: Verify backend**

Run:

```powershell
gofmt -w backend/internal/domain/observability backend/internal/domain/dashboard backend/internal/domain/metaorg backend/internal/domain/aigateway backend/internal/domain/toolruntime backend/internal/domain/finance
go test ./...
go build ./cmd/server
```

Expected: all tests pass and build succeeds.

- [ ] **Step 5: Commit**

Run:

```powershell
git add backend/internal/domain/observability backend/internal/domain/dashboard backend/internal/domain/metaorg backend/internal/domain/aigateway backend/internal/domain/toolruntime backend/internal/domain/finance
git commit -m "Integrate AI operations into Meta-Org observability"
```

Expected: commit succeeds.

---

### Task 10: CI, Migration Validation, and Operations Docs

**Files:**
- Create: `.github/workflows/ci.yml`
- Create: `docs/operations/meta-org-production-runbook.md`
- Create: `docs/operations/finance-adapter.md`
- Modify: `README.md`
- Modify: `README_EN.md`

- [ ] **Step 1: Add CI workflow**

Create `.github/workflows/ci.yml`:

```yaml
name: CI

on:
  push:
    branches: [master, main]
  pull_request:
    branches: [master, main]

jobs:
  backend:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:16-alpine
        env:
          POSTGRES_USER: postgres
          POSTGRES_PASSWORD: postgres
          POSTGRES_DB: meta_org
        ports:
          - 5432:5432
        options: >-
          --health-cmd "pg_isready -U postgres"
          --health-interval 5s
          --health-timeout 5s
          --health-retries 5
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - name: Test backend
        working-directory: backend
        env:
          DATABASE_URL: postgres://postgres:postgres@localhost:5432/meta_org?sslmode=disable
          MIGRATIONS_PATH: ../migrations
          MODEL_SECRET_KEY: 0123456789abcdef0123456789abcdef
        run: |
          go test ./...
          go build ./cmd/server

  frontend:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: npm
          cache-dependency-path: frontend/package-lock.json
      - name: Install
        working-directory: frontend
        run: npm ci
      - name: Lint
        working-directory: frontend
        run: npm run lint
      - name: Build
        working-directory: frontend
        run: npm run build
```

- [ ] **Step 2: Add production runbook**

Create `docs/operations/meta-org-production-runbook.md` covering:

- Required environment variables.
- Secret generation for `JWT_SECRET` and `MODEL_SECRET_KEY`.
- Provider setup steps.
- Finance adapter setup.
- Backup before migrating from `harness_org` to `meta_org`.
- Restore procedure.
- Streaming troubleshooting.
- Finance export retry procedure.
- Provider key rotation procedure.

- [ ] **Step 3: Add finance adapter doc**

Create `docs/operations/finance-adapter.md` covering:

- Export request payload.
- HMAC signature header.
- Webhook callback payload.
- Idempotency key rules.
- Status mapping.
- Retry expectations.

- [ ] **Step 4: Update README**

README must describe:

- Meta-Org identity.
- Fresh setup using `meta_org`.
- AI Gateway provider setup.
- Developer Tools.
- Finance exports.
- Test commands.

- [ ] **Step 5: Verify all**

Run:

```powershell
git diff --check
go test ./...
go build ./cmd/server
npm run lint
npm run build
```

Expected: all commands pass.

- [ ] **Step 6: Commit**

Run:

```powershell
git add .github docs README.md README_EN.md
git commit -m "Add Meta-Org production operations docs and CI"
```

Expected: commit succeeds.

---

## Final Verification

After all tasks are complete, run from repository root:

```powershell
git status --short
cd backend
go test ./...
go build ./cmd/server
cd ..\frontend
npm run lint
npm run build
cd ..
docker compose config
```

Expected:

- Working tree is clean.
- Backend tests pass.
- Backend builds.
- Frontend lint passes.
- Frontend builds.
- Docker Compose config renders with `meta_org`.

Manual acceptance scenario:

1. Start PostgreSQL, backend, and frontend with Docker Compose.
2. Register/login as a human user.
3. Open Meta-Org Home and verify overview and inbox load.
4. Create model providers for OpenAI, Anthropic, and Gemini with test keys in a non-production environment.
5. Test each provider.
6. Stream one response from each provider.
7. Confirm each invocation has usage and cost records.
8. Trigger a project member recommendation tool call.
9. Confirm governance decision and tool execution audit.
10. Approve a mutating tool call from inbox.
11. Post AI usage into project cost.
12. Create and submit a finance export batch to a test webhook endpoint.
13. Send a signed webhook callback and verify batch status updates.

