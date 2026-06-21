'use client'

import { Boxes, ClipboardCheck, MoveRight, PackagePlus, RotateCw, Warehouse as WarehouseIcon } from 'lucide-react'
import { FormEvent, useCallback, useEffect, useMemo, useState } from 'react'
import {
  createBusinessPartner,
  createInventoryAdjustment,
  createInventoryCount,
  createInventoryItem,
  createInventoryMovement,
  createInventoryTransfer,
  createWarehouse,
  listBusinessPartners,
  listInventoryAdjustments,
  listInventoryBalances,
  listInventoryCounts,
  listInventoryItems,
  listInventoryMovements,
  listInventoryTransfers,
  listWarehouses,
  type BusinessPartner,
  type InventoryAdjustment,
  type InventoryBalance,
  type InventoryCount,
  type InventoryItem,
  type InventoryMovement,
  type InventoryTransfer,
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

interface InventoryWorkspaceProps {
  token: string
  currentSupplyChainFunctionID?: string
  externalSelection?: SupplyChainSelection | null
}

type TabID = 'master' | 'balances' | 'movements' | 'documents'

const tabs: Array<{ id: TabID; label: string; icon: typeof Boxes }> = [
  { id: 'master', label: 'inventory.masterData', icon: Boxes },
  { id: 'balances', label: 'inventory.balances', icon: WarehouseIcon },
  { id: 'movements', label: 'inventory.movements', icon: MoveRight },
  { id: 'documents', label: 'inventory.documents', icon: ClipboardCheck },
]

const movementTypes = [
  'purchase_receipt',
  'purchase_return',
  'sales_shipment',
  'sales_return',
  'transfer_in',
  'transfer_out',
  'adjustment_in',
  'adjustment_out',
  'count_gain',
  'count_loss',
]

export function InventoryWorkspace({ token, currentSupplyChainFunctionID, externalSelection }: InventoryWorkspaceProps) {
  const { t } = useI18n()
  const [activeTab, setActiveTab] = useState<TabID>('master')
  const [partners, setPartners] = useState<BusinessPartner[]>([])
  const [items, setItems] = useState<InventoryItem[]>([])
  const [warehouses, setWarehouses] = useState<Warehouse[]>([])
  const [balances, setBalances] = useState<InventoryBalance[]>([])
  const [movements, setMovements] = useState<InventoryMovement[]>([])
  const [transfers, setTransfers] = useState<InventoryTransfer[]>([])
  const [adjustments, setAdjustments] = useState<InventoryAdjustment[]>([])
  const [counts, setCounts] = useState<InventoryCount[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [partnerForm, setPartnerForm] = useState({
    partner_code: '',
    partner_type: 'supplier',
    name: '',
    email: '',
    phone: '',
  })
  const [itemForm, setItemForm] = useState({
    item_code: '',
    name: '',
    item_type: 'material',
    base_uom: 'EA',
  })
  const [warehouseForm, setWarehouseForm] = useState({
    warehouse_code: '',
    name: '',
  })
  const [movementForm, setMovementForm] = useState({
    movement_type: 'purchase_receipt',
    source_type: 'manual',
    item_id: '',
    warehouse_id: '',
    quantity: '1',
    unit_cost: '0',
    currency: 'CNY',
  })
  const [transferForm, setTransferForm] = useState({
    transfer_number: '',
    from_warehouse_id: '',
    to_warehouse_id: '',
    item_id: '',
    quantity: '1',
    unit_cost: '0',
  })
  const [adjustmentForm, setAdjustmentForm] = useState({
    adjustment_number: '',
    warehouse_id: '',
    reason: '',
    item_id: '',
    quantity_delta: '1',
    unit_cost: '0',
  })
  const [countForm, setCountForm] = useState({
    count_number: '',
    warehouse_id: '',
    item_id: '',
    book_qty: '0',
    counted_qty: '0',
  })

  const itemLabels = useMemo(() => Object.fromEntries(items.map((item) => [item.id, `${item.item_code || item.master_key || item.id.slice(0, 8)} · ${item.name}`])), [items])
  const warehouseLabels = useMemo(() => Object.fromEntries(warehouses.map((warehouse) => [warehouse.id, `${warehouse.warehouse_code || warehouse.master_key || warehouse.id.slice(0, 8)} · ${warehouse.name}`])), [warehouses])
  const selectedDocument = useMemo(
    () =>
      selectedInventoryDocument(externalSelection, {
        partners,
        items,
        warehouses,
        balances,
        movements,
        transfers,
        adjustments,
        counts,
      }),
    [adjustments, balances, counts, externalSelection, items, movements, partners, transfers, warehouses],
  )
  const detailTitle = selectedDocument ? inventoryDocumentTitle(selectedDocument) : ''
  const lineColumns = inventoryLineColumns(selectedDocument?.targetType)
  const lineRows = inventoryLineRows(selectedDocument?.document, selectedDocument?.targetType, itemLabels, warehouseLabels, t)

  const loadInventory = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const [partnerData, itemData, warehouseData, balanceData, movementData, transferData, adjustmentData, countData] = await Promise.all([
        listBusinessPartners(token),
        listInventoryItems(token),
        listWarehouses(token),
        listInventoryBalances(token),
        listInventoryMovements(token),
        listInventoryTransfers(token),
        listInventoryAdjustments(token),
        listInventoryCounts(token),
      ])
      setPartners(partnerData)
      setItems(itemData)
      setWarehouses(warehouseData)
      setBalances(balanceData)
      setMovements(movementData)
      setTransfers(transferData)
      setAdjustments(adjustmentData)
      setCounts(countData)
      setMovementForm((current) => ({ ...current, item_id: current.item_id || itemData[0]?.id || '', warehouse_id: current.warehouse_id || warehouseData[0]?.id || '' }))
      setTransferForm((current) => ({
        ...current,
        item_id: current.item_id || itemData[0]?.id || '',
        from_warehouse_id: current.from_warehouse_id || warehouseData[0]?.id || '',
        to_warehouse_id: current.to_warehouse_id || warehouseData[1]?.id || '',
      }))
      setAdjustmentForm((current) => ({ ...current, item_id: current.item_id || itemData[0]?.id || '', warehouse_id: current.warehouse_id || warehouseData[0]?.id || '' }))
      setCountForm((current) => ({ ...current, item_id: current.item_id || itemData[0]?.id || '', warehouse_id: current.warehouse_id || warehouseData[0]?.id || '' }))
    } catch (err) {
      setError(err instanceof Error ? err.message : t('inventory.loadFailed'))
    } finally {
      setLoading(false)
    }
  }, [t, token])

  useEffect(() => {
    const timer = window.setTimeout(() => {
      void loadInventory()
    }, 0)
    return () => window.clearTimeout(timer)
  }, [loadInventory])

  useEffect(() => {
    const nextTab = inventoryTabForSelection(externalSelection?.targetType) ?? inventoryTabForFunction(currentSupplyChainFunctionID)
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
      await loadInventory()
    } catch (err) {
      setError(err instanceof Error ? err.message : t('common.operationFailed'))
    } finally {
      setLoading(false)
    }
  }

  async function submitPartner(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await run(
      () =>
        createBusinessPartner(token, {
          partner_code: partnerForm.partner_code,
          partner_type: partnerForm.partner_type,
          name: partnerForm.name,
          email: partnerForm.email,
          phone: partnerForm.phone,
          metadata: {},
        }).then(() => setPartnerForm((current) => ({ ...current, partner_code: '', name: '', email: '', phone: '' }))),
      'inventory.partnerSaved',
    )
  }

  async function submitItem(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await run(
      () =>
        createInventoryItem(token, {
          item_code: itemForm.item_code,
          name: itemForm.name,
          item_type: itemForm.item_type,
          base_uom: itemForm.base_uom,
          metadata: {},
        }).then(() => setItemForm((current) => ({ ...current, item_code: '', name: '' }))),
      'inventory.itemSaved',
    )
  }

  async function submitWarehouse(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await run(
      () =>
        createWarehouse(token, {
          warehouse_code: warehouseForm.warehouse_code,
          name: warehouseForm.name,
          metadata: {},
        }).then(() => setWarehouseForm({ warehouse_code: '', name: '' })),
      'inventory.warehouseSaved',
    )
  }

  async function submitMovement(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await run(
      () =>
        createInventoryMovement(token, {
          movement_type: movementForm.movement_type,
          source_type: movementForm.source_type,
          item_id: movementForm.item_id,
          warehouse_id: movementForm.warehouse_id,
          quantity: Number(movementForm.quantity || 0),
          unit_cost: Number(movementForm.unit_cost || 0),
          currency: movementForm.currency,
          metadata: { source: 'inventory_workspace' },
        }).then(() => setMovementForm((current) => ({ ...current, quantity: '1', unit_cost: '0' }))),
      'inventory.movementPosted',
    )
  }

  async function submitTransfer(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await run(
      () =>
        createInventoryTransfer(token, {
          transfer_number: transferForm.transfer_number,
          from_warehouse_id: transferForm.from_warehouse_id,
          to_warehouse_id: transferForm.to_warehouse_id,
          lines: [
            {
              item_id: transferForm.item_id,
              quantity: Number(transferForm.quantity || 0),
              unit_cost: Number(transferForm.unit_cost || 0),
            },
          ],
          metadata: {},
        }).then(() => setTransferForm((current) => ({ ...current, transfer_number: '', quantity: '1', unit_cost: '0' }))),
      'inventory.transferSaved',
    )
  }

  async function submitAdjustment(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await run(
      () =>
        createInventoryAdjustment(token, {
          adjustment_number: adjustmentForm.adjustment_number,
          warehouse_id: adjustmentForm.warehouse_id,
          reason: adjustmentForm.reason,
          lines: [
            {
              item_id: adjustmentForm.item_id,
              quantity_delta: Number(adjustmentForm.quantity_delta || 0),
              unit_cost: Number(adjustmentForm.unit_cost || 0),
            },
          ],
          metadata: {},
        }).then(() => setAdjustmentForm((current) => ({ ...current, adjustment_number: '', reason: '', quantity_delta: '1', unit_cost: '0' }))),
      'inventory.adjustmentSaved',
    )
  }

  async function submitCount(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await run(
      () =>
        createInventoryCount(token, {
          count_number: countForm.count_number,
          warehouse_id: countForm.warehouse_id,
          lines: [
            {
              item_id: countForm.item_id,
              book_qty: Number(countForm.book_qty || 0),
              counted_qty: Number(countForm.counted_qty || 0),
            },
          ],
          metadata: {},
        }).then(() => setCountForm((current) => ({ ...current, count_number: '', book_qty: '0', counted_qty: '0' }))),
      'inventory.countSaved',
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
        <RefreshButton loading={loading} onClick={() => void loadInventory()} />
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
          mainFields={inventoryMainFields(selectedDocument.document, selectedDocument.targetType, itemLabels, warehouseLabels, t)}
          lineColumns={lineColumns}
          lineRows={lineRows}
        />
      )}

      {!selectedDocument && (
        <>
      {activeTab === 'master' && (
        <div className="grid gap-5 xl:grid-cols-3">
          <Panel title="inventory.partners">
            <DataTable
              headers={['inventory.partnerCode', 'inventory.partnerType', 'common.name', 'developer.status']}
              rows={partners.map((partner) => [partner.partner_code || documentKey(partner), t(`inventory.partnerType.${partner.partner_type}`), partner.name, <StatusPill key={partner.id} label={partner.status} />])}
            />
          </Panel>
          <Panel title="inventory.items">
            <DataTable
              headers={['inventory.itemCode', 'common.name', 'inventory.itemType', 'inventory.uom']}
              rows={items.map((item) => [item.item_code || documentKey(item), item.name, t(`inventory.itemType.${item.item_type}`), item.base_uom])}
            />
          </Panel>
          <Panel title="inventory.warehouses">
            <DataTable
              headers={['inventory.warehouseCode', 'common.name', 'developer.status']}
              rows={warehouses.map((warehouse) => [warehouse.warehouse_code || documentKey(warehouse), warehouse.name, <StatusPill key={warehouse.id} label={warehouse.status} />])}
            />
          </Panel>
          <Panel title="inventory.createPartner">
            <form className="space-y-3" onSubmit={submitPartner}>
              <TextInput label="inventory.partnerCode" value={partnerForm.partner_code} onChange={(value) => setPartnerForm({ ...partnerForm, partner_code: value })} />
              <SelectInput
                label="inventory.partnerType"
                value={partnerForm.partner_type}
                onChange={(value) => setPartnerForm({ ...partnerForm, partner_type: value })}
                options={['supplier', 'customer', 'both']}
                labels={{ supplier: t('inventory.partnerType.supplier'), customer: t('inventory.partnerType.customer'), both: t('inventory.partnerType.both') }}
                required
              />
              <TextInput label="common.name" value={partnerForm.name} onChange={(value) => setPartnerForm({ ...partnerForm, name: value })} required />
              <div className="grid gap-3 sm:grid-cols-2">
                <TextInput label="inventory.email" value={partnerForm.email} onChange={(value) => setPartnerForm({ ...partnerForm, email: value })} />
                <TextInput label="inventory.phone" value={partnerForm.phone} onChange={(value) => setPartnerForm({ ...partnerForm, phone: value })} />
              </div>
              <SubmitButton loading={loading} label="inventory.savePartner" />
            </form>
          </Panel>
          <Panel title="inventory.createItem">
            <form className="space-y-3" onSubmit={submitItem}>
              <TextInput label="inventory.itemCode" value={itemForm.item_code} onChange={(value) => setItemForm({ ...itemForm, item_code: value })} />
              <TextInput label="common.name" value={itemForm.name} onChange={(value) => setItemForm({ ...itemForm, name: value })} required />
              <div className="grid gap-3 sm:grid-cols-2">
                <SelectInput
                  label="inventory.itemType"
                  value={itemForm.item_type}
                  onChange={(value) => setItemForm({ ...itemForm, item_type: value })}
                  options={['material', 'service']}
                  labels={{ material: t('inventory.itemType.material'), service: t('inventory.itemType.service') }}
                />
                <TextInput label="inventory.uom" value={itemForm.base_uom} onChange={(value) => setItemForm({ ...itemForm, base_uom: value })} />
              </div>
              <SubmitButton loading={loading} label="inventory.saveItem" />
            </form>
          </Panel>
          <Panel title="inventory.createWarehouse">
            <form className="space-y-3" onSubmit={submitWarehouse}>
              <TextInput label="inventory.warehouseCode" value={warehouseForm.warehouse_code} onChange={(value) => setWarehouseForm({ ...warehouseForm, warehouse_code: value })} />
              <TextInput label="common.name" value={warehouseForm.name} onChange={(value) => setWarehouseForm({ ...warehouseForm, name: value })} required />
              <SubmitButton loading={loading} label="inventory.saveWarehouse" />
            </form>
          </Panel>
        </div>
      )}

      {activeTab === 'balances' && (
        <Panel title="inventory.balances">
          <DataTable
            headers={['inventory.item', 'inventory.warehouse', 'inventory.quantity', 'inventory.reservedQty', 'inventory.averageCost', 'inventory.valueAmount']}
            rows={balances.map((balance) => [
              itemLabels[balance.item_id] ?? balance.item_id,
              warehouseLabels[balance.warehouse_id] ?? balance.warehouse_id,
              quantity(balance.quantity),
              quantity(balance.reserved_qty),
              money(balance.average_cost, balance.currency),
              money(balance.value_amount, balance.currency),
            ])}
          />
        </Panel>
      )}

      {activeTab === 'movements' && (
        <div className="grid gap-5 xl:grid-cols-[1fr_360px]">
          <Panel title="inventory.movements">
            <DataTable
              headers={['inventory.movementType', 'inventory.item', 'inventory.warehouse', 'inventory.quantity', 'inventory.unitCost', 'inventory.balanceAfter']}
              rows={movements.map((movement) => [
                t(`inventory.movementType.${movement.movement_type}`),
                itemLabels[movement.item_id] ?? movement.item_id,
                warehouseLabels[movement.warehouse_id] ?? movement.warehouse_id,
                quantity(movement.quantity),
                money(movement.unit_cost, movement.currency),
                quantity(movement.balance_after),
              ])}
            />
          </Panel>
          <Panel title="inventory.createMovement">
            <form className="space-y-3" onSubmit={submitMovement}>
              <SelectInput
                label="inventory.movementType"
                value={movementForm.movement_type}
                onChange={(value) => setMovementForm({ ...movementForm, movement_type: value })}
                options={movementTypes}
                labels={Object.fromEntries(movementTypes.map((type) => [type, t(`inventory.movementType.${type}`)]))}
              />
              <TextInput label="inventory.sourceType" value={movementForm.source_type} onChange={(value) => setMovementForm({ ...movementForm, source_type: value })} />
              <SelectInput label="inventory.item" value={movementForm.item_id} onChange={(value) => setMovementForm({ ...movementForm, item_id: value })} options={items.map((item) => item.id)} labels={itemLabels} required />
              <SelectInput label="inventory.warehouse" value={movementForm.warehouse_id} onChange={(value) => setMovementForm({ ...movementForm, warehouse_id: value })} options={warehouses.map((warehouse) => warehouse.id)} labels={warehouseLabels} required />
              <div className="grid gap-3 sm:grid-cols-2">
                <TextInput label="inventory.quantity" value={movementForm.quantity} onChange={(value) => setMovementForm({ ...movementForm, quantity: value })} required />
                <TextInput label="inventory.unitCost" value={movementForm.unit_cost} onChange={(value) => setMovementForm({ ...movementForm, unit_cost: value })} />
              </div>
              <SubmitButton loading={loading || !movementForm.item_id || !movementForm.warehouse_id} label="inventory.postMovement" />
            </form>
          </Panel>
        </div>
      )}

      {activeTab === 'documents' && (
        <div className="grid gap-5 xl:grid-cols-2">
          <Panel title="inventory.transfers">
            <DataTable
              headers={['inventory.documentNumber', 'inventory.fromWarehouse', 'inventory.toWarehouse', 'developer.status']}
              rows={transfers.map((transfer) => [
                transfer.transfer_number || documentKey(transfer),
                warehouseLabels[transfer.from_warehouse_id] ?? transfer.from_warehouse_id,
                warehouseLabels[transfer.to_warehouse_id] ?? transfer.to_warehouse_id,
                <StatusPill key={transfer.id} label={transfer.status} />,
              ])}
            />
            <form className="mt-5 space-y-3 border-t border-slate-100 pt-4" onSubmit={submitTransfer}>
              <TextInput label="inventory.documentNumber" value={transferForm.transfer_number} onChange={(value) => setTransferForm({ ...transferForm, transfer_number: value })} />
              <div className="grid gap-3 sm:grid-cols-2">
                <SelectInput label="inventory.fromWarehouse" value={transferForm.from_warehouse_id} onChange={(value) => setTransferForm({ ...transferForm, from_warehouse_id: value })} options={warehouses.map((warehouse) => warehouse.id)} labels={warehouseLabels} required />
                <SelectInput label="inventory.toWarehouse" value={transferForm.to_warehouse_id} onChange={(value) => setTransferForm({ ...transferForm, to_warehouse_id: value })} options={warehouses.map((warehouse) => warehouse.id)} labels={warehouseLabels} required />
              </div>
              <SelectInput label="inventory.item" value={transferForm.item_id} onChange={(value) => setTransferForm({ ...transferForm, item_id: value })} options={items.map((item) => item.id)} labels={itemLabels} required />
              <div className="grid gap-3 sm:grid-cols-2">
                <TextInput label="inventory.quantity" value={transferForm.quantity} onChange={(value) => setTransferForm({ ...transferForm, quantity: value })} />
                <TextInput label="inventory.unitCost" value={transferForm.unit_cost} onChange={(value) => setTransferForm({ ...transferForm, unit_cost: value })} />
              </div>
              <SubmitButton loading={loading || !transferForm.item_id || !transferForm.from_warehouse_id || !transferForm.to_warehouse_id} label="inventory.saveTransfer" />
            </form>
          </Panel>

          <Panel title="inventory.adjustments">
            <DataTable
              headers={['inventory.documentNumber', 'inventory.warehouse', 'inventory.reason', 'developer.status']}
              rows={adjustments.map((adjustment) => [
                adjustment.adjustment_number || documentKey(adjustment),
                warehouseLabels[adjustment.warehouse_id] ?? adjustment.warehouse_id,
                adjustment.reason,
                <StatusPill key={adjustment.id} label={adjustment.status} />,
              ])}
            />
            <form className="mt-5 space-y-3 border-t border-slate-100 pt-4" onSubmit={submitAdjustment}>
              <TextInput label="inventory.documentNumber" value={adjustmentForm.adjustment_number} onChange={(value) => setAdjustmentForm({ ...adjustmentForm, adjustment_number: value })} />
              <SelectInput label="inventory.warehouse" value={adjustmentForm.warehouse_id} onChange={(value) => setAdjustmentForm({ ...adjustmentForm, warehouse_id: value })} options={warehouses.map((warehouse) => warehouse.id)} labels={warehouseLabels} required />
              <TextInput label="inventory.reason" value={adjustmentForm.reason} onChange={(value) => setAdjustmentForm({ ...adjustmentForm, reason: value })} />
              <SelectInput label="inventory.item" value={adjustmentForm.item_id} onChange={(value) => setAdjustmentForm({ ...adjustmentForm, item_id: value })} options={items.map((item) => item.id)} labels={itemLabels} required />
              <div className="grid gap-3 sm:grid-cols-2">
                <TextInput label="inventory.quantityDelta" value={adjustmentForm.quantity_delta} onChange={(value) => setAdjustmentForm({ ...adjustmentForm, quantity_delta: value })} />
                <TextInput label="inventory.unitCost" value={adjustmentForm.unit_cost} onChange={(value) => setAdjustmentForm({ ...adjustmentForm, unit_cost: value })} />
              </div>
              <SubmitButton loading={loading || !adjustmentForm.item_id || !adjustmentForm.warehouse_id} label="inventory.saveAdjustment" />
            </form>
          </Panel>

          <Panel title="inventory.counts">
            <DataTable
              headers={['inventory.documentNumber', 'inventory.warehouse', 'developer.status']}
              rows={counts.map((count) => [count.count_number || documentKey(count), warehouseLabels[count.warehouse_id] ?? count.warehouse_id, <StatusPill key={count.id} label={count.status} />])}
            />
            <form className="mt-5 space-y-3 border-t border-slate-100 pt-4" onSubmit={submitCount}>
              <TextInput label="inventory.documentNumber" value={countForm.count_number} onChange={(value) => setCountForm({ ...countForm, count_number: value })} />
              <SelectInput label="inventory.warehouse" value={countForm.warehouse_id} onChange={(value) => setCountForm({ ...countForm, warehouse_id: value })} options={warehouses.map((warehouse) => warehouse.id)} labels={warehouseLabels} required />
              <SelectInput label="inventory.item" value={countForm.item_id} onChange={(value) => setCountForm({ ...countForm, item_id: value })} options={items.map((item) => item.id)} labels={itemLabels} required />
              <div className="grid gap-3 sm:grid-cols-2">
                <TextInput label="inventory.bookQty" value={countForm.book_qty} onChange={(value) => setCountForm({ ...countForm, book_qty: value })} />
                <TextInput label="inventory.countedQty" value={countForm.counted_qty} onChange={(value) => setCountForm({ ...countForm, counted_qty: value })} />
              </div>
              <SubmitButton loading={loading || !countForm.item_id || !countForm.warehouse_id} label="inventory.saveCount" />
            </form>
          </Panel>

          <Panel title="inventory.quickActions">
            <div className="grid gap-3 sm:grid-cols-3">
              <ActionButton label="inventory.createItem" onClick={() => setActiveTab('master')} icon={<PackagePlus className="h-3.5 w-3.5" />} />
              <ActionButton label="inventory.postMovement" onClick={() => setActiveTab('movements')} icon={<RotateCw className="h-3.5 w-3.5" />} />
              <ActionButton label="inventory.refreshBalances" onClick={() => void loadInventory()} icon={<WarehouseIcon className="h-3.5 w-3.5" />} />
            </div>
          </Panel>
        </div>
      )}
        </>
      )}
    </div>
  )
}

