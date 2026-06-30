package erp

import "testing"

func TestDefaultCatalogIncludesERPBusinessHierarchy(t *testing.T) {
	catalog := DefaultCatalog()
	cases := []struct {
		module    string
		submodule string
		document  string
		table     string
		child     string
		action    string
	}{
		{"Project", "Requirements", "Requirement", "MREQ", "REQ1", "approve"},
		{"Project", "Projects", "Project", "MPRJ", "APRJ", "refresh-cost"},
		{"Purchasing", "Purchase Orders", "Purchase Order", "MPOR", "POR1", "approve"},
		{"Purchasing", "Goods Receipt PO", "Goods Receipt PO", "MPDN", "PDN1", "post"},
		{"Sales", "Sales Orders", "Sales Order", "MRDR", "RDR1", "confirm"},
		{"Sales", "A/R Invoices", "A/R Invoice", "MINV", "INV1", "post"},
		{"Inventory", "Goods Issues", "Goods Issue", "MIGE", "IGE1", "post"},
		{"Finance", "Chart of Accounts", "G/L Account", "MACT", "AACT", ""},
		{"Finance", "Cost Centers", "Cost Center", "MPRC", "APRC", ""},
		{"Finance", "Journal Entries", "Journal Entry", "MJDT", "JDT1", "post"},
		{"Finance", "Trial Balance", "Trial Balance", "MGLR", "", "run"},
		{"Master Data", "Items", "Item", "MITM", "ITM1", ""},
		{"Retail", "Stores", "Store", "MBRN", "BRN1", ""},
		{"Retail", "POS", "POS Sale", "MRPS", "RPS1", "close"},
		{"Retail", "Distribution", "Distribution Request", "MDRQ", "DRQ1", "auto-allocate"},
		{"Retail", "Distribution", "Distribution Shipment", "MDSP", "DSP1", "ship"},
		{"Retail", "Distribution", "Distribution Receipt", "MDRC", "DRC1", "receive"},
		{"Retail", "Distribution", "Distribution Difference", "MDIF", "DIF1", "resolve"},
		{"Retail", "Inventory Control", "Stock Policy", "MSTP", "STP1", "replenish"},
		{"Retail", "Inventory Control", "Store Count", "MCNT", "CNT1", "post-adjustment"},
		{"Retail", "Special Procurement", "Special Purchase Request", "MSPR", "SPR1", "convert-to-purchase-order"},
		{"Manufacturing", "BOM", "Bill of Materials", "MBOM", "BOM1", "make-work-order"},
		{"Manufacturing", "Work Orders", "Work Order", "MWOR", "WOR1", "complete"},
	}

	for _, tc := range cases {
		if !catalog.HasBusinessDocument(tc.module, tc.submodule, tc.document, tc.table, tc.child, tc.action) {
			t.Fatalf("catalog missing hierarchy %#v", tc)
		}
	}
}

func TestDefaultCatalogUsesERPCodeTablesForRetailInsteadOfSemanticSupplyChain(t *testing.T) {
	catalog := DefaultCatalog()
	semanticTables := []string{
		"inventory_counts",
		"inventory_transfers",
		"purchase_orders",
		"sales_shipments",
	}
	for _, table := range semanticTables {
		if _, ok := catalog.Table(table); ok {
			t.Fatalf("catalog contains semantic supply-chain table %s, want ERP code-table only", table)
		}
	}
	for _, table := range []string{"MCNT", "MDSP", "MDRC", "MPOR", "MDLN"} {
		if _, ok := catalog.Table(table); !ok {
			t.Fatalf("catalog missing ERP code-table %s", table)
		}
	}
}

func TestDefaultCatalogGLFieldsUseProjectCompatibleNames(t *testing.T) {
	catalog := DefaultCatalog()

	account, ok := catalog.Table("MACT")
	if !ok {
		t.Fatal("DefaultCatalog().Table(\"MACT\") missing")
	}
	for _, fieldName := range []string{"AcctCode", "Name", "AccountType", "Currency", "ParentAcctCode", "Postable", "Active"} {
		if _, ok := account.Field(fieldName); !ok {
			t.Fatalf("MACT missing field %s", fieldName)
		}
	}

	journal, ok := catalog.Table("MJDT")
	if !ok {
		t.Fatal("DefaultCatalog().Table(\"MJDT\") missing")
	}
	for _, fieldName := range []string{"TransId", "ReferenceDate", "Memo", "Status", "Currency", "PostedAt"} {
		if _, ok := journal.Field(fieldName); !ok {
			t.Fatalf("MJDT missing field %s", fieldName)
		}
	}
	line, ok := journal.Child("JDT1")
	if !ok {
		t.Fatal("MJDT child JDT1 missing")
	}
	for _, fieldName := range []string{"LineNum", "AccountCode", "Debit", "Credit", "CostCenterCode"} {
		if _, ok := line.Field(fieldName); !ok {
			t.Fatalf("JDT1 missing field %s", fieldName)
		}
	}
}
