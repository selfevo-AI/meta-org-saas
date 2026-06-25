package finance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/observability"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/project"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
)

var (
	ErrValidation = errors.New("validation error")
	ErrForbidden  = errors.New("forbidden")
	ErrNotFound   = errors.New("not found")
)

type Repository interface {
	CreateAdapter(ctx context.Context, input CreateAdapterInput) (*FinanceAdapter, error)
	ListAdapters(ctx context.Context, limit int) ([]FinanceAdapter, error)
	UpdateAdapter(ctx context.Context, id uuid.UUID, input UpdateAdapterInput) (*FinanceAdapter, error)
	GetAdapterSecret(ctx context.Context, id uuid.UUID) (AdapterSecret, error)
	CreateExportBatch(ctx context.Context, input CreateExportBatchInput) (*ExportBatch, error)
	ListExportBatches(ctx context.Context, limit int) ([]ExportBatch, error)
	GetExportBatch(ctx context.Context, id uuid.UUID) (*ExportBatch, error)
	UpdateExportBatchStatus(ctx context.Context, id uuid.UUID, input UpdateExportBatchStatusInput) (*ExportBatch, error)
	RecordWebhookEvent(ctx context.Context, input RecordWebhookEventInput) (*WebhookEvent, error)
	UpdateExportLineStatus(ctx context.Context, id uuid.UUID, input UpdateExportLineStatusInput) (*ExportLine, error)
	LinkProjectCostEntry(ctx context.Context, lineID uuid.UUID, entryID uuid.UUID) error
	ListReconciliation(ctx context.Context, limit int) ([]ReconciliationItem, error)
	CreateImportBatch(ctx context.Context, adapterID *uuid.UUID, sourceType string, fileName string, total int, metadata map[string]any) (*ImportBatch, error)
	CompleteImportBatch(ctx context.Context, id uuid.UUID, processed int, failed int) (*ImportBatch, error)
	ListImportBatches(ctx context.Context, limit int) ([]ImportBatch, error)
	ListImportRecords(ctx context.Context, limit int) ([]ImportRecord, error)
	CreateImportedExpense(ctx context.Context, batchID uuid.UUID, adapterID uuid.UUID, raw map[string]any, input FinanceExpenseInput, occurredAt time.Time, dates financeExpenseDates) (*ImportRecord, error)
	CreateSettlementOrder(ctx context.Context, input CreateSettlementOrderInput) (*SettlementOrder, error)
	ListSettlementOrders(ctx context.Context, limit int) ([]SettlementOrder, error)
	GetSettlementOrder(ctx context.Context, id uuid.UUID) (*SettlementOrder, error)
	UpdateSettlementOrder(ctx context.Context, id uuid.UUID, input UpdateSettlementOrderInput) (*SettlementOrder, error)
	VoidSettlementOrder(ctx context.Context, id uuid.UUID, reason string) (*SettlementOrder, error)
	PostSettlementOrder(ctx context.Context, id uuid.UUID) (*Receivable, error)
	CreateReceivable(ctx context.Context, input CreateReceivableInput, dates financeExpenseDates) (*Receivable, error)
	ListReceivables(ctx context.Context, limit int) ([]Receivable, error)
	GetReceivable(ctx context.Context, id uuid.UUID) (*Receivable, error)
	UpdateReceivable(ctx context.Context, id uuid.UUID, input UpdateReceivableInput, dates financeExpenseDates) (*Receivable, error)
	UpdateReceivableStatus(ctx context.Context, id uuid.UUID, status string) (*Receivable, error)
	VoidReceivable(ctx context.Context, id uuid.UUID, reason string) (*Receivable, error)
	CreateReceipt(ctx context.Context, input CreateReceiptInput, receivedAt *time.Time) (*Receipt, error)
	ListReceipts(ctx context.Context, limit int) ([]Receipt, error)
	AllocateReceipt(ctx context.Context, receiptID uuid.UUID, input AllocateReceiptInput) (*ReceiptAllocation, error)
	CreatePayable(ctx context.Context, input CreatePayableInput, dates financeExpenseDates) (*Payable, error)
	ListPayables(ctx context.Context, limit int) ([]Payable, error)
	GetPayable(ctx context.Context, id uuid.UUID) (*Payable, error)
	UpdatePayable(ctx context.Context, id uuid.UUID, input UpdatePayableInput, dates financeExpenseDates) (*Payable, error)
	VoidPayable(ctx context.Context, id uuid.UUID, reason string) (*Payable, error)
	CreatePayment(ctx context.Context, input CreatePaymentInput, paidAt *time.Time) (*Payment, error)
	ListPayments(ctx context.Context, limit int) ([]Payment, error)
	GetPayment(ctx context.Context, id uuid.UUID) (*Payment, error)
	UpdatePayment(ctx context.Context, id uuid.UUID, input UpdatePaymentInput, paidAt *time.Time) (*Payment, error)
	VoidPayment(ctx context.Context, id uuid.UUID, reason string) (*Payment, error)
	AllocatePayment(ctx context.Context, paymentID uuid.UUID, input AllocatePaymentInput) (*PaymentAllocation, error)
	CreateGLAccount(ctx context.Context, input CreateGLAccountInput) (*GLAccount, error)
	ListGLAccounts(ctx context.Context, limit int) ([]GLAccount, error)
	CreateGLCostCenter(ctx context.Context, input CreateGLCostCenterInput) (*GLCostCenter, error)
	ListGLCostCenters(ctx context.Context, limit int) ([]GLCostCenter, error)
	CreateGLJournalEntry(ctx context.Context, input CreateGLJournalEntryInput, referenceDate time.Time) (*GLJournalEntry, error)
	ListGLJournalEntries(ctx context.Context, limit int) ([]GLJournalEntry, error)
	GetGLJournalEntry(ctx context.Context, id uuid.UUID) (*GLJournalEntry, error)
	PostGLJournalEntry(ctx context.Context, id uuid.UUID) (*GLJournalEntry, error)
	GetGLTrialBalance(ctx context.Context, input GLTrialBalanceInput) (*GLTrialBalance, error)
}

type CostPoster interface {
	CreateCostEntryFromAIUsage(ctx context.Context, projectID uuid.UUID, input project.CreateCostEntryInput) (*project.CostEntry, error)
}

type ObservabilityRecorder interface {
	StartTrace(ctx context.Context, workflowID *uuid.UUID, metadata map[string]any) (*observability.Trace, error)
	RecordSpan(ctx context.Context, input observability.RecordSpanInput) (*observability.Span, error)
	RecordMetric(ctx context.Context, input observability.RecordMetricInput) (*observability.Metric, error)
	CompleteTrace(ctx context.Context, id uuid.UUID, status string) error
}

type Service struct {
	repo          Repository
	httpClient    *http.Client
	costPoster    CostPoster
	observability ObservabilityRecorder
}

type ServiceOption func(*Service)

func WithHTTPClient(client *http.Client) ServiceOption {
	return func(s *Service) {
		if client != nil {
			s.httpClient = client
		}
	}
}

func WithCostPoster(costPoster CostPoster) ServiceOption {
	return func(s *Service) {
		s.costPoster = costPoster
	}
}

func WithObservability(recorder ObservabilityRecorder) ServiceOption {
	return func(s *Service) {
		s.observability = recorder
	}
}

