'use client'

import { AlertTriangle, ArrowRight, Check, ChevronDown, Search, X } from 'lucide-react'
import { createContext, useCallback, useContext, useEffect, useId, useMemo, useRef, useState } from 'react'
import type { KeyboardEvent, ReactNode } from 'react'

import { useI18n } from '@/lib/i18n'

let openDialogCount = 0
let originalBodyOverflow = ''

export function lockPageScroll() {
  if (openDialogCount === 0) {
    originalBodyOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
  }
  openDialogCount += 1
  let released = false
  return () => {
    if (released) return
    released = true
    openDialogCount -= 1
    if (openDialogCount === 0) document.body.style.overflow = originalBodyOverflow
  }
}

export function Dialog({ open, ...props }: DialogProps & { open: boolean }) {
  return open ? <OpenDialog {...props} /> : null
}

interface DialogProps {
  title: string
  description?: string
  children: ReactNode
  footer?: ReactNode
  onClose: () => void
  busy?: boolean
  size?: 'sm' | 'md' | 'lg'
  danger?: boolean
  className?: string
}

function OpenDialog({ title, description, children, footer, onClose, busy = false, size = 'md', danger = false, className = '' }: DialogProps) {
  const { t } = useI18n()
  const titleID = useId()
  const descriptionID = useId()
  const dialogRef = useRef<HTMLDialogElement>(null)

  useEffect(() => {
    const dialog = dialogRef.current
    if (!dialog) return
    const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null
    const unlock = lockPageScroll()
    dialog.showModal()
    const focusTarget = dialog.querySelector<HTMLElement>('[data-autofocus], [data-dialog-cancel]')
    focusTarget?.focus()
    return () => {
      dialog.close()
      unlock()
      if (previousFocus?.isConnected) previousFocus.focus({ preventScroll: true })
    }
  }, [])

  return (
    <dialog
      ref={dialogRef}
      aria-labelledby={titleID}
      aria-describedby={description ? descriptionID : undefined}
      aria-busy={busy}
      className={`ui-dialog ui-dialog-${size} ${className}`}
      onCancel={(event) => { event.preventDefault(); if (!busy) onClose() }}
      onClick={(event) => {
        if (event.target !== event.currentTarget || busy) return
        const rect = event.currentTarget.getBoundingClientRect()
        if (event.clientX < rect.left || event.clientX > rect.right || event.clientY < rect.top || event.clientY > rect.bottom) onClose()
      }}
    >
      <div className="ui-dialog-header">
        {danger && <span className="ui-dialog-warning"><AlertTriangle size={21} /></span>}
        <div className="min-w-0 flex-1">
          <h2 id={titleID}>{title}</h2>
          {description && <p id={descriptionID}>{description}</p>}
        </div>
        <button type="button" className="ui-icon-button" onClick={onClose} disabled={busy} aria-label={t('common.close')}><X size={18} /></button>
      </div>
      <div className="ui-dialog-body">{children}</div>
      {footer && <div className="ui-dialog-footer">{footer}</div>}
    </dialog>
  )
}

export interface ActionMenuItem {
  id: string
  label: string
  icon?: ReactNode
  onSelect: () => void
  danger?: boolean
  disabled?: boolean
}

