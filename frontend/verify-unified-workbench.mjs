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
  projectLifecycle: read('src/app/project-lifecycle-workspace.tsx'),
  page: read('src/app/page.tsx'),
  api: read('src/lib/api.ts'),
  stream: read('src/lib/stream.ts'),
  assistant: read('src/app/ai-assistant.tsx'),
  systemAdmin: read('src/app/system-admin-workspace.tsx'),
  i18n: `${read('src/lib/i18n.tsx')}\n${read('src/lib/i18n.en.ts')}`,
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
  ['editable field defaults', sources.component, 'defaultValue={capability.masked'],
  ['header create operation', sources.component, 'onCreateHeader'],
  ['header update operation', sources.component, 'onUpdateHeader'],
  ['header delete operation', sources.component, 'onDeleteHeader'],
  ['line create operation', sources.component, 'onCreateLine'],
  ['line update operation', sources.component, 'onUpdateLine'],
  ['line delete operation', sources.component, 'onDeleteLine'],
  ['tenant menu documents', sources.page, 'buildTenantDocumentMenuItems'],
  ['middle business tree removed', sources.page, 'const showBusinessChrome = false'],
  ['business panel not rendered', sources.page, 'BusinessTreePanelRemoved'],
  ['platform tenant scope badge', sources.page, 'sessionScope={activeSessionScope}'],
  ['tenant scope translation', sources.i18n, "'scope.tenant': 'Tenant operations'"],
  ['platform scope translation', sources.i18n, "'scope.platform': '平台控制台'"],
  ['api update record', sources.api, 'export async function updateERPRecord'],
  ['api delete record', sources.api, 'export async function deleteERPRecord'],
  ['api update child', sources.api, 'export async function updateERPChildRecord'],
  ['api delete child', sources.api, 'export async function deleteERPChildRecord'],
  ['tenant ERP integration', sources.erp, 'buildERPDocumentWorkbenchDefinition'],
  ['tenant ERP render', sources.erp, '<DocumentWorkbench'],
  ['platform console integration', sources.systemAdmin, 'PlatformGovernanceMap'],
  ['platform deep links', sources.systemAdmin, 'systemAdmin.unifiedWorkbench'],
  ['stream scoped auth import', sources.stream, "import { getCurrentOrganizationId, normalizeOrganizationId } from './auth'"],
  ['stream scoped options', sources.stream, 'interface StreamRequestOptions'],
  ['stream tenant organization header', sources.stream, "headers['X-Organization-ID'] = organizationId"],
  ['assistant run stream scope', sources.assistant, 'scope: apiScope'],
  ['assistant run stream signal', sources.assistant, 'signal: controller.signal'],
  ['document download scoped auth import', sources.projectLifecycle, "import { getCurrentOrganizationId, normalizeOrganizationId } from '@/lib/auth'"],
  ['document download tenant organization header', sources.projectLifecycle, "headers['X-Organization-ID'] = organizationId"],
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