func NewService(repo Repository, opts ...ServiceOption) *Service {
	s := &Service{
		repo:       repo,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Service) CreateAdapter(ctx context.Context, input CreateAdapterInput) (*FinanceAdapter, error) {
	if input.Name == "" || input.EndpointURL == "" || input.Secret == "" {
		return nil, fmt.Errorf("%w: name, endpoint_url, and secret are required", ErrValidation)
	}
	normalizeCreateAdapterInput(&input)
	if err := validateEndpoint(input.EndpointURL); err != nil {
		return nil, err
	}
	if !validAuthType(input.AuthType) {
		return nil, fmt.Errorf("%w: unsupported auth_type %q", ErrValidation, input.AuthType)
	}
	if !validAdapterStatus(input.Status) {
		return nil, fmt.Errorf("%w: unsupported adapter status %q", ErrValidation, input.Status)
	}
	return s.repo.CreateAdapter(ctx, input)
}

func (s *Service) ListAdapters(ctx context.Context, limit int) ([]FinanceAdapter, error) {
	items, err := s.repo.ListAdapters(ctx, normalizeLimit(limit))
	if items == nil {
		items = []FinanceAdapter{}
	}
	return items, err
}

func (s *Service) UpdateAdapter(ctx context.Context, id uuid.UUID, input UpdateAdapterInput) (*FinanceAdapter, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("%w: adapter id is required", ErrValidation)
	}
	if input.EndpointURL != nil {
		if err := validateEndpoint(*input.EndpointURL); err != nil {
			return nil, err
		}
	}
	if input.AuthType != nil && !validAuthType(*input.AuthType) {
		return nil, fmt.Errorf("%w: unsupported auth_type %q", ErrValidation, *input.AuthType)
	}
	if input.Secret != nil && strings.TrimSpace(*input.Secret) == "" {
		return nil, fmt.Errorf("%w: secret cannot be empty", ErrValidation)
	}
	if input.Status != nil && !validAdapterStatus(*input.Status) {
		return nil, fmt.Errorf("%w: unsupported adapter status %q", ErrValidation, *input.Status)
	}
	return s.repo.UpdateAdapter(ctx, id, input)
}

func (s *Service) TestAdapter(ctx context.Context, id uuid.UUID) error {
	adapter, err := s.repo.GetAdapterSecret(ctx, id)
	if err != nil {
		return err
	}
	if adapter.Status == AdapterDisabled {
		return fmt.Errorf("%w: adapter is disabled", ErrValidation)
	}
	payload := map[string]any{
		"format_version": "meta-org.finance.test.v1",
		"adapter_id":     adapter.ID.String(),
		"sent_at":        time.Now().UTC().Format(time.RFC3339),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal adapter test payload: %w", err)
	}
	result, err := s.sendAdapterRequest(ctx, adapter, body, "adapter-test")
	if err != nil {
		status := AdapterError
		_, _ = s.repo.UpdateAdapter(ctx, id, UpdateAdapterInput{Status: &status})
		return fmt.Errorf("test finance adapter: %w", err)
	}
	if result.StatusCode < 200 || result.StatusCode >= 300 {
		status := AdapterError
		_, _ = s.repo.UpdateAdapter(ctx, id, UpdateAdapterInput{Status: &status})
		return fmt.Errorf("%w: finance adapter returned HTTP %d", ErrValidation, result.StatusCode)
	}
	status := AdapterActive
	_, _ = s.repo.UpdateAdapter(ctx, id, UpdateAdapterInput{Status: &status})
	return nil
}

func (s *Service) CreateExportBatch(ctx context.Context, input CreateExportBatchInput) (*ExportBatch, error) {
	if input.AdapterID == uuid.Nil {
		return nil, fmt.Errorf("%w: adapter_id is required", ErrValidation)
	}
	adapter, err := s.repo.GetAdapterSecret(ctx, input.AdapterID)
	if err != nil {
		return nil, err
	}
	if adapter.Status != "" && adapter.Status != AdapterActive {
		return nil, fmt.Errorf("%w: finance adapter is %s", ErrValidation, adapter.Status)
	}
	if err := normalizeExportBatchInput(&input); err != nil {
		return nil, err
	}
	batch, err := s.repo.CreateExportBatch(ctx, input)
	if err != nil {
		return nil, err
	}
	if err := s.postProjectCosts(ctx, batch, input.ActorID, input.ActorType); err != nil {
		_, _ = s.repo.UpdateExportBatchStatus(ctx, batch.ID, UpdateExportBatchStatusInput{
			Status:       BatchFailed,
			ErrorMessage: err.Error(),
			Metadata: map[string]any{
				"failed_stage": "project_cost_posting",
			},
		})
		s.recordFinanceMetric(ctx, "finance_export_failed", &batch.ID, "finance_export_batch", 1, map[string]any{"stage": "project_cost_posting"})
		return nil, err
	}
	return batch, nil
}

func (s *Service) ListExportBatches(ctx context.Context, limit int) ([]ExportBatch, error) {
	items, err := s.repo.ListExportBatches(ctx, normalizeLimit(limit))
	if items == nil {
		items = []ExportBatch{}
	}
	return items, err
}

func (s *Service) GetExportBatch(ctx context.Context, id uuid.UUID) (*ExportBatch, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("%w: batch id is required", ErrValidation)
	}
	return s.repo.GetExportBatch(ctx, id)
}

