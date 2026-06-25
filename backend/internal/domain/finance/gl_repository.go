package finance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (r *PostgresRepository) CreateGLAccount(ctx context.Context, input CreateGLAccountInput) (*GLAccount, error) {
	account := &GLAccount{}
	err := scanGLAccount(r.db.QueryRow(ctx, `
		INSERT INTO gl_accounts (
			account_code, name, account_type, currency, parent_account_code,
			postable, active, organization_id, department_id, metadata
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING id, master_key, account_code, name, account_type, currency, parent_account_code,
			postable, active, organization_id, department_id, metadata, created_at, updated_at
	`, input.AccountCode, input.Name, input.AccountType, input.Currency, input.ParentAccountCode,
		boolFromPtr(input.Postable, true), boolFromPtr(input.Active, true), nullableUUID(input.OrganizationID),
		nullableUUID(input.DepartmentID), mustJSON(input.Metadata)), account)
	if err != nil {
		return nil, fmt.Errorf("create GL account: %w", err)
	}
	return account, nil
}

func (r *PostgresRepository) ListGLAccounts(ctx context.Context, limit int) ([]GLAccount, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, master_key, account_code, name, account_type, currency, parent_account_code,
		       postable, active, organization_id, department_id, metadata, created_at, updated_at
		FROM gl_accounts
		ORDER BY account_code
		LIMIT $1
	`, normalizeLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("list GL accounts: %w", err)
	}
	defer rows.Close()
	items := []GLAccount{}
	for rows.Next() {
		var item GLAccount
		if err := scanGLAccount(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) CreateGLCostCenter(ctx context.Context, input CreateGLCostCenterInput) (*GLCostCenter, error) {
	center := &GLCostCenter{}
	err := scanGLCostCenter(r.db.QueryRow(ctx, `
		INSERT INTO gl_cost_centers (
			cost_center_code, name, active, organization_id, department_id, metadata
		)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, master_key, cost_center_code, name, active, organization_id, department_id,
			metadata, created_at, updated_at
	`, input.CostCenterCode, input.Name, boolFromPtr(input.Active, true), nullableUUID(input.OrganizationID),
		nullableUUID(input.DepartmentID), mustJSON(input.Metadata)), center)
	if err != nil {
		return nil, fmt.Errorf("create GL cost center: %w", err)
	}
	return center, nil
}

func (r *PostgresRepository) ListGLCostCenters(ctx context.Context, limit int) ([]GLCostCenter, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, master_key, cost_center_code, name, active, organization_id, department_id,
		       metadata, created_at, updated_at
		FROM gl_cost_centers
		ORDER BY cost_center_code
		LIMIT $1
	`, normalizeLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("list GL cost centers: %w", err)
	}
	defer rows.Close()
	items := []GLCostCenter{}
	for rows.Next() {
		var item GLCostCenter
		if err := scanGLCostCenter(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) CreateGLJournalEntry(ctx context.Context, input CreateGLJournalEntryInput, referenceDate time.Time) (*GLJournalEntry, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin GL journal entry: %w", err)
	}
	defer tx.Rollback(ctx)

	entry := &GLJournalEntry{}
	err = scanGLJournalEntry(tx.QueryRow(ctx, `
		INSERT INTO gl_journal_entries (
			entry_number, reference_date, memo, status, currency, source_type, source_id,
			organization_id, department_id, metadata
		)
		VALUES ($1,$2,$3,'draft',$4,$5,$6,$7,$8,$9)
		RETURNING id, master_key, entry_number, reference_date, memo, status, currency, source_type,
			source_id, organization_id, department_id, metadata, posted_at, created_at, updated_at
	`, input.EntryNumber, referenceDate, input.Memo, input.Currency, input.SourceType, nullableUUID(input.SourceID),
		nullableUUID(input.OrganizationID), nullableUUID(input.DepartmentID), mustJSON(input.Metadata)), entry)
	if err != nil {
		return nil, fmt.Errorf("create GL journal entry: %w", err)
	}
	for index, line := range input.Lines {
		item := &GLJournalEntryLine{}
		if err := scanGLJournalEntryLine(tx.QueryRow(ctx, `
			INSERT INTO gl_journal_entry_lines (
				entry_id, line_num, account_code, account_name, cost_center_code,
				debit, credit, description, metadata
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			RETURNING id, entry_id, line_num, account_code, account_name, cost_center_code,
				debit::float8, credit::float8, description, metadata, created_at
		`, entry.ID, index+1, line.AccountCode, line.AccountName, line.CostCenterCode,
			line.Debit, line.Credit, line.Description, mustJSON(line.Metadata)), item); err != nil {
			return nil, fmt.Errorf("create GL journal entry line: %w", err)
		}
		entry.Lines = append(entry.Lines, *item)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit GL journal entry: %w", err)
	}
	return entry, nil
}

