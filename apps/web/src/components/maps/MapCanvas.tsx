import { useEffect, useRef, useState } from 'react'
import { ArrowsOutSimpleIcon, MinusIcon, PlusIcon } from '@phosphor-icons/react'
import { AttributionControl, LngLatBounds, Map as LibreMap, Marker } from 'maplibre-gl'
import type { GeoJSONSource } from 'maplibre-gl'
import type { Locale, MapGeometry, MapPoint } from '../../types'
import { mapStyle } from './map-style'
import { initializeMapRuntime, retryMapArchive } from './map-browser-runtime'
import 'maplibre-gl/dist/maplibre-gl.css'
import './map-canvas.css'

export interface MapCanvasProps {
  points: MapPoint[]
  geometry?: MapGeometry
  archiveUrl: string
  locale: Locale
  onPick?: (point: { lat: number; lon: number }) => void
  disabled?: boolean
}

const copy = {
  en: { label: 'Ride map', loading: 'Loading map…', error: 'The map could not be loaded.', retry: 'Try again', pick: 'Click the map to choose a point.', zoomIn: 'Zoom in', zoomOut: 'Zoom out', fit: 'Show all stops' },
  ru: { label: 'Карта поездки', loading: 'Загрузка карты…', error: 'Не удалось загрузить карту.', retry: 'Повторить', pick: 'Нажмите на карту, чтобы выбрать точку.', zoomIn: 'Приблизить', zoomOut: 'Отдалить', fit: 'Показать все остановки' },
  kk: { label: 'Сапар картасы', loading: 'Карта жүктелуде…', error: 'Картаны жүктеу мүмкін болмады.', retry: 'Қайталау', pick: 'Нүктені таңдау үшін картаны басыңыз.', zoomIn: 'Жақындату', zoomOut: 'Алыстату', fit: 'Барлық аялдамаларды көрсету' },
}

function validCoordinate(lon: number, lat: number) {
  return Number.isFinite(lon) && Number.isFinite(lat) && Math.abs(lon) <= 180 && Math.abs(lat) <= 90
}

function fitStops(map: LibreMap, points: MapPoint[], geometry?: MapGeometry) {
  const bounds = new LngLatBounds()
  for (const point of points) {
    if (validCoordinate(point.lon, point.lat)) bounds.extend([point.lon, point.lat])
  }
  for (const [lon, lat] of geometry?.coordinates ?? []) {
    if (validCoordinate(lon, lat)) bounds.extend([lon, lat])
  }
  if (!bounds.isEmpty()) map.fitBounds(bounds, { padding: 40, maxZoom: 14, duration: 0 })
}

