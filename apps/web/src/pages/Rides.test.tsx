import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { setCsrfToken } from '../api'
import { I18nProvider } from '../i18n'
import { RequireUser, SessionProvider } from '../session'
import type { CommunityRoute } from '../routes'
import { RideFormPage } from './Rides'
import type { Ride } from '../types'

vi.mock('../components/maps/MapCanvas', () => ({ default: () => <div /> }))

const sourceId = '00000000-0000-4000-8000-000000000001'
const previewDigest = 'a'.repeat(64)
const points = [
  { lat: 51.15, lon: 71.43, label: 'North meeting point' },
  { lat: 51.18, lon: 71.39, label: 'Rest stop' },
  { lat: 51.2, lon: 71.46, label: 'End point' },
]
const source: CommunityRoute = {
  id: sourceId, owner_id: 'owner', owner_name: 'Route author', title: 'Weekend loop', city: 'Astana', description: 'Source description',
  surface: 'asphalt', kind: 'planned', published: true, version: 1, created_at: '', updated_at: '', distance_meters: 6000, duration_seconds: 600,
  points, segments: [[[71.43, 51.15], [71.39, 51.18], [71.46, 51.2]]],
}

function response(value: unknown, status = 200) {
  return new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json' } })
}

function mount(sourceResponse: () => Response, ride?: Ride) {
  const fetch = vi.fn(async (url: string, options?: RequestInit) => {
    if (url.endsWith('/session')) return response({ user: { id: 'rider', onboarding_complete: true, profile: { display_name: 'Ride host', city: 'Almaty', locale: 'en' } }, csrf_token: 'test' })
    if (ride && url === `/api/v1/rides/${ride.id}`) return response({ ride })
    if (ride && url === `/api/v1/rides/${ride.id}/cover`) return response({ image: null, version: 1 })
    if (url === `/api/v1/routes/${sourceId}?scope=public`) return sourceResponse()
    if (url.endsWith('/geo/config')) return response({ enabled: true, archive_url: '/map-data/test.pmtiles' })
    if (url.endsWith('/geo/route')) return response({ plan: { points, geometry: { type: 'LineString', coordinates: [[71.43, 51.15], [71.44, 51.17], [71.46, 51.2]] }, distance_meters: 7000, duration_seconds: 780 }, preview_digest: previewDigest })
    if (url.endsWith('/rides') && options?.method === 'POST') return response({ ride: { id: 'new-ride' } }, 201)
    return response({}, 404)
  })
  vi.stubGlobal('fetch', fetch)
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
  render(<QueryClientProvider client={client}><MemoryRouter initialEntries={[ride ? `/rides/${ride.id}/edit` : `/rides/new?route=${sourceId}`]}><I18nProvider><SessionProvider><Routes>
    <Route path="/rides/new" element={<RequireUser><RideFormPage /></RequireUser>} />
    <Route path="/rides/:rideId/edit" element={<RequireUser><RideFormPage edit /></RequireUser>} />
    <Route path="/rides/:rideId" element={<p>Ride published</p>} />
  </Routes></SessionProvider></I18nProvider></MemoryRouter></QueryClientProvider>)
  return fetch
}

beforeEach(() => { localStorage.setItem('locale', 'en') })
afterEach(() => { vi.unstubAllGlobals(); setCsrfToken() })

