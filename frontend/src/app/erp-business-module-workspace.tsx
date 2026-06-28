'use client'

import { CheckCircle2, Clock3, FileText, Play, Plus, RefreshCw, Rows3, ShieldAlert } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'

import {
  createERPChildRecord,
  createERPRecord,
  deleteERPChildRecord,
  deleteERPRecord,
  getFinanceGLTrialBalance,
  listERPActionExecutions,
  listERPChildRecords,
  listERPRecords,
  listRuntimeOperations,
  runERPAction,
  updateERPChildRecord,
  updateERPRecord,
  type ERPActionExecution,
  type ERPActionResult,
  type FinanceGLTrialBalance,
} from '@/lib/api'
import { useI18n } from '@/lib/i18n'
import type { ApiOperation } from '@/lib/operations'
import {
  defaultWorkbenchFields,
  defaultWorkbenchLineFields,
  type DocumentWorkbenchDefinition,
} from '@/lib/workbench'
import { DocumentWorkbench } from './document-workbench'

type ERPBusinessModule = 'project' | 'procurement' | 'sales' | 'inventory' | 'finance' | 'retail'

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
  sortOrder?: number
  actionParams?: Record<string, unknown>
  kind?: 'document' | 'report'
}

type ERPBusinessModuleWorkspaceProps = {
  token: string
  module: ERPBusinessModule
  externalSelection?: BusinessSelection | null
  activeDocumentID?: string | null
}

type ERPBusinessRecord = Record<string, unknown> & { key: string }

type ERPActionAvailability = {
  action: string
  available: boolean
  reasonKey: string
}

type ERPTimelineEvent = {
  id: string
  titleKey: string
  detail: string
}

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
  finance: [
    { id: 'gl_account', labelKey: 'erp.document.glAccount', submoduleKey: 'erp.submodule.chartOfAccounts', tableCode: 'MACT', primaryKey: 'AcctCode', childCode: 'AACT', sortOrder: 10 },
    { id: 'cost_center', labelKey: 'erp.document.costCenter', submoduleKey: 'erp.submodule.costCenters', tableCode: 'MPRC', primaryKey: 'PrcCode', childCode: 'APRC', sortOrder: 20 },
    { id: 'journal_entry', labelKey: 'erp.document.journalEntry', submoduleKey: 'erp.submodule.journalEntries', tableCode: 'MJDT', primaryKey: 'TransId', childCode: 'JDT1', actions: ['post'], sortOrder: 30 },
    { id: 'trial_balance', labelKey: 'erp.document.trialBalance', submoduleKey: 'erp.submodule.trialBalance', tableCode: 'MGLR', primaryKey: 'ReportCode', actions: ['run'], sortOrder: 40, kind: 'report' },
    { id: 'ar_invoice', labelKey: 'erp.document.arInvoice', submoduleKey: 'erp.submodule.arInvoices', tableCode: 'MINV', primaryKey: 'DocEntry', childCode: 'INV1', actions: ['post'], sortOrder: 50 },
    { id: 'ap_invoice', labelKey: 'erp.document.apInvoice', submoduleKey: 'erp.submodule.apInvoices', tableCode: 'MPCH', primaryKey: 'DocEntry', childCode: 'PCH1', sortOrder: 60 },
    { id: 'incoming_payment', labelKey: 'erp.document.incomingPayment', submoduleKey: 'erp.submodule.incomingPayments', tableCode: 'MRCT', primaryKey: 'DocEntry', childCode: 'RCT1', actions: ['allocate'], sortOrder: 70 },
  ],
}

function deriveRuntimeDocuments(operations: ApiOperation[], module: ERPBusinessModule): DocumentConfig[] {
  const byID = new Map<string, DocumentConfig>()
  for (const operation of operations) {
    const workspace = recordMap(operation.metadata?.workspace)
    if (workspace.module !== module) continue
    const tableCode = textValue(workspace.table_code)
    const primaryKey = textValue(workspace.primary_key)
    const documentID = textValue(workspace.document_id)
    if (!tableCode || !primaryKey || !documentID) continue
    const current = byID.get(documentID) ?? {
      id: documentID,
      labelKey: textValue(workspace.document_label_key) || `erp.document.${documentID}`,
      submoduleKey: textValue(workspace.submodule_key) || `erp.submodule.${documentID}`,
      tableCode,
      primaryKey,
      childCode: textValue(workspace.child_code) || undefined,
      actions: [],
      sortOrder: numberValue(workspace.sort_order),
      kind: textValue(workspace.kind) === 'report' ? 'report' : 'document',
    }
    const action = textValue(workspace.action)
    if (action && !(current.actions ?? []).includes(action)) {
      current.actions = [...(current.actions ?? []), action]
    }
    if (workspace.action_params && typeof workspace.action_params === 'object') {
      current.actionParams = workspace.action_params as Record<string, unknown>
    }
    byID.set(documentID, current)
  }
  return Array.from(byID.values()).sort((left, right) => (left.sortOrder ?? 999) - (right.sortOrder ?? 999) || left.id.localeCompare(right.id))
}

