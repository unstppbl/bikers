# Feature Catalogue

Native Space page: [Open in Space](https://chatgpt.com/space/page_4314e7d6b2e881919f1f014b7dc1c478)  
Space snapshot: 3 November 2026

[Project Home](project-home.md)

**Version:** 0.1  
**Captured:** 1 November 2026  
**MVP scope updated:** 3 November 2026  
**Status:** All 32 feature proposals and their agreed MVP slices/deferrals are preserved. The agreed MVP slices are implemented for local review; detailed defaults and public-pilot operations remain reviewable.  
**Source:** The feature proposal in the current BIKERS.KZ planning conversation, positively received by Altay and saved for further discussion

## Product premise

BIKERS.KZ should connect a rider’s social life, riding activity, motorcycle ownership, services, purchases, and media. Features should reinforce one another rather than behave like unrelated apps inside a menu.

The BIKERS.KZ name, Kazakhstan-first launch, English/Russian/Kazakh support, and all-in-one motorcycle vision are confirmed. Feature-level MVP notes identify agreed scope and deferrals. All other feature designs, implementation details, and priorities below remain proposals. Phrases such as “early,” “next,” and “later” describe suggested sequencing, not approved delivery commitments. Examples are illustrative, not defaults already chosen for the product.

## 1. Rides — the proposed core experience

### RIDE-01 — “Who’s riding?” availability

**MVP decision — 2 November 2026:** Expiring availability statuses and spontaneous invitations are deferred beyond the MVP. The full proposal is retained below.

Let riders publish an expiring availability status, such as “Available tonight · City ride · Relaxed pace · Open to meeting people.” Include a time window, approximate area, ride type, and audience: friends, a club, or the local community.

Others can express interest or send an invitation. Availability must not require publicly broadcasting exact live location. Automatic expiry prevents old statuses from appearing current.

### RIDE-02 — Structured planned rides

**MVP agreed — 2 November 2026:** Planned rides include title, organizer, meeting place/time, destination or route link, riding expectations, capacity, participation rules, join/leave, cancellation, and clear plan updates. **Access policy agreed — 3 November 2026:** The organizer approves join requests. Public previews show city, date, and a short description; exact meeting details, the participant list, chat, and photos are visible only to approved participants. Other participation behavior and remaining ride fields require design.

Each ride has a dedicated page with a title, meeting point, departure time, route or destination, planned stops, estimated distance and duration, and an organizer.

Describe riding style, surface, experience expectations, and group rules. Example labels include relaxed, touring, beginner-friendly, paved, mixed, and off-road. Rules might cover overtaking the leader or regrouping at stops.

Support public, approval-required, and invite-only participation; a participant limit; and join/leave actions. Prioritize suitability based on expectations, experience, and road conditions rather than primarily on engine displacement.

### RIDE-03 — Ride Rooms

**MVP agreed — 2 November 2026:** Ride Rooms include the current plan, participants, organizer announcements, lightweight text chat, and notifications for significant changes or cancellation. Live tracking remains deferred. Club chat and direct messaging are separate from this scope.

Joining a ride opens one place for participants, announcements, lightweight conversation, the plan, and notifications. Departure-time or meeting-point changes should be unmistakable rather than buried in chat.

The room persists across the ride lifecycle: preparation before departure, coordination during the outing, and shared memories afterward.

Later capabilities may include optional group-location sharing and designated regrouping points. Exact tracking is not required for the initial version.

### RIDE-04 — Shared ride memories

**MVP agreed in part — 2 November 2026:** A small shared photo album attached to a ride is included. Written ride recaps are deferred. Planned rides and lightweight Ride Rooms are agreed separately under RIDE-02–03.

Support a shared photo album and ride recap attached to the ride. Connect the recap to participating riders, relevant clubs, and bikes where appropriate and permitted.

The experience should make organizing the next ride natural rather than end as an isolated post.

## 2. Rider profiles and garages

### GARAGE-01 — Rider identity

**MVP agreed — 2 November 2026:** Name, avatar, city, languages, and riding interests. Owning a motorcycle is optional. Rich profile customization is deferred.

Profiles can show city, languages, riding interests, clubs, and motorcycles. Motorcycle ownership is not mandatory: prospective riders and passengers should be able to participate.

### GARAGE-02 — Individual motorcycle pages

**MVP agreed — 2 November 2026:** Multiple motorcycles with make, model, year, and photos. Detailed modifications, ownership stories, and previously owned bikes are deferred. **Model pictures agreed — 3 November 2026:** Selecting a model and year should automatically show a verified matching model reference photo. Show no photo when that year/generation is unavailable. **Bike uploads agreed — 3 November 2026:** Riders can add their own bike photos while the catalogue reference remains the default cover until deliberately changed (D-041).

Support multiple motorcycles, including previously owned bikes, instead of a single current-bike field. A bike page may contain model, year, photos, modifications, an ownership story, and related rides, posts, or questions.

### GARAGE-03 — Private ownership records

**MVP agreed — 2 November 2026:** A private maintenance log with date, mileage, and work performed, separate from the bike’s visible profile. Receipts, expense tracking, reminders, and analytics are deferred.

Maintain service history, maintenance entries, expenses, receipts, and reminders based on dates or mileage. Begin with manual entries and reminders rather than complex integrations.

Separate the public showcase from private records. Making a bike profile public must not automatically expose receipts, identifying details, storage locations, or costs. Recorded trip distance is not the authoritative odometer reading.

### GARAGE-04 — Model-based discovery

**MVP decision — 2 November 2026:** Deferred beyond the MVP. The full model-based discovery proposal is retained below.

Help riders find other owners of the same model, their modifications, discussions, and recommended services. Model relationships can also connect the garage to relevant marketplace listings.

## 3. Clubs and community

### COMMUNITY-01 — Club workspaces

**MVP agreed — 2 November 2026:** A club page with name, image, description, city, member list, one owner/admin, and linked rides. Advanced roles, club chat, recurring rides, and calendars are deferred. Club membership uses owner approval: riders request to join, the owner approves/removes members, and members may leave. The owner must transfer ownership or close the club before leaving. Detailed visibility defaults and club-only ride behavior remain reviewable. **Implemented default — 3 November 2026:** One optional club image appears on its public basic page and club card. The active owner can upload, replace, or remove it. Closure keeps the public image visible and makes it read-only; ownership transfer preserves it under the new owner’s control.

Support informal groups and established clubs with public/private pages, membership approvals, member roles, announcements, discussions, recurring rides, and event calendars.

Organizer tools should include assigning ride leaders, pinning instructions, and moderating conversations. Strong organizer utility is a proposed adoption strategy, not a proven acquisition channel.

### COMMUNITY-02 — Contextual feeds

**MVP decision — 2 November 2026:** Contextual feeds are deferred beyond the MVP. The full proposal is retained below.

Proposed views:

- **Following:** People and clubs the rider follows.

- **Local:** Relevant rides, events, questions, and posts for a selected city.

- **My bikes:** Content connected to models the rider owns or follows.

Avoid making a generic infinite feed the initial product’s main value.

### COMMUNITY-03 — Structured posts

**MVP decision — 2 November 2026:** General structured posts are deferred beyond the MVP. The full proposal is retained below.

Post types may include ride reports, questions, modifications, reviews, photos, and event announcements. Type and model/location context should help people retrieve useful information later.

### COMMUNITY-04 — Invitations and public previews

**MVP agreed — 2 November 2026:** Shareable ride and club links with useful previews that respect visibility settings. Richer invitation tools are deferred.

Ride and club links should have useful previews when shared into existing messaging groups. Minimize signup friction and avoid requiring an entire community to abandon its current chats before BIKERS.KZ becomes useful.

The exact split between public content and content requiring an account remains open.

## 4. Explore — routes, places, and local knowledge

### EXPLORE-01 — Community routes

**MVP decision — 2 November 2026:** The community route library, route discovery, route photos and stops, and recorded tracks are deferred beyond the MVP. The full proposal is retained below; limited ride-plan scope is agreed under RIDE-02.

**Scope added — 11 November 2026 ([#35](https://github.com/unstppbl/bikers/issues/35)):** Community route discovery, saved private collections and in-app GPS recording are approved and in progress. This supersedes the route-library and recorded-track deferrals for this task. GPS tracks are private by default; explicit consent publishes a separate public route snapshot. The foreground PWA provides start/pause/resume/finish controls, requires location permission and does not guarantee background recording. GPX import is outside this task. Live group-location sharing, offline maps and native navigation remain deferred. See the [feature backlog](feature-backlog.md) for all nine remaining areas.

Route pages may include surface, estimated duration, suggested stops, fuel-availability notes, photos, and when someone last rode the route.

Keep a planned route separate from a recorded ride. A participant’s detour must not silently overwrite the organizer’s plan.

### EXPLORE-02 — Navigation handoff

**MVP agreed — 2 November 2026:** Open a meeting point or destination in an external navigation app and support an organizer-supplied route link. Saved route collections, native navigation, offline maps, and advanced routing are deferred.

**Scope added — 5 November 2026 ([#18](https://github.com/unstppbl/bikers/issues/18)):** Self-hosted Kazakhstan maps, address search and road-motorcycle route calculation belong in the private ride plan. Organizers can select a meeting point, destination and intermediate stops; only the organizer and approved participants receive the exact saved points and route. External navigation handoff remains available. This does not add a community route library, recorded tracks, native turn-by-turn navigation or offline maps. The issue records live rollout acceptance separately from source implementation.

### EXPLORE-03 — Map

**MVP decision — 2 November 2026:** The useful-places map and directory are deferred beyond the MVP. The full proposal is retained below.

**Scope added — 9 November 2026 ([#22](https://github.com/unstppbl/bikers/issues/22)):** The owner approved a public useful-places map and directory, initially covering fuel, workshops, tire services, motorcycle shops, parking and meeting spots. Public OSM records retain their source and snapshot date. Signed-in riders can propose additions, corrections or closures; administrators review them before publication. Imported records are not community verification. This approval supersedes the earlier useful-places deferral. Service booking, payments and provider inquiries remain separate deferred work.

**Curation decision — 9 November 2026 ([#24](https://github.com/unstppbl/bikers/issues/24)):** Rename the feature **Map / Карта** and place it immediately after Motorcycles in navigation. Clear the initial place entries and build the directory through user suggestions and administrator review. OSM POI seeding is no longer the population policy; the self-hosted basemap, address search and routing remain. A future bulk import requires a new owner instruction.

Map categories may include meeting spots, motorcycle parking, workshops, tire services, shops, riding schools, and seasonal storage providers.

The map’s purpose is to answer “Where can I ride, stop, or get help?” rather than simply display nearby people.

### EXPLORE-04 — Dated condition reports

**MVP decision — 2 November 2026:** Community condition reports are deferred beyond the MVP. The full proposal is retained below.

**Scope added — 11 November 2026 ([#35](https://github.com/unstppbl/bikers/issues/35)):** Dated road-condition reports associated with community routes are approved and in progress, superseding this deferral. Reports show their author attribution and observation time, use the existing reporting/administrator moderation controls and must not imply that a route is safe. Production rollout acceptance remains tracked in the issue.

Community reports may flag loose gravel, closures, or other relevant conditions. Show attribution and submission/update time; prevent stale reports from remaining unexplained permanent warnings.

Do not promise that a community route is safe. Provide relevant information and its freshness instead.

## 5. Services and marketplace

### COMMERCE-01 — Service discovery and inquiries

**MVP decision — 2 November 2026:** Service-provider directory and inquiries are deferred beyond the MVP, along with bookings, quotes, deposits, and payments. A basic directory may be revisited separately from in-app inquiries; no post-MVP sequence is committed.

Let riders select a motorcycle and describe a need, such as tire fitting and balancing. Show relevant providers with service categories, contact details, opening hours, and attributed customer feedback.

Start with a directory and inquiry flow. Booking, quotes, deposits, payments, and reviews tied to completed jobs are later candidates.

### COMMERCE-02 — Structured marketplace listings

**MVP decision — 2 November 2026:** Marketplace listings for motorcycles, parts, accessories, and gear, including model-based listing discovery, are deferred beyond the MVP. The full proposal is retained below.

Support motorcycles, parts, accessories, and riding gear. Use structured fields alongside free text.

A proposed useful connection is browsing listings relevant to a motorcycle in the rider’s garage. Seller-declared compatibility must be labeled as a seller claim, not a guaranteed fit.

### COMMERCE-03 — Selective service-history sharing

**MVP decision — 2 November 2026:** Selective maintenance-history sharing for bike-sale listings is deferred beyond the MVP. The full proposal is retained below.

For a future bike sale, allow owners to attach selected maintenance history to a listing. Private receipts and identifying details remain hidden unless explicitly shared.

Escrow, shipping, financing, and full checkout are not proposed as initial-release requirements.

## 6. Events, media, and BIKERS.KZ Live

### MEDIA-01 — Events beyond rides

**MVP decision — 2 November 2026:** Standalone events beyond rides, including meetups, workshops, exhibitions, and other event types, are deferred beyond the MVP.

Support meetups, training sessions, workshops, exhibitions, club gatherings, and creator broadcasts. Connect the organizer, attendees, discussions, and media. Ticketing and streaming can be added later.

### MEDIA-02 — Staged content capabilities

**MVP agreed — 2 November 2026:** Small photo albums attached to rides and links to externally hosted videos. Written ride recaps, general media posts, native video uploads, creator profiles, subscriptions, and monetization are deferred.

Suggested progression:

1. Photo albums, ride reports, and links to external creator videos.

2. Native video uploads, creator profiles, and following/subscribing to creators; payment mechanics remain undecided.

3. Scheduled livestreams, moderated chat, replays, and creator monetization.

### MEDIA-03 — BIKERS.KZ Live

**MVP decision — 2 November 2026:** BIKERS.KZ Live, livestreams, live chat, and replays are deferred beyond the MVP. Streaming remains part of the long-term vision.

Initial streaming use cases could include event coverage, motorcycle builds, workshop sessions, interviews, and bike walkarounds. Streaming should not be defined only as broadcasting while riding.

A connected event journey could be: discover → register → attend or watch → discuss → view the replay.

### MEDIA-04 — On-bike media safeguards

**MVP decision — 2 November 2026:** On-bike streaming implementation is deferred. The safeguards below remain requirements for any future on-bike streaming: hands-free operation, no required screen or chat interaction while moving, and no automatic publication of exact live rider location.

If on-bike streaming is introduced, design for hands-free operation without required screen or chat interaction while moving. A public broadcast must not automatically publish exact live rider location.

Streaming is part of the long-term vision, not a launch infrastructure requirement.

## 7. Trust, privacy, assistance, and operations

### TRUST-01 — Location controls

**MVP agreed in part — 2 November 2026:** Content visibility controls for rides, clubs, albums, links, and previews are included. Live-location sharing, live tracking, and their tracking controls are deferred beyond the MVP.

Allow users to choose the audience, duration, and precision of location sharing. Joining a ride must not automatically switch tracking on. Sessions should expire, and stale positions must display their last update time.

### TRUST-02 — Sensitive-location protection

**MVP agreed in part — 2 November 2026:** Maintenance records and sensitive ownership information stay private and separate from visible bike profiles. Tools for hiding sensitive locations in recorded tracks are deferred.

Provide controls to obscure home and other sensitive locations in published tracks. Keep exact storage locations and sensitive bike records private by default.

### TRUST-03 — Moderation and verification

**MVP agreed — 2 November 2026:** Reporting, blocking, and a simple administrator review queue are included. Verification badges are included and assigned manually by an admin. Badge meaning, recipient types, assignment criteria, and supporting checks remain to be defined.

Include reporting, blocking, club moderation, and an administrator review queue from the first public release.

Verification labels must explain what was actually checked: for example, a phone number, business identity, or completed booking. Do not imply that one check establishes general trustworthiness.

### TRUST-04 — Practical assistance

**MVP decision — 2 November 2026:** Community assistance requests are deferred beyond the MVP. The full proposal is retained below.

A later opt-in flow could request help with a flat tire, tools, or towing contacts. Keep community assistance distinct from emergency dispatch. Do not imply that nearby riders will see a request, respond, or arrive within a guaranteed time.

### TRUST-05 — Positive incentives

**MVP decision — 2 November 2026:** Reputation, reward, badge-based incentive, and recognition systems are deferred beyond the MVP. This deferral does not apply to the admin-assigned verification badges agreed in TRUST-03.

Avoid speed leaderboards. Candidate recognition could reward helpful answers, welcoming ride organization, and maintaining useful route information.

## 8. Kazakhstan-first experience and international readiness

### LOCAL-01 — Complete three-language experience

**MVP implementation agreed — 2 November 2026:** English, Russian, and Kazakh cover onboarding, core screens/actions, notifications, and error messages. The three-language commitment was already confirmed. Translation ownership and quality review remain to be assigned.

English, Russian, and Kazakh should cover onboarding, core actions, and notifications, not only menu labels. Translation ownership and quality checks need definition.

### LOCAL-02 — Search across scripts and aliases

**MVP agreed in part — 2 November 2026:** Basic motorcycle-model lookup supports common aliases and relevant Cyrillic/Latin spellings. Advanced cross-language content search and automatic translation of user content are deferred.

Search should accommodate relevant Cyrillic/Latin spellings and motorcycle model aliases. Multilingual user-generated content and automatic translation remain topics for later design.

### LOCAL-03 — Independent locale settings

**MVP agreed — 2 November 2026:** Metric units and KZT where applicable, with country, language, and currency kept independent. City selection is manual and does not require location permission. These defaults add no payment or expense-tracking scope.

Use metric units and KZT as proposed launch defaults while keeping country, language, and currency separate. Let people select a city without granting location access.

### LOCAL-04 — Off-season usefulness

**MVP agreed in part — 2 November 2026:** Off-season value comes from the already-approved garage and private maintenance log. There is no separate off-season feature package in the MVP; the wider long-term proposal is retained below.

Support garages, maintenance, builds, workshops, and community content so that value is not wholly dependent on organizing rides.

## Connections that make the platform coherent

- A rider joins a club, discovers a ride, participates through a Ride Room, and contributes to its album.

- A motorcycle page connects ownership history, modifications, model discussions, services, and relevant listings.

- A route connects local knowledge, useful stops, planned outings, and distinct recorded rides.

- An event connects discovery, attendance, conversations, media, and eventually live viewing and replays.

## Research boundary

This catalogue describes intended experiences, not verified product uniqueness, market demand, technical feasibility, legal compliance, or competitor gaps. Earlier discussion mentioned MotoSpot, Cardo Ride, and REVER as research leads. Those comparisons should be freshly verified and sourced before being used in a business case or external material.
