import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider, onlineManager, useQuery } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { request, setCsrfToken } from '../api'
import { I18nProvider } from '../i18n'
import { SessionProvider, useSession } from '../session'
import type { Participant, Ride, User } from '../types'
import { ParticipantsPanel } from './ParticipantsPanel'

const actor: User = { id: '00000000-0000-4000-8000-000000000001', profile: { display_name: 'Organizer', city: 'Almaty', languages: ['en'], locale: 'en', interests: '' }, onboarding_complete: true }
const target = '00000000-0000-4000-8000-000000000002', applicant = '00000000-0000-4000-8000-000000000003', rideId = '00000000-0000-4000-8000-000000000004', imageId = '00000000-0000-4000-8000-000000000005'
const ride: Ride = { id: rideId, title: 'Ride', city: 'Almaty', date: '2026-12-01', description: '', status: 'published', organizer_id: actor.id, can_manage: true, participation_state: 'approved', capacity: 10 }
const initialPeople: Participant[] = [{ user_id: actor.id, display_name: 'Organizer', state: 'approved' }, { user_id: target, display_name: 'Approved Rider', state: 'approved' }, { user_id: applicant, display_name: 'Pending Rider', state: 'requested' }]
const response = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json' } })
function Harness({ onAccessLost }: { onAccessLost: () => void }) {
  const { user } = useSession()
  const query = useQuery({ queryKey: ['ride', rideId, user?.id], queryFn: ({ signal }) => request<{ ride: Ride }>('/rides/' + rideId, { signal }), enabled: !!user, staleTime: Infinity })
  return query.data ? <ParticipantsPanel ride={query.data.ride} onAccessLost={onAccessLost} /> : null
}
function mount({ value = ride, user = actor, onAccessLost = vi.fn() }: { value?: Ride; user?: User; onAccessLost?: () => void } = {}) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
  client.setQueryData(['ride', rideId, user.id], { ride: value })
  render(<QueryClientProvider client={client}><MemoryRouter><I18nProvider><SessionProvider><Harness onAccessLost={onAccessLost} /></SessionProvider></I18nProvider></MemoryRouter></QueryClientProvider>)
  return { client, onAccessLost }
}
function reads(url: string, people = initialPeople, value = ride, user = actor, admin = false) {
  if (url.endsWith('/session')) return response({ user, csrf_token: 'csrf-test' })
  if (url.endsWith('/rides/' + rideId)) return response({ ride: value })
  if (url.endsWith('/participants')) return response({ participants: people })
  if (url.endsWith('/me/trust')) return response({ is_admin: admin })
  if (url.endsWith('/me/blocks')) return response({ blocks: [], next_cursor: '' })
  if (url.endsWith('/badge')) return response({ badge: null })
  if (url.includes('/avatars/')) return response({ image: { id: imageId, url: '/api/v1/rides/' + rideId + '/avatars/' + url.split('/').at(-1) + '/' + imageId + '/content', width: 200, height: 200, created_at: '2026-11-09T10:00:00Z' } })
  throw new Error('Unexpected request ' + url)
}
async function open(name: string) {
  const trigger = await screen.findByRole('button', { name: 'Rider actions for ' + name })
  await userEvent.click(trigger)
  return { trigger, menu: await screen.findByRole('menu', { name: 'Rider actions for ' + name }) }
}
beforeEach(() => {
  onlineManager.setOnline(true)
  setCsrfToken()
  vi.stubGlobal('PointerEvent', MouseEvent)
  vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} })
  if (!HTMLElement.prototype.scrollIntoView) HTMLElement.prototype.scrollIntoView = vi.fn()
})
afterEach(() => { onlineManager.setOnline(true); setCsrfToken(); vi.unstubAllGlobals() })

