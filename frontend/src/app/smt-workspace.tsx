'use client'

import { useCallback, useEffect, useMemo, useState } from 'react'
import type { DependencyList, ReactNode } from 'react'
import dynamic from 'next/dynamic'
import {
  AlertTriangle,
  Bell,
  CheckCircle2,
  ClipboardCheck,
  Cpu,
  Gauge,
  History,
  ListChecks,
  Loader2,
  Play,
  Radio,
  RefreshCw,
  ScanLine,
  Search,
  Settings2,
  ShieldCheck,
  Wrench,
  X,
} from 'lucide-react'
import {
  createSMTEdge,
  deleteSMTEdge,
  getSMTBoard,
  getSMTMachineContext,
  getSMTReelUnits,
  getSMTReport,
  getSMTSettings,
  getSMTTraceSN,
  getSMTWorkOrderProgress,
  listERPRecords,
  listSMTEdgeEvents,
  listSMTEdges,
  pullSMTEdge,
  pushSMTEdge,
  runERPAction,
  updateSMTEdge,
  updateSMTSettings,
} from '@/lib/api'
import type { ERPActionResult, SMTEdgeEvent, SMTEdgeInstance, SMTMachineContext } from '@/lib/api'
import { useI18n } from '@/lib/i18n'
import { smtBeep, smtSpeak } from '@/lib/smt-feedback'
import { FeedbackMessage } from './workspace-ui'

const ERPBusinessModuleWorkspace = dynamic(
  () => import('./erp-business-module-workspace').then((module) => module.ERPBusinessModuleWorkspace),
  { loading: () => <SMTLoading /> },
)

// ---------------------------------------------------------------------------
// Shared primitives.
// ---------------------------------------------------------------------------

type SMTRecord = Record<string, unknown> & { key: string }
type SMTFeedbackKind = 'OK' | 'ERROR' | 'CONFIRM'
type SMTFeedbackState = { kind: SMTFeedbackKind; message: string; voice?: string }

const smtTabList = [
  { id: 'station', labelKey: 'smt.tab.station', icon: ScanLine },
  { id: 'units', labelKey: 'smt.tab.units', icon: Play },
  { id: 'repair', labelKey: 'smt.tab.repair', icon: Wrench },
  { id: 'andon', labelKey: 'smt.tab.andon', icon: Bell },
  { id: 'board', labelKey: 'smt.tab.board', icon: Gauge },
  { id: 'trace', labelKey: 'smt.tab.trace', icon: Search },
  { id: 'reports', labelKey: 'smt.tab.reports', icon: ClipboardCheck },
  { id: 'settings', labelKey: 'smt.tab.settings', icon: Settings2 },
  { id: 'edges', labelKey: 'smt.tab.edges', icon: Radio },
  { id: 'master', labelKey: 'smt.tab.master', icon: ListChecks },
] as const
type SMTTab = (typeof smtTabList)[number]['id']

const smtButton = 'inline-flex h-9 items-center justify-center gap-1.5 rounded-lg border border-slate-700 px-3 text-sm font-semibold text-slate-200 transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-40'
const smtPrimaryButton = 'inline-flex h-9 items-center justify-center gap-1.5 rounded-lg bg-[#DF6A24] px-3 text-sm font-semibold text-white transition hover:bg-[#c95c1c] disabled:cursor-not-allowed disabled:opacity-40'
const smtDangerButton = 'inline-flex h-9 items-center justify-center gap-1.5 rounded-lg border border-red-900 px-3 text-sm font-semibold text-red-300 transition hover:bg-red-950/60 disabled:cursor-not-allowed disabled:opacity-40'
const smtInput = 'h-10 w-full rounded-lg border border-slate-700 bg-slate-950 px-3 text-sm text-slate-100 outline-none placeholder:text-slate-600 focus:border-[#DF6A24]'
const smtLabel = 'mb-1 block text-xs font-semibold uppercase tracking-wide text-slate-400'
const smtTableWrap = 'overflow-x-auto rounded-lg border border-slate-800'
const smtTh = 'whitespace-nowrap px-3 py-2 text-xs font-semibold uppercase tracking-wide text-slate-500'
const smtTd = 'whitespace-nowrap px-3 py-2 text-slate-300'

const smtStateStyles: Record<string, string> = {
  OK: 'bg-emerald-950/50 text-emerald-300 border-emerald-800',
  ALT: 'bg-sky-950/50 text-sky-300 border-sky-800',
  MISMATCH: 'bg-red-950/50 text-red-300 border-red-800',
  EXTRA: 'bg-amber-950/50 text-amber-300 border-amber-800',
  EMPTY: 'bg-slate-900 text-slate-400 border-slate-700',
  NA: 'bg-slate-950 text-slate-600 border-slate-800',
}

function smtKey(prefix: string): string {
  const stamp = new Date().toISOString().replace(/\D/g, '').slice(2, 14)
  return `${prefix}-${stamp}-${Math.floor(Math.random() * 9000 + 1000)}`
}

function smtErrorText(error: unknown): string {
  if (error instanceof Error && error.message) return error.message
  return String(error)
}

function fieldText(record: Record<string, unknown> | undefined, ...keys: string[]): string {
  if (!record) return ''
  for (const key of keys) {
    const value = record[key]
    if (value === undefined || value === null || value === '') continue
    if (typeof value === 'object') continue
    return String(value)
  }
  return ''
}

function numberValue(value: unknown): number {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) return parsed
  }
  return 0
}

function fieldNumber(record: Record<string, unknown> | undefined, ...keys: string[]): number {
  if (!record) return 0
  for (const key of keys) {
    const value = record[key]
    if (value === undefined || value === null || value === '') continue
    return numberValue(value)
  }
  return 0
}

function boolValue(value: unknown): boolean {
  if (typeof value === 'boolean') return value
  if (typeof value === 'string') {
    const lower = value.trim().toLowerCase()
    return lower === 'true' || lower === '1' || lower === 'y' || lower === 'yes'
  }
  return false
}

function objectList(value: unknown): Array<Record<string, unknown>> {
  if (!Array.isArray(value)) return []
  return value.filter((item): item is Record<string, unknown> => typeof item === 'object' && item !== null)
}

function percent(value: number): string {
  return `${(value * 100).toFixed(1)}%`
}

function todayISO(): string {
  return new Date().toISOString().slice(0, 10)
}

function daysAgoISO(days: number): string {
  return new Date(Date.now() - days * 24 * 3600 * 1000).toISOString().slice(0, 10)
}

function SMTLoading() {
  const { t } = useI18n()
  return (
    <p className="flex items-center gap-2 text-sm text-slate-500">
      <Loader2 className="h-4 w-4 animate-spin" />
      {t('common.loading')}
    </p>
  )
}

function Panel({ title, actions, children }: { title: string; actions?: ReactNode; children: ReactNode }) {
  return (
    <section className="min-w-0 rounded-lg border border-slate-800 bg-[#17181d] p-4">
      <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
        <h3 className="text-sm font-semibold text-slate-100">{title}</h3>
        {actions}
      </div>
      {children}
    </section>
  )
}

function SMTFeedbackBanner({ feedback }: { feedback: SMTFeedbackState | null }) {
  if (!feedback) return null
  const tone =
    feedback.kind === 'OK'
      ? 'border-emerald-700 bg-emerald-950/50 text-emerald-200'
      : feedback.kind === 'CONFIRM'
        ? 'border-amber-700 bg-amber-950/50 text-amber-200'
        : 'border-red-800 bg-red-950/50 text-red-200'
  const Icon = feedback.kind === 'OK' ? CheckCircle2 : feedback.kind === 'CONFIRM' ? AlertTriangle : X
  return (
    <div className={`flex items-start gap-2 rounded-lg border px-3 py-2 text-sm ${tone}`} role="status" aria-live="assertive">
      <Icon className="mt-0.5 h-4 w-4 shrink-0" />
      <div className="min-w-0">
        <p className="font-semibold">{feedback.message}</p>
        {feedback.voice ? <p className="text-xs opacity-80">{feedback.voice}</p> : null}
      </div>
    </div>
  )
}

function useSMTFeedback() {
  const [feedback, setFeedback] = useState<SMTFeedbackState | null>(null)
  const notify = useCallback((kind: SMTFeedbackKind, message: string, voice = '') => {
    setFeedback({ kind, message, voice })
    smtBeep(kind)
    if (voice) smtSpeak(voice)
  }, [])
  const clear = useCallback(() => setFeedback(null), [])
  return { feedback, notify, clear }
}

