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

func TestBuildSchemaMigrationPlanDetectsFullSchemaChanges(t *testing.T) {
	current := validOrganizationPackage()
	current.Tables[0].Fields = append(current.Tables[0].Fields,
		SchemaFieldDefinition{Name: "legacy_code", DataType: "text", Nullable: true},
		SchemaFieldDefinition{Name: "display_name", DataType: "text", Nullable: true},
	)
	current.Tables = append(current.Tables, SchemaTableDefinition{
		Name: "obsolete_table",
		Fields: []SchemaFieldDefinition{
			{Name: "id", DataType: "uuid", PrimaryKey: true},
		},
	})

	desired := validOrganizationPackage()
	desired.Tables[0].Fields = append(desired.Tables[0].Fields,
		SchemaFieldDefinition{Name: "external_code", DataType: "text", Nullable: true, PreviousName: "legacy_code"},
		SchemaFieldDefinition{Name: "display_name", DataType: "varchar(255)", Nullable: false, Default: "''"},
		SchemaFieldDefinition{Name: "search_vector", DataType: "text", Nullable: true},
	)
	desired.Tables[0].Indexes = []SchemaIndexDefinition{{Name: "idx_organization_masters_external_code", Fields: []string{"external_code"}}}
	desired.Tables = append(desired.Tables, SchemaTableDefinition{
		Name: "audit_events",
		Fields: []SchemaFieldDefinition{
			{Name: "id", DataType: "uuid", PrimaryKey: true, Default: "gen_random_uuid()"},
			{Name: "payload", DataType: "jsonb", Nullable: false, Default: "'{}'::jsonb"},
		},
	})

	plan, err := BuildSchemaMigrationPlan("org_123e4567e89b12d3a456426614174000", current, desired)
	if err != nil {
		t.Fatalf("BuildSchemaMigrationPlan error = %v", err)
	}
	joined := strings.Join(plan.Statements, "\n")
	expected := []string{
		`CREATE TABLE IF NOT EXISTS "org_123e4567e89b12d3a456426614174000"."audit_events"`,
		`ALTER TABLE "org_123e4567e89b12d3a456426614174000"."organization_masters" RENAME COLUMN "legacy_code" TO "external_code"`,
		`ALTER TABLE "org_123e4567e89b12d3a456426614174000"."organization_masters" ALTER COLUMN "display_name" TYPE VARCHAR(255)`,
		`ALTER TABLE "org_123e4567e89b12d3a456426614174000"."organization_masters" ADD COLUMN "search_vector" TEXT`,
		`DROP TABLE "org_123e4567e89b12d3a456426614174000"."obsolete_table"`,
		`CREATE INDEX IF NOT EXISTS "idx_organization_masters_external_code"`,
	}
	for _, snippet := range expected {
		if !strings.Contains(joined, snippet) {
			t.Fatalf("migration statements missing %q\nstatements:\n%s", snippet, joined)
		}
	}
	if plan.RiskLevel != SchemaRiskDestructive {
		t.Fatalf("risk level = %q, want %q", plan.RiskLevel, SchemaRiskDestructive)
	}
	if len(plan.Diff) == 0 {
		t.Fatalf("migration diff is empty")
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
