# Assistant Runtime and Verified Context Engine Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the phase-one backend framework for a Go-native assistant runtime, verified context engine, dictionary import flow, tool gate, and event sink while preserving existing assistant APIs.

**Architecture:** Keep the `assistant` package as the integration boundary, but split the current service loop into focused files: runtime, harness, event sink, context model, rule evaluator, dictionary service, context repository, context engine, and tool runner. New context metadata is persisted in append-only migrations, with a compatibility context source so existing hardcoded context behavior can remain available while project, finance, and governance seed rules are introduced.

**Tech Stack:** Go 1.22, Chi, pgx, PostgreSQL migrations, existing AI Gateway and Tool Runtime interfaces, `gopkg.in/yaml.v3` for YAML dictionary imports, standard-library CSV and XLSX parsing.

---

## Scope Check

The design spans runtime, context, dictionary metadata, and tool governance. This plan keeps them as one phase because each part is needed for a working framework boundary, but it limits phase one to backend framework behavior and three seed domains: `project`, `finance`, and `governance`.

The plan does not implement frontend dictionary management, automatic DDL execution, cross-organization inference, full session tree branching, or automatic weight learning.

## File Structure

- Create `migrations/035_assistant_context_engine.sql`: metadata tables for dictionary versions, domains, entities, fields, mappings, rules, proposals, migration drafts, and context packages.
- Modify `backend/go.mod` and `backend/go.sum`: add `gopkg.in/yaml.v3`.
- Create `backend/internal/domain/assistant/runtime_events.go`: event constants and typed runtime event payloads.
- Create `backend/internal/domain/assistant/harness.go`: immutable run harness and request structs.
- Create `backend/internal/domain/assistant/context_model.go`: dictionary, context package, validation, attention budget, and rule model types.
- Create `backend/internal/domain/assistant/context_import.go`: JSON/YAML/CSV import normalization.
- Create `backend/internal/domain/assistant/context_import_excel.go`: standard-library XLSX worksheet importer.
- Create `backend/internal/domain/assistant/context_dictionary.go`: `DictionaryService` validation, proposal, and migration draft orchestration.
- Create `backend/internal/domain/assistant/context_repository.go`: PostgreSQL persistence for dictionary metadata and context packages.
- Create `backend/internal/domain/assistant/context_rule_evaluator.go`: deterministic permission, workflow, finance, weight, and attention-budget logic.
- Create `backend/internal/domain/assistant/context_engine.go`: `VerifiedContextEngine` that composes repository, evaluator, and existing record resolver.
- Create `backend/internal/domain/assistant/tool_runner.go`: `ToolRunner` before/after tool-call gate wrapping `toolruntime.Service`.
- Create `backend/internal/domain/assistant/event_sink.go`: persistence and SSE-facing event sink.
- Create `backend/internal/domain/assistant/runtime.go`: assistant turn loop extracted from `Service`.
- Modify `backend/internal/domain/assistant/service.go`: delegate `Run` and `Resume` to runtime while keeping session, proposal, and skill methods.
- Modify `backend/internal/domain/assistant/model.go`: add context package IDs to run events where needed.
- Modify `backend/internal/domain/assistant/repository.go`: add small event/context persistence methods or use dedicated repository adapter.
- Modify `backend/internal/domain/assistant/handler.go`: keep existing run/resume endpoints and add dictionary import/preview endpoints.
- Modify `backend/cmd/server/main.go`: construct context repository, dictionary service, context engine, tool runner, event sink, and runtime.
- Test files:
  - `backend/internal/domain/assistant/context_import_test.go`
  - `backend/internal/domain/assistant/context_rule_evaluator_test.go`
  - `backend/internal/domain/assistant/context_engine_test.go`
  - `backend/internal/domain/assistant/tool_runner_test.go`
  - `backend/internal/domain/assistant/runtime_test.go`
  - Extend `backend/internal/domain/assistant/service_test.go`

---

### Task 1: Add Runtime Event and Harness Types

**Files:**
- Create: `backend/internal/domain/assistant/runtime_events.go`
- Create: `backend/internal/domain/assistant/harness.go`
- Test: `backend/internal/domain/assistant/runtime_test.go`

- [ ] **Step 1: Write the failing runtime event test**

Add this test to `backend/internal/domain/assistant/runtime_test.go`:

```go
package assistant

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewAssistantHarnessFreezesRunScope(t *testing.T) {
	actorID := uuid.New()
	orgID := uuid.New()
	sessionID := uuid.New()
	contextPackageID := uuid.New()

	harness := NewAssistantHarness(AssistantHarnessInput{
		SessionID:        sessionID,
		ActorID:          actorID,
		ActorType:        "internal_human",
		OrganizationID:   &orgID,
		ModuleKey:        "project",
		TargetType:       "project",
		ContextPackageID: contextPackageID,
		Model:            "gpt-4o-mini",
		ProviderType:     "openai",
	})

	if harness.SessionID != sessionID {
		t.Fatalf("session id = %s, want %s", harness.SessionID, sessionID)
	}
	if harness.ActorID != actorID || harness.ActorType != "internal_human" {
		t.Fatalf("actor = %s/%s, want %s/internal_human", harness.ActorID, harness.ActorType, actorID)
	}
	if harness.OrganizationID == nil || *harness.OrganizationID != orgID {
		t.Fatalf("organization id = %v, want %s", harness.OrganizationID, orgID)
	}
	if harness.ModuleKey != "project" || harness.TargetType != "project" {
		t.Fatalf("scope = %s/%s, want project/project", harness.ModuleKey, harness.TargetType)
	}
	if harness.ContextPackageID != contextPackageID {
		t.Fatalf("context package = %s, want %s", harness.ContextPackageID, contextPackageID)
	}
}

func TestRuntimeEventNamesAreStable(t *testing.T) {
	if RuntimeEventContextBuilt != "context_built" {
		t.Fatalf("context event = %q, want context_built", RuntimeEventContextBuilt)
	}
	if RuntimeEventToolBlocked != "tool_blocked" {
		t.Fatalf("tool blocked event = %q, want tool_blocked", RuntimeEventToolBlocked)
	}
	if RuntimeErrorFinanceValidationFailed != "finance_validation_failed" {
		t.Fatalf("finance error = %q, want finance_validation_failed", RuntimeErrorFinanceValidationFailed)
	}
}
```

- [ ] **Step 2: Run the test and verify it fails**

Run:

```powershell
cd backend
go test ./internal/domain/assistant -run "TestNewAssistantHarnessFreezesRunScope|TestRuntimeEventNamesAreStable"
```

Expected: FAIL because `NewAssistantHarness`, `AssistantHarnessInput`, and runtime event constants are not defined.

- [ ] **Step 3: Add runtime event constants**

Create `backend/internal/domain/assistant/runtime_events.go`:

```go
package assistant

const (
	RuntimeEventRunStarted         = "run_started"
	RuntimeEventContextBuilt       = "context_built"
	RuntimeEventMessageAppended    = "message_appended"
	RuntimeEventLLMInvoked         = "llm_invoked"
	RuntimeEventToolRequested      = "tool_requested"
	RuntimeEventToolBlocked        = "tool_blocked"
	RuntimeEventApprovalRequired   = "approval_required"
	RuntimeEventToolCompleted      = "tool_completed"
	RuntimeEventContextInvalidated = "context_invalidated"
	RuntimeEventProposalCreated    = "proposal_created"
	RuntimeEventMemoryUpdated      = "memory_updated"
	RuntimeEventRunCompleted       = "run_completed"
	RuntimeEventRunFailed          = "run_failed"

	RuntimeErrorContext               = "context_error"
	RuntimeErrorProvider              = "provider_error"
	RuntimeErrorTool                  = "tool_error"
	RuntimeErrorGovernanceDenied      = "governance_denied"
	RuntimeErrorFinanceValidationFailed = "finance_validation_failed"
	RuntimeErrorApprovalRejected      = "approval_rejected"
	RuntimeErrorRuntime               = "runtime_error"
)
```

- [ ] **Step 4: Add harness types**

Create `backend/internal/domain/assistant/harness.go`:

```go
package assistant

import "github.com/google/uuid"

type AssistantHarnessInput struct {
	SessionID        uuid.UUID
	ActorID          uuid.UUID
	ActorType        string
	OrganizationID   *uuid.UUID
	DepartmentID     *uuid.UUID
	PositionID       *uuid.UUID
	ProjectID        *uuid.UUID
	WorkflowID       *uuid.UUID
	TaskID           *uuid.UUID
	ModuleKey        string
	TargetType       string
	TargetID         *uuid.UUID
	ContextPackageID uuid.UUID
	ProviderID       *uuid.UUID
	ChannelID        *uuid.UUID
	ProviderType     string
	Model            string
	ServiceTier      string
	ReasoningEffort  string
}

type AssistantHarness struct {
	SessionID        uuid.UUID
	ActorID          uuid.UUID
	ActorType        string
	OrganizationID   *uuid.UUID
	DepartmentID     *uuid.UUID
	PositionID       *uuid.UUID
	ProjectID        *uuid.UUID
	WorkflowID       *uuid.UUID
	TaskID           *uuid.UUID
	ModuleKey        string
	TargetType       string
	TargetID         *uuid.UUID
	ContextPackageID uuid.UUID
	ProviderID       *uuid.UUID
	ChannelID        *uuid.UUID
	ProviderType     string
	Model            string
	ServiceTier      string
	ReasoningEffort  string
}

func NewAssistantHarness(input AssistantHarnessInput) AssistantHarness {
	return AssistantHarness{
		SessionID:        input.SessionID,
		ActorID:          input.ActorID,
		ActorType:        input.ActorType,
		OrganizationID:   cloneUUID(input.OrganizationID),
		DepartmentID:     cloneUUID(input.DepartmentID),
		PositionID:       cloneUUID(input.PositionID),
		ProjectID:        cloneUUID(input.ProjectID),
		WorkflowID:       cloneUUID(input.WorkflowID),
		TaskID:           cloneUUID(input.TaskID),
		ModuleKey:        normalizedModule(input.ModuleKey),
		TargetType:       input.TargetType,
		TargetID:         cloneUUID(input.TargetID),
		ContextPackageID: input.ContextPackageID,
		ProviderID:       cloneUUID(input.ProviderID),
		ChannelID:        cloneUUID(input.ChannelID),
		ProviderType:     input.ProviderType,
		Model:            input.Model,
		ServiceTier:      input.ServiceTier,
		ReasoningEffort:  input.ReasoningEffort,
	}
}

func cloneUUID(id *uuid.UUID) *uuid.UUID {
	if id == nil {
		return nil
	}
	copied := *id
	return &copied
}
```

- [ ] **Step 5: Run the test and verify it passes**

Run:

```powershell
cd backend
go test ./internal/domain/assistant -run "TestNewAssistantHarnessFreezesRunScope|TestRuntimeEventNamesAreStable"
```

Expected: PASS.

- [ ] **Step 6: Commit Task 1**

Run:

```powershell
git add backend/internal/domain/assistant/runtime_events.go backend/internal/domain/assistant/harness.go backend/internal/domain/assistant/runtime_test.go
git commit -m "Add assistant runtime harness types"
```

---

### Task 2: Add Context Metadata Migration

**Files:**
- Create: `migrations/035_assistant_context_engine.sql`

- [ ] **Step 1: Add migration file**

Create `migrations/035_assistant_context_engine.sql` with these tables:

