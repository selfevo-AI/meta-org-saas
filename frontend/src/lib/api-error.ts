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
