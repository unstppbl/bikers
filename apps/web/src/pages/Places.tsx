import { lazy, Suspense, useEffect, useId, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link, useLocation, useSearchParams } from 'react-router-dom'
import { ArrowLeftIcon, ListBulletsIcon, MapTrifoldIcon, PlusIcon, MagnifyingGlassIcon, SlidersHorizontalIcon } from '@phosphor-icons/react'
import { request } from '../api'
import { useI18n } from '../i18n'
import { useSession } from '../session'
import { byteLength, categoryLabels, placeCategories, placesQuery, readPlaceFilters, validBounds } from '../places'
import { ErrorNotice, Loading } from '../ui'
import { CitySelector, useCityOptions } from '../components/CitySelector'
import { MapDiscoverySwitch } from '../components/maps/MapDiscoverySwitch'
import { cityCenter } from '../cities'
import { PlaceCategoryIcon, PlaceDetails } from '../components/places/shared'
import type { MapConfig, Place, PlaceBounds } from '../types'
import './places.css'
const PlacesMap = lazy(() => import('../components/places/PlacesMap'))
const noPlaces: Place[] = []
export function usePlacesConfig() { return useQuery({ queryKey: ['places-config'], queryFn: ({ signal }) => request<MapConfig>('/places/config', { signal }), meta: { private: false }, staleTime: 60_000, retry: false }) }

