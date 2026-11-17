import { describe, expect, it } from 'vitest'
import { cityCenter } from './cities'
import type { CityOption } from './cities'

const cities: CityOption[] = [
  { value: 'Astana', names: { en: 'Astana', ru: 'Астана', kk: 'Астана' }, aliases: ['Nur-Sultan', 'Нур-Султан', 'Нұр-Сұлтан'], center: { lat: 51.1801, lon: 71.44598 } },
  { value: 'Almaty', names: { en: 'Almaty', ru: 'Алматы', kk: 'Алматы' }, aliases: ['Alma-Ata', 'Алма-Ата'], center: { lat: 43.25249, lon: 76.9115 } },
]

describe('city map centers', () => {
  it('resolves stable values, localized names and historical aliases through the shared vocabulary', () => {
    for (const name of ['Astana', ' АСТАНА ', 'nur sultan', 'Нұр-Сұлтан']) expect(cityCenter(name, cities)).toEqual({ lat: 51.1801, lon: 71.44598 })
    for (const name of ['Almaty', ' Алматы ', 'ALMA ATA']) expect(cityCenter(name, cities)).toEqual({ lat: 43.25249, lon: 76.9115 })
    expect(cityCenter('Astana', cities)).not.toBe(cities[0].center)
  })

  it('leaves unknown, partial and unavailable catalogue entries without a camera target', () => {
    expect(cityCenter('Bishkek', cities)).toBeUndefined()
    expect(cityCenter('Almaty district', cities)).toBeUndefined()
    expect(cityCenter(' ', cities)).toBeUndefined()
    expect(cityCenter('Astana', [])).toBeUndefined()
    expect(cityCenter('Astana', [{ ...cities[0], center: undefined }])).toBeUndefined()
  })

  it('rejects unusable coordinates from a malformed catalogue', () => {
    for (const center of [{ lat: NaN, lon: 71 }, { lat: 51, lon: Infinity }, { lat: 91, lon: 71 }, { lat: 51, lon: -181 }]) {
      expect(cityCenter('Astana', [{ ...cities[0], center }])).toBeUndefined()
    }
  })
})
