# Verified Context and Tool Loop Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Verified Context and Tool Runtime the governed assistant run/resume path for ERP, finance, governance, schema preview, runtime operations, and context proposal application.

**Architecture:** The implementation keeps current APIs stable and moves behavior behind existing backend boundaries. `VerifiedContextEngine` reads active dictionary rules and packages governed context; `AssistantRuntime` rebuilds context for run and approval resume; `ToolRuntime` becomes the single execution gateway for AI-requested operations.

**Tech Stack:** Go, pgx, PostgreSQL JSONB, existing `assistant`, `toolruntime`, `erp`, `runtime`, `systemadmin`, `aigateway`, and `observability` domains.

---

## File Structure

- Modify `backend/internal/domain/assistant/context_model.go`
  - Add `ContextRuleRecord`, `ContextRuleSource`, and compact context metadata helpers.
- Modify `backend/internal/domain/assistant/context_engine.go`
  - Add dictionary rule source support, rule-backed item building, permission omission, finance risk, workflow weight, and compatibility fallback provenance.
- Modify `backend/internal/domain/assistant/context_repository.go`
  - Implement `ListActiveContextRules`.
- Modify `backend/internal/domain/assistant/context_engine_test.go`
  - Add rule-backed context tests.
- Modify `backend/internal/domain/assistant/service.go`
  - Add assistant turn context helpers, include context package metadata in AI invocations and steps, and pass context metadata into tool execution.
- Modify `backend/internal/domain/assistant/runtime.go`
  - Use the runtime context engine for `Run` and `Resume` instead of only pre-building context.
- Modify `backend/internal/domain/assistant/runtime_test.go`
  - Add run/resume context rebuild tests.
- Modify `backend/internal/domain/assistant/tool_runner.go`
  - Inject context metadata into Tool Runtime arguments and idempotency keys.
- Create `backend/internal/domain/assistant/context_prompt_test.go`
  - Test verified context prompt conversion and metadata summaries.
- Modify `backend/internal/domain/toolruntime/model.go`
  - Add no new persisted columns; keep context metadata in execution arguments.
- Modify `backend/internal/domain/toolruntime/service.go`
  - Add `RegisterAdapters` for post-construction adapter wiring.
- Modify `backend/internal/domain/toolruntime/internal_tools.go`
  - Add ERP action, schema preview, runtime operation, and context proposal apply tool adapters and default definitions.
- Create `backend/internal/domain/toolruntime/internal_tools_test.go`
  - Test the new tool adapters with fake services.
- Modify `backend/cmd/server/main.go`
  - Wire ERP, system admin schema verification, runtime operation, and context proposal adapters into Tool Runtime.
- No migration file is expected for this slice.
  - Use `assistant_steps.data`, `tool_executions.arguments`, `aigateway` metadata, and existing `context_packages`.

## Task 1: Add Dictionary Rule Source to Verified Context

**Files:**
- Modify: `backend/internal/domain/assistant/context_model.go`
- Modify: `backend/internal/domain/assistant/context_engine.go`
- Modify: `backend/internal/domain/assistant/context_repository.go`
- Test: `backend/internal/domain/assistant/context_engine_test.go`

- [ ] **Step 1: Write failing context rule tests**

Append these tests and fakes to `backend/internal/domain/assistant/context_engine_test.go`:

```go
func TestVerifiedContextEngineAppliesActiveContextRules(t *testing.T) {
	sessionID := uuid.New()
	dictionaryID := uuid.New()
	ruleID := uuid.New()
	resolver := &fakeContextResolver{
		result: WorkRecordContext{
			ModuleKey: "erp",
			Records: []WorkRecord{{
				ID:     "REQ-1",
				Type:   "requirement",
				Title:  "Approve launch requirement",
				Status: "approved",
				Data: map[string]any{
					"status":     "approved",
					"risk_level": "medium",
				},
			}},
		},
	}
	engine := NewVerifiedContextEngine(VerifiedContextEngineConfig{
		Resolver: resolver,
		Evaluator: NewContextRuleEvaluator(ContextRuleEvaluatorConfig{AttentionCoreRatio: 0.8}),
		RuleSource: &fakeContextRuleSource{rules: []ContextRuleRecord{{
			ID:                  ruleID,
			DictionaryVersionID: dictionaryID,
			ModuleKey:           "erp",
			EntityKey:           "requirement",
			FieldKey:            "status",
			RuleType:            "attention",
			Rule:                map[string]any{"base_weight": float64(9), "attention_core": true},
			Status:              DictionaryStatusActive,
		}}},
	})

	pkg, err := engine.BuildContextPackage(context.Background(), ContextRequest{
		SessionID:   sessionID,
		ActorID:     uuid.New(),
		ActorType:   "internal_human",
		ModuleKey:   "erp",
		TargetType:  "requirement",
		TokenBudget: 400,
	})
	if err != nil {
		t.Fatalf("BuildContextPackage returned error: %v", err)
	}
	if pkg.DictionaryVersionID == nil || *pkg.DictionaryVersionID != dictionaryID {
		t.Fatalf("dictionary version = %v, want %s", pkg.DictionaryVersionID, dictionaryID)
	}
	if len(pkg.AttentionCore) != 1 {
		t.Fatalf("attention core len = %d, want 1: %#v", len(pkg.AttentionCore), pkg.AttentionCore)
	}
	item := pkg.AttentionCore[0]
	if item.EntityKey != "requirement" || item.FieldKey != "status" || item.Source != "context_dictionary" {
		t.Fatalf("item = %#v, want dictionary-backed requirement.status", item)
	}
	if item.Metadata["rule_id"] != ruleID.String() {
		t.Fatalf("rule metadata = %#v, want rule_id %s", item.Metadata, ruleID)
	}
	if pkg.Provenance["source"] != "context_dictionary" {
		t.Fatalf("provenance = %#v, want context_dictionary", pkg.Provenance)
	}
}

func TestVerifiedContextEnginePermissionRuleCreatesOmission(t *testing.T) {
	engine := NewVerifiedContextEngine(VerifiedContextEngineConfig{
		Resolver: &fakeContextResolver{result: WorkRecordContext{
			ModuleKey: "governance",
			Records: []WorkRecord{{ID: "DEC-1", Type: "access_decision", Data: map[string]any{"decision": "deny"}}},
		}},
		RuleSource: &fakeContextRuleSource{rules: []ContextRuleRecord{{
			ID:        uuid.New(),
			ModuleKey: "governance",
			EntityKey: "access_decision",
			FieldKey:  "decision",
			RuleType:  "permission",
			Rule:      map[string]any{"allowed_actor_types": []any{"platform_admin"}},
			Status:    DictionaryStatusActive,
		}}},
	})

	pkg, err := engine.BuildContextPackage(context.Background(), ContextRequest{
		SessionID: uuid.New(),
		ActorID:   uuid.New(),
		ActorType: "internal_human",
		ModuleKey: "governance",
	})
	if err != nil {
		t.Fatalf("BuildContextPackage returned error: %v", err)
	}
	if len(pkg.Omissions) != 1 || pkg.Omissions[0].Reason != "permission_denied" {
		t.Fatalf("omissions = %#v, want permission_denied", pkg.Omissions)
	}
	if len(pkg.AttentionCore) != 0 {
		t.Fatalf("attention core = %#v, want denied field omitted", pkg.AttentionCore)
	}
}

func TestVerifiedContextEngineFinanceRuleCreatesRiskSignal(t *testing.T) {
	engine := NewVerifiedContextEngine(VerifiedContextEngineConfig{
		Resolver: &fakeContextResolver{result: WorkRecordContext{
			ModuleKey: "finance",
			Records: []WorkRecord{{ID: "COST-1", Type: "cost_ledger_entry", Data: map[string]any{"amount": float64(120)}}},
		}},
		RuleSource: &fakeContextRuleSource{rules: []ContextRuleRecord{{
			ID:        uuid.New(),
			ModuleKey: "finance",
			EntityKey: "cost_ledger_entry",
			FieldKey:  "amount",
			RuleType:  "finance",
			Rule:      map[string]any{"requires_validation": true, "validation_status": "unverified", "unverified_as_signal": true},
			Status:    DictionaryStatusActive,
		}}},
	})

	pkg, err := engine.BuildContextPackage(context.Background(), ContextRequest{
		SessionID: uuid.New(),
		ActorID:   uuid.New(),
		ActorType: "internal_human",
		ModuleKey: "finance",
	})
	if err != nil {
		t.Fatalf("BuildContextPackage returned error: %v", err)
	}
	if len(pkg.RiskAndSignals) != 1 {
		t.Fatalf("risk signals = %#v, want one finance signal", pkg.RiskAndSignals)
	}
	if pkg.RiskAndSignals[0].ValidationState != ValidationFinanceConflict {
		t.Fatalf("validation = %q, want finance conflict", pkg.RiskAndSignals[0].ValidationState)
	}
}

type fakeContextRuleSource struct {
	rules []ContextRuleRecord
	err   error
}

func (f *fakeContextRuleSource) ListActiveContextRules(_ context.Context, _ ContextRequest) ([]ContextRuleRecord, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.rules, nil
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
cd backend
go test ./internal/domain/assistant -run 'TestVerifiedContextEngine(AppliesActiveContextRules|PermissionRuleCreatesOmission|FinanceRuleCreatesRiskSignal)' -count=1
```

