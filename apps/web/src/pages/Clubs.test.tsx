import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, Route, Routes, useLocation, useNavigate } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { setCsrfToken } from '../api'
import { I18nProvider } from '../i18n'
import { SessionProvider, useSession } from '../session'
import { ClubsPage, MyClubsPage } from './Clubs'
import type { ClubSummary, User } from '../types'

const rider: User = { id: 'rider-a', profile: { display_name: 'Rider A', city: 'Almaty', languages: ['en'], interests: '', locale: 'en' }, onboarding_complete: true }
const nextRider: User = { ...rider, id: 'rider-b' }
const club = (n: number, name: string): ClubSummary => ({ id: `00000000-0000-4000-8000-${String(n).padStart(12, '0')}`, name, city: 'Almaty', description: 'Community rides', status: 'active' })
const response = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json' } })
const parsed = (url: string) => new URL(url, 'https://bikers.kz')
const isClubList = (url: string) => parsed(url).pathname === '/api/v1/clubs'
function ancillary(url: string) {
  if (url.endsWith('/cities')) return response({ cities: [{ value: 'Almaty', names: { en: 'Almaty', ru: 'Алматы', kk: 'Алматы' }, aliases: ['Алматы'] }, { value: 'Astana', names: { en: 'Astana', ru: 'Астана', kk: 'Астана' }, aliases: [] }] })
  if (url.endsWith('/image')) return response({ image: null, version: 1 })
  return undefined
}
function Controls() {
  const location = useLocation(), navigate = useNavigate(), session = useSession()
  return <><output data-testid="location">{location.pathname + location.search}</output><button onClick={() => void navigate(-1)}>History back</button><button onClick={() => void navigate(1)}>History forward</button><button onClick={() => session.accept({ user: nextRider, csrf_token: 'csrf-b' })}>Switch account</button></>
}
function mount(path: string) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
  render(<QueryClientProvider client={client}><MemoryRouter initialEntries={[path]}><I18nProvider><SessionProvider><Routes><Route path="/clubs" element={<ClubsPage />} /><Route path="/my-clubs" element={<MyClubsPage />} /><Route path="/login" element={<p>Sign in test page</p>} /></Routes><Controls /></SessionProvider></I18nProvider></MemoryRouter></QueryClientProvider>)
  return client
}
beforeEach(() => setCsrfToken())
afterEach(() => { setCsrfToken(); vi.unstubAllGlobals() })

