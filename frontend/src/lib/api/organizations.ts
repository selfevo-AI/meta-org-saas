import { CreateOrganizationInvitationInput, OrganizationIndustryAdoption, OrganizationInvitation, OrganizationSubscription } from './types'
import { apiRequest } from './core'
export async function getOrganizationIndustry(token: string, organizationID: string): Promise<OrganizationIndustryAdoption> {
  return apiRequest<OrganizationIndustryAdoption>(`/organizations/${encodeURIComponent(organizationID)}/industry`, {
    token,
    organizationId: organizationID,
  })
}

export async function getOrganizationSubscription(token: string, organizationID: string): Promise<OrganizationSubscription> {
  return apiRequest<OrganizationSubscription>(`/organizations/${encodeURIComponent(organizationID)}/subscription`, {
    token,
    organizationId: organizationID,
  })
}

export async function getOrganizationEntitlements(token: string, organizationID: string): Promise<Record<string, boolean>> {
  return apiRequest<Record<string, boolean>>(`/organizations/${encodeURIComponent(organizationID)}/entitlements`, {
    token,
    organizationId: organizationID,
  })
}

export async function updateOrganizationModules(
  token: string,
  organizationID: string,
  enabledModules: string[],
): Promise<Record<string, boolean>> {
  return apiRequest<Record<string, boolean>>(`/organizations/${encodeURIComponent(organizationID)}/modules`, {
    method: 'PATCH',
    token,
    organizationId: organizationID,
    body: { enabled_modules: enabledModules },
  })
}

export async function listOrganizationInvitations(token: string, organizationID: string, limit = 100): Promise<OrganizationInvitation[]> {
  const query = limit > 0 ? `?limit=${encodeURIComponent(String(limit))}` : ''
  return apiRequest<OrganizationInvitation[]>(`/organizations/${encodeURIComponent(organizationID)}/invitations${query}`, {
    token,
    organizationId: organizationID,
  })
}

export async function createOrganizationInvitation(
  token: string,
  organizationID: string,
  input: CreateOrganizationInvitationInput,
): Promise<OrganizationInvitation> {
  return apiRequest<OrganizationInvitation>(`/organizations/${encodeURIComponent(organizationID)}/invitations`, {
    method: 'POST',
    token,
    organizationId: organizationID,
    body: input,
  })
}