function MapSurface({ points, geometry, archiveUrl, locale, onPick, disabled, retry }: MapCanvasProps & { retry: () => void }) {
  const container = useRef<HTMLDivElement>(null)
  const mapRef = useRef<LibreMap | null>(null)
  const loaded = useRef(false)
  const [status, setStatus] = useState<'loading' | 'ready' | 'error'>('loading')
  const text = copy[locale]

  useEffect(() => {
    const element = container.current
    if (!element) return
    let active = true
    let map: LibreMap | undefined
    try {
      initializeMapRuntime()
      map = new LibreMap({
        container: element,
        style: mapStyle(archiveUrl, locale, window.location.origin),
        center: [66.888315, 48.030725],
        zoom: 4,
        maxBounds: [[46, 40.5], [87.5, 55.5]],
        attributionControl: false,
        dragRotate: false,
        touchPitch: false,
      })
      mapRef.current = map
      map.addControl(new AttributionControl({ compact: false }), 'bottom-left')
      map.getCanvas().setAttribute('aria-label', copy[locale].label)
      map.on('load', () => {
        if (!active) return
        loaded.current = true
        setStatus((current) => current === 'error' ? current : 'ready')
      })
      map.on('error', (event) => {
        if (active && !(event.error instanceof Error && event.error.name === 'AbortError')) setStatus('error')
      })
    } catch {
      queueMicrotask(() => { if (active) setStatus('error') })
    }
    const observer = map && typeof ResizeObserver !== 'undefined'
      ? new ResizeObserver(() => map?.resize())
      : undefined
    observer?.observe(element)
    return () => {
      active = false
      observer?.disconnect()
      loaded.current = false
      mapRef.current = null
      map?.remove()
    }
  }, [archiveUrl, locale])

  useEffect(() => {
    const map = mapRef.current
    if (!map) return
    const markers: Marker[] = []
    let active = true
    const update = () => {
      if (!active) return
      const data = geometry
        ? { type: 'Feature' as const, geometry, properties: {} }
        : { type: 'FeatureCollection' as const, features: [] }
      const source = map.getSource('ride-route') as GeoJSONSource | undefined
      if (source) source.setData(data)
      else {
        map.addSource('ride-route', { type: 'geojson', data })
        const belowLabels = map.getStyle().layers.find((layer) => layer.type === 'symbol')?.id
        map.addLayer({ id: 'ride-route-casing', type: 'line', source: 'ride-route', layout: { 'line-cap': 'round', 'line-join': 'round' }, paint: { 'line-color': '#181818', 'line-width': 8 } }, belowLabels)
        map.addLayer({ id: 'ride-route-line', type: 'line', source: 'ride-route', layout: { 'line-cap': 'round', 'line-join': 'round' }, paint: { 'line-color': '#d2ed75', 'line-width': 4 } }, belowLabels)
      }
      for (const [index, point] of points.entries()) {
        if (!validCoordinate(point.lon, point.lat)) continue
        const element = document.createElement('div')
        element.className = 'map-stop-marker'
        element.textContent = String(index + 1)
        element.title = `${index + 1}. ${point.label}`
        element.setAttribute('role', 'img')
        element.setAttribute('aria-label', element.title)
        element.addEventListener('click', (event) => event.stopPropagation())
        markers.push(new Marker({ element }).setLngLat([point.lon, point.lat]).addTo(map))
      }
      fitStops(map, points, geometry)
    }
    if (loaded.current) update()
    else map.once('load', update)
    return () => {
      active = false
      map.off('load', update)
      markers.forEach((marker) => marker.remove())
    }
  }, [points, geometry])

  useEffect(() => {
    const map = mapRef.current
    if (!map || !onPick || disabled) return
    const pick = (event: { lngLat: { lat: number; lng: number } }) => onPick({ lat: event.lngLat.lat, lon: event.lngLat.lng })
    map.on('click', pick)
    map.getCanvas().style.cursor = 'crosshair'
    return () => {
      map.off('click', pick)
      map.getCanvas().style.cursor = ''
    }
  }, [onPick, disabled])

  return <div className="map-canvas" role="region" aria-label={text.label}>
    <div ref={container} className="map-canvas-surface" />
    <div className="map-canvas-controls" role="group" aria-label={text.label}>
      <button type="button" aria-label={text.zoomIn} title={text.zoomIn} onClick={() => mapRef.current?.zoomIn({ duration: 0 })}><PlusIcon size={20} /></button>
      <button type="button" aria-label={text.zoomOut} title={text.zoomOut} onClick={() => mapRef.current?.zoomOut({ duration: 0 })}><MinusIcon size={20} /></button>
      <button type="button" aria-label={text.fit} title={text.fit} disabled={!points.length && !geometry?.coordinates.length} onClick={() => { if (mapRef.current) fitStops(mapRef.current, points, geometry) }}><ArrowsOutSimpleIcon size={20} /></button>
    </div>
    {status === 'loading' && <div className="map-canvas-status" role="status"><span className="map-canvas-loading-bar" aria-hidden="true" />{text.loading}</div>}
    {status === 'error' && <div className="map-canvas-status map-canvas-error" role="alert"><span>{text.error}</span><button type="button" className="text-button" onClick={retry}>{text.retry}</button></div>}
    {onPick && !disabled && <p className="map-canvas-pick-hint">{text.pick}</p>}
  </div>
}

export default function MapCanvas(props: MapCanvasProps) {
  const [attempt, setAttempt] = useState(0)
  const retry = () => {
    try {
      retryMapArchive(props.archiveUrl)
    } catch {
      return
    }
    setAttempt((current) => current + 1)
  }
  return <MapSurface key={`${props.archiveUrl}:${props.locale}:${attempt}`} {...props} retry={retry} />
}
