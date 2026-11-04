import { useState } from 'react'
import type { FormEvent } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft, ArrowUpRight, Check, Globe, Languages } from 'lucide-react'
import { Link } from 'react-router-dom'
import { ApiError, request } from '../api'
import { useI18n } from '../i18n'
import { useSession } from '../session'
import { useSessionTask } from '../useSessionTask'
import { ErrorNotice, Field, Loading, useOnline } from '../ui'
import { PushSettings } from '../components/PushSettings'
import { BlockedUsersPanel } from '../components/Trust'
import type { Locale } from '../types'

export const countries = ['KZ', 'KG', 'UZ', 'RU', 'ZZ'] as const
export const currencies = ['KZT', 'KGS', 'UZS', 'RUB', 'USD', 'EUR'] as const
export interface Preferences { country_code: typeof countries[number]; currency_code: typeof currencies[number]; locale: Locale }
export function safePreferences(value: unknown): Preferences {
  if (!value || typeof value !== 'object' || !('country_code' in value) || !countries.some((code) => code === value.country_code) || !('currency_code' in value) || !currencies.some((code) => code === value.currency_code) || !('locale' in value) || !['en', 'ru', 'kk'].includes(String(value.locale))) throw new ApiError(502, 'request_failed')
  return { country_code: value.country_code as Preferences['country_code'], currency_code: value.currency_code as Preferences['currency_code'], locale: value.locale as Locale }
}

export function SettingsPage() {
  const { user } = useSession()
  const { t } = useI18n()
  const query = useQuery({ queryKey: ['preferences', user?.id], queryFn: async ({ signal }) => safePreferences(await request('/me/preferences', { signal })), enabled: !!user, meta: { private: true }, retry: false })
  return <div className="page-content narrow-page"><Link className="back-link" to="/"><ArrowLeft size={17} />{t('back')}</Link><div className="page-heading"><span className="eyebrow">{t('settings')}</span><h1>{t('preferencesTitle')}</h1><p>{t('preferencesText')}</p></div>{query.isPending ? <Loading /> : query.isError ? <ErrorNotice error={query.error} retry={() => void query.refetch()} /> : <PreferencesForm key={user?.id} preferences={query.data} />}<PushSettings key={`push:${user?.id}`} /><BlockedUsersPanel key={`blocks:${user?.id}`} /><Link className="surface settings-link" to="/install"><span><Globe size={20} />{t('installApp')}</span><ArrowUpRight size={18} /></Link></div>
}

function PreferencesForm({ preferences }: { preferences: Preferences }) {
  const { user } = useSession()
  const { t, locale, setLocale } = useI18n()
  const queryClient = useQueryClient()
  const start = useSessionTask()
  const online = useOnline()
  const [form, setForm] = useState<Preferences>({ ...preferences, locale })
  const [busy, setBusy] = useState(false)
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState<unknown>()
  async function submit(event: FormEvent) {
    event.preventDefault()
    const task = start()
    if (!task.current()) { task.finish(); return }
    setBusy(true); setSaved(false); setError(undefined)
    try {
      const next = safePreferences(await request('/me/preferences', { method: 'PATCH', signal: task.signal, body: JSON.stringify(form) }))
      if (!task.current()) return
      queryClient.setQueryDefaults(['preferences', user?.id], { meta: { private: true } })
      queryClient.setQueryData(['preferences', user?.id], next)
      setLocale(next.locale); setSaved(true)
    } catch (value) { if (task.current()) setError(value) }
    finally { if (task.current()) setBusy(false); task.finish() }
  }
  return <form className="surface profile-form preferences-form" onSubmit={(event) => void submit(event)}><div className="panel-title"><span className="panel-icon"><Languages size={23} /></span><h2>{t('settings')}</h2></div><Field label={t('preferredCountry')}><select value={form.country_code} disabled={busy} onChange={(event) => { setSaved(false); setForm({ ...form, country_code: event.target.value as Preferences['country_code'] }) }}>{countries.map((code) => <option key={code} value={code}>{t(`country${code}`)}</option>)}</select></Field><Field label={t('preferredCurrency')}><select value={form.currency_code} disabled={busy} onChange={(event) => { setSaved(false); setForm({ ...form, currency_code: event.target.value as Preferences['currency_code'] }) }}>{currencies.map((code) => <option key={code} value={code}>{code} · {t(`currency${code}`)}</option>)}</select></Field><Field label={t('preferenceLanguage')}><select value={form.locale} disabled={busy} onChange={(event) => { setSaved(false); setForm({ ...form, locale: event.target.value as Locale }) }}><option value="en">English</option><option value="ru">Русский</option><option value="kk">Қазақша</option></select></Field><p className="section-intro">{t('preferencesHint')}</p>{error != null && <ErrorNotice error={error} />}{saved && <p className="success-message" role="status"><Check size={17} />{t('saved')}</p>}<div className="form-actions"><button className="button button-primary" type="submit" disabled={busy || !online}>{t(busy ? 'saving' : 'preferencesSave')}<Check size={16} /></button></div></form>
}
