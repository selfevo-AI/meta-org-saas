'use client'

import { AlertCircle, AlertTriangle, CheckCircle2, ListChecks, Play, RefreshCw, X } from 'lucide-react'
import { FormEvent, useMemo, useState } from 'react'
import type { ReactNode } from 'react'
import { apiRequest } from '@/lib/api'
import { useI18n } from '@/lib/i18n'
import { getOperationProfile, type ApiOperation, type OperationDangerLevel, type OperationKind } from '@/lib/operations'

interface OperationFormState {
  path: Record<string, string>
  query: Record<string, string>
  body: string
}

export const methodTone: Record<ApiOperation['method'], string> = {
  GET: 'border-emerald-200 bg-emerald-50 text-emerald-700',
  POST: 'border-blue-200 bg-blue-50 text-blue-700',
  PUT: 'border-amber-200 bg-amber-50 text-amber-700',
  PATCH: 'border-violet-200 bg-violet-50 text-violet-700',
  DELETE: 'border-red-200 bg-red-50 text-red-700',
}

export function createOperationFormState(operation: ApiOperation, initialPathValues: Record<string, string> = {}): OperationFormState {
  return {
    path: Object.fromEntries((operation.pathParams ?? []).map((field) => [field.name, initialPathValues[field.name] ?? ''])),
    query: Object.fromEntries((operation.queryParams ?? []).map((field) => [field.name, ''])),
    body: operation.bodyTemplate === undefined ? '' : JSON.stringify(operation.bodyTemplate, null, 2),
  }
}

export function buildOperationRequestPath(operation: ApiOperation, state: OperationFormState): string {
  let path = operation.path
  for (const [name, value] of Object.entries(state.path)) {
    path = path.replace(`{${name}}`, encodeURIComponent(value.trim()))
  }

  const query = new URLSearchParams()
  for (const [name, value] of Object.entries(state.query)) {
    const trimmed = value.trim()
    if (trimmed !== '') query.set(name, trimmed)
  }

  const queryString = query.toString()
  return queryString ? `${path}?${queryString}` : path
}

function scopeOperationRequestPath(path: string, apiScope: 'tenant' | 'platform'): string {
  if (apiScope !== 'platform' || path.startsWith('/platform/admin')) return path
  return `/platform/admin${path.startsWith('/') ? path : `/${path}`}`
}

function parseBody(operation: ApiOperation, bodyText: string): unknown {
  if (operation.bodyTemplate === undefined) return undefined
  if (bodyText.trim() === '') return {}
  return JSON.parse(bodyText)
}

