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
	GetContextChangeProposal(context.Context, uuid.UUID) (*ContextChangeProposal, error)
	ActivateContextRules(context.Context, *ContextChangeProposal, uuid.UUID, []ContextRuleRecord) ([]ContextRuleRecord, error)
	MarkContextChangeProposalApplied(context.Context, uuid.UUID, uuid.UUID, map[string]any) (*ContextChangeProposal, error)
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
	DictionaryVersionID uuid.UUID                  `json:"dictionary_version_id"`
	Validation          DictionaryValidationResult `json:"validation"`
	ProposalID          *uuid.UUID                 `json:"proposal_id,omitempty"`
	MigrationDraftIDs   []uuid.UUID                `json:"migration_draft_ids"`
}

type DictionaryValidationResult struct {
	Errors   []DictionaryValidationIssue `json:"errors"`
	Warnings []DictionaryValidationIssue `json:"warnings"`
}

type DictionaryValidationIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path"`
}

type ContextChangeProposalInput struct {
	DictionaryVersionID uuid.UUID
	ProposalType        string
	Title               string
	Summary             string
	Payload             map[string]any
	Status              string
}

type ContextChangeProposal struct {
	ID                  uuid.UUID      `json:"id"`
	DictionaryVersionID uuid.UUID      `json:"dictionary_version_id"`
	ProposalType        string         `json:"proposal_type"`
	Title               string         `json:"title"`
	Summary             string         `json:"summary"`
	Payload             map[string]any `json:"payload"`
	Status              string         `json:"status"`
	ReviewerID          *uuid.UUID     `json:"reviewer_id,omitempty"`
	ReviewReason        string         `json:"review_reason,omitempty"`
	ApplyResult         map[string]any `json:"apply_result"`
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

func (s *DictionaryService) ApplyContextProposal(ctx context.Context, proposalID uuid.UUID, reviewerID uuid.UUID, reviewerType string) (map[string]any, error) {
	if proposalID == uuid.Nil || reviewerID == uuid.Nil {
		return nil, fmt.Errorf("%w: proposal and reviewer are required", ErrValidation)
	}
	if !isHumanActor(reviewerType) {
		return nil, fmt.Errorf("%w: only human users can apply context proposals", ErrValidation)
	}
	if s.repo == nil {
		return nil, fmt.Errorf("%w: dictionary repository is not configured", ErrValidation)
	}
	proposal, err := s.repo.GetContextChangeProposal(ctx, proposalID)
	if err != nil {
		return nil, err
	}
	if proposal.Status == ProposalApplied {
		return proposal.ApplyResult, nil
	}
	if proposal.Status != DictionaryStatusApproved {
		return nil, fmt.Errorf("%w: context proposal must be approved before apply", ErrValidation)
	}
	rules, err := contextRulesFromProposal(proposal)
	if err != nil {
		return nil, err
	}
	activated, err := s.repo.ActivateContextRules(ctx, proposal, reviewerID, rules)
	if err != nil {
		return nil, err
	}
	result := map[string]any{
		"proposal_id":           proposal.ID.String(),
		"dictionary_version_id": proposal.DictionaryVersionID.String(),
		"status":                DictionaryStatusActive,
		"activated_rule_count":  len(activated),
		"activated_rule_ids":    contextRuleIDs(activated),
	}
	if _, err := s.repo.MarkContextChangeProposalApplied(ctx, proposal.ID, reviewerID, result); err != nil {
		return nil, err
	}
	return result, nil
}

func contextRulesFromProposal(proposal *ContextChangeProposal) ([]ContextRuleRecord, error) {
	rawRules, ok := proposal.Payload["rules"]
	if !ok {
		rawRules = proposal.Payload["context_rules"]
	}
	values, ok := rawRules.([]any)
	if !ok || len(values) == 0 {
		return nil, fmt.Errorf("%w: context proposal payload must include rules", ErrValidation)
	}
	rules := make([]ContextRuleRecord, 0, len(values))
	for _, raw := range values {
		item, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%w: context rule payload must be an object", ErrValidation)
		}
		rule := ContextRuleRecord{
			ID:                  uuidFromString(fmt.Sprint(item["id"])),
			DictionaryVersionID: proposal.DictionaryVersionID,
			ModuleKey:           fmt.Sprint(item["module_key"]),
			EntityKey:           fmt.Sprint(item["entity_key"]),
			FieldKey:            fmt.Sprint(item["field_key"]),
			RuleType:            fmt.Sprint(item["rule_type"]),
			Rule:                mapFromAny(item["rule"]),
			Status:              DictionaryStatusActive,
		}
		if rule.ID == uuid.Nil {
			rule.ID = uuid.New()
		}
		if rawDictionaryID := uuidFromString(fmt.Sprint(item["dictionary_version_id"])); rawDictionaryID != uuid.Nil {
			rule.DictionaryVersionID = rawDictionaryID
		}
		if rule.ModuleKey == "" || rule.ModuleKey == "<nil>" || rule.EntityKey == "" || rule.EntityKey == "<nil>" || rule.FieldKey == "" || rule.FieldKey == "<nil>" || rule.RuleType == "" || rule.RuleType == "<nil>" {
			return nil, fmt.Errorf("%w: context rule requires module_key, entity_key, field_key, and rule_type", ErrValidation)
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func contextRuleIDs(rules []ContextRuleRecord) []string {
	ids := make([]string, 0, len(rules))
	for _, rule := range rules {
		ids = append(ids, rule.ID.String())
	}
	return ids
}

func uuidFromString(value string) uuid.UUID {
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil
	}
	return id
}

func mapFromAny(value any) map[string]any {
	if result, ok := value.(map[string]any); ok {
		return result
	}
	return map[string]any{}
}
