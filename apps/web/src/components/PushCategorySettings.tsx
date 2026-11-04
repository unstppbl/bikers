import { useState } from 'react'
import type { FormEvent } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { BellIcon as Bell, CheckIcon as Check } from '@phosphor-icons/react'
import { ApiError, request } from '../api'
import { useI18n } from '../i18n'
import { useSession } from '../session'
import { useSessionTask } from '../useSessionTask'
import { ErrorNotice, useOnline } from '../ui'
import type { PushCategories } from '../types'

const categoryKeys = ['ride_updates', 'participation', 'club_membership', 'ride_reminders'] as const
const labels = { ride_updates: 'pushCategoryRideUpdates', participation: 'pushCategoryParticipation', club_membership: 'pushCategoryClubs', ride_reminders: 'pushCategoryReminders' } as const

export function safePushCategories(value: unknown): PushCategories {
  if (!value || typeof value !== 'object') throw new ApiError(502, 'request_failed')
  const fields = value as Record<string, unknown>
  if (typeof fields.ride_updates !== 'boolean' || typeof fields.participation !== 'boolean' || typeof fields.club_membership !== 'boolean' || typeof fields.ride_reminders !== 'boolean') throw new ApiError(502, 'request_failed')
  return { ride_updates: fields.ride_updates, participation: fields.participation, club_membership: fields.club_membership, ride_reminders: fields.ride_reminders }
}

export function PushCategorySettings({ categories }: { categories: PushCategories }) {
  const { user } = useSession()
  const { t } = useI18n()
  const client = useQueryClient()
  const start = useSessionTask()
  const online = useOnline()
  const [form, setForm] = useState({ ...categories })
  const [busy, setBusy] = useState(false)
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState<unknown>()
  async function submit(event: FormEvent) {
    event.preventDefault()
    const task = start()
    if (!task.current()) { task.finish(); return }
    setBusy(true); setSaved(false); setError(undefined)
    try {
      const next = await request<{ push_categories: unknown }>('/me/preferences', { method: 'PATCH', signal: task.signal, body: JSON.stringify({ push_categories: form }) })
      const accepted = safePushCategories(next.push_categories)
      if (!task.current()) return
      setForm(accepted); setSaved(true)
      await client.invalidateQueries({ queryKey: ['preferences', user?.id] })
    } catch (value) { if (task.current()) setError(value) }
    finally { if (task.current()) setBusy(false); task.finish() }
  }
  return <form className="surface profile-form notification-preferences" onSubmit={(event) => void submit(event)}><div className="panel-title"><span className="panel-icon"><Bell size={22} /></span><h2>{t('pushCategoriesTitle')}</h2></div><p className="section-intro">{t('pushCategoriesHint')}</p><fieldset disabled={busy || !online} className="notification-options"><legend className="sr-only">{t('pushCategoriesTitle')}</legend>{categoryKeys.map((key) => <label className="notification-option" key={key}><input type="checkbox" checked={form[key]} onChange={(event) => { setSaved(false); setForm({ ...form, [key]: event.target.checked }) }} /><span>{t(labels[key])}</span></label>)}</fieldset>{error != null && <ErrorNotice error={error} />}{saved && <p className="success-message" role="status"><Check size={17} />{t('saved')}</p>}<div className="form-actions"><button type="submit" className="button button-primary" disabled={busy || !online}>{t(busy ? 'saving' : 'pushCategoriesSave')}<Check size={16} /></button></div></form>
}
