package inventory

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) CreateBusinessPartner(ctx context.Context, input CreateBusinessPartnerInput) (*BusinessPartner, error) {
	row := r.db.QueryRow(ctx, `
		INSERT INTO business_partners (partner_code, partner_type, name, email, phone, status, organization_id, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, master_key, partner_code, partner_type, name, email, phone, status, organization_id, metadata, created_at, updated_at`,
		input.PartnerCode, input.PartnerType, input.Name, input.Email, input.Phone, input.Status, input.OrganizationID, jsonMap(input.Metadata))
	return scanBusinessPartner(row)
}

func (r *PostgresRepository) ListBusinessPartners(ctx context.Context, limit int) ([]BusinessPartner, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, master_key, partner_code, partner_type, name, email, phone, status, organization_id, metadata, created_at, updated_at
		FROM business_partners
		ORDER BY created_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BusinessPartner
	for rows.Next() {
		item, err := scanBusinessPartner(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) CreateItem(ctx context.Context, input CreateItemInput) (*Item, error) {
	row := r.db.QueryRow(ctx, `
		INSERT INTO items (item_code, name, item_type, base_uom, status, organization_id, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, master_key, item_code, name, item_type, base_uom, status, organization_id, metadata, created_at, updated_at`,
		input.ItemCode, input.Name, input.ItemType, input.BaseUOM, input.Status, input.OrganizationID, jsonMap(input.Metadata))
	return scanItem(row)
}

func (r *PostgresRepository) ListItems(ctx context.Context, limit int) ([]Item, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, master_key, item_code, name, item_type, base_uom, status, organization_id, metadata, created_at, updated_at
		FROM items
		ORDER BY created_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Item
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) CreateWarehouse(ctx context.Context, input CreateWarehouseInput) (*Warehouse, error) {
	row := r.db.QueryRow(ctx, `
		INSERT INTO warehouses (warehouse_code, name, status, organization_id, department_id, metadata)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, master_key, warehouse_code, name, status, organization_id, department_id, metadata, created_at, updated_at`,
		input.WarehouseCode, input.Name, input.Status, input.OrganizationID, input.DepartmentID, jsonMap(input.Metadata))
	return scanWarehouse(row)
}

func (r *PostgresRepository) ListWarehouses(ctx context.Context, limit int) ([]Warehouse, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, master_key, warehouse_code, name, status, organization_id, department_id, metadata, created_at, updated_at
		FROM warehouses
		ORDER BY created_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Warehouse
	for rows.Next() {
		item, err := scanWarehouse(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) CreateWarehouseLocation(ctx context.Context, input CreateWarehouseLocationInput) (*WarehouseLocation, error) {
	row := r.db.QueryRow(ctx, `
		INSERT INTO warehouse_locations (warehouse_id, location_code, name, status, metadata)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, master_key, warehouse_id, location_code, name, status, metadata, created_at, updated_at`,
		input.WarehouseID, input.LocationCode, input.Name, input.Status, jsonMap(input.Metadata))
	return scanWarehouseLocation(row)
}

func (r *PostgresRepository) GetBalance(ctx context.Context, itemID uuid.UUID, warehouseID uuid.UUID, locationID *uuid.UUID) (*InventoryBalance, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, master_key, item_id, warehouse_id, location_id, quantity::float8, reserved_qty::float8,
		       average_cost::float8, value_amount::float8, currency, organization_id, metadata, updated_at
		FROM inventory_balances
		WHERE item_id = $1
		  AND warehouse_id = $2
		  AND COALESCE(location_id, '00000000-0000-0000-0000-000000000000'::uuid) = COALESCE($3::uuid, '00000000-0000-0000-0000-000000000000'::uuid)`,
		itemID, warehouseID, locationID)
	balance, err := scanBalance(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return balance, err
}

func (r *PostgresRepository) UpsertBalance(ctx context.Context, balance InventoryBalance) (*InventoryBalance, error) {
	row := r.db.QueryRow(ctx, `
		INSERT INTO inventory_balances (item_id, warehouse_id, location_id, quantity, reserved_qty, average_cost, value_amount, currency, organization_id, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (item_id, warehouse_id, COALESCE(location_id, '00000000-0000-0000-0000-000000000000'::uuid))
		DO UPDATE SET
			quantity = EXCLUDED.quantity,
			reserved_qty = EXCLUDED.reserved_qty,
			average_cost = EXCLUDED.average_cost,
			value_amount = EXCLUDED.value_amount,
			currency = EXCLUDED.currency,
			organization_id = COALESCE(EXCLUDED.organization_id, inventory_balances.organization_id),
			metadata = inventory_balances.metadata || EXCLUDED.metadata,
			updated_at = NOW()
		RETURNING id, master_key, item_id, warehouse_id, location_id, quantity::float8, reserved_qty::float8,
		          average_cost::float8, value_amount::float8, currency, organization_id, metadata, updated_at`,
		balance.ItemID, balance.WarehouseID, balance.LocationID, balance.Quantity, balance.ReservedQty,
		balance.AverageCost, balance.ValueAmount, balance.Currency, balance.OrganizationID, jsonMap(balance.Metadata))
	return scanBalance(row)
}

func (r *PostgresRepository) CreateMovement(ctx context.Context, input CreateInventoryMovementInput, balance InventoryBalance) (*InventoryMovement, error) {
	row := r.db.QueryRow(ctx, `
		INSERT INTO inventory_movements (
			movement_type, source_type, source_id, item_id, warehouse_id, location_id,
			quantity, unit_cost, amount, currency, balance_after, organization_id, department_id, occurred_at, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $7::numeric * $8::numeric, $9, $10, $11, $12, COALESCE($13, NOW()), $14)
		RETURNING id, master_key, movement_type, source_type, source_id, item_id, warehouse_id, location_id,
		          quantity::float8, unit_cost::float8, amount::float8, currency, balance_after::float8,
		          organization_id, department_id, occurred_at, metadata, created_at`,
		input.MovementType, input.SourceType, input.SourceID, input.ItemID, input.WarehouseID, input.LocationID,
		input.Quantity, input.UnitCost, input.Currency, balance.Quantity, input.OrganizationID, input.DepartmentID,
		input.OccurredAt, jsonMap(input.Metadata))
	return scanMovement(row)
}

func (r *PostgresRepository) PostMovementAtomic(ctx context.Context, input CreateInventoryMovementInput, direction int) (*InventoryMovement, error) {
	if lineKey, lineID, ok := movementLineIdentity(input); ok && input.SourceID != nil {
		movement, err := r.FindMovementBySourceLine(ctx, input.SourceType, *input.SourceID, lineKey, lineID)
		if err == nil {
			return movement, nil
		}
		if err != nil && !errors.Is(err, ErrNotFound) {
			return nil, err
		}
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var balance *InventoryBalance
	row := tx.QueryRow(ctx, `
		SELECT id, master_key, item_id, warehouse_id, location_id, quantity::float8, reserved_qty::float8,
		       average_cost::float8, value_amount::float8, currency, organization_id, metadata, updated_at
		FROM inventory_balances
		WHERE item_id = $1
		  AND warehouse_id = $2
		  AND COALESCE(location_id, '00000000-0000-0000-0000-000000000000'::uuid) = COALESCE($3::uuid, '00000000-0000-0000-0000-000000000000'::uuid)
		FOR UPDATE`, input.ItemID, input.WarehouseID, input.LocationID)
	balance, err = scanBalance(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) && direction > 0 {
			balance = &InventoryBalance{
				ItemID:         input.ItemID,
				WarehouseID:    input.WarehouseID,
				LocationID:     input.LocationID,
				Currency:       input.Currency,
				OrganizationID: input.OrganizationID,
				Metadata:       map[string]any{},
			}
		} else if errors.Is(err, pgx.ErrNoRows) {
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

	updated, err := scanBalanceRow(tx.QueryRow(ctx, `
		INSERT INTO inventory_balances (item_id, warehouse_id, location_id, quantity, reserved_qty, average_cost, value_amount, currency, organization_id, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (item_id, warehouse_id, COALESCE(location_id, '00000000-0000-0000-0000-000000000000'::uuid))
		DO UPDATE SET
			quantity = EXCLUDED.quantity,
			reserved_qty = EXCLUDED.reserved_qty,
			average_cost = EXCLUDED.average_cost,
			value_amount = EXCLUDED.value_amount,
			currency = EXCLUDED.currency,
			organization_id = COALESCE(EXCLUDED.organization_id, inventory_balances.organization_id),
			metadata = inventory_balances.metadata || EXCLUDED.metadata,
			updated_at = NOW()
		RETURNING id, master_key, item_id, warehouse_id, location_id, quantity::float8, reserved_qty::float8,
		          average_cost::float8, value_amount::float8, currency, organization_id, metadata, updated_at`,
		next.ItemID, next.WarehouseID, next.LocationID, next.Quantity, next.ReservedQty,
		next.AverageCost, next.ValueAmount, next.Currency, next.OrganizationID, jsonMap(next.Metadata)))
	if err != nil {
		return nil, err
	}
	input.Currency = updated.Currency
	if input.UnitCost <= 0 {
		input.UnitCost = updated.AverageCost
	}

	movement, err := scanMovementRow(tx.QueryRow(ctx, `
		INSERT INTO inventory_movements (
			movement_type, source_type, source_id, item_id, warehouse_id, location_id,
			quantity, unit_cost, amount, currency, balance_after, organization_id, department_id, occurred_at, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $7::numeric * $8::numeric, $9, $10, $11, $12, COALESCE($13, NOW()), $14)
		RETURNING id, master_key, movement_type, source_type, source_id, item_id, warehouse_id, location_id,
		          quantity::float8, unit_cost::float8, amount::float8, currency, balance_after::float8,
		          organization_id, department_id, occurred_at, metadata, created_at`,
		input.MovementType, input.SourceType, input.SourceID, input.ItemID, input.WarehouseID, input.LocationID,
		input.Quantity, input.UnitCost, input.Currency, updated.Quantity, input.OrganizationID, input.DepartmentID,
		input.OccurredAt, jsonMap(input.Metadata)))
	if err != nil {
		if isUniqueViolation(err) {
			tx.Rollback(ctx)
			if lineKey, lineID, ok := movementLineIdentity(input); ok && input.SourceID != nil {
				return r.FindMovementBySourceLine(ctx, input.SourceType, *input.SourceID, lineKey, lineID)
			}
		}
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return movement, nil
}

func (r *PostgresRepository) FindMovementBySourceLine(ctx context.Context, sourceType string, sourceID uuid.UUID, lineKey string, lineID uuid.UUID) (*InventoryMovement, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, master_key, movement_type, source_type, source_id, item_id, warehouse_id, location_id,
		       quantity::float8, unit_cost::float8, amount::float8, currency, balance_after::float8,
		       organization_id, department_id, occurred_at, metadata, created_at
		FROM inventory_movements
		WHERE source_type = $1
		  AND source_id = $2
		  AND metadata ->> $3 = $4
		ORDER BY created_at DESC
		LIMIT 1`, sourceType, sourceID, lineKey, lineID.String())
	movement, err := scanMovement(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return movement, err
}

func (r *PostgresRepository) ListBalances(ctx context.Context, limit int) ([]InventoryBalance, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, master_key, item_id, warehouse_id, location_id, quantity::float8, reserved_qty::float8,
		       average_cost::float8, value_amount::float8, currency, organization_id, metadata, updated_at
		FROM inventory_balances
		ORDER BY updated_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []InventoryBalance
	for rows.Next() {
		item, err := scanBalance(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) ListMovements(ctx context.Context, limit int) ([]InventoryMovement, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, master_key, movement_type, source_type, source_id, item_id, warehouse_id, location_id,
		       quantity::float8, unit_cost::float8, amount::float8, currency, balance_after::float8,
		       organization_id, department_id, occurred_at, metadata, created_at
		FROM inventory_movements
		ORDER BY occurred_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []InventoryMovement
	for rows.Next() {
		item, err := scanMovement(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) CreateTransfer(ctx context.Context, input CreateInventoryTransferInput) (*InventoryTransfer, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var out InventoryTransfer
	var raw []byte
	err = tx.QueryRow(ctx, `
		INSERT INTO inventory_transfers (transfer_number, from_warehouse_id, to_warehouse_id, status, organization_id, department_id, workflow_instance_id, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, master_key, transfer_number, from_warehouse_id, to_warehouse_id, status, organization_id, department_id, workflow_instance_id, metadata, created_at, updated_at`,
		input.TransferNumber, input.FromWarehouseID, input.ToWarehouseID, input.Status, input.OrganizationID, input.DepartmentID, input.WorkflowID, jsonMap(input.Metadata)).
		Scan(&out.ID, &out.MasterKey, &out.TransferNumber, &out.FromWarehouseID, &out.ToWarehouseID, &out.Status, &out.OrganizationID, &out.DepartmentID, &out.WorkflowID, &raw, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return nil, err
	}
	out.Metadata = parseJSONMap(raw)
	for _, line := range input.Lines {
		var created InventoryTransferLine
		err = tx.QueryRow(ctx, `
			INSERT INTO inventory_transfer_lines (transfer_id, item_id, quantity, unit_cost)
			VALUES ($1, $2, $3, $4)
			RETURNING id, transfer_id, item_id, quantity::float8, unit_cost::float8`,
			out.ID, line.ItemID, line.Quantity, line.UnitCost).
			Scan(&created.ID, &created.TransferID, &created.ItemID, &created.Quantity, &created.UnitCost)
		if err != nil {
			return nil, err
		}
		out.Lines = append(out.Lines, created)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *PostgresRepository) ListTransfers(ctx context.Context, limit int) ([]InventoryTransfer, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, master_key, transfer_number, from_warehouse_id, to_warehouse_id, status, organization_id, department_id, workflow_instance_id, metadata, created_at, updated_at
		FROM inventory_transfers
		ORDER BY created_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []InventoryTransfer
	for rows.Next() {
		var item InventoryTransfer
		var raw []byte
		if err := rows.Scan(&item.ID, &item.MasterKey, &item.TransferNumber, &item.FromWarehouseID, &item.ToWarehouseID, &item.Status, &item.OrganizationID, &item.DepartmentID, &item.WorkflowID, &raw, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.Metadata = parseJSONMap(raw)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) CreateAdjustment(ctx context.Context, input CreateInventoryAdjustmentInput) (*InventoryAdjustment, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var out InventoryAdjustment
	var raw []byte
	err = tx.QueryRow(ctx, `
		INSERT INTO inventory_adjustments (adjustment_number, warehouse_id, reason, status, organization_id, department_id, workflow_instance_id, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, master_key, adjustment_number, warehouse_id, reason, status, organization_id, department_id, workflow_instance_id, metadata, created_at, updated_at`,
		input.AdjustmentNumber, input.WarehouseID, input.Reason, input.Status, input.OrganizationID, input.DepartmentID, input.WorkflowID, jsonMap(input.Metadata)).
		Scan(&out.ID, &out.MasterKey, &out.AdjustmentNumber, &out.WarehouseID, &out.Reason, &out.Status, &out.OrganizationID, &out.DepartmentID, &out.WorkflowID, &raw, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return nil, err
	}
	out.Metadata = parseJSONMap(raw)
	for _, line := range input.Lines {
		var created InventoryAdjustmentLine
		err = tx.QueryRow(ctx, `
			INSERT INTO inventory_adjustment_lines (adjustment_id, item_id, quantity_delta, unit_cost)
			VALUES ($1, $2, $3, $4)
			RETURNING id, adjustment_id, item_id, quantity_delta::float8, unit_cost::float8`,
			out.ID, line.ItemID, line.QuantityDelta, line.UnitCost).
			Scan(&created.ID, &created.AdjustmentID, &created.ItemID, &created.QuantityDelta, &created.UnitCost)
		if err != nil {
			return nil, err
		}
		out.Lines = append(out.Lines, created)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *PostgresRepository) ListAdjustments(ctx context.Context, limit int) ([]InventoryAdjustment, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, master_key, adjustment_number, warehouse_id, reason, status, organization_id, department_id, workflow_instance_id, metadata, created_at, updated_at
		FROM inventory_adjustments
		ORDER BY created_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []InventoryAdjustment
	for rows.Next() {
		var item InventoryAdjustment
		var raw []byte
		if err := rows.Scan(&item.ID, &item.MasterKey, &item.AdjustmentNumber, &item.WarehouseID, &item.Reason, &item.Status, &item.OrganizationID, &item.DepartmentID, &item.WorkflowID, &raw, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.Metadata = parseJSONMap(raw)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) CreateCount(ctx context.Context, input CreateInventoryCountInput) (*InventoryCount, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var out InventoryCount
	var raw []byte
	err = tx.QueryRow(ctx, `
		INSERT INTO inventory_counts (count_number, warehouse_id, status, organization_id, department_id, workflow_instance_id, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, master_key, count_number, warehouse_id, status, organization_id, department_id, workflow_instance_id, metadata, created_at, updated_at`,
		input.CountNumber, input.WarehouseID, input.Status, input.OrganizationID, input.DepartmentID, input.WorkflowID, jsonMap(input.Metadata)).
		Scan(&out.ID, &out.MasterKey, &out.CountNumber, &out.WarehouseID, &out.Status, &out.OrganizationID, &out.DepartmentID, &out.WorkflowID, &raw, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return nil, err
	}
	out.Metadata = parseJSONMap(raw)
	for _, line := range input.Lines {
		var created InventoryCountLine
		variance := line.CountedQty - line.BookQty
		err = tx.QueryRow(ctx, `
			INSERT INTO inventory_count_lines (count_id, item_id, book_qty, counted_qty, variance_qty)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id, count_id, item_id, book_qty::float8, counted_qty::float8, variance_qty::float8`,
			out.ID, line.ItemID, line.BookQty, line.CountedQty, variance).
			Scan(&created.ID, &created.CountID, &created.ItemID, &created.BookQty, &created.CountedQty, &created.VarianceQty)
		if err != nil {
			return nil, err
		}
		out.Lines = append(out.Lines, created)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *PostgresRepository) ListCounts(ctx context.Context, limit int) ([]InventoryCount, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, master_key, count_number, warehouse_id, status, organization_id, department_id, workflow_instance_id, metadata, created_at, updated_at
		FROM inventory_counts
		ORDER BY created_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []InventoryCount
	for rows.Next() {
		var item InventoryCount
		var raw []byte
		if err := rows.Scan(&item.ID, &item.MasterKey, &item.CountNumber, &item.WarehouseID, &item.Status, &item.OrganizationID, &item.DepartmentID, &item.WorkflowID, &raw, &item.CreatedAt, &item.UpdatedAt); err != nil {
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

func scanBusinessPartner(row rowScanner) (*BusinessPartner, error) {
	var item BusinessPartner
	var raw []byte
	err := row.Scan(&item.ID, &item.MasterKey, &item.PartnerCode, &item.PartnerType, &item.Name, &item.Email, &item.Phone, &item.Status, &item.OrganizationID, &raw, &item.CreatedAt, &item.UpdatedAt)
	item.Metadata = parseJSONMap(raw)
	return &item, err
}

func scanItem(row rowScanner) (*Item, error) {
	var item Item
	var raw []byte
	err := row.Scan(&item.ID, &item.MasterKey, &item.ItemCode, &item.Name, &item.ItemType, &item.BaseUOM, &item.Status, &item.OrganizationID, &raw, &item.CreatedAt, &item.UpdatedAt)
	item.Metadata = parseJSONMap(raw)
	return &item, err
}

func scanWarehouse(row rowScanner) (*Warehouse, error) {
	var item Warehouse
	var raw []byte
	err := row.Scan(&item.ID, &item.MasterKey, &item.WarehouseCode, &item.Name, &item.Status, &item.OrganizationID, &item.DepartmentID, &raw, &item.CreatedAt, &item.UpdatedAt)
	item.Metadata = parseJSONMap(raw)
	return &item, err
}

func scanWarehouseLocation(row rowScanner) (*WarehouseLocation, error) {
	var item WarehouseLocation
	var raw []byte
	err := row.Scan(&item.ID, &item.MasterKey, &item.WarehouseID, &item.LocationCode, &item.Name, &item.Status, &raw, &item.CreatedAt, &item.UpdatedAt)
	item.Metadata = parseJSONMap(raw)
	return &item, err
}

func scanBalance(row rowScanner) (*InventoryBalance, error) {
	var item InventoryBalance
	var raw []byte
	err := row.Scan(&item.ID, &item.MasterKey, &item.ItemID, &item.WarehouseID, &item.LocationID, &item.Quantity, &item.ReservedQty, &item.AverageCost, &item.ValueAmount, &item.Currency, &item.OrganizationID, &raw, &item.UpdatedAt)
	item.Metadata = parseJSONMap(raw)
	return &item, err
}

func scanMovement(row rowScanner) (*InventoryMovement, error) {
	var item InventoryMovement
	var raw []byte
	err := row.Scan(&item.ID, &item.MasterKey, &item.MovementType, &item.SourceType, &item.SourceID, &item.ItemID, &item.WarehouseID, &item.LocationID, &item.Quantity, &item.UnitCost, &item.Amount, &item.Currency, &item.BalanceAfter, &item.OrganizationID, &item.DepartmentID, &item.OccurredAt, &raw, &item.CreatedAt)
	item.Metadata = parseJSONMap(raw)
	return &item, err
}

func scanBalanceRow(row rowScanner) (*InventoryBalance, error) {
	return scanBalance(row)
}

func scanMovementRow(row rowScanner) (*InventoryMovement, error) {
	return scanMovement(row)
}

func movementLineIdentity(input CreateInventoryMovementInput) (string, uuid.UUID, bool) {
	for _, key := range []string{"receipt_line_id", "shipment_line_id"} {
		raw, ok := input.Metadata[key]
		if !ok {
			continue
		}
		value, ok := raw.(string)
		if !ok {
			continue
		}
		id, err := uuid.Parse(value)
		if err == nil {
			return key, id, true
		}
	}
	return "", uuid.Nil, false
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
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
