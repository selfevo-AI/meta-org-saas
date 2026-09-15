package ontology

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/erp"
)

type BusinessService interface {
	Catalog(context.Context) erp.Catalog
	Actions(context.Context) []erp.ActionDefinition
	CheckAccess(context.Context, string, string) error
	GetRecord(context.Context, string, string) (*erp.Record, error)
	QueryRecords(context.Context, string, erp.RecordQuery) (*erp.RecordPage, error)
	ListChildRecords(context.Context, string, string, string, int) ([]erp.Record, error)
	ListActionExecutions(context.Context, string, string, int) ([]erp.ActionExecution, error)
	RunAction(context.Context, string, string, string, erp.ActionInput) (*erp.ActionResult, error)
}

type Service struct {
	business BusinessService
	types    []ObjectType
	catalog  CatalogRepository
}

func NewService(business BusinessService, repositories ...CatalogRepository) *Service {
	ctx := context.Background()
	actions := business.Actions(ctx)
	sort.Slice(actions, func(i, j int) bool { return actions[i].Action < actions[j].Action })
	service := &Service{business: business, types: defaultTypes(business.Catalog(ctx), actions)}
	if len(repositories) > 0 {
		service.catalog = repositories[0]
	}
	return service
}

func (s *Service) loadTypes(ctx context.Context) ([]ObjectType, error) {
	if s.catalog != nil {
		return s.catalog.LoadTypes(ctx)
	}
	return s.types, nil
}

func (s *Service) Types(ctx context.Context) ([]ObjectType, error) {
	types, err := s.loadTypes(ctx)
	if err != nil {
		return nil, err
	}
	result := []ObjectType{}
	for _, typ := range types {
		if s.business.CheckAccess(ctx, typ.TableCode, "read") == nil {
			result = append(result, typ)
		}
	}
	return result, nil
}

func (s *Service) Type(ctx context.Context, key string) (ObjectType, error) {
	types, err := s.loadTypes(ctx)
	if err != nil {
		return ObjectType{}, err
	}
	for _, typ := range types {
		if typ.Key == key {
			if err := s.business.CheckAccess(ctx, typ.TableCode, "read"); err != nil {
				return ObjectType{}, err
			}
			return typ, nil
		}
	}
	return ObjectType{}, fmt.Errorf("%w: unknown object type %s", erp.ErrNotFound, key)
}

