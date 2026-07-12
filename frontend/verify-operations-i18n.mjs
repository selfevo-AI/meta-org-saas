import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const frontendRoot = fileURLToPath(new URL('.', import.meta.url))
const operationsSource = readFileSync(`${frontendRoot}src/lib/operations.ts`, 'utf8')
const i18nSource = readFileSync(`${frontendRoot}src/lib/i18n.tsx`, 'utf8')
const englishI18nSource = readFileSync(`${frontendRoot}src/lib/i18n.en.ts`, 'utf8')

const requiredRoutes = [
  ['POST', '/inventory/partners'],
  ['GET', '/inventory/partners'],
  ['POST', '/inventory/items'],
  ['GET', '/inventory/items'],
  ['POST', '/inventory/warehouses'],
  ['GET', '/inventory/warehouses'],
  ['POST', '/inventory/locations'],
  ['GET', '/inventory/balances'],
  ['POST', '/inventory/movements'],
  ['GET', '/inventory/movements'],
  ['POST', '/inventory/transfers'],
  ['GET', '/inventory/transfers'],
  ['POST', '/inventory/adjustments'],
  ['GET', '/inventory/adjustments'],
  ['POST', '/inventory/counts'],
  ['GET', '/inventory/counts'],
  ['POST', '/procurement/requisitions'],
  ['GET', '/procurement/requisitions'],
  ['POST', '/procurement/requisitions/{id}/submit'],
  ['POST', '/procurement/requisitions/{id}/approve'],
  ['POST', '/procurement/orders'],
  ['GET', '/procurement/orders'],
  ['POST', '/procurement/orders/{id}/submit'],
  ['POST', '/procurement/orders/{id}/approve'],
  ['POST', '/procurement/receipts'],
  ['GET', '/procurement/receipts'],
  ['POST', '/procurement/receipts/{id}/post'],
  ['POST', '/procurement/returns'],
  ['GET', '/procurement/returns'],
  ['POST', '/sales/quotations'],
  ['GET', '/sales/quotations'],
  ['POST', '/sales/orders'],
  ['GET', '/sales/orders'],
  ['POST', '/sales/orders/{id}/confirm'],
  ['POST', '/sales/orders/{id}/approve'],
  ['POST', '/sales/shipments'],
  ['GET', '/sales/shipments'],
  ['POST', '/sales/shipments/{id}/post'],
  ['POST', '/sales/returns'],
  ['GET', '/sales/returns'],
]

const operationPattern =
  /id:\s*'([^']+)'[\s\S]*?domain:\s*'([^']+)'[\s\S]*?title:\s*'([^']+)'[\s\S]*?method:\s*'([^']+)'[\s\S]*?path:\s*'([^']+)'/g
const operations = []
for (const match of operationsSource.matchAll(operationPattern)) {
  operations.push({
    id: match[1],
    domain: match[2],
    title: match[3],
    method: match[4],
    path: match[5],
  })
}

const operationRoutes = new Set(operations.map((operation) => `${operation.method} ${operation.path}`))
const missingRoutes = requiredRoutes.filter(([method, path]) => !operationRoutes.has(`${method} ${path}`))

const supplyChainOperations = operations.filter((operation) =>
  operation.path.startsWith('/inventory/') ||
  operation.path.startsWith('/procurement/') ||
  operation.path.startsWith('/sales/'),
)
const untranslatedSupplyChainTitles = supplyChainOperations.filter((operation) => !operation.title.startsWith('operation.'))

function dictionarySlice(name, nextName) {
  const source = name === 'en' ? englishI18nSource : i18nSource
  const start = source.indexOf(`const ${name}:`)
  const end = source.indexOf(nextName, start)
  if (start === -1 || end === -1) {
    throw new Error(`Could not locate ${name} dictionary`)
  }
  return source.slice(start, end)
}

const enDictionary = dictionarySlice('en', 'export default en')
const zhDictionary = dictionarySlice('zh', 'let englishDictionaryPromise')
const operationKeys = Array.from(
  new Set([...operationsSource.matchAll(/(?:title|label):\s*'(operation\.[^']+)'/g)].map((match) => match[1])),
)

function hasKey(dictionary, key) {
  const escaped = key.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  return new RegExp(`'${escaped}'\\s*:`).test(dictionary)
}

const missingEnKeys = operationKeys.filter((key) => !hasKey(enDictionary, key))
const missingZhKeys = operationKeys.filter((key) => !hasKey(zhDictionary, key))

const failures = []
if (missingRoutes.length > 0) {
  failures.push(`Missing supply-chain operations:\n${missingRoutes.map(([method, path]) => `  - ${method} ${path}`).join('\n')}`)
}
if (untranslatedSupplyChainTitles.length > 0) {
  failures.push(
    `Supply-chain operation titles must use operation.* keys:\n${untranslatedSupplyChainTitles
      .map((operation) => `  - ${operation.id}: ${operation.title}`)
      .join('\n')}`,
  )
}
if (missingEnKeys.length > 0) {
  failures.push(`Missing English operation i18n keys:\n${missingEnKeys.map((key) => `  - ${key}`).join('\n')}`)
}
if (missingZhKeys.length > 0) {
  failures.push(`Missing Chinese operation i18n keys:\n${missingZhKeys.map((key) => `  - ${key}`).join('\n')}`)
}

if (failures.length > 0) {
  console.error(failures.join('\n\n'))
  process.exit(1)
}

console.log(`Verified ${requiredRoutes.length} supply-chain routes and ${operationKeys.length} operation i18n keys.`)
