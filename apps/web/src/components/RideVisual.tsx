import type { CSSProperties } from 'react'
import { formatInTimeZone } from 'date-fns-tz'
import { dateParts } from '../ui'
import type { Locale, Ride } from '../types'

// "You're going" only fits a ride that still lies ahead.
export function goingLabel(ride: Ride, now = new Date()): 'youAreGoing' | 'approved' {
  const past = ride.starts_at ? new Date(ride.starts_at) <= now : ride.date < formatInTimeZone(now, ride.timezone ?? 'Asia/Almaty', 'yyyy-MM-dd')
  return ride.status === 'cancelled' || past ? 'approved' : 'youAreGoing'
}

// Compact telemetry date pill for ride cards and headers.
export function DatePlate({ date, locale, timeWindow, size = 'card' }: { date: string; locale: Locale; timeWindow?: string; size?: 'card' | 'hero' }) {
  const parts = dateParts(date, locale)
  return <span className={`date-plate date-plate-${size}`} aria-hidden="true">
    <strong>{parts.day}</strong>
    <span>{parts.month}</span>
    {timeWindow && <em className="date-plate-window">{timeWindow}</em>}
  </span>
}

// Subtle topographic fallback backdrop when neither a cover photo nor a city photo is available.
export function CityArt({ city }: { city: string }) {
  const name = city.trim()
  const words = name.split(/\s+/)
  const characters = Math.max(4, ...words.map((word) => [...word].length), words.length > 1 ? Math.ceil([...name].length * 0.6) : 0)
  return <span className="city-art" aria-hidden="true" style={{ '--city-characters': characters } as CSSProperties}>
    <svg className="city-art-route" viewBox="0 0 320 140" fill="none" preserveAspectRatio="none">
      <path d="M16 112 Q92 26 168 74 T304 30" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeDasharray="6 6" />
      <circle cx="16" cy="112" r="4.5" fill="currentColor" />
      <circle cx="304" cy="30" r="4.5" fill="currentColor" />
    </svg>
    <span className="city-art-name">{name}</span>
  </span>
}
