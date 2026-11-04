# BIKERS.KZ web client

Mobile-first React, TypeScript and Vite PWA for rides and rooms, structured garages with private maintenance, and owner-approved clubs. The Go service serves the production `dist/` directory and the same-origin `/api/v1` API. No runtime frontend environment variables or standalone frontend server are needed in production.

Use `npm ci`, then `npm run dev`. The development server proxies HTTP and WebSocket requests under `/api` to `127.0.0.1:8080`; configure the backend's exact browser origin accordingly. `npm run build` creates `dist/`. `npm run typecheck`, `npm run lint`, and `npm test` verify the client.

For the temporary motorcycle visual study, run `npm run prototype` while the existing local backend is running on port 33043. Open `http://localhost:5173/?variant=A` and use the floating arrows to compare Night Ride (`A`), Rally Roadbook (`B`) and Clubhouse (`C`). The preview keeps ride fetching read-only; empty data uses explicitly marked sample rides. Search and preview details stay in memory. The preview and switcher are gated to development builds. The default route remains the current application.

The application bundles the unmodified [Geist variable font from Google Fonts](https://github.com/google/fonts/tree/main/ofl/geist) in `src/assets/fonts/Geist.ttf`, with its SIL Open Font License alongside it and delivered at `/font-licenses/Geist-OFL.txt`. The font supports the complete English, Russian and Kazakh alphabets, including Ә, Ғ, Қ, Ң, Ө, Ұ, Ү, Һ and І. Body text, navigation and headings share this typeface, with interface weights capped at 700. Font loading uses a local asset and the installed PWA precaches it. The earlier Roboto Condensed asset and its licence remain available for the development study in `src/pages/prototype-assets/`.

`src/shell.css` owns the application chrome and navigation; `src/discovery.css` owns the discovery hero, shared ride rows and ride filters. Shared controls and feature surfaces remain in `src/styles.css`.

`src/theme.css` defines the shared neutral surfaces, lime accent, semantic status colors, spacing and radius scales, and named typography styles from the installed landing page design skill. Each text style is available as a class such as `heading-1` or `body-3` and a font shorthand token such as `--type-heading-1`. Color transitions use 300ms and layout transitions use 500ms with the shared deceleration curve; reduced motion removes movement durations.

The service worker precaches application assets only. API and media paths have no runtime caching, and navigation fallback excludes both. Cookies remain browser-managed; CSRF material, OTP challenges, and drafts stay in memory. Browser storage contains only the selected UI locale. Sign-out, session expiry and confirmed access loss clear protected client state. Async mutation completions are bound to the original actor and in-memory session epoch, so delayed responses cannot restore another session’s cache or navigate its UI. Cookie-dependent public views wait for session resolution and whitelist guest projections.

English, Russian and Kazakh strings live in `src/i18n.tsx`. Translation review by native speakers remains part of the public-release gate. The client renders server-authorized ride views and never derives permission from a local profile or UI control.

PWA icons are generated from `public/app-icon.svg` by `node scripts/icons.mjs`. The committed PNG variants support installed apps and Apple touch icons.

Ride, garage and club collection pages use server cursors. Discovery applies city, inclusive date and public-description filters on the server. `/my-rides` provides Organizing, Joined, Pending and Past views. Filters and page cursors live in the URL; changing filters, views or actors resets paging, and Back/Forward restores the current view. Cursor errors offer a return to the first page. Past includes earlier/cancelled organized or approved/requested rides and does not prove attendance. The shared native city selector uses `/cities`, localizes suggestions and permits manual entry when a city is absent or suggestions are unavailable. Profile, ride and club forms use the same selector. Club ride linking pages through the organizing view. Motorcycle sharing is explicit; maintenance never appears in a public projection. Club membership does not grant access to ride plans or rooms. Owner transfers, closure, deletion and sharing use accessible in-page confirmation dialogs with focus management and cancellation. Photos and uploads remain outside this slice.

Automatic motorcycle references come from the public model catalogue, separately from owner bike records. The client requires an exact canonical model ID and a verified year/generation range; it never substitutes a nearby year or another model. Only local `/bike-models/` image assets render, with visible author/source/license credits. Unsupported or failed images are omitted; valid known-model forms explain when a reference is unavailable. These catalogue references are separate from owner-uploaded photos. Public bike payloads remain the same five basic fields, and model image requests use a shared public query cache. JPEG references are not added to the PWA's static-shell precache.

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
