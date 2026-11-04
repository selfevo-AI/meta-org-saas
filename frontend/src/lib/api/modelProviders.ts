import { ModelProvider } from './types'
import { apiRequest } from './core'
export async function listModelProviders(token: string): Promise<ModelProvider[]> {
  return apiRequest<ModelProvider[]>('/model-providers', { token })
}
