# MVP delivery checklist

The agreed MVP scope is in [Proposed MVP and Roadmap](mvp-and-roadmap.md). This checklist tracks implementation and verification for the full-scope request on 3 November 2026. Numeric limits and policies below are implementation defaults, not additional confirmed product decisions. Deferred catalogue features remain deferred.

## Existing verified foundation

- [x] Telegram-only OTP, durable limits, cookie sessions, CSRF, profile onboarding.
- [x] Public ride previews, organizer approval, participant-only plans and persistent chat.
- [x] Multiple private-by-default bikes, private maintenance, model aliases and verified model-year reference images.
- [x] Bike uploads with catalogue-default covers; safe shared image processing.
- [x] Owner-approved clubs, ownership transfer, closure, public ride associations, club images.
- [x] English, Russian, and Kazakh foundation; PWA shell with no private API/media cache.

## Completed feature delivery

- [x] Profile avatar upload, replacement, removal, initials fallback, and context-authorized display.
- [x] Participant-only ride photo albums, scoped deletion, external video links, and access revocation.
- [x] Separate organizer announcements, with durable ordering and idempotent publication.
- [x] Durable inbox for significant ride changes, cancellation, announcements, and participation decisions.
- [x] Opt-in Web Push, subscription removal, retry/deduplication, generic lock-screen text, install guidance, and denied/unsupported states.
- [x] Reporting with accessible confirmation, bounded submission, and an administrator queue showing the reported evidence.
- [x] Blocking with a documented shared-room policy, including history/media filtering and new join/approval restrictions.
- [x] Administrator roles, audited report actions/content hiding/account restrictions, and operator bootstrap command.
- [x] Manually assigned community badges, grant/revoke audit, and explicit meaning separate from OTP.
- [x] Independent country, language, and currency settings; manual city selection and metric units.
- [x] Core feature strings and errors in all three languages, safe sharing/navigation/video links, accessible phone layouts.

## Verified local delivery

- [x] Full PostgreSQL-backed race suite, architecture check, vet, frontend tests/typecheck/lint/build.
- [x] Permission matrix across HTTP, content, history, previews, and already-open events after removal/block/restriction.
- [x] Migration upgrade/replay with immutable prior migrations and retained accounts/sessions/media.
- [x] Container readiness, non-root execution, one app port/private database, media excluded from PWA cache.
- [x] Backup/restore rehearsal including all media, notification state, and audited admin state.
- [x] Browser journeys using disposable test accounts; EN/RU/Kazakh and narrow mobile views.
- [x] Apply verified migrations/image to the backed-up loopback local app; preserve the existing account/session.
- [x] Update native Space pages and repository mirrors; preserve all 32 catalogue proposals.
- [x] Operational handoff: Compose, backup/restore/rollback, Web Push configuration, proxy requirements, monitoring and release limits.

Homelab/VPS public deployment and real-device push permission/delivery are operational acceptance checks. The infrastructure contract assigns public domains/TLS to the owner. Software verification must not be presented as proof of those checks.

## Initial MVP delivery evidence

On 3 November 2026 the full PostgreSQL-backed Go race suite, vet and architecture checks passed. All 88 frontend tests, typecheck, lint and production/container builds passed. Browser journeys covered two-account approval/inbox, avatars, announcements, albums/video links, reporting/evidence/hiding, manual badges, blocking/unblocking with retained coordination, independent preferences, PWA updates, three languages and a 390-pixel layout. The final block-cache regression was reproduced in the browser, fixed and verified.

A disconnected PostgreSQL 18.6 upgrade/replay and second backup/restore preserved all 28 tables, two sequences, all four image hashes, selections, notification read state, push leases and moderation audit relationships. The initial MVP image `bbe5fb969b47ca7859bd09210b2d6cea5c3197af4fafa33912fb94a1626ee89b` was verified healthy at `http://localhost:33043`, unprivileged/read-only, with receipts 1–7 and only a loopback app port. A fresh private pre-upgrade dump and rollback image are retained; every original column/row in all 14 original data tables matched after migration. The existing account/session and authenticated browser session were retained. No real OTP or push was sent at that checkpoint.

