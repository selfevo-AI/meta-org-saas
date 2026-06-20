package sales

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org/backend/internal/domain/finance"
	"github.com/selfevo-AI/meta-org/backend/internal/domain/inventory"
)

var (
	ErrValidation = errors.New("validation error")
	ErrNotFound   = errors.New("not found")
)

type Repository interface {
	CreateQuotation(ctx context.Context, input CreateSalesQuotationInput) (*SalesQuotation, error)
	ListQuotations(ctx context.Context, limit int) ([]SalesQuotation, error)
	CreateOrder(ctx context.Context, input CreateSalesOrderInput) (*SalesOrder, error)
	ListOrders(ctx context.Context, limit int) ([]SalesOrder, error)
	GetOrder(ctx context.Context, id uuid.UUID) (*SalesOrder, error)
	UpdateOrderStatus(ctx context.Context, id uuid.UUID, status string) (*SalesOrder, error)
	CreateShipment(ctx context.Context, input CreateSalesShipmentInput) (*SalesShipment, error)
	ListShipments(ctx context.Context, limit int) ([]SalesShipment, error)
	GetShipment(ctx context.Context, id uuid.UUID) (*SalesShipment, error)
	MarkShipmentPosted(ctx context.Context, id uuid.UUID, receivableID *uuid.UUID) (*SalesShipment, error)
	CreateReturn(ctx context.Context, input CreateSalesReturnInput) (*SalesReturn, error)
	ListReturns(ctx context.Context, limit int) ([]SalesReturn, error)
}

type InventoryPoster interface {
	PostMovement(ctx context.Context, input inventory.CreateInventoryMovementInput) (*inventory.InventoryMovement, error)
}

type inventoryMovementFinder interface {
	FindMovementBySourceLine(ctx context.Context, sourceType string, sourceID uuid.UUID, lineKey string, lineID uuid.UUID) (*inventory.InventoryMovement, error)
}

type FinancePoster interface {
	CreateReceivable(ctx context.Context, input finance.CreateReceivableInput) (*finance.Receivable, error)
}

type financeReceivableFinder interface {
	FindReceivableBySource(ctx context.Context, sourceType string, sourceID uuid.UUID) (*finance.Receivable, error)
}

type Service struct {
	repo      Repository
	inventory InventoryPoster
	finance   FinancePoster
}

type ServiceOption func(*Service)

func WithInventoryPoster(poster InventoryPoster) ServiceOption {
	return func(s *Service) {
		s.inventory = poster
	}
}

func WithFinancePoster(poster FinancePoster) ServiceOption {
	return func(s *Service) {
		s.finance = poster
	}
}

func NewService(repo Repository, opts ...ServiceOption) *Service {
	s := &Service{repo: repo}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Service) CreateQuotation(ctx context.Context, input CreateSalesQuotationInput) (*SalesQuotation, error) {
	normalizeQuotationInput(&input)
	if input.CustomerID == "" && input.CustomerName == "" {
		return nil, fmt.Errorf("%w: customer is required", ErrValidation)
	}
	return s.repo.CreateQuotation(ctx, input)
}

func (s *Service) ListQuotations(ctx context.Context, limit int) ([]SalesQuotation, error) {
	return s.repo.ListQuotations(ctx, normalizeLimit(limit))
}

func (s *Service) CreateOrder(ctx context.Context, input CreateSalesOrderInput) (*SalesOrder, error) {
	normalizeOrderInput(&input)
	if input.CustomerID == "" && input.CustomerName == "" {
		return nil, fmt.Errorf("%w: customer is required", ErrValidation)
	}
	return s.repo.CreateOrder(ctx, input)
}

func (s *Service) ListOrders(ctx context.Context, limit int) ([]SalesOrder, error) {
	return s.repo.ListOrders(ctx, normalizeLimit(limit))
}

func (s *Service) ConfirmOrder(ctx context.Context, id uuid.UUID) (*SalesOrder, error) {
	order, err := s.repo.GetOrder(ctx, id)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, ErrNotFound
	}
	if order.Status != "draft" {
		return nil, fmt.Errorf("%w: sales order must be draft to confirm", ErrValidation)
	}
	return s.repo.UpdateOrderStatus(ctx, id, "confirmed")
}

func (s *Service) ApproveOrder(ctx context.Context, id uuid.UUID) (*SalesOrder, error) {
	order, err := s.repo.GetOrder(ctx, id)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, ErrNotFound
	}
	if order.Status != "confirmed" {
		return nil, fmt.Errorf("%w: sales order must be confirmed to approve", ErrValidation)
	}
	return s.repo.UpdateOrderStatus(ctx, id, "approved")
}

func (s *Service) CreateShipment(ctx context.Context, input CreateSalesShipmentInput) (*SalesShipment, error) {
	normalizeShipmentInput(&input)
	if len(input.Lines) == 0 {
		return nil, fmt.Errorf("%w: at least one shipment line is required", ErrValidation)
	}
	return s.repo.CreateShipment(ctx, input)
}

func (s *Service) ListShipments(ctx context.Context, limit int) ([]SalesShipment, error) {
	return s.repo.ListShipments(ctx, normalizeLimit(limit))
}