Expected: FAIL with compile errors for `ContextRuleRecord`, `RuleSource`, and `ListActiveContextRules`.

- [ ] **Step 3: Add context rule model types**

Add to `backend/internal/domain/assistant/context_model.go` below `ContextMigrationIntentInput`:

```go
type ContextRuleRecord struct {
	ID                  uuid.UUID      `json:"id"`
	DictionaryVersionID uuid.UUID      `json:"dictionary_version_id"`
	ModuleKey           string         `json:"module_key"`
	EntityKey           string         `json:"entity_key"`
	FieldKey            string         `json:"field_key"`
	RuleType            string         `json:"rule_type"`
	Rule                map[string]any `json:"rule"`
	Status              string         `json:"status"`
}

type ContextRuleSource interface {
	ListActiveContextRules(context.Context, ContextRequest) ([]ContextRuleRecord, error)
}
```

Add `context` to the import block in the same file:

```go
import (
	"context"

	"github.com/google/uuid"
)
```

- [ ] **Step 4: Add rule source support to the engine**

Modify `backend/internal/domain/assistant/context_engine.go`:

```go
type VerifiedContextEngineConfig struct {
	Resolver   ContextResolver
	Evaluator  *ContextRuleEvaluator
	Repository ContextPackageRepository
	RuleSource ContextRuleSource
}

type VerifiedContextEngine struct {
	resolver   ContextResolver
	evaluator  *ContextRuleEvaluator
	repository ContextPackageRepository
	ruleSource ContextRuleSource
}
```

Update `NewVerifiedContextEngine`:

```go
return &VerifiedContextEngine{
	resolver:   config.Resolver,
	evaluator:  config.Evaluator,
	repository: config.Repository,
	ruleSource: config.RuleSource,
}
```

Inside `BuildContextPackage`, after resolving `workContext`, load rules and build items:

```go
rules := []ContextRuleRecord{}
if e.ruleSource != nil {
	var err error
	rules, err = e.ruleSource.ListActiveContextRules(ctx, request)
	if err != nil {
		return nil, err
	}
}
items, dictionaryVersionID := contextItemsFromRules(workContext, rules, request)
source := "context_dictionary"
if len(items) == 0 {
	items = compatibilityContextItems(workContext)
	source = "compatibility_resolver"
}
validations := map[string]any{
	"permission": "checked",
	"workflow":   "checked",
	"finance":    "checked",
}
if source == "compatibility_resolver" {
	validations = map[string]any{"permission": "compatibility_checked", "workflow": "compatibility_checked", "finance": "not_applicable"}
}
pkg := e.evaluator.BuildPackage(ContextRuleEvaluationInput{
	SessionID:           request.SessionID,
	DictionaryVersionID: dictionaryVersionID,
	Items:               items,
	TokenBudget:         firstPositive(request.TokenBudget, 4096),
	Validations:         validations,
	Provenance: map[string]any{
		"source":          source,
		"fallback_source": fallbackSource(source),
		"module_key":      normalizedModule(request.ModuleKey),
		"rule_count":      len(rules),
	},
})
```

Add helper functions to the same file:

```go
func compatibilityContextItems(workContext WorkRecordContext) []ContextItem {
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
	return items
}

func contextItemsFromRules(workContext WorkRecordContext, rules []ContextRuleRecord, request ContextRequest) ([]ContextItem, *uuid.UUID) {
	items := []ContextItem{}
	var dictionaryVersionID *uuid.UUID
	for _, rule := range rules {
		if rule.Status != "" && rule.Status != DictionaryStatusActive {
			continue
		}
		if dictionaryVersionID == nil && rule.DictionaryVersionID != uuid.Nil {
			id := rule.DictionaryVersionID
			dictionaryVersionID = &id
		}
		for _, record := range workContext.Records {
			if record.Type != rule.EntityKey {
				continue
			}
			value, ok := contextRuleValue(record, rule)
			if !ok {
				continue
			}
			items = append(items, contextItemFromRule(record, rule, value, request))
		}
	}
	return items, dictionaryVersionID
}

func contextRuleValue(record WorkRecord, rule ContextRuleRecord) (any, bool) {
	if rule.FieldKey == "" || rule.FieldKey == "record" {
		return map[string]any{"title": record.Title, "status": record.Status, "data": record.Data}, true
	}
	if rule.FieldKey == "status" {
		return record.Status, true
	}
	if record.Data == nil {
		return nil, false
	}
	value, ok := record.Data[rule.FieldKey]
	return value, ok
}

func contextItemFromRule(record WorkRecord, rule ContextRuleRecord, value any, request ContextRequest) ContextItem {
	validation := ValidationVerified
	if rule.RuleType == "permission" && !ruleAllowsActor(rule.Rule, request.ActorType) {
		validation = ValidationPermissionDenied
	}
	if rule.RuleType == "finance" && financeRuleConflicts(rule.Rule) {
		validation = ValidationFinanceConflict
	}
	return ContextItem{
		EntityKey:       rule.EntityKey,
		FieldKey:        rule.FieldKey,
		RecordID:        record.ID,
		Value:           value,
		Weight:          contextRuleWeight(rule),
		EstimatedTokens: estimateRuleValueTokens(value),
		ValidationState: validation,
		Source:          "context_dictionary",
		Metadata: map[string]any{
			"rule_id":               rule.ID.String(),
			"dictionary_version_id": rule.DictionaryVersionID.String(),
			"rule_type":             rule.RuleType,
		},
	}
}
```

Add the small rule helpers:

```go
func ruleAllowsActor(rule map[string]any, actorType string) bool {
	raw, ok := rule["allowed_actor_types"]
	if !ok {
		return true
	}
	values, ok := raw.([]any)
	if !ok {
		return true
	}
	for _, value := range values {
		if fmt.Sprint(value) == actorType {
			return true
		}
	}
	return false
}

func financeRuleConflicts(rule map[string]any) bool {
	return rule["requires_validation"] == true &&
		fmt.Sprint(rule["validation_status"]) == "unverified" &&
		rule["unverified_as_signal"] == true
}

func contextRuleWeight(rule ContextRuleRecord) float64 {
	if value, ok := rule.Rule["base_weight"].(float64); ok && value > 0 {
		return value
	}
	if value, ok := rule.Rule["weight"].(float64); ok && value > 0 {
		return value
	}
	return 5
}

func estimateRuleValueTokens(value any) int {
	text := marshalToolContent(map[string]any{"value": value})
	tokens := len(text) / 4
	if tokens < 16 {
		return 16
	}
	return tokens
}

func fallbackSource(source string) string {
	if source == "context_dictionary" {
		return "compatibility_resolver"
	}
	return ""
}
```

- [ ] **Step 5: Implement Postgres active rule lookup**

Add this method to `backend/internal/domain/assistant/context_repository.go`:

