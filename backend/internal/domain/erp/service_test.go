package erp

import (
	"context"
	"errors"
	"testing"
)

type fakeRepository struct {
	records       map[string][]Record
	childRecords  map[string][]Record
	created       Record
	createdChild  Record
	listTableCode string
}

func (r *fakeRepository) ListRecords(ctx context.Context, table TableDefinition, limit int) ([]Record, error) {
	r.listTableCode = table.Code
	return r.records[table.Code], nil
}

func (r *fakeRepository) CreateRecord(ctx context.Context, table TableDefinition, input RecordInput) (*Record, error) {
	r.created = Record{TableCode: table.Code, Key: input.Key, Data: input.Data}
	return &r.created, nil
}

func (r *fakeRepository) GetRecord(ctx context.Context, table TableDefinition, key string) (*Record, error) {
	for _, record := range r.records[table.Code] {
		if record.Key == key {
			return &record, nil
		}
	}
	return nil, ErrNotFound
}

func (r *fakeRepository) UpdateRecord(ctx context.Context, table TableDefinition, key string, input RecordInput) (*Record, error) {
	record := Record{TableCode: table.Code, Key: key, Data: input.Data}
	return &record, nil
}

func (r *fakeRepository) DeleteRecord(ctx context.Context, table TableDefinition, key string) error {
	return nil
}

func (r *fakeRepository) ListChildRecords(ctx context.Context, parent TableDefinition, child ChildTableDefinition, parentKey string, limit int) ([]Record, error) {
	return r.childRecords[child.Code], nil
}

func (r *fakeRepository) CreateChildRecord(ctx context.Context, parent TableDefinition, child ChildTableDefinition, parentKey string, input RecordInput) (*Record, error) {
	r.createdChild = Record{TableCode: child.Code, ParentTableCode: parent.Code, ParentKey: parentKey, Key: input.Key, Data: input.Data}
	return &r.createdChild, nil
}

func TestCatalogIncludesDocumentTablesAndChildRows(t *testing.T) {
	catalog := DefaultCatalog()

	minv, ok := catalog.Table("MINV")
	if !ok {
		t.Fatal("DefaultCatalog().Table(\"MINV\") missing")
	}
	if minv.Name != "A/R Invoice" {
		t.Fatalf("MINV name = %q, want A/R Invoice", minv.Name)
	}
	if minv.PrimaryKey != "DocEntry" {
		t.Fatalf("MINV primary key = %q, want DocEntry", minv.PrimaryKey)
	}
	child, ok := minv.Child("INV1")
	if !ok {
		t.Fatal("MINV child INV1 missing")
	}
	if child.ParentKey != "DocEntry" || child.LineKey != "LineNum" {
		t.Fatalf("INV1 keys = %#v, want DocEntry/LineNum", child)
	}
}

func TestCreateRecordRejectsUnknownFields(t *testing.T) {
	service := NewService(&fakeRepository{}, DefaultCatalog())

	_, err := service.CreateRecord(context.Background(), "MCRD", RecordInput{
		Key:  "C0001",
		Data: map[string]any{"CardType": "C", "NotAField": "bad"},
	})

	if !errors.Is(err, ErrValidation) {
		t.Fatalf("CreateRecord() error = %v, want ErrValidation", err)
	}
}

func TestCreateChildRecordUsesParentAndChildDefinitions(t *testing.T) {
	repo := &fakeRepository{}
	service := NewService(repo, DefaultCatalog())

	record, err := service.CreateChildRecord(context.Background(), "MINV", "1001", "INV1", RecordInput{
		Key: "1",
		Data: map[string]any{
			"LineNum":    float64(1),
			"Payload":    map[string]any{"ItemCode": "A-100"},
			"LineStatus": "O",
		},
	})

	if err != nil {
		t.Fatalf("CreateChildRecord() error = %v", err)
	}
	if record.TableCode != "INV1" || record.ParentTableCode != "MINV" || record.ParentKey != "1001" {
		t.Fatalf("child record = %#v, want INV1 under MINV/1001", record)
	}
}
