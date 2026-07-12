import { existsSync, readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const root = fileURLToPath(new URL('.', import.meta.url))
const pageSource = readFileSync(`${root}src/app/page.tsx`, 'utf8')
const routeSource = readFileSync(`${root}src/lib/workspace-routes.ts`, 'utf8')
const platformPage = `${root}src/app/platform/[[...workspace]]/page.tsx`
const tenantPage = `${root}src/app/tenant/[organizationId]/[[...workspace]]/page.tsx`

const requiredPageSnippets = [
  "import { usePathname, useRouter } from 'next/navigation'",
  'parseWorkspacePath(pathname)',
  'workspacePath(sessionScope, currentOrganizationID',
  'router.push(target)',
  "router.replace('/')",
]

const requiredRouteSnippets = [
  "'PlatformAdmin:models': 'models'",
  "Project: 'projects'",
  "MetaOrg: 'meta-org'",
  "Dashboard: 'dashboard'",
  "ERP: 'erp'",
  "Organization: 'organization'",
  "Governance: 'governance'",
  "segments[0] === 'platform'",
  "segments[0] === 'tenant'",
  '`/platform/${slug || \'overview\'}`',
  '`/tenant/${encodeURIComponent(organizationId)}/${slug || \'overview\'}`',
]

const failures = []
if (!existsSync(platformPage)) failures.push('Missing platform catch-all workspace page')
if (!existsSync(tenantPage)) failures.push('Missing tenant organization catch-all workspace page')
for (const snippet of requiredPageSnippets) {
  if (!pageSource.includes(snippet)) failures.push(`Missing page routing contract: ${snippet}`)
}
for (const snippet of requiredRouteSnippets) {
  if (!routeSource.includes(snippet)) failures.push(`Missing workspace route mapping: ${snippet}`)
}

if (failures.length > 0) {
  console.error(failures.join('\n'))
  process.exit(1)
}

console.log('Verified platform and tenant workspace deep-link routing contracts.')
