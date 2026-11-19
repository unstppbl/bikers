import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { App } from '../App'
import { setCsrfToken } from '../api'
import { I18nProvider } from '../i18n'
import { SessionProvider } from '../session'
import type { Ride, RideCoverMetadata, RideCoverPhoto, User } from '../types'
import { safeRideCover, safeRideCoverPhoto } from './RideCover'

vi.mock('virtual:pwa-register/react', () => ({ useRegisterSW: () => ({ needRefresh: [false, vi.fn()], updateServiceWorker: vi.fn() }) }))

const organizer: User = { id: '10000000-0000-4000-8000-000000000001', profile: { display_name: 'Organizer', city: 'Almaty', languages: ['en'], interests: '', locale: 'en' }, onboarding_complete: true }
const rider: User = { ...organizer, id: '10000000-0000-4000-8000-000000000002', profile: { ...organizer.profile, display_name: 'Rider' } }
const rideId = '20000000-0000-4000-8000-000000000001'
const cover = (n: number): RideCoverPhoto => { const id = `75000000-0000-4000-8000-00000000000${n}`; return { id, url: `/api/v1/rides/${rideId}/cover/${id}/content`, width: 1600, height: 900, created_at: '2026-11-19T09:00:00Z' } }
const oldCover = cover(1), newCover = cover(2)
const summary: Ride = { id: rideId, city: 'Almaty', date: '2026-12-01', description: 'Sunday loop to Kok-Zhailau', status: 'published' }
const fullRide: Ride = { ...summary, participation_state: 'approved', can_manage: true, title: 'Kok-Zhailau loop', starts_at: '2026-12-01T04:00:00Z', timezone: 'Asia/Almaty', meeting_point: 'Medeu parking', destination: '', route_url: '', expectations: '', capacity: 8, plan_version: 1, organizer_id: organizer.id }
const response = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json' } })
const path = (url: string) => new URL(url, 'https://bikers.kz').pathname

interface State { user: User | null; ride: Ride; metadata: RideCoverMetadata }
function base(url: string, state: State) {
  const pathname = path(url)
  if (pathname === '/api/v1/session') return response({ user: state.user, ...(state.user ? { csrf_token: `csrf-${state.user.id}` } : {}) })
  if (pathname === `/api/v1/rides/${rideId}`) return response({ ride: state.ride })
  if (pathname === `/api/v1/rides/${rideId}/cover`) return response(state.metadata)
  if (pathname === '/api/v1/rides') return response({ rides: [], next_cursor: '' })
  if (pathname.endsWith('/participants')) return response({ participants: [] })
  if (pathname.endsWith('/messages')) return response({ messages: [], next_sequence: 0 })
  if (pathname.endsWith('/announcements')) return response({ announcements: [], next_cursor: '' })
  if (pathname.endsWith('/reminders')) return response({ day_before: false, hour_before: false })
  if (pathname.endsWith('/album')) return response({ photos: [], videos: [], version: 1 })
  if (pathname === '/api/v1/geo/config') return response({ enabled: false, archive_url: '' })
  if (pathname === '/api/v1/cities') return response({ cities: [] })
  if (pathname === '/api/v1/me/trust') return response({ is_admin: false })
  if (pathname === '/api/v1/me/blocks') return response({ blocks: [], next_cursor: '' })
  if (pathname.endsWith('/unread-count')) return response({ unread_count: 0 })
  if (pathname.includes('/avatar')) return response({ image: null })
  if (pathname.endsWith('/badge')) return response({ badge: null })
  throw new Error(`Unexpected API ${url}`)
}
function organizerState(): State { return { user: organizer, ride: { ...fullRide, cover: oldCover, cover_version: 1 }, metadata: { image: oldCover, version: 1 } } }
const reads = (fetch: ReturnType<typeof vi.fn>, pathname: string) => fetch.mock.calls.filter(([url, options]) => path(url as string) === pathname && !(options as RequestInit | undefined)?.method).length

