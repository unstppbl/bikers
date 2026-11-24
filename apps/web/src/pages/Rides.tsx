import { useState } from 'react'
import type { FormEvent } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowLeftIcon as ArrowLeft, CalendarDotsIcon, CaretRightIcon as ChevronRight, CompassIcon as Compass, ImageSquareIcon as ImageSquare, LockKeyIcon as LockKeyhole, ShieldCheckIcon as ShieldCheck } from '@phosphor-icons/react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { ApiError, mutation, request, safeExternalUrl } from '../api'
import { useI18n } from '../i18n'
import { useSession, useSessionGuard } from '../session'
import { CreateRideLink, ErrorNotice, Field, Loading, PrivacyNote, TextArea, useOnline } from '../ui'
import type { MapPlan, Ride, RideInput } from '../types'
import type { CommunityRoute } from '../routes'
import { newRidePollPath, ridePollPath } from '../ride-polls'
import type { RidePollResponse } from '../ride-polls'
import { localRideTime, rideTimeToInstant, rideTimezone } from '../ride-time'
import { CitySelector } from '../components/CitySelector'
import { RideCoverEditor } from '../components/RideCover'
import { RideList } from './RideList'
import { MapPlanEditor } from '../components/maps/MapPlanEditor'
import type { MapDraft } from '../components/maps/MapPlanEditor'
import './ride-polls.css'

export function BrowsePage() {
  const { t } = useI18n()
  return <div className="page-content browse-page">
    <header className="browse-hero">
      <h1>{t('rides')}</h1>
      <p>{t('ridesFeedText')}</p>
      <div className="ride-poll-entry"><CreateRideLink /><PollForTimeLink /></div>
    </header>
    <RideList view="discover" />
    <details className="browse-guide">
      <summary><Compass size={18} />{t('howItWorks')}</summary>
      <div className="browse-guide-content">
        <section className="how-card"><ol>{(['One', 'Two', 'Three'] as const).map((step, index) => <li key={step}><span className="step-number">{index + 1}</span><div><strong>{t(`step${step}`)}</strong><p>{t(`step${step}Text`)}</p></div></li>)}</ol></section>
        <PrivacyNote />
      </div>
    </details>
  </div>
}

export function RideFormPage({ edit = false }: { edit?: boolean }) {
  const { rideId } = useParams()
  const [searchParams] = useSearchParams()
  const pollId = edit ? null : searchParams.get('poll')
  const sourceId = edit || pollId ? null : searchParams.get('route')
  const { user } = useSession()
  const { t } = useI18n()
  const rideQuery = useQuery({ queryKey: ['ride', rideId, user?.id], queryFn: () => request<{ ride: Ride }>(`/rides/${rideId}`), enabled: edit && !!rideId, refetchOnWindowFocus: false })
  const sourceQuery = useQuery({
    queryKey: ['ride-route-source', sourceId, user?.id],
    queryFn: ({ signal }) => request<{ route: CommunityRoute }>(`/routes/${encodeURIComponent(sourceId ?? '')}?scope=public`, { signal }),
    enabled: !!sourceId, staleTime: Infinity, gcTime: 0, retry: false, meta: { private: true },
  })
  const pollQuery = useQuery({
    queryKey: ['ride-poll-source', pollId, user?.id],
    queryFn: ({ signal }) => request<RidePollResponse>(ridePollPath(pollId ?? ''), { signal }),
    enabled: !!pollId, gcTime: 0, retry: false, meta: { private: true },
  })
  const [openedAt] = useState(Date.now)
  if (edit && rideQuery.isPending) return <Loading />
  if (edit && rideQuery.isError) return <ErrorNotice error={rideQuery.error} retry={() => void rideQuery.refetch()} />
  if (edit && !rideQuery.data?.ride.can_manage) return <ErrorNotice text={t('noAccess')} />
  if (sourceId && sourceQuery.isPending) return <Loading />
  if (pollId && pollQuery.isPending) return <Loading />
  const source = sourceQuery.data?.route
  if (sourceId && (!source?.published || source.kind !== 'planned' || !source.points || source.points.length < 2 || source.points.length > 10 || (source.segments?.[0]?.length ?? 0) < 2)) {
    return <div className="page-content form-page">
      <ErrorNotice text={t('mapRideSourceUnavailable')} retry={sourceQuery.isError ? () => void sourceQuery.refetch() : undefined} />
      <Link className="button button-outline" to="/rides/new">{t('mapRideStartBlank')}</Link>
    </div>
  }
  const poll = pollQuery.data?.poll
  const selected = poll?.options.find((option) => option.id === poll.selected_option_id)
  const pollPrefill = poll?.can_manage && selected && Date.parse(selected.starts_at) > openedAt
    ? { id: poll.id, city: poll.city, description: poll.description, startsAt: selected.starts_at, timezone: poll.timezone }
    : undefined
  if (pollId && !pollPrefill) {
    return <div className="page-content form-page">
      <ErrorNotice text={t('pollSourceUnavailable')} retry={pollQuery.isError ? () => void pollQuery.refetch() : undefined} />
      <Link className="button button-outline" to="/rides/new">{t('mapRideStartBlank')}</Link>
    </div>
  }
  const draftKey = edit ? `${rideId}-${rideQuery.data?.ride.plan_version}` : pollId ?? sourceId ?? 'new'
  return <RideForm key={`${user?.id}-${draftKey}`} ride={edit ? rideQuery.data?.ride : undefined} source={sourceId ? source : undefined} poll={pollPrefill} reload={() => void rideQuery.refetch()} />
}

