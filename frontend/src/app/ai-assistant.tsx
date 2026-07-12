'use client'

import { Bot, CheckCircle2, CircleStop, ListChecks, Send, ShieldCheck, XCircle } from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import {
  API_BASE,
  approvePlatformToolApproval,
  approveToolApproval,
  createAssistantSession,
  createPlatformAssistantSession,
  getAssistantContextPackageDiagnostic,
  getAIInvocation,
  getPlatformAssistantContextPackageDiagnostic,
  getPlatformAIInvocation,
  listModelProviders,
  listModels,
  listPlatformModelProviders,
  listPlatformModels,
  rejectPlatformToolApproval,
  rejectToolApproval,
  type AssistantContextPackageDiagnostic,
  type AssistantStep,
  type CostBreakdown,
  type ModelCatalogItem,
  type ModelProvider,
} from '@/lib/api'
import { useI18n } from '@/lib/i18n'
import { streamSSEPost } from '@/lib/stream'

type AssistantState = 'idle' | 'streaming' | 'approval_required' | 'completed' | 'provider_error' | 'governance_denied' | 'cancelled'

interface GatewayStreamData {
  invocation_id?: string
  delta?: string
  error?: string
  done?: boolean
  step?: AssistantStep
  data?: Record<string, unknown>
  estimated_cost_amount?: number
  cost_amount?: number
  currency?: string
  usage?: {
    input_tokens?: number
    output_tokens?: number
    cache_creation_tokens?: number
    cache_read_tokens?: number
    cache_creation_5m_tokens?: number
    cache_creation_1h_tokens?: number
    image_output_tokens?: number
  }
}

interface AssistantUsage {
  input_tokens?: number
  output_tokens?: number
  cache_creation_tokens?: number
  cache_read_tokens?: number
  cache_creation_5m_tokens?: number
  cache_creation_1h_tokens?: number
  image_output_tokens?: number
}

interface AssistantCost {
  estimated?: number
  final?: number
  currency: string
  breakdown?: CostBreakdown
}

interface ChatMessage {
  id: string
  role: 'user' | 'assistant'
  content: string
}

interface AIAssistantProps {
  token: string
  contextType: string
  contextID?: string
  targetType?: string
  targetID?: string
  autoModel?: boolean
  hideModelSelector?: boolean
  apiScope?: 'tenant' | 'platform'
  initialIntent?: string
  initialIntentKey?: string
  autoRunInitialIntent?: boolean
  className?: string
  onSessionCreated?: (sessionID: string) => void
}

const modelPreferenceKey = 'meta_org.assistant.model_by_module.v1'

function assistantMode(contextType: string): 'business_process' | 'self_evolution' {
  return contextType === 'self_evolution' ? 'self_evolution' : 'business_process'
}

function completedState(current: AssistantState): AssistantState {
  if (current === 'provider_error' || current === 'governance_denied' || current === 'cancelled') return current
  return 'completed'
}

function money(value: number | undefined, currency: string, empty: string): string {
  return typeof value === 'number' ? `${currency} ${value.toFixed(4)}` : empty
}

function totalTokens(usage: AssistantUsage): number | undefined {
  if (typeof usage.input_tokens !== 'number' && typeof usage.output_tokens !== 'number') return undefined
  return (usage.input_tokens ?? 0) + (usage.output_tokens ?? 0)
}

function loadModelPreferences(): Record<string, string> {
  if (typeof window === 'undefined') return {}
  try {
    return JSON.parse(window.localStorage.getItem(modelPreferenceKey) || '{}')
  } catch {
    return {}
  }
}

function saveModelPreference(moduleKey: string, modelID: string) {
  if (typeof window === 'undefined' || !modelID) return
  const preferences = loadModelPreferences()
  preferences[moduleKey] = modelID
  window.localStorage.setItem(modelPreferenceKey, JSON.stringify(preferences))
}

