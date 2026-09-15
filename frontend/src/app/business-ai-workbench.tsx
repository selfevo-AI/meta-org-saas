'use client'

import { Bot, CheckCircle2, Loader2, Send, XCircle } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'

import { apiRequest } from '@/lib/api'
import { useI18n } from '@/lib/i18n'
import { Dialog, FeedbackMessage } from './workspace-ui'

type BusinessAIStage = 'plan' | 'do' | 'change' | 'accept' | 'learn'

interface BusinessAIAnalysis {
  summary: string
  findings: Array<{ title: string; evidence: string; impact: string }>
  recommendations: Array<{ title: string; rationale: string; priority: string }>
  risks: Array<{ title: string; probability: string; impact: string; mitigation: string }>
  proposal: { action: string; tool_name: string; arguments: Record<string, unknown>; requires_approval: boolean }
  confidence: number
  evidence_refs: string[]
}

interface BusinessAIRun {
  id: string
  stage: BusinessAIStage
  status: 'running' | 'completed' | 'failed'
  invocation_id?: string
  resolved_model: string
  analysis?: BusinessAIAnalysis
  cost_amount: number
  currency: string
  input_tokens: number
  output_tokens: number
  error_message?: string
  proposal_status: 'not_submitted' | 'submitting' | 'approval_required' | 'completed' | 'rejected' | 'failed' | 'denied'
  tool_execution_id?: string
  tool_approval_id?: string
  proposal_result: Record<string, unknown>
  proposal_error?: string
}

interface ModelProvider {
  id: string
  provider_type: string
  name: string
  status: string
}

interface AIModel {
  provider_id: string
  model_key: string
  display_name: string
  status: string
}

interface ProjectOption {
  id: string
  master_key?: string
  name: string
}

