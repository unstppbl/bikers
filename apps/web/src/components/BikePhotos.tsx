import { useEffect, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { CheckIcon as Check, ImageSquareIcon as ImagePlus, ArrowCounterClockwiseIcon as RotateCcw, TrashIcon as Trash2, UploadSimpleIcon as Upload, XIcon as X } from '@phosphor-icons/react'
import { ApiError, getSessionEpoch, request, uploadFile } from '../api'
import { ReportButton } from './Trust'
import { useI18n } from '../i18n'
import { useSession, useSessionGuard } from '../session'
import { ErrorNotice, Loading, useConfirmation, useOnline } from '../ui'
import type { BikePhoto, BikePhotoGallery, PublicBike } from '../types'
import { ModelReference } from './ModelReference'
import { LocalPhotoPreview, photoFileError } from './PhotoInput'

const MAX_PHOTOS = 6
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i

// Cookie-dependent responses are reduced before entering the query cache. Media
// must use the exact authorized content route for this bike and photo.
export function safeBikeGallery(value: unknown, bikeId: string, authenticated: boolean): BikePhotoGallery {
  if (!uuid.test(bikeId) || !value || typeof value !== 'object' || !('photos' in value) || !Array.isArray(value.photos) || !('cover_photo_id' in value) || typeof value.cover_photo_id !== 'string') throw new ApiError(502, 'request_failed')
  const photos: BikePhoto[] = []
  for (const item of value.photos) {
    if (!item || typeof item !== 'object') continue
    const { id, url, width, height, created_at } = item
    if (typeof id !== 'string' || !uuid.test(id) || typeof url !== 'string' || url !== `/api/v1/bikes/${bikeId}/photos/${id}/content`) continue
    if (!Number.isSafeInteger(width) || width < 1 || width > 1920 || !Number.isSafeInteger(height) || height < 1 || height > 1920 || typeof created_at !== 'string') continue
    if (!photos.some((photo) => photo.id === id)) photos.push({ id, url, width, height, created_at })
    if (photos.length === MAX_PHOTOS) break
  }
  const cover_photo_id = photos.some((photo) => photo.id === value.cover_photo_id) ? value.cover_photo_id : ''
  const version = 'version' in value && authenticated && Number.isSafeInteger(value.version) && Number(value.version) > 0 ? Number(value.version) : undefined
  return { photos, cover_photo_id, ...(version === undefined ? {} : { version }) }
}

function useBikeGallery(bikeId: string) {
  const { user, loading, error, refresh } = useSession()
  const ready = !loading && !error
  const queryKey = ['bike-photos', bikeId, user?.id ?? 'guest']
  const query = useQuery({
    queryKey,
    queryFn: async ({ signal }) => safeBikeGallery(await request<unknown>(`/bikes/${bikeId}/photos`, { signal }), bikeId, !!user),
    enabled: ready,
    meta: { private: true },
    staleTime: 30_000,
    retry: false,
  })
  return { query, queryKey, ready, sessionError: error, refreshSession: refresh }
}

export function BikeCover({ bike, variant = 'detail' }: { bike: PublicBike; variant?: 'card' | 'detail' }) {
  const { query, ready } = useBikeGallery(bike.id)
  const gallery = ready && !query.isError ? query.data : undefined
  const photo = gallery?.photos.find((item) => item.id === gallery.cover_photo_id)
  if (!photo) return <ModelReference bike={bike} variant={variant} />
  return <UploadedCover key={`${bike.id}:${photo.id}:${photo.url}`} photo={photo} bike={bike} variant={variant} />
}

function UploadedCover({ photo, bike, variant }: { photo: BikePhoto; bike: PublicBike; variant: 'card' | 'detail' }) {
  const { t } = useI18n()
  const [failed, setFailed] = useState(false)
  if (failed) return <ModelReference bike={bike} variant={variant} />
  return <figure className={`model-reference model-reference-${variant} uploaded-bike-cover`}><div className="model-reference-frame"><img src={photo.url} width={photo.width} height={photo.height} alt={t('photoAlt', { make: bike.make, model: bike.model })} loading={variant === 'card' ? 'lazy' : 'eager'} decoding="async" referrerPolicy="no-referrer" onError={() => setFailed(true)} /></div><figcaption><p>{t('photoOwnerLabel')}<span>{bike.make} {bike.model}</span></p></figcaption></figure>
}

export function BikePhotos({ bike, owner = false }: { bike: PublicBike; owner?: boolean }) {
  const { t } = useI18n()
  const { user } = useSession()
  const { query, queryKey, ready, sessionError, refreshSession } = useBikeGallery(bike.id)
  if (sessionError) return <section className="bike-photos-section"><ErrorNotice text={t('serviceUnavailable')} retry={() => void refreshSession()} /></section>
  if (!ready || query.isPending) return owner ? <Loading /> : null
  if (query.isError) return <section className="bike-photos-section"><ErrorNotice error={query.error} retry={() => void query.refetch()} /></section>
  const gallery = query.data
  if (owner) {
    if (!user || gallery.version === undefined) return <ErrorNotice text={t('noAccess')} retry={() => void query.refetch()} />
    return <OwnerPhotos key={`${user.id}:${getSessionEpoch()}`} bike={bike} gallery={gallery} queryKey={queryKey} reload={() => query.refetch()} />
  }
  if (!gallery.photos.length) return null
  return <section className="bike-photos-section"><div className="section-header"><h2>{t('bikePhotos')}</h2></div><div className="bike-photo-grid">{gallery.photos.map((photo, index) => <GalleryPhoto key={photo.id} photo={photo} number={index + 1} selected={photo.id === gallery.cover_photo_id} />)}</div></section>
}

function GalleryPhoto({ photo, number, selected, actions }: { photo: BikePhoto; number: number; selected: boolean; actions?: (failed: boolean) => ReactNode }) {
  const { t } = useI18n()
  const [failed, setFailed] = useState(false)
  return <article className="surface bike-photo" aria-label={t('photoGalleryAlt', { number })}><div className="bike-photo-frame">{failed ? <p className="small-empty">{t('photoUnavailable')}</p> : <img src={photo.url} width={photo.width} height={photo.height} alt={t('photoGalleryAlt', { number })} loading="lazy" decoding="async" referrerPolicy="no-referrer" onError={() => setFailed(true)} />}{selected && <span className="photo-cover-badge"><Check size={12} />{t('photoCover')}</span>}</div>{(actions || !failed) && <div className="bike-photo-actions">{actions?.(failed)}{!failed && <ReportButton targetType="bike_photo" targetId={photo.id} />}</div>}</article>
}

function OwnerPhotos({ bike, gallery, queryKey, reload }: { bike: PublicBike; gallery: BikePhotoGallery; queryKey: (string | undefined)[]; reload: () => Promise<unknown> }) {
  const { t } = useI18n()
  const { confirm, dialog } = useConfirmation()
  const queryClient = useQueryClient()
  const captureSession = useSessionGuard()
  const online = useOnline()
  const active = useRef<AbortController | null>(null)
  const [file, setFile] = useState<File | null>(null)
  const [busy, setBusy] = useState('')
  const [error, setError] = useState<unknown>()
  const [validation, setValidation] = useState('')
  const [unconfirmed, setUnconfirmed] = useState(false)
  useEffect(() => () => { active.current?.abort() }, [])

  async function operate(kind: string, command: (signal: AbortSignal) => Promise<{ gallery: unknown }>) {
    if (active.current) return
    const currentSession = captureSession()
    const controller = new AbortController()
    active.current = controller
    setBusy(kind); setError(undefined); setValidation('')
    const current = () => !controller.signal.aborted && active.current === controller && currentSession()
    try {
      const result = await command(controller.signal)
      if (!current()) return
      const next = safeBikeGallery(result.gallery, bike.id, true)
      queryClient.setQueryDefaults(queryKey, { meta: { private: true } })
      queryClient.setQueryData(queryKey, next)
      if (kind === 'upload') { setFile(null); setUnconfirmed(false) }
      void queryClient.invalidateQueries({ queryKey: ['bike-photos', bike.id], refetchType: 'none' })
    } catch (value) {
      if (!current()) return
      if (kind === 'upload' && value instanceof ApiError && (value.status === 0 || value.status >= 500)) { setUnconfirmed(true); setValidation(t('photoUploadUnconfirmed')) }
      else setError(value)
      if (value instanceof ApiError && value.code === 'stale_version') void reload()
    } finally {
      if (current()) { active.current = null; setBusy('') }
    }
  }

  async function remove(photo: BikePhoto) {
    const currentSession = captureSession()
    if (!await confirm({ message: t('deletePhotoConfirm'), action: t('deletePhoto'), destructive: true }) || !currentSession()) return
    await operate(`delete:${photo.id}`, (signal) => request(`/me/bikes/${bike.id}/photos/${photo.id}`, { method: 'DELETE', signal, body: JSON.stringify({ version: gallery.version }) }))
  }
  function cover(photoId: string) {
    void operate(`cover:${photoId}`, (signal) => request(`/me/bikes/${bike.id}/photo-cover`, { method: 'PUT', signal, body: JSON.stringify({ version: gallery.version, photo_id: photoId }) }))
  }
  function cancelUpload() {
    active.current?.abort(); active.current = null
    setBusy(''); setFile(null); setValidation(''); setError(undefined); setUnconfirmed(false)
    void reload()
  }
  async function refresh() {
    const currentSession = captureSession()
    const result = await reload()
    if (currentSession() && result && typeof result === 'object' && 'isError' in result && !result.isError) { setUnconfirmed(false); setValidation(''); setError(undefined) }
  }
  return <section className="bike-photos-section">{dialog}<div className="section-header"><div><span className="eyebrow">{t('photoCount', { count: gallery.photos.length })}</span><h2>{t('bikePhotos')}</h2></div>{gallery.cover_photo_id ? <button className="button button-outline" onClick={() => cover('')} disabled={!!busy || !online}><RotateCcw size={15} />{t('restoreCatalogue')}</button> : <span className="photo-catalogue-label"><Check size={13} />{t('catalogueCoverActive')}</span>}</div><p className="section-intro">{t('bikePhotosText')}</p><p className="photo-default-note">{t('catalogueDefaultHint')}</p>
    <div className="surface photo-upload-panel"><div className="photo-upload-heading"><span className="panel-icon"><ImagePlus size={22} /></span><div><label className="photo-picker-label" htmlFor={`photo-file-${bike.id}`}>{t('choosePhoto')}</label><p>{t('photoUploadHint')}</p></div></div><input id={`photo-file-${bike.id}`} className="photo-file-input" type="file" accept="image/jpeg,image/png,image/webp,.jpg,.jpeg,.png,.webp" disabled={!!busy || !online || gallery.photos.length >= MAX_PHOTOS} onChange={(event) => {
      const chosen = event.target.files?.[0]
      event.target.value = ''
      setFile(null); setError(undefined); setValidation(''); setUnconfirmed(false)
      if (!chosen) return
      const invalid = photoFileError(chosen)
      if (invalid) { setValidation(t(invalid)); return }
      setFile(chosen)
    }} />
      {file && <><LocalPhotoPreview key={`${file.name}:${file.size}:${file.lastModified}`} file={file} /><div className="photo-upload-actions">{busy === 'upload' ? <><div className="photo-upload-progress" role="progressbar" aria-label={t('uploadingPhoto')}><span className="spinner" />{t('uploadingPhoto')}</div><button className="button button-outline" onClick={cancelUpload}><X size={15} />{t('cancelUpload')}</button></> : <><button className="button button-primary" disabled={!!busy || !online || unconfirmed || gallery.photos.length >= MAX_PHOTOS} onClick={() => void operate('upload', (signal) => uploadFile(`/me/bikes/${bike.id}/photos`, file, signal))}><Upload size={15} />{t('uploadPhoto')}</button><button className="text-button" disabled={!!busy} onClick={() => setFile(null)}>{t('cancel')}</button></>}</div></>}
      {gallery.photos.length >= MAX_PHOTOS && <p className="photo-limit-note">{t('photoLimit')}</p>}
      {validation && <ErrorNotice text={validation} retry={unconfirmed ? () => void refresh() : undefined} />}{error != null && <ErrorNotice error={error} retry={() => void refresh()} />}
    </div>
    {gallery.photos.length ? <div className="bike-photo-grid">{gallery.photos.map((photo, index) => <GalleryPhoto key={photo.id} photo={photo} number={index + 1} selected={photo.id === gallery.cover_photo_id} actions={(failed) => <><button className="text-button" onClick={() => cover(photo.id)} disabled={failed || !!busy || !online || photo.id === gallery.cover_photo_id}>{photo.id === gallery.cover_photo_id ? <Check size={14} /> : <ImagePlus size={14} />}{t(photo.id === gallery.cover_photo_id ? 'photoCover' : 'usePhotoCover')}</button><button className="text-button danger-text" onClick={() => void remove(photo)} disabled={!!busy || !online}><Trash2 size={14} />{t('deletePhoto')}</button></>} />)}</div> : <p className="small-empty photo-gallery-empty">{t('noBikePhotos')}</p>}
  </section>
}
