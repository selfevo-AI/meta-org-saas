import { CreateMetaResourceInput, MetaResource, MetaResourceSummary } from './types'
import { apiRequest } from './core'
export async function listMetaResources(token: string, filter: { resource_type?: string; status?: string } = {}): Promise<MetaResource[]> {
  const params = new URLSearchParams()
  if (filter.resource_type) params.set('resource_type', filter.resource_type)
  if (filter.status) params.set('status', filter.status)
  const query = params.toString() ? `?${params.toString()}` : ''
  return apiRequest<MetaResource[]>(`/meta-resources${query}`, { token })
}

export async function createMetaResource(token: string, input: CreateMetaResourceInput): Promise<MetaResource> {
  return apiRequest<MetaResource>('/meta-resources', { method: 'POST', token, body: input })
}

export async function syncExistingMetaResources(token: string): Promise<Record<string, number>> {
  return apiRequest<Record<string, number>>('/meta-resources/sync-existing', { method: 'POST', token })
}

export async function getMetaResourceSummary(token: string): Promise<MetaResourceSummary> {
  return apiRequest<MetaResourceSummary>('/meta-resources/summary', { token })
}
