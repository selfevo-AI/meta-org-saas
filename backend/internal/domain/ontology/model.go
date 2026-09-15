package ontology

import "github.com/selfevo-AI/meta-org-saas/backend/internal/domain/erp"

type Label struct {
	ZH string `json:"zh"`
	EN string `json:"en"`
}

type Property struct {
	Key         string `json:"key"`
	Label       Label  `json:"label"`
	DataType    string `json:"data_type"`
	SourceField string `json:"source_field"`
}

type Action struct {
	Key              string     `json:"key"`
	Label            Label      `json:"label"`
	RequiresApproval bool       `json:"requires_approval"`
	Parameters       []Property `json:"parameters,omitempty"`
}

type LinkType struct {
	Key         string `json:"key"`
	Label       Label  `json:"label"`
	TargetType  string `json:"target_type"`
	Cardinality string `json:"cardinality"`
	field       string
	child       string
	inverse     bool
	baseTable   bool
}

type ObjectType struct {
	Importable    bool       `json:"importable"`
	SchemaVersion int        `json:"schema_version"`
	Key           string     `json:"key"`
	Label         Label      `json:"label"`
	TableCode     string     `json:"table_code"`
	Module        string     `json:"module"`
	PrimaryKey    string     `json:"primary_key"`
	Properties    []Property `json:"properties"`
	Links         []LinkType `json:"links"`
	Actions       []Action   `json:"actions"`
}

type Object struct {
	Type       string         `json:"type"`
	Key        string         `json:"key"`
	Title      string         `json:"title"`
	TableCode  string         `json:"table_code"`
	Properties map[string]any `json:"properties"`
	Actions    []Action       `json:"actions"`
	Provenance map[string]any `json:"provenance,omitempty"`
}

type QueryInput struct {
	Status    string         `json:"status,omitempty"`
	Sort      string         `json:"sort,omitempty"`
	Direction string         `json:"direction,omitempty"`
	Filters   map[string]any `json:"filters,omitempty"`
	Search    string         `json:"search,omitempty"`
	Cursor    string         `json:"cursor,omitempty"`
	Limit     int            `json:"limit,omitempty"`
}

type ObjectPage struct {
	Total      int64    `json:"total"`
	Objects    []Object `json:"objects"`
	NextCursor string   `json:"next_cursor,omitempty"`
}

type Link struct {
	Type   string `json:"type"`
	Label  Label  `json:"label"`
	Object Object `json:"object"`
}

type ObjectLinks struct {
	Links     []Link `json:"links"`
	Truncated bool   `json:"truncated"`
}

type ActionResult struct {
	Object    Object            `json:"object"`
	Execution *erp.ActionResult `json:"execution"`
}
