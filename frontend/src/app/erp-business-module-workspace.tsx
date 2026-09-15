'use client'

import { ArrowLeft, Download, RefreshCw } from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'

import {
  createERPChildRecord, createERPRecord, deleteERPChildRecord, deleteERPRecord,
  getFinanceGLTrialBalance, listERPActionExecutions, listERPChildRecords, listERPRecords,
  runERPAction, updateERPChildRecord, updateERPRecord, type ERPActionExecution, type FinanceGLTrialBalance,
} from '@/lib/api'
import { useI18n } from '@/lib/i18n'
import {
  executeOntologyAction, getBusinessRecord, getOntologyHistory, getOntologyLinks, getOntologyType,
  ontologyRecord, ontologyTypeByTable, queryOntologyObjects, type OntologyLinks, type OntologyType,
} from '@/lib/ontology'
import { defaultWorkbenchFields, defaultWorkbenchLineFields, type DocumentWorkbenchDefinition } from '@/lib/workbench'
import { DocumentWorkbench, type WorkbenchLookupOptions } from './document-workbench'
import { DocumentImportWorkspace } from './document-import-workspace'
import { BusinessAIWorkbench } from './business-ai-workbench'
import { FeedbackMessage, useWorkspaceInteraction } from './workspace-ui'

type ERPBusinessModule = 'project' | 'procurement' | 'sales' | 'inventory' | 'finance' | 'retail' | 'manufacturing'
type BusinessSelection = { targetID?: string; label?: string }
type DocumentConfig = {
  id: string
  labelKey: string
  submoduleKey: string
  tableCode: string
  primaryKey: string
  childCode?: string
  actions?: string[]
  sortOrder?: number
  kind?: 'document' | 'report'
}
type ERPBusinessModuleWorkspaceProps = {
  token: string
  module: ERPBusinessModule
  externalSelection?: BusinessSelection | null
  activeDocumentID?: string | null
}
type ERPBusinessRecord = Record<string, unknown> & { key: string }

