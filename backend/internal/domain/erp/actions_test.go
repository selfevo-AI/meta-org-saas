package erp

import "testing"

func TestDefaultActionRegistryIncludesBusinessActions(t *testing.T) {
	registry := DefaultActionRegistry()
	cases := []struct {
		table  string
		action string
	}{
		{"MREQ", "approve"},
		{"MREQ", "convert-to-project"},
		{"MPOR", "submit"},
		{"MPOR", "approve"},
		{"MPDN", "post"},
		{"MRDR", "confirm"},
		{"MRDR", "approve"},
		{"MDLN", "post"},
		{"MINV", "post"},
		{"MRCT", "allocate"},
		{"MIGN", "post"},
		{"MIGE", "post"},
		{"MJDT", "post"},
		{"MRPS", "close"},
		{"MDRQ", "submit"},
		{"MDRQ", "approve"},
		{"MDRQ", "auto-allocate"},
		{"MDSP", "ship"},
		{"MDRC", "receive"},
		{"MDIF", "resolve"},
		{"MSTP", "replenish"},
		{"MCNT", "submit"},
		{"MCNT", "approve"},
		{"MCNT", "post-adjustment"},
		{"MSPR", "submit"},
		{"MSPR", "approve"},
		{"MSPR", "convert-to-purchase-order"},
	}

	for _, tc := range cases {
		if _, ok := registry.Lookup(tc.table, tc.action); !ok {
			t.Fatalf("registry missing %s/%s", tc.table, tc.action)
		}
	}
}
