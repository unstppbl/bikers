import { useRef, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ImagePlus, ShieldCheck, Trash2, Upload, X } from 'lucide-react'
import { ApiError, getSessionEpoch, request, uploadFile } from '../api'
import { useI18n } from '../i18n'
import { useSession } from '../session'
import { useSessionTask } from '../useSessionTask'
import type { SessionTask } from '../useSessionTask'
import { ErrorNotice, Loading, initials, useConfirmation, useOnline } from '../ui'
import type { AvatarMetadata, BikePhoto } from '../types'
import { LocalPhotoPreview, photoFileError } from './PhotoInput'

export interface AvatarScope { kind: 'ride' | 'club'; id: string }
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i
export function safeMediaPhoto(value: unknown, url: (id: string) => string): BikePhoto | null {
  if (!value || typeof value !== 'object' || !('id' in value) || !('url' in value) || !('width' in value) || !('height' in value) || !('created_at' in value)) return null
  const { id, width, height, created_at } = value
  return typeof id === 'string' && uuid.test(id) && value.url === url(id) && typeof width === 'number' && Number.isSafeInteger(width) && width > 0 && width <= 1920 && typeof height === 'number' && Number.isSafeInteger(height) && height > 0 && height <= 1920 && typeof created_at === 'string' ? { id, url: url(id), width, height, created_at } : null
}
export function safeAvatar(value: unknown, base: string, self: boolean): AvatarMetadata {
  if (!value || typeof value !== 'object' || !('image' in value)) throw new ApiError(502, 'request_failed')
  const image = safeMediaPhoto(value.image, (id) => `/api/v1${base}/${id}/content`)
  const version = self && 'version' in value && Number.isSafeInteger(value.version) && Number(value.version) > 0 ? Number(value.version) : undefined
  return { image, ...(version === undefined ? {} : { version }) }
}
export function useAvatarMetadata(userId: string, scope?: AvatarScope) {
  const { user, loading, error } = useSession()
  const self = !scope && user?.id === userId
  const base = scope ? `/${scope.kind === 'ride' ? 'rides' : 'clubs'}/${encodeURIComponent(scope.id)}/avatars/${encodeURIComponent(userId)}` : '/me/avatar'
  const ready = !!user && !loading && !error && (!!scope || self)
  const queryKey = ['avatar', user?.id, scope?.kind ?? 'self', scope?.id ?? '', userId]
  const query = useQuery({ queryKey, queryFn: async ({ signal }) => safeAvatar(await request(base, { signal }), base, self), enabled: ready, meta: { private: true }, retry: false, staleTime: 30_000 })
  return { query, queryKey, ready, base }
}
export function Avatar({ userId, name, scope, className = '' }: { userId: string; name: string; scope?: AvatarScope; className?: string }) {
  const { query, ready } = useAvatarMetadata(userId, scope)
  const image = ready && !query.isError ? query.data?.image : null
  return <span className={`avatar ${className}`}>{image ? <AvatarImage key={image.id} image={image} name={name} /> : initials(name)}</span>
}
function AvatarImage({ image, name }: { image: BikePhoto; name: string }) {
  const { t } = useI18n()
  const [failed, setFailed] = useState(false)
  return failed ? initials(name) : <img src={image.url} width={image.width} height={image.height} alt={t('avatarAlt', { name })} loading="lazy" decoding="async" referrerPolicy="no-referrer" onError={() => setFailed(true)} />
}
export function AvatarEditor() {
  const { user } = useSession()
  const { t } = useI18n()
  const { query, queryKey, base, ready } = useAvatarMetadata(user?.id ?? '')
  if (!ready) return null
  if (query.isPending) return <Loading />
  if (query.isError) return <ErrorNotice error={query.error} retry={() => void query.refetch()} />
  if (query.data.version === undefined) return <ErrorNotice text={t('noAccess')} />
  return <AvatarForm key={`${user!.id}:${getSessionEpoch()}`} metadata={query.data} queryKey={queryKey} base={base} reload={() => query.refetch()} />
}
function AvatarForm({ metadata, queryKey, base, reload }: { metadata: AvatarMetadata; queryKey: (string | undefined)[]; base: string; reload: () => Promise<unknown> }) {
  const { user } = useSession()
  const { t } = useI18n()
  const { confirm, dialog } = useConfirmation()
  const client = useQueryClient()
  const start = useSessionTask()
  const online = useOnline()
  const active = useRef<SessionTask | null>(null)
  const [file, setFile] = useState<File | null>(null)
  const [busy, setBusy] = useState('')
  const [validation, setValidation] = useState('')
  const [error, setError] = useState<unknown>()
  const [uncertain, setUncertain] = useState(false)
  async function operate(remove = false) {
    if (active.current) return
    const task = start(); active.current = task
    if (remove && (!await confirm({ message: t('avatarRemoveConfirm'), action: t('avatarRemove'), destructive: true }) || !task.current())) { active.current = null; task.finish(); return }
    if (!task.current()) { task.finish(); active.current = null; return }
    setBusy(remove ? 'remove' : 'upload'); setError(undefined); setValidation('')
    try {
      const result = remove ? await request(base, { method: 'DELETE', signal: task.signal, body: JSON.stringify({ version: metadata.version }) }) : await uploadFile(`${base}?version=${metadata.version}`, file!, task.signal)
      if (!task.current()) return
      client.setQueryDefaults(queryKey, { meta: { private: true } }); client.setQueryData(queryKey, safeAvatar(result, base, true))
      void client.invalidateQueries({ queryKey: ['avatar'], refetchType: 'none' })
      setFile(null); setUncertain(false)
    } catch (value) {
      if (task.current()) {
        if (!remove && value instanceof ApiError && (value.status === 0 || value.status >= 500)) { setUncertain(true); setValidation(t('photoUploadUnconfirmed')) } else setError(value)
        if (value instanceof ApiError && value.code === 'stale_version') void reload()
      }
    } finally { if (task.current()) setBusy(''); if (active.current === task) active.current = null; task.finish() }
  }
  function cancel() { const task = start(); const refreshAllowed = task.current(); task.finish(); active.current?.abort(); active.current = null; setFile(null); setBusy(''); setValidation(''); setError(undefined); setUncertain(false); if (refreshAllowed) void reload() }
  async function refresh() { const task = start(); if (!task.current()) { task.finish(); return }; await reload(); if (task.current()) { setUncertain(false); setValidation(''); setError(undefined) } task.finish() }
  return <section className="surface photo-upload-panel avatar-editor">{dialog}<div className="club-image-editor-heading"><Avatar userId={user!.id} name={user!.profile.display_name} className="avatar-large" /><div><h2>{t('avatarTitle')}</h2>{!metadata.image && <p>{t('avatarEmpty')}</p>}</div>{metadata.image && <button className="text-button danger-text" disabled={!!busy || !online} onClick={() => void operate(true)}><Trash2 size={15} />{t('avatarRemove')}</button>}</div><div className="inline-note"><ShieldCheck size={18} /><p>{t('avatarPrivacy')}</p></div><div className="photo-upload-heading"><span className="panel-icon"><ImagePlus size={22} /></span><div><label className="photo-picker-label" htmlFor="rider-photo-file">{t('choosePhoto')}</label><p>{t('clubImageHint')}</p></div></div><input id="rider-photo-file" className="photo-file-input" type="file" accept="image/jpeg,image/png,image/webp,.jpg,.jpeg,.png,.webp" disabled={!!busy || !online} onChange={(event) => { const chosen = event.target.files?.[0]; event.target.value = ''; setFile(null); setValidation(''); setError(undefined); setUncertain(false); if (chosen) { const invalid = photoFileError(chosen); if (invalid) setValidation(t(invalid)); else setFile(chosen) } }} />{file && <><LocalPhotoPreview file={file} /><div className="photo-upload-actions">{busy === 'upload' ? <><div role="progressbar" aria-label={t('uploadingPhoto')} className="photo-upload-progress"><span className="spinner" />{t('uploadingPhoto')}</div><button className="button button-outline" onClick={cancel}><X size={15} />{t('cancelUpload')}</button></> : <><button className="button button-primary" disabled={!!busy || !online || uncertain} onClick={() => void operate()}><Upload size={15} />{t(metadata.image ? 'avatarReplace' : 'avatarUpload')}</button><button className="text-button" disabled={!!busy} onClick={() => setFile(null)}>{t('cancel')}</button></>}</div></>}{validation && <ErrorNotice text={validation} retry={uncertain ? () => void refresh() : undefined} />}{error != null && <ErrorNotice error={error} retry={() => void refresh()} />}</section>
}
