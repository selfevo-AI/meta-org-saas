package assistant

import "testing"

func TestERPContextTargetsIncludeBusinessObjects(t *testing.T) {
	targets := erpContextTargetsForCatalog([]string{"MREQ", "MPRJ", "MPOR", "MRDR", "MINV"})
	want := []string{"requirement", "project", "purchase_order", "sales_order", "ar_invoice"}
	for _, key := range want {
		if !containsContextTarget(targets, key) {
			t.Fatalf("missing ERP context target %s in %#v", key, targets)
		}
	}
}

func TestERPProposalTargetUsesTableCodeKeyAndActionPayload(t *testing.T) {
	target, ok, err := erpProposalTargetFromPayload(&Proposal{
		ModuleKey:  "erp",
		TargetType: "erp_action",
		Payload: map[string]any{
			"table_code": "MREQ",
			"key":        "REQ-1",
			"action":     "approve",
		},
	})
	if err != nil {
		t.Fatalf("erpProposalTargetFromPayload returned error: %v", err)
	}
	if !ok {
		t.Fatal("erpProposalTargetFromPayload ok = false, want true")
	}
	if target.tableCode != "MREQ" || target.primaryKey != "ReqCode" || target.key != "REQ-1" || target.action != "approve" {
		t.Fatalf("target = %#v, want MREQ/ReqCode/REQ-1/approve", target)
	}
}

func containsContextTarget(targets []string, key string) bool {
	for _, target := range targets {
		if target == key {
			return true
		}
	}
	return false
}
