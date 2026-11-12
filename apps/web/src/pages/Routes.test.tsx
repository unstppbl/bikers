import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, Route, Routes, useLocation, useNavigate } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { App } from '../App'
import { setCsrfToken } from '../api'
import { I18nProvider } from '../i18n'
import { bindRecorderAccount } from '../route-recorder'
import type { CommunityRoute, RecordedTrack } from '../routes'
import { SessionProvider, useSession } from '../session'
import type { User } from '../types'
import { RouteDetailPage, RouteFormPage, RoutesPage, TrackRecorderPage } from './Routes'

vi.mock('virtual:pwa-register/react', () => ({ useRegisterSW: () => ({ needRefresh: [false, vi.fn()], updateServiceWorker: vi.fn() }) }))
vi.mock('../components/maps/MapCanvas', () => ({ default: ({ segments }: { segments?: number[][][] }) => <div aria-label="Route map preview">{segments?.length ?? 0} segments</div> }))

const rider: User = { id: 'rider-a', profile: { display_name: 'Rider A', city: 'Almaty', languages: ['en'], interests: '', locale: 'en' }, onboarding_complete: true }
const nextRider: User = { ...rider, id: 'rider-b' }
const segments: [number, number][][] = [[[76.95, 43.25], [76.951, 43.25]], [[76.96, 43.26], [76.961, 43.26]]]
const route: CommunityRoute = { id: 'route-a', owner_id: rider.id, owner_name: 'Rider A', title: 'Foothill loop', city: 'Almaty', description: 'Two roads with a pause between them.', surface: 'asphalt', kind: 'recorded', published: true, version: 1, created_at: '2026-11-11T06:00:00Z', updated_at: '2026-11-11T06:00:00Z', distance_meters: 163, duration_seconds: 120, points: [], segments }
const track: RecordedTrack = { id: 'track-a', name: 'My private recording', segments, distance_meters: 163, duration_seconds: 120, version: 1, created_at: route.created_at }
const response = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json' } })
const pathname = (value: string) => new URL(value, 'https://bikers.kz').pathname
function ancillary(url: string) {
  if (url.endsWith('/places/config')) return response({ enabled: true, archive_url: '/map-data/archive/kazakhstan.pmtiles' })
  if (url.endsWith('/geo/config')) return response({ enabled: false, archive_url: '' })
  if (url.endsWith('/cities')) return response({ cities: [] })
  if (url.endsWith('/me/trust')) return response({ is_admin: false })
  if (url.endsWith('/me/notifications/unread-count')) return response({ unread_count: 0 })
  if (url.includes('/avatar')) return response({ image: null, version: 1 })
  if (url.endsWith('/me/blocks')) return response({ blocks: [], next_cursor: '' })
  return undefined
}
function Controls() {
  const location = useLocation(), session = useSession(), navigate = useNavigate()
  return <><output data-testid="location">{location.pathname + location.search}</output><button onClick={() => session.accept({ user: nextRider, csrf_token: 'csrf-b' })}>Switch account</button><button onClick={() => void navigate('/routes?tab=tracks')}>Leave recorder</button><button onClick={() => void navigate('/routes/record')}>Return to recorder</button></>
}
function mount(path: string, app = false) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
  render(<QueryClientProvider client={client}><MemoryRouter initialEntries={[path]}><I18nProvider><SessionProvider>{app ? <App /> : <Routes><Route path="/routes" element={<RoutesPage />} /><Route path="/routes/new" element={<RouteFormPage />} /><Route path="/routes/record" element={<TrackRecorderPage />} /><Route path="/routes/:routeId" element={<RouteDetailPage />} /><Route path="/routes/:routeId/edit" element={<RouteFormPage edit />} /></Routes>}<Controls /></SessionProvider></I18nProvider></MemoryRouter></QueryClientProvider>)
  return client
}
beforeEach(() => { setCsrfToken(); bindRecorderAccount(null) })
afterEach(() => { bindRecorderAccount(null); setCsrfToken(); vi.unstubAllGlobals() })

