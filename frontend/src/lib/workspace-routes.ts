import type { SessionScope } from './auth'

export type WorkspaceView = 'overview' | `domain:${string}`

export interface WorkspaceRoute {
  scope: SessionScope
  organizationId: string | null
  view: WorkspaceView
}

const tenantSlugByDomain: Record<string, string> = {
  MetaOrg: 'meta-org',
  Dashboard: 'dashboard',
  ERP: 'erp',
  Organization: 'organization',
  MetaResource: 'meta-resources',
  Governance: 'governance',
  Evolution: 'evolution',
  Capability: 'capability',
  Workflow: 'workflow',
  Requirement: 'requirements',
  Project: 'projects',
  Delivery: 'delivery',
  Cost: 'project-cost',
  Feedback: 'feedback',
  Finance: 'finance',
  Costing: 'costing',
  FinanceAccounting: 'finance-accounting',
  FinanceReceivables: 'receivables',
  FinancePayables: 'payables',
  FinanceCostAccounting: 'cost-accounting',
  Inventory: 'inventory',
  Procurement: 'procurement',
  Sales: 'sales',
  Retail: 'retail',
  Manufacturing: 'manufacturing',
}

const platformSlugByDomain: Record<string, string> = {
  'PlatformAdmin:assistant': 'assistant',
  'PlatformAdmin:monitoring': 'monitoring',
  'PlatformAdmin:saas': 'organizations',
  'PlatformAdmin:industry': 'industry-solutions',
  'PlatformAdmin:features': 'features',
  'PlatformAdmin:permissions': 'permissions',
  'PlatformAdmin:users': 'users',
  'PlatformAdmin:database': 'database',
  'PlatformAdmin:models': 'models',
  'PlatformAdmin:runtime': 'runtime',
  'PlatformAdmin:catalog': 'catalog',
  Capability: 'capability',
  Governance: 'governance',
  Evolution: 'evolution',
  Verification: 'verification',
  Identity: 'identity',
  Layer: 'layer',
  Observability: 'observability',
  SystemAdmin: 'system-admin',
  DeveloperTools: 'developer-tools',
}

const tenantDomainBySlug = invert(tenantSlugByDomain)
const platformDomainBySlug = invert(platformSlugByDomain)

export function parseWorkspacePath(pathname: string): WorkspaceRoute | null {
  const segments = pathname.split('/').filter(Boolean).map(decodeURIComponent)
  if (segments[0] === 'platform') {
    const slug = segments[1] || 'overview'
    const domain = platformDomainBySlug[slug]
    return domain || slug === 'overview'
      ? { scope: 'platform', organizationId: null, view: domain ? `domain:${domain}` : 'overview' }
      : null
  }
  if (segments[0] === 'tenant' && segments[1]) {
    const slug = segments[2] || 'overview'
    const domain = tenantDomainBySlug[slug]
    return domain || slug === 'overview'
      ? { scope: 'tenant', organizationId: segments[1], view: domain ? `domain:${domain}` : 'overview' }
      : null
  }
  return null
}

export function workspacePath(scope: SessionScope, organizationId: string | null, view: WorkspaceView): string {
  const domain = view === 'overview' ? null : view.replace('domain:', '')
  if (scope === 'platform') {
    const slug = domain ? platformSlugByDomain[domain] : 'overview'
    return `/platform/${slug || 'overview'}`
  }
  if (!organizationId) return '/'
  const slug = domain ? tenantSlugByDomain[domain] : 'overview'
  return `/tenant/${encodeURIComponent(organizationId)}/${slug || 'overview'}`
}

function invert(values: Record<string, string>): Record<string, string> {
  return Object.fromEntries(Object.entries(values).map(([key, value]) => [value, key]))
}
