'use client'

import { useEffect, useMemo, useState } from 'react'
import { Database, RefreshCw, Save } from 'lucide-react'
import { apiRequest } from '@/lib/api'
import { useI18n } from '@/lib/i18n'

interface ERPField {
  name: string
  data_type: string
  size?: string
  description?: string
  primary_key: boolean
}

interface ERPChildTable {
  code: string
  name: string
  parent_key: string
  line_key: string
  fields: ERPField[]
}

interface ERPTable {
  code: string
  name: string
  module: string
  primary_key: string
  fields: ERPField[]
  children?: ERPChildTable[]
}

interface ERPCatalog {
  tables: ERPTable[]
}

interface ERPRecord {
  table_code: string
  key: string
  data: Record<string, unknown>
  created_at?: string
  updated_at?: string
}

interface ERPListResponse {
  records: ERPRecord[]
}

interface ERPCodeWorkspaceProps {
  token: string
  module?: string
}

const moduleDefaults: Record<string, string> = {
  partner: 'MCRD',
  product: 'MITM',
  warehouse: 'MWHS',
  inventory: 'MITM',
  purchase: 'MPOR',
  procurement: 'MPOR',
  sale: 'MRDR',
  sales: 'MRDR',
  finance: 'MINV',
  project: 'MPRJ',
  platform: 'MORG',
}

function defaultPayload(tableCode: string): Record<string, unknown> {
  if (tableCode === 'MCRD') return { key: 'C0001', data: { CardCode: 'C0001', CardName: 'Acme Customer', CardType: 'C' } }
  if (tableCode === 'MITM') return { key: 'I0001', data: { ItemCode: 'I0001', ItemName: 'Standard Item', InvntItem: 'Y' } }
  if (tableCode === 'MWHS') return { key: 'W001', data: { WhsCode: 'W001', WhsName: 'Main Warehouse' } }
  if (tableCode === 'MPOR') return { key: '1001', data: { DocEntry: '1001', DocNum: '1001', CardCode: 'S0001', DocStatus: 'O' } }
  if (tableCode === 'MRDR') return { key: '2001', data: { DocEntry: '2001', DocNum: '2001', CardCode: 'C0001', DocStatus: 'O' } }
  if (tableCode === 'MINV') return { key: '3001', data: { DocEntry: '3001', DocNum: '3001', CardCode: 'C0001', DocStatus: 'O' } }
  return { key: '', data: {} }
}

