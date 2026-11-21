import { createHash } from 'node:crypto'
import { readdirSync, readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import type { CityOption } from './cities'
import { cityPhoto, cityPhotos } from './cityPhotos'
import type { CityPhoto } from './cityPhotos'

// Provenance of each served file, kept outside public/ so it is never served.
interface Provenance { city: string; url: string; width: number; height: number; bytes: number; sha256: string; source_page: string; license: string; reviewed_on: string }
const web = resolve(__dirname, '..')
const provenance = JSON.parse(readFileSync(resolve(web, 'city-photos.json'), 'utf8')) as Provenance[]
const publicFile = (url: string) => readFileSync(resolve(web, `public${url}`))

// Width and height of a lossy (VP8) or extended (VP8X) WebP file.
function webpSize(data: Buffer) {
  expect(data.toString('latin1', 0, 4) + data.toString('latin1', 8, 12)).toBe('RIFFWEBP')
  const chunk = data.toString('latin1', 12, 16)
  if (chunk === 'VP8 ') return { width: data.readUInt16LE(26) & 0x3fff, height: data.readUInt16LE(28) & 0x3fff }
  expect(chunk).toBe('VP8X')
  return { width: 1 + data.readUIntLE(24, 3), height: 1 + data.readUIntLE(27, 3) }
}

const cities: CityOption[] = [
  { value: 'Astana', names: { en: 'Astana', ru: 'Астана', kk: 'Астана' }, aliases: ['Nur-Sultan', 'Нур-Султан', 'Нұр-Сұлтан'] },
  { value: 'Almaty', names: { en: 'Almaty', ru: 'Алматы', kk: 'Алматы' }, aliases: ['Alma-Ata', 'Алма-Ата'] },
]
const rideId = '20000000-0000-4000-8000-000000000001'
const sampleRideId = (n: number) => `20000000-0000-4000-8000-${n.toString(16).padStart(12, '0')}`
const curated = Object.entries(cityPhotos)

describe('city photos', () => {
  it('lists same-origin files under /city-photos/ with their dimensions', () => {
    expect(curated.length).toBeGreaterThan(0)
    for (const [, photos] of curated) {
      expect(photos.length).toBeGreaterThan(0)
      expect(new Set(photos.map((photo) => photo.url)).size).toBe(photos.length)
      for (const photo of photos) {
        expect(photo.url).toMatch(/^\/city-photos\/[^/?#\s]+$/)
        expect(Number.isSafeInteger(photo.width) && photo.width > 0).toBe(true)
        expect(Number.isSafeInteger(photo.height) && photo.height > 0).toBe(true)
      }
    }
  })

  it('serves exactly the recorded files, each CC0 or public domain and unchanged since review', () => {
    const byUrl = (a: { url: string }, b: { url: string }) => a.url.localeCompare(b.url)
    const listed = curated.flatMap(([city, photos]) => photos.map((photo) => ({ city, ...photo }))).sort(byUrl)
    expect([...provenance].sort(byUrl).map(({ city, url, width, height }) => ({ city, url, width, height }))).toEqual(listed)
    expect(readdirSync(resolve(web, 'public/city-photos')).sort()).toEqual(listed.map((photo) => photo.url.slice('/city-photos/'.length)).sort())
    for (const entry of provenance) {
      const data = publicFile(entry.url)
      expect(data.length).toBe(entry.bytes)
      expect(data.length).toBeLessThanOrEqual(400_000)
      expect(createHash('sha256').update(data).digest('hex')).toBe(entry.sha256)
      expect(entry.url).toContain(`-${entry.sha256.slice(0, 8)}.webp`)
      expect(webpSize(data)).toEqual({ width: entry.width, height: entry.height })
      expect(['CC0', 'Public domain']).toContain(entry.license)
      expect(entry.source_page).toMatch(/^https:\/\/commons\.wikimedia\.org\/wiki\/File:/)
      expect(entry.reviewed_on).toMatch(/^\d{4}-\d{2}-\d{2}$/)
    }
  })

  it('gives the same ride the same photo every time, with or without the catalogue', () => {
    for (const [city, photos] of curated) {
      const chosen = cityPhoto(city, rideId, [])
      expect(photos).toContain(chosen)
      expect(cityPhoto(city, rideId, [])).toBe(chosen)
      expect(cityPhoto(city, rideId, cities)).toBe(chosen)
    }
  })

  it('spreads different rides across every photo of a city', () => {
    for (const [city, photos] of curated) {
      // Reaching n photos takes about n·ln(n) rides on average, so 100 rides per photo leaves a wide margin.
      const chosen = new Set<CityPhoto | undefined>()
      for (let n = 0; n < photos.length * 100 && chosen.size < photos.length; n++) chosen.add(cityPhoto(city, sampleRideId(n), []))
      expect(chosen).toEqual(new Set(photos))
    }
  })

  it('resolves catalogue aliases and localized names once the catalogue is loaded', () => {
    const expected = cityPhoto('Almaty', rideId, [])
    expect(cityPhotos.Almaty).toContain(expected)
    for (const name of ['Алма-Ата', 'Алматы', ' almaty ', 'ALMA ATA']) {
      expect(cityPhoto(name, rideId, cities)).toBe(expected)
    }
    // Without the catalogue only the stored value is known.
    expect(cityPhoto('Алматы', rideId, [])).toBeUndefined()
  })

  it('resolves the stored catalogue value before the catalogue loads', () => {
    for (const [city, photos] of curated) expect(photos).toContain(cityPhoto(city, rideId, []))
  })

  it('has no photo for custom, unknown, blank or prototype city names', () => {
    // A malformed catalogue entry must not reach prototype members either.
    const catalogue: CityOption[] = [...cities, { value: 'toString', names: { en: 'Tostring', ru: 'Тостринг', kk: 'Тостринг' }, aliases: ['Village'] }]
    for (const name of ['Kok-Zhailau camp', 'Bishkek', 'Almaty district', '', '   ', 'constructor', '__proto__', 'toString', 'hasOwnProperty', 'Village']) {
      expect(cityPhoto(name, rideId, [])).toBeUndefined()
      expect(cityPhoto(name, rideId, catalogue)).toBeUndefined()
    }
  })
})
