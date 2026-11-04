import { CreateIndustryExtensionInput, IndustryExtension, IndustryPublicationRequest } from './types'
import { apiRequest } from './core'
export async function listIndustryExtensions(token: string, organizationID: string, limit = 100): Promise<IndustryExtension[]> {
  const params = new URLSearchParams({ organization_id: organizationID })
  if (limit > 0) params.set('limit', String(limit))
  return apiRequest<IndustryExtension[]>(`/industry/extensions?${params.toString()}`, {
    token,
    organizationId: organizationID,
  })
}

export async function createIndustryExtension(token: string, input: CreateIndustryExtensionInput): Promise<IndustryExtension> {
  return apiRequest<IndustryExtension>('/industry/extensions', {
    method: 'POST',
    token,
    organizationId: input.organization_id,
    body: input,
  })
}

export async function submitIndustryExtensionPublication(token: string, extensionID: string, reason = ''): Promise<IndustryPublicationRequest> {
  return apiRequest<IndustryPublicationRequest>(`/industry/extensions/${encodeURIComponent(extensionID)}/submit-publication`, {
    method: 'POST',
    token,
    body: { reason },
  })
}