describe('community routes', () => {
  it('keeps only route summaries in list caches and applies bounded server filters', async () => {
    const fetch = vi.fn(async (url: string) => {
      if (url.endsWith('/session')) return response({ user: rider, csrf_token: 'csrf-a' })
      if (pathname(url) === '/api/v1/routes') return response({ routes: [{ ...route, source_track_id: 'private-track' }], next_cursor: '' })
      const extra = ancillary(url); if (extra) return extra
      throw new Error(url)
    })
    vi.stubGlobal('fetch', fetch)
    const client = mount('/routes')
    await screen.findByRole('heading', { name: 'Foothill loop' })
    const cached = client.getQueriesData<{ routes: Record<string, unknown>[] }>({ queryKey: ['community-routes'] })[0][1]!
    expect(cached.routes[0]).not.toHaveProperty('segments')
    expect(cached.routes[0]).not.toHaveProperty('source_track_id')
    fireEvent.change(screen.getByRole('searchbox', { name: 'Search routes' }), { target: { value: ' Gravel ' } })
    fireEvent.change(screen.getByRole('combobox', { name: 'Route city' }), { target: { value: 'Astana' } })
    await userEvent.click(screen.getByRole('button', { name: 'Apply filters' }))
    await waitFor(() => expect(fetch.mock.calls.some(([url]) => new URL(url, 'https://bikers.kz').searchParams.get('q') === 'Gravel')).toBe(true))
    expect(screen.getByTestId('location')).toHaveTextContent('q=Gravel&city=Astana')
    expect(fetch.mock.calls.every(([url]) => !/\/tracks\b|\/route-collections\b/.test(url))).toBe(true)
  })

  it('requires fresh publication consent and previews a separate private track before creating a route', async () => {
    const writes: Record<string, unknown>[] = []
    vi.stubGlobal('fetch', vi.fn(async (url: string, options?: RequestInit) => {
      if (url.endsWith('/session')) return response({ user: rider, csrf_token: 'csrf-a' })
      if (url.endsWith('/tracks/track-a')) return response({ track })
      if (pathname(url) === '/api/v1/routes' && options?.method === 'POST') { writes.push(JSON.parse(String(options.body))); return response({ route }) }
      if (url.endsWith('/routes/route-a')) return response({ route })
      if (url.includes('/conditions')) return response({ conditions: [], next_cursor: '' })
      if (url.includes('/route-collections')) return response({ collections: [], next_cursor: '' })
      const extra = ancillary(url); if (extra) return extra
      throw new Error(url)
    }))
    mount('/routes/new?track=track-a')
    await screen.findByRole('heading', { name: 'Create route from this track' })
    expect(await screen.findByLabelText('Route map preview')).toHaveTextContent('2 segments')
    expect(screen.getByRole('checkbox', { name: 'Publish to the community' })).not.toBeChecked()
    await userEvent.click(screen.getByRole('checkbox', { name: 'Publish to the community' }))
    await userEvent.click(screen.getByRole('button', { name: 'Save route' }))
    expect(writes).toEqual([])
    await userEvent.click(screen.getByRole('checkbox', { name: /I have checked the map/ }))
    await userEvent.click(screen.getByRole('button', { name: 'Save route' }))
    await screen.findByRole('heading', { name: 'Foothill loop' })
    expect(writes).toEqual([expect.objectContaining({ track_id: 'track-a', published: true, publish_consent: true })])
    expect(writes[0]).not.toHaveProperty('segments')
  })

  it('shows report freshness and author dates instead of implying old reports remain current', async () => {
    vi.stubGlobal('fetch', vi.fn(async (url: string) => {
      if (url.endsWith('/session')) return response({ user: null })
      if (url.endsWith('/routes/route-a')) return response({ route })
      if (url.includes('/conditions')) return response({ conditions: [{ id: 'condition-a', route_id: route.id, author_id: 'rider-b', author_name: 'Dana', kind: 'gravel', description: 'Loose gravel near the turn.', observed_at: '2026-09-01T06:00:00Z', updated_at: '2026-09-02T06:00:00Z', expires_at: '2026-10-01T06:00:00Z', resolved: false, version: 1, state: 'expired' }], next_cursor: '' })
      const extra = ancillary(url); if (extra) return extra
      throw new Error(url)
    }))
    mount('/routes/route-a')
    const card = await screen.findByRole('heading', { name: 'Loose gravel' })
    const article = card.closest('article')!
    expect(within(article).getByText('Older report')).toBeInTheDocument()
    expect(article).toHaveTextContent('Dana')
    expect(article).toHaveTextContent(/Observed.*Updated/)
    expect(screen.queryByRole('button', { name: 'Add a report' })).not.toBeInTheDocument()
  })

  it('requires a confirmation before deleting a route', async () => {
    const writes: string[] = []
    vi.stubGlobal('fetch', vi.fn(async (url: string, options?: RequestInit) => {
      if (url.endsWith('/session')) return response({ user: rider, csrf_token: 'csrf-a' })
      if (url.endsWith('/routes/route-a')) { if (options?.method === 'DELETE') { writes.push(String(options.body)); return response({ ok: true }) }; return response({ route }) }
      if (pathname(url) === '/api/v1/routes') return response({ routes: [], next_cursor: '' })
      if (url.includes('/conditions')) return response({ conditions: [], next_cursor: '' })
      if (url.includes('/route-collections')) return response({ collections: [], next_cursor: '' })
      const extra = ancillary(url); if (extra) return extra
      throw new Error(url)
    }))
    mount('/routes/route-a')
    await userEvent.click(await screen.findByRole('button', { name: 'Delete route' }))
    expect(writes).toEqual([])
    await userEvent.click(within(screen.getByRole('dialog', { name: 'Delete route' })).getByRole('button', { name: 'Cancel' }))
    expect(writes).toEqual([])
    await userEvent.click(screen.getByRole('button', { name: 'Delete route' }))
    await userEvent.click(within(screen.getByRole('dialog', { name: 'Delete route' })).getByRole('button', { name: 'Delete route' }))
    await waitFor(() => expect(writes).toEqual(['{"version":1}']))
  })

  it.each([['ru', 'Маршруты', 'Дороги, которыми хочется делиться.'], ['kk', 'Бағыттар', 'Бөлісуге тұрарлық жолдар.']])('localizes the route hub and navigation in %s', async (locale, nav, heading) => {
    localStorage.setItem('bikerskz.locale', locale)
    vi.stubGlobal('fetch', vi.fn(async (url: string) => {
      if (url.endsWith('/session')) return response({ user: null })
      if (pathname(url) === '/api/v1/routes') return response({ routes: [], next_cursor: '' })
      const extra = ancillary(url); if (extra) return extra
      throw new Error(url)
    }))
    mount('/routes', true)
    await screen.findByRole('heading', { name: heading })
    expect(screen.getByRole('link', { name: nav })).toHaveAttribute('aria-current', 'page')
  })
})