```go
func (r *PostgresContextRepository) ListActiveContextRules(ctx context.Context, request ContextRequest) ([]ContextRuleRecord, error) {
	rows, err := r.db.Query(ctx, `
		SELECT cr.id, cr.dictionary_version_id, cr.module_key, cr.entity_key, cr.field_key, cr.rule_type, cr.rule, cr.status
		FROM context_rules cr
		JOIN context_dictionary_versions cdv ON cdv.id = cr.dictionary_version_id
		WHERE cr.status = 'active'
			AND cdv.status = 'active'
			AND (cr.module_key = '' OR cr.module_key = $1 OR $1 = '')
			AND (cdv.organization_id IS NULL OR cdv.organization_id IS NOT DISTINCT FROM $2)
		ORDER BY
			CASE WHEN cdv.organization_id IS NOT NULL THEN 0 ELSE 1 END,
			cr.created_at DESC
	`, request.ModuleKey, request.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("list active context rules: %w", err)
	}
	defer rows.Close()
	rules := []ContextRuleRecord{}
	for rows.Next() {
		var item ContextRuleRecord
		var ruleData map[string]any
		if err := rows.Scan(&item.ID, &item.DictionaryVersionID, &item.ModuleKey, &item.EntityKey, &item.FieldKey, &item.RuleType, &ruleData, &item.Status); err != nil {
			return nil, fmt.Errorf("scan active context rule: %w", err)
		}
		if ruleData == nil {
			ruleData = map[string]any{}
		}
		item.Rule = ruleData
		rules = append(rules, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active context rules: %w", err)
	}
	return rules, nil
}
```

- [ ] **Step 6: Wire the repository as rule source**

Modify `backend/cmd/server/main.go` context engine construction:

```go
contextEngine := assistant.NewVerifiedContextEngine(assistant.VerifiedContextEngineConfig{
	Resolver:   contextResolver,
	Evaluator:  assistant.NewContextRuleEvaluator(assistant.ContextRuleEvaluatorConfig{AttentionCoreRatio: 0.4}),
	Repository: contextRepo,
	RuleSource: contextRepo,
})
```

- [ ] **Step 7: Run tests and commit**

Run:

```bash
cd backend
go test ./internal/domain/assistant -run 'TestVerifiedContextEngine' -count=1
gofmt -w internal/domain/assistant/context_model.go internal/domain/assistant/context_engine.go internal/domain/assistant/context_repository.go cmd/server/main.go
go test ./internal/domain/assistant -run 'TestVerifiedContextEngine' -count=1
```

Expected: PASS.

Commit:

```bash
git add backend/internal/domain/assistant/context_model.go backend/internal/domain/assistant/context_engine.go backend/internal/domain/assistant/context_repository.go backend/internal/domain/assistant/context_engine_test.go backend/cmd/server/main.go
git commit -m "Add dictionary backed context rules"
```

## Task 2: Add Verified Context Prompt and Metadata Helpers

**Files:**
- Create: `backend/internal/domain/assistant/context_prompt_test.go`
- Modify: `backend/internal/domain/assistant/service.go`

- [ ] **Step 1: Write failing prompt helper tests**

Create `backend/internal/domain/assistant/context_prompt_test.go`:

```go
package assistant

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestContextPackageMetadataSummarizesPackage(t *testing.T) {
	dictionaryID := uuid.New()
	pkg := &ContextPackage{
		ID:                  uuid.New(),
		DictionaryVersionID: &dictionaryID,
		AttentionCore:       []ContextItem{{EntityKey: "requirement", FieldKey: "status"}},
		SupportingContext:   []ContextItem{{EntityKey: "project", FieldKey: "status"}},
		RiskAndSignals:      []ContextItem{{EntityKey: "cost_ledger_entry", FieldKey: "amount"}},
		Omissions:           []ContextOmission{{EntityKey: "access_decision", FieldKey: "decision", Reason: "permission_denied"}},
		Provenance:          map[string]any{"source": "context_dictionary"},
	}

	metadata := contextPackageMetadata(pkg)

	if metadata["context_package_id"] != pkg.ID.String() {
		t.Fatalf("metadata = %#v, want context package id", metadata)
	}
	if metadata["dictionary_version_id"] != dictionaryID.String() {
		t.Fatalf("metadata = %#v, want dictionary version id", metadata)
	}
	if metadata["attention_core_count"] != 1 || metadata["risk_signal_count"] != 1 || metadata["omission_count"] != 1 {
		t.Fatalf("metadata counts = %#v", metadata)
	}
}

func TestContextPackageToWorkRecordContextIncludesRisksAndOmissions(t *testing.T) {
	pkg := &ContextPackage{
		ID: uuid.New(),
		AttentionCore: []ContextItem{{
			EntityKey: "requirement",
			FieldKey:  "status",
			RecordID:  "REQ-1",
			Value:     "approved",
		}},
		RiskAndSignals: []ContextItem{{
			EntityKey:       "cost_ledger_entry",
			FieldKey:        "amount",
			RecordID:        "COST-1",
			ValidationState: ValidationFinanceConflict,
		}},
		Omissions: []ContextOmission{{EntityKey: "access_decision", FieldKey: "decision", Reason: "permission_denied"}},
	}

	workContext := contextPackageToWorkRecordContext("erp", pkg)
	prompt := systemPrompt(&Session{ModuleKey: "erp", WorkingMemory: map[string]any{}}, nil, workContext)

	if !strings.Contains(prompt, "Verified context package") {
		t.Fatalf("prompt missing verified context section: %s", prompt)
	}
	if !strings.Contains(prompt, "requirement.status") || !strings.Contains(prompt, "REQ-1") {
		t.Fatalf("prompt missing attention core: %s", prompt)
	}
	if !strings.Contains(prompt, "risk_signals=1") || !strings.Contains(prompt, "omissions=1") {
		t.Fatalf("prompt missing risk and omission summary: %s", prompt)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
cd backend
go test ./internal/domain/assistant -run 'TestContextPackage' -count=1
```

Expected: FAIL with undefined `contextPackageMetadata` and `contextPackageToWorkRecordContext`.

- [ ] **Step 3: Add metadata and prompt conversion helpers**

Add these helpers near `buildAIMessages` in `backend/internal/domain/assistant/service.go`:

```go
func contextPackageMetadata(pkg *ContextPackage) map[string]any {
	if pkg == nil {
		return map[string]any{}
	}
	metadata := map[string]any{
		"context_package_id":     pkg.ID.String(),
		"attention_core_count":   len(pkg.AttentionCore),
		"supporting_context_count": len(pkg.SupportingContext),
		"risk_signal_count":      len(pkg.RiskAndSignals),
		"omission_count":         len(pkg.Omissions),
		"context_token_budget":   pkg.TokenBudget,
		"context_estimated_tokens": pkg.TotalEstimatedTokens(),
	}
	if pkg.DictionaryVersionID != nil {
		metadata["dictionary_version_id"] = pkg.DictionaryVersionID.String()
	}
	if source, ok := pkg.Provenance["source"]; ok {
		metadata["context_source"] = source
	}
	return metadata
}

func contextPackageToWorkRecordContext(moduleKey string, pkg *ContextPackage) WorkRecordContext {
	workContext := WorkRecordContext{ModuleKey: moduleKey}
	if pkg == nil {
		return workContext
	}
	records := make([]WorkRecord, 0, len(pkg.AttentionCore)+len(pkg.SupportingContext))
	for _, item := range append(append([]ContextItem{}, pkg.AttentionCore...), pkg.SupportingContext...) {
		records = append(records, WorkRecord{
			ID:     item.RecordID,
			Type:   item.EntityKey,
			Title:  item.EntityKey + "." + item.FieldKey,
			Status: item.ValidationState,
			Data: map[string]any{
				"field_key":          item.FieldKey,
				"value":              item.Value,
				"source":             item.Source,
				"validation_state":   item.ValidationState,
				"context_package_id": pkg.ID.String(),
			},
		})
	}
	workContext.Records = records
	workContext.Metadata = map[string]any{
		"verified_context_package": true,
		"context_package_id":       pkg.ID.String(),
		"risk_signals":             len(pkg.RiskAndSignals),
		"omissions":                len(pkg.Omissions),
	}
	return workContext
}
```

If `WorkRecordContext` has no `Metadata` field, add it in `backend/internal/domain/assistant/context_resolver.go` or the file where it is defined:

```go
type WorkRecordContext struct {
	ModuleKey string
	Records   []WorkRecord
	Error     string
	Metadata  map[string]any
}
```

- [ ] **Step 4: Include verified context summary in prompt**

Modify `systemPrompt` in `backend/internal/domain/assistant/service.go` before the `Recent work records` block:

