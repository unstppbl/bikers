let csrfToken: string | undefined
let sessionEpoch = 0

export function setCsrfToken(token?: string) {
  if (token !== csrfToken) sessionEpoch++
  csrfToken = token
}

export function getSessionEpoch() { return sessionEpoch }

export class ApiError extends Error {
  readonly code: string
  readonly status: number
  constructor(status: number, code: string) {
    super(code)
    this.name = 'ApiError'
    this.code = code
    this.status = status
  }
}

export async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers)
  headers.set('Accept', 'application/json')
  if (typeof options.body === 'string' && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json')
  if (options.method && !['GET', 'HEAD'].includes(options.method) && csrfToken) {
    headers.set('X-CSRF-Token', csrfToken)
  }
  let response: Response
  try {
    response = await fetch(`/api/v1${path}`, {
      ...options,
      headers,
      credentials: 'same-origin',
      cache: 'no-store',
    })
  } catch (error) {
    if (options.signal?.aborted) throw error
    throw new ApiError(0, 'network_error')
  }
  if (!response.ok) {
    const payload = await response.json().catch(() => null)
    if (response.status === 401 && csrfToken) {
      setCsrfToken()
      window.dispatchEvent(new Event('bikerskz:session-expired'))
    }
    throw new ApiError(response.status, payload?.error?.code ?? 'request_failed')
  }
  if (response.status === 204) return undefined as T
  return response.json() as Promise<T>
}

export function mutation<T>(path: string, method: string, body?: unknown) {
  return request<T>(path, { method, ...(body === undefined ? {} : { body: JSON.stringify(body) }) })
}

export function uploadFile<T>(path: string, file: File, signal: AbortSignal) {
  return request<T>(path, { method: 'POST', body: file, signal, headers: { 'Content-Type': file.type || 'application/octet-stream' } })
}

export function publicRide(ride: import('./types').Ride): import('./types').Ride {
  return {
    id: ride.id,
    city: ride.city,
    date: ride.date,
    description: ride.description,
    status: ride.status,
    participation_state: undefined,
    can_manage: false,
  }
}

export function safeExternalUrl(value?: string) {
  if (!value) return undefined
  try {
    const parsed = new URL(value)
    return ['https:', 'http:'].includes(parsed.protocol) ? parsed.href : undefined
  } catch {
    return undefined
  }
}
