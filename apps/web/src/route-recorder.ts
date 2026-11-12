import { pointDistance, validTrackPoint } from './routes'
import type { TrackPoint, TrackSegments } from './routes'

export const maxTrackPoints = 20000
export const maxTrackDuration = 604800
const maxSegments = 256
const draftKey = (actor: string) => `bikerskz.route-draft.${actor}`
type RecorderError = 'permission' | 'unavailable' | 'timeout' | 'outside' | 'limit' | 'storage' | ''
export interface RecordingSnapshot {
  name: string; segments: TrackSegments; duration_seconds: number; status: 'idle' | 'recording' | 'paused'
  last_fix_at: number; checked_at: number; error: RecorderError
}
interface StoredDraft { version: number; actor: string; name: string; segments: TrackSegments; duration_seconds: number }

// Only coordinates and total active duration are retained. Device metadata,
// per-point timestamps and accuracy values never enter the saved recording.
export class TrackRecorder {
  private value: RecordingSnapshot = { name: '', segments: [], duration_seconds: 0, status: 'idle', last_fix_at: 0, checked_at: 0, error: '' }
  private listeners = new Set<() => void>()
  private watch: number | undefined
  private generation = 0
  private changes = 0
  private started = 0
  private elapsed = 0
  private lastPosition: { point: TrackPoint; timestamp: number } | undefined
  constructor(readonly actor: string, private geo: Geolocation | undefined, private storage: Pick<Storage, 'getItem' | 'setItem' | 'removeItem'> | undefined, private now = Date.now) {
    try {
      const raw = storage?.getItem(draftKey(actor))
      if (!raw || raw.length > 1600000) return
      const input: StoredDraft = JSON.parse(raw)
      if (input.version !== 1 || input.actor !== actor || typeof input.name !== 'string' || input.name.length > 200 || !Number.isFinite(input.duration_seconds) || input.duration_seconds < 0 || input.duration_seconds > 604800 || !Array.isArray(input.segments) || input.segments.length > maxSegments || input.segments.some((s) => !Array.isArray(s) || s.some((p) => !validTrackPoint(p))) || input.segments.reduce((n, s) => n + s.length, 0) > maxTrackPoints) return
      this.elapsed = input.duration_seconds
      this.value = { ...this.value, name: input.name, segments: input.segments, duration_seconds: input.duration_seconds, status: 'paused' }
    } catch { /* A damaged or inaccessible local draft does not start GPS. */ }
  }
  snapshot = () => this.value
  revision = () => this.changes
  subscribe = (listener: () => void) => { this.listeners.add(listener); return () => { this.listeners.delete(listener) } }
  private emit(persist = true) {
    ++this.changes
    this.value = { ...this.value, checked_at: this.now(), duration_seconds: this.elapsed + (this.value.status === 'recording' ? Math.max(0, this.now() - this.started) / 1000 : 0) }
    if (persist) {
      try {
        if (!this.storage) throw new Error('Storage unavailable')
        this.storage.setItem(draftKey(this.actor), JSON.stringify({ version: 1, actor: this.actor, name: this.value.name, segments: this.value.segments, duration_seconds: this.value.duration_seconds }))
      } catch { this.value = { ...this.value, error: 'storage' } }
    }
    this.listeners.forEach((listener) => listener())
  }
  name = (name: string) => { this.value = { ...this.value, name }; this.emit() }
  tick = () => {
    if (this.value.status !== 'recording') return
    if (this.elapsed + Math.max(0, this.now() - this.started) / 1000 >= maxTrackDuration) {
      this.pause(); this.value = { ...this.value, error: 'limit' }; this.emit()
    } else this.emit()
  }
  start = () => {
    if (this.value.status === 'recording') return
    if (!this.geo) { this.value = { ...this.value, error: 'unavailable' }; this.emit(false); return }
    if (this.elapsed >= maxTrackDuration || this.value.segments.length >= maxSegments || this.value.segments.reduce((n, s) => n + s.length, 0) >= maxTrackPoints) { this.value = { ...this.value, error: 'limit' }; this.emit(); return }
    this.started = this.now()
    this.lastPosition = undefined
    this.value = { ...this.value, status: 'recording', error: '', last_fix_at: 0, segments: [...this.value.segments.filter((s) => s.length), []] }
    const generation = ++this.generation
    this.emit()
    try {
      this.watch = this.geo.watchPosition((position) => {
        if (generation !== this.generation || this.value.status !== 'recording') return
        const { latitude: lat, longitude: lon, accuracy } = position.coords
        const timestamp = position.timestamp
        if (!validTrackPoint([lon, lat])) { this.value = { ...this.value, error: 'outside' }; this.emit(); return }
        if (!Number.isFinite(accuracy) || accuracy < 0 || accuracy > 100 || !Number.isFinite(timestamp) || Math.abs(this.now() - timestamp) > 30000) return
        const point: TrackPoint = [lon, lat]
        const previous = this.lastPosition
        if (previous && timestamp <= previous.timestamp) return
        const seconds = previous ? (timestamp - previous.timestamp) / 1000 : 0
        const distance = previous ? pointDistance(previous.point, point) : 0
        if (previous && seconds <= 30 && distance / seconds > 90) return
        const segments = this.value.segments.map((s) => [...s])
        if (previous && seconds > 30) {
          if (segments.length >= maxSegments) { this.pause(); this.value = { ...this.value, error: 'limit' }; this.emit(); return }
          segments.push([])
        }
        const segment = segments[segments.length - 1]
        if (!segment.length || pointDistance(segment[segment.length - 1], point) >= 3) segment.push(point)
        this.lastPosition = { point, timestamp }
        this.value = { ...this.value, segments, last_fix_at: this.now(), error: '' }
        if (segments.reduce((n, s) => n + s.length, 0) >= maxTrackPoints) { this.pause(); this.value = { ...this.value, error: 'limit' } }
        this.emit()
      }, (error) => {
        if (generation !== this.generation || this.value.status !== 'recording') return
        if (error.code === 1) this.pause()
        this.value = { ...this.value, error: error.code === 1 ? 'permission' : error.code === 3 ? 'timeout' : 'unavailable' }
        this.emit()
      }, { enableHighAccuracy: true, maximumAge: 0, timeout: 15000 })
      if (generation !== this.generation && this.watch !== undefined) {
        this.geo.clearWatch(this.watch)
        this.watch = undefined
      }
    } catch { this.pause(); this.value = { ...this.value, error: 'unavailable' }; this.emit() }
  }
  pause = () => {
    ++this.generation
    if (this.watch !== undefined) this.geo?.clearWatch(this.watch)
    this.watch = undefined
    if (this.value.status === 'idle') return
    if (this.value.status === 'recording') this.elapsed = Math.min(maxTrackDuration, this.elapsed + Math.max(0, this.now() - this.started) / 1000)
    this.value = { ...this.value, status: 'paused' }
    this.emit()
  }
  clear = () => {
    this.pause()
    this.elapsed = 0
    this.value = { name: '', segments: [], duration_seconds: 0, status: 'idle', last_fix_at: 0, checked_at: 0, error: '' }
    try { this.storage?.removeItem(draftKey(this.actor)) } catch { /* Keep the in-memory recording cleared. */ }
    this.emit(false)
  }
  clearSaved = (revision: number) => {
    if (revision !== this.changes) return false
    this.clear()
    return true
  }
  saveInput = () => {
    this.pause()
    return { name: this.value.name.trim(), segments: this.value.segments.filter((s) => s.length >= 2), duration_seconds: Math.max(1, Math.round(this.value.duration_seconds)) }
  }
}

let boundActor: string | null | undefined
const recorders = new Map<string, TrackRecorder>()
export function bindRecorderAccount(actor: string | null) {
  for (const [owner, recorder] of recorders) {
    if (owner === actor) continue
    recorder.clear()
    recorders.delete(owner)
  }
  if (boundActor && boundActor !== actor) {
    try { localStorage.removeItem(draftKey(boundActor)) } catch { /* Storage can be disabled by the browser. */ }
  }
  boundActor = actor
}
export function recorderFor(actor: string) {
  let recorder = recorders.get(actor)
  if (!recorder) {
    let storage: Storage | undefined
    try { storage = localStorage } catch { /* Recording remains available without persistence. */ }
    recorder = new TrackRecorder(actor, navigator.geolocation, storage)
    recorders.set(actor, recorder)
  }
  return recorder
}
