import type { ReactNode } from 'react'
import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { requestCalendar, setCsrfToken } from '../api'
import { I18nProvider } from '../i18n'
import { SessionProvider, useSession } from '../session'
import type { PushCategories, Ride, User } from '../types'
import { PushCategorySettings, safePushCategories } from './PushCategorySettings'
import { NotificationBadge, useUnreadNotifications } from './NotificationBadge'
import { RideSchedule, safeRideReminders } from './RideSchedule'
import { InboxPage } from '../pages/Inbox'

const actor: User = { id: '00000000-0000-4000-8000-000000000001', profile: { display_name: 'Rider', city: 'Almaty', languages: ['en'], locale: 'en', interests: '' }, onboarding_complete: true }
const other: User = { ...actor, id: '00000000-0000-4000-8000-000000000009' }
const ride: Ride = { id: '00000000-0000-4000-8000-000000000002', city: 'Almaty', date: '2050-12-01', description: 'Public description', status: 'published', starts_at: '2050-12-01T05:00:00Z', timezone: 'Asia/Almaty', title: 'Private title', participation_state: 'approved' }
const categories: PushCategories = { ride_updates: true, participation: true, club_membership: true, ride_reminders: true }
const response = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json' } })
const calendar = () => new Response('BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n', { headers: { 'Content-Type': 'text/calendar; charset=utf-8' } })
function session(url: string) { return url.endsWith('/session') ? response({ user: actor, csrf_token: 'csrf-notifications' }) : undefined }
function Ready({ children }: { children: ReactNode }) { return useSession().user ? children : null }
function mount(children: ReactNode, setup?: (client: QueryClient) => void) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
  setup?.(client)
  render(<QueryClientProvider client={client}><MemoryRouter><I18nProvider><SessionProvider><Ready>{children}</Ready></SessionProvider></I18nProvider></MemoryRouter></QueryClientProvider>)
  return client
}
function Unread() {
  const { user } = useSession()
  const query = useUnreadNotifications()
  return <div>{user && <NotificationBadge count={query.isError ? 0 : query.data ?? 0} />}</div>
}

beforeEach(() => {
  Object.defineProperty(URL, 'createObjectURL', { configurable: true, writable: true, value: vi.fn(() => 'blob:private-calendar') })
  Object.defineProperty(URL, 'revokeObjectURL', { configurable: true, writable: true, value: vi.fn() })
})
afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks(); setCsrfToken() })