class TestSocket { onopen: (() => void) | null = null; onclose: (() => void) | null = null; onmessage: ((event: { data: string }) => void) | null = null; close() {} }
function mount(route: string) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: Infinity }, mutations: { retry: false } } })
  render(<QueryClientProvider client={client}><MemoryRouter initialEntries={[route]}><I18nProvider><SessionProvider><App /></SessionProvider></I18nProvider></MemoryRouter></QueryClientProvider>)
  return client
}
beforeEach(() => {
  setCsrfToken()
  vi.stubGlobal('WebSocket', TestSocket)
  Object.defineProperty(URL, 'createObjectURL', { configurable: true, writable: true, value: vi.fn(() => 'blob:cover-preview') })
  Object.defineProperty(URL, 'revokeObjectURL', { configurable: true, writable: true, value: vi.fn() })
})
afterEach(() => { setCsrfToken(); vi.unstubAllGlobals() })

describe('ride cover projection', () => {
  it('keeps five fields and the owner-only version, and rejects covers that point anywhere else', () => {
    const raw = { image: { ...oldCover, uploader_id: organizer.id, filename: 'gps.jpg' }, version: 4, extra: 'secret' }
    expect(safeRideCover(raw, rideId, false)).toEqual({ image: oldCover })
    expect(safeRideCover(raw, rideId, true)).toEqual({ image: oldCover, version: 4 })
    for (const url of ['https://example.org/cover.jpg', `${oldCover.url}?x=1`, oldCover.url.replace(rideId, '20000000-0000-4000-8000-000000000009'), oldCover.url.replace('/cover/', '/photos/')]) {
      expect(safeRideCoverPhoto({ ...oldCover, url }, rideId)).toBeNull()
    }
    expect(safeRideCoverPhoto({ ...oldCover, width: 4000 }, rideId)).toBeNull()
    expect(safeRideCover({ image: null, version: 0 }, rideId, true)).toEqual({ image: null })
    expect(() => safeRideCover({ cover: oldCover }, rideId, true)).toThrow()
  })
})

