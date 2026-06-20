package inventory

import (
	"time"

	"github.com/google/uuid"
)

const (
	MovementPurchaseReceipt = "purchase_receipt"
	MovementPurchaseReturn  = "purchase_return"
	MovementSalesShipment   = "sales_shipment"
	MovementSalesReturn     = "sales_return"
	MovementTransferIn      = "transfer_in"
	MovementTransferOut     = "transfer_out"
	MovementAdjustmentIn    = "adjustment_in"
	MovementAdjustmentOut   = "adjustment_out"
	MovementCountGain       = "count_gain"
	MovementCountLoss       = "count_loss"
)

type BusinessPartner struct {
	ID             uuid.UUID      `json:"id"`
	MasterKey      string         `json:"master_key,omitempty"`
	PartnerCode    string         `json:"partner_code"`
	PartnerType    string         `json:"partner_type"`
	Name           string         `json:"name"`
	Email          string         `json:"email"`
	Phone          string         `json:"phone"`
	Status         string         `json:"status"`
	OrganizationID *uuid.UUID     `json:"organization_id,omitempty"`
	Metadata       map[string]any `json:"metadata"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type CreateBusinessPartnerInput struct {
	PartnerCode    string         `json:"partner_code,omitempty"`
	PartnerType    string         `json:"partner_type"`
	Name           string         `json:"name"`
	Email          string         `json:"email,omitempty"`
	Phone          string         `json:"phone,omitempty"`
	Status         string         `json:"status,omitempty"`
	OrganizationID *uuid.UUID     `json:"organization_id,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type Item struct {
	ID             uuid.UUID      `json:"id"`
	MasterKey      string         `json:"master_key,omitempty"`
	ItemCode       string         `json:"item_code"`
	Name           string         `json:"name"`
	ItemType       string         `json:"item_type"`
	BaseUOM        string         `json:"base_uom"`
	Status         string         `json:"status"`
	OrganizationID *uuid.UUID     `json:"organization_id,omitempty"`
	Metadata       map[string]any `json:"metadata"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type CreateItemInput struct {
	ItemCode       string         `json:"item_code,omitempty"`
	Name           string         `json:"name"`
	ItemType       string         `json:"item_type,omitempty"`
	BaseUOM        string         `json:"base_uom,omitempty"`
	Status         string         `json:"status,omitempty"`
	OrganizationID *uuid.UUID     `json:"organization_id,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type ItemUOM struct {
	ID        uuid.UUID      `json:"id"`
	MasterKey string         `json:"master_key,omitempty"`
	ItemID    uuid.UUID      `json:"item_id"`
	UOM       string         `json:"uom"`
	Factor    float64        `json:"factor"`
	IsBase    bool           `json:"is_base"`
	Metadata  map[string]any `json:"metadata"`
	CreatedAt time.Time      `json:"created_at"`
}

