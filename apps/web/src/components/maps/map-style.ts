import { osm } from '@versatiles/style'
import type { StyleSpecification } from 'maplibre-gl'
import type { Locale } from '../../types'
import { resolveArchiveUrl } from './map-runtime'

const attribution = '<a href="https://www.openstreetmap.org/copyright" target="_blank" rel="noopener noreferrer">© OpenStreetMap contributors</a> · <a href="https://www.geofabrik.de/" target="_blank" rel="noopener noreferrer">Geofabrik</a>'

export function mapStyle(archiveUrl: string, locale: Locale, origin: string): StyleSpecification {
  const archive = resolveArchiveUrl(archiveUrl, origin)
  const style = osm({
    theme: 'muted',
    projection: 'mercator',
    sky: false,
    features: { terrain: false, hillshade: false, landcover: false, buildings: 'flat' },
    text: { language: locale === 'en' ? 'en' : 'local', font: 'noto_sans_regular' },
    urls: {
      base: origin,
      osm: archive,
      glyphsPattern: `${origin}/map-data/fonts/{fontstack}/{range}.pbf`,
      sprite: [{ id: 'base', url: `${origin}/map-data/sprites/base` }],
    },
  })
  for (const source of Object.values(style.sources)) {
    if (source.type === 'vector') {
      // Assign after generation: the builder normalizes nested protocol URLs.
      source.url = `pmtiles://${archive}`
      source.attribution = attribution
    }
  }
  return style
}
