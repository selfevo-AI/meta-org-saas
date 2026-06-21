import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const frontendRoot = fileURLToPath(new URL('.', import.meta.url))
const apiSource = readFileSync(`${frontendRoot}src/lib/api.ts`, 'utf8')
const workbenchSource = readFileSync(`${frontendRoot}src/app/api-workbench.tsx`, 'utf8')

const requiredApiSnippets = [
  "import type { ApiOperation } from './operations'",
  'export async function listRuntimeOperations',
  "'/runtime/operations'",
]

const requiredWorkbenchSnippets = [
  'listRuntimeOperations',
  'const [runtimeOperations, setRuntimeOperations]',
  'const operationCatalog = runtimeOperations.length > 0 ? runtimeOperations : apiOperations',
  'operationCatalog.filter',
  'operationDomainsForCatalog',
  'formatRuntimeDomainLabel',
  'saas.module.${domain}',
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
