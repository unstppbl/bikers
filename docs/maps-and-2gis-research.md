# Maps: 2GIS and OpenStreetMap research

Checked **5 November 2026**. This is research and a proposed integration path, not approval to add map features, buy a subscription, or change the existing ride-access policy. No accounts, subscriptions, paid API calls, or vendor contacts were created for this review.

**Recommendation after comparing OSM options:** trial MapLibre with Geoapify's commercial Free tier for a low-traffic pin-and-map pilot, provided its daily quota fits, and compare it against 2GIS RasterJS in the same Kazakhstan test area. Keep MapTiler as a paid comparator and regional self-hosted tiles as an independence option. OSM has no data royalty, while tiles/search/routing still need a provider or infrastructure. Neither provider's local quality has been benchmarked here. Retain external navigation handoff and scope search/routing separately. [OSM license](https://www.openstreetmap.org/copyright), [MapLibre](https://maplibre.org/projects/gl-js/), [Geoapify pricing](https://www.geoapify.com/pricing/), [2GIS map choices](https://docs.2gis.com/en/maps/overview)

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
| 2GIS RasterJS or MapGL | Separate tile/API subscriptions; see tariff comparison above | Candidate for a Kazakhstan pilot; evaluate actual meeting places and routes before committing. |
| MapTiler Cloud with its SDK or MapLibre | Flex lists **US$30/month excluding VAT**, with 25,000 map sessions through MapTiler SDK or 500,000 API requests for third-party SDK use; 3,000 search sessions | A useful price/coverage comparator. These are different billing metrics: MapLibre does not automatically inherit the SDK session allowance. The free plan is for testing/personal noncommercial use. [MapTiler pricing](https://www.maptiler.com/cloud/pricing/), [MapLibre project](https://maplibre.org/projects/gl-js/) |
| Yandex Maps | Paid tariffs depend on the product and selected offer | Its free-use terms require open/free access, while explicitly allowing ordinary registration. Organizer-approved private ride details need written licensing confirmation; do not treat either “all authentication is forbidden” or “any authenticated page is free” as the rule. [Terms, section 4.1.1](https://www.yandex.ru/legal/maps_api/en/), [tariffs](https://yandex.ru/maps-api/tariffs) |

## OpenStreetMap: data, rendering and services

OSM is an open geographic database, not one bundled maps/search/routing product. A practical stack has separate parts:

| Part | Example | What we pay for |
| --- | --- | --- |
| Map data | OSM under ODbL | No data royalty; attribution and license obligations apply. |
| Browser rendering | MapLibre GL JS for vector maps; Leaflet for raster maps | Open-source library; it does not supply hosted map data or search by itself. |
| Tiles, styles and map assets | Hosted provider, or our PMTiles archive/static host | Provider plan, or storage/requests/bandwidth and operations. |
| Address/POI search | Hosted geocoder, or our own search service | Separate service/hosting; displaying a pin does not require it. |
| Road-route calculation | Hosted or self-operated routing engine | Separate service/hosting; a map renderer can draw a supplied route without calculating it. |

[OSM data license](https://www.openstreetmap.org/copyright), [MapLibre project](https://maplibre.org/projects/gl-js/), [Leaflet](https://leafletjs.com/), [Protomaps system overview](https://docs.protomaps.com/)

### Hosted costs for a pilot

KZT estimates below use the NBK's 5 November 2026 rate of **₸452.91 per USD** and exclude applicable taxes and payment fees. [NBK rates](https://nationalbank.kz/en/exchangerates/ezhednevnye-oficialnye-rynochnye-kursy-valyut)

| Option | Published allowance and price | BIKERS.KZ implication |
| --- | --- | --- |
| Geoapify Free | **US$0**, 3,000 credits/day; pricing FAQ explicitly permits commercial production use within the limits and attribution requirements | A practical first trial. The daily pool is shared across APIs, not a monthly allowance. |
| Geoapify API 10 | **US$59/month**, 10,000 credits/day; taxes extra | About **₸26,722/month** at the exchange rate used above. |
| MapTiler Flex | **US$30/month**, excluding VAT; SDK sessions and third-party-renderer requests have separate allowances | About **₸13,587/month**; use the request allowance when choosing MapLibre, as explained in the shortlist. |
| Our regional PMTiles | **US$0 OSM data royalty**; hosting, delivery, build/refresh and operations are ours | No vendor map-call license fee; exact cost needs an archive-size/traffic measurement and hosting choice. |

Geoapify lists 0.25 credit per standard map tile and one credit per simple geocoding/autocomplete request; routing costs depend on waypoints, distance and additional features. If all Free credits were spent on tiles, that is **12,000 tiles/day**, with no room left for other APIs. It is not 12,000 map opens, and unused daily allowance is not a monthly budget. Free use requires Geoapify and data-source attribution, including OpenMapTiles for most styles. [Geoapify prices and production-use FAQ](https://www.geoapify.com/pricing/), [credit rules](https://www.geoapify.com/pricing-details/), [map API accounting](https://apidocs.geoapify.com/docs/maps/), [MapTiler pricing](https://www.maptiler.com/cloud/pricing/)

Geoapify separately documents a motorcycle routing mode and permission to store/redistribute route results, subject to attribution and underlying-data licensing. That makes it worth evaluating for a saved-route feature rather than assuming all hosted API output must be temporary. Local route quality remains untested. [Geoapify motorcycle routing and result reuse](https://www.geoapify.com/routing-api/)

### License and our ride data

OSM permits commercial use. Show visible “© OpenStreetMap contributors” attribution linked to the license information. ODbL does not require disclosing our application source code. Independently collected ride/member data shown as a separate overlay does not automatically become an OSM derivative database; keep its provenance and separation clear. An adapted OSM database or substantial extracted dataset can have share-alike obligations, so do not treat a copied OSM POI directory as our independent data. [Copyright and attribution](https://www.openstreetmap.org/copyright), [ODbL sections 2.3, 3.1 and 4](https://opendatacommons.org/licenses/odbl/1-0/), [OSMF horizontal-layer guidance](https://osmfoundation.org/wiki/Licence/Community_Guidelines/Horizontal_Map_Layers_-_Guideline)

### Public OSM services have separate rules

The standard `tile.openstreetmap.org` service allows compliant normal interactive viewing; it does **not** categorically ban commercial applications. It requires attribution, an identifying client/referrer and caching according to response headers (at least seven days if those headers cannot be read). Bulk/offline downloading and prefetching are prohibited. It has no SLA and access can be withdrawn, so it is unsuitable as our assumed unlimited production infrastructure. These restrictions apply to that public raster service, not to all OSM-based providers or self-hosted data. [OSMF tile policy](https://operations.osmfoundation.org/policies/tiles/)

Public Nominatim allows moderate user-triggered searches at an aggregate maximum of **one request/second for the entire application**. It requires identification, attribution and the ability to switch service; caching is recommended. Autocomplete, systematic POI extraction and sending confidential data are prohibited. Consequently, it is not a drop-in free autocomplete backend for BIKERS.KZ. A commercial geocoder or self-hosted service has its own capacity and terms. [Public Nominatim policy](https://operations.osmfoundation.org/policies/nominatim/)

### Practical Kazakhstan self-hosting path

Our proposed vector-map path is:

`Kazakhstan OSM extract → Planetiler/Protomaps → versioned PMTiles + styles/fonts/icons → static HTTPS hosting → MapLibre in the PWA`

Geofabrik provides a Kazakhstan `.osm.pbf` extract, listed at **215 MB** when checked, plus update files. That is compressed input data, not the finished tileset size or a runtime memory estimate. Planetiler can generate PMTiles from regional OSM data. Alternatively, Protomaps documents extracting a chosen bounding box from its daily basemap builds. Neither path requires downloading tiles from OSMF's public raster server. [Kazakhstan extract](https://download.geofabrik.de/asia/kazakhstan.html), [Planetiler](https://github.com/onthegomap/planetiler), [Protomaps regional extraction](https://docs.protomaps.com/guide/getting-started)

PMTiles can be served by object storage or Nginx/Caddy using HTTP Range requests, with appropriate CORS when hosted on another origin. MapLibre also needs compatible styles, glyphs and sprites. This avoids operating a dynamic tile-rendering database for each map view, but still requires rebuilds/refreshes, versioned publication, capacity monitoring and storage/traffic costs. Keep private rider overlays in the existing authenticated API, separate from the public basemap archive. [PMTiles hosting](https://docs.protomaps.com/pmtiles/cloud-storage), [MapLibre assets](https://docs.protomaps.com/basemaps/maplibre)

Future self-hosted routing could evaluate Valhalla's **beta** motorcycle costing or GraphHopper's example motorcycle/curvature models; these require profile validation and Kazakhstan route tests, and are not interchangeable with a scooter profile. [Valhalla routing reference](https://valhalla.github.io/valhalla/api/route/api-reference/), [GraphHopper example configuration](https://github.com/graphhopper/graphhopper/blob/master/config-example.yml)

For the first release, I would trial **MapLibre plus Geoapify Free**, measure daily tile/search credits and compare actual Kazakhstan roads, addresses and English/Russian/Kazakh labels against 2GIS. Use manual pin selection and external navigation first. Move to paid hosting when the quota or features require it; choose a Kazakhstan PMTiles archive if owning updates/hosting becomes worthwhile. Add geocoding or routing when that feature is approved; neither is included merely by adopting OSM. A combination with 2GIS remains subject to its own cross-provider display/storage terms.

## Can the Kazakhstan stack run on Jarvis?

**Conditional yes for a low-traffic pilot:** serve regional PMTiles, use Photon for address autocomplete and reverse search, and evaluate Valhalla's motorcycle mode. This is a feasibility proposal; no map data has been imported and no runtime capacity or search/route quality has been benchmarked.

### Observed capacity — 5 November 2026

A read-only audit at 08:24:50 UTC found a Macmini6,2 with an Intel i7-3615QM, four physical cores/eight threads, and about **15.5 GiB RAM**. Approximately 5.8 GiB was used and 9.7 GiB available at that moment, with low CPU load. These are a snapshot, not a guarantee about existing workload peaks.

The current root SSD filesystem has about **37 GiB available**. The SSD volume group also has about **344 GiB unallocated**, so additional SSD capacity could be provisioned separately. That unallocated space is not presently free space inside the filesystem. A separate HDD has ample space for cold archives, but the active search/routing indexes should use SSD. The CPU lacks AVX2; verify the selected Java/native binaries and container-image CPU baselines before choosing versions. This research changes no partitions, volumes or services.

### Proposed services and planning budget

All figures below are **initial test allocations**, not upstream Kazakhstan requirements, measured dataset sizes or throughput promises:

| Component | Low-traffic serving allocation | Build/refresh approach |
| --- | --- | --- |
| PMTiles + style/fonts/icons over static HTTP | About **0.25 GiB RAM**, shared CPU; no live tile-rendering database | Generate/extract and validate a new archive separately, then publish a versioned file. |
| Photon, Kazakhstan-filtered snapshot | About **4 GiB total RAM**, initially around 2 GiB Java heap; shared CPU | Import JSON into a separate index, validate, swap and retain rollback. |
| Valhalla, country road graph | About **2–3 GiB total RAM**, shared CPU | Build a separate graph; validate motorcycle routes before switching. |
| Combined map services | Roughly **6–8 GiB RAM including overhead**, initially **2–4 shared CPU threads** | Reserve **100 GiB SSD** operational capacity for current/next data, inputs, temporary files and rollback; this is not an estimate that the actual indexes occupy 100 GiB. |

OpenSearch recommends reserving roughly half of available service memory for the Java heap and leaving room for native memory/file caching. Do not equate Photon heap size with total RAM, or allocate half of all host RAM to each Java service. [OpenSearch memory guidance](https://docs.opensearch.org/latest/install-and-configure/configuring-opensearch/configuration-system/)

Jarvis has plausible runtime headroom for that pilot budget, but its current filesystem does not contain the proposed SSD reserve. A staged import may need an exploratory **6–8 GiB** allocation for one job; it is unmeasured. Building a replacement Photon index or routing graph while both old services and existing workloads run can exceed this 16 GiB host. Staggering jobs alone does not prove they fit. Build data elsewhere, or use a planned, isolated maintenance window with explicit resource caps and abort criteria; measure before committing to routine refreshes.

### Address search: Photon first, Nominatim when needed

Photon directly supports search-as-you-type, typo tolerance, location bias, multilingual names and reverse lookup. Its current stack is Java 21+ with embedded OpenSearch; an external cluster is optional. For the pilot, import a JSON snapshot filtered to Kazakhstan and explicitly request `kk`, `ru` and `en` names. The provider lists a Central Asia JSON dump that includes Kazakhstan; verify its data timestamp. Importing a snapshot avoids permanently operating Nominatim. [Photon capabilities/requirements](https://github.com/komoot/photon), [country/language filtering and import](https://github.com/komoot/photon/blob/master/docs/usage.md), [regional dump](https://download1.graphhopper.com/public/asia/central-asia/index.html)

Prebuilt Photon database dumps have a limited language set; using JSON permits our chosen languages, but cannot create missing translations or missing addresses. Photon can serve the imported index independently. Its incremental-update path currently depends on a Nominatim database and coordinated update runs; otherwise we replace snapshots, keeping at least current and next indexes. [Photon update/deployment workflow](https://github.com/komoot/photon/blob/master/docs/usage.md), [snapshot replacement](https://github.com/komoot/photon/blob/master/README.md)

Nominatim is an alternative for explicit submitted address searches and reverse geocoding, using PostgreSQL/PostGIS. It is not the search-as-you-type choice. Self-hosting removes the public server's one-request/second policy, but does not turn its search API into a dedicated autocomplete engine. Country extracts and daily replication are supported. Initial import runs osm2pgsql and PostgreSQL together; osm2pgsql alone needs at least 2 GiB plus its node cache, with PostgreSQL using additional memory. Do not interpret the documented 2 GB installation floor as sufficient for the full import stack. No official Kazakhstan sizing benchmark was found. [Nominatim search](https://nominatim.org/release-docs/latest/api/Search/), [import/memory requirements](https://nominatim.org/release-docs/latest/admin/Import/), [regional updates](https://nominatim.org/release-docs/latest/admin/Update/), [public-service policy scope](https://operations.osmfoundation.org/policies/nominatim/)

### Motorcycle routing scope and worker limits

Evaluate Valhalla's dedicated **beta `motorcycle`** costing with basic point-to-point routes, initially allowing one or two simultaneous calculations. Explicitly cap `server_threads` at one or two: the scripted image defaults to all detected CPUs and uses this setting for both graph construction and serving. A country-only graph cannot route over roads missing outside its extract; define neighboring coverage deliberately for cross-border rides. [Motorcycle costing](https://valhalla.github.io/valhalla/api/route/api-reference/), [Docker worker/build settings](https://github.com/valhalla/valhalla/blob/master/docker/README.md)

Start without elevation files or live traffic. Elevation adds its own storage/build work; current traffic requires an external data feed and is not supplied by an OSM extract. Keep graph refreshes staged and versioned rather than letting an unplanned rebuild compete with the running application. [Elevation/build options](https://github.com/valhalla/valhalla/blob/master/docker/README.md), [speed and traffic inputs](https://valhalla.github.io/valhalla/concepts/speeds/)

### Validation before deployment

Start with a separate country-scoped data build. Record peak process/container memory including file cache, CPU, temporary disk and final archive/index/graph sizes. Test cold and warm queries, concurrent autocomplete, and actual Kazakh/Russian/English addresses, house numbers, renamed streets and rural meeting points. Measure latency and missing/wrong results; language configuration is not a coverage guarantee. Then rehearse one complete refresh with current data retained, monitoring existing application latency and available memory/disk. Only those measurements can confirm the final allocation and safe refresh procedure. No downloads, imports, benchmarks, capacity changes or deployment were performed in this review.