describe('club discovery and My clubs', () => {
  it('restores URL filters, searches on the server, and preserves browser history', async () => {
    const fetch = vi.fn(async (url: string) => {
      if (url.endsWith('/session')) return response({ user: rider, csrf_token: 'csrf-a' })
      if (isClubList(url)) return response({ clubs: [club(1, `Club in ${parsed(url).searchParams.get('city') ?? 'any city'}`)], next_cursor: '' })
      const extra = ancillary(url); if (extra) return extra
      throw new Error(`Unexpected request ${url}`)
    })
    vi.stubGlobal('fetch', fetch)
    mount('/clubs?city=Almaty&q=Touring&cursor=previous-page')
    await screen.findByText('Club in Almaty')
    expect(screen.getByLabelText('Search clubs')).toHaveValue('Touring')
    expect(screen.getByLabelText('City')).toHaveValue('Almaty')
    expect(Object.fromEntries(parsed(fetch.mock.calls.find(([url]) => isClubList(url))![0]).searchParams)).toMatchObject({ view: 'discover', city: 'Almaty', q: 'Touring', cursor: 'previous-page' })
    fireEvent.change(screen.getByLabelText('Search clubs'), { target: { value: 'City riders' } })
    fireEvent.change(screen.getByLabelText('City'), { target: { value: 'Astana' } })
    await userEvent.click(screen.getByRole('button', { name: 'Apply filters' }))
    await screen.findByText('Club in Astana')
    expect(Object.fromEntries(parsed(fetch.mock.calls.filter(([url]) => isClubList(url)).at(-1)![0]).searchParams)).toEqual({ view: 'discover', city: 'Astana', q: 'City riders' })
    await userEvent.click(screen.getByRole('button', { name: 'History back' }))
    await screen.findByText('Club in Almaty')
    expect(screen.getByLabelText('Search clubs')).toHaveValue('Touring')
    await userEvent.click(screen.getByRole('button', { name: 'History forward' }))
    await screen.findByText('Club in Astana')
    await userEvent.click(screen.getByRole('button', { name: 'Clear filters' }))
    await screen.findByText('Club in any city')
    expect(screen.getByLabelText('Search clubs')).toHaveValue('')
    expect(parsed(screen.getByTestId('location').textContent!).searchParams.has('cursor')).toBe(false)
  })

  it('pages and switches between organizing, joined and pending without reusing cursors', async () => {
    const fetch = vi.fn(async (url: string) => {
      if (url.endsWith('/session')) return response({ user: rider, csrf_token: 'csrf-a' })
      if (isClubList(url)) {
        const params = parsed(url).searchParams, second = !!params.get('cursor'), view = params.get('view')!
        return response({ clubs: [club(second ? 2 : 1, `${view} ${second ? 'second' : 'first'} page`)], next_cursor: view === 'organizing' && !second ? 'next-page' : '' })
      }
      const extra = ancillary(url); if (extra) return extra
      throw new Error(`Unexpected request ${url}`)
    })
    vi.stubGlobal('fetch', fetch)
    mount('/my-clubs?city=Almaty')
    await screen.findByText('organizing first page')
    await userEvent.click(screen.getByRole('button', { name: 'Next page' }))
    await screen.findByText('organizing second page')
    await userEvent.click(screen.getByRole('button', { name: 'Previous page' }))
    await screen.findByText('organizing first page')
    await userEvent.click(screen.getByRole('button', { name: 'Next page' }))
    await screen.findByText('organizing second page')
    for (const [label, view] of [['Joined', 'joined'], ['Pending', 'pending']] as const) {
      await userEvent.click(screen.getByRole('link', { name: label }))
      await screen.findByText(`${view} first page`)
      const params = parsed(screen.getByTestId('location').textContent!).searchParams
      expect(params.get('view')).toBe(view)
      expect(params.get('city')).toBe('Almaty')
      expect(params.has('cursor')).toBe(false)
    }
  })

  it('requires sign-in before requesting personal clubs and preserves the requested view', async () => {
    const fetch = vi.fn(async (url: string) => {
      if (url.endsWith('/session')) return response({ user: null })
      throw new Error(`Unexpected guest request ${url}`)
    })
    vi.stubGlobal('fetch', fetch)
    mount('/my-clubs?view=pending&city=Astana')
    await screen.findByText('Sign in test page')
    expect(fetch.mock.calls.some(([url]) => isClubList(url))).toBe(false)
    expect(parsed(screen.getByTestId('location').textContent!).searchParams.get('next')).toBe('/my-clubs?view=pending&city=Astana')
  })

  it('resets actor-bound paging and discards responses from a previous account', async () => {
    let current = rider
    const reads: { actor: string; cursor: string | null }[] = []
    let finishOld: (value: Response) => void = () => { throw new Error('No pending old response') }
    const oldPage = new Promise<Response>((resolve) => { finishOld = resolve })
    const fetch = vi.fn(async (url: string) => {
      if (url.endsWith('/session')) return response({ user: current, csrf_token: current.id })
      if (isClubList(url)) {
        const cursor = parsed(url).searchParams.get('cursor'); reads.push({ actor: current.id, cursor })
        if (current.id === rider.id && cursor) return oldPage
        return response({ clubs: [club(current.id === rider.id ? 1 : 2, current.id === rider.id ? 'Original account club' : 'New account club')], next_cursor: current.id === rider.id ? 'account-a-next' : '' })
      }
      const extra = ancillary(url); if (extra) return extra
      throw new Error(`Unexpected request ${url}`)
    })
    vi.stubGlobal('fetch', fetch)
    mount('/my-clubs?view=joined')
    await screen.findByText('Original account club')
    await userEvent.click(screen.getByRole('button', { name: 'Next page' }))
    await waitFor(() => expect(reads.some((read) => read.cursor === 'account-a-next')).toBe(true))
    current = nextRider
    await userEvent.click(screen.getByRole('button', { name: 'Switch account' }))
    await screen.findByText('New account club')
    await act(async () => { finishOld(response({ clubs: [club(3, 'Late private club')], next_cursor: '' })); await oldPage })
    expect(screen.queryByText('Late private club')).not.toBeInTheDocument()
    expect(reads.filter((read) => read.actor === nextRider.id)).toEqual([{ actor: nextRider.id, cursor: null }])
    expect(parsed(screen.getByTestId('location').textContent!).searchParams.has('cursor')).toBe(false)
  })

  it('offers first-page recovery when a saved cursor expires after restart', async () => {
    const fetch = vi.fn(async (url: string) => {
      if (url.endsWith('/session')) return response({ user: rider, csrf_token: 'csrf-a' })
      if (isClubList(url)) return parsed(url).searchParams.has('cursor') ? response({ error: { code: 'invalid_cursor', message: 'Restart this view' } }, 422) : response({ clubs: [club(1, 'First page recovered')], next_cursor: '' })
      const extra = ancillary(url); if (extra) return extra
      throw new Error(`Unexpected request ${url}`)
    })
    vi.stubGlobal('fetch', fetch)
    mount('/clubs?cursor=expired-token')
    await userEvent.click(await screen.findByRole('button', { name: 'Return to the first club page' }))
    await screen.findByText('First page recovered')
    expect(parsed(screen.getByTestId('location').textContent!).searchParams.has('cursor')).toBe(false)
  })
})
