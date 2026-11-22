import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, useLocation } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { App } from '../App'
import { setCsrfToken } from '../api'
import { I18nProvider } from '../i18n'
import { SessionProvider } from '../session'
import type { Ride, User } from '../types'

vi.mock('virtual:pwa-register/react', () => ({ useRegisterSW: () => ({ needRefresh: [false, vi.fn()], updateServiceWorker: vi.fn() }) }))

const rider: User = { id: 'plan-rider', profile: { display_name: 'Plan rider', city: 'Almaty', languages: ['en'], interests: '', locale: 'en' }, onboarding_complete: true }
// A single DNS label may be 63 characters long, wider than the plan column on a phone.
const routeUrl = `https://${'a'.repeat(63)}.example.org/routes/charyn`
const ride: Ride = { id: 'plan-ride', city: 'Almaty', date: '2026-12-01', description: 'Plan fixture', status: 'published', title: 'Charyn loop', meeting_point: 'Medeu parking', destination: 'Charyn canyon', starts_at: '2026-12-01T04:00:00Z', timezone: 'Asia/Almaty', route_url: routeUrl, expectations: '', capacity: 8, plan_version: 1, organizer_id: 'organizer', participation_state: 'approved' }
const response = (value: unknown) => new Response(JSON.stringify(value), { status: 200, headers: { 'Content-Type': 'application/json' } })
const update = { id: 'announcement', sequence: 1, owner_id: 'organizer', owner_name: 'Ride organizer', body: 'Meet by the east entrance', created_at: '2026-11-28T08:15:00Z', client_announcement_id: 'announcement-client' }
let viewer: User | null
let currentRide: Ride
let announcementReads: number

class TestSocket { onopen: (() => void) | null = null; onclose: (() => void) | null = null; onmessage: ((event: { data: string }) => void) | null = null; close() {} }
beforeEach(() => {
  localStorage.setItem('bikerskz.locale', 'en')
  viewer = rider
  currentRide = ride
  announcementReads = 0
  setCsrfToken(); vi.stubGlobal('WebSocket', TestSocket)
  vi.stubGlobal('fetch', vi.fn(async (raw: string) => {
    const { pathname } = new URL(raw, 'https://bikers.kz')
    if (pathname === '/api/v1/session') return response({ user: viewer, csrf_token: 'plan-csrf' })
    if (pathname === `/api/v1/rides/${ride.id}`) return response({ ride: currentRide })
    if (pathname === '/api/v1/rides') return response({ rides: [], next_cursor: '' })
    if (pathname.endsWith('/participants')) return response({ participants: [] })
    if (pathname.endsWith('/messages')) return response({ messages: [], next_sequence: 0 })
    if (pathname.endsWith('/announcements')) { announcementReads++; return response({ announcements: [update], next_cursor: '' }) }
    if (pathname.endsWith('/reminders')) return response({ day_before: false, hour_before: false })
    if (pathname.endsWith('/album')) return response({ photos: [], videos: [], version: 1 })
    if (pathname.endsWith('/geo/config')) return response({ enabled: false, archive_url: '' })
    if (pathname.endsWith('/cities')) return response({ cities: [] })
    if (pathname.endsWith('/me/trust')) return response({ is_admin: false })
    if (pathname.endsWith('/me/blocks')) return response({ blocks: [], next_cursor: '' })
    if (pathname.endsWith('/unread-count')) return response({ unread_count: 0 })
    if (pathname.includes('/avatar')) return response({ image: null, version: 1 })
    if (pathname.endsWith('/badge')) return response({ badge: null })
    throw new Error(`Unexpected plan fixture request ${raw}`)
  }))
})
afterEach(() => { vi.unstubAllGlobals(); setCsrfToken() })

function mount() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } } })
  render(<QueryClientProvider client={client}><MemoryRouter initialEntries={[`/rides/${ride.id}`]}><I18nProvider><SessionProvider><App /><Location /></SessionProvider></I18nProvider></MemoryRouter></QueryClientProvider>)
}

function Location() {
  const { pathname, search } = useLocation()
  return <output data-testid="location">{pathname}{search}</output>
}

