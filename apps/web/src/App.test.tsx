import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MapTrifoldIcon, RoadHorizonIcon, UserCircleIcon, UsersThreeIcon } from '@phosphor-icons/react'
import type { Icon } from '@phosphor-icons/react'
import { renderToStaticMarkup } from 'react-dom/server'
import { MemoryRouter, useLocation, useNavigate } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { App, pageSection } from './App'
import { setCsrfToken } from './api'
import { I18nProvider } from './i18n'
import { SessionProvider } from './session'
import { localRideTime, rideTimeToInstant } from './pages/Rides'
import { mergeMessages } from './pages/RideDetail'
import { dateParts, formatRideDateTime, validNext } from './ui'
import type { Message, User } from './types'

vi.mock('virtual:pwa-register/react', () => ({ useRegisterSW: () => ({ needRefresh: [false, vi.fn()], updateServiceWorker: vi.fn() }) }))

const user: User = { id: 'test-rider', profile: { display_name: 'Test Rider', city: 'Almaty', languages: ['en'], interests: '', locale: 'en' }, onboarding_complete: true }
const summary = { id: 'test-ride', city: 'Almaty', date: '2026-12-01', description: 'A gentle city ride', status: 'published' }
const full = { ...summary, title: 'Private ride title', meeting_point: 'Private meeting place', starts_at: '2026-12-01T05:00:00Z', timezone: 'Asia/Almaty', capacity: 8, plan_version: 1, organizer_id: 'organizer', participation_state: 'approved' }

class TestSocket {
  static sockets: TestSocket[] = []
  onopen: (() => void) | null = null
  onclose: (() => void) | null = null
  onmessage: ((event: { data: string }) => void) | null = null
  constructor() { TestSocket.sockets.push(this) }
  close() { /* Explicit test events control connection state. */ }
}

function response(value: unknown, status = 200) { return new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json' } }) }
function mvpRead(url: string) {
  if (url.endsWith('/me/notifications/unread-count')) return response({ unread_count: 0 })
  if (url.endsWith('/reminders')) return response({ day_before: false, hour_before: false })
  if (url.endsWith('/cities')) return response({ cities: [] })
  if (url.endsWith('/me/trust')) return response({ is_admin: false })
  if (url.endsWith('/badge')) return response({ badge: null })
  if (url.includes('/avatar')) return response({ image: null, version: 1 })
  if (url.includes('/announcements')) return response({ announcements: [], next_cursor: '' })
  if (url.includes('/album')) return response({ photos: [], videos: [], version: 1 })
  if (url.endsWith('/me/blocks')) return response({ blocks: [], next_cursor: '' })
  return null
}
// Test-only history controls and a location readout for asserting where the app navigated.
function HistoryControls() {
  const navigate = useNavigate()
  const location = useLocation()
  return <>
    <button onClick={() => void navigate(-1)}>Test history back</button>
    <button onClick={() => void navigate(1)}>Test history forward</button>
    <button onClick={() => void navigate({ pathname: location.pathname, search: '?q=steppe' })}>Test same-page search</button>
    <button onClick={() => void navigate({ pathname: location.pathname, search: location.search, hash: '#ride-results' })}>Test same-page anchor</button>
    <p data-testid="test-location">{location.pathname + location.search + location.hash}</p>
  </>
}

function mount(path = '/', historyControls = false) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } } })
  render(<QueryClientProvider client={client}><MemoryRouter initialEntries={[path]}><I18nProvider><SessionProvider><App />{historyControls && <HistoryControls />}</SessionProvider></I18nProvider></MemoryRouter></QueryClientProvider>)
  return client
}

beforeEach(() => { TestSocket.sockets = []; vi.stubGlobal('WebSocket', TestSocket); setCsrfToken() })
afterEach(() => { vi.unstubAllGlobals(); setCsrfToken() })

