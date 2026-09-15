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
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/erp"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
)

func (r *PostgresRepository) CreateGLAccount(ctx context.Context, input CreateGLAccountInput) (*GLAccount, error) {
	_, err := r.canonicalLedger().CreateRecord(ctx, "MACT", erp.RecordInput{Key: input.AccountCode, Data: map[string]any{
		"Name": input.Name, "AccountType": input.AccountType, "Currency": input.Currency,
		"ParentAcctCode": input.ParentAccountCode, "Postable": glFlag(input.Postable), "Active": glFlag(input.Active),
		"OrganizationID": input.OrganizationID, "DepartmentID": input.DepartmentID, "Metadata": input.Metadata,
	}})
	if err != nil {
		return nil, financeLedgerError(err)
	}
	account := &GLAccount{}
	err = scanGLAccount(r.db.QueryRow(ctx, `
  SELECT id, master_key, account_code, name, account_type, currency, parent_account_code,
    postable, active, organization_id, department_id, metadata, created_at, updated_at
  FROM erp_gl_accounts WHERE account_code = $1`, input.AccountCode), account)
	return account, err
}

func (r *PostgresRepository) ListGLAccounts(ctx context.Context, limit int) ([]GLAccount, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, master_key, account_code, name, account_type, currency, parent_account_code,
		       postable, active, organization_id, department_id, metadata, created_at, updated_at
		FROM erp_gl_accounts
		WHERE ($1::uuid IS NULL OR organization_id IS NULL OR organization_id = $1)
		ORDER BY account_code
		LIMIT $2
	`, nullableUUID(currentTenantOrganizationID(ctx)), normalizeLimit(limit))
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
	_, err := r.canonicalLedger().CreateRecord(ctx, "MPRC", erp.RecordInput{Key: input.CostCenterCode, Data: map[string]any{
		"Name": input.Name, "Active": glFlag(input.Active), "OrganizationID": input.OrganizationID,
		"DepartmentID": input.DepartmentID, "Metadata": input.Metadata,
	}})
	if err != nil {
		return nil, financeLedgerError(err)
	}
	center := &GLCostCenter{}
	err = scanGLCostCenter(r.db.QueryRow(ctx, `
  SELECT id, master_key, cost_center_code, name, active, organization_id, department_id,
    metadata, created_at, updated_at FROM erp_gl_cost_centers WHERE cost_center_code = $1`, input.CostCenterCode), center)
	return center, err
}

func (r *PostgresRepository) ListGLCostCenters(ctx context.Context, limit int) ([]GLCostCenter, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, master_key, cost_center_code, name, active, organization_id, department_id,
		       metadata, created_at, updated_at
		FROM erp_gl_cost_centers
		WHERE ($1::uuid IS NULL OR organization_id IS NULL OR organization_id = $1)
		ORDER BY cost_center_code
		LIMIT $2
	`, nullableUUID(currentTenantOrganizationID(ctx)), normalizeLimit(limit))
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
	key := input.EntryNumber
	if key == "" {
		key = "GL-" + uuid.NewString()
	}
	repo := erp.NewRepository(r.db)
	err := repo.RunInTx(ctx, func(tx erp.Repository) error {
		ledger := erp.NewService(tx, erp.DefaultCatalog())
		_, err := ledger.CreateRecord(ctx, "MJDT", erp.RecordInput{Key: key, Data: map[string]any{
			"EntryNumber": input.EntryNumber, "RefDate": referenceDate.Format("2006-01-02"), "Memo": input.Memo,
			"Currency": input.Currency, "SourceType": input.SourceType, "SourceID": input.SourceID,
			"OrganizationID": input.OrganizationID, "DepartmentID": input.DepartmentID, "Metadata": input.Metadata,
		}})
		if err != nil {
			return err
		}
		for index, line := range input.Lines {
			_, err := ledger.CreateChildRecord(ctx, "MJDT", key, "JDT1", erp.RecordInput{Key: fmt.Sprint(index + 1), Data: map[string]any{
				"AccountCode": line.AccountCode, "AccountName": line.AccountName, "CostCenterCode": line.CostCenterCode,
				"Debit": line.Debit, "Credit": line.Credit, "Description": line.Description, "Metadata": line.Metadata,
			}})
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, financeLedgerError(err)
	}
	var id uuid.UUID
	if err := r.db.QueryRow(ctx, "SELECT id FROM erp_gl_journal_entries WHERE master_key = $1", key).Scan(&id); err != nil {
		return nil, err
	}
	return r.GetGLJournalEntry(ctx, id)
}

func (r *PostgresRepository) ListGLJournalEntries(ctx context.Context, limit int) ([]GLJournalEntry, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, master_key, entry_number, reference_date, memo, status, currency, source_type,
		       source_id, organization_id, department_id, metadata, posted_at, created_at, updated_at
		FROM erp_gl_journal_entries
		WHERE ($1::uuid IS NULL OR organization_id IS NULL OR organization_id = $1)
		ORDER BY reference_date DESC, created_at DESC
		LIMIT $2
	`, nullableUUID(currentTenantOrganizationID(ctx)), normalizeLimit(limit))
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
		FROM erp_gl_journal_entries
		WHERE id = $1
		  AND ($2::uuid IS NULL OR organization_id IS NULL OR organization_id = $2)
	`, id, nullableUUID(currentTenantOrganizationID(ctx))), entry)
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
	entry, err := r.GetGLJournalEntry(ctx, id)
	if err != nil {
		return nil, err
	}
	input := erp.ActionInput{Source: "finance_api"}
	if user, ok := middleware.UserFromContext(ctx); ok {
		actorID, err := uuid.Parse(user.ID)
		if err != nil {
			return nil, ErrForbidden
		}
		input.ActorID, input.ActorType = &actorID, user.Type
	}
	if _, err := r.canonicalLedger().RunAction(ctx, "MJDT", entry.MasterKey, "post", input); err != nil {
		return nil, financeLedgerError(err)
	}
	return r.GetGLJournalEntry(ctx, id)
}

