package tenantdb

import (
	"testing"

	"github.com/google/uuid"
)

func TestSchemaNameForOrganizationUsesStableSafeIdentifier(t *testing.T) {
	id := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")

	got := SchemaNameForOrganization(id)
	want := "org_123e4567e89b12d3a456426614174000"

	if got != want {
		t.Fatalf("SchemaNameForOrganization() = %q, want %q", got, want)
	}
}

func TestQuoteIdentifierRejectsUnsafeNames(t *testing.T) {
	unsafe := []string{
		"",
		"123table",
		"table-name",
		"table;drop",
		"table name",
		"table.name",
	}

	for _, name := range unsafe {
		if _, err := QuoteIdentifier(name); err == nil {
			t.Fatalf("QuoteIdentifier(%q) succeeded, want error", name)
		}
	}
}

func TestSearchPathSQLIncludesOrganizationThenPlatformAndPublic(t *testing.T) {
	got, err := SearchPathSQL("org_123e4567e89b12d3a456426614174000")
	if err != nil {
		t.Fatalf("SearchPathSQL() error = %v", err)
	}
	want := `SET LOCAL search_path = "org_123e4567e89b12d3a456426614174000", "platform", "public"`
	if got != want {
		t.Fatalf("SearchPathSQL() = %q, want %q", got, want)
	}
}
