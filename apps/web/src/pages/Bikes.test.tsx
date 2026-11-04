import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, Route, Routes, useLocation, useNavigate } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { setCsrfToken } from '../api'
import { App } from '../App'
import { I18nProvider } from '../i18n'
import { SessionProvider, useSession } from '../session'
import type { Bike, PublicBike, User } from '../types'
import { BikesPage } from './Bikes'
import { BikeFormPage, BikeOwnerPage, PublicBikePage } from './Garage'

vi.mock('virtual:pwa-register/react', () => ({ useRegisterSW: () => ({ needRefresh: [false, vi.fn()], updateServiceWorker: vi.fn() }) }))

const rider: User = { id: 'rider-a', profile: { display_name: 'Rider A', city: 'Almaty', languages: ['en'], interests: '', locale: 'en' }, onboarding_complete: true }
const nextRider: User = { ...rider, id: 'rider-b' }
const bike = (n: number, model: string): PublicBike => ({ id: `00000000-0000-4000-8000-${String(n).padStart(12, '0')}`, make: 'Honda', model, model_id: '', year: 2024 })
const response = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json' } })
const parsed = (url: string) => new URL(url, 'https://bikers.kz')
const isList = (url: string) => parsed(url).pathname === '/api/v1/bikes'
function ancillary(url: string) {
  if (url.endsWith('/photos')) return response({ photos: [], cover_photo_id: '' })
  return undefined
}
function Controls() {
  const location = useLocation(), navigate = useNavigate(), session = useSession()
  return <><output data-testid="location">{location.pathname + location.search}</output><button onClick={() => void navigate(-1)}>History back</button><button onClick={() => void navigate(1)}>History forward</button><button onClick={() => session.accept({ user: nextRider, csrf_token: 'csrf-b' })}>Switch account</button></>
}
function mount(path: string, app = false) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
  render(<QueryClientProvider client={client}><MemoryRouter initialEntries={[path]}><I18nProvider><SessionProvider>{app ? <App /> : <Routes><Route path="/bikes" element={<BikesPage />} /><Route path="/bikes/:bikeId" element={<PublicBikePage />} /><Route path="/garage/:bikeId" element={<BikeOwnerPage />} /><Route path="/garage/:bikeId/edit" element={<BikeFormPage edit />} /><Route path="/garage" element={<p>Owner garage test page</p>} /></Routes>}<Controls /></SessionProvider></I18nProvider></MemoryRouter></QueryClientProvider>)
  return client
}
beforeEach(() => setCsrfToken())
afterEach(() => { setCsrfToken(); vi.unstubAllGlobals() })

