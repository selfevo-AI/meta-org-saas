package erp

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
)

func TestLineTotalsRejectsInvalidNumericTypes(t *testing.T) {
	for _, field := range []string{"Quantity", "Price", "TaxRate"} {
		for _, value := range []any{nil, true, []any{}, map[string]any{}, "invalid", math.NaN(), math.Inf(1)} {
			t.Run(fmt.Sprintf("%s/%T/%v", field, value, value), func(t *testing.T) {
				line := map[string]any{"Quantity": 1, "Price": 10, "TaxRate": 0}
				line[field] = value
				if _, _, err := lineTotals([]map[string]any{line}); !errors.Is(err, ErrValidation) {
					t.Fatalf("invalid %s accepted: %v", field, err)
				}
			})
		}
	}
	if net, tax, err := lineTotals([]map[string]any{{"Quantity": 1, "Price": 10}}); err != nil || net != 10 || tax != 0 {
		t.Fatalf("missing optional tax rate: net=%v tax=%v err=%v", net, tax, err)
	}
}

func seedCommerceReferences(repo *businessFakeRepository) {
	for _, code := range []string{"SUP", "S-1", "SUP-DEMO"} {
		repo.seed("MCRD", code, map[string]any{"CardType": "S"})
	}
	for _, code := range []string{"CUS", "C-1", "CUS-DEMO", "CASH"} {
		repo.seed("MCRD", code, map[string]any{"CardType": "C"})
	}
	for _, code := range []string{accountCash, accountReceivable, accountInventory, accountAccruedPurchases, accountPayable, accountInputTax, accountOutputTax, accountRevenue, accountCostOfSales, accountPurchaseExpense, accountInventoryAdjustment} {
		repo.seed("MACT", code, map[string]any{"Name": code, "Postable": "Y", "Active": "Y"})
	}
	for _, code := range []string{"I-1", "RAW-DEMO", "FG-DEMO"} {
		repo.seed("MITM", code, map[string]any{"ItemCode": code})
	}
	for _, code := range []string{"W-1", "RM", "FG"} {
		repo.seed("MWHS", code, map[string]any{"WhsCode": code})
	}
}

func (r *businessFakeRepository) QueryTrialBalance(_ context.Context, input TrialBalanceInput) (*TrialBalance, error) {
	balance := &TrialBalance{Currency: input.Currency, Rows: []TrialBalanceRow{}}
	for _, journal := range r.records["MJDT"] {
		if !documentFieldEquals(&journal, "BtfStatus", "P") || documentCurrency(&journal) != input.Currency {
			continue
		}
		balance.JournalCount++
		for _, line := range r.children[childBucket("MJDT", journal.Key, "JDT1")] {
			balance.TotalDebit += numericValue(line.Data, "Debit")
			balance.TotalCredit += numericValue(line.Data, "Credit")
		}
	}
	return balance, nil
}

func TestPurchaseToPayWithTaxAndPartialSettlement(t *testing.T) {
	repo := newBusinessFakeRepository()
	seedCommerceReferences(repo)
	svc := NewService(repo, DefaultCatalog())
	ctx := context.Background()
	repo.seed("MPDN", "GR-TAX", map[string]any{"WddStatus": "A", "CardCode": "SUP", "DocCur": "CNY"})
	repo.seedChild("MPDN", "GR-TAX", "PDN1", map[string]any{"ItemCode": "I-1", "WhsCode": "W-1", "Quantity": 2, "Price": 10, "TaxRate": 13})
	if _, err := svc.RunAction(ctx, "MPDN", "GR-TAX", "post", ActionInput{}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RunAction(ctx, "MPCH", "AP-GR-TAX", "post", ActionInput{}); err != nil {
		t.Fatal(err)
	}
	repo.seed("MVPM", "PAY", map[string]any{"DocTotal": 22.6, "DocCur": "CNY", "CardCode": "SUP"})
	for i, amount := range []float64{10, 12.6} {
		input := ActionInput{IdempotencyKey: fmt.Sprint(i), Data: map[string]any{"TargetKey": "AP-GR-TAX", "Amount": amount}}
		first, err := svc.RunAction(ctx, "MVPM", "PAY", "allocate", input)
		if err != nil {
			t.Fatal(err)
		}
		replay, err := svc.RunAction(ctx, "MVPM", "PAY", "allocate", input)
		if err != nil || replay.ExecutionID != first.ExecutionID {
			t.Fatalf("replay = %#v, %v", replay, err)
		}
	}
	if got := repo.records["MPCH"]["AP-GR-TAX"].Data["PaidToDate"]; got != 22.6 {
		t.Fatalf("paid = %v", got)
	}
	if got := repo.records["MVPM"]["PAY"].Data["OpenBal"]; got != 0.0 {
		t.Fatalf("open balance = %v", got)
	}
	if stock := repo.records["MITW"]["I-1|W-1"]; stock.Data["OnHand"] != 2.0 || stock.Data["InventoryValue"] != 20.0 {
		t.Fatalf("stock = %#v", stock)
	}
	if len(repo.records["MJDT"]) != 4 {
		t.Fatalf("journals = %d, want receipt, invoice and two payments", len(repo.records["MJDT"]))
	}
	if _, err := svc.RunAction(ctx, "MIGN", "IGN-GR-TAX", "post", ActionInput{}); err != nil {
		t.Fatal(err)
	}
	if repo.balance("I-1", "W-1") != 2 {
		t.Fatal("generated movement was posted twice")
	}
}

func TestPaymentRejectsInvalidAllocationsWithoutEffects(t *testing.T) {
	for _, test := range []struct {
		name                   string
		payment, invoice, data map[string]any
	}{
		{name: "exhausted payment", payment: map[string]any{"OpenBal": 0.0}},
		{name: "invoice overpayment", invoice: map[string]any{"PaidToDate": 95.0}},
		{name: "unposted invoice", invoice: map[string]any{"Posted": "N"}},
		{name: "wrong currency", payment: map[string]any{"DocCur": "USD"}},
		{name: "wrong partner", payment: map[string]any{"CardCode": "OTHER"}},
		{name: "wrong target type", data: map[string]any{"TargetTable": "MPOR"}},
		{name: "negative amount", data: map[string]any{"Amount": -10.0}},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := newBusinessFakeRepository()
			seedCommerceReferences(repo)
			payment := map[string]any{"DocTotal": 100.0, "DocCur": "CNY", "CardCode": "CUS"}
			invoice := map[string]any{"DocTotal": 100.0, "DocCur": "CNY", "CardCode": "CUS", "Posted": "Y"}
			data := map[string]any{"TargetKey": "INV", "Amount": 10.0}
			for k, v := range test.payment {
				payment[k] = v
			}
			for k, v := range test.invoice {
				invoice[k] = v
			}
			for k, v := range test.data {
				data[k] = v
			}
			repo.seed("MRCT", "PAY", payment)
			repo.seed("MINV", "INV", invoice)
			_, err := NewService(repo, DefaultCatalog()).RunAction(context.Background(), "MRCT", "PAY", "allocate", ActionInput{Data: data})
			if !errors.Is(err, ErrValidation) {
				t.Fatalf("error = %v", err)
			}
			if len(repo.records["MJDT"]) != 0 {
				t.Fatal("invalid allocation created a journal")
			}
		})
	}
}

