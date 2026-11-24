import { useState } from 'react'
import { DownloadSimpleIcon } from '@phosphor-icons/react'
import { useQueryClient } from '@tanstack/react-query'
import type { UseQueryResult } from '@tanstack/react-query'
import { useI18n } from '../i18n'
import type { AnnouncementPage } from '../ride-announcements'
import { rideBriefHTML, rideBriefStops } from '../ride-brief'
import { useSession } from '../session'
import { useSessionTask } from '../useSessionTask'
import { ErrorNotice, useOnline } from '../ui'
import type { Ride } from '../types'
import { RideAnnouncement } from './Announcements'
import { liveRideAccess } from './RideMedia'

export function RideDayBrief({ ride, announcements, refreshRide }: { ride: Ride; announcements: UseQueryResult<AnnouncementPage>; refreshRide: () => Promise<Ride | undefined> }) {
  const { t, locale } = useI18n()
  const { user } = useSession()
  const start = useSessionTask()
  const client = useQueryClient()
  const online = useOnline()
  const [counts, setCounts] = useState<Record<number, string>>({})
  const [countsOpen, setCountsOpen] = useState(false)
  const [download, setDownload] = useState<'idle' | 'checking' | 'started' | 'failed'>('idle')
  const stops = rideBriefStops(ride)
  const timeZone = ride.timezone ?? 'Asia/Almaty'
  const latest = announcements.data?.announcements[0]
  const canSave = online && announcements.isSuccess && download !== 'checking'

  async function save() {
    if (!canSave) return
    const task = start()
    if (!task.current()) { task.finish(); return }
    setDownload('checking')
    const active = () => liveRideAccess(client, ride, user?.id, true)
    try {
      const refreshed = await refreshRide()
      if (!task.current()) return
      if (!refreshed || !active()) throw new Error('Ride access or plan changed')
      const refreshedUpdates = await announcements.refetch({ cancelRefetch: false })
      if (!task.current()) return
      if (!refreshedUpdates.isSuccess || !active()) throw new Error('Ride announcements could not be confirmed')
      const update = refreshedUpdates.data.announcements[0]
      const now = new Date()
      const at = now.toISOString()
      const format = new Intl.DateTimeFormat(locale, { dateStyle: 'medium', timeStyle: 'short', timeZone, hourCycle: 'h23' })
      const time = (value: string) => format.format(new Date(value))
      const html = rideBriefHTML({
        locale,
        heading: t('rideBriefTitle'),
        title: ride.title ?? t('rideIn', { city: ride.city }),
        departure: { value: ride.starts_at ?? ride.date, label: t('departure'), text: ride.starts_at ? `${time(ride.starts_at)} (${timeZone})` : ride.date },
        savedAt: { value: at, label: t('rideBriefSavedAt'), text: `${format.format(now)} (${timeZone})` },
        routeStops: t('rideBriefStops'),
        navigateToMeeting: t('rideNavigateMeeting'),
        stops: stops.map((stop, index) => ({ ...stop, count: counts[index] ? t('rideBriefRecordedCount', { count: Number(counts[index]) }) : undefined })),
        latestHeading: t('rideLatestUpdate'),
        latest: update ? { body: update.body, at: update.created_at, when: time(update.created_at) } : undefined,
        noLatest: t('announcementEmpty'),
        notices: [t('rideBriefPrivateCopy'), t('rideBriefStaleCopy'), t('rideBriefDirectionsOnline')],
      })
      const url = URL.createObjectURL(new Blob([html], { type: 'text/html;charset=utf-8' }))
      const link = document.createElement('a')
      link.href = url
      link.download = `bikers-kz-ride-brief-${ride.date}-${at.slice(0, 10)}.html`
      document.body.append(link)
      try { link.click() } finally { link.remove(); window.setTimeout(() => URL.revokeObjectURL(url), 0) }
      setDownload('started')
    } catch {
      if (task.current()) setDownload('failed')
    } finally {
      task.finish()
    }
  }

  return <div className="ride-brief">
    <div className="ride-brief-update">
      {latest && !announcements.isError ? <RideAnnouncement ride={ride} item={latest} latest /> : <>
        <h3>{t('rideLatestUpdate')}</h3>
        {announcements.isError ? <ErrorNotice error={announcements.error} retry={() => void announcements.refetch()} /> : <p className="field-hint">{t(announcements.isPending ? 'rideBriefLoadingUpdate' : 'announcementEmpty')}</p>}
      </>}
    </div>
    {stops.length > 0 && <details className="ride-brief-counts" onToggle={(event) => setCountsOpen(event.currentTarget.open)}>
      <summary>{t('rideBriefHeadCounts')}</summary>
      {countsOpen && <><p className="field-hint">{t('rideBriefCountsNote')}</p><ol>
        {stops.map((stop, index) => <li key={index}>
          <label><span>{stop.name}</span>
            <input type="number" min={0} max={1000} step={1} inputMode="numeric" aria-label={t('rideBriefCountAtStop', { stop: stop.name })} value={counts[index] ?? ''} onChange={(event) => {
              const value = event.currentTarget.value
              if (/^\d{0,4}$/.test(value) && Number(value) <= 1000) setCounts((previous) => ({ ...previous, [index]: value }))
            }} />
          </label>
        </li>)}
      </ol></>}
    </details>}
    <div className="ride-brief-download">
      <button className="button button-outline" type="button" disabled={!canSave} onClick={() => void save()} aria-describedby="ride-brief-copy-note"><DownloadSimpleIcon size={18} />{t(download === 'checking' ? 'rideBriefChecking' : 'rideBriefSave')}</button>
      <p className="field-hint" id="ride-brief-copy-note">{t('rideBriefPrivateCopy')} {t('rideBriefStaleCopy')}</p>
      {!online && <p className="field-hint">{t('rideBriefConnect')}</p>}
      {(download === 'started' || download === 'failed') && <p className="field-hint" role="status">{t(download === 'started' ? 'rideBriefDownloadStarted' : 'rideBriefDownloadFailed')}</p>}
    </div>
  </div>
}
