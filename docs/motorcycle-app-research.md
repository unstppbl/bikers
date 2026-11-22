# Motorcycle app research for BIKERS.KZ

23 November 2026 · [Research issue #54](https://github.com/unstppbl/bikers/issues/54)

**Start with route discovery and reuse.** Give Routes a visible entry under
the phone Map tab, publish a small organizer-reviewed Astana collection, and
let an organizer create a group ride from a route. Improve the invite and
approved meet-up experience alongside that work. This gives a rider something
to do when the ride calendar is quiet and gives an organizer a reason to use
the same app for the next ride.

The comparison uses first-party help, product pages and publicly visible web
screens. The BIKERS.KZ baseline is `main` at `a3b22f2` and public API spot
checks. Competitor mobile apps, Kazakhstan navigation coverage and on-device
performance were outside the review. Product priorities and pilot targets
below are proposals.

## What comparable apps do

| App | Observed workflow | Opportunity for BIKERS.KZ |
| --- | --- | --- |
| REVER | Select a riding style on the map and preview a curated route. The app offers a single Record action and distinguishes following a stored line from calculating directions through waypoints. GPX export and offline maps are listed as Pro features. [Quick start](https://www.rever.co/help/getting-started) · [Plan comparison](https://www.rever.co/pro) | Show a route preview before the detail page. Keep **Navigate to start** and **Download GPX** distinct. |
| calimoto | Generate a round trip from a chosen start, length and direction. Its map labels TomTom closures and OpenStreetMap access restrictions separately and shows source and duration. [Round trips](https://support.calimoto.com/hc/en-us/articles/7989918956572-How-Do-I-Plan-a-Round-Trip) · [Road closures](https://support.calimoto.com/hc/en-us/articles/19562744207900-See-Real-Time-Road-Closures-Right-on-the-Map) | Offer curated loops by time and surface. Keep a rider's observed road condition distinct from a formal closure or access restriction, and show when it was observed. |
| Kurviger | When importing a track, the preview overlays its geometry against a calculated route and shows where they diverge. Offline navigation requires preparation with the saved route and downloaded maps. [Import preview](https://docs.kurviger.com/route_transfer/import) · [Offline navigation](https://docs.kurviger.com/offlinenavigation) | Preserve the published track through export. Show the difference before an organizer accepts a newly calculated ride plan. |
| Detecht | Riders can trim endpoints when saving a recording. Ride Sessions use a shared link or QR code and show members' positions and statuses within the session. Joining the session requires the app. [Track and save](https://www.detechtapp.com/help/track-and-save-a-ride) · [Session help](https://www.detechtapp.com/help/ride-together-in-a-ride-session) | Preview and trim the public route copy while keeping the original recording private. Make BIKERS.KZ ride invites useful in a guest browser. |
| Cardo Ride, formerly RISER | Getaways organize a future group ride. PackRide handles live falling-behind and break signals. The Pro comparison includes unlimited PackRide. [Features](https://cardoride.com/features) · [Pro comparison](https://www.cardoride.com/pro) | Put the meeting point, regroup stops and latest organizer update together in the approved Ride Room. |
| MotoHub.kz | The Kazakhstan portal has city filters, a calendar, useful places and routes. Its public Almaty season-opening page includes dates, status, a named organizer and sharing. Its riding project offers Kazakhstan route ideas and Telegram/WhatsApp sharing. [Event](https://motohub.kz/events/22-otkrytie-motosezona) · [Jelmen Jarysqan](https://motohub.kz/projects/jelmen-jarysqan) | Focus on the complete organizer workflow from sharing and approval to the actual meetup and a route that can be used again. Local listings alone are already available. |
| BRAP | A same-day ride plan has a departure time and optional notifications. It disappears from discovery at departure. Its garage provides service reminders by date or mileage. [Same-day planner](https://brap-app.com/en/ride-planner) · [Garage](https://brap-app.com/en/motorcycle-garage) | Consider an expiring **Riding today** option if organizers already coordinate short-notice rides. A private service reminder could give owners a reason to return between rides. |

## BIKERS.KZ today

- **The visual direction is already established.** Main uses warm asphalt,
  orange actions, Fira Sans, ride photography, city fallbacks and cream date
  plates. Garage now has its own phone tab. Routes already selects Map in the
  bottom bar, but the phone Map screen offers no direct switch to Routes.
  The current route finder is under **Me → Explore → Routes**. Keep the visual
  system and fix that path in context. [Design review](design-review.md#photo-led-redesign--19-november-2026) · [Navigation](../apps/web/src/App.tsx) · [Me](../apps/web/src/pages/Me.tsx) · [Map](../apps/web/src/pages/Places.tsx)
- **Routes has the core feature set.** Community search already lists
  distance, duration and surface. The detail has a map, attribution, private
  collection save and dated condition reports further down. Private foreground
  recording and explicit publication already exist. The list has no line
  previews or time/surface filters. A route cannot prefill a group ride.
  [Routes screen](../apps/web/src/pages/Routes.tsx) · [Routes delivery #35](https://github.com/unstppbl/bikers/issues/35)
- **Ride Room already coordinates the group.** Guests get the city, date,
  description, status and any approved cover. Approved participants get the
  exact plan, roadbook, announcements, chat, roster and album. The share
  action copies a link. The roadbook already opens navigation to the meeting
  point. A clearer invite, explicit rider-fit fields, and placing the latest
  update beside the meetup are improvements to those existing flows.
  [Ride detail](../apps/web/src/pages/RideDetail.tsx) · [Share action](../apps/web/src/ui.tsx) · [Access contract](development.md#accounts-and-permissions)
- **The pilot needs real content.** The 23 November public spot check returned five
  upcoming rides, one in Astana, and empty public route and place listings.
  Recruit local route contributors alongside the interface work so the pilot
  can distinguish an empty catalogue from an unusable one.
  [Rides feed](https://bikers.kz/api/v1/rides) · [Routes feed](https://bikers.kz/api/v1/routes) · [Places feed](https://bikers.kz/api/v1/places)

## Proposed screen flows

### 1. Find a local loop

Open **Map → Routes** on a phone. Put **Places** and **Routes** side by side
above the current workspace, keep the explicitly selected city when switching,
and make the existing Routes URL shareable with its filters. Start with the
list and a small route-line thumbnail on each card. Add a full Routes map
view when the pilot catalogue is large enough for spatial browsing.

```text
Map                         Astana ▾
Places     Routes
Search routes
Riding time ▾     Surface ▾

   Route line     Organizer's weekend loop
                  86 km · about 2 h · Paved
                  Surface checked 14 Nov · organizer note
```

Illustrative card. Use estimated time for plans and recorded time for tracks.
Show **No recent reports** when the only condition information is missing,
expired or resolved. A recent gravel or roadworks report displays its observed
date and opens the existing report. Use a bounded preview derived from the
authorized public route, with no per-card map instance or detail fetch.

### 2. Go from route detail to a group ride

Put distance, surface, latest relevant condition and any verified public fuel
or rest stops close to the map. Offer **Create ride from route** beside
**Save to collection**. Start with planned community routes. Prefill the city
and eligible waypoints in the existing **New ride** form, show the resulting
plan, and ask the organizer to confirm the meetup and departure before
publishing. Keep the source route and the ride plan independent so later route
edits cannot silently move a published ride's meetup or stops. If routing
recalculates a different line, show the difference in the organizer preview.
[Route detail](../apps/web/src/pages/Routes.tsx) · [Ride form](../apps/web/src/pages/Rides.tsx)

Follow with **Navigate to start** for a point handoff and **Download GPX** for
the full authorized geometry. Start with public route export. Preserve
recorded segments if tracks are supported later, and keep private meetup
coordinates out of public links, public route cards and exports.

### 3. Share, join and find the meetup

Put **Share ride** on ride detail with a native share option where supported,
**Share to Telegram**, and **Copy link**. Generate a preview from the public
ride projection so the forwarded link exposes the same fields as guest web.
Test organizer-controlled public riding time, surface and suitability before
expanding that projection. Keep the exact time, meeting pin and route in the
approved room. A rider who signs in lands back on the same ride and sees their
request status. [Telegram sharing](https://core.telegram.org/widgets/share) · [Current access contract](development.md#accounts-and-permissions)

In the approved **Ride Room**, order the first actions around the departure
time, **Navigate to meeting point**, and the latest organizer update with a
timestamp. Add regroup stops to the existing roadbook when organizers use
them. For EN, RU and KK, test readable status labels and one-hand access at
320 and 390 pixels, including narrow Telegram webviews and iOS Safari. Test
guest, pending, approved, removed and cancelled states for both page and
share preview. [Existing roadbook and state handling](../apps/web/src/pages/RideDetail.tsx)

## Pilot and follow-ups

Pilot with two or three Astana organizers, six contributed routes and a few
verified public stops over four active riding weeks. Astana matches the
existing pilot map and gives moderation and translation a bounded area. For
each route, keep the contributor, observation date, surface and public start
point. Prioritize routes riders have actually ridden. [Astana viewport](https://github.com/unstppbl/bikers/issues/50) · [Place data policy](places-data.md)

| Question | Proposed check |
| --- | --- |
| Can riders find a route that fits? | In five phone sessions, aim for four riders to find a matching loop and explain its surface and latest report within a minute. Count route list to detail and detail to collection save separately. |
| Will organizers reuse routes? | Aim for two organizers to publish from a community route and at least one to repeat. Track route-to-form opens and successful publishes and compare with starting from the blank form. |
| Does sharing get people to the meetup? | Count invite opens, join requests and approvals as distinct funnel steps. In the phone sessions, aim for four of five approved riders to find the meet pin within 20 seconds. If attendance matters, ask the organizer to confirm it. A directions click measures intent. |

Use aggregate counts with no GPS, chat contents or contact identifiers in
pilot analytics. Treat this small sample as a usability and repeat-use check.

**Next, if the pilot shows demand.** Add endpoint trimming and public preview
to the private recording's **Review for publishing** flow. Publish or export
only the approved geometry. Try **Riding today** with expiry and the current
approval flow if organizers ask for shorter lead times. A private date-based
service reminder fits the existing garage maintenance log and can be tested
independently. [Current recording scope](feature-backlog.md#approved-routes-scope) · [Garage research](garage-product-research.md)

**Later.** GPX import needs an original-line preview and a decision on
recalculation. Route generators, offline navigation and group location need
measured Kazakhstan routing quality, battery and background behavior, and
repeatable rider demand. Crash detection would require separate local
validation of device behavior and who can receive the alert. The existing
backlog already covers live coordination, native navigation, offline maps,
social feeds and commerce. [Feature backlog](feature-backlog.md) · [Detecht crash alert help](https://www.detechtapp.com/help/set-up-crash-detection)
