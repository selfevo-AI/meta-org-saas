package ontology

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/tenantdb"
)

type CatalogRepository interface {
	LoadTypes(context.Context) ([]ObjectType, error)
}

type PostgresRepository struct{ db tenantdb.DB }

func NewRepository(db tenantdb.DB) *PostgresRepository { return &PostgresRepository{db: db} }

func (r *PostgresRepository) LoadTypes(ctx context.Context) ([]ObjectType, error) {
	rows, err := r.db.Query(ctx, `SELECT t.key, t.table_code, t.primary_key, t.module,
		t.label_zh, t.label_en, t.importable, t.schema_version,
		COALESCE((SELECT jsonb_agg(jsonb_build_object('key',p.key,'source_field',p.source_field,
		'data_type',p.data_type,'label',jsonb_build_object('zh',p.label_zh,'en',p.label_en)) ORDER BY p.ordinal,p.key)
		FROM ontology_properties p WHERE p.object_type=t.key),'[]'::jsonb),
		COALESCE((SELECT jsonb_agg(jsonb_build_object('key',l.key,'target_type',COALESCE(l.target_type,'*'),
		'cardinality',l.cardinality,'field',l.source_field,'child',l.child_table,'inverse',l.inverse,
		'base_table',l.polymorphic,'label',jsonb_build_object('zh',l.label_zh,'en',l.label_en)) ORDER BY l.key)
		FROM ontology_link_types l WHERE l.object_type=t.key),'[]'::jsonb),
		COALESCE((SELECT jsonb_agg(jsonb_build_object('key',a.key,'requires_approval',a.requires_approval,
		'parameters',a.parameters,'label',jsonb_build_object('zh',a.label_zh,'en',a.label_en)) ORDER BY a.key)
		FROM ontology_action_types a WHERE a.object_type=t.key),'[]'::jsonb)
		FROM ontology_object_types t ORDER BY t.key`)
	if err != nil {
		return nil, fmt.Errorf("load ontology catalog: %w", err)
	}
	defer rows.Close()
	result := []ObjectType{}
	for rows.Next() {
		var typ ObjectType
		var properties, links, actions []byte
		if err := rows.Scan(&typ.Key, &typ.TableCode, &typ.PrimaryKey, &typ.Module,
			&typ.Label.ZH, &typ.Label.EN, &typ.Importable, &typ.SchemaVersion, &properties, &links, &actions); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(properties, &typ.Properties); err != nil {
			return nil, err
		}
		var mappings []struct {
			LinkType
			Field     string `json:"field"`
			Child     string `json:"child"`
			Inverse   bool   `json:"inverse"`
			BaseTable bool   `json:"base_table"`
		}
		if err := json.Unmarshal(links, &mappings); err != nil {
			return nil, err
		}
		typ.Links = []LinkType{}
		for _, mapping := range mappings {
			link := mapping.LinkType
			link.field, link.child, link.inverse, link.baseTable = mapping.Field, mapping.Child, mapping.Inverse, mapping.BaseTable
			typ.Links = append(typ.Links, link)
		}
		if err := json.Unmarshal(actions, &typ.Actions); err != nil {
			return nil, err
		}
		result = append(result, typ)
	}
	return result, rows.Err()
}
