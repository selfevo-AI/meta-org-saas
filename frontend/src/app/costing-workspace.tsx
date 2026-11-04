'use client'

import { BadgeDollarSign, Ban, Calculator, Coins, Gauge, Layers3, ListChecks, Pencil, RefreshCw, WalletCards } from 'lucide-react'
import { FormEvent, useCallback, useEffect, useMemo, useState } from 'react'
import type { ReactNode } from 'react'
import {
  convertCostAmount,
  createCostBudget,
  createCostLedgerEntry,
  createCostRateCard,
  createExchangeRate,
  getCostSummary,
  listCostBudgets,
  listCostLedgerEntries,
  listCostRateCards,
  listCurrencies,
  listExchangeRates,
  updateCostBudget,
  updateCostLedgerEntry,
  updateCostRateCard,
  updateExchangeRate,
  upsertCurrency,
  voidCostBudget,
  voidCostLedgerEntry,
  voidCostRateCard,
  voidCurrency,
  voidExchangeRate,
  type ConversionResult,
  type CostBudget,
  type CostLedgerEntry,
  type CostRateCard,
  type CostSummary,
  type Currency,
  type ExchangeRateVersion,
} from '@/lib/api'
import { useI18n } from '@/lib/i18n'
import { describeApiError } from '@/lib/api-error'

interface CostingWorkspaceProps {
  token: string
}

type TabID = 'summary' | 'currencies' | 'rates' | 'rateCards' | 'budgets' | 'ledger'

const tabs: Array<{ id: TabID; label: string; icon: typeof Gauge }> = [
  { id: 'summary', label: 'costing.summary', icon: Gauge },
  { id: 'currencies', label: 'costing.currencies', icon: Coins },
  { id: 'rates', label: 'costing.exchangeRates', icon: Calculator },
  { id: 'rateCards', label: 'costing.rateCards', icon: BadgeDollarSign },
  { id: 'budgets', label: 'costing.budgets', icon: WalletCards },
  { id: 'ledger', label: 'costing.ledger', icon: ListChecks },
]

const currencyTypes = ['fiat', 'virtual']
const costCategories = ['human', 'resource', 'agent', 'model_token', 'capability', 'tool', 'finance', 'adjustment', 'manual']
const subjectTypes = ['human', 'external_human', 'agent', 'resource', 'capability', 'tool']
const rateTypes = ['hourly', 'daily', 'monthly', 'token', 'unit', 'fixed']
const scopeTypes = ['organization', 'department', 'requirement', 'project', 'capability', 'workflow', 'task']

function money(value: number | undefined, currency = 'CNY'): string {
  return `${currency} ${Number(value ?? 0).toFixed(4)}`
}

function dateText(value?: string): string {
  if (!value) return ''
  return new Date(value).toLocaleString()
}

function numberValue(value: string): number {
  return Number(value || 0)
}