```go
if workContext.Metadata != nil && workContext.Metadata["verified_context_package"] == true {
	b.WriteString("Verified context package: ")
	b.WriteString(fmt.Sprintf("id=%s risk_signals=%v omissions=%v\n",
		fmt.Sprint(workContext.Metadata["context_package_id"]),
		workContext.Metadata["risk_signals"],
		workContext.Metadata["omissions"],
	))
}
```

- [ ] **Step 5: Run tests and commit**

Run:

```bash
cd backend
gofmt -w internal/domain/assistant/service.go internal/domain/assistant/context_prompt_test.go
go test ./internal/domain/assistant -run 'TestContextPackage' -count=1
```

Expected: PASS.

Commit:

```bash
git add backend/internal/domain/assistant/service.go backend/internal/domain/assistant/context_prompt_test.go
git commit -m "Add verified context prompt metadata"
```

## Task 3: Rebuild Context in Assistant Run and Resume

**Files:**
- Modify: `backend/internal/domain/assistant/service.go`
- Modify: `backend/internal/domain/assistant/runtime.go`
- Modify: `backend/internal/domain/assistant/runtime_test.go`

- [ ] **Step 1: Write failing runtime tests**

Append to `backend/internal/domain/assistant/runtime_test.go`:

```go
func TestAssistantRunAddsContextMetadataToInvocation(t *testing.T) {
	sessionID := uuid.New()
	actorID := uuid.New()
	pkgID := uuid.New()
	ai := &capturingAIInvoker{}
	repo := &fakeRepository{
		session: &Session{
			ID:           sessionID,
			ActorID:      actorID,
			ActorType:    "internal_human",
			ModuleKey:    "erp",
			ProviderType: "openai",
			Model:        "gpt-4o-mini",
			WorkingMemory: map[string]any{},
		},
	}
	service := NewService(repo, ai, &fakeToolExecutor{})
	engine := &fakeContextEngine{pkg: &ContextPackage{
		ID:            pkgID,
		AttentionCore: []ContextItem{{EntityKey: "requirement", FieldKey: "status", RecordID: "REQ-1", Value: "approved"}},
		Provenance:    map[string]any{"source": "context_dictionary"},
	}}
	runtime := NewAssistantRuntime(service, engine, nil, NewMemoryEventSink(repo))

	events, err := runtime.Run(context.Background(), AssistantRunRequest{
		SessionID: sessionID,
		ActorID:   actorID,
		ActorType: "internal_human",
		Input:     RunInput{Message: "summarize"},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	for range events {
	}
	if ai.lastInput.Metadata["context_package_id"] != pkgID.String() {
		t.Fatalf("AI metadata = %#v, want context package id", ai.lastInput.Metadata)
	}
	if repo.lastStep.Data["context_package_id"] != pkgID.String() {
		t.Fatalf("last step data = %#v, want context package id", repo.lastStep.Data)
	}
}

func TestAssistantResumeRebuildsContextAfterApproval(t *testing.T) {
	sessionID := uuid.New()
	actorID := uuid.New()
	approvalID := uuid.New()
	executionID := uuid.New()
	firstPkg := uuid.New()
	secondPkg := uuid.New()
	engine := &sequenceContextEngine{packages: []*ContextPackage{
		{ID: firstPkg, AttentionCore: []ContextItem{{EntityKey: "requirement", FieldKey: "status", RecordID: "REQ-1", Value: "approved"}}},
		{ID: secondPkg, AttentionCore: []ContextItem{{EntityKey: "project", FieldKey: "status", RecordID: "PRJ-1", Value: "active"}}},
	}}
	repo := &fakeRepository{
		session: &Session{
			ID:           sessionID,
			ActorID:      actorID,
			ActorType:    "internal_human",
			ModuleKey:    "erp",
			ProviderType: "openai",
			Model:        "gpt-4o-mini",
			WorkingMemory: map[string]any{},
		},
	}
	tools := &fakeToolExecutor{
		approval: &toolruntime.ToolApproval{ID: approvalID, ExecutionID: executionID, Status: toolruntime.ApprovalApproved},
		execution: &toolruntime.ToolExecution{
			ID:            executionID,
			Status:        toolruntime.ExecutionCompleted,
			ResultSummary: "ERP action posted",
			Result:        map[string]any{"ok": true},
		},
	}
	service := NewService(repo, &capturingAIInvoker{}, tools)
	runtime := NewAssistantRuntime(service, engine, nil, NewMemoryEventSink(repo))

	events, err := runtime.Resume(context.Background(), AssistantResumeRequest{
		SessionID: sessionID,
		ActorID:   actorID,
		ActorType: "internal_human",
		Input:     ResumeInput{ToolApprovalID: approvalID},
	})
	if err != nil {
		t.Fatalf("Resume returned error: %v", err)
	}
	for range events {
	}
	if engine.calls != 1 {
		t.Fatalf("context rebuild calls = %d, want 1", engine.calls)
	}
	if repo.lastStep.Data["context_package_id"] != secondPkg.String() {
		t.Fatalf("tool result step data = %#v, want refreshed context package", repo.lastStep.Data)
	}
}

type capturingAIInvoker struct {
	lastInput aigateway.InvokeInput
}

func (f *capturingAIInvoker) Invoke(_ context.Context, input aigateway.InvokeInput) (*aigateway.InvokeOutput, error) {
	f.lastInput = input
	return &aigateway.InvokeOutput{
		InvocationID: uuid.New(),
		Content:      "done",
		ProviderType: input.ProviderType,
		Model:        input.Model,
		CompletedAt:  time.Now(),
	}, nil
}

type sequenceContextEngine struct {
	packages []*ContextPackage
	calls    int
}

func (f *sequenceContextEngine) BuildContextPackage(_ context.Context, request ContextRequest) (*ContextPackage, error) {
	if len(f.packages) == 0 {
		f.calls++
		return &ContextPackage{ID: uuid.New(), SessionID: request.SessionID}, nil
	}
	index := f.calls
	if index >= len(f.packages) {
		index = len(f.packages) - 1
	}
	f.calls++
	return f.packages[index], nil
}
```

Add `toolruntime` to the test imports:

```go
"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/toolruntime"
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
cd backend
go test ./internal/domain/assistant -run 'TestAssistant(RunAddsContextMetadataToInvocation|ResumeRebuildsContextAfterApproval)' -count=1
```

Expected: FAIL because runtime still delegates to legacy loops without passing the runtime context package into invocation and resumed tool result metadata.

- [ ] **Step 3: Add assistant turn context helper**

Add this type and helper near the run loop functions in `backend/internal/domain/assistant/service.go`:

```go
type assistantTurnContext struct {
	Package     *ContextPackage
	WorkContext WorkRecordContext
	Metadata    map[string]any
}

func (s *Service) buildAssistantTurnContext(ctx context.Context, session *Session, intent string, tokenBudget int, engine ContextPackageBuilder) (assistantTurnContext, error) {
	if engine == nil {
		engine = s.contextEngine
	}
	if engine != nil {
		pkg, err := engine.BuildContextPackage(ctx, ContextRequest{
			SessionID:      session.ID,
			ActorID:        session.ActorID,
			ActorType:      session.ActorType,
			OrganizationID: session.OrganizationID,
			ModuleKey:      session.ModuleKey,
			WorkflowID:     session.WorkflowID,
			TaskID:         session.TaskID,
			TargetType:     session.TargetType,
			TargetID:       session.TargetID,
			Intent:         intent,
			Mode:           session.Mode,
			TokenBudget:    firstPositive(tokenBudget, 4096),
		})
		if err != nil {
			return assistantTurnContext{}, err
		}
		return assistantTurnContext{
			Package:     pkg,
			WorkContext: contextPackageToWorkRecordContext(session.ModuleKey, pkg),
			Metadata:    contextPackageMetadata(pkg),
		}, nil
	}
	workContext := WorkRecordContext{ModuleKey: session.ModuleKey}
	if s.contextResolver != nil {
		workContext = s.contextResolver.Resolve(ctx, session)
	}
	return assistantTurnContext{
		WorkContext: workContext,
		Metadata: map[string]any{
			"context_record_count": len(workContext.Records),
			"context_error":        workContext.Error,
			"context_source":       "compatibility_resolver",
		},
	}, nil
}
```

- [ ] **Step 4: Pass runtime context engine into loops**