// Reads for a signed-in administrator browsing the shell and the Me page.
function navigationReads(url: string) {
  if (url.endsWith('/session')) return response({ user, csrf_token: 'test-csrf' })
  if (url.endsWith('/me/trust')) return response({ is_admin: true })
  if (new URL(url, 'https://bikers.kz').pathname === '/api/v1/rides') return response({ rides: [], next_cursor: '' })
  if (url.endsWith('/clubs')) return response({ clubs: [], next_cursor: '' })
  const extra = mvpRead(url)
  if (extra) return extra
  throw new Error(`Unexpected navigation API call ${url}`)
}
function guestReads(url: string) {
  if (url.endsWith('/session')) return response({ user: null })
  if (new URL(url, 'https://bikers.kz').pathname === '/api/v1/rides') return response({ rides: [], next_cursor: '' })
  const extra = mvpRead(url)
  if (extra) return extra
  throw new Error(`Unexpected guest API call ${url}`)
}
// The top bar and the phone tab bar are both always in the DOM; CSS shows one per screen width.
const primaryNav = (name = 'Main navigation') => screen.getByRole('navigation', { name })
const tabBar = (name = 'Sections') => screen.getByRole('navigation', { name })
function expectLinks(nav: HTMLElement, expected: [name: string, href: string][]) {
  const links = within(nav).getAllByRole('link')
  expect(links.map((link) => link.getAttribute('href'))).toEqual(expected.map(([, href]) => href))
  expected.forEach(([name], index) => expect(links[index]).toHaveAccessibleName(name))
}
function expectCurrent(nav: HTMLElement, name?: string) {
  expect(within(nav).queryAllByRole('link', { current: 'page' })).toEqual(name ? [within(nav).getByRole('link', { name })] : [])
}
// Phosphor draws each weight with its own paths, so a tab's drawing shows whether its icon is filled.
const tabIcons: [name: string, icon: Icon][] = [['Rides', RoadHorizonIcon], ['Map', MapTrifoldIcon], ['Clubs', UsersThreeIcon], ['Me', UserCircleIcon]]
function drawing(markup: string) {
  const template = document.createElement('template')
  template.innerHTML = markup
  return template.content.querySelector('svg')?.innerHTML
}
function expectFilledIcon(tabs: HTMLElement, current?: string) {
  for (const [name, TabIcon] of tabIcons) {
    const shown = within(tabs).getByRole('link', { name }).querySelector('svg')?.innerHTML
    expect(shown, name).toBe(drawing(renderToStaticMarkup(<TabIcon size={24} weight={name === current ? 'fill' : 'regular'} />)))
  }
}
// The unread dot is decorative; the inbox bell carries the count for assistive technology.
const hasUnreadDot = (meTab: HTMLElement) => meTab.querySelector('.tab-dot') !== null

