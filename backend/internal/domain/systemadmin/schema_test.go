package systemadmin

import (
	"strings"
	"testing"
)

func TestValidateSchemaPackageRequiresMasterDetailTables(t *testing.T) {
	pkg := SchemaPackage{
		FormatVersion: "meta-org.schema.v1",
		ModuleKey:     "organization",
		Tables: []SchemaTableDefinition{
			{
				Name: "custom_records",
				Fields: []SchemaFieldDefinition{
					{Name: "id", DataType: "uuid", PrimaryKey: true},
				},
			},
		},
	}

	err := ValidateSchemaPackage(pkg)
	if err == nil {
		t.Fatal("ValidateSchemaPackage() succeeded, want error")
	}
	if !strings.Contains(err.Error(), "organization_masters") {
		t.Fatalf("ValidateSchemaPackage() error = %q, want organization_masters requirement", err.Error())
	}
}

func TestValidateSchemaPackageRejectsUnsafeFieldType(t *testing.T) {
	pkg := validOrganizationPackage()
	pkg.Tables[0].Fields[2].DataType = "text); drop table users; --"

	err := ValidateSchemaPackage(pkg)
	if err == nil {
		t.Fatal("ValidateSchemaPackage() succeeded, want error")
	}
	if !strings.Contains(err.Error(), "unsupported data type") {
		t.Fatalf("ValidateSchemaPackage() error = %q, want unsupported data type", err.Error())
	}
}

func TestBuildCreateTableStatementsUsesQuotedSchemaAndNonDestructiveDDL(t *testing.T) {
	statements, err := BuildCreateTableStatements("org_123e4567e89b12d3a456426614174000", validOrganizationPackage())
	if err != nil {
		t.Fatalf("BuildCreateTableStatements() error = %v", err)
	}
	if len(statements) != 2 {
		t.Fatalf("BuildCreateTableStatements() returned %d statements, want 2", len(statements))
	}
	for _, stmt := range statements {
		if !strings.Contains(stmt, `CREATE TABLE IF NOT EXISTS "org_123e4567e89b12d3a456426614174000".`) {
			t.Fatalf("statement does not target quoted org schema: %s", stmt)
		}
		if strings.Contains(strings.ToLower(stmt), " drop ") || strings.Contains(strings.ToLower(stmt), "alter table") {
			t.Fatalf("statement contains destructive DDL: %s", stmt)
		}
	}
	if !strings.Contains(statements[0], `"organization_masters"`) {
		t.Fatalf("first statement = %s, want organization_masters", statements[0])
	}
}

func validOrganizationPackage() SchemaPackage {
	return SchemaPackage{
		FormatVersion: "meta-org.schema.v1",
		ModuleKey:     "organization",
		Tables: []SchemaTableDefinition{
			{
				Name: "organization_masters",
				Fields: []SchemaFieldDefinition{
					{Name: "master_key", DataType: "text", PrimaryKey: true},
					{Name: "entity_type", DataType: "text", Nullable: false},
					{Name: "payload", DataType: "jsonb", Nullable: false, Default: "'{}'::jsonb"},
				},
			},
			{
				Name: "organization_details",
				Fields: []SchemaFieldDefinition{
					{Name: "detail_key", DataType: "text", PrimaryKey: true},
					{Name: "master_key", DataType: "text", Nullable: false},
					{Name: "detail_type", DataType: "text", Nullable: false},
					{Name: "payload", DataType: "jsonb", Nullable: false, Default: "'{}'::jsonb"},
				},
			},
		},
	}
}
