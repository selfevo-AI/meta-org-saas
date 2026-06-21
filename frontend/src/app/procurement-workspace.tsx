'use client'

import { CheckCircle2, ClipboardList, FileCheck2, PackageCheck, RotateCcw, Send } from 'lucide-react'
import { FormEvent, useCallback, useEffect, useMemo, useState } from 'react'
import {
  approvePurchaseOrder,
  approvePurchaseRequisition,
  createPurchaseOrder,
  createPurchaseReceipt,
  createPurchaseRequisition,
  createPurchaseReturn,
  listBusinessPartners,
  listInventoryItems,
  listPurchaseOrders,
  listPurchaseReceipts,
  listPurchaseRequisitions,
  listPurchaseReturns,
  listWarehouses,
  postPurchaseReceipt,
  submitPurchaseOrder,
  submitPurchaseRequisition,
  type BusinessPartner,
  type InventoryItem,
  type PurchaseOrder,
  type PurchaseReceipt,
  type PurchaseRequisition,
  type PurchaseReturn,
  type Warehouse,
} from '@/lib/api'
import { useI18n } from '@/lib/i18n'
import {
  ActionButton,
  DataTable,
  Panel,
  RefreshButton,
  SelectInput,
  StatusPill,
  SubmitButton,
  SupplyChainDocumentDetail,
  TextInput,
  documentKey,
  money,
  quantity,
  type SupplyChainSelection,
} from './supply-chain-ui'

interface ProcurementWorkspaceProps {
  token: string
  currentSupplyChainFunctionID?: string
  externalSelection?: SupplyChainSelection | null
}

type TabID = 'requisitions' | 'orders' | 'receipts' | 'returns'

const tabs: Array<{ id: TabID; label: string; icon: typeof ClipboardList }> = [
  { id: 'requisitions', label: 'procurement.requisitions', icon: ClipboardList },
  { id: 'orders', label: 'procurement.orders', icon: FileCheck2 },
  { id: 'receipts', label: 'procurement.receipts', icon: PackageCheck },
  { id: 'returns', label: 'procurement.returns', icon: RotateCcw },
]