func (s *Service) Query(ctx context.Context, typeKey string, input QueryInput) (*ObjectPage, error) {
	typ, err := s.Type(ctx, typeKey)
	if err != nil {
		return nil, err
	}
	filters := map[string]any{}
	for key, value := range input.Filters {
		found := false
		for _, property := range typ.Properties {
			if property.Key == key {
				filters[property.SourceField], found = value, true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("%w: unknown property %s", erp.ErrValidation, key)
		}
	}
	sortField, sortType := "", ""
	if input.Sort != "" {
		for _, property := range typ.Properties {
			if property.Key == input.Sort {
				sortField, sortType = property.SourceField, property.DataType
			}
		}
		if sortField == "" {
			return nil, fmt.Errorf("%w: unknown sort property", erp.ErrValidation)
		}
	}
	page, err := s.business.QueryRecords(ctx, typ.TableCode, erp.RecordQuery{Filters: filters, Search: input.Search, After: input.Cursor, Limit: input.Limit,
		Status: input.Status, SortField: sortField, SortType: sortType, Direction: input.Direction})
	if err != nil {
		return nil, err
	}
	result := &ObjectPage{Objects: []Object{}, NextCursor: page.NextCursor, Total: page.Total}
	for _, record := range page.Records {
		result.Objects = append(result.Objects, objectFromRecord(typ, record))
	}
	return result, nil
}

func (s *Service) Get(ctx context.Context, typeKey, key string) (*Object, error) {
	typ, err := s.Type(ctx, typeKey)
	if err != nil {
		return nil, err
	}
	record, err := s.business.GetRecord(ctx, typ.TableCode, key)
	if err != nil {
		return nil, err
	}
	object := objectFromRecord(typ, *record)
	return &object, nil
}

func (s *Service) Links(ctx context.Context, typeKey, key string) (*ObjectLinks, error) {
	typ, err := s.Type(ctx, typeKey)
	if err != nil {
		return nil, err
	}
	record, err := s.business.GetRecord(ctx, typ.TableCode, key)
	if err != nil {
		return nil, err
	}
	result := &ObjectLinks{Links: []Link{}}
	seen := map[string]bool{}
	appendObject := func(def LinkType, object Object) {
		identity := def.Key + ":" + object.Type + ":" + object.Key
		if !seen[identity] {
			seen[identity] = true
			result.Links = append(result.Links, Link{Type: def.Key, Label: def.Label, Object: object})
		}
	}
	for _, def := range typ.Links {
		targetType := def.TargetType
		if def.baseTable && !def.inverse {
			targetType = s.typeForTable(stringValue(record.Data["BaseTable"]))
			if targetType == "" {
				continue
			}
		}
		target, err := s.Type(ctx, targetType)
		if errors.Is(err, erp.ErrForbidden) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if def.inverse {
			filters := map[string]any{def.field: key}
			if def.baseTable {
				filters["BaseTable"] = typ.TableCode
			}
			query := erp.RecordQuery{Filters: filters, Limit: 100}
			if def.child != "" {
				query.Filters, query.ChildCode, query.ChildFilters = nil, def.child, map[string]any{def.field: key, "TargetTable": typ.TableCode}
			}
			page, err := s.business.QueryRecords(ctx, target.TableCode, query)
			if err != nil {
				return nil, err
			}
			result.Truncated = result.Truncated || page.NextCursor != ""
			for _, item := range page.Records {
				appendObject(def, objectFromRecord(target, item))
			}
			continue
		}
		keys := []string{}
		if def.child != "" {
			lines, err := s.business.ListChildRecords(ctx, typ.TableCode, key, def.child, 201)
			if err != nil {
				return nil, err
			}
			if len(lines) > 200 {
				result.Truncated = true
				lines = lines[:200]
			}
			for _, line := range lines {
				keys = append(keys, stringValue(line.Data[def.field]))
			}
		} else {
			keys = append(keys, stringValue(record.Data[def.field]))
		}
		for _, linkedKey := range keys {
			if linkedKey == "" || seen[def.Key+":"+targetType+":"+linkedKey] {
				continue
			}
			linked, err := s.Get(ctx, targetType, linkedKey)
			if errors.Is(err, erp.ErrNotFound) {
				continue
			}
			if err != nil {
				return nil, err
			}
			appendObject(def, *linked)
		}
	}
	return result, nil
}

func (s *Service) History(ctx context.Context, typeKey, key string, limit int) ([]erp.ActionExecution, error) {
	typ, err := s.Type(ctx, typeKey)
	if err != nil {
		return nil, err
	}
	if _, err := s.business.GetRecord(ctx, typ.TableCode, key); err != nil {
		return nil, err
	}
	return s.business.ListActionExecutions(ctx, typ.TableCode, key, limit)
}

func (s *Service) Execute(ctx context.Context, typeKey, key, action string, input erp.ActionInput) (*ActionResult, error) {
	typ, err := s.Type(ctx, typeKey)
	if err != nil {
		return nil, err
	}
	allowed := false
	for _, definition := range typ.Actions {
		if definition.Key == action {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, fmt.Errorf("%w: action is not defined for this object type", erp.ErrValidation)
	}
	result, err := s.business.RunAction(ctx, typ.TableCode, key, action, input)
	if err != nil {
		return nil, err
	}
	object, err := s.Get(ctx, typeKey, key)
	if err != nil {
		return nil, err
	}
	return &ActionResult{Object: *object, Execution: result}, nil
}

func (s *Service) typeForTable(table string) string {
	for _, typ := range s.types {
		if typ.TableCode == table {
			return typ.Key
		}
	}
	return ""
}

func objectFromRecord(typ ObjectType, record erp.Record) Object {
	object := Object{Type: typ.Key, Key: record.Key, TableCode: typ.TableCode, Title: record.Key, Properties: map[string]any{}, Actions: typ.Actions}
	for _, property := range typ.Properties {
		if value, ok := record.Data[property.SourceField]; ok {
			object.Properties[property.Key] = value
		}
	}
	object.Properties["key"] = record.Key
	if name := stringValue(object.Properties["name"]); name != "" {
		object.Title = name
	}
	if provenance, ok := record.Data["provenance"].(map[string]any); ok {
		object.Provenance = provenance
	}
	return object
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}
