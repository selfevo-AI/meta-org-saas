package finance

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (s *Service) CreateGLAccount(ctx context.Context, input CreateGLAccountInput) (*GLAccount, error) {
	normalizeGLAccountInput(&input)
	if input.AccountCode == "" || input.Name == "" {
		return nil, fmt.Errorf("%w: account_code and name are required", ErrValidation)
	}
	if input.OrganizationID == nil {
		input.OrganizationID = currentTenantOrganizationID(ctx)
	}
	if err := ensureTenantOrganization(ctx, input.OrganizationID); err != nil {
		return nil, err
	}
	return s.repo.CreateGLAccount(ctx, input)
}

func (s *Service) ListGLAccounts(ctx context.Context, limit int) ([]GLAccount, error) {
	items, err := s.repo.ListGLAccounts(ctx, normalizeLimit(limit))
	if items == nil {
		items = []GLAccount{}
	}
	return items, err
}

func (s *Service) CreateGLCostCenter(ctx context.Context, input CreateGLCostCenterInput) (*GLCostCenter, error) {
	normalizeGLCostCenterInput(&input)
	if input.CostCenterCode == "" || input.Name == "" {
		return nil, fmt.Errorf("%w: cost_center_code and name are required", ErrValidation)
	}
	if input.OrganizationID == nil {
		input.OrganizationID = currentTenantOrganizationID(ctx)
	}
	if err := ensureTenantOrganization(ctx, input.OrganizationID); err != nil {
		return nil, err
	}
	return s.repo.CreateGLCostCenter(ctx, input)
}

func (s *Service) ListGLCostCenters(ctx context.Context, limit int) ([]GLCostCenter, error) {
	items, err := s.repo.ListGLCostCenters(ctx, normalizeLimit(limit))
	if items == nil {
		items = []GLCostCenter{}
	}
	return items, err
}

func (s *Service) CreateGLJournalEntry(ctx context.Context, input CreateGLJournalEntryInput) (*GLJournalEntry, error) {
	normalizeGLJournalEntryInput(&input)
	if len(input.Lines) < 2 {
		return nil, fmt.Errorf("%w: journal entry requires at least two lines", ErrValidation)
	}
	referenceDate, err := parseGLRequiredDate(input.ReferenceDate)
	if err != nil {
		return nil, err
	}
	if input.OrganizationID == nil {
		input.OrganizationID = currentTenantOrganizationID(ctx)
	}
	if err := ensureTenantOrganization(ctx, input.OrganizationID); err != nil {
		return nil, err
	}
	for index := range input.Lines {
		normalizeGLJournalEntryLineInput(&input.Lines[index])
		if err := validateGLJournalEntryLine(input.Lines[index]); err != nil {
			return nil, err
		}
	}
	return s.repo.CreateGLJournalEntry(ctx, input, referenceDate)
}

func (s *Service) ListGLJournalEntries(ctx context.Context, limit int) ([]GLJournalEntry, error) {
	items, err := s.repo.ListGLJournalEntries(ctx, normalizeLimit(limit))
	if items == nil {
		items = []GLJournalEntry{}
	}
	return items, err
}

func (s *Service) GetGLJournalEntry(ctx context.Context, id uuid.UUID) (*GLJournalEntry, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("%w: journal entry id is required", ErrValidation)
	}
	return s.repo.GetGLJournalEntry(ctx, id)
}

func (s *Service) PostGLJournalEntry(ctx context.Context, id uuid.UUID) (*GLJournalEntry, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("%w: journal entry id is required", ErrValidation)
	}
	entry, err := s.repo.GetGLJournalEntry(ctx, id)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, ErrNotFound
	}
	if entry.Status == "posted" {
		return entry, nil
	}
	if err := validateGLJournalEntryBalance(*entry); err != nil {
		return nil, err
	}
	return s.repo.PostGLJournalEntry(ctx, id)
}

