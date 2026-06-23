'use client'

import { FileText, Play, Plus, RefreshCw, Rows3 } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'

import { createERPChildRecord, createERPRecord, listERPRecords, runERPAction } from '@/lib/api'
import { useI18n } from '@/lib/i18n'

type ERPBusinessModule = 'project' | 'procurement' | 'sales' | 'inventory' | 'finance'

type BusinessSelection = {
  targetID?: string
  label?: string
}

type DocumentConfig = {
  id: string
  labelKey: string
  submoduleKey: string
  tableCode: string
  primaryKey: string
  childCode?: string
  actions?: string[]
}

type ERPBusinessModuleWorkspaceProps = {
  token: string
  module: ERPBusinessModule
  externalSelection?: BusinessSelection | null
}

type ERPBusinessRecord = Record<string, unknown> & { key: string }

const moduleDocuments: Record<ERPBusinessModule, DocumentConfig[]> = {
  project: [
    { id: 'requirement', labelKey: 'erp.document.requirement', submoduleKey: 'erp.submodule.requirements', tableCode: 'MREQ', primaryKey: 'ReqCode', childCode: 'REQ1', actions: ['analyze', 'approve', 'convert-to-project'] },
    { id: 'project', labelKey: 'erp.document.project', submoduleKey: 'erp.submodule.projects', tableCode: 'MPRJ', primaryKey: 'PrjCode', childCode: 'APRJ', actions: ['refresh-cost', 'close-feedback'] },
    { id: 'deliverable', labelKey: 'erp.document.delivery', submoduleKey: 'erp.submodule.deliveries', tableCode: 'MDLN', primaryKey: 'DocEntry', childCode: 'DLN1', actions: ['post'] },
    { id: 'cost', labelKey: 'erp.document.cost', submoduleKey: 'erp.submodule.costs', tableCode: 'MCST', primaryKey: 'CostCode', childCode: 'CST1' },
    { id: 'feedback', labelKey: 'erp.document.feedback', submoduleKey: 'erp.submodule.feedback', tableCode: 'MFDB', primaryKey: 'FeedbackCode', childCode: 'FDB1' },
  ],
  procurement: [
    { id: 'purchase_order', labelKey: 'erp.document.purchaseOrder', submoduleKey: 'erp.submodule.purchaseOrders', tableCode: 'MPOR', primaryKey: 'DocEntry', childCode: 'POR1', actions: ['submit', 'approve'] },
    { id: 'goods_receipt_po', labelKey: 'erp.document.goodsReceiptPO', submoduleKey: 'erp.submodule.goodsReceiptPO', tableCode: 'MPDN', primaryKey: 'DocEntry', childCode: 'PDN1', actions: ['post'] },
    { id: 'ap_invoice', labelKey: 'erp.document.apInvoice', submoduleKey: 'erp.submodule.apInvoices', tableCode: 'MPCH', primaryKey: 'DocEntry', childCode: 'PCH1' },
  ],
  sales: [
    { id: 'sales_order', labelKey: 'erp.document.salesOrder', submoduleKey: 'erp.submodule.salesOrders', tableCode: 'MRDR', primaryKey: 'DocEntry', childCode: 'RDR1', actions: ['confirm', 'approve'] },
    { id: 'delivery', labelKey: 'erp.document.delivery', submoduleKey: 'erp.submodule.deliveries', tableCode: 'MDLN', primaryKey: 'DocEntry', childCode: 'DLN1', actions: ['post'] },
    { id: 'ar_invoice', labelKey: 'erp.document.arInvoice', submoduleKey: 'erp.submodule.arInvoices', tableCode: 'MINV', primaryKey: 'DocEntry', childCode: 'INV1', actions: ['post'] },
    { id: 'incoming_payment', labelKey: 'erp.document.incomingPayment', submoduleKey: 'erp.submodule.incomingPayments', tableCode: 'MRCT', primaryKey: 'DocEntry', childCode: 'RCT1', actions: ['allocate'] },
  ],
  inventory: [
    { id: 'business_partner', labelKey: 'erp.document.businessPartner', submoduleKey: 'erp.submodule.partners', tableCode: 'MCRD', primaryKey: 'CardCode', childCode: 'CRD1' },
    { id: 'item', labelKey: 'erp.document.item', submoduleKey: 'erp.submodule.items', tableCode: 'MITM', primaryKey: 'ItemCode', childCode: 'ITM1' },
    { id: 'warehouse', labelKey: 'erp.document.warehouse', submoduleKey: 'erp.submodule.warehouses', tableCode: 'MWHS', primaryKey: 'WhsCode', childCode: 'AWHS' },
    { id: 'warehouse_balance', labelKey: 'erp.document.warehouseBalance', submoduleKey: 'erp.submodule.warehouseBalances', tableCode: 'MITW', primaryKey: 'ItemCode', childCode: 'ITW1' },
    { id: 'goods_receipt', labelKey: 'erp.document.goodsReceipt', submoduleKey: 'erp.submodule.goodsReceipts', tableCode: 'MIGN', primaryKey: 'DocEntry', childCode: 'IGN1', actions: ['post'] },
    { id: 'goods_issue', labelKey: 'erp.document.goodsIssue', submoduleKey: 'erp.submodule.goodsIssues', tableCode: 'MIGE', primaryKey: 'DocEntry', childCode: 'IGE1', actions: ['post'] },
  ],
  finance: [
    { id: 'journal_entry', labelKey: 'erp.document.journalEntry', submoduleKey: 'erp.submodule.journalEntries', tableCode: 'MJDT', primaryKey: 'TransId', childCode: 'JDT1', actions: ['post'] },
    { id: 'ar_invoice', labelKey: 'erp.document.arInvoice', submoduleKey: 'erp.submodule.arInvoices', tableCode: 'MINV', primaryKey: 'DocEntry', childCode: 'INV1', actions: ['post'] },
    { id: 'ap_invoice', labelKey: 'erp.document.apInvoice', submoduleKey: 'erp.submodule.apInvoices', tableCode: 'MPCH', primaryKey: 'DocEntry', childCode: 'PCH1' },
    { id: 'incoming_payment', labelKey: 'erp.document.incomingPayment', submoduleKey: 'erp.submodule.incomingPayments', tableCode: 'MRCT', primaryKey: 'DocEntry', childCode: 'RCT1', actions: ['allocate'] },
  ],
}

