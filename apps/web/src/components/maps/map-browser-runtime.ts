import { addProtocol, setWorkerUrl } from 'maplibre-gl'
import workerUrl from 'maplibre-gl/dist/maplibre-gl-worker.mjs?worker&url'
import { Protocol } from 'pmtiles'
import { resetMapArchive } from './map-runtime'
let protocol: Protocol | undefined
export function initializeMapRuntime() {
  setWorkerUrl(workerUrl)
  if (!protocol) { protocol = new Protocol(); addProtocol('pmtiles', protocol.tile) }
}
export function retryMapArchive(archiveUrl: string) {
  if (protocol) resetMapArchive(protocol, archiveUrl, window.location.origin)
}
