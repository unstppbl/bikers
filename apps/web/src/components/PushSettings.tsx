import { useEffect, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { BellIcon as Bell, BellSlashIcon as BellOff, TrashIcon as Trash2 } from '@phosphor-icons/react'
import { Link } from 'react-router-dom'
import { ApiError, request } from '../api'
import { useI18n } from '../i18n'
import { useSession } from '../session'
import { useSessionTask } from '../useSessionTask'
import { ErrorNotice, Loading, useConfirmation, useOnline } from '../ui'
import { isInstalled } from '../pages/Install'

interface Subscription { id: string; created_at: string }
export function applicationServerKey(value: string) {
  const normalized = value.replaceAll('-', '+').replaceAll('_', '/')
  return Uint8Array.from(atob(normalized + '='.repeat((4 - normalized.length % 4) % 4)), (character) => character.charCodeAt(0))
}
export function pushCapability(): 'pushSecure' | 'pushUnsupported' | 'pushDenied' | 'pushInstall' | null {
  if (!window.isSecureContext) return 'pushSecure'
  if (!('serviceWorker' in navigator) || !('PushManager' in window) || !('Notification' in window)) return 'pushUnsupported'
  if (Notification.permission === 'denied') return 'pushDenied'
  if (/iPad|iPhone|iPod/.test(navigator.userAgent) && !isInstalled()) return 'pushInstall'
  return null
}
export function PushSettings() {
  const { user } = useSession()
  const { t, locale } = useI18n()
  const client = useQueryClient()
  const start = useSessionTask()
  const { confirm, dialog } = useConfirmation()
  const online = useOnline()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<unknown>()
  const storageKey = `bikerskz.push.${user?.id}`
  const [currentId, setCurrentId] = useState(() => { try { return localStorage.getItem(storageKey) ?? '' } catch { return '' } })
  const [permission, setPermission] = useState(pushCapability)
  useEffect(() => { const update = () => setPermission(pushCapability()); window.addEventListener('focus', update); return () => window.removeEventListener('focus', update) }, [])
  const settings = useQuery({ queryKey: ['push-settings', user?.id], queryFn: ({ signal }) => request<{ enabled: boolean; public_key: string }>('/me/push/settings', { signal }), meta: { private: true }, retry: false })
  const subscriptions = useQuery({ queryKey: ['push-subscriptions', user?.id], queryFn: ({ signal }) => request<{ subscriptions: Subscription[] }>('/me/push/subscriptions', { signal }), meta: { private: true }, retry: false })
  const enabled = !!currentId && subscriptions.data?.subscriptions.some((item) => item.id === currentId)
  function saveId(id: string) { setCurrentId(id); try { if (id) localStorage.setItem(storageKey, id); else localStorage.removeItem(storageKey) } catch { /* Device preference persistence is optional. */ } }
  async function enable() {
    const task = start(); if (!task.current()) { task.finish(); return }
    setBusy(true); setError(undefined)
    try {
      const unavailable = pushCapability(); if (unavailable) { setPermission(unavailable); return }
      if (!settings.data?.enabled) throw new ApiError(503, 'push_unavailable')
      const granted = await Notification.requestPermission()
      if (!task.current()) return
      if (granted !== 'granted') { setPermission(granted === 'denied' ? 'pushDenied' : null); return }
      const registration = await navigator.serviceWorker.getRegistration('/')
      if (!registration?.active) throw new ApiError(503, 'push_not_ready')
      let subscription = await registration.pushManager.getSubscription()
      if (!task.current()) return
      if (!subscription) subscription = await registration.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: applicationServerKey(settings.data.public_key) })
      if (!task.current()) return
      const result = await request<{ subscription: Subscription }>('/me/push/subscriptions', { method: 'POST', signal: task.signal, body: JSON.stringify(subscription.toJSON()) })
      if (!task.current()) return
      saveId(result.subscription.id); setPermission(null)
      await client.invalidateQueries({ queryKey: ['push-subscriptions', user?.id] })
    } catch (value) { if (task.current()) setError(value) }
    finally { if (task.current()) setBusy(false); task.finish() }
  }
  async function remove(id: string) {
    const task = start()
    if (!await confirm({ message: t('removeSubscriptionConfirm'), action: t('removeSubscription'), destructive: true }) || !task.current()) { task.finish(); return }
    setBusy(true); setError(undefined)
    try {
      await request(`/me/push/subscriptions/${encodeURIComponent(id)}`, { method: 'DELETE', signal: task.signal })
      if (!task.current()) return
      if (id === currentId) { saveId(''); const registration = await navigator.serviceWorker?.getRegistration('/'); if (task.current()) await (await registration?.pushManager.getSubscription())?.unsubscribe() }
      if (task.current()) await client.invalidateQueries({ queryKey: ['push-subscriptions', user?.id] })
    } catch (value) { if (task.current()) setError(value) }
    finally { if (task.current()) setBusy(false); task.finish() }
  }
  return <section className="surface settings-panel push-settings">{dialog}<div className="panel-title"><span className="panel-icon"><Bell size={23} /></span><h2>{t('pushTitle')}</h2></div><p>{t('pushText')}</p>{settings.isPending || subscriptions.isPending ? <Loading /> : settings.isError || subscriptions.isError ? <ErrorNotice error={settings.error ?? subscriptions.error} retry={() => { void settings.refetch(); void subscriptions.refetch() }} /> : <>{!settings.data.enabled ? <p className="inline-note">{t('pushUnavailable')}</p> : permission ? <p className="inline-note">{t(permission)}</p> : enabled ? <p className="success-message" role="status">{t('pushEnabled')}</p> : null}<div className="form-actions">{enabled ? <button className="button button-outline" disabled={busy || !online} onClick={() => void remove(currentId)}><BellOff size={16} />{t('pushDisable')}</button> : <button className="button button-outline" disabled={busy || !online || !!permission || !settings.data.enabled} onClick={() => void enable()}><Bell size={16} />{t('pushEnable')}</button>}<Link className="inline-link" to="/install">{t('installApp')}</Link></div>{subscriptions.data.subscriptions.length > 0 && <div className="push-subscriptions"><h3>{t('pushSubscriptions')}</h3>{subscriptions.data.subscriptions.map((item, index) => <div className="subscription-row" key={item.id}><span>{t('pushSubscription', { number: index + 1 })}<small>{new Date(item.created_at).toLocaleDateString(locale === 'en' ? 'en-GB' : locale === 'ru' ? 'ru-RU' : 'kk-KZ')}</small></span><button className="icon-button danger-text" aria-label={`${t('removeSubscription')} ${index + 1}`} disabled={busy || !online} onClick={() => void remove(item.id)}><Trash2 size={17} /></button></div>)}</div>}</>}{error != null && <ErrorNotice error={error} />}</section>
}