```sql
CREATE TABLE IF NOT EXISTS context_dictionary_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_level TEXT NOT NULL CHECK (scope_level IN ('saas', 'organization', 'module')),
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    module_key TEXT NOT NULL DEFAULT '',
    version_key TEXT NOT NULL,
    source_type TEXT NOT NULL CHECK (source_type IN ('json', 'yaml', 'csv', 'xlsx')),
    source_name TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'ai_reviewed', 'approved', 'active', 'rejected', 'archived')),
    checksum TEXT NOT NULL DEFAULT '',
    imported_by UUID REFERENCES users(id) ON DELETE SET NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (scope_level, organization_id, module_key, version_key)
);

CREATE TABLE IF NOT EXISTS context_business_domains (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dictionary_version_id UUID NOT NULL REFERENCES context_dictionary_versions(id) ON DELETE CASCADE,
    module_key TEXT NOT NULL,
    name TEXT NOT NULL,
    scope_level TEXT NOT NULL CHECK (scope_level IN ('saas', 'organization', 'module')),
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'draft',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (dictionary_version_id, module_key)
);

CREATE TABLE IF NOT EXISTS context_entities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dictionary_version_id UUID NOT NULL REFERENCES context_dictionary_versions(id) ON DELETE CASCADE,
    domain_id UUID REFERENCES context_business_domains(id) ON DELETE CASCADE,
    entity_key TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (dictionary_version_id, entity_key)
);

CREATE TABLE IF NOT EXISTS context_fields (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dictionary_version_id UUID NOT NULL REFERENCES context_dictionary_versions(id) ON DELETE CASCADE,
    entity_id UUID NOT NULL REFERENCES context_entities(id) ON DELETE CASCADE,
    field_key TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    data_type TEXT NOT NULL DEFAULT 'string',
    semantic_type TEXT NOT NULL DEFAULT '',
    sensitivity_level TEXT NOT NULL DEFAULT 'normal'
        CHECK (sensitivity_level IN ('public', 'normal', 'sensitive', 'restricted')),
    base_weight DOUBLE PRECISION NOT NULL DEFAULT 1,
    is_finance_field BOOLEAN NOT NULL DEFAULT FALSE,
    is_workflow_field BOOLEAN NOT NULL DEFAULT FALSE,
    is_governance_field BOOLEAN NOT NULL DEFAULT FALSE,
    mask_strategy TEXT NOT NULL DEFAULT 'none',
    status TEXT NOT NULL DEFAULT 'draft',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (entity_id, field_key)
);

CREATE TABLE IF NOT EXISTS context_physical_mappings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dictionary_version_id UUID NOT NULL REFERENCES context_dictionary_versions(id) ON DELETE CASCADE,
    entity_id UUID NOT NULL REFERENCES context_entities(id) ON DELETE CASCADE,
    field_id UUID REFERENCES context_fields(id) ON DELETE CASCADE,
    table_name TEXT NOT NULL,
    column_name TEXT NOT NULL DEFAULT '',
    join_path JSONB NOT NULL DEFAULT '[]',
    tenant_column TEXT NOT NULL DEFAULT 'organization_id',
    status TEXT NOT NULL DEFAULT 'draft',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS context_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dictionary_version_id UUID NOT NULL REFERENCES context_dictionary_versions(id) ON DELETE CASCADE,
    module_key TEXT NOT NULL DEFAULT '',
    entity_key TEXT NOT NULL DEFAULT '',
    field_key TEXT NOT NULL DEFAULT '',
    rule_type TEXT NOT NULL CHECK (rule_type IN ('permission', 'workflow', 'finance', 'governance', 'weight', 'attention')),
    rule JSONB NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'approved', 'active', 'rejected', 'archived')),
    approved_by UUID REFERENCES users(id) ON DELETE SET NULL,
    approved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS context_change_proposals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dictionary_version_id UUID NOT NULL REFERENCES context_dictionary_versions(id) ON DELETE CASCADE,
    proposal_type TEXT NOT NULL DEFAULT 'dictionary_change',
    title TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    payload JSONB NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'approved', 'rejected', 'blocked')),
    reviewer_id UUID REFERENCES users(id) ON DELETE SET NULL,
    review_reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS context_migration_drafts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dictionary_version_id UUID NOT NULL REFERENCES context_dictionary_versions(id) ON DELETE CASCADE,
    title TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    sql_up TEXT NOT NULL DEFAULT '',
    sql_down TEXT NOT NULL DEFAULT '',
    risk_level TEXT NOT NULL DEFAULT 'medium',
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'reviewed', 'executed', 'rejected')),
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS context_packages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID REFERENCES assistant_sessions(id) ON DELETE SET NULL,
    dictionary_version_id UUID REFERENCES context_dictionary_versions(id) ON DELETE SET NULL,
    actor_id UUID NOT NULL,
    actor_type TEXT NOT NULL DEFAULT '',
    organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
    module_key TEXT NOT NULL DEFAULT '',
    target_type TEXT NOT NULL DEFAULT '',
    target_id UUID,
    workflow_id UUID REFERENCES workflow_instances(id) ON DELETE SET NULL,
    task_id UUID REFERENCES tasks(id) ON DELETE SET NULL,
    attention_core JSONB NOT NULL DEFAULT '[]',
    supporting_context JSONB NOT NULL DEFAULT '[]',
    risk_and_signals JSONB NOT NULL DEFAULT '[]',
    omissions JSONB NOT NULL DEFAULT '[]',
    weights JSONB NOT NULL DEFAULT '{}',
    validations JSONB NOT NULL DEFAULT '{}',
    provenance JSONB NOT NULL DEFAULT '{}',
    token_budget INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_context_dictionary_versions_scope
    ON context_dictionary_versions(scope_level, organization_id, module_key, status, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_context_rules_lookup
    ON context_rules(module_key, entity_key, field_key, rule_type, status);
CREATE INDEX IF NOT EXISTS idx_context_packages_session
    ON context_packages(session_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_context_packages_target
    ON context_packages(module_key, target_type, target_id, created_at DESC);
```

- [ ] **Step 2: Verify migration file syntax through backend tests**

Run:

```powershell
cd backend
go test ./internal/pkg/database ./internal/domain/assistant
```

Expected: PASS for packages that do not need a live database. If database migration tests are not present, the command should still compile.

- [ ] **Step 3: Commit Task 2**

Run:

```powershell
git add migrations/035_assistant_context_engine.sql
git commit -m "Add assistant context metadata migration"
```

---

### Task 3: Add Context Model Types

**Files:**
- Create: `backend/internal/domain/assistant/context_model.go`
- Test: `backend/internal/domain/assistant/context_rule_evaluator_test.go`

- [ ] **Step 1: Write the failing context model test**

Create `backend/internal/domain/assistant/context_rule_evaluator_test.go`:

```go
package assistant

import "testing"

func TestContextPackageAttentionCoreBudget(t *testing.T) {
	pkg := ContextPackage{
		AttentionCore: []ContextItem{
			{EntityKey: "project", FieldKey: "status", Value: "active", Weight: 10, EstimatedTokens: 20},
		},
		SupportingContext: []ContextItem{
			{EntityKey: "project", FieldKey: "description", Value: "long", Weight: 3, EstimatedTokens: 80},
		},
		TokenBudget: 100,
	}

	if pkg.AttentionCoreTokens() != 20 {
		t.Fatalf("attention core tokens = %d, want 20", pkg.AttentionCoreTokens())
	}
	if pkg.TotalEstimatedTokens() != 100 {
		t.Fatalf("total tokens = %d, want 100", pkg.TotalEstimatedTokens())
	}
}
```

- [ ] **Step 2: Run the test and verify it fails**

Run:

```powershell
cd backend
go test ./internal/domain/assistant -run TestContextPackageAttentionCoreBudget
```

Expected: FAIL because `ContextPackage` and `ContextItem` are undefined.

- [ ] **Step 3: Add context model types**

Create `backend/internal/domain/assistant/context_model.go`:

```go
package assistant

import "github.com/google/uuid"

const (
	ContextScopeSaaS         = "saas"
	ContextScopeOrganization = "organization"
	ContextScopeModule       = "module"

	ContextSourceJSON = "json"
	ContextSourceYAML = "yaml"
	ContextSourceCSV  = "csv"
	ContextSourceXLSX = "xlsx"

	DictionaryStatusDraft     = "draft"
	DictionaryStatusAIReviewed = "ai_reviewed"
	DictionaryStatusApproved  = "approved"
	DictionaryStatusActive    = "active"
	DictionaryStatusRejected  = "rejected"
	DictionaryStatusArchived  = "archived"
)

type DictionaryImportModel struct {
	VersionKey                string                         `json:"version_key" yaml:"version_key"`
	SourceType                string                         `json:"source_type" yaml:"source_type"`
	SourceName                string                         `json:"source_name" yaml:"source_name"`
	ScopeLevel                string                         `json:"scope_level" yaml:"scope_level"`
	OrganizationID            *uuid.UUID                     `json:"organization_id,omitempty" yaml:"organization_id,omitempty"`
	ModuleKey                 string                         `json:"module_key" yaml:"module_key"`
	Domains                   []ContextBusinessDomainInput   `json:"domains" yaml:"domains"`
	Entities                  []ContextEntityInput           `json:"entities" yaml:"entities"`
	Fields                    []ContextFieldInput            `json:"fields" yaml:"fields"`
	Relationships             []ContextRelationshipInput     `json:"relationships" yaml:"relationships"`
	WorkflowContextProfiles   []WorkflowContextProfileInput  `json:"workflow_context_profiles" yaml:"workflow_context_profiles"`
	FinanceValidationProfiles []FinanceValidationProfileInput `json:"finance_validation_profiles" yaml:"finance_validation_profiles"`
	Permissions               []ContextPermissionInput       `json:"permissions" yaml:"permissions"`
	MigrationIntents          []ContextMigrationIntentInput  `json:"migration_intents" yaml:"migration_intents"`
}

type ContextBusinessDomainInput struct {
	ModuleKey   string         `json:"module_key" yaml:"module_key"`
	Name        string         `json:"name" yaml:"name"`
	ScopeLevel  string         `json:"scope_level" yaml:"scope_level"`
	Metadata    map[string]any `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

