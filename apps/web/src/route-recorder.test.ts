import { afterEach, describe, expect, it, vi } from 'vitest'
import { bindRecorderAccount, recorderFor, TrackRecorder } from './route-recorder'

function harness() {
  let now = Date.UTC(2026, 10, 11, 6)
  const callbacks: { success: PositionCallback; error?: PositionErrorCallback | null }[] = []
  const geo: Geolocation = {
    getCurrentPosition: vi.fn(),
    watchPosition: vi.fn((success, error) => { callbacks.push({ success, error }); return callbacks.length }),
    clearWatch: vi.fn(),
  }
  const values = new Map<string, string>()
  const storage = { getItem: (key: string) => values.get(key) ?? null, setItem: (key: string, value: string) => { values.set(key, value) }, removeItem: (key: string) => { values.delete(key) } }
  const recorder = new TrackRecorder('rider-a', geo, storage, () => now)
  function point(lon: number, lat = 43.25, accuracy = 5, timestamp = now, watcher = callbacks.length - 1) {
    callbacks[watcher].success({ timestamp, coords: { longitude: lon, latitude: lat, accuracy, altitude: null, altitudeAccuracy: null, heading: null, speed: null, toJSON: () => ({}) }, toJSON: () => ({}) })
  }
  return { recorder, geo, values, storage, callbacks, point, advance: (ms: number) => { now += ms }, now: () => now }
}
afterEach(() => { bindRecorderAccount(null); vi.unstubAllGlobals() })

