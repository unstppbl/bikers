import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, Route, Routes, useLocation, useNavigate } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { setCsrfToken } from '../api'
import { I18nProvider } from '../i18n'
import { RequireUser, SessionProvider, useSession } from '../session'
import type { Place, PlaceSubmission, User } from '../types'
import { PlacesPage } from './Places'
import { MyPlaceSubmissionsPage, PlaceSubmissionPage } from './PlaceSubmissions'
import { AdminPage } from './Admin'
vi.mock('../components/places/PlacesMap', () => ({ default: ({ places, onViewport, onSelect }: { places: Place[]; onViewport: (b: object) => void; onSelect: (id: string) => void }) => <div><button onClick={() => onViewport({ west: 76, south: 43, east: 77, north: 44 })}>Pan test map</button>{places.map((p) => <button key={p.id} onClick={() => onSelect(p.id)}>{'Map ' + p.name}</button>)}</div> }))
vi.mock('../components/maps/MapCanvas', () => ({ default: () => <p>Point map test surface</p> }))
const rider: User = { id: 'rider-a', profile: { display_name: 'A', city: 'Almaty', languages: ['en'], interests: '', locale: 'en' }, onboarding_complete: true }
const place: Place = { id: 'place-a', name: 'Road workshop', category: 'workshop', lat: 43.2, lon: 76.9, city: 'Almaty', address: 'Abay avenue', description: '', website: '', phone: '', opening_hours: '', source: 'osm', source_ref: 'node/12', source_url: 'https://www.openstreetmap.org/node/12', source_date: '2026-11-01', updated_at: '2026-11-05T00:00:00Z', version: 3, status: 'published' }
const proposal: PlaceSubmission = { id: 'suggestion-a', kind: 'correction', place_id: place.id, base_version: 3, place: { ...place, name: 'Updated workshop' }, reason: 'Sign observed today', status: 'pending', version: 2, created_at: place.updated_at, updated_at: place.updated_at }
const response = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
function Controls() { const session = useSession(), location = useLocation(), navigate = useNavigate(); return <><button onClick={() => void navigate(-1)}>History back</button><button onClick={() => void navigate(1)}>History forward</button><output data-testid="location">{location.pathname + location.search}</output><button onClick={() => session.accept({ user: { ...rider, id: 'rider-b' }, csrf_token: 'b' })}>Switch account</button></> }
function mount(path: string) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
  render(<QueryClientProvider client={client}><MemoryRouter initialEntries={[path]}><I18nProvider><SessionProvider><Routes><Route path="/places" element={<PlacesPage />} /><Route path="/places/new" element={<RequireUser><PlaceSubmissionPage /></RequireUser>} /><Route path="/places/:placeId/suggest" element={<RequireUser><PlaceSubmissionPage /></RequireUser>} /><Route path="/places/submissions" element={<RequireUser><MyPlaceSubmissionsPage /></RequireUser>} /><Route path="/admin" element={<RequireUser><AdminPage /></RequireUser>} /><Route path="/login" element={<p>Sign in test page</p>} /></Routes><Controls /></SessionProvider></I18nProvider></MemoryRouter></QueryClientProvider>)
  return client
}
function ancillary(url: string, user: User | null = rider) {
  if (url.endsWith('/session')) return response({ user, csrf_token: user ? 'a' : undefined })
  if (url.endsWith('/places/config')) return response({ enabled: false, archive_url: '' })
  if (url.endsWith('/geo/config')) return response({ enabled: false, archive_url: '' })
  if (url.endsWith('/cities')) return response({ cities: [] })
  if (url.endsWith('/places/place-a')) return response({ place })
  return undefined
}
beforeEach(() => setCsrfToken())
afterEach(() => { setCsrfToken(); vi.unstubAllGlobals() })