func (s *Service) SubmitExportBatch(ctx context.Context, id uuid.UUID) (*ExportBatch, error) {
	batch, err := s.repo.GetExportBatch(ctx, id)
	if err != nil {
		return nil, err
	}
	trace := s.startFinanceTrace(ctx, "finance_export", batch.AdapterID, &batch.ID, map[string]any{
		"status":          batch.Status,
		"currency":        batch.Currency,
		"total_amount":    batch.TotalAmount,
		"idempotency_key": batch.IdempotencyKey,
	})
	if batch.Status == BatchCancelled || batch.Status == BatchReconciled {
		message := fmt.Sprintf("batch status %q cannot be submitted", batch.Status)
		s.recordFinanceSpan(ctx, trace, observability.SpanFinanceExport, &batch.ID, "finance_export_batch", map[string]any{"status": batch.Status}, map[string]any{"status": BatchFailed, "error": message}, 0)
		s.completeObservationTrace(ctx, trace, observability.TraceFailed)
		return nil, fmt.Errorf("%w: batch status %q cannot be submitted", ErrValidation, batch.Status)
	}
	if len(batch.Lines) == 0 {
		message := "export batch has no lines"
		s.recordFinanceSpan(ctx, trace, observability.SpanFinanceExport, &batch.ID, "finance_export_batch", map[string]any{"status": batch.Status}, map[string]any{"status": BatchFailed, "error": message}, 0)
		s.completeObservationTrace(ctx, trace, observability.TraceFailed)
		return nil, fmt.Errorf("%w: export batch has no lines", ErrValidation)
	}
	adapter, err := s.repo.GetAdapterSecret(ctx, batch.AdapterID)
	if err != nil {
		s.recordFinanceSpan(ctx, trace, observability.SpanFinanceExport, &batch.ID, "finance_export_batch", map[string]any{"adapter_id": batch.AdapterID.String()}, map[string]any{"status": BatchFailed, "error": err.Error()}, 0)
		s.completeObservationTrace(ctx, trace, observability.TraceFailed)
		return nil, err
	}
	if adapter.Status != AdapterActive {
		message := fmt.Sprintf("finance adapter is %s", adapter.Status)
		s.recordFinanceSpan(ctx, trace, observability.SpanFinanceExport, &batch.ID, "finance_export_batch", map[string]any{"adapter_status": adapter.Status}, map[string]any{"status": BatchFailed, "error": message}, 0)
		s.completeObservationTrace(ctx, trace, observability.TraceFailed)
		return nil, fmt.Errorf("%w: finance adapter is %s", ErrValidation, adapter.Status)
	}
	payload := exportPayload(batch)
	body, err := json.Marshal(payload)
	if err != nil {
		s.recordFinanceSpan(ctx, trace, observability.SpanFinanceExport, &batch.ID, "finance_export_batch", map[string]any{"line_count": len(batch.Lines)}, map[string]any{"status": BatchFailed, "error": err.Error()}, 0)
		s.completeObservationTrace(ctx, trace, observability.TraceFailed)
		return nil, fmt.Errorf("marshal finance export payload: %w", err)
	}
	if _, err := s.repo.UpdateExportBatchStatus(ctx, id, UpdateExportBatchStatusInput{Status: BatchExporting, Submitted: true}); err != nil {
		s.recordFinanceSpan(ctx, trace, observability.SpanFinanceExport, &batch.ID, "finance_export_batch", map[string]any{"line_count": len(batch.Lines)}, map[string]any{"status": BatchFailed, "error": err.Error()}, 0)
		s.completeObservationTrace(ctx, trace, observability.TraceFailed)
		return nil, err
	}
	started := time.Now()
	result, err := s.sendAdapterRequest(ctx, adapter, body, batch.IdempotencyKey)
	durationMS := int(time.Since(started).Milliseconds())
	if err != nil {
		s.recordFinanceSpan(ctx, trace, observability.SpanFinanceExport, &batch.ID, "finance_export_batch", map[string]any{"line_count": len(batch.Lines)}, map[string]any{"status": BatchFailed, "error": err.Error()}, durationMS)
		s.completeObservationTrace(ctx, trace, observability.TraceFailed)
		return s.markExportFailed(ctx, id, fmt.Sprintf("submit finance export: %v", err))
	}
	responseBody := result.Body
	if result.StatusCode < 200 || result.StatusCode >= 300 {
		message := fmt.Sprintf("finance adapter returned HTTP %d", result.StatusCode)
		s.recordFinanceSpan(ctx, trace, observability.SpanFinanceExport, &batch.ID, "finance_export_batch", map[string]any{"line_count": len(batch.Lines)}, map[string]any{"status": BatchFailed, "error": message}, durationMS)
		s.completeObservationTrace(ctx, trace, observability.TraceFailed)
		return s.markExportFailed(ctx, id, fmt.Sprintf("finance adapter returned HTTP %d: %s", result.StatusCode, strings.TrimSpace(string(responseBody))))
	}
	statusInput := UpdateExportBatchStatusInput{
		Status:    BatchExported,
		Submitted: true,
		Metadata: map[string]any{
			"submit_status_code": result.StatusCode,
		},
	}
	var adapterResponse struct {
		ExternalBatchID string         `json:"external_batch_id"`
		Status          string         `json:"status"`
		Metadata        map[string]any `json:"metadata"`
	}
	if len(responseBody) > 0 && json.Unmarshal(responseBody, &adapterResponse) == nil {
		statusInput.ExternalBatchID = adapterResponse.ExternalBatchID
		if mapped := mapExternalBatchStatus(adapterResponse.Status); mapped != "" {
			statusInput.Status = mapped
		}
		for key, value := range adapterResponse.Metadata {
			statusInput.Metadata[key] = value
		}
	}
	updated, err := s.repo.UpdateExportBatchStatus(ctx, id, statusInput)
	if err != nil {
		s.recordFinanceSpan(ctx, trace, observability.SpanFinanceExport, &batch.ID, "finance_export_batch", map[string]any{"line_count": len(batch.Lines)}, map[string]any{"status": BatchFailed, "error": err.Error()}, durationMS)
		s.completeObservationTrace(ctx, trace, observability.TraceFailed)
		return nil, err
	}
	s.recordFinanceSpan(ctx, trace, observability.SpanFinanceExport, &batch.ID, "finance_export_batch", map[string]any{"line_count": len(batch.Lines)}, map[string]any{"status": updated.Status, "status_code": result.StatusCode}, durationMS)
	if updated.Status == BatchFailed {
		s.recordFinanceMetric(ctx, "finance_export_failed", &batch.ID, "finance_export_batch", 1, map[string]any{"stage": "adapter_response"})
		s.completeObservationTrace(ctx, trace, observability.TraceFailed)
	} else {
		s.completeObservationTrace(ctx, trace, observability.TraceComplete)
	}
	return updated, nil
}

func (s *Service) ReceiveWebhook(ctx context.Context, adapterID uuid.UUID, body []byte, signature string, authorization string) (*WebhookEvent, error) {
	if adapterID == uuid.Nil {
		return nil, fmt.Errorf("%w: adapter id is required", ErrValidation)
	}
	adapter, err := s.repo.GetAdapterSecret(ctx, adapterID)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, fmt.Errorf("%w: invalid webhook payload", ErrValidation)
		}
	}
	trace := s.startFinanceTrace(ctx, "finance_webhook", adapterID, uuidFromPayload(payload, "batch_id"), map[string]any{
		"event_type": stringFromPayload(payload, "event_type", "finance.webhook"),
	})
	started := time.Now()
	valid := verifyAdapterCallback(adapter, body, signature, authorization)
	eventInput := RecordWebhookEventInput{
		AdapterID:      adapterID,
		BatchID:        uuidFromPayload(payload, "batch_id"),
		EventType:      stringFromPayload(payload, "event_type", "finance.webhook"),
		SignatureValid: valid,
		Payload:        payload,
		Processed:      false,
	}
	if !valid {
		eventInput.ErrorMessage = "invalid signature"
		event, _ := s.repo.RecordWebhookEvent(ctx, eventInput)
		s.recordFinanceSpan(ctx, trace, observability.SpanFinanceWebhook, nil, "finance_webhook_event", map[string]any{"adapter_id": adapterID.String()}, map[string]any{"signature_valid": false, "error": eventInput.ErrorMessage}, int(time.Since(started).Milliseconds()))
		s.recordFinanceMetric(ctx, "finance_webhook_invalid_signature", nil, "finance_webhook_event", 1, map[string]any{"adapter_id": adapterID.String()})
		s.completeObservationTrace(ctx, trace, observability.TraceFailed)
		return event, fmt.Errorf("%w: invalid webhook signature", ErrForbidden)
	}
	if eventInput.BatchID != nil {
		if amount, ok := externalAmountFloatFromPayload(payload); ok {
			if batch, err := s.repo.GetExportBatch(ctx, *eventInput.BatchID); err == nil {
				s.recordFinanceMetric(ctx, "finance_reconciliation_difference", eventInput.BatchID, "finance_export_batch", amount-batch.TotalAmount, map[string]any{
					"external_amount": amount,
					"total_amount":    batch.TotalAmount,
					"currency":        batch.Currency,
				})
			}
		}
		status := mapExternalBatchStatus(firstNonEmptyString(payload, "status", "batch_status"))
		if status != "" {
			metadata := map[string]any{
				"last_webhook": payload,
			}
			if amount, ok := externalAmountFromPayload(payload); ok {
				metadata["external_amount"] = amount
			}
			update := UpdateExportBatchStatusInput{
				Status:          status,
				ExternalBatchID: stringFromPayload(payload, "external_batch_id", ""),
				ErrorMessage:    stringFromPayload(payload, "error_message", ""),
				Metadata:        metadata,
			}
			if _, err := s.repo.UpdateExportBatchStatus(ctx, *eventInput.BatchID, update); err != nil {
				eventInput.ErrorMessage = err.Error()
				event, _ := s.repo.RecordWebhookEvent(ctx, eventInput)
				s.recordFinanceSpan(ctx, trace, observability.SpanFinanceWebhook, eventInput.BatchID, "finance_export_batch", map[string]any{"adapter_id": adapterID.String()}, map[string]any{"processed": false, "error": err.Error()}, int(time.Since(started).Milliseconds()))
				s.completeObservationTrace(ctx, trace, observability.TraceFailed)
				return event, err
			}
			if status == BatchFailed {
				s.recordFinanceMetric(ctx, "finance_export_failed", eventInput.BatchID, "finance_export_batch", 1, map[string]any{"stage": "webhook"})
			}
		}
	}
	s.updateWebhookLines(ctx, payload)
	eventInput.Processed = true
	event, err := s.repo.RecordWebhookEvent(ctx, eventInput)
	if err != nil {
		s.recordFinanceSpan(ctx, trace, observability.SpanFinanceWebhook, eventInput.BatchID, "finance_webhook_event", map[string]any{"adapter_id": adapterID.String()}, map[string]any{"processed": false, "error": err.Error()}, int(time.Since(started).Milliseconds()))
		s.completeObservationTrace(ctx, trace, observability.TraceFailed)
		return nil, err
	}
	entityID := &event.ID
	entityType := "finance_webhook_event"
	if eventInput.BatchID != nil {
		entityID = eventInput.BatchID
		entityType = "finance_export_batch"
	}
	s.recordFinanceSpan(ctx, trace, observability.SpanFinanceWebhook, entityID, entityType, map[string]any{"adapter_id": adapterID.String(), "signature_valid": true}, map[string]any{"processed": true, "event_type": event.EventType}, int(time.Since(started).Milliseconds()))
	s.completeObservationTrace(ctx, trace, observability.TraceComplete)
	return event, nil
}

