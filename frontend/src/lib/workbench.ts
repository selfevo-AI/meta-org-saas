import type { ApiOperation } from './operations'

export type FieldAccessBehavior = 'allow' | 'readonly' | 'mask' | 'deny'
export type WorkbenchFieldAction = 'read' | 'write' | 'delete'

export interface FieldPermissionLike {
  table_name?: string
  field_name?: string
  action?: string
  behavior?: string
  reason?: string
  priority?: number
  status?: string
}

export interface DocumentWorkbenchField {
  name: string
  labelKey: string
  tableName: string
  dataType?: string
  required?: boolean
  primary?: boolean
  strongBusinessLogic?: boolean
  deletable?: boolean
  width?: number
  readOnly?: boolean
  defaultValue?: string
  options?: Array<{ value: string; labelKey: string }>
}

export interface DocumentWorkbenchDetailTable {
  tableName: string
  labelKey: string
  parentKey: string
  lineKey: string
  fields: DocumentWorkbenchField[]
  allowCreate?: boolean
  allowDelete?: boolean
}

export interface DocumentWorkbenchAction {
  id: string
  labelKey: string
  operation?: ApiOperation
  dangerLevel?: 'low' | 'medium' | 'high'
  disabledReasonKey?: string
  parameters?: DocumentWorkbenchField[]
}

export interface DocumentWorkbenchLink {
  id: string
  labelKey: string
  href: string
  kind: 'module' | 'table' | 'field' | 'operation' | 'document'
}

export interface DocumentWorkbenchDefinition {
  id: string
  moduleKey: string
  titleKey: string
  tableName: string
  primaryKey: string
  headerFields: DocumentWorkbenchField[]
  detailTables: DocumentWorkbenchDetailTable[]
  actions: DocumentWorkbenchAction[]
  links: DocumentWorkbenchLink[]
  fieldPermissions?: FieldPermissionLike[]
}

export interface ResolvedFieldCapability {
  readable: boolean
  writable: boolean
  deletable: boolean
  masked: boolean
  lockedReason?: 'strong_business_logic' | 'permission_rule'
  reason?: string
}

export function resolveFieldCapability(
  field: DocumentWorkbenchField,
  permissions: FieldPermissionLike[] = [],
): ResolvedFieldCapability {
  const readRule = strongestRule(field, permissions, 'read')
  const writeRule = strongestRule(field, permissions, 'write')
  const deleteRule = strongestRule(field, permissions, 'delete')
  const readBehavior = normalizeBehavior(readRule?.behavior)
  const writeBehavior = normalizeBehavior(writeRule?.behavior)
  const deleteBehavior = normalizeBehavior(deleteRule?.behavior)
  const strongLocked = field.strongBusinessLogic || field.primary || field.deletable === false

  if (strongLocked) {
    return {
      readable: readBehavior !== 'deny',
      writable: !field.readOnly && readBehavior !== 'deny' && readBehavior !== 'mask' && writeBehavior === 'allow',
      deletable: false,
      masked: readBehavior === 'mask',
      lockedReason: 'strong_business_logic',
      reason: readRule?.reason || writeRule?.reason || deleteRule?.reason,
    }
  }

  return {
    readable: readBehavior !== 'deny',
    writable: !field.readOnly && readBehavior !== 'deny' && readBehavior !== 'mask' && writeBehavior === 'allow',
    deletable: readBehavior !== 'deny' && deleteBehavior === 'allow',
    masked: readBehavior === 'mask',
    lockedReason: deniedByRule(readBehavior, writeBehavior, deleteBehavior) ? 'permission_rule' : undefined,
    reason: readRule?.reason || writeRule?.reason || deleteRule?.reason,
  }
}

