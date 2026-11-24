import { fireEvent, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { setCsrfToken } from '../api'
import { I18nProvider } from '../i18n'
import type { RidePoll } from '../ride-polls'
import { RequireUser, SessionProvider } from '../session'
import { NewRidePollPage, RidePollPage } from './RidePolls'
import { RideFormPage } from './Rides'

const pollPath = '/ride-polls/poll-1'
const poll: RidePoll = {
  id: 'poll-1',
  city: 'Astana',
  description: 'Easy paved loop with a café break',
  timezone: 'Asia/Almaty',
  options: [
    { id: 'one', starts_at: '2099-06-05T03:00:00Z', votes: 1 },
    { id: 'two', starts_at: '2099-06-06T04:30:00Z', votes: 2 },
  ],
  my_option_ids: [],
  can_manage: false,
}

function json(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
}

function setup(path: string, initial: RidePoll, host = initial.can_manage) {
  let current = initial
  const writes: { url: string; method?: string; body: unknown }[] = []
  const fetch = vi.fn(async (url: string, options?: RequestInit) => {
    if (url === '/api/v1/session') return json(host
      ? { user: { id: 'host', onboarding_complete: true, profile: { city: 'Almaty', locale: 'en' } }, csrf_token: 'csrf-host' }
      : { user: null })
    if (url === '/api/v1/cities') return json({ cities: [] })
    if (url.endsWith('/geo/config')) return json({ enabled: false, archive_url: '' })
    if (url === '/api/v1' + pollPath) return json({ poll: current })
    if (options?.method && options.method !== 'GET') {
      const body = JSON.parse(String(options.body))
      writes.push({ url, method: options.method, body })
      if (url.endsWith('/vote')) current = { ...current, my_option_ids: body.option_ids }
      if (url.endsWith('/selection')) current = { ...current, selected_option_id: body.option_id }
      return json({ poll: current }, url === '/api/v1/ride-polls' ? 201 : 200)
    }
    return json({}, 404)
  })
  vi.stubGlobal('fetch', fetch)
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
  render(<QueryClientProvider client={client}><MemoryRouter initialEntries={[path]}><I18nProvider><SessionProvider><Routes>
    <Route path="/ride-polls/new" element={<RequireUser><NewRidePollPage /></RequireUser>} />
    <Route path="/ride-polls/:pollId" element={<RidePollPage />} />
    <Route path="/rides/new" element={<RequireUser><RideFormPage /></RequireUser>} />
    <Route path="/login" element={<h1>Sign in test</h1>} />
  </Routes></SessionProvider></I18nProvider></MemoryRouter></QueryClientProvider>)
  return { writes, fetch, rider: userEvent.setup() }
}

beforeEach(() => { localStorage.setItem('bikerskz.locale', 'en') })
afterEach(() => { vi.unstubAllGlobals(); setCsrfToken() })

describe('ride time polls', () => {
  it('starts a separate poll from the ride form using two or three distinct future Almaty times', async () => {
    const { writes, rider } = setup('/rides/new', { ...poll, can_manage: true })
    await rider.click(await screen.findByRole('link', { name: 'Poll for a time' }))
    await screen.findByRole('heading', { name: 'Find a time to ride' })
    fireEvent.change(screen.getByRole('combobox', { name: 'City' }), { target: { value: 'Astana' } })
    await rider.type(screen.getByRole('textbox', { name: 'Ride proposal' }), poll.description)
    fireEvent.change(screen.getByLabelText(/Option 1/), { target: { value: '2099-06-05T08:00' } })
    fireEvent.change(screen.getByLabelText(/Option 2/), { target: { value: '2099-06-05T08:00' } })
    await rider.click(screen.getByRole('button', { name: 'Create poll' }))
    expect(screen.getByRole('alert')).toHaveTextContent('Choose two or three different future dates and times.')
    expect(writes).toEqual([])
    fireEvent.change(screen.getByLabelText(/Option 2/), { target: { value: '2099-06-06T09:30' } })
    await rider.click(screen.getByRole('button', { name: 'Add a third option' }))
    fireEvent.change(screen.getByLabelText(/Option 3/), { target: { value: '2099-06-09T07:15' } })
    await rider.click(screen.getByRole('button', { name: 'Create poll' }))
    await screen.findByRole('heading', { name: 'Ride time poll' })
    expect(writes).toEqual([{ url: '/api/v1/ride-polls', method: 'POST', body: {
      city: 'Astana', description: poll.description, options: ['2099-06-05T03:00:00.000Z', '2099-06-06T04:30:00.000Z', '2099-06-09T02:15:00.000Z'],
    } }])
  })

  it('shares the link, and lets a guest vote for multiple choices and clear the vote', async () => {
    const { writes, rider } = setup(pollPath, poll)
    await screen.findByRole('heading', { name: 'Ride time poll' })
    await rider.click(screen.getByText('Share poll'))
    const telegram = screen.getByRole('link', { name: 'Share to Telegram' })
    expect(new URL(telegram.getAttribute('href') ?? '').searchParams.get('url')).toBe(window.location.origin + pollPath)
    expect(screen.getByRole('button', { name: 'Copy link' })).toBeInTheDocument()
    const [one, two] = screen.getAllByRole('checkbox')
    await rider.click(one)
    await rider.click(two)
    await rider.click(screen.getByRole('button', { name: 'Save vote' }))
    expect(await screen.findByText('Vote saved on this browser.')).toBeInTheDocument()
    expect(one).toBeChecked()
    expect(two).toBeChecked()
    await rider.click(screen.getByRole('button', { name: 'Clear my vote' }))
    expect(await screen.findByText('Your vote was cleared.')).toBeInTheDocument()
    expect(writes).toEqual([
      { url: '/api/v1' + pollPath + '/vote', method: 'PUT', body: { option_ids: ['one', 'two'] } },
      { url: '/api/v1' + pollPath + '/vote', method: 'PUT', body: { option_ids: [] } },
    ])
  })

  it('asks the host before closing voting and opens an unpublished ride form with the selected time', async () => {
    const { writes, rider } = setup(pollPath, { ...poll, can_manage: true })
    const choices = await screen.findAllByRole('radio')
    await rider.click(choices[1])
    await rider.click(screen.getByRole('button', { name: 'Confirm time' }))
    const dialog = within(await screen.findByRole('dialog', { name: 'Confirm time' }))
    expect(dialog.getByText('Close voting and confirm this time?')).toBeInTheDocument()
    await rider.click(dialog.getByRole('button', { name: 'Cancel' }))
    expect(writes).toEqual([])
    await rider.click(screen.getByRole('button', { name: 'Confirm time' }))
    await rider.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Confirm time' }))
    await rider.click(await screen.findByRole('link', { name: 'Continue to ride form' }))
    expect(await screen.findByDisplayValue('Astana')).toBeInTheDocument()
    expect(screen.getByLabelText(/Public description/)).toHaveValue(poll.description)
    expect(screen.getByLabelText(/Date and time/)).toHaveValue('2099-06-06T09:30')
    expect(screen.getByLabelText(/Ride title/)).toHaveValue('')
    expect(screen.getByRole('button', { name: 'Publish ride' })).toBeInTheDocument()
    expect(writes).toEqual([{ url: '/api/v1' + pollPath + '/selection', method: 'PUT', body: { option_id: 'two' } }])
  })

  it('keeps a closed poll readable for a guest without vote or prefill controls', async () => {
    setup(pollPath, { ...poll, selected_option_id: 'one' })
    expect(await screen.findByRole('heading', { name: 'Confirmed time' })).toBeInTheDocument()
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Continue to ride form' })).not.toBeInTheDocument()
  })

  it('only lets the selected poll’s host prefill a ride', async () => {
    setup('/rides/new?poll=poll-1', { ...poll, selected_option_id: 'one' }, true)
    expect(await screen.findByRole('alert')).toHaveTextContent('This poll has no future time confirmed by your account.')
    expect(screen.queryByRole('button', { name: 'Publish ride' })).not.toBeInTheDocument()
  })
})
