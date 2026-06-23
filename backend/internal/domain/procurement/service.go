package procurement

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/finance"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/inventory"
)

var (
	ErrValidation = errors.New("validation error")
	ErrNotFound   = errors.New("not found")
)

type Repository interface {
	CreateRequisition(ctx context.Context, input CreatePurchaseRequisitionInput) (*PurchaseRequisition, error)
	ListRequisitions(ctx context.Context, limit int) ([]PurchaseRequisition, error)
	GetRequisition(ctx context.Context, id uuid.UUID) (*PurchaseRequisition, error)
	UpdateRequisitionStatus(ctx context.Context, id uuid.UUID, status string) (*PurchaseRequisition, error)
	CreateOrder(ctx context.Context, input CreatePurchaseOrderInput) (*PurchaseOrder, error)
	ListOrders(ctx context.Context, limit int) ([]PurchaseOrder, error)
	GetOrder(ctx context.Context, id uuid.UUID) (*PurchaseOrder, error)
	UpdateOrderStatus(ctx context.Context, id uuid.UUID, status string) (*PurchaseOrder, error)
	CreateReceipt(ctx context.Context, input CreatePurchaseReceiptInput) (*PurchaseReceipt, error)
	ListReceipts(ctx context.Context, limit int) ([]PurchaseReceipt, error)
	GetReceipt(ctx context.Context, id uuid.UUID) (*PurchaseReceipt, error)
	MarkReceiptPosted(ctx context.Context, id uuid.UUID, payableID *uuid.UUID) (*PurchaseReceipt, error)
	CreateReturn(ctx context.Context, input CreatePurchaseReturnInput) (*PurchaseReturn, error)
	ListReturns(ctx context.Context, limit int) ([]PurchaseReturn, error)
}

type InventoryPoster interface {
	PostMovement(ctx context.Context, input inventory.CreateInventoryMovementInput) (*inventory.InventoryMovement, error)
}

type inventoryMovementFinder interface {
	FindMovementBySourceLine(ctx context.Context, sourceType string, sourceID uuid.UUID, lineKey string, lineID uuid.UUID) (*inventory.InventoryMovement, error)
}

type FinancePoster interface {
	CreatePayable(ctx context.Context, input finance.CreatePayableInput) (*finance.Payable, error)
}

type financePayableFinder interface {
	FindPayableBySource(ctx context.Context, sourceType string, sourceID uuid.UUID) (*finance.Payable, error)
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

func (s *Service) CreateRequisition(ctx context.Context, input CreatePurchaseRequisitionInput) (*PurchaseRequisition, error) {
	normalizeRequisitionInput(&input)
	if input.Title == "" {
		return nil, fmt.Errorf("%w: title is required", ErrValidation)
	}
	return s.repo.CreateRequisition(ctx, input)
}

func (s *Service) ListRequisitions(ctx context.Context, limit int) ([]PurchaseRequisition, error) {
	return s.repo.ListRequisitions(ctx, normalizeLimit(limit))
}

func (s *Service) SubmitRequisition(ctx context.Context, id uuid.UUID) (*PurchaseRequisition, error) {
	requisition, err := s.repo.GetRequisition(ctx, id)
	if err != nil {
		return nil, err
	}
	if requisition == nil {
		return nil, ErrNotFound
	}
	if requisition.Status != "draft" {
		return nil, fmt.Errorf("%w: only draft requisitions can be submitted", ErrValidation)
	}
	return s.repo.UpdateRequisitionStatus(ctx, id, "submitted")
}

func (s *Service) ApproveRequisition(ctx context.Context, id uuid.UUID) (*PurchaseRequisition, error) {
	requisition, err := s.repo.GetRequisition(ctx, id)
	if err != nil {
		return nil, err
	}
	if requisition == nil {
		return nil, ErrNotFound
	}
	if requisition.Status != "submitted" {
		return nil, fmt.Errorf("%w: only submitted requisitions can be approved", ErrValidation)
	}
	return s.repo.UpdateRequisitionStatus(ctx, id, "approved")
}

func (s *Service) CreateOrder(ctx context.Context, input CreatePurchaseOrderInput) (*PurchaseOrder, error) {
	normalizeOrderInput(&input)
	if input.SupplierID == "" && input.SupplierName == "" {
		return nil, fmt.Errorf("%w: supplier is required", ErrValidation)
	}
	return s.repo.CreateOrder(ctx, input)
}

func (s *Service) ListOrders(ctx context.Context, limit int) ([]PurchaseOrder, error) {
	return s.repo.ListOrders(ctx, normalizeLimit(limit))
}

func (s *Service) SubmitOrder(ctx context.Context, id uuid.UUID) (*PurchaseOrder, error) {
	order, err := s.repo.GetOrder(ctx, id)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, ErrNotFound
	}
	if order.Status != "draft" {
		return nil, fmt.Errorf("%w: only draft purchase orders can be submitted", ErrValidation)
	}
	return s.repo.UpdateOrderStatus(ctx, id, "submitted")
}

func (s *Service) ApproveOrder(ctx context.Context, id uuid.UUID) (*PurchaseOrder, error) {
	order, err := s.repo.GetOrder(ctx, id)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, ErrNotFound
	}
	if order.Status != "submitted" {
		return nil, fmt.Errorf("%w: only submitted purchase orders can be approved", ErrValidation)
	}
	return s.repo.UpdateOrderStatus(ctx, id, "approved")
}

