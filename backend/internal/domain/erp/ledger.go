package erp

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type TrialBalanceInput struct {
	PeriodStart string `json:"period_start,omitempty"`
	PeriodEnd   string `json:"period_end,omitempty"`
	Currency    string `json:"currency"`
}

type TrialBalanceRow struct {
	AccountCode string  `json:"account_code"`
	AccountName string  `json:"account_name"`
	Debit       float64 `json:"debit"`
	Credit      float64 `json:"credit"`
	NetAmount   float64 `json:"net_amount"`
}

type TrialBalance struct {
	Currency     string            `json:"currency"`
	TotalDebit   float64           `json:"total_debit"`
	TotalCredit  float64           `json:"total_credit"`
	JournalCount int               `json:"journal_count"`
	Rows         []TrialBalanceRow `json:"rows"`
}

type ledgerReader interface {
	QueryTrialBalance(context.Context, TrialBalanceInput) (*TrialBalance, error)
}

func (s *Service) TrialBalance(ctx context.Context, input TrialBalanceInput) (*TrialBalance, error) {
	if err := s.authorize(ctx, "MJDT", "read"); err != nil {
		return nil, err
	}
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	if input.Currency == "" {
		input.Currency = "CNY"
	}
	if len(input.Currency) != 3 {
		return nil, fmt.Errorf("%w: a three-letter currency is required", ErrValidation)
	}
	for _, date := range []string{input.PeriodStart, input.PeriodEnd} {
		if date != "" {
			if _, err := time.Parse("2006-01-02", date); err != nil {
				return nil, fmt.Errorf("%w: invalid trial balance date", ErrValidation)
			}
		}
	}
	if input.PeriodStart != "" && input.PeriodEnd != "" && input.PeriodStart > input.PeriodEnd {
		return nil, fmt.Errorf("%w: period end precedes period start", ErrValidation)
	}
	if reader, ok := s.repo.(ledgerReader); ok {
		return reader.QueryTrialBalance(ctx, input)
	}
	return nil, fmt.Errorf("%w: ledger aggregation is not configured", ErrValidation)
}

func (r *PostgresRepository) QueryTrialBalance(ctx context.Context, input TrialBalanceInput) (*TrialBalance, error) {
	const filter = `(e."Payload"->>'BtfStatus' = 'P' OR e."Payload"->>'Posted' = 'Y')
		AND COALESCE(NULLIF(e."Payload"->>'Currency',''), NULLIF(e."Payload"->>'DocCur',''), 'CNY') = $1
		AND ($2::date IS NULL OR COALESCE(NULLIF(e."Payload"->>'RefDate','')::date, e."CreatedAt"::date) >= $2::date)
		AND ($3::date IS NULL OR COALESCE(NULLIF(e."Payload"->>'RefDate','')::date, e."CreatedAt"::date) <= $3::date)`
	var start, end any
	if input.PeriodStart != "" {
		start = input.PeriodStart
	}
	if input.PeriodEnd != "" {
		end = input.PeriodEnd
	}
	balance := &TrialBalance{Currency: input.Currency, Rows: []TrialBalanceRow{}}
	rows, err := r.querier.Query(ctx, `
		SELECT COALESCE(l."Payload"->>'AccountCode', l."Payload"->>'Account', ''),
		       COALESCE(MAX(NULLIF(a."Payload"->>'Name','')), MAX(NULLIF(l."Payload"->>'AccountName','')), l."Payload"->>'AccountCode', l."Payload"->>'Account', ''),
		       COALESCE(SUM((l."Payload"->>'Debit')::numeric),0)::float8,
		       COALESCE(SUM((l."Payload"->>'Credit')::numeric),0)::float8
		FROM "MJDT" e JOIN "JDT1" l ON l."TransId" = e."TransId"
		LEFT JOIN "MACT" a ON a."AcctCode" = COALESCE(l."Payload"->>'AccountCode', l."Payload"->>'Account')
		WHERE `+filter+`
		GROUP BY l."Payload"->>'AccountCode', l."Payload"->>'Account'
		ORDER BY 1`, input.Currency, start, end)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var row TrialBalanceRow
		if err := rows.Scan(&row.AccountCode, &row.AccountName, &row.Debit, &row.Credit); err != nil {
			rows.Close()
			return nil, err
		}
		row.NetAmount = moneyAdd(row.Debit, -row.Credit)
		balance.TotalDebit, balance.TotalCredit = moneyAdd(balance.TotalDebit, row.Debit), moneyAdd(balance.TotalCredit, row.Credit)
		balance.Rows = append(balance.Rows, row)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if err := r.querier.QueryRow(ctx, `SELECT COUNT(*) FROM "MJDT" e WHERE `+filter, input.Currency, start, end).Scan(&balance.JournalCount); err != nil {
		return nil, err
	}
	return balance, nil
}

type allChildReader interface {
	ListBusinessLines(context.Context, TableDefinition, ChildTableDefinition, string) ([]Record, error)
}

func (r *PostgresRepository) ListBusinessLines(ctx context.Context, parent TableDefinition, child ChildTableDefinition, key string) ([]Record, error) {
	query := fmt.Sprintf(`SELECT %s::TEXT, row_to_json(t)::jsonb, "CreatedAt", "UpdatedAt" FROM %s t WHERE %s::TEXT = $1 ORDER BY %s LIMIT 10001`,
		quoteIdent(child.LineKey), quoteIdent(child.Code), quoteIdent(child.ParentKey), quoteIdent(child.LineKey))
	rows, err := r.querier.Query(ctx, query, key)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	lines, err := scanRecords(rows, child.Code, parent.Code, key)
	if err != nil {
		return nil, err
	}
	if len(lines) > 10000 {
		return nil, fmt.Errorf("%w: document exceeds 10000 lines", ErrValidation)
	}
	return lines, nil
}