export function CostingWorkspace({ token }: CostingWorkspaceProps) {
  const { t } = useI18n()
  const [activeTab, setActiveTab] = useState<TabID>('summary')
  const [currencies, setCurrencies] = useState<Currency[]>([])
  const [exchangeRates, setExchangeRates] = useState<ExchangeRateVersion[]>([])
  const [rateCards, setRateCards] = useState<CostRateCard[]>([])
  const [budgets, setBudgets] = useState<CostBudget[]>([])
  const [ledgerEntries, setLedgerEntries] = useState<CostLedgerEntry[]>([])
  const [summary, setSummary] = useState<CostSummary | null>(null)
  const [conversion, setConversion] = useState<ConversionResult | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [currencyForm, setCurrencyForm] = useState({
    code: '',
    name: '',
    currency_type: 'fiat',
    symbol: '',
    precision_digits: '2',
    chain_id: '',
    contract_address: '',
    external_source: '',
  })
  const [rateForm, setRateForm] = useState({
    id: '',
    from_currency: 'USD',
    to_currency: 'CNY',
    rate: '7.2',
    source: 'manual',
    provider: '',
    external_rate_id: '',
  })
  const [convertForm, setConvertForm] = useState({
    amount: '1',
    from_currency: 'USD',
    to_currency: 'CNY',
  })
  const [rateCardForm, setRateCardForm] = useState({
    id: '',
    subject_type: 'human',
    rate_type: 'hourly',
    amount: '0',
    currency: 'CNY',
    scope_type: '',
  })
  const [budgetForm, setBudgetForm] = useState({
    id: '',
    scope_type: 'project',
    amount: '0',
    currency: 'CNY',
    status: 'active',
  })
  const [ledgerForm, setLedgerForm] = useState({
    id: '',
    cost_category: 'manual',
    source_type: 'manual',
    amount: '0',
    currency: 'CNY',
    description: '',
  })

  const currencyOptions = useMemo(() => {
    const codes = currencies.map((currency) => currency.code)
    return codes.length > 0 ? codes : ['CNY', 'USD']
  }, [currencies])

  const loadCosting = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const [currencyData, rateData, cardData, budgetData, ledgerData, summaryData] = await Promise.all([
        listCurrencies(token),
        listExchangeRates(token),
        listCostRateCards(token),
        listCostBudgets(token),
        listCostLedgerEntries(token),
        getCostSummary(token),
      ])
      setCurrencies(currencyData)
      setExchangeRates(rateData)
      setRateCards(cardData)
      setBudgets(budgetData)
      setLedgerEntries(ledgerData)
      setSummary(summaryData)
    } catch (err) {
      setError(describeApiError(err, t, t('costing.loadFailed')))
    } finally {
      setLoading(false)
    }
  }, [t, token])

  useEffect(() => {
    const timer = window.setTimeout(() => {
      void loadCosting()
    }, 0)
    return () => window.clearTimeout(timer)
  }, [loadCosting])

  async function run(action: () => Promise<void>, success: string) {
    setLoading(true)
    setError('')
    setNotice('')
    try {
      await action()
      setNotice(t(success))
      await loadCosting()
    } catch (err) {
      setError(describeApiError(err, t, t('common.operationFailed')))
    } finally {
      setLoading(false)
    }
  }

  async function submitCurrency(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await run(
      () =>
        upsertCurrency(token, {
          code: currencyForm.code,
          name: currencyForm.name,
          currency_type: currencyForm.currency_type,
          symbol: currencyForm.symbol,
          precision_digits: numberValue(currencyForm.precision_digits),
          chain_id: currencyForm.chain_id,
          contract_address: currencyForm.contract_address,
          external_source: currencyForm.external_source,
          is_active: true,
          metadata: {},
        }).then(() => setCurrencyForm((current) => ({ ...current, code: '', name: '', symbol: '' }))),
      'costing.currencySaved',
    )
  }

  async function submitRate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const input = {
      from_currency: rateForm.from_currency,
      to_currency: rateForm.to_currency,
      rate: numberValue(rateForm.rate),
      source: rateForm.source,
      provider: rateForm.provider,
      external_rate_id: rateForm.external_rate_id,
      metadata: {},
    }
    await run(
      () =>
        (rateForm.id ? updateExchangeRate(token, rateForm.id, input) : createExchangeRate(token, input)).then(() =>
          setRateForm((current) => ({ ...current, id: '' })),
        ),
      'costing.rateSaved',
    )
  }

  async function submitConversion(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await run(
      () =>
        convertCostAmount(token, {
          amount: numberValue(convertForm.amount),
          from_currency: convertForm.from_currency,
          to_currency: convertForm.to_currency,
        }).then((result) => setConversion(result)),
      'costing.converted',
    )
  }

  async function submitRateCard(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const input = {
      subject_type: rateCardForm.subject_type,
      rate_type: rateCardForm.rate_type,
      amount: numberValue(rateCardForm.amount),
      currency: rateCardForm.currency,
      scope_type: rateCardForm.scope_type,
      status: 'active',
      metadata: {},
    }
    await run(
      () =>
        (rateCardForm.id ? updateCostRateCard(token, rateCardForm.id, input) : createCostRateCard(token, input)).then(() =>
          setRateCardForm((current) => ({ ...current, id: '', amount: '0' })),
        ),
      'costing.rateCardSaved',
    )
  }

  async function submitBudget(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const input = {
      scope_type: budgetForm.scope_type,
      amount: numberValue(budgetForm.amount),
      currency: budgetForm.currency,
      status: budgetForm.status,
      metadata: {},
    }
    await run(
      () =>
        (budgetForm.id ? updateCostBudget(token, budgetForm.id, input) : createCostBudget(token, input)).then(() =>
          setBudgetForm((current) => ({ ...current, id: '', amount: '0' })),
        ),
      'costing.budgetSaved',
    )
  }

  async function submitLedger(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const input = {
      ledger_type: 'actual',
      cost_category: ledgerForm.cost_category,
      source_type: ledgerForm.source_type,
      amount: numberValue(ledgerForm.amount),
      currency: ledgerForm.currency,
      status: 'posted',
      description: ledgerForm.description,
      metadata: { source_ui: 'costing_workspace' },
    }
    await run(
      () =>
        (ledgerForm.id ? updateCostLedgerEntry(token, ledgerForm.id, input) : createCostLedgerEntry(token, input)).then(() =>
          setLedgerForm((current) => ({ ...current, id: '', amount: '0', description: '' })),
        ),
      'costing.ledgerSaved',
    )
  }

  function editCurrency(currency: Currency) {
    setCurrencyForm({
      code: currency.code,
      name: currency.name,
      currency_type: currency.currency_type,
      symbol: currency.symbol,
      precision_digits: String(currency.precision_digits),
      chain_id: currency.chain_id ?? '',
      contract_address: currency.contract_address ?? '',
      external_source: currency.external_source ?? '',
    })
  }

  function editRate(rate: ExchangeRateVersion) {
    setRateForm({
      id: rate.id,
      from_currency: rate.from_currency,
      to_currency: rate.to_currency,
      rate: String(rate.rate),
      source: rate.source,
      provider: rate.provider ?? '',
      external_rate_id: rate.external_rate_id ?? '',
    })
  }

  function editRateCard(card: CostRateCard) {
    setRateCardForm({
      id: card.id,
      subject_type: card.subject_type,
      rate_type: card.rate_type,
      amount: String(card.amount),
      currency: card.currency,
      scope_type: card.scope_type ?? '',
    })
  }

  function editBudget(budget: CostBudget) {
    setBudgetForm({
      id: budget.id,
      scope_type: budget.scope_type,
      amount: String(budget.amount),
      currency: budget.currency,
      status: budget.status,
    })
  }

  function editLedger(entry: CostLedgerEntry) {
    setLedgerForm({
      id: entry.id,
      cost_category: entry.cost_category,
      source_type: entry.source_type,
      amount: String(entry.amount),
      currency: entry.currency,
      description: entry.description,
    })
  }

  async function voidCostingRecord(action: () => Promise<unknown>) {
    await run(() => action().then(() => undefined), 'costing.recordVoided')
  }

  return (
    <div className="space-y-5">
      <div className="flex flex-wrap gap-2 rounded-lg border border-slate-200 bg-white p-2 shadow-sm">
        {tabs.map((tab) => {
          const Icon = tab.icon
          return (
            <button
              key={tab.id}
              type="button"
              onClick={() => setActiveTab(tab.id)}
              className={`inline-flex h-10 items-center gap-2 rounded-md px-3 text-sm font-semibold transition ${
                activeTab === tab.id ? 'bg-slate-950 text-white' : 'text-slate-600 hover:bg-slate-100'
              }`}
            >
              <Icon className="h-4 w-4" />
              {t(tab.label)}
            </button>
          )
        })}
        <button
          type="button"
          onClick={() => void loadCosting()}
          disabled={loading}
          className="ml-auto inline-flex h-10 items-center gap-2 rounded-md border border-slate-300 px-3 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:opacity-50"
        >
          <RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
          {t('common.refresh')}
        </button>
      </div>

      {(error || notice) && (
        <div
          className={`rounded-lg border px-3 py-2 text-sm ${
            error ? 'border-red-200 bg-red-50 text-red-700' : 'border-emerald-200 bg-emerald-50 text-emerald-700'
          }`}
        >
          {error || notice}
        </div>
      )}

      {activeTab === 'summary' && (
        <div className="space-y-5">
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
            <MetricCard icon={Layers3} label="costing.totalActual" value={money(summary?.total_amount, summary?.currency)} />
            <MetricCard icon={WalletCards} label="costing.totalBudget" value={money(summary?.budget_amount, summary?.currency)} />
            <MetricCard icon={Calculator} label="costing.variance" value={money(summary?.budget_variance, summary?.currency)} />
            <MetricCard icon={ListChecks} label="costing.entryCount" value={String(summary?.entry_count ?? 0)} />
          </div>
          <div className="grid gap-5 xl:grid-cols-3">
            <Panel title="costing.byCategory">
              <KeyValueTable items={summary?.by_category ?? {}} currency={summary?.currency ?? 'CNY'} />
            </Panel>
            <Panel title="costing.bySource">
              <KeyValueTable items={summary?.by_source ?? {}} currency={summary?.currency ?? 'CNY'} />
            </Panel>
            <Panel title="costing.byCurrency">
              <KeyValueTable items={summary?.by_currency ?? {}} />
            </Panel>
          </div>
          <Panel title="costing.recentEntries">
            <LedgerTable entries={summary?.recent_entries ?? []} />
          </Panel>
        </div>
      )}

      {activeTab === 'currencies' && (
        <div className="space-y-5">
          <Panel title="costing.currencies">
            <Table
              headers={['costing.code', 'common.name', 'costing.type', 'costing.symbol', 'developer.status']}
              rows={currencies.map((currency) => [
                currency.code,
                currency.name,
                t(`costing.${currency.currency_type}`),
                currency.symbol,
                currency.is_active ? t('active') : t('disabled'),
              ])}
              actions={currencies.map((currency) => (
                <RowActions
                  key={currency.code}
                  onEdit={() => editCurrency(currency)}
                  onVoid={() => void voidCostingRecord(() => voidCurrency(token, currency.code))}
                />
              ))}
            />
          </Panel>
          <Panel title="costing.currencySettings">
            <form className="space-y-3" onSubmit={submitCurrency}>
              <TextInput label="costing.code" value={currencyForm.code} onChange={(value) => setCurrencyForm({ ...currencyForm, code: value })} />
              <TextInput label="common.name" value={currencyForm.name} onChange={(value) => setCurrencyForm({ ...currencyForm, name: value })} />
              <SelectInput
                label="costing.type"
                value={currencyForm.currency_type}
                onChange={(value) => setCurrencyForm({ ...currencyForm, currency_type: value })}
                options={currencyTypes}
                prefix="costing"
              />
              <TextInput label="costing.symbol" value={currencyForm.symbol} onChange={(value) => setCurrencyForm({ ...currencyForm, symbol: value })} />
              <TextInput
                label="costing.precision"
                type="number"
                value={currencyForm.precision_digits}
                onChange={(value) => setCurrencyForm({ ...currencyForm, precision_digits: value })}
              />
              <TextInput label="costing.chainId" value={currencyForm.chain_id} onChange={(value) => setCurrencyForm({ ...currencyForm, chain_id: value })} />
              <TextInput
                label="costing.contractAddress"
                value={currencyForm.contract_address}
                onChange={(value) => setCurrencyForm({ ...currencyForm, contract_address: value })}
              />
              <TextInput
                label="costing.externalSource"
                value={currencyForm.external_source}
                onChange={(value) => setCurrencyForm({ ...currencyForm, external_source: value })}
              />
              <SubmitButton loading={loading} label="costing.saveCurrency" />
            </form>
          </Panel>
        </div>
      )}

      {activeTab === 'rates' && (
        <div className="space-y-5">
          <div className="space-y-5">
            <Panel title="costing.exchangeRates">
              <Table
                headers={['costing.from', 'costing.to', 'costing.rate', 'costing.source', 'costing.effectiveFrom']}
                rows={exchangeRates.map((rate) => [
                  rate.from_currency,
                  rate.to_currency,
                  String(rate.rate),
                  t(`costing.${rate.source}`),
                  dateText(rate.effective_from),
                ])}
                actions={exchangeRates.map((rate) => (
                  <RowActions
                    key={rate.id}
                    onEdit={() => editRate(rate)}
                    onVoid={() => void voidCostingRecord(() => voidExchangeRate(token, rate.id))}
                  />
                ))}
              />
            </Panel>
            <Panel title="costing.conversion">
              <form className="grid gap-3 md:grid-cols-4" onSubmit={submitConversion}>
                <TextInput label="finance.amount" type="number" value={convertForm.amount} onChange={(value) => setConvertForm({ ...convertForm, amount: value })} />
                <SelectInput
                  label="costing.from"
                  value={convertForm.from_currency}
                  onChange={(value) => setConvertForm({ ...convertForm, from_currency: value })}
                  options={currencyOptions}
                />
                <SelectInput
                  label="costing.to"
                  value={convertForm.to_currency}
                  onChange={(value) => setConvertForm({ ...convertForm, to_currency: value })}
                  options={currencyOptions}
                />
                <SubmitButton loading={loading} label="costing.convert" />
              </form>
              {conversion && (
                <p className="mt-3 text-sm font-semibold text-slate-700">
                  {money(conversion.converted_amount, conversion.to_currency)}
                </p>
              )}
            </Panel>
          </div>
          <Panel title="costing.rateSettings">
            <form className="space-y-3" onSubmit={submitRate}>
              <SelectInput label="costing.from" value={rateForm.from_currency} onChange={(value) => setRateForm({ ...rateForm, from_currency: value })} options={currencyOptions} />
              <SelectInput label="costing.to" value={rateForm.to_currency} onChange={(value) => setRateForm({ ...rateForm, to_currency: value })} options={currencyOptions} />
              <TextInput label="costing.rate" type="number" value={rateForm.rate} onChange={(value) => setRateForm({ ...rateForm, rate: value })} />
              <SelectInput label="costing.source" value={rateForm.source} onChange={(value) => setRateForm({ ...rateForm, source: value })} options={['manual', 'external']} prefix="costing" />
              <TextInput label="costing.provider" value={rateForm.provider} onChange={(value) => setRateForm({ ...rateForm, provider: value })} />
              <TextInput label="costing.externalRateId" value={rateForm.external_rate_id} onChange={(value) => setRateForm({ ...rateForm, external_rate_id: value })} />
              <SubmitButton loading={loading} label={rateForm.id ? 'costing.updateRate' : 'costing.saveRate'} />
            </form>
          </Panel>
        </div>
      )}

      {activeTab === 'rateCards' && (
        <div className="space-y-5">
          <Panel title="costing.rateCards">
            <Table
              headers={['costing.subject', 'costing.rateType', 'finance.amount', 'costing.baseAmount', 'developer.status']}
              rows={rateCards.map((card) => [
                t(`costing.${card.subject_type}`),
                t(`costing.${card.rate_type}`),
                money(card.amount, card.currency),
                money(card.base_amount, card.base_currency),
                t(card.status),
              ])}
              actions={rateCards.map((card) => (
                <RowActions
                  key={card.id}
                  onEdit={() => editRateCard(card)}
                  onVoid={() => void voidCostingRecord(() => voidCostRateCard(token, card.id))}
                />
              ))}
            />
          </Panel>
          <Panel title="costing.rateCardSettings">
            <form className="space-y-3" onSubmit={submitRateCard}>
              <SelectInput label="costing.subject" value={rateCardForm.subject_type} onChange={(value) => setRateCardForm({ ...rateCardForm, subject_type: value })} options={subjectTypes} prefix="costing" />
              <SelectInput label="costing.rateType" value={rateCardForm.rate_type} onChange={(value) => setRateCardForm({ ...rateCardForm, rate_type: value })} options={rateTypes} prefix="costing" />
              <TextInput label="finance.amount" type="number" value={rateCardForm.amount} onChange={(value) => setRateCardForm({ ...rateCardForm, amount: value })} />
              <SelectInput label="finance.currency" value={rateCardForm.currency} onChange={(value) => setRateCardForm({ ...rateCardForm, currency: value })} options={currencyOptions} />
              <SelectInput label="costing.scope" value={rateCardForm.scope_type} onChange={(value) => setRateCardForm({ ...rateCardForm, scope_type: value })} options={['', ...scopeTypes]} prefix="costing" />
              <SubmitButton loading={loading} label={rateCardForm.id ? 'costing.updateRateCard' : 'costing.saveRateCard'} />
            </form>
          </Panel>
        </div>
      )}

      {activeTab === 'budgets' && (
        <div className="space-y-5">
          <Panel title="costing.budgets">
            <Table
              headers={['costing.scope', 'finance.amount', 'costing.baseAmount', 'developer.status']}
              rows={budgets.map((budget) => [
                t(`costing.${budget.scope_type}`),
                money(budget.amount, budget.currency),
                money(budget.base_amount, budget.base_currency),
                t(budget.status),
              ])}
              actions={budgets.map((budget) => (
                <RowActions
                  key={budget.id}
                  onEdit={() => editBudget(budget)}
                  onVoid={() => void voidCostingRecord(() => voidCostBudget(token, budget.id))}
                />
              ))}
            />
          </Panel>
          <Panel title="costing.budgetSettings">
            <form className="space-y-3" onSubmit={submitBudget}>
              <SelectInput label="costing.scope" value={budgetForm.scope_type} onChange={(value) => setBudgetForm({ ...budgetForm, scope_type: value })} options={scopeTypes} prefix="costing" />
              <TextInput label="finance.amount" type="number" value={budgetForm.amount} onChange={(value) => setBudgetForm({ ...budgetForm, amount: value })} />
              <SelectInput label="finance.currency" value={budgetForm.currency} onChange={(value) => setBudgetForm({ ...budgetForm, currency: value })} options={currencyOptions} />
              <SubmitButton loading={loading} label={budgetForm.id ? 'costing.updateBudget' : 'costing.saveBudget'} />
            </form>
          </Panel>
        </div>
      )}

      {activeTab === 'ledger' && (
        <div className="space-y-5">
          <Panel title="costing.ledger">
            <LedgerTable
              entries={ledgerEntries}
              actions={ledgerEntries.map((entry) => (
                <RowActions
                  key={entry.id}
                  onEdit={() => editLedger(entry)}
                  onVoid={() => void voidCostingRecord(() => voidCostLedgerEntry(token, entry.id))}
                />
              ))}
            />
          </Panel>
          <Panel title="costing.ledgerSettings">
            <form className="space-y-3" onSubmit={submitLedger}>
              <SelectInput label="costing.category" value={ledgerForm.cost_category} onChange={(value) => setLedgerForm({ ...ledgerForm, cost_category: value })} options={costCategories} prefix="costing" />
              <TextInput label="costing.sourceType" value={ledgerForm.source_type} onChange={(value) => setLedgerForm({ ...ledgerForm, source_type: value })} />
              <TextInput label="finance.amount" type="number" value={ledgerForm.amount} onChange={(value) => setLedgerForm({ ...ledgerForm, amount: value })} />
              <SelectInput label="finance.currency" value={ledgerForm.currency} onChange={(value) => setLedgerForm({ ...ledgerForm, currency: value })} options={currencyOptions} />
              <TextInput label="costing.description" value={ledgerForm.description} onChange={(value) => setLedgerForm({ ...ledgerForm, description: value })} />
              <SubmitButton loading={loading} label={ledgerForm.id ? 'costing.updateLedger' : 'costing.saveLedger'} />
            </form>
          </Panel>
        </div>
      )}
    </div>
  )
}

