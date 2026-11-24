import { formatInTimeZone, fromZonedTime } from 'date-fns-tz'

export const rideTimezone = 'Asia/Almaty'

export function localRideTime(value?: string, timezone = rideTimezone) {
  return value ? formatInTimeZone(value, timezone, "yyyy-MM-dd'T'HH:mm") : ''
}

export function rideTimeToInstant(value: string, timezone = rideTimezone) {
  return fromZonedTime(value, timezone)
}