describe('participant rider menus', () => {
  it('keeps identities, states and counts visible while pending rows expose only existing decisions', async () => {
    const fetch = vi.fn(async (url: string) => reads(url))
    vi.stubGlobal('fetch', fetch)
    mount()
    await screen.findByText('Approved Rider')
    expect(screen.getByText('Pending Rider')).toBeInTheDocument()
    expect(screen.getByText('Organizer', { selector: 'strong' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Rider actions for Organizer' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Remove Approved Rider' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Report rider' })).not.toBeInTheDocument()
    const { menu } = await open('Pending Rider')
    expect(within(menu).getAllByRole('menuitem').map((item) => item.textContent)).toEqual(['Approve', 'Decline'])
    expect(fetch.mock.calls.some(([url]) => url.includes('/avatars/' + applicant))).toBe(false)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })
  it.each([['Approve', 'approve', 'approved'], ['Decline', 'reject', 'rejected']])('sends %s with the actual applicant and CSRF after menu dismissal', async (label, decision, state) => {
    let people = initialPeople.map((p) => ({ ...p })), body: unknown, csrf: string | null = null
    vi.stubGlobal('fetch', vi.fn(async (url: string, options?: RequestInit) => {
      if (url.endsWith('/decision')) {
        expect(url).toBe('/api/v1/rides/' + rideId + '/participants/' + applicant + '/decision')
        body = JSON.parse(String(options?.body)); csrf = new Headers(options?.headers).get('X-CSRF-Token')
        people = people.map((p) => p.user_id === applicant ? { ...p, state } : p)
        return response({})
      }
      return reads(url, people)
    }))
    mount()
    const { menu } = await open('Pending Rider')
    await userEvent.click(within(menu).getByRole('menuitem', { name: label }))
    await waitFor(() => expect(body).toEqual({ decision }))
    expect(csrf).toBe('csrf-test')
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
    await waitFor(() => expect(screen.queryByText('Pending Rider')?.closest('.request-participant')).toBeFalsy())
  })
  it('keeps removal confirmation outside the menu, restores cancel focus and focuses the roster after removal', async () => {
    let people = initialPeople.map((p) => ({ ...p })), finish: (value: Response) => void = () => {}, signal: AbortSignal | undefined, writes = 0
    vi.stubGlobal('fetch', vi.fn(async (url: string, options?: RequestInit) => {
      if (url.endsWith('/decision')) { writes++; signal = options?.signal as AbortSignal; return new Promise<Response>((resolve) => { finish = resolve }) }
      return reads(url, people)
    }))
    mount()
    const { trigger, menu } = await open('Approved Rider')
    await userEvent.click(within(menu).getByRole('menuitem', { name: 'Remove' }))
    const cancel = await screen.findByRole('dialog', { name: 'Remove' })
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
    expect(within(cancel).getByRole('button', { name: 'Cancel' })).toHaveFocus()
    await userEvent.click(within(cancel).getByRole('button', { name: 'Cancel' }))
    expect(trigger).toHaveFocus(); expect(writes).toBe(0)
    await userEvent.click(trigger)
    await userEvent.click(await screen.findByRole('menuitem', { name: 'Remove' }))
    await userEvent.click(within(await screen.findByRole('dialog', { name: 'Remove' })).getByRole('button', { name: 'Remove' }))
    await waitFor(() => expect(writes).toBe(1))
    expect(signal?.aborted).toBe(false)
    people = people.filter((p) => p.user_id !== target)
    await act(async () => finish(response({})))
    await waitFor(() => expect(screen.queryByText('Approved Rider')).not.toBeInTheDocument())
    expect(screen.getByRole('heading', { name: 'Participants' })).toHaveFocus()
  })
  it('does not grant participant decisions to an ordinary rider even with an administrator badge', async () => {
    const viewer: User = { ...actor, id: applicant }
    const value = { ...ride, can_manage: false }
    vi.stubGlobal('fetch', vi.fn(async (url: string) => reads(url, initialPeople, value, viewer, true)))
    mount({ value, user: viewer })
    const { menu } = await open('Organizer')
    expect(within(menu).queryByRole('menuitem', { name: 'Remove' })).not.toBeInTheDocument()
    expect(within(menu).queryByRole('menuitem', { name: 'Approve' })).not.toBeInTheDocument()
    expect(await within(menu).findByRole('menuitem', { name: 'Manage rider' })).toBeInTheDocument()
    expect(screen.queryByText('Pending Rider')).not.toBeInTheDocument()
  })
  it('preserves cancelled-ride behavior: pending decisions disabled, approved removal still available', async () => {
    const value = { ...ride, status: 'cancelled' }
    vi.stubGlobal('fetch', vi.fn(async (url: string) => reads(url, initialPeople, value)))
    mount({ value })
    const pending = await open('Pending Rider')
    expect(within(pending.menu).getByRole('menuitem', { name: 'Approve' })).toHaveAttribute('aria-disabled', 'true')
    expect(within(pending.menu).getByRole('menuitem', { name: 'Decline' })).toHaveAttribute('aria-disabled', 'true')
    await userEvent.keyboard('{Escape}')
    const approved = await open('Approved Rider')
    expect(within(approved.menu).getByRole('menuitem', { name: 'Remove' })).not.toHaveAttribute('aria-disabled')
  })
  it('disables decisions offline and while another participant decision is working', async () => {
    let finish: (value: Response) => void = () => {}
    vi.stubGlobal('fetch', vi.fn(async (url: string) => url.endsWith('/decision') ? new Promise<Response>((resolve) => { finish = resolve }) : reads(url)))
    mount()
    const pending = await open('Pending Rider')
    await userEvent.click(within(pending.menu).getByRole('menuitem', { name: 'Approve' }))
    const approved = await open('Approved Rider')
    expect(within(approved.menu).getByRole('menuitem', { name: 'Remove' })).toHaveAttribute('aria-disabled', 'true')
    expect(within(approved.menu).getByRole('menuitem', { name: 'Report rider' })).not.toHaveAttribute('aria-disabled')
    await userEvent.keyboard('{Escape}')
    await act(async () => finish(response({})))
    vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(false)
    fireEvent(window, new Event('offline'))
    const offline = await open('Approved Rider')
    expect(within(offline.menu).getByRole('menuitem', { name: 'Remove' })).toHaveAttribute('aria-disabled', 'true')
  })
})

describe('participant decision session and state boundaries', () => {
  it('checks organizer permission again after removal confirmation before any write', async () => {
    let value = { ...ride }, writes = 0
    vi.stubGlobal('fetch', vi.fn(async (url: string) => { if (url.endsWith('/decision')) writes++; return reads(url, initialPeople, value) }))
    const { client, onAccessLost } = mount()
    const { menu } = await open('Approved Rider')
    await userEvent.click(within(menu).getByRole('menuitem', { name: 'Remove' }))
    const dialog = await screen.findByRole('dialog', { name: 'Remove' })
    value = { ...ride, can_manage: false }
    await act(async () => { client.setQueryData(['ride', rideId, actor.id], { ride: value }) })
    await userEvent.click(within(dialog).getByRole('button', { name: 'Remove' }))
    await waitFor(() => expect(onAccessLost).toHaveBeenCalled())
    expect(writes).toBe(0)
  })
  it('refreshes a changed roster instead of removing a stale target', async () => {
    let people = initialPeople.map((p) => ({ ...p })), writes = 0, rosterReads = 0
    vi.stubGlobal('fetch', vi.fn(async (url: string) => { if (url.endsWith('/decision')) writes++; if (url.endsWith('/participants')) rosterReads++; return reads(url, people) }))
    const { client } = mount()
    const { menu } = await open('Approved Rider')
    await userEvent.click(within(menu).getByRole('menuitem', { name: 'Remove' }))
    const dialog = await screen.findByRole('dialog', { name: 'Remove' })
    people = people.map((p) => p.user_id === target ? { ...p, state: 'left' } : p)
    await act(async () => { client.setQueryData(['participants', rideId, actor.id], { participants: people }) })
    await userEvent.click(within(dialog).getByRole('button', { name: 'Remove' }))
    await screen.findByRole('alert')
    await waitFor(() => expect(rosterReads).toBeGreaterThan(1))
    expect(writes).toBe(0)
  })
  it.each(['account', 'epoch'])('cancels removal confirmation after a %s change without a write', async (kind) => {
    let writes = 0
    vi.stubGlobal('fetch', vi.fn(async (url: string) => { if (url.endsWith('/decision')) writes++; return reads(url) }))
    const { client } = mount()
    const { menu } = await open('Approved Rider')
    await userEvent.click(within(menu).getByRole('menuitem', { name: 'Remove' }))
    await screen.findByRole('dialog', { name: 'Remove' })
    await act(async () => {
      setCsrfToken('replacement')
      client.setQueryData(['session'], { user: kind === 'account' ? { ...actor, id: imageId } : actor, csrf_token: 'replacement' })
    })
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(writes).toBe(0)
  })
  it('prevents a queued approval crossing a session handoff', async () => {
    let writes = 0
    vi.stubGlobal('fetch', vi.fn(async (url: string) => { if (url.endsWith('/decision')) writes++; return reads(url) }))
    const { client } = mount()
    const { menu } = await open('Pending Rider')
    await act(async () => {
      fireEvent.click(within(menu).getByRole('menuitem', { name: 'Approve' }))
      setCsrfToken('replacement')
      client.setQueryData(['session'], { user: actor, csrf_token: 'replacement' })
    })
    await waitFor(() => expect(screen.queryByRole('menu')).not.toBeInTheDocument())
    expect(writes).toBe(0)
  })
  it('aborts a pending request on account change without letting its late reply refresh the next account', async () => {
    let finish: (value: Response) => void = () => {}, signal: AbortSignal | undefined
    vi.stubGlobal('fetch', vi.fn(async (url: string, options?: RequestInit) => {
      if (url.endsWith('/decision')) { signal = options?.signal as AbortSignal; return new Promise<Response>((resolve) => { finish = resolve }) }
      return reads(url)
    }))
    const { client } = mount()
    const { menu } = await open('Pending Rider')
    await userEvent.click(within(menu).getByRole('menuitem', { name: 'Approve' }))
    await waitFor(() => expect(signal).toBeDefined())
    await act(async () => { setCsrfToken('replacement'); client.setQueryData(['session'], { user: { ...actor, id: imageId }, csrf_token: 'replacement' }) })
    await waitFor(() => expect(signal?.aborted).toBe(true))
    const invalidate = vi.spyOn(client, 'invalidateQueries')
    await act(async () => finish(response({})))
    expect(invalidate).not.toHaveBeenCalled()
  })
  it.each([['participation_changed', 409], ['forbidden', 403]] as const)('preserves %s feedback and the relevant roster/access refresh', async (code, status) => {
    let rosterReads = 0
    vi.stubGlobal('fetch', vi.fn(async (url: string) => {
      if (url.endsWith('/decision')) return response({ error: { code } }, status)
      if (url.endsWith('/participants')) rosterReads++
      return reads(url)
    }))
    const { onAccessLost } = mount()
    const { menu } = await open('Pending Rider')
    await userEvent.click(within(menu).getByRole('menuitem', { name: 'Approve' }))
    await screen.findByRole('alert')
    if (status === 403) expect(onAccessLost).toHaveBeenCalled()
    else await waitFor(() => expect(rosterReads).toBeGreaterThan(1))
  })
})

describe('participant safety actions and localized menus', () => {
  it.each([['Report rider', 'user', target], ['Report rider photo', 'avatar', imageId]])('reports the scoped %s target after the menu closes', async (label, type, id) => {
    let payload: unknown
    vi.stubGlobal('fetch', vi.fn(async (url: string, options?: RequestInit) => {
      if (url.endsWith('/reports')) { payload = JSON.parse(String(options?.body)); return response({}, 201) }
      return reads(url)
    }))
    mount()
    const { trigger, menu } = await open('Approved Rider')
    await userEvent.click(await within(menu).findByRole('menuitem', { name: label }))
    await screen.findByRole('combobox', { name: 'Reason' })
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
    await userEvent.type(screen.getByRole('textbox', { name: 'Details' }), 'Review this rider')
    await userEvent.click(screen.getByRole('button', { name: 'Send report' }))
    await screen.findByText('Your report was sent for review.')
    expect(payload).toMatchObject({ target_type: type, target_id: id, details: 'Review this rider' })
    expect(trigger).toHaveFocus()
  })
  it('keeps block confirmation independent of the popup and reconciles block state', async () => {
    let blocked = false, writes = 0
    vi.stubGlobal('fetch', vi.fn(async (url: string, options?: RequestInit) => {
      if (url.endsWith('/me/blocks/' + target)) { writes++; expect(options?.method).toBe('PUT'); blocked = true; return response({ blocked }) }
      if (url.endsWith('/me/blocks')) return response({ blocks: blocked ? [{ user_id: target, display_name: 'Approved Rider', created_at: '2026-11-09T00:00:00Z' }] : [], next_cursor: '' })
      return reads(url)
    }))
    mount()
    const { trigger, menu } = await open('Approved Rider')
    await userEvent.click(within(menu).getByRole('menuitem', { name: 'Block rider' }))
    const dialog = await screen.findByRole('dialog', { name: 'Block rider' })
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
    await userEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))
    expect(trigger).toHaveFocus(); expect(writes).toBe(0)
    await userEvent.click(trigger)
    await userEvent.click(await screen.findByRole('menuitem', { name: 'Block rider' }))
    await userEvent.click(within(await screen.findByRole('dialog', { name: 'Block rider' })).getByRole('button', { name: 'Block rider' }))
    await waitFor(() => expect(writes).toBe(1))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    const next = await open('Approved Rider')
    expect(await within(next.menu).findByRole('menuitem', { name: 'Unblock rider' })).toBeInTheDocument()
  })
  it.each([
    ['en', 'Rider actions for Pending Rider', 'Approve', 'Decline', 'Rider actions for Approved Rider', 'Remove', 'Participation'],
    ['ru', 'Действия с участником: Pending Rider', 'Одобрить', 'Отклонить', 'Действия с участником: Approved Rider', 'Удалить', 'Участие'],
    ['kk', 'Pending Rider қатысушысының әрекеттері', 'Мақұлдау', 'Қабылдамау', 'Approved Rider қатысушысының әрекеттері', 'Шеттету', 'Қатысу'],
  ])('localizes pending and approved rider menus in %s', async (locale, pendingLabel, approve, decline, approvedLabel, remove, group) => {
    localStorage.setItem('bikerskz.locale', locale)
    vi.stubGlobal('fetch', vi.fn(async (url: string) => reads(url)))
    mount()
    const pendingTrigger = await screen.findByRole('button', { name: pendingLabel })
    pendingTrigger.focus()
    await userEvent.keyboard('{ArrowDown}')
    const pending = await screen.findByRole('menu', { name: pendingLabel })
    expect(within(pending).getByRole('group', { name: group })).toBeInTheDocument()
    expect(within(pending).getByRole('menuitem', { name: approve })).toBeInTheDocument()
    expect(within(pending).getByRole('menuitem', { name: decline })).toBeInTheDocument()
    await userEvent.keyboard('{Escape}')
    expect(pendingTrigger).toHaveFocus()
    await userEvent.click(screen.getByRole('button', { name: approvedLabel }))
    const approved = await screen.findByRole('menu', { name: approvedLabel })
    expect(within(approved).getByRole('menuitem', { name: remove })).toBeInTheDocument()
  })
})
