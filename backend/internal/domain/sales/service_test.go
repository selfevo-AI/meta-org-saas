package sales

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org/backend/internal/domain/finance"
	"github.com/selfevo-AI/meta-org/backend/internal/domain/inventory"
)

func TestPostShipmentRejectsInsufficientStockBeforeReceivable(t *testing.T) {
	ctx := context.Background()
	shipmentID := uuid.New()
	repo := shipmentRepositoryWithLine(shipmentID)
	inv := &salesInventoryRecorder{err: inventory.ErrInsufficientStock}
	fin := &receivableRecorder{}
	svc := NewService(repo, WithInventoryPoster(inv), WithFinancePoster(fin))

	_, err := svc.PostShipment(ctx, shipmentID)
	if !errors.Is(err, inventory.ErrInsufficientStock) {
		t.Fatalf("PostShipment error = %v, want ErrInsufficientStock", err)
	}
	if fin.receivable != nil {
		t.Fatalf("receivable created despite inventory failure: %#v", fin.receivable)
	}
	if repo.shipment.Status != "draft" {
		t.Fatalf("shipment status = %q, want draft after rejected post", repo.shipment.Status)
	}
}

func TestPostShipmentCreatesInventoryMovementsAndReceivable(t *testing.T) {
	ctx := context.Background()
	shipmentID := uuid.New()
	repo := shipmentRepositoryWithLine(shipmentID)
	inv := &salesInventoryRecorder{}
	fin := &receivableRecorder{}
	svc := NewService(repo, WithInventoryPoster(inv), WithFinancePoster(fin))

	posted, err := svc.PostShipment(ctx, shipmentID)
	if err != nil {
		t.Fatalf("PostShipment error = %v", err)
	}
	if posted.Status != "posted" {
		t.Fatalf("posted status = %q, want posted", posted.Status)
	}
	if len(inv.movements) != 1 {
		t.Fatalf("inventory movement count = %d, want 1", len(inv.movements))
	}
	movement := inv.movements[0]
	if movement.MovementType != "sales_shipment" {
		t.Fatalf("movement type = %q, want sales_shipment", movement.MovementType)
	}
	if movement.Quantity != 3 {
		t.Fatalf("movement quantity = %v, want 3", movement.Quantity)
	}
	if fin.receivable == nil {
		t.Fatalf("expected receivable to be created")
	}
	if fin.receivable.SourceType != "sales_shipment" || fin.receivable.SourceID == nil || *fin.receivable.SourceID != shipmentID {
		t.Fatalf("receivable source = %s/%v, want sales_shipment/%s", fin.receivable.SourceType, fin.receivable.SourceID, shipmentID)
	}
	if fin.receivable.CustomerID != "CUS-100" || fin.receivable.CustomerName != "Customer Co" {
		t.Fatalf("receivable customer = %s/%s", fin.receivable.CustomerID, fin.receivable.CustomerName)
	}
	if fin.receivable.Amount != 90 || fin.receivable.TaxAmount != 11.7 {
		t.Fatalf("receivable amount/tax = %v/%v, want 90/11.7", fin.receivable.Amount, fin.receivable.TaxAmount)
	}
}

func shipmentRepositoryWithLine(shipmentID uuid.UUID) *shipmentRepository {
	orgID := uuid.New()
	deptID := uuid.New()
	itemID := uuid.New()
	warehouseID := uuid.New()
	return &shipmentRepository{
		shipment: &SalesShipment{
			ID:             shipmentID,
			OrganizationID: &orgID,
			DepartmentID:   &deptID,
			CustomerID:     "CUS-100",
			CustomerName:   "Customer Co",
			Currency:       "CNY",
			Status:         "draft",
			Lines: []SalesShipmentLine{
				{
					ID:          uuid.New(),
					ShipmentID:  shipmentID,
					ItemID:      itemID,
					WarehouseID: warehouseID,
					Quantity:    3,
					UnitPrice:   30,
					TaxRate:     0.13,
					Amount:      90,
					TaxAmount:   11.7,
					TotalAmount: 101.7,
				},
			},
		},
	}
}

type shipmentRepository struct {
	shipment *SalesShipment
}

func (r *shipmentRepository) CreateQuotation(ctx context.Context, input CreateSalesQuotationInput) (*SalesQuotation, error) {
	return nil, nil
}

func (r *shipmentRepository) ListQuotations(ctx context.Context, limit int) ([]SalesQuotation, error) {
	return nil, nil
}

func (r *shipmentRepository) CreateOrder(ctx context.Context, input CreateSalesOrderInput) (*SalesOrder, error) {
	return nil, nil
}

func (r *shipmentRepository) ListOrders(ctx context.Context, limit int) ([]SalesOrder, error) {
	return nil, nil
}

func (r *shipmentRepository) GetOrder(ctx context.Context, id uuid.UUID) (*SalesOrder, error) {
	return nil, nil
}

func (r *shipmentRepository) UpdateOrderStatus(ctx context.Context, id uuid.UUID, status string) (*SalesOrder, error) {
	return nil, nil
}

func (r *shipmentRepository) CreateShipment(ctx context.Context, input CreateSalesShipmentInput) (*SalesShipment, error) {
	return r.shipment, nil
}

func (r *shipmentRepository) ListShipments(ctx context.Context, limit int) ([]SalesShipment, error) {
	return []SalesShipment{*r.shipment}, nil
}

func (r *shipmentRepository) GetShipment(ctx context.Context, id uuid.UUID) (*SalesShipment, error) {
	return r.shipment, nil
}

func (r *shipmentRepository) MarkShipmentPosted(ctx context.Context, id uuid.UUID, receivableID *uuid.UUID) (*SalesShipment, error) {
	copy := *r.shipment
	copy.Status = "posted"
	copy.ReceivableID = receivableID
	r.shipment = &copy
	return &copy, nil
}

func (r *shipmentRepository) CreateReturn(ctx context.Context, input CreateSalesReturnInput) (*SalesReturn, error) {
	return nil, nil
}

func (r *shipmentRepository) ListReturns(ctx context.Context, limit int) ([]SalesReturn, error) {
	return nil, nil
}

type salesInventoryRecorder struct {
	movements []inventory.CreateInventoryMovementInput
	err       error
}

func (r *salesInventoryRecorder) PostMovement(ctx context.Context, input inventory.CreateInventoryMovementInput) (*inventory.InventoryMovement, error) {
	if r.err != nil {
		return nil, r.err
	}
	r.movements = append(r.movements, input)
	return &inventory.InventoryMovement{ID: uuid.New(), UnitCost: 6, Amount: input.Quantity * 6}, nil
}

type receivableRecorder struct {
	receivable *finance.CreateReceivableInput
}

func (r *receivableRecorder) CreateReceivable(ctx context.Context, input finance.CreateReceivableInput) (*finance.Receivable, error) {
	r.receivable = &input
	id := uuid.New()
	return &finance.Receivable{ID: id}, nil
}
