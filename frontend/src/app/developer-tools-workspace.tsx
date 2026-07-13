'use client'

import { Activity, Bot, FileCode2, Gauge, KeyRound, Network, Plus, RefreshCw, Route, ServerCog, Wrench } from 'lucide-react'
import { FormEvent, useCallback, useEffect, useMemo, useState } from 'react'
import type { ReactNode } from 'react'
import {
  adjustAIGatewayBalance,
  createModel,
  createInterfaceFile,
  createModelProvider,
  createProviderChannel,
  createAIAccessToken,
  createAIModelGroup,
  createAIModelChannelAbility,
  createRoutingRule,
  getAICostSummary,
  getAIGatewayBalance,
  getAIUsageAnalysis,
  listAIAccessTokens,
  listAIAdapters,
  listAIBalanceTransactions,
  listAIModelChannelAbilities,
  listAIModelGroups,
  listInterfaceFiles,
  listInvocations,
  listModelProviders,
  listModels,
  listPlatformModelProviders,
  listPlatformModels,
  listProviderChannels,
  listRoutingRules,
  listToolExecutions,
  listTools,
  rotateModelProviderKey,
  rotateProviderChannelKey,
  testModelProvider,
  testProviderChannel,
  updateInterfaceFile,
  type AICostSummary,
  type AIAccessToken,
  type AIAdapterDescriptor,
  type AIBalanceTransaction,
  type AIGatewayBalance,
  type AIInvocation,
  type AIModelChannelAbility,
  type AIModelGroup,
  type AIRoutingRule,
  type InterfaceFile,
  type AIUsageAnalysis,
  type ModelCatalogItem,
  type ModelProvider,
  type ProviderChannel,
  type ToolDefinition,
  type ToolExecution,
} from '@/lib/api'
import { useI18n } from '@/lib/i18n'

interface DeveloperToolsWorkspaceProps {
  token: string
  apiScope?: 'tenant' | 'platform'
}

type TabID = 'overview' | 'providers' | 'channels' | 'models' | 'groups' | 'routing' | 'accessTokens' | 'balance' | 'invocations' | 'analysis' | 'adapters' | 'tools' | 'interfaces'
type InterfaceFileType = InterfaceFile['file_type']

const emptyInterfaceForm = {
  name: '',
  file_type: 'json' as InterfaceFileType,
  content: '',
  metadata: '{}',
}

const tabs: Array<{ id: TabID; label: string; icon: typeof ServerCog }> = [
  { id: 'overview', label: 'developer.gatewayOverview', icon: Gauge },
  { id: 'providers', label: 'developer.providers', icon: ServerCog },
  { id: 'channels', label: 'developer.channels', icon: Network },
  { id: 'models', label: 'developer.models', icon: Bot },
  { id: 'groups', label: 'developer.modelGroups', icon: Network },
  { id: 'routing', label: 'developer.routing', icon: Route },
  { id: 'accessTokens', label: 'developer.accessTokens', icon: KeyRound },
  { id: 'balance', label: 'developer.balanceAndCost', icon: Gauge },
  { id: 'invocations', label: 'developer.invocations', icon: Activity },
  { id: 'analysis', label: 'developer.usageAnalysis', icon: Gauge },
  { id: 'adapters', label: 'developer.adapterMatrix', icon: FileCode2 },
  { id: 'tools', label: 'developer.tools', icon: Wrench },
  { id: 'interfaces', label: 'developer.interfaces', icon: FileCode2 },
]

function money(value: number | undefined, currency = 'CNY'): string {
  return `${currency} ${Number(value ?? 0).toFixed(4)}`
}

