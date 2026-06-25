package finance

import (
	"time"

	"github.com/google/uuid"
)

type GLAccount struct {
	ID                uuid.UUID      `json:"id"`
	MasterKey         string         `json:"master_key"`
	AccountCode       string         `json:"account_code"`
	Name              string         `json:"name"`
	AccountType       string         `json:"account_type"`
	Currency          string         `json:"currency"`
	ParentAccountCode string         `json:"parent_account_code"`
	Postable          bool           `json:"postable"`
	Active            bool           `json:"active"`
	OrganizationID    *uuid.UUID     `json:"organization_id,omitempty"`
	DepartmentID      *uuid.UUID     `json:"department_id,omitempty"`
	Metadata          map[string]any `json:"metadata"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

type GLAccountInput struct {
	AccountCode       string         `json:"account_code"`
	Name              string         `json:"name"`
	AccountType       string         `json:"account_type,omitempty"`
	Currency          string         `json:"currency,omitempty"`
	ParentAccountCode string         `json:"parent_account_code,omitempty"`
	Postable          *bool          `json:"postable,omitempty"`
	Active            *bool          `json:"active,omitempty"`
	OrganizationID    *uuid.UUID     `json:"organization_id,omitempty"`
	DepartmentID      *uuid.UUID     `json:"department_id,omitempty"`
	Metadata          map[string]any `json:"metadata,omitempty"`
}

type CreateGLAccountInput = GLAccountInput

type GLCostCenter struct {
	ID             uuid.UUID      `json:"id"`
	MasterKey      string         `json:"master_key"`
	CostCenterCode string         `json:"cost_center_code"`
	Name           string         `json:"name"`
	Active         bool           `json:"active"`
	OrganizationID *uuid.UUID     `json:"organization_id,omitempty"`
	DepartmentID   *uuid.UUID     `json:"department_id,omitempty"`
	Metadata       map[string]any `json:"metadata"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type GLCostCenterInput struct {
	CostCenterCode string         `json:"cost_center_code"`
	Name           string         `json:"name"`
	Active         *bool          `json:"active,omitempty"`
	OrganizationID *uuid.UUID     `json:"organization_id,omitempty"`
	DepartmentID   *uuid.UUID     `json:"department_id,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type CreateGLCostCenterInput = GLCostCenterInput

type GLJournalEntry struct {
	ID             uuid.UUID            `json:"id"`
	MasterKey      string               `json:"master_key"`
	EntryNumber    string               `json:"entry_number"`
	ReferenceDate  time.Time            `json:"reference_date"`
	Memo           string               `json:"memo"`
	Status         string               `json:"status"`
	Currency       string               `json:"currency"`
	SourceType     string               `json:"source_type"`
	SourceID       *uuid.UUID           `json:"source_id,omitempty"`
	OrganizationID *uuid.UUID           `json:"organization_id,omitempty"`
	DepartmentID   *uuid.UUID           `json:"department_id,omitempty"`
	Metadata       map[string]any       `json:"metadata"`
	Lines          []GLJournalEntryLine `json:"lines,omitempty"`
	PostedAt       *time.Time           `json:"posted_at,omitempty"`
	CreatedAt      time.Time            `json:"created_at"`
	UpdatedAt      time.Time            `json:"updated_at"`
}

type GLJournalEntryLine struct {
	ID             uuid.UUID      `json:"id"`
	EntryID        uuid.UUID      `json:"entry_id"`
	LineNum        int            `json:"line_num"`
	AccountCode    string         `json:"account_code"`
	AccountName    string         `json:"account_name"`
	CostCenterCode string         `json:"cost_center_code"`
	Debit          float64        `json:"debit"`
	Credit         float64        `json:"credit"`
	Description    string         `json:"description"`
	Metadata       map[string]any `json:"metadata"`
	CreatedAt      time.Time      `json:"created_at"`
}

type CreateGLJournalEntryInput struct {
	EntryNumber    string                          `json:"entry_number,omitempty"`
	ReferenceDate  string                          `json:"reference_date"`
	Memo           string                          `json:"memo,omitempty"`
	Currency       string                          `json:"currency,omitempty"`
	SourceType     string                          `json:"source_type,omitempty"`
	SourceID       *uuid.UUID                      `json:"source_id,omitempty"`
	OrganizationID *uuid.UUID                      `json:"organization_id,omitempty"`
	DepartmentID   *uuid.UUID                      `json:"department_id,omitempty"`
	Metadata       map[string]any                  `json:"metadata,omitempty"`
	Lines          []CreateGLJournalEntryLineInput `json:"lines"`
}

type CreateGLJournalEntryLineInput struct {
	AccountCode    string         `json:"account_code"`
	AccountName    string         `json:"account_name,omitempty"`
	CostCenterCode string         `json:"cost_center_code,omitempty"`
	Debit          float64        `json:"debit,omitempty"`
	Credit         float64        `json:"credit,omitempty"`
	Description    string         `json:"description,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type GLTrialBalanceInput struct {
	OrganizationID *uuid.UUID `json:"organization_id,omitempty"`
	PeriodStart    string     `json:"period_start,omitempty"`
	PeriodEnd      string     `json:"period_end,omitempty"`
	Currency       string     `json:"currency,omitempty"`

	periodStartTime *time.Time
	periodEndTime   *time.Time
}

type GLTrialBalance struct {
	Rows        []GLTrialBalanceRow `json:"rows"`
	TotalDebit  float64             `json:"total_debit"`
	TotalCredit float64             `json:"total_credit"`
	Currency    string              `json:"currency"`
}

type GLTrialBalanceRow struct {
	AccountCode string  `json:"account_code"`
	AccountName string  `json:"account_name"`
	Debit       float64 `json:"debit"`
	Credit      float64 `json:"credit"`
	NetAmount   float64 `json:"net_amount"`
}
