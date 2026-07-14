export type APIErrorPayload = {
  error?: string
  code?: string
  request_id?: string
}

export class APIError extends Error {
  readonly status: number
  readonly code: string
  readonly requestId: string

  constructor(message: string, status: number, code = 'request_failed', requestId = '') {
    super(message)
    this.name = 'APIError'
    this.status = status
    this.code = code
    this.requestId = requestId
  }
}

export function createRequestId(): string {
  return typeof globalThis.crypto?.randomUUID === 'function' ? globalThis.crypto.randomUUID() : ''
}

export async function apiErrorFromResponse(response: Response): Promise<APIError> {
  const payload = await response.json().catch(() => ({} as APIErrorPayload)) as APIErrorPayload
  return new APIError(
    payload.error || `HTTP ${response.status}`,
    response.status,
    payload.code || response.headers.get('X-Error-Code') || 'request_failed',
    payload.request_id || response.headers.get('X-Request-ID') || '',
  )
}

const errorCodeMessageKeys: Record<string, string> = {
  invalid_request: 'apiError.validation',
  validation_error: 'apiError.validation',
  authentication_required: 'apiError.unauthorized',
  forbidden: 'apiError.forbidden',
  not_found: 'apiError.notFound',
  conflict: 'apiError.conflict',
  rate_limited: 'apiError.rateLimited',
  upstream_unavailable: 'apiError.unavailable',
  service_unavailable: 'apiError.unavailable',
  internal_error: 'apiError.internal',
}

// describeApiError renders an error for display: known stable `code`s map to
// localized copy (never the raw backend message), unknown codes fall back to
// the free-text message, and the request_id is appended so users can quote it
// to support. Branch on `code`, never on the message text. `fallback` is used
// when the value is not an Error (or carries no message).
export function describeApiError(err: unknown, t: (key: string) => string, fallback?: string): string {
  if (err instanceof APIError) {
    const key = errorCodeMessageKeys[err.code]
    const message = key ? t(key) : err.message || fallback || t('apiError.internal')
    return err.requestId ? `${message} (Request-ID: ${err.requestId})` : message
  }
  if (err instanceof TypeError) {
    return t('apiError.network')
  }
  if (err instanceof Error && err.message) {
    return err.message
  }
  return fallback || t('apiError.internal')
}
