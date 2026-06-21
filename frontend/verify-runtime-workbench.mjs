import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const frontendRoot = fileURLToPath(new URL('.', import.meta.url))
const apiSource = readFileSync(`${frontendRoot}src/lib/api.ts`, 'utf8')
const workbenchSource = readFileSync(`${frontendRoot}src/app/api-workbench.tsx`, 'utf8')

const requiredApiSnippets = [
  "import type { ApiOperation } from './operations'",
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
