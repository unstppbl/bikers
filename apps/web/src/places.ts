import type { Place, PlaceBounds, PlaceCategory, PlaceInput } from './types'
export const placeCategories: PlaceCategory[] = ['fuel', 'workshop', 'tires', 'motorcycle_shop', 'parking', 'meeting_spot']
export const categoryLabels = { fuel: 'placesFuel', workshop: 'placesWorkshop', tires: 'placesTires', motorcycle_shop: 'placesShop', parking: 'placesParking', meeting_spot: 'placesMeeting' } as const
export const region = { west: 46, south: 40.5, east: 87.5, north: 55.5 }
export const byteLength = (value: string) => new TextEncoder().encode(value.trim()).length
export function validBounds(b: PlaceBounds) { return Object.values(b).every(Number.isFinite) && b.west >= region.west && b.east <= region.east && b.south >= region.south && b.north <= region.north && b.west < b.east && b.south < b.north }
export function readPlaceFilters(params: URLSearchParams) {
  const names = ['west', 'south', 'east', 'north'] as const
  const bounds = Object.fromEntries(names.map((key) => [key, params.has(key) ? Number(params.get(key)) : NaN])) as unknown as PlaceBounds
  const category = params.get('category') ?? ''
  return { q: params.get('q') ?? '', city: params.get('city') ?? '', category: placeCategories.includes(category as PlaceCategory) ? category as PlaceCategory : '', bounds: validBounds(bounds) ? bounds : undefined, after: params.get('after') ?? '' }
}
export function placesQuery(params: URLSearchParams) {
  const f = readPlaceFilters(params), query = new URLSearchParams({ limit: '100' })
  if (f.q.trim()) query.set('q', f.q.trim())
  if (f.city.trim()) query.set('city', f.city.trim())
  if (f.category) query.set('category', f.category)
  if (f.after) query.set('after', f.after)
  if (f.bounds) for (const [key, value] of Object.entries(f.bounds)) query.set(key, String(value))
  return query.toString()
}
export function validPoint(lat: number, lon: number) { return Number.isFinite(lat) && Number.isFinite(lon) && lat >= region.south && lat <= region.north && lon >= region.west && lon <= region.east }
export function validReason(reason: string) { return byteLength(reason) > 0 && byteLength(reason) <= 2000 }
export function validPlace(place: PlaceInput) {
  const limits = { name: 200, city: 160, address: 500, description: 2000, website: 2000, phone: 100, opening_hours: 300 } as const
  if (!place.name.trim() || !placeCategories.includes(place.category) || !validPoint(place.lat, place.lon)) return false
  if (Object.entries(limits).some(([key, max]) => byteLength(place[key as keyof typeof limits]) > max)) return false
  if (place.website.trim()) { try { const url = new URL(place.website.trim()); if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password) return false } catch { return false } }
  return true
}
export function placeInput(place: Place): PlaceInput { const { name, category, lat, lon, city, address, description, website, phone, opening_hours } = place; return { name, category, lat, lon, city, address, description, website, phone, opening_hours } }

export const kindLabels = { new: 'placesNew', correction: 'placesCorrection', closed: 'placesClosed' } as const
