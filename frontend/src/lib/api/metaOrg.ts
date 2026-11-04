import { InboxItem, MetaOrgOverview } from './types'
import { apiRequest } from './core'
export async function getMetaOrgOverview(token: string): Promise<MetaOrgOverview> {
  return apiRequest<MetaOrgOverview>('/meta-org/overview', { token })
}

export async function getMetaOrgInbox(token: string): Promise<InboxItem[]> {
  return apiRequest<InboxItem[]>('/meta-org/inbox', { token })
}
