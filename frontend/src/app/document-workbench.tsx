'use client'

import { ArrowDown, ArrowRight, ArrowUp, ArrowUpDown, Check, CheckCircle2, ClipboardCopy, Clock3, FileInput, FileText, Link2, Loader2, LockKeyhole, MoreHorizontal, PanelBottomClose, PanelBottomOpen, Pencil, Play, Plus, RefreshCw, Save, Search, Trash2, Upload } from 'lucide-react'
import { type FormEvent, type KeyboardEvent, useId, useRef, useState } from 'react'

import { type ERPActionExecution } from '@/lib/api'
import { describeApiError } from '@/lib/api-error'
import { useI18n } from '@/lib/i18n'
import { type OntologyLink } from '@/lib/ontology'
import { resolveFieldCapability, type DocumentWorkbenchDefinition, type DocumentWorkbenchField } from '@/lib/workbench'
import { ActionMenu, Dialog, FeedbackMessage, StatusBadge, useUnsavedChanges, useWorkspaceInteraction } from './workspace-ui'

type WorkbenchRecord = Record<string, unknown> & { key?: string }
type Mutation = (key: string, data: Record<string, unknown>) => Promise<void>
export type WorkbenchLookupOptions = Record<string, Array<{ value: string; label: string }>>
type RecordStatus = 'draft' | 'pending' | 'approved' | 'posted' | 'closed' | 'active' | 'inactive' | 'void'
type FieldChange = { field: DocumentWorkbenchField; before: unknown; after: unknown }
type PendingSave = { kind: 'header' | 'line'; key: string; creating: boolean; data: Record<string, unknown>; changes: FieldChange[] }

interface DocumentWorkbenchProps {
  definition: DocumentWorkbenchDefinition
  records: WorkbenchRecord[]
  childRows: WorkbenchRecord[]
  selectedKey: string
  selectedRecord?: WorkbenchRecord
  onSelectRecord: (key: string) => void
  onRefresh: () => void
  onCreateHeader?: Mutation
  onUpdateHeader?: Mutation
  onDeleteHeader?: (key: string) => Promise<void>
  onCreateLine?: Mutation
  onUpdateLine?: Mutation
  onDeleteLine?: (key: string) => Promise<void>
  onExecute: (action: string, data: Record<string, unknown>) => Promise<void>
  onSearch: (search: string) => void
  onLoadMore?: () => void
  links: OntologyLink[]
  linksTruncated?: boolean
  history: ERPActionExecution[]
  onOpenLink: (table: string, key: string) => void
  busy?: boolean
  detailLoading?: boolean
  version?: number
  readOnly?: boolean
  lookupOptions?: WorkbenchLookupOptions
  totalCount?: number
  filterStatus?: string
  onStatusFilter?: (status: string) => void
  sortKey?: string
  sortDirection?: 'asc' | 'desc'
  onSort?: (key: string, direction: 'asc' | 'desc') => void
  onImport?: () => void
  onOpenSource?: () => void
}

const statusTones = {
  draft: 'neutral', pending: 'amber', approved: 'green', posted: 'blue',
  closed: 'neutral', active: 'green', inactive: 'neutral', void: 'red',
} as const