func (r *PostgresRepository) ListGLJournalEntries(ctx context.Context, limit int) ([]GLJournalEntry, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, master_key, entry_number, reference_date, memo, status, currency, source_type,
		       source_id, organization_id, department_id, metadata, posted_at, created_at, updated_at
		FROM gl_journal_entries
		ORDER BY reference_date DESC, created_at DESC
		LIMIT $1
	`, normalizeLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("list GL journal entries: %w", err)
	}
	defer rows.Close()
	items := []GLJournalEntry{}
	for rows.Next() {
		var item GLJournalEntry
		if err := scanGLJournalEntry(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) GetGLJournalEntry(ctx context.Context, id uuid.UUID) (*GLJournalEntry, error) {
	entry := &GLJournalEntry{}
	err := scanGLJournalEntry(r.db.QueryRow(ctx, `
		SELECT id, master_key, entry_number, reference_date, memo, status, currency, source_type,
		       source_id, organization_id, department_id, metadata, posted_at, created_at, updated_at
		FROM gl_journal_entries
		WHERE id = $1
	`, id), entry)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get GL journal entry: %w", err)
	}
	lines, err := r.listGLJournalEntryLines(ctx, id)
	if err != nil {
		return nil, err
	}
	entry.Lines = lines
	return entry, nil
}

func (r *PostgresRepository) PostGLJournalEntry(ctx context.Context, id uuid.UUID) (*GLJournalEntry, error) {
	_, err := r.db.Exec(ctx, `
		UPDATE gl_journal_entries
		SET status = 'posted', posted_at = COALESCE(posted_at, NOW()), updated_at = NOW()
		WHERE id = $1
	`, id)
	if err != nil {
		return nil, fmt.Errorf("post GL journal entry: %w", err)
	}
	return r.GetGLJournalEntry(ctx, id)
}

func (r *PostgresRepository) GetGLTrialBalance(ctx context.Context, input GLTrialBalanceInput) (*GLTrialBalance, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			l.account_code,
			COALESCE(NULLIF(MAX(l.account_name), ''), l.account_code) AS account_name,
			COALESCE(SUM(l.debit), 0)::float8 AS debit,
			COALESCE(SUM(l.credit), 0)::float8 AS credit
		FROM gl_journal_entries e
		JOIN gl_journal_entry_lines l ON l.entry_id = e.id
		WHERE e.status = 'posted'
		  AND ($1::uuid IS NULL OR e.organization_id = $1)
		  AND ($2::date IS NULL OR e.reference_date >= $2)
		  AND ($3::date IS NULL OR e.reference_date <= $3)
		  AND ($4 = '' OR e.currency = $4)
		GROUP BY l.account_code
		ORDER BY l.account_code
	`, nullableUUID(input.OrganizationID), nullableDate(input.periodStartTime), nullableDate(input.periodEndTime), input.Currency)
	if err != nil {
		return nil, fmt.Errorf("get GL trial balance: %w", err)
	}
	defer rows.Close()
	balance := &GLTrialBalance{Rows: []GLTrialBalanceRow{}, Currency: input.Currency}
	for rows.Next() {
		var row GLTrialBalanceRow
		if err := rows.Scan(&row.AccountCode, &row.AccountName, &row.Debit, &row.Credit); err != nil {
			return nil, err
		}
		row.NetAmount = row.Debit - row.Credit
		balance.TotalDebit += row.Debit
		balance.TotalCredit += row.Credit
		balance.Rows = append(balance.Rows, row)
	}
	return balance, rows.Err()
}

