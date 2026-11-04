import { apiRequest } from './core'
import { ERPActionExecution, ERPActionResult, ERPRecord } from './types'
export async function listERPRecords<T extends Record<string, unknown>>(
  token: string,
  tableCode: string,
  limit = 100,
): Promise<Array<T & { key: string }>> {
  const result = await apiRequest<{ records: Array<ERPRecord<T>> }>(`/erp/${encodeURIComponent(tableCode)}?limit=${limit}`, { token })
  return (result.records ?? []).map((record) => ({ ...(record.data ?? ({} as T)), key: record.key }))
}

export async function listERPChildRecords<T extends Record<string, unknown>>(
  token: string,
  tableCode: string,
  key: string,
  childCode: string,
  limit = 100,
): Promise<Array<T & { key: string }>> {
  const result = await apiRequest<{ records: Array<ERPRecord<T>> }>(
    `/erp/${encodeURIComponent(tableCode)}/${encodeURIComponent(key)}/${encodeURIComponent(childCode)}?limit=${limit}`,
    { token },
  )
  return (result.records ?? []).map((record) => ({ ...(record.data ?? ({} as T)), key: record.key }))
}

export async function listERPActionExecutions(
  token: string,
  tableCode: string,
  key: string,
  limit = 50,
): Promise<ERPActionExecution[]> {
  const result = await apiRequest<{ action_executions: ERPActionExecution[] }>(
    `/erp/${encodeURIComponent(tableCode)}/${encodeURIComponent(key)}/action-executions?limit=${limit}`,
    { token },
  )
  return result.action_executions ?? []
}

export async function createERPRecord<T extends Record<string, unknown>>(
  token: string,
  tableCode: string,
  key: string,
  data: T,
): Promise<ERPRecord<T>> {
  return apiRequest<ERPRecord<T>>(`/erp/${encodeURIComponent(tableCode)}`, {
    method: 'POST',
    token,
    body: { key, data },
  })
}

export async function updateERPRecord<T extends Record<string, unknown>>(
  token: string,
  tableCode: string,
  key: string,
  data: T,
): Promise<ERPRecord<T>> {
  return apiRequest<ERPRecord<T>>(`/erp/${encodeURIComponent(tableCode)}/${encodeURIComponent(key)}`, {
    method: 'PATCH',
    token,
    body: { data },
  })
}

export async function deleteERPRecord(token: string, tableCode: string, key: string): Promise<void> {
  await apiRequest<void>(`/erp/${encodeURIComponent(tableCode)}/${encodeURIComponent(key)}`, {
    method: 'DELETE',
    token,
  })
}

export async function createERPChildRecord<T extends Record<string, unknown>>(
  token: string,
  tableCode: string,
  key: string,
  childCode: string,
  lineKey: string,
  data: T,
): Promise<ERPRecord<T>> {
  return apiRequest<ERPRecord<T>>(`/erp/${encodeURIComponent(tableCode)}/${encodeURIComponent(key)}/${encodeURIComponent(childCode)}`, {
    method: 'POST',
    token,
    body: { key: lineKey, data },
  })
}

export async function updateERPChildRecord<T extends Record<string, unknown>>(
  token: string,
  tableCode: string,
  key: string,
  childCode: string,
  lineKey: string,
  data: T,
): Promise<ERPRecord<T>> {
  return apiRequest<ERPRecord<T>>(
    `/erp/${encodeURIComponent(tableCode)}/${encodeURIComponent(key)}/${encodeURIComponent(childCode)}/${encodeURIComponent(lineKey)}`,
    {
      method: 'PATCH',
      token,
      body: { data },
    },
  )
}

export async function deleteERPChildRecord(
  token: string,
  tableCode: string,
  key: string,
  childCode: string,
  lineKey: string,
): Promise<void> {
  await apiRequest<void>(
    `/erp/${encodeURIComponent(tableCode)}/${encodeURIComponent(key)}/${encodeURIComponent(childCode)}/${encodeURIComponent(lineKey)}`,
    {
      method: 'DELETE',
      token,
    },
  )
}

export async function runERPAction<T extends Record<string, unknown>>(
  token: string,
  tableCode: string,
  key: string,
  action: string,
  data: Record<string, unknown> = {},
): Promise<ERPActionResult<T>> {
  return apiRequest<ERPActionResult<T>>(
    `/erp/${encodeURIComponent(tableCode)}/${encodeURIComponent(key)}/actions/${encodeURIComponent(action)}`,
    {
      method: 'POST',
      token,
      body: { data },
    },
  )
}