export function DocumentWorkbench({
  definition, records, childRows, selectedKey, selectedRecord, onSelectRecord, onRefresh,
  onCreateHeader, onUpdateHeader, onDeleteHeader, onCreateLine, onUpdateLine, onDeleteLine,
  onExecute, onSearch, onLoadMore, links, linksTruncated, history, onOpenLink,
  busy = false, detailLoading = false, version = 0, readOnly = false, lookupOptions = {},
  totalCount, filterStatus, onStatusFilter, sortKey = 'key', sortDirection = 'asc', onSort, onImport, onOpenSource,
}: DocumentWorkbenchProps) {
  const { t, locale } = useI18n()
  const { requestNavigation } = useWorkspaceInteraction()
  const tabsID = useId()
  const lineFormID = useId()
  const actionFormID = useId()
  const submitLock = useRef(false)
  const detailRef = useRef<HTMLDivElement>(null)
  const [detailsVisible, setDetailsVisible] = useState(true)
  const [creating, setCreating] = useState(false)
  const [editing, setEditing] = useState(false)
  const [headerDirty, setHeaderDirty] = useState(false)
  const [lineDirty, setLineDirty] = useState(false)
  const [lineMode, setLineMode] = useState<string | null>(null)
  const [tab, setTab] = useState<'detail' | 'links' | 'history'>('detail')
  const [actionID, setActionID] = useState('')
  const [deleteTarget, setDeleteTarget] = useState<'header' | 'line' | null>(null)
  const [pendingSave, setPendingSave] = useState<PendingSave | null>(null)
  const [formError, setFormError] = useState('')
  const [notice, setNotice] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [search, setSearch] = useState('')
  const [statusFilter, setStatusFilter] = useState('all')
  const disabled = busy || submitting
  const locked = readOnly || (!creating && recordIsImmutable(selectedRecord))
  const fieldsDisabled = disabled || detailLoading
  const editingHeader = creating || editing
  const detail = definition.detailTables[0]
  const fields = definition.headerFields.filter((field) => resolveFieldCapability(field, definition.fieldPermissions).readable)
  const lineFields = detail?.fields.filter((field) => resolveFieldCapability(field, definition.fieldPermissions).readable) ?? []
  const line = childRows.find((row) => String(row.key ?? row[detail?.lineKey ?? 'LineNum']) === lineMode)
  const nextLine = String(Math.max(0, ...childRows.map((row) => Number(row[detail?.lineKey ?? 'LineNum'] ?? row.key) || 0)) + 1)
  const activeAction = definition.actions.find((action) => action.id === actionID)
  const activeFilter = filterStatus ?? statusFilter
  const visibleRecords = onStatusFilter || activeFilter === 'all' ? records : records.filter((record) => recordStatus(record) === activeFilter)
  const isDocument = definition.primaryKey === 'DocEntry'
  const columns = isDocument ? ['key', 'partner', 'date', 'status', 'currency', 'total', 'external_number'] : ['key', 'name', 'status']
  const status = recordStatus(selectedRecord)
  const firstAvailableAction = definition.actions.find((action) => !action.disabledReasonKey)
  const hasDialog = !!pendingSave || !!activeAction || !!deleteTarget || !!lineMode
  useUnsavedChanges(headerDirty || lineDirty)

  async function submit(task: () => Promise<void>, after?: () => void) {
    if (submitLock.current) return
    submitLock.current = true
    setSubmitting(true)
    setFormError('')
    setNotice('')
    try {
      await task()
      after?.()
    } catch (err) {
      setFormError(describeApiError(err, t, t('common.operationFailed')))
    } finally {
      submitLock.current = false
      setSubmitting(false)
    }
  }

  function resetEditing() {
    setCreating(false)
    setEditing(false)
    setHeaderDirty(false)
    setLineMode(null)
    setLineDirty(false)
    setFormError('')
    setActionID('')
    setDeleteTarget(null)
    setPendingSave(null)
  }

  function select(key: string) {
    if (key === selectedKey && !creating) { setDetailsVisible(true); return }
    requestNavigation(() => {
      resetEditing(); setNotice(''); setDetailsVisible(true); onSelectRecord(key)
      requestAnimationFrame(() => detailRef.current?.scrollIntoView({ block: 'nearest', behavior: 'smooth' }))
    })
  }

  function prepareSave(event: FormEvent<HTMLFormElement>, kind: 'header' | 'line') {
    event.preventDefault()
    if (fieldsDisabled || locked) return
    const isNew = kind === 'header' ? creating : lineMode === 'new'
    const targetFields = kind === 'header' ? fields : lineFields
    const data = collectFields(event.currentTarget, targetFields, isNew, definition)
    const primaryKey = kind === 'header' ? definition.primaryKey : detail?.lineKey ?? 'LineNum'
    const key = isNew ? String(data[primaryKey] ?? '').trim() : kind === 'header' ? selectedKey : lineMode || ''
    const handler = kind === 'header' ? (isNew ? onCreateHeader : onUpdateHeader) : (isNew ? onCreateLine : onUpdateLine)
    if (!key || !handler) return
    const changes = fieldChanges(data, targetFields, kind === 'header' ? selectedRecord : line, isNew)
    if (changes.length === 0) return
    setFormError('')
    setPendingSave({ kind, key, creating: isNew, changes, data: isNew ? data : Object.fromEntries(changes.map((change) => [change.field.name, change.after])) })
  }

  function saveConfirmed() {
    if (!pendingSave || fieldsDisabled || locked) return
    const handler = pendingSave.kind === 'header'
      ? pendingSave.creating ? onCreateHeader : onUpdateHeader
      : pendingSave.creating ? onCreateLine : onUpdateLine
    if (!handler) return
    void submit(() => handler(pendingSave.key, pendingSave.data), () => {
      setPendingSave(null)
      if (pendingSave.kind === 'header') {
        setCreating(false)
        setEditing(false)
        setHeaderDirty(false)
      } else {
        setLineMode(null)
        setLineDirty(false)
      }
    })
  }

  function openLine(mode: string) {
    setFormError('')
    setLineDirty(false)
    setLineMode(mode)
  }

  function closeLine() {
    requestNavigation(() => { setLineMode(null); setLineDirty(false); setFormError('') })
  }

  function changeTab(next: typeof tab) {
    setTab(next)
  }

  function tabKeys(event: KeyboardEvent<HTMLElement>) {
    if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return
    event.preventDefault()
    const tabs = Array.from(event.currentTarget.querySelectorAll<HTMLButtonElement>('[role="tab"]'))
    const index = tabs.indexOf(document.activeElement as HTMLButtonElement)
    const next = event.key === 'Home' ? 0 : event.key === 'End' ? tabs.length - 1 : (index + (event.key === 'ArrowRight' ? 1 : -1) + tabs.length) % tabs.length
    tabs[next]?.click()
    tabs[next]?.focus()
  }

  async function copyKey() {
    try {
      await navigator.clipboard.writeText(selectedKey)
      setNotice(t('ui.document.copied'))
    } catch {
      setFormError(t('ui.document.copyFailed'))
    }
  }

  const contextCard = (key = selectedKey) => <div className="ui-dialog-context">
    <FileText size={23} /><div className="min-w-0 flex-1"><strong>{t(definition.titleKey)}</strong><small>{t('ui.document.recordNumber')}: {key}</small></div>
    {!creating && <StatusBadge tone={statusTones[status]}>{t('ui.status.' + status)}</StatusBadge>}
  </div>

  return (
    <section data-testid="document-workbench" className="document-workbench document-table-workbench" aria-busy={disabled}>
      <header className="document-toolbar">
        <div className="document-heading"><FileText size={19} /><div><h3>{t(definition.titleKey)}</h3><p>{t('ui.document.totalRecords', { count: totalCount ?? visibleRecords.length })}</p></div></div>
        <div className="document-toolbar-actions">
          {onImport && <button type="button" className="ui-button ui-button-secondary" disabled={disabled} onClick={() => requestNavigation(() => { resetEditing(); onImport() })}><Upload size={16} />{t('import.open')}</button>}
          {onCreateHeader && <button type="button" className="ui-button ui-button-primary" disabled={disabled} onClick={() => requestNavigation(() => { resetEditing(); setDetailsVisible(true); setCreating(true) })}><Plus size={16} />{t('ui.document.new')}</button>}
          <button type="button" className="ui-icon-button" disabled={disabled} onClick={() => requestNavigation(() => { resetEditing(); onRefresh() })} title={t('common.refresh')} aria-label={t('common.refresh')}><RefreshCw size={16} className={busy ? 'animate-spin' : ''} /></button>
          <button type="button" className="ui-icon-button" aria-expanded={detailsVisible} title={t(detailsVisible ? 'ui.document.hideDetail' : 'ui.document.showDetail')} aria-label={t(detailsVisible ? 'ui.document.hideDetail' : 'ui.document.showDetail')} onClick={() => setDetailsVisible((value) => !value)}>{detailsVisible ? <PanelBottomClose size={16} /> : <PanelBottomOpen size={16} />}</button>
        </div>
      </header>
      <div className="document-grid">
        <aside className="document-list-panel" aria-label={t('ui.document.records')}>
          <div className="document-list-controls">
          <form className="document-search" role="search" onSubmit={(event) => { event.preventDefault(); requestNavigation(() => { resetEditing(); onSearch(search.trim()) }) }}>
            <Search size={15} className="shrink-0" />
            <input name="search" value={search} onChange={(event) => setSearch(event.target.value)} aria-label={t('ontology.search')} placeholder={t('ontology.search')} />
            <button type="submit" className="ui-icon-button" disabled={disabled} title={t('ontology.search')} aria-label={t('ontology.search')}><ArrowRight size={15} /></button>
          </form>
          <div className="document-filter"><select className="ui-select" aria-label={t('ui.document.filter')} value={activeFilter} disabled={disabled} onChange={(event) => {
            const value = event.target.value
            requestNavigation(() => { resetEditing(); if (onStatusFilter) onStatusFilter(value); else setStatusFilter(value) })
          }}>
            <option value="all">{t('ui.document.allStatuses')}</option>
            {Object.keys(statusTones).map((value) => <option key={value} value={value}>{t('ui.status.' + value)}</option>)}
          </select></div>
          <span className="document-list-count">{t('ui.document.loadedRecords', { count: visibleRecords.length, total: totalCount ?? visibleRecords.length })}</span>
          </div>
          <div className="document-record-list" tabIndex={0} role="region" aria-label={t('ui.document.records')}>
            <table className="document-register" data-testid="document-register">
              <thead><tr>{columns.map((column) => <th scope="col" key={column} className={column === 'total' ? 'document-amount-cell' : ''}
                aria-sort={onSort && sortKey === column ? sortDirection === 'asc' ? 'ascending' : 'descending' : undefined}>
                {onSort && column !== 'status' ? <button type="button" disabled={disabled} onClick={() => requestNavigation(() => { resetEditing(); onSort(column, sortKey === column && sortDirection === 'asc' ? 'desc' : 'asc') })}>
                  {t('import.field.' + column)}{sortKey === column ? sortDirection === 'asc' ? <ArrowUp size={13} /> : <ArrowDown size={13} /> : <ArrowUpDown size={13} />}
                </button> : t('import.field.' + column)}
              </th>)}</tr></thead>
              <tbody>{visibleRecords.map((record) => {
              const key = String(record.key ?? record[definition.primaryKey])
              const state = recordStatus(record)
              const partner = String(record.CardCode ?? '')
              return <tr key={key} data-record-key={key} data-selected={!creating && selectedKey === key ? 'true' : undefined} onClick={() => { if (!disabled) select(key) }}>
                <td><button type="button" className="document-record-link" disabled={disabled} onClick={(event) => { event.stopPropagation(); select(key) }} aria-current={!creating && selectedKey === key ? 'true' : undefined}>{key}</button></td>
                {isDocument ? <>
                  <td><span className="document-partner-name">{String(record.CardName || lookupOptions.CardCode?.find((option) => option.value === partner)?.label || partner || '-')}</span></td>
                  <td className="document-date-cell">{String(record.DocDate ?? '').slice(0, 10) || '-'}</td>
                  <td><StatusBadge tone={statusTones[state]}>{t('ui.status.' + state)}</StatusBadge></td>
                  <td>{String(record.DocCur || record.Currency || '-')}</td>
                  <td className="document-amount-cell">{recordAmount(record, locale, false) || '-'}</td>
                  <td>{String(record.NumAtCard || '-')}</td>
                </> : <><td>{recordTitle(record)}</td><td><StatusBadge tone={statusTones[state]}>{t('ui.status.' + state)}</StatusBadge></td></>}
              </tr>
            })}</tbody>
            </table>
            {visibleRecords.length === 0 && <div className="ui-empty">
              {busy ? <Loader2 size={22} className="animate-spin" /> : <Search size={22} />}
              <strong>{t(busy ? 'common.loading' : 'ui.document.empty')}</strong>{!busy && <p>{t('ui.document.emptyHint')}</p>}
            </div>}
            {onLoadMore && <button type="button" className="ui-button ui-button-ghost w-full" disabled={disabled} onClick={onLoadMore}><ArrowDown size={15} />{t('ontology.loadMore')}</button>}
          </div>
        </aside>
        <div className="document-main" ref={detailRef} hidden={!detailsVisible}>
          {formError && !hasDialog && <FeedbackMessage error>{formError}</FeedbackMessage>}
          {notice && <FeedbackMessage>{notice}</FeedbackMessage>}
          {creating || selectedRecord ? <>
            <div className="document-context">
              <div className="min-w-0"><h4>{creating ? t('ui.document.new') : recordTitle(selectedRecord!)}</h4><p>{creating ? t('ui.document.requiredHint') : t('ui.document.recordNumber') + ': ' + selectedKey}</p></div>
              <div className="flex flex-wrap items-center gap-2">
                <StatusBadge tone={editingHeader ? 'amber' : statusTones[status]}>{t(editingHeader ? 'ui.document.editing' : 'ui.status.' + status)}</StatusBadge>
                {!creating && onOpenSource && <button type="button" className="ui-button ui-button-secondary" disabled={fieldsDisabled} onClick={() => requestNavigation(onOpenSource)}><FileInput size={15} />{t('import.sources')}</button>}
                {!editingHeader && onUpdateHeader && <button type="button" className="ui-button ui-button-secondary" disabled={fieldsDisabled || locked} title={locked ? t('ui.document.lockedHint') : undefined} onClick={() => { setEditing(true); setHeaderDirty(false); setFormError('') }}><Pencil size={15} />{t('ui.document.edit')}</button>}
                {!editingHeader && <ActionMenu label={t('ui.document.more')} icon={<MoreHorizontal size={16} />} testId="document-more-actions" items={[
                  { id: 'copy', label: t('ui.document.copyKey'), icon: <ClipboardCopy size={16} />, onSelect: () => void copyKey() },
                  ...(onDeleteHeader ? [{ id: 'delete', label: t('ui.document.delete'), icon: <Trash2 size={16} />, danger: true, disabled: fieldsDisabled || locked, onSelect: () => { setFormError(''); setDeleteTarget('header') } }] : []),
                ]} />}
              </div>
            </div>
            {locked && <div className="document-lock-notice"><LockKeyhole size={14} /><span><strong className="mr-2">{t('ontology.readOnly')}</strong>{t(readOnly ? 'ui.document.readOnlyHint' : 'ui.document.lockedHint')}</span></div>}
            <form
              key={(creating ? 'new' : selectedKey) + ':' + version + ':' + editing}
              data-testid="document-header-form"
              className="document-header-form"
              onSubmit={(event) => prepareSave(event, 'header')}
              onChange={(event) => {
                const data = collectFields(event.currentTarget, fields, creating, definition)
                setHeaderDirty(creating || fieldChanges(data, fields, selectedRecord, false).length > 0)
              }}
            >
              <div className="document-section-title"><h4>{t('ui.document.details')}</h4>{editingHeader && <small>{t('ui.document.requiredHint')}</small>}</div>
              <div className={'document-fields' + (editingHeader ? ' document-fields-edit' : '')}>
                {fields.filter((field) => !creating || !field.readOnly).map((field) => <FieldEditor key={field.name} field={field} value={creating ? undefined : selectedRecord?.[field.name]} definition={definition} locked={!editingHeader || locked || (!!field.primary && !creating)} disabled={fieldsDisabled} creating={creating} lookupOptions={lookupOptions} />)}
              </div>
              {editingHeader && <div className="document-savebar">
                <span>{headerDirty ? <Pencil size={14} /> : <FileText size={14} />}{t(headerDirty ? 'ui.document.unsaved' : 'ui.document.requiredHint')}</span>
                <div>
                  <button type="button" className="ui-button ui-button-secondary" disabled={submitting} onClick={() => requestNavigation(resetEditing)}>{t('common.cancel')}</button>
                  <button type="submit" className="ui-button ui-button-primary" disabled={fieldsDisabled || locked || (!creating && !headerDirty)}><Save size={16} />{t(creating ? 'ontology.create' : 'ui.document.save')}</button>
                </div>
              </div>}
            </form>
            {!creating && <>
              {definition.actions.length > 0 && <div className="document-actions" role="group" aria-label={t('ui.document.availableActions')}>
                <span>{t('ui.document.actions')}</span>
                {definition.actions.map((action) => <button key={action.id} type="button" className={'ui-button ' + (action.id === firstAvailableAction?.id ? 'ui-button-primary' : 'ui-button-secondary')}
                  title={t(editingHeader ? 'ui.document.saveBeforeAction' : action.disabledReasonKey ?? 'ui.document.actionHint')}
                  disabled={fieldsDisabled || editingHeader || !!action.disabledReasonKey}
                  onClick={() => { setFormError(''); setActionID(action.id) }}>
                  {['approve', 'confirm'].includes(action.id) ? <CheckCircle2 size={15} /> : <Play size={14} />}{t(action.labelKey)}
                </button>)}
              </div>}
              <nav className="document-tabs" role="tablist" aria-label={t('ontology.recordViews')} onKeyDown={tabKeys}>
                {(['detail', 'links', 'history'] as const).map((id) => <button key={id} type="button" role="tab" aria-label={t('ontology.tab.' + id)} id={tabsID + '-' + id} aria-controls={tabsID + '-panel'} aria-selected={tab === id} tabIndex={tab === id ? 0 : -1} onClick={() => changeTab(id)}>
                  {id === 'links' ? <Link2 size={15} /> : id === 'history' ? <Clock3 size={15} /> : <FileText size={15} />}{t('ontology.tab.' + id)}
                  <small>{id === 'detail' ? childRows.length : id === 'links' ? links.length : history.length}</small>
                </button>)}
              </nav>
              <div id={tabsID + '-panel'} role="tabpanel" aria-labelledby={tabsID + '-' + tab} className="document-tab-panel">
                {tab === 'detail' && <>
                  <div className="document-section-title"><h4>{t('workbench.detail')}<small>{t('ui.document.lineCount', { count: childRows.length })}</small></h4>
                    {onCreateLine && <button type="button" className="ui-button ui-button-secondary" disabled={fieldsDisabled || locked || editingHeader} onClick={() => openLine('new')}><Plus size={16} />{t('ui.document.addLine')}</button>}
                  </div>
                  {detail && childRows.length > 0 ? <div className="document-table-scroll" tabIndex={0} aria-label={t('workbench.detail')}>
                    <table className="document-table">
                      <thead><tr>{lineFields.map((field) => <th key={field.name} scope="col" data-numeric={isNumeric(field)}>{t(field.labelKey)}</th>)}{onUpdateLine && <th scope="col" className="document-table-actions">{t('common.actions')}</th>}</tr></thead>
                      <tbody>{childRows.map((row) => {
                        const key = String(row.key ?? row[detail.lineKey])
                        return <tr key={key}>
                          {lineFields.map((field) => <td key={field.name} data-numeric={isNumeric(field)} title={resolveFieldCapability(field, definition.fieldPermissions).masked ? '***' : formatValue(field, row[field.name], t)}>
                            {resolveFieldCapability(field, definition.fieldPermissions).masked ? '***' : formatValue(field, row[field.name], t) || '—'}
                          </td>)}
                          {onUpdateLine && <td className="document-table-actions"><button type="button" className="ui-button ui-button-link" disabled={fieldsDisabled || locked || editingHeader} aria-label={t('ui.document.editLine') + ' ' + key} onClick={() => openLine(key)}><Pencil size={14} />{t('ontology.edit')}</button></td>}
                        </tr>
                      })}</tbody>
                    </table>
                  </div> : <div className="ui-empty"><FileText size={23} /><strong>{t('ui.document.noLines')}</strong><p>{t('ui.document.noLinesHint')}</p></div>}
                  {childRows.length === 500 && <p className="mt-3 text-xs ui-muted">{t('ontology.linesLimit')}</p>}
                </>}
                {tab === 'links' && <>
                  {links.map((link) => <button key={link.type + ':' + link.object.type + ':' + link.object.key} type="button" className="document-link-row" onClick={() => requestNavigation(() => onOpenLink(link.object.table_code, link.object.key))}>
                    <Link2 size={18} /><span className="min-w-0 flex-1"><strong>{link.object.title}</strong><small>{link.label[locale]} · {link.object.key}</small></span><ArrowRight size={16} className="shrink-0" />
                  </button>)}
                  {links.length === 0 && <div className="ui-empty"><Link2 size={23} /><strong>{t('ontology.noLinks')}</strong><p>{t('ui.document.noLinksHint')}</p></div>}
                  {linksTruncated && <p className="text-xs ui-muted">{t('ontology.linksLimit')}</p>}
                </>}
                {tab === 'history' && <>
                  <ol className="document-history">{history.map((event) => <li key={event.id}>
                    <div className="flex flex-wrap items-center justify-between gap-2"><h4>{t('erp.action.' + event.action)}</h4><StatusBadge tone={event.status === 'failed' ? 'red' : 'green'}>{t('ontology.execution.' + event.status)}</StatusBadge></div>
                    <p>{event.started_at ? new Date(event.started_at).toLocaleString(locale === 'zh' ? 'zh-CN' : 'en-US') : '—'}</p>
                    {event.failure_message && <FeedbackMessage error>{event.failure_message}</FeedbackMessage>}
                    {(event.generated_records ?? []).map((record) => <button key={record.line_num} type="button" className="ui-button ui-button-link mt-2 max-w-full" onClick={() => requestNavigation(() => onOpenLink(record.generated_table_code, record.generated_key))}><Link2 size={14} /><span className="truncate">{record.generated_key}</span><ArrowRight size={14} /></button>)}
                    <details><summary>{t('ui.document.viewDetails')}</summary><p>{event.id}</p><p>{event.actor_id || event.actor_type}</p></details>
                  </li>)}</ol>
                  {history.length === 0 && <div className="ui-empty"><Clock3 size={23} /><strong>{t('erp.business.noTimeline')}</strong><p>{t('ui.document.noHistoryHint')}</p></div>}
                </>}
              </div>
            </>}
          </> : <div className="ui-empty min-h-96">
            {detailLoading || busy ? <Loader2 size={26} className="animate-spin" /> : <FileText size={26} />}
            <strong>{t(detailLoading || busy ? 'common.loading' : 'ui.document.noSelection')}</strong>{!detailLoading && !busy && <p>{t('ui.document.noSelectionHint')}</p>}
          </div>}
        </div>
      </div>

      <Dialog open={!!lineMode && !locked} title={t(lineMode === 'new' ? 'ui.document.addLine' : 'ui.document.editLine')} description={t('ui.document.lineHint')} onClose={closeLine} busy={submitting} size="lg"
        footer={<>
          {lineMode !== 'new' && onDeleteLine && <button type="button" className="ui-button ui-button-ghost mr-auto" disabled={fieldsDisabled} onClick={() => { setFormError(''); setDeleteTarget('line') }}><Trash2 size={16} />{t('ontology.deleteLine')}</button>}
          <button type="button" className="ui-button ui-button-secondary" data-dialog-cancel disabled={submitting} onClick={closeLine}>{t('common.cancel')}</button>
          <button type="submit" form={lineFormID} className="ui-button ui-button-primary" disabled={fieldsDisabled || (lineMode !== 'new' && !lineDirty)}><Save size={16} />{t(lineMode === 'new' ? 'ui.document.addLine' : 'ontology.saveLine')}</button>
        </>}>
        {contextCard()}
        <form id={lineFormID} key={lineMode + ':' + version} data-testid="document-line-form" onSubmit={(event) => prepareSave(event, 'line')} onChange={(event) => {
          const isNew = lineMode === 'new'
          setLineDirty(isNew || fieldChanges(collectFields(event.currentTarget, lineFields, isNew, definition), lineFields, line, false).length > 0)
        }}>
          <div className="document-fields document-fields-edit">{lineFields.map((field) => <FieldEditor key={field.name} field={field} definition={definition} value={lineMode === 'new' ? (field.primary ? nextLine : undefined) : line?.[field.name]} locked={!!field.primary && lineMode !== 'new'} disabled={fieldsDisabled} creating={lineMode === 'new'} lookupOptions={lookupOptions} />)}</div>
        </form>
        {formError && !pendingSave && !deleteTarget && <FeedbackMessage error>{formError}</FeedbackMessage>}
      </Dialog>

      <Dialog open={!!pendingSave} title={t(pendingSave?.creating ? 'ui.document.reviewCreate' : 'ui.document.reviewChanges')} description={t('ui.document.reviewHint')} onClose={() => { setPendingSave(null); setFormError('') }} busy={submitting} size="lg"
        footer={<>
          <button type="button" className="ui-button ui-button-secondary" data-dialog-cancel disabled={submitting} onClick={() => { setPendingSave(null); setFormError('') }}>{t('ui.unsaved.keepEditing')}</button>
          <button type="button" className="ui-button ui-button-primary" disabled={fieldsDisabled || locked} onClick={saveConfirmed}>{submitting ? <Loader2 size={16} className="animate-spin" /> : <Check size={16} />}{t(submitting ? 'ui.document.processing' : pendingSave?.creating ? 'ui.document.createConfirm' : 'ui.document.saveConfirm')}</button>
        </>}>
        {pendingSave && <>
          {contextCard(pendingSave.kind === 'header' ? pendingSave.key : selectedKey)}
          {pendingSave.kind === 'line' && <p className="mb-3 text-xs ui-muted">{t('ui.document.lineLabel', { number: pendingSave.key })}</p>}
          {!pendingSave.creating && <p className="mb-3 text-xs ui-muted">{t('ui.document.changesCount', { count: pendingSave.changes.length })}</p>}
          <table className="document-diff">
            <thead><tr><th scope="col">{t('ui.document.field')}</th>{!pendingSave.creating && <th scope="col">{t('ui.document.before')}</th>}<th scope="col">{t(pendingSave.creating ? 'ui.document.value' : 'ui.document.after')}</th></tr></thead>
            <tbody>{pendingSave.changes.map(({ field, before, after }) => <tr key={field.name}><td>{t(field.labelKey)}</td>{!pendingSave.creating && <td>{formatValue(field, before, t) || t('ui.document.noValue')}</td>}<td>{formatValue(field, after, t) || t('ui.document.noValue')}</td></tr>)}</tbody>
          </table>
          {formError && <FeedbackMessage error>{formError}</FeedbackMessage>}
        </>}
      </Dialog>

      <Dialog open={!!activeAction || !!deleteTarget} title={t(deleteTarget ? deleteTarget === 'header' ? 'ui.document.deleteTitle' : 'ui.document.lineDeleteTitle' : activeAction?.labelKey ?? 'ui.document.actions')}
        description={deleteTarget ? t('ui.document.deleteHint') : t('ui.document.actionHint')} danger={!!deleteTarget} onClose={() => { setActionID(''); setDeleteTarget(null); setFormError('') }} busy={submitting}
        footer={<>
          <button type="button" className="ui-button ui-button-secondary" data-dialog-cancel disabled={submitting} onClick={() => { setActionID(''); setDeleteTarget(null); setFormError('') }}>{t('common.cancel')}</button>
          <button type="submit" form={actionFormID} className={'ui-button ' + (deleteTarget ? 'ui-button-danger' : 'ui-button-primary')}
            disabled={fieldsDisabled || !selectedRecord || (!!deleteTarget && locked) || (!deleteTarget && !!activeAction?.disabledReasonKey)}>
            {submitting ? <Loader2 size={16} className="animate-spin" /> : deleteTarget ? <Trash2 size={16} /> : <CheckCircle2 size={16} />}{t(submitting ? 'ui.document.processing' : deleteTarget ? 'ontology.confirmDelete' : 'ontology.confirm')}
          </button>
        </>}>
        {contextCard()}
        {deleteTarget === 'line' && <p className="mb-4 text-sm ui-secondary">{t('ui.document.lineLabel', { number: lineMode ?? '' })}</p>}
        {!deleteTarget && <p className="document-action-impact">{t('ui.document.actionImpact.' + (['approve', 'post', 'submit'].includes(actionID) ? actionID : 'default'))}</p>}
        <dl className="document-action-summary">
          {selectedRecord && ['DocDate', 'CardCode', 'DocTotal', 'Currency', 'DocCur'].map((name) => {
            const field = fields.find((item) => item.name === name)
            if (!field || selectedRecord[name] == null || resolveFieldCapability(field, definition.fieldPermissions).masked) return null
            return <div key={name}><dt>{t(field.labelKey)}</dt><dd>{formatValue(field, selectedRecord[name], t) || '—'}</dd></div>
          })}
        </dl>
        <form id={actionFormID} onSubmit={(event) => {
          event.preventDefault()
          if (fieldsDisabled || !selectedRecord) return
          if (deleteTarget && !locked) {
            const handler = deleteTarget === 'header' ? onDeleteHeader : onDeleteLine
            const key = deleteTarget === 'header' ? selectedKey : lineMode
            if (!handler || !key) return
            void submit(() => handler(key), () => { resetEditing() })
          } else if (activeAction && !activeAction.disabledReasonKey) {
            const data = collectFields(event.currentTarget, activeAction.parameters ?? [], true, definition)
            void submit(() => onExecute(activeAction.id, data), () => setActionID(''))
          }
        }}>
          <div className="document-fields">{activeAction?.parameters?.map((field) => <FieldEditor key={field.name} field={field} definition={definition} creating disabled={submitting} lookupOptions={lookupOptions} />)}</div>
        </form>
        {formError && <FeedbackMessage error>{formError}</FeedbackMessage>}
      </Dialog>
    </section>
  )
}

