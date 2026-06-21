'use client'

import { CheckCircle2, FileText, PackageMinus, RotateCcw, Send, ShoppingCart } from 'lucide-react'
import { FormEvent, useCallback, useEffect, useMemo, useState } from 'react'
import {
  approveSalesOrder,
  confirmSalesOrder,
  createSalesOrder,
  createSalesQuotation,
  createSalesReturn,
  createSalesShipment,
  listBusinessPartners,
  listInventoryItems,
  listSalesOrders,
  listSalesQuotations,
  listSalesReturns,
  listSalesShipments,
  listWarehouses,
  postSalesShipment,
  type BusinessPartner,
  type InventoryItem,
  type SalesOrder,
  type SalesQuotation,
  type SalesReturn,
  type SalesShipment,
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

interface SalesWorkspaceProps {
  token: string
  currentSupplyChainFunctionID?: string
  externalSelection?: SupplyChainSelection | null
}

type TabID = 'quotations' | 'orders' | 'shipments' | 'returns'

const tabs: Array<{ id: TabID; label: string; icon: typeof FileText }> = [
  { id: 'quotations', label: 'sales.quotations', icon: FileText },
  { id: 'orders', label: 'sales.orders', icon: ShoppingCart },
  { id: 'shipments', label: 'sales.shipments', icon: PackageMinus },
  { id: 'returns', label: 'sales.returns', icon: RotateCcw },
]

export function SalesWorkspace({ token, currentSupplyChainFunctionID, externalSelection }: SalesWorkspaceProps) {
  const { t } = useI18n()
  const [activeTab, setActiveTab] = useState<TabID>('quotations')
  const [partners, setPartners] = useState<BusinessPartner[]>([])
  const [items, setItems] = useState<InventoryItem[]>([])
  const [warehouses, setWarehouses] = useState<Warehouse[]>([])
  const [quotations, setQuotations] = useState<SalesQuotation[]>([])
  const [orders, setOrders] = useState<SalesOrder[]>([])
  const [shipments, setShipments] = useState<SalesShipment[]>([])
  const [returns, setReturns] = useState<SalesReturn[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [quotationForm, setQuotationForm] = useState({
    quotation_number: '',
    customer_id: '',
    customer_name: '',
    item_id: '',
    warehouse_id: '',
    quantity: '1',
    unit_price: '0',
    tax_rate: '0.13',
    currency: 'CNY',
  })
  const [orderForm, setOrderForm] = useState({
    order_number: '',
    quotation_id: '',
    customer_id: '',
    customer_name: '',
    item_id: '',
    warehouse_id: '',
    quantity: '1',
    unit_price: '0',
    tax_rate: '0.13',
    currency: 'CNY',
  })
  const [shipmentForm, setShipmentForm] = useState({
    shipment_number: '',
    order_id: '',
    customer_id: '',
    customer_name: '',
    item_id: '',
    warehouse_id: '',
    quantity: '1',
    unit_price: '0',
    tax_rate: '0.13',
    currency: 'CNY',
  })
  const [returnForm, setReturnForm] = useState({
    return_number: '',
    shipment_id: '',
    customer_id: '',
    customer_name: '',
    item_id: '',
    warehouse_id: '',
    quantity: '1',
    unit_price: '0',
    tax_rate: '0.13',
    currency: 'CNY',
  })

  const customers = useMemo(() => partners.filter((partner) => partner.partner_type === 'customer' || partner.partner_type === 'both'), [partners])
  const customerLabels = useMemo(() => Object.fromEntries(customers.map((partner) => [partner.id, `${partner.partner_code || partner.master_key || partner.id.slice(0, 8)} · ${partner.name}`])), [customers])
  const itemLabels = useMemo(() => Object.fromEntries(items.map((item) => [item.id, `${item.item_code || item.master_key || item.id.slice(0, 8)} · ${item.name}`])), [items])
  const warehouseLabels = useMemo(() => Object.fromEntries(warehouses.map((warehouse) => [warehouse.id, `${warehouse.warehouse_code || warehouse.master_key || warehouse.id.slice(0, 8)} · ${warehouse.name}`])), [warehouses])
  const quotationLabels = useMemo(() => Object.fromEntries(quotations.map((item) => [item.id, item.quotation_number || documentKey(item)])), [quotations])
  const orderLabels = useMemo(() => Object.fromEntries(orders.map((item) => [item.id, item.order_number || documentKey(item)])), [orders])
  const shipmentLabels = useMemo(() => Object.fromEntries(shipments.map((item) => [item.id, item.shipment_number || documentKey(item)])), [shipments])
  const selectedDocument = useMemo(
    () => selectedSalesDocument(externalSelection, { quotations, orders, shipments, returns }),
    [externalSelection, orders, quotations, returns, shipments],
  )
  const detailTitle = selectedDocument ? salesDocumentTitle(selectedDocument) : ''
  const detailActions = selectedDocument ? salesDetailActions(selectedDocument, token, run) : null
  const lineColumns = salesLineColumns()
  const lineRows = salesLineRows(selectedDocument?.document, itemLabels, warehouseLabels)

  const loadSales = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const [partnerData, itemData, warehouseData, quotationData, orderData, shipmentData, returnData] = await Promise.all([
        listBusinessPartners(token),
        listInventoryItems(token),
        listWarehouses(token),
        listSalesQuotations(token),
        listSalesOrders(token),
        listSalesShipments(token),
        listSalesReturns(token),
      ])
      const customerData = partnerData.filter((partner) => partner.partner_type === 'customer' || partner.partner_type === 'both')
      const firstCustomer = customerData[0]
      setPartners(partnerData)
      setItems(itemData)
      setWarehouses(warehouseData)
      setQuotations(quotationData)
      setOrders(orderData)
      setShipments(shipmentData)
      setReturns(returnData)
      setQuotationForm((current) => ({
        ...current,
        customer_id: current.customer_id || firstCustomer?.partner_code || '',
        customer_name: current.customer_name || firstCustomer?.name || '',
        item_id: current.item_id || itemData[0]?.id || '',
        warehouse_id: current.warehouse_id || warehouseData[0]?.id || '',
      }))
      setOrderForm((current) => ({
        ...current,
        customer_id: current.customer_id || firstCustomer?.partner_code || '',
        customer_name: current.customer_name || firstCustomer?.name || '',
        item_id: current.item_id || itemData[0]?.id || '',
        warehouse_id: current.warehouse_id || warehouseData[0]?.id || '',
        quotation_id: current.quotation_id || quotationData[0]?.id || '',
      }))
      setShipmentForm((current) => ({
        ...current,
        customer_id: current.customer_id || firstCustomer?.partner_code || '',
        customer_name: current.customer_name || firstCustomer?.name || '',
        item_id: current.item_id || itemData[0]?.id || '',
        warehouse_id: current.warehouse_id || warehouseData[0]?.id || '',
        order_id: current.order_id || orderData[0]?.id || '',
      }))
      setReturnForm((current) => ({
        ...current,
        customer_id: current.customer_id || firstCustomer?.partner_code || '',
        customer_name: current.customer_name || firstCustomer?.name || '',
        item_id: current.item_id || itemData[0]?.id || '',
        warehouse_id: current.warehouse_id || warehouseData[0]?.id || '',
        shipment_id: current.shipment_id || shipmentData[0]?.id || '',
      }))
    } catch (err) {
      setError(err instanceof Error ? err.message : t('sales.loadFailed'))
    } finally {
      setLoading(false)
    }
  }, [t, token])

  useEffect(() => {
    const timer = window.setTimeout(() => {
      void loadSales()
    }, 0)
    return () => window.clearTimeout(timer)
  }, [loadSales])

  useEffect(() => {
    const nextTab = salesTabForSelection(externalSelection?.targetType) ?? salesTabForFunction(currentSupplyChainFunctionID)
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
      await loadSales()
    } catch (err) {
      setError(err instanceof Error ? err.message : t('common.operationFailed'))
    } finally {
      setLoading(false)
    }
  }

  function applyCustomer<T extends { customer_id: string; customer_name: string }>(form: T, value: string): T {
    const selected = customers.find((customer) => customer.id === value)
    return {
      ...form,
      customer_id: selected?.partner_code || value,
      customer_name: selected?.name || form.customer_name,
    }
  }

  async function submitQuotation(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await run(
      () =>
        createSalesQuotation(token, {
          quotation_number: quotationForm.quotation_number,
          customer_id: quotationForm.customer_id,
          customer_name: quotationForm.customer_name,
          currency: quotationForm.currency,
          lines: [
            {
              item_id: quotationForm.item_id,
              warehouse_id: quotationForm.warehouse_id,
              quantity: Number(quotationForm.quantity || 0),
              unit_price: Number(quotationForm.unit_price || 0),
              tax_rate: Number(quotationForm.tax_rate || 0),
            },
          ],
          metadata: {},
        }).then(() => setQuotationForm((current) => ({ ...current, quotation_number: '', quantity: '1', unit_price: '0' }))),
      'sales.quotationSaved',
    )
  }

  async function submitOrder(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await run(
      () =>
        createSalesOrder(token, {
          order_number: orderForm.order_number,
          quotation_id: orderForm.quotation_id || undefined,
          customer_id: orderForm.customer_id,
          customer_name: orderForm.customer_name,
          currency: orderForm.currency,
          lines: [
            {
              item_id: orderForm.item_id,
              warehouse_id: orderForm.warehouse_id,
              quantity: Number(orderForm.quantity || 0),
              unit_price: Number(orderForm.unit_price || 0),
              tax_rate: Number(orderForm.tax_rate || 0),
            },
          ],
          metadata: {},
        }).then(() => setOrderForm((current) => ({ ...current, order_number: '', quantity: '1', unit_price: '0' }))),
      'sales.orderSaved',
    )
  }

  async function submitShipment(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await run(
      () =>
        createSalesShipment(token, {
          shipment_number: shipmentForm.shipment_number,
          order_id: shipmentForm.order_id || undefined,
          customer_id: shipmentForm.customer_id,
          customer_name: shipmentForm.customer_name,
          currency: shipmentForm.currency,
          lines: [
            {
              item_id: shipmentForm.item_id,
              warehouse_id: shipmentForm.warehouse_id,
              quantity: Number(shipmentForm.quantity || 0),
              unit_price: Number(shipmentForm.unit_price || 0),
              tax_rate: Number(shipmentForm.tax_rate || 0),
            },
          ],
          metadata: {},
        }).then(() => setShipmentForm((current) => ({ ...current, shipment_number: '', quantity: '1', unit_price: '0' }))),
      'sales.shipmentSaved',
    )
  }

  async function submitReturn(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await run(
      () =>
        createSalesReturn(token, {
          return_number: returnForm.return_number,
          shipment_id: returnForm.shipment_id || undefined,
          customer_id: returnForm.customer_id,
          customer_name: returnForm.customer_name,
          currency: returnForm.currency,
          lines: [
            {
              item_id: returnForm.item_id,
              warehouse_id: returnForm.warehouse_id,
              quantity: Number(returnForm.quantity || 0),
              unit_price: Number(returnForm.unit_price || 0),
              tax_rate: Number(returnForm.tax_rate || 0),
            },
          ],
          metadata: {},
        }).then(() => setReturnForm((current) => ({ ...current, return_number: '', quantity: '1', unit_price: '0' }))),
      'sales.returnSaved',
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
        <RefreshButton loading={loading} onClick={() => void loadSales()} />
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
          mainFields={salesMainFields(selectedDocument.document, selectedDocument.targetType)}
          lineColumns={lineColumns}
          lineRows={lineRows}
          actions={detailActions}
        />
      )}

      {!selectedDocument && (
        <>

      {activeTab === 'quotations' && (
        <div className="grid gap-5 xl:grid-cols-[1fr_360px]">
          <Panel title="sales.quotations">
            <DataTable
              headers={['sales.document', 'finance.customer', 'finance.amount', 'developer.status']}
              rows={quotations.map((item) => [item.quotation_number || documentKey(item), item.customer_name, money(item.total_amount, item.currency), <StatusPill key={item.id} label={item.status} />])}
            />
          </Panel>
          <Panel title="sales.createQuotation">
            <form className="space-y-3" onSubmit={submitQuotation}>
              <TextInput label="sales.documentNumber" value={quotationForm.quotation_number} onChange={(value) => setQuotationForm({ ...quotationForm, quotation_number: value })} />
              <SelectInput label="finance.customer" value={customers.find((customer) => customer.partner_code === quotationForm.customer_id)?.id || ''} onChange={(value) => setQuotationForm(applyCustomer(quotationForm, value))} options={customers.map((customer) => customer.id)} labels={customerLabels} />
              <TextInput label="finance.customer" value={quotationForm.customer_name} onChange={(value) => setQuotationForm({ ...quotationForm, customer_name: value })} />
              <SelectInput label="inventory.item" value={quotationForm.item_id} onChange={(value) => setQuotationForm({ ...quotationForm, item_id: value })} options={items.map((item) => item.id)} labels={itemLabels} required />
              <SelectInput label="inventory.warehouse" value={quotationForm.warehouse_id} onChange={(value) => setQuotationForm({ ...quotationForm, warehouse_id: value })} options={warehouses.map((warehouse) => warehouse.id)} labels={warehouseLabels} required />
              <div className="grid gap-3 sm:grid-cols-3">
                <TextInput label="inventory.quantity" value={quotationForm.quantity} onChange={(value) => setQuotationForm({ ...quotationForm, quantity: value })} />
                <TextInput label="sales.unitPrice" value={quotationForm.unit_price} onChange={(value) => setQuotationForm({ ...quotationForm, unit_price: value })} />
                <TextInput label="finance.taxAmount" value={quotationForm.tax_rate} onChange={(value) => setQuotationForm({ ...quotationForm, tax_rate: value })} />
              </div>
              <SubmitButton loading={loading || !quotationForm.item_id || !quotationForm.warehouse_id} label="sales.saveQuotation" />
            </form>
          </Panel>
        </div>
      )}

      {activeTab === 'orders' && (
        <div className="grid gap-5 xl:grid-cols-[1fr_360px]">
          <Panel title="sales.orders">
            <DataTable
              headers={['sales.document', 'finance.customer', 'finance.amount', 'developer.status', 'sales.approvalStatus']}
              rows={orders.map((item) => [item.order_number || documentKey(item), item.customer_name, money(item.total_amount, item.currency), <StatusPill key={item.id} label={item.status} />, <StatusPill key={`${item.id}-approval`} label={item.approval_status} />])}
              actions={orders.map((item) => (
                <div key={item.id} className="flex flex-wrap gap-2">
                  <ActionButton label="sales.confirm" onClick={() => void run(() => confirmSalesOrder(token, item.id).then(() => undefined), 'sales.orderConfirmed')} disabled={item.status === 'confirmed' || item.status === 'posted'} icon={<Send className="h-3.5 w-3.5" />} />
                  <ActionButton label="sales.approve" onClick={() => void run(() => approveSalesOrder(token, item.id).then(() => undefined), 'sales.orderApproved')} disabled={item.status === 'approved'} tone="green" icon={<CheckCircle2 className="h-3.5 w-3.5" />} />
                </div>
              ))}
            />
          </Panel>
          <Panel title="sales.createOrder">
            <form className="space-y-3" onSubmit={submitOrder}>
              <TextInput label="sales.documentNumber" value={orderForm.order_number} onChange={(value) => setOrderForm({ ...orderForm, order_number: value })} />
              <SelectInput label="sales.quotation" value={orderForm.quotation_id} onChange={(value) => setOrderForm({ ...orderForm, quotation_id: value })} options={quotations.map((item) => item.id)} labels={quotationLabels} />
              <SelectInput label="finance.customer" value={customers.find((customer) => customer.partner_code === orderForm.customer_id)?.id || ''} onChange={(value) => setOrderForm(applyCustomer(orderForm, value))} options={customers.map((customer) => customer.id)} labels={customerLabels} />
              <TextInput label="finance.customer" value={orderForm.customer_name} onChange={(value) => setOrderForm({ ...orderForm, customer_name: value })} />
              <SelectInput label="inventory.item" value={orderForm.item_id} onChange={(value) => setOrderForm({ ...orderForm, item_id: value })} options={items.map((item) => item.id)} labels={itemLabels} required />
              <SelectInput label="inventory.warehouse" value={orderForm.warehouse_id} onChange={(value) => setOrderForm({ ...orderForm, warehouse_id: value })} options={warehouses.map((warehouse) => warehouse.id)} labels={warehouseLabels} required />
              <div className="grid gap-3 sm:grid-cols-3">
                <TextInput label="inventory.quantity" value={orderForm.quantity} onChange={(value) => setOrderForm({ ...orderForm, quantity: value })} />
                <TextInput label="sales.unitPrice" value={orderForm.unit_price} onChange={(value) => setOrderForm({ ...orderForm, unit_price: value })} />
                <TextInput label="finance.taxAmount" value={orderForm.tax_rate} onChange={(value) => setOrderForm({ ...orderForm, tax_rate: value })} />
              </div>
              <SubmitButton loading={loading || !orderForm.item_id || !orderForm.warehouse_id} label="sales.saveOrder" />
            </form>
          </Panel>
        </div>
      )}

      {activeTab === 'shipments' && (
        <div className="grid gap-5 xl:grid-cols-[1fr_360px]">
          <Panel title="sales.shipments">
            <DataTable
              headers={['sales.document', 'finance.customer', 'finance.amount', 'developer.status', 'finance.receivable']}
              rows={shipments.map((item) => [item.shipment_number || documentKey(item), item.customer_name, money(item.total_amount, item.currency), <StatusPill key={item.id} label={item.status} />, item.receivable_id || t('common.none')])}
              actions={shipments.map((item) => (
                <ActionButton key={item.id} label="sales.postShipment" onClick={() => void run(() => postSalesShipment(token, item.id).then(() => undefined), 'sales.shipmentPosted')} disabled={item.status === 'posted'} tone="primary" icon={<PackageMinus className="h-3.5 w-3.5" />} />
              ))}
            />
          </Panel>
          <Panel title="sales.createShipment">
            <form className="space-y-3" onSubmit={submitShipment}>
              <TextInput label="sales.documentNumber" value={shipmentForm.shipment_number} onChange={(value) => setShipmentForm({ ...shipmentForm, shipment_number: value })} />
              <SelectInput label="sales.order" value={shipmentForm.order_id} onChange={(value) => setShipmentForm({ ...shipmentForm, order_id: value })} options={orders.map((item) => item.id)} labels={orderLabels} />
              <TextInput label="finance.customer" value={shipmentForm.customer_name} onChange={(value) => setShipmentForm({ ...shipmentForm, customer_name: value })} />
              <SelectInput label="inventory.item" value={shipmentForm.item_id} onChange={(value) => setShipmentForm({ ...shipmentForm, item_id: value })} options={items.map((item) => item.id)} labels={itemLabels} required />
              <SelectInput label="inventory.warehouse" value={shipmentForm.warehouse_id} onChange={(value) => setShipmentForm({ ...shipmentForm, warehouse_id: value })} options={warehouses.map((warehouse) => warehouse.id)} labels={warehouseLabels} required />
              <div className="grid gap-3 sm:grid-cols-3">
                <TextInput label="inventory.quantity" value={shipmentForm.quantity} onChange={(value) => setShipmentForm({ ...shipmentForm, quantity: value })} />
                <TextInput label="sales.unitPrice" value={shipmentForm.unit_price} onChange={(value) => setShipmentForm({ ...shipmentForm, unit_price: value })} />
                <TextInput label="finance.taxAmount" value={shipmentForm.tax_rate} onChange={(value) => setShipmentForm({ ...shipmentForm, tax_rate: value })} />
              </div>
              <SubmitButton loading={loading || !shipmentForm.item_id || !shipmentForm.warehouse_id} label="sales.saveShipment" />
            </form>
          </Panel>
        </div>
      )}

      {activeTab === 'returns' && (
        <div className="grid gap-5 xl:grid-cols-[1fr_360px]">
          <Panel title="sales.returns">
            <DataTable
              headers={['sales.document', 'finance.customer', 'finance.amount', 'developer.status']}
              rows={returns.map((item) => [item.return_number || documentKey(item), item.customer_name, money(item.total_amount, item.currency), <StatusPill key={item.id} label={item.status} />])}
            />
          </Panel>
          <Panel title="sales.createReturn">
            <form className="space-y-3" onSubmit={submitReturn}>
              <TextInput label="sales.documentNumber" value={returnForm.return_number} onChange={(value) => setReturnForm({ ...returnForm, return_number: value })} />
              <SelectInput label="sales.shipment" value={returnForm.shipment_id} onChange={(value) => setReturnForm({ ...returnForm, shipment_id: value })} options={shipments.map((item) => item.id)} labels={shipmentLabels} />
              <TextInput label="finance.customer" value={returnForm.customer_name} onChange={(value) => setReturnForm({ ...returnForm, customer_name: value })} />
              <SelectInput label="inventory.item" value={returnForm.item_id} onChange={(value) => setReturnForm({ ...returnForm, item_id: value })} options={items.map((item) => item.id)} labels={itemLabels} required />
              <SelectInput label="inventory.warehouse" value={returnForm.warehouse_id} onChange={(value) => setReturnForm({ ...returnForm, warehouse_id: value })} options={warehouses.map((warehouse) => warehouse.id)} labels={warehouseLabels} required />
              <div className="grid gap-3 sm:grid-cols-3">
                <TextInput label="inventory.quantity" value={returnForm.quantity} onChange={(value) => setReturnForm({ ...returnForm, quantity: value })} />
                <TextInput label="sales.unitPrice" value={returnForm.unit_price} onChange={(value) => setReturnForm({ ...returnForm, unit_price: value })} />
                <TextInput label="finance.taxAmount" value={returnForm.tax_rate} onChange={(value) => setReturnForm({ ...returnForm, tax_rate: value })} />
              </div>
              <SubmitButton loading={loading || !returnForm.item_id || !returnForm.warehouse_id} label="sales.saveReturn" />
            </form>
          </Panel>
        </div>
      )}
        </>
      )}
    </div>
  )
}