func (s *Service) ListReconciliation(ctx context.Context, limit int) ([]ReconciliationItem, error) {
	items, err := s.repo.ListReconciliation(ctx, normalizeLimit(limit))
	if items == nil {
		items = []ReconciliationItem{}
	}
	return items, err
}

func (s *Service) ImportExpenses(ctx context.Context, input ImportExpensesInput) (*ImportExpensesResult, error) {
	if input.AdapterID == uuid.Nil {
		return nil, fmt.Errorf("%w: adapter_id is required", ErrValidation)
	}
	if len(input.Records) == 0 {
		return nil, fmt.Errorf("%w: records are required", ErrValidation)
	}
	if input.SourceType == "" {
		input.SourceType = "api"
	}
	adapter, err := s.repo.GetAdapterSecret(ctx, input.AdapterID)
	if err != nil {
		return nil, err
	}
	batch, err := s.repo.CreateImportBatch(ctx, &input.AdapterID, input.SourceType, input.FileName, len(input.Records), input.Metadata)
	if err != nil {
		return nil, err
	}
	records := make([]ImportRecord, 0, len(input.Records))
	processed := 0
	failed := 0
	for _, raw := range input.Records {
		expense, dates, err := normalizeExpenseRecord(raw, adapter.FieldMapping)
		if err != nil {
			failed++
			continue
		}
		if err := applyTenantFinanceExpenseScope(ctx, &expense); err != nil {
			failed++
			continue
		}
		record, err := s.repo.CreateImportedExpense(ctx, batch.ID, input.AdapterID, raw, expense, expenseOccurredAt(dates), dates)
		if err != nil {
			failed++
			continue
		}
		records = append(records, *record)
		processed++
	}
	batch, err = s.repo.CompleteImportBatch(ctx, batch.ID, processed, failed)
	if err != nil {
		return nil, err
	}
	return &ImportExpensesResult{Batch: batch, Records: records}, nil
}

func (s *Service) ReceiveExpenseWebhook(ctx context.Context, adapterID uuid.UUID, body []byte, signature string, authorization string) (*ImportExpensesResult, error) {
	if adapterID == uuid.Nil {
		return nil, fmt.Errorf("%w: adapter id is required", ErrValidation)
	}
	adapter, err := s.repo.GetAdapterSecret(ctx, adapterID)
	if err != nil {
		return nil, err
	}
	if !verifyAdapterCallback(adapter, body, signature, authorization) {
		return nil, fmt.Errorf("%w: invalid webhook signature", ErrForbidden)
	}
	payload := map[string]any{}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, fmt.Errorf("%w: invalid webhook payload", ErrValidation)
		}
	}
	records := recordsFromPayload(payload)
	return s.ImportExpenses(ctx, ImportExpensesInput{
		AdapterID:  adapterID,
		SourceType: "webhook",
		Records:    records,
		Metadata:   map[string]any{"webhook_payload": payload},
	})
}

func (s *Service) PullAdapterExpenses(ctx context.Context, adapterID uuid.UUID) (*ImportExpensesResult, error) {
	if adapterID == uuid.Nil {
		return nil, fmt.Errorf("%w: adapter id is required", ErrValidation)
	}
	adapter, err := s.repo.GetAdapterSecret(ctx, adapterID)
	if err != nil {
		return nil, err
	}
	if adapter.EndpointURL == "" {
		return nil, fmt.Errorf("%w: adapter endpoint_url is required", ErrValidation)
	}
	reqCtx, cancel := context.WithTimeout(ctx, adapterTimeout(adapter.TimeoutMS))
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, adapter.EndpointURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create finance pull request: %w", err)
	}
	if adapter.AuthType == AuthBearer {
		req.Header.Set("Authorization", "Bearer "+adapter.Secret)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pull finance expenses: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: pull endpoint returned HTTP %d", ErrValidation, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("read finance pull response: %w", err)
	}
	payload := map[string]any{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("%w: invalid pull response", ErrValidation)
	}
	return s.ImportExpenses(ctx, ImportExpensesInput{
		AdapterID:  adapterID,
		SourceType: "pull",
		Records:    recordsFromPayload(payload),
		Metadata:   map[string]any{"pull_status_code": resp.StatusCode},
	})
}

func (s *Service) ListImportBatches(ctx context.Context, limit int) ([]ImportBatch, error) {
	items, err := s.repo.ListImportBatches(ctx, normalizeLimit(limit))
	if items == nil {
		items = []ImportBatch{}
	}
	return items, err
}

func (s *Service) ListImportRecords(ctx context.Context, limit int) ([]ImportRecord, error) {
	items, err := s.repo.ListImportRecords(ctx, normalizeLimit(limit))
	if items == nil {
		items = []ImportRecord{}
	}
	return items, err
}

func (s *Service) CreateSettlementOrder(ctx context.Context, input CreateSettlementOrderInput) (*SettlementOrder, error) {
	normalizeSettlementOrderInput(&input)
	if len(input.Lines) == 0 {
		return nil, fmt.Errorf("%w: settlement lines are required", ErrValidation)
	}
	for i := range input.Lines {
		normalizeSettlementLineInput(&input.Lines[i])
		if input.Lines[i].Amount <= 0 {
			return nil, fmt.Errorf("%w: settlement line amount must be greater than zero", ErrValidation)
		}
	}
	return s.repo.CreateSettlementOrder(ctx, input)
}

func (s *Service) ListSettlementOrders(ctx context.Context, limit int) ([]SettlementOrder, error) {
	items, err := s.repo.ListSettlementOrders(ctx, normalizeLimit(limit))
	if items == nil {
		items = []SettlementOrder{}
	}
	return items, err
}

func (s *Service) GetSettlementOrder(ctx context.Context, id uuid.UUID) (*SettlementOrder, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("%w: settlement_order_id is required", ErrValidation)
	}
	return s.repo.GetSettlementOrder(ctx, id)
}

func (s *Service) UpdateSettlementOrder(ctx context.Context, id uuid.UUID, input UpdateSettlementOrderInput) (*SettlementOrder, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("%w: settlement_order_id is required", ErrValidation)
	}
	for i := range input.Lines {
		normalizeSettlementLineInput(&input.Lines[i])
		if input.Lines[i].Amount <= 0 {
			return nil, fmt.Errorf("%w: settlement line amount must be greater than zero", ErrValidation)
		}
	}
	return s.repo.UpdateSettlementOrder(ctx, id, input)
}

func (s *Service) VoidSettlementOrder(ctx context.Context, id uuid.UUID, reason string) (*SettlementOrder, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("%w: settlement_order_id is required", ErrValidation)
	}
	return s.repo.VoidSettlementOrder(ctx, id, strings.TrimSpace(reason))
}

func (s *Service) PostSettlementOrder(ctx context.Context, id uuid.UUID) (*Receivable, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("%w: settlement_order_id is required", ErrValidation)
	}
	return s.repo.PostSettlementOrder(ctx, id)
}

