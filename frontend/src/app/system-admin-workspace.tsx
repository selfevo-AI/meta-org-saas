'use client'

import {
  Activity,
  Bot,
  Braces,
  Check,
  Database,
  Download,
  FileJson,
  Layers3,
  Play,
  RefreshCw,
  Send,
  ShieldCheck,
  Table2,
  Upload,
  Users,
} from 'lucide-react'
import { ChangeEvent, Fragment, useCallback, useEffect, useMemo, useState } from 'react'
import { AIAssistant } from './ai-assistant'
import { ApiWorkbench } from './api-workbench'
import { DeveloperToolsWorkspace } from './developer-tools-workspace'
import {
  applySchemaChange,
  applyIndustryPackageToOrganization,
  approveSchemaChange,
  closePlatformOrganization,
  createERPStandardSolutionFlow,
  createDatabaseMaintenanceJob,
  createIndustryExtension,
  createIndustrySolutionSchemaChange,
  createOrganizationSchemaChange,
  createOrganizationInvitation,
  createPlatformFeature,
  createPlatformUser,
  disablePlatformUser,
  getPlatformPermissionProfile,
  getPlatformAssistantContextHealth,
  getMonitoringAgentStatus,
  exportOrganizationSchema,
  getOrganizationIndustry,
  getOrganizationEntitlements,
  getOrganizationSubscription,
  getSchemaChangePackageDiff,
  listIndustries,
  listIndustryExtensions,
  listIndustryPackages,
  listIndustryPublicationRequests,
  listMonitoringAgentRuns,
  listOrganizationInvitations,
  listOrganizationSchemaTargets,
  listDatabaseMaintenanceJobs,
  listPlatformDetails,
  listPlatformFeatures,
  listPlatformMasters,
  listPlatformOrganizations,
  listPlatformPermissions,
  listPlatformRoles,
  listPlatformUsers,
  listSaaSModules,
  publishPlatformFeature,
  resetPlatformUserPassword,
  reviewDatabaseMaintenanceJob,
  reviewIndustryPublicationRequest,
  runMonitoringAgent,
  setPlatformRolePermissions,
  submitIndustryExtensionPublication,
  verifySchemaChange,
  type DatabaseMaintenanceJob,
  type Industry,
  type IndustryExtension,
  type IndustryPackage,
  type IndustryPublicationRequest,
  type IndustrySolutionManifest,
  type OrganizationIndustryAdoption,
  updateOrganizationModules,
  type OrganizationInvitation,
  type OrganizationSubscription,
  type OrganizationSchemaTarget,
  type PlatformDetail,
  type PlatformFeature,
  type PlatformMaster,
  type PlatformPermission,
  type PlatformPermissionProfile,
  type PlatformRole,
  type PlatformUser,
  type AssistantContextHealthSummary,
  type MonitoringAgentRun,
  type MonitoringAgentStatus,
  type PublicationGateResult,
  type SaaSModule,
  type PackageAssetDiff,
  type SchemaApplyAssetResult,
  type SchemaApplyJob,
  type SchemaChangeRequest,
  type SchemaPackage,
  type SchemaVerificationReport,
  type SessionOrganization,
  updatePlatformOrganizationProfile,
} from '@/lib/api'
import { useI18n } from '@/lib/i18n'

interface SystemAdminWorkspaceProps {
  token: string
  organizations: SessionOrganization[]
  currentOrganizationID?: string | null
  activeSection?: string
}

type TabID =
  | 'assistant'
  | 'monitoring'
  | 'saas'
  | 'industry'
  | 'features'
  | 'permissions'
  | 'users'
  | 'database'
  | 'models'
  | 'runtime'
  | 'catalog'
  | 'targets'
  | 'schema'

const tabs: Array<{ id: TabID; label: string; icon: typeof Database; permission?: string }> = [
  { id: 'assistant', label: 'systemAdmin.platformAssistant', icon: Bot, permission: 'assistant.platform.run' },
  { id: 'monitoring', label: 'systemAdmin.monitoringAgent', icon: Activity, permission: 'platform.read' },
  { id: 'saas', label: 'systemAdmin.saasOrganizations', icon: Users, permission: 'platform.read' },
  { id: 'industry', label: 'systemAdmin.industries', icon: Layers3, permission: 'industry.solution.manage' },
  { id: 'features', label: 'systemAdmin.platformFeatures', icon: ShieldCheck, permission: 'platform.feature.manage' },
  { id: 'permissions', label: 'systemAdmin.permissions', icon: ShieldCheck, permission: 'platform.rbac.manage' },
  { id: 'users', label: 'systemAdmin.platformUsers', icon: Users, permission: 'platform.user.manage' },
  { id: 'database', label: 'systemAdmin.databaseMaintenance', icon: Database, permission: 'database.maintenance.manage' },
  { id: 'models', label: 'systemAdmin.modelAndApiSettings', icon: Table2, permission: 'model.manage' },
  { id: 'runtime', label: 'systemAdmin.apiWorkbench', icon: Table2, permission: 'runtime.manage' },
  { id: 'catalog', label: 'systemAdmin.platformCatalog', icon: Layers3, permission: 'platform.read' },
  { id: 'targets', label: 'systemAdmin.schemaTargets', icon: Database, permission: 'schema.manage' },
  { id: 'schema', label: 'systemAdmin.schemaPackage', icon: FileJson, permission: 'schema.manage' },
]

const moduleOptions = ['data_catalog', 'saas', 'security', 'assistant', 'organization', 'skill', 'finance', 'system']
const platformPermissionCatalog = [
  'platform.read',
  'organization.manage',
  'organization.close',
  'schema.manage',
  'schema.approve',
  'schema.apply',
  'model.manage',
  'runtime.manage',
  'assistant.platform.run',
  'platform.feature.manage',
  'platform.user.manage',
  'platform.rbac.manage',
  'database.maintenance.manage',
  'database.maintenance.approve',
  'api.manage',
  'industry.solution.manage',
  'industry.solution.import',
  'industry.solution.export',
  'tenant.industry_solution.apply',
]
const erpSolutionModules = ['project', 'procurement', 'inventory', 'sales', 'finance']
const erpSolutionAssets = [
  'database_assets',
  'business_functions',
  'process_loops',
  'permissions',
  'api_operations',
  'ui_workspaces',
  'assistant_targets',
  'context_rules',
  'tool_definitions',
  'assistant_skills',
  'quality_gates',
  'verification_scenarios',
]
const platformFeatureTabs = [
  { id: 'capability', label: 'systemAdmin.feature.capability', permission: 'platform.read', moduleKey: 'capability' },
  { id: 'governance', label: 'systemAdmin.feature.governance', permission: 'platform.read', moduleKey: 'governance' },
  { id: 'evolution', label: 'systemAdmin.feature.evolution', permission: 'platform.read', moduleKey: 'evolution' },
  { id: 'verification', label: 'systemAdmin.feature.verification', permission: 'platform.read', moduleKey: 'verification' },
  { id: 'system', label: 'systemAdmin.feature.system', permission: 'runtime.manage', moduleKey: 'system' },
  { id: 'systemAdmin', label: 'systemAdmin.feature.systemAdmin', permission: 'platform.read', moduleKey: 'saas' },
  { id: 'modelSettings', label: 'systemAdmin.feature.modelSettings', permission: 'model.manage', moduleKey: 'developer_tools' },
  { id: 'identity', label: 'systemAdmin.feature.identity', permission: 'platform.read', moduleKey: 'identity' },
  { id: 'layer', label: 'systemAdmin.feature.layer', permission: 'platform.read', moduleKey: 'layer' },
  { id: 'observability', label: 'systemAdmin.feature.observability', permission: 'platform.read', moduleKey: 'observability' },
]

function jsonText(value: unknown): string {
  return JSON.stringify(value ?? {}, null, 2)
}

function formatDateTime(value?: string): string {
  if (!value) return ''
  return new Intl.DateTimeFormat(undefined, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value))
}

function countFields(pkg: SchemaPackage | null): number {
  return pkg?.tables.reduce((total, table) => total + table.fields.length, 0) ?? 0
}

function summarizeSchemaDiff(diff?: SchemaChangeRequest['diff']): string[] {
  if (!Array.isArray(diff)) return []
  return diff.map((item) =>
    [item.action, item.table, item.field, item.from && item.to ? `${item.from} -> ${item.to}` : item.to || item.from, item.risk]
      .filter(Boolean)
      .join(' / '),
  )
}

function summaryNumber(summary: Record<string, unknown> | undefined, key: string): number {
  const value = summary?.[key]
  return typeof value === 'number' && Number.isFinite(value) ? value : 0
}

function categoryEntries(summary: Record<string, unknown> | undefined): Array<[string, number]> {
  const value = summary?.by_category
  if (!value || typeof value !== 'object' || Array.isArray(value)) return []
  return Object.entries(value)
    .map(([key, count]) => [key, typeof count === 'number' ? count : Number(count) || 0] as [string, number])
    .filter(([, count]) => count > 0)
}

function parseSchemaPackage(source: string): SchemaPackage {
  const parsed = JSON.parse(source) as Partial<SchemaPackage>
  if (!parsed || typeof parsed !== 'object' || !Array.isArray(parsed.tables)) {
    throw new Error('invalid schema package')
  }
  return parsed as SchemaPackage
}

const tabIDs = new Set<TabID>(tabs.map((tab) => tab.id))

function normalizeTabID(value?: string): TabID | undefined {
  return value && tabIDs.has(value as TabID) ? (value as TabID) : undefined
}

function splitLines(value: string): string[] {
  return value
    .split(/[\n,]/)
    .map((item) => item.trim())
    .filter(Boolean)
}