describe('public community map', () => {
  it('browses even when the session service fails, without private requests or geolocation', async () => {
    const gps = vi.fn()
    Object.defineProperty(navigator, 'geolocation', { configurable: true, value: { getCurrentPosition: gps } })
    const fetch = vi.fn(async (url: string) => {
      if (url.endsWith('/session')) return response({ error: { code: 'service_unavailable' } }, 503)
      if (url.includes('/places?')) return response({ places: [place], next_cursor: '' })
      return ancillary(url, null) ?? response({})
    })
    vi.stubGlobal('fetch', fetch); mount('/places')
    await userEvent.click(await screen.findByRole('button', { name: /Road workshop/ }))
    expect(await screen.findByRole('heading', { name: place.name })).toBeInTheDocument()
    expect(screen.getByText('2026-11-01')).toBeInTheDocument()
    expect(screen.getByText(/Directory information is not verified/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Navigate here' })).toHaveAttribute('href', expect.stringContaining('43.2,76.9'))
    expect(fetch.mock.calls.some(([url]) => /\/me\/|\/geo\/|\/admin\//.test(url))).toBe(false)
    expect(gps).not.toHaveBeenCalled()
  })
  it('applies text/city/category and only searches a map viewport on explicit action', async () => {
    const requests: string[] = []
    vi.stubGlobal('fetch', vi.fn(async (url: string) => {
      if (url.endsWith('/places/config')) return response({ enabled: true, archive_url: '/map-data/kz.pmtiles' })
      if (url.includes('/places?')) { requests.push(url); return response({ places: [place], next_cursor: 'next-page' }) }
      return ancillary(url, null) ?? response({})
    }))
    mount('/places')
    await screen.findByText('Only this page is shown on the map. Narrow the area or filters, or open the next page.')
    await userEvent.type(screen.getByRole('textbox', { name: 'Search places' }), 'repair')
    await userEvent.type(screen.getByRole('combobox', { name: 'City' }), 'Almaty')
    await userEvent.click(screen.getByRole('button', { name: 'Workshops' }))
    await userEvent.click(screen.getByRole('button', { name: 'Apply filters' }))
    await waitFor(() => expect(requests.at(-1)).toContain('category=workshop'))
    expect(requests.at(-1)).toContain('q=repair')
    const before = requests.length
    await userEvent.click(await screen.findByRole('button', { name: 'Pan test map' }))
    expect(requests).toHaveLength(before)
    await userEvent.click(screen.getByRole('button', { name: 'Search this area' }))
    await waitFor(() => expect(requests.at(-1)).toContain('west=76'))
    expect(requests.at(-1)).toContain('north=44')
  })
  it('aborts stale list requests and never replaces newer results with a late response', async () => {
    let oldResolve: (value: Response) => void = () => {}, oldSignal: AbortSignal | undefined
    vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
      if (url.includes('/places?')) {
        if (!url.includes('q=latest')) { oldSignal = init?.signal as AbortSignal; return new Promise<Response>((resolve) => { oldResolve = resolve }) }
        return response({ places: [{ ...place, name: 'Latest place' }], next_cursor: '' })
      }
      return ancillary(url, null) ?? response({})
    }))
    mount('/places')
    await waitFor(() => expect(oldSignal).toBeDefined())
    await userEvent.type(screen.getByRole('textbox', { name: 'Search places' }), 'latest')
    await userEvent.click(screen.getByRole('button', { name: 'Apply filters' }))
    await screen.findByRole('button', { name: /Latest place/ })
    expect(oldSignal?.aborted).toBe(true)
    await act(async () => oldResolve(response({ places: [place], next_cursor: '' })))
    expect(screen.queryByRole('button', { name: /Road workshop/ })).not.toBeInTheDocument()
  })
})

describe('place suggestions', () => {
  it('redirects guests before fetching a private suggestion history', async () => {
    const fetch = vi.fn(async (url: string) => ancillary(url, null) ?? response({}))
    vi.stubGlobal('fetch', fetch); mount('/places/submissions')
    await screen.findByText('Sign in test page')
    expect(fetch.mock.calls.some(([url]) => url.includes('/me/place-submissions'))).toBe(false)
  })
  it('submits manual coordinates without map services and confirms moderation', async () => {
    let payload: Record<string, unknown> | undefined
    vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
      if (url.endsWith('/places/submissions')) { payload = JSON.parse(String(init?.body)); return response({ submission: proposal }, 201) }
      return ancillary(url) ?? response({})
    }))
    mount('/places/new')
    await userEvent.type(await screen.findByRole('textbox', { name: /Place name/ }), 'New workshop')
    await userEvent.selectOptions(screen.getByRole('combobox', { name: /Category/ }), 'workshop')
    await userEvent.type(screen.getByRole('spinbutton', { name: /Latitude/ }), '43.2')
    await userEvent.type(screen.getByRole('spinbutton', { name: /Longitude/ }), '76.9')
    await userEvent.type(screen.getByRole('textbox', { name: /What should change/ }), 'Visited today')
    await userEvent.click(screen.getByRole('button', { name: 'Send for review' }))
    await screen.findByRole('heading', { name: 'Suggestion sent for review' })
    expect(payload).toMatchObject({ kind: 'new', reason: 'Visited today', place: { name: 'New workshop', category: 'workshop', lat: 43.2, lon: 76.9 } })
    expect((payload?.place as Record<string, unknown>).source).toBeUndefined()
  })
  it('sends a closed report with its base version and no replacement place', async () => {
    let payload: Record<string, unknown> | undefined
    vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
      if (url.endsWith('/places/submissions')) { payload = JSON.parse(String(init?.body)); return response({ submission: proposal }, 201) }
      return ancillary(url) ?? response({})
    }))
    mount('/places/place-a/suggest?kind=closed')
    await userEvent.type(await screen.findByRole('textbox', { name: /What should change/ }), 'Permanently closed')
    await userEvent.click(screen.getByRole('button', { name: 'Send for review' }))
    await screen.findByRole('heading', { name: 'Suggestion sent for review' })
    expect(payload).toEqual({ kind: 'closed', place_id: place.id, base_version: 3, reason: 'Permanently closed' })
  })
  it('clears private cached suggestion history on account change', async () => {
    let account = 'a'
    vi.stubGlobal('fetch', vi.fn(async (url: string) => {
      if (url.includes('/me/place-submissions')) return response({ submissions: account === 'a' ? [{ ...proposal, reason: 'Private first account note' }] : [], next_cursor: '' })
      return ancillary(url) ?? response({})
    }))
    const client = mount('/places/submissions')
    await screen.findByText('Private first account note')
    account = 'b'
    await userEvent.click(screen.getByRole('button', { name: 'Switch account' }))
    await screen.findByText('You have no suggestions yet.')
    expect(screen.queryByText('Private first account note')).not.toBeInTheDocument()
    expect(client.getQueryCache().getAll().some((q) => q.meta?.private && q.queryKey.includes('rider-a'))).toBe(false)
  })
})

