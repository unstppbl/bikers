import { useEffect, useId, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { GlobeIcon as Globe, ImageSquareIcon as ImagePlus, TrashIcon as Trash2, UploadSimpleIcon as Upload, XIcon as X } from '@phosphor-icons/react'
import { ApiError, getSessionEpoch, request, uploadFile } from '../api'
import { cityLabel } from '../cities'
import { cityPhoto } from '../cityPhotos'
import type { CityPhoto } from '../cityPhotos'
import { useI18n } from '../i18n'
import { useSession, useSessionGuard } from '../session'
import { ErrorNotice, Loading, useConfirmation, useOnline } from '../ui'
import type { Ride, RideCoverMetadata, RideCoverPhoto } from '../types'
import { useCityOptions } from './CitySelector'
import { LocalPhotoPreview, photoFileError } from './PhotoInput'
import { CityArt } from './RideVisual'
import { ReportButton } from './Trust'
import './ride-cover.css'

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i

// Keeps the five public photo fields, and only when the content route belongs to this ride.
export function safeRideCoverPhoto(value: unknown, rideId: string): RideCoverPhoto | null {
  if (!uuid.test(rideId) || !value || typeof value !== 'object' || !('id' in value && 'url' in value && 'width' in value && 'height' in value && 'created_at' in value)) return null
  const { id, url, width, height, created_at } = value
  return typeof id === 'string' && uuid.test(id) && url === `/api/v1/rides/${rideId}/cover/${id}/content` && typeof width === 'number' && Number.isSafeInteger(width) && width > 0 && width <= 1920 && typeof height === 'number' && Number.isSafeInteger(height) && height > 0 && height <= 1920 && typeof created_at === 'string' ? { id, url, width, height, created_at } : null
}

export function safeRideCover(value: unknown, rideId: string, organizer: boolean): RideCoverMetadata {
  if (!uuid.test(rideId) || !value || typeof value !== 'object' || !('image' in value)) throw new ApiError(502, 'request_failed')
  const image = safeRideCoverPhoto(value.image, rideId)
  const version = organizer && 'version' in value && Number.isSafeInteger(value.version) && Number(value.version) > 0 ? Number(value.version) : undefined
  return { image, ...(version === undefined ? {} : { version }) }
}

interface Picture { url: string; width: number; height: number; alt: string }
interface PictureProps { pictures: readonly (Picture | null | undefined)[]; className: string; priority: boolean; fallback?: ReactNode; onFail?: (url: string) => void }

// Shows the first picture that has not failed to load, or the fallback once all of them have failed.
// Failures are tracked by URL, so a cover that fails moves on to the city photo. They belong to one subject
// (the ride, and in the editor its stored cover) and one list of candidate URLs: when either changes, for
// example on another ride or when a cover is uploaded, replaced, removed or comes back, every candidate is
// tried again, even a city photo whose URL is unchanged.
function FirstPicture({ subject, ...props }: PictureProps & { subject: readonly (string | undefined)[] }) {
  return <Candidates key={JSON.stringify([...subject, ...props.pictures.map((picture) => picture?.url)])} {...props} />
}

function Candidates({ pictures, className, priority, fallback = null, onFail }: PictureProps) {
  const [failed, setFailed] = useState<readonly string[]>([])
  const picture = pictures.find((candidate) => candidate && !failed.includes(candidate.url))
  return picture ? <img key={picture.url} className={className} src={picture.url} width={picture.width} height={picture.height} alt={picture.alt} decoding="async" loading={priority ? undefined : 'lazy'} referrerPolicy="no-referrer" onError={() => { setFailed((current) => [...current, picture.url]); onFail?.(picture.url) }} /> : fallback
}

// A ride's picture: its visible cover, else a photo of its catalogue city, else nothing, so the city name
// art beside it shows. A cover that fails to load (replaced in the meantime, or hidden by moderation) moves on
// to the city photo, and a city photo that fails leaves the city name. Cards are decorative because their text
// already names the ride; the ride page describes the picture, and never calls a city photo a photo of the ride.
export function RideImage({ ride, className, described = false, priority = false }: { ride: Ride; className: string; described?: boolean; priority?: boolean }) {
  const { t, locale } = useI18n()
  const cities = useCityOptions().data?.cities ?? []
  const city = cityLabel(ride.city, locale, cities)
  const cover = safeRideCoverPhoto(ride.cover, ride.id)
  const photo = cityPhoto(ride.city, ride.id, cities)
  return <FirstPicture subject={[ride.id]} className={className} priority={priority} pictures={[
    cover && { ...cover, alt: described ? t('rideCoverAlt', { city }) : '' },
    photo && { ...photo, alt: described ? t('cityPhotoAlt', { city }) : '' },
  ]} />
}

export function RideCoverReport({ ride }: { ride: Ride }) {
  const photo = safeRideCoverPhoto(ride.cover, ride.id)
  return photo && !ride.can_manage ? <ReportButton targetType="ride_cover" targetId={photo.id} label="reportCover" /> : null
}

// The organizer's editor reads GET /cover, which also returns a cover hidden by moderation so it can be replaced or removed.
export function RideCoverEditor({ ride }: { ride: Ride }) {
  const { user, loading, error } = useSession()
  const { t, locale } = useI18n()
  const cities = useCityOptions().data?.cities ?? []
  const organizer = !!user && !loading && !error && ride.can_manage === true && (!ride.organizer_id || ride.organizer_id === user.id)
  const editable = organizer && ride.status !== 'cancelled'
  const queryKey = ['ride-cover', ride.id, user?.id ?? 'guest']
  const query = useQuery({
    queryKey,
    queryFn: async ({ signal }) => safeRideCover(await request<unknown>(`/rides/${ride.id}/cover`, { signal }), ride.id, true),
    meta: { private: true },
    enabled: editable,
    staleTime: 30_000,
    retry: false,
  })
  if (!organizer) return null
  if (!editable) return <div className="inline-note ride-cover-note"><ImagePlus size={20} /><p>{t('rideCoverCancelled')}</p></div>
  if (query.isPending) return <Loading />
  if (query.isError) return <section className="surface ride-cover-editor"><ErrorNotice error={query.error} retry={() => void query.refetch()} /></section>
  if (query.data.version === undefined) return <section className="surface ride-cover-editor"><ErrorNotice text={t('noAccess')} retry={() => void query.refetch()} /></section>
  return <CoverEditor key={`${ride.id}:${user.id}:${getSessionEpoch()}`} ride={ride} city={cityLabel(ride.city, locale, cities)} photo={cityPhoto(ride.city, ride.id, cities)} metadata={query.data} queryKey={queryKey} reload={() => query.refetch()} />
}

const coverErrors: Record<string, 'rideCoverBusy' | 'rideCoverChanged' | 'rideCoverExpired'> = { ride_cover_busy: 'rideCoverBusy', stale_version: 'rideCoverChanged', invalid_photo_choice: 'rideCoverChanged', photo_upload_expired: 'rideCoverExpired' }

function CoverEditor({ ride, city, photo, metadata, queryKey, reload }: { ride: Ride; city: string; photo: CityPhoto | undefined; metadata: RideCoverMetadata; queryKey: string[]; reload: () => Promise<unknown> }) {
  const { user } = useSession()
  const { t } = useI18n()
  const { confirm, dialog } = useConfirmation()
  const queryClient = useQueryClient()
  const captureSession = useSessionGuard()
  const online = useOnline()
  const inputId = useId()
  const section = useRef<HTMLElement>(null)
  const input = useRef<HTMLInputElement>(null)
  const uploadButton = useRef<HTMLButtonElement>(null)
  const cancelUpload = useRef<HTMLButtonElement>(null)
  const removeButton = useRef<HTMLButtonElement>(null)
  const focusAfter = useRef<'' | 'picker' | 'upload' | 'remove'>('')
  const active = useRef<AbortController | null>(null)
  const [file, setFile] = useState<File | null>(null)
  const [busy, setBusy] = useState('')
  const [validation, setValidation] = useState('')
  const [error, setError] = useState<unknown>()
  const [unconfirmed, setUnconfirmed] = useState(false)
  const [saved, setSaved] = useState<'' | 'rideCoverSaved' | 'rideCoverRemoved'>('')
  const [currentFailed, setCurrentFailed] = useState('')
  useEffect(() => () => { active.current?.abort() }, [])
  // The buttons change as an action runs, so keyboard focus would fall back to the page. Keep it in the editor:
  // on Cancel upload while uploading, on the upload or Remove button after it failed, otherwise on the file picker.
  useEffect(() => {
    const focused = document.activeElement
    const lost = !focused || focused === document.body
    if (busy === 'upload' && lost) { cancelUpload.current?.focus(); return }
    if (busy || !focusAfter.current || !input.current || input.current.disabled) return
    const failed = focusAfter.current === 'upload' ? uploadButton.current : focusAfter.current === 'remove' ? removeButton.current : null
    const target = failed && !failed.disabled ? failed : input.current
    focusAfter.current = ''
    if (lost || section.current?.contains(focused)) target.focus()
  })

  // The ride can be cancelled, or the session can change, while an upload runs.
  // Check the live ride query as well as the session before accepting a completion.
  function currentOrganizer() {
    const state = queryClient.getQueryState<{ ride: Ride }>(['ride', ride.id, user?.id])
    const current = state?.data?.ride
    return state?.status === 'success' && current?.can_manage === true && current.status !== 'cancelled' && (!current.organizer_id || current.organizer_id === user?.id)
  }
  function refreshRides() {
    void queryClient.invalidateQueries({ queryKey: ['ride', ride.id] })
    void queryClient.invalidateQueries({ queryKey: ['rides'] })
    void queryClient.invalidateQueries({ queryKey: ['club-rides'] })
  }
  async function operate(kind: 'upload' | 'remove', command: (signal: AbortSignal) => Promise<unknown>) {
    if (active.current || !currentOrganizer()) return
    const currentSession = captureSession()
    const controller = new AbortController()
    active.current = controller
    const current = () => !controller.signal.aborted && active.current === controller && currentSession() && currentOrganizer()
    setBusy(kind); setError(undefined); setValidation(''); setSaved('')
    try {
      const result = await command(controller.signal)
      if (!current()) return
      const next = safeRideCover(result, ride.id, true)
      // A cover read that started before this change would otherwise land after it and restore the old version.
      await queryClient.cancelQueries({ queryKey, exact: true })
      if (!current()) return
      queryClient.setQueryDefaults(queryKey, { meta: { private: true } })
      queryClient.setQueryData(queryKey, next)
      setFile(null); setUnconfirmed(false); setSaved(kind === 'upload' ? 'rideCoverSaved' : 'rideCoverRemoved')
      focusAfter.current = 'picker'
      refreshRides()
    } catch (value) {
      if (!current()) return
      focusAfter.current = kind
      if (kind === 'upload' && value instanceof ApiError && (value.status === 0 || value.status >= 500)) setUnconfirmed(true)
      else setError(value)
      if (value instanceof ApiError && ([401, 403, 404].includes(value.status) || ['stale_version', 'ride_cancelled'].includes(value.code))) { refreshRides(); void reload() }
    } finally {
      if (active.current === controller && !controller.signal.aborted && currentSession()) { active.current = null; setBusy('') }
    }
  }
  // A cancelled or unconfirmed upload may still have replaced the cover, so the ride and the feeds reload with the cover.
  function cancel() {
    active.current?.abort(); active.current = null
    focusAfter.current = 'picker'
    setBusy(''); setFile(null); setValidation(''); setError(undefined)
    if (currentOrganizer()) { void reload(); refreshRides() }
  }
  async function remove() {
    const currentSession = captureSession()
    if (!await confirm({ message: t(photo ? 'rideCoverRemoveConfirmCityPhoto' : 'rideCoverRemoveConfirm', { city }), action: t('rideCoverRemove'), destructive: true }) || !currentSession() || !currentOrganizer()) return
    await operate('remove', (signal) => request(`/rides/${ride.id}/cover`, { method: 'DELETE', signal, body: JSON.stringify({ version: metadata.version }) }))
  }
  async function refresh() {
    const currentSession = captureSession()
    const result = await reload()
    if (currentSession() && currentOrganizer() && result && typeof result === 'object' && 'isError' in result && !result.isError) { setUnconfirmed(false); setValidation(''); setError(undefined); refreshRides() }
  }
  const image = metadata.image
  const shown = image && currentFailed !== image.id ? image : null
  const specificError = error instanceof ApiError ? coverErrors[error.code] : undefined
  const note = image ? shown ? 'rideCoverReplacement' : 'rideCoverUnavailable' : photo ? 'rideCoverEmptyCityPhoto' : 'rideCoverEmpty'
  // The current slot shows what the ride shows: the cover, else the city photo, else the city name.
  return <section ref={section} className="surface photo-upload-panel ride-cover-editor" aria-labelledby={`${inputId}-heading`}>{dialog}
    <div className="ride-cover-editor-heading">
      <span className="ride-cover-current"><FirstPicture subject={[ride.id, image?.id]} className="ride-cover-current-image" priority fallback={<CityArt city={city} />} pictures={[
        shown && { ...shown, alt: t('rideCoverCurrentAlt') },
        photo && { ...photo, alt: t('cityPhotoAlt', { city }) },
      ]} onFail={(url) => { if (shown && url === shown.url) setCurrentFailed(shown.id) }} /></span>
      <div><h2 id={`${inputId}-heading`}>{t('rideCover')}</h2><p>{t(note, { city })}</p></div>
      {image && <button ref={removeButton} type="button" className="text-button danger-text" disabled={!!busy || !online} onClick={() => void remove()}><Trash2 size={15} />{t('rideCoverRemove')}</button>}
    </div>
    <div className="inline-note ride-cover-public-note"><Globe size={18} /><p>{t('rideCoverPublic')}</p></div>
    <div className="photo-upload-heading"><span className="panel-icon"><ImagePlus size={22} /></span><div><label className="button button-outline photo-picker-label" htmlFor={inputId}>{t('choosePhoto')}</label><p>{t('rideCoverHint')}</p></div></div>
    <input ref={input} id={inputId} className="photo-file-input" type="file" accept="image/jpeg,image/png,image/webp,.jpg,.jpeg,.png,.webp" disabled={!!busy || !online} onChange={(event) => {
      const chosen = event.target.files?.[0]
      event.target.value = ''
      // An unconfirmed upload stays blocked until the cover is reloaded, whichever photo comes next.
      setFile(null); setValidation(''); setError(undefined); setSaved('')
      if (!chosen) return
      const invalid = photoFileError(chosen)
      if (invalid) { setValidation(t(invalid)); return }
      setFile(chosen)
    }} />
    {file && <><LocalPhotoPreview key={`${file.name}:${file.size}:${file.lastModified}`} file={file} /><div className="photo-upload-actions">{busy === 'upload' ? <><div className="photo-upload-progress" role="progressbar" aria-label={t('uploadingPhoto')}><span className="spinner" />{t('uploadingPhoto')}</div><button ref={cancelUpload} type="button" className="button button-outline" onClick={cancel}><X size={15} />{t('cancelUpload')}</button></> : <><button ref={uploadButton} type="button" className="button button-primary" disabled={!!busy || !online || unconfirmed} onClick={() => void operate('upload', (signal) => uploadFile(`/rides/${ride.id}/cover?version=${metadata.version}`, file, signal))}><Upload size={15} />{t(image ? 'rideCoverReplace' : 'rideCoverUpload')}</button><button type="button" className="text-button" disabled={!!busy} onClick={() => { focusAfter.current = 'picker'; setFile(null) }}>{t('cancel')}</button></>}</div></>}
    {saved && <p className="success-message" role="status">{t(saved)}</p>}
    {unconfirmed && <ErrorNotice text={t('rideCoverUnconfirmed')} retry={() => void refresh()} />}{validation && <ErrorNotice text={validation} />}{error != null && <ErrorNotice {...(specificError ? { text: t(specificError) } : { error })} retry={() => void refresh()} />}
  </section>
}
