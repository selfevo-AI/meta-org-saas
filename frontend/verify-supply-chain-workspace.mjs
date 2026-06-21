import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const frontendRoot = fileURLToPath(new URL('.', import.meta.url))
const pageSource = readFileSync(`${frontendRoot}src/app/page.tsx`, 'utf8')
const procurementSource = readFileSync(`${frontendRoot}src/app/procurement-workspace.tsx`, 'utf8')
const salesSource = readFileSync(`${frontendRoot}src/app/sales-workspace.tsx`, 'utf8')
const inventorySource = readFileSync(`${frontendRoot}src/app/inventory-workspace.tsx`, 'utf8')
const uiSource = readFileSync(`${frontendRoot}src/app/supply-chain-ui.tsx`, 'utf8')
const apiSource = readFileSync(`${frontendRoot}src/lib/api.ts`, 'utf8')
const i18nSource = readFileSync(`${frontendRoot}src/lib/i18n.tsx`, 'utf8')

const requiredPageSnippets = [
  'SupplyChainFunctionID',
  'supplyChainFunctionGroups',
  'activeSupplyChainFunction',
  'targetTypes: [',
  'loadBusinessTreeNodes(token, activeDomain, activeSupplyChainFunction)',
  'currentSupplyChainFunctionID',
  'onSupplyChainFunctionChange',
  'externalSelection={activeBusinessSelection}',
  "setWorkspaceView('overview')",
  "getPlatformMetaOrgOverview",
  "getPlatformMetaOrgInbox",
]

const requiredWorkspaceSnippets = [
  {
    name: 'procurement',
    source: procurementSource,
    snippets: [
      'externalSelection?: SupplyChainSelection',
      'SupplyChainDocumentDetail',
      'useEffect(() => {',
      'setActiveTab',
      'selectedDocument',
      'detailTitle',
      'lineColumns',
    ],
  },
  {
    name: 'sales',
    source: salesSource,
    snippets: [
      'externalSelection?: SupplyChainSelection',
      'SupplyChainDocumentDetail',
      'useEffect(() => {',
      'setActiveTab',
      'selectedDocument',
      'detailTitle',
      'lineColumns',
    ],
  },
  {
    name: 'inventory',
    source: inventorySource,
    snippets: [
      'externalSelection?: SupplyChainSelection',
      'SupplyChainDocumentDetail',
      'useEffect(() => {',
      'setActiveTab',
      'selectedDocument',
      'detailTitle',
      'lineColumns',
    ],
  },
]

const requiredUISnippets = [
  'export type SupplyChainSelection',
  'export interface SupplyChainDocumentDetailProps',
  'export function SupplyChainDocumentDetail',
  'mainFields',
  'lineColumns',
  'lineRows',
  "t('supplyChain.documentDetail')",
  "t('supplyChain.noLineItems')",
]

const requiredApiSnippets = [
  'export async function getPlatformMetaOrgOverview',
  'export async function getPlatformMetaOrgInbox',
  "'/platform/admin/meta-org/overview'",
  "'/platform/admin/meta-org/inbox'",
]

const requiredI18nKeys = [
  'supplyChain.documentDetail',
  'supplyChain.lineItems',
  'supplyChain.noLineItems',
  'supplyChain.selectedDocument',
  'businessTree.type.inventory_transfer',
  'businessTree.type.inventory_adjustment',
  'businessTree.type.inventory_count',
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

const missingPageSnippets = requiredPageSnippets.filter((snippet) => !pageSource.includes(snippet))
if (missingPageSnippets.length > 0) {
  failures.push(`Missing supply chain page integration snippets:\n${missingPageSnippets.map((snippet) => `  - ${snippet}`).join('\n')}`)
}

for (const workspace of requiredWorkspaceSnippets) {
  const missing = workspace.snippets.filter((snippet) => !workspace.source.includes(snippet))
  if (missing.length > 0) {
    failures.push(`Missing ${workspace.name} document detail snippets:\n${missing.map((snippet) => `  - ${snippet}`).join('\n')}`)
  }
}

const missingUISnippets = requiredUISnippets.filter((snippet) => !uiSource.includes(snippet))
if (missingUISnippets.length > 0) {
  failures.push(`Missing reusable supply chain detail UI snippets:\n${missingUISnippets.map((snippet) => `  - ${snippet}`).join('\n')}`)
}

const missingApiSnippets = requiredApiSnippets.filter((snippet) => !apiSource.includes(snippet))
if (missingApiSnippets.length > 0) {
  failures.push(`Missing platform overview API snippets:\n${missingApiSnippets.map((snippet) => `  - ${snippet}`).join('\n')}`)
}

const missingEnKeys = requiredI18nKeys.filter((key) => !hasDictionaryKey(enDictionary, key))
if (missingEnKeys.length > 0) {
  failures.push(`Missing English supply chain i18n keys:\n${missingEnKeys.map((key) => `  - ${key}`).join('\n')}`)
}

const missingZhKeys = requiredI18nKeys.filter((key) => !hasDictionaryKey(zhDictionary, key))
if (missingZhKeys.length > 0) {
  failures.push(`Missing Chinese supply chain i18n keys:\n${missingZhKeys.map((key) => `  - ${key}`).join('\n')}`)
}

if (failures.length > 0) {
  console.error(failures.join('\n\n'))
  process.exit(1)
}

console.log('Verified supply chain submenu, document detail, and platform overview integration.')
