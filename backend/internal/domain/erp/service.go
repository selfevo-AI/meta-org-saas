package erp

import (
	"context"
	"errors"
	"fmt"
)

type Repository interface {
	ListRecords(ctx context.Context, table TableDefinition, limit int) ([]Record, error)
	CreateRecord(ctx context.Context, table TableDefinition, input RecordInput) (*Record, error)
	GetRecord(ctx context.Context, table TableDefinition, key string) (*Record, error)
	UpdateRecord(ctx context.Context, table TableDefinition, key string, input RecordInput) (*Record, error)
	DeleteRecord(ctx context.Context, table TableDefinition, key string) error
	ListChildRecords(ctx context.Context, parent TableDefinition, child ChildTableDefinition, parentKey string, limit int) ([]Record, error)
	CreateChildRecord(ctx context.Context, parent TableDefinition, child ChildTableDefinition, parentKey string, input RecordInput) (*Record, error)
}

type Service struct {
	repo    Repository
	catalog Catalog
	actions ActionRegistry
}

func NewService(repo Repository, catalog Catalog) *Service {
	if catalog.byCode == nil {
		catalog = DefaultCatalog()
	}
	return &Service{repo: repo, catalog: catalog, actions: DefaultActionRegistry()}
}

func (s *Service) Catalog(ctx context.Context) Catalog {
	return s.catalog
}

func (s *Service) Actions(ctx context.Context) []ActionDefinition {
	return s.actions.List()
}

func (s *Service) ListRecords(ctx context.Context, tableCode string, limit int) ([]Record, error) {
	table, err := s.table(tableCode)
	if err != nil {
		return nil, err
	}
	return s.repo.ListRecords(ctx, table, limit)
}

func (s *Service) CreateRecord(ctx context.Context, tableCode string, input RecordInput) (*Record, error) {
	table, err := s.table(tableCode)
	if err != nil {
		return nil, err
	}
	if err := validateRecordInput(table, input); err != nil {
		return nil, err
	}
	return s.repo.CreateRecord(ctx, table, input)
}

func (s *Service) GetRecord(ctx context.Context, tableCode, key string) (*Record, error) {
	table, err := s.table(tableCode)
	if err != nil {
		return nil, err
	}
	return s.repo.GetRecord(ctx, table, key)
}

func (s *Service) UpdateRecord(ctx context.Context, tableCode, key string, input RecordInput) (*Record, error) {
	table, err := s.table(tableCode)
	if err != nil {
		return nil, err
	}
	if err := validateRecordInput(table, input); err != nil {
		return nil, err
	}
	return s.repo.UpdateRecord(ctx, table, key, input)
}

func (s *Service) DeleteRecord(ctx context.Context, tableCode, key string) error {
	table, err := s.table(tableCode)
	if err != nil {
		return err
	}
	return s.repo.DeleteRecord(ctx, table, key)
}

func (s *Service) ListChildRecords(ctx context.Context, tableCode, parentKey, childCode string, limit int) ([]Record, error) {
	parent, child, err := s.child(tableCode, childCode)
	if err != nil {
		return nil, err
	}
	return s.repo.ListChildRecords(ctx, parent, child, parentKey, limit)
}

func (s *Service) CreateChildRecord(ctx context.Context, tableCode, parentKey, childCode string, input RecordInput) (*Record, error) {
	parent, child, err := s.child(tableCode, childCode)
	if err != nil {
		return nil, err
	}
	if err := validateChildRecordInput(child, input); err != nil {
		return nil, err
	}
	return s.repo.CreateChildRecord(ctx, parent, child, parentKey, input)
}

func (s *Service) RunAction(ctx context.Context, tableCode string, key string, action string, input ActionInput) (*ActionResult, error) {
	if _, err := s.table(tableCode); err != nil {
		return nil, err
	}
	def, ok := s.actions.Lookup(tableCode, action)
	if !ok {
		return nil, fmt.Errorf("%w: unknown action %s for %s", ErrValidation, action, tableCode)
	}
	if result, err := s.runBusinessAction(ctx, tableCode, key, action, input); err == nil {
		return result, nil
	} else if !errors.Is(err, errUnsupportedERPAction) {
		return nil, err
	}
	return &ActionResult{
		TableCode: tableCode,
		Key:       key,
		Action:    def.Action,
		Status:    "accepted",
		Effects: map[string]any{
			"definition": def.Label,
		},
	}, nil
}

func (s *Service) table(tableCode string) (TableDefinition, error) {
	table, ok := s.catalog.Table(tableCode)
	if !ok {
		return TableDefinition{}, fmt.Errorf("%w: unknown table %s", ErrValidation, tableCode)
	}
	return table, nil
}

func (s *Service) child(tableCode, childCode string) (TableDefinition, ChildTableDefinition, error) {
	parent, err := s.table(tableCode)
	if err != nil {
		return TableDefinition{}, ChildTableDefinition{}, err
	}
	child, ok := parent.Child(childCode)
	if !ok {
		return TableDefinition{}, ChildTableDefinition{}, fmt.Errorf("%w: unknown child table %s for %s", ErrValidation, childCode, tableCode)
	}
	return parent, child, nil
}

func validateRecordInput(table TableDefinition, input RecordInput) error {
	for name := range input.Data {
		if _, ok := table.Field(name); !ok {
			return fmt.Errorf("%w: unknown field %s for %s", ErrValidation, name, table.Code)
		}
	}
	return nil
}

func validateChildRecordInput(child ChildTableDefinition, input RecordInput) error {
	for name := range input.Data {
		if _, ok := child.Field(name); !ok {
			return fmt.Errorf("%w: unknown field %s for %s", ErrValidation, name, child.Code)
		}
	}
	return nil
}
