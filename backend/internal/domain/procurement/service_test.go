package procurement

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/finance"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/inventory"
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

func TestPostReceiptRetryAfterFinanceFailureDoesNotDuplicateInventory(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.New()
	itemID := uuid.New()
	warehouseID := uuid.New()
	receiptID := uuid.New()
	repo := &receiptRepository{
		receipt: &PurchaseReceipt{
			ID:             receiptID,
			OrganizationID: &orgID,
			SupplierName:   "Retry Supplier",
			Currency:       "CNY",
			Status:         "draft",
			Lines: []PurchaseReceiptLine{
				{
					ID:          uuid.New(),
					ReceiptID:   receiptID,
					ItemID:      itemID,
					WarehouseID: warehouseID,
					Quantity:    2,
					UnitCost:    10,
					Amount:      20,
				},
			},
		},
	}
	inv := &inventoryRecorder{}
	fin := &payableRecorder{err: errors.New("finance unavailable")}
	svc := NewService(repo, WithInventoryPoster(inv), WithFinancePoster(fin))

	_, err := svc.PostReceipt(ctx, receiptID)
	if err == nil {
		t.Fatalf("PostReceipt error = nil, want finance failure")
	}
	if len(inv.movements) != 1 {
		t.Fatalf("inventory movements after failed post = %d, want 1", len(inv.movements))
	}

	fin.err = nil
	posted, err := svc.PostReceipt(ctx, receiptID)
	if err != nil {
		t.Fatalf("PostReceipt retry error = %v", err)
	}
	if posted.Status != "posted" {
		t.Fatalf("posted status = %q, want posted", posted.Status)
	}
	if len(inv.movements) != 1 {
		t.Fatalf("inventory movements after retry = %d, want 1", len(inv.movements))
	}
}

func TestPostReceiptRetryAfterMarkFailureDoesNotDuplicatePayable(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.New()
	itemID := uuid.New()
	warehouseID := uuid.New()
	receiptID := uuid.New()
	repo := &receiptRepository{
		markErr: errors.New("mark failed"),
		receipt: &PurchaseReceipt{
			ID:             receiptID,
			OrganizationID: &orgID,
			SupplierName:   "Retry Supplier",
			Currency:       "CNY",
			Status:         "draft",
			Lines: []PurchaseReceiptLine{
				{
					ID:          uuid.New(),
					ReceiptID:   receiptID,
					ItemID:      itemID,
					WarehouseID: warehouseID,
					Quantity:    2,
					UnitCost:    10,
					Amount:      20,
				},
			},
		},
	}
	inv := &inventoryRecorder{}
	fin := &payableRecorder{}
	svc := NewService(repo, WithInventoryPoster(inv), WithFinancePoster(fin))

	_, err := svc.PostReceipt(ctx, receiptID)
	if err == nil {
		t.Fatalf("PostReceipt error = nil, want mark failure")
	}
	if fin.created != 1 {
		t.Fatalf("payables after failed mark = %d, want 1", fin.created)
	}

	repo.markErr = nil
	posted, err := svc.PostReceipt(ctx, receiptID)
	if err != nil {
		t.Fatalf("PostReceipt retry error = %v", err)
	}
	if posted.Status != "posted" {
		t.Fatalf("posted status = %q, want posted", posted.Status)
	}
	if fin.created != 1 {
		t.Fatalf("payables after retry = %d, want 1", fin.created)
	}
}

func TestApproveRequisitionRequiresSubmittedStatus(t *testing.T) {
	ctx := context.Background()
	reqID := uuid.New()
	repo := &receiptRepository{requisition: &PurchaseRequisition{ID: reqID, Status: "draft"}}
	svc := NewService(repo)

	_, err := svc.ApproveRequisition(ctx, reqID)
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("ApproveRequisition error = %v, want ErrValidation", err)
	}
	if repo.requisition.Status != "draft" {
		t.Fatalf("requisition status = %q, want draft", repo.requisition.Status)
	}
}

func TestApproveOrderRequiresSubmittedStatus(t *testing.T) {
	ctx := context.Background()
	orderID := uuid.New()
	repo := &receiptRepository{order: &PurchaseOrder{ID: orderID, Status: "draft"}}
	svc := NewService(repo)

	_, err := svc.ApproveOrder(ctx, orderID)
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("ApproveOrder error = %v, want ErrValidation", err)
	}
	if repo.order.Status != "draft" {
		t.Fatalf("order status = %q, want draft", repo.order.Status)
	}
}

