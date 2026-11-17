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

export const cityCopy: Record<Locale, { hint: string; unavailable: string }> = {
  en: { hint: 'Choose a suggested city or enter another city.', unavailable: 'City suggestions are unavailable. You can still enter a city.' },
  ru: { hint: 'Выберите город из списка или введите другой.', unavailable: 'Список городов недоступен. Вы можете ввести город вручную.' },
  kk: { hint: 'Тізімнен қаланы таңдаңыз немесе басқа қаланы енгізіңіз.', unavailable: 'Қалалар тізімі қолжетімсіз. Қаланы қолмен енгізуге болады.' },
}
