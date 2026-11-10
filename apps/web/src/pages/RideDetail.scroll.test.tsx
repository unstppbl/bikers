import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, useNavigate } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { App } from '../App'
import { setCsrfToken } from '../api'
import { I18nProvider } from '../i18n'
import { SessionProvider } from '../session'
import type { Message, User } from '../types'

vi.mock('virtual:pwa-register/react', () => ({ useRegisterSW: () => ({ needRefresh: [false, vi.fn()], updateServiceWorker: vi.fn() }) }))

const actor: User = { id: 'scroll-rider', profile: { display_name: 'Scroll rider', city: 'Almaty', languages: ['en'], locale: 'en', interests: '' }, onboarding_complete: true }
const response = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
const history = (ride: string, count = 20): Message[] => Array.from({ length: count }, (_, index) => ({ id: `${ride}-message-${index + 1}`, sequence: index + 1, sender_id: actor.id, sender_name: actor.profile.display_name, body: `${ride} message ${index + 1}`, created_at: '2026-11-10T10:00:00Z', client_message_id: '' }))
const ride = (id: string) => ({ id, city: 'Almaty', date: '2026-12-01', description: 'Local scroll fixture', status: 'published', title: id === 'ride-a' ? 'Ride A plan' : 'Ride B plan', meeting_point: 'Test meeting', starts_at: '2026-12-01T05:00:00Z', timezone: 'Asia/Almaty', capacity: 8, plan_version: 1, organizer_id: 'organizer', participation_state: 'approved' })
class TestSocket {
  static sockets: TestSocket[] = []
  onopen: (() => void) | null = null
  onclose: (() => void) | null = null
  onmessage: ((event: { data: string }) => void) | null = null
  constructor() { TestSocket.sockets.push(this) }
  close() {}
}
function Controls() {
  const navigate = useNavigate()
  return <><button onClick={() => void navigate('/rides/ride-a')}>Open ride A</button><button onClick={() => void navigate('/rides/ride-b')}>Open ride B</button><button onClick={() => void navigate(-1)}>Go back</button></>
}
function fixture(path = '/rides/ride-a') {
  const messages = new Map([['ride-a', history('ride-a')], ['ride-b', history('ride-b')]])
  const gates = new Map<string, { promise: Promise<void>; resolve: () => void }>()
  for (const id of messages.keys()) { let resolve = () => {}; const promise = new Promise<void>((done) => { resolve = done }); gates.set(id, { promise, resolve }) }
  const fetch = vi.fn(async (raw: string) => {
    const url = new URL(raw, 'https://bikers.kz'), match = url.pathname.match(/^\/api\/v1\/rides\/(ride-[ab])(.*)$/)
    if (url.pathname === '/api/v1/session') return response({ user: actor, csrf_token: 'scroll-csrf' })
    if (url.pathname === '/api/v1/rides') return response({ rides: [], next_cursor: '' })
    if (match) {
      const [, id, suffix] = match
      if (!suffix) return response({ ride: ride(id) })
      if (suffix === '/messages') { await gates.get(id)!.promise; const after = Number(url.searchParams.get('after') ?? 0), rows = messages.get(id)!; return response({ messages: rows.filter((m) => m.sequence > after), next_sequence: rows.at(-1)?.sequence ?? 0 }) }
      if (suffix === '/participants') return response({ participants: [] })
      if (suffix === '/announcements') return response({ announcements: [], next_cursor: '' })
      if (suffix === '/reminders') return response({ day_before: false, hour_before: false })
      if (suffix === '/album') return response({ photos: [], videos: [], version: 1 })
    }
    if (url.pathname.endsWith('/me/trust')) return response({ is_admin: false })
    if (url.pathname.endsWith('/me/blocks')) return response({ blocks: [], next_cursor: '' })
    if (url.pathname.endsWith('/unread-count')) return response({ unread_count: 0 })
    if (url.pathname.includes('/avatar')) return response({ image: null, version: 1 })
    if (url.pathname.endsWith('/badge')) return response({ badge: null })
    if (url.pathname.endsWith('/cities')) return response({ cities: [] })
    throw new Error(`Unexpected scroll fixture request ${raw}`)
  })
  vi.stubGlobal('fetch', fetch)
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } } })
  const view = render(<QueryClientProvider client={client}><MemoryRouter initialEntries={[path]}><I18nProvider><SessionProvider><App /><Controls /></SessionProvider></I18nProvider></MemoryRouter></QueryClientProvider>)
  return { client, fetch, messages, view, release: async (id = 'ride-a') => { await act(async () => { gates.get(id)!.resolve() }) }, update: async (id = 'ride-a') => { await act(async () => { TestSocket.sockets.at(-1)?.onmessage?.({ data: JSON.stringify({ type: 'message.created', ride_id: id }) }) }) } }
}

