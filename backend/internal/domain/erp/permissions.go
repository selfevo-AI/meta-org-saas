package erp

import (
	"context"
	"fmt"

	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
)

type approvedActionKey struct{}

// WithApprovedAction is used only by the Tool Runtime after its approval gate.
// No HTTP request field can create this capability.
func WithApprovedAction(ctx context.Context) context.Context {
	return context.WithValue(ctx, approvedActionKey{}, true)
}

func ModuleForTable(table TableDefinition) string {
	switch table.Code {
	case "MINV", "MPCH", "MRCT", "MVPM", "MRIN", "MRPC":
		return "finance"
	case "MPRJ":
		return "project"
	}
	switch table.Module {
	case "sale":
		return "sales"
	case "purchase":
		return "procurement"
	case "warehouse", "product", "partner":
		return "inventory"
	default:
		return table.Module
	}
}

func (s *Service) authorize(ctx context.Context, tableCode, operation string) error {
	table, err := s.table(tableCode)
	if err != nil {
		return err
	}
	if operation != "read" && archivedIndustryTable(table) {
		return fmt.Errorf("%w: legacy industry records are read only; use the operational ontology", ErrForbidden)
	}
	tenant, ok := middleware.TenantFromContext(ctx)
	if !ok {
		if _, authenticated := middleware.UserFromContext(ctx); authenticated {
			return fmt.Errorf("%w: tenant context is required", ErrForbidden)
		}
		return nil
	}
	if tenant.Mode == "saas" {
		if tenant.TenantDatabaseDeploymentMode == "shared_schema" {
			return fmt.Errorf("%w: ERP requires a dedicated tenant database", ErrForbidden)
		}
		module := ModuleForTable(table)
		if module != "platform" && module != "user" && !tenant.EnabledModules[module] {
			return fmt.Errorf("%w: %s module is disabled", ErrForbidden, module)
		}
	}
	if operation == "read" {
		return nil
	}
	if table.Module == "platform" || table.Module == "user" {
		return fmt.Errorf("%w: platform projections are read only", ErrForbidden)
	}
	if approved, _ := ctx.Value(approvedActionKey{}).(bool); approved {
		return nil
	}
	if user, ok := middleware.UserFromContext(ctx); ok && user.Type != "human" {
		return fmt.Errorf("%w: agent changes require an approved business action", ErrForbidden)
	}
	switch tenant.AuthorityTier {
	case "organization_creator", "organization_admin", "reviewer":
		return nil
	case "executor":
		switch operation {
		case "create", "update", "delete", "submit", "confirm", "receive", "deliver", "run", "analyze":
			return nil
		}
	}
	return fmt.Errorf("%w: reviewer authority is required for this action", ErrForbidden)
}

func validateDraftInput(table TableDefinition, input RecordInput) error {
	if table.Code == "MITW" {
		return fmt.Errorf("%w: stock balances are maintained by inventory actions", ErrValidation)
	}
	for _, name := range []string{"Posted", "PostedAt", "LegacyID", "LegacyMasterKey", "BtfStatus", "WddStatus", "PaidToDate", "AllocatedAmount", "InventoryValue", "provenance", "FulfillmentEntry", "FulfillmentTable", "InvoiceEntry", "JournalEntry", "OpenBal", "BaseEntry", "BaseTable", "Confirmed"} {
		if value, ok := input.Data[name]; ok {
			if name == "Posted" && (value == "N" || value == false) ||
				name == "BtfStatus" && value == "O" || name == "WddStatus" && (value == "W" || value == "N" || value == "-") ||
				name == "PaidToDate" && numericValue(input.Data, name) == 0 {
				continue
			}
			return fmt.Errorf("%w: %s is maintained by business actions", ErrValidation, name)
		}
	}
	if status := stringValue(input.Data, "DocStatus", "O"); status != "O" && status != "draft" {
		return fmt.Errorf("%w: document status is maintained by business actions", ErrValidation)
	}
	return nil
}

func (s *Service) authorizeActionDependencies(ctx context.Context, tableCode, action string) error {
	dependencies := map[string][]string{
		"MPDN:post": {"MITW", "MJDT", "MPCH"}, "MDLN:post": {"MITW", "MJDT", "MINV"},
		"MIGN:post": {"MJDT"}, "MIGE:post": {"MJDT"},
	}
	for _, table := range dependencies[tableCode+":"+action] {
		if err := s.authorize(ctx, table, "read"); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ensureEditable(ctx context.Context, table TableDefinition, key string) error {
	if table.Code == "MITW" {
		return fmt.Errorf("%w: stock balances are maintained by inventory actions", ErrValidation)
	}
	record, err := s.repo.GetRecord(ctx, table, key)
	if err != nil {
		return err
	}
	if isPostedDocument(record) || documentFieldEquals(record, "BtfStatus", "P") || documentFieldEquals(record, "BtfStatus", "V") || stringValue(record.Data, "BaseEntry", "") != "" ||
		documentFieldEquals(record, "WddStatus", "A") || isClosedDocument(record) ||
		numericValue(record.Data, "AllocatedAmount") > 0 {
		return fmt.Errorf("%w: approved, posted, or void documents are immutable", ErrConflict)
	}
	return nil
}

func archivedIndustryTable(table TableDefinition) bool {
	return table.Module == "retail" || table.Module == "manufacturing"
}

func (s *Service) recordInTx(ctx context.Context, fn func(*Service) (*Record, error)) (*Record, error) {
	var record *Record
	_, err := s.runInTx(ctx, func(tx *Service) (*ActionResult, error) {
		var err error
		record, err = fn(tx)
		return nil, err
	})
	return record, err
}
