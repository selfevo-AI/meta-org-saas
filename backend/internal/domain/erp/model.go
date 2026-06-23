package erp

import (
	"errors"
	"time"
)

var (
	ErrValidation = errors.New("validation error")
	ErrNotFound   = errors.New("not found")
)

type FieldDefinition struct {
	Name        string `json:"name"`
	DataType    string `json:"data_type"`
	Size        string `json:"size,omitempty"`
	Description string `json:"description,omitempty"`
	PrimaryKey  bool   `json:"primary_key"`
}

type ChildTableDefinition struct {
	Code        string            `json:"code"`
	Name        string            `json:"name"`
	ParentKey   string            `json:"parent_key"`
	LineKey     string            `json:"line_key"`
	Fields      []FieldDefinition `json:"fields"`
	fieldLookup map[string]FieldDefinition
}

type TableDefinition struct {
	Code        string                 `json:"code"`
	Name        string                 `json:"name"`
	Module      string                 `json:"module"`
	PrimaryKey  string                 `json:"primary_key"`
	Fields      []FieldDefinition      `json:"fields"`
	Children    []ChildTableDefinition `json:"children,omitempty"`
	fieldLookup map[string]FieldDefinition
	childLookup map[string]ChildTableDefinition
}

type BusinessDocumentType struct {
	Key         string   `json:"key"`
	Name        string   `json:"name"`
	TableCode   string   `json:"table_code"`
	ChildCodes  []string `json:"child_codes,omitempty"`
	ActionNames []string `json:"action_names,omitempty"`
}

type SubmoduleDefinition struct {
	Key       string                 `json:"key"`
	Name      string                 `json:"name"`
	Documents []BusinessDocumentType `json:"documents"`
}

type ModuleDefinition struct {
	Key        string                `json:"key"`
	Name       string                `json:"name"`
	Submodules []SubmoduleDefinition `json:"submodules"`
}

type Catalog struct {
	Tables  []TableDefinition  `json:"tables"`
	Modules []ModuleDefinition `json:"modules"`
	byCode  map[string]TableDefinition
}

type Record struct {
	TableCode       string         `json:"table_code"`
	ParentTableCode string         `json:"parent_table_code,omitempty"`
	ParentKey       string         `json:"parent_key,omitempty"`
	Key             string         `json:"key"`
	Data            map[string]any `json:"data"`
	CreatedAt       time.Time      `json:"created_at,omitempty"`
	UpdatedAt       time.Time      `json:"updated_at,omitempty"`
}

type RecordInput struct {
	Key  string         `json:"key,omitempty"`
	Data map[string]any `json:"data"`
}

type ActionDefinition struct {
	TableCode   string   `json:"table_code"`
	Action      string   `json:"action"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	NextTables  []string `json:"next_tables,omitempty"`
}

type ActionInput struct {
	Data map[string]any `json:"data"`
}

type ActionResult struct {
	TableCode        string         `json:"table_code"`
	Key              string         `json:"key"`
	Action           string         `json:"action"`
	Status           string         `json:"status"`
	Record           *Record        `json:"record,omitempty"`
	GeneratedRecords []Record       `json:"generated_records,omitempty"`
	Effects          map[string]any `json:"effects,omitempty"`
}

func (c Catalog) Table(code string) (TableDefinition, bool) {
	table, ok := c.byCode[code]
	return table, ok
}

func (c Catalog) HasBusinessDocument(moduleName, submoduleName, documentName, tableCode, childCode, actionName string) bool {
	for _, module := range c.Modules {
		if module.Name != moduleName {
			continue
		}
		for _, submodule := range module.Submodules {
			if submodule.Name != submoduleName {
				continue
			}
			for _, document := range submodule.Documents {
				if document.Name != documentName || document.TableCode != tableCode {
					continue
				}
				if childCode != "" && !containsString(document.ChildCodes, childCode) {
					continue
				}
				if actionName != "" && !containsString(document.ActionNames, actionName) {
					continue
				}
				return true
			}
		}
	}
	return false
}

func (t TableDefinition) Field(name string) (FieldDefinition, bool) {
	field, ok := t.fieldLookup[name]
	return field, ok
}

func (t TableDefinition) Child(code string) (ChildTableDefinition, bool) {
	child, ok := t.childLookup[code]
	return child, ok
}

func (c ChildTableDefinition) Field(name string) (FieldDefinition, bool) {
	field, ok := c.fieldLookup[name]
	return field, ok
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
