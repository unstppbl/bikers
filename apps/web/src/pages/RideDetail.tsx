import { useCallback, useEffect, useRef, useState } from 'react'
import type { Dispatch, FormEvent, SetStateAction } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowLeftIcon as ArrowLeft, ArrowRightIcon as ArrowRight, ArrowUpRightIcon as ArrowUpRight, CalendarDotsIcon as CalendarDays, CaretRightIcon as ChevronRight, CheckIcon as Check, ClockIcon as Clock3, FlagIcon as Flag, DoorOpenIcon as DoorOpen, LockKeyIcon as LockKeyhole, MapPinIcon as MapPin, ChatCircleIcon as MessageCircle, NavigationArrowIcon as Navigation, PencilSimpleIcon as Pencil, PaperPlaneTiltIcon as Send, ShieldCheckIcon as ShieldCheck, UsersThreeIcon as Users, XIcon as X } from '@phosphor-icons/react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { formatInTimeZone } from 'date-fns-tz'
import { ApiError, mutation, publicRide, request, safeExternalUrl } from '../api'
import { useI18n } from '../i18n'
import { cityLabel } from '../cities'
import { useCityOptions } from '../components/CitySelector'
import { CityArt, DatePlate, goingLabel } from '../components/RideVisual'
import { RideCoverReport, RideImage } from '../components/RideCover'
import { useSession, useSessionGuard } from '../session'
import { ErrorNotice, Loading, PrivacyNote, dateParts, formatRideDateTime, useConfirmation, useOnline } from '../ui'
import { ParticipantsPanel } from '../components/ParticipantsPanel'
import { Avatar } from '../components/Avatar'
import { Announcements } from '../components/Announcements'
import type { AnnouncementDraft } from '../components/Announcements'
import { RideMedia } from '../components/RideMedia'
import { RideSchedule } from '../components/RideSchedule'
import { RideShare } from '../components/RideShare'
import { SavedRideMap } from '../components/maps/MapPlanEditor'
import { MessageActions } from '../components/MessageActions'
import { CommunityBadge, ReportButton } from '../components/Trust'
import { resetTrustQueries } from '../trust-cache'
import type { Message, MessagesResponse, Ride } from '../types'

interface RoomDraft {
  actor: string
  rideId: string
  body: string
  pending?: { body: string; client_message_id: string }
}