type InventoryDocumentSelection = {
  targetType: string
  document: Record<string, any>
}

function inventoryTabForSelection(targetType?: string): TabID | null {
  const tabsByType: Record<string, TabID> = {
    business_partner: 'master',
    inventory_item: 'master',
    warehouse: 'master',
    inventory_balance: 'balances',
    inventory_movement: 'movements',
    inventory_transfer: 'documents',
    inventory_adjustment: 'documents',
    inventory_count: 'documents',
  }
  return targetType ? tabsByType[targetType] ?? null : null
}

function inventoryTabForFunction(functionID?: string): TabID | null {
  const tabsByFunction: Record<string, TabID> = {
    'inventory:partners': 'master',
    'inventory:items': 'master',
    'inventory:warehouses': 'master',
    'inventory:balances': 'balances',
    'inventory:movements': 'movements',
    'inventory:transfers': 'documents',
    'inventory:adjustments': 'documents',
    'inventory:counts': 'documents',
  }
  return functionID ? tabsByFunction[functionID] ?? null : null
}

function selectedInventoryDocument(
  selection: SupplyChainSelection | null | undefined,
  data: {
    partners: BusinessPartner[]
    items: InventoryItem[]
    warehouses: Warehouse[]
    balances: InventoryBalance[]
    movements: InventoryMovement[]
    transfers: InventoryTransfer[]
    adjustments: InventoryAdjustment[]
    counts: InventoryCount[]
  },
): InventoryDocumentSelection | null {
  if (!selection) return null
  const byType: Record<string, Array<Record<string, any>>> = {
    business_partner: data.partners,
    inventory_item: data.items,
    warehouse: data.warehouses,
    inventory_balance: data.balances,
    inventory_movement: data.movements,
    inventory_transfer: data.transfers,
    inventory_adjustment: data.adjustments,
    inventory_count: data.counts,
  }
  const document = byType[selection.targetType]?.find((item) => item.id === selection.targetID) ?? selection.record
  return document ? { targetType: selection.targetType, document: document as Record<string, any> } : null
}