export function SystemAdminWorkspace({ token, organizations, currentOrganizationID, activeSection }: SystemAdminWorkspaceProps) {
  const { t } = useI18n()
  const activeSectionID = normalizeTabID(activeSection)
  const [activeTab, setActiveTab] = useState<TabID>(activeSectionID ?? 'assistant')
  const [moduleKey, setModuleKey] = useState('data_catalog')
  const [platformPermissions, setPlatformPermissions] = useState<PlatformPermissionProfile | null>(null)
  const [platformOrganizations, setPlatformOrganizations] = useState<SessionOrganization[]>([])
  const [platformFeatures, setPlatformFeatures] = useState<PlatformFeature[]>([])
  const [platformPermissionItems, setPlatformPermissionItems] = useState<PlatformPermission[]>([])
  const [platformRoles, setPlatformRoles] = useState<PlatformRole[]>([])
  const [platformUsers, setPlatformUsers] = useState<PlatformUser[]>([])
  const [databaseMaintenanceJobs, setDatabaseMaintenanceJobs] = useState<DatabaseMaintenanceJob[]>([])
  const [saasModules, setSaaSModules] = useState<SaaSModule[]>([])
  const [industries, setIndustries] = useState<Industry[]>([])
  const [industryPackages, setIndustryPackages] = useState<IndustryPackage[]>([])
  const [industryAdoption, setIndustryAdoption] = useState<OrganizationIndustryAdoption | null>(null)
  const [industryExtensions, setIndustryExtensions] = useState<IndustryExtension[]>([])
  const [publicationRequests, setPublicationRequests] = useState<IndustryPublicationRequest[]>([])
  const [selectedIndustryKey, setSelectedIndustryKey] = useState('general')
  const [selectedPackageID, setSelectedPackageID] = useState('')
  const [industryModuleDraft, setIndustryModuleDraft] = useState<string[]>([])
  const [erpSolutionModuleDraft, setERPSolutionModuleDraft] = useState<string[]>(erpSolutionModules)
  const [industryTableDraft, setIndustryTableDraft] = useState({
    tableName: '',
    displayName: '',
    fieldName: '',
    dataType: 'varchar(120)',
    defaultValue: '',
    nullable: true,
  })
  const [extensionKey, setExtensionKey] = useState('')
  const [extensionName, setExtensionName] = useState('')
  const [extensionModuleKey, setExtensionModuleKey] = useState('')
  const [publicationReason, setPublicationReason] = useState('')
  const [subscription, setSubscription] = useState<OrganizationSubscription | null>(null)
  const [entitlements, setEntitlements] = useState<Record<string, boolean>>({})
  const [moduleDraft, setModuleDraft] = useState<string[]>([])
  const [invitations, setInvitations] = useState<OrganizationInvitation[]>([])
  const [inviteEmail, setInviteEmail] = useState('')
  const [inviteName, setInviteName] = useState('')
  const [inviteAuthority, setInviteAuthority] = useState('organization_admin')
  const [organizationProfileForm, setOrganizationProfileForm] = useState({ name: '', description: '' })
  const [showClosedOrganizations, setShowClosedOrganizations] = useState(false)
  const [closeReason, setCloseReason] = useState('')
  const [featureDraft, setFeatureDraft] = useState({ featureKey: '', moduleKey: 'platform_admin', title: '', permissionKeys: 'platform.read' })
  const [rolePermissionDraft, setRolePermissionDraft] = useState<Record<string, string>>({})
  const [platformUserDraft, setPlatformUserDraft] = useState({ name: '', email: '', roles: 'operator' })
  const [temporaryCredential, setTemporaryCredential] = useState('')
  const [maintenanceDraft, setMaintenanceDraft] = useState({ jobType: 'backup', scope: 'platform', reason: '', backupRef: '' })
  const [masters, setMasters] = useState<PlatformMaster[]>([])
  const [details, setDetails] = useState<PlatformDetail[]>([])
  const [targets, setTargets] = useState<OrganizationSchemaTarget[]>([])
  const [selectedMasterKey, setSelectedMasterKey] = useState('')
  const [selectedOrganizationID, setSelectedOrganizationID] = useState(currentOrganizationID || organizations[0]?.id || '')
  const [schemaPackage, setSchemaPackage] = useState<SchemaPackage | null>(null)
  const [schemaJson, setSchemaJson] = useState('')
  const [changeRequest, setChangeRequest] = useState<SchemaChangeRequest | null>(null)
  const [applyJob, setApplyJob] = useState<SchemaApplyJob | null>(null)
  const [packageAssetDiff, setPackageAssetDiff] = useState<PackageAssetDiff[]>([])
  const [verificationReport, setVerificationReport] = useState<SchemaVerificationReport | null>(null)
  const [contextHealth, setContextHealth] = useState<AssistantContextHealthSummary | null>(null)
  const [monitoringStatus, setMonitoringStatus] = useState<MonitoringAgentStatus | null>(null)
  const [monitoringRuns, setMonitoringRuns] = useState<MonitoringAgentRun[]>([])
  const [reason, setReason] = useState('')
  const [loading, setLoading] = useState(false)
  const [notice, setNotice] = useState('')
  const [error, setError] = useState('')

  const managementOrganizations = useMemo(() => {
    const byID = new Map<string, SessionOrganization>()
    for (const item of [...platformOrganizations, ...organizations]) {
      byID.set(item.id, item)
    }
    return Array.from(byID.values())
  }, [organizations, platformOrganizations])
  const visibleManagementOrganizations = useMemo(
    () => managementOrganizations.filter((item) => showClosedOrganizations || item.status !== 'closed'),
    [managementOrganizations, showClosedOrganizations],
  )
  const organizationByID = useMemo(
    () => Object.fromEntries(managementOrganizations.map((item) => [item.id, item.name])),
    [managementOrganizations],
  )
  const activeOrganizationID = useMemo(() => {
    if (selectedOrganizationID && visibleManagementOrganizations.some((item) => item.id === selectedOrganizationID)) return selectedOrganizationID
    if (currentOrganizationID && visibleManagementOrganizations.some((item) => item.id === currentOrganizationID)) return currentOrganizationID
    return visibleManagementOrganizations[0]?.id || ''
  }, [currentOrganizationID, selectedOrganizationID, visibleManagementOrganizations])
  const selectedOrganization = useMemo(
    () => managementOrganizations.find((item) => item.id === activeOrganizationID) ?? null,
    [activeOrganizationID, managementOrganizations],
  )
  const selectedMaster = useMemo(() => masters.find((item) => item.master_key === selectedMasterKey) ?? masters[0], [masters, selectedMasterKey])
  const selectedTarget = useMemo(
    () => targets.find((item) => item.organization_id === activeOrganizationID) ?? null,
    [activeOrganizationID, targets],
  )
  const verificationBlocksApply =
    !!changeRequest && verificationReport?.change_request_id === changeRequest.id && verificationReport.can_apply === false
  const schemaDiffItems = useMemo(() => summarizeSchemaDiff(changeRequest?.diff), [changeRequest?.diff])
  const industryManifest = useMemo(() => {
    const manifest = changeRequest?.schema_package.metadata?.industry_manifest
    return manifest && typeof manifest === 'object' ? (manifest as IndustrySolutionManifest) : null
  }, [changeRequest])
  const packageAssetsByType = useMemo(() => {
    const groups = new Map<string, IndustrySolutionManifest['assets']>()
    for (const asset of industryManifest?.assets ?? []) {
      groups.set(asset.asset_type, [...(groups.get(asset.asset_type) ?? []), asset])
    }
    return Array.from(groups.entries()).map(([assetType, assets]) => ({ assetType, assets }))
  }, [industryManifest])
  const assetResults = useMemo<SchemaApplyAssetResult[]>(() => applyJob?.metadata?.asset_results ?? [], [applyJob])
  const selectedIndustryPackage = useMemo(
    () => industryPackages.find((item) => item.id === selectedPackageID) ?? industryPackages[0] ?? null,
    [industryPackages, selectedPackageID],
  )
  const selectedIndustryPackageModules = useMemo(
    () =>
      selectedIndustryPackage?.assets
        .filter((asset) => asset.asset_type === 'module')
        .map((asset) => String(asset.payload.module_key || ''))
        .filter(Boolean) ?? [],
    [selectedIndustryPackage],
  )
  const canPlatform = useCallback(
    (permission: string) => !platformPermissions || !!platformPermissions.permissions[permission],
    [platformPermissions],
  )
  const visibleTabs = useMemo(() => tabs.filter((tab) => !tab.permission || canPlatform(tab.permission)), [canPlatform])
  const effectiveActiveTab =
    activeSectionID && visibleTabs.some((tab) => tab.id === activeSectionID)
      ? activeSectionID
      : visibleTabs.some((tab) => tab.id === activeTab)
        ? activeTab
        : visibleTabs[0]?.id
  const activeTabDefinition = tabs.find((tab) => tab.id === effectiveActiveTab)
  const ActiveTabIcon = activeTabDefinition?.icon ?? Database

  function saasModuleLabel(item: SaaSModule): string {
    const key = `saas.module.${item.module_key}`
    const label = t(key)
    return label === key ? item.display_name : label
  }

  function toggleModuleDraft(moduleKeyValue: string) {
    setModuleDraft((current) =>
      current.includes(moduleKeyValue) ? current.filter((item) => item !== moduleKeyValue) : [...current, moduleKeyValue],
    )
  }

  function toggleIndustryModuleDraft(moduleKeyValue: string) {
    setIndustryModuleDraft((current) =>
      current.includes(moduleKeyValue) ? current.filter((item) => item !== moduleKeyValue) : [...current, moduleKeyValue],
    )
  }

  function publicationGates(item: IndustryPublicationRequest): PublicationGateResult[] {
    return item.metadata?.publication_gates ?? []
  }

  async function refreshPackageAssetDiff(request: SchemaChangeRequest) {
    if (!request.schema_package.metadata?.industry_manifest) {
      setPackageAssetDiff([])
      return
    }
    try {
      setPackageAssetDiff(await getSchemaChangePackageDiff(token, request.id))
    } catch {
      setPackageAssetDiff([])
    }
  }

  const loadPlatformPermissions = useCallback(async () => {
    try {
      setPlatformPermissions(await getPlatformPermissionProfile(token))
    } catch (err) {
      setError(err instanceof Error ? err.message : t('systemAdmin.loadFailed'))
    }
  }, [t, token])

  const loadContextHealth = useCallback(async () => {
    if (!canPlatform('assistant.platform.run')) {
      setContextHealth(null)
      return
    }
    setError('')
    try {
      setContextHealth(await getPlatformAssistantContextHealth(token, activeOrganizationID || undefined))
    } catch (err) {
      setError(err instanceof Error ? err.message : t('systemAdmin.loadFailed'))
    }
  }, [activeOrganizationID, canPlatform, t, token])

  const loadMonitoringAgent = useCallback(async () => {
    if (!canPlatform('platform.read')) {
      setMonitoringStatus(null)
      setMonitoringRuns([])
      return
    }
    setLoading(true)
    setError('')
    try {
      const [status, runs] = await Promise.all([
        getMonitoringAgentStatus(token, activeOrganizationID || undefined),
        listMonitoringAgentRuns(token, activeOrganizationID || undefined, 20),
      ])
      setMonitoringStatus(status)
      setMonitoringRuns(runs)
    } catch (err) {
      setError(err instanceof Error ? err.message : t('systemAdmin.loadFailed'))
    } finally {
      setLoading(false)
    }
  }, [activeOrganizationID, canPlatform, t, token])

  const loadSaaSManagement = useCallback(async () => {
    if (!canPlatform('platform.read')) return
    setLoading(true)
    setError('')
    try {
      const [orgItems, moduleItems] = await Promise.all([
        listPlatformOrganizations(token, 100),
        listSaaSModules(token),
      ])
      setPlatformOrganizations(orgItems)
      setSaaSModules(moduleItems)
      const selectableItems = showClosedOrganizations ? orgItems : orgItems.filter((item) => item.status !== 'closed')
      setSelectedOrganizationID((current) =>
        current && selectableItems.some((item) => item.id === current) ? current : selectableItems[0]?.id || '',
      )
    } catch (err) {
      setError(err instanceof Error ? err.message : t('systemAdmin.loadFailed'))
    } finally {
      setLoading(false)
    }
  }, [canPlatform, showClosedOrganizations, t, token])

  const loadFeatureCatalog = useCallback(async () => {
    if (!canPlatform('platform.feature.manage') && !canPlatform('platform.read')) return
    setLoading(true)
    setError('')
    try {
      setPlatformFeatures(await listPlatformFeatures(token, '', 200))
    } catch (err) {
      setError(err instanceof Error ? err.message : t('systemAdmin.loadFailed'))
    } finally {
      setLoading(false)
    }
  }, [canPlatform, t, token])

  const loadRBAC = useCallback(async () => {
    if (!canPlatform('platform.rbac.manage') && !canPlatform('platform.read')) return
    setLoading(true)
    setError('')
    try {
      const [permissionItems, roleItems] = await Promise.all([listPlatformPermissions(token), listPlatformRoles(token)])
      setPlatformPermissionItems(permissionItems)
      setPlatformRoles(roleItems)
      setRolePermissionDraft((current) => {
        const next = { ...current }
        for (const role of roleItems) {
          if (next[role.role_key] === undefined) {
            next[role.role_key] = (role.permissions ?? []).join('\n')
          }
        }
        return next
      })
    } catch (err) {
      setError(err instanceof Error ? err.message : t('systemAdmin.loadFailed'))
    } finally {
      setLoading(false)
    }
  }, [canPlatform, t, token])

  const loadPlatformUsers = useCallback(async () => {
    if (!canPlatform('platform.user.manage')) return
    setLoading(true)
    setError('')
    try {
      setPlatformUsers(await listPlatformUsers(token, 100))
    } catch (err) {
      setError(err instanceof Error ? err.message : t('systemAdmin.loadFailed'))
    } finally {
      setLoading(false)
    }
  }, [canPlatform, t, token])

  const loadDatabaseMaintenance = useCallback(async () => {
    if (!canPlatform('database.maintenance.manage')) return
    setLoading(true)
    setError('')
    try {
      setDatabaseMaintenanceJobs(await listDatabaseMaintenanceJobs(token, 100))
    } catch (err) {
      setError(err instanceof Error ? err.message : t('systemAdmin.loadFailed'))
    } finally {
      setLoading(false)
    }
  }, [canPlatform, t, token])

  const loadOrganizationSaaSDetails = useCallback(async () => {
    if (!activeOrganizationID || selectedOrganization?.status === 'closed' || !canPlatform('organization.manage')) {
      setSubscription(null)
      setEntitlements({})
      setModuleDraft([])
      setInvitations([])
      return
    }
    setLoading(true)
    setError('')
    try {
      const [subscriptionItem, entitlementItems, invitationItems] = await Promise.all([
        getOrganizationSubscription(token, activeOrganizationID).catch(() => null),
        getOrganizationEntitlements(token, activeOrganizationID).catch(() => ({} as Record<string, boolean>)),
        listOrganizationInvitations(token, activeOrganizationID, 100).catch(() => [] as OrganizationInvitation[]),
      ])
      setSubscription(subscriptionItem)
      setEntitlements(entitlementItems)
      setModuleDraft(Object.entries(entitlementItems).filter(([, enabled]) => enabled).map(([key]) => key))
      setInvitations(invitationItems)
    } catch (err) {
      setError(err instanceof Error ? err.message : t('systemAdmin.loadFailed'))
    } finally {
      setLoading(false)
    }
  }, [activeOrganizationID, canPlatform, selectedOrganization?.status, t, token])

  const loadIndustryManagement = useCallback(async () => {
    if (!canPlatform('platform.read')) return
    setLoading(true)
    setError('')
    try {
      const industryItems = await listIndustries(token)
      setIndustries(industryItems)
      const nextIndustryKey = selectedIndustryKey || industryItems[0]?.industry_key || 'general'
      setSelectedIndustryKey(nextIndustryKey)
      const [packageItems, requestItems] = await Promise.all([
        listIndustryPackages(token, nextIndustryKey, 100),
        listIndustryPublicationRequests(token, 100),
      ])
      setIndustryPackages(packageItems)
      const nextPackage = packageItems.find((item) => item.id === selectedPackageID) ?? packageItems[0] ?? null
      setSelectedPackageID(nextPackage?.id || '')
      const packageModules =
        nextPackage?.assets
          .filter((asset) => asset.asset_type === 'module')
          .map((asset) => String(asset.payload.module_key || ''))
          .filter(Boolean) ?? []
      setIndustryModuleDraft(packageModules)
      setPublicationRequests(requestItems)
      if (activeOrganizationID) {
        const [adoptionResult, extensionItems] = await Promise.allSettled([
          getOrganizationIndustry(token, activeOrganizationID),
          listIndustryExtensions(token, activeOrganizationID, 100),
        ])
        setIndustryAdoption(adoptionResult.status === 'fulfilled' ? adoptionResult.value : null)
        setIndustryExtensions(extensionItems.status === 'fulfilled' ? extensionItems.value : [])
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : t('systemAdmin.loadFailed'))
    } finally {
      setLoading(false)
    }
  }, [activeOrganizationID, canPlatform, selectedIndustryKey, selectedPackageID, t, token])

  const loadCatalog = useCallback(async () => {
    if (!canPlatform('platform.read')) return
    setLoading(true)
    setError('')
    try {
      const items = await listPlatformMasters(token, moduleKey, 100)
      setMasters(items)
      setSelectedMasterKey((current) => (items.some((item) => item.master_key === current) ? current : items[0]?.master_key || ''))
      if (items.length === 0) {
        setDetails([])
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : t('systemAdmin.loadFailed'))
    } finally {
      setLoading(false)
    }
  }, [canPlatform, moduleKey, t, token])

  const loadTargets = useCallback(async () => {
    if (!canPlatform('schema.manage')) return
    setLoading(true)
    setError('')
    try {
      setTargets(await listOrganizationSchemaTargets(token, 100))
    } catch (err) {
      setError(err instanceof Error ? err.message : t('systemAdmin.loadFailed'))
    } finally {
      setLoading(false)
    }
  }, [canPlatform, t, token])

  useEffect(() => {
    const timer = window.setTimeout(() => {
      void loadPlatformPermissions()
    }, 0)
    return () => window.clearTimeout(timer)
  }, [loadPlatformPermissions])

  useEffect(() => {
    if (effectiveActiveTab !== 'assistant') return
    const timer = window.setTimeout(() => {
      void loadContextHealth()
    }, 0)
    return () => window.clearTimeout(timer)
  }, [effectiveActiveTab, loadContextHealth])

  useEffect(() => {
    if (effectiveActiveTab !== 'monitoring') return
    const timer = window.setTimeout(() => {
      void loadMonitoringAgent()
    }, 0)
    return () => window.clearTimeout(timer)
  }, [effectiveActiveTab, loadMonitoringAgent])

  useEffect(() => {
    if (!effectiveActiveTab || !['saas', 'industry', 'features', 'schema', 'targets'].includes(effectiveActiveTab)) return
    const timer = window.setTimeout(() => {
      void loadSaaSManagement()
    }, 0)
    return () => window.clearTimeout(timer)
  }, [effectiveActiveTab, loadSaaSManagement])

  useEffect(() => {
    if (effectiveActiveTab !== 'industry') return
    const timer = window.setTimeout(() => {
      void loadIndustryManagement()
    }, 0)
    return () => window.clearTimeout(timer)
  }, [effectiveActiveTab, loadIndustryManagement])

  useEffect(() => {
    if (!effectiveActiveTab || !['saas', 'features'].includes(effectiveActiveTab)) return
    const timer = window.setTimeout(() => {
      void loadOrganizationSaaSDetails()
    }, 0)
    return () => window.clearTimeout(timer)
  }, [effectiveActiveTab, loadOrganizationSaaSDetails])

  useEffect(() => {
    if (effectiveActiveTab !== 'features') return
    const timer = window.setTimeout(() => {
      void loadFeatureCatalog()
    }, 0)
    return () => window.clearTimeout(timer)
  }, [effectiveActiveTab, loadFeatureCatalog])

  useEffect(() => {
    if (effectiveActiveTab !== 'permissions') return
    const timer = window.setTimeout(() => {
      void loadRBAC()
    }, 0)
    return () => window.clearTimeout(timer)
  }, [effectiveActiveTab, loadRBAC])

  useEffect(() => {
    if (effectiveActiveTab !== 'users') return
    const timer = window.setTimeout(() => {
      void loadPlatformUsers()
    }, 0)
    return () => window.clearTimeout(timer)
  }, [effectiveActiveTab, loadPlatformUsers])

  useEffect(() => {
    if (effectiveActiveTab !== 'database') return
    const timer = window.setTimeout(() => {
      void loadDatabaseMaintenance()
    }, 0)
    return () => window.clearTimeout(timer)
  }, [effectiveActiveTab, loadDatabaseMaintenance])

  useEffect(() => {
    const timer = window.setTimeout(() => {
      setOrganizationProfileForm({
        name: selectedOrganization?.name ?? '',
        description: selectedOrganization?.description ?? '',
      })
    }, 0)
    return () => window.clearTimeout(timer)
  }, [selectedOrganization?.description, selectedOrganization?.name])

  useEffect(() => {
    const timer = window.setTimeout(() => {
      void loadCatalog()
    }, 0)
    return () => window.clearTimeout(timer)
  }, [loadCatalog])

  useEffect(() => {
    const timer = window.setTimeout(() => {
      void loadTargets()
    }, 0)
    return () => window.clearTimeout(timer)
  }, [loadTargets])

  useEffect(() => {
    if (!selectedMasterKey || !canPlatform('platform.read')) {
      return
    }
    let cancelled = false
    listPlatformDetails(token, selectedMasterKey)
      .then((items) => {
        if (!cancelled) setDetails(items)
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof Error ? err.message : t('systemAdmin.loadFailed'))
      })
    return () => {
      cancelled = true
    }
  }, [canPlatform, selectedMasterKey, t, token])

  async function run(action: () => Promise<void>, successKey: string) {
    setLoading(true)
    setError('')
    setNotice('')
    try {
      await action()
      setNotice(t(successKey))
      await loadTargets()
    } catch (err) {
      setError(err instanceof Error ? err.message : t('common.operationFailed'))
    } finally {
      setLoading(false)
    }
  }

  async function runSaaS(action: () => Promise<void>, successKey: string) {
    setLoading(true)
    setError('')
    setNotice('')
    try {
      await action()
      setNotice(t(successKey))
      await loadOrganizationSaaSDetails()
    } catch (err) {
      setError(err instanceof Error ? err.message : t('common.operationFailed'))
    } finally {
      setLoading(false)
    }
  }

  async function saveOrganizationModules() {
    if (!activeOrganizationID || !canPlatform('organization.manage')) return
    await runSaaS(async () => {
      const updated = await updateOrganizationModules(token, activeOrganizationID, moduleDraft)
      setEntitlements(updated)
      setModuleDraft(Object.entries(updated).filter(([, enabled]) => enabled).map(([key]) => key))
    }, 'systemAdmin.modulesUpdated')
  }

  async function saveOrganizationProfile() {
    if (!activeOrganizationID || !organizationProfileForm.name.trim() || !canPlatform('organization.manage')) return
    await runSaaS(async () => {
      await updatePlatformOrganizationProfile(token, activeOrganizationID, {
        name: organizationProfileForm.name.trim(),
        description: organizationProfileForm.description.trim(),
      })
      await loadSaaSManagement()
    }, 'systemAdmin.organizationProfileUpdated')
  }

  async function runMonitoringScan() {
    if (!canPlatform('platform.read')) return
    setLoading(true)
    setError('')
    setNotice('')
    try {
      await runMonitoringAgent(token, {
        organization_id: activeOrganizationID || undefined,
        lookback_hours: monitoringStatus?.lookback_hours,
      })
      setNotice(t('systemAdmin.monitoringRunCreated'))
      await loadMonitoringAgent()
    } catch (err) {
      setError(err instanceof Error ? err.message : t('common.operationFailed'))
    } finally {
      setLoading(false)
    }
  }

  async function applySelectedIndustryPackage() {
    if (!activeOrganizationID || !selectedIndustryPackage) return
    await run(async () => {
      const adoption = await applyIndustryPackageToOrganization(token, selectedIndustryPackage.id, activeOrganizationID, industryModuleDraft)
      setIndustryAdoption(adoption)
      await loadIndustryManagement()
    }, 'systemAdmin.industryApplied')
  }

  async function createERPSolutionFlow() {
    if (!activeOrganizationID || !canPlatform('schema.manage')) return
    await run(async () => {
      const request = await createERPStandardSolutionFlow(token, activeOrganizationID, {
        industry_key: selectedIndustryKey || 'standard_erp',
        package_key: 'erp_standard',
        name: 'ERP Standard',
        enabled_modules: erpSolutionModuleDraft,
      })
      setChangeRequest(request)
      setVerificationReport(null)
      setPackageAssetDiff(await getSchemaChangePackageDiff(token, request.id))
      setSchemaPackage(request.schema_package)
      setSchemaJson(jsonText(request.schema_package))
      setActiveTab('schema')
    }, 'systemAdmin.erpSolutionCreated')
  }

  async function createIndustryTableFieldChange() {
    if (!activeOrganizationID || !canPlatform('industry.solution.manage') || !canPlatform('schema.manage')) return
    if (!industryTableDraft.tableName.trim() || !industryTableDraft.fieldName.trim() || !industryTableDraft.dataType.trim()) return
    await run(async () => {
      const request = await createIndustrySolutionSchemaChange(token, activeOrganizationID, {
        industry_key: selectedIndustryKey || 'general',
        package_key: selectedIndustryPackage?.package_key || 'custom',
        reason,
        table: {
          name: industryTableDraft.tableName.trim(),
          display_name: industryTableDraft.displayName.trim(),
          fields: [
            {
              name: industryTableDraft.fieldName.trim(),
              data_type: industryTableDraft.dataType.trim(),
              nullable: industryTableDraft.nullable,
              default: industryTableDraft.defaultValue.trim(),
            },
          ],
        },
      })
      setChangeRequest(request)
      setVerificationReport(null)
      setPackageAssetDiff(await getSchemaChangePackageDiff(token, request.id).catch(() => []))
      setSchemaPackage(request.schema_package)
      setSchemaJson(jsonText(request.schema_package))
    }, 'systemAdmin.industrySchemaChangeCreated')
  }

  async function createFeature() {
    if (!featureDraft.featureKey.trim() || !featureDraft.moduleKey.trim() || !featureDraft.title.trim()) return
    await run(async () => {
      await createPlatformFeature(token, {
        feature_key: featureDraft.featureKey.trim(),
        module_key: featureDraft.moduleKey.trim(),
        title: featureDraft.title.trim(),
        permission_keys: splitLines(featureDraft.permissionKeys),
        metadata: { extension_mode: 'metadata_only' },
      })
      setFeatureDraft({ featureKey: '', moduleKey: 'platform_admin', title: '', permissionKeys: 'platform.read' })
      await loadFeatureCatalog()
    }, 'systemAdmin.featureCreated')
  }

  async function publishFeature(featureKey: string) {
    await run(async () => {
      await publishPlatformFeature(token, featureKey)
      await loadFeatureCatalog()
    }, 'systemAdmin.featurePublished')
  }

  async function saveRolePermissions(roleKey: string) {
    await run(async () => {
      await setPlatformRolePermissions(token, roleKey, splitLines(rolePermissionDraft[roleKey] ?? ''))
      await loadRBAC()
    }, 'systemAdmin.rolePermissionsSaved')
  }

  async function createUser() {
    if (!platformUserDraft.name.trim() || !platformUserDraft.email.trim()) return
    await run(async () => {
      const result = await createPlatformUser(token, {
        name: platformUserDraft.name.trim(),
        email: platformUserDraft.email.trim(),
        roles: splitLines(platformUserDraft.roles),
      })
      setTemporaryCredential(result.temporary_password)
      setPlatformUserDraft({ name: '', email: '', roles: 'operator' })
      await loadPlatformUsers()
    }, 'systemAdmin.platformUserCreated')
  }

  async function resetUserPassword(userID: string) {
    await run(async () => {
      const result = await resetPlatformUserPassword(token, userID)
      setTemporaryCredential(result.temporary_password)
      await loadPlatformUsers()
    }, 'systemAdmin.platformPasswordReset')
  }

  async function disableUser(userID: string) {
    await run(async () => {
      await disablePlatformUser(token, userID)
      await loadPlatformUsers()
    }, 'systemAdmin.platformUserDisabled')
  }

  async function createMaintenanceJob() {
    if (!maintenanceDraft.reason.trim()) return
    await run(async () => {
      await createDatabaseMaintenanceJob(token, {
        job_type: maintenanceDraft.jobType as 'backup' | 'restore',
        scope: maintenanceDraft.scope,
        reason: maintenanceDraft.reason.trim(),
        backup_ref: maintenanceDraft.backupRef.trim(),
      })
      setMaintenanceDraft({ jobType: 'backup', scope: 'platform', reason: '', backupRef: '' })
      await loadDatabaseMaintenance()
    }, 'systemAdmin.databaseJobCreated')
  }

  async function reviewMaintenanceJob(jobID: string, decision: 'approve' | 'reject') {
    await run(async () => {
      await reviewDatabaseMaintenanceJob(token, jobID, decision, reason)
      await loadDatabaseMaintenance()
    }, decision === 'approve' ? 'systemAdmin.databaseJobApproved' : 'systemAdmin.databaseJobRejected')
  }

  async function createPrivateIndustryExtension() {
    if (!activeOrganizationID || !selectedIndustryPackage || !extensionKey.trim() || !extensionName.trim() || !extensionModuleKey.trim()) return
    await run(async () => {
      await createIndustryExtension(token, {
        organization_id: activeOrganizationID,
        industry_key: selectedIndustryKey,
        package_id: selectedIndustryPackage.id,
        extension_key: extensionKey.trim(),
        name: extensionName.trim(),
        assets: [
          {
            asset_key: `${extensionModuleKey.trim()}-module`,
            asset_type: 'module',
            payload: {
              module_key: extensionModuleKey.trim(),
              display_name: extensionName.trim(),
            },
          },
        ],
      })
      setExtensionKey('')
      setExtensionName('')
      setExtensionModuleKey('')
      await loadIndustryManagement()
    }, 'systemAdmin.extensionCreated')
  }

  async function submitExtensionPublication(extensionID: string) {
    await run(async () => {
      await submitIndustryExtensionPublication(token, extensionID, publicationReason)
      setPublicationReason('')
      await loadIndustryManagement()
    }, 'systemAdmin.publicationSubmitted')
  }

  async function reviewPublication(requestID: string, action: 'approve' | 'reject') {
    await run(async () => {
      await reviewIndustryPublicationRequest(token, requestID, action, publicationReason)
      setPublicationReason('')
      await loadIndustryManagement()
    }, action === 'approve' ? 'systemAdmin.publicationApproved' : 'systemAdmin.publicationRejected')
  }

  async function submitInvitation() {
    if (!activeOrganizationID || !inviteEmail.trim() || !canPlatform('organization.manage')) return
    await runSaaS(async () => {
      await createOrganizationInvitation(token, activeOrganizationID, {
        email: inviteEmail.trim(),
        name: inviteName.trim() || undefined,
        authority_tier: inviteAuthority,
      })
      setInviteEmail('')
      setInviteName('')
    }, 'systemAdmin.invitationCreated')
  }

  async function closeOrganization() {
    if (!activeOrganizationID || !canPlatform('organization.close')) return
    setLoading(true)
    setError('')
    setNotice('')
    try {
      const closed = await closePlatformOrganization(token, activeOrganizationID, closeReason)
      const orgItems = await listPlatformOrganizations(token, 100)
      setPlatformOrganizations(orgItems.map((item) => (item.id === closed.id ? closed : item)))
      const selectableItems = orgItems.filter((item) => item.id !== closed.id && item.status !== 'closed')
      setSelectedOrganizationID(selectableItems[0]?.id || '')
      setSubscription(null)
      setEntitlements({})
      setModuleDraft([])
      setInvitations([])
      setCloseReason('')
      setNotice(t('systemAdmin.organizationClosed'))
    } catch (err) {
      setError(err instanceof Error ? err.message : t('common.operationFailed'))
    } finally {
      setLoading(false)
    }
  }

  async function exportSchema() {
    if (!activeOrganizationID || !canPlatform('schema.manage')) return
    await run(async () => {
      const pkg = await exportOrganizationSchema(token, activeOrganizationID)
      setSchemaPackage(pkg)
      setSchemaJson(jsonText(pkg))
      setVerificationReport(null)
      setPackageAssetDiff([])
    }, 'systemAdmin.schemaExported')
  }

  function downloadSchema() {
    if (!schemaJson.trim()) return
    const blob = new Blob([schemaJson], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `${selectedTarget?.schema_name || 'organization-schema'}.json`
    link.click()
    URL.revokeObjectURL(url)
  }

  async function importSchemaFile(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0]
    if (!file) return
    try {
      const content = await file.text()
      const pkg = parseSchemaPackage(content)
      setSchemaPackage(pkg)
      setSchemaJson(jsonText(pkg))
      setVerificationReport(null)
      setPackageAssetDiff([])
      setNotice(t('systemAdmin.jsonImported'))
      setError('')
    } catch {
      setError(t('systemAdmin.invalidJson'))
    } finally {
      event.target.value = ''
    }
  }

  async function createChange() {
    if (!activeOrganizationID || !canPlatform('schema.manage')) return
    let pkg: SchemaPackage
    try {
      pkg = parseSchemaPackage(schemaJson)
    } catch {
      setError(t('systemAdmin.invalidJson'))
      return
    }
    await run(async () => {
      const request = await createOrganizationSchemaChange(token, activeOrganizationID, {
        request_type: 'schema_package_update',
        reason,
        schema_package: pkg,
      })
      setSchemaPackage(pkg)
      setChangeRequest(request)
      setApplyJob(null)
      setVerificationReport(null)
      await refreshPackageAssetDiff(request)
    }, 'systemAdmin.changeCreated')
  }

  async function approveChange() {
    if (!changeRequest || !canPlatform('schema.approve')) return
    await run(async () => {
      setChangeRequest(await approveSchemaChange(token, changeRequest.id, reason))
      setVerificationReport(null)
    }, 'systemAdmin.changeApproved')
  }

  async function verifyChange() {
    if (!changeRequest || !canPlatform('schema.manage')) return
    await run(async () => {
      setVerificationReport(await verifySchemaChange(token, changeRequest.id))
      await refreshPackageAssetDiff(changeRequest)
    }, 'systemAdmin.changeVerified')
  }

  async function applyChange() {
    if (!changeRequest || verificationBlocksApply || !canPlatform('schema.apply')) return
    await run(async () => {
      const job = await applySchemaChange(token, changeRequest.id)
      setApplyJob(job)
      setVerificationReport(null)
    }, 'systemAdmin.changeApplied')
  }

  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-center gap-3 rounded-lg border border-slate-200 bg-white p-3 shadow-sm">
        {activeTabDefinition && (
          <div className="flex min-w-0 items-center gap-2">
            <ActiveTabIcon className="h-5 w-5 text-slate-500" />
            <h2 className="truncate text-base font-semibold text-slate-950">{t(activeTabDefinition.label)}</h2>
          </div>
        )}
        <button
          type="button"
          onClick={() => {
            void loadPlatformPermissions()
            void loadSaaSManagement()
            void loadOrganizationSaaSDetails()
            void loadIndustryManagement()
            void loadContextHealth()
            void loadMonitoringAgent()
            void loadFeatureCatalog()
            void loadRBAC()
            void loadPlatformUsers()
            void loadDatabaseMaintenance()
            void loadCatalog()
            void loadTargets()
          }}
          disabled={loading}
          className="ml-auto inline-flex h-10 items-center gap-2 rounded-md border border-slate-300 px-3 text-sm font-semibold text-slate-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50"
        >
          <RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
          {t('common.refresh')}
        </button>
      </div>

      {(notice || error) && (
        <p className={`rounded-lg border px-4 py-3 text-sm ${error ? 'border-red-200 bg-red-50 text-red-700' : 'border-emerald-200 bg-emerald-50 text-emerald-700'}`}>
          {error || notice}
        </p>
      )}

      {effectiveActiveTab === 'assistant' && (
        <div className="grid gap-5 xl:grid-cols-[360px_1fr]">
          <ContextHealthPanel health={contextHealth} />
          <section className="rounded-lg border border-slate-200 bg-white shadow-sm">
            <div className="flex items-center justify-between gap-3 border-b border-slate-200 px-5 py-4">
              <div>
                <h2 className="text-base font-semibold text-slate-950">{t('systemAdmin.platformAssistant')}</h2>
                <p className="mt-1 text-sm text-slate-500">{t('systemAdmin.platformAssistantSummary')}</p>
              </div>
              <Bot className="h-5 w-5 text-slate-500" />
            </div>
            <div className="h-[680px] min-h-0 overflow-hidden">
              <AIAssistant
                token={token}
                contextType="platform_admin"
                autoModel
                hideModelSelector
                apiScope="platform"
                className="h-full"
              />
            </div>
          </section>
        </div>
      )}

      {effectiveActiveTab === 'monitoring' && (
        <div className="grid gap-5 xl:grid-cols-[360px_1fr]">
          <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
            <div className="flex items-start justify-between gap-3">
              <div>
                <h2 className="text-base font-semibold text-slate-950">{t('systemAdmin.monitoringAgent')}</h2>
                <p className="mt-1 text-sm text-slate-500">{t('systemAdmin.monitoringAgentSummary')}</p>
              </div>
              <Activity className="h-5 w-5 text-slate-500" />
            </div>
            <div className="mt-4 grid gap-2">
              <Metric label={t('systemAdmin.scheduler')} value={monitoringStatus?.scheduler_enabled ? t('common.yes') : t('common.no')} />
              <Metric label={t('systemAdmin.dailyTime')} value={monitoringStatus?.daily_time || '02:00'} />
              <Metric label={t('systemAdmin.lookbackHours')} value={String(monitoringStatus?.lookback_hours ?? 24)} />
              <Metric label={t('systemAdmin.maxSignalsPerRun')} value={String(monitoringStatus?.max_signals_per_run ?? 100)} />
            </div>
            <button
              type="button"
              onClick={() => void runMonitoringScan()}
              disabled={loading || !canPlatform('platform.read')}
              className="mt-4 inline-flex h-10 w-full items-center justify-center gap-2 rounded-md bg-[#AD4714] px-3 text-sm font-semibold text-[#fffaf5] transition hover:bg-[#B84F18] disabled:cursor-not-allowed disabled:opacity-50"
            >
              <Play className="h-4 w-4" />
              {t('systemAdmin.runMonitoringScan')}
            </button>
            {monitoringStatus?.latest_run && (
              <div className="mt-4 rounded-lg border border-slate-200 bg-slate-50 p-3">
                <div className="flex items-center justify-between gap-2">
                  <p className="text-xs font-semibold text-slate-500">{t('systemAdmin.latestRun')}</p>
                  <StatusBadge label={monitoringStatus.latest_run.status} />
                </div>
                <div className="mt-3 grid gap-2">
                  <Metric label={t('systemAdmin.signalsCreated')} value={String(monitoringStatus.latest_run.signals_created)} />
                  <Metric label={t('systemAdmin.duplicatesSuppressed')} value={String(monitoringStatus.latest_run.duplicates_suppressed)} />
                  <Metric label={t('systemAdmin.contextProposalsCreated')} value={String(summaryNumber(monitoringStatus.latest_run.summary, 'context_proposals_created'))} />
                  <Metric label={t('systemAdmin.startedAt')} value={formatDateTime(monitoringStatus.latest_run.started_at)} />
                </div>
              </div>
            )}
          </section>

          <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
            <div className="flex items-start justify-between gap-3">
              <div>
                <h2 className="text-base font-semibold text-slate-950">{t('systemAdmin.monitoringRuns')}</h2>
                <p className="mt-1 text-sm text-slate-500">{t('systemAdmin.monitoringRunsSummary')}</p>
              </div>
              <RefreshCw className={`h-5 w-5 text-slate-500 ${loading ? 'animate-spin' : ''}`} />
            </div>
            <div className="mt-4 space-y-3">
              {monitoringRuns.length > 0 ? (
                monitoringRuns.map((run) => {
                  const categories = categoryEntries(run.summary)
                  return (
                    <div key={run.id} className="rounded-lg border border-slate-200 bg-slate-50 p-4">
                      <div className="flex flex-wrap items-start justify-between gap-3">
                        <div className="min-w-0">
                          <p className="truncate text-sm font-semibold text-slate-950">{run.id}</p>
                          <p className="mt-1 text-xs text-slate-500">
                            {t(`systemAdmin.monitoringTrigger.${run.trigger_type}`)} / {formatDateTime(run.started_at)}
                          </p>
                        </div>
                        <StatusBadge label={run.status} />
                      </div>
                      <div className="mt-3 grid gap-2 sm:grid-cols-4">
                        <Metric label={t('systemAdmin.signalsCreated')} value={String(run.signals_created)} />
                        <Metric label={t('systemAdmin.duplicatesSuppressed')} value={String(run.duplicates_suppressed)} />
                        <Metric label={t('systemAdmin.totalFindings')} value={String(summaryNumber(run.summary, 'total_findings'))} />
                        <Metric label={t('systemAdmin.contextProposalsCreated')} value={String(summaryNumber(run.summary, 'context_proposals_created'))} />
                      </div>
                      {categories.length > 0 && (
                        <div className="mt-3 flex flex-wrap gap-1.5">
                          {categories.map(([category, count]) => (
                            <span key={category} className="inline-flex items-center gap-1 rounded-md border border-slate-200 bg-white px-2 py-1 text-xs text-slate-700">
                              <span className="font-semibold">{category}</span>
                              <span>{count}</span>
                            </span>
                          ))}
                        </div>
                      )}
                      {run.error_message && <p className="mt-3 rounded-md bg-red-50 p-2 text-xs text-red-700">{run.error_message}</p>}
                    </div>
                  )
                })
              ) : (
                <p className="rounded-lg border border-dashed border-slate-300 p-4 text-sm text-slate-500">{t('systemAdmin.noMonitoringRuns')}</p>
              )}
            </div>
          </section>
        </div>
      )}

      {effectiveActiveTab === 'saas' && (
        <div className="grid gap-5 xl:grid-cols-[360px_1fr]">
          <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
            <div className="flex items-center justify-between gap-3">
              <div>
                <h2 className="text-base font-semibold text-slate-950">{t('systemAdmin.saasOrganizations')}</h2>
                <p className="mt-1 text-sm text-slate-500">{t('systemAdmin.saasOrganizationsSummary')}</p>
              </div>
              <Users className="h-5 w-5 text-slate-500" />
            </div>
            <label className="mt-4 flex items-center gap-2 text-sm font-semibold text-slate-600">
              <input
                type="checkbox"
                checked={showClosedOrganizations}
                onChange={(event) => setShowClosedOrganizations(event.target.checked)}
                className="h-4 w-4 rounded border-slate-300 text-[#AD4714] focus:ring-[#DF6A24]"
              />
              {t('systemAdmin.showClosedOrganizations')}
            </label>
            <div className="mt-4 space-y-2">
              {visibleManagementOrganizations.length > 0 ? (
                visibleManagementOrganizations.map((item) => (
                  <button
                    key={item.id}
                    type="button"
                    onClick={() => setSelectedOrganizationID(item.id)}
                    className={`w-full rounded-lg border px-3 py-3 text-left transition ${
                      activeOrganizationID === item.id
                        ? 'border-slate-900 bg-slate-950 text-white'
                        : 'border-slate-200 bg-white text-slate-900 hover:border-slate-300 hover:bg-slate-50'
                    }`}
                  >
                    <span className="flex min-w-0 items-center justify-between gap-2">
                      <span className="block truncate text-sm font-semibold">{item.name}</span>
                      {item.status && <StatusBadge label={item.status} />}
                    </span>
                    <span className="mt-1 block truncate text-xs opacity-75">{item.authority_tier || item.id}</span>
                  </button>
                ))
              ) : (
                <p className="rounded-lg border border-dashed border-slate-300 p-4 text-sm text-slate-500">{t('systemAdmin.noOrganizations')}</p>
              )}
            </div>
          </section>

          <section className="space-y-5">
            <div className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <h2 className="text-base font-semibold text-slate-950">{selectedOrganization?.name || t('common.notSelected')}</h2>
                  <p className="mt-1 text-sm text-slate-500">{activeOrganizationID || t('common.notSelected')}</p>
                </div>
                {(selectedOrganization?.status || subscription?.status) && <StatusBadge label={selectedOrganization?.status || subscription?.status || ''} />}
              </div>
              <div className="mt-4 grid gap-3 md:grid-cols-3">
                <Metric label={t('systemAdmin.subscription')} value={subscription?.plan_name || subscription?.plan_code || t('common.notSelected')} />
                <Metric label={t('systemAdmin.status')} value={selectedOrganization?.status ? t(selectedOrganization.status) : subscription?.status ? t(subscription.status) : t('common.notSelected')} />
                <Metric label={t('systemAdmin.enabledModules')} value={String(Object.values(entitlements).filter(Boolean).length)} />
              </div>
              <div className="mt-4 grid gap-3 md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto]">
                <input
                  value={organizationProfileForm.name}
                  onChange={(event) => setOrganizationProfileForm((current) => ({ ...current, name: event.target.value }))}
                  placeholder={t('systemAdmin.organizationName')}
                  disabled={!activeOrganizationID || selectedOrganization?.status === 'closed' || loading || !canPlatform('organization.manage')}
                  className="h-10 rounded-lg border border-slate-300 px-3 text-sm text-slate-900 outline-none focus:border-[#AD4714] focus:ring-2 focus:ring-[#DF6A24]/20 disabled:cursor-not-allowed disabled:bg-slate-50 disabled:text-slate-400"
                />
                <input
                  value={organizationProfileForm.description}
                  onChange={(event) => setOrganizationProfileForm((current) => ({ ...current, description: event.target.value }))}
                  placeholder={t('systemAdmin.organizationDescription')}
                  disabled={!activeOrganizationID || selectedOrganization?.status === 'closed' || loading || !canPlatform('organization.manage')}
                  className="h-10 rounded-lg border border-slate-300 px-3 text-sm text-slate-900 outline-none focus:border-[#AD4714] focus:ring-2 focus:ring-[#DF6A24]/20 disabled:cursor-not-allowed disabled:bg-slate-50 disabled:text-slate-400"
                />
                <button
                  type="button"
                  onClick={() => void saveOrganizationProfile()}
                  disabled={!activeOrganizationID || selectedOrganization?.status === 'closed' || !organizationProfileForm.name.trim() || loading || !canPlatform('organization.manage')}
                  className="inline-flex h-10 items-center justify-center gap-2 rounded-md border border-slate-300 px-3 text-sm font-semibold text-slate-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <Check className="h-4 w-4" />
                  {t('systemAdmin.saveOrganizationProfile')}
                </button>
              </div>
              <div className="mt-4 grid gap-3 md:grid-cols-[1fr_auto]">
                <input
                  value={closeReason}
                  onChange={(event) => setCloseReason(event.target.value)}
                  placeholder={t('systemAdmin.closeOrganizationReason')}
                  disabled={!activeOrganizationID || selectedOrganization?.status === 'closed' || loading || !canPlatform('organization.close')}
                  className="h-10 rounded-lg border border-slate-300 px-3 text-sm text-slate-900 outline-none focus:border-[#AD4714] focus:ring-2 focus:ring-[#DF6A24]/20 disabled:cursor-not-allowed disabled:bg-slate-50 disabled:text-slate-400"
                />
                <button
                  type="button"
                  onClick={() => void closeOrganization()}
                  disabled={!activeOrganizationID || selectedOrganization?.status === 'closed' || loading || !canPlatform('organization.close')}
                  className="inline-flex h-10 items-center justify-center gap-2 rounded-md border border-red-200 bg-red-50 px-3 text-sm font-semibold text-red-700 transition hover:bg-red-100 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <ShieldCheck className="h-4 w-4" />
                  {t('systemAdmin.closeOrganization')}
                </button>
              </div>
            </div>

            <div className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <h2 className="text-base font-semibold text-slate-950">{t('systemAdmin.entitlements')}</h2>
                  <p className="mt-1 text-sm text-slate-500">{t('systemAdmin.enabledModules')}</p>
                </div>
                <button
                  type="button"
                  onClick={() => void saveOrganizationModules()}
                  disabled={!activeOrganizationID || selectedOrganization?.status === 'closed' || loading || !canPlatform('organization.manage')}
                  className="inline-flex h-9 items-center gap-2 rounded-md bg-[#AD4714] px-3 text-sm font-semibold text-[#fffaf5] transition hover:bg-[#B84F18] disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <Check className="h-4 w-4" />
                  {t('systemAdmin.updateModules')}
                </button>
              </div>
              <div className="mt-4 grid gap-2 md:grid-cols-2 xl:grid-cols-3">
                {saasModules.length > 0 ? (
                  saasModules.map((item) => (
                    <label key={item.module_key} className="flex min-h-[64px] items-start gap-3 rounded-lg border border-slate-200 bg-slate-50 p-3">
                      <input
                        type="checkbox"
                        checked={moduleDraft.includes(item.module_key)}
                        onChange={() => toggleModuleDraft(item.module_key)}
                        disabled={selectedOrganization?.status === 'closed' || !canPlatform('organization.manage')}
                        className="mt-1 h-4 w-4 rounded border-slate-300 text-[#AD4714] focus:ring-[#DF6A24]"
                      />
                      <span className="min-w-0">
                        <span className="block truncate text-sm font-semibold text-slate-900">{saasModuleLabel(item)}</span>
                        <span className="mt-1 block truncate text-xs text-slate-500">{item.category} / {item.license_scope}</span>
                      </span>
                    </label>
                  ))
                ) : (
                  <p className="rounded-lg border border-dashed border-slate-300 p-4 text-sm text-slate-500 md:col-span-2 xl:col-span-3">{t('common.noData')}</p>
                )}
              </div>
            </div>

            <div className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <h2 className="text-base font-semibold text-slate-950">{t('systemAdmin.invitations')}</h2>
                  <p className="mt-1 text-sm text-slate-500">{selectedOrganization?.name || t('common.notSelected')}</p>
                </div>
                <button
                  type="button"
                  onClick={() => void submitInvitation()}
                  disabled={!activeOrganizationID || selectedOrganization?.status === 'closed' || !inviteEmail.trim() || loading || !canPlatform('organization.manage')}
                  className="inline-flex h-9 items-center gap-2 rounded-md border border-slate-300 px-3 text-sm font-semibold text-slate-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <Send className="h-4 w-4" />
                  {t('systemAdmin.createInvitation')}
                </button>
              </div>
              <div className="mt-4 grid gap-3 md:grid-cols-3">
                <input
                  value={inviteEmail}
                  onChange={(event) => setInviteEmail(event.target.value)}
                  placeholder={t('systemAdmin.inviteEmail')}
                  type="email"
                  disabled={selectedOrganization?.status === 'closed' || !canPlatform('organization.manage')}
                  className="h-10 rounded-lg border border-slate-300 px-3 text-sm text-slate-900 outline-none focus:border-[#AD4714] focus:ring-2 focus:ring-[#DF6A24]/20"
                />
                <input
                  value={inviteName}
                  onChange={(event) => setInviteName(event.target.value)}
                  placeholder={t('systemAdmin.inviteName')}
                  disabled={selectedOrganization?.status === 'closed' || !canPlatform('organization.manage')}
                  className="h-10 rounded-lg border border-slate-300 px-3 text-sm text-slate-900 outline-none focus:border-[#AD4714] focus:ring-2 focus:ring-[#DF6A24]/20"
                />
                <select
                  value={inviteAuthority}
                  onChange={(event) => setInviteAuthority(event.target.value)}
                  aria-label={t('systemAdmin.inviteAuthority')}
                  disabled={selectedOrganization?.status === 'closed' || !canPlatform('organization.manage')}
                  className="h-10 rounded-lg border border-slate-300 bg-white px-3 text-sm text-slate-900 outline-none focus:border-[#AD4714] focus:ring-2 focus:ring-[#DF6A24]/20"
                >
                  {['organization_admin', 'reviewer', 'executor'].map((item) => (
                    <option key={item} value={item}>
                      {t(item)}
                    </option>
                  ))}
                </select>
              </div>
              <div className="mt-5 overflow-hidden rounded-lg border border-slate-200">
                <table className="min-w-full divide-y divide-slate-200 text-sm">
                  <thead className="bg-slate-50 text-left text-xs font-semibold uppercase text-slate-500">
                    <tr>
                      <th className="px-3 py-2">{t('systemAdmin.inviteEmail')}</th>
                      <th className="px-3 py-2">{t('common.name')}</th>
                      <th className="px-3 py-2">{t('systemAdmin.inviteAuthority')}</th>
                      <th className="px-3 py-2">{t('systemAdmin.status')}</th>
                      <th className="px-3 py-2">{t('systemAdmin.updated')}</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-100">
                    {invitations.length > 0 ? (
                      invitations.map((item) => (
                        <tr key={item.id}>
                          <td className="px-3 py-3 font-medium text-slate-900">{item.email}</td>
                          <td className="px-3 py-3 text-slate-600">{item.name || t('common.none')}</td>
                          <td className="px-3 py-3 text-slate-600">{t(item.authority_tier)}</td>
                          <td className="px-3 py-3">
                            <StatusBadge label={item.status} />
                          </td>
                          <td className="px-3 py-3 text-slate-500">{formatDateTime(item.updated_at)}</td>
                        </tr>
                      ))
                    ) : (
                      <tr>
                        <td colSpan={5} className="px-3 py-6 text-center text-slate-500">
                          {t('systemAdmin.noInvitations')}
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            </div>
          </section>
        </div>
      )}

      {effectiveActiveTab === 'industry' && (
        <div className="grid gap-5 xl:grid-cols-[360px_1fr]">
          <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
            <div className="flex items-center justify-between gap-3">
              <div>
                <h2 className="text-base font-semibold text-slate-950">{t('systemAdmin.industries')}</h2>
                <p className="mt-1 text-sm text-slate-500">{t('systemAdmin.industrySummary')}</p>
              </div>
              <Layers3 className="h-5 w-5 text-slate-500" />
            </div>
            <div className="mt-4 space-y-3">
              <label className="block">
                <span className="text-xs font-semibold text-slate-500">{t('systemAdmin.industry')}</span>
                <select
                  value={selectedIndustryKey}
                  onChange={(event) => {
                    setSelectedIndustryKey(event.target.value)
                    setSelectedPackageID('')
                  }}
                  className="mt-1 h-10 w-full rounded-md border border-slate-300 bg-white px-3 text-sm"
                >
                  {industries.map((item) => (
                    <option key={item.industry_key} value={item.industry_key}>
                      {item.name}
                    </option>
                  ))}
                </select>
              </label>
              <label className="block">
                <span className="text-xs font-semibold text-slate-500">{t('systemAdmin.industryPackage')}</span>
                <select
                  value={selectedPackageID}
                  onChange={(event) => {
                    const nextID = event.target.value
                    setSelectedPackageID(nextID)
                    const nextPackage = industryPackages.find((item) => item.id === nextID)
                    setIndustryModuleDraft(
                      nextPackage?.assets
                        .filter((asset) => asset.asset_type === 'module')
                        .map((asset) => String(asset.payload.module_key || ''))
                        .filter(Boolean) ?? [],
                    )
                  }}
                  className="mt-1 h-10 w-full rounded-md border border-slate-300 bg-white px-3 text-sm"
                >
                  {industryPackages.map((item) => (
                    <option key={item.id} value={item.id}>
                      {item.name} v{item.version}
                    </option>
                  ))}
                </select>
              </label>
              <div className="grid grid-cols-2 gap-2">
                <Metric label={t('systemAdmin.packageAssets')} value={String(selectedIndustryPackage?.assets.length ?? 0)} />
                <Metric label={t('systemAdmin.packageModules')} value={String(selectedIndustryPackageModules.length)} />
              </div>
              <div className="space-y-2">
                <span className="text-xs font-semibold text-slate-500">{t('systemAdmin.enabledModules')}</span>
                {selectedIndustryPackageModules.length === 0 ? (
                  <p className="rounded-lg border border-dashed border-slate-300 p-4 text-sm text-slate-500">{t('systemAdmin.noPackages')}</p>
                ) : (
                  selectedIndustryPackageModules.map((moduleKeyValue) => (
                    <label key={moduleKeyValue} className="flex items-center gap-2 rounded-md border border-slate-200 px-3 py-2 text-sm">
                      <input
                        type="checkbox"
                        checked={industryModuleDraft.includes(moduleKeyValue)}
                        onChange={() => toggleIndustryModuleDraft(moduleKeyValue)}
                      />
                      <span className="font-medium text-slate-800">{t(`saas.module.${moduleKeyValue}`)}</span>
                    </label>
                  ))
                )}
              </div>
              <button
                type="button"
                onClick={() => void applySelectedIndustryPackage()}
                disabled={!activeOrganizationID || !selectedIndustryPackage || loading}
                className="inline-flex h-10 w-full items-center justify-center gap-2 rounded-md bg-slate-950 px-4 text-sm font-semibold text-white disabled:cursor-not-allowed disabled:opacity-50"
              >
                <Check className="h-4 w-4" />
                {t('systemAdmin.applyIndustry')}
              </button>
              <div className="rounded-lg border border-[#F1D7C7] bg-[#fff8f3] p-4">
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <h3 className="text-sm font-semibold text-slate-950">{t('systemAdmin.erpSolutionFlow')}</h3>
                    <p className="mt-1 text-xs text-slate-600">{t('systemAdmin.erpSolutionSummary')}</p>
                  </div>
                  <Database className="h-5 w-5 text-[#AD4714]" />
                </div>
                <div className="mt-3 space-y-2">
                  <span className="text-xs font-semibold text-slate-500">{t('systemAdmin.enabledModules')}</span>
                  <div className="grid grid-cols-2 gap-2">
                    {erpSolutionModules.map((moduleKeyValue) => (
                      <label key={moduleKeyValue} className="flex items-center gap-2 rounded-md border border-[#F1D7C7] bg-white px-3 py-2 text-sm">
                        <input
                          type="checkbox"
                          checked={erpSolutionModuleDraft.includes(moduleKeyValue)}
                          onChange={(event) => {
                            setERPSolutionModuleDraft((current) =>
                              event.target.checked ? [...new Set([...current, moduleKeyValue])] : current.filter((item) => item !== moduleKeyValue),
                            )
                          }}
                        />
                        {t(`erp.module.${moduleKeyValue}`)}
                      </label>
                    ))}
                  </div>
                </div>
                <div className="mt-3 flex flex-wrap gap-2">
                  {erpSolutionAssets.map((asset) => (
                    <span key={asset} className="rounded-md bg-white px-2 py-1 text-xs font-semibold text-slate-600">
                      {t(`systemAdmin.erpAsset.${asset}`)}
                    </span>
                  ))}
                </div>
                <button
                  type="button"
                  onClick={() => void createERPSolutionFlow()}
                  disabled={!activeOrganizationID || loading || !canPlatform('schema.manage') || erpSolutionModuleDraft.length === 0}
                  className="mt-4 inline-flex h-10 w-full items-center justify-center gap-2 rounded-md bg-[#AD4714] px-4 text-sm font-semibold text-white transition hover:bg-[#B84F18] disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <Braces className="h-4 w-4" />
                  {t('systemAdmin.createERPSolution')}
                </button>
              </div>
              <div className="rounded-lg border border-slate-200 bg-slate-50 p-4">
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <h3 className="text-sm font-semibold text-slate-950">{t('systemAdmin.solutionTableFieldChange')}</h3>
                    <p className="mt-1 text-xs text-slate-600">{t('systemAdmin.solutionTableFieldChangeSummary')}</p>
                  </div>
                  <Table2 className="h-5 w-5 text-slate-500" />
                </div>
                <div className="mt-3 grid gap-2">
                  <input
                    value={industryTableDraft.tableName}
                    onChange={(event) => setIndustryTableDraft((current) => ({ ...current, tableName: event.target.value }))}
                    placeholder={t('systemAdmin.tableName')}
                    className="h-9 rounded-md border border-slate-300 px-3 text-sm"
                  />
                  <input
                    value={industryTableDraft.displayName}
                    onChange={(event) => setIndustryTableDraft((current) => ({ ...current, displayName: event.target.value }))}
                    placeholder={t('systemAdmin.displayName')}
                    className="h-9 rounded-md border border-slate-300 px-3 text-sm"
                  />
                  <div className="grid gap-2 sm:grid-cols-2">
                    <input
                      value={industryTableDraft.fieldName}
                      onChange={(event) => setIndustryTableDraft((current) => ({ ...current, fieldName: event.target.value }))}
                      placeholder={t('systemAdmin.fieldName')}
                      className="h-9 rounded-md border border-slate-300 px-3 text-sm"
                    />
                    <input
                      value={industryTableDraft.dataType}
                      onChange={(event) => setIndustryTableDraft((current) => ({ ...current, dataType: event.target.value }))}
                      placeholder={t('systemAdmin.dataType')}
                      className="h-9 rounded-md border border-slate-300 px-3 text-sm"
                    />
                  </div>
                  <div className="grid gap-2 sm:grid-cols-[1fr_auto]">
                    <input
                      value={industryTableDraft.defaultValue}
                      onChange={(event) => setIndustryTableDraft((current) => ({ ...current, defaultValue: event.target.value }))}
                      placeholder={t('systemAdmin.defaultValue')}
                      className="h-9 rounded-md border border-slate-300 px-3 text-sm"
                    />
                    <label className="inline-flex h-9 items-center gap-2 rounded-md border border-slate-300 bg-white px-3 text-sm text-slate-700">
                      <input
                        type="checkbox"
                        checked={industryTableDraft.nullable}
                        onChange={(event) => setIndustryTableDraft((current) => ({ ...current, nullable: event.target.checked }))}
                      />
                      {t('systemAdmin.nullable')}
                    </label>
                  </div>
                </div>
                <button
                  type="button"
                  onClick={() => void createIndustryTableFieldChange()}
                  disabled={!activeOrganizationID || loading || !canPlatform('industry.solution.manage') || !canPlatform('schema.manage')}
                  className="mt-4 inline-flex h-10 w-full items-center justify-center gap-2 rounded-md border border-slate-300 bg-white px-4 text-sm font-semibold text-slate-800 transition hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <Braces className="h-4 w-4" />
                  {t('systemAdmin.createSchemaChange')}
                </button>
              </div>
            </div>
          </section>

          <div className="space-y-5">
            <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
              <div className="flex items-center justify-between gap-3">
                <div>
                  <h2 className="text-base font-semibold text-slate-950">{t('systemAdmin.currentIndustry')}</h2>
                  <p className="mt-1 text-sm text-slate-500">{selectedOrganization?.name || t('common.notSelected')}</p>
                </div>
                <StatusBadge label={industryAdoption?.status || 'inactive'} />
              </div>
              <div className="mt-4 grid gap-3 md:grid-cols-3">
                <Metric label={t('systemAdmin.industry')} value={industryAdoption?.industry_key || t('common.notSelected')} />
                <Metric label={t('systemAdmin.packageId')} value={industryAdoption?.package_id || t('common.notSelected')} />
                <Metric label={t('systemAdmin.enabledModules')} value={String(industryAdoption?.enabled_modules?.length ?? 0)} />
              </div>
            </section>

            <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
              <div className="flex items-center justify-between gap-3">
                <div>
                  <h2 className="text-base font-semibold text-slate-950">{t('systemAdmin.privateExtensions')}</h2>
                  <p className="mt-1 text-sm text-slate-500">{t('systemAdmin.privateExtensionsSummary')}</p>
                </div>
                <button
                  type="button"
                  onClick={() => void createPrivateIndustryExtension()}
                  disabled={!extensionKey.trim() || !extensionName.trim() || !extensionModuleKey.trim() || loading}
                  className="inline-flex h-9 items-center gap-2 rounded-md bg-slate-950 px-3 text-sm font-semibold text-white disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <Send className="h-4 w-4" />
                  {t('systemAdmin.createExtension')}
                </button>
              </div>
              <div className="mt-4 grid gap-3 md:grid-cols-3">
                <input
                  value={extensionKey}
                  onChange={(event) => setExtensionKey(event.target.value)}
                  placeholder={t('systemAdmin.extensionKey')}
                  className="h-10 rounded-md border border-slate-300 px-3 text-sm"
                />
                <input
                  value={extensionName}
                  onChange={(event) => setExtensionName(event.target.value)}
                  placeholder={t('systemAdmin.extensionName')}
                  className="h-10 rounded-md border border-slate-300 px-3 text-sm"
                />
                <input
                  value={extensionModuleKey}
                  onChange={(event) => setExtensionModuleKey(event.target.value)}
                  placeholder={t('systemAdmin.extensionModuleKey')}
                  className="h-10 rounded-md border border-slate-300 px-3 text-sm"
                />
              </div>
              <div className="mt-4 overflow-hidden rounded-lg border border-slate-200">
                <table className="min-w-full divide-y divide-slate-200 text-left text-sm">
                  <thead className="bg-slate-50 text-xs uppercase text-slate-500">
                    <tr>
                      <th className="px-3 py-2">{t('common.name')}</th>
                      <th className="px-3 py-2">{t('systemAdmin.industry')}</th>
                      <th className="px-3 py-2">{t('systemAdmin.status')}</th>
                      <th className="px-3 py-2">{t('table.actions')}</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-100">
                    {industryExtensions.map((item) => (
                      <tr key={item.id}>
                        <td className="px-3 py-3 font-medium text-slate-900">{item.name}</td>
                        <td className="px-3 py-3 font-mono text-xs text-slate-600">{item.industry_key}</td>
                        <td className="px-3 py-3"><StatusBadge label={item.status} /></td>
                        <td className="px-3 py-3">
                          <button
                            type="button"
                            onClick={() => void submitExtensionPublication(item.id)}
                            className="rounded-md border border-slate-300 px-3 py-1.5 text-xs font-semibold text-slate-700 hover:bg-slate-50"
                          >
                            {t('systemAdmin.submitPublication')}
                          </button>
                        </td>
                      </tr>
                    ))}
                    {industryExtensions.length === 0 && (
                      <tr>
                        <td colSpan={4} className="px-3 py-6 text-center text-sm text-slate-500">{t('systemAdmin.noExtensions')}</td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            </section>

            <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <h2 className="text-base font-semibold text-slate-950">{t('systemAdmin.publicationRequests')}</h2>
                  <p className="mt-1 text-sm text-slate-500">{t('systemAdmin.publicationRequestsSummary')}</p>
                </div>
                <input
                  value={publicationReason}
                  onChange={(event) => setPublicationReason(event.target.value)}
                  placeholder={t('systemAdmin.reasonPlaceholder')}
                  className="h-10 w-full rounded-md border border-slate-300 px-3 text-sm md:w-80"
                />
              </div>
              <div className="mt-4 overflow-hidden rounded-lg border border-slate-200">
                <table className="min-w-full divide-y divide-slate-200 text-left text-sm">
                  <thead className="bg-slate-50 text-xs uppercase text-slate-500">
                    <tr>
                      <th className="px-3 py-2">{t('systemAdmin.industry')}</th>
                      <th className="px-3 py-2">{t('systemAdmin.organization')}</th>
                      <th className="px-3 py-2">{t('systemAdmin.status')}</th>
                      <th className="px-3 py-2">{t('table.actions')}</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-100">
                    {publicationRequests.map((item) => (
                      <Fragment key={item.id}>
                        <tr>
                          <td className="px-3 py-3 font-medium text-slate-900">{item.industry_key}</td>
                          <td className="px-3 py-3 font-mono text-xs text-slate-600">{organizationByID[item.source_organization_id] || item.source_organization_id}</td>
                          <td className="px-3 py-3"><StatusBadge label={item.status} /></td>
                          <td className="px-3 py-3">
                            <div className="flex flex-wrap gap-2">
                              <button
                                type="button"
                                onClick={() => void reviewPublication(item.id, 'approve')}
                                disabled={item.status !== 'pending'}
                                className="rounded-md border border-slate-300 px-3 py-1.5 text-xs font-semibold text-slate-700 hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50"
                              >
                                {t('systemAdmin.approve')}
                              </button>
                              <button
                                type="button"
                                onClick={() => void reviewPublication(item.id, 'reject')}
                                disabled={item.status !== 'pending'}
                                className="rounded-md border border-slate-300 px-3 py-1.5 text-xs font-semibold text-slate-700 hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50"
                              >
                                {t('systemAdmin.reject')}
                              </button>
                            </div>
                          </td>
                        </tr>
                        {publicationGates(item).length > 0 && (
                          <tr>
                            <td colSpan={4} className="px-3 pb-3">
                              <div className="space-y-2 rounded-lg border border-slate-200 bg-slate-50 p-3">
                                <p className="text-xs font-semibold text-slate-500">{t('systemAdmin.publicationGates')}</p>
                                {publicationGates(item).map((gate) => (
                                  <div key={gate.key} className="rounded-md border border-slate-200 bg-white p-2">
                                    <div className="flex items-start justify-between gap-2">
                                      <p className="text-xs font-semibold text-slate-700">{t(`systemAdmin.gate.${gate.key}`)}</p>
                                      <StatusBadge label={gate.status} />
                                    </div>
                                    <p className="mt-1 text-xs text-slate-500">{gate.message}</p>
                                  </div>
                                ))}
                              </div>
                            </td>
                          </tr>
                        )}
                      </Fragment>
                    ))}
                    {publicationRequests.length === 0 && (
                      <tr>
                        <td colSpan={4} className="px-3 py-6 text-center text-sm text-slate-500">{t('systemAdmin.noPublicationRequests')}</td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            </section>
          </div>
        </div>
      )}

      {effectiveActiveTab === 'features' && (
        <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
          <div className="flex items-center justify-between gap-3">
            <div>
              <h2 className="text-base font-semibold text-slate-950">{t('systemAdmin.platformFeatures')}</h2>
              <p className="mt-1 text-sm text-slate-500">{t('systemAdmin.platformFeaturesSummary')}</p>
            </div>
            <ShieldCheck className="h-5 w-5 text-slate-500" />
          </div>
          <div className="mt-5 grid gap-3 lg:grid-cols-[1fr_320px]">
            <div className="overflow-hidden rounded-lg border border-slate-200">
              <table className="min-w-full divide-y divide-slate-200 text-sm">
                <thead className="bg-slate-50 text-left text-xs font-semibold uppercase text-slate-500">
                  <tr>
                    <th className="px-3 py-2">{t('systemAdmin.featureKey')}</th>
                    <th className="px-3 py-2">{t('systemAdmin.module')}</th>
                    <th className="px-3 py-2">{t('systemAdmin.status')}</th>
                    <th className="px-3 py-2">{t('systemAdmin.permissions')}</th>
                    <th className="px-3 py-2">{t('common.actions')}</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100">
                  {platformFeatures.length > 0 ? (
                    platformFeatures.map((feature) => (
                      <tr key={feature.feature_key}>
                        <td className="px-3 py-3">
                          <p className="font-medium text-slate-900">{feature.title}</p>
                          <p className="font-mono text-xs text-slate-500">{feature.feature_key}</p>
                        </td>
                        <td className="px-3 py-3 text-slate-600">{feature.module_key}</td>
                        <td className="px-3 py-3">
                          <StatusBadge label={feature.status} />
                        </td>
                        <td className="px-3 py-3 text-xs text-slate-600">{feature.permission_keys.join(', ') || t('common.empty')}</td>
                        <td className="px-3 py-3">
                          <button
                            type="button"
                            onClick={() => void publishFeature(feature.feature_key)}
                            disabled={feature.status === 'active' || loading || !canPlatform('platform.feature.manage')}
                            className="inline-flex h-8 items-center rounded-md border border-slate-300 px-2.5 text-xs font-semibold text-slate-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50"
                          >
                            {t('systemAdmin.publish')}
                          </button>
                        </td>
                      </tr>
                    ))
                  ) : (
                    <tr>
                      <td colSpan={5} className="px-3 py-6 text-center text-slate-500">
                        {t('systemAdmin.noFeatures')}
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
            <aside className="rounded-lg border border-slate-200 bg-slate-50 p-4">
              <h3 className="text-sm font-semibold text-slate-950">{t('systemAdmin.createFeature')}</h3>
              <div className="mt-3 space-y-2">
                <input
                  value={featureDraft.featureKey}
                  onChange={(event) => setFeatureDraft((current) => ({ ...current, featureKey: event.target.value }))}
                  placeholder={t('systemAdmin.featureKey')}
                  className="h-9 w-full rounded-md border border-slate-300 px-3 text-sm"
                />
                <input
                  value={featureDraft.title}
                  onChange={(event) => setFeatureDraft((current) => ({ ...current, title: event.target.value }))}
                  placeholder={t('systemAdmin.title')}
                  className="h-9 w-full rounded-md border border-slate-300 px-3 text-sm"
                />
                <input
                  value={featureDraft.moduleKey}
                  onChange={(event) => setFeatureDraft((current) => ({ ...current, moduleKey: event.target.value }))}
                  placeholder={t('systemAdmin.module')}
                  className="h-9 w-full rounded-md border border-slate-300 px-3 text-sm"
                />
                <textarea
                  value={featureDraft.permissionKeys}
                  onChange={(event) => setFeatureDraft((current) => ({ ...current, permissionKeys: event.target.value }))}
                  placeholder={t('systemAdmin.permissionKey')}
                  className="min-h-[84px] w-full rounded-md border border-slate-300 p-3 text-sm"
                />
                <button
                  type="button"
                  onClick={() => void createFeature()}
                  disabled={loading || !canPlatform('platform.feature.manage')}
                  className="inline-flex h-9 w-full items-center justify-center rounded-md bg-slate-950 px-3 text-sm font-semibold text-white disabled:opacity-50"
                >
                  {t('systemAdmin.createFeature')}
                </button>
              </div>
            </aside>
          </div>
        </section>
      )}

      {effectiveActiveTab === 'permissions' && (
        <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
          <div className="flex items-center justify-between gap-3">
            <div>
              <h2 className="text-base font-semibold text-slate-950">{t('systemAdmin.permissions')}</h2>
              <p className="mt-1 text-sm text-slate-500">{t('systemAdmin.permissionsSummary')}</p>
            </div>
            <ShieldCheck className="h-5 w-5 text-slate-500" />
          </div>
          <div className="mt-4 grid gap-3 md:grid-cols-3">
            <Metric label={t('systemAdmin.role')} value={platformPermissions?.role || t('common.notSelected')} />
            <Metric label={t('systemAdmin.enabledPermissions')} value={String(Object.values(platformPermissions?.permissions ?? {}).filter(Boolean).length)} />
            <Metric label={t('systemAdmin.menuItems')} value={String(platformPermissions?.menu_items?.length ?? 0)} />
          </div>
          <div className="mt-5 grid gap-4 xl:grid-cols-[1fr_360px]">
            <div className="space-y-3">
              {platformRoles.map((role) => (
                <article key={role.role_key} className="rounded-lg border border-slate-200 bg-slate-50 p-4">
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <h3 className="text-sm font-semibold text-slate-950">{role.name}</h3>
                      <p className="mt-1 font-mono text-xs text-slate-500">{role.role_key}</p>
                    </div>
                    <StatusBadge label={role.status} />
                  </div>
                  <textarea
                    value={rolePermissionDraft[role.role_key] ?? (role.permissions ?? []).join('\n')}
                    onChange={(event) => setRolePermissionDraft((current) => ({ ...current, [role.role_key]: event.target.value }))}
                    className="mt-3 min-h-[120px] w-full rounded-md border border-slate-300 bg-white p-3 font-mono text-xs text-slate-700"
                  />
                  <button
                    type="button"
                    onClick={() => void saveRolePermissions(role.role_key)}
                    disabled={loading || !canPlatform('platform.rbac.manage')}
                    className="mt-3 inline-flex h-9 items-center rounded-md bg-slate-950 px-3 text-sm font-semibold text-white disabled:opacity-50"
                  >
                    {t('systemAdmin.savePermissions')}
                  </button>
                </article>
              ))}
            </div>
            <aside className="overflow-hidden rounded-lg border border-slate-200">
              <table className="min-w-full divide-y divide-slate-200 text-sm">
                <thead className="bg-slate-50 text-left text-xs font-semibold uppercase text-slate-500">
                  <tr>
                    <th className="px-3 py-2">{t('systemAdmin.permissionKey')}</th>
                    <th className="px-3 py-2">{t('systemAdmin.status')}</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100">
                  {(platformPermissionItems.length ? platformPermissionItems : platformPermissionCatalog.map((permission) => ({
                    permission_key: permission,
                    name: t(`systemAdmin.permission.${permission}`),
                    category: 'platform',
                    status: platformPermissions?.permissions[permission] ? 'active' : 'disabled',
                    metadata: {},
                    created_at: '',
                    updated_at: '',
                  }))).map((permission) => (
                    <tr key={permission.permission_key}>
                      <td className="px-3 py-3">
                        <p className="font-medium text-slate-900">{permission.name}</p>
                        <p className="font-mono text-xs text-slate-500">{permission.permission_key}</p>
                      </td>
                      <td className="px-3 py-3">
                        <StatusBadge label={permission.status} />
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </aside>
          </div>
        </section>
      )}

      {effectiveActiveTab === 'users' && (
        <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
          <div className="flex items-center justify-between gap-3">
            <div>
              <h2 className="text-base font-semibold text-slate-950">{t('systemAdmin.platformUsers')}</h2>
              <p className="mt-1 text-sm text-slate-500">{t('systemAdmin.platformUsersSummary')}</p>
            </div>
            <Users className="h-5 w-5 text-slate-500" />
          </div>
          {temporaryCredential && (
            <pre className="mt-4 overflow-auto rounded-lg border border-amber-200 bg-amber-50 p-3 font-mono text-xs text-amber-800">
              {temporaryCredential}
            </pre>
          )}
          <div className="mt-5 grid gap-4 xl:grid-cols-[320px_1fr]">
            <aside className="rounded-lg border border-slate-200 bg-slate-50 p-4">
              <h3 className="text-sm font-semibold text-slate-950">{t('systemAdmin.createPlatformUser')}</h3>
              <div className="mt-3 space-y-2">
                <input
                  value={platformUserDraft.name}
                  onChange={(event) => setPlatformUserDraft((current) => ({ ...current, name: event.target.value }))}
                  placeholder={t('systemAdmin.name')}
                  className="h-9 w-full rounded-md border border-slate-300 px-3 text-sm"
                />
                <input
                  value={platformUserDraft.email}
                  onChange={(event) => setPlatformUserDraft((current) => ({ ...current, email: event.target.value }))}
                  placeholder={t('systemAdmin.email')}
                  className="h-9 w-full rounded-md border border-slate-300 px-3 text-sm"
                />
                <input
                  value={platformUserDraft.roles}
                  onChange={(event) => setPlatformUserDraft((current) => ({ ...current, roles: event.target.value }))}
                  placeholder={t('systemAdmin.roles')}
                  className="h-9 w-full rounded-md border border-slate-300 px-3 text-sm"
                />
                <button
                  type="button"
                  onClick={() => void createUser()}
                  disabled={loading || !canPlatform('platform.user.manage')}
                  className="inline-flex h-9 w-full items-center justify-center rounded-md bg-slate-950 px-3 text-sm font-semibold text-white disabled:opacity-50"
                >
                  {t('systemAdmin.createPlatformUser')}
                </button>
              </div>
            </aside>
            <div className="overflow-hidden rounded-lg border border-slate-200">
              <table className="min-w-full divide-y divide-slate-200 text-sm">
                <thead className="bg-slate-50 text-left text-xs font-semibold uppercase text-slate-500">
                  <tr>
                    <th className="px-3 py-2">{t('systemAdmin.user')}</th>
                    <th className="px-3 py-2">{t('systemAdmin.roles')}</th>
                    <th className="px-3 py-2">{t('systemAdmin.status')}</th>
                    <th className="px-3 py-2">{t('common.actions')}</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100">
                  {platformUsers.map((user) => (
                    <tr key={user.user_id}>
                      <td className="px-3 py-3">
                        <p className="font-medium text-slate-900">{user.name}</p>
                        <p className="text-xs text-slate-500">{user.email}</p>
                      </td>
                      <td className="px-3 py-3 text-slate-600">{user.roles.join(', ')}</td>
                      <td className="px-3 py-3">
                        <StatusBadge label={user.account_status} />
                      </td>
                      <td className="px-3 py-3">
                        <div className="flex flex-wrap gap-2">
                          <button
                            type="button"
                            onClick={() => void resetUserPassword(user.user_id)}
                            disabled={loading || !canPlatform('platform.user.manage')}
                            className="inline-flex h-8 items-center rounded-md border border-slate-300 px-2 text-xs font-semibold text-slate-700 disabled:opacity-50"
                          >
                            {t('systemAdmin.resetPassword')}
                          </button>
                          <button
                            type="button"
                            onClick={() => void disableUser(user.user_id)}
                            disabled={loading || user.account_status === 'disabled' || !canPlatform('platform.user.manage')}
                            className="inline-flex h-8 items-center rounded-md border border-red-200 bg-red-50 px-2 text-xs font-semibold text-red-700 disabled:opacity-50"
                          >
                            {t('systemAdmin.disableUser')}
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                  {platformUsers.length === 0 && (
                    <tr>
                      <td colSpan={4} className="px-3 py-6 text-center text-slate-500">{t('systemAdmin.noPlatformUsers')}</td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </section>
      )}

      {effectiveActiveTab === 'database' && (
        <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
          <div className="flex items-center justify-between gap-3">
            <div>
              <h2 className="text-base font-semibold text-slate-950">{t('systemAdmin.databaseMaintenance')}</h2>
              <p className="mt-1 text-sm text-slate-500">{t('systemAdmin.databaseMaintenanceSummary')}</p>
            </div>
            <Database className="h-5 w-5 text-slate-500" />
          </div>
          <div className="mt-5 grid gap-4 xl:grid-cols-[320px_1fr]">
            <aside className="rounded-lg border border-slate-200 bg-slate-50 p-4">
              <h3 className="text-sm font-semibold text-slate-950">{t('systemAdmin.createDatabaseJob')}</h3>
              <div className="mt-3 space-y-2">
                <select
                  value={maintenanceDraft.jobType}
                  onChange={(event) => setMaintenanceDraft((current) => ({ ...current, jobType: event.target.value }))}
                  className="h-9 w-full rounded-md border border-slate-300 px-3 text-sm"
                >
                  <option value="backup">{t('systemAdmin.backup')}</option>
                  <option value="restore">{t('systemAdmin.restore')}</option>
                </select>
                <input
                  value={maintenanceDraft.backupRef}
                  onChange={(event) => setMaintenanceDraft((current) => ({ ...current, backupRef: event.target.value }))}
                  placeholder={t('systemAdmin.backupRef')}
                  className="h-9 w-full rounded-md border border-slate-300 px-3 text-sm"
                />
                <textarea
                  value={maintenanceDraft.reason}
                  onChange={(event) => setMaintenanceDraft((current) => ({ ...current, reason: event.target.value }))}
                  placeholder={t('systemAdmin.reasonPlaceholder')}
                  className="min-h-[84px] w-full rounded-md border border-slate-300 p-3 text-sm"
                />
                <button
                  type="button"
                  onClick={() => void createMaintenanceJob()}
                  disabled={loading || !canPlatform('database.maintenance.manage')}
                  className="inline-flex h-9 w-full items-center justify-center rounded-md bg-slate-950 px-3 text-sm font-semibold text-white disabled:opacity-50"
                >
                  {t('systemAdmin.createDatabaseJob')}
                </button>
              </div>
            </aside>
            <div className="overflow-hidden rounded-lg border border-slate-200">
              <table className="min-w-full divide-y divide-slate-200 text-sm">
                <thead className="bg-slate-50 text-left text-xs font-semibold uppercase text-slate-500">
                  <tr>
                    <th className="px-3 py-2">{t('systemAdmin.jobType')}</th>
                    <th className="px-3 py-2">{t('systemAdmin.status')}</th>
                    <th className="px-3 py-2">{t('systemAdmin.reason')}</th>
                    <th className="px-3 py-2">{t('common.actions')}</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100">
                  {databaseMaintenanceJobs.map((job) => (
                    <tr key={job.id}>
                      <td className="px-3 py-3 font-medium text-slate-900">{t(`systemAdmin.${job.job_type}`)}</td>
                      <td className="px-3 py-3">
                        <StatusBadge label={job.status} />
                      </td>
                      <td className="px-3 py-3 text-slate-600">{job.reason || job.backup_ref || t('common.empty')}</td>
                      <td className="px-3 py-3">
                        <div className="flex flex-wrap gap-2">
                          <button
                            type="button"
                            onClick={() => void reviewMaintenanceJob(job.id, 'approve')}
                            disabled={job.status !== 'pending_approval' || loading || !canPlatform('database.maintenance.approve')}
                            className="inline-flex h-8 items-center rounded-md border border-emerald-200 bg-emerald-50 px-2 text-xs font-semibold text-emerald-700 disabled:opacity-50"
                          >
                            {t('systemAdmin.approve')}
                          </button>
                          <button
                            type="button"
                            onClick={() => void reviewMaintenanceJob(job.id, 'reject')}
                            disabled={job.status !== 'pending_approval' || loading || !canPlatform('database.maintenance.approve')}
                            className="inline-flex h-8 items-center rounded-md border border-red-200 bg-red-50 px-2 text-xs font-semibold text-red-700 disabled:opacity-50"
                          >
                            {t('systemAdmin.reject')}
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                  {databaseMaintenanceJobs.length === 0 && (
                    <tr>
                      <td colSpan={4} className="px-3 py-6 text-center text-slate-500">{t('systemAdmin.noDatabaseJobs')}</td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </section>
      )}

      {effectiveActiveTab === 'models' && (
        <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
          <div className="mb-5 flex items-center justify-between gap-3">
            <div>
              <h2 className="text-base font-semibold text-slate-950">{t('systemAdmin.modelAndApiSettings')}</h2>
              <p className="mt-1 text-sm text-slate-500">{t('systemAdmin.modelAndApiSettingsSummary')}</p>
            </div>
            <Table2 className="h-5 w-5 text-slate-500" />
          </div>
          <DeveloperToolsWorkspace token={token} apiScope="platform" />
        </section>
      )}

      {effectiveActiveTab === 'runtime' && (
        <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
          <div className="mb-5 flex items-center justify-between gap-3">
            <div>
              <h2 className="text-base font-semibold text-slate-950">{t('systemAdmin.apiWorkbench')}</h2>
              <p className="mt-1 text-sm text-slate-500">{t('systemAdmin.apiWorkbenchSummary')}</p>
            </div>
            <Table2 className="h-5 w-5 text-slate-500" />
          </div>
          <ApiWorkbench token={token} apiScope="platform" />
        </section>
      )}

      {effectiveActiveTab === 'catalog' && (
        <div className="grid gap-5 xl:grid-cols-[360px_1fr]">
          <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
            <div className="flex items-center justify-between gap-3">
              <div>
                <h2 className="text-base font-semibold text-slate-950">{t('systemAdmin.platformCatalog')}</h2>
                <p className="mt-1 text-sm text-slate-500">{t('systemAdmin.masterRecords')}</p>
              </div>
              <Table2 className="h-5 w-5 text-slate-500" />
            </div>
            <label className="mt-4 block">
              <span className="text-xs font-semibold text-slate-500">{t('systemAdmin.module')}</span>
              <select
                value={moduleKey}
                onChange={(event) => setModuleKey(event.target.value)}
                className="mt-1 h-10 w-full rounded-lg border border-slate-300 bg-white px-3 text-sm text-slate-900 outline-none focus:border-[#AD4714] focus:ring-2 focus:ring-[#DF6A24]/20"
              >
                {moduleOptions.map((item) => (
                  <option key={item} value={item}>
                    {item}
                  </option>
                ))}
              </select>
            </label>
            <div className="mt-4 space-y-2">
              {masters.length > 0 ? (
                masters.map((item) => (
                  <button
                    key={item.master_key}
                    type="button"
                    onClick={() => setSelectedMasterKey(item.master_key)}
                    className={`w-full rounded-lg border px-3 py-3 text-left transition ${
                      selectedMaster?.master_key === item.master_key
                        ? 'border-slate-900 bg-slate-950 text-white'
                        : 'border-slate-200 bg-white text-slate-900 hover:border-slate-300 hover:bg-slate-50'
                    }`}
                  >
                    <span className="block truncate text-sm font-semibold">{item.title || item.master_key}</span>
                    <span className="mt-1 block truncate text-xs opacity-75">{item.entity_type} / {item.source_table || item.master_key}</span>
                  </button>
                ))
              ) : (
                <p className="rounded-lg border border-dashed border-slate-300 p-4 text-sm text-slate-500">{t('systemAdmin.noMasters')}</p>
              )}
            </div>
          </section>

          <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div>
                <h2 className="text-base font-semibold text-slate-950">{selectedMaster?.title || t('systemAdmin.details')}</h2>
                <p className="mt-1 text-sm text-slate-500">{selectedMaster?.master_key || t('common.notSelected')}</p>
              </div>
              {selectedMaster && <StatusBadge label={selectedMaster.status} />}
            </div>
            <div className="mt-4 grid gap-3 md:grid-cols-3">
              <Metric label={t('systemAdmin.entityType')} value={selectedMaster?.entity_type || t('common.notSelected')} />
              <Metric label={t('systemAdmin.source')} value={selectedMaster?.source_table || t('common.notSelected')} />
              <Metric label={t('systemAdmin.detailCount')} value={String(details.length)} />
            </div>
            <div className="mt-5 overflow-hidden rounded-lg border border-slate-200">
              <table className="min-w-full divide-y divide-slate-200 text-sm">
                <thead className="bg-slate-50 text-left text-xs font-semibold uppercase text-slate-500">
                  <tr>
                    <th className="px-3 py-2">{t('systemAdmin.detailType')}</th>
                    <th className="px-3 py-2">{t('systemAdmin.field')}</th>
                    <th className="px-3 py-2">{t('systemAdmin.payload')}</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100">
                  {details.length > 0 ? (
                    details.map((item) => (
                      <tr key={item.detail_key}>
                        <td className="px-3 py-3 font-medium text-slate-900">{item.detail_type}</td>
                        <td className="px-3 py-3 text-slate-600">{item.field_key || item.line_no}</td>
                        <td className="px-3 py-3">
                          <pre className="max-h-24 overflow-auto rounded bg-slate-50 p-2 text-xs text-slate-600">{jsonText(item.payload)}</pre>
                        </td>
                      </tr>
                    ))
                  ) : (
                    <tr>
                      <td colSpan={3} className="px-3 py-6 text-center text-slate-500">
                        {t('systemAdmin.noDetails')}
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          </section>
        </div>
      )}

      {effectiveActiveTab === 'targets' && (
        <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
          <div className="flex items-center justify-between gap-3">
            <div>
              <h2 className="text-base font-semibold text-slate-950">{t('systemAdmin.schemaTargets')}</h2>
              <p className="mt-1 text-sm text-slate-500">{t('systemAdmin.schemaTargetSummary')}</p>
            </div>
            <Database className="h-5 w-5 text-slate-500" />
          </div>
          <div className="mt-5 overflow-hidden rounded-lg border border-slate-200">
            <table className="min-w-full divide-y divide-slate-200 text-sm">
              <thead className="bg-slate-50 text-left text-xs font-semibold uppercase text-slate-500">
                <tr>
                  <th className="px-3 py-2">{t('systemAdmin.organization')}</th>
                  <th className="px-3 py-2">{t('systemAdmin.schemaName')}</th>
                  <th className="px-3 py-2">{t('systemAdmin.templateVersion')}</th>
                  <th className="px-3 py-2">{t('systemAdmin.status')}</th>
                  <th className="px-3 py-2">{t('systemAdmin.updated')}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {targets.length > 0 ? (
                  targets.map((item) => (
                    <tr key={item.organization_id}>
                      <td className="px-3 py-3 font-medium text-slate-900">{organizationByID[item.organization_id] || item.organization_id}</td>
                      <td className="px-3 py-3 text-slate-600">{item.schema_name}</td>
                      <td className="px-3 py-3 text-slate-600">{item.template_version}</td>
                      <td className="px-3 py-3">
                        <StatusBadge label={item.status} />
                      </td>
                      <td className="px-3 py-3 text-slate-500">{formatDateTime(item.updated_at)}</td>
                    </tr>
                  ))
                ) : (
                  <tr>
                    <td colSpan={5} className="px-3 py-6 text-center text-slate-500">
                      {t('systemAdmin.noTargets')}
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </section>
      )}

      {effectiveActiveTab === 'schema' && (
        <div className="grid gap-5 xl:grid-cols-[1fr_360px]">
          <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div>
                <h2 className="text-base font-semibold text-slate-950">{t('systemAdmin.schemaPackage')}</h2>
                <p className="mt-1 text-sm text-slate-500">{t('systemAdmin.schemaPackageSummary')}</p>
              </div>
              <div className="flex flex-wrap gap-2">
                <button
                  type="button"
                  onClick={() => void exportSchema()}
                  disabled={!activeOrganizationID || loading || !canPlatform('schema.manage')}
                  className="inline-flex h-9 items-center gap-2 rounded-md bg-slate-950 px-3 text-sm font-semibold text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <Download className="h-4 w-4" />
                  {t('systemAdmin.exportSchema')}
                </button>
                <button
                  type="button"
                  onClick={downloadSchema}
                  disabled={!schemaJson.trim()}
                  className="inline-flex h-9 items-center gap-2 rounded-md border border-slate-300 px-3 text-sm font-semibold text-slate-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <FileJson className="h-4 w-4" />
                  {t('systemAdmin.downloadJson')}
                </button>
              </div>
            </div>

            <div className="mt-4 grid gap-3 md:grid-cols-3">
              <label className="block">
                <span className="text-xs font-semibold text-slate-500">{t('systemAdmin.selectedOrganization')}</span>
                <select
                  value={activeOrganizationID}
                  onChange={(event) => setSelectedOrganizationID(event.target.value)}
                  className="mt-1 h-10 w-full rounded-lg border border-slate-300 bg-white px-3 text-sm text-slate-900 outline-none focus:border-[#AD4714] focus:ring-2 focus:ring-[#DF6A24]/20"
                >
                  {visibleManagementOrganizations.map((item) => (
                    <option key={item.id} value={item.id}>
                      {item.name}
                    </option>
                  ))}
                </select>
              </label>
              <Metric label={t('systemAdmin.tables')} value={String(schemaPackage?.tables.length ?? 0)} />
              <Metric label={t('systemAdmin.fields')} value={String(countFields(schemaPackage))} />
            </div>

            <div className="mt-4 flex flex-wrap items-center gap-2">
              <input id="system-admin-json-file" type="file" accept="application/json,.json" className="sr-only" onChange={(event) => void importSchemaFile(event)} />
              <label
                htmlFor="system-admin-json-file"
                className="inline-flex h-9 cursor-pointer items-center gap-2 rounded-md border border-slate-300 px-3 text-sm font-semibold text-slate-700 transition hover:bg-slate-50"
              >
                <Upload className="h-4 w-4" />
                {t('systemAdmin.importJson')}
              </label>
              <input
                value={reason}
                onChange={(event) => setReason(event.target.value)}
                placeholder={t('systemAdmin.reasonPlaceholder')}
                className="h-9 min-w-[240px] flex-1 rounded-md border border-slate-300 px-3 text-sm text-slate-900 outline-none focus:border-[#AD4714] focus:ring-2 focus:ring-[#DF6A24]/20"
              />
            </div>

            <label className="mt-4 block">
              <span className="text-xs font-semibold text-slate-500">{t('systemAdmin.schemaJson')}</span>
              <textarea
                value={schemaJson}
                onChange={(event) => {
                  setSchemaJson(event.target.value)
                  setSchemaPackage(null)
                }}
                placeholder={t('systemAdmin.schemaJsonPlaceholder')}
                spellCheck={false}
                className="mt-1 min-h-[420px] w-full resize-y rounded-lg border border-slate-300 bg-slate-950 p-4 font-mono text-xs text-slate-100 outline-none focus:border-[#AD4714] focus:ring-2 focus:ring-[#DF6A24]/20"
              />
            </label>
          </section>

          <aside className="space-y-5">
            <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
              <div className="flex items-center gap-2">
                <ShieldCheck className="h-5 w-5 text-slate-500" />
                <h2 className="text-base font-semibold text-slate-950">{t('systemAdmin.changeWorkflow')}</h2>
              </div>
              <div className="mt-4 space-y-2">
                <button
                  type="button"
                  onClick={() => void createChange()}
                  disabled={!activeOrganizationID || !schemaJson.trim() || loading || !canPlatform('schema.manage')}
                  className="inline-flex h-10 w-full items-center justify-center gap-2 rounded-md bg-[#AD4714] px-3 text-sm font-semibold text-[#fffaf5] transition hover:bg-[#B84F18] disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <Braces className="h-4 w-4" />
                  {t('systemAdmin.createChange')}
                </button>
                <button
                  type="button"
                  onClick={() => void approveChange()}
                  disabled={!changeRequest || changeRequest.status !== 'pending' || loading || !canPlatform('schema.approve')}
                  className="inline-flex h-10 w-full items-center justify-center gap-2 rounded-md border border-emerald-200 bg-emerald-50 px-3 text-sm font-semibold text-emerald-700 transition hover:bg-emerald-100 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <Check className="h-4 w-4" />
                  {t('systemAdmin.approve')}
                </button>
                <button
                  type="button"
                  onClick={() => void verifyChange()}
                  disabled={!changeRequest || loading || !canPlatform('schema.manage')}
                  className="inline-flex h-10 w-full items-center justify-center gap-2 rounded-md border border-blue-200 bg-blue-50 px-3 text-sm font-semibold text-blue-700 transition hover:bg-blue-100 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <ShieldCheck className="h-4 w-4" />
                  {t('systemAdmin.verify')}
                </button>
                <button
                  type="button"
                  onClick={() => void applyChange()}
                  disabled={!changeRequest || !['approved', 'applied'].includes(changeRequest.status) || verificationBlocksApply || loading || !canPlatform('schema.apply')}
                  className="inline-flex h-10 w-full items-center justify-center gap-2 rounded-md border border-slate-300 px-3 text-sm font-semibold text-slate-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <Play className="h-4 w-4" />
                  {t('systemAdmin.apply')}
                </button>
              </div>
            </section>

            <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
              <h2 className="text-base font-semibold text-slate-950">{t('systemAdmin.currentTarget')}</h2>
              <div className="mt-4 space-y-3">
                <Metric label={t('systemAdmin.schemaName')} value={selectedTarget?.schema_name || t('common.notSelected')} />
                <Metric label={t('systemAdmin.templateVersion')} value={selectedTarget?.template_version || t('common.notSelected')} />
                <Metric label={t('systemAdmin.status')} value={selectedTarget?.status ? t(selectedTarget.status) : t('common.notSelected')} />
              </div>
            </section>

            <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
              <h2 className="text-base font-semibold text-slate-950">{t('systemAdmin.latestRequest')}</h2>
              {changeRequest ? (
                <div className="mt-4 space-y-3 text-sm">
                  <Metric label={t('systemAdmin.requestId')} value={changeRequest.id} />
                  <Metric label={t('systemAdmin.status')} value={t(changeRequest.status)} />
                  <Metric label={t('systemAdmin.statementCount')} value={String(changeRequest.statements.length)} />
                  {packageAssetsByType.length > 0 && (
                    <div>
                      <p className="text-xs font-semibold text-slate-500">{t('systemAdmin.packageAssets')}</p>
                      <div className="mt-2 grid gap-2 sm:grid-cols-2">
                        {packageAssetsByType.map((group) => (
                          <div key={group.assetType} className="rounded-lg border border-slate-200 bg-slate-50 p-3">
                            <div className="flex items-center justify-between gap-2">
                              <p className="min-w-0 truncate text-sm font-semibold text-slate-900">{t(`systemAdmin.assetType.${group.assetType}`)}</p>
                              <StatusBadge label={String(group.assets.length)} />
                            </div>
                            <p className="mt-1 truncate text-xs text-slate-500">{group.assets[0]?.asset_key || t('common.empty')}</p>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}
                  <div>
                    <p className="text-xs font-semibold text-slate-500">{t('systemAdmin.packageDiff')}</p>
                    {schemaDiffItems.length > 0 ? (
                      <ul className="mt-2 space-y-1 rounded-lg border border-slate-200 bg-slate-50 p-3 text-xs text-slate-600">
                        {schemaDiffItems.map((item) => (
                          <li key={item}>{item}</li>
                        ))}
                      </ul>
                    ) : (
                      <p className="mt-2 rounded-lg border border-dashed border-slate-300 p-3 text-xs text-slate-500">{t('systemAdmin.noPackageDiff')}</p>
                    )}
                  </div>
                  {packageAssetDiff.length > 0 && (
                    <div>
                      <p className="text-xs font-semibold text-slate-500">{t('systemAdmin.metadataAssets')}</p>
                      <div className="mt-2 space-y-2">
                        {packageAssetDiff.map((item) => (
                          <div key={`${item.asset_type}-${item.asset_key}`} className="rounded-lg border border-slate-200 bg-slate-50 p-3">
                            <div className="flex items-start justify-between gap-3">
                              <div className="min-w-0">
                                <p className="truncate text-sm font-semibold text-slate-900">{item.asset_key}</p>
                                <p className="mt-1 text-xs text-slate-500">
                                  {t(`systemAdmin.assetType.${item.asset_type}`)} / {t(item.risk_level)}
                                </p>
                              </div>
                              <StatusBadge label={item.action} />
                            </div>
                            {item.blocking_reason && (
                              <p className="mt-2 rounded-md bg-red-50 p-2 text-xs text-red-700">
                                {t('systemAdmin.blockingReason')}: {item.blocking_reason}
                              </p>
                            )}
                          </div>
                        ))}
                      </div>
                    </div>
                  )}
                  <pre className="max-h-56 overflow-auto rounded-lg bg-slate-50 p-3 text-xs text-slate-600">
                    {changeRequest.statements.join('\n\n') || t('common.empty')}
                  </pre>
                </div>
              ) : (
                <p className="mt-4 rounded-lg border border-dashed border-slate-300 p-4 text-sm text-slate-500">{t('systemAdmin.noChangeRequest')}</p>
              )}
            </section>

            {verificationReport && (
              <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
                <div className="flex items-center justify-between gap-3">
                  <h2 className="text-base font-semibold text-slate-950">{t('systemAdmin.verificationReport')}</h2>
                  <StatusBadge label={verificationReport.status} />
                </div>
                <div className="mt-4 grid gap-3 sm:grid-cols-2">
                  <Metric label={t('systemAdmin.canApply')} value={verificationReport.can_apply ? t('common.yes') : t('common.no')} />
                  <Metric label={t('systemAdmin.blockingIssues')} value={String(verificationReport.blocking_issues)} />
                  <Metric label={t('systemAdmin.statementCount')} value={String(verificationReport.statement_count)} />
                  <Metric label={t('systemAdmin.status')} value={t(verificationReport.request_status)} />
                </div>
                <div className="mt-4 space-y-2">
                  <p className="text-xs font-semibold text-slate-500">{t('systemAdmin.checks')}</p>
                  {verificationReport.checks.map((check) => (
                    <div key={`${check.key}-${check.status}`} className="rounded-lg border border-slate-200 bg-slate-50 p-3">
                      <div className="flex items-start justify-between gap-3">
                        <p className="min-w-0 text-sm font-semibold text-slate-900">{t(`systemAdmin.check.${check.key}`)}</p>
                        <StatusBadge label={check.status} />
                      </div>
                      <p className="mt-2 text-xs text-slate-600">{check.message}</p>
                    </div>
                  ))}
                </div>
              </section>
            )}

            {applyJob && (
              <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
                <h2 className="text-base font-semibold text-slate-950">{t('systemAdmin.applyJob')}</h2>
                <div className="mt-4 space-y-3">
                  <Metric label={t('systemAdmin.status')} value={t(applyJob.status)} />
                  <Metric label={t('systemAdmin.statementCount')} value={String(applyJob.statements.length)} />
                  {applyJob.error_message && <p className="rounded-lg bg-red-50 p-3 text-sm text-red-700">{applyJob.error_message}</p>}
                  {assetResults.length > 0 && (
                    <div className="space-y-2">
                      <p className="text-xs font-semibold text-slate-500">{t('systemAdmin.assetResults')}</p>
                      {assetResults.map((item) => (
                        <div key={`${item.asset_type}-${item.asset_key}`} className="rounded-lg border border-slate-200 bg-slate-50 p-3">
                          <div className="flex items-start justify-between gap-3">
                            <p className="min-w-0 truncate text-sm font-semibold text-slate-900">{item.asset_key}</p>
                            <StatusBadge label={item.status} />
                          </div>
                          <p className="mt-1 truncate text-xs text-slate-500">{item.target}</p>
                          {item.error_message && <p className="mt-2 rounded-md bg-red-50 p-2 text-xs text-red-700">{item.error_message}</p>}
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </section>
            )}
          </aside>
        </div>
      )}
    </div>
  )
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0 rounded-lg border border-slate-200 bg-slate-50 px-3 py-2">
      <p className="text-xs font-semibold text-slate-500">{label}</p>
      <p className="mt-1 truncate text-sm font-semibold text-slate-900">{value}</p>
    </div>
  )
}

function ContextHealthPanel({ health }: { health: AssistantContextHealthSummary | null }) {
  const { t } = useI18n()
  const strictModules = health?.strict_modules?.length ? health.strict_modules : ['erp', 'finance', 'governance']
  const missingModules = health?.missing_strict_modules ?? strictModules
  return (
    <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h2 className="text-base font-semibold text-slate-950">{t('systemAdmin.verifiedLoop')}</h2>
          <p className="mt-1 text-sm text-slate-500">{t('systemAdmin.verifiedLoopSummary')}</p>
        </div>
        <ShieldCheck className="h-5 w-5 text-slate-500" />
      </div>
      <div className="mt-4 grid gap-2">
        <Metric label={t('systemAdmin.activeContextRules')} value={String(health?.active_rule_count ?? 0)} />
        <Metric label={t('systemAdmin.recentContextPackages')} value={String(health?.recent_package_count ?? 0)} />
        <Metric label={t('systemAdmin.fallbackContextPackages')} value={String(health?.fallback_package_count ?? 0)} />
        <Metric label={t('systemAdmin.contextBuildFailures')} value={String(health?.context_build_failure_count ?? 0)} />
        <Metric label={t('systemAdmin.pendingContextProposals')} value={String(health?.pending_proposal_count ?? 0)} />
        <Metric label={t('systemAdmin.approvedContextProposals')} value={String(health?.approved_proposal_count ?? 0)} />
        <Metric label={t('systemAdmin.appliedContextProposals')} value={String(health?.applied_proposal_count ?? 0)} />
        <Metric label={t('systemAdmin.toolApprovalBacklog')} value={String(health?.tool_approval_backlog ?? 0)} />
      </div>
      <div className="mt-4 rounded-lg border border-slate-200 bg-slate-50 p-3">
        <div className="flex items-center justify-between gap-2">
          <p className="text-xs font-semibold text-slate-500">{t('systemAdmin.strictCoverage')}</p>
          <StatusBadge label={missingModules.length ? 'warning' : 'active'} />
        </div>
        <div className="mt-2 flex flex-wrap gap-1.5">
          {strictModules.map((moduleKey) => (
            <span key={moduleKey} className="inline-flex items-center gap-1 rounded-md border border-slate-200 bg-white px-2 py-1 text-xs text-slate-700">
              <span className="font-semibold">{moduleKey}</span>
              <span>{health?.strict_module_coverage?.[moduleKey] ?? 0}</span>
            </span>
          ))}
        </div>
        <p className="mt-2 text-xs text-slate-500">
          {missingModules.length ? `${t('systemAdmin.missingStrictModules')}: ${missingModules.join(', ')}` : t('systemAdmin.noMissingStrictModules')}
        </p>
      </div>
    </section>
  )
}

function StatusBadge({ label }: { label: string }) {
  const { t } = useI18n()
  const tone =
    label === 'active' || label === 'applied' || label === 'passed'
      ? 'border-emerald-200 bg-emerald-50 text-emerald-700'
      : label === 'pending' || label === 'approved' || label === 'warning'
        ? 'border-amber-200 bg-amber-50 text-amber-700'
        : label === 'failed' || label === 'error'
          ? 'border-red-200 bg-red-50 text-red-700'
          : 'border-slate-200 bg-slate-50 text-slate-700'

  return <span className={`inline-flex h-7 items-center rounded-full border px-2.5 text-xs font-semibold ${tone}`}>{t(label)}</span>
}
