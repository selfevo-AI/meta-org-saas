package erp

import (
	"context"
	"errors"
	"testing"
)

func TestRecordIdentityAndSourceCannotBeChanged(t *testing.T) {
	repo := newBusinessFakeRepository()
	repo.seed("MPOR", "order", map[string]any{"DocEntry": "order"})
	repo.seedChild("MPOR", "order", "POR1", map[string]any{"LineNum": "1"})
	service := NewService(repo, DefaultCatalog())
	for _, input := range []RecordInput{
		{Key: "other"}, {Data: map[string]any{"DocEntry": "other"}},
		{Data: map[string]any{"Payload": map[string]any{"DocEntry": "other"}}},
		{Data: map[string]any{"BaseEntry": "other"}}, {Data: map[string]any{"Confirmed": "Y"}},
	} {
		if _, err := service.UpdateRecord(context.Background(), "MPOR", "order", input); !errors.Is(err, ErrValidation) {
			t.Fatalf("input %#v: %v", input, err)
		}
	}
	for _, data := range []map[string]any{{"LineNum": "2"}, {"DocEntry": "other"}} {
		if _, err := service.UpdateChildRecord(context.Background(), "MPOR", "order", "POR1", "1", RecordInput{Data: data}); !errors.Is(err, ErrValidation) {
			t.Fatalf("line input %#v: %v", data, err)
		}
	}
}

func TestFulfillmentMustMatchSourcePricesTaxAndWarehouse(t *testing.T) {
	repo := newBusinessFakeRepository()
	repo.seed("MPOR", "order", map[string]any{"WddStatus": "A", "CardCode": "supplier"})
	repo.seedChild("MPOR", "order", "POR1", map[string]any{"LineNum": "1", "ItemCode": "item", "WhsCode": "warehouse", "Quantity": 2, "Price": 10, "TaxRate": 13})
	service := NewService(repo, DefaultCatalog())
	receipt := &Record{Key: "receipt", Data: map[string]any{"BaseEntry": "order", "BaseTable": "MPOR", "CardCode": "supplier"}}
	valid := map[string]any{"ItemCode": "item", "WhsCode": "warehouse", "Quantity": 2, "Price": 10, "TaxRate": 13}
	if err := service.validateFulfillmentSource(context.Background(), receipt, "MPDN", []map[string]any{valid}); err != nil {
		t.Fatal(err)
	}
	for field, value := range map[string]any{"Price": 11, "TaxRate": 0, "WhsCode": "other", "Quantity": 3} {
		line := copyData(valid)
		line[field] = value
		if err := service.validateFulfillmentSource(context.Background(), receipt, "MPDN", []map[string]any{line}); !errors.Is(err, ErrValidation) {
			t.Fatalf("modified %s accepted: %v", field, err)
		}
	}
}

func TestLegacyGeneratedStockDocumentCannotPostAgain(t *testing.T) {
	repo := newBusinessFakeRepository()
	repo.seed("MIGE", "legacy", map[string]any{"BaseEntry": "POS-old"})
	service := NewService(repo, DefaultCatalog())
	if _, err := service.RunAction(context.Background(), "MIGE", "legacy", "post", ActionInput{}); !errors.Is(err, ErrConflict) {
		t.Fatalf("legacy generated document post = %v", err)
	}
}