type ContextEntityInput struct {
	EntityKey   string         `json:"entity_key" yaml:"entity_key"`
	ModuleKey   string         `json:"module_key" yaml:"module_key"`
	DisplayName string         `json:"display_name" yaml:"display_name"`
	Description string         `json:"description" yaml:"description"`
	Metadata    map[string]any `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

type ContextFieldInput struct {
	EntityKey         string         `json:"entity_key" yaml:"entity_key"`
	FieldKey          string         `json:"field_key" yaml:"field_key"`
	DisplayName       string         `json:"display_name" yaml:"display_name"`
	DataType          string         `json:"data_type" yaml:"data_type"`
	SemanticType      string         `json:"semantic_type" yaml:"semantic_type"`
	SensitivityLevel  string         `json:"sensitivity_level" yaml:"sensitivity_level"`
	BaseWeight        float64        `json:"base_weight" yaml:"base_weight"`
	IsFinanceField    bool           `json:"is_finance_field" yaml:"is_finance_field"`
	IsWorkflowField   bool           `json:"is_workflow_field" yaml:"is_workflow_field"`
	IsGovernanceField bool           `json:"is_governance_field" yaml:"is_governance_field"`
	MaskStrategy      string         `json:"mask_strategy" yaml:"mask_strategy"`
	TableName         string         `json:"table_name" yaml:"table_name"`
	ColumnName        string         `json:"column_name" yaml:"column_name"`
	Metadata          map[string]any `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

type ContextRelationshipInput struct {
	FromEntityKey string         `json:"from_entity_key" yaml:"from_entity_key"`
	ToEntityKey   string         `json:"to_entity_key" yaml:"to_entity_key"`
	RelationKey   string         `json:"relation_key" yaml:"relation_key"`
	JoinPath      []string       `json:"join_path" yaml:"join_path"`
	Metadata      map[string]any `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

type WorkflowContextProfileInput struct {
	ModuleKey       string             `json:"module_key" yaml:"module_key"`
	WorkflowStage   string             `json:"workflow_stage" yaml:"workflow_stage"`
	FieldMultipliers map[string]float64 `json:"field_multipliers" yaml:"field_multipliers"`
}

type FinanceValidationProfileInput struct {
	ModuleKey         string   `json:"module_key" yaml:"module_key"`
	FieldKey          string   `json:"field_key" yaml:"field_key"`
	RequiredStatus    []string `json:"required_status" yaml:"required_status"`
	UnverifiedAsSignal bool    `json:"unverified_as_signal" yaml:"unverified_as_signal"`
}

type ContextPermissionInput struct {
	ModuleKey        string   `json:"module_key" yaml:"module_key"`
	EntityKey        string   `json:"entity_key" yaml:"entity_key"`
	FieldKey         string   `json:"field_key" yaml:"field_key"`
	AllowedActorTypes []string `json:"allowed_actor_types" yaml:"allowed_actor_types"`
	MinimumTier      string   `json:"minimum_tier" yaml:"minimum_tier"`
}

type ContextMigrationIntentInput struct {
	IntentType string `json:"intent_type" yaml:"intent_type"`
	EntityKey  string `json:"entity_key" yaml:"entity_key"`
	FieldKey   string `json:"field_key" yaml:"field_key"`
	Reason     string `json:"reason" yaml:"reason"`
}

type ContextRequest struct {
	SessionID      uuid.UUID
	ActorID        uuid.UUID
	ActorType      string
	OrganizationID *uuid.UUID
	ModuleKey      string
	WorkflowID     *uuid.UUID
	TaskID         *uuid.UUID
	TargetType     string
	TargetID       *uuid.UUID
	Intent         string
	Mode           string
	RiskLevel      string
	TokenBudget    int
}

type ContextItem struct {
	EntityKey       string         `json:"entity_key"`
	FieldKey        string         `json:"field_key"`
	RecordID        string         `json:"record_id"`
	Value           any            `json:"value"`
	Weight          float64        `json:"weight"`
	EstimatedTokens int            `json:"estimated_tokens"`
	ValidationState string         `json:"validation_state"`
	Source          string         `json:"source"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

type ContextOmission struct {
	EntityKey string `json:"entity_key"`
	FieldKey  string `json:"field_key"`
	Reason    string `json:"reason"`
}

type ContextPackage struct {
	ID                uuid.UUID        `json:"id"`
	SessionID         uuid.UUID        `json:"session_id"`
	DictionaryVersionID *uuid.UUID    `json:"dictionary_version_id,omitempty"`
	AttentionCore     []ContextItem   `json:"attention_core"`
	SupportingContext []ContextItem   `json:"supporting_context"`
	RiskAndSignals    []ContextItem   `json:"risk_and_signals"`
	Omissions         []ContextOmission `json:"omissions"`
	Weights           map[string]float64 `json:"weights"`
	Validations       map[string]any   `json:"validations"`
	Provenance        map[string]any   `json:"provenance"`
	TokenBudget       int              `json:"token_budget"`
}

func (p ContextPackage) AttentionCoreTokens() int {
	total := 0
	for _, item := range p.AttentionCore {
		total += item.EstimatedTokens
	}
	return total
}

func (p ContextPackage) TotalEstimatedTokens() int {
	total := p.AttentionCoreTokens()
	for _, item := range p.SupportingContext {
		total += item.EstimatedTokens
	}
	for _, item := range p.RiskAndSignals {
		total += item.EstimatedTokens
	}
	return total
}
```

- [ ] **Step 4: Run the test and verify it passes**

Run:

```powershell
cd backend
go test ./internal/domain/assistant -run TestContextPackageAttentionCoreBudget
```

Expected: PASS.

- [ ] **Step 5: Commit Task 3**

Run:

```powershell
git add backend/internal/domain/assistant/context_model.go backend/internal/domain/assistant/context_rule_evaluator_test.go
git commit -m "Add assistant context model types"
```

---

### Task 4: Add Dictionary Import Normalization

**Files:**
- Modify: `backend/go.mod`
- Modify: `backend/go.sum`
- Create: `backend/internal/domain/assistant/context_import.go`
- Create: `backend/internal/domain/assistant/context_import_excel.go`
- Test: `backend/internal/domain/assistant/context_import_test.go`

- [ ] **Step 1: Add YAML dependency**

Run:

```powershell
cd backend
go get gopkg.in/yaml.v3@v3.0.1
```

Expected: `backend/go.mod` includes `gopkg.in/yaml.v3 v3.0.1`. If dependency download fails because of network restrictions, request escalation for `go get gopkg.in/yaml.v3@v3.0.1`.

- [ ] **Step 2: Write failing import tests**

Create `backend/internal/domain/assistant/context_import_test.go`:

```go
package assistant

import (
	"strings"
	"testing"
)

func TestNormalizeDictionaryImportJSON(t *testing.T) {
	raw := `{"version_key":"v1","source_type":"json","scope_level":"module","module_key":"project","domains":[{"module_key":"project","name":"Project","scope_level":"module"}],"entities":[{"entity_key":"project","module_key":"project","display_name":"Project"}],"fields":[{"entity_key":"project","field_key":"status","display_name":"Status","data_type":"string","base_weight":3,"table_name":"projects","column_name":"status"}]}`

	model, err := NormalizeDictionaryImport(DictionaryImportSource{
		SourceType: ContextSourceJSON,
		SourceName: "project.json",
		Content:    []byte(raw),
	})
	if err != nil {
		t.Fatalf("NormalizeDictionaryImport returned error: %v", err)
	}
	if model.VersionKey != "v1" || model.ModuleKey != "project" {
		t.Fatalf("model = %s/%s, want v1/project", model.VersionKey, model.ModuleKey)
	}
	if len(model.Fields) != 1 || model.Fields[0].FieldKey != "status" {
		t.Fatalf("fields = %#v, want status", model.Fields)
	}
}

func TestNormalizeDictionaryImportCSV(t *testing.T) {
	csv := strings.Join([]string{
		"module_key,entity_key,field_key,display_name,data_type,base_weight,table_name,column_name,is_finance_field",
		"finance,cost_ledger_entry,amount,Amount,number,8,cost_ledger_entries,amount,true",
	}, "\n")

	model, err := NormalizeDictionaryImport(DictionaryImportSource{
		SourceType: ContextSourceCSV,
		SourceName: "finance.csv",
		Content:    []byte(csv),
		ScopeLevel: ContextScopeModule,
		ModuleKey:  "finance",
		VersionKey:  "finance-v1",
	})
	if err != nil {
		t.Fatalf("NormalizeDictionaryImport returned error: %v", err)
	}
	if len(model.Fields) != 1 {
		t.Fatalf("fields len = %d, want 1", len(model.Fields))
	}
	field := model.Fields[0]
	if field.FieldKey != "amount" || !field.IsFinanceField || field.BaseWeight != 8 {
		t.Fatalf("field = %#v, want amount finance weight 8", field)
	}
}
```

- [ ] **Step 3: Run tests and verify they fail**

Run:

```powershell
cd backend
go test ./internal/domain/assistant -run "TestNormalizeDictionaryImportJSON|TestNormalizeDictionaryImportCSV"
```

Expected: FAIL because `NormalizeDictionaryImport` and `DictionaryImportSource` are undefined.

- [ ] **Step 4: Add JSON/YAML/CSV normalizer**

Create `backend/internal/domain/assistant/context_import.go`:

```go
package assistant

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type DictionaryImportSource struct {
	SourceType string
	SourceName string
	Content    []byte
	ScopeLevel string
	ModuleKey  string
	VersionKey  string
}

func NormalizeDictionaryImport(source DictionaryImportSource) (DictionaryImportModel, error) {
	switch strings.ToLower(strings.TrimSpace(source.SourceType)) {
	case ContextSourceJSON:
		return normalizeJSONDictionary(source)
	case ContextSourceYAML:
		return normalizeYAMLDictionary(source)
	case ContextSourceCSV:
		return normalizeCSVDictionary(source)
	case ContextSourceXLSX:
		return normalizeXLSXDictionary(source)
	default:
		return DictionaryImportModel{}, fmt.Errorf("%w: unsupported dictionary source type", ErrValidation)
	}
}

func normalizeJSONDictionary(source DictionaryImportSource) (DictionaryImportModel, error) {
	var model DictionaryImportModel
	if err := json.Unmarshal(source.Content, &model); err != nil {
		return model, fmt.Errorf("%w: invalid dictionary json", ErrValidation)
	}
	return fillImportDefaults(model, source), nil
}

func normalizeYAMLDictionary(source DictionaryImportSource) (DictionaryImportModel, error) {
	var model DictionaryImportModel
	if err := yaml.Unmarshal(source.Content, &model); err != nil {
		return model, fmt.Errorf("%w: invalid dictionary yaml", ErrValidation)
	}
	return fillImportDefaults(model, source), nil
}

func normalizeCSVDictionary(source DictionaryImportSource) (DictionaryImportModel, error) {
	reader := csv.NewReader(bytes.NewReader(source.Content))
	reader.TrimLeadingSpace = true
	rows, err := reader.ReadAll()
	if err != nil {
		return DictionaryImportModel{}, fmt.Errorf("%w: invalid dictionary csv", ErrValidation)
	}
	if len(rows) < 2 {
		return DictionaryImportModel{}, fmt.Errorf("%w: csv dictionary requires header and one row", ErrValidation)
	}
	header := map[string]int{}
	for i, name := range rows[0] {
		header[strings.TrimSpace(name)] = i
	}
	model := DictionaryImportModel{
		VersionKey: source.VersionKey,
		SourceType: ContextSourceCSV,
		SourceName: source.SourceName,
		ScopeLevel: source.ScopeLevel,
		ModuleKey: source.ModuleKey,
	}
	entitySeen := map[string]bool{}
	domainSeen := map[string]bool{}
	for _, row := range rows[1:] {
		moduleKey := csvValue(row, header, "module_key")
		entityKey := csvValue(row, header, "entity_key")
		fieldKey := csvValue(row, header, "field_key")
		if moduleKey == "" || entityKey == "" || fieldKey == "" {
			return model, fmt.Errorf("%w: csv rows require module_key, entity_key, and field_key", ErrValidation)
		}
		if !domainSeen[moduleKey] {
			model.Domains = append(model.Domains, ContextBusinessDomainInput{ModuleKey: moduleKey, Name: moduleKey, ScopeLevel: source.ScopeLevel})
			domainSeen[moduleKey] = true
		}
		if !entitySeen[entityKey] {
			model.Entities = append(model.Entities, ContextEntityInput{EntityKey: entityKey, ModuleKey: moduleKey, DisplayName: entityKey})
			entitySeen[entityKey] = true
		}
		model.Fields = append(model.Fields, ContextFieldInput{
			EntityKey:      entityKey,
			FieldKey:       fieldKey,
			DisplayName:    csvValue(row, header, "display_name"),
			DataType:       firstNonEmpty(csvValue(row, header, "data_type"), "string"),
			BaseWeight:     csvFloat(row, header, "base_weight", 1),
			IsFinanceField: csvBool(row, header, "is_finance_field"),
			TableName:      csvValue(row, header, "table_name"),
			ColumnName:     csvValue(row, header, "column_name"),
		})
	}
	return fillImportDefaults(model, source), nil
}

func fillImportDefaults(model DictionaryImportModel, source DictionaryImportSource) DictionaryImportModel {
	if model.SourceType == "" {
		model.SourceType = source.SourceType
	}
	if model.SourceName == "" {
		model.SourceName = source.SourceName
	}
	if model.ScopeLevel == "" {
		model.ScopeLevel = firstNonEmpty(source.ScopeLevel, ContextScopeModule)
	}
	if model.ModuleKey == "" {
		model.ModuleKey = source.ModuleKey
	}
	if model.VersionKey == "" {
		model.VersionKey = source.VersionKey
	}
	return model
}

func csvValue(row []string, header map[string]int, key string) string {
	index, ok := header[key]
	if !ok || index < 0 || index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

func csvBool(row []string, header map[string]int, key string) bool {
	value := strings.ToLower(csvValue(row, header, key))
	return value == "true" || value == "1" || value == "yes"
}

func csvFloat(row []string, header map[string]int, key string, fallback float64) float64 {
	value := csvValue(row, header, key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}
```

- [ ] **Step 5: Add XLSX importer skeleton with standard-library parsing**

Create `backend/internal/domain/assistant/context_import_excel.go`:

```go
package assistant

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

type xlsxSharedStrings struct {
	Items []struct {
		Text string `xml:"t"`
	} `xml:"si"`
}

type xlsxWorksheet struct {
	Rows []struct {
		Cells []struct {
			Ref   string `xml:"r,attr"`
			Type  string `xml:"t,attr"`
			Value string `xml:"v"`
		} `xml:"c"`
	} `xml:"sheetData>row"`
}

func normalizeXLSXDictionary(source DictionaryImportSource) (DictionaryImportModel, error) {
	reader, err := zip.NewReader(bytes.NewReader(source.Content), int64(len(source.Content)))
	if err != nil {
		return DictionaryImportModel{}, fmt.Errorf("%w: invalid xlsx archive", ErrValidation)
	}
	shared, err := readXLSXSharedStrings(reader)
	if err != nil {
		return DictionaryImportModel{}, err
	}
	rows, err := readXLSXFirstSheet(reader, shared)
	if err != nil {
		return DictionaryImportModel{}, err
	}
	var csvText strings.Builder
	for rowIndex, row := range rows {
		if rowIndex > 0 {
			csvText.WriteByte('\n')
		}
		for colIndex, cell := range row {
			if colIndex > 0 {
				csvText.WriteByte(',')
			}
			csvText.WriteString(escapeCSVCell(cell))
		}
	}
	source.SourceType = ContextSourceCSV
	source.Content = []byte(csvText.String())
	model, err := normalizeCSVDictionary(source)
	model.SourceType = ContextSourceXLSX
	return model, err
}

func readXLSXSharedStrings(reader *zip.Reader) ([]string, error) {
	file := findZipFile(reader, "xl/sharedStrings.xml")
	if file == nil {
		return nil, nil
	}
	data, err := readZipFile(file)
	if err != nil {
		return nil, fmt.Errorf("%w: read shared strings", ErrValidation)
	}
	var parsed xlsxSharedStrings
	if err := xml.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("%w: parse shared strings", ErrValidation)
	}
	values := make([]string, 0, len(parsed.Items))
	for _, item := range parsed.Items {
		values = append(values, item.Text)
	}
	return values, nil
}

func readXLSXFirstSheet(reader *zip.Reader, shared []string) ([][]string, error) {
	file := findZipFile(reader, "xl/worksheets/sheet1.xml")
	if file == nil {
		return nil, fmt.Errorf("%w: xlsx sheet1.xml is required", ErrValidation)
	}
	data, err := readZipFile(file)
	if err != nil {
		return nil, fmt.Errorf("%w: read xlsx sheet", ErrValidation)
	}
	var parsed xlsxWorksheet
	if err := xml.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("%w: parse xlsx sheet", ErrValidation)
	}
	rows := [][]string{}
	for _, row := range parsed.Rows {
		cells := []string{}
		for _, cell := range row.Cells {
			value := cell.Value
			if cell.Type == "s" {
				index, ok := parseSmallInt(value)
				if ok && index >= 0 && index < len(shared) {
					value = shared[index]
				}
			}
			cells = append(cells, value)
		}
		rows = append(rows, cells)
	}
	return rows, nil
}

func findZipFile(reader *zip.Reader, name string) *zip.File {
	for _, file := range reader.File {
		if file.Name == name {
			return file
		}
	}
	return nil
}

func readZipFile(file *zip.File) ([]byte, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func escapeCSVCell(value string) string {
	if strings.ContainsAny(value, ",\"\n\r") {
		return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
	}
	return value
}

func parseSmallInt(value string) (int, bool) {
	total := 0
	if value == "" {
		return 0, false
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return 0, false
		}
		total = total*10 + int(ch-'0')
	}
	return total, true
}
```

- [ ] **Step 6: Run import tests**

Run:

```powershell
cd backend
go test ./internal/domain/assistant -run "TestNormalizeDictionaryImportJSON|TestNormalizeDictionaryImportCSV"
```

Expected: PASS.

- [ ] **Step 7: Commit Task 4**

Run:

```powershell
git add backend/go.mod backend/go.sum backend/internal/domain/assistant/context_import.go backend/internal/domain/assistant/context_import_excel.go backend/internal/domain/assistant/context_import_test.go
git commit -m "Add assistant dictionary import normalization"
```

---

### Task 5: Add Dictionary Service Validation and Proposals

**Files:**
- Create: `backend/internal/domain/assistant/context_dictionary.go`
- Test: `backend/internal/domain/assistant/context_import_test.go`

- [ ] **Step 1: Add failing validation test**

Append to `backend/internal/domain/assistant/context_import_test.go`:

```go
func TestDictionaryServiceRejectsFieldWithoutEntity(t *testing.T) {
	svc := NewDictionaryService(nil, nil)
	model := DictionaryImportModel{
		VersionKey: "bad-v1",
		SourceType: ContextSourceJSON,
		ScopeLevel: ContextScopeModule,
		ModuleKey: "project",
		Fields: []ContextFieldInput{{EntityKey: "missing", FieldKey: "status"}},
	}

	result, err := svc.ValidateImport(model)
	if err == nil {
		t.Fatalf("ValidateImport returned nil error")
	}
	if len(result.Errors) != 1 {
		t.Fatalf("errors len = %d, want 1", len(result.Errors))
	}
	if result.Errors[0].Code != "unknown_entity" {
		t.Fatalf("error code = %s, want unknown_entity", result.Errors[0].Code)
	}
}

func TestDictionaryServiceCreatesMigrationDraftForIntent(t *testing.T) {
	repo := &fakeDictionaryRepository{}
	svc := NewDictionaryService(repo, nil)
	model := DictionaryImportModel{
		VersionKey: "project-v1",
		SourceType: ContextSourceJSON,
		ScopeLevel: ContextScopeModule,
		ModuleKey: "project",
		Domains: []ContextBusinessDomainInput{{ModuleKey: "project", Name: "Project", ScopeLevel: ContextScopeModule}},
		Entities: []ContextEntityInput{{EntityKey: "project", ModuleKey: "project"}},
		Fields: []ContextFieldInput{{EntityKey: "project", FieldKey: "priority", TableName: "projects", ColumnName: "priority"}},
		MigrationIntents: []ContextMigrationIntentInput{{IntentType: "add_column", EntityKey: "project", FieldKey: "priority", Reason: "prioritize work"}},
	}

	created, err := svc.Import(context.Background(), DictionaryImportRequest{Model: model})
	if err != nil {
		t.Fatalf("Import returned error: %v", err)
	}
	if created.DictionaryVersionID == uuid.Nil {
		t.Fatalf("dictionary version id is nil")
	}
	if len(repo.migrationDrafts) != 1 {
		t.Fatalf("migration drafts = %d, want 1", len(repo.migrationDrafts))
	}
	if repo.migrationDrafts[0].RiskLevel != "medium" {
		t.Fatalf("risk level = %s, want medium", repo.migrationDrafts[0].RiskLevel)
	}
}
```

Also add imports at the top of `context_import_test.go`:

```go
import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
)
```

- [ ] **Step 2: Run validation tests and verify failure**

Run:

```powershell
cd backend
go test ./internal/domain/assistant -run "TestDictionaryServiceRejectsFieldWithoutEntity|TestDictionaryServiceCreatesMigrationDraftForIntent"
```

Expected: FAIL because `DictionaryService` types are undefined.

- [ ] **Step 3: Add dictionary service**

Create `backend/internal/domain/assistant/context_dictionary.go`:

```go
package assistant

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type DictionaryRepository interface {
	CreateDictionaryVersion(context.Context, DictionaryImportModel, *uuid.UUID) (uuid.UUID, error)
	CreateContextChangeProposal(context.Context, ContextChangeProposalInput) (uuid.UUID, error)
	CreateContextMigrationDraft(context.Context, ContextMigrationDraftInput) (uuid.UUID, error)
}

type SuggestionProvider interface {
	SuggestDictionaryChanges(context.Context, DictionaryImportModel) (map[string]any, error)
}

type DictionaryService struct {
	repo       DictionaryRepository
	suggestion SuggestionProvider
}

func NewDictionaryService(repo DictionaryRepository, suggestion SuggestionProvider) *DictionaryService {
	return &DictionaryService{repo: repo, suggestion: suggestion}
}

type DictionaryImportRequest struct {
	Model      DictionaryImportModel
	ImportedBy *uuid.UUID
}

type DictionaryImportResult struct {
	DictionaryVersionID uuid.UUID
	Validation          DictionaryValidationResult
	ProposalID          *uuid.UUID
	MigrationDraftIDs   []uuid.UUID
}

type DictionaryValidationResult struct {
	Errors   []DictionaryValidationIssue
	Warnings []DictionaryValidationIssue
}

type DictionaryValidationIssue struct {
	Code    string
	Message string
	Path    string
}

type ContextChangeProposalInput struct {
	DictionaryVersionID uuid.UUID
	ProposalType        string
	Title               string
	Summary             string
	Payload             map[string]any
	Status              string
}

type ContextMigrationDraftInput struct {
	DictionaryVersionID uuid.UUID
	Title               string
	Summary             string
	SQLUp               string
	SQLDown             string
	RiskLevel           string
	Metadata            map[string]any
}

func (s *DictionaryService) ValidateImport(model DictionaryImportModel) (DictionaryValidationResult, error) {
	result := DictionaryValidationResult{}
	if model.VersionKey == "" {
		result.Errors = append(result.Errors, DictionaryValidationIssue{Code: "missing_version_key", Message: "version_key is required", Path: "version_key"})
	}
	if model.ScopeLevel != ContextScopeSaaS && model.ScopeLevel != ContextScopeOrganization && model.ScopeLevel != ContextScopeModule {
		result.Errors = append(result.Errors, DictionaryValidationIssue{Code: "invalid_scope_level", Message: "scope_level must be saas, organization, or module", Path: "scope_level"})
	}
	entities := map[string]bool{}
	for _, entity := range model.Entities {
		if entity.EntityKey == "" {
			result.Errors = append(result.Errors, DictionaryValidationIssue{Code: "missing_entity_key", Message: "entity_key is required", Path: "entities"})
			continue
		}
		entities[entity.EntityKey] = true
	}
	for _, field := range model.Fields {
		if field.FieldKey == "" {
			result.Errors = append(result.Errors, DictionaryValidationIssue{Code: "missing_field_key", Message: "field_key is required", Path: "fields"})
		}
		if !entities[field.EntityKey] {
			result.Errors = append(result.Errors, DictionaryValidationIssue{Code: "unknown_entity", Message: "field references an unknown entity", Path: field.EntityKey + "." + field.FieldKey})
		}
		if field.IsFinanceField && field.BaseWeight > 0 && len(model.FinanceValidationProfiles) == 0 {
			result.Warnings = append(result.Warnings, DictionaryValidationIssue{Code: "finance_validation_missing", Message: "finance field has no validation profile", Path: field.EntityKey + "." + field.FieldKey})
		}
	}
	if len(result.Errors) > 0 {
		return result, fmt.Errorf("%w: dictionary import validation failed", ErrValidation)
	}
	return result, nil
}

func (s *DictionaryService) Import(ctx context.Context, input DictionaryImportRequest) (*DictionaryImportResult, error) {
	validation, err := s.ValidateImport(input.Model)
	if err != nil {
		return &DictionaryImportResult{Validation: validation}, err
	}
	if s.repo == nil {
		return nil, fmt.Errorf("%w: dictionary repository is not configured", ErrValidation)
	}
	versionID, err := s.repo.CreateDictionaryVersion(ctx, input.Model, input.ImportedBy)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{"version_key": input.Model.VersionKey, "module_key": input.Model.ModuleKey}
	if s.suggestion != nil {
		if suggestions, suggestErr := s.suggestion.SuggestDictionaryChanges(ctx, input.Model); suggestErr == nil {
			payload["ai_suggestions"] = suggestions
		}
	}
	proposalID, err := s.repo.CreateContextChangeProposal(ctx, ContextChangeProposalInput{
		DictionaryVersionID: versionID,
		ProposalType:        "dictionary_change",
		Title:               "Dictionary import " + input.Model.VersionKey,
		Summary:             "Review imported context dictionary before activation",
		Payload:             payload,
		Status:              ProposalPending,
	})
	if err != nil {
		return nil, err
	}
	draftIDs := []uuid.UUID{}
	for _, intent := range input.Model.MigrationIntents {
		draftID, err := s.repo.CreateContextMigrationDraft(ctx, ContextMigrationDraftInput{
			DictionaryVersionID: versionID,
			Title:               "Migration draft for " + intent.EntityKey + "." + intent.FieldKey,
			Summary:             intent.Reason,
			SQLUp:               "-- " + intent.IntentType + " " + intent.EntityKey + "." + intent.FieldKey,
			SQLDown:             "-- rollback " + intent.IntentType + " " + intent.EntityKey + "." + intent.FieldKey,
			RiskLevel:           "medium",
			Metadata:            map[string]any{"intent_type": intent.IntentType},
		})
		if err != nil {
			return nil, err
		}
		draftIDs = append(draftIDs, draftID)
	}
	return &DictionaryImportResult{DictionaryVersionID: versionID, Validation: validation, ProposalID: &proposalID, MigrationDraftIDs: draftIDs}, nil
}
```

- [ ] **Step 4: Add fake repository to test file**

Append to `backend/internal/domain/assistant/context_import_test.go`:

```go
type fakeDictionaryRepository struct {
	versionID       uuid.UUID
	proposals       []ContextChangeProposalInput
	migrationDrafts []ContextMigrationDraftInput
}

func (f *fakeDictionaryRepository) CreateDictionaryVersion(context.Context, DictionaryImportModel, *uuid.UUID) (uuid.UUID, error) {
	if f.versionID == uuid.Nil {
		f.versionID = uuid.New()
	}
	return f.versionID, nil
}

func (f *fakeDictionaryRepository) CreateContextChangeProposal(_ context.Context, input ContextChangeProposalInput) (uuid.UUID, error) {
	f.proposals = append(f.proposals, input)
	return uuid.New(), nil
}

func (f *fakeDictionaryRepository) CreateContextMigrationDraft(_ context.Context, input ContextMigrationDraftInput) (uuid.UUID, error) {
	f.migrationDrafts = append(f.migrationDrafts, input)
	return uuid.New(), nil
}
```

- [ ] **Step 5: Run dictionary service tests**

Run:

```powershell
cd backend
go test ./internal/domain/assistant -run "TestDictionaryServiceRejectsFieldWithoutEntity|TestDictionaryServiceCreatesMigrationDraftForIntent"
```

Expected: PASS.

- [ ] **Step 6: Commit Task 5**

Run:

```powershell
git add backend/internal/domain/assistant/context_dictionary.go backend/internal/domain/assistant/context_import_test.go
git commit -m "Add assistant dictionary service validation"
```

---

### Task 6: Add Rule Evaluator and Attention Budget

**Files:**
- Create: `backend/internal/domain/assistant/context_rule_evaluator.go`
- Test: `backend/internal/domain/assistant/context_rule_evaluator_test.go`

- [ ] **Step 1: Add failing evaluator tests**

Append to `backend/internal/domain/assistant/context_rule_evaluator_test.go`:

```go
func TestRuleEvaluatorMovesFinanceConflictToSignal(t *testing.T) {
	evaluator := NewContextRuleEvaluator(ContextRuleEvaluatorConfig{AttentionCoreRatio: 0.4})
	items := []ContextItem{
		{EntityKey: "cost_ledger_entry", FieldKey: "amount", Value: 100, Weight: 9, EstimatedTokens: 10, ValidationState: "finance_conflict"},
		{EntityKey: "project", FieldKey: "status", Value: "active", Weight: 8, EstimatedTokens: 10, ValidationState: "verified"},
	}

	pkg := evaluator.BuildPackage(ContextRuleEvaluationInput{Items: items, TokenBudget: 100})

	if len(pkg.AttentionCore) != 1 || pkg.AttentionCore[0].FieldKey != "status" {
		t.Fatalf("attention core = %#v, want only status", pkg.AttentionCore)
	}
	if len(pkg.RiskAndSignals) != 1 || pkg.RiskAndSignals[0].FieldKey != "amount" {
		t.Fatalf("signals = %#v, want amount", pkg.RiskAndSignals)
	}
}

func TestRuleEvaluatorEnforcesAttentionCoreBudget(t *testing.T) {
	evaluator := NewContextRuleEvaluator(ContextRuleEvaluatorConfig{AttentionCoreRatio: 0.3})
	items := []ContextItem{
		{EntityKey: "project", FieldKey: "a", Weight: 10, EstimatedTokens: 20, ValidationState: "verified"},
		{EntityKey: "project", FieldKey: "b", Weight: 9, EstimatedTokens: 20, ValidationState: "verified"},
		{EntityKey: "project", FieldKey: "c", Weight: 8, EstimatedTokens: 20, ValidationState: "verified"},
	}

	pkg := evaluator.BuildPackage(ContextRuleEvaluationInput{Items: items, TokenBudget: 100})

	if len(pkg.AttentionCore) != 1 || pkg.AttentionCore[0].FieldKey != "a" {
		t.Fatalf("attention core = %#v, want only highest-weight field a", pkg.AttentionCore)
	}
	if len(pkg.SupportingContext) != 2 {
		t.Fatalf("supporting context len = %d, want 2", len(pkg.SupportingContext))
	}
}
```

- [ ] **Step 2: Run evaluator tests and verify failure**

Run:

```powershell
cd backend
go test ./internal/domain/assistant -run "TestRuleEvaluatorMovesFinanceConflictToSignal|TestRuleEvaluatorEnforcesAttentionCoreBudget"
```

Expected: FAIL because evaluator types are undefined.

- [ ] **Step 3: Add evaluator implementation**

Create `backend/internal/domain/assistant/context_rule_evaluator.go`:

```go
package assistant

import (
	"sort"

	"github.com/google/uuid"
)

const (
	ValidationVerified        = "verified"
	ValidationFinanceConflict = "finance_conflict"
	ValidationPermissionDenied = "permission_denied"
)

type ContextRuleEvaluatorConfig struct {
	AttentionCoreRatio float64
}

type ContextRuleEvaluator struct {
	config ContextRuleEvaluatorConfig
}

type ContextRuleEvaluationInput struct {
	SessionID            uuid.UUID
	DictionaryVersionID *uuid.UUID
	Items                []ContextItem
	Omissions            []ContextOmission
	TokenBudget          int
	Validations          map[string]any
	Provenance           map[string]any
}

func NewContextRuleEvaluator(config ContextRuleEvaluatorConfig) *ContextRuleEvaluator {
	if config.AttentionCoreRatio <= 0 || config.AttentionCoreRatio > 1 {
		config.AttentionCoreRatio = 0.4
	}
	return &ContextRuleEvaluator{config: config}
}

func (e *ContextRuleEvaluator) BuildPackage(input ContextRuleEvaluationInput) ContextPackage {
	items := append([]ContextItem{}, input.Items...)
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Weight > items[j].Weight
	})
	coreBudget := int(float64(input.TokenBudget) * e.config.AttentionCoreRatio)
	if coreBudget <= 0 {
		coreBudget = 512
	}
	pkg := ContextPackage{
		ID:                  uuid.New(),
		SessionID:           input.SessionID,
		DictionaryVersionID: input.DictionaryVersionID,
		Omissions:           append([]ContextOmission{}, input.Omissions...),
		Weights:             map[string]float64{},
		Validations:         copyMap(input.Validations),
		Provenance:          copyMap(input.Provenance),
		TokenBudget:         input.TokenBudget,
	}
	coreTokens := 0
	for _, item := range items {
		pkg.Weights[item.EntityKey+"."+item.FieldKey] = item.Weight
		switch item.ValidationState {
		case ValidationPermissionDenied:
			pkg.Omissions = append(pkg.Omissions, ContextOmission{EntityKey: item.EntityKey, FieldKey: item.FieldKey, Reason: "permission_denied"})
		case ValidationFinanceConflict:
			pkg.RiskAndSignals = append(pkg.RiskAndSignals, item)
		default:
			if coreTokens+item.EstimatedTokens <= coreBudget {
				pkg.AttentionCore = append(pkg.AttentionCore, item)
				coreTokens += item.EstimatedTokens
			} else {
				pkg.SupportingContext = append(pkg.SupportingContext, item)
			}
		}
	}
	return pkg
}
```

- [ ] **Step 4: Run evaluator tests**

Run:

```powershell
cd backend
go test ./internal/domain/assistant -run "TestRuleEvaluator"
```

Expected: PASS.

- [ ] **Step 5: Commit Task 6**

Run:

```powershell
git add backend/internal/domain/assistant/context_rule_evaluator.go backend/internal/domain/assistant/context_rule_evaluator_test.go
git commit -m "Add assistant context rule evaluator"
```

---

### Task 7: Add Verified Context Engine with Compatibility Source

**Files:**
- Create: `backend/internal/domain/assistant/context_engine.go`
- Test: `backend/internal/domain/assistant/context_engine_test.go`

- [ ] **Step 1: Write failing context engine test**

Create `backend/internal/domain/assistant/context_engine_test.go`:

```go
package assistant

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestVerifiedContextEngineBuildsAttentionCoreFromWorkRecords(t *testing.T) {
	sessionID := uuid.New()
	resolver := &fakeContextResolver{
		result: WorkRecordContext{
			ModuleKey: "project",
			Records: []WorkRecord{
				{ID: uuid.New().String(), Type: "project", Title: "Launch", Status: "active"},
			},
		},
	}
	engine := NewVerifiedContextEngine(VerifiedContextEngineConfig{
		Resolver:  resolver,
		Evaluator: NewContextRuleEvaluator(ContextRuleEvaluatorConfig{AttentionCoreRatio: 0.5}),
	})

	pkg, err := engine.BuildContextPackage(context.Background(), ContextRequest{
		SessionID:   sessionID,
		ActorID:     uuid.New(),
		ActorType:   "internal_human",
		ModuleKey:   "project",
		TargetType:  "project",
		TokenBudget: 200,
	})
	if err != nil {
		t.Fatalf("BuildContextPackage returned error: %v", err)
	}
	if pkg.ID == uuid.Nil {
		t.Fatalf("context package id is nil")
	}
	if len(pkg.AttentionCore) == 0 {
		t.Fatalf("attention core is empty")
	}
	if pkg.AttentionCore[0].EntityKey != "project" {
		t.Fatalf("entity = %s, want project", pkg.AttentionCore[0].EntityKey)
	}
	if pkg.Provenance["source"] != "compatibility_resolver" {
		t.Fatalf("source = %v, want compatibility_resolver", pkg.Provenance["source"])
	}
}
```

- [ ] **Step 2: Run context engine test and verify failure**

Run:

```powershell
cd backend
go test ./internal/domain/assistant -run TestVerifiedContextEngineBuildsAttentionCoreFromWorkRecords
```

Expected: FAIL because `VerifiedContextEngine` is undefined.

- [ ] **Step 3: Add context engine implementation**

Create `backend/internal/domain/assistant/context_engine.go`:

```go
package assistant

