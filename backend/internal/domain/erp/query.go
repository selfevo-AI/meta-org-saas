package erp

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type RecordQuery struct {
	Status       string         `json:"status,omitempty"`
	SortField    string         `json:"sort_field,omitempty"`
	SortType     string         `json:"sort_type,omitempty"`
	Direction    string         `json:"direction,omitempty"`
	Filters      map[string]any `json:"filters,omitempty"`
	Search       string         `json:"search,omitempty"`
	After        string         `json:"after,omitempty"`
	Limit        int            `json:"limit,omitempty"`
	ChildCode    string         `json:"-"`
	ChildFilters map[string]any `json:"-"`
}

type RecordPage struct {
	Total      int64    `json:"total"`
	Records    []Record `json:"records"`
	NextCursor string   `json:"next_cursor,omitempty"`
}

type QueryRepository interface {
	QueryRecords(context.Context, TableDefinition, RecordQuery) (*RecordPage, error)
}

func (s *Service) CheckAccess(ctx context.Context, tableCode, operation string) error {
	return s.authorize(ctx, tableCode, operation)
}

func (s *Service) QueryRecords(ctx context.Context, tableCode string, input RecordQuery) (*RecordPage, error) {
	if err := s.authorize(ctx, tableCode, "read"); err != nil {
		return nil, err
	}
	if len(input.Filters) > 20 || len(input.Search) > 200 || len(input.After) > 4096 {
		return nil, fmt.Errorf("%w: query exceeds limits", ErrValidation)
	}
	if input.SortField != "" && (!identPattern.MatchString(input.SortField) || len(input.SortField) > 80) {
		return nil, fmt.Errorf("%w: invalid sort field", ErrValidation)
	}
	if input.Direction != "" && input.Direction != "asc" && input.Direction != "desc" {
		return nil, fmt.Errorf("%w: invalid sort direction", ErrValidation)
	}
	switch input.Status {
	case "", "all", "draft", "pending", "approved", "posted", "closed", "active", "inactive", "void":
	default:
		return nil, fmt.Errorf("%w: invalid document status", ErrValidation)
	}
	for key, value := range input.Filters {
		if !identPattern.MatchString(key) {
			return nil, fmt.Errorf("%w: invalid property", ErrValidation)
		}
		switch value.(type) {
		case string, bool, float64, int, json.Number, nil:
		default:
			return nil, fmt.Errorf("%w: filter values must be scalar", ErrValidation)
		}
	}
	if input.Limit <= 0 {
		input.Limit = 50
	}
	if input.Limit > 200 {
		input.Limit = 200
	}
	repo, ok := s.repo.(QueryRepository)
	if !ok {
		return nil, fmt.Errorf("%w: repository does not support object queries", ErrValidation)
	}
	table, _ := s.table(tableCode)
	if input.ChildCode != "" {
		if _, ok := table.Child(input.ChildCode); !ok {
			return nil, fmt.Errorf("%w: unknown child table", ErrValidation)
		}
	}
	return repo.QueryRecords(ctx, table, input)
}

