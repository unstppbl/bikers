# BIKERS.KZ web client

Mobile-first React, TypeScript and Vite PWA for rides and rooms, structured garages with private maintenance, and owner-approved clubs. The Go service serves the production `dist/` directory and the same-origin `/api/v1` API. No runtime frontend environment variables or standalone frontend server are needed in production.

Use `npm ci`, then `npm run dev`. The development server proxies HTTP and WebSocket requests under `/api` to `127.0.0.1:8080`; configure the backend's exact browser origin accordingly. Set `BIKERSKZ_DEV_API_TARGET` to use another backend, such as the local review instance at `http://localhost:33043`. `npm run build` creates `dist/`. `npm run typecheck`, `npm run lint`, and `npm test` verify the client.

The application self-hosts [Fira Sans](https://fontsource.org/fonts/fira-sans) for text and [Fira Sans Extra Condensed](https://fontsource.org/fonts/fira-sans-extra-condensed) (700 and 800) for headings and the wordmark, from the `@fontsource` packages. `src/fonts.ts` imports only the weights the interface uses. The Latin and Cyrillic subsets cover the complete English, Russian and Kazakh alphabets, including Ә, Ғ, Қ, Ң, Ө, Ұ, Ү, Һ and І. Vite bundles the font files, the installed PWA precaches the WOFF2 files, and the SIL Open Font License is delivered at `/font-licenses/FiraSans-OFL.txt`.

Every screen has the top bar. On screens up to 900px wide, a fixed tab bar adds Rides, Map (also current on Routes), New ride, Clubs and Me. `/me` gathers the rider's own pages (My rides, garage, clubs, inbox), the shared sections, settings, administration for administrators, and sign-out. The inbox, settings, install, administration and profile pages link back to it. `App.tsx` keeps one typed route table, and the current top-bar link and tab come from the matched route's section. The current tab is shown by colour and by a filled icon.

`src/shell.css` owns the top bar, the phone tab bar, the footer and the Me page; `src/discovery.css` owns the rides feed, ride cards, filters, date plates and city art; `src/collections.css` owns the garage, motorcycle, club and My rides collections; `src/pages/ride-detail.css` owns the ride page; `src/components/ride-cover.css` owns the ride cover editor on the ride form. Shared controls, page headings and feature surfaces remain in `src/styles.css`. `main.tsx` loads `shell.css`, `discovery.css`, `ride-detail.css` and `collections.css` after `styles.css` so their rules take precedence. Components and lazily loaded pages import their own CSS beside them, such as `ride-cover.css`, the map and planner styles in `src/components/maps/`, and `src/pages/places.css` and `src/pages/routes.css`. On phones the rides feed and collection pages keep large display titles; every other page title uses `--type-heading-4`.

`src/theme.css` defines the warm asphalt surfaces, bone text, signal-orange accent, cream plate color, semantic status colors, spacing and radius scales, and named typography styles. Headings use the condensed display face; titles and body text use Fira Sans. Each text style is available as a class such as `heading-1` or `body-3` and a font shorthand token such as `--type-heading-1`. Color transitions use 180ms and panel transitions use 280ms with the shared deceleration curve; reduced motion removes movement durations.

The service worker precaches application assets only. API and media paths have no runtime caching, and navigation fallback excludes both. Cookies remain browser-managed; CSRF material, OTP challenges, and drafts stay in memory. Browser storage contains only the selected UI locale. Sign-out, session expiry and confirmed access loss clear protected client state. Async mutation completions are bound to the original actor and in-memory session epoch, so delayed responses cannot restore another session’s cache or navigate its UI. Cookie-dependent public views wait for session resolution and whitelist guest projections.

English, Russian and Kazakh strings live in `src/i18n.tsx` and the feature files it merges, such as `src/shell-i18n.ts` and `src/photos-i18n.ts`. Translation review by native speakers remains part of the public-release gate. The client renders server-authorized ride views and never derives permission from a local profile or UI control.

PWA icons are generated from `public/app-icon.svg` by `node scripts/icons.mjs`. The SVG holds the wordmark, BIKERS over an orange KZ plate, as outlined paths, so it renders without fonts; keep new artwork inside the central 80% maskable safe zone. The script writes rounded 192px and 512px icons, an opaque 180px Apple touch icon and an opaque 512px maskable icon.

Ride, garage and club collection pages use server cursors. Discovery applies city, inclusive date and public-description filters on the server. `/my-rides` provides Organizing, Joined, Pending and Past views. Filters and page cursors live in the URL; changing filters, views or actors resets paging, and Back/Forward restores the current view. Cursor errors offer a return to the first page. Past includes earlier/cancelled organized or approved/requested rides and does not prove attendance. The shared native city selector uses `/cities`, localizes suggestions and permits manual entry when a city is absent or suggestions are unavailable. Profile, ride and club forms use the same selector. Club ride linking pages through the organizing view. Motorcycle sharing is explicit; maintenance never appears in a public projection. Club membership does not grant access to ride plans or rooms. Owner transfers, closure, deletion and sharing use accessible in-page confirmation dialogs with focus management and cancellation. Written-out dates leave out the year while it is the current one and show it for older records and rides in another year; the date plate always shows only the day and month.

Riders upload avatars, garage bike photos, club images, ride album photos and one public cover per ride. `src/components/PhotoInput.tsx` accepts JPEG, PNG or WebP files up to 12 MiB and previews them locally before upload. The client keeps a photo only when its metadata names the exact authorized same-origin content route for that object. Ride summaries embed the current cover, so cards need no extra request; a ride without a cover shows its city name as artwork (`CityArt`). Once a ride exists, the organizer adds, replaces or removes its cover on the edit form (`src/components/RideCover.tsx`), where a note says the cover is public; other signed-in riders can report it. Ride videos remain external links.

Automatic motorcycle references come from the public model catalogue, separately from owner bike records. The client requires an exact canonical model ID and a verified year/generation range; it never substitutes a nearby year or another model. Only local `/bike-models/` image assets render, with visible author/source/license credits. Unsupported or failed images are omitted; valid known-model forms explain when a reference is unavailable. These catalogue references are separate from owner-uploaded photos. Public bike payloads remain the same five basic fields, and model image requests use a shared public query cache. JPEG references are not added to the PWA's static-shell precache.

`/bikes` is the public catalogue of members' explicitly shared garage bikes,
separate from that model reference lookup. Guests and members can search by
make/model, page through results and open public bike details. Search/cursors
live in the URL; session changes reset paging and abort old requests. Catalogue
cache entries retain only the five public bike fields for every viewer,
including the owner. Cards reuse the authorized uploaded cover or verified
model reference. Publishing consent names catalogue discovery; sharing, hiding,
editing and deletion invalidate the collection. New bikes remain private.

The inbox combines ride updates and club membership requests/decisions. Known
club kinds link only to their validated `club_id`; ride kinds link only to
`ride_id`. Missing, conflicting or unknown targets have no action link. Read
state, pagination and EN/RU/KK labels are shared. Push notifications continue
to open the inbox with generic text rather than exposing membership details.

## Club discovery and ride schedules

`/clubs` supports city/name search; `/my-clubs` separates Organizing, Joined and
Pending with the same actor-bound URL/cursor behavior as rides. Navigation and
the header display a localized unread inbox count, capped visually at `99+`;
reading notices invalidates the shared count and polling refreshes it.

Settings has independent push categories. Muting push never hides durable inbox
updates. Approved riders/organizers can choose 24-hour and/or 1-hour departure
reminders on each ride; neither is selected by default. The private calendar
download uses an abortable, same-origin `no-store` request and discards late
responses after an account/session change. A visible hint explains that downloaded
events do not synchronize future changes. All controls and guidance are in
English, Russian and Kazakh.
