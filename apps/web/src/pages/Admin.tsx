import { AdminPlacesQueue } from '../components/places/AdminPlacesQueue'
import { useState } from 'react'
import type { FormEvent } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowLeftIcon as ArrowLeft, CheckIcon as Check, ShieldCheckIcon as ShieldCheck } from '@phosphor-icons/react'
import { Link, useSearchParams } from 'react-router-dom'
import { request, safeExternalUrl } from '../api'
import { useI18n } from '../i18n'
import { useSession } from '../session'
import { useSessionTask } from '../useSessionTask'
import { ErrorNotice, Field, Loading, Pagination, TextArea, useConfirmation, useOnline } from '../ui'
import { useTrust } from '../components/Trust'
import { resetTrustQueries } from '../trust-cache'
import type { ReportTarget } from '../types'

interface Report { id: string; reporter_id: string; target_type: ReportTarget; target_id: string; reason: string; details: string; status: string; version: number; created_at: string }
interface Target { type: ReportTarget; id: string; author_id?: string; scope_id?: string; text?: string; display_name?: string; city?: string; date?: string; status?: string; media_url?: string }
interface Audit { id: string; action: string; target_type: ReportTarget | 'report'; target_id: string; reason: string; created_at: string; source: string }
const statuses = ['open', 'reviewing', 'resolved', 'dismissed'] as const
const actions = ['hide_content', 'restore_content', 'restrict_account', 'restore_account', 'assign_badge', 'remove_badge'] as const
const auditActionLabels = { hide_content: 'action_hide_content', restore_content: 'action_restore_content', restrict_account: 'action_restrict_account', restore_account: 'action_restore_account', assign_badge: 'action_assign_badge', remove_badge: 'action_remove_badge', review_content: 'auditReviewContent', grant_admin: 'auditGrantAdmin', revoke_admin: 'auditRevokeAdmin', report_open: 'auditReportOpen', report_reviewing: 'auditReportReviewing', report_resolved: 'auditReportResolved', report_dismissed: 'auditReportDismissed', update_report: 'auditReportUpdated' } as const
const auditTargetLabels = { user: 'target_user', ride: 'target_ride', club: 'target_club', message: 'target_message', bike: 'target_bike', bike_photo: 'target_bike_photo', club_image: 'target_club_image', avatar: 'target_avatar', ride_photo: 'target_ride_photo', ride_video: 'target_ride_video', announcement: 'target_announcement', route: 'target_route', route_condition: 'target_route_condition', report: 'target_report' } as const
const targets: ReportTarget[] = ['user', 'ride', 'club', 'message', 'bike', 'bike_photo', 'club_image', 'avatar', 'ride_photo', 'ride_video', 'announcement', 'route', 'route_condition']
export function AdminPage() {
  const { t } = useI18n()
  const trust = useTrust()
  if (trust.isPending) return <Loading />
  if (trust.isError) return <div className="page-content"><ErrorNotice error={trust.error} retry={() => void trust.refetch()} /></div>
  if (!trust.data.is_admin) return <div className="page-content"><ErrorNotice text={t('noAccess')} /></div>
  return <AdminWorkspace />
}
function AdminWorkspace() {
  const { t } = useI18n()
  const { user } = useSession()
  const [tab, setTab] = useState<'reports' | 'audit' | 'places'>('reports')
  const [search] = useSearchParams()
  const targetUserId = search.get('user') ?? ''
  const target: Target | undefined = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(targetUserId) ? { type: 'user', id: targetUserId } : undefined
  return <div className="page-content admin-page"><Link className="back-link" to="/"><ArrowLeft size={17} />{t('back')}</Link><div className="page-heading"><span className="eyebrow">{t('admin')}</span><h1>{t('adminTitle')}</h1></div><nav className="collection-tabs"><button className={tab === 'reports' ? 'active' : ''} onClick={() => setTab('reports')}>{t('adminReports')}</button><button className={tab === 'audit' ? 'active' : ''} onClick={() => setTab('audit')}>{t('adminAudit')}</button><button className={tab === 'places' ? 'active' : ''} onClick={() => setTab('places')}>{t('placesTitle')}</button></nav>{tab === 'reports' ? <ReportQueue key={`reports:${user?.id}`} /> : tab === 'places' ? <AdminPlacesQueue key={user?.id} /> : <AuditHistory key={`audit:${user?.id}`} />}{tab !== 'places' && <AdminActionForm key={`actions:${user?.id}:${target?.id ?? ''}`} target={target} />}</div>
}
function ReportQueue() {
  const { t, locale } = useI18n()
  const { user } = useSession()
  const [status, setStatus] = useState<typeof statuses[number]>('open')
  const [cursors, setCursors] = useState([''])
  const [selected, setSelected] = useState('')
  const page = cursors.length - 1
  const query = useQuery({ queryKey: ['admin-reports', user?.id, status, cursors[page]], queryFn: ({ signal }) => request<{ reports: Report[]; next_cursor: string }>(`/admin/reports?status=${status}${cursors[page] ? `&after=${encodeURIComponent(cursors[page])}` : ''}`, { signal }), meta: { private: true }, retry: false })
  return <section className="admin-reports"><Field label={t('reportStatus')}><select value={status} onChange={(event) => { setStatus(event.target.value as typeof status); setCursors(['']); setSelected('') }}>{statuses.map((value) => <option key={value} value={value}>{t(`status_${value}`)}</option>)}</select></Field>{query.isPending ? <Loading /> : query.isError ? <ErrorNotice error={query.error} retry={() => void query.refetch()} /> : <><div className="report-queue">{query.data.reports.length ? query.data.reports.map((report) => <button key={report.id} className={`surface report-queue-item ${selected === report.id ? 'selected' : ''}`} onClick={() => setSelected(report.id)}><span><strong>{t(`target_${report.target_type}`)}</strong> · {t(`reason_${report.reason as 'spam' | 'harassment' | 'dangerous_content' | 'other'}`)}</span><small>{new Date(report.created_at).toLocaleString(locale)}</small></button>) : <p className="surface empty-panel">{t('adminNoReports')}</p>}</div><Pagination page={page} busy={query.isFetching} hasNext={!!query.data.next_cursor} previous={() => setCursors((values) => values.slice(0, -1))} next={() => { if (query.data.next_cursor) setCursors((values) => [...values, query.data.next_cursor]) }} /></>}{selected && <ReportReview key={selected} id={selected} />}</section>
}
function ReportReview({ id }: { id: string }) {
  const { t } = useI18n()
  const { user } = useSession()
  const query = useQuery({ queryKey: ['admin-report', user?.id, id], queryFn: ({ signal }) => request<{ report: Report; target: Target }>(`/admin/reports/${id}`, { signal }), meta: { private: true }, retry: false })
  if (query.isPending) return <Loading />
  if (query.isError) return <ErrorNotice error={query.error} retry={() => void query.refetch()} />
  return <section className="surface report-review"><h2>{t('adminEvidence')}</h2><dl className="admin-evidence"><div><dt>{t('reportReason')}</dt><dd>{t(`reason_${query.data.report.reason as 'spam' | 'harassment' | 'dangerous_content' | 'other'}`)}</dd></div><div><dt>{t('reportDetails')}</dt><dd>{query.data.report.details}</dd></div><div><dt>{t(`target_${query.data.target.type}`)}</dt><dd>{query.data.target.display_name ?? query.data.target.id}</dd></div>{query.data.target.author_id && <div><dt>{t('adminUserId')}</dt><dd>{query.data.target.author_id}</dd></div>}{query.data.target.text && <div><dt>{t('adminEvidence')}</dt><dd>{query.data.target.text}</dd></div>}{query.data.target.media_url && safeExternalUrl(query.data.target.media_url) && <div><dt>{t('videoUrl')}</dt><dd><a href={safeExternalUrl(query.data.target.media_url)} target="_blank" rel="noopener noreferrer">{query.data.target.media_url}</a></dd></div>}</dl>{['bike_photo', 'club_image', 'avatar', 'ride_photo'].includes(query.data.target.type) && <EvidenceImage id={id} />}<ReportReviewForm key={`${id}:${query.data.report.version}`} report={query.data.report} /><AdminActionForm target={query.data.target} /></section>
}
function EvidenceImage({ id }: { id: string }) {
  const { t } = useI18n()
  const [failed, setFailed] = useState(false)
  return failed ? <p className="inline-note">{t('evidenceUnavailable')}</p> : <img className="admin-evidence-image" src={`/api/v1/admin/reports/${id}/content`} alt={t('adminEvidence')} referrerPolicy="no-referrer" onError={() => setFailed(true)} />
}
function ReportReviewForm({ report }: { report: Report }) {
  const { t } = useI18n()
  const client = useQueryClient()
  const start = useSessionTask()
  const online = useOnline()
  const [status, setStatus] = useState(report.status)
  const [reason, setReason] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<unknown>()
  async function submit(event: FormEvent) {
    event.preventDefault(); const task = start(); if (!task.current()) { task.finish(); return }
    setBusy(true); setError(undefined)
    try { await request(`/admin/reports/${report.id}`, { method: 'PATCH', signal: task.signal, body: JSON.stringify({ status, reason: reason.trim(), version: report.version }) }); if (task.current()) { await client.invalidateQueries({ queryKey: ['admin-report'] }); await client.invalidateQueries({ queryKey: ['admin-reports'] }); await client.invalidateQueries({ queryKey: ['admin-audit'] }) } }
    catch (value) { if (task.current()) setError(value) }
    finally { if (task.current()) setBusy(false); task.finish() }
  }
  return <form className="admin-review-form" onSubmit={(event) => void submit(event)}><Field label={t('reportStatus')}><select value={status} disabled={busy} onChange={(event) => setStatus(event.target.value)}>{statuses.map((value) => <option key={value} value={value}>{t(`status_${value}`)}</option>)}</select></Field><TextArea label={t('adminReason')} value={reason} onChange={setReason} required maxLength={4000} />{error != null && <ErrorNotice error={error} />}<div className="form-actions"><button className="button button-primary" disabled={busy || !online || !reason.trim()}><Check size={16} />{t('adminSave')}</button></div></form>
}
function AdminActionForm({ target }: { target?: Target }) {
  const { t } = useI18n()
  const client = useQueryClient()
  const start = useSessionTask()
  const { confirm, dialog } = useConfirmation()
  const online = useOnline()
  const [action, setAction] = useState<typeof actions[number]>(target?.type === 'user' ? 'assign_badge' : target ? 'hide_content' : 'assign_badge')
  const [targetType, setTargetType] = useState<ReportTarget>(target?.type === 'user' ? 'ride' : target?.type ?? 'ride')
  const [targetId, setTargetId] = useState(target?.id ?? '')
  const [reason, setReason] = useState('')
  const [pending, setPending] = useState<{ fingerprint: string; id: string }>()
  const [busy, setBusy] = useState(false)
  const [sent, setSent] = useState(false)
  const [error, setError] = useState<unknown>()
  const userAction = ['assign_badge', 'remove_badge', 'restrict_account', 'restore_account'].includes(action)
  async function submit(event: FormEvent) {
    event.preventDefault(); const task = start()
    if (!await confirm({ message: t('adminActionConfirm'), action: t(`action_${action}`), destructive: ['hide_content', 'restrict_account', 'remove_badge'].includes(action) }) || !task.current()) { task.finish(); return }
    const input = { action, target_type: userAction ? 'user' : targetType, target_id: targetId.trim(), reason: reason.trim() }
    const fingerprint = JSON.stringify(input)
    const id = pending?.fingerprint === fingerprint ? pending.id : crypto.randomUUID()
    setPending({ fingerprint, id }); setBusy(true); setSent(false); setError(undefined)
    try {
      await request('/admin/actions', { method: 'POST', signal: task.signal, body: JSON.stringify({ ...input, client_action_id: id }) })
      if (!task.current()) return
      setPending(undefined); setSent(true); setReason('')
      await resetTrustQueries(client)
      await client.invalidateQueries({ queryKey: ['admin-audit'] })
    } catch (value) { if (task.current()) setError(value) }
    finally { if (task.current()) setBusy(false); task.finish() }
  }
  return <form className="surface admin-action-form" onSubmit={(event) => void submit(event)}>{dialog}<div className="panel-title"><span className="panel-icon"><ShieldCheck size={23} /></span><h2>{t('adminAction')}</h2></div><Field label={t('adminAction')}><select value={action} disabled={busy} onChange={(event) => { const next = event.target.value as typeof action; setAction(next); setTargetId(['assign_badge', 'remove_badge', 'restrict_account', 'restore_account'].includes(next) ? target?.author_id ?? (target?.type === 'user' ? target.id : '') : target?.id ?? ''); setSent(false) }}>{actions.filter((value) => !target || target.type !== 'user' || !['hide_content', 'restore_content'].includes(value)).map((value) => <option key={value} value={value}>{t(`action_${value}`)}</option>)}</select></Field>{!userAction && <Field label={t('adminTargetId')}><select value={targetType} disabled={busy || !!target} onChange={(event) => setTargetType(event.target.value as ReportTarget)}>{targets.filter((value) => value !== 'user').map((value) => <option key={value} value={value}>{t(`target_${value}`)}</option>)}</select></Field>}<Field label={t(userAction ? 'adminUserId' : 'adminTargetId')} value={targetId} onChange={(event) => { setTargetId(event.target.value); setSent(false) }} maxLength={36} required disabled={busy} />{['assign_badge', 'remove_badge'].includes(action) && <p className="inline-note">{t('badgeDisclaimer')}</p>}<TextArea label={t('adminReason')} value={reason} onChange={(value) => { setReason(value); setSent(false) }} maxLength={4000} required />{error != null && <ErrorNotice error={error} />}{sent && <p className="success-message" role="status">{t('adminActionSent')}</p>}<div className="form-actions"><button className="button button-primary" disabled={busy || !online || !reason.trim() || !/^[0-9a-f-]{36}$/i.test(targetId.trim())}>{t(`action_${action}`)}</button></div></form>
}
function AuditHistory() {
  const { t, locale } = useI18n()
  const { user } = useSession()
  const [cursors, setCursors] = useState([''])
  const page = cursors.length - 1
  const query = useQuery({ queryKey: ['admin-audit', user?.id, cursors[page]], queryFn: ({ signal }) => request<{ actions: Audit[]; next_cursor: string }>(`/admin/audit${cursors[page] ? `?after=${encodeURIComponent(cursors[page])}` : ''}`, { signal }), meta: { private: true }, retry: false })
  return query.isPending ? <Loading /> : query.isError ? <ErrorNotice error={query.error} retry={() => void query.refetch()} /> : <section className="audit-history">{query.data.actions.length ? query.data.actions.map((item) => <article key={item.id} className="surface audit-item"><h3>{t(auditActionLabels[item.action as keyof typeof auditActionLabels] ?? 'adminAction')}</h3><p>{item.reason}</p><small>{t(auditTargetLabels[item.target_type] ?? 'adminContent')} · {item.target_id}</small><time dateTime={item.created_at}>{new Date(item.created_at).toLocaleString(locale)}</time></article>) : <p className="surface empty-panel">{t('adminNoActions')}</p>}<Pagination page={page} busy={query.isFetching} hasNext={!!query.data.next_cursor} previous={() => setCursors((values) => values.slice(0, -1))} next={() => { if (query.data.next_cursor) setCursors((values) => [...values, query.data.next_cursor]) }} /></section>
}
