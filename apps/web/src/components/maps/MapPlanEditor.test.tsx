import { useState } from 'react'
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { I18nProvider } from '../../i18n'
import { publicRide } from '../../api'
import type { MapPlan } from '../../types'
import { MapPlanEditor } from './MapPlanEditor'
import type { MapDraft } from './MapPlanEditor'

vi.mock('../../session', () => ({ useSession: () => ({ user: { id: 'map-planner-user' } }), useSessionGuard: () => () => () => true }))
vi.mock('./MapCanvas', () => ({ default: ({ onPick, geometry }: { onPick?: (point: { lat: number; lon: number }) => void; geometry?: unknown }) => <div><button type="button" onClick={() => onPick?.({ lat: 43.25, lon: 76.95 })}>Pick test pin</button><output data-testid="geometry">{geometry ? 'Route shown' : 'No route'}</output></div> }))

const first = { label: 'Meeting address', lat: 43.24, lon: 76.91 }
const last = { label: 'Destination address', lat: 43.26, lon: 76.94 }
const plan: MapPlan = { points: [first, last], geometry: { type: 'LineString', coordinates: [[76.91, 43.24], [76.94, 43.26]] }, distance_meters: 4200, duration_seconds: 900 }
const response = (data: unknown, status = 200) => new Response(JSON.stringify(data), { status, headers: { 'Content-Type': 'application/json' } })

function mount(initialPlan?: MapPlan, startWithMap = true) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
  function Editor() {
    const [meeting, setMeeting] = useState(initialPlan?.points[0].label ?? '')
    const [destination, setDestination] = useState(initialPlan?.points.at(-1)?.label ?? '')
    const [draft, setDraft] = useState<MapDraft>({ points: initialPlan?.points ?? [], valid: true })
    return <><MapPlanEditor initialPlan={initialPlan} meeting={meeting} destination={destination} onMeeting={setMeeting} onDestination={setDestination} onChange={setDraft} disabled={false} startWithMap={startWithMap} /><output data-testid="draft">{JSON.stringify(draft)}</output></>
  }
  render(<QueryClientProvider client={client}><I18nProvider><Editor /></I18nProvider></QueryClientProvider>)
  return client
}
const draft = () => JSON.parse(screen.getByTestId('draft').textContent!) as MapDraft
beforeEach(() => { localStorage.setItem('locale', 'en') })
afterEach(() => { vi.unstubAllGlobals() })