func (s *Service) CreateReceivable(ctx context.Context, input CreateReceivableInput) (*Receivable, error) {
	normalizeReceivableInput(&input)
	if input.OrganizationID == nil {
		input.OrganizationID = currentTenantOrganizationID(ctx)
	}
	if err := ensureTenantOrganization(ctx, input.OrganizationID); err != nil {
		return nil, err
	}
	if input.Amount <= 0 {
		return nil, fmt.Errorf("%w: amount must be greater than zero", ErrValidation)
	}
	dates, err := datesFromReceivable(input)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateReceivable(ctx, input, dates)
}

func (s *Service) ListReceivables(ctx context.Context, limit int) ([]Receivable, error) {
	items, err := s.repo.ListReceivables(ctx, normalizeLimit(limit))
	if items == nil {
		items = []Receivable{}
	}
	return items, err
}

func (s *Service) FindReceivableBySource(ctx context.Context, sourceType string, sourceID uuid.UUID) (*Receivable, error) {
	sourceType = strings.TrimSpace(sourceType)
	if sourceType == "" || sourceID == uuid.Nil {
		return nil, fmt.Errorf("%w: source_type and source_id are required", ErrValidation)
	}
	repo, ok := s.repo.(interface {
		FindReceivableBySource(context.Context, string, uuid.UUID) (*Receivable, error)
	})
	if !ok {
		return nil, ErrNotFound
	}
	return repo.FindReceivableBySource(ctx, sourceType, sourceID)
}

func (s *Service) UpdateReceivable(ctx context.Context, id uuid.UUID, input UpdateReceivableInput) (*Receivable, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("%w: receivable_id is required", ErrValidation)
	}
	createInput := CreateReceivableInput(input)
	normalizeReceivableInput(&createInput)
	dates, err := datesFromReceivable(createInput)
	if err != nil {
		return nil, err
	}
	return s.repo.UpdateReceivable(ctx, id, input, dates)
}

func (s *Service) VoidReceivable(ctx context.Context, id uuid.UUID, reason string) (*Receivable, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("%w: receivable_id is required", ErrValidation)
	}
	receivable, err := s.repo.GetReceivable(ctx, id)
	if err != nil {
		return nil, err
	}
	if receivable.ReceivedAmount > 0 || receivable.Status == "paid" || receivable.Status == "partially_received" {
		return nil, fmt.Errorf("%w: allocated receivable cannot be voided", ErrValidation)
	}
	return s.repo.VoidReceivable(ctx, id, strings.TrimSpace(reason))
}

func (s *Service) CreateReceipt(ctx context.Context, input CreateReceiptInput) (*Receipt, error) {
	normalizeReceiptInput(&input)
	if input.Amount <= 0 {
		return nil, fmt.Errorf("%w: amount must be greater than zero", ErrValidation)
	}
	receivedAt, err := parseOptionalTime(input.ReceivedAt)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateReceipt(ctx, input, receivedAt)
}

func (s *Service) ListReceipts(ctx context.Context, limit int) ([]Receipt, error) {
	items, err := s.repo.ListReceipts(ctx, normalizeLimit(limit))
	if items == nil {
		items = []Receipt{}
	}
	return items, err
}

func (s *Service) AllocateReceipt(ctx context.Context, receiptID uuid.UUID, input AllocateReceiptInput) (*ReceiptAllocation, error) {
	if receiptID == uuid.Nil || input.ReceivableID == uuid.Nil {
		return nil, fmt.Errorf("%w: receipt_id and receivable_id are required", ErrValidation)
	}
	if input.Amount <= 0 {
		return nil, fmt.Errorf("%w: amount must be greater than zero", ErrValidation)
	}
	if input.Currency == "" {
		input.Currency = "CNY"
	} else {
		input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
	receivable, err := s.repo.GetReceivable(ctx, input.ReceivableID)
	if err != nil {
		return nil, err
	}
	received := receivable.ReceivedAmount + input.Amount
	status := "partially_received"
	if received >= receivable.Amount {
		status = "paid"
	}
	allocation, err := s.repo.AllocateReceipt(ctx, receiptID, input)
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.UpdateReceivableStatus(ctx, input.ReceivableID, status); err != nil {
		return nil, err
	}
	return allocation, nil
}

func (s *Service) CreatePayable(ctx context.Context, input CreatePayableInput) (*Payable, error) {
	normalizePayableInput(&input)
	if input.OrganizationID == nil {
		input.OrganizationID = currentTenantOrganizationID(ctx)
	}
	if err := ensureTenantOrganization(ctx, input.OrganizationID); err != nil {
		return nil, err
	}
	if input.Amount <= 0 {
		return nil, fmt.Errorf("%w: amount must be greater than zero", ErrValidation)
	}
	dates, err := datesFromPayable(input)
	if err != nil {
		return nil, err
	}
	return s.repo.CreatePayable(ctx, input, dates)
}

func (s *Service) ListPayables(ctx context.Context, limit int) ([]Payable, error) {
	items, err := s.repo.ListPayables(ctx, normalizeLimit(limit))
	if items == nil {
		items = []Payable{}
	}
	return items, err
}

func (s *Service) FindPayableBySource(ctx context.Context, sourceType string, sourceID uuid.UUID) (*Payable, error) {
	sourceType = strings.TrimSpace(sourceType)
	if sourceType == "" || sourceID == uuid.Nil {
		return nil, fmt.Errorf("%w: source_type and source_id are required", ErrValidation)
	}
	repo, ok := s.repo.(interface {
		FindPayableBySource(context.Context, string, uuid.UUID) (*Payable, error)
	})
	if !ok {
		return nil, ErrNotFound
	}
	return repo.FindPayableBySource(ctx, sourceType, sourceID)
}

func (s *Service) UpdatePayable(ctx context.Context, id uuid.UUID, input UpdatePayableInput) (*Payable, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("%w: payable_id is required", ErrValidation)
	}
	createInput := CreatePayableInput(input)
	normalizePayableInput(&createInput)
	dates, err := datesFromPayable(createInput)
	if err != nil {
		return nil, err
	}
	return s.repo.UpdatePayable(ctx, id, input, dates)
}

func (s *Service) VoidPayable(ctx context.Context, id uuid.UUID, reason string) (*Payable, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("%w: payable_id is required", ErrValidation)
	}
	payable, err := s.repo.GetPayable(ctx, id)
	if err != nil {
		return nil, err
	}
	if payable.PaidAmount > 0 || payable.Status == "paid" || payable.Status == "partially_paid" {
		return nil, fmt.Errorf("%w: allocated payable cannot be voided", ErrValidation)
	}
	return s.repo.VoidPayable(ctx, id, strings.TrimSpace(reason))
}

func (s *Service) CreatePayment(ctx context.Context, input CreatePaymentInput) (*Payment, error) {
	normalizePaymentInput(&input)
	if input.Amount <= 0 {
		return nil, fmt.Errorf("%w: amount must be greater than zero", ErrValidation)
	}
	paidAt, err := parseOptionalTime(input.PaidAt)
	if err != nil {
		return nil, err
	}
	return s.repo.CreatePayment(ctx, input, paidAt)
}

func (s *Service) ListPayments(ctx context.Context, limit int) ([]Payment, error) {
	items, err := s.repo.ListPayments(ctx, normalizeLimit(limit))
	if items == nil {
		items = []Payment{}
	}
	return items, err
}

func (s *Service) UpdatePayment(ctx context.Context, id uuid.UUID, input UpdatePaymentInput) (*Payment, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("%w: payment_id is required", ErrValidation)
	}
	createInput := CreatePaymentInput(input)
	normalizePaymentInput(&createInput)
	paidAt, err := parseOptionalTime(createInput.PaidAt)
	if err != nil {
		return nil, err
	}
	return s.repo.UpdatePayment(ctx, id, input, paidAt)
}

func (s *Service) VoidPayment(ctx context.Context, id uuid.UUID, reason string) (*Payment, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("%w: payment_id is required", ErrValidation)
	}
	return s.repo.VoidPayment(ctx, id, strings.TrimSpace(reason))
}

