import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { I18nProvider, useI18n } from '../../i18n'
import type { Place } from '../../types'
import PlacesMap from './PlacesMap'
const state = vi.hoisted(() => ({ maps: [] as { handlers: Record<string, (() => void)[]>; remove: ReturnType<typeof vi.fn>; easeTo: ReturnType<typeof vi.fn>; fitBounds: ReturnType<typeof vi.fn> }[], removed: 0 }))
vi.mock('../maps/map-browser-runtime', () => ({ initializeMapRuntime: vi.fn(), retryMapArchive: vi.fn() }))
vi.mock('../maps/map-style', () => ({ mapStyle: () => ({ version: 8, sources: {}, layers: [] }) }))
vi.mock('maplibre-gl', () => ({
  Map: class {
    handlers: Record<string, (() => void)[]> = {}
    remove = vi.fn(); easeTo = vi.fn(); fitBounds = vi.fn()
    constructor() { state.maps.push(this) }
    addControl() {}
    getCanvas() { return document.createElement('canvas') }
    on(name: string, fn: () => void) { (this.handlers[name] ??= []).push(fn) }
    getBounds() { return { getWest: () => 76, getSouth: () => 43, getEast: () => 77, getNorth: () => 44 } }
    resize() {} zoomIn() {} zoomOut() {}
  },
  AttributionControl: class {},
  LngLatBounds: class { extend() { return this } },
  Marker: class {
    element: HTMLElement
    constructor({ element }: { element: HTMLElement }) { this.element = element }
    setLngLat() { return this }
    addTo() { document.body.append(this.element); return this }
    remove() { this.element.remove(); state.removed++ }
  },
}))
const places: Place[] = Array.from({ length: 60 }, (_, i) => ({ id: 'p-' + i, name: 'Workshop ' + i, category: 'workshop', lat: 43.2, lon: 76.9, city: '', address: '', description: '', website: '', phone: '', opening_hours: '', source: 'community', source_url: '', updated_at: '2026-11-05T00:00:00Z', version: 1, status: 'published' }))
function LocaleControl() { const { setLocale } = useI18n(); return <button onClick={() => setLocale('ru')}>Use Russian</button> }
describe('Places map lifecycle', () => {
  it('supports arbitrary result markers, keyboard buttons, selection and full cleanup', async () => {
    state.maps = []; state.removed = 0
    const onSelect = vi.fn(), onViewport = vi.fn()
    const { rerender, unmount } = render(<I18nProvider><PlacesMap places={places} archiveUrl="/map-data/kz.pmtiles" onSelect={onSelect} onViewport={onViewport} /></I18nProvider>)
    await waitFor(() => expect(screen.getAllByRole('button', { name: /^Select Workshop/ })).toHaveLength(60))
    const marker = screen.getByRole('button', { name: 'Select Workshop 47, Workshops' })
    marker.focus(); fireEvent.click(marker)
    expect(onSelect).toHaveBeenCalledWith('p-47')
    rerender(<I18nProvider><PlacesMap places={places} selected="p-47" archiveUrl="/map-data/kz.pmtiles" onSelect={onSelect} onViewport={onViewport} /></I18nProvider>)
    expect(marker).toHaveFocus()
    expect(marker).toHaveAttribute('aria-pressed', 'true')
    expect(state.maps[0].easeTo).toHaveBeenCalledWith({ center: [76.9, 43.2], duration: 0 })
    act(() => state.maps[0].handlers.load.forEach((fn) => fn()))
    expect(onViewport).toHaveBeenCalledWith({ west: 76, south: 43, east: 77, north: 44 })
    unmount()
    expect(state.removed).toBe(60)
    expect(state.maps[0].remove).toHaveBeenCalledOnce()
    const before = onViewport.mock.calls.length
    act(() => state.maps[0].handlers.moveend.forEach((fn) => fn()))
    expect(onViewport).toHaveBeenCalledTimes(before)
  })
  it('restores selected markers after locale recreation and follows changed URL bounds', async () => {
    state.maps = []
    const common = { places, selected: 'p-47', archiveUrl: '/map-data/kz.pmtiles', onSelect: vi.fn(), onViewport: vi.fn() }
    const { rerender } = render(<I18nProvider><PlacesMap {...common} bounds={{ west: 76, south: 43, east: 77, north: 44 }} /><LocaleControl /></I18nProvider>)
    expect(await screen.findByRole('button', { name: 'Select Workshop 47, Workshops' })).toHaveAttribute('aria-pressed', 'true')
    expect(state.maps[0].fitBounds).toHaveBeenCalledWith([[76, 43], [77, 44]], { duration: 0, padding: 24 })
    fireEvent.click(screen.getByRole('button', { name: 'Use Russian' }))
    expect(await screen.findByRole('button', { name: 'Выбрать Workshop 47, Мастерские' })).toHaveAttribute('aria-pressed', 'true')
    expect(state.maps[1].easeTo).toHaveBeenCalledWith({ center: [76.9, 43.2], duration: 0 })
    rerender(<I18nProvider><PlacesMap {...common} bounds={{ west: 70, south: 50, east: 72, north: 52 }} /><LocaleControl /></I18nProvider>)
    expect(state.maps[1].fitBounds).toHaveBeenLastCalledWith([[70, 50], [72, 52]], { duration: 0, padding: 24 })
  })

})
