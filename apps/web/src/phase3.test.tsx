import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { App } from './App'
import { setCsrfToken } from './api'
import { I18nProvider } from './i18n'
import { SessionProvider } from './session'
import type { Bike, Club, ClubSummary, MaintenanceEntry, Ride, Session, User } from './types'

vi.mock('virtual:pwa-register/react', () => ({ useRegisterSW: () => ({ needRefresh: [false, vi.fn()], updateServiceWorker: vi.fn() }) }))

const rider: User = { id: 'rider-a', profile: { display_name: 'Rider A', city: 'Almaty', languages: ['en'], interests: '', locale: 'en' }, onboarding_complete: true }
const other: User = { ...rider, id: 'rider-b', profile: { ...rider.profile, display_name: 'Rider B' } }
const bike: Bike = { id: '11111111-1111-4111-8111-111111111111', owner_id: rider.id, make: 'Yamaha', model: 'Ténéré 700', model_id: 'yamaha-tenere-700', year: 2024, visibility: 'private', version: 1, can_manage: true, created_at: '2026-11-03T00:00:00Z', updated_at: '2026-11-03T00:00:00Z' }
const club: Club = { id: '22222222-2222-4222-8222-222222222222', name: 'Test Riders', city: 'Almaty', description: 'A test riding group', status: 'active', owner_id: rider.id, version: 1, member_count: 2, membership_state: 'approved', can_manage: true }
const entry: MaintenanceEntry = { id: '33333333-3333-4333-8333-333333333333', bike_id: bike.id, service_date: '2026-10-01', mileage_km: 12000, work_performed: 'Private oil change', version: 2, created_at: '2026-11-03T00:00:00Z', updated_at: '2026-11-03T00:00:00Z' }
const response = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json' } })
const signedIn = (user = rider): Session => ({ user, csrf_token: `csrf-${user.id}` })
function mount(path: string) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } } })
  render(<QueryClientProvider client={client}><MemoryRouter initialEntries={[path]}><I18nProvider><SessionProvider><App /></SessionProvider></I18nProvider></MemoryRouter></QueryClientProvider>)
  return client
}
beforeEach(() => { setCsrfToken(); vi.spyOn(window, 'confirm').mockReturnValue(true) })
afterEach(() => { setCsrfToken(); vi.unstubAllGlobals() })

describe('cookie-bound public views', () => {
  it.each([['bike', `/bikes/${bike.id}`], ['club', `/clubs/${club.id}`], ['clubs', '/clubs']])('does not read %s with an unresolved failed session and safely reconciles anonymous views', async (kind, path) => {
    let failing = true
    const fetch = vi.fn(async (url: string) => {
      if (url.endsWith('/session')) return failing ? response({ error: { code: 'service_unavailable' } }, 503) : response({ user: null })
      if (url.endsWith(`/bikes/${bike.id}`)) return response({ bike })
      if (url.endsWith(`/clubs/${club.id}`)) return response({ club })
      if (url.endsWith(`/clubs/${club.id}/rides`)) return response({ rides: [] })
      if (new URL(url, 'https://bikers.kz').pathname === '/api/v1/clubs') return response({ clubs: [club], next_cursor: '' })
      if (url.endsWith('/cities')) return response({ cities: [] })
      throw new Error(`Unexpected private request ${url}`)
    })
    vi.stubGlobal('fetch', fetch)
    const client = mount(path)
    await screen.findByText('We couldn’t reach BIKERS.KZ.')
    expect(fetch.mock.calls.every(([url]) => url.endsWith('/session'))).toBe(true)
    failing = false
    await userEvent.click(screen.getByRole('button', { name: 'Try again' }))
    await screen.findByText(kind === 'bike' ? 'Ténéré 700' : 'Test Riders')
    const key = kind === 'bike' ? ['public-bike', bike.id, 'guest'] : kind === 'club' ? ['club', club.id, 'guest'] : ['clubs', 'guest', 'discover', '', '', '']
    const cached = client.getQueryData<Record<string, unknown>>(key)
    const value = kind === 'bike' ? cached?.bike : kind === 'club' ? cached?.club : (cached?.clubs as unknown[])?.[0]
    expect(value).not.toHaveProperty('owner_id')
    expect(value).not.toHaveProperty('version')
    expect(value).not.toHaveProperty('member_count')
    expect(fetch.mock.calls.some(([url]) => /\/members|\/maintenance/.test(url))).toBe(false)
  })

  it('does not fetch a roster for pending or historical closed owners who have left', async () => {
    const fetch = vi.fn(async (url: string) => {
      if (url.endsWith('/session')) return response(signedIn())
      if (url.endsWith(`/clubs/${club.id}`)) return response({ club: { id: club.id, name: club.name, city: club.city, description: club.description, status: 'closed', membership_state: 'left', can_manage: false } })
      if (new URL(url, 'https://bikers.kz').pathname === '/api/v1/rides') return response({ rides: [], next_cursor: '' })
      if (url.endsWith('/rides')) return response({ rides: [] })
      throw new Error(`Unexpected private request ${url}`)
    })
    vi.stubGlobal('fetch', fetch)
    mount(`/clubs/${club.id}`)
    await screen.findByRole('heading', { name: club.name })
    expect(screen.queryByRole('heading', { name: 'Club members' })).not.toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Owner tools' })).not.toBeInTheDocument()
    expect(fetch.mock.calls.some(([url]) => url.endsWith('/members'))).toBe(false)
  })
})