function inventoryDocumentTitle(selection: InventoryDocumentSelection): string {
  const document = selection.document
  return String(
    document.name ||
      document.partner_code ||
      document.item_code ||
      document.warehouse_code ||
      document.transfer_number ||
      document.adjustment_number ||
      document.count_number ||
      document.movement_type ||
      document.master_key ||
      document.id ||
      '',
  )
}

function inventoryMainFields(
  document: Record<string, any>,
  targetType: string,
  itemLabels: Record<string, string>,
  warehouseLabels: Record<string, string>,
  t: (key: string) => string,
): Array<{ label: string; value: any }> {
  if (targetType === 'business_partner') {
    return [
      { label: 'inventory.partnerCode', value: document.partner_code || document.master_key },
      { label: 'inventory.partnerType', value: document.partner_type ? t(`inventory.partnerType.${document.partner_type}`) : '' },
      { label: 'common.name', value: document.name },
      { label: 'inventory.email', value: document.email },
      { label: 'inventory.phone', value: document.phone },
      { label: 'developer.status', value: <StatusPill label={document.status || ''} /> },
      { label: 'businessStatus.updated', value: document.updated_at || document.created_at },
      { label: 'businessStatus.code', value: document.id },
    ]
  }
  if (targetType === 'inventory_item') {
    return [
      { label: 'inventory.itemCode', value: document.item_code || document.master_key },
      { label: 'common.name', value: document.name },
      { label: 'inventory.itemType', value: document.item_type ? t(`inventory.itemType.${document.item_type}`) : '' },
      { label: 'inventory.uom', value: document.base_uom },
      { label: 'developer.status', value: <StatusPill label={document.status || ''} /> },
      { label: 'businessStatus.updated', value: document.updated_at || document.created_at },
      { label: 'businessStatus.code', value: document.id },
    ]
  }
  if (targetType === 'warehouse') {
    return [
      { label: 'inventory.warehouseCode', value: document.warehouse_code || document.master_key },
      { label: 'common.name', value: document.name },
      { label: 'developer.status', value: <StatusPill label={document.status || ''} /> },
      { label: 'businessStatus.updated', value: document.updated_at || document.created_at },
      { label: 'businessStatus.code', value: document.id },
    ]
  }
  if (targetType === 'inventory_balance') {
    return [
      { label: 'inventory.item', value: itemLabels[document.item_id] ?? document.item_id },
      { label: 'inventory.warehouse', value: warehouseLabels[document.warehouse_id] ?? document.warehouse_id },
      { label: 'inventory.quantity', value: quantity(document.quantity) },
      { label: 'inventory.reservedQty', value: quantity(document.reserved_qty) },
      { label: 'inventory.averageCost', value: money(document.average_cost, document.currency) },
      { label: 'inventory.valueAmount', value: money(document.value_amount, document.currency) },
      { label: 'finance.currency', value: document.currency },
      { label: 'businessStatus.updated', value: document.updated_at },
    ]
  }
  if (targetType === 'inventory_movement') {
    return [
      { label: 'inventory.movementType', value: document.movement_type ? t(`inventory.movementType.${document.movement_type}`) : '' },
      { label: 'inventory.sourceType', value: document.source_type },
      { label: 'inventory.item', value: itemLabels[document.item_id] ?? document.item_id },
      { label: 'inventory.warehouse', value: warehouseLabels[document.warehouse_id] ?? document.warehouse_id },
      { label: 'inventory.quantity', value: quantity(document.quantity) },
      { label: 'inventory.unitCost', value: money(document.unit_cost, document.currency) },
      { label: 'inventory.balanceAfter', value: quantity(document.balance_after) },
      { label: 'businessStatus.updated', value: document.occurred_at || document.created_at },
    ]
  }
  if (targetType === 'inventory_transfer') {
    return [
      { label: 'inventory.documentNumber', value: document.transfer_number || document.master_key },
      { label: 'inventory.fromWarehouse', value: warehouseLabels[document.from_warehouse_id] ?? document.from_warehouse_id },
      { label: 'inventory.toWarehouse', value: warehouseLabels[document.to_warehouse_id] ?? document.to_warehouse_id },
      { label: 'developer.status', value: <StatusPill label={document.status || ''} /> },
      { label: 'businessStatus.updated', value: document.updated_at || document.created_at },
      { label: 'businessStatus.code', value: document.id },
    ]
  }
  if (targetType === 'inventory_adjustment') {
    return [
      { label: 'inventory.documentNumber', value: document.adjustment_number || document.master_key },
      { label: 'inventory.warehouse', value: warehouseLabels[document.warehouse_id] ?? document.warehouse_id },
      { label: 'inventory.reason', value: document.reason },
      { label: 'developer.status', value: <StatusPill label={document.status || ''} /> },
      { label: 'businessStatus.updated', value: document.updated_at || document.created_at },
      { label: 'businessStatus.code', value: document.id },
    ]
  }
  return [
    { label: 'inventory.documentNumber', value: document.count_number || document.master_key },
    { label: 'inventory.warehouse', value: warehouseLabels[document.warehouse_id] ?? document.warehouse_id },
    { label: 'developer.status', value: <StatusPill label={document.status || ''} /> },
    { label: 'businessStatus.updated', value: document.updated_at || document.created_at },
    { label: 'businessStatus.code', value: document.id },
  ]
}

