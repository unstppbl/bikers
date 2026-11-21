import { normalizeCity } from './cities'
import type { CityOption } from './cities'

export interface CityPhoto { url: string; width: number; height: number }

// Curated photos of catalogue cities, keyed by the catalogue value and served from /city-photos/.
// Every file is CC0 or public domain, so the app shows no credits. Their provenance lives in
// apps/web/city-photos.json, which is not served.
export const cityPhotos: Readonly<Record<string, readonly CityPhoto[]>> = {
  Almaty: [
    { url: '/city-photos/almaty-1-e7845a81.webp', width: 1600, height: 1018 },
    { url: '/city-photos/almaty-2-44c6b951.webp', width: 1600, height: 1200 },
    { url: '/city-photos/almaty-3-c92eeea2.webp', width: 1600, height: 1067 },
  ],
  Astana: [
    { url: '/city-photos/astana-1-15309e2a.webp', width: 1600, height: 1067 },
    { url: '/city-photos/astana-2-a77ae13b.webp', width: 1600, height: 1067 },
  ],
  Aktau: [
    { url: '/city-photos/aktau-1-bb56e410.webp', width: 1600, height: 1067 },
  ],
  Kokshetau: [
    { url: '/city-photos/kokshetau-1-25d42bbf.webp', width: 2600, height: 1001 },
  ],
  Petropavl: [
    { url: '/city-photos/petropavl-1-0f22e93a.webp', width: 1600, height: 900 },
  ],
}

// 32-bit FNV-1a: the same ride gets the same photo for every viewer, and rides spread across a city's photos.
function hash(value: string) {
  let result = 0x811c9dc5
  for (let index = 0; index < value.length; index++) result = Math.imul(result ^ value.charCodeAt(index), 0x01000193)
  return result >>> 0
}

// A ride without a cover shows a photo of its catalogue city. Aliases and localized names resolve once the
// catalogue has loaded; a stored catalogue value resolves before that. Custom cities have no photo.
export function cityPhoto(city: string, rideId: string, cities: readonly CityOption[]): CityPhoto | undefined {
  const value = normalizeCity(city, cities)
  const photos = Object.hasOwn(cityPhotos, value) ? cityPhotos[value] : undefined
  return photos?.length ? photos[hash(rideId) % photos.length] : undefined
}
