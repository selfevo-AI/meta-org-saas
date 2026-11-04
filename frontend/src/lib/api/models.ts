import { ModelCatalogItem } from './types'
import { apiRequest } from './core'
export async function listModels(token: string): Promise<ModelCatalogItem[]> {
  return apiRequest<ModelCatalogItem[]>('/models', { token })
}