export function defaultWorkbenchFields(tableName: string, primaryKey: string): DocumentWorkbenchField[] {
  const f = (name: string, label: string, extra: Partial<DocumentWorkbenchField> = {}): DocumentWorkbenchField => ({ name, labelKey: `ontology.field.${label}`, tableName, ...extra })
  const primary = f(primaryKey, 'key', { primary: true, required: true, deletable: false })
  const currency = (name = 'Currency') => f(name, 'currency', { defaultValue: 'CNY', required: true })
  const name = f('Name', 'name', { required: true })
  const active = f('Active', 'active', { dataType: 'boolean', defaultValue: 'Y' })
  const readonly = (name: string, label: string) => f(name, label, { readOnly: true })
  const today = new Date().toISOString().slice(0, 10)
  switch (tableName) {
    case 'MCRD':
      return [primary, f('CardName', 'name', { required: true }), f('CardType', 'partnerType', { required: true, defaultValue: 'S', options: [{ value: 'S', labelKey: 'ontology.supplier' }, { value: 'C', labelKey: 'ontology.customer' }] }), currency(), f('Phone1', 'phone'), f('E_Mail', 'email', { dataType: 'email' }), f('ValidFor', 'active', { dataType: 'boolean', defaultValue: 'Y' })]
    case 'MITM':
      return [primary, f('ItemName', 'name', { required: true }), f('InvntryUom', 'unit', { required: true }), f('InvntItem', 'inventoryItem', { dataType: 'boolean', defaultValue: 'Y' }), f('validFor', 'active', { dataType: 'boolean', defaultValue: 'Y' })]
    case 'MWHS':
      return [primary, f('WhsName', 'name', { required: true }), f('Inactive', 'inactive', { dataType: 'boolean', defaultValue: 'N' })]
    case 'MITW':
      return [primary, readonly('BaseItemCode', 'item'), readonly('WhsCode', 'warehouse'), readonly('OnHand', 'onHand'), readonly('InventoryValue', 'inventoryValue'), readonly('AvgPrice', 'averageCost'), readonly('Currency', 'currency')]
    case 'MACT':
      return [primary, name, f('AccountType', 'accountType', { defaultValue: 'asset', options: ['asset', 'liability', 'equity', 'revenue', 'expense'].map((value) => ({ value, labelKey: `ontology.accountType.${value}` })) }), currency(), f('ParentAcctCode', 'parentAccount'), active, f('Postable', 'postable', { dataType: 'boolean', defaultValue: 'Y' })]
    case 'MPRC':
      return [primary, name, active]
    case 'MJDT':
      return [primary, f('Memo', 'memo', { required: true }), f('RefDate', 'postingDate', { dataType: 'date', required: true, defaultValue: today }), currency(), readonly('BtfStatus', 'status'), readonly('BaseEntry', 'source')]
    case 'MBOM':
    case 'MWOR':
      return [primary, name, f('ItemCode', 'item', { required: true }), f('Quantity', 'quantity', { dataType: 'number', defaultValue: '1', required: true }), f('SourceWhsCode', 'sourceWarehouse'), f('FinishedWhsCode', 'warehouse'), readonly('Status', 'status')]
    // SMT MES industry solution tables.
    case 'MLIN':
      return [primary, name, readonly('Status', 'status')]
    case 'MMAC':
      return [primary, name, f('line_code', 'lineName'), readonly('current_work_order', 'workOrder'), readonly('verified_at', 'verifiedAt'), readonly('signed_off_at', 'signedOffAt'), readonly('Status', 'status')]
    case 'MOPR':
      return [primary, name, f('OpType', 'opType', { options: ['SMT', 'REFLOW', 'INSPECT', 'TEST', 'ASSEMBLY', 'PACK', 'DEPANEL', 'OTHER'].map((value) => ({ value, labelKey: `ontology.opType.${value}` })) }), readonly('Status', 'status')]
    case 'MROU':
      return [primary, f('product_code', 'product', { required: true }), f('version', 'version'), f('active', 'active', { dataType: 'boolean', defaultValue: 'true' }), readonly('Status', 'status')]
    case 'MPRD':
      return [primary, name, f('version', 'version'), f('customer_code', 'customer'), readonly('Status', 'status')]
    case 'MCPM':
      return [primary, f('customer_code', 'customer', { required: true }), f('customer_part_no', 'customerPartNo', { required: true }), f('part_no', 'item', { required: true }), f('Remark', 'remarks')]
    case 'MFSB':
      return [primary, f('product_code', 'product', { required: true }), f('machine_code', 'machine', { required: true }), f('program_name', 'program'), f('version', 'version'), f('active', 'active', { dataType: 'boolean', defaultValue: 'true' }), readonly('Status', 'status')]
    case 'MSWO':
      return [primary, f('product_code', 'product', { required: true }), f('line_code', 'lineName'), f('plan_qty', 'planQty', { dataType: 'number', required: true, defaultValue: '0' }), readonly('done_qty', 'doneQty'), f('customer_code', 'customer'), f('route_code', 'routing'), readonly('Status', 'status')]
    case 'MRQN':
      return [primary, f('work_order', 'workOrder', { required: true }), f('batch_qty', 'batchQty', { dataType: 'number' }), f('loss_rate', 'lossRate', { dataType: 'number' }), f('unit_basis', 'unitBasis', { options: [{ value: 'QTY', labelKey: 'smt.unitBasis.QTY' }, { value: 'REEL', labelKey: 'smt.unitBasis.REEL' }] }), readonly('Status', 'status')]
    case 'MDFC':
      return [primary, name, f('Category', 'category'), readonly('Status', 'status')]
    case 'MREL':
      return [primary, readonly('part_no', 'item'), readonly('qty', 'quantity'), readonly('lot_no', 'lot'), readonly('status', 'status'), readonly('owner_type', 'ownership'), readonly('location', 'location'), readonly('current_slot', 'currentSlot')]
    case 'MLOD':
      return [primary, readonly('work_order', 'workOrder'), readonly('machine_code', 'machine'), readonly('slot_no', 'slotNo'), readonly('reel_code', 'reelCode'), readonly('part_no', 'item'), readonly('position', 'position'), readonly('status', 'status')]
    case 'MVER':
      return [primary, readonly('machine_code', 'machine'), readonly('slot_no', 'slotNo'), readonly('reel_code', 'reelCode'), readonly('result', 'result'), readonly('error_type', 'errorType'), readonly('message', 'message')]
    case 'MLWA':
      return [primary, readonly('machine_code', 'machine'), readonly('slot_no', 'slotNo'), readonly('part_no', 'item'), readonly('qty_est', 'qtyEst'), readonly('boards_left', 'boardsLeft'), readonly('status', 'status')]
    case 'MUNT':
      return [primary, readonly('work_order', 'workOrder'), readonly('product_code', 'product'), readonly('status', 'status'), readonly('last_seq', 'lastSeq'), readonly('fail_count', 'failCount'), readonly('panel_code', 'panel')]
    case 'MPNL':
      return [primary, readonly('work_order', 'workOrder'), readonly('board_count', 'boardCount'), readonly('status', 'status')]
    case 'MUDF':
      return [primary, readonly('sn', 'sn'), readonly('defect_code', 'defectCode'), readonly('location', 'location'), readonly('part_no', 'item'), readonly('status', 'status')]
    case 'MINS':
      return [primary, f('insp_type', 'inspType', { options: [{ value: 'FAI', labelKey: 'smt.inspType.FAI' }, { value: 'IPQC', labelKey: 'smt.inspType.IPQC' }] }), readonly('work_order', 'workOrder'), readonly('sample_sn', 'sampleSn'), readonly('result', 'result')]
    case 'MAND':
      return [primary, f('line_code', 'lineName'), f('machine_code', 'machine'), f('work_order', 'workOrder'), f('call_type', 'callType', { options: ['MATERIAL', 'EQUIPMENT', 'QUALITY', 'PROCESS', 'OTHER'].map((value) => ({ value, labelKey: `smt.andonType.${value}` })) }), readonly('status', 'status'), f('description', 'description'), f('cause', 'cause'), f('solution', 'solution')]
    case 'MDWN':
      return [primary, f('machine_code', 'machine'), f('line_code', 'lineName'), f('reason_type', 'reasonType', { options: ['EQUIPMENT', 'MATERIAL', 'CHANGEOVER', 'QUALITY', 'PROCESS', 'PLANNED', 'OTHER'].map((value) => ({ value, labelKey: `smt.downtimeReason.${value}` })) }), f('reason', 'reason'), f('planned', 'planned', { dataType: 'boolean', defaultValue: 'false' }), readonly('started_at', 'startedAt'), f('ended_at', 'endedAt')]
    case 'MSET':
      return [primary, f('Value', 'value'), f('Description', 'description')]
  }
  if (primaryKey !== 'DocEntry') return [primary, name, f('Comments', 'remarks')]
  const payment = tableName === 'MRCT' || tableName === 'MVPM'
  const inventory = tableName === 'MIGN' || tableName === 'MIGE'
  return [
    primary,
    f('DocDate', 'date', { dataType: 'date', defaultValue: today, required: true }),
    ...(!inventory ? [f('CardCode', 'partner', { required: true })] : []),
    currency('DocCur'),
    ...(!payment && !inventory ? [f('DocDueDate', 'dueDate', { dataType: 'date' })] : []),
    f('DocTotal', 'total', { dataType: 'number', readOnly: !payment, required: payment }),
    ...(!payment && !inventory ? [readonly('VatSum', 'tax')] : []),
    ...(payment ? [readonly('AllocatedAmount', 'allocated'), readonly('OpenBal', 'openBalance')] : []),
    ...(['MINV', 'MPCH'].includes(tableName) ? [readonly('PaidToDate', 'paid')] : []),
    readonly('DocStatus', 'status'), readonly('WddStatus', 'approvalStatus'), readonly('Posted', 'posted'),
    readonly('BaseEntry', 'source'), f('Comments', 'remarks'),
  ]
}

