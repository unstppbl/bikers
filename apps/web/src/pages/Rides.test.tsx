import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { setCsrfToken } from '../api'
import { I18nProvider } from '../i18n'
import { RequireUser, SessionProvider } from '../session'
import type { CommunityRoute } from '../routes'
import { RideFormPage } from './Rides'

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

function mount(sourceResponse: () => Response) {
  const fetch = vi.fn(async (url: string, options?: RequestInit) => {
    if (url.endsWith('/session')) return response({ user: { id: 'rider', onboarding_complete: true, profile: { city: 'Almaty', locale: 'en' } }, csrf_token: 'test' })
    if (url === `/api/v1/routes/${sourceId}?scope=public`) return sourceResponse()
    if (url.endsWith('/geo/config')) return response({ enabled: true, archive_url: '/map-data/test.pmtiles' })
    if (url.endsWith('/geo/route')) return response({ plan: { points, geometry: { type: 'LineString', coordinates: [[71.43, 51.15], [71.44, 51.17], [71.46, 51.2]] }, distance_meters: 7000, duration_seconds: 780 }, preview_digest: previewDigest })
    if (url.endsWith('/rides') && options?.method === 'POST') return response({ ride: { id: 'new-ride' } }, 201)
    return response({}, 404)
  })
  vi.stubGlobal('fetch', fetch)
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
  render(<QueryClientProvider client={client}><MemoryRouter initialEntries={[`/rides/new?route=${sourceId}`]}><I18nProvider><SessionProvider><Routes>
    <Route path="/rides/new" element={<RequireUser><RideFormPage /></RequireUser>} />
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
    await rider.type(screen.getByLabelText(/Public description/), 'About two hours on paved roads')
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
    expect(body).toMatchObject({ city: 'Astana', title: 'Weekend loop', starts_at: '2099-05-01T04:30:00.000Z', meeting_point: points[0].label, map_points: points, expected_map_digest: previewDigest })
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
