# Design review

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

Run `npm run dev` from `apps/web`, then open
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

## Map city focus and speed cameras

Issue #40 refines Map without changing the shared shell. The initial canvas
opens on Astana at city zoom. Applying a suggested city focuses its catalogue
point, even when the community directory has no results. Changing cities
clears the previous area filter; explicit area links and selected places keep
their location intent. Late city metadata cannot override an opened place or
manual panning within an explicit area. Search text remains a place search.

City/category filters open initially and remain open after applying or clearing
them. The bounded panel still scrolls on phones and can be collapsed to leave
more canvas. Speed cameras use a shared camera icon and EN/RU/KK category
labels across filters, markers, details, submissions and moderation. Camera
locations follow the existing review flow and require schema 13; the migration
adds no camera records.

Validation includes the actual MapLibre Mercator camera, filter/history and
submission regressions, plus read-only local desktop and Russian phone
previews with the public basemap. Phone previews confirm the open panel can
scroll independently of the map. Physical iOS/Safari acceptance remains open.

## Photo-led redesign · 19 November 2026

The owner approved a dark, photo-led direction in issue
[#46](https://github.com/unstppbl/bikers/issues/46). It replaces the Night
Ride charcoal and lime palette, the Geist font, and the sidebar and drawer
navigation. The entries above record those designs as history.

Surfaces are warm asphalt (`#131211`, `#1d1b19`, `#2a2724`), text is bone
(`#f3eee6`) and actions are signal orange (`#ff5a1f`). Green marks approval
only, so save and send confirmations use bone text. Text uses Fira Sans and
headings use Fira Sans Extra Condensed, both self-hosted through Fontsource.
Each font file holds one subset and declares its Unicode range, so a page
downloads only the Latin or Cyrillic files its text needs. Kazakh letters are
in the extended Cyrillic subset. The Greek and Vietnamese files stay out of
the offline precache.

The ride date sits on a cream number plate on ride cards and the ride page. It
is the one signature element; everything else stays quiet. The wordmark puts
KZ on a matching orange plate, and the app icons and PWA colours use the same
wordmark and palette. A ride without a photo shows its city name as large
background type, sized to the length of the name. On phones the city type
starts 12px after the plate, so the two do not overlap.

Ride dates, timestamps and the date filter summary show the year only when it
is not the current year. Ride dates and the filter compare with the current
year in Kazakhstan; timestamps compare in their own time zone. Maintenance
dates always show the year.

Every screen has a top bar. Wide screens show Rides, Map, Routes, Clubs and
Motorcycles with the inbox bell, the avatar and New ride. Phones up to 900px
wide get a bottom tab bar: Rides, Map, a New ride plate, Clubs and Me. The
current tab also fills its icon, so it is not marked by colour alone. The new
Me page groups My rides, Garage, My clubs and Inbox; then Routes, Motorcycles,
Map and Install; then Settings, Admin and Sign out. Inbox, Settings, Admin,
Install and the profile editor lead back to Me.

The rides feed has a large title, one filter bar and photo or typographic
cards; the first card on the first page spans two columns. The ride page opens
with the plate and city art, then shows the description, the plan as a route
line, the map, announcements, chat and album beside the rider list. Garage,
motorcycle and club collections use large titles. Motorcycle cards show
edge-to-edge photos with a year plate, and the club and motorcycle lists use
pill filters. On phones, the rides feed title is 56px and collection list
titles are 44px. Forms, settings and administration use the 36px ride title
size, and so do sign-in, the 404 page, and the ride, route, club, motorcycle,
public motorcycle and profile pages. On wider screens the 404 page and the
ride, route, club, motorcycle and public motorcycle pages use the 56px page
title.
The sign-in page has one heading on a plain surface. Photo pickers use a
styled Choose a photo button instead of the browser's file control, which
showed its own words in the browser's language. On phones, Map opens with its
filters closed to leave room for the map.

The community badge looks the same on every page: a green label and a muted
note. Public and private motorcycles in the garage use the same neutral pill,
with a globe or a lock icon. The 404 icon uses the subtle text colour. Unused
copy was removed: 37 strings in each language, from 908 to 871 keys. Every
city field and filter, the profile and ride forms included, checks the
backend's limit of 160 UTF-8 bytes on the same value the backend stores (a
catalogue city or the trimmed text): about 80 Cyrillic letters. A longer name
is not cut while the rider types; the field shows a message, also in the
filter bars, and is marked invalid instead.

