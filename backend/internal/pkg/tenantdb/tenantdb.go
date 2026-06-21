package tenantdb

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/google/uuid"
)

func SchemaNameForOrganization(id uuid.UUID) string {
	return "org_" + strings.ReplaceAll(id.String(), "-", "")
}

func QuoteIdentifier(name string) (string, error) {
	if !validIdentifier(name) {
		return "", fmt.Errorf("unsafe sql identifier %q", name)
	}
	return `"` + name + `"`, nil
}

func SearchPathSQL(schemaName string) (string, error) {
	quotedSchema, err := QuoteIdentifier(schemaName)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("SET LOCAL search_path = %s, \"platform\", \"public\"", quotedSchema), nil
}

func validIdentifier(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		if i == 0 {
			if r != '_' && !unicode.IsLetter(r) {
				return false
			}
			continue
		}
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
