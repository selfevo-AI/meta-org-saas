import { readdirSync, readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import vm from 'node:vm'

const root = process.cwd()
const providerPath = resolve(root, 'src/lib/i18n.tsx')
const englishPath = resolve(root, 'src/lib/i18n.en.ts')
const providerSource = readFileSync(providerPath, 'utf8')
const englishSource = readFileSync(englishPath, 'utf8')

if (!providerSource.includes("import('./i18n.en')")) {
  throw new Error('LanguageProvider must dynamically import the English dictionary')
}
if (providerSource.includes('const en:')) {
  throw new Error('English translations must not remain in the default i18n module')
}

function readDictionary(source, startMarker, endMarker) {
  const start = source.indexOf(startMarker)
  const end = source.indexOf(endMarker, start)
  if (start < 0 || end <= start) {
    throw new Error(`dictionary markers not found: ${startMarker}`)
  }
  const open = source.indexOf('{', start)
  const close = source.lastIndexOf('}', end)
  return vm.runInNewContext(`(${source.slice(open, close + 1)})`)
}

const zh = readDictionary(providerSource, 'const zh: Dictionary = {', 'let englishDictionaryPromise')
const en = readDictionary(englishSource, 'const en: Record<string, string> = {', 'export default en')
const missingEnglishKeys = Object.keys(zh).filter((key) => !(key in en))
if (missingEnglishKeys.length > 0) {
  throw new Error(`English dictionary is missing ${missingEnglishKeys.length} keys: ${missingEnglishKeys.slice(0, 10).join(', ')}`)
}

const manifestContext = { globalThis: {} }
vm.runInNewContext(readFileSync(resolve(root, '.next/server/app/page_client-reference-manifest.js'), 'utf8'), manifestContext)
const initialChunks = new Set(
  manifestContext.globalThis.__RSC_MANIFEST?.['/page']?.entryJSFiles?.['[project]/src/app/page'] ?? [],
)
const chunkDir = resolve(root, '.next/static/chunks')
const englishChunks = readdirSync(chunkDir)
  .filter((name) => name.endsWith('.js'))
  .filter((name) => readFileSync(resolve(chunkDir, name), 'utf8').includes('Meta-Org Console'))
  .map((name) => `static/chunks/${name}`)

if (englishChunks.length === 0) {
  throw new Error('English dictionary chunk was not found; run npm run build first')
}
const eagerlyLoadedEnglishChunks = englishChunks.filter((chunk) => initialChunks.has(chunk))
if (eagerlyLoadedEnglishChunks.length > 0) {
  throw new Error(`English dictionary is eagerly loaded by the page entry: ${eagerlyLoadedEnglishChunks.join(', ')}`)
}

console.log(`Verified lazy English dictionary with ${Object.keys(en).length} keys; default Chinese dictionary has ${Object.keys(zh).length} stable keys.`)