Organizers add, replace or remove one public cover photo per ride on the
ride's edit page. The editor says that the cover is public, keeps the current
cover visible until a replacement finishes and asks before removing it. A new
ride gets its cover after it is created. Cards and the ride page show the
cover under the date plate with a dark scrim. Rides without a cover, or with a
cover that fails to load, keep the city card. Other riders can report a cover.
The development-only ride design study (Night Ride, Rally Roadbook and
Clubhouse), its Roboto Condensed font and the `npm run prototype` script were
removed.

The ride pages were compared with the mockups at 390, 768 and 1440px in
English, Russian and Kazakh. Long city names now wrap at their spaces and
start clear of the date plate and below any status pills. As in the mockups,
the type bleeds a few pixels past the right edge of the card. Card titles keep to two lines, as in the mockups. Past and
cancelled rides say Approved rather than You’re going. On the ride page, Edit
plan sits beside Copy link and the report controls sit on the right. The map
attribution stays on one line inside the map, and Apply in the ride filters
keeps keyboard focus.

Validation: `make check-web` passes TypeScript, ESLint, all 431 tests in 32
files and the production/PWA build; the precache holds 52 entries. New
regressions cover ride covers on cards and the ride page, the cover editor and
its reports, the Me page and sign-out, back links, years in dates, the sign-in
heading, the phone Map filters, the garage pill, revealing focus under the
bars, place list button names and the city limit, including the profile and
ride forms. `make check-backend` passes the architecture check, Go tests and
vet. An earlier `make check-integration` run
on the same backend source passed against a disposable PostgreSQL database.
The 148 script tests pass; one parser test runs only in its Linux job.

Browser review ran the app in Chrome against a mocked API, with a frozen clock
and an offline map stub. It captured all 62 catalogue views, covering every
route and the cover editor, administration and participant menu states, at
390, 768 and 1440px in English, Russian and Kazakh. The 558 full-page
screenshots show no page, console or map errors, no unanswered requests, no
broken images and no horizontal overflow. A second pass over the same views
called `.focus()` on each control in page order. It skipped visually hidden
inputs and controls inside the bars. It does not establish real Tab order or
keyboard use of the photo pickers. No measured control was fully covered by
the sticky top bar or the bottom tab bar. Chrome scrolls a focused button
clear of the bars, but not a text field: the announcement text area sat 52px
under the tab bar at 390 and 768px. The shell now scrolls a focused element
into view when a bar is on top of it and the focus is visible. The text area
now sits 16px above the tab bar. Tapped buttons and focus inside a bar or
dialog do not scroll. The root scroll padding that keeps content clear of the
sticky top bar also counted the bar's own controls as covered, so focusing one
while the page was scrolled moved the page up by 390 to 460px. The bar's
controls now opt out of that padding. A real-keyboard check of the ride page
and the rides feed at 390, 768 and 1440px in all three languages found that no
top bar control or tab bar link moves the page; the fixed tab bar never did.
After the skip link, the next Tab lands at least 16px below the top bar. Text on solid surfaces measured at least 4.5:1 (lowest 4.76:1); text
on photos was checked by eye on its scrim. Controls are at least 24px, and at
least 44px high on touch phones. This now includes the route and collection
title links and the place website link; a hit-test grid on them found no wrong
hits and no overlapping controls. Visible labels are part of the accessible
names. The place list buttons now take their name from the visible name,
category and address. Map markers have no visible text and keep their label.

