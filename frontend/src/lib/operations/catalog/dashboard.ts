import type { ApiOperation } from '../types'

export const dashboardOperations: ApiOperation[] = [
  {
    id: 'system-health',
    domain: 'Dashboard',
    title: '健康检查',
    method: 'GET',
    path: '/health',
    auth: false,
  },
  {
    id: 'dashboard-overview',
    domain: 'Dashboard',
    title: '系统总览',
    method: 'GET',
    path: '/dashboard/overview',
  },
]