describe('public motorcycle catalogue', () => {
  it.each([null, rider])('stores only the public projection for viewer %j', async (user) => {
    const publicBike = bike(1, 'CB500X')
    const fetch = vi.fn(async (url: string) => {
      if (url.endsWith('/session')) return response({ user, csrf_token: user ? 'csrf-a' : undefined })
      if (isList(url)) return response({ bikes: [{ ...publicBike, owner_id: 'private-owner', version: 3, maintenance: 'Secret service log' }], next_cursor: '' })
      const extra = ancillary(url); if (extra) return extra
      throw new Error(`Unexpected request ${url}`)
    })
    vi.stubGlobal('fetch', fetch)
    const client = mount('/bikes')
    await screen.findByRole('heading', { name: 'CB500X' })
    const cached = client.getQueryData<{ bikes: Record<string, unknown>[] }>(['public-bikes', user?.id ?? 'guest', '', ''])!
    expect(Object.keys(cached.bikes[0]).sort()).toEqual(['id', 'make', 'model', 'model_id', 'year'])
    expect(screen.queryByText(/private-owner|Secret service log/)).not.toBeInTheDocument()
    expect(fetch.mock.calls.some(([url]) => /\/me\/|\/maintenance/.test(url))).toBe(false)
    expect(within(screen.getByRole('article')).getByRole('link', { name: 'View motorcycle' })).toHaveAttribute('href', `/bikes/${publicBike.id}`)
  })

  it('waits for session recovery before requesting the public catalogue', async () => {
    let failing = true
    const fetch = vi.fn(async (url: string) => {
      if (url.endsWith('/session')) return failing ? response({ error: { code: 'service_unavailable' } }, 503) : response({ user: null })
      if (isList(url)) return response({ bikes: [], next_cursor: '' })
      throw new Error(`Unexpected request ${url}`)
    })
    vi.stubGlobal('fetch', fetch)
    mount('/bikes')
    await screen.findByText('We couldn’t reach BIKERS.KZ.')
    expect(fetch.mock.calls.every(([url]) => url.endsWith('/session'))).toBe(true)
    failing = false
    await userEvent.click(screen.getByRole('button', { name: 'Try again' }))
    await screen.findByRole('heading', { name: 'The public garage is waiting.' })
  })

  it('searches on the server, clears paging and restores URL filters through history', async () => {
    const fetch = vi.fn(async (url: string) => {
      if (url.endsWith('/session')) return response({ user: null })
      if (isList(url)) return response({ bikes: [bike(1, parsed(url).searchParams.get('q') || 'All makes')], next_cursor: '' })
      const extra = ancillary(url); if (extra) return extra
      throw new Error(`Unexpected request ${url}`)
    })
    vi.stubGlobal('fetch', fetch)
    mount('/bikes?q=Honda&cursor=old-page')
    await screen.findByRole('heading', { name: 'Honda' })
    expect(screen.getByLabelText('Search motorcycles')).toHaveValue('Honda')
    fireEvent.change(screen.getByLabelText('Search motorcycles'), { target: { value: '  Yamaha MT  ' } })
    await userEvent.click(screen.getByRole('button', { name: 'Apply filters' }))
    await screen.findByRole('heading', { name: 'Yamaha MT' })
    expect(Object.fromEntries(parsed(fetch.mock.calls.filter(([url]) => isList(url)).at(-1)![0]).searchParams)).toEqual({ q: 'Yamaha MT' })
    await userEvent.click(screen.getByRole('button', { name: 'History back' }))
    await screen.findByRole('heading', { name: 'Honda' })
    expect(screen.getByLabelText('Search motorcycles')).toHaveValue('Honda')
    await userEvent.click(screen.getByRole('button', { name: 'History forward' }))
    await screen.findByRole('heading', { name: 'Yamaha MT' })
    await userEvent.click(screen.getByRole('button', { name: 'Clear filters' }))
    await screen.findByRole('heading', { name: 'All makes' })
    expect(screen.getByLabelText('Search motorcycles')).toHaveValue('')
    expect(screen.getByTestId('location')).toHaveTextContent('/bikes')
  })

  it('pages with server cursors and restores the earlier page', async () => {
    const cursor = bike(1, '').id
    vi.stubGlobal('fetch', vi.fn(async (url: string) => {
      if (url.endsWith('/session')) return response({ user: null })
      if (isList(url)) return parsed(url).searchParams.has('cursor') ? response({ bikes: [bike(2, 'Second page')], next_cursor: '' }) : response({ bikes: [bike(1, 'First page')], next_cursor: cursor })
      const extra = ancillary(url); if (extra) return extra
      throw new Error(`Unexpected request ${url}`)
    }))
    mount('/bikes?q=Honda')
    await screen.findByRole('heading', { name: 'First page' })
    await userEvent.click(screen.getByRole('button', { name: 'Next page' }))
    await screen.findByRole('heading', { name: 'Second page' })
    expect(parsed(screen.getByTestId('location').textContent!).searchParams.get('cursor')).toBe(cursor)
    expect(screen.queryByRole('heading', { name: 'First page' })).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Previous page' }))
    await screen.findByRole('heading', { name: 'First page' })
    expect(parsed(screen.getByTestId('location').textContent!).searchParams.has('cursor')).toBe(false)
  })

  it('resets actor-bound paging and aborts a previous account request', async () => {
    let current = rider
    const reads: { actor: string; cursor: string | null }[] = []
    const oldRequest: { signal?: AbortSignal | null } = {}
    let finishOld: (value: Response) => void = () => { throw new Error('No pending read') }
    const oldPage = new Promise<Response>((resolve) => { finishOld = resolve })
    vi.stubGlobal('fetch', vi.fn(async (url: string, options?: RequestInit) => {
      if (url.endsWith('/session')) return response({ user: current, csrf_token: current.id })
      if (isList(url)) {
        const cursor = parsed(url).searchParams.get('cursor'); reads.push({ actor: current.id, cursor })
        if (current.id === rider.id && cursor) { oldRequest.signal = options?.signal; return oldPage }
        return response({ bikes: [bike(current.id === rider.id ? 1 : 2, current.id === rider.id ? 'Original account page' : 'New account page')], next_cursor: current.id === rider.id ? bike(1, '').id : '' })
      }
      const extra = ancillary(url); if (extra) return extra
      throw new Error(`Unexpected request ${url}`)
    }))
    const client = mount('/bikes')
    await screen.findByRole('heading', { name: 'Original account page' })
    await userEvent.click(screen.getByRole('button', { name: 'Next page' }))
    await waitFor(() => expect(reads.some((read) => !!read.cursor)).toBe(true))
    current = nextRider
    await userEvent.click(screen.getByRole('button', { name: 'Switch account' }))
    await screen.findByRole('heading', { name: 'New account page' })
    expect(oldRequest.signal?.aborted).toBe(true)
    await act(async () => { finishOld(response({ bikes: [bike(3, 'Late account page')], next_cursor: '' })); await oldPage })
    expect(screen.queryByRole('heading', { name: 'Late account page' })).not.toBeInTheDocument()
    expect(reads.filter((read) => read.actor === nextRider.id)).toEqual([{ actor: nextRider.id, cursor: null }])
    expect(client.getQueriesData({ queryKey: ['public-bikes', rider.id] })).toEqual([])
  })

  it('recovers an unavailable cursor and distinguishes empty search results', async () => {
    vi.stubGlobal('fetch', vi.fn(async (url: string) => {
      if (url.endsWith('/session')) return response({ user: null })
      if (isList(url)) return parsed(url).searchParams.has('cursor') ? response({ error: { code: 'invalid_cursor' } }, 422) : response({ bikes: [], next_cursor: '' })
      throw new Error(`Unexpected request ${url}`)
    }))
    mount('/bikes?q=NoMatch&cursor=expired')
    await userEvent.click(await screen.findByRole('button', { name: 'Return to the first motorcycle page' }))
    await screen.findByText('No public motorcycles match this search. Try another make or model, or clear the filters.')
    expect(screen.getByRole('link', { name: 'Clear filters' })).toHaveAttribute('href', '/bikes')
    expect(parsed(screen.getByTestId('location').textContent!).searchParams.has('cursor')).toBe(false)
  })

  it('links the guest catalogue from Explore and public details back to the catalogue', async () => {
    const selected = bike(1, 'CB500X')
    vi.stubGlobal('fetch', vi.fn(async (url: string) => {
      if (url.endsWith('/session')) return response({ user: null })
      if (isList(url)) return response({ bikes: [selected], next_cursor: '' })
      if (url.endsWith(`/bikes/${selected.id}`)) return response({ bike: selected })
      const extra = ancillary(url); if (extra) return extra
      throw new Error(`Unexpected request ${url}`)
    }))
    mount('/bikes', true)
    const explore = await screen.findByRole('link', { name: 'Motorcycles' })
    expect(explore).toHaveAttribute('aria-current', 'page')
    await userEvent.click(await screen.findByRole('link', { name: 'View motorcycle' }))
    const back = await screen.findByRole('link', { name: 'Back to motorcycles' })
    expect(back).toHaveAttribute('href', '/bikes')
    await userEvent.click(back)
    await screen.findByRole('heading', { name: 'Motorcycles in the community.' })
  })

  it.each([
    ['ru', 'Мотоциклы', 'Мотоциклы сообщества.', 'Поиск мотоциклов'],
    ['kk', 'Мотоциклдер', 'Қауымдастық мотоциклдері.', 'Мотоциклдерді іздеу'],
  ])('localizes the public navigation and catalogue in %s', async (locale, navigation, title, search) => {
    localStorage.setItem('bikerskz.locale', locale)
    vi.stubGlobal('fetch', vi.fn(async (url: string) => {
      if (url.endsWith('/session')) return response({ user: null })
      if (isList(url)) return response({ bikes: [], next_cursor: '' })
      throw new Error(`Unexpected request ${url}`)
    }))
    mount('/bikes', true)
    await screen.findByRole('heading', { name: title })
    expect(screen.getByRole('link', { name: navigation })).toHaveAttribute('href', '/bikes')
    expect(screen.getByRole('searchbox', { name: search })).toBeInTheDocument()
  })

  it.each(['publish', 'hide', 'edit', 'delete'] as const)('invalidates a fresh catalogue after an owner chooses to %s a bike', async (operation) => {
    let owned: Bike = { ...bike(1, 'CB500X'), owner_id: rider.id, visibility: operation === 'publish' ? 'private' : 'public', version: 1, can_manage: true, created_at: '2026-11-03T00:00:00Z', updated_at: '2026-11-03T00:00:00Z' }
    vi.stubGlobal('fetch', vi.fn(async (url: string, options?: RequestInit) => {
      if (url.endsWith('/session')) return response({ user: rider, csrf_token: 'csrf-a' })
      if (url.endsWith(`/me/bikes/${owned.id}`) && options?.method === 'DELETE') return response({ ok: true })
      if (url.endsWith(`/me/bikes/${owned.id}`) && options?.method === 'PATCH') { owned = { ...owned, ...JSON.parse(String(options.body)), version: 2 }; return response({ bike: owned }) }
      if (url.endsWith(`/bikes/${owned.id}`)) return response({ bike: owned })
      if (url.includes('/maintenance')) return response({ entries: [], next_cursor: '' })
      const extra = ancillary(url); if (extra) return extra
      throw new Error(`Unexpected request ${url}`)
    }))
    const client = mount(`/garage/${owned.id}${operation === 'edit' ? '/edit' : ''}`)
    await screen.findByRole('heading', { name: operation === 'edit' ? 'Your motorcycle details.' : 'CB500X' })
    const key = ['public-bikes', rider.id, '', '']
    client.setQueryDefaults(['public-bikes'], { staleTime: Infinity, gcTime: Infinity })
    client.setQueryData(key, { bikes: [bike(1, 'CB500X')], next_cursor: '' })
    expect(client.getQueryState(key)?.isInvalidated).toBe(false)
    if (operation === 'edit') {
      fireEvent.change(screen.getByLabelText(/^Model \*/), { target: { value: 'CB650R' } })
      await userEvent.click(screen.getByRole('button', { name: 'Save motorcycle' }))
    } else {
      const action = operation === 'publish' ? 'Publish basic page' : operation === 'hide' ? 'Make basic page private' : 'Delete motorcycle'
      await userEvent.click(screen.getByRole('button', { name: action }))
      const dialog = await screen.findByRole('dialog', { name: action })
      if (operation === 'publish' || operation === 'hide') expect(dialog).toHaveTextContent('public catalogue')
      await userEvent.click(within(dialog).getByRole('button', { name: action }))
    }
    await waitFor(() => expect(client.getQueryState(key)?.isInvalidated).toBe(true))
  })
})
