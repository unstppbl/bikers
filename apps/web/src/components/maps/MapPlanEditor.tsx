import { lazy, Suspense, useCallback, useEffect, useId, useMemo, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { MapPinIcon, PlusIcon, TrashIcon, PathIcon } from '@phosphor-icons/react'
import { request } from '../../api'
import { intlLocale, useI18n } from '../../i18n'
import { useSession, useSessionGuard } from '../../session'
import { Field, useOnline } from '../../ui'
import type { MapConfig, MapPlan, MapPlace, MapPoint } from '../../types'
import './map-planner.css'

const MapCanvas = lazy(() => import('./MapCanvas'))
const signature = (points: MapPoint[]) => JSON.stringify(points)

export function useMapConfig() {
  const { user } = useSession()
  return useQuery({ queryKey: ['geo', 'config', user?.id], queryFn: ({ signal }) => request<MapConfig>('/geo/config', { signal }), enabled: !!user, staleTime: 60_000, meta: { private: true } })
}

export function RouteSummary({ plan }: { plan: MapPlan }) {
  const { t, locale } = useI18n()
  const minutes = Math.max(1, Math.round(plan.duration_seconds / 60))
  return <div className="map-route-summary"><PathIcon size={20} /><strong>{t('mapKilometers', { distance: new Intl.NumberFormat(intlLocale(locale), { maximumFractionDigits: 1 }).format(plan.distance_meters / 1000) })}</strong><span>{minutes >= 60 ? t('mapHours', { hours: Math.floor(minutes / 60), minutes: minutes % 60 }) : t('mapMinutes', { minutes })}</span></div>
}

export function SavedRideMap({ plan }: { plan: MapPlan }) {
  const config = useMapConfig()
  const { t, locale } = useI18n()
  return <section className="surface ride-map-panel" aria-label={t('mapAndRoute')}>
    <h2>{t('mapAndRoute')}</h2>
    {config.data?.archive_url && <Suspense fallback={<div className="map-loading-placeholder" role="status">{t('loading')}</div>}><MapCanvas points={plan.points} geometry={plan.geometry} archiveUrl={config.data.archive_url} locale={locale} /></Suspense>}
    {config.isError && <p className="field-hint" role="status">{t('mapServiceUnavailable')}</p>}
    <ol className="map-saved-points">{plan.points.map((point, index) => <li key={index}><span>{point.label}</span><small>{point.lat.toFixed(5)}, {point.lon.toFixed(5)}</small></li>)}</ol>
    {plan.geometry && <RouteSummary plan={plan} />}
    <p className="field-hint">{t('mapRoadsNote')}</p>
  </section>
}

interface LocationFieldProps {
  label: string; value: string; selected?: MapPoint; bias?: MapPoint; required?: boolean; disabled: boolean
  onText: (value: string) => void; onSelect: (point: MapPoint) => void; onPick: () => void
}

function LocationField({ label, value, selected, bias, required = false, disabled, onText, onSelect, onPick }: LocationFieldProps) {
  const { t, locale } = useI18n()
  const { user } = useSession()
  const id = useId()
  const [focused, setFocused] = useState(false)
  const [search, setSearch] = useState(value)
  const [active, setActive] = useState(-1)
  useEffect(() => { const timer = setTimeout(() => { setSearch(value.trim()); setActive(-1) }, 350); return () => clearTimeout(timer) }, [value])
  const query = useQuery({
    queryKey: ['geo', 'search', user?.id, locale, search, bias?.lat, bias?.lon],
    queryFn: ({ signal }) => {
      return request<{ places: MapPlace[] }>('/geo/search', {
        method: 'POST', signal,
        body: JSON.stringify({ query: search, lang: locale, ...(bias ? { near: { lat: bias.lat, lon: bias.lon } } : {}) }),
      })
    },
    enabled: focused && !selected && search.length >= 2 && search === value.trim() && !disabled,
    gcTime: 0, retry: false, meta: { private: true },
  })
  const places = query.data?.places ?? []
  const open = focused && !selected && value.trim().length >= 2 && !disabled
  function choose(point: MapPoint) { onSelect({ label: point.label, lat: point.lat, lon: point.lon }); setFocused(false); setActive(-1) }
  const category = (kind: string) => t(kind === 'street' ? 'mapStreet' : kind === 'house' ? 'mapHouse' : kind === 'city' ? 'mapCity' : kind === 'state' ? 'mapState' : 'mapPlace')
  return <div className="map-location-field" onBlur={(event) => { if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setFocused(false) }}>
    <Field label={label} required={required}>
      <input value={value} disabled={disabled} required={required} maxLength={300} placeholder={t('mapSearchPlaceholder')} autoComplete="off" role="combobox" aria-autocomplete="list" aria-expanded={open} aria-controls={`${id}-places`} aria-activedescendant={active >= 0 && open ? `${id}-${active}` : undefined}
        onFocus={() => setFocused(true)} onChange={(event) => { onText(event.target.value); setFocused(true); setActive(-1) }}
        onKeyDown={(event) => {
          if (event.key === 'Escape') { setFocused(false); setActive(-1) }
          if (open && places.length && (event.key === 'ArrowDown' || event.key === 'ArrowUp')) { event.preventDefault(); setActive((current) => (current + (event.key === 'ArrowDown' ? 1 : -1) + places.length) % places.length) }
          if (open && event.key === 'Enter' && active >= 0 && places[active]) { event.preventDefault(); choose(places[active]) }
        }} />
    </Field>
    {open && <div className="map-search-results" id={`${id}-places`} role="listbox" aria-label={label}>
      {query.isFetching || search !== value.trim() ? <p role="status">{t('mapSearchLoading')}</p> : query.isError ? <p role="status">{t('mapSearchUnavailable')}</p> : places.length ? places.map((place, index) => <button key={`${place.id}-${index}`} id={`${id}-${index}`} type="button" role="option" aria-selected={index === active} className={index === active ? 'is-active' : ''} onMouseDown={(event) => event.preventDefault()} onClick={() => choose(place)}><span>{place.label}</span><small>{category(place.kind)}</small></button>) : <p role="status">{t('mapNoPlaces')}</p>}
    </div>}
    <div className="map-point-actions"><button type="button" className="text-button" onClick={onPick} disabled={disabled}><MapPinIcon size={16} />{t('mapChoosePin')}</button>{selected && <span className="field-hint" aria-label={t('mapSelected')}>{selected.lat.toFixed(5)}, {selected.lon.toFixed(5)}</span>}</div>
  </div>
}

export interface MapDraft { points: MapPoint[]; valid: boolean }
interface Stop { key: number; text: string; point?: MapPoint }

export function MapPlanEditor({ initialPlan, meeting, destination, onMeeting, onDestination, onChange, disabled, startWithMap = true }: {
  initialPlan?: MapPlan; meeting: string; destination: string; onMeeting: (value: string) => void; onDestination: (value: string) => void
  onChange: (draft: MapDraft) => void; disabled: boolean; startWithMap?: boolean
}) {
  const { t, locale } = useI18n()
  const config = useMapConfig()
  const online = useOnline()
  const captureSession = useSessionGuard()
  const [mapChoice, setMapChoice] = useState<boolean | null>(initialPlan ? true : startWithMap ? null : false)
  const usingMap = mapChoice ?? !!config.data?.enabled
  const [first, setFirst] = useState<MapPoint | undefined>(initialPlan?.points[0])
  const [last, setLast] = useState<MapPoint | undefined>(initialPlan && initialPlan.points.length > 1 ? initialPlan.points.at(-1) : undefined)
  const [stops, setStops] = useState<Stop[]>(() => initialPlan?.points.slice(1, -1).map((point, key) => ({ key, text: point.label, point })) ?? [])
  const stopKey = useRef(stops.length)
  const [pickTarget, setPickTarget] = useState<number>(-1)
  const [reverseBusy, setReverseBusy] = useState(false)
  const [routeState, setRouteState] = useState<{ key: string; id: number; busy: boolean; error: boolean }>()
  const routeSequence = useRef(0)
  const [preview, setPreview] = useState<{ plan: MapPlan; signature: string } | undefined>(initialPlan ? { plan: initialPlan, signature: signature(initialPlan.points) } : undefined)
  const routeRequest = useRef<AbortController | undefined>(undefined)
  const reverseRequest = useRef<AbortController | undefined>(undefined)
  const valid = !usingMap || !!first && (!destination.trim() && !stops.length || !!last && stops.every((stop) => !!stop.point))
  const points = useMemo(() => usingMap && first ? [first, ...stops.flatMap((stop) => stop.point ? [stop.point] : []), ...(last ? [last] : [])] : [], [usingMap, first, stops, last])
  const pointsKey = signature(points)
  const currentKey = useRef(pointsKey)
  const routeBusy = routeState?.key === pointsKey && routeState.busy
  const routeError = routeState?.key === pointsKey && routeState.error
  useEffect(() => { onChange({ points: JSON.parse(pointsKey) as MapPoint[], valid }) }, [pointsKey, valid, onChange])
  useEffect(() => { currentKey.current = pointsKey; routeRequest.current?.abort() }, [pointsKey])
  useEffect(() => () => { routeRequest.current?.abort(); reverseRequest.current?.abort() }, [])
  const activePlan = preview?.signature === pointsKey ? preview.plan : undefined
  const targetName = pickTarget === -1 ? t('meetingPoint') : pickTarget === -2 ? t('destination') : t('mapStop', { number: stops.findIndex((stop) => stop.key === pickTarget) + 1 })
  const cancelReverse = useCallback(() => { reverseRequest.current?.abort(); setReverseBusy(false) }, [])
  const select = useCallback((target: number, point: MapPoint) => {
    cancelReverse()
    if (target === -1) { setFirst(point); onMeeting(point.label) }
    else if (target === -2) { setLast(point); onDestination(point.label) }
    else setStops((current) => current.map((stop) => stop.key === target ? { ...stop, text: point.label, point } : stop))
  }, [onMeeting, onDestination, cancelReverse])
  async function choosePin(point: { lat: number; lon: number }) {
    const isCurrent = captureSession()
    const target = pickTarget
    reverseRequest.current?.abort()
    const controller = new AbortController(); reverseRequest.current = controller
    setReverseBusy(true)
    try {
      const result = await request<{ places: MapPlace[] }>('/geo/reverse', { method: 'POST', body: JSON.stringify({ point, lang: locale }), signal: controller.signal })
      if (isCurrent() && !controller.signal.aborted) select(target, { ...point, label: result.places[0]?.label ?? t('mapPin') })
    } catch { if (isCurrent() && !controller.signal.aborted) select(target, { ...point, label: t('mapPin') }) }
    finally { if (!controller.signal.aborted) setReverseBusy(false) }
  }
  async function calculate() {
    const isCurrent = captureSession()
    const expected = pointsKey
    routeRequest.current?.abort()
    const controller = new AbortController(); routeRequest.current = controller
    const requestID = ++routeSequence.current
    setRouteState({ key: expected, id: requestID, busy: true, error: false })
    try {
      const result = await request<{ plan: MapPlan }>('/geo/route', { method: 'POST', body: JSON.stringify({ points, lang: locale }), signal: controller.signal })
      if (isCurrent() && !controller.signal.aborted && currentKey.current === expected) setPreview({ plan: result.plan, signature: expected })
    } catch { if (isCurrent() && !controller.signal.aborted) setRouteState({ key: expected, id: requestID, busy: false, error: true }) }
    finally { if (isCurrent()) setRouteState((current) => current?.id === requestID ? { ...current, busy: false } : current) }
  }
  return <div className="map-plan-editor">
    {(config.data?.enabled || initialPlan) && <div className="map-mode-row"><h3>{t('mapAndRoute')}</h3><button type="button" className="text-button" disabled={disabled} onClick={() => { cancelReverse(); setMapChoice(!usingMap) }}>{t(usingMap ? 'mapUseText' : 'mapUseMap')}</button></div>}
    {!usingMap ? <><Field label={t('meetingPoint')} value={meeting} onChange={(event) => onMeeting(event.target.value)} placeholder={t('meetingPlaceholder')} maxLength={300} required disabled={disabled} /><Field label={`${t('destination')} · ${t('optional')}`} value={destination} onChange={(event) => onDestination(event.target.value)} placeholder={t('destinationPlaceholder')} maxLength={300} disabled={disabled} />{config.isError && <p className="field-hint" role="status">{t('mapServiceUnavailable')}</p>}</> : <>
      <p className="field-hint">{t('mapSearchHint')}</p>
      <LocationField label={t('meetingPoint')} value={meeting} selected={first} required disabled={disabled} onText={(value) => { cancelReverse(); setFirst(undefined); onMeeting(value) }} onSelect={(point) => select(-1, point)} onPick={() => { cancelReverse(); setPickTarget(-1) }} />
      {stops.map((stop, index) => <div className="map-via-row" key={stop.key}><LocationField label={t('mapStop', { number: index + 1 })} value={stop.text} selected={stop.point} bias={first} required disabled={disabled} onText={(value) => { cancelReverse(); setStops((current) => current.map((entry) => entry.key === stop.key ? { ...entry, text: value, point: undefined } : entry)) }} onSelect={(point) => select(stop.key, point)} onPick={() => { cancelReverse(); setPickTarget(stop.key) }} /><button type="button" className="icon-button" aria-label={t('mapRemoveStop', { number: index + 1 })} disabled={disabled} onClick={() => { cancelReverse(); setStops((current) => current.filter((entry) => entry.key !== stop.key)); setPickTarget(-1) }}><TrashIcon size={18} /></button></div>)}
      <LocationField label={`${t('destination')} · ${t('optional')}`} value={destination} selected={last} bias={first} disabled={disabled} onText={(value) => { cancelReverse(); setLast(undefined); onDestination(value) }} onSelect={(point) => select(-2, point)} onPick={() => { cancelReverse(); setPickTarget(-2) }} />
      <button type="button" className="text-button" disabled={disabled || stops.length >= 8} onClick={() => setStops((current) => [...current, { key: stopKey.current++, text: '' }])}><PlusIcon size={16} />{t('mapAddStop')}</button>
      {config.data?.archive_url && <><p className="map-pick-hint" role="status">{reverseBusy ? t('mapLookingUpPin') : t('mapClickPin', { place: targetName })}</p><Suspense fallback={<div className="map-loading-placeholder" role="status">{t('loading')}</div>}><MapCanvas points={points} geometry={activePlan?.geometry} archiveUrl={config.data.archive_url} locale={locale} onPick={(point) => void choosePin(point)} disabled={disabled || !online} /></Suspense></>}
      {!valid && <p className="field-hint">{t(first ? 'mapSelectionRequired' : 'mapMeetingRequired')}</p>}
      <div className="map-route-actions"><button type="button" className="button button-outline" disabled={disabled || !online || !valid || points.length < 2 || routeBusy} onClick={() => void calculate()}><PathIcon size={18} />{t(routeBusy ? 'mapCalculating' : 'mapCalculate')}</button>{activePlan?.geometry && <RouteSummary plan={activePlan} />}</div>
      {routeError && <p className="error-notice" role="alert">{t('mapRouteUnavailable')}</p>}
      {preview && !activePlan && points.length >= 2 && <p className="field-hint" role="status">{t('mapRouteStale')}</p>}
      {points.length < 2 && <p className="field-hint">{t('mapRouteNoDestination')}</p>}
      <p className="field-hint">{t('mapRoadsNote')}</p><p className="field-hint">{t('mapPrivate')}</p>
    </>}
  </div>
}
