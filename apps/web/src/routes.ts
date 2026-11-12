import type { MapPoint } from './types'

export type TrackPoint = [number, number]
export type TrackSegments = TrackPoint[][]
export const routeSurfaces = ['unknown', 'asphalt', 'mixed', 'unpaved'] as const
export type RouteSurface = typeof routeSurfaces[number]
export interface CommunityRouteSummary {
  id: string; owner_id: string; owner_name: string; title: string; city: string; description: string
  surface: RouteSurface; kind: 'planned' | 'recorded'; published: boolean; version: number
  created_at: string; updated_at: string; distance_meters: number; duration_seconds: number
}
export interface CommunityRoute extends CommunityRouteSummary { points: MapPoint[]; segments: TrackSegments }
export interface RecordedTrack { id: string; name: string; segments: TrackSegments; distance_meters: number; duration_seconds: number; version: number; created_at: string }
export interface RouteCollection { id: string; name: string; version: number; route_count: number }
export const conditionKinds = ['gravel', 'closure', 'roadworks', 'damage', 'other'] as const
export type ConditionKind = typeof conditionKinds[number]
export interface RouteCondition {
  id: string; route_id: string; author_id: string; author_name: string; kind: ConditionKind
  description: string; observed_at: string; updated_at: string; expires_at: string; resolved: boolean
  version: number; state: 'recent' | 'expired' | 'resolved'
}
export interface RoutesPageData { routes: CommunityRouteSummary[]; next_cursor: string }
export interface TracksPageData { tracks: RecordedTrack[]; next_cursor: string }
export interface CollectionsPageData { collections: RouteCollection[]; next_cursor: string }
export interface ConditionsPageData { conditions: RouteCondition[]; next_cursor: string }

// A list never retains a private recording, geometry or hidden track reference.
export function routeSummary(route: CommunityRouteSummary): CommunityRouteSummary {
  const { id, owner_id, owner_name, title, city, description, surface, kind, published, version, created_at, updated_at, distance_meters, duration_seconds } = route
  return { id, owner_id, owner_name, title, city, description, surface, kind, published, version, created_at, updated_at, distance_meters, duration_seconds }
}
export function trackDistance(segments: TrackSegments) {
  let meters = 0
  for (const segment of segments) for (let i = 1; i < segment.length; i++) meters += pointDistance(segment[i - 1], segment[i])
  return meters
}
export function pointDistance(a: TrackPoint, b: TrackPoint) {
  const radians = Math.PI / 180
  const lat = Math.sin((b[1] - a[1]) * radians / 2) ** 2
  const lon = Math.cos(a[1] * radians) * Math.cos(b[1] * radians) * Math.sin((b[0] - a[0]) * radians / 2) ** 2
  return 6371000 * 2 * Math.asin(Math.min(1, Math.sqrt(lat + lon)))
}
export function validTrackPoint(point: unknown): point is TrackPoint {
  return Array.isArray(point) && point.length === 2 && point.every(Number.isFinite) && point[0] >= 46 && point[0] <= 87.5 && point[1] >= 40.5 && point[1] <= 55.5
}
export const byteLength = (value: string) => new TextEncoder().encode(value.trim()).length