describe('ride cover on the ride page', () => {
  it('shows the cover in the hero with alt text, and returns to the city art when the image fails', async () => {
    const state: State = { user: null, ride: { ...summary, cover: oldCover }, metadata: { image: oldCover } }
    vi.stubGlobal('fetch', vi.fn(async (url: string) => base(url, state)))
    mount(`/rides/${rideId}`)
    const image = await screen.findByRole('img', { name: 'Ride in Almaty' })
    expect(image).toHaveAttribute('src', oldCover.url)
    expect(image).toHaveAttribute('width', '1600')
    expect(image).toHaveAttribute('height', '900')
    expect(image).toHaveAttribute('referrerpolicy', 'no-referrer')
    expect(image).not.toHaveAttribute('loading')
    const hero = image.closest('section')!
    fireEvent.error(image)
    expect(screen.queryByRole('img', { name: 'Ride in Almaty' })).not.toBeInTheDocument()
    expect(hero.querySelector('.city-art')).toHaveTextContent('Almaty')
  })

  it('lets riders who are not the organizer report the cover', async () => {
    const state: State = { user: rider, ride: { ...summary, cover: oldCover }, metadata: { image: oldCover } }
    const fetch = vi.fn(async (url: string, options?: RequestInit) => path(url) === '/api/v1/reports' && options?.method === 'POST' ? response({ id: 'report' }, 201) : base(url, state))
    vi.stubGlobal('fetch', fetch)
    mount(`/rides/${rideId}`)
    await userEvent.click(await screen.findByRole('button', { name: 'Report cover photo' }))
    await userEvent.click(screen.getByRole('button', { name: 'Send report' }))
    await screen.findByText('Your report was sent for review.')
    const [, options] = fetch.mock.calls.find(([url]) => path(url) === '/api/v1/reports')!
    expect(JSON.parse(options!.body as string)).toEqual({ target_type: 'ride_cover', target_id: oldCover.id, reason: 'spam', details: '' })
  })

  it('shows no cover report to the organizer', async () => {
    const state = organizerState()
    vi.stubGlobal('fetch', vi.fn(async (url: string) => base(url, state)))
    mount(`/rides/${rideId}`)
    await screen.findByRole('heading', { name: 'Kok-Zhailau loop', level: 1 })
    expect(screen.getByRole('img', { name: 'Ride in Almaty' })).toHaveAttribute('src', oldCover.url)
    expect(screen.getByRole('button', { name: 'Report' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Report cover photo' })).not.toBeInTheDocument()
  })
})

describe('ride cover editor', () => {
  it('replaces the cover with the current version, keeps the old one meanwhile, then removes it after confirmation', async () => {
    const state = organizerState()
    let finish: (value: Response) => void = () => { throw new Error('not started') }
    const pending = new Promise<Response>((resolve) => { finish = resolve })
    const commands: { method: string; version: number; csrf: string | null; contentType: string | null }[] = []
    const fetch = vi.fn(async (url: string, options?: RequestInit) => {
      if (path(url) === `/api/v1/rides/${rideId}/cover` && options?.method === 'POST') {
        expect(options.body).toBeInstanceOf(File)
        commands.push({ method: 'POST', version: Number(new URL(url, 'https://bikers.kz').searchParams.get('version')), csrf: new Headers(options.headers).get('X-CSRF-Token'), contentType: new Headers(options.headers).get('Content-Type') })
        return pending
      }
      if (path(url) === `/api/v1/rides/${rideId}/cover` && options?.method === 'DELETE') {
        commands.push({ method: 'DELETE', version: JSON.parse(options.body as string).version, csrf: new Headers(options.headers).get('X-CSRF-Token'), contentType: new Headers(options.headers).get('Content-Type') })
        state.metadata = { image: null, version: 3 }; state.ride = { ...state.ride, cover: undefined, cover_version: 3 }
        return response(state.metadata)
      }
      return base(url, state)
    })
    vi.stubGlobal('fetch', fetch)
    mount(`/rides/${rideId}/edit`)
    const actor = userEvent.setup()
    expect(await screen.findByText('The cover is public, like the city and description, and stays visible after the ride is cancelled. Avoid photos that reveal the meeting point.')).toBeInTheDocument()
    expect(screen.getByRole('img', { name: 'Current cover photo' })).toHaveAttribute('src', oldCover.url)
    await actor.upload(screen.getByLabelText('Choose a photo'), new File(['bytes'], 'cover.webp', { type: 'image/webp' }))
    await screen.findByRole('img', { name: 'Preview of the selected photo' })
    await actor.click(screen.getByRole('button', { name: 'Replace cover photo' }))
    await screen.findByRole('progressbar')
    expect(screen.getByRole('button', { name: 'Cancel upload' })).toHaveFocus()
    expect(screen.getByRole('img', { name: 'Current cover photo' })).toHaveAttribute('src', oldCover.url)
    const rideReads = reads(fetch, `/api/v1/rides/${rideId}`)
    state.metadata = { image: newCover, version: 2 }; state.ride = { ...state.ride, cover: newCover, cover_version: 2 }
    await act(async () => { finish(response(state.metadata, 201)); await pending })
    await waitFor(() => expect(screen.getByRole('img', { name: 'Current cover photo' })).toHaveAttribute('src', newCover.url))
    expect(screen.getByRole('status')).toHaveTextContent('Cover photo saved.')
    expect(screen.getByLabelText('Choose a photo')).toHaveFocus()
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:cover-preview')
    await waitFor(() => expect(reads(fetch, `/api/v1/rides/${rideId}`)).toBeGreaterThan(rideReads))

    await actor.click(screen.getByRole('button', { name: 'Remove cover photo' }))
    await actor.click(within(await screen.findByRole('dialog', { name: 'Remove cover photo' })).getByRole('button', { name: 'Cancel' }))
    expect(commands).toHaveLength(1)
    await actor.click(screen.getByRole('button', { name: 'Remove cover photo' }))
    await actor.click(within(await screen.findByRole('dialog', { name: 'Remove cover photo' })).getByRole('button', { name: 'Remove cover photo' }))
    await screen.findByText('Until you upload a cover, the ride card shows the city name.')
    expect(screen.queryByRole('img', { name: 'Current cover photo' })).not.toBeInTheDocument()
    expect(screen.getByRole('status')).toHaveTextContent('Cover photo removed.')
    expect(screen.getByLabelText('Choose a photo')).toHaveFocus()
    expect(commands).toEqual([
      { method: 'POST', version: 1, csrf: `csrf-${organizer.id}`, contentType: 'image/webp' },
      { method: 'DELETE', version: 2, csrf: `csrf-${organizer.id}`, contentType: 'application/json' },
    ])
  })

  it('keeps the upload result when an older cover read finishes after it', async () => {
    const state = organizerState()
    let holdRead = false
    let finishRead: () => void = () => { throw new Error('not started') }
    const removed: number[] = []
    const fetch = vi.fn(async (url: string, options?: RequestInit) => {
      if (path(url) !== `/api/v1/rides/${rideId}/cover`) return base(url, state)
      if (options?.method === 'POST') { state.metadata = { image: newCover, version: 2 }; state.ride = { ...state.ride, cover: newCover, cover_version: 2 }; return response(state.metadata, 201) }
      if (options?.method === 'DELETE') { removed.push(JSON.parse(options.body as string).version); state.metadata = { image: null, version: 3 }; return response(state.metadata) }
      if (!holdRead) return base(url, state)
      holdRead = false
      const old = response(state.metadata)
      return new Promise<Response>((resolve) => { finishRead = () => resolve(old) })
    })
    vi.stubGlobal('fetch', fetch)
    const client = mount(`/rides/${rideId}/edit`)
    const actor = userEvent.setup()
    await actor.upload(await screen.findByLabelText('Choose a photo'), new File(['bytes'], 'cover.webp', { type: 'image/webp' }))
    // A background read of the old cover is still running when the replacement succeeds.
    holdRead = true
    act(() => { void client.refetchQueries({ queryKey: ['ride-cover', rideId] }) })
    await waitFor(() => expect(holdRead).toBe(false))
    await actor.click(screen.getByRole('button', { name: 'Replace cover photo' }))
    expect(await screen.findByRole('status')).toHaveTextContent('Cover photo saved.')
    await act(async () => { finishRead() })
    await waitFor(() => expect(client.getQueryState(['ride-cover', rideId, organizer.id])?.fetchStatus).toBe('idle'))
    expect(screen.getByRole('img', { name: 'Current cover photo' })).toHaveAttribute('src', newCover.url)
    await actor.click(screen.getByRole('button', { name: 'Remove cover photo' }))
    await actor.click(within(await screen.findByRole('dialog', { name: 'Remove cover photo' })).getByRole('button', { name: 'Remove cover photo' }))
    await screen.findByText('Until you upload a cover, the ride card shows the city name.')
    expect(removed).toEqual([2])
  })

  it('uploads a first cover with the version from the cover metadata', async () => {
    const state: State = { user: organizer, ride: { ...fullRide, cover_version: 1 }, metadata: { image: null, version: 1 } }
    const fetch = vi.fn(async (url: string, options?: RequestInit) => {
      if (options?.method === 'POST' && path(url) === `/api/v1/rides/${rideId}/cover`) { state.metadata = { image: newCover, version: 2 }; return response(state.metadata, 201) }
      return base(url, state)
    })
    vi.stubGlobal('fetch', fetch)
    mount(`/rides/${rideId}/edit`)
    const actor = userEvent.setup()
    await screen.findByText('Until you upload a cover, the ride card shows the city name.')
    expect(screen.queryByRole('button', { name: 'Remove cover photo' })).not.toBeInTheDocument()
    await actor.upload(screen.getByLabelText('Choose a photo'), new File(['bytes'], 'cover.jpg', { type: 'image/jpeg' }))
    await actor.click(screen.getByRole('button', { name: 'Upload cover photo' }))
    await waitFor(() => expect(screen.getByRole('img', { name: 'Current cover photo' })).toHaveAttribute('src', newCover.url))
    expect(fetch.mock.calls.some(([url, options]) => options?.method === 'POST' && new URL(url, 'https://bikers.kz').search === '?version=1')).toBe(true)
  })

  it.each([
    ['stale_version', 409, 'The cover changed in another tab or window. Reload the latest cover before trying again.', true],
    ['ride_cover_busy', 409, 'Another cover upload for this ride is in progress. Reload the cover before trying again.', false],
    ['photo_storage_limit', 409, 'Photo storage is full. Delete an unneeded photo or try again later.', false],
    ['photo_upload_expired', 409, 'This upload expired. Choose the photo and upload it again.', false],
    ['photo_too_large', 413, 'Choose a smaller photo: up to 12 MiB, 32 megapixels and 12,000 pixels per side.', false],
    ['unsupported_photo', 415, 'Choose a JPEG, PNG or WebP photo. Convert HEIC, AVIF and other formats before uploading.', false],
    ['photo_rate_limited', 429, 'Too many photo uploads. Please wait before trying again.', false],
  ])('keeps the current cover and explains a rejected upload: %s', async (code, status, message, reloads) => {
    const state = organizerState()
    const fetch = vi.fn(async (url: string, options?: RequestInit) => options?.method === 'POST' && path(url) === `/api/v1/rides/${rideId}/cover` ? response({ error: { code } }, status) : base(url, state))
    vi.stubGlobal('fetch', fetch)
    mount(`/rides/${rideId}/edit`)
    const actor = userEvent.setup()
    await actor.upload(await screen.findByLabelText('Choose a photo'), new File(['bytes'], 'cover.png', { type: 'image/png' }))
    const coverReads = reads(fetch, `/api/v1/rides/${rideId}/cover`)
    await actor.click(screen.getByRole('button', { name: 'Replace cover photo' }))
    expect(await screen.findByRole('alert')).toHaveTextContent(message)
    expect(screen.getByRole('img', { name: 'Current cover photo' })).toHaveAttribute('src', oldCover.url)
    expect(screen.getByRole('button', { name: 'Replace cover photo' })).toHaveFocus()
    if (reloads) await waitFor(() => expect(reads(fetch, `/api/v1/rides/${rideId}/cover`)).toBeGreaterThan(coverReads))
    else expect(reads(fetch, `/api/v1/rides/${rideId}/cover`)).toBe(coverReads)
  })

  it('blocks a second upload until an unconfirmed result is reloaded', async () => {
    const state = organizerState()
    vi.stubGlobal('fetch', vi.fn(async (url: string, options?: RequestInit) => {
      if (options?.method === 'POST' && path(url) === `/api/v1/rides/${rideId}/cover`) throw new TypeError('connection lost')
      return base(url, state)
    }))
    mount(`/rides/${rideId}/edit`)
    const actor = userEvent.setup()
    await actor.upload(await screen.findByLabelText('Choose a photo'), new File(['bytes'], 'cover.jpg', { type: 'image/jpeg' }))
    await actor.click(screen.getByRole('button', { name: 'Replace cover photo' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('The upload result could not be confirmed. Reload the cover before uploading again.')
    expect(screen.getByRole('button', { name: 'Replace cover photo' })).toBeDisabled()
    // Choosing another photo does not lift the block: the first upload may still have landed.
    await actor.upload(screen.getByLabelText('Choose a photo'), new File(['other'], 'other.jpg', { type: 'image/jpeg' }))
    await screen.findByRole('img', { name: 'Preview of the selected photo' })
    expect(screen.getByRole('button', { name: 'Replace cover photo' })).toBeDisabled()
    expect(screen.getByRole('alert')).toHaveTextContent('The upload result could not be confirmed. Reload the cover before uploading again.')
    await actor.click(screen.getByRole('button', { name: 'Try again' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Replace cover photo' })).toBeEnabled())
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it.each(['cancels the chosen photo', 'chooses an invalid file'])('keeps an unconfirmed upload blocked when the organizer %s', async (step) => {
    const state = organizerState()
    vi.stubGlobal('fetch', vi.fn(async (url: string, options?: RequestInit) => {
      if (options?.method === 'POST' && path(url) === `/api/v1/rides/${rideId}/cover`) throw new TypeError('connection lost')
      return base(url, state)
    }))
    mount(`/rides/${rideId}/edit`)
    const actor = userEvent.setup()
    const unconfirmed = 'The upload result could not be confirmed. Reload the cover before uploading again.'
    await actor.upload(await screen.findByLabelText('Choose a photo'), new File(['bytes'], 'cover.jpg', { type: 'image/jpeg' }))
    await actor.click(screen.getByRole('button', { name: 'Replace cover photo' }))
    await screen.findByText(unconfirmed)
    if (step === 'cancels the chosen photo') await actor.click(screen.getByRole('button', { name: 'Cancel' }))
    else {
      await actor.upload(screen.getByLabelText('Choose a photo'), new File([], 'empty.jpg', { type: 'image/jpeg' }))
      expect(screen.getByText('Choose a photo that is not empty.')).toBeInTheDocument()
    }
    expect(screen.queryByRole('img', { name: 'Preview of the selected photo' })).not.toBeInTheDocument()
    expect(screen.getByText(unconfirmed)).toBeInTheDocument()
    // The next valid photo stays blocked: the first upload may still have landed.
    await actor.upload(screen.getByLabelText('Choose a photo'), new File(['other'], 'other.jpg', { type: 'image/jpeg' }))
    await screen.findByRole('img', { name: 'Preview of the selected photo' })
    expect(screen.getByRole('button', { name: 'Replace cover photo' })).toBeDisabled()
    expect(screen.getByText(unconfirmed)).toBeInTheDocument()
  })

  it.each(['try again', 'cancel upload'])('reloads the ride feeds after %s, because the upload may have replaced the cover', async (how) => {
    const state = organizerState()
    const fetch = vi.fn(async (url: string, options?: RequestInit) => {
      if (options?.method !== 'POST' || path(url) !== `/api/v1/rides/${rideId}/cover`) return base(url, state)
      // The server stores the new cover, but its reply never arrives.
      state.metadata = { image: newCover, version: 2 }; state.ride = { ...state.ride, cover: newCover, cover_version: 2 }
      if (how === 'try again') throw new TypeError('connection lost')
      return new Promise<Response>((_, reject) => options.signal?.addEventListener('abort', () => reject(new DOMException('Aborted', 'AbortError'))))
    })
    vi.stubGlobal('fetch', fetch)
    const client = mount(`/rides/${rideId}/edit`)
    const picker = await screen.findByLabelText('Choose a photo')
    // The feed was read before the upload and still holds the old cover.
    const feed = ['rides', organizer.id, 'discover', '', '', '', '', '']
    client.setQueryData(feed, { rides: [{ ...summary, cover: oldCover }], next_cursor: '' })
    const actor = userEvent.setup()
    await actor.upload(picker, new File(['bytes'], 'cover.jpg', { type: 'image/jpeg' }))
    await actor.click(screen.getByRole('button', { name: 'Replace cover photo' }))
    if (how === 'try again') {
      await screen.findByText('The upload result could not be confirmed. Reload the cover before uploading again.')
      await actor.click(screen.getByRole('button', { name: 'Try again' }))
    } else await actor.click(await screen.findByRole('button', { name: 'Cancel upload' }))
    await waitFor(() => expect(screen.getByRole('img', { name: 'Current cover photo' })).toHaveAttribute('src', newCover.url))
    expect(client.getQueryState(feed)?.isInvalidated).toBe(true)
  })

  it('returns focus to Remove when a removal fails', async () => {
    const state = organizerState()
    let fail: () => void = () => { throw new Error('not started') }
    vi.stubGlobal('fetch', vi.fn(async (url: string, options?: RequestInit) => options?.method === 'DELETE' && path(url) === `/api/v1/rides/${rideId}/cover`
      ? new Promise<Response>((resolve) => { fail = () => resolve(response({ error: { code: 'internal' } }, 500)) })
      : base(url, state)))
    mount(`/rides/${rideId}/edit`)
    const actor = userEvent.setup()
    const remove = await screen.findByRole('button', { name: 'Remove cover photo' })
    await actor.click(remove)
    await actor.click(within(await screen.findByRole('dialog', { name: 'Remove cover photo' })).getByRole('button', { name: 'Remove cover photo' }))
    await waitFor(() => expect(remove).toBeDisabled())
    // Browsers move focus to the page once the focused control is disabled. JSDOM keeps it on the control.
    document.body.tabIndex = -1; document.body.focus(); document.body.removeAttribute('tabindex')
    expect(document.activeElement).toBe(document.body)
    await act(async () => { fail() })
    await screen.findByRole('alert')
    expect(remove).toBeEnabled()
    expect(remove).toHaveFocus()
  })

  it('lets the organizer replace or remove a cover that moderation hid', async () => {
    const state: State = { user: organizer, ride: { ...fullRide, cover_version: 3 }, metadata: { image: oldCover, version: 3 } }
    vi.stubGlobal('fetch', vi.fn(async (url: string) => base(url, state)))
    mount(`/rides/${rideId}/edit`)
    fireEvent.error(await screen.findByRole('img', { name: 'Current cover photo' }))
    await screen.findByText('The current cover can’t be shown. It may have been hidden after a report. You can replace or remove it.')
    expect(screen.getByRole('button', { name: 'Remove cover photo' })).toBeEnabled()
  })

  it('offers no editor on a cancelled ride and never reads the cover metadata', async () => {
    const state: State = { user: organizer, ride: { ...fullRide, status: 'cancelled', cover: oldCover }, metadata: { image: oldCover } }
    const fetch = vi.fn(async (url: string) => base(url, state))
    vi.stubGlobal('fetch', fetch)
    mount(`/rides/${rideId}/edit`)
    await screen.findByText('The cover of a cancelled ride can’t be changed.')
    expect(screen.queryByLabelText('Choose a photo')).not.toBeInTheDocument()
    expect(reads(fetch, `/api/v1/rides/${rideId}/cover`)).toBe(0)
  })

  it('explains on a new ride that the cover comes after creation', async () => {
    const state = organizerState()
    vi.stubGlobal('fetch', vi.fn(async (url: string) => base(url, state)))
    mount('/rides/new')
    await screen.findByText('You can add a cover photo once the ride is created: open the ride and choose Edit plan. Until then, the ride card shows the city name.')
    expect(screen.queryByLabelText('Choose a photo')).not.toBeInTheDocument()
    expect(within(screen.getByRole('main')).getByRole('link', { name: 'Back to rides' })).toHaveAttribute('href', '/')
  })

  it('leads back to the ride, not the feed, from the edit form', async () => {
    const state = organizerState()
    vi.stubGlobal('fetch', vi.fn(async (url: string) => base(url, state)))
    mount(`/rides/${rideId}/edit`)
    expect(await within(screen.getByRole('main')).findByRole('link', { name: 'Back to ride' })).toHaveAttribute('href', `/rides/${rideId}`)
  })

  it.each([['ru', 'Обложка видна всем, как город и описание, и остаётся видимой после отмены поездки. Не выбирайте фото, по которому можно узнать место встречи.', 'Удалить обложку'], ['kk', 'Мұқаба қала мен сипаттама сияқты бәріне көрінеді және сапар тоқтатылғаннан кейін де қалады. Кездесу орнын анықтауға болатын фотоны таңдамаңыз.', 'Мұқабаны жою']])('localizes the editor in %s', async (locale, note, action) => {
    localStorage.setItem('bikerskz.locale', locale)
    const state = organizerState()
    vi.stubGlobal('fetch', vi.fn(async (url: string) => base(url, state)))
    mount(`/rides/${rideId}/edit`)
    await screen.findByText(note)
    expect(screen.getByRole('button', { name: action })).toBeInTheDocument()
  })
})