let pageTop = 0
let intoView: ReturnType<typeof vi.fn>
const originalIntoView = Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'scrollIntoView')
const scrollProperties = ['clientHeight', 'scrollHeight', 'scrollTop'] as const
const originalScrollProperties = scrollProperties.map((name) => Object.getOwnPropertyDescriptor(HTMLElement.prototype, name))
beforeEach(() => {
  pageTop = 0; TestSocket.sockets = []; setCsrfToken(); vi.stubGlobal('WebSocket', TestSocket)
  vi.mocked(window.scrollTo).mockImplementation((left: unknown, top?: number) => { pageTop = typeof left === 'object' && left !== null ? (left as ScrollToOptions).top ?? pageTop : top ?? 0 })
  // Model the native ancestor-scrolling behavior that JSDOM cannot lay out:
  // scrolling a chat descendant into view can also move the document viewport.
  intoView = vi.fn(function(this: HTMLElement) { if (this.closest('.messages')) pageTop = 900 })
  Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', { configurable: true, value: intoView })
  const offsets = new WeakMap<HTMLElement, number>()
  Object.defineProperties(HTMLElement.prototype, {
    clientHeight: { configurable: true, get(this: HTMLElement) { return this.matches('.messages') ? 200 : 0 } },
    scrollHeight: { configurable: true, get(this: HTMLElement) { return this.matches('.messages') ? Math.max(220, this.querySelectorAll('.message').length * 50) : 0 } },
    scrollTop: { configurable: true, get(this: HTMLElement) { return offsets.get(this) ?? 0 }, set(this: HTMLElement, value: number) { offsets.set(this, Math.max(0, Math.min(value, this.scrollHeight - this.clientHeight))) } },
  })
})
afterEach(() => {
  if (originalIntoView) Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', originalIntoView); else delete (HTMLElement.prototype as { scrollIntoView?: unknown }).scrollIntoView
  scrollProperties.forEach((name, index) => { const descriptor = originalScrollProperties[index]; if (descriptor) Object.defineProperty(HTMLElement.prototype, name, descriptor); else Reflect.deleteProperty(HTMLElement.prototype, name) })
  vi.unstubAllGlobals(); setCsrfToken()
})

function viewport() {
  return document.querySelector<HTMLDivElement>('.messages')!
}

describe('ride room scroll ownership', () => {
  it('keeps normal route entry at the top when chat mounts and its delayed history arrives', async () => {
    const f = fixture('/')
    await screen.findByRole('heading', { name: 'The road is open.' })
    pageTop = 500
    await userEvent.click(screen.getByRole('button', { name: 'Open ride A' }))
    await screen.findByRole('heading', { name: 'Ride A plan' })
    expect(pageTop).toBe(0)
    const chat = viewport()
    await f.release()
    await screen.findByText('ride-a message 20')
    await waitFor(() => expect(chat.scrollTop).toBe(800))
    expect(pageTop).toBe(0)
    expect(intoView.mock.instances.some((element) => element instanceof HTMLElement && !!element.closest('.messages'))).toBe(false)
  })

  it('leaves the document position alone when first history resolves after the reader moves', async () => {
    const f = fixture()
    await screen.findByRole('heading', { name: 'Ride A plan' })
    viewport(); pageTop = 175
    await f.release()
    await screen.findByText('ride-a message 20')
    expect(pageTop).toBe(175)
  })

  it('follows realtime messages only within chat while the reader stays near its bottom', async () => {
    const f = fixture()
    await screen.findByRole('heading', { name: 'Ride A plan' })
    const chat = viewport(); await f.release(); await screen.findByText('ride-a message 20')
    await waitFor(() => expect(chat.scrollTop).toBe(800))
    chat.scrollTop = 780; fireEvent.scroll(chat); pageTop = 175
    f.messages.set('ride-a', history('ride-a', 21)); await f.update(); await screen.findByText('ride-a message 21')
    await waitFor(() => expect(chat.scrollTop).toBe(850))
    expect(pageTop).toBe(175)
  })

  it('preserves the position of a reader inspecting older messages and resumes following after they return', async () => {
    const f = fixture()
    await screen.findByRole('heading', { name: 'Ride A plan' })
    const chat = viewport(); await f.release(); await screen.findByText('ride-a message 20')
    chat.scrollTop = 100; fireEvent.scroll(chat); pageTop = 175
    f.messages.set('ride-a', history('ride-a', 21)); await f.update(); await screen.findByText('ride-a message 21')
    expect(chat.scrollTop).toBe(100); expect(pageTop).toBe(175)
    chat.scrollTop = 850; fireEvent.scroll(chat)
    f.messages.set('ride-a', history('ride-a', 22)); await f.update(); await screen.findByText('ride-a message 22')
    await waitFor(() => expect(chat.scrollTop).toBe(900))
    expect(pageTop).toBe(175)
  })

  it('initializes a different ride at recent chat without carrying over the old ride reader position', async () => {
    const f = fixture()
    await screen.findByRole('heading', { name: 'Ride A plan' })
    const first = viewport(); await f.release(); await screen.findByText('ride-a message 20')
    first.scrollTop = 100; fireEvent.scroll(first)
    // A cached destination can skip the loading/unmount path. Its room still
    // needs its own initial follow state even when both cursors equal twenty.
    f.client.setQueryData(['ride', 'ride-b', actor.id], { ride: ride('ride-b') })
    f.client.setQueryData(['messages', 'ride-b', actor.id], { messages: history('ride-b'), next_sequence: 20, more: false })
    await userEvent.click(screen.getByRole('button', { name: 'Open ride B' }))
    await screen.findByRole('heading', { name: 'Ride B plan' })
    const second = viewport(); expect(pageTop).toBe(0)
    await f.release('ride-b'); await screen.findByText('ride-b message 20')
    await waitFor(() => expect(second.scrollTop).toBe(800))
    expect(pageTop).toBe(0)
  })

  it('does not override Back navigation when a cached room initializes again', async () => {
    const f = fixture()
    await screen.findByRole('heading', { name: 'Ride A plan' }); await f.release(); await screen.findByText('ride-a message 20')
    await userEvent.click(screen.getByRole('button', { name: 'Open ride B' })); await screen.findByRole('heading', { name: 'Ride B plan' })
    await f.release('ride-b'); await screen.findByText('ride-b message 20')
    pageTop = 350
    await userEvent.click(screen.getByRole('button', { name: 'Go back' })); await screen.findByRole('heading', { name: 'Ride A plan' })
    expect(pageTop).toBe(350)
  })
})
