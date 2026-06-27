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
      writable: readBehavior !== 'deny' && writeBehavior === 'allow',
      deletable: false,
      masked: readBehavior === 'mask',
      lockedReason: 'strong_business_logic',
      reason: readRule?.reason || writeRule?.reason || deleteRule?.reason,
    }
  }

  return {
    readable: readBehavior !== 'deny',
    writable: readBehavior !== 'deny' && writeBehavior === 'allow',
    deletable: readBehavior !== 'deny' && deleteBehavior === 'allow',
    masked: readBehavior === 'mask',
    lockedReason: deniedByRule(readBehavior, writeBehavior, deleteBehavior) ? 'permission_rule' : undefined,
    reason: readRule?.reason || writeRule?.reason || deleteRule?.reason,
  }
}

export function defaultWorkbenchFields(tableName: string, primaryKey: string): DocumentWorkbenchField[] {
  return [
    { name: primaryKey, labelKey: 'workbench.field.primaryKey', tableName, primary: true, strongBusinessLogic: true, deletable: false },
    { name: 'DocNum', labelKey: 'workbench.field.documentNumber', tableName, strongBusinessLogic: true, deletable: false },
    { name: 'DocDate', labelKey: 'workbench.field.documentDate', tableName },
    { name: 'CardCode', labelKey: 'workbench.field.counterparty', tableName },
    { name: 'DocStatus', labelKey: 'workbench.field.status', tableName, strongBusinessLogic: true, deletable: false },
    { name: 'Comments', labelKey: 'workbench.field.remarks', tableName },
  ]
}

export function defaultWorkbenchLineFields(tableName: string): DocumentWorkbenchField[] {
  return [
    { name: 'LineNum', labelKey: 'workbench.field.lineNumber', tableName, primary: true, strongBusinessLogic: true, deletable: false },
    { name: 'ItemCode', labelKey: 'workbench.field.itemCode', tableName },
    { name: 'Dscription', labelKey: 'workbench.field.description', tableName },
    { name: 'Quantity', labelKey: 'workbench.field.quantity', tableName },
    { name: 'Price', labelKey: 'workbench.field.price', tableName },
    { name: 'WhsCode', labelKey: 'workbench.field.warehouse', tableName },
  ]
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
