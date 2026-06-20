package inventory

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestPostMovementUsesMovingAverageForInboundAndOutbound(t *testing.T) {
	ctx := context.Background()
	itemID := uuid.New()
	warehouseID := uuid.New()
	repo := newMemoryRepository()
	repo.balance = &InventoryBalance{
		ItemID:      itemID,
		WarehouseID: warehouseID,
		Quantity:    10,
		AverageCost: 5,
		ValueAmount: 50,
		Currency:    "CNY",
	}
	svc := NewService(repo)

	inbound, err := svc.PostMovement(ctx, CreateInventoryMovementInput{
		MovementType: "purchase_receipt",
		ItemID:       itemID,
		WarehouseID:  warehouseID,
		Quantity:     5,
		UnitCost:     8,
		Currency:     "CNY",
	})
	if err != nil {
		t.Fatalf("PostMovement inbound error = %v", err)
	}
	if inbound.Amount != 40 {
		t.Fatalf("inbound amount = %v, want 40", inbound.Amount)
	}
	if repo.balance.Quantity != 15 {
		t.Fatalf("balance quantity after inbound = %v, want 15", repo.balance.Quantity)
	}
	if repo.balance.AverageCost != 6 {
		t.Fatalf("balance average cost after inbound = %v, want 6", repo.balance.AverageCost)
	}
	if repo.balance.ValueAmount != 90 {
		t.Fatalf("balance value after inbound = %v, want 90", repo.balance.ValueAmount)
	}

	outbound, err := svc.PostMovement(ctx, CreateInventoryMovementInput{
		MovementType: "sales_shipment",
		ItemID:       itemID,
		WarehouseID:  warehouseID,
		Quantity:     4,
		Currency:     "CNY",
	})
	if err != nil {
		t.Fatalf("PostMovement outbound error = %v", err)
	}
	if outbound.UnitCost != 6 {
		t.Fatalf("outbound unit cost = %v, want current average 6", outbound.UnitCost)
	}
	if outbound.Amount != 24 {
		t.Fatalf("outbound amount = %v, want 24", outbound.Amount)
	}
	if repo.balance.Quantity != 11 {
		t.Fatalf("balance quantity after outbound = %v, want 11", repo.balance.Quantity)
	}
	if repo.balance.AverageCost != 6 {
		t.Fatalf("balance average cost after outbound = %v, want unchanged 6", repo.balance.AverageCost)
	}
	if repo.balance.ValueAmount != 66 {
		t.Fatalf("balance value after outbound = %v, want 66", repo.balance.ValueAmount)
	}
}

func TestPostMovementRejectsInsufficientStock(t *testing.T) {
	ctx := context.Background()
	itemID := uuid.New()
	warehouseID := uuid.New()
	repo := newMemoryRepository()
	repo.balance = &InventoryBalance{
		ItemID:      itemID,
		WarehouseID: warehouseID,
		Quantity:    2,
		AverageCost: 7,
		ValueAmount: 14,
		Currency:    "CNY",
	}
	svc := NewService(repo)

	_, err := svc.PostMovement(ctx, CreateInventoryMovementInput{
		MovementType: "sales_shipment",
		ItemID:       itemID,
		WarehouseID:  warehouseID,
		Quantity:     3,
		Currency:     "CNY",
	})
	if !errors.Is(err, ErrInsufficientStock) {
		t.Fatalf("PostMovement error = %v, want ErrInsufficientStock", err)
	}
	if repo.balance.Quantity != 2 {
		t.Fatalf("balance quantity changed after rejected movement = %v, want 2", repo.balance.Quantity)
	}
}

type memoryRepository struct {
	balance   *InventoryBalance
	movements []InventoryMovement
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{}
}

func (r *memoryRepository) GetBalance(ctx context.Context, itemID uuid.UUID, warehouseID uuid.UUID, locationID *uuid.UUID) (*InventoryBalance, error) {
	if r.balance == nil {
		return nil, ErrNotFound
	}
	copy := *r.balance
	return &copy, nil
}

func (r *memoryRepository) UpsertBalance(ctx context.Context, balance InventoryBalance) (*InventoryBalance, error) {
	r.balance = &balance
	copy := balance
	return &copy, nil
}

func (r *memoryRepository) CreateMovement(ctx context.Context, input CreateInventoryMovementInput, balance InventoryBalance) (*InventoryMovement, error) {
	movement := InventoryMovement{
		ID:           uuid.New(),
		MovementType: input.MovementType,
		SourceType:   input.SourceType,
		SourceID:     input.SourceID,
		ItemID:       input.ItemID,
		WarehouseID:  input.WarehouseID,
		LocationID:   input.LocationID,
		Quantity:     input.Quantity,
		UnitCost:     input.UnitCost,
		Amount:       input.Quantity * input.UnitCost,
		Currency:     input.Currency,
		BalanceAfter: balance.Quantity,
	}
	r.movements = append(r.movements, movement)
	return &movement, nil
}