export function ActionMenu({ label, items, icon, className = '', testId }: {
  label: string
  items: ActionMenuItem[]
  icon?: ReactNode
  className?: string
  testId?: string
}) {
  const [open, setOpen] = useState(false)
  const menuID = useId()
  const root = useRef<HTMLDivElement>(null)
  const trigger = useRef<HTMLButtonElement>(null)
  const menu = useRef<HTMLDivElement>(null)

  const close = useCallback((restoreFocus = false) => {
    setOpen(false)
    if (restoreFocus) trigger.current?.focus()
  }, [])

  useEffect(() => {
    if (!open) return
    const panel = menu.current
    if (!panel) return
    panel.showPopover()
    const position = () => {
      if (!trigger.current) return
      const anchor = trigger.current.getBoundingClientRect()
      const rect = panel.getBoundingClientRect()
      panel.style.left = `${Math.max(8, Math.min(anchor.right - rect.width, window.innerWidth - rect.width - 8))}px`
      panel.style.top = `${Math.max(8, anchor.bottom + rect.height + 7 > window.innerHeight ? anchor.top - rect.height - 7 : anchor.bottom + 7)}px`
    }
    position()
    panel.querySelector<HTMLButtonElement>('button:not(:disabled)')?.focus()
    const outside = (event: PointerEvent) => {
      if (!root.current?.contains(event.target as Node)) close()
    }
    const escape = (event: globalThis.KeyboardEvent) => {
      if (event.key === 'Escape') { event.stopPropagation(); close(true) }
    }
    document.addEventListener('pointerdown', outside)
    document.addEventListener('keydown', escape)
    document.addEventListener('scroll', position, true)
    window.addEventListener('resize', position)
    return () => {
      document.removeEventListener('pointerdown', outside)
      document.removeEventListener('keydown', escape)
      document.removeEventListener('scroll', position, true)
      window.removeEventListener('resize', position)
      if (panel.isConnected && panel.matches(':popover-open')) panel.hidePopover()
    }
  }, [open, close])

  function handleKeys(event: KeyboardEvent<HTMLDivElement>) {
    const buttons = Array.from(menu.current?.querySelectorAll<HTMLButtonElement>('button:not(:disabled)') ?? [])
    const index = buttons.indexOf(document.activeElement as HTMLButtonElement)
    let target = index
    if (event.key === 'ArrowDown') target = (index + 1) % buttons.length
    else if (event.key === 'ArrowUp') target = (index - 1 + buttons.length) % buttons.length
    else if (event.key === 'Home') target = 0
    else if (event.key === 'End') target = buttons.length - 1
    else if (event.key === 'Tab') { close(); return }
    else return
    event.preventDefault()
    buttons[target]?.focus()
  }

  return (
    <div ref={root} className={`ui-action-menu ${className}`}>
      <button
        ref={trigger}
        type="button"
        data-testid={testId}
        className="ui-button ui-button-secondary"
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={open ? menuID : undefined}
        onClick={() => setOpen((current) => !current)}
        onKeyDown={(event) => { if (event.key === 'ArrowDown') { event.preventDefault(); setOpen(true) } }}
      >
        {icon}<span>{label}</span><ChevronDown size={14} />
      </button>
      {open && (
        <div ref={menu} id={menuID} popover="manual" role="menu" aria-label={label} className="ui-menu-panel" onKeyDown={handleKeys}>
          {items.map((item) => (
            <button
              key={item.id}
              type="button"
              role="menuitem"
              disabled={item.disabled}
              className={item.danger ? 'ui-menu-danger' : ''}
              onClick={() => { close(true); item.onSelect() }}
            >{item.icon}<span>{item.label}</span></button>
          ))}
        </div>
      )}
    </div>
  )
}

interface WorkspaceInteraction {
  updateDirty: (id: string, dirty: boolean) => void
  requestNavigation: (action: () => void) => void
  pendingNavigation: (() => void) | null
  cancelNavigation: () => void
  discardChanges: () => void
}

const InteractionContext = createContext<WorkspaceInteraction | null>(null)

export function WorkspaceInteractionProvider({ children }: { children: ReactNode }) {
  const dirtyEntries = useRef(new Set<string>())
  const [pendingNavigation, setPendingNavigation] = useState<(() => void) | null>(null)
  const updateDirty = useCallback((id: string, dirty: boolean) => {
    if (dirty) dirtyEntries.current.add(id)
    else dirtyEntries.current.delete(id)
  }, [])
  const requestNavigation = useCallback((action: () => void) => {
    if (dirtyEntries.current.size > 0) setPendingNavigation(() => action)
    else action()
  }, [])
  const cancelNavigation = useCallback(() => setPendingNavigation(null), [])
  const discardChanges = useCallback(() => {
    dirtyEntries.current.clear()
    setPendingNavigation(null)
    pendingNavigation?.()
  }, [pendingNavigation])

  useEffect(() => {
    const beforeUnload = (event: BeforeUnloadEvent) => {
      if (dirtyEntries.current.size === 0) return
      event.preventDefault()
      event.returnValue = ''
    }
    window.addEventListener('beforeunload', beforeUnload)
    return () => window.removeEventListener('beforeunload', beforeUnload)
  }, [])

  const value = useMemo(() => ({ updateDirty, requestNavigation, pendingNavigation, cancelNavigation, discardChanges }), [updateDirty, requestNavigation, pendingNavigation, cancelNavigation, discardChanges])
  return <InteractionContext.Provider value={value}>{children}</InteractionContext.Provider>
}

export function useWorkspaceInteraction() {
  const context = useContext(InteractionContext)
  if (!context) throw new Error('WorkspaceInteractionProvider is required')
  return context
}

export function useUnsavedChanges(dirty: boolean) {
  const { updateDirty } = useWorkspaceInteraction()
  const id = useId()
  useEffect(() => {
    updateDirty(id, dirty)
    return () => updateDirty(id, false)
  }, [dirty, id, updateDirty])
}

