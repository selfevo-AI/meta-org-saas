package documentimport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/erp"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/ontology"
	"github.com/shopspring/decimal"
)

var headerFields = map[string]string{
	"partner": "CardCode", "date": "DocDate", "due_date": "DocDueDate", "currency": "DocCur",
	"total": "DocTotal", "tax": "VatSum", "external_number": "NumAtCard", "note": "Comments",
}
var lineFields = map[string]string{
	"item": "ItemCode", "warehouse": "WhsCode", "quantity": "Quantity", "unit_price": "Price",
	"tax_rate": "TaxRate", "description": "Dscription",
}
var numericPattern = regexp.MustCompile(`^[+-]?[0-9]+(\.[0-9]+)?$`)

func normalizeDraft(input Draft, objectType string, complete bool) (Draft, error) {
	result := Draft{Key: strings.TrimSpace(input.Key), Properties: map[string]any{}, Lines: []map[string]any{}}
	issues := []Issue{}
	add := func(path, code string) { issues = append(issues, Issue{Path: path, Code: code}) }
	if (complete && result.Key == "") || len(result.Key) > 128 || strings.IndexFunc(result.Key, unicode.IsControl) >= 0 {
		add("key", "required")
	}
	for key, value := range input.Properties {
		if _, ok := headerFields[key]; !ok {
			add("properties."+key, "unknown_field")
			continue
		}
		if value == nil || value == "" {
			continue
		}
		path := "properties." + key
		if key == "total" || key == "tax" {
			number, err := parseDecimal(value)
			if err != nil || number.IsNegative() || number.GreaterThan(decimal.NewFromInt(1000000000000)) {
				add(path, "invalid_number")
				continue
			}
			result.Properties[key] = number.String()
			continue
		}
		text, ok := value.(string)
		if !ok {
			add(path, "invalid")
			continue
		}
		text = strings.TrimSpace(text)
		max := 128
		if key == "note" {
			max = 2000
		}
		if len(text) > max {
			add(path, "too_long")
			continue
		}
		if key == "currency" {
			text = strings.ToUpper(text)
			if len(text) != 3 || strings.Trim(text, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") != "" {
				add(path, "invalid_currency")
				continue
			}
		}
		if key == "date" || key == "due_date" {
			if _, err := time.Parse("2006-01-02", text); err != nil {
				add(path, "invalid_date")
				continue
			}
		}
		result.Properties[key] = text
	}
	if complete {
		for _, key := range []string{"date", "currency"} {
			if stringField(result.Properties, key) == "" {
				add("properties."+key, "required")
			}
		}
		if objectType != "inventory_receipt" && objectType != "inventory_issue" && stringField(result.Properties, "partner") == "" {
			add("properties.partner", "required")
		}
	}
	if due, date := stringField(result.Properties, "due_date"), stringField(result.Properties, "date"); due != "" && date != "" && due < date {
		add("properties.due_date", "invalid_date_range")
	}
	if len(input.Lines) > MaxLines {
		add("lines", "line_limit")
	}
	for index, line := range input.Lines {
		if index >= MaxLines {
			break
		}
		normalized := map[string]any{}
		prefix := fmt.Sprintf("lines.%d.", index)
		for key, value := range line {
			if _, ok := lineFields[key]; !ok {
				add(prefix+key, "unknown_field")
				continue
			}
			if value == nil || value == "" {
				continue
			}
			if key == "quantity" || key == "unit_price" || key == "tax_rate" {
				number, err := parseDecimal(value)
				if err != nil || number.IsNegative() || number.GreaterThan(decimal.NewFromInt(1000000000)) || (key == "quantity" && !number.IsPositive()) || (key == "tax_rate" && number.GreaterThan(decimal.NewFromInt(100))) {
					add(prefix+key, "invalid_number")
					continue
				}
				normalized[key] = number.String()
			} else {
				text, ok := value.(string)
				if !ok || len(text) > 500 {
					add(prefix+key, "invalid")
					continue
				}
				normalized[key] = strings.TrimSpace(text)
			}
		}
		if complete {
			for _, key := range []string{"item", "warehouse", "quantity", "unit_price"} {
				if stringField(normalized, key) == "" {
					add(prefix+key, "required")
				}
			}
			if _, ok := normalized["tax_rate"]; !ok {
				normalized["tax_rate"] = "0"
			}
		}
		result.Lines = append(result.Lines, normalized)
	}
	payment := objectType == "incoming_payment" || objectType == "outgoing_payment"
	if payment && len(result.Lines) > 0 {
		add("lines", "payment_lines")
	}
	if complete && !payment && len(result.Lines) == 0 {
		add("lines", "required")
	}
	if len(issues) > 0 {
		return result, &ValidationError{Issues: issues}
	}
	if complete {
		if payment {
			total, err := parseDecimal(result.Properties["total"])
			if err != nil || !total.IsPositive() {
				return result, issue("properties.total", "required")
			}
		} else {
			net, tax := decimal.Zero, decimal.Zero
			for _, line := range result.Lines {
				quantity, _ := parseDecimal(line["quantity"])
				price, _ := parseDecimal(line["unit_price"])
				rate, _ := parseDecimal(line["tax_rate"])
				amount := quantity.Mul(price).Round(6)
				net = net.Add(amount)
				tax = tax.Add(amount.Mul(rate).Div(decimal.NewFromInt(100)).Round(6))
			}
			for key, computed := range map[string]decimal.Decimal{"total": net.Add(tax), "tax": tax} {
				if computed.GreaterThan(decimal.NewFromInt(1000000000000)) {
					return result, issue("properties."+key, "invalid_number")
				}
				if value, ok := result.Properties[key]; ok {
					number, _ := parseDecimal(value)
					if !number.Equal(computed) {
						return result, issue("properties."+key, "amount_mismatch")
					}
				} else {
					result.Properties[key] = computed.String()
				}
			}
		}
	}
	return result, nil
}

