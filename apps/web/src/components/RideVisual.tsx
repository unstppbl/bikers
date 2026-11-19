import type { CSSProperties } from 'react'
import { formatInTimeZone } from 'date-fns-tz'
import { dateParts } from '../ui'
import type { Locale, Ride } from '../types'

// "You're going" only fits a ride that still lies ahead. A summary has only its local date,
// in Kazakhstan time unless the ride carries its own time zone.
export function goingLabel(ride: Ride, now = new Date()): 'youAreGoing' | 'approved' {
  const past = ride.starts_at ? new Date(ride.starts_at) <= now : ride.date < formatInTimeZone(now, ride.timezone ?? 'Asia/Almaty', 'yyyy-MM-dd')
  return ride.status === 'cancelled' || past ? 'approved' : 'youAreGoing'
}

// The date on a cream number plate is the ride's signature mark on cards and detail pages.
// It repeats the date that is also written out in full nearby, so it is hidden from assistive technology.
export function DatePlate({ date, locale, size = 'card' }: { date: string; locale: Locale; size?: 'card' | 'hero' }) {
  const parts = dateParts(date, locale)
  return <span className={`date-plate date-plate-${size}`} aria-hidden="true"><strong>{parts.day}</strong><span>{parts.month}</span></span>
}

// Rides without a photo use their city name as the artwork, sized so that it fills the card's width.
// A name of several words wraps at its spaces onto about two lines, so its size follows the longest
// word or three fifths of the name, whichever is longer.
export function CityArt({ city }: { city: string }) {
  const name = city.trim()
  const words = name.split(/\s+/)
  const characters = Math.max(4, ...words.map((word) => [...word].length), words.length > 1 ? Math.ceil([...name].length * 0.6) : 0)
  return <span className="city-art" aria-hidden="true" style={{ '--city-characters': characters } as CSSProperties}>{name}</span>
}
