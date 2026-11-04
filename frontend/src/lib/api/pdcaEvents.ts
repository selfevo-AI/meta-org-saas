import { CreatePDCAEventInput, PDCAEvent } from './types'
import { apiRequest } from './core'
export async function listPDCAEvents(token: string, cycleID?: string): Promise<PDCAEvent[]> {
  const query = cycleID ? `?cycle_id=${encodeURIComponent(cycleID)}` : ''
  return apiRequest<PDCAEvent[]>(`/pdca-events${query}`, { token })
}

export async function createPDCAEvent(token: string, input: CreatePDCAEventInput): Promise<PDCAEvent> {
  return apiRequest<PDCAEvent>('/pdca-events', { method: 'POST', token, body: input })
}
