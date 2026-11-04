import { Industry } from './types'
import { apiRequest } from './core'
export async function listIndustries(token: string): Promise<Industry[]> {
  return apiRequest<Industry[]>('/industries/catalog', { token })
}
