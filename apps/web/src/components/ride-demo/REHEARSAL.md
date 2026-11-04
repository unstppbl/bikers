# BIKERS.KZ ride-flow rehearsal

## Purpose and integration

This is an authored product illustration, not a working ride interface. It never
imports the API client, fetches data, or changes a user account. The fictional
example is labelled in both the surrounding figure and the faux browser chrome.
The public preview contains city, date, and description. An organizer's approval
reveals the illustrative private meeting plan and group chat.

```tsx
import { BikersRideDemo } from '../components/ride-demo/BikersRideDemo'

<BikersRideDemo locale={locale} />
```

The component imports its own scoped CSS. It uses the host's Night Ride semantic
tokens and local font. It requires no new application dependency. It is intended
for the dedicated development preview, not the production home card.

The normal scene is 358 × 342 CSS pixels; the compact scene is 320 × 308.
Below a 300px container, the figure hides rather than scaling text below the
intended reading size. Its real Play/Replay control is 44px tall, outside the
`aria-hidden` faux interface. No element inside the story is a tab target.
The figure supplies a localized screen-reader summary.

## Gold and construction

The starting composition was Stageshow `examples/dark-hero`: a compact dark
window with a deliberate bottom bleed. Measured click-to-state choreography
came from `examples/share`. The original gold was rendered before adaptation.
The adaptation retains the kit's Window, Row, States, Badge, Thread, Bubble,
cursor, measurements, and clock. It replaces the content, composition, timings,
tokens, and copy with the approval-to-private-room story.

The 26 files in `kit/` are an unmodified dependency closure. SHA-256 hashes and
installed source paths are in `VENDOR-MANIFEST.json`; the MIT license and
copyright notice are retained alongside this file. No teaching library,
unrelated example, network font, or external artwork ships with this component.

## Playback

The initial view is the complete payoff poster. Play example is the only cause
of playback. The story runs once, holds the final room, and can be replayed
explicitly. Replay fades the old scene before restarting. It does not loop.
Reduced motion shows the same complete poster with a static caption and no
playback control. The idle poster has no simulated cursor.

Five beats, about eleven seconds including the final hold:

1. Select a public Almaty ride.
2. Request a place.
3. Wait for the organizer.
4. Receive approval and open the room.
5. See the private meeting plan and group chat.

The cursor lands on measured targets before each user-triggered state change.
Approval is a timed system event. English, Russian, and Kazakh use the same
story and layout; all primary labels and chat text are localized.

## Rehearsal environment and evidence

All automated rendering was confined to the authored synthetic fixture at
`http://127.0.0.1:5187/`, in `/private/tmp/bikerskz-stageshow-preview`.
It imports this component and a snapshot of the host's design tokens; it does
not import application routing, session state, API code, or real user content.
No browser automation was run against the BIKERS.KZ application or its users.

Stageshow's purpose-built `film.mjs`, `jumps.mjs`, and `layout.mjs` supplied
the captures and checks. A temporary copy of `film.mjs` added PNG-sequence
output only, restricted to that synthetic loopback origin. The same captured
frames were encoded using ffmpeg; the skill's installed files were unchanged.

Evidence is retained in:

`/Users/user/.codex/visualizations/2026/11/01/01a197b8-57c6-7a12-946b-c767709cb502/ride-flow/`

The short videos are 13 seconds at 20fps, including a final hold. English and
Russian use the 358px design; Kazakh also exercises the compact 320px design.
Final posters cover all three languages. The Kazakh poster was additionally
captured with reduced motion. Local captures include the five-beat state sheet,
50ms frame sequences, a 4× text/glyph inspection, and the original gold.

## Findings and corrections

The rehearsal caught and corrected inherited dark text, uneven faux-browser
chrome inset, an underfilled transition frame, a placeholder meeting value,
and chrome text that was too small. The final example uses a concrete fictional
café courtyard, 12px chrome text, and crossfades without the former delayed
entrance. Primary content remains whole; only decorative continuation bleeds.

The final English, Russian, and compact Kazakh full-story captures returned zero
never-intentional gate findings. Final localized poster gates also passed.
The outer Kazakh 390px host layout scan found one real control and three padded
containers with no box-fit findings. These gate results are necessary checks,
not a claim of complete visual or performance approval.

Remaining detector candidates were inspected rather than suppressed:

- The `near aligned` candidate compares a hidden board heading with text inside
  a separately padded card. The visible heading/card inset is intentional.
- `half-pixel glyph` candidates concern icons inside fractional-width flex text
  groups. Enlarged stills and sampled transition frames show whole glyphs;
  the independent reviewer found no clipped glyphs.
- The repeated MapPin is recognized as a potential handoff during navigation.
  These are separate panes crossfading, not a shared icon flight.

A fresh-eye reviewer compared the rendered gold with the localized posters and
selected 50ms story frames. After the meeting-value and chrome-size corrections,
the reviewer reported no remaining concrete composition findings. A second
review of every frame 0023–0035 and 0150–0175 found no visible hop or clipped
glyph: the briefly duplicated pins were outgoing and incoming navigation panes.

## Limits

Automated motion coverage is **inconclusive**: occasional host rAF sampling gaps
exceeded the tool's threshold. The English 13s capture observed 3,145 geometry
frames with 17 gaps; Russian observed 3,159 with three gaps; compact Kazakh
observed 3,152 with 16 gaps. A separate scan
without screenshot overhead still had one gap. Motion within those gaps was
not judged, and the exported 20fps video is not proof of live frame rate.
The reports retain the gaps and candidates; no threshold or ignore marker was
changed to obtain a pass.

The host's separate browser QA checked the actual development preview: manual
play at 390px in English, reduced-motion static content, Russian, and the 320px
Kazakh static page without horizontal overflow. The isolated headless Chromium
rehearsal does not establish Safari rendering, real-device performance,
offscreen/hidden-tab behavior, or complete keyboard behavior inside the host.
The original MVP flows and user content were outside this rehearsal's scope.

## Rechecking the vendored source

From `apps/web`, with the same installed skill available:

```sh
python3 - <<'PY'
from pathlib import Path
import hashlib, json
base = Path('src/components/ride-demo')
source = Path('/Users/user/.codex/skills/stageshow')
manifest = json.loads((base / 'VENDOR-MANIFEST.json').read_text())
for item in manifest['files']:
    local = (base / item['file']).read_bytes()
    assert local == (source / item['file']).read_bytes(), item['file']
    assert hashlib.sha256(local).hexdigest() == item['sha256'], item['file']
assert (base / 'LICENSE-STAGESHOW.txt').read_bytes() == (source / 'LICENSE').read_bytes()
assert (base / 'NOTICE-STAGESHOW.md').read_bytes() == (source / 'NOTICE.md').read_bytes()
print(f"Verified {len(manifest['files'])} files and exact license/notice.")
PY
```

This check passed for all 26 files after the final runtime freeze. Authored
component/dictionary ESLint and whole-app TypeScript checks also passed. The
host's integration run separately passed its 90 tests and production build.
