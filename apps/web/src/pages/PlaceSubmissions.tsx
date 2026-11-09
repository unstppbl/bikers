import { lazy, Suspense, useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import { ArrowLeftIcon, CheckCircleIcon } from '@phosphor-icons/react'
import { ApiError, request } from '../api'
import { intlLocale, useI18n } from '../i18n'
import { useSession } from '../session'
import { useSessionTask } from '../useSessionTask'
import { categoryLabels, kindLabels, placeCategories, placeInput, region, validPlace, validPoint, validReason } from '../places'
import { ErrorNotice, Field, Loading, Pagination, TextArea, useOnline } from '../ui'
import { LocationField, useMapConfig } from '../components/maps/MapPlanEditor'
import { CitySelector } from '../components/CitySelector'
import { PlaceInformation } from '../components/places/shared'
import { usePlacesConfig } from './Places'
import type { MapPlace, MapPoint, Place, PlaceInput, PlaceSubmission, PlaceSubmissionKind } from '../types'
import './places.css'
const MapCanvas = lazy(() => import('../components/maps/MapCanvas'))
const statusLabels = { pending: 'placesPending', approved: 'placesApproved', rejected: 'placesRejected' } as const
const blank: PlaceInput = { name: '', category: 'fuel', lat: NaN, lon: NaN, city: '', address: '', description: '', website: '', phone: '', opening_hours: '' }

export function PlaceSubmissionPage() {
  const { t } = useI18n(), { user } = useSession(), { placeId } = useParams(), [params] = useSearchParams()
  const [revision, setRevision] = useState(0)
  const kind: PlaceSubmissionKind = !placeId ? 'new' : params.get('kind') === 'closed' ? 'closed' : 'correction'
  const query = useQuery({ queryKey: ['places', 'detail', placeId], queryFn: ({ signal }) => request<{ place: Place }>('/places/' + encodeURIComponent(placeId!), { signal }), enabled: !!placeId, meta: { private: false }, retry: false, refetchOnWindowFocus: false, refetchOnReconnect: false })
  return <div className="page-content"><Link className="back-link" to="/places"><ArrowLeftIcon size={16} />{t('placesBack')}</Link><h1>{t(kindLabels[kind])}</h1>{placeId && query.isPending ? <Loading /> : placeId && query.isError ? <ErrorNotice error={query.error} retry={() => void query.refetch()} /> : <SubmissionForm key={user?.id + ':' + kind + ':' + placeId + ':' + revision} kind={kind} current={query.data?.place} refresh={() => { void query.refetch().then((result) => { if (!result.isError) setRevision((v) => v + 1) }) }} />}</div>
}
function SubmissionForm({ kind, current: latest, refresh }: { kind: PlaceSubmissionKind; current?: Place; refresh: () => void }) {
  const { t, locale } = useI18n(), start = useSessionTask(), online = useOnline(), client = useQueryClient(), config = usePlacesConfig(), geo = useMapConfig()
  const [current] = useState(latest)
  const [place, setPlace] = useState<PlaceInput>(current ? placeInput(current) : blank), [reason, setReason] = useState('')
  const [latText, setLatText] = useState(current ? String(current.lat) : ''), [lonText, setLonText] = useState(current ? String(current.lon) : '')
  const [error, setError] = useState<unknown>(), [invalid, setInvalid] = useState(false), [busy, setBusy] = useState(false), [sent, setSent] = useState(false)
  const reverse = useRef<ReturnType<typeof start> | undefined>(undefined), mapArea = useRef<HTMLDivElement>(null)
  const selected: MapPoint | undefined = validPoint(place.lat, place.lon) ? { label: place.address || place.name || t('mapPlace'), lat: place.lat, lon: place.lon } : undefined
  useEffect(() => () => { reverse.current?.abort(); reverse.current?.finish() }, [])
  function update<K extends keyof PlaceInput>(key: K, value: PlaceInput[K]) { setPlace((p) => ({ ...p, [key]: value })) }
  function abortReverse() { reverse.current?.abort(); reverse.current?.finish(); reverse.current = undefined }
  function coordinates(lat: number, lon: number) { abortReverse(); setLatText(Number.isFinite(lat) ? String(lat) : ''); setLonText(Number.isFinite(lon) ? String(lon) : ''); setPlace((p) => ({ ...p, lat, lon })) }
  async function pick(point: { lat: number; lon: number }) {
    coordinates(point.lat, point.lon)
    if (!geo.data?.enabled) return
    const task = start(); if (!task.current()) { task.finish(); return }; reverse.current = task
    try { const value = await request<{ places: MapPlace[] }>('/geo/reverse', { method: 'POST', signal: task.signal, body: JSON.stringify({ point, lang: locale }) }); if (task.current() && value.places[0]) update('address', value.places[0].label) }
    catch { /* The chosen point remains valid; its address can be entered manually. */ }
    finally { task.finish(); if (reverse.current === task) reverse.current = undefined }
  }
  async function submit(event: FormEvent) {
    event.preventDefault(); abortReverse()
    if (!validReason(reason) || (kind !== 'closed' && !validPlace(place))) { setInvalid(true); return }
    const task = start(); if (!task.current()) { task.finish(); return }; setBusy(true); setError(undefined); setInvalid(false)
    const input = { ...place, ...Object.fromEntries(Object.entries(place).filter(([, value]) => typeof value === 'string').map(([key, value]) => [key, (value as string).trim()])) }
    try {
      await request('/places/submissions', { method: 'POST', signal: task.signal, body: JSON.stringify({ kind, reason: reason.trim(), ...(current ? { place_id: current.id, base_version: current.version } : {}), ...(kind !== 'closed' ? { place: input } : {}) }) })
      if (task.current()) { setSent(true); void client.invalidateQueries({ queryKey: ['place-submissions', 'mine'] }) }
    } catch (value) { if (task.current()) setError(value) }
    finally { if (task.current()) setBusy(false); task.finish() }
  }
  if (sent) return <section className="surface place-form" role="status"><CheckCircleIcon size={32} /><h2>{t('placesPendingTitle')}</h2><p>{t('placesPendingHint')}</p><Link to="/places/submissions" className="button button-primary">{t('placesMy')}</Link><Link to="/places">{t('placesBack')}</Link></section>
  const stale = error instanceof ApiError && error.code === 'stale_version'
  return <form className="surface place-form" onSubmit={(event) => void submit(event)}>
    {kind === 'closed' && current ? <><h2>{current.name}</h2><PlaceInformation place={current} /><p>{t('placesClosedHint')}</p></> : <fieldset disabled={busy}>
      <Field label={t('placesName')} value={place.name} onChange={(event) => update('name', event.target.value)} required maxLength={200} />
      <div className="place-form-grid"><Field label={t('placesCategory')} required><select value={place.category} onChange={(event) => update('category', event.target.value as PlaceInput['category'])}>{placeCategories.map((category) => <option key={category} value={category}>{t(categoryLabels[category])}</option>)}</select></Field><CitySelector label={t('placesCity')} value={place.city} onChange={(value) => update('city', value)} maxLength={160} /></div>
      {geo.data?.enabled ? <LocationField label={t('placesAddress')} value={place.address} selected={selected} disabled={busy} onText={(address) => { abortReverse(); setLatText(''); setLonText(''); setPlace((p) => ({ ...p, address, lat: NaN, lon: NaN })) }} onSelect={(point) => { coordinates(point.lat, point.lon); update('address', point.label) }} onPick={() => { mapArea.current?.scrollIntoView({ block: 'center' }); mapArea.current?.focus() }} /> : <Field label={t('placesAddress')} value={place.address} maxLength={500} onChange={(event) => { abortReverse(); update('address', event.target.value) }} />}
      <p className="field-hint">{t('placesCoordinates')}</p><div className="place-form-grid"><Field label={t('placesLatitude')} value={latText} required type="number" step="any" min={region.south} max={region.north} onChange={(event) => { abortReverse(); setLatText(event.target.value); update('lat', event.target.value.trim() ? Number(event.target.value) : NaN) }} /><Field label={t('placesLongitude')} value={lonText} required type="number" step="any" min={region.west} max={region.east} onChange={(event) => { abortReverse(); setLonText(event.target.value); update('lon', event.target.value.trim() ? Number(event.target.value) : NaN) }} /></div>
      <div ref={mapArea} tabIndex={-1}>{config.data?.enabled && config.data.archive_url ? <Suspense fallback={<Loading />}><MapCanvas points={selected ? [selected] : []} archiveUrl={config.data.archive_url} locale={locale} onPick={(point) => void pick(point)} disabled={busy} /></Suspense> : <p className="field-hint">{t('placesMapUnavailable')}</p>}</div><p className="field-hint">{t('placesManual')}</p>
      <TextArea label={t('placesDescription')} value={place.description} onChange={(value) => update('description', value)} maxLength={2000} rows={4} />
      <div className="place-form-grid"><Field label={t('placesWebsite')} value={place.website} type="url" maxLength={2000} onChange={(event) => update('website', event.target.value)} /><Field label={t('placesPhone')} value={place.phone} type="tel" maxLength={100} onChange={(event) => update('phone', event.target.value)} /></div>
      <Field label={t('placesHours')} value={place.opening_hours} maxLength={300} onChange={(event) => update('opening_hours', event.target.value)} />
    </fieldset>}
    <fieldset disabled={busy}><TextArea label={t('placesReason')} value={reason} onChange={setReason} required maxLength={2000} hint={t('placesReasonHint')} rows={4} /></fieldset>
    <p className="field-hint">{t('placesFieldLimit')}</p>{invalid && <ErrorNotice text={t('placesFormInvalid')} />}{error != null && <ErrorNotice error={error} text={stale ? t('placesStale') : undefined} />}
    {stale && <div><p className="field-hint">{t('placesRefreshHint')}</p><button type="button" className="button button-outline" onClick={refresh}>{t('placesRefresh')}</button></div>}
    <div className="form-actions"><Link className="button button-outline" to="/places">{t('cancel')}</Link><button className="button button-primary" type="submit" disabled={busy || !online || stale}>{t(busy ? 'sending' : 'placesSend')}</button></div>
  </form>
}
export function MyPlaceSubmissionsPage() {
  const { t, locale } = useI18n(), { user } = useSession(), [cursors, setCursors] = useState([''])
  const page = cursors.length - 1
  const query = useQuery({ queryKey: ['place-submissions', 'mine', user?.id, cursors[page]], queryFn: ({ signal }) => request<{ submissions: PlaceSubmission[]; next_cursor: string }>('/me/place-submissions' + (cursors[page] ? '?after=' + encodeURIComponent(cursors[page]) : ''), { signal }), meta: { private: true }, gcTime: 0, retry: false, enabled: !!user })
  return <div className="page-content"><Link className="back-link" to="/places"><ArrowLeftIcon size={16} />{t('placesBack')}</Link><div className="places-heading"><h1>{t('placesMy')}</h1><Link className="button button-primary" to="/places/new">{t('placesSuggest')}</Link></div>{query.isPending ? <Loading /> : query.isError ? <ErrorNotice error={query.error} retry={() => void query.refetch()} /> : <><div className="place-submission-list">{query.data.submissions.length ? query.data.submissions.map((submission) => <article key={submission.id} className="surface place-submission-card"><div className="place-submission-meta"><span>{t(kindLabels[submission.kind])}</span><strong>{t(statusLabels[submission.status])}</strong><time dateTime={submission.created_at}>{new Date(submission.created_at).toLocaleDateString(intlLocale(locale))}</time></div><h2>{submission.place?.name ?? t('placesClosed')}</h2><p className="place-submission-reason">{submission.reason}</p>{submission.review_reason && <p className="place-submission-reason"><strong>{t('placesReviewReason')}: </strong>{submission.review_reason}</p>}{submission.status === 'pending' && <p className="field-hint">{t('placesPendingHint')}</p>}</article>) : <p className="surface empty-panel">{t('placesNoSubmissions')}</p>}</div><Pagination page={page} busy={query.isFetching} hasNext={!!query.data.next_cursor} next={() => { if (query.data.next_cursor) setCursors((v) => [...v, query.data.next_cursor]) }} previous={() => setCursors((v) => v.slice(0, -1))} /></>}</div>
}
