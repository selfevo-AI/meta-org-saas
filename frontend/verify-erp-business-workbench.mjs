import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const frontendRoot = fileURLToPath(new URL('.', import.meta.url))
const workspaceSource = readFileSync(`${frontendRoot}src/app/erp-business-module-workspace.tsx`, 'utf8')
const apiSource = readFileSync(`${frontendRoot}src/lib/api.ts`, 'utf8')
const i18nSource = readFileSync(`${frontendRoot}src/lib/i18n.tsx`, 'utf8')
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
  "t('erp.business.documentDetail')",
  "t('erp.business.unavailableActions')",
]

const requiredApiSnippets = [
  'export async function listERPChildRecords',
  '`/erp/${encodeURIComponent(tableCode)}/${encodeURIComponent(key)}/${encodeURIComponent(childCode)}?limit=${limit}`',
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
  const start = i18nSource.indexOf(`const ${name}: Record<string, string> = {`)
  const end = i18nSource.indexOf(nextName, start)
  if (start === -1 || end === -1) {
    throw new Error(`Could not locate ${name} dictionary`)
  }
  return i18nSource.slice(start, end)
}

function hasDictionaryKey(dictionary, key) {
  const quoted = key.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  return new RegExp(`(?:'${quoted}'|${quoted})\\s*:`).test(dictionary)
}

const enDictionary = dictionarySlice('en', 'const zh')
const zhDictionary = dictionarySlice('zh', 'const dictionaries')
const failures = []

const missingWorkspaceSnippets = requiredWorkspaceSnippets.filter((snippet) => !workspaceSource.includes(snippet))
if (missingWorkspaceSnippets.length > 0) {
  failures.push(`Missing ERP business workbench snippets:\n${missingWorkspaceSnippets.map((snippet) => `  - ${snippet}`).join('\n')}`)
}

const missingApiSnippets = requiredApiSnippets.filter((snippet) => !apiSource.includes(snippet))
if (missingApiSnippets.length > 0) {
  failures.push(`Missing ERP child row API snippets:\n${missingApiSnippets.map((snippet) => `  - ${snippet}`).join('\n')}`)
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