func (r *PostgresRepository) listGLJournalEntryLines(ctx context.Context, entryID uuid.UUID) ([]GLJournalEntryLine, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, entry_id, line_num, account_code, account_name, cost_center_code,
		       debit::float8, credit::float8, description, metadata, created_at
		FROM gl_journal_entry_lines
		WHERE entry_id = $1
		ORDER BY line_num
	`, entryID)
	if err != nil {
		return nil, fmt.Errorf("list GL journal entry lines: %w", err)
	}
	defer rows.Close()
	items := []GLJournalEntryLine{}
	for rows.Next() {
		var item GLJournalEntryLine
		if err := scanGLJournalEntryLine(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanGLAccount(row scanner, account *GLAccount) error {
	var metadataJSON []byte
	var organizationID, departmentID pgtype.UUID
	if err := row.Scan(&account.ID, &account.MasterKey, &account.AccountCode, &account.Name,
		&account.AccountType, &account.Currency, &account.ParentAccountCode, &account.Postable,
		&account.Active, &organizationID, &departmentID, &metadataJSON, &account.CreatedAt,
		&account.UpdatedAt); err != nil {
		return err
	}
	account.OrganizationID = uuidPtr(organizationID)
	account.DepartmentID = uuidPtr(departmentID)
	return json.Unmarshal(metadataJSON, &account.Metadata)
}

func scanGLCostCenter(row scanner, center *GLCostCenter) error {
	var metadataJSON []byte
	var organizationID, departmentID pgtype.UUID
	if err := row.Scan(&center.ID, &center.MasterKey, &center.CostCenterCode, &center.Name,
		&center.Active, &organizationID, &departmentID, &metadataJSON, &center.CreatedAt,
		&center.UpdatedAt); err != nil {
		return err
	}
	center.OrganizationID = uuidPtr(organizationID)
	center.DepartmentID = uuidPtr(departmentID)
	return json.Unmarshal(metadataJSON, &center.Metadata)
}

func scanGLJournalEntry(row scanner, entry *GLJournalEntry) error {
	var metadataJSON []byte
	var sourceID, organizationID, departmentID pgtype.UUID
	if err := row.Scan(&entry.ID, &entry.MasterKey, &entry.EntryNumber, &entry.ReferenceDate,
		&entry.Memo, &entry.Status, &entry.Currency, &entry.SourceType, &sourceID,
		&organizationID, &departmentID, &metadataJSON, &entry.PostedAt, &entry.CreatedAt,
		&entry.UpdatedAt); err != nil {
		return err
	}
	entry.SourceID = uuidPtr(sourceID)
	entry.OrganizationID = uuidPtr(organizationID)
	entry.DepartmentID = uuidPtr(departmentID)
	return json.Unmarshal(metadataJSON, &entry.Metadata)
}

func scanGLJournalEntryLine(row scanner, line *GLJournalEntryLine) error {
	var metadataJSON []byte
	if err := row.Scan(&line.ID, &line.EntryID, &line.LineNum, &line.AccountCode,
		&line.AccountName, &line.CostCenterCode, &line.Debit, &line.Credit,
		&line.Description, &metadataJSON, &line.CreatedAt); err != nil {
		return err
	}
	return json.Unmarshal(metadataJSON, &line.Metadata)
}

func boolFromPtr(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func nullableDate(value *time.Time) any {
	if value == nil {
		return nil
	}
	return *value
}
