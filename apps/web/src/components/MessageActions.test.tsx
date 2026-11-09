import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, useLocation } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { setCsrfToken } from '../api'
import { I18nProvider } from '../i18n'
import { SessionProvider, useSession } from '../session'
import type { Message, User } from '../types'
import { MessageActions } from './MessageActions'

const actor: User = { id: '00000000-0000-4000-8000-000000000001', profile: { display_name: 'Rider', city: 'Almaty', languages: ['en'], locale: 'en', interests: '' }, onboarding_complete: true }
const sender = '00000000-0000-4000-8000-000000000002', rideId = '00000000-0000-4000-8000-000000000003', imageId = '00000000-0000-4000-8000-000000000004'
const message: Message = { id: '00000000-0000-4000-8000-000000000005', sequence: 1, sender_id: sender, sender_name: 'Organizer', body: 'Meet by the road', created_at: '2026-11-09T10:00:00Z', client_message_id: '' }
const response = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json' } })
function Controls() {
  const session = useSession(), location = useLocation()
  return <><button onClick={() => session.accept({ user: { ...actor, id: '00000000-0000-4000-8000-000000000099' }, csrf_token: 'next-csrf' })}>Switch account</button><output data-testid="location">{location.pathname + location.search}</output><button>Outside action</button></>
}
function mount(value = message) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
  render(<QueryClientProvider client={client}><MemoryRouter><I18nProvider><SessionProvider><div className="message message-with-actions"><div className="message-content"><p>{value.body}</p></div><MessageActions message={value} rideId={rideId} /></div><Controls /></SessionProvider></I18nProvider></MemoryRouter></QueryClientProvider>)
  return client
}
function reads(url: string, { admin = false, avatar = true, blocked = false, user = actor as User | null } = {}) {
  if (url.endsWith('/session')) return response({ user, csrf_token: user ? 'csrf-test' : undefined })
  if (url.endsWith('/me/trust')) return response({ is_admin: admin })
  if (url.endsWith('/me/blocks')) return response({ blocks: blocked ? [{ user_id: sender, display_name: 'Organizer', created_at: message.created_at }] : [], next_cursor: '' })
  if (url.includes('/avatars/')) return response({ image: avatar ? { id: imageId, url: '/api/v1/rides/' + rideId + '/avatars/' + url.split('/').at(-1) + '/' + imageId + '/content', width: 200, height: 200, created_at: message.created_at } : null })
  return undefined
}
beforeEach(() => {
  setCsrfToken()
  vi.stubGlobal('PointerEvent', MouseEvent)
  vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} })
  if (!HTMLElement.prototype.scrollIntoView) HTMLElement.prototype.scrollIntoView = vi.fn()
})
afterEach(() => { setCsrfToken(); vi.unstubAllGlobals() })

async function open() {
  const trigger = await screen.findByRole('button', { name: 'Message actions for Organizer' })
  await userEvent.click(trigger)
  return { trigger, menu: await screen.findByRole('menu', { name: 'Message actions for Organizer' }) }
}

