# Astana route and ride pilot

The first release connects three tasks. Riders find a local route in Map,
organizers reuse a published planned route in New ride, and invited riders
request approval and find their exact meeting point. [Research and proposed screen flows](motorcycle-app-research.md) · [Implementation issue #56](https://github.com/unstppbl/bikers/issues/56)

## Release and operating plan

1. Release the Places and Routes switch, route filters, route previews and
   recent reports. Keep list reads bounded and preserve the selected city.
2. Release route-to-ride prefill, point navigation and public GPX. The
   organizer checks the dashed source line against the calculated ride plan,
   confirms the meeting point and departure, and publishes an independent
   ride. Public GPX uses published route geometry.
3. Release guest share links, Telegram previews and the plan-first approved
   Ride Room. The guest preview comes from the existing public ride fields.
   Approximate riding time, surface and suitability can go in the organizer's
   public description. Exact time, meeting point, plan and announcements
   remain in the approved room.
4. Run the app's web, backend and PostgreSQL checks. Check the mobile flows in
   English, Russian and Kazakh and the guest, pending, approved, removed and
   cancelled access states. Merge through CI and verify the deployed Git
   revision, both release receipts and public readiness. This release uses
   the existing database schema. [Deployment procedure](github-actions-deployment.md)

## Run the pilot

Recruit two or three Astana organizers. Enroll five riders for short, observed
phone sessions and run the pilot for four active riding weeks after content
is ready. Assign someone to review submissions and translations before sharing
the pilot with a wider group.

Ask the organizers to contribute six loops they have ridden, with permission
to publish the geometry and its start point. Each entry needs a city, title,
surface, description with suitability or distance to fuel where relevant, and
the name of the contributor. Date any condition observation using the Routes
report form. For a few public fuel or rest stops, verify the location and
date and submit them via the existing Map moderation flow. Keep organizer
meetup pins and rider addresses out of the contributed geometry and public
descriptions. [Place submission policy](places-data.md#current-policy-user-curated-places)

| Task | Record in the pilot tally |
| --- | --- |
| Find a matching loop on a phone | Number who find it in one minute and can explain its surface and report age. Target four of five. Count detail opens and collection saves separately. |
| Publish a group ride | Count source-route form opens and successful publishes. Aim for two organizers to publish from a route and at least one to repeat. |
| Use an invite and find the meetup | Count invite opens, requests and approvals separately. Aim for four of five approved participants to find the meetup in 20 seconds. Use organizer confirmation for attendance when needed. |

Use an aggregate session tally without account identifiers, chat, invite URLs
or GPS traces. Record the number of attempts, period, city, locale coverage and
recurring obstacles. Use the results to decide whether to test recording
endpoint trimming, same-day ride expiry or private service reminders.

## Scheduling and ride-day trial

Ask two or three organizers to share a scheduling poll with a group they
already ride with. Select the agreed time and publish the prefilled New ride
form. Share the published ride link so riders can request approval. Close the
loop by having an approved participant open **Ride-day brief**, save a dated
copy and open the file in airplane mode. Use a named regroup or fuel stop
and a head count during a stationary check. [Implementation issue #60](https://github.com/unstppbl/bikers/issues/60)

At the end of the trial, count the organizers who use the poll and brief for a
second ride without prompting. The proposed signal for testing the **Today**
home is two repeat organizers. Log only aggregate completion and repeat-use
counts.
