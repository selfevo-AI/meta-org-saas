package systemadmin

func DefaultOrganizationSchemaPackage() SchemaPackage {
	return SchemaPackage{
		FormatVersion: SchemaPackageFormatVersion,
		ModuleKey:     "organization",
		Tables: []SchemaTableDefinition{
			{
				Name: "organization_masters",
				Fields: []SchemaFieldDefinition{
					{Name: "master_key", DataType: "text", PrimaryKey: true},
					{Name: "entity_type", DataType: "text", Nullable: false},
					{Name: "legacy_table", DataType: "text", Nullable: false, Default: "''"},
					{Name: "legacy_pk", DataType: "text", Nullable: false, Default: "''"},
					{Name: "legacy_id", DataType: "uuid", Nullable: true},
					{Name: "title", DataType: "text", Nullable: false, Default: "''"},
					{Name: "status", DataType: "text", Nullable: false, Default: "'active'"},
					{Name: "parent_master_key", DataType: "text", Nullable: false, Default: "''"},
					{Name: "core_data", DataType: "jsonb", Nullable: false, Default: "'{}'::jsonb"},
					{Name: "metadata", DataType: "jsonb", Nullable: false, Default: "'{}'::jsonb"},
					{Name: "created_at", DataType: "timestamptz", Nullable: false, Default: "now()"},
					{Name: "updated_at", DataType: "timestamptz", Nullable: false, Default: "now()"},
				},
			},
			{
				Name: "organization_details",
				Fields: []SchemaFieldDefinition{
					{Name: "detail_key", DataType: "text", PrimaryKey: true},
					{Name: "master_key", DataType: "text", Nullable: false},
					{Name: "detail_type", DataType: "text", Nullable: false},
					{Name: "field_key", DataType: "text", Nullable: false, Default: "''"},
					{Name: "line_no", DataType: "integer", Nullable: false, Default: "0"},
					{Name: "payload", DataType: "jsonb", Nullable: false, Default: "'{}'::jsonb"},
					{Name: "metadata", DataType: "jsonb", Nullable: false, Default: "'{}'::jsonb"},
					{Name: "created_at", DataType: "timestamptz", Nullable: false, Default: "now()"},
					{Name: "updated_at", DataType: "timestamptz", Nullable: false, Default: "now()"},
				},
			},
		},
		Metadata: map[string]any{
			"default": true,
			"source":  "systemadmin",
		},
	}
}