describe('ride plan', () => {
  it.each([
    ['en', 'Route', 'Open organizer’s route'],
    ['ru', 'Маршрут', 'Открыть маршрут организатора'],
    ['kk', 'Бағыт', 'Ұйымдастырушының бағытын ашу'],
  ])('names the organizer’s route link in words instead of its host name (%s)', async (locale, label, name) => {
    localStorage.setItem('bikerskz.locale', locale)
    mount()
    const plan = await screen.findByRole('region', { name: { en: 'The plan', ru: 'План', kk: 'Жоспар' }[locale] })
    expect(within(plan).getByText(label)).toBeInTheDocument()
    expect(within(plan).getByRole('link', { name })).toHaveAttribute('href', routeUrl)
    expect(plan).not.toHaveTextContent('a'.repeat(63))
  })

  it('puts directions and the timestamped latest update before the description in the approved room', async () => {
    currentRide = { ...ride, map_plan: { points: [
      { label: 'Medeu parking', lat: 43.16, lon: 77.06 }, { label: 'Fuel stop', lat: 43.58, lon: 77.43 }, { label: 'Charyn canyon', lat: 43.35, lon: 79.07 },
    ], distance_meters: 255000, duration_seconds: 15000 } }
    mount()

    const meetup = await screen.findByRole('link', { name: 'Navigate to meeting point' })
    expect(meetup).toHaveAttribute('href', 'https://www.google.com/maps/dir/?api=1&destination=43.16%2C77.06')
    expect(screen.getByText('Route stops (1)')).toBeInTheDocument()
    const latest = (await screen.findByRole('heading', { name: 'Latest organizer update' })).closest('article')!
    expect(latest).toHaveTextContent(update.body)
    expect(latest.querySelector('time')).toHaveAttribute('datetime', update.created_at)
    expect(latest.compareDocumentPosition(screen.getByText(ride.description))).toBe(Node.DOCUMENT_POSITION_FOLLOWING)
    expect(announcementReads).toBe(1)
  })

  it.each(['requested', 'removed'] as const)('keeps the meetup and update private when access is %s', async (state) => {
    currentRide = { ...ride, participation_state: state }
    mount()

    expect(await screen.findByRole('heading', { name: 'A ride in Almaty' })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Navigate to meeting point' })).not.toBeInTheDocument()
    expect(screen.queryByText(ride.meeting_point!)).not.toBeInTheDocument()
    expect(screen.queryByText(update.body)).not.toBeInTheDocument()
    expect(announcementReads).toBe(0)
  })

  it('keeps the cancelled plan readable without inviting riders to navigate to the meetup', async () => {
    currentRide = { ...ride, status: 'cancelled' }
    mount()

    expect(await screen.findByText(ride.meeting_point!)).toBeInTheDocument()
    expect(await screen.findByText(update.body)).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Navigate to meeting point' })).not.toBeInTheDocument()
  })
})

describe('ride sharing', () => {
  it('shares only the public URL through apps, Telegram and the clipboard, and returns guests to the ride after login', async () => {
    viewer = null
    const share = vi.fn().mockResolvedValue(undefined)
    const writeText = vi.fn().mockResolvedValue(undefined)
    vi.stubGlobal('navigator', { onLine: true, share, clipboard: { writeText } })
    mount()

    const summary = await screen.findByText('Share ride')
    fireEvent.click(summary)
    const url = `${window.location.origin}/rides/${ride.id}`
    const telegram = new URL(screen.getByRole('link', { name: 'Share to Telegram' }).getAttribute('href')!)
    expect(telegram.origin + telegram.pathname).toBe('https://t.me/share/url')
    expect([...telegram.searchParams]).toEqual([['url', url]])
    fireEvent.click(screen.getByRole('button', { name: 'Share to an app' }))
    await waitFor(() => expect(share).toHaveBeenCalledWith({ url }))
    fireEvent.click(screen.getByRole('button', { name: 'Copy link' }))
    await waitFor(() => expect(writeText).toHaveBeenCalledWith(url))

    expect(screen.queryByText(ride.meeting_point!)).not.toBeInTheDocument()
    expect(announcementReads).toBe(0)
    fireEvent.click(screen.getByRole('button', { name: 'Sign in to request a place' }))
    expect(screen.getByTestId('location')).toHaveTextContent(`/login?next=${encodeURIComponent(`/rides/${ride.id}`)}`)
  })

  it('keeps Telegram and copy available in a browser without system sharing', async () => {
    viewer = null
    mount()

    fireEvent.click(await screen.findByText('Share ride'))
    expect(screen.getByRole('link', { name: 'Share to Telegram' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Copy link' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Share to an app' })).not.toBeInTheDocument()
  })
})