describe('application navigation', () => {
  it.each([
    ['en', ['Main navigation', 'Sections'], ['Rides', 'Map', 'Routes', 'Clubs', 'Motorcycles'], ['New ride', 'New ride', 'Me']],
    ['ru', ['Основная навигация', 'Разделы'], ['Поездки', 'Карта', 'Маршруты', 'Клубы', 'Мотоциклы'], ['Создать поездку', 'Создать', 'Я']],
    ['kk', ['Негізгі навигация', 'Бөлімдер'], ['Сапарлар', 'Карта', 'Бағыттар', 'Клубтар', 'Мотоциклдер'], ['Сапар құру', 'Құру', 'Мен']],
  ])('lists the localized sections in the top bar and the phone tab bar (%s)', async (locale, [mainLabel, tabsLabel], [rides, map, routes, clubs, bikes], [createName, createText, me]) => {
    localStorage.setItem('bikerskz.locale', locale)
    vi.stubGlobal('fetch', vi.fn(navigationReads))
    mount()
    await screen.findByRole('heading', { level: 1, name: rides })
    const primary = primaryNav(mainLabel)
    const tabs = tabBar(tabsLabel)
    expectLinks(primary, [[rides, '/'], [map, '/places'], [routes, '/routes'], [clubs, '/clubs'], [bikes, '/bikes']])
    expectLinks(tabs, [[rides, '/'], [map, '/places'], [createName, '/rides/new'], [clubs, '/clubs'], [me, '/me']])
    expectCurrent(primary, rides)
    expectCurrent(tabs, rides)
    // Both create buttons show a short label. Their accessible name contains it, so speech users can say what they see.
    const create = [within(primary.closest('header')!).getByRole('link', { name: createName }), within(tabs).getByRole('link', { name: createName })]
    create.forEach((link) => {
      expect(link).toHaveAttribute('href', '/rides/new')
      expect(link).toHaveTextContent(createText)
    })
    expect(createName.toLocaleLowerCase(locale)).toContain(createText.toLocaleLowerCase(locale))
  })

  it.each([
    ['/places', 'Map', 'Map', 'Map'],
    ['/routes', 'Routes', 'Routes', 'Map'],
    ['/clubs', 'Clubs', 'Clubs', 'Clubs'],
    ['/me', 'Test Rider', undefined, 'Me'],
    ['/garage', 'My garage', undefined, 'Me'],
    ['/settings', 'Settings', undefined, 'Me'],
    ['/bikes', 'Motorcycles', 'Motorcycles', 'Me'],
    ['/rides/test-ride', 'A ride in Almaty', 'Rides', 'Rides'],
  ])('marks the current section on %s', async (path, heading, primaryCurrent, tabCurrent) => {
    vi.stubGlobal('fetch', vi.fn((url: string) => {
      if (url.endsWith('/rides/test-ride')) return response({ ride: { ...summary, participation_state: 'requested' } })
      if (new URL(url, 'https://bikers.kz').pathname === '/api/v1/bikes') return response({ bikes: [], next_cursor: '' })
      return navigationReads(url)
    }))
    mount(path)
    await screen.findByRole('heading', { level: 1, name: heading })
    expectCurrent(primaryNav(), primaryCurrent)
    expectCurrent(tabBar(), tabCurrent)
    // The current tab is not shown by colour alone: its icon is filled and the others stay outlined.
    expectFilledIcon(tabBar(), tabCurrent)
  })

  // Every route in App.tsx, including those whose pages need more data than these tests provide.
  // The top bar links the section; the tab bar shows places and routes as Map, and bikes as Me.
  it.each([
    ['/', 'rides'], ['/rides/test-ride', 'rides'], ['/rides/test-ride/edit', 'rides'], ['/rides/new', 'create'], ['/rides/new/', 'create'],
    ['/places', 'places'], ['/places/new', 'places'], ['/places/place/suggest', 'places'], ['/places/submissions', 'places'],
    ['/routes', 'routes'], ['/routes/new', 'routes'], ['/routes/record', 'routes'], ['/routes/route', 'routes'], ['/routes/route/edit', 'routes'],
    ['/clubs', 'clubs'], ['/clubs/new', 'clubs'], ['/clubs/club', 'clubs'], ['/clubs/club/edit', 'clubs'],
    ['/me', 'me'], ['/my-rides', 'me'], ['/my-clubs', 'me'], ['/inbox', 'me'], ['/profile', 'me'], ['/settings', 'me'], ['/admin', 'me'], ['/install', 'me'],
    ['/garage', 'me'], ['/garage/new', 'me'], ['/garage/bike', 'me'], ['/garage/bike/edit', 'me'], ['/bikes', 'bikes'], ['/bikes/bike', 'bikes'],
    ['/login', undefined], ['/preview/ride-flow', undefined], ['/rides', undefined], ['/no-such-page', undefined],
    // Unknown paths under a section render the not-found page, so they select nothing either.
    ['/clubs/club/extra', undefined], ['/settings/extra', undefined], ['/rides/test-ride/edit/extra', undefined], ['/places/place', undefined],
    ['/routes/route/edit/extra', undefined], ['/garage/bike/edit/extra', undefined], ['/bikes/bike/extra', undefined], ['/me/extra', undefined],
  ])('assigns %s to the %s section', (path, section) => {
    expect(pageSection(path)).toBe(section)
  })

  it.each(['/clubs/club/extra', '/settings/extra', '/bikes/bike/extra'])('shows the not-found page on %s with no section current', async (path) => {
    vi.stubGlobal('fetch', vi.fn(navigationReads))
    mount(path)
    await screen.findByRole('heading', { level: 1, name: 'That page couldn’t be found.' })
    expectCurrent(primaryNav())
    expectCurrent(tabBar())
    expectFilledIcon(tabBar())
    expect(within(screen.getByRole('main')).getByRole('link', { name: 'Back to rides' })).toHaveAttribute('href', '/')
  })

  // Pages reached from the Me page lead back to it. Links to the rides feed keep "Back to rides".
  it.each(['/settings', '/install', '/inbox', '/admin', '/profile'])('leads back to the Me page from %s', async (path) => {
    vi.stubGlobal('fetch', vi.fn(navigationReads))
    mount(path)
    const main = screen.getByRole('main')
    expect(await within(main).findByRole('link', { name: 'Back to your space' })).toHaveAttribute('href', '/me')
    expect(within(main).queryByRole('link', { name: 'Back to rides' })).not.toBeInTheDocument()
  })

  it.each([
    ['ru', 'К вашему пространству'],
    ['kk', 'Жеке кеңістікке оралу'],
  ])('names the way back to the Me page in %s', async (locale, label) => {
    localStorage.setItem('bikerskz.locale', locale)
    vi.stubGlobal('fetch', vi.fn(navigationReads))
    mount('/install')
    expect(await within(screen.getByRole('main')).findByRole('link', { name: label })).toHaveAttribute('href', '/me')
  })

  it('leads a new rider from onboarding back to the rides feed', async () => {
    vi.stubGlobal('fetch', vi.fn((url: string) => url.endsWith('/session') ? response({ user: { ...user, onboarding_complete: false }, csrf_token: 'test-csrf' }) : navigationReads(url)))
    mount('/profile')
    const main = screen.getByRole('main')
    await within(main).findByRole('heading', { level: 1, name: 'Create your profile' })
    expect(within(main).getByRole('link', { name: 'Back to rides' })).toHaveAttribute('href', '/')
    expect(within(main).queryByRole('link', { name: 'Back to your space' })).not.toBeInTheDocument()
  })

  it.each([
    [3, 'Inbox, 3 unread updates', true],
    [0, 'Inbox', false],
  ])('shows account controls to a signed-in rider with %i unread updates', async (count, inboxName, dot) => {
    vi.stubGlobal('fetch', vi.fn((url: string) => url.endsWith('/me/notifications/unread-count') ? response({ unread_count: count }) : navigationReads(url)))
    mount()
    expect(await screen.findByRole('link', { name: inboxName })).toHaveAttribute('href', '/inbox')
    expect(screen.getByRole('link', { name: 'Your space and settings' })).toHaveAttribute('href', '/me')
    expect(screen.queryByRole('link', { name: 'Sign in' })).not.toBeInTheDocument()
    expect(hasUnreadDot(within(tabBar()).getByRole('link', { name: 'Me' }))).toBe(dot)
  })

  it('offers guests sign-in instead of account controls', async () => {
    vi.stubGlobal('fetch', vi.fn(guestReads))
    mount()
    expect(await screen.findByRole('link', { name: 'Sign in' })).toHaveAttribute('href', '/login')
    expect(screen.queryByRole('link', { name: /^Inbox/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Your space and settings' })).not.toBeInTheDocument()
    const me = within(tabBar()).getByRole('link', { name: 'Me' })
    expect(me).toHaveAttribute('href', '/me')
    expect(hasUnreadDot(me)).toBe(false)
  })

  it('starts a new page at its main content without overriding history or same-page navigation', async () => {
    vi.stubGlobal('fetch', vi.fn(navigationReads))
    mount('/', true)
    await screen.findByRole('heading', { level: 1, name: 'Rides' })
    const actor = userEvent.setup()
    const main = screen.getByRole('main')
    const location = screen.getByTestId('test-location')
    const scroll = vi.mocked(window.scrollTo)
    scroll.mockClear()
    await actor.click(within(primaryNav()).getByRole('link', { name: 'Clubs' }))
    await screen.findByRole('heading', { level: 1, name: 'Clubs' })
    // The lazy page can render before the parent route's scroll and focus effects finish.
    await waitFor(() => {
      expect(scroll).toHaveBeenCalledWith(0, 0)
      expect(main).toHaveFocus()
    })
    scroll.mockClear()
    // Flush the POP effects before asserting that they leave scroll and focus alone.
    const back = screen.getByRole('button', { name: 'Test history back' })
    await act(async () => { await actor.click(back) })
    await screen.findByRole('heading', { level: 1, name: 'Rides' })
    expect(scroll).not.toHaveBeenCalled()
    expect(back).toHaveFocus()
    const forward = screen.getByRole('button', { name: 'Test history forward' })
    await act(async () => { await actor.click(forward) })
    await screen.findByRole('heading', { level: 1, name: 'Clubs' })
    expect(scroll).not.toHaveBeenCalled()
    expect(forward).toHaveFocus()
    await actor.click(within(tabBar()).getByRole('link', { name: 'Rides' }))
    await screen.findByRole('heading', { level: 1, name: 'Rides' })
    await waitFor(() => {
      expect(scroll).toHaveBeenCalledWith(0, 0)
      expect(main).toHaveFocus()
    })
    scroll.mockClear()
    const search = screen.getByRole('button', { name: 'Test same-page search' })
    await act(async () => { await actor.click(search) })
    expect(location).toHaveTextContent('/?q=steppe')
    expect(scroll).not.toHaveBeenCalled()
    expect(search).toHaveFocus()
    const anchor = screen.getByRole('button', { name: 'Test same-page anchor' })
    await act(async () => { await actor.click(anchor) })
    expect(location).toHaveTextContent('/?q=steppe#ride-results')
    expect(scroll).not.toHaveBeenCalled()
    expect(anchor).toHaveFocus()
  })
})

describe('Me page', () => {
  function signOutReads(signOut: () => Response) {
    return vi.fn((url: string, options?: RequestInit) => url.endsWith('/session') && options?.method === 'DELETE' ? signOut() : navigationReads(url))
  }

  it('invites guests to sign in and keeps the shared sections within reach', async () => {
    vi.stubGlobal('fetch', vi.fn(guestReads))
    mount('/me')
    const main = screen.getByRole('main')
    await within(main).findByRole('heading', { level: 1, name: 'Your space' })
    expect(within(main).getByRole('link', { name: 'Sign in' })).toHaveAttribute('href', '/login?next=%2Fme')
    expectLinks(within(main).getByRole('region', { name: 'Explore' }), [['Routes', '/routes'], ['Motorcycles', '/bikes'], ['Map', '/places'], ['Install the app', '/install']])
    expect(within(main).queryByRole('link', { name: 'My rides' })).not.toBeInTheDocument()
    expect(within(main).queryByRole('button', { name: 'Sign out' })).not.toBeInTheDocument()
  })

  it.each([true, false])('gathers personal pages, shared sections and account actions (admin: %s)', async (admin) => {
    vi.stubGlobal('fetch', vi.fn((url: string) => url.endsWith('/me/trust') ? response({ is_admin: admin }) : navigationReads(url)))
    mount('/me')
    const main = screen.getByRole('main')
    await within(main).findByRole('heading', { level: 1, name: 'Test Rider' })
    expect(within(main).getByRole('link', { name: 'Edit profile' })).toHaveAttribute('href', '/profile')
    expectLinks(within(main).getByRole('region', { name: 'Your space' }), [['My rides', '/my-rides'], ['My garage', '/garage'], ['My clubs', '/my-clubs'], ['Inbox', '/inbox']])
    expectLinks(within(main).getByRole('region', { name: 'Explore' }), [['Routes', '/routes'], ['Motorcycles', '/bikes'], ['Map', '/places'], ['Install the app', '/install']])
    const account = within(main).getByRole('region', { name: 'Account' })
    // Trust loads after the session, so wait for the administrator link before checking the list.
    if (admin) await within(account).findByRole('link', { name: 'Administration' })
    expectLinks(account, admin ? [['Settings', '/settings'], ['Administration', '/admin']] : [['Settings', '/settings']])
    expect(within(account).getByRole('button', { name: 'Sign out' })).toBeEnabled()
  })

  it('signs out, clears the session and returns to the rides page', async () => {
    const fetch = signOutReads(() => new Response(null, { status: 204 }))
    vi.stubGlobal('fetch', fetch)
    const client = mount('/me', true)
    const actor = userEvent.setup()
    await actor.click(await screen.findByRole('button', { name: 'Sign out' }))
    await screen.findByRole('heading', { level: 1, name: 'Rides' })
    expect(screen.getByTestId('test-location')).toHaveTextContent(/^\/$/)
    expect(client.getQueryData(['session'])).toEqual({ user: null })
    expect(screen.getByRole('link', { name: 'Sign in' })).toHaveAttribute('href', '/login')
    expect(screen.queryByRole('link', { name: 'Your space and settings' })).not.toBeInTheDocument()
    const call = fetch.mock.calls.find(([, options]) => options?.method === 'DELETE')
    expect(call?.[0]).toBe('/api/v1/session')
    expect(new Headers(call?.[1]?.headers).get('X-CSRF-Token')).toBe('test-csrf')
    await waitFor(() => expect(screen.getByRole('main')).toHaveFocus())
  })

  it('disables sign-out while offline', async () => {
    const online = vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(false)
    const fetch = signOutReads(() => new Response(null, { status: 204 }))
    vi.stubGlobal('fetch', fetch)
    mount('/me')
    const signOut = await screen.findByRole('button', { name: 'Sign out' })
    expect(signOut).toBeDisabled()
    expect(screen.getByText('You’re offline. Reconnect to load rides and make changes.')).toBeInTheDocument()
    online.mockReturnValue(true)
    act(() => { fireEvent(window, new Event('online')) })
    expect(signOut).toBeEnabled()
    expect(fetch.mock.calls.some(([, options]) => options?.method === 'DELETE')).toBe(false)
  })

  it('keeps the rider signed in and explains a failed sign-out', async () => {
    vi.stubGlobal('fetch', signOutReads(() => response({ error: { code: 'internal_error' } }, 500)))
    const client = mount('/me', true)
    const actor = userEvent.setup()
    const signOut = await screen.findByRole('button', { name: 'Sign out' })
    await actor.click(signOut)
    expect(await screen.findByRole('alert')).toHaveTextContent('Something went wrong. Please try again.')
    expect(screen.getByRole('heading', { level: 1, name: 'Test Rider' })).toBeInTheDocument()
    expect(screen.getByTestId('test-location')).toHaveTextContent(/^\/me$/)
    expect(client.getQueryData(['session'])).toMatchObject({ user: { id: user.id } })
    expect(signOut).toBeEnabled()
  })
})

describe('authentication and onboarding', () => {
  it('titles the sign-in page once and gives only the code step its own heading', async () => {
    vi.stubGlobal('fetch', vi.fn(async (url: string) => url.endsWith('/auth/challenges')
      ? response({ challenge_id: 'server-challenge', expires_at: new Date(Date.now() + 300_000).toISOString(), resend_after: new Date(Date.now() + 60_000).toISOString() })
      : guestReads(url)))
    mount('/login')
    const actor = userEvent.setup()
    const main = screen.getByRole('main')
    const title = await within(main).findByRole('heading', { level: 1, name: 'Sign in' })
    expect(within(main).getAllByRole('heading')).toEqual([title])
    expect(within(main).getByRole('link', { name: 'Back to rides' })).toHaveAttribute('href', '/')
    const card = within(main).getByRole('region', { name: 'Sign in' })
    await actor.type(within(card).getByLabelText(/Phone number/), '+7 700 000 00 00')
    await actor.click(within(card).getByRole('button', { name: /Send Telegram code/ }))
    const step = await within(main).findByRole('heading', { level: 2, name: 'Check Telegram.' })
    expect(within(main).getAllByRole('heading')).toEqual([title, step])
    expect(within(main).getByRole('region', { name: 'Check Telegram.' })).toContainElement(within(main).getByLabelText(/Verification code/))
  })

  it('binds verification to the returned challenge and keeps OTP material out of browser storage', async () => {
    const fetch = vi.fn(async (url: string, options?: RequestInit) => {
      if (url.endsWith('/session')) return response({ user: null })
      if (url.endsWith('/auth/challenges')) return response({ challenge_id: 'server-challenge', expires_at: new Date(Date.now() + 300_000).toISOString(), resend_after: new Date(Date.now() + 60_000).toISOString() })
      if (url.endsWith('/auth/challenges/verify')) return response({ user: { ...user, onboarding_complete: false }, csrf_token: 'test-csrf' })
      const extra = mvpRead(url); if (extra) return extra
      throw new Error(`Unexpected API call ${url} ${options?.method}`)
    })
    vi.stubGlobal('fetch', fetch)
    mount('/login')
    const actor = userEvent.setup()
    await actor.type(await screen.findByLabelText(/Phone number/), '+7 700 000 00 00')
    await actor.click(screen.getByRole('button', { name: /Send Telegram code/ }))
    await actor.type(await screen.findByLabelText(/Verification code/), '123456')
    await actor.click(screen.getByRole('button', { name: /Verify and continue/ }))
    await screen.findByRole('heading', { name: 'Create your profile' })
    const call = fetch.mock.calls.find(([url]) => url.endsWith('/auth/challenges/verify'))
    expect(JSON.parse(call?.[1]?.body as string)).toEqual({ challenge_id: 'server-challenge', code: '123456' })
    expect(Object.keys(localStorage)).toEqual(['bikerskz.locale'])
    expect(localStorage.getItem('bikerskz.locale')).toBe('en')
  })

  it('requires a spoken language and accepts profiles without a motorcycle', async () => {
    const fetch = vi.fn(async (url: string, options?: RequestInit) => {
      if (url.endsWith('/session')) return response({ user, csrf_token: 'test-csrf' })
      if (url.endsWith('/me/profile')) return response({ user })
      const extra = mvpRead(url); if (extra) return extra
      throw new Error(`Unexpected API call ${url} ${options?.method}`)
    })
    vi.stubGlobal('fetch', fetch)
    mount('/profile')
    const actor = userEvent.setup()
    await actor.click(await screen.findByLabelText('English'))
    await actor.click(screen.getByRole('button', { name: /Save profile/ }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Select at least one language')
    expect(fetch.mock.calls.some(([url]) => url.endsWith('/me/profile'))).toBe(false)
    await actor.click(screen.getByLabelText('Қазақша'))
    await actor.click(screen.getByRole('button', { name: /Save profile/ }))
    await screen.findByText('Changes saved.')
    const call = fetch.mock.calls.find(([url]) => url.endsWith('/me/profile'))
    expect(JSON.parse(call?.[1]?.body as string)).toMatchObject({ display_name: 'Test Rider', languages: ['kk'] })
  })

  // The API accepts a profile or ride city of up to 160 UTF-8 bytes: 80 Cyrillic letters.
  it.each(['/profile', '/rides/new'])('accepts as long a city on %s as the API does', async (path) => {
    vi.stubGlobal('fetch', vi.fn(navigationReads))
    mount(path)
    const city = await within(screen.getByRole('main')).findByRole('combobox', { name: 'City' })
    expect(city).not.toHaveAttribute('maxlength')
    fireEvent.change(city, { target: { value: 'Ж'.repeat(80) } })
    expect(city).not.toHaveAttribute('aria-invalid')
    fireEvent.change(city, { target: { value: 'Ж'.repeat(81) } })
    expect(city).toHaveAttribute('aria-invalid', 'true')
  })
})

describe('ride privacy and localization', () => {
  it('keeps pending riders on the public preview without fetching room data', async () => {
    const fetch = vi.fn(async (url: string) => {
      if (url.endsWith('/session')) return response({ user, csrf_token: 'test-csrf' })
      if (url.endsWith('/rides/test-ride')) return response({ ride: { ...summary, participation_state: 'requested' } })
      const extra = mvpRead(url); if (extra) return extra
      throw new Error(`Unexpected private read ${url}`)
    })
    vi.stubGlobal('fetch', fetch)
    mount('/rides/test-ride')
    await screen.findByRole('heading', { name: 'Your request is with the organizer.' })
    expect(screen.queryByText('Private meeting place')).not.toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Ride Room' })).not.toBeInTheDocument()
    expect(fetch.mock.calls.some(([url]) => /messages|participants|events/.test(url))).toBe(false)
  })

  it('immediately removes private details and chat when the event connection closes', async () => {
    let authorized = true
    vi.stubGlobal('fetch', vi.fn(async (url: string) => {
      if (url.endsWith('/session')) return response({ user, csrf_token: 'test-csrf' })
      if (url.endsWith('/rides/test-ride')) return response({ ride: authorized ? full : summary })
      if (url.includes('/participants')) return response({ participants: [{ user_id: user.id, display_name: user.profile.display_name, state: 'approved' }] })
      if (url.includes('/messages')) return response({ messages: [{ id: 'm1', sequence: 1, sender_id: user.id, sender_name: user.profile.display_name, body: 'Private conversation', created_at: '2026-12-01T06:00:00Z', client_message_id: 'test-command' }], next_sequence: 1 })
      const extra = mvpRead(url); if (extra) return extra
      throw new Error(`Unexpected API call ${url}`)
    }))
    const client = mount('/rides/test-ride')
    await screen.findByText('Private meeting place')
    await screen.findByText('Private conversation')
    authorized = false
    act(() => { TestSocket.sockets[0].onclose?.() })
    expect(screen.queryByText('Private meeting place')).not.toBeInTheDocument()
    expect(screen.queryByText('Private conversation')).not.toBeInTheDocument()
    expect(client.getQueryData(['messages', summary.id, user.id])).toBeUndefined()
  })

  it('clears private caches when session restoration returns an anonymous 200 response', async () => {
    let signedIn = true
    vi.stubGlobal('fetch', vi.fn(async (url: string) => {
      if (url.endsWith('/session')) return response(signedIn ? { user, csrf_token: 'test-csrf' } : { user: null })
      if (url.endsWith('/rides/test-ride')) return response({ ride: signedIn ? full : summary })
      if (url.includes('/participants')) return response({ participants: [] })
      if (url.includes('/messages')) return response({ messages: [], next_sequence: 0 })
      const extra = mvpRead(url); if (extra) return extra
      throw new Error(`Unexpected API call ${url}`)
    }))
    const client = mount('/rides/test-ride')
    await screen.findByText('Private meeting place')
    signedIn = false
    await act(async () => { await client.refetchQueries({ queryKey: ['session'] }) })
    await waitFor(() => expect(screen.queryByText('Private meeting place')).not.toBeInTheDocument())
    expect(client.getQueryData(['ride', summary.id, user.id])).toBeUndefined()
    expect(client.getQueryData(['messages', summary.id, user.id])).toBeUndefined()
  })

  it('preserves an uncertain send across a connection recheck and retries the original command ID', async () => {
    const commands: { body: string; client_message_id: string }[] = []
    let messages: Message[] = []
    vi.stubGlobal('fetch', vi.fn(async (url: string, options?: RequestInit) => {
      if (url.endsWith('/session')) return response({ user, csrf_token: 'test-csrf' })
      if (url.endsWith('/rides/test-ride')) return response({ ride: full })
      if (url.includes('/participants')) return response({ participants: [] })
      if (url.endsWith('/messages') && options?.method === 'POST') {
        const command = JSON.parse(options.body as string)
        commands.push(command)
        if (commands.length === 1) throw new TypeError('Lost response')
        messages = [{ id: 'm1', sequence: 1, sender_id: user.id, sender_name: user.profile.display_name, body: command.body, created_at: '2026-12-01T06:00:00Z', client_message_id: command.client_message_id }]
        return response({ message: messages[0] })
      }
      if (url.includes('/messages')) return response({ messages, next_sequence: messages.length })
      const extra = mvpRead(url); if (extra) return extra
      throw new Error(`Unexpected API call ${url}`)
    }))
    mount('/rides/test-ride')
    const actor = userEvent.setup()
    await actor.type(await screen.findByRole('textbox', { name: 'Message the Ride Room' }), 'Uncertain send')
    await actor.click(screen.getByRole('button', { name: 'Send message' }))
    await screen.findByText('Connection interrupted. Check your connection and try again.')
    act(() => { TestSocket.sockets[0].onclose?.() })
    const composer = await screen.findByRole('textbox', { name: 'Message the Ride Room' }, { timeout: 3000 })
    expect(composer).toHaveValue('Uncertain send')
    await actor.click(screen.getByRole('button', { name: 'Send message' }))
    await screen.findByText('Uncertain send')
    const message = screen.getByText('Uncertain send').closest<HTMLElement>('.message')!
    expect(within(message).getByRole('button', { name: 'Message actions for Test Rider' })).toBeInTheDocument()
    expect(within(message).queryByRole('button', { name: 'Report' })).not.toBeInTheDocument()
    expect(commands).toHaveLength(2)
    expect(commands[1].client_message_id).toBe(commands[0].client_message_id)
    expect(composer).toHaveValue('')
  })

  it('reconciles a committed send whose response was lost when history returns after reconnect', async () => {
    const commands: { body: string; client_message_id: string }[] = []
    let committed = false, historyRequested = false
    let releaseHistory = () => {}
    const historyReady = new Promise<void>((resolve) => { releaseHistory = resolve })
    vi.stubGlobal('fetch', vi.fn(async (url: string, options?: RequestInit) => {
      if (url.endsWith('/session')) return response({ user, csrf_token: 'test-csrf' })
      if (url.endsWith('/rides/test-ride')) return response({ ride: full })
      if (url.includes('/participants')) return response({ participants: [] })
      if (url.endsWith('/messages') && options?.method === 'POST') {
        commands.push(JSON.parse(options.body as string)); committed = true
        throw new TypeError('Committed but response lost')
      }
      if (url.includes('/messages')) {
        if (committed) { historyRequested = true; await historyReady }
        return response({ messages: committed ? [{ id: 'm1', sequence: 1, sender_id: user.id, sender_name: user.profile.display_name, body: commands[0].body, created_at: '2026-12-01T06:00:00Z', client_message_id: commands[0].client_message_id }] : [], next_sequence: committed ? 1 : 0 })
      }
      const extra = mvpRead(url); if (extra) return extra
      throw new Error(`Unexpected API call ${url}`)
    }))
    mount('/rides/test-ride')
    const actor = userEvent.setup()
    await actor.type(await screen.findByRole('textbox', { name: 'Message the Ride Room' }), 'Already committed')
    await actor.click(screen.getByRole('button', { name: 'Send message' }))
    await screen.findByText('Connection interrupted. Check your connection and try again.')
    act(() => { TestSocket.sockets[0].onclose?.() })
    await waitFor(() => expect(historyRequested).toBe(true), { timeout: 3000 })
    const composer = screen.getByRole('textbox', { name: 'Message the Ride Room' })
    try {
      expect(composer).toHaveValue('Already committed')
      expect(screen.queryByText('Already committed', { selector: '.message-content p' })).not.toBeInTheDocument()
    } finally {
      await act(async () => releaseHistory())
    }
    // The restored textarea also contains the draft text; only a history message proves acceptance.
    await screen.findByText('Already committed', { selector: '.message-content p' }, { timeout: 3000 })
    await waitFor(() => expect(composer).toHaveValue(''))
    expect(screen.getByRole('button', { name: 'Send message' })).toBeDisabled()
    expect(screen.getAllByText('Already committed', { selector: '.message-content p' })).toHaveLength(1)
    expect(commands).toHaveLength(1)
  })

  it('switches core screens between English, Russian and Kazakh using locale-only storage', async () => {
    vi.stubGlobal('fetch', vi.fn(async (url: string) => response(url.endsWith('/session') ? { user: null } : url.endsWith('/cities') ? { cities: [] } : { rides: [], next_cursor: '' })))
    mount()
    await screen.findByRole('heading', { name: 'Rides' })
    fireEvent.change(screen.getByRole('combobox', { name: 'Language' }), { target: { value: 'ru' } })
    expect(screen.getByRole('heading', { name: 'Поездки' })).toBeInTheDocument()
    fireEvent.change(screen.getByRole('combobox', { name: 'Язык' }), { target: { value: 'kk' } })
    expect(screen.getByRole('heading', { name: 'Сапарлар' })).toBeInTheDocument()
    expect(localStorage.getItem('bikerskz.locale')).toBe('kk')
    expect(document.documentElement.lang).toBe('kk')
  })
})

describe('ride instants and durable message reconciliation', () => {
  it('converts using the ride IANA timezone rather than the browser timezone', () => {
    expect(rideTimeToInstant('2026-12-01T10:30', 'Asia/Almaty').toISOString()).toBe('2026-12-01T05:30:00.000Z')
    expect(rideTimeToInstant('2026-07-01T10:30', 'Europe/Berlin').toISOString()).toBe('2026-07-01T08:30:00.000Z')
    expect(localRideTime('2026-07-01T08:30:00Z', 'Europe/Berlin')).toBe('2026-07-01T10:30')
  })

  it('renders Kazakh calendar text without relying on browser Intl locale availability', () => {
    expect(dateParts('2026-11-05', 'kk').full).toContain('қараша')
    expect(dateParts('2026-11-05', 'kk').full).toContain('бейсенбі')
    expect(formatRideDateTime('2026-11-05T05:30:00Z', 'Asia/Almaty', 'kk')).toContain('қараша')
  })

  it('rejects ambiguous external redirect paths', () => {
    expect(validNext('/\\evil.example')).toBe('/')
    expect(validNext('//evil.example')).toBe('/')
    expect(validNext('/rides/example')).toBe('/rides/example')
  })

  it('orders committed messages and deduplicates repeated reconnect batches by sequence', () => {
    const message = (sequence: number, body: string): Message => ({ id: String(sequence), sequence, sender_id: 'test', sender_name: 'Test', body, created_at: '2026-12-01T00:00:00Z', client_message_id: `command-${sequence}` })
    expect(mergeMessages([message(2, 'Second')], [message(1, 'First'), message(2, 'Second')]).map((item) => item.body)).toEqual(['First', 'Second'])
  })
})