function PollForTimeLink() {
  const { t } = useI18n()
  return <Link className="button button-outline" to={newRidePollPath}><CalendarDotsIcon size={19} />{t('pollForTime')}</Link>
}

interface PollPrefill {
  id: string
  city: string
  description: string
  startsAt: string
  timezone: string
}

function RideForm({ ride, source, poll, reload }: { ride?: Ride; source?: CommunityRoute; poll?: PollPrefill; reload: () => void }) {
  const { t } = useI18n()
  const { user } = useSession()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const online = useOnline()
  const captureSession = useSessionGuard()
  const initialPlan: MapPlan | undefined = ride?.map_plan ?? (source ? { points: source.points, distance_meters: source.distance_meters, duration_seconds: source.duration_seconds } : undefined)
  const referenceLine = source?.segments?.[0]
  const timezone = ride?.timezone ?? poll?.timezone ?? rideTimezone
  const initialTime = localRideTime(ride?.starts_at ?? poll?.startsAt, timezone)
  const [form, setForm] = useState({
    title: ride?.title ?? source?.title ?? '', city: ride?.city ?? poll?.city ?? source?.city ?? user?.profile.city ?? '', description: ride?.description ?? poll?.description ?? '', starts_at: initialTime,
    meeting_point: ride?.meeting_point ?? source?.points[0]?.label ?? '', destination: ride?.destination ?? source?.points.at(-1)?.label ?? '', route_url: ride?.route_url ?? '', expectations: ride?.expectations ?? '', capacity: String(ride?.capacity ?? 8),
  })
  const [mapDraft, setMapDraft] = useState<MapDraft>({ points: initialPlan?.points ?? [], valid: true })
  const [confirmedPlan, setConfirmedPlan] = useState('')
  const planSignature = JSON.stringify([form.starts_at, form.meeting_point, mapDraft.points, mapDraft.previewDigest])
  const sourcePlanReady = !!mapDraft.previewDigest && !!form.starts_at && !!form.meeting_point.trim()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<unknown>()
  const [validation, setValidation] = useState('')
  function update(key: keyof typeof form, value: string) { setForm((current) => ({ ...current, [key]: value })) }
  async function submit(event: FormEvent) {
    const currentSession = captureSession()
    event.preventDefault(); setValidation(''); setError(undefined)
    if (![form.title, form.city, form.description, form.starts_at, form.meeting_point].every((value) => value.trim())) { setValidation(t('requiredFields')); return }
    if (!mapDraft.valid) { setValidation(t('mapSelectionRequired')); return }
    if (source && (!sourcePlanReady || confirmedPlan !== planSignature)) { setValidation(t('mapRideConfirmRequired')); return }
    const startsAt = poll && form.starts_at === initialTime ? new Date(poll.startsAt) : rideTimeToInstant(form.starts_at, timezone)
    if (Number.isNaN(startsAt.getTime()) || (!ride && startsAt.getTime() <= Date.now())) { setValidation(t('invalidTime')); return }
    if (!Number.isInteger(Number(form.capacity)) || Number(form.capacity) < 1 || Number(form.capacity) > 1000) { setValidation(t('invalidCapacity')); return }
    if (form.route_url.trim() && !safeExternalUrl(form.route_url.trim())) { setValidation(t('invalidUrl')); return }
    const body: RideInput = { title: form.title.trim(), city: form.city.trim(), description: form.description.trim(), starts_at: startsAt.toISOString(), timezone, meeting_point: form.meeting_point.trim(), destination: form.destination.trim(), route_url: form.route_url.trim(), expectations: form.expectations.trim(), capacity: Number(form.capacity), ...(ride ? { plan_version: ride.plan_version } : {}), ...(JSON.stringify(mapDraft.points) !== JSON.stringify(ride?.map_plan?.points ?? []) ? { map_points: mapDraft.points } : {}), ...(source ? { expected_map_digest: mapDraft.previewDigest } : {}) }
    setBusy(true)
    try {
      const result = await mutation<{ ride: Ride }>(ride ? `/rides/${ride.id}` : '/rides', ride ? 'PATCH' : 'POST', body)
      if (!currentSession()) return
      await queryClient.invalidateQueries({ queryKey: ['rides'] })
      if (!currentSession()) return
      queryClient.setQueryData(['ride', result.ride.id, user?.id], result)
      void navigate(`/rides/${result.ride.id}`)
    } catch (value) {
      if (value instanceof ApiError && value.code === 'route_preview_changed') setConfirmedPlan('')
      setError(value)
    }
    finally { setBusy(false) }
  }
  const backPath = ride ? `/rides/${ride.id}` : poll ? ridePollPath(poll.id) : '/'
  return <div className="page-content form-page"><Link className="back-link" to={backPath}><ArrowLeft size={17} />{t(ride ? 'backToRide' : poll ? 'pollBack' : 'back')}</Link><div className="page-heading"><span className="eyebrow">{t('createEyebrow')}</span><h1>{t(ride ? 'editTitle' : 'createTitle')}</h1><p>{t('createText')}</p></div>
    {/* The cover uploads on its own, so it sits outside the ride form. A new ride gets one after it is created. */}
    {ride && <RideCoverEditor ride={ride} />}
    {!ride && !source && !poll && <div className="ride-poll-entry ride-poll-start-entry"><PollForTimeLink /></div>}
    {poll && <div className="inline-note route-source-note"><CalendarDotsIcon size={20} /><p>{t('pollSourceNote')}</p></div>}
    {source && <div className="inline-note route-source-note"><Compass size={20} /><div><p>{t('mapRideSourceNote', { title: source.title })}</p><Link className="inline-link" to="/rides/new">{t('mapRideStartBlank')}</Link></div></div>}
    <form onSubmit={(event) => void submit(event)} noValidate>
      <section className="surface form-section">
        <div className="form-section-heading"><span className="form-section-number">01</span><div><h2>{t('publicFields')}</h2><p>{t('publicFieldsHint')}</p></div><Compass size={22} /></div>
        <CitySelector label={t('city')} value={form.city} onChange={(city) => update('city', city)} placeholder={t('cityPlaceholder')} required disabled={busy} />
        <TextArea label={t('description')} hint={t('mapRidePublicDescription')} value={form.description} onChange={(value) => update('description', value)} placeholder={t('descriptionPlaceholder')} maxLength={500} required rows={3} />
        {!ride && <div className="inline-note ride-cover-note"><ImageSquare size={20} /><p>{t('rideCoverAfterCreate')}</p></div>}
      </section>
      <section className="surface form-section">
        <div className="form-section-heading"><span className="form-section-number">02</span><div><h2>{t('privateFields')}</h2><p>{t('privateFieldsHint')}</p></div><LockKeyhole size={21} /></div>
        <Field label={t('title')} value={form.title} onChange={(event) => update('title', event.target.value)} placeholder={t('titlePlaceholder')} maxLength={150} required disabled={busy} />
        <div className="form-grid">
          <Field label={t('startsAt')} hint={timezone !== rideTimezone ? timezone : t('timezoneHint')} type="datetime-local" value={form.starts_at} onChange={(event) => update('starts_at', event.target.value)} required disabled={busy} />
          <Field label={t('capacity')} hint={t('capacityHint')} type="number" min="1" max="1000" inputMode="numeric" value={form.capacity} onChange={(event) => update('capacity', event.target.value)} required disabled={busy} />
        </div>
        <MapPlanEditor initialPlan={initialPlan} referenceLine={referenceLine} meeting={form.meeting_point} destination={form.destination} onMeeting={(value) => update('meeting_point', value)} onDestination={(value) => update('destination', value)} onChange={setMapDraft} disabled={busy} startWithMap={!ride} />
        {source && <label className="map-plan-confirm">
          <input type="checkbox" checked={sourcePlanReady && confirmedPlan === planSignature} disabled={busy || !sourcePlanReady} onChange={(event) => setConfirmedPlan(event.target.checked ? planSignature : '')} />
          {t('mapRideConfirmPlan')}
        </label>}
        <Field label={`${t('route')} · ${t('optional')}`} type="url" value={form.route_url} onChange={(event) => update('route_url', event.target.value)} placeholder={t('routePlaceholder')} maxLength={2000} disabled={busy} />
        <TextArea label={`${t('expectations')} · ${t('optional')}`} value={form.expectations} onChange={(value) => update('expectations', value)} placeholder={t('expectationsPlaceholder')} maxLength={1000} rows={3} />
        <div className="inline-note"><ShieldCheck size={20} /><p>{t('approvalNote')}</p></div>
      </section>
      {validation && <ErrorNotice text={validation} />}{error != null && <ErrorNotice error={error} retry={ride ? reload : undefined} />}
      <div className="form-actions"><Link className="button button-outline" to={backPath}>{t('notNow')}</Link><button className="button button-primary" type="submit" disabled={busy || !online}>{busy ? t('saving') : t(ride ? 'savePlan' : 'create')}<ChevronRight size={18} /></button></div>
    </form>
  </div>
}
