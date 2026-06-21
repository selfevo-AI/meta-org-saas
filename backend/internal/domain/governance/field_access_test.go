package governance

import "testing"

func TestApplyFieldAccessNullsDeniedFieldsAndReportsMetadata(t *testing.T) {
	record := map[string]any{
		"id":            "row-1",
		"name":          "Visible",
		"password_hash": "secret",
	}
	checks := map[string]FieldAccessCheckResult{
		"id":            {Allowed: true, Behavior: "allow"},
		"name":          {Allowed: true, Behavior: "allow"},
		"password_hash": {Allowed: false, Behavior: "deny", Reason: "sensitive field"},
	}

	filtered, meta := ApplyFieldAccess(record, checks)

	if filtered["password_hash"] != nil {
		t.Fatalf("password_hash = %#v, want nil", filtered["password_hash"])
	}
	if filtered["name"] != "Visible" {
		t.Fatalf("name = %#v, want Visible", filtered["name"])
	}
	if len(meta.DeniedFields) != 1 || meta.DeniedFields[0] != "password_hash" {
		t.Fatalf("denied fields = %#v, want password_hash", meta.DeniedFields)
	}
}