func parseDecimal(value any) (decimal.Decimal, error) {
	var text string
	switch number := value.(type) {
	case string:
		text = strings.TrimSpace(number)
	case float64:
		text = fmt.Sprint(number)
	case int:
		text = fmt.Sprint(number)
	case json.Number:
		text = number.String()
	default:
		return decimal.Zero, fmt.Errorf("invalid decimal")
	}
	if len(text) > 40 || !numericPattern.MatchString(text) {
		return decimal.Zero, fmt.Errorf("invalid decimal")
	}
	number, err := decimal.NewFromString(text)
	if err != nil || !number.Equal(number.Round(6)) {
		return decimal.Zero, fmt.Errorf("invalid decimal precision")
	}
	return number, nil
}

func stringField(data map[string]any, key string) string {
	if value, ok := data[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func validateReferences(ctx context.Context, business *erp.Service, draft Draft, objectType string) error {
	issues := []Issue{}
	check := func(table, key, path string) (*erp.Record, error) {
		record, err := business.GetRecord(ctx, table, key)
		if err != nil {
			if errors.Is(err, erp.ErrNotFound) {
				issues = append(issues, Issue{Path: path, Code: "unknown_reference"})
				return nil, nil
			}
			return nil, err
		}
		if record.Data["Inactive"] == "Y" || record.Data["Active"] == "N" || record.Data["ValidFor"] == "N" || record.Data["validFor"] == "N" {
			issues = append(issues, Issue{Path: path, Code: "inactive_reference"})
		}
		return record, nil
	}
	if partner := stringField(draft.Properties, "partner"); partner != "" {
		record, err := check("MCRD", partner, "properties.partner")
		if err != nil {
			return err
		}
		expected := "C"
		if objectType == "purchase_order" || objectType == "goods_receipt" || objectType == "payable_invoice" || objectType == "outgoing_payment" {
			expected = "S"
		}
		if record != nil && record.Data["CardType"] != expected {
			issues = append(issues, Issue{Path: "properties.partner", Code: "partner_type"})
		}
	}
	for index, line := range draft.Lines {
		for field, table := range map[string]string{"item": "MITM", "warehouse": "MWHS"} {
			if _, err := check(table, stringField(line, field), fmt.Sprintf("lines.%d.%s", index, field)); err != nil {
				return err
			}
		}
	}
	if len(issues) > 0 {
		return &ValidationError{Issues: issues}
	}
	return nil
}

func mapHeader(draft Draft, typ ontology.ObjectType) map[string]any {
	data := map[string]any{}
	for _, property := range typ.Properties {
		if _, allowed := headerFields[property.Key]; allowed {
			if value, ok := draft.Properties[property.Key]; ok {
				if text, ok := value.(string); ok && property.DataType == "decimal" {
					value = json.Number(text)
				}
				data[property.SourceField] = value
			}
		}
	}
	return data
}
