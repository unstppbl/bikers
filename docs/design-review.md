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