export function AIAssistant({
  token,
  contextType,
  contextID,
  targetType,
  targetID,
  autoModel = false,
  hideModelSelector = false,
  apiScope = 'tenant',
  initialIntent,
  initialIntentKey,
  autoRunInitialIntent = false,
  className = '',
  onSessionCreated,
}: AIAssistantProps) {
  const { t } = useI18n()
  const [models, setModels] = useState<ModelCatalogItem[]>([])
  const [providers, setProviders] = useState<ModelProvider[]>([])
  const [selectedModelID, setSelectedModelID] = useState('')
  const [prompt, setPrompt] = useState('')
  const [state, setState] = useState<AssistantState>('idle')
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [invocationID, setInvocationID] = useState('')
  const [sessionID, setSessionID] = useState('')
  const [pendingApprovalID, setPendingApprovalID] = useState('')
  const [steps, setSteps] = useState<AssistantStep[]>([])
  const [usage, setUsage] = useState<AssistantUsage>({})
  const [cost, setCost] = useState<AssistantCost>({ currency: 'CNY' })
  const abortRef = useRef<AbortController | null>(null)
  const consumedIntentRef = useRef('')

  useEffect(() => {
    let cancelled = false
    if (apiScope === 'platform' && autoModel) {
      setModels([])
      setProviders([])
      setSelectedModelID('')
      return () => {
        cancelled = true
      }
    }
    const loadModels = apiScope === 'platform' ? listPlatformModels : listModels
    const loadProviders = apiScope === 'platform' ? listPlatformModelProviders : listModelProviders
    Promise.all([loadModels(token), loadProviders(token)])
      .then(([modelItems, providerItems]) => {
        if (cancelled) return
        const activeProviderIDs = new Set(
          providerItems.filter((provider) => provider.status === 'active').map((provider) => provider.id),
        )
        const activeModels = modelItems.filter(
          (item) => item.status === 'active' && activeProviderIDs.has(item.provider_id),
        )
        const nextModels = activeModels
        const preferences = loadModelPreferences()
        setModels(nextModels)
        setProviders(providerItems)
        setSelectedModelID((current) => {
          const preferredID = current || preferences[contextType] || ''
          if (nextModels.some((model) => model.id === preferredID)) return preferredID
          return nextModels[0]?.id || ''
        })
      })
      .catch(() => {
        if (!cancelled) {
          setModels([])
          setProviders([])
          setSelectedModelID('')
        }
      })
    return () => {
      cancelled = true
    }
  }, [apiScope, autoModel, contextType, token])

  const selectedModel = useMemo(
    () => models.find((model) => model.id === selectedModelID) ?? models[0],
    [models, selectedModelID],
  )
  const providerByID = useMemo(() => Object.fromEntries(providers.map((provider) => [provider.id, provider])), [providers])
  const hasRunnableModel = autoModel || !!selectedModel

  function changeModel(modelID: string) {
    setSelectedModelID(modelID)
    saveModelPreference(contextType, modelID)
  }

  useEffect(() => {
    const key = initialIntentKey || initialIntent || ''
    if (!initialIntent || !key || consumedIntentRef.current === key || state === 'streaming') return
    consumedIntentRef.current = key
    if (autoRunInitialIntent) {
      void send(initialIntent)
      return
    }
    setPrompt(initialIntent)
    // This effect is keyed by initialIntentKey; including send would resubmit when chat state changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [autoRunInitialIntent, initialIntent, initialIntentKey, state])

  async function send(messageOverride?: string) {
    const trimmed = (messageOverride ?? prompt).trim()
    if (!trimmed || state === 'streaming' || !hasRunnableModel) return
    const controller = new AbortController()
    abortRef.current = controller
    setState('streaming')
    setInvocationID('')
    setSessionID('')
    setPendingApprovalID('')
    setSteps([])
    setUsage({})
    setCost({ currency: 'CNY' })
    if (!messageOverride) setPrompt('')

    const userMessage: ChatMessage = { id: `user-${Date.now()}`, role: 'user', content: trimmed }
    const assistantMessage: ChatMessage = { id: `assistant-${Date.now()}`, role: 'assistant', content: '' }
    setMessages((current) => [...current, userMessage, assistantMessage])

    const provider = selectedModel ? providerByID[selectedModel.provider_id] : undefined
    const modelKey = selectedModel?.model_key || 'gpt-4o-mini'
    let currentInvocationID = ''
    let stoppedForApproval = false

    try {
      const session = await (apiScope === 'platform' ? createPlatformAssistantSession : createAssistantSession)(token, {
        title: trimmed.slice(0, 80),
        mode: assistantMode(contextType),
        module_key: contextType,
        provider_id: autoModel ? undefined : selectedModel?.provider_id,
        provider_type: autoModel ? undefined : selectedModel ? undefined : 'openai',
        model: autoModel ? undefined : modelKey,
        target_type: targetType,
        target_id: targetID,
        auto_model: autoModel,
        metadata: {
          source_context: contextType,
          context_id: contextID || '',
          target_type: targetType || '',
          target_id: targetID || '',
          selected_model_id: selectedModel?.id || '',
          selected_model_key: autoModel ? '' : modelKey,
          selected_provider_type: provider?.provider_type || 'openai',
        },
      })
      setSessionID(session.id)
      onSessionCreated?.(session.id)
      const assistantBasePath = apiScope === 'platform' ? '/platform/admin/assistant/sessions' : '/assistant/sessions'
      await streamSSEPost<GatewayStreamData>(
        `${API_BASE}${assistantBasePath}/${session.id}/runs`,
        token,
        {
          message: trimmed,
          provider_id: autoModel ? undefined : selectedModel?.provider_id,
          provider_type: autoModel ? undefined : selectedModel ? undefined : 'openai',
          model: autoModel ? undefined : modelKey,
        },
        ({ data }) => {
          const nextInvocationID = data.invocation_id || data.step?.invocation_id
          if (nextInvocationID) {
            currentInvocationID = nextInvocationID
            setInvocationID(nextInvocationID)
          }
          if (data.step) {
            setSteps((current) => {
              if (current.some((step) => step.id === data.step?.id)) return current
              return [...current, data.step as AssistantStep]
            })
          }
          if (data.delta) {
            setMessages((current) =>
              current.map((message) =>
                message.id === assistantMessage.id ? { ...message, content: message.content + data.delta } : message,
              ),
            )
          }
          if (data.usage) {
            setUsage({
              input_tokens: data.usage.input_tokens,
              output_tokens: data.usage.output_tokens,
              cache_creation_tokens: data.usage.cache_creation_tokens,
              cache_read_tokens: data.usage.cache_read_tokens,
              cache_creation_5m_tokens: data.usage.cache_creation_5m_tokens,
              cache_creation_1h_tokens: data.usage.cache_creation_1h_tokens,
              image_output_tokens: data.usage.image_output_tokens,
            })
          }
          if (typeof data.estimated_cost_amount === 'number' || typeof data.cost_amount === 'number' || data.currency) {
            setCost((current) => ({
              estimated: typeof data.estimated_cost_amount === 'number' ? data.estimated_cost_amount : current.estimated,
              final: typeof data.cost_amount === 'number' ? data.cost_amount : current.final,
              currency: data.currency || current.currency,
            }))
          }
          if (data.step?.status === 'approval_required') setState('approval_required')
          if (data.step?.status === 'approval_required') {
            stoppedForApproval = true
            const approvalID =
              data.step.tool_approval_id ||
              (typeof data.data?.approval_id === 'string' ? data.data.approval_id : '')
            setPendingApprovalID(approvalID)
          }
          if (data.error) {
            const denied = data.error.toLowerCase().includes('denied') || data.error.toLowerCase().includes('forbidden')
            setState(denied ? 'governance_denied' : 'provider_error')
            setMessages((current) =>
              current.map((message) =>
                message.id === assistantMessage.id ? { ...message, content: data.error || t('assistant.error') } : message,
              ),
            )
          }
          if (data.done && !stoppedForApproval) setState(completedState)
        },
        { signal: controller.signal, scope: apiScope },
      )
      if (currentInvocationID) {
        try {
          const loadInvocation = apiScope === 'platform' ? getPlatformAIInvocation : getAIInvocation
          const invocation = await loadInvocation(token, currentInvocationID)
          setUsage({
            input_tokens: invocation.input_tokens,
            output_tokens: invocation.output_tokens,
            cache_creation_tokens: invocation.cache_creation_tokens,
            cache_read_tokens: invocation.cache_read_tokens,
            cache_creation_5m_tokens: invocation.cache_creation_5m_tokens,
            cache_creation_1h_tokens: invocation.cache_creation_1h_tokens,
            image_output_tokens: invocation.image_output_tokens,
          })
          setCost({ final: invocation.cost_amount, currency: invocation.currency, breakdown: invocation.cost_breakdown })
        } catch {
          setCost((current) => current)
        }
      }
      setState(stoppedForApproval ? 'approval_required' : completedState)
    } catch (err) {
      if (controller.signal.aborted) {
        setState('cancelled')
        return
      }
      setState('provider_error')
      setMessages((current) =>
        current.map((message) =>
          message.id === assistantMessage.id
            ? { ...message, content: err instanceof Error ? err.message : t('assistant.error') }
            : message,
        ),
      )
    } finally {
      abortRef.current = null
    }
  }

  async function resumeAfterApproval(approvalID: string) {
    if (!sessionID || !approvalID) return
    const controller = new AbortController()
    abortRef.current = controller
    setState('streaming')
    setPendingApprovalID('')
    const assistantMessage: ChatMessage = { id: `assistant-resume-${Date.now()}`, role: 'assistant', content: '' }
    setMessages((current) => [...current, assistantMessage])
    let currentInvocationID = ''
    let stoppedForApproval = false
    try {
      await streamSSEPost<GatewayStreamData>(
        `${API_BASE}${apiScope === 'platform' ? '/platform/admin/assistant/sessions' : '/assistant/sessions'}/${sessionID}/resume`,
        token,
        { tool_approval_id: approvalID },
        ({ data }) => {
          const nextInvocationID = data.invocation_id || data.step?.invocation_id
          if (nextInvocationID) {
            currentInvocationID = nextInvocationID
            setInvocationID(nextInvocationID)
          }
          if (data.step) {
            setSteps((current) => {
              if (current.some((step) => step.id === data.step?.id)) return current
              return [...current, data.step as AssistantStep]
            })
          }
          if (data.delta) {
            setMessages((current) =>
              current.map((message) =>
                message.id === assistantMessage.id ? { ...message, content: message.content + data.delta } : message,
              ),
            )
          }
          if (data.step?.status === 'approval_required') {
            stoppedForApproval = true
            const nextApprovalID =
              data.step.tool_approval_id ||
              (typeof data.data?.approval_id === 'string' ? data.data.approval_id : '')
            setPendingApprovalID(nextApprovalID)
            setState('approval_required')
          }
          if (data.error) {
            const denied = data.error.toLowerCase().includes('denied') || data.error.toLowerCase().includes('forbidden')
            setState(denied ? 'governance_denied' : 'provider_error')
            setMessages((current) =>
              current.map((message) =>
                message.id === assistantMessage.id ? { ...message, content: data.error || t('assistant.error') } : message,
              ),
            )
          }
          if (data.done && !stoppedForApproval) setState(completedState)
        },
        { signal: controller.signal, scope: apiScope },
      )
      if (currentInvocationID) {
        try {
          const loadInvocation = apiScope === 'platform' ? getPlatformAIInvocation : getAIInvocation
          const invocation = await loadInvocation(token, currentInvocationID)
          setUsage({
            input_tokens: invocation.input_tokens,
            output_tokens: invocation.output_tokens,
            cache_creation_tokens: invocation.cache_creation_tokens,
            cache_read_tokens: invocation.cache_read_tokens,
            cache_creation_5m_tokens: invocation.cache_creation_5m_tokens,
            cache_creation_1h_tokens: invocation.cache_creation_1h_tokens,
            image_output_tokens: invocation.image_output_tokens,
          })
          setCost({ final: invocation.cost_amount, currency: invocation.currency, breakdown: invocation.cost_breakdown })
        } catch {
          setCost((current) => current)
        }
      }
      setState(stoppedForApproval ? 'approval_required' : completedState)
    } catch (err) {
      if (controller.signal.aborted) {
        setState('cancelled')
        return
      }
      setState('provider_error')
      setMessages((current) =>
        current.map((message) =>
          message.id === assistantMessage.id
            ? { ...message, content: err instanceof Error ? err.message : t('assistant.error') }
            : message,
        ),
      )
    } finally {
      abortRef.current = null
    }
  }

  async function approvePendingApproval() {
    if (!pendingApprovalID || state === 'streaming') return
    setState('streaming')
    try {
      const reviewApproval = apiScope === 'platform' ? approvePlatformToolApproval : approveToolApproval
      const result = await reviewApproval(token, pendingApprovalID, 'approved from assistant')
      if (result.execution.status !== 'completed') {
        setState('provider_error')
        setMessages((current) => [
          ...current,
          { id: `assistant-approval-error-${Date.now()}`, role: 'assistant', content: result.execution.error_message || t('assistant.approvalExecutionFailed') },
        ])
        return
      }
      await resumeAfterApproval(result.approval.id)
    } catch (err) {
      setState('provider_error')
      setMessages((current) => [
        ...current,
        { id: `assistant-approval-error-${Date.now()}`, role: 'assistant', content: err instanceof Error ? err.message : t('assistant.approvalExecutionFailed') },
      ])
    }
  }

  async function rejectPendingApproval() {
    if (!pendingApprovalID || state === 'streaming') return
    setState('streaming')
    try {
      const reviewRejection = apiScope === 'platform' ? rejectPlatformToolApproval : rejectToolApproval
      await reviewRejection(token, pendingApprovalID, 'rejected from assistant')
      setPendingApprovalID('')
      setState('cancelled')
      setMessages((current) => [
        ...current,
        { id: `assistant-approval-rejected-${Date.now()}`, role: 'assistant', content: t('assistant.approvalRejected') },
      ])
    } catch (err) {
      setState('provider_error')
      setMessages((current) => [
        ...current,
        { id: `assistant-approval-error-${Date.now()}`, role: 'assistant', content: err instanceof Error ? err.message : t('assistant.error') },
      ])
    }
  }

  function cancel() {
    abortRef.current?.abort()
    setState('cancelled')
  }

  const tokenTotal = totalTokens(usage)

  return (
    <aside className={`flex h-full min-h-0 flex-col bg-white ${className}`}>
      <div className="flex items-center justify-between gap-3 border-b border-slate-200 px-5 py-4">
        <div className="flex min-w-0 items-center gap-2">
          <Bot className="h-5 w-5 shrink-0 text-[#AD4714]" />
          <div className="min-w-0">
            <h2 className="truncate text-base font-semibold text-slate-950">{t('assistant.title')}</h2>
            <p className="truncate text-xs text-slate-500">{contextType}</p>
          </div>
        </div>
        <div className="flex min-w-0 items-center gap-2">
          {!hideModelSelector && (
            <select
              value={selectedModelID}
              onChange={(event) => changeModel(event.target.value)}
              className="h-9 max-w-[220px] rounded-lg border border-slate-300 bg-white px-2 text-xs font-semibold text-slate-700 outline-none focus:border-[#AD4714] focus:ring-2 focus:ring-[#DF6A24]/20"
              aria-label={t('assistant.model')}
            >
              {models.length === 0 ? (
                <option value="">{t('assistant.defaultModel')}</option>
              ) : (
                models.map((model) => (
                  <option key={model.id} value={model.id}>
                    {model.display_name || model.model_key}
                  </option>
                ))
              )}
            </select>
          )}
          <StateBadge state={state} />
        </div>
      </div>

      <div className="flex-1 space-y-3 overflow-y-auto bg-slate-50 px-5 py-4">
        {messages.length === 0 ? (
          <div className="rounded-lg border border-dashed border-slate-300 bg-white p-4 text-sm text-slate-500">
            {t('assistant.empty')}
          </div>
        ) : (
          messages.map((message) => (
            <div
              key={message.id}
              className={`max-w-[88%] rounded-lg px-3 py-2 text-sm leading-6 ${
                message.role === 'user'
                  ? 'ml-auto bg-[#AD4714] text-[#fffaf5]'
                  : 'mr-auto border border-slate-200 bg-white text-slate-800'
              }`}
            >
              {message.content || (message.role === 'assistant' && state === 'streaming' ? t('assistant.thinking') : '')}
            </div>
          ))
        )}
      </div>

      <div className="border-t border-slate-200 bg-white px-5 py-3">
        <div className="mb-3 flex flex-wrap items-center gap-2 text-xs text-slate-500">
          <span>{t('assistant.tokens')}: {tokenTotal ?? t('common.none')}</span>
          <span>{t('assistant.finalCost')}: {money(cost.final ?? cost.estimated, cost.currency, t('common.none'))}</span>
          {invocationID && <span className="max-w-[220px] truncate">{t('assistant.invocation')}: {invocationID}</span>}
        </div>
        <div className="flex items-end gap-2">
          <textarea
            value={prompt}
            onChange={(event) => setPrompt(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) {
                event.preventDefault()
                void send()
              }
            }}
            placeholder={t('assistant.chatPlaceholder')}
            className="min-h-[72px] flex-1 resize-none rounded-lg border border-slate-300 p-3 text-sm text-slate-900 outline-none transition focus:border-[#AD4714] focus:ring-2 focus:ring-[#DF6A24]/20"
          />
          {state === 'streaming' ? (
            <button
              type="button"
              onClick={cancel}
              className="inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border border-slate-300 text-slate-700 transition hover:bg-slate-100"
              aria-label={t('assistant.cancel')}
            >
              <CircleStop className="h-4 w-4" />
            </button>
          ) : (
            <button
              type="button"
              onClick={() => void send()}
              disabled={!prompt.trim() || !hasRunnableModel}
              className="inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-[#AD4714] text-[#fffaf5] transition hover:bg-[#B84F18] disabled:cursor-not-allowed disabled:opacity-50"
              aria-label={t('assistant.send')}
            >
              <Send className="h-4 w-4" />
            </button>
          )}
        </div>
        {state === 'approval_required' && pendingApprovalID && (
          <div className="mt-3 flex gap-2">
            <button
              type="button"
              onClick={() => void approvePendingApproval()}
              className="inline-flex h-9 flex-1 items-center justify-center gap-2 rounded-lg bg-emerald-600 px-3 text-sm font-semibold text-white transition hover:bg-emerald-700"
            >
              <CheckCircle2 className="h-4 w-4" />
              {t('assistant.approveAndContinue')}
            </button>
            <button
              type="button"
              onClick={() => void rejectPendingApproval()}
              className="inline-flex h-9 flex-1 items-center justify-center gap-2 rounded-lg border border-slate-300 px-3 text-sm font-semibold text-slate-700 transition hover:bg-slate-100"
            >
              <XCircle className="h-4 w-4" />
              {t('assistant.rejectApproval')}
            </button>
          </div>
        )}
        {steps.length > 0 && (
          <div className="mt-3 rounded-lg border border-slate-200 bg-slate-50 p-3">
            <div className="mb-2 flex items-center gap-2 text-xs font-semibold text-slate-600">
              <ListChecks className="h-4 w-4" />
              {t('assistant.executionTimeline')}
            </div>
            <div className="space-y-2">
              {steps.map((step) => (
                <StepCard key={step.id} step={step} token={token} apiScope={apiScope} />
              ))}
            </div>
          </div>
        )}
      </div>
    </aside>
  )
}

function StepCard({ step, token, apiScope }: { step: AssistantStep; token: string; apiScope: 'tenant' | 'platform' }) {
  const { t } = useI18n()
  const [expanded, setExpanded] = useState(false)
  const [diagnostic, setDiagnostic] = useState<AssistantContextPackageDiagnostic | null>(null)
  const [loadingDiagnostic, setLoadingDiagnostic] = useState(false)
  const [diagnosticError, setDiagnosticError] = useState('')
  const tone =
    step.status === 'completed'
      ? 'border-emerald-200 bg-emerald-50 text-emerald-800'
      : step.status === 'approval_required'
        ? 'border-amber-200 bg-amber-50 text-amber-800'
        : step.status === 'failed'
          ? 'border-red-200 bg-red-50 text-red-800'
          : 'border-slate-200 bg-white text-slate-700'
  const toolName = typeof step.data?.tool_name === 'string' ? step.data.tool_name : ''
  const contextPackageID = stringFromStepData(step.data, 'context_package_id')
  const previousContextPackageID = stringFromStepData(step.data, 'previous_context_package_id')
  const refreshedContextPackageID = stringFromStepData(step.data, 'refreshed_context_package_id')
  const riskSignalCount = numberFromStepData(step.data, 'risk_signal_count')
  const omissionCount = numberFromStepData(step.data, 'omission_count')
  const supportingContextCount = numberFromStepData(step.data, 'supporting_context_count')
  const hasContext = !!contextPackageID || !!previousContextPackageID || !!refreshedContextPackageID

  async function toggleDiagnostic() {
    const nextExpanded = !expanded
    setExpanded(nextExpanded)
    if (!nextExpanded || !contextPackageID || diagnostic || loadingDiagnostic) return
    setLoadingDiagnostic(true)
    setDiagnosticError('')
    try {
      const loader = apiScope === 'platform' ? getPlatformAssistantContextPackageDiagnostic : getAssistantContextPackageDiagnostic
      setDiagnostic(await loader(token, contextPackageID))
    } catch (err) {
      setDiagnosticError(err instanceof Error ? err.message : t('assistant.contextLoadFailed'))
    } finally {
      setLoadingDiagnostic(false)
    }
  }

  return (
    <div className={`rounded-lg border px-3 py-2 text-xs ${tone}`}>
      <div className="flex items-center justify-between gap-2">
        <span className="font-semibold">{t(`assistant.step.${step.step_type}`)}</span>
        <span className="shrink-0">{t(`assistant.state.${step.status}`)}</span>
      </div>
      <p className="mt-1 line-clamp-2 text-[11px] opacity-80">{step.summary || toolName || t('common.none')}</p>
      {(step.tool_execution_id || step.tool_approval_id) && (
        <p className="mt-1 truncate text-[10px] opacity-70">
          {step.tool_execution_id || step.tool_approval_id}
        </p>
      )}
      {hasContext && (
        <div className="mt-2 flex flex-wrap gap-1.5 text-[10px]">
          {contextPackageID && <ContextBadge label={t('assistant.contextPackage')} value={shortID(contextPackageID)} />}
          {typeof riskSignalCount === 'number' && <ContextBadge label={t('assistant.riskSignals')} value={String(riskSignalCount)} />}
          {typeof omissionCount === 'number' && <ContextBadge label={t('assistant.omissions')} value={String(omissionCount)} />}
          {typeof supportingContextCount === 'number' && <ContextBadge label={t('assistant.supportingContext')} value={String(supportingContextCount)} />}
          {previousContextPackageID && <ContextBadge label={t('assistant.previousContext')} value={shortID(previousContextPackageID)} />}
          {refreshedContextPackageID && <ContextBadge label={t('assistant.refreshedContext')} value={shortID(refreshedContextPackageID)} />}
        </div>
      )}
      {contextPackageID && (
        <button
          type="button"
          onClick={() => void toggleDiagnostic()}
          className="mt-2 inline-flex h-7 items-center gap-1 rounded-md border border-current px-2 text-[11px] font-semibold opacity-80 transition hover:opacity-100"
        >
          <ShieldCheck className="h-3.5 w-3.5" />
          {expanded ? t('assistant.hideContextDetails') : t('assistant.contextDetails')}
        </button>
      )}
      {expanded && (
        <div className="mt-2 rounded-md border border-current/20 bg-white/65 p-2 text-[11px] text-slate-700">
          {loadingDiagnostic && <p>{t('assistant.loadingContextDetails')}</p>}
          {diagnosticError && <p className="text-red-700">{diagnosticError}</p>}
          {diagnostic && (
            <div className="space-y-2">
              <div className="grid grid-cols-2 gap-1">
                <ContextMetric label={t('assistant.attentionCore')} value={String(diagnostic.summary.attention_core_count)} />
                <ContextMetric label={t('assistant.supportingContext')} value={String(diagnostic.summary.supporting_context_count)} />
                <ContextMetric label={t('assistant.riskSignals')} value={String(diagnostic.summary.risk_signal_count)} />
                <ContextMetric label={t('assistant.omissions')} value={String(diagnostic.summary.omission_count)} />
                <ContextMetric label={t('assistant.tokens')} value={String(diagnostic.summary.estimated_tokens)} />
                <ContextMetric label={t('assistant.source')} value={diagnostic.summary.source || t('common.none')} />
              </div>
              {diagnostic.omissions.length > 0 && (
                <div className="space-y-1">
                  {diagnostic.omissions.slice(0, 3).map((item) => (
                    <p key={`${item.entity_key}.${item.field_key}.${item.reason}`} className="truncate text-[10px] text-amber-700">
                      {item.entity_key}.{item.field_key}: {item.reason}
                    </p>
                  ))}
                </div>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  )
}

function ContextBadge({ label, value }: { label: string; value: string }) {
  return (
    <span className="inline-flex max-w-full items-center gap-1 rounded-md border border-current/25 bg-white/60 px-1.5 py-0.5">
      <span className="shrink-0 opacity-70">{label}</span>
      <span className="truncate font-semibold">{value}</span>
    </span>
  )
}

function ContextMetric({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0 rounded border border-slate-200 bg-white px-2 py-1">
      <p className="truncate text-[10px] font-semibold text-slate-500">{label}</p>
      <p className="truncate font-semibold text-slate-900">{value}</p>
    </div>
  )
}

function stringFromStepData(data: Record<string, unknown> | undefined, key: string): string {
  const value = data?.[key]
  return typeof value === 'string' ? value : ''
}

function numberFromStepData(data: Record<string, unknown> | undefined, key: string): number | undefined {
  const value = data?.[key]
  if (typeof value === 'number') return value
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value)
    return Number.isFinite(parsed) ? parsed : undefined
  }
  return undefined
}

function shortID(value: string): string {
  return value.length > 12 ? `${value.slice(0, 8)}...` : value
}

function StateBadge({ state }: { state: AssistantState }) {
  const { t } = useI18n()
  const tone =
    state === 'completed'
      ? 'border-emerald-200 bg-emerald-50 text-emerald-700'
      : state === 'provider_error' || state === 'governance_denied'
        ? 'border-red-200 bg-red-50 text-red-700'
        : state === 'approval_required'
          ? 'border-amber-200 bg-amber-50 text-amber-700'
          : 'border-slate-200 bg-slate-50 text-slate-600'
  return <span className={`inline-flex h-7 max-w-[150px] items-center truncate rounded-md border px-2 text-xs font-semibold ${tone}`}>{t(`assistant.state.${state}`)}</span>
}
