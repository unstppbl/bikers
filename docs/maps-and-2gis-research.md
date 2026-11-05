# Maps and 2GIS API research

Checked **5 November 2026**. This is research and a proposed integration path, not approval to add map features, buy a subscription, or change the existing ride-access policy. No accounts, subscriptions, paid API calls, or vendor contacts were created for this review.

**Recommendation:** evaluate 2GIS for the Kazakhstan pilot, starting with its cheaper 2D RasterJS/Raster Tiles option for an organizer-selected meeting/destination pin and an approved-participant map. Keep MapGL/vector maps as an alternative if their styling, rendering or mobile experience justify the higher tariff. Both can display our own pins and route overlays. Retain external navigation handoff, and add address search and route calculation only when they justify their separate API charges. [Map product choices](https://docs.2gis.com/en/maps/overview), [RasterJS capabilities](https://docs.2gis.com/en/maps/others/rasterjs/overview)

## Which 2GIS products we would use

| BIKERS.KZ capability | Product | Scope and distinction |
| --- | --- | --- |
| 2D map, our meeting pins and route overlays | RasterJS API + Raster Tiles API | Browser renderer with clickable/draggable markers, popups and polylines; raster-tile requests are the paid resource. |
| Vector/3D map, custom styles and our overlays | MapGL JS API + Map Tiles API | Browser renderer included with vector-tile access; tile requests are the paid resource. |
| Address autocomplete | Suggest API | Suggests possible addresses or route endpoints while the user types. |
| Address to coordinates, or coordinates to address | Geocoder API | A separate lookup; selecting coordinates by clicking our map does not itself require geocoding. |
| Organization/place details | Places API | Relevant if users select a cafe or service provider; not needed merely to show our own ride pins. |
| Calculated road route, distance and travel time | Routing API | Separate from displaying a map or drawing an organizer-supplied line. |
| Noninteractive map image | Static API | An alternative for snapshots, billed separately; it is not automatically included with Map Tiles. |

These product boundaries come from the [map API overview](https://docs.2gis.com/en/maps/overview), [Suggest overview](https://docs.2gis.com/en/api/search/suggest/overview), [Geocoder overview](https://docs.2gis.com/en/api/search/geocoder/overview), and [search integration tutorial](https://docs.2gis.com/en/api/search/suggest/tutorial). The tutorial combines several products to demonstrate search; BIKERS.KZ need not subscribe to every demonstrated service.

## Public pricing and budgeting

The current public monthly tariff has these entry packages. Approximate KZT amounts use the National Bank of Kazakhstan's **5 November 2026 rate of ₸5.53 per RUB**; they are conversions, not a Kazakhstan vendor quote. [Official prices](https://docs.2gis.com/en/platform-manager/subscription/pricing), [NBK exchange rates](https://nationalbank.kz/en/exchangerates/ezhednevnye-oficialnye-rynochnye-kursy-valyut)

| Product | Monthly included volume | Published monthly price | Approximate KZT |
| --- | ---: | ---: | ---: |
| Raster Tiles, for 2D maps | 500,000 tiles | ₽3,500 | ₸19,355 |
| Map Tiles, for MapGL/vector maps | 100,000 tiles | ₽8,000 | ₸44,240 |
| Geocoder | 10,000 successful lookups | ₽4,700 | ₸25,991 |
| Suggest | 100,000 successful requests | ₽7,000 | ₸38,710 |
| Places | 10,000 successful requests | ₽6,700 | ₸37,051 |
| Routing | 10,000 calculations | ₽6,700 | ₸37,051 |

Each row is a separate service allowance, not one interchangeable request pool. A small map-only pilot can therefore start at the raster or vector minimum; search is an additional subscription choice. The published vector steps include 300,000 tiles for ₽19,200 and 500,000 for ₽32,000. The annual entry vector package lists ₽86,400 paid upfront (approximately ₸477,792), retaining the 100,000-tile monthly allowance. Both live English and Russian tables also list 130 million units at ₽200,000 under both monthly and annual pricing; confirm that unusual high-volume offer before using it for procurement. [English tariff tables](https://docs.2gis.com/en/platform-manager/subscription/pricing), [Russian tariff tables](https://docs.2gis.com/platform-manager/subscription/pricing)

Illustrative monthly totals, excluding Places and any unresolved local taxes:

| Feature bundle | 2D raster | Vector/MapGL |
| --- | ---: | ---: |
| Map only, entry package | ₽3,500 ≈ ₸19,355 | ₽8,000 ≈ ₸44,240 |
| Map + Geocoder + Suggest | ₽15,200 ≈ ₸84,056 | ₽19,700 ≈ ₸108,941 |
| Above + Routing | ₽21,900 ≈ ₸121,107 | ₽26,400 ≈ ₸145,992 |

For traffic sensitivity, assume **four map opens per monthly active user and 40 tile requests per open**. These are our illustrative assumptions, not measured usage or a provider map-load billing unit:

| Monthly active users | Assumed monthly tiles | Fitting raster package | Fitting vector package |
| ---: | ---: | ---: | ---: |
| 500 | 80,000 | 500k: ₽3,500 | 100k: ₽8,000 |
| 2,000 | 320,000 | 500k: ₽3,500 | 500k: ₽32,000 |
| 10,000 | 1,600,000 | 3m: ₽14,700 | 3m: ₽120,000 |

These scenario totals cover tiles only. A single map screen loads several tiles; panning and zooming load more. Autocomplete can generate multiple requests per finished address, and route calculations have their own billing rules. Use real pilot statistics before selecting a production package. [Package volumes](https://docs.2gis.com/en/platform-manager/subscription/pricing), [tile accounting](https://docs.2gis.com/en/maps/others/rasterjs/overview)

The documented demo lasts one month and is a one-time trial, not a permanent production free tier. Its allowances differ by service: 500,000 vector tiles, 30,000 raster tiles, and 1,000 requests for each relevant search/routing service. Monthly limits can stop the service when exhausted; do not assume unlimited automatic overage. The listed rate limits are 3,000 requests/minute for maps and 600 for search/routing. [Demo creation and duration](https://docs.2gis.com/en/platform-manager/subscription/keys), [demo and subscription limits](https://docs.2gis.com/en/platform-manager/subscription/pricing)

## Payment and data-use conditions

The documented self-service card flow accepts cards issued in Russia; invoice purchasing is also documented. A Kazakhstan company must confirm the available contract, payment method, billing currency and taxes before treating the table as a payable quote. The web API offer is denominated in RUB; its Russian VAT wording does not settle Kazakhstan tax treatment. [Purchase instructions](https://docs.2gis.com/en/platform-manager/subscription/purchase), [offer, sections 5.2–5.4](https://law.2gis.ru/offer-license-agreement-webapi/)

Preserve the vendor's attribution and rights notices. The offer also restricts extraction/storage/reuse of API output beyond the granted scope; an API subscription is not blanket permission to copy the directory, permanently cache geocoded addresses or retain route geometry for a reusable community route library. Obtain written confirmation for those persistent uses, distinguishing our own user-authored route inputs from provider-generated results. [Offer, sections 2.10, 3.1 and 3.5](https://law.2gis.ru/offer-license-agreement-webapi/)

## 2D integration and tile-request estimates

RasterJS exposes `DG.map`, clickable/draggable markers, map-click events and `DG.polyline` overlays. It therefore meets the basic pin-selection and route-display requirements without MapGL. Create one map for the mounted container, subscribe to clicks/marker dragging, and call `remove()` on cleanup. Keep coordinate conversion explicit: RasterJS uses its `LatLng` convention, whereas MapGL uses `[longitude, latitude]`. [Raster map lifecycle/events](https://docs.2gis.com/en/maps/others/rasterjs/reference/map), [markers](https://docs.2gis.com/en/maps/others/rasterjs/reference/markers), [polyline overlays](https://docs.2gis.com/en/maps/others/rasterjs/reference/vector-layers)

RasterJS charges through successful Raster Tiles requests; one request loads a 256×256 PNG tile. The documented minimum visible tile count is `ceil(width/256) × ceil(height/256)`, with extra edge tiles recommended for smooth interaction. The vendor's 800×600 example needs 12 tiles minimum or 30 with a surrounding buffer. Panning, zooming and fresh loads change usage, so visitors or map opens cannot be converted to a fixed billed count without measurement. [RasterJS request accounting](https://docs.2gis.com/en/maps/others/rasterjs/overview)

## MapGL integration with React, TypeScript and Vite

The official `@2gis/mapgl` package supplies an asynchronous `load()` function and TypeScript support. The documented initialization creates a `mapgl.Map` with a DOM container, an API key, a center in `[longitude, latitude]` order, and a zoom. Official React guidance creates the map in an effect and calls `destroy()` on unmount. For our implementation, also handle navigation/unmount while `load()` is still pending, so a late result cannot create an orphaned map. The lifecycle safeguard is our implementation recommendation. [Getting started](https://docs.2gis.com/en/mapgl/start/first-steps), [React integration](https://docs.2gis.com/en/mapgl/start/react)

MapGL supports markers and HTML popups, and its pointer events expose geographic coordinates. A first pin picker can therefore use a click to place our marker and save the selected longitude/latitude, with a user-authored label. An optional reverse-geocoding request can suggest an address after selection; it should not run on every drag event. [Markers](https://docs.2gis.com/en/mapgl/objects/markers), [popups](https://docs.2gis.com/en/mapgl/objects/popups), [map-click search example](https://docs.2gis.com/en/api/search/suggest/tutorial)

MapGL uses WebGL. Its supported-browser list includes modern desktop browsers, Android and iOS, but the vendor warns that hardware/OS/driver problems can still prevent rendering. Use `isSupported()` and retain a useful location label and external map link when rendering fails. A raster or static map is another option if we choose the corresponding service. [Browser support](https://docs.2gis.com/en/mapgl/overview/browser-support)

The recommended MapGL major-version URL receives compatible updates. A fixed version has a limited support period and is intended for cases such as Subresource Integrity; do not assume a pinned hosted build is supported indefinitely. [Release/version policy](https://docs.2gis.com/en/mapgl/releases)

## Search, language and geographic coverage

Use the API reference as the endpoint authority: Suggest is `GET https://catalog.api.2gis.com/3.0/suggests`. It supports location bias, a viewport or region, and suggestion types including addresses and route endpoints. The current English overview contains an inconsistent `/items` example, while the reference and Russian overview identify `/suggests`. [Suggest reference](https://docs.2gis.com/en/api/search/suggest/reference/3.0/suggests), [Russian overview](https://docs.2gis.com/api/search/suggest/overview)

The reference lists `kk_KZ` and `ru_KZ` search locales, as well as English locale variants. MapGL independently exposes a `lang` option and `setLanguage()`. Do not infer complete English/Russian/Kazakh map labels, organization data or route instructions from search-locale availability. Validate all three product languages in the actual Kazakhstan test area. [Suggest locale reference](https://docs.2gis.com/en/api/search/suggest/reference/3.0/suggests), [MapGL options](https://docs.2gis.com/en/mapgl/reference/interface), [MapGL language method](https://docs.2gis.com/en/mapgl/reference/class)

Kazakhstan is explicitly listed in the Routing product coverage. For acceptance, test the chosen launch city, suburban meeting locations and actual weekend destinations; public coverage statements do not guarantee a particular road, address, entrance or point of interest. [Routing coverage](https://dev.2gis.ru/api/routing-api)

## Motorcycle routing: useful, with clear limits

The current Routing API accepts `transport: "motorcycle"` at `POST https://routing.api.2gis.com/routing/7.0.0/global?key=API_KEY`. It can exclude dirt roads, toll roads and ferries, and return either a summary or detailed geometry. 2GIS describes motorcycle routing as using regular car-route parameters while accounting for faster movement through traffic. The reviewed documentation does **not** establish a scenic/curvy-road preference, motorcycle suitability ratings or verified road-condition advice. [Transportation types](https://docs.2gis.com/en/api/navigation/routing/examples/types)

For a normal motorcycle route, the documented limit is ten points including start/end. A demo key limits distance between route points to 50 km. One route-point set is billed as one calculation, including alternative route options returned in that request; for multiple origin/destination pairs, each pair is billed separately. These units matter when estimating route-planner cost. [Routing limits and billing](https://docs.2gis.com/en/api/navigation/routing/overview)

For BIKERS.KZ, start with organizer-selected stops and an explicit calculate action. Do not recalculate on every map movement or every rider opening an unchanged ride. This is a cost-control proposal; any storage/reuse of vendor-generated route geometry must follow the applicable license.

## Keys, permissions and backend ownership

2GIS supports access-key restrictions using HTTP headers (`Referer`, `Origin`, `User-Agent`), IP/subnets and mobile application identifiers. Restrictions apply to all services on a key. Subscription/key usage limits can block requests when exhausted, and the Platform Manager exposes consumption statistics and notifications. [Key management](https://docs.2gis.com/en/platform-manager/subscription/managing-keys), [usage controls](https://docs.2gis.com/en/maps/overview)

Our proposed split is a browser key restricted to the approved BIKERS.KZ origins for map rendering, plus a separate server-side key for paid search/routing if those features are added. A key delivered to the browser is visible to the browser; restrictions and service quotas are the controls, not hiding the value in a frontend environment variable. Debounce autocomplete, cancel superseded requests, cap each user’s server-side usage, and show a recoverable error if the vendor or quota is unavailable.

The current [ride model](../apps/backend/internal/models/models.go) stores `meeting_point`, `destination` and `route_url` as text, without structured coordinates. Embedded maps require an explicit coordinate/waypoint model and database migration; existing text must not be silently bulk-geocoded. Preserve the current approved-participant access boundary for exact meeting details, route endpoints and private overlays. Public ride previews must not acquire exact coordinates incidentally through map payloads, static images or share URLs. See [current access rules](development.md).

Within the existing backend layers, domain coordinate types and validation belong in `models`; application authorization/quotas belong in the owning manager; provider HTTP calls belong in a `clients/dgis` package implementing manager-owned interfaces; `app` wires settings and clients. Provider calls should run outside database locks. This is a proposed application of the [existing package rules](../AGENTS.md), not a new architecture layer.

## Proposed evaluation before implementation

1. Confirm the Kazakhstan commercial offer and required products, including whether results may be retained for saved community routes.
2. Use a time-limited demo to test mobile map rendering, a pin picker, search in the three UI languages and selected Kazakhstan routes. The 50 km demo restriction means longer touring routes need separate evaluation access.
3. Measure actual tile requests for opening, panning, zooming and returning to a ride map. Measure autocomplete calls per completed selection and route calculations per published ride.
4. Choose a package from those measurements, with quota alerts and a defined exhausted-quota behavior.
5. Scope the first implementation separately: organizer pin selection, approved-participant map and external navigation handoff. Search and motorcycle route calculation are optional follow-ups.

This review does not change the deferred status of a community route library, live tracking, native navigation or offline maps.

## Shortlist of alternatives

| Option | Published commercial starting point | Fit and qualification |
| --- | --- | --- |
| 2GIS RasterJS or MapGL | Separate tile/API subscriptions; see tariff comparison above | Preferred candidate for a Kazakhstan pilot; evaluate actual meeting places and routes before committing. |
| MapTiler Cloud with its SDK or MapLibre | Flex lists **US$30/month excluding VAT**, with 25,000 map sessions through MapTiler SDK or 500,000 API requests for third-party SDK use; 3,000 search sessions | A useful price/coverage comparator. These are different billing metrics: MapLibre does not automatically inherit the SDK session allowance. The free plan is for testing/personal noncommercial use. [MapTiler pricing](https://www.maptiler.com/cloud/pricing/), [MapLibre project](https://maplibre.org/projects/gl-js/) |
| Yandex Maps | Paid tariffs depend on the product and selected offer | Its free-use terms require open/free access, while explicitly allowing ordinary registration. Organizer-approved private ride details need written licensing confirmation; do not treat either “all authentication is forbidden” or “any authenticated page is free” as the rule. [Terms, section 4.1.1](https://www.yandex.ru/legal/maps_api/en/), [tariffs](https://yandex.ru/maps-api/tariffs) |

OpenStreetMap data and MapLibre rendering are separate from a production tile-hosting service. The standard OSM tile server has usage restrictions, no SLA and no permission for offline/bulk downloads. An OSM-based option needs a suitable paid tile provider or a self-hosting/operations budget. [OSM tile-use policy](https://operations.osmfoundation.org/policies/tiles/)