export function RideDetailPage() {
  const { rideId = '' } = useParams()
  const { user, loading, refresh, error: sessionError } = useSession()
  const captureSession = useSessionGuard()
  const { t, locale } = useI18n()
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const online = useOnline()
  const [working, setWorking] = useState(false)
  const [actionError, setActionError] = useState<unknown>()
  const { confirm, dialog } = useConfirmation()
  const [announcementDraft, setAnnouncementDraft] = useState<AnnouncementDraft>({ actor: user?.id ?? '', rideId, body: '' })
  const reconnectTimer = useRef<number | undefined>(undefined)
  const reconnectDelay = useRef(1000)
  const [roomDraft, setRoomDraft] = useState<RoomDraft>({ actor: user?.id ?? '', rideId, body: '' })
  const [rechecking, setRechecking] = useState(false)
  const cities = useCityOptions()
  if (roomDraft.actor !== (user?.id ?? '') || roomDraft.rideId !== rideId) {
    setAnnouncementDraft({ actor: user?.id ?? '', rideId, body: '' })
    setRoomDraft({ actor: user?.id ?? '', rideId, body: '' })
    setRechecking(false)
  }
  const key = ['ride', rideId, user?.id ?? 'guest']
  const rideQuery = useQuery({
    queryKey: key,
    queryFn: async ({ signal }) => {
      try {
        const value = await request<{ ride: Ride }>(`/rides/${rideId}`, { signal })
        if (!(value.ride.title && (value.ride.can_manage || value.ride.participation_state === 'approved'))) {
          setRoomDraft((draft) => draft.actor === (user?.id ?? '') ? { actor: user?.id ?? '', rideId, body: '' } : draft)
        }
        setRechecking(false)
        return user ? value : { ride: publicRide(value.ride) }
      } catch (error) {
        if (error instanceof ApiError && [401, 403].includes(error.status)) setRoomDraft((draft) => draft.actor === (user?.id ?? '') ? { actor: user?.id ?? '', rideId, body: '' } : draft)
        throw error
      }
    },
    enabled: !loading && !sessionError,
    meta: { private: true },
    refetchInterval: (query) => query.state.data?.ride.participation_state === 'requested' ? 10_000 : false,
  })
  const ride = rideQuery.data?.ride
  const canAccess = !!user && !!ride?.title && !!(ride.can_manage || ride.participation_state === 'approved') && !rideQuery.isError && !rechecking

  const recheckAccess = useCallback(() => {
    setRechecking(true)
    const actorKey = user?.id ?? 'guest'
    void queryClient.cancelQueries({ queryKey: ['ride', rideId, actorKey] })
    queryClient.setQueryData<{ ride: Ride }>(['ride', rideId, actorKey], (value) => value ? { ride: publicRide(value.ride) } : undefined)
    for (const name of ['messages', 'ride-album', 'announcements', 'ride-reminders']) { void queryClient.cancelQueries({ queryKey: [name, rideId] }); queryClient.removeQueries({ queryKey: [name, rideId] }) }
    queryClient.removeQueries({ queryKey: ['avatar'] })
    queryClient.removeQueries({ queryKey: ['participants', rideId] })
    if (reconnectTimer.current) window.clearTimeout(reconnectTimer.current)
    reconnectTimer.current = window.setTimeout(() => {
      void refresh().then(() => queryClient.invalidateQueries({ queryKey: ['ride', rideId] }))
    }, reconnectDelay.current)
    reconnectDelay.current = Math.min(30_000, reconnectDelay.current * 2)
  }, [queryClient, refresh, rideId, user?.id])
  const markConnected = useCallback(() => { reconnectDelay.current = 1000 }, [])
  useEffect(() => () => { if (reconnectTimer.current) window.clearTimeout(reconnectTimer.current) }, [rideId])

  async function participate(leave = false) {
    const currentSession = captureSession()
    if (!user) { void navigate(`/login?next=${encodeURIComponent(`/rides/${rideId}`)}`); return }
    if (!user.onboarding_complete) { void navigate(`/profile?next=${encodeURIComponent(`/rides/${rideId}`)}`); return }
    if (leave && !await confirm({ message: t('leaveConfirm'), action: t('leaveRide') })) return
    setWorking(true); setActionError(undefined)
    try {
      await mutation(`/rides/${rideId}/participation`, leave ? 'DELETE' : 'PUT')
      if (!currentSession()) return
      if (leave) recheckAccess()
      await queryClient.invalidateQueries({ queryKey: ['ride', rideId] })
      await queryClient.invalidateQueries({ queryKey: ['rides'] })
    } catch (error) { setActionError(error) }
    finally { setWorking(false) }
  }

  async function cancelRide() {
    const currentSession = captureSession()
    if (!ride || !await confirm({ message: t('cancelConfirm'), action: t('cancelRide'), destructive: true })) return
    setWorking(true); setActionError(undefined)
    try {
      await mutation(`/rides/${rideId}/cancel`, 'POST', { plan_version: ride.plan_version })
      if (!currentSession()) return
      await queryClient.invalidateQueries({ queryKey: ['ride', rideId] })
      await queryClient.invalidateQueries({ queryKey: ['rides'] })
    } catch (error) { setActionError(error) }
    finally { setWorking(false) }
  }

  if (sessionError) return <div className="page-content"><ErrorNotice text={t('serviceUnavailable')} retry={() => void refresh()} /></div>
  if (loading || rideQuery.isPending) return <Loading />
  if (!ride) return <div className="page-content"><Link className="back-link" to="/"><ArrowLeft size={17} />{t('back')}</Link><ErrorNotice error={rideQuery.error} retry={() => void rideQuery.refetch()} /></div>
  const date = dateParts(ride.date, locale)
  const status = ride.participation_state
  const city = cityLabel(ride.city, locale, cities.data?.cities ?? [])
  const meetingPin = ride.map_plan?.points[0]
  return <div className="page-content ride-detail">{dialog}<Link className="back-link" to="/"><ArrowLeft size={17} />{t('back')}</Link>
    {rideQuery.isError && <ErrorNotice error={rideQuery.error} retry={() => void rideQuery.refetch()} />}
    <section className={`ride-hero${ride.status === 'cancelled' ? ' is-cancelled' : ''}${canAccess ? ' is-ride-room' : ''}`} aria-labelledby="ride-title">
      <RideImage ride={ride} className="ride-hero-cover" described priority />
      <CityArt city={city} />
      <div className="ride-hero-body">
        <DatePlate date={ride.date} locale={locale} size="hero" />
        <div className="ride-hero-copy">
          <div className="ride-hero-status">{ride.status === 'cancelled' && <span className="status-pill cancelled">{t('cancelled')}</span>}{ride.can_manage && <span className="status-pill">{t('organizer')}</span>}{canAccess && !ride.can_manage && <span className="status-pill approved"><Check size={13} weight="bold" />{t(goingLabel(ride))}</span>}{status === 'requested' && <span className="status-pill pending"><Clock3 size={13} />{t('requested')}</span>}</div>
          <h1 id="ride-title">{canAccess ? ride.title : t('rideIn', { city })}</h1>
          <div className="detail-meta"><span><MapPin size={17} />{city}</span><span><CalendarDays size={17} />{date.full}</span>{canAccess && ride.starts_at && <span><Clock3 size={17} />{formatInTimeZone(ride.starts_at, ride.timezone ?? 'Asia/Almaty', 'HH:mm')}</span>}</div>
          {canAccess && ride.status !== 'cancelled' && <a className="button button-primary ride-meeting-action" href={directionsTo(meetingPin ? `${meetingPin.lat},${meetingPin.lon}` : ride.meeting_point ?? '')} target="_blank" rel="noopener noreferrer">{t('rideNavigateMeeting')}<ArrowUpRight size={16} /></a>}
        </div>
      </div>
    </section>
    <div className="ride-toolbar"><RideShare key={ride.id} rideId={ride.id} />{canAccess && ride.can_manage && ride.status !== 'cancelled' && <Link className="button button-outline" to={`/rides/${ride.id}/edit`}><Pencil size={16} />{t('editRide')}</Link>}<div className="ride-toolbar-reports"><ReportButton targetType="ride" targetId={ride.id} /><RideCoverReport ride={ride} /></div></div>
    {ride.status === 'cancelled' && <div className="cancelled-banner" role="status"><X size={19} /><span>{t('cancelledText')}</span></div>}
    {actionError != null && <ErrorNotice error={actionError} />}
    {canAccess ? <div className="ride-layout">
      <div className="ride-main">
        <RidePlan ride={ride} />
        <Announcements key={`announcements:${ride.id}:${user?.id}`} ride={ride} draft={announcementDraft} setDraft={setAnnouncementDraft} onAccessLost={recheckAccess} />
        <p className="ride-description">{ride.description}</p>
        {ride.map_plan && <SavedRideMap key={`map:${ride.id}:${user?.id}:${ride.plan_version}`} plan={ride.map_plan} />}
        <RideRoom key={`room:${ride.id}:${user?.id}`} ride={ride} draft={roomDraft} setDraft={setRoomDraft} onAccessLost={recheckAccess} onConnected={markConnected} />
        <RideMedia key={`media:${ride.id}:${user?.id}`} ride={ride} onAccessLost={recheckAccess} />
      </div>
      <aside className="ride-aside">
        <ParticipantsPanel ride={ride} onAccessLost={recheckAccess} />
        <RideSchedule key={`schedule:${ride.id}:${user?.id}`} ride={ride} onAccessLost={recheckAccess} />
        <div className="ride-lower-actions">{ride.can_manage ? ride.status !== 'cancelled' && <button className="text-button danger-text" disabled={working || !online} onClick={() => void cancelRide()}>{t('cancelRide')}</button> : <button className="text-button" disabled={working || !online} onClick={() => void participate(true)}><DoorOpen size={17} />{t('leaveRide')}</button>}</div>
      </aside>
    </div> : <div className="ride-layout ride-preview">
      <div className="ride-main">
        <p className="ride-description ride-lead">{ride.description}</p>
        <section className="surface access-card"><div className="access-icon">{status === 'requested' ? <Clock3 size={28} /> : <LockKeyhole size={28} />}</div><div className="access-copy"><h2>{status === 'requested' ? t('pendingTitle') : status === 'rejected' ? t('rejectedTitle') : status === 'removed' ? t('removedTitle') : t('exactDetails')}</h2><p>{status === 'requested' ? t('pendingText') : t('previewText')}</p></div>{ride.status !== 'cancelled' && (status === 'requested' ? <button className="button button-outline" disabled={working || !online} onClick={() => void participate(true)}>{t('leaveRide')}</button> : <button className="button button-primary access-join" disabled={working || !online} onClick={() => void participate()}>{user ? t(status === 'rejected' || status === 'left' ? 'requestAgain' : 'requestJoin') : t('signInJoin')}<ArrowRight size={18} /></button>)}<span className="access-footnote"><ShieldCheck size={15} />{t('organizerApproval')}</span></section>
        <LockedPlan />
      </div>
      <aside className="ride-aside"><PrivacyNote /></aside>
    </div>}
  </div>
}

