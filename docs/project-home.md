# Project Home

Native Space page: [Open in Space](https://chatgpt.com/space/page_61a8f62f0cdc819199cb5455c4475e8c)  
Space snapshot: 4 November 2026

BIKERS.KZ is an all-in-one motorcycle platform for riders and their communities. The vision connects riding together, motorcycle ownership, services, commerce, events, and media.

**Owner:** Altay · **Updated:** 4 November 2026

## Confirmed direction

- **Name:** BIKERS.KZ.

- **First market:** Kazakhstan.

- **Languages:** English, Russian, and Kazakh.

- **Vision:** An all-in-one motorcycle platform, with international expansion and streaming in view. Their timing and implementation remain open.

## Agreed MVP scope

**MVP scope is agreed:** planned rides and lightweight Ride Rooms, profiles and garages, basic clubs and shareable links, external navigation, ride photos and external video links, privacy/moderation, admin-assigned badges, and complete English/Russian/Kazakh MVP flows. The [MVP roadmap](mvp-and-roadmap.md) records exact limits and deferrals.

**Delivery direction:** Web/PWA first, mobile apps later; React/TypeScript/Vite, Go, and PostgreSQL; Telegram OTP-only user signup/login; homelab deployment behind the public VPS. The technical design, implementation defaults, and build order remain draft recommendations.

**Confirmed release process — 4 November 2026 (D-042):** Commit all changes and push to [the GitHub repository](https://github.com/unstppbl/bikers), then build and deploy on Jarvis from its `main` checkout. Production images are built on Jarvis and tagged with the exact commit; the Compose project remains `bikerskz`. At this confirmation, the first checkout-based cutover is planned.

**Implementation — 3 November 2026:** All agreed MVP feature slices are implemented and deployed: Telegram OTP, profiles/avatars, garages/private maintenance and catalogue-default bike photos, owner-approved clubs, approved rides with private chat/announcements/albums/video links, inbox and optional Web Push, reports/blocks, audited administrator review/manual community badges, and independent locale preferences. English, Russian and Kazakh screens and errors are included. PostgreSQL-backed race checks, frontend checks and a private backup/restore rehearsal passed. Jarvis now runs the restored app with the owner's audited admin role and configured Web Push credentials. [bikers.kz](https://bikers.kz) is live over verified HTTPS. The owner enabled notifications in Arc and registered one browser subscription. One owner-only diagnostic push was accepted by the provider. The owner confirmed that the notification appeared on his Mac in Arc and clicking it opened `/inbox`. Moderation/badge criteria and account-lifecycle policies remain launch gates. Implementation defaults remain reviewable.

**Visual refresh — 4 November 2026:** Altay approved the deployed Night Ride baseline: charcoal and lime, condensed type, motorcycle icons and the credited KTM 390 Adventure 2024 reference photo. The latest usability pass improves 44px controls, shorter-screen navigation, keyboard access, route focus and reduced motion. All 90 frontend tests, typecheck, lint and build passed, with desktop and compact English/Russian/Kazakh browser checks. A separate development-only ride walkthrough has localized animations and static reduced-motion posters. MVP feature scope is unchanged.

One journey supported by the agreed scope is: discover a ride → understand the plan → join → coordinate → meet → share photos → ride together again. Clubs provide group context, while garages and private maintenance logs provide personal utility between rides. Ride Rooms as the flagship and the adoption strategy remain proposals.

## Project pages

- [Feature Catalogue](feature-catalogue.md) — Complete catalogue of 32 features across eight areas, including connected experiences and safeguards.

- [Proposed MVP & Roadmap](mvp-and-roadmap.md) — Agreed MVP scope and deferrals, with proposed later capabilities and validation metrics.

- [Decisions & Open Questions](decisions-and-open-questions.md) — Confirmed decisions, proposals awaiting agreement, and unresolved questions.

- [Draft Technical Design and Build Plan](technical-design-and-build-plan.md) — Proposed modules, data model, API, account flow, permissions, deployment design, and six build phases.

## Next decisions

Choose the first launch city or community, primary audience, delivery order, and later mobile-app timing/framework. Ride organizer approval and the public-preview/approved-participant visibility rules are confirmed. Define remaining participation/club-membership behavior, other visibility settings, verification-badge meaning and criteria, and moderation responsibilities. Session/recovery policy, detailed tooling, staffing, budget, delivery dates, pricing, monetization, and success measures remain open.
