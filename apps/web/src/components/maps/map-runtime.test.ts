import { afterEach, describe, expect, it, vi } from 'vitest'
import { PMTiles, Protocol } from 'pmtiles'
import { resetMapArchive, resolveArchiveUrl } from './map-runtime'

const origin = 'https://bikers.kz'
const archive = '/map-data/2583113c45ca/kazakhstan.pmtiles'

function emptyArchive() {
  const bytes = new Uint8Array(128)
  bytes.set(new TextEncoder().encode('PMTiles'))
  const view = new DataView(bytes.buffer)
  view.setUint8(7, 3)
  view.setBigUint64(8, 127n, true) // root directory offset
  view.setBigUint64(16, 1n, true) // one zero-entry varint
  for (const offset of [24, 40, 56]) view.setBigUint64(offset, 128n, true)
  view.setUint8(96, 1) // clustered
  view.setUint8(97, 1) // internal compression: none
  view.setUint8(98, 1) // tile compression: none
  view.setUint8(99, 1) // MVT
  view.setUint8(101, 14)
  for (const [offset, coordinate] of [[102, 46], [106, 40.5], [110, 87.5], [114, 55.5], [119, 66], [123, 48]]) {
    view.setInt32(offset, coordinate * 10_000_000, true)
  }
  return bytes.buffer
}

afterEach(() => vi.unstubAllGlobals())

describe('map archive retry', () => {
  it('recovers a real Protocol from a cached 503 by replacing only the failed archive', async () => {
    const fetch = vi.fn()
      .mockResolvedValueOnce(new Response(null, { status: 503 }))
      .mockImplementation(async () => new Response(emptyArchive(), {
        status: 206, headers: { 'Content-Length': '128', 'Content-Range': 'bytes 0-127/128' },
      }))
    vi.stubGlobal('fetch', fetch)
    const protocol = new Protocol()
    const other = new PMTiles(`${origin}/map-data/another/kazakhstan.pmtiles`)
    protocol.add(other)
    const request = { url: `pmtiles://${origin}${archive}`, type: 'json' as const }

    await expect(protocol.tile(request, new AbortController())).rejects.toThrow('503')
    const poisoned = protocol.get(`${origin}${archive}`)
    await expect(protocol.tile(request, new AbortController())).rejects.toThrow('503')
    expect(fetch).toHaveBeenCalledTimes(1)

    resetMapArchive(protocol, archive, origin)
    expect(protocol.get(`${origin}${archive}`)).not.toBe(poisoned)
    expect(protocol.get(other.source.getKey())).toBe(other)
    expect(protocol.get(archive)).toBeUndefined()
    await expect(protocol.tile(request, new AbortController())).resolves.toMatchObject({
      data: { minzoom: 0, maxzoom: 14, bounds: [46, 40.5, 87.5, 55.5] },
    })
    expect(fetch).toHaveBeenCalledTimes(2)
    expect(fetch.mock.calls[1][0]).toBe(`${origin}${archive}`)
  })

  it('uses the same canonical origin URL and refuses to reset another origin', () => {
    const protocol = new Protocol()
    const foreign = new PMTiles(`https://elsewhere.example${archive}`)
    protocol.add(foreign)
    expect(resolveArchiveUrl(`/map-data/../${archive.slice(1)}`, origin)).toBe(`${origin}${archive}`)
    expect(() => resetMapArchive(protocol, foreign.source.getKey(), origin)).toThrow('application origin')
    expect(protocol.get(foreign.source.getKey())).toBe(foreign)
    expect(protocol.get(`${origin}${archive}`)).toBeUndefined()
  })
})
