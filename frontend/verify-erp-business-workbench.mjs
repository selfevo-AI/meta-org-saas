import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const frontendRoot = fileURLToPath(new URL('.', import.meta.url))
const workspaceSource = readFileSync(`${frontendRoot}src/app/erp-business-module-workspace.tsx`, 'utf8')
const apiSource = readFileSync(`${frontendRoot}src/lib/api.ts`, 'utf8')
const operationsSource = readFileSync(`${frontendRoot}src/lib/operations.ts`, 'utf8')
const pageSource = readFileSync(`${frontendRoot}src/app/page.tsx`, 'utf8')
const i18nSource = readFileSync(`${frontendRoot}src/lib/i18n.tsx`, 'utf8')
const englishI18nSource = readFileSync(`${frontendRoot}src/lib/i18n.en.ts`, 'utf8')
const packageSource = readFileSync(`${frontendRoot}package.json`, 'utf8')

const requiredWorkspaceSnippets = [
  'type ERPActionAvailability',
  'listERPChildRecords',
  'childRows',
  'actionResult',
  'availableActions',
  'blockedActions',
  'businessTimeline',
  'assistantProposals',
  'generatedRecords',
  'isERPActionAvailable',
  'ERPDocumentDetail',
  'ERPDocumentTimeline',
  'listRuntimeOperations',
  'listERPActionExecutions',
  'deriveRuntimeDocuments',
  'runtimeDocuments',
  'metadata?.workspace',
  'actionExecutions',
  "t('erp.business.documentDetail')",
  "t('erp.business.unavailableActions')",
]

const requiredApiSnippets = [
  'export async function listERPChildRecords',
  'export async function listERPActionExecutions',
  '`/erp/${encodeURIComponent(tableCode)}/${encodeURIComponent(key)}/${encodeURIComponent(childCode)}?limit=${limit}`',
  '`/erp/${encodeURIComponent(tableCode)}/${encodeURIComponent(key)}/action-executions?limit=${limit}`',
]

const requiredOperationsSnippets = [
  'metadata?: Record<string, unknown>',
]

const forbiddenPageSnippets = [
  "from './project-lifecycle-workspace'",
  "from './procurement-workspace'",
  "from './sales-workspace'",
  "from './inventory-workspace'",
  "from './finance-workspace'",
]

const requiredPackageSnippets = ['"test:erp-business": "node verify-erp-business-workbench.mjs"']

const requiredI18nKeys = [
  'erp.business.documentDetail',
  'erp.business.availableActions',
  'erp.business.unavailableActions',
  'erp.business.timeline',
  'erp.business.generatedRecords',
  'erp.business.assistantProposals',
  'erp.business.noTimeline',
  'erp.business.noChildRows',
  'erp.business.childRows',
  'erp.business.costImpact',
  'erp.business.relatedProject',
  'erp.business.statusReason',
  'erp.business.actionBlocked',
  'erp.business.actionCompleted',
]

function dictionarySlice(name, nextName) {
  const source = name === 'en' ? englishI18nSource : i18nSource
  const start = source.indexOf(`const ${name}:`)
  const end = source.indexOf(nextName, start)
  if (start === -1 || end === -1) {
    throw new Error(`Could not locate ${name} dictionary`)
  }
  return source.slice(start, end)
}

function hasDictionaryKey(dictionary, key) {
  const quoted = key.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  return new RegExp(`(?:'${quoted}'|${quoted})\\s*:`).test(dictionary)
}

const enDictionary = dictionarySlice('en', 'export default en')
const zhDictionary = dictionarySlice('zh', 'let englishDictionaryPromise')
const failures = []

const missingWorkspaceSnippets = requiredWorkspaceSnippets.filter((snippet) => !workspaceSource.includes(snippet))
if (missingWorkspaceSnippets.length > 0) {
  failures.push(`Missing ERP business workbench snippets:\n${missingWorkspaceSnippets.map((snippet) => `  - ${snippet}`).join('\n')}`)
}

const missingApiSnippets = requiredApiSnippets.filter((snippet) => !apiSource.includes(snippet))
if (missingApiSnippets.length > 0) {
  failures.push(`Missing ERP child row API snippets:\n${missingApiSnippets.map((snippet) => `  - ${snippet}`).join('\n')}`)
}

const missingOperationsSnippets = requiredOperationsSnippets.filter((snippet) => !operationsSource.includes(snippet))
if (missingOperationsSnippets.length > 0) {
  failures.push(`Missing runtime operation metadata snippets:\n${missingOperationsSnippets.map((snippet) => `  - ${snippet}`).join('\n')}`)
}

const linkedLegacyWorkspaces = forbiddenPageSnippets.filter((snippet) => pageSource.includes(snippet))
if (linkedLegacyWorkspaces.length > 0) {
  failures.push(`Legacy semantic workspaces are still linked from page.tsx:\n${linkedLegacyWorkspaces.map((snippet) => `  - ${snippet}`).join('\n')}`)
}

const missingPackageSnippets = requiredPackageSnippets.filter((snippet) => !packageSource.includes(snippet))
if (missingPackageSnippets.length > 0) {
  failures.push(`Missing package script snippets:\n${missingPackageSnippets.map((snippet) => `  - ${snippet}`).join('\n')}`)
}

const missingEnKeys = requiredI18nKeys.filter((key) => !hasDictionaryKey(enDictionary, key))
if (missingEnKeys.length > 0) {
  failures.push(`Missing English ERP business i18n keys:\n${missingEnKeys.map((key) => `  - ${key}`).join('\n')}`)
}

const missingZhKeys = requiredI18nKeys.filter((key) => !hasDictionaryKey(zhDictionary, key))
if (missingZhKeys.length > 0) {
  failures.push(`Missing Chinese ERP business i18n keys:\n${missingZhKeys.map((key) => `  - ${key}`).join('\n')}`)
}

if (failures.length > 0) {
  console.error(failures.join('\n\n'))
  process.exit(1)
}

console.log('Verified ERP business workbench document detail, state-gated actions, child rows, generated records, and assistant proposal timeline.')
