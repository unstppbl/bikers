import { describe, expect, it } from 'vitest'
import { byteLength, placesQuery, readPlaceFilters, validBounds, validPlace, validReason } from './places'
import type { PlaceInput } from './types'
const place: PlaceInput = { name: 'Workshop', category: 'workshop', lat: 43.2, lon: 76.9, city: '', address: '', description: '', website: '', phone: '', opening_hours: '' }
describe('places request and field validation', () => {
  it('sends a single category and complete ordered regional bounds, excluding selection', () => {
    expect(placesQuery(new URLSearchParams('q=+repair+&category=workshop&city=Almaty&west=76&south=43&east=77&north=44&place=private-ui'))).toBe('limit=100&q=repair&city=Almaty&category=workshop&west=76&south=43&east=77&north=44')
    expect(readPlaceFilters(new URLSearchParams('category=bogus&west=76&south=43&east=77')).bounds).toBeUndefined()
    expect(readPlaceFilters(new URLSearchParams('category=bogus')).category).toBe('')
    expect(validBounds({ west: 45, south: 43, east: 77, north: 44 })).toBe(false)
    expect(validBounds({ west: 77, south: 43, east: 76, north: 44 })).toBe(false)
  })
  it('validates UTF8 byte budgets, finite coordinates and website schemes', () => {
    expect(byteLength(' Қала ')).toBe(8)
    expect(validPlace(place)).toBe(true)
    expect(validPlace({ ...place, name: 'Қ'.repeat(101) })).toBe(false)
    expect(validPlace({ ...place, lat: NaN })).toBe(false)
    expect(validPlace({ ...place, lon: 10 })).toBe(false)
    expect(validPlace({ ...place, website: 'javascript:alert(1)' })).toBe(false)
    expect(validPlace({ ...place, website: 'https://example.org' })).toBe(true)
    expect(validReason(' ')).toBe(false)
    expect(validReason('Ә'.repeat(1001))).toBe(false)
  })
})
