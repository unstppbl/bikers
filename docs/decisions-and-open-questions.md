# Decisions & Open Questions

Native Space page: [Open in Space](https://chatgpt.com/space/page_5150f0f0e93c8191b6e26726a15379c1)  
Space snapshot: 4 November 2026

[Project Home](project-home.md)

**Captured:** 1 November 2026  
**Updated:** 4 November 2026  
**Status:** Decision register with confirmed direction, proposals awaiting agreement, and open questions.

BIKERS.KZ’s name, Kazakhstan-first launch, English/Russian/Kazakh support, and all-in-one motorcycle vision are established. The first MVP feature-scope pass is complete, with agreed slices and deferrals recorded in D-010–030. Web/PWA first, mobile apps later, and the homelab-plus-VPS deployment target are confirmed in D-031–032. React/TypeScript/Vite, Go, and PostgreSQL are confirmed in D-033. Telegram OTP-only user signup/login is confirmed. Session, recovery, number-change policies, detailed implementation/tooling, delivery sequencing, proposed flagship positioning, and unresolved launch/design decisions remain open.

## Status meanings

- **Confirmed:** Explicitly established by Altay.

- **Proposed:** Suggested direction that still needs a decision.

- **Open:** Not yet resolved.

- **Deferred:** Outside the MVP when explicitly confirmed in a decision; otherwise a timing recommendation awaiting agreement.

## Confirmed decisions

| ID | Decision | Basis |
| --- | --- | --- |
| D-001 | Use BIKERS.KZ as the project name | Altay’s explicit instruction on 1 November 2026 to use BIKERS.KZ throughout these pages; supersedes the earlier working-name decision |
| D-002 | Begin with users in Kazakhstan | Altay’s original project brief |
| D-003 | Support English, Russian, and Kazakh | Altay’s original project brief |
| D-004 | Build toward an all-in-one motorcycle platform | Altay clarified “super-app, all in one” |
| D-005 | Include future streaming in the vision | Altay explicitly mentioned streaming; timing and implementation remain undecided |
| D-006 | Keep international expansion in view | Altay’s naming brief; timing and implementation remain undecided |
| D-007 | Prefer a name that is not too abstract | Altay’s naming feedback |
| D-008 | Document the proposed features before further detailed discussion | Altay liked the feature list and requested documentation |
| D-009 | Keep project knowledge in a dedicated ChatGPT Space using native pages | Altay’s original request for a dedicated knowledge location, followed by the explicit request for Project Home and native subpages |
| D-010 | Include GARAGE-01 in the MVP: name, avatar, city, languages, riding interests; motorcycle ownership optional | Altay’s approval on 2 November 2026; rich profile customization deferred |
| D-011 | Include GARAGE-02 in the MVP: multiple bikes with make, model, year, photos | Altay’s approval on 2 November 2026; detailed modifications, ownership stories, and previously owned bikes deferred |
| D-012 | Include GARAGE-03 in the MVP: private maintenance log with date, mileage, work performed, separate from the bike’s visible profile | Altay’s approval on 2 November 2026; receipts, expense tracking, reminders, and analytics deferred |
| D-013 | Defer GARAGE-04 model-based discovery beyond the MVP | Altay’s approval on 2 November 2026; retain the full catalogue proposal |
| D-014 | Include limited COMMUNITY-01 club pages: name, image, description, city, member list, one owner/admin, linked rides | Altay’s approval on 2 November 2026; advanced roles, club chat, recurring rides, calendars deferred |
| D-015 | Defer COMMUNITY-02 contextual feeds and COMMUNITY-03 general structured posts beyond the MVP | Altay’s approval on 2 November 2026; retain the full catalogue proposals |
| D-016 | Include limited COMMUNITY-04: shareable ride/club links with useful previews respecting visibility settings | Altay’s approval on 2 November 2026; richer invitation tools deferred |
| D-017 | Include limited EXPLORE-02: open a meeting point/destination in an external navigation app and support an organizer-supplied route link | Altay’s approval on 2 November 2026; saved route collections, native navigation, offline maps, and advanced routing deferred |
| D-018 | Defer EXPLORE-01 community route library/discovery/photos/stops/recorded tracks, EXPLORE-03 useful-places map/directory, and EXPLORE-04 condition reports beyond the MVP | Altay’s approval on 2 November 2026; retain the full catalogue proposals |
| D-019 | Defer all COMMERCE-01–03 capabilities beyond the MVP: provider directory/inquiries, bookings/quotes/deposits/payments, marketplace/model-based listing discovery, and selective service-history sharing for sales | Altay’s approval on 2 November 2026; later commerce sequence remains undecided |
| D-020 | Include small ride-attached photo albums and external video links in the MVP (MEDIA-02; photo-album portion of RIDE-04 only) | Altay’s approval on 2 November 2026; written ride recaps deferred; limited planned rides and Ride Rooms agreed separately in D-029 |
| D-021 | Defer standalone events beyond rides, written recaps, native video, creator profiles/subscriptions/monetization, Live/live chat/replays, and on-bike streaming implementation | Altay’s approval on 2 November 2026; general media posts also remain deferred; MEDIA-04 safeguards retained as requirements for future on-bike streaming |
| D-022 | Include verification badges in the MVP, manually assigned by an admin | Altay’s explicit instruction on 2 November 2026; badge meaning, recipient types, criteria, and supporting checks remain open |
| D-023 | Include content visibility controls, private maintenance/sensitive ownership records, reporting, blocking, and a simple administrator review queue in the MVP | Altay’s approval on 2 November 2026; limited TRUST-01–03 scope, alongside admin-assigned badges in D-022 |
| D-024 | Defer live-location sharing/tracking and tracking controls, tools for hiding locations in recorded tracks, assistance requests, and reputation/reward/recognition systems | Altay’s approval on 2 November 2026; admin-assigned verification badges remain included |
| D-025 | Apply English, Russian, and Kazakh to MVP onboarding, core screens/actions, notifications, and errors | Altay’s approval on 2 November 2026; implementation depth extends the established language commitment D-003 |
| D-026 | Include basic motorcycle-model lookup for common aliases and relevant Cyrillic/Latin spellings; defer advanced cross-language content search and automatic user-content translation | Altay’s approval on 2 November 2026; limited LOCAL-02 scope |
| D-027 | Use metric units and KZT where applicable, keep country/language/currency independent, and allow manual city selection without location permission | Altay’s approval on 2 November 2026; LOCAL-03 adds no payment or expense-tracking scope |
| D-028 | Deliver off-season MVP value through the approved garage and private maintenance log, with no separate feature package | Altay’s approval on 2 November 2026; limited LOCAL-04 scope |
| D-029 | Include limited RIDE-02 planned rides and RIDE-03 Ride Rooms: ride details/organizer/meeting place-time/destination or route link/expectations/capacity/participation rules/join-leave/cancellation/updates; current plan, participants, organizer announcements, lightweight text chat, significant-change/cancellation notifications | Altay’s approval on 2 November 2026; organizer approval and preview access confirmed in D-036; other participation details open; live tracking deferred |
| D-030 | Defer RIDE-01 expiring availability statuses and spontaneous invitations beyond the MVP | Altay’s approval on 2 November 2026; RIDE-04 photo albums remain included and written recaps remain deferred |
| D-031 | Launch web/PWA first for fast testing and iteration; build mobile apps in a later phase | Altay’s explicit preference on 3 November 2026; mobile framework, exact platform coverage, and timing remain open |
| D-032 | Use the homelab Mac mini behind the public VPS as the planned deployment target, following the Docker Compose/one-HTTP-port contract | Altay’s deployment plan and homelab-infra.md on 3 November 2026; VPS handles public HTTPS and reverse-proxies over WireGuard; host/runtime/capacity verification remains open |
| D-033 | Use React/TypeScript/Vite for the mobile-first PWA, Go for the application server serving frontend/API/chat, and PostgreSQL; keep business rules/permissions in a backend API reusable by later mobile clients | Altay’s approval on 3 November 2026; one app port and internal database follow D-032; session design, package/version choices, and implementation details remain open |
| D-034 | Use Telegram Gateway OTP for phone verification, adapting the existing Go client | Altay’s instruction on 3 November 2026; Telegram-only signup/login confirmed in D-035; recovery and session details remain open; profile badges remain admin-assigned |
| D-035 | Use Telegram Gateway OTP as the only MVP user signup/login method; defer Google, email, password, and SMS login/fallback channels | Altay’s explicit choice on 3 November 2026; users need a Telegram-registered number; number-change, recovery, and session policy remain open |
| D-036 | Require organizer approval for ride join requests. Public previews show city/date/short description; exact meeting details, participant list, chat, and photos are restricted to approved participants | Altay’s approval on 3 November 2026; club-specific eligibility, capacity accounting, and other policies remain proposed |
| D-037 | Keep project documentation both as native Space pages and Markdown under the project-root docs directory | Altay’s explicit instruction on 3 November 2026; synchronization is explicit, with no automatic sync configured |
| D-038 | Follow the backend package ownership and dependency rules in repository AGENTS.md, enforced by an automated architecture check | Altay’s explicit instruction on 3 November 2026 to align the implementation with AGENTS.md |
| D-039 | Use owner-approved club membership: riders request to join; the owner approves and removes members; members can leave; the owner must transfer ownership or close the club before leaving | Altay’s selection of owner approval on 3 November 2026 |
| D-040 | Automatically show a motorcycle model reference picture only when its model and year/generation are verified to match the selected bike; show no photo when a match is unavailable | Altay’s clarification and explicit year/generation matching choice on 3 November 2026 |
| D-041 | Add user-uploaded bike photos while keeping the catalogue image as the default cover | Altay’s explicit implementation request on 3 November 2026; owners can deliberately select an uploaded cover and restore the catalogue default |
| D-042 | Commit all changes and push to the repository, then build/deploy production on Jarvis from that Git checkout; tag images with the exact commit and retain Compose project `bikerskz`. Production images must be built on Jarvis, without local image builds/copies | Altay’s explicit deployment-process instruction on 4 November 2026; repository `unstppbl/bikers`, branch `main`; first checkout-based cutover planned |

D-008 is approval to preserve and discuss the catalogue. It does not establish that every feature, suggested phase, safeguard implementation, or metric is approved for delivery.

## Proposal status

| ID | Proposal | Status |
| --- | --- | --- |
| P-001 | Make ride organization and Ride Rooms the initial flagship | RIDE-02–03 inclusion agreed in D-029; flagship positioning and delivery priority remain proposed |
| P-002 | Pair the initial ride experience with clubs and a simple garage | Limited ride, club, and garage scope included in D-010–014 and D-029; adoption strategy remains a hypothesis, and delivery order is open |
| P-003 | Add live GPS after the core coordination flow | Live GPS deferred beyond MVP in D-024; exact post-MVP timing remains proposed |
| P-004 | Start services with directory/inquiries rather than full booking/payments | Services deferred beyond MVP in D-019; sequencing within later commerce work remains proposed |
| P-005 | Start media with photos and external video links before native video/live | Initial photo albums/external video links agreed in D-020; richer media deferred in D-021; later sequencing remains open |
| P-006 | Prioritize repeat riding and organizer retention over downloads | Proposed measurement direction |
| P-007 | Use “Your people. Your bike. Your riding life.” as positioning | Working copy only |
| P-008 | Preserve public/private boundaries and moderation from the first release | Limited privacy/moderation scope agreed in D-023; admin-assigned badges agreed in D-022 |
| P-009 | Use React/TypeScript/Vite for the PWA, a Go application server, and PostgreSQL, with a backend API reusable by later mobile clients | Confirmed in D-033 on 3 November 2026 |
| P-010 | Adopt the module layout, schema/API, session/permission policies, and six-phase build order in the [Draft Technical Design and Build Plan](technical-design-and-build-plan.md) | Draft except ride approval/preview access in D-036, backend package ownership in D-038, club membership rules in D-039, and model-picture matching in D-040; remaining design defaults and build order await review |

## Naming history

The discussion moved from Kazakh-rooted or invented names toward obvious motorcycle-related branding. Alternatives included BIKERZ, BAIKZ, BIKEZ, MOTORCOM, and MOTOGARAGE, alongside the earlier compact bike-related working name. Altay’s current instruction establishes **BIKERS.KZ** as the name used throughout these pages.

Earlier naming discussion considered a possible KZ/Kazakhstan association. Whether that association should be explicit, and the intended pronunciation, remain open. Visual identity and approved brand copy also remain unresolved.

Earlier brand-conflict comments are not trademark clearance. Domains, social handles, app-store naming, and relevant trademark jurisdictions/classes need a fresh verification process before significant brand investment.

## Open questions for later sessions

### Product and launch

What is the first launch city or community? Who is the first primary segment: established clubs, casual city riders, new riders, touring riders, or another group? Which exact problem will drive the first installation and repeated use?

### Rides and clubs

Organizer approval and ride-preview/participant access are confirmed in D-036. What are the remaining cancellation, plan-change, capacity, no-show, and attendance behaviors? How will club membership joining, removal, and visibility work within the agreed basic club scope? Are direct private conversations needed later? Club chat and advanced roles are deferred.

### Profiles garages and discovery

Which profile fields are required? What is public by default? How will motorcycle makes/models be normalized? Which content should be visible without signing in? The minimum maintenance log fields are agreed in D-012.

### Location safety and moderation

What are the retention and deletion policies for MVP content? Who operates the administrator review queue? What do the admin-assigned verification badges mean, which recipient types can receive them, and what criteria and evidence support assignment? For deferred live tracking, who may see location data, for how long, and at what precision? For deferred condition reports, how will route reports be refreshed or expired?

### Commercial model

For deferred commerce work: which marketplace categories come first? How are providers onboarded? How are misleading listings and reviews handled? What is the eventual revenue model, and which services, if any, involve payments inside BIKERS.KZ? The directory and in-app inquiry flow can be scoped separately; their post-MVP order remains undecided.

### Media and streaming

For deferred events and streaming: which creator/event use case justifies native video or live streaming? What are the moderation, privacy, infrastructure, and cost requirements? MEDIA-04 establishes hands-free operation, no required screen/chat interaction while moving, and no automatic exact live-location publication for future on-bike streaming.

### Localization and international growth

Who maintains and reviews the English, Russian, and Kazakh translations? How will common model aliases and Cyrillic/Latin spellings be curated for basic MVP lookup? For later stages, how should cross-language discussions, advanced search, and automatic content translation work? When should a second market be considered, and which assumptions are Kazakhstan-specific?

### Engineering and delivery

How should session expiry/revocation, phone-number changes, and account recovery work with Telegram-only signup/login? What are the session, API, chat transport, photo-processing, backup, analytics, and operational choices within the agreed React/TypeScript/Vite + Go + PostgreSQL stack? Which detailed libraries and versions should be adopted? How will later mobile clients authenticate and which mobile framework/platforms will be targeted? What are the actual homelab OS, CPU architecture, Docker runtime, available storage/capacity, team, budget, timeline, and operational responsibilities?

No technology or schedule should be inferred from Altay’s other projects or general engineering background.

## Knowledge maintenance

Update the relevant page when a decision is made, and add a dated entry below. Mark superseded decisions explicitly rather than silently replacing their rationale. Keep ideas and research findings separate from approved commitments. Preserve useful deferred ideas without silently adding them to the MVP.

Use the catalogue’s stable feature identifiers when discussing scope. Cite external evidence when adding research; claims about product uniqueness, route safety, verification, or name availability need an appropriate basis.

These pages are a saved snapshot of the discussion. Creating them does not automatically move this chat into a ChatGPT Project or archive future conversations. Updates should be made explicitly as the project develops.

## Change log

- **1 November 2026 — v0.1:** Captured the chosen working brand, market/languages, super-app ambition, full proposed feature catalogue, suggested MVP/roadmap, and unresolved questions. Established a dedicated project knowledge location.

- **1 November 2026 — Native pages and naming update:** D-001 supersedes the earlier working-name decision with BIKERS.KZ, following Altay’s explicit instruction. D-009 reflects the requested native Space pages. Original source documents remain intact. MVP scope and sequencing remain proposals.

- **2 November 2026 — Garage MVP scope:** Added D-010–013 following Altay’s approval of limited profiles, multiple basic motorcycle pages, and a private maintenance log. Richer garage features and model-based discovery are deferred. Ride and club scope remain provisional.

- **2 November 2026 — Clubs and community MVP scope:** Added D-014–016 following Altay’s approval of basic club pages and shareable ride/club links with previews respecting visibility. Contextual feeds, general structured posts, advanced club roles, club chat, recurring rides, calendars, and richer invitation tools are deferred. Ride scope remains provisional.

- **2 November 2026 — Explore MVP scope:** Added D-017–018 following Altay’s approval of basic external navigation handoff and organizer route links. Community routes, useful-places maps/directories, condition reports, saved route collections, native navigation, offline maps, and advanced routing are deferred. Ride scope remains provisional.

- **2 November 2026 — Commerce MVP scope:** Added D-019 following Altay’s approval to defer all three commerce features. Service discovery/inquiries, transaction workflows, marketplace listings, and maintenance-history sharing for sales remain in the catalogue for later consideration. No later commerce sequence is committed.

- **2 November 2026 — Media MVP scope:** Added D-020–021 following Altay’s approval of small ride-attached photo albums and external video links. The album portion of RIDE-04 is included; written recaps and richer media/events/streaming are deferred. Future on-bike streaming retains MEDIA-04 safeguards. Ride planning and Ride Rooms remain provisional.

- **2 November 2026 — Verification badges:** Added D-022 following Altay’s instruction to keep admin-assigned verification badges in the MVP. Meaning, recipient types, criteria, and supporting checks remain open. Other Trust scope recommendations remain pending.

- **2 November 2026 — Privacy and moderation MVP scope:** Added D-023–024 following Altay’s approval of visibility controls, private maintenance records, reporting, blocking, and admin review. Live tracking, recorded-track privacy tools, assistance requests, and reputation/reward systems are deferred. Admin-assigned verification badges remain in the MVP under D-022.

- **2 November 2026 — Localization MVP scope:** Added D-025–028 following Altay’s approval of complete three-language MVP flows, basic model aliases/script matching, metric/KZT locale defaults, independent locale settings, manual city selection, and off-season value through the existing garage/log. Advanced cross-language search and automatic content translation are deferred. Ride planning and Ride Rooms remain provisional.

- **2 November 2026 — Ride scope and completed catalogue pass:** Added D-029–030 following Altay’s approval of planned rides and lightweight Ride Rooms, with expiring availability deferred. All 32 features now have an MVP scope or deferral note. The full catalogue remains intact. Delivery order, estimates, dates, flagship positioning, and detailed implementation decisions are still open.

- **3 November 2026 — Platform and hosting:** Added D-031–032: web/PWA first for fast testing/iteration, mobile apps later, and planned homelab deployment behind the public VPS using the existing Compose/WireGuard/reverse-proxy contract. Added P-009 as an unapproved stack proposal. Mobile framework/timing, actual runtime details, and the technology stack remain open.

- **3 November 2026 — Technical stack:** Added D-033 following Altay’s approval of React/TypeScript/Vite, Go, and PostgreSQL, with a reusable backend API for later mobile clients. P-009 is now confirmed. Authentication and detailed implementation/tooling remain open.

- **3 November 2026 — Telegram phone verification:** Added D-034 following Altay’s proposal to reuse the existing Telegram Gateway client. Adaptation must validate the verification result, bind each challenge to its original phone number, and enforce expiry/single-use/attempt limits. Full sign-in, fallback, recovery, and session policy remain open. Telegram OTP does not grant the admin-assigned profile badge.

- **3 November 2026 — Telegram-only signup/login:** Added D-035 following Altay’s explicit choice to use Telegram Gateway OTP as the sole MVP user signup/login method. Other login/fallback channels are deferred. Session lifecycle, number changes, and recovery remain design topics. Phone OTP stays separate from the admin-assigned profile badge.

- **3 November 2026 — Technical design draft:** Created the [Draft Technical Design and Build Plan](technical-design-and-build-plan.md) under Project Home, covering the data model/API, Telegram OTP/session lifecycle, permissions, room reliability, media, localization, homelab deployment, and phased acceptance gates. Added P-010. New defaults and sequencing remain proposed; no application code or live infrastructure was changed by this drafting task.

- **3 November 2026 — Ride access and repository documentation:** Added D-036 for organizer-approved joining and the public-preview/approved-participant visibility boundary. Added D-037 to maintain native Space pages alongside Markdown in the repository docs directory. Other technical defaults and sequencing remain proposals.

- **3 November 2026 — Full MVP implementation:** Implemented the remaining agreed avatars, ride albums/external video links, organizer announcements, durable inbox/optional Web Push, reporting/blocking, administrator review/manual community badges, and independent locale preferences. PostgreSQL-backed race checks, 88 frontend tests and a private schema 1–7 backup/restore rehearsal passed. Shared-room block behavior, Community verified badge wording, numeric limits and push defaults are implementation choices awaiting product/operations acceptance, not new confirmed decisions. At this implementation milestone, homelab/VPS rollout and real-device push were still pending; subsequent verification is recorded below. Account lifecycle/retention and moderation/translation ownership remain release gates.

- **3 November 2026 — Owner access and homelab deployment:** At Altay's request, granted administrator access to his existing account through the audited operator command, configured durable Web Push credentials with https://bikers.kz as the contact, and deployed the restored app to Jarvis port 3050. All 28 tables and two sequences matched the source export; the account/session and admin audit were retained. After the owner's explicit override, installed the dedicated VPS route and domain-only HTTPS certificate. Public health, asset hashes, API privacy boundaries and HTTP-to-HTTPS redirection passed checks without changing unrelated nginx configurations. Browser subscription and actual device delivery were not yet verified during this activation pass; no real OTP or push was sent then.

- **3 November 2026 — Browser subscription and push-provider check:** Confirmed the owner's administrator navigation at [bikers.kz](https://bikers.kz). At the owner's request, allowed notifications in Arc and registered one browser subscription. A single operator-only diagnostic push to that subscription exited successfully and was accepted by the push provider. No retry, synthetic ride, inbox item or delivery job was created. After that provider check, the owner confirmed that the notification appeared on his Mac in Arc and clicking it opened `/inbox`. Visible delivery and click-through are verified for that browser; installed iPhone/Android behavior remains a separate acceptance check.

- **4 November 2026 — Motorcycle visual refresh:** At Altay's request for a stronger motorcycle identity, deployed the Night Ride design with charcoal/lime, condensed typography, motorcycle navigation icons and a credited KTM 390 Adventure 2024 reference hero. At this deployment milestone, Night Ride was a working visual default; the owner's subsequent approval is recorded below. Public browser inspection, 320/390-pixel English/Russian/Kazakh checks and all 88 frontend tests/typecheck/lint/build passed. The backend binary, schema 7 and existing account/session/subscription state were preserved. No new MVP features or confirmed decision IDs were added.

- **4 November 2026 — Design approval and usability polish:** Altay approved the Night Ride baseline and requested [Unslop by Damir Sokolovsky](https://github.com/heydamir/unslop) and [Stageshow by Benji Taylor](https://github.com/benjitaylor/skills/tree/main/stageshow). Deployed focused improvements to 44px controls, shorter-screen administrator navigation, keyboard drawer access, route focus/history behavior and reduced motion. The synthetic ride-request → approval → Ride Room walkthrough remains development-only, with three localized 13-second MP4s and static reduced-motion posters. All 90 frontend tests/typecheck/lint/build and the stated manual browser checks passed; automatic motion timing coverage remained inconclusive because of frame-sampling gaps. The backend, schema 7 and existing account/session/subscription state were retained. This approves the visual baseline and preserves the agreed MVP feature scope.

- **4 November 2026 — Git-based production releases:** Added D-042 following Altay's instruction to commit and push all changes to [unstppbl/bikers](https://github.com/unstppbl/bikers), then clone/update `main` and build/deploy production from that checkout on Jarvis. The planned checkout is `/home/user/apps/bikerskz/repo`, with private `.env` retained in its parent, exact commit-tagged images and Compose project `bikerskz`. Production image builds/copies from the local machine are excluded. At this confirmation, the first checkout-based cutover is still planned.
