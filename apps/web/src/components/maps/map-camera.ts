import type { MapOptions } from 'maplibre-gl'

export const defaultMapCamera = {
  center: [71.43316394219903, 51.11307657146983],
  zoom: 14,
} satisfies Pick<MapOptions, 'center' | 'zoom'>