type Warehouse struct {
	ID             uuid.UUID      `json:"id"`
	MasterKey      string         `json:"master_key,omitempty"`
	WarehouseCode  string         `json:"warehouse_code"`
	Name           string         `json:"name"`
	Status         string         `json:"status"`
	OrganizationID *uuid.UUID     `json:"organization_id,omitempty"`
	DepartmentID   *uuid.UUID     `json:"department_id,omitempty"`
	Metadata       map[string]any `json:"metadata"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type CreateWarehouseInput struct {
	WarehouseCode  string         `json:"warehouse_code,omitempty"`
	Name           string         `json:"name"`
	Status         string         `json:"status,omitempty"`
	OrganizationID *uuid.UUID     `json:"organization_id,omitempty"`
	DepartmentID   *uuid.UUID     `json:"department_id,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type WarehouseLocation struct {
	ID           uuid.UUID      `json:"id"`
	MasterKey    string         `json:"master_key,omitempty"`
	WarehouseID  uuid.UUID      `json:"warehouse_id"`
	LocationCode string         `json:"location_code"`
	Name         string         `json:"name"`
	Status       string         `json:"status"`
	Metadata     map[string]any `json:"metadata"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type CreateWarehouseLocationInput struct {
	WarehouseID  uuid.UUID      `json:"warehouse_id"`
	LocationCode string         `json:"location_code,omitempty"`
	Name         string         `json:"name"`
	Status       string         `json:"status,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type InventoryBalance struct {
	ID             uuid.UUID      `json:"id"`
	MasterKey      string         `json:"master_key,omitempty"`
	ItemID         uuid.UUID      `json:"item_id"`
	WarehouseID    uuid.UUID      `json:"warehouse_id"`
	LocationID     *uuid.UUID     `json:"location_id,omitempty"`
	Quantity       float64        `json:"quantity"`
	ReservedQty    float64        `json:"reserved_qty"`
	AverageCost    float64        `json:"average_cost"`
	ValueAmount    float64        `json:"value_amount"`
	Currency       string         `json:"currency"`
	OrganizationID *uuid.UUID     `json:"organization_id,omitempty"`
	Metadata       map[string]any `json:"metadata"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type InventoryMovement struct {
	ID             uuid.UUID      `json:"id"`
	MasterKey      string         `json:"master_key,omitempty"`
	MovementType   string         `json:"movement_type"`
	SourceType     string         `json:"source_type"`
	SourceID       *uuid.UUID     `json:"source_id,omitempty"`
	ItemID         uuid.UUID      `json:"item_id"`
	WarehouseID    uuid.UUID      `json:"warehouse_id"`
	LocationID     *uuid.UUID     `json:"location_id,omitempty"`
	Quantity       float64        `json:"quantity"`
	UnitCost       float64        `json:"unit_cost"`
	Amount         float64        `json:"amount"`
	Currency       string         `json:"currency"`
	BalanceAfter   float64        `json:"balance_after"`
	OrganizationID *uuid.UUID     `json:"organization_id,omitempty"`
	DepartmentID   *uuid.UUID     `json:"department_id,omitempty"`
	OccurredAt     time.Time      `json:"occurred_at"`
	Metadata       map[string]any `json:"metadata"`
	CreatedAt      time.Time      `json:"created_at"`
}

type CreateInventoryMovementInput struct {
	MovementType   string         `json:"movement_type"`
	SourceType     string         `json:"source_type,omitempty"`
	SourceID       *uuid.UUID     `json:"source_id,omitempty"`
	ItemID         uuid.UUID      `json:"item_id"`
	WarehouseID    uuid.UUID      `json:"warehouse_id"`
	LocationID     *uuid.UUID     `json:"location_id,omitempty"`
	Quantity       float64        `json:"quantity"`
	UnitCost       float64        `json:"unit_cost,omitempty"`
	Currency       string         `json:"currency,omitempty"`
	OrganizationID *uuid.UUID     `json:"organization_id,omitempty"`
	DepartmentID   *uuid.UUID     `json:"department_id,omitempty"`
	OccurredAt     *time.Time     `json:"occurred_at,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type InventoryTransfer struct {
	ID              uuid.UUID               `json:"id"`
	MasterKey       string                  `json:"master_key,omitempty"`
	TransferNumber  string                  `json:"transfer_number"`
	FromWarehouseID uuid.UUID               `json:"from_warehouse_id"`
	ToWarehouseID   uuid.UUID               `json:"to_warehouse_id"`
	Status          string                  `json:"status"`
	OrganizationID  *uuid.UUID              `json:"organization_id,omitempty"`
	DepartmentID    *uuid.UUID              `json:"department_id,omitempty"`
	WorkflowID      *uuid.UUID              `json:"workflow_instance_id,omitempty"`
	Metadata        map[string]any          `json:"metadata"`
	Lines           []InventoryTransferLine `json:"lines,omitempty"`
	CreatedAt       time.Time               `json:"created_at"`
	UpdatedAt       time.Time               `json:"updated_at"`
}

type InventoryTransferLine struct {
	ID         uuid.UUID `json:"id"`
	TransferID uuid.UUID `json:"transfer_id"`
	ItemID     uuid.UUID `json:"item_id"`
	Quantity   float64   `json:"quantity"`
	UnitCost   float64   `json:"unit_cost"`
}

type CreateInventoryTransferInput struct {
	TransferNumber  string                             `json:"transfer_number,omitempty"`
	FromWarehouseID uuid.UUID                          `json:"from_warehouse_id"`
	ToWarehouseID   uuid.UUID                          `json:"to_warehouse_id"`
	Status          string                             `json:"status,omitempty"`
	OrganizationID  *uuid.UUID                         `json:"organization_id,omitempty"`
	DepartmentID    *uuid.UUID                         `json:"department_id,omitempty"`
	WorkflowID      *uuid.UUID                         `json:"workflow_instance_id,omitempty"`
	Metadata        map[string]any                     `json:"metadata,omitempty"`
	Lines           []CreateInventoryTransferLineInput `json:"lines,omitempty"`
}

