import { apiRequest } from '@/lib/api'

export type ImportDraft = { key: string; properties: Record<string, string>; lines: Array<Record<string, string>> }
export type ImportEvidence = { path: string; source_id: string; page?: number; quote: string; confidence: number }
export type ImportSource = { id: string; import_id: string; name: string; media_type: string; byte_size: number; sha256: string }
export type DocumentImport = {
  id: string
  object_type: string
  status: 'uploaded' | 'recognizing' | 'needs_review' | 'confirmed' | 'rejected' | 'failed'
  version: number
  draft: ImportDraft
  extraction: { draft?: ImportDraft; evidence?: ImportEvidence[]; warnings?: string[] }
  recognition_method: string
  recognition_started_at?: string
  invocation_id?: string
  error_code?: string
  confirmed_key?: string
  reviewed_by?: string
  reviewed_at?: string
  created_at: string
  files: ImportSource[]
  events?: Array<{ id: number; version: number; event: string; actor_id: string; created_at: string }>
}

const importPath = (id: string) => `/document-imports/${encodeURIComponent(id)}`

export function listDocumentImports(token: string, objectType: string, status = '', cursor = '') {
  const params = new URLSearchParams({ object_type: objectType, status, cursor })
  return apiRequest<{ imports: DocumentImport[]; next_cursor?: string }>(`/document-imports?${params}`, { token })
}

export function getDocumentImport(token: string, id: string) {
  return apiRequest<DocumentImport>(importPath(id), { token })
}

export function uploadDocumentImport(token: string, objectType: string, files: File[]) {
  const body = new FormData()
  body.append('object_type', objectType)
  files.forEach((file) => body.append('files', file))
  return apiRequest<DocumentImport>('/document-imports', { token, method: 'POST', body })
}

export function recognizeDocumentImport(token: string, item: DocumentImport) {
  return apiRequest<DocumentImport>(`${importPath(item.id)}/recognize`, { token, method: 'POST', body: { version: item.version } })
}

export function saveDocumentImport(token: string, item: DocumentImport, draft: ImportDraft) {
  return apiRequest<DocumentImport>(`${importPath(item.id)}/review`, { token, method: 'PATCH', body: { version: item.version, draft } })
}

export function confirmDocumentImport(token: string, item: DocumentImport, draft: ImportDraft) {
  return apiRequest<DocumentImport>(`${importPath(item.id)}/confirm`, { token, method: 'POST', body: { version: item.version, draft, confirmed: true } })
}

export function rejectDocumentImport(token: string, item: DocumentImport) {
  return apiRequest<DocumentImport>(`${importPath(item.id)}/reject`, { token, method: 'POST', body: { version: item.version } })
}

export function getDocumentImportFile(token: string, item: DocumentImport, file: ImportSource) {
  return apiRequest<Blob>(`${importPath(item.id)}/files/${encodeURIComponent(file.id)}`, { token, responseType: 'blob' })
}
