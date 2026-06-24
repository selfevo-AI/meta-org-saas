package erp

import (
	"errors"
	"time"

	"github.com/google/uuid"
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
	Data               map[string]any `json:"data"`
	ActorID            *uuid.UUID     `json:"actor_id,omitempty"`
	ActorType          string         `json:"actor_type,omitempty"`
	IdempotencyKey     string         `json:"idempotency_key,omitempty"`
	Source             string         `json:"source,omitempty"`
	ToolExecutionID    *uuid.UUID     `json:"tool_execution_id,omitempty"`
	AssistantSessionID *uuid.UUID     `json:"assistant_session_id,omitempty"`
}

type ActionResult struct {
	TableCode            string               `json:"table_code"`
	Key                  string               `json:"key"`
	Action               string               `json:"action"`
	Status               string               `json:"status"`
	Record               *Record              `json:"record,omitempty"`
	GeneratedRecords     []Record             `json:"generated_records,omitempty"`
	Effects              map[string]any       `json:"effects,omitempty"`
	ExecutionID          uuid.UUID            `json:"execution_id,omitempty"`
	IdempotencyKey       string               `json:"idempotency_key,omitempty"`
	PreconditionsChecked []ActionPrecondition `json:"preconditions_checked,omitempty"`
	Provenance           map[string]any       `json:"provenance,omitempty"`
	FailureReason        *ActionFailure       `json:"failure_reason,omitempty"`
}

const (
	ActionExecutionRunning          = "running"
	ActionExecutionCompleted        = "completed"
	ActionExecutionFailed           = "failed"
	ActionExecutionIdempotentReplay = "idempotent_replay"
)

type ActionFailure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ActionPrecondition struct {
	Key     string `json:"key"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type ActionExecution struct {
	ID                 uuid.UUID               `json:"id"`
	TableCode          string                  `json:"table_code"`
	RecordKey          string                  `json:"record_key"`
	Action             string                  `json:"action"`
	Status             string                  `json:"status"`
	IdempotencyKey     string                  `json:"idempotency_key"`
	ActorID            *uuid.UUID              `json:"actor_id,omitempty"`
	ActorType          string                  `json:"actor_type,omitempty"`
	ToolExecutionID    *uuid.UUID              `json:"tool_execution_id,omitempty"`
	AssistantSessionID *uuid.UUID              `json:"assistant_session_id,omitempty"`
	Source             string                  `json:"source,omitempty"`
	FailureCode        string                  `json:"failure_code,omitempty"`
	FailureMessage     string                  `json:"failure_message,omitempty"`
	Payload            map[string]any          `json:"payload"`
	GeneratedRecords   []ActionGeneratedRecord `json:"generated_records,omitempty"`
	StartedAt          time.Time               `json:"started_at,omitempty"`
	CompletedAt        *time.Time              `json:"completed_at,omitempty"`
}

type ActionGeneratedRecord struct {
	ActionID           uuid.UUID      `json:"action_id"`
	LineNum            int            `json:"line_num"`
	GeneratedTableCode string         `json:"generated_table_code"`
	GeneratedKey       string         `json:"generated_key"`
	RelationType       string         `json:"relation_type"`
	Payload            map[string]any `json:"payload"`
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