function useSMTResource<T>(loader: () => Promise<T>, deps: DependencyList, enabled = true) {
  const [data, setData] = useState<T | null>(null)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [version, setVersion] = useState(0)
  useEffect(() => {
    if (!enabled) return
    let cancelled = false
    const timer = window.setTimeout(() => {
      setLoading(true)
      setError('')
      loader()
        .then((result) => {
          if (!cancelled) setData(result)
        })
        .catch((err: unknown) => {
          if (!cancelled) setError(smtErrorText(err))
        })
        .finally(() => {
          if (!cancelled) setLoading(false)
        })
    }, 0)
    return () => {
      cancelled = true
      window.clearTimeout(timer)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, version, enabled])
  const reload = useCallback(() => setVersion((value) => value + 1), [])
  return { data, error, loading, reload, setData }
}

// ---------------------------------------------------------------------------
// Workspace shell.
// ---------------------------------------------------------------------------

export function SMTWorkspace({ token }: { token: string }) {
  const { t } = useI18n()
  const [tab, setTab] = useState<SMTTab>('station')
  return (
    <div className="min-w-0 space-y-4">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="text-lg font-semibold text-white">{t('erp.module.smt_mes')}</h2>
          <p className="text-sm text-slate-400">{t('smt.edges.hint')}</p>
        </div>
        <Cpu className="hidden h-6 w-6 text-slate-600 sm:block" />
      </header>
      <nav className="flex flex-wrap gap-2" aria-label={t('erp.module.smt_mes')}>
        {smtTabList.map((entry) => {
          const Icon = entry.icon
          const active = entry.id === tab
          return (
            <button
              key={entry.id}
              type="button"
              onClick={() => setTab(entry.id)}
              data-testid={`smt-tab-${entry.id}`}
              className={`inline-flex h-9 items-center gap-1.5 rounded-lg border px-3 text-sm font-semibold transition ${
                active ? 'border-[#DF6A24] bg-[#DF6A24]/15 text-[#f0a06a]' : 'border-slate-800 text-slate-300 hover:bg-slate-800'
              }`}
            >
              <Icon className="h-4 w-4" />
              {t(entry.labelKey)}
            </button>
          )
        })}
      </nav>
      {tab === 'station' ? <SMTStationTab token={token} /> : null}
      {tab === 'units' ? <SMTUnitsTab token={token} /> : null}
      {tab === 'repair' ? <SMTQualityTab token={token} /> : null}
      {tab === 'andon' ? <SMTAndonTab token={token} /> : null}
      {tab === 'board' ? <SMTBoardTab token={token} /> : null}
      {tab === 'trace' ? <SMTTraceTab token={token} /> : null}
      {tab === 'reports' ? <SMTReportsTab token={token} /> : null}
      {tab === 'settings' ? <SMTSettingsTab token={token} /> : null}
      {tab === 'edges' ? <SMTEdgesTab token={token} /> : null}
      {tab === 'master' ? <ERPBusinessModuleWorkspace token={token} module="smt_mes" /> : null}
    </div>
  )
}

// ---------------------------------------------------------------------------
// Loading station.
// ---------------------------------------------------------------------------

function SMTStationTab({ token }: { token: string }) {
  const { t } = useI18n()
  const { feedback, notify } = useSMTFeedback()
  const [machines, setMachines] = useState<SMTRecord[]>([])
  const [machineCode, setMachineCode] = useState('')
  const [context, setContext] = useState<SMTMachineContext | null>(null)
  const [contextError, setContextError] = useState('')
  const [busy, setBusy] = useState(false)
  const [slotInput, setSlotInput] = useState('')
  const [reelInput, setReelInput] = useState('')
  const [workOrders, setWorkOrders] = useState<SMTRecord[]>([])
  const [bindWorkOrder, setBindWorkOrder] = useState('')
  const [unloadTarget, setUnloadTarget] = useState<{ slotNo: string; loadCode: string } | null>(null)
  const [unloadReason, setUnloadReason] = useState('CHANGEOVER')
  const [unloadDisposition, setUnloadDisposition] = useState('RETURN')
  const [unloadQty, setUnloadQty] = useState('')
  const [progressQty, setProgressQty] = useState('')
  const [alerts, setAlerts] = useState<SMTRecord[]>([])

  const reloadContext = useCallback(
    async (code: string) => {
      if (!code) {
        setContext(null)
        return
      }
      try {
        const next = await getSMTMachineContext(token, code)
        setContext(next)
        setContextError('')
      } catch (err) {
        setContext(null)
        setContextError(smtErrorText(err))
      }
    },
    [token],
  )

  const reloadAlerts = useCallback(async () => {
    try {
      const rows = await listERPRecords<SMTRecord>(token, 'MLWA', 100)
      setAlerts(rows.filter((row) => fieldText(row, 'status') !== 'RESOLVED' && fieldText(row, 'status') !== 'closed'))
    } catch {
      setAlerts([])
    }
  }, [token])

  useEffect(() => {
    let cancelled = false
    void Promise.all([
      listERPRecords<SMTRecord>(token, 'MMAC', 200).catch(() => []),
      listERPRecords<SMTRecord>(token, 'MSWO', 200).catch(() => []),
    ]).then(([machineRows, orderRows]) => {
      if (cancelled) return
      setMachines(machineRows)
      setWorkOrders(orderRows.filter((row) => fieldText(row, 'Status', 'status') !== 'CLOSED'))
      setMachineCode((current) => current || machineRows[0]?.key || '')
    })
    return () => {
      cancelled = true
    }
  }, [token])

  useEffect(() => {
    const timer = window.setTimeout(() => void reloadContext(machineCode), 0)
    return () => window.clearTimeout(timer)
  }, [machineCode, reloadContext])

  useEffect(() => {
    const initial = window.setTimeout(() => void reloadAlerts(), 0)
    const timer = window.setInterval(() => void reloadAlerts(), 60000)
    return () => {
      window.clearTimeout(initial)
      window.clearInterval(timer)
    }
  }, [reloadAlerts])

  const applyResult = useCallback(
    (result: ERPActionResult) => {
      const effects: Record<string, unknown> = result.effects ?? {}
      const status = result.status.toUpperCase()
      const kind: SMTFeedbackKind = status === 'OK' ? 'OK' : status === 'CONFIRM' ? 'CONFIRM' : 'ERROR'
      const message = fieldText(effects, 'message') || t(`smt.station.feedback.${kind}`)
      notify(kind, message, fieldText(effects, 'voice'))
      if (kind === 'OK') {
        setSlotInput('')
        setReelInput('')
      } else if (kind === 'CONFIRM') {
        setSlotInput(fieldText(effects, 'slot_no') || slotInput)
      }
    },
    [notify, slotInput, t],
  )

  const load = useCallback(
    async (mode: 'load' | 'splice' | 'replace') => {
      const slot = slotInput.trim()
      const reel = reelInput.trim()
      if (!machineCode) {
        notify('ERROR', t('smt.station.noMachine'))
        return
      }
      if (!slot || !reel) return
      setBusy(true)
      try {
        const result = await runERPAction(token, 'MLOD', smtKey('LD'), 'load', {
          machine_code: machineCode,
          slot_code: slot,
          reel_code: reel,
          splice: mode === 'splice',
          replace: mode === 'replace',
        })
        applyResult(result)
        await reloadContext(machineCode)
        await reloadAlerts()
      } catch (err) {
        notify('ERROR', smtErrorText(err))
      } finally {
        setBusy(false)
      }
    },
    [applyResult, machineCode, notify, reloadAlerts, reloadContext, reelInput, slotInput, t, token],
  )

  const runMachineAction = useCallback(
    async (action: 'verify' | 'sign-off' | 'bind', data: Record<string, unknown> = {}) => {
      if (!machineCode) {
        notify('ERROR', t('smt.station.noMachine'))
        return
      }
      setBusy(true)
      try {
        const result = await runERPAction(token, 'MMAC', machineCode, action, data)
        applyResult(result)
        await reloadContext(machineCode)
      } catch (err) {
        notify('ERROR', smtErrorText(err))
      } finally {
        setBusy(false)
      }
    },
    [applyResult, machineCode, notify, reloadContext, t, token],
  )

  const toggleSlotLock = useCallback(
    async (slotNo: string, locked: boolean) => {
      if (!machineCode) return
      setBusy(true)
      try {
        const result = await runERPAction(token, 'MMAC', machineCode, locked ? 'unlock-slot' : 'lock-slot', { slot_no: slotNo })
        applyResult(result)
        await reloadContext(machineCode)
      } catch (err) {
        notify('ERROR', smtErrorText(err))
      } finally {
        setBusy(false)
      }
    },
    [applyResult, machineCode, notify, reloadContext, token],
  )

  const unload = useCallback(async () => {
    const target = unloadTarget
    if (!target || !machineCode) return
    setBusy(true)
    try {
      const result = await runERPAction(token, 'MLOD', target.loadCode, 'unload', {
        reason: unloadReason,
        old_reel_action: unloadDisposition,
        qty_at_unload: unloadQty === '' ? undefined : Number(unloadQty),
        machine_code: machineCode,
      })
      applyResult(result)
      setUnloadTarget(null)
      setUnloadQty('')
      await reloadContext(machineCode)
    } catch (err) {
      notify('ERROR', smtErrorText(err))
    } finally {
      setBusy(false)
    }
  }, [applyResult, machineCode, notify, reloadContext, token, unloadDisposition, unloadQty, unloadReason, unloadTarget])

  const reportProgress = useCallback(async () => {
    const workOrder = context?.work_order
    if (!workOrder) return
    setBusy(true)
    try {
      const result = await runERPAction(token, 'MSWO', workOrder, 'progress', { done_qty: Number(progressQty || 0) })
      applyResult(result)
      await reloadContext(machineCode)
      await reloadAlerts()
    } catch (err) {
      notify('ERROR', smtErrorText(err))
    } finally {
      setBusy(false)
    }
  }, [applyResult, context?.work_order, machineCode, notify, progressQty, reloadAlerts, reloadContext, token])

  const slots = objectList(context?.slots)
  const summary: Record<string, number> = context?.summary ?? {}

  return (
    <div className="space-y-4">
      <SMTFeedbackBanner feedback={feedback} />
      {contextError ? <FeedbackMessage error>{contextError}</FeedbackMessage> : null}
      <Panel
        title={t('smt.tab.station')}
        actions={
          <div className="flex items-center gap-2">
            <label className="text-xs text-slate-500" htmlFor="smt-machine">
              {t('smt.station.machine')}
            </label>
            <select
              id="smt-machine"
              className={`${smtInput} w-56`}
              value={machineCode}
              onChange={(event) => setMachineCode(event.target.value)}
            >
              <option value="">{t('smt.station.machinePlaceholder')}</option>
              {machines.map((machine) => (
                <option key={machine.key} value={machine.key}>
                  {fieldText(machine, 'Name') ? `${fieldText(machine, 'Name')} · ${machine.key}` : machine.key}
                </option>
              ))}
            </select>
          </div>
        }
      >
        {!machineCode ? (
          <p className="text-sm text-slate-500">{t('smt.station.noMachine')}</p>
        ) : (
          <div className="space-y-4">
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
              <div className="rounded-lg border border-slate-800 bg-slate-950/60 px-3 py-2">
                <p className="text-xs uppercase tracking-wide text-slate-500">{t('smt.station.workOrder')}</p>
                <p className="mt-1 text-sm font-semibold text-slate-100">{context?.work_order || t('common.none')}</p>
              </div>
              <div className="rounded-lg border border-slate-800 bg-slate-950/60 px-3 py-2">
                <p className="text-xs uppercase tracking-wide text-slate-500">{t('smt.station.product')}</p>
                <p className="mt-1 text-sm font-semibold text-slate-100">{context?.product_code || t('common.none')}</p>
              </div>
              <div className="rounded-lg border border-slate-800 bg-slate-950/60 px-3 py-2">
                <p className="text-xs uppercase tracking-wide text-slate-500">{t('smt.station.summary')}</p>
                <p className="mt-1 text-sm text-slate-200">
                  {t('smt.station.state.OK')} {summary.ok ?? 0} · {t('smt.station.state.ALT')} {summary.alt ?? 0} ·{' '}
                  {t('smt.station.state.MISMATCH')} {summary.mismatch ?? 0} · {t('smt.station.state.EMPTY')} {summary.empty ?? 0}
                </p>
              </div>
              <div className="rounded-lg border border-slate-800 bg-slate-950/60 px-3 py-2">
                <p className="text-xs uppercase tracking-wide text-slate-500">{t('smt.station.context')}</p>
                <p
                  className={`mt-1 text-sm font-semibold ${
                    context?.ready ? 'text-emerald-300' : context?.awaiting_signoff ? 'text-amber-300' : 'text-slate-300'
                  }`}
                >
                  {context?.ready
                    ? t('smt.station.ready')
                    : context?.awaiting_signoff
                      ? t('smt.station.awaitingSignoff')
                      : t('smt.station.notReady')}
                </p>
              </div>
            </div>

            <div className="flex flex-wrap items-end gap-2">
              <div className="w-44">
                <label className={smtLabel} htmlFor="smt-slot">
                  {t('smt.station.slot')}
                </label>
                <input
                  id="smt-slot"
                  className={smtInput}
                  value={slotInput}
                  autoComplete="off"
                  placeholder={t('smt.station.scanSlot')}
                  onChange={(event) => setSlotInput(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter') void load('load')
                  }}
                />
              </div>
              <div className="w-56">
                <label className={smtLabel} htmlFor="smt-reel">
                  {t('smt.station.reel')}
                </label>
                <input
                  id="smt-reel"
                  className={smtInput}
                  value={reelInput}
                  autoComplete="off"
                  placeholder={t('smt.station.scanReel')}
                  onChange={(event) => setReelInput(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter') void load('load')
                  }}
                />
              </div>
              <button type="button" className={smtPrimaryButton} disabled={busy} onClick={() => void load('load')}>
                <ScanLine className="h-4 w-4" />
                {t('smt.station.load')}
              </button>
              <button type="button" className={smtButton} disabled={busy} onClick={() => void load('splice')}>
                {t('smt.station.splice')}
              </button>
              <button type="button" className={smtButton} disabled={busy} onClick={() => void load('replace')}>
                {t('smt.station.replace')}
              </button>
            </div>

            <div className="flex flex-wrap items-end gap-2 border-t border-slate-800 pt-3">
              <button
                type="button"
                className={smtButton}
                disabled={busy || !context?.work_order}
                onClick={() => void runMachineAction('verify')}
              >
                <ShieldCheck className="h-4 w-4" />
                {t('smt.station.verify')}
              </button>
              <button
                type="button"
                className={smtButton}
                disabled={busy || !context?.awaiting_signoff}
                onClick={() => void runMachineAction('sign-off')}
              >
                <CheckCircle2 className="h-4 w-4" />
                {t('smt.station.signOff')}
              </button>
              <div className="flex items-end gap-2">
                <div className="w-56">
                  <label className={smtLabel} htmlFor="smt-bind">
                    {t('smt.station.bind')}
                  </label>
                  <select id="smt-bind" className={smtInput} value={bindWorkOrder} onChange={(event) => setBindWorkOrder(event.target.value)}>
                    <option value="">{t('smt.station.machinePlaceholder')}</option>
                    {workOrders.map((order) => (
                      <option key={order.key} value={order.key}>
                        {order.key} · {fieldText(order, 'product_code', 'Name')}
                      </option>
                    ))}
                  </select>
                </div>
                <button
                  type="button"
                  className={smtButton}
                  disabled={busy || !bindWorkOrder}
                  onClick={() => void runMachineAction('bind', { work_order: bindWorkOrder })}
                >
                  {t('smt.station.bind')}
                </button>
              </div>
              <div className="flex items-end gap-2">
                <div className="w-32">
                  <label className={smtLabel} htmlFor="smt-progress">
                    {t('smt.station.progressQty')}
                  </label>
                  <input
                    id="smt-progress"
                    className={smtInput}
                    inputMode="numeric"
                    value={progressQty}
                    onChange={(event) => setProgressQty(event.target.value)}
                  />
                </div>
                <button type="button" className={smtButton} disabled={busy || !context?.work_order} onClick={() => void reportProgress()}>
                  {t('smt.station.progress')}
                </button>
              </div>
            </div>
            <p className="text-xs text-slate-500">{t('smt.station.bindHint')}</p>
          </div>
        )}
      </Panel>

      {machineCode ? (
        <Panel
          title={t('smt.station.slots')}
          actions={
            <button type="button" className={smtButton} onClick={() => void reloadContext(machineCode)}>
              <RefreshCw className="h-4 w-4" />
              {t('common.refresh')}
            </button>
          }
        >
          <div className={smtTableWrap}>
            <table className="min-w-full text-left text-sm">
              <thead className="bg-slate-900/60">
                <tr>
                  <th className={smtTh}>{t('smt.station.slot')}</th>
                  <th className={smtTh}>{t('ontology.field.status')}</th>
                  <th className={smtTh}>{t('smt.station.required')}</th>
                  <th className={smtTh}>{t('smt.station.reel')}</th>
                  <th className={smtTh}>{t('ontology.field.item')}</th>
                  <th className={smtTh}>{t('smt.station.qtyEst')}</th>
                  <th className={smtTh}>{t('common.actions')}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800">
                {slots.map((slot, index) => {
                  const state = fieldText(slot, 'state') || 'NA'
                  const main = (slot.main ?? null) as Record<string, unknown> | null
                  const spliced = (slot.spliced ?? null) as Record<string, unknown> | null
                  const slotNo = fieldText(slot, 'slot_no')
                  const locked = boolValue(slot.locked)
                  return (
                    <tr key={`${slotNo}-${index}`} className="bg-[#141519]">
                      <td className={smtTd}>
                        <span className="font-semibold text-slate-100">{slotNo}</span>
                        {locked ? <span className="ml-2 text-xs text-amber-300">{t('smt.station.locked')}</span> : null}
                      </td>
                      <td className={smtTd}>
                        <span
                          className={`inline-flex rounded-md border px-2 py-0.5 text-xs font-semibold ${
                            smtStateStyles[state] ?? smtStateStyles.NA
                          }`}
                        >
                          {t(`smt.station.state.${state}`)}
                        </span>
                      </td>
                      <td className={smtTd}>{fieldText(slot, 'required_part_no') || '—'}</td>
                      <td className={smtTd}>{fieldText(main ?? undefined, 'reel_code') || '—'}</td>
                      <td className={smtTd}>
                        {fieldText(main ?? undefined, 'part_no') || '—'}
                        {main && boolValue(main.is_alternate) ? (
                          <span className="ml-1 text-xs text-sky-300">{t('smt.station.state.ALT')}</span>
                        ) : null}
                        {main && boolValue(main.low) ? (
                          <span className="ml-1 text-xs text-red-300">{t('smt.station.low')}</span>
                        ) : null}
                        {spliced ? <span className="ml-1 text-xs text-slate-500">+{fieldText(spliced, 'reel_code')}</span> : null}
                      </td>
                      <td className={smtTd}>{main ? Math.round(numberValue(main.qty_est)) : '—'}</td>
                      <td className={smtTd}>
                        <div className="flex gap-2">
                          <button
                            type="button"
                            className={smtButton}
                            disabled={busy}
                            onClick={() => void toggleSlotLock(slotNo, locked)}
                          >
                            {locked ? t('smt.station.unlockSlot') : t('smt.station.lockSlot')}
                          </button>
                          {main ? (
                            <button
                              type="button"
                              className={smtButton}
                              disabled={busy}
                              onClick={() => {
                                setUnloadTarget({ slotNo, loadCode: fieldText(main, 'load_code') })
                                setUnloadQty(String(Math.round(numberValue(main.qty_est))))
                              }}
                            >
                              {t('smt.station.unload')}
                            </button>
                          ) : null}
                        </div>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
          {slots.length === 0 ? <p className="mt-2 text-sm text-slate-500">{t('common.noData')}</p> : null}
        </Panel>
      ) : null}

      {unloadTarget ? (
        <Panel
          title={`${t('smt.station.unload')} · ${unloadTarget.slotNo}`}
          actions={
            <button type="button" className={smtButton} onClick={() => setUnloadTarget(null)}>
              <X className="h-4 w-4" />
              {t('common.cancel')}
            </button>
          }
        >
          <div className="grid gap-3 sm:grid-cols-3">
            <div>
              <label className={smtLabel} htmlFor="smt-unload-reason">
                {t('smt.station.unloadReason')}
              </label>
              <input
                id="smt-unload-reason"
                className={smtInput}
                value={unloadReason}
                onChange={(event) => setUnloadReason(event.target.value)}
              />
            </div>
            <div>
              <label className={smtLabel} htmlFor="smt-unload-disposition">
                {t('smt.station.disposition')}
              </label>
              <select
                id="smt-unload-disposition"
                className={smtInput}
                value={unloadDisposition}
                onChange={(event) => setUnloadDisposition(event.target.value)}
              >
                {['RETURN', 'EMPTY', 'KEEP'].map((value) => (
                  <option key={value} value={value}>
                    {t(`smt.station.disposition.${value}`)}
                  </option>
                ))}
              </select>
            </div>
            <div>
              <label className={smtLabel} htmlFor="smt-unload-qty">
                {t('smt.station.remainingQty')}
              </label>
              <input
                id="smt-unload-qty"
                className={smtInput}
                inputMode="numeric"
                value={unloadQty}
                onChange={(event) => setUnloadQty(event.target.value)}
              />
            </div>
          </div>
          <div className="mt-3">
            <button type="button" className={smtPrimaryButton} disabled={busy} onClick={() => void unload()}>
              {t('smt.station.unload')}
            </button>
          </div>
        </Panel>
      ) : null}

      <Panel
        title={t('smt.station.alerts')}
        actions={
          <button type="button" className={smtButton} onClick={() => void reloadAlerts()}>
            <RefreshCw className="h-4 w-4" />
            {t('common.refresh')}
          </button>
        }
      >
        {alerts.length === 0 ? (
          <p className="text-sm text-slate-500">{t('smt.station.noAlerts')}</p>
        ) : (
          <div className={smtTableWrap}>
            <table className="min-w-full text-left text-sm">
              <thead className="bg-slate-900/60">
                <tr>
                  <th className={smtTh}>{t('smt.station.machine')}</th>
                  <th className={smtTh}>{t('smt.station.slot')}</th>
                  <th className={smtTh}>{t('ontology.field.item')}</th>
                  <th className={smtTh}>{t('smt.station.qtyEst')}</th>
                  <th className={smtTh}>{t('ontology.field.boardsLeft')}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800">
                {alerts.map((alert) => (
                  <tr key={alert.key} className="bg-[#141519]">
                    <td className={smtTd}>{fieldText(alert, 'machine_code')}</td>
                    <td className={smtTd}>{fieldText(alert, 'slot_no')}</td>
                    <td className={smtTd}>{fieldText(alert, 'part_no')}</td>
                    <td className={smtTd}>{Math.round(fieldNumber(alert, 'qty_est'))}</td>
                    <td className={smtTd}>{Math.round(fieldNumber(alert, 'boards_left'))}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Panel>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Pass station.
// ---------------------------------------------------------------------------

function SMTUnitsTab({ token }: { token: string }) {
  const { t } = useI18n()
  const { feedback, notify } = useSMTFeedback()
  const [operations, setOperations] = useState<SMTRecord[]>([])
  const [defects, setDefects] = useState<SMTRecord[]>([])
  const [workOrders, setWorkOrders] = useState<SMTRecord[]>([])
  const [machines, setMachines] = useState<SMTRecord[]>([])
  const [sn, setSn] = useState('')
  const [operationCode, setOperationCode] = useState('')
  const [machineCode, setMachineCode] = useState('')
  const [workOrder, setWorkOrder] = useState('')
  const [defectCode, setDefectCode] = useState('')
  const [location, setLocation] = useState('')
  const [busy, setBusy] = useState(false)
  const [history, setHistory] = useState<Record<string, unknown> | null>(null)
  const [historyError, setHistoryError] = useState('')
  const [panelCode, setPanelCode] = useState('')
  const [panelUnits, setPanelUnits] = useState('')
  const [panelWorkOrder, setPanelWorkOrder] = useState('')

  useEffect(() => {
    let cancelled = false
    void Promise.all([
      listERPRecords<SMTRecord>(token, 'MOPR', 200).catch(() => []),
      listERPRecords<SMTRecord>(token, 'MDFC', 200).catch(() => []),
      listERPRecords<SMTRecord>(token, 'MSWO', 200).catch(() => []),
      listERPRecords<SMTRecord>(token, 'MMAC', 200).catch(() => []),
    ]).then(([operationRows, defectRows, orderRows, machineRows]) => {
      if (cancelled) return
      setOperations(operationRows)
      setDefects(defectRows)
      setWorkOrders(orderRows.filter((row) => fieldText(row, 'Status', 'status') !== 'CLOSED'))
      setMachines(machineRows)
    })
    return () => {
      cancelled = true
    }
  }, [token])

  const loadHistory = useCallback(
    async (value: string) => {
      if (!value) return
      try {
        const result = await getSMTTraceSN(token, value)
        setHistory(result)
        setHistoryError('')
      } catch (err) {
        setHistory(null)
        setHistoryError(smtErrorText(err))
      }
    },
    [token],
  )

  const runUnitAction = useCallback(
    async (action: 'pass' | 'fail', value: string) => {
      const target = value.trim()
      if (!target) return
      setBusy(true)
      try {
        const data: Record<string, unknown> = {
          operation_code: operationCode,
          machine_code: machineCode,
          work_order: workOrder,
        }
        if (action === 'fail') {
          data.defects = [
            { defect_code: defectCode, location, part_no: '' },
          ]
        }
        const result = await runERPAction(token, 'MUNT', target, action, data)
        const effects: Record<string, unknown> = result.effects ?? {}
        const status = result.status.toUpperCase()
        const kind: SMTFeedbackKind = status === 'OK' ? 'OK' : status === 'CONFIRM' ? 'CONFIRM' : 'ERROR'
        const message = fieldText(effects, 'message') || t(`smt.station.feedback.${kind}`)
        notify(kind, message, fieldText(effects, 'voice'))
        if (kind === 'OK') {
          setSn('')
          setLocation('')
        }
        await loadHistory(target)
      } catch (err) {
        notify('ERROR', smtErrorText(err))
      } finally {
        setBusy(false)
      }
    },
    [defectCode, loadHistory, location, machineCode, notify, operationCode, t, token, workOrder],
  )

  const bindPanel = useCallback(async () => {
    const panel = panelCode.trim()
    if (!panel) return
    setBusy(true)
    try {
      const sns = panelUnits
        .split(/\r?\n|,/)
        .map((value) => value.trim())
        .filter(Boolean)
      const result = await runERPAction(token, 'MPNL', panel, 'bind', { work_order: panelWorkOrder, sns })
      const status = result.status.toUpperCase()
      notify(status === 'OK' || status === '' ? 'OK' : 'ERROR', t('common.save'))
    } catch (err) {
      notify('ERROR', smtErrorText(err))
    } finally {
      setBusy(false)
    }
  }, [notify, panelCode, panelUnits, panelWorkOrder, t, token])

  const splitPanel = useCallback(async () => {
    const panel = panelCode.trim()
    if (!panel) return
    setBusy(true)
    try {
      await runERPAction(token, 'MPNL', panel, 'split', {})
      notify('OK', t('common.save'))
      setPanelCode('')
      setPanelUnits('')
    } catch (err) {
      notify('ERROR', smtErrorText(err))
    } finally {
      setBusy(false)
    }
  }, [notify, panelCode, t, token])

  const historyEvents = objectList(history?.events ?? history?.smt_events)
  const unitInfo = (history?.unit ?? null) as Record<string, unknown> | null

  return (
    <div className="space-y-4">
      <SMTFeedbackBanner feedback={feedback} />
      {historyError ? <FeedbackMessage error>{historyError}</FeedbackMessage> : null}
      <Panel title={t('smt.tab.units')}>
        <div className="grid gap-3 lg:grid-cols-3">
          <div className="w-full">
            <label className={smtLabel} htmlFor="smt-sn">
              {t('smt.units.scanSn')}
            </label>
            <input
              id="smt-sn"
              className={smtInput}
              value={sn}
              autoComplete="off"
              onChange={(event) => setSn(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === 'Enter') {
                  event.preventDefault()
                  void runUnitAction('pass', sn)
                }
              }}
            />
          </div>
          <div>
            <label className={smtLabel} htmlFor="smt-operation">
              {t('smt.units.operation')}
            </label>
            <select id="smt-operation" className={smtInput} value={operationCode} onChange={(event) => setOperationCode(event.target.value)}>
              <option value="">{t('smt.units.operation')}</option>
              {operations.map((operation) => (
                <option key={operation.key} value={operation.key}>
                  {operation.key} · {fieldText(operation, 'Name')}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label className={smtLabel} htmlFor="smt-unit-machine">
              {t('smt.units.machine')}
            </label>
            <select id="smt-unit-machine" className={smtInput} value={machineCode} onChange={(event) => setMachineCode(event.target.value)}>
              <option value="">{t('smt.units.machine')}</option>
              {machines.map((machine) => (
                <option key={machine.key} value={machine.key}>
                  {fieldText(machine, 'Name') ? `${fieldText(machine, 'Name')} · ${machine.key}` : machine.key}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label className={smtLabel} htmlFor="smt-unit-wo">
              {t('smt.units.workOrder')}
            </label>
            <select id="smt-unit-wo" className={smtInput} value={workOrder} onChange={(event) => setWorkOrder(event.target.value)}>
              <option value="">{t('smt.units.workOrder')}</option>
              {workOrders.map((order) => (
                <option key={order.key} value={order.key}>
                  {order.key} · {fieldText(order, 'product_code')}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label className={smtLabel} htmlFor="smt-defect">
              {t('smt.units.defect')}
            </label>
            <select id="smt-defect" className={smtInput} value={defectCode} onChange={(event) => setDefectCode(event.target.value)}>
              <option value="">{t('smt.units.defect')}</option>
              {defects.map((defect) => (
                <option key={defect.key} value={defect.key}>
                  {defect.key} · {fieldText(defect, 'Name')}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label className={smtLabel} htmlFor="smt-location">
              {t('smt.units.location')}
            </label>
            <input id="smt-location" className={smtInput} value={location} onChange={(event) => setLocation(event.target.value)} />
          </div>
        </div>
        <div className="mt-3 flex flex-wrap gap-2">
          <button type="button" className={smtPrimaryButton} disabled={busy || !sn.trim()} onClick={() => void runUnitAction('pass', sn)}>
            <CheckCircle2 className="h-4 w-4" />
            {t('smt.units.pass')}
          </button>
          <button type="button" className={smtDangerButton} disabled={busy || !sn.trim()} onClick={() => void runUnitAction('fail', sn)}>
            <AlertTriangle className="h-4 w-4" />
            {t('smt.units.fail')}
          </button>
          <button type="button" className={smtButton} disabled={!sn.trim()} onClick={() => void loadHistory(sn.trim())}>
            <History className="h-4 w-4" />
            {t('smt.units.history')}
          </button>
        </div>
      </Panel>

      <Panel title={t('smt.units.bindPanel')}>
        <div className="grid gap-3 sm:grid-cols-2">
          <div>
            <label className={smtLabel} htmlFor="smt-panel-code">
              {t('smt.units.panelCode')}
            </label>
            <input id="smt-panel-code" className={smtInput} value={panelCode} onChange={(event) => setPanelCode(event.target.value)} />
          </div>
          <div>
            <label className={smtLabel} htmlFor="smt-panel-wo">
              {t('smt.units.workOrder')}
            </label>
            <select id="smt-panel-wo" className={smtInput} value={panelWorkOrder} onChange={(event) => setPanelWorkOrder(event.target.value)}>
              <option value="">{t('smt.units.workOrder')}</option>
              {workOrders.map((order) => (
                <option key={order.key} value={order.key}>
                  {order.key} · {fieldText(order, 'product_code')}
                </option>
              ))}
            </select>
          </div>
        </div>
        <div className="mt-3">
          <label className={smtLabel} htmlFor="smt-panel-units">
            {t('smt.units.panelUnits')}
          </label>
          <textarea
            id="smt-panel-units"
            className="min-h-24 w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-100 outline-none focus:border-[#DF6A24]"
            value={panelUnits}
            onChange={(event) => setPanelUnits(event.target.value)}
          />
        </div>
        <div className="mt-3 flex gap-2">
          <button type="button" className={smtButton} disabled={busy || !panelCode.trim()} onClick={() => void bindPanel()}>
            {t('smt.units.bindPanel')}
          </button>
          <button type="button" className={smtButton} disabled={busy || !panelCode.trim()} onClick={() => void splitPanel()}>
            {t('smt.units.splitPanel')}
          </button>
        </div>
      </Panel>

      <Panel title={t('smt.units.history')}>
        {!history ? (
          <p className="text-sm text-slate-500">{t('smt.units.noHistory')}</p>
        ) : (
          <div className="space-y-3">
            {unitInfo ? (
              <div className="grid gap-2 sm:grid-cols-4">
                {[
                  ['status', fieldText(unitInfo, 'status', 'Status')],
                  ['workOrder', fieldText(unitInfo, 'work_order')],
                  ['product', fieldText(unitInfo, 'product_code')],
                  ['lastSeq', String(Math.round(fieldNumber(unitInfo, 'last_seq')))],
                ].map(([label, value]) => (
                  <div key={label} className="rounded-lg border border-slate-800 bg-slate-950/60 px-3 py-2">
                    <p className="text-xs uppercase tracking-wide text-slate-500">{t(`ontology.field.${label}`)}</p>
                    <p className="mt-1 text-sm font-semibold text-slate-100">{value || '—'}</p>
                  </div>
                ))}
              </div>
            ) : null}
            {historyEvents.length === 0 ? (
              <p className="text-sm text-slate-500">{t('smt.units.noHistory')}</p>
            ) : (
              <div className={smtTableWrap}>
                <table className="min-w-full text-left text-sm">
                  <thead className="bg-slate-900/60">
                    <tr>
                      <th className={smtTh}>{t('ontology.field.seq')}</th>
                      <th className={smtTh}>{t('ontology.field.operation')}</th>
                      <th className={smtTh}>{t('ontology.field.result')}</th>
                      <th className={smtTh}>{t('ontology.field.location')}</th>
                      <th className={smtTh}>{t('ontology.field.machine')}</th>
                      <th className={smtTh}>{t('ontology.field.createdAt')}</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-800">
                    {historyEvents.map((event, index) => (
                      <tr key={`${fieldText(event, 'seq')}-${index}`} className="bg-[#141519]">
                        <td className={smtTd}>{Math.round(fieldNumber(event, 'seq'))}</td>
                        <td className={smtTd}>{fieldText(event, 'operation_code')}</td>
                        <td className={smtTd}>{fieldText(event, 'result')}</td>
                        <td className={smtTd}>{fieldText(event, 'location')}</td>
                        <td className={smtTd}>{fieldText(event, 'machine_code')}</td>
                        <td className={smtTd}>{fieldText(event, 'created_at')}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        )}
      </Panel>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Quality: repair station and FAI / IPQC inspection.
// ---------------------------------------------------------------------------

function SMTQualityTab({ token }: { token: string }) {
  const { t } = useI18n()
  const { feedback, notify } = useSMTFeedback()
  const [busy, setBusy] = useState(false)
  const [repairAction, setRepairAction] = useState('REPAIRED')
  const [repairRemark, setRepairRemark] = useState('')
  const [rootCause, setRootCause] = useState('')
  const [inspType, setInspType] = useState('FAI')
  const [inspWorkOrder, setInspWorkOrder] = useState('')
  const [inspSample, setInspSample] = useState('')
  const [inspResult, setInspResult] = useState('PASS')
  const [workOrders, setWorkOrders] = useState<SMTRecord[]>([])
  const [version, setVersion] = useState(0)

  const defects = useSMTResource(() => listERPRecords<SMTRecord>(token, 'MUDF', 200), [token, version])
  const inspections = useSMTResource(() => listERPRecords<SMTRecord>(token, 'MINS', 50), [token, version])

  useEffect(() => {
    let cancelled = false
    void listERPRecords<SMTRecord>(token, 'MSWO', 200)
      .then((rows) => {
        if (!cancelled) setWorkOrders(rows.filter((row) => fieldText(row, 'Status', 'status') !== 'CLOSED'))
      })
      .catch(() => undefined)
    return () => {
      cancelled = true
    }
  }, [token])

  const openDefects = useMemo(
    () => (defects.data ?? []).filter((row) => fieldText(row, 'status', 'Status').toUpperCase() === 'OPEN'),
    [defects.data],
  )

  const repairUnit = useCallback(
    async (sn: string, defectKey: string, action: string) => {
      setBusy(true)
      try {
        const result = await runERPAction(token, 'MUNT', sn, 'repair', {
          action,
          defect_key: defectKey,
          repair_action: repairRemark,
          root_cause: rootCause,
        })
        const status = result.status.toUpperCase()
        const effects: Record<string, unknown> = result.effects ?? {}
        notify(status === 'ERROR' ? 'ERROR' : 'OK', fieldText(effects, 'message') || t('common.save'), fieldText(effects, 'voice'))
        setRepairRemark('')
        setRootCause('')
        setVersion((value) => value + 1)
      } catch (err) {
        notify('ERROR', smtErrorText(err))
      } finally {
        setBusy(false)
      }
    },
    [notify, repairRemark, rootCause, t, token],
  )

  const recordInspection = useCallback(async () => {
    if (!inspWorkOrder) return
    setBusy(true)
    try {
      await runERPAction(token, 'MINS', smtKey('INS'), 'record', {
        insp_type: inspType,
        work_order: inspWorkOrder,
        sample_sn: inspSample,
        result: inspResult,
      })
      notify('OK', t('smt.insp.recorded'))
      setInspSample('')
      setVersion((value) => value + 1)
    } catch (err) {
      notify('ERROR', smtErrorText(err))
    } finally {
      setBusy(false)
    }
  }, [inspResult, inspSample, inspType, inspWorkOrder, notify, t, token])

  return (
    <div className="space-y-4">
      <SMTFeedbackBanner feedback={feedback} />
      <Panel title={t('smt.repair.open')}>
        {openDefects.length === 0 ? (
          <p className="text-sm text-slate-500">{t('smt.repair.empty')}</p>
        ) : (
          <div className={smtTableWrap}>
            <table className="min-w-full text-left text-sm">
              <thead className="bg-slate-900/60">
                <tr>
                  <th className={smtTh}>{t('ontology.field.sn')}</th>
                  <th className={smtTh}>{t('ontology.field.defectCode')}</th>
                  <th className={smtTh}>{t('ontology.field.location')}</th>
                  <th className={smtTh}>{t('smt.repair.step')}</th>
                  <th className={smtTh}>{t('common.actions')}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800">
                {openDefects.map((defect) => (
                  <tr key={defect.key} className="bg-[#141519]">
                    <td className={smtTd}>{fieldText(defect, 'sn')}</td>
                    <td className={smtTd}>{fieldText(defect, 'defect_code')}</td>
                    <td className={smtTd}>{fieldText(defect, 'location')}</td>
                    <td className={smtTd}>{fieldText(defect, 'operation_code', 'step')}</td>
                    <td className={smtTd}>
                      <div className="flex flex-wrap gap-2">
                        <button
                          type="button"
                          className={smtPrimaryButton}
                          disabled={busy}
                          onClick={() => void repairUnit(fieldText(defect, 'sn'), defect.key, repairAction)}
                        >
                          {t('smt.repair.repair')}
                        </button>
                        <button
                          type="button"
                          className={smtButton}
                          disabled={busy}
                          onClick={() => void repairUnit(fieldText(defect, 'sn'), defect.key, 'MISJUDGE')}
                        >
                          {t('smt.repair.misjudge')}
                        </button>
                        <button
                          type="button"
                          className={smtDangerButton}
                          disabled={busy}
                          onClick={() => void repairUnit(fieldText(defect, 'sn'), defect.key, 'SCRAPPED')}
                        >
                          {t('smt.repair.scrap')}
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        <div className="mt-3 grid gap-3 sm:grid-cols-3">
          <div>
            <label className={smtLabel} htmlFor="smt-repair-action">
              {t('smt.repair.action')}
            </label>
            <input id="smt-repair-action" className={smtInput} value={repairRemark} onChange={(event) => setRepairRemark(event.target.value)} />
          </div>
          <div>
            <label className={smtLabel} htmlFor="smt-repair-cause">
              {t('smt.repair.rootCause')}
            </label>
            <input id="smt-repair-cause" className={smtInput} value={rootCause} onChange={(event) => setRootCause(event.target.value)} />
          </div>
          <div>
            <label className={smtLabel} htmlFor="smt-repair-mode">
              {t('smt.repair.repair')}
            </label>
            <select id="smt-repair-mode" className={smtInput} value={repairAction} onChange={(event) => setRepairAction(event.target.value)}>
              {['REPAIRED', 'MISJUDGE', 'SCRAPPED'].map((value) => (
                <option key={value} value={value}>
                  {t(`smt.repair.status.${value}`)}
                </option>
              ))}
            </select>
          </div>
        </div>
      </Panel>

      <Panel title={t('smt.insp.title')}>
        <div className="grid gap-3 sm:grid-cols-4">
          <div>
            <label className={smtLabel} htmlFor="smt-insp-type">
              {t('smt.insp.inspType')}
            </label>
            <select id="smt-insp-type" className={smtInput} value={inspType} onChange={(event) => setInspType(event.target.value)}>
              {['FAI', 'IPQC'].map((value) => (
                <option key={value} value={value}>
                  {t(`smt.inspType.${value}`)}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label className={smtLabel} htmlFor="smt-insp-wo">
              {t('smt.units.workOrder')}
            </label>
            <select id="smt-insp-wo" className={smtInput} value={inspWorkOrder} onChange={(event) => setInspWorkOrder(event.target.value)}>
              <option value="">{t('smt.units.workOrder')}</option>
              {workOrders.map((order) => (
                <option key={order.key} value={order.key}>
                  {order.key} · {fieldText(order, 'product_code')}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label className={smtLabel} htmlFor="smt-insp-sample">
              {t('smt.insp.sampleSn')}
            </label>
            <input id="smt-insp-sample" className={smtInput} value={inspSample} onChange={(event) => setInspSample(event.target.value)} />
          </div>
          <div>
            <label className={smtLabel} htmlFor="smt-insp-result">
              {t('smt.insp.result')}
            </label>
            <select id="smt-insp-result" className={smtInput} value={inspResult} onChange={(event) => setInspResult(event.target.value)}>
              {['PASS', 'FAIL'].map((value) => (
                <option key={value} value={value}>
                  {t(`smt.insp.result.${value}`)}
                </option>
              ))}
            </select>
          </div>
        </div>
        <div className="mt-3">
          <button type="button" className={smtPrimaryButton} disabled={busy || !inspWorkOrder} onClick={() => void recordInspection()}>
            <ClipboardCheck className="h-4 w-4" />
            {t('smt.insp.record')}
          </button>
        </div>
        <div className="mt-4">
          <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-slate-500">{t('smt.insp.recent')}</p>
          {(inspections.data ?? []).length === 0 ? (
            <p className="text-sm text-slate-500">{t('common.noData')}</p>
          ) : (
            <div className={smtTableWrap}>
              <table className="min-w-full text-left text-sm">
                <thead className="bg-slate-900/60">
                  <tr>
                    <th className={smtTh}>{t('erp.document.smtInspection')}</th>
                    <th className={smtTh}>{t('ontology.field.inspType')}</th>
                    <th className={smtTh}>{t('smt.units.workOrder')}</th>
                    <th className={smtTh}>{t('smt.insp.sampleSn')}</th>
                    <th className={smtTh}>{t('ontology.field.result')}</th>
                    <th className={smtTh}>{t('ontology.field.createdAt')}</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800">
                  {(inspections.data ?? []).map((row) => (
                    <tr key={row.key} className="bg-[#141519]">
                      <td className={smtTd}>{row.key}</td>
                      <td className={smtTd}>{t(`smt.inspType.${fieldText(row, 'insp_type', 'type')}`)}</td>
                      <td className={smtTd}>{fieldText(row, 'work_order')}</td>
                      <td className={smtTd}>{fieldText(row, 'sample_sn')}</td>
                      <td className={smtTd}>{fieldText(row, 'result')}</td>
                      <td className={smtTd}>{fieldText(row, 'created_at')}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </Panel>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Andon.
// ---------------------------------------------------------------------------

function SMTAndonTab({ token }: { token: string }) {
  const { t } = useI18n()
  const { feedback, notify } = useSMTFeedback()
  const [busy, setBusy] = useState(false)
  const [version, setVersion] = useState(0)
  const [callType, setCallType] = useState('MATERIAL')
  const [machineCode, setMachineCode] = useState('')
  const [lineCode, setLineCode] = useState('')
  const [workOrder, setWorkOrder] = useState('')
  const [description, setDescription] = useState('')
  const [cause, setCause] = useState('')
  const [solution, setSolution] = useState('')
  const [machines, setMachines] = useState<SMTRecord[]>([])
  const [lines, setLines] = useState<SMTRecord[]>([])
  const [workOrders, setWorkOrders] = useState<SMTRecord[]>([])
  const calls = useSMTResource(() => listERPRecords<SMTRecord>(token, 'MAND', 200), [token, version])

  useEffect(() => {
    let cancelled = false
    void Promise.all([
      listERPRecords<SMTRecord>(token, 'MMAC', 200).catch(() => []),
      listERPRecords<SMTRecord>(token, 'MLIN', 100).catch(() => []),
      listERPRecords<SMTRecord>(token, 'MSWO', 200).catch(() => []),
    ]).then(([machineRows, lineRows, orderRows]) => {
      if (cancelled) return
      setMachines(machineRows)
      setLines(lineRows)
      setWorkOrders(orderRows.filter((row) => fieldText(row, 'Status', 'status') !== 'CLOSED'))
    })
    return () => {
      cancelled = true
    }
  }, [token])

  const openCalls = useMemo(
    () => (calls.data ?? []).filter((row) => fieldText(row, 'status', 'Status').toUpperCase() !== 'CLOSED'),
    [calls.data],
  )

  const raise = useCallback(async () => {
    setBusy(true)
    try {
      await runERPAction(token, 'MAND', smtKey('AC'), 'raise', {
        call_type: callType,
        machine_code: machineCode,
        line_code: lineCode,
        work_order: workOrder,
        description,
      })
      notify('OK', t('smt.andon.raised'))
      setDescription('')
      setVersion((value) => value + 1)
    } catch (err) {
      notify('ERROR', smtErrorText(err))
    } finally {
      setBusy(false)
    }
  }, [callType, description, lineCode, machineCode, notify, t, token, workOrder])

  const transition = useCallback(
    async (callNo: string, action: 'ack' | 'close') => {
      setBusy(true)
      try {
        await runERPAction(token, 'MAND', callNo, action, action === 'close' ? { cause, solution } : {})
        notify('OK', t('common.save'))
        setCause('')
        setSolution('')
        setVersion((value) => value + 1)
      } catch (err) {
        notify('ERROR', smtErrorText(err))
      } finally {
        setBusy(false)
      }
    },
    [cause, notify, solution, t, token],
  )

  return (
    <div className="space-y-4">
      <SMTFeedbackBanner feedback={feedback} />
      <Panel title={t('smt.andon.raise')}>
        <div className="grid gap-3 sm:grid-cols-3 lg:grid-cols-5">
          <div>
            <label className={smtLabel} htmlFor="smt-andon-type">
              {t('smt.andon.callType')}
            </label>
            <select id="smt-andon-type" className={smtInput} value={callType} onChange={(event) => setCallType(event.target.value)}>
              {['MATERIAL', 'EQUIPMENT', 'QUALITY', 'PROCESS', 'OTHER'].map((value) => (
                <option key={value} value={value}>
                  {t(`smt.andonType.${value}`)}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label className={smtLabel} htmlFor="smt-andon-machine">
              {t('smt.station.machine')}
            </label>
            <select id="smt-andon-machine" className={smtInput} value={machineCode} onChange={(event) => setMachineCode(event.target.value)}>
              <option value="">{t('smt.station.machinePlaceholder')}</option>
              {machines.map((machine) => (
                <option key={machine.key} value={machine.key}>
                  {fieldText(machine, 'Name') ? `${fieldText(machine, 'Name')} · ${machine.key}` : machine.key}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label className={smtLabel} htmlFor="smt-andon-line">
              {t('smt.board.line')}
            </label>
            <select id="smt-andon-line" className={smtInput} value={lineCode} onChange={(event) => setLineCode(event.target.value)}>
              <option value="">{t('smt.board.allLines')}</option>
              {lines.map((line) => (
                <option key={line.key} value={line.key}>
                  {fieldText(line, 'Name') ? `${fieldText(line, 'Name')} · ${line.key}` : line.key}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label className={smtLabel} htmlFor="smt-andon-wo">
              {t('smt.units.workOrder')}
            </label>
            <select id="smt-andon-wo" className={smtInput} value={workOrder} onChange={(event) => setWorkOrder(event.target.value)}>
              <option value="">{t('smt.units.workOrder')}</option>
              {workOrders.map((order) => (
                <option key={order.key} value={order.key}>
                  {order.key} · {fieldText(order, 'product_code')}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label className={smtLabel} htmlFor="smt-andon-desc">
              {t('smt.andon.description')}
            </label>
            <input id="smt-andon-desc" className={smtInput} value={description} onChange={(event) => setDescription(event.target.value)} />
          </div>
        </div>
        <div className="mt-3 flex flex-wrap items-end gap-3">
          <button type="button" className={smtPrimaryButton} disabled={busy} onClick={() => void raise()}>
            <Bell className="h-4 w-4" />
            {t('smt.andon.raise')}
          </button>
          <div className="w-56">
            <label className={smtLabel} htmlFor="smt-andon-cause">
              {t('smt.andon.cause')}
            </label>
            <input id="smt-andon-cause" className={smtInput} value={cause} onChange={(event) => setCause(event.target.value)} />
          </div>
          <div className="w-56">
            <label className={smtLabel} htmlFor="smt-andon-solution">
              {t('smt.andon.solution')}
            </label>
            <input id="smt-andon-solution" className={smtInput} value={solution} onChange={(event) => setSolution(event.target.value)} />
          </div>
        </div>
      </Panel>

      <Panel
        title={t('smt.andon.open')}
        actions={
          <button type="button" className={smtButton} onClick={() => setVersion((value) => value + 1)}>
            <RefreshCw className="h-4 w-4" />
            {t('common.refresh')}
          </button>
        }
      >
        {openCalls.length === 0 ? (
          <p className="text-sm text-slate-500">{t('smt.andon.empty')}</p>
        ) : (
          <div className={smtTableWrap}>
            <table className="min-w-full text-left text-sm">
              <thead className="bg-slate-900/60">
                <tr>
                  <th className={smtTh}>{t('erp.document.smtAndonCall')}</th>
                  <th className={smtTh}>{t('smt.andon.callType')}</th>
                  <th className={smtTh}>{t('smt.station.machine')}</th>
                  <th className={smtTh}>{t('ontology.field.status')}</th>
                  <th className={smtTh}>{t('smt.andon.description')}</th>
                  <th className={smtTh}>{t('common.actions')}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800">
                {openCalls.map((call) => {
                  const status = fieldText(call, 'status', 'Status').toUpperCase()
                  return (
                    <tr key={call.key} className="bg-[#141519]">
                      <td className={smtTd}>{call.key}</td>
                      <td className={smtTd}>{t(`smt.andonType.${fieldText(call, 'call_type')}`)}</td>
                      <td className={smtTd}>{fieldText(call, 'machine_code')}</td>
                      <td className={smtTd}>{t(`smt.andon.status.${status}`)}</td>
                      <td className={smtTd}>{fieldText(call, 'description')}</td>
                      <td className={smtTd}>
                        <div className="flex gap-2">
                          <button
                            type="button"
                            className={smtButton}
                            disabled={busy || status !== 'OPEN'}
                            onClick={() => void transition(call.key, 'ack')}
                          >
                            {t('smt.andon.ack')}
                          </button>
                          <button type="button" className={smtButton} disabled={busy} onClick={() => void transition(call.key, 'close')}>
                            {t('smt.andon.close')}
                          </button>
                        </div>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
      </Panel>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Line board.
// ---------------------------------------------------------------------------

function SMTBoardTab({ token }: { token: string }) {
  const { t } = useI18n()
  const [lineCode, setLineCode] = useState('')
  const [autoRefresh, setAutoRefresh] = useState(true)
  const [version, setVersion] = useState(0)
  const [progressOrder, setProgressOrder] = useState('')
  const board = useSMTResource(() => getSMTBoard(token, lineCode), [token, lineCode, version])
  const lines = useSMTResource(() => listERPRecords<SMTRecord>(token, 'MLIN', 100), [token])
  const progress = useSMTResource(
    () => (progressOrder ? getSMTWorkOrderProgress(token, progressOrder) : Promise.resolve(null)),
    [token, progressOrder, version],
  )

  useEffect(() => {
    if (!autoRefresh) return
    const timer = window.setInterval(() => setVersion((value) => value + 1), 15000)
    return () => window.clearInterval(timer)
  }, [autoRefresh])

  const machines = objectList(board.data?.machines)
  const orders = objectList(board.data?.work_orders)
  const andons = objectList(board.data?.open_andons)

  return (
    <div className="space-y-4">
      {board.error ? <FeedbackMessage error>{board.error}</FeedbackMessage> : null}
      <Panel
        title={t('smt.tab.board')}
        actions={
          <div className="flex flex-wrap items-center gap-3">
            <label className="text-xs text-slate-500" htmlFor="smt-board-line">
              {t('smt.board.line')}
            </label>
            <select id="smt-board-line" className={`${smtInput} w-48`} value={lineCode} onChange={(event) => setLineCode(event.target.value)}>
              <option value="">{t('smt.board.allLines')}</option>
              {(lines.data ?? []).map((line) => (
                <option key={line.key} value={line.key}>
                  {fieldText(line, 'Name') ? `${fieldText(line, 'Name')} · ${line.key}` : line.key}
                </option>
              ))}
            </select>
            <label className="inline-flex items-center gap-2 text-xs text-slate-400">
              <input type="checkbox" checked={autoRefresh} onChange={(event) => setAutoRefresh(event.target.checked)} />
              {t('smt.board.autoRefresh')}
            </label>
            <button type="button" className={smtButton} onClick={() => setVersion((value) => value + 1)}>
              <RefreshCw className={`h-4 w-4 ${board.loading ? 'animate-spin' : ''}`} />
              {t('common.refresh')}
            </button>
          </div>
        }
      >
        <div className="grid gap-3 sm:grid-cols-3">
          <div className="rounded-lg border border-slate-800 bg-slate-950/60 px-3 py-2">
            <p className="text-xs uppercase tracking-wide text-slate-500">{t('smt.board.readyMachines')}</p>
            <p className="mt-1 text-lg font-semibold text-emerald-300">
              {String(board.data?.ready_machines ?? 0)} / {String(board.data?.machine_count ?? 0)}
            </p>
          </div>
          <div className="rounded-lg border border-slate-800 bg-slate-950/60 px-3 py-2">
            <p className="text-xs uppercase tracking-wide text-slate-500">{t('smt.board.workOrders')}</p>
            <p className="mt-1 text-lg font-semibold text-slate-100">{orders.length}</p>
          </div>
          <div className="rounded-lg border border-slate-800 bg-slate-950/60 px-3 py-2">
            <p className="text-xs uppercase tracking-wide text-slate-500">{t('smt.board.openAndons')}</p>
            <p className="mt-1 text-lg font-semibold text-amber-300">{andons.length}</p>
          </div>
        </div>
        <p className="mt-2 text-xs text-slate-500">
          {t('smt.board.generatedAt')}: {fieldText(board.data ?? undefined, 'generated_at') || '—'}
        </p>
      </Panel>

      <Panel title={t('smt.board.machines')}>
        {machines.length === 0 ? (
          <p className="text-sm text-slate-500">{t('common.noData')}</p>
        ) : (
          <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
            {machines.map((machine) => {
              const summary = (machine.summary ?? {}) as Record<string, unknown>
              const ready = boolValue(machine.ready)
              return (
                <div
                  key={fieldText(machine, 'machine_code')}
                  className={`rounded-lg border px-3 py-3 ${
                    ready ? 'border-emerald-800 bg-emerald-950/30' : 'border-slate-800 bg-slate-950/60'
                  }`}
                >
                  <div className="flex items-center justify-between gap-2">
                    <p className="text-sm font-semibold text-slate-100">
                      {fieldText(machine, 'name') || fieldText(machine, 'machine_code')}
                    </p>
                    <span className={`text-xs font-semibold ${ready ? 'text-emerald-300' : 'text-slate-400'}`}>
                      {ready ? t('smt.station.ready') : t('smt.station.notReady')}
                    </span>
                  </div>
                  <p className="mt-1 text-xs text-slate-400">
                    {fieldText(machine, 'machine_code')} · {t('smt.station.workOrder')} {fieldText(machine, 'work_order') || '—'}
                  </p>
                  <p className="mt-2 text-xs text-slate-400">
                    {t('smt.station.state.OK')} {Math.round(numberValue(summary.ok))} · {t('smt.station.state.ALT')}{' '}
                    {Math.round(numberValue(summary.alt))} · {t('smt.station.state.MISMATCH')} {Math.round(numberValue(summary.mismatch))} ·{' '}
                    {t('smt.station.state.EMPTY')} {Math.round(numberValue(summary.empty))}
                  </p>
                </div>
              )
            })}
          </div>
        )}
      </Panel>

      <div className="grid gap-4 xl:grid-cols-2">
        <Panel title={t('smt.board.workOrders')}>
          {orders.length === 0 ? (
            <p className="text-sm text-slate-500">{t('common.noData')}</p>
          ) : (
            <div className={smtTableWrap}>
              <table className="min-w-full text-left text-sm">
                <thead className="bg-slate-900/60">
                  <tr>
                    <th className={smtTh}>{t('smt.units.workOrder')}</th>
                    <th className={smtTh}>{t('ontology.field.product')}</th>
                    <th className={smtTh}>{t('smt.board.plan')}</th>
                    <th className={smtTh}>{t('smt.board.done')}</th>
                    <th className={smtTh}>{t('smt.board.achievement')}</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800">
                  {orders.map((order) => (
                    <tr
                      key={fieldText(order, 'work_order')}
                      className={`cursor-pointer ${progressOrder === fieldText(order, 'work_order') ? 'bg-[#1d1e24]' : 'bg-[#141519]'}`}
                      onClick={() => setProgressOrder(fieldText(order, 'work_order'))}
                    >
                      <td className={smtTd}>{fieldText(order, 'work_order')}</td>
                      <td className={smtTd}>{fieldText(order, 'product_code')}</td>
                      <td className={smtTd}>{Math.round(numberValue(order.plan_qty))}</td>
                      <td className={smtTd}>{Math.round(numberValue(order.done_qty))}</td>
                      <td className={smtTd}>{percent(numberValue(order.achievement))}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Panel>
        <Panel title={t('smt.board.openAndons')}>
          {andons.length === 0 ? (
            <p className="text-sm text-slate-500">{t('smt.andon.empty')}</p>
          ) : (
            <div className={smtTableWrap}>
              <table className="min-w-full text-left text-sm">
                <thead className="bg-slate-900/60">
                  <tr>
                    <th className={smtTh}>{t('erp.document.smtAndonCall')}</th>
                    <th className={smtTh}>{t('smt.andon.callType')}</th>
                    <th className={smtTh}>{t('smt.station.machine')}</th>
                    <th className={smtTh}>{t('ontology.field.status')}</th>
                    <th className={smtTh}>{t('smt.andon.description')}</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800">
                  {andons.map((call) => (
                    <tr key={fieldText(call, 'call_no')} className="bg-[#141519]">
                      <td className={smtTd}>{fieldText(call, 'call_no')}</td>
                      <td className={smtTd}>{t(`smt.andonType.${fieldText(call, 'call_type')}`)}</td>
                      <td className={smtTd}>{fieldText(call, 'machine_code')}</td>
                      <td className={smtTd}>{t(`smt.andon.status.${fieldText(call, 'status')}`)}</td>
                      <td className={smtTd}>{fieldText(call, 'description')}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Panel>
      </div>

      {progressOrder ? (
        <Panel
          title={`${t('smt.units.history')} · ${progressOrder}`}
          actions={
            <button type="button" className={smtButton} onClick={() => setProgressOrder('')}>
              <X className="h-4 w-4" />
              {t('common.close')}
            </button>
          }
        >
          {progress.error ? <FeedbackMessage error>{progress.error}</FeedbackMessage> : null}
          {progress.data ? (
            <div className="space-y-3">
              <div className="grid gap-3 sm:grid-cols-3 lg:grid-cols-5">
                {[
                  [t('ontology.field.product'), fieldText(progress.data, 'product_code')],
                  [t('ontology.field.status'), fieldText(progress.data, 'status')],
                  [t('smt.board.plan'), String(Math.round(numberValue(progress.data.plan_qty)))],
                  [t('smt.board.done'), String(Math.round(numberValue(progress.data.done_qty)))],
                  [t('smt.reports.rty'), percent(numberValue(progress.data.rty))],
                ].map(([label, value]) => (
                  <div key={label} className="rounded-lg border border-slate-800 bg-slate-950/60 px-3 py-2">
                    <p className="text-xs uppercase tracking-wide text-slate-500">{label}</p>
                    <p className="mt-1 text-sm font-semibold text-slate-100">{value || '—'}</p>
                  </div>
                ))}
              </div>
              <div className={smtTableWrap}>
                <table className="min-w-full text-left text-sm">
                  <thead className="bg-slate-900/60">
                    <tr>
                      <th className={smtTh}>{t('smt.reports.step')}</th>
                      <th className={smtTh}>{t('smt.reports.judged')}</th>
                      <th className={smtTh}>{t('smt.reports.firstPass')}</th>
                      <th className={smtTh}>{t('smt.reports.failed')}</th>
                      <th className={smtTh}>{t('smt.reports.rejected')}</th>
                      <th className={smtTh}>{t('smt.reports.ngHold')}</th>
                      <th className={smtTh}>{t('smt.reports.fpy')}</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-800">
                    {objectList(progress.data.steps).map((step) => (
                      <tr key={fieldText(step, 'operation_code')} className="bg-[#141519]">
                        <td className={smtTd}>
                          {Math.round(numberValue(step.seq))} · {fieldText(step, 'operation_code')}
                        </td>
                        <td className={smtTd}>{Math.round(numberValue(step.judged))}</td>
                        <td className={smtTd}>{Math.round(numberValue(step.first_pass))}</td>
                        <td className={smtTd}>{Math.round(numberValue(step.failed))}</td>
                        <td className={smtTd}>{Math.round(numberValue(step.rejected))}</td>
                        <td className={smtTd}>{Math.round(numberValue(step.ng_hold))}</td>
                        <td className={smtTd}>{percent(numberValue(step.fpy))}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          ) : (
            <SMTLoading />
          )}
        </Panel>
      ) : null}
    </div>
  )
}

// ---------------------------------------------------------------------------
// Traceability.
// ---------------------------------------------------------------------------

function SMTTraceTab({ token }: { token: string }) {
  const { t } = useI18n()
  const [snInput, setSnInput] = useState('')
  const [reelInput, setReelInput] = useState('')
  const [snResult, setSnResult] = useState<Record<string, unknown> | null>(null)
  const [reelResult, setReelResult] = useState<Record<string, unknown> | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const querySN = useCallback(async () => {
    const sn = snInput.trim()
    if (!sn) return
    setBusy(true)
    setError('')
    try {
      setSnResult(await getSMTTraceSN(token, sn))
    } catch (err) {
      setSnResult(null)
      setError(smtErrorText(err))
    } finally {
      setBusy(false)
    }
  }, [snInput, token])

  const queryReel = useCallback(async () => {
    const reel = reelInput.trim()
    if (!reel) return
    setBusy(true)
    setError('')
    try {
      setReelResult(await getSMTReelUnits(token, reel))
    } catch (err) {
      setReelResult(null)
      setError(smtErrorText(err))
    } finally {
      setBusy(false)
    }
  }, [reelInput, token])

  const unit = (snResult?.unit ?? null) as Record<string, unknown> | null
  const events = objectList(snResult?.events)
  const defects = objectList(snResult?.defects)
  const materials = objectList(snResult?.materials)
  const panel = (snResult?.panel ?? null) as Record<string, unknown> | null
  const reel = (reelResult?.reel ?? null) as Record<string, unknown> | null
  const reelUnits = objectList(reelResult?.units)

  return (
    <div className="space-y-4">
      {error ? <FeedbackMessage error>{error}</FeedbackMessage> : null}
      <Panel
        title={t('smt.tab.trace')}
        actions={
          <button type="button" className={smtButton} disabled={busy} onClick={() => void querySN()}>
            <Search className="h-4 w-4" />
            {t('smt.trace.querySn')}
          </button>
        }
      >
        <div className="flex flex-wrap items-end gap-2">
          <div className="w-64">
            <label className={smtLabel} htmlFor="smt-trace-sn">
              {t('ontology.field.sn')}
            </label>
            <input
              id="smt-trace-sn"
              className={smtInput}
              value={snInput}
              placeholder={t('smt.trace.snPlaceholder')}
              onChange={(event) => setSnInput(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === 'Enter') void querySN()
              }}
            />
          </div>
          <div className="w-64">
            <label className={smtLabel} htmlFor="smt-trace-reel">
              {t('ontology.field.reelCode')}
            </label>
            <input
              id="smt-trace-reel"
              className={smtInput}
              value={reelInput}
              placeholder={t('smt.trace.reelPlaceholder')}
              onChange={(event) => setReelInput(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === 'Enter') void queryReel()
              }}
            />
          </div>
          <button type="button" className={smtButton} disabled={busy} onClick={() => void queryReel()}>
            <Search className="h-4 w-4" />
            {t('smt.trace.queryReel')}
          </button>
        </div>
      </Panel>

      {snResult ? (
        <div className="space-y-4">
          <Panel title={t('smt.trace.unit')}>
            {unit ? (
              <div className="grid gap-2 sm:grid-cols-3 lg:grid-cols-5">
                {[
                  ['status', fieldText(unit, 'status', 'Status')],
                  ['workOrder', fieldText(unit, 'work_order')],
                  ['product', fieldText(unit, 'product_code')],
                  ['lastSeq', String(Math.round(fieldNumber(unit, 'last_seq')))],
                  ['failCount', String(Math.round(fieldNumber(unit, 'fail_count')))],
                  ['panel', fieldText(unit, 'panel_code')],
                ].map(([label, value]) => (
                  <div key={label} className="rounded-lg border border-slate-800 bg-slate-950/60 px-3 py-2">
                    <p className="text-xs uppercase tracking-wide text-slate-500">{t(`ontology.field.${label}`)}</p>
                    <p className="mt-1 text-sm font-semibold text-slate-100">{value || '—'}</p>
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-sm text-slate-500">{t('smt.trace.notFound')}</p>
            )}
          </Panel>
          <Panel title={t('smt.trace.events')}>
            {events.length === 0 ? (
              <p className="text-sm text-slate-500">{t('smt.units.noHistory')}</p>
            ) : (
              <div className={smtTableWrap}>
                <table className="min-w-full text-left text-sm">
                  <thead className="bg-slate-900/60">
                    <tr>
                      <th className={smtTh}>{t('ontology.field.seq')}</th>
                      <th className={smtTh}>{t('ontology.field.operation')}</th>
                      <th className={smtTh}>{t('ontology.field.result')}</th>
                      <th className={smtTh}>{t('ontology.field.rejectType')}</th>
                      <th className={smtTh}>{t('ontology.field.machine')}</th>
                      <th className={smtTh}>{t('ontology.field.createdAt')}</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-800">
                    {events.map((event, index) => (
                      <tr key={`${fieldText(event, 'seq')}-${index}`} className="bg-[#141519]">
                        <td className={smtTd}>{Math.round(fieldNumber(event, 'seq'))}</td>
                        <td className={smtTd}>{fieldText(event, 'operation_code')}</td>
                        <td className={smtTd}>{fieldText(event, 'result')}</td>
                        <td className={smtTd}>{fieldText(event, 'reject_type') || '—'}</td>
                        <td className={smtTd}>{fieldText(event, 'machine_code')}</td>
                        <td className={smtTd}>{fieldText(event, 'created_at')}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </Panel>
          <div className="grid gap-4 xl:grid-cols-3">
            <Panel title={t('smt.trace.defects')}>
              {defects.length === 0 ? (
                <p className="text-sm text-slate-500">{t('common.noData')}</p>
              ) : (
                <ul className="space-y-2 text-sm text-slate-300">
                  {defects.map((defect, index) => (
                    <li key={`${fieldText(defect, 'defect_code')}-${index}`} className="rounded-lg border border-slate-800 bg-slate-950/60 px-3 py-2">
                      <p className="font-semibold text-slate-100">
                        {fieldText(defect, 'defect_code')} · {t(`smt.repair.status.${fieldText(defect, 'status', 'Status').toUpperCase()}`)}
                      </p>
                      <p className="text-xs text-slate-400">
                        {t('ontology.field.location')}: {fieldText(defect, 'location') || '—'} ·{' '}
                        {t('ontology.field.createdAt')}: {fieldText(defect, 'created_at')}
                      </p>
                    </li>
                  ))}
                </ul>
              )}
            </Panel>
            <Panel title={t('smt.trace.materials')}>
              {materials.length === 0 ? (
                <p className="text-sm text-slate-500">{t('common.noData')}</p>
              ) : (
                <div className={smtTableWrap}>
                  <table className="min-w-full text-left text-sm">
                    <thead className="bg-slate-900/60">
                      <tr>
                        <th className={smtTh}>{t('smt.station.machine')}</th>
                        <th className={smtTh}>{t('smt.station.slot')}</th>
                        <th className={smtTh}>{t('ontology.field.reelCode')}</th>
                        <th className={smtTh}>{t('ontology.field.item')}</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-800">
                      {materials.map((material, index) => (
                        <tr key={`${fieldText(material, 'reel_code')}-${index}`} className="bg-[#141519]">
                          <td className={smtTd}>{fieldText(material, 'machine_code')}</td>
                          <td className={smtTd}>{fieldText(material, 'slot_no')}</td>
                          <td className={smtTd}>{fieldText(material, 'reel_code')}</td>
                          <td className={smtTd}>{fieldText(material, 'part_no')}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </Panel>
            <Panel title={t('smt.trace.panel')}>
              {panel ? (
                <div className="space-y-2 text-sm text-slate-300">
                  <p>
                    <span className="text-slate-500">{t('smt.units.panelCode')}: </span>
                    {fieldText(panel, 'panel_code')}
                  </p>
                  <p>
                    <span className="text-slate-500">{t('smt.units.boardCount')}: </span>
                    {Math.round(fieldNumber(panel, 'board_count'))}
                  </p>
                  <p>
                    <span className="text-slate-500">{t('ontology.field.status')}: </span>
                    {fieldText(panel, 'status', 'Status')}
                  </p>
                </div>
              ) : (
                <p className="text-sm text-slate-500">{t('common.noData')}</p>
              )}
            </Panel>
          </div>
        </div>
      ) : null}

      {reelResult ? (
        <Panel title={t('smt.trace.reel')}>
          <div className="space-y-3">
            {reel ? (
              <div className="grid gap-2 sm:grid-cols-3 lg:grid-cols-5">
                {[
                  ['item', fieldText(reel, 'part_no')],
                  ['lot', fieldText(reel, 'lot_no')],
                  ['quantity', String(Math.round(fieldNumber(reel, 'qty')))],
                  ['status', fieldText(reel, 'status', 'Status')],
                  ['ownership', fieldText(reel, 'owner_type')],
                ].map(([label, value]) => (
                  <div key={label} className="rounded-lg border border-slate-800 bg-slate-950/60 px-3 py-2">
                    <p className="text-xs uppercase tracking-wide text-slate-500">{t(`ontology.field.${label}`)}</p>
                    <p className="mt-1 text-sm font-semibold text-slate-100">{value || '—'}</p>
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-sm text-slate-500">{t('smt.trace.notFound')}</p>
            )}
            <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{t('smt.trace.units')}</p>
            {reelUnits.length === 0 ? (
              <p className="text-sm text-slate-500">{t('common.noData')}</p>
            ) : (
              <div className={smtTableWrap}>
                <table className="min-w-full text-left text-sm">
                  <thead className="bg-slate-900/60">
                    <tr>
                      <th className={smtTh}>{t('ontology.field.sn')}</th>
                      <th className={smtTh}>{t('smt.units.workOrder')}</th>
                      <th className={smtTh}>{t('ontology.field.machine')}</th>
                      <th className={smtTh}>{t('ontology.field.createdAt')}</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-800">
                    {reelUnits.map((row, index) => (
                      <tr key={`${fieldText(row, 'sn')}-${index}`} className="bg-[#141519]">
                        <td className={smtTd}>{fieldText(row, 'sn')}</td>
                        <td className={smtTd}>{fieldText(row, 'work_order')}</td>
                        <td className={smtTd}>{fieldText(row, 'machine_code')}</td>
                        <td className={smtTd}>{fieldText(row, 'passed_at', 'created_at')}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </Panel>
      ) : null}
    </div>
  )
}

// ---------------------------------------------------------------------------
// Reports.
// ---------------------------------------------------------------------------

type SMTReportKind = 'production' | 'andon' | 'downtime' | 'materials'

function SMTReportsTab({ token }: { token: string }) {
  const { t } = useI18n()
  const [kind, setKind] = useState<SMTReportKind>('production')
  const [from, setFrom] = useState(daysAgoISO(6))
  const [to, setTo] = useState(todayISO())
  const [lineCode, setLineCode] = useState('')
  const [version, setVersion] = useState(0)
  const lines = useSMTResource(() => listERPRecords<SMTRecord>(token, 'MLIN', 100), [token])
  const report = useSMTResource(
    () => getSMTReport(token, kind, { from, to, line_code: lineCode || undefined }),
    [token, kind, from, to, lineCode, version],
  )
  const data = report.data
  const daily = objectList(data?.daily)
  const steps = objectList(data?.steps)
  const defectPareto = objectList(data?.defects)
  const rejectPareto = objectList(data?.rejects)
  const byType = (data?.by_type ?? null) as Record<string, unknown> | null
  const calls = objectList(data?.calls)
  const downtimes = objectList(data?.machines)
  const reasons = objectList(data?.reasons)
  const parts = objectList(data?.parts)

  return (
    <div className="space-y-4">
      {report.error ? <FeedbackMessage error>{report.error}</FeedbackMessage> : null}
      <Panel
        title={t('smt.tab.reports')}
        actions={
          <button type="button" className={smtButton} onClick={() => setVersion((value) => value + 1)}>
            <RefreshCw className={`h-4 w-4 ${report.loading ? 'animate-spin' : ''}`} />
            {t('smt.reports.query')}
          </button>
        }
      >
        <div className="flex flex-wrap items-end gap-3">
          <div className="w-56">
            <label className={smtLabel} htmlFor="smt-report-kind">
              {t('smt.reports.kind')}
            </label>
            <select id="smt-report-kind" className={smtInput} value={kind} onChange={(event) => setKind(event.target.value as SMTReportKind)}>
              {(['production', 'andon', 'downtime', 'materials'] as const).map((value) => (
                <option key={value} value={value}>
                  {t(`smt.reports.${value}`)}
                </option>
              ))}
            </select>
          </div>
          <div className="w-44">
            <label className={smtLabel} htmlFor="smt-report-from">
              {t('smt.reports.from')}
            </label>
            <input id="smt-report-from" type="date" className={smtInput} value={from} onChange={(event) => setFrom(event.target.value)} />
          </div>
          <div className="w-44">
            <label className={smtLabel} htmlFor="smt-report-to">
              {t('smt.reports.to')}
            </label>
            <input id="smt-report-to" type="date" className={smtInput} value={to} onChange={(event) => setTo(event.target.value)} />
          </div>
          {kind === 'production' ? (
            <div className="w-48">
              <label className={smtLabel} htmlFor="smt-report-line">
                {t('smt.board.line')}
              </label>
              <select id="smt-report-line" className={smtInput} value={lineCode} onChange={(event) => setLineCode(event.target.value)}>
                <option value="">{t('smt.board.allLines')}</option>
                {(lines.data ?? []).map((line) => (
                  <option key={line.key} value={line.key}>
                    {fieldText(line, 'Name') ? `${fieldText(line, 'Name')} · ${line.key}` : line.key}
                  </option>
                ))}
              </select>
            </div>
          ) : null}
        </div>
      </Panel>

      {data ? (
        <div className="space-y-4">
          {kind === 'production' ? (
            <>
              <div className="grid gap-3 sm:grid-cols-2">
                <div className="rounded-lg border border-slate-800 bg-slate-950/60 px-3 py-2">
                  <p className="text-xs uppercase tracking-wide text-slate-500">{t('smt.reports.unitCount')}</p>
                  <p className="mt-1 text-lg font-semibold text-slate-100">{Math.round(numberValue(data.unit_count))}</p>
                </div>
                <div className="rounded-lg border border-slate-800 bg-slate-950/60 px-3 py-2">
                  <p className="text-xs uppercase tracking-wide text-slate-500">{t('smt.reports.rty')}</p>
                  <p className="mt-1 text-lg font-semibold text-slate-100">{percent(numberValue(data.rty))}</p>
                </div>
              </div>
              <div className="grid gap-4 xl:grid-cols-2">
                <Panel title={t('smt.reports.daily')}>
                  {daily.length === 0 ? (
                    <p className="text-sm text-slate-500">{t('common.noData')}</p>
                  ) : (
                    <div className={smtTableWrap}>
                      <table className="min-w-full text-left text-sm">
                        <thead className="bg-slate-900/60">
                          <tr>
                            <th className={smtTh}>{t('smt.reports.date')}</th>
                            <th className={smtTh}>{t('smt.reports.in')}</th>
                            <th className={smtTh}>{t('smt.reports.out')}</th>
                          </tr>
                        </thead>
                        <tbody className="divide-y divide-slate-800">
                          {daily.map((row) => (
                            <tr key={fieldText(row, 'day')} className="bg-[#141519]">
                              <td className={smtTd}>{fieldText(row, 'day')}</td>
                              <td className={smtTd}>{Math.round(numberValue(row.in))}</td>
                              <td className={smtTd}>{Math.round(numberValue(row.out))}</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  )}
                </Panel>
                <Panel title={t('smt.reports.steps')}>
                  {steps.length === 0 ? (
                    <p className="text-sm text-slate-500">{t('common.noData')}</p>
                  ) : (
                    <div className={smtTableWrap}>
                      <table className="min-w-full text-left text-sm">
                        <thead className="bg-slate-900/60">
                          <tr>
                            <th className={smtTh}>{t('smt.reports.step')}</th>
                            <th className={smtTh}>{t('smt.reports.judged')}</th>
                            <th className={smtTh}>{t('smt.reports.firstPass')}</th>
                            <th className={smtTh}>{t('smt.reports.failed')}</th>
                            <th className={smtTh}>{t('smt.reports.fpy')}</th>
                          </tr>
                        </thead>
                        <tbody className="divide-y divide-slate-800">
                          {steps.map((row) => (
                            <tr key={fieldText(row, 'operation_code')} className="bg-[#141519]">
                              <td className={smtTd}>{fieldText(row, 'operation_code')}</td>
                              <td className={smtTd}>{Math.round(numberValue(row.judged))}</td>
                              <td className={smtTd}>{Math.round(numberValue(row.first_pass))}</td>
                              <td className={smtTd}>{Math.round(numberValue(row.failed))}</td>
                              <td className={smtTd}>{percent(numberValue(row.fpy))}</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  )}
                </Panel>
                <Panel title={t('smt.reports.defectsPareto')}>
                  <SMTReportPareto rows={defectPareto} emptyLabel={t('common.noData')} labels={{ code: t('smt.reports.code'), count: t('smt.reports.count') }} />
                </Panel>
                <Panel title={t('smt.reports.rejectsPareto')}>
                  <SMTReportPareto rows={rejectPareto} emptyLabel={t('common.noData')} labels={{ code: t('smt.reports.code'), count: t('smt.reports.count') }} />
                </Panel>
              </div>
            </>
          ) : null}

          {kind === 'andon' ? (
            <>
              <div className="grid gap-3 sm:grid-cols-4">
                {[
                  [t('smt.reports.total'), String(Math.round(numberValue(data.total)))],
                  [t('smt.reports.avgResponse'), String(numberValue(data.avg_response_min).toFixed(1))],
                  [t('smt.reports.avgClose'), String(numberValue(data.avg_close_min).toFixed(1))],
                  [t('smt.reports.byType'), byType ? Object.entries(byType).map(([key, value]) => `${key}:${value}`).join(' · ') : '—'],
                ].map(([label, value]) => (
                  <div key={label} className="rounded-lg border border-slate-800 bg-slate-950/60 px-3 py-2">
                    <p className="text-xs uppercase tracking-wide text-slate-500">{label}</p>
                    <p className="mt-1 text-sm font-semibold text-slate-100">{value}</p>
                  </div>
                ))}
              </div>
              <Panel title={t('smt.reports.calls')}>
                {calls.length === 0 ? (
                  <p className="text-sm text-slate-500">{t('smt.andon.empty')}</p>
                ) : (
                  <div className={smtTableWrap}>
                    <table className="min-w-full text-left text-sm">
                      <thead className="bg-slate-900/60">
                        <tr>
                          <th className={smtTh}>{t('erp.document.smtAndonCall')}</th>
                          <th className={smtTh}>{t('smt.andon.callType')}</th>
                          <th className={smtTh}>{t('smt.station.machine')}</th>
                          <th className={smtTh}>{t('ontology.field.status')}</th>
                          <th className={smtTh}>{t('ontology.field.createdAt')}</th>
                          <th className={smtTh}>{t('smt.reports.avgResponse')}</th>
                          <th className={smtTh}>{t('smt.reports.avgClose')}</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-slate-800">
                        {calls.map((row, index) => (
                          <tr key={`${fieldText(row, 'call_no')}-${index}`} className="bg-[#141519]">
                            <td className={smtTd}>{fieldText(row, 'call_no')}</td>
                            <td className={smtTd}>{t(`smt.andonType.${fieldText(row, 'call_type')}`)}</td>
                            <td className={smtTd}>{fieldText(row, 'machine_code')}</td>
                            <td className={smtTd}>{t(`smt.andon.status.${fieldText(row, 'status')}`)}</td>
                            <td className={smtTd}>{fieldText(row, 'called_at')}</td>
                            <td className={smtTd}>{fieldText(row, 'ack_at') || '—'}</td>
                            <td className={smtTd}>{fieldText(row, 'closed_at') || '—'}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
              </Panel>
            </>
          ) : null}

          {kind === 'downtime' ? (
            <>
              <div className="grid gap-3 sm:grid-cols-2">
                <div className="rounded-lg border border-slate-800 bg-slate-950/60 px-3 py-2">
                  <p className="text-xs uppercase tracking-wide text-slate-500">{t('smt.reports.mttr')}</p>
                  <p className="mt-1 text-lg font-semibold text-slate-100">{numberValue(data.mttr_min).toFixed(1)}</p>
                </div>
              </div>
              <div className="grid gap-4 xl:grid-cols-2">
                <Panel title={t('smt.reports.availability')}>
                  {downtimes.length === 0 ? (
                    <p className="text-sm text-slate-500">{t('common.noData')}</p>
                  ) : (
                    <div className={smtTableWrap}>
                      <table className="min-w-full text-left text-sm">
                        <thead className="bg-slate-900/60">
                          <tr>
                            <th className={smtTh}>{t('smt.reports.machine')}</th>
                            <th className={smtTh}>{t('smt.reports.availableMin')}</th>
                            <th className={smtTh}>{t('smt.reports.usedMin')}</th>
                            <th className={smtTh}>{t('smt.reports.availability')}</th>
                          </tr>
                        </thead>
                        <tbody className="divide-y divide-slate-800">
                          {downtimes.map((row) => (
                            <tr key={fieldText(row, 'machine_code')} className="bg-[#141519]">
                              <td className={smtTd}>{fieldText(row, 'machine_code')}</td>
                              <td className={smtTd}>{Math.round(numberValue(row.planned_seconds) / 60)}</td>
                              <td className={smtTd}>{Math.round(numberValue(row.downtime_seconds) / 60)}</td>
                              <td className={smtTd}>{percent(numberValue(row.availability))}</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  )}
                </Panel>
                <Panel title={t('smt.reports.reason')}>
                  <SMTReportPareto rows={reasons} emptyLabel={t('common.noData')} labels={{ code: t('smt.reports.code'), count: t('smt.reports.count') }} />
                </Panel>
              </div>
            </>
          ) : null}

          {kind === 'materials' ? (
            <>
              <div className="grid gap-3 sm:grid-cols-3">
                {[
                  [t('smt.reports.inQty'), String(Math.round(numberValue((data.total as Record<string, unknown> | null)?.in_qty)))],
                  [t('smt.reports.consumedQty'), String(Math.round(numberValue((data.total as Record<string, unknown> | null)?.consumed_qty)))],
                  [t('smt.reports.inReels'), String(Math.round(numberValue((data.total as Record<string, unknown> | null)?.in_reels)))],
                ].map(([label, value]) => (
                  <div key={label} className="rounded-lg border border-slate-800 bg-slate-950/60 px-3 py-2">
                    <p className="text-xs uppercase tracking-wide text-slate-500">{label}</p>
                    <p className="mt-1 text-lg font-semibold text-slate-100">{value}</p>
                  </div>
                ))}
              </div>
              <Panel title={t('smt.reports.materials')}>
                {parts.length === 0 ? (
                  <p className="text-sm text-slate-500">{t('common.noData')}</p>
                ) : (
                  <div className={smtTableWrap}>
                    <table className="min-w-full text-left text-sm">
                      <thead className="bg-slate-900/60">
                        <tr>
                          <th className={smtTh}>{t('smt.reports.part')}</th>
                          <th className={smtTh}>{t('smt.reports.inQty')}</th>
                          <th className={smtTh}>{t('smt.reports.consumedQty')}</th>
                          <th className={smtTh}>{t('smt.reports.inReels')}</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-slate-800">
                        {parts.map((row) => (
                          <tr key={fieldText(row, 'part_no')} className="bg-[#141519]">
                            <td className={smtTd}>{fieldText(row, 'part_no')}</td>
                            <td className={smtTd}>{Math.round(numberValue(row.in_qty))}</td>
                            <td className={smtTd}>{Math.round(numberValue(row.consumed_qty))}</td>
                            <td className={smtTd}>{Math.round(numberValue(row.in_reels))}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
              </Panel>
            </>
          ) : null}
        </div>
      ) : (
        <SMTLoading />
      )}
    </div>
  )
}

function SMTReportPareto({
  rows,
  emptyLabel,
  labels,
}: {
  rows: Array<Record<string, unknown>>
  emptyLabel: string
  labels: { code: string; count: string }
}) {
  if (rows.length === 0) return <p className="text-sm text-slate-500">{emptyLabel}</p>
  const max = Math.max(...rows.map((row) => numberValue(row.count)), 1)
  return (
    <ul className="space-y-2 text-sm text-slate-300">
      {rows.map((row) => {
        const count = numberValue(row.count)
        return (
          <li key={String(row.key)} className="space-y-1">
            <div className="flex items-center justify-between gap-2">
              <span className="truncate" title={String(row.key)}>
                {String(row.key)}
              </span>
              <span className="text-slate-400">{count}</span>
            </div>
            <div className="h-1.5 w-full overflow-hidden rounded-full bg-slate-800">
              <div className="h-full rounded-full bg-[#DF6A24]" style={{ width: `${Math.max((count / max) * 100, 2)}%` }} />
            </div>
          </li>
        )
      })}
    </ul>
  )
}

// ---------------------------------------------------------------------------
// Settings.
// ---------------------------------------------------------------------------

function SMTSettingsTab({ token }: { token: string }) {
  const { t } = useI18n()
  const settings = useSMTResource(() => getSMTSettings(token), [token])
  const [draft, setDraft] = useState<Record<string, string>>({})
  const [notice, setNotice] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const stored: Record<string, string> = settings.data?.stored ?? {}
  const settingValues = settings.data?.values
  const values = useMemo<Record<string, string>>(() => settingValues ?? {}, [settingValues])
  const keys = useMemo(() => Object.keys(values).sort(), [values])

  const save = useCallback(async () => {
    const changed: Record<string, string> = {}
    for (const [key, value] of Object.entries(draft)) {
      if (values[key] !== value) changed[key] = value
    }
    if (Object.keys(changed).length === 0) {
      setNotice(t('common.noData'))
      return
    }
    setBusy(true)
    setError('')
    try {
      await updateSMTSettings(token, changed)
      setNotice(t('smt.settings.saved'))
      setDraft({})
      settings.reload()
    } catch (err) {
      setError(smtErrorText(err))
    } finally {
      setBusy(false)
    }
  }, [draft, settings, t, token, values])

  return (
    <div className="space-y-4">
      {error ? <FeedbackMessage error>{error}</FeedbackMessage> : null}
      {notice ? <FeedbackMessage>{notice}</FeedbackMessage> : null}
      <Panel
        title={t('smt.tab.settings')}
        actions={
          <div className="flex gap-2">
            <button type="button" className={smtButton} onClick={() => settings.reload()}>
              <RefreshCw className="h-4 w-4" />
              {t('common.refresh')}
            </button>
            <button type="button" className={smtPrimaryButton} disabled={busy} onClick={() => void save()}>
              <Settings2 className="h-4 w-4" />
              {t('smt.settings.save')}
            </button>
          </div>
        }
      >
        <p className="mb-3 text-xs text-slate-500">{t('smt.settings.storedHint')}</p>
        {settings.error ? <FeedbackMessage error>{settings.error}</FeedbackMessage> : null}
        {settings.loading && keys.length === 0 ? (
          <SMTLoading />
        ) : (
          <div className={smtTableWrap}>
            <table className="min-w-full text-left text-sm">
              <thead className="bg-slate-900/60">
                <tr>
                  <th className={smtTh}>{t('smt.settings.key')}</th>
                  <th className={smtTh}>{t('smt.settings.value')}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800">
                {keys.map((key) => (
                  <tr key={key} className="bg-[#141519]">
                    <td className={smtTd}>
                      <span className="font-mono text-xs text-slate-200">{key}</span>
                      {stored[key] !== undefined ? (
                        <span className="ml-2 rounded bg-slate-800 px-1.5 py-0.5 text-xs text-slate-400">
                          {t('smt.edges.configured')}
                        </span>
                      ) : null}
                    </td>
                    <td className={smtTd}>
                      <input
                        className={`${smtInput} h-9`}
                        value={draft[key] ?? values[key] ?? ''}
                        onChange={(event) => setDraft((current) => ({ ...current, [key]: event.target.value }))}
                      />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Panel>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Edge integration.
// ---------------------------------------------------------------------------

const smtPushScopes = ['materials', 'customers', 'products', 'station-tables', 'work-orders'] as const

function SMTEdgesTab({ token }: { token: string }) {
  const { t } = useI18n()
  const { feedback, notify } = useSMTFeedback()
  const [version, setVersion] = useState(0)
  const [selected, setSelected] = useState('')
  const [busy, setBusy] = useState(false)
  const [name, setName] = useState('')
  const [baseUrl, setBaseUrl] = useState('')
  const [apiKey, setApiKey] = useState('')
  const [scopes, setScopes] = useState<string[]>([])
  const edges = useSMTResource(() => listSMTEdges(token), [token, version])
  const events = useSMTResource(() => (selected ? listSMTEdgeEvents(token, selected, 50) : Promise.resolve([])), [token, selected, version])

  useEffect(() => {
    if (selected) return
    const first = edges.data?.[0]?.instance_key
    if (!first) return
    const timer = window.setTimeout(() => setSelected(first), 0)
    return () => window.clearTimeout(timer)
  }, [edges.data, selected])

  const current = (edges.data ?? []).find((edge) => edge.instance_key === selected)

  const createInstance = useCallback(async () => {
    if (!name.trim() || !baseUrl.trim() || !apiKey.trim()) return
    setBusy(true)
    try {
      await createSMTEdge(token, { name: name.trim(), base_url: baseUrl.trim(), api_key: apiKey.trim() })
      notify('OK', t('common.save'))
      setName('')
      setBaseUrl('')
      setApiKey('')
      setVersion((value) => value + 1)
    } catch (err) {
      notify('ERROR', smtErrorText(err))
    } finally {
      setBusy(false)
    }
  }, [apiKey, baseUrl, name, notify, t, token])

  const toggleStatus = useCallback(async () => {
    if (!current) return
    setBusy(true)
    try {
      await updateSMTEdge(token, current.instance_key, { status: current.status === 'active' ? 'disabled' : 'active' })
      setVersion((value) => value + 1)
    } catch (err) {
      notify('ERROR', smtErrorText(err))
    } finally {
      setBusy(false)
    }
  }, [current, notify, token])

  const removeInstance = useCallback(async () => {
    if (!current) return
    setBusy(true)
    try {
      await deleteSMTEdge(token, current.instance_key)
      setSelected('')
      setVersion((value) => value + 1)
    } catch (err) {
      notify('ERROR', smtErrorText(err))
    } finally {
      setBusy(false)
    }
  }, [current, notify, token])

  const runPush = useCallback(async () => {
    if (!current) return
    setBusy(true)
    try {
      const result = await pushSMTEdge(token, current.instance_key, scopes)
      const results = objectList(result.results)
      const summary = results
        .map((row) => `${fieldText(row, 'scope', 'kind')}:${fieldText(row, 'status', 'result')}`)
        .filter(Boolean)
        .join(' · ')
      notify('OK', summary || t('common.save'))
      setVersion((value) => value + 1)
    } catch (err) {
      notify('ERROR', smtErrorText(err))
    } finally {
      setBusy(false)
    }
  }, [current, notify, scopes, t, token])

  const runPull = useCallback(async () => {
    if (!current) return
    setBusy(true)
    try {
      const result = await pullSMTEdge(token, current.instance_key)
      notify('OK', `${t('smt.edges.pulled')} · ${fieldText(result, 'processed', 'received') || 0}`)
      setVersion((value) => value + 1)
    } catch (err) {
      notify('ERROR', smtErrorText(err))
    } finally {
      setBusy(false)
    }
  }, [current, notify, t, token])

  return (
    <div className="space-y-4">
      <SMTFeedbackBanner feedback={feedback} />
      {edges.error ? <FeedbackMessage error>{edges.error}</FeedbackMessage> : null}
      <Panel title={t('smt.tab.edges')}>
        <p className="mb-3 text-xs text-slate-500">{t('smt.edges.hint')}</p>
        <div className="grid gap-3 sm:grid-cols-3">
          <div>
            <label className={smtLabel} htmlFor="smt-edge-name">
              {t('smt.edges.name')}
            </label>
            <input id="smt-edge-name" className={smtInput} value={name} onChange={(event) => setName(event.target.value)} />
          </div>
          <div>
            <label className={smtLabel} htmlFor="smt-edge-url">
              {t('smt.edges.baseUrl')}
            </label>
            <input
              id="smt-edge-url"
              className={smtInput}
              placeholder="http://127.0.0.1:8000"
              value={baseUrl}
              onChange={(event) => setBaseUrl(event.target.value)}
            />
          </div>
          <div>
            <label className={smtLabel} htmlFor="smt-edge-key">
              {t('smt.edges.apiKey')}
            </label>
            <input id="smt-edge-key" className={smtInput} value={apiKey} onChange={(event) => setApiKey(event.target.value)} />
          </div>
        </div>
        <div className="mt-3">
          <button type="button" className={smtPrimaryButton} disabled={busy} onClick={() => void createInstance()}>
            <Radio className="h-4 w-4" />
            {t('smt.edges.create')}
          </button>
        </div>
      </Panel>

      {(edges.data ?? []).length === 0 ? (
        <p className="text-sm text-slate-500">{t('smt.edges.empty')}</p>
      ) : (
        <div className="grid gap-4 xl:grid-cols-2">
          <Panel title={t('smt.edges.instances')}>
            <div className={smtTableWrap}>
              <table className="min-w-full text-left text-sm">
                <thead className="bg-slate-900/60">
                  <tr>
                    <th className={smtTh}>{t('smt.edges.name')}</th>
                    <th className={smtTh}>{t('smt.edges.baseUrl')}</th>
                    <th className={smtTh}>{t('ontology.field.status')}</th>
                    <th className={smtTh}>{t('smt.edges.lastSync')}</th>
                    <th className={smtTh}>{t('smt.edges.lastEvent')}</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800">
                  {(edges.data ?? []).map((edge: SMTEdgeInstance) => (
                    <tr
                      key={edge.instance_key}
                      className={`cursor-pointer ${edge.instance_key === selected ? 'bg-[#1d1e24]' : 'bg-[#141519]'}`}
                      onClick={() => setSelected(edge.instance_key)}
                    >
                      <td className={smtTd}>{edge.name}</td>
                      <td className={smtTd}>{edge.base_url}</td>
                      <td className={smtTd}>{edge.status}</td>
                      <td className={smtTd}>{edge.last_sync_at || '—'}</td>
                      <td className={smtTd}>{edge.last_event_at || '—'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            {current ? (
              <div className="mt-4 space-y-3">
                <div className="flex flex-wrap items-center gap-3 text-xs text-slate-400">
                  <span>
                    {t('smt.edges.apiKey')}: {current.api_key_masked || t('smt.edges.configured')}
                  </span>
                  {current.webhook_path ? (
                    <span>
                      {t('smt.edges.webhook')}: <span className="font-mono">{current.webhook_path}</span>
                    </span>
                  ) : null}
                </div>
                <div className="flex flex-wrap gap-2">
                  {smtPushScopes.map((scope) => (
                    <label key={scope} className="inline-flex items-center gap-1 text-xs text-slate-400">
                      <input
                        type="checkbox"
                        checked={scopes.includes(scope)}
                        onChange={(event) =>
                          setScopes((current2) =>
                            event.target.checked ? [...current2, scope] : current2.filter((value) => value !== scope),
                          )
                        }
                      />
                      {scope}
                    </label>
                  ))}
                </div>
                <div className="flex flex-wrap gap-2">
                  <button type="button" className={smtButton} disabled={busy} onClick={() => void runPush()}>
                    {t('smt.edges.push')}
                  </button>
                  <button type="button" className={smtButton} disabled={busy} onClick={() => void runPull()}>
                    {t('smt.edges.pull')}
                  </button>
                  <button type="button" className={smtButton} disabled={busy} onClick={() => void toggleStatus()}>
                    {current.status === 'active' ? t('smt.edges.disable') : t('smt.edges.enable')}
                  </button>
                  <button type="button" className={smtDangerButton} disabled={busy} onClick={() => void removeInstance()}>
                    {t('smt.edges.delete')}
                  </button>
                </div>
                <p className="text-xs text-slate-500">{t('smt.edges.scopeHint')}</p>
              </div>
            ) : null}
          </Panel>

          <Panel
            title={t('smt.edges.events')}
            actions={
              <button type="button" className={smtButton} onClick={() => setVersion((value) => value + 1)}>
                <RefreshCw className="h-4 w-4" />
                {t('common.refresh')}
              </button>
            }
          >
            {!selected ? (
              <p className="text-sm text-slate-500">{t('smt.edges.empty')}</p>
            ) : (events.data ?? []).length === 0 ? (
              <p className="text-sm text-slate-500">{t('smt.edges.noEvents')}</p>
            ) : (
              <div className={smtTableWrap}>
                <table className="min-w-full text-left text-sm">
                  <thead className="bg-slate-900/60">
                    <tr>
                      <th className={smtTh}>{t('smt.edges.eventType')}</th>
                      <th className={smtTh}>{t('smt.edges.eventRef')}</th>
                      <th className={smtTh}>{t('ontology.field.status')}</th>
                      <th className={smtTh}>{t('smt.edges.receivedAt')}</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-800">
                    {(events.data ?? []).map((event: SMTEdgeEvent) => (
                      <tr key={`${event.instance_key}-${event.id}`} className="bg-[#141519]">
                        <td className={smtTd}>{event.event_type}</td>
                        <td className={smtTd}>{event.ref || `#${event.event_id}`}</td>
                        <td className={smtTd}>{t(`smt.edges.status.${event.status}`)}</td>
                        <td className={smtTd}>{event.received_at}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </Panel>
        </div>
      )}
    </div>
  )
}
