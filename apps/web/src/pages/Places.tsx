import { lazy, Suspense, useState } from 'react'
import type { FormEvent } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link, useLocation, useSearchParams } from 'react-router-dom'
import { MapPinIcon, PlusIcon, MagnifyingGlassIcon } from '@phosphor-icons/react'
import { request } from '../api'
import { useI18n } from '../i18n'
import { useSession } from '../session'
import { byteLength, categoryLabels, placeCategories, placesQuery, readPlaceFilters, validBounds } from '../places'
import { ErrorNotice, Field, Loading } from '../ui'
import { CitySelector } from '../components/CitySelector'
import { PlaceCategoryIcon, PlaceDetails } from '../components/places/shared'
import type { MapConfig, Place, PlaceBounds } from '../types'
import './places.css'
const PlacesMap = lazy(() => import('../components/places/PlacesMap'))
const noPlaces: Place[] = []
export function usePlacesConfig() { return useQuery({ queryKey: ['places-config'], queryFn: ({ signal }) => request<MapConfig>('/places/config', { signal }), meta: { private: false }, staleTime: 60_000, retry: false }) }

export function PlacesPage() {
  const { t } = useI18n(), { user, error: sessionError } = useSession()
  const [params, setParams] = useSearchParams(), f = readPlaceFilters(params), config = usePlacesConfig()
  const [viewport, setViewport] = useState<PlaceBounds>()
  const location = useLocation()
  const scope = JSON.stringify([f.q, f.city, f.category, f.bounds])
  const stored = (location.state as { placesPaging?: { scope: string; cursors: string[] } } | null)?.placesPaging
  const cursors = stored?.scope === scope && stored.cursors.at(-1) === f.after ? stored.cursors : f.after ? ['', f.after] : ['']
  const queryString = placesQuery(params)
  const query = useQuery({ queryKey: ['places', 'list', queryString], queryFn: ({ signal }) => request<{ places: Place[]; next_cursor: string }>('/places?' + queryString, { signal }), meta: { private: false }, retry: false, enabled: byteLength(f.q) <= 160 && byteLength(f.city) <= 160 })
  const selectedId = params.get('place') ?? '', listed = query.data?.places.find((p) => p.id === selectedId)
  const selectedQuery = useQuery({ queryKey: ['places', 'detail', selectedId], queryFn: ({ signal }) => request<{ place: Place }>('/places/' + encodeURIComponent(selectedId), { signal }), enabled: !!selectedId && !listed, meta: { private: false }, retry: false })
  const selected = listed ?? (selectedQuery.isError ? undefined : selectedQuery.data?.place)
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
  const filtersKey = JSON.stringify([f.q, f.city, f.category])
  return <div className="page-content places-page">
    <div className="places-heading"><div><span className="eyebrow">{t('community')}</span><h1>{t('placesTitle')}</h1><p>{t('placesIntro')}</p></div><div className="place-actions"><Link className="button button-primary" to={user && !sessionError ? '/places/new' : '/login?next=%2Fplaces%2Fnew'}><PlusIcon size={18} />{t('placesSuggest')}</Link>{user && !sessionError && <Link className="text-button" to="/places/submissions">{t('placesMy')}</Link>}</div></div>
    <PlaceFilters key={filtersKey} params={params} apply={change} />
    {f.bounds && <p className="places-area-note">{t('placesArea')} <button className="text-button" onClick={() => { const next = new URLSearchParams(params); ['west', 'south', 'east', 'north'].forEach((key) => next.delete(key)); change(next) }}>{t('placesClearArea')}</button></p>}
    <div className="places-layout"><section className="places-results" aria-label={t('placesTitle')}>
      {byteLength(f.q) > 160 || byteLength(f.city) > 160 ? <ErrorNotice text={t('placesTextLimit')} /> : query.isPending ? <Loading /> : query.isError ? <ErrorNotice error={query.error} retry={() => void query.refetch()} /> : <>
        <p className="field-hint" role="status">{t('placesCount', { count: query.data.places.length })}</p>
        {query.data.places.length ? <ul className="places-list">{query.data.places.map((place) => <li key={place.id}><button type="button" className={'surface place-list-item ' + (selectedId === place.id ? 'selected' : '')} aria-pressed={selectedId === place.id} onClick={() => select(place.id)}><span className="place-list-icon"><PlaceCategoryIcon category={place.category} size={24} /></span><span><strong>{place.name}</strong><span>{t(categoryLabels[place.category])}</span><small>{[place.city, place.address].filter(Boolean).join(' · ') || place.lat.toFixed(4) + ', ' + place.lon.toFixed(4)}</small></span><MapPinIcon size={18} /></button></li>)}</ul> : <div className="surface empty-panel"><h2>{t('placesEmpty')}</h2><p>{t('placesEmptyHint')}</p></div>}
        {query.data.next_cursor && <p className="field-hint">{t('placesCap')}</p>}
        <div className="places-pagination"><button type="button" className="button button-outline" disabled={!f.after || query.isFetching} onClick={() => goToCursor(cursors.slice(0, -1))}>{t('previousPage')}</button><button type="button" className="button button-outline" disabled={!query.data.next_cursor || query.isFetching} onClick={() => goToCursor([...cursors, query.data.next_cursor])}>{t('nextPage')}</button></div>
      </>}
    </section><section className="places-map-column" aria-label={t('placesMap')}>
      {config.data?.enabled && config.data.archive_url ? <><Suspense fallback={<Loading />}><PlacesMap places={query.data?.places ?? noPlaces} selected={selectedId} bounds={f.bounds} archiveUrl={config.data.archive_url} onSelect={select} onViewport={setViewport} /></Suspense><div className="places-area-action"><button className="button button-outline" type="button" disabled={!viewport || !validBounds(viewport) || query.isFetching} onClick={searchArea}><MagnifyingGlassIcon size={18} />{t('placesSearchArea')}</button></div></> : config.isPending ? <Loading /> : <p className="surface empty-panel">{t('placesMapUnavailable')}</p>}
      {selected ? <PlaceDetails place={selected} /> : selectedId && selectedQuery.isPending ? <Loading /> : selectedQuery.isError && selectedId ? <ErrorNotice error={selectedQuery.error} retry={() => void selectedQuery.refetch()} /> : <p className="field-hint">{t('placesChoose')}</p>}
      <p className="place-source"><a href="https://www.openstreetmap.org/copyright" target="_blank" rel="noopener noreferrer">© OpenStreetMap contributors · ODbL</a></p>
    </section></div>
  </div>
}
function PlaceFilters({ params, apply }: { params: URLSearchParams; apply: (params: URLSearchParams) => void }) {
  const { t } = useI18n(), f = readPlaceFilters(params)
  const [q, setQ] = useState(f.q), [city, setCity] = useState(f.city), [category, setCategory] = useState(f.category), [invalid, setInvalid] = useState(false)
  function submit(event: FormEvent) { event.preventDefault(); if (byteLength(q) > 160 || byteLength(city) > 160) { setInvalid(true); return }; const next = new URLSearchParams(params); for (const [key, value] of Object.entries({ q: q.trim(), city: city.trim(), category })) { if (value) next.set(key, value); else next.delete(key) }; apply(next) }
  return <form className="surface places-filters" onSubmit={submit}><div className="places-filter-fields"><Field label={t('placesSearch')} value={q} onChange={(event) => setQ(event.target.value)} maxLength={160} /><CitySelector label={t('placesCity')} value={city} onChange={setCity} maxLength={160} /><button className="button button-primary" type="submit"><MagnifyingGlassIcon size={18} />{t('placesApply')}</button><button className="text-button" type="button" onClick={() => { setQ(''); setCity(''); setCategory(''); apply(new URLSearchParams()) }}>{t('placesClear')}</button></div><div className="places-categories" role="group" aria-label={t('placesCategory')}><button type="button" aria-pressed={!category} className={'place-category-filter ' + (!category ? 'active' : '')} onClick={() => setCategory('')}>{t('placesAll')}</button>{placeCategories.map((value) => <button type="button" key={value} className={'place-category-filter ' + (value === category ? 'active' : '')} aria-pressed={value === category} onClick={() => setCategory(value)}><PlaceCategoryIcon category={value} />{t(categoryLabels[value])}</button>)}</div>{invalid && <ErrorNotice text={t('placesTextLimit')} />}</form>
}
