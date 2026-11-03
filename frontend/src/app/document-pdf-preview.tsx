'use client'

import { ChevronLeft, ChevronRight, Loader2, ZoomIn, ZoomOut } from 'lucide-react'
import type { PDFDocumentLoadingTask, PDFDocumentProxy, RenderTask } from 'pdfjs-dist'
import { useEffect, useRef, useState } from 'react'

import { useI18n } from '@/lib/i18n'
import { FeedbackMessage } from './workspace-ui'

export function DocumentPdfPreview({ url, name }: { url: string; name: string }) {
  const { t } = useI18n()
  const container = useRef<HTMLDivElement>(null)
  const canvas = useRef<HTMLCanvasElement>(null)
  const renderTask = useRef<RenderTask | null>(null)
  const renderQueue = useRef<Promise<void>>(Promise.resolve())
  const [document, setDocument] = useState<PDFDocumentProxy | null>(null)
  const [width, setWidth] = useState(0)
  const [page, setPage] = useState(1)
  const [zoom, setZoom] = useState(1)
  const [rendered, setRendered] = useState('')
  const [failed, setFailed] = useState(false)
  const renderKey = `${page}:${zoom}:${width}`
  const loading = !document || rendered !== renderKey

  useEffect(() => {
    let cancelled = false
    let task: PDFDocumentLoadingTask | undefined
    void import('pdfjs-dist').then(async (pdf) => {
      if (cancelled) return
      pdf.GlobalWorkerOptions.workerSrc = '/pdfjs/pdf.worker.min.mjs'
      task = pdf.getDocument({ url, isEvalSupported: false, enableXfa: false,
        cMapUrl: '/pdfjs/cmaps/', cMapPacked: true, standardFontDataUrl: '/pdfjs/standard_fonts/', wasmUrl: '/pdfjs/wasm/' })
      const loaded = await task.promise
      if (!cancelled) setDocument(loaded)
    }).catch(() => { if (!cancelled) setFailed(true) })
    return () => { cancelled = true; void task?.destroy() }
  }, [url])

  useEffect(() => {
    const element = container.current
    if (!element) return
    const observer = new ResizeObserver(([entry]) => setWidth(Math.floor(entry.contentRect.width)))
    observer.observe(element)
    return () => observer.disconnect()
  }, [])

  useEffect(() => {
    if (!document || !width || !canvas.current) return
    let cancelled = false
    const element = canvas.current
    // pdf.js locks a canvas for the full duration of render() and, when a task is
    // cancelled before it initializes, aborts the page's operator list so the next
    // render fails. Serializing through a promise queue keeps one render per canvas
    // (React StrictMode re-runs this effect immediately in development) and lets a
    // superseded render finish harmlessly instead of cancelling it.
    const run = async () => {
      if (cancelled) return
      const content = await document.getPage(page)
      if (cancelled) return
      const natural = content.getViewport({ scale: 1 })
      const displayWidth = Math.min(width, 1600) * zoom
      const viewport = content.getViewport({ scale: displayWidth / natural.width * Math.min(window.devicePixelRatio || 1, 2) })
      element.width = Math.ceil(viewport.width)
      element.height = Math.ceil(viewport.height)
      element.style.width = `${displayWidth}px`
      element.style.height = `${displayWidth * natural.height / natural.width}px`
      // Render page content only; embedded PDF scripts and interactive actions are never executed.
      const task = content.render({ canvas: element, viewport })
      renderTask.current = task
      try {
        await task.promise
      } finally {
        if (renderTask.current === task) renderTask.current = null
      }
      if (!cancelled) setRendered(renderKey)
    }
    const queued = renderQueue.current.then(run)
    renderQueue.current = queued.catch(() => undefined)
    queued.catch((reason) => {
      if (cancelled) return
      console.error('PDF preview render failed', reason)
      setFailed(true)
    })
    return () => { cancelled = true }
  }, [document, width, page, zoom, renderKey])

  useEffect(() => () => { renderTask.current?.cancel(); renderTask.current = null }, [])

  return <div className="import-pdf-viewer">
    <div className="import-pdf-toolbar" role="group" aria-label={t('import.pdfControls')}>
      <button type="button" className="ui-icon-button" title={t('import.previousPage')} aria-label={t('import.previousPage')} disabled={!document || page <= 1} onClick={() => setPage((value) => value - 1)}><ChevronLeft size={16} /></button>
      <span className="import-pdf-page-count">{t('import.pageCount', { page, count: document?.numPages ?? 0 })}</span>
      <button type="button" className="ui-icon-button" title={t('import.nextPage')} aria-label={t('import.nextPage')} disabled={!document || page >= document.numPages} onClick={() => setPage((value) => value + 1)}><ChevronRight size={16} /></button>
      <button type="button" className="ui-icon-button" title={t('import.zoomOut')} aria-label={t('import.zoomOut')} disabled={!document || zoom <= 1} onClick={() => setZoom((value) => Math.max(1, value - 0.25))}><ZoomOut size={16} /></button>
      <span className="import-pdf-zoom">{Math.round(zoom * 100)}%</span>
      <button type="button" className="ui-icon-button" title={t('import.zoomIn')} aria-label={t('import.zoomIn')} disabled={!document || zoom >= 2} onClick={() => setZoom((value) => Math.min(2, value + 0.25))}><ZoomIn size={16} /></button>
    </div>
    {failed && <FeedbackMessage error>{t('import.error.preview_failed')}</FeedbackMessage>}
    <div ref={container} className="import-pdf-pages" aria-busy={loading && !failed}>
      {loading && !failed && <span className="import-pdf-loading" role="status"><Loader2 size={18} className="animate-spin" />{t('common.loading')}</span>}
      <canvas ref={canvas} role="img" aria-label={`${name} / ${t('import.page', { number: page })}`} data-testid="import-pdf-canvas" data-rendered={rendered === renderKey ? page : undefined} />
    </div>
  </div>
}