describe('private map planner', () => {
  it('supports text details when GIS is disabled', async () => {
    const fetch = vi.fn(async () => response({ enabled: false, archive_url: '' }))
    vi.stubGlobal('fetch', fetch)
    mount()
    fireEvent.change(screen.getByLabelText(/Meeting point/), { target: { value: 'Courtyard entrance' } })
    await waitFor(() => expect(fetch).toHaveBeenCalled())
    expect(screen.getByLabelText(/Meeting point/)).toHaveValue('Courtyard entrance')
    expect(screen.queryByRole('combobox')).not.toBeInTheDocument()
    expect(draft()).toEqual({ points: [], valid: true })
  })

  it('debounces address search and requires choosing a candidate', async () => {
    const fetch = vi.fn(async (url: string, options?: RequestInit) => url.includes('/geo/config') ? response({ enabled: true, archive_url: '/map-data/version/kazakhstan.pmtiles' }) : options?.method === 'POST' ? response({ places: [{ ...first, id: 'osm-1', kind: 'house' }] }) : response({}, 405))
    vi.stubGlobal('fetch', fetch)
    mount()
    const meeting = await screen.findByRole('combobox', { name: /Meeting point/ })
    fireEvent.focus(meeting)
    fireEvent.change(meeting, { target: { value: 'Meeting' } })
    expect(fetch.mock.calls.filter(([url]) => url.includes('/geo/search'))).toHaveLength(0)
    expect(draft().valid).toBe(false)
    const candidate = await screen.findByRole('option', { name: /Meeting address/ })
    await userEvent.click(candidate)
    await waitFor(() => expect(draft()).toEqual({ points: [first], valid: true }))
    expect(fetch.mock.calls.filter(([url]) => url.includes('/geo/search'))).toHaveLength(1)
    const call = fetch.mock.calls.find(([url]) => url.includes('/geo/search'))!
    expect(call[0]).toBe('/api/v1/geo/search')
    expect(JSON.parse(call[1]!.body as string)).toEqual({ query: 'Meeting', lang: 'en' })
  })

  it('ignores a stale search response after the query changes', async () => {
    let resolveOld: ((value: Response) => void) | undefined
    const fetch = vi.fn(async (url: string, options?: RequestInit) => {
      if (url.includes('/geo/config')) return response({ enabled: true, archive_url: '/map-data/version/kazakhstan.pmtiles' })
      if (JSON.parse(options?.body as string).query === 'Old') return new Promise<Response>((resolve) => { resolveOld = resolve })
      return response({ places: [{ ...first, label: 'New candidate', id: 'new', kind: 'house' }] })
    })
    vi.stubGlobal('fetch', fetch)
    mount()
    const meeting = await screen.findByRole('combobox', { name: /Meeting point/ })
    fireEvent.focus(meeting); fireEvent.change(meeting, { target: { value: 'Old' } })
    await waitFor(() => expect(resolveOld).toBeDefined())
    fireEvent.change(meeting, { target: { value: 'New' } })
    await act(async () => resolveOld?.(response({ places: [{ ...first, label: 'Old candidate', id: 'old', kind: 'house' }] })))
    expect(screen.queryByRole('option', { name: /Old candidate/ })).not.toBeInTheDocument()
    expect(await screen.findByRole('option', { name: /New candidate/ })).toBeInTheDocument()
  })

  it('keeps the chosen pin coordinates instead of the reverse result center', async () => {
    const fetch = vi.fn(async (url: string) => url.includes('/geo/config') ? response({ enabled: true, archive_url: '/map-data/version/kazakhstan.pmtiles' }) : response({ places: [{ id: 'city', kind: 'city', label: 'Almaty', lat: 43.0, lon: 77.0 }] }))
    vi.stubGlobal('fetch', fetch)
    mount()
    await userEvent.click(await screen.findByRole('button', { name: 'Pick test pin' }))
    await waitFor(() => expect(draft()).toEqual({ points: [{ label: 'Almaty', lat: 43.25, lon: 76.95 }], valid: true }))
  })

  it('does not let a delayed reverse lookup overwrite a newer confirmed address', async () => {
    let resolveReverse: ((value: Response) => void) | undefined
    vi.stubGlobal('fetch', vi.fn(async (url: string) => {
      if (url.includes('/geo/config')) return response({ enabled: true, archive_url: '/map-data/version/kazakhstan.pmtiles' })
      if (url.includes('/geo/reverse')) return new Promise<Response>((resolve) => { resolveReverse = resolve })
      return response({ places: [{ ...first, id: 'new', kind: 'house' }] })
    }))
    mount()
    await userEvent.click(await screen.findByRole('button', { name: 'Pick test pin' }))
    await waitFor(() => expect(resolveReverse).toBeDefined())
    const meeting = screen.getByRole('combobox', { name: /Meeting point/ })
    fireEvent.focus(meeting); fireEvent.change(meeting, { target: { value: 'Meeting' } })
    await userEvent.click(await screen.findByRole('option', { name: /Meeting address/ }))
    await act(async () => resolveReverse?.(response({ places: [{ label: 'Old pin address', lat: 43.0, lon: 77.0, id: 'old', kind: 'city' }] })))
    await waitFor(() => expect(draft()).toEqual({ points: [first], valid: true }))
    expect(meeting).toHaveValue(first.label)
  })

  it('does not retain old route geometry when points are edited, and explicitly clears maps in text mode', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => response({ enabled: true, archive_url: '/map-data/version/kazakhstan.pmtiles' })))
    mount(plan)
    expect(await screen.findByText('Route shown')).toBeInTheDocument()
    fireEvent.change(screen.getByRole('combobox', { name: /Destination/ }), { target: { value: 'A different destination' } })
    await waitFor(() => expect(screen.getByTestId('geometry')).toHaveTextContent('No route'))
    expect(draft().valid).toBe(false)
    await userEvent.click(screen.getByRole('button', { name: 'Use text details' }))
    await waitFor(() => expect(draft()).toEqual({ points: [], valid: true }))
  })

  it('calculates only selected points and never accepts client route geometry as input', async () => {
    const fetch = vi.fn(async (url: string, options?: RequestInit) => url.includes('/geo/config') ? response({ enabled: true, archive_url: '/map-data/version/kazakhstan.pmtiles' }) : options?.method === 'POST' ? response({ plan }) : response({ places: [] }))
    vi.stubGlobal('fetch', fetch)
    mount(plan)
    await userEvent.click(await screen.findByRole('button', { name: 'Calculate motorcycle route' }))
    await waitFor(() => expect(fetch.mock.calls.some(([, options]) => options?.method === 'POST')).toBe(true))
    const call = fetch.mock.calls.find(([, options]) => options?.method === 'POST')!
    expect(JSON.parse(call[1]!.body as string)).toEqual({ points: plan.points, lang: 'en' })
    expect(await screen.findByText('4.2 km')).toBeInTheDocument()
  })

  it('keeps map points and route geometry out of public ride projections', () => {
    const safe = publicRide({ id: 'r', city: 'Almaty', date: '2026-12-01', description: 'Public preview', status: 'published', map_plan: plan })
    expect(safe).not.toHaveProperty('map_plan')
    expect(JSON.stringify(safe)).not.toContain(first.label)
  })
})
