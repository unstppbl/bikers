import { useCallback, useEffect, useId, useRef, useState } from 'react'
import type { InputHTMLAttributes, ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { WarningCircleIcon as AlertCircle, ArrowLeftIcon as ArrowLeft, ArrowRightIcon as ArrowRight, CheckIcon as Check, CopyIcon as Copy, PathIcon as Route, ShieldCheckIcon as ShieldCheck } from '@phosphor-icons/react'
import { Link } from 'react-router-dom'
import { formatInTimeZone } from 'date-fns-tz'
import { enGB, kk, ru } from 'date-fns/locale'
import { useI18n, errorText } from './i18n'
import type { Locale } from './types'

export function Loading({ label }: { label?: string }) {
  const { t } = useI18n()
  return <div className="page-loading" role="status"><span className="loading-skeleton" aria-hidden="true"><span /><span /><span /></span><span>{label ?? t('loading')}</span></div>
}

export function ErrorNotice({ error, text, retry }: { error?: unknown; text?: string; retry?: () => void }) {
  const { t } = useI18n()
  return <div className="error-banner" role="alert"><AlertCircle size={18} /><span>{text ?? errorText(error, t)}</span>{retry && <button className="text-button" onClick={retry}>{t('retry')}</button>}</div>
}

export function EmptyState({ title, text, action }: { title: string; text: string; action?: ReactNode }) {
  return <div className="empty-state"><div className="empty-icon"><Route size={32} strokeWidth={1.5} /></div><h3>{title}</h3><p>{text}</p>{action}</div>
}

export function Field({ label, hint, children, ...props }: InputHTMLAttributes<HTMLInputElement> & { label: string; hint?: string; children?: ReactNode }) {
  return <label className="field"><span className="field-label">{label}{props.required && <span aria-hidden="true"> *</span>}</span>{children ?? <input {...props} />}{hint && <span className="field-hint">{hint}</span>}</label>
}

export function TextArea({ label, hint, value, onChange, required = false, maxLength = 1000, placeholder, rows = 4 }: {
  label: string; hint?: string; value: string; onChange: (value: string) => void; required?: boolean; maxLength?: number; placeholder?: string; rows?: number
}) {
  return <label className="field"><span className="field-label">{label}{required && <span aria-hidden="true"> *</span>}</span><textarea value={value} onChange={(event) => onChange(event.target.value)} required={required} maxLength={maxLength} placeholder={placeholder} rows={rows} />{hint && <span className="field-hint">{hint}</span>}</label>
}

export function PrivacyNote() {
  const { t } = useI18n()
  return <div className="privacy-note"><ShieldCheck size={20} /><div><strong>{t('privacyLabel')}</strong><p>{t('privacyText')}</p></div></div>
}

export function CreateRideLink({ className = 'button button-primary' }: { className?: string }) {
  const { t } = useI18n()
  return <Link className={className} to="/rides/new">{t('newRide')}<ArrowRight size={17} /></Link>
}

export function useOnline() {
  const [online, setOnline] = useState(navigator.onLine)
  useEffect(() => {
    const update = () => setOnline(navigator.onLine)
    window.addEventListener('online', update)
    window.addEventListener('offline', update)
    return () => { window.removeEventListener('online', update); window.removeEventListener('offline', update) }
  }, [])
  return online
}

// Written-out dates leave out the year while it is the current one. Older records and rides in another year show it.
const currentYear = (timezone: string, now: Date) => formatInTimeZone(now, timezone, 'yyyy')

// The plate's day and month stay short. A ride date is a calendar day without a time zone, so its year is compared
// with the current year in Kazakhstan time.
export function dateParts(date: string, locale: Locale, now = new Date()) {
  const value = new Date(`${date}T12:00:00Z`)
  if (Number.isNaN(value.getTime())) return { day: '—', month: '', full: date }
  const options = { locale: dateLocale(locale) }
  const sameYear = formatInTimeZone(value, 'UTC', 'yyyy') === currentYear('Asia/Almaty', now)
  return {
    day: formatInTimeZone(value, 'UTC', 'dd', options),
    month: formatInTimeZone(value, 'UTC', 'MMM', options).replace('.', ''),
    full: formatInTimeZone(value, 'UTC', sameYear ? 'EEEE, d MMMM' : 'EEEE, d MMMM yyyy', options),
  }
}

const dateLocale = (locale: Locale) => ({ en: enGB, ru, kk })[locale]

export function formatRideDateTime(value: string, timezone: string, locale: Locale, short = false, now = new Date()) {
  const day = short ? 'd MMM' : 'd MMMM'
  const sameYear = formatInTimeZone(value, timezone, 'yyyy') === currentYear(timezone, now)
  return formatInTimeZone(value, timezone, sameYear ? `${day}, HH:mm` : `${day} yyyy, HH:mm`, { locale: dateLocale(locale) })
}

export function formatCalendarDate(value: string, locale: Locale) {
  return formatInTimeZone(`${value}T12:00:00Z`, 'UTC', 'd MMMM yyyy', { locale: dateLocale(locale) })
}

// The date filter's summary sits beside the city field on a phone, so it stays short: "1–30 Jun",
// "28 Jun – 3 Jul", "1 Jun –". Like a ride date, a calendar day shows its year only outside the current year
// in Kazakhstan time. A date in the URL that is not a calendar day is shown as typed.
export function dateRangeLabel(from: string, to: string, locale: Locale, now = new Date()) {
  const calendarDay = (value: string) => { const date = new Date(`${value}T12:00:00Z`); return /^\d{4}-\d{2}-\d{2}$/.test(value) && !Number.isNaN(date.getTime()) && date.toISOString().startsWith(value) }
  if ([from, to].some((value) => value && !calendarDay(value))) return [from, to].filter(Boolean).join(' – ')
  const year = currentYear('Asia/Almaty', now)
  const format = (value: string, pattern: string) => formatInTimeZone(`${value}T12:00:00Z`, 'UTC', pattern, { locale: dateLocale(locale) })
  const day = (value: string) => format(value, value.startsWith(year) ? 'd MMM' : 'd MMM yyyy')
  if (!from || !to) return from ? `${day(from)} –` : `– ${day(to)}`
  if (from === to) return day(from)
  if (from.slice(0, 7) === to.slice(0, 7)) return `${format(from, 'd')}–${day(to)}`
  return `${day(from)} – ${day(to)}`
}

export function initials(name: string) {
  return name.trim().split(/\s+/).slice(0, 2).map((part) => part[0]).join('').toLocaleUpperCase() || '↗'
}

export function validNext(value: string | null) {
  return value?.startsWith('/') && !value.startsWith('//') && !value.includes('\\') && ![...value].some((character) => character.charCodeAt(0) < 32) && !value.startsWith('/login') ? value : '/'
}

export function Pagination({ page, next, previous, hasNext, busy = false }: { page: number; next: () => void; previous: () => void; hasNext: boolean; busy?: boolean }) {
  const { t } = useI18n()
  if (!hasNext && page === 0) return null
  return <nav className="pagination" aria-label={t('pageNumber', { page: page + 1 })}><button type="button" className="button button-outline" onClick={previous} disabled={page === 0 || busy}><ArrowLeft size={16} />{t('previousPage')}</button><span>{t('pageNumber', { page: page + 1 })}</span><button type="button" className="button button-outline" onClick={next} disabled={!hasNext || busy}>{t('nextPage')}<ArrowRight size={16} /></button></nav>
}

export function ShareLink({ path }: { path: string }) {
  const { t } = useI18n()
  const [result, setResult] = useState<'idle' | 'copied' | 'failed'>('idle')
  const url = new URL(path, window.location.origin).href
  async function copy() {
    try { await navigator.clipboard.writeText(url); setResult('copied') } catch { setResult('failed') }
  }
  return <div className="share-link"><button className="button button-outline" type="button" onClick={() => void copy()}>{result === 'copied' ? <Check size={16} /> : <Copy size={16} />}{t('copyLink')}</button>{result === 'copied' && <span className="success-message" role="status">{t('linkCopied')}</span>}{result === 'failed' && <label className="field"><span className="field-hint" role="status">{t('copyFailed')}</span><input aria-label={t('shareLink')} value={url} readOnly onFocus={(event) => event.target.select()} /></label>}</div>
}

export function useDebouncedValue(value: string, delay = 250) {
  const [debounced, setDebounced] = useState(value)
  useEffect(() => { const timer = window.setTimeout(() => setDebounced(value), delay); return () => window.clearTimeout(timer) }, [value, delay])
  return debounced
}

interface Confirmation { message: string; action: string; destructive?: boolean }

export function useConfirmation() {
  const [request, setRequest] = useState<Confirmation | null>(null)
  const resolveRef = useRef<((confirmed: boolean) => void) | undefined>(undefined)
  const confirm = useCallback((options: Confirmation) => new Promise<boolean>((resolve) => {
    resolveRef.current?.(false)
    resolveRef.current = resolve
    setRequest(options)
  }), [])
  const finish = useCallback((result: boolean) => { resolveRef.current?.(result); resolveRef.current = undefined; setRequest(null) }, [])
  useEffect(() => () => { resolveRef.current?.(false) }, [])
  return { confirm, dialog: request ? <ConfirmationDialog request={request} finish={finish} /> : null }
}

function ConfirmationDialog({ request, finish }: { request: Confirmation; finish: (result: boolean) => void }) {
  const { t } = useI18n()
  const id = useId()
  const panelRef = useRef<HTMLDivElement>(null)
  const cancelRef = useRef<HTMLButtonElement>(null)
  useEffect(() => {
    const previous = document.activeElement instanceof HTMLElement ? document.activeElement : null
    const root = document.getElementById('root')
    const wasInert = root?.inert ?? false
    const previousHidden = root?.getAttribute('aria-hidden')
    const previousOverflow = document.body.style.overflow
    if (root) { root.inert = true; root.setAttribute('aria-hidden', 'true') }
    document.body.style.overflow = 'hidden'
    cancelRef.current?.focus()
    function keydown(event: KeyboardEvent) {
      if (event.key === 'Escape') { event.preventDefault(); finish(false); return }
      if (event.key !== 'Tab') return
      const buttons = panelRef.current?.querySelectorAll<HTMLButtonElement>('button:not(:disabled)')
      if (!buttons?.length) return
      const first = buttons[0], last = buttons[buttons.length - 1]
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus() }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
    }
    document.addEventListener('keydown', keydown)
    return () => {
      document.removeEventListener('keydown', keydown)
      document.body.style.overflow = previousOverflow
      if (root) { root.inert = wasInert; if (previousHidden === null || previousHidden === undefined) root.removeAttribute('aria-hidden'); else root.setAttribute('aria-hidden', previousHidden) }
      if (previous?.isConnected) previous.focus()
    }
  }, [finish])
  return createPortal(<div className="confirmation-backdrop" onMouseDown={(event) => { if (event.target === event.currentTarget) finish(false) }}><div className="confirmation-dialog surface" ref={panelRef} role="dialog" aria-modal="true" aria-labelledby={`${id}-title`} aria-describedby={`${id}-message`}><span className="confirmation-icon"><ShieldCheck size={25} strokeWidth={1.5} /></span><h2 id={`${id}-title`}>{request.action}</h2><p id={`${id}-message`}>{request.message}</p><div className="confirmation-actions"><button ref={cancelRef} className="button button-outline" onClick={() => finish(false)}>{t('cancel')}</button><button className={`button ${request.destructive ? 'button-danger' : 'button-primary'}`} onClick={() => finish(true)}>{request.action}</button></div></div></div>, document.body)
}
