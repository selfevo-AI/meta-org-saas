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
