import { CreateDemandProfileInput, DemandProfile } from './types'
import { apiRequest } from './core'
export async function listDemandProfiles(token: string): Promise<DemandProfile[]> {
  return apiRequest<DemandProfile[]>('/demand-profiles', { token })
}

export async function createDemandProfile(token: string, input: CreateDemandProfileInput): Promise<DemandProfile> {
  return apiRequest<DemandProfile>('/demand-profiles', { method: 'POST', token, body: input })
}
