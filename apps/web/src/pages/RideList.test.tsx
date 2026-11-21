import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, useLocation, useNavigate } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { App } from '../App'
import { setCsrfToken } from '../api'
import { I18nProvider } from '../i18n'
import { SessionProvider, useSession } from '../session'
import { cityPhoto, cityPhotos } from '../cityPhotos'
import type { Ride, User } from '../types'

vi.mock('virtual:pwa-register/react', () => ({ useRegisterSW: () => ({ needRefresh: [false, vi.fn()], updateServiceWorker: vi.fn() }) }))

const rider: User = { id: 'rider-a', profile: { display_name: 'Rider A', city: 'Almaty', languages: ['en'], interests: '', locale: 'en' }, onboarding_complete: true }
const nextRider: User = { ...rider, id: 'rider-b', profile: { ...rider.profile, display_name: 'Rider B' } }
const city = { value: 'Almaty', names: { en: 'Almaty', ru: 'Алматы', kk: 'Алматы' }, aliases: ['Алматы'] }
const summary = (id: string, description: string): Ride => ({ id, city: 'Almaty', date: '2026-12-01', description, status: 'published' })
const response = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json' } })
const parsed = (url: string) => new URL(url, 'https://bikers.kz')
const isRideList = (url: string) => parsed(url).pathname === '/api/v1/rides'

function ancillaryRead(url: string) {
  if (url.endsWith('/cities')) return response({ cities: [city, { value: 'Astana', names: { en: 'Astana', ru: 'Астана', kk: 'Астана' }, aliases: ['Астана'] }] })
  if (url.endsWith('/me/trust')) return response({ is_admin: false })
  if (url.includes('/avatar')) return response({ image: null, version: 1 })
  return undefined
}

function TestControls() {
  const location = useLocation()
  const navigate = useNavigate()
  const session = useSession()
  return <><output data-testid="location">{location.pathname + location.search}</output><button onClick={() => session.accept({ user: nextRider, csrf_token: 'csrf-b' })}>Switch test rider</button><button onClick={() => void navigate(-1)}>Go back</button></>
}

function mount(path = '/') {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } } })
  render(<QueryClientProvider client={client}><MemoryRouter initialEntries={[path]}><I18nProvider><SessionProvider><App /><TestControls /></SessionProvider></I18nProvider></MemoryRouter></QueryClientProvider>)
  return client
}

beforeEach(() => setCsrfToken())
afterEach(() => { setCsrfToken(); vi.unstubAllGlobals(); vi.useRealTimers() })
// Date labels leave out the current year, so tests that read them fix the date (timers stay real).
const fixToday = (iso: string) => { vi.useFakeTimers({ toFake: ['Date'] }); vi.setSystemTime(new Date(iso)) }

