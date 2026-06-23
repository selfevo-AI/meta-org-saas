package costing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
)

var (
	ErrValidation = errors.New("validation")
	ErrNotFound   = errors.New("not found")
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) UpsertCurrency(ctx context.Context, input CreateCurrencyInput) (*Currency, error) {
	input.Code = normalizeCurrency(input.Code)
	if input.Code == "" {
		return nil, fmt.Errorf("%w: code is required", ErrValidation)
	}
	if input.CurrencyType == "" {
		input.CurrencyType = CurrencyFiat
	}
	if input.PrecisionDigits <= 0 {
		if input.CurrencyType == CurrencyVirtual {
			input.PrecisionDigits = 8
		} else {
			input.PrecisionDigits = 2
		}
	}
	return s.repo.UpsertCurrency(ctx, input)
}

func (s *Service) ListCurrencies(ctx context.Context) ([]Currency, error) {
	return s.repo.ListCurrencies(ctx)
}

func (s *Service) VoidCurrency(ctx context.Context, code string) (*Currency, error) {
	code = normalizeCurrency(code)
	if code == "" {
		return nil, fmt.Errorf("%w: code is required", ErrValidation)
	}
	return s.repo.VoidCurrency(ctx, code)
}

func (s *Service) CreateExchangeRate(ctx context.Context, input CreateExchangeRateInput) (*ExchangeRateVersion, error) {
	input.FromCurrency = normalizeCurrency(input.FromCurrency)
	input.ToCurrency = normalizeCurrency(input.ToCurrency)
	if input.FromCurrency == "" || input.ToCurrency == "" {
		return nil, fmt.Errorf("%w: from_currency and to_currency are required", ErrValidation)
	}
	if input.FromCurrency == input.ToCurrency {
		return nil, fmt.Errorf("%w: from_currency and to_currency must differ", ErrValidation)
	}
	if input.Rate <= 0 {
		return nil, fmt.Errorf("%w: rate must be positive", ErrValidation)
	}
	if input.Source == "" {
		input.Source = RateSourceManual
	}
	return s.repo.CreateExchangeRate(ctx, input)
}

func (s *Service) ListExchangeRates(ctx context.Context, limit int) ([]ExchangeRateVersion, error) {
	return s.repo.ListExchangeRates(ctx, limit)
}

func (s *Service) UpdateExchangeRate(ctx context.Context, id string, input UpdateExchangeRateInput) (*ExchangeRateVersion, error) {
	rateID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid rate id", ErrValidation)
	}
	createInput := CreateExchangeRateInput(input)
	createInput.FromCurrency = normalizeCurrency(createInput.FromCurrency)
	createInput.ToCurrency = normalizeCurrency(createInput.ToCurrency)
	if createInput.Rate <= 0 {
		return nil, fmt.Errorf("%w: rate must be positive", ErrValidation)
	}
	return s.repo.UpdateExchangeRate(ctx, rateID, createInput)
}

func (s *Service) VoidExchangeRate(ctx context.Context, id string) (*ExchangeRateVersion, error) {
	rateID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid rate id", ErrValidation)
	}
	return s.repo.VoidExchangeRate(ctx, rateID)
}

func (s *Service) Convert(ctx context.Context, input ConvertInput) (*ConversionResult, error) {
	input.FromCurrency = normalizeCurrency(input.FromCurrency)
	input.ToCurrency = normalizeCurrency(input.ToCurrency)
	if input.FromCurrency == "" {
		input.FromCurrency = BaseCurrency
	}
	if input.ToCurrency == "" {
		input.ToCurrency = BaseCurrency
	}
	at := time.Now().UTC()
	if input.At != nil {
		at = *input.At
	}
	result, err := s.convert(ctx, input.Amount, input.FromCurrency, input.ToCurrency, at)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *Service) CreateRateCard(ctx context.Context, input CreateRateCardInput) (*CostRateCard, error) {
	input.SubjectType = strings.TrimSpace(input.SubjectType)
	if input.SubjectType == "" {
		return nil, fmt.Errorf("%w: subject_type is required", ErrValidation)
	}
	if input.RateType == "" {
		input.RateType = "fixed"
	}
	if input.Status == "" {
		input.Status = "active"
	}
	if input.ScopeType == "" {
		if orgID := currentTenantOrganizationID(ctx); orgID != nil {
			input.ScopeType = "organization"
			input.ScopeID = orgID
		}
	}
	if err := ensureScopeAccess(ctx, input.ScopeType, input.ScopeID); err != nil {
		return nil, err
	}
	input.Currency = defaultCurrency(input.Currency)
	conversion, err := s.convert(ctx, input.Amount, input.Currency, BaseCurrency, effectiveTime(input.EffectiveFrom))
	if err != nil {
		return nil, err
	}
	return s.repo.CreateRateCard(ctx, input, conversion)
}