describe('garage and private maintenance', () => {
  it('shows the selected model-year reference and clears it immediately when year or model changes', async () => {
    const model = { id: 'honda-cb500x', make: 'Honda', model: 'CB500X', aliases: [], images: [{ url: '/bike-models/honda-cb500x-2019.jpg', year_from: 2019, year_to: 2019, width: 1200, height: 800, source_url: 'https://example.org/cb500x', author: 'Test Photographer', license: 'CC BY-SA 4.0', license_url: 'https://creativecommons.org/licenses/by-sa/4.0/' }] }
    vi.stubGlobal('fetch', vi.fn(async (url: string) => {
      if (url.endsWith('/session')) return response(signedIn())
      if (url.includes('/bike-models?')) return response({ models: [model] })
      throw new Error(`Unexpected API ${url}`)
    }))
    mount('/garage/new')
    const actor = userEvent.setup()
    await actor.type(await screen.findByRole('searchbox', { name: 'Find a model' }), 'Honda')
    await actor.click(await screen.findByRole('button', { name: 'Honda CB500X' }))
    const year = screen.getByLabelText(/^Year/)
    await actor.type(year, '2019')
    await screen.findByRole('img', { name: 'Reference image of Honda CB500X, 2019' })
    fireEvent.change(year, { target: { value: '2020' } })
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
    expect(screen.getByText('No verified reference image is available for this model and year.')).toBeInTheDocument()
    fireEvent.change(screen.getByLabelText(/^Make \*/), { target: { value: 'Freeform make' } })
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
    expect(screen.queryByText(/No verified reference image/)).not.toBeInTheDocument()
  })

  it('keeps card attribution outside garage navigation links and omits broken/freeform photos', async () => {
    const referenceBike = { ...bike, make: 'Honda', model: 'CB500X', model_id: 'honda-cb500x', year: 2019 }
    vi.stubGlobal('fetch', vi.fn(async (url: string) => {
      if (url.endsWith('/session')) return response(signedIn())
      if (url.endsWith('/me/bikes')) return response({ bikes: [referenceBike, { ...bike, id: 'freeform', model_id: '' }], next_cursor: '' })
      if (url.includes('/bike-models?')) return response({ models: [{ id: 'honda-cb500x', make: 'Honda', model: 'CB500X', aliases: [], images: [{ url: '/bike-models/honda-cb500x-2019.jpg', year_from: 2019, year_to: 2019, width: 1200, height: 800, source_url: 'https://example.org/cb500x', author: 'Test Photographer', license: 'CC BY-SA 4.0', license_url: 'https://creativecommons.org/licenses/by-sa/4.0/' }] }] })
      throw new Error(`Unexpected API ${url}`)
    }))
    mount('/garage')
    const photo = await screen.findByRole('img')
    const card = photo.closest('article')!
    const source = within(card).getByRole('link', { name: 'Source' })
    expect(source.parentElement?.closest('a')).toBeNull()
    expect(within(card).getByRole('link', { name: /Honda.*CB500X/ })).toHaveAttribute('href', `/garage/${bike.id}`)
    expect(screen.getAllByRole('img')).toHaveLength(1)
    fireEvent.error(photo)
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
    expect(document.querySelector('.model-reference-frame')).not.toBeInTheDocument()
    expect(screen.getByText('Ténéré 700')).toBeInTheDocument()
  })

  it('resolves a public reference without adding images or owner fields to the five-field bike projection', async () => {
    const referenceBike = { ...bike, make: 'Honda', model: 'CB500X', model_id: 'honda-cb500x', year: 2019 }
    vi.stubGlobal('fetch', vi.fn(async (url: string) => {
      if (url.endsWith('/session')) return response({ user: null })
      if (url.endsWith(`/bikes/${bike.id}`)) return response({ bike: referenceBike })
      if (url.includes('/bike-models?')) return response({ models: [{ id: 'honda-cb500x', make: 'Honda', model: 'CB500X', aliases: [], images: [{ url: '/bike-models/honda-cb500x-2019.jpg', year_from: 2019, year_to: 2019, width: 1200, height: 800, source_url: 'https://example.org/cb500x', author: 'Test Photographer', license: 'CC BY-SA 4.0', license_url: 'https://creativecommons.org/licenses/by-sa/4.0/' }] }] })
      throw new Error(`Unexpected API ${url}`)
    }))
    const client = mount(`/bikes/${bike.id}`)
    await screen.findByRole('img')
    const cached = client.getQueryData<{ bike: Record<string, unknown> }>(['public-bike', bike.id, 'guest'])!
    expect(Object.keys(cached.bike).sort()).toEqual(['id', 'make', 'model', 'model_id', 'year'])
    expect(screen.queryByText('Private oil change')).not.toBeInTheDocument()
  })

  it('selects canonical aliases and creates a private bike with CSRF', async () => {
    const fetch = vi.fn(async (url: string, options?: RequestInit) => {
      if (url.endsWith('/session')) return response(signedIn())
      if (url.includes('/bike-models?')) return response({ models: [{ id: bike.model_id, make: bike.make, model: bike.model, aliases: ['тенере'] }] })
      if (url.endsWith('/me/bikes') && options?.method === 'POST') return response({ bike }, 201)
      if (url.endsWith(`/bikes/${bike.id}`)) return response({ bike })
      if (url.includes('/maintenance')) return response({ entries: [], next_cursor: '' })
      throw new Error(`Unexpected API ${url} ${options?.method}`)
    })
    vi.stubGlobal('fetch', fetch)
    mount('/garage/new')
    const actor = userEvent.setup()
    await actor.type(await screen.findByRole('searchbox', { name: 'Find a model' }), 'тенере')
    await actor.click(await screen.findByRole('button', { name: 'Yamaha Ténéré 700' }))
    await actor.type(screen.getByLabelText(/^Year/), '2024')
    await actor.click(screen.getByRole('button', { name: 'Save motorcycle' }))
    await screen.findByRole('heading', { name: 'Ténéré 700' })
    const call = fetch.mock.calls.find(([url, options]) => url.endsWith('/me/bikes') && options?.method === 'POST')!
    expect(JSON.parse(call[1]!.body as string)).toMatchObject({ make: bike.make, model: bike.model, model_id: bike.model_id, year: 2024, visibility: 'private' })
    expect(new Headers(call[1]!.headers).get('X-CSRF-Token')).toBe('csrf-rider-a')
    expect(screen.queryByRole('button', { name: 'Copy link' })).not.toBeInTheDocument()
  })

  it('uses server cursor paging and keeps maintenance out of public bike views', async () => {
    const nextBike = { ...bike, id: '44444444-4444-4444-8444-444444444444', model: 'Second model' }
    const fetch = vi.fn(async (url: string) => {
      if (url.endsWith('/session')) return response(signedIn())
      if (url.includes('/me/bikes?cursor=')) return response({ bikes: [nextBike], next_cursor: '' })
      if (url.endsWith('/me/bikes')) return response({ bikes: [bike], next_cursor: bike.id })
      throw new Error(`Unexpected API ${url}`)
    })
    vi.stubGlobal('fetch', fetch)
    mount('/garage')
    await screen.findByText('Ténéré 700')
    await userEvent.click(screen.getByRole('button', { name: 'Next page' }))
    await screen.findByText('Second model')
    expect(fetch.mock.calls.some(([url]) => url.endsWith(`?cursor=${bike.id}`))).toBe(true)
    expect(screen.queryByText('Ténéré 700')).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Previous page' }))
    await screen.findByText('Ténéré 700')
  })

  it('edits and deletes private log entries with their current versions and confirmation', async () => {
    let entries = [entry]
    const commands: { method: string; body: Record<string, unknown>; csrf: string | null }[] = []
    vi.stubGlobal('fetch', vi.fn(async (url: string, options?: RequestInit) => {
      if (url.endsWith('/session')) return response(signedIn())
      if (url.endsWith(`/bikes/${bike.id}`)) return response({ bike })
      if (url.endsWith(`/maintenance/${entry.id}`) && options?.method) {
        const body = JSON.parse(options.body as string)
        commands.push({ method: options.method, body, csrf: new Headers(options.headers).get('X-CSRF-Token') })
        if (options.method === 'DELETE') { entries = []; return response({ ok: true }) }
        entries = [{ ...entry, ...body, version: 3 }]
        return response({ entry: entries[0] })
      }
      if (url.includes('/maintenance')) return response({ entries, next_cursor: '' })
      throw new Error(`Unexpected API ${url}`)
    }))
    mount(`/garage/${bike.id}`)
    await screen.findByText('Private oil change')
    const actor = userEvent.setup()
    await actor.click(screen.getByRole('button', { name: 'Edit' }))
    const work = screen.getByLabelText(/^Work performed/)
    await actor.clear(work); await actor.type(work, 'Private valve check')
    await actor.click(screen.getByRole('button', { name: 'Save entry' }))
    await screen.findByText('Private valve check')
    await actor.click(screen.getByRole('button', { name: 'Delete entry' }))
    const firstDialog = await screen.findByRole('dialog', { name: 'Delete entry' })
    expect(within(firstDialog).getByRole('button', { name: 'Cancel' })).toHaveFocus()
    await actor.click(within(firstDialog).getByRole('button', { name: 'Cancel' }))
    expect(commands).toHaveLength(1)
    await actor.click(screen.getByRole('button', { name: 'Delete entry' }))
    await actor.click(within(await screen.findByRole('dialog', { name: 'Delete entry' })).getByRole('button', { name: 'Delete entry' }))
    await screen.findByRole('heading', { name: 'A clean page for your bike.' })
    expect(commands[0]).toMatchObject({ method: 'PATCH', body: { version: 2, work_performed: 'Private valve check' }, csrf: 'csrf-rider-a' })
    expect(commands[1]).toMatchObject({ method: 'DELETE', body: { version: 3 }, csrf: 'csrf-rider-a' })
    expect(window.confirm).not.toHaveBeenCalled()
  })
})

