import { render, screen, within } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { I18nProvider } from '../i18n'
import { SessionProvider } from '../session'
import type { Bike, User } from '../types'
import { GaragePage } from './Garage'

const rider: User = { id: 'rider-a', profile: { display_name: 'Rider A', city: 'Almaty', languages: ['en'], interests: '', locale: 'en' }, onboarding_complete: true }
const bike = (n: number, model: string, visibility: Bike['visibility']): Bike => ({ id: `00000000-0000-4000-8000-${String(n).padStart(12, '0')}`, owner_id: rider.id, make: 'Honda', model, model_id: '', year: 2024, visibility, version: 1, can_manage: true, created_at: '2026-11-03T00:00:00Z', updated_at: '2026-11-03T00:00:00Z' })
const response = (value: unknown) => new Response(JSON.stringify(value), { status: 200, headers: { 'Content-Type': 'application/json' } })
afterEach(() => vi.unstubAllGlobals())

describe('garage cards', () => {
  // Green marks approval only, so a public page and a private bike get the same neutral pill.
  it('shows visibility in a neutral pill with its own icon', async () => {
    vi.stubGlobal('fetch', vi.fn(async (url: string) => {
      if (url.endsWith('/session')) return response({ user: rider, csrf_token: 'csrf-a' })
      if (url.endsWith('/me/bikes')) return response({ bikes: [bike(1, 'Shared bike', 'public'), bike(2, 'Hidden bike', 'private')], next_cursor: '' })
      if (url.endsWith('/photos')) return response({ photos: [], cover_photo_id: '' })
      throw new Error(`Unexpected request ${url}`)
    }))
    const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
    render(<QueryClientProvider client={client}><MemoryRouter><I18nProvider><SessionProvider><GaragePage /></SessionProvider></I18nProvider></MemoryRouter></QueryClientProvider>)
    const cards = await screen.findAllByRole('article')
    const pills = [['Shared bike', 'Public basic page'], ['Hidden bike', 'Private']].map(([model, label]) => {
      const card = cards.find((item) => within(item).queryByText(model))!
      return within(card).getByText(label)
    })
    for (const pill of pills) {
      expect(pill).toHaveClass('status-pill', { exact: true })
      expect(pill.querySelector('svg')).not.toBeNull()
    }
    // The globe and the lock tell the two apart.
    expect(pills[0].querySelector('svg')!.innerHTML).not.toBe(pills[1].querySelector('svg')!.innerHTML)
  })
})
