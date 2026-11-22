import { lazy, Suspense, useCallback, useEffect, useMemo, useRef, useState, useSyncExternalStore } from 'react'
import type { FormEvent, ReactNode } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowLeftIcon, ArrowUpRightIcon, BookmarkSimpleIcon, DownloadSimpleIcon, NavigationArrowIcon, PathIcon, PauseIcon, PlusIcon, RecordIcon, TrashIcon } from '@phosphor-icons/react'
import { Link, Navigate, useLocation, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { mutation, request } from '../api'
import { CitySelector } from '../components/CitySelector'
import { MapDiscoverySwitch } from '../components/maps/MapDiscoverySwitch'
import { MapPlanEditor } from '../components/maps/MapPlanEditor'
import type { MapDraft } from '../components/maps/MapPlanEditor'
import { ReportButton } from '../components/Trust'
import { intlLocale, useI18n } from '../i18n'
import { bindRecorderAccount, recorderFor } from '../route-recorder'
import { byteLength, conditionKinds, routeSummary, routeSurfaces, trackDistance } from '../routes'
import type { CollectionsPageData, CommunityRoute, CommunityRouteSummary, ConditionsPageData, RecordedTrack, RouteCollection, RouteCondition, RouteConditionSummary, RoutesPageData, RouteSurface, TracksPageData, TrackSegments } from '../routes'
import { useSession, useSessionGuard } from '../session'
import type { MapConfig, MapPlan, MapPoint } from '../types'
import { EmptyState, ErrorNotice, Field, formatRideDateTime, Loading, Pagination, ShareLink, TextArea, useConfirmation, useOnline } from '../ui'
import './routes.css'

const MapCanvas = lazy(() => import('../components/maps/MapCanvas'))
const surfaceLabel = { unknown: 'routesUnknown', asphalt: 'routesAsphalt', mixed: 'routesMixed', unpaved: 'routesUnpaved' } as const
const conditionLabel = { gravel: 'routesGravel', closure: 'routesClosure', roadworks: 'routesRoadworks', damage: 'routesDamage', other: 'routesOther' } as const
const noPoints: MapPoint[] = []
const conditionState = { recent: 'routesConditionRecent', expired: 'routesConditionExpired', resolved: 'routesConditionResolved' } as const
const durationLimits = ['3600', '7200', '14400', '28800']
const filterKeys = ['q', 'city', 'surface', 'max_duration_seconds'] as const
const latestReportAnchor = 'latest-road-report'
const saveRouteAnchor = 'save-route'

function routeFilters(params: URLSearchParams) {
  const surface = params.get('surface') ?? ''
  const duration = params.get('max_duration_seconds') ?? ''
  return {
    q: params.get('q') ?? '',
    city: params.get('city') ?? '',
    surface: routeSurfaces.includes(surface as RouteSurface) ? surface : '',
    max_duration_seconds: durationLimits.includes(duration) ? duration : '',
  }
}
const listParams = (params: URLSearchParams, tab: string) => {
  const out = new URLSearchParams({ limit: '20', scope: tab === 'mine' ? 'mine' : 'public' })
  for (const [key, value] of Object.entries(routeFilters(params))) if (value) out.set(key, value)
  for (const key of ['after', 'collection']) if (params.get(key)) out.set(key, params.get(key)!)
  return out
}

function useRouteAction() {
  const client = useQueryClient(), capture = useSessionGuard()
  const [busy, setBusy] = useState(false), [error, setError] = useState<unknown>()
  const mounted = useRef(true), pending = useRef(false)
  useEffect(() => { mounted.current = true; return () => { mounted.current = false } }, [])
  async function run<T>(call: () => Promise<T>, done?: (result: T) => void) {
    if (pending.current) return
    pending.current = true
    const current = capture()
    setBusy(true); setError(undefined)
    try {
      const result = await call()
      if (!current()) return
      if (mounted.current) done?.(result)
      void client.invalidateQueries({ predicate: (q) => ['community-routes', 'community-route', 'route-collections', 'recorded-tracks', 'recorded-track', 'route-conditions'].includes(String(q.queryKey[0])) })
    } catch (value) { if (current() && mounted.current) setError(value) }
    finally { pending.current = false; if (current() && mounted.current) setBusy(false) }
  }
  return { busy, error, run }
}
function useArchive() {
  return useQuery({ queryKey: ['route-map-config'], queryFn: ({ signal }) => request<MapConfig>('/places/config', { signal }), staleTime: 60000, retry: false })
}
function RouteMap({ segments, points = noPoints }: { segments: TrackSegments; points?: MapPoint[] }) {
  const config = useArchive(), { t, locale } = useI18n()
  return <div className="route-map-preview">
    {config.isPending ? <Loading /> : config.data?.archive_url ? <Suspense fallback={<Loading />}><MapCanvas points={points} segments={segments} archiveUrl={config.data.archive_url} locale={locale} /></Suspense> : <ErrorNotice text={t('routesMapUnavailable')} retry={() => void config.refetch()} />}
  </div>
}
function RouteStats({ distance, duration, recorded = false }: { distance: number; duration: number; recorded?: boolean }) {
  const { t, locale } = useI18n()
  const minutes = Math.max(1, Math.round(duration / 60))
  return <div className="route-stats"><strong>{t('routesDistance', { distance: new Intl.NumberFormat(intlLocale(locale), { maximumFractionDigits: 1 }).format(distance / 1000) })}</strong><span>{duration > 0 ? t(recorded ? 'routesDurationRecorded' : 'routesDurationEstimated', { minutes }) : t('routesDurationUnavailable')}</span></div>
}
function BackToRoutes() {
  const { t } = useI18n()
  return <Link className="back-link" to="/routes"><ArrowLeftIcon size={17} />{t('routesBack')}</Link>
}
function PageState({ pending, error, retry, children }: { pending: boolean; error?: unknown; retry: () => void; children: ReactNode }) {
  return pending ? <Loading /> : error != null ? <ErrorNotice error={error} retry={retry} /> : children
}

export function RoutesPage() {
  const { user, loading, error, refresh } = useSession()
  const { t } = useI18n(), [params] = useSearchParams()
  const rawTab = params.get('tab') ?? 'public'
  const tab = ['mine', 'collections', 'tracks'].includes(rawTab) ? rawTab : 'public'
  if (loading) return <Loading />
  if (error) return <div className="page-content"><ErrorNotice error={error} retry={() => void refresh()} /></div>
  if (tab !== 'public' && !user) return <Navigate to={`/login?next=${encodeURIComponent('/routes?' + params)}`} replace />
  const city = params.get('city') ?? ''
  return <div className="page-content routes-page" key={user?.id ?? 'guest'}>
    <MapDiscoverySwitch current="routes" city={city} />
    <div className="collection-heading"><div className="page-heading"><h1>{t('routesHeading')}</h1></div><div className="route-actions"><Link className="button button-primary" to="/routes/new"><PlusIcon size={18} />{t('routesNew')}</Link><Link className="button button-outline" to="/routes/record"><RecordIcon size={18} />{t('routesRecord')}</Link></div></div>
    <nav className="routes-tabs" aria-label={t('routesTitle')}>{(['public', 'mine', 'collections', 'tracks'] as const).map((value) => {
      const next = new URLSearchParams(city ? { city } : {})
      if (value !== 'public') next.set('tab', value)
      return <Link key={value} aria-current={tab === value ? 'page' : undefined} to={value === tab ? '/routes' + (params.size ? '?' + params : '') : '/routes' + (next.size ? '?' + next : '')}>{t(({ public: 'routesPublic', mine: 'routesMine', collections: 'routesCollections', tracks: 'routesTracks' } as const)[value])}</Link>
    })}</nav>
    {tab === 'collections' ? <Collections /> : tab === 'tracks' ? <Tracks /> : <RouteList tab={tab} />}
  </div>
}

function RouteList({ tab }: { tab: string }) {
  const { t } = useI18n(), { user } = useSession(), [params, setParams] = useSearchParams()
  const location = useLocation(), navigate = useNavigate()
  const queryParams = listParams(params, tab), after = params.get('after') ?? ''
  const scope = JSON.stringify([user?.id, tab, routeFilters(params), params.get('collection')])
  const saved = (location.state as { routePaging?: { scope: string; cursors: string[] } } | null)?.routePaging
  const wrongActor = !!after && !!saved && saved.scope !== scope
  const cursors = saved?.scope === scope && saved.cursors.at(-1) === after ? saved.cursors : after ? ['', after] : ['']
  const query = useQuery({ queryKey: ['community-routes', user?.id ?? 'guest', queryParams.toString()], enabled: !wrongActor, queryFn: async ({ signal }) => { const result = await request<RoutesPageData>(`/routes?${queryParams}`, { signal }); return { ...result, routes: result.routes.map(routeSummary) } }, meta: { private: true }, retry: false })
  function page(next: string[]) {
    const result = new URLSearchParams(params)
    if (next.at(-1)) result.set('after', next.at(-1)!)
    else result.delete('after')
    void navigate({ pathname: '/routes', search: result.toString() }, { state: { routePaging: { scope, cursors: next } } })
  }
  if (wrongActor) { const result = new URLSearchParams(params); result.delete('after'); return <Navigate to={`/routes?${result}`} replace /> }
  return <section aria-label={t(tab === 'mine' ? 'routesMine' : 'routesPublic')}>
    <RouteSearch key={JSON.stringify(routeFilters(params))} />
    {params.has('collection') && <p className="field-hint">{t('routesCollectionPrivate')}</p>}
    <PageState pending={query.isPending} error={query.error} retry={() => void query.refetch()}>
      {!query.data?.routes.length ? <EmptyState title={t('routesEmpty')} text={t('routesEmptyText')} action={<Link className="button button-outline" to="/routes/new">{t('routesNew')}</Link>} /> : <div className="route-grid">{query.data.routes.map((route) => <RouteCard key={route.id} route={route} showReport={tab === 'public' && !params.has('collection')} collection={params.get('collection') ?? undefined} />)}</div>}
    </PageState>
    {query.isError && after && <button className="text-button" onClick={() => { const next = new URLSearchParams(params); next.delete('after'); setParams(next) }}>{t('routesFirstPage')}</button>}
    {!query.isError && <Pagination page={cursors.length - 1} hasNext={!!query.data?.next_cursor} busy={query.isFetching} next={() => { if (query.data?.next_cursor) page([...cursors, query.data.next_cursor]) }} previous={() => page(cursors.slice(0, -1))} />}
  </section>
}
function RouteSearch() {
  const { t } = useI18n(), [params, setParams] = useSearchParams()
  const [filters, setFilters] = useState(() => routeFilters(params))
  function update(key: typeof filterKeys[number], value: string) { setFilters((current) => ({ ...current, [key]: value })) }
  function apply(event: FormEvent) {
    event.preventDefault()
    const next = new URLSearchParams(params)
    next.delete('after')
    for (const key of filterKeys) {
      const value = filters[key].trim()
      if (value) next.set(key, value)
      else next.delete(key)
    }
    setParams(next)
  }
  function clear() {
    const next = new URLSearchParams(params)
    for (const key of [...filterKeys, 'after']) next.delete(key)
    setFilters(routeFilters(next))
    setParams(next)
  }
  return <form className="routes-filters" onSubmit={apply}>
    <Field type="search" label={t('routesSearch')} value={filters.q} onChange={(e) => update('q', e.target.value)} maxLength={160} />
    <CitySelector value={filters.city} onChange={(value) => update('city', value)} label={t('routesCity')} hint="" />
    <Field label={t('routesRidingTime')}><select value={filters.max_duration_seconds} onChange={(e) => update('max_duration_seconds', e.target.value)}>
      <option value="">{t('routesAnyDuration')}</option>
      {durationLimits.map((seconds) => <option key={seconds} value={seconds}>{t('routesDurationLimit', { hours: Number(seconds) / 3600 })}</option>)}
    </select></Field>
    <Field label={t('routesSurface')}><select value={filters.surface} onChange={(e) => update('surface', e.target.value)}>
      <option value="">{t('routesAnySurface')}</option>
      {routeSurfaces.map((surface) => <option key={surface} value={surface}>{t(surfaceLabel[surface])}</option>)}
    </select></Field>
    <div className="route-actions routes-filter-actions"><button className="button button-outline" type="submit" aria-label={t('applyFilters')}>{t('routesApply')}</button><button className="text-button" type="button" onClick={clear} aria-label={t('clearFilters')}>{t('routesClear')}</button></div>
  </form>
}

function RouteThumbnail({ segments }: { segments?: TrackSegments }) {
  if (!segments?.length) return null
  return <svg aria-hidden="true" focusable="false" className="route-thumbnail" viewBox="-0.1 -0.1 1.2 1.2">
    {segments.map((points, index) => <polyline key={index} points={points.map(([x, y]) => `${x},${y}`).join(' ')} vectorEffect="non-scaling-stroke" />)}
  </svg>
}

function ActiveCondition({ condition, path }: { condition?: RouteConditionSummary; path?: string }) {
  const { t, locale } = useI18n()
  const [openedAt] = useState(Date.now)
  if (!condition || !Number.isFinite(Date.parse(condition.observed_at)) || !(Date.parse(condition.expires_at) > openedAt)) return <p className="route-latest-report">{t('routesNoRecentReports')}</p>
  const date = new Intl.DateTimeFormat(intlLocale(locale), { day: 'numeric', month: 'short', year: 'numeric', timeZone: 'Asia/Almaty' }).format(new Date(condition.observed_at))
  const label = t(conditionLabel[condition.kind])
  return <div className="route-latest-report">
    <p>{path ? <Link to={path}>{label}</Link> : <strong>{label}</strong>} <span>· <time dateTime={condition.observed_at}>{date}</time> · {condition.author_name}</span></p>
    {!path && condition.description && <p className="route-description">{condition.description}</p>}
  </div>
}

function RouteCard({ route, collection, showReport }: { route: CommunityRouteSummary; collection?: string; showReport: boolean }) {
  const { t } = useI18n(), action = useRouteAction(), online = useOnline()
  const path = `/routes/${route.id}`
  return <article className="surface route-card">
    <div className="route-card-summary">
      <div>
        <div className="route-card-top"><PathIcon size={20} aria-hidden="true" /><span>{t(route.kind === 'recorded' ? 'routesRecorded' : 'routesPlanned')}</span>{!route.published && <span className="status-pill">{t('routesPrivate')}</span>}</div>
        <h2><Link to={path}>{route.title}</Link></h2>
        <p className="route-card-city">{route.city} · {t(surfaceLabel[route.surface] ?? 'routesUnknown')}</p>
        <RouteStats distance={route.distance_meters} duration={route.duration_seconds} recorded={route.kind === 'recorded'} />
      </div>
      <RouteThumbnail segments={route.preview_segments} />
    </div>
    {route.published && showReport && <ActiveCondition condition={route.latest_condition} path={`${path}#${latestReportAnchor}`} />}
    <p className="route-card-description">{route.description}</p>
    <p className="field-hint">{t('routesBy', { name: route.owner_name })}</p>
    <div className="route-actions"><Link className="text-button" to={path}>{t('routesOpen')}<ArrowUpRightIcon size={18} /></Link>{collection && <button className="text-button" disabled={!online || action.busy} onClick={() => void action.run(() => mutation(`/route-collections/${collection}/routes/${route.id}`, 'DELETE'))}>{t('routesCollectionRemove')}</button>}</div>
    {action.error != null && <ErrorNotice error={action.error} />}
  </article>
}

function Collections() {
  const { user } = useSession(), { t } = useI18n(), action = useRouteAction(), online = useOnline()
  const [after, setAfter] = useState(''), [name, setName] = useState('')
  const query = useQuery({ queryKey: ['route-collections', user?.id, after], queryFn: ({ signal }) => request<CollectionsPageData>(`/route-collections?limit=50${after ? `&after=${encodeURIComponent(after)}` : ''}`, { signal }), meta: { private: true } })
  function create(e: FormEvent) { e.preventDefault(); if (!name.trim() || byteLength(name) > 100) return; void action.run(() => mutation('/route-collections', 'POST', { name: name.trim() }), () => setName('')) }
  return <section><p className="field-hint">{t('routesCollectionPrivate')}</p><form className="surface routes-inline-form" onSubmit={create}><Field label={t('routesCollectionName')} hint={byteLength(name) > 100 ? t('errorValidation') : undefined} value={name} onChange={(e) => setName(e.target.value)} required maxLength={100} /><button className="button button-primary" disabled={action.busy || !online || !name.trim() || byteLength(name) > 100}>{t('routesCollectionCreate')}</button></form>{action.error != null && <ErrorNotice error={action.error} />}<PageState pending={query.isPending} error={query.error} retry={() => void query.refetch()}>{!query.data?.collections.length ? <EmptyState title={t('routesCollectionEmpty')} text={t('routesCollectionEmptyText')} /> : <div className="route-grid">{query.data.collections.map((collection) => <CollectionCard key={collection.id + ':' + collection.version} collection={collection} />)}</div>}</PageState><div className="route-actions">{after && <button className="text-button" onClick={() => setAfter('')}>{t('routesFirstPage')}</button>}{query.data?.next_cursor && <button className="button button-outline" onClick={() => setAfter(query.data.next_cursor)}>{t('nextPage')}</button>}</div></section>
}
function CollectionCard({ collection }: { collection: RouteCollection }) {
  const { t } = useI18n(), [name, setName] = useState(collection.name), [editing, setEditing] = useState(false), action = useRouteAction(), confirmation = useConfirmation(), online = useOnline()
  async function remove() { if (await confirmation.confirm({ action: t('routesCollectionDelete'), message: t('routesCollectionDeleteNote'), destructive: true })) await action.run(() => mutation(`/route-collections/${collection.id}`, 'DELETE', { version: collection.version })) }
  return <article className="surface route-card"><BookmarkSimpleIcon size={24} /><h2><Link to={`/routes?collection=${collection.id}`}>{collection.name}</Link></h2>{Number.isFinite(collection.route_count) && <p>{t('routesCollectionCount', { count: collection.route_count })}</p>}{editing && <form className="routes-stack" onSubmit={(e) => { e.preventDefault(); if (name.trim() && byteLength(name) <= 100) void action.run(() => mutation(`/route-collections/${collection.id}`, 'PUT', { name: name.trim(), version: collection.version }), () => setEditing(false)) }}><Field label={t('routesCollectionName')} hint={byteLength(name) > 100 ? t('errorValidation') : undefined} value={name} maxLength={100} onChange={(e) => setName(e.target.value)} required /><button className="button button-outline" disabled={action.busy || !online || !name.trim() || byteLength(name) > 100}>{t('routesCollectionRename')}</button><button type="button" className="text-button" onClick={() => setEditing(false)}>{t('cancel')}</button></form>}<div className="route-actions"><button className="text-button" disabled={action.busy || !online} onClick={() => setEditing(!editing)}>{t('routesCollectionRename')}</button><button className="text-button" disabled={action.busy || !online} onClick={() => void remove()}>{t('routesCollectionDelete')}</button></div>{action.error != null && <ErrorNotice error={action.error} />}{confirmation.dialog}</article>
}
function Tracks() {
  const { user } = useSession(), { t } = useI18n(), [after, setAfter] = useState('')
  const query = useQuery({ queryKey: ['recorded-tracks', user?.id, after], queryFn: ({ signal }) => request<TracksPageData>(`/tracks?limit=20${after ? `&after=${encodeURIComponent(after)}` : ''}`, { signal }), meta: { private: true } })
  return <section><PageState pending={query.isPending} error={query.error} retry={() => void query.refetch()}>{!query.data?.tracks.length ? <EmptyState title={t('routesTrackEmpty')} text={t('routesTrackEmptyText')} action={<Link className="button button-primary" to="/routes/record">{t('routesRecord')}</Link>} /> : <div className="route-grid">{query.data.tracks.map((track) => <TrackCard key={track.id} track={track} />)}</div>}</PageState><div className="route-actions">{after && <button className="text-button" onClick={() => setAfter('')}>{t('routesFirstPage')}</button>}{query.data?.next_cursor && <button className="button button-outline" onClick={() => setAfter(query.data.next_cursor)}>{t('nextPage')}</button>}</div></section>
}
function TrackCard({ track }: { track: RecordedTrack }) {
  const { t, locale } = useI18n(), action = useRouteAction(), confirmation = useConfirmation(), online = useOnline()
  async function remove() { if (await confirmation.confirm({ action: t('routesTrackDelete'), message: t('routesTrackDeleteNote'), destructive: true })) await action.run(() => mutation(`/tracks/${track.id}`, 'DELETE', { version: track.version })) }
  return <article className="surface route-card"><span className="status-pill">{t('routesPrivate')}</span><h2>{track.name}</h2><p className="field-hint">{formatRideDateTime(track.created_at, 'Asia/Almaty', locale)}</p><RouteStats distance={track.distance_meters} duration={track.duration_seconds} recorded /><div className="route-actions"><Link className="text-button" to={`/routes/new?track=${track.id}`}>{t('routesFromTrack')}<ArrowUpRightIcon size={18} /></Link><button className="text-button" disabled={action.busy || !online} onClick={() => void remove()}>{t('routesTrackDelete')}</button></div>{action.error != null && <ErrorNotice error={action.error} />}{confirmation.dialog}</article>
}

export function RouteDetailPage() {
  const { routeId } = useParams(), { user, loading, error } = useSession()
  const query = useQuery({ queryKey: ['community-route', routeId, user?.id ?? 'guest'], enabled: !loading && !error && !!routeId, queryFn: ({ signal }) => request<{ route: CommunityRoute }>(`/routes/${routeId}`, { signal }), meta: { private: true }, retry: false })
  return <div className="page-content routes-page"><BackToRoutes /><PageState pending={loading || query.isPending && !error} error={error ?? query.error} retry={() => void query.refetch()}>{query.data && <RouteDetails key={query.data.route.id + ':' + user?.id} route={query.data.route} />}</PageState></div>
}
function RouteDetails({ route }: { route: CommunityRoute }) {
  const { user } = useSession(), { t, locale } = useI18n(), action = useRouteAction(), confirmation = useConfirmation(), navigate = useNavigate(), online = useOnline()
  const { hash } = useLocation()
  const latestReport = useRef<HTMLElement>(null)
  const saveCollection = useRef<HTMLElement>(null)
  useEffect(() => {
    const target = hash === '#' + latestReportAnchor ? latestReport.current : hash === '#' + saveRouteAnchor ? saveCollection.current : null
    if (!target) return
    target.focus({ preventScroll: true })
    target.scrollIntoView?.()
  }, [hash])
  async function remove() { if (await confirmation.confirm({ action: t('routesDelete'), message: t('routesDeleteNote'), destructive: true })) await action.run(() => mutation(`/routes/${route.id}`, 'DELETE', { version: route.version }), () => void navigate('/routes?tab=mine')) }
  const path = `/routes/${encodeURIComponent(route.id)}`
  const savePath = `#${saveRouteAnchor}`
  const [firstPoint] = route.points ?? []
  const start = firstPoint ? [firstPoint.lon, firstPoint.lat] : route.segments?.find((segment) => segment.length)?.[0]
  const stops = route.points?.slice(1, -1) ?? []
  const planned = route.kind === 'planned'
  const publicRoute = route.published && route.publicly_visible === true
  return <>
    <div className="routes-heading">
      <div className="page-heading">
        <span className="route-kind">{t(planned ? 'routesPlanned' : 'routesRecorded')} · {t(publicRoute ? 'routesPublished' : route.published ? 'routesUnavailablePublic' : 'routesPrivate')}</span>
        <h1>{route.title}</h1>
        <p>{route.city} · {t(surfaceLabel[route.surface] ?? 'routesUnknown')}</p>
        <RouteStats distance={route.distance_meters} duration={route.duration_seconds} recorded={!planned} />
      </div>
      {user?.id === route.owner_id && <div className="route-actions"><Link className="button button-outline" to={path + '/edit'}>{t('routesEdit')}</Link><button className="text-button" disabled={action.busy || !online} onClick={() => void remove()}>{t('routesDelete')}</button></div>}
    </div>
    {publicRoute && <>
      <div className="route-actions route-detail-actions">
        {planned && (route.points?.length ?? 0) >= 2 && <Link className="button button-primary" to={`/rides/new?route=${encodeURIComponent(route.id)}`}><PlusIcon size={18} />{t('routesCreateRide')}</Link>}
        {user ? <Link className="button button-outline" to={savePath}><BookmarkSimpleIcon size={18} />{t('routesCollectionSave')}</Link> : <Link className="button button-outline" to={`/login?next=${encodeURIComponent(path + savePath)}`}><BookmarkSimpleIcon size={18} />{t('routesCollectionSave')}</Link>}
        {start && <a className="button button-outline" href={`https://www.google.com/maps/dir/?api=1&destination=${encodeURIComponent(`${start[1]},${start[0]}`)}&travelmode=driving`} target="_blank" rel="noopener noreferrer"><NavigationArrowIcon size={18} />{t('routesNavigateToStart')}</a>}
        <a className="button button-outline" href={`/api/v1${path}/gpx`} download><DownloadSimpleIcon size={18} />{t('routesDownloadGPX')}</a>
      </div>
      <section id={latestReportAnchor} ref={latestReport} tabIndex={-1} className="surface route-panel route-report-brief" aria-labelledby="route-report-brief-heading">
        <h2 id="route-report-brief-heading">{t('routesLatestReport')}</h2>
        <ActiveCondition condition={route.latest_condition} />
        <a className="text-button" href="#route-conditions">{t('routesAllReports')}</a>
      </section>
    </>}
    <div className={`route-detail-layout ${!user && publicRoute ? 'route-detail-full' : ''}`}>
      <section className="routes-stack">
        <RouteMap segments={route.segments ?? []} points={route.points ?? []} />
        {stops.length > 0 && <section className="surface route-panel route-stops"><h2>{t('routesStops')}</h2><ol>{stops.map((point, index) => <li key={index}>{point.label}</li>)}</ol></section>}
        <div className="surface route-panel">
          <p className="route-description">{route.description}</p>
          <p className="field-hint">{t('routesBy', { name: route.owner_name })}</p>
          <p className="field-hint">{t('routesUpdated', { date: formatRideDateTime(route.updated_at, 'Asia/Almaty', locale) })}</p>
          {publicRoute && <ShareLink path={path} />}
          <ReportButton targetType="route" targetId={route.id} />
        </div>
      </section>
      {user && publicRoute && <aside id={saveRouteAnchor} ref={saveCollection} tabIndex={-1} aria-label={t('routesCollectionSave')}><SaveToCollection routeId={route.id} /></aside>}
      {!publicRoute && <p className="surface route-panel field-hint">{t(route.published ? 'routesUnavailablePublicNote' : 'routesDraftNote')}</p>}
    </div>
    {publicRoute && <Conditions routeId={route.id} />}
    {action.error != null && <ErrorNotice error={action.error} />}
    {confirmation.dialog}
  </>
}
function SaveToCollection({ routeId }: { routeId: string }) {
  const { user } = useSession(), { t } = useI18n(), [id, setId] = useState(''), action = useRouteAction(), online = useOnline()
  const query = useQuery({ queryKey: ['route-collections', user?.id, 'save'], queryFn: ({ signal }) => request<CollectionsPageData>('/route-collections?limit=50', { signal }), meta: { private: true } })
  const [saved, setSaved] = useState(false)
  return <form className="surface route-panel routes-stack" onSubmit={(e) => { e.preventDefault(); if (id) void action.run(() => mutation(`/route-collections/${id}/routes/${routeId}`, 'PUT'), () => setSaved(true)) }}><h2>{t('routesCollectionSave')}</h2>{query.isPending ? <Loading /> : query.isError ? <ErrorNotice error={query.error} retry={() => void query.refetch()} /> : query.data.collections.length ? <><Field label={t('routesCollections')}><select value={id} required onChange={(e) => { setId(e.target.value); setSaved(false) }}><option value="">{t('routesCollectionSave')}</option>{query.data.collections.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}</select></Field><button className="button button-outline" disabled={action.busy || !online || !id}>{t('routesCollectionAdd')}</button></> : <Link to="/routes?tab=collections">{t('routesCollectionCreate')}</Link>}{saved && <p className="success-message" role="status">{t('saved')}</p>}{action.error != null && <ErrorNotice error={action.error} />}</form>
}

export function RouteFormPage({ edit = false }: { edit?: boolean }) {
  const { routeId } = useParams(), [params] = useSearchParams(), { user } = useSession(), { t } = useI18n()
  const trackId = !edit ? params.get('track') : null
  const route = useQuery({ queryKey: ['community-route', routeId, user?.id], enabled: edit && !!routeId, queryFn: ({ signal }) => request<{ route: CommunityRoute }>(`/routes/${routeId}`, { signal }), meta: { private: true }, refetchOnWindowFocus: false })
  const track = useQuery({ queryKey: ['recorded-track', trackId, user?.id], enabled: !!trackId, queryFn: ({ signal }) => request<{ track: RecordedTrack }>(`/tracks/${trackId}`, { signal }), meta: { private: true }, refetchOnWindowFocus: false })
  if (edit && route.isPending || trackId && track.isPending) return <Loading />
  if (route.isError && edit || track.isError && trackId) return <div className="page-content"><ErrorNotice error={route.error ?? track.error} retry={() => { void route.refetch(); void track.refetch() }} /></div>
  if (edit && route.data?.route.owner_id !== user?.id) return <ErrorNotice text={t('noAccess')} />
  return <div className="page-content routes-page"><BackToRoutes /><RouteForm key={`${user?.id}:${route.data?.route.id}:${route.data?.route.version}:${trackId}`} route={edit ? route.data?.route : undefined} track={track.data?.track} /></div>
}
function RouteForm({ route, track }: { route?: CommunityRoute; track?: RecordedTrack }) {
  const { t } = useI18n(), { user } = useSession(), action = useRouteAction(), online = useOnline(), navigate = useNavigate()
  const [title, setTitle] = useState(route?.title ?? track?.name ?? ''), [city, setCity] = useState(route?.city ?? user?.profile.city ?? ''), [description, setDescription] = useState(route?.description ?? ''), [surface, setSurface] = useState<RouteSurface>(route?.surface ?? 'unknown')
  const [published, setPublished] = useState(route?.published ?? false), [consent, setConsent] = useState(false), [validation, setValidation] = useState('')
  const [meeting, setMeeting] = useState(route?.points?.[0]?.label ?? ''), [destination, setDestination] = useState(route?.points?.at(-1)?.label ?? '')
  const [draft, setDraft] = useState<MapDraft>({ points: route?.points ?? [], valid: true })
  const changeDraft = useCallback((value: MapDraft) => { setDraft(value); setConsent(false) }, [])
  const recorded = !!track || route?.kind === 'recorded'
  const initialPlan = useMemo<MapPlan | undefined>(() => route && route.kind === 'planned' ? { points: route.points ?? [], distance_meters: route.distance_meters, duration_seconds: route.duration_seconds, geometry: route.segments?.[0] ? { type: 'LineString', coordinates: route.segments[0] } : undefined } : undefined, [route])
  function submit(e: FormEvent) {
    e.preventDefault(); setValidation('')
    if (!title.trim() || !city.trim() || byteLength(title) > 200 || byteLength(city) > 160 || byteLength(description) > 4000) { setValidation(t('requiredFields')); return }
    if (!recorded && (!draft.valid || draft.points.length < 2)) { setValidation(t('routesMapRequired')); return }
    if (published && !consent) { setValidation(t('routesConsent')); return }
    const input = { title: title.trim(), city: city.trim(), description: description.trim(), surface, published, publish_consent: published && consent, version: route?.version ?? 0, ...(!recorded ? { map_points: draft.points } : track ? { track_id: track.id } : {}) }
    void action.run(() => mutation<{ route: CommunityRoute }>(route ? `/routes/${route.id}` : '/routes', route ? 'PUT' : 'POST', input), (result) => void navigate(`/routes/${result.route.id}`))
  }
  return <><div className="page-heading"><h1>{t(route ? 'routesEdit' : recorded ? 'routesFromTrack' : 'routesNew')}</h1><p>{t('routesDraftNote')}</p></div><form className="surface route-form" onSubmit={submit}><fieldset disabled={action.busy}><Field label={t('routesName')} value={title} onChange={(e) => setTitle(e.target.value)} maxLength={200} required /><CitySelector value={city} onChange={setCity} required /><TextArea label={t('routesDescription')} value={description} onChange={setDescription} maxLength={4000} /><Field label={t('routesSurface')}><select value={surface} onChange={(e) => setSurface(e.target.value as RouteSurface)}>{routeSurfaces.map((s) => <option key={s} value={s}>{t(surfaceLabel[s])}</option>)}</select></Field>{recorded ? <><p className="field-hint">{t('routesTrackSource')}</p><RouteMap segments={track?.segments ?? route?.segments ?? []} /></> : <MapPlanEditor initialPlan={initialPlan} meeting={meeting} destination={destination} onMeeting={setMeeting} onDestination={setDestination} onChange={changeDraft} disabled={action.busy} privacyNote={t('routesDraftNote')} />}<label className="route-checkbox"><input type="checkbox" checked={published} onChange={(e) => { setPublished(e.target.checked); setConsent(false) }} /><span>{t('routesPublish')}</span></label>{published && <div className="route-publication"><p>{t('routesPublicNote')}</p><label className="route-checkbox"><input type="checkbox" checked={consent} onChange={(e) => setConsent(e.target.checked)} required /><span>{t('routesConsent')}</span></label></div>}{validation && <ErrorNotice text={validation} />}{action.error != null && <ErrorNotice error={action.error} />}<div className="route-actions"><button className="button button-primary" disabled={action.busy || !online}>{t(action.busy ? 'saving' : 'routesSave')}</button><Link className="text-button" to={route ? `/routes/${route.id}` : '/routes'}>{t('cancel')}</Link></div></fieldset></form></>
}

export function TrackRecorderPage() {
  const { user } = useSession()
  if (!user) return null
  return <Recorder key={user.id} actor={user.id} />
}
function Recorder({ actor }: { actor: string }) {
  const recorder = useMemo(() => recorderFor(actor), [actor]), snapshot = useSyncExternalStore(recorder.subscribe, recorder.snapshot)
  const { t } = useI18n(), action = useRouteAction(), confirmation = useConfirmation(), online = useOnline(), navigate = useNavigate()
  const [validation, setValidation] = useState('')
  useEffect(() => {
    bindRecorderAccount(actor)
    const interval = window.setInterval(recorder.tick, 1000)
    const leave = (event: BeforeUnloadEvent) => { if (recorder.snapshot().status === 'recording') { event.preventDefault(); event.returnValue = '' } }
    window.addEventListener('beforeunload', leave)
    return () => { window.clearInterval(interval); window.removeEventListener('beforeunload', leave); recorder.pause() }
  }, [actor, recorder])
  const count = snapshot.segments.reduce((n, segment) => n + segment.length, 0)
  const errorLabels = { permission: 'routesGpsPermission', unavailable: 'routesGpsUnavailable', timeout: 'routesGpsTimeout', outside: 'routesGpsOutside', limit: 'routesGpsLimit', storage: 'routesGpsStorage' } as const
  async function discard() { if (await confirmation.confirm({ action: t('routesDiscard'), message: t('routesDiscardNote'), destructive: true })) { recorder.clear(); setValidation('') } }
  function save() {
    const input = recorder.saveInput()
    const revision = recorder.revision()
    if (!input.name || byteLength(input.name) > 200) { setValidation(t('requiredFields')); return }
    if (!input.segments.length) { setValidation(t('routesNeedPoints')); return }
    setValidation('')
    void action.run(() => mutation<{ track: RecordedTrack }>('/tracks', 'POST', input), () => { if (recorder.clearSaved(revision)) void navigate('/routes?tab=tracks', { state: { saved: true } }) })
  }
  const stale = snapshot.status === 'recording' && snapshot.last_fix_at > 0 && snapshot.checked_at - snapshot.last_fix_at > 15000
  return <div className="page-content routes-page"><BackToRoutes /><div className="page-heading"><h1>{t('routesRecordHeading')}</h1><p>{t('routesRecordPrivacy')}</p></div><div className="surface route-recorder routes-stack"><p className="route-foreground-note">{t('routesForeground')}</p><Field label={t('routesTrackName')} required maxLength={200} value={snapshot.name} disabled={action.busy} onChange={(e) => recorder.name(e.target.value)} /><div className="recorder-status" role="status"><span className={snapshot.status === 'recording' ? 'recorder-active' : ''}><RecordIcon size={20} />{t(snapshot.status === 'recording' ? 'routesRecording' : snapshot.status === 'paused' ? 'routesPaused' : 'routesReady')}</span><span>{t('routesPointCount', { count })}</span></div><RouteStats distance={trackDistance(snapshot.segments)} duration={snapshot.duration_seconds} />{snapshot.status === 'paused' && count > 0 && <p className="field-hint">{t('routesSavedDraft')}</p>}{snapshot.status === 'recording' && !snapshot.last_fix_at && <p role="status">{t('routesWaiting')}</p>}{stale && <p className="route-foreground-note" role="status">{t('routesStale')}</p>}{snapshot.error && <ErrorNotice text={t(errorLabels[snapshot.error])} />}{count > 0 && <RouteMap segments={snapshot.segments} />}<div className="route-actions">{snapshot.status === 'recording' ? <button className="button button-outline" onClick={recorder.pause} disabled={action.busy}><PauseIcon size={18} />{t('routesPause')}</button> : <button className="button button-primary" disabled={action.busy || snapshot.error === 'limit'} onClick={recorder.start}><RecordIcon size={18} />{t(snapshot.status === 'idle' ? 'routesStart' : 'routesResume')}</button>}<button className="button button-outline" disabled={action.busy || !online || count < 2} onClick={save}>{t(action.busy ? 'saving' : 'routesTrackSave')}</button>{snapshot.status !== 'idle' && <button className="text-button" onClick={() => void discard()} disabled={action.busy}><TrashIcon size={16} />{t('routesDiscard')}</button>}</div>{validation && <ErrorNotice text={validation} />}{action.error != null && <><ErrorNotice error={action.error} /><p className="field-hint">{t('routesTrackSaveRetry')}</p></>}{confirmation.dialog}</div></div>
}

function Conditions({ routeId }: { routeId: string }) {
  const { user } = useSession(), { t } = useI18n(), [after, setAfter] = useState(''), [editing, setEditing] = useState<RouteCondition | null | undefined>()
  const query = useQuery({ queryKey: ['route-conditions', routeId, user?.id, after], queryFn: ({ signal }) => request<ConditionsPageData>(`/routes/${routeId}/conditions?limit=20${after ? `&after=${encodeURIComponent(after)}` : ''}`, { signal }), meta: { private: true }, retry: false })
  return <section id="route-conditions" className="routes-conditions"><div className="section-header"><h2>{t('routesConditions')}</h2>{user && editing === undefined && <button className="button button-outline" onClick={() => setEditing(null)}><PlusIcon size={17} />{t('routesConditionAdd')}</button>}</div><p className="field-hint">{t('routesConditionsNote')}</p>{editing !== undefined && <ConditionForm key={editing?.id ?? 'new'} routeId={routeId} condition={editing ?? undefined} close={() => setEditing(undefined)} />}<PageState pending={query.isPending} error={query.error} retry={() => void query.refetch()}>{!query.data?.conditions.length ? <p className="surface route-panel">{t('routesConditionsEmpty')}</p> : <div className="routes-stack">{query.data.conditions.map((condition) => <ConditionCard key={condition.id + ':' + condition.version} condition={condition} edit={() => setEditing(condition)} />)}</div>}</PageState><div className="route-actions">{after && <button className="text-button" onClick={() => setAfter('')}>{t('routesFirstPage')}</button>}{query.data?.next_cursor && <button className="button button-outline" onClick={() => setAfter(query.data.next_cursor)}>{t('nextPage')}</button>}</div></section>
}
function ConditionCard({ condition, edit }: { condition: RouteCondition; edit: () => void }) {
  const { user } = useSession(), { t, locale } = useI18n(), action = useRouteAction(), confirmation = useConfirmation(), online = useOnline()
  async function remove() { if (await confirmation.confirm({ action: t('routesConditionDelete'), message: t('routesConditionDeleteNote'), destructive: true })) await action.run(() => mutation(`/routes/${condition.route_id}/conditions/${condition.id}`, 'DELETE', { version: condition.version })) }
  return <article className={`surface route-panel route-condition route-condition-${condition.state}`}><div className="route-actions"><h3>{t(conditionLabel[condition.kind])}</h3><span className="status-pill">{t(conditionState[condition.state])}</span></div><p className="route-description">{condition.description}</p><p className="field-hint">{condition.author_name}</p><p className="field-hint">{t('routesConditionDates', { observed: formatRideDateTime(condition.observed_at, 'Asia/Almaty', locale), updated: formatRideDateTime(condition.updated_at, 'Asia/Almaty', locale) })}</p><p className="field-hint">{t('routesExpires', { date: formatRideDateTime(condition.expires_at, 'Asia/Almaty', locale) })}</p><div className="route-actions">{user?.id === condition.author_id && <><button className="text-button" onClick={edit} disabled={action.busy || !online}>{t('routesConditionEdit')}</button><button className="text-button" onClick={() => void remove()} disabled={action.busy || !online}>{t('routesConditionDelete')}</button></>}<ReportButton targetType="route_condition" targetId={condition.id} /></div>{action.error != null && <ErrorNotice error={action.error} />}{confirmation.dialog}</article>
}
function localDate(value?: string) {
  const date = value ? new Date(value) : new Date()
  return new Date(date.getTime() - date.getTimezoneOffset() * 60000).toISOString().slice(0, 16)
}
function ConditionForm({ routeId, condition, close }: { routeId: string; condition?: RouteCondition; close: () => void }) {
  const { t } = useI18n(), action = useRouteAction(), online = useOnline()
  const [kind, setKind] = useState(condition?.kind ?? 'other'), [description, setDescription] = useState(condition?.description ?? ''), [observed, setObserved] = useState(localDate(condition?.observed_at)), [resolved, setResolved] = useState(condition?.resolved ?? false), [validation, setValidation] = useState('')
  function submit(event: FormEvent) { event.preventDefault(); setValidation(''); const instant = new Date(observed); if (!description.trim() || byteLength(description) > 2000 || Number.isNaN(instant.getTime()) || instant.getTime() > Date.now()) { setValidation(t('errorValidation')); return }; void action.run(() => mutation(condition ? `/routes/${routeId}/conditions/${condition.id}` : `/routes/${routeId}/conditions`, condition ? 'PUT' : 'POST', { kind, description: description.trim(), observed_at: instant.toISOString(), resolved, version: condition?.version ?? 0 }), close) }
  return <form className="surface route-panel routes-stack" onSubmit={submit}><h3>{t(condition ? 'routesConditionEdit' : 'routesConditionAdd')}</h3><fieldset disabled={action.busy}><Field label={t('routesConditionKind')}><select value={kind} onChange={(event) => setKind(event.target.value as typeof kind)}>{conditionKinds.map((value) => <option key={value} value={value}>{t(conditionLabel[value])}</option>)}</select></Field><TextArea label={t('routesConditionText')} value={description} onChange={setDescription} maxLength={2000} required /><Field label={t('routesObserved')} type="datetime-local" required value={observed} max={localDate()} onChange={(e) => setObserved(e.target.value)} />{condition && <label className="route-checkbox"><input type="checkbox" checked={resolved} onChange={(e) => setResolved(e.target.checked)} /><span>{t('routesConditionResolved')}</span></label>}{validation && <ErrorNotice text={validation} />}{action.error != null && <ErrorNotice error={action.error} />}<div className="route-actions"><button className="button button-primary" disabled={action.busy || !online}>{t(action.busy ? 'saving' : 'routesConditionSave')}</button><button type="button" className="text-button" onClick={close} disabled={action.busy}>{t('cancel')}</button></div></fieldset></form>
}
