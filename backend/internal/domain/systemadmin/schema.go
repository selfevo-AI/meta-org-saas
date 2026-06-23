package systemadmin

import (
	"fmt"
	"sort"
	"strings"

	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/tenantdb"
)

const SchemaPackageFormatVersion = "meta-org.schema.v1"

const (
	SchemaRiskSafe        = "safe"
	SchemaRiskDestructive = "destructive"
)

type SchemaPackage struct {
	FormatVersion string                  `json:"format_version"`
	ModuleKey     string                  `json:"module_key"`
	Tables        []SchemaTableDefinition `json:"tables"`
	Metadata      map[string]any          `json:"metadata,omitempty"`
}

type SchemaTableDefinition struct {
	Name         string                  `json:"name"`
	PreviousName string                  `json:"previous_name,omitempty"`
	Fields       []SchemaFieldDefinition `json:"fields"`
	Indexes      []SchemaIndexDefinition `json:"indexes,omitempty"`
	Constraints  []string                `json:"constraints,omitempty"`
	Seeds        []map[string]any        `json:"seeds,omitempty"`
	Metadata     map[string]any          `json:"metadata,omitempty"`
}

type SchemaFieldDefinition struct {
	Name         string `json:"name"`
	PreviousName string `json:"previous_name,omitempty"`
	DataType     string `json:"data_type"`
	Nullable     bool   `json:"nullable"`
	PrimaryKey   bool   `json:"primary_key,omitempty"`
	Default      string `json:"default,omitempty"`
}

type SchemaIndexDefinition struct {
	Name    string   `json:"name"`
	Fields  []string `json:"fields"`
	Unique  bool     `json:"unique,omitempty"`
	Where   string   `json:"where,omitempty"`
	Comment string   `json:"comment,omitempty"`
}

type SchemaDiff struct {
	Action string `json:"action"`
	Table  string `json:"table,omitempty"`
	Field  string `json:"field,omitempty"`
	From   string `json:"from,omitempty"`
	To     string `json:"to,omitempty"`
	Risk   string `json:"risk"`
}

type SchemaMigrationPlan struct {
	Statements []string     `json:"statements"`
	RiskLevel  string       `json:"risk_level"`
	Diff       []SchemaDiff `json:"diff"`
}

func ValidateSchemaPackage(pkg SchemaPackage) error {
	if pkg.FormatVersion != SchemaPackageFormatVersion {
		return fmt.Errorf("unsupported schema package format %q", pkg.FormatVersion)
	}
	if _, err := tenantdb.QuoteIdentifier(pkg.ModuleKey); err != nil {
		return fmt.Errorf("invalid module_key: %w", err)
	}
	seenTables := map[string]bool{}
	for _, table := range pkg.Tables {
		if err := validateTable(table); err != nil {
			return err
		}
		seenTables[table.Name] = true
	}
	if pkg.ModuleKey == "organization" {
		if !seenTables["organization_masters"] {
			return fmt.Errorf("schema package requires organization_masters")
		}
		if !seenTables["organization_details"] {
			return fmt.Errorf("schema package requires organization_details")
		}
	}
	return nil
}

func BuildCreateTableStatements(schemaName string, pkg SchemaPackage) ([]string, error) {
	if err := ValidateSchemaPackage(pkg); err != nil {
		return nil, err
	}
	quotedSchema, err := tenantdb.QuoteIdentifier(schemaName)
	if err != nil {
		return nil, err
	}
	statements := make([]string, 0, len(pkg.Tables))
	for _, table := range pkg.Tables {
		statement, err := buildCreateTableStatement(quotedSchema, table)
		if err != nil {
			return nil, err
		}
		statements = append(statements, statement)
	}
	return statements, nil
}