type SalesDocumentSelection = {
  targetType: string
  document: Record<string, any>
}

function salesTabForSelection(targetType?: string): TabID | null {
  const tabsByType: Record<string, TabID> = {
    sales_quotation: 'quotations',
    sales_order: 'orders',
    sales_shipment: 'shipments',
    sales_return: 'returns',
  }
  return targetType ? tabsByType[targetType] ?? null : null
}

function salesTabForFunction(functionID?: string): TabID | null {
  const tabsByFunction: Record<string, TabID> = {
    'sales:quotations': 'quotations',
    'sales:orders': 'orders',
    'sales:shipments': 'shipments',
    'sales:returns': 'returns',
  }
  return functionID ? tabsByFunction[functionID] ?? null : null
}

function selectedSalesDocument(
  selection: SupplyChainSelection | null | undefined,
  data: {
    quotations: SalesQuotation[]
    orders: SalesOrder[]
    shipments: SalesShipment[]
    returns: SalesReturn[]
  },
): SalesDocumentSelection | null {
  if (!selection) return null
  const byType: Record<string, Array<Record<string, any>>> = {
    sales_quotation: data.quotations,
    sales_order: data.orders,
    sales_shipment: data.shipments,
    sales_return: data.returns,
  }
  const document = byType[selection.targetType]?.find((item) => item.id === selection.targetID) ?? selection.record
  return document ? { targetType: selection.targetType, document: document as Record<string, any> } : null
}

