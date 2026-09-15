'use client'

import { AlertTriangle, ArrowDown, ArrowLeft, Check, CheckCircle2, Download, FileInput, FileText, History, Image as ImageIcon, Loader2, Plus, RefreshCw, Save, ScanLine, Trash2, Upload, X } from 'lucide-react'
import { useCallback, useEffect, useId, useRef, useState } from 'react'

import { APIError, describeApiError } from '@/lib/api-error'
import {
  confirmDocumentImport, getDocumentImport, getDocumentImportFile, listDocumentImports, recognizeDocumentImport,
  rejectDocumentImport, saveDocumentImport, uploadDocumentImport,
  type DocumentImport, type ImportDraft, type ImportEvidence, type ImportSource,
} from '@/lib/document-import'
import { useI18n } from '@/lib/i18n'
import { type WorkbenchLookupOptions } from './document-workbench'
import { DocumentPdfPreview } from './document-pdf-preview'
import { Dialog, FeedbackMessage, StatusBadge, useUnsavedChanges, useWorkspaceInteraction } from './workspace-ui'

const headerFields = ['external_number', 'partner', 'date', 'due_date', 'currency', 'total', 'tax', 'note']
const lineFields = ['item', 'description', 'warehouse', 'quantity', 'unit_price', 'tax_rate']
const lookupFields: Record<string, string> = { partner: 'CardCode', item: 'ItemCode', warehouse: 'WhsCode' }
const numericFields = new Set(['total', 'tax', 'quantity', 'unit_price', 'tax_rate'])
const blankDraft = (): ImportDraft => ({ key: '', properties: {}, lines: [] })
const tones = { uploaded: 'neutral', recognizing: 'blue', needs_review: 'amber', confirmed: 'green', rejected: 'neutral', failed: 'red' } as const

function copyDraft(value: ImportDraft): ImportDraft {
  return {
    key: value.key ?? '',
    properties: Object.fromEntries(Object.entries(value.properties ?? {}).map(([key, item]) => [key, item == null ? '' : String(item)])),
    lines: (value.lines ?? []).map((line) => Object.fromEntries(Object.entries(line).map(([key, item]) => [key, item == null ? '' : String(item)]))),
  }
}

