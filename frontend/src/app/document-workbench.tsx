'use client'

import { Braces, ExternalLink, FileText, LockKeyhole, RefreshCw, Rows3, ShieldCheck, Table2 } from 'lucide-react'
import { useMemo, useState } from 'react'

import { useI18n } from '@/lib/i18n'
import { resolveFieldCapability, type DocumentWorkbenchDefinition } from '@/lib/workbench'
import type { ApiOperation } from '@/lib/operations'
import { OperationRunnerDrawer } from './operation-runner'

type WorkbenchRecord = Record<string, unknown> & { key?: string }

interface DocumentWorkbenchProps {
  token: string
  definition: DocumentWorkbenchDefinition
  records: WorkbenchRecord[]
  childRows: WorkbenchRecord[]
  selectedKey?: string
  onSelectRecord?: (key: string) => void
  onRefresh?: () => void
  busy?: boolean
}

export function DocumentWorkbench({
  token,
  definition,
  records,
  childRows,
  selectedKey,
  onSelectRecord,
  onRefresh,
  busy = false,
}: DocumentWorkbenchProps) {
  const { t } = useI18n()
  const [operation, setOperation] = useState<ApiOperation | null>(null)
  const selectedRecord = records.find((record) => recordKey(record, definition.primaryKey) === selectedKey) ?? records[0]
  const visibleHeaderFields = useMemo(
    () => definition.headerFields.filter((field) => resolveFieldCapability(field, definition.fieldPermissions).readable),
    [definition.fieldPermissions, definition.headerFields],
  )
  const activeDetail = definition.detailTables[0]
  const visibleDetailFields = useMemo(
    () => activeDetail?.fields.filter((field) => resolveFieldCapability(field, definition.fieldPermissions).readable) ?? [],
    [activeDetail, definition.fieldPermissions],
  )

  return (
    <section className="overflow-hidden rounded-lg border border-slate-200 bg-white shadow-sm">
      <div className="grid min-h-[640px] lg:grid-cols-[260px_minmax(0,1fr)]">
        <aside className="border-b border-slate-200 bg-slate-50 lg:border-b-0 lg:border-r">
          <div className="flex h-14 items-center gap-2 border-b border-slate-200 px-4">
            <FileText className="h-4 w-4 text-slate-500" />
            <div className="min-w-0">
              <h3 className="truncate text-sm font-semibold text-slate-950">{t(definition.titleKey)}</h3>
              <p className="font-mono text-xs text-slate-500">{definition.tableName}</p>
            </div>
          </div>
          <div className="max-h-[580px] overflow-y-auto">
            {records.length > 0 ? (
              records.map((record) => {
                const key = recordKey(record, definition.primaryKey)
                return (
                  <button
                    key={key}
                    type="button"
                    onClick={() => onSelectRecord?.(key)}
                    className={`block w-full border-b border-slate-100 px-4 py-3 text-left transition ${
                      selectedKey === key ? 'bg-[#fff8f3]' : 'hover:bg-white'
                    }`}
                  >
                    <span className="block truncate text-sm font-semibold text-slate-900">{recordTitle(record, definition.primaryKey)}</span>
                    <span className="mt-1 block truncate font-mono text-xs text-slate-500">{key}</span>
                  </button>
                )
              })
            ) : (
              <p className="px-4 py-8 text-center text-sm text-slate-500">{t('table.empty')}</p>
            )}
          </div>
        </aside>

        <div className="min-w-0">
          <div className="flex min-h-14 flex-wrap items-center justify-between gap-3 border-b border-slate-200 px-4 py-3">
            <div className="flex flex-wrap items-center gap-2">
              <span className="inline-flex h-7 items-center gap-1 rounded-md border border-slate-200 bg-slate-50 px-2 text-xs font-semibold text-slate-600">
                <ShieldCheck className="h-3.5 w-3.5" />
                {t('workbench.permission.fieldLevel')}
              </span>
              <span className="inline-flex h-7 items-center gap-1 rounded-md border border-slate-200 bg-slate-50 px-2 text-xs font-semibold text-slate-600">
                <Braces className="h-3.5 w-3.5" />
                {t('workbench.api.embedded')}
              </span>
            </div>
            <div className="flex items-center gap-2">
              {definition.actions.slice(0, 4).map((action) => (
                <button
                  key={action.id}
                  type="button"
                  onClick={() => action.operation && setOperation(action.operation)}
                  disabled={!action.operation}
                  className="inline-flex h-9 items-center gap-2 rounded-md border border-slate-300 bg-white px-3 text-sm font-semibold text-slate-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <Braces className="h-4 w-4" />
                  {t(action.labelKey)}
                </button>
              ))}
              {onRefresh && (
                <button
                  type="button"
                  onClick={onRefresh}
                  disabled={busy}
                  className="inline-flex h-9 w-9 items-center justify-center rounded-md border border-slate-300 text-slate-600 transition hover:bg-slate-50 disabled:opacity-50"
                  aria-label={t('common.refresh')}
                >
                  <RefreshCw className={`h-4 w-4 ${busy ? 'animate-spin' : ''}`} />
                </button>
              )}
            </div>
          </div>

          <div className="grid min-h-[300px] border-b border-slate-200 lg:grid-cols-[minmax(0,1fr)_220px]">
            <div className="p-4">
              <div className="mb-3 flex items-center gap-2">
                <Table2 className="h-4 w-4 text-slate-500" />
                <h4 className="text-sm font-semibold text-slate-950">{t('workbench.header')}</h4>
              </div>
              <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
                {visibleHeaderFields.map((field) => {
                  const capability = resolveFieldCapability(field, definition.fieldPermissions)
                  return (
                    <label key={`${field.tableName}.${field.name}`} className="block min-w-0">
                      <span className="flex items-center gap-1 text-xs font-semibold text-slate-500">
                        {t(field.labelKey)}
                        {!capability.writable && <LockKeyhole className="h-3 w-3 text-slate-400" />}
                      </span>
                      <input
                        key={`${selectedKey ?? ''}.${field.tableName}.${field.name}`}
                        defaultValue={capability.masked ? '***' : displayValue(selectedRecord?.[field.name])}
                        readOnly={!capability.writable}
                        className={`mt-1 h-9 w-full rounded-md border px-3 text-sm outline-none ${
                          capability.writable
                            ? 'border-slate-300 bg-white text-slate-900 focus:border-[#AD4714] focus:ring-2 focus:ring-[#DF6A24]/20'
                            : 'border-slate-200 bg-slate-50 text-slate-600'
                        }`}
                      />
                    </label>
                  )
                })}
              </div>
            </div>
            <div className="border-t border-slate-200 bg-slate-50 p-4 lg:border-l lg:border-t-0">
              <h4 className="text-sm font-semibold text-slate-950">{t('workbench.links')}</h4>
              <div className="mt-3 space-y-2">
                {definition.links.map((link) => (
                  <a
                    key={link.id}
                    href={link.href}
                    className="flex min-w-0 items-center justify-between gap-2 rounded-md border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50"
                  >
                    <span className="truncate">{t(link.labelKey)}</span>
                    <ExternalLink className="h-3.5 w-3.5 shrink-0 text-slate-400" />
                  </a>
                ))}
              </div>
            </div>
          </div>

          <div className="p-4">
            <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
              <div className="flex items-center gap-2">
                <Rows3 className="h-4 w-4 text-slate-500" />
                <h4 className="text-sm font-semibold text-slate-950">{t(activeDetail?.labelKey ?? 'workbench.detail')}</h4>
              </div>
              <div className="flex gap-2">
                <button type="button" className="h-8 rounded-md border border-slate-300 px-3 text-xs font-semibold text-slate-700">
                  {t('workbench.line.add')}
                </button>
                <button type="button" className="h-8 rounded-md border border-slate-300 px-3 text-xs font-semibold text-slate-700">
                  {t('workbench.line.delete')}
                </button>
                <button type="button" className="h-8 rounded-md border border-slate-300 px-3 text-xs font-semibold text-slate-700">
                  {t('workbench.line.batch')}
                </button>
              </div>
            </div>
            <div className="overflow-auto rounded-lg border border-slate-200">
              <table className="w-full min-w-[760px] table-fixed text-left text-sm">
                <thead className="bg-slate-50 text-xs font-semibold text-slate-500">
                  <tr>
                    {visibleDetailFields.map((field) => (
                      <th key={`${field.tableName}.${field.name}`} className="px-3 py-2" style={{ width: field.width }}>
                        {t(field.labelKey)}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {childRows.length > 0 ? (
                    childRows.map((row, index) => (
                      <tr key={recordKey(row, activeDetail?.lineKey ?? String(index)) || index} className="border-t border-slate-100">
                        {visibleDetailFields.map((field) => {
                          const capability = resolveFieldCapability(field, definition.fieldPermissions)
                          return (
                            <td key={`${field.tableName}.${field.name}`} className="truncate px-3 py-2 text-slate-700">
                              {capability.masked ? '***' : displayValue(row[field.name])}
                            </td>
                          )
                        })}
                      </tr>
                    ))
                  ) : (
                    <tr>
                      <td className="px-3 py-8 text-center text-sm text-slate-500" colSpan={Math.max(visibleDetailFields.length, 1)}>
                        {t('table.empty')}
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </div>

      <OperationRunnerDrawer
        token={token}
        operation={operation}
        initialPathValues={{ tableCode: definition.tableName, key: selectedKey ?? '', id: selectedKey ?? '' }}
        onClose={() => setOperation(null)}
      />
    </section>
  )
}

function recordKey(record: WorkbenchRecord | undefined, primaryKey: string): string {
  if (!record) return ''
  const value = record.key ?? record[primaryKey] ?? record.id
  return value === undefined || value === null ? '' : String(value)
}

function recordTitle(record: WorkbenchRecord, primaryKey: string): string {
  const value = record.Name ?? record.CardName ?? record.ItemName ?? record.DocNum ?? record[primaryKey] ?? record.key
  return displayValue(value)
}

function displayValue(value: unknown): string {
  if (value === undefined || value === null || value === '') return ''
  if (typeof value === 'string') return value
  if (typeof value === 'number' || typeof value === 'boolean') return String(value)
  return JSON.stringify(value)
}
