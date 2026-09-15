package erp

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// These payload fields are cast by the canonical finance views, even for drafts.
func validateLedgerPayload(tableCode string, data map[string]any) error {
	switch tableCode {
	case "MACT", "MPRC", "MJDT", "JDT1":
	default:
		return nil
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("%w: invalid ledger payload: %v", ErrValidation, err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		return fmt.Errorf("%w: invalid ledger payload", ErrValidation)
	}
	for _, name := range []string{"LegacyID", "LegacyMasterKey"} {
		if _, exists := fields[name]; exists {
			return fmt.Errorf("%w: %s is maintained by migrations", ErrValidation, name)
		}
	}
	for _, name := range []string{"OrganizationID", "DepartmentID", "SourceID", "RefDate"} {
		raw, exists := fields[name]
		if !exists {
			continue
		}
		var value *string
		if err := json.Unmarshal(raw, &value); err != nil {
			return fmt.Errorf("%w: %s must be a string", ErrValidation, name)
		}
		if value == nil || *value == "" {
			continue
		}
		if name == "RefDate" {
			_, err = time.Parse("2006-01-02", *value)
		} else {
			_, err = uuid.Parse(*value)
		}
		if err != nil {
			return fmt.Errorf("%w: invalid %s", ErrValidation, name)
		}
	}
	if raw, exists := fields["Metadata"]; exists {
		var metadata map[string]any
		if err := json.Unmarshal(raw, &metadata); err != nil {
			return fmt.Errorf("%w: Metadata must be an object", ErrValidation)
		}
	}
	if tableCode == "JDT1" {
		for _, name := range []string{"Debit", "Credit"} {
			if _, exists := fields[name]; !exists {
				continue
			}
			value := numericValue(data, name)
			if !validNumber(value) || value < 0 || value != money(value) {
				return fmt.Errorf("%w: %s must be a non-negative amount with at most six decimal places", ErrValidation, name)
			}
		}
	}
	return nil
}