func (s *Service) ListRateCards(ctx context.Context, limit int) ([]CostRateCard, error) {
	if orgID := currentTenantOrganizationID(ctx); orgID != nil {
		return s.repo.ListRateCardsByScope(ctx, "organization", *orgID, limit)
	}
	return s.repo.ListRateCards(ctx, limit)
}

func (s *Service) UpdateRateCard(ctx context.Context, id string, input UpdateRateCardInput) (*CostRateCard, error) {
	cardID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid rate card id", ErrValidation)
	}
	createInput := CreateRateCardInput(input)
	if createInput.Status == "" {
		createInput.Status = "active"
	}
	createInput.Currency = defaultCurrency(createInput.Currency)
	conversion, err := s.convert(ctx, createInput.Amount, createInput.Currency, BaseCurrency, effectiveTime(createInput.EffectiveFrom))
	if err != nil {
		return nil, err
	}
	return s.repo.UpdateRateCard(ctx, cardID, createInput, conversion)
}

func (s *Service) VoidRateCard(ctx context.Context, id string) (*CostRateCard, error) {
	cardID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid rate card id", ErrValidation)
	}
	return s.repo.VoidRateCard(ctx, cardID)
}

func (s *Service) CreateBudget(ctx context.Context, input CreateBudgetInput) (*CostBudget, error) {
	input.ScopeType = strings.TrimSpace(input.ScopeType)
	if input.ScopeType == "" {
		if orgID := currentTenantOrganizationID(ctx); orgID != nil {
			input.ScopeType = "organization"
			input.ScopeID = orgID
		}
	}
	if input.ScopeType == "" {
		return nil, fmt.Errorf("%w: scope_type is required", ErrValidation)
	}
	if err := ensureScopeAccess(ctx, input.ScopeType, input.ScopeID); err != nil {
		return nil, err
	}
	if input.Status == "" {
		input.Status = "active"
	}
	input.Currency = defaultCurrency(input.Currency)
	conversion, err := s.convert(ctx, input.Amount, input.Currency, BaseCurrency, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	return s.repo.CreateBudget(ctx, input, conversion)
}

func (s *Service) ListBudgets(ctx context.Context, limit int) ([]CostBudget, error) {
	if orgID := currentTenantOrganizationID(ctx); orgID != nil {
		return s.repo.ListBudgetsByScope(ctx, "organization", *orgID, limit)
	}
	return s.repo.ListBudgets(ctx, limit)
}

func (s *Service) UpdateBudget(ctx context.Context, id string, input UpdateBudgetInput) (*CostBudget, error) {
	budgetID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid budget id", ErrValidation)
	}
	createInput := CreateBudgetInput(input)
	if createInput.Status == "" {
		createInput.Status = "active"
	}
	createInput.Currency = defaultCurrency(createInput.Currency)
	conversion, err := s.convert(ctx, createInput.Amount, createInput.Currency, BaseCurrency, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	return s.repo.UpdateBudget(ctx, budgetID, createInput, conversion)
}

func (s *Service) VoidBudget(ctx context.Context, id string) (*CostBudget, error) {
	budgetID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid budget id", ErrValidation)
	}
	return s.repo.VoidBudget(ctx, budgetID)
}

func (s *Service) CreateLedgerEntry(ctx context.Context, input CreateLedgerEntryInput) (*CostLedgerEntry, error) {
	normalizeLedgerEntryInput(&input)
	if input.OrganizationID == nil {
		input.OrganizationID = currentTenantOrganizationID(ctx)
	}
	if err := ensureOrganizationAccess(ctx, input.OrganizationID); err != nil {
		return nil, err
	}
	if input.Amount == 0 {
		return nil, fmt.Errorf("%w: amount must not be zero", ErrValidation)
	}
	conversion, err := s.convert(ctx, input.Amount, input.Currency, BaseCurrency, effectiveTime(input.OccurredAt))
	if err != nil {
		return nil, err
	}
	return s.repo.CreateLedgerEntry(ctx, input, conversion)
}

func (s *Service) ListLedgerEntries(ctx context.Context, filter SummaryFilter) ([]CostLedgerEntry, error) {
	if filter.ScopeType == "" && filter.ScopeID == nil {
		if orgID := currentTenantOrganizationID(ctx); orgID != nil {
			filter.ScopeType = "organization"
			filter.ScopeID = orgID
		}
	}
	return s.repo.ListLedgerEntries(ctx, filter)
}

