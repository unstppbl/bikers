import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { I18nProvider } from '../i18n'
import { ModelReference, selectModelReference } from './ModelReference'
import type { BikeModel, BikeModelImage } from '../types'

const image: BikeModelImage = { url: '/bike-models/honda-cb500x-2019.jpg', year_from: 2019, year_to: 2019, width: 1200, height: 800, source_url: 'https://example.org/honda-cb500x-2019', author: 'Reference Photographer', license: 'CC BY-SA 4.0', license_url: 'https://creativecommons.org/licenses/by-sa/4.0/' }
const model: BikeModel = { id: 'honda-cb500x', make: 'Honda', model: 'CB500X', aliases: ['Хонда CB500X'], images: [image] }
const identity = { model_id: model.id, make: model.make, model: model.model, year: 2019 }
const response = (models: BikeModel[]) => new Response(JSON.stringify({ models }), { status: 200 })

function renderReference(bike = identity, showUnavailable = false) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
  const tree = (value: typeof identity) => <QueryClientProvider client={client}><I18nProvider><ModelReference bike={value} variant={showUnavailable ? 'form' : 'card'} showUnavailable={showUnavailable} /></I18nProvider></QueryClientProvider>
  const view = render(tree(bike))
  return { client, ...view, change: (value: typeof identity) => view.rerender(tree(value)) }
}
afterEach(() => { vi.unstubAllGlobals() })

describe('verified model-year selection', () => {
  it('requires the exact canonical ID and a covered year without nearby-year fallback', () => {
    expect(selectModelReference([model], model.id, 2019, 2027)?.image).toEqual(image)
    for (const year of [2018, 2020, 1884, 2028, 2019.5, Number.NaN, Number.POSITIVE_INFINITY]) {
      expect(selectModelReference([model], model.id, year, 2027)).toBeUndefined()
    }
    expect(selectModelReference([model], 'another-cb500x-generation', 2019, 2027)).toBeUndefined()
    expect(selectModelReference([model], '', 2019, 2027)).toBeUndefined()
    expect(selectModelReference([{ ...model, images: [] }], model.id, 2019, 2027)).toBeUndefined()
  })

  it('uses an explicitly verified generation range and prefers a narrower exact-year image', () => {
    const generation = { ...image, url: '/bike-models/honda-cb500x-generation.jpg', year_from: 2018, year_to: 2020 }
    expect(selectModelReference([{ ...model, images: [generation] }], model.id, 2019, 2027)?.image).toEqual(generation)
    expect(selectModelReference([{ ...model, images: [generation, image] }], model.id, 2019, 2027)?.image).toEqual(image)
    expect(selectModelReference([{ ...model, images: [generation] }], model.id, 2021, 2027)).toBeUndefined()
  })

  it('rejects external image fetches, path traversal, missing attribution and malformed bounds', () => {
    const invalid: BikeModelImage[] = [
      { ...image, url: 'https://example.org/reference.jpg' },
      { ...image, url: '//example.org/reference.jpg' },
      { ...image, url: '/bike-models/../reference.jpg' },
      { ...image, url: '/bike-models/reference.jpg?proxy=external' },
      { ...image, year_from: 2020, year_to: 2019 },
      { ...image, width: 0 },
      { ...image, source_url: 'javascript:alert(1)' },
      { ...image, author: '' },
      { ...image, license_url: '' },
    ]
    for (const candidate of invalid) expect(selectModelReference([{ ...model, images: [candidate] }], model.id, 2019, 2027)).toBeUndefined()
  })
})

describe('automatic references and attribution', () => {
  it('uses only public metadata and local images with visible author, source and license', async () => {
    const fetch = vi.fn().mockResolvedValue(response([model]))
    vi.stubGlobal('fetch', fetch)
    const { client } = renderReference()
    const photo = await screen.findByRole('img', { name: 'Reference image of Honda CB500X, 2019' })
    expect(photo).toHaveAttribute('src', image.url)
    expect(photo).toHaveAttribute('width', '1200')
    expect(screen.getByText('Model reference · 2019')).toBeInTheDocument()
    expect(screen.getByText('Photo by Reference Photographer')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Source' })).toHaveAttribute('href', image.source_url)
    expect(screen.getByRole('link', { name: image.license })).toHaveAttribute('href', image.license_url)
    expect(fetch.mock.calls[0][0]).toBe('/api/v1/bike-models?query=Honda%20CB500X')
    expect(client.getQueryCache().find({ queryKey: ['bike-model-reference', 'Honda', 'CB500X'] })?.meta?.private).toBe(false)
  })

  it('does not look up freeform identities or invalid years', () => {
    const fetch = vi.fn()
    vi.stubGlobal('fetch', fetch)
    const view = renderReference({ ...identity, model_id: '' }, true)
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
    expect(screen.queryByText(/No verified reference/)).not.toBeInTheDocument()
    view.change({ ...identity, year: 0 })
    expect(fetch).not.toHaveBeenCalled()
  })

  it('clears the old image immediately when the year becomes unsupported or the identity changes', async () => {
    const otherModel: BikeModel = { id: 'ktm-390-duke', make: 'KTM', model: '390 Duke', aliases: [], images: [{ ...image, url: '/bike-models/ktm-390-duke-2018.jpg', year_from: 2018, year_to: 2018 }] }
    vi.stubGlobal('fetch', vi.fn(async () => response([model, otherModel])))
    const view = renderReference(identity, true)
    await screen.findByRole('img', { name: /Honda/ })
    view.change({ ...identity, year: 2020 })
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
    expect(screen.getByText('No verified reference image is available for this model and year.')).toBeInTheDocument()
    view.change({ ...identity, model_id: '' })
    expect(screen.queryByText(/No verified reference/)).not.toBeInTheDocument()
    view.change({ model_id: otherModel.id, make: otherModel.make, model: otherModel.model, year: 2018 })
    expect(screen.queryByRole('img', { name: /Honda/ })).not.toBeInTheDocument()
    expect(await screen.findByRole('img', { name: /KTM/ })).toHaveAttribute('src', '/bike-models/ktm-390-duke-2018.jpg')
  })

  it('removes a broken card photo and its frame/credits, then allows a new valid selection', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(response([model])))
    const view = renderReference()
    fireEvent.error(await screen.findByRole('img'))
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
    expect(view.container.querySelector('.model-reference-frame')).not.toBeInTheDocument()
    expect(screen.queryByText('Photo by Reference Photographer')).not.toBeInTheDocument()
    view.change({ ...identity, year: 2020 })
    view.change(identity)
    await screen.findByRole('img')
  })

  it('shows unavailable help only in a valid known-model form, including after a broken image', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(response([model])))
    const view = renderReference(identity, true)
    fireEvent.error(await screen.findByRole('img'))
    expect(screen.getByText('No verified reference image is available for this model and year.')).toBeInTheDocument()
    view.change({ ...identity, model_id: 'unknown-id' })
    expect(screen.queryByText(/No verified reference/)).not.toBeInTheDocument()
    view.change({ ...identity, year: Number.NaN })
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
    expect(screen.queryByText(/No verified reference/)).not.toBeInTheDocument()
  })

  it.each([['ru', 'Изображение модели · 2019', 'Источник'], ['kk', 'Модельдің анықтамалық суреті · 2019', 'Дереккөз']])('localizes the reference caption in %s', async (locale, label, source) => {
    localStorage.setItem('bikerskz.locale', locale)
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(response([model])))
    renderReference()
    await waitFor(() => expect(screen.getByText(label)).toBeInTheDocument())
    expect(screen.getByRole('link', { name: source })).toHaveAttribute('href', image.source_url)
  })
})