func BuildSchemaMigrationPlan(schemaName string, current SchemaPackage, desired SchemaPackage) (*SchemaMigrationPlan, error) {
	if err := ValidateSchemaPackage(current); err != nil {
		return nil, err
	}
	if err := ValidateSchemaPackage(desired); err != nil {
		return nil, err
	}
	quotedSchema, err := tenantdb.QuoteIdentifier(schemaName)
	if err != nil {
		return nil, err
	}
	plan := &SchemaMigrationPlan{RiskLevel: SchemaRiskSafe}
	currentTables := map[string]SchemaTableDefinition{}
	desiredTableNames := map[string]bool{}
	renamedTables := map[string]bool{}
	for _, table := range current.Tables {
		currentTables[table.Name] = table
	}
	for _, table := range desired.Tables {
		desiredTableNames[table.Name] = true
		sourceName := table.Name
		if table.PreviousName != "" {
			sourceName = table.PreviousName
			renamedTables[table.PreviousName] = true
			if _, ok := currentTables[table.PreviousName]; ok {
				stmt, err := renameTableStatement(quotedSchema, table.PreviousName, table.Name)
				if err != nil {
					return nil, err
				}
				plan.add(stmt, SchemaDiff{Action: "rename_table", Table: table.Name, From: table.PreviousName, To: table.Name, Risk: SchemaRiskDestructive})
			}
		}
		currentTable, exists := currentTables[sourceName]
		if !exists {
			stmt, err := buildCreateTableStatement(quotedSchema, table)
			if err != nil {
				return nil, err
			}
			plan.add(stmt, SchemaDiff{Action: "create_table", Table: table.Name, Risk: SchemaRiskSafe})
			plan.addIndexStatements(quotedSchema, table, map[string]bool{})
			continue
		}
		if err := plan.addColumnMigrationStatements(quotedSchema, currentTable, table); err != nil {
			return nil, err
		}
	}
	for _, table := range current.Tables {
		if desiredTableNames[table.Name] || renamedTables[table.Name] {
			continue
		}
		stmt, err := dropTableStatement(quotedSchema, table.Name)
		if err != nil {
			return nil, err
		}
		plan.add(stmt, SchemaDiff{Action: "drop_table", Table: table.Name, Risk: SchemaRiskDestructive})
	}
	return plan, nil
}

func (p *SchemaMigrationPlan) add(statement string, diff SchemaDiff) {
	p.Statements = append(p.Statements, statement)
	p.Diff = append(p.Diff, diff)
	if diff.Risk == SchemaRiskDestructive {
		p.RiskLevel = SchemaRiskDestructive
	}
}

func (p *SchemaMigrationPlan) addColumnMigrationStatements(quotedSchema string, currentTable SchemaTableDefinition, desiredTable SchemaTableDefinition) error {
	currentFields := map[string]SchemaFieldDefinition{}
	desiredFieldNames := map[string]bool{}
	renamedFields := map[string]bool{}
	for _, field := range currentTable.Fields {
		currentFields[field.Name] = field
	}
	quotedTable, err := tenantdb.QuoteIdentifier(desiredTable.Name)
	if err != nil {
		return err
	}
	for _, field := range desiredTable.Fields {
		desiredFieldNames[field.Name] = true
		sourceName := field.Name
		if field.PreviousName != "" {
			sourceName = field.PreviousName
			renamedFields[field.PreviousName] = true
			if _, ok := currentFields[field.PreviousName]; ok {
				stmt, err := renameColumnStatement(quotedSchema, quotedTable, field.PreviousName, field.Name)
				if err != nil {
					return err
				}
				p.add(stmt, SchemaDiff{Action: "rename_field", Table: desiredTable.Name, Field: field.Name, From: field.PreviousName, To: field.Name, Risk: SchemaRiskDestructive})
			}
		}
		currentField, exists := currentFields[sourceName]
		if !exists {
			stmt, err := addColumnStatement(quotedSchema, quotedTable, field)
			if err != nil {
				return err
			}
			p.add(stmt, SchemaDiff{Action: "add_field", Table: desiredTable.Name, Field: field.Name, Risk: SchemaRiskSafe})
			continue
		}
		if err := p.addAlterColumnStatements(quotedSchema, quotedTable, desiredTable.Name, currentField, field); err != nil {
			return err
		}
	}
	for _, field := range currentTable.Fields {
		if desiredFieldNames[field.Name] || renamedFields[field.Name] {
			continue
		}
		stmt, err := dropColumnStatement(quotedSchema, quotedTable, field.Name)
		if err != nil {
			return err
		}
		p.add(stmt, SchemaDiff{Action: "drop_field", Table: desiredTable.Name, Field: field.Name, Risk: SchemaRiskDestructive})
	}
	currentIndexes := map[string]bool{}
	for _, index := range currentTable.Indexes {
		currentIndexes[index.Name] = true
	}
	return p.addIndexStatements(quotedSchema, desiredTable, currentIndexes)
}