function directionsTo(destination: string) {
  return `https://www.google.com/maps/dir/?api=1&destination=${encodeURIComponent(destination)}`
}

function RidePlan({ ride }: { ride: Ride }) {
  const { t, locale } = useI18n()
  const timezone = ride.timezone ?? 'Asia/Almaty'
  const route = safeExternalUrl(ride.route_url)
  const last = ride.map_plan && ride.map_plan.points.length > 1 ? ride.map_plan.points.at(-1) : undefined
  const stops = ride.map_plan?.points.slice(1, -1) ?? []
  return <section className="surface plan-card" aria-labelledby="plan-heading">
    <h2 id="plan-heading">{t('planTitle')}</h2>
    <ol className="roadbook">
      {ride.starts_at && <li><span className="roadbook-marker"><Clock3 size={19} /></span><div className="roadbook-copy"><span className="roadbook-label">{t('departure')}</span><strong className="roadbook-time">{formatInTimeZone(ride.starts_at, timezone, 'HH:mm')}</strong><span className="roadbook-sub">{formatRideDateTime(ride.starts_at, timezone, locale)}, {timezone}</span></div></li>}
      <li><span className="roadbook-marker"><Flag size={19} /></span><div className="roadbook-copy"><span className="roadbook-label">{t('meetingPoint')}</span><p>{ride.meeting_point}</p></div></li>
      {route && <li><span className="roadbook-marker"><Navigation size={19} /></span><div className="roadbook-copy"><span className="roadbook-label">{t('planRoute')}</span><a className="inline-link" href={route} target="_blank" rel="noopener noreferrer">{t('routeLink')}<ArrowUpRight size={14} /></a></div></li>}
      {stops.length > 0 && <li><span className="roadbook-marker"><MapPin size={19} /></span><details className="roadbook-stops"><summary>{t('rideRouteStops', { count: stops.length })}</summary><ol>{stops.map((stop, index) => <li key={index}>{stop.label}</li>)}</ol></details></li>}
      {ride.destination && <li><span className="roadbook-marker"><MapPin size={19} /></span><div className="roadbook-copy"><span className="roadbook-label">{t('destination')}</span><p>{ride.destination}</p><a className="inline-link" href={directionsTo(last ? `${last.lat},${last.lon}` : ride.destination)} target="_blank" rel="noopener noreferrer">{t('navigate')}<ArrowUpRight size={14} /></a></div></li>}
    </ol>
    <dl className="plan-facts"><div><dt><Users size={17} />{t('capacity')}</dt><dd>{ride.capacity}<small>{t('capacityHint')}</small></dd></div></dl>
    {ride.expectations && <div className="expectations"><h3>{t('expectations')}</h3><p>{ride.expectations}</p></div>}
  </section>
}

