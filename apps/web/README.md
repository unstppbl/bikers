# BIKERS.KZ web client

Mobile-first React, TypeScript and Vite PWA for rides and rooms, structured garages with private maintenance, and owner-approved clubs. The Go service serves the production `dist/` directory and the same-origin `/api/v1` API. No runtime frontend environment variables or standalone frontend server are needed in production.

Use `npm ci`, then `npm run dev`. The development server proxies HTTP and WebSocket requests under `/api` to `127.0.0.1:8080`; configure the backend's exact browser origin accordingly. `npm run build` creates `dist/`. `npm run typecheck`, `npm run lint`, and `npm test` verify the client.

For the temporary motorcycle visual study, run `npm run prototype` while the existing local backend is running on port 33043. Open `http://localhost:5173/?variant=A` and use the floating arrows to compare Night Ride (`A`), Rally Roadbook (`B`) and Clubhouse (`C`). The preview keeps ride fetching read-only; empty data uses explicitly marked sample rides. Search and preview details stay in memory. The preview and switcher are gated to development builds. The default route remains the current application.

The application bundles the unmodified [Roboto Condensed font from Google Fonts](https://github.com/google/fonts/tree/main/ofl/robotocondensed) in `src/assets/fonts/`, with its SIL Open Font License alongside it and delivered at `/font-licenses/RobotoCondensed-OFL.txt`. Font loading uses a local asset rather than a third-party request, and the installed PWA precaches it. The development study retains its own copy in `src/pages/prototype-assets/`.

The service worker precaches application assets only. API and media paths have no runtime caching, and navigation fallback excludes both. Cookies remain browser-managed; CSRF material, OTP challenges, and drafts stay in memory. Browser storage contains only the selected UI locale. Sign-out, session expiry and confirmed access loss clear protected client state. Async mutation completions are bound to the original actor and in-memory session epoch, so delayed responses cannot restore another session’s cache or navigate its UI. Cookie-dependent public views wait for session resolution and whitelist guest projections.

English, Russian and Kazakh strings live in `src/i18n.tsx`. Translation review by native speakers remains part of the public-release gate. The client renders server-authorized ride views and never derives permission from a local profile or UI control.

PWA icons are generated from `public/app-icon.svg` by `node scripts/icons.mjs`. The committed PNG variants support installed apps and Apple touch icons.

Garage and club collection pages use server cursors. Motorcycle sharing is explicit; maintenance never appears in a public projection. Club membership does not grant access to ride plans or rooms. Owner transfers, closure, deletion and sharing use accessible in-page confirmation dialogs with focus management and cancellation. Photos and uploads remain outside this slice.

Automatic motorcycle references come from the public model catalogue, separately from owner bike records. The client requires an exact canonical model ID and a verified year/generation range; it never substitutes a nearby year or another model. Only local `/bike-models/` image assets render, with visible author/source/license credits. Unsupported or failed images are omitted; valid known-model forms explain when a reference is unavailable. These catalogue references are separate from owner-uploaded photos. Public bike payloads remain the same five basic fields, and model image requests use a shared public query cache. JPEG references are not added to the PWA's static-shell precache.
