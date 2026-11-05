import { PMTiles } from 'pmtiles'
import type { Protocol } from 'pmtiles'

export function resolveArchiveUrl(archiveUrl: string, origin: string): string {
  const archive = new URL(archiveUrl, origin)
  if (!archiveUrl || archive.origin !== origin || !['http:', 'https:'].includes(archive.protocol)) {
    throw new Error('Map archive must use the application origin')
  }
  return archive.href
}

export function resetMapArchive(protocol: Protocol, archiveUrl: string, origin: string) {
  // PMTiles retains rejected header promises. A retry needs a new cache for
  // this exact source; keep the global protocol and other archives intact.
  protocol.add(new PMTiles(resolveArchiveUrl(archiveUrl, origin)))
}
