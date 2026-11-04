import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft, ArrowUpRight, Bell, Check } from 'lucide-react'
import { Link } from 'react-router-dom'
import { request } from '../api'
import { useI18n } from '../i18n'
import { useSession } from '../session'
import { useSessionTask } from '../useSessionTask'
import { ErrorNotice, Loading, Pagination, formatRideDateTime, useOnline } from '../ui'
import type { NotificationPage } from '../types'

const noticeKeys = { 'ride.updated': 'notice_ride_updated', 'ride.cancelled': 'notice_ride_cancelled', 'ride.announcement': 'notice_ride_announcement', 'participation.requested': 'notice_participation_requested', 'participation.approved': 'notice_participation_approved', 'participation.rejected': 'notice_participation_rejected', 'participation.removed': 'notice_participation_removed' } as const
export function InboxPage() {
  const { user } = useSession()
  const { t, locale } = useI18n()
  const client = useQueryClient()
  const start = useSessionTask()
  const online = useOnline()
  const [cursors, setCursors] = useState([''])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<unknown>()
  const page = cursors.length - 1
  const query = useQuery({ queryKey: ['inbox', user?.id, cursors[page]], queryFn: ({ signal }) => request<NotificationPage>(`/me/notifications${cursors[page] ? `?before=${encodeURIComponent(cursors[page])}` : ''}`, { signal }), meta: { private: true }, retry: false, refetchInterval: 30_000 })
  async function read(id?: string) {
    const task = start(); if (!task.current()) { task.finish(); return }
    setBusy(true); setError(undefined)
    try { await request(`/me/notifications/${id ? `${encodeURIComponent(id)}/` : ''}read`, { method: 'PUT', signal: task.signal }); if (task.current()) await client.invalidateQueries({ queryKey: ['inbox', user?.id] }) }
    catch (value) { if (task.current()) setError(value) }
    finally { if (task.current()) setBusy(false); task.finish() }
  }
  return <div className="page-content narrow-page"><Link className="back-link" to="/"><ArrowLeft size={17} />{t('back')}</Link><div className="page-heading"><span className="eyebrow">{t('inbox')}</span><h1>{t('inboxTitle')}</h1><p>{t('inboxText')}</p></div>{error != null && <ErrorNotice error={error} />}{query.isPending ? <Loading /> : query.isError ? <ErrorNotice error={query.error} retry={() => void query.refetch()} /> : <><div className="section-header"><span>{t('unreadUpdates', { count: query.data.unread_count })}</span><button className="text-button" disabled={busy || !online || !query.data.unread_count} onClick={() => void read()}><Check size={16} />{t('markAllRead')}</button></div><div className="inbox-list">{query.data.notifications.length ? query.data.notifications.map((item) => <article key={item.id} className={`surface inbox-item ${item.read_at ? '' : 'unread'}`}><span className="panel-icon"><Bell size={20} /></span><div><h2>{t(noticeKeys[item.kind as keyof typeof noticeKeys] ?? 'inbox')}</h2><time dateTime={item.created_at}>{formatRideDateTime(item.created_at, 'Asia/Almaty', locale)}</time><div className="inbox-actions"><Link className="inline-link" to={`/rides/${encodeURIComponent(item.ride_id)}`}>{t('openRide')}<ArrowUpRight size={14} /></Link>{!item.read_at && <button className="text-button" disabled={busy || !online} onClick={() => void read(item.id)}>{t('markRead')}</button>}</div></div></article>) : <p className="surface empty-panel">{t('inboxEmpty')}</p>}</div><Pagination page={page} busy={query.isFetching} hasNext={!!query.data.next_cursor} previous={() => setCursors((values) => values.slice(0, -1))} next={() => { if (query.data.next_cursor) setCursors((values) => [...values, query.data.next_cursor]) }} /></>}</div>
}
