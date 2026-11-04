import type { ReactNode } from 'react'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { setCsrfToken } from '../api'
import { I18nProvider } from '../i18n'
import { SessionProvider, useSession } from '../session'
import type { InboxNotification, User } from '../types'
import { InboxPage } from './Inbox'

const actor: User = { id: '00000000-0000-4000-8000-000000000001', profile: { display_name: 'Rider', city: 'Almaty', languages: ['en'], locale: 'en', interests: '' }, onboarding_complete: true }
const rideID = '00000000-0000-4000-8000-000000000002'
const clubID = '00000000-0000-4000-8000-000000000003'
const otherClubID = '00000000-0000-4000-8000-000000000004'
const created_at = '2026-11-04T12:00:00Z'
const clubKinds = ['club.membership.requested', 'club.membership.approved', 'club.membership.rejected', 'club.membership.removed']
const clubNotices = (): InboxNotification[] => clubKinds.map((kind, index) => ({ id: `00000000-0000-4000-8000-00000000001${index}`, kind, club_id: clubID, created_at, read_at: null }))
const rideNotice: InboxNotification = { id: '00000000-0000-4000-8000-000000000020', kind: 'ride.updated', ride_id: rideID, created_at, read_at: null }
const response = (value: unknown) => new Response(JSON.stringify(value), { headers: { 'Content-Type': 'application/json' } })

function Ready({ children }: { children: ReactNode }) { return useSession().user ? children : null }
function mount() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
  render(<QueryClientProvider client={client}><MemoryRouter initialEntries={['/inbox']}><I18nProvider><SessionProvider><Ready><InboxPage /></Ready></SessionProvider></I18nProvider></MemoryRouter></QueryClientProvider>)
}
function session(url: string) {
  return url.endsWith('/session') ? response({ user: actor, csrf_token: 'csrf-inbox' }) : undefined
}

afterEach(() => { vi.unstubAllGlobals(); setCsrfToken() })