describe('club owner transitions and private access', () => {
  it('pages organized ride choices without linking until the owner explicitly submits', async () => {
    const first: Ride = { id: '44444444-4444-4444-8444-444444444444', city: 'Almaty', date: '2026-12-01', description: 'First organized ride', status: 'published', can_manage: true }
    const second: Ride = { ...first, id: '55555555-5555-4555-8555-555555555555', description: 'Second organized ride' }
    const commands: { url: string; method: string; csrf: string | null }[] = []
    const pages: string[] = []
    let linked: Ride[] = []
    const fetch = vi.fn(async (url: string, options?: RequestInit) => {
      if (url.endsWith('/session')) return response(signedIn())
      if (url.endsWith('/me/trust')) return response({ is_admin: false })
      if (url.includes('/avatar')) return response({ image: null, version: 1 })
      if (url.endsWith('/badge')) return response({ badge: null })
      if (url.endsWith('/me/blocks')) return response({ blocks: [], next_cursor: '' })
      if (url.endsWith(`/clubs/${club.id}`)) return response({ club })
      if (url.endsWith(`/clubs/${club.id}/image`)) return response({ image: null, version: 1 })
      if (url.endsWith('/members')) return response({ members: [{ user_id: rider.id, display_name: rider.profile.display_name, state: 'approved', role: 'owner' }] })
      if (url.endsWith(`/clubs/${club.id}/rides/${second.id}`) && options?.method === 'PUT') {
        commands.push({ url, method: options.method, csrf: new Headers(options.headers).get('X-CSRF-Token') })
        linked = [second]
        return response({ ok: true })
      }
      if (url.endsWith(`/clubs/${club.id}/rides`)) return response({ rides: linked })
      const parsed = new URL(url, 'https://bikers.kz')
      if (parsed.pathname === '/api/v1/rides') {
        expect(parsed.searchParams.get('view')).toBe('organizing')
        const cursor = parsed.searchParams.get('cursor') ?? ''
        pages.push(cursor)
        return response({ rides: [cursor ? second : first], next_cursor: cursor ? '' : 'owned-page-two' })
      }
      if (options?.method && options.method !== 'GET') commands.push({ url, method: options.method, csrf: new Headers(options.headers).get('X-CSRF-Token') })
      throw new Error(`Unexpected API ${url} ${options?.method}`)
    })
    vi.stubGlobal('fetch', fetch)
    mount(`/clubs/${club.id}`)
    const actor = userEvent.setup()
    const chooser = await screen.findByRole('combobox', { name: 'Choose your ride' })
    await actor.selectOptions(chooser, first.id)
    await actor.click(screen.getByRole('button', { name: 'Next page' }))
    await screen.findByRole('option', { name: /Second organized ride/ })
    expect(screen.queryByRole('option', { name: /First organized ride/ })).not.toBeInTheDocument()
    expect(commands).toEqual([])
    await actor.selectOptions(screen.getByRole('combobox', { name: 'Choose your ride' }), second.id)
    await actor.click(screen.getByRole('button', { name: 'Previous page' }))
    await screen.findByRole('option', { name: /First organized ride/ })
    expect(commands).toEqual([])
    await actor.click(screen.getByRole('button', { name: 'Next page' }))
    await screen.findByRole('option', { name: /Second organized ride/ })
    await actor.selectOptions(screen.getByRole('combobox', { name: 'Choose your ride' }), second.id)
    expect(screen.getByRole('textbox', { name: 'Ride link or ID' })).toHaveValue(second.id)
    expect(commands).toEqual([])
    await actor.click(screen.getByRole('button', { name: 'Link a ride' }))
    await screen.findByRole('link', { name: /Second organized ride/ })
    expect(commands).toEqual([{ url: `/api/v1/clubs/${club.id}/rides/${second.id}`, method: 'PUT', csrf: 'csrf-rider-a' }])
    expect(pages).toContain('owned-page-two')
  })

  it('transfers ownership without retaining owner-only requests or exposing linked ride details', async () => {
    let current: Club = club
    const fetch = vi.fn(async (url: string, options?: RequestInit) => {
      if (url.endsWith('/session')) return response(signedIn())
      if (url.endsWith(`/clubs/${club.id}/ownership`)) { current = { ...club, owner_id: other.id, version: 2, can_manage: false }; return response({ club: current }) }
      if (url.endsWith(`/clubs/${club.id}`)) return response({ club: current })
      if (url.endsWith('/members')) return response({ members: [{ user_id: rider.id, display_name: rider.profile.display_name, state: 'approved', role: current.owner_id === rider.id ? 'owner' : 'member' }, { user_id: other.id, display_name: other.profile.display_name, state: 'approved', role: current.owner_id === other.id ? 'owner' : 'member' }, ...(current.can_manage ? [{ user_id: 'pending', display_name: 'Pending Private Name', state: 'requested', role: 'member' }] : [])] })
      if (url.endsWith(`/clubs/${club.id}/rides`)) return response({ rides: [{ id: 'ride', city: 'Almaty', date: '2026-12-01', description: 'Public ride summary', status: 'published', title: 'Secret ride title', meeting_point: 'Secret place' }] })
      if (new URL(url, 'https://bikers.kz').pathname === '/api/v1/rides') return response({ rides: [], next_cursor: '' })
      if (url.endsWith('/rides')) return response({ rides: [] })
      throw new Error(`Unexpected API ${url} ${options?.method}`)
    })
    vi.stubGlobal('fetch', fetch)
    const client = mount(`/clubs/${club.id}`)
    await screen.findByText('Pending Private Name')
    const actor = userEvent.setup()
    await actor.selectOptions(screen.getByLabelText('Choose the next owner'), other.id)
    await actor.click(screen.getByRole('button', { name: 'Transfer ownership' }))
    const firstDialog = await screen.findByRole('dialog', { name: 'Transfer ownership' })
    expect(firstDialog).toHaveTextContent('You will no longer have owner controls.')
    await actor.click(within(firstDialog).getByRole('button', { name: 'Cancel' }))
    expect(fetch.mock.calls.some(([url]) => url.endsWith('/ownership'))).toBe(false)
    await actor.click(screen.getByRole('button', { name: 'Transfer ownership' }))
    await actor.click(within(await screen.findByRole('dialog', { name: 'Transfer ownership' })).getByRole('button', { name: 'Transfer ownership' }))
    await waitFor(() => expect(screen.queryByRole('heading', { name: 'Owner tools' })).not.toBeInTheDocument())
    expect(screen.queryByText('Pending Private Name')).not.toBeInTheDocument()
    expect(client.getQueryData(['club-members', club.id, rider.id, 'owner'])).toBeUndefined()
    expect(screen.queryByText('Secret ride title')).not.toBeInTheDocument()
    expect(screen.queryByText('Secret place')).not.toBeInTheDocument()
    const call = fetch.mock.calls.find(([url]) => url.endsWith('/ownership'))!
    expect(JSON.parse(call[1]!.body as string)).toEqual({ version: 1, user_id: other.id })
    expect(new Headers(call[1]!.headers).get('X-CSRF-Token')).toBe('csrf-rider-a')
  })

  it('allows the owner to leave after closing and removes the closed roster immediately', async () => {
    let current: Club | ClubSummary = club
    const fetch = vi.fn(async (url: string, options?: RequestInit) => {
      if (url.endsWith('/session')) return response(signedIn())
      if (url.endsWith('/close')) { current = { ...club, status: 'closed', version: 2, can_manage: false }; return response({ club: current }) }
      if (url.endsWith('/membership') && options?.method === 'DELETE') { current = { id: club.id, name: club.name, city: club.city, description: club.description, status: 'closed', membership_state: 'left', can_manage: false }; return response({ state: 'left' }) }
      if (url.endsWith(`/clubs/${club.id}`)) return response({ club: current })
      if (url.endsWith('/members')) return response({ members: [{ user_id: rider.id, display_name: 'Private Owner Name', state: 'approved', role: 'owner' }] })
      if (new URL(url, 'https://bikers.kz').pathname === '/api/v1/rides') return response({ rides: [], next_cursor: '' })
      if (url.endsWith('/rides')) return response({ rides: [] })
      throw new Error(`Unexpected API ${url}`)
    })
    vi.stubGlobal('fetch', fetch)
    const client = mount(`/clubs/${club.id}`)
    await screen.findByText('Private Owner Name')
    expect(screen.queryByRole('button', { name: 'Leave club' })).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Close club' }))
    await userEvent.click(within(await screen.findByRole('dialog', { name: 'Close club' })).getByRole('button', { name: 'Close club' }))
    await userEvent.click(await screen.findByRole('button', { name: 'Leave club' }))
    await userEvent.click(within(await screen.findByRole('dialog', { name: 'Leave club' })).getByRole('button', { name: 'Leave club' }))
    await waitFor(() => expect(screen.queryByRole('heading', { name: 'Club members' })).not.toBeInTheDocument())
    expect(screen.queryByText('Private Owner Name')).not.toBeInTheDocument()
    expect(client.getQueriesData({ queryKey: ['club-members', club.id] }).every(([, value]) => value === undefined)).toBe(true)
    expect(window.confirm).not.toHaveBeenCalled()
  })
})

