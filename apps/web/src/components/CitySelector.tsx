import { useId, useLayoutEffect, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { request } from '../api'
import { cityCopy, cityLabel, cityTooLong, normalizeCity } from '../cities'
import type { CityOption } from '../cities'
import { useI18n } from '../i18n'

interface CitySelectorProps {
  value: string
  onChange: (value: string) => void
  label?: string
  // None by default. Where the label is hidden, the placeholder is the visible label, so it repeats the label.
  placeholder?: string
  hint?: string
  required?: boolean
  disabled?: boolean
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
export function CitySelector({ value, onChange, label, placeholder, hint, required = false, disabled = false, name }: CitySelectorProps) {
  const { locale, t } = useI18n()
  const query = useCityOptions()
  const cities = query.data?.cities ?? []
  const id = useId()
  const [draft, setDraft] = useState<{ text: string; value: string } | null>(null)
  const description = hint ?? cityCopy[locale][query.isError ? 'unavailable' : 'hint']
  // The text is never rewritten while the rider types; an over-long city is flagged instead, and blocks forms that use
  // native validation. Forms with noValidate still get the visible message, and the API rejects the value.
  // A parent may pass a raw value (from the URL, for example), so the check normalizes it as the API does.
  const tooLong = cityTooLong(normalizeCity(value, cities))
  const error = tooLong ? cityCopy[locale].tooLong : ''
  const input = useRef<HTMLInputElement>(null)
  useLayoutEffect(() => { input.current?.setCustomValidity(error) }, [error])
  const describedBy = [error && `${id}-error`, description && `${id}-hint`].filter(Boolean).join(' ')
  return <>
    <label className="field city-selector" htmlFor={id}>
      <span className="field-label" id={`${id}-label`}>{label ?? t('city')}{required && <span aria-hidden="true"> *</span>}</span>
      <input ref={input} id={id} name={name} list={`${id}-options`} value={draft?.value === value ? draft.text : cityLabel(value, locale, cities)}
        autoComplete="address-level2" placeholder={placeholder}
        required={required} disabled={disabled} aria-labelledby={`${id}-label`} aria-describedby={describedBy || undefined} aria-invalid={tooLong || undefined}
        onFocus={() => setDraft({ text: cityLabel(value, locale, cities), value })}
        onChange={(event) => { const normalized = normalizeCity(event.target.value, cities); setDraft({ text: event.target.value, value: normalized }); onChange(normalized) }}
        onBlur={(event) => { setDraft(null); const normalized = normalizeCity(event.currentTarget.value, cities); if (normalized !== value) onChange(normalized) }} />
      {error && <span className="field-hint field-error" id={`${id}-error`}>{error}</span>}
      {description && <span className="field-hint" id={`${id}-hint`}>{description}</span>}
    </label>
    <datalist id={`${id}-options`}>
      {cities.map((city) => <option key={city.value} value={city.names[locale]} label={[city.value, ...city.aliases, ...Object.values(city.names)].filter((name, index, all) => name !== city.names[locale] && all.indexOf(name) === index).join(' · ')} />)}
    </datalist>
  </>
}
