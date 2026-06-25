package finance

import (
	"context"
	"errors"
	"testing"
)

func TestPostGLJournalEntryRejectsUnbalancedLines(t *testing.T) {
	repo := newFakeGLRepository()
	svc := NewService(repo)

	entry, err := svc.CreateGLJournalEntry(context.Background(), CreateGLJournalEntryInput{
		ReferenceDate: "2026-06-24",
		Memo:          "unbalanced inventory posting",
		Currency:      "cny",
		Lines: []CreateGLJournalEntryLineInput{
			{AccountCode: "1405", Debit: 100},
			{AccountCode: "2202", Credit: 90},
		},
	})
	if err != nil {
		t.Fatalf("CreateGLJournalEntry returned error: %v", err)
	}

	_, err = svc.PostGLJournalEntry(context.Background(), entry.ID)
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("PostGLJournalEntry error = %v, want ErrValidation", err)
	}
}

func TestPostGLJournalEntryAndTrialBalance(t *testing.T) {
	repo := newFakeGLRepository()
	svc := NewService(repo)

	_, err := svc.CreateGLAccount(context.Background(), CreateGLAccountInput{
		AccountCode: "1122",
		Name:        "Accounts receivable",
		AccountType: "asset",
		Currency:    "cny",
	})
	if err != nil {
		t.Fatalf("CreateGLAccount returned error: %v", err)
	}
	_, err = svc.CreateGLAccount(context.Background(), CreateGLAccountInput{
		AccountCode: "6001",
		Name:        "Revenue",
		AccountType: "revenue",
		Currency:    "cny",
	})
	if err != nil {
		t.Fatalf("CreateGLAccount returned error: %v", err)
	}
	entry, err := svc.CreateGLJournalEntry(context.Background(), CreateGLJournalEntryInput{
		ReferenceDate: "2026-06-24",
		Memo:          "sales invoice posting",
		Currency:      "cny",
		Lines: []CreateGLJournalEntryLineInput{
			{AccountCode: "1122", Debit: 1200},
			{AccountCode: "6001", Credit: 1200},
		},
	})
	if err != nil {
		t.Fatalf("CreateGLJournalEntry returned error: %v", err)
	}

	posted, err := svc.PostGLJournalEntry(context.Background(), entry.ID)
	if err != nil {
		t.Fatalf("PostGLJournalEntry returned error: %v", err)
	}
	if posted.Status != "posted" {
		t.Fatalf("posted status = %q, want posted", posted.Status)
	}

	balance, err := svc.GetGLTrialBalance(context.Background(), GLTrialBalanceInput{})
	if err != nil {
		t.Fatalf("GetGLTrialBalance returned error: %v", err)
	}
	if balance.TotalDebit != 1200 || balance.TotalCredit != 1200 {
		t.Fatalf("trial balance totals = %v/%v, want 1200/1200", balance.TotalDebit, balance.TotalCredit)
	}
	if len(balance.Rows) != 2 {
		t.Fatalf("trial balance rows = %d, want 2", len(balance.Rows))
	}
}
