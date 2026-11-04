import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { App } from '../App'
import { setCsrfToken } from '../api'
import { I18nProvider } from '../i18n'
import { SessionProvider, useSession } from '../session'
import type { Club, ClubImageMetadata, ClubImagePhoto, ClubSummary, User } from '../types'
import { safeClubImage } from './ClubImage'

vi.mock('virtual:pwa-register/react', () => ({ useRegisterSW: () => ({ needRefresh: [false, vi.fn()], updateServiceWorker: vi.fn() }) }))

const rider: User = { id: 'rider-a', profile: { display_name: 'Rider A', city: 'Almaty', languages: ['en'], interests: '', locale: 'en' }, onboarding_complete: true }
const other: User = { ...rider, id: 'rider-b', profile: { ...rider.profile, display_name: 'Rider B' } }
const club: Club = { id: '11111111-1111-4111-8111-111111111111', name: 'Test Riders', city: 'Almaty', description: 'A test riding club', status: 'active', owner_id: rider.id, version: 1, member_count: 2, membership_state: 'approved', can_manage: true }
const oldImage: ClubImagePhoto = { id: '22222222-2222-4222-8222-222222222222', url: `/api/v1/clubs/${club.id}/image/22222222-2222-4222-8222-222222222222/content`, width: 1200, height: 800, created_at: '2026-11-03T10:00:00Z' }
const newImage: ClubImagePhoto = { ...oldImage, id: '33333333-3333-4333-8333-333333333333', url: `/api/v1/clubs/${club.id}/image/33333333-3333-4333-8333-333333333333/content` }
const response = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json' } })
interface State { club: Club | ClubSummary; metadata: ClubImageMetadata; user: User | null; failedSession?: boolean }
function state(): State { return { club, metadata: { image: oldImage, version: 1 }, user: rider } }
function base(url: string, current: State) {
  if (url.endsWith('/session')) return current.failedSession ? response({ error: { code: 'service_unavailable' } }, 503) : response({ user: current.user, ...(current.user ? { csrf_token: `csrf-${current.user.id}` } : {}) })
  if (url.endsWith(`/clubs/${club.id}/image`)) return response(current.metadata)
  if (url.endsWith(`/clubs/${club.id}`)) return response({ club: current.club })
  if (url.endsWith('/clubs')) return response({ clubs: [current.club], next_cursor: '' })
  if (url.endsWith('/members')) return response({ members: [{ user_id: rider.id, display_name: rider.profile.display_name, state: 'approved', role: 'owner' }, { user_id: other.id, display_name: other.profile.display_name, state: 'approved', role: 'member' }] })
  if (url.endsWith('/rides')) return response({ rides: [] })
  throw new Error(`Unexpected API ${url}`)
}
function SessionControls() {
  const { clear, accept } = useSession()
  return <><button onClick={clear}>Expire test session</button><button onClick={() => accept({ user: other, csrf_token: 'csrf-rider-b' })}>Switch test actor</button></>
}
function mount(path = `/clubs/${club.id}`) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: Infinity }, mutations: { retry: false } } })
  render(<QueryClientProvider client={client}><MemoryRouter initialEntries={[path]}><I18nProvider><SessionProvider><SessionControls /><App /></SessionProvider></I18nProvider></MemoryRouter></QueryClientProvider>)
  return client
}
beforeEach(() => {
  setCsrfToken()
  Object.defineProperty(URL, 'createObjectURL', { configurable: true, writable: true, value: vi.fn(() => 'blob:club-preview') })
  Object.defineProperty(URL, 'revokeObjectURL', { configurable: true, writable: true, value: vi.fn() })
})
afterEach(() => { setCsrfToken(); vi.unstubAllGlobals() })

