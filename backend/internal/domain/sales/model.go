package sales

import (
	"time"

	"github.com/google/uuid"
)

type SalesQuotation struct {
	ID              uuid.UUID            `json:"id"`
	MasterKey       string               `json:"master_key,omitempty"`
	QuotationNumber string               `json:"quotation_number"`
	CustomerID      string               `json:"customer_id"`
	CustomerName    string               `json:"customer_name"`
	Status          string               `json:"status"`
	ApprovalStatus  string               `json:"approval_status"`
	OrganizationID  *uuid.UUID           `json:"organization_id,omitempty"`
	DepartmentID    *uuid.UUID           `json:"department_id,omitempty"`
	WorkflowID      *uuid.UUID           `json:"workflow_instance_id,omitempty"`
	Currency        string               `json:"currency"`
	Subtotal        float64              `json:"subtotal"`
	TaxAmount       float64              `json:"tax_amount"`
	TotalAmount     float64              `json:"total_amount"`
	Metadata        map[string]any       `json:"metadata"`
	Lines           []SalesQuotationLine `json:"lines,omitempty"`
	CreatedAt       time.Time            `json:"created_at"`
	UpdatedAt       time.Time            `json:"updated_at"`
}

type SalesQuotationLine struct {
	ID          uuid.UUID      `json:"id"`
	QuotationID uuid.UUID      `json:"quotation_id"`
	ItemID      uuid.UUID      `json:"item_id"`
	WarehouseID uuid.UUID      `json:"warehouse_id"`
	Quantity    float64        `json:"quantity"`
	UnitPrice   float64        `json:"unit_price"`
	TaxRate     float64        `json:"tax_rate"`
	Amount      float64        `json:"amount"`
	TaxAmount   float64        `json:"tax_amount"`
	TotalAmount float64        `json:"total_amount"`
	Metadata    map[string]any `json:"metadata"`
}

type CreateSalesQuotationInput struct {
	QuotationNumber string                          `json:"quotation_number,omitempty"`
	CustomerID      string                          `json:"customer_id,omitempty"`
	CustomerName    string                          `json:"customer_name,omitempty"`
	Status          string                          `json:"status,omitempty"`
	ApprovalStatus  string                          `json:"approval_status,omitempty"`
	OrganizationID  *uuid.UUID                      `json:"organization_id,omitempty"`
	DepartmentID    *uuid.UUID                      `json:"department_id,omitempty"`
	WorkflowID      *uuid.UUID                      `json:"workflow_instance_id,omitempty"`
	Currency        string                          `json:"currency,omitempty"`
	Metadata        map[string]any                  `json:"metadata,omitempty"`
	Lines           []CreateSalesQuotationLineInput `json:"lines,omitempty"`
}