Change runtime delegation in `backend/internal/domain/assistant/runtime.go`:

```go
func (r *AssistantRuntime) Run(ctx context.Context, request AssistantRunRequest) (<-chan RunEvent, error) {
	return r.service.runWithContextEngine(ctx, request.SessionID, request.ActorID, request.ActorType, request.Input, r.contextEngine)
}

func (r *AssistantRuntime) Resume(ctx context.Context, request AssistantResumeRequest) (<-chan RunEvent, error) {
	return r.service.resumeWithContextEngine(ctx, request.SessionID, request.ActorID, request.ActorType, request.Input, r.contextEngine)
}
```

Add private methods to `backend/internal/domain/assistant/service.go`:

```go
func (s *Service) runWithContextEngine(ctx context.Context, sessionID uuid.UUID, actorID uuid.UUID, actorType string, input RunInput, engine ContextPackageBuilder) (<-chan RunEvent, error) {
	if strings.TrimSpace(input.Message) == "" {
		return nil, fmt.Errorf("%w: message is required", ErrValidation)
	}
	if s.ai == nil {
		return nil, fmt.Errorf("%w: ai gateway is not configured", ErrValidation)
	}
	if s.tools == nil {
		return nil, fmt.Errorf("%w: tool runtime is not configured", ErrValidation)
	}
	session, err := s.repo.GetSession(ctx, sessionID, actorID, actorType)
	if err != nil {
		return nil, err
	}
	if err := s.repo.UpdateSessionStatus(ctx, session.ID, StatusRunning, ""); err != nil {
		return nil, err
	}
	events := make(chan RunEvent)
	go s.runLoop(ctx, events, session, input, engine)
	return events, nil
}

func (s *Service) resumeWithContextEngine(ctx context.Context, sessionID uuid.UUID, actorID uuid.UUID, actorType string, input ResumeInput, engine ContextPackageBuilder) (<-chan RunEvent, error) {
	if input.ToolApprovalID == uuid.Nil {
		return nil, fmt.Errorf("%w: tool_approval_id is required", ErrValidation)
	}
	if s.ai == nil {
		return nil, fmt.Errorf("%w: ai gateway is not configured", ErrValidation)
	}
	if s.tools == nil {
		return nil, fmt.Errorf("%w: tool runtime is not configured", ErrValidation)
	}
	session, err := s.repo.GetSession(ctx, sessionID, actorID, actorType)
	if err != nil {
		return nil, err
	}
	if err := s.repo.UpdateSessionStatus(ctx, session.ID, StatusRunning, ""); err != nil {
		return nil, err
	}
	events := make(chan RunEvent)
	go s.resumeLoop(ctx, events, session, input, engine)
	return events, nil
}
```

Update the existing `runLegacy` and `resumeLegacy` callers:

```go
return s.runWithContextEngine(ctx, sessionID, actorID, actorType, input, s.contextEngine)
```

```go
return s.resumeWithContextEngine(ctx, sessionID, actorID, actorType, input, s.contextEngine)
```

- [ ] **Step 5: Use turn context in run loop and resume loop**

Change signatures:

```go
func (s *Service) runLoop(ctx context.Context, events chan<- RunEvent, session *Session, input RunInput, engine ContextPackageBuilder)
func (s *Service) resumeLoop(ctx context.Context, events chan<- RunEvent, session *Session, input ResumeInput, engine ContextPackageBuilder)
```

In `runLoop`, replace direct `contextResolver` usage with:

```go
turnContext, err := s.buildAssistantTurnContext(ctx, session, input.Message, 4096, engine)
if err != nil {
	fail(err, 0)
	return
}
```

Use:

```go
messages := buildAIMessages(session, memories, history, turnContext.WorkContext)
```

Call continuation with `turnContext`:

```go
s.continueAssistantTurns(ctx, send, fail, session, messages, turnContext, tools, providerID, channelID, providerType, model, serviceTier, effort, 1)
```

In `resumeLoop`, build context after the completed tool execution check and before adding the tool result message:

```go
turnContext, err := s.buildAssistantTurnContext(ctx, session, "resume_after_tool_approval", 4096, engine)
if err != nil {
	fail(err, turn)
	return
}
payload := map[string]any{
	"status":             execution.Status,
	"summary":            execution.ResultSummary,
	"result":             execution.Result,
	"error":              execution.ErrorMessage,
	"context_package_id": turnContext.Metadata["context_package_id"],
}
```

Use `turnContext` when rebuilding messages and continuing.

- [ ] **Step 6: Attach context metadata to AI invocation and steps**

Change `continueAssistantTurns` parameter:

```go
turnContext assistantTurnContext
```

Replace invocation metadata with a merged map:

```go
metadata := map[string]any{
	"assistant_session_id":   session.ID.String(),
	"module_key":             session.ModuleKey,
	"position_id":            uuidString(session.PositionID),
	"position_assignment_id": uuidString(session.PositionAssignmentID),
}
for key, value := range turnContext.Metadata {
	metadata[key] = value
}
```

Use `metadata` in `aigateway.InvokeInput.Metadata`.

For the LLM step data:

```go
stepData := map[string]any{"tool_call_count": len(output.ToolCalls), "model": output.Model, "provider_type": output.ProviderType}
for key, value := range turnContext.Metadata {
	stepData[key] = value
}
```

Use `stepData` in `AddStepInput.Data`.

- [ ] **Step 7: Run tests and commit**

Run:

```bash
cd backend
gofmt -w internal/domain/assistant/service.go internal/domain/assistant/runtime.go internal/domain/assistant/runtime_test.go
go test ./internal/domain/assistant -run 'TestAssistant(RunAddsContextMetadataToInvocation|ResumeRebuildsContextAfterApproval|RuntimeBuildsContextBeforeLegacyRun)' -count=1
go test ./internal/domain/assistant -count=1
```

Expected: PASS.

Commit:

```bash
git add backend/internal/domain/assistant/service.go backend/internal/domain/assistant/runtime.go backend/internal/domain/assistant/runtime_test.go
git commit -m "Rebuild verified context in assistant runtime"
```

## Task 4: Pass Verified Context Metadata Through Tool Runner

**Files:**
- Modify: `backend/internal/domain/assistant/tool_runner.go`
- Modify: `backend/internal/domain/assistant/service.go`
- Test: `backend/internal/domain/assistant/runtime_test.go`

- [ ] **Step 1: Write failing tool metadata test**

Append to `backend/internal/domain/assistant/runtime_test.go`:

```go
func TestToolRunnerInjectsContextMetadata(t *testing.T) {
	sessionID := uuid.New()
	pkgID := uuid.New()
	executor := &capturingToolExecutor{}
	runner := NewToolRunner(executor, ToolRunnerConfig{})
	session := &Session{
		ID:        sessionID,
		ActorID:   uuid.New(),
		ActorType: "internal_human",
		ModuleKey: "erp",
	}

	_, err := runner.ExecuteTool(context.Background(), ToolRunRequest{
		Session:        session,
		ContextPackage: &ContextPackage{ID: pkgID},
		Call: ToolCallRequest{
			ID:        "call-1",
			Name:      "erp.action.execute",
			Arguments: map[string]any{"table_code": "MREQ", "key": "REQ-1", "action": "approve"},
		},
	})
	if err != nil {
		t.Fatalf("ExecuteTool returned error: %v", err)
	}
	if executor.input.Arguments["context_package_id"] != pkgID.String() {
		t.Fatalf("tool args = %#v, want context package id", executor.input.Arguments)
	}
	if executor.input.IdempotencyKey != "assistant:"+sessionID.String()+":"+pkgID.String()+":call-1" {
		t.Fatalf("idempotency = %q", executor.input.IdempotencyKey)
	}
}

type capturingToolExecutor struct {
	input toolruntime.ExecuteToolInput
}

func (f *capturingToolExecutor) ExecuteTool(_ context.Context, input toolruntime.ExecuteToolInput) (*toolruntime.ExecuteToolOutput, error) {
	f.input = input
	return &toolruntime.ExecuteToolOutput{Execution: &toolruntime.ToolExecution{ID: uuid.New(), Status: toolruntime.ExecutionCompleted, Result: map[string]any{}}}, nil
}

func (f *capturingToolExecutor) ListTools(context.Context, int) ([]toolruntime.ToolDefinition, error) {
	return []toolruntime.ToolDefinition{}, nil
}

func (f *capturingToolExecutor) GetApproval(context.Context, uuid.UUID) (*toolruntime.ToolApproval, error) {
	return nil, toolruntime.ErrNotFound
}

func (f *capturingToolExecutor) GetExecution(context.Context, uuid.UUID) (*toolruntime.ToolExecution, error) {
	return nil, toolruntime.ErrNotFound
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
cd backend
go test ./internal/domain/assistant -run TestToolRunnerInjectsContextMetadata -count=1
```

