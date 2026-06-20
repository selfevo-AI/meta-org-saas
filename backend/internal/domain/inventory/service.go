package inventory

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

var (
	ErrValidation        = errors.New("validation error")
	ErrNotFound          = errors.New("not found")
	ErrInsufficientStock = errors.New("insufficient stock")
)

type MovementRepository interface {
	GetBalance(ctx context.Context, itemID uuid.UUID, warehouseID uuid.UUID, locationID *uuid.UUID) (*InventoryBalance, error)
	UpsertBalance(ctx context.Context, balance InventoryBalance) (*InventoryBalance, error)
	CreateMovement(ctx context.Context, input CreateInventoryMovementInput, balance InventoryBalance) (*InventoryMovement, error)
}

type atomicMovementRepository interface {
	PostMovementAtomic(ctx context.Context, input CreateInventoryMovementInput, direction int) (*InventoryMovement, error)
}

type Service struct {
	repo MovementRepository
	raw  any
}

func NewService(repo MovementRepository) *Service {
	return &Service{repo: repo, raw: repo}
}

func (s *Service) FindMovementBySourceLine(ctx context.Context, sourceType string, sourceID uuid.UUID, lineKey string, lineID uuid.UUID) (*InventoryMovement, error) {
	repo, ok := s.raw.(interface {
		FindMovementBySourceLine(context.Context, string, uuid.UUID, string, uuid.UUID) (*InventoryMovement, error)
	})
	if !ok {
		return nil, ErrNotFound
	}
	sourceType = strings.TrimSpace(sourceType)
	lineKey = strings.TrimSpace(lineKey)
	if sourceType == "" || sourceID == uuid.Nil || lineKey == "" || lineID == uuid.Nil {
		return nil, fmt.Errorf("%w: source_type, source_id, line key, and line id are required", ErrValidation)
	}
	return repo.FindMovementBySourceLine(ctx, sourceType, sourceID, lineKey, lineID)
}

func (s *Service) PostMovement(ctx context.Context, input CreateInventoryMovementInput) (*InventoryMovement, error) {
	normalizeMovementInput(&input)
	if input.ItemID == uuid.Nil {
		return nil, fmt.Errorf("%w: item_id is required", ErrValidation)
	}
	if input.WarehouseID == uuid.Nil {
		return nil, fmt.Errorf("%w: warehouse_id is required", ErrValidation)
	}
	if input.Quantity <= 0 {
		return nil, fmt.Errorf("%w: quantity must be greater than zero", ErrValidation)
	}
	direction, err := movementDirection(input.MovementType)
	if err != nil {
		return nil, err
	}
	if repo, ok := s.raw.(atomicMovementRepository); ok {
		return repo.PostMovementAtomic(ctx, input, direction)
	}

	balance, err := s.repo.GetBalance(ctx, input.ItemID, input.WarehouseID, input.LocationID)
	if err != nil {
		if errors.Is(err, ErrNotFound) && direction > 0 {
			balance = &InventoryBalance{
				ItemID:         input.ItemID,
				WarehouseID:    input.WarehouseID,
				LocationID:     input.LocationID,
				Currency:       input.Currency,
				OrganizationID: input.OrganizationID,
				Metadata:       map[string]any{},
			}
		} else if errors.Is(err, ErrNotFound) {
			return nil, ErrInsufficientStock
		} else {
			return nil, err
		}
	}

	next := *balance
	if next.Currency == "" {
		next.Currency = input.Currency
	}
	if next.OrganizationID == nil {
		next.OrganizationID = input.OrganizationID
	}

	if direction > 0 {
		oldValue := next.Quantity * next.AverageCost
		addedValue := input.Quantity * input.UnitCost
		next.Quantity += input.Quantity
		if next.Quantity > 0 {
			next.AverageCost = (oldValue + addedValue) / next.Quantity
		}
		next.ValueAmount = next.Quantity * next.AverageCost
	} else {
		if next.Quantity < input.Quantity {
			return nil, ErrInsufficientStock
		}
		if input.UnitCost <= 0 {
			input.UnitCost = next.AverageCost
		}
		next.Quantity -= input.Quantity
		next.ValueAmount = next.Quantity * next.AverageCost
	}

	updated, err := s.repo.UpsertBalance(ctx, next)
	if err != nil {
		return nil, err
	}
	input.Currency = updated.Currency
	if input.UnitCost <= 0 {
		input.UnitCost = updated.AverageCost
	}
	return s.repo.CreateMovement(ctx, input, *updated)
}