export function DeveloperToolsWorkspace({ token, apiScope = 'tenant' }: DeveloperToolsWorkspaceProps) {
  const { t } = useI18n()
  const isPlatformScope = apiScope === 'platform'
  const [activeTab, setActiveTab] = useState<TabID>('providers')
  const [providers, setProviders] = useState<ModelProvider[]>([])
  const [channels, setChannels] = useState<ProviderChannel[]>([])
  const [models, setModels] = useState<ModelCatalogItem[]>([])
  const [rules, setRules] = useState<AIRoutingRule[]>([])
  const [tools, setTools] = useState<ToolDefinition[]>([])
  const [interfaceFiles, setInterfaceFiles] = useState<InterfaceFile[]>([])
  const [executions, setExecutions] = useState<ToolExecution[]>([])
  const [invocations, setInvocations] = useState<AIInvocation[]>([])
  const [cost, setCost] = useState<AICostSummary | null>(null)
  const [usageAnalysis, setUsageAnalysis] = useState<AIUsageAnalysis | null>(null)
  const [modelGroups, setModelGroups] = useState<AIModelGroup[]>([])
  const [modelChannelAbilities, setModelChannelAbilities] = useState<AIModelChannelAbility[]>([])
  const [accessTokens, setAccessTokens] = useState<AIAccessToken[]>([])
  const [gatewayBalance, setGatewayBalance] = useState<AIGatewayBalance | null>(null)
  const [balanceTransactions, setBalanceTransactions] = useState<AIBalanceTransaction[]>([])
  const [adapters, setAdapters] = useState<AIAdapterDescriptor[]>([])
  const [selectedProviderID, setSelectedProviderID] = useState('')
  const [selectedChannelID, setSelectedChannelID] = useState('')
  const [selectedOrganizationID, setSelectedOrganizationID] = useState('')
  const [selectedInterfaceFileID, setSelectedInterfaceFileID] = useState('')
  const [notice, setNotice] = useState('')
  const [error, setError] = useState('')
  const [lastPlainAccessToken, setLastPlainAccessToken] = useState('')
  const [loading, setLoading] = useState(false)
  const [providerForm, setProviderForm] = useState({
    name: '',
    provider_type: 'openai' as 'openai' | 'anthropic' | 'gemini',
    base_url: '',
    api_key: '',
    risk_level: 'medium',
    timeout_ms: '60000',
    retry_count: '1',
    tags: '',
  })
  const [channelForm, setChannelForm] = useState({
    provider_id: '',
    name: '',
    base_url: '',
    api_key: '',
    owner_type: '',
    priority: '50',
    concurrency_limit: '0',
    load_factor: '1',
    rate_multiplier: '1',
    quota_amount: '0',
    quota_currency: 'CNY',
    supported_model_patterns: '*',
    model_mapping: '',
  })
  const [modelForm, setModelForm] = useState({
    provider_id: '',
    model_key: '',
    display_name: '',
    context_window: '',
    max_output_tokens: '',
    input_price_per_1k: '',
    output_price_per_1k: '',
    cache_creation_price_per_1k: '',
    cache_read_price_per_1k: '',
    image_output_price_per_1k: '',
    priority_input_price_per_1k: '',
    priority_output_price_per_1k: '',
    long_context_threshold: '',
    long_context_input_multiplier: '',
    long_context_output_multiplier: '',
    currency: 'CNY',
    capabilities: '',
  })
  const [routingForm, setRoutingForm] = useState({
    name: '',
    provider_id: '',
    channel_id: '',
    match_scope: 'global',
    match_value: '',
    model_pattern: '*',
    priority: '100',
  })
  const [modelGroupForm, setModelGroupForm] = useState({
    organization_id: '',
    name: '',
    group_key: '',
    rate_multiplier: '1',
  })
  const [modelAbilityForm, setModelAbilityForm] = useState({
    model_group_id: '',
    channel_id: '',
    requested_model: '',
    model_pattern: '*',
    upstream_model: '',
    priority: '100',
  })
  const [accessTokenForm, setAccessTokenForm] = useState({
    organization_id: '',
    model_group_id: '',
    name: '',
    allowed_models: '*',
    quota_amount: '0',
    quota_currency: 'CNY',
    allow_channel_override: false,
  })
  const [balanceForm, setBalanceForm] = useState({
    organization_id: '',
    amount: '',
    currency: 'CNY',
    reason: '',
  })
  const [interfaceForm, setInterfaceForm] = useState(emptyInterfaceForm)
  const [secretInput, setSecretInput] = useState('')
  const [channelSecretInput, setChannelSecretInput] = useState('')
  const [testModel, setTestModel] = useState('')
  const [channelTestModel, setChannelTestModel] = useState('')

  const providerLabels = useMemo(() => Object.fromEntries(providers.map((provider) => [provider.id, provider.name])), [providers])
  const channelLabels = useMemo(() => Object.fromEntries(channels.map((channel) => [channel.id, channel.name])), [channels])
  const groupLabels = useMemo(() => Object.fromEntries(modelGroups.map((group) => [group.id, group.name])), [modelGroups])
  const visibleTabs = useMemo(
    () =>
      isPlatformScope
        ? tabs.filter((tab) => ['overview', 'providers', 'channels', 'models', 'groups', 'accessTokens', 'balance', 'adapters'].includes(tab.id))
        : tabs.filter((tab) => !['groups', 'accessTokens', 'balance', 'adapters'].includes(tab.id)),
    [isPlatformScope],
  )
  const effectiveActiveTab = visibleTabs.some((tab) => tab.id === activeTab) ? activeTab : visibleTabs[0]?.id ?? 'providers'
  const selectedProvider = useMemo(
    () => providers.find((provider) => provider.id === selectedProviderID) ?? providers[0],
    [providers, selectedProviderID],
  )
  const selectedChannel = useMemo(
    () => channels.find((channel) => channel.id === selectedChannelID) ?? channels[0],
    [channels, selectedChannelID],
  )
  const loadAll = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const loadProviders = isPlatformScope ? listPlatformModelProviders : listModelProviders
      const loadModels = isPlatformScope ? listPlatformModels : listModels
      const organizationID = selectedOrganizationID || modelGroupForm.organization_id || accessTokenForm.organization_id
      const [
        providerData,
        channelData,
        modelData,
        ruleData,
        toolData,
        interfaceFileData,
        executionData,
        invocationData,
        costData,
        analysisData,
        modelGroupData,
        modelAbilityData,
        accessTokenData,
        adapterData,
        balanceData,
        balanceTransactionData,
      ] = await Promise.all([
        loadProviders(token),
        listProviderChannels(token, undefined, apiScope),
        loadModels(token),
        isPlatformScope ? Promise.resolve<AIRoutingRule[]>([]) : listRoutingRules(token),
        isPlatformScope ? Promise.resolve<ToolDefinition[]>([]) : listTools(token),
        isPlatformScope ? Promise.resolve<InterfaceFile[]>([]) : listInterfaceFiles(token),
        isPlatformScope ? Promise.resolve<ToolExecution[]>([]) : listToolExecutions(token),
        isPlatformScope ? Promise.resolve<AIInvocation[]>([]) : listInvocations(token),
        isPlatformScope ? Promise.resolve<AICostSummary | null>(null) : getAICostSummary(token),
        isPlatformScope ? Promise.resolve<AIUsageAnalysis | null>(null) : getAIUsageAnalysis(token),
        isPlatformScope ? listAIModelGroups(token, organizationID || undefined, apiScope) : Promise.resolve<AIModelGroup[]>([]),
        isPlatformScope ? listAIModelChannelAbilities(token, modelAbilityForm.model_group_id || undefined, apiScope) : Promise.resolve<AIModelChannelAbility[]>([]),
        isPlatformScope ? listAIAccessTokens(token, organizationID || undefined, apiScope) : Promise.resolve<AIAccessToken[]>([]),
        isPlatformScope ? listAIAdapters(token, apiScope) : Promise.resolve<AIAdapterDescriptor[]>([]),
        isPlatformScope && organizationID ? getAIGatewayBalance(token, organizationID, 'CNY', apiScope) : Promise.resolve<AIGatewayBalance | null>(null),
        isPlatformScope && organizationID ? listAIBalanceTransactions(token, organizationID, apiScope) : Promise.resolve<AIBalanceTransaction[]>([]),
      ])
      setProviders(providerData)
      setChannels(channelData)
      setModels(modelData)
      setRules(ruleData)
      setTools(toolData)
      setInterfaceFiles(interfaceFileData)
      setExecutions(executionData)
      setInvocations(invocationData)
      setCost(costData)
      setUsageAnalysis(analysisData)
      setModelGroups(modelGroupData)
      setModelChannelAbilities(modelAbilityData)
      setAccessTokens(accessTokenData)
      setAdapters(adapterData)
      setGatewayBalance(balanceData)
      setBalanceTransactions(balanceTransactionData)
      setSelectedProviderID((current) => current || providerData[0]?.id || '')
      setSelectedChannelID((current) => current || channelData[0]?.id || '')
      setModelForm((current) => ({ ...current, provider_id: current.provider_id || providerData[0]?.id || '' }))
      setChannelForm((current) => ({ ...current, provider_id: current.provider_id || providerData[0]?.id || '' }))
      setRoutingForm((current) => ({ ...current, provider_id: current.provider_id || providerData[0]?.id || '' }))
      setSelectedOrganizationID((current) => current || organizationID || accessTokenData[0]?.organization_id || modelGroupData[0]?.organization_id || '')
      setModelAbilityForm((current) => ({
        ...current,
        model_group_id: current.model_group_id || modelGroupData[0]?.id || '',
        channel_id: current.channel_id || channelData[0]?.id || '',
      }))
      setBalanceForm((current) => ({ ...current, organization_id: current.organization_id || organizationID || '' }))
    } catch (err) {
      setError(err instanceof Error ? err.message : t('developer.loadFailed'))
    } finally {
      setLoading(false)
    }
  }, [accessTokenForm.organization_id, apiScope, isPlatformScope, modelAbilityForm.model_group_id, modelGroupForm.organization_id, selectedOrganizationID, t, token])

  useEffect(() => {
    const timer = window.setTimeout(() => {
      void loadAll()
    }, 0)
    return () => window.clearTimeout(timer)
  }, [loadAll])

  async function run(action: () => Promise<void>, success: string) {
    setLoading(true)
    setError('')
    setNotice('')
    try {
      await action()
      setNotice(t(success))
      await loadAll()
    } catch (err) {
      setError(err instanceof Error ? err.message : t('common.operationFailed'))
    } finally {
      setLoading(false)
    }
  }

  async function submitProvider(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await run(
      () =>
        createModelProvider(token, {
          name: providerForm.name,
          provider_type: providerForm.provider_type,
          base_url: providerForm.base_url || undefined,
          api_key: providerForm.api_key,
          risk_level: providerForm.risk_level,
          timeout_ms: Number(providerForm.timeout_ms || 60000),
          retry_count: Number(providerForm.retry_count || 1),
          tags: splitCsv(providerForm.tags),
          metadata: {},
        }, apiScope).then(() => setProviderForm((current) => ({ ...current, api_key: '' }))),
      'developer.providerCreated',
    )
  }

  async function submitChannel(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const providerID = channelForm.provider_id || selectedProvider?.id
    if (!providerID) return
    await run(
      () =>
        createProviderChannel(token, providerID, {
          provider_id: providerID,
          name: channelForm.name,
          base_url: channelForm.base_url || undefined,
          api_key: channelForm.api_key,
          owner_type: channelForm.owner_type || undefined,
          priority: numberOrUndefined(channelForm.priority),
          concurrency_limit: numberOrUndefined(channelForm.concurrency_limit),
          load_factor: numberOrUndefined(channelForm.load_factor),
          rate_multiplier: numberOrUndefined(channelForm.rate_multiplier),
          quota_amount: numberOrUndefined(channelForm.quota_amount),
          quota_currency: channelForm.quota_currency || undefined,
          supported_model_patterns: splitCsv(channelForm.supported_model_patterns),
          model_mapping: parseMapping(channelForm.model_mapping),
          metadata: {},
        }, apiScope).then(() => setChannelForm((current) => ({ ...current, name: '', api_key: '' }))),
      'developer.channelCreated',
    )
  }

  async function submitModel(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await run(
      () =>
        createModel(token, {
          provider_id: modelForm.provider_id,
          model_key: modelForm.model_key,
          display_name: modelForm.display_name,
          context_window: Number(modelForm.context_window || 0),
          max_output_tokens: Number(modelForm.max_output_tokens || 0),
          input_price_per_1k: Number(modelForm.input_price_per_1k || 0),
          output_price_per_1k: Number(modelForm.output_price_per_1k || 0),
          cache_creation_price_per_1k: numberOrUndefined(modelForm.cache_creation_price_per_1k),
          cache_read_price_per_1k: numberOrUndefined(modelForm.cache_read_price_per_1k),
          image_output_price_per_1k: numberOrUndefined(modelForm.image_output_price_per_1k),
          priority_input_price_per_1k: numberOrUndefined(modelForm.priority_input_price_per_1k),
          priority_output_price_per_1k: numberOrUndefined(modelForm.priority_output_price_per_1k),
          long_context_threshold: numberOrUndefined(modelForm.long_context_threshold),
          long_context_input_multiplier: numberOrUndefined(modelForm.long_context_input_multiplier),
          long_context_output_multiplier: numberOrUndefined(modelForm.long_context_output_multiplier),
          currency: modelForm.currency,
          capabilities: splitCsv(modelForm.capabilities),
          metadata: {},
        }, apiScope).then(() => setModelForm((current) => ({ ...current, model_key: '', display_name: '' }))),
      'developer.modelCreated',
    )
  }

  async function submitRoutingRule(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await run(
      () =>
        createRoutingRule(token, {
          name: routingForm.name,
          provider_id: routingForm.provider_id || undefined,
          channel_id: routingForm.channel_id || undefined,
          match_scope: routingForm.match_scope,
          match_value: routingForm.match_value || undefined,
          model_pattern: routingForm.model_pattern || '*',
          priority: Number(routingForm.priority || 100),
          status: 'active',
          metadata: {},
        }).then(() => setRoutingForm((current) => ({ ...current, name: '', match_value: '' }))),
      'developer.routingRuleCreated',
    )
  }

  async function submitModelGroup(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const organizationID = modelGroupForm.organization_id || undefined
    await run(
      () =>
        createAIModelGroup(token, {
          organization_id: organizationID,
          name: modelGroupForm.name,
          group_key: modelGroupForm.group_key || undefined,
          rate_multiplier: Number(modelGroupForm.rate_multiplier || 1),
          status: 'active',
          metadata: {},
        }, apiScope).then((group) => {
          setSelectedOrganizationID(group.organization_id || organizationID || '')
          setModelGroupForm((current) => ({ ...current, name: '', group_key: '' }))
        }),
      'developer.modelGroupCreated',
    )
  }

  async function submitModelAbility(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!modelAbilityForm.channel_id) return
    await run(
      () =>
        createAIModelChannelAbility(token, {
          model_group_id: modelAbilityForm.model_group_id || undefined,
          channel_id: modelAbilityForm.channel_id,
          requested_model: modelAbilityForm.requested_model || undefined,
          model_pattern: modelAbilityForm.model_pattern || '*',
          upstream_model: modelAbilityForm.upstream_model || undefined,
          priority: Number(modelAbilityForm.priority || 100),
          enabled: true,
          metadata: {},
        }, apiScope).then(() => setModelAbilityForm((current) => ({ ...current, requested_model: '', upstream_model: '' }))),
      'developer.modelAbilityCreated',
    )
  }

  async function submitAccessToken(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await run(
      () =>
        createAIAccessToken(token, {
          organization_id: accessTokenForm.organization_id,
          model_group_id: accessTokenForm.model_group_id || undefined,
          name: accessTokenForm.name,
          allowed_models: splitCsv(accessTokenForm.allowed_models),
          quota_amount: numberOrUndefined(accessTokenForm.quota_amount),
          quota_currency: accessTokenForm.quota_currency || 'CNY',
          allow_channel_override: accessTokenForm.allow_channel_override,
          metadata: {},
        }, apiScope).then((created) => {
          setSelectedOrganizationID(created.organization_id)
          setAccessTokenForm((current) => ({ ...current, name: '' }))
          if (created.plain_token) setLastPlainAccessToken(created.plain_token)
        }),
      'developer.accessTokenCreated',
    )
  }

  async function submitBalanceAdjustment(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await run(
      () =>
        adjustAIGatewayBalance(token, {
          organization_id: balanceForm.organization_id || selectedOrganizationID,
          amount: Number(balanceForm.amount || 0),
          currency: balanceForm.currency || 'CNY',
          reason: balanceForm.reason,
          metadata: {},
        }, apiScope).then((balance) => {
          setSelectedOrganizationID(balance.organization_id)
          setBalanceForm((current) => ({ ...current, organization_id: balance.organization_id, amount: '', reason: '' }))
        }),
      'developer.balanceAdjusted',
    )
  }

  function startNewInterfaceFile() {
    setSelectedInterfaceFileID('')
    setInterfaceForm(emptyInterfaceForm)
  }

  function selectInterfaceFile(file: InterfaceFile) {
    setSelectedInterfaceFileID(file.id)
    setInterfaceForm({
      name: file.name,
      file_type: file.file_type,
      content: file.content,
      metadata: JSON.stringify(file.metadata ?? {}, null, 2),
    })
  }

  function applyInterfaceTemplate(nameKey: string, value: unknown) {
    setSelectedInterfaceFileID('')
    setInterfaceForm({
      name: t(nameKey),
      file_type: 'json',
      content: JSON.stringify(value, null, 2),
      metadata: JSON.stringify({ template: nameKey }, null, 2),
    })
  }

  async function submitInterfaceFile(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const metadata = parseJSONMap(interfaceForm.metadata)
    if (!metadata) {
      setError(t('developer.metadataInvalid'))
      setNotice('')
      return
    }
    const payload = {
      name: interfaceForm.name,
      file_type: interfaceForm.file_type,
      content: interfaceForm.content,
      metadata,
    }
    await run(async () => {
      const saved = selectedInterfaceFileID
        ? await updateInterfaceFile(token, selectedInterfaceFileID, payload)
        : await createInterfaceFile(token, payload)
      setSelectedInterfaceFileID(saved.id)
      setInterfaceForm({
        name: saved.name,
        file_type: saved.file_type,
        content: saved.content,
        metadata: JSON.stringify(saved.metadata ?? {}, null, 2),
      })
    }, selectedInterfaceFileID ? 'developer.interfaceFileSaved' : 'developer.interfaceFileCreated')
  }

  async function rotateKey() {
    if (!selectedProvider || !secretInput) return
    await run(
      () => rotateModelProviderKey(token, selectedProvider.id, secretInput, apiScope).then(() => setSecretInput('')),
      'developer.keyRotated',
    )
  }

  async function rotateChannelKey() {
    if (!selectedChannel || !channelSecretInput) return
    await run(
      () => rotateProviderChannelKey(token, selectedChannel.id, channelSecretInput, apiScope).then(() => setChannelSecretInput('')),
      'developer.channelKeyRotated',
    )
  }

  async function testProvider() {
    if (!selectedProvider) return
    await run(() => testModelProvider(token, selectedProvider.id, testModel || undefined, apiScope).then(() => undefined), 'developer.providerTested')
  }

  async function testChannel() {
    if (!selectedChannel) return
    await run(() => testProviderChannel(token, selectedChannel.id, channelTestModel || undefined, apiScope).then(() => undefined), 'developer.channelTested')
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
                effectiveActiveTab === tab.id ? 'bg-slate-950 text-white' : 'text-slate-600 hover:bg-slate-100'
              }`}
            >
              <Icon className="h-4 w-4" />
              {t(tab.label)}
            </button>
          )
        })}
        <button
          type="button"
          onClick={() => void loadAll()}
          disabled={loading}
          className="ml-auto inline-flex h-10 items-center gap-2 rounded-md border border-slate-300 px-3 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:opacity-50"
        >
          <RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
          {t('common.refresh')}
        </button>
      </div>

      {(error || notice) && (
        <div
          className={`rounded-lg border px-3 py-2 text-sm ${
            error ? 'border-red-200 bg-red-50 text-red-700' : 'border-emerald-200 bg-emerald-50 text-emerald-700'
          }`}
        >
          {error || notice}
        </div>
      )}

      {effectiveActiveTab === 'overview' && (
        <div className="space-y-5">
          <Panel title="developer.gatewayOverview">
            <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
              <Metric label="developer.providers" value={String(providers.length)} />
              <Metric label="developer.channels" value={String(channels.length)} />
              <Metric label="developer.models" value={String(models.length)} />
              <Metric label="developer.accessTokens" value={String(accessTokens.length)} />
              <Metric label="developer.adapterMatrix" value={String(adapters.length)} />
            </div>
            <div className="mt-4 max-w-xl">
              <TextInput
                label="developer.organizationId"
                value={selectedOrganizationID}
                onChange={(value) => {
                  setSelectedOrganizationID(value)
                  setModelGroupForm((current) => ({ ...current, organization_id: value }))
                  setAccessTokenForm((current) => ({ ...current, organization_id: value }))
                }}
              />
            </div>
          </Panel>
        </div>
      )}

      {effectiveActiveTab === 'providers' && (
        <div className={`grid gap-5 ${isPlatformScope ? '' : 'xl:grid-cols-[minmax(0,1fr)_380px]'}`}>
          <Panel title="developer.modelProviders">
            <div className="divide-y divide-slate-100">
              {providers.map((provider) => (
                <button
                  key={provider.id}
                  type="button"
                  onClick={() => setSelectedProviderID(provider.id)}
                  className={`grid w-full gap-2 py-3 text-left md:grid-cols-[1fr_auto] ${
                    selectedProvider?.id === provider.id ? 'text-slate-950' : 'text-slate-700'
                  }`}
                >
                  <div className="min-w-0">
                    <p className="truncate text-sm font-semibold">{provider.name}</p>
                    <p className="mt-1 truncate text-xs text-slate-500">
                      {t(`provider.${provider.provider_type}`)} · {provider.masked_api_key || t('common.none')}
                    </p>
                  </div>
                  <StatusBadge label={provider.last_test_status || provider.status} />
                </button>
              ))}
              {providers.length === 0 && <EmptyText>{t('developer.noProviders')}</EmptyText>}
            </div>
          </Panel>

          {!isPlatformScope && (
          <Panel title="developer.providerSettings">
            <form className="space-y-3" onSubmit={submitProvider}>
              <TextInput label="common.name" value={providerForm.name} onChange={(value) => setProviderForm({ ...providerForm, name: value })} />
              <SelectInput
                label="developer.providerType"
                value={providerForm.provider_type}
                onChange={(value) => setProviderForm({ ...providerForm, provider_type: value as 'openai' | 'anthropic' | 'gemini' })}
                options={['openai', 'anthropic', 'gemini']}
              />
              <TextInput label="developer.baseUrl" value={providerForm.base_url} onChange={(value) => setProviderForm({ ...providerForm, base_url: value })} />
              <TextInput label="developer.apiKey" type="password" value={providerForm.api_key} onChange={(value) => setProviderForm({ ...providerForm, api_key: value })} />
              <div className="grid gap-3 sm:grid-cols-2">
                <TextInput label="developer.timeout" value={providerForm.timeout_ms} onChange={(value) => setProviderForm({ ...providerForm, timeout_ms: value })} />
                <TextInput label="developer.retries" value={providerForm.retry_count} onChange={(value) => setProviderForm({ ...providerForm, retry_count: value })} />
              </div>
              <TextInput label="developer.tags" value={providerForm.tags} onChange={(value) => setProviderForm({ ...providerForm, tags: value })} />
              <SubmitButton loading={loading} label="developer.createProvider" />
            </form>

            <div className="mt-5 border-t border-slate-100 pt-4">
              <p className="text-sm font-semibold text-slate-950">{t('developer.keyRotation')}</p>
              <TextInput label="developer.newKey" type="password" value={secretInput} onChange={setSecretInput} />
              <button
                type="button"
                onClick={() => void rotateKey()}
                disabled={!selectedProvider || !secretInput || loading}
                className="mt-3 inline-flex h-10 w-full items-center justify-center gap-2 rounded-lg border border-slate-300 px-3 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:opacity-50"
              >
                <KeyRound className="h-4 w-4" />
                {t('developer.rotateKey')}
              </button>
              <TextInput label="developer.testModel" value={testModel} onChange={setTestModel} />
              <button
                type="button"
                onClick={() => void testProvider()}
                disabled={!selectedProvider || loading}
                className="mt-3 inline-flex h-10 w-full items-center justify-center rounded-lg bg-slate-950 px-3 text-sm font-semibold text-white hover:bg-slate-800 disabled:opacity-50"
              >
                {t('developer.testProvider')}
              </button>
            </div>
          </Panel>
          )}
        </div>
      )}

      {effectiveActiveTab === 'channels' && (
        <div className="grid gap-5 2xl:grid-cols-[minmax(0,1fr)_420px]">
          <Panel title="developer.channelPool">
            <Table
              headers={['developer.channel', 'developer.provider', 'developer.reliability', 'developer.load', 'developer.quota', 'developer.rateMultiplier']}
              rows={channels.map((channel) => [
                channel.name,
                providerLabels[channel.provider_id] || channel.provider_id,
                `${t(channel.health_status || channel.status)} · ${channel.success_count}/${channel.failure_count} (${channel.consecutive_failure_count})${
                  channel.circuit_open_until ? ` · ${new Date(channel.circuit_open_until).toLocaleString()}` : ''
                }`,
                `${channel.inflight_requests}/${channel.concurrency_limit || t('common.none')}`,
                `${money(channel.quota_used, channel.quota_currency)} / ${channel.quota_amount || t('common.none')}`,
                String(channel.rate_multiplier),
              ])}
            />
          </Panel>

          <Panel title="developer.channelSettings">
            <form className="space-y-3" onSubmit={submitChannel}>
              <SelectInput label="developer.provider" value={channelForm.provider_id} onChange={(value) => setChannelForm({ ...channelForm, provider_id: value })} options={providers.map((provider) => provider.id)} labels={providerLabels} />
              <TextInput label="developer.channel" value={channelForm.name} onChange={(value) => setChannelForm({ ...channelForm, name: value })} />
              <TextInput label="developer.baseUrl" value={channelForm.base_url} onChange={(value) => setChannelForm({ ...channelForm, base_url: value })} />
              <TextInput label="developer.apiKey" type="password" value={channelForm.api_key} onChange={(value) => setChannelForm({ ...channelForm, api_key: value })} />
              <div className="grid gap-3 sm:grid-cols-2">
                <TextInput label="developer.priority" value={channelForm.priority} onChange={(value) => setChannelForm({ ...channelForm, priority: value })} />
                <TextInput label="developer.rateMultiplier" value={channelForm.rate_multiplier} onChange={(value) => setChannelForm({ ...channelForm, rate_multiplier: value })} />
                <TextInput label="developer.concurrency" value={channelForm.concurrency_limit} onChange={(value) => setChannelForm({ ...channelForm, concurrency_limit: value })} />
                <TextInput label="developer.loadFactor" value={channelForm.load_factor} onChange={(value) => setChannelForm({ ...channelForm, load_factor: value })} />
                <TextInput label="developer.quota" value={channelForm.quota_amount} onChange={(value) => setChannelForm({ ...channelForm, quota_amount: value })} />
                <TextInput label="finance.currency" value={channelForm.quota_currency} onChange={(value) => setChannelForm({ ...channelForm, quota_currency: value })} />
              </div>
              <TextInput label="developer.modelPatterns" value={channelForm.supported_model_patterns} onChange={(value) => setChannelForm({ ...channelForm, supported_model_patterns: value })} />
              <TextInput label="developer.modelMapping" value={channelForm.model_mapping} onChange={(value) => setChannelForm({ ...channelForm, model_mapping: value })} />
              <SubmitButton loading={loading} label="developer.createChannel" />
            </form>

            <div className="mt-5 border-t border-slate-100 pt-4">
              <SelectInput label="developer.channel" value={selectedChannel?.id || ''} onChange={setSelectedChannelID} options={channels.map((channel) => channel.id)} labels={channelLabels} />
              <TextInput label="developer.newKey" type="password" value={channelSecretInput} onChange={setChannelSecretInput} />
              <button
                type="button"
                onClick={() => void rotateChannelKey()}
                disabled={!selectedChannel || !channelSecretInput || loading}
                className="mt-3 inline-flex h-10 w-full items-center justify-center gap-2 rounded-lg border border-slate-300 px-3 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:opacity-50"
              >
                <KeyRound className="h-4 w-4" />
                {t('developer.rotateChannelKey')}
              </button>
              <TextInput label="developer.testModel" value={channelTestModel} onChange={setChannelTestModel} />
              <button
                type="button"
                onClick={() => void testChannel()}
                disabled={!selectedChannel || loading}
                className="mt-3 inline-flex h-10 w-full items-center justify-center rounded-lg bg-slate-950 px-3 text-sm font-semibold text-white hover:bg-slate-800 disabled:opacity-50"
              >
                {t('developer.testChannel')}
              </button>
            </div>
          </Panel>
        </div>
      )}

      {effectiveActiveTab === 'models' && (
        <div className={`grid gap-5 ${isPlatformScope ? '' : 'xl:grid-cols-[minmax(0,1fr)_420px]'}`}>
          <Panel title="developer.modelCatalog">
            <Table
              headers={['developer.model', 'developer.provider', 'developer.status', 'developer.context']}
              rows={models.map((model) => [
                model.display_name || model.model_key,
                providerLabels[model.provider_id] || model.provider_id,
                t(model.status),
                String(model.context_window || 0),
              ])}
            />
          </Panel>
          <Panel title="developer.createModel">
            <form className="space-y-3" onSubmit={submitModel}>
              <SelectInput label="developer.provider" value={modelForm.provider_id} onChange={(value) => setModelForm({ ...modelForm, provider_id: value })} options={providers.map((provider) => provider.id)} labels={providerLabels} />
              <TextInput label="developer.modelKey" value={modelForm.model_key} onChange={(value) => setModelForm({ ...modelForm, model_key: value })} />
              <TextInput label="developer.displayName" value={modelForm.display_name} onChange={(value) => setModelForm({ ...modelForm, display_name: value })} />
              <div className="grid gap-3 sm:grid-cols-2">
                <TextInput label="developer.context" value={modelForm.context_window} onChange={(value) => setModelForm({ ...modelForm, context_window: value })} />
                <TextInput label="developer.maxOutput" value={modelForm.max_output_tokens} onChange={(value) => setModelForm({ ...modelForm, max_output_tokens: value })} />
                <TextInput label="developer.inputPrice" value={modelForm.input_price_per_1k} onChange={(value) => setModelForm({ ...modelForm, input_price_per_1k: value })} />
                <TextInput label="developer.outputPrice" value={modelForm.output_price_per_1k} onChange={(value) => setModelForm({ ...modelForm, output_price_per_1k: value })} />
                <TextInput label="developer.cacheCreationPrice" value={modelForm.cache_creation_price_per_1k} onChange={(value) => setModelForm({ ...modelForm, cache_creation_price_per_1k: value })} />
                <TextInput label="developer.cacheReadPrice" value={modelForm.cache_read_price_per_1k} onChange={(value) => setModelForm({ ...modelForm, cache_read_price_per_1k: value })} />
                <TextInput label="developer.imageOutputPrice" value={modelForm.image_output_price_per_1k} onChange={(value) => setModelForm({ ...modelForm, image_output_price_per_1k: value })} />
                <TextInput label="developer.priorityInputPrice" value={modelForm.priority_input_price_per_1k} onChange={(value) => setModelForm({ ...modelForm, priority_input_price_per_1k: value })} />
                <TextInput label="developer.priorityOutputPrice" value={modelForm.priority_output_price_per_1k} onChange={(value) => setModelForm({ ...modelForm, priority_output_price_per_1k: value })} />
                <TextInput label="developer.longContextThreshold" value={modelForm.long_context_threshold} onChange={(value) => setModelForm({ ...modelForm, long_context_threshold: value })} />
              </div>
              <TextInput label="developer.capabilities" value={modelForm.capabilities} onChange={(value) => setModelForm({ ...modelForm, capabilities: value })} />
              <SubmitButton loading={loading} label="developer.createModel" />
            </form>
          </Panel>
        </div>
      )}

      {effectiveActiveTab === 'groups' && (
        <div className="space-y-5">
          <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_420px]">
            <Panel title="developer.modelGroups">
              <Table
                headers={['common.name', 'developer.organizationId', 'developer.groupKey', 'developer.rateMultiplier', 'developer.status']}
                rows={modelGroups.map((group) => [
                  group.name,
                  group.organization_id || t('common.none'),
                  group.group_key || t('common.none'),
                  String(group.rate_multiplier || 1),
                  t(group.status),
                ])}
              />
            </Panel>
            <Panel title="developer.createModelGroup">
              <form className="space-y-3" onSubmit={submitModelGroup}>
                <TextInput label="developer.organizationId" value={modelGroupForm.organization_id} onChange={(value) => setModelGroupForm({ ...modelGroupForm, organization_id: value })} />
                <TextInput label="common.name" value={modelGroupForm.name} onChange={(value) => setModelGroupForm({ ...modelGroupForm, name: value })} />
                <TextInput label="developer.groupKey" value={modelGroupForm.group_key} onChange={(value) => setModelGroupForm({ ...modelGroupForm, group_key: value })} />
                <TextInput label="developer.rateMultiplier" value={modelGroupForm.rate_multiplier} onChange={(value) => setModelGroupForm({ ...modelGroupForm, rate_multiplier: value })} />
                <SubmitButton loading={loading} label="developer.createModelGroup" />
              </form>
            </Panel>
          </div>
          <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_420px]">
            <Panel title="developer.modelAbilities">
              <Table
                headers={['developer.modelGroup', 'developer.channel', 'developer.requestedModel', 'developer.upstreamModel', 'developer.priority']}
                rows={modelChannelAbilities.map((ability) => [
                  ability.model_group_id ? groupLabels[ability.model_group_id] || ability.model_group_id : t('common.none'),
                  channelLabels[ability.channel_id] || ability.channel_id,
                  ability.requested_model || ability.model_pattern || '*',
                  ability.upstream_model || t('common.none'),
                  String(ability.priority),
                ])}
              />
            </Panel>
            <Panel title="developer.createModelAbility">
              <form className="space-y-3" onSubmit={submitModelAbility}>
                <SelectInput label="developer.modelGroup" value={modelAbilityForm.model_group_id} onChange={(value) => setModelAbilityForm({ ...modelAbilityForm, model_group_id: value })} options={['', ...modelGroups.map((group) => group.id)]} labels={{ '': t('common.none'), ...groupLabels }} />
                <SelectInput label="developer.channel" value={modelAbilityForm.channel_id} onChange={(value) => setModelAbilityForm({ ...modelAbilityForm, channel_id: value })} options={channels.map((channel) => channel.id)} labels={channelLabels} />
                <TextInput label="developer.requestedModel" value={modelAbilityForm.requested_model} onChange={(value) => setModelAbilityForm({ ...modelAbilityForm, requested_model: value })} />
                <TextInput label="developer.modelPattern" value={modelAbilityForm.model_pattern} onChange={(value) => setModelAbilityForm({ ...modelAbilityForm, model_pattern: value })} />
                <TextInput label="developer.upstreamModel" value={modelAbilityForm.upstream_model} onChange={(value) => setModelAbilityForm({ ...modelAbilityForm, upstream_model: value })} />
                <TextInput label="developer.priority" value={modelAbilityForm.priority} onChange={(value) => setModelAbilityForm({ ...modelAbilityForm, priority: value })} />
                <SubmitButton loading={loading} label="developer.createModelAbility" />
              </form>
            </Panel>
          </div>
        </div>
      )}

      {effectiveActiveTab === 'routing' && (
        <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_420px]">
          <Panel title="developer.routingRules">
            <Table
              headers={['common.name', 'developer.modelPattern', 'developer.matchScope', 'developer.channel', 'developer.priority']}
              rows={rules.map((rule) => [
                rule.name,
                rule.model_pattern || '*',
                `${rule.match_scope}:${rule.match_value || t('common.none')}`,
                rule.channel_id ? channelLabels[rule.channel_id] || rule.channel_id : providerLabels[rule.provider_id || ''] || t('common.none'),
                String(rule.priority),
              ])}
            />
          </Panel>
          <Panel title="developer.createRoutingRule">
            <form className="space-y-3" onSubmit={submitRoutingRule}>
              <TextInput label="common.name" value={routingForm.name} onChange={(value) => setRoutingForm({ ...routingForm, name: value })} />
              <SelectInput label="developer.provider" value={routingForm.provider_id} onChange={(value) => setRoutingForm({ ...routingForm, provider_id: value })} options={['', ...providers.map((provider) => provider.id)]} labels={{ '': t('common.none'), ...providerLabels }} />
              <SelectInput label="developer.channel" value={routingForm.channel_id} onChange={(value) => setRoutingForm({ ...routingForm, channel_id: value })} options={['', ...channels.map((channel) => channel.id)]} labels={{ '': t('common.none'), ...channelLabels }} />
              <SelectInput label="developer.matchScope" value={routingForm.match_scope} onChange={(value) => setRoutingForm({ ...routingForm, match_scope: value })} options={['global', 'organization', 'department', 'project', 'requirement', 'workflow', 'task', 'agent', 'user', 'source_surface']} labels={scopeLabels(t)} />
              <TextInput label="developer.matchValue" value={routingForm.match_value} onChange={(value) => setRoutingForm({ ...routingForm, match_value: value })} />
              <div className="grid gap-3 sm:grid-cols-2">
                <TextInput label="developer.modelPattern" value={routingForm.model_pattern} onChange={(value) => setRoutingForm({ ...routingForm, model_pattern: value })} />
                <TextInput label="developer.priority" value={routingForm.priority} onChange={(value) => setRoutingForm({ ...routingForm, priority: value })} />
              </div>
              <SubmitButton loading={loading} label="developer.createRoutingRule" />
            </form>
          </Panel>
        </div>
      )}

      {effectiveActiveTab === 'accessTokens' && (
        <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_420px]">
          <Panel title="developer.accessTokens">
            {lastPlainAccessToken && (
              <div className="mb-4 rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800">
                <p className="text-xs font-semibold">{t('developer.oneTimePlainToken')}</p>
                <p className="mt-1 break-all font-mono text-xs">{lastPlainAccessToken}</p>
              </div>
            )}
            <Table
              headers={['common.name', 'developer.organizationId', 'developer.modelGroup', 'developer.allowedModels', 'developer.quota', 'developer.status']}
              rows={accessTokens.map((item) => [
                item.name,
                item.organization_id,
                item.model_group_id ? groupLabels[item.model_group_id] || item.model_group_id : t('common.none'),
                [...(item.allowed_models ?? []), ...(item.allowed_model_patterns ?? [])].join(', ') || '*',
                `${money(item.quota_used, item.quota_currency)} / ${money(item.quota_amount, item.quota_currency)}`,
                t(item.status),
              ])}
            />
          </Panel>
          <Panel title="developer.createAccessToken">
            <form className="space-y-3" onSubmit={submitAccessToken}>
              <TextInput label="developer.organizationId" value={accessTokenForm.organization_id} onChange={(value) => setAccessTokenForm({ ...accessTokenForm, organization_id: value })} />
              <SelectInput label="developer.modelGroup" value={accessTokenForm.model_group_id} onChange={(value) => setAccessTokenForm({ ...accessTokenForm, model_group_id: value })} options={['', ...modelGroups.map((group) => group.id)]} labels={{ '': t('common.none'), ...groupLabels }} />
              <TextInput label="common.name" value={accessTokenForm.name} onChange={(value) => setAccessTokenForm({ ...accessTokenForm, name: value })} />
              <TextInput label="developer.allowedModels" value={accessTokenForm.allowed_models} onChange={(value) => setAccessTokenForm({ ...accessTokenForm, allowed_models: value })} />
              <div className="grid gap-3 sm:grid-cols-2">
                <TextInput label="developer.quotaAmount" value={accessTokenForm.quota_amount} onChange={(value) => setAccessTokenForm({ ...accessTokenForm, quota_amount: value })} />
                <TextInput label="developer.currency" value={accessTokenForm.quota_currency} onChange={(value) => setAccessTokenForm({ ...accessTokenForm, quota_currency: value })} />
              </div>
              <label className="flex items-center gap-2 text-sm font-semibold text-slate-700">
                <input
                  type="checkbox"
                  checked={accessTokenForm.allow_channel_override}
                  onChange={(event) => setAccessTokenForm({ ...accessTokenForm, allow_channel_override: event.target.checked })}
                  className="h-4 w-4 rounded border-slate-300"
                />
                {t('developer.allowChannelOverride')}
              </label>
              <SubmitButton loading={loading} label="developer.createAccessToken" />
            </form>
          </Panel>
        </div>
      )}

      {effectiveActiveTab === 'invocations' && (
        <Panel title="developer.invocationLogs">
          <Table
            headers={['developer.invocation', 'developer.modelRoute', 'developer.channel', 'developer.tokens', 'developer.serviceTier', 'developer.cost']}
            rows={invocations.map((invocation) => [
              invocation.id,
              `${invocation.requested_model || invocation.model_id} -> ${invocation.upstream_model || invocation.model_id}`,
              invocation.channel_id ? channelLabels[invocation.channel_id] || invocation.channel_id : t('developer.providerDefault'),
              `${invocation.input_tokens || 0}/${invocation.output_tokens || 0}`,
              invocation.service_tier || t('common.none'),
              money(invocation.cost_amount, invocation.currency),
            ])}
          />
        </Panel>
      )}

      {effectiveActiveTab === 'balance' && (
        <div className="space-y-5">
          <Panel title="developer.balanceAndCost">
            <div className="grid gap-3 sm:grid-cols-3">
              <Metric label="developer.balanceAmount" value={money(gatewayBalance?.balance_amount, gatewayBalance?.currency)} />
              <Metric label="developer.reservedAmount" value={money(gatewayBalance?.reserved_amount, gatewayBalance?.currency)} />
              <Metric label="developer.availableAmount" value={money((gatewayBalance?.balance_amount ?? 0) - (gatewayBalance?.reserved_amount ?? 0), gatewayBalance?.currency)} />
            </div>
            <form className="mt-5 grid gap-3 md:grid-cols-[minmax(0,1fr)_120px_100px_minmax(0,1fr)_auto]" onSubmit={submitBalanceAdjustment}>
              <TextInput label="developer.organizationId" value={balanceForm.organization_id || selectedOrganizationID} onChange={(value) => setBalanceForm({ ...balanceForm, organization_id: value })} />
              <TextInput label="developer.amount" value={balanceForm.amount} onChange={(value) => setBalanceForm({ ...balanceForm, amount: value })} />
              <TextInput label="developer.currency" value={balanceForm.currency} onChange={(value) => setBalanceForm({ ...balanceForm, currency: value })} />
              <TextInput label="developer.reason" value={balanceForm.reason} onChange={(value) => setBalanceForm({ ...balanceForm, reason: value })} />
              <SubmitButton loading={loading} label="developer.adjustBalance" />
            </form>
          </Panel>
          <Panel title="developer.balanceTransactions">
            <Table
              headers={['developer.transactionType', 'developer.amount', 'developer.accessToken', 'developer.reason', 'developer.createdAt']}
              rows={balanceTransactions.map((item) => [
                t(item.transaction_type),
                money(item.amount, item.currency),
                item.access_token_id || t('common.none'),
                item.reason || t('common.none'),
                item.created_at ? new Date(item.created_at).toLocaleString() : t('common.none'),
              ])}
            />
          </Panel>
        </div>
      )}

      {effectiveActiveTab === 'analysis' && (
        <Panel title="developer.usageAnalysis">
          <div className="grid gap-3 sm:grid-cols-3">
            <Metric label="developer.totalCost" value={money(usageAnalysis?.total_cost ?? cost?.total, usageAnalysis?.currency ?? cost?.currency)} />
            <Metric label="developer.invocationCount" value={String(usageAnalysis?.invocation_count ?? invocations.length)} />
            <Metric label="developer.unexportedCost" value={money(cost?.unexported, cost?.currency)} />
          </div>
          <div className="mt-5 grid gap-4 lg:grid-cols-2">
            <Breakdown title="developer.byProvider" values={usageAnalysis?.by_provider ?? cost?.by_provider ?? {}} currency={usageAnalysis?.currency ?? cost?.currency} />
            <Breakdown title="developer.byChannel" values={usageAnalysis?.by_channel ?? cost?.by_channel ?? {}} currency={usageAnalysis?.currency ?? cost?.currency} />
            <Breakdown title="developer.byModel" values={usageAnalysis?.by_model ?? {}} currency={usageAnalysis?.currency ?? cost?.currency} />
            <Breakdown title="developer.byActor" values={usageAnalysis?.by_actor ?? {}} currency={usageAnalysis?.currency ?? cost?.currency} />
          </div>
        </Panel>
      )}

      {effectiveActiveTab === 'adapters' && (
        <Panel title="developer.adapterMatrix">
          <Table
            headers={['developer.adapter', 'developer.providerType', 'developer.adapterMode', 'developer.baseUrl', 'developer.supportedModes']}
            rows={adapters.map((adapter) => [
              adapter.display_name || adapter.adapter_key,
              t(`provider.${adapter.provider_type}`),
              t(`developer.adapterMode.${adapter.adapter_mode}`),
              adapter.default_base_url || t('common.none'),
              (adapter.supported_modes ?? []).join(', '),
            ])}
          />
        </Panel>
      )}

      {effectiveActiveTab === 'tools' && (
        <Panel title="developer.toolRuntime">
          <Table
            headers={['developer.tool', 'developer.category', 'developer.approvalTier', 'developer.policy', 'developer.risk']}
            rows={tools.map((tool) => [
              tool.name,
              t(tool.tool_category || 'execution_operation'),
              t(tool.approval_tier_required || 'executor'),
              t(tool.default_policy),
              t(tool.risk_level),
            ])}
          />
          <div className="mt-5">
            <p className="text-sm font-semibold text-slate-950">{t('developer.recentExecutions')}</p>
            <Table
              headers={['developer.tool', 'developer.actor', 'developer.status', 'developer.error']}
              rows={executions.map((execution) => [
                execution.tool_name || execution.tool_id,
                `${execution.actor_type}:${execution.actor_id}`,
                t(execution.status),
                execution.error_message || t('common.none'),
              ])}
            />
          </div>
        </Panel>
      )}

      {effectiveActiveTab === 'interfaces' && (
        <div className="space-y-5">
          <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_460px]">
            <Panel title="developer.interfaceFiles">
              <div className="mb-3 flex justify-end">
                <button
                  type="button"
                  onClick={startNewInterfaceFile}
                  className="inline-flex h-9 items-center gap-2 rounded-md border border-slate-300 px-3 text-sm font-semibold text-slate-700 hover:bg-slate-100"
                >
                  <Plus className="h-4 w-4" />
                  {t('developer.newInterfaceFile')}
                </button>
              </div>
              <div className="divide-y divide-slate-100">
                {interfaceFiles.map((file) => (
                  <button
                    key={file.id}
                    type="button"
                    onClick={() => selectInterfaceFile(file)}
                    className={`grid w-full gap-2 py-3 text-left md:grid-cols-[1fr_auto] ${
                      selectedInterfaceFileID === file.id ? 'text-slate-950' : 'text-slate-700'
                    }`}
                  >
                    <div className="min-w-0">
                      <p className="truncate text-sm font-semibold">{file.name}</p>
                      <p className="mt-1 truncate text-xs text-slate-500">{file.updated_at}</p>
                    </div>
                    <StatusBadge label={`developer.fileType.${file.file_type}`} />
                  </button>
                ))}
                {interfaceFiles.length === 0 && <EmptyText>{t('developer.noInterfaceFiles')}</EmptyText>}
              </div>
            </Panel>

            <Panel title="developer.interfaceEditor">
              <form className="space-y-3" onSubmit={submitInterfaceFile}>
                <TextInput label="common.name" value={interfaceForm.name} onChange={(value) => setInterfaceForm({ ...interfaceForm, name: value })} />
                <SelectInput
                  label="developer.fileType"
                  value={interfaceForm.file_type}
                  onChange={(value) => setInterfaceForm({ ...interfaceForm, file_type: value as InterfaceFileType })}
                  options={['json', 'yaml', 'markdown']}
                  labels={interfaceFileTypeLabels(t)}
                />
                <TextAreaInput label="developer.content" value={interfaceForm.content} onChange={(value) => setInterfaceForm({ ...interfaceForm, content: value })} minRows={16} />
                <TextAreaInput label="developer.metadataJson" value={interfaceForm.metadata} onChange={(value) => setInterfaceForm({ ...interfaceForm, metadata: value })} minRows={5} />
                <SubmitButton loading={loading} label={selectedInterfaceFileID ? 'developer.saveInterfaceFile' : 'developer.createInterfaceFile'} />
              </form>
            </Panel>
          </div>

          <Panel title="developer.interfaceTemplates">
            <div className="grid gap-4 lg:grid-cols-2">
              <JsonTemplate title="developer.providerContract" value={providerContract()} onUse={() => applyInterfaceTemplate('developer.providerContract', providerContract())} />
              <JsonTemplate title="developer.channelContract" value={channelContract()} onUse={() => applyInterfaceTemplate('developer.channelContract', channelContract())} />
              <JsonTemplate title="developer.routingContract" value={routingContract()} onUse={() => applyInterfaceTemplate('developer.routingContract', routingContract())} />
              <JsonTemplate title="developer.toolContract" value={toolContract()} onUse={() => applyInterfaceTemplate('developer.toolContract', toolContract())} />
            </div>
          </Panel>
        </div>
      )}
    </div>
  )
}

function Panel({ title, children }: { title: string; children: ReactNode }) {
  const { t } = useI18n()
  return (
    <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
      <h2 className="text-base font-semibold text-slate-950">{t(title)}</h2>
      <div className="mt-4">{children}</div>
    </section>
  )
}

function TextInput({ label, value, onChange, type = 'text' }: { label: string; value: string; onChange: (value: string) => void; type?: string }) {
  const { t } = useI18n()
  return (
    <label className="block">
      <span className="text-xs font-semibold text-slate-500">{t(label)}</span>
      <input
        type={type}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="mt-1 h-10 w-full rounded-lg border border-slate-300 px-3 text-sm outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
      />
    </label>
  )
}

function SelectInput({
  label,
  value,
  onChange,
  options,
  labels = {},
}: {
  label: string
  value: string
  onChange: (value: string) => void
  options: string[]
  labels?: Record<string, string>
}) {
  const { t } = useI18n()
  return (
    <label className="block">
      <span className="text-xs font-semibold text-slate-500">{t(label)}</span>
      <select
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="mt-1 h-10 w-full rounded-lg border border-slate-300 bg-white px-3 text-sm outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
      >
        {options.map((option) => (
          <option key={option || 'none'} value={option}>
            {labels[option] ?? t(`provider.${option}`)}
          </option>
        ))}
      </select>
    </label>
  )
}

function TextAreaInput({
  label,
  value,
  onChange,
  minRows = 4,
}: {
  label: string
  value: string
  onChange: (value: string) => void
  minRows?: number
}) {
  const { t } = useI18n()
  return (
    <label className="block">
      <span className="text-xs font-semibold text-slate-500">{t(label)}</span>
      <textarea
        value={value}
        onChange={(event) => onChange(event.target.value)}
        rows={minRows}
        className="mt-1 w-full resize-y rounded-lg border border-slate-300 px-3 py-2 font-mono text-sm outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
      />
    </label>
  )
}

function SubmitButton({ loading, label }: { loading: boolean; label: string }) {
  const { t } = useI18n()
  return (
    <button
      type="submit"
      disabled={loading}
      className="inline-flex h-10 w-full items-center justify-center rounded-lg bg-slate-950 px-3 text-sm font-semibold text-white hover:bg-slate-800 disabled:opacity-50"
    >
      {t(label)}
    </button>
  )
}

function Table({ headers, rows }: { headers: string[]; rows: string[][] }) {
  const { t } = useI18n()
  return (
    <div className="overflow-x-auto rounded-lg border border-slate-200">
      <table className="min-w-full divide-y divide-slate-200 text-sm">
        <thead className="bg-slate-50">
          <tr>
            {headers.map((header) => (
              <th key={header} className="px-3 py-2 text-left text-xs font-semibold uppercase tracking-normal text-slate-500">
                {t(header)}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-100 bg-white">
          {rows.map((row, index) => (
            <tr key={`${row[0]}-${index}`}>
              {row.map((cell, cellIndex) => (
                <td key={`${cell}-${cellIndex}`} className="max-w-[280px] truncate px-3 py-2 text-slate-700" title={cell}>
                  {cell}
                </td>
              ))}
            </tr>
          ))}
          {rows.length === 0 && (
            <tr>
              <td className="px-3 py-4 text-sm text-slate-500" colSpan={headers.length}>
                {t('common.noData')}
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  )
}

function Breakdown({ title, values, currency = 'CNY' }: { title: string; values: Record<string, number>; currency?: string }) {
  const { t } = useI18n()
  const entries = Object.entries(values)
  return (
    <div className="rounded-lg border border-slate-200 p-4">
      <p className="text-sm font-semibold text-slate-950">{t(title)}</p>
      <div className="mt-3 space-y-2">
        {entries.map(([key, value]) => (
          <div key={key} className="flex items-center justify-between gap-3 text-sm">
            <span className="min-w-0 truncate text-slate-600">{key}</span>
            <span className="shrink-0 font-semibold text-slate-900">{money(value, currency)}</span>
          </div>
        ))}
        {entries.length === 0 && <p className="text-sm text-slate-500">{t('common.noData')}</p>}
      </div>
    </div>
  )
}

function JsonTemplate({ title, value, onUse }: { title: string; value: unknown; onUse?: () => void }) {
  const { t } = useI18n()
  return (
    <div>
      <div className="flex items-center justify-between gap-3">
        <p className="text-sm font-semibold text-slate-950">{t(title)}</p>
        {onUse && (
          <button
            type="button"
            onClick={onUse}
            className="inline-flex h-8 items-center rounded-md border border-slate-300 px-2 text-xs font-semibold text-slate-700 hover:bg-slate-100"
          >
            {t('developer.useTemplate')}
          </button>
        )}
      </div>
      <pre className="mt-2 min-h-[220px] overflow-auto rounded-lg border border-slate-200 bg-slate-950 p-3 text-xs text-slate-50">
        {JSON.stringify(value, null, 2)}
      </pre>
    </div>
  )
}

function Metric({ label, value }: { label: string; value: string }) {
  const { t } = useI18n()
  return (
    <div className="rounded-lg border border-slate-200 bg-slate-50 p-4">
      <p className="text-xs font-semibold text-slate-500">{t(label)}</p>
      <p className="mt-2 truncate text-xl font-semibold text-slate-950" title={value}>
        {value}
      </p>
    </div>
  )
}

function StatusBadge({ label }: { label: string }) {
  const { t } = useI18n()
  return <span className="inline-flex h-7 items-center rounded-md border border-slate-200 bg-slate-50 px-2 text-xs font-semibold text-slate-600">{t(label)}</span>
}

function EmptyText({ children }: { children: ReactNode }) {
  return <p className="py-4 text-sm text-slate-500">{children}</p>
}

function splitCsv(value: string): string[] {
  return value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
}

function parseMapping(value: string): Record<string, string> {
  const trimmed = value.trim()
  if (!trimmed) return {}
  try {
    const parsed = JSON.parse(trimmed)
    return typeof parsed === 'object' && parsed ? (parsed as Record<string, string>) : {}
  } catch {
    return Object.fromEntries(
      trimmed
        .split(/[\n,]/)
        .map((line) => line.trim())
        .filter(Boolean)
        .map((line) => {
          const [from, to] = line.split('=').map((part) => part.trim())
          return [from, to]
        })
        .filter(([from, to]) => from && to),
    )
  }
}

function parseJSONMap(value: string): Record<string, unknown> | null {
  const trimmed = value.trim()
  if (!trimmed) return {}
  try {
    const parsed = JSON.parse(trimmed)
    if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') {
      return null
    }
    return parsed as Record<string, unknown>
  } catch {
    return null
  }
}

function numberOrUndefined(value: string): number | undefined {
  if (value.trim() === '') return undefined
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : undefined
}

function interfaceFileTypeLabels(t: (key: string) => string): Record<string, string> {
  return {
    json: t('developer.fileType.json'),
    yaml: t('developer.fileType.yaml'),
    markdown: t('developer.fileType.markdown'),
  }
}

function scopeLabels(t: (key: string) => string): Record<string, string> {
  return {
    global: t('developer.scope.global'),
    organization: t('developer.scope.organization'),
    department: t('developer.scope.department'),
    project: t('developer.scope.project'),
    requirement: t('developer.scope.requirement'),
    workflow: t('developer.scope.workflow'),
    task: t('developer.scope.task'),
    agent: t('developer.scope.agent'),
    user: t('developer.scope.user'),
    source_surface: t('developer.scope.sourceSurface'),
  }
}

function providerContract() {
  return {
    format_version: 'meta-org.provider.v2',
    provider_type: 'openai | anthropic | gemini',
    auth: { type: 'api_key', encrypted_at_rest: true, provider_default_key: true },
    streaming: { protocol: 'sse', events: ['lifecycle', 'delta', 'usage_update', 'error', 'done'] },
  }
}

function channelContract() {
  return {
    format_version: 'meta-org.channel.v1',
    scheduling: { priority: 'ascending', load_factor: 'weighted', last_used_at: 'least recently used' },
    routing: { supported_model_patterns: ['gpt-*'], model_mapping: { 'logical-model': 'upstream-model' } },
    accounting: { rate_multiplier: '0 allowed', quota_currency: 'CNY' },
  }
}

function routingContract() {
  return {
    format_version: 'meta-org.routing.v1',
    match_scope: 'global | organization | project | agent | user | source_surface',
    model_pattern: '* wildcard supported',
    target: { provider_id: 'optional', channel_id: 'optional' },
  }
}

function toolContract() {
  return {
    format_version: 'meta-org.tool.v1',
    policy: 'auto | notify | approve | deny',
    governance: { required_level: 'L1-L4', risk_level: 'low | medium | high | critical' },
    execution: { idempotency_key: 'required for mutating tools', audit: true },
  }
}