function salesDocumentTitle(selection: SalesDocumentSelection): string {
  const document = selection.document
  return String(document.quotation_number || document.order_number || document.shipment_number || document.return_number || document.master_key || document.id || '')
}

function salesMainFields(document: Record<string, any>, targetType: string): Array<{ label: string; value: any }> {
  return [
    { label: 'sales.documentNumber', value: document.quotation_number || document.order_number || document.shipment_number || document.return_number || document.master_key },
    { label: 'finance.customer', value: document.customer_name || document.customer_id },
    { label: 'developer.status', value: <StatusPill label={document.status || ''} /> },
    { label: 'sales.approvalStatus', value: <StatusPill label={document.approval_status || (targetType === 'sales_quotation' ? 'not_required' : '')} /> },
    { label: 'finance.amount', value: money(document.total_amount, document.currency) },
    { label: 'finance.currency', value: document.currency },
    { label: 'businessStatus.updated', value: document.updated_at || document.created_at },
    { label: 'businessStatus.code', value: document.id },
  ]
}

function salesLineColumns(): string[] {
  return ['inventory.item', 'inventory.warehouse', 'inventory.quantity', 'sales.unitPrice', 'finance.taxAmount', 'finance.amount']
}

function salesLineRows(document: Record<string, any> | undefined, itemLabels: Record<string, string>, warehouseLabels: Record<string, string>): any[][] {
  const lines = Array.isArray(document?.lines) ? document.lines : []
  return lines.map((line: Record<string, any>) => [
    itemLabels[line.item_id] ?? line.item_id,
    warehouseLabels[line.warehouse_id] ?? line.warehouse_id,
    quantity(line.quantity),
    money(line.unit_price, document?.currency),
    money(line.tax_amount, document?.currency),
    money(line.total_amount ?? line.amount, document?.currency),
  ])
}

function salesDetailActions(
  selection: SalesDocumentSelection,
  token: string,
  run: (action: () => Promise<void>, success: string) => Promise<void>,
) {
  const document = selection.document
  if (!document.id) return null
  if (selection.targetType === 'sales_order') {
    return (
      <div className="flex flex-wrap gap-2">
        <ActionButton label="sales.confirm" onClick={() => void run(() => confirmSalesOrder(token, document.id).then(() => undefined), 'sales.orderConfirmed')} disabled={document.status === 'confirmed' || document.status === 'posted'} icon={<Send className="h-3.5 w-3.5" />} />
        <ActionButton label="sales.approve" onClick={() => void run(() => approveSalesOrder(token, document.id).then(() => undefined), 'sales.orderApproved')} disabled={document.status === 'approved'} tone="green" icon={<CheckCircle2 className="h-3.5 w-3.5" />} />
      </div>
    )
  }
  if (selection.targetType === 'sales_shipment') {
    return <ActionButton label="sales.postShipment" onClick={() => void run(() => postSalesShipment(token, document.id).then(() => undefined), 'sales.shipmentPosted')} disabled={document.status === 'posted'} tone="primary" icon={<PackageMinus className="h-3.5 w-3.5" />} />
  }
  return null
}