export function BusinessAIWorkbench({ token, projectID }: { token: string; projectID: string }) {
  const { t } = useI18n()
  const [stage, setStage] = useState<BusinessAIStage>('plan')
  const [focus, setFocus] = useState('')
  const [runs, setRuns] = useState<BusinessAIRun[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [reviewAction, setReviewAction] = useState<{ kind: 'submit' | 'approve' | 'reject'; run: BusinessAIRun; projectID: string; projectName: string } | null>(null)
  const [reviewReason, setReviewReason] = useState('')
  const [needsAnalysis, setNeedsAnalysis] = useState(false)
  const reviewLock = useRef(false)
  const focusInput = useRef<HTMLTextAreaElement>(null)
  const [providers, setProviders] = useState<ModelProvider[]>([])
  const [models, setModels] = useState<AIModel[]>([])
  const [modelSelection, setModelSelection] = useState('')
  const [projects, setProjects] = useState<ProjectOption[]>([])
  const [activeProjectID, setActiveProjectID] = useState(projectID)
  const stages: BusinessAIStage[] = ['plan', 'do', 'change', 'accept', 'learn']
  const latest = runs[0]
  const selectedModel = models.find((item) => `${item.provider_id}:${item.model_key}` === modelSelection)
  const providerType = providers.find((item) => item.id === selectedModel?.provider_id)?.provider_type || ''

  useEffect(() => {
    let cancelled = false
    Promise.allSettled([
      apiRequest<ModelProvider[]>('/model-providers', { token }),
      apiRequest<AIModel[]>('/models', { token }),
      apiRequest<ProjectOption[]>('/projects?limit=100', { token }),
    ])
      .then(([providerResult, modelResult, projectResult]) => {
        if (cancelled) return
        const providerData = providerResult.status === 'fulfilled' ? providerResult.value : []
        const modelData = modelResult.status === 'fulfilled' ? modelResult.value : []
        const projectData = projectResult.status === 'fulfilled' ? projectResult.value : []
        const nextProviders = Array.isArray(providerData) ? providerData.filter((item) => item.status === 'active') : []
        const activeProviderIDs = new Set(nextProviders.map((item) => item.id))
        const nextModels = Array.isArray(modelData)
          ? modelData.filter((item) => item.status === 'active' && activeProviderIDs.has(item.provider_id))
          : []
        const nextProjects = Array.isArray(projectData) ? projectData : []
        setProviders(nextProviders)
        setModels(nextModels)
        setProjects(nextProjects)
        setModelSelection((current) => current || (nextModels[0] ? `${nextModels[0].provider_id}:${nextModels[0].model_key}` : ''))
        setActiveProjectID((current) => {
          const matched = nextProjects.find((item) => item.id === projectID || item.master_key === projectID || item.id === current)
          return matched?.id || nextProjects[0]?.id || current || projectID
        })
        const failedResult = [providerResult, modelResult, projectResult].find((result) => result.status === 'rejected')
        if (failedResult?.status === 'rejected') {
          setError(failedResult.reason instanceof Error ? failedResult.reason.message : t('common.operationFailed'))
        }
      })
    return () => {
      cancelled = true
    }
  }, [projectID, t, token])

  useEffect(() => {
    if (projectID && projects.length > 0) {
      const matched = projects.find((item) => item.id === projectID || item.master_key === projectID)
      if (matched) Promise.resolve().then(() => setActiveProjectID(matched.id))
    }
  }, [projectID, projects])

  useEffect(() => {
    if (!activeProjectID) {
      Promise.resolve().then(() => setRuns([]))
      return
    }
    let cancelled = false
    apiRequest<BusinessAIRun[]>(`/projects/${encodeURIComponent(activeProjectID)}/ai-analyses?limit=30`, { token })
      .then((data) => {
        if (!cancelled) setRuns(Array.isArray(data) ? data : [])
      })
      .catch((requestError) => {
        if (!cancelled) setError(requestError instanceof Error ? requestError.message : t('common.operationFailed'))
      })
    return () => {
      cancelled = true
    }
  }, [activeProjectID, t, token])

  async function analyze() {
    if (!activeProjectID) return
    setLoading(true)
    setError('')
    try {
      const completedRun = await apiRequest<BusinessAIRun>(`/projects/${encodeURIComponent(activeProjectID)}/ai-analyses`, {
        method: 'POST',
        token,
        body: {
          stage,
          focus,
          provider_type: providerType,
          model: selectedModel?.model_key || '',
          context: { source_ui: 'erp_project_workbench' },
        },
      })
      setRuns((current) => [completedRun, ...current.filter((item) => item.id !== completedRun.id)])
      const data = await apiRequest<BusinessAIRun[]>(`/projects/${encodeURIComponent(activeProjectID)}/ai-analyses?limit=30`, { token })
      setRuns(Array.isArray(data) ? data : [])
      setNeedsAnalysis(false)
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : t('common.operationFailed'))
    } finally {
      setLoading(false)
    }
  }

  async function refreshRuns() {
    if (!activeProjectID) return
    const data = await apiRequest<BusinessAIRun[]>(`/projects/${encodeURIComponent(activeProjectID)}/ai-analyses?limit=30`, { token })
    setRuns(Array.isArray(data) ? data : [])
  }

  async function submitProposal() {
    if (!reviewAction || reviewLock.current) return
    reviewLock.current = true
    setLoading(true)
    setError('')
    try {
      const run = await apiRequest<BusinessAIRun>(`/projects/${encodeURIComponent(reviewAction.projectID)}/ai-analyses/${reviewAction.run.id}/submit-proposal`, {
        method: 'POST', token, body: {},
      })
      setRuns((current) => [run, ...current.filter((item) => item.id !== run.id)])
      setReviewAction(null)
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : t('common.operationFailed'))
    } finally {
      reviewLock.current = false
      setLoading(false)
    }
  }

  async function reviewProposal(decision: 'approve' | 'reject') {
    if (!reviewAction?.run.tool_approval_id || reviewLock.current) return
    reviewLock.current = true
    setLoading(true)
    setError('')
    try {
      await apiRequest(`/tool-approvals/${reviewAction.run.tool_approval_id}/${decision}`, {
        method: 'POST', token, body: { reason: reviewReason.trim() || t(decision === 'approve' ? 'ui.home.approve' : 'ui.home.reject') },
      })
      setReviewAction(null)
      await refreshRuns()
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : t('common.operationFailed'))
    } finally {
      reviewLock.current = false
      setLoading(false)
    }
  }

  return (
    <section data-testid="business-ai-workbench" className="ui-card px-5 py-5">
      <div className="flex items-center gap-2">
        <Bot className="h-5 w-5 text-[#AD4714]" />
        <h2 className="text-base font-semibold text-slate-950">{t('businessAI.title')}</h2>
      </div>
      <p className="mt-2 text-xs ui-muted">{t('ui.assistant.subtitle')}</p>
      <div className="mt-4 grid grid-cols-5 overflow-hidden rounded-md border border-slate-300" role="group" aria-label={t('businessAI.stage')}>
        {stages.map((item) => (
          <button
            key={item}
            type="button"
            data-testid={`business-ai-stage-${item}`}
            onClick={() => { setStage(item); if (latest) setNeedsAnalysis(true) }}
            disabled={loading}
            aria-pressed={stage === item}
            className={`min-h-10 border-r border-slate-300 px-1 text-xs font-semibold last:border-r-0 sm:px-2 sm:text-sm ${stage === item ? 'bg-slate-950 text-white' : 'bg-white text-slate-700 hover:bg-slate-100'}`}
          >
            {t(`businessAI.stage.${item}`)}
          </button>
        ))}
      </div>
      <label className="mt-4 block max-w-xl">
        <span className="text-sm font-medium text-slate-700">{t('businessAI.project')}</span>
        <select data-testid="business-ai-project" value={activeProjectID} disabled={loading} onChange={(event) => { setRuns([]); setNeedsAnalysis(false); setActiveProjectID(event.target.value) }} className="ui-select mt-1">
          {projects.length === 0 && <option value="">{t('businessAI.noProject')}</option>}
          {projects.map((item) => {
            return <option key={item.id} value={item.id}>{item.name} · {item.master_key || item.id}</option>
          })}
        </select>
      </label>
      <label className="mt-4 block">
        <span className="text-sm font-medium text-slate-700">{t('businessAI.focus')}</span>
        <textarea ref={focusInput} value={focus} onChange={(event) => { setFocus(event.target.value); if (latest) setNeedsAnalysis(true) }} disabled={loading} className="ui-input mt-1 min-h-24 resize-y" />
      </label>
      <label className="mt-3 block max-w-md">
        <span className="text-sm font-medium text-slate-700">{t('businessAI.model')}</span>
        <select value={modelSelection} disabled={loading} onChange={(event) => { setModelSelection(event.target.value); if (latest) setNeedsAnalysis(true) }} className="ui-select mt-1">
          {models.length === 0 && <option value="">{t('businessAI.noModel')}</option>}
          {models.map((item) => <option key={`${item.provider_id}:${item.model_key}`} value={`${item.provider_id}:${item.model_key}`}>{item.display_name || item.model_key}</option>)}
        </select>
      </label>
      <button data-testid="business-ai-analyze" type="button" onClick={() => void analyze()} disabled={loading || !activeProjectID || !selectedModel || !providerType} className="mt-3 inline-flex h-10 items-center gap-2 rounded-md bg-slate-950 px-3 text-sm font-semibold text-white hover:bg-slate-800 disabled:opacity-50">
        {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : <Bot className="h-4 w-4" />}
        {t('businessAI.analyze')}
      </button>
      {needsAnalysis && <p className="mt-3 text-sm ui-muted">{t('ui.review.reanalyzeHint')}</p>}

      {error && !reviewAction && <div className="mt-4"><FeedbackMessage error>{error}</FeedbackMessage></div>}
      {latest?.analysis ? (
        <div className="mt-5 space-y-4 border-t border-slate-200 pt-4">
          <div className="flex flex-wrap items-start justify-between gap-2">
            <div>
              <p className="font-semibold text-slate-950">{latest.analysis.summary}</p>
              <p className="mt-1 text-xs text-slate-500">{t(`businessAI.stage.${latest.stage}`)} · {latest.resolved_model} · {Math.round(latest.analysis.confidence * 100)}%</p>
            </div>
            <span className="rounded-md bg-emerald-50 px-2 py-1 text-xs font-semibold text-emerald-700">{t('ui.status.' + latest.status)}</span>
          </div>
          <div className="grid gap-4 lg:grid-cols-3">
            <ResultList title={t('businessAI.findings')} items={latest.analysis.findings.map((item) => ({ title: item.title, detail: `${item.evidence} · ${item.impact}` }))} />
            <ResultList title={t('businessAI.recommendations')} items={latest.analysis.recommendations.map((item) => ({ title: item.title, detail: `${item.priority} · ${item.rationale}` }))} />
            <ResultList title={t('businessAI.risks')} items={latest.analysis.risks.map((item) => ({ title: item.title, detail: `${item.probability} · ${item.mitigation}` }))} />
          </div>
          {latest.analysis.proposal.action && (
            <div className="border-l-4 border-amber-400 bg-amber-50 px-4 py-3">
              <p className="text-sm font-semibold text-amber-950">{t('businessAI.proposal')}: {latest.analysis.proposal.action}</p>
              <p className="mt-1 break-all text-xs text-amber-800">{latest.analysis.proposal.tool_name || t('businessAI.noTool')} · {latest.analysis.proposal.requires_approval ? t('businessAI.approvalRequired') : t('businessAI.advisory')}</p>
            </div>
          )}
          <div className="flex flex-wrap items-center gap-2">
            {latest.proposal_status === 'not_submitted' && latest.analysis.proposal.tool_name && (
              <button data-testid="business-ai-submit-proposal" type="button" onClick={() => { setError(''); setReviewReason(''); setReviewAction({ kind: 'submit', run: latest, projectID: activeProjectID, projectName: projects.find((item) => item.id === activeProjectID)?.name || activeProjectID }) }} disabled={loading || needsAnalysis} className="ui-button ui-button-primary">
                <Send className="h-4 w-4" />{t('businessAI.submitProposal')}
              </button>
            )}
            {latest.proposal_status === 'approval_required' && latest.tool_approval_id && (
              <>
                <button data-testid="business-ai-approve-proposal" type="button" onClick={() => { setError(''); setReviewReason(''); setReviewAction({ kind: 'approve', run: latest, projectID: activeProjectID, projectName: projects.find((item) => item.id === activeProjectID)?.name || activeProjectID }) }} disabled={loading} className="ui-button ui-button-primary">
                  <CheckCircle2 className="h-4 w-4" />{t('businessAI.approveProposal')}
                </button>
                <button type="button" onClick={() => { setError(''); setReviewReason(''); setReviewAction({ kind: 'reject', run: latest, projectID: activeProjectID, projectName: projects.find((item) => item.id === activeProjectID)?.name || activeProjectID }) }} disabled={loading} className="ui-button ui-button-secondary">
                  <XCircle className="h-4 w-4" />{t('businessAI.rejectProposal')}
                </button>
              </>
            )}
            <span data-testid="business-ai-proposal-status" className="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-700">{t(`businessAI.proposalStatus.${latest.proposal_status || 'not_submitted'}`)}</span>
          </div>
          {latest.proposal_error && <p className="border-l-4 border-red-400 bg-red-50 px-4 py-3 text-sm text-red-700">{latest.proposal_error}</p>}
          {latest.proposal_status === 'completed' && (
            <details className="text-xs ui-muted"><summary className="cursor-pointer">{t('ui.document.viewDetails')}</summary><pre className="mt-2 max-h-48 overflow-auto rounded-md bg-slate-50 p-3">{JSON.stringify(latest.proposal_result, null, 2)}</pre></details>
          )}
          <p className="break-all text-xs text-slate-500">{t('businessAI.audit')}: {latest.invocation_id} · {latest.input_tokens + latest.output_tokens} tokens · {latest.cost_amount.toFixed(6)} {latest.currency}</p>
        </div>
      ) : !error ? (
        <p className="mt-4 border border-dashed border-slate-300 px-4 py-5 text-sm text-slate-500">{t('businessAI.empty')}</p>
      ) : null}
      <Dialog open={!!reviewAction} title={t('ui.review.title')} description={t('ui.home.reviewHint')} busy={loading} onClose={() => { setReviewAction(null); setError('') }}
        footer={<>
          {reviewAction?.kind === 'submit' && <button type="button" className="ui-button ui-button-ghost mr-auto" disabled={loading} onClick={() => { setNeedsAnalysis(true); setReviewAction(null); requestAnimationFrame(() => focusInput.current?.focus()) }}>{t('ui.review.adjust')}</button>}
          <button type="button" className="ui-button ui-button-secondary" data-dialog-cancel disabled={loading} onClick={() => { setReviewAction(null); setError('') }}>{t('common.cancel')}</button>
          <button type="button" className="ui-button ui-button-primary" disabled={loading} onClick={() => reviewAction?.kind === 'submit' ? void submitProposal() : reviewAction && void reviewProposal(reviewAction.kind)}><CheckCircle2 size={16} />{t(loading ? 'ui.document.processing' : reviewAction?.kind === 'reject' ? 'ui.home.reject' : 'ontology.confirm')}</button>
        </>}>
        {reviewAction && <>
          <div className="ui-dialog-context"><Bot size={22} /><div><strong>{reviewAction.projectName}</strong><small>{t('businessAI.stage.' + reviewAction.run.stage)}</small></div></div>
          <p className="document-action-impact">{reviewAction.run.analysis?.proposal.action}</p>
          <dl className="document-action-summary">{Object.entries(reviewAction.run.analysis?.proposal.arguments ?? {}).map(([name, value]) => <div key={name}><dt>{t('ui.review.parameter', { name })}</dt><dd>{typeof value === 'object' ? JSON.stringify(value) : String(value)}</dd></div>)}</dl>
          {reviewAction.kind !== 'submit' && <label className="ui-field"><span className="ui-field-label">{t('ui.home.reviewReason')}</span><textarea className="ui-input min-h-24" value={reviewReason} onChange={(event) => setReviewReason(event.target.value)} placeholder={t('ui.home.reviewReasonPlaceholder')} disabled={loading} /></label>}
          {error && <FeedbackMessage error>{error}</FeedbackMessage>}
        </>}
      </Dialog>
    </section>
  )
}

function ResultList({ title, items }: { title: string; items: Array<{ title: string; detail: string }> }) {
  return (
    <div>
      <h3 className="text-sm font-semibold text-slate-900">{title}</h3>
      <div className="mt-2 space-y-2">
        {items.map((item, index) => (
          <div key={`${item.title}-${index}`} className="border-l-2 border-slate-300 pl-3">
            <p className="text-sm font-medium text-slate-800">{item.title}</p>
            <p className="mt-0.5 text-xs text-slate-500">{item.detail}</p>
          </div>
        ))}
      </div>
    </div>
  )
}
