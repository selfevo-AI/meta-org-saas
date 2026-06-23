package industry

import "testing"

func TestJSONBytesEncodesNilMapAsObject(t *testing.T) {
	var metadata map[string]any

	got := string(jsonBytes(metadata))

	if got != "{}" {
		t.Fatalf("jsonBytes(nil map) = %s, want {}", got)
	}
}
