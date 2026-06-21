package costing

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) UpsertCurrency(ctx context.Context, input CreateCurrencyInput) (*Currency, error) {
	currency := &Currency{}
	err := scanCurrency(r.db.QueryRow(ctx, `
		INSERT INTO currencies (
			code, name, currency_type, symbol, precision_digits, chain_id,
			contract_address, external_source, is_active, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (code) DO UPDATE SET
			name = EXCLUDED.name,
			currency_type = EXCLUDED.currency_type,
			symbol = EXCLUDED.symbol,
			precision_digits = EXCLUDED.precision_digits,
			chain_id = EXCLUDED.chain_id,
			contract_address = EXCLUDED.contract_address,
			external_source = EXCLUDED.external_source,
			is_active = EXCLUDED.is_active,
			metadata = EXCLUDED.metadata,
			updated_at = NOW()
		RETURNING code, name, currency_type, symbol, precision_digits, chain_id,
			contract_address, external_source, is_active, metadata, created_at, updated_at
	`, input.Code, input.Name, input.CurrencyType, input.Symbol, input.PrecisionDigits, input.ChainID,
		input.ContractAddress, input.ExternalSource, activeBool(input.IsActive), mustJSON(input.Metadata)), currency)
	if err != nil {
		return nil, fmt.Errorf("upsert currency: %w", err)
	}
	return currency, nil
}