Expected: FAIL because `ToolRunner` does not inject `context_package_id` and does not include package ID in idempotency.

- [ ] **Step 3: Update ToolRunner argument and idempotency handling**

Modify `backend/internal/domain/assistant/tool_runner.go`:

```go
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
	args := copyMap(request.Call.Arguments)
	if args == nil {
		args = map[string]any{}
	}
	contextPackageID := ""
	if request.ContextPackage != nil && request.ContextPackage.ID != uuid.Nil {
		contextPackageID = request.ContextPackage.ID.String()
		args["context_package_id"] = contextPackageID
		args["context_source"] = request.ContextPackage.Provenance["source"]
	}
	args["assistant_session_id"] = request.Session.ID.String()
	idempotencyKey := fmt.Sprintf("assistant:%s:%s", request.Session.ID, request.Call.ID)
	if contextPackageID != "" {
		idempotencyKey = fmt.Sprintf("assistant:%s:%s:%s", request.Session.ID, contextPackageID, request.Call.ID)
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
		IdempotencyKey: idempotencyKey,
		Arguments:      args,
	})
}
```

- [ ] **Step 4: Use ToolRunner in assistant continuation**

In `continueAssistantTurns` in `backend/internal/domain/assistant/service.go`, replace direct `s.tools.ExecuteTool` with:

```go
runner := NewToolRunner(s.tools, ToolRunnerConfig{})
result, err := runner.ExecuteTool(ctx, ToolRunRequest{
	Session:        session,
	ContextPackage: turnContext.Package,
	Call: ToolCallRequest{
		ID:        callID,
		Name:      call.Name,
		Arguments: call.Arguments,
	},
	InvocationID: &output.InvocationID,
})
```

Remove the old direct `toolruntime.ExecuteToolInput` construction.

When creating the tool call step, include context metadata:

```go
callData := map[string]any{"tool_call_id": callID, "tool_name": call.Name, "arguments": call.Arguments}
for key, value := range turnContext.Metadata {
	callData[key] = value
}
```

Use `callData` in the `StepToolCall` step.

- [ ] **Step 5: Run tests and commit**

Run:

```bash
cd backend
gofmt -w internal/domain/assistant/tool_runner.go internal/domain/assistant/service.go internal/domain/assistant/runtime_test.go
go test ./internal/domain/assistant -run 'TestToolRunnerInjectsContextMetadata|TestAssistantRunAddsContextMetadataToInvocation' -count=1
go test ./internal/domain/assistant -count=1
```

Expected: PASS.

Commit:

```bash
git add backend/internal/domain/assistant/tool_runner.go backend/internal/domain/assistant/service.go backend/internal/domain/assistant/runtime_test.go
git commit -m "Attach verified context to assistant tools"
```

## Task 5: Add Phase 4 Tool Runtime Adapters

**Files:**
- Modify: `backend/internal/domain/toolruntime/service.go`
- Modify: `backend/internal/domain/toolruntime/internal_tools.go`
- Create: `backend/internal/domain/toolruntime/internal_tools_test.go`

- [ ] **Step 1: Write failing adapter tests**

Create `backend/internal/domain/toolruntime/internal_tools_test.go`:

```go
package toolruntime

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/erp"
	domainruntime "github.com/selfevo-AI/meta-org-saas/backend/internal/domain/runtime"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/systemadmin"
)

func TestERPActionExecuteToolRunsERPService(t *testing.T) {
	erpSvc := &fakeERPActionService{}
	tools := InternalToolsWithPlatform(nil, nil, nil, PlatformToolServices{ERP: erpSvc})
	result, err := tools["erp.action.execute"](context.Background(), ExecuteToolInput{
		ActorID:   uuid.New(),
		ActorType: "internal_human",
		Arguments: map[string]any{
			"table_code": "MREQ",
			"key":        "REQ-1",
			"action":     "approve",
			"data":       map[string]any{"approver": "u1"},
		},
	})
	if err != nil {
		t.Fatalf("tool returned error: %v", err)
	}
	if erpSvc.tableCode != "MREQ" || erpSvc.key != "REQ-1" || erpSvc.action != "approve" {
		t.Fatalf("erp call = %s/%s/%s", erpSvc.tableCode, erpSvc.key, erpSvc.action)
	}
	if result.Data["erp_action"] == nil {
		t.Fatalf("result data = %#v, want erp_action", result.Data)
	}
}

func TestSchemaChangePreviewToolRunsVerifier(t *testing.T) {
	requestID := uuid.New()
	verifier := &fakeSchemaVerifier{}
	tools := InternalToolsWithPlatform(nil, nil, nil, PlatformToolServices{SchemaVerifier: verifier})
	result, err := tools["schema.change.preview"](context.Background(), ExecuteToolInput{
		ActorID:   uuid.New(),
		ActorType: "internal_human",
		Arguments: map[string]any{"request_id": requestID.String()},
	})
	if err != nil {
		t.Fatalf("tool returned error: %v", err)
	}
	if verifier.requestID != requestID {
		t.Fatalf("request id = %s, want %s", verifier.requestID, requestID)
	}
	if result.Data["verification"] == nil {
		t.Fatalf("result data = %#v, want verification", result.Data)
	}
}

func TestRuntimeOperationExecuteToolRunsRuntimeService(t *testing.T) {
	runtimeSvc := &fakeRuntimeOperationService{}
	tools := InternalToolsWithPlatform(nil, nil, nil, PlatformToolServices{Runtime: runtimeSvc})
	result, err := tools["runtime.operation.execute"](context.Background(), ExecuteToolInput{
		ActorID:   uuid.New(),
		ActorType: "internal_human",
		Arguments: map[string]any{
			"operation_id": "op-1",
			"path":         map[string]any{"recordKey": "R-1"},
			"query":        map[string]any{"limit": "10"},
			"body":         map[string]any{"title": "Runtime row"},
		},
	})
	if err != nil {
		t.Fatalf("tool returned error: %v", err)
	}
	if runtimeSvc.operationID != "op-1" {
		t.Fatalf("operation id = %s, want op-1", runtimeSvc.operationID)
	}
	if result.Data["runtime_result"] == nil {
		t.Fatalf("result data = %#v, want runtime_result", result.Data)
	}
}

func TestContextProposalApplyToolRequiresProposalService(t *testing.T) {
	tools := InternalToolsWithPlatform(nil, nil, nil, PlatformToolServices{})
	_, err := tools["context.proposal.apply"](context.Background(), ExecuteToolInput{
		ActorID:   uuid.New(),
		ActorType: "internal_human",
		Arguments: map[string]any{"proposal_id": uuid.New().String()},
	})
	if err == nil {
		t.Fatalf("context proposal tool returned nil error without service")
	}
}

type fakeERPActionService struct {
	tableCode string
	key       string
	action    string
}

func (f *fakeERPActionService) RunAction(_ context.Context, tableCode string, key string, action string, input erp.ActionInput) (*erp.ActionResult, error) {
	f.tableCode = tableCode
	f.key = key
	f.action = action
	return &erp.ActionResult{TableCode: tableCode, Key: key, Action: action, Status: "approved", Record: &erp.Record{TableCode: tableCode, Key: key}}, nil
}

type fakeSchemaVerifier struct {
	requestID uuid.UUID
}

func (f *fakeSchemaVerifier) VerifySchemaChange(_ context.Context, actorID uuid.UUID, requestID uuid.UUID) (*systemadmin.SchemaVerificationReport, error) {
	f.requestID = requestID
	return &systemadmin.SchemaVerificationReport{ChangeRequestID: requestID, Status: "passed", CanApply: true}, nil
}

type fakeRuntimeOperationService struct {
	operationID string
}

func (f *fakeRuntimeOperationService) ExecuteOperation(_ context.Context, operationID string, input domainruntime.RuntimeExecutionRequest) (*domainruntime.RuntimeExecutionResult, error) {
	f.operationID = operationID
	return &domainruntime.RuntimeExecutionResult{Status: "ok", Data: input.Body}, nil
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
cd backend
go test ./internal/domain/toolruntime -run 'Test(ERPActionExecuteTool|SchemaChangePreviewTool|RuntimeOperationExecuteTool|ContextProposalApplyTool)' -count=1
```

