package procurement

import (
	"time"

	"github.com/google/uuid"
)

type PurchaseRequisition struct {
	ID             uuid.UUID                 `json:"id"`
	MasterKey      string                    `json:"master_key,omitempty"`
	Title          string                    `json:"title"`
	SupplierID     string                    `json:"supplier_id"`
	SupplierName   string                    `json:"supplier_name"`
	Status         string                    `json:"status"`
	ApprovalStatus string                    `json:"approval_status"`
	OrganizationID *uuid.UUID                `json:"organization_id,omitempty"`
	DepartmentID   *uuid.UUID                `json:"department_id,omitempty"`
	WorkflowID     *uuid.UUID                `json:"workflow_instance_id,omitempty"`
	Currency       string                    `json:"currency"`
	TotalAmount    float64                   `json:"total_amount"`
	Metadata       map[string]any            `json:"metadata"`
	Lines          []PurchaseRequisitionLine `json:"lines,omitempty"`
	CreatedAt      time.Time                 `json:"created_at"`
	UpdatedAt      time.Time                 `json:"updated_at"`
}

type PurchaseRequisitionLine struct {
	ID            uuid.UUID      `json:"id"`
	RequisitionID uuid.UUID      `json:"requisition_id"`
	ItemID        uuid.UUID      `json:"item_id"`
	WarehouseID   uuid.UUID      `json:"warehouse_id"`
	Quantity      float64        `json:"quantity"`
	UnitCost      float64        `json:"unit_cost"`
	Amount        float64        `json:"amount"`
	Metadata      map[string]any `json:"metadata"`
}

type CreatePurchaseRequisitionInput struct {
	Title          string                               `json:"title"`
	SupplierID     string                               `json:"supplier_id,omitempty"`
	SupplierName   string                               `json:"supplier_name,omitempty"`
	Status         string                               `json:"status,omitempty"`
	ApprovalStatus string                               `json:"approval_status,omitempty"`
	OrganizationID *uuid.UUID                           `json:"organization_id,omitempty"`
	DepartmentID   *uuid.UUID                           `json:"department_id,omitempty"`
	WorkflowID     *uuid.UUID                           `json:"workflow_instance_id,omitempty"`
	Currency       string                               `json:"currency,omitempty"`
	Metadata       map[string]any                       `json:"metadata,omitempty"`
	Lines          []CreatePurchaseRequisitionLineInput `json:"lines,omitempty"`
}