describe('per-message action menu', () => {
  it.each([
    ['Report message', 'message', message.id],
    ['Report rider', 'user', sender],
    ['Report rider photo', 'avatar', imageId],
  ])('reports the actual %s target after the menu closes', async (label, type, id) => {
    let payload: unknown, csrf: string | null = null
    vi.stubGlobal('fetch', vi.fn(async (url: string, options?: RequestInit) => {
      if (url.endsWith('/reports')) { payload = JSON.parse(String(options?.body)); csrf = new Headers(options?.headers).get('X-CSRF-Token'); return response({ report: { id: 'r1' } }, 201) }
      return reads(url) ?? response({})
    }))
    mount()
    const { trigger, menu } = await open()
    expect(within(menu).queryByRole('textbox')).not.toBeInTheDocument()
    await userEvent.click(await within(menu).findByRole('menuitem', { name: label }))
    const reason = await screen.findByRole('combobox', { name: 'Reason' })
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
    expect(reason).toHaveFocus()
    fireEvent.change(reason, { target: { value: 'harassment' } })
    await userEvent.type(screen.getByRole('textbox', { name: 'Details' }), 'Please review this item')
    await userEvent.click(screen.getByRole('button', { name: 'Send report' }))
    await screen.findByText('Your report was sent for review.')
    expect(payload).toEqual({ target_type: type, target_id: id, reason: 'harassment', details: 'Please review this item' })
    expect(csrf).toBe('csrf-test')
    expect(trigger).toHaveFocus()
  })
  it('opens from keyboard, navigates items and closes with Escape returning focus', async () => {
    vi.stubGlobal('fetch', vi.fn(async (url: string) => reads(url) ?? response({})))
    mount()
    const trigger = await screen.findByRole('button', { name: 'Message actions for Organizer' })
    trigger.focus()
    await userEvent.keyboard('{ArrowDown}')
    const menu = await screen.findByRole('menu')
    await waitFor(() => expect(within(menu).getByRole('menuitem', { name: 'Report message' })).toHaveFocus())
    await userEvent.keyboard('{ArrowDown}')
    expect(within(menu).getByRole('menuitem', { name: 'Report rider' })).toHaveFocus()
    await userEvent.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('menu')).not.toBeInTheDocument())
    expect(trigger).toHaveFocus()
  })
  it('closes on outside pointer interaction and can cancel an inline report back to its persistent trigger', async () => {
    vi.stubGlobal('fetch', vi.fn(async (url: string) => reads(url) ?? response({})))
    mount()
    const { trigger } = await open()
    fireEvent.pointerDown(document.body, { button: 0 })
    await waitFor(() => expect(screen.queryByRole('menu')).not.toBeInTheDocument())
    await userEvent.click(trigger)
    await userEvent.click(await screen.findByRole('menuitem', { name: 'Report message' }))
    await screen.findByRole('combobox', { name: 'Reason' })
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(screen.queryByRole('textbox', { name: 'Details' })).not.toBeInTheDocument()
    expect(trigger).toHaveFocus()
  })
})

