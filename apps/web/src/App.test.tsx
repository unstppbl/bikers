import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, useNavigate } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { App } from './App'
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
function HistoryControls() {
  const navigate = useNavigate()
  return <button onClick={() => void navigate(-1)}>Test history back</button>
}

function mount(path = '/', historyControls = false) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } } })
  render(<QueryClientProvider client={client}><MemoryRouter initialEntries={[path]}><I18nProvider><SessionProvider><App />{historyControls && <HistoryControls />}</SessionProvider></I18nProvider></MemoryRouter></QueryClientProvider>)
  return client
}

beforeEach(() => { TestSocket.sockets = []; vi.stubGlobal('WebSocket', TestSocket); setCsrfToken() })
afterEach(() => { vi.unstubAllGlobals(); setCsrfToken() })

describe('application navigation', () => {
  function navigationReads(url: string) {
    if (url.endsWith('/session')) return response({ user, csrf_token: 'test-csrf' })
    if (url.endsWith('/me/trust')) return response({ is_admin: true })
    if (new URL(url, 'https://bikers.kz').pathname === '/api/v1/rides') return response({ rides: [], next_cursor: '' })
    if (url.endsWith('/clubs')) return response({ clubs: [], next_cursor: '' })
    const extra = mvpRead(url)
    if (extra) return extra
    throw new Error(`Unexpected navigation API call ${url}`)
  }

  it.each([
    ['en', 'Map', 'Motorcycles', 'Navigation'],
    ['ru', 'Карта', 'Мотоциклы', 'Навигация'],
    ['kk', 'Карта', 'Мотоциклдер', 'Навигация'],
  ])('keeps the localized map directly below motorcycles in shared desktop and mobile navigation (%s)', async (locale, mapLabel, bikesLabel, navigationLabel) => {
    localStorage.setItem('bikerskz.locale', locale)
    vi.stubGlobal('fetch', vi.fn(navigationReads))
    mount()
    const desktop = screen.getByRole('complementary', { name: navigationLabel })
    const map = within(desktop).getByRole('link', { name: mapLabel })
    const bikes = within(desktop).getByRole('link', { name: bikesLabel })
    expect(map).toHaveAttribute('href', '/places')
    expect(bikes).toHaveAttribute('href', '/bikes')
    expect(bikes.nextElementSibling).toBe(map)
    await userEvent.click(screen.getByRole('button', { name: navigationLabel }))
    const mobile = screen.getByRole('dialog', { name: navigationLabel })
    expect(within(mobile).getByRole('link', { name: mapLabel })).toBe(map)
    expect(within(mobile).getByRole('link', { name: bikesLabel }).nextElementSibling).toBe(map)
  })

  it('contains mobile-menu keyboard focus and restores it after Escape or a desktop resize', async () => {
    vi.stubGlobal('fetch', vi.fn(navigationReads))
    mount()
    await screen.findByRole('heading', { name: 'The road is open.' })
    const actor = userEvent.setup()
    const trigger = screen.getByRole('button', { name: 'Navigation' })
    const main = screen.getByRole('main')
    const previousOverflow = document.body.style.overflow
    await actor.click(trigger)
    const dialog = screen.getByRole('dialog', { name: 'Navigation' })
    expect(trigger).toHaveAttribute('aria-expanded', 'true')
    expect(within(dialog).getByRole('button', { name: 'Close' })).toHaveFocus()
    expect(main.parentElement).toHaveAttribute('inert')
    expect(screen.queryByRole('main')).not.toBeInTheDocument()
    expect(document.body.style.overflow).toBe('hidden')
    const last = within(dialog).getByRole('button', { name: 'Sign out' })
    last.focus()
    await actor.tab()
    expect(within(dialog).getByRole('link', { name: /BIKERS/ })).toHaveFocus()
    await actor.tab({ shift: true })
    expect(last).toHaveFocus()
    await actor.keyboard('{Escape}')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(trigger).toHaveFocus()
    expect(trigger).toHaveAttribute('aria-expanded', 'false')
    expect(main.parentElement).not.toHaveAttribute('inert')
    expect(document.body.style.overflow).toBe(previousOverflow)
    await actor.click(trigger)
    fireEvent(window, new Event('resize'))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(main.parentElement).not.toHaveAttribute('inert')
  })

  it('starts a new page at its main content without overriding Back navigation', async () => {
    vi.stubGlobal('fetch', vi.fn(navigationReads))
    mount('/', true)
    await screen.findByRole('heading', { name: 'The road is open.' })
    const actor = userEvent.setup()
    const scroll = vi.mocked(window.scrollTo)
    scroll.mockClear()
    await actor.click(screen.getByRole('button', { name: 'Navigation' }))
    await actor.click(within(screen.getByRole('dialog', { name: 'Navigation' })).getByRole('link', { name: 'Clubs' }))
    await screen.findByRole('heading', { name: 'A place for your riding people.' })
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(scroll).toHaveBeenCalledWith(0, 0)
    expect(screen.getByRole('main')).toHaveFocus()
    scroll.mockClear()
    await actor.click(screen.getByRole('button', { name: 'Test history back' }))
    await screen.findByRole('heading', { name: 'The road is open.' })
    expect(scroll).not.toHaveBeenCalled()
    await actor.click(screen.getByRole('link', { name: 'Rides' }))
    expect(scroll).toHaveBeenCalledWith(0, 0)
    expect(screen.getByRole('main')).toHaveFocus()
  })
})

