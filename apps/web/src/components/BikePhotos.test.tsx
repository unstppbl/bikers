import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { setCsrfToken } from '../api'
import { I18nProvider } from '../i18n'
import { SessionProvider, useSession } from '../session'
import type { BikeModel, BikePhoto, BikePhotoGallery, PublicBike, Session, User } from '../types'
import { BikeCover, BikePhotos, safeBikeGallery } from './BikePhotos'
import { photoFileError } from './PhotoInput'

const rider: User = { id: 'rider-a', profile: { display_name: 'Rider A', city: 'Almaty', languages: ['en'], interests: '', locale: 'en' }, onboarding_complete: true }
const other: User = { ...rider, id: 'rider-b' }
const bike: PublicBike = { id: '11111111-1111-4111-8111-111111111111', make: 'Honda', model: 'CB500X', model_id: 'honda-cb500x', year: 2019 }
const photo: BikePhoto = { id: '22222222-2222-4222-8222-222222222222', url: `/api/v1/bikes/${bike.id}/photos/22222222-2222-4222-8222-222222222222/content`, width: 1200, height: 800, created_at: '2026-11-03T10:00:00Z' }
const model: BikeModel = { id: bike.model_id, make: bike.make, model: bike.model, aliases: [], images: [{ url: '/bike-models/honda-cb500x-2019.jpg', year_from: 2019, year_to: 2019, width: 1200, height: 800, source_url: 'https://example.org/reference', author: 'Photographer', license: 'CC BY 4.0', license_url: 'https://creativecommons.org/licenses/by/4.0/' }] }
const response = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json' } })
const session = (user = rider): Session => ({ user, csrf_token: `csrf-${user.id}` })

function SessionControls() {
  const { clear, accept } = useSession()
  return <><button onClick={clear}>Expire test session</button><button onClick={() => accept(session(other))}>Switch test actor</button></>
}
function mount(owner = true) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: Infinity }, mutations: { retry: false } } })
  const view = render(<QueryClientProvider client={client}><I18nProvider><SessionProvider><SessionControls /><BikeCover bike={bike} /><BikePhotos bike={bike} owner={owner} /></SessionProvider></I18nProvider></QueryClientProvider>)
  return { client, ...view }
}
function defaults(url: string) {
  if (url.endsWith('/session')) return response(session())
  if (url.includes('/bike-models?')) return response({ models: [model] })
  throw new Error(`Unexpected API ${url}`)
}
beforeEach(() => {
  setCsrfToken()
  Object.defineProperty(URL, 'createObjectURL', { configurable: true, writable: true, value: vi.fn(() => 'blob:photo-preview') })
  Object.defineProperty(URL, 'revokeObjectURL', { configurable: true, writable: true, value: vi.fn() })
})
afterEach(() => { setCsrfToken(); vi.unstubAllGlobals() })

describe('photo input and media privacy boundary', () => {
  it('whitelists safe photo fields, discards guest versions and rejects foreign or ambiguous media URLs', () => {
    const result = safeBikeGallery({ photos: [{ ...photo, owner_id: 'secret', filename: 'gps-original.jpg', bytes: 'secret' }, ...['https://example.org/image.jpg', '//example.org/image.jpg', `${photo.url}?secret=1`, photo.url.replace('/content', '/content/../other'), photo.url.replace('/api/', '/\\api/')].map((url) => ({ ...photo, url }))], cover_photo_id: photo.id, version: 7, owner_id: 'secret' }, bike.id, false)
    expect(result).toEqual({ photos: [photo], cover_photo_id: photo.id })
    expect(Object.keys(result.photos[0]).sort()).toEqual(['created_at', 'height', 'id', 'url', 'width'])
    expect(safeBikeGallery({ photos: [photo], cover_photo_id: 'deleted-photo', version: 7 }, bike.id, true)).toEqual({ photos: [photo], cover_photo_id: '', version: 7 })
    expect(() => safeBikeGallery({ photos: null }, bike.id, false)).toThrow()
  })

  it('accepts JPEG/PNG/WebP or known extensions with unknown MIME and enforces the source limit', () => {
    expect(photoFileError(new File(['bytes'], 'bike.webp', { type: 'image/webp' }))).toBeUndefined()
    expect(photoFileError(new File(['bytes'], 'bike.PNG'))).toBeUndefined()
    expect(photoFileError(new File(['bytes'], 'bike.heic', { type: 'image/heic' }))).toBe('photoInvalidType')
    expect(photoFileError(new File(['bytes'], 'fake.jpg', { type: 'image/svg+xml' }))).toBe('photoInvalidType')
    expect(photoFileError(new File([], 'empty.jpg', { type: 'image/jpeg' }))).toBe('photoEmpty')
    const large = new File(['bytes'], 'large.png', { type: 'image/png' })
    Object.defineProperty(large, 'size', { value: 12 * 1024 * 1024 + 1 })
    expect(photoFileError(large)).toBe('photoTooLarge')
  })

  it('blocks gallery reads after a failed session, then keeps a reconciled guest gallery safe and private', async () => {
    let failed = true
    const fetch = vi.fn(async (url: string) => {
      if (url.endsWith('/session')) return failed ? response({ error: { code: 'service_unavailable' } }, 503) : response({ user: null })
      if (url.endsWith('/photos')) return response({ photos: [{ ...photo, owner_id: 'secret', filename: 'secret.jpg' }], cover_photo_id: photo.id, version: 8, owner_id: rider.id })
      return defaults(url)
    })
    vi.stubGlobal('fetch', fetch)
    const { client } = mount(false)
    await screen.findByText('We couldn’t reach BIKERS.KZ.')
    expect(fetch.mock.calls.some(([url]) => url.endsWith('/photos'))).toBe(false)
    failed = false
    await userEvent.click(screen.getByRole('button', { name: 'Try again' }))
    await screen.findByRole('img', { name: 'Photo of Honda CB500X' })
    expect(client.getQueryData(['bike-photos', bike.id, 'guest'])).toEqual({ photos: [photo], cover_photo_id: photo.id })
    expect(client.getQueryCache().find({ queryKey: ['bike-photos', bike.id, 'guest'] })?.meta?.private).toBe(true)
    expect(screen.queryByRole('button', { name: 'Use as cover' })).not.toBeInTheDocument()
  })
})