func (s *Service) PostShipment(ctx context.Context, id uuid.UUID) (*SalesShipment, error) {
	if s.inventory == nil {
		return nil, fmt.Errorf("%w: inventory poster is required", ErrValidation)
	}
	if s.finance == nil {
		return nil, fmt.Errorf("%w: finance poster is required", ErrValidation)
	}
	shipment, err := s.repo.GetShipment(ctx, id)
	if err != nil {
		return nil, err
	}
	if shipment.Status == "posted" {
		return shipment, nil
	}

	subtotal := 0.0
	taxAmount := 0.0
	for _, line := range shipment.Lines {
		if line.Quantity <= 0 {
			return nil, fmt.Errorf("%w: shipment line quantity must be greater than zero", ErrValidation)
		}
		if line.WarehouseID == uuid.Nil {
			return nil, fmt.Errorf("%w: shipment line warehouse_id is required", ErrValidation)
		}
		subtotal += line.Amount
		taxAmount += line.TaxAmount
		if movementAlreadyPosted(ctx, s.inventory, "sales_shipment", shipment.ID, "shipment_line_id", line.ID) {
			continue
		}
		if _, err := s.inventory.PostMovement(ctx, inventory.CreateInventoryMovementInput{
			MovementType:   inventory.MovementSalesShipment,
			SourceType:     "sales_shipment",
			SourceID:       &shipment.ID,
			ItemID:         line.ItemID,
			WarehouseID:    line.WarehouseID,
			LocationID:     line.LocationID,
			Quantity:       line.Quantity,
			Currency:       shipment.Currency,
			OrganizationID: shipment.OrganizationID,
			DepartmentID:   shipment.DepartmentID,
			Metadata:       map[string]any{"shipment_line_id": line.ID.String()},
		}); err != nil {
			return nil, err
		}
	}

	receivable, err := receivableForSource(ctx, s.finance, "sales_shipment", shipment.ID, finance.CreateReceivableInput{
		ReceivableType: "sales",
		SourceType:     "sales_shipment",
		SourceID:       &shipment.ID,
		CustomerID:     shipment.CustomerID,
		CustomerName:   shipment.CustomerName,
		OrganizationID: shipment.OrganizationID,
		DepartmentID:   shipment.DepartmentID,
		Amount:         subtotal,
		TaxAmount:      taxAmount,
		Currency:       shipment.Currency,
		Metadata: map[string]any{
			"sales_shipment_id": shipment.ID.String(),
		},
	})
	if err != nil {
		return nil, err
	}
	return s.repo.MarkShipmentPosted(ctx, shipment.ID, &receivable.ID)
}

func (s *Service) CreateReturn(ctx context.Context, input CreateSalesReturnInput) (*SalesReturn, error) {
	normalizeReturnInput(&input)
	if len(input.Lines) == 0 {
		return nil, fmt.Errorf("%w: at least one return line is required", ErrValidation)
	}
	return s.repo.CreateReturn(ctx, input)
}

func (s *Service) ListReturns(ctx context.Context, limit int) ([]SalesReturn, error) {
	return s.repo.ListReturns(ctx, normalizeLimit(limit))
}

func movementAlreadyPosted(ctx context.Context, poster InventoryPoster, sourceType string, sourceID uuid.UUID, lineKey string, lineID uuid.UUID) bool {
	finder, ok := poster.(inventoryMovementFinder)
	if !ok {
		return false
	}
	movement, err := finder.FindMovementBySourceLine(ctx, sourceType, sourceID, lineKey, lineID)
	return err == nil && movement != nil
}

func receivableForSource(ctx context.Context, poster FinancePoster, sourceType string, sourceID uuid.UUID, input finance.CreateReceivableInput) (*finance.Receivable, error) {
	finder, ok := poster.(financeReceivableFinder)
	if ok {
		receivable, err := finder.FindReceivableBySource(ctx, sourceType, sourceID)
		if err == nil && receivable != nil {
			return receivable, nil
		}
		if err != nil && !errors.Is(err, finance.ErrNotFound) {
			return nil, err
		}
	}
	return poster.CreateReceivable(ctx, input)
}

func normalizeQuotationInput(input *CreateSalesQuotationInput) {
	input.CustomerID = strings.TrimSpace(input.CustomerID)
	input.CustomerName = strings.TrimSpace(input.CustomerName)
	input.Currency = normalizeCurrency(input.Currency)
	if input.Status == "" {
		input.Status = "draft"
	}
	if input.ApprovalStatus == "" {
		input.ApprovalStatus = "not_required"
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
}

func normalizeOrderInput(input *CreateSalesOrderInput) {
	input.CustomerID = strings.TrimSpace(input.CustomerID)
	input.CustomerName = strings.TrimSpace(input.CustomerName)
	input.Currency = normalizeCurrency(input.Currency)
	if input.Status == "" {
		input.Status = "draft"
	}
	if input.ApprovalStatus == "" {
		input.ApprovalStatus = "not_required"
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
}

func normalizeShipmentInput(input *CreateSalesShipmentInput) {
	input.CustomerID = strings.TrimSpace(input.CustomerID)
	input.CustomerName = strings.TrimSpace(input.CustomerName)
	input.Currency = normalizeCurrency(input.Currency)
	if input.Status == "" {
		input.Status = "draft"
	}
	if input.ApprovalStatus == "" {
		input.ApprovalStatus = "not_required"
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
}

func normalizeReturnInput(input *CreateSalesReturnInput) {
	input.CustomerID = strings.TrimSpace(input.CustomerID)
	input.CustomerName = strings.TrimSpace(input.CustomerName)
	input.Currency = normalizeCurrency(input.Currency)
	if input.Status == "" {
		input.Status = "draft"
	}
	if input.ApprovalStatus == "" {
		input.ApprovalStatus = "not_required"
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