describe('new ride from a public community route', () => {
  it('prefills stops, requires an up-to-date preview and confirmation, and posts an independent ride plan', async () => {
    const fetch = mount(() => response({ route: source }))
    const rider = userEvent.setup()
    expect(await screen.findByDisplayValue('Astana')).toBeInTheDocument()
    expect(screen.getByDisplayValue('Weekend loop')).toBeInTheDocument()
    expect(screen.getByDisplayValue('Rest stop')).toBeInTheDocument()
    fireEvent.change(screen.getByLabelText(/Public description/), { target: { value: 'About two hours on paved roads' } })
    const date = screen.getByLabelText(/Date and time/)
    fireEvent.change(date, { target: { value: '2099-05-01T09:00' } })
    const confirm = screen.getByRole('checkbox', { name: /I checked the route/ })
    expect(confirm).toBeDisabled()
    await rider.click(screen.getByRole('button', { name: 'Publish ride' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Calculate the route and confirm')
    await rider.click(screen.getByRole('button', { name: 'Calculate motorcycle route' }))
    expect(await screen.findByText(/Solid orange line/)).toBeInTheDocument()
    await rider.click(confirm)
    expect(confirm).toBeChecked()
    fireEvent.change(date, { target: { value: '2099-05-01T09:30' } })
    expect(confirm).not.toBeChecked()
    await rider.click(confirm)
    await rider.click(screen.getByRole('button', { name: 'Publish ride' }))
    expect(await screen.findByText('Ride published')).toBeInTheDocument()
    const create = fetch.mock.calls.find(([url]) => url === '/api/v1/rides')
    const body = JSON.parse(create?.[1]?.body as string)
    expect(body).toMatchObject({ city: 'Astana', title: 'Weekend loop', starts_at: '2099-05-01T04:30:00.000Z', meeting_point: points[0].label, map_points: points, expected_map_digest: previewDigest, distance_km: 0, publish_public_details: true })
    expect(body).not.toHaveProperty('route_id')
    expect(body).not.toHaveProperty('geometry')
  })

  it.each([{ ...source, published: false }, { ...source, kind: 'recorded' }])('rejects a source outside the published planned route contract', async (route) => {
    mount(() => response({ route }))
    expect(await screen.findByRole('alert')).toHaveTextContent('This planned route is unavailable')
    expect(screen.getByRole('link', { name: 'Start with a blank ride' })).toHaveAttribute('href', '/rides/new')
    await waitFor(() => expect(screen.queryByDisplayValue(source.title)).not.toBeInTheDocument())
  })
})

describe('editing ride publication and distance', () => {
  const original: Ride = { id: sourceId, city: 'Astana', date: '2099-05-01', title: 'Private route title', description: 'Always public description', status: 'published', can_manage: true, organizer_id: 'rider', starts_at: '2099-05-01T04:00:00Z', timezone: 'Asia/Almaty', meeting_point: points[0].label, destination: points.at(-1)!.label, capacity: 8, approved_count: 3, plan_version: 1, distance_km: 100, distance_override_km: 0, public_details_published: false, map_plan: { points, distance_meters: 100000, duration_seconds: 600 } }

  it('keeps legacy expanded details private on ordinary edits and leaves an automatic distance unset', async () => {
    const fetch = mount(() => response({ route: source }), original)
    const title = await screen.findByLabelText(/Ride title/)
    const publish = screen.getByRole('checkbox', { name: 'Publish expanded ride details' })
    expect(publish).not.toBeChecked()
    expect(screen.getByLabelText(/Approx. distance/)).toHaveValue(null)
    const preview = screen.getByRole('heading', { name: 'What guests see' }).parentElement!
    expect(preview).not.toHaveTextContent('Private route title')
    expect(preview).not.toHaveTextContent('Ride host')
    expect(preview).not.toHaveTextContent('3/8 spots')
    expect(preview).toHaveTextContent('Always public description')
    fireEvent.change(title, { target: { value: 'Still private title' } })
    await userEvent.click(screen.getByRole('button', { name: 'Save plan' }))
    await screen.findByText('Ride published')
    const patch = fetch.mock.calls.find(([, options]) => options?.method === 'PATCH')
    expect(JSON.parse(patch![1]!.body as string)).toMatchObject({ title: 'Still private title', publish_public_details: false, distance_km: 0 })
  })

  it('publishes expanded details only when selected and previews the organizer, coarse time and capacity', async () => {
    const fetch = mount(() => response({ route: source }), original)
    await screen.findByLabelText(/Ride title/)
    await userEvent.click(screen.getByRole('checkbox', { name: 'Publish expanded ride details' }))
    const preview = screen.getByRole('heading', { name: 'What guests see' }).parentElement!
    expect(preview).toHaveTextContent('Private route title')
    expect(preview).toHaveTextContent('By Ride host')
    expect(preview).toHaveTextContent('Morning')
    expect(preview).toHaveTextContent('3/8 spots')
    expect(screen.getByRole('heading', { name: 'What approved riders see' }).parentElement!).toHaveTextContent('3/8 spots')
    const basics = screen.getByRole('heading', { name: 'Basics & vibe' }).closest('section')!
    expect(within(basics).getByLabelText(/Participant limit/)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Save plan' }))
    await screen.findByText('Ride published')
    const patch = fetch.mock.calls.find(([, options]) => options?.method === 'PATCH')
    expect(JSON.parse(patch![1]!.body as string)).toMatchObject({ publish_public_details: true, distance_km: 0 })
  })

  it('preserves a manual distance and lets a published ride return to automatic distance and private details', async () => {
    const fetch = mount(() => response({ route: source }), { ...original, distance_km: 175, distance_override_km: 175, public_details_published: true })
    const distance = await screen.findByLabelText(/Approx. distance/)
    expect(distance).toHaveValue(175)
    const publish = screen.getByRole('checkbox', { name: 'Publish expanded ride details' })
    expect(publish).toBeChecked()
    fireEvent.change(distance, { target: { value: '' } })
    await userEvent.click(publish)
    await userEvent.click(screen.getByRole('button', { name: 'Save plan' }))
    await screen.findByText('Ride published')
    const patch = fetch.mock.calls.find(([, options]) => options?.method === 'PATCH')
    expect(JSON.parse(patch![1]!.body as string)).toMatchObject({ publish_public_details: false, distance_km: 0 })
  })
})