func (s *Service) UpdateLedgerEntry(ctx context.Context, id string, input UpdateLedgerEntryInput) (*CostLedgerEntry, error) {
	entryID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid ledger entry id", ErrValidation)
	}
	createInput := CreateLedgerEntryInput(input)
	normalizeLedgerEntryInput(&createInput)
	if createInput.Amount == 0 {
		return nil, fmt.Errorf("%w: amount must not be zero", ErrValidation)
	}
	conversion, err := s.convert(ctx, createInput.Amount, createInput.Currency, BaseCurrency, effectiveTime(createInput.OccurredAt))
	if err != nil {
		return nil, err
	}
	return s.repo.UpdateLedgerEntry(ctx, entryID, createInput, conversion)
}

func (s *Service) VoidLedgerEntry(ctx context.Context, id string) (*CostLedgerEntry, error) {
	entryID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid ledger entry id", ErrValidation)
	}
	return s.repo.VoidLedgerEntry(ctx, entryID)
}

func (s *Service) Summary(ctx context.Context, filter SummaryFilter) (*CostSummary, error) {
	if filter.ScopeType == "" && filter.ScopeID == nil {
		if orgID := currentTenantOrganizationID(ctx); orgID != nil {
			filter.ScopeType = "organization"
			filter.ScopeID = orgID
		}
	}
	if filter.Currency == "" {
		filter.Currency = BaseCurrency
	}
	return s.repo.Summary(ctx, filter)
}

func (s *Service) RecordActual(ctx context.Context, input CreateLedgerEntryInput) (*CostLedgerEntry, error) {
	input.LedgerType = LedgerActual
	return s.CreateLedgerEntry(ctx, input)
}

func (s *Service) convert(ctx context.Context, amount float64, fromCurrency, toCurrency string, at time.Time) (ConversionResult, error) {
	fromCurrency = defaultCurrency(fromCurrency)
	toCurrency = defaultCurrency(toCurrency)
	if fromCurrency == toCurrency {
		return ConversionResult{
			Amount:          amount,
			FromCurrency:    fromCurrency,
			ToCurrency:      toCurrency,
			ConvertedAmount: amount,
			Rate:            1,
		}, nil
	}
	rate, err := s.repo.FindExchangeRate(ctx, fromCurrency, toCurrency, at)
	if err == nil {
		return ConversionResult{
			Amount:                amount,
			FromCurrency:          fromCurrency,
			ToCurrency:            toCurrency,
			ConvertedAmount:       amount * rate.Rate,
			Rate:                  rate.Rate,
			ExchangeRateVersionID: &rate.ID,
		}, nil
	}
	if !isNoRows(err) {
		return ConversionResult{}, fmt.Errorf("find exchange rate: %w", err)
	}
	inverse, inverseErr := s.repo.FindExchangeRate(ctx, toCurrency, fromCurrency, at)
	if inverseErr == nil {
		rateValue := 1 / inverse.Rate
		return ConversionResult{
			Amount:                amount,
			FromCurrency:          fromCurrency,
			ToCurrency:            toCurrency,
			ConvertedAmount:       amount * rateValue,
			Rate:                  rateValue,
			ExchangeRateVersionID: &inverse.ID,
		}, nil
	}
	if !isNoRows(inverseErr) {
		return ConversionResult{}, fmt.Errorf("find inverse exchange rate: %w", inverseErr)
	}
	return ConversionResult{}, fmt.Errorf("%w: exchange rate %s to %s is required", ErrValidation, fromCurrency, toCurrency)
}

func normalizeLedgerEntryInput(input *CreateLedgerEntryInput) {
	if input.LedgerType == "" {
		input.LedgerType = LedgerActual
	}
	if input.CostCategory == "" {
		input.CostCategory = "manual"
	}
	if input.SourceType == "" {
		input.SourceType = "manual"
	}
	if input.Status == "" {
		input.Status = "posted"
	}
	input.Currency = defaultCurrency(input.Currency)
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
}

func defaultCurrency(value string) string {
	value = normalizeCurrency(value)
	if value == "" {
		return BaseCurrency
	}
	return value
}

func normalizeCurrency(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func effectiveTime(value *time.Time) time.Time {
	if value == nil {
		return time.Now().UTC()
	}
	return *value
}

func currentTenantOrganizationID(ctx context.Context) *uuid.UUID {
	tenant, ok := middleware.TenantFromContext(ctx)
	if !ok || tenant.OrganizationID == nil {
		return nil
	}
	id := *tenant.OrganizationID
	return &id
}

func ensureOrganizationAccess(ctx context.Context, organizationID *uuid.UUID) error {
	tenant, ok := middleware.TenantFromContext(ctx)
	if !ok || tenant.OrganizationID == nil || organizationID == nil {
		return nil
	}
	if *tenant.OrganizationID != *organizationID {
		return fmt.Errorf("%w: resource is outside current organization", ErrValidation)
	}
	return nil
}

func ensureScopeAccess(ctx context.Context, scopeType string, scopeID *uuid.UUID) error {
	if scopeType != "organization" {
		return nil
	}
	return ensureOrganizationAccess(ctx, scopeID)
}