func TestApproveRequisitionReturnsNotFoundForNilRecord(t *testing.T) {
	ctx := context.Background()
	repo := &receiptRepository{returnNilRequisition: true}
	svc := NewService(repo)

	_, err := svc.ApproveRequisition(ctx, uuid.New())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("ApproveRequisition error = %v, want ErrNotFound", err)
	}
}

func TestApproveOrderReturnsNotFoundForNilRecord(t *testing.T) {
	ctx := context.Background()
	repo := &receiptRepository{returnNilOrder: true}
	svc := NewService(repo)

	_, err := svc.ApproveOrder(ctx, uuid.New())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("ApproveOrder error = %v, want ErrNotFound", err)
	}
}

type receiptRepository struct {
	requisition          *PurchaseRequisition
	order                *PurchaseOrder
	receipt              *PurchaseReceipt
	markErr              error
	returnNilRequisition bool
	returnNilOrder       bool
}

func (r *receiptRepository) CreateRequisition(ctx context.Context, input CreatePurchaseRequisitionInput) (*PurchaseRequisition, error) {
	return nil, nil
}

func (r *receiptRepository) ListRequisitions(ctx context.Context, limit int) ([]PurchaseRequisition, error) {
	return nil, nil
}

func (r *receiptRepository) GetRequisition(ctx context.Context, id uuid.UUID) (*PurchaseRequisition, error) {
	if r.returnNilRequisition {
		return nil, nil
	}
	if r.requisition == nil {
		return nil, ErrNotFound
	}
	return r.requisition, nil
}

func (r *receiptRepository) UpdateRequisitionStatus(ctx context.Context, id uuid.UUID, status string) (*PurchaseRequisition, error) {
	if r.requisition == nil {
		return nil, ErrNotFound
	}
	copy := *r.requisition
	copy.Status = status
	r.requisition = &copy
	return &copy, nil
}

func (r *receiptRepository) CreateOrder(ctx context.Context, input CreatePurchaseOrderInput) (*PurchaseOrder, error) {
	return nil, nil
}

func (r *receiptRepository) ListOrders(ctx context.Context, limit int) ([]PurchaseOrder, error) {
	return nil, nil
}

func (r *receiptRepository) GetOrder(ctx context.Context, id uuid.UUID) (*PurchaseOrder, error) {
	if r.returnNilOrder {
		return nil, nil
	}
	if r.order == nil {
		return nil, ErrNotFound
	}
	return r.order, nil
}

func (r *receiptRepository) UpdateOrderStatus(ctx context.Context, id uuid.UUID, status string) (*PurchaseOrder, error) {
	if r.order == nil {
		return nil, ErrNotFound
	}
	copy := *r.order
	copy.Status = status
	r.order = &copy
	return &copy, nil
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
	if r.markErr != nil {
		return nil, r.markErr
	}
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

func (r *inventoryRecorder) FindMovementBySourceLine(ctx context.Context, sourceType string, sourceID uuid.UUID, lineKey string, lineID uuid.UUID) (*inventory.InventoryMovement, error) {
	for _, movement := range r.movements {
		if movement.SourceType != sourceType || movement.SourceID == nil || *movement.SourceID != sourceID {
			continue
		}
		if value, ok := movement.Metadata[lineKey].(string); ok && value == lineID.String() {
			return &inventory.InventoryMovement{ID: uuid.New()}, nil
		}
	}
	return nil, inventory.ErrNotFound
}

func (r *inventoryRecorder) PostMovement(ctx context.Context, input inventory.CreateInventoryMovementInput) (*inventory.InventoryMovement, error) {
	r.movements = append(r.movements, input)
	return &inventory.InventoryMovement{ID: uuid.New(), UnitCost: input.UnitCost, Amount: input.Quantity * input.UnitCost}, nil
}

type payableRecorder struct {
	payable   *finance.CreatePayableInput
	payableID uuid.UUID
	err       error
	created   int
}

func (r *payableRecorder) FindPayableBySource(ctx context.Context, sourceType string, sourceID uuid.UUID) (*finance.Payable, error) {
	if r.payable == nil || r.payable.SourceType != sourceType || r.payable.SourceID == nil || *r.payable.SourceID != sourceID {
		return nil, finance.ErrNotFound
	}
	return &finance.Payable{ID: r.payableID}, nil
}

func (r *payableRecorder) CreatePayable(ctx context.Context, input finance.CreatePayableInput) (*finance.Payable, error) {
	if r.err != nil {
		return nil, r.err
	}
	r.payable = &input
	r.payableID = uuid.New()
	r.created++
	return &finance.Payable{ID: r.payableID}, nil
}