type CreatePurchaseRequisitionLineInput struct {
	ItemID      uuid.UUID      `json:"item_id"`
	WarehouseID uuid.UUID      `json:"warehouse_id"`
	Quantity    float64        `json:"quantity"`
	UnitCost    float64        `json:"unit_cost"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type PurchaseOrder struct {
	ID             uuid.UUID           `json:"id"`
	MasterKey      string              `json:"master_key,omitempty"`
	OrderNumber    string              `json:"order_number"`
	RequisitionID  *uuid.UUID          `json:"requisition_id,omitempty"`
	SupplierID     string              `json:"supplier_id"`
	SupplierName   string              `json:"supplier_name"`
	Status         string              `json:"status"`
	ApprovalStatus string              `json:"approval_status"`
	OrganizationID *uuid.UUID          `json:"organization_id,omitempty"`
	DepartmentID   *uuid.UUID          `json:"department_id,omitempty"`
	WorkflowID     *uuid.UUID          `json:"workflow_instance_id,omitempty"`
	Currency       string              `json:"currency"`
	Subtotal       float64             `json:"subtotal"`
	TaxAmount      float64             `json:"tax_amount"`
	TotalAmount    float64             `json:"total_amount"`
	Metadata       map[string]any      `json:"metadata"`
	Lines          []PurchaseOrderLine `json:"lines,omitempty"`
	CreatedAt      time.Time           `json:"created_at"`
	UpdatedAt      time.Time           `json:"updated_at"`
}

type PurchaseOrderLine struct {
	ID          uuid.UUID      `json:"id"`
	OrderID     uuid.UUID      `json:"order_id"`
	ItemID      uuid.UUID      `json:"item_id"`
	WarehouseID uuid.UUID      `json:"warehouse_id"`
	Quantity    float64        `json:"quantity"`
	UnitCost    float64        `json:"unit_cost"`
	TaxRate     float64        `json:"tax_rate"`
	Amount      float64        `json:"amount"`
	TaxAmount   float64        `json:"tax_amount"`
	TotalAmount float64        `json:"total_amount"`
	Metadata    map[string]any `json:"metadata"`
}

type CreatePurchaseOrderInput struct {
	OrderNumber    string                         `json:"order_number,omitempty"`
	RequisitionID  *uuid.UUID                     `json:"requisition_id,omitempty"`
	SupplierID     string                         `json:"supplier_id,omitempty"`
	SupplierName   string                         `json:"supplier_name,omitempty"`
	Status         string                         `json:"status,omitempty"`
	ApprovalStatus string                         `json:"approval_status,omitempty"`
	OrganizationID *uuid.UUID                     `json:"organization_id,omitempty"`
	DepartmentID   *uuid.UUID                     `json:"department_id,omitempty"`
	WorkflowID     *uuid.UUID                     `json:"workflow_instance_id,omitempty"`
	Currency       string                         `json:"currency,omitempty"`
	Metadata       map[string]any                 `json:"metadata,omitempty"`
	Lines          []CreatePurchaseOrderLineInput `json:"lines,omitempty"`
}

type CreatePurchaseOrderLineInput struct {
	ItemID      uuid.UUID      `json:"item_id"`
	WarehouseID uuid.UUID      `json:"warehouse_id"`
	Quantity    float64        `json:"quantity"`
	UnitCost    float64        `json:"unit_cost"`
	TaxRate     float64        `json:"tax_rate,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type PurchaseReceipt struct {
	ID             uuid.UUID             `json:"id"`
	MasterKey      string                `json:"master_key,omitempty"`
	ReceiptNumber  string                `json:"receipt_number"`
	OrderID        *uuid.UUID            `json:"order_id,omitempty"`
	SupplierID     string                `json:"supplier_id"`
	SupplierName   string                `json:"supplier_name"`
	Status         string                `json:"status"`
	ApprovalStatus string                `json:"approval_status"`
	OrganizationID *uuid.UUID            `json:"organization_id,omitempty"`
	DepartmentID   *uuid.UUID            `json:"department_id,omitempty"`
	WorkflowID     *uuid.UUID            `json:"workflow_instance_id,omitempty"`
	WarehouseID    *uuid.UUID            `json:"warehouse_id,omitempty"`
	PayableID      *uuid.UUID            `json:"payable_id,omitempty"`
	Currency       string                `json:"currency"`
	Subtotal       float64               `json:"subtotal"`
	TaxAmount      float64               `json:"tax_amount"`
	TotalAmount    float64               `json:"total_amount"`
	Metadata       map[string]any        `json:"metadata"`
	Lines          []PurchaseReceiptLine `json:"lines,omitempty"`
	CreatedAt      time.Time             `json:"created_at"`
	UpdatedAt      time.Time             `json:"updated_at"`
}

type PurchaseReceiptLine struct {
	ID          uuid.UUID      `json:"id"`
	ReceiptID   uuid.UUID      `json:"receipt_id"`
	OrderLineID *uuid.UUID     `json:"order_line_id,omitempty"`
	ItemID      uuid.UUID      `json:"item_id"`
	WarehouseID uuid.UUID      `json:"warehouse_id"`
	LocationID  *uuid.UUID     `json:"location_id,omitempty"`
	Quantity    float64        `json:"quantity"`
	UnitCost    float64        `json:"unit_cost"`
	TaxRate     float64        `json:"tax_rate"`
	Amount      float64        `json:"amount"`
	TaxAmount   float64        `json:"tax_amount"`
	TotalAmount float64        `json:"total_amount"`
	Metadata    map[string]any `json:"metadata"`
}

