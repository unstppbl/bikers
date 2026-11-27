import { useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { useQuery } from '@tanstack/react-query'
import { ArrowRightIcon, CalendarDotsIcon, CheckIcon as Check, ClockIcon, LockKeyIcon, MapPinIcon as MapPin, MagnifyingGlassIcon as Search, UsersThreeIcon, XIcon } from '@phosphor-icons/react'
import { formatInTimeZone } from 'date-fns-tz'
import { Link, Navigate, useLocation, useNavigate, useSearchParams } from 'react-router-dom'
import { publicRide, request } from '../api'
import { cityLabel } from '../cities'
import { CitySelector, useCityOptions } from '../components/CitySelector'
import { useI18n } from '../i18n'
import { useSession } from '../session'
import { CreateRideLink, EmptyState, ErrorNotice, Field, Loading, Pagination, dateParts, dateRangeLabel } from '../ui'
import { CityArt, DatePlate, goingLabel } from '../components/RideVisual'
import { RideImage } from '../components/RideCover'
import type { Ride, RidePage, RideStyle, RideSurface } from '../types'

const personalViews = ['organizing', 'joined', 'pending', 'past'] as const
type PersonalRideView = typeof personalViews[number]
type RideView = 'discover' | PersonalRideView
const styleChoices: Exclude<RideStyle, ''>[] = ['chill', 'touring', 'sport', 'enduro', 'city']
const surfaceChoices: Exclude<RideSurface, ''>[] = ['asphalt', 'mixed', 'unpaved']
interface Filters { q: string; city: string; from: string; to: string; style: string; surface: string }
interface Paging { scope: string; cursors: string[] }

function kazakhstanDay(offsetDays = 0) {
  const date = new Date(Date.now() + offsetDays * 86_400_000)
  return formatInTimeZone(date, 'Asia/Almaty', 'yyyy-MM-dd')
}

function upcomingWeekendRange(): [string, string] {
  const now = new Date()
  const isoDay = Number(formatInTimeZone(now, 'Asia/Almaty', 'i')) // 1=Mon .. 7=Sun
  const daysToSat = isoDay <= 6 ? 6 - isoDay : 0
  const daysToSun = 7 - isoDay
  return [kazakhstanDay(daysToSat), kazakhstanDay(daysToSun)]
}

export function RidesPillarSwitch({ current }: { current: 'discover' | 'mine' }) {
  const { t } = useI18n()
  return <nav className="pillar-tabs" aria-label={t('rides')}>
    <Link to="/" aria-current={current === 'discover' ? 'page' : undefined}>{t('discoverTab')}</Link>
    <Link to="/my-rides" aria-current={current === 'mine' ? 'page' : undefined}>{t('myRides')}</Link>
  </nav>
}

export function MyRidesPage() {
  const { t } = useI18n()
  const [params] = useSearchParams()
  const selected = params.get('view')
  const view = personalViews.find((value) => value === selected) ?? 'organizing'
  return <div className="page-content collection-page my-rides-page">
    <div className="collection-heading"><div className="page-heading"><span className="eyebrow">{t('yourRidingPlans')}</span><h1>{t('myRides')}</h1><p>{t('myRidesText')}</p></div><CreateRideLink /></div>
    <RidesPillarSwitch current="mine" />
    <nav className="collection-tabs ride-view-tabs" aria-label={t('myRides')}>
      {personalViews.map((value) => {
        const next = new URLSearchParams(params)
        next.set('view', value)
        next.delete('cursor')
        return <Link key={value} className={view === value ? 'active' : ''} aria-current={view === value ? 'page' : undefined} to={{ pathname: '/my-rides', search: next.toString() }}>{t(value)}</Link>
      })}
    </nav>
    <RideList view={view} />
  </div>
}

export function RideList({ view }: { view: RideView }) {
  const { t } = useI18n()
  const { user, loading, error, refresh } = useSession()
  const [params] = useSearchParams()
  const location = useLocation()
  const navigate = useNavigate()
  const filters: Filters = {
    q: params.get('q') ?? '',
    city: params.get('city') ?? '',
    from: params.get('from') ?? '',
    to: params.get('to') ?? '',
    style: params.get('style') ?? '',
    surface: params.get('surface') ?? '',
  }
  const cursor = params.get('cursor') ?? ''
  const scope = JSON.stringify([user?.id ?? 'guest', view, filters.q, filters.city, filters.from, filters.to, filters.style, filters.surface])
  const stored = (location.state as { ridePaging?: Paging } | null)?.ridePaging
  const staleCursor = !!cursor && !!stored && stored.scope !== scope
  const cursors = stored?.scope === scope && stored.cursors.at(-1) === cursor ? stored.cursors : cursor ? ['', cursor] : ['']
  const page = cursors.length - 1
  const queryParams = new URLSearchParams({ view })
  for (const [key, value] of Object.entries(filters)) if (value) queryParams.set(key, value)
  if (cursor) queryParams.set('cursor', cursor)
  const query = useQuery({
    queryKey: ['rides', user?.id ?? 'guest', view, filters.q, filters.city, filters.from, filters.to, filters.style, filters.surface, cursor],
    enabled: !loading && !error && !staleCursor && (view === 'discover' || !!user),
    queryFn: async ({ signal }) => {
      const result = await request<RidePage>(`/rides?${queryParams}`, { signal })
      return user ? result : { ...result, rides: result.rides.map(publicRide) }
    },
    meta: { private: true },
  })

  function goToCursor(nextCursors: string[]) {
    const next = new URLSearchParams(params)
    const current = nextCursors.at(-1) ?? ''
    if (current) next.set('cursor', current)
    else next.delete('cursor')
    void navigate({ pathname: location.pathname, search: next.toString() }, { state: { ridePaging: { scope, cursors: nextCursors } } })
  }

  if (loading) return <Loading />
  if (error) return <ErrorNotice text={t('serviceUnavailable')} retry={() => void refresh()} />
  if (staleCursor) {
    const next = new URLSearchParams(params)
    next.delete('cursor')
    return <Navigate to={{ pathname: location.pathname, search: next.toString() }} replace />
  }
  const heading = view === 'discover' ? t('upcoming') : t(view)
  const hasFilters = Object.values(filters).some(Boolean)
  const featured = view === 'discover' && page === 0
  return <section className="ride-list-section" aria-labelledby="rides-heading">
    <div className={`section-header${view === 'discover' ? ' sr-only' : ''}`}><h2 id="rides-heading" tabIndex={-1}>{heading}</h2>{query.data && <span className="count-pill">{t('ridesOnPage', { count: query.data.rides.length })}</span>}</div>
    <RideFilters filters={filters} showChips={view === 'discover'} />
    {view === 'past' && <p className="section-intro">{t('pastRidesHint')}</p>}
    {query.isPending ? <Loading /> : query.isError ? <><ErrorNotice error={query.error} retry={() => void query.refetch()} />{cursor && <button type="button" className="text-button" onClick={() => goToCursor([''])}>{t('firstRidePage')}</button>}</> : !query.data.rides.length ? <EmptyState
      title={hasFilters || cursor ? t('noResults') : t(view === 'discover' ? 'emptyTitle' : 'myRidesEmpty')}
      text={hasFilters || cursor ? t('rideFiltersEmptyText') : t(view === 'discover' ? 'emptyText' : `${view}Empty`)}
      action={view === 'organizing' || view === 'discover' ? <CreateRideLink className="button button-outline" /> : <Link className="button button-outline" to="/">{t('browse')}</Link>}
    /> : <div className={`ride-list${featured ? ' ride-list-featured' : ''}`}>{query.data.rides.map((ride, index) => <RideCard key={ride.id} ride={ride} featured={featured && index === 0} />)}</div>}
    {!query.isError && <Pagination page={page} busy={query.isFetching} hasNext={!!query.data?.next_cursor} previous={() => goToCursor(cursors.slice(0, -1))} next={() => { if (query.data?.next_cursor) goToCursor([...cursors, query.data.next_cursor]) }} />}
  </section>
}

const sameFilters = (a: Filters, b: Filters) => a.q === b.q && a.city === b.city && a.from === b.from && a.to === b.to && a.style === b.style && a.surface === b.surface

function RideFilters({ filters, showChips }: { filters: Filters; showChips: boolean }) {
  const { t, locale } = useI18n()
  const [params, setParams] = useSearchParams()
  const form = useRef<HTMLFormElement>(null)
  const [draft, setDraft] = useState(filters)
  const [shown, setShown] = useState(filters)
  const [invalidRange, setInvalidRange] = useState(false)
  const [datesOpen, setDatesOpen] = useState(!!filters.from || !!filters.to)
  if (!sameFilters(shown, filters)) {
    setShown(filters); setDraft(filters); setInvalidRange(false); setDatesOpen(!!filters.from || !!filters.to)
  }
  function apply(values: Filters) {
    const next = new URLSearchParams(params)
    next.delete('cursor')
    for (const [key, value] of Object.entries(values)) {
      if (value.trim()) next.set(key, value.trim())
      else next.delete(key)
    }
    setParams(next)
  }
  function submit(event: FormEvent) {
    event.preventDefault()
    const invalid = !!draft.from && !!draft.to && draft.from > draft.to
    setInvalidRange(invalid)
    if (invalid) setDatesOpen(true)
    else apply(draft)
  }
  function clear() {
    const empty: Filters = { q: '', city: '', from: '', to: '', style: '', surface: '' }
    setDraft(empty)
    setInvalidRange(false)
    apply(empty)
    form.current?.querySelector<HTMLInputElement>('input[type="search"]')?.focus()
  }
  const today = kazakhstanDay(0)
  const [sat, sun] = upcomingWeekendRange()
  const isToday = filters.from === today && filters.to === today
  const isWeekend = filters.from === sat && filters.to === sun
  const dateLabel = filters.from || filters.to ? dateRangeLabel(filters.from, filters.to, locale) : t('allDates')
  return <>
    <form ref={form} className="ride-filters" aria-label={t('rideFilters')} onSubmit={submit}>
      <div className="ride-filter-main">
        <div className="ride-filter-search"><Search className="ride-filter-icon" size={18} /><Field label={t('rideSearch')} type="search" value={draft.q} onChange={(event) => setDraft({ ...draft, q: event.target.value })} maxLength={200} placeholder={t('rideSearch')} /></div>
        <div className="ride-filter-city"><MapPin className="ride-filter-icon" size={18} /><CitySelector value={draft.city} onChange={(city) => setDraft({ ...draft, city })} label={t('city')} placeholder={t('city')} hint="" /></div>
      </div>
      <details className="ride-date-filters" open={datesOpen} onToggle={(event) => setDatesOpen(event.currentTarget.open)}>
        <summary><CalendarDotsIcon size={17} />{dateLabel}</summary>
        <div className="ride-date-panel">
          <div className="ride-filter-fields">
            <Field label={t('fromDate')} type="date" value={draft.from} onChange={(event) => setDraft({ ...draft, from: event.target.value })} max={draft.to || undefined} />
            <Field label={t('toDate')} type="date" value={draft.to} onChange={(event) => setDraft({ ...draft, to: event.target.value })} min={draft.from || undefined} />
          </div>
          <p className="ride-filter-note">{t('rideDatesHint')}</p>
        </div>
      </details>
      <div className="ride-filter-actions"><button type="submit" className="button button-primary apply-filters"><span className="apply-label">{t('applyFilters')}</span><ArrowRightIcon className="apply-icon" size={18} weight="bold" /></button></div>
      {Object.values(filters).some(Boolean) && <button type="button" className="text-button clear-filters" onClick={clear}><XIcon size={15} />{t('clearFilters')}</button>}
      {invalidRange && <ErrorNotice text={t('rideDateRangeInvalid')} />}
    </form>
    {showChips && <div className="ride-chips" role="group" aria-label={t('rideFilters')}>
      <button type="button" className="filter-chip" aria-pressed={isToday} onClick={() => apply({ ...draft, from: isToday ? '' : today, to: isToday ? '' : today })}>{t('quickToday')}</button>
      <button type="button" className="filter-chip" aria-pressed={isWeekend} onClick={() => apply({ ...draft, from: isWeekend ? '' : sat, to: isWeekend ? '' : sun })}>{t('quickWeekend')}</button>
      <span className="filter-chip-sep" aria-hidden="true" />
      {styleChoices.map((value) => <button key={value} type="button" className="filter-chip" aria-pressed={filters.style === value} onClick={() => apply({ ...draft, style: filters.style === value ? '' : value })}>{t(`style_${value}`)}</button>)}
      <span className="filter-chip-sep" aria-hidden="true" />
      {surfaceChoices.map((value) => <button key={value} type="button" className="filter-chip" aria-pressed={filters.surface === value} onClick={() => apply({ ...draft, surface: filters.surface === value ? '' : value })}>{t(`surface_${value}`)}</button>)}
    </div>}
  </>
}

export function RideCard({ ride, featured = false }: { ride: Ride; featured?: boolean }) {
  const { t, locale } = useI18n()
  const cities = useCityOptions()
  const city = cityLabel(ride.city, locale, cities.data?.cities ?? [])
  const state = ride.participation_state
  const member = ride.can_manage || state === 'approved'
  const heading = ride.title?.trim() || ride.description
  const showSnippet = !!ride.title?.trim() && ride.description.trim() && ride.title.trim() !== ride.description.trim()
  const timeWindowLabel = ride.time_window ? t(`window_${ride.time_window}`) : undefined
  const hasSpots = !!ride.capacity && ride.capacity > 0
  const approved = ride.approved_count ?? 0
  const isFull = hasSpots && approved >= (ride.capacity ?? 0)
  return <Link to={`/rides/${ride.id}`} className={`ride-card${ride.status === 'cancelled' ? ' is-cancelled' : ''}`}>
    <div className="ride-card-media">
      <RideImage ride={ride} className="ride-card-cover" priority={featured} />
      <CityArt city={city} />
      <DatePlate date={ride.date} locale={locale} timeWindow={timeWindowLabel} />
      <span className="ride-card-status">
        {ride.status === 'cancelled' && <span className="status-pill cancelled">{t('cancelled')}</span>}
        {ride.can_manage && <span className="status-pill">{t('organizer')}</span>}
        {state === 'requested' && <span className="status-pill pending"><ClockIcon size={13} />{t('requested')}</span>}
        {state === 'approved' && !ride.can_manage && <span className="status-pill approved"><Check size={13} weight="bold" />{t(goingLabel(ride))}</span>}
      </span>
    </div>
    <div className="ride-card-body">
      <h3>{heading}</h3>
      {showSnippet && <p className="ride-card-snippet">{ride.description}</p>}
      <div className="ride-card-specs">
        <span className="ride-spec-tag"><MapPin size={13} />{city}</span>
        {ride.distance_km ? <span className="ride-spec-tag is-mono">{t('approxKm', { km: ride.distance_km })}</span> : null}
        {ride.style && <span className="ride-spec-tag">{t(`style_${ride.style}`)}</span>}
        {ride.surface && <span className="ride-spec-tag">{t(`surface_${ride.surface}`)}</span>}
        {hasSpots && <span className="ride-spec-tag is-mono"><UsersThreeIcon size={13} />{isFull ? t('spotsFull', { approved, capacity: ride.capacity! }) : t('spotsCount', { approved, capacity: ride.capacity! })}</span>}
      </div>
      <div className="ride-card-meta">
        <span><CalendarDotsIcon size={14} />{dateParts(ride.date, locale).full}</span>
        {ride.organizer_name ? <span className="ride-card-city">{t('byOrganizer', { name: ride.organizer_name })}</span> : !member && ride.status !== 'cancelled' && <span><LockKeyIcon size={14} />{t('planAfterApproval')}</span>}
      </div>
    </div>
  </Link>
}
