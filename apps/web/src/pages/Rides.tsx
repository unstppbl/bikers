import { lazy, Suspense, useState } from 'react'
import type { FormEvent } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowDown, ArrowLeft, ArrowUpRight, ChevronRight, Compass, LockKeyhole, ShieldCheck } from 'lucide-react'
import { fromZonedTime, formatInTimeZone } from 'date-fns-tz'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { mutation, publicRide, request, safeExternalUrl } from '../api'
import { useI18n } from '../i18n'
import { useSession, useSessionGuard } from '../session'
import { CreateRideLink, ErrorNotice, Field, Loading, PrivacyNote, TextArea, useOnline } from '../ui'
import type { Ride, RideInput, RidePage } from '../types'
import { CitySelector } from '../components/CitySelector'
import { RideList } from './RideList'

const RidesDesignPrototype = import.meta.env.DEV
  ? lazy(() => import('./RidesDesignPrototype'))
  : null

export function BrowsePage() {
  const { t } = useI18n()
  const { user, loading, error } = useSession()
  const [filter, setFilter] = useState('')
  const [heroImageAvailable, setHeroImageAvailable] = useState(true)
  const [searchParams] = useSearchParams()
  const prototype = !!RidesDesignPrototype && ['A', 'B', 'C'].includes(searchParams.get('variant') ?? '')
  const ridesQuery = useQuery({ queryKey: ['rides', user?.id ?? 'guest', 'prototype'], enabled: prototype && !loading && !error, queryFn: async ({ signal }) => {
    const result = await request<RidePage>('/rides?view=discover', { signal })
    return user ? result : { rides: result.rides.map(publicRide) }
  }, meta: { private: true } })
  const rides = ridesQuery.data?.rides ?? []
  if (RidesDesignPrototype && ['A', 'B', 'C'].includes(searchParams.get('variant') ?? '')) {
    return <Suspense fallback={<Loading />}><RidesDesignPrototype rides={rides} filter={filter} onFilterChange={setFilter} loading={ridesQuery.isPending} error={ridesQuery.error ?? undefined} /></Suspense>
  }
  return <div className="page-content browse-page">
    <section className="browse-hero">
      <div className="hero-copy">
        <span className="eyebrow">{t('chapter')}</span>
        <h1>{t('heroFirst')}<br /><span>{t('heroSecond')}</span></h1>
        <p>{t('heroText')}</p>
        <div className="hero-actions"><a className="button button-primary" href="#rides-heading">{t('browse')}<ArrowDown size={18} /></a><CreateRideLink className="button button-outline" /></div>
      </div>
      <span className="hero-bike-mark" aria-hidden="true">KZ</span>
      {heroImageAvailable && <figure className="hero-motorcycle">
        <img src="/bike-models/ktm-390-adventure-2024.png" width={918} height={629} alt={t('modelReferenceAlt', { make: 'KTM', model: '390 Adventure', years: '2024' })} decoding="async" fetchPriority="high" onError={() => setHeroImageAvailable(false)} />
        <figcaption><span>KTM 390 Adventure · 2024</span><a href="https://commons.wikimedia.org/wiki/File:Ktm-390-adventure-2024.png" target="_blank" rel="noopener noreferrer">{t('modelReferenceAuthor', { author: 'KTM AG' })}</a><a href="https://creativecommons.org/licenses/by-sa/4.0/" target="_blank" rel="noopener noreferrer">CC BY-SA 4.0</a></figcaption>
      </figure>}
      <div className="hero-proof"><ShieldCheck size={15} />{t('organizerApproval')}</div>
    </section>
    <div className="browse-grid"><RideList view="discover" /><aside className="browse-aside"><section className="how-card"><div className="mini-icon"><Compass size={23} strokeWidth={1.5} /></div><h3>{t('howItWorks')}</h3><ol>{(['One', 'Two', 'Three'] as const).map((step, index) => <li key={step}><span className="step-number">0{index + 1}</span><div><strong>{t(`step${step}`)}</strong><p>{t(`step${step}Text`)}</p></div></li>)}</ol><ArrowUpRight className="how-arrow" size={25} /></section><PrivacyNote /></aside></div>
  </div>
}

export function RideFormPage({ edit = false }: { edit?: boolean }) {
  const { rideId } = useParams()
  const { user } = useSession()
  const { t } = useI18n()
  const rideQuery = useQuery({ queryKey: ['ride', rideId, user?.id], queryFn: () => request<{ ride: Ride }>(`/rides/${rideId}`), enabled: edit && !!rideId, refetchOnWindowFocus: false })
  if (edit && rideQuery.isPending) return <Loading />
  if (edit && rideQuery.isError) return <ErrorNotice error={rideQuery.error} retry={() => void rideQuery.refetch()} />
  if (edit && !rideQuery.data?.ride.can_manage) return <ErrorNotice text={t('noAccess')} />
  return <RideForm key={`${user?.id}-${edit ? `${rideId}-${rideQuery.data?.ride.plan_version}` : 'new'}`} ride={edit ? rideQuery.data?.ride : undefined} reload={() => void rideQuery.refetch()} />
}

export function localRideTime(value?: string, timezone = 'Asia/Almaty') {
  if (!value) return ''
  return formatInTimeZone(value, timezone, "yyyy-MM-dd'T'HH:mm")
}

export function rideTimeToInstant(value: string, timezone: string) { return fromZonedTime(value, timezone) }