function FieldEditor({ field, value, definition, locked, disabled, creating, lookupOptions }: {
  field: DocumentWorkbenchField
  value?: unknown
  definition: DocumentWorkbenchDefinition
  locked?: boolean
  disabled?: boolean
  creating: boolean
  lookupOptions?: WorkbenchLookupOptions
}) {
  const { t } = useI18n()
  const fieldID = useId()
  const capability = resolveFieldCapability(field, definition.fieldPermissions)
  const readOnly = locked || !capability.writable
  const initial = initialValue(field, value, creating)
  const candidates = field.primary ? undefined : lookupOptions?.[field.name]
  const candidate = candidates?.find((item) => item.value === initial)
  const label = <span id={fieldID + '-label'} className="ui-field-label">{t(field.labelKey)}{field.required && !readOnly && <em aria-hidden="true">*</em>}</span>
  if (readOnly) return <div className="ui-field">{label}<output data-testid={'field-value-' + field.name} aria-labelledby={fieldID + '-label'} className="ui-field-value block">{capability.masked ? '***' : candidate?.label || formatValue(field, initial, t) || '—'}</output></div>
  if (field.dataType === 'boolean') return <label className="ui-checkbox"><input id={fieldID} name={field.name} type="checkbox" value="Y" defaultChecked={initial === 'Y'} disabled={disabled} /><span>{t(field.labelKey)}</span></label>
  if (field.options) return <label className="ui-field">{label}<select id={fieldID} name={field.name} defaultValue={initial} required={field.required} disabled={disabled} className="ui-select">{field.options.map((option) => <option key={option.value} value={option.value}>{t(option.labelKey)}</option>)}</select></label>
  if (candidates?.length) return <label className="ui-field">{label}
    <input id={fieldID} name={field.name} list={fieldID + '-options'} defaultValue={initial} required={field.required} disabled={disabled} className="ui-input" autoComplete="off" aria-describedby={fieldID + '-hint'} />
    <datalist id={fieldID + '-options'}>{candidates.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}</datalist>
    <small id={fieldID + '-hint'} className="mt-1 block text-[11px] ui-muted">{t('ui.document.lookupHint')}</small>
  </label>
  if (['Comments', 'Memo', 'Description'].includes(field.name)) return <label className="ui-field">{label}<textarea id={fieldID} name={field.name} defaultValue={initial} required={field.required} disabled={disabled} className="ui-input min-h-20 resize-y" /></label>
  const numeric = isNumeric(field)
  return <label className="ui-field">{label}<input id={fieldID} name={field.name} type={numeric ? 'number' : field.dataType === 'date' ? 'date' : field.dataType === 'email' ? 'email' : 'text'} defaultValue={initial} required={field.required} disabled={disabled}
    min={numeric ? (field.name === 'Quantity' ? 0.000001 : 0) : undefined} max={field.name === 'TaxRate' ? 100 : undefined} step={numeric ? (field.dataType === 'integer' ? 1 : '0.000001') : undefined} className="ui-input" /></label>
}

