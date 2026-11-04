import { useId, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { request } from '../api'
import { cityCopy, cityLabel, normalizeCity } from '../cities'
import type { CityOption } from '../cities'
import { useI18n } from '../i18n'

interface CitySelectorProps {
  value: string
  onChange: (value: string) => void
  label?: string
  placeholder?: string
  hint?: string
  required?: boolean
  disabled?: boolean
  maxLength?: number
  name?: string
}

export function useCityOptions() {
  return useQuery({
    queryKey: ['cities'],
    queryFn: ({ signal }) => request<{ cities: CityOption[] }>('/cities', { signal }),
    meta: { private: false },
    staleTime: 24 * 60 * 60_000,
    retry: false,
  })
}

// A native datalist supplies browser keyboard/selection behavior while retaining
// manual entry for cities outside the suggested Kazakhstan vocabulary.
export function CitySelector({ value, onChange, label, placeholder, hint, required = false, disabled = false, maxLength = 100, name }: CitySelectorProps) {
  const { locale, t } = useI18n()
  const query = useCityOptions()
  const cities = query.data?.cities ?? []
  const id = useId()
  const [draft, setDraft] = useState<{ text: string; value: string } | null>(null)
  const description = hint ?? cityCopy[locale][query.isError ? 'unavailable' : 'hint']
  return <>
    <label className="field city-selector" htmlFor={id}>
      <span className="field-label" id={`${id}-label`}>{label ?? t('city')}{required && <span aria-hidden="true"> *</span>}</span>
      <input id={id} name={name} list={`${id}-options`} value={draft?.value === value ? draft.text : cityLabel(value, locale, cities)}
        autoComplete="address-level2" placeholder={placeholder ?? t('cityPlaceholder')} maxLength={maxLength}
        required={required} disabled={disabled} aria-labelledby={`${id}-label`} aria-describedby={description ? `${id}-hint` : undefined}
        onFocus={() => setDraft({ text: cityLabel(value, locale, cities), value })}
        onChange={(event) => { const normalized = normalizeCity(event.target.value, cities); setDraft({ text: event.target.value, value: normalized }); onChange(normalized) }}
        onBlur={(event) => { setDraft(null); const normalized = normalizeCity(event.currentTarget.value, cities); if (normalized !== value) onChange(normalized) }} />
      {description && <span className="field-hint" id={`${id}-hint`}>{description}</span>}
    </label>
    <datalist id={`${id}-options`}>
      {cities.map((city) => <option key={city.value} value={city.names[locale]} label={[city.value, ...city.aliases, ...Object.values(city.names)].filter((name, index, all) => name !== city.names[locale] && all.indexOf(name) === index).join(' · ')} />)}
    </datalist>
  </>
}
