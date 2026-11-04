import { ConversionResult, ConvertCostInput, CostBudget, CostLedgerEntry, CostRateCard, CostSummary, CreateCostBudgetInput, CreateCostLedgerEntryInput, CreateCostRateCardInput, CreateCurrencyInput, CreateExchangeRateInput, Currency, ExchangeRateVersion } from './types'
import { apiRequest } from './core'
export async function listCurrencies(token: string): Promise<Currency[]> {
  return apiRequest<Currency[]>('/costing/currencies', { token })
}

export async function upsertCurrency(token: string, input: CreateCurrencyInput): Promise<Currency> {
  return apiRequest<Currency>('/costing/currencies', { method: 'POST', token, body: input })
}

export async function voidCurrency(token: string, code: string): Promise<Currency> {
  return apiRequest<Currency>(`/costing/currencies/${encodeURIComponent(code)}/void`, { method: 'POST', token })
}

export async function listExchangeRates(token: string): Promise<ExchangeRateVersion[]> {
  return apiRequest<ExchangeRateVersion[]>('/costing/exchange-rates', { token })
}

export async function createExchangeRate(token: string, input: CreateExchangeRateInput): Promise<ExchangeRateVersion> {
  return apiRequest<ExchangeRateVersion>('/costing/exchange-rates', { method: 'POST', token, body: input })
}

export async function updateExchangeRate(token: string, id: string, input: CreateExchangeRateInput): Promise<ExchangeRateVersion> {
  return apiRequest<ExchangeRateVersion>(`/costing/exchange-rates/${id}`, { method: 'PATCH', token, body: input })
}

export async function voidExchangeRate(token: string, id: string): Promise<ExchangeRateVersion> {
  return apiRequest<ExchangeRateVersion>(`/costing/exchange-rates/${id}/void`, { method: 'POST', token })
}

export async function convertCostAmount(token: string, input: ConvertCostInput): Promise<ConversionResult> {
  return apiRequest<ConversionResult>('/costing/convert', { method: 'POST', token, body: input })
}

export async function listCostRateCards(token: string): Promise<CostRateCard[]> {
  return apiRequest<CostRateCard[]>('/costing/rate-cards', { token })
}

export async function createCostRateCard(token: string, input: CreateCostRateCardInput): Promise<CostRateCard> {
  return apiRequest<CostRateCard>('/costing/rate-cards', { method: 'POST', token, body: input })
}

export async function updateCostRateCard(token: string, id: string, input: CreateCostRateCardInput): Promise<CostRateCard> {
  return apiRequest<CostRateCard>(`/costing/rate-cards/${id}`, { method: 'PATCH', token, body: input })
}

export async function voidCostRateCard(token: string, id: string): Promise<CostRateCard> {
  return apiRequest<CostRateCard>(`/costing/rate-cards/${id}/void`, { method: 'POST', token })
}

export async function listCostBudgets(token: string): Promise<CostBudget[]> {
  return apiRequest<CostBudget[]>('/costing/budgets', { token })
}

export async function createCostBudget(token: string, input: CreateCostBudgetInput): Promise<CostBudget> {
  return apiRequest<CostBudget>('/costing/budgets', { method: 'POST', token, body: input })
}

export async function updateCostBudget(token: string, id: string, input: CreateCostBudgetInput): Promise<CostBudget> {
  return apiRequest<CostBudget>(`/costing/budgets/${id}`, { method: 'PATCH', token, body: input })
}

export async function voidCostBudget(token: string, id: string): Promise<CostBudget> {
  return apiRequest<CostBudget>(`/costing/budgets/${id}/void`, { method: 'POST', token })
}

export async function listCostLedgerEntries(token: string): Promise<CostLedgerEntry[]> {
  return apiRequest<CostLedgerEntry[]>('/costing/ledger-entries', { token })
}

export async function createCostLedgerEntry(token: string, input: CreateCostLedgerEntryInput): Promise<CostLedgerEntry> {
  return apiRequest<CostLedgerEntry>('/costing/ledger-entries', { method: 'POST', token, body: input })
}

export async function updateCostLedgerEntry(token: string, id: string, input: CreateCostLedgerEntryInput): Promise<CostLedgerEntry> {
  return apiRequest<CostLedgerEntry>(`/costing/ledger-entries/${id}`, { method: 'PATCH', token, body: input })
}

export async function voidCostLedgerEntry(token: string, id: string): Promise<CostLedgerEntry> {
  return apiRequest<CostLedgerEntry>(`/costing/ledger-entries/${id}/void`, { method: 'POST', token })
}

export async function getCostSummary(token: string): Promise<CostSummary> {
  return apiRequest<CostSummary>('/costing/summary', { token })
}
