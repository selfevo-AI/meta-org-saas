import { existsSync, readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const frontendRoot = fileURLToPath(new URL('.', import.meta.url))
const apiSource = readFileSync(`${frontendRoot}src/lib/api.ts`, 'utf8')
const authSource = readFileSync(`${frontendRoot}src/lib/auth.ts`, 'utf8')
const pageSource = readFileSync(`${frontendRoot}src/app/page.tsx`, 'utf8')
const assistantSource = readFileSync(`${frontendRoot}src/app/ai-assistant.tsx`, 'utf8')
const apiWorkbenchSource = readFileSync(`${frontendRoot}src/app/api-workbench.tsx`, 'utf8')
const i18nSource = readFileSync(`${frontendRoot}src/lib/i18n.tsx`, 'utf8')
const operationsSource = readFileSync(`${frontendRoot}src/lib/operations.ts`, 'utf8')
const workspacePath = `${frontendRoot}src/app/system-admin-workspace.tsx`
const workspaceSource = existsSync(workspacePath) ? readFileSync(workspacePath, 'utf8') : ''

const requiredApiExports = [
  'listPlatformMasters',
  'listPlatformDetails',
  'listOrganizationSchemaTargets',
  'exportOrganizationSchema',
  'createOrganizationSchemaChange',
  'approveSchemaChange',
  'verifySchemaChange',
  'applySchemaChange',
  'listPlatformOrganizations',
  'getOrganizationSubscription',
  'getOrganizationEntitlements',
  'updateOrganizationModules',
  'listOrganizationInvitations',
  'createOrganizationInvitation',
  'closePlatformOrganization',
  'getPlatformPermissionProfile',
  'createPlatformAssistantSession',
  'approvePlatformToolApproval',
  'rejectPlatformToolApproval',
  'getPlatformAIInvocation',
  'listPlatformModelProviders',
]

const requiredApiTypes = [
  'PlatformMaster',
  'PlatformDetail',
  'OrganizationSchemaTarget',
  'SchemaPackage',
  'SchemaTableDefinition',
  'SchemaFieldDefinition',
  'SchemaChangeRequest',
  'SchemaVerificationReport',
  'SchemaVerificationCheck',
  'SchemaApplyJob',
  'CreateSchemaChangeRequestInput',
  'OrganizationSubscription',
  'OrganizationInvitation',
  'CreateOrganizationInvitationInput',
  'PlatformPermissionProfile',
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
  'systemAdmin.verify',
  'systemAdmin.apply',
  'systemAdmin.changeCreated',
  'systemAdmin.changeApproved',
  'systemAdmin.changeVerified',
  'systemAdmin.changeApplied',
  'systemAdmin.verificationReport',
  'systemAdmin.canApply',
  'systemAdmin.blockingIssues',
  'systemAdmin.checks',
  'systemAdmin.packageDiff',
  'systemAdmin.noPackageDiff',
  'systemAdmin.check.schema_package',
  'systemAdmin.check.ddl_plan',
  'systemAdmin.check.risk_level',
  'systemAdmin.check.lifecycle_status',
  'systemAdmin.check.permissions_impact',
  'systemAdmin.check.runtime_operations',
  'systemAdmin.check.assistant_context',
  'systemAdmin.check.tool_policy',
  'systemAdmin.check.assistant_skills',
  'systemAdmin.check.quality_gates',
  'systemAdmin.check.verification_scenarios',
  'systemAdmin.check.rollback_risk',
  'systemAdmin.erpAsset.context_rules',
  'systemAdmin.erpAsset.tool_definitions',
  'systemAdmin.erpAsset.assistant_skills',
  'systemAdmin.erpAsset.quality_gates',
  'systemAdmin.erpAsset.verification_scenarios',
  'systemAdmin.loadFailed',
  'systemAdmin.permissions',
  'systemAdmin.closeOrganization',
  'systemAdmin.organizationClosed',
  'systemAdmin.showClosedOrganizations',
  'systemAdmin.platformAssistant',
  'systemAdmin.platformFeatures',
  'systemAdmin.apiWorkbench',
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
  'showBusinessChrome',
  "showBusinessControl={showBusinessChrome}",
  'const platformMenuGroups',
  'const visibleMenuGroups = isPlatformAdminSession ? platformMenuGroups : menuGroups',
  'groups={visibleMenuGroups}',
  '!isPlatformAdminSession && <BusinessStatusPanel',
  'getPlatformMetaOrgOverview',
  'getPlatformMetaOrgInbox',
  'listPlatformModels',
  'listPlatformAssistantSkills',
  "setWorkspaceView('overview')",
  "apiScope={isPlatformAdminSession ? 'platform' : 'tenant'}",
  "setCurrentOrganizationId(null)",
  "t('auth.platformAdmin')",
  "t('auth.emailAlreadyRegistered')",
  'const tenantOnlyDomains',
]

const requiredWorkspaceSnippets = [
  'export function SystemAdminWorkspace',
  'AIAssistant',
  'apiScope="platform"',
  'ApiWorkbench',
  'platformFeatureTabs',
  'platformPermissions',
  'canPlatform',
  'listPlatformMasters',
  'listPlatformOrganizations',
  'getPlatformPermissionProfile',
  'closePlatformOrganization',
  'getOrganizationSubscription',
  'getOrganizationEntitlements',
  'updateOrganizationModules',
  'listOrganizationInvitations',
  'createOrganizationInvitation',
  'listOrganizationSchemaTargets',
  'exportOrganizationSchema',
  'createOrganizationSchemaChange',
  'approveSchemaChange',
  'verifySchemaChange',
  'verificationReport',
  'applySchemaChange',
  'schemaDiffItems',
  'changeRequest?.diff',
  "t('systemAdmin.packageDiff')",
  "'context_rules'",
  "'tool_definitions'",
  "'assistant_skills'",
  "'quality_gates'",
  "'verification_scenarios'",
  "'saas'",
  "t('systemAdmin.",
]

const requiredAssistantSnippets = [
  'approvePlatformToolApproval',
  'rejectPlatformToolApproval',
  "apiScope === 'platform' ? approvePlatformToolApproval : approveToolApproval",
  "apiScope === 'platform' ? rejectPlatformToolApproval : rejectToolApproval",
  "apiScope === 'platform' ? getPlatformAIInvocation : getAIInvocation",
  "apiScope === 'platform' ? listPlatformModelProviders : listModelProviders",
]

const requiredOperationSnippets = [
  "id: 'system-admin-schema-change-verify'",
  "title: 'operation.systemAdmin.schemaChangeVerify'",
  "path: '/platform/admin/schema-change-requests/{id}/verify'",
  "label: 'operation.systemAdmin.schemaChangeRequestId'",
  "operationKind: 'admin'",
  "dangerLevel: 'low'",
  "resultView: 'audit'",
]

requiredApiExports.push('getSchemaChangePackageDiff')

requiredApiTypes.push(
  'IndustrySolutionManifest',
  'IndustrySolutionAsset',
  'PackageAssetDiff',
  'SchemaApplyAssetResult',
  'PublicationGateResult',
)

requiredI18nKeys.push(
  'systemAdmin.packageAssets',
  'systemAdmin.assetResults',
  'systemAdmin.publicationGates',
  'systemAdmin.blockingReason',
  'systemAdmin.metadataAssets',
  'systemAdmin.assetType.database_asset',
  'systemAdmin.assetType.business_function',
  'systemAdmin.assetType.process_loop',
  'systemAdmin.assetType.runtime_operation',
  'systemAdmin.assetType.tool_policy',
  'systemAdmin.assetType.tool_definition',
  'systemAdmin.assetType.context_rule',
  'systemAdmin.assetType.assistant_skill',
  'systemAdmin.assetType.quality_gate',
  'systemAdmin.assetType.verification_scenario',
  'systemAdmin.check.industry_manifest',
  'systemAdmin.gate.anonymization_check',
  'systemAdmin.gate.knowledge_source_permission_check',
  'systemAdmin.gate.verification_scenario_check',
)

requiredWorkspaceSnippets.push(
  'getSchemaChangePackageDiff',
  'packageAssetDiff',
  'packageAssetsByType',
  'assetResults',
  'publication_gates',
  "t('systemAdmin.packageAssets')",
  "t('systemAdmin.assetResults')",
  "t('systemAdmin.publicationGates')",
  '<DeveloperToolsWorkspace token={token} apiScope="platform" />',
)

const requiredI18nValueSnippets = [
  "'systemAdmin.modelAndApiSettings': 'AI models and API access'",
  "'systemAdmin.modelAndApiSettings': 'AI模型及API接入'",
]

const movedPlatformDomains = [
  'Capability',
  'Governance',
  'Evolution',
  'Verification',
  'DeveloperTools',
  'Identity',
  'Layer',
  'Observability',
  'SystemAdmin',
]

const requiredAuthSnippets = [
  'user.platform_role',
  'localStorage.removeItem(ORGANIZATION_KEY)',
]

const requiredApiSnippets = [
  'organizationId?: string | null',
  'organizationId: organizationID',
  'diff?: SchemaDiff[]',
  'action: string',
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

const defaultMenuStart = pageSource.indexOf('const defaultMenuGroups')
const platformMenuStart = pageSource.indexOf('const platformMenuGroups')
if (defaultMenuStart === -1 || platformMenuStart === -1 || platformMenuStart <= defaultMenuStart) {
  failures.push('Could not locate tenant and platform menu group boundaries')
} else {
  const defaultMenuSource = pageSource.slice(defaultMenuStart, platformMenuStart)
  const domainsStillInTenantMenu = movedPlatformDomains.filter((domain) => defaultMenuSource.includes(`'${domain}'`))
  if (domainsStillInTenantMenu.length > 0) {
    failures.push(`Platform-only domains still appear in tenant default menu:\n${domainsStillInTenantMenu.map((domain) => `  - ${domain}`).join('\n')}`)
  }
}

if (!workspaceSource) {
  failures.push('Missing src/app/system-admin-workspace.tsx')
} else {
  const missingWorkspaceSnippets = requiredWorkspaceSnippets.filter((snippet) => !workspaceSource.includes(snippet))
  if (missingWorkspaceSnippets.length > 0) {
    failures.push(`Missing workspace integration snippets:\n${missingWorkspaceSnippets.map((snippet) => `  - ${snippet}`).join('\n')}`)
  }
}

const missingAssistantSnippets = requiredAssistantSnippets.filter((snippet) => !assistantSource.includes(snippet))
if (missingAssistantSnippets.length > 0) {
  failures.push(`Missing platform assistant approval snippets:\n${missingAssistantSnippets.map((snippet) => `  - ${snippet}`).join('\n')}`)
}

const missingOperationSnippets = requiredOperationSnippets.filter((snippet) => !operationsSource.includes(snippet))
if (missingOperationSnippets.length > 0) {
  failures.push(`Missing schema verification operation snippets:\n${missingOperationSnippets.map((snippet) => `  - ${snippet}`).join('\n')}`)
}

const platformDomainsMatch = apiWorkbenchSource.match(/const platformOperationDomains = new Set\(\[([\s\S]*?)\]\)/)
if (!platformDomainsMatch) {
  failures.push('Could not locate ApiWorkbench platformOperationDomains')
} else if (platformDomainsMatch[1].includes("'DeveloperTools'")) {
  failures.push('API Workbench platform domain menu must not include DeveloperTools; AI model/API access belongs in the dedicated platform settings workspace.')
}

const missingI18nValueSnippets = requiredI18nValueSnippets.filter((snippet) => !i18nSource.includes(snippet))
if (missingI18nValueSnippets.length > 0) {
  failures.push(`Missing exact AI model/API access labels:\n${missingI18nValueSnippets.map((snippet) => `  - ${snippet}`).join('\n')}`)
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
