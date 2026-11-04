import { apiRequest } from './core'
import type { ApiOperation } from '@/lib/operations'
export async function listRuntimeOperations(token: string): Promise<ApiOperation[]> {
  return apiRequest<ApiOperation[]>('/runtime/operations', { token })
}