func (s *Service) CreateBusinessPartner(ctx context.Context, input CreateBusinessPartnerInput) (*BusinessPartner, error) {
	normalizePartnerInput(&input)
	repo, ok := s.raw.(interface {
		CreateBusinessPartner(context.Context, CreateBusinessPartnerInput) (*BusinessPartner, error)
	})
	if !ok {
		return nil, fmt.Errorf("%w: business partner repository unavailable", ErrValidation)
	}
	if input.PartnerType == "" || input.Name == "" {
		return nil, fmt.Errorf("%w: partner_type and name are required", ErrValidation)
	}
	return repo.CreateBusinessPartner(ctx, input)
}

func (s *Service) ListBusinessPartners(ctx context.Context, limit int) ([]BusinessPartner, error) {
	repo, ok := s.raw.(interface {
		ListBusinessPartners(context.Context, int) ([]BusinessPartner, error)
	})
	if !ok {
		return nil, nil
	}
	return repo.ListBusinessPartners(ctx, normalizeLimit(limit))
}

func (s *Service) CreateItem(ctx context.Context, input CreateItemInput) (*Item, error) {
	normalizeItemInput(&input)
	repo, ok := s.raw.(interface {
		CreateItem(context.Context, CreateItemInput) (*Item, error)
	})
	if !ok {
		return nil, fmt.Errorf("%w: item repository unavailable", ErrValidation)
	}
	if input.Name == "" {
		return nil, fmt.Errorf("%w: item name is required", ErrValidation)
	}
	return repo.CreateItem(ctx, input)
}

func (s *Service) ListItems(ctx context.Context, limit int) ([]Item, error) {
	repo, ok := s.raw.(interface {
		ListItems(context.Context, int) ([]Item, error)
	})
	if !ok {
		return nil, nil
	}
	return repo.ListItems(ctx, normalizeLimit(limit))
}

func (s *Service) CreateWarehouse(ctx context.Context, input CreateWarehouseInput) (*Warehouse, error) {
	normalizeWarehouseInput(&input)
	repo, ok := s.raw.(interface {
		CreateWarehouse(context.Context, CreateWarehouseInput) (*Warehouse, error)
	})
	if !ok {
		return nil, fmt.Errorf("%w: warehouse repository unavailable", ErrValidation)
	}
	if input.Name == "" {
		return nil, fmt.Errorf("%w: warehouse name is required", ErrValidation)
	}
	return repo.CreateWarehouse(ctx, input)
}

func (s *Service) ListWarehouses(ctx context.Context, limit int) ([]Warehouse, error) {
	repo, ok := s.raw.(interface {
		ListWarehouses(context.Context, int) ([]Warehouse, error)
	})
	if !ok {
		return nil, nil
	}
	return repo.ListWarehouses(ctx, normalizeLimit(limit))
}

func (s *Service) CreateWarehouseLocation(ctx context.Context, input CreateWarehouseLocationInput) (*WarehouseLocation, error) {
	normalizeLocationInput(&input)
	repo, ok := s.raw.(interface {
		CreateWarehouseLocation(context.Context, CreateWarehouseLocationInput) (*WarehouseLocation, error)
	})
	if !ok {
		return nil, fmt.Errorf("%w: warehouse location repository unavailable", ErrValidation)
	}
	if input.WarehouseID == uuid.Nil || input.Name == "" {
		return nil, fmt.Errorf("%w: warehouse_id and name are required", ErrValidation)
	}
	return repo.CreateWarehouseLocation(ctx, input)
}

func (s *Service) ListBalances(ctx context.Context, limit int) ([]InventoryBalance, error) {
	repo, ok := s.raw.(interface {
		ListBalances(context.Context, int) ([]InventoryBalance, error)
	})
	if !ok {
		return nil, nil
	}
	return repo.ListBalances(ctx, normalizeLimit(limit))
}

func (s *Service) ListMovements(ctx context.Context, limit int) ([]InventoryMovement, error) {
	repo, ok := s.raw.(interface {
		ListMovements(context.Context, int) ([]InventoryMovement, error)
	})
	if !ok {
		return nil, nil
	}
	return repo.ListMovements(ctx, normalizeLimit(limit))
}

