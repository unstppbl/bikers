import { describe, expect, it } from 'vitest'
import { dateParts, dateRangeLabel, formatRideDateTime } from './ui'

// A fixed "now" keeps these tests independent of the real date.
const now = new Date('2026-11-19T06:00:00Z')

describe('written-out dates', () => {
  it.each([
    ['en', '5 November, 10:30', '5 November 2025, 10:30', '5 Nov 2025, 10:30', '2 January 2027, 09:00'],
    ['ru', '5 ноября, 10:30', '5 ноября 2025, 10:30', '5 нояб. 2025, 10:30', '2 января 2027, 09:00'],
    ['kk', '5 қараша, 10:30', '5 қараша 2025, 10:30', '5 қар 2025, 10:30', '2 қаңтар 2027, 09:00'],
  ] as const)('shows the year only outside the current year (%s)', (locale, current, older, olderShort, next) => {
    expect(formatRideDateTime('2026-11-05T05:30:00Z', 'Asia/Almaty', locale, false, now)).toBe(current)
    expect(formatRideDateTime('2025-11-05T05:30:00Z', 'Asia/Almaty', locale, false, now)).toBe(older)
    expect(formatRideDateTime('2025-11-05T05:30:00Z', 'Asia/Almaty', locale, true, now)).toBe(olderShort)
    expect(formatRideDateTime('2027-01-02T04:00:00Z', 'Asia/Almaty', locale, false, now)).toBe(next)
  })

  it('compares years in the time zone the date is shown in', () => {
    // 31 December 20:00 UTC is already New Year's Day in Almaty.
    expect(formatRideDateTime('2026-12-31T20:00:00Z', 'Asia/Almaty', 'en', false, new Date('2027-01-01T06:00:00Z'))).toBe('1 January, 01:00')
    expect(formatRideDateTime('2026-12-31T20:00:00Z', 'Europe/Berlin', 'en', false, new Date('2027-01-01T06:00:00Z'))).toBe('31 December 2026, 21:00')
    // Late on 31 December UTC, Almaty has reached the new year, so last year's date shows its year.
    expect(formatRideDateTime('2026-06-01T05:00:00Z', 'Asia/Almaty', 'en', false, new Date('2026-12-31T20:00:00Z'))).toBe('1 June 2026, 10:00')
  })

  it.each([
    ['en', 'Tuesday, 1 December', 'Friday, 15 January 2027', 'Jan'],
    ['ru', 'вторник, 1 декабря', 'пятница, 15 января 2027', 'янв'],
    ['kk', 'сейсенбі, 1 желтоқсан', 'жұма, 15 қаңтар 2027', 'қаң'],
  ] as const)('writes out ride dates in another year with their year and keeps the plate short (%s)', (locale, current, next, month) => {
    expect(dateParts('2026-12-01', locale, now).full).toBe(current)
    expect(dateParts('2027-01-15', locale, now)).toEqual({ day: '15', month, full: next })
  })

  it('keeps an unreadable ride date as it is', () => {
    expect(dateParts('soon', 'en', now)).toEqual({ day: '—', month: '', full: 'soon' })
  })
})

describe('date filter summary', () => {
  const june = new Date('2026-06-10T04:00:00Z')
  it.each([
    ['en', '2026-06-01', '2026-06-30', '1–30 Jun'],
    ['ru', '2026-06-01', '2026-06-30', '1–30 июн.'],
    ['kk', '2026-06-01', '2026-06-30', '1–30 мау'],
    ['en', '2026-06-28', '2026-07-03', '28 Jun – 3 Jul'],
    ['ru', '2026-06-28', '2026-07-03', '28 июн. – 3 июл.'],
    ['kk', '2026-06-28', '2026-07-03', '28 мау – 3 шіл'],
    ['en', '2026-12-28', '2027-01-03', '28 Dec – 3 Jan 2027'],
    ['ru', '2027-06-01', '2027-06-30', '1–30 июн. 2027'],
    ['en', '2026-06-12', '2026-06-12', '12 Jun'],
    ['en', '2026-06-01', '', '1 Jun –'],
    ['kk', '', '2026-06-30', '– 30 мау'],
    ['en', '2026-02-31', '', '2026-02-31'],
    ['en', 'soon', '2026-06-30', 'soon – 2026-06-30'],
  ] as const)('%s: %s to %s reads “%s”', (locale, from, to, label) => {
    expect(dateRangeLabel(from, to, locale, june)).toBe(label)
  })

  it('compares the year in Kazakhstan time', () => {
    // 31 December 20:00 UTC is already New Year's Day in Almaty.
    expect(dateRangeLabel('2027-01-02', '', 'en', new Date('2026-12-31T20:00:00Z'))).toBe('2 Jan –')
  })
})