function MetricCard({ icon: Icon, label, value }: { icon: typeof Gauge; label: string; value: string }) {
  const { t } = useI18n()
  return (
    <section className="rounded-lg border border-slate-200 bg-white p-4 shadow-sm">
      <div className="flex items-center justify-between gap-3">
        <span className="text-sm font-semibold text-slate-500">{t(label)}</span>
        <Icon className="h-5 w-5 text-slate-400" />
      </div>
      <p className="mt-3 truncate text-2xl font-semibold text-slate-950">{value}</p>
    </section>
  )
}

function Panel({ title, children }: { title: string; children: ReactNode }) {
  const { t } = useI18n()
  return (
    <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
      <h2 className="text-base font-semibold text-slate-950">{t(title)}</h2>
      <div className="mt-4">{children}</div>
    </section>
  )
}

function TextInput({
  label,
  value,
  onChange,
  type = 'text',
}: {
  label: string
  value: string
  onChange: (value: string) => void
  type?: string
}) {
  const { t } = useI18n()
  return (
    <label className="block">
      <span className="text-xs font-semibold text-slate-500">{t(label)}</span>
      <input
        type={type}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="mt-1 h-10 w-full rounded-lg border border-slate-300 px-3 text-sm outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
      />
    </label>
  )
}

function SelectInput({
  label,
  value,
  onChange,
  options,
  prefix,
}: {
  label: string
  value: string
  onChange: (value: string) => void
  options: string[]
  prefix?: string
}) {
  const { t } = useI18n()
  return (
    <label className="block">
      <span className="text-xs font-semibold text-slate-500">{t(label)}</span>
      <select
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="mt-1 h-10 w-full rounded-lg border border-slate-300 bg-white px-3 text-sm outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
      >
        {options.map((option) => (
          <option key={option} value={option}>
            {option === '' ? t('common.none') : t(prefix ? `${prefix}.${option}` : option)}
          </option>
        ))}
      </select>
    </label>
  )
}

