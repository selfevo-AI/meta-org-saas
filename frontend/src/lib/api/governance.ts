import { CreateFieldPermissionRuleInput, DataField, DataTable, FieldAccessCheckInput, FieldAccessCheckResult, FieldPermissionRule, SaveUserFieldPreferenceInput, UserFieldPreference } from './types'
import { apiRequest } from './core'
export async function listDataTables(token: string, category?: string): Promise<DataTable[]> {
  const query = category ? `?category=${encodeURIComponent(category)}` : ''
  return apiRequest<DataTable[]>(`/governance/data/tables${query}`, { token })
}

export async function listDataFields(token: string, tableName: string): Promise<DataField[]> {
  return apiRequest<DataField[]>(`/governance/data/tables/${encodeURIComponent(tableName)}/fields`, { token })
}

export async function getUserFieldPreference(token: string, tableName: string): Promise<UserFieldPreference> {
  return apiRequest<UserFieldPreference>(`/governance/data/field-preferences/${encodeURIComponent(tableName)}`, { token })
}

export async function saveUserFieldPreference(token: string, tableName: string, input: SaveUserFieldPreferenceInput): Promise<UserFieldPreference> {
  return apiRequest<UserFieldPreference>(`/governance/data/field-preferences/${encodeURIComponent(tableName)}`, {
    method: 'PUT',
    token,
    body: input,
  })
}

export async function createFieldPermissionRule(token: string, input: CreateFieldPermissionRuleInput): Promise<FieldPermissionRule> {
  return apiRequest<FieldPermissionRule>('/governance/data/field-permissions', {
    method: 'POST',
    token,
    body: input,
  })
}

export async function listFieldPermissionRules(token: string, tableName?: string): Promise<FieldPermissionRule[]> {
  const query = tableName ? `?table=${encodeURIComponent(tableName)}` : ''
  return apiRequest<FieldPermissionRule[]>(`/governance/data/field-permissions${query}`, { token })
}

export async function checkFieldAccess(token: string, input: FieldAccessCheckInput): Promise<FieldAccessCheckResult> {
  return apiRequest<FieldAccessCheckResult>('/governance/data/field-access/check', {
    method: 'POST',
    token,
    body: input,
  })
}
