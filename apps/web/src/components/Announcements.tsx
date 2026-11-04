import { useEffect, useState } from 'react'
import type { Dispatch, FormEvent, SetStateAction } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Megaphone, Send } from 'lucide-react'
import { ApiError, request } from '../api'
import { useI18n } from '../i18n'
import { useSession } from '../session'
import { useSessionTask } from '../useSessionTask'
import { ErrorNotice, Loading, Pagination, formatRideDateTime, useOnline } from '../ui'
import type { Announcement, Ride } from '../types'
import { Avatar } from './Avatar'
import { liveRideAccess } from './RideMedia'
import { CommunityBadge, ReportButton, UserSafety } from './Trust'

export interface AnnouncementDraft { actor: string; rideId: string; body: string; pending?: { body: string; client_announcement_id: string } }
interface AnnouncementPage { announcements: Announcement[]; next_cursor: string }
export function Announcements({ ride, draft, setDraft, onAccessLost }: { ride: Ride; draft: AnnouncementDraft; setDraft: Dispatch<SetStateAction<AnnouncementDraft>>; onAccessLost: () => void }) {
  const { user } = useSession()
  const { t, locale } = useI18n()
  const client = useQueryClient()
  const start = useSessionTask()
  const online = useOnline()
  const [cursors, setCursors] = useState([''])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<unknown>()
  const page = cursors.length - 1
  const query = useQuery({ queryKey: ['announcements', ride.id, user?.id, cursors[page]], meta: { private: true }, retry: false, refetchInterval: 15_000, queryFn: async ({ signal }) => {
    const value = await request<AnnouncementPage>(`/rides/${ride.id}/announcements${cursors[page] ? `?before=${encodeURIComponent(cursors[page])}` : ''}`, { signal })
    setDraft((current) => current.actor === user?.id && current.rideId === ride.id && current.pending && value.announcements.some((item) => item.owner_id === user.id && item.client_announcement_id === current.pending?.client_announcement_id) ? { ...current, body: current.body.trim() === current.pending.body ? '' : current.body, pending: undefined } : current)
    return { announcements: value.announcements.map(({ id, sequence, owner_id, owner_name, body, created_at, client_announcement_id }) => ({ id, sequence, owner_id, owner_name, body, created_at, client_announcement_id })), next_cursor: value.next_cursor }
  } })
  useEffect(() => { if (query.error instanceof ApiError && [401, 403, 404].includes(query.error.status)) onAccessLost() }, [query.error, onAccessLost])
  async function send(event: FormEvent) {
    event.preventDefault()
    const task = start()
    const current = () => task.current() && liveRideAccess(client, ride, user?.id, true) && client.getQueryData<{ ride: Ride }>(['ride', ride.id, user?.id])?.ride.can_manage === true
    const body = draft.body.trim()
    if (!current() || !body || busy || new TextEncoder().encode(body).length > 4000) { if (body && new TextEncoder().encode(body).length > 4000) setError(new ApiError(422, 'invalid_announcement')); task.finish(); return }
    const command = draft.pending?.body === body ? draft.pending : { body, client_announcement_id: crypto.randomUUID() }
    setDraft((value) => ({ ...value, pending: command })); setBusy(true); setError(undefined)
    try {
      await request(`/rides/${ride.id}/announcements`, { method: 'POST', signal: task.signal, body: JSON.stringify(command) })
      if (!current()) return
      setDraft((value) => value.actor === user?.id && value.rideId === ride.id && value.pending?.client_announcement_id === command.client_announcement_id ? { ...value, body: value.body.trim() === command.body ? '' : value.body, pending: undefined } : value)
      setCursors(['']); await client.invalidateQueries({ queryKey: ['announcements', ride.id] })
    } catch (value) { if (task.current()) { setError(value); if (value instanceof ApiError && [401, 403, 404].includes(value.status)) onAccessLost() } }
    finally { if (task.current()) setBusy(false); task.finish() }
  }
  return <section className="surface announcements-panel"><div className="panel-title"><span className="panel-icon"><Megaphone size={22} /></span><div><h2>{t('announcements')}</h2><p>{t('announcementText')}</p></div></div>{query.isPending ? <Loading /> : query.isError ? <ErrorNotice error={query.error} retry={() => void query.refetch()} /> : !query.data.announcements.length ? <p className="small-empty">{t('announcementEmpty')}</p> : <div className="announcement-list">{query.data.announcements.map((item) => <article key={item.id} className="announcement"><div className="announcement-author"><Avatar userId={item.owner_id} name={item.owner_name} scope={{ kind: 'ride', id: ride.id }} /><div><strong>{item.owner_name}</strong><CommunityBadge userId={item.owner_id} /><time dateTime={item.created_at}>{formatRideDateTime(item.created_at, ride.timezone ?? 'Asia/Almaty', locale, true)}</time></div></div><p>{item.body}</p><ReportButton targetType="announcement" targetId={item.id} /><UserSafety userId={item.owner_id} name={item.owner_name} scope={{ kind: 'ride', id: ride.id }} /></article>)}</div>}<Pagination page={page} hasNext={!!query.data?.next_cursor} busy={query.isFetching} previous={() => setCursors((values) => values.slice(0, -1))} next={() => { if (query.data?.next_cursor) setCursors((values) => [...values, query.data.next_cursor]) }} />{ride.can_manage && ride.status !== 'cancelled' && <form className="announcement-composer" onSubmit={(event) => void send(event)}><label className="field"><span className="field-label">{t('announcementBody')}</span><textarea value={draft.body} maxLength={4000} rows={3} disabled={busy || !online} onChange={(event) => setDraft((value) => ({ ...value, body: event.target.value }))} /></label>{error != null && <ErrorNotice error={error} />}<div className="form-actions"><button className="button button-primary" disabled={busy || !online || !draft.body.trim()} type="submit"><Send size={15} />{t(busy ? 'sending' : 'publishAnnouncement')}</button></div></form>}</section>
}