function formatJSON(value: unknown): string {
  return JSON.stringify(value, null, 2)
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function extractResultSummary(value: unknown): { title: string; details: string[] } {
  if (Array.isArray(value)) {
    return {
      title: 'operation.result.listTitle',
      details: [`${value.length}`],
    }
  }

  if (!isRecord(value)) {
    return {
      title: 'operation.result.completedTitle',
      details: [],
    }
  }

  const details: string[] = []
  const id = typeof value.id === 'string' ? value.id : undefined
  const status = typeof value.status === 'string' ? value.status : undefined
  const name = typeof value.name === 'string' ? value.name : typeof value.title === 'string' ? value.title : undefined
  const message = typeof value.message === 'string' ? value.message : undefined

  if (name) details.push(name)
  if (status) details.push(status)
  if (id) details.push(id)
  if (message) details.push(message)

  return {
    title: 'operation.result.completedTitle',
    details,
  }
}

export function OperationRunner({
  token,
  operation,
  initialPathValues,
  compact = false,
  apiScope = 'tenant',
}: {
  token: string
  operation: ApiOperation
  initialPathValues?: Record<string, string>
  compact?: boolean
  apiScope?: 'tenant' | 'platform'
}) {
  const { t } = useI18n()
  const [formState, setFormState] = useState<OperationFormState>(() => createOperationFormState(operation, initialPathValues))
  const [response, setResponse] = useState<unknown>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  const requestPath = useMemo(() => buildOperationRequestPath(operation, formState), [operation, formState])
  const scopedRequestPath = useMemo(() => scopeOperationRequestPath(requestPath, apiScope), [apiScope, requestPath])
  const profile = useMemo(() => getOperationProfile(operation), [operation])
  const resultSummary = useMemo(() => extractResultSummary(response), [response])

  function updatePathValue(name: string, value: string) {
    setFormState((current) => ({ ...current, path: { ...current.path, [name]: value } }))
  }

  function updateQueryValue(name: string, value: string) {
    setFormState((current) => ({ ...current, query: { ...current.query, [name]: value } }))
  }

  async function submitOperation(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setLoading(true)
    setError(null)
    setResponse(null)

    if (profile.dangerLevel === 'high' && !window.confirm(t('operation.confirmHighRisk'))) {
      setLoading(false)
      return
    }

    try {
      const body = parseBody(operation, formState.body)
      const result = await apiRequest<unknown>(scopedRequestPath, {
        method: operation.method,
        token: operation.auth === false ? undefined : token,
        body,
      })
      setResponse(result)
    } catch (err) {
      setError(err instanceof Error ? err.message : t('operation.requestFailed'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <section className={compact ? '' : 'rounded-lg border border-slate-200 bg-white p-5 shadow-sm'}>
      <div className="flex flex-col gap-3 border-b border-slate-100 pb-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="min-w-0">
          <div className="flex min-w-0 items-center gap-2">
            <MethodBadge method={operation.method} />
            <OperationKindBadge kind={profile.kind} />
            <DangerBadge dangerLevel={profile.dangerLevel} />
            <h2 className="truncate text-base font-semibold text-slate-950">{t(operation.title)}</h2>
          </div>
          <p className="mt-2 break-all text-sm text-slate-500">{scopedRequestPath}</p>
        </div>
        {operation.auth === false ? <StatusBadge label={t('common.public')} tone="blue" /> : <StatusBadge label={t('common.jwt')} tone="green" />}
      </div>

      <div className="mt-4 grid gap-3 sm:grid-cols-3">
        <InfoTile label="operation.humanReady" value={profile.humanReady ? t('operation.ready.yes') : t('operation.ready.needsContext')} />
        <InfoTile label="operation.inputMode" value={profile.requiresEntityContext ? t('operation.input.contextual') : t('operation.input.direct')} />
        <InfoTile label="operation.resultMode" value={t(`operation.resultView.${profile.resultView}`)} />
      </div>

      <form className="mt-5 space-y-5" onSubmit={submitOperation}>
        {profile.requiresEntityContext && (
          <div className="flex items-start gap-2 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800">
            <AlertTriangle className="mt-0.5 h-4 w-4 flex-none" />
            <span>{t('operation.contextHint')}</span>
          </div>
        )}

        {operation.pathParams && operation.pathParams.length > 0 && (
          <FieldGroup title="common.pathParams">
            {operation.pathParams.map((field) => (
              <TextInput
                key={field.name}
                field={field}
                value={formState.path[field.name] ?? ''}
                onChange={(value) => updatePathValue(field.name, value)}
              />
            ))}
          </FieldGroup>
        )}

        {operation.queryParams && operation.queryParams.length > 0 && (
          <FieldGroup title="common.queryParams">
            {operation.queryParams.map((field) => (
              <TextInput
                key={field.name}
                field={field}
                value={formState.query[field.name] ?? ''}
                onChange={(value) => updateQueryValue(field.name, value)}
              />
            ))}
          </FieldGroup>
        )}

        {operation.bodyTemplate !== undefined && (
          <div>
            <label className="text-sm font-medium text-slate-700" htmlFor="operation-body">
              {t('operation.jsonBody')}
            </label>
            <textarea
              id="operation-body"
              value={formState.body}
              onChange={(event) => setFormState((current) => ({ ...current, body: event.target.value }))}
              className="mt-2 h-56 w-full resize-y rounded-lg border border-slate-300 bg-slate-950 p-3 font-mono text-sm text-slate-50 outline-none transition focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
              spellCheck={false}
            />
          </div>
        )}

        {error && (
          <div className="flex items-start gap-2 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
            <AlertCircle className="mt-0.5 h-4 w-4 flex-none" />
            <span>{error}</span>
          </div>
        )}

        <button
          type="submit"
          disabled={loading}
          className="inline-flex h-10 items-center justify-center gap-2 rounded-lg bg-[#AD4714] px-4 text-sm font-semibold text-[#fffaf5] transition hover:bg-[#B84F18] disabled:cursor-not-allowed disabled:opacity-60"
        >
          {loading ? <RefreshCw className="h-4 w-4 animate-spin" /> : <Play className="h-4 w-4" />}
          {t('common.execute')}
        </button>
      </form>

      <div className="mt-6">
        <div className="mb-2 flex items-center gap-2">
          <CheckCircle2 className="h-4 w-4 text-slate-500" />
          <h3 className="text-sm font-semibold text-slate-950">{t('common.response')}</h3>
        </div>
        {response === null ? (
          <div className="min-h-[120px] rounded-lg border border-dashed border-slate-200 bg-slate-50 p-4 text-sm text-slate-500">
            {t('common.emptyResponse')}
          </div>
        ) : (
          <div className="space-y-3">
            <div className="rounded-lg border border-emerald-200 bg-emerald-50 p-3">
              <div className="flex items-center gap-2 text-sm font-semibold text-emerald-800">
                <ListChecks className="h-4 w-4" />
                {t(resultSummary.title)}
              </div>
              {resultSummary.details.length > 0 && (
                <div className="mt-2 flex flex-wrap gap-2">
                  {resultSummary.details.slice(0, 4).map((detail) => (
                    <span key={detail} className="max-w-full truncate rounded-md border border-emerald-200 bg-white px-2 py-1 text-xs font-medium text-emerald-800">
                      {detail}
                    </span>
                  ))}
                </div>
              )}
            </div>
            <details className="rounded-lg border border-slate-200 bg-slate-50">
              <summary className="cursor-pointer px-3 py-2 text-sm font-semibold text-slate-700">{t('operation.rawResponse')}</summary>
              <pre className="max-h-[360px] overflow-auto border-t border-slate-200 p-3 text-sm text-slate-800">
                {formatJSON(response)}
              </pre>
            </details>
          </div>
        )}
      </div>
    </section>
  )
}

export function OperationRunnerDrawer({
  token,
  operation,
  initialPathValues,
  onClose,
}: {
  token: string
  operation: ApiOperation | null
  initialPathValues?: Record<string, string>
  onClose: () => void
}) {
  const { t } = useI18n()
  if (!operation) return null
  return (
    <div className="fixed inset-0 z-50">
      <button type="button" className="absolute inset-0 bg-black/55" aria-label={t('common.close')} onClick={onClose} />
      <aside className="absolute right-0 top-0 flex h-full w-full max-w-2xl flex-col bg-white shadow-2xl">
        <div className="flex items-center justify-between gap-3 border-b border-slate-200 px-5 py-4">
          <div className="min-w-0">
            <p className="text-xs font-bold uppercase tracking-[0.16em] text-slate-500">{t('operation.drawerTitle')}</p>
            <h2 className="mt-1 truncate text-lg font-semibold text-slate-950">{t(operation.title)}</h2>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border border-slate-200 text-slate-500 transition hover:bg-slate-100 hover:text-slate-900"
            aria-label={t('common.close')}
          >
            <X className="h-4 w-4" />
          </button>
        </div>
        <div className="flex-1 overflow-y-auto p-5">
          <OperationRunner key={`${operation.id}-${JSON.stringify(initialPathValues ?? {})}`} token={token} operation={operation} initialPathValues={initialPathValues} compact />
        </div>
      </aside>
    </div>
  )
}

export function MethodBadge({ method }: { method: ApiOperation['method'] }) {
  return <span className={`inline-flex h-6 items-center rounded-md border px-2 text-xs font-semibold ${methodTone[method]}`}>{method}</span>
}

export function OperationKindBadge({ kind }: { kind: OperationKind }) {
  const { t } = useI18n()
  const toneClass = {
    direct: 'border-blue-200 bg-blue-50 text-blue-700',
    contextual: 'border-amber-200 bg-amber-50 text-amber-700',
    agent_assisted: 'border-violet-200 bg-violet-50 text-violet-700',
    admin: 'border-slate-200 bg-slate-50 text-slate-700',
  }[kind]

  return <span className={`inline-flex h-6 items-center rounded-md border px-2 text-xs font-semibold ${toneClass}`}>{t(`operation.kind.${kind}`)}</span>
}

export function DangerBadge({ dangerLevel }: { dangerLevel: OperationDangerLevel }) {
  const { t } = useI18n()
  const toneClass = {
    low: 'border-emerald-200 bg-emerald-50 text-emerald-700',
    medium: 'border-amber-200 bg-amber-50 text-amber-700',
    high: 'border-red-200 bg-red-50 text-red-700',
  }[dangerLevel]

  return <span className={`inline-flex h-6 items-center rounded-md border px-2 text-xs font-semibold ${toneClass}`}>{t(`operation.danger.${dangerLevel}`)}</span>
}

function InfoTile({ label, value }: { label: string; value: string }) {
  const { t } = useI18n()
  return (
    <div className="rounded-lg border border-slate-200 bg-slate-50 px-3 py-2">
      <p className="text-xs font-semibold text-slate-500">{t(label)}</p>
      <p className="mt-1 truncate text-sm font-semibold text-slate-900">{value}</p>
    </div>
  )
}

function FieldGroup({ title, children }: { title: string; children: ReactNode }) {
  const { t } = useI18n()
  return (
    <div>
      <h3 className="text-sm font-semibold text-slate-950">{t(title)}</h3>
      <div className="mt-3 grid gap-3 sm:grid-cols-2">{children}</div>
    </div>
  )
}

function TextInput({
  field,
  value,
  onChange,
}: {
  field: { name: string; label: string; placeholder?: string }
  value: string
  onChange: (value: string) => void
}) {
  const { t } = useI18n()
  return (
    <label className="block">
      <span className="text-sm font-medium text-slate-700">{t(field.label)}</span>
      <input
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={field.placeholder ? t(field.placeholder) : undefined}
        className="mt-1 h-10 w-full rounded-lg border border-slate-300 px-3 text-sm outline-none transition focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
      />
    </label>
  )
}

function StatusBadge({ label, tone }: { label: string; tone: 'blue' | 'green' }) {
  const toneClass = {
    blue: 'border-blue-200 bg-blue-50 text-blue-700',
    green: 'border-emerald-200 bg-emerald-50 text-emerald-700',
  }[tone]

  return <span className={`inline-flex h-7 items-center rounded-md border px-2.5 text-xs font-semibold ${toneClass}`}>{label}</span>
}