describe('session completion boundaries', () => {
  it('discards a late club creation in the logout epoch before the session cache has caught up', async () => {
    let finish: (value: Response) => void = () => { throw new Error('not started') }
    const pending = new Promise<Response>((resolve) => { finish = resolve })
    const fetch = vi.fn(async (url: string, options?: RequestInit) => {
      if (url.endsWith('/session')) return response(signedIn())
      if (url.endsWith('/cities')) return response({ cities: [] })
      if (url.endsWith('/clubs') && options?.method === 'POST') return pending
      throw new Error(`Unexpected API ${url}`)
    })
    vi.stubGlobal('fetch', fetch)
    const client = mount('/clubs/new')
    const actor = userEvent.setup()
    await actor.type(await screen.findByLabelText(/^Club name/), club.name)
    await actor.type(screen.getByLabelText(/^Club description/), club.description)
    await actor.click(screen.getByRole('button', { name: 'Create a club' }))
    await waitFor(() => expect(fetch.mock.calls.some(([url, options]) => url.endsWith('/clubs') && options?.method === 'POST')).toBe(true))
    await act(async () => { setCsrfToken(); finish(response({ club }, 201)); await pending })
    expect(client.getQueryData(['club', club.id, rider.id])).toBeUndefined()
    expect(screen.getByRole('heading', { name: 'Give your group a home.' })).toBeInTheDocument()
  })

  it('discards a delayed bike creation after account switch without restoring cache or navigating', async () => {
    let finish: (value: Response) => void = () => { throw new Error('not started') }
    const pending = new Promise<Response>((resolve) => { finish = resolve })
    const fetch = vi.fn(async (url: string, options?: RequestInit) => {
      if (url.endsWith('/session')) return response(signedIn())
      if (url.endsWith('/me/bikes') && options?.method === 'POST') return pending
      throw new Error(`Unexpected API ${url}`)
    })
    vi.stubGlobal('fetch', fetch)
    const client = mount('/garage/new')
    const actor = userEvent.setup()
    await actor.type(await screen.findByLabelText(/^Make \*/), bike.make)
    await actor.type(screen.getByLabelText(/^Model \*/), bike.model)
    await actor.type(screen.getByLabelText(/^Year/), '2024')
    await actor.click(screen.getByRole('button', { name: 'Save motorcycle' }))
    await waitFor(() => expect(fetch.mock.calls.some(([url, options]) => url.endsWith('/me/bikes') && options?.method === 'POST')).toBe(true))
    await act(async () => { setCsrfToken('csrf-rider-b'); client.setQueryData(['session'], signedIn(other)) })
    await act(async () => { finish(response({ bike }, 201)); await pending })
    expect(client.getQueryData(['bike', bike.id, rider.id])).toBeUndefined()
    expect(screen.getByRole('heading', { name: 'Make room for your bike.' })).toBeInTheDocument()
    await waitFor(() => expect(screen.getByLabelText(/^Make \*/)).toHaveValue(''))
  })

  it('clears every new private query family when the session becomes anonymous, even without metadata', async () => {
    let active = true
    vi.stubGlobal('fetch', vi.fn(async (url: string) => {
      if (url.endsWith('/session')) return response(active ? signedIn() : { user: null })
      if (url.endsWith('/me/bikes')) return response({ bikes: [], next_cursor: '' })
      if (new URL(url, 'https://bikers.kz').pathname === '/api/v1/rides') return response({ rides: [], next_cursor: '' })
      if (url.endsWith('/rides')) return response({ rides: [] })
      throw new Error(`Unexpected API ${url}`)
    }))
    const client = mount('/garage')
    await screen.findByRole('heading', { name: 'Room for your first bike.' })
    const keys = [['bike', bike.id, rider.id], ['public-bike', bike.id, rider.id], ['maintenance', bike.id, rider.id, ''], ['club', club.id, rider.id], ['club-members', club.id, rider.id, 'owner'], ['clubs', rider.id, 'organizing', '', '', ''], ['linkable-rides', rider.id]]
    for (const key of keys) client.setQueryData(key, { private_value: 'not for the next actor' })
    active = false
    await act(async () => { await client.refetchQueries({ queryKey: ['session'] }) })
    for (const key of keys) expect(client.getQueryData(key)).toBeUndefined()
  })

  it('keeps new core fields and actions available in Russian and Kazakh', async () => {
    vi.stubGlobal('fetch', vi.fn(async (url: string) => response(url.endsWith('/session') ? signedIn() : { bikes: [], next_cursor: '' })))
    mount('/garage/new')
    await screen.findByRole('heading', { name: 'Make room for your bike.' })
    fireEvent.change(screen.getByRole('combobox', { name: 'Language' }), { target: { value: 'ru' } })
    expect(screen.getByRole('button', { name: 'Сохранить мотоцикл' })).toBeInTheDocument()
    fireEvent.change(screen.getByRole('combobox', { name: 'Язык' }), { target: { value: 'kk' } })
    expect(screen.getByRole('button', { name: 'Мотоциклді сақтау' })).toBeInTheDocument()
    expect(screen.getByLabelText(/^Маркасы/)).toBeInTheDocument()
  })
})