describe('GPS recording screen', () => {
  it('keeps a newer recording after leaving and reopening while an older save is pending', async () => {
    const fixes: PositionCallback[] = []
    const geo: Geolocation = { getCurrentPosition: vi.fn(), watchPosition: vi.fn((callback) => { fixes.push(callback); return fixes.length }), clearWatch: vi.fn() }
    Object.defineProperty(navigator, 'geolocation', { value: geo, configurable: true })
    let finish: (response: Response) => void = () => { throw new Error('No pending request') }
    const pending = new Promise<Response>((resolve) => { finish = resolve })
    const writes: unknown[] = []
    vi.stubGlobal('fetch', vi.fn(async (url: string, options?: RequestInit) => {
      if (url.endsWith('/session')) return response({ user: rider, csrf_token: 'csrf-a' })
      if (pathname(url) === '/api/v1/tracks' && options?.method === 'POST') { writes.push(JSON.parse(String(options.body))); return pending }
      if (pathname(url) === '/api/v1/tracks') return response({ tracks: [], next_cursor: '' })
      const extra = ancillary(url); if (extra) return extra
      throw new Error(url)
    }))
    mount('/routes/record')
    await screen.findByRole('heading', { name: 'Record your ride.' })
    fireEvent.change(screen.getByLabelText(/^Track name/), { target: { value: 'First recording' } })
    await userEvent.click(screen.getByRole('button', { name: 'Start recording' }))
    function point(lon: number, callback: PositionCallback, timestamp: number) { callback({ timestamp, coords: { longitude: lon, latitude: 43.25, accuracy: 5, altitude: null, altitudeAccuracy: null, heading: null, speed: null, toJSON: () => ({}) }, toJSON: () => ({}) }) }
    act(() => { point(76.95, fixes[0], Date.now() - 10000); point(76.951, fixes[0], Date.now()) })
    await userEvent.click(screen.getByRole('button', { name: 'Finish and save privately' }))
    await waitFor(() => expect(writes).toHaveLength(1))
    await userEvent.click(screen.getByRole('button', { name: 'Leave recorder' }))
    await screen.findByRole('heading', { name: 'Your rides, recorded privately.' })
    await userEvent.click(screen.getByRole('button', { name: 'Return to recorder' }))
    await userEvent.click(await screen.findByRole('button', { name: 'Resume recording' }))
    act(() => { point(76.96, fixes[1], Date.now() - 10000); point(76.961, fixes[1], Date.now()) })
    await act(async () => { finish(response({ track })); await pending })
    expect(screen.getByTestId('location')).toHaveTextContent('/routes/record')
    expect(screen.getByText('4 points')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Pause recording' })).toBeInTheDocument()
    expect(localStorage.getItem('bikerskz.route-draft.rider-a')).toContain('76.961')
  })

  it('keeps a failed save private and retryable, then clears the draft after durable success', async () => {
    let success: PositionCallback | undefined
    const geo: Geolocation = { getCurrentPosition: vi.fn(), watchPosition: vi.fn((callback) => { success = callback; return 1 }), clearWatch: vi.fn() }
    Object.defineProperty(navigator, 'geolocation', { value: geo, configurable: true })
    let fails = true; const writes: unknown[] = []
    vi.stubGlobal('fetch', vi.fn(async (url: string, options?: RequestInit) => {
      if (url.endsWith('/session')) return response({ user: rider, csrf_token: 'csrf-a' })
      if (pathname(url) === '/api/v1/tracks' && options?.method === 'POST') { writes.push(JSON.parse(String(options.body))); return fails ? response({ error: { code: 'network_error' } }, 503) : response({ track }) }
      if (pathname(url) === '/api/v1/tracks') return response({ tracks: [track], next_cursor: '' })
      const extra = ancillary(url); if (extra) return extra
      throw new Error(url)
    }))
    mount('/routes/record')
    await screen.findByRole('heading', { name: 'Record your ride.' })
    expect(geo.watchPosition).not.toHaveBeenCalled()
    fireEvent.change(screen.getByLabelText(/^Track name/), { target: { value: 'Private Saturday ride' } })
    await userEvent.click(screen.getByRole('button', { name: 'Start recording' }))
    const now = Date.now()
    function point(lon: number, timestamp: number) { success?.({ timestamp, coords: { longitude: lon, latitude: 43.25, accuracy: 5, altitude: null, altitudeAccuracy: null, heading: null, speed: null, toJSON: () => ({}) }, toJSON: () => ({}) }) }
    act(() => { point(76.95, now - 10000); point(76.951, now) })
    await userEvent.click(screen.getByRole('button', { name: 'Finish and save privately' }))
    await screen.findByText('Your recording is still on this device. Reconnect and retry saving.')
    expect(localStorage.getItem('bikerskz.route-draft.rider-a')).toContain('76.951')
    expect(writes).toEqual([expect.objectContaining({ name: 'Private Saturday ride', segments: [[[76.95, 43.25], [76.951, 43.25]]] })])
    fails = false
    await userEvent.click(screen.getByRole('button', { name: 'Finish and save privately' }))
    await screen.findByRole('heading', { name: 'My private recording' })
    expect(localStorage.getItem('bikerskz.route-draft.rider-a')).toBeNull()
    expect(geo.clearWatch).toHaveBeenCalledWith(1)
  })

  it('stops the previous account GPS watch and does not expose its draft after account switch', async () => {
    let success: PositionCallback | undefined
    const geo: Geolocation = { getCurrentPosition: vi.fn(), watchPosition: vi.fn((callback) => { success = callback; return 7 }), clearWatch: vi.fn() }
    Object.defineProperty(navigator, 'geolocation', { value: geo, configurable: true })
    vi.stubGlobal('fetch', vi.fn(async (url: string) => {
      if (url.endsWith('/session')) return response({ user: rider, csrf_token: 'csrf-a' })
      const extra = ancillary(url); if (extra) return extra
      throw new Error(url)
    }))
    mount('/routes/record')
    await userEvent.click(await screen.findByRole('button', { name: 'Start recording' }))
    const fix = { timestamp: Date.now(), coords: { longitude: 76.95, latitude: 43.25, accuracy: 5, altitude: null, altitudeAccuracy: null, heading: null, speed: null, toJSON: () => ({}) }, toJSON: () => ({}) }
    act(() => success?.(fix))
    expect(localStorage.getItem('bikerskz.route-draft.rider-a')).toContain('76.95')
    await userEvent.click(screen.getByRole('button', { name: 'Switch account' }))
    expect(geo.clearWatch).toHaveBeenCalledWith(7)
    act(() => success?.({ ...fix, timestamp: Date.now() + 10000 }))
    expect(screen.getByText('0 points')).toBeInTheDocument()
    expect(screen.queryByLabelText('Route map preview')).not.toBeInTheDocument()
    expect(localStorage.getItem('bikerskz.route-draft.rider-a') ?? '').not.toContain('76.95')
  })
})
