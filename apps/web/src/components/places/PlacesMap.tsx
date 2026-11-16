import { useEffect, useRef, useState } from 'react'
import { AttributionControl, LngLatBounds, Map as LibreMap, Marker } from 'maplibre-gl'
import type { GeoJSONSource } from 'maplibre-gl'
import { ArrowsOutSimpleIcon, MinusIcon, PlusIcon, XIcon } from '@phosphor-icons/react'
import { renderToStaticMarkup } from 'react-dom/server'
import { useI18n } from '../../i18n'
import { categoryLabels, region } from '../../places'
import type { Place, PlaceBounds } from '../../types'
import { initializeMapRuntime, retryMapArchive } from '../maps/map-browser-runtime'
import { mapStyle } from '../maps/map-style'
import { PlaceCategoryIcon } from './shared'
import 'maplibre-gl/dist/maplibre-gl.css'
import '../maps/map-canvas.css'

const sourceId = 'community-places'
interface Props { places: Place[]; selected?: string; selectedPlace?: Place; archiveUrl: string; bounds?: PlaceBounds; onSelect: (id: string) => void; onViewport: (bounds: PlaceBounds) => void }
interface Pin { key: string; point: [number, number]; place?: Place; cluster?: { id: number; count: number } }
export default function PlacesMap(props: Props) {
  const [attempt, setAttempt] = useState(0)
  return <MapSurface key={props.archiveUrl + ':' + attempt} {...props} retry={() => { try { retryMapArchive(props.archiveUrl); setAttempt((v) => v + 1) } catch { /* Invalid archive remains an inline error. */ } }} />
}
function MapSurface({ places, selected, selectedPlace, archiveUrl, bounds, onSelect, onViewport, retry }: Props & { retry: () => void }) {
  const { t, locale } = useI18n(), container = useRef<HTMLDivElement>(null), mapRef = useRef<LibreMap | null>(null)
  const inputs = useRef({ places, selected, selectedPlace, onSelect, onViewport, t })
  const refreshSource = useRef<(() => void) | undefined>(undefined)
  const chooser = useRef<HTMLDivElement>(null)
  const [choices, setChoices] = useState<{ items: Place[]; places: Place[]; selected?: string } | null>(null)
  const visibleChoices = choices?.places === places && choices.selected === selected ? choices.items : null
  const [status, setStatus] = useState<'loading' | 'ready' | 'error'>('loading')
  const { west, south, east, north } = bounds ?? region
  useEffect(() => { inputs.current = { places, selected, selectedPlace, onSelect, onViewport, t } })
  useEffect(() => { if (visibleChoices) chooser.current?.querySelector('button')?.focus() }, [visibleChoices])
  useEffect(() => {
    if (!container.current) return
    let active = true, loaded = false, sourcePending = false, sourceRevision = 0, map: LibreMap | undefined
    const markers = new Map<string, { marker: Marker; element: HTMLButtonElement; category?: Place['category'] }>()
    function data() {
      return { type: 'FeatureCollection' as const, features: inputs.current.places.filter((place) => place.id !== inputs.current.selected).map((place) => ({ type: 'Feature' as const, geometry: { type: 'Point' as const, coordinates: [place.lon, place.lat] }, properties: { id: place.id } })) }
    }
    function updateMarkers() {
      if (!active || !map || !loaded || sourcePending || !map.isSourceLoaded(sourceId)) return
      const current = inputs.current, byId = new Map(current.places.map((place) => [place.id, place]))
      const desired = new Set<string>(), pins: Pin[] = [], visibleBounds = map.getBounds()
      for (const feature of map.querySourceFeatures(sourceId)) {
        if (feature.geometry.type !== 'Point') continue
        if (!visibleBounds.contains(feature.geometry.coordinates as [number, number])) continue
        const properties = feature.properties
        if (properties.cluster) pins.push({ key: ['cluster', properties.cluster_id, properties.point_count, ...feature.geometry.coordinates].join('-'), point: feature.geometry.coordinates as [number, number], cluster: { id: Number(properties.cluster_id), count: Number(properties.point_count) } })
        else {
          const place = byId.get(String(properties.id))
          if (place && place.id !== current.selected) pins.push({ key: place.id, point: [place.lon, place.lat], place })
        }
      }
      const selectedItem = byId.get(current.selected ?? '') ?? current.selectedPlace
      if (current.selected && selectedItem && visibleBounds.contains([selectedItem.lon, selectedItem.lat])) pins.push({ key: selectedItem.id, point: [selectedItem.lon, selectedItem.lat], place: selectedItem })
      for (const pin of pins) {
        if (desired.has(pin.key)) continue // Adjacent source tiles can return the same feature.
        desired.add(pin.key)
        let entry = markers.get(pin.key)
        if (!entry) {
          const element = document.createElement('button')
          element.type = 'button'; element.className = 'place-map-marker' + (pin.cluster ? ' place-map-cluster' : '')
          if (pin.place) {
            const place = pin.place
            element.innerHTML = renderToStaticMarkup(<PlaceCategoryIcon category={place.category} size={18} />)
            element.addEventListener('click', (event) => { event.stopPropagation(); setChoices(null); inputs.current.onSelect(place.id) })
          } else if (pin.cluster) {
            const { id, count } = pin.cluster
            element.textContent = String(count)
            element.addEventListener('click', async (event) => {
              event.stopPropagation()
              const currentMap = map, source = currentMap?.getSource(sourceId) as GeoJSONSource | undefined
              if (!source || !currentMap || sourcePending) return
              const expectedRevision = sourceRevision
              try {
                const zoom = await source.getClusterExpansionZoom(id)
                if (!active || expectedRevision !== sourceRevision) return
                if (zoom <= currentMap.getMaxZoom()) { setChoices(null); currentMap.easeTo({ center: pin.point, zoom, duration: 0 }) }
                else {
                  // Coincident places cannot separate through zoom. Keep each one reachable.
                  const leaves = await source.getClusterLeaves(id, count, 0)
                  if (!active || expectedRevision !== sourceRevision) return
                  const ids = new Set(leaves.map((leaf) => String(leaf.properties?.id)))
                  setChoices({ items: inputs.current.places.filter((place) => ids.has(place.id)), places: inputs.current.places, selected: inputs.current.selected })
                }
              } catch { /* A refreshed source can retire a cluster while its click is pending. */ }
            })
          }
          entry = { marker: new Marker({ element }).setLngLat(pin.point).addTo(map), element, category: pin.place?.category }
          markers.set(pin.key, entry)
        } else entry.marker.setLngLat(pin.point)
        const isSelected = pin.place?.id === current.selected
        entry.element.classList.toggle('selected', isSelected)
        if (pin.place) {
          if (entry.category !== pin.place.category) {
            entry.element.innerHTML = renderToStaticMarkup(<PlaceCategoryIcon category={pin.place.category} size={18} />)
            entry.category = pin.place.category
          }
          entry.element.setAttribute('aria-label', current.t('placesSelect', { name: pin.place.name, category: current.t(categoryLabels[pin.place.category]) }))
          entry.element.setAttribute('aria-pressed', String(isSelected)); entry.element.title = pin.place.name
        } else if (pin.cluster) {
          entry.element.setAttribute('aria-label', current.t('placesCluster', { count: pin.cluster.count })); entry.element.title = current.t('placesCluster', { count: pin.cluster.count })
        }
      }
      for (const [key, entry] of markers) {
        if (desired.has(key)) continue
        const focused = document.activeElement === entry.element
        entry.marker.remove(); markers.delete(key)
        if (focused) map.getCanvas().focus({ preventScroll: true })
      }
    }
    function updateSource() {
      if (!active || !map || !loaded) return
      const source = map.getSource(sourceId) as GeoJSONSource | undefined
      if (!source) return
      const revision = ++sourceRevision
      sourcePending = true
      void source.setData(data()).then(() => {
        if (!active || revision !== sourceRevision) return
        sourcePending = false; updateMarkers()
      }).catch(() => { if (active && revision === sourceRevision) { sourcePending = false; setStatus('error') } })
    }
    try {
      initializeMapRuntime()
      map = new LibreMap({ container: container.current, style: mapStyle(archiveUrl, locale, window.location.origin), center: [66.888315, 48.030725], zoom: 4, maxZoom: 20, renderWorldCopies: false, attributionControl: false, dragRotate: false, touchPitch: false })
      mapRef.current = map
      map.addControl(new AttributionControl({ compact: false }), 'bottom-left')
      map.getCanvas().setAttribute('aria-label', t('placesMap'))
      const viewport = () => {
        if (!active || !map) return
        const b = map.getBounds()
        inputs.current.onViewport({ west: Math.max(region.west, b.getWest()), south: Math.max(region.south, b.getSouth()), east: Math.min(region.east, b.getEast()), north: Math.min(region.north, b.getNorth()) })
      }
      map.on('moveend', viewport)
      map.on('render', updateMarkers)
      map.on('load', () => {
        if (!active || !map) return
        map.addSource(sourceId, { type: 'geojson', data: data(), cluster: true, clusterRadius: 52, clusterMaxZoom: 20, maxzoom: 22 })
        // HTML buttons provide keyboard access; this invisible layer loads source tiles.
        map.addLayer({ id: sourceId, type: 'circle', source: sourceId, paint: { 'circle-radius': 0, 'circle-opacity': 0 } })
        loaded = true; setStatus('ready'); updateMarkers(); viewport()
      })
      map.on('error', (event) => { if (active && !(event.error instanceof Error && event.error.name === 'AbortError')) setStatus('error') })
      refreshSource.current = updateSource
    } catch { queueMicrotask(() => { if (active) setStatus('error') }) }
    const observer = map && typeof ResizeObserver !== 'undefined' ? new ResizeObserver(() => map?.resize()) : undefined
    if (container.current) observer?.observe(container.current)
    return () => { active = false; loaded = false; refreshSource.current = undefined; observer?.disconnect(); markers.forEach(({ marker }) => marker.remove()); markers.clear(); mapRef.current = null; map?.remove() }
    // Locale recreates the style and all accessible marker labels.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [archiveUrl, locale])
  useEffect(() => { refreshSource.current?.() }, [places, selected, selectedPlace, locale])
  useEffect(() => {
    const map = mapRef.current
    if (map) map.fitBounds([[west, south], [east, north]], { duration: 0, padding: 24 })
  }, [archiveUrl, locale, west, south, east, north])
  useEffect(() => {
    const place = places.find((p) => p.id === selected) ?? selectedPlace
    if (place && selected) mapRef.current?.easeTo({ center: [place.lon, place.lat], duration: 0 })
  }, [selected, selectedPlace, places, locale])
  function fit() { if (!mapRef.current || !places.length) return; const b = new LngLatBounds(); places.forEach((p) => b.extend([p.lon, p.lat])); mapRef.current.fitBounds(b, { padding: 40, maxZoom: 14, duration: 0 }) }
  function closeChoices() { setChoices(null); mapRef.current?.getCanvas().focus({ preventScroll: true }) }
  return <div className="map-canvas places-map" role="region" aria-label={t('placesMap')}><div className="places-map-viewport"><div className="map-canvas-surface" ref={container} /><div className="map-canvas-controls" role="group" aria-label={t('placesMap')}><button type="button" aria-label={t('placesZoomIn')} onClick={() => mapRef.current?.zoomIn({ duration: 0 })}><PlusIcon size={20} /></button><button type="button" aria-label={t('placesZoomOut')} onClick={() => mapRef.current?.zoomOut({ duration: 0 })}><MinusIcon size={20} /></button><button type="button" aria-label={t('placesFit')} disabled={!places.length} onClick={fit}><ArrowsOutSimpleIcon size={20} /></button></div>{visibleChoices && <div ref={chooser} className="place-cluster-chooser" role="group" aria-label={t('placesClusterChoose')} onKeyDown={(event) => { if (event.key === 'Escape') { event.preventDefault(); closeChoices() } }}><button type="button" className="text-button" onClick={closeChoices}><XIcon size={16} />{t('close')}</button>{visibleChoices.map((place) => <button key={place.id} type="button" onClick={() => { setChoices(null); onSelect(place.id) }}>{place.name}</button>)}</div>}</div>{status === 'loading' && <div className="map-canvas-status" role="status"><span className="map-canvas-loading-bar" />{t('loading')}</div>}{status === 'error' && <div className="map-canvas-status map-canvas-error" role="alert">{t('placesMapUnavailable')}<button type="button" className="text-button" onClick={retry}>{t('retry')}</button></div>}</div>
}
