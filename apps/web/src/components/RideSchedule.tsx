import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { BellIcon as Bell, CalendarPlusIcon as CalendarPlus, CheckIcon as Check } from '@phosphor-icons/react'
import { Link } from 'react-router-dom'
import { ApiError, request, requestCalendar } from '../api'
import { useI18n } from '../i18n'
import { useSession } from '../session'
import { useSessionTask } from '../useSessionTask'
import { ErrorNotice, Loading, useOnline } from '../ui'
import type { Ride, RideReminders } from '../types'

export function safeRideReminders(value: unknown): RideReminders {
  if (!value || typeof value !== 'object' || !('day_before' in value) || typeof value.day_before !== 'boolean' || !('hour_before' in value) || typeof value.hour_before !== 'boolean') throw new ApiError(502, 'request_failed')
  return { day_before: value.day_before, hour_before: value.hour_before }
}

export function RideSchedule({ ride, onAccessLost }: { ride: Ride; onAccessLost: () => void }) {
  const { user } = useSession()
  const { t } = useI18n()
  const start = useSessionTask()
  const online = useOnline()
  const [downloading, setDownloading] = useState(false)
  const [error, setError] = useState<unknown>()
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => { const timer = window.setInterval(() => setNow(Date.now()), 30_000); return () => window.clearInterval(timer) }, [])
  const upcoming = ride.status === 'published' && !!ride.starts_at && new Date(ride.starts_at).getTime() > now
  const query = useQuery({ queryKey: ['ride-reminders', ride.id, user?.id], queryFn: async ({ signal }) => safeRideReminders(await request(`/rides/${encodeURIComponent(ride.id)}/reminders`, { signal })), enabled: !!user && upcoming, meta: { private: true }, retry: false })
  useEffect(() => { if (query.error instanceof ApiError && [401, 403, 404].includes(query.error.status)) onAccessLost() }, [query.error, onAccessLost])
  async function download() {
    const task = start()
    if (!task.current()) { task.finish(); return }
    setDownloading(true); setError(undefined)
    try {
      const calendar = await requestCalendar(ride.id, task.signal)
      if (!task.current()) return
      const url = URL.createObjectURL(calendar)
      const link = document.createElement('a')
      link.href = url; link.download = `ride-${ride.id}.ics`
      document.body.append(link)
      try { link.click() } finally { link.remove(); window.setTimeout(() => URL.revokeObjectURL(url), 0) }
    } catch (value) {
      if (!task.current()) return
      setError(value)
      if (value instanceof ApiError && [401, 403, 404].includes(value.status)) onAccessLost()
    } finally { if (task.current()) setDownloading(false); task.finish() }
  }
  return <section className="surface ride-schedule"><div className="panel-title"><span className="panel-icon"><Bell size={20} /></span><h2>{t('rideScheduleTitle')}</h2></div><p className="section-intro">{t('rideRemindersHint')} <Link to="/settings" className="inline-link">{t('settings')}</Link></p>{upcoming ? query.isPending ? <Loading /> : query.isError ? <ErrorNotice error={query.error} retry={() => void query.refetch()} /> : <ReminderForm key={`${ride.id}:${user?.id}`} rideID={ride.id} preferences={query.data} onAccessLost={onAccessLost} /> : <p className="section-intro">{t('reminderUnavailable')}</p>}<div className="calendar-download"><button type="button" className="button button-outline" disabled={downloading || !online} onClick={() => void download()}><CalendarPlus size={17} />{t(downloading ? 'calendarDownloading' : 'calendarDownload')}</button><p className="section-intro">{t('calendarSnapshotHint')}</p></div>{error != null && <ErrorNotice error={error} />}</section>
}

function ReminderForm({ rideID, preferences, onAccessLost }: { rideID: string; preferences: RideReminders; onAccessLost: () => void }) {
  const { user } = useSession()
  const { t } = useI18n()
  const client = useQueryClient()
  const start = useSessionTask()
  const online = useOnline()
  const [draft, setDraft] = useState<RideReminders | null>(null)
  const form = draft ?? preferences
  const [busy, setBusy] = useState(false)
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState<unknown>()
  async function submit(event: FormEvent) {
    event.preventDefault()
    const task = start()
    if (!task.current()) { task.finish(); return }
    setBusy(true); setSaved(false); setError(undefined)
    try {
      await client.cancelQueries({ queryKey: ['ride-reminders', rideID, user?.id] })
      if (!task.current()) return
      const next = safeRideReminders(await request(`/rides/${encodeURIComponent(rideID)}/reminders`, { method: 'PUT', signal: task.signal, body: JSON.stringify(form) }))
      if (!task.current()) return
      client.setQueryData(['ride-reminders', rideID, user?.id], next)
      setDraft(null); setSaved(true)
    } catch (value) {
      if (!task.current()) return
      setError(value)
      if (value instanceof ApiError && [401, 403, 404].includes(value.status)) onAccessLost()
    } finally { if (task.current()) setBusy(false); task.finish() }
  }
  return <form onSubmit={(event) => void submit(event)}><fieldset disabled={busy || !online} className="notification-options"><legend className="sr-only">{t('rideScheduleTitle')}</legend><label className="notification-option"><input type="checkbox" checked={form.day_before} onChange={(event) => { setSaved(false); setDraft({ ...form, day_before: event.target.checked }) }} /><span>{t('reminderDayBefore')}</span></label><label className="notification-option"><input type="checkbox" checked={form.hour_before} onChange={(event) => { setSaved(false); setDraft({ ...form, hour_before: event.target.checked }) }} /><span>{t('reminderHourBefore')}</span></label></fieldset>{error != null && <ErrorNotice error={error} />}{saved && <p className="success-message" role="status"><Check size={17} />{t('saved')}</p>}<button className="button button-outline" type="submit" disabled={busy || !online}>{t(busy ? 'saving' : 'reminderSave')}</button></form>
}
