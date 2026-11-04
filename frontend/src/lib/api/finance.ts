import { AllocateFinancePaymentInput, AllocateFinanceReceiptInput, CreateFinanceAdapterInput, CreateFinanceExportBatchInput, CreateFinanceGLAccountInput, CreateFinanceGLCostCenterInput, CreateFinanceGLJournalEntryInput, CreateFinancePayableInput, CreateFinancePaymentInput, CreateFinanceReceiptInput, CreateFinanceReceivableInput, CreateFinanceSettlementOrderInput, FinanceAdapter, FinanceExportBatch, FinanceGLAccount, FinanceGLCostCenter, FinanceGLJournalEntry, FinanceGLTrialBalance, FinanceGLTrialBalanceInput, FinanceImportBatch, FinanceImportRecord, FinanceImportResult, FinancePayable, FinancePayment, FinancePaymentAllocation, FinanceReceipt, FinanceReceiptAllocation, FinanceReceivable, FinanceReconciliationItem, FinanceSettlementOrder, ImportFinanceExpensesInput } from './types'
import { apiRequest } from './core'
export async function listFinanceAdapters(token: string): Promise<FinanceAdapter[]> {
  return apiRequest<FinanceAdapter[]>('/finance/adapters', { token })
}

export async function createFinanceAdapter(token: string, input: CreateFinanceAdapterInput): Promise<FinanceAdapter> {
  return apiRequest<FinanceAdapter>('/finance/adapters', { method: 'POST', token, body: input })
}

export async function testFinanceAdapter(token: string, id: string): Promise<{ status: string }> {
  return apiRequest<{ status: string }>(`/finance/adapters/${id}/test`, { method: 'POST', token })
}

export async function createFinanceExportBatch(
  token: string,
  input: CreateFinanceExportBatchInput,
): Promise<FinanceExportBatch> {
  return apiRequest<FinanceExportBatch>('/finance/accounting-batches', { method: 'POST', token, body: input })
}

export async function listFinanceExportBatches(token: string): Promise<FinanceExportBatch[]> {
  return apiRequest<FinanceExportBatch[]>('/finance/accounting-batches', { token })
}

export async function getFinanceExportBatch(token: string, id: string): Promise<FinanceExportBatch> {
  return apiRequest<FinanceExportBatch>(`/finance/accounting-batches/${id}`, { token })
}

export async function submitFinanceExportBatch(token: string, id: string): Promise<FinanceExportBatch> {
  return apiRequest<FinanceExportBatch>(`/finance/accounting-batches/${id}/submit`, { method: 'POST', token })
}

export async function listFinanceReconciliation(token: string): Promise<FinanceReconciliationItem[]> {
  return apiRequest<FinanceReconciliationItem[]>('/finance/reconciliation', { token })
}

export async function createFinanceGLAccount(token: string, input: CreateFinanceGLAccountInput): Promise<FinanceGLAccount> {
  return apiRequest<FinanceGLAccount>('/finance/gl/accounts', { method: 'POST', token, body: input })
}

export async function listFinanceGLAccounts(token: string): Promise<FinanceGLAccount[]> {
  return apiRequest<FinanceGLAccount[]>('/finance/gl/accounts', { token })
}

export async function createFinanceGLCostCenter(token: string, input: CreateFinanceGLCostCenterInput): Promise<FinanceGLCostCenter> {
  return apiRequest<FinanceGLCostCenter>('/finance/gl/cost-centers', { method: 'POST', token, body: input })
}

export async function listFinanceGLCostCenters(token: string): Promise<FinanceGLCostCenter[]> {
  return apiRequest<FinanceGLCostCenter[]>('/finance/gl/cost-centers', { token })
}

export async function createFinanceGLJournalEntry(token: string, input: CreateFinanceGLJournalEntryInput): Promise<FinanceGLJournalEntry> {
  return apiRequest<FinanceGLJournalEntry>('/finance/gl/journal-entries', { method: 'POST', token, body: input })
}

export async function listFinanceGLJournalEntries(token: string): Promise<FinanceGLJournalEntry[]> {
  return apiRequest<FinanceGLJournalEntry[]>('/finance/gl/journal-entries', { token })
}

export async function getFinanceGLJournalEntry(token: string, id: string): Promise<FinanceGLJournalEntry> {
  return apiRequest<FinanceGLJournalEntry>(`/finance/gl/journal-entries/${encodeURIComponent(id)}`, { token })
}

export async function postFinanceGLJournalEntry(token: string, id: string): Promise<FinanceGLJournalEntry> {
  return apiRequest<FinanceGLJournalEntry>(`/finance/gl/journal-entries/${encodeURIComponent(id)}/post`, { method: 'POST', token })
}

export async function getFinanceGLTrialBalance(token: string, input: FinanceGLTrialBalanceInput = {}): Promise<FinanceGLTrialBalance> {
  const params = new URLSearchParams()
  if (input.organization_id) params.set('organization_id', input.organization_id)
  if (input.period_start) params.set('period_start', input.period_start)
  if (input.period_end) params.set('period_end', input.period_end)
  if (input.currency) params.set('currency', input.currency)
  const query = params.toString()
  return apiRequest<FinanceGLTrialBalance>(`/finance/gl/trial-balance${query ? `?${query}` : ''}`, { token })
}

export async function importFinanceExpenses(token: string, input: ImportFinanceExpensesInput): Promise<FinanceImportResult> {
  return apiRequest<FinanceImportResult>('/finance/imports', { method: 'POST', token, body: input })
}

