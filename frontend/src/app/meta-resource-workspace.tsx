'use client'

import { Boxes, GitBranch, ListChecks, RefreshCw, Route, Sparkles } from 'lucide-react'
import { FormEvent, useCallback, useEffect, useMemo, useState } from 'react'
import type { ReactNode } from 'react'
import {
  createDemandProfile,
  createMetaResource,
  createPDCAEvent,
  createPDCACycle,
  getMetaResourceSummary,
  listDemandProfiles,
  listMetaResources,
  listPDCAEvents,
  listPDCACycles,
  syncExistingMetaResources,
  type DemandProfile,
  type MetaResource,
  type MetaResourceSummary,
  type PDCAEvent,
  type PDCACycle,
} from '@/lib/api'
import { useI18n } from '@/lib/i18n'
import { describeApiError } from '@/lib/api-error'

interface MetaResourceWorkspaceProps {
  token: string
}

type TabID = 'summary' | 'resources' | 'demands' | 'pdca'

const tabs: Array<{ id: TabID; label: string; icon: typeof Boxes }> = [
  { id: 'summary', label: 'metaresource.summary', icon: Boxes },
  { id: 'resources', label: 'metaresource.resources', icon: Sparkles },
  { id: 'demands', label: 'metaresource.demands', icon: Route },
  { id: 'pdca', label: 'metaresource.pdca', icon: GitBranch },
]

const resourceTypes = ['human', 'external_human', 'agent', 'model_channel', 'tool', 'material', 'time', 'capability', 'budget', 'resource']
const stages = ['plan', 'do', 'change', 'accept']