export function ERPCodeWorkspace({ token, module = 'sale' }: ERPCodeWorkspaceProps) {
  const { t } = useI18n()
  const [catalog, setCatalog] = useState<ERPTable[]>([])
  const [selectedCode, setSelectedCode] = useState(moduleDefaults[module] ?? 'MCRD')
  const [records, setRecords] = useState<ERPRecord[]>([])
  const [body, setBody] = useState(JSON.stringify(defaultPayload(selectedCode), null, 2))
  const [message, setMessage] = useState('')
  const [loading, setLoading] = useState(false)

  const visibleTables = useMemo(() => {
    const normalized = module.toLowerCase()
    const filtered = catalog.filter((table) => table.module === normalized || moduleDefaults[normalized] === table.code)
    return filtered.length > 0 ? filtered : catalog
  }, [catalog, module])
  const selectedTable = catalog.find((table) => table.code === selectedCode) ?? visibleTables[0]

  useEffect(() => {
    let cancelled = false
    apiRequest<ERPCatalog>('/erp/catalog', { token })
      .then((data) => {
        if (cancelled) return
        setCatalog(data.tables)
        const nextCode = moduleDefaults[module.toLowerCase()] ?? data.tables[0]?.code ?? 'MCRD'
        setSelectedCode(nextCode)
        setBody(JSON.stringify(defaultPayload(nextCode), null, 2))
      })
      .catch((error: Error) => setMessage(error.message))
    return () => {
      cancelled = true
    }
  }, [module, token])

  useEffect(() => {
    if (!selectedCode) return
    refreshRecords(selectedCode)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedCode])

  async function refreshRecords(code = selectedCode) {
    if (!code) return
    setLoading(true)
    try {
      const data = await apiRequest<ERPListResponse>(`/erp/${encodeURIComponent(code)}?limit=50`, { token })
      setRecords(data.records)
      setMessage('')
    } catch (error) {
      setMessage(error instanceof Error ? error.message : t('erp.loadFailed'))
    } finally {
      setLoading(false)
    }
  }

  async function createRecord() {
    try {
      const payload = JSON.parse(body) as Record<string, unknown>
      await apiRequest<ERPRecord>(`/erp/${encodeURIComponent(selectedCode)}`, {
        method: 'POST',
        token,
        body: payload,
      })
      setMessage(t('erp.recordSaved'))
      await refreshRecords()
    } catch (error) {
      setMessage(error instanceof Error ? error.message : t('erp.saveFailed'))
    }
  }

  return (
    <div className="space-y-5">
      <section className="rounded-lg border border-slate-800 bg-[#17191f] p-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 className="text-base font-semibold text-slate-100">{t('erp.workspace')}</h2>
            <p className="mt-1 text-sm text-slate-400">{t('erp.workspaceHint')}</p>
          </div>
          <button
            type="button"
            onClick={() => refreshRecords()}
            className="inline-flex h-9 items-center gap-2 rounded-lg border border-slate-700 px-3 text-sm text-slate-200 hover:bg-slate-800"
          >
            <RefreshCw size={16} />
            {t('common.refresh')}
          </button>
        </div>
        <div className="mt-4 grid gap-3 md:grid-cols-[220px_1fr]">
          <select
            value={selectedCode}
            onChange={(event) => {
              setSelectedCode(event.target.value)
              setBody(JSON.stringify(defaultPayload(event.target.value), null, 2))
            }}
            className="h-10 rounded-lg border border-slate-700 bg-slate-950 px-3 text-sm text-slate-100"
          >
            {visibleTables.map((table) => (
              <option key={table.code} value={table.code}>
                {table.code} - {table.name}
              </option>
            ))}
          </select>
          {selectedTable && (
            <div className="rounded-lg border border-slate-800 bg-slate-950 p-3 text-sm text-slate-300">
              <div className="flex items-center gap-2 font-semibold text-slate-100">
                <Database size={16} />
                {selectedTable.code} / {selectedTable.primary_key}
              </div>
              <div className="mt-2 flex flex-wrap gap-2">
                {selectedTable.fields.slice(0, 12).map((field) => (
                  <span key={field.name} className="rounded border border-slate-800 px-2 py-1 text-xs text-slate-400">
                    {field.name}
                  </span>
                ))}
              </div>
            </div>
          )}
        </div>
      </section>

      <section className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_360px]">
        <div className="rounded-lg border border-slate-800 bg-[#17191f] p-4">
          <h3 className="text-sm font-semibold text-slate-100">{t('erp.records')}</h3>
          <div className="mt-3 overflow-hidden rounded-lg border border-slate-800">
            <table className="w-full min-w-[520px] text-left text-sm">
              <thead className="bg-slate-950 text-xs uppercase text-slate-500">
                <tr>
                  <th className="px-3 py-2">{t('erp.recordKey')}</th>
                  <th className="px-3 py-2">{t('erp.data')}</th>
                  <th className="px-3 py-2">{t('businessStatus.updated')}</th>
                </tr>
              </thead>
              <tbody>
                {records.map((record) => (
                  <tr key={record.key} className="border-t border-slate-800 text-slate-300">
                    <td className="px-3 py-2 font-mono text-xs">{record.key}</td>
                    <td className="max-w-[420px] truncate px-3 py-2 font-mono text-xs">{JSON.stringify(record.data)}</td>
                    <td className="px-3 py-2 text-xs text-slate-500">{record.updated_at ?? ''}</td>
                  </tr>
                ))}
                {records.length === 0 && (
                  <tr>
                    <td className="px-3 py-8 text-center text-sm text-slate-500" colSpan={3}>
                      {loading ? t('common.loading') : t('table.empty')}
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </div>

        <div className="rounded-lg border border-slate-800 bg-[#17191f] p-4">
          <h3 className="text-sm font-semibold text-slate-100">{t('erp.createRecord')}</h3>
          <textarea
            value={body}
            onChange={(event) => setBody(event.target.value)}
            className="mt-3 h-72 w-full resize-none rounded-lg border border-slate-700 bg-slate-950 p-3 font-mono text-xs text-slate-100 outline-none focus:border-slate-500"
          />
          <button
            type="button"
            onClick={createRecord}
            className="mt-3 inline-flex h-9 w-full items-center justify-center gap-2 rounded-lg bg-slate-100 px-3 text-sm font-semibold text-slate-950 hover:bg-white"
          >
            <Save size={16} />
            {t('erp.saveRecord')}
          </button>
          {message && <p className="mt-3 text-sm text-slate-400">{message}</p>}
        </div>
      </section>
    </div>
  )
}
