import { readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'

const frontendRoot = fileURLToPath(new URL('.', import.meta.url))
// api.ts was split into a directory of per-domain modules.
function readApiSource() {
  return readdirSync(`${frontendRoot}src/lib/api`, { recursive: true })
    .filter((entry) => String(entry).endsWith('.ts'))
    .sort()
    .map((entry) => readFileSync(join(`${frontendRoot}src/lib/api`, String(entry)), 'utf8'))
    .join('\n')
}

const apiSource = readApiSource()
const workbenchSource = readFileSync(`${frontendRoot}src/app/api-workbench.tsx`, 'utf8')

const requiredApiSnippets = [
  "import type { ApiOperation } from '@/lib/operations'",
  'export async function listRuntimeOperations',
  'export async function listPlatformRuntimeOperations',
  "'/runtime/operations'",
  "'/platform/admin/runtime/operations'",
]

const requiredWorkbenchSnippets = [
  'listRuntimeOperations',
  'listPlatformRuntimeOperations',
  "apiScope?: 'tenant' | 'platform'",
  "apiScope === 'platform' ? listPlatformRuntimeOperations : listRuntimeOperations",
  'platformOperationDomains',
  'platformOperationAvailable',
  'const [runtimeOperations, setRuntimeOperations]',
  'const operationCatalog = scopedRuntimeOperations.length > 0 ? scopedRuntimeOperations : scopedFallbackOperations',
  'operationCatalog.filter',
  'operationDomainsForCatalog',
  'formatRuntimeDomainLabel',
  'saas.module.${domain}',
  'apiScope={apiScope}',
]

const failures = []

const missingApiSnippets = requiredApiSnippets.filter((snippet) => !apiSource.includes(snippet))
if (missingApiSnippets.length > 0) {
  failures.push(`Missing runtime operation API snippets:\n${missingApiSnippets.map((snippet) => `  - ${snippet}`).join('\n')}`)
}

const missingWorkbenchSnippets = requiredWorkbenchSnippets.filter((snippet) => !workbenchSource.includes(snippet))
if (missingWorkbenchSnippets.length > 0) {
  failures.push(`Missing runtime workbench integration snippets:\n${missingWorkbenchSnippets.map((snippet) => `  - ${snippet}`).join('\n')}`)
}

if (failures.length > 0) {
  console.error(failures.join('\n\n'))
  process.exit(1)
}

console.log('Verified runtime API Workbench operation catalog integration.')
