package finance

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/dberrors"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterPublicRoutes(r chi.Router) {
	r.Post("/finance/webhooks/{adapterID}", h.receiveWebhook)
	r.Post("/finance/imports/webhooks/{adapterID}", h.receiveImportWebhook)
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/finance/adapters", h.createAdapter)
	r.Get("/finance/adapters", h.listAdapters)
	r.Patch("/finance/adapters/{id}", h.updateAdapter)
	r.Post("/finance/adapters/{id}/test", h.testAdapter)
	r.Post("/finance/export-batches", h.createExportBatch)
	r.Get("/finance/export-batches", h.listExportBatches)
	r.Get("/finance/export-batches/{id}", h.getExportBatch)
	r.Post("/finance/export-batches/{id}/submit", h.submitExportBatch)
	r.Post("/finance/accounting-batches", h.createExportBatch)
	r.Get("/finance/accounting-batches", h.listExportBatches)
	r.Get("/finance/accounting-batches/{id}", h.getExportBatch)
	r.Post("/finance/accounting-batches/{id}/submit", h.submitExportBatch)
	r.Get("/finance/reconciliation", h.listReconciliation)
	r.Post("/finance/gl/accounts", h.createGLAccount)
	r.Get("/finance/gl/accounts", h.listGLAccounts)
	r.Post("/finance/gl/cost-centers", h.createGLCostCenter)
	r.Get("/finance/gl/cost-centers", h.listGLCostCenters)
	r.Post("/finance/gl/journal-entries", h.createGLJournalEntry)
	r.Get("/finance/gl/journal-entries", h.listGLJournalEntries)
	r.Get("/finance/gl/journal-entries/{id}", h.getGLJournalEntry)
	r.Post("/finance/gl/journal-entries/{id}/post", h.postGLJournalEntry)
	r.Get("/finance/gl/trial-balance", h.getGLTrialBalance)
	r.Post("/finance/imports", h.importExpenses)
	r.Post("/finance/imports/files", h.importExpenseFile)
	r.Post("/finance/imports/{adapterID}/pull", h.pullExpenses)
	r.Get("/finance/import-batches", h.listImportBatches)
	r.Get("/finance/import-records", h.listImportRecords)
	r.Post("/finance/settlement-orders", h.createSettlementOrder)
	r.Get("/finance/settlement-orders", h.listSettlementOrders)
	r.Get("/finance/settlement-orders/{id}", h.getSettlementOrder)
	r.Patch("/finance/settlement-orders/{id}", h.updateSettlementOrder)
	r.Post("/finance/settlement-orders/{id}/post", h.postSettlementOrder)
	r.Post("/finance/settlement-orders/{id}/void", h.voidSettlementOrder)
	r.Post("/finance/receivables", h.createReceivable)
	r.Get("/finance/receivables", h.listReceivables)
	r.Patch("/finance/receivables/{id}", h.updateReceivable)
	r.Post("/finance/receivables/{id}/void", h.voidReceivable)
	r.Post("/finance/receipts", h.createReceipt)
	r.Get("/finance/receipts", h.listReceipts)
	r.Post("/finance/receipts/{id}/allocate", h.allocateReceipt)
	r.Post("/finance/payables", h.createPayable)
	r.Get("/finance/payables", h.listPayables)
	r.Patch("/finance/payables/{id}", h.updatePayable)
	r.Post("/finance/payables/{id}/void", h.voidPayable)
	r.Post("/finance/payments", h.createPayment)
	r.Get("/finance/payments", h.listPayments)
	r.Patch("/finance/payments/{id}", h.updatePayment)
	r.Post("/finance/payments/{id}/void", h.voidPayment)
	r.Post("/finance/payments/{id}/allocate", h.allocatePayment)
}

