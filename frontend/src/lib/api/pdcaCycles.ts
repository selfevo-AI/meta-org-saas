import { CreatePDCACycleInput, PDCACycle } from './types'
import { apiRequest } from './core'
export async function listPDCACycles(token: string): Promise<PDCACycle[]> {
  return apiRequest<PDCACycle[]>('/pdca-cycles', { token })
}

export async function createPDCACycle(token: string, input: CreatePDCACycleInput): Promise<PDCACycle> {
  return apiRequest<PDCACycle>('/pdca-cycles', { method: 'POST', token, body: input })
}
