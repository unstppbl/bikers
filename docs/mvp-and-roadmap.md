# Proposed MVP & Roadmap

Native Space page: [Open in Space](https://chatgpt.com/space/page_1a9627318dd08191b58140311e0e8f41)  
Space snapshot: 3 November 2026

[Project Home](project-home.md)

**Version:** 0.1  
**Captured:** 1 November 2026  
**Updated:** 3 November 2026  
**Status:** All agreed MVP feature slices are implemented for local review, including avatars, ride albums, announcements, inbox/optional push, moderation/manual badges and independent locale settings. Pilot operations, real-device push, policy acceptance and human translation review remain release gates. Detailed defaults remain reviewable.

The BIKERS.KZ name, Kazakhstan-first launch, English/Russian/Kazakh support, and all-in-one motorcycle vision are established. The first pass through all 32 catalogue features is complete. The launch table records agreed MVP slices; the notes below record deferrals. Delivery sequencing, dates, estimates, detailed behavior, and the proposed flagship positioning remain open.

## Platform and deployment

**Confirmed — 3 November 2026:** Launch as a mobile-first web/PWA to test and iterate quickly. Mobile apps follow in a later phase; their framework, exact platform coverage, and timing remain open.

**Planned hosting:** The homelab Mac mini hosts the application’s Docker Compose stack. The public VPS terminates HTTPS and reverse-proxies over WireGuard to one plain-HTTP app port. Datastore ports remain internal, data persists in volumes, and deployment configuration comes from environment variables. The actual host OS, CPU architecture, Docker runtime, and available capacity still need verification.

**Confirmed stack — 3 November 2026:** React/TypeScript/Vite for the mobile-first PWA, a Go application server serving the built frontend, API, and chat connections, and PostgreSQL. Business rules and permissions live in the backend, with an API reusable by later mobile clients. Docker Compose publishes one application port and keeps PostgreSQL internal, with persistent database and photo storage. Session/recovery design, package/version choices, and detailed implementation remain open.

**Signup and login — confirmed 3 November 2026:** Telegram Gateway OTP is the only MVP user signup/login method, adapting the existing Go client. The user enters a phone number, receives a code through Telegram, and enters it in BIKERS.KZ; the backend validates the bound challenge before creating or signing in to the account. Google, email, password, and SMS login/fallback channels are outside the MVP. The phone number must be registered with Telegram. Session design, number changes, and account recovery remain open. Phone verification does not grant the admin-assigned profile badge.

**Reuse requirements:** Parse and validate the verification result, bind the challenge to the server-stored original phone number, and enforce expiry, single use, and attempt/resend limits. The source client is a starting point for adaptation, not a complete account/session implementation. [Telegram Gateway API](https://core.telegram.org/gateway/api)

## Initial hypothesis

A useful ride-organizing experience can bring a real community into BIKERS.KZ. Clubs give organizers reasons to invite members; a basic garage provides personal utility before there is a large network.

**Proposed flagship:** Ride Rooms.

**First complete journey:** Discover a ride → understand the plan → join → coordinate → meet → share → return for another ride.

## Launch scope and status

| Component and status | Initial scope | Catalogue references |
| --- | --- | --- |
| Profiles and garage — agreed limited scope | Profile: name, avatar, city, languages, riding interests; motorcycle ownership optional. Multiple bikes: make, model, year, photos. Private maintenance log: date, mileage, work performed. | GARAGE-01–03 |
| Planned rides — agreed limited scope | Title, organizer, meeting place/time, destination/route link, riding expectations, capacity, participation rules, join/leave, cancellation, clear plan updates | RIDE-02 |
| Ride Rooms — agreed limited scope | Current plan, participants, organizer announcements, lightweight text chat, notifications for significant changes/cancellation | RIDE-03 |
| Clubs — agreed limited scope | Club page: name, image, description, city, member list, one owner/admin, linked rides | COMMUNITY-01 |
| Ride media — agreed limited scope | Small ride-attached photo albums and links to external videos; written ride recaps deferred | RIDE-04 in part; MEDIA-02 |
| Invitations — agreed limited scope | Shareable ride/club links and useful previews respecting visibility settings | COMMUNITY-04 |
| Navigation handoff — agreed limited scope | Open meeting point/destination in an external navigation app; support an organizer-supplied route link | EXPLORE-02 |
| Verification badges — agreed limited scope | Admin manually assigns verification badges; meaning, recipient types, criteria, and supporting checks remain to be defined | TRUST-03 in part |
| Privacy and moderation — agreed limited scope | Content visibility controls; private maintenance/sensitive ownership records; reporting, blocking, simple admin review queue | TRUST-01–03 in part |
| Three-language experience — agreed scope | English, Russian, Kazakh across onboarding, core screens/actions, notifications, errors | LOCAL-01 |
| Model lookup — agreed limited scope | Common motorcycle-model aliases and relevant Cyrillic/Latin spellings; advanced cross-language search and automatic content translation deferred | LOCAL-02 |
| Locale settings — agreed scope | Metric units; KZT where applicable; independent country/language/currency; manual city selection without location permission | LOCAL-03 |
| Off-season usefulness — covered by agreed garage | Garage and private maintenance log; no separate off-season feature package | LOCAL-04 |

**Agreed garage deferrals — 2 November 2026:** Rich profile customization, detailed modifications, ownership stories, previously owned bikes, receipts, expense tracking, maintenance reminders, analytics, and model-based discovery (GARAGE-04) are outside the MVP. Maintenance records stay private and separate from the bike’s visible profile.

**Agreed community deferrals — 2 November 2026:** Contextual feeds (COMMUNITY-02), general structured posts (COMMUNITY-03), advanced club roles, club chat, recurring rides, calendars, and richer invitation tools are outside the MVP. Club membership uses owner approval: riders request to join, the owner approves/removes members, and members may leave. The owner must transfer ownership or close the club before leaving. Detailed visibility defaults and club-only ride behavior remain reviewable.

**Agreed Explore deferrals — 2 November 2026:** Community route library, discovery, route photos/stops, recorded tracks (EXPLORE-01), useful-places map/directory (EXPLORE-03), and condition reports (EXPLORE-04) are outside the MVP. Saved route collections, native navigation, offline maps, and advanced routing are also deferred.

**Agreed commerce deferrals — 2 November 2026:** All COMMERCE-01–03 capabilities are outside the MVP: service-provider directory/inquiries, booking, quotes, deposits, payments, marketplace listings and model-based listing discovery, and selective maintenance-history sharing for bike sales. A simple directory may be revisited separately from inquiries; no post-MVP sequence is committed.

**Agreed media deferrals — 2 November 2026:** Standalone events beyond rides (MEDIA-01), written ride recaps, general media posts, native video uploads, creator profiles, subscriptions, monetization, livestreams/live chat/replays (MEDIA-03), and on-bike streaming implementation are outside the MVP. MEDIA-04 safeguards remain requirements for future on-bike streaming. Only the photo-album portion of RIDE-04 and photos/external-video links in MEDIA-02 are agreed for MVP.

**Agreed verification scope — 2 November 2026:** Manually admin-assigned verification badges are in the MVP. The badge’s meaning, recipient types, criteria, and supporting checks remain open. Basic privacy and moderation are also agreed; the Trust deferrals below apply.

**Agreed Trust deferrals — 2 November 2026:** Live-location sharing/tracking and tracking controls, sensitive-location hiding for recorded tracks, assistance requests, and reputation/reward/recognition systems are outside the MVP. These incentive-system deferrals do not exclude manually admin-assigned verification badges.

**Agreed localization limits — 2 November 2026:** Advanced cross-language content search and automatic user-content translation are deferred. KZT defaults apply where relevant and do not add commerce, payment, or expense-tracking features. Off-season usefulness comes from the agreed garage and maintenance log.

**Agreed ride scope — 2 November 2026:** Limited planned rides (RIDE-02) and lightweight Ride Rooms (RIDE-03) are included. Expiring availability statuses and spontaneous invitations (RIDE-01) are deferred. RIDE-04 includes only the already-agreed photo album; written recaps remain deferred. **Access policy agreed — 3 November 2026:** The organizer approves join requests. Public previews show city, date, and a short description; exact meeting details, participant list, chat, and photos require approved-participant access. Other participation behavior and notification details remain open.

The table fixes the agreed feature slices. Organizer approval and ride-preview/participant access are confirmed in D-036. Required/optional fields, validation, club membership rules, other visibility settings, and remaining behavior still require design. Full club discussions, contextual feeds, and advanced maintenance analytics are outside the MVP.

## Requested ride-discovery follow-up — 4 November 2026

D-043 adds My rides (Organizing, Joined, Pending and Past), server-side city/date
filters and pagination, plus a shared city selector. The implementation also
keeps description search on the server. Organizing/Joined/Pending are upcoming
published rides; Past includes earlier or cancelled organized/approved/requested
rides and is not an attendance record. The selector suggests localized
Kazakhstan cities and allows manual entry elsewhere, without location access.
See the [API contract](development.md#ride-lists-and-cities) for exact defaults.
This implementation follow-up does not change the separate deferred features.

## Early follow-on candidates

These are not ordered commitments:

- Expiring “Who’s riding?” availability is deferred beyond the MVP; its later priority is undecided.

- Optional live group locations and regrouping points.

- Richer model-based discussions and local discovery.

- More capable maintenance reminders and ownership records.

- Community routes, useful places, and service-directory inquiries are deferred beyond the MVP.

Live GPS tracking is deferred beyond the MVP. It was proposed as the next major capability after the core ride journey; its post-MVP timing remains undecided. Background behavior, battery use, permissions, freshness indicators, audience controls, and expiry require explicit testing before any future release.

## Later-stage candidates

Develop based on usage and research rather than a fixed calendar:

- Structured marketplace and model-aware listing discovery.

- Service bookings, quotes, deposits, and payment workflows.

- Native video, creator tools, livestreaming, moderated live chat, and replays.

- Native navigation, offline maps, and advanced routing.

- Selective maintenance-history sharing for bike sales.

- Practical community-assistance requests.

## Explicitly not initial requirements

Escrow, shipping operations, financing, a full commerce checkout system, complex recommendation algorithms, full native navigation, and streaming infrastructure are not proposed prerequisites for the first public release.

This is proposed sequencing, not a rejection of those ideas.

## Proposed validation metrics

The original recommendation was to emphasize repeat participation over downloads or passive feed viewing.

| Question | Candidate measurement |
| --- | --- |
| Do riders receive real value? | Riders who participate in a ride and later return for another |
| Do organizers keep using BIKERS.KZ? | Repeat ride creation and completed outings per organizer |
| Does a plan lead to participation? | Discovery-to-join and join-to-attendance progression |
| Does personal utility help retention? | Repeat garage use and maintenance-record activity |
| Does the community remain usable? | Reporting and moderation outcomes, alongside user feedback |

The last three measurements are planning elaborations, not previously approved success criteria. Attendance confirmation, privacy-preserving analytics, metric definitions, observation windows, and numerical targets remain undecided. A tap on “Join” should not be treated as proof of attendance.

## Remaining decisions and release gates

The [Draft Technical Design and Build Plan](technical-design-and-build-plan.md) proposes implementation defaults and a six-phase sequence, starting with foundation and the complete internal ride journey. These recommendations are not additional confirmed product decisions or delivery-date commitments.

Choose the first launch community/city and primary audience, assign moderation/translation ownership, and accept account recovery/deletion/retention and badge criteria before a public pilot. Implemented defaults cover capacity including the organizer, owner-approved clubs, bilateral shared-room blocking, bounded albums, durable inbox/optional generic push and manual Community verified user badges. They are reviewable implementation choices, not newly confirmed product decisions. Organizer approval/ride access and owner transfer-or-close rules remain confirmed. Verify homelab/VPS TLS, proxy behavior, real-device push, capacity, off-host backups and recovery targets. Later mobile timing/framework, pricing, monetization and success measures remain open.

Off-season MVP usefulness is provided by the agreed garage and private maintenance log (LOCAL-04). No separate off-season feature package is included.
