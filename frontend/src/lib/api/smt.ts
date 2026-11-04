import { SMTEdgeEvent, SMTEdgeInstance, SMTMachineContext } from './types'
import { apiRequest } from './core'
export async function getSMTMachineContext(token: string, machineCode: string): Promise<SMTMachineContext> {
  return apiRequest<SMTMachineContext>(`/smt/machines/${encodeURIComponent(machineCode)}/context`, { token })
}

export async function getSMTBoard(token: string, lineCode = ''): Promise<Record<string, unknown>> {
  const query = lineCode ? `?line_code=${encodeURIComponent(lineCode)}` : ''
  return apiRequest<Record<string, unknown>>(`/smt/board${query}`, { token })
}

export async function getSMTWorkOrderProgress(token: string, workOrder: string): Promise<Record<string, unknown>> {
  return apiRequest<Record<string, unknown>>(`/smt/work-orders/${encodeURIComponent(workOrder)}/progress`, { token })
}

export async function getSMTTraceSN(token: string, sn: string): Promise<Record<string, unknown>> {
  return apiRequest<Record<string, unknown>>(`/smt/trace/sn/${encodeURIComponent(sn)}`, { token })
}

export async function getSMTReelUnits(token: string, reelCode: string): Promise<Record<string, unknown>> {
  return apiRequest<Record<string, unknown>>(`/smt/trace/reels/${encodeURIComponent(reelCode)}/units`, { token })
}

export async function getSMTReport(
  token: string,
  kind: 'production' | 'andon' | 'downtime' | 'materials',
  filters: { from?: string; to?: string; line_code?: string } = {},
): Promise<Record<string, unknown>> {
  const params = new URLSearchParams()
  if (filters.from) params.set('from', filters.from)
  if (filters.to) params.set('to', filters.to)
  if (filters.line_code) params.set('line_code', filters.line_code)
  const query = params.toString()
  return apiRequest<Record<string, unknown>>(`/smt/reports/${kind}${query ? `?${query}` : ''}`, { token })
}

export async function getSMTSettings(token: string): Promise<{ values: Record<string, string>; stored: Record<string, string> }> {
  return apiRequest(`/smt/settings`, { token })
}

export async function updateSMTSettings(token: string, values: Record<string, string>): Promise<Record<string, unknown>> {
  return apiRequest(`/smt/settings`, { method: 'PATCH', token, body: { values } })
}

export async function listSMTEdges(token: string): Promise<SMTEdgeInstance[]> {
  const result = await apiRequest<{ instances: SMTEdgeInstance[] }>(`/smt/edges`, { token })
  return result.instances ?? []
}

export async function createSMTEdge(token: string, input: {
  name: string; base_url: string; api_key: string; webhook_secret?: string; subscribed_events?: string; auto_push?: boolean
}): Promise<SMTEdgeInstance> {
  return apiRequest<SMTEdgeInstance>(`/smt/edges`, { method: 'POST', token, body: input })
}

export async function updateSMTEdge(token: string, instanceKey: string, input: Record<string, unknown>): Promise<SMTEdgeInstance> {
  return apiRequest<SMTEdgeInstance>(`/smt/edges/${encodeURIComponent(instanceKey)}`, { method: 'PATCH', token, body: input })
}

export async function deleteSMTEdge(token: string, instanceKey: string): Promise<void> {
  await apiRequest<void>(`/smt/edges/${encodeURIComponent(instanceKey)}`, { method: 'DELETE', token })
}

export async function listSMTEdgeEvents(token: string, instanceKey: string, limit = 50): Promise<SMTEdgeEvent[]> {
  const result = await apiRequest<{ events: SMTEdgeEvent[] }>(
    `/smt/edges/${encodeURIComponent(instanceKey)}/events?limit=${limit}`,
    { token },
  )
  return result.events ?? []
}

export async function pushSMTEdge(token: string, instanceKey: string, scopes: string[] = []): Promise<Record<string, unknown>> {
  return apiRequest<Record<string, unknown>>(`/smt/edges/${encodeURIComponent(instanceKey)}/push`, {
    method: 'POST',
    token,
    body: { scopes },
  })
}

export async function pullSMTEdge(token: string, instanceKey: string): Promise<Record<string, unknown>> {
  return apiRequest<Record<string, unknown>>(`/smt/edges/${encodeURIComponent(instanceKey)}/pull`, {
    method: 'POST',
    token,
    body: {},
  })
}
