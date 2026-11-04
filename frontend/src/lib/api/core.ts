import { RequestOptions } from './types'
import { getCurrentOrganizationId, normalizeOrganizationId } from '@/lib/auth'
import { apiErrorFromResponse, createRequestId } from '@/lib/api-error'
export const API_BASE = '/api/v1'

// Fired when the backend rejects a request because the token belongs to a
// revoked session (password change, reset, or disable); the shell signs out.
export const SESSION_REVOKED_EVENT = 'meta-org:session-revoked'

export async function apiRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const isFormData = typeof FormData !== 'undefined' && options.body instanceof FormData
  const headers: Record<string, string> = {}
  const requestId = createRequestId()
  if (requestId) headers['X-Request-ID'] = requestId
  const requestScope = options.scope ?? (isPlatformPath(path) ? 'platform' : 'tenant')

  if (options.token) {
    headers['Authorization'] = `Bearer ${options.token}`
    const organizationId = normalizeOrganizationId(
      options.organizationId !== undefined
        ? options.organizationId
        : requestScope === 'tenant'
          ? getCurrentOrganizationId('tenant')
          : null,
    )
    if (organizationId) {
      headers['X-Organization-ID'] = organizationId
    }
  }
  if (!isFormData) {
    headers['Content-Type'] = 'application/json'
  }
  const requestBody = options.body ? (isFormData ? (options.body as BodyInit) : JSON.stringify(options.body)) : undefined

  const response = await fetch(`${API_BASE}${path}`, {
    method: options.method || 'GET',
    headers,
    body: requestBody,
  })

  if (!response.ok) {
    const error = await apiErrorFromResponse(response)
    if (error.code === 'session_revoked' && typeof window !== 'undefined') {
      window.dispatchEvent(new Event(SESSION_REVOKED_EVENT))
    }
    throw error
  }

  if (response.status === 204) {
    return undefined as T
  }

  if (options.responseType === 'blob') return response.blob() as Promise<T>

  return response.json()
}

export function isPlatformPath(path: string): boolean {
  return path === '/platform' || path.startsWith('/platform/')
}

export function platformAdminPath(path: string, scope: 'tenant' | 'platform' = 'tenant'): string {
  return scope === 'platform' ? `/platform/admin${path}` : path
}

export function aiGatewayPath(path: string, scope: 'tenant' | 'platform' = 'tenant'): string {
  return scope === 'platform' ? `/platform/admin${path}` : path
}