func (p *SchemaMigrationPlan) addAlterColumnStatements(quotedSchema string, quotedTable string, tableName string, current SchemaFieldDefinition, desired SchemaFieldDefinition) error {
	quotedField, err := tenantdb.QuoteIdentifier(desired.Name)
	if err != nil {
		return err
	}
	if normalizeType(current.DataType) != normalizeType(desired.DataType) {
		stmt := fmt.Sprintf("ALTER TABLE %s.%s ALTER COLUMN %s TYPE %s", quotedSchema, quotedTable, quotedField, normalizeType(desired.DataType))
		p.add(stmt, SchemaDiff{Action: "alter_field_type", Table: tableName, Field: desired.Name, From: current.DataType, To: desired.DataType, Risk: SchemaRiskDestructive})
	}
	if current.Nullable != desired.Nullable {
		action := "DROP NOT NULL"
		if !desired.Nullable {
			action = "SET NOT NULL"
		}
		stmt := fmt.Sprintf("ALTER TABLE %s.%s ALTER COLUMN %s %s", quotedSchema, quotedTable, quotedField, action)
		p.add(stmt, SchemaDiff{Action: "alter_field_nullable", Table: tableName, Field: desired.Name, From: fmt.Sprint(current.Nullable), To: fmt.Sprint(desired.Nullable), Risk: SchemaRiskDestructive})
	}
	if strings.TrimSpace(current.Default) != strings.TrimSpace(desired.Default) {
		stmt := fmt.Sprintf("ALTER TABLE %s.%s ALTER COLUMN %s DROP DEFAULT", quotedSchema, quotedTable, quotedField)
		if desired.Default != "" {
			stmt = fmt.Sprintf("ALTER TABLE %s.%s ALTER COLUMN %s SET DEFAULT %s", quotedSchema, quotedTable, quotedField, desired.Default)
		}
		p.add(stmt, SchemaDiff{Action: "alter_field_default", Table: tableName, Field: desired.Name, From: current.Default, To: desired.Default, Risk: SchemaRiskSafe})
	}
	return nil
}

func (p *SchemaMigrationPlan) addIndexStatements(quotedSchema string, table SchemaTableDefinition, currentIndexes map[string]bool) error {
	quotedTable, err := tenantdb.QuoteIdentifier(table.Name)
	if err != nil {
		return err
	}
	for _, index := range table.Indexes {
		if currentIndexes[index.Name] {
			continue
		}
		quotedIndex, err := tenantdb.QuoteIdentifier(index.Name)
		if err != nil {
			return err
		}
		fields := make([]string, 0, len(index.Fields))
		for _, field := range index.Fields {
			quotedField, err := tenantdb.QuoteIdentifier(field)
			if err != nil {
				return err
			}
			fields = append(fields, quotedField)
		}
		unique := ""
		if index.Unique {
			unique = "UNIQUE "
		}
		stmt := fmt.Sprintf("CREATE %sINDEX IF NOT EXISTS %s ON %s.%s (%s)", unique, quotedIndex, quotedSchema, quotedTable, strings.Join(fields, ", "))
		if strings.TrimSpace(index.Where) != "" {
			stmt += " WHERE " + index.Where
		}
		p.add(stmt, SchemaDiff{Action: "create_index", Table: table.Name, Field: strings.Join(index.Fields, ","), To: index.Name, Risk: SchemaRiskSafe})
	}
	return nil
}

func buildCreateTableStatement(quotedSchema string, table SchemaTableDefinition) (string, error) {
	quotedTable, err := tenantdb.QuoteIdentifier(table.Name)
	if err != nil {
		return "", err
	}
	columnDefs := make([]string, 0, len(table.Fields))
	primaryKeys := make([]string, 0)
	for _, field := range table.Fields {
		def, err := columnDefinition(field)
		if err != nil {
			return "", err
		}
		columnDefs = append(columnDefs, def)
		if field.PrimaryKey {
			quotedField, err := tenantdb.QuoteIdentifier(field.Name)
			if err != nil {
				return "", err
			}
			primaryKeys = append(primaryKeys, quotedField)
		}
	}
	if len(primaryKeys) > 0 {
		columnDefs = append(columnDefs, "PRIMARY KEY ("+strings.Join(primaryKeys, ", ")+")")
	}
	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s.%s (%s)", quotedSchema, quotedTable, strings.Join(columnDefs, ", ")), nil
}

func columnDefinition(field SchemaFieldDefinition) (string, error) {
	quotedField, err := tenantdb.QuoteIdentifier(field.Name)
	if err != nil {
		return "", err
	}
	def := quotedField + " " + normalizeType(field.DataType)
	if !field.Nullable || field.PrimaryKey {
		def += " NOT NULL"
	}
	if field.Default != "" {
		def += " DEFAULT " + field.Default
	}
	return def, nil
}

func renameTableStatement(quotedSchema string, previousName string, nextName string) (string, error) {
	quotedPrevious, err := tenantdb.QuoteIdentifier(previousName)
	if err != nil {
		return "", err
	}
	quotedNext, err := tenantdb.QuoteIdentifier(nextName)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("ALTER TABLE %s.%s RENAME TO %s", quotedSchema, quotedPrevious, quotedNext), nil
}

func dropTableStatement(quotedSchema string, tableName string) (string, error) {
	quotedTable, err := tenantdb.QuoteIdentifier(tableName)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("DROP TABLE %s.%s", quotedSchema, quotedTable), nil
}

