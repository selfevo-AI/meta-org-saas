package erp

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestDecodeRecordCursorValidatesNumericValues(t *testing.T) {
	for _, test := range []struct {
		value string
		valid bool
	}{
		{"0", true}, {"141.25", true}, {"-0.000001", true},
		{"not-a-number", false}, {"NaN", false}, {"Infinity", false},
		{"1e100000", false}, {"1.2.3", false}, {" 1", false},
		{strings.Repeat("9", 128), false},
	} {
		t.Run(test.value[:min(20, len(test.value))], func(t *testing.T) {
			encoded, _ := json.Marshal(recordCursor{Key: "KEY", Value: &test.value, Signature: "query"})
			_, err := decodeRecordCursor(base64.RawURLEncoding.EncodeToString(encoded), "query", true)
			if test.valid && err != nil || !test.valid && !errors.Is(err, ErrValidation) {
				t.Fatalf("cursor value %q: %v", test.value, err)
			}
		})
	}
	encoded, _ := json.Marshal(recordCursor{Key: "KEY", Signature: "query"})
	if _, err := decodeRecordCursor(base64.RawURLEncoding.EncodeToString(encoded), "query", true); err != nil {
		t.Fatalf("null numeric cursor: %v", err)
	}
}
