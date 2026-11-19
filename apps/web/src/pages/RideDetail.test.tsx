import { render, screen, within } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
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

class TestSocket { onopen: (() => void) | null = null; onclose: (() => void) | null = null; onmessage: ((event: { data: string }) => void) | null = null; close() {} }
beforeEach(() => {
  setCsrfToken(); vi.stubGlobal('WebSocket', TestSocket)
  vi.stubGlobal('fetch', vi.fn(async (raw: string) => {
    const { pathname } = new URL(raw, 'https://bikers.kz')
    if (pathname === '/api/v1/session') return response({ user: rider, csrf_token: 'plan-csrf' })
    if (pathname === `/api/v1/rides/${ride.id}`) return response({ ride })
    if (pathname === '/api/v1/rides') return response({ rides: [], next_cursor: '' })
    if (pathname.endsWith('/participants')) return response({ participants: [] })
    if (pathname.endsWith('/messages')) return response({ messages: [], next_sequence: 0 })
    if (pathname.endsWith('/announcements')) return response({ announcements: [], next_cursor: '' })
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
  render(<QueryClientProvider client={client}><MemoryRouter initialEntries={[`/rides/${ride.id}`]}><I18nProvider><SessionProvider><App /></SessionProvider></I18nProvider></MemoryRouter></QueryClientProvider>)
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
})