describe('places moderation', () => {
  it('does not load the queue without administrator access', async () => {
    const fetch = vi.fn(async (url: string) => url.endsWith('/me/trust') ? response({ is_admin: false }) : ancillary(url) ?? response({}))
    vi.stubGlobal('fetch', fetch); mount('/admin')
    await screen.findByText('This page isn’t available to your account.')
    expect(fetch.mock.calls.some(([url]) => url.includes('/admin/place-submissions'))).toBe(false)
  })
  it('reviews the explicit pending version and reason', async () => {
    let review: Record<string, unknown> | undefined
    vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
      if (url.endsWith('/me/trust')) return response({ is_admin: true })
      if (url.includes('/admin/reports')) return response({ reports: [], next_cursor: '' })
      if (url.includes('/admin/place-submissions?')) return response({ submissions: [proposal], next_cursor: '' })
      if (url.endsWith('/review')) { review = JSON.parse(String(init?.body)); return response({ submission: { ...proposal, status: 'approved' } }) }
      return ancillary(url) ?? response({})
    }))
    mount('/admin')
    await userEvent.click(await screen.findByRole('button', { name: 'Map' }))
    await userEvent.click(await screen.findByRole('button', { name: /Updated workshop/ }))
    expect(await screen.findByText('Current published place')).toBeInTheDocument()
    await userEvent.type(screen.getByRole('textbox', { name: /Review reason/ }), 'Checked the submitted evidence')
    await userEvent.click(screen.getByRole('button', { name: 'Approve suggestion' }))
    await waitFor(() => expect(review).toEqual({ version: 2, status: 'approved', reason: 'Checked the submitted evidence' }))
  })
})

