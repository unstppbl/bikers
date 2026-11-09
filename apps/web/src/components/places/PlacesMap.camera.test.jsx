import { act, fireEvent, render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
// Exercise the pinned MapLibre camera and Mercator transform without WebGL.
// JavaScript keeps package-internal TS sources out of the app's typecheck.
import { Camera } from '../../../node_modules/maplibre-gl/src/ui/camera.ts'
import { LngLatBounds } from '../../../node_modules/maplibre-gl/src/geo/lng_lat_bounds.ts'
import { LngLat } from '../../../node_modules/maplibre-gl/src/geo/lng_lat.ts'
import { I18nProvider, useI18n } from '../../i18n'
import { region, validBounds } from '../../places'
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
        this.canvas = document.createElement('canvas')
        state.maps.push(this)
      }
      addControl() {}
      getCanvas() { return this.canvas }
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
describe('Places map uses the actual Mercator camera', () => {
  it('shows the full country initially and fits distant results on a narrow viewport', () => {
    render(<I18nProvider><PlacesMap places={places} archiveUrl="/map-data/kz.pmtiles" onSelect={() => {}} onViewport={() => {}} /></I18nProvider>)
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
  it('keeps the selected marker and its camera center after locale recreation', () => {
    render(<I18nProvider><PlacesMap places={places} selected="p0" archiveUrl="/map-data/kz.pmtiles" onSelect={() => {}} onViewport={() => {}} /><LocaleControl /></I18nProvider>)
    expect(state.maps[0].transform.center.lng).toBeCloseTo(region.west)
    expect(state.maps[0].transform.center.lat).toBeCloseTo(region.south)
    fireEvent.click(screen.getByRole('button', { name: 'Use Russian' }))
    expect(state.maps).toHaveLength(2)
    expect(state.maps[1].transform.center.lng).toBeCloseTo(region.west)
    expect(state.maps[1].transform.center.lat).toBeCloseTo(region.south)
    expect(screen.getByRole('button', { name: 'Выбрать Stop 0, Заправки' })).toHaveAttribute('aria-pressed', 'true')
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
