import type { ApiOperation } from '../types'

export const costOperations: ApiOperation[] = [
  {
    id: 'cost-create',
    domain: 'Cost',
    title: '写入成本',
    method: 'POST',
    path: '/projects/{id}/cost-entries',
    pathParams: [{ name: 'id', label: '项目 ID' }],
    bodyTemplate: {
      source_type: 'manual',
      entry_actor_id: null,
      entry_actor_type: 'internal_human',
      amount: 1000,
      currency: 'CNY',
      description: 'manual cost entry',
      metadata: {},
    },
  },
  {
    id: 'cost-list',
    domain: 'Cost',
    title: '成本明细',
    method: 'GET',
    path: '/projects/{id}/cost-entries',
    pathParams: [{ name: 'id', label: '项目 ID' }],
  },
  {
    id: 'cost-summary',
    domain: 'Cost',
    title: '成本汇总',
    method: 'GET',
    path: '/projects/{id}/cost-summary',
    pathParams: [{ name: 'id', label: '项目 ID' }],
  },
  {
    id: 'cost-refresh',
    domain: 'Cost',
    title: '刷新成员成本',
    method: 'POST',
    path: '/projects/{id}/cost-refresh',
    pathParams: [{ name: 'id', label: '项目 ID' }],
    bodyTemplate: {},
  },
]