describe('stale moderation and private request lifecycle', () => {
  it('requires a fresh review for a changed place and permits rejecting the outdated proposal', async () => {
    let payload: Record<string, unknown> | undefined
    vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
      if (url.endsWith('/me/trust')) return response({ is_admin: true })
      if (url.includes('/admin/reports')) return response({ reports: [], next_cursor: '' })
      if (url.includes('/admin/place-submissions?')) return response({ submissions: [proposal], next_cursor: '' })
      if (url.endsWith('/places/place-a')) return response({ place: { ...place, version: 4 } })
      if (url.endsWith('/review')) { payload = JSON.parse(String(init?.body)); return response({ submission: { ...proposal, status: 'rejected' } }) }
      return ancillary(url) ?? response({})
    }))
    mount('/admin')
    await userEvent.click(await screen.findByRole('button', { name: 'Map' }))
    await userEvent.click(await screen.findByRole('button', { name: /Updated workshop/ }))
    await screen.findByText('This place or suggestion changed. Load the latest version and review your changes again before sending.')
    expect(screen.getByRole('button', { name: 'Approve suggestion' })).toBeDisabled()
    await userEvent.type(screen.getByRole('textbox', { name: /Review reason/ }), 'Please submit against the updated place')
    await userEvent.click(screen.getByRole('button', { name: 'Reject suggestion' }))
    await waitFor(() => expect(payload).toMatchObject({ version: 2, status: 'rejected' }))
  })
  it('aborts a suggestion on account change and suppresses its late confirmation', async () => {
    let finish: (value: Response) => void = () => {}, signal: AbortSignal | undefined
    vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
      if (url.endsWith('/places/submissions')) { signal = init?.signal as AbortSignal; return new Promise<Response>((resolve) => { finish = resolve }) }
      return ancillary(url) ?? response({})
    }))
    mount('/places/place-a/suggest?kind=closed')
    await userEvent.type(await screen.findByRole('textbox', { name: /What should change/ }), 'Closed')
    await userEvent.click(screen.getByRole('button', { name: 'Send for review' }))
    await waitFor(() => expect(signal).toBeDefined())
    await userEvent.click(screen.getByRole('button', { name: 'Switch account' }))
    expect(signal?.aborted).toBe(true)
    await act(async () => finish(response({ submission: proposal }, 201)))
    expect(screen.queryByRole('heading', { name: 'Suggestion sent for review' })).not.toBeInTheDocument()
    expect(screen.getByRole('textbox', { name: /What should change/ })).toHaveValue('')
  })
})

describe('suggestion rebase and translated directory', () => {
  it('reloads and manually rebases a stale correction before retrying with the new base version', async () => {
    let version = 3
    const payloads: Record<string, unknown>[] = []
    vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
      if (url.endsWith('/places/place-a')) return response({ place: { ...place, version } })
      if (url.endsWith('/places/submissions')) {
        payloads.push(JSON.parse(String(init?.body)))
        if (payloads.length === 1) { version = 4; return response({ error: { code: 'stale_version' } }, 409) }
        return response({ submission: proposal }, 201)
      }
      return ancillary(url) ?? response({})
    }))
    mount('/places/place-a/suggest?kind=correction')
    await userEvent.type(await screen.findByRole('textbox', { name: /What should change/ }), 'Observed on Monday')
    await userEvent.click(screen.getByRole('button', { name: 'Send for review' }))
    await screen.findByText('This place or suggestion changed. Load the latest version and review your changes again before sending.')
    expect(screen.getByRole('button', { name: 'Send for review' })).toBeDisabled()
    await userEvent.click(screen.getByRole('button', { name: 'Load latest version' }))
    await waitFor(() => expect(screen.getByRole('textbox', { name: /What should change/ })).toHaveValue(''))
    await userEvent.type(screen.getByRole('textbox', { name: /What should change/ }), 'Checked the latest information')
    await userEvent.click(screen.getByRole('button', { name: 'Send for review' }))
    await screen.findByRole('heading', { name: 'Suggestion sent for review' })
    expect(payloads.map((p) => p.base_version)).toEqual([3, 4])
    expect(payloads[1].reason).toBe('Checked the latest information')
  })
  it.each([['ru', 'Карта', 'Заправки'], ['kk', 'Карта', 'Жанармай бекеттері']])('renders the %s public directory copy', async (locale, title, category) => {
    localStorage.setItem('bikerskz.locale', locale)
    vi.stubGlobal('fetch', vi.fn(async (url: string) => url.includes('/places?') ? response({ places: [], next_cursor: '' }) : ancillary(url, null) ?? response({})))
    mount('/places')
    expect(await screen.findByRole('heading', { name: title })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: category })).toBeInTheDocument()
  })
})

