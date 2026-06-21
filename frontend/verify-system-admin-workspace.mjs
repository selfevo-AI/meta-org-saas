import { existsSync, readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const frontendRoot = fileURLToPath(new URL('.', import.meta.url))
const apiSource = readFileSync(`${frontendRoot}src/lib/api.ts`, 'utf8')
const authSource = readFileSync(`${frontendRoot}src/lib/auth.ts`, 'utf8')
const pageSource = readFileSync(`${frontendRoot}src/app/page.tsx`, 'utf8')
const i18nSource = readFileSync(`${frontendRoot}src/lib/i18n.tsx`, 'utf8')
const workspacePath = `${frontendRoot}src/app/system-admin-workspace.tsx`
const workspaceSource = existsSync(workspacePath) ? readFileSync(workspacePath, 'utf8') : ''

const requiredApiExports = [
  'listPlatformMasters',
  'listPlatformDetails',
  'listOrganizationSchemaTargets',
  'exportOrganizationSchema',
  'createOrganizationSchemaChange',
  'approveSchemaChange',
  'applySchemaChange',
  'listPlatformOrganizations',
  'getOrganizationSubscription',
  'getOrganizationEntitlements',
  'updateOrganizationModules',
  'listOrganizationInvitations',
  'createOrganizationInvitation',
]

const requiredApiTypes = [
  'PlatformMaster',
  'PlatformDetail',
  'OrganizationSchemaTarget',
  'SchemaPackage',
  'SchemaTableDefinition',
  'SchemaFieldDefinition',
  'SchemaChangeRequest',
  'SchemaApplyJob',
  'CreateSchemaChangeRequestInput',
  'OrganizationSubscription',
  'OrganizationInvitation',
  'CreateOrganizationInvitationInput',
]

const requiredI18nKeys = [
  'auth.tenantConsole',
  'auth.platformAdmin',
  'auth.organizationLogin',
  'auth.platformLogin',
  'auth.platformRoleRequired',
  'auth.usePlatformLogin',
  'auth.emailAlreadyRegistered',
  'SystemAdmin',
  'systemAdmin.title',
  'systemAdmin.platformCatalog',
  'systemAdmin.saasOrganizations',
  'systemAdmin.schemaTargets',
  'systemAdmin.schemaPackage',
  'systemAdmin.saasOrganizationsSummary',
  'systemAdmin.subscription',
  'systemAdmin.entitlements',
  'systemAdmin.enabledModules',
  'systemAdmin.updateModules',
  'systemAdmin.modulesUpdated',
  'systemAdmin.invitations',
  'systemAdmin.inviteEmail',
  'systemAdmin.inviteName',
  'systemAdmin.inviteAuthority',
  'systemAdmin.createInvitation',
  'systemAdmin.invitationCreated',
  'systemAdmin.noOrganizations',
  'systemAdmin.noInvitations',
  'systemAdmin.module',
  'systemAdmin.selectedOrganization',
  'systemAdmin.exportSchema',
  'systemAdmin.importJson',
  'systemAdmin.createChange',
  'systemAdmin.approve',
  'systemAdmin.apply',
  'systemAdmin.changeCreated',
  'systemAdmin.changeApproved',
  'systemAdmin.changeApplied',
  'systemAdmin.loadFailed',
]

const requiredPageSnippets = [
  "type LoginSurface = 'tenant' | 'platform'",
  "SystemAdmin: '",
  'SystemAdminWorkspace',
  "'SystemAdmin'",
  "effectiveWorkspaceView === 'domain:SystemAdmin'",
  'platformRole',
  'loginSurface',
  'effectiveWorkspaceView',
  "isPlatformAdminSession && workspaceView === 'overview' ? 'domain:SystemAdmin' : workspaceView",
  'showBusinessChrome',
  "showBusinessControl={showBusinessChrome}",
  "setWorkspaceView('domain:SystemAdmin')",
  "setCurrentOrganizationId(null)",
  "t('auth.platformAdmin')",
  "t('auth.emailAlreadyRegistered')",
]

const requiredWorkspaceSnippets = [
  'export function SystemAdminWorkspace',
  'listPlatformMasters',
  'listPlatformOrganizations',
  'getOrganizationSubscription',
  'getOrganizationEntitlements',
  'updateOrganizationModules',
  'listOrganizationInvitations',
  'createOrganizationInvitation',
  'listOrganizationSchemaTargets',
  'exportOrganizationSchema',
  'createOrganizationSchemaChange',
  'approveSchemaChange',
  'applySchemaChange',
  "'saas'",
  "t('systemAdmin.",
]

const requiredAuthSnippets = [
  'user.platform_role',
  'localStorage.removeItem(ORGANIZATION_KEY)',
]

const requiredApiSnippets = [
  'organizationId?: string | null',
  'organizationId: organizationID',
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

const missingExports = requiredApiExports.filter((name) => !apiSource.includes(`export async function ${name}`))
if (missingExports.length > 0) {
  failures.push(`Missing system admin API functions:\n${missingExports.map((name) => `  - ${name}`).join('\n')}`)
}

const missingTypes = requiredApiTypes.filter((name) => !apiSource.includes(`export interface ${name}`))
if (missingTypes.length > 0) {
  failures.push(`Missing system admin API types:\n${missingTypes.map((name) => `  - ${name}`).join('\n')}`)
}

const missingApiSnippets = requiredApiSnippets.filter((snippet) => !apiSource.includes(snippet))
if (missingApiSnippets.length > 0) {
  failures.push(`Missing per-request organization context snippets:\n${missingApiSnippets.map((snippet) => `  - ${snippet}`).join('\n')}`)
}

const missingAuthSnippets = requiredAuthSnippets.filter((snippet) => !authSource.includes(snippet))
if (missingAuthSnippets.length > 0) {
  failures.push(`Missing platform session isolation snippets:\n${missingAuthSnippets.map((snippet) => `  - ${snippet}`).join('\n')}`)
}

const missingPageSnippets = requiredPageSnippets.filter((snippet) => !pageSource.includes(snippet))
if (missingPageSnippets.length > 0) {
  failures.push(`Missing page integration snippets:\n${missingPageSnippets.map((snippet) => `  - ${snippet}`).join('\n')}`)
}

if (!workspaceSource) {
  failures.push('Missing src/app/system-admin-workspace.tsx')
} else {
  const missingWorkspaceSnippets = requiredWorkspaceSnippets.filter((snippet) => !workspaceSource.includes(snippet))
  if (missingWorkspaceSnippets.length > 0) {
    failures.push(`Missing workspace integration snippets:\n${missingWorkspaceSnippets.map((snippet) => `  - ${snippet}`).join('\n')}`)
  }
}

const missingEnKeys = requiredI18nKeys.filter((key) => !hasDictionaryKey(enDictionary, key))
if (missingEnKeys.length > 0) {
  failures.push(`Missing English system admin i18n keys:\n${missingEnKeys.map((key) => `  - ${key}`).join('\n')}`)
}

const missingZhKeys = requiredI18nKeys.filter((key) => !hasDictionaryKey(zhDictionary, key))
if (missingZhKeys.length > 0) {
  failures.push(`Missing Chinese system admin i18n keys:\n${missingZhKeys.map((key) => `  - ${key}`).join('\n')}`)
}

if (failures.length > 0) {
  console.error(failures.join('\n\n'))
  process.exit(1)
}

console.log(`Verified ${requiredApiExports.length} system admin API functions, workspace integration, and bilingual i18n keys.`)