describe('notification categories and unread count', () => {
  it('validates every category and reminder flag rather than accepting partial or truthy data', () => {
    expect(safePushCategories(categories)).toEqual(categories)
    expect(() => safePushCategories({ ...categories, ride_reminders: 'true' })).toThrow()
    expect(() => safePushCategories({ participation: false })).toThrow()
    expect(safeRideReminders({ day_before: false, hour_before: true })).toEqual({ day_before: false, hour_before: true })
    expect(() => safeRideReminders({ day_before: 0, hour_before: true })).toThrow()
  })

  it('saves only push category choices with CSRF while explaining that the inbox remains complete', async () => {
    const fetch = vi.fn(async (url: string, options?: RequestInit) => session(url) ?? response({ push_categories: JSON.parse(String(options?.body)).push_categories }))
    vi.stubGlobal('fetch', fetch)
    mount(<PushCategorySettings categories={categories} />)
    await screen.findByRole('heading', { name: 'Choose your push notifications' })
    expect(screen.getByText(/Every update remains available in your inbox/)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('checkbox', { name: 'Club membership requests and decisions' }))
    await userEvent.click(screen.getByRole('button', { name: 'Save notification choices' }))
    await screen.findByText('Changes saved.')
    const save = fetch.mock.calls.find(([, options]) => options?.method === 'PATCH')
    expect(save?.[0]).toBe('/api/v1/me/preferences')
    expect(JSON.parse(String(save?.[1]?.body))).toEqual({ push_categories: { ...categories, club_membership: false } })
    expect(new Headers(save?.[1]?.headers).get('X-CSRF-Token')).toBe('csrf-notifications')
  })

  it('caps the visual badge and retains a localized full unread count for assistive technology', async () => {
    localStorage.setItem('bikerskz.locale', 'ru')
    vi.stubGlobal('fetch', vi.fn(async (url: string) => session(url) ?? response({ unread_count: 123 })))
    mount(<Unread />)
    expect(await screen.findByText('99+')).toHaveAttribute('aria-label', 'Непрочитанных: 123')
  })


  it('refreshes the global unread count immediately after marking the inbox read', async () => {
    let read = false
    vi.stubGlobal('fetch', vi.fn(async (url: string, options?: RequestInit) => {
      const auth = session(url); if (auth) return auth
      if (url.endsWith('/read') && options?.method === 'PUT') { read = true; return response({}) }
      if (url.endsWith('/unread-count')) return response({ unread_count: read ? 0 : 1 })
      return response({ notifications: [{ id: '00000000-0000-4000-8000-000000000003', kind: 'ride.reminder', ride_id: ride.id, created_at: '2026-11-04T10:00:00Z', read_at: read ? '2026-11-04T10:00:01Z' : null }], next_cursor: '', unread_count: read ? 0 : 1 })
    }))
    mount(<><Unread /><InboxPage /></>)
    await screen.findByRole('heading', { name: 'Your ride departs soon' })
    expect(await screen.findByText('1')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Open ride' })).toHaveAttribute('href', `/rides/${ride.id}`)
    await userEvent.click(screen.getByRole('button', { name: 'Mark all as read' }))
    await screen.findByText('0 unread updates')
    await waitFor(() => expect(screen.queryByText('1')).not.toBeInTheDocument())
  })


  it('hides a cached unread badge while the current session cannot be verified', async () => {
    let failed = false
    vi.stubGlobal('fetch', vi.fn(async (url: string) => {
      if (url.endsWith('/session')) { if (failed) throw new Error('Session unavailable'); return session(url)! }
      return response({ unread_count: 4 })
    }))
    const client = mount(<Unread />)
    await screen.findByText('4')
    failed = true
    await act(async () => { await client.refetchQueries({ queryKey: ['session'] }) })
    await waitFor(() => expect(screen.queryByText('4')).not.toBeInTheDocument())
  })

  it('cancels an old account count request and never shows its late unread count for the next rider', async () => {
    let finish: ((value: Response) => void) | undefined
    let oldSignal: AbortSignal | null | undefined
    let countReads = 0
    vi.stubGlobal('fetch', vi.fn(async (url: string, options?: RequestInit) => {
      const auth = session(url); if (auth) return auth
      if (++countReads === 1) { oldSignal = options?.signal; return new Promise<Response>((resolve) => { finish = resolve }) }
      return response({ unread_count: 2 })
    }))
    const client = mount(<Unread />)
    await waitFor(() => expect(finish).toBeTypeOf('function'))
    await act(async () => { setCsrfToken('next-session'); client.setQueryData(['session'], { user: other, csrf_token: 'next-session' }) })
    await screen.findByText('2')
    expect(oldSignal?.aborted).toBe(true)
    await act(async () => { finish!(response({ unread_count: 72 })) })
    expect(screen.queryByText('72')).not.toBeInTheDocument()
    expect(screen.getByText('2')).toBeInTheDocument()
  })
})

describe('ride reminder choices and calendar downloads', () => {
  it.each([
    ['en', '24 hours before departure', '1 hour before departure', 'Save reminders', 'Add to calendar'],
    ['ru', 'За 24 часа до отправления', 'За 1 час до отправления', 'Сохранить напоминания', 'Добавить в календарь'],
    ['kk', 'Жолға шығудан 24 сағат бұрын', 'Жолға шығудан 1 сағат бұрын', 'Еске салғыштарды сақтау', 'Күнтізбеге қосу'],
  ])('allows explicit ride reminder choices and an authenticated no-store calendar export in %s', async (locale, day, hour, saveLabel, calendarLabel) => {
    localStorage.setItem('bikerskz.locale', locale)
    const fetch = vi.fn(async (url: string, options?: RequestInit) => {
      const auth = session(url); if (auth) return auth
      if (url.endsWith('/calendar')) return calendar()
      if (options?.method === 'PUT') return response(JSON.parse(String(options.body)))
      return response({ day_before: false, hour_before: false })
    })
    vi.stubGlobal('fetch', fetch)
    const clicked = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
    mount(<RideSchedule ride={ride} onAccessLost={vi.fn()} />)
    expect(await screen.findByRole('checkbox', { name: day })).not.toBeChecked()
    expect(screen.getByRole('checkbox', { name: hour })).not.toBeChecked()
    await userEvent.click(screen.getByRole('checkbox', { name: hour }))
    await userEvent.click(screen.getByRole('button', { name: saveLabel }))
    await waitFor(() => expect(fetch.mock.calls.some(([, options]) => options?.method === 'PUT')).toBe(true))
    const put = fetch.mock.calls.find(([, options]) => options?.method === 'PUT')
    expect(JSON.parse(String(put?.[1]?.body))).toEqual({ day_before: false, hour_before: true })
    expect(new Headers(put?.[1]?.headers).get('X-CSRF-Token')).toBe('csrf-notifications')
    await userEvent.click(screen.getByRole('button', { name: calendarLabel }))
    await waitFor(() => expect(clicked).toHaveBeenCalledOnce())
    const get = fetch.mock.calls.find(([url]) => url.endsWith('/calendar'))
    expect(get?.[1]).toMatchObject({ credentials: 'same-origin', cache: 'no-store' })
    expect(URL.createObjectURL).toHaveBeenCalledOnce()
    await waitFor(() => expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:private-calendar'))
  })


  it('accepts a refreshed consent reset while retaining deliberate unsaved choices during later refreshes', async () => {
    let finish: ((value: Response) => void) | undefined
    let reads = 0
    vi.stubGlobal('fetch', vi.fn(async (url: string) => {
      if (!url.endsWith('/reminders')) throw new Error(url)
      if (++reads === 1) return new Promise<Response>((resolve) => { finish = resolve })
      return response({ day_before: false, hour_before: false })
    }))
    const key = ['ride-reminders', ride.id, actor.id]
    const client = mount(<RideSchedule ride={ride} onAccessLost={vi.fn()} />, (cache) => {
      setCsrfToken('csrf-notifications')
      cache.setQueryData(['session'], { user: actor, csrf_token: 'csrf-notifications' })
      cache.setQueryDefaults(key, { gcTime: Infinity, meta: { private: true } })
      cache.setQueryData(key, { day_before: true, hour_before: true })
    })
    const hour = await screen.findByRole('checkbox', { name: '1 hour before departure' })
    expect(hour).toBeChecked()
    await waitFor(() => expect(finish).toBeTypeOf('function'))
    await act(async () => { finish!(response({ day_before: false, hour_before: false })) })
    await waitFor(() => expect(hour).not.toBeChecked())
    expect(screen.getByRole('checkbox', { name: '24 hours before departure' })).not.toBeChecked()
    await userEvent.click(hour)
    expect(hour).toBeChecked()
    await act(async () => { await client.refetchQueries({ queryKey: key }) })
    expect(hour).toBeChecked()
    expect(client.getQueryData(key)).toEqual({ day_before: false, hour_before: false })
  })

  it('does not download a calendar that arrives after the user switches accounts', async () => {
    let finish: ((value: Response) => void) | undefined
    let downloadSignal: AbortSignal | null | undefined
    vi.stubGlobal('fetch', vi.fn(async (url: string, options?: RequestInit) => {
      const auth = session(url); if (auth) return auth
      if (url.endsWith('/calendar')) { downloadSignal = options?.signal; return new Promise<Response>((resolve) => { finish = resolve }) }
      return response({ day_before: false, hour_before: false })
    }))
    const clicked = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
    const client = mount(<RideSchedule ride={ride} onAccessLost={vi.fn()} />)
    await userEvent.click(await screen.findByRole('button', { name: 'Add to calendar' }))
    await waitFor(() => expect(finish).toBeTypeOf('function'))
    await act(async () => { setCsrfToken('next-session'); client.setQueryData(['session'], { user: other, csrf_token: 'next-session' }) })
    await waitFor(() => expect(downloadSignal?.aborted).toBe(true))
    await act(async () => { finish!(calendar()) })
    expect(clicked).not.toHaveBeenCalled()
    expect(URL.createObjectURL).not.toHaveBeenCalled()
  })

  it('keeps calendar export for cancelled plans while hiding reminder choices', async () => {
    const fetch = vi.fn(async (url: string) => session(url) ?? calendar())
    vi.stubGlobal('fetch', fetch)
    mount(<RideSchedule ride={{ ...ride, status: 'cancelled' }} onAccessLost={vi.fn()} />)
    await screen.findByText('Reminders are available for upcoming active rides.')
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Add to calendar' })).toBeEnabled()
    expect(fetch.mock.calls.some(([url]) => url.endsWith('/reminders'))).toBe(false)
  })

  it('rejects JSON served as a calendar and respects permission errors', async () => {
    const controller = new AbortController()
    vi.stubGlobal('fetch', vi.fn(async () => response({ error: 'not a calendar' })))
    await expect(requestCalendar(ride.id, controller.signal)).rejects.toMatchObject({ status: 502 })
    vi.stubGlobal('fetch', vi.fn(async () => response({ error: { code: 'forbidden' } }, 403)))
    await expect(requestCalendar(ride.id, controller.signal)).rejects.toMatchObject({ status: 403, code: 'forbidden' })
  })
})
