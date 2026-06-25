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
	}

	for _, tc := range cases {
		if !catalog.HasBusinessDocument(tc.module, tc.submodule, tc.document, tc.table, tc.child, tc.action) {
			t.Fatalf("catalog missing hierarchy %#v", tc)
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
