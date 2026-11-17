import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Link, MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { setCsrfToken } from '../api'
import { I18nProvider } from '../i18n'
import { PlaceInformation } from '../components/places/shared'
import { RequireUser, SessionProvider } from '../session'
import type { Place, PlaceInput, PlaceSubmission, PlaceSubmissionInput, User } from '../types'
import { AdminPage } from './Admin'
import { PlaceSubmissionPage } from './PlaceSubmissions'

const rider: User = { id: 'camera-test-rider', profile: { display_name: 'Test rider', city: 'Almaty', languages: ['en'], interests: '', locale: 'en' }, onboarding_complete: true }
const camera: PlaceInput = { name: 'Camera test fixture', category: 'speed_camera', lat: 43.2, lon: 76.9, city: '', address: '', description: '', website: '', phone: '', opening_hours: '' }
const published: Place = { ...camera, id: 'camera-test', source: 'community', source_url: '', updated_at: '2026-11-17T00:00:00Z', version: 3, status: 'published' }
const response = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json' } })

function mount(path: string) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
  render(<QueryClientProvider client={client}><MemoryRouter initialEntries={[path]}><I18nProvider><SessionProvider><Routes>
    <Route path="/places/new" element={<RequireUser><PlaceSubmissionPage /></RequireUser>} />
    <Route path="/places/:placeId/suggest" element={<RequireUser><PlaceSubmissionPage /></RequireUser>} />
    <Route path="/admin" element={<RequireUser><AdminPage /></RequireUser>} />
  </Routes><Link to="/admin">Review test suggestions</Link></SessionProvider></I18nProvider></MemoryRouter></QueryClientProvider>)
}

beforeEach(() => setCsrfToken())
afterEach(() => { setCsrfToken(); vi.unstubAllGlobals() })

describe('speed-camera suggestions', () => {
  it.each(['new', 'correction'] as const)('preserves the camera category through the existing %s submission and approval flow', async (kind) => {
    let sent: PlaceSubmissionInput | undefined
    let pending: PlaceSubmission | undefined
    let review: Record<string, unknown> | undefined
    vi.stubGlobal('fetch', vi.fn(async (raw: string, init?: RequestInit) => {
      const path = new URL(raw, 'https://bikers.kz').pathname
      if (path === '/api/v1/session') return response({ user: rider, csrf_token: 'camera-test-csrf' })
      if (path === '/api/v1/cities') return response({ cities: [] })
      if (path === '/api/v1/places/config' || path === '/api/v1/geo/config') return response({ enabled: false, archive_url: '' })
      if (path === '/api/v1/me/trust') return response({ is_admin: true })
      if (path === '/api/v1/admin/reports') return response({ reports: [], next_cursor: '' })
      if (path === '/api/v1/places/camera-test') return response({ place: published })
      if (path === '/api/v1/places/submissions' && init?.method === 'POST') {
        sent = JSON.parse(String(init.body)) as PlaceSubmissionInput
        pending = { ...sent, id: 'camera-test-submission', status: 'pending', version: 1, created_at: published.updated_at, updated_at: published.updated_at }
        return response({ submission: pending }, 201)
      }
      if (path === '/api/v1/admin/place-submissions') return response({ submissions: pending ? [pending] : [], next_cursor: '' })
      if (path === '/api/v1/admin/place-submissions/camera-test-submission/review' && init?.method === 'POST') {
        review = JSON.parse(String(init.body)) as Record<string, unknown>
        return response({ submission: { ...pending, status: 'approved' } })
      }
      throw new Error(`Unexpected camera test request ${path}`)
    }))
    mount(kind === 'new' ? '/places/new' : '/places/camera-test/suggest?kind=correction')
    const actor = userEvent.setup()
    const name = await screen.findByRole('textbox', { name: /Place name/ })
    if (kind === 'new') {
      await actor.type(name, camera.name)
      await actor.selectOptions(screen.getByRole('combobox', { name: /Category/ }), 'speed_camera')
      await actor.type(screen.getByRole('spinbutton', { name: /Latitude/ }), String(camera.lat))
      await actor.type(screen.getByRole('spinbutton', { name: /Longitude/ }), String(camera.lon))
    } else {
      expect(name).toHaveValue(camera.name)
      expect(screen.getByRole('combobox', { name: /Category/ })).toHaveValue('speed_camera')
    }
    await actor.type(screen.getByRole('textbox', { name: /What should change/ }), 'Camera category evidence in test fixture')
    await actor.click(screen.getByRole('button', { name: 'Send for review' }))
    await screen.findByRole('heading', { name: 'Suggestion sent for review' })
    expect(sent).toMatchObject({ kind, place: { name: camera.name, category: 'speed_camera' } })
    if (kind === 'correction') expect(sent).toMatchObject({ place_id: published.id, base_version: published.version })
    expect(sent?.place).not.toHaveProperty('source')

    await actor.click(screen.getByRole('link', { name: 'Review test suggestions' }))
    await actor.click(await screen.findByRole('button', { name: 'Map' }))
    await actor.click(await screen.findByRole('button', { name: new RegExp(camera.name) }))
    const proposal = screen.getByRole('heading', { name: 'Proposed information' }).closest('section')!
    expect(within(proposal).getByText('Speed cameras')).toBeInTheDocument()
    await actor.type(screen.getByRole('textbox', { name: 'Review reason' }), 'Reviewed camera test evidence')
    await actor.click(screen.getByRole('button', { name: 'Approve suggestion' }))
    await waitFor(() => expect(review).toEqual({ version: 1, status: 'approved', reason: 'Reviewed camera test evidence' }))
    expect(pending?.place?.category).toBe('speed_camera')
  })

  it.each([
    ['en', 'Speed cameras'],
    ['ru', 'Камеры контроля скорости'],
    ['kk', 'Жылдамдықты бақылау камералары'],
  ])('uses the %s camera category label in shared place information', (locale, label) => {
    localStorage.setItem('bikerskz.locale', locale)
    render(<I18nProvider><PlaceInformation place={camera} /></I18nProvider>)
    expect(screen.getByText(label)).toBeInTheDocument()
  })
})