export function ProcurementWorkspace({ token, currentSupplyChainFunctionID, externalSelection }: ProcurementWorkspaceProps) {
  const { t } = useI18n()
  const [activeTab, setActiveTab] = useState<TabID>('requisitions')
  const [partners, setPartners] = useState<BusinessPartner[]>([])
  const [items, setItems] = useState<InventoryItem[]>([])
  const [warehouses, setWarehouses] = useState<Warehouse[]>([])
  const [requisitions, setRequisitions] = useState<PurchaseRequisition[]>([])
  const [orders, setOrders] = useState<PurchaseOrder[]>([])
  const [receipts, setReceipts] = useState<PurchaseReceipt[]>([])
  const [returns, setReturns] = useState<PurchaseReturn[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [requisitionForm, setRequisitionForm] = useState({
    title: '',
    supplier_id: '',
    supplier_name: '',
    item_id: '',
    warehouse_id: '',
    quantity: '1',
    unit_cost: '0',
    currency: 'CNY',
  })
  const [orderForm, setOrderForm] = useState({
    order_number: '',
    requisition_id: '',
    supplier_id: '',
    supplier_name: '',
    item_id: '',
    warehouse_id: '',
    quantity: '1',
    unit_cost: '0',
    tax_rate: '0.13',
    currency: 'CNY',
  })
  const [receiptForm, setReceiptForm] = useState({
    receipt_number: '',
    order_id: '',
    supplier_id: '',
    supplier_name: '',
    item_id: '',
    warehouse_id: '',
    quantity: '1',
    unit_cost: '0',
    tax_rate: '0.13',
    currency: 'CNY',
  })
  const [returnForm, setReturnForm] = useState({
    return_number: '',
    receipt_id: '',
    supplier_id: '',
    supplier_name: '',
    item_id: '',
    warehouse_id: '',
    quantity: '1',
    unit_cost: '0',
    tax_rate: '0.13',
    currency: 'CNY',
  })

  const suppliers = useMemo(() => partners.filter((partner) => partner.partner_type === 'supplier' || partner.partner_type === 'both'), [partners])
  const supplierLabels = useMemo(() => Object.fromEntries(suppliers.map((partner) => [partner.id, `${partner.partner_code || partner.master_key || partner.id.slice(0, 8)} · ${partner.name}`])), [suppliers])
  const itemLabels = useMemo(() => Object.fromEntries(items.map((item) => [item.id, `${item.item_code || item.master_key || item.id.slice(0, 8)} · ${item.name}`])), [items])
  const warehouseLabels = useMemo(() => Object.fromEntries(warehouses.map((warehouse) => [warehouse.id, `${warehouse.warehouse_code || warehouse.master_key || warehouse.id.slice(0, 8)} · ${warehouse.name}`])), [warehouses])
  const requisitionLabels = useMemo(() => Object.fromEntries(requisitions.map((item) => [item.id, item.title || documentKey(item)])), [requisitions])
  const orderLabels = useMemo(() => Object.fromEntries(orders.map((item) => [item.id, item.order_number || documentKey(item)])), [orders])
  const receiptLabels = useMemo(() => Object.fromEntries(receipts.map((item) => [item.id, item.receipt_number || documentKey(item)])), [receipts])
  const selectedDocument = useMemo(
    () => selectedProcurementDocument(externalSelection, { requisitions, orders, receipts, returns }),
    [externalSelection, orders, receipts, requisitions, returns],
  )
  const detailTitle = selectedDocument ? procurementDocumentTitle(selectedDocument) : ''
  const detailActions = selectedDocument ? procurementDetailActions(selectedDocument, token, run) : null
  const lineColumns = procurementLineColumns(selectedDocument?.targetType)
  const lineRows = procurementLineRows(selectedDocument?.document, selectedDocument?.targetType, itemLabels, warehouseLabels)

  const loadProcurement = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const [partnerData, itemData, warehouseData, requisitionData, orderData, receiptData, returnData] = await Promise.all([
        listBusinessPartners(token),
        listInventoryItems(token),
        listWarehouses(token),
        listPurchaseRequisitions(token),
        listPurchaseOrders(token),
        listPurchaseReceipts(token),
        listPurchaseReturns(token),
      ])
      const supplierData = partnerData.filter((partner) => partner.partner_type === 'supplier' || partner.partner_type === 'both')
      const firstSupplier = supplierData[0]
      setPartners(partnerData)
      setItems(itemData)
      setWarehouses(warehouseData)
      setRequisitions(requisitionData)
      setOrders(orderData)
      setReceipts(receiptData)
      setReturns(returnData)
      setRequisitionForm((current) => ({
        ...current,
        supplier_id: current.supplier_id || firstSupplier?.partner_code || '',
        supplier_name: current.supplier_name || firstSupplier?.name || '',
        item_id: current.item_id || itemData[0]?.id || '',
        warehouse_id: current.warehouse_id || warehouseData[0]?.id || '',
      }))
      setOrderForm((current) => ({
        ...current,
        supplier_id: current.supplier_id || firstSupplier?.partner_code || '',
        supplier_name: current.supplier_name || firstSupplier?.name || '',
        item_id: current.item_id || itemData[0]?.id || '',
        warehouse_id: current.warehouse_id || warehouseData[0]?.id || '',
      }))
      setReceiptForm((current) => ({
        ...current,
        supplier_id: current.supplier_id || firstSupplier?.partner_code || '',
        supplier_name: current.supplier_name || firstSupplier?.name || '',
        item_id: current.item_id || itemData[0]?.id || '',
        warehouse_id: current.warehouse_id || warehouseData[0]?.id || '',
        order_id: current.order_id || orderData[0]?.id || '',
      }))
      setReturnForm((current) => ({
        ...current,
        supplier_id: current.supplier_id || firstSupplier?.partner_code || '',
        supplier_name: current.supplier_name || firstSupplier?.name || '',
        item_id: current.item_id || itemData[0]?.id || '',
        warehouse_id: current.warehouse_id || warehouseData[0]?.id || '',
        receipt_id: current.receipt_id || receiptData[0]?.id || '',
      }))
    } catch (err) {
      setError(err instanceof Error ? err.message : t('procurement.loadFailed'))
    } finally {
      setLoading(false)
    }
  }, [t, token])

  useEffect(() => {
    const timer = window.setTimeout(() => {
      void loadProcurement()
    }, 0)
    return () => window.clearTimeout(timer)
  }, [loadProcurement])

  useEffect(() => {
    const nextTab = procurementTabForSelection(externalSelection?.targetType) ?? procurementTabForFunction(currentSupplyChainFunctionID)
    if (!nextTab) return
    const timer = window.setTimeout(() => setActiveTab(nextTab), 0)
    return () => window.clearTimeout(timer)
  }, [currentSupplyChainFunctionID, externalSelection?.targetType])

  async function run(action: () => Promise<void>, success: string) {
    setLoading(true)
    setError('')
    setNotice('')
    try {
      await action()
      setNotice(t(success))
      await loadProcurement()
    } catch (err) {
      setError(err instanceof Error ? err.message : t('common.operationFailed'))
    } finally {
      setLoading(false)
    }
  }

  function applySupplier<T extends { supplier_id: string; supplier_name: string }>(form: T, value: string): T {
    const selected = suppliers.find((supplier) => supplier.id === value)
    return {
      ...form,
      supplier_id: selected?.partner_code || value,
      supplier_name: selected?.name || form.supplier_name,
    }
  }

  async function submitRequisition(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await run(
      () =>
        createPurchaseRequisition(token, {
          title: requisitionForm.title,
          supplier_id: requisitionForm.supplier_id,
          supplier_name: requisitionForm.supplier_name,
          currency: requisitionForm.currency,
          lines: [
            {
              item_id: requisitionForm.item_id,
              warehouse_id: requisitionForm.warehouse_id,
              quantity: Number(requisitionForm.quantity || 0),
              unit_cost: Number(requisitionForm.unit_cost || 0),
            },
          ],
          metadata: {},
        }).then(() => setRequisitionForm((current) => ({ ...current, title: '', quantity: '1', unit_cost: '0' }))),
      'procurement.requisitionSaved',
    )
  }

  async function submitOrder(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await run(
      () =>
        createPurchaseOrder(token, {
          order_number: orderForm.order_number,
          requisition_id: orderForm.requisition_id || undefined,
          supplier_id: orderForm.supplier_id,
          supplier_name: orderForm.supplier_name,
          currency: orderForm.currency,
          lines: [
            {
              item_id: orderForm.item_id,
              warehouse_id: orderForm.warehouse_id,
              quantity: Number(orderForm.quantity || 0),
              unit_cost: Number(orderForm.unit_cost || 0),
              tax_rate: Number(orderForm.tax_rate || 0),
            },
          ],
          metadata: {},
        }).then(() => setOrderForm((current) => ({ ...current, order_number: '', quantity: '1', unit_cost: '0' }))),
      'procurement.orderSaved',
    )
  }

  async function submitReceipt(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await run(
      () =>
        createPurchaseReceipt(token, {
          receipt_number: receiptForm.receipt_number,
          order_id: receiptForm.order_id || undefined,
          supplier_id: receiptForm.supplier_id,
          supplier_name: receiptForm.supplier_name,
          warehouse_id: receiptForm.warehouse_id,
          currency: receiptForm.currency,
          lines: [
            {
              item_id: receiptForm.item_id,
              warehouse_id: receiptForm.warehouse_id,
              quantity: Number(receiptForm.quantity || 0),
              unit_cost: Number(receiptForm.unit_cost || 0),
              tax_rate: Number(receiptForm.tax_rate || 0),
            },
          ],
          metadata: {},
        }).then(() => setReceiptForm((current) => ({ ...current, receipt_number: '', quantity: '1', unit_cost: '0' }))),
      'procurement.receiptSaved',
    )
  }

  async function submitReturn(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await run(
      () =>
        createPurchaseReturn(token, {
          return_number: returnForm.return_number,
          receipt_id: returnForm.receipt_id || undefined,
          supplier_id: returnForm.supplier_id,
          supplier_name: returnForm.supplier_name,
          currency: returnForm.currency,
          lines: [
            {
              item_id: returnForm.item_id,
              warehouse_id: returnForm.warehouse_id,
              quantity: Number(returnForm.quantity || 0),
              unit_cost: Number(returnForm.unit_cost || 0),
              tax_rate: Number(returnForm.tax_rate || 0),
            },
          ],
          metadata: {},
        }).then(() => setReturnForm((current) => ({ ...current, return_number: '', quantity: '1', unit_cost: '0' }))),
      'procurement.returnSaved',
    )
  }

  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="inline-flex rounded-lg border border-slate-200 bg-white p-1 shadow-sm">
          {tabs.map((tab) => {
            const Icon = tab.icon
            return (
              <button
                key={tab.id}
                type="button"
                onClick={() => setActiveTab(tab.id)}
                className={`inline-flex h-9 items-center gap-2 rounded-md px-3 text-sm font-semibold ${activeTab === tab.id ? 'bg-slate-950 text-white' : 'text-slate-600 hover:bg-slate-100'}`}
              >
                <Icon className="h-4 w-4" />
                {t(tab.label)}
              </button>
            )
          })}
        </div>
        <RefreshButton loading={loading} onClick={() => void loadProcurement()} />
      </div>

      {(notice || error) && (
        <p className={`rounded-md px-3 py-2 text-sm ${error ? 'bg-red-50 text-red-700' : 'bg-emerald-50 text-emerald-700'}`}>
          {error || notice}
        </p>
      )}

      {selectedDocument && (
        <SupplyChainDocumentDetail
          title={detailTitle}
          subtitle={t('supplyChain.selectedDocument')}
          mainFields={procurementMainFields(selectedDocument.document, selectedDocument.targetType)}
          lineColumns={lineColumns}
          lineRows={lineRows}
          actions={detailActions}
        />
      )}

      {!selectedDocument && (
        <>

      {activeTab === 'requisitions' && (
        <div className="grid gap-5 xl:grid-cols-[1fr_360px]">
          <Panel title="procurement.requisitions">
            <DataTable
              headers={['procurement.document', 'finance.vendor', 'finance.amount', 'developer.status', 'procurement.approvalStatus']}
              rows={requisitions.map((item) => [item.title || documentKey(item), item.supplier_name, money(item.total_amount, item.currency), <StatusPill key={item.id} label={item.status} />, <StatusPill key={`${item.id}-approval`} label={item.approval_status} />])}
              actions={requisitions.map((item) => (
                <div key={item.id} className="flex flex-wrap gap-2">
                  <ActionButton label="procurement.submit" onClick={() => void run(() => submitPurchaseRequisition(token, item.id).then(() => undefined), 'procurement.requisitionSubmitted')} disabled={item.status !== 'draft'} icon={<Send className="h-3.5 w-3.5" />} />
                  <ActionButton label="procurement.approve" onClick={() => void run(() => approvePurchaseRequisition(token, item.id).then(() => undefined), 'procurement.requisitionApproved')} disabled={item.status === 'approved'} tone="green" icon={<CheckCircle2 className="h-3.5 w-3.5" />} />
                </div>
              ))}
            />
          </Panel>
          <Panel title="procurement.createRequisition">
            <form className="space-y-3" onSubmit={submitRequisition}>
              <TextInput label="finance.title" value={requisitionForm.title} onChange={(value) => setRequisitionForm({ ...requisitionForm, title: value })} required />
              <SelectInput label="finance.vendor" value={suppliers.find((supplier) => supplier.partner_code === requisitionForm.supplier_id)?.id || ''} onChange={(value) => setRequisitionForm(applySupplier(requisitionForm, value))} options={suppliers.map((supplier) => supplier.id)} labels={supplierLabels} />
              <TextInput label="finance.vendor" value={requisitionForm.supplier_name} onChange={(value) => setRequisitionForm({ ...requisitionForm, supplier_name: value })} />
              <SelectInput label="inventory.item" value={requisitionForm.item_id} onChange={(value) => setRequisitionForm({ ...requisitionForm, item_id: value })} options={items.map((item) => item.id)} labels={itemLabels} required />
              <SelectInput label="inventory.warehouse" value={requisitionForm.warehouse_id} onChange={(value) => setRequisitionForm({ ...requisitionForm, warehouse_id: value })} options={warehouses.map((warehouse) => warehouse.id)} labels={warehouseLabels} required />
              <div className="grid gap-3 sm:grid-cols-2">
                <TextInput label="inventory.quantity" value={requisitionForm.quantity} onChange={(value) => setRequisitionForm({ ...requisitionForm, quantity: value })} />
                <TextInput label="inventory.unitCost" value={requisitionForm.unit_cost} onChange={(value) => setRequisitionForm({ ...requisitionForm, unit_cost: value })} />
              </div>
              <SubmitButton loading={loading || !requisitionForm.item_id || !requisitionForm.warehouse_id} label="procurement.saveRequisition" />
            </form>
          </Panel>
        </div>
      )}

      {activeTab === 'orders' && (
        <div className="grid gap-5 xl:grid-cols-[1fr_360px]">
          <Panel title="procurement.orders">
            <DataTable
              headers={['procurement.document', 'finance.vendor', 'finance.amount', 'developer.status', 'procurement.approvalStatus']}
              rows={orders.map((item) => [item.order_number || documentKey(item), item.supplier_name, money(item.total_amount, item.currency), <StatusPill key={item.id} label={item.status} />, <StatusPill key={`${item.id}-approval`} label={item.approval_status} />])}
              actions={orders.map((item) => (
                <div key={item.id} className="flex flex-wrap gap-2">
                  <ActionButton label="procurement.submit" onClick={() => void run(() => submitPurchaseOrder(token, item.id).then(() => undefined), 'procurement.orderSubmitted')} disabled={item.status !== 'draft'} icon={<Send className="h-3.5 w-3.5" />} />
                  <ActionButton label="procurement.approve" onClick={() => void run(() => approvePurchaseOrder(token, item.id).then(() => undefined), 'procurement.orderApproved')} disabled={item.status === 'approved'} tone="green" icon={<CheckCircle2 className="h-3.5 w-3.5" />} />
                </div>
              ))}
            />
          </Panel>
          <Panel title="procurement.createOrder">
            <form className="space-y-3" onSubmit={submitOrder}>
              <TextInput label="procurement.documentNumber" value={orderForm.order_number} onChange={(value) => setOrderForm({ ...orderForm, order_number: value })} />
              <SelectInput label="procurement.requisition" value={orderForm.requisition_id} onChange={(value) => setOrderForm({ ...orderForm, requisition_id: value })} options={requisitions.map((item) => item.id)} labels={requisitionLabels} />
              <SelectInput label="finance.vendor" value={suppliers.find((supplier) => supplier.partner_code === orderForm.supplier_id)?.id || ''} onChange={(value) => setOrderForm(applySupplier(orderForm, value))} options={suppliers.map((supplier) => supplier.id)} labels={supplierLabels} />
              <TextInput label="finance.vendor" value={orderForm.supplier_name} onChange={(value) => setOrderForm({ ...orderForm, supplier_name: value })} />
              <SelectInput label="inventory.item" value={orderForm.item_id} onChange={(value) => setOrderForm({ ...orderForm, item_id: value })} options={items.map((item) => item.id)} labels={itemLabels} required />
              <SelectInput label="inventory.warehouse" value={orderForm.warehouse_id} onChange={(value) => setOrderForm({ ...orderForm, warehouse_id: value })} options={warehouses.map((warehouse) => warehouse.id)} labels={warehouseLabels} required />
              <div className="grid gap-3 sm:grid-cols-3">
                <TextInput label="inventory.quantity" value={orderForm.quantity} onChange={(value) => setOrderForm({ ...orderForm, quantity: value })} />
                <TextInput label="inventory.unitCost" value={orderForm.unit_cost} onChange={(value) => setOrderForm({ ...orderForm, unit_cost: value })} />
                <TextInput label="finance.taxAmount" value={orderForm.tax_rate} onChange={(value) => setOrderForm({ ...orderForm, tax_rate: value })} />
              </div>
              <SubmitButton loading={loading || !orderForm.item_id || !orderForm.warehouse_id} label="procurement.saveOrder" />
            </form>
          </Panel>
        </div>
      )}

      {activeTab === 'receipts' && (
        <div className="grid gap-5 xl:grid-cols-[1fr_360px]">
          <Panel title="procurement.receipts">
            <DataTable
              headers={['procurement.document', 'finance.vendor', 'finance.amount', 'developer.status', 'finance.payable']}
              rows={receipts.map((item) => [item.receipt_number || documentKey(item), item.supplier_name, money(item.total_amount, item.currency), <StatusPill key={item.id} label={item.status} />, item.payable_id || t('common.none')])}
              actions={receipts.map((item) => (
                <ActionButton key={item.id} label="procurement.postReceipt" onClick={() => void run(() => postPurchaseReceipt(token, item.id).then(() => undefined), 'procurement.receiptPosted')} disabled={item.status === 'posted'} tone="primary" icon={<PackageCheck className="h-3.5 w-3.5" />} />
              ))}
            />
          </Panel>
          <Panel title="procurement.createReceipt">
            <form className="space-y-3" onSubmit={submitReceipt}>
              <TextInput label="procurement.documentNumber" value={receiptForm.receipt_number} onChange={(value) => setReceiptForm({ ...receiptForm, receipt_number: value })} />
              <SelectInput label="procurement.order" value={receiptForm.order_id} onChange={(value) => setReceiptForm({ ...receiptForm, order_id: value })} options={orders.map((item) => item.id)} labels={orderLabels} />
              <TextInput label="finance.vendor" value={receiptForm.supplier_name} onChange={(value) => setReceiptForm({ ...receiptForm, supplier_name: value })} />
              <SelectInput label="inventory.item" value={receiptForm.item_id} onChange={(value) => setReceiptForm({ ...receiptForm, item_id: value })} options={items.map((item) => item.id)} labels={itemLabels} required />
              <SelectInput label="inventory.warehouse" value={receiptForm.warehouse_id} onChange={(value) => setReceiptForm({ ...receiptForm, warehouse_id: value })} options={warehouses.map((warehouse) => warehouse.id)} labels={warehouseLabels} required />
              <div className="grid gap-3 sm:grid-cols-3">
                <TextInput label="inventory.quantity" value={receiptForm.quantity} onChange={(value) => setReceiptForm({ ...receiptForm, quantity: value })} />
                <TextInput label="inventory.unitCost" value={receiptForm.unit_cost} onChange={(value) => setReceiptForm({ ...receiptForm, unit_cost: value })} />
                <TextInput label="finance.taxAmount" value={receiptForm.tax_rate} onChange={(value) => setReceiptForm({ ...receiptForm, tax_rate: value })} />
              </div>
              <SubmitButton loading={loading || !receiptForm.item_id || !receiptForm.warehouse_id} label="procurement.saveReceipt" />
            </form>
          </Panel>
        </div>
      )}

      {activeTab === 'returns' && (
        <div className="grid gap-5 xl:grid-cols-[1fr_360px]">
          <Panel title="procurement.returns">
            <DataTable
              headers={['procurement.document', 'finance.vendor', 'finance.amount', 'developer.status']}
              rows={returns.map((item) => [item.return_number || documentKey(item), item.supplier_name, money(item.total_amount, item.currency), <StatusPill key={item.id} label={item.status} />])}
            />
          </Panel>
          <Panel title="procurement.createReturn">
            <form className="space-y-3" onSubmit={submitReturn}>
              <TextInput label="procurement.documentNumber" value={returnForm.return_number} onChange={(value) => setReturnForm({ ...returnForm, return_number: value })} />
              <SelectInput label="procurement.receipt" value={returnForm.receipt_id} onChange={(value) => setReturnForm({ ...returnForm, receipt_id: value })} options={receipts.map((item) => item.id)} labels={receiptLabels} />
              <TextInput label="finance.vendor" value={returnForm.supplier_name} onChange={(value) => setReturnForm({ ...returnForm, supplier_name: value })} />
              <SelectInput label="inventory.item" value={returnForm.item_id} onChange={(value) => setReturnForm({ ...returnForm, item_id: value })} options={items.map((item) => item.id)} labels={itemLabels} required />
              <SelectInput label="inventory.warehouse" value={returnForm.warehouse_id} onChange={(value) => setReturnForm({ ...returnForm, warehouse_id: value })} options={warehouses.map((warehouse) => warehouse.id)} labels={warehouseLabels} required />
              <div className="grid gap-3 sm:grid-cols-3">
                <TextInput label="inventory.quantity" value={returnForm.quantity} onChange={(value) => setReturnForm({ ...returnForm, quantity: value })} />
                <TextInput label="inventory.unitCost" value={returnForm.unit_cost} onChange={(value) => setReturnForm({ ...returnForm, unit_cost: value })} />
                <TextInput label="finance.taxAmount" value={returnForm.tax_rate} onChange={(value) => setReturnForm({ ...returnForm, tax_rate: value })} />
              </div>
              <SubmitButton loading={loading || !returnForm.item_id || !returnForm.warehouse_id} label="procurement.saveReturn" />
            </form>
          </Panel>
        </div>
      )}
        </>
      )}
    </div>
  )
}

