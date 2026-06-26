package procurement

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

func (r *PostgresRepository) CreateRequisition(ctx context.Context, input CreatePurchaseRequisitionInput) (*PurchaseRequisition, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	req, err := insertRequisition(ctx, tx, input)
	if err != nil {
		return nil, err
	}
	for _, line := range input.Lines {
		created, err := insertRequisitionLine(ctx, tx, req.ID, line)
		if err != nil {
			return nil, err
		}
		req.TotalAmount += created.Amount
		req.Lines = append(req.Lines, *created)
	}
	if err := updateRequisitionTotal(ctx, tx, req.ID, req.TotalAmount); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return req, nil
}

func (r *PostgresRepository) ListRequisitions(ctx context.Context, limit int) ([]PurchaseRequisition, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, master_key, title, supplier_id, supplier_name, status, approval_status, organization_id, department_id,
		       workflow_instance_id, currency, total_amount::float8, metadata, created_at, updated_at
		FROM purchase_requisitions
		ORDER BY created_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PurchaseRequisition
	for rows.Next() {
		item, err := scanRequisition(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) GetRequisition(ctx context.Context, id uuid.UUID) (*PurchaseRequisition, error) {
	item, err := scanRequisition(r.db.QueryRow(ctx, `
		SELECT id, master_key, title, supplier_id, supplier_name, status, approval_status, organization_id, department_id,
		       workflow_instance_id, currency, total_amount::float8, metadata, created_at, updated_at
		FROM purchase_requisitions
		WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return item, err
}

func (r *PostgresRepository) UpdateRequisitionStatus(ctx context.Context, id uuid.UUID, status string) (*PurchaseRequisition, error) {
	item, err := scanRequisition(r.db.QueryRow(ctx, `
		UPDATE purchase_requisitions
		SET status = $2, approval_status = CASE WHEN $2 = 'approved' THEN 'approved' ELSE approval_status END, updated_at = NOW()
		WHERE id = $1
		RETURNING id, master_key, title, supplier_id, supplier_name, status, approval_status, organization_id, department_id,
		          workflow_instance_id, currency, total_amount::float8, metadata, created_at, updated_at`, id, status))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return item, err
}

func (r *PostgresRepository) CreateOrder(ctx context.Context, input CreatePurchaseOrderInput) (*PurchaseOrder, error) {
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

func (r *PostgresRepository) ListOrders(ctx context.Context, limit int) ([]PurchaseOrder, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, master_key, order_number, requisition_id, supplier_id, supplier_name, status, approval_status,
		       organization_id, department_id, workflow_instance_id, currency, subtotal::float8, tax_amount::float8,
		       total_amount::float8, metadata, created_at, updated_at
		FROM purchase_orders
		ORDER BY created_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PurchaseOrder
	for rows.Next() {
		item, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) GetOrder(ctx context.Context, id uuid.UUID) (*PurchaseOrder, error) {
	item, err := scanOrder(r.db.QueryRow(ctx, `
		SELECT id, master_key, order_number, requisition_id, supplier_id, supplier_name, status, approval_status,
		       organization_id, department_id, workflow_instance_id, currency, subtotal::float8, tax_amount::float8,
		       total_amount::float8, metadata, created_at, updated_at
		FROM purchase_orders
		WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return item, err
}

func (r *PostgresRepository) UpdateOrderStatus(ctx context.Context, id uuid.UUID, status string) (*PurchaseOrder, error) {
	item, err := scanOrder(r.db.QueryRow(ctx, `
		UPDATE purchase_orders
		SET status = $2, approval_status = CASE WHEN $2 = 'approved' THEN 'approved' ELSE approval_status END, updated_at = NOW()
		WHERE id = $1
		RETURNING id, master_key, order_number, requisition_id, supplier_id, supplier_name, status, approval_status,
		          organization_id, department_id, workflow_instance_id, currency, subtotal::float8, tax_amount::float8,
		          total_amount::float8, metadata, created_at, updated_at`, id, status))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return item, err
}

func (r *PostgresRepository) CreateReceipt(ctx context.Context, input CreatePurchaseReceiptInput) (*PurchaseReceipt, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	receipt, err := insertReceipt(ctx, tx, input)
	if err != nil {
		return nil, err
	}
	for _, line := range input.Lines {
		created, err := insertReceiptLine(ctx, tx, receipt.ID, line)
		if err != nil {
			return nil, err
		}
		receipt.Subtotal += created.Amount
		receipt.TaxAmount += created.TaxAmount
		receipt.TotalAmount += created.TotalAmount
		receipt.Lines = append(receipt.Lines, *created)
	}
	if err := updateReceiptTotals(ctx, tx, receipt.ID, receipt.Subtotal, receipt.TaxAmount, receipt.TotalAmount); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return receipt, nil
}

func (r *PostgresRepository) ListReceipts(ctx context.Context, limit int) ([]PurchaseReceipt, error) {
	rows, err := r.db.Query(ctx, receiptSelect()+` ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PurchaseReceipt
	for rows.Next() {
		item, err := scanReceipt(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) GetReceipt(ctx context.Context, id uuid.UUID) (*PurchaseReceipt, error) {
	receipt, err := scanReceipt(r.db.QueryRow(ctx, receiptSelect()+` WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	lines, err := r.listReceiptLines(ctx, id)
	if err != nil {
		return nil, err
	}
	receipt.Lines = lines
	return receipt, nil
}

func (r *PostgresRepository) MarkReceiptPosted(ctx context.Context, id uuid.UUID, payableID *uuid.UUID) (*PurchaseReceipt, error) {
	receipt, err := scanReceipt(r.db.QueryRow(ctx, receiptSelect(`
		UPDATE purchase_receipts
		SET status = 'posted', payable_id = $2, updated_at = NOW()
		WHERE id = $1
		RETURNING `), id, payableID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	lines, err := r.listReceiptLines(ctx, id)
	if err != nil {
		return nil, err
	}
	receipt.Lines = lines
	return receipt, nil
}

func (r *PostgresRepository) CreateReturn(ctx context.Context, input CreatePurchaseReturnInput) (*PurchaseReturn, error) {
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

func (r *PostgresRepository) ListReturns(ctx context.Context, limit int) ([]PurchaseReturn, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, master_key, return_number, receipt_id, supplier_id, supplier_name, status, approval_status,
		       organization_id, department_id, workflow_instance_id, currency, subtotal::float8, tax_amount::float8,
		       total_amount::float8, metadata, created_at, updated_at
		FROM purchase_returns
		ORDER BY created_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PurchaseReturn
	for rows.Next() {
		item, err := scanReturn(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

type pgxTx interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconnTag, error)
}

type pgconnTag interface{}

func insertRequisition(ctx context.Context, tx pgx.Tx, input CreatePurchaseRequisitionInput) (*PurchaseRequisition, error) {
	return scanRequisition(tx.QueryRow(ctx, `
		INSERT INTO purchase_requisitions (title, supplier_id, supplier_name, status, approval_status, organization_id, department_id, workflow_instance_id, currency, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, master_key, title, supplier_id, supplier_name, status, approval_status, organization_id, department_id,
		          workflow_instance_id, currency, total_amount::float8, metadata, created_at, updated_at`,
		input.Title, input.SupplierID, input.SupplierName, input.Status, input.ApprovalStatus, input.OrganizationID, input.DepartmentID, input.WorkflowID, input.Currency, jsonMap(input.Metadata)))
}

func insertRequisitionLine(ctx context.Context, tx pgx.Tx, requisitionID uuid.UUID, input CreatePurchaseRequisitionLineInput) (*PurchaseRequisitionLine, error) {
	var item PurchaseRequisitionLine
	var raw []byte
	err := tx.QueryRow(ctx, `
		INSERT INTO purchase_requisition_lines (requisition_id, item_id, warehouse_id, quantity, unit_cost, amount, metadata)
		VALUES ($1, $2, $3, $4, $5, $4::numeric * $5::numeric, $6)
		RETURNING id, requisition_id, item_id, warehouse_id, quantity::float8, unit_cost::float8, amount::float8, metadata`,
		requisitionID, input.ItemID, input.WarehouseID, input.Quantity, input.UnitCost, jsonMap(input.Metadata)).
		Scan(&item.ID, &item.RequisitionID, &item.ItemID, &item.WarehouseID, &item.Quantity, &item.UnitCost, &item.Amount, &raw)
	item.Metadata = parseJSONMap(raw)
	return &item, err
}

func insertOrder(ctx context.Context, tx pgx.Tx, input CreatePurchaseOrderInput) (*PurchaseOrder, error) {
	return scanOrder(tx.QueryRow(ctx, `
		INSERT INTO purchase_orders (order_number, requisition_id, supplier_id, supplier_name, status, approval_status, organization_id, department_id, workflow_instance_id, currency, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, master_key, order_number, requisition_id, supplier_id, supplier_name, status, approval_status,
		          organization_id, department_id, workflow_instance_id, currency, subtotal::float8, tax_amount::float8,
		          total_amount::float8, metadata, created_at, updated_at`,
		input.OrderNumber, input.RequisitionID, input.SupplierID, input.SupplierName, input.Status, input.ApprovalStatus, input.OrganizationID, input.DepartmentID, input.WorkflowID, input.Currency, jsonMap(input.Metadata)))
}

func insertOrderLine(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, input CreatePurchaseOrderLineInput) (*PurchaseOrderLine, error) {
	var item PurchaseOrderLine
	var raw []byte
	err := tx.QueryRow(ctx, `
		INSERT INTO purchase_order_lines (order_id, item_id, warehouse_id, quantity, unit_cost, tax_rate, amount, tax_amount, total_amount, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $4::numeric * $5::numeric, ($4::numeric * $5::numeric) * $6::numeric, ($4::numeric * $5::numeric) * (1 + $6::numeric), $7)
		RETURNING id, order_id, item_id, warehouse_id, quantity::float8, unit_cost::float8, tax_rate::float8, amount::float8, tax_amount::float8, total_amount::float8, metadata`,
		orderID, input.ItemID, input.WarehouseID, input.Quantity, input.UnitCost, input.TaxRate, jsonMap(input.Metadata)).
		Scan(&item.ID, &item.OrderID, &item.ItemID, &item.WarehouseID, &item.Quantity, &item.UnitCost, &item.TaxRate, &item.Amount, &item.TaxAmount, &item.TotalAmount, &raw)
	item.Metadata = parseJSONMap(raw)
	return &item, err
}

func insertReceipt(ctx context.Context, tx pgx.Tx, input CreatePurchaseReceiptInput) (*PurchaseReceipt, error) {
	return scanReceipt(tx.QueryRow(ctx, `
		INSERT INTO purchase_receipts (receipt_number, order_id, supplier_id, supplier_name, status, approval_status, organization_id, department_id, workflow_instance_id, warehouse_id, currency, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, master_key, receipt_number, order_id, supplier_id, supplier_name, status, approval_status,
		          organization_id, department_id, workflow_instance_id, warehouse_id, payable_id, currency,
		          subtotal::float8, tax_amount::float8, total_amount::float8, metadata, created_at, updated_at`,
		input.ReceiptNumber, input.OrderID, input.SupplierID, input.SupplierName, input.Status, input.ApprovalStatus, input.OrganizationID, input.DepartmentID, input.WorkflowID, input.WarehouseID, input.Currency, jsonMap(input.Metadata)))
}

func insertReceiptLine(ctx context.Context, tx pgx.Tx, receiptID uuid.UUID, input CreatePurchaseReceiptLineInput) (*PurchaseReceiptLine, error) {
	var item PurchaseReceiptLine
	var raw []byte
	err := tx.QueryRow(ctx, `
		INSERT INTO purchase_receipt_lines (receipt_id, order_line_id, item_id, warehouse_id, location_id, quantity, unit_cost, tax_rate, amount, tax_amount, total_amount, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $6::numeric * $7::numeric, ($6::numeric * $7::numeric) * $8::numeric, ($6::numeric * $7::numeric) * (1 + $8::numeric), $9)
		RETURNING id, receipt_id, order_line_id, item_id, warehouse_id, location_id, quantity::float8, unit_cost::float8, tax_rate::float8, amount::float8, tax_amount::float8, total_amount::float8, metadata`,
		receiptID, input.OrderLineID, input.ItemID, input.WarehouseID, input.LocationID, input.Quantity, input.UnitCost, input.TaxRate, jsonMap(input.Metadata)).
		Scan(&item.ID, &item.ReceiptID, &item.OrderLineID, &item.ItemID, &item.WarehouseID, &item.LocationID, &item.Quantity, &item.UnitCost, &item.TaxRate, &item.Amount, &item.TaxAmount, &item.TotalAmount, &raw)
	item.Metadata = parseJSONMap(raw)
	return &item, err
}

func insertReturn(ctx context.Context, tx pgx.Tx, input CreatePurchaseReturnInput) (*PurchaseReturn, error) {
	return scanReturn(tx.QueryRow(ctx, `
		INSERT INTO purchase_returns (return_number, receipt_id, supplier_id, supplier_name, status, approval_status, organization_id, department_id, workflow_instance_id, currency, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, master_key, return_number, receipt_id, supplier_id, supplier_name, status, approval_status,
		          organization_id, department_id, workflow_instance_id, currency, subtotal::float8, tax_amount::float8,
		          total_amount::float8, metadata, created_at, updated_at`,
		input.ReturnNumber, input.ReceiptID, input.SupplierID, input.SupplierName, input.Status, input.ApprovalStatus, input.OrganizationID, input.DepartmentID, input.WorkflowID, input.Currency, jsonMap(input.Metadata)))
}

func insertReturnLine(ctx context.Context, tx pgx.Tx, returnID uuid.UUID, input CreatePurchaseReturnLineInput) (*PurchaseReturnLine, error) {
	var item PurchaseReturnLine
	var raw []byte
	err := tx.QueryRow(ctx, `
		INSERT INTO purchase_return_lines (return_id, item_id, warehouse_id, location_id, quantity, unit_cost, amount, tax_amount, total_amount, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $5::numeric * $6::numeric, ($5::numeric * $6::numeric) * $7::numeric, ($5::numeric * $6::numeric) * (1 + $7::numeric), $8)
		RETURNING id, return_id, item_id, warehouse_id, location_id, quantity::float8, unit_cost::float8, amount::float8, tax_amount::float8, total_amount::float8, metadata`,
		returnID, input.ItemID, input.WarehouseID, input.LocationID, input.Quantity, input.UnitCost, input.TaxRate, jsonMap(input.Metadata)).
		Scan(&item.ID, &item.ReturnID, &item.ItemID, &item.WarehouseID, &item.LocationID, &item.Quantity, &item.UnitCost, &item.Amount, &item.TaxAmount, &item.TotalAmount, &raw)
	item.Metadata = parseJSONMap(raw)
	return &item, err
}

func updateRequisitionTotal(ctx context.Context, tx pgx.Tx, id uuid.UUID, total float64) error {
	_, err := tx.Exec(ctx, `UPDATE purchase_requisitions SET total_amount = $2, updated_at = NOW() WHERE id = $1`, id, total)
	return err
}

func updateOrderTotals(ctx context.Context, tx pgx.Tx, id uuid.UUID, subtotal float64, tax float64, total float64) error {
	_, err := tx.Exec(ctx, `UPDATE purchase_orders SET subtotal = $2, tax_amount = $3, total_amount = $4, updated_at = NOW() WHERE id = $1`, id, subtotal, tax, total)
	return err
}

func updateReceiptTotals(ctx context.Context, tx pgx.Tx, id uuid.UUID, subtotal float64, tax float64, total float64) error {
	_, err := tx.Exec(ctx, `UPDATE purchase_receipts SET subtotal = $2, tax_amount = $3, total_amount = $4, updated_at = NOW() WHERE id = $1`, id, subtotal, tax, total)
	return err
}

func updateReturnTotals(ctx context.Context, tx pgx.Tx, id uuid.UUID, subtotal float64, tax float64, total float64) error {
	_, err := tx.Exec(ctx, `UPDATE purchase_returns SET subtotal = $2, tax_amount = $3, total_amount = $4, updated_at = NOW() WHERE id = $1`, id, subtotal, tax, total)
	return err
}

func (r *PostgresRepository) listReceiptLines(ctx context.Context, receiptID uuid.UUID) ([]PurchaseReceiptLine, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, receipt_id, order_line_id, item_id, warehouse_id, location_id, quantity::float8, unit_cost::float8,
		       tax_rate::float8, amount::float8, tax_amount::float8, total_amount::float8, metadata
		FROM purchase_receipt_lines
		WHERE receipt_id = $1
		ORDER BY created_at ASC`, receiptID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PurchaseReceiptLine
	for rows.Next() {
		var item PurchaseReceiptLine
		var raw []byte
		if err := rows.Scan(&item.ID, &item.ReceiptID, &item.OrderLineID, &item.ItemID, &item.WarehouseID, &item.LocationID, &item.Quantity, &item.UnitCost, &item.TaxRate, &item.Amount, &item.TaxAmount, &item.TotalAmount, &raw); err != nil {
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

func scanRequisition(row rowScanner) (*PurchaseRequisition, error) {
	var item PurchaseRequisition
	var raw []byte
	err := row.Scan(&item.ID, &item.MasterKey, &item.Title, &item.SupplierID, &item.SupplierName, &item.Status, &item.ApprovalStatus, &item.OrganizationID, &item.DepartmentID, &item.WorkflowID, &item.Currency, &item.TotalAmount, &raw, &item.CreatedAt, &item.UpdatedAt)
	item.Metadata = parseJSONMap(raw)
	return &item, err
}

func scanOrder(row rowScanner) (*PurchaseOrder, error) {
	var item PurchaseOrder
	var raw []byte
	err := row.Scan(&item.ID, &item.MasterKey, &item.OrderNumber, &item.RequisitionID, &item.SupplierID, &item.SupplierName, &item.Status, &item.ApprovalStatus, &item.OrganizationID, &item.DepartmentID, &item.WorkflowID, &item.Currency, &item.Subtotal, &item.TaxAmount, &item.TotalAmount, &raw, &item.CreatedAt, &item.UpdatedAt)
	item.Metadata = parseJSONMap(raw)
	return &item, err
}

func scanReceipt(row rowScanner) (*PurchaseReceipt, error) {
	var item PurchaseReceipt
	var raw []byte
	err := row.Scan(&item.ID, &item.MasterKey, &item.ReceiptNumber, &item.OrderID, &item.SupplierID, &item.SupplierName, &item.Status, &item.ApprovalStatus, &item.OrganizationID, &item.DepartmentID, &item.WorkflowID, &item.WarehouseID, &item.PayableID, &item.Currency, &item.Subtotal, &item.TaxAmount, &item.TotalAmount, &raw, &item.CreatedAt, &item.UpdatedAt)
	item.Metadata = parseJSONMap(raw)
	return &item, err
}

func scanReturn(row rowScanner) (*PurchaseReturn, error) {
	var item PurchaseReturn
	var raw []byte
	err := row.Scan(&item.ID, &item.MasterKey, &item.ReturnNumber, &item.ReceiptID, &item.SupplierID, &item.SupplierName, &item.Status, &item.ApprovalStatus, &item.OrganizationID, &item.DepartmentID, &item.WorkflowID, &item.Currency, &item.Subtotal, &item.TaxAmount, &item.TotalAmount, &raw, &item.CreatedAt, &item.UpdatedAt)
	item.Metadata = parseJSONMap(raw)
	return &item, err
}

func receiptSelect(prefix ...string) string {
	cols := `id, master_key, receipt_number, order_id, supplier_id, supplier_name, status, approval_status,
		organization_id, department_id, workflow_instance_id, warehouse_id, payable_id, currency,
		subtotal::float8, tax_amount::float8, total_amount::float8, metadata, created_at, updated_at`
	if len(prefix) > 0 {
		return prefix[0] + cols
	}
	return `SELECT ` + cols + ` FROM purchase_receipts`
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
