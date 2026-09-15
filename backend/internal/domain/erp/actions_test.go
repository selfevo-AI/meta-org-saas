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
	}

	for _, tc := range cases {
		if _, ok := registry.Lookup(tc.table, tc.action); !ok {
			t.Fatalf("registry missing %s/%s", tc.table, tc.action)
		}
	}
}