export function MetaResourceWorkspace({ token }: MetaResourceWorkspaceProps) {
  const { t } = useI18n()
  const [activeTab, setActiveTab] = useState<TabID>('summary')
  const [resources, setResources] = useState<MetaResource[]>([])
  const [summary, setSummary] = useState<MetaResourceSummary | null>(null)
  const [demands, setDemands] = useState<DemandProfile[]>([])
  const [cycles, setCycles] = useState<PDCACycle[]>([])
  const [events, setEvents] = useState<PDCAEvent[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [resourceForm, setResourceForm] = useState({
    resource_type: 'resource',
    name: '',
    capability_profile: '',
    cost_profile: '',
    capacity_profile: '',
    risk_profile: '',
  })
  const [demandForm, setDemandForm] = useState({
    title: '',
    goal: '',
    required_capabilities: '',
    acceptance_criteria: '',
    budget_constraints: '',
    time_constraints: '',
    risk_constraints: '',
  })
  const [cycleForm, setCycleForm] = useState({
    demand_profile_id: '',
    current_stage: 'plan',
    summary: '',
  })
  const [eventForm, setEventForm] = useState({
    cycle_id: '',
    stage: 'plan',
    event_type: 'note',
    decision: '',
    next_action: '',
    evidence: '',
  })

  const cycleLabels = useMemo(() => Object.fromEntries(cycles.map((cycle) => [cycle.id, `${cycle.current_stage} · ${cycle.summary || cycle.id}`])), [cycles])
  const demandLabels = useMemo(() => Object.fromEntries(demands.map((demand) => [demand.id, demand.title])), [demands])

  const loadAll = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const [resourceData, summaryData, demandData, cycleData, eventData] = await Promise.all([
        listMetaResources(token),
        getMetaResourceSummary(token),
        listDemandProfiles(token),
        listPDCACycles(token),
        listPDCAEvents(token),
      ])
      setResources(resourceData)
      setSummary(summaryData)
      setDemands(demandData)
      setCycles(cycleData)
      setEvents(eventData)
      setCycleForm((current) => ({ ...current, demand_profile_id: current.demand_profile_id || demandData[0]?.id || '' }))
      setEventForm((current) => ({ ...current, cycle_id: current.cycle_id || cycleData[0]?.id || '' }))
    } catch (err) {
      setError(describeApiError(err, t, t('metaresource.loadFailed')))
    } finally {
      setLoading(false)
    }
  }, [t, token])

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
      setError(describeApiError(err, t, t('common.operationFailed')))
    } finally {
      setLoading(false)
    }
  }

  async function syncResources() {
    await run(
      () =>
        syncExistingMetaResources(token).then((result) => {
          setNotice(Object.entries(result).map(([key, value]) => `${key}: ${value}`).join(', '))
        }),
      'metaresource.synced',
    )
  }

  async function submitResource(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await run(
      () =>
        createMetaResource(token, {
          resource_type: resourceForm.resource_type,
          name: resourceForm.name,
          capability_profile: jsonObject(resourceForm.capability_profile),
          cost_profile: jsonObject(resourceForm.cost_profile),
          capacity_profile: jsonObject(resourceForm.capacity_profile),
          risk_profile: jsonObject(resourceForm.risk_profile),
          metadata: { source_ui: 'meta_resource_workspace' },
        }).then(() => setResourceForm((current) => ({ ...current, name: '' }))),
      'metaresource.resourceCreated',
    )
  }

  async function submitDemand(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await run(
      () =>
        createDemandProfile(token, {
          title: demandForm.title,
          goal: demandForm.goal,
          required_capabilities: jsonList(demandForm.required_capabilities),
          acceptance_criteria: jsonList(demandForm.acceptance_criteria),
          budget_constraints: jsonObject(demandForm.budget_constraints),
          time_constraints: jsonObject(demandForm.time_constraints),
          risk_constraints: jsonObject(demandForm.risk_constraints),
          metadata: { source_ui: 'meta_resource_workspace' },
        }).then(() => setDemandForm((current) => ({ ...current, title: '', goal: '' }))),
      'metaresource.demandCreated',
    )
  }

  async function submitCycle(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await run(
      () =>
        createPDCACycle(token, {
          demand_profile_id: cycleForm.demand_profile_id || undefined,
          current_stage: cycleForm.current_stage,
          summary: cycleForm.summary,
          metadata: { source_ui: 'meta_resource_workspace' },
        }).then(() => setCycleForm((current) => ({ ...current, summary: '' }))),
      'metaresource.cycleCreated',
    )
  }

  async function submitEvent(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!eventForm.cycle_id) return
    await run(
      () =>
        createPDCAEvent(token, {
          cycle_id: eventForm.cycle_id,
          stage: eventForm.stage,
          event_type: eventForm.event_type,
          decision: eventForm.decision,
          next_action: eventForm.next_action,
          evidence: jsonObject(eventForm.evidence),
          metadata: { source_ui: 'meta_resource_workspace' },
        }).then(() => setEventForm((current) => ({ ...current, decision: '', next_action: '', evidence: '' }))),
      'metaresource.eventCreated',
    )
  }

  return (
    <div className="space-y-5">
      <div className="flex flex-wrap gap-2 rounded-lg border border-slate-200 bg-white p-2 shadow-sm">
        {tabs.map((tab) => {
          const Icon = tab.icon
          return (
            <button
              key={tab.id}
              type="button"
              onClick={() => setActiveTab(tab.id)}
              className={`inline-flex h-10 items-center gap-2 rounded-md px-3 text-sm font-semibold transition ${
                activeTab === tab.id ? 'bg-slate-950 text-white' : 'text-slate-600 hover:bg-slate-100'
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

      {activeTab === 'summary' && (
        <div className="space-y-5">
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
            <Metric icon={Boxes} label="metaresource.totalResources" value={String(summary?.total ?? 0)} />
            <Metric icon={Sparkles} label="metaresource.activeResources" value={String(summary?.active ?? 0)} />
            <Metric icon={Route} label="metaresource.demandCount" value={String(demands.length)} />
            <Metric icon={GitBranch} label="metaresource.cycleCount" value={String(cycles.length)} />
          </div>
          <div className="grid gap-5 lg:grid-cols-2">
            <Panel title="metaresource.byType">
              <KeyValue items={summary?.by_type ?? {}} />
            </Panel>
            <Panel title="metaresource.byStatus">
              <KeyValue items={summary?.by_status ?? {}} />
            </Panel>
          </div>
          <Panel title="metaresource.recentResources">
            <ResourceTable resources={summary?.recent ?? []} />
          </Panel>
        </div>
      )}

      {activeTab === 'resources' && (
        <div className="space-y-5">
          <Panel title="metaresource.resources">
            <div className="mb-4 flex justify-end">
              <button
                type="button"
                onClick={() => void syncResources()}
                disabled={loading}
                className="inline-flex h-10 items-center gap-2 rounded-lg bg-slate-950 px-3 text-sm font-semibold text-white hover:bg-slate-800 disabled:opacity-50"
              >
                <ListChecks className="h-4 w-4" />
                {t('metaresource.syncExisting')}
              </button>
            </div>
            <ResourceTable resources={resources} />
          </Panel>
          <Panel title="metaresource.createResource">
            <form className="space-y-3" onSubmit={submitResource}>
              <SelectInput label="metaresource.resourceType" value={resourceForm.resource_type} onChange={(value) => setResourceForm({ ...resourceForm, resource_type: value })} options={resourceTypes} prefix="metaresource.type" />
              <TextInput label="common.name" value={resourceForm.name} onChange={(value) => setResourceForm({ ...resourceForm, name: value })} />
              <TextInput label="metaresource.capabilityProfile" value={resourceForm.capability_profile} onChange={(value) => setResourceForm({ ...resourceForm, capability_profile: value })} />
              <TextInput label="metaresource.costProfile" value={resourceForm.cost_profile} onChange={(value) => setResourceForm({ ...resourceForm, cost_profile: value })} />
              <TextInput label="metaresource.capacityProfile" value={resourceForm.capacity_profile} onChange={(value) => setResourceForm({ ...resourceForm, capacity_profile: value })} />
              <TextInput label="metaresource.riskProfile" value={resourceForm.risk_profile} onChange={(value) => setResourceForm({ ...resourceForm, risk_profile: value })} />
              <SubmitButton loading={loading} label="metaresource.createResource" />
            </form>
          </Panel>
        </div>
      )}

      {activeTab === 'demands' && (
        <div className="space-y-5">
          <Panel title="metaresource.demands">
            <Table
              headers={['common.name', 'metaresource.goal', 'developer.status', 'metaresource.requiredCapabilities']}
              rows={demands.map((demand) => [
                demand.title,
                demand.goal || t('common.none'),
                t(demand.status),
                String(demand.required_capabilities.length),
              ])}
            />
          </Panel>
          <Panel title="metaresource.createDemand">
            <form className="space-y-3" onSubmit={submitDemand}>
              <TextInput label="common.name" value={demandForm.title} onChange={(value) => setDemandForm({ ...demandForm, title: value })} />
              <TextInput label="metaresource.goal" value={demandForm.goal} onChange={(value) => setDemandForm({ ...demandForm, goal: value })} />
              <TextInput label="metaresource.requiredCapabilities" value={demandForm.required_capabilities} onChange={(value) => setDemandForm({ ...demandForm, required_capabilities: value })} />
              <TextInput label="metaresource.acceptanceCriteria" value={demandForm.acceptance_criteria} onChange={(value) => setDemandForm({ ...demandForm, acceptance_criteria: value })} />
              <TextInput label="metaresource.budgetConstraints" value={demandForm.budget_constraints} onChange={(value) => setDemandForm({ ...demandForm, budget_constraints: value })} />
              <TextInput label="metaresource.timeConstraints" value={demandForm.time_constraints} onChange={(value) => setDemandForm({ ...demandForm, time_constraints: value })} />
              <TextInput label="metaresource.riskConstraints" value={demandForm.risk_constraints} onChange={(value) => setDemandForm({ ...demandForm, risk_constraints: value })} />
              <SubmitButton loading={loading} label="metaresource.createDemand" />
            </form>
          </Panel>
        </div>
      )}

      {activeTab === 'pdca' && (
        <div className="space-y-5">
          <div className="space-y-5">
            <Panel title="metaresource.cycles">
              <Table
                headers={['metaresource.stage', 'developer.status', 'metaresource.summary', 'metaresource.demand']}
                rows={cycles.map((cycle) => [
                  t(`metaresource.stage.${cycle.current_stage}`),
                  t(cycle.status),
                  cycle.summary || t('common.none'),
                  cycle.demand_profile_id ? demandLabels[cycle.demand_profile_id] || cycle.demand_profile_id : t('common.none'),
                ])}
              />
            </Panel>
            <Panel title="metaresource.events">
              <Table
                headers={['metaresource.stage', 'metaresource.eventType', 'metaresource.decision', 'metaresource.nextAction']}
                rows={events.map((event) => [
                  t(`metaresource.stage.${event.stage}`),
                  event.event_type,
                  event.decision || t('common.none'),
                  event.next_action || t('common.none'),
                ])}
              />
            </Panel>
          </div>
          <div className="grid gap-5 xl:grid-cols-2">
            <Panel title="metaresource.createCycle">
              <form className="space-y-3" onSubmit={submitCycle}>
                <SelectInput label="metaresource.demand" value={cycleForm.demand_profile_id} onChange={(value) => setCycleForm({ ...cycleForm, demand_profile_id: value })} options={['', ...demands.map((demand) => demand.id)]} labels={{ '': t('common.none'), ...demandLabels }} />
                <SelectInput label="metaresource.stage" value={cycleForm.current_stage} onChange={(value) => setCycleForm({ ...cycleForm, current_stage: value })} options={stages} prefix="metaresource.stage" />
                <TextInput label="metaresource.summary" value={cycleForm.summary} onChange={(value) => setCycleForm({ ...cycleForm, summary: value })} />
                <SubmitButton loading={loading} label="metaresource.createCycle" />
              </form>
            </Panel>
            <Panel title="metaresource.createEvent">
              <form className="space-y-3" onSubmit={submitEvent}>
                <SelectInput label="metaresource.cycle" value={eventForm.cycle_id} onChange={(value) => setEventForm({ ...eventForm, cycle_id: value })} options={cycles.map((cycle) => cycle.id)} labels={cycleLabels} />
                <SelectInput label="metaresource.stage" value={eventForm.stage} onChange={(value) => setEventForm({ ...eventForm, stage: value })} options={stages} prefix="metaresource.stage" />
                <TextInput label="metaresource.eventType" value={eventForm.event_type} onChange={(value) => setEventForm({ ...eventForm, event_type: value })} />
                <TextInput label="metaresource.decision" value={eventForm.decision} onChange={(value) => setEventForm({ ...eventForm, decision: value })} />
                <TextInput label="metaresource.nextAction" value={eventForm.next_action} onChange={(value) => setEventForm({ ...eventForm, next_action: value })} />
                <TextInput label="metaresource.evidence" value={eventForm.evidence} onChange={(value) => setEventForm({ ...eventForm, evidence: value })} />
                <SubmitButton loading={loading} label="metaresource.createEvent" />
              </form>
            </Panel>
          </div>
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

function Metric({ icon: Icon, label, value }: { icon: typeof Boxes; label: string; value: string }) {
  const { t } = useI18n()
  return (
    <div className="rounded-lg border border-slate-200 bg-white p-4 shadow-sm">
      <div className="flex items-center justify-between gap-3">
        <p className="text-xs font-semibold text-slate-500">{t(label)}</p>
        <Icon className="h-4 w-4 text-slate-400" />
      </div>
      <p className="mt-3 text-2xl font-semibold text-slate-950">{value}</p>
    </div>
  )
}

function ResourceTable({ resources }: { resources: MetaResource[] }) {
  const { t } = useI18n()
  return (
    <Table
      headers={['common.name', 'metaresource.resourceType', 'developer.status', 'metaresource.source', 'metaresource.capabilityProfile']}
      rows={resources.map((resource) => [
        resource.name,
        t(`metaresource.type.${resource.resource_type}`),
        t(resource.status),
        resource.source_type ? `${resource.source_type}:${resource.source_id || ''}` : t('common.none'),
        compactJSON(resource.capability_profile),
      ])}
    />
  )
}

function KeyValue({ items }: { items: Record<string, number> }) {
  const { t } = useI18n()
  const entries = Object.entries(items)
  return (
    <div className="space-y-2">
      {entries.map(([key, value]) => (
        <div key={key} className="flex items-center justify-between gap-3 text-sm">
          <span className="truncate text-slate-600">{t(`metaresource.type.${key}`)}</span>
          <span className="font-semibold text-slate-950">{value}</span>
        </div>
      ))}
      {entries.length === 0 && <p className="text-sm text-slate-500">{t('common.noData')}</p>}
    </div>
  )
}

function TextInput({ label, value, onChange }: { label: string; value: string; onChange: (value: string) => void }) {
  const { t } = useI18n()
  return (
    <label className="block">
      <span className="text-xs font-semibold text-slate-500">{t(label)}</span>
      <input
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
  prefix,
}: {
  label: string
  value: string
  onChange: (value: string) => void
  options: string[]
  labels?: Record<string, string>
  prefix?: string
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
            {labels[option] ?? (prefix ? t(`${prefix}.${option}`) : option)}
          </option>
        ))}
      </select>
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

function jsonObject(value: string): Record<string, unknown> {
  if (!value.trim()) return {}
  try {
    const parsed = JSON.parse(value)
    return typeof parsed === 'object' && parsed && !Array.isArray(parsed) ? (parsed as Record<string, unknown>) : { value }
  } catch {
    return { value }
  }
}

function jsonList(value: string): Array<Record<string, unknown>> {
  if (!value.trim()) return []
  try {
    const parsed = JSON.parse(value)
    if (Array.isArray(parsed)) return parsed.filter((item) => typeof item === 'object' && item) as Array<Record<string, unknown>>
  } catch {
    return value
      .split(',')
      .map((item) => item.trim())
      .filter(Boolean)
      .map((item) => ({ name: item }))
  }
  return []
}

function compactJSON(value: Record<string, unknown>): string {
  const raw = JSON.stringify(value)
  return raw === '{}' ? '' : raw
}