describe('authentication and onboarding', () => {
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
    await screen.findByRole('heading', { name: 'Make yourself at home.' })
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
    expect(commands).toHaveLength(2)
    expect(commands[1].client_message_id).toBe(commands[0].client_message_id)
    expect(composer).toHaveValue('')
  })

  it('reconciles a committed send whose response was lost when history returns after reconnect', async () => {
    const commands: { body: string; client_message_id: string }[] = []
    let committed = false
    vi.stubGlobal('fetch', vi.fn(async (url: string, options?: RequestInit) => {
      if (url.endsWith('/session')) return response({ user, csrf_token: 'test-csrf' })
      if (url.endsWith('/rides/test-ride')) return response({ ride: full })
      if (url.includes('/participants')) return response({ participants: [] })
      if (url.endsWith('/messages') && options?.method === 'POST') {
        commands.push(JSON.parse(options.body as string)); committed = true
        throw new TypeError('Committed but response lost')
      }
      if (url.includes('/messages')) return response({ messages: committed ? [{ id: 'm1', sequence: 1, sender_id: user.id, sender_name: user.profile.display_name, body: commands[0].body, created_at: '2026-12-01T06:00:00Z', client_message_id: commands[0].client_message_id }] : [], next_sequence: committed ? 1 : 0 })
      const extra = mvpRead(url); if (extra) return extra
      throw new Error(`Unexpected API call ${url}`)
    }))
    mount('/rides/test-ride')
    const actor = userEvent.setup()
    await actor.type(await screen.findByRole('textbox', { name: 'Message the Ride Room' }), 'Already committed')
    await actor.click(screen.getByRole('button', { name: 'Send message' }))
    await screen.findByText('Connection interrupted. Check your connection and try again.')
    act(() => { TestSocket.sockets[0].onclose?.() })
    await screen.findByText('Already committed', {}, { timeout: 3000 })
    expect(screen.getByRole('textbox', { name: 'Message the Ride Room' })).toHaveValue('')
    expect(screen.getByRole('button', { name: 'Send message' })).toBeDisabled()
    expect(commands).toHaveLength(1)
  })

  it('switches core screens between English, Russian and Kazakh using locale-only storage', async () => {
    vi.stubGlobal('fetch', vi.fn(async (url: string) => response(url.endsWith('/session') ? { user: null } : url.endsWith('/cities') ? { cities: [] } : { rides: [], next_cursor: '' })))
    mount()
    await screen.findByRole('heading', { name: 'The road is open.' })
    fireEvent.change(screen.getByRole('combobox', { name: 'Language' }), { target: { value: 'ru' } })
    expect(screen.getByRole('heading', { name: 'Дорога открыта.' })).toBeInTheDocument()
    fireEvent.change(screen.getByRole('combobox', { name: 'Язык' }), { target: { value: 'kk' } })
    expect(screen.getByRole('heading', { name: 'Жол ашық.' })).toBeInTheDocument()
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