export function DocumentImportWorkspace({ token, objectType, title, initialID, lookupOptions, onClose, onConfirmed }: {
  token: string; objectType: string; title: string; initialID?: string; lookupOptions: WorkbenchLookupOptions
  onClose: () => void; onConfirmed: (key: string) => void
}) {
  const { t, locale } = useI18n()
  const { requestNavigation } = useWorkspaceInteraction()
  const [imports, setImports] = useState<DocumentImport[]>([])
  const [inboxStatus, setInboxStatus] = useState('')
  const [cursor, setCursor] = useState('')
  const [nextCursor, setNextCursor] = useState('')
  const [version, setVersion] = useState(0)
  const [loading, setLoading] = useState(true)
  const [item, setItem] = useState<DocumentImport | null>(null)
  const [draft, setDraft] = useState<ImportDraft>(blankDraft)
  const [files, setFiles] = useState<File[]>([])
  const [activeSource, setActiveSource] = useState('')
  const [busy, setBusy] = useState(false)
  const [phase, setPhase] = useState('')
  const [error, setError] = useState('')
  const [issues, setIssues] = useState<Array<{ path: string; code: string }>>([])
  const [dirty, setDirty] = useState(false)
  const [confirmation, setConfirmation] = useState<'confirm' | 'reject' | null>(null)
  const [acknowledged, setAcknowledged] = useState(false)
  const [sourceTab, setSourceTab] = useState<'source' | 'evidence' | 'history'>('source')
  const [recognitionExpired, setRecognitionExpired] = useState(false)
  const operationLock = useRef(false)
  const fileInput = useRef<HTMLInputElement>(null)
  const fieldID = useId()
  const isPayment = objectType === 'incoming_payment' || objectType === 'outgoing_payment'
  const editable = !!item && ['uploaded', 'needs_review', 'failed'].includes(item.status)
  const disabled = busy || !editable
  const selectedFile = item?.files.find((file) => file.id === activeSource) ?? item?.files[0]
  useUnsavedChanges(dirty || files.length > 0)

  const adopt = useCallback((record: DocumentImport) => {
    setItem(record)
    setRecognitionExpired(record.status === 'recognizing' && (!record.recognition_started_at || Date.now() - new Date(record.recognition_started_at).getTime() >= 180_000))
    setDraft(copyDraft(record.draft))
    setActiveSource((current) => record.files.some((file) => file.id === current) ? current : record.files[0]?.id ?? '')
    setDirty(false)
  }, [])

  useEffect(() => {
    let cancelled = false
    listDocumentImports(token, objectType, inboxStatus, cursor)
      .then((page) => {
        if (cancelled) return
        setImports((previous) => cursor ? [...previous, ...(page.imports ?? [])] : page.imports ?? [])
        setNextCursor(page.next_cursor ?? '')
      })
      .catch((err) => { if (!cancelled) setError(describeApiError(err, t, t('import.error.operation_failed'))) })
      .finally(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [token, objectType, inboxStatus, cursor, version, t])

  useEffect(() => {
    if (!initialID) return
    let cancelled = false
    getDocumentImport(token, initialID).then((record) => { if (!cancelled) adopt(record) })
      .catch((err) => { if (!cancelled) setError(describeApiError(err, t)) })
    return () => { cancelled = true }
  }, [token, initialID, adopt, t])

  useEffect(() => {
    if (!item || item.status !== 'recognizing' || busy) return
    const controller = { cancelled: false }
    const timer = window.setInterval(() => {
      getDocumentImport(token, item.id).then((record) => { if (!controller.cancelled) adopt(record) })
        .catch(() => { if (!controller.cancelled) setError(t('import.error.operation_failed')) })
    }, 3000)
    return () => { controller.cancelled = true; window.clearInterval(timer) }
  }, [token, item, busy, adopt, t])

  async function run(task: () => Promise<void>) {
    if (operationLock.current) return
    operationLock.current = true
    setBusy(true)
    setError('')
    setIssues([])
    try {
      await task()
      setLoading(true)
      setCursor('')
      setVersion((value) => value + 1)
    } catch (err) {
      setError(describeApiError(err, t, t('import.error.operation_failed')))
      if (err instanceof APIError) setIssues(err.issues)
    } finally {
      operationLock.current = false
      setBusy(false)
      setPhase('')
    }
  }

  function chooseFiles(incoming: FileList | File[] | null) {
    if (!incoming) return
    const next = [...files, ...Array.from(incoming)]
    setError('')
    if (next.length > 5) { setError(t('import.issue.file_count')); return }
    if (next.some((file) => file.size === 0 || file.size > 10 * 1024 * 1024) || next.reduce((sum, file) => sum + file.size, 0) > 20 * 1024 * 1024) {
      setError(t('import.issue.file_size')); return
    }
    setFiles(next)
  }

  function beginUpload() {
    if (files.length === 0) return
    void run(async () => {
      setPhase('uploading')
      const uploaded = await uploadDocumentImport(token, objectType, files)
      setFiles([])
      adopt(uploaded)
      if (uploaded.status === 'uploaded') {
        setPhase('recognizing')
        adopt(await recognizeDocumentImport(token, uploaded))
      }
    })
  }

  function selectImport(id: string) {
    requestNavigation(() => { void run(async () => { setFiles([]); adopt(await getDocumentImport(token, id)) }) })
  }

  function updateField(field: string, value: string, line?: number) {
    setDraft((current) => line === undefined
      ? field === 'key' ? { ...current, key: value } : { ...current, properties: { ...current.properties, [field]: value } }
      : { ...current, lines: current.lines.map((row, index) => index === line ? { ...row, [field]: value } : row) })
    setDirty(true)
    setAcknowledged(false)
  }

  function fieldLabel(path: string) {
    const [group, position, field] = path.split('.')
    if (group === 'lines' && field) return t('import.line', { number: Number(position) + 1 }) + ' / ' + t('import.field.' + field)
    if (group === 'properties') return t('import.field.' + position)
    if (group === 'key') return t('import.field.key')
    return t('import.field.' + group)
  }

  function evidenceFor(path: string) {
    return item?.extraction?.evidence?.filter((evidence) => evidence.path === path).sort((left, right) => left.confidence - right.confidence)[0]
  }

  function fieldEditor(field: string, line?: number) {
    const path = line === undefined ? field === 'key' ? 'key' : 'properties.' + field : `lines.${line}.${field}`
    const id = `${fieldID}-${path}`
    const value = line === undefined ? field === 'key' ? draft.key : draft.properties[field] ?? '' : draft.lines[line]?.[field] ?? ''
    const original = item?.extraction?.draft
    const matchesOriginal = line === undefined
      ? value === String(original?.properties?.[field] ?? '')
      : lineFields.every((key) => String(original?.lines?.[line]?.[key] ?? '') === (draft.lines[line]?.[key] ?? ''))
    const evidence = matchesOriginal ? evidenceFor(path) : undefined
    const options = lookupOptions[lookupFields[field]]
    const fieldIssues = issues.filter((issue) => issue.path === path)
    return <div className="import-field" data-low-confidence={evidence && evidence.confidence < 0.8 ? 'true' : undefined}>
      {line === undefined && <label htmlFor={id}>{t('import.field.' + field)}</label>}
      <input id={id} name={path} aria-label={line === undefined ? t('import.field.' + field) : `${t('import.line', { number: line + 1 })} ${t('import.field.' + field)}`}
        type={numericFields.has(field) ? 'number' : field === 'date' || field === 'due_date' ? 'date' : 'text'}
        inputMode={numericFields.has(field) ? 'decimal' : undefined} step={numericFields.has(field) ? '0.000001' : undefined}
        min={numericFields.has(field) ? 0 : undefined} max={field === 'tax_rate' ? 100 : undefined}
        maxLength={field === 'note' ? 2000 : field === 'description' ? 500 : 128}
        className="ui-input" value={value} disabled={disabled} list={options ? id + '-options' : undefined}
        onChange={(event) => updateField(field, event.target.value, line)} aria-invalid={fieldIssues.length > 0}
        aria-describedby={fieldIssues.length ? id + '-error' : undefined} />
      {options && <datalist id={id + '-options'}>{options.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</datalist>}
      {evidence && <button type="button" className="import-evidence-link" title={evidence.quote} onClick={() => { setActiveSource(evidence.source_id); setSourceTab('source') }}>
        {evidence.confidence < 0.8 ? <AlertTriangle size={12} /> : <ScanLine size={12} />}{t('import.confidence', { value: Math.round(evidence.confidence * 100) })}
      </button>}
      {fieldIssues.length > 0 && <small id={id + '-error'} className="import-field-error">{fieldIssues.map((issue) => t('import.issue.' + issue.code)).join(' ')}</small>}
    </div>
  }

  const corrections = draftChanges(item?.extraction?.draft, draft)
  const statusBadge = (record: DocumentImport) => <StatusBadge tone={tones[record.status]}>{t('import.status.' + record.status)}</StatusBadge>
  const close = () => { if (!busy) requestNavigation(onClose) }

  return <>
    <Dialog open title={t('import.title') + ' / ' + title} onClose={close} busy={busy} size="lg" className="document-import-dialog"
      footer={item ? <>
        <span className="import-review-state">{statusBadge(item)}</span>
        {item.status === 'confirmed' && <button type="button" className="ui-button ui-button-primary" onClick={() => { onConfirmed(item.confirmed_key!); onClose() }}><FileInput size={16} />{t('import.openDocument')}</button>}
        {editable && <>
          <button type="button" className="ui-button ui-button-ghost" disabled={busy} onClick={() => { setConfirmation('reject'); setError('') }}><X size={16} />{t('import.reject')}</button>
          <button type="button" className="ui-button ui-button-secondary" disabled={busy || !dirty} onClick={() => void run(async () => { adopt(await saveDocumentImport(token, item, draft)) })}><Save size={16} />{t('import.saveReview')}</button>
          <button type="button" className="ui-button ui-button-primary" disabled={busy} onClick={() => { setAcknowledged(false); setConfirmation('confirm'); setError('') }}><CheckCircle2 size={16} />{t('import.confirm')}</button>
        </>}
      </> : <button type="button" className="ui-button ui-button-primary" disabled={busy || !files.length} onClick={beginUpload}>{busy ? <Loader2 size={16} className="animate-spin" /> : <ScanLine size={16} />}{t('import.uploadRecognize')}</button>}
    >
      <div data-testid="document-import-workspace" aria-busy={busy}>
        <div className="import-toolbar">
          {item ? <button type="button" className="ui-button ui-button-ghost" disabled={busy} onClick={() => requestNavigation(() => { setItem(null); setDirty(false); setError(''); setIssues([]) })}><ArrowLeft size={16} />{t('import.inbox')}</button>
            : <h3>{t('import.inbox')}</h3>}
          <div className="flex flex-wrap items-center gap-2">
            {item && (editable || recognitionExpired) && <button type="button" className="ui-button ui-button-secondary" disabled={busy} onClick={() => requestNavigation(() => { void run(async () => { setPhase('recognizing'); adopt(await recognizeDocumentImport(token, item)) }) })}><ScanLine size={16} />{t('import.recognize')}</button>}
            <button type="button" className="ui-icon-button" disabled={busy} title={t('common.refresh')} aria-label={t('common.refresh')} onClick={() => requestNavigation(() => {
              void run(async () => { if (item) adopt(await getDocumentImport(token, item.id)) })
            })}><RefreshCw size={16} /></button>
          </div>
        </div>
        {error && !confirmation && <FeedbackMessage error>{error}</FeedbackMessage>}
        {issues.length > 0 && !confirmation && <ul className="import-issues">{issues.map((issue, index) => <li key={index}>{fieldLabel(issue.path)}: {t('import.issue.' + issue.code)}</li>)}</ul>}
        {(phase || item?.status === 'recognizing') && <div className="import-progress" role="status">{recognitionExpired && !phase ? <><AlertTriangle size={18} />{t('import.error.recognition_timeout')}</> : <><Loader2 size={18} className="animate-spin" />{t('import.status.' + (phase || 'recognizing'))}</>}</div>}
        {!item ? <>
          <div className="import-upload-zone" onDragOver={(event) => { event.preventDefault() }} onDrop={(event) => { event.preventDefault(); if (!busy) chooseFiles(event.dataTransfer.files) }}>
            <Upload size={26} aria-hidden="true" />
            <button type="button" className="ui-button ui-button-secondary" disabled={busy} onClick={() => fileInput.current?.click()}>{t('import.chooseFiles')}</button>
            <span>{t('import.fileLimits')}</span>
            <input ref={fileInput} data-testid="import-file-input" aria-label={t('import.chooseFiles')} className="sr-only" type="file" multiple accept=".png,.jpg,.jpeg,.webp,.pdf,.docx,.txt,.csv"
              onChange={(event) => { chooseFiles(event.target.files); event.target.value = '' }} disabled={busy} />
          </div>
          {files.length > 0 && <ul className="import-selected-files">{files.map((file, index) => <li key={index}><FileText size={16} /><span>{file.name}</span><small>{fileSize(file.size, locale)}</small>
            <button type="button" className="ui-icon-button" disabled={busy} title={t('import.removeFile')} aria-label={t('import.removeFile') + ': ' + file.name} onClick={() => setFiles((current) => current.filter((_, position) => position !== index))}><X size={15} /></button>
          </li>)}</ul>}
          <div className="import-inbox-heading"><h3>{t('import.recent')}</h3><select className="ui-select" value={inboxStatus} aria-label={t('ui.document.filter')} onChange={(event) => { setLoading(true); setCursor(''); setInboxStatus(event.target.value) }}>
            <option value="">{t('ui.document.allStatuses')}</option>{Object.keys(tones).map((status) => <option key={status} value={status}>{t('import.status.' + status)}</option>)}
          </select></div>
          <div className="import-inbox-table"><table className="document-register"><thead><tr>{['file', 'status', 'created_at'].map((field) => <th key={field}>{t('import.field.' + field)}</th>)}</tr></thead>
            <tbody>{imports.map((record) => <tr key={record.id}><td><button type="button" className="document-record-link" disabled={busy} onClick={() => selectImport(record.id)}>{record.files[0]?.name ?? record.draft.key}</button><small>{record.draft.properties?.external_number ?? ''}</small></td><td>{statusBadge(record)}</td><td>{formatDate(record.created_at, locale)}</td></tr>)}</tbody>
          </table></div>
          {!imports.length && <div className="ui-empty">{loading ? <Loader2 size={22} className="animate-spin" /> : <FileInput size={22} />}<strong>{t(loading ? 'common.loading' : 'import.empty')}</strong></div>}
          {nextCursor && <button type="button" className="ui-button ui-button-ghost" disabled={loading} onClick={() => { setLoading(true); setCursor(nextCursor) }}><ArrowDown size={16} />{t('ontology.loadMore')}</button>}
        </> : <>
          {item.error_code && <div className="import-warning" role="status"><AlertTriangle size={16} />{t('import.error.' + item.error_code)}</div>}
          <div className="import-review-layout">
            <section className="import-source-panel" aria-label={t('import.sources')}>
              <div className="import-panel-tabs" role="tablist" aria-label={t('import.sources')}>
                {(['source', 'evidence', 'history'] as const).map((tab) => <button key={tab} id={`${fieldID}-${tab}-tab`} type="button" role="tab" aria-controls={`${fieldID}-${tab}-panel`} aria-selected={sourceTab === tab} onClick={() => {
                  setSourceTab(tab)
                  if (tab === 'history') getDocumentImport(token, item.id).then((record) => setItem((current) => current?.id === record.id ? { ...current, events: record.events } : current)).catch((err) => setError(describeApiError(err, t)))
                }}>{tab === 'source' ? <FileText size={15} /> : tab === 'evidence' ? <ScanLine size={15} /> : <History size={15} />}{t('import.tab.' + tab)}</button>)}
              </div>
              <div role="tabpanel" id={`${fieldID}-${sourceTab}-panel`} aria-labelledby={`${fieldID}-${sourceTab}-tab`}>
                {sourceTab === 'source' && <>
                  <select className="ui-select import-source-select" aria-label={t('import.sources')} value={selectedFile?.id ?? ''} onChange={(event) => setActiveSource(event.target.value)}>{item.files.map((file) => <option key={file.id} value={file.id}>{file.name}</option>)}</select>
                  {selectedFile && <SourcePreview key={selectedFile.id} token={token} item={item} source={selectedFile} />}
                </>}
                {sourceTab === 'evidence' && <div className="import-evidence-list">
                  {(item.extraction?.evidence ?? []).map((evidence, index) => <EvidenceRow key={index} evidence={evidence} label={fieldLabel(evidence.path)} file={item.files.find((file) => file.id === evidence.source_id)} onSource={() => { setActiveSource(evidence.source_id); setSourceTab('source') }} />)}
                  {!item.extraction?.evidence?.length && <div className="ui-empty"><ScanLine size={22} /><strong>{t('import.noEvidence')}</strong></div>}
                </div>}
                {sourceTab === 'history' && <ol className="import-event-list">{(item.events ?? []).map((event) => <li key={event.id}><strong>{t('import.event.' + event.event)}</strong><time>{formatDate(event.created_at, locale)}</time><small>{t('import.version', { number: event.version })}</small></li>)}</ol>}
              </div>
            </section>
            <section className="import-draft-panel" aria-label={t('import.review')}>
              <div className="import-draft-heading"><h3>{t('import.review')}</h3><span>{t('import.version', { number: item.version })}</span></div>
              {(item.extraction?.warnings ?? []).length > 0 && <div className="import-warning"><AlertTriangle size={16} /><ul>{item.extraction.warnings!.map((warning, index) => <li key={index}>{warning}</li>)}</ul></div>}
              <div className="import-header-fields">{fieldEditor('key')}{headerFields.filter((field) => !isPayment || !['tax', 'due_date'].includes(field)).map((field) => <div key={field}>{fieldEditor(field)}</div>)}</div>
              {!isPayment && <>
                <div className="import-lines-heading"><h4>{t('import.lines', { count: draft.lines.length })}</h4><button type="button" className="ui-button ui-button-ghost" disabled={disabled || draft.lines.length >= 200}
                  onClick={() => { setDraft((current) => ({ ...current, lines: [...current.lines, { item: '', warehouse: '', quantity: '', unit_price: '', tax_rate: '0' }] })); setDirty(true) }}><Plus size={15} />{t('ui.document.addLine')}</button></div>
                <div className="import-lines-table"><table><thead><tr>{lineFields.map((field) => <th key={field}>{t('import.field.' + field)}</th>)}<th><span className="sr-only">{t('common.actions')}</span></th></tr></thead>
                  <tbody>{draft.lines.map((_, index) => <tr key={index}>{lineFields.map((field) => <td key={field}>{fieldEditor(field, index)}</td>)}<td><button type="button" className="ui-icon-button" disabled={disabled} title={t('ui.document.deleteLine')} aria-label={t('ui.document.deleteLine') + ': ' + (index + 1)}
                    onClick={() => { setDraft((current) => ({ ...current, lines: current.lines.filter((_, position) => position !== index) })); setDirty(true) }}><Trash2 size={15} /></button></td></tr>)}</tbody>
                </table></div>
              </>}
            </section>
          </div>
        </>}
      </div>
    </Dialog>
    <Dialog open={!!confirmation} title={t(confirmation === 'reject' ? 'import.rejectTitle' : 'import.confirmTitle')} busy={busy} onClose={() => { setConfirmation(null); setError('') }} size="lg" danger={confirmation === 'reject'}
      footer={<>
        <button type="button" className="ui-button ui-button-secondary" disabled={busy} data-dialog-cancel onClick={() => { setConfirmation(null); setError('') }}>{t('common.cancel')}</button>
        <button type="button" className={'ui-button ' + (confirmation === 'reject' ? 'ui-button-danger' : 'ui-button-primary')} disabled={busy || (confirmation === 'confirm' && !acknowledged)} onClick={() => {
          if (!item) return
          void run(async () => {
            const result = confirmation === 'reject' ? await rejectDocumentImport(token, item) : await confirmDocumentImport(token, item, draft)
            adopt(result)
            setConfirmation(null)
            if (result.status === 'confirmed') onConfirmed(result.confirmed_key!)
          })
        }}>{busy ? <Loader2 size={16} className="animate-spin" /> : <Check size={16} />}{t(confirmation === 'reject' ? 'import.reject' : 'import.confirmCreate')}</button>
      </>}
    >
      <div className="import-confirm-summary"><FileText size={22} /><div><strong>{title}</strong><span>{draft.key}</span></div></div>
      {confirmation === 'confirm' && <>
        <dl className="document-action-summary">{['partner', 'date', 'currency', 'total'].map((field) => <div key={field}><dt>{t('import.field.' + field)}</dt><dd>{draft.properties[field] || t('import.notSpecified')}</dd></div>)}</dl>
        {corrections.length > 0 && <div className="import-diff-table"><table className="document-diff"><thead><tr><th>{t('ui.document.field')}</th><th>{t('import.recognized')}</th><th>{t('import.reviewed')}</th></tr></thead>
          <tbody>{corrections.map((change) => <tr key={change.path}><td>{fieldLabel(change.path)}</td><td>{change.before || '-'}</td><td>{change.after || '-'}</td></tr>)}</tbody></table></div>}
        <label className="import-acknowledgement"><input type="checkbox" checked={acknowledged} onChange={(event) => setAcknowledged(event.target.checked)} disabled={busy} />{t('import.acknowledgement')}</label>
      </>}
      {error && <FeedbackMessage error>{error}</FeedbackMessage>}
      {issues.length > 0 && <ul className="import-issues">{issues.map((issue, index) => <li key={index}>{fieldLabel(issue.path)}: {t('import.issue.' + issue.code)}</li>)}</ul>}
    </Dialog>
  </>
}

function SourcePreview({ token, item, source }: { token: string; item: DocumentImport; source: ImportSource }) {
  const { t, locale } = useI18n()
  const [preview, setPreview] = useState<{ url: string; text?: string } | null>(null)
  const [error, setError] = useState('')
  useEffect(() => {
    let cancelled = false
    let url = ''
    getDocumentImportFile(token, item, source).then(async (blob) => {
      const text = source.media_type.startsWith('text/') ? await blob.text() : undefined
      if (cancelled) return
      url = URL.createObjectURL(blob)
      setPreview({ url, text })
    }).catch((err) => { if (!cancelled) setError(describeApiError(err, t)) })
    return () => { cancelled = true; if (url) URL.revokeObjectURL(url) }
  }, [token, item.id, source.id, source.media_type, t]) // eslint-disable-line react-hooks/exhaustive-deps

  return <div className="import-source-preview">
    <div className="import-source-meta"><span>{fileSize(source.byte_size, locale)}</span>{preview && <a className="ui-icon-button" href={preview.url} download={source.name} title={t('ontology.download')} aria-label={t('ontology.download')}><Download size={16} /></a>}</div>
    {error ? <FeedbackMessage error>{error}</FeedbackMessage> : !preview ? <div className="ui-empty"><Loader2 size={24} className="animate-spin" /><strong>{t('common.loading')}</strong></div>
      : source.media_type.startsWith('image/') ? <img src={preview.url} alt={source.name} className="import-source-image" /> // eslint-disable-line @next/next/no-img-element
        : source.media_type === 'application/pdf' ? <DocumentPdfPreview key={preview.url} url={preview.url} name={source.name} />
          : preview.text !== undefined ? <pre>{preview.text}</pre>
            : <div className="ui-empty"><FileText size={32} /><strong>{t('import.previewUnavailable')}</strong></div>}
  </div>
}

function EvidenceRow({ evidence, label, file, onSource }: { evidence: ImportEvidence; label: string; file?: ImportSource; onSource: () => void }) {
  const { t } = useI18n()
  return <div className="import-evidence-row" data-low-confidence={evidence.confidence < 0.8 ? 'true' : undefined}>
    <div><strong>{label}</strong><StatusBadge tone={evidence.confidence < 0.8 ? 'amber' : 'neutral'}>{t('import.confidence', { value: Math.round(evidence.confidence * 100) })}</StatusBadge></div>
    <blockquote>{evidence.quote}</blockquote>
    <button type="button" className="import-evidence-link" onClick={onSource}>{file?.media_type.startsWith('image/') ? <ImageIcon size={13} /> : <FileText size={13} />}{file?.name}{evidence.page ? ' / ' + t('import.page', { number: evidence.page }) : ''}</button>
  </div>
}

function draftChanges(original: ImportDraft | undefined, reviewed: ImportDraft) {
  const flatten = (draft?: ImportDraft) => Object.fromEntries([
    ...Object.entries(draft?.properties ?? {}).map(([key, value]) => ['properties.' + key, String(value ?? '')]),
    ...(draft?.lines ?? []).flatMap((line, index) => Object.entries(line).map(([key, value]) => [`lines.${index}.${key}`, String(value ?? '')])),
  ])
  const before = flatten(original), after = flatten(reviewed)
  return Array.from(new Set([...Object.keys(before), ...Object.keys(after)])).filter((path) => (before[path] ?? '') !== (after[path] ?? ''))
    .map((path) => ({ path, before: before[path] ?? '', after: after[path] ?? '' }))
}

function fileSize(bytes: number, locale: string) {
  return new Intl.NumberFormat(locale === 'zh' ? 'zh-CN' : 'en-US', { maximumFractionDigits: 1 }).format(bytes / 1024) + ' KB'
}

function formatDate(value: string, locale: string) {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '' : new Intl.DateTimeFormat(locale === 'zh' ? 'zh-CN' : 'en-US', { dateStyle: 'short', timeStyle: 'short' }).format(date)
}
