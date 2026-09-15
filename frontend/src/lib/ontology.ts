import { apiRequest, type ERPActionExecution, type ERPActionResult, type ERPRecord } from './api'

export type OntologyLabel = { zh: string; en: string }
export type OntologyProperty = { key: string; source_field: string; data_type: string; label: OntologyLabel }
export type OntologyAction = { key: string; label: OntologyLabel; requires_approval: boolean; parameters?: OntologyProperty[] }
export type OntologyType = {
  importable?: boolean
  schema_version?: number
  key: string
  table_code: string
  label: OntologyLabel
  primary_key: string
  properties: OntologyProperty[]
  actions: OntologyAction[]
}
export type OntologyObject = {
  type: string
  key: string
  table_code: string
  title: string
  properties: Record<string, unknown>
}
export type OntologyLink = { type: string; label: OntologyLabel; object: OntologyObject }
export type OntologyLinks = { links: OntologyLink[]; truncated: boolean }

export const ontologyTypeByTable: Record<string, string> = {
  MCRD: 'business_partner', MITM: 'item', MWHS: 'warehouse', MITW: 'stock_balance',
  MPOR: 'purchase_order', MPDN: 'goods_receipt', MPCH: 'payable_invoice', MVPM: 'outgoing_payment',
  MRDR: 'sales_order', MDLN: 'delivery', MINV: 'receivable_invoice', MRCT: 'incoming_payment',
  MIGN: 'inventory_receipt', MIGE: 'inventory_issue', MACT: 'account', MJDT: 'journal_entry',
  MPRC: 'cost_center', MPRJ: 'project', MREQ: 'requirement',
}

const objectPath = (type: string, key?: string) => `/ontology/objects/${encodeURIComponent(type)}${key === undefined ? '' : `/${encodeURIComponent(key)}`}`

export function getOntologyType(token: string, type: string) {
  return apiRequest<OntologyType>(`/ontology/types/${encodeURIComponent(type)}`, { token })
}

export type OntologyQueryOptions = { status?: string; sort?: string; direction?: 'asc' | 'desc' }

export function queryOntologyObjects(token: string, type: string, search = '', cursor = '', filters: Record<string, unknown> = {}, options: OntologyQueryOptions = {}) {
  return apiRequest<{ objects: OntologyObject[]; next_cursor?: string; total?: number }>(`${objectPath(type)}/query`, {
    method: 'POST', token, body: { search, cursor, filters, limit: 50, ...options },
  })
}

export function getOntologyLinks(token: string, type: string, key: string) {
  return apiRequest<OntologyLinks>(`${objectPath(type, key)}/links`, { token })
}

export async function getOntologyHistory(token: string, type: string, key: string) {
  const result = await apiRequest<{ executions: ERPActionExecution[] }>(`${objectPath(type, key)}/history`, { token })
  return result.executions ?? []
}

export function executeOntologyAction(token: string, type: string, key: string, action: string, data: Record<string, unknown>, idempotencyKey: string) {
  return apiRequest<{ object: OntologyObject; execution: ERPActionResult }>(`${objectPath(type, key)}/actions/${encodeURIComponent(action)}`, {
    method: 'POST', token, body: { data, idempotency_key: idempotencyKey },
  })
}

export function getBusinessRecord(token: string, table: string, key: string) {
  return apiRequest<ERPRecord>(`/erp/${encodeURIComponent(table)}/${encodeURIComponent(key)}`, { token })
}

export function ontologyRecord(object: OntologyObject, type: OntologyType): Record<string, unknown> & { key: string } {
  const data: Record<string, unknown> = {}
  for (const property of type.properties) {
    if (property.key in object.properties) data[property.source_field] = object.properties[property.key]
  }
  return { ...data, key: object.key }
}
