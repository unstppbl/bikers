# BIKERS.KZ documentation

These editable Markdown copies mirror the native BIKERS.KZ Space pages. This snapshot includes the decisions recorded through **3 November 2026**, including organizer-approved ride joining and the public-preview versus approved-participant access rules.

| Project document | Native Space page |
| --- | --- |
| [Project Home](project-home.md) | [Open in Space](https://chatgpt.com/space/page_61a8f62f0cdc819199cb5455c4475e8c) |
| [Feature Catalogue](feature-catalogue.md) | [Open in Space](https://chatgpt.com/space/page_4314e7d6b2e881919f1f014b7dc1c478) |
| [Proposed MVP & Roadmap](mvp-and-roadmap.md) | [Open in Space](https://chatgpt.com/space/page_1a9627318dd08191b58140311e0e8f41) |
| [Decisions & Open Questions](decisions-and-open-questions.md) | [Open in Space](https://chatgpt.com/space/page_5150f0f0e93c8191b6e26726a15379c1) |
| [Draft Technical Design and Build Plan](technical-design-and-build-plan.md) | [Open in Space](https://chatgpt.com/space/page_0da7058a10148191a17f468709afbc18) |

[Local development and API](development.md) documents the repository implementation and operating contract. Run commands are in the [repository README](../README.md).

[Backend architecture](backend-architecture.md) records the approved package map, transaction ownership, and automated dependency check for the implementation.

[GitHub Actions deployments](github-actions-deployment.md) records the public CI/private runner boundary, automatic release gates, and pause/retry procedures.

[Operations](operations.md) covers administrator bootstrap, Web Push configuration, proxy requirements, backup/restore and rollback. [MVP delivery checklist](mvp-delivery-checklist.md) tracks the full-scope implementation and verification.

[Model reference photographs](model-reference-images.md) records the verified model-year coverage, source credits, and image licenses.

[Night Ride design review](design-review.md) records the approved visual direction, Unslop usability improvements and development-only Stageshow walkthrough.

[Motorcycle photo API research](motorcycle-photo-api-research.md) compares official API coverage, costs, storage rights, and generated-image limitations.

## Maintaining both copies

Keep the repository documents and native Space pages aligned when the project changes. Synchronization is performed explicitly; no automatic or scheduled sync is configured. Reconcile intervening edits before copying content in either direction.

Each file retains its source Page URL and snapshot date. Links between these local documents use relative paths. The local document title adds one heading level above the native Page section hierarchy; fenced code and Mermaid diagrams are preserved.

Confirmed decisions and accepted MVP limits are distinct from proposed technical defaults and build order. The technical design remains a draft except where it explicitly cites confirmed decisions. Keep the complete feature catalogue, including deferred ideas, and leave the original library documents intact.