describe('ride discovery filters and personal views', () => {
  it('rejects a reversed date range without requesting it and accepts the corrected range', async () => {
    const fetch = vi.fn(async (url: string) => {
      if (url.endsWith('/session')) return response({ user: rider, csrf_token: 'csrf-a' })
      if (isRideList(url)) return response({ rides: [summary('discovery', 'A discovery ride')], next_cursor: '' })
      const extra = ancillaryRead(url); if (extra) return extra
      throw new Error(`Unexpected request ${url}`)
    })
    vi.stubGlobal('fetch', fetch)
    mount()
    await screen.findByText('A discovery ride')
    const initialReads = fetch.mock.calls.filter(([url]) => isRideList(url)).length
    const dateFilters = screen.getByLabelText('From date').closest('details')!
    expect(dateFilters).not.toHaveAttribute('open')
    // The control's name is its visible label, so voice commands that say what is shown reach it.
    const dateToggle = dateFilters.querySelector('summary')!
    expect(dateToggle).toHaveAccessibleName('Any date')
    await userEvent.click(dateToggle)
    expect(dateFilters).toHaveAttribute('open')
    fireEvent.change(screen.getByLabelText('From date'), { target: { value: '2027-01-31' } })
    fireEvent.change(screen.getByLabelText('To date'), { target: { value: '2027-01-01' } })
    await userEvent.click(screen.getByRole('button', { name: 'Apply filters' }))
    expect(fetch.mock.calls.filter(([url]) => isRideList(url))).toHaveLength(initialReads)
    // Programmatic submission also enforces the range when native validation is bypassed.
    fireEvent.submit(screen.getByRole('form', { name: 'Ride filters' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('The end date must be on or after the start date.')
    expect(fetch.mock.calls.filter(([url]) => isRideList(url))).toHaveLength(initialReads)
    fireEvent.change(screen.getByLabelText('To date'), { target: { value: '2027-02-01' } })
    await userEvent.click(screen.getByRole('button', { name: 'Apply filters' }))
    await waitFor(() => expect(fetch.mock.calls.some(([url]) => isRideList(url) && parsed(url).searchParams.get('from') === '2027-01-31' && parsed(url).searchParams.get('to') === '2027-02-01')).toBe(true))
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('restores city and inclusive date filters from the URL and applies them on the server', async () => {
    const fetch = vi.fn(async (url: string) => {
      if (url.endsWith('/session')) return response({ user: rider, csrf_token: 'csrf-a' })
      if (isRideList(url)) return response({ rides: [summary(parsed(url).searchParams.get('city') ?? 'all', 'A filtered ride')], next_cursor: '' })
      const extra = ancillaryRead(url); if (extra) return extra
      throw new Error(`Unexpected request ${url}`)
    })
    vi.stubGlobal('fetch', fetch)
    fixToday('2026-11-19T06:00:00Z')
    mount('/?q=touring&city=Almaty&from=2026-12-01&to=2026-12-31&cursor=original-page-two')
    await screen.findByText('A filtered ride')
    expect(screen.getByLabelText('City')).toHaveValue('Almaty')
    expect(screen.getByLabelText('From date').closest('details')).toHaveAttribute('open')
    // A short range keeps the summary beside the city field on a phone; its name is the text it shows.
    expect(screen.getByLabelText('From date').closest('details')!.querySelector('summary')).toHaveAccessibleName('1–31 Dec')
    expect(screen.getByLabelText('From date')).toHaveValue('2026-12-01')
    expect(screen.getByLabelText('To date')).toHaveValue('2026-12-31')
    expect(screen.getByLabelText('Search public descriptions')).toHaveValue('touring')
    // The placeholder is the only visible label, so the accessible name says the same.
    expect(screen.getByRole('searchbox', { name: 'Search public descriptions' })).toHaveAttribute('placeholder', 'Search public descriptions')
    expect(screen.getByRole('combobox', { name: 'City' })).toHaveAttribute('placeholder', 'City')
    const initial = parsed(fetch.mock.calls.find(([url]) => isRideList(url))![0]).searchParams
    expect(initial.get('city')).toBe('Almaty')
    expect(initial.get('from')).toBe('2026-12-01')
    expect(initial.get('to')).toBe('2026-12-31')
    expect(initial.get('q')).toBe('touring')
    expect(initial.get('cursor')).toBe('original-page-two')

    fireEvent.change(screen.getByLabelText('Search public descriptions'), { target: { value: 'city loop' } })
    fireEvent.change(screen.getByLabelText('City'), { target: { value: 'Astana' } })
    fireEvent.change(screen.getByLabelText('From date'), { target: { value: '2027-01-01' } })
    fireEvent.change(screen.getByLabelText('To date'), { target: { value: '2027-01-31' } })
    await userEvent.click(screen.getByRole('button', { name: 'Apply filters' }))
    await waitFor(() => expect(fetch.mock.calls.some(([url]) => isRideList(url) && parsed(url).searchParams.get('city') === 'Astana')).toBe(true))
    const latest = parsed(fetch.mock.calls.filter(([url]) => isRideList(url)).at(-1)![0]).searchParams
    expect(Object.fromEntries(latest)).toMatchObject({ q: 'city loop', city: 'Astana', from: '2027-01-01', to: '2027-01-31' })
    expect(latest.has('cursor')).toBe(false)
    const route = parsed(screen.getByTestId('location').textContent!)
    expect(route.searchParams.get('city')).toBe('Astana')
    expect(route.searchParams.get('from')).toBe('2027-01-01')

    await userEvent.click(screen.getByRole('button', { name: 'Clear filters' }))
    await waitFor(() => expect(screen.getByLabelText('City')).toHaveValue(''))
    expect(screen.getByLabelText('From date')).toHaveValue('')
    expect(screen.getByLabelText('To date')).toHaveValue('')
    expect(screen.getByLabelText('Search public descriptions')).toHaveValue('')
    expect(parsed(screen.getByTestId('location').textContent!).searchParams.has('city')).toBe(false)
  })

  it.each([['ru', '1–30 июн.'], ['kk', '1–30 мау']])('names the date summary by its short range in %s', async (locale, label) => {
    localStorage.setItem('bikerskz.locale', locale)
    fixToday('2026-06-10T04:00:00Z')
    vi.stubGlobal('fetch', vi.fn(async (url: string) => {
      if (url.endsWith('/session')) return response({ user: rider, csrf_token: 'csrf-a' })
      if (isRideList(url)) return response({ rides: [summary('ranged', 'A ranged ride')], next_cursor: '' })
      const extra = ancillaryRead(url); if (extra) return extra
      throw new Error(`Unexpected request ${url}`)
    }))
    mount('/?from=2026-06-01&to=2026-06-30')
    await screen.findByText('A ranged ride')
    expect(screen.getByRole('form').querySelector('details > summary')).toHaveAccessibleName(label)
  })

  it('keeps keyboard focus on Apply, follows history, and moves focus to the search field after Clear', async () => {
    const fetch = vi.fn(async (url: string) => {
      if (url.endsWith('/session')) return response({ user: rider, csrf_token: 'csrf-a' })
      if (isRideList(url)) return response({ rides: [summary('discovery', 'A discovery ride')], next_cursor: '' })
      const extra = ancillaryRead(url); if (extra) return extra
      throw new Error(`Unexpected request ${url}`)
    })
    vi.stubGlobal('fetch', fetch)
    mount()
    await screen.findByText('A discovery ride')
    const actor = userEvent.setup()
    const search = screen.getByLabelText('Search public descriptions')
    await actor.type(search, 'loop')
    const apply = screen.getByRole('button', { name: 'Apply filters' })
    await actor.click(apply)
    await waitFor(() => expect(parsed(screen.getByTestId('location').textContent!).searchParams.get('q')).toBe('loop'))
    expect(screen.getByRole('button', { name: 'Apply filters' })).toBe(apply)
    expect(apply).toHaveFocus()
    expect(search).toHaveValue('loop')

    await actor.click(screen.getByRole('button', { name: 'Go back' }))
    await waitFor(() => expect(search).toHaveValue(''))
    expect(parsed(screen.getByTestId('location').textContent!).searchParams.has('q')).toBe(false)

    await actor.type(search, 'chrome')
    await actor.keyboard('{Enter}')
    await waitFor(() => expect(parsed(screen.getByTestId('location').textContent!).searchParams.get('q')).toBe('chrome'))
    expect(search).toHaveFocus()
    await actor.click(screen.getByRole('button', { name: 'Clear filters' }))
    await waitFor(() => expect(parsed(screen.getByTestId('location').textContent!).searchParams.has('q')).toBe(false))
    expect(screen.queryByRole('button', { name: 'Clear filters' })).not.toBeInTheDocument()
    expect(search).toHaveFocus()
    expect(search).toHaveValue('')
  })

  it('shows cover photos on cards, decorative and lazy except on the featured card, then the city photo, then the city art', async () => {
    const id = (n: number) => `20000000-0000-4000-8000-00000000000${n}`
    const cover = (ride: string, n: number) => ({ id: `75000000-0000-4000-8000-00000000000${n}`, url: `/api/v1/rides/${ride}/cover/75000000-0000-4000-8000-00000000000${n}/content`, width: 1600, height: 900, created_at: '2026-11-19T09:00:00Z' })
    const rides: Ride[] = [
      { ...summary(id(1), 'Featured with cover'), cover: cover(id(1), 1) },
      { ...summary(id(2), 'Second with cover'), cover: cover(id(2), 2) },
      { ...summary(id(3), 'Foreign cover'), cover: { ...cover(id(3), 3), url: 'https://tracker.example/cover.jpg' } },
      summary(id(4), 'No cover'),
      { ...summary(id(5), 'Custom city'), city: 'Kok-Zhailau camp' },
    ]
    const photo = (n: number) => cityPhoto('Almaty', id(n), [city])!
    vi.stubGlobal('fetch', vi.fn(async (url: string) => {
      if (url.endsWith('/session')) return response({ user: null })
      if (isRideList(url)) return response({ rides, next_cursor: '' })
      const extra = ancillaryRead(url); if (extra) return extra
      throw new Error(`Unexpected request ${url}`)
    }))
    mount()
    const card = async (text: string) => (await screen.findByText(text)).closest('a')!
    const featured = (await card('Featured with cover')).querySelector('img.ride-card-cover')
    expect(featured).toHaveAttribute('src', rides[0].cover!.url)
    expect(featured).toHaveAttribute('alt', '')
    expect(featured).toHaveAttribute('width', '1600')
    expect(featured).toHaveAttribute('height', '900')
    expect(featured).not.toHaveAttribute('loading')
    const second = (await card('Second with cover')).querySelector('img.ride-card-cover')
    expect(second).toHaveAttribute('loading', 'lazy')
    expect(second).toHaveAttribute('referrerpolicy', 'no-referrer')
    // A ride without a visible cover shows a photo of its city, as decoratively as a cover.
    expect(cityPhotos.Almaty).toContain(photo(4))
    for (const [text, n] of [['Foreign cover', 3], ['No cover', 4]] as const) {
      const image = (await card(text)).querySelector('img.ride-card-cover')
      expect(image).toHaveAttribute('src', photo(n).url)
      expect(image).toHaveAttribute('alt', '')
      expect(image).toHaveAttribute('width', String(photo(n).width))
      expect(image).toHaveAttribute('height', String(photo(n).height))
      expect(image).toHaveAttribute('loading', 'lazy')
      expect(image).toHaveAttribute('referrerpolicy', 'no-referrer')
    }
    expect((await card('Custom city')).querySelector('img')).toBeNull()
    expect((await card('Custom city')).querySelector('.city-art')).toHaveTextContent('Kok-Zhailau camp')
    expect(screen.queryAllByRole('img')).toHaveLength(0)
    // A cover that fails moves on to the city photo, and a city photo that fails leaves the city art.
    fireEvent.error(second!)
    const fallback = (await card('Second with cover')).querySelector('img.ride-card-cover')
    expect(fallback).toHaveAttribute('src', photo(2).url)
    expect(fallback).toHaveAttribute('alt', '')
    fireEvent.error(fallback!)
    expect((await card('Second with cover')).querySelector('img')).toBeNull()
    expect((await card('Second with cover')).querySelector('.city-art')).toHaveTextContent('Almaty')
  })

  it('says “You’re going” only for approved rides that lie ahead', async () => {
    const approved = (id: string, description: string, date: string, status = 'published'): Ride => ({ ...summary(id, description), date, status, participation_state: 'approved' })
    vi.stubGlobal('fetch', vi.fn(async (url: string) => {
      if (url.endsWith('/session')) return response({ user: rider, csrf_token: 'csrf-a' })
      if (isRideList(url)) return response({ rides: [approved('ahead', 'Ride ahead', '2099-06-01'), approved('past', 'Ride in the past', '2020-06-01'), approved('cancelled', 'Cancelled ride', '2099-06-02', 'cancelled')], next_cursor: '' })
      const extra = ancillaryRead(url); if (extra) return extra
      throw new Error(`Unexpected request ${url}`)
    }))
    mount('/my-rides?view=joined')
    const status = async (text: string) => (await screen.findByText(text)).closest('a')!.querySelector('.ride-card-status')
    expect(await status('Ride ahead')).toHaveTextContent('You’re going')
    expect(await status('Ride in the past')).toHaveTextContent('Approved')
    expect(await status('Cancelled ride')).toHaveTextContent('CancelledApproved')
  })

  it('pages with server cursors and resets the cursor when switching personal views', async () => {
    const fetch = vi.fn(async (url: string) => {
      if (url.endsWith('/session')) return response({ user: rider, csrf_token: 'csrf-a' })
      if (isRideList(url)) {
        const params = parsed(url).searchParams
        const view = params.get('view') ?? 'discover'
        const secondPage = params.get('cursor') === 'page-two'
        return response({ rides: [summary(`${view}-${secondPage ? 'two' : 'one'}`, `${view} page ${secondPage ? 'two' : 'one'}`)], next_cursor: view === 'organizing' && !secondPage ? 'page-two' : '' })
      }
      const extra = ancillaryRead(url); if (extra) return extra
      throw new Error(`Unexpected request ${url}`)
    })
    vi.stubGlobal('fetch', fetch)
    mount('/my-rides?view=organizing&city=Almaty')
    await screen.findByText('organizing page one')
    await userEvent.click(screen.getByRole('button', { name: 'Next page' }))
    await screen.findByText('organizing page two')
    expect(screen.queryByText('organizing page one')).not.toBeInTheDocument()
    expect(parsed(screen.getByTestId('location').textContent!).searchParams.get('cursor')).toBe('page-two')
    await userEvent.click(screen.getByRole('button', { name: 'Previous page' }))
    await screen.findByText('organizing page one')
    await userEvent.click(screen.getByRole('button', { name: 'Next page' }))
    await screen.findByText('organizing page two')

    for (const [label, view] of [['Joined', 'joined'], ['Pending', 'pending'], ['Past', 'past']] as const) {
      await userEvent.click(screen.getByRole('link', { name: label }))
      await screen.findByText(`${view} page one`)
      const params = parsed(screen.getByTestId('location').textContent!).searchParams
      expect(params.get('view')).toBe(view)
      expect(params.get('city')).toBe('Almaty')
      expect(params.has('cursor')).toBe(false)
    }
    const personalRequests = fetch.mock.calls.filter(([url]) => isRideList(url)).map(([url]) => parsed(url).searchParams)
    expect(personalRequests.some((params) => params.get('view') === 'organizing' && params.get('cursor') === 'page-two')).toBe(true)
    expect(personalRequests.filter((params) => params.get('view') !== 'organizing').every((params) => !params.has('cursor'))).toBe(true)
  })

  it('requires sign-in before fetching any personal list and preserves the requested view', async () => {
    const fetch = vi.fn(async (url: string) => {
      if (url.endsWith('/session')) return response({ user: null })
      const extra = ancillaryRead(url); if (extra) return extra
      throw new Error(`Unexpected anonymous request ${url}`)
    })
    vi.stubGlobal('fetch', fetch)
    mount('/my-rides?view=pending&city=Astana')
    await screen.findByLabelText(/Phone number/)
    expect(fetch.mock.calls.some(([url]) => isRideList(url))).toBe(false)
    const location = parsed(screen.getByTestId('location').textContent!)
    expect(location.pathname).toBe('/login')
    expect(location.searchParams.get('next')).toBe('/my-rides?view=pending&city=Astana')
  })

  it('waits for session resolution before reading cookie-dependent discovery and strips guest fields', async () => {
    let finishSession: (value: Response) => void = () => { throw new Error('Session not requested') }
    const session = new Promise<Response>((resolve) => { finishSession = resolve })
    const fetch = vi.fn(async (url: string) => {
      if (url.endsWith('/session')) return session
      if (isRideList(url)) return response({ rides: [{ ...summary('public', 'Public ride'), title: 'Private title', meeting_point: 'Private place', participation_state: 'approved', can_manage: true }], next_cursor: '' })
      const extra = ancillaryRead(url); if (extra) return extra
      throw new Error(`Unexpected request ${url}`)
    })
    vi.stubGlobal('fetch', fetch)
    const client = mount()
    await waitFor(() => expect(fetch.mock.calls.some(([url]) => url.endsWith('/session'))).toBe(true))
    expect(fetch.mock.calls.some(([url]) => isRideList(url))).toBe(false)
    await act(async () => { finishSession(response({ user: null })); await session })
    await screen.findByText('Public ride')
    expect(screen.queryByText('Private title')).not.toBeInTheDocument()
    expect(screen.queryByText('Private place')).not.toBeInTheDocument()
    expect(screen.queryByText('Organizer')).not.toBeInTheDocument()
    expect(screen.queryByText('Approved')).not.toBeInTheDocument()
    const listings = client.getQueriesData<{ rides: Ride[] }>({ queryKey: ['rides'] })
    expect(listings.some(([, page]) => page?.rides.some((ride) => ride.id === 'public'))).toBe(true)
    expect(listings.some(([, page]) => page?.rides.some((ride) => ride.meeting_point !== undefined || ride.title !== undefined || ride.can_manage))).toBe(false)
  })

  it('discards an old account listing response after switching riders', async () => {
    let finishOld: (value: Response) => void = () => { throw new Error('Old list not requested') }
    const oldPage = new Promise<Response>((resolve) => { finishOld = resolve })
    let currentActor = rider
    const fetch = vi.fn(async (url: string) => {
      if (url.endsWith('/session')) return response({ user: currentActor, csrf_token: currentActor.id === rider.id ? 'csrf-a' : 'csrf-b' })
      if (isRideList(url)) return currentActor.id === rider.id ? oldPage : response({ rides: [summary('rider-b', 'Next rider list')], next_cursor: '' })
      const extra = ancillaryRead(url); if (extra) return extra
      throw new Error(`Unexpected request ${url}`)
    })
    vi.stubGlobal('fetch', fetch)
    mount('/my-rides?view=joined')
    await waitFor(() => expect(fetch.mock.calls.some(([url]) => isRideList(url))).toBe(true))
    currentActor = nextRider
    await userEvent.click(screen.getByRole('button', { name: 'Switch test rider' }))
    await screen.findByText('Next rider list')
    await act(async () => { finishOld(response({ rides: [summary('rider-a', 'Old rider private list')], next_cursor: '' })); await oldPage })
    expect(screen.queryByText('Old rider private list')).not.toBeInTheDocument()
    expect(screen.getByText('Next rider list')).toBeInTheDocument()
  })

  it('returns to the first page instead of sending another account cursor after switching riders', async () => {
    let currentActor = rider
    const listReads: { actor: string; cursor: string | null }[] = []
    const fetch = vi.fn(async (url: string) => {
      if (url.endsWith('/session')) return response({ user: currentActor, csrf_token: currentActor.id === rider.id ? 'csrf-a' : 'csrf-b' })
      if (isRideList(url)) {
        const cursor = parsed(url).searchParams.get('cursor')
        listReads.push({ actor: currentActor.id, cursor })
        const title = currentActor.id === rider.id ? `Original account ${cursor ? 'second' : 'first'} page` : 'Next account first page'
        return response({ rides: [summary(`${currentActor.id}-${cursor ?? 'first'}`, title)], next_cursor: currentActor.id === rider.id && !cursor ? 'account-a-page-two' : '' })
      }
      const extra = ancillaryRead(url); if (extra) return extra
      throw new Error(`Unexpected request ${url}`)
    })
    vi.stubGlobal('fetch', fetch)
    mount('/my-rides?view=joined')
    await screen.findByText('Original account first page')
    await userEvent.click(screen.getByRole('button', { name: 'Next page' }))
    await screen.findByText('Original account second page')
    currentActor = nextRider
    await userEvent.click(screen.getByRole('button', { name: 'Switch test rider' }))
    await screen.findByText('Next account first page')
    expect(listReads.filter((read) => read.actor === nextRider.id)).toEqual([{ actor: nextRider.id, cursor: null }])
    expect(parsed(screen.getByTestId('location').textContent!).searchParams.has('cursor')).toBe(false)
    expect(screen.queryByText('Original account second page')).not.toBeInTheDocument()
  })
})