function collectFields(form: HTMLFormElement, fields: DocumentWorkbenchField[], creating: boolean, definition: DocumentWorkbenchDefinition) {
  const values = new FormData(form)
  const data: Record<string, unknown> = {}
  for (const field of fields) {
    if (!resolveFieldCapability(field, definition.fieldPermissions).writable || (field.primary && !creating)) continue
    if (field.dataType === 'boolean') { data[field.name] = values.has(field.name) ? 'Y' : 'N'; continue }
    if (!values.has(field.name)) continue
    const value = String(values.get(field.name) ?? '').trim()
    if (creating && value === '' && !field.required) continue
    data[field.name] = normalizeValue(field, value)
  }
  return data
}

function fieldChanges(data: Record<string, unknown>, fields: DocumentWorkbenchField[], record: WorkbenchRecord | undefined, creating: boolean): FieldChange[] {
  return fields.flatMap((field) => {
    if (!(field.name in data)) return []
    const before = normalizeValue(field, initialValue(field, record?.[field.name], false))
    const after = data[field.name]
    return creating || before !== after ? [{ field, before, after }] : []
  })
}

function normalizeValue(field: DocumentWorkbenchField, value: unknown) {
  const text = displayValue(value).trim()
  if (isNumeric(field)) return Number(text)
  if (field.dataType === 'boolean') return ['Y', 'true', '1'].includes(text) ? 'Y' : 'N'
  return ['Currency', 'DocCur'].includes(field.name) ? text.toUpperCase() : text
}