Expected: FAIL with undefined `InternalToolsWithPlatform` and `PlatformToolServices`.

- [ ] **Step 3: Add adapter registration support**

Add to `backend/internal/domain/toolruntime/service.go` after `NewService`:

```go
func (s *Service) RegisterAdapters(adapters map[string]ToolAdapter) {
	if s.adapters == nil {
		s.adapters = map[string]ToolAdapter{}
	}
	for name, adapter := range adapters {
		if name == "" || adapter == nil {
			continue
		}
		s.adapters[name] = adapter
	}
}
```

- [ ] **Step 4: Add platform tool service interfaces**

Modify imports in `backend/internal/domain/toolruntime/internal_tools.go`:

```go
domainruntime "github.com/selfevo-AI/meta-org-saas/backend/internal/domain/runtime"
"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/systemadmin"
```

Add interfaces near the existing service interfaces:

```go
type ERPActionService interface {
	RunAction(context.Context, string, string, string, erp.ActionInput) (*erp.ActionResult, error)
}

type SchemaVerifier interface {
	VerifySchemaChange(context.Context, uuid.UUID, uuid.UUID) (*systemadmin.SchemaVerificationReport, error)
}

type RuntimeOperationService interface {
	ExecuteOperation(context.Context, string, domainruntime.RuntimeExecutionRequest) (*domainruntime.RuntimeExecutionResult, error)
}

type ContextProposalService interface {
	ApplyContextProposal(context.Context, uuid.UUID, uuid.UUID, string) (map[string]any, error)
}

type PlatformToolServices struct {
	ERP             ERPActionService
	SchemaVerifier  SchemaVerifier
	Runtime         RuntimeOperationService
	ContextProposal ContextProposalService
}
```

- [ ] **Step 5: Add InternalToolsWithPlatform and keep the old constructor**

Replace the start of `InternalTools` with:

```go
func InternalTools(projectSvc ProjectService, financeSvc FinanceService, evolutionSvc EvolutionService) map[string]ToolAdapter {
	return InternalToolsWithPlatform(projectSvc, financeSvc, evolutionSvc, PlatformToolServices{})
}

func InternalToolsWithPlatform(projectSvc ProjectService, financeSvc FinanceService, evolutionSvc EvolutionService, platform PlatformToolServices) map[string]ToolAdapter {
	tools := map[string]ToolAdapter{
		"governance.explain_decision": explainGovernanceDecision,
	}
```

Keep existing project, finance, and evolution adapter logic inside `InternalToolsWithPlatform`.

Before the final `return tools`, add:

```go
if platform.ERP == nil {
	tools["erp.action.execute"] = notConfiguredTool("ERP action service is not configured")
} else {
	tools["erp.action.execute"] = erpActionExecuteTool(platform.ERP)
}
if platform.SchemaVerifier == nil {
	tools["schema.change.preview"] = notConfiguredTool("schema verifier is not configured")
} else {
	tools["schema.change.preview"] = schemaChangePreviewTool(platform.SchemaVerifier)
}
if platform.Runtime == nil {
	tools["runtime.operation.execute"] = notConfiguredTool("runtime operation service is not configured")
} else {
	tools["runtime.operation.execute"] = runtimeOperationExecuteTool(platform.Runtime)
}
if platform.ContextProposal == nil {
	tools["context.proposal.apply"] = notConfiguredTool("context proposal service is not configured")
} else {
	tools["context.proposal.apply"] = contextProposalApplyTool(platform.ContextProposal)
}
return tools
```

- [ ] **Step 6: Add the adapter implementations**

Add to `backend/internal/domain/toolruntime/internal_tools.go`:

```go
func erpActionExecuteTool(erpSvc ERPActionService) ToolAdapter {
	return func(ctx context.Context, input ExecuteToolInput) (ToolResult, error) {
		tableCode := stringArg(input.Arguments, "table_code")
		key := firstNonEmptyString(stringArg(input.Arguments, "key"), stringArg(input.Arguments, "record_key"), stringArg(input.Arguments, "target_key"))
		action := stringArg(input.Arguments, "action")
		if tableCode == "" || key == "" || action == "" {
			return ToolResult{}, fmt.Errorf("%w: table_code, key, and action are required", ErrValidation)
		}
		result, err := erpSvc.RunAction(ctx, tableCode, key, action, erp.ActionInput{Data: mapArg(input.Arguments, "data")})
		if err != nil {
			return ToolResult{}, err
		}
		return ToolResult{Summary: "ERP action executed", Data: map[string]any{"erp_action": result}}, nil
	}
}

func schemaChangePreviewTool(verifier SchemaVerifier) ToolAdapter {
	return func(ctx context.Context, input ExecuteToolInput) (ToolResult, error) {
		requestID, err := firstUUIDArg(input.Arguments, "request_id", "schema_change_request_id", "id")
		if err != nil {
			return ToolResult{}, err
		}
		report, err := verifier.VerifySchemaChange(ctx, input.ActorID, requestID)
		if err != nil {
			return ToolResult{}, err
		}
		return ToolResult{Summary: "Schema change verified", Data: map[string]any{"verification": report}}, nil
	}
}

func runtimeOperationExecuteTool(runtimeSvc RuntimeOperationService) ToolAdapter {
	return func(ctx context.Context, input ExecuteToolInput) (ToolResult, error) {
		operationID := firstNonEmptyString(stringArg(input.Arguments, "operation_id"), stringArg(input.Arguments, "id"))
		if operationID == "" {
			return ToolResult{}, fmt.Errorf("%w: operation_id is required", ErrValidation)
		}
		result, err := runtimeSvc.ExecuteOperation(ctx, operationID, domainruntime.RuntimeExecutionRequest{
			Path:  stringMapArg(input.Arguments, "path"),
			Query: stringMapArg(input.Arguments, "query"),
			Body:  mapArg(input.Arguments, "body"),
		})
		if err != nil {
			return ToolResult{}, err
		}
		return ToolResult{Summary: "Runtime operation executed", Data: map[string]any{"runtime_result": result}}, nil
	}
}

func contextProposalApplyTool(proposals ContextProposalService) ToolAdapter {
	return func(ctx context.Context, input ExecuteToolInput) (ToolResult, error) {
		proposalID, err := firstUUIDArg(input.Arguments, "proposal_id", "context_proposal_id", "id")
		if err != nil {
			return ToolResult{}, err
		}
		result, err := proposals.ApplyContextProposal(ctx, proposalID, input.ActorID, input.ActorType)
		if err != nil {
			return ToolResult{}, err
		}
		return ToolResult{Summary: "Context proposal applied", Data: map[string]any{"context_proposal": result}}, nil
	}
}
```

Add helper functions:

```go
func firstUUIDArg(args map[string]any, keys ...string) (uuid.UUID, error) {
	for _, key := range keys {
		raw := stringArg(args, key)
		if raw == "" {
			continue
		}
		id, err := uuid.Parse(raw)
		if err != nil {
			return uuid.Nil, fmt.Errorf("%w: invalid %s", ErrValidation, key)
		}
		return id, nil
	}
	return uuid.Nil, fmt.Errorf("%w: %s is required", ErrValidation, strings.Join(keys, " or "))
}

func stringMapArg(args map[string]any, key string) map[string]string {
	raw, ok := args[key].(map[string]any)
	if !ok {
		if values, ok := args[key].(map[string]string); ok {
			return values
		}
		return map[string]string{}
	}
	result := map[string]string{}
	for itemKey, value := range raw {
		result[itemKey] = fmt.Sprint(value)
	}
	return result
}
```

- [ ] **Step 7: Add default tool definitions**

Append these entries to `DefaultToolDefinitions()`:

