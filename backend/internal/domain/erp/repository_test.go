package erp

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"testing"
	"time"
)

func TestRepositoryRejectsInvalidJSONBeforeQuery(t *testing.T) {
	repo := NewRepository(nil)
	table, _ := DefaultCatalog().Table("MPOR")
	child, _ := table.Child("POR1")
	input := RecordInput{Key: "1", Data: map[string]any{"Price": math.NaN()}}
	for name, call := range map[string]func() error{
		"create": func() error { _, err := repo.CreateRecord(context.Background(), table, input); return err },
		"update": func() error { _, err := repo.UpdateRecord(context.Background(), table, "1", input); return err },
		"create child": func() error {
			_, err := repo.CreateChildRecord(context.Background(), table, child, "1", input)
			return err
		},
		"update child": func() error {
			_, err := repo.UpdateChildRecord(context.Background(), table, child, "1", "1", input)
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := call(); !errors.Is(err, ErrValidation) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestPostgresColumnTypeMapsDocumentTypes(t *testing.T) {
	cases := []struct {
		field FieldDefinition
		want  string
	}{
		{field("DocEntry", "Int", "11", "", true), "BIGINT"},
		{field("CardName", "nVarChar", "100", "", false), "VARCHAR(100)"},
		{field("DocTotal", "Numeric", "19,6", "", false), "NUMERIC(19,6)"},
		{field("DocDate", "Date", "", "", false), "DATE"},
		{field("Payload", "JSONB", "", "", false), "JSONB"},
	}

	for _, tc := range cases {
		got := postgresColumnType(tc.field)
		if got != tc.want {
			t.Fatalf("postgresColumnType(%#v) = %q, want %q", tc.field, got, tc.want)
		}
	}
}

func TestScanRecordPreservesNestedPayloadField(t *testing.T) {
	rowPayload, err := json.Marshal(map[string]any{
		"DocEntry": "GR-1",
		"Payload": map[string]any{
			"LineNum": "1",
			"Payload": map[string]any{
				"ItemCode": "RAW-1",
				"WhsCode":  "RM",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	record, err := scanRecord("PDN1", "MPDN", "GR-1", fakeRecordRow{
		key:      "1",
		dataJSON: rowPayload,
		created:  time.Now(),
		updated:  time.Now(),
	})

	if err != nil {
		t.Fatalf("scanRecord() error = %v", err)
	}
	nested, ok := record.Data["Payload"].(map[string]any)
	if !ok {
		t.Fatalf("record payload = %#v, want nested Payload preserved", record.Data["Payload"])
	}
	if nested["ItemCode"] != "RAW-1" || nested["WhsCode"] != "RM" {
		t.Fatalf("nested payload = %#v, want item and warehouse fields", nested)
	}
}

type fakeRecordRow struct {
	key      string
	dataJSON []byte
	created  time.Time
	updated  time.Time
}

func (r fakeRecordRow) Scan(dest ...any) error {
	*dest[0].(*string) = r.key
	*dest[1].(*[]byte) = r.dataJSON
	*dest[2].(*time.Time) = r.created
	*dest[3].(*time.Time) = r.updated
	return nil
}
