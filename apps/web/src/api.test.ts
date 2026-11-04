import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError, mutation, publicRide, request, requestCalendar, safeExternalUrl, setCsrfToken, uploadFile } from './api'

afterEach(() => { setCsrfToken(); vi.unstubAllGlobals() })

describe('same-origin API boundary', () => {
  it('uploads raw file bytes with their content type, CSRF and cancellation signal', async () => {
    const fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify({ gallery: {} }), { status: 201 }))
    vi.stubGlobal('fetch', fetch)
    setCsrfToken('photo-csrf')
    const file = new File(['photo bytes'], 'bike.webp', { type: 'image/webp' })
    const controller = new AbortController()
    await uploadFile('/me/bikes/example/photos', file, controller.signal)
    const [url, options] = fetch.mock.calls[0]
    expect(url).toBe('/api/v1/me/bikes/example/photos')
    expect(options.body).toBe(file)
    expect(options.headers.get('Content-Type')).toBe('image/webp')
    expect(options.headers.get('X-CSRF-Token')).toBe('photo-csrf')
    expect(options.credentials).toBe('same-origin')
    expect(options.cache).toBe('no-store')
    expect(options.signal).toBe(controller.signal)
  })

  it('uses octet-stream for an unknown browser MIME and preserves a deliberate abort', async () => {
    const controller = new AbortController()
    const fetch = vi.fn(async (_url, options) => {
      expect(options.headers.get('Content-Type')).toBe('application/octet-stream')
      controller.abort()
      throw new DOMException('Cancelled', 'AbortError')
    })
    vi.stubGlobal('fetch', fetch)
    await expect(uploadFile('/me/bikes/example/photos', new File(['bytes'], 'bike.png'), controller.signal)).rejects.toMatchObject({ name: 'AbortError' })
  })

  it('keeps credentials and cache private and sends the in-memory CSRF token on authenticated writes', async () => {
    const fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify({ ok: true }), { status: 200 }))
    vi.stubGlobal('fetch', fetch)
    setCsrfToken('test-csrf')
    await mutation('/rides', 'POST', { title: 'Test ride' })
    const [url, options] = fetch.mock.calls[0]
    expect(url).toBe('/api/v1/rides')
    expect(options.credentials).toBe('same-origin')
    expect(options.cache).toBe('no-store')
    expect(options.headers.get('X-CSRF-Token')).toBe('test-csrf')
    expect(localStorage.length).toBe(0)
  })

  it('never sends CSRF material on reads and raises typed server errors', async () => {
    const fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: { code: 'ride_full', message: 'diagnostic' } }), { status: 409 }))
    vi.stubGlobal('fetch', fetch)
    setCsrfToken('test-csrf')
    await expect(request('/rides/example')).rejects.toMatchObject({ status: 409, code: 'ride_full' })
    expect(fetch.mock.calls[0][1].headers.has('X-CSRF-Token')).toBe(false)
  })


  it.each(['json', 'calendar'])('does not expire a new login when an old %s request returns 401', async (kind) => {
    let finish: ((value: Response) => void) | undefined
    const fetch = vi.fn<(url: string, options?: RequestInit) => Promise<Response>>(() => new Promise<Response>((resolve) => { finish = resolve }))
    vi.stubGlobal('fetch', fetch)
    const expired = vi.fn()
    window.addEventListener('bikerskz:session-expired', expired)
    try {
      setCsrfToken('account-a')
      const controller = new AbortController()
      const pending = kind === 'calendar' ? requestCalendar('example', controller.signal) : request('/me/notifications/unread-count', { signal: controller.signal })
      setCsrfToken('account-b')
      finish!(new Response(JSON.stringify({ error: { code: 'authentication_required' } }), { status: 401 }))
      await expect(pending).rejects.toMatchObject({ status: 401 })
      expect(expired).not.toHaveBeenCalled()
      fetch.mockResolvedValueOnce(new Response(JSON.stringify({ ok: true })))
      await mutation('/me/preferences', 'PATCH', {})
      expect(new Headers(fetch.mock.calls[1][1]?.headers).get('X-CSRF-Token')).toBe('account-b')
    } finally { window.removeEventListener('bikerskz:session-expired', expired) }
  })

  it.each(['json', 'calendar'])('does not expire the current login for an aborted %s request that returns 401', async (kind) => {
    let finish: ((value: Response) => void) | undefined
    vi.stubGlobal('fetch', vi.fn(() => new Promise<Response>((resolve) => { finish = resolve })))
    const expired = vi.fn()
    window.addEventListener('bikerskz:session-expired', expired)
    try {
      setCsrfToken('current-account')
      const controller = new AbortController()
      const pending = kind === 'calendar' ? requestCalendar('example', controller.signal) : request('/me/preferences', { signal: controller.signal })
      controller.abort()
      finish!(new Response(JSON.stringify({ error: { code: 'authentication_required' } }), { status: 401 }))
      await expect(pending).rejects.toMatchObject({ status: 401 })
      expect(expired).not.toHaveBeenCalled()
    } finally { window.removeEventListener('bikerskz:session-expired', expired) }
  })

  it('signals expired authenticated sessions without exposing diagnostic content', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: { code: 'authentication_required', message: 'private diagnostic' } }), { status: 401 })))
    const expired = vi.fn()
    window.addEventListener('bikerskz:session-expired', expired)
    setCsrfToken('test-csrf')
    await expect(request('/rides/example/messages')).rejects.toBeInstanceOf(ApiError)
    expect(expired).toHaveBeenCalledOnce()
    window.removeEventListener('bikerskz:session-expired', expired)
  })
})

describe('untrusted and revoked ride data', () => {
  it('removes every participant-only property when reducing a ride to a public preview', () => {
    const ride = publicRide({ id: 'r1', city: 'Test city', date: '2026-12-01', description: 'Public summary', status: 'published', title: 'Private title', meeting_point: 'Private place', starts_at: '2026-12-01T10:00:00Z', capacity: 8, organizer_id: 'owner', can_manage: true, participation_state: 'approved' })
    expect(ride).not.toHaveProperty('title')
    expect(ride).not.toHaveProperty('meeting_point')
    expect(ride).not.toHaveProperty('organizer_id')
    expect(ride.can_manage).toBe(false)
  })

  it('allows navigation links only for HTTP or HTTPS', () => {
    expect(safeExternalUrl('javascript:alert(1)')).toBeUndefined()
    expect(safeExternalUrl('data:text/html,example')).toBeUndefined()
    expect(safeExternalUrl('//example.com')).toBeUndefined()
    expect(safeExternalUrl('https://example.com/route')).toBe('https://example.com/route')
  })
})