func (s *Service) AllocatePayment(ctx context.Context, paymentID uuid.UUID, input AllocatePaymentInput) (*PaymentAllocation, error) {
	if paymentID == uuid.Nil || input.PayableID == uuid.Nil {
		return nil, fmt.Errorf("%w: payment_id and payable_id are required", ErrValidation)
	}
	if input.Amount <= 0 {
		return nil, fmt.Errorf("%w: amount must be greater than zero", ErrValidation)
	}
	if input.Currency == "" {
		input.Currency = "CNY"
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
	return s.repo.AllocatePayment(ctx, paymentID, input)
}

func (s *Service) markExportFailed(ctx context.Context, id uuid.UUID, message string) (*ExportBatch, error) {
	batch, err := s.repo.UpdateExportBatchStatus(ctx, id, UpdateExportBatchStatusInput{
		Status:       BatchFailed,
		ErrorMessage: message,
	})
	if err != nil {
		return nil, err
	}
	s.recordFinanceMetric(ctx, "finance_export_failed", &id, "finance_export_batch", 1, map[string]any{"message": message})
	return batch, fmt.Errorf("%w: %s", ErrValidation, message)
}

func (s *Service) postProjectCosts(ctx context.Context, batch *ExportBatch, actorID *uuid.UUID, actorType string) error {
	if s.costPoster == nil || batch == nil {
		return nil
	}
	for _, line := range batch.Lines {
		if line.ProjectID == nil || line.UsageLedgerID == nil || line.ProjectCostEntryID != nil || line.Amount == 0 {
			continue
		}
		entry, err := s.costPoster.CreateCostEntryFromAIUsage(ctx, *line.ProjectID, project.CreateCostEntryInput{
			ActorInput:  project.ActorInput{ActorID: actorID, ActorType: actorType},
			SourceID:    line.UsageLedgerID,
			Amount:      line.Amount,
			Currency:    line.Currency,
			OccurredAt:  &line.CreatedAt,
			Description: "AI usage cost",
			Metadata: map[string]any{
				"finance_export_batch_id": batch.ID.String(),
				"finance_export_line_id":  line.ID.String(),
			},
		})
		if err != nil {
			return fmt.Errorf("post AI usage to project cost: %w", err)
		}
		if entry != nil && entry.ID != uuid.Nil {
			if err := s.repo.LinkProjectCostEntry(ctx, line.ID, entry.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) updateWebhookLines(ctx context.Context, payload map[string]any) {
	raw, ok := payload["lines"].([]any)
	if !ok {
		return
	}
	for _, item := range raw {
		linePayload, ok := item.(map[string]any)
		if !ok {
			continue
		}
		lineID := uuidFromPayload(linePayload, "line_id")
		if lineID == nil {
			continue
		}
		_, _ = s.repo.UpdateExportLineStatus(ctx, *lineID, UpdateExportLineStatusInput{
			Status:         mapLineStatus(stringFromPayload(linePayload, "status", "")),
			ExternalLineID: stringFromPayload(linePayload, "external_line_id", ""),
			Metadata: map[string]any{
				"last_webhook_line": linePayload,
			},
		})
	}
}

type adapterHTTPResult struct {
	StatusCode int
	Body       []byte
}

func (s *Service) sendAdapterRequest(ctx context.Context, adapter AdapterSecret, body []byte, idempotencyKey string) (adapterHTTPResult, error) {
	attempts := adapterAttempts(adapter.RetryCount)
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		reqCtx, cancel := context.WithTimeout(ctx, adapterTimeout(adapter.TimeoutMS))
		req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, adapter.EndpointURL, bytes.NewReader(body))
		if err != nil {
			cancel()
			return adapterHTTPResult{}, fmt.Errorf("create finance adapter request: %w", err)
		}
		applyAuthHeaders(req, adapter, body, idempotencyKey)
		resp, err := s.httpClient.Do(req)
		if err != nil {
			cancel()
			lastErr = err
			if ctx.Err() != nil || attempt == attempts-1 {
				break
			}
			sleepBeforeRetry(ctx, attempt)
			continue
		}
		responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		closeErr := resp.Body.Close()
		cancel()
		if readErr != nil {
			return adapterHTTPResult{}, fmt.Errorf("read finance adapter response: %w", readErr)
		}
		if closeErr != nil {
			return adapterHTTPResult{}, fmt.Errorf("close finance adapter response: %w", closeErr)
		}
		result := adapterHTTPResult{StatusCode: resp.StatusCode, Body: responseBody}
		if shouldRetryStatus(resp.StatusCode) && attempt < attempts-1 {
			sleepBeforeRetry(ctx, attempt)
			continue
		}
		return result, nil
	}
	if lastErr == nil {
		lastErr = ctx.Err()
	}
	return adapterHTTPResult{}, lastErr
}

func adapterAttempts(retryCount int) int {
	if retryCount < 0 {
		retryCount = 0
	}
	if retryCount > 5 {
		retryCount = 5
	}
	return retryCount + 1
}

func adapterTimeout(timeoutMS int) time.Duration {
	if timeoutMS <= 0 {
		return 30 * time.Second
	}
	return time.Duration(timeoutMS) * time.Millisecond
}

func shouldRetryStatus(status int) bool {
	return status == http.StatusTooManyRequests || status >= 500
}

func sleepBeforeRetry(ctx context.Context, attempt int) {
	timer := time.NewTimer(time.Duration(attempt+1) * 100 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func normalizeCreateAdapterInput(input *CreateAdapterInput) {
	input.Name = strings.TrimSpace(input.Name)
	input.EndpointURL = strings.TrimSpace(input.EndpointURL)
	if input.AuthType == "" {
		input.AuthType = AuthHMAC
	}
	if input.AdapterType == "" {
		input.AdapterType = "generic"
	}
	if input.Direction == "" {
		input.Direction = "bidirectional"
	}
	if input.Status == "" {
		input.Status = AdapterActive
	}
	if input.TimeoutMS <= 0 {
		input.TimeoutMS = 30000
	}
	if input.RetryCount < 0 {
		input.RetryCount = 0
	}
	if input.RetryCount == 0 {
		input.RetryCount = 3
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
	if input.FieldMapping == nil {
		input.FieldMapping = map[string]any{}
	}
	if input.PullConfig == nil {
		input.PullConfig = map[string]any{}
	}
}

func normalizeExportBatchInput(input *CreateExportBatchInput) error {
	start, err := parseDate(input.PeriodStart)
	if err != nil {
		return err
	}
	end, err := parseDate(input.PeriodEnd)
	if err != nil {
		return err
	}
	if end.Before(start) {
		return fmt.Errorf("%w: period_end must be on or after period_start", ErrValidation)
	}
	input.periodStartTime = start
	input.periodEndTime = end
	if input.Currency == "" {
		input.Currency = "CNY"
	}
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	if input.IdempotencyKey == "" {
		input.IdempotencyKey = BatchIdempotencyKey(input.AdapterID.String(), input.PeriodStart, input.PeriodEnd, input.Currency)
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
	return nil
}

func parseDate(raw string) (time.Time, error) {
	value, err := time.Parse("2006-01-02", strings.TrimSpace(raw))
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: date must use YYYY-MM-DD", ErrValidation)
	}
	return value.UTC(), nil
}

func validateEndpoint(endpoint string) error {
	parsed, err := url.ParseRequestURI(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("%w: endpoint_url must be an absolute URL", ErrValidation)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%w: endpoint_url scheme must be http or https", ErrValidation)
	}
	return nil
}

func validAuthType(authType string) bool {
	return authType == AuthHMAC || authType == AuthBearer
}

func validAdapterStatus(status string) bool {
	return status == AdapterActive || status == AdapterDisabled || status == AdapterError
}

func recordsFromPayload(payload map[string]any) []map[string]any {
	for _, key := range []string{"records", "expenses", "lines"} {
		raw, ok := payload[key].([]any)
		if !ok {
			continue
		}
		records := make([]map[string]any, 0, len(raw))
		for _, item := range raw {
			if record, ok := item.(map[string]any); ok {
				records = append(records, record)
			}
		}
		return records
	}
	return []map[string]any{payload}
}

func normalizeExpenseRecord(raw map[string]any, mapping map[string]any) (FinanceExpenseInput, financeExpenseDates, error) {
	value := func(field string) any {
		if external, ok := mapping[field].(string); ok && external != "" {
			if v, exists := raw[external]; exists {
				return v
			}
		}
		return raw[field]
	}
	input := FinanceExpenseInput{
		ExternalRecordID: stringAny(value("external_record_id")),
		ExpenseType:      firstNonEmpty(stringAny(value("expense_type")), "daily_expense"),
		CostCategory:     stringAny(value("cost_category")),
		Amount:           floatAny(value("amount")),
		Currency:         firstNonEmpty(stringAny(value("currency")), "CNY"),
		OccurredAt:       stringAny(value("occurred_at")),
		Description:      stringAny(value("description")),
		AccountCode:      stringAny(value("account_code")),
		AccountName:      stringAny(value("account_name")),
		CostCenterCode:   stringAny(value("cost_center_code")),
		CostCenterName:   stringAny(value("cost_center_name")),
		VendorID:         stringAny(value("vendor_id")),
		VendorName:       stringAny(value("vendor_name")),
		EmployeeID:       stringAny(value("employee_id")),
		EmployeeName:     stringAny(value("employee_name")),
		AgentName:        stringAny(value("agent_name")),
		TaxAmount:        floatAny(value("tax_amount")),
		TaxRate:          floatAny(value("tax_rate")),
		InvoiceNumber:    stringAny(value("invoice_number")),
		InvoiceDate:      stringAny(value("invoice_date")),
		PaymentStatus:    firstNonEmpty(stringAny(value("payment_status")), "unpaid"),
		PaymentDueDate:   stringAny(value("payment_due_date")),
		PaidAt:           stringAny(value("paid_at")),
		PeriodStart:      stringAny(value("period_start")),
		PeriodEnd:        stringAny(value("period_end")),
		Metadata:         map[string]any{"raw_source": "finance_import"},
	}
	if input.CostCategory == "" {
		input.CostCategory = costCategoryForExpense(input.ExpenseType)
	}
	input.AgentID = uuidAny(value("agent_id"))
	input.OrganizationID = uuidAny(value("organization_id"))
	input.DepartmentID = uuidAny(value("department_id"))
	input.RequirementID = uuidAny(value("requirement_id"))
	input.ProjectID = uuidAny(value("project_id"))
	input.WorkflowID = uuidAny(value("workflow_id"))
	input.TaskID = uuidAny(value("task_id"))
	input.CapabilityID = uuidAny(value("capability_id"))
	if input.ExternalRecordID == "" {
		return input, financeExpenseDates{}, fmt.Errorf("%w: external_record_id is required", ErrValidation)
	}
	if input.Amount == 0 {
		return input, financeExpenseDates{}, fmt.Errorf("%w: amount is required", ErrValidation)
	}
	dates, err := datesFromExpense(input)
	if err != nil {
		return input, dates, err
	}
	return input, dates, nil
}

func datesFromExpense(input FinanceExpenseInput) (financeExpenseDates, error) {
	occurredAt, err := parseOptionalTime(input.OccurredAt)
	if err != nil {
		return financeExpenseDates{}, err
	}
	invoiceDate, err := parseOptionalDate(input.InvoiceDate)
	if err != nil {
		return financeExpenseDates{}, err
	}
	dueDate, err := parseOptionalDate(input.PaymentDueDate)
	if err != nil {
		return financeExpenseDates{}, err
	}
	paidAt, err := parseOptionalTime(input.PaidAt)
	if err != nil {
		return financeExpenseDates{}, err
	}
	periodStart, err := parseOptionalDate(input.PeriodStart)
	if err != nil {
		return financeExpenseDates{}, err
	}
	periodEnd, err := parseOptionalDate(input.PeriodEnd)
	if err != nil {
		return financeExpenseDates{}, err
	}
	return financeExpenseDates{
		OccurredAt:     occurredAt,
		InvoiceDate:    invoiceDate,
		PaymentDueDate: dueDate,
		PaidAt:         paidAt,
		PeriodStart:    periodStart,
		PeriodEnd:      periodEnd,
	}, nil
}

func datesFromPayable(input CreatePayableInput) (financeExpenseDates, error) {
	invoiceDate, err := parseOptionalDate(input.InvoiceDate)
	if err != nil {
		return financeExpenseDates{}, err
	}
	dueDate, err := parseOptionalDate(input.DueDate)
	if err != nil {
		return financeExpenseDates{}, err
	}
	periodStart, err := parseOptionalDate(input.PeriodStart)
	if err != nil {
		return financeExpenseDates{}, err
	}
	periodEnd, err := parseOptionalDate(input.PeriodEnd)
	if err != nil {
		return financeExpenseDates{}, err
	}
	return financeExpenseDates{InvoiceDate: invoiceDate, PaymentDueDate: dueDate, PeriodStart: periodStart, PeriodEnd: periodEnd}, nil
}

func datesFromReceivable(input CreateReceivableInput) (financeExpenseDates, error) {
	invoiceDate, err := parseOptionalDate(input.InvoiceDate)
	if err != nil {
		return financeExpenseDates{}, err
	}
	dueDate, err := parseOptionalDate(input.DueDate)
	if err != nil {
		return financeExpenseDates{}, err
	}
	periodStart, err := parseOptionalDate(input.PeriodStart)
	if err != nil {
		return financeExpenseDates{}, err
	}
	periodEnd, err := parseOptionalDate(input.PeriodEnd)
	if err != nil {
		return financeExpenseDates{}, err
	}
	return financeExpenseDates{InvoiceDate: invoiceDate, PaymentDueDate: dueDate, PeriodStart: periodStart, PeriodEnd: periodEnd}, nil
}

func expenseOccurredAt(dates financeExpenseDates) time.Time {
	if dates.OccurredAt != nil {
		return *dates.OccurredAt
	}
	return time.Now()
}

func parseOptionalDate(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err == nil {
		return &parsed, nil
	}
	return parseOptionalTime(value)
}

func parseOptionalTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return &parsed, nil
	}
	if parsed, err := time.Parse("2006-01-02", value); err == nil {
		return &parsed, nil
	}
	return nil, fmt.Errorf("%w: invalid date/time %q", ErrValidation, value)
}

func costCategoryForExpense(expenseType string) string {
	switch strings.ToLower(strings.TrimSpace(expenseType)) {
	case "salary":
		return "human"
	case "model_fee":
		return "model_token"
	case "agent_fee":
		return "agent"
	case "project_expense", "daily_expense":
		return "finance"
	default:
		return "manual"
	}
}

func normalizeSettlementOrderInput(input *CreateSettlementOrderInput) {
	input.Currency = defaultCurrency(input.Currency)
	if input.Status == "" {
		input.Status = "draft"
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
}

func normalizeSettlementLineInput(input *CreateSettlementLineInput) {
	input.LineType = strings.TrimSpace(input.LineType)
	if input.LineType == "" {
		input.LineType = "manual"
	}
	if input.Quantity == 0 {
		input.Quantity = 1
	}
	if input.Amount == 0 && input.UnitPrice != 0 {
		input.Amount = input.Quantity * input.UnitPrice
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
}

func normalizePayableInput(input *CreatePayableInput) {
	if input.PayableType == "" {
		input.PayableType = "expense"
	}
	if input.SourceType == "" {
		input.SourceType = "manual"
	}
	if input.Currency == "" {
		input.Currency = "CNY"
	}
	if input.Status == "" {
		input.Status = "open"
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
}

func normalizeReceivableInput(input *CreateReceivableInput) {
	if input.ReceivableType == "" {
		input.ReceivableType = "project"
	}
	if input.SourceType == "" {
		input.SourceType = "manual"
	}
	input.Currency = defaultCurrency(input.Currency)
	if input.Status == "" {
		input.Status = "unpaid"
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
}

func normalizePaymentInput(input *CreatePaymentInput) {
	if input.Currency == "" {
		input.Currency = "CNY"
	}
	if input.Status == "" {
		input.Status = "paid"
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
}

func normalizeReceiptInput(input *CreateReceiptInput) {
	input.Currency = defaultCurrency(input.Currency)
	if input.Status == "" {
		input.Status = "received"
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
}

func applyTenantFinanceExpenseScope(ctx context.Context, input *FinanceExpenseInput) error {
	orgID := currentTenantOrganizationID(ctx)
	if orgID == nil {
		return nil
	}
	if input.OrganizationID != nil && *input.OrganizationID != *orgID {
		return fmt.Errorf("%w: finance expense organization must match current organization", ErrForbidden)
	}
	id := *orgID
	input.OrganizationID = &id
	return nil
}

func currentTenantOrganizationID(ctx context.Context) *uuid.UUID {
	tenant, ok := middleware.TenantFromContext(ctx)
	if !ok || tenant.OrganizationID == nil {
		return nil
	}
	id := *tenant.OrganizationID
	return &id
}

func ensureTenantOrganization(ctx context.Context, organizationID *uuid.UUID) error {
	orgID := currentTenantOrganizationID(ctx)
	if orgID == nil || organizationID == nil {
		return nil
	}
	if *orgID != *organizationID {
		return fmt.Errorf("%w: finance record is outside current organization", ErrForbidden)
	}
	return nil
}

func defaultCurrency(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return "CNY"
	}
	return value
}

func stringAny(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func floatAny(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		f, _ := v.Float64()
		return f
	case string:
		var out float64
		_, _ = fmt.Sscanf(strings.TrimSpace(v), "%f", &out)
		return out
	default:
		return 0
	}
}

func uuidAny(value any) *uuid.UUID {
	raw := stringAny(value)
	if raw == "" || raw == "<nil>" {
		return nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil
	}
	return &id
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" && strings.TrimSpace(value) != "<nil>" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func exportPayload(batch *ExportBatch) map[string]any {
	lines := make([]map[string]any, 0, len(batch.Lines))
	for _, line := range batch.Lines {
		lines = append(lines, map[string]any{
			"line_id":               line.ID.String(),
			"usage_ledger_id":       uuidString(line.UsageLedgerID),
			"cost_ledger_entry_id":  uuidString(line.CostLedgerEntryID),
			"project_cost_entry_id": uuidString(line.ProjectCostEntryID),
			"organization_id":       uuidString(line.OrganizationID),
			"department_id":         uuidString(line.DepartmentID),
			"project_id":            uuidString(line.ProjectID),
			"provider_id":           uuidString(line.ProviderID),
			"model_id":              uuidString(line.ModelID),
			"amount":                line.Amount,
			"currency":              line.Currency,
			"metadata":              line.Metadata,
		})
	}
	return map[string]any{
		"format_version":  "meta-org.finance.export.v1",
		"batch_id":        batch.ID.String(),
		"adapter_id":      batch.AdapterID.String(),
		"period_start":    batch.PeriodStart.Format("2006-01-02"),
		"period_end":      batch.PeriodEnd.Format("2006-01-02"),
		"currency":        batch.Currency,
		"total_amount":    batch.TotalAmount,
		"idempotency_key": batch.IdempotencyKey,
		"metadata":        batch.Metadata,
		"lines":           lines,
	}
}

func applyAuthHeaders(req *http.Request, adapter AdapterSecret, body []byte, idempotencyKey string) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", idempotencyKey)
	req.Header.Set("X-Meta-Org-Adapter-ID", adapter.ID.String())
	if adapter.AuthType == AuthBearer {
		req.Header.Set("Authorization", "Bearer "+adapter.Secret)
		return
	}
	req.Header.Set("X-Meta-Org-Signature", SignPayload(body, adapter.Secret))
}

func verifyAdapterCallback(adapter AdapterSecret, body []byte, signature string, authorization string) bool {
	if adapter.AuthType == AuthBearer {
		return strings.TrimSpace(authorization) == "Bearer "+adapter.Secret
	}
	return VerifyPayload(body, signature, adapter.Secret)
}

func mapExternalBatchStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "":
		return ""
	case "exported", "submitted":
		return BatchExported
	case "accepted", "acknowledged":
		return BatchAcknowledged
	case "posted":
		return BatchPosted
	case "reconciled":
		return BatchReconciled
	case "rejected", "failed", "error":
		return BatchFailed
	default:
		return ""
	}
}

func mapLineStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "posted", "reconciled", "failed", "rejected", "exported", "acknowledged":
		return strings.ToLower(strings.TrimSpace(status))
	default:
		return "ready"
	}
}

func uuidFromPayload(payload map[string]any, key string) *uuid.UUID {
	raw := stringFromPayload(payload, key, "")
	if raw == "" {
		return nil
	}
	parsed, err := uuid.Parse(raw)
	if err != nil {
		return nil
	}
	return &parsed
}

func stringFromPayload(payload map[string]any, key string, fallback string) string {
	if value, ok := payload[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return fallback
}

func firstNonEmptyString(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := stringFromPayload(payload, key, ""); value != "" {
			return value
		}
	}
	return ""
}

func externalAmountFromPayload(payload map[string]any) (any, bool) {
	for _, key := range []string{"external_amount", "reconciled_amount", "posted_amount"} {
		if value, ok := payload[key]; ok {
			return value, true
		}
	}
	return nil, false
}

func externalAmountFloatFromPayload(payload map[string]any) (float64, bool) {
	value, ok := externalAmountFromPayload(payload)
	if !ok {
		return 0, false
	}
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	default:
		return 0, false
	}
}

func uuidString(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

func (s *Service) startFinanceTrace(ctx context.Context, category string, adapterID uuid.UUID, batchID *uuid.UUID, metadata map[string]any) *observability.Trace {
	if s.observability == nil {
		return nil
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["category"] = category
	metadata["adapter_id"] = adapterID.String()
	metadata["batch_id"] = uuidString(batchID)
	trace, err := s.observability.StartTrace(ctx, nil, metadata)
	if err != nil {
		return nil
	}
	return trace
}

func (s *Service) recordFinanceSpan(ctx context.Context, trace *observability.Trace, spanType observability.SpanType, entityID *uuid.UUID, entityType string, input map[string]any, output map[string]any, durationMS int) {
	if s.observability == nil || trace == nil {
		return
	}
	_, _ = s.observability.RecordSpan(ctx, observability.RecordSpanInput{
		TraceID:    trace.ID,
		SpanType:   spanType,
		EntityID:   entityID,
		EntityType: entityType,
		Input:      input,
		Output:     output,
		DurationMs: durationMS,
		Metadata: map[string]any{
			"category": string(spanType),
		},
	})
}

func (s *Service) recordFinanceMetric(ctx context.Context, name string, entityID *uuid.UUID, entityType string, value float64, metadata map[string]any) {
	if s.observability == nil {
		return
	}
	metricType := observability.MetricHealth
	if name == "finance_reconciliation_difference" {
		metricType = observability.MetricCost
	}
	_, _ = s.observability.RecordMetric(ctx, observability.RecordMetricInput{
		MetricType: metricType,
		MetricName: name,
		EntityID:   entityID,
		EntityType: entityType,
		Value:      value,
		Metadata:   metadata,
	})
}

func (s *Service) completeObservationTrace(ctx context.Context, trace *observability.Trace, status observability.TraceStatus) {
	if s.observability == nil || trace == nil {
		return
	}
	_ = s.observability.CompleteTrace(ctx, trace.ID, string(status))
}