export function ERPBusinessModuleWorkspace({ token, module, externalSelection }: ERPBusinessModuleWorkspaceProps) {
  const { t } = useI18n()
  const documents = moduleDocuments[module]
  const [activeID, setActiveID] = useState(documents[0]?.id ?? '')
  const activeDocument = useMemo(() => documents.find((item) => item.id === activeID) ?? documents[0], [activeID, documents])
  const [records, setRecords] = useState<ERPBusinessRecord[]>([])
  const [selectedKey, setSelectedKey] = useState('')
  const [form, setForm] = useState({ key: '', name: '', cardCode: '', itemCode: '', whsCode: '', quantity: '1', price: '0', targetKey: '', amount: '0' })
  const [lineForm, setLineForm] = useState({ lineNum: '1', itemCode: '', whsCode: '', quantity: '1', price: '0' })
  const [busy, setBusy] = useState(false)
  const [notice, setNotice] = useState('')
  const [error, setError] = useState('')

  const selectedRecord = records.find((record) => record.key === selectedKey)

  async function loadRecords(document = activeDocument) {
    if (!document) return
    setBusy(true)
    setError('')
    try {
      const items = await listERPRecords<ERPBusinessRecord>(token, document.tableCode, 100)
      setRecords(items)
      setSelectedKey((current) => {
        if (current && items.some((item) => item.key === current)) return current
        if (externalSelection?.targetID && items.some((item) => item.key === externalSelection.targetID)) return externalSelection.targetID
        return items[0]?.key || ''
      })
    } catch (err) {
      setError(err instanceof Error ? err.message : t('erp.business.loadFailed'))
    } finally {
      setBusy(false)
    }
  }

  useEffect(() => {
    if (!activeDocument) return
    let cancelled = false
    listERPRecords<ERPBusinessRecord>(token, activeDocument.tableCode, 100)
      .then((items) => {
        if (cancelled) return
        setRecords(items)
        setSelectedKey((current) => {
          if (current && items.some((item) => item.key === current)) return current
          if (externalSelection?.targetID && items.some((item) => item.key === externalSelection.targetID)) return externalSelection.targetID
          return items[0]?.key || ''
        })
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof Error ? err.message : t('erp.business.loadFailed'))
      })
    return () => {
      cancelled = true
    }
  }, [activeDocument, externalSelection?.targetID, t, token])

  async function handleCreateRecord() {
    if (!activeDocument || !form.key.trim()) return
    setBusy(true)
    setError('')
    setNotice('')
    try {
      const data = buildRecordData(activeDocument, form)
      await createERPRecord(token, activeDocument.tableCode, form.key.trim(), data)
      setForm((current) => ({ ...current, key: '', name: '' }))
      setNotice(t('erp.business.recordCreated'))
      await loadRecords(activeDocument)
    } catch (err) {
      setError(err instanceof Error ? err.message : t('common.operationFailed'))
    } finally {
      setBusy(false)
    }
  }

  async function handleCreateLine() {
    if (!activeDocument?.childCode || !selectedKey) return
    setBusy(true)
    setError('')
    setNotice('')
    try {
      await createERPChildRecord(token, activeDocument.tableCode, selectedKey, activeDocument.childCode, lineForm.lineNum.trim(), {
        [activeDocument.primaryKey]: selectedKey,
        LineNum: lineForm.lineNum.trim(),
        LineStatus: 'O',
        Payload: {
          ItemCode: lineForm.itemCode.trim(),
          WhsCode: lineForm.whsCode.trim(),
          Quantity: Number(lineForm.quantity || 0),
          Price: Number(lineForm.price || 0),
        },
      })
      setLineForm((current) => ({ ...current, lineNum: String(Number(current.lineNum || 1) + 1) }))
      setNotice(t('erp.business.lineCreated'))
    } catch (err) {
      setError(err instanceof Error ? err.message : t('common.operationFailed'))
    } finally {
      setBusy(false)
    }
  }

  async function handleAction(action: string) {
    if (!activeDocument || !selectedKey) return
    setBusy(true)
    setError('')
    setNotice('')
    try {
      await runERPAction(token, activeDocument.tableCode, selectedKey, action, buildActionData(action, form))
      setNotice(t('erp.business.actionDone'))
      await loadRecords(activeDocument)
    } catch (err) {
      setError(err instanceof Error ? err.message : t('common.operationFailed'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="text-lg font-semibold text-slate-950">{t(`erp.module.${module}`)}</h2>
          <p className="mt-1 text-sm text-slate-500">{t('erp.business.moduleHint')}</p>
        </div>
        <button
          type="button"
          onClick={() => void loadRecords(activeDocument)}
          disabled={busy}
          className="inline-flex h-9 items-center gap-2 rounded-md border border-slate-300 bg-white px-3 text-sm font-semibold text-slate-700 transition hover:bg-slate-50 disabled:opacity-50"
        >
          <RefreshCw className={`h-4 w-4 ${busy ? 'animate-spin' : ''}`} />
          {t('common.refresh')}
        </button>
      </div>

      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
        {documents.map((document) => (
          <button
            key={document.id}
            type="button"
            onClick={() => setActiveID(document.id)}
            className={`rounded-lg border p-4 text-left transition ${
              activeDocument?.id === document.id ? 'border-[#AD4714] bg-[#fff8f3]' : 'border-slate-200 bg-white hover:bg-slate-50'
            }`}
          >
            <div className="flex items-center gap-2">
              <FileText className="h-4 w-4 text-slate-500" />
              <span className="text-sm font-semibold text-slate-950">{t(document.labelKey)}</span>
            </div>
            <p className="mt-2 text-xs text-slate-500">{t(document.submoduleKey)}</p>
            <p className="mt-3 font-mono text-xs text-slate-500">
              {document.tableCode}
              {document.childCode ? ` / ${document.childCode}` : ''}
            </p>
          </button>
        ))}
      </div>

      {activeDocument && (
        <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_360px]">
          <section className="rounded-lg border border-slate-200 bg-white">
            <div className="flex items-center justify-between border-b border-slate-100 px-4 py-3">
              <div>
                <h3 className="text-sm font-semibold text-slate-950">{t(activeDocument.labelKey)}</h3>
                <p className="mt-1 font-mono text-xs text-slate-500">{activeDocument.tableCode}</p>
              </div>
              <Rows3 className="h-4 w-4 text-slate-400" />
            </div>
            <div className="divide-y divide-slate-100">
              {records.length > 0 ? (
                records.map((record) => (
                  <button
                    key={record.key}
                    type="button"
                    onClick={() => setSelectedKey(record.key)}
                    className={`block w-full px-4 py-3 text-left transition ${selectedKey === record.key ? 'bg-[#fff8f3]' : 'hover:bg-slate-50'}`}
                  >
                    <div className="flex items-center justify-between gap-3">
                      <span className="truncate text-sm font-semibold text-slate-900">{recordTitle(record)}</span>
                      <span className="shrink-0 rounded-md bg-slate-100 px-2 py-1 font-mono text-xs text-slate-600">{record.key}</span>
                    </div>
                    <p className="mt-1 truncate text-xs text-slate-500">{recordStatus(record)}</p>
                  </button>
                ))
              ) : (
                <p className="px-4 py-8 text-center text-sm text-slate-500">{t('erp.business.noRecords')}</p>
              )}
            </div>
          </section>

          <aside className="space-y-4">
            <section className="rounded-lg border border-slate-200 bg-white p-4">
              <h3 className="text-sm font-semibold text-slate-950">{t('erp.business.createDocument')}</h3>
              <div className="mt-3 space-y-2">
                <ERPInput label={t('erp.business.key')} value={form.key} onChange={(value) => setForm((current) => ({ ...current, key: value }))} placeholder={activeDocument.primaryKey} />
                <ERPInput label={t('erp.business.name')} value={form.name} onChange={(value) => setForm((current) => ({ ...current, name: value }))} placeholder={t(activeDocument.labelKey)} />
                <ERPInput label={t('erp.business.cardCode')} value={form.cardCode} onChange={(value) => setForm((current) => ({ ...current, cardCode: value }))} placeholder="C-1001" />
                <button
                  type="button"
                  onClick={() => void handleCreateRecord()}
                  disabled={busy || !form.key.trim()}
                  className="inline-flex h-9 w-full items-center justify-center gap-2 rounded-md bg-[#AD4714] px-3 text-sm font-semibold text-white transition hover:bg-[#B84F18] disabled:opacity-50"
                >
                  <Plus className="h-4 w-4" />
                  {t('erp.business.createDocument')}
                </button>
              </div>
            </section>

            {activeDocument.childCode && (
              <section className="rounded-lg border border-slate-200 bg-white p-4">
                <h3 className="text-sm font-semibold text-slate-950">{t('erp.business.createLine')}</h3>
                <div className="mt-3 space-y-2">
                  <ERPInput label={t('erp.business.lineNum')} value={lineForm.lineNum} onChange={(value) => setLineForm((current) => ({ ...current, lineNum: value }))} placeholder="1" />
                  <ERPInput label={t('erp.business.itemCode')} value={lineForm.itemCode} onChange={(value) => setLineForm((current) => ({ ...current, itemCode: value }))} placeholder="I-1001" />
                  <ERPInput label={t('erp.business.whsCode')} value={lineForm.whsCode} onChange={(value) => setLineForm((current) => ({ ...current, whsCode: value }))} placeholder="W-1" />
                  <div className="grid grid-cols-2 gap-2">
                    <ERPInput label={t('erp.business.quantity')} value={lineForm.quantity} onChange={(value) => setLineForm((current) => ({ ...current, quantity: value }))} placeholder="1" />
                    <ERPInput label={t('erp.business.price')} value={lineForm.price} onChange={(value) => setLineForm((current) => ({ ...current, price: value }))} placeholder="0" />
                  </div>
                  <button
                    type="button"
                    onClick={() => void handleCreateLine()}
                    disabled={busy || !selectedKey}
                    className="inline-flex h-9 w-full items-center justify-center gap-2 rounded-md border border-slate-300 bg-white px-3 text-sm font-semibold text-slate-800 transition hover:bg-slate-50 disabled:opacity-50"
                  >
                    <Plus className="h-4 w-4" />
                    {t('erp.business.createLine')}
                  </button>
                </div>
              </section>
            )}

            {(activeDocument.actions ?? []).length > 0 && (
              <section className="rounded-lg border border-slate-200 bg-white p-4">
                <h3 className="text-sm font-semibold text-slate-950">{t('erp.business.actions')}</h3>
                {activeDocument.tableCode === 'MRCT' && (
                  <div className="mt-3 space-y-2">
                    <ERPInput label={t('erp.business.targetKey')} value={form.targetKey} onChange={(value) => setForm((current) => ({ ...current, targetKey: value }))} placeholder="INV-1001" />
                    <ERPInput label={t('erp.business.amount')} value={form.amount} onChange={(value) => setForm((current) => ({ ...current, amount: value }))} placeholder="100" />
                  </div>
                )}
                <div className="mt-3 grid gap-2">
                  {activeDocument.actions?.map((action) => (
                    <button
                      key={action}
                      type="button"
                      onClick={() => void handleAction(action)}
                      disabled={busy || !selectedKey}
                      className="inline-flex h-9 items-center justify-center gap-2 rounded-md border border-slate-300 bg-white px-3 text-sm font-semibold text-slate-800 transition hover:bg-slate-50 disabled:opacity-50"
                    >
                      <Play className="h-4 w-4" />
                      {t(`erp.action.${action}`)}
                    </button>
                  ))}
                </div>
              </section>
            )}

            {selectedRecord && (
              <section className="rounded-lg border border-slate-200 bg-white p-4">
                <h3 className="text-sm font-semibold text-slate-950">{t('erp.business.selected')}</h3>
                <pre className="mt-3 max-h-64 overflow-auto rounded-md bg-slate-950 p-3 text-xs text-slate-100">{JSON.stringify(selectedRecord, null, 2)}</pre>
              </section>
            )}

            {(notice || error) && (
              <p className={`rounded-md px-3 py-2 text-sm ${error ? 'bg-red-50 text-red-700' : 'bg-emerald-50 text-emerald-700'}`}>
                {error || notice}
              </p>
            )}
          </aside>
        </div>
      )}
    </section>
  )
}

function ERPInput({ label, value, placeholder, onChange }: { label: string; value: string; placeholder: string; onChange: (value: string) => void }) {
  return (
    <label className="block">
      <span className="text-xs font-semibold text-slate-500">{label}</span>
      <input
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        className="mt-1 h-9 w-full rounded-md border border-slate-300 px-3 text-sm text-slate-900 outline-none transition focus:border-[#AD4714] focus:ring-2 focus:ring-[#DF6A24]/20"
      />
    </label>
  )
}

function buildRecordData(document: DocumentConfig, form: { key: string; name: string; cardCode: string; quantity: string; price: string }) {
  const key = form.key.trim()
  const payload = {
    Name: form.name.trim(),
  }
  const data: Record<string, unknown> = {
    [document.primaryKey]: key,
    Payload: payload,
  }
  if (document.primaryKey === 'DocEntry') {
    data.DocNum = key
    data.DocStatus = 'O'
    data.CardCode = form.cardCode.trim()
    data.DocTotal = Number(form.quantity || 0) * Number(form.price || 0)
  }
  if (document.primaryKey === 'TransId') {
    data.BtfStatus = 'O'
  }
  if (document.primaryKey === 'PrjCode') {
    data.Active = 'Y'
  }
  return data
}

function buildActionData(action: string, form: { targetKey: string; amount: string }) {
  if (action === 'allocate') {
    return {
      TargetTable: 'MINV',
      TargetKey: form.targetKey.trim(),
      Amount: Number(form.amount || 0),
    }
  }
  return {}
}

function recordTitle(record: ERPBusinessRecord) {
  return String(record.Name || record.CardCode || record.ItemCode || record.WhsCode || record.PrjCode || record.DocEntry || record.TransId || record.key)
}

function recordStatus(record: ERPBusinessRecord) {
  return String(record.Status || record.DocStatus || record.WddStatus || record.BtfStatus || record.Active || '')
}
