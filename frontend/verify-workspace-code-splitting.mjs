import { readFileSync, statSync } from 'node:fs'
import { resolve } from 'node:path'
import vm from 'node:vm'

const root = process.cwd()
const pageSource = readFileSync(resolve(root, 'src/app/page.tsx'), 'utf8')
const dynamicModules = [
  './control-workspaces',
  './costing-workspace',
  './developer-tools-workspace',
  './erp-business-module-workspace',
  './erp-workspace',
  './meta-resource-workspace',
  './organization-workspace',
  './system-admin-workspace',
]

if (!pageSource.includes("import dynamic from 'next/dynamic'")) {
  throw new Error('page.tsx must use next/dynamic for domain workspaces')
}
for (const modulePath of dynamicModules) {
  if (!pageSource.includes(`import('${modulePath}')`)) {
    throw new Error(`page.tsx is missing dynamic workspace import ${modulePath}`)
  }
  const staticImportPattern = new RegExp(`from ['\"]${modulePath.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}['\"]`)
  if (staticImportPattern.test(pageSource)) {
    throw new Error(`page.tsx statically imports split workspace ${modulePath}`)
  }
}

const manifestPath = resolve(root, '.next/server/app/page_client-reference-manifest.js')
const context = { globalThis: {} }
vm.runInNewContext(readFileSync(manifestPath, 'utf8'), context)
const manifest = context.globalThis.__RSC_MANIFEST?.['/page']
const initialChunks = manifest?.entryJSFiles?.['[project]/src/app/page'] ?? []
if (initialChunks.length === 0) {
  throw new Error('page entry chunks were not found; run npm run build first')
}

const sizes = initialChunks.map((chunk) => ({
  chunk,
  bytes: statSync(resolve(root, '.next', chunk)).size,
}))
const totalBytes = sizes.reduce((total, item) => total + item.bytes, 0)
const largestBytes = Math.max(...sizes.map((item) => item.bytes))

if (totalBytes > 600_000) {
  throw new Error(`initial page JS is ${totalBytes} bytes, expected at most 600000`)
}
if (largestBytes > 250_000) {
  throw new Error(`largest initial page chunk is ${largestBytes} bytes, expected at most 250000`)
}

console.log(`Verified ${dynamicModules.length} lazy workspace modules; initial page JS ${totalBytes} bytes, largest chunk ${largestBytes} bytes.`)
