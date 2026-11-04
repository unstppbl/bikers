import { useEffect, useRef, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { GlobeIcon as Globe, ImageSquareIcon as ImagePlus, TrashIcon as Trash2, UploadSimpleIcon as Upload, XIcon as X } from '@phosphor-icons/react'
import { ApiError, getSessionEpoch, request, uploadFile } from '../api'
import { ReportButton } from './Trust'
import { useI18n } from '../i18n'
import { useSession, useSessionGuard } from '../session'
import { ErrorNotice, Loading, initials, useConfirmation, useOnline } from '../ui'
import type { Club, ClubImageMetadata, ClubImagePhoto, ClubSummary } from '../types'
import { LocalPhotoPreview, photoFileError } from './PhotoInput'

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i

export function safeClubImage(value: unknown, clubId: string, owner: boolean): ClubImageMetadata {
  if (!uuid.test(clubId) || !value || typeof value !== 'object' || !('image' in value)) throw new ApiError(502, 'request_failed')
  let image: ClubImagePhoto | null = null
  const item = value.image
  if (item && typeof item === 'object' && 'id' in item && 'url' in item && 'width' in item && 'height' in item && 'created_at' in item) {
    const { id, url, width, height, created_at } = item
    if (typeof id === 'string' && uuid.test(id) && url === `/api/v1/clubs/${clubId}/image/${id}/content` && typeof width === 'number' && Number.isSafeInteger(width) && width > 0 && width <= 1920 && typeof height === 'number' && Number.isSafeInteger(height) && height > 0 && height <= 1920 && typeof created_at === 'string') image = { id, url, width, height, created_at }
  }
  const version = owner && 'version' in value && Number.isSafeInteger(value.version) && Number(value.version) > 0 ? Number(value.version) : undefined
  return { image, ...(version === undefined ? {} : { version }) }
}

function useClubImage(club: ClubSummary, ownerView: boolean) {
  const { user, loading, error } = useSession()
  const ready = !loading && !error
  const owner = !!user && ownerView && club.status === 'active'
  const queryKey = ['club-image', club.id, user?.id ?? 'guest', owner ? 'owner' : 'public']
  const query = useQuery({
    queryKey,
    queryFn: async ({ signal }) => safeClubImage(await request<unknown>(`/clubs/${club.id}/image`, { signal }), club.id, owner),
    meta: { private: true },
    enabled: ready,
    staleTime: 30_000,
    retry: false,
  })
  return { query, queryKey, ready }
}

export function ClubEmblem({ club, variant = 'card', ownerView = club.can_manage === true }: { club: ClubSummary; variant?: 'card' | 'detail' | 'editor'; ownerView?: boolean }) {
  const { query, ready } = useClubImage(club, ownerView)
  const photo = ready && !query.isError ? query.data?.image : null
  return <span className={`club-emblem club-emblem-${variant}`}>{photo ? <EmblemPhoto key={photo.id} photo={photo} name={club.name} /> : initials(club.name)}</span>
}

function EmblemPhoto({ photo, name }: { photo: ClubImagePhoto; name: string }) {
  const { t } = useI18n()
  const [failed, setFailed] = useState(false)
  return failed ? initials(name) : <img src={photo.url} width={photo.width} height={photo.height} alt={t('clubImageAlt', { name })} decoding="async" loading="lazy" referrerPolicy="no-referrer" onError={() => setFailed(true)} />
}

export function ClubImageEditor({ club }: { club: Club }) {
  const { user, loading, error } = useSession()
  const { t } = useI18n()
  const canManage = !!user && user.id === club.owner_id && club.can_manage === true && club.status === 'active' && !loading && !error
  const { query, queryKey } = useClubImage(club, canManage)
  if (!canManage) return null
  if (query.isPending) return <Loading />
  if (query.isError) return <section className="club-image-section"><ErrorNotice error={query.error} retry={() => void query.refetch()} /></section>
  if (query.data.version === undefined) return <section className="club-image-section"><ErrorNotice text={t('noAccess')} retry={() => void query.refetch()} /></section>
  return <ImageEditor key={`${club.id}:${user.id}:${getSessionEpoch()}:${club.version}:${club.owner_id}`} club={club} metadata={query.data} queryKey={queryKey} reload={() => query.refetch()} />
}

function ImageEditor({ club, metadata, queryKey, reload }: { club: Club; metadata: ClubImageMetadata; queryKey: string[]; reload: () => Promise<unknown> }) {
  const { user } = useSession()
  const { t } = useI18n()
  const { confirm, dialog } = useConfirmation()
  const queryClient = useQueryClient()
  const captureSession = useSessionGuard()
  const online = useOnline()
  const active = useRef<AbortController | null>(null)
  const [file, setFile] = useState<File | null>(null)
  const [busy, setBusy] = useState('')
  const [validation, setValidation] = useState('')
  const [error, setError] = useState<unknown>()
  const [unconfirmed, setUnconfirmed] = useState(false)
  useEffect(() => () => { active.current?.abort() }, [])

  // Ownership can change while the same user stays signed in. Check the live
  // club query as well as the session epoch before accepting any completion.
  function currentOwner() {
    const state = queryClient.getQueryState<{ club: Club | ClubSummary }>(['club', club.id, user?.id ?? 'guest'])
    const current = state?.data?.club
    return state?.status === 'success' && current?.status === 'active' && current.can_manage === true && 'owner_id' in current && current.owner_id === user?.id && 'version' in current && current.version === club.version
  }
  function refreshClub() { void queryClient.invalidateQueries({ queryKey: ['club', club.id] }) }
  async function operate(kind: string, command: (signal: AbortSignal) => Promise<unknown>) {
    if (active.current || !currentOwner()) return
    const currentSession = captureSession()
    const controller = new AbortController()
    active.current = controller
    const current = () => !controller.signal.aborted && active.current === controller && currentSession() && currentOwner()
    setBusy(kind); setError(undefined); setValidation('')
    try {
      const result = await command(controller.signal)
      if (!current()) return
      const next = safeClubImage(result, club.id, true)
      queryClient.setQueryDefaults(queryKey, { meta: { private: true } })
      queryClient.setQueryData(queryKey, next)
      setFile(null); setUnconfirmed(false)
      void queryClient.invalidateQueries({ queryKey: ['club-image', club.id], refetchType: 'none' })
    } catch (value) {
      if (!current()) return
      if (kind === 'upload' && value instanceof ApiError && (value.status === 0 || value.status >= 500)) { setUnconfirmed(true); setValidation(t('clubImageUnconfirmed')) }
      else setError(value)
      if (value instanceof ApiError && ([401, 403, 404].includes(value.status) || ['stale_version', 'club_closed'].includes(value.code))) { refreshClub(); void reload() }
    } finally {
      if (active.current === controller && !controller.signal.aborted && currentSession()) { active.current = null; setBusy('') }
    }
  }
  function cancel() {
    active.current?.abort(); active.current = null
    setBusy(''); setFile(null); setValidation(''); setError(undefined); setUnconfirmed(false)
    if (currentOwner()) void reload()
  }
  async function remove() {
    const currentSession = captureSession()
    if (!await confirm({ message: t('clubImageRemoveConfirm'), action: t('clubImageRemove'), destructive: true }) || !currentSession() || !currentOwner()) return
    await operate('remove', (signal) => request(`/clubs/${club.id}/image`, { method: 'DELETE', signal, body: JSON.stringify({ version: metadata.version }) }))
  }
  async function refresh() {
    const currentSession = captureSession()
    const result = await reload()
    if (currentSession() && currentOwner() && result && typeof result === 'object' && 'isError' in result && !result.isError) { setUnconfirmed(false); setValidation(''); setError(undefined) }
  }
  const specificError = error instanceof ApiError && error.code === 'invalid_photo_choice' ? t('clubImageChanged') : error instanceof ApiError && error.code === 'photo_limit' ? t('clubImageBusy') : undefined
  return <section className="surface photo-upload-panel club-image-section">{dialog}<div className="club-image-editor-heading"><ClubEmblem club={club} variant="editor" ownerView /><div><h2>{t('clubImage')}</h2><p>{t(metadata.image ? 'clubImageReplacement' : 'clubImageEmpty')}</p></div>{metadata.image && <button className="text-button danger-text" disabled={!!busy || !online} onClick={() => void remove()}><Trash2 size={15} />{t('clubImageRemove')}</button>}</div><div className="inline-note club-image-public-note"><Globe size={18} /><p>{t('clubImagePublic')}</p></div><div className="photo-upload-heading"><span className="panel-icon"><ImagePlus size={22} /></span><div><label className="photo-picker-label" htmlFor={`club-image-file-${club.id}`}>{t('choosePhoto')}</label><p>{t('clubImageHint')}</p></div></div><input id={`club-image-file-${club.id}`} className="photo-file-input" type="file" accept="image/jpeg,image/png,image/webp,.jpg,.jpeg,.png,.webp" disabled={!!busy || !online} onChange={(event) => {
      const chosen = event.target.files?.[0]
      event.target.value = ''
      setFile(null); setValidation(''); setError(undefined); setUnconfirmed(false)
      if (!chosen) return
      const invalid = photoFileError(chosen)
      if (invalid) { setValidation(t(invalid)); return }
      setFile(chosen)
    }} />
    {file && <><LocalPhotoPreview key={`${file.name}:${file.size}:${file.lastModified}`} file={file} /><div className="photo-upload-actions">{busy === 'upload' ? <><div className="photo-upload-progress" role="progressbar" aria-label={t('uploadingPhoto')}><span className="spinner" />{t('uploadingPhoto')}</div><button className="button button-outline" onClick={cancel}><X size={15} />{t('cancelUpload')}</button></> : <><button className="button button-primary" disabled={!!busy || !online || unconfirmed} onClick={() => void operate('upload', (signal) => uploadFile(`/clubs/${club.id}/image?version=${metadata.version}`, file, signal))}><Upload size={15} />{t(metadata.image ? 'clubImageReplace' : 'clubImageUpload')}</button><button className="text-button" disabled={!!busy} onClick={() => setFile(null)}>{t('cancel')}</button></>}</div></>}
    {validation && <ErrorNotice text={validation} retry={unconfirmed ? () => void refresh() : undefined} />}{error != null && <ErrorNotice {...(specificError ? { text: specificError } : { error })} retry={() => void refresh()} />}
  </section>
}

export function ClubImageReport({ club }: { club: ClubSummary }) {
  const { query, ready } = useClubImage(club, false)
  return ready && !query.isError && query.data?.image ? <ReportButton targetType="club_image" targetId={query.data.image.id} /> : null
}