type ProcurementDocumentSelection = {
  targetType: string
  document: Record<string, any>
}

function procurementTabForSelection(targetType?: string): TabID | null {
  const tabsByType: Record<string, TabID> = {
    purchase_requisition: 'requisitions',
    purchase_order: 'orders',
    purchase_receipt: 'receipts',
    purchase_return: 'returns',
  }
  return targetType ? tabsByType[targetType] ?? null : null
}

function procurementTabForFunction(functionID?: string): TabID | null {
  const tabsByFunction: Record<string, TabID> = {
    'procurement:requisitions': 'requisitions',
    'procurement:orders': 'orders',
    'procurement:receipts': 'receipts',
    'procurement:returns': 'returns',
  }
  return functionID ? tabsByFunction[functionID] ?? null : null
}

function selectedProcurementDocument(
  selection: SupplyChainSelection | null | undefined,
  data: {
    requisitions: PurchaseRequisition[]
    orders: PurchaseOrder[]
    receipts: PurchaseReceipt[]
    returns: PurchaseReturn[]
  },
): ProcurementDocumentSelection | null {
  if (!selection) return null
  const byType: Record<string, Array<Record<string, any>>> = {
    purchase_requisition: data.requisitions,
    purchase_order: data.orders,
    purchase_receipt: data.receipts,
    purchase_return: data.returns,
  }
  const document = byType[selection.targetType]?.find((item) => item.id === selection.targetID) ?? selection.record
  return document ? { targetType: selection.targetType, document: document as Record<string, any> } : null
}

