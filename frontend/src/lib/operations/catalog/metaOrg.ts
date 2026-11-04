import type { ApiOperation } from '../types'

export const metaOrgOperations: ApiOperation[] = [
  {
    id: 'meta-org-overview',
    domain: 'MetaOrg',
    title: 'Meta-Org 总览',
    method: 'GET',
    path: '/meta-org/overview',
  },
  {
    id: 'meta-org-inbox',
    domain: 'MetaOrg',
    title: 'Meta-Org 收件箱',
    method: 'GET',
    path: '/meta-org/inbox',
  },
]