func renameColumnStatement(quotedSchema string, quotedTable string, previousName string, nextName string) (string, error) {
	quotedPrevious, err := tenantdb.QuoteIdentifier(previousName)
	if err != nil {
		return "", err
	}
	quotedNext, err := tenantdb.QuoteIdentifier(nextName)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("ALTER TABLE %s.%s RENAME COLUMN %s TO %s", quotedSchema, quotedTable, quotedPrevious, quotedNext), nil
}

func addColumnStatement(quotedSchema string, quotedTable string, field SchemaFieldDefinition) (string, error) {
	def, err := columnDefinition(field)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("ALTER TABLE %s.%s ADD COLUMN %s", quotedSchema, quotedTable, def), nil
}

func dropColumnStatement(quotedSchema string, quotedTable string, fieldName string) (string, error) {
	quotedField, err := tenantdb.QuoteIdentifier(fieldName)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("ALTER TABLE %s.%s DROP COLUMN %s", quotedSchema, quotedTable, quotedField), nil
}

func validateTable(table SchemaTableDefinition) error {
	if _, err := tenantdb.QuoteIdentifier(table.Name); err != nil {
		return fmt.Errorf("invalid table name %q: %w", table.Name, err)
	}
	if table.PreviousName != "" {
		if _, err := tenantdb.QuoteIdentifier(table.PreviousName); err != nil {
			return fmt.Errorf("invalid previous table name %q: %w", table.PreviousName, err)
		}
	}
	if len(table.Fields) == 0 {
		return fmt.Errorf("table %s requires at least one field", table.Name)
	}
	seenFields := map[string]bool{}
	for _, field := range table.Fields {
		if _, err := tenantdb.QuoteIdentifier(field.Name); err != nil {
			return fmt.Errorf("invalid field name %q: %w", field.Name, err)
		}
		if field.PreviousName != "" {
			if _, err := tenantdb.QuoteIdentifier(field.PreviousName); err != nil {
				return fmt.Errorf("invalid previous field name %q: %w", field.PreviousName, err)
			}
		}
		if seenFields[field.Name] {
			return fmt.Errorf("duplicate field %s.%s", table.Name, field.Name)
		}
		seenFields[field.Name] = true
		if normalizeType(field.DataType) == "" {
			return fmt.Errorf("unsupported data type %q for %s.%s", field.DataType, table.Name, field.Name)
		}
		if field.Default != "" && !safeDefaultExpression(field.Default) {
			return fmt.Errorf("unsafe default expression for %s.%s", table.Name, field.Name)
		}
	}
	return nil
}

func normalizeType(dataType string) string {
	normalized := strings.ToLower(strings.TrimSpace(dataType))
	allowed := map[string]string{
		"bigint":      "BIGINT",
		"boolean":     "BOOLEAN",
		"bool":        "BOOLEAN",
		"date":        "DATE",
		"integer":     "INTEGER",
		"int":         "INTEGER",
		"json":        "JSON",
		"jsonb":       "JSONB",
		"numeric":     "NUMERIC",
		"text":        "TEXT",
		"timestamp":   "TIMESTAMP",
		"timestamptz": "TIMESTAMPTZ",
		"uuid":        "UUID",
	}
	if value, ok := allowed[normalized]; ok {
		return value
	}
	if strings.HasPrefix(normalized, "varchar(") && strings.HasSuffix(normalized, ")") {
		return strings.ToUpper(normalized)
	}
	if strings.HasPrefix(normalized, "numeric(") && strings.HasSuffix(normalized, ")") {
		return strings.ToUpper(normalized)
	}
	return ""
}

func safeDefaultExpression(expr string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(expr))
	if trimmed == "" {
		return false
	}
	blocked := []string{";", "--", "/*", "*/", " drop ", " alter ", " delete ", " insert ", " update "}
	for _, item := range blocked {
		if strings.Contains(trimmed, item) {
			return false
		}
	}
	allowed := []string{
		"now()",
		"gen_random_uuid()",
		"'{}'::jsonb",
		"'[]'::jsonb",
		"''",
		"0",
		"false",
		"true",
	}
	sort.Strings(allowed)
	index := sort.SearchStrings(allowed, trimmed)
	if index < len(allowed) && allowed[index] == trimmed {
		return true
	}
	return safeQuotedLiteral(trimmed)
}

func safeQuotedLiteral(value string) bool {
	if len(value) < 2 || value[0] != '\'' || value[len(value)-1] != '\'' {
		return false
	}
	body := value[1 : len(value)-1]
	for _, r := range body {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == ' ' {
			continue
		}
		return false
	}
	return true
}