import (
	"context"
	"fmt"
)

type ContextPackageRepository interface {
	CreateContextPackage(context.Context, ContextRequest, ContextPackage) (*ContextPackage, error)
}

type VerifiedContextEngineConfig struct {
	Resolver   ContextResolver
	Evaluator  *ContextRuleEvaluator
	Repository ContextPackageRepository
}

type VerifiedContextEngine struct {
	resolver   ContextResolver
	evaluator  *ContextRuleEvaluator
	repository ContextPackageRepository
}

func NewVerifiedContextEngine(config VerifiedContextEngineConfig) *VerifiedContextEngine {
	if config.Evaluator == nil {
		config.Evaluator = NewContextRuleEvaluator(ContextRuleEvaluatorConfig{AttentionCoreRatio: 0.4})
	}
	return &VerifiedContextEngine{resolver: config.Resolver, evaluator: config.Evaluator, repository: config.Repository}
}

func (e *VerifiedContextEngine) BuildContextPackage(ctx context.Context, request ContextRequest) (*ContextPackage, error) {
	if request.ActorID == uuidNil() || request.ActorType == "" {
		return nil, fmt.Errorf("%w: actor is required for context package", ErrValidation)
	}
	if e.resolver == nil {
		return nil, fmt.Errorf("%w: context resolver is not configured", ErrValidation)
	}
	session := &Session{
		ID:             request.SessionID,
		ModuleKey:      normalizedModule(request.ModuleKey),
		TargetType:     request.TargetType,
		TargetID:       request.TargetID,
		ActorID:        request.ActorID,
		ActorType:      request.ActorType,
		OrganizationID: request.OrganizationID,
		WorkflowID:     request.WorkflowID,
		TaskID:         request.TaskID,
	}
	workContext := e.resolver.Resolve(ctx, session)
	if workContext.Error != "" {
		return nil, fmt.Errorf("%w: %s", ErrValidation, workContext.Error)
	}
	items := make([]ContextItem, 0, len(workContext.Records))
	for _, record := range workContext.Records {
		items = append(items, ContextItem{
			EntityKey:       record.Type,
			FieldKey:        "record",
			RecordID:        record.ID,
			Value:           map[string]any{"title": record.Title, "status": record.Status, "created_at": record.CreatedAt, "data": record.Data},
			Weight:          5,
			EstimatedTokens: estimateContextRecordTokens(record),
			ValidationState: ValidationVerified,
			Source:          "compatibility_resolver",
		})
	}
	pkg := e.evaluator.BuildPackage(ContextRuleEvaluationInput{
		SessionID:   request.SessionID,
		Items:       items,
		TokenBudget: firstPositive(request.TokenBudget, 4096),
		Validations: map[string]any{"permission": "compatibility_checked", "workflow": "compatibility_checked", "finance": "not_applicable"},
		Provenance:  map[string]any{"source": "compatibility_resolver", "module_key": normalizedModule(request.ModuleKey)},
	})
	if e.repository != nil {
		return e.repository.CreateContextPackage(ctx, request, pkg)
	}
	return &pkg, nil
}