function RideForm({ ride, reload }: { ride?: Ride; reload: () => void }) {
  const { t } = useI18n()
  const { user } = useSession()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const online = useOnline()
  const captureSession = useSessionGuard()
  const [form, setForm] = useState({
    title: ride?.title ?? '', city: ride?.city ?? user?.profile.city ?? '', description: ride?.description ?? '', starts_at: localRideTime(ride?.starts_at, ride?.timezone),
    meeting_point: ride?.meeting_point ?? '', destination: ride?.destination ?? '', route_url: ride?.route_url ?? '', expectations: ride?.expectations ?? '', capacity: String(ride?.capacity ?? 8),
  })
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<unknown>()
  const [validation, setValidation] = useState('')
  function update(key: keyof typeof form, value: string) { setForm((current) => ({ ...current, [key]: value })) }
  async function submit(event: FormEvent) {
    const currentSession = captureSession()
    event.preventDefault(); setValidation(''); setError(undefined)
    if (![form.title, form.city, form.description, form.starts_at, form.meeting_point].every((value) => value.trim())) { setValidation(t('requiredFields')); return }
    const timezone = ride?.timezone ?? 'Asia/Almaty'
    const startsAt = rideTimeToInstant(form.starts_at, timezone)
    if (Number.isNaN(startsAt.getTime()) || (!ride && startsAt.getTime() <= Date.now())) { setValidation(t('invalidTime')); return }
    if (!Number.isInteger(Number(form.capacity)) || Number(form.capacity) < 1 || Number(form.capacity) > 1000) { setValidation(t('invalidCapacity')); return }
    if (form.route_url.trim() && !safeExternalUrl(form.route_url.trim())) { setValidation(t('invalidUrl')); return }
    const body: RideInput = { title: form.title.trim(), city: form.city.trim(), description: form.description.trim(), starts_at: startsAt.toISOString(), timezone, meeting_point: form.meeting_point.trim(), destination: form.destination.trim(), route_url: form.route_url.trim(), expectations: form.expectations.trim(), capacity: Number(form.capacity), ...(ride ? { plan_version: ride.plan_version } : {}) }
    setBusy(true)
    try {
      const result = await mutation<{ ride: Ride }>(ride ? `/rides/${ride.id}` : '/rides', ride ? 'PATCH' : 'POST', body)
      if (!currentSession()) return
      await queryClient.invalidateQueries({ queryKey: ['rides'] })
      if (!currentSession()) return
      queryClient.setQueryData(['ride', result.ride.id, user?.id], result)
      void navigate(`/rides/${result.ride.id}`)
    } catch (value) { setError(value) }
    finally { setBusy(false) }
  }
  return <div className="page-content form-page"><Link className="back-link" to={ride ? `/rides/${ride.id}` : '/'}><ArrowLeft size={17} />{t('back')}</Link><div className="page-heading"><span className="eyebrow">{t('createEyebrow')}</span><h1>{t(ride ? 'editTitle' : 'createTitle')}</h1><p>{t('createText')}</p></div>
    <form onSubmit={(event) => void submit(event)} noValidate><section className="surface form-section"><div className="form-section-heading"><span className="form-section-number">01</span><div><h2>{t('publicFields')}</h2><p>{t('publicFieldsHint')}</p></div><Compass size={22} /></div><CitySelector label={t('city')} value={form.city} onChange={(city) => update('city', city)} placeholder={t('cityPlaceholder')} maxLength={100} required disabled={busy} /><TextArea label={t('description')} value={form.description} onChange={(value) => update('description', value)} placeholder={t('descriptionPlaceholder')} maxLength={500} required rows={3} /></section>
      <section className="surface form-section"><div className="form-section-heading"><span className="form-section-number">02</span><div><h2>{t('privateFields')}</h2><p>{t('privateFieldsHint')}</p></div><LockKeyhole size={21} /></div><Field label={t('title')} value={form.title} onChange={(event) => update('title', event.target.value)} placeholder={t('titlePlaceholder')} maxLength={150} required disabled={busy} /><div className="form-grid"><Field label={t('startsAt')} hint={ride?.timezone && ride.timezone !== 'Asia/Almaty' ? ride.timezone : t('timezoneHint')} type="datetime-local" value={form.starts_at} onChange={(event) => update('starts_at', event.target.value)} required disabled={busy} /><Field label={t('capacity')} hint={t('capacityHint')} type="number" min="1" max="1000" inputMode="numeric" value={form.capacity} onChange={(event) => update('capacity', event.target.value)} required disabled={busy} /></div><Field label={t('meetingPoint')} value={form.meeting_point} onChange={(event) => update('meeting_point', event.target.value)} placeholder={t('meetingPlaceholder')} maxLength={300} required disabled={busy} /><Field label={`${t('destination')} · ${t('optional')}`} value={form.destination} onChange={(event) => update('destination', event.target.value)} placeholder={t('destinationPlaceholder')} maxLength={300} disabled={busy} /><Field label={`${t('route')} · ${t('optional')}`} type="url" value={form.route_url} onChange={(event) => update('route_url', event.target.value)} placeholder={t('routePlaceholder')} maxLength={2000} disabled={busy} /><TextArea label={`${t('expectations')} · ${t('optional')}`} value={form.expectations} onChange={(value) => update('expectations', value)} placeholder={t('expectationsPlaceholder')} maxLength={1000} rows={3} /><div className="inline-note"><ShieldCheck size={20} /><p>{t('approvalNote')}</p></div></section>
      {validation && <ErrorNotice text={validation} />}{error != null && <ErrorNotice error={error} retry={ride ? reload : undefined} />}
      <div className="form-actions"><Link className="button button-outline" to={ride ? `/rides/${ride.id}` : '/'}>{t('notNow')}</Link><button className="button button-primary" type="submit" disabled={busy || !online}>{busy ? t('saving') : t(ride ? 'savePlan' : 'create')}<ChevronRight size={18} /></button></div>
    </form>
  </div>
}
