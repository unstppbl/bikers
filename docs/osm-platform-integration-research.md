# Self-hosted map integration

Checked **5 November 2026** for issue #18. This note records primary-source findings and proposed integration seams; it is not a deployment receipt. The [Kazakhstan benchmark](osm-kazakhstan-benchmark.md) covers the existing PMTiles archive, Photon index and Valhalla graph. It did not test browser rendering or combined production traffic.

## Recommended map stack and pins

Use **MapLibre GL JS + the PMTiles browser protocol + a Shortbread-compatible VersaTiles style**, with tiles, glyphs and sprites served from our origin. This is a proposal based on the existing archive schema, not a claim that another provider has worse Kazakhstan coverage.

| Component | Version checked | Primary evidence |
| --- | --- | --- |
| `maplibre-gl` | **6.16.0** | [Official release](https://github.com/maplibre/maplibre-gl-js/releases/tag/v6.16.0) |
| `pmtiles` browser package | **4.5.0** | [Maintainer package manifest](https://github.com/protomaps/PMTiles/blob/main/js/package.json), [published versions](https://www.npmjs.com/package/pmtiles?activeTab=versions) |
| `@versatiles/style` | **8.1.1** | [Live release metadata](https://api.github.com/repos/versatiles-org/versatiles-style/releases/latest), [tagged package](https://github.com/versatiles-org/versatiles-style/blob/v8.1.1/package.json) |
| VersaTiles Noto Sans glyphs | **3.0.0** | [Live release metadata](https://api.github.com/repos/versatiles-org/versatiles-fonts/releases/latest) |

The live GitHub API reported style **8.1.1**, published 3 November; cached search/HTML pages still showed 7.x. Pin versions and asset hashes rather than a `latest` URL in the deployment. The style's Node requirement is `>=20`; our web package requires Node `>=22.12.0`. [Tagged style package](https://github.com/versatiles-org/versatiles-style/blob/v8.1.1/package.json), [web package](../apps/web/package.json)

Shortbread's own documentation lists VersaTiles Colorful/Neutrino and a basic demo style. Use the current builder's **`osm()` Shortbread entry**, not its separate OpenMapTiles or Protomaps-schema builders. Converting an MBTiles container to PMTiles does not change its vector-layer schema. [Shortbread styles](https://shortbread-tiles.org/styles/), [tagged style API](https://github.com/versatiles-org/versatiles-style/blob/v8.1.1/src/index.ts), [measured archive metadata](osm-kazakhstan-benchmark-results.json)

## Browser, React and Vite

MapLibre 6 uses ESM and named/namespace imports; its old default import, UMD bundle and separate CSP build are gone. Its current Vite recipe imports CSS and obtains the worker with `maplibre-gl/dist/maplibre-gl-worker.mjs?worker&url`, then calls `setWorkerUrl` before constructing maps. Our app already uses React 19, TypeScript, Vite 8 and ESM, so no React wrapper is required. Compatibility still needs a real production build and browser test. [MapLibre migration](https://maplibre.org/maplibre-gl-js/docs/guides/v5-to-v6-migration-guide/), [Vite installation recipe](https://maplibre.org/maplibre-gl-js/docs/), [current package](../apps/web/package.json)

PMTiles supplies `Protocol`; register its `tile` handler with MapLibre's `addProtocol('pmtiles', ...)` **once before maps mount**. A vector source then uses `pmtiles://https://OUR_ORIGIN/maps/DATASET.pmtiles`; the protocol derives source min/max zoom. The official React example unregisters on application teardown. Prefer an application-scoped registration so removing one map does not disable another. [PMTiles MapLibre/React integration](https://docs.protomaps.com/pmtiles/maplibre)

**Proposed component ownership:** a lazy-loaded map component creates one map per mounted container, removes it on unmount, and updates markers/GeoJSON sources when props change. Keep picked coordinates in React form state, not in the map as the only copy. Use ordinary accessible form controls for search/results and a visible textual fallback when WebGL, worker loading or map assets fail. This proposal avoids adding a second React state model around the renderer.

For a same-origin worker, the documented CSP requirements include `worker-src 'self'` and `img-src data: blob: 'self'`. Keep map/style/API fetches on our origin and account for them in `connect-src`; inspect the actual deployed CSP instead of assuming the repository controls every proxy header. A CDN's automatic blob-worker path has different policy requirements. Test the emitted worker URL after Vite builds it. [MapLibre CSP guidance](https://maplibre.org/maplibre-gl-js/docs/), [Vite worker handling](https://vite.dev/guide/features.html#web-workers)

## A style with no hosted dependencies

The current style builder accepts `urls.osm`, `glyphsPattern`, sprite entries, projection and optional terrain/hillshade/landcover. Its defaults contain VersaTiles paths, and prebuilt published styles also include landcover. Explicitly replace every active source and asset URL. Our archive contains Shortbread 1.0 and zooms **0–14**, not a DEM or separate global landcover dataset. [Tagged URL options](https://github.com/versatiles-org/versatiles-style/blob/v8.1.1/src/options/parts/urls.ts), [feature options](https://github.com/versatiles-org/versatiles-style/blob/v8.1.1/src/options/parts/features.ts), [archive inventory](osm-kazakhstan-benchmark-results.json)

Builder settings, with `origin` taken from the current application origin and the archive path supplied by geo configuration:

```ts
{
  theme: 'muted',
  projection: 'mercator',
  sky: false,
  features: { terrain: false, hillshade: false, landcover: false, buildings: 'flat' },
  text: { language: locale === 'en' ? 'en' : 'local', font: 'noto_sans_regular' },
  urls: {
    base: origin,
    osm: `${origin}/map-data/DATASET_HASH/kazakhstan.pmtiles`,
    glyphsPattern: `${origin}/map-data/fonts/{fontstack}/{range}.pbf`,
    sprite: [{ id: 'base', url: `${origin}/map-data/sprites/base` }],
  },
}
```

The integration sanity check found that the pinned builder normalizes a nested `pmtiles://https://...` input to `pmtiles://https//...`. The [style adapter](../apps/web/src/components/maps/map-style.ts) therefore supplies a normal archive URL during generation, then assigns `pmtiles://` plus the absolute archive URL directly to the resulting vector source. A [regression test](../apps/web/src/components/maps/map-style.test.ts) checks the exact URL, local assets and style-spec validity. This result does not establish browser acceptance.

The builder's `inlineSources` helper resolves HTTP TileJSON sources and preserves their attribution/bounds, but deliberately skips non-HTTP protocols; avoid resolving the raw archive as HTTP TileJSON. Inspect generated source IDs before adding ride overlays. [Builder usage](https://github.com/versatiles-org/versatiles-style/blob/v8.1.1/README.md), [non-HTTP source support](https://github.com/versatiles-org/versatiles-style/releases/tag/v6.3.1)

Sprites need matching JSON/PNG assets, including high-DPI variants; the `base` sprite ID must match generated icon references. Merely replacing the sprite URL with an unrelated icon sheet can leave missing icons. [Sprite specification](https://maplibre.org/maplibre-style-spec/sprite/), [tagged sprite options](https://github.com/versatiles-org/versatiles-style/blob/v8.1.1/src/options/parts/sprite.ts)

### Pinned small asset acquisitions

These are source URLs and release-reported hashes, not downloads performed by this research:

| Asset | Download | Bytes | SHA256 |
| --- | --- | ---: | --- |
| Noto Sans glyph archive | [noto_sans.tar.gz, v3.0.0](https://github.com/versatiles-org/versatiles-fonts/releases/download/v3.0.0/noto_sans.tar.gz) | 31,304,823 | `f58be6be2eb3d127b6ff1c90d828b69ff4807d1d88cd13a43930943fda0e334e` |
| Matching sprites | [sprites.tar.gz, v8.1.1](https://github.com/versatiles-org/versatiles-style/releases/download/v8.1.1/sprites.tar.gz) | 855,499 | `2d3266d99a8ce96f56d64f67ba2dbd3cf5d7547a5f9ade3f0e31f949188358de` |

Both values come from the official release APIs: [fonts](https://api.github.com/repos/versatiles-org/versatiles-fonts/releases/latest), [style assets](https://api.github.com/repos/versatiles-org/versatiles-style/releases/latest). Inspect archive paths before mapping them to the proposed URL layout. Font archives can contain tar hardlinks; extract with a reader that supports them. No font rebuilding is needed. [Font packaging instructions](https://github.com/versatiles-org/versatiles-fonts/blob/v3.0.0/README.md)

Retain the [style MIT notice](https://github.com/versatiles-org/versatiles-style/blob/v8.1.1/LICENSE.md), [sprite CC0 notice](https://github.com/versatiles-org/versatiles-style/blob/v8.1.1/icons/LICENSE.md), [glyph-source OFL](https://github.com/versatiles-org/versatiles-fonts/blob/v3.0.0/fonts/Noto%20Sans/LICENSE) and [Noto copyright/OFL](https://github.com/google/fonts/blob/c92b209a838649c205a1565b5f28c1973e049659/ofl/notosans/OFL.txt). The glyph-build project's MIT license does not replace the fonts' licenses.

## English, Russian and Kazakh labels

Shortbread 1.0 exposes `name`, `name_en` and `name_de` on label layers; the benchmark archive's metadata agrees. It does **not** declare separate `name_ru` or `name_kk` fields. For this archive, use English where present and the feature's local `name` otherwise; use local names for Russian/Kazakh UI. Do not label this a translated Kazakh/Russian basemap. Independent KK/RU names require a different/regenerated tile schema or a separate overlay. [Shortbread 1.0 schema](https://shortbread-tiles.org/schema/1.0/), [archive metadata](osm-kazakhstan-benchmark-results.json)

The builder supports a local-name fallback when `languageStrict` is false. Keep that behavior, and test empty strings as well as missing name fields. The language of the **map labels** is separate from Photon search's `lang` and our interface translations. [Tagged text options](https://github.com/versatiles-org/versatiles-style/blob/v8.1.1/src/options/parts/text.ts), [measured search behavior](osm-kazakhstan-benchmark.md)

MapLibre's SDF glyph URL uses `{fontstack}` and 256-code-point `{range}` blocks. Font names must exactly match the extracted asset paths and `text-font` values; VersaTiles uses names such as `noto_sans_regular`. Verify Latin and Cyrillic glyph responses and visually check **Ә Ғ Қ Ң Ө Ұ Ү Һ І / ә ғ қ ң ө ұ ү һ і**. A font-family name alone does not prove that every required character is present. [Glyph specification](https://maplibre.org/maplibre-style-spec/glyphs/), [tagged font configuration](https://github.com/versatiles-org/versatiles-fonts/blob/v3.0.0/fonts/Noto%20Sans/fonts.json), [style font naming](https://github.com/versatiles-org/versatiles-style/blob/v8.1.1/src/options/parts/text.ts)

## Serving and privacy seams

PMTiles requires real HTTP Range delivery. Cross-origin hosting also needs CORS; official examples allow GET/HEAD with `Range`/`If-Match` and expose `ETag`. **Integration plan:** keep tested Nginx static delivery behind `/map-data/`, use immutable versioned archive filenames, preserve byte-range behavior through the public proxy, and verify public 206 responses and bytes after release. Add OSM/Geofabrik source attribution to the style and retain a visible attribution control. [PMTiles hosting requirements](https://docs.protomaps.com/pmtiles/cloud-storage), [measured attribution metadata](osm-kazakhstan-benchmark-results.json)

Current frontend seams and proposed changes:

| Owning source | Observed behavior → proposed integration |
| --- | --- |
| [Ride form](../apps/web/src/pages/Rides.tsx) | Meeting point/destination are strings and the route is an external URL → add a shared address picker plus map click/drag, explicit selected coordinates and a route preview. Editing text after a selection must clear stale coordinates. |
| [Ride types](../apps/web/src/types.ts) | No coordinate/geometry fields → model selected points explicitly and keep labels separate from coordinates; use [GeoJSON longitude/latitude order](https://datatracker.ietf.org/doc/html/rfc7946#section-3.1.1) at the map boundary. |
| [Ride detail](../apps/web/src/pages/RideDetail.tsx) | Exact plans render only when `canAccess` and navigation opens Google text search → place the map/route inside the same private-plan branch. Server authorization must also gate coordinates and geometry. |
| [Public ride mapper](../apps/web/src/api.ts) | Public data is allowlisted to city/date/description/status → retain an explicit public allowlist when new location fields are introduced. |
| [City selector](../apps/web/src/components/CitySelector.tsx) | A localized city vocabulary permits manual entry → keep city selection independent from a precise private address picker. |
| [API wrapper](../apps/web/src/api.ts) | Same-origin session/CSRF handling and `cache: 'no-store'` → call our Go search/reverse/routing endpoints through this boundary, not publicly exposed Photon/Valhalla admin ports. |
| [Service worker](../apps/web/src/sw.ts), [Vite configuration](../apps/web/vite.config.ts) | Small built assets are precached and API/media routes excluded from navigation fallback → exclude map service paths from SPA navigation fallback; keep the country archive and private search/routes out of automatic offline caching. |

These are code-derived proposals, not implemented behavior. Precise ride data must stay private across server responses, query keys, session changes and browser caches. Basemap files can remain public independently of a private overlay.

## Required browser verification

Before considering the integration complete, verify a production Vite build with the emitted worker, no unexpected external tile/font requests, visible attribution, successful public ranges, sprites at normal/high DPI, mobile layout, and the Kazakh glyph string above. Exercise map click/drag, keyboard search selection, stale-response cancellation, manual-address fallback and coordinate invalidation after text edits. Check anonymous/requested/approved/removed access, locale changes, missing names, a missing asset and WebGL failure. Test browser route overlays and simultaneous serving load; the earlier component benchmark did not establish these results.
