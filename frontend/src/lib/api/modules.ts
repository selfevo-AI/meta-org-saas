import { SaaSModule } from './types'
import { apiRequest } from './core'
import type { SessionScope } from '@/lib/auth'
export async function listSaaSModules(token: string, scope: SessionScope = 'tenant'): Promise<SaaSModule[]> {
  return apiRequest<SaaSModule[]>('/modules', { token, scope })
}
