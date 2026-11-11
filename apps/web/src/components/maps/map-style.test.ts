import { describe, expect, it } from 'vitest'
import { validateStyleMin } from '@maplibre/maplibre-gl-style-spec'
import { mapStyle } from './map-style'

const origin = 'https://bikers.kz'
const archive = '/map-data/2583113c45ca/kazakhstan.pmtiles'

describe('Kazakhstan map style', () => {
  it('preserves the nested PMTiles URL and uses only local assets', () => {
    const style = mapStyle(archive, 'en', origin)
    expect(Object.values(style.sources)).toHaveLength(1)
    expect(Object.values(style.sources)[0]).toMatchObject({
      type: 'vector', url: `pmtiles://${origin}${archive}`,
      attribution: expect.stringContaining('OpenStreetMap contributors'),
    })
    expect(style.glyphs).toBe(`${origin}/map-data/fonts/{fontstack}/{range}.pbf`)
    expect(style.sprite).toEqual([{ id: 'base', url: `${origin}/map-data/sprites/base` }])
    expect(validateStyleMin(style)).toEqual([])
  })

  it('prefers English names and retains local names for Russian and Kazakh', () => {
    for (const locale of ['en', 'ru', 'kk'] as const) {
      const style = mapStyle(archive, locale, origin)
      const label = style.layers.find((layer) => layer.type === 'symbol' && 'source-layer' in layer && layer['source-layer'] === 'pois')
      expect(label?.type === 'symbol' && label.layout?.['text-field']).toEqual(locale === 'en'
        ? ['coalesce', ['get', 'name_en'], ['get', 'name']]
        : ['get', 'name'])
    }
  })

  it('rejects missing or external archives', () => {
    expect(() => mapStyle('', 'en', origin)).toThrow()
    expect(() => mapStyle('https://example.com/map.pmtiles', 'en', origin)).toThrow()
  })
})
