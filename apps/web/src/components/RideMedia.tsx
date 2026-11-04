import { useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowUpRightIcon as ArrowUpRight, ImageSquareIcon as ImagePlus, LinkSimpleIcon as Link2, TrashIcon as Trash2, UploadSimpleIcon as Upload, XIcon as X } from '@phosphor-icons/react'
import { ApiError, request, uploadFile } from '../api'
import { useI18n } from '../i18n'
import { useSession } from '../session'
import { useSessionTask } from '../useSessionTask'
import type { SessionTask } from '../useSessionTask'
import { ErrorNotice, Field, Loading, useConfirmation, useOnline } from '../ui'
import type { Ride, RideAlbum, RidePhoto, RideVideo } from '../types'
import { safeMediaPhoto } from './Avatar'
import { LocalPhotoPreview, photoFileError } from './PhotoInput'
import { ReportButton } from './Trust'

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i
export function safeVideoUrl(value: string) {
  try {
    const url = new URL(value)
    const host = url.hostname.toLowerCase()
    if (url.protocol !== 'https:' || url.username || url.password || !host.includes('.') || host.startsWith('[') || /^\d+(?:\.\d+){3}$/.test(host) || /(?:^|\.)(?:localhost|local|internal)$/.test(host) || url.port === '0') return undefined
    return url.href
  } catch { return undefined }
}
export function safeRideAlbum(value: unknown, rideId: string): RideAlbum {
  if (!uuid.test(rideId) || !value || typeof value !== 'object' || !('photos' in value) || !Array.isArray(value.photos) || !('videos' in value) || !Array.isArray(value.videos) || !('version' in value) || !Number.isSafeInteger(value.version) || Number(value.version) < 1) throw new ApiError(502, 'request_failed')
  const photos: RidePhoto[] = []
  for (const item of value.photos) {
    if (!item || typeof item !== 'object' || typeof item.uploader_id !== 'string') continue
    const can_delete = item.can_delete === true
    const hidden = item.hidden === true
    if (hidden) {
      if (can_delete && typeof item.id === 'string' && uuid.test(item.id) && typeof item.created_at === 'string') photos.push({ id: item.id, url: '', width: 1, height: 1, created_at: item.created_at, uploader_id: item.uploader_id, can_delete, hidden: true })
    } else {
      const photo = safeMediaPhoto(item, (id) => `/api/v1/rides/${rideId}/album/photos/${id}/content`)
      if (photo) photos.push({ ...photo, uploader_id: item.uploader_id, can_delete })
    }
  }
  const videos: RideVideo[] = []
  for (const item of value.videos) {
    if (!item || typeof item !== 'object' || typeof item.id !== 'string' || !uuid.test(item.id) || typeof item.uploader_id !== 'string' || typeof item.created_at !== 'string') continue
    const can_delete = item.can_delete === true
    if (item.hidden === true) { if (can_delete) videos.push({ id: item.id, url: '', title: '', uploader_id: item.uploader_id, created_at: item.created_at, can_delete, hidden: true }); continue }
    const url = typeof item.url === 'string' ? safeVideoUrl(item.url) : undefined
    if (url && typeof item.title === 'string') videos.push({ id: item.id, url, title: item.title, uploader_id: item.uploader_id, created_at: item.created_at, can_delete })
  }
  return { photos: photos.slice(0, 24), videos: videos.slice(0, 12), version: Number(value.version) }
}
export function liveRideAccess(client: ReturnType<typeof useQueryClient>, ride: Ride, actor?: string, write = false) {
  const state = client.getQueryState<{ ride: Ride }>(['ride', ride.id, actor ?? 'guest'])
  const current = state?.data?.ride
  return !!actor && state?.status === 'success' && !!current?.title && (current.can_manage || current.participation_state === 'approved') && (!write || (current.status !== 'cancelled' && current.plan_version === ride.plan_version))
}
export function RideMedia({ ride, onAccessLost }: { ride: Ride; onAccessLost: () => void }) {
  const { user } = useSession()
  const { t } = useI18n()
  const key = ['ride-album', ride.id, user?.id]
  const query = useQuery({ queryKey: key, queryFn: async ({ signal }) => safeRideAlbum(await request(`/rides/${ride.id}/album`, { signal }), ride.id), enabled: !!user, meta: { private: true }, retry: false, refetchInterval: 15_000 })
  useEffect(() => { if (query.error instanceof ApiError && [401, 403, 404].includes(query.error.status)) onAccessLost() }, [query.error, onAccessLost])
  return <section className="ride-media-section"><div className="section-header"><div><span className="eyebrow">{t('rideRoom')}</span><h2>{t('rideAlbum')}</h2></div></div><p className="section-intro">{t('albumText')}</p>{query.isPending ? <Loading /> : query.isError ? <ErrorNotice error={query.error} retry={() => void query.refetch()} /> : <AlbumContent key={`${ride.id}:${user?.id}:${ride.plan_version}:${ride.status}`} ride={ride} album={query.data} queryKey={key} reload={() => query.refetch()} />}</section>
}
function AlbumContent({ ride, album, queryKey, reload }: { ride: Ride; album: RideAlbum; queryKey: (string | undefined)[]; reload: () => Promise<unknown> }) {
  const { user } = useSession()
  const { t } = useI18n()
  const { confirm, dialog } = useConfirmation()
  const client = useQueryClient()
  const start = useSessionTask()
  const active = useRef<SessionTask | null>(null)
  const online = useOnline()
  const [file, setFile] = useState<File | null>(null)
  const [video, setVideo] = useState({ url: '', title: '' })
  const [busy, setBusy] = useState('')
  const [validation, setValidation] = useState('')
  const [error, setError] = useState<unknown>()
  const [uncertain, setUncertain] = useState(false)
  const readonly = ride.status === 'cancelled'
  const fullPhotos = album.photos.length >= 24 || album.photos.filter((photo) => photo.uploader_id === user?.id).length >= 6
  const fullVideos = album.videos.length >= 12 || album.videos.filter((item) => item.uploader_id === user?.id).length >= 3
  async function command(kind: string, send: (signal: AbortSignal) => Promise<unknown>, confirmation?: string) {
    if (active.current) return
    const task = start(); active.current = task
    const write = kind === 'upload' || kind === 'video'
    const current = () => task.current() && liveRideAccess(client, ride, user?.id, write)
    if ((confirmation && !await confirm({ message: confirmation, action: t(kind === 'delete-photo' ? 'deletePhoto' : 'deleteVideo'), destructive: true })) || !current()) { active.current = null; task.finish(); return }
    setBusy(kind); setError(undefined); setValidation('')
    try {
      const value = await send(task.signal)
      if (!current()) return
      const raw = value && typeof value === 'object' && 'album' in value ? value.album : value
      client.setQueryDefaults(queryKey, { meta: { private: true } }); client.setQueryData(queryKey, safeRideAlbum(raw, ride.id))
      if (kind === 'upload') setFile(null)
      if (kind === 'video') setVideo({ url: '', title: '' })
      setUncertain(false)
    } catch (value) { if (current()) { setError(value); if (value instanceof ApiError && (value.status === 0 || value.status >= 500)) setUncertain(true); if (value instanceof ApiError && value.code === 'stale_version') void reload() } }
    finally { if (task.current()) setBusy(''); if (active.current === task) active.current = null; task.finish() }
  }
  function cancel() { active.current?.abort(); active.current = null; setFile(null); setBusy(''); setError(undefined); setUncertain(false); if (liveRideAccess(client, ride, user?.id)) void reload() }
  async function refresh() { const task = start(); if (!task.current()) { task.finish(); return }; await reload(); if (task.current() && liveRideAccess(client, ride, user?.id)) { setUncertain(false); setError(undefined); setValidation('') } task.finish() }
  function addVideo(event: FormEvent) {
    event.preventDefault()
    const url = safeVideoUrl(video.url.trim())
    if (!url || new TextEncoder().encode(url).length > 2048 || new TextEncoder().encode(video.title.trim()).length > 160) { setValidation(t('httpsVideoRequired')); return }
    void command('video', (signal) => request(`/rides/${ride.id}/album/videos`, { method: 'POST', signal, body: JSON.stringify({ url, title: video.title.trim() }) }))
  }
  return <>{dialog}{!readonly && <div className="album-editors"><section className="surface photo-upload-panel"><div className="photo-upload-heading"><span className="panel-icon"><ImagePlus size={21} /></span><div><label className="photo-picker-label" htmlFor={`album-file-${ride.id}`}>{t('choosePhoto')}</label><p>{t('albumPhotoLimit')}</p></div></div><input id={`album-file-${ride.id}`} className="photo-file-input" type="file" accept="image/jpeg,image/png,image/webp,.jpg,.jpeg,.png,.webp" disabled={!!busy || !online || fullPhotos} onChange={(event) => { const next = event.target.files?.[0]; event.target.value = ''; setFile(null); setValidation(''); setError(undefined); setUncertain(false); if (next) { const invalid = photoFileError(next); if (invalid) setValidation(t(invalid)); else setFile(next) } }} />{file && <><LocalPhotoPreview file={file} /><div className="photo-upload-actions">{busy === 'upload' ? <><div className="photo-upload-progress" role="progressbar" aria-label={t('uploadingPhoto')}><span className="spinner" />{t('uploadingPhoto')}</div><button className="button button-outline" onClick={cancel}><X size={15} />{t('cancelUpload')}</button></> : <><button className="button button-primary" disabled={!!busy || !online || uncertain || fullPhotos} onClick={() => void command('upload', (signal) => uploadFile(`/rides/${ride.id}/album/photos`, file, signal))}><Upload size={15} />{t('uploadPhoto')}</button><button className="text-button" disabled={!!busy} onClick={() => setFile(null)}>{t('cancel')}</button></>}</div></>}</section><form className="surface form-section album-video-form" onSubmit={addVideo}><h3><Link2 size={18} />{t('addVideo')}</h3><Field label={t('videoUrl')} type="url" value={video.url} maxLength={2048} required disabled={!!busy || fullVideos} onChange={(event) => setVideo({ ...video, url: event.target.value })} /><Field label={`${t('videoTitle')} · ${t('optional')}`} value={video.title} maxLength={160} disabled={!!busy || fullVideos} onChange={(event) => setVideo({ ...video, title: event.target.value })} /><p className="field-hint">{t('videoLinkHint')}</p><button className="button button-outline" disabled={!!busy || !online || fullVideos || uncertain} type="submit">{t(busy === 'video' ? 'saving' : 'addVideo')}</button></form></div>}{validation && <ErrorNotice text={validation} />}{error != null && <ErrorNotice error={error} retry={() => void refresh()} />}
    {!album.photos.length && !album.videos.length && <p className="small-empty surface album-empty">{t('albumEmpty')}</p>}<div className="bike-photo-grid">{album.photos.map((photo, index) => <AlbumPhoto key={photo.id} photo={photo} number={index + 1} busy={!!busy || !online} remove={() => void command('delete-photo', (signal) => request(`/rides/${ride.id}/album/photos/${photo.id}`, { method: 'DELETE', signal, body: JSON.stringify({ version: album.version }) }), t('deleteRidePhotoConfirm'))} />)}</div>{!!album.videos.length && <div className="album-videos">{album.videos.map((item) => <article className="surface album-video" key={item.id}><div>{item.hidden ? <p>{t('hiddenModeration')}</p> : <><a className="inline-link" href={item.url} target="_blank" rel="noopener noreferrer">{item.title || new URL(item.url).hostname}<ArrowUpRight size={15} /></a><p>{new URL(item.url).hostname}</p></>}</div><div className="entry-actions">{!item.hidden && <ReportButton targetType="ride_video" targetId={item.id} />}{item.can_delete && <button className="text-button danger-text" disabled={!!busy || !online} onClick={() => void command('delete-video', (signal) => request(`/rides/${ride.id}/album/videos/${item.id}`, { method: 'DELETE', signal, body: JSON.stringify({ version: album.version }) }), t('deleteVideoConfirm'))}><Trash2 size={14} />{t('deleteVideo')}</button>}</div></article>)}</div>}</>
}
function AlbumPhoto({ photo, number, busy, remove }: { photo: RidePhoto; number: number; busy: boolean; remove: () => void }) {
  const { t } = useI18n()
  const [failed, setFailed] = useState(false)
  return <article className="surface bike-photo"><div className="bike-photo-frame">{photo.hidden ? <p className="small-empty">{t('hiddenModeration')}</p> : failed ? <p className="small-empty">{t('photoUnavailable')}</p> : <img src={photo.url} width={photo.width} height={photo.height} alt={t('albumPhoto', { number })} loading="lazy" decoding="async" referrerPolicy="no-referrer" onError={() => setFailed(true)} />}</div><div className="bike-photo-actions">{!photo.hidden && <ReportButton targetType="ride_photo" targetId={photo.id} />}{photo.can_delete && <button className="text-button danger-text" disabled={busy} onClick={remove}><Trash2 size={14} />{t('deletePhoto')}</button>}</div></article>
}