function mergeDocumentConfigs(fallback: DocumentConfig[], runtime: DocumentConfig[]) {
  if (runtime.length === 0) return fallback
  const merged = runtime.map((document) => {
    const existing = fallback.find((item) => item.id === document.id || item.tableCode === document.tableCode)
    return existing ? { ...existing, ...document, actions: mergeActions(existing.actions, document.actions) } : document
  })
  for (const document of fallback) {
    if (!merged.some((item) => item.id === document.id || item.tableCode === document.tableCode)) {
      merged.push(document)
    }
  }
  return merged.sort((left, right) => (left.sortOrder ?? 999) - (right.sortOrder ?? 999) || left.id.localeCompare(right.id))
}

function mergeActions(left: string[] | undefined, right: string[] | undefined) {
  const items: string[] = []
  for (const action of [...(left ?? []), ...(right ?? [])]) {
    if (!items.includes(action)) items.push(action)
  }
  return items.length > 0 ? items : undefined
}

async function loadBusinessRecords(token: string, document: DocumentConfig): Promise<ERPBusinessRecord[]> {
  if (document.kind === 'report' && document.tableCode === 'MGLR') {
    return trialBalanceRecords(await getFinanceGLTrialBalance(token, { currency: 'CNY' }))
  }
  return listERPRecords<ERPBusinessRecord>(token, document.tableCode, 100)
}

function trialBalanceRecords(balance: FinanceGLTrialBalance): ERPBusinessRecord[] {
  return (balance.rows ?? []).map((row) => ({
    ...row,
    key: row.account_code,
    ReportCode: 'trial-balance',
    Currency: balance.currency,
    TotalDebit: balance.total_debit,
    TotalCredit: balance.total_credit,
  }))
}

export function buildERPDocumentWorkbenchDefinition(
  document: DocumentConfig,
  module: ERPBusinessModule,
  operations: ApiOperation[],
): DocumentWorkbenchDefinition {
  const documentOperations = operations.filter((operation) => {
    const workspace = recordMap(operation.metadata?.workspace)
    return textValue(workspace.module) === module && textValue(workspace.table_code) === document.tableCode
  })
  const actionOperations = (document.actions ?? []).map((action) => {
    const operation = documentOperations.find((item) => textValue(recordMap(item.metadata?.workspace).action) === action)
    return {
      id: action,
      labelKey: `erp.action.${action}`,
      operation,
      dangerLevel: operation?.dangerLevel,
      disabledReasonKey: operation ? undefined : 'workbench.api.operationMissing',
    }
  })

  return {
    id: `${module}.${document.id}`,
    moduleKey: `erp.module.${module}`,
    titleKey: document.labelKey,
    tableName: document.tableCode,
    primaryKey: document.primaryKey,
    headerFields: defaultWorkbenchFields(document.tableCode, document.primaryKey),
    detailTables: document.childCode
      ? [
          {
            tableName: document.childCode,
            labelKey: 'workbench.detail',
            parentKey: document.primaryKey,
            lineKey: 'LineNum',
            fields: defaultWorkbenchLineFields(document.childCode),
            allowCreate: document.kind !== 'report',
            allowDelete: document.kind !== 'report',
          },
        ]
      : [],
    actions: actionOperations,
    links: [
      { id: 'module', labelKey: 'workbench.link.module', href: `#module-${module}`, kind: 'module' },
      { id: 'table', labelKey: 'workbench.link.masterTable', href: `#table-${document.tableCode}`, kind: 'table' },
      ...(document.childCode ? [{ id: 'child', labelKey: 'workbench.link.detailTable', href: `#table-${document.childCode}`, kind: 'table' as const }] : []),
      ...documentOperations.map((operation) => ({
        id: operation.id,
        labelKey: operation.title,
        href: `#operation-${operation.id}`,
        kind: 'operation' as const,
      })),
    ],
    fieldPermissions: [
      {
        table_name: document.tableCode,
        field_name: document.primaryKey,
        action: 'delete',
        behavior: 'deny',
        reason: 'strong business primary key',
        priority: 1000,
        status: 'active',
      },
      {
        table_name: document.tableCode,
        field_name: 'DocStatus',
        action: 'write',
        behavior: 'readonly',
        reason: 'status is changed through approved actions',
        priority: 900,
        status: 'active',
      },
    ],
  }
}