func estimateContextRecordTokens(record WorkRecord) int {
	total := len(record.ID) + len(record.Type) + len(record.Title) + len(record.Status) + len(record.CreatedAt)
	if len(record.Data) > 0 {
		total += len(marshalToolContent(record.Data))
	}
	tokens := total / 4
	if tokens < 16 {
		return 16
	}
	return tokens
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func uuidNil() uuid.UUID {
	return uuid.Nil
}
```

Add the missing import to `context_engine.go`:

```go
import "github.com/google/uuid"
```

- [ ] **Step 4: Run context engine test**

Run:

```powershell
cd backend
go test ./internal/domain/assistant -run TestVerifiedContextEngineBuildsAttentionCoreFromWorkRecords
```

Expected: PASS.

- [ ] **Step 5: Commit Task 7**

Run:

```powershell
git add backend/internal/domain/assistant/context_engine.go backend/internal/domain/assistant/context_engine_test.go
git commit -m "Add verified assistant context engine"
```

---

### Task 8: Add ToolRunner Gate

**Files:**
- Create: `backend/internal/domain/assistant/tool_runner.go`
- Test: `backend/internal/domain/assistant/tool_runner_test.go`

- [ ] **Step 1: Write failing ToolRunner tests**

Create `backend/internal/domain/assistant/tool_runner_test.go`:

```go
package assistant

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/toolruntime"
)