function initialValue(field: DocumentWorkbenchField, value: unknown, creating: boolean) {
  const initial = value === undefined ? (creating ? field.defaultValue ?? '' : '') : displayValue(value)
  if (field.dataType === 'date') return initial.slice(0, 10)
  if (field.dataType === 'boolean') return ['Y', 'true', '1'].includes(initial) ? 'Y' : 'N'
  return initial
}

function formatValue(field: DocumentWorkbenchField, value: unknown, t: (key: string) => string) {
  const text = displayValue(value)
  if (field.dataType === 'boolean') return t(['Y', 'true', '1'].includes(text) ? 'common.yes' : 'common.no')
  const option = field.options?.find((item) => item.value === text)
  if (option) return t(option.labelKey)
  const states: Record<string, Record<string, string>> = {
    DocStatus: { O: 'draft', S: 'submitted', C: 'closed' },
    WddStatus: { A: 'approved', W: 'pending', N: 'draft', '-': 'draft' },
    BtfStatus: { O: 'draft', P: 'posted', V: 'void' }, Posted: { Y: 'posted', N: 'unposted' },
  }
  const status = states[field.name]?.[text]
  return status ? t('ontology.status.' + status) : field.dataType === 'date' ? text.slice(0, 10) : text
}

export function recordIsImmutable(record?: WorkbenchRecord) {
  return !!record && (record.Posted === 'Y' || record.BtfStatus === 'P' || record.BtfStatus === 'V' || record.WddStatus === 'A' || record.DocStatus === 'C' || !!record.BaseEntry || Number(record.AllocatedAmount) > 0)
}

