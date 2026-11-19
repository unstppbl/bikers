import type { Locale } from './types'

export interface CityOption {
  value: string
  names: Record<Locale, string>
  aliases: string[]
  center?: { lat: number; lon: number }
}

function cityKey(value: string) { return value.toLowerCase().replace(/[^\p{L}\p{N}]/gu, '') }

function findCity(value: string, cities: readonly CityOption[]) {
  const key = cityKey(value)
  if (!key) return undefined
  return cities.find((city) => [city.value, ...Object.values(city.names), ...city.aliases].some((name) => cityKey(name) === key))
}

// The API accepts a normalized city (a catalogue value, or the trimmed text) of up to 160 UTF-8 bytes in every
// form and filter. An input's maxLength counts UTF-16 units, so the selector checks the normalized value instead.
const cityMaxBytes = 160
const encoder = new TextEncoder()

export function cityTooLong(normalized: string) {
  return encoder.encode(normalized).length > cityMaxBytes
}

export function normalizeCity(value: string, cities: readonly CityOption[]) {
  return findCity(value, cities)?.value ?? value.trim()
}

export function cityLabel(value: string, locale: Locale, cities: readonly CityOption[]) {
  return findCity(value, cities)?.names[locale] ?? value
}

// Use the catalogue's existing localized names and aliases for camera lookup too.
export function cityCenter(value: string, cities: readonly CityOption[]) {
  const center = findCity(value, cities)?.center
  if (!center || !Number.isFinite(center.lat) || !Number.isFinite(center.lon) || Math.abs(center.lat) > 90 || Math.abs(center.lon) > 180) return undefined
  return { lat: center.lat, lon: center.lon }
}

export const cityCopy: Record<Locale, { hint: string; unavailable: string; tooLong: string }> = {
  en: { hint: 'Choose a suggested city or enter another city.', unavailable: 'City suggestions are unavailable. You can still enter a city.', tooLong: 'This city name is too long. Shorten it to save.' },
  ru: { hint: 'Выберите город из списка или введите другой.', unavailable: 'Список городов недоступен. Вы можете ввести город вручную.', tooLong: 'Название города слишком длинное. Сократите его, чтобы сохранить.' },
  kk: { hint: 'Тізімнен қаланы таңдаңыз немесе басқа қаланы енгізіңіз.', unavailable: 'Қалалар тізімі қолжетімсіз. Қаланы қолмен енгізуге болады.', tooLong: 'Қала атауы тым ұзын. Сақтау үшін оны қысқартыңыз.' },
}
