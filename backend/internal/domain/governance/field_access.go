package governance

type FieldAccessMetadata struct {
	DeniedFields []string `json:"denied_fields"`
}

func ApplyFieldAccess(record map[string]any, checks map[string]FieldAccessCheckResult) (map[string]any, FieldAccessMetadata) {
	filtered := make(map[string]any, len(record))
	meta := FieldAccessMetadata{DeniedFields: []string{}}
	for key, value := range record {
		check, ok := checks[key]
		if ok && !check.Allowed {
			filtered[key] = nil
			meta.DeniedFields = append(meta.DeniedFields, key)
			continue
		}
		filtered[key] = value
	}
	return filtered, meta
}
