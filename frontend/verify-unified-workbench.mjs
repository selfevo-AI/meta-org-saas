import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const frontendRoot = fileURLToPath(new URL('.', import.meta.url))

function read(path) {
  return readFileSync(`${frontendRoot}${path}`, 'utf8')
}

const sources = {
  package: read('package.json'),
  model: read('src/lib/workbench.ts'),
  component: read('src/app/document-workbench.tsx'),
  erp: read('src/app/erp-business-module-workspace.tsx'),
  systemAdmin: read('src/app/system-admin-workspace.tsx'),
  i18n: read('src/lib/i18n.tsx'),
  docs: read('../docs/operations/unified-workbench-governance.md'),
}

const checks = [
  ['package script', sources.package, '"test:unified-workbench": "node verify-unified-workbench.mjs"'],
  ['workbench model', sources.model, 'export interface DocumentWorkbenchDefinition'],
  ['permission normalization', sources.model, 'export function resolveFieldCapability'],
  ['strong field lock', sources.model, "lockedReason: 'strong_business_logic'"],
  ['document component', sources.component, 'export function DocumentWorkbench'],
  ['embedded operation drawer', sources.component, 'OperationRunnerDrawer'],
  ['field capability rendering', sources.component, 'resolveFieldCapability'],
  ['tenant ERP integration', sources.erp, 'buildERPDocumentWorkbenchDefinition'],
  ['tenant ERP render', sources.erp, '<DocumentWorkbench'],
  ['platform console integration', sources.systemAdmin, 'PlatformGovernanceMap'],
  ['platform deep links', sources.systemAdmin, 'systemAdmin.unifiedWorkbench'],
  ['english i18n', sources.i18n, "'workbench.unified.title': 'Unified operations workbench'"],
  ['chinese i18n', sources.i18n, "'workbench.unified.title': '统一操作工作台'"],
  ['documentation', sources.docs, '统一工作台与字段权限治理规范'],
]

const failures = checks
  .filter(([, source, snippet]) => !source.includes(snippet))
  .map(([label, , snippet]) => `${label}: missing ${snippet}`)

if (failures.length > 0) {
  console.error(failures.join('\n'))
  process.exit(1)
}

console.log('Verified unified workbench contract, tenant/platform integration, i18n, and governance documentation.')