Native Space updates were read back with unchanged unrelated blocks and all 32 catalogue features intact. Repository mirrors match the saved native content. Administrator bootstrap and VAPID setup are operator steps in [Operations](operations.md); no live role was silently granted and no push permission was requested. Public homelab/VPS deployment, real-device push, policy acceptance, off-host recovery and human translation review remain separate pilot acceptance gates.

## Subsequent owner activation

Owner-authorized activation granted the existing account administrator access, generated durable VAPID credentials and deployed the restored database/app to Jarvis port 3050. All 28 tables and two sequences matched the frozen source. The owner then explicitly authorized the dedicated VPS route and certificate: [https://bikers.kz](https://bikers.kz) is live, HTTP redirects to HTTPS, asset hashes match, private APIs and Origin checks pass, and scoped certificate-renewal testing passed. The owner signed in, administrator navigation was verified, and one browser subscription was enabled with permission. One diagnostic push was accepted by the provider, and the owner confirmed that it appeared on this Mac and clicking it opened the inbox. The original full-MVP image evidence above is historical; current release tags and receipts are in [Operations](operations.md#current-deployment).

## Motorcycle visual refresh

- [x] Apply the owner-approved Night Ride direction across the shared shell and feature screens, using semantic colors and locally bundled condensed type.
- [x] Replace the abstract browse artwork with a credited motorcycle reference and retain real ride, model-year and upload behavior.
- [x] Verify desktop, 390-pixel English/Russian/Kazakh layouts, 320-pixel Kazakh headline wrapping and mobile navigation.
- [x] Pass 88 frontend tests, TypeScript, ESLint and both platform builds; exclude development sample rides and preview controls from production.
- [x] Deploy app-only updates locally and on Jarvis; verify identical backend executables, retained database/configuration and all 46 public artifacts over HTTPS.
- [x] Include the font in the PWA precache and ship its licence with the app.

## Approved design and skills polish

- [x] Retain the owner-approved Night Ride palette, condensed type and motorcycle imagery.
- [x] Tighten sidebar spacing; retain 44px navigation and header targets.
- [x] Add modal drawer focus, Escape, scroll locking and background isolation; preserve route/history and anchor behavior.
- [x] Share motion tokens and suppress movement for reduced motion.
- [x] Create a labelled, synthetic Stageshow walkthrough in English, Russian and Kazakh, with manual playback and a static poster.
- [x] Keep the walkthrough development-only and export three 13-second clips with provenance and a rehearsal record.
- [x] Pass 90 frontend tests, TypeScript, ESLint, production build and vendor hash verification; check desktop, 390px and compact 320px layouts.

The [design review](design-review.md) records credits, intentional style choices,
actual-browser checks and the automated motion-sampling limitation. Deployment
receipts and retained-state checks are in [Operations](operations.md#current-deployment).


## My rides and city discovery — 4 November 2026

- [x] Separate Organizing, Joined, Pending and Past views, enforced by the backend.
- [x] Server-side city aliases, inclusive local-date range and public-description filters before pagination.
- [x] Twenty-row pages with authenticated date/UUID cursors, query/caller binding and restart recovery.
- [x] Shared EN/RU/KK Kazakhstan city suggestions and manual-entry fallback in discovery, profiles, rides and clubs.
- [x] URL filters/pages, Back/Forward restoration, actor-bound query cancellation and page reset.
- [x] Paginate the club owner's ride-link chooser without submitting the link form.
- [x] PostgreSQL-backed view/permission/date/cursor regressions and frontend navigation/race tests.

No schema migration is needed. Verification uses disposable local accounts and
a fake authentication provider; it does not send real OTPs or push notifications.


## Club membership notification follow-up — 4 November 2026

The requested follow-up adds owner join-request alerts and applicant
approval/rejection/removal alerts through the existing inbox and opt-in push.
Membership and notification changes are atomic, and unchanged retries do not
create duplicate alerts. Club inbox links use the existing access rules.
Migration 8 is explicit and is verified only in disposable local databases;
production rollout is a separate operation.

Verified: 107 frontend tests, typecheck/lint/build, full PostgreSQL-backed Go
race suite, vet and architecture checks. Schema-7 upgrade/replay preserves old
inbox/read state, session, club, subscription/job lease and prior receipts.
The isolated browser flow verifies owner request → club queue → approval →
applicant inbox/member view, read-all state and EN/RU/KK phone layouts. Real OTPs,
production push deliveries and production schema changes are outside this check.
