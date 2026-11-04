import { useCallback, useEffect, useId, useRef, useState } from 'react'
import type { InputHTMLAttributes, ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { AlertCircle, ArrowLeft, ArrowRight, Check, Copy, LoaderCircle, Route, ShieldCheck } from 'lucide-react'
import { Link } from 'react-router-dom'
import { formatInTimeZone } from 'date-fns-tz'
import { enGB, kk, ru } from 'date-fns/locale'
import { useI18n, errorText } from './i18n'
import type { Locale } from './types'

export function Loading({ label }: { label?: string }) {
  const { t } = useI18n()
  return <div className="page-loading" role="status"><LoaderCircle className="spin" size={24} /><span>{label ?? t('loading')}</span></div>
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

export function dateParts(date: string, locale: Locale) {
  const value = new Date(`${date}T12:00:00Z`)
  if (Number.isNaN(value.getTime())) return { day: '—', month: '', full: date }
  return {
    day: formatInTimeZone(value, 'UTC', 'dd', { locale: dateLocale(locale) }),
    month: formatInTimeZone(value, 'UTC', 'MMM', { locale: dateLocale(locale) }).replace('.', ''),
    full: formatInTimeZone(value, 'UTC', 'EEEE, d MMMM', { locale: dateLocale(locale) }),
  }
}

const dateLocale = (locale: Locale) => ({ en: enGB, ru, kk })[locale]

export function formatRideDateTime(value: string, timezone: string, locale: Locale, short = false) {
  return formatInTimeZone(value, timezone, short ? 'd MMM, HH:mm' : 'd MMMM, HH:mm', { locale: dateLocale(locale) })
}

export function formatCalendarDate(value: string, locale: Locale) {
  return formatInTimeZone(`${value}T12:00:00Z`, 'UTC', 'd MMMM yyyy', { locale: dateLocale(locale) })
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
  return <nav className="pagination" aria-label={t('pageNumber', { page: page + 1 })}><button className="button button-outline" onClick={previous} disabled={page === 0 || busy}><ArrowLeft size={16} />{t('previousPage')}</button><span>{t('pageNumber', { page: page + 1 })}</span><button className="button button-outline" onClick={next} disabled={!hasNext || busy}>{t('nextPage')}<ArrowRight size={16} /></button></nav>
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