describe('private GPS recorder', () => {
  it('requests location only after Start and persists normalized coordinates without device metadata', () => {
    const h = harness()
    expect(h.geo.watchPosition).not.toHaveBeenCalled()
    h.recorder.name('Sunday ride')
    expect(h.geo.watchPosition).not.toHaveBeenCalled()
    h.recorder.start()
    expect(h.geo.watchPosition).toHaveBeenCalledWith(expect.any(Function), expect.any(Function), { enableHighAccuracy: true, maximumAge: 0, timeout: 15000 })
    h.point(76.95); h.advance(10000); h.point(76.951)
    const input = h.recorder.saveInput()
    expect(input).toEqual({ name: 'Sunday ride', segments: [[[76.95, 43.25], [76.951, 43.25]]], duration_seconds: 10 })
    const stored = [...h.values.values()][0]
    expect(stored).not.toMatch(/timestamp|accuracy|altitude|heading|speed/)
    expect(h.geo.clearWatch).toHaveBeenCalledWith(1)
  })

  it('keeps pause and suspension gaps separate and excludes paused time', () => {
    const h = harness(); h.recorder.start()
    h.point(76.95); h.advance(10000); h.point(76.951)
    h.recorder.pause(); h.advance(60000)
    h.point(76.96, 43.25, 5, h.now(), 0)
    expect(h.recorder.snapshot().segments[0]).toHaveLength(2)
    h.recorder.start(); h.point(76.97); h.advance(10000); h.point(76.971)
    h.advance(40000); h.point(76.98); h.advance(10000); h.point(76.981)
    const saved = h.recorder.saveInput()
    expect(saved.segments).toHaveLength(3)
    expect(saved.segments.map((s) => s.length)).toEqual([2, 2, 2])
    expect(saved.duration_seconds).toBe(70)
  })

  it('rejects inaccurate, stale, out of region and implausible GPS fixes without losing slow movement', () => {
    const h = harness(); h.recorder.start()
    h.point(76.95, 43.25, 150)
    h.point(76.95, 43.25, 5, h.now() - 31000)
    expect(h.recorder.snapshot().segments[0]).toHaveLength(0)
    h.point(76.95); h.advance(1000); h.point(80)
    expect(h.recorder.snapshot().segments[0]).toHaveLength(1)
    h.point(76.95002); h.advance(1000); h.point(76.95004)
    expect(h.recorder.snapshot().segments[0]).toHaveLength(2)
    h.point(10)
    expect(h.recorder.snapshot().error).toBe('outside')
    expect(h.recorder.snapshot().segments[0]).toHaveLength(2)
  })

  it('stops on denied permission and ignores late replies from a previous watch', () => {
    const h = harness(); h.recorder.start()
    h.callbacks[0].error?.({ code: 1, message: 'denied', PERMISSION_DENIED: 1, POSITION_UNAVAILABLE: 2, TIMEOUT: 3 })
    expect(h.recorder.snapshot().status).toBe('paused')
    expect(h.recorder.snapshot().error).toBe('permission')
    expect(h.geo.clearWatch).toHaveBeenCalledWith(1)
    h.recorder.start(); h.point(76.95)
    h.point(76.96, 43.25, 5, h.now() + 10000, 0)
    expect(h.recorder.snapshot().segments.flat()).toEqual([[76.95, 43.25]])
  })

  it('recovers a draft paused and never requests location during recovery', () => {
    const h = harness(); h.recorder.name('Recovered route'); h.recorder.start(); h.point(76.95); h.advance(10000); h.point(76.951); h.recorder.pause()
    const recovered = new TrackRecorder('rider-a', h.geo, h.storage, h.now)
    expect(recovered.snapshot().status).toBe('paused')
    expect(recovered.snapshot().name).toBe('Recovered route')
    expect(h.geo.watchPosition).toHaveBeenCalledTimes(1)
    expect(new TrackRecorder('rider-b', h.geo, h.storage, h.now).snapshot().segments).toEqual([])
    h.values.set('bikerskz.route-draft.rider-a', '{"version":1,"actor":"rider-b","name":"secret","duration_seconds":2,"segments":[[[76,43],[77,43]]]}')
    expect(new TrackRecorder('rider-a', h.geo, h.storage, h.now).snapshot().segments).toEqual([])
  })

  it('keeps an in-memory recording when draft storage fills', () => {
    const h = harness()
    const recorder = new TrackRecorder('a', h.geo, { getItem: () => null, setItem: () => { throw new Error('quota') }, removeItem: () => {} }, h.now)
    recorder.start(); h.point(76.95); h.advance(10000); h.point(76.951)
    expect(recorder.snapshot().error).toBe('storage')
    expect(recorder.saveInput().segments.flat()).toHaveLength(2)
    recorder.clear()
    expect(recorder.snapshot().segments).toEqual([])
  })

  it('stops GPS and deletes the previous actor draft on account change or logout', () => {
    const h = harness()
    vi.stubGlobal('navigator', { geolocation: h.geo }); vi.stubGlobal('localStorage', h.storage)
    bindRecorderAccount(null)
    const original = recorderFor('rider-a'); original.start(); h.point(76.95)
    expect(h.values.has('bikerskz.route-draft.rider-a')).toBe(true)
    bindRecorderAccount('rider-b')
    expect(h.geo.clearWatch).toHaveBeenCalledWith(1)
    expect(h.values.has('bikerskz.route-draft.rider-a')).toBe(false)
    expect(recorderFor('rider-b').snapshot().segments).toEqual([])
    bindRecorderAccount(null)
    h.point(76.96, 43.25, 5, h.now(), 0)
    expect(h.values.has('bikerskz.route-draft.rider-a')).toBe(false)
  })

  it('removes a prior actor draft after reload even if the recorder page was never opened', () => {
    const h = harness(); vi.stubGlobal('localStorage', h.storage)
    bindRecorderAccount('rider-a')
    h.values.set('bikerskz.route-draft.rider-a', 'private existing coordinates')
    bindRecorderAccount('rider-b')
    expect(h.values.has('bikerskz.route-draft.rider-a')).toBe(false)
  })

  it('stops at the active-duration limit and keeps the recording saveable', () => {
    const h = harness(); h.recorder.start(); h.point(76.95); h.advance(10000); h.point(76.951)
    h.advance(604800000); h.recorder.tick()
    expect(h.recorder.snapshot().status).toBe('paused')
    expect(h.recorder.snapshot().error).toBe('limit')
    expect(h.recorder.saveInput().duration_seconds).toBe(604800)
  })

  it('never clears a newer draft when an earlier recording save completes', () => {
    const h = harness(); h.recorder.start(); h.point(76.95); h.advance(10000); h.point(76.951)
    h.recorder.saveInput(); const saved = h.recorder.revision()
    h.recorder.start(); h.point(76.96); h.advance(10000); h.point(76.961)
    expect(h.recorder.clearSaved(saved)).toBe(false)
    expect(h.recorder.snapshot().segments.flat()).toHaveLength(4)
    const current = h.recorder.revision()
    expect(h.recorder.clearSaved(current)).toBe(true)
    expect(h.recorder.snapshot().segments).toEqual([])
  })
})
