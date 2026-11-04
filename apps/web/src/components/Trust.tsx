import { useState } from 'react'
import type { FormEvent } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ProhibitIcon as Ban, FlagIcon as Flag, ShieldCheckIcon as ShieldCheck } from '@phosphor-icons/react'
import { Link, useNavigate } from 'react-router-dom'
import { ApiError, getSessionEpoch, request } from '../api'
import { useI18n } from '../i18n'
import { useSession, useSessionGuard } from '../session'
import { useSessionTask } from '../useSessionTask'
import { ErrorNotice, Field, Loading, Pagination, TextArea, useConfirmation, useOnline } from '../ui'
import type { BlockedRider, ReportTarget } from '../types'
import { resetTrustQueries } from '../trust-cache'
import { useAvatarMetadata } from './Avatar'
import type { AvatarScope } from './Avatar'

export const reportReasons = ['spam', 'harassment', 'dangerous_content', 'other'] as const
export function useTrust() {
  const { user, loading, error } = useSession()
  return useQuery({ queryKey: ['trust', user?.id], queryFn: ({ signal }) => request<{ is_admin: boolean }>('/me/trust', { signal }), enabled: !!user && !loading && !error, meta: { private: true }, retry: false, staleTime: 30_000, refetchInterval: user ? 30_000 : false })
}
export function ReportButton({ targetType, targetId, label = 'report' }: { targetType: ReportTarget; targetId: string; label?: 'report' | 'reportRider' | 'reportAvatar' }) {
  const { user, loading, error } = useSession()
  if (!user || loading || error) return null
  return <ReportForm key={`${user.id}:${getSessionEpoch()}:${targetType}:${targetId}`} targetType={targetType} targetId={targetId} label={label} />
}
function ReportForm({ targetType, targetId, label }: { targetType: ReportTarget; targetId: string; label: 'report' | 'reportRider' | 'reportAvatar' }) {
  const { t } = useI18n()
  const start = useSessionTask()
  const online = useOnline()
  const [open, setOpen] = useState(false)
  const [reason, setReason] = useState<typeof reportReasons[number]>('spam')
  const [details, setDetails] = useState('')
  const [busy, setBusy] = useState(false)
  const [sent, setSent] = useState(false)
  const [error, setError] = useState<unknown>()
  async function submit(event: FormEvent) {
    event.preventDefault()
    if (new TextEncoder().encode(details.trim()).length > 4000) { setError(new ApiError(422, 'invalid_report')); return }
    const task = start(); if (!task.current()) { task.finish(); return }; setBusy(true); setError(undefined)
    try { await request('/reports', { method: 'POST', signal: task.signal, body: JSON.stringify({ target_type: targetType, target_id: targetId, reason, details: details.trim() }) }); if (task.current()) { setOpen(false); setSent(true); setDetails('') } }
    catch (value) { if (task.current()) setError(value) }
    finally { if (task.current()) setBusy(false); task.finish() }
  }
  return <div className={`report-control ${open ? 'report-control-open' : ''}`}><button type="button" className="text-button report-button" aria-expanded={open} onClick={() => { setOpen(!open); setSent(false) }}><Flag size={13} />{t(label)}</button>{sent && <span className="success-message" role="status">{t('reportSent')}</span>}{open && <form className="surface report-form" onSubmit={(event) => void submit(event)}><h3>{t(label)}</h3><p>{t('reportHint')}</p><Field label={t('reportReason')}><select value={reason} disabled={busy} onChange={(event) => setReason(event.target.value as typeof reason)}>{reportReasons.map((value) => <option key={value} value={value}>{t(`reason_${value}`)}</option>)}</select></Field><TextArea label={t('reportDetails')} value={details} onChange={setDetails} maxLength={4000} rows={3} />{error != null && <ErrorNotice error={error} />}<div className="form-actions"><button className="button button-outline" type="button" disabled={busy} onClick={() => setOpen(false)}>{t('cancel')}</button><button className="button button-primary" type="submit" disabled={busy || !online}>{t(busy ? 'sending' : 'reportSubmit')}</button></div></form>}</div>
}
export function useBlocks() {
  const { user, loading, error } = useSession()
  return useQuery({
    queryKey: ['blocks', user?.id], enabled: !!user && !loading && !error, meta: { private: true }, retry: false, staleTime: 30_000, refetchOnMount: 'always', refetchOnWindowFocus: 'always', refetchInterval: 30_000,
    queryFn: async ({ signal }) => {
      const blocks: BlockedRider[] = []; let cursor = ''
      for (let page = 0; page < 10; page++) {
        const value = await request<{ blocks: BlockedRider[]; next_cursor: string }>(`/me/blocks${cursor ? `?after=${encodeURIComponent(cursor)}` : ''}`, { signal })
        blocks.push(...value.blocks.map(({ user_id, display_name, created_at }) => ({ user_id, display_name, created_at })))
        if (!value.next_cursor) return blocks
        if (value.next_cursor === cursor) throw new ApiError(502, 'request_failed')
        cursor = value.next_cursor
      }
      throw new ApiError(502, 'request_failed')
    },
  })
}
export function BlockButton({ userId, name }: { userId: string; name: string }) {
  const { user, error: sessionError } = useSession()
  const { t } = useI18n()
  const blocks = useBlocks()
  const { confirm, dialog } = useConfirmation()
  const client = useQueryClient()
  const start = useSessionTask()
  const navigate = useNavigate()
  const online = useOnline()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<unknown>()
  const captureSession = useSessionGuard()
  const blocked = blocks.data?.some((item) => item.user_id === userId) === true
  async function change() {
    if (!user) { void navigate('/login'); return }
    const currentSession = captureSession()
    const sessionCurrent = () => currentSession() && client.getQueryState(['session'])?.status === 'success'
    const actorId = user.id
    const task = start()
    if (!blocked && (!await confirm({ message: t('blockConfirm', { name }), action: t('blockRider') }) || !task.current())) { task.finish(); return }
    if (!task.current()) { task.finish(); return }
    setBusy(true); setError(undefined)
    try {
      await request(`/me/blocks/${encodeURIComponent(userId)}`, { method: blocked ? 'DELETE' : 'PUT', signal: task.signal })
      // Trust hints can unmount this button before the HTTP reply arrives.
      // Cache reconciliation belongs to the authenticated session, not this view.
      if (sessionCurrent()) await resetTrustQueries(client)
    } catch (value) { if (task.current()) setError(value) }
    finally {
      if (sessionCurrent()) await client.invalidateQueries({ queryKey: ['blocks', actorId] })
      if (task.current()) setBusy(false)
      task.finish()
    }
  }
  if (!user || user.id === userId || sessionError) return null
  return <div className="block-control">{dialog}<button type="button" className="text-button" disabled={busy || !online || blocks.isPending || blocks.isError} onClick={() => void change()}><Ban size={13} />{t(blocked ? 'unblockRider' : 'blockRider')}</button>{error != null && <ErrorNotice error={error} />}</div>
}
export function ManageUserLink({ userId }: { userId: string }) {
  const { t } = useI18n()
  const trust = useTrust()
  return trust.data?.is_admin ? <Link className="inline-link manage-rider-link" to={`/admin?user=${encodeURIComponent(userId)}`}><ShieldCheck size={13} />{t('adminManageRider')}</Link> : null
}
export function UserSafety({ userId, name, scope }: { userId: string; name: string; scope: AvatarScope }) {
  const { user } = useSession()
  const avatar = useAvatarMetadata(userId, scope)
  if (!user || user.id === userId) return null
  return <div className="user-safety"><ReportButton targetType="user" targetId={userId} label="reportRider" />{avatar.ready && !avatar.query.isError && avatar.query.data?.image && <ReportButton targetType="avatar" targetId={avatar.query.data.image.id} label="reportAvatar" />}<BlockButton userId={userId} name={name} /><ManageUserLink userId={userId} /></div>
}
export function BlockedUsersPanel() {
  const { t } = useI18n()
  const blocks = useBlocks()
  const [page, setPage] = useState(0)
  const entries = blocks.data?.slice(page * 50, (page + 1) * 50) ?? []
  return <section className="surface settings-panel"><h2>{t('blockedRiders')}</h2>{blocks.isPending ? <Loading /> : blocks.isError ? <ErrorNotice error={blocks.error} retry={() => void blocks.refetch()} /> : !blocks.data.length ? <p className="small-empty">{t('noBlockedRiders')}</p> : <><div className="blocked-users-list">{entries.map((person) => <div key={person.user_id}><strong>{person.display_name}</strong><BlockButton userId={person.user_id} name={person.display_name} /></div>)}</div><Pagination page={page} hasNext={(page + 1) * 50 < blocks.data.length} previous={() => setPage(Math.max(0, page - 1))} next={() => setPage(page + 1)} /></>}</section>
}
export function CommunityBadge({ userId }: { userId: string }) {
  const { t } = useI18n()
  const query = useQuery({ queryKey: ['badge', userId], queryFn: ({ signal }) => request<{ badge: { label: string; assigned_at: string; disclaimer: string } | null }>(`/users/${encodeURIComponent(userId)}/badge`, { signal }), enabled: !!userId, meta: { private: false }, retry: false, staleTime: 60_000 })
  if (query.isError || query.data?.badge?.label !== 'Community verified') return null
  return <span className="community-badge"><span><ShieldCheck size={13} />{t('communityVerified')}</span><small>{t('badgeDisclaimer')}</small></span>
}