describe('club and ride inbox notifications', () => {
  it.each([
    ['en', ['New club membership request', 'Your club membership request was approved', 'Your club membership request was declined', 'Your club membership ended'], 'Open club', 'Open ride', 'A ride plan changed'],
    ['ru', ['Новая заявка на вступление в клуб', 'Ваша заявка в клуб одобрена', 'Ваша заявка в клуб отклонена', 'Ваше членство в клубе прекращено'], 'Открыть клуб', 'Открыть поездку', 'План поездки изменился'],
    ['kk', ['Клубқа мүшелікке жаңа өтінім', 'Клубқа мүшелік өтініміңіз мақұлданды', 'Клубқа мүшелік өтініміңіз қабылданбады', 'Клубтағы мүшелігіңіз тоқтатылды'], 'Клубты ашу', 'Сапарды ашу', 'Сапар жоспары өзгерді'],
  ])('localizes all membership events and routes mixed notifications to the right resource in %s', async (locale, labels, openClub, openRide, rideLabel) => {
    localStorage.setItem('bikerskz.locale', locale)
    vi.stubGlobal('fetch', vi.fn(async (url: string) => {
      const auth = session(url); if (auth) return auth
      if (url.endsWith('/me/notifications')) return response({ notifications: [...clubNotices(), rideNotice], next_cursor: '', unread_count: 5 })
      throw new Error(`Unexpected request ${url}`)
    }))
    mount()
    for (const label of labels) await screen.findByRole('heading', { name: label })
    expect(screen.getByRole('heading', { name: rideLabel })).toBeInTheDocument()
    const clubLinks = screen.getAllByRole('link', { name: openClub })
    expect(clubLinks).toHaveLength(4)
    for (const link of clubLinks) expect(link).toHaveAttribute('href', `/clubs/${clubID}`)
    expect(screen.getByRole('link', { name: openRide })).toHaveAttribute('href', `/rides/${rideID}`)
    expect(document.querySelector('a[href*="undefined"]')).toBeNull()
  })

  it('marks club notices read, pages to ride notices, and marks all types read through the same CSRF-protected inbox', async () => {
    const clubs = clubNotices().slice(0, 2)
    clubs[1] = { ...clubs[1], club_id: otherClubID }
    const readIDs = new Set<string>()
    let allRead = false
    const fetch = vi.fn(async (url: string, options?: RequestInit) => {
      const auth = session(url); if (auth) return auth
      const path = new URL(url, 'https://bikers.kz').pathname
      if (path === '/api/v1/me/notifications/read' && options?.method === 'PUT') { allRead = true; return response({}) }
      if (path === `/api/v1/me/notifications/${clubs[0].id}/read` && options?.method === 'PUT') { readIDs.add(clubs[0].id); return response({}) }
      if (path === '/api/v1/me/notifications') {
        const second = new URL(url, 'https://bikers.kz').searchParams.get('before') === 'older'
        return response({ notifications: (second ? [rideNotice] : clubs).map((notice) => ({ ...notice, read_at: allRead || readIDs.has(notice.id) ? created_at : null })), next_cursor: second ? '' : 'older', unread_count: allRead ? 0 : 3 - readIDs.size })
      }
      throw new Error(`Unexpected request ${url}`)
    })
    vi.stubGlobal('fetch', fetch)
    mount()
    const heading = await screen.findByRole('heading', { name: 'New club membership request' })
    const requested = heading.closest('article')
    expect(requested).not.toBeNull()
    await userEvent.click(within(requested!).getByRole('button', { name: 'Mark as read' }))
    await screen.findByText('2 unread updates')
    expect(within(requested!).queryByRole('button', { name: 'Mark as read' })).not.toBeInTheDocument()
    expect(screen.getAllByRole('link', { name: 'Open club' }).map((link) => link.getAttribute('href'))).toEqual([`/clubs/${clubID}`, `/clubs/${otherClubID}`])

    await userEvent.click(screen.getByRole('button', { name: 'Next page' }))
    await screen.findByRole('heading', { name: 'A ride plan changed' })
    expect(screen.queryByRole('link', { name: 'Open club' })).not.toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Open ride' })).toHaveAttribute('href', `/rides/${rideID}`)
    await userEvent.click(screen.getByRole('button', { name: 'Mark all as read' }))
    await screen.findByText('0 unread updates')
    expect(screen.queryByRole('button', { name: 'Mark as read' })).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Previous page' }))
    await screen.findByRole('heading', { name: 'New club membership request' })
    await waitFor(() => expect(screen.queryAllByRole('button', { name: 'Mark as read' })).toHaveLength(0))
    for (const [, options] of fetch.mock.calls.filter(([, options]) => options?.method === 'PUT')) {
      expect(new Headers(options?.headers).get('X-CSRF-Token')).toBe('csrf-inbox')
    }
    expect(fetch.mock.calls.filter(([, options]) => options?.method === 'PUT')).toHaveLength(2)
  })

  it('does not turn unknown, missing, mixed or malformed notification targets into links', async () => {
    const notices: InboxNotification[] = [
      { ...rideNotice, id: 'unknown', kind: 'club.membership.future', club_id: clubID, ride_id: undefined },
      { ...clubNotices()[0], id: 'missing', club_id: undefined },
      { ...clubNotices()[1], id: 'mixed', ride_id: rideID },
      { ...clubNotices()[2], id: 'invalid', club_id: 'https://other.example/private' },
      { ...rideNotice, id: 'wrong-resource', club_id: clubID, ride_id: undefined },
      { ...rideNotice, id: 'prototype-kind', kind: '__proto__' },
    ]
    vi.stubGlobal('fetch', vi.fn(async (url: string) => session(url) ?? response({ notifications: notices, next_cursor: '', unread_count: notices.length })))
    mount()
    await screen.findByRole('heading', { name: 'New club membership request' })
    expect(screen.queryByRole('link', { name: 'Open club' })).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Open ride' })).not.toBeInTheDocument()
    expect(screen.getAllByRole('heading', { name: 'Inbox' })).toHaveLength(2)
    expect(screen.getAllByRole('button', { name: 'Mark as read' })).toHaveLength(notices.length)
  })
})