func (s *Service) CreateReceipt(ctx context.Context, input CreatePurchaseReceiptInput) (*PurchaseReceipt, error) {
	normalizeReceiptInput(&input)
	if len(input.Lines) == 0 {
		return nil, fmt.Errorf("%w: at least one receipt line is required", ErrValidation)
	}
	return s.repo.CreateReceipt(ctx, input)
}

func (s *Service) ListReceipts(ctx context.Context, limit int) ([]PurchaseReceipt, error) {
	return s.repo.ListReceipts(ctx, normalizeLimit(limit))
}

func (s *Service) PostReceipt(ctx context.Context, id uuid.UUID) (*PurchaseReceipt, error) {
	if s.inventory == nil {
		return nil, fmt.Errorf("%w: inventory poster is required", ErrValidation)
	}
	if s.finance == nil {
		return nil, fmt.Errorf("%w: finance poster is required", ErrValidation)
	}
	receipt, err := s.repo.GetReceipt(ctx, id)
	if err != nil {
		return nil, err
	}
	if receipt.Status == "posted" {
		return receipt, nil
	}

	subtotal := 0.0
	taxAmount := 0.0
	for _, line := range receipt.Lines {
		if line.Quantity <= 0 {
			return nil, fmt.Errorf("%w: receipt line quantity must be greater than zero", ErrValidation)
		}
		if line.WarehouseID == uuid.Nil {
			return nil, fmt.Errorf("%w: receipt line warehouse_id is required", ErrValidation)
		}
		subtotal += line.Amount
		taxAmount += line.TaxAmount
		if movementAlreadyPosted(ctx, s.inventory, "purchase_receipt", receipt.ID, "receipt_line_id", line.ID) {
			continue
		}
		if _, err := s.inventory.PostMovement(ctx, inventory.CreateInventoryMovementInput{
			MovementType:   inventory.MovementPurchaseReceipt,
			SourceType:     "purchase_receipt",
			SourceID:       &receipt.ID,
			ItemID:         line.ItemID,
			WarehouseID:    line.WarehouseID,
			LocationID:     line.LocationID,
			Quantity:       line.Quantity,
			UnitCost:       line.UnitCost,
			Currency:       receipt.Currency,
			OrganizationID: receipt.OrganizationID,
			DepartmentID:   receipt.DepartmentID,
			Metadata:       map[string]any{"receipt_line_id": line.ID.String()},
		}); err != nil {
			return nil, err
		}
	}

	payable, err := payableForSource(ctx, s.finance, "purchase_receipt", receipt.ID, finance.CreatePayableInput{
		PayableType:    "vendor",
		SourceType:     "purchase_receipt",
		SourceID:       &receipt.ID,
		VendorID:       receipt.SupplierID,
		VendorName:     receipt.SupplierName,
		OrganizationID: receipt.OrganizationID,
		DepartmentID:   receipt.DepartmentID,
		Amount:         subtotal,
		TaxAmount:      taxAmount,
		Currency:       receipt.Currency,
		Metadata: map[string]any{
			"purchase_receipt_id": receipt.ID.String(),
		},
	})
	if err != nil {
		return nil, err
	}
	return s.repo.MarkReceiptPosted(ctx, receipt.ID, &payable.ID)
}

func (s *Service) CreateReturn(ctx context.Context, input CreatePurchaseReturnInput) (*PurchaseReturn, error) {
	normalizeReturnInput(&input)
	if len(input.Lines) == 0 {
		return nil, fmt.Errorf("%w: at least one return line is required", ErrValidation)
	}
	return s.repo.CreateReturn(ctx, input)
}

func (s *Service) ListReturns(ctx context.Context, limit int) ([]PurchaseReturn, error) {
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

func payableForSource(ctx context.Context, poster FinancePoster, sourceType string, sourceID uuid.UUID, input finance.CreatePayableInput) (*finance.Payable, error) {
	finder, ok := poster.(financePayableFinder)
	if ok {
		payable, err := finder.FindPayableBySource(ctx, sourceType, sourceID)
		if err == nil && payable != nil {
			return payable, nil
		}
		if err != nil && !errors.Is(err, finance.ErrNotFound) {
			return nil, err
		}
	}
	return poster.CreatePayable(ctx, input)
}

func normalizeRequisitionInput(input *CreatePurchaseRequisitionInput) {
	input.Title = strings.TrimSpace(input.Title)
	input.SupplierID = strings.TrimSpace(input.SupplierID)
	input.SupplierName = strings.TrimSpace(input.SupplierName)
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

func normalizeOrderInput(input *CreatePurchaseOrderInput) {
	input.SupplierID = strings.TrimSpace(input.SupplierID)
	input.SupplierName = strings.TrimSpace(input.SupplierName)
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

func normalizeReceiptInput(input *CreatePurchaseReceiptInput) {
	input.SupplierID = strings.TrimSpace(input.SupplierID)
	input.SupplierName = strings.TrimSpace(input.SupplierName)
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

func normalizeReturnInput(input *CreatePurchaseReturnInput) {
	input.SupplierID = strings.TrimSpace(input.SupplierID)
	input.SupplierName = strings.TrimSpace(input.SupplierName)
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