function SubmitButton({ loading, label }: { loading: boolean; label: string }) {
  const { t } = useI18n()
  return (
    <button
      type="submit"
      disabled={loading}
      className="inline-flex h-10 w-full items-center justify-center rounded-lg bg-slate-950 px-3 text-sm font-semibold text-white hover:bg-slate-800 disabled:opacity-50"
    >
      {t(label)}
    </button>
  )
}

function LedgerTable({ entries, actions = [] }: { entries: CostLedgerEntry[]; actions?: ReactNode[] }) {
  const { t } = useI18n()
  return (
    <Table
      headers={['costing.category', 'costing.sourceType', 'finance.amount', 'costing.baseAmount', 'developer.status']}
      rows={entries.map((entry) => [
        t(`costing.${entry.cost_category}`),
        entry.source_type,
        money(entry.amount, entry.currency),
        money(entry.base_amount, entry.base_currency),
        t(entry.status),
      ])}
      actions={actions}
    />
  )
}

function KeyValueTable({ items, currency }: { items: Record<string, number>; currency?: string }) {
  const { t } = useI18n()
  const rows = Object.entries(items).map(([key, value]) => [
    t(`costing.${key}`),
    currency ? money(value, currency) : `${key} ${Number(value).toFixed(4)}`,
  ])
  return <Table headers={['common.name', 'finance.amount']} rows={rows} />
}

