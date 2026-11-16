# Night Ride design review

On 4 November 2026 the owner approved the deployed Night Ride design and asked
to apply Unslop and Stageshow. This pass refines that established direction.
It does not change the agreed MVP scope or add catalogue commitments.

## Direction and audit

The product is a Kazakhstan-first motorcycle community. Its tone is direct,
mechanical and sociable, drawing from asphalt, garages and instruments.
Motorcycle photography and the approval-to-Ride-Room flow supply the product
specificity. Condensed headings, charcoal surfaces and the lime accent are
the approved baseline.

The audit used [Unslop by Damir Sokolovsky](https://github.com/heydamir/unslop)
(CC BY 4.0), including its spacing and token guidance. It classified the
interface as needing light polish. The existing palette already had sufficient
measured text contrast, so it was retained. Generic suggestions to reduce
display weight or remove the existing uppercase labels were not applied:
the owner had approved this visual direction.

The useful findings were oversized sidebar rows that overflowed shorter
screens, small header hit targets, incomplete mobile drawer keyboard handling,
and missing focus/scroll handling between routes.

## Implemented polish

- Navigation rows use a 44px minimum target and a regular spacing grid.
  The seven-link administrator menu fits a 720px-high desktop viewport.
- Language, avatar, menu and primary header controls have 44px hit targets.
- Mobile navigation exposes a labelled modal dialog, moves focus to Close,
  traps Tab within the drawer, closes with Escape or a desktop resize, restores
  focus and prevents interaction or background scrolling behind the drawer.
- New route navigation focuses the main content and starts at the top.
  Back/Forward restoration, same-page anchors and query-only changes retain
  their appropriate browser behavior.
- Motion durations share tokens. Reduced motion removes hover movement,
  spinner animation and animated drawer transitions.

## Authored ride walkthrough

[Stageshow by Benji Taylor](https://github.com/benjitaylor/skills/tree/main/stageshow)
provided the compact window, measured cursor, clock and transition primitives.
The illustration shows a public preview, join request, organizer approval,
then the private meeting plan and group chat. Its content is fictional and
labelled as an example. It does not call the application API.

Run `npm run prototype` from `apps/web`, then open
`http://localhost:5173/preview/ride-flow`. English, Russian and Kazakh have
localized copy, manual Play/Replay controls and a complete static poster for
reduced motion. The preview includes a visible reduced-motion toggle for
inspection. It is excluded from production builds.

The source is in `apps/web/src/components/ride-demo/`. Its 26 vendored kit
files remain byte-identical to the installed skill; the MIT licence, source
notice and SHA-256 manifest are retained. The narrow lint exclusion covers
only that unchanged vendor directory. Authored code remains linted.

Three 13-second MP4s and their posters are retained under
`/Users/user/.codex/visualizations/2026/11/01/01a197b8-57c6-7a12-946b-c767709cb502/ride-flow/`.
The component's `REHEARSAL.md` records the gold reference, frame inspection,
detector findings and corrections. Automated timing coverage remains
inconclusive because of host animation-frame sampling gaps; the exported
20fps video does not establish real-device frame rate or Safari behavior.

## Verification

All 90 frontend tests, TypeScript, ESLint and the production build passed.
Two navigation regressions cover modal focus behavior and route/history
handling. Actual-browser review covered the owner/admin desktop menu,
390px controls and drawer, manual playback, static rendering, Russian copy
and the compact 320px Kazakh view without horizontal overflow.

The production asset search found no walkthrough route, scene styles, example
meeting copy or preview controls. The font remains locally bundled with its
licence and the PWA precache contains 19 entries. Current app-only rollout
evidence is recorded in [Operations](operations.md#current-deployment).

## Design system refresh · 4 November 2026

The next refresh applies [Landing Page Design](https://github.com/elayadesign/ai-design-skills/tree/main/skills/landing-page-design)
and its [existing-project redesign companion](https://github.com/elayadesign/redesign-skill/tree/main/skills/redesign-existing-projects).
Both skills are installed in the owner's Codex skills directory. The application
keeps the Night Ride charcoal and lime identity while replacing condensed,
italic display typography with one locally bundled Geist family. Its font
character map includes the complete English, Russian and Kazakh alphabets.
The motorcycle image and its visible source/licence credits are retained.

The shared system has named text styles, a closed spacing scale, flat neutral
surfaces, calmer lime actions and consistent corners. Navigation separates
Explore, Your space and Account; its mobile drawer uses the full screen. The
header sits inside a floating, bordered surface. Ride discovery has one primary
hero action, readable ride rows and paired date filters that stack below 361px.
Forms, collection tabs and empty states share the same visual language. Loading
and session resolution use skeletons. Phosphor replaces the authored Lucide
imports; decorative icons are hidden from assistive technology. A localized
skip link and a branded recovery page complete keyboard navigation and 404s.

`src/shell.css` owns application chrome and `src/discovery.css` owns discovery
and ride rows. Their rules were extracted from the shared stylesheet, including
old responsive overrides. The default page width has zero selector specificity
so narrower profile, settings and form containers retain their own limits.
The backend, authorization contracts, public ride projections and stored data
are outside this visual refresh.

Browser verification covered the 1280px desktop layout, 390px Russian
navigation/profile/clubs/garage, and 320px Kazakh discovery and filters, with no
horizontal overflow. The 320px check caught native date clipping; the final
layout gives dates a full row at that width. Menu Escape restores trigger focus,
route navigation focuses main content, and the desktop ride form retains its
900px container. The local backend's collection data was empty, so populated
collection behavior is covered by the existing tests rather than fabricated
browser content. Native city suggestions remain subject to the local backend's
available API version.

The final `make check-web` run passed all 129 tests, TypeScript, ESLint and
the production/PWA build. The output bundles Geist locally and continues to
exclude development previews.

This source refresh has not been deployed. Production activation and its
verification must follow the repository deployment procedure in Operations.

## Compact application layout · 16 November 2026

The owner approved a compact direction across the application in issue
[#37](https://github.com/unstppbl/bikers/issues/37). The charcoal/lime palette,
local Geist font and Phosphor icons remain the visual foundation. Page titles
now name their screen or task in English, Russian and Kazakh.

The shared header, navigation, page gutters, forms and content panels use less
space. Navigation switches to the mobile drawer at 900px so the sidebar does
not consume the useful width of a landscape phone. Inputs retain 16px text;
touch controls, keyboard focus and reduced motion remain supported. Ride
discovery starts with filters and compact rows; its instructions and privacy
guidance are available in an inline disclosure below the results. Date filters
expand inline and open automatically when dates are present in the URL.

Map is a viewport workspace with a scrolling results/details pane beside the
canvas on desktop. Phones provide Map/List controls and one details region.
Search stays visible, while city/category filters expand in a bounded panel.
On mobile, My suggestions is available inside Filters. Short screens can scroll
the workspace and use a horizontal map control rail, preserving enough canvas
for controls and attribution.

MapLibre clusters nearby places. Accessible cluster buttons expand the map;
places at identical coordinates remain individually selectable in a chooser.
The selected place has its own marker, including when opened from outside the
current result page. Source revisions guard pending cluster requests, marker
updates preserve keyboard identity, and offscreen markers leave the tab order.
Map sizing has one owner; canvas, controls, chooser, status and attribution have
separate reserved regions. Loading/error text sits below the map viewport.

This change affects frontend presentation and map interaction only. Server
permissions, public/private projections, stored records and the release
workflow continue to use their existing contracts.

Validation: `make check-web` passes all 254 tests, TypeScript, ESLint and the
production/PWA build. New regressions cover clustering, stale expansion
requests, coincident points, off-page selections, category corrections,
keyboard identity and focus restoration, and the mobile Map/List flow.
Existing authorization, paging/history and public/private projection coverage
continues to pass.

Browser review used a local read-only fixture server with populated ride,
club, route, garage, settings and place views and the public self-hosted
basemap. Checks covered English, Russian and Kazakh, 320px/390px phones,
844×390 landscape selection and scrolling, and a 1280×800 desktop map. The
mobile drawer opens with Close focused and returns focus on Escape. These
Chromium viewport checks do not establish physical iOS/Safari acceptance.
No production records, accounts or runtime configuration were changed.
