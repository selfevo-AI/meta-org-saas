import { Role } from './types'
import { apiRequest } from './core'
export async function listRoles(): Promise<Role[]> {
  return apiRequest<Role[]>('/roles')
}
