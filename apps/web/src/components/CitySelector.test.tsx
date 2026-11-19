import { useState } from 'react'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { I18nProvider, useI18n } from '../i18n'
import { cityLabel, normalizeCity } from '../cities'
import type { CityOption } from '../cities'
import { CitySelector } from './CitySelector'

const cities: CityOption[] = [
  { value: 'Almaty', names: { en: 'Almaty', ru: 'Алматы', kk: 'Алматы' }, aliases: ['Alma-Ata', 'Алма-Ата'] },
  { value: 'Astana', names: { en: 'Astana', ru: 'Астана', kk: 'Астана' }, aliases: ['Nur-Sultan', 'Нур-Султан', 'Нұр-Сұлтан'] },
  { value: 'Oskemen', names: { en: 'Oskemen', ru: 'Усть-Каменогорск', kk: 'Өскемен' }, aliases: ['Ust-Kamenogorsk'] },
]

function Harness({ initial = 'Almaty' }: { initial?: string }) {
  const [value, setValue] = useState(initial)
  const { setLocale } = useI18n()
  return <>
    <CitySelector value={value} onChange={setValue} required />
    <output aria-label="Saved city">{value}</output>
    <button onClick={() => setValue('Astana')}>Change city</button>
    <button onClick={() => setLocale('kk')}>Kazakh</button>
  </>
}

function renderSelector(initial?: string) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
  const view = render(<QueryClientProvider client={client}><I18nProvider><Harness initial={initial} /></I18nProvider></QueryClientProvider>)
  return { client, ...view }
}

afterEach(() => { vi.unstubAllGlobals() })

describe('city values and names', () => {
  it('normalizes known names and aliases without changing custom cities', () => {
    for (const value of ['almaty', ' Алматы ', 'ALMA ATA']) expect(normalizeCity(value, cities)).toBe('Almaty')
    expect(normalizeCity('Нұр-Сұлтан', cities)).toBe('Astana')
    expect(normalizeCity('Өскемен', cities)).toBe('Oskemen')
    expect(normalizeCity('  Bishkek  ', cities)).toBe('Bishkek')
    expect(normalizeCity('Almaty district', cities)).toBe('Almaty district')
    expect(normalizeCity(' ', cities)).toBe('')
    expect(cityLabel('Ust-Kamenogorsk', 'kk', cities)).toBe('Өскемен')
    expect(cityLabel('Bishkek', 'ru', cities)).toBe('Bishkek')
  })
})

describe('city selector', () => {
  it('offers localized native suggestions from one public cached catalogue and saves stable values', async () => {
    localStorage.setItem('bikerskz.locale', 'ru')
    const fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify({ cities })))
    vi.stubGlobal('fetch', fetch)
    const { client } = renderSelector()
    const input = screen.getByRole('combobox', { name: 'Город' })
    await waitFor(() => expect(input).toHaveValue('Алматы'))
    expect(input).toBeRequired()
    const list = document.getElementById(input.getAttribute('list') ?? '')
    expect(list?.tagName).toBe('DATALIST')
    expect(list?.querySelector('option[value="Астана"]')).toBeInTheDocument()
    expect(list?.querySelector('option[value="Усть-Каменогорск"]')).toHaveAttribute('label', expect.stringContaining('Ust-Kamenogorsk'))
    fireEvent.focus(input)
    fireEvent.change(input, { target: { value: 'Нур-Султан' } })
    expect(input).toHaveValue('Нур-Султан')
    expect(screen.getByLabelText('Saved city')).toHaveTextContent('Astana')
    fireEvent.blur(input)
    expect(input).toHaveValue('Астана')
    expect(fetch).toHaveBeenCalledTimes(1)
    expect(fetch.mock.calls[0][0]).toBe('/api/v1/cities')
    expect(client.getQueryCache().find({ queryKey: ['cities'] })?.meta?.private).toBe(false)
  })

  // The API accepts a trimmed city of up to 160 UTF-8 bytes; Cyrillic letters take two bytes each.
  it('flags a city longer than the API accepts, counted in UTF-8 bytes after trimming, without rewriting it', () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({ cities }))))
    renderSelector()
    const input = screen.getByRole('combobox', { name: 'City' }) as HTMLInputElement
    expect(input).not.toHaveAttribute('maxlength')
    fireEvent.focus(input)
    for (const fits of [' ' + 'a'.repeat(160) + ' ', ' ' + 'Ж'.repeat(80)]) {
      fireEvent.change(input, { target: { value: fits } })
      expect(input).toHaveValue(fits)
      expect(screen.getByLabelText('Saved city').textContent).toBe(fits.trim())
      expect(input).not.toHaveAttribute('aria-invalid')
      expect(input.validity.valid).toBe(true)
      expect(screen.queryByText('This city name is too long. Shorten it to save.')).not.toBeInTheDocument()
    }
    const tooLong = 'ЖЖ' + 'x' + 'Ж'.repeat(78)
    fireEvent.change(input, { target: { value: tooLong } })
    expect(input).toHaveValue(tooLong)
    expect(input).toHaveAttribute('aria-invalid', 'true')
    expect(input.validity.customError).toBe(true)
    expect(input).toHaveAccessibleDescription(expect.stringContaining('This city name is too long. Shorten it to save.'))
    fireEvent.change(input, { target: { value: 'Ж'.repeat(80) } })
    expect(input).not.toHaveAttribute('aria-invalid')
    expect(input.validity.valid).toBe(true)
  })

  // A parent can pass a raw value, for example from the URL; the API trims it before counting.
  it('checks a raw value from the parent after normalizing it', () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({ cities }))))
    renderSelector(' ' + 'Ж'.repeat(80) + ' ')
    const input = screen.getByRole('combobox', { name: 'City' }) as HTMLInputElement
    expect(input).not.toHaveAttribute('aria-invalid')
    expect(input.validity.valid).toBe(true)
  })

  it('keeps manual entry usable if the suggested catalogue is unavailable', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('offline')))
    renderSelector('Bishkek')
    const input = screen.getByRole('combobox', { name: 'City' })
    expect(await screen.findByText('City suggestions are unavailable. You can still enter a city.')).toBeInTheDocument()
    fireEvent.focus(input)
    fireEvent.change(input, { target: { value: '  Cholpon-Ata  ' } })
    fireEvent.blur(input)
    expect(input).toHaveValue('Cholpon-Ata')
    expect(screen.getByLabelText('Saved city')).toHaveTextContent('Cholpon-Ata')
  })

  it('reflects changes to the selected city and language without remounting the form', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({ cities }))))
    renderSelector('Oskemen')
    const input = screen.getByRole('combobox', { name: 'City' })
    await waitFor(() => expect(input).toHaveValue('Oskemen'))
    fireEvent.click(screen.getByRole('button', { name: 'Kazakh' }))
    await waitFor(() => expect(input).toHaveValue('Өскемен'))
    fireEvent.click(screen.getByRole('button', { name: 'Change city' }))
    expect(input).toHaveValue('Астана')
    expect(screen.getByLabelText('Saved city')).toHaveTextContent('Astana')
    fireEvent.focus(input)
    fireEvent.change(input, { target: { value: 'Issyk' } })
    fireEvent.click(screen.getByRole('button', { name: 'Change city' }))
    expect(input).toHaveValue('Астана')
  })
})
