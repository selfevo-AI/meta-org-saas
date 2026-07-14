package systemadmin

import (
	"strings"
	"testing"
)

func TestValidateIndustrySolutionManifestRequiresMasterDetailTables(t *testing.T) {
	manifest := IndustrySolutionManifest{
		FormatVersion: IndustrySolutionManifestFormatVersion,
		ModuleKey:     "organization",
		Tables: []IndustrySolutionTableDefinition{
			{
				Name: "custom_records",
				Fields: []IndustrySolutionFieldDefinition{
					{Name: "id", DataType: "uuid", PrimaryKey: true},
				},
			},
		},
	}

	err := ValidateIndustrySolutionManifest(manifest)
	if err == nil {
		t.Fatal("ValidateIndustrySolutionManifest() succeeded, want error")
	}
	if !strings.Contains(err.Error(), "organization_masters") {
		t.Fatalf("ValidateIndustrySolutionManifest() error = %q, want organization_masters requirement", err.Error())
	}
}

func TestValidateIndustrySolutionManifestRejectsUnsafeFieldType(t *testing.T) {
	manifest := validOrganizationManifest()
	manifest.Tables[0].Fields[2].DataType = "text); drop table users; --"

	err := ValidateIndustrySolutionManifest(manifest)
	if err == nil {
		t.Fatal("ValidateIndustrySolutionManifest() succeeded, want error")
	}
	if !strings.Contains(err.Error(), "unsupported data type") {
		t.Fatalf("ValidateIndustrySolutionManifest() error = %q, want unsupported data type", err.Error())
	}
}

func TestSafeIndexPredicate(t *testing.T) {
	safe := []string{
		"status = 'active'",
		"finance_payable_id is not null",
		"deleted_at IS NULL AND status <> 'void'",
		"(amount > 0 or credit > 0)",
		"created_at >= '2026-01-01'",
	}
	for _, expr := range safe {
		if !safeIndexPredicate(expr) {
			t.Fatalf("safeIndexPredicate(%q) = false, want true", expr)
		}
	}
	unsafe := []string{
		"",
		"true); drop table platform_users; --",
		"status = 'x' ; delete from users",
		"1=1 union select secret from platform_users",
		"pg_sleep(10) is null",
		"status = 'a' /* comment */",
		"col = $1",
	}
	for _, expr := range unsafe {
		if safeIndexPredicate(expr) {
			t.Fatalf("safeIndexPredicate(%q) = true, want false", expr)
		}
	}
}

func TestValidateIndustrySolutionManifestRejectsUnsafeIndexPredicate(t *testing.T) {
	manifest := validOrganizationManifest()
	manifest.Tables[0].Indexes = append(manifest.Tables[0].Indexes, IndustrySolutionIndexDefinition{
		Name:   "idx_injection",
		Fields: []string{manifest.Tables[0].Fields[0].Name},
		Where:  "true); drop table platform_users; --",
	})

	err := ValidateIndustrySolutionManifest(manifest)
	if err == nil {
		t.Fatal("ValidateIndustrySolutionManifest() succeeded, want error")
	}
	if !strings.Contains(err.Error(), "unsafe index predicate") {
		t.Fatalf("ValidateIndustrySolutionManifest() error = %q, want unsafe index predicate", err.Error())
	}
}

func TestBuildIndustrySolutionTableStatementsUsesQuotedSchemaAndNonDestructiveDDL(t *testing.T) {
	statements, err := BuildIndustrySolutionTableStatements("org_123e4567e89b12d3a456426614174000", validOrganizationManifest())
	if err != nil {
		t.Fatalf("BuildIndustrySolutionTableStatements() error = %v", err)
	}
	if len(statements) != 2 {
		t.Fatalf("BuildIndustrySolutionTableStatements() returned %d statements, want 2", len(statements))
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

func TestBuildIndustrySolutionMigrationPlanDetectsFullChanges(t *testing.T) {
	current := validOrganizationManifest()
	current.Tables[0].Fields = append(current.Tables[0].Fields,
		IndustrySolutionFieldDefinition{Name: "legacy_code", DataType: "text", Nullable: true},
		IndustrySolutionFieldDefinition{Name: "display_name", DataType: "text", Nullable: true},
	)
	current.Tables = append(current.Tables, IndustrySolutionTableDefinition{
		Name: "obsolete_table",
		Fields: []IndustrySolutionFieldDefinition{
			{Name: "id", DataType: "uuid", PrimaryKey: true},
		},
	})

	desired := validOrganizationManifest()
	desired.Tables[0].Fields = append(desired.Tables[0].Fields,
		IndustrySolutionFieldDefinition{Name: "external_code", DataType: "text", Nullable: true, PreviousName: "legacy_code"},
		IndustrySolutionFieldDefinition{Name: "display_name", DataType: "varchar(255)", Nullable: false, Default: "''"},
		IndustrySolutionFieldDefinition{Name: "search_vector", DataType: "text", Nullable: true},
	)
	desired.Tables[0].Indexes = []IndustrySolutionIndexDefinition{{Name: "idx_organization_masters_external_code", Fields: []string{"external_code"}}}
	desired.Tables = append(desired.Tables, IndustrySolutionTableDefinition{
		Name: "audit_events",
		Fields: []IndustrySolutionFieldDefinition{
			{Name: "id", DataType: "uuid", PrimaryKey: true, Default: "gen_random_uuid()"},
			{Name: "payload", DataType: "jsonb", Nullable: false, Default: "'{}'::jsonb"},
		},
	})

	plan, err := BuildIndustrySolutionMigrationPlan("org_123e4567e89b12d3a456426614174000", current, desired)
	if err != nil {
		t.Fatalf("BuildIndustrySolutionMigrationPlan error = %v", err)
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
	if plan.RiskLevel != IndustrySolutionRiskDestructive {
		t.Fatalf("risk level = %q, want %q", plan.RiskLevel, IndustrySolutionRiskDestructive)
	}
	if len(plan.Diff) == 0 {
		t.Fatalf("migration diff is empty")
	}
}

func validOrganizationManifest() IndustrySolutionManifest {
	return IndustrySolutionManifest{
		FormatVersion: IndustrySolutionManifestFormatVersion,
		ModuleKey:     "organization",
		Tables: []IndustrySolutionTableDefinition{
			{
				Name: "organization_masters",
				Fields: []IndustrySolutionFieldDefinition{
					{Name: "master_key", DataType: "text", PrimaryKey: true},
					{Name: "entity_type", DataType: "text", Nullable: false},
					{Name: "payload", DataType: "jsonb", Nullable: false, Default: "'{}'::jsonb"},
				},
			},
			{
				Name: "organization_details",
				Fields: []IndustrySolutionFieldDefinition{
					{Name: "detail_key", DataType: "text", PrimaryKey: true},
					{Name: "master_key", DataType: "text", Nullable: false},
					{Name: "detail_type", DataType: "text", Nullable: false},
					{Name: "payload", DataType: "jsonb", Nullable: false, Default: "'{}'::jsonb"},
				},
			},
		},
	}
}