```go
{Name: "erp.action.execute", Description: "Execute an ERP business action", SourceType: SourceInternalAPI, DefaultPolicy: PolicyApprove, RiskLevel: "high", RequiredLevel: "L3", ToolCategory: ToolCategoryBusinessApproval, ApprovalTierRequired: ApprovalTierReviewer},
{Name: "schema.change.preview", Description: "Verify a schema change request without applying it", SourceType: SourceInternalAPI, DefaultPolicy: PolicyNotify, RiskLevel: "low", RequiredLevel: "L2", ToolCategory: ToolCategoryExecutionOperation, ApprovalTierRequired: ApprovalTierExecutor},
{Name: "runtime.operation.execute", Description: "Execute a platform runtime operation", SourceType: SourceInternalAPI, DefaultPolicy: PolicyNotify, RiskLevel: "medium", RequiredLevel: "L2", ToolCategory: ToolCategoryExecutionOperation, ApprovalTierRequired: ApprovalTierExecutor},
{Name: "context.proposal.apply", Description: "Apply an approved context change proposal", SourceType: SourceManualApproval, DefaultPolicy: PolicyApprove, RiskLevel: "high", RequiredLevel: "L3", ToolCategory: ToolCategoryBusinessApproval, ApprovalTierRequired: ApprovalTierReviewer},
```

- [ ] **Step 8: Run tests and commit**

Run:

```bash
cd backend
gofmt -w internal/domain/toolruntime/service.go internal/domain/toolruntime/internal_tools.go internal/domain/toolruntime/internal_tools_test.go
go test ./internal/domain/toolruntime -run 'Test(ERPActionExecuteTool|SchemaChangePreviewTool|RuntimeOperationExecuteTool|ContextProposalApplyTool)' -count=1
go test ./internal/domain/toolruntime -count=1
```

Expected: PASS.

Commit:

```bash
git add backend/internal/domain/toolruntime/service.go backend/internal/domain/toolruntime/internal_tools.go backend/internal/domain/toolruntime/internal_tools_test.go
git commit -m "Add verified loop tool adapters"
```

## Task 6: Expose Context Proposal Application Through Assistant Service

**Files:**
- Modify: `backend/internal/domain/assistant/service.go`
- Test: `backend/internal/domain/assistant/service_test.go`

- [ ] **Step 1: Write failing service test**

Append to `backend/internal/domain/assistant/service_test.go`:

```go
func TestApplyContextProposalUsesHumanConfirmation(t *testing.T) {
	proposalID := uuid.New()
	reviewerID := uuid.New()
	repo := &fakeRepository{
		proposal: &Proposal{
			ID:           proposalID,
			SessionID:    uuid.New(),
			ModuleKey:    "erp",
			TargetType:   "erp_action",
			ProposalType: "context_change",
			Status:       ProposalPending,
			Payload:      map[string]any{"table_code": "MREQ", "key": "REQ-1", "action": "approve"},
		},
	}
	applicator := &fakeProposalApplicator{}
	svc := NewService(repo, nil, nil, WithProposalApplicator(applicator))

	result, err := svc.ApplyContextProposal(context.Background(), proposalID, reviewerID, "internal_human")
	if err != nil {
		t.Fatalf("ApplyContextProposal returned error: %v", err)
	}
	if result["proposal_id"] != proposalID.String() || result["status"] != ProposalApplied {
		t.Fatalf("result = %#v, want applied proposal", result)
	}
	if applicator.calls != 1 {
		t.Fatalf("applicator calls = %d, want 1", applicator.calls)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd backend
go test ./internal/domain/assistant -run TestApplyContextProposalUsesHumanConfirmation -count=1
```

Expected: FAIL with undefined `ApplyContextProposal`.

- [ ] **Step 3: Add ApplyContextProposal method**

Add to `backend/internal/domain/assistant/service.go` near proposal methods:

```go
func (s *Service) ApplyContextProposal(ctx context.Context, proposalID uuid.UUID, reviewerID uuid.UUID, reviewerType string) (map[string]any, error) {
	proposal, err := s.ConfirmProposal(ctx, proposalID, reviewerID, reviewerType)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"proposal_id": proposal.ID.String(),
		"status":      proposal.Status,
		"module_key":  proposal.ModuleKey,
		"target_type": proposal.TargetType,
		"target_id":   uuidString(proposal.TargetID),
		"apply_result": proposal.ApplyResult,
	}, nil
}
```

- [ ] **Step 4: Run tests and commit**

Run:

```bash
cd backend
gofmt -w internal/domain/assistant/service.go internal/domain/assistant/service_test.go
go test ./internal/domain/assistant -run 'TestApplyContextProposalUsesHumanConfirmation|TestConfirmProposalRequiresHumanAndIsIdempotent' -count=1
```

Expected: PASS.

Commit:

```bash
git add backend/internal/domain/assistant/service.go backend/internal/domain/assistant/service_test.go
git commit -m "Expose context proposal tool application"
```

## Task 7: Wire Platform Tool Adapters in Server Startup

**Files:**
- Modify: `backend/cmd/server/main.go`
- Test: compile through `go test ./...`

- [ ] **Step 1: Wire adapters that are available before assistant creation**

Modify the Tool Runtime construction in `backend/cmd/server/main.go`:

```go
toolSvc := toolruntime.NewService(
	toolRepo,
	govSvc,
	toolruntime.InternalToolsWithPlatform(projectSvc, financeSvc, evoSvc, toolruntime.PlatformToolServices{
		ERP:            erpSvc,
		SchemaVerifier: systemAdminSvc,
		Runtime:        runtimeSvc,
	}),
	toolruntime.WithObservability(obsSvc),
	toolruntime.WithSecurityKernel(securityKernel),
)
```

- [ ] **Step 2: Register context proposal adapter after assistant creation**

After `assistantSvc` is created and before `assistantHandler := assistant.NewHandler(assistantSvc)`, add:

```go
toolSvc.RegisterAdapters(toolruntime.InternalToolsWithPlatform(nil, nil, nil, toolruntime.PlatformToolServices{
	ContextProposal: assistantSvc,
}))
```

This works because `assistant.Service` implements the `ContextProposalService` interface by method shape, without importing `assistant` from `toolruntime`.

- [ ] **Step 3: Run backend compile tests**

Run:

```bash
cd backend
gofmt -w cmd/server/main.go
go test ./...
```

Expected: PASS.

- [ ] **Step 4: Commit**

Commit:

```bash
git add backend/cmd/server/main.go
git commit -m "Wire verified loop tool adapters"
```

## Task 8: Final Verification

**Files:**
- Review: all modified backend files
- No expected frontend changes

- [ ] **Step 1: Run focused backend tests**

Run:

```bash
cd backend
go test ./internal/domain/assistant ./internal/domain/toolruntime ./internal/domain/aigateway
```

Expected: PASS.

- [ ] **Step 2: Run full backend tests**

Run:

```bash
cd backend
go test ./...
```

Expected: PASS.

- [ ] **Step 3: Build backend**

Run:

```bash
cd backend
go build ./cmd/server
```

Expected: PASS with no output.

- [ ] **Step 4: Check formatting and whitespace**

Run:

```bash
git diff --check
```

Expected: no output.

- [ ] **Step 5: Inspect final diff**

Run:

```bash
git status --short
git diff --stat HEAD
```

Expected: only the intended Phase 4 files are changed. Untracked user files such as the YC markdown, investor archive, investor directory, and tools directory remain untracked unless the user explicitly asks to include them.

- [ ] **Step 6: Final commit if verification caused cleanup changes**

If formatting or final review changed files, commit them:

```bash
git add backend/internal/domain/assistant backend/internal/domain/toolruntime backend/cmd/server/main.go
git commit -m "Finalize verified context tool loop"
```

If there are no additional changes, do not create an empty commit.

## Plan Self-Review

- Spec coverage: context dictionary primary path is covered in Task 1; assistant run/resume context rebuild is covered in Task 3; Tool Runtime-only execution is covered in Tasks 4, 5, and 7; context proposal governance is covered in Task 6; observability/cost linkage metadata is covered by context metadata in AI invocation, step data, and tool arguments.
- Scope check: no frontend UI work is included because this slice can satisfy the Phase 4 backend control-loop requirements without changing visible UI.
- Migration check: no schema migration is planned because existing JSON metadata fields and existing `context_packages` carry the required references.
- Type consistency: `ContextRuleSource`, `ContextPackageBuilder`, `ToolExecutor`, `PlatformToolServices`, and `ContextProposalService` are all introduced before use in the task order.