func TestToolRunnerBlocksToolOutsideAllowlist(t *testing.T) {
	runner := NewToolRunner(&fakeToolExecutor{}, ToolRunnerConfig{AllowedTools: []string{"project.match_members"}})

	_, err := runner.ExecuteTool(context.Background(), ToolRunRequest{
		Session: &Session{ID: uuid.New(), ActorID: uuid.New(), ActorType: "internal_human", ModuleKey: "project"},
		Call:    aigatewayToolCall("project.create_cost_entry"),
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("error = %v, want ErrForbidden", err)
	}
}

func TestToolRunnerPassesAllowedTool(t *testing.T) {
	executor := &fakeToolExecutor{output: &toolruntime.ExecuteToolOutput{Execution: &toolruntime.ToolExecution{ID: uuid.New(), Status: toolruntime.ExecutionCompleted}}}
	runner := NewToolRunner(executor, ToolRunnerConfig{AllowedTools: []string{"project.match_members"}})

	output, err := runner.ExecuteTool(context.Background(), ToolRunRequest{
		Session: &Session{ID: uuid.New(), ActorID: uuid.New(), ActorType: "internal_human", ModuleKey: "project"},
		Call:    aigatewayToolCall("project.match_members"),
	})
	if err != nil {
		t.Fatalf("ExecuteTool returned error: %v", err)
	}
	if output.Execution.Status != toolruntime.ExecutionCompleted {
		t.Fatalf("status = %s, want completed", output.Execution.Status)
	}
}
```

Use this helper in the test:

```go
func aigatewayToolCall(name string) ToolCallRequest {
	return ToolCallRequest{ID: "call-1", Name: name, Arguments: map[string]any{"project_id": uuid.New().String()}}
}
```

- [ ] **Step 2: Run ToolRunner tests and verify failure**

Run:

```powershell
cd backend
go test ./internal/domain/assistant -run "TestToolRunner"
```

Expected: FAIL because `ToolRunner` is undefined.

- [ ] **Step 3: Add ToolRunner implementation**

Create `backend/internal/domain/assistant/tool_runner.go`:

```go
package assistant

import (
	"context"
	"fmt"

	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/toolruntime"
)

type ToolCallRequest struct {
	ID        string
	Name      string
	Arguments map[string]any
}

type ToolRunRequest struct {
	Session        *Session
	ContextPackage *ContextPackage
	Call           ToolCallRequest
	InvocationID   *uuid.UUID
}

type ToolRunnerConfig struct {
	AllowedTools []string
}

type ToolRunner struct {
	executor ToolExecutor
	allowed  map[string]bool
}

func NewToolRunner(executor ToolExecutor, config ToolRunnerConfig) *ToolRunner {
	allowed := map[string]bool{}
	for _, name := range config.AllowedTools {
		if name != "" {
			allowed[name] = true
		}
	}
	return &ToolRunner{executor: executor, allowed: allowed}
}

func (r *ToolRunner) ExecuteTool(ctx context.Context, request ToolRunRequest) (*toolruntime.ExecuteToolOutput, error) {
	if r == nil || r.executor == nil {
		return nil, fmt.Errorf("%w: tool runner executor is not configured", ErrValidation)
	}
	if request.Session == nil {
		return nil, fmt.Errorf("%w: tool runner session is required", ErrValidation)
	}
	if len(r.allowed) > 0 && !r.allowed[request.Call.Name] {
		return nil, fmt.Errorf("%w: tool %s is not allowed in this assistant context", ErrForbidden, request.Call.Name)
	}
	args := request.Call.Arguments
	if args == nil {
		args = map[string]any{}
	}
	return r.executor.ExecuteTool(ctx, toolruntime.ExecuteToolInput{
		ToolName:       request.Call.Name,
		InvocationID:   request.InvocationID,
		ActorID:        request.Session.ActorID,
		ActorType:      request.Session.ActorType,
		OrganizationID: request.Session.OrganizationID,
		DepartmentID:   request.Session.DepartmentID,
		ProjectID:      request.Session.ProjectID,
		WorkflowID:     request.Session.WorkflowID,
		TaskID:         request.Session.TaskID,
		IdempotencyKey: fmt.Sprintf("assistant:%s:%s", request.Session.ID, request.Call.ID),
		Arguments:      args,
	})
}
```

Add the missing import to `tool_runner.go`:

```go
import "github.com/google/uuid"
```

- [ ] **Step 4: Extend fakeToolExecutor in tests**

In `tool_runner_test.go`, add:

```go
type fakeToolExecutor struct {
	output *toolruntime.ExecuteToolOutput
}

func (f *fakeToolExecutor) ExecuteTool(context.Context, toolruntime.ExecuteToolInput) (*toolruntime.ExecuteToolOutput, error) {
	if f.output == nil {
		return &toolruntime.ExecuteToolOutput{Execution: &toolruntime.ToolExecution{ID: uuid.New(), Status: toolruntime.ExecutionCompleted}}, nil
	}
	return f.output, nil
}

func (f *fakeToolExecutor) ListTools(context.Context, int) ([]toolruntime.ToolDefinition, error) {
	return []toolruntime.ToolDefinition{}, nil
}

func (f *fakeToolExecutor) GetApproval(context.Context, uuid.UUID) (*toolruntime.ToolApproval, error) {
	return nil, ErrNotFound
}

func (f *fakeToolExecutor) GetExecution(context.Context, uuid.UUID) (*toolruntime.ToolExecution, error) {
	return nil, ErrNotFound
}
```

- [ ] **Step 5: Run ToolRunner tests**

Run:

```powershell
cd backend
go test ./internal/domain/assistant -run "TestToolRunner"
```

Expected: PASS.

- [ ] **Step 6: Commit Task 8**

Run:

```powershell
git add backend/internal/domain/assistant/tool_runner.go backend/internal/domain/assistant/tool_runner_test.go
git commit -m "Add assistant tool runner gate"
```

---

### Task 9: Add EventSink

**Files:**
- Create: `backend/internal/domain/assistant/event_sink.go`
- Test: `backend/internal/domain/assistant/runtime_test.go`

- [ ] **Step 1: Add failing EventSink test**

Append to `backend/internal/domain/assistant/runtime_test.go`:

```go
func TestMemoryEventSinkAddsStep(t *testing.T) {
	repo := &fakeRepository{}
	sink := NewMemoryEventSink(repo)
	session := &Session{ID: uuid.New(), ModuleKey: "project", WorkingMemory: map[string]any{}}

	event, err := sink.Emit(context.Background(), session, AssistantRuntimeEvent{
		Type: RuntimeEventContextBuilt,
		Status: StatusCompleted,
		Summary: "context built",
		Data: map[string]any{"context_package_id": uuid.New().String()},
	})
	if err != nil {
		t.Fatalf("Emit returned error: %v", err)
	}
	if event.Type != RuntimeEventContextBuilt {
		t.Fatalf("event type = %s, want context_built", event.Type)
	}
	if repo.lastStep.StepType != StepContext {
		t.Fatalf("step type = %s, want context", repo.lastStep.StepType)
	}
}
```

Add `context` to the imports in `runtime_test.go`.

- [ ] **Step 2: Run EventSink test and verify failure**

Run:

```powershell
cd backend
go test ./internal/domain/assistant -run TestMemoryEventSinkAddsStep
```

Expected: FAIL because `EventSink`, `AssistantRuntimeEvent`, and `StepContext` are undefined.

- [ ] **Step 3: Add context step constant**

Modify `backend/internal/domain/assistant/model.go` and add:

```go
StepContext = "context"
```

Update migration compatibility later by storing `StepContext` as `memory` if the DB check constraint is not expanded in Task 2. Prefer expanding the DB check constraint in a follow-up migration when changing persisted step types.

- [ ] **Step 4: Add EventSink implementation**

Create `backend/internal/domain/assistant/event_sink.go`:

```go
package assistant

import (
	"context"
)

type AssistantRuntimeEvent struct {
	Type    string
	Status  string
	Summary string
	Data    map[string]any
	Turn    int
}

type EventSink interface {
	Emit(context.Context, *Session, AssistantRuntimeEvent) (RunEvent, error)
}

type MemoryEventSink struct {
	repo Repository
}

func NewMemoryEventSink(repo Repository) *MemoryEventSink {
	return &MemoryEventSink{repo: repo}
}

func (s *MemoryEventSink) Emit(ctx context.Context, session *Session, event AssistantRuntimeEvent) (RunEvent, error) {
	stepType := runtimeEventStepType(event.Type)
	step, err := s.repo.AddStep(ctx, session, AddStepInput{
		StepType: stepType,
		Status:   firstNonEmpty(event.Status, StatusCompleted),
		Summary:  event.Summary,
		Data:     event.Data,
		Turn:     event.Turn,
	})
	if err != nil {
		return RunEvent{}, err
	}
	return RunEvent{Type: event.Type, Step: step, Data: event.Data}, nil
}

func runtimeEventStepType(eventType string) string {
	switch eventType {
	case RuntimeEventContextBuilt:
		return StepContext
	case RuntimeEventToolRequested:
		return StepToolCall
	case RuntimeEventToolCompleted:
		return StepToolResult
	case RuntimeEventApprovalRequired:
		return StepApproval
	case RuntimeEventRunFailed:
		return StepError
	default:
		return StepLLM
	}
}
```

- [ ] **Step 5: Update step migration constraint**

Modify `migrations/035_assistant_context_engine.sql` to add a safe constraint migration:

```sql
ALTER TABLE assistant_steps DROP CONSTRAINT IF EXISTS assistant_steps_step_type_check;
ALTER TABLE assistant_steps
    ADD CONSTRAINT assistant_steps_step_type_check
    CHECK (step_type IN ('llm', 'tool_call', 'tool_result', 'memory', 'approval', 'error', 'context'));
```

- [ ] **Step 6: Update fake repository to capture last step**

In `backend/internal/domain/assistant/service_test.go`, add `lastStep AddStepInput` to `fakeRepository` and update `AddStep`:

```go
func (f *fakeRepository) AddStep(_ context.Context, _ *Session, input AddStepInput) (*Step, error) {
	f.lastStep = input
	return &Step{ID: uuid.New(), StepType: input.StepType, Status: input.Status, Data: input.Data}, nil
}
```

- [ ] **Step 7: Run EventSink test**

Run:

```powershell
cd backend
go test ./internal/domain/assistant -run TestMemoryEventSinkAddsStep
```

Expected: PASS.

- [ ] **Step 8: Commit Task 9**

Run:

```powershell
git add backend/internal/domain/assistant/event_sink.go backend/internal/domain/assistant/model.go backend/internal/domain/assistant/runtime_test.go backend/internal/domain/assistant/service_test.go migrations/035_assistant_context_engine.sql
git commit -m "Add assistant runtime event sink"
```

---

### Task 10: Add AssistantRuntime and Delegate Service.Run

**Files:**
- Create: `backend/internal/domain/assistant/runtime.go`
- Modify: `backend/internal/domain/assistant/service.go`
- Modify: `backend/internal/domain/assistant/service_test.go`
- Test: `backend/internal/domain/assistant/runtime_test.go`

- [ ] **Step 1: Write failing runtime delegation test**

Append to `backend/internal/domain/assistant/runtime_test.go`:

```go
func TestServiceRunDelegatesToRuntime(t *testing.T) {
	sessionID := uuid.New()
	actorID := uuid.New()
	runtime := &fakeAssistantRuntime{events: make(chan RunEvent, 1)}
	runtime.events <- RunEvent{Type: RuntimeEventRunCompleted, Done: true}
	close(runtime.events)
	svc := NewService(&fakeRepository{}, nil, nil, WithAssistantRuntime(runtime))

	events, err := svc.Run(context.Background(), sessionID, actorID, "internal_human", RunInput{Message: "summarize"})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	received := <-events
	if received.Type != RuntimeEventRunCompleted {
		t.Fatalf("event = %s, want run_completed", received.Type)
	}
	if runtime.request.SessionID != sessionID || runtime.request.ActorID != actorID {
		t.Fatalf("runtime request = %#v", runtime.request)
	}
}
```

Add helper:

```go
type fakeAssistantRuntime struct {
	request AssistantRunRequest
	events  chan RunEvent
}

func (f *fakeAssistantRuntime) Run(_ context.Context, request AssistantRunRequest) (<-chan RunEvent, error) {
	f.request = request
	return f.events, nil
}

func (f *fakeAssistantRuntime) Resume(_ context.Context, request AssistantResumeRequest) (<-chan RunEvent, error) {
	return f.events, nil
}
```

- [ ] **Step 2: Run delegation test and verify failure**

Run:

```powershell
cd backend
go test ./internal/domain/assistant -run TestServiceRunDelegatesToRuntime
```

Expected: FAIL because `WithAssistantRuntime` and runtime request types are undefined.

- [ ] **Step 3: Add runtime interface and request types**

Create `backend/internal/domain/assistant/runtime.go`:

```go
package assistant

import (
	"context"

	"github.com/google/uuid"
)

type AssistantRuntimeRunner interface {
	Run(context.Context, AssistantRunRequest) (<-chan RunEvent, error)
	Resume(context.Context, AssistantResumeRequest) (<-chan RunEvent, error)
}

type AssistantRunRequest struct {
	SessionID uuid.UUID
	ActorID   uuid.UUID
	ActorType string
	Input     RunInput
}

type AssistantResumeRequest struct {
	SessionID uuid.UUID
	ActorID   uuid.UUID
	ActorType string
	Input     ResumeInput
}

type AssistantRuntime struct {
	service *Service
	contextEngine *VerifiedContextEngine
	toolRunner *ToolRunner
	eventSink EventSink
}

func NewAssistantRuntime(service *Service, contextEngine *VerifiedContextEngine, toolRunner *ToolRunner, eventSink EventSink) *AssistantRuntime {
	return &AssistantRuntime{service: service, contextEngine: contextEngine, toolRunner: toolRunner, eventSink: eventSink}
}

func (r *AssistantRuntime) Run(ctx context.Context, request AssistantRunRequest) (<-chan RunEvent, error) {
	return r.service.runLegacy(ctx, request.SessionID, request.ActorID, request.ActorType, request.Input)
}

func (r *AssistantRuntime) Resume(ctx context.Context, request AssistantResumeRequest) (<-chan RunEvent, error) {
	return r.service.resumeLegacy(ctx, request.SessionID, request.ActorID, request.ActorType, request.Input)
}
```

This first runtime is an adapter around the current loop. Later tasks move context building and tool gating into it without breaking API compatibility.

- [ ] **Step 4: Modify service options for runtime injection**

Modify `backend/internal/domain/assistant/service.go`:

```go
type Service struct {
	repo               Repository
	ai                 AIInvoker
	tools              ToolExecutor
	contextResolver    ContextResolver
	proposalApplicator ProposalApplicator
	runtime            AssistantRuntimeRunner
	maxTurns           int
	maxHistory         int
}

func WithAssistantRuntime(runtime AssistantRuntimeRunner) ServiceOption {
	return func(s *Service) {
		s.runtime = runtime
	}
}
```

Update `Run`:

```go
func (s *Service) Run(ctx context.Context, sessionID uuid.UUID, actorID uuid.UUID, actorType string, input RunInput) (<-chan RunEvent, error) {
	if s.runtime != nil {
		return s.runtime.Run(ctx, AssistantRunRequest{SessionID: sessionID, ActorID: actorID, ActorType: actorType, Input: input})
	}
	return s.runLegacy(ctx, sessionID, actorID, actorType, input)
}
```

Rename the existing `Run` body to:

```go
func (s *Service) runLegacy(ctx context.Context, sessionID uuid.UUID, actorID uuid.UUID, actorType string, input RunInput) (<-chan RunEvent, error) {
	// move the current Run body here unchanged
}
```

Apply the same pattern to `Resume`:

```go
func (s *Service) Resume(ctx context.Context, sessionID uuid.UUID, actorID uuid.UUID, actorType string, input ResumeInput) (<-chan RunEvent, error) {
	if s.runtime != nil {
		return s.runtime.Resume(ctx, AssistantResumeRequest{SessionID: sessionID, ActorID: actorID, ActorType: actorType, Input: input})
	}
	return s.resumeLegacy(ctx, sessionID, actorID, actorType, input)
}
```

Rename the current `Resume` body to `resumeLegacy`.

- [ ] **Step 5: Run delegation test**

Run:

```powershell
cd backend
go test ./internal/domain/assistant -run TestServiceRunDelegatesToRuntime
```

Expected: PASS.

- [ ] **Step 6: Run existing assistant tests**

Run:

```powershell
cd backend
go test ./internal/domain/assistant
```

Expected: PASS.

- [ ] **Step 7: Commit Task 10**

Run:

```powershell
git add backend/internal/domain/assistant/runtime.go backend/internal/domain/assistant/service.go backend/internal/domain/assistant/runtime_test.go
git commit -m "Add assistant runtime delegation"
```

---

### Task 11: Move Context Build Into Runtime Adapter

**Files:**
- Modify: `backend/internal/domain/assistant/runtime.go`
- Modify: `backend/internal/domain/assistant/service.go`
- Test: `backend/internal/domain/assistant/runtime_test.go`

- [ ] **Step 1: Add failing context build runtime test**

Append to `backend/internal/domain/assistant/runtime_test.go`:

```go
func TestAssistantRuntimeBuildsContextBeforeLegacyRun(t *testing.T) {
	repo := &fakeRepository{}
	service := NewService(repo, &fakeAIInvoker{}, &fakeToolExecutor{})
	engine := &fakeContextEngine{pkg: &ContextPackage{ID: uuid.New(), AttentionCore: []ContextItem{{EntityKey: "project", FieldKey: "status", Value: "active"}}}}
	runtime := NewAssistantRuntime(service, engine, nil, NewMemoryEventSink(repo))
	sessionID := uuid.New()
	actorID := uuid.New()

	events, err := runtime.Run(context.Background(), AssistantRunRequest{
		SessionID: sessionID,
		ActorID:   actorID,
		ActorType: "internal_human",
		Input:     RunInput{Message: "status"},
	})
	if err == nil {
		for range events {
		}
	}
	if engine.request.SessionID != sessionID || engine.request.ActorID != actorID {
		t.Fatalf("context request = %#v, want session and actor", engine.request)
	}
}
```

Add fake context engine:

```go
type fakeContextEngine struct {
	request ContextRequest
	pkg     *ContextPackage
}

func (f *fakeContextEngine) BuildContextPackage(_ context.Context, request ContextRequest) (*ContextPackage, error) {
	f.request = request
	if f.pkg == nil {
		f.pkg = &ContextPackage{ID: uuid.New()}
	}
	return f.pkg, nil
}
```

- [ ] **Step 2: Run test and verify failure**

Run:

```powershell
cd backend
go test ./internal/domain/assistant -run TestAssistantRuntimeBuildsContextBeforeLegacyRun
```

Expected: FAIL because runtime expects concrete `*VerifiedContextEngine`, not the test fake interface.

- [ ] **Step 3: Introduce context builder interface**

Modify `backend/internal/domain/assistant/runtime.go`:

```go
type ContextPackageBuilder interface {
	BuildContextPackage(context.Context, ContextRequest) (*ContextPackage, error)
}

type AssistantRuntime struct {
	service       *Service
	contextEngine ContextPackageBuilder
	toolRunner    *ToolRunner
	eventSink      EventSink
}

func NewAssistantRuntime(service *Service, contextEngine ContextPackageBuilder, toolRunner *ToolRunner, eventSink EventSink) *AssistantRuntime {
	return &AssistantRuntime{service: service, contextEngine: contextEngine, toolRunner: toolRunner, eventSink: eventSink}
}
```

Update `Run` to build and emit context before legacy run:

```go
func (r *AssistantRuntime) Run(ctx context.Context, request AssistantRunRequest) (<-chan RunEvent, error) {
	session, err := r.service.repo.GetSession(ctx, request.SessionID, request.ActorID, request.ActorType)
	if err != nil {
		return nil, err
	}
	if r.contextEngine != nil {
		pkg, err := r.contextEngine.BuildContextPackage(ctx, ContextRequest{
			SessionID:      request.SessionID,
			ActorID:        request.ActorID,
			ActorType:      request.ActorType,
			OrganizationID: session.OrganizationID,
			ModuleKey:      session.ModuleKey,
			WorkflowID:     session.WorkflowID,
			TaskID:         session.TaskID,
			TargetType:     session.TargetType,
			TargetID:       session.TargetID,
			Intent:         request.Input.Message,
			Mode:           session.Mode,
			TokenBudget:    4096,
		})
		if err != nil {
			return nil, err
		}
		if r.eventSink != nil && pkg != nil {
			_, _ = r.eventSink.Emit(ctx, session, AssistantRuntimeEvent{
				Type:    RuntimeEventContextBuilt,
				Status:  StatusCompleted,
				Summary: "Verified context package built",
				Data: map[string]any{
					"context_package_id": pkg.ID.String(),
					"attention_core_count": len(pkg.AttentionCore),
				},
			})
		}
	}
	return r.service.runLegacy(ctx, request.SessionID, request.ActorID, request.ActorType, request.Input)
}
```

- [ ] **Step 4: Add fake AI invoker if test needs it**

If `runtime_test.go` does not already have `fakeAIInvoker`, add:

```go
type fakeAIInvoker struct{}

func (f *fakeAIInvoker) Invoke(context.Context, aigateway.InvokeInput) (*aigateway.InvokeOutput, error) {
	return &aigateway.InvokeOutput{
		InvocationID: uuid.New(),
		Content:      "ok",
		ProviderType: "openai",
		Model:        "gpt-4o-mini",
		CompletedAt:  time.Now(),
	}, nil
}
```

Add imports for `time` and `github.com/selfevo-AI/meta-org-saas/backend/internal/domain/aigateway` when needed.

- [ ] **Step 5: Run runtime context test**

Run:

```powershell
cd backend
go test ./internal/domain/assistant -run TestAssistantRuntimeBuildsContextBeforeLegacyRun
```

Expected: PASS or a validation error from legacy run after context request is captured. The assertion must pass before the legacy error is evaluated.

- [ ] **Step 6: Commit Task 11**

Run:

```powershell
git add backend/internal/domain/assistant/runtime.go backend/internal/domain/assistant/runtime_test.go
git commit -m "Build verified context in assistant runtime"
```

---

### Task 12: Add Context Repository Persistence

**Files:**
- Create: `backend/internal/domain/assistant/context_repository.go`
- Test: compile-focused with `backend/internal/domain/assistant/context_engine_test.go`

- [ ] **Step 1: Add repository implementation**

Create `backend/internal/domain/assistant/context_repository.go`:

```go
package assistant

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresContextRepository struct {
	db *pgxpool.Pool
}

func NewContextRepository(db *pgxpool.Pool) *PostgresContextRepository {
	return &PostgresContextRepository{db: db}
}

func (r *PostgresContextRepository) CreateDictionaryVersion(ctx context.Context, model DictionaryImportModel, importedBy *uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.db.QueryRow(ctx, `
		INSERT INTO context_dictionary_versions (scope_level, organization_id, module_key, version_key, source_type, source_name, imported_by, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`, model.ScopeLevel, model.OrganizationID, model.ModuleKey, model.VersionKey, model.SourceType, model.SourceName, importedBy, mustJSON(map[string]any{"field_count": len(model.Fields)})).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create context dictionary version: %w", err)
	}
	return id, nil
}