describe('gallery and explicit cover behavior', () => {
  it('blocks uploads at six ready photos and explains unsupported HEIC selections before sending', async () => {
    let full = false
    const six = Array.from({ length: 6 }, (_, index) => {
      const id = `22222222-2222-4222-8222-22222222222${index}`
      return { ...photo, id, url: `/api/v1/bikes/${bike.id}/photos/${id}/content` }
    })
    const fetch = vi.fn(async (url: string) => url.endsWith('/photos') ? response({ photos: full ? six : [], cover_photo_id: '', version: 1 }) : defaults(url))
    vi.stubGlobal('fetch', fetch)
    const { client } = mount()
    const actor = userEvent.setup({ applyAccept: false })
    await actor.upload(await screen.findByLabelText('Choose a photo'), new File(['heic'], 'bike.heic', { type: 'image/heic' }))
    expect(await screen.findByText(/Convert HEIC/)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Upload photo' })).not.toBeInTheDocument()
    full = true
    await act(async () => { await client.invalidateQueries({ queryKey: ['bike-photos', bike.id] }) })
    await waitFor(() => expect(screen.getByLabelText('Choose a photo')).toBeDisabled())
    expect(screen.getByText('This motorcycle has reached its limit of 6 photos. Delete a photo before adding another.')).toBeInTheDocument()
  })

  it('uploads without changing the catalogue cover, explicitly selects/restores cover and confirms deletion', async () => {
    let gallery: BikePhotoGallery = { photos: [], cover_photo_id: '', version: 1 }
    const commands: { method: string; version?: number; photo_id?: string; csrf: string | null }[] = []
    const fetch = vi.fn(async (url: string, options?: RequestInit) => {
      if (url.endsWith(`/me/bikes/${bike.id}/photos`) && options?.method === 'POST') {
        expect(options.body).toBeInstanceOf(File)
        expect(new Headers(options.headers).get('Content-Type')).toBe('image/jpeg')
        commands.push({ method: 'POST', csrf: new Headers(options.headers).get('X-CSRF-Token') })
        gallery = { photos: [photo], cover_photo_id: '', version: 2 }
        return response({ photo, gallery }, 201)
      }
      if (url.endsWith('/photo-cover') || (url.endsWith(`/photos/${photo.id}`) && options?.method === 'DELETE')) {
        const body = JSON.parse(options!.body as string)
        commands.push({ method: options!.method!, ...body, csrf: new Headers(options!.headers).get('X-CSRF-Token') })
        gallery = options!.method === 'DELETE' ? { photos: [], cover_photo_id: '', version: gallery.version! + 1 } : { ...gallery, cover_photo_id: body.photo_id, version: gallery.version! + 1 }
        return response({ gallery })
      }
      if (url.endsWith(`/bikes/${bike.id}/photos`)) return response(gallery)
      return defaults(url)
    })
    vi.stubGlobal('fetch', fetch)
    mount()
    const actor = userEvent.setup()
    await actor.upload(await screen.findByLabelText('Choose a photo'), new File(['photo bytes'], 'bike.jpg', { type: 'image/jpeg' }))
    await screen.findByRole('img', { name: 'Preview of the selected photo' })
    await actor.click(screen.getByRole('button', { name: 'Upload photo' }))
    const item = await screen.findByRole('article', { name: 'Motorcycle photo 1' })
    expect(await screen.findByRole('img', { name: 'Reference image of Honda CB500X, 2019' })).toBeInTheDocument()
    expect(screen.queryByRole('img', { name: 'Photo of Honda CB500X' })).not.toBeInTheDocument()
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:photo-preview')
    await actor.click(within(item).getByRole('button', { name: 'Use as cover' }))
    await screen.findByRole('img', { name: 'Photo of Honda CB500X' })
    await actor.click(screen.getByRole('button', { name: 'Restore catalogue cover' }))
    await waitFor(() => expect(screen.queryByRole('img', { name: 'Photo of Honda CB500X' })).not.toBeInTheDocument())
    await actor.click(within(item).getByRole('button', { name: 'Use as cover' }))
    await screen.findByRole('img', { name: 'Photo of Honda CB500X' })
    await actor.click(within(item).getByRole('button', { name: 'Delete photo' }))
    await actor.click(within(await screen.findByRole('dialog', { name: 'Delete photo' })).getByRole('button', { name: 'Cancel' }))
    expect(commands).toHaveLength(4)
    await actor.click(within(item).getByRole('button', { name: 'Delete photo' }))
    await actor.click(within(await screen.findByRole('dialog', { name: 'Delete photo' })).getByRole('button', { name: 'Delete photo' }))
    await screen.findByText('No uploaded photos yet.')
    expect(screen.queryByRole('img', { name: 'Photo of Honda CB500X' })).not.toBeInTheDocument()
    expect(commands).toEqual([{ method: 'POST', csrf: 'csrf-rider-a' }, { method: 'PUT', version: 2, photo_id: photo.id, csrf: 'csrf-rider-a' }, { method: 'PUT', version: 3, photo_id: '', csrf: 'csrf-rider-a' }, { method: 'PUT', version: 4, photo_id: photo.id, csrf: 'csrf-rider-a' }, { method: 'DELETE', version: 5, csrf: 'csrf-rider-a' }])
  })

  it('falls back to the catalogue after a broken custom cover and hides protected images on gallery failure', async () => {
    let fail = false
    vi.stubGlobal('fetch', vi.fn(async (url: string) => {
      if (url.endsWith('/photos')) return fail ? response({ error: { code: 'service_unavailable' } }, 503) : response({ photos: [photo], cover_photo_id: photo.id, version: 1 })
      return defaults(url)
    }))
    const { client } = mount()
    fireEvent.error(await screen.findByRole('img', { name: 'Photo of Honda CB500X' }))
    await screen.findByRole('img', { name: 'Reference image of Honda CB500X, 2019' })
    fail = true
    await act(async () => { await client.invalidateQueries({ queryKey: ['bike-photos', bike.id] }) })
    await waitFor(() => expect(screen.queryByRole('img', { name: 'Motorcycle photo 1' })).not.toBeInTheDocument())
    expect(screen.queryByLabelText('Choose a photo')).not.toBeInTheDocument()
  })

  it.each([['ru', 'Выбрать фото', 'Выбрана обложка из каталога'], ['kk', 'Фото таңдау', 'Каталог мұқабасы таңдалған']])('localizes upload and cover controls in %s', async (locale, label, selected) => {
    localStorage.setItem('bikerskz.locale', locale)
    vi.stubGlobal('fetch', vi.fn(async (url: string) => url.endsWith('/photos') ? response({ photos: [], cover_photo_id: '', version: 1 }) : defaults(url)))
    mount()
    await screen.findByLabelText(label)
    expect(screen.getByText(selected)).toBeInTheDocument()
  })
})

describe('upload cancellation and authentication changes', () => {
  it.each(['Expire test session', 'Switch test actor', 'Cancel upload'])('aborts and discards late upload results on %s and revokes its local preview', async (action) => {
    let finish: (value: Response) => void = () => { throw new Error('upload not started') }
    const pending = new Promise<Response>((resolve) => { finish = resolve })
    let signal: AbortSignal | undefined
    const fetch = vi.fn(async (url: string, options?: RequestInit) => {
      if (url.endsWith(`/me/bikes/${bike.id}/photos`) && options?.method === 'POST') { signal = options.signal!; return pending }
      if (url.endsWith('/photos')) return response({ photos: [], cover_photo_id: '', version: 1 })
      return defaults(url)
    })
    vi.stubGlobal('fetch', fetch)
    const { client } = mount()
    const actor = userEvent.setup()
    await actor.upload(await screen.findByLabelText('Choose a photo'), new File(['photo bytes'], 'private.jpg', { type: 'image/jpeg' }))
    await screen.findByRole('img', { name: 'Preview of the selected photo' })
    await actor.click(screen.getByRole('button', { name: 'Upload photo' }))
    expect(await screen.findByRole('progressbar', { name: 'Uploading photo…' })).not.toHaveAttribute('aria-valuenow')
    await actor.click(screen.getByRole('button', { name: action }))
    await waitFor(() => expect(signal?.aborted).toBe(true))
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:photo-preview')
    await act(async () => { finish(response({ photo, gallery: { photos: [photo], cover_photo_id: photo.id, version: 2 } }, 201)); await pending })
    expect(screen.queryByRole('img', { name: 'Preview of the selected photo' })).not.toBeInTheDocument()
    expect(screen.queryByRole('img', { name: 'Photo of Honda CB500X' })).not.toBeInTheDocument()
    if (action !== 'Cancel upload') expect(client.getQueryData(['bike-photos', bike.id, rider.id])).toBeUndefined()
    else expect(client.getQueryData<BikePhotoGallery>(['bike-photos', bike.id, rider.id])?.photos).toEqual([])
  })
})
