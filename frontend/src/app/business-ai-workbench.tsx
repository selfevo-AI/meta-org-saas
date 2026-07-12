'use client'

import { Bot, Loader2 } from 'lucide-react'
import { useEffect, useState } from 'react'

import { apiRequest } from '@/lib/api'
import { useI18n } from '@/lib/i18n'

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
}

interface ModelProvider {
  id: string
  provider_type: string
  name: string
}

interface AIModel {
  provider_id: string
  model_key: string
  display_name: string
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
    Promise.all([
      apiRequest<ModelProvider[]>('/model-providers', { token }),
      apiRequest<AIModel[]>('/models', { token }),
      apiRequest<ProjectOption[]>('/projects?limit=100', { token }),
    ])
      .then(([providerData, modelData, projectData]) => {
        if (cancelled) return
        const nextProviders = Array.isArray(providerData) ? providerData : []
        const nextModels = Array.isArray(modelData) ? modelData : []
        const nextProjects = Array.isArray(projectData) ? projectData : []
        setProviders(nextProviders)
        setModels(nextModels)
        setProjects(nextProjects)
        setModelSelection((current) => current || (nextModels[0] ? `${nextModels[0].provider_id}:${nextModels[0].model_key}` : ''))
        setActiveProjectID((current) => {
          const matched = nextProjects.find((item) => item.id === projectID || item.master_key === projectID || item.id === current)
          return matched?.id || nextProjects[0]?.id || ''
        })
      })
      .catch((requestError) => {
        if (!cancelled) setError(requestError instanceof Error ? requestError.message : t('common.operationFailed'))
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
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : t('common.operationFailed'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <section data-testid="business-ai-workbench" className="border-t border-slate-200 bg-white px-4 py-5 sm:px-5">
      <div className="flex items-center gap-2">
        <Bot className="h-5 w-5 text-[#AD4714]" />
        <h2 className="text-base font-semibold text-slate-950">{t('businessAI.title')}</h2>
      </div>
      <div className="mt-4 grid grid-cols-5 overflow-hidden rounded-md border border-slate-300" role="group" aria-label={t('businessAI.stage')}>
        {stages.map((item) => (
          <button
            key={item}
            type="button"
            data-testid={`business-ai-stage-${item}`}
            onClick={() => setStage(item)}
            className={`min-h-10 border-r border-slate-300 px-1 text-xs font-semibold last:border-r-0 sm:px-2 sm:text-sm ${stage === item ? 'bg-slate-950 text-white' : 'bg-white text-slate-700 hover:bg-slate-100'}`}
          >
            {t(`businessAI.stage.${item}`)}
          </button>
        ))}
      </div>
      <label className="mt-4 block max-w-xl">
        <span className="text-sm font-medium text-slate-700">{t('businessAI.project')}</span>
        <select data-testid="business-ai-project" value={activeProjectID} onChange={(event) => setActiveProjectID(event.target.value)} className="mt-1 h-10 w-full rounded-md border border-slate-300 px-3 text-sm outline-none focus:border-[#AD4714] focus:ring-2 focus:ring-[#DF6A24]/20">
          {projects.length === 0 && <option value="">{t('businessAI.noProject')}</option>}
          {projects.map((item) => {
            return <option key={item.id} value={item.id}>{item.name} · {item.master_key || item.id}</option>
          })}
        </select>
      </label>
      <label className="mt-4 block">
        <span className="text-sm font-medium text-slate-700">{t('businessAI.focus')}</span>
        <textarea value={focus} onChange={(event) => setFocus(event.target.value)} className="mt-1 h-20 w-full resize-y rounded-md border border-slate-300 px-3 py-2 text-sm outline-none focus:border-[#AD4714] focus:ring-2 focus:ring-[#DF6A24]/20" />
      </label>
      <label className="mt-3 block max-w-md">
        <span className="text-sm font-medium text-slate-700">{t('businessAI.model')}</span>
        <select value={modelSelection} onChange={(event) => setModelSelection(event.target.value)} className="mt-1 h-10 w-full rounded-md border border-slate-300 px-3 text-sm outline-none focus:border-[#AD4714] focus:ring-2 focus:ring-[#DF6A24]/20">
          {models.length === 0 && <option value="">{t('businessAI.noModel')}</option>}
          {models.map((item) => <option key={`${item.provider_id}:${item.model_key}`} value={`${item.provider_id}:${item.model_key}`}>{item.display_name || item.model_key}</option>)}
        </select>
      </label>
      <button data-testid="business-ai-analyze" type="button" onClick={() => void analyze()} disabled={loading || !activeProjectID || !selectedModel || !providerType} className="mt-3 inline-flex h-10 items-center gap-2 rounded-md bg-slate-950 px-3 text-sm font-semibold text-white hover:bg-slate-800 disabled:opacity-50">
        {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : <Bot className="h-4 w-4" />}
        {t('businessAI.analyze')}
      </button>

      {error && <p className="mt-4 border-l-4 border-red-400 bg-red-50 px-4 py-3 text-sm text-red-700">{error}</p>}
      {latest?.analysis ? (
        <div className="mt-5 space-y-4 border-t border-slate-200 pt-4">
          <div className="flex flex-wrap items-start justify-between gap-2">
            <div>
              <p className="font-semibold text-slate-950">{latest.analysis.summary}</p>
              <p className="mt-1 text-xs text-slate-500">{t(`businessAI.stage.${latest.stage}`)} · {latest.resolved_model} · {Math.round(latest.analysis.confidence * 100)}%</p>
            </div>
            <span className="rounded-md bg-emerald-50 px-2 py-1 text-xs font-semibold text-emerald-700">{latest.status}</span>
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
          <p className="break-all text-xs text-slate-500">{t('businessAI.audit')}: {latest.invocation_id} · {latest.input_tokens + latest.output_tokens} tokens · {latest.cost_amount.toFixed(6)} {latest.currency}</p>
        </div>
      ) : !error ? (
        <p className="mt-4 border border-dashed border-slate-300 px-4 py-5 text-sm text-slate-500">{t('businessAI.empty')}</p>
      ) : null}
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
