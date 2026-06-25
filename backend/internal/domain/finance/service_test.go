package finance

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/project"
)

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

func TestCreateExportBatchGeneratesStableIdempotencyKey(t *testing.T) {
	adapterID := uuid.New()
	repo := &fakeRepository{}
	svc := NewService(repo)

	batch, err := svc.CreateExportBatch(context.Background(), CreateExportBatchInput{
		AdapterID:   adapterID,
		PeriodStart: "2026-06-01",
		PeriodEnd:   "2026-06-30",
		Currency:    "cny",
	})
	if err != nil {
		t.Fatalf("CreateExportBatch returned error: %v", err)
	}

	want := BatchIdempotencyKey(adapterID.String(), "2026-06-01", "2026-06-30", "CNY")
	if repo.createdBatch.IdempotencyKey != want {
		t.Fatalf("idempotency key = %q, want %q", repo.createdBatch.IdempotencyKey, want)
	}
	if batch.Currency != "CNY" {
		t.Fatalf("currency = %q, want CNY", batch.Currency)
	}
}

func TestCreateExportBatchRejectsInvalidPeriod(t *testing.T) {
	svc := NewService(&fakeRepository{})
	_, err := svc.CreateExportBatch(context.Background(), CreateExportBatchInput{
		AdapterID:   uuid.New(),
		PeriodStart: "2026-06-30",
		PeriodEnd:   "2026-06-01",
	})
	if err == nil {
		t.Fatalf("CreateExportBatch accepted invalid period")
	}
}