describe('message menu workflows and authorization', () => {
  it('confirms blocking after menu dismissal, returns cancel focus and keeps block/unblock operations alive', async () => {
    let blocked = false, finish: (value: Response) => void = () => {}, signal: AbortSignal | undefined
    const writes: string[] = []
    vi.stubGlobal('fetch', vi.fn(async (url: string, options?: RequestInit) => {
      if (url.endsWith('/me/blocks/' + sender)) {
        writes.push(options?.method ?? '')
        if (options?.method === 'PUT') { signal = options.signal as AbortSignal; return new Promise<Response>((resolve) => { finish = resolve }) }
        blocked = false; return response({ blocked: false })
      }
      return reads(url, { blocked }) ?? response({})
    }))
    mount()
    const { trigger, menu } = await open()
    await userEvent.click(within(menu).getByRole('menuitem', { name: 'Block rider' }))
    const cancelDialog = await screen.findByRole('dialog', { name: 'Block rider' })
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
    expect(within(cancelDialog).getByRole('button', { name: 'Cancel' })).toHaveFocus()
    await userEvent.click(within(cancelDialog).getByRole('button', { name: 'Cancel' }))
    expect(trigger).toHaveFocus()
    expect(writes).toEqual([])
    await userEvent.click(trigger)
    await userEvent.click(await screen.findByRole('menuitem', { name: 'Block rider' }))
    const dialog = await screen.findByRole('dialog', { name: 'Block rider' })
    await userEvent.click(within(dialog).getByRole('button', { name: 'Block rider' }))
    await waitFor(() => expect(signal).toBeDefined())
    expect(signal?.aborted).toBe(false)
    blocked = true
    await act(async () => finish(response({ blocked: true })))
    await userEvent.click(trigger)
    await userEvent.click(await screen.findByRole('menuitem', { name: 'Unblock rider' }))
    await waitFor(() => expect(writes).toEqual(['PUT', 'DELETE']))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })
  it('keeps a report request alive while opening and dismissing the menu again', async () => {
    let finish: (value: Response) => void = () => {}, signal: AbortSignal | undefined
    vi.stubGlobal('fetch', vi.fn(async (url: string, options?: RequestInit) => {
      if (url.endsWith('/reports')) { signal = options?.signal as AbortSignal; return new Promise<Response>((resolve) => { finish = resolve }) }
      return reads(url) ?? response({})
    }))
    mount()
    const { trigger, menu } = await open()
    await userEvent.click(within(menu).getByRole('menuitem', { name: 'Report message' }))
    await screen.findByRole('combobox', { name: 'Reason' })
    await userEvent.type(screen.getByRole('textbox', { name: 'Details' }), 'Keep this request')
    await userEvent.click(screen.getByRole('button', { name: 'Send report' }))
    await waitFor(() => expect(signal).toBeDefined())
    await userEvent.click(trigger)
    expect(await screen.findByRole('menuitem', { name: 'Report message' })).toHaveAttribute('aria-disabled', 'true')
    await userEvent.keyboard('{Escape}')
    expect(signal?.aborted).toBe(false)
    await act(async () => finish(response({ report: { id: 'r1' } }, 201)))
    await screen.findByText('Your report was sent for review.')
    expect(trigger).toHaveFocus()
  })
  it('preserves own-message reporting without self rider, avatar, block or admin actions', async () => {
    let target: unknown
    vi.stubGlobal('fetch', vi.fn(async (url: string, options?: RequestInit) => {
      if (url.endsWith('/reports')) { target = JSON.parse(String(options?.body)); return response({}, 201) }
      return reads(url, { admin: true }) ?? response({})
    }))
    mount({ ...message, sender_id: actor.id, sender_name: 'Rider' })
    await userEvent.click(await screen.findByRole('button', { name: 'Message actions for Rider' }))
    const menu = await screen.findByRole('menu')
    expect(within(menu).getAllByRole('menuitem')).toHaveLength(1)
    await userEvent.click(within(menu).getByRole('menuitem', { name: 'Report message' }))
    await screen.findByRole('combobox', { name: 'Reason' })
    await userEvent.click(screen.getByRole('button', { name: 'Send report' }))
    await screen.findByText('Your report was sent for review.')
    expect(target).toMatchObject({ target_type: 'message', target_id: message.id })
  })
  it('hides unavailable avatar reports and does not infer administrator access from a sender name', async () => {
    vi.stubGlobal('fetch', vi.fn(async (url: string) => url.includes('/avatars/') ? response({ error: { code: 'forbidden' } }, 403) : reads(url) ?? response({})))
    mount()
    const { menu } = await open()
    expect(within(menu).queryByRole('menuitem', { name: 'Report rider photo' })).not.toBeInTheDocument()
    expect(within(menu).queryByRole('menuitem', { name: 'Manage rider' })).not.toBeInTheDocument()
    expect(within(menu).getByRole('menuitem', { name: 'Report rider' })).toBeInTheDocument()
  })
  it('uses only the active administrator role and navigates with the real sender ID', async () => {
    vi.stubGlobal('fetch', vi.fn(async (url: string) => reads(url, { admin: true }) ?? response({})))
    const client = mount()
    const { menu } = await open()
    await userEvent.click(await within(menu).findByRole('menuitem', { name: 'Manage rider' }))
    await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent('/admin?user=' + sender))
    // A background failure can retain old successful data; it must fail closed.
    await act(async () => { await client.fetchQuery({ queryKey: ['trust', actor.id], queryFn: async () => { throw new Error('Trust unavailable') }, retry: false }).catch(() => {}) })
    const next = await open()
    expect(within(next.menu).queryByRole('menuitem', { name: 'Manage rider' })).not.toBeInTheDocument()
  })
  it('rechecks a revoked administrator role between item selection and navigation', async () => {
    vi.stubGlobal('fetch', vi.fn(async (url: string) => reads(url, { admin: true }) ?? response({})))
    const client = mount()
    const { menu } = await open()
    await act(async () => {
      fireEvent.click(await within(menu).findByRole('menuitem', { name: 'Manage rider' }))
      client.setQueryData(['trust', actor.id], { is_admin: false })
    })
    await waitFor(() => expect(screen.queryByRole('menu')).not.toBeInTheDocument())
    expect(screen.getByTestId('location')).toHaveTextContent(/^\/$/)
  })
  it('aborts a report on session replacement and suppresses its late feedback', async () => {
    let finish: (value: Response) => void = () => {}, signal: AbortSignal | undefined
    vi.stubGlobal('fetch', vi.fn(async (url: string, options?: RequestInit) => {
      if (url.endsWith('/reports')) { signal = options?.signal as AbortSignal; return new Promise<Response>((resolve) => { finish = resolve }) }
      return reads(url) ?? response({})
    }))
    const client = mount()
    const { menu } = await open()
    await userEvent.click(within(menu).getByRole('menuitem', { name: 'Report message' }))
    await screen.findByRole('combobox', { name: 'Reason' })
    await userEvent.type(screen.getByRole('textbox', { name: 'Details' }), 'Private report draft')
    await userEvent.click(screen.getByRole('button', { name: 'Send report' }))
    await waitFor(() => expect(signal).toBeDefined())
    await act(async () => { setCsrfToken('next-csrf'); client.setQueryData(['session'], { user: { ...actor, id: imageId }, csrf_token: 'next-csrf' }) })
    await waitFor(() => expect(signal?.aborted).toBe(true))
    await act(async () => finish(response({}, 201)))
    expect(screen.queryByText('Your report was sent for review.')).not.toBeInTheDocument()
    expect(screen.queryByDisplayValue('Private report draft')).not.toBeInTheDocument()
  })
  it('cancels a pending block confirmation when the session changes before any write', async () => {
    const writes: string[] = []
    vi.stubGlobal('fetch', vi.fn(async (url: string, options?: RequestInit) => { if (options?.method === 'PUT') writes.push(url); return reads(url) ?? response({}) }))
    const client = mount()
    const { menu } = await open()
    await userEvent.click(within(menu).getByRole('menuitem', { name: 'Block rider' }))
    await screen.findByRole('dialog', { name: 'Block rider' })
    await act(async () => { setCsrfToken('next-csrf'); client.setQueryData(['session'], { user: { ...actor, id: imageId }, csrf_token: 'next-csrf' }) })
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(writes).toEqual([])
  })
  it.each([
    ['en', 'Message actions for Organizer', 'Report message', 'Report rider', 'Message', 'Rider'],
    ['ru', 'Действия с сообщением: Organizer', 'Пожаловаться на сообщение', 'Пожаловаться на райдера', 'Сообщение', 'Райдер'],
    ['kk', 'Organizer хабарламасының әрекеттері', 'Хабарламаға шағымдану', 'Райдерге шағымдану', 'Хабарлама', 'Райдер'],
  ])('localizes the trigger and grouped actions in %s', async (locale, triggerLabel, reportMessage, reportRider, messageGroup, riderGroup) => {
    localStorage.setItem('bikerskz.locale', locale)
    vi.stubGlobal('fetch', vi.fn(async (url: string) => reads(url) ?? response({})))
    mount()
    await userEvent.click(await screen.findByRole('button', { name: triggerLabel }))
    const menu = await screen.findByRole('menu')
    expect(within(menu).getByRole('group', { name: messageGroup })).toContainElement(within(menu).getByRole('menuitem', { name: reportMessage }))
    expect(within(menu).getByRole('group', { name: riderGroup })).toContainElement(within(menu).getByRole('menuitem', { name: reportRider }))
  })
})