function procurementDocumentTitle(selection: ProcurementDocumentSelection): string {
  const document = selection.document
  return String(document.title || document.order_number || document.receipt_number || document.return_number || document.master_key || document.id || '')
}

function procurementMainFields(document: Record<string, any>, targetType: string): Array<{ label: string; value: any }> {
  return [
    { label: targetType === 'purchase_requisition' ? 'finance.title' : 'procurement.documentNumber', value: document.title || document.order_number || document.receipt_number || document.return_number || document.master_key },
    { label: 'finance.vendor', value: document.supplier_name || document.supplier_id },
    { label: 'developer.status', value: <StatusPill label={document.status || ''} /> },
    { label: 'procurement.approvalStatus', value: <StatusPill label={document.approval_status || 'not_required'} /> },
    { label: 'finance.amount', value: money(document.total_amount, document.currency) },
    { label: 'finance.currency', value: document.currency },
    { label: 'businessStatus.updated', value: document.updated_at || document.created_at },
    { label: 'businessStatus.code', value: document.id },
  ]
}

function procurementLineColumns(targetType?: string): string[] {
  if (targetType === 'purchase_requisition') {
    return ['inventory.item', 'inventory.warehouse', 'inventory.quantity', 'inventory.unitCost', 'finance.amount']
  }
  return ['inventory.item', 'inventory.warehouse', 'inventory.quantity', 'inventory.unitCost', 'finance.taxAmount', 'finance.amount']
}