func TestPostSettlementOrderCreatesReceivable(t *testing.T) {
	projectID := uuid.New()
	repo := &fakeRepository{}
	svc := NewService(repo)

	order, err := svc.CreateSettlementOrder(context.Background(), CreateSettlementOrderInput{
		ProjectID:    &projectID,
		CustomerName: "ACME",
		Currency:     "cny",
		Lines: []CreateSettlementLineInput{
			{
				LineType:    "delivery",
				Description: "Accepted milestone",
				Amount:      1200,
				TaxAmount:   72,
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateSettlementOrder returned error: %v", err)
	}

	receivable, err := svc.PostSettlementOrder(context.Background(), order.ID)
	if err != nil {
		t.Fatalf("PostSettlementOrder returned error: %v", err)
	}
	if receivable.SettlementOrderID == nil || *receivable.SettlementOrderID != order.ID {
		t.Fatalf("receivable settlement id = %v, want %s", receivable.SettlementOrderID, order.ID)
	}
	if receivable.Amount != 1272 {
		t.Fatalf("receivable amount = %v, want 1272", receivable.Amount)
	}
	if repo.postedSettlementID != order.ID {
		t.Fatalf("posted settlement id = %s, want %s", repo.postedSettlementID, order.ID)
	}
}

func TestAllocateReceiptMarksReceivablePaid(t *testing.T) {
	receivableID := uuid.New()
	receiptID := uuid.New()
	repo := &fakeRepository{receivableForAllocation: &Receivable{
		ID:       receivableID,
		Amount:   100,
		Currency: "CNY",
		Status:   "unpaid",
	}}
	svc := NewService(repo)

	allocation, err := svc.AllocateReceipt(context.Background(), receiptID, AllocateReceiptInput{
		ReceivableID: receivableID,
		Amount:       100,
		Currency:     "cny",
	})
	if err != nil {
		t.Fatalf("AllocateReceipt returned error: %v", err)
	}
	if allocation.Currency != "CNY" {
		t.Fatalf("allocation currency = %q, want CNY", allocation.Currency)
	}
	if repo.receivableStatus != "paid" {
		t.Fatalf("receivable status = %q, want paid", repo.receivableStatus)
	}
}

func TestAllocateReceiptKeepsPartiallyReceivedWhenRepositoryReturnsUpdatedAmount(t *testing.T) {
	receivableID := uuid.New()
	receiptID := uuid.New()
	repo := &fakeRepository{
		receivableForAllocation: &Receivable{
			ID:             receivableID,
			Amount:         100,
			ReceivedAmount: 40,
			Currency:       "CNY",
			Status:         "partially_received",
		},
		mutateReceivableOnReceiptAllocation: true,
	}
	svc := NewService(repo)

	_, err := svc.AllocateReceipt(context.Background(), receiptID, AllocateReceiptInput{
		ReceivableID: receivableID,
		Amount:       30,
		Currency:     "CNY",
	})
	if err != nil {
		t.Fatalf("AllocateReceipt returned error: %v", err)
	}
	if repo.receivableStatus != "partially_received" {
		t.Fatalf("receivable status = %q, want partially_received", repo.receivableStatus)
	}
}

func TestVoidPayableRejectsAllocatedPayable(t *testing.T) {
	payableID := uuid.New()
	repo := &fakeRepository{payableForVoid: &Payable{
		ID:         payableID,
		Amount:     100,
		PaidAmount: 10,
		Status:     "partially_paid",
	}}
	svc := NewService(repo)

	_, err := svc.VoidPayable(context.Background(), payableID, "duplicate")
	if err == nil {
		t.Fatalf("VoidPayable accepted allocated payable")
	}
}

type fakeRepository struct {
	createdBatch                        CreateExportBatchInput
	settlementOrder                     *SettlementOrder
	postedSettlementID                  uuid.UUID
	receivableForAllocation             *Receivable
	receivableStatus                    string
	mutateReceivableOnReceiptAllocation bool
	payableForVoid                      *Payable
	glAccounts                          map[string]GLAccount
	glCostCenters                       map[string]GLCostCenter
	glJournalEntries                    map[uuid.UUID]GLJournalEntry
}

func (f *fakeRepository) CreateAdapter(context.Context, CreateAdapterInput) (*FinanceAdapter, error) {
	return &FinanceAdapter{}, nil
}

func (f *fakeRepository) ListAdapters(context.Context, int) ([]FinanceAdapter, error) {
	return []FinanceAdapter{}, nil
}

func (f *fakeRepository) UpdateAdapter(context.Context, uuid.UUID, UpdateAdapterInput) (*FinanceAdapter, error) {
	return &FinanceAdapter{}, nil
}

func (f *fakeRepository) GetAdapterSecret(context.Context, uuid.UUID) (AdapterSecret, error) {
	return AdapterSecret{}, nil
}

func (f *fakeRepository) CreateExportBatch(_ context.Context, input CreateExportBatchInput) (*ExportBatch, error) {
	f.createdBatch = input
	return &ExportBatch{
		ID:             uuid.New(),
		AdapterID:      input.AdapterID,
		PeriodStart:    input.periodStartTime,
		PeriodEnd:      input.periodEndTime,
		Status:         "ready",
		Currency:       input.Currency,
		IdempotencyKey: input.IdempotencyKey,
		Lines:          []ExportLine{},
	}, nil
}

func (f *fakeRepository) ListExportBatches(context.Context, int) ([]ExportBatch, error) {
	return []ExportBatch{}, nil
}

func (f *fakeRepository) GetExportBatch(context.Context, uuid.UUID) (*ExportBatch, error) {
	return &ExportBatch{}, nil
}

func (f *fakeRepository) UpdateExportBatchStatus(context.Context, uuid.UUID, UpdateExportBatchStatusInput) (*ExportBatch, error) {
	return &ExportBatch{}, nil
}

func (f *fakeRepository) RecordWebhookEvent(context.Context, RecordWebhookEventInput) (*WebhookEvent, error) {
	return &WebhookEvent{}, nil
}

func (f *fakeRepository) UpdateExportLineStatus(context.Context, uuid.UUID, UpdateExportLineStatusInput) (*ExportLine, error) {
	return &ExportLine{}, nil
}

func (f *fakeRepository) LinkProjectCostEntry(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}

func (f *fakeRepository) ListReconciliation(context.Context, int) ([]ReconciliationItem, error) {
	return []ReconciliationItem{}, nil
}

func (f *fakeRepository) CreateImportBatch(context.Context, *uuid.UUID, string, string, int, map[string]any) (*ImportBatch, error) {
	return &ImportBatch{}, nil
}

func (f *fakeRepository) CompleteImportBatch(context.Context, uuid.UUID, int, int) (*ImportBatch, error) {
	return &ImportBatch{}, nil
}

func (f *fakeRepository) ListImportBatches(context.Context, int) ([]ImportBatch, error) {
	return []ImportBatch{}, nil
}

func (f *fakeRepository) ListImportRecords(context.Context, int) ([]ImportRecord, error) {
	return []ImportRecord{}, nil
}

func (f *fakeRepository) CreateImportedExpense(context.Context, uuid.UUID, uuid.UUID, map[string]any, FinanceExpenseInput, time.Time, financeExpenseDates) (*ImportRecord, error) {
	return &ImportRecord{}, nil
}

func (f *fakeRepository) CreatePayable(context.Context, CreatePayableInput, financeExpenseDates) (*Payable, error) {
	return &Payable{}, nil
}

func (f *fakeRepository) ListPayables(context.Context, int) ([]Payable, error) {
	return []Payable{}, nil
}

func (f *fakeRepository) CreatePayment(context.Context, CreatePaymentInput, *time.Time) (*Payment, error) {
	return &Payment{}, nil
}

func (f *fakeRepository) ListPayments(context.Context, int) ([]Payment, error) {
	return []Payment{}, nil
}

func (f *fakeRepository) AllocatePayment(context.Context, uuid.UUID, AllocatePaymentInput) (*PaymentAllocation, error) {
	return &PaymentAllocation{}, nil
}

func (f *fakeRepository) CreateSettlementOrder(_ context.Context, input CreateSettlementOrderInput) (*SettlementOrder, error) {
	lineID := uuid.New()
	order := &SettlementOrder{
		ID:           uuid.New(),
		ProjectID:    input.ProjectID,
		CustomerName: input.CustomerName,
		Currency:     input.Currency,
		Subtotal:     1200,
		TaxAmount:    72,
		TotalAmount:  1272,
		Status:       "draft",
		Lines: []SettlementLine{{
			ID:          lineID,
			LineType:    "delivery",
			Amount:      1200,
			TaxAmount:   72,
			TotalAmount: 1272,
		}},
	}
	f.settlementOrder = order
	return order, nil
}

func (f *fakeRepository) ListSettlementOrders(context.Context, int) ([]SettlementOrder, error) {
	if f.settlementOrder == nil {
		return []SettlementOrder{}, nil
	}
	return []SettlementOrder{*f.settlementOrder}, nil
}

func (f *fakeRepository) GetSettlementOrder(context.Context, uuid.UUID) (*SettlementOrder, error) {
	return f.settlementOrder, nil
}

func (f *fakeRepository) UpdateSettlementOrder(context.Context, uuid.UUID, UpdateSettlementOrderInput) (*SettlementOrder, error) {
	return f.settlementOrder, nil
}

func (f *fakeRepository) VoidSettlementOrder(context.Context, uuid.UUID, string) (*SettlementOrder, error) {
	return f.settlementOrder, nil
}

func (f *fakeRepository) PostSettlementOrder(_ context.Context, id uuid.UUID) (*Receivable, error) {
	f.postedSettlementID = id
	return &Receivable{
		ID:                uuid.New(),
		SettlementOrderID: &id,
		Amount:            1272,
		Currency:          "CNY",
		Status:            "unpaid",
	}, nil
}

func (f *fakeRepository) CreateReceivable(context.Context, CreateReceivableInput, financeExpenseDates) (*Receivable, error) {
	return &Receivable{}, nil
}

func (f *fakeRepository) ListReceivables(context.Context, int) ([]Receivable, error) {
	return []Receivable{}, nil
}

func (f *fakeRepository) GetReceivable(_ context.Context, id uuid.UUID) (*Receivable, error) {
	if f.receivableForAllocation != nil {
		return f.receivableForAllocation, nil
	}
	return &Receivable{ID: id, Amount: 100, Currency: "CNY", Status: "unpaid"}, nil
}

func (f *fakeRepository) UpdateReceivableStatus(_ context.Context, id uuid.UUID, status string) (*Receivable, error) {
	f.receivableStatus = status
	return &Receivable{ID: id, Amount: 100, Currency: "CNY", Status: status}, nil
}

func (f *fakeRepository) UpdateReceivable(context.Context, uuid.UUID, UpdateReceivableInput, financeExpenseDates) (*Receivable, error) {
	return &Receivable{}, nil
}

func (f *fakeRepository) VoidReceivable(context.Context, uuid.UUID, string) (*Receivable, error) {
	return &Receivable{}, nil
}

func (f *fakeRepository) CreateReceipt(context.Context, CreateReceiptInput, *time.Time) (*Receipt, error) {
	return &Receipt{}, nil
}

func (f *fakeRepository) ListReceipts(context.Context, int) ([]Receipt, error) {
	return []Receipt{}, nil
}

func (f *fakeRepository) AllocateReceipt(_ context.Context, _ uuid.UUID, input AllocateReceiptInput) (*ReceiptAllocation, error) {
	if f.mutateReceivableOnReceiptAllocation && f.receivableForAllocation != nil {
		f.receivableForAllocation.ReceivedAmount += input.Amount
	}
	return &ReceiptAllocation{ID: uuid.New(), Amount: input.Amount, Currency: "CNY"}, nil
}

func (f *fakeRepository) GetPayable(context.Context, uuid.UUID) (*Payable, error) {
	if f.payableForVoid != nil {
		return f.payableForVoid, nil
	}
	return &Payable{}, nil
}

func (f *fakeRepository) UpdatePayable(context.Context, uuid.UUID, UpdatePayableInput, financeExpenseDates) (*Payable, error) {
	return &Payable{}, nil
}

func (f *fakeRepository) VoidPayable(context.Context, uuid.UUID, string) (*Payable, error) {
	return &Payable{}, nil
}

func (f *fakeRepository) GetPayment(context.Context, uuid.UUID) (*Payment, error) {
	return &Payment{}, nil
}

func (f *fakeRepository) UpdatePayment(context.Context, uuid.UUID, UpdatePaymentInput, *time.Time) (*Payment, error) {
	return &Payment{}, nil
}

func (f *fakeRepository) VoidPayment(context.Context, uuid.UUID, string) (*Payment, error) {
	return &Payment{}, nil
}

func newFakeGLRepository() *fakeRepository {
	return &fakeRepository{
		glAccounts:       map[string]GLAccount{},
		glCostCenters:    map[string]GLCostCenter{},
		glJournalEntries: map[uuid.UUID]GLJournalEntry{},
	}
}

func (f *fakeRepository) ensureGLStorage() {
	if f.glAccounts == nil {
		f.glAccounts = map[string]GLAccount{}
	}
	if f.glCostCenters == nil {
		f.glCostCenters = map[string]GLCostCenter{}
	}
	if f.glJournalEntries == nil {
		f.glJournalEntries = map[uuid.UUID]GLJournalEntry{}
	}
}

func (f *fakeRepository) CreateGLAccount(_ context.Context, input CreateGLAccountInput) (*GLAccount, error) {
	f.ensureGLStorage()
	now := time.Now()
	account := GLAccount{
		ID:                uuid.New(),
		MasterKey:         input.AccountCode,
		AccountCode:       input.AccountCode,
		Name:              input.Name,
		AccountType:       input.AccountType,
		Currency:          input.Currency,
		ParentAccountCode: input.ParentAccountCode,
		Postable:          boolFromPtr(input.Postable, true),
		Active:            boolFromPtr(input.Active, true),
		OrganizationID:    input.OrganizationID,
		DepartmentID:      input.DepartmentID,
		Metadata:          input.Metadata,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	f.glAccounts[account.AccountCode] = account
	return &account, nil
}

func (f *fakeRepository) ListGLAccounts(context.Context, int) ([]GLAccount, error) {
	f.ensureGLStorage()
	items := make([]GLAccount, 0, len(f.glAccounts))
	for _, item := range f.glAccounts {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].AccountCode < items[j].AccountCode
	})
	return items, nil
}

func (f *fakeRepository) CreateGLCostCenter(_ context.Context, input CreateGLCostCenterInput) (*GLCostCenter, error) {
	f.ensureGLStorage()
	now := time.Now()
	center := GLCostCenter{
		ID:             uuid.New(),
		MasterKey:      input.CostCenterCode,
		CostCenterCode: input.CostCenterCode,
		Name:           input.Name,
		Active:         boolFromPtr(input.Active, true),
		OrganizationID: input.OrganizationID,
		DepartmentID:   input.DepartmentID,
		Metadata:       input.Metadata,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	f.glCostCenters[center.CostCenterCode] = center
	return &center, nil
}

func (f *fakeRepository) ListGLCostCenters(context.Context, int) ([]GLCostCenter, error) {
	f.ensureGLStorage()
	items := make([]GLCostCenter, 0, len(f.glCostCenters))
	for _, item := range f.glCostCenters {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].CostCenterCode < items[j].CostCenterCode
	})
	return items, nil
}

func (f *fakeRepository) CreateGLJournalEntry(_ context.Context, input CreateGLJournalEntryInput, referenceDate time.Time) (*GLJournalEntry, error) {
	f.ensureGLStorage()
	now := time.Now()
	entry := GLJournalEntry{
		ID:             uuid.New(),
		MasterKey:      input.EntryNumber,
		EntryNumber:    input.EntryNumber,
		ReferenceDate:  referenceDate,
		Memo:           input.Memo,
		Status:         "draft",
		Currency:       input.Currency,
		SourceType:     input.SourceType,
		SourceID:       input.SourceID,
		OrganizationID: input.OrganizationID,
		DepartmentID:   input.DepartmentID,
		Metadata:       input.Metadata,
		Lines:          make([]GLJournalEntryLine, 0, len(input.Lines)),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if entry.MasterKey == "" {
		entry.MasterKey = entry.ID.String()
	}
	for index, line := range input.Lines {
		accountName := line.AccountName
		if accountName == "" {
			if account, ok := f.glAccounts[line.AccountCode]; ok {
				accountName = account.Name
			}
		}
		entry.Lines = append(entry.Lines, GLJournalEntryLine{
			ID:             uuid.New(),
			EntryID:        entry.ID,
			LineNum:        index + 1,
			AccountCode:    line.AccountCode,
			AccountName:    accountName,
			CostCenterCode: line.CostCenterCode,
			Debit:          line.Debit,
			Credit:         line.Credit,
			Description:    line.Description,
			Metadata:       line.Metadata,
			CreatedAt:      now,
		})
	}
	f.glJournalEntries[entry.ID] = entry
	return &entry, nil
}

func (f *fakeRepository) ListGLJournalEntries(context.Context, int) ([]GLJournalEntry, error) {
	f.ensureGLStorage()
	items := make([]GLJournalEntry, 0, len(f.glJournalEntries))
	for _, item := range f.glJournalEntries {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].ReferenceDate.After(items[j].ReferenceDate)
	})
	return items, nil
}

