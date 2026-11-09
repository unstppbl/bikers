import { useEffect, useRef, useState } from 'react'
import { AttributionControl, LngLatBounds, Map as LibreMap, Marker } from 'maplibre-gl'
import { ArrowsOutSimpleIcon, MinusIcon, PlusIcon } from '@phosphor-icons/react'
import { renderToStaticMarkup } from 'react-dom/server'
import { useI18n } from '../../i18n'
import { categoryLabels, region } from '../../places'
import type { Place, PlaceBounds } from '../../types'
import { initializeMapRuntime, retryMapArchive } from '../maps/map-browser-runtime'
import { mapStyle } from '../maps/map-style'
import { PlaceCategoryIcon } from './shared'
import 'maplibre-gl/dist/maplibre-gl.css'
import '../maps/map-canvas.css'

interface Props { places: Place[]; selected?: string; archiveUrl: string; bounds?: PlaceBounds; onSelect: (id: string) => void; onViewport: (bounds: PlaceBounds) => void }
export default function PlacesMap(props: Props) {
  const [attempt, setAttempt] = useState(0)
  return <MapSurface key={props.archiveUrl + ':' + attempt} {...props} retry={() => { try { retryMapArchive(props.archiveUrl); setAttempt((v) => v + 1) } catch { /* Invalid archive remains an inline error. */ } }} />
}
function MapSurface({ places, selected, archiveUrl, bounds, onSelect, onViewport, retry }: Props & { retry: () => void }) {
  const { t, locale } = useI18n(), container = useRef<HTMLDivElement>(null), mapRef = useRef<LibreMap | null>(null)
  const callbacks = useRef({ onSelect, onViewport }); const markerRefs = useRef<{ id: string; element: HTMLButtonElement }[]>([])
  const [status, setStatus] = useState<'loading' | 'ready' | 'error'>('loading')
  const { west, south, east, north } = bounds ?? region
  useEffect(() => { callbacks.current = { onSelect, onViewport } }, [onSelect, onViewport])
  useEffect(() => {
    if (!container.current) return
    let active = true, map: LibreMap | undefined
    try {
      initializeMapRuntime()
      map = new LibreMap({ container: container.current, style: mapStyle(archiveUrl, locale, window.location.origin), center: [66.888315, 48.030725], zoom: 4, maxBounds: [[region.west, region.south], [region.east, region.north]], attributionControl: false, dragRotate: false, touchPitch: false })
      mapRef.current = map
      map.addControl(new AttributionControl({ compact: false }), 'bottom-left')
      map.getCanvas().setAttribute('aria-label', t('placesMap'))
      const viewport = () => {
        if (!active || !map) return
        const b = map.getBounds()
        callbacks.current.onViewport({ west: Math.max(region.west, b.getWest()), south: Math.max(region.south, b.getSouth()), east: Math.min(region.east, b.getEast()), north: Math.min(region.north, b.getNorth()) })
      }
      map.on('moveend', viewport)
      map.on('load', () => { if (active) { setStatus('ready'); viewport() } })
      map.on('error', (event) => { if (active && !(event.error instanceof Error && event.error.name === 'AbortError')) setStatus('error') })
    } catch { queueMicrotask(() => { if (active) setStatus('error') }) }
    const observer = map && typeof ResizeObserver !== 'undefined' ? new ResizeObserver(() => map?.resize()) : undefined
    if (container.current) observer?.observe(container.current)
    return () => { active = false; observer?.disconnect(); mapRef.current = null; map?.remove() }
    // Locale recreates the style and labels; callbacks are read through the ref.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [archiveUrl, locale])
  useEffect(() => {
    const map = mapRef.current
    if (!map) return
    map.fitBounds([[west, south], [east, north]], { duration: 0, padding: 24 })
  }, [archiveUrl, locale, west, south, east, north])
  useEffect(() => {
    const map = mapRef.current
    if (!map) return
    const markers: Marker[] = []
    markerRefs.current = []
    for (const place of places) {
      const element = document.createElement('button')
      element.type = 'button'; element.className = 'place-map-marker place-category-' + place.category
      element.innerHTML = renderToStaticMarkup(<PlaceCategoryIcon category={place.category} size={20} />)
      element.setAttribute('aria-label', t('placesSelect', { name: place.name, category: t(categoryLabels[place.category]) }))
      element.title = place.name
      element.addEventListener('click', (event) => { event.stopPropagation(); callbacks.current.onSelect(place.id) })
      markerRefs.current.push({ id: place.id, element })
      markers.push(new Marker({ element }).setLngLat([place.lon, place.lat]).addTo(map))
    }
    return () => { markers.forEach((marker) => marker.remove()); markerRefs.current = [] }
    // t is recreated by the provider; locale covers label changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [places, locale])
  useEffect(() => {
    markerRefs.current.forEach(({ id, element }) => { element.setAttribute('aria-pressed', String(id === selected)); element.classList.toggle('selected', id === selected) })
    const place = places.find((p) => p.id === selected)
    if (place) mapRef.current?.easeTo({ center: [place.lon, place.lat], duration: 0 })
  }, [selected, places, locale])
  function fit() { if (!mapRef.current || !places.length) return; const b = new LngLatBounds(); places.forEach((p) => b.extend([p.lon, p.lat])); mapRef.current.fitBounds(b, { padding: 40, maxZoom: 14, duration: 0 }) }
  return <div className="map-canvas places-map" role="region" aria-label={t('placesMap')}><div className="map-canvas-surface" ref={container} /><div className="map-canvas-controls"><button type="button" aria-label={t('placesZoomIn')} onClick={() => mapRef.current?.zoomIn({ duration: 0 })}><PlusIcon size={20} /></button><button type="button" aria-label={t('placesZoomOut')} onClick={() => mapRef.current?.zoomOut({ duration: 0 })}><MinusIcon size={20} /></button><button type="button" aria-label={t('placesFit')} disabled={!places.length} onClick={fit}><ArrowsOutSimpleIcon size={20} /></button></div>{status === 'loading' && <div className="map-canvas-status" role="status"><span className="map-canvas-loading-bar" />{t('loading')}</div>}{status === 'error' && <div className="map-canvas-status map-canvas-error" role="alert">{t('placesMapUnavailable')}<button type="button" className="text-button" onClick={retry}>{t('retry')}</button></div>}</div>
}
