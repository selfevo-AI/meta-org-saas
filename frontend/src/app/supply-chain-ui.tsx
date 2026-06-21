'use client'

import type { ReactNode } from 'react'
import { RefreshCw, Save } from 'lucide-react'
import { useI18n } from '@/lib/i18n'

export type SupplyChainSelection = {
  targetType: string
  targetID?: string
  label: string
  record?: Record<string, unknown>
}

export interface SupplyChainDocumentDetailProps {
  title: string
  subtitle?: string
  mainFields: Array<{ label: string; value: ReactNode }>
  lineColumns: string[]
  lineRows: ReactNode[][]
  actions?: ReactNode
}

export function money(value: number | undefined, currency = 'CNY'): string {
  return `${currency} ${Number(value ?? 0).toFixed(2)}`
}

export function quantity(value: number | undefined): string {
  return Number(value ?? 0).toFixed(2)
}

export function dateOnly(value?: string): string {
  if (!value) return ''
  return new Date(value).toISOString().slice(0, 10)
}

export function documentKey(record: { master_key?: string; id: string }, fallback = ''): string {
  return record.master_key || fallback || record.id.slice(0, 8)
}

export function Panel({ title, children, action }: { title: string; children: ReactNode; action?: ReactNode }) {
  const { t } = useI18n()
  return (
    <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
      <div className="flex items-center justify-between gap-3">
        <h2 className="text-base font-semibold text-slate-950">{t(title)}</h2>
        {action}
      </div>
      <div className="mt-4">{children}</div>
    </section>
  )
}

export function SupplyChainDocumentDetail({
  title,
  subtitle,
  mainFields,
  lineColumns,
  lineRows,
  actions,
}: SupplyChainDocumentDetailProps) {
  const { t } = useI18n()
  return (
    <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="text-xs font-bold uppercase tracking-normal text-slate-500">{t('supplyChain.documentDetail')}</p>
          <h2 className="mt-1 truncate text-lg font-semibold text-slate-950">{t(title)}</h2>
          {subtitle && <p className="mt-1 truncate text-sm text-slate-500">{subtitle}</p>}
        </div>
        {actions}
      </div>
      <div className="mt-5 grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        {mainFields.map((field) => (
          <div key={field.label} className="rounded-lg border border-slate-200 bg-slate-50 px-3 py-2">
            <p className="text-[11px] font-semibold uppercase tracking-normal text-slate-500">{t(field.label)}</p>
            <div className="mt-1 min-h-5 break-words text-sm font-semibold text-slate-900">{field.value || t('common.none')}</div>
          </div>
        ))}
      </div>
      <div className="mt-6">
        <div className="mb-3 flex items-center justify-between gap-3">
          <h3 className="text-sm font-semibold text-slate-950">{t('supplyChain.lineItems')}</h3>
          <span className="text-xs font-semibold text-slate-500">{lineRows.length}</span>
        </div>
        {lineRows.length > 0 ? (
          <DataTable headers={lineColumns} rows={lineRows} />
        ) : (
          <div className="rounded-lg border border-dashed border-slate-300 px-3 py-6 text-center text-sm text-slate-500">
            {t('supplyChain.noLineItems')}
          </div>
        )}
      </div>
    </section>
  )
}

export function TextInput({
  label,
  value,
  onChange,
  type = 'text',
  required = false,
}: {
  label: string
  value: string
  onChange: (value: string) => void
  type?: string
  required?: boolean
}) {
  const { t } = useI18n()
  return (
    <label className="block">
      <span className="text-xs font-semibold text-slate-500">{t(label)}</span>
      <input
        type={type}
        required={required}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="mt-1 h-10 w-full rounded-lg border border-slate-300 px-3 text-sm outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
      />
    </label>
  )
}

export function SelectInput({
  label,
  value,
  onChange,
  options,
  labels = {},
  required = false,
}: {
  label: string
  value: string
  onChange: (value: string) => void
  options: string[]
  labels?: Record<string, string>
  required?: boolean
}) {
  const { t } = useI18n()
  return (
    <label className="block">
      <span className="text-xs font-semibold text-slate-500">{t(label)}</span>
      <select
        required={required}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="mt-1 h-10 w-full rounded-lg border border-slate-300 bg-white px-3 text-sm outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
      >
        <option value="">{t('common.notSelected')}</option>
        {options.map((option) => (
          <option key={option} value={option}>
            {labels[option] ?? t(option)}
          </option>
        ))}
      </select>
    </label>
  )
}

export function SubmitButton({ loading, label }: { loading: boolean; label: string }) {
  const { t } = useI18n()
  return (
    <button
      type="submit"
      disabled={loading}
      className="inline-flex h-10 w-full items-center justify-center gap-2 rounded-lg bg-slate-950 px-3 text-sm font-semibold text-white hover:bg-slate-800 disabled:opacity-50"
    >
      <Save className="h-4 w-4" />
      {t(label)}
    </button>
  )
}

export function RefreshButton({ loading, onClick }: { loading: boolean; onClick: () => void }) {
  const { t } = useI18n()
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={loading}
      className="inline-flex h-9 items-center gap-2 rounded-md border border-slate-300 px-3 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:opacity-50"
    >
      <RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
      {t('common.refresh')}
    </button>
  )
}

export function ActionButton({
  label,
  onClick,
  disabled = false,
  tone = 'default',
  icon,
}: {
  label: string
  onClick: () => void
  disabled?: boolean
  tone?: 'default' | 'primary' | 'green'
  icon?: ReactNode
}) {
  const { t } = useI18n()
  const classes = {
    default: 'border border-slate-300 text-slate-700 hover:bg-slate-100',
    primary: 'bg-slate-950 text-white hover:bg-slate-800',
    green: 'bg-emerald-600 text-white hover:bg-emerald-700',
  }[tone]
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className={`inline-flex h-8 items-center gap-1.5 rounded-md px-2.5 text-xs font-semibold disabled:opacity-50 ${classes}`}
    >
      {icon}
      {t(label)}
    </button>
  )
}

export function StatusPill({ label }: { label: string }) {
  const { t } = useI18n()
  const tone =
    label === 'posted' || label === 'approved' || label === 'confirmed'
      ? 'border-emerald-200 bg-emerald-50 text-emerald-700'
      : label === 'draft'
        ? 'border-slate-200 bg-slate-50 text-slate-700'
        : 'border-blue-200 bg-blue-50 text-blue-700'
  return <span className={`inline-flex h-7 max-w-[160px] items-center truncate rounded-full border px-2.5 text-xs font-semibold ${tone}`}>{t(label)}</span>
}

export function DataTable({
  headers,
  rows,
  actions = [],
}: {
  headers: string[]
  rows: ReactNode[][]
  actions?: ReactNode[]
}) {
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
            {actions.length > 0 && <th className="px-3 py-2 text-left text-xs font-semibold text-slate-500">{t('common.action')}</th>}
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-100 bg-white">
          {rows.map((row, index) => (
            <tr key={index}>
              {row.map((cell, cellIndex) => (
                <td key={cellIndex} className="max-w-[260px] truncate px-3 py-2 text-slate-700">
                  {cell}
                </td>
              ))}
              {actions.length > 0 && <td className="px-3 py-2">{actions[index]}</td>}
            </tr>
          ))}
          {rows.length === 0 && (
            <tr>
              <td className="px-3 py-4 text-sm text-slate-500" colSpan={headers.length + (actions.length > 0 ? 1 : 0)}>
                {t('common.noData')}
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  )
}