const moduleDocuments: Record<ERPBusinessModule, DocumentConfig[]> = {
  project: [
    { id: 'requirement', labelKey: 'erp.document.requirement', submoduleKey: 'erp.submodule.requirements', tableCode: 'MREQ', primaryKey: 'ReqCode', childCode: 'REQ1', actions: ['analyze', 'approve', 'convert-to-project'] },
    { id: 'project', labelKey: 'erp.document.project', submoduleKey: 'erp.submodule.projects', tableCode: 'MPRJ', primaryKey: 'PrjCode', childCode: 'APRJ', actions: ['refresh-cost', 'close-feedback'] },
    { id: 'deliverable', labelKey: 'erp.document.delivery', submoduleKey: 'erp.submodule.deliveries', tableCode: 'MDLN', primaryKey: 'DocEntry', childCode: 'DLN1', actions: ['approve', 'post'] },
    { id: 'cost', labelKey: 'erp.document.cost', submoduleKey: 'erp.submodule.costs', tableCode: 'MCST', primaryKey: 'CostCode', childCode: 'CST1' },
    { id: 'feedback', labelKey: 'erp.document.feedback', submoduleKey: 'erp.submodule.feedback', tableCode: 'MFDB', primaryKey: 'FeedbackCode', childCode: 'FDB1' },
  ],
  procurement: [
    { id: 'purchase_order', labelKey: 'erp.document.purchaseOrder', submoduleKey: 'erp.submodule.purchaseOrders', tableCode: 'MPOR', primaryKey: 'DocEntry', childCode: 'POR1', actions: ['submit', 'approve', 'receive'] },
    { id: 'goods_receipt_po', labelKey: 'erp.document.goodsReceiptPO', submoduleKey: 'erp.submodule.goodsReceiptPO', tableCode: 'MPDN', primaryKey: 'DocEntry', childCode: 'PDN1', actions: ['approve', 'post'] },
    { id: 'ap_invoice', labelKey: 'erp.document.apInvoice', submoduleKey: 'erp.submodule.apInvoices', tableCode: 'MPCH', primaryKey: 'DocEntry', childCode: 'PCH1', actions: ['post'] },
  ],
  sales: [
    { id: 'sales_order', labelKey: 'erp.document.salesOrder', submoduleKey: 'erp.submodule.salesOrders', tableCode: 'MRDR', primaryKey: 'DocEntry', childCode: 'RDR1', actions: ['confirm', 'approve', 'deliver'] },
    { id: 'delivery', labelKey: 'erp.document.delivery', submoduleKey: 'erp.submodule.deliveries', tableCode: 'MDLN', primaryKey: 'DocEntry', childCode: 'DLN1', actions: ['approve', 'post'] },
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
  retail: [
    { id: 'retail_branch', labelKey: 'erp.document.retailBranch', submoduleKey: 'erp.submodule.retailBranches', tableCode: 'MBRN', primaryKey: 'StoreCode', childCode: 'BRN1', sortOrder: 10 },
    { id: 'retail_terminal', labelKey: 'erp.document.retailTerminal', submoduleKey: 'erp.submodule.retailTerminals', tableCode: 'MTER', primaryKey: 'TerminalCode', childCode: 'TER1', sortOrder: 20 },
    { id: 'retail_member', labelKey: 'erp.document.retailMember', submoduleKey: 'erp.submodule.retailMembers', tableCode: 'MMBR', primaryKey: 'MemberCode', childCode: 'MBR1', sortOrder: 30 },
    { id: 'retail_promotion', labelKey: 'erp.document.retailPromotion', submoduleKey: 'erp.submodule.retailPromotions', tableCode: 'MPRM', primaryKey: 'PromotionCode', childCode: 'PRM1', sortOrder: 40 },
    { id: 'retail_publishing', labelKey: 'erp.document.retailPublishing', submoduleKey: 'erp.submodule.retailPublishing', tableCode: 'MPUB', primaryKey: 'PublicationCode', childCode: 'PUB1', actions: ['publish'], sortOrder: 50 },
    { id: 'pos_sale', labelKey: 'erp.document.posSale', submoduleKey: 'erp.submodule.posSales', tableCode: 'MRPS', primaryKey: 'DocEntry', childCode: 'RPS1', actions: ['close'], sortOrder: 60 },
    { id: 'distribution_request', labelKey: 'erp.document.distributionRequest', submoduleKey: 'erp.submodule.distributionRequests', tableCode: 'MDRQ', primaryKey: 'DocEntry', childCode: 'DRQ1', actions: ['submit', 'approve', 'auto-allocate'], sortOrder: 70 },
    { id: 'distribution_shipment', labelKey: 'erp.document.distributionShipment', submoduleKey: 'erp.submodule.distributionShipments', tableCode: 'MDSP', primaryKey: 'DocEntry', childCode: 'DSP1', actions: ['ship'], sortOrder: 80 },
    { id: 'distribution_receipt', labelKey: 'erp.document.distributionReceipt', submoduleKey: 'erp.submodule.distributionReceipts', tableCode: 'MDRC', primaryKey: 'DocEntry', childCode: 'DRC1', actions: ['receive'], sortOrder: 90 },
    { id: 'distribution_difference', labelKey: 'erp.document.distributionDifference', submoduleKey: 'erp.submodule.distributionDifferences', tableCode: 'MDIF', primaryKey: 'DocEntry', childCode: 'DIF1', actions: ['resolve'], sortOrder: 100 },
    { id: 'stock_policy', labelKey: 'erp.document.stockPolicy', submoduleKey: 'erp.submodule.stockPolicies', tableCode: 'MSTP', primaryKey: 'PolicyCode', childCode: 'STP1', actions: ['replenish'], sortOrder: 110 },
    { id: 'store_count', labelKey: 'erp.document.storeCount', submoduleKey: 'erp.submodule.storeCounts', tableCode: 'MCNT', primaryKey: 'DocEntry', childCode: 'CNT1', actions: ['submit', 'approve', 'post-adjustment'], sortOrder: 120 },
    { id: 'special_purchase_request', labelKey: 'erp.document.specialPurchaseRequest', submoduleKey: 'erp.submodule.specialPurchaseRequests', tableCode: 'MSPR', primaryKey: 'DocEntry', childCode: 'SPR1', actions: ['submit', 'approve', 'convert-to-purchase-order'], sortOrder: 130 },
  ],
  manufacturing: [
    { id: 'bill_of_materials', labelKey: 'erp.document.billOfMaterials', submoduleKey: 'erp.submodule.bom', tableCode: 'MBOM', primaryKey: 'BOMCode', childCode: 'BOM1', actions: ['approve', 'make-work-order'], sortOrder: 10 },
    { id: 'work_order', labelKey: 'erp.document.workOrder', submoduleKey: 'erp.submodule.workOrders', tableCode: 'MWOR', primaryKey: 'WorkOrderCode', childCode: 'WOR1', actions: ['release', 'issue-material', 'complete', 'stop', 'reopen', 'close'], sortOrder: 20 },
    { id: 'material_issue', labelKey: 'erp.document.goodsIssue', submoduleKey: 'erp.submodule.materialIssue', tableCode: 'MIGE', primaryKey: 'DocEntry', childCode: 'IGE1', actions: ['post'], sortOrder: 30 },
    { id: 'finished_goods_receipt', labelKey: 'erp.document.goodsReceipt', submoduleKey: 'erp.submodule.finishedGoodsReceipt', tableCode: 'MIGN', primaryKey: 'DocEntry', childCode: 'IGN1', actions: ['post'], sortOrder: 40 },
    { id: 'production_journal', labelKey: 'erp.document.journalEntry', submoduleKey: 'erp.submodule.productionJournal', tableCode: 'MJDT', primaryKey: 'TransId', childCode: 'JDT1', actions: ['post'], sortOrder: 50 },
  ],
  finance: [
    { id: 'gl_account', labelKey: 'erp.document.glAccount', submoduleKey: 'erp.submodule.chartOfAccounts', tableCode: 'MACT', primaryKey: 'AcctCode', childCode: 'AACT', sortOrder: 10 },
    { id: 'cost_center', labelKey: 'erp.document.costCenter', submoduleKey: 'erp.submodule.costCenters', tableCode: 'MPRC', primaryKey: 'PrcCode', childCode: 'APRC', sortOrder: 20 },
    { id: 'journal_entry', labelKey: 'erp.document.journalEntry', submoduleKey: 'erp.submodule.journalEntries', tableCode: 'MJDT', primaryKey: 'TransId', childCode: 'JDT1', actions: ['post'], sortOrder: 30 },
    { id: 'trial_balance', labelKey: 'erp.document.trialBalance', submoduleKey: 'erp.submodule.trialBalance', tableCode: 'MGLR', primaryKey: 'ReportCode', actions: ['run'], sortOrder: 40, kind: 'report' },
    { id: 'ar_invoice', labelKey: 'erp.document.arInvoice', submoduleKey: 'erp.submodule.arInvoices', tableCode: 'MINV', primaryKey: 'DocEntry', childCode: 'INV1', actions: ['post'], sortOrder: 50 },
    { id: 'ap_invoice', labelKey: 'erp.document.apInvoice', submoduleKey: 'erp.submodule.apInvoices', tableCode: 'MPCH', primaryKey: 'DocEntry', childCode: 'PCH1', actions: ['post'], sortOrder: 60 },
    { id: 'incoming_payment', labelKey: 'erp.document.incomingPayment', submoduleKey: 'erp.submodule.incomingPayments', tableCode: 'MRCT', primaryKey: 'DocEntry', childCode: 'RCT1', actions: ['allocate'], sortOrder: 70 },
    { id: 'outgoing_payment', labelKey: 'erp.document.outgoingPayment', submoduleKey: 'erp.submodule.outgoingPayments', tableCode: 'MVPM', primaryKey: 'DocEntry', childCode: 'VPM1', actions: ['allocate'], sortOrder: 80 },
  ],
}


export function buildERPDocumentWorkbenchDefinition(document: DocumentConfig, module: ERPBusinessModule): DocumentWorkbenchDefinition {
  const editableLines = !['AACT', 'APRC', 'APRJ', 'AWHS', 'ITW1'].includes(document.childCode ?? '')
  return {
    id: module + '.' + document.id,
    moduleKey: 'erp.module.' + module,
    titleKey: document.labelKey,
    tableName: document.tableCode,
    primaryKey: document.primaryKey,
    headerFields: defaultWorkbenchFields(document.tableCode, document.primaryKey),
    detailTables: document.childCode ? [{
      tableName: document.childCode, labelKey: 'workbench.detail', parentKey: document.primaryKey, lineKey: 'LineNum',
      fields: defaultWorkbenchLineFields(document.childCode),
      allowCreate: editableLines && !['MRCT', 'MVPM'].includes(document.tableCode),
      allowDelete: editableLines && !['MRCT', 'MVPM'].includes(document.tableCode),
    }] : [],
    actions: (document.actions ?? []).map((id) => ({
      id, labelKey: 'erp.action.' + id,
      parameters: id === 'allocate' ? [
        { name: 'TargetKey', labelKey: 'ontology.field.invoice', tableName: document.tableCode, required: true },
        { name: 'Amount', labelKey: 'ontology.field.amount', tableName: document.tableCode, dataType: 'number', required: true },
      ] : undefined,
    })),
    links: [],
  }
}

export function ERPBusinessModuleWorkspace({ token, module, externalSelection, activeDocumentID }: ERPBusinessModuleWorkspaceProps) {
  const { t } = useI18n()
  const { requestNavigation } = useWorkspaceInteraction()
  const documents = moduleDocuments[module]
  const [selection, setSelection] = useState<{ document: DocumentConfig; key?: string; origin?: string } | null>(null)
  const base = documents.find((item) => item.id === activeDocumentID) ?? documents[0]
  const selected = selection?.origin === (activeDocumentID ?? undefined) ? selection : null
  const active = selected?.document ?? base
  const initialKey = selected?.key ?? externalSelection?.targetID
  function openLink(table: string, key: string) {
    const document = Object.values(moduleDocuments).flat().find((item) => item.tableCode === table)
    if (document) setSelection({ document, key, origin: activeDocumentID ?? undefined })
  }
  return (
    <section data-testid="erp-business-module-workspace" className="ontology-workspace min-w-0 space-y-4">
      {selected && <div className="flex flex-wrap items-center gap-3">
        <button type="button" onClick={() => requestNavigation(() => setSelection(null))} className="ui-button ui-button-secondary"><ArrowLeft size={16} />{t('ontology.back')}</button>
        <span className="text-sm ui-muted">{t(active.labelKey)}</span>
      </div>}
      {!activeDocumentID && <div role="tablist" aria-label={t('ontology.documents')} className="flex flex-wrap gap-x-4 gap-y-1 border-b border-slate-200">
        {documents.map((item) => <button key={item.id} type="button" role="tab" aria-selected={active.id === item.id} data-testid={'erp-document-' + item.id}
          onClick={() => requestNavigation(() => setSelection({ document: item, origin: activeDocumentID ?? undefined }))}
          className={'min-h-10 border-b-2 pb-2 text-sm ' + (active.id === item.id ? 'border-teal-600 font-medium text-teal-800' : 'border-transparent text-slate-500')}>{t(item.labelKey)}</button>)}
      </div>}
      {active.kind === 'report' ? <TrialBalanceReport key={active.id} token={token} /> :
        <BusinessDocument key={active.tableCode + ':' + (initialKey ?? '')} token={token} module={module} document={active} initialKey={initialKey} onOpenLink={openLink} />}
    </section>
  )
}

function BusinessDocument({ token, module, document, initialKey, onOpenLink }: {
  token: string; module: ERPBusinessModule; document: DocumentConfig; initialKey?: string; onOpenLink: (table: string, key: string) => void
}) {
  const { t } = useI18n()
  const [records, setRecords] = useState<ERPBusinessRecord[]>([])
  const [selectedKey, setSelectedKey] = useState(initialKey ?? '')
  const [detail, setDetail] = useState<{ record: ERPBusinessRecord; rows: ERPBusinessRecord[]; history: ERPActionExecution[]; links: OntologyLinks } | null>(null)
  const [objectType, setObjectType] = useState<OntologyType | null>(null)
  const [search, setSearch] = useState('')
  const [cursor, setCursor] = useState('')
  const [nextCursor, setNextCursor] = useState('')
  const [totalCount, setTotalCount] = useState<number | undefined>()
  const [filterStatus, setFilterStatus] = useState('all')
  const [sortKey, setSortKey] = useState('key')
  const [sortDirection, setSortDirection] = useState<'asc' | 'desc'>('asc')
  const [importView, setImportView] = useState<{ id?: string } | null>(null)
  const [loading, setLoading] = useState(true)
  const [detailLoading, setDetailLoading] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [version, setVersion] = useState(0)
  const [lookupOptions, setLookupOptions] = useState<WorkbenchLookupOptions>({})
  const requestID = useRef<{ signature: string; id: string } | null>(null)
  const typeKey = ontologyTypeByTable[document.tableCode]
  const readOnly = module === 'retail' || module === 'manufacturing' || document.tableCode === 'MITW' || !typeKey
  const selectedRecord = detail?.record.key === selectedKey ? detail.record : undefined
  const definition = useMemo(() => {
    const base = buildERPDocumentWorkbenchDefinition(document, module)
    return {
      ...base,
      actions: base.actions.filter((action) => !readOnly && (!objectType || objectType.actions.some((candidate) => candidate.key === action.id)))
        .map((action) => ({ ...action, disabledReasonKey: actionAvailable(document.tableCode, selectedRecord, action.id) ? undefined : 'ontology.actionUnavailable' })),
    }
  }, [document, module, objectType, selectedRecord, readOnly])

  useEffect(() => {
    if (readOnly) return
    let cancelled = false
    const editableFields = [
      ...defaultWorkbenchFields(document.tableCode, document.primaryKey),
      ...(document.childCode ? defaultWorkbenchLineFields(document.childCode) : []),
    ].filter((field) => !field.primary)
    const fieldNames = new Set(editableFields.map((field) => field.name))
    const sources = [
      { table: 'MCRD', fields: ['CardCode'], names: ['CardName'] },
      { table: 'MITM', fields: ['ItemCode', 'BaseItemCode'], names: ['ItemName'] },
      { table: 'MWHS', fields: ['WhsCode', 'SourceWhsCode', 'FinishedWhsCode'], names: ['WhsName'] },
      { table: 'MACT', fields: ['AccountCode', 'ParentAcctCode'], names: ['Name'] },
      { table: 'MPRC', fields: ['CostCenterCode'], names: ['Name'] },
    ].filter((source) => source.fields.some((name) => fieldNames.has(name)))
    void Promise.allSettled(sources.map((source) => listERPRecords<ERPBusinessRecord>(token, source.table, 200))).then((results) => {
      if (cancelled) return
      const options: WorkbenchLookupOptions = {}
      results.forEach((result, index) => {
        if (result.status !== 'fulfilled') return
        const source = sources[index]
        const isPurchasing = ['MPOR', 'MPDN', 'MPCH', 'MVPM'].includes(document.tableCode)
        const records = result.value.filter((record) => source.table !== 'MCRD' || !record.CardType || record.CardType === (isPurchasing ? 'S' : 'C'))
        const values = records.map((record) => {
          const name = source.names.map((field) => record[field]).find(Boolean)
          return { value: record.key, label: name ? `${String(name)} · ${record.key}` : record.key }
        })
        source.fields.forEach((field) => { options[field] = values })
      })
      setLookupOptions(options)
    })
    return () => { cancelled = true }
  }, [token, document.tableCode, document.primaryKey, document.childCode, readOnly])

  useEffect(() => {
    let cancelled = false
    async function load() {
      setLoading(true)
      setError('')
      try {
        let items: ERPBusinessRecord[]
        if (typeKey) {
          const [type, page] = await Promise.all([getOntologyType(token, typeKey), queryOntologyObjects(token, typeKey, search, cursor, {}, { status: filterStatus, sort: sortKey, direction: sortDirection })])
          if (cancelled) return
          setObjectType(type)
          items = page.objects.map((object) => ontologyRecord(object, type))
          setNextCursor(page.next_cursor ?? '')
          setTotalCount(page.total)
        } else {
          items = await listERPRecords<ERPBusinessRecord>(token, document.tableCode, 500)
          if (cancelled) return
          if (search) items = items.filter((item) => JSON.stringify(item).toLowerCase().includes(search.toLowerCase()))
          setNextCursor('')
          setTotalCount(items.length)
        }
        if (!cancelled) {
          setRecords((previous) => cursor ? Array.from(new Map([...previous, ...items].map((item) => [item.key, item])).values()) : items)
          setSelectedKey((current) => current || items[0]?.key || '')
        }
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : t('erp.business.loadFailed'))
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    void load()
    return () => { cancelled = true }
  }, [token, typeKey, document.tableCode, search, cursor, version, filterStatus, sortKey, sortDirection, t])

  useEffect(() => {
    if (!selectedKey) return
    let cancelled = false
    async function loadDetail() {
      setDetailLoading(true)
      try {
        const [record, rows, history, links] = await Promise.all([
          getBusinessRecord(token, document.tableCode, selectedKey),
          document.childCode ? listERPChildRecords<ERPBusinessRecord>(token, document.tableCode, selectedKey, document.childCode, 500) : Promise.resolve([]),
          typeKey ? getOntologyHistory(token, typeKey, selectedKey) : listERPActionExecutions(token, document.tableCode, selectedKey),
          typeKey ? getOntologyLinks(token, typeKey, selectedKey) : Promise.resolve({ links: [], truncated: false }),
        ])
        if (!cancelled) setDetail({ record: { ...record.data, key: record.key }, rows, history, links })
      } catch (err) {
        if (!cancelled) { setDetail(null); setError(err instanceof Error ? err.message : t('erp.business.loadFailed')) }
      } finally {
        if (!cancelled) setDetailLoading(false)
      }
    }
    void loadDetail()
    return () => { cancelled = true }
  }, [token, typeKey, document, selectedKey, version, t])

  function refresh() {
    setCursor('')
    setVersion((value) => value + 1)
  }

  async function mutate(task: () => Promise<unknown>, message: string) {
    setError('')
    setNotice('')
    await task()
    setNotice(t(message))
    refresh()
  }

  async function execute(action: string, data: Record<string, unknown>) {
    const signature = JSON.stringify([document.tableCode, selectedKey, action, data])
    if (requestID.current?.signature !== signature) requestID.current = { signature, id: crypto.randomUUID() }
    if (typeKey) await executeOntologyAction(token, typeKey, selectedKey, action, data, requestID.current.id)
    else await runERPAction(token, document.tableCode, selectedKey, action, data)
    requestID.current = null
    setNotice(t('erp.business.actionDone'))
    refresh()
  }

  const lineEditing = !readOnly && definition.detailTables[0]?.allowCreate && !['MREQ', 'MPRJ'].includes(document.tableCode)
  const sourceImportID = (selectedRecord?.provenance as { import_id?: string } | undefined)?.import_id
  return (
    <div className="min-w-0 space-y-3">
      {error && <FeedbackMessage error>{error}</FeedbackMessage>}
      {notice && <FeedbackMessage>{notice}</FeedbackMessage>}
      <DocumentWorkbench definition={definition} records={records} childRows={selectedRecord ? detail?.rows ?? [] : []}
        selectedKey={selectedKey} selectedRecord={selectedRecord} onSelectRecord={setSelectedKey} onRefresh={refresh}
        onCreateHeader={readOnly ? undefined : async (key, data) => { await mutate(() => createERPRecord(token, document.tableCode, key, data), 'erp.business.documentCreated'); setSelectedKey(key) }}
        onUpdateHeader={readOnly ? undefined : async (key, data) => mutate(() => updateERPRecord(token, document.tableCode, key, data), 'erp.business.documentUpdated')}
        onDeleteHeader={readOnly || ['MREQ', 'MPRJ'].includes(document.tableCode) ? undefined : async (key) => { await mutate(() => deleteERPRecord(token, document.tableCode, key), 'erp.business.documentDeleted'); setSelectedKey(''); setDetail(null) }}
        onCreateLine={!lineEditing ? undefined : async (key, data) => mutate(() => createERPChildRecord(token, document.tableCode, selectedKey, document.childCode!, key, data), 'erp.business.lineCreated')}
        onUpdateLine={!lineEditing ? undefined : async (key, data) => mutate(() => updateERPChildRecord(token, document.tableCode, selectedKey, document.childCode!, key, data), 'erp.business.lineUpdated')}
        onDeleteLine={!lineEditing ? undefined : async (key) => mutate(() => deleteERPChildRecord(token, document.tableCode, selectedKey, document.childCode!, key), 'erp.business.lineDeleted')}
        onExecute={execute} onSearch={(value) => { setCursor(''); setSearch(value); setSelectedKey(''); setDetail(null) }}
        totalCount={totalCount} filterStatus={typeKey ? filterStatus : undefined}
        onStatusFilter={typeKey ? (value) => { setCursor(''); setFilterStatus(value); setSelectedKey(''); setDetail(null) } : undefined}
        sortKey={sortKey} sortDirection={sortDirection}
        onSort={typeKey ? (key, direction) => { setCursor(''); setSortKey(key); setSortDirection(direction) } : undefined}
        onImport={!readOnly && objectType?.importable ? () => setImportView({}) : undefined}
        onOpenSource={sourceImportID ? () => setImportView({ id: sourceImportID }) : undefined}
        onLoadMore={nextCursor && !loading ? () => setCursor(nextCursor) : undefined}
        links={selectedRecord ? detail?.links.links ?? [] : []} linksTruncated={detail?.links.truncated}
        history={selectedRecord ? detail?.history ?? [] : []} onOpenLink={onOpenLink}
        busy={loading} detailLoading={detailLoading} readOnly={readOnly} version={version} lookupOptions={lookupOptions} />
      {!readOnly && document.tableCode === 'MPRJ' && selectedKey && <BusinessAIWorkbench token={token} projectID={selectedKey} />}
      {importView && typeKey && <DocumentImportWorkspace token={token} objectType={typeKey} title={t(document.labelKey)} initialID={importView.id}
        lookupOptions={lookupOptions} onClose={() => setImportView(null)} onConfirmed={(key) => { setSelectedKey(key); refresh() }} />}
    </div>
  )
}

function TrialBalanceReport({ token }: { token: string }) {
  const { t, locale } = useI18n()
  const [filters, setFilters] = useState({ currency: 'CNY', period_start: '', period_end: '' })
  const [balance, setBalance] = useState<FinanceGLTrialBalance | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(true)
  const [version, setVersion] = useState(0)
  useEffect(() => {
    let cancelled = false
    getFinanceGLTrialBalance(token, filters).then((value) => { if (!cancelled) { setBalance(value); setError('') } })
      .catch((err) => { if (!cancelled) { setBalance(null); setError(err instanceof Error ? err.message : t('erp.business.loadFailed')) } })
      .finally(() => { if (!cancelled) setBusy(false) })
    return () => { cancelled = true }
  }, [token, filters, version, t])
  const amount = (value: number) => new Intl.NumberFormat(locale === 'zh' ? 'zh-CN' : 'en-US', { maximumFractionDigits: 6 }).format(value)
  function download() {
    if (!balance) return
    const columns = ['account', 'name', 'debit', 'credit', 'balance'].map((name) => t('ontology.field.' + name))
    const rows = [columns, ...balance.rows.map((row) => [row.account_code, row.account_name, row.debit, row.credit, row.net_amount])]
    const csv = rows.map((row) => row.map((cell) => '"' + String(cell).replace(/"/g, '""').replace(/^[=+\-@]/, "'$&") + '"').join(',')).join('\r\n')
    const url = URL.createObjectURL(new Blob(['\uFEFF', csv], { type: 'text/csv;charset=utf-8' }))
    const link = window.document.createElement('a')
    link.href = url
    link.download = 'trial-balance.csv'
    link.click()
    URL.revokeObjectURL(url)
  }
  return <section className="min-w-0 border-y border-slate-200 bg-white p-4">
    <h3 className="text-sm font-semibold">{t('erp.document.trialBalance')}</h3>
    <form className="my-4 flex flex-wrap items-end gap-3" onSubmit={(event) => {
      event.preventDefault()
      setBusy(true)
      const data = new FormData(event.currentTarget)
      setFilters({ currency: String(data.get('currency')).trim().toUpperCase(), period_start: String(data.get('period_start')), period_end: String(data.get('period_end')) })
      setVersion((value) => value + 1)
    }}>
      {(['currency', 'period_start', 'period_end'] as const).map((name) => <label key={name} className="min-w-0"><span className="mb-1 block text-xs text-slate-500">{t('ontology.field.' + name)}</span><input className="h-9 w-36 max-w-full rounded-md border border-slate-300 px-2 text-sm" type={name === 'currency' ? 'text' : 'date'} name={name} defaultValue={filters[name]} required={name === 'currency'} pattern={name === 'currency' ? '[A-Za-z]{3}' : undefined} /></label>)}
      <button className="flex h-9 w-9 items-center justify-center rounded-md border border-slate-300" disabled={busy} title={t('common.refresh')} aria-label={t('common.refresh')}><RefreshCw size={16} /></button>
      <button type="button" className="flex h-9 w-9 items-center justify-center rounded-md border border-slate-300" onClick={download} disabled={!balance} title={t('ontology.download')} aria-label={t('ontology.download')}><Download size={16} /></button>
    </form>
    {error && <p role="alert" className="mb-3 break-words text-sm text-red-700">{error}</p>}
    {balance && <div className="overflow-auto"><table className="w-full min-w-[520px] text-left text-sm">
      <thead className="bg-slate-50"><tr>{['account', 'name', 'debit', 'credit', 'balance'].map((name) => <th key={name} className="border-b border-slate-200 p-2 font-medium">{t('ontology.field.' + name)}</th>)}</tr></thead>
      <tbody>{balance.rows.map((row) => <tr key={row.account_code} className="border-b border-slate-100"><td className="p-2 font-mono">{row.account_code}</td><td className="p-2">{row.account_name}</td><td className="p-2 tabular-nums">{amount(row.debit)}</td><td className="p-2 tabular-nums">{amount(row.credit)}</td><td className="p-2 tabular-nums">{amount(row.net_amount)}</td></tr>)}</tbody>
      <tfoot className="bg-teal-50 font-medium"><tr><td colSpan={2} className="p-2">{t('ontology.field.total')} ({balance.currency})</td><td className="p-2">{amount(balance.total_debit)}</td><td className="p-2">{amount(balance.total_credit)}</td><td className="p-2">{amount(balance.total_debit - balance.total_credit)}</td></tr></tfoot>
    </table></div>}
  </section>
}

function actionAvailable(table: string, record: ERPBusinessRecord | undefined, action: string): boolean {
  if (!record) return false
  const approved = record.WddStatus === 'A'
  if ((record.DocStatus === 'C' || record.BtfStatus === 'P' || record.Posted === 'Y') && !['refresh-cost', 'close-feedback'].includes(action)) return false
  switch (table + ':' + action) {
    case 'MPOR:submit': return !approved && record.DocStatus !== 'S'
    case 'MPOR:approve': return record.DocStatus === 'S' && !approved
    case 'MPOR:receive':
    case 'MRDR:deliver': return approved && !record.FulfillmentEntry
    case 'MRDR:confirm': return record.Confirmed !== 'Y' && !approved
    case 'MRDR:approve': return record.Confirmed === 'Y' && !approved
    case 'MPDN:approve':
    case 'MDLN:approve': return !approved
    case 'MPDN:post':
    case 'MDLN:post': return approved
    case 'MRCT:allocate':
    case 'MVPM:allocate': return Number(record.OpenBal ?? record.DocTotal) > 0
    case 'MREQ:approve': return record.Status === 'analyzed'
    case 'MREQ:convert-to-project': return record.Status === 'approved'
    case 'MREQ:analyze': return !['analyzed', 'approved', 'converted'].includes(String(record.Status))
    case 'MWOR:close': return record.Status === 'completed'
    default: return true
  }
}