type CreateSalesQuotationLineInput struct {
	ItemID      uuid.UUID      `json:"item_id"`
	WarehouseID uuid.UUID      `json:"warehouse_id"`
	Quantity    float64        `json:"quantity"`
	UnitPrice   float64        `json:"unit_price"`
	TaxRate     float64        `json:"tax_rate,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type SalesOrder struct {
	ID             uuid.UUID        `json:"id"`
	MasterKey      string           `json:"master_key,omitempty"`
	OrderNumber    string           `json:"order_number"`
	QuotationID    *uuid.UUID       `json:"quotation_id,omitempty"`
	CustomerID     string           `json:"customer_id"`
	CustomerName   string           `json:"customer_name"`
	Status         string           `json:"status"`
	ApprovalStatus string           `json:"approval_status"`
	OrganizationID *uuid.UUID       `json:"organization_id,omitempty"`
	DepartmentID   *uuid.UUID       `json:"department_id,omitempty"`
	WorkflowID     *uuid.UUID       `json:"workflow_instance_id,omitempty"`
	Currency       string           `json:"currency"`
	Subtotal       float64          `json:"subtotal"`
	TaxAmount      float64          `json:"tax_amount"`
	TotalAmount    float64          `json:"total_amount"`
	Metadata       map[string]any   `json:"metadata"`
	Lines          []SalesOrderLine `json:"lines,omitempty"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

type SalesOrderLine struct {
	ID          uuid.UUID      `json:"id"`
	OrderID     uuid.UUID      `json:"order_id"`
	ItemID      uuid.UUID      `json:"item_id"`
	WarehouseID uuid.UUID      `json:"warehouse_id"`
	Quantity    float64        `json:"quantity"`
	UnitPrice   float64        `json:"unit_price"`
	TaxRate     float64        `json:"tax_rate"`
	Amount      float64        `json:"amount"`
	TaxAmount   float64        `json:"tax_amount"`
	TotalAmount float64        `json:"total_amount"`
	Metadata    map[string]any `json:"metadata"`
}

type CreateSalesOrderInput struct {
	OrderNumber    string                      `json:"order_number,omitempty"`
	QuotationID    *uuid.UUID                  `json:"quotation_id,omitempty"`
	CustomerID     string                      `json:"customer_id,omitempty"`
	CustomerName   string                      `json:"customer_name,omitempty"`
	Status         string                      `json:"status,omitempty"`
	ApprovalStatus string                      `json:"approval_status,omitempty"`
	OrganizationID *uuid.UUID                  `json:"organization_id,omitempty"`
	DepartmentID   *uuid.UUID                  `json:"department_id,omitempty"`
	WorkflowID     *uuid.UUID                  `json:"workflow_instance_id,omitempty"`
	Currency       string                      `json:"currency,omitempty"`
	Metadata       map[string]any              `json:"metadata,omitempty"`
	Lines          []CreateSalesOrderLineInput `json:"lines,omitempty"`
}

type CreateSalesOrderLineInput struct {
	ItemID      uuid.UUID      `json:"item_id"`
	WarehouseID uuid.UUID      `json:"warehouse_id"`
	Quantity    float64        `json:"quantity"`
	UnitPrice   float64        `json:"unit_price"`
	TaxRate     float64        `json:"tax_rate,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type SalesShipment struct {
	ID             uuid.UUID           `json:"id"`
	MasterKey      string              `json:"master_key,omitempty"`
	ShipmentNumber string              `json:"shipment_number"`
	OrderID        *uuid.UUID          `json:"order_id,omitempty"`
	CustomerID     string              `json:"customer_id"`
	CustomerName   string              `json:"customer_name"`
	Status         string              `json:"status"`
	ApprovalStatus string              `json:"approval_status"`
	OrganizationID *uuid.UUID          `json:"organization_id,omitempty"`
	DepartmentID   *uuid.UUID          `json:"department_id,omitempty"`
	WorkflowID     *uuid.UUID          `json:"workflow_instance_id,omitempty"`
	ReceivableID   *uuid.UUID          `json:"receivable_id,omitempty"`
	Currency       string              `json:"currency"`
	Subtotal       float64             `json:"subtotal"`
	TaxAmount      float64             `json:"tax_amount"`
	TotalAmount    float64             `json:"total_amount"`
	Metadata       map[string]any      `json:"metadata"`
	Lines          []SalesShipmentLine `json:"lines,omitempty"`
	CreatedAt      time.Time           `json:"created_at"`
	UpdatedAt      time.Time           `json:"updated_at"`
}

type SalesShipmentLine struct {
	ID          uuid.UUID      `json:"id"`
	ShipmentID  uuid.UUID      `json:"shipment_id"`
	OrderLineID *uuid.UUID     `json:"order_line_id,omitempty"`
	ItemID      uuid.UUID      `json:"item_id"`
	WarehouseID uuid.UUID      `json:"warehouse_id"`
	LocationID  *uuid.UUID     `json:"location_id,omitempty"`
	Quantity    float64        `json:"quantity"`
	UnitPrice   float64        `json:"unit_price"`
	TaxRate     float64        `json:"tax_rate"`
	Amount      float64        `json:"amount"`
	TaxAmount   float64        `json:"tax_amount"`
	TotalAmount float64        `json:"total_amount"`
	Metadata    map[string]any `json:"metadata"`
}

type CreateSalesShipmentInput struct {
	ShipmentNumber string                         `json:"shipment_number,omitempty"`
	OrderID        *uuid.UUID                     `json:"order_id,omitempty"`
	CustomerID     string                         `json:"customer_id,omitempty"`
	CustomerName   string                         `json:"customer_name,omitempty"`
	Status         string                         `json:"status,omitempty"`
	ApprovalStatus string                         `json:"approval_status,omitempty"`
	OrganizationID *uuid.UUID                     `json:"organization_id,omitempty"`
	DepartmentID   *uuid.UUID                     `json:"department_id,omitempty"`
	WorkflowID     *uuid.UUID                     `json:"workflow_instance_id,omitempty"`
	Currency       string                         `json:"currency,omitempty"`
	Metadata       map[string]any                 `json:"metadata,omitempty"`
	Lines          []CreateSalesShipmentLineInput `json:"lines,omitempty"`
}

type CreateSalesShipmentLineInput struct {
	OrderLineID *uuid.UUID     `json:"order_line_id,omitempty"`
	ItemID      uuid.UUID      `json:"item_id"`
	WarehouseID uuid.UUID      `json:"warehouse_id"`
	LocationID  *uuid.UUID     `json:"location_id,omitempty"`
	Quantity    float64        `json:"quantity"`
	UnitPrice   float64        `json:"unit_price"`
	TaxRate     float64        `json:"tax_rate,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type SalesReturn struct {
	ID             uuid.UUID         `json:"id"`
	MasterKey      string            `json:"master_key,omitempty"`
	ReturnNumber   string            `json:"return_number"`
	ShipmentID     *uuid.UUID        `json:"shipment_id,omitempty"`
	CustomerID     string            `json:"customer_id"`
	CustomerName   string            `json:"customer_name"`
	Status         string            `json:"status"`
	ApprovalStatus string            `json:"approval_status"`
	OrganizationID *uuid.UUID        `json:"organization_id,omitempty"`
	DepartmentID   *uuid.UUID        `json:"department_id,omitempty"`
	WorkflowID     *uuid.UUID        `json:"workflow_instance_id,omitempty"`
	Currency       string            `json:"currency"`
	Subtotal       float64           `json:"subtotal"`
	TaxAmount      float64           `json:"tax_amount"`
	TotalAmount    float64           `json:"total_amount"`
	Metadata       map[string]any    `json:"metadata"`
	Lines          []SalesReturnLine `json:"lines,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

type SalesReturnLine struct {
	ID          uuid.UUID      `json:"id"`
	ReturnID    uuid.UUID      `json:"return_id"`
	ItemID      uuid.UUID      `json:"item_id"`
	WarehouseID uuid.UUID      `json:"warehouse_id"`
	LocationID  *uuid.UUID     `json:"location_id,omitempty"`
	Quantity    float64        `json:"quantity"`
	UnitPrice   float64        `json:"unit_price"`
	TaxAmount   float64        `json:"tax_amount"`
	Amount      float64        `json:"amount"`
	TotalAmount float64        `json:"total_amount"`
	Metadata    map[string]any `json:"metadata"`
}

type CreateSalesReturnInput struct {
	ReturnNumber   string                       `json:"return_number,omitempty"`
	ShipmentID     *uuid.UUID                   `json:"shipment_id,omitempty"`
	CustomerID     string                       `json:"customer_id,omitempty"`
	CustomerName   string                       `json:"customer_name,omitempty"`
	Status         string                       `json:"status,omitempty"`
	ApprovalStatus string                       `json:"approval_status,omitempty"`
	OrganizationID *uuid.UUID                   `json:"organization_id,omitempty"`
	DepartmentID   *uuid.UUID                   `json:"department_id,omitempty"`
	WorkflowID     *uuid.UUID                   `json:"workflow_instance_id,omitempty"`
	Currency       string                       `json:"currency,omitempty"`
	Metadata       map[string]any               `json:"metadata,omitempty"`
	Lines          []CreateSalesReturnLineInput `json:"lines,omitempty"`
}

type CreateSalesReturnLineInput struct {
	ItemID      uuid.UUID      `json:"item_id"`
	WarehouseID uuid.UUID      `json:"warehouse_id"`
	LocationID  *uuid.UUID     `json:"location_id,omitempty"`
	Quantity    float64        `json:"quantity"`
	UnitPrice   float64        `json:"unit_price"`
	TaxRate     float64        `json:"tax_rate,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}