type CreateInventoryTransferLineInput struct {
	ItemID   uuid.UUID `json:"item_id"`
	Quantity float64   `json:"quantity"`
	UnitCost float64   `json:"unit_cost,omitempty"`
}

type InventoryAdjustment struct {
	ID               uuid.UUID                 `json:"id"`
	MasterKey        string                    `json:"master_key,omitempty"`
	AdjustmentNumber string                    `json:"adjustment_number"`
	WarehouseID      uuid.UUID                 `json:"warehouse_id"`
	Reason           string                    `json:"reason"`
	Status           string                    `json:"status"`
	OrganizationID   *uuid.UUID                `json:"organization_id,omitempty"`
	DepartmentID     *uuid.UUID                `json:"department_id,omitempty"`
	WorkflowID       *uuid.UUID                `json:"workflow_instance_id,omitempty"`
	Metadata         map[string]any            `json:"metadata"`
	Lines            []InventoryAdjustmentLine `json:"lines,omitempty"`
	CreatedAt        time.Time                 `json:"created_at"`
	UpdatedAt        time.Time                 `json:"updated_at"`
}

type InventoryAdjustmentLine struct {
	ID            uuid.UUID `json:"id"`
	AdjustmentID  uuid.UUID `json:"adjustment_id"`
	ItemID        uuid.UUID `json:"item_id"`
	QuantityDelta float64   `json:"quantity_delta"`
	UnitCost      float64   `json:"unit_cost"`
}

type CreateInventoryAdjustmentInput struct {
	AdjustmentNumber string                               `json:"adjustment_number,omitempty"`
	WarehouseID      uuid.UUID                            `json:"warehouse_id"`
	Reason           string                               `json:"reason,omitempty"`
	Status           string                               `json:"status,omitempty"`
	OrganizationID   *uuid.UUID                           `json:"organization_id,omitempty"`
	DepartmentID     *uuid.UUID                           `json:"department_id,omitempty"`
	WorkflowID       *uuid.UUID                           `json:"workflow_instance_id,omitempty"`
	Metadata         map[string]any                       `json:"metadata,omitempty"`
	Lines            []CreateInventoryAdjustmentLineInput `json:"lines,omitempty"`
}

type CreateInventoryAdjustmentLineInput struct {
	ItemID        uuid.UUID `json:"item_id"`
	QuantityDelta float64   `json:"quantity_delta"`
	UnitCost      float64   `json:"unit_cost,omitempty"`
}

type InventoryCount struct {
	ID             uuid.UUID            `json:"id"`
	MasterKey      string               `json:"master_key,omitempty"`
	CountNumber    string               `json:"count_number"`
	WarehouseID    uuid.UUID            `json:"warehouse_id"`
	Status         string               `json:"status"`
	OrganizationID *uuid.UUID           `json:"organization_id,omitempty"`
	DepartmentID   *uuid.UUID           `json:"department_id,omitempty"`
	WorkflowID     *uuid.UUID           `json:"workflow_instance_id,omitempty"`
	Metadata       map[string]any       `json:"metadata"`
	Lines          []InventoryCountLine `json:"lines,omitempty"`
	CreatedAt      time.Time            `json:"created_at"`
	UpdatedAt      time.Time            `json:"updated_at"`
}

type InventoryCountLine struct {
	ID          uuid.UUID `json:"id"`
	CountID     uuid.UUID `json:"count_id"`
	ItemID      uuid.UUID `json:"item_id"`
	BookQty     float64   `json:"book_qty"`
	CountedQty  float64   `json:"counted_qty"`
	VarianceQty float64   `json:"variance_qty"`
}

type CreateInventoryCountInput struct {
	CountNumber    string                          `json:"count_number,omitempty"`
	WarehouseID    uuid.UUID                       `json:"warehouse_id"`
	Status         string                          `json:"status,omitempty"`
	OrganizationID *uuid.UUID                      `json:"organization_id,omitempty"`
	DepartmentID   *uuid.UUID                      `json:"department_id,omitempty"`
	WorkflowID     *uuid.UUID                      `json:"workflow_instance_id,omitempty"`
	Metadata       map[string]any                  `json:"metadata,omitempty"`
	Lines          []CreateInventoryCountLineInput `json:"lines,omitempty"`
}

type CreateInventoryCountLineInput struct {
	ItemID     uuid.UUID `json:"item_id"`
	BookQty    float64   `json:"book_qty,omitempty"`
	CountedQty float64   `json:"counted_qty"`
}