export function defaultWorkbenchLineFields(tableName: string): DocumentWorkbenchField[] {
  const f = (name: string, label: string, extra: Partial<DocumentWorkbenchField> = {}): DocumentWorkbenchField => ({ name, labelKey: `ontology.field.${label}`, tableName, ...extra })
  const line = f('LineNum', 'line', { primary: true, dataType: 'integer', required: true })
  if (tableName === 'JDT1') return [line, f('AccountCode', 'account', { required: true }), f('Debit', 'debit', { dataType: 'number', defaultValue: '0' }), f('Credit', 'credit', { dataType: 'number', defaultValue: '0' }), f('CostCenterCode', 'costCenter'), f('Description', 'description')]
  if (tableName === 'RCT1' || tableName === 'VPM1') return [line, f('TargetKey', 'invoice', { readOnly: true }), f('Amount', 'amount', { readOnly: true }), f('Currency', 'currency', { readOnly: true })]
  if (tableName === 'CRD1') return [line, f('Address', 'address'), f('City', 'city'), f('Country', 'country')]
  if (tableName === 'ITM1') return [line, f('Price', 'price', { dataType: 'number' }), f('Currency', 'currency', { defaultValue: 'CNY' })]
  // SMT MES industry solution detail rows.
  if (tableName === 'LIN1') return [line, f('MachineCode', 'machine', { required: true }), f('Name', 'name'), f('MachineType', 'machineType')]
  if (tableName === 'MAC1') return [line, f('SlotNo', 'slotNo', { required: true }), f('Barcode', 'barcode', { required: true }), f('Locked', 'locked', { dataType: 'boolean', defaultValue: 'false' })]
  if (tableName === 'FSB1') return [line, f('SlotNo', 'slotNo', { required: true }), f('PartNo', 'item', { required: true }), f('AltPartNos', 'altParts'), f('Remark', 'remarks')]
  if (tableName === 'ROU1') return [line, f('Seq', 'seq', { dataType: 'integer', required: true }), f('OpCode', 'operation', { required: true }), f('Remark', 'remarks')]
  if (tableName === 'PRD1') return [line, f('PartNo', 'item', { required: true }), f('Qty', 'quantity', { dataType: 'number', defaultValue: '1', required: true }), f('RefDesignators', 'refDesignators'), f('AltPartNos', 'altParts')]
  if (tableName === 'REL1') return [line, f('type', 'txType', { readOnly: true }), f('qty_change', 'qtyChange', { dataType: 'number', readOnly: true }), f('qty_after', 'qtyAfter', { dataType: 'number', readOnly: true }), f('ref', 'ref', { readOnly: true }), f('remark', 'remarks', { readOnly: true })]
  if (tableName === 'LOD1') return [line, f('action', 'action', { readOnly: true }), f('reel_code', 'reelCode', { readOnly: true }), f('part_no', 'item', { readOnly: true }), f('operator', 'operator', { readOnly: true }), f('created_at', 'createdAt', { readOnly: true })]
  if (tableName === 'SWO1') return [line, f('PartNo', 'item', { readOnly: true }), f('RequiredQty', 'requiredQty', { dataType: 'number', readOnly: true }), f('IssuedQty', 'issuedQty', { dataType: 'number', readOnly: true }), f('Gap', 'gap', { dataType: 'number', readOnly: true })]
  if (tableName === 'RQN1') return [line, f('PartNo', 'item', { required: true }), f('RequiredQty', 'requiredQty', { dataType: 'number' }), f('IssuedQty', 'issuedQty', { dataType: 'number', readOnly: true })]
  if (tableName === 'UNT1') return [line, f('seq', 'seq', { dataType: 'integer', readOnly: true }), f('operation_code', 'operation', { readOnly: true }), f('result', 'result', { readOnly: true }), f('reject_type', 'rejectType', { readOnly: true }), f('defect_code', 'defectCode', { readOnly: true }), f('location', 'location', { readOnly: true }), f('machine_code', 'machine', { readOnly: true }), f('created_at', 'createdAt', { readOnly: true })]
  if (tableName === 'PNL1') return [line, f('pos', 'pos', { dataType: 'integer', required: true }), f('sn', 'sn', { required: true })]
  if (tableName === 'INS1') return [line, f('ItemName', 'itemName'), f('Result', 'result'), f('Remark', 'remarks')]
  if (tableName === 'AND1') return [line, f('action', 'action', { readOnly: true }), f('actor', 'operator', { readOnly: true }), f('note', 'notes', { readOnly: true }), f('created_at', 'createdAt', { readOnly: true })]
  return [line, f('ItemCode', 'item', { required: true }), f('Dscription', 'description'), f('Quantity', 'quantity', { dataType: 'number', defaultValue: '1', required: true }), f('Price', 'price', { dataType: 'number', defaultValue: '0', required: true }), f('TaxRate', 'taxRate', { dataType: 'number', defaultValue: '0' }), f('WhsCode', 'warehouse', { required: true })]
}

function strongestRule(field: DocumentWorkbenchField, permissions: FieldPermissionLike[], action: WorkbenchFieldAction) {
  return permissions
    .filter((rule) => {
      const ruleAction = (rule.action || '').toLowerCase()
      return (
        rule.status !== 'disabled' &&
        (!rule.table_name || rule.table_name === field.tableName) &&
        (!rule.field_name || rule.field_name === field.name) &&
        (!ruleAction || ruleAction === action)
      )
    })
    .sort((left, right) => (right.priority ?? 0) - (left.priority ?? 0))[0]
}

function normalizeBehavior(value?: string): FieldAccessBehavior {
  if (value === 'deny' || value === 'readonly' || value === 'mask') return value
  return 'allow'
}

function deniedByRule(...behaviors: FieldAccessBehavior[]): boolean {
  return behaviors.some((behavior) => behavior === 'deny' || behavior === 'readonly' || behavior === 'mask')
}
