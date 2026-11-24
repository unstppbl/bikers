import type { Locale, MapPoint, Ride } from './types'

export function directionsTo(destination: string) {
  return `https://www.google.com/maps/dir/?api=1&destination=${encodeURIComponent(destination)}`
}

export function rideBriefStops(ride: Ride) {
  const points = ride.map_plan?.points ?? []
  const finish = points.length > 1 ? points.at(-1) : undefined
  return [
    { name: ride.meeting_point ?? points[0]?.label ?? '', point: points[0] },
    ...points.slice(1, -1).map((point) => ({ name: point.label, point })),
    { name: ride.destination || finish?.label || '', point: finish },
  ].filter((stop) => stop.name)
}

interface BriefTime { value: string; label: string; text: string }
interface BriefStop { name: string; point?: MapPoint; count?: string }
interface BriefSnapshot {
  locale: Locale
  heading: string
  title: string
  departure: BriefTime
  savedAt: BriefTime
  routeStops: string
  navigateToMeeting: string
  stops: BriefStop[]
  latestHeading: string
  latest?: { body: string; at: string; when: string }
  noLatest: string
  notices: string[]
}

function escapeHTML(value: string) {
  return value.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;').replaceAll('"', '&quot;').replaceAll("'", '&#39;')
}

// Save only the fields riders use at the meetup. The file works without the
// app, loads no resources, and has no connection to account or ride storage.
export function rideBriefHTML(brief: BriefSnapshot) {
  const e = escapeHTML
  const time = (entry: BriefTime) => `<p>${e(entry.label)} <time datetime="${e(entry.value)}">${e(entry.text)}</time></p>`
  const meeting = brief.stops[0]
  const directions = meeting ? directionsTo(meeting.point ? `${meeting.point.lat},${meeting.point.lon}` : meeting.name) : undefined
  return `<!doctype html>
<html lang="${e(brief.locale)}"><head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'">
<meta name="referrer" content="no-referrer">
<title>${e(brief.heading)} · BIKERS.KZ</title>
<style>
  body { margin: 24px auto; padding: 0 16px; max-width: 640px; font: 16px/1.5 system-ui, sans-serif; }
  h1, h2, p, li { overflow-wrap: anywhere; }
  h1 { line-height: 1.2; }
  h2 { font-size: 1.2em; margin-top: 1.5em; }
  li { margin: 8px 0; }
  .notice { border-block-start: 1px solid #888; margin-top: 24px; }
  .update { white-space: pre-wrap; }
</style></head><body>
<p>BIKERS.KZ · ${e(brief.heading)}</p><h1>${e(brief.title)}</h1>
${time(brief.departure)}
<h2>${e(brief.routeStops)}</h2>
<ol>${brief.stops.map((stop) => `<li>${e(stop.name)}${stop.point ? ` <small>(${e(String(stop.point.lat))}, ${e(String(stop.point.lon))})</small>` : ''}${stop.count ? `<br>${e(stop.count)}` : ''}</li>`).join('')}</ol>
${directions ? `<p><a href="${e(directions)}" rel="noreferrer">${e(brief.navigateToMeeting)}</a></p>` : ''}
<h2>${e(brief.latestHeading)}</h2>
${brief.latest ? `<time datetime="${e(brief.latest.at)}">${e(brief.latest.when)}</time><p class="update">${e(brief.latest.body)}</p>` : `<p>${e(brief.noLatest)}</p>`}
<div class="notice">${time(brief.savedAt)}${brief.notices.map((notice) => `<p>${e(notice)}</p>`).join('')}</div>
</body></html>`
}
