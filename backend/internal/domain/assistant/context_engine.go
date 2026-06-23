package assistant

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type ContextPackageRepository interface {
	CreateContextPackage(context.Context, ContextRequest, ContextPackage) (*ContextPackage, error)
}

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

func NewVerifiedContextEngine(config VerifiedContextEngineConfig) *VerifiedContextEngine {
	if config.Evaluator == nil {
		config.Evaluator = NewContextRuleEvaluator(ContextRuleEvaluatorConfig{AttentionCoreRatio: 0.4})
	}
	return &VerifiedContextEngine{resolver: config.Resolver, evaluator: config.Evaluator, repository: config.Repository, ruleSource: config.RuleSource}
}

func (e *VerifiedContextEngine) BuildContextPackage(ctx context.Context, request ContextRequest) (*ContextPackage, error) {
	if request.ActorID == uuid.Nil || request.ActorType == "" {
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
	validations := map[string]any{"permission": "checked", "workflow": "checked", "finance": "checked"}
	if len(items) == 0 {
		items = compatibilityContextItems(workContext)
		source = "compatibility_resolver"
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
	if e.repository != nil {
		return e.repository.CreateContextPackage(ctx, request, pkg)
	}
	return &pkg, nil
}

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