export function PlacesPage() {
  const { t } = useI18n(), { user, error: sessionError } = useSession()
  const [params, setParams] = useSearchParams(), f = readPlaceFilters(params), config = usePlacesConfig()
  const cities = useCityOptions()
  const [viewport, setViewport] = useState<PlaceBounds>()
  const [view, setView] = useState<'map' | 'list'>('map')
  const mapViewRef = useRef<HTMLButtonElement>(null)
  const detailsRef = useRef<HTMLElement>(null)
  const resultsRef = useRef<HTMLElement>(null)
  const previousSelection = useRef('')
  const focusedSelection = useRef('')
  const location = useLocation()
  const scope = JSON.stringify([f.q, f.city, f.category, f.bounds])
  const stored = (location.state as { placesPaging?: { scope: string; cursors: string[] } } | null)?.placesPaging
  const cursors = stored?.scope === scope && stored.cursors.at(-1) === f.after ? stored.cursors : f.after ? ['', f.after] : ['']
  const queryString = placesQuery(params)
  const query = useQuery({ queryKey: ['places', 'list', queryString], queryFn: ({ signal }) => request<{ places: Place[]; next_cursor: string }>('/places?' + queryString, { signal }), meta: { private: false }, retry: false, enabled: byteLength(f.q) <= 160 && byteLength(f.city) <= 160 })
  const selectedId = params.get('place') ?? '', listed = query.data?.places.find((p) => p.id === selectedId)
  const selectedQuery = useQuery({ queryKey: ['places', 'detail', selectedId], queryFn: ({ signal }) => request<{ place: Place }>('/places/' + encodeURIComponent(selectedId), { signal }), enabled: !!selectedId && !listed, meta: { private: false }, retry: false })
  const selected = listed ?? (selectedQuery.isError ? undefined : selectedQuery.data?.place)
  const hasSelected = !!selected
  useEffect(() => {
    if (selectedId && hasSelected && focusedSelection.current !== selectedId) {
      detailsRef.current?.focus({ preventScroll: true })
      focusedSelection.current = selectedId
    }
    else if (previousSelection.current && !selectedId) {
      const button = Array.from(resultsRef.current?.querySelectorAll<HTMLButtonElement>('[data-place-id]') ?? []).find((item) => item.dataset.placeId === previousSelection.current)
      if (view === 'map' && window.innerWidth <= 900) mapViewRef.current?.focus({ preventScroll: true })
      else button?.focus({ preventScroll: true })
    }
    if (!selectedId) focusedSelection.current = ''
    previousSelection.current = selectedId
  }, [selectedId, hasSelected, view])
  function clearSelection() { const next = new URLSearchParams(params); next.delete('place'); setParams(next, { replace: true, state: location.state }) }
  function change(next: URLSearchParams) { next.delete('after'); next.delete('place'); setParams(next) }
  function select(id: string) { const next = new URLSearchParams(params); next.set('place', id); setParams(next, { replace: true, state: location.state }) }
  function goToCursor(nextCursors: string[]) {
    const next = new URLSearchParams(params), cursor = nextCursors.at(-1) ?? ''
    if (cursor) next.set('after', cursor); else next.delete('after')
    next.delete('place')
    setParams(next, { state: { placesPaging: { scope, cursors: nextCursors } } })
  }
  function searchArea() {
    if (!viewport || !validBounds(viewport)) return
    const next = new URLSearchParams(params)
    for (const [key, value] of Object.entries(viewport)) next.set(key, value.toFixed(6))
    change(next)
  }
  return <div className="page-content places-page" data-view={view} data-selected={!!selectedId}>
    <div className="places-heading">
      <div className="places-heading-main">
        <h1>{t('placesTitle')}</h1>
        <MapDiscoverySwitch current="places" city={f.city} />
      </div>
      <div className="place-actions">{user && !sessionError && <Link className="text-button places-my-suggestions" to="/places/submissions">{t('placesMy')}</Link>}<Link className="button button-outline" to={user && !sessionError ? '/places/new' : '/login?next=%2Fplaces%2Fnew'}><PlusIcon size={16} />{t('placesSuggest')}</Link></div>
    </div>
    <PlaceFilters params={params} apply={change} suggestions={!!user && !sessionError} />
    <div className="places-workspace-bar">
      <p className="places-result-count" role="status">{query.data && t('placesCount', { count: query.data.places.length })}</p>
      {f.bounds && <div className="places-area-note"><span>{t('placesArea')}</span><button className="text-button" onClick={() => { const next = new URLSearchParams(params); ['west', 'south', 'east', 'north'].forEach((key) => next.delete(key)); change(next) }}>{t('placesClearArea')}</button></div>}
      <div className="places-view-switch" role="group" aria-label={t('placesView')}><button ref={mapViewRef} type="button" aria-pressed={view === 'map'} onClick={() => setView('map')}><MapTrifoldIcon size={16} />{t('placesMapView')}</button><button type="button" aria-pressed={view === 'list'} onClick={() => setView('list')}><ListBulletsIcon size={16} />{t('placesList')}</button></div>
    </div>
    <div className="places-layout">
      <section ref={resultsRef} className="places-sidebar" aria-label={t('placesTitle')}>
        {selectedId ? <section className="places-detail-view" ref={detailsRef} tabIndex={-1} aria-label={t('placesDetails')}><button type="button" className="text-button places-back-results" onClick={clearSelection}><ArrowLeftIcon size={16} />{t('placesBackResults')}</button>{selected ? <PlaceDetails place={selected} /> : selectedQuery.isPending ? <Loading /> : selectedQuery.isError ? <ErrorNotice error={selectedQuery.error} retry={() => void selectedQuery.refetch()} /> : null}</section> : <div className="places-results">
          {byteLength(f.q) > 160 || byteLength(f.city) > 160 ? <ErrorNotice text={t('placesTextLimit')} /> : query.isPending ? <Loading /> : query.isError ? <ErrorNotice error={query.error} retry={() => void query.refetch()} /> : <>
            {query.data.places.length ? <ul className="places-list">{query.data.places.map((place) => <li key={place.id}><button type="button" className="place-list-item" data-place-id={place.id} onClick={() => select(place.id)}><span className="place-list-icon"><PlaceCategoryIcon category={place.category} size={20} /></span><span><strong>{place.name}</strong><span>{t(categoryLabels[place.category])}</span><small>{[place.city, place.address].filter(Boolean).join(' · ') || place.lat.toFixed(4) + ', ' + place.lon.toFixed(4)}</small></span></button></li>)}</ul> : <div className="empty-panel"><h2>{t('placesEmpty')}</h2><p>{t('placesEmptyHint')}</p></div>}
            {query.data.next_cursor && <p className="places-page-cap">{t('placesCap')}</p>}
            <div className="places-pagination"><button type="button" className="button button-outline" disabled={!f.after || query.isFetching} onClick={() => goToCursor(cursors.slice(0, -1))}>{t('previousPage')}</button><button type="button" className="button button-outline" disabled={!query.data.next_cursor || query.isFetching} onClick={() => goToCursor([...cursors, query.data.next_cursor])}>{t('nextPage')}</button></div>
          </>}
        </div>}
      </section>
      <section className="places-map-column" aria-label={t('placesMap')}>
        {config.data?.enabled && config.data.archive_url ? <><div className="places-area-action"><button className="button button-outline" type="button" disabled={!viewport || !validBounds(viewport) || query.isFetching} onClick={searchArea}><MagnifyingGlassIcon size={16} />{t('placesSearchArea')}</button></div><Suspense fallback={<Loading />}><PlacesMap places={query.data?.places ?? noPlaces} selected={selectedId} selectedPlace={selected} bounds={f.bounds} center={cityCenter(f.city, cities.data?.cities ?? [])} archiveUrl={config.data.archive_url} onSelect={select} onViewport={setViewport} /></Suspense></> : config.isPending ? <Loading /> : <div className="places-map-unavailable"><MapTrifoldIcon size={32} /><p>{t('placesMapUnavailable')}</p><button className="button button-outline" type="button" onClick={() => setView('list')}>{t('placesList')}</button></div>}
      </section>
    </div>
  </div>
}

