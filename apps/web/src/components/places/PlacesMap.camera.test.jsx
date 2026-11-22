import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
// Exercise the pinned MapLibre camera and Mercator transform without WebGL.
// JavaScript keeps package-internal TS sources out of the app's typecheck.
import { Camera } from '../../../node_modules/maplibre-gl/src/ui/camera.ts'
import { LngLatBounds } from '../../../node_modules/maplibre-gl/src/geo/lng_lat_bounds.ts'
import { LngLat } from '../../../node_modules/maplibre-gl/src/geo/lng_lat.ts'
import { I18nProvider, useI18n } from '../../i18n'
import { region, validBounds } from '../../places'
import MapCanvas from '../maps/MapCanvas'
import PlacesMap from './PlacesMap'

const state = vi.hoisted(() => ({ maps: [], viewport: undefined }))
vi.mock('../maps/map-browser-runtime', () => ({ initializeMapRuntime: vi.fn(), retryMapArchive: vi.fn() }))
vi.mock('../maps/map-style', () => ({ mapStyle: () => ({ version: 8, sources: {}, layers: [] }) }))
vi.mock('maplibre-gl', async () => {
  const { Camera } = await import('../../../node_modules/maplibre-gl/src/ui/camera.ts')
  const { LngLatBounds } = await import('../../../node_modules/maplibre-gl/src/geo/lng_lat_bounds.ts')
  return {
    Map: class extends Camera {
      constructor(options) {
        super({ minZoom: 0, maxZoom: 22, minPitch: 0, maxPitch: 60, bearingSnap: 0, zoomSnap: 0, centerClampedToGround: false, terrain: null, transformConstrain: null, transformCameraUpdate: null, requestRenderFrame: () => 0, cancelRenderFrame: () => {}, ...options })
        this.transform.resize(390, 480)
        if (options.maxBounds) this.transform.setMaxBounds(new LngLatBounds(options.maxBounds))
        this.jumpTo({ center: options.center, zoom: options.zoom })
        this.container = options.container
        this.styleSpec = options.style
        this.canvas = document.createElement('canvas')
        state.maps.push(this)
        queueMicrotask(() => this.fire('load'))
      }
      addSource(_id, options) { this.source = { data: options.data, setData: async (data) => { this.source.data = data } } }
      getSource() { return this.source }
      addLayer() {}
      isSourceLoaded() { return !!this.source }
      querySourceFeatures() { return this.source?.data.features ?? [] }
      addControl() {}
      getCanvas() { return this.canvas }
      getContainer() { return this.container }
      getStyle() { return this.styleSpec }
      getBounds() { return state.viewport ?? this.transform.getBounds() }
      remove() {}
      resize() {}
    },
    AttributionControl: class {},
    LngLatBounds,
    Marker: class {
      constructor({ element }) { this.element = element }
      setLngLat() { return this }
      addTo() { document.body.append(this.element); return this }
      remove() { this.element.remove() }
    },
  }
})
const corners = [[region.west, region.south], [region.east, region.north]]
const places = corners.map(([lon, lat], i) => ({ id: 'p' + i, name: 'Stop ' + i, category: 'fuel', lat, lon, city: '', address: '', description: '', website: '', phone: '', opening_hours: '', source: 'community', source_url: '', updated_at: '2026-11-05', version: 1, status: 'published' }))
function expectPilotView(map) {
  expect(map.getCenter().lng).toBeCloseTo(71.43316394219903, 10)
  expect(map.getCenter().lat).toBeCloseTo(51.11307657146983, 10)
  expect(map.getZoom()).toBe(11.65)
}
function expectVisible(map, points, padding) {
  points.forEach(([lon, lat]) => {
    const p = map.transform.locationToScreenPoint(new LngLat(lon, lat))
    expect(p.x).toBeGreaterThanOrEqual(padding - 0.01)
    expect(p.x).toBeLessThanOrEqual(390 - padding + 0.01)
    expect(p.y).toBeGreaterThanOrEqual(padding - 0.01)
    expect(p.y).toBeLessThanOrEqual(480 - padding + 0.01)
  })
}
function LocaleControl() { const { setLocale } = useI18n(); return <button onClick={() => setLocale('ru')}>Use Russian</button> }
beforeEach(() => { state.maps = []; state.viewport = undefined })
describe('Shared ride, route, and pin canvas uses the actual Mercator camera', () => {
  it('opens on the pilot view without a pin and fits a chosen point', async () => {
    const onPick = vi.fn()
    const common = { archiveUrl: '/map-data/kz.pmtiles', locale: 'en', onPick }
    const { rerender } = render(<MapCanvas {...common} points={[]} />)
    const map = state.maps[0]
    await waitFor(() => expect(screen.queryByText('Loading map…')).not.toBeInTheDocument())
    expectPilotView(map)
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
    expect(onPick).not.toHaveBeenCalled()

    const point = { label: 'Almaty stop', lon: 76.95, lat: 43.25 }
    rerender(<MapCanvas {...common} points={[point]} />)
    expect(screen.getByRole('img', { name: '1. Almaty stop' })).toBeInTheDocument()
    expect(map.getCenter().lng).toBeCloseTo(point.lon, 10)
    expect(map.getCenter().lat).toBeCloseTo(point.lat, 10)
    expect(map.getZoom()).toBe(14)
  })
})
describe('Places map uses the actual Mercator camera', () => {
  it('opens on the pilot view without requiring any community places', () => {
    render(<I18nProvider><PlacesMap places={[]} archiveUrl="/map-data/kz.pmtiles" onSelect={() => {}} onViewport={() => {}} /></I18nProvider>)
    expectPilotView(state.maps[0])
  })
  it.each([
    ['city filter', { center: { lat: 43.25249, lon: 76.9115 } }],
    ['search area', { bounds: region }],
  ])('restores the pilot view when clearing the %s', (_label, cameraProps) => {
    const common = { places: [], archiveUrl: '/map-data/kz.pmtiles', onSelect: () => {}, onViewport: () => {} }
    const { rerender } = render(<I18nProvider><PlacesMap {...common} {...cameraProps} /></I18nProvider>)
    const map = state.maps[0]
    expect(map.getZoom()).toBeLessThan(11.65)
    rerender(<I18nProvider><PlacesMap {...common} /></I18nProvider>)
    expectPilotView(map)
  })
  it('focuses a chosen city, retains manual panning on result refresh and lets a selected place take priority', () => {
    const common = { archiveUrl: '/map-data/kz.pmtiles', onSelect: () => {}, onViewport: () => {} }
    const { rerender } = render(<I18nProvider><PlacesMap {...common} places={[]} center={{ lat: 43.25249, lon: 76.9115 }} /></I18nProvider>)
    const map = state.maps[0]
    expect(map.transform.center.lng).toBeCloseTo(76.9115)
    expect(map.transform.center.lat).toBeCloseTo(43.25249)
    expect(map.getZoom()).toBe(11)
    map.jumpTo({ center: [76.95, 43.3], zoom: 13 })
    rerender(<I18nProvider><PlacesMap {...common} places={[...places]} center={{ lat: 43.25249, lon: 76.9115 }} /></I18nProvider>)
    expect(map.transform.center.lng).toBeCloseTo(76.95)
    expect(map.transform.center.lat).toBeCloseTo(43.3)
    expect(map.getZoom()).toBe(13)
    rerender(<I18nProvider><PlacesMap {...common} places={places} selected="p0" center={{ lat: 43.25249, lon: 76.9115 }} /></I18nProvider>)
    expect(map.transform.center.lng).toBeCloseTo(region.west)
    expect(map.transform.center.lat).toBeCloseTo(region.south)
  })
  it('preserves explicit country bounds and fits distant results on a narrow viewport', () => {
    render(<I18nProvider><PlacesMap places={places} bounds={region} center={{ lat: 43.25249, lon: 76.9115 }} archiveUrl="/map-data/kz.pmtiles" onSelect={() => {}} onViewport={() => {}} /></I18nProvider>)
    const map = state.maps[0]
    expect(map).toBeInstanceOf(Camera)
    expectVisible(map, corners, 24)
    fireEvent.click(screen.getByRole('button', { name: 'Show these places' }))
    expectVisible(map, corners, 40)

    // Prove this assertion detects the previous maxBounds regression.
    map.transform.setMaxBounds(new LngLatBounds(corners))
    map.fitBounds(new LngLatBounds(corners), { duration: 0, padding: 40 })
    const west = map.transform.locationToScreenPoint(new LngLat(...corners[0]))
    const east = map.transform.locationToScreenPoint(new LngLat(...corners[1]))
    expect(west.x < 0 || east.x > 390).toBe(true)
  })
  it('keeps an opened place centered when city metadata arrives after its details', () => {
    const common = { places, selected: 'p0', archiveUrl: '/map-data/kz.pmtiles', onSelect: () => {}, onViewport: () => {} }
    const { rerender } = render(<I18nProvider><PlacesMap {...common} /></I18nProvider>)
    const map = state.maps[0]
    rerender(<I18nProvider><PlacesMap {...common} center={{ lat: 43.25249, lon: 76.9115 }} /></I18nProvider>)
    expect(map.transform.center.lng).toBeCloseTo(region.west)
    expect(map.transform.center.lat).toBeCloseTo(region.south)
    rerender(<I18nProvider><PlacesMap {...common} center={{ lat: 43.25249, lon: 76.9115 }} bounds={{ west: 76, south: 43, east: 77, north: 44 }} /></I18nProvider>)
    expect(map.transform.center.lng).toBeCloseTo(region.west)
    expect(map.transform.center.lat).toBeCloseTo(region.south)
  })
  it('preserves manual panning within an explicit area when lower-priority city metadata arrives', () => {
    const common = { places: [], bounds: region, archiveUrl: '/map-data/kz.pmtiles', onSelect: () => {}, onViewport: () => {} }
    const { rerender } = render(<I18nProvider><PlacesMap {...common} /></I18nProvider>)
    const map = state.maps[0]
    map.jumpTo({ center: [71.45, 51.18], zoom: 13 })
    rerender(<I18nProvider><PlacesMap {...common} center={{ lat: 43.25249, lon: 76.9115 }} /></I18nProvider>)
    expect(map.transform.center.lng).toBeCloseTo(71.45)
    expect(map.transform.center.lat).toBeCloseTo(51.18)
    expect(map.getZoom()).toBe(13)
  })
  it('keeps the selected marker and its camera center after locale recreation', async () => {
    render(<I18nProvider><PlacesMap places={places} selected="p0" archiveUrl="/map-data/kz.pmtiles" onSelect={() => {}} onViewport={() => {}} /><LocaleControl /></I18nProvider>)
    expect(state.maps[0].transform.center.lng).toBeCloseTo(region.west)
    expect(state.maps[0].transform.center.lat).toBeCloseTo(region.south)
    fireEvent.click(screen.getByRole('button', { name: 'Use Russian' }))
    expect(state.maps).toHaveLength(2)
    expect(state.maps[1].transform.center.lng).toBeCloseTo(region.west)
    expect(state.maps[1].transform.center.lat).toBeCloseTo(region.south)
    expect(await screen.findByRole('button', { name: 'Выбрать Stop 0, Заправки' })).toHaveAttribute('aria-pressed', 'true')
  })
  it('intersects an expanded viewport with coverage and invalidates an outside viewport', () => {
    const onViewport = vi.fn()
    render(<I18nProvider><PlacesMap places={[]} archiveUrl="/map-data/kz.pmtiles" onSelect={() => {}} onViewport={onViewport} /></I18nProvider>)
    const map = state.maps[0]
    state.viewport = new LngLatBounds([[40, 35], [90, 60]])
    act(() => map.fire('moveend'))
    expect(onViewport).toHaveBeenLastCalledWith(region)
    expect(validBounds(onViewport.mock.calls.at(-1)[0])).toBe(true)
    state.viewport = new LngLatBounds([[100, 20], [120, 30]])
    act(() => map.fire('moveend'))
    expect(validBounds(onViewport.mock.calls.at(-1)[0])).toBe(false)
  })
})