func (s *Service) CreateTransfer(ctx context.Context, input CreateInventoryTransferInput) (*InventoryTransfer, error) {
	if input.FromWarehouseID == uuid.Nil || input.ToWarehouseID == uuid.Nil {
		return nil, fmt.Errorf("%w: from_warehouse_id and to_warehouse_id are required", ErrValidation)
	}
	if input.FromWarehouseID == input.ToWarehouseID {
		return nil, fmt.Errorf("%w: transfer warehouses must be different", ErrValidation)
	}
	if input.Status == "" {
		input.Status = "draft"
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
	repo, ok := s.raw.(interface {
		CreateTransfer(context.Context, CreateInventoryTransferInput) (*InventoryTransfer, error)
	})
	if !ok {
		return nil, fmt.Errorf("%w: transfer repository unavailable", ErrValidation)
	}
	return repo.CreateTransfer(ctx, input)
}

func (s *Service) ListTransfers(ctx context.Context, limit int) ([]InventoryTransfer, error) {
	repo, ok := s.raw.(interface {
		ListTransfers(context.Context, int) ([]InventoryTransfer, error)
	})
	if !ok {
		return nil, nil
	}
	return repo.ListTransfers(ctx, normalizeLimit(limit))
}

func (s *Service) CreateAdjustment(ctx context.Context, input CreateInventoryAdjustmentInput) (*InventoryAdjustment, error) {
	if input.WarehouseID == uuid.Nil {
		return nil, fmt.Errorf("%w: warehouse_id is required", ErrValidation)
	}
	if input.Status == "" {
		input.Status = "draft"
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
	repo, ok := s.raw.(interface {
		CreateAdjustment(context.Context, CreateInventoryAdjustmentInput) (*InventoryAdjustment, error)
	})
	if !ok {
		return nil, fmt.Errorf("%w: adjustment repository unavailable", ErrValidation)
	}
	return repo.CreateAdjustment(ctx, input)
}

func (s *Service) ListAdjustments(ctx context.Context, limit int) ([]InventoryAdjustment, error) {
	repo, ok := s.raw.(interface {
		ListAdjustments(context.Context, int) ([]InventoryAdjustment, error)
	})
	if !ok {
		return nil, nil
	}
	return repo.ListAdjustments(ctx, normalizeLimit(limit))
}

func (s *Service) CreateCount(ctx context.Context, input CreateInventoryCountInput) (*InventoryCount, error) {
	if input.WarehouseID == uuid.Nil {
		return nil, fmt.Errorf("%w: warehouse_id is required", ErrValidation)
	}
	if input.Status == "" {
		input.Status = "draft"
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
	repo, ok := s.raw.(interface {
		CreateCount(context.Context, CreateInventoryCountInput) (*InventoryCount, error)
	})
	if !ok {
		return nil, fmt.Errorf("%w: count repository unavailable", ErrValidation)
	}
	return repo.CreateCount(ctx, input)
}

func (s *Service) ListCounts(ctx context.Context, limit int) ([]InventoryCount, error) {
	repo, ok := s.raw.(interface {
		ListCounts(context.Context, int) ([]InventoryCount, error)
	})
	if !ok {
		return nil, nil
	}
	return repo.ListCounts(ctx, normalizeLimit(limit))
}

func movementDirection(movementType string) (int, error) {
	switch movementType {
	case MovementPurchaseReceipt, MovementSalesReturn, MovementTransferIn, MovementAdjustmentIn, MovementCountGain:
		return 1, nil
	case MovementPurchaseReturn, MovementSalesShipment, MovementTransferOut, MovementAdjustmentOut, MovementCountLoss:
		return -1, nil
	default:
		return 0, fmt.Errorf("%w: unsupported movement type %q", ErrValidation, movementType)
	}
}

func normalizeMovementInput(input *CreateInventoryMovementInput) {
	input.MovementType = strings.TrimSpace(input.MovementType)
	input.SourceType = strings.TrimSpace(input.SourceType)
	input.Currency = normalizeCurrency(input.Currency)
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
}

func normalizePartnerInput(input *CreateBusinessPartnerInput) {
	input.PartnerCode = strings.TrimSpace(input.PartnerCode)
	input.PartnerType = strings.TrimSpace(input.PartnerType)
	input.Name = strings.TrimSpace(input.Name)
	if input.Status == "" {
		input.Status = "active"
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
}

func normalizeItemInput(input *CreateItemInput) {
	input.ItemCode = strings.TrimSpace(input.ItemCode)
	input.Name = strings.TrimSpace(input.Name)
	if input.ItemType == "" {
		input.ItemType = "material"
	}
	if input.BaseUOM == "" {
		input.BaseUOM = "EA"
	}
	if input.Status == "" {
		input.Status = "active"
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
}

func normalizeWarehouseInput(input *CreateWarehouseInput) {
	input.WarehouseCode = strings.TrimSpace(input.WarehouseCode)
	input.Name = strings.TrimSpace(input.Name)
	if input.Status == "" {
		input.Status = "active"
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
}

func normalizeLocationInput(input *CreateWarehouseLocationInput) {
	input.LocationCode = strings.TrimSpace(input.LocationCode)
	input.Name = strings.TrimSpace(input.Name)
	if input.Status == "" {
		input.Status = "active"
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
}

func normalizeCurrency(value string) string {
	value = strings.TrimSpace(strings.ToUpper(value))
	if value == "" {
		return "CNY"
	}
	return value
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 100
	}
	if limit > 500 {
		return 500
	}
	return limit
}