function PlaceFilters({ params, apply, suggestions }: { params: URLSearchParams; apply: (params: URLSearchParams) => void; suggestions: boolean }) {
  const { t } = useI18n(), f = readPlaceFilters(params)
  const [q, setQ] = useState(f.q), [city, setCity] = useState(f.city), [category, setCategory] = useState(f.category), [invalid, setInvalid] = useState(false)
  // Phones (the tab bar layout) open on the map; wider screens have room for the filters.
  const [expanded, setExpanded] = useState(() => !window.matchMedia?.('(max-width: 900px)').matches)
  // Drafts follow the URL (apply, clear, history) without remounting the form, so focus and the panel stay put.
  const filtersKey = JSON.stringify([f.q, f.city, f.category]), [shownKey, setShownKey] = useState(filtersKey)
  if (shownKey !== filtersKey) { setShownKey(filtersKey); setQ(f.q); setCity(f.city); setCategory(f.category); setInvalid(false) }
  const panelId = useId()
  const activeCount = Number(!!f.city) + Number(!!f.category)
  function submit(event: FormEvent) { event.preventDefault(); if (byteLength(q) > 160 || byteLength(city) > 160) { setInvalid(true); setExpanded(true); return }; const next = new URLSearchParams(params); if (city.trim() !== f.city) ['west', 'south', 'east', 'north'].forEach((key) => next.delete(key)); for (const [key, value] of Object.entries({ q: q.trim(), city: city.trim(), category })) { if (value) next.set(key, value); else next.delete(key) }; apply(next) }
  // The form checks lengths itself and opens the panel to show a problem; native validation would block that silently.
  return <form className="places-filters" onSubmit={submit} noValidate>
    <div className="places-search-row"><label className="places-search"><MagnifyingGlassIcon size={18} /><span className="sr-only">{t('placesSearch')}</span><input type="search" value={q} onChange={(event) => setQ(event.target.value)} maxLength={160} placeholder={t('placesSearch')} /></label><button className="button button-primary" type="submit">{t('placesSearchAction')}</button><button type="button" className="button button-outline places-filter-toggle" aria-label={t('placesFilters') + (activeCount ? ` (${activeCount})` : '')} aria-expanded={expanded} aria-controls={panelId} onClick={() => setExpanded((value) => !value)}><SlidersHorizontalIcon size={18} />{t('placesFilters')}{activeCount > 0 && <span className="places-filter-count">{activeCount}</span>}</button></div>
    <div id={panelId} className="places-filter-panel" hidden={!expanded}>
      <CitySelector label={t('placesCity')} value={city} onChange={setCity} hint="" />
      <div className="places-categories" role="group" aria-label={t('placesCategory')}><button type="button" aria-pressed={!category} className={'place-category-filter ' + (!category ? 'active' : '')} onClick={() => setCategory('')}>{t('placesAll')}</button>{placeCategories.map((value) => <button type="button" key={value} className={'place-category-filter ' + (value === category ? 'active' : '')} aria-pressed={value === category} onClick={() => setCategory(value)}><PlaceCategoryIcon category={value} size={16} />{t(categoryLabels[value])}</button>)}</div>
      <div className="places-filter-actions"><button className="button button-primary" type="submit">{t('placesApply')}</button><button className="text-button" type="button" onClick={() => { setQ(''); setCity(''); setCategory(''); apply(new URLSearchParams()) }}>{t('placesClear')}</button>{suggestions && <Link className="text-button places-mobile-suggestions" to="/places/submissions">{t('placesMy')}</Link>}</div>
    </div>{invalid && <ErrorNotice text={t('placesTextLimit')} />}
  </form>
}
