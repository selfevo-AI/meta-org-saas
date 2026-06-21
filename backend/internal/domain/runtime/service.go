package runtime

import (
	"context"
	"fmt"
	"strings"

	"github.com/selfevo-AI/meta-org/backend/internal/pkg/middleware"
)

type Repository interface {
	ListOperations(context.Context) ([]OperationDefinition, error)
	GetOperation(context.Context, string) (*OperationDefinition, error)
	GetEntity(context.Context, string) (*EntityDefinition, error)
	ListRecords(context.Context, EntityDefinition, int) ([]RuntimeRecord, error)
	GetRecord(context.Context, EntityDefinition, string) (*RuntimeRecord, error)
	CreateRecord(context.Context, EntityDefinition, RuntimeRecordInput) (*RuntimeRecord, error)
	UpdateRecord(context.Context, EntityDefinition, string, RuntimeRecordInput) (*RuntimeRecord, error)
	DeleteRecord(context.Context, EntityDefinition, string) error
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func ValidateEntityDefinition(def EntityDefinition) error {
	if strings.TrimSpace(def.EntityKey) == "" {
		return fmt.Errorf("%w: entity_key is required", ErrValidation)
	}
	if strings.TrimSpace(def.ModuleKey) == "" {
		return fmt.Errorf("%w: module_key is required", ErrValidation)
	}
	if strings.TrimSpace(def.StorageTable) == "" {
		return fmt.Errorf("%w: storage_table is required", ErrValidation)
	}
	if strings.TrimSpace(def.EntityType) == "" {
		return fmt.Errorf("%w: entity_type is required", ErrValidation)
	}
	seen := map[string]bool{}
	for _, field := range def.Fields {
		key := strings.TrimSpace(field.FieldKey)
		if key == "" {
			return fmt.Errorf("%w: field_key is required", ErrValidation)
		}
		if seen[key] {
			return fmt.Errorf("%w: duplicate field %s", ErrValidation, key)
		}
		seen[key] = true
		switch strings.TrimSpace(field.DataType) {
		case "text", "number", "boolean", "date", "datetime", "json", "uuid":
		default:
			return fmt.Errorf("%w: unsupported data type %q", ErrValidation, field.DataType)
		}
	}
	return nil
}

func (s *Service) ListOperations(ctx context.Context) ([]OperationDefinition, error) {
	items, err := s.repo.ListOperations(ctx)
	if err != nil {
		return nil, err
	}
	active := make([]OperationDefinition, 0, len(items))
	for _, item := range items {
		if item.Status != "" && item.Status != StatusActive {
			continue
		}
		if !tenantAllowsRuntimeModule(ctx, item.Domain) {
			continue
		}
		active = append(active, item)
	}
	return active, nil
}

func (s *Service) ListRecords(ctx context.Context, entityKey string, limit int) ([]RuntimeRecord, error) {
	entity, err := s.activeEntity(ctx, entityKey)
	if err != nil {
		return nil, err
	}
	return s.repo.ListRecords(ctx, *entity, normalizeLimit(limit))
}

func (s *Service) GetRecord(ctx context.Context, entityKey string, recordKey string) (*RuntimeRecord, error) {
	entity, err := s.activeEntity(ctx, entityKey)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(recordKey) == "" {
		return nil, fmt.Errorf("%w: record key is required", ErrValidation)
	}
	return s.repo.GetRecord(ctx, *entity, recordKey)
}

func (s *Service) CreateRecord(ctx context.Context, entityKey string, input RuntimeRecordInput) (*RuntimeRecord, error) {
	entity, err := s.activeEntity(ctx, entityKey)
	if err != nil {
		return nil, err
	}
	normalized := normalizeInput(input)
	if err := validateRecordInput(*entity, normalized, true); err != nil {
		return nil, err
	}
	return s.repo.CreateRecord(ctx, *entity, normalized)
}

func (s *Service) UpdateRecord(ctx context.Context, entityKey string, recordKey string, input RuntimeRecordInput) (*RuntimeRecord, error) {
	entity, err := s.activeEntity(ctx, entityKey)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(recordKey) == "" {
		return nil, fmt.Errorf("%w: record key is required", ErrValidation)
	}
	normalized := normalizeInput(input)
	if err := validateRecordInput(*entity, normalized, false); err != nil {
		return nil, err
	}
	return s.repo.UpdateRecord(ctx, *entity, recordKey, normalized)
}

func (s *Service) DeleteRecord(ctx context.Context, entityKey string, recordKey string) error {
	entity, err := s.activeEntity(ctx, entityKey)
	if err != nil {
		return err
	}
	if strings.TrimSpace(recordKey) == "" {
		return fmt.Errorf("%w: record key is required", ErrValidation)
	}
	return s.repo.DeleteRecord(ctx, *entity, recordKey)
}

func (s *Service) ExecuteOperation(ctx context.Context, operationID string, input RuntimeExecutionRequest) (*RuntimeExecutionResult, error) {
	operation, err := s.repo.GetOperation(ctx, operationID)
	if err != nil {
		return nil, err
	}
	if operation.Status != "" && operation.Status != StatusActive {
		return nil, ErrNotFound
	}
	switch operation.ActionType {
	case ActionCRUDList:
		records, err := s.ListRecords(ctx, operation.EntityKey, intFromQuery(input.Query, "limit", 100))
		if err != nil {
			return nil, err
		}
		return &RuntimeExecutionResult{Status: "ok", Records: records}, nil
	case ActionCRUDGet:
		record, err := s.GetRecord(ctx, operation.EntityKey, firstNonEmpty(input.Path["recordKey"], input.Path["id"], stringFromBody(input.Body, "record_key")))
		if err != nil {
			return nil, err
		}
		return &RuntimeExecutionResult{Status: "ok", Record: record}, nil
	case ActionCRUDCreate:
		record, err := s.CreateRecord(ctx, operation.EntityKey, inputFromBody(input.Body))
		if err != nil {
			return nil, err
		}
		return &RuntimeExecutionResult{Status: "created", Record: record}, nil
	case ActionCRUDUpdate:
		record, err := s.UpdateRecord(ctx, operation.EntityKey, firstNonEmpty(input.Path["recordKey"], input.Path["id"], stringFromBody(input.Body, "record_key")), inputFromBody(input.Body))
		if err != nil {
			return nil, err
		}
		return &RuntimeExecutionResult{Status: "updated", Record: record}, nil
	case ActionCRUDDelete:
		if err := s.DeleteRecord(ctx, operation.EntityKey, firstNonEmpty(input.Path["recordKey"], input.Path["id"], stringFromBody(input.Body, "record_key"))); err != nil {
			return nil, err
		}
		return &RuntimeExecutionResult{Status: "deleted"}, nil
	default:
		return nil, fmt.Errorf("%w: unsupported action_type %q", ErrValidation, operation.ActionType)
	}
}

func (s *Service) activeEntity(ctx context.Context, entityKey string) (*EntityDefinition, error) {
	entity, err := s.repo.GetEntity(ctx, entityKey)
	if err != nil {
		return nil, err
	}
	if entity.Status != "" && entity.Status != StatusActive {
		return nil, ErrNotFound
	}
	if err := ValidateEntityDefinition(*entity); err != nil {
		return nil, err
	}
	if !tenantAllowsRuntimeModule(ctx, entity.ModuleKey) {
		return nil, ErrForbidden
	}
	return entity, nil
}

func tenantAllowsRuntimeModule(ctx context.Context, moduleKey string) bool {
	tenant, ok := middleware.TenantFromContext(ctx)
	if !ok || tenant == nil || tenant.Mode != "saas" {
		return true
	}
	key := strings.ToLower(strings.TrimSpace(moduleKey))
	if key == "" {
		return false
	}
	return tenant.EnabledModules[key]
}

func normalizeInput(input RuntimeRecordInput) RuntimeRecordInput {
	input.Title = strings.TrimSpace(input.Title)
	input.Status = strings.TrimSpace(input.Status)
	if input.Status == "" {
		input.Status = StatusActive
	}
	if input.Data == nil {
		input.Data = map[string]any{}
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
	if input.Title == "" {
		if title, ok := input.Data["title"].(string); ok {
			input.Title = strings.TrimSpace(title)
		}
	}
	if input.Title != "" {
		if _, ok := input.Data["title"]; !ok {
			input.Data["title"] = input.Title
		}
	}
	if input.Status != "" {
		if _, ok := input.Data["status"]; !ok {
			input.Data["status"] = input.Status
		}
	}
	return input
}

func validateRecordInput(entity EntityDefinition, input RuntimeRecordInput, creating bool) error {
	if !creating {
		return nil
	}
	for _, field := range entity.Fields {
		if !field.Required {
			continue
		}
		value, ok := input.Data[field.FieldKey]
		if !ok || value == nil || strings.TrimSpace(fmt.Sprint(value)) == "" {
			return fmt.Errorf("%w: %s is required", ErrValidation, field.FieldKey)
		}
	}
	return nil
}

func inputFromBody(body map[string]any) RuntimeRecordInput {
	if body == nil {
		body = map[string]any{}
	}
	data := map[string]any{}
	for key, value := range body {
		data[key] = value
	}
	title, _ := body["title"].(string)
	status, _ := body["status"].(string)
	if raw, ok := body["data"].(map[string]any); ok {
		data = raw
	}
	metadata := map[string]any{}
	if raw, ok := body["metadata"].(map[string]any); ok {
		metadata = raw
	}
	return RuntimeRecordInput{Title: title, Status: status, Data: data, Metadata: metadata}
}

func normalizeLimit(limit int) int {
	if limit <= 0 || limit > 200 {
		return 100
	}
	return limit
}

func intFromQuery(query map[string]string, key string, fallback int) int {
	if query == nil {
		return fallback
	}
	var value int
	if _, err := fmt.Sscanf(query[key], "%d", &value); err != nil || value <= 0 {
		return fallback
	}
	return value
}

func stringFromBody(body map[string]any, key string) string {
	if body == nil {
		return ""
	}
	value, _ := body[key].(string)
	return strings.TrimSpace(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