describe('message action draft and session boundaries', () => {
  it('does not launch a selected report after the same actor receives a new session epoch', async () => {
    vi.stubGlobal('fetch', vi.fn(async (url: string) => reads(url) ?? response({})))
    const client = mount()
    const { menu } = await open()
    await act(async () => {
      fireEvent.click(within(menu).getByRole('menuitem', { name: 'Report message' }))
      setCsrfToken('replacement-csrf')
      client.setQueryData(['session'], { user: actor, csrf_token: 'replacement-csrf' })
    })
    await waitFor(() => expect(screen.queryByRole('menu')).not.toBeInTheDocument())
    expect(screen.queryByRole('combobox', { name: 'Reason' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Message actions for Organizer' })).toBeInTheDocument()
  })
  it('keeps failed report details for retry outside the menu without changing its target', async () => {
    let attempts = 0
    const payloads: unknown[] = []
    vi.stubGlobal('fetch', vi.fn(async (url: string, options?: RequestInit) => {
      if (url.endsWith('/reports')) { payloads.push(JSON.parse(String(options?.body))); return ++attempts === 1 ? response({ error: { code: 'service_unavailable' } }, 503) : response({}, 201) }
      return reads(url) ?? response({})
    }))
    mount()
    const { trigger, menu } = await open()
    await userEvent.click(within(menu).getByRole('menuitem', { name: 'Report message' }))
    await screen.findByRole('combobox', { name: 'Reason' })
    await userEvent.type(screen.getByRole('textbox', { name: 'Details' }), 'Keep this evidence')
    await userEvent.click(screen.getByRole('button', { name: 'Send report' }))
    await screen.findByRole('alert')
    expect(screen.getByRole('textbox', { name: 'Details' })).toHaveValue('Keep this evidence')
    await userEvent.click(trigger)
    await screen.findByRole('menu')
    await userEvent.keyboard('{Escape}')
    await userEvent.click(screen.getByRole('button', { name: 'Send report' }))
    await screen.findByText('Your report was sent for review.')
    expect(payloads).toHaveLength(2)
    expect(payloads[0]).toEqual(payloads[1])
    expect(payloads[1]).toMatchObject({ target_type: 'message', target_id: message.id, details: 'Keep this evidence' })
  })
})
