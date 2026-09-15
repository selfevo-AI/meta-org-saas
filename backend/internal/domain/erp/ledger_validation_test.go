package erp

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestLedgerDraftRejectsInvalidViewFields(t *testing.T) {
	service := NewService(newBusinessFakeRepository(), DefaultCatalog())
	for _, tc := range []struct {
		name  string
		table string
		data  map[string]any
	}{
		{"invalid date", "MJDT", map[string]any{"RefDate": "2026-02-30"}},
		{"numeric date", "MJDT", map[string]any{"RefDate": 12}},
		{"organization", "MACT", map[string]any{"OrganizationID": "not-a-uuid"}},
		{"department", "MPRC", map[string]any{"DepartmentID": false}},
		{"source", "MJDT", map[string]any{"SourceID": "bad"}},
		{"legacy identity", "MACT", map[string]any{"LegacyID": uuid.NewString()}},
		{"posting time", "MJDT", map[string]any{"PostedAt": "bad"}},
		{"metadata", "MPRC", map[string]any{"Metadata": []string{"invalid"}}},
		{"nested payload", "MJDT", map[string]any{"Payload": map[string]any{"RefDate": "bad"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := service.CreateRecord(context.Background(), tc.table, RecordInput{Key: "invalid", Data: tc.data})
			if !errors.Is(err, ErrValidation) {
				t.Fatalf("expected validation error, got %v", err)
			}
		})
	}
	for _, value := range []any{"bad", "", nil, false, -1, 0.0000001} {
		if err := validateLedgerPayload("JDT1", map[string]any{"Debit": value}); !errors.Is(err, ErrValidation) {
			t.Errorf("invalid debit %v accepted: %v", value, err)
		}
	}
	if err := validateLedgerPayload("MJDT", map[string]any{"SourceID": uuid.New(), "DepartmentID": (*uuid.UUID)(nil), "RefDate": "2026-02-28"}); err != nil {
		t.Fatalf("typed finance inputs rejected: %v", err)
	}
}

func TestVoidJournalCannotBePostedEditedOrDeleted(t *testing.T) {
	repo := newBusinessFakeRepository()
	repo.seed("MJDT", "void", map[string]any{"BtfStatus": "V", "Posted": "N"})
	service := NewService(repo, DefaultCatalog())
	ctx := context.Background()
	if _, err := service.RunAction(ctx, "MJDT", "void", "post", ActionInput{}); !errors.Is(err, ErrConflict) {
		t.Fatalf("void posting returned %v", err)
	}
	if _, err := service.UpdateRecord(ctx, "MJDT", "void", RecordInput{Data: map[string]any{"Memo": "changed"}}); !errors.Is(err, ErrConflict) {
		t.Fatalf("void update returned %v", err)
	}
	if err := service.DeleteRecord(ctx, "MJDT", "void"); !errors.Is(err, ErrConflict) {
		t.Fatalf("void deletion returned %v", err)
	}
}