func (r *Repository) ListCurrencies(ctx context.Context) ([]Currency, error) {
	rows, err := r.db.Query(ctx, `
		SELECT code, name, currency_type, symbol, precision_digits, chain_id,
			contract_address, external_source, is_active, metadata, created_at, updated_at
		FROM currencies
		ORDER BY currency_type, code
	`)
	if err != nil {
		return nil, fmt.Errorf("list currencies: %w", err)
	}
	defer rows.Close()
	items := []Currency{}
	for rows.Next() {
		var item Currency
		if err := scanCurrency(rows, &item); err != nil {
			return nil, fmt.Errorf("scan currency: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) VoidCurrency(ctx context.Context, code string) (*Currency, error) {
	currency := &Currency{}
	err := scanCurrency(r.db.QueryRow(ctx, `
		UPDATE currencies
		SET is_active = FALSE, updated_at = NOW()
		WHERE code = $1
		RETURNING code, name, currency_type, symbol, precision_digits, chain_id,
			contract_address, external_source, is_active, metadata, created_at, updated_at
	`, code), currency)
	if err != nil {
		return nil, fmt.Errorf("void currency: %w", err)
	}
	return currency, nil
}

func (r *Repository) CreateExchangeRate(ctx context.Context, input CreateExchangeRateInput) (*ExchangeRateVersion, error) {
	rate := &ExchangeRateVersion{}
	effectiveFrom := time.Now().UTC()
	if input.EffectiveFrom != nil {
		effectiveFrom = *input.EffectiveFrom
	}
	err := scanExchangeRate(r.db.QueryRow(ctx, `
		INSERT INTO exchange_rate_versions (
			from_currency, to_currency, rate, source, provider, external_rate_id,
			effective_from, effective_to, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, from_currency, to_currency, rate::float8, source, provider,
			external_rate_id, effective_from, effective_to, metadata, created_at
	`, input.FromCurrency, input.ToCurrency, input.Rate, input.Source, input.Provider, input.ExternalRateID,
		effectiveFrom, input.EffectiveTo, mustJSON(input.Metadata)), rate)
	if err != nil {
		return nil, fmt.Errorf("create exchange rate: %w", err)
	}
	return rate, nil
}

func (r *Repository) ListExchangeRates(ctx context.Context, limit int) ([]ExchangeRateVersion, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, from_currency, to_currency, rate::float8, source, provider,
			external_rate_id, effective_from, effective_to, metadata, created_at
		FROM exchange_rate_versions
		ORDER BY effective_from DESC, created_at DESC
		LIMIT $1
	`, normalizeLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("list exchange rates: %w", err)
	}
	defer rows.Close()
	items := []ExchangeRateVersion{}
	for rows.Next() {
		var item ExchangeRateVersion
		if err := scanExchangeRate(rows, &item); err != nil {
			return nil, fmt.Errorf("scan exchange rate: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) UpdateExchangeRate(ctx context.Context, id uuid.UUID, input CreateExchangeRateInput) (*ExchangeRateVersion, error) {
	rate := &ExchangeRateVersion{}
	err := scanExchangeRate(r.db.QueryRow(ctx, `
		UPDATE exchange_rate_versions
		SET from_currency = COALESCE(NULLIF($2, ''), from_currency),
		    to_currency = COALESCE(NULLIF($3, ''), to_currency),
		    rate = $4,
		    source = COALESCE(NULLIF($5, ''), source),
		    provider = $6,
		    external_rate_id = $7,
		    effective_from = COALESCE($8, effective_from),
		    effective_to = $9,
		    metadata = CASE WHEN $10::jsonb IS NULL THEN metadata ELSE $10::jsonb END
		WHERE id = $1
		RETURNING id, from_currency, to_currency, rate::float8, source, provider,
			external_rate_id, effective_from, effective_to, metadata, created_at
	`, id, input.FromCurrency, input.ToCurrency, input.Rate, input.Source, input.Provider,
		input.ExternalRateID, input.EffectiveFrom, input.EffectiveTo, nullableJSON(input.Metadata)), rate)
	if err != nil {
		return nil, fmt.Errorf("update exchange rate: %w", err)
	}
	return rate, nil
}

func (r *Repository) VoidExchangeRate(ctx context.Context, id uuid.UUID) (*ExchangeRateVersion, error) {
	now := time.Now().UTC()
	rate := &ExchangeRateVersion{}
	err := scanExchangeRate(r.db.QueryRow(ctx, `
		UPDATE exchange_rate_versions
		SET effective_to = $2
		WHERE id = $1
		RETURNING id, from_currency, to_currency, rate::float8, source, provider,
			external_rate_id, effective_from, effective_to, metadata, created_at
	`, id, now), rate)
	if err != nil {
		return nil, fmt.Errorf("void exchange rate: %w", err)
	}
	return rate, nil
}

func (r *Repository) FindExchangeRate(ctx context.Context, fromCurrency, toCurrency string, at time.Time) (*ExchangeRateVersion, error) {
	rate := &ExchangeRateVersion{}
	err := scanExchangeRate(r.db.QueryRow(ctx, `
		SELECT id, from_currency, to_currency, rate::float8, source, provider,
			external_rate_id, effective_from, effective_to, metadata, created_at
		FROM exchange_rate_versions
		WHERE from_currency = $1
			AND to_currency = $2
			AND effective_from <= $3
			AND (effective_to IS NULL OR effective_to > $3)
		ORDER BY effective_from DESC, created_at DESC
		LIMIT 1
	`, fromCurrency, toCurrency, at), rate)
	if err != nil {
		return nil, err
	}
	return rate, nil
}

func (r *Repository) CreateRateCard(ctx context.Context, input CreateRateCardInput, conversion ConversionResult) (*CostRateCard, error) {
	card := &CostRateCard{}
	effectiveFrom := time.Now().UTC()
	if input.EffectiveFrom != nil {
		effectiveFrom = *input.EffectiveFrom
	}
	err := scanRateCard(r.db.QueryRow(ctx, `
		INSERT INTO cost_rate_cards (
			subject_type, subject_id, scope_type, scope_id, rate_type, amount, currency,
			base_amount, base_currency, exchange_rate_version_id, effective_from,
			effective_to, status, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id, subject_type, subject_id, scope_type, scope_id, rate_type,
			amount::float8, currency, base_amount::float8, base_currency,
			exchange_rate_version_id, effective_from, effective_to, status, metadata, created_at
	`, input.SubjectType, input.SubjectID, input.ScopeType, input.ScopeID, input.RateType,
		input.Amount, input.Currency, conversion.ConvertedAmount, conversion.ToCurrency,
		conversion.ExchangeRateVersionID, effectiveFrom, input.EffectiveTo, input.Status, mustJSON(input.Metadata)), card)
	if err != nil {
		return nil, fmt.Errorf("create rate card: %w", err)
	}
	return card, nil
}

func (r *Repository) ListRateCards(ctx context.Context, limit int) ([]CostRateCard, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, subject_type, subject_id, scope_type, scope_id, rate_type,
			amount::float8, currency, base_amount::float8, base_currency,
			exchange_rate_version_id, effective_from, effective_to, status, metadata, created_at
		FROM cost_rate_cards
		ORDER BY created_at DESC
		LIMIT $1
	`, normalizeLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("list rate cards: %w", err)
	}
	defer rows.Close()
	items := []CostRateCard{}
	for rows.Next() {
		var item CostRateCard
		if err := scanRateCard(rows, &item); err != nil {
			return nil, fmt.Errorf("scan rate card: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) ListRateCardsByScope(ctx context.Context, scopeType string, scopeID uuid.UUID, limit int) ([]CostRateCard, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, subject_type, subject_id, scope_type, scope_id, rate_type,
			amount::float8, currency, base_amount::float8, base_currency,
			exchange_rate_version_id, effective_from, effective_to, status, metadata, created_at
		FROM cost_rate_cards
		WHERE (scope_type = $1 AND scope_id = $2)
		   OR (scope_type = '' OR scope_type IS NULL)
		ORDER BY created_at DESC
		LIMIT $3
	`, scopeType, scopeID, normalizeLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("list scoped rate cards: %w", err)
	}
	defer rows.Close()
	items := []CostRateCard{}
	for rows.Next() {
		var item CostRateCard
		if err := scanRateCard(rows, &item); err != nil {
			return nil, fmt.Errorf("scan rate card: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) UpdateRateCard(ctx context.Context, id uuid.UUID, input CreateRateCardInput, conversion ConversionResult) (*CostRateCard, error) {
	card := &CostRateCard{}
	effectiveFrom := time.Now().UTC()
	if input.EffectiveFrom != nil {
		effectiveFrom = *input.EffectiveFrom
	}
	err := scanRateCard(r.db.QueryRow(ctx, `
		UPDATE cost_rate_cards
		SET subject_type = COALESCE(NULLIF($2, ''), subject_type),
		    subject_id = COALESCE($3, subject_id),
		    scope_type = $4,
		    scope_id = $5,
		    rate_type = COALESCE(NULLIF($6, ''), rate_type),
		    amount = $7,
		    currency = COALESCE(NULLIF($8, ''), currency),
		    base_amount = $9,
		    base_currency = $10,
		    exchange_rate_version_id = $11,
		    effective_from = $12,
		    effective_to = $13,
		    status = COALESCE(NULLIF($14, ''), status),
		    metadata = CASE WHEN $15::jsonb IS NULL THEN metadata ELSE $15::jsonb END
		WHERE id = $1
		RETURNING id, subject_type, subject_id, scope_type, scope_id, rate_type,
			amount::float8, currency, base_amount::float8, base_currency,
			exchange_rate_version_id, effective_from, effective_to, status, metadata, created_at
	`, id, input.SubjectType, input.SubjectID, input.ScopeType, input.ScopeID, input.RateType,
		input.Amount, input.Currency, conversion.ConvertedAmount, conversion.ToCurrency,
		conversion.ExchangeRateVersionID, effectiveFrom, input.EffectiveTo, input.Status, nullableJSON(input.Metadata)), card)
	if err != nil {
		return nil, fmt.Errorf("update rate card: %w", err)
	}
	return card, nil
}

func (r *Repository) VoidRateCard(ctx context.Context, id uuid.UUID) (*CostRateCard, error) {
	card := &CostRateCard{}
	err := scanRateCard(r.db.QueryRow(ctx, `
		UPDATE cost_rate_cards
		SET status = 'inactive', effective_to = NOW()
		WHERE id = $1
		RETURNING id, subject_type, subject_id, scope_type, scope_id, rate_type,
			amount::float8, currency, base_amount::float8, base_currency,
			exchange_rate_version_id, effective_from, effective_to, status, metadata, created_at
	`, id), card)
	if err != nil {
		return nil, fmt.Errorf("void rate card: %w", err)
	}
	return card, nil
}

func (r *Repository) CreateBudget(ctx context.Context, input CreateBudgetInput, conversion ConversionResult) (*CostBudget, error) {
	budget := &CostBudget{}
	err := scanBudget(r.db.QueryRow(ctx, `
		INSERT INTO cost_budgets (
			scope_type, scope_id, amount, currency, base_amount, base_currency,
			exchange_rate_version_id, period_start, period_end, status, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, scope_type, scope_id, amount::float8, currency, base_amount::float8,
			base_currency, exchange_rate_version_id, period_start, period_end, status,
			metadata, created_at, updated_at
	`, input.ScopeType, input.ScopeID, input.Amount, input.Currency, conversion.ConvertedAmount,
		conversion.ToCurrency, conversion.ExchangeRateVersionID, input.PeriodStart, input.PeriodEnd,
		input.Status, mustJSON(input.Metadata)), budget)
	if err != nil {
		return nil, fmt.Errorf("create budget: %w", err)
	}
	return budget, nil
}

func (r *Repository) ListBudgets(ctx context.Context, limit int) ([]CostBudget, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, scope_type, scope_id, amount::float8, currency, base_amount::float8,
			base_currency, exchange_rate_version_id, period_start, period_end, status,
			metadata, created_at, updated_at
		FROM cost_budgets
		ORDER BY created_at DESC
		LIMIT $1
	`, normalizeLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("list budgets: %w", err)
	}
	defer rows.Close()
	items := []CostBudget{}
	for rows.Next() {
		var item CostBudget
		if err := scanBudget(rows, &item); err != nil {
			return nil, fmt.Errorf("scan budget: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) ListBudgetsByScope(ctx context.Context, scopeType string, scopeID uuid.UUID, limit int) ([]CostBudget, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, scope_type, scope_id, amount::float8, currency, base_amount::float8,
			base_currency, exchange_rate_version_id, period_start, period_end, status,
			metadata, created_at, updated_at
		FROM cost_budgets
		WHERE scope_type = $1 AND scope_id = $2
		ORDER BY created_at DESC
		LIMIT $3
	`, scopeType, scopeID, normalizeLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("list scoped budgets: %w", err)
	}
	defer rows.Close()
	items := []CostBudget{}
	for rows.Next() {
		var item CostBudget
		if err := scanBudget(rows, &item); err != nil {
			return nil, fmt.Errorf("scan budget: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) UpdateBudget(ctx context.Context, id uuid.UUID, input CreateBudgetInput, conversion ConversionResult) (*CostBudget, error) {
	budget := &CostBudget{}
	err := scanBudget(r.db.QueryRow(ctx, `
		UPDATE cost_budgets
		SET scope_type = COALESCE(NULLIF($2, ''), scope_type),
		    scope_id = COALESCE($3, scope_id),
		    amount = $4,
		    currency = COALESCE(NULLIF($5, ''), currency),
		    base_amount = $6,
		    base_currency = $7,
		    exchange_rate_version_id = $8,
		    period_start = $9,
		    period_end = $10,
		    status = COALESCE(NULLIF($11, ''), status),
		    metadata = CASE WHEN $12::jsonb IS NULL THEN metadata ELSE $12::jsonb END,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id, scope_type, scope_id, amount::float8, currency, base_amount::float8,
			base_currency, exchange_rate_version_id, period_start, period_end, status,
			metadata, created_at, updated_at
	`, id, input.ScopeType, input.ScopeID, input.Amount, input.Currency, conversion.ConvertedAmount,
		conversion.ToCurrency, conversion.ExchangeRateVersionID, input.PeriodStart, input.PeriodEnd,
		input.Status, nullableJSON(input.Metadata)), budget)
	if err != nil {
		return nil, fmt.Errorf("update budget: %w", err)
	}
	return budget, nil
}

func (r *Repository) VoidBudget(ctx context.Context, id uuid.UUID) (*CostBudget, error) {
	budget := &CostBudget{}
	err := scanBudget(r.db.QueryRow(ctx, `
		UPDATE cost_budgets
		SET status = 'void', updated_at = NOW()
		WHERE id = $1
		RETURNING id, scope_type, scope_id, amount::float8, currency, base_amount::float8,
			base_currency, exchange_rate_version_id, period_start, period_end, status,
			metadata, created_at, updated_at
	`, id), budget)
	if err != nil {
		return nil, fmt.Errorf("void budget: %w", err)
	}
	return budget, nil
}

func (r *Repository) CreateLedgerEntry(ctx context.Context, input CreateLedgerEntryInput, conversion ConversionResult) (*CostLedgerEntry, error) {
	entry := &CostLedgerEntry{}
	occurredAt := time.Now().UTC()
	if input.OccurredAt != nil {
		occurredAt = *input.OccurredAt
	}
	err := scanLedgerEntry(r.db.QueryRow(ctx, `
		INSERT INTO cost_ledger_entries (
			ledger_type, cost_category, source_type, source_id, organization_id,
			department_id, requirement_id, project_id, workflow_id, task_id,
			capability_id, actor_id, actor_type, resource_type, amount, currency,
			base_amount, base_currency, exchange_rate_version_id, occurred_at,
			status, description, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
			$13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23)
		ON CONFLICT (source_type, source_id, ledger_type)
			WHERE source_id IS NOT NULL AND ledger_type = 'actual'
		DO UPDATE SET
			amount = EXCLUDED.amount,
			currency = EXCLUDED.currency,
			base_amount = EXCLUDED.base_amount,
			base_currency = EXCLUDED.base_currency,
			exchange_rate_version_id = EXCLUDED.exchange_rate_version_id,
			status = EXCLUDED.status,
			description = EXCLUDED.description,
			metadata = EXCLUDED.metadata
		RETURNING id, ledger_type, cost_category, source_type, source_id,
			organization_id, department_id, requirement_id, project_id, workflow_id,
			task_id, capability_id, actor_id, actor_type, resource_type,
			amount::float8, currency, base_amount::float8, base_currency,
			exchange_rate_version_id, occurred_at, status, finance_export_line_id,
			description, metadata, created_at
	`, input.LedgerType, input.CostCategory, input.SourceType, input.SourceID, input.OrganizationID,
		input.DepartmentID, input.RequirementID, input.ProjectID, input.WorkflowID, input.TaskID,
		input.CapabilityID, input.ActorID, input.ActorType, input.ResourceType, input.Amount,
		input.Currency, conversion.ConvertedAmount, conversion.ToCurrency, conversion.ExchangeRateVersionID,
		occurredAt, input.Status, input.Description, mustJSON(input.Metadata)), entry)
	if err != nil {
		return nil, fmt.Errorf("create ledger entry: %w", err)
	}
	return entry, nil
}

func (r *Repository) ListLedgerEntries(ctx context.Context, filter SummaryFilter) ([]CostLedgerEntry, error) {
	limit := normalizeLimit(filter.Limit)
	args := []any{}
	where := []string{"status <> 'void'"}
	if filter.ScopeType != "" && filter.ScopeID != nil {
		column := scopeColumn(filter.ScopeType)
		if column != "" {
			args = append(args, *filter.ScopeID)
			where = append(where, fmt.Sprintf("%s = $%d", column, len(args)))
		}
	}
	args = append(args, limit)
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT id, ledger_type, cost_category, source_type, source_id,
			organization_id, department_id, requirement_id, project_id, workflow_id,
			task_id, capability_id, actor_id, actor_type, resource_type,
			amount::float8, currency, base_amount::float8, base_currency,
			exchange_rate_version_id, occurred_at, status, finance_export_line_id,
			description, metadata, created_at
		FROM cost_ledger_entries
		WHERE %s
		ORDER BY occurred_at DESC, created_at DESC
		LIMIT $%d
	`, strings.Join(where, " AND "), len(args)), args...)
	if err != nil {
		return nil, fmt.Errorf("list ledger entries: %w", err)
	}
	defer rows.Close()
	items := []CostLedgerEntry{}
	for rows.Next() {
		var item CostLedgerEntry
		if err := scanLedgerEntry(rows, &item); err != nil {
			return nil, fmt.Errorf("scan ledger entry: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) UpdateLedgerEntry(ctx context.Context, id uuid.UUID, input CreateLedgerEntryInput, conversion ConversionResult) (*CostLedgerEntry, error) {
	entry := &CostLedgerEntry{}
	occurredAt := time.Now().UTC()
	if input.OccurredAt != nil {
		occurredAt = *input.OccurredAt
	}
	err := scanLedgerEntry(r.db.QueryRow(ctx, `
		UPDATE cost_ledger_entries
		SET ledger_type = COALESCE(NULLIF($2, ''), ledger_type),
		    cost_category = COALESCE(NULLIF($3, ''), cost_category),
		    source_type = COALESCE(NULLIF($4, ''), source_type),
		    amount = $5,
		    currency = COALESCE(NULLIF($6, ''), currency),
		    base_amount = $7,
		    base_currency = $8,
		    exchange_rate_version_id = $9,
		    occurred_at = $10,
		    status = COALESCE(NULLIF($11, ''), status),
		    description = $12,
		    metadata = CASE WHEN $13::jsonb IS NULL THEN metadata ELSE $13::jsonb END
		WHERE id = $1 AND finance_export_line_id IS NULL
		RETURNING id, ledger_type, cost_category, source_type, source_id,
			organization_id, department_id, requirement_id, project_id, workflow_id,
			task_id, capability_id, actor_id, actor_type, resource_type,
			amount::float8, currency, base_amount::float8, base_currency,
			exchange_rate_version_id, occurred_at, status, finance_export_line_id,
			description, metadata, created_at
	`, id, input.LedgerType, input.CostCategory, input.SourceType, input.Amount, input.Currency,
		conversion.ConvertedAmount, conversion.ToCurrency, conversion.ExchangeRateVersionID,
		occurredAt, input.Status, input.Description, nullableJSON(input.Metadata)), entry)
	if err != nil {
		return nil, fmt.Errorf("update ledger entry: %w", err)
	}
	return entry, nil
}

func (r *Repository) VoidLedgerEntry(ctx context.Context, id uuid.UUID) (*CostLedgerEntry, error) {
	entry := &CostLedgerEntry{}
	err := scanLedgerEntry(r.db.QueryRow(ctx, `
		UPDATE cost_ledger_entries
		SET status = 'void'
		WHERE id = $1 AND finance_export_line_id IS NULL
		RETURNING id, ledger_type, cost_category, source_type, source_id,
			organization_id, department_id, requirement_id, project_id, workflow_id,
			task_id, capability_id, actor_id, actor_type, resource_type,
			amount::float8, currency, base_amount::float8, base_currency,
			exchange_rate_version_id, occurred_at, status, finance_export_line_id,
			description, metadata, created_at
	`, id), entry)
	if err != nil {
		return nil, fmt.Errorf("void ledger entry: %w", err)
	}
	return entry, nil
}

func (r *Repository) Summary(ctx context.Context, filter SummaryFilter) (*CostSummary, error) {
	if filter.Currency == "" {
		filter.Currency = BaseCurrency
	}
	summary := &CostSummary{
		ScopeType:  filter.ScopeType,
		ScopeID:    filter.ScopeID,
		Currency:   filter.Currency,
		ByCategory: map[string]float64{},
		BySource:   map[string]float64{},
		ByCurrency: map[string]float64{},
		Metadata:   map[string]any{},
	}
	where, args := scopeWhere(filter.ScopeType, filter.ScopeID)
	totalSQL := fmt.Sprintf(`
		SELECT COUNT(*), COALESCE(SUM(base_amount), 0)::float8
		FROM cost_ledger_entries
		WHERE status <> 'void' AND ledger_type IN ('actual', 'adjustment') %s
	`, where)
	if err := r.db.QueryRow(ctx, totalSQL, args...).Scan(&summary.EntryCount, &summary.TotalAmount); err != nil {
		return nil, fmt.Errorf("cost summary total: %w", err)
	}
	budgetWhere, budgetArgs := budgetScopeWhere(filter.ScopeType, filter.ScopeID)
	budgetSQL := fmt.Sprintf(`
		SELECT COALESCE(SUM(base_amount), 0)::float8
		FROM cost_budgets
		WHERE status = 'active' %s
	`, budgetWhere)
	if err := r.db.QueryRow(ctx, budgetSQL, budgetArgs...).Scan(&summary.BudgetAmount); err != nil {
		return nil, fmt.Errorf("cost budget summary: %w", err)
	}
	summary.BudgetVariance = summary.BudgetAmount - summary.TotalAmount
	if err := r.scanSummaryMap(ctx, summary.ByCategory, "cost_category", where, args); err != nil {
		return nil, err
	}
	if err := r.scanSummaryMap(ctx, summary.BySource, "source_type", where, args); err != nil {
		return nil, err
	}
	if err := r.scanCurrencyMap(ctx, summary.ByCurrency, where, args); err != nil {
		return nil, err
	}
	recent, err := r.ListLedgerEntries(ctx, SummaryFilter{ScopeType: filter.ScopeType, ScopeID: filter.ScopeID, Limit: 8})
	if err != nil {
		return nil, err
	}
	summary.RecentEntries = recent
	return summary, nil
}

func (r *Repository) scanSummaryMap(ctx context.Context, target map[string]float64, column string, where string, args []any) error {
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT %s, COALESCE(SUM(base_amount), 0)::float8
		FROM cost_ledger_entries
		WHERE status <> 'void' AND ledger_type IN ('actual', 'adjustment') %s
		GROUP BY %s
		ORDER BY SUM(base_amount) DESC
	`, column, where, column), args...)
	if err != nil {
		return fmt.Errorf("cost summary %s: %w", column, err)
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var value float64
		if err := rows.Scan(&key, &value); err != nil {
			return err
		}
		target[key] = value
	}
	return rows.Err()
}

func (r *Repository) scanCurrencyMap(ctx context.Context, target map[string]float64, where string, args []any) error {
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT currency, COALESCE(SUM(amount), 0)::float8
		FROM cost_ledger_entries
		WHERE status <> 'void' AND ledger_type IN ('actual', 'adjustment') %s
		GROUP BY currency
		ORDER BY SUM(amount) DESC
	`, where), args...)
	if err != nil {
		return fmt.Errorf("cost summary currency: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var value float64
		if err := rows.Scan(&key, &value); err != nil {
			return err
		}
		target[key] = value
	}
	return rows.Err()
}

func scopeWhere(scopeType string, scopeID *uuid.UUID) (string, []any) {
	if scopeType == "" || scopeID == nil {
		return "", []any{}
	}
	column := scopeColumn(scopeType)
	if column == "" {
		return "", []any{}
	}
	return " AND " + column + " = $1", []any{*scopeID}
}

func budgetScopeWhere(scopeType string, scopeID *uuid.UUID) (string, []any) {
	if scopeType == "" || scopeID == nil {
		return "", []any{}
	}
	return " AND scope_type = $1 AND scope_id = $2", []any{scopeType, *scopeID}
}

func scopeColumn(scopeType string) string {
	switch scopeType {
	case "organization":
		return "organization_id"
	case "department":
		return "department_id"
	case "requirement":
		return "requirement_id"
	case "project":
		return "project_id"
	case "workflow":
		return "workflow_id"
	case "task":
		return "task_id"
	case "capability":
		return "capability_id"
	default:
		return ""
	}
}

type scanner interface {
	Scan(dest ...any) error
}

func scanCurrency(row scanner, item *Currency) error {
	var metadata []byte
	if err := row.Scan(&item.Code, &item.Name, &item.CurrencyType, &item.Symbol, &item.PrecisionDigits,
		&item.ChainID, &item.ContractAddress, &item.ExternalSource, &item.IsActive, &metadata,
		&item.CreatedAt, &item.UpdatedAt); err != nil {
		return err
	}
	item.Metadata = unmarshalMap(metadata)
	return nil
}

func scanExchangeRate(row scanner, item *ExchangeRateVersion) error {
	var metadata []byte
	if err := row.Scan(&item.ID, &item.FromCurrency, &item.ToCurrency, &item.Rate, &item.Source,
		&item.Provider, &item.ExternalRateID, &item.EffectiveFrom, &item.EffectiveTo,
		&metadata, &item.CreatedAt); err != nil {
		return err
	}
	item.Metadata = unmarshalMap(metadata)
	return nil
}

func scanRateCard(row scanner, item *CostRateCard) error {
	var metadata []byte
	var subjectID, scopeID, rateID pgtype.UUID
	if err := row.Scan(&item.ID, &item.SubjectType, &subjectID, &item.ScopeType, &scopeID,
		&item.RateType, &item.Amount, &item.Currency, &item.BaseAmount, &item.BaseCurrency,
		&rateID, &item.EffectiveFrom, &item.EffectiveTo, &item.Status, &metadata,
		&item.CreatedAt); err != nil {
		return err
	}
	item.SubjectID = uuidFromPg(subjectID)
	item.ScopeID = uuidFromPg(scopeID)
	item.ExchangeRateVersionID = uuidFromPg(rateID)
	item.Metadata = unmarshalMap(metadata)
	return nil
}

func scanBudget(row scanner, item *CostBudget) error {
	var metadata []byte
	var scopeID, rateID pgtype.UUID
	if err := row.Scan(&item.ID, &item.ScopeType, &scopeID, &item.Amount, &item.Currency,
		&item.BaseAmount, &item.BaseCurrency, &rateID, &item.PeriodStart, &item.PeriodEnd,
		&item.Status, &metadata, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return err
	}
	item.ScopeID = uuidFromPg(scopeID)
	item.ExchangeRateVersionID = uuidFromPg(rateID)
	item.Metadata = unmarshalMap(metadata)
	return nil
}

func scanLedgerEntry(row scanner, item *CostLedgerEntry) error {
	var metadata []byte
	var sourceID, orgID, deptID, reqID, projectID, workflowID, taskID, capabilityID, actorID, rateID, lineID pgtype.UUID
	if err := row.Scan(&item.ID, &item.LedgerType, &item.CostCategory, &item.SourceType, &sourceID,
		&orgID, &deptID, &reqID, &projectID, &workflowID, &taskID, &capabilityID, &actorID,
		&item.ActorType, &item.ResourceType, &item.Amount, &item.Currency, &item.BaseAmount,
		&item.BaseCurrency, &rateID, &item.OccurredAt, &item.Status, &lineID,
		&item.Description, &metadata, &item.CreatedAt); err != nil {
		return err
	}
	item.SourceID = uuidFromPg(sourceID)
	item.OrganizationID = uuidFromPg(orgID)
	item.DepartmentID = uuidFromPg(deptID)
	item.RequirementID = uuidFromPg(reqID)
	item.ProjectID = uuidFromPg(projectID)
	item.WorkflowID = uuidFromPg(workflowID)
	item.TaskID = uuidFromPg(taskID)
	item.CapabilityID = uuidFromPg(capabilityID)
	item.ActorID = uuidFromPg(actorID)
	item.ExchangeRateVersionID = uuidFromPg(rateID)
	item.FinanceExportLineID = uuidFromPg(lineID)
	item.Metadata = unmarshalMap(metadata)
	return nil
}

func uuidFromPg(value pgtype.UUID) *uuid.UUID {
	if !value.Valid {
		return nil
	}
	id, err := uuid.FromBytes(value.Bytes[:])
	if err != nil {
		return nil
	}
	return &id
}

func mustJSON(value map[string]any) []byte {
	if value == nil {
		value = map[string]any{}
	}
	data, _ := json.Marshal(value)
	return data
}

func nullableJSON(value map[string]any) any {
	if value == nil {
		return nil
	}
	return mustJSON(value)
}

func unmarshalMap(data []byte) map[string]any {
	out := map[string]any{}
	_ = json.Unmarshal(data, &out)
	return out
}

func activeBool(value *bool) bool {
	if value == nil {
		return true
	}
	return *value
}

func normalizeLimit(limit int) int {
	if limit <= 0 || limit > 200 {
		return 50
	}
	return limit
}

func isNoRows(err error) bool {
	return err == pgx.ErrNoRows
}