func (r *PostgresContextRepository) CreateContextChangeProposal(ctx context.Context, input ContextChangeProposalInput) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.db.QueryRow(ctx, `
		INSERT INTO context_change_proposals (dictionary_version_id, proposal_type, title, summary, payload, status)
		VALUES ($1, $2, $3, $4, $5, COALESCE(NULLIF($6, ''), 'pending'))
		RETURNING id
	`, input.DictionaryVersionID, input.ProposalType, input.Title, input.Summary, mustJSON(input.Payload), input.Status).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create context change proposal: %w", err)
	}
	return id, nil
}

func (r *PostgresContextRepository) CreateContextMigrationDraft(ctx context.Context, input ContextMigrationDraftInput) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.db.QueryRow(ctx, `
		INSERT INTO context_migration_drafts (dictionary_version_id, title, summary, sql_up, sql_down, risk_level, metadata)
		VALUES ($1, $2, $3, $4, $5, COALESCE(NULLIF($6, ''), 'medium'), $7)
		RETURNING id
	`, input.DictionaryVersionID, input.Title, input.Summary, input.SQLUp, input.SQLDown, input.RiskLevel, mustJSON(input.Metadata)).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create context migration draft: %w", err)
	}
	return id, nil
}

func (r *PostgresContextRepository) CreateContextPackage(ctx context.Context, request ContextRequest, pkg ContextPackage) (*ContextPackage, error) {
	var id uuid.UUID
	err := r.db.QueryRow(ctx, `
		INSERT INTO context_packages (
			session_id, dictionary_version_id, actor_id, actor_type, organization_id, module_key,
			target_type, target_id, workflow_id, task_id, attention_core, supporting_context,
			risk_and_signals, omissions, weights, validations, provenance, token_budget
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		RETURNING id
	`, request.SessionID, pkg.DictionaryVersionID, request.ActorID, request.ActorType, request.OrganizationID, request.ModuleKey,
		request.TargetType, request.TargetID, request.WorkflowID, request.TaskID, mustJSONValue(pkg.AttentionCore), mustJSONValue(pkg.SupportingContext),
		mustJSONValue(pkg.RiskAndSignals), mustJSONValue(pkg.Omissions), mustJSONValue(pkg.Weights), mustJSONValue(pkg.Validations),
		mustJSONValue(pkg.Provenance), pkg.TokenBudget).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("create context package: %w", err)
	}
	pkg.ID = id
	return &pkg, nil
}

