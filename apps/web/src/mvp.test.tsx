import { useState } from 'react'
import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider, useQuery } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { setCsrfToken } from './api'
import { I18nProvider } from './i18n'
import { SessionProvider, useSession } from './session'
import { AdminPage } from './pages/Admin'
import { InboxPage } from './pages/Inbox'
import { PushSettings, applicationServerKey, pushCapability } from './components/PushSettings'
import { BlockButton, BlockedUsersPanel, ReportButton } from './components/Trust'
import { AvatarEditor, safeAvatar } from './components/Avatar'
import { safeRideAlbum, safeVideoUrl } from './components/RideMedia'
import { pushNotification } from './push-notification'
import { resetTrustQueries } from './trust-cache'
import type { User } from './types'

const actor: User = { id: '00000000-0000-4000-8000-000000000001', profile: { display_name: 'Rider', city: 'Almaty', languages: ['en'], locale: 'en', interests: '' }, onboarding_complete: true }
const rideId = '00000000-0000-4000-8000-000000000002'
const photoId = '00000000-0000-4000-8000-000000000003'
const reportId = '00000000-0000-4000-8000-000000000004'
const created_at = '2026-11-03T12:00:00Z'
function response(value: unknown, status = 200) { return new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json' } }) }
function Ready({ children }: { children: React.ReactNode }) { const { user } = useSession(); return user ? children : null }
function mount(children: React.ReactNode, path = '/') {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
  render(<QueryClientProvider client={client}><MemoryRouter initialEntries={[path]}><I18nProvider><SessionProvider><Ready>{children}</Ready></SessionProvider></I18nProvider></MemoryRouter></QueryClientProvider>)
  return client
}
function session(url: string) { if (url.endsWith('/session')) return response({ user: actor, csrf_token: 'csrf-test' }); return null }
afterEach(() => { vi.unstubAllGlobals(); setCsrfToken() })

describe('MVP media boundaries', () => {
  it('only accepts contextual avatar URLs and preserves hidden album deletion controls without bytes', () => {
    const image = { id: photoId, url: `/api/v1/me/avatar/${photoId}/content`, width: 200, height: 100, created_at }
    expect(safeAvatar({ image, version: 1 }, '/me/avatar', true).image?.id).toBe(photoId)
    expect(safeAvatar({ image: { ...image, url: 'https://third-party.test/tracker' }, version: 1 }, '/me/avatar', true).image).toBeNull()
    const album = safeRideAlbum({ version: 2, photos: [{ ...image, hidden: true, can_delete: true, uploader_id: actor.id }], videos: [{ id: reportId, uploader_id: actor.id, can_delete: true, hidden: true, created_at, url: 'https://private.test/secret', title: 'Hidden title' }] }, rideId)
    expect(album.photos[0]).toMatchObject({ hidden: true, can_delete: true, url: '' })
    expect(album.videos[0]).toMatchObject({ hidden: true, can_delete: true, url: '', title: '' })
    expect(safeRideAlbum({ version: 2, photos: [{ ...image, hidden: true, uploader_id: actor.id }], videos: [] }, rideId).photos).toEqual([])
  })
  it('shows the initials fallback hint only while the rider has no uploaded avatar', async () => {
    let image: unknown = null
    vi.stubGlobal('fetch', vi.fn(async (url: string) => session(url) ?? response({ image, version: 1 })))
    const client = mount(<AvatarEditor />)
    await screen.findByText('Your initials are shown until you add a photo.')
    image = { id: photoId, url: `/api/v1/me/avatar/${photoId}/content`, width: 200, height: 100, created_at }
    await act(async () => { await client.refetchQueries({ queryKey: ['avatar'] }) })
    await screen.findByRole('button', { name: 'Remove rider photo' })
    expect(screen.queryByText('Your initials are shown until you add a photo.')).not.toBeInTheDocument()
  })
  it('rejects video credentials, executable schemes, and local addresses', () => {
    for (const value of ['javascript:alert(1)', 'http://video.example/watch', 'https://user:secret@video.example/watch', 'https://127.0.0.1/video', 'https://localhost/video', 'https://[::1]/video']) expect(safeVideoUrl(value)).toBeUndefined()
    expect(safeVideoUrl('https://youtu.be/example')).toBe('https://youtu.be/example')
  })
  it('resets active mounted history and its incremental cursor before refetching after blocking', async () => {
    let blocked = false
    vi.stubGlobal('fetch', vi.fn(async (url: string) => session(url)!))
    function History() { const query = useQuery({ queryKey: ['messages', rideId, actor.id], queryFn: () => ({ messages: blocked ? [] : ['Old private text'], next_sequence: blocked ? 0 : 8 }) }); return <p>{query.data?.messages.join(',') ?? 'Loading'}</p> }
    const client = mount(<History />)
    await screen.findByText('Old private text')
    blocked = true
    await act(async () => { await resetTrustQueries(client) })
    await waitFor(() => expect(screen.queryByText('Old private text')).not.toBeInTheDocument())
    expect(client.getQueryData(['messages', rideId, actor.id])).toEqual({ messages: [], next_sequence: 0 })
  })
})

describe('blocking cache reconciliation', () => {
  it('refreshes blocks after a ride reset unmounts the PUT caller before its response arrives', async () => {
    let blocked = false
    let finishPut: ((value: Response) => void) | undefined
    let putSignal: AbortSignal | null | undefined
    const rideRefresh: { pending?: Promise<{ id: string }> } = {}
    let finishRide: ((value: { id: string }) => void) | undefined
    vi.stubGlobal('fetch', vi.fn(async (url: string, options?: RequestInit) => {
      const auth = session(url); if (auth) return auth
      if (url.endsWith('/me/blocks')) return response({ blocks: blocked ? [{ user_id: photoId, display_name: 'Organizer', created_at }] : [], next_cursor: '' })
      if (url.endsWith(`/me/blocks/${photoId}`) && options?.method === 'PUT') {
        blocked = true; putSignal = options.signal
        return new Promise<Response>((resolve) => { finishPut = resolve })
      }
      throw new Error(url)
    }))
    function RideOrSettings() {
      const [settings, setSettings] = useState(false)
      const ride = useQuery({ queryKey: ['ride', rideId, actor.id], queryFn: async () => rideRefresh.pending ?? { id: rideId } })
      return settings ? <BlockedUsersPanel /> : <><button onClick={() => setSettings(true)}>Open settings</button>{ride.data && <BlockButton userId={photoId} name="Organizer" />}</>
    }
    const client = mount(<RideOrSettings />)
    await userEvent.click(await screen.findByRole('button', { name: 'Block rider' }))
    await userEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Block rider' }))
    await waitFor(() => expect(finishPut).toBeTypeOf('function'))
    // The server commits and sends trust.changed before returning the PUT reply.
    rideRefresh.pending = new Promise((resolve) => { finishRide = resolve })
    let resetting: Promise<void> | undefined
    await act(async () => { resetting = resetTrustQueries(client) })
    await waitFor(() => expect(putSignal?.aborted).toBe(true))
    await act(async () => { finishPut!(response({ blocked: true })); finishRide!({ id: rideId }); await resetting })
    await userEvent.click(screen.getByRole('button', { name: 'Open settings' }))
    await screen.findByText('Organizer')
    expect(await screen.findByRole('button', { name: 'Unblock rider' })).toBeEnabled()
    expect(screen.queryByText('You have not blocked any riders.')).not.toBeInTheDocument()
  })
  it('does not let a completed old-session PUT reset the next account caches', async () => {
    let finishPut: ((value: Response) => void) | undefined
    vi.stubGlobal('fetch', vi.fn(async (url: string, options?: RequestInit) => {
      const auth = session(url); if (auth) return auth
      if (url.endsWith('/me/blocks')) return response({ blocks: [], next_cursor: '' })
      if (url.endsWith(`/me/blocks/${photoId}`) && options?.method === 'PUT') return new Promise<Response>((resolve) => { finishPut = resolve })
      throw new Error(url)
    }))
    const client = mount(<BlockButton userId={photoId} name="Organizer" />)
    await userEvent.click(await screen.findByRole('button', { name: 'Block rider' }))
    await userEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Block rider' }))
    await waitFor(() => expect(finishPut).toBeTypeOf('function'))
    const nextActor = { ...actor, id: reportId }
    await act(async () => { setCsrfToken('next-session'); client.setQueryData(['session'], { user: nextActor, csrf_token: 'next-session' }) })
    // This unobserved sentinel must survive the fixture's immediate garbage collection.
    client.setQueryDefaults(['messages', rideId, nextActor.id], { gcTime: Infinity })
    client.setQueryData(['messages', rideId, nextActor.id], { messages: ['Next account history'], next_sequence: 9 })
    await act(async () => { finishPut!(response({ blocked: true })); await new Promise((resolve) => setTimeout(resolve, 0)) })
    expect(client.getQueryData(['messages', rideId, nextActor.id])).toEqual({ messages: ['Next account history'], next_sequence: 9 })
  })
})

describe('durable inbox and reporting', () => {
  it('renders generic notices, navigates to authorized ride reads, and marks all read through CSRF', async () => {
    let unread = true
    const fetch = vi.fn(async (url: string, options?: RequestInit) => {
      const auth = session(url); if (auth) return auth
      if (url.endsWith('/me/notifications/read')) { unread = false; return new Response(null, { status: 204 }) }
      if (url.endsWith('/me/notifications')) return response({ notifications: [{ id: reportId, kind: 'ride.updated', ride_id: rideId, created_at, read_at: unread ? null : created_at }], unread_count: unread ? 1 : 0, next_cursor: '' })
      throw new Error(`${url} ${options?.method}`)
    })
    vi.stubGlobal('fetch', fetch)
    mount(<InboxPage />)
    await screen.findByRole('heading', { name: 'A ride plan changed' })
    expect(screen.getByRole('link', { name: 'Open ride' })).toHaveAttribute('href', `/rides/${rideId}`)
    await userEvent.click(screen.getByRole('button', { name: 'Mark all as read' }))
    await screen.findByText('0 unread updates')
    const call = fetch.mock.calls.find(([url]) => url.endsWith('/notifications/read'))!
    expect(call[1]?.method).toBe('PUT')
    expect(new Headers(call[1]?.headers).get('X-CSRF-Token')).toBe('csrf-test')
  })
  it('submits report type, reason and details and confirms without navigating away', async () => {
    let sent: Record<string, unknown> = {}
    vi.stubGlobal('fetch', vi.fn(async (url: string, options?: RequestInit) => {
      const auth = session(url); if (auth) return auth
      if (url.endsWith('/reports')) { sent = JSON.parse(options?.body as string); return response({ report: { id: reportId, status: 'open', created_at } }, 201) }
      throw new Error(url)
    }))
    mount(<ReportButton targetType="ride_photo" targetId={photoId} />)
    await userEvent.click(await screen.findByRole('button', { name: 'Report' }))
    fireEvent.change(screen.getByRole('combobox', { name: 'Reason' }), { target: { value: 'harassment' } })
    await userEvent.type(screen.getByRole('textbox', { name: 'Details' }), 'Please review this photo')
    await userEvent.click(screen.getByRole('button', { name: 'Send report' }))
    await screen.findByRole('status')
    expect(sent).toEqual({ target_type: 'ride_photo', target_id: photoId, reason: 'harassment', details: 'Please review this photo' })
    expect(screen.queryByRole('textbox', { name: 'Details' })).not.toBeInTheDocument()
  })
})

describe('administrator review', () => {
  it('does not request admin evidence when the signed-in account lacks the role', async () => {
    const fetch = vi.fn(async (url: string) => session(url) ?? response({ is_admin: false }))
    vi.stubGlobal('fetch', fetch)
    mount(<AdminPage />)
    await screen.findByText('This page isn’t available to your account.')
    expect(fetch.mock.calls.some(([url]) => url.includes('/admin/'))).toBe(false)
  })
  it.each([
    ['en', 'Action history', ['Reported image reviewed', 'Administrator access granted', 'Administrator access revoked', 'Report marked open', 'Report marked under review', 'Report resolved', 'Report dismissed', 'Report updated'], ['Ride photo', 'Report']],
    ['ru', 'История действий', ['Изображение по жалобе просмотрено', 'Доступ администратора предоставлен', 'Доступ администратора отозван', 'Жалоба открыта', 'Жалоба взята на рассмотрение', 'Жалоба решена', 'Жалоба отклонена', 'Жалоба обновлена'], ['Фото поездки', 'Жалоба']],
    ['kk', 'Әрекеттер тарихы', ['Шағым түскен сурет қаралды', 'Әкімші құқығы берілді', 'Әкімші құқығы қайтарылды', 'Шағым ашық деп белгіленді', 'Шағым қарауға алынды', 'Шағым шешілді', 'Шағым қабылданбады', 'Шағым жаңартылды'], ['Сапар фотосы', 'Шағым']],
  ])('localizes every audit action and target label in %s while preserving the review reason', async (locale, history, labels, targetLabels) => {
    localStorage.setItem('bikerskz.locale', locale)
    const auditActions = ['review_content', 'grant_admin', 'revoke_admin', 'report_open', 'report_reviewing', 'report_resolved', 'report_dismissed', 'update_report']
    const reason = 'User-entered reason: тексеру / проверка'
    vi.stubGlobal('fetch', vi.fn(async (url: string) => {
      const auth = session(url); if (auth) return auth
      if (url.endsWith('/me/trust')) return response({ is_admin: true })
      if (url.includes('/admin/reports?')) return response({ reports: [], next_cursor: '' })
      if (url.endsWith('/admin/audit')) return response({ actions: auditActions.map((action, index) => ({ id: String(index), action, target_type: index === 0 ? 'ride_photo' : 'report', target_id: photoId, reason, created_at, source: 'admin' })), next_cursor: '' })
      throw new Error(url)
    }))
    mount(<AdminPage />)
    await userEvent.click(await screen.findByRole('button', { name: history }))
    for (const label of labels) await screen.findByRole('heading', { name: label })
    for (const label of targetLabels) expect(screen.getAllByText(`${label} · ${photoId}`).length).toBeGreaterThan(0)
    expect(screen.getAllByText(reason)).toHaveLength(auditActions.length)
    expect(screen.queryByText('review_content')).not.toBeInTheDocument()
    expect(screen.queryByText(`ride_photo · ${photoId}`)).not.toBeInTheDocument()
  })
  it('prefills the rider chosen from an administrator roster link for badge assignment', async () => {
    vi.stubGlobal('fetch', vi.fn(async (url: string) => {
      const auth = session(url); if (auth) return auth
      if (url.endsWith('/me/trust')) return response({ is_admin: true })
      if (url.includes('/admin/reports?')) return response({ reports: [], next_cursor: '' })
      throw new Error(url)
    }))
    mount(<AdminPage />, `/admin?user=${actor.id}`)
    expect(await screen.findByRole('textbox', { name: 'User ID' })).toHaveValue(actor.id)
    expect(screen.getByRole('button', { name: 'Assign badge' })).toBeDisabled()
    expect(screen.getByText('Administrator-assigned community badge; not identity, licence, or safety verification.')).toBeInTheDocument()
  })
  it('reviews scoped evidence and reuses an uncertain administrator action ID', async () => {
    const commands: Record<string, unknown>[] = []
    vi.stubGlobal('fetch', vi.fn(async (url: string, options?: RequestInit) => {
      const auth = session(url); if (auth) return auth
      if (url.endsWith('/me/trust')) return response({ is_admin: true })
      if (url.includes('/admin/reports?')) return response({ reports: [{ id: reportId, target_type: 'ride_photo', target_id: photoId, reason: 'spam', details: 'Review it', status: 'open', version: 1, created_at }], next_cursor: '' })
      if (url.endsWith(`/admin/reports/${reportId}`)) return response({ report: { id: reportId, target_type: 'ride_photo', target_id: photoId, reason: 'spam', details: 'Review it', status: 'open', version: 1, created_at }, target: { type: 'ride_photo', id: photoId, author_id: actor.id, text: 'Reported photo' } })
      if (url.endsWith('/admin/actions')) { commands.push(JSON.parse(options?.body as string)); if (commands.length === 1) throw new TypeError('Lost response'); return response({ action: {} }) }
      throw new Error(url)
    }))
    mount(<AdminPage />)
    await userEvent.click(await screen.findByRole('button', { name: /Ride photo.*Spam/ }))
    await screen.findByText('Reported photo')
    const image = screen.getByRole('img', { name: 'Reported content snapshot' })
    expect(image).toHaveAttribute('src', `/api/v1/admin/reports/${reportId}/content`)
    const actionForm = document.querySelector('.report-review .admin-action-form')!
    await userEvent.type(within(actionForm as HTMLElement).getByRole('textbox', { name: 'Required review reason' }), 'Review confirmed spam')
    const submit = within(actionForm as HTMLElement).getByRole('button', { name: 'Hide content' })
    await userEvent.click(submit)
    await userEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Hide content' }))
    await screen.findByText('Connection interrupted. Check your connection and try again.')
    await userEvent.click(submit)
    await userEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Hide content' }))
    await screen.findByText('Action recorded.')
    expect(commands).toHaveLength(2)
    expect(commands[1]).toEqual(commands[0])
    expect(commands[0]).toMatchObject({ action: 'hide_content', target_type: 'ride_photo', target_id: photoId, reason: 'Review confirmed spam' })
  })
})

describe('opt-in device notifications', () => {
  it('keeps private text and arbitrary URLs out of the lock screen and deduplicates redelivery', () => {
    const safe = pushNotification({ title: 'Private ride title', body: 'Meet at a secret garage', url: 'https://attacker.example', notification_id: reportId })
    expect(safe).toMatchObject({ body: 'You have a new update', tag: `bikerskz-${reportId}`, data: { url: '/inbox' } })
    expect(pushNotification({ body: 'Сізде жаңа жаңарту бар', notification_id: reportId }).tag).toBe(safe.tag)
    expect(pushNotification({ body: 'У вас новое обновление' }).body).toBe('У вас новое обновление')
    expect(pushNotification(null).body).toBe('You have a new update')
  })
  it('recognizes unsupported and denied states without requesting permission', () => {
    vi.stubGlobal('isSecureContext', true)
    vi.stubGlobal('PushManager', class {})
    vi.stubGlobal('navigator', { userAgent: 'Desktop', serviceWorker: {} })
    const requestPermission = vi.fn()
    vi.stubGlobal('Notification', { permission: 'denied', requestPermission })
    expect(pushCapability()).toBe('pushDenied')
    vi.stubGlobal('isSecureContext', false)
    expect(pushCapability()).toBe('pushSecure')
    expect(requestPermission).not.toHaveBeenCalled()
  })
  it('only asks browser permission after an explicit click and binds the browser subscription through CSRF', async () => {
    const permission = vi.fn(async () => 'granted')
    const subscribe = vi.fn(async () => ({ toJSON: () => ({ endpoint: 'https://fcm.googleapis.com/example', keys: { p256dh: 'key', auth: 'auth' } }) }))
    vi.stubGlobal('isSecureContext', true)
    vi.stubGlobal('PushManager', class {})
    vi.stubGlobal('Notification', { permission: 'default', requestPermission: permission })
    const getRegistration = vi.fn(async () => ({ active: {}, pushManager: { getSubscription: async () => null, subscribe } }))
    vi.stubGlobal('navigator', { onLine: true, userAgent: 'Desktop test', serviceWorker: { getRegistration } })
    const key = btoa(String.fromCharCode(...new Uint8Array(65).fill(1))).replaceAll('+', '-').replaceAll('/', '_').replaceAll('=', '')
    let registered = false
    const fetch = vi.fn(async (url: string, options?: RequestInit) => {
      const auth = session(url); if (auth) return auth
      if (url.endsWith('/push/settings')) return response({ enabled: true, public_key: key })
      if (url.endsWith('/push/subscriptions') && options?.method === 'POST') { registered = true; return response({ subscription: { id: reportId, created_at } }, 201) }
      if (url.endsWith('/push/subscriptions')) return response({ subscriptions: registered ? [{ id: reportId, created_at }] : [] })
      throw new Error(url)
    })
    vi.stubGlobal('fetch', fetch)
    mount(<PushSettings />)
    const button = await screen.findByRole('button', { name: 'Enable on this device' })
    expect(permission).not.toHaveBeenCalled()
    await userEvent.click(button)
    await screen.findByText('Notifications enabled on this device.')
    expect(permission).toHaveBeenCalledOnce()
    expect(subscribe).toHaveBeenCalledWith({ userVisibleOnly: true, applicationServerKey: applicationServerKey(key) })
    const call = fetch.mock.calls.find(([url, options]) => url.endsWith('/push/subscriptions') && options?.method === 'POST')!
    expect(new Headers(call[1]?.headers).get('X-CSRF-Token')).toBe('csrf-test')
    expect(JSON.parse(call[1]?.body as string).endpoint).toBe('https://fcm.googleapis.com/example')
    expect(Object.values(localStorage)).not.toContain('https://fcm.googleapis.com/example')
  })
})