func (h *Handler) createAdapter(w http.ResponseWriter, r *http.Request) {
	var input CreateAdapterInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateAdapter(r.Context(), input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) listAdapters(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListAdapters(r.Context(), queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) updateAdapter(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var input UpdateAdapterInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.UpdateAdapter(r.Context(), id, input)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) testAdapter(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	err := h.service.TestAdapter(r.Context(), id)
	writeResult(w, http.StatusOK, map[string]string{"status": "ok"}, err)
}

func (h *Handler) createExportBatch(w http.ResponseWriter, r *http.Request) {
	var input CreateExportBatchInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateExportBatch(r.Context(), input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) listExportBatches(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListExportBatches(r.Context(), queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) getExportBatch(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	result, err := h.service.GetExportBatch(r.Context(), id)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) submitExportBatch(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	result, err := h.service.SubmitExportBatch(r.Context(), id)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) receiveWebhook(w http.ResponseWriter, r *http.Request) {
	adapterID, ok := parseID(w, r, "adapterID")
	if !ok {
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 2<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	result, err := h.service.ReceiveWebhook(
		r.Context(),
		adapterID,
		body,
		firstHeader(r, "X-Meta-Org-Signature", "X-Hub-Signature-256"),
		r.Header.Get("Authorization"),
	)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) listReconciliation(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListReconciliation(r.Context(), queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) createGLAccount(w http.ResponseWriter, r *http.Request) {
	var input CreateGLAccountInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateGLAccount(r.Context(), input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) listGLAccounts(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListGLAccounts(r.Context(), queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) createGLCostCenter(w http.ResponseWriter, r *http.Request) {
	var input CreateGLCostCenterInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateGLCostCenter(r.Context(), input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) listGLCostCenters(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListGLCostCenters(r.Context(), queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) createGLJournalEntry(w http.ResponseWriter, r *http.Request) {
	var input CreateGLJournalEntryInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateGLJournalEntry(r.Context(), input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) listGLJournalEntries(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListGLJournalEntries(r.Context(), queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) getGLJournalEntry(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	result, err := h.service.GetGLJournalEntry(r.Context(), id)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) postGLJournalEntry(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	result, err := h.service.PostGLJournalEntry(r.Context(), id)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) getGLTrialBalance(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	input := GLTrialBalanceInput{
		PeriodStart: query.Get("period_start"),
		PeriodEnd:   query.Get("period_end"),
		Currency:    query.Get("currency"),
	}
	if raw := query.Get("organization_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid organization_id"})
			return
		}
		input.OrganizationID = &id
	}
	result, err := h.service.GetGLTrialBalance(r.Context(), input)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) importExpenses(w http.ResponseWriter, r *http.Request) {
	var input ImportExpensesInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.ImportExpenses(r.Context(), input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) receiveImportWebhook(w http.ResponseWriter, r *http.Request) {
	adapterID, ok := parseID(w, r, "adapterID")
	if !ok {
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 4<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	result, err := h.service.ReceiveExpenseWebhook(
		r.Context(),
		adapterID,
		body,
		firstHeader(r, "X-Meta-Org-Signature", "X-Hub-Signature-256"),
		r.Header.Get("Authorization"),
	)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) importExpenseFile(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid multipart form"})
		return
	}
	adapterID, err := uuid.Parse(r.FormValue("adapter_id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid adapter_id"})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file field is required"})
		return
	}
	defer file.Close()
	records, err := csvRecords(file)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	result, err := h.service.ImportExpenses(r.Context(), ImportExpensesInput{
		AdapterID:  adapterID,
		SourceType: "file",
		FileName:   header.Filename,
		Records:    records,
		Metadata:   map[string]any{"content_type": header.Header.Get("Content-Type")},
	})
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) pullExpenses(w http.ResponseWriter, r *http.Request) {
	adapterID, ok := parseID(w, r, "adapterID")
	if !ok {
		return
	}
	result, err := h.service.PullAdapterExpenses(r.Context(), adapterID)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) listImportBatches(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListImportBatches(r.Context(), queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) listImportRecords(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListImportRecords(r.Context(), queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) createSettlementOrder(w http.ResponseWriter, r *http.Request) {
	var input CreateSettlementOrderInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateSettlementOrder(r.Context(), input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) listSettlementOrders(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListSettlementOrders(r.Context(), queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) getSettlementOrder(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	result, err := h.service.GetSettlementOrder(r.Context(), id)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) updateSettlementOrder(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var input UpdateSettlementOrderInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.UpdateSettlementOrder(r.Context(), id, input)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) postSettlementOrder(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	result, err := h.service.PostSettlementOrder(r.Context(), id)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) voidSettlementOrder(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	result, err := h.service.VoidSettlementOrder(r.Context(), id, reasonFromBody(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) createReceivable(w http.ResponseWriter, r *http.Request) {
	var input CreateReceivableInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateReceivable(r.Context(), input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) listReceivables(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListReceivables(r.Context(), queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) updateReceivable(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var input UpdateReceivableInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.UpdateReceivable(r.Context(), id, input)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) voidReceivable(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	result, err := h.service.VoidReceivable(r.Context(), id, reasonFromBody(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) createReceipt(w http.ResponseWriter, r *http.Request) {
	var input CreateReceiptInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreateReceipt(r.Context(), input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) listReceipts(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListReceipts(r.Context(), queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) allocateReceipt(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var input AllocateReceiptInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.AllocateReceipt(r.Context(), id, input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) createPayable(w http.ResponseWriter, r *http.Request) {
	var input CreatePayableInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreatePayable(r.Context(), input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) listPayables(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListPayables(r.Context(), queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) updatePayable(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var input UpdatePayableInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.UpdatePayable(r.Context(), id, input)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) voidPayable(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	result, err := h.service.VoidPayable(r.Context(), id, reasonFromBody(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) createPayment(w http.ResponseWriter, r *http.Request) {
	var input CreatePaymentInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.CreatePayment(r.Context(), input)
	writeResult(w, http.StatusCreated, result, err)
}

func (h *Handler) listPayments(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListPayments(r.Context(), queryLimit(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) updatePayment(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var input UpdatePaymentInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.UpdatePayment(r.Context(), id, input)
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) voidPayment(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	result, err := h.service.VoidPayment(r.Context(), id, reasonFromBody(r))
	writeResult(w, http.StatusOK, result, err)
}

func (h *Handler) allocatePayment(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var input AllocatePaymentInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.service.AllocatePayment(r.Context(), id, input)
	writeResult(w, http.StatusCreated, result, err)
}

func reasonFromBody(r *http.Request) string {
	var input struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&input)
	return input.Reason
}

func csvRecords(reader io.Reader) ([]map[string]any, error) {
	csvReader := csv.NewReader(reader)
	csvReader.TrimLeadingSpace = true
	rows, err := csvReader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 {
		return nil, errors.New("csv requires a header row and at least one data row")
	}
	headers := rows[0]
	records := make([]map[string]any, 0, len(rows)-1)
	for _, row := range rows[1:] {
		record := map[string]any{}
		for i, header := range headers {
			if i < len(row) {
				record[header] = row[i]
			}
		}
		records = append(records, record)
	}
	return records, nil
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dest any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(dest); err != nil {
		if errors.Is(err, io.EOF) {
			return true
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return false
	}
	return true
}

func parseID(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return uuid.Nil, false
	}
	return id, true
}

func queryLimit(r *http.Request) int {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	return limit
}

func writeResult(w http.ResponseWriter, successStatus int, payload any, err error) {
	if err != nil {
		writeJSON(w, statusFromError(err), map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, successStatus, payload)
}

func statusFromError(err error) int {
	switch {
	case errors.Is(err, ErrValidation):
		return http.StatusBadRequest
	case dberrors.IsUniqueViolation(err):
		return http.StatusConflict
	case errors.Is(err, ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, ErrNotFound), errors.Is(err, pgx.ErrNoRows):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("finance writeJSON error: %v", err)
	}
}

func firstHeader(r *http.Request, names ...string) string {
	for _, name := range names {
		if value := r.Header.Get(name); value != "" {
			return value
		}
	}
	return ""
}
