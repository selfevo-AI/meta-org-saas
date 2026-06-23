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
		{"Finance", "Journal Entries", "Journal Entry", "MJDT", "JDT1", "post"},
		{"Master Data", "Items", "Item", "MITM", "ITM1", ""},
	}

	for _, tc := range cases {
		if !catalog.HasBusinessDocument(tc.module, tc.submodule, tc.document, tc.table, tc.child, tc.action) {
			t.Fatalf("catalog missing hierarchy %#v", tc)
		}
	}
}
