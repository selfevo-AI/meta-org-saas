package finance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type secretBox interface {
	Encrypt(string) (string, error)
	Decrypt(string) (string, error)
}

type PostgresRepository struct {
	db  *pgxpool.Pool
	box secretBox
}

func NewRepository(db *pgxpool.Pool, box secretBox) *PostgresRepository {
	return &PostgresRepository{db: db, box: box}
}

func (r *PostgresRepository) CreateAdapter(ctx context.Context, input CreateAdapterInput) (*FinanceAdapter, error) {
	encrypted, err := r.box.Encrypt(input.Secret)
	if err != nil {
		return nil, fmt.Errorf("encrypt finance adapter secret: %w", err)
	}
	adapter := &FinanceAdapter{}
	err = scanAdapter(r.db.QueryRow(ctx, `
		INSERT INTO finance_adapters (
			name, endpoint_url, auth_type, encrypted_secret, masked_secret,
			status, timeout_ms, retry_count, adapter_type, direction, field_mapping,
			pull_config, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id, name, endpoint_url, auth_type, adapter_type, direction, masked_secret, status,
			timeout_ms, retry_count, field_mapping, pull_config, last_sync_at, last_sync_status, metadata, created_at, updated_at
	`, input.Name, input.EndpointURL, input.AuthType, encrypted, maskSecret(input.Secret), input.Status,
		input.TimeoutMS, input.RetryCount, input.AdapterType, input.Direction, mustJSON(input.FieldMapping),
		mustJSON(input.PullConfig), mustJSON(input.Metadata)), adapter)
	if err != nil {
		return nil, fmt.Errorf("create finance adapter: %w", err)
	}
	return adapter, nil
}

func (r *PostgresRepository) ListAdapters(ctx context.Context, limit int) ([]FinanceAdapter, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, endpoint_url, auth_type, adapter_type, direction, masked_secret, status,
			timeout_ms, retry_count, field_mapping, pull_config, last_sync_at, last_sync_status, metadata, created_at, updated_at
		FROM finance_adapters
		ORDER BY created_at DESC
		LIMIT $1
	`, normalizeLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("list finance adapters: %w", err)
	}
	defer rows.Close()
	adapters := []FinanceAdapter{}
	for rows.Next() {
		var adapter FinanceAdapter
		if err := scanAdapter(rows, &adapter); err != nil {
			return nil, fmt.Errorf("scan finance adapter: %w", err)
		}
		adapters = append(adapters, adapter)
	}
	return adapters, rows.Err()
}

func (r *PostgresRepository) UpdateAdapter(ctx context.Context, id uuid.UUID, input UpdateAdapterInput) (*FinanceAdapter, error) {
	var encryptedSecret *string
	var maskedSecret *string
	if input.Secret != nil {
		encrypted, err := r.box.Encrypt(*input.Secret)
		if err != nil {
			return nil, fmt.Errorf("encrypt finance adapter secret: %w", err)
		}
		masked := maskSecret(*input.Secret)
		encryptedSecret = &encrypted
		maskedSecret = &masked
	}
	adapter := &FinanceAdapter{}
	err := scanAdapter(r.db.QueryRow(ctx, `
		UPDATE finance_adapters
		SET name = COALESCE($2, name),
			endpoint_url = COALESCE($3, endpoint_url),
			auth_type = COALESCE($4, auth_type),
			adapter_type = COALESCE($5, adapter_type),
			direction = COALESCE($6, direction),
			encrypted_secret = COALESCE($7, encrypted_secret),
			masked_secret = COALESCE($8, masked_secret),
			status = COALESCE($9, status),
			timeout_ms = COALESCE($10, timeout_ms),
			retry_count = COALESCE($11, retry_count),
			field_mapping = CASE WHEN $12::jsonb IS NULL THEN field_mapping ELSE $12::jsonb END,
			pull_config = CASE WHEN $13::jsonb IS NULL THEN pull_config ELSE $13::jsonb END,
			metadata = metadata || COALESCE($14::jsonb, '{}'::jsonb),
			updated_at = NOW()
		WHERE id = $1
		RETURNING id, name, endpoint_url, auth_type, adapter_type, direction, masked_secret, status,
			timeout_ms, retry_count, field_mapping, pull_config, last_sync_at, last_sync_status, metadata, created_at, updated_at
	`, id, input.Name, input.EndpointURL, input.AuthType, input.AdapterType, input.Direction, encryptedSecret, maskedSecret, input.Status,
		input.TimeoutMS, input.RetryCount, nullableJSON(input.FieldMapping), nullableJSON(input.PullConfig), nullableJSON(input.Metadata)), adapter)
	if err != nil {
		return nil, fmt.Errorf("update finance adapter: %w", err)
	}
	return adapter, nil
}

func (r *PostgresRepository) GetAdapterSecret(ctx context.Context, id uuid.UUID) (AdapterSecret, error) {
	var adapter AdapterSecret
	var encrypted string
	var fieldMappingJSON []byte
	var pullConfigJSON []byte
	err := r.db.QueryRow(ctx, `
		SELECT id, name, endpoint_url, auth_type, adapter_type, direction, encrypted_secret, status,
		       timeout_ms, retry_count, field_mapping, pull_config
		FROM finance_adapters
		WHERE id = $1
	`, id).Scan(&adapter.ID, &adapter.Name, &adapter.EndpointURL, &adapter.AuthType, &adapter.AdapterType, &adapter.Direction, &encrypted,
		&adapter.Status, &adapter.TimeoutMS, &adapter.RetryCount, &fieldMappingJSON, &pullConfigJSON)
	if err != nil {
		return adapter, fmt.Errorf("get finance adapter secret: %w", err)
	}
	secret, err := r.box.Decrypt(encrypted)
	if err != nil {
		return adapter, fmt.Errorf("decrypt finance adapter secret: %w", err)
	}
	adapter.Secret = secret
	_ = json.Unmarshal(fieldMappingJSON, &adapter.FieldMapping)
	_ = json.Unmarshal(pullConfigJSON, &adapter.PullConfig)
	return adapter, nil
}

func (r *PostgresRepository) CreateExportBatch(ctx context.Context, input CreateExportBatchInput) (*ExportBatch, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin finance export batch: %w", err)
	}
	defer tx.Rollback(ctx)

	existingID, err := r.findBatchByIdempotencyKey(ctx, tx, input.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	if existingID != nil {
		batch, err := r.getExportBatch(ctx, tx, *existingID)
		if err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit finance export idempotent read: %w", err)
		}
		return batch, nil
	}

	batch := &ExportBatch{}
	err = scanBatch(tx.QueryRow(ctx, `
		INSERT INTO finance_export_batches (
			adapter_id, period_start, period_end, status, currency,
			idempotency_key, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, adapter_id, period_start, period_end, status, currency,
			total_amount::float8, external_batch_id, error_message, idempotency_key,
			metadata, created_at, submitted_at, updated_at
	`, input.AdapterID, input.periodStartTime, input.periodEndTime, BatchDraft, input.Currency,
		input.IdempotencyKey, mustJSON(input.Metadata)), batch)
	if err != nil {
		return nil, fmt.Errorf("create finance export batch: %w", err)
	}

	costRows, err := r.exportableCostRows(ctx, tx, input.periodStartTime, input.periodEndTime, input.Currency)
	if err != nil {
		return nil, err
	}
	total := 0.0
	lines := make([]ExportLine, 0, len(costRows))
	for _, cost := range costRows {
		line := &ExportLine{}
		err := scanLine(tx.QueryRow(ctx, `
			INSERT INTO finance_export_lines (
				batch_id, usage_ledger_id, cost_ledger_entry_id, project_cost_entry_id,
				organization_id, department_id, project_id, provider_id, model_id, amount,
				currency, metadata
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			RETURNING id, batch_id, usage_ledger_id, cost_ledger_entry_id, project_cost_entry_id, organization_id,
				department_id, project_id, provider_id, model_id, amount::float8, currency,
				external_line_id, status, metadata, created_at
		`, batch.ID, cost.UsageLedgerID, cost.CostLedgerEntryID, cost.ProjectCostEntryID, cost.OrganizationID, cost.DepartmentID,
			cost.ProjectID, cost.ProviderID, cost.ModelID, cost.Amount, cost.Currency, mustJSON(map[string]any{
				"cost_ledger_created_at": cost.CostCreatedAt.Format(time.RFC3339),
				"cost_source_type":       cost.SourceType,
			})), line)
		if err != nil {
			return nil, fmt.Errorf("create finance export line: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE cost_ledger_entries
			SET finance_export_line_id = $1
			WHERE id = $2
		`, line.ID, cost.CostLedgerEntryID); err != nil {
			return nil, fmt.Errorf("link cost ledger to finance export line: %w", err)
		}
		if cost.UsageLedgerID != nil {
			if _, err := tx.Exec(ctx, `
				UPDATE ai_usage_ledger
				SET finance_export_line_id = $1
				WHERE id = $2
			`, line.ID, *cost.UsageLedgerID); err != nil {
				return nil, fmt.Errorf("link usage ledger to finance export line: %w", err)
			}
		}
		lines = append(lines, *line)
		total += line.Amount
	}
	status := BatchDraft
	if len(lines) > 0 {
		status = BatchReady
	}
	if err := scanBatch(tx.QueryRow(ctx, `
		UPDATE finance_export_batches
		SET total_amount = $2, status = $3, updated_at = NOW()
		WHERE id = $1
		RETURNING id, adapter_id, period_start, period_end, status, currency,
			total_amount::float8, external_batch_id, error_message, idempotency_key,
			metadata, created_at, submitted_at, updated_at
	`, batch.ID, total, status), batch); err != nil {
		return nil, fmt.Errorf("update finance export batch total: %w", err)
	}
	batch.Lines = lines
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit finance export batch: %w", err)
	}
	return batch, nil
}