func (s *Service) GetGLTrialBalance(ctx context.Context, input GLTrialBalanceInput) (*GLTrialBalance, error) {
	if orgID := currentTenantOrganizationID(ctx); orgID != nil {
		if input.OrganizationID != nil && *input.OrganizationID != *orgID {
			return nil, fmt.Errorf("%w: trial balance organization must match current organization", ErrForbidden)
		}
		input.OrganizationID = orgID
	}
	input.Currency = defaultCurrency(input.Currency)
	var err error
	input.periodStartTime, err = parseOptionalDate(input.PeriodStart)
	if err != nil {
		return nil, err
	}
	input.periodEndTime, err = parseOptionalDate(input.PeriodEnd)
	if err != nil {
		return nil, err
	}
	if input.periodStartTime != nil && input.periodEndTime != nil && input.periodEndTime.Before(*input.periodStartTime) {
		return nil, fmt.Errorf("%w: period_end must be on or after period_start", ErrValidation)
	}
	balance, err := s.repo.GetGLTrialBalance(ctx, input)
	if balance != nil && balance.Rows == nil {
		balance.Rows = []GLTrialBalanceRow{}
	}
	return balance, err
}

func normalizeGLAccountInput(input *CreateGLAccountInput) {
	input.AccountCode = strings.TrimSpace(input.AccountCode)
	input.Name = strings.TrimSpace(input.Name)
	input.AccountType = strings.ToLower(strings.TrimSpace(input.AccountType))
	if input.AccountType == "" {
		input.AccountType = "expense"
	}
	input.Currency = defaultCurrency(input.Currency)
	input.ParentAccountCode = strings.TrimSpace(input.ParentAccountCode)
	if input.Postable == nil {
		value := true
		input.Postable = &value
	}
	if input.Active == nil {
		value := true
		input.Active = &value
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
}

func normalizeGLCostCenterInput(input *CreateGLCostCenterInput) {
	input.CostCenterCode = strings.TrimSpace(input.CostCenterCode)
	input.Name = strings.TrimSpace(input.Name)
	if input.Active == nil {
		value := true
		input.Active = &value
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
}

func normalizeGLJournalEntryInput(input *CreateGLJournalEntryInput) {
	input.EntryNumber = strings.TrimSpace(input.EntryNumber)
	input.ReferenceDate = strings.TrimSpace(input.ReferenceDate)
	input.Memo = strings.TrimSpace(input.Memo)
	input.Currency = defaultCurrency(input.Currency)
	input.SourceType = strings.TrimSpace(input.SourceType)
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
}

func normalizeGLJournalEntryLineInput(input *CreateGLJournalEntryLineInput) {
	input.AccountCode = strings.TrimSpace(input.AccountCode)
	input.AccountName = strings.TrimSpace(input.AccountName)
	input.CostCenterCode = strings.TrimSpace(input.CostCenterCode)
	input.Description = strings.TrimSpace(input.Description)
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
}

func parseGLRequiredDate(value string) (time.Time, error) {
	parsed, err := parseOptionalDate(value)
	if err != nil {
		return time.Time{}, err
	}
	if parsed == nil {
		return time.Time{}, fmt.Errorf("%w: reference_date is required", ErrValidation)
	}
	return *parsed, nil
}

func validateGLJournalEntryLine(line CreateGLJournalEntryLineInput) error {
	if line.AccountCode == "" {
		return fmt.Errorf("%w: journal line account_code is required", ErrValidation)
	}
	if line.Debit < 0 || line.Credit < 0 {
		return fmt.Errorf("%w: journal line debit and credit cannot be negative", ErrValidation)
	}
	if line.Debit == 0 && line.Credit == 0 {
		return fmt.Errorf("%w: journal line requires debit or credit", ErrValidation)
	}
	if line.Debit > 0 && line.Credit > 0 {
		return fmt.Errorf("%w: journal line cannot contain both debit and credit", ErrValidation)
	}
	return nil
}

func validateGLJournalEntryBalance(entry GLJournalEntry) error {
	if len(entry.Lines) < 2 {
		return fmt.Errorf("%w: journal entry requires at least two lines", ErrValidation)
	}
	debit, credit := glEntryTotals(entry.Lines)
	if math.Abs(debit-credit) > 0.000001 {
		return fmt.Errorf("%w: journal entry is not balanced", ErrValidation)
	}
	return nil
}

func glEntryTotals(lines []GLJournalEntryLine) (float64, float64) {
	debit := 0.0
	credit := 0.0
	for _, line := range lines {
		debit += line.Debit
		credit += line.Credit
	}
	return debit, credit
}