func TestJournalRejectsUnbalancedAndInvalidLines(t *testing.T) {
	for _, test := range []struct {
		name          string
		debit, credit float64
		account       string
	}{
		{"unbalanced", 100, 90, accountCash}, {"negative", -1, -1, accountCash}, {"empty", 0, 0, accountCash}, {"unknown account", 100, 100, "MISSING"},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := newBusinessFakeRepository()
			seedCommerceReferences(repo)
			repo.seed("MJDT", "J", map[string]any{"BtfStatus": "O"})
			repo.seedChild("MJDT", "J", "JDT1", journalLine(test.account, test.debit, 0))
			repo.seedChild("MJDT", "J", "JDT1", journalLine(accountRevenue, 0, test.credit))
			_, err := NewService(repo, DefaultCatalog()).RunAction(context.Background(), "MJDT", "J", "post", ActionInput{})
			if !errors.Is(err, ErrValidation) {
				t.Fatalf("error = %v", err)
			}
			if repo.records["MJDT"]["J"].Data["BtfStatus"] != "O" {
				t.Fatal("invalid journal was posted")
			}
		})
	}
}

func TestERPRejectsDirectPostedEditsAndUnauthorizedActions(t *testing.T) {
	repo := newBusinessFakeRepository()
	repo.seed("MJDT", "J", map[string]any{"BtfStatus": "P"})
	svc := NewService(repo, DefaultCatalog())
	if _, err := svc.UpdateRecord(context.Background(), "MJDT", "J", RecordInput{Data: map[string]any{"Memo": "changed"}}); !errors.Is(err, ErrConflict) {
		t.Fatalf("posted edit error = %v", err)
	}
	if _, err := svc.CreateRecord(context.Background(), "MINV", RecordInput{Key: "I", Data: map[string]any{"Posted": "Y"}}); !errors.Is(err, ErrValidation) {
		t.Fatalf("forged posting error = %v", err)
	}
	ctx := context.WithValue(context.Background(), middleware.TenantContextKey, &middleware.TenantContext{Mode: "saas", AuthorityTier: "executor", EnabledModules: map[string]bool{"finance": true}})
	if _, err := svc.RunAction(ctx, "MJDT", "J", "post", ActionInput{}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("executor posting error = %v", err)
	}
}

type auditFailureRepository struct{ *businessFakeRepository }

func (r *auditFailureRepository) RunInTx(ctx context.Context, fn func(Repository) error) error {
	return r.businessFakeRepository.RunInTx(ctx, func(Repository) error { return fn(r) })
}

func (r *auditFailureRepository) CompleteActionExecution(ctx context.Context, id uuid.UUID, status string, payload map[string]any, failure *ActionFailure) (*ActionExecution, error) {
	if status == ActionExecutionCompleted {
		return nil, errors.New("audit unavailable")
	}
	return r.businessFakeRepository.CompleteActionExecution(ctx, id, status, payload, failure)
}

func TestAuditFailureRollsBackBusinessState(t *testing.T) {
	repo := &auditFailureRepository{newBusinessFakeRepository()}
	seedCommerceReferences(repo.businessFakeRepository)
	repo.seed("MPDN", "GR", map[string]any{"WddStatus": "A", "CardCode": "SUP"})
	repo.seedChild("MPDN", "GR", "PDN1", map[string]any{"ItemCode": "I-1", "WhsCode": "W-1", "Quantity": 2, "Price": 10})
	if _, err := NewService(repo, DefaultCatalog()).RunAction(context.Background(), "MPDN", "GR", "post", ActionInput{}); err == nil {
		t.Fatal("expected audit failure")
	}
	if repo.balance("I-1", "W-1") != 0 || len(repo.records["MPCH"]) != 0 || len(repo.records["MJDT"]) != 0 {
		t.Fatal("business state survived failed audit commit")
	}
}