function inventoryLineColumns(targetType?: string): string[] {
  if (targetType === 'inventory_movement') {
    return ['inventory.item', 'inventory.warehouse', 'inventory.movementType', 'inventory.sourceType', 'inventory.quantity', 'inventory.unitCost', 'inventory.balanceAfter']
  }
  if (targetType === 'inventory_transfer') {
    return ['inventory.item', 'inventory.quantity', 'inventory.unitCost']
  }
  if (targetType === 'inventory_adjustment') {
    return ['inventory.item', 'inventory.quantityDelta', 'inventory.unitCost']
  }
  if (targetType === 'inventory_count') {
    return ['inventory.item', 'inventory.bookQty', 'inventory.countedQty', 'inventory.varianceQty']
  }
  return ['supplyChain.lineItems']
}

function inventoryLineRows(
  document: Record<string, any> | undefined,
  targetType: string | undefined,
  itemLabels: Record<string, string>,
  warehouseLabels: Record<string, string>,
  t: (key: string) => string,
): any[][] {
  if (!document) return []
  if (targetType === 'inventory_movement') {
    return [
      [
        itemLabels[document.item_id] ?? document.item_id,
        warehouseLabels[document.warehouse_id] ?? document.warehouse_id,
        document.movement_type ? t(`inventory.movementType.${document.movement_type}`) : '',
        document.source_type,
        quantity(document.quantity),
        money(document.unit_cost, document.currency),
        quantity(document.balance_after),
      ],
    ]
  }
  const lines = Array.isArray(document.lines) ? document.lines : []
  if (targetType === 'inventory_transfer') {
    return lines.map((line: Record<string, any>) => [
      itemLabels[line.item_id] ?? line.item_id,
      quantity(line.quantity),
      money(line.unit_cost),
    ])
  }
  if (targetType === 'inventory_adjustment') {
    return lines.map((line: Record<string, any>) => [
      itemLabels[line.item_id] ?? line.item_id,
      quantity(line.quantity_delta),
      money(line.unit_cost),
    ])
  }
  if (targetType === 'inventory_count') {
    return lines.map((line: Record<string, any>) => [
      itemLabels[line.item_id] ?? line.item_id,
      quantity(line.book_qty),
      quantity(line.counted_qty),
      quantity(line.variance_qty),
    ])
  }
  return []
}