describe('club image projection and public display', () => {
  it('keeps only five image fields and owner-only version, and rejects foreign or mismatched content routes', () => {
    const raw = { image: { ...oldImage, owner_id: 'secret', filename: 'gps-original.jpg', content: 'bytes' }, version: 7, uploader: 'secret' }
    expect(safeClubImage(raw, club.id, false)).toEqual({ image: oldImage })
    expect(safeClubImage(raw, club.id, true)).toEqual({ image: oldImage, version: 7 })
    for (const url of ['https://example.org/image.jpg', '//example.org/image.jpg', `${oldImage.url}?secret=1`, oldImage.url.replace('/content', '/content/../other'), oldImage.url.replace(club.id, newImage.id)]) {
      expect(safeClubImage({ image: { ...oldImage, url }, version: 7 }, club.id, false)).toEqual({ image: null })
    }
    expect(safeClubImage({ image: null }, club.id, false)).toEqual({ image: null })
  })

  it('shows the public card image and returns to initials on a failed image', async () => {
    const current = state(); current.user = null
    vi.stubGlobal('fetch', vi.fn(async (url: string) => base(url, current)))
    const client = mount('/clubs')
    const image = await screen.findByRole('img', { name: 'Image of Test Riders' })
    expect(image).toHaveAttribute('src', oldImage.url)
    expect(client.getQueryData(['club-image', club.id, 'guest', 'public'])).toEqual({ image: oldImage })
    expect(client.getQueryCache().find({ queryKey: ['club-image', club.id, 'guest', 'public'] })?.meta?.private).toBe(true)
    fireEvent.error(image)
    expect(screen.queryByRole('img', { name: 'Image of Test Riders' })).not.toBeInTheDocument()
    expect(screen.getByText('TR')).toBeInTheDocument()
  })

  it('keeps a closed club image public and exposes no image editor', async () => {
    const current = state(); current.user = null; current.club = { ...club, status: 'closed', can_manage: false }
    vi.stubGlobal('fetch', vi.fn(async (url: string) => base(url, current)))
    mount()
    await screen.findByRole('img', { name: 'Image of Test Riders' })
    expect(screen.queryByLabelText('Choose a photo')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Remove club image' })).not.toBeInTheDocument()
  })

  it('does not fetch cookie-dependent image metadata after session failure and sanitizes guest recovery', async () => {
    const current = state(); current.failedSession = true
    const fetch = vi.fn(async (url: string) => base(url, current))
    vi.stubGlobal('fetch', fetch)
    const client = mount('/clubs')
    await screen.findByText('We couldn’t reach BIKERS.KZ.')
    expect(fetch.mock.calls.some(([url]) => url.endsWith('/image'))).toBe(false)
    current.failedSession = false; current.user = null
    await userEvent.click(screen.getByRole('button', { name: 'Try again' }))
    await screen.findByRole('img', { name: 'Image of Test Riders' })
    expect(client.getQueryData(['club-image', club.id, 'guest', 'public'])).toEqual({ image: oldImage })
    expect(screen.queryByLabelText('Choose a photo')).not.toBeInTheDocument()
  })
})

describe('single-image owner editing', () => {
  it('keeps the old image during a raw versioned replacement, then confirms removal and restores initials', async () => {
    const current = state()
    let finish: (value: Response) => void = () => { throw new Error('not started') }
    const pending = new Promise<Response>((resolve) => { finish = resolve })
    const commands: { method: string; version: number; csrf: string | null; contentType: string | null }[] = []
    const fetch = vi.fn(async (url: string, options?: RequestInit) => {
      if (url.includes(`/clubs/${club.id}/image?version=`) && options?.method === 'POST') {
        expect(options.body).toBeInstanceOf(File)
        commands.push({ method: 'POST', version: Number(new URL(url, 'http://example.org').searchParams.get('version')), csrf: new Headers(options.headers).get('X-CSRF-Token'), contentType: new Headers(options.headers).get('Content-Type') })
        return pending
      }
      if (url.endsWith('/image') && options?.method === 'DELETE') {
        commands.push({ method: 'DELETE', version: JSON.parse(options.body as string).version, csrf: new Headers(options.headers).get('X-CSRF-Token'), contentType: new Headers(options.headers).get('Content-Type') })
        current.metadata = { image: null, version: 3 }
        return response(current.metadata)
      }
      return base(url, current)
    })
    vi.stubGlobal('fetch', fetch)
    mount()
    const actor = userEvent.setup()
    await actor.upload(await screen.findByLabelText('Choose a photo'), new File(['bytes'], 'club.webp', { type: 'image/webp' }))
    await screen.findByRole('img', { name: 'Preview of the selected photo' })
    await actor.click(screen.getByRole('button', { name: 'Replace club image' }))
    expect(await screen.findByRole('progressbar')).not.toHaveAttribute('aria-valuenow')
    expect(screen.getAllByRole('img', { name: 'Image of Test Riders' }).every((image) => image.getAttribute('src') === oldImage.url)).toBe(true)
    current.metadata = { image: newImage, version: 2 }
    await act(async () => { finish(response(current.metadata, 201)); await pending })
    await waitFor(() => expect(screen.getAllByRole('img', { name: 'Image of Test Riders' }).every((image) => image.getAttribute('src') === newImage.url)).toBe(true))
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:club-preview')
    await actor.click(screen.getByRole('button', { name: 'Remove club image' }))
    await actor.click(within(await screen.findByRole('dialog', { name: 'Remove club image' })).getByRole('button', { name: 'Cancel' }))
    expect(commands).toHaveLength(1)
    await actor.click(screen.getByRole('button', { name: 'Remove club image' }))
    await actor.click(within(await screen.findByRole('dialog', { name: 'Remove club image' })).getByRole('button', { name: 'Remove club image' }))
    await waitFor(() => expect(screen.queryByRole('img', { name: 'Image of Test Riders' })).not.toBeInTheDocument())
    expect(screen.getByText('The club’s initials are shown until you upload an image.')).toBeInTheDocument()
    expect(commands).toEqual([{ method: 'POST', version: 1, csrf: 'csrf-rider-a', contentType: 'image/webp' }, { method: 'DELETE', version: 2, csrf: 'csrf-rider-a', contentType: 'application/json' }])
  })

  it('preserves the old image on rejected replacement and uses club-specific busy/choice copy', async () => {
    const current = state()
    let code = 'club_image_busy'
    vi.stubGlobal('fetch', vi.fn(async (url: string, options?: RequestInit) => {
      if (options?.method === 'POST' && url.includes('/image?')) return response({ error: { code } }, code === 'invalid_photo_choice' ? 422 : 409)
      return base(url, current)
    }))
    mount()
    const actor = userEvent.setup()
    await actor.upload(await screen.findByLabelText('Choose a photo'), new File(['bytes'], 'club.png', { type: 'image/png' }))
    await actor.click(screen.getByRole('button', { name: 'Replace club image' }))
    await screen.findByText('Another club image upload is in progress. Reload the image before trying again.')
    expect(screen.getAllByRole('img', { name: 'Image of Test Riders' }).every((image) => image.getAttribute('src') === oldImage.url)).toBe(true)
    code = 'invalid_photo_choice'
    await actor.click(screen.getByRole('button', { name: 'Replace club image' }))
    await screen.findByText('The club image changed. Reload the latest image before trying again.')
    expect(screen.queryByText(/6 photos per/)).not.toBeInTheDocument()
  })

  it.each([['ru', 'Выбрать фото', 'Это изображение видно всем, в том числе после закрытия клуба.'], ['kk', 'Фото таңдау', 'Бұл сурет клуб жабылғаннан кейін де бәріне көрінеді.']])('localizes the owner editor and public-visibility note in %s', async (locale, picker, note) => {
    localStorage.setItem('bikerskz.locale', locale)
    const current = state()
    vi.stubGlobal('fetch', vi.fn(async (url: string) => base(url, current)))
    mount()
    await screen.findByLabelText(picker)
    expect(screen.getByText(note)).toBeInTheDocument()
  })
})

describe('upload session and club authority changes', () => {
  it('discards a completion when the live club cache loses authority before React can unmount the editor', async () => {
    const current = state()
    let finish: (value: Response) => void = () => { throw new Error('not started') }
    const pending = new Promise<Response>((resolve) => { finish = resolve })
    vi.stubGlobal('fetch', vi.fn(async (url: string, options?: RequestInit) => options?.method === 'POST' && url.includes('/image?') ? pending : base(url, current)))
    const client = mount()
    const actor = userEvent.setup()
    await actor.upload(await screen.findByLabelText('Choose a photo'), new File(['bytes'], 'new.jpg', { type: 'image/jpeg' }))
    await actor.click(screen.getByRole('button', { name: 'Replace club image' }))
    await screen.findByRole('progressbar')
    current.club = { ...club, owner_id: other.id, can_manage: false, version: 2 }
    await act(async () => {
      client.setQueryData(['club', club.id, rider.id], { club: current.club })
      finish(response({ image: newImage, version: 2 }, 201))
      await pending
    })
    await waitFor(() => expect(screen.queryByLabelText('Choose a photo')).not.toBeInTheDocument())
    expect(client.getQueryData<ClubImageMetadata>(['club-image', club.id, rider.id, 'owner'])?.image?.id).not.toBe(newImage.id)
    expect(screen.getAllByRole('img', { name: 'Image of Test Riders' }).every((image) => image.getAttribute('src') === oldImage.url)).toBe(true)
  })

  it.each(['logout', 'actor switch', 'transfer', 'closure', 'ABA version', 'cancel'])('aborts, revokes and rejects a late replacement after %s', async (change) => {
    const current = state()
    let finish: (value: Response) => void = () => { throw new Error('not started') }
    const pending = new Promise<Response>((resolve) => { finish = resolve })
    let signal: AbortSignal | undefined
    vi.stubGlobal('fetch', vi.fn(async (url: string, options?: RequestInit) => {
      if (options?.method === 'POST' && url.includes('/image?')) { signal = options.signal!; return pending }
      return base(url, current)
    }))
    const client = mount()
    const actor = userEvent.setup()
    await actor.upload(await screen.findByLabelText('Choose a photo'), new File(['bytes'], 'new.jpg', { type: 'image/jpeg' }))
    await screen.findByRole('img', { name: 'Preview of the selected photo' })
    await actor.click(screen.getByRole('button', { name: 'Replace club image' }))
    await screen.findByRole('progressbar')
    if (change === 'logout') { current.user = null; current.club = { ...club, can_manage: false }; await actor.click(screen.getByRole('button', { name: 'Expire test session' })) }
    else if (change === 'actor switch') { current.user = other; current.club = { ...club, can_manage: false }; await actor.click(screen.getByRole('button', { name: 'Switch test actor' })) }
    else if (change === 'cancel') await actor.click(screen.getByRole('button', { name: 'Cancel upload' }))
    else {
      current.club = change === 'transfer' ? { ...club, owner_id: other.id, can_manage: false, version: 2 } : change === 'closure' ? { ...club, status: 'closed', can_manage: false, version: 2 } : { ...club, version: 3 }
      await act(async () => { await client.invalidateQueries({ queryKey: ['club', club.id] }) })
    }
    await waitFor(() => expect(signal?.aborted).toBe(true))
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:club-preview')
    await act(async () => { finish(response({ image: newImage, version: 2 }, 201)); await pending })
    await waitFor(() => expect(screen.queryByRole('img', { name: 'Preview of the selected photo' })).not.toBeInTheDocument())
    expect(screen.getAllByRole('img', { name: 'Image of Test Riders' }).every((image) => image.getAttribute('src') === oldImage.url)).toBe(true)
    const cached = client.getQueriesData<ClubImageMetadata>({ queryKey: ['club-image', club.id] })
    expect(cached.every(([, value]) => value?.image?.id !== newImage.id)).toBe(true)
    if (['logout', 'actor switch', 'transfer', 'closure'].includes(change)) expect(client.getQueryData(['club-image', club.id, rider.id, 'owner'])).toBeUndefined()
    if (['logout', 'actor switch', 'transfer', 'closure'].includes(change)) expect(screen.queryByLabelText('Choose a photo')).not.toBeInTheDocument()
  })
})