func (r *PostgresRepository) QueryRecords(ctx context.Context, table TableDefinition, input RecordQuery) (*RecordPage, error) {
	filters := input.Filters
	if filters == nil {
		filters = map[string]any{}
	}
	filterJSON, err := json.Marshal(filters)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid filters", ErrValidation)
	}
	base := fmt.Sprintf(` FROM %s t CROSS JOIN LATERAL (SELECT (to_jsonb(t) - 'Payload' || t."Payload") AS data) o
		WHERE data @> $1::jsonb AND ($2 = '' OR position(lower($2) in lower(data::text)) > 0)
		AND ($3 IN ('','all') OR (`+documentStatusSQL+`) = $3)`, quoteIdent(table.Code))
	args := []any{string(filterJSON), strings.TrimSpace(input.Search), input.Status}
	if input.ChildCode != "" {
		child, ok := table.Child(input.ChildCode)
		if !ok {
			return nil, fmt.Errorf("%w: unknown child table", ErrValidation)
		}
		filter, err := json.Marshal(input.ChildFilters)
		if err != nil {
			return nil, err
		}
		base += fmt.Sprintf(` AND EXISTS (SELECT 1 FROM %s c WHERE c.%s = t.%s AND c."Payload" @> $4::jsonb) `, quoteIdent(child.Code), quoteIdent(child.ParentKey), quoteIdent(table.PrimaryKey))
		args = append(args, string(filter))
	}
	page := &RecordPage{Records: []Record{}}
	if err := r.querier.QueryRow(ctx, `SELECT COUNT(*)`+base, args...).Scan(&page.Total); err != nil {
		return nil, err
	}
	keySQL := "t." + quoteIdent(table.PrimaryKey) + "::text"
	sortSQL, valueCast := keySQL, "::text"
	direction, comparison := "ASC", ">"
	if input.Direction == "desc" {
		direction, comparison = "DESC", "<"
	}
	if input.SortField != "" && input.SortField != table.PrimaryKey {
		if !identPattern.MatchString(input.SortField) {
			return nil, fmt.Errorf("%w: invalid sort field", ErrValidation)
		}
		sortSQL = fmt.Sprintf("NULLIF(data->>'%s','')", input.SortField)
		if input.SortType == "decimal" {
			sortSQL = fmt.Sprintf("CASE WHEN length(%[1]s) < 128 AND %[1]s ~ '^[+-]?[0-9]+([.][0-9]+)?$' THEN (%[1]s)::numeric END", sortSQL)
			valueCast = "::numeric"
		}
	}
	signatureInput := input
	signatureInput.After, signatureInput.Limit = "", 0
	signatureJSON, _ := json.Marshal(struct {
		Table string
		Query RecordQuery
	}{table.Code, signatureInput})
	signature := fmt.Sprintf("%x", sha256.Sum256(signatureJSON))
	legacy := input.SortField == "" && input.Direction == ""
	if input.After != "" {
		if legacy {
			args = append(args, input.After)
			base += fmt.Sprintf(" AND %s > $%d", keySQL, len(args))
		} else {
			cursor, err := decodeRecordCursor(input.After, signature, valueCast == "::numeric")
			if err != nil {
				return nil, err
			}
			args = append(args, cursor.Value, cursor.Key)
			valueParam, keyParam := fmt.Sprintf("$%d%s", len(args)-1, valueCast), fmt.Sprintf("$%d", len(args))
			base += fmt.Sprintf(` AND ((%[1]s %[2]s %[3]s) OR (%[1]s IS NOT DISTINCT FROM %[3]s AND %[4]s %[2]s %[5]s)
				OR (%[1]s IS NULL AND %[3]s IS NOT NULL))`, sortSQL, comparison, valueParam, keySQL, keyParam)
		}
	}
	args = append(args, input.Limit+1)
	query := fmt.Sprintf(`SELECT %s, row_to_json(t)::jsonb, "CreatedAt", "UpdatedAt", (%s)::text`, keySQL, sortSQL) + base +
		fmt.Sprintf(" ORDER BY %s %s NULLS LAST, %s %s LIMIT $%d", sortSQL, direction, keySQL, direction, len(args))
	rows, err := r.querier.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var lastValue *string
	for rows.Next() {
		var value *string
		record, err := scanRecord(table.Code, "", "", sortedRecordRow{row: rows, value: &value})
		if err != nil {
			return nil, err
		}
		if len(page.Records) == input.Limit {
			last := page.Records[len(page.Records)-1]
			page.NextCursor = last.Key
			if !legacy {
				encoded, _ := json.Marshal(recordCursor{Key: last.Key, Value: lastValue, Signature: signature})
				page.NextCursor = base64.RawURLEncoding.EncodeToString(encoded)
			}
			break
		}
		page.Records = append(page.Records, *record)
		lastValue = value
	}
	return page, rows.Err()
}

type sortedRecordRow struct {
	row   interface{ Scan(...any) error }
	value **string
}

func (r sortedRecordRow) Scan(dest ...any) error { return r.row.Scan(append(dest, r.value)...) }

type recordCursor struct {
	Key       string  `json:"key"`
	Value     *string `json:"value"`
	Signature string  `json:"signature"`
}

var sortDecimalPattern = regexp.MustCompile(`^[+-]?[0-9]+(\.[0-9]+)?$`)

func decodeRecordCursor(encoded, signature string, numeric bool) (recordCursor, error) {
	var cursor recordCursor
	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || json.Unmarshal(data, &cursor) != nil || cursor.Signature != signature || cursor.Key == "" || len(cursor.Key) > 256 {
		return cursor, fmt.Errorf("%w: cursor does not match this query", ErrValidation)
	}
	if numeric && cursor.Value != nil && (len(*cursor.Value) >= 128 || !sortDecimalPattern.MatchString(*cursor.Value)) {
		return cursor, fmt.Errorf("%w: invalid numeric cursor value", ErrValidation)
	}
	return cursor, nil
}

const documentStatusSQL = `CASE
	WHEN data->>'BtfStatus' = 'V' THEN 'void'
	WHEN data->>'Posted' = 'Y' OR data->>'BtfStatus' = 'P' THEN 'posted'
	WHEN data->>'DocStatus' = 'C' OR data->>'Status' = 'closed' THEN 'closed'
	WHEN data->>'WddStatus' = 'A' OR data->>'Status' = 'approved' THEN 'approved'
	WHEN data->>'DocStatus' = 'S' OR data->>'WddStatus' = 'W' OR data->>'Status' IN ('submitted','pending') THEN 'pending'
	WHEN data->>'Inactive' = 'Y' OR data->>'Active' = 'N' OR data->>'ValidFor' = 'N' OR data->>'validFor' = 'N' THEN 'inactive'
	WHEN data->>'Active' = 'Y' OR data->>'ValidFor' = 'Y' OR data->>'validFor' = 'Y' OR data->>'Status' = 'active' THEN 'active'
	ELSE 'draft' END`