function LockedPlan() {
  const { t } = useI18n()
  const stops = [['meetingPoint', <Flag key="flag" size={19} />], ['departure', <Clock3 key="clock" size={19} />], ['destination', <MapPin key="pin" size={19} />]] as const
  return <section className="surface plan-card plan-locked" aria-labelledby="plan-heading">
    <h2 id="plan-heading">{t('planTitle')}</h2>
    <ol className="roadbook">{stops.map(([key, icon]) => <li key={key}><span className="roadbook-marker">{icon}</span><div className="roadbook-copy"><span className="roadbook-label">{t(key)}</span><span className="roadbook-locked"><LockKeyhole size={15} />{t('planLocked')}</span></div></li>)}</ol>
  </section>
}

interface RoomHistory extends MessagesResponse { more: boolean }

export function mergeMessages(previous: Message[], incoming: Message[]) {
  return [...new Map([...previous, ...incoming].map((message) => [message.sequence, message])).values()].sort((a, b) => a.sequence - b.sequence)
}

function RideRoom({ ride, draft, setDraft, onAccessLost, onConnected }: { ride: Ride; draft: RoomDraft; setDraft: Dispatch<SetStateAction<RoomDraft>>; onAccessLost: () => void; onConnected: () => void }) {
  const { user } = useSession()
  const { t, locale } = useI18n()
  const queryClient = useQueryClient()
  const online = useOnline()
  const captureSession = useSessionGuard()
  const [sending, setSending] = useState(false)
  const [sendError, setSendError] = useState<unknown>()
  const [connection, setConnection] = useState<'connecting' | 'connected'>('connecting')
  const messagesViewport = useRef<HTMLDivElement>(null)
  const followMessages = useRef(true)
  const key = ['messages', ride.id, user?.id]
  const query = useQuery({
    queryKey: key,
    queryFn: async ({ signal }) => {
      const previous = queryClient.getQueryData<RoomHistory>(['messages', ride.id, user?.id])
      let messages = previous?.messages ?? []
      let cursor = previous?.next_sequence ?? 0
      let more = false
      for (let page = 0; page < 100; page++) {
        const result = await request<MessagesResponse>(`/rides/${ride.id}/messages?after=${cursor}`, { signal })
        messages = mergeMessages(messages, result.messages)
        more = result.messages.length >= 100 && result.next_sequence > cursor
        cursor = Math.max(cursor, result.next_sequence)
        if (!more) break
      }
      setDraft((current) => {
        if (current.actor !== user?.id || current.rideId !== ride.id || !current.pending) return current
        const pending = current.pending
        if (!messages.some((message) => message.sender_id === user.id && message.client_message_id === pending.client_message_id)) return current
        return { ...current, body: current.body.trim() === pending.body ? '' : current.body, pending: undefined }
      })
      return { messages, next_sequence: cursor, more }
    },
    refetchOnWindowFocus: true,
  })
  useEffect(() => { if (query.error instanceof ApiError && [401, 403].includes(query.error.status)) onAccessLost() }, [query.error, onAccessLost])
  useEffect(() => {
    let disposed = false
    const url = new URL('/api/v1/events', window.location.origin)
    url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
    url.searchParams.set('ride_id', ride.id)
    const socket = new WebSocket(url)
    socket.onopen = () => {
      if (disposed) return
      setConnection('connected'); onConnected()
      void queryClient.invalidateQueries({ queryKey: ['messages', ride.id] })
      void queryClient.invalidateQueries({ queryKey: ['ride', ride.id] })
    }
    socket.onmessage = (event) => {
      if (disposed) return
      try {
        const hint = JSON.parse(event.data)
        if (hint.ride_id !== ride.id) return
        if (hint.type === 'trust.changed') void resetTrustQueries(queryClient)
        if (hint.type === 'announcement.created') { void queryClient.invalidateQueries({ queryKey: ['announcements', ride.id] }); void queryClient.invalidateQueries({ queryKey: ['inbox'] }) }
        if (hint.type === 'message.created') void queryClient.invalidateQueries({ queryKey: ['messages', ride.id] })
        if (hint.type === 'ride.updated' || hint.type === 'participation.changed') {
          void queryClient.invalidateQueries({ queryKey: ['ride', ride.id] })
          void queryClient.invalidateQueries({ queryKey: ['participants', ride.id] })
        }
      } catch { /* Only recognized server hints trigger reads. */ }
    }
    socket.onclose = () => { if (!disposed) onAccessLost() }
    return () => { disposed = true; socket.close() }
  }, [ride.id, user?.id, queryClient, onAccessLost, onConnected])
  const nextSequence = query.data?.next_sequence
  useEffect(() => {
    const viewport = messagesViewport.current
    if (viewport && nextSequence !== undefined && followMessages.current) {
      // scrollIntoView also moves outer ancestors, pulling the ride page down.
      // The chat owns its scrolling; history readers keep their position.
      viewport.scrollTop = viewport.scrollHeight
    }
  }, [nextSequence])

  async function send(event?: FormEvent) {
    const currentSession = captureSession()
    event?.preventDefault()
    const text = draft.body.trim()
    if (!text || sending || !online || ride.status === 'cancelled') return
    const command = draft.pending?.body === text ? draft.pending : { body: text, client_message_id: crypto.randomUUID() }
    setDraft((current) => ({ ...current, pending: command }))
    setSending(true); setSendError(undefined)
    try {
      await mutation(`/rides/${ride.id}/messages`, 'POST', command)
      if (!currentSession()) return
      setDraft((current) => current.actor === user?.id && current.rideId === ride.id && current.pending?.client_message_id === command.client_message_id ? { ...current, body: current.body.trim() === text ? '' : current.body, pending: undefined } : current)
      await queryClient.invalidateQueries({ queryKey: ['messages', ride.id] })
    } catch (error) { setSendError(error); if (error instanceof ApiError && [401, 403].includes(error.status)) onAccessLost() }
    finally { setSending(false) }
  }
  const messages = query.data?.messages ?? []
  function changeBody(value: string) { setDraft((current) => ({ ...current, body: value })) }
  return <section className="surface room-card" aria-labelledby="room-heading"><div className="room-heading"><div className="panel-title"><span className="panel-icon room-panel-icon"><MessageCircle size={22} /></span><div><h2 id="room-heading">{t('rideRoom')}</h2><p>{t('roomText')}</p></div></div><span className={`room-connection ${connection === 'connected' ? 'is-connected' : ''}`} title={t(connection === 'connected' ? 'connected' : 'connecting')}><span className="status-dot" /><span>{t(connection === 'connected' ? 'connected' : 'connecting')}</span></span></div>
    <div className="messages" ref={messagesViewport} onScroll={(event) => { const viewport = event.currentTarget; followMessages.current = viewport.scrollHeight - viewport.clientHeight - viewport.scrollTop <= 64 }} aria-live="polite" aria-relevant="additions text">{query.isPending ? <Loading /> : query.isError ? <ErrorNotice error={query.error} retry={() => void query.refetch()} /> : !messages.length ? <div className="chat-empty"><MessageCircle size={28} strokeWidth={1.4} /><h3>{t('messagesEmpty')}</h3><p>{t('messagesEmptyText')}</p></div> : messages.map((message) => <div key={message.sequence} className={`message message-with-actions ${message.sender_id === user?.id ? 'own-message' : ''}`}><Avatar userId={message.sender_id} name={message.sender_name} scope={{ kind: 'ride', id: ride.id }} /><div className="message-content"><div className="message-meta"><strong>{message.sender_name}</strong><time dateTime={message.created_at}>{formatRideDateTime(message.created_at, ride.timezone ?? 'Asia/Almaty', locale, true)}</time></div><p>{message.body}</p><CommunityBadge userId={message.sender_id} /></div><MessageActions message={message} rideId={ride.id} /></div>)}</div>
    {query.data?.more && <button className="text-button" onClick={() => void query.refetch()} disabled={query.isFetching}>{t('retry')}<ChevronRight size={16} /></button>}
    {sendError != null && <ErrorNotice error={sendError} />}
    {ride.status === 'cancelled' ? <div className="room-read-only"><LockKeyhole size={16} />{t('roomReadOnly')}</div> : <form className="message-composer" onSubmit={(event) => void send(event)}><label className="sr-only" htmlFor="message-body">{t('messageLabel')}</label><textarea id="message-body" placeholder={t('messagePlaceholder')} value={draft.body} maxLength={2000} rows={2} disabled={sending || !online} onChange={(event) => changeBody(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter' && !event.shiftKey && !event.nativeEvent.isComposing) { event.preventDefault(); void send() } }} /><button className="button button-primary send-button" aria-label={sending ? t('sendingMessage') : t('send')} disabled={sending || !draft.body.trim() || !online} type="submit"><Send size={20} /></button></form>}
  </section>
}