export async function importFinanceExpenseFile(token: string, adapterID: string, file: File): Promise<FinanceImportResult> {
  const form = new FormData()
  form.append('adapter_id', adapterID)
  form.append('file', file)
  return apiRequest<FinanceImportResult>('/finance/imports/files', { method: 'POST', token, body: form })
}

export async function pullFinanceExpenses(token: string, adapterID: string): Promise<FinanceImportResult> {
  return apiRequest<FinanceImportResult>(`/finance/imports/${adapterID}/pull`, { method: 'POST', token })
}

export async function listFinanceImportBatches(token: string): Promise<FinanceImportBatch[]> {
  return apiRequest<FinanceImportBatch[]>('/finance/import-batches', { token })
}

export async function listFinanceImportRecords(token: string): Promise<FinanceImportRecord[]> {
  return apiRequest<FinanceImportRecord[]>('/finance/import-records', { token })
}

export async function createFinanceSettlementOrder(token: string, input: CreateFinanceSettlementOrderInput): Promise<FinanceSettlementOrder> {
  return apiRequest<FinanceSettlementOrder>('/finance/settlement-orders', { method: 'POST', token, body: input })
}

export async function listFinanceSettlementOrders(token: string): Promise<FinanceSettlementOrder[]> {
  return apiRequest<FinanceSettlementOrder[]>('/finance/settlement-orders', { token })
}

export async function updateFinanceSettlementOrder(token: string, id: string, input: CreateFinanceSettlementOrderInput): Promise<FinanceSettlementOrder> {
  return apiRequest<FinanceSettlementOrder>(`/finance/settlement-orders/${id}`, { method: 'PATCH', token, body: input })
}

export async function postFinanceSettlementOrder(token: string, id: string): Promise<FinanceReceivable> {
  return apiRequest<FinanceReceivable>(`/finance/settlement-orders/${id}/post`, { method: 'POST', token })
}

export async function voidFinanceSettlementOrder(token: string, id: string, reason: string): Promise<FinanceSettlementOrder> {
  return apiRequest<FinanceSettlementOrder>(`/finance/settlement-orders/${id}/void`, { method: 'POST', token, body: { reason } })
}

export async function createFinanceReceivable(token: string, input: CreateFinanceReceivableInput): Promise<FinanceReceivable> {
  return apiRequest<FinanceReceivable>('/finance/receivables', { method: 'POST', token, body: input })
}

export async function listFinanceReceivables(token: string): Promise<FinanceReceivable[]> {
  return apiRequest<FinanceReceivable[]>('/finance/receivables', { token })
}

export async function updateFinanceReceivable(token: string, id: string, input: CreateFinanceReceivableInput): Promise<FinanceReceivable> {
  return apiRequest<FinanceReceivable>(`/finance/receivables/${id}`, { method: 'PATCH', token, body: input })
}

export async function voidFinanceReceivable(token: string, id: string, reason: string): Promise<FinanceReceivable> {
  return apiRequest<FinanceReceivable>(`/finance/receivables/${id}/void`, { method: 'POST', token, body: { reason } })
}

export async function createFinanceReceipt(token: string, input: CreateFinanceReceiptInput): Promise<FinanceReceipt> {
  return apiRequest<FinanceReceipt>('/finance/receipts', { method: 'POST', token, body: input })
}

export async function listFinanceReceipts(token: string): Promise<FinanceReceipt[]> {
  return apiRequest<FinanceReceipt[]>('/finance/receipts', { token })
}

export async function allocateFinanceReceipt(token: string, receiptID: string, input: AllocateFinanceReceiptInput): Promise<FinanceReceiptAllocation> {
  return apiRequest<FinanceReceiptAllocation>(`/finance/receipts/${receiptID}/allocate`, { method: 'POST', token, body: input })
}

export async function createFinancePayable(token: string, input: CreateFinancePayableInput): Promise<FinancePayable> {
  return apiRequest<FinancePayable>('/finance/payables', { method: 'POST', token, body: input })
}

export async function listFinancePayables(token: string): Promise<FinancePayable[]> {
  return apiRequest<FinancePayable[]>('/finance/payables', { token })
}

export async function updateFinancePayable(token: string, id: string, input: CreateFinancePayableInput): Promise<FinancePayable> {
  return apiRequest<FinancePayable>(`/finance/payables/${id}`, { method: 'PATCH', token, body: input })
}

export async function voidFinancePayable(token: string, id: string, reason: string): Promise<FinancePayable> {
  return apiRequest<FinancePayable>(`/finance/payables/${id}/void`, { method: 'POST', token, body: { reason } })
}

export async function createFinancePayment(token: string, input: CreateFinancePaymentInput): Promise<FinancePayment> {
  return apiRequest<FinancePayment>('/finance/payments', { method: 'POST', token, body: input })
}

export async function listFinancePayments(token: string): Promise<FinancePayment[]> {
  return apiRequest<FinancePayment[]>('/finance/payments', { token })
}

export async function updateFinancePayment(token: string, id: string, input: CreateFinancePaymentInput): Promise<FinancePayment> {
  return apiRequest<FinancePayment>(`/finance/payments/${id}`, { method: 'PATCH', token, body: input })
}

export async function voidFinancePayment(token: string, id: string, reason: string): Promise<FinancePayment> {
  return apiRequest<FinancePayment>(`/finance/payments/${id}/void`, { method: 'POST', token, body: { reason } })
}

export async function allocateFinancePayment(token: string, paymentID: string, input: AllocateFinancePaymentInput): Promise<FinancePaymentAllocation> {
  return apiRequest<FinancePaymentAllocation>(`/finance/payments/${paymentID}/allocate`, { method: 'POST', token, body: input })
}
