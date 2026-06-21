'use client'

import {
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
import { ChangeEvent, useCallback, useEffect, useMemo, useState } from 'react'
import { AIAssistant } from './ai-assistant'
import { ApiWorkbench } from './api-workbench'
import {
  applySchemaChange,
  applyIndustryPackageToOrganization,
  approveSchemaChange,
  closePlatformOrganization,
  createIndustryExtension,
  createOrganizationSchemaChange,
  createOrganizationInvitation,
  getPlatformPermissionProfile,
  exportOrganizationSchema,
  getOrganizationIndustry,
  getOrganizationEntitlements,
  getOrganizationSubscription,
  listIndustries,
  listIndustryExtensions,
  listIndustryPackages,
  listIndustryPublicationRequests,
  listOrganizationInvitations,
  listOrganizationSchemaTargets,
  listPlatformDetails,
  listPlatformMasters,
  listPlatformOrganizations,
  listSaaSModules,
  reviewIndustryPublicationRequest,
  submitIndustryExtensionPublication,
  type Industry,
  type IndustryExtension,
  type IndustryPackage,
  type IndustryPublicationRequest,
  type OrganizationIndustryAdoption,
  updateOrganizationModules,
  type OrganizationInvitation,
  type OrganizationSubscription,
  type OrganizationSchemaTarget,
  type PlatformDetail,
  type PlatformMaster,
  type PlatformPermissionProfile,
  type SaaSModule,
  type SchemaApplyJob,
  type SchemaChangeRequest,
  type SchemaPackage,
  type SessionOrganization,
} from '@/lib/api'
import { useI18n } from '@/lib/i18n'

interface SystemAdminWorkspaceProps {
  token: string
  organizations: SessionOrganization[]
  currentOrganizationID?: string | null
}

type TabID = 'assistant' | 'saas' | 'industry' | 'features' | 'permissions' | 'runtime' | 'catalog' | 'targets' | 'schema'

const tabs: Array<{ id: TabID; label: string; icon: typeof Database; permission?: string }> = [
  { id: 'assistant', label: 'systemAdmin.platformAssistant', icon: Bot, permission: 'assistant.platform.run' },
  { id: 'saas', label: 'systemAdmin.saasOrganizations', icon: Users, permission: 'platform.read' },
  { id: 'industry', label: 'systemAdmin.industries', icon: Layers3, permission: 'platform.read' },
  { id: 'features', label: 'systemAdmin.platformFeatures', icon: ShieldCheck, permission: 'platform.read' },
  { id: 'permissions', label: 'systemAdmin.permissions', icon: ShieldCheck, permission: 'platform.read' },
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

function parseSchemaPackage(source: string): SchemaPackage {
  const parsed = JSON.parse(source) as Partial<SchemaPackage>
  if (!parsed || typeof parsed !== 'object' || !Array.isArray(parsed.tables)) {
    throw new Error('invalid schema package')
  }
  return parsed as SchemaPackage
}

export function SystemAdminWorkspace({ token, organizations, currentOrganizationID }: SystemAdminWorkspaceProps) {
  const { t } = useI18n()
  const [activeTab, setActiveTab] = useState<TabID>('assistant')
  const [moduleKey, setModuleKey] = useState('data_catalog')
  const [platformPermissions, setPlatformPermissions] = useState<PlatformPermissionProfile | null>(null)
  const [platformOrganizations, setPlatformOrganizations] = useState<SessionOrganization[]>([])
  const [saasModules, setSaaSModules] = useState<SaaSModule[]>([])
  const [industries, setIndustries] = useState<Industry[]>([])
  const [industryPackages, setIndustryPackages] = useState<IndustryPackage[]>([])
  const [industryAdoption, setIndustryAdoption] = useState<OrganizationIndustryAdoption | null>(null)
  const [industryExtensions, setIndustryExtensions] = useState<IndustryExtension[]>([])
  const [publicationRequests, setPublicationRequests] = useState<IndustryPublicationRequest[]>([])
  const [selectedIndustryKey, setSelectedIndustryKey] = useState('general')
  const [selectedPackageID, setSelectedPackageID] = useState('')
  const [industryModuleDraft, setIndustryModuleDraft] = useState<string[]>([])
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
  const [showClosedOrganizations, setShowClosedOrganizations] = useState(false)
  const [closeReason, setCloseReason] = useState('')
  const [masters, setMasters] = useState<PlatformMaster[]>([])
  const [details, setDetails] = useState<PlatformDetail[]>([])
  const [targets, setTargets] = useState<OrganizationSchemaTarget[]>([])
  const [selectedMasterKey, setSelectedMasterKey] = useState('')
  const [selectedOrganizationID, setSelectedOrganizationID] = useState(currentOrganizationID || organizations[0]?.id || '')
  const [schemaPackage, setSchemaPackage] = useState<SchemaPackage | null>(null)
  const [schemaJson, setSchemaJson] = useState('')
  const [changeRequest, setChangeRequest] = useState<SchemaChangeRequest | null>(null)
  const [applyJob, setApplyJob] = useState<SchemaApplyJob | null>(null)
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
  const effectiveActiveTab = visibleTabs.some((tab) => tab.id === activeTab) ? activeTab : visibleTabs[0]?.id

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

  const loadPlatformPermissions = useCallback(async () => {
    try {
      setPlatformPermissions(await getPlatformPermissionProfile(token))
    } catch (err) {
      setError(err instanceof Error ? err.message : t('systemAdmin.loadFailed'))
    }
  }, [t, token])

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

  async function applySelectedIndustryPackage() {
    if (!activeOrganizationID || !selectedIndustryPackage) return
    await run(async () => {
      const adoption = await applyIndustryPackageToOrganization(token, selectedIndustryPackage.id, activeOrganizationID, industryModuleDraft)
      setIndustryAdoption(adoption)
      await loadIndustryManagement()
    }, 'systemAdmin.industryApplied')
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
    }, 'systemAdmin.changeCreated')
  }

  async function approveChange() {
    if (!changeRequest || !canPlatform('schema.approve')) return
    await run(async () => {
      setChangeRequest(await approveSchemaChange(token, changeRequest.id, reason))
    }, 'systemAdmin.changeApproved')
  }

  async function applyChange() {
    if (!changeRequest || !canPlatform('schema.apply')) return
    await run(async () => {
      const job = await applySchemaChange(token, changeRequest.id)
      setApplyJob(job)
    }, 'systemAdmin.changeApplied')
  }

  return (
    <div className="space-y-5">
      <div className="flex flex-wrap gap-2 rounded-lg border border-slate-200 bg-white p-2 shadow-sm">
        {visibleTabs.map((tab) => {
          const Icon = tab.icon
          return (
            <button
              key={tab.id}
              type="button"
              onClick={() => setActiveTab(tab.id)}
              className={`inline-flex h-10 items-center gap-2 rounded-md px-3 text-sm font-semibold transition ${
                effectiveActiveTab === tab.id ? 'bg-slate-950 text-white' : 'text-slate-600 hover:bg-slate-100 hover:text-slate-950'
              }`}
            >
              <Icon className="h-4 w-4" />
              {t(tab.label)}
            </button>
          )
        })}
        <button
          type="button"
          onClick={() => {
            void loadPlatformPermissions()
            void loadSaaSManagement()
            void loadOrganizationSaaSDetails()
            void loadIndustryManagement()
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
                      <tr key={item.id}>
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
          <div className="mt-4 grid gap-3 md:grid-cols-2 xl:grid-cols-3">
            {platformFeatureTabs.map((feature) => {
              const permissionAllowed = canPlatform(feature.permission)
              const organizationEnabled = feature.moduleKey ? !!entitlements[feature.moduleKey] : false
              return (
                <div key={feature.id} className="rounded-lg border border-slate-200 bg-slate-50 p-4">
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0">
                      <h3 className="truncate text-sm font-semibold text-slate-950">{t(feature.label)}</h3>
                      <p className="mt-1 truncate text-xs text-slate-500">{feature.moduleKey || feature.permission}</p>
                    </div>
                    <StatusBadge label={permissionAllowed ? 'active' : 'disabled'} />
                  </div>
                  <div className="mt-4 grid gap-2">
                    <Metric label={t('systemAdmin.platformPermission')} value={t(`systemAdmin.permission.${feature.permission}`)} />
                    <Metric label={t('systemAdmin.organizationEntitlement')} value={organizationEnabled ? t('active') : t('disabled')} />
                  </div>
                </div>
              )
            })}
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
          <div className="mt-5 overflow-hidden rounded-lg border border-slate-200">
            <table className="min-w-full divide-y divide-slate-200 text-sm">
              <thead className="bg-slate-50 text-left text-xs font-semibold uppercase text-slate-500">
                <tr>
                  <th className="px-3 py-2">{t('systemAdmin.permissions')}</th>
                  <th className="px-3 py-2">{t('systemAdmin.permissionKey')}</th>
                  <th className="px-3 py-2">{t('systemAdmin.status')}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {platformPermissionCatalog.map((permission) => {
                  const enabled = !!platformPermissions?.permissions[permission]
                  return (
                    <tr key={permission}>
                      <td className="px-3 py-3 font-medium text-slate-900">{t(`systemAdmin.permission.${permission}`)}</td>
                      <td className="px-3 py-3 font-mono text-xs text-slate-600">{permission}</td>
                      <td className="px-3 py-3">
                        <StatusBadge label={enabled ? 'active' : 'disabled'} />
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
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
                  onClick={() => void applyChange()}
                  disabled={!changeRequest || !['approved', 'applied'].includes(changeRequest.status) || loading || !canPlatform('schema.apply')}
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
                  <pre className="max-h-56 overflow-auto rounded-lg bg-slate-50 p-3 text-xs text-slate-600">
                    {changeRequest.statements.join('\n\n') || t('common.empty')}
                  </pre>
                </div>
              ) : (
                <p className="mt-4 rounded-lg border border-dashed border-slate-300 p-4 text-sm text-slate-500">{t('systemAdmin.noChangeRequest')}</p>
              )}
            </section>

            {applyJob && (
              <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
                <h2 className="text-base font-semibold text-slate-950">{t('systemAdmin.applyJob')}</h2>
                <div className="mt-4 space-y-3">
                  <Metric label={t('systemAdmin.status')} value={t(applyJob.status)} />
                  <Metric label={t('systemAdmin.statementCount')} value={String(applyJob.statements.length)} />
                  {applyJob.error_message && <p className="rounded-lg bg-red-50 p-3 text-sm text-red-700">{applyJob.error_message}</p>}
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

function StatusBadge({ label }: { label: string }) {
  const { t } = useI18n()
  const tone =
    label === 'active' || label === 'applied'
      ? 'border-emerald-200 bg-emerald-50 text-emerald-700'
      : label === 'pending' || label === 'approved'
        ? 'border-amber-200 bg-amber-50 text-amber-700'
        : 'border-slate-200 bg-slate-50 text-slate-700'

  return <span className={`inline-flex h-7 items-center rounded-full border px-2.5 text-xs font-semibold ${tone}`}>{t(label)}</span>
}