function recordStatus(record?: WorkbenchRecord): RecordStatus {
  if (record?.BtfStatus === 'V') return 'void'
  if (record?.Posted === 'Y' || record?.BtfStatus === 'P') return 'posted'
  if (record?.DocStatus === 'C' || record?.Status === 'closed') return 'closed'
  if (record?.WddStatus === 'A' || record?.Status === 'approved') return 'approved'
  if (record?.DocStatus === 'S' || record?.WddStatus === 'W' || ['submitted', 'pending'].includes(String(record?.Status))) return 'pending'
  if (record?.Inactive === 'Y' || record?.Active === 'N' || record?.ValidFor === 'N' || record?.validFor === 'N') return 'inactive'
  if (record?.Active === 'Y' || record?.ValidFor === 'Y' || record?.validFor === 'Y' || record?.Status === 'active') return 'active'
  return 'draft'
}

function recordTitle(record: WorkbenchRecord) {
  return displayValue(record.CardName || record.ItemName || record.WhsName || record.Name || record.Memo || record.DocNum || record.key)
}

function recordAmount(record: WorkbenchRecord, locale: string, includeCurrency = true) {
  if (record.DocTotal == null) return ''
  const amount = Number(record.DocTotal)
  if (!Number.isFinite(amount)) return ''
  const currency = displayValue(record.DocCur || record.Currency)
  return new Intl.NumberFormat(locale === 'zh' ? 'zh-CN' : 'en-US', { maximumFractionDigits: 2, minimumFractionDigits: 2 }).format(amount) + (includeCurrency && currency ? ' ' + currency : '')
}

function isNumeric(field: DocumentWorkbenchField) {
  return field.dataType === 'number' || field.dataType === 'integer'
}

function displayValue(value: unknown) {
  return value == null ? '' : typeof value === 'object' ? JSON.stringify(value) : String(value)
}