func (r *PostgresRepository) GetGLTrialBalance(ctx context.Context, input GLTrialBalanceInput) (*GLTrialBalance, error) {
	result, err := r.canonicalLedger().TrialBalance(ctx, erp.TrialBalanceInput{PeriodStart: input.PeriodStart, PeriodEnd: input.PeriodEnd, Currency: input.Currency})
	if err != nil {
		return nil, financeLedgerError(err)
	}
	balance := &GLTrialBalance{Rows: []GLTrialBalanceRow{}, Currency: result.Currency, TotalDebit: result.TotalDebit, TotalCredit: result.TotalCredit}
	for _, row := range result.Rows {
		balance.Rows = append(balance.Rows, GLTrialBalanceRow{AccountCode: row.AccountCode, AccountName: row.AccountName, Debit: row.Debit, Credit: row.Credit, NetAmount: row.NetAmount})
	}
	return balance, nil
}

func (r *PostgresRepository) canonicalLedger() *erp.Service {
	return erp.NewService(erp.NewRepository(r.db), erp.DefaultCatalog())
}

func glFlag(value *bool) string {
	if value != nil && !*value {
		return "N"
	}
	return "Y"
}

func financeLedgerError(err error) error {
	switch {
	case errors.Is(err, erp.ErrValidation), errors.Is(err, erp.ErrConflict):
		return fmt.Errorf("%w: %v", ErrValidation, err)
	case errors.Is(err, erp.ErrForbidden):
		return fmt.Errorf("%w: %v", ErrForbidden, err)
	case errors.Is(err, erp.ErrNotFound):
		return ErrNotFound
	default:
		return err
	}
}

func (r *PostgresRepository) listGLJournalEntryLines(ctx context.Context, entryID uuid.UUID) ([]GLJournalEntryLine, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, entry_id, line_num, account_code, account_name, cost_center_code,
		       debit::float8, credit::float8, description, metadata, created_at
		FROM erp_gl_journal_entry_lines
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
