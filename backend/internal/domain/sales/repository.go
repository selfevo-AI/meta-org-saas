package sales

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/tenantdb"
)

type PostgresRepository struct {
	db tenantdb.DB
}

func NewRepository(db tenantdb.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) CreateQuotation(ctx context.Context, input CreateSalesQuotationInput) (*SalesQuotation, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	quotation, err := insertQuotation(ctx, tx, input)
	if err != nil {
		return nil, err
	}
	for _, line := range input.Lines {
		created, err := insertQuotationLine(ctx, tx, quotation.ID, line)
		if err != nil {
			return nil, err
		}
		quotation.Subtotal += created.Amount
		quotation.TaxAmount += created.TaxAmount
		quotation.TotalAmount += created.TotalAmount
		quotation.Lines = append(quotation.Lines, *created)
	}
	if err := updateQuotationTotals(ctx, tx, quotation.ID, quotation.Subtotal, quotation.TaxAmount, quotation.TotalAmount); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return quotation, nil
}

func (r *PostgresRepository) ListQuotations(ctx context.Context, limit int) ([]SalesQuotation, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, master_key, quotation_number, customer_id, customer_name, status, approval_status,
		       organization_id, department_id, workflow_instance_id, currency, subtotal::float8, tax_amount::float8,
		       total_amount::float8, metadata, created_at, updated_at
		FROM sales_quotations
		ORDER BY created_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SalesQuotation
	for rows.Next() {
		item, err := scanQuotation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) CreateOrder(ctx context.Context, input CreateSalesOrderInput) (*SalesOrder, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	order, err := insertOrder(ctx, tx, input)
	if err != nil {
		return nil, err
	}
	for _, line := range input.Lines {
		created, err := insertOrderLine(ctx, tx, order.ID, line)
		if err != nil {
			return nil, err
		}
		order.Subtotal += created.Amount
		order.TaxAmount += created.TaxAmount
		order.TotalAmount += created.TotalAmount
		order.Lines = append(order.Lines, *created)
	}
	if err := updateOrderTotals(ctx, tx, order.ID, order.Subtotal, order.TaxAmount, order.TotalAmount); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return order, nil
}

func (r *PostgresRepository) ListOrders(ctx context.Context, limit int) ([]SalesOrder, error) {
	rows, err := r.db.Query(ctx, orderSelect()+` ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SalesOrder
	for rows.Next() {
		item, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) GetOrder(ctx context.Context, id uuid.UUID) (*SalesOrder, error) {
	item, err := scanOrder(r.db.QueryRow(ctx, orderSelect()+` WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return item, err
}

func (r *PostgresRepository) UpdateOrderStatus(ctx context.Context, id uuid.UUID, status string) (*SalesOrder, error) {
	item, err := scanOrder(r.db.QueryRow(ctx, orderSelect(`
		UPDATE sales_orders
		SET status = $2, approval_status = CASE WHEN $2 = 'approved' THEN 'approved' ELSE approval_status END, updated_at = NOW()
		WHERE id = $1
		RETURNING `), id, status))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return item, err
}

func (r *PostgresRepository) CreateShipment(ctx context.Context, input CreateSalesShipmentInput) (*SalesShipment, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	shipment, err := insertShipment(ctx, tx, input)
	if err != nil {
		return nil, err
	}
	for _, line := range input.Lines {
		created, err := insertShipmentLine(ctx, tx, shipment.ID, line)
		if err != nil {
			return nil, err
		}
		shipment.Subtotal += created.Amount
		shipment.TaxAmount += created.TaxAmount
		shipment.TotalAmount += created.TotalAmount
		shipment.Lines = append(shipment.Lines, *created)
	}
	if err := updateShipmentTotals(ctx, tx, shipment.ID, shipment.Subtotal, shipment.TaxAmount, shipment.TotalAmount); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return shipment, nil
}

func (r *PostgresRepository) ListShipments(ctx context.Context, limit int) ([]SalesShipment, error) {
	rows, err := r.db.Query(ctx, shipmentSelect()+` ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SalesShipment
	for rows.Next() {
		item, err := scanShipment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) GetShipment(ctx context.Context, id uuid.UUID) (*SalesShipment, error) {
	shipment, err := scanShipment(r.db.QueryRow(ctx, shipmentSelect()+` WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	lines, err := r.listShipmentLines(ctx, id)
	if err != nil {
		return nil, err
	}
	shipment.Lines = lines
	return shipment, nil
}

func (r *PostgresRepository) MarkShipmentPosted(ctx context.Context, id uuid.UUID, receivableID *uuid.UUID) (*SalesShipment, error) {
	shipment, err := scanShipment(r.db.QueryRow(ctx, shipmentSelect(`
		UPDATE sales_shipments
		SET status = 'posted', receivable_id = $2, updated_at = NOW()
		WHERE id = $1
		RETURNING `), id, receivableID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	lines, err := r.listShipmentLines(ctx, id)
	if err != nil {
		return nil, err
	}
	shipment.Lines = lines
	return shipment, nil
}

func (r *PostgresRepository) CreateReturn(ctx context.Context, input CreateSalesReturnInput) (*SalesReturn, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	ret, err := insertReturn(ctx, tx, input)
	if err != nil {
		return nil, err
	}
	for _, line := range input.Lines {
		created, err := insertReturnLine(ctx, tx, ret.ID, line)
		if err != nil {
			return nil, err
		}
		ret.Subtotal += created.Amount
		ret.TaxAmount += created.TaxAmount
		ret.TotalAmount += created.TotalAmount
		ret.Lines = append(ret.Lines, *created)
	}
	if err := updateReturnTotals(ctx, tx, ret.ID, ret.Subtotal, ret.TaxAmount, ret.TotalAmount); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *PostgresRepository) ListReturns(ctx context.Context, limit int) ([]SalesReturn, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, master_key, return_number, shipment_id, customer_id, customer_name, status, approval_status,
		       organization_id, department_id, workflow_instance_id, currency, subtotal::float8, tax_amount::float8,
		       total_amount::float8, metadata, created_at, updated_at
		FROM sales_returns
		ORDER BY created_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SalesReturn
	for rows.Next() {
		item, err := scanReturn(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func insertQuotation(ctx context.Context, tx pgx.Tx, input CreateSalesQuotationInput) (*SalesQuotation, error) {
	return scanQuotation(tx.QueryRow(ctx, `
		INSERT INTO sales_quotations (quotation_number, customer_id, customer_name, status, approval_status, organization_id, department_id, workflow_instance_id, currency, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, master_key, quotation_number, customer_id, customer_name, status, approval_status,
		          organization_id, department_id, workflow_instance_id, currency, subtotal::float8, tax_amount::float8,
		          total_amount::float8, metadata, created_at, updated_at`,
		input.QuotationNumber, input.CustomerID, input.CustomerName, input.Status, input.ApprovalStatus, input.OrganizationID, input.DepartmentID, input.WorkflowID, input.Currency, jsonMap(input.Metadata)))
}

func insertQuotationLine(ctx context.Context, tx pgx.Tx, quotationID uuid.UUID, input CreateSalesQuotationLineInput) (*SalesQuotationLine, error) {
	var item SalesQuotationLine
	var raw []byte
	err := tx.QueryRow(ctx, `
		INSERT INTO sales_quotation_lines (quotation_id, item_id, warehouse_id, quantity, unit_price, tax_rate, amount, tax_amount, total_amount, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $4::numeric * $5::numeric, ($4::numeric * $5::numeric) * $6::numeric, ($4::numeric * $5::numeric) * (1 + $6::numeric), $7)
		RETURNING id, quotation_id, item_id, warehouse_id, quantity::float8, unit_price::float8, tax_rate::float8, amount::float8, tax_amount::float8, total_amount::float8, metadata`,
		quotationID, input.ItemID, input.WarehouseID, input.Quantity, input.UnitPrice, input.TaxRate, jsonMap(input.Metadata)).
		Scan(&item.ID, &item.QuotationID, &item.ItemID, &item.WarehouseID, &item.Quantity, &item.UnitPrice, &item.TaxRate, &item.Amount, &item.TaxAmount, &item.TotalAmount, &raw)
	item.Metadata = parseJSONMap(raw)
	return &item, err
}

func insertOrder(ctx context.Context, tx pgx.Tx, input CreateSalesOrderInput) (*SalesOrder, error) {
	return scanOrder(tx.QueryRow(ctx, `
		INSERT INTO sales_orders (order_number, quotation_id, customer_id, customer_name, status, approval_status, organization_id, department_id, workflow_instance_id, currency, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, master_key, order_number, quotation_id, customer_id, customer_name, status, approval_status,
		          organization_id, department_id, workflow_instance_id, currency, subtotal::float8, tax_amount::float8,
		          total_amount::float8, metadata, created_at, updated_at`,
		input.OrderNumber, input.QuotationID, input.CustomerID, input.CustomerName, input.Status, input.ApprovalStatus, input.OrganizationID, input.DepartmentID, input.WorkflowID, input.Currency, jsonMap(input.Metadata)))
}

func insertOrderLine(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, input CreateSalesOrderLineInput) (*SalesOrderLine, error) {
	var item SalesOrderLine
	var raw []byte
	err := tx.QueryRow(ctx, `
		INSERT INTO sales_order_lines (order_id, item_id, warehouse_id, quantity, unit_price, tax_rate, amount, tax_amount, total_amount, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $4::numeric * $5::numeric, ($4::numeric * $5::numeric) * $6::numeric, ($4::numeric * $5::numeric) * (1 + $6::numeric), $7)
		RETURNING id, order_id, item_id, warehouse_id, quantity::float8, unit_price::float8, tax_rate::float8, amount::float8, tax_amount::float8, total_amount::float8, metadata`,
		orderID, input.ItemID, input.WarehouseID, input.Quantity, input.UnitPrice, input.TaxRate, jsonMap(input.Metadata)).
		Scan(&item.ID, &item.OrderID, &item.ItemID, &item.WarehouseID, &item.Quantity, &item.UnitPrice, &item.TaxRate, &item.Amount, &item.TaxAmount, &item.TotalAmount, &raw)
	item.Metadata = parseJSONMap(raw)
	return &item, err
}

func insertShipment(ctx context.Context, tx pgx.Tx, input CreateSalesShipmentInput) (*SalesShipment, error) {
	return scanShipment(tx.QueryRow(ctx, `
		INSERT INTO sales_shipments (shipment_number, order_id, customer_id, customer_name, status, approval_status, organization_id, department_id, workflow_instance_id, currency, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, master_key, shipment_number, order_id, customer_id, customer_name, status, approval_status,
		          organization_id, department_id, workflow_instance_id, receivable_id, currency, subtotal::float8, tax_amount::float8,
		          total_amount::float8, metadata, created_at, updated_at`,
		input.ShipmentNumber, input.OrderID, input.CustomerID, input.CustomerName, input.Status, input.ApprovalStatus, input.OrganizationID, input.DepartmentID, input.WorkflowID, input.Currency, jsonMap(input.Metadata)))
}

func insertShipmentLine(ctx context.Context, tx pgx.Tx, shipmentID uuid.UUID, input CreateSalesShipmentLineInput) (*SalesShipmentLine, error) {
	var item SalesShipmentLine
	var raw []byte
	err := tx.QueryRow(ctx, `
		INSERT INTO sales_shipment_lines (shipment_id, order_line_id, item_id, warehouse_id, location_id, quantity, unit_price, tax_rate, amount, tax_amount, total_amount, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $6::numeric * $7::numeric, ($6::numeric * $7::numeric) * $8::numeric, ($6::numeric * $7::numeric) * (1 + $8::numeric), $9)
		RETURNING id, shipment_id, order_line_id, item_id, warehouse_id, location_id, quantity::float8, unit_price::float8, tax_rate::float8, amount::float8, tax_amount::float8, total_amount::float8, metadata`,
		shipmentID, input.OrderLineID, input.ItemID, input.WarehouseID, input.LocationID, input.Quantity, input.UnitPrice, input.TaxRate, jsonMap(input.Metadata)).
		Scan(&item.ID, &item.ShipmentID, &item.OrderLineID, &item.ItemID, &item.WarehouseID, &item.LocationID, &item.Quantity, &item.UnitPrice, &item.TaxRate, &item.Amount, &item.TaxAmount, &item.TotalAmount, &raw)
	item.Metadata = parseJSONMap(raw)
	return &item, err
}

func insertReturn(ctx context.Context, tx pgx.Tx, input CreateSalesReturnInput) (*SalesReturn, error) {
	return scanReturn(tx.QueryRow(ctx, `
		INSERT INTO sales_returns (return_number, shipment_id, customer_id, customer_name, status, approval_status, organization_id, department_id, workflow_instance_id, currency, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, master_key, return_number, shipment_id, customer_id, customer_name, status, approval_status,
		          organization_id, department_id, workflow_instance_id, currency, subtotal::float8, tax_amount::float8,
		          total_amount::float8, metadata, created_at, updated_at`,
		input.ReturnNumber, input.ShipmentID, input.CustomerID, input.CustomerName, input.Status, input.ApprovalStatus, input.OrganizationID, input.DepartmentID, input.WorkflowID, input.Currency, jsonMap(input.Metadata)))
}

func insertReturnLine(ctx context.Context, tx pgx.Tx, returnID uuid.UUID, input CreateSalesReturnLineInput) (*SalesReturnLine, error) {
	var item SalesReturnLine
	var raw []byte
	err := tx.QueryRow(ctx, `
		INSERT INTO sales_return_lines (return_id, item_id, warehouse_id, location_id, quantity, unit_price, amount, tax_amount, total_amount, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $5::numeric * $6::numeric, ($5::numeric * $6::numeric) * $7::numeric, ($5::numeric * $6::numeric) * (1 + $7::numeric), $8)
		RETURNING id, return_id, item_id, warehouse_id, location_id, quantity::float8, unit_price::float8, amount::float8, tax_amount::float8, total_amount::float8, metadata`,
		returnID, input.ItemID, input.WarehouseID, input.LocationID, input.Quantity, input.UnitPrice, input.TaxRate, jsonMap(input.Metadata)).
		Scan(&item.ID, &item.ReturnID, &item.ItemID, &item.WarehouseID, &item.LocationID, &item.Quantity, &item.UnitPrice, &item.Amount, &item.TaxAmount, &item.TotalAmount, &raw)
	item.Metadata = parseJSONMap(raw)
	return &item, err
}

func updateQuotationTotals(ctx context.Context, tx pgx.Tx, id uuid.UUID, subtotal float64, tax float64, total float64) error {
	_, err := tx.Exec(ctx, `UPDATE sales_quotations SET subtotal = $2, tax_amount = $3, total_amount = $4, updated_at = NOW() WHERE id = $1`, id, subtotal, tax, total)
	return err
}

func updateOrderTotals(ctx context.Context, tx pgx.Tx, id uuid.UUID, subtotal float64, tax float64, total float64) error {
	_, err := tx.Exec(ctx, `UPDATE sales_orders SET subtotal = $2, tax_amount = $3, total_amount = $4, updated_at = NOW() WHERE id = $1`, id, subtotal, tax, total)
	return err
}

func updateShipmentTotals(ctx context.Context, tx pgx.Tx, id uuid.UUID, subtotal float64, tax float64, total float64) error {
	_, err := tx.Exec(ctx, `UPDATE sales_shipments SET subtotal = $2, tax_amount = $3, total_amount = $4, updated_at = NOW() WHERE id = $1`, id, subtotal, tax, total)
	return err
}

func updateReturnTotals(ctx context.Context, tx pgx.Tx, id uuid.UUID, subtotal float64, tax float64, total float64) error {
	_, err := tx.Exec(ctx, `UPDATE sales_returns SET subtotal = $2, tax_amount = $3, total_amount = $4, updated_at = NOW() WHERE id = $1`, id, subtotal, tax, total)
	return err
}

func (r *PostgresRepository) listShipmentLines(ctx context.Context, shipmentID uuid.UUID) ([]SalesShipmentLine, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, shipment_id, order_line_id, item_id, warehouse_id, location_id, quantity::float8, unit_price::float8,
		       tax_rate::float8, amount::float8, tax_amount::float8, total_amount::float8, metadata
		FROM sales_shipment_lines
		WHERE shipment_id = $1
		ORDER BY created_at ASC`, shipmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SalesShipmentLine
	for rows.Next() {
		var item SalesShipmentLine
		var raw []byte
		if err := rows.Scan(&item.ID, &item.ShipmentID, &item.OrderLineID, &item.ItemID, &item.WarehouseID, &item.LocationID, &item.Quantity, &item.UnitPrice, &item.TaxRate, &item.Amount, &item.TaxAmount, &item.TotalAmount, &raw); err != nil {
			return nil, err
		}
		item.Metadata = parseJSONMap(raw)
		out = append(out, item)
	}
	return out, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanQuotation(row rowScanner) (*SalesQuotation, error) {
	var item SalesQuotation
	var raw []byte
	err := row.Scan(&item.ID, &item.MasterKey, &item.QuotationNumber, &item.CustomerID, &item.CustomerName, &item.Status, &item.ApprovalStatus, &item.OrganizationID, &item.DepartmentID, &item.WorkflowID, &item.Currency, &item.Subtotal, &item.TaxAmount, &item.TotalAmount, &raw, &item.CreatedAt, &item.UpdatedAt)
	item.Metadata = parseJSONMap(raw)
	return &item, err
}

func scanOrder(row rowScanner) (*SalesOrder, error) {
	var item SalesOrder
	var raw []byte
	err := row.Scan(&item.ID, &item.MasterKey, &item.OrderNumber, &item.QuotationID, &item.CustomerID, &item.CustomerName, &item.Status, &item.ApprovalStatus, &item.OrganizationID, &item.DepartmentID, &item.WorkflowID, &item.Currency, &item.Subtotal, &item.TaxAmount, &item.TotalAmount, &raw, &item.CreatedAt, &item.UpdatedAt)
	item.Metadata = parseJSONMap(raw)
	return &item, err
}

func scanShipment(row rowScanner) (*SalesShipment, error) {
	var item SalesShipment
	var raw []byte
	err := row.Scan(&item.ID, &item.MasterKey, &item.ShipmentNumber, &item.OrderID, &item.CustomerID, &item.CustomerName, &item.Status, &item.ApprovalStatus, &item.OrganizationID, &item.DepartmentID, &item.WorkflowID, &item.ReceivableID, &item.Currency, &item.Subtotal, &item.TaxAmount, &item.TotalAmount, &raw, &item.CreatedAt, &item.UpdatedAt)
	item.Metadata = parseJSONMap(raw)
	return &item, err
}

func scanReturn(row rowScanner) (*SalesReturn, error) {
	var item SalesReturn
	var raw []byte
	err := row.Scan(&item.ID, &item.MasterKey, &item.ReturnNumber, &item.ShipmentID, &item.CustomerID, &item.CustomerName, &item.Status, &item.ApprovalStatus, &item.OrganizationID, &item.DepartmentID, &item.WorkflowID, &item.Currency, &item.Subtotal, &item.TaxAmount, &item.TotalAmount, &raw, &item.CreatedAt, &item.UpdatedAt)
	item.Metadata = parseJSONMap(raw)
	return &item, err
}

func orderSelect(prefix ...string) string {
	cols := `id, master_key, order_number, quotation_id, customer_id, customer_name, status, approval_status,
		organization_id, department_id, workflow_instance_id, currency, subtotal::float8, tax_amount::float8,
		total_amount::float8, metadata, created_at, updated_at`
	if len(prefix) > 0 {
		return prefix[0] + cols
	}
	return `SELECT ` + cols + ` FROM sales_orders`
}

func shipmentSelect(prefix ...string) string {
	cols := `id, master_key, shipment_number, order_id, customer_id, customer_name, status, approval_status,
		organization_id, department_id, workflow_instance_id, receivable_id, currency,
		subtotal::float8, tax_amount::float8, total_amount::float8, metadata, created_at, updated_at`
	if len(prefix) > 0 {
		return prefix[0] + cols
	}
	return `SELECT ` + cols + ` FROM sales_shipments`
}

func jsonMap(value map[string]any) []byte {
	if value == nil {
		value = map[string]any{}
	}
	raw, _ := json.Marshal(value)
	return raw
}

func parseJSONMap(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return map[string]any{}
	}
	return out
}