function RowActions({ onEdit, onVoid }: { onEdit: () => void; onVoid: () => void }) {
  const { t } = useI18n()
  return (
    <div className="flex gap-2">
      <button type="button" onClick={onEdit} className="inline-flex h-8 items-center gap-1 rounded-md border border-slate-300 px-2 text-xs font-semibold text-slate-700 hover:bg-slate-100">
        <Pencil className="h-3.5 w-3.5" />
        {t('common.edit')}
      </button>
      <button type="button" onClick={onVoid} className="inline-flex h-8 items-center gap-1 rounded-md border border-red-200 px-2 text-xs font-semibold text-red-700 hover:bg-red-50">
        <Ban className="h-3.5 w-3.5" />
        {t('common.void')}
      </button>
    </div>
  )
}

function Table({ headers, rows, actions = [] }: { headers: string[]; rows: string[][]; actions?: ReactNode[] }) {
  const { t } = useI18n()
  return (
    <div className="overflow-x-auto rounded-lg border border-slate-200">
      <table className="min-w-full divide-y divide-slate-200 text-sm">
        <thead className="bg-slate-50">
          <tr>
            {headers.map((header) => (
              <th key={header} className="px-3 py-2 text-left text-xs font-semibold uppercase tracking-normal text-slate-500">
                {t(header)}
              </th>
            ))}
            {actions.length > 0 && <th className="px-3 py-2 text-left text-xs font-semibold uppercase tracking-normal text-slate-500">{t('common.action')}</th>}
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-100 bg-white">
          {rows.map((row, index) => (
            <tr key={`${row[0]}-${index}`}>
              {row.map((cell, cellIndex) => (
                <td key={`${cell}-${cellIndex}`} className="max-w-[260px] truncate px-3 py-2 text-slate-700">
                  {cell}
                </td>
              ))}
              {actions.length > 0 && <td className="px-3 py-2">{actions[index]}</td>}
            </tr>
          ))}
          {rows.length === 0 && (
            <tr>
              <td colSpan={headers.length + (actions.length > 0 ? 1 : 0)} className="px-3 py-6 text-center text-sm text-slate-400">
                {t('common.empty')}
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  )
}