describe('directory history and latest public state', () => {
  it('restores scoped cursor predecessors and bounded filters through browser history', async () => {
    vi.stubGlobal('fetch', vi.fn(async (url: string) => {
      if (url.includes('/places?')) {
        const cursor = new URL(url, 'https://bikers.kz').searchParams.get('after')
        return response({ places: [{ ...place, name: cursor === 'c3' ? 'Third page' : cursor === 'c2' ? 'Second page' : 'First page' }], next_cursor: cursor === 'c3' ? '' : cursor === 'c2' ? 'c3' : 'c2' })
      }
      return ancillary(url, null) ?? response({})
    }))
    mount('/places?q=repair&west=76&south=43&east=77&north=44')
    await screen.findByRole('button', { name: /First page/ })
    await userEvent.click(screen.getByRole('button', { name: 'Next page' }))
    await screen.findByRole('button', { name: /Second page/ })
    await userEvent.click(screen.getByRole('button', { name: 'Next page' }))
    await screen.findByRole('button', { name: /Third page/ })
    await userEvent.click(screen.getByRole('button', { name: 'History back' }))
    await screen.findByRole('button', { name: /Second page/ })
    await userEvent.click(screen.getByRole('button', { name: 'Previous page' }))
    await screen.findByRole('button', { name: /First page/ })
    expect(screen.getByTestId('location')).not.toHaveTextContent('after=')
    await userEvent.click(screen.getByRole('button', { name: 'Clear filters' }))
    expect(screen.getByRole('textbox', { name: 'Search places' })).toHaveValue('')
    await userEvent.click(screen.getByRole('button', { name: 'History back' }))
    expect(screen.getByRole('textbox', { name: 'Search places' })).toHaveValue('repair')
    expect(screen.getByTestId('location')).toHaveTextContent('west=76')
    expect(screen.getByText('Results are limited to the selected map area.')).toBeInTheDocument()
  })
  it('hides cached place details after an authoritative not-found response', async () => {
    let finish: (value: Response) => void = () => {}
    vi.stubGlobal('fetch', vi.fn(async (url: string) => {
      if (url.includes('/places?')) return response({ places: [], next_cursor: '' })
      if (url.endsWith('/places/place-a')) return new Promise<Response>((resolve) => { finish = resolve })
      return ancillary(url, null) ?? response({})
    }))
    const client = mount('/places?place=place-a')
    await waitFor(() => expect(client.getQueryState(['places', 'detail', 'place-a'])?.fetchStatus).toBe('fetching'))
    await act(async () => { client.setQueryData(['places', 'detail', 'place-a'], { place }) })
    await screen.findByRole('heading', { name: place.name })
    await act(async () => finish(response({ error: { code: 'place_not_found' } }, 404)))
    await waitFor(() => expect(screen.queryByRole('heading', { name: place.name })).not.toBeInTheDocument())
    expect(screen.queryByRole('link', { name: 'Navigate here' })).not.toBeInTheDocument()
  })
  it('preserves a correction draft and its original base version when public cache updates', async () => {
    let payload: Record<string, unknown> | undefined
    vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
      if (url.endsWith('/places/submissions')) { payload = JSON.parse(String(init?.body)); return response({ submission: proposal }, 201) }
      return ancillary(url) ?? response({})
    }))
    const client = mount('/places/place-a/suggest?kind=correction')
    await userEvent.type(await screen.findByRole('textbox', { name: /What should change/ }), 'Keep this draft')
    await act(async () => { client.setQueryData(['places', 'detail', 'place-a'], { place: { ...place, version: 4, name: 'Changed elsewhere' } }) })
    expect(screen.getByRole('textbox', { name: /What should change/ })).toHaveValue('Keep this draft')
    expect(screen.getByRole('textbox', { name: /Place name/ })).toHaveValue(place.name)
    await userEvent.click(screen.getByRole('button', { name: 'Send for review' }))
    await screen.findByRole('heading', { name: 'Suggestion sent for review' })
    expect(payload).toMatchObject({ base_version: 3, reason: 'Keep this draft' })
  })
})
