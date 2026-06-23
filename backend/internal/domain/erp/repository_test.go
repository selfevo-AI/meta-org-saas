package erp

import "testing"

func TestPostgresColumnTypeMapsDocumentTypes(t *testing.T) {
	cases := []struct {
		field FieldDefinition
		want  string
	}{
		{field("DocEntry", "Int", "11", "", true), "BIGINT"},
		{field("CardName", "nVarChar", "100", "", false), "VARCHAR(100)"},
		{field("DocTotal", "Numeric", "19,6", "", false), "NUMERIC(19,6)"},
		{field("DocDate", "Date", "", "", false), "DATE"},
		{field("Payload", "JSONB", "", "", false), "JSONB"},
	}

	for _, tc := range cases {
		got := postgresColumnType(tc.field)
		if got != tc.want {
			t.Fatalf("postgresColumnType(%#v) = %q, want %q", tc.field, got, tc.want)
		}
	}
}