These checks do not establish physical iOS/Safari behaviour or real basemap
labels, because the map stub draws none. The redesign was released on 19
November 2026 with the guarded schema 14 rollout described in
[Operations](operations.md#ride-cover-schema-upgrade).

## City photo fallback · 20 November 2026

After the release, production had no uploaded photos, so every card fell back
to the city name and the new design read as a recolour. Issue
[#48](https://github.com/unstppbl/bikers/issues/48) adds a middle step: a ride
without a visible cover shows a photo of its catalogue city, and only a custom
city, a city without a photo or an image that fails to load shows the city
name. The photo is chosen from the ride ID, so every viewer sees the same one.
Cards, the ride page and the cover editor use the same order, and a cover that
fails to load moves on to the city photo. The photo sits in the cover slot, so
the scrim, the cancelled-ride grey and the hidden city type apply unchanged.
The ride page describes it as a photo of the city, never of the ride.

The owner chose to show no credits or sources anywhere. Only Wikimedia Commons
files under CC0 were therefore used, re-checked against the Commons API
(licence CC0 or public domain, no attribution required, no restrictions). Kazakhstan
has no freedom of panorama, so photos whose main subject is a recent building
or monument were rejected. The two Astana photos are general skyline views
in which modern buildings are unavoidable. Eight photos cover five cities: three
for Almaty (Zenkov Cathedral, Big Almaty Lake and the city from Kök-Töbe), two
for Astana, and one each for Aktau, Kokshetau and Petropavl. Commons had no
usable free photo of the other 15 cities, including Karaganda and Shymkent: the
candidates were archival scans, maps, portraits, night shots or recent
landmarks. They keep the city name.

The files are WebP, 1600px wide (2600px for the Kokshetau panorama), without
metadata, 97 to 242 KB each, named with a content hash, served from
`/city-photos/` and kept out of the PWA precache. `apps/web/city-photos.json`,
which is not served, records each file's Commons page, licence, source hash
and review date, and a test checks the served files against it.

Validation: `make check-web` passes TypeScript, ESLint, 448 tests in 33 files
and the production/PWA build; the precache still holds 52 entries. Browser
review against the mocked API with every uploaded cover removed, as in
production, captured the feed, a filtered feed, My rides, four ride pages, a
cancelled ride, the cover editor and a club page at 390, 768 and 1440px in
English, Russian and Kazakh, and the same views with covers at 390 and 1440px
in English. The 110 screenshots show no page or console errors, no failed
requests, no broken images and no horizontal overflow. Titles and metadata
over each photo were checked by eye on the existing scrim.

## Adaptive Telemetry & Daylight Redesign · 27 November 2026

This whole-app redesign replaces the warm-asphalt dark-only theme and condensed
display typography with an adaptive visual system (**Telemetry Dark** `#0b0e13`
and **Daylight Roadbook** `#f4f6f9`) that follows `prefers-color-scheme` by
default and offers an explicit theme toggle (`Auto` / `Daylight` / `Dark`) in
the top bar and Settings. Typography moves to self-hosted `Inter` (`--font-body`,
`--font-display`) paired with `JetBrains Mono` (`--font-mono`) for tabular dates,
distances, capacities, and coordinates across English, Russian, and Kazakh.

Key information-architecture and UX changes:
- **Four-pillar navigation:** `Rides` (`/` + `/my-rides`), `Explore` (`/places`
  and `/routes` with an always-visible view switcher), `Community` (`/clubs` and
  `/bikes` unified under a shared tab switcher), and `You` (`/me`, `/garage`,
  `/inbox`, `/settings`, `/profile`, `/admin`).
- **Scannable Ride Tickets & public fit fields:** Backend migration 16 adds
  optional `style`, `surface`, and `distance_km` columns to `rides`, plus
  `public_details_published` defaulting to false. Organizers explicitly publish
  expanded fit metadata (`title`, coarse `time_window`, `style`, `surface`,
  5 km-rounded `distance_km`, `capacity`, `approved_count`, `organizer_name`)
  onto `RideSummary`; existing titles and identity stay private until that
  publication. Calculated distance stays distinct from a manual override, so
  route edits update the estimate. Exact departure time (`starts_at`), meeting coordinates,
  destination, route link, expectations, roster, announcements, chat, and map
  geometry remain strictly private until organizer approval.
- **Instant discovery filters:** Ride discovery adds one-tap quick-filter chips
  for date shortcuts (Today / Weekend), ride style, and surface alongside city
  and keyword search (which searches both `title` and `description`).
- **Tabbed Ride Room & 1-tap approvals:** Approved riders get a compact hero
  with one-tap map navigation and a 4-tab Ride Room (`Plan | Chat | Riders |
  Album`) on mobile/tablet while desktop displays Plan and Chat side by side
  without disconnecting WebSocket updates. Pending join requests expose direct
  1-tap `Approve` / `Decline` buttons instead of hiding them inside a row menu.
- **Structured Ride Creation & Rider Identity Hub:** The ride creation form is
  grouped into Ride basics, Meetup & route (private until approved), and
  Settings, with a live Guest vs. Approved privacy preview. The `You` hub leads
  with the rider's identity and primary garage bike.
