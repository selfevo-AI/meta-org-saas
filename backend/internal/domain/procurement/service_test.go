package procurement

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org/backend/internal/domain/finance"
	"github.com/selfevo-AI/meta-org/backend/internal/domain/inventory"
)

func TestPostReceiptCreatesInventoryMovementsAndPayable(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.New()
	deptID := uuid.New()
	itemID := uuid.New()
	warehouseID := uuid.New()
	receiptID := uuid.New()
	repo := &receiptRepository{
		receipt: &PurchaseReceipt{
			ID:             receiptID,
			OrganizationID: &orgID,
			DepartmentID:   &deptID,
			SupplierID:     "SUP-100",
			SupplierName:   "Acme Supplies",
			Currency:       "CNY",
			Status:         "draft",
			Lines: []PurchaseReceiptLine{
				{
					ID:          uuid.New(),
					ReceiptID:   receiptID,
					ItemID:      itemID,
					WarehouseID: warehouseID,
					Quantity:    5,
					UnitCost:    12,
					TaxRate:     0.13,
					Amount:      60,
					TaxAmount:   7.8,
					TotalAmount: 67.8,
				},
			},
		},
	}
	inv := &inventoryRecorder{}
	fin := &payableRecorder{}
	svc := NewService(repo, WithInventoryPoster(inv), WithFinancePoster(fin))

	posted, err := svc.PostReceipt(ctx, receiptID)
	if err != nil {
		t.Fatalf("PostReceipt error = %v", err)
	}
	if posted.Status != "posted" {
		t.Fatalf("posted status = %q, want posted", posted.Status)
	}
	if len(inv.movements) != 1 {
		t.Fatalf("inventory movement count = %d, want 1", len(inv.movements))
	}
	movement := inv.movements[0]
	if movement.MovementType != "purchase_receipt" || movement.ItemID != itemID || movement.WarehouseID != warehouseID {
		t.Fatalf("movement = %#v, want purchase receipt movement for item/warehouse", movement)
	}
	if movement.Quantity != 5 || movement.UnitCost != 12 {
		t.Fatalf("movement qty/unit = %v/%v, want 5/12", movement.Quantity, movement.UnitCost)
	}
	if fin.payable == nil {
		t.Fatalf("expected payable to be created")
	}
	if fin.payable.SourceType != "purchase_receipt" || fin.payable.SourceID == nil || *fin.payable.SourceID != receiptID {
		t.Fatalf("payable source = %s/%v, want purchase_receipt/%s", fin.payable.SourceType, fin.payable.SourceID, receiptID)
	}
	if fin.payable.VendorID != "SUP-100" || fin.payable.VendorName != "Acme Supplies" {
		t.Fatalf("payable vendor = %s/%s", fin.payable.VendorID, fin.payable.VendorName)
	}
	if fin.payable.Amount != 60 || fin.payable.TaxAmount != 7.8 {
		t.Fatalf("payable amount/tax = %v/%v, want 60/7.8", fin.payable.Amount, fin.payable.TaxAmount)
	}
}

type receiptRepository struct {
	receipt *PurchaseReceipt
}

func (r *receiptRepository) CreateRequisition(ctx context.Context, input CreatePurchaseRequisitionInput) (*PurchaseRequisition, error) {
	return nil, nil
}

func (r *receiptRepository) ListRequisitions(ctx context.Context, limit int) ([]PurchaseRequisition, error) {
	return nil, nil
}

func (r *receiptRepository) GetRequisition(ctx context.Context, id uuid.UUID) (*PurchaseRequisition, error) {
	return nil, nil
}

func (r *receiptRepository) UpdateRequisitionStatus(ctx context.Context, id uuid.UUID, status string) (*PurchaseRequisition, error) {
	return nil, nil
}

func (r *receiptRepository) CreateOrder(ctx context.Context, input CreatePurchaseOrderInput) (*PurchaseOrder, error) {
	return nil, nil
}

func (r *receiptRepository) ListOrders(ctx context.Context, limit int) ([]PurchaseOrder, error) {
	return nil, nil
}

func (r *receiptRepository) GetOrder(ctx context.Context, id uuid.UUID) (*PurchaseOrder, error) {
	return nil, nil
}

func (r *receiptRepository) UpdateOrderStatus(ctx context.Context, id uuid.UUID, status string) (*PurchaseOrder, error) {
	return nil, nil
}

func (r *receiptRepository) CreateReceipt(ctx context.Context, input CreatePurchaseReceiptInput) (*PurchaseReceipt, error) {
	return r.receipt, nil
}

func (r *receiptRepository) ListReceipts(ctx context.Context, limit int) ([]PurchaseReceipt, error) {
	return []PurchaseReceipt{*r.receipt}, nil
}

func (r *receiptRepository) GetReceipt(ctx context.Context, id uuid.UUID) (*PurchaseReceipt, error) {
	return r.receipt, nil
}

func (r *receiptRepository) MarkReceiptPosted(ctx context.Context, id uuid.UUID, payableID *uuid.UUID) (*PurchaseReceipt, error) {
	copy := *r.receipt
	copy.Status = "posted"
	copy.PayableID = payableID
	r.receipt = &copy
	return &copy, nil
}

func (r *receiptRepository) CreateReturn(ctx context.Context, input CreatePurchaseReturnInput) (*PurchaseReturn, error) {
	return nil, nil
}

func (r *receiptRepository) ListReturns(ctx context.Context, limit int) ([]PurchaseReturn, error) {
	return nil, nil
}

type inventoryRecorder struct {
	movements []inventory.CreateInventoryMovementInput
}

func (r *inventoryRecorder) PostMovement(ctx context.Context, input inventory.CreateInventoryMovementInput) (*inventory.InventoryMovement, error) {
	r.movements = append(r.movements, input)
	return &inventory.InventoryMovement{ID: uuid.New(), UnitCost: input.UnitCost, Amount: input.Quantity * input.UnitCost}, nil
}

type payableRecorder struct {
	payable *finance.CreatePayableInput
}

func (r *payableRecorder) CreatePayable(ctx context.Context, input finance.CreatePayableInput) (*finance.Payable, error) {
	r.payable = &input
	id := uuid.New()
	return &finance.Payable{ID: id}, nil
}
