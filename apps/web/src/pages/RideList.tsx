import { useState } from 'react'
import type { FormEvent } from 'react'
import { useQuery } from '@tanstack/react-query'
import { ArrowUpRight, Check, MapPin, Search, SlidersHorizontal } from 'lucide-react'
import { Link, Navigate, useLocation, useNavigate, useSearchParams } from 'react-router-dom'
import { publicRide, request } from '../api'
import { cityLabel } from '../cities'
import { CitySelector, useCityOptions } from '../components/CitySelector'
import { useI18n } from '../i18n'
import { useSession } from '../session'
import { CreateRideLink, EmptyState, ErrorNotice, Field, Loading, Pagination, dateParts, formatCalendarDate } from '../ui'
import type { Ride, RidePage } from '../types'

const personalViews = ['organizing', 'joined', 'pending', 'past'] as const
type PersonalRideView = typeof personalViews[number]
type RideView = 'discover' | PersonalRideView
interface Filters { q: string; city: string; from: string; to: string }
interface Paging { scope: string; cursors: string[] }

export function MyRidesPage() {
  const { t } = useI18n()
  const [params] = useSearchParams()
  const selected = params.get('view')
  const view = personalViews.find((value) => value === selected) ?? 'organizing'
  return <div className="page-content collection-page my-rides-page">
    <div className="collection-heading"><div className="page-heading"><span className="eyebrow">{t('yourRidingPlans')}</span><h1>{t('myRides')}</h1><p>{t('myRidesText')}</p></div><CreateRideLink /></div>
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
  const filters: Filters = { q: params.get('q') ?? '', city: params.get('city') ?? '', from: params.get('from') ?? '', to: params.get('to') ?? '' }
  const cursor = params.get('cursor') ?? ''
  const scope = JSON.stringify([user?.id ?? 'guest', view, filters.q, filters.city, filters.from, filters.to])
  const stored = (location.state as { ridePaging?: Paging } | null)?.ridePaging
  const staleCursor = !!cursor && !!stored && stored.scope !== scope
  const cursors = stored?.scope === scope && stored.cursors.at(-1) === cursor ? stored.cursors : cursor ? ['', cursor] : ['']
  const page = cursors.length - 1
  const queryParams = new URLSearchParams({ view })
  for (const [key, value] of Object.entries(filters)) if (value) queryParams.set(key, value)
  if (cursor) queryParams.set('cursor', cursor)
  const query = useQuery({
    queryKey: ['rides', user?.id ?? 'guest', view, filters.q, filters.city, filters.from, filters.to, cursor],
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
  return <section className="ride-list-section" aria-labelledby="rides-heading">
    <div className="section-header"><div>{view === 'discover' && <span className="eyebrow">{t('browse')}</span>}<h2 id="rides-heading" tabIndex={-1}>{heading}</h2></div>{query.data && <span className="count-pill">{t('ridesOnPage', { count: query.data.rides.length })}</span>}</div>
    <RideFilters key={JSON.stringify(filters)} filters={filters} />
    {view === 'past' && <p className="section-intro">{t('pastRidesHint')}</p>}
    {query.isPending ? <Loading /> : query.isError ? <><ErrorNotice error={query.error} retry={() => void query.refetch()} />{cursor && <button type="button" className="text-button" onClick={() => goToCursor([''])}>{t('firstRidePage')}</button>}</> : !query.data.rides.length ? <EmptyState
      title={hasFilters || cursor ? t('noResults') : t(view === 'discover' ? 'emptyTitle' : 'myRidesEmpty')}
      text={hasFilters || cursor ? t('rideFiltersEmptyText') : t(view === 'discover' ? 'emptyText' : `${view}Empty`)}
      action={view === 'organizing' || view === 'discover' ? <CreateRideLink className="button button-outline" /> : <Link className="button button-outline" to="/">{t('browse')}</Link>}
    /> : <div className="ride-list">{query.data.rides.map((ride) => <RideCard key={ride.id} ride={ride} />)}</div>}
    {!query.isError && <Pagination page={page} busy={query.isFetching} hasNext={!!query.data?.next_cursor} previous={() => goToCursor(cursors.slice(0, -1))} next={() => { if (query.data?.next_cursor) goToCursor([...cursors, query.data.next_cursor]) }} />}
  </section>
}

function RideFilters({ filters }: { filters: Filters }) {
  const { t } = useI18n()
  const [params, setParams] = useSearchParams()
  const [draft, setDraft] = useState(filters)
  const [invalidRange, setInvalidRange] = useState(false)
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
    if (!invalid) apply(draft)
  }
  function clear() {
    const empty = { q: '', city: '', from: '', to: '' }
    setDraft(empty)
    setInvalidRange(false)
    apply(empty)
  }
  return <form className="ride-filters" aria-label={t('rideFilters')} onSubmit={submit}>
    <div className="ride-filter-fields">
      <Field label={t('rideSearch')} type="search" value={draft.q} onChange={(event) => setDraft({ ...draft, q: event.target.value })} maxLength={200} placeholder={t('rideSearchPlaceholder')} />
      <CitySelector value={draft.city} onChange={(city) => setDraft({ ...draft, city })} label={t('city')} placeholder={t('allCities')} hint="" maxLength={160} />
      <Field label={t('fromDate')} type="date" value={draft.from} onChange={(event) => setDraft({ ...draft, from: event.target.value })} max={draft.to || undefined} />
      <Field label={t('toDate')} type="date" value={draft.to} onChange={(event) => setDraft({ ...draft, to: event.target.value })} min={draft.from || undefined} />
    </div>
    {invalidRange && <ErrorNotice text={t('rideDateRangeInvalid')} />}
    <div className="ride-filter-actions"><button type="submit" className="button button-dark"><SlidersHorizontal size={17} />{t('applyFilters')}</button><button type="button" className="text-button" onClick={clear}>{t('clearFilters')}</button><span className="ride-filter-note"><Search size={14} />{t('rideDatesHint')}</span></div>
  </form>
}

function RideCard({ ride }: { ride: Ride }) {
  const { t, locale } = useI18n()
  const cities = useCityOptions()
  const date = dateParts(ride.date, locale)
  const state = ride.participation_state
  return <Link to={`/rides/${ride.id}`} className="ride-card"><div className="date-block"><strong>{date.day}</strong><span>{date.month}</span></div><div className="ride-card-copy"><div className="ride-card-top"><span className="city-label"><MapPin size={14} />{cityLabel(ride.city, locale, cities.data?.cities ?? [])}</span>{ride.status === 'cancelled' && <span className="status-pill cancelled">{t('cancelled')}</span>}{ride.can_manage && <span className="status-pill">{t('organizer')}</span>}{state === 'requested' && <span className="status-pill pending">{t('requested')}</span>}{state === 'approved' && !ride.can_manage && <span className="status-pill approved"><Check size={12} />{t('approved')}</span>}</div><h3>{ride.description}</h3><span className="card-policy">{formatCalendarDate(ride.date, locale)}</span></div><span className="card-arrow" aria-label={t('viewRide')}><ArrowUpRight size={22} /></span></Link>
}