export function ERPBusinessModuleWorkspace({ token, module, externalSelection, activeDocumentID }: ERPBusinessModuleWorkspaceProps) {
  const { t } = useI18n()
  const fallbackDocuments = moduleDocuments[module]
  const [runtimeOperations, setRuntimeOperations] = useState<ApiOperation[]>([])
  const [runtimeDocuments, setRuntimeDocuments] = useState<DocumentConfig[]>([])
  const documents = useMemo(() => mergeDocumentConfigs(fallbackDocuments, runtimeDocuments), [fallbackDocuments, runtimeDocuments])
  const [activeID, setActiveID] = useState(documents[0]?.id ?? '')
  const activeDocument = useMemo(() => documents.find((item) => item.id === activeID) ?? documents[0], [activeID, documents])
  const [records, setRecords] = useState<ERPBusinessRecord[]>([])
  const [childRows, setChildRows] = useState<ERPBusinessRecord[]>([])
  const [actionExecutions, setActionExecutions] = useState<ERPActionExecution[]>([])
  const [selectedKey, setSelectedKey] = useState('')
  const [form, setForm] = useState({ key: '', name: '', cardCode: '', itemCode: '', whsCode: '', quantity: '1', price: '0', targetKey: '', amount: '0' })
  const [lineForm, setLineForm] = useState({ lineNum: '1', itemCode: '', whsCode: '', quantity: '1', price: '0' })
  const [actionResult, setActionResult] = useState<ERPActionResult<ERPBusinessRecord> | null>(null)
  const [busy, setBusy] = useState(false)
  const [notice, setNotice] = useState('')
  const [error, setError] = useState('')

  const selectedRecord = records.find((record) => record.key === selectedKey)
  const actionAvailability = useMemo(
    () => (activeDocument?.actions ?? []).map((action) => isERPActionAvailable(activeDocument, selectedRecord, action)),
    [activeDocument, selectedRecord],
  )
  const availableActions = actionAvailability.filter((item) => item.available)
  const blockedActions = actionAvailability.filter((item) => !item.available)
  const currentActionResult = actionResult?.table_code === activeDocument?.tableCode && actionResult.key === selectedKey ? actionResult : null
  const generatedRecords = currentActionResult?.generated_records ?? generatedRecordsFromExecutions(actionExecutions)
  const assistantProposals = useMemo(() => recordArray(selectedRecord?.assistant_confirmed_proposals), [selectedRecord])
  const businessTimeline = useMemo(
    () => buildBusinessTimeline(selectedRecord, childRows, currentActionResult, assistantProposals, actionExecutions),
    [actionExecutions, assistantProposals, childRows, currentActionResult, selectedRecord],
  )
  const workbenchDefinition = useMemo(
    () => (activeDocument ? buildERPDocumentWorkbenchDefinition(activeDocument, module, runtimeOperations) : null),
    [activeDocument, module, runtimeOperations],
  )

  useEffect(() => {
    let cancelled = false
    listRuntimeOperations(token)
      .then((operations) => {
        if (!cancelled) {
          setRuntimeOperations(operations)
          setRuntimeDocuments(deriveRuntimeDocuments(operations, module))
        }
      })
      .catch(() => {
        if (!cancelled) {
          setRuntimeOperations([])
          setRuntimeDocuments([])
        }
      })
    return () => {
      cancelled = true
    }
  }, [module, token])

  useEffect(() => {
    if (activeDocumentID && documents.some((document) => document.id === activeDocumentID)) {
      const timer = window.setTimeout(() => setActiveID(activeDocumentID), 0)
      return () => window.clearTimeout(timer)
    }
  }, [activeDocumentID, documents])

  async function loadRecords(document = activeDocument) {
    if (!document) return
    setBusy(true)
    setError('')
    try {
      const items = await loadBusinessRecords(token, document)
      setRecords(items)
      setSelectedKey((current) => {
        if (current && items.some((item) => item.key === current)) return current
        if (externalSelection?.targetID && items.some((item) => item.key === externalSelection.targetID)) return externalSelection.targetID
        return items[0]?.key || ''
      })
      if (document.kind === 'report') {
        setChildRows([])
        setActionExecutions([])
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : t('erp.business.loadFailed'))
    } finally {
      setBusy(false)
    }
  }

  async function loadChildRows(document = activeDocument, key = selectedKey) {
    if (!document?.childCode || !key) {
      setChildRows([])
      return
    }
    try {
      const rows = await listERPChildRecords<ERPBusinessRecord>(token, document.tableCode, key, document.childCode, 100)
      setChildRows(rows)
    } catch (err) {
      setChildRows([])
      setError(err instanceof Error ? err.message : t('erp.business.loadFailed'))
    }
  }

  async function loadActionExecutions(document = activeDocument, key = selectedKey) {
    if (!document || !key) {
      setActionExecutions([])
      return
    }
    try {
      const items = await listERPActionExecutions(token, document.tableCode, key, 50)
      setActionExecutions(items)
    } catch {
      setActionExecutions([])
    }
  }

  useEffect(() => {
    if (!activeDocument) return
    let cancelled = false
    loadBusinessRecords(token, activeDocument)
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

  useEffect(() => {
    if (!activeDocument?.childCode || activeDocument.kind === 'report' || !selectedKey) {
      const timer = window.setTimeout(() => setChildRows([]), 0)
      return () => window.clearTimeout(timer)
    }
    let cancelled = false
    listERPChildRecords<ERPBusinessRecord>(token, activeDocument.tableCode, selectedKey, activeDocument.childCode, 100)
      .then((items) => {
        if (!cancelled) setChildRows(items)
      })
      .catch((err) => {
        if (!cancelled) {
          setChildRows([])
          setError(err instanceof Error ? err.message : t('erp.business.loadFailed'))
        }
      })
    return () => {
      cancelled = true
    }
  }, [activeDocument, selectedKey, t, token])

  useEffect(() => {
    if (!activeDocument || activeDocument.kind === 'report' || !selectedKey) {
      const timer = window.setTimeout(() => setActionExecutions([]), 0)
      return () => window.clearTimeout(timer)
    }
    let cancelled = false
    listERPActionExecutions(token, activeDocument.tableCode, selectedKey, 50)
      .then((items) => {
        if (!cancelled) setActionExecutions(items)
      })
      .catch(() => {
        if (!cancelled) setActionExecutions([])
      })
    return () => {
      cancelled = true
    }
  }, [activeDocument, selectedKey, token])

  async function handleCreateRecord() {
    if (!activeDocument || activeDocument.kind === 'report' || !form.key.trim()) return
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

  async function handleWorkbenchCreateHeader(key: string, data: Record<string, unknown>) {
    if (!activeDocument || activeDocument.kind === 'report') return
    setBusy(true)
    setError('')
    try {
      await createERPRecord(token, activeDocument.tableCode, key, data)
      setNotice(t('erp.business.documentCreated'))
      await loadRecords(activeDocument)
      setSelectedKey(key)
    } catch (err) {
      setError(err instanceof Error ? err.message : t('common.operationFailed'))
    } finally {
      setBusy(false)
    }
  }

  async function handleWorkbenchUpdateHeader(key: string, data: Record<string, unknown>) {
    if (!activeDocument || activeDocument.kind === 'report') return
    setBusy(true)
    setError('')
    try {
      await updateERPRecord(token, activeDocument.tableCode, key, data)
      setNotice(t('erp.business.documentUpdated'))
      await loadRecords(activeDocument)
      setSelectedKey(key)
    } catch (err) {
      setError(err instanceof Error ? err.message : t('common.operationFailed'))
    } finally {
      setBusy(false)
    }
  }

  async function handleWorkbenchDeleteHeader(key: string) {
    if (!activeDocument || activeDocument.kind === 'report') return
    setBusy(true)
    setError('')
    try {
      await deleteERPRecord(token, activeDocument.tableCode, key)
      setNotice(t('erp.business.documentDeleted'))
      setSelectedKey('')
      await loadRecords(activeDocument)
    } catch (err) {
      setError(err instanceof Error ? err.message : t('common.operationFailed'))
    } finally {
      setBusy(false)
    }
  }

  async function handleCreateLine() {
    if (!activeDocument?.childCode || activeDocument.kind === 'report' || !selectedKey) return
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
      await loadChildRows(activeDocument, selectedKey)
    } catch (err) {
      setError(err instanceof Error ? err.message : t('common.operationFailed'))
    } finally {
      setBusy(false)
    }
  }

  async function handleWorkbenchCreateLine(lineKey: string, data: Record<string, unknown>) {
    if (!activeDocument?.childCode || activeDocument.kind === 'report' || !selectedKey) return
    setBusy(true)
    setError('')
    try {
      await createERPChildRecord(token, activeDocument.tableCode, selectedKey, activeDocument.childCode, lineKey, data)
      setNotice(t('erp.business.lineCreated'))
      await loadChildRows(activeDocument, selectedKey)
    } catch (err) {
      setError(err instanceof Error ? err.message : t('common.operationFailed'))
    } finally {
      setBusy(false)
    }
  }

  async function handleWorkbenchUpdateLine(lineKey: string, data: Record<string, unknown>) {
    if (!activeDocument?.childCode || activeDocument.kind === 'report' || !selectedKey) return
    setBusy(true)
    setError('')
    try {
      await updateERPChildRecord(token, activeDocument.tableCode, selectedKey, activeDocument.childCode, lineKey, data)
      setNotice(t('erp.business.lineUpdated'))
      await loadChildRows(activeDocument, selectedKey)
    } catch (err) {
      setError(err instanceof Error ? err.message : t('common.operationFailed'))
    } finally {
      setBusy(false)
    }
  }

  async function handleWorkbenchDeleteLine(lineKey: string) {
    if (!activeDocument?.childCode || activeDocument.kind === 'report' || !selectedKey) return
    setBusy(true)
    setError('')
    try {
      await deleteERPChildRecord(token, activeDocument.tableCode, selectedKey, activeDocument.childCode, lineKey)
      setNotice(t('erp.business.lineDeleted'))
      await loadChildRows(activeDocument, selectedKey)
    } catch (err) {
      setError(err instanceof Error ? err.message : t('common.operationFailed'))
    } finally {
      setBusy(false)
    }
  }

  async function handleAction(action: string) {
    if (!activeDocument || (!selectedKey && activeDocument.kind !== 'report')) return
    setBusy(true)
    setError('')
    setNotice('')
    try {
      if (activeDocument.kind === 'report' && activeDocument.tableCode === 'MGLR' && action === 'run') {
        await loadRecords(activeDocument)
        setNotice(t('erp.business.reportRan'))
        return
      }
      const result = await runERPAction<ERPBusinessRecord>(token, activeDocument.tableCode, selectedKey, action, buildActionData(action, form))
      setActionResult(result)
      setNotice(t('erp.business.actionDone'))
      await loadRecords(activeDocument)
      await loadChildRows(activeDocument, selectedKey)
      await loadActionExecutions(activeDocument, selectedKey)
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
        <div className="space-y-4">
          {workbenchDefinition && (
            <DocumentWorkbench
              token={token}
              definition={workbenchDefinition}
              records={records}
              childRows={childRows}
              selectedKey={selectedKey}
              onSelectRecord={setSelectedKey}
              onRefresh={() => void loadRecords(activeDocument)}
              onCreateHeader={handleWorkbenchCreateHeader}
              onUpdateHeader={handleWorkbenchUpdateHeader}
              onDeleteHeader={handleWorkbenchDeleteHeader}
              onCreateLine={handleWorkbenchCreateLine}
              onUpdateLine={handleWorkbenchUpdateLine}
              onDeleteLine={handleWorkbenchDeleteLine}
              busy={busy}
            />
          )}

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
            <ERPDocumentDetail
              document={activeDocument}
              record={selectedRecord}
              childRows={childRows}
              generatedRecords={generatedRecords}
              assistantProposals={assistantProposals}
              businessTimeline={businessTimeline}
            />

            {activeDocument.kind !== 'report' && (
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
            )}

            {activeDocument.kind !== 'report' && activeDocument.childCode && (
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
                <h3 className="text-sm font-semibold text-slate-950">{t('erp.business.availableActions')}</h3>
                {activeDocument.tableCode === 'MRCT' && (
                  <div className="mt-3 space-y-2">
                    <ERPInput label={t('erp.business.targetKey')} value={form.targetKey} onChange={(value) => setForm((current) => ({ ...current, targetKey: value }))} placeholder="INV-1001" />
                    <ERPInput label={t('erp.business.amount')} value={form.amount} onChange={(value) => setForm((current) => ({ ...current, amount: value }))} placeholder="100" />
                  </div>
                )}
                <div className="mt-3 grid gap-2">
                  {availableActions.map(({ action }) => (
                    <button
                      key={action}
                      type="button"
                      onClick={() => void handleAction(action)}
                      disabled={busy || (!selectedKey && activeDocument.kind !== 'report')}
                      className="inline-flex h-9 items-center justify-center gap-2 rounded-md border border-slate-300 bg-white px-3 text-sm font-semibold text-slate-800 transition hover:bg-slate-50 disabled:opacity-50"
                    >
                      <Play className="h-4 w-4" />
                      {t(`erp.action.${action}`)}
                    </button>
                  ))}
                  {availableActions.length === 0 && (
                    <p className="rounded-md border border-dashed border-slate-300 px-3 py-2 text-xs text-slate-500">{t('erp.business.actionBlocked')}</p>
                  )}
                </div>
                {blockedActions.length > 0 && (
                  <div className="mt-4 border-t border-slate-100 pt-3">
                    <p className="text-xs font-semibold text-slate-500">{t('erp.business.unavailableActions')}</p>
                    <div className="mt-2 space-y-1">
                      {blockedActions.map((item) => (
                        <div key={item.action} className="flex items-start gap-2 rounded-md bg-slate-50 px-2 py-1.5 text-xs text-slate-600">
                          <ShieldAlert className="mt-0.5 h-3.5 w-3.5 shrink-0 text-amber-600" />
                          <span>{t('erp.business.actionBlocked', { action: t(`erp.action.${item.action}`), reason: t(item.reasonKey) })}</span>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              </section>
            )}

            {(notice || error) && (
              <p className={`rounded-md px-3 py-2 text-sm ${error ? 'bg-red-50 text-red-700' : 'bg-emerald-50 text-emerald-700'}`}>
                {error || notice}
              </p>
            )}
          </aside>
          </div>
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

function ERPDocumentDetail({
  document,
  record,
  childRows,
  generatedRecords,
  assistantProposals,
  businessTimeline,
}: {
  document: DocumentConfig
  record?: ERPBusinessRecord
  childRows: ERPBusinessRecord[]
  generatedRecords: NonNullable<ERPActionResult['generated_records']>
  assistantProposals: Array<Record<string, unknown>>
  businessTimeline: ERPTimelineEvent[]
}) {
  const { t } = useI18n()
  if (!record) {
    return (
      <section className="rounded-lg border border-slate-200 bg-white p-4">
        <h3 className="text-sm font-semibold text-slate-950">{t('erp.business.documentDetail')}</h3>
        <p className="mt-3 rounded-md border border-dashed border-slate-300 px-3 py-4 text-sm text-slate-500">{t('common.notSelected')}</p>
      </section>
    )
  }
  const status = recordStatus(record) || 'ready'
  const fields =
    document.kind === 'report'
      ? [
          { label: 'finance.accountCode', value: String(record.account_code || record.key) },
          { label: 'common.name', value: String(record.account_name || '') },
          { label: 'finance.debit', value: displayValue(record.debit) },
          { label: 'finance.credit', value: displayValue(record.credit) },
          { label: 'finance.netAmount', value: displayValue(record.net_amount) },
          { label: 'finance.currency', value: String(record.Currency || '') },
        ]
      : [
          { label: document.primaryKey, value: record.key },
          { label: 'erp.business.statusReason', value: status },
          { label: 'erp.business.relatedProject', value: String(record.ProjectCode || record.PrjCode || record.RequirementCode || record.BaseEntry || '') },
          { label: 'erp.business.costImpact', value: String(record.DocTotal || record.PaidToDate || record.OpenBal || record.LastCostCode || '') },
        ]
  return (
    <section className="rounded-lg border border-slate-200 bg-white p-4">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h3 className="text-sm font-semibold text-slate-950">{t('erp.business.documentDetail')}</h3>
          <p className="mt-1 truncate font-mono text-xs text-slate-500">{document.tableCode} / {record.key}</p>
        </div>
        <ERPStatusPill value={status} />
      </div>
      <div className="mt-4 grid gap-2 sm:grid-cols-2">
        {fields.map((field) => (
          <div key={field.label} className="min-w-0 border-b border-slate-100 pb-2">
            <p className="text-[11px] font-semibold uppercase tracking-normal text-slate-500">{field.label.startsWith('erp.') ? t(field.label) : field.label}</p>
            <p className="mt-1 truncate text-sm font-semibold text-slate-900">{field.value || t('common.none')}</p>
          </div>
        ))}
      </div>
      <div className="mt-4 border-t border-slate-100 pt-3">
        <div className="flex items-center justify-between gap-3">
          <p className="text-xs font-semibold text-slate-500">{t('erp.business.childRows')}</p>
          <span className="font-mono text-xs text-slate-500">{childRows.length}</span>
        </div>
        {childRows.length > 0 ? (
          <div className="mt-2 max-h-36 overflow-auto rounded-md border border-slate-200">
            <table className="min-w-full divide-y divide-slate-100 text-xs">
              <tbody className="divide-y divide-slate-100">
                {childRows.map((row) => (
                  <tr key={row.key}>
                    <td className="px-2 py-1.5 font-mono text-slate-600">{row.key}</td>
                    <td className="px-2 py-1.5 text-slate-700">{displayValue(row.ItemCode || row.WhsCode || row.LineStatus || row.Name)}</td>
                    <td className="px-2 py-1.5 text-right text-slate-600">{displayValue(row.Quantity || row.Price || row.Amount)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <p className="mt-2 rounded-md border border-dashed border-slate-300 px-3 py-3 text-xs text-slate-500">{t('erp.business.noChildRows')}</p>
        )}
      </div>
      <div className="mt-4 grid gap-3 border-t border-slate-100 pt-3">
        <div>
          <p className="text-xs font-semibold text-slate-500">{t('erp.business.generatedRecords')}</p>
          {generatedRecords.length > 0 ? (
            <div className="mt-2 flex flex-wrap gap-2">
              {generatedRecords.map((item) => (
                <span key={`${item.table_code}-${item.key}`} className="rounded-md bg-slate-100 px-2 py-1 font-mono text-xs text-slate-700">
                  {item.table_code}:{item.key}
                </span>
              ))}
            </div>
          ) : (
            <p className="mt-2 text-xs text-slate-500">{t('common.none')}</p>
          )}
        </div>
        <div>
          <p className="text-xs font-semibold text-slate-500">{t('erp.business.assistantProposals')}</p>
          {assistantProposals.length > 0 ? (
            <div className="mt-2 space-y-1">
              {assistantProposals.map((item, index) => (
                <p key={`${displayValue(item.proposal_id)}-${index}`} className="truncate rounded-md bg-slate-50 px-2 py-1 text-xs text-slate-600">
                  {displayValue(item.title || item.summary || item.action || item.proposal_type)}
                </p>
              ))}
            </div>
          ) : (
            <p className="mt-2 text-xs text-slate-500">{t('common.none')}</p>
          )}
        </div>
      </div>
      <ERPDocumentTimeline events={businessTimeline} />
    </section>
  )
}

function ERPDocumentTimeline({ events }: { events: ERPTimelineEvent[] }) {
  const { t } = useI18n()
  return (
    <div className="mt-4 border-t border-slate-100 pt-3">
      <div className="mb-2 flex items-center gap-2">
        <Clock3 className="h-4 w-4 text-slate-400" />
        <p className="text-xs font-semibold text-slate-500">{t('erp.business.timeline')}</p>
      </div>
      {events.length > 0 ? (
        <ol className="space-y-2">
          {events.map((event) => (
            <li key={event.id} className="flex gap-2 text-xs">
              <CheckCircle2 className="mt-0.5 h-3.5 w-3.5 shrink-0 text-emerald-600" />
              <div className="min-w-0">
                <p className="font-semibold text-slate-800">{t(event.titleKey)}</p>
                <p className="truncate text-slate-500">{event.detail}</p>
              </div>
            </li>
          ))}
        </ol>
      ) : (
        <p className="rounded-md border border-dashed border-slate-300 px-3 py-3 text-xs text-slate-500">{t('erp.business.noTimeline')}</p>
      )}
    </div>
  )
}

function ERPStatusPill({ value }: { value: string }) {
  const { t } = useI18n()
  const tone =
    ['approved', 'A', 'posted', 'P', 'converted', 'closed', 'C', 'confirmed'].includes(value)
      ? 'border-emerald-200 bg-emerald-50 text-emerald-700'
      : ['S', 'submitted', 'analyzed'].includes(value)
        ? 'border-amber-200 bg-amber-50 text-amber-700'
        : 'border-slate-200 bg-slate-50 text-slate-700'
  return <span className={`inline-flex h-7 max-w-[140px] items-center truncate rounded-full border px-2.5 text-xs font-semibold ${tone}`}>{t(value)}</span>
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

function isERPActionAvailable(document: DocumentConfig | undefined, record: ERPBusinessRecord | undefined, action: string): ERPActionAvailability {
  if (!document || !record) {
    if (document?.kind === 'report' && action === 'run') {
      return { action, available: true, reasonKey: 'ready' }
    }
    return { action, available: false, reasonKey: 'common.notSelected' }
  }
  if (isClosedOrPosted(record) && !['refresh-cost', 'close-feedback'].includes(action)) {
    return { action, available: false, reasonKey: 'closed' }
  }
  const status = normalizedStatus(record)
  const approvalStatus = normalizedText(record.WddStatus)
  const posted = normalizedText(record.Posted) === 'y'
  switch (`${document.tableCode}:${action}`) {
    case 'MREQ:analyze':
      return gate(action, !['analyzed', 'approved', 'converted'].includes(status), 'erp.business.statusReason')
    case 'MREQ:approve':
      return gate(action, ['analyzed', 'open', 'draft', ''].includes(status), 'erp.business.statusReason')
    case 'MREQ:convert-to-project':
      return gate(action, status === 'approved', 'erp.business.statusReason')
    case 'MPRJ:refresh-cost':
      return gate(action, normalizedText(record.Active) !== 'n', 'erp.business.statusReason')
    case 'MPRJ:close-feedback':
      return gate(action, normalizedText(record.FeedbackStatus) !== 'closed', 'erp.business.statusReason')
    case 'MPOR:submit':
      return gate(action, !['s', 'c'].includes(status) && approvalStatus !== 'a', 'erp.business.statusReason')
    case 'MPOR:approve':
      return gate(action, status === 's' && approvalStatus !== 'a', 'erp.business.statusReason')
    case 'MPDN:post':
    case 'MDLN:post':
      return gate(action, approvalStatus === 'a' && !posted, 'erp.business.statusReason')
    case 'MINV:post':
    case 'MIGN:post':
    case 'MIGE:post':
      return gate(action, !posted, 'erp.business.statusReason')
    case 'MRDR:confirm':
      return gate(action, normalizedText(record.Confirmed) !== 'y', 'erp.business.statusReason')
    case 'MRDR:approve':
      return gate(action, normalizedText(record.Confirmed) === 'y' && approvalStatus !== 'a', 'erp.business.statusReason')
    case 'MJDT:post':
      return gate(action, normalizedText(record.BtfStatus) !== 'p', 'erp.business.statusReason')
    default:
      return { action, available: true, reasonKey: 'ready' }
  }
}

function gate(action: string, available: boolean, reasonKey: string): ERPActionAvailability {
  return { action, available, reasonKey }
}

function buildBusinessTimeline(
  record: ERPBusinessRecord | undefined,
  childRows: ERPBusinessRecord[],
  actionResult: ERPActionResult<ERPBusinessRecord> | null,
  assistantProposals: Array<Record<string, unknown>>,
  actionExecutions: ERPActionExecution[],
): ERPTimelineEvent[] {
  if (!record) return []
  const events: ERPTimelineEvent[] = [
    { id: 'selected', titleKey: 'erp.business.documentDetail', detail: recordTitle(record) },
  ]
  if (childRows.length > 0) {
    events.push({ id: 'childRows', titleKey: 'erp.business.childRows', detail: String(childRows.length) })
  }
  if (actionResult) {
    events.push({
      id: `action-${actionResult.action}-${actionResult.status}`,
      titleKey: 'erp.business.actionCompleted',
      detail: `${actionResult.table_code}:${actionResult.key} / ${actionResult.action} / ${actionResult.status}`,
    })
  }
  const currentExecutionID = textValue(actionResult?.execution_id)
  for (const execution of actionExecutions.filter((item) => item.id !== currentExecutionID)) {
    events.push({
      id: `execution-${execution.id}`,
      titleKey: 'erp.business.actionCompleted',
      detail: `${execution.table_code}:${execution.record_key} / ${execution.action} / ${execution.status}${execution.failure_message ? ` / ${execution.failure_message}` : ''}`,
    })
    for (const generated of execution.generated_records ?? []) {
      events.push({
        id: `execution-${execution.id}-generated-${generated.line_num}`,
        titleKey: 'erp.business.generatedRecords',
        detail: `${generated.generated_table_code}:${generated.generated_key}`,
      })
    }
  }
  for (const [index, proposal] of assistantProposals.entries()) {
    events.push({
      id: `proposal-${displayValue(proposal.proposal_id)}-${index}`,
      titleKey: 'erp.business.assistantProposals',
      detail: displayValue(proposal.title || proposal.summary || proposal.action || proposal.proposal_type),
    })
  }
  return events
}

function generatedRecordsFromExecutions(actionExecutions: ERPActionExecution[]): NonNullable<ERPActionResult['generated_records']> {
  return actionExecutions.flatMap((execution) =>
    (execution.generated_records ?? []).map((record) => ({
      table_code: record.generated_table_code,
      key: record.generated_key,
      data: record.payload ?? {},
    })),
  )
}

function recordArray(value: unknown): Array<Record<string, unknown>> {
  return Array.isArray(value) ? value.filter((item): item is Record<string, unknown> => !!item && typeof item === 'object' && !Array.isArray(item)) : []
}

function isClosedOrPosted(record: ERPBusinessRecord) {
  return normalizedStatus(record) === 'c' || normalizedText(record.Posted) === 'y' || normalizedText(record.BtfStatus) === 'p'
}

function normalizedStatus(record: ERPBusinessRecord) {
  return normalizedText(record.Status || record.DocStatus || record.WddStatus || record.BtfStatus || record.Active)
}

function normalizedText(value: unknown) {
  return String(value ?? '').trim().toLowerCase()
}

function textValue(value: unknown) {
  return String(value ?? '').trim()
}

function numberValue(value: unknown) {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : undefined
}

function recordMap(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' && !Array.isArray(value) ? (value as Record<string, unknown>) : {}
}

function recordTitle(record: ERPBusinessRecord) {
  return String(record.Name || record.name || record.account_name || record.account_code || record.CardCode || record.ItemCode || record.WhsCode || record.PrjCode || record.DocEntry || record.TransId || record.key)
}

function recordStatus(record: ERPBusinessRecord) {
  return String(record.Status || record.DocStatus || record.WddStatus || record.BtfStatus || record.Active || '')
}

function displayValue(value: unknown) {
  if (value === undefined || value === null || value === '') return ''
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}