func (f *fakeRepository) GetGLJournalEntry(_ context.Context, id uuid.UUID) (*GLJournalEntry, error) {
	f.ensureGLStorage()
	entry, ok := f.glJournalEntries[id]
	if !ok {
		return nil, ErrNotFound
	}
	return &entry, nil
}

func (f *fakeRepository) PostGLJournalEntry(_ context.Context, id uuid.UUID) (*GLJournalEntry, error) {
	f.ensureGLStorage()
	entry, ok := f.glJournalEntries[id]
	if !ok {
		return nil, ErrNotFound
	}
	now := time.Now()
	entry.Status = "posted"
	entry.PostedAt = &now
	entry.UpdatedAt = now
	f.glJournalEntries[id] = entry
	return &entry, nil
}

func (f *fakeRepository) GetGLTrialBalance(_ context.Context, input GLTrialBalanceInput) (*GLTrialBalance, error) {
	f.ensureGLStorage()
	rowsByAccount := map[string]*GLTrialBalanceRow{}
	for _, entry := range f.glJournalEntries {
		if entry.Status != "posted" {
			continue
		}
		if input.OrganizationID != nil && (entry.OrganizationID == nil || *entry.OrganizationID != *input.OrganizationID) {
			continue
		}
		if input.periodStartTime != nil && entry.ReferenceDate.Before(*input.periodStartTime) {
			continue
		}
		if input.periodEndTime != nil && entry.ReferenceDate.After(*input.periodEndTime) {
			continue
		}
		if input.Currency != "" && entry.Currency != input.Currency {
			continue
		}
		for _, line := range entry.Lines {
			row := rowsByAccount[line.AccountCode]
			if row == nil {
				row = &GLTrialBalanceRow{AccountCode: line.AccountCode, AccountName: line.AccountName}
				if row.AccountName == "" {
					if account, ok := f.glAccounts[line.AccountCode]; ok {
						row.AccountName = account.Name
					}
				}
				rowsByAccount[line.AccountCode] = row
			}
			row.Debit += line.Debit
			row.Credit += line.Credit
			row.NetAmount = row.Debit - row.Credit
		}
	}
	accountCodes := make([]string, 0, len(rowsByAccount))
	for accountCode := range rowsByAccount {
		accountCodes = append(accountCodes, accountCode)
	}
	sort.Strings(accountCodes)
	balance := &GLTrialBalance{Rows: []GLTrialBalanceRow{}, Currency: input.Currency}
	for _, accountCode := range accountCodes {
		row := *rowsByAccount[accountCode]
		balance.TotalDebit += row.Debit
		balance.TotalCredit += row.Credit
		balance.Rows = append(balance.Rows, row)
	}
	return balance, nil
}

type fakeCostPoster struct{}

func (fakeCostPoster) CreateCostEntryFromAIUsage(context.Context, uuid.UUID, project.CreateCostEntryInput) (*project.CostEntry, error) {
	return &project.CostEntry{}, nil
}