export function UnsavedChangesDialog() {
  const { t } = useI18n()
  const { pendingNavigation, cancelNavigation, discardChanges } = useWorkspaceInteraction()
  return (
    <Dialog
      open={!!pendingNavigation}
      title={t('ui.unsaved.title')}
      description={t('ui.unsaved.description')}
      onClose={cancelNavigation}
      size="sm"
      danger
      footer={<>
        <button type="button" className="ui-button ui-button-secondary" data-dialog-cancel onClick={cancelNavigation}>{t('ui.unsaved.keepEditing')}</button>
        <button type="button" className="ui-button ui-button-danger" onClick={discardChanges}>{t('ui.unsaved.discard')}</button>
      </>}
    >
      <p className="ui-muted text-sm">{t('ui.unsaved.hint')}</p>
    </Dialog>
  )
}

export interface NavigationSearchItem {
  id: string
  label: string
  group: string
  keywords?: string
  icon: ReactNode
  onSelect: () => void
}

export function NavigationSearch({ open, onClose, items }: { open: boolean; onClose: () => void; items: NavigationSearchItem[] }) {
  const { t } = useI18n()
  return <Dialog open={open} onClose={onClose} title={t('ui.search.title')} description={t('ui.search.description')} className="ui-search-dialog">
    <SearchContent items={items} onClose={onClose} />
  </Dialog>
}

function SearchContent({ items, onClose }: { items: NavigationSearchItem[]; onClose: () => void }) {
  const { t } = useI18n()
  const [query, setQuery] = useState('')
  const [active, setActive] = useState(0)
  const listID = useId()
  const results = useMemo(() => {
    const words = query.toLocaleLowerCase().trim().split(/\s+/).filter(Boolean)
    return items.filter((item) => words.every((word) => `${item.label} ${item.group} ${item.keywords ?? ''}`.toLocaleLowerCase().includes(word))).slice(0, 30)
  }, [items, query])
  const choose = (item: NavigationSearchItem) => { onClose(); item.onSelect() }

  function handleKeys(event: KeyboardEvent<HTMLInputElement>) {
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault()
      const next = results.length ? (active + (event.key === 'ArrowDown' ? 1 : -1) + results.length) % results.length : 0
      setActive(next)
      document.getElementById(`${listID}-${next}`)?.scrollIntoView({ block: 'nearest' })
    } else if (event.key === 'Enter' && results[active]) {
      event.preventDefault()
      choose(results[active])
    }
  }

  return <>
    <div className="ui-search-input"><Search size={19} /><input
      data-autofocus
      role="combobox"
      aria-label={t('ui.search.placeholder')}
      aria-controls={listID}
      aria-expanded="true"
      aria-autocomplete="list"
      aria-activedescendant={results[active] ? `${listID}-${active}` : undefined}
      value={query}
      onChange={(event) => { setQuery(event.target.value); setActive(0) }}
      onKeyDown={handleKeys}
      placeholder={t('ui.search.placeholder')}
    /></div>
    <div id={listID} role="listbox" aria-label={t('ui.search.results')} className="ui-search-results">
      {results.map((item, index) => <button key={item.id} type="button" id={`${listID}-${index}`} role="option" aria-selected={index === active} tabIndex={-1} onClick={() => choose(item)}>
        <span className="ui-search-result-icon">{item.icon}</span><span className="min-w-0 flex-1"><strong>{item.label}</strong><small>{item.group}</small></span><ArrowRight size={16} />
      </button>)}
      {results.length === 0 && <div className="ui-empty"><Search size={24} /><strong>{t('ui.search.empty')}</strong><p>{t('ui.search.emptyHint')}</p></div>}
    </div>
    <div className="ui-search-help"><span><kbd>↑</kbd><kbd>↓</kbd> {t('ui.search.move')}</span><span><kbd>Enter</kbd> {t('ui.search.open')}</span><span><kbd>Esc</kbd> {t('common.close')}</span></div>
  </>
}

export function StatusBadge({ children, tone = 'neutral' }: { children: ReactNode; tone?: 'neutral' | 'blue' | 'green' | 'amber' | 'red' }) {
  return <span className={`ui-status ui-status-${tone}`}><span aria-hidden="true" />{children}</span>
}

export function FeedbackMessage({ children, error = false }: { children: ReactNode; error?: boolean }) {
  return <div role={error ? 'alert' : 'status'} className={`ui-feedback ${error ? 'ui-feedback-error' : 'ui-feedback-success'}`}>
    {error ? <AlertTriangle size={17} /> : <Check size={17} />}<span>{children}</span>
  </div>
}
