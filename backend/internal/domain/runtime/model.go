package runtime

import (
	"errors"
	"time"
)

const (
	StatusActive   = "active"
	StatusDisabled = "disabled"
	StatusArchived = "archived"

	ActionCRUDList   = "crud.list"
	ActionCRUDGet    = "crud.get"
	ActionCRUDCreate = "crud.create"
	ActionCRUDUpdate = "crud.update"
	ActionCRUDDelete = "crud.delete"
)

var (
	ErrValidation = errors.New("validation error")
	ErrNotFound   = errors.New("not found")
	ErrForbidden  = errors.New("forbidden")
)

type FieldDefinition struct {
	FieldKey     string         `json:"field_key"`
	LabelKey     string         `json:"label_key"`
	DataType     string         `json:"data_type"`
	Required     bool           `json:"required"`
	Unique       bool           `json:"unique"`
	DefaultValue any            `json:"default_value,omitempty"`
	DisplayOrder int            `json:"display_order"`
	Metadata     map[string]any `json:"metadata"`
}

type EntityDefinition struct {
	EntityKey    string            `json:"entity_key"`
	ModuleKey    string            `json:"module_key"`
	StorageTable string            `json:"storage_table"`
	EntityType   string            `json:"entity_type"`
	TitleKey     string            `json:"title_key"`
	Status       string            `json:"status"`
	Fields       []FieldDefinition `json:"fields"`
	Metadata     map[string]any    `json:"metadata"`
	CreatedAt    time.Time         `json:"created_at,omitempty"`
	UpdatedAt    time.Time         `json:"updated_at,omitempty"`
}

type OperationField struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Placeholder string `json:"placeholder,omitempty"`
}

type OperationDefinition struct {
	ID                    string           `json:"id"`
	Domain                string           `json:"domain"`
	Title                 string           `json:"title"`
	Method                string           `json:"method"`
	Path                  string           `json:"path"`
	Auth                  bool             `json:"auth"`
	PathParams            []OperationField `json:"pathParams,omitempty"`
	QueryParams           []OperationField `json:"queryParams,omitempty"`
	BodyTemplate          any              `json:"bodyTemplate,omitempty"`
	OperationKind         string           `json:"operationKind,omitempty"`
	DangerLevel           string           `json:"dangerLevel,omitempty"`
	ResultView            string           `json:"resultView,omitempty"`
	AssistantEligible     bool             `json:"assistantEligible,omitempty"`
	RequiresEntityContext bool             `json:"requiresEntityContext,omitempty"`
	Status                string           `json:"status"`
	ActionType            string           `json:"action_type"`
	EntityKey             string           `json:"entity_key,omitempty"`
	AdapterKey            string           `json:"adapter_key,omitempty"`
	Metadata              map[string]any   `json:"metadata"`
	CreatedAt             time.Time        `json:"created_at,omitempty"`
	UpdatedAt             time.Time        `json:"updated_at,omitempty"`
}

type RuntimeRecord struct {
	MasterKey  string         `json:"master_key"`
	EntityKey  string         `json:"entity_key"`
	EntityType string         `json:"entity_type"`
	Title      string         `json:"title"`
	Status     string         `json:"status"`
	Data       map[string]any `json:"data"`
	Metadata   map[string]any `json:"metadata"`
	CreatedAt  time.Time      `json:"created_at,omitempty"`
	UpdatedAt  time.Time      `json:"updated_at,omitempty"`
}

type RuntimeRecordInput struct {
	Title    string         `json:"title,omitempty"`
	Status   string         `json:"status,omitempty"`
	Data     map[string]any `json:"data,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type RuntimeExecutionRequest struct {
	Path  map[string]string `json:"path,omitempty"`
	Query map[string]string `json:"query,omitempty"`
	Body  map[string]any    `json:"body,omitempty"`
}

type RuntimeExecutionResult struct {
	Status  string           `json:"status"`
	Records []RuntimeRecord  `json:"records,omitempty"`
	Record  *RuntimeRecord   `json:"record,omitempty"`
	Data    any              `json:"data,omitempty"`
	Errors  []RuntimeMessage `json:"errors,omitempty"`
}

type RuntimeMessage struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
