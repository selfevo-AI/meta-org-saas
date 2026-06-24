package assistant

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
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
		VersionKey: "finance-v1",
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

func TestDictionaryServiceRejectsFieldWithoutEntity(t *testing.T) {
	svc := NewDictionaryService(nil, nil)
	model := DictionaryImportModel{
		VersionKey: "bad-v1",
		SourceType: ContextSourceJSON,
		ScopeLevel: ContextScopeModule,
		ModuleKey:  "project",
		Fields:     []ContextFieldInput{{EntityKey: "missing", FieldKey: "status"}},
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
		ModuleKey:  "project",
		Domains:    []ContextBusinessDomainInput{{ModuleKey: "project", Name: "Project", ScopeLevel: ContextScopeModule}},
		Entities:   []ContextEntityInput{{EntityKey: "project", ModuleKey: "project"}},
		Fields:     []ContextFieldInput{{EntityKey: "project", FieldKey: "priority", TableName: "projects", ColumnName: "priority"}},
		MigrationIntents: []ContextMigrationIntentInput{
			{IntentType: "add_column", EntityKey: "project", FieldKey: "priority", Reason: "prioritize work"},
		},
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

func TestDictionaryServiceAppliesApprovedContextProposal(t *testing.T) {
	proposalID := uuid.New()
	reviewerID := uuid.New()
	ruleID := uuid.New()
	repo := &fakeDictionaryRepository{
		contextProposal: &ContextChangeProposal{
			ID:                  proposalID,
			DictionaryVersionID: uuid.New(),
			ProposalType:        "dictionary_change",
			Status:              DictionaryStatusApproved,
			Payload: map[string]any{
				"rules": []any{
					map[string]any{
						"id":         ruleID.String(),
						"module_key": "erp",
						"entity_key": "requirement",
						"field_key":  "status",
						"rule_type":  "attention",
						"rule":       map[string]any{"base_weight": float64(9)},
					},
				},
			},
		},
	}
	svc := NewDictionaryService(repo, nil)

	result, err := svc.ApplyContextProposal(context.Background(), proposalID, reviewerID, "internal_human")
	if err != nil {
		t.Fatalf("ApplyContextProposal returned error: %v", err)
	}
	if result["status"] != DictionaryStatusActive {
		t.Fatalf("result = %#v, want active status", result)
	}
	if repo.appliedProposalID != proposalID {
		t.Fatalf("applied proposal = %s, want %s", repo.appliedProposalID, proposalID)
	}
	if len(repo.activatedRules) != 1 || repo.activatedRules[0].ID != ruleID {
		t.Fatalf("activated rules = %#v, want rule %s", repo.activatedRules, ruleID)
	}
}

func TestDictionaryServiceRejectsUnapprovedContextProposalApply(t *testing.T) {
	proposalID := uuid.New()
	svc := NewDictionaryService(&fakeDictionaryRepository{
		contextProposal: &ContextChangeProposal{ID: proposalID, Status: ProposalPending, Payload: map[string]any{}},
	}, nil)

	_, err := svc.ApplyContextProposal(context.Background(), proposalID, uuid.New(), "internal_human")
	if err == nil || !strings.Contains(err.Error(), "must be approved") {
		t.Fatalf("ApplyContextProposal error = %v, want approved proposal validation", err)
	}
}

type fakeDictionaryRepository struct {
	versionID         uuid.UUID
	proposals         []ContextChangeProposalInput
	migrationDrafts   []ContextMigrationDraftInput
	contextProposal   *ContextChangeProposal
	activatedRules    []ContextRuleRecord
	appliedProposalID uuid.UUID
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

func (f *fakeDictionaryRepository) GetContextChangeProposal(_ context.Context, id uuid.UUID) (*ContextChangeProposal, error) {
	if f.contextProposal == nil || f.contextProposal.ID != id {
		return nil, ErrNotFound
	}
	return f.contextProposal, nil
}

func (f *fakeDictionaryRepository) ActivateContextRules(_ context.Context, proposal *ContextChangeProposal, reviewerID uuid.UUID, rules []ContextRuleRecord) ([]ContextRuleRecord, error) {
	f.activatedRules = append([]ContextRuleRecord{}, rules...)
	for index := range f.activatedRules {
		f.activatedRules[index].Status = DictionaryStatusActive
	}
	return f.activatedRules, nil
}

func (f *fakeDictionaryRepository) MarkContextChangeProposalApplied(_ context.Context, id uuid.UUID, reviewerID uuid.UUID, result map[string]any) (*ContextChangeProposal, error) {
	f.appliedProposalID = id
	if f.contextProposal == nil {
		return nil, ErrNotFound
	}
	f.contextProposal.Status = ProposalApplied
	f.contextProposal.ReviewerID = &reviewerID
	f.contextProposal.ApplyResult = result
	return f.contextProposal, nil
}