function procurementLineRows(
  document: Record<string, any> | undefined,
  targetType: string | undefined,
  itemLabels: Record<string, string>,
  warehouseLabels: Record<string, string>,
): any[][] {
  const lines = Array.isArray(document?.lines) ? document.lines : []
  return lines.map((line: Record<string, any>) => {
    if (targetType === 'purchase_requisition') {
      return [
        itemLabels[line.item_id] ?? line.item_id,
        warehouseLabels[line.warehouse_id] ?? line.warehouse_id,
        quantity(line.quantity),
        money(line.unit_cost, document?.currency),
        money(line.amount, document?.currency),
      ]
    }
    return [
      itemLabels[line.item_id] ?? line.item_id,
      warehouseLabels[line.warehouse_id] ?? line.warehouse_id,
      quantity(line.quantity),
      money(line.unit_cost, document?.currency),
      money(line.tax_amount, document?.currency),
      money(line.total_amount ?? line.amount, document?.currency),
    ]
  })
}

function procurementDetailActions(
  selection: ProcurementDocumentSelection,
  token: string,
  run: (action: () => Promise<void>, success: string) => Promise<void>,
) {
  const document = selection.document
  if (!document.id) return null
  if (selection.targetType === 'purchase_requisition') {
    return (
      <div className="flex flex-wrap gap-2">
        <ActionButton label="procurement.submit" onClick={() => void run(() => submitPurchaseRequisition(token, document.id).then(() => undefined), 'procurement.requisitionSubmitted')} disabled={document.status !== 'draft'} icon={<Send className="h-3.5 w-3.5" />} />
        <ActionButton label="procurement.approve" onClick={() => void run(() => approvePurchaseRequisition(token, document.id).then(() => undefined), 'procurement.requisitionApproved')} disabled={document.status === 'approved'} tone="green" icon={<CheckCircle2 className="h-3.5 w-3.5" />} />
      </div>
    )
  }
  if (selection.targetType === 'purchase_order') {
    return (
      <div className="flex flex-wrap gap-2">
        <ActionButton label="procurement.submit" onClick={() => void run(() => submitPurchaseOrder(token, document.id).then(() => undefined), 'procurement.orderSubmitted')} disabled={document.status !== 'draft'} icon={<Send className="h-3.5 w-3.5" />} />
        <ActionButton label="procurement.approve" onClick={() => void run(() => approvePurchaseOrder(token, document.id).then(() => undefined), 'procurement.orderApproved')} disabled={document.status === 'approved'} tone="green" icon={<CheckCircle2 className="h-3.5 w-3.5" />} />
      </div>
    )
  }
  if (selection.targetType === 'purchase_receipt') {
    return <ActionButton label="procurement.postReceipt" onClick={() => void run(() => postPurchaseReceipt(token, document.id).then(() => undefined), 'procurement.receiptPosted')} disabled={document.status === 'posted'} tone="primary" icon={<PackageCheck className="h-3.5 w-3.5" />} />
  }
  return null
}