type CreatePurchaseReceiptInput struct {
	ReceiptNumber  string                           `json:"receipt_number,omitempty"`
	OrderID        *uuid.UUID                       `json:"order_id,omitempty"`
	SupplierID     string                           `json:"supplier_id,omitempty"`
	SupplierName   string                           `json:"supplier_name,omitempty"`
	Status         string                           `json:"status,omitempty"`
	ApprovalStatus string                           `json:"approval_status,omitempty"`
	OrganizationID *uuid.UUID                       `json:"organization_id,omitempty"`
	DepartmentID   *uuid.UUID                       `json:"department_id,omitempty"`
	WorkflowID     *uuid.UUID                       `json:"workflow_instance_id,omitempty"`
	WarehouseID    *uuid.UUID                       `json:"warehouse_id,omitempty"`
	Currency       string                           `json:"currency,omitempty"`
	Metadata       map[string]any                   `json:"metadata,omitempty"`
	Lines          []CreatePurchaseReceiptLineInput `json:"lines,omitempty"`
}

type CreatePurchaseReceiptLineInput struct {
	OrderLineID *uuid.UUID     `json:"order_line_id,omitempty"`
	ItemID      uuid.UUID      `json:"item_id"`
	WarehouseID uuid.UUID      `json:"warehouse_id"`
	LocationID  *uuid.UUID     `json:"location_id,omitempty"`
	Quantity    float64        `json:"quantity"`
	UnitCost    float64        `json:"unit_cost"`
	TaxRate     float64        `json:"tax_rate,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type PurchaseReturn struct {
	ID             uuid.UUID            `json:"id"`
	MasterKey      string               `json:"master_key,omitempty"`
	ReturnNumber   string               `json:"return_number"`
	ReceiptID      *uuid.UUID           `json:"receipt_id,omitempty"`
	SupplierID     string               `json:"supplier_id"`
	SupplierName   string               `json:"supplier_name"`
	Status         string               `json:"status"`
	ApprovalStatus string               `json:"approval_status"`
	OrganizationID *uuid.UUID           `json:"organization_id,omitempty"`
	DepartmentID   *uuid.UUID           `json:"department_id,omitempty"`
	WorkflowID     *uuid.UUID           `json:"workflow_instance_id,omitempty"`
	Currency       string               `json:"currency"`
	Subtotal       float64              `json:"subtotal"`
	TaxAmount      float64              `json:"tax_amount"`
	TotalAmount    float64              `json:"total_amount"`
	Metadata       map[string]any       `json:"metadata"`
	Lines          []PurchaseReturnLine `json:"lines,omitempty"`
	CreatedAt      time.Time            `json:"created_at"`
	UpdatedAt      time.Time            `json:"updated_at"`
}

type PurchaseReturnLine struct {
	ID          uuid.UUID      `json:"id"`
	ReturnID    uuid.UUID      `json:"return_id"`
	ItemID      uuid.UUID      `json:"item_id"`
	WarehouseID uuid.UUID      `json:"warehouse_id"`
	LocationID  *uuid.UUID     `json:"location_id,omitempty"`
	Quantity    float64        `json:"quantity"`
	UnitCost    float64        `json:"unit_cost"`
	TaxAmount   float64        `json:"tax_amount"`
	Amount      float64        `json:"amount"`
	TotalAmount float64        `json:"total_amount"`
	Metadata    map[string]any `json:"metadata"`
}

type CreatePurchaseReturnInput struct {
	ReturnNumber   string                          `json:"return_number,omitempty"`
	ReceiptID      *uuid.UUID                      `json:"receipt_id,omitempty"`
	SupplierID     string                          `json:"supplier_id,omitempty"`
	SupplierName   string                          `json:"supplier_name,omitempty"`
	Status         string                          `json:"status,omitempty"`
	ApprovalStatus string                          `json:"approval_status,omitempty"`
	OrganizationID *uuid.UUID                      `json:"organization_id,omitempty"`
	DepartmentID   *uuid.UUID                      `json:"department_id,omitempty"`
	WorkflowID     *uuid.UUID                      `json:"workflow_instance_id,omitempty"`
	Currency       string                          `json:"currency,omitempty"`
	Metadata       map[string]any                  `json:"metadata,omitempty"`
	Lines          []CreatePurchaseReturnLineInput `json:"lines,omitempty"`
}

type CreatePurchaseReturnLineInput struct {
	ItemID      uuid.UUID      `json:"item_id"`
	WarehouseID uuid.UUID      `json:"warehouse_id"`
	LocationID  *uuid.UUID     `json:"location_id,omitempty"`
	Quantity    float64        `json:"quantity"`
	UnitCost    float64        `json:"unit_cost"`
	TaxRate     float64        `json:"tax_rate,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}
