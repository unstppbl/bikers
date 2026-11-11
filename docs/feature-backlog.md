# Feature backlog

Updated 11 November 2026. The agreed MVP is implemented; this table preserves the remaining feature areas from the product discussion. Deferred entries are proposals without an approved sequence or delivery date. [#35](https://github.com/unstppbl/bikers/issues/35) approves the Routes work below.

| Area | Remaining scope | Status |
| --- | --- | --- |
| Spontaneous rides | “Who’s riding?” statuses and spontaneous invitations | Deferred |
| Live coordination | Group locations, GPS tracking and regrouping points | Deferred |
| Clubs | Club chat, richer roles, recurring rides and club calendars | Deferred |
| Garage | Maintenance reminders, expenses, receipts, modifications and ownership history | Deferred |
| Routes | Community route library, saved collections, recorded tracks and road-condition reports | In progress — [#35](https://github.com/unstppbl/bikers/issues/35) |
| Social | Feeds, structured posts, model-owner discussions and written ride recaps | Deferred |
| Commerce | Marketplace, service inquiries/bookings, quotes and payments | Deferred |
| Media/events | Standalone events, native video, creator tools and livestreaming | Deferred |
| Other expansion | Assistance requests, reputation, automatic translation, native mobile apps, offline maps and native navigation | Deferred |

## Approved Routes scope

Routes includes community route discovery, saved private collections, private GPS tracks recorded in the app, and dated community road-condition reports. Recording uses explicit start, pause, resume and finish controls in the foreground PWA. Browser/device support and permission are required; background recording is not guaranteed. GPX import is outside this task.

Tracks remain private by default. Publishing requires explicit consent and creates a separate public route snapshot; it does not expose the private track or turn on live group tracking. Collections remain private. Route and condition content uses the existing reporting and administrator moderation controls. The separate Live coordination row stays deferred.

The [roadmap](mvp-and-roadmap.md) and [feature catalogue](feature-catalogue.md) retain the original decisions and dated scope additions. Map/useful-place curation, private ride-plan maps, existing ride reminders and calendar downloads are already delivered feature areas, rather than entries in this deferred backlog. Production rollout and acceptance for Routes are tracked separately in #35.
