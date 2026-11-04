import { DashboardOverview } from './types'
import { apiRequest } from './core'
export async function getDashboardOverview(token: string): Promise<DashboardOverview> {
  return apiRequest<DashboardOverview>('/dashboard/overview', { token })
}