func (r *PostgresRepository) ListExportBatches(ctx context.Context, limit int) ([]ExportBatch, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, adapter_id, period_start, period_end, status, currency,
			total_amount::float8, external_batch_id, error_message, idempotency_key,
			metadata, created_at, submitted_at, updated_at
		FROM finance_export_batches
		WHERE ($1::uuid IS NULL OR EXISTS (
			SELECT 1 FROM finance_export_lines l
			WHERE l.batch_id = finance_export_batches.id AND l.organization_id IS NOT DISTINCT FROM $1
		))
		ORDER BY created_at DESC
		LIMIT $2
	`, nullableUUID(currentTenantOrganizationID(ctx)), normalizeLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("list finance export batches: %w", err)
	}
	defer rows.Close()
	batches := []ExportBatch{}
	for rows.Next() {
		var batch ExportBatch
		if err := scanBatch(rows, &batch); err != nil {
			return nil, fmt.Errorf("scan finance export batch: %w", err)
		}
		batches = append(batches, batch)
	}
	return batches, rows.Err()
}

func (r *PostgresRepository) GetExportBatch(ctx context.Context, id uuid.UUID) (*ExportBatch, error) {
	return r.getExportBatch(ctx, r.db, id)
}

func (r *PostgresRepository) UpdateExportBatchStatus(ctx context.Context, id uuid.UUID, input UpdateExportBatchStatusInput) (*ExportBatch, error) {
	batch := &ExportBatch{}
	err := scanBatch(r.db.QueryRow(ctx, `
		UPDATE finance_export_batches
		SET status = COALESCE(NULLIF($2, ''), status),
			external_batch_id = COALESCE(NULLIF($3, ''), external_batch_id),
			error_message = CASE
				WHEN $2 IN ('exported', 'acknowledged', 'posted', 'reconciled') THEN ''
				ELSE COALESCE(NULLIF($4, ''), error_message)
			END,
			metadata = metadata || COALESCE($5::jsonb, '{}'::jsonb),
			submitted_at = CASE WHEN $6 THEN COALESCE(submitted_at, NOW()) ELSE submitted_at END,
			updated_at = NOW()
		WHERE id = $1
		RETURNING id, adapter_id, period_start, period_end, status, currency,
			total_amount::float8, external_batch_id, error_message, idempotency_key,
			metadata, created_at, submitted_at, updated_at
	`, id, input.Status, input.ExternalBatchID, input.ErrorMessage, nullableJSON(input.Metadata), input.Submitted), batch)
	if err != nil {
		return nil, fmt.Errorf("update finance export batch status: %w", err)
	}
	lines, err := r.listBatchLines(ctx, r.db, id)
	if err != nil {
		return nil, err
	}
	batch.Lines = lines
	return batch, nil
}

func (r *PostgresRepository) RecordWebhookEvent(ctx context.Context, input RecordWebhookEventInput) (*WebhookEvent, error) {
	event := &WebhookEvent{}
	err := scanWebhookEvent(r.db.QueryRow(ctx, `
		INSERT INTO finance_webhook_events (
			adapter_id, batch_id, event_type, signature_valid, payload,
			processed, error_message
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, adapter_id, batch_id, event_type, signature_valid,
			payload, processed, error_message, created_at
	`, input.AdapterID, input.BatchID, input.EventType, input.SignatureValid, mustJSON(input.Payload),
		input.Processed, input.ErrorMessage), event)
	if err != nil {
		return nil, fmt.Errorf("record finance webhook event: %w", err)
	}
	return event, nil
}

func (r *PostgresRepository) UpdateExportLineStatus(ctx context.Context, id uuid.UUID, input UpdateExportLineStatusInput) (*ExportLine, error) {
	line := &ExportLine{}
	err := scanLine(r.db.QueryRow(ctx, `
		UPDATE finance_export_lines
		SET status = COALESCE(NULLIF($2, ''), status),
			external_line_id = COALESCE(NULLIF($3, ''), external_line_id),
			metadata = metadata || COALESCE($4::jsonb, '{}'::jsonb)
		WHERE id = $1
		RETURNING id, batch_id, usage_ledger_id, cost_ledger_entry_id, project_cost_entry_id, organization_id,
			department_id, project_id, provider_id, model_id, amount::float8, currency,
			external_line_id, status, metadata, created_at
	`, id, input.Status, input.ExternalLineID, nullableJSON(input.Metadata)), line)
	if err != nil {
		return nil, fmt.Errorf("update finance export line status: %w", err)
	}
	return line, nil
}

func (r *PostgresRepository) LinkProjectCostEntry(ctx context.Context, lineID uuid.UUID, entryID uuid.UUID) error {
	var usageID, costLedgerEntryID pgtype.UUID
	if err := r.db.QueryRow(ctx, `
		UPDATE finance_export_lines
		SET project_cost_entry_id = $2,
			metadata = metadata || '{"project_cost_posted": true}'::jsonb
		WHERE id = $1
		RETURNING usage_ledger_id, cost_ledger_entry_id
	`, lineID, entryID).Scan(&usageID, &costLedgerEntryID); err != nil {
		return fmt.Errorf("link finance line to project cost entry: %w", err)
	}
	if costLedgerEntryID.Valid {
		costLedgerUUID := uuidPtr(costLedgerEntryID)
		if costLedgerUUID != nil {
			if _, err := r.db.Exec(ctx, `
				UPDATE cost_ledger_entries
				SET metadata = metadata || jsonb_build_object('project_cost_entry_id', $2::text)
				WHERE id = $1
			`, *costLedgerUUID, entryID.String()); err != nil {
				return fmt.Errorf("link cost ledger to project cost entry: %w", err)
			}
		}
	}
	if usageID.Valid {
		usageUUID := uuidPtr(usageID)
		if usageUUID == nil {
			return nil
		}
		if _, err := r.db.Exec(ctx, `
			UPDATE ai_usage_ledger
			SET project_cost_entry_id = $2,
				posted_to_project_cost = TRUE
			WHERE id = $1
		`, *usageUUID, entryID); err != nil {
			return fmt.Errorf("link usage ledger to project cost entry: %w", err)
		}
	}
	return nil
}

func (r *PostgresRepository) ListReconciliation(ctx context.Context, limit int) ([]ReconciliationItem, error) {
	rows, err := r.db.Query(ctx, `
		WITH batches AS (
			SELECT id, adapter_id, status, currency, total_amount::float8 AS total_amount,
				CASE
					WHEN (metadata->>'external_amount') ~ '^-?[0-9]+(\.[0-9]+)?$'
						THEN (metadata->>'external_amount')::float8
					ELSE total_amount::float8
				END AS external_amount,
				external_batch_id, error_message, submitted_at, updated_at
			FROM finance_export_batches
			WHERE status IN ('exported', 'acknowledged', 'posted', 'reconciled', 'failed')
				AND ($1::uuid IS NULL OR EXISTS (
					SELECT 1 FROM finance_export_lines l
					WHERE l.batch_id = finance_export_batches.id AND l.organization_id IS NOT DISTINCT FROM $1
				))
		)
		SELECT id, adapter_id, status, currency, total_amount,
			external_amount, external_amount - total_amount AS difference_amount,
			external_batch_id, error_message, submitted_at, updated_at
		FROM batches
		ORDER BY updated_at DESC
		LIMIT $2
	`, nullableUUID(currentTenantOrganizationID(ctx)), normalizeLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("list finance reconciliation: %w", err)
	}
	defer rows.Close()
	items := []ReconciliationItem{}
	for rows.Next() {
		var item ReconciliationItem
		if err := rows.Scan(&item.BatchID, &item.AdapterID, &item.Status, &item.Currency,
			&item.TotalAmount, &item.ExternalAmount, &item.DifferenceAmount, &item.ExternalBatchID,
			&item.ErrorMessage, &item.SubmittedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan finance reconciliation item: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) CreateImportBatch(ctx context.Context, adapterID *uuid.UUID, sourceType string, fileName string, total int, metadata map[string]any) (*ImportBatch, error) {
	batch := &ImportBatch{}
	err := scanImportBatch(r.db.QueryRow(ctx, `
		INSERT INTO finance_import_batches(adapter_id, source_type, file_name, total_records, metadata)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, adapter_id, source_type, file_name, status, total_records, processed_records,
		          failed_records, metadata, created_at, completed_at
	`, adapterID, sourceType, fileName, total, mustJSON(metadata)), batch)
	if err != nil {
		return nil, fmt.Errorf("create finance import batch: %w", err)
	}
	return batch, nil
}

func (r *PostgresRepository) CompleteImportBatch(ctx context.Context, id uuid.UUID, processed int, failed int) (*ImportBatch, error) {
	status := "completed"
	if failed > 0 && processed > 0 {
		status = "completed_with_errors"
	} else if failed > 0 {
		status = "failed"
	}
	batch := &ImportBatch{}
	err := scanImportBatch(r.db.QueryRow(ctx, `
		UPDATE finance_import_batches
		SET status = $2, processed_records = $3, failed_records = $4, completed_at = NOW()
		WHERE id = $1
		RETURNING id, adapter_id, source_type, file_name, status, total_records, processed_records,
		          failed_records, metadata, created_at, completed_at
	`, id, status, processed, failed), batch)
	if err != nil {
		return nil, fmt.Errorf("complete finance import batch: %w", err)
	}
	return batch, nil
}

func (r *PostgresRepository) ListImportBatches(ctx context.Context, limit int) ([]ImportBatch, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, adapter_id, source_type, file_name, status, total_records, processed_records,
		       failed_records, metadata, created_at, completed_at
		FROM finance_import_batches
		WHERE ($1::uuid IS NULL OR EXISTS (
			SELECT 1
			FROM finance_import_records rec
			LEFT JOIN cost_ledger_entries c ON c.id = rec.cost_ledger_entry_id
			LEFT JOIN finance_payables p ON p.id = rec.payable_id
			WHERE rec.batch_id = finance_import_batches.id
				AND (c.organization_id IS NOT DISTINCT FROM $1 OR p.organization_id IS NOT DISTINCT FROM $1)
		))
		ORDER BY created_at DESC
		LIMIT $2
	`, nullableUUID(currentTenantOrganizationID(ctx)), normalizeLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("list finance import batches: %w", err)
	}
	defer rows.Close()
	items := []ImportBatch{}
	for rows.Next() {
		var item ImportBatch
		if err := scanImportBatch(rows, &item); err != nil {
			return nil, fmt.Errorf("scan finance import batch: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) ListImportRecords(ctx context.Context, limit int) ([]ImportRecord, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, batch_id, adapter_id, external_record_id, expense_type, raw_payload,
		       normalized_payload, cost_ledger_entry_id, payable_id, status, error_message,
		       metadata, created_at
		FROM finance_import_records
		WHERE ($1::uuid IS NULL
			OR EXISTS (
				SELECT 1 FROM cost_ledger_entries c
				WHERE c.id = finance_import_records.cost_ledger_entry_id AND c.organization_id IS NOT DISTINCT FROM $1
			)
			OR EXISTS (
				SELECT 1 FROM finance_payables p
				WHERE p.id = finance_import_records.payable_id AND p.organization_id IS NOT DISTINCT FROM $1
			))
		ORDER BY created_at DESC
		LIMIT $2
	`, nullableUUID(currentTenantOrganizationID(ctx)), normalizeLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("list finance import records: %w", err)
	}
	defer rows.Close()
	items := []ImportRecord{}
	for rows.Next() {
		var item ImportRecord
		if err := scanImportRecord(rows, &item); err != nil {
			return nil, fmt.Errorf("scan finance import record: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) CreateImportedExpense(ctx context.Context, batchID uuid.UUID, adapterID uuid.UUID, raw map[string]any, input FinanceExpenseInput, occurredAt time.Time, dates financeExpenseDates) (*ImportRecord, error) {
	existing, err := r.findImportRecord(ctx, adapterID, input.ExternalRecordID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin finance import expense: %w", err)
	}
	defer tx.Rollback(ctx)

	var payableID *uuid.UUID
	if input.InvoiceNumber != "" || input.PaymentStatus != "" {
		payable, err := r.createPayable(ctx, tx, CreatePayableInput{
			PayableType:       payableTypeForExpense(input.ExpenseType),
			SourceType:        "finance_import",
			ExternalPayableID: input.ExternalRecordID,
			InvoiceNumber:     input.InvoiceNumber,
			VendorID:          input.VendorID,
			VendorName:        input.VendorName,
			EmployeeID:        input.EmployeeID,
			EmployeeName:      input.EmployeeName,
			AgentID:           input.AgentID,
			ProjectID:         input.ProjectID,
			OrganizationID:    input.OrganizationID,
			DepartmentID:      input.DepartmentID,
			AccountCode:       input.AccountCode,
			AccountName:       input.AccountName,
			CostCenterCode:    input.CostCenterCode,
			CostCenterName:    input.CostCenterName,
			Amount:            input.Amount,
			TaxAmount:         input.TaxAmount,
			Currency:          input.Currency,
			PeriodStart:       input.PeriodStart,
			PeriodEnd:         input.PeriodEnd,
			InvoiceDate:       input.InvoiceDate,
			DueDate:           input.PaymentDueDate,
			Status:            payableStatusFromPayment(input.PaymentStatus),
			Metadata:          input.Metadata,
		}, dates)
		if err != nil {
			return nil, err
		}
		payableID = &payable.ID
	}

	var ledgerID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO cost_ledger_entries (
			ledger_type, cost_category, source_type, source_id, organization_id, department_id,
			requirement_id, project_id, workflow_id, task_id, capability_id, actor_id, actor_type,
			resource_type, amount, currency, base_amount, base_currency, occurred_at, status,
			description, metadata, expense_type, account_code, account_name, cost_center_code,
			cost_center_name, vendor_id, vendor_name, employee_id, employee_name, agent_id,
			agent_name, tax_amount, tax_rate, invoice_number, invoice_date, payment_status,
			payment_due_date, paid_at, period_start, period_end, finance_payable_id
		)
		VALUES (
			'actual', $1, 'finance_import', NULL, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13, $12, $13, $14, 'posted', $15, $16, $17, $18, $19, $20,
			$21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33, $34, $35, $36, $37
		)
		RETURNING id
	`, input.CostCategory, input.OrganizationID, input.DepartmentID, input.RequirementID, input.ProjectID,
		input.WorkflowID, input.TaskID, input.CapabilityID, actorIDForExpense(input), actorTypeForExpense(input),
		input.ExpenseType, input.Amount, input.Currency, occurredAt, input.Description, mustJSON(input.Metadata),
		input.ExpenseType, input.AccountCode, input.AccountName, input.CostCenterCode, input.CostCenterName,
		input.VendorID, input.VendorName, input.EmployeeID, input.EmployeeName, input.AgentID, input.AgentName,
		input.TaxAmount, input.TaxRate, input.InvoiceNumber, dates.InvoiceDate, input.PaymentStatus,
		dates.PaymentDueDate, dates.PaidAt, dates.PeriodStart, dates.PeriodEnd, payableID).Scan(&ledgerID)
	if err != nil {
		return nil, fmt.Errorf("create imported cost ledger entry: %w", err)
	}

	record := &ImportRecord{}
	normalized := expensePayload(input)
	err = scanImportRecord(tx.QueryRow(ctx, `
		INSERT INTO finance_import_records (
			batch_id, adapter_id, external_record_id, expense_type, raw_payload,
			normalized_payload, cost_ledger_entry_id, payable_id, status, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'posted', $9)
		RETURNING id, batch_id, adapter_id, external_record_id, expense_type, raw_payload,
		          normalized_payload, cost_ledger_entry_id, payable_id, status, error_message,
		          metadata, created_at
	`, batchID, adapterID, input.ExternalRecordID, input.ExpenseType, mustJSON(raw), mustJSON(normalized),
		ledgerID, payableID, mustJSON(map[string]any{"auto_posted": true})), record)
	if err != nil {
		return nil, fmt.Errorf("create finance import record: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE cost_ledger_entries SET source_id = $2, finance_import_record_id = $2 WHERE id = $1`, ledgerID, record.ID); err != nil {
		return nil, fmt.Errorf("link import record to cost ledger: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit finance import expense: %w", err)
	}
	return record, nil
}

func (r *PostgresRepository) CreatePayable(ctx context.Context, input CreatePayableInput, dates financeExpenseDates) (*Payable, error) {
	return r.createPayable(ctx, r.db, input, dates)
}

func (r *PostgresRepository) ListPayables(ctx context.Context, limit int) ([]Payable, error) {
	rows, err := r.db.Query(ctx, payableSelectSQL()+`
		WHERE ($1::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $1)
		ORDER BY created_at DESC LIMIT $2`, nullableUUID(currentTenantOrganizationID(ctx)), normalizeLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("list finance payables: %w", err)
	}
	defer rows.Close()
	items := []Payable{}
	for rows.Next() {
		var item Payable
		if err := scanPayable(rows, &item); err != nil {
			return nil, fmt.Errorf("scan finance payable: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) FindPayableBySource(ctx context.Context, sourceType string, sourceID uuid.UUID) (*Payable, error) {
	payable := &Payable{}
	err := scanPayable(r.db.QueryRow(ctx, payableSelectSQL()+`
		WHERE source_type = $1
		  AND source_id = $2
		  AND ($3::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $3)
		ORDER BY created_at DESC
		LIMIT 1`, sourceType, sourceID, nullableUUID(currentTenantOrganizationID(ctx))), payable)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find finance payable by source: %w", err)
	}
	return payable, nil
}

func (r *PostgresRepository) CreatePayment(ctx context.Context, input CreatePaymentInput, paidAt *time.Time) (*Payment, error) {
	payment := &Payment{}
	err := scanPayment(r.db.QueryRow(ctx, `
		INSERT INTO finance_payments (
			payment_number, external_payment_id, payment_method, payer_account, payee_account,
			vendor_id, vendor_name, employee_id, employee_name, amount, currency, paid_at,
			status, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id, payment_number, external_payment_id, payment_method, payer_account, payee_account,
		          vendor_id, vendor_name, employee_id, employee_name, amount::float8, currency,
		          paid_at, status, metadata, created_at, updated_at
	`, input.PaymentNumber, input.ExternalPaymentID, input.PaymentMethod, input.PayerAccount, input.PayeeAccount,
		input.VendorID, input.VendorName, input.EmployeeID, input.EmployeeName, input.Amount, input.Currency,
		paidAt, input.Status, mustJSON(input.Metadata)), payment)
	if err != nil {
		return nil, fmt.Errorf("create finance payment: %w", err)
	}
	return payment, nil
}

func (r *PostgresRepository) ListPayments(ctx context.Context, limit int) ([]Payment, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, payment_number, external_payment_id, payment_method, payer_account, payee_account,
		       vendor_id, vendor_name, employee_id, employee_name, amount::float8, currency,
		       paid_at, status, metadata, created_at, updated_at
		FROM finance_payments
		ORDER BY created_at DESC
		LIMIT $1
	`, normalizeLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("list finance payments: %w", err)
	}
	defer rows.Close()
	items := []Payment{}
	for rows.Next() {
		var item Payment
		if err := scanPayment(rows, &item); err != nil {
			return nil, fmt.Errorf("scan finance payment: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) AllocatePayment(ctx context.Context, paymentID uuid.UUID, input AllocatePaymentInput) (*PaymentAllocation, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin finance payment allocation: %w", err)
	}
	defer tx.Rollback(ctx)
	allocation := &PaymentAllocation{}
	err = scanPaymentAllocation(tx.QueryRow(ctx, `
		INSERT INTO finance_payment_allocations(payment_id, payable_id, amount, currency, metadata)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, payment_id, payable_id, amount::float8, currency, metadata, created_at
	`, paymentID, input.PayableID, input.Amount, input.Currency, mustJSON(input.Metadata)), allocation)
	if err != nil {
		return nil, fmt.Errorf("create finance payment allocation: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE finance_payables
		SET paid_amount = paid_amount + $2,
		    status = CASE
		        WHEN paid_amount + $2 >= amount THEN 'paid'
		        WHEN paid_amount + $2 > 0 THEN 'partially_paid'
		        ELSE status
		    END,
		    updated_at = NOW()
		WHERE id = $1
	`, input.PayableID, input.Amount); err != nil {
		return nil, fmt.Errorf("update payable paid amount: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE cost_ledger_entries
		SET finance_payment_id = $2,
		    payment_status = CASE
		        WHEN p.status = 'paid' THEN 'paid'
		        WHEN p.status = 'partially_paid' THEN 'partially_paid'
		        ELSE payment_status
		    END,
		    paid_at = CASE WHEN p.status = 'paid' THEN NOW() ELSE paid_at END
		FROM finance_payables p
		WHERE cost_ledger_entries.finance_payable_id = p.id
		  AND p.id = $1
	`, input.PayableID, paymentID); err != nil {
		return nil, fmt.Errorf("update cost ledger payment status: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit finance payment allocation: %w", err)
	}
	return allocation, nil
}

func (r *PostgresRepository) CreateSettlementOrder(ctx context.Context, input CreateSettlementOrderInput) (*SettlementOrder, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin finance settlement order: %w", err)
	}
	defer tx.Rollback(ctx)

	subtotal, taxAmount := settlementTotals(input.Lines)
	order := &SettlementOrder{}
	err = scanSettlementOrder(tx.QueryRow(ctx, `
		INSERT INTO finance_settlement_orders (
			settlement_number, project_id, requirement_id, deliverable_id, customer_id, customer_name,
			title, description, subtotal, tax_amount, total_amount, currency, settlement_date,
			due_date, status, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING id, settlement_number, project_id, requirement_id, deliverable_id, customer_id,
		          customer_name, title, description, subtotal::float8, tax_amount::float8,
		          total_amount::float8, currency, settlement_date, due_date, status, receivable_id,
		          metadata, created_at, updated_at
	`, input.SettlementNumber, input.ProjectID, input.RequirementID, input.DeliverableID, input.CustomerID,
		input.CustomerName, input.Title, input.Description, subtotal, taxAmount, subtotal+taxAmount,
		input.Currency, parseDateForSQL(input.SettlementDate), parseDateForSQL(input.DueDate), input.Status,
		mustJSON(input.Metadata)), order)
	if err != nil {
		return nil, fmt.Errorf("create finance settlement order: %w", err)
	}
	lines, err := r.replaceSettlementLines(ctx, tx, order.ID, input.Lines)
	if err != nil {
		return nil, err
	}
	order.Lines = lines
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit finance settlement order: %w", err)
	}
	return order, nil
}

func (r *PostgresRepository) ListSettlementOrders(ctx context.Context, limit int) ([]SettlementOrder, error) {
	rows, err := r.db.Query(ctx, settlementOrderSelectSQL()+`
		WHERE `+settlementOrderTenantPredicate("$1")+`
		ORDER BY created_at DESC LIMIT $2`, nullableUUID(currentTenantOrganizationID(ctx)), normalizeLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("list finance settlement orders: %w", err)
	}
	defer rows.Close()
	items := []SettlementOrder{}
	for rows.Next() {
		var item SettlementOrder
		if err := scanSettlementOrder(rows, &item); err != nil {
			return nil, fmt.Errorf("scan finance settlement order: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) GetSettlementOrder(ctx context.Context, id uuid.UUID) (*SettlementOrder, error) {
	order := &SettlementOrder{}
	if err := scanSettlementOrder(r.db.QueryRow(ctx, settlementOrderSelectSQL()+`
		WHERE id = $1 AND `+settlementOrderTenantPredicate("$2"), id, nullableUUID(currentTenantOrganizationID(ctx))), order); err != nil {
		return nil, fmt.Errorf("get finance settlement order: %w", err)
	}
	lines, err := r.listSettlementLines(ctx, r.db, id)
	if err != nil {
		return nil, err
	}
	order.Lines = lines
	return order, nil
}

func (r *PostgresRepository) UpdateSettlementOrder(ctx context.Context, id uuid.UUID, input UpdateSettlementOrderInput) (*SettlementOrder, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin update finance settlement order: %w", err)
	}
	defer tx.Rollback(ctx)
	subtotal, taxAmount := settlementTotals(input.Lines)
	var total any
	if len(input.Lines) > 0 {
		total = subtotal + taxAmount
	}
	order := &SettlementOrder{}
	err = scanSettlementOrder(tx.QueryRow(ctx, `
		UPDATE finance_settlement_orders
		SET settlement_number = COALESCE($2, settlement_number),
		    project_id = COALESCE($3, project_id),
		    requirement_id = COALESCE($4, requirement_id),
		    deliverable_id = COALESCE($5, deliverable_id),
		    customer_id = COALESCE($6, customer_id),
		    customer_name = COALESCE($7, customer_name),
		    title = COALESCE($8, title),
		    description = COALESCE($9, description),
		    subtotal = COALESCE($10, subtotal),
		    tax_amount = COALESCE($11, tax_amount),
		    total_amount = COALESCE($12, total_amount),
		    currency = COALESCE($13, currency),
		    settlement_date = COALESCE($14, settlement_date),
		    due_date = COALESCE($15, due_date),
		    status = COALESCE($16, status),
		    metadata = CASE WHEN $17::jsonb IS NULL THEN metadata ELSE $17::jsonb END,
		    updated_at = NOW()
		WHERE id = $1 AND status <> 'posted'
		RETURNING id, settlement_number, project_id, requirement_id, deliverable_id, customer_id,
		          customer_name, title, description, subtotal::float8, tax_amount::float8,
		          total_amount::float8, currency, settlement_date, due_date, status, receivable_id,
		          metadata, created_at, updated_at
	`, id, input.SettlementNumber, input.ProjectID, input.RequirementID, input.DeliverableID,
		input.CustomerID, input.CustomerName, input.Title, input.Description, nullableFloat(len(input.Lines), subtotal),
		nullableFloat(len(input.Lines), taxAmount), total, input.Currency, parseOptionalDateForSQL(input.SettlementDate),
		parseOptionalDateForSQL(input.DueDate), input.Status, nullableJSON(input.Metadata)), order)
	if err != nil {
		return nil, fmt.Errorf("update finance settlement order: %w", err)
	}
	if len(input.Lines) > 0 {
		lines, err := r.replaceSettlementLines(ctx, tx, id, input.Lines)
		if err != nil {
			return nil, err
		}
		order.Lines = lines
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit update finance settlement order: %w", err)
	}
	return order, nil
}

func (r *PostgresRepository) VoidSettlementOrder(ctx context.Context, id uuid.UUID, reason string) (*SettlementOrder, error) {
	order := &SettlementOrder{}
	err := scanSettlementOrder(r.db.QueryRow(ctx, settlementOrderSelectSQL()+`
		WHERE id = $1 AND status <> 'posted'
	`, id), order)
	if err != nil {
		return nil, fmt.Errorf("void finance settlement order read: %w", err)
	}
	err = scanSettlementOrder(r.db.QueryRow(ctx, `
		UPDATE finance_settlement_orders
		SET status = 'void',
		    metadata = metadata || $2::jsonb,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id, settlement_number, project_id, requirement_id, deliverable_id, customer_id,
		          customer_name, title, description, subtotal::float8, tax_amount::float8,
		          total_amount::float8, currency, settlement_date, due_date, status, receivable_id,
		          metadata, created_at, updated_at
	`, id, mustJSON(map[string]any{"void_reason": reason})), order)
	if err != nil {
		return nil, fmt.Errorf("void finance settlement order: %w", err)
	}
	return order, nil
}

func (r *PostgresRepository) PostSettlementOrder(ctx context.Context, id uuid.UUID) (*Receivable, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin post finance settlement order: %w", err)
	}
	defer tx.Rollback(ctx)
	order := &SettlementOrder{}
	if err := scanSettlementOrder(tx.QueryRow(ctx, settlementOrderSelectSQL()+` WHERE id = $1 FOR UPDATE`, id), order); err != nil {
		return nil, fmt.Errorf("get finance settlement order for posting: %w", err)
	}
	if order.ReceivableID != nil {
		receivable := &Receivable{}
		if err := scanReceivable(tx.QueryRow(ctx, receivableSelectSQL()+` WHERE id = $1`, *order.ReceivableID), receivable); err != nil {
			return nil, fmt.Errorf("get existing finance receivable: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit idempotent settlement post: %w", err)
		}
		return receivable, nil
	}
	lines, err := r.listSettlementLines(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	receivable := &Receivable{}
	err = scanReceivable(tx.QueryRow(ctx, `
		INSERT INTO finance_receivables (
			receivable_type, settlement_order_id, source_type, source_id, customer_id, customer_name,
			project_id, requirement_id, amount, tax_amount, currency, due_date, status, metadata
		)
		VALUES ('project', $1, 'settlement_order', $1, $2, $3, $4, $5, $6, $7, $8, $9, 'unpaid', $10)
		RETURNING id, receivable_type, settlement_order_id, source_type, source_id,
		          external_receivable_id, invoice_number, customer_id, customer_name, project_id,
		          requirement_id, organization_id, department_id, account_code, account_name,
		          amount::float8, tax_amount::float8, currency, period_start, period_end,
		          invoice_date, due_date, status, received_amount::float8, metadata, created_at, updated_at
	`, order.ID, order.CustomerID, order.CustomerName, order.ProjectID, order.RequirementID,
		order.TotalAmount, order.TaxAmount, order.Currency, order.DueDate,
		mustJSON(map[string]any{"posted_from": "settlement_order"})), receivable)
	if err != nil {
		return nil, fmt.Errorf("create receivable from settlement: %w", err)
	}
	for _, line := range lines {
		if _, err := tx.Exec(ctx, `
			INSERT INTO finance_receivable_lines (
				receivable_id, settlement_line_id, line_type, description, amount, tax_amount, total_amount, metadata
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, receivable.ID, line.ID, line.LineType, line.Description, line.Amount, line.TaxAmount, line.TotalAmount, mustJSON(line.Metadata)); err != nil {
			return nil, fmt.Errorf("create receivable line from settlement: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, `
		UPDATE finance_settlement_orders
		SET status = 'posted', receivable_id = $2, updated_at = NOW()
		WHERE id = $1
	`, order.ID, receivable.ID); err != nil {
		return nil, fmt.Errorf("mark settlement posted: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit settlement posting: %w", err)
	}
	return receivable, nil
}

func (r *PostgresRepository) CreateReceivable(ctx context.Context, input CreateReceivableInput, dates financeExpenseDates) (*Receivable, error) {
	receivable := &Receivable{}
	err := scanReceivable(r.db.QueryRow(ctx, receivableInsertSQL()+`
		RETURNING id, receivable_type, settlement_order_id, source_type, source_id,
		          external_receivable_id, invoice_number, customer_id, customer_name, project_id,
		          requirement_id, organization_id, department_id, account_code, account_name,
		          amount::float8, tax_amount::float8, currency, period_start, period_end,
		          invoice_date, due_date, status, received_amount::float8, metadata, created_at, updated_at
	`, receivableInsertArgs(input, dates)...), receivable)
	if err != nil {
		return nil, fmt.Errorf("create finance receivable: %w", err)
	}
	return receivable, nil
}

func (r *PostgresRepository) ListReceivables(ctx context.Context, limit int) ([]Receivable, error) {
	rows, err := r.db.Query(ctx, receivableSelectSQL()+`
		WHERE ($1::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $1)
		ORDER BY created_at DESC LIMIT $2`, nullableUUID(currentTenantOrganizationID(ctx)), normalizeLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("list finance receivables: %w", err)
	}
	defer rows.Close()
	items := []Receivable{}
	for rows.Next() {
		var item Receivable
		if err := scanReceivable(rows, &item); err != nil {
			return nil, fmt.Errorf("scan finance receivable: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) FindReceivableBySource(ctx context.Context, sourceType string, sourceID uuid.UUID) (*Receivable, error) {
	receivable := &Receivable{}
	err := scanReceivable(r.db.QueryRow(ctx, receivableSelectSQL()+`
		WHERE source_type = $1
		  AND source_id = $2
		  AND ($3::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $3)
		ORDER BY created_at DESC
		LIMIT 1`, sourceType, sourceID, nullableUUID(currentTenantOrganizationID(ctx))), receivable)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find finance receivable by source: %w", err)
	}
	return receivable, nil
}

func (r *PostgresRepository) GetReceivable(ctx context.Context, id uuid.UUID) (*Receivable, error) {
	receivable := &Receivable{}
	if err := scanReceivable(r.db.QueryRow(ctx, receivableSelectSQL()+`
		WHERE id = $1 AND ($2::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $2)`,
		id, nullableUUID(currentTenantOrganizationID(ctx))), receivable); err != nil {
		return nil, fmt.Errorf("get finance receivable: %w", err)
	}
	return receivable, nil
}

func (r *PostgresRepository) UpdateReceivable(ctx context.Context, id uuid.UUID, input UpdateReceivableInput, dates financeExpenseDates) (*Receivable, error) {
	update := CreateReceivableInput(input)
	receivable := &Receivable{}
	err := scanReceivable(r.db.QueryRow(ctx, `
		UPDATE finance_receivables
		SET receivable_type = COALESCE(NULLIF($2, ''), receivable_type),
		    external_receivable_id = COALESCE(NULLIF($3, ''), external_receivable_id),
		    invoice_number = COALESCE(NULLIF($4, ''), invoice_number),
		    customer_id = COALESCE(NULLIF($5, ''), customer_id),
		    customer_name = COALESCE(NULLIF($6, ''), customer_name),
		    amount = CASE WHEN $7::numeric = 0 THEN amount ELSE $7 END,
		    tax_amount = $8,
		    currency = COALESCE(NULLIF($9, ''), currency),
		    period_start = COALESCE($10, period_start),
		    period_end = COALESCE($11, period_end),
		    invoice_date = COALESCE($12, invoice_date),
		    due_date = COALESCE($13, due_date),
		    status = COALESCE(NULLIF($14, ''), status),
		    metadata = CASE WHEN $15::jsonb IS NULL THEN metadata ELSE $15::jsonb END,
		    updated_at = NOW()
		WHERE id = $1 AND status <> 'paid'
			AND ($16::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $16)
		RETURNING id, receivable_type, settlement_order_id, source_type, source_id,
		          external_receivable_id, invoice_number, customer_id, customer_name, project_id,
		          requirement_id, organization_id, department_id, account_code, account_name,
		          amount::float8, tax_amount::float8, currency, period_start, period_end,
		          invoice_date, due_date, status, received_amount::float8, metadata, created_at, updated_at
	`, id, update.ReceivableType, update.ExternalReceivableID, update.InvoiceNumber, update.CustomerID,
		update.CustomerName, update.Amount, update.TaxAmount, update.Currency, dates.PeriodStart,
		dates.PeriodEnd, dates.InvoiceDate, dates.PaymentDueDate, update.Status, nullableJSON(update.Metadata),
		nullableUUID(currentTenantOrganizationID(ctx))), receivable)
	if err != nil {
		return nil, fmt.Errorf("update finance receivable: %w", err)
	}
	return receivable, nil
}

func (r *PostgresRepository) UpdateReceivableStatus(ctx context.Context, id uuid.UUID, status string) (*Receivable, error) {
	receivable := &Receivable{}
	err := scanReceivable(r.db.QueryRow(ctx, `
		UPDATE finance_receivables
		SET status = $2, updated_at = NOW()
		WHERE id = $1 AND ($3::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $3)
		RETURNING id, receivable_type, settlement_order_id, source_type, source_id,
		          external_receivable_id, invoice_number, customer_id, customer_name, project_id,
		          requirement_id, organization_id, department_id, account_code, account_name,
		          amount::float8, tax_amount::float8, currency, period_start, period_end,
		          invoice_date, due_date, status, received_amount::float8, metadata, created_at, updated_at
	`, id, status, nullableUUID(currentTenantOrganizationID(ctx))), receivable)
	if err != nil {
		return nil, fmt.Errorf("update finance receivable status: %w", err)
	}
	return receivable, nil
}

func (r *PostgresRepository) VoidReceivable(ctx context.Context, id uuid.UUID, reason string) (*Receivable, error) {
	receivable := &Receivable{}
	err := scanReceivable(r.db.QueryRow(ctx, `
		UPDATE finance_receivables
		SET status = 'void',
		    metadata = metadata || $2::jsonb,
		    updated_at = NOW()
		WHERE id = $1 AND received_amount = 0
			AND ($3::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $3)
		RETURNING id, receivable_type, settlement_order_id, source_type, source_id,
		          external_receivable_id, invoice_number, customer_id, customer_name, project_id,
		          requirement_id, organization_id, department_id, account_code, account_name,
		          amount::float8, tax_amount::float8, currency, period_start, period_end,
		          invoice_date, due_date, status, received_amount::float8, metadata, created_at, updated_at
	`, id, mustJSON(map[string]any{"void_reason": reason}), nullableUUID(currentTenantOrganizationID(ctx))), receivable)
	if err != nil {
		return nil, fmt.Errorf("void finance receivable: %w", err)
	}
	return receivable, nil
}

func (r *PostgresRepository) CreateReceipt(ctx context.Context, input CreateReceiptInput, receivedAt *time.Time) (*Receipt, error) {
	receipt := &Receipt{}
	err := scanReceipt(r.db.QueryRow(ctx, `
		INSERT INTO finance_receipts (
			receipt_number, external_receipt_id, payment_method, payer_account, receiver_account,
			customer_id, customer_name, amount, currency, received_at, status, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, receipt_number, external_receipt_id, payment_method, payer_account,
		          receiver_account, customer_id, customer_name, amount::float8, currency,
		          received_at, status, metadata, created_at, updated_at
	`, input.ReceiptNumber, input.ExternalReceiptID, input.PaymentMethod, input.PayerAccount,
		input.ReceiverAccount, input.CustomerID, input.CustomerName, input.Amount, input.Currency,
		receivedAt, input.Status, mustJSON(input.Metadata)), receipt)
	if err != nil {
		return nil, fmt.Errorf("create finance receipt: %w", err)
	}
	return receipt, nil
}

func (r *PostgresRepository) ListReceipts(ctx context.Context, limit int) ([]Receipt, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, receipt_number, external_receipt_id, payment_method, payer_account,
		       receiver_account, customer_id, customer_name, amount::float8, currency,
		       received_at, status, metadata, created_at, updated_at
		FROM finance_receipts
		ORDER BY created_at DESC
		LIMIT $1
	`, normalizeLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("list finance receipts: %w", err)
	}
	defer rows.Close()
	items := []Receipt{}
	for rows.Next() {
		var item Receipt
		if err := scanReceipt(rows, &item); err != nil {
			return nil, fmt.Errorf("scan finance receipt: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) AllocateReceipt(ctx context.Context, receiptID uuid.UUID, input AllocateReceiptInput) (*ReceiptAllocation, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin finance receipt allocation: %w", err)
	}
	defer tx.Rollback(ctx)
	allocation := &ReceiptAllocation{}
	err = scanReceiptAllocation(tx.QueryRow(ctx, `
		INSERT INTO finance_receipt_allocations(receipt_id, receivable_id, amount, currency, metadata)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, receipt_id, receivable_id, amount::float8, currency, metadata, created_at
	`, receiptID, input.ReceivableID, input.Amount, input.Currency, mustJSON(input.Metadata)), allocation)
	if err != nil {
		return nil, fmt.Errorf("create finance receipt allocation: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE finance_receivables
		SET received_amount = received_amount + $2,
		    status = CASE
		        WHEN received_amount + $2 >= amount THEN 'paid'
		        WHEN received_amount + $2 > 0 THEN 'partially_received'
		        ELSE status
		    END,
		    updated_at = NOW()
		WHERE id = $1
	`, input.ReceivableID, input.Amount); err != nil {
		return nil, fmt.Errorf("update receivable received amount: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE finance_receipts
		SET status = 'allocated', updated_at = NOW()
		WHERE id = $1
	`, receiptID); err != nil {
		return nil, fmt.Errorf("mark receipt allocated: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit finance receipt allocation: %w", err)
	}
	return allocation, nil
}

func (r *PostgresRepository) GetPayable(ctx context.Context, id uuid.UUID) (*Payable, error) {
	payable := &Payable{}
	if err := scanPayable(r.db.QueryRow(ctx, payableSelectSQL()+`
		WHERE id = $1 AND ($2::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $2)`,
		id, nullableUUID(currentTenantOrganizationID(ctx))), payable); err != nil {
		return nil, fmt.Errorf("get finance payable: %w", err)
	}
	return payable, nil
}

func (r *PostgresRepository) UpdatePayable(ctx context.Context, id uuid.UUID, input UpdatePayableInput, dates financeExpenseDates) (*Payable, error) {
	update := CreatePayableInput(input)
	payable := &Payable{}
	err := scanPayable(r.db.QueryRow(ctx, `
		UPDATE finance_payables
		SET payable_type = COALESCE(NULLIF($2, ''), payable_type),
		    external_payable_id = COALESCE(NULLIF($3, ''), external_payable_id),
		    invoice_number = COALESCE(NULLIF($4, ''), invoice_number),
		    vendor_id = COALESCE(NULLIF($5, ''), vendor_id),
		    vendor_name = COALESCE(NULLIF($6, ''), vendor_name),
		    employee_id = COALESCE(NULLIF($7, ''), employee_id),
		    employee_name = COALESCE(NULLIF($8, ''), employee_name),
		    amount = CASE WHEN $9::numeric = 0 THEN amount ELSE $9 END,
		    tax_amount = $10,
		    currency = COALESCE(NULLIF($11, ''), currency),
		    period_start = COALESCE($12, period_start),
		    period_end = COALESCE($13, period_end),
		    invoice_date = COALESCE($14, invoice_date),
		    due_date = COALESCE($15, due_date),
		    status = COALESCE(NULLIF($16, ''), status),
		    metadata = CASE WHEN $17::jsonb IS NULL THEN metadata ELSE $17::jsonb END,
		    updated_at = NOW()
		WHERE id = $1 AND paid_amount = 0
			AND ($18::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $18)
		RETURNING id, payable_type, source_type, source_id, external_payable_id, invoice_number,
		          vendor_id, vendor_name, employee_id, employee_name, agent_id, project_id,
		          organization_id, department_id, account_code, account_name, cost_center_code,
		          cost_center_name, amount::float8, tax_amount::float8, currency, period_start,
		          period_end, invoice_date, due_date, status, paid_amount::float8, metadata,
		          created_at, updated_at
	`, id, update.PayableType, update.ExternalPayableID, update.InvoiceNumber, update.VendorID,
		update.VendorName, update.EmployeeID, update.EmployeeName, update.Amount, update.TaxAmount,
		update.Currency, dates.PeriodStart, dates.PeriodEnd, dates.InvoiceDate, dates.PaymentDueDate,
		update.Status, nullableJSON(update.Metadata), nullableUUID(currentTenantOrganizationID(ctx))), payable)
	if err != nil {
		return nil, fmt.Errorf("update finance payable: %w", err)
	}
	return payable, nil
}

func (r *PostgresRepository) VoidPayable(ctx context.Context, id uuid.UUID, reason string) (*Payable, error) {
	payable := &Payable{}
	err := scanPayable(r.db.QueryRow(ctx, `
		UPDATE finance_payables
		SET status = 'void', void_reason = $2, updated_at = NOW()
		WHERE id = $1 AND paid_amount = 0
			AND ($3::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $3)
		RETURNING id, payable_type, source_type, source_id, external_payable_id, invoice_number,
		          vendor_id, vendor_name, employee_id, employee_name, agent_id, project_id,
		          organization_id, department_id, account_code, account_name, cost_center_code,
		          cost_center_name, amount::float8, tax_amount::float8, currency, period_start,
		          period_end, invoice_date, due_date, status, paid_amount::float8, metadata,
		          created_at, updated_at
	`, id, reason, nullableUUID(currentTenantOrganizationID(ctx))), payable)
	if err != nil {
		return nil, fmt.Errorf("void finance payable: %w", err)
	}
	return payable, nil
}

func (r *PostgresRepository) GetPayment(ctx context.Context, id uuid.UUID) (*Payment, error) {
	payment := &Payment{}
	if err := scanPayment(r.db.QueryRow(ctx, paymentSelectSQL()+` WHERE id = $1`, id), payment); err != nil {
		return nil, fmt.Errorf("get finance payment: %w", err)
	}
	return payment, nil
}

func (r *PostgresRepository) UpdatePayment(ctx context.Context, id uuid.UUID, input UpdatePaymentInput, paidAt *time.Time) (*Payment, error) {
	update := CreatePaymentInput(input)
	payment := &Payment{}
	err := scanPayment(r.db.QueryRow(ctx, `
		UPDATE finance_payments
		SET payment_number = COALESCE(NULLIF($2, ''), payment_number),
		    external_payment_id = COALESCE(NULLIF($3, ''), external_payment_id),
		    payment_method = COALESCE(NULLIF($4, ''), payment_method),
		    payer_account = COALESCE(NULLIF($5, ''), payer_account),
		    payee_account = COALESCE(NULLIF($6, ''), payee_account),
		    vendor_id = COALESCE(NULLIF($7, ''), vendor_id),
		    vendor_name = COALESCE(NULLIF($8, ''), vendor_name),
		    employee_id = COALESCE(NULLIF($9, ''), employee_id),
		    employee_name = COALESCE(NULLIF($10, ''), employee_name),
		    amount = CASE WHEN $11::numeric = 0 THEN amount ELSE $11 END,
		    currency = COALESCE(NULLIF($12, ''), currency),
		    paid_at = COALESCE($13, paid_at),
		    status = COALESCE(NULLIF($14, ''), status),
		    metadata = CASE WHEN $15::jsonb IS NULL THEN metadata ELSE $15::jsonb END,
		    updated_at = NOW()
		WHERE id = $1 AND status <> 'allocated'
		RETURNING id, payment_number, external_payment_id, payment_method, payer_account, payee_account,
		          vendor_id, vendor_name, employee_id, employee_name, amount::float8, currency,
		          paid_at, status, metadata, created_at, updated_at
	`, id, update.PaymentNumber, update.ExternalPaymentID, update.PaymentMethod, update.PayerAccount,
		update.PayeeAccount, update.VendorID, update.VendorName, update.EmployeeID, update.EmployeeName,
		update.Amount, update.Currency, paidAt, update.Status, nullableJSON(update.Metadata)), payment)
	if err != nil {
		return nil, fmt.Errorf("update finance payment: %w", err)
	}
	return payment, nil
}

func (r *PostgresRepository) VoidPayment(ctx context.Context, id uuid.UUID, reason string) (*Payment, error) {
	payment := &Payment{}
	err := scanPayment(r.db.QueryRow(ctx, `
		UPDATE finance_payments
		SET status = 'void', void_reason = $2, updated_at = NOW()
		WHERE id = $1 AND status <> 'allocated'
		RETURNING id, payment_number, external_payment_id, payment_method, payer_account, payee_account,
		          vendor_id, vendor_name, employee_id, employee_name, amount::float8, currency,
		          paid_at, status, metadata, created_at, updated_at
	`, id, reason), payment)
	if err != nil {
		return nil, fmt.Errorf("void finance payment: %w", err)
	}
	return payment, nil
}

type batchStore interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (r *PostgresRepository) getExportBatch(ctx context.Context, store batchStore, id uuid.UUID) (*ExportBatch, error) {
	batch := &ExportBatch{}
	err := scanBatch(store.QueryRow(ctx, `
		SELECT id, adapter_id, period_start, period_end, status, currency,
			total_amount::float8, external_batch_id, error_message, idempotency_key,
			metadata, created_at, submitted_at, updated_at
		FROM finance_export_batches
		WHERE id = $1 AND ($2::uuid IS NULL OR EXISTS (
			SELECT 1 FROM finance_export_lines l
			WHERE l.batch_id = finance_export_batches.id AND l.organization_id IS NOT DISTINCT FROM $2
		))
	`, id, nullableUUID(currentTenantOrganizationID(ctx))), batch)
	if err != nil {
		return nil, fmt.Errorf("get finance export batch: %w", err)
	}
	lines, err := r.listBatchLines(ctx, store, id)
	if err != nil {
		return nil, err
	}
	batch.Lines = lines
	return batch, nil
}

func (r *PostgresRepository) listBatchLines(ctx context.Context, store batchStore, batchID uuid.UUID) ([]ExportLine, error) {
	rows, err := store.Query(ctx, `
		SELECT id, batch_id, usage_ledger_id, cost_ledger_entry_id, project_cost_entry_id, organization_id,
			department_id, project_id, provider_id, model_id, amount::float8, currency,
			external_line_id, status, metadata, created_at
		FROM finance_export_lines
		WHERE batch_id = $1 AND ($2::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $2)
		ORDER BY created_at ASC
	`, batchID, nullableUUID(currentTenantOrganizationID(ctx)))
	if err != nil {
		return nil, fmt.Errorf("list finance export lines: %w", err)
	}
	defer rows.Close()
	lines := []ExportLine{}
	for rows.Next() {
		var line ExportLine
		if err := scanLine(rows, &line); err != nil {
			return nil, fmt.Errorf("scan finance export line: %w", err)
		}
		lines = append(lines, line)
	}
	return lines, rows.Err()
}

func (r *PostgresRepository) findBatchByIdempotencyKey(ctx context.Context, tx pgx.Tx, key string) (*uuid.UUID, error) {
	var id uuid.UUID
	err := tx.QueryRow(ctx, `SELECT id FROM finance_export_batches WHERE idempotency_key = $1`, key).Scan(&id)
	if errorsIsNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find finance export batch by idempotency key: %w", err)
	}
	return &id, nil
}

func (r *PostgresRepository) findImportRecord(ctx context.Context, adapterID uuid.UUID, externalRecordID string) (*ImportRecord, error) {
	record := &ImportRecord{}
	err := scanImportRecord(r.db.QueryRow(ctx, `
		SELECT id, batch_id, adapter_id, external_record_id, expense_type, raw_payload,
		       normalized_payload, cost_ledger_entry_id, payable_id, status, error_message,
		       metadata, created_at
		FROM finance_import_records
		WHERE adapter_id = $1 AND external_record_id = $2
	`, adapterID, externalRecordID), record)
	if errorsIsNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find finance import record: %w", err)
	}
	return record, nil
}

type payableStore interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (r *PostgresRepository) createPayable(ctx context.Context, store payableStore, input CreatePayableInput, dates financeExpenseDates) (*Payable, error) {
	payable := &Payable{}
	err := scanPayable(store.QueryRow(ctx, `
		INSERT INTO finance_payables (
			payable_type, source_type, source_id, external_payable_id, invoice_number,
			vendor_id, vendor_name, employee_id, employee_name, agent_id, project_id,
			organization_id, department_id, account_code, account_name, cost_center_code,
			cost_center_name, amount, tax_amount, currency, period_start, period_end,
			invoice_date, due_date, status, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13,
		        $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26)
		RETURNING id, payable_type, source_type, source_id, external_payable_id, invoice_number,
		          vendor_id, vendor_name, employee_id, employee_name, agent_id, project_id,
		          organization_id, department_id, account_code, account_name, cost_center_code,
		          cost_center_name, amount::float8, tax_amount::float8, currency, period_start,
		          period_end, invoice_date, due_date, status, paid_amount::float8, metadata,
		          created_at, updated_at
	`, input.PayableType, input.SourceType, input.SourceID, input.ExternalPayableID, input.InvoiceNumber,
		input.VendorID, input.VendorName, input.EmployeeID, input.EmployeeName, input.AgentID, input.ProjectID,
		input.OrganizationID, input.DepartmentID, input.AccountCode, input.AccountName, input.CostCenterCode,
		input.CostCenterName, input.Amount, input.TaxAmount, input.Currency, dates.PeriodStart, dates.PeriodEnd,
		dates.InvoiceDate, dates.PaymentDueDate, input.Status, mustJSON(input.Metadata)), payable)
	if err != nil {
		return nil, fmt.Errorf("create finance payable: %w", err)
	}
	return payable, nil
}

func payableSelectSQL() string {
	return `SELECT id, payable_type, source_type, source_id, external_payable_id, invoice_number,
		          vendor_id, vendor_name, employee_id, employee_name, agent_id, project_id,
		          organization_id, department_id, account_code, account_name, cost_center_code,
		          cost_center_name, amount::float8, tax_amount::float8, currency, period_start,
		          period_end, invoice_date, due_date, status, paid_amount::float8, metadata,
		          created_at, updated_at
	        FROM finance_payables`
}

func paymentSelectSQL() string {
	return `SELECT id, payment_number, external_payment_id, payment_method, payer_account, payee_account,
		          vendor_id, vendor_name, employee_id, employee_name, amount::float8, currency,
		          paid_at, status, metadata, created_at, updated_at
	        FROM finance_payments`
}

func settlementOrderSelectSQL() string {
	return `SELECT id, settlement_number, project_id, requirement_id, deliverable_id, customer_id,
		          customer_name, title, description, subtotal::float8, tax_amount::float8,
		          total_amount::float8, currency, settlement_date, due_date, status, receivable_id,
		          metadata, created_at, updated_at
	        FROM finance_settlement_orders`
}

func receivableSelectSQL() string {
	return `SELECT id, receivable_type, settlement_order_id, source_type, source_id,
		          external_receivable_id, invoice_number, customer_id, customer_name, project_id,
		          requirement_id, organization_id, department_id, account_code, account_name,
		          amount::float8, tax_amount::float8, currency, period_start, period_end,
		          invoice_date, due_date, status, received_amount::float8, metadata, created_at, updated_at
	        FROM finance_receivables`
}

func receivableInsertSQL() string {
	return `INSERT INTO finance_receivables (
			receivable_type, settlement_order_id, source_type, source_id, external_receivable_id,
			invoice_number, customer_id, customer_name, project_id, requirement_id, organization_id,
			department_id, account_code, account_name, amount, tax_amount, currency, period_start,
			period_end, invoice_date, due_date, status, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13,
		        $14, $15, $16, $17, $18, $19, $20, $21, $22, $23)`
}

func receivableInsertArgs(input CreateReceivableInput, dates financeExpenseDates) []any {
	return []any{
		input.ReceivableType, input.SettlementOrderID, input.SourceType, input.SourceID,
		input.ExternalReceivableID, input.InvoiceNumber, input.CustomerID, input.CustomerName,
		input.ProjectID, input.RequirementID, input.OrganizationID, input.DepartmentID,
		input.AccountCode, input.AccountName, input.Amount, input.TaxAmount, input.Currency,
		dates.PeriodStart, dates.PeriodEnd, dates.InvoiceDate, dates.PaymentDueDate,
		input.Status, mustJSON(input.Metadata),
	}
}

type settlementStore interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (r *PostgresRepository) replaceSettlementLines(ctx context.Context, store settlementStore, orderID uuid.UUID, inputs []CreateSettlementLineInput) ([]SettlementLine, error) {
	if _, err := store.Exec(ctx, `DELETE FROM finance_settlement_lines WHERE settlement_order_id = $1`, orderID); err != nil {
		return nil, fmt.Errorf("clear finance settlement lines: %w", err)
	}
	lines := []SettlementLine{}
	for _, input := range inputs {
		total := input.Amount + input.TaxAmount
		line := SettlementLine{}
		err := scanSettlementLine(store.QueryRow(ctx, `
			INSERT INTO finance_settlement_lines (
				settlement_order_id, line_type, source_type, source_id, deliverable_id, description,
				quantity, unit_price, amount, tax_amount, total_amount, metadata
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			RETURNING id, settlement_order_id, line_type, source_type, source_id, deliverable_id,
			          description, quantity::float8, unit_price::float8, amount::float8,
			          tax_amount::float8, total_amount::float8, metadata, created_at
		`, orderID, input.LineType, input.SourceType, input.SourceID, input.DeliverableID,
			input.Description, input.Quantity, input.UnitPrice, input.Amount, input.TaxAmount,
			total, mustJSON(input.Metadata)), &line)
		if err != nil {
			return nil, fmt.Errorf("create finance settlement line: %w", err)
		}
		lines = append(lines, line)
	}
	return lines, nil
}

func (r *PostgresRepository) listSettlementLines(ctx context.Context, store batchStore, orderID uuid.UUID) ([]SettlementLine, error) {
	rows, err := store.Query(ctx, `
		SELECT id, settlement_order_id, line_type, source_type, source_id, deliverable_id,
		       description, quantity::float8, unit_price::float8, amount::float8,
		       tax_amount::float8, total_amount::float8, metadata, created_at
		FROM finance_settlement_lines
		WHERE settlement_order_id = $1
		ORDER BY created_at ASC
	`, orderID)
	if err != nil {
		return nil, fmt.Errorf("list finance settlement lines: %w", err)
	}
	defer rows.Close()
	lines := []SettlementLine{}
	for rows.Next() {
		var line SettlementLine
		if err := scanSettlementLine(rows, &line); err != nil {
			return nil, fmt.Errorf("scan finance settlement line: %w", err)
		}
		lines = append(lines, line)
	}
	return lines, rows.Err()
}

type exportableCostRow struct {
	UsageLedgerID      *uuid.UUID
	CostLedgerEntryID  uuid.UUID
	ProjectCostEntryID *uuid.UUID
	OrganizationID     *uuid.UUID
	DepartmentID       *uuid.UUID
	ProjectID          *uuid.UUID
	ProviderID         *uuid.UUID
	ModelID            *uuid.UUID
	Amount             float64
	Currency           string
	SourceType         string
	CostCreatedAt      time.Time
}

func (r *PostgresRepository) exportableCostRows(ctx context.Context, tx pgx.Tx, start time.Time, end time.Time, currency string) ([]exportableCostRow, error) {
	rows, err := tx.Query(ctx, `
		SELECT c.id,
			u.id,
			CASE WHEN c.source_type = 'project_cost_entry' THEN c.source_id ELSE NULL END AS project_cost_entry_id,
			c.organization_id, c.department_id, c.project_id,
			i.provider_id, i.model_id, c.amount::float8, c.currency, c.source_type, c.created_at
		FROM cost_ledger_entries c
		LEFT JOIN ai_invocations i ON c.source_type = 'ai_invocation' AND i.id = c.source_id
		LEFT JOIN LATERAL (
			SELECT id
			FROM ai_usage_ledger
			WHERE invocation_id = i.id
			ORDER BY created_at ASC
			LIMIT 1
		) u ON TRUE
		WHERE c.finance_export_line_id IS NULL
			AND c.status = 'posted'
			AND c.ledger_type IN ('actual', 'adjustment')
			AND c.amount <> 0
			AND c.currency = $3
			AND ($4::uuid IS NULL OR c.organization_id IS NOT DISTINCT FROM $4)
			AND c.occurred_at >= $1
			AND c.occurred_at < ($2::date + INTERVAL '1 day')
		ORDER BY c.occurred_at ASC, c.created_at ASC
		FOR UPDATE OF c SKIP LOCKED
	`, start, end, currency, nullableUUID(currentTenantOrganizationID(ctx)))
	if err != nil {
		return nil, fmt.Errorf("query exportable cost ledger: %w", err)
	}
	defer rows.Close()
	items := []exportableCostRow{}
	for rows.Next() {
		var item exportableCostRow
		var usageLedgerID, projectCostEntryID, organizationID, departmentID, projectID, providerID, modelID pgtype.UUID
		if err := rows.Scan(&item.CostLedgerEntryID, &usageLedgerID, &projectCostEntryID, &organizationID, &departmentID,
			&projectID, &providerID, &modelID, &item.Amount, &item.Currency, &item.SourceType, &item.CostCreatedAt); err != nil {
			return nil, fmt.Errorf("scan exportable cost ledger: %w", err)
		}
		item.UsageLedgerID = uuidPtr(usageLedgerID)
		item.ProjectCostEntryID = uuidPtr(projectCostEntryID)
		item.OrganizationID = uuidPtr(organizationID)
		item.DepartmentID = uuidPtr(departmentID)
		item.ProjectID = uuidPtr(projectID)
		item.ProviderID = uuidPtr(providerID)
		item.ModelID = uuidPtr(modelID)
		items = append(items, item)
	}
	return items, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanImportBatch(row scanner, batch *ImportBatch) error {
	var metadataJSON []byte
	var adapterID pgtype.UUID
	if err := row.Scan(&batch.ID, &adapterID, &batch.SourceType, &batch.FileName, &batch.Status,
		&batch.TotalRecords, &batch.ProcessedRecords, &batch.FailedRecords, &metadataJSON,
		&batch.CreatedAt, &batch.CompletedAt); err != nil {
		return err
	}
	batch.AdapterID = uuidPtr(adapterID)
	return json.Unmarshal(metadataJSON, &batch.Metadata)
}

func scanImportRecord(row scanner, record *ImportRecord) error {
	var rawJSON, normalizedJSON, metadataJSON []byte
	var adapterID, ledgerID, payableID pgtype.UUID
	if err := row.Scan(&record.ID, &record.BatchID, &adapterID, &record.ExternalRecordID,
		&record.ExpenseType, &rawJSON, &normalizedJSON, &ledgerID, &payableID, &record.Status,
		&record.ErrorMessage, &metadataJSON, &record.CreatedAt); err != nil {
		return err
	}
	record.AdapterID = uuidPtr(adapterID)
	record.CostLedgerEntryID = uuidPtr(ledgerID)
	record.PayableID = uuidPtr(payableID)
	_ = json.Unmarshal(rawJSON, &record.RawPayload)
	_ = json.Unmarshal(normalizedJSON, &record.NormalizedPayload)
	return json.Unmarshal(metadataJSON, &record.Metadata)
}

func scanSettlementOrder(row scanner, order *SettlementOrder) error {
	var metadataJSON []byte
	var projectID, requirementID, deliverableID, receivableID pgtype.UUID
	if err := row.Scan(&order.ID, &order.SettlementNumber, &projectID, &requirementID,
		&deliverableID, &order.CustomerID, &order.CustomerName, &order.Title, &order.Description,
		&order.Subtotal, &order.TaxAmount, &order.TotalAmount, &order.Currency,
		&order.SettlementDate, &order.DueDate, &order.Status, &receivableID, &metadataJSON,
		&order.CreatedAt, &order.UpdatedAt); err != nil {
		return err
	}
	order.ProjectID = uuidPtr(projectID)
	order.RequirementID = uuidPtr(requirementID)
	order.DeliverableID = uuidPtr(deliverableID)
	order.ReceivableID = uuidPtr(receivableID)
	return json.Unmarshal(metadataJSON, &order.Metadata)
}

func scanSettlementLine(row scanner, line *SettlementLine) error {
	var metadataJSON []byte
	var sourceID, deliverableID pgtype.UUID
	if err := row.Scan(&line.ID, &line.SettlementOrderID, &line.LineType, &line.SourceType,
		&sourceID, &deliverableID, &line.Description, &line.Quantity, &line.UnitPrice,
		&line.Amount, &line.TaxAmount, &line.TotalAmount, &metadataJSON, &line.CreatedAt); err != nil {
		return err
	}
	line.SourceID = uuidPtr(sourceID)
	line.DeliverableID = uuidPtr(deliverableID)
	return json.Unmarshal(metadataJSON, &line.Metadata)
}

func scanReceivable(row scanner, receivable *Receivable) error {
	var metadataJSON []byte
	var settlementOrderID, sourceID, projectID, requirementID, organizationID, departmentID pgtype.UUID
	if err := row.Scan(&receivable.ID, &receivable.ReceivableType, &settlementOrderID,
		&receivable.SourceType, &sourceID, &receivable.ExternalReceivableID, &receivable.InvoiceNumber,
		&receivable.CustomerID, &receivable.CustomerName, &projectID, &requirementID, &organizationID,
		&departmentID, &receivable.AccountCode, &receivable.AccountName, &receivable.Amount,
		&receivable.TaxAmount, &receivable.Currency, &receivable.PeriodStart, &receivable.PeriodEnd,
		&receivable.InvoiceDate, &receivable.DueDate, &receivable.Status, &receivable.ReceivedAmount,
		&metadataJSON, &receivable.CreatedAt, &receivable.UpdatedAt); err != nil {
		return err
	}
	receivable.SettlementOrderID = uuidPtr(settlementOrderID)
	receivable.SourceID = uuidPtr(sourceID)
	receivable.ProjectID = uuidPtr(projectID)
	receivable.RequirementID = uuidPtr(requirementID)
	receivable.OrganizationID = uuidPtr(organizationID)
	receivable.DepartmentID = uuidPtr(departmentID)
	return json.Unmarshal(metadataJSON, &receivable.Metadata)
}

func scanPayable(row scanner, payable *Payable) error {
	var metadataJSON []byte
	var sourceID, agentID, projectID, organizationID, departmentID pgtype.UUID
	if err := row.Scan(&payable.ID, &payable.PayableType, &payable.SourceType, &sourceID,
		&payable.ExternalPayableID, &payable.InvoiceNumber, &payable.VendorID, &payable.VendorName,
		&payable.EmployeeID, &payable.EmployeeName, &agentID, &projectID, &organizationID,
		&departmentID, &payable.AccountCode, &payable.AccountName, &payable.CostCenterCode,
		&payable.CostCenterName, &payable.Amount, &payable.TaxAmount, &payable.Currency,
		&payable.PeriodStart, &payable.PeriodEnd, &payable.InvoiceDate, &payable.DueDate,
		&payable.Status, &payable.PaidAmount, &metadataJSON, &payable.CreatedAt, &payable.UpdatedAt); err != nil {
		return err
	}
	payable.SourceID = uuidPtr(sourceID)
	payable.AgentID = uuidPtr(agentID)
	payable.ProjectID = uuidPtr(projectID)
	payable.OrganizationID = uuidPtr(organizationID)
	payable.DepartmentID = uuidPtr(departmentID)
	return json.Unmarshal(metadataJSON, &payable.Metadata)
}

func scanPayment(row scanner, payment *Payment) error {
	var metadataJSON []byte
	if err := row.Scan(&payment.ID, &payment.PaymentNumber, &payment.ExternalPaymentID,
		&payment.PaymentMethod, &payment.PayerAccount, &payment.PayeeAccount, &payment.VendorID,
		&payment.VendorName, &payment.EmployeeID, &payment.EmployeeName, &payment.Amount,
		&payment.Currency, &payment.PaidAt, &payment.Status, &metadataJSON, &payment.CreatedAt,
		&payment.UpdatedAt); err != nil {
		return err
	}
	return json.Unmarshal(metadataJSON, &payment.Metadata)
}

func scanReceipt(row scanner, receipt *Receipt) error {
	var metadataJSON []byte
	if err := row.Scan(&receipt.ID, &receipt.ReceiptNumber, &receipt.ExternalReceiptID,
		&receipt.PaymentMethod, &receipt.PayerAccount, &receipt.ReceiverAccount,
		&receipt.CustomerID, &receipt.CustomerName, &receipt.Amount, &receipt.Currency,
		&receipt.ReceivedAt, &receipt.Status, &metadataJSON, &receipt.CreatedAt,
		&receipt.UpdatedAt); err != nil {
		return err
	}
	return json.Unmarshal(metadataJSON, &receipt.Metadata)
}

func scanPaymentAllocation(row scanner, allocation *PaymentAllocation) error {
	var metadataJSON []byte
	if err := row.Scan(&allocation.ID, &allocation.PaymentID, &allocation.PayableID,
		&allocation.Amount, &allocation.Currency, &metadataJSON, &allocation.CreatedAt); err != nil {
		return err
	}
	return json.Unmarshal(metadataJSON, &allocation.Metadata)
}

func scanReceiptAllocation(row scanner, allocation *ReceiptAllocation) error {
	var metadataJSON []byte
	if err := row.Scan(&allocation.ID, &allocation.ReceiptID, &allocation.ReceivableID,
		&allocation.Amount, &allocation.Currency, &metadataJSON, &allocation.CreatedAt); err != nil {
		return err
	}
	return json.Unmarshal(metadataJSON, &allocation.Metadata)
}

func scanAdapter(row scanner, adapter *FinanceAdapter) error {
	var metadataJSON []byte
	var fieldMappingJSON []byte
	var pullConfigJSON []byte
	if err := row.Scan(&adapter.ID, &adapter.Name, &adapter.EndpointURL, &adapter.AuthType,
		&adapter.AdapterType, &adapter.Direction, &adapter.MaskedSecret, &adapter.Status, &adapter.TimeoutMS, &adapter.RetryCount,
		&fieldMappingJSON, &pullConfigJSON, &adapter.LastSyncAt, &adapter.LastSyncStatus,
		&metadataJSON, &adapter.CreatedAt, &adapter.UpdatedAt); err != nil {
		return err
	}
	_ = json.Unmarshal(fieldMappingJSON, &adapter.FieldMapping)
	_ = json.Unmarshal(pullConfigJSON, &adapter.PullConfig)
	return json.Unmarshal(metadataJSON, &adapter.Metadata)
}

func scanBatch(row scanner, batch *ExportBatch) error {
	var metadataJSON []byte
	if err := row.Scan(&batch.ID, &batch.AdapterID, &batch.PeriodStart, &batch.PeriodEnd,
		&batch.Status, &batch.Currency, &batch.TotalAmount, &batch.ExternalBatchID,
		&batch.ErrorMessage, &batch.IdempotencyKey, &metadataJSON, &batch.CreatedAt,
		&batch.SubmittedAt, &batch.UpdatedAt); err != nil {
		return err
	}
	return json.Unmarshal(metadataJSON, &batch.Metadata)
}

func scanLine(row scanner, line *ExportLine) error {
	var metadataJSON []byte
	var usageLedgerID, costLedgerEntryID, projectCostEntryID, organizationID, departmentID, projectID, providerID, modelID pgtype.UUID
	if err := row.Scan(&line.ID, &line.BatchID, &usageLedgerID, &costLedgerEntryID, &projectCostEntryID, &organizationID,
		&departmentID, &projectID, &providerID, &modelID, &line.Amount, &line.Currency,
		&line.ExternalLineID, &line.Status, &metadataJSON, &line.CreatedAt); err != nil {
		return err
	}
	line.UsageLedgerID = uuidPtr(usageLedgerID)
	line.CostLedgerEntryID = uuidPtr(costLedgerEntryID)
	line.ProjectCostEntryID = uuidPtr(projectCostEntryID)
	line.OrganizationID = uuidPtr(organizationID)
	line.DepartmentID = uuidPtr(departmentID)
	line.ProjectID = uuidPtr(projectID)
	line.ProviderID = uuidPtr(providerID)
	line.ModelID = uuidPtr(modelID)
	return json.Unmarshal(metadataJSON, &line.Metadata)
}

func scanWebhookEvent(row scanner, event *WebhookEvent) error {
	var payloadJSON []byte
	var batchID pgtype.UUID
	if err := row.Scan(&event.ID, &event.AdapterID, &batchID, &event.EventType,
		&event.SignatureValid, &payloadJSON, &event.Processed, &event.ErrorMessage,
		&event.CreatedAt); err != nil {
		return err
	}
	event.BatchID = uuidPtr(batchID)
	return json.Unmarshal(payloadJSON, &event.Payload)
}

type financeExpenseDates struct {
	OccurredAt     *time.Time
	InvoiceDate    *time.Time
	PaymentDueDate *time.Time
	PaidAt         *time.Time
	PeriodStart    *time.Time
	PeriodEnd      *time.Time
}

func actorIDForExpense(input FinanceExpenseInput) *uuid.UUID {
	if input.AgentID != nil {
		return input.AgentID
	}
	return nil
}

func actorTypeForExpense(input FinanceExpenseInput) string {
	if input.AgentID != nil {
		return "agent"
	}
	if input.EmployeeID != "" {
		return "human"
	}
	return ""
}

func payableTypeForExpense(expenseType string) string {
	switch expenseType {
	case "salary":
		return "salary"
	case "project_expense":
		return "project"
	case "model_fee":
		return "model"
	case "agent_fee":
		return "agent"
	default:
		return "expense"
	}
}

func payableStatusFromPayment(status string) string {
	switch status {
	case "paid":
		return "paid"
	case "partially_paid":
		return "partially_paid"
	case "void":
		return "void"
	default:
		return "open"
	}
}

func expensePayload(input FinanceExpenseInput) map[string]any {
	return map[string]any{
		"external_record_id": input.ExternalRecordID,
		"expense_type":       input.ExpenseType,
		"cost_category":      input.CostCategory,
		"amount":             input.Amount,
		"currency":           input.Currency,
		"occurred_at":        input.OccurredAt,
		"description":        input.Description,
		"account_code":       input.AccountCode,
		"account_name":       input.AccountName,
		"cost_center_code":   input.CostCenterCode,
		"cost_center_name":   input.CostCenterName,
		"vendor_id":          input.VendorID,
		"vendor_name":        input.VendorName,
		"employee_id":        input.EmployeeID,
		"employee_name":      input.EmployeeName,
		"agent_id":           input.AgentID,
		"agent_name":         input.AgentName,
		"organization_id":    input.OrganizationID,
		"department_id":      input.DepartmentID,
		"requirement_id":     input.RequirementID,
		"project_id":         input.ProjectID,
		"workflow_id":        input.WorkflowID,
		"task_id":            input.TaskID,
		"capability_id":      input.CapabilityID,
		"tax_amount":         input.TaxAmount,
		"tax_rate":           input.TaxRate,
		"invoice_number":     input.InvoiceNumber,
		"invoice_date":       input.InvoiceDate,
		"payment_status":     input.PaymentStatus,
		"payment_due_date":   input.PaymentDueDate,
		"paid_at":            input.PaidAt,
		"period_start":       input.PeriodStart,
		"period_end":         input.PeriodEnd,
		"metadata":           input.Metadata,
	}
}

func settlementTotals(lines []CreateSettlementLineInput) (float64, float64) {
	var subtotal float64
	var taxAmount float64
	for _, line := range lines {
		subtotal += line.Amount
		taxAmount += line.TaxAmount
	}
	return subtotal, taxAmount
}

func parseDateForSQL(value string) any {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil
	}
	return parsed
}

func parseOptionalDateForSQL(value *string) any {
	if value == nil {
		return nil
	}
	return parseDateForSQL(*value)
}

func nullableFloat(count int, value float64) any {
	if count == 0 {
		return nil
	}
	return value
}

func uuidPtr(value pgtype.UUID) *uuid.UUID {
	if !value.Valid {
		return nil
	}
	id := uuidPtrValue(value)
	if id == uuid.Nil {
		return nil
	}
	return &id
}

func uuidPtrValue(value pgtype.UUID) uuid.UUID {
	if !value.Valid {
		return uuid.Nil
	}
	id, err := uuid.FromBytes(value.Bytes[:])
	if err != nil {
		return uuid.Nil
	}
	return id
}

func mustJSON(value map[string]any) []byte {
	if value == nil {
		value = map[string]any{}
	}
	data, err := json.Marshal(value)
	if err != nil {
		return []byte("{}")
	}
	return data
}

func nullableJSON(value map[string]any) any {
	if value == nil {
		return nil
	}
	return mustJSON(value)
}

func nullableUUID(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return *id
}

func settlementOrderTenantPredicate(param string) string {
	return fmt.Sprintf(`(%s::uuid IS NULL
			OR EXISTS (SELECT 1 FROM projects p WHERE p.id = finance_settlement_orders.project_id AND p.organization_id IS NOT DISTINCT FROM %s)
			OR EXISTS (SELECT 1 FROM requirements req WHERE req.id = finance_settlement_orders.requirement_id AND req.organization_id IS NOT DISTINCT FROM %s)
			OR EXISTS (
				SELECT 1
				FROM deliverables d
				JOIN projects p ON p.id = d.project_id
				WHERE d.id = finance_settlement_orders.deliverable_id AND p.organization_id IS NOT DISTINCT FROM %s
			))`, param, param, param, param)
}

func maskSecret(secret string) string {
	if len(secret) <= 8 {
		return "****"
	}
	return secret[:4] + "****" + secret[len(secret)-4:]
}

func errorsIsNoRows(err error) bool {
	return err == pgx.ErrNoRows
}