func mustJSONBytes(input any) []byte {
	data, err := json.Marshal(input)
	if err != nil {
		return []byte("{}")
	}
	return data
}
```

Replace `mustJSONValue(...)` calls in this file with `mustJSONBytes(...)` where the input is not `map[string]any`.

- [ ] **Step 2: Compile assistant package**

Run:

```powershell
cd backend
go test ./internal/domain/assistant
```

Expected: PASS.

- [ ] **Step 3: Commit Task 12**

Run:

```powershell
git add backend/internal/domain/assistant/context_repository.go
git commit -m "Persist assistant context metadata"
```

---

### Task 13: Wire Runtime Framework in Server

**Files:**
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: Modify server wiring**

In `backend/cmd/server/main.go`, replace assistant service construction with:

```go
	assistantRepo := assistant.NewRepository(db)
	contextRepo := assistant.NewContextRepository(db)
	contextResolver := assistant.NewDBContextResolver(db)
	contextEngine := assistant.NewVerifiedContextEngine(assistant.VerifiedContextEngineConfig{
		Resolver:   contextResolver,
		Evaluator:  assistant.NewContextRuleEvaluator(assistant.ContextRuleEvaluatorConfig{AttentionCoreRatio: 0.4}),
		Repository: contextRepo,
	})
	assistantSvc := assistant.NewService(
		assistantRepo,
		aiSvc,
		toolSvc,
		assistant.WithContextResolver(contextResolver),
		assistant.WithProposalApplicator(assistant.NewDBProposalApplicator(db)),
	)
	toolRunner := assistant.NewToolRunner(toolSvc, assistant.ToolRunnerConfig{})
	eventSink := assistant.NewMemoryEventSink(assistantRepo)
	assistantRuntime := assistant.NewAssistantRuntime(assistantSvc, contextEngine, toolRunner, eventSink)
	assistantSvc.SetRuntime(assistantRuntime)
	assistantHandler := assistant.NewHandler(assistantSvc)
```

Add this method to `backend/internal/domain/assistant/service.go` if not already added:

```go
func (s *Service) SetRuntime(runtime AssistantRuntimeRunner) {
	s.runtime = runtime
}
```

- [ ] **Step 2: Compile server**

Run:

```powershell
cd backend
go test ./...
go build ./cmd/server
```

Expected: PASS.

- [ ] **Step 3: Commit Task 13**

Run:

```powershell
git add backend/cmd/server/main.go backend/internal/domain/assistant/service.go
git commit -m "Wire assistant runtime framework"
```

---

### Task 14: Add Dictionary Import and Preview Endpoints

**Files:**
- Modify: `backend/internal/domain/assistant/handler.go`
- Modify: `backend/internal/domain/assistant/service.go`
- Test: compile with package tests

- [ ] **Step 1: Add service fields and methods**

Modify `Service` in `backend/internal/domain/assistant/service.go`:

```go
	dictionary *DictionaryService
	contextEngine ContextPackageBuilder
```

Add options:

```go
func WithDictionaryService(dictionary *DictionaryService) ServiceOption {
	return func(s *Service) {
		s.dictionary = dictionary
	}
}

func WithVerifiedContextEngine(engine ContextPackageBuilder) ServiceOption {
	return func(s *Service) {
		s.contextEngine = engine
	}
}
```

Add methods:

```go
type ImportDictionaryInput struct {
	SourceType string `json:"source_type"`
	SourceName string `json:"source_name"`
	Content    string `json:"content"`
	ScopeLevel string `json:"scope_level"`
	ModuleKey  string `json:"module_key"`
	VersionKey  string `json:"version_key"`
}

func (s *Service) ImportDictionary(ctx context.Context, actorID uuid.UUID, input ImportDictionaryInput) (*DictionaryImportResult, error) {
	if s.dictionary == nil {
		return nil, fmt.Errorf("%w: dictionary service is not configured", ErrValidation)
	}
	model, err := NormalizeDictionaryImport(DictionaryImportSource{
		SourceType: input.SourceType,
		SourceName: input.SourceName,
		Content:    []byte(input.Content),
		ScopeLevel: input.ScopeLevel,
		ModuleKey:  input.ModuleKey,
		VersionKey:  input.VersionKey,
	})
	if err != nil {
		return nil, err
	}
	return s.dictionary.Import(ctx, DictionaryImportRequest{Model: model, ImportedBy: &actorID})
}

func (s *Service) PreviewContext(ctx context.Context, actorID uuid.UUID, actorType string, request ContextRequest) (*ContextPackage, error) {
	if s.contextEngine == nil {
		return nil, fmt.Errorf("%w: context engine is not configured", ErrValidation)
	}
	request.ActorID = actorID
	request.ActorType = actorType
	return s.contextEngine.BuildContextPackage(ctx, request)
}
```

- [ ] **Step 2: Add handler routes**

Modify `RegisterRoutes` in `backend/internal/domain/assistant/handler.go`:

```go
	r.Post("/assistant/context-dictionaries/imports", h.importContextDictionary)
	r.Post("/assistant/context-preview", h.previewContext)
```

Add handlers:

```go
func (h *Handler) importContextDictionary(w http.ResponseWriter, r *http.Request) {
	actorID, _, ok := authenticatedActor(w, r)
	if !ok {
		return
	}
	var input ImportDictionaryInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.ImportDictionary(r.Context(), actorID, input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) previewContext(w http.ResponseWriter, r *http.Request) {
	actorID, actorType, ok := authenticatedActor(w, r)
	if !ok {
		return
	}
	var input ContextRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.PreviewContext(r.Context(), actorID, actorType, input)
	writeResult(w, http.StatusOK, result, err)
}
```

- [ ] **Step 3: Wire services in main**

In `backend/cmd/server/main.go`, create dictionary service:

```go
	dictionarySvc := assistant.NewDictionaryService(contextRepo, nil)
```

Pass options to `assistant.NewService`:

```go
		assistant.WithDictionaryService(dictionarySvc),
		assistant.WithVerifiedContextEngine(contextEngine),
```

- [ ] **Step 4: Compile assistant package and server**

Run:

```powershell
cd backend
go test ./internal/domain/assistant
go build ./cmd/server
```

Expected: PASS.

- [ ] **Step 5: Commit Task 14**

Run:

```powershell
git add backend/internal/domain/assistant/handler.go backend/internal/domain/assistant/service.go backend/cmd/server/main.go
git commit -m "Add assistant context dictionary endpoints"
```

---

### Task 15: Add Seed Rule Coverage for Project, Finance, and Governance

**Files:**
- Modify: `migrations/035_assistant_context_engine.sql`

- [ ] **Step 1: Append seed dictionary data**

Append to `migrations/035_assistant_context_engine.sql`:

```sql
WITH seed_version AS (
    INSERT INTO context_dictionary_versions (scope_level, module_key, version_key, source_type, source_name, status, metadata)
    VALUES ('saas', 'assistant_seed', 'assistant-context-seed-v1', 'json', 'migration_035_seed', 'active',
        '{"seed":true,"domains":["project","finance","governance"]}'::jsonb)
    ON CONFLICT (scope_level, organization_id, module_key, version_key) DO UPDATE
    SET status = 'active', updated_at = NOW()
    RETURNING id
),
domains AS (
    INSERT INTO context_business_domains (dictionary_version_id, module_key, name, scope_level, status)
    SELECT id, 'project', 'Project', 'saas', 'active' FROM seed_version
    UNION ALL SELECT id, 'finance', 'Finance', 'saas', 'active' FROM seed_version
    UNION ALL SELECT id, 'governance', 'Governance', 'saas', 'active' FROM seed_version
    ON CONFLICT (dictionary_version_id, module_key) DO NOTHING
    RETURNING id, module_key
),
entities AS (
    INSERT INTO context_entities (dictionary_version_id, domain_id, entity_key, display_name, status)
    SELECT seed_version.id, domains.id, 'project', 'Project', 'active' FROM seed_version, domains WHERE domains.module_key = 'project'
    UNION ALL SELECT seed_version.id, domains.id, 'requirement', 'Requirement', 'active' FROM seed_version, domains WHERE domains.module_key = 'project'
    UNION ALL SELECT seed_version.id, domains.id, 'cost_ledger_entry', 'Cost Ledger Entry', 'active' FROM seed_version, domains WHERE domains.module_key = 'finance'
    UNION ALL SELECT seed_version.id, domains.id, 'access_decision', 'Access Decision', 'active' FROM seed_version, domains WHERE domains.module_key = 'governance'
    ON CONFLICT (dictionary_version_id, entity_key) DO NOTHING
    RETURNING id, entity_key
)
INSERT INTO context_rules (dictionary_version_id, module_key, entity_key, field_key, rule_type, rule, status)
SELECT id, 'project', 'project', 'status', 'attention', '{"base_weight":8,"attention_core":true}'::jsonb, 'active' FROM seed_version
UNION ALL SELECT id, 'project', 'requirement', 'risk_level', 'workflow', '{"stage_multiplier":{"analysis":1.5,"execution":0.8}}'::jsonb, 'active' FROM seed_version
UNION ALL SELECT id, 'finance', 'cost_ledger_entry', 'amount', 'finance', '{"requires_validation":true,"unverified_as_signal":true}'::jsonb, 'active' FROM seed_version
UNION ALL SELECT id, 'governance', 'access_decision', 'decision', 'permission', '{"sensitivity":"restricted","explicit_rule_required":true}'::jsonb, 'active' FROM seed_version;
```

- [ ] **Step 2: Verify migration compiles with backend**

Run:

```powershell
cd backend
go test ./...
```

Expected: PASS.

- [ ] **Step 3: Commit Task 15**

Run:

```powershell
git add migrations/035_assistant_context_engine.sql
git commit -m "Seed assistant context rules"
```

---

### Task 16: Final Verification

**Files:**
- No code files created in this task.

- [ ] **Step 1: Run assistant package tests**

Run:

```powershell
cd backend
go test ./internal/domain/assistant
```

Expected: PASS.

- [ ] **Step 2: Run backend tests**

Run:

```powershell
cd backend
go test ./...
```

Expected: PASS.

- [ ] **Step 3: Build backend server**

Run:

```powershell
cd backend
go build ./cmd/server
```

Expected: PASS and create the server binary for the current platform.

- [ ] **Step 4: Inspect git status**

Run:

```powershell
git status --short
```

Expected: only files changed by this plan are present as committed changes. Existing unrelated user changes may still appear and must not be reverted.

- [ ] **Step 5: Record manual verification summary**

Add a short note to the final response with:

```text
Verified:
- go test ./internal/domain/assistant
- go test ./...
- go build ./cmd/server
```

If any command fails, include the exact failing package and first actionable error.

## Self-Review

Spec coverage:

- Runtime/harness split: Tasks 1, 10, 11, 13.
- Verified Context Engine: Tasks 3, 6, 7, 12, 15.
- Data dictionary import and proposals: Tasks 2, 4, 5, 12, 14.
- AI cannot execute DDL: Task 5 creates migration drafts only; Task 14 exposes import/preview but no DDL endpoint.
- SaaS/organization/module scope: Tasks 2, 3, 5, 15.
- Permission/workflow/finance validation: Tasks 3, 6, 7, 15.
- AttentionCore budget: Tasks 3, 6, 16.
- Pi framework mapping into Go runtime: Tasks 1, 10, 11, 13.
- Tool before/after gate: Task 8.
- Event sink and auditability: Task 9.

Placeholder scan:

- The plan contains no `TBD`, no `TODO`, and no unspecified "add validation" steps.
- Migration and code snippets define concrete types, table names, commands, and expected outcomes.

Type consistency:

- `ContextRequest`, `ContextPackage`, `DictionaryImportModel`, `ToolRunner`, `AssistantRuntimeRunner`, and `EventSink` are introduced before use in later tasks.
- Repository interface methods match the fake repository methods in tests.
- Runtime delegation uses `AssistantRunRequest` and `AssistantResumeRequest` consistently.

