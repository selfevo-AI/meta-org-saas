package systemadmin

import (
	"fmt"
	"sort"
	"strings"

	"github.com/selfevo-AI/meta-org/backend/internal/pkg/tenantdb"
)

const SchemaPackageFormatVersion = "meta-org.schema.v1"

type SchemaPackage struct {
	FormatVersion string                  `json:"format_version"`
	ModuleKey     string                  `json:"module_key"`
	Tables        []SchemaTableDefinition `json:"tables"`
	Metadata      map[string]any          `json:"metadata,omitempty"`
}

type SchemaTableDefinition struct {
	Name        string                  `json:"name"`
	Fields      []SchemaFieldDefinition `json:"fields"`
	Indexes     []SchemaIndexDefinition `json:"indexes,omitempty"`
	Constraints []string                `json:"constraints,omitempty"`
	Seeds       []map[string]any        `json:"seeds,omitempty"`
	Metadata    map[string]any          `json:"metadata,omitempty"`
}

type SchemaFieldDefinition struct {
	Name       string `json:"name"`
	DataType   string `json:"data_type"`
	Nullable   bool   `json:"nullable"`
	PrimaryKey bool   `json:"primary_key,omitempty"`
	Default    string `json:"default,omitempty"`
}

type SchemaIndexDefinition struct {
	Name    string   `json:"name"`
	Fields  []string `json:"fields"`
	Unique  bool     `json:"unique,omitempty"`
	Where   string   `json:"where,omitempty"`
	Comment string   `json:"comment,omitempty"`
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
		quotedTable, err := tenantdb.QuoteIdentifier(table.Name)
		if err != nil {
			return nil, err
		}
		columnDefs := make([]string, 0, len(table.Fields))
		primaryKeys := make([]string, 0)
		for _, field := range table.Fields {
			quotedField, err := tenantdb.QuoteIdentifier(field.Name)
			if err != nil {
				return nil, err
			}
			def := quotedField + " " + normalizeType(field.DataType)
			if !field.Nullable || field.PrimaryKey {
				def += " NOT NULL"
			}
			if field.Default != "" {
				def += " DEFAULT " + field.Default
			}
			columnDefs = append(columnDefs, def)
			if field.PrimaryKey {
				primaryKeys = append(primaryKeys, quotedField)
			}
		}
		if len(primaryKeys) > 0 {
			columnDefs = append(columnDefs, "PRIMARY KEY ("+strings.Join(primaryKeys, ", ")+")")
		}
		statements = append(statements, fmt.Sprintf(
			"CREATE TABLE IF NOT EXISTS %s.%s (%s)",
			quotedSchema,
			quotedTable,
			strings.Join(columnDefs, ", "),
		))
	}
	return statements, nil
}

func validateTable(table SchemaTableDefinition) error {
	if _, err := tenantdb.QuoteIdentifier(table.Name); err != nil {
		return fmt.Errorf("invalid table name %q: %w", table.Name, err)
	}
	if len(table.Fields) == 0 {
		return fmt.Errorf("table %s requires at least one field", table.Name)
	}
	seenFields := map[string]bool{}
	for _, field := range table.Fields {
		if _, err := tenantdb.QuoteIdentifier(field.Name); err != nil {
			return fmt.Errorf("invalid field name %q: %w", field.Name, err)
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
