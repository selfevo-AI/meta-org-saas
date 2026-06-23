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
		AttentionCore: []ContextItem{
			{
				EntityKey: "requirement",
				FieldKey:  "status",
				RecordID:  "REQ-1",
				Value:     "approved",
			},
		},
		RiskAndSignals: []ContextItem{
			{
				EntityKey:       "cost_ledger_entry",
				FieldKey:        "amount",
				RecordID:        "COST-1",
				ValidationState: ValidationFinanceConflict,
			},
		},
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
