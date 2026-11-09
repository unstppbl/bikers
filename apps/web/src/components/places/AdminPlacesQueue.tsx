import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ApiError, request } from '../../api'
import { intlLocale, useI18n } from '../../i18n'
import { kindLabels, validReason } from '../../places'
import { useSession } from '../../session'
import { useSessionTask } from '../../useSessionTask'
import { ErrorNotice, Loading, Pagination, TextArea, useOnline } from '../../ui'
import type { Place, PlaceSubmission } from '../../types'
import { PlaceInformation } from './shared'
import '../../pages/places.css'
export function AdminPlacesQueue() {
  const { t, locale } = useI18n(), { user } = useSession(), [cursors, setCursors] = useState(['']), [selected, setSelected] = useState('')
  const page = cursors.length - 1
  const query = useQuery({ queryKey: ['place-submissions', 'admin', user?.id, cursors[page]], queryFn: ({ signal }) => request<{ submissions: PlaceSubmission[]; next_cursor: string }>('/admin/place-submissions?status=pending' + (cursors[page] ? '&after=' + encodeURIComponent(cursors[page]) : ''), { signal }), meta: { private: true }, gcTime: 0, retry: false })
  const submission = query.data?.submissions.find((item) => item.id === selected)
  return <section aria-label={t('placesReviewTitle')}><h2>{t('placesReviewTitle')}</h2>{query.isPending ? <Loading /> : query.isError ? <ErrorNotice error={query.error} retry={() => void query.refetch()} /> : <><div className="place-submission-list">{query.data.submissions.length ? query.data.submissions.map((item) => <button type="button" className={'surface place-list-item ' + (selected === item.id ? 'selected' : '')} key={item.id} aria-pressed={selected === item.id} onClick={() => setSelected(item.id)}><span><strong>{item.place?.name ?? t('placesClosed')}</strong><small>{t(kindLabels[item.kind])} · {new Date(item.created_at).toLocaleString(intlLocale(locale))}</small></span></button>) : <p className="surface empty-panel">{t('placesNoPending')}</p>}</div><Pagination page={page} busy={query.isFetching} hasNext={!!query.data.next_cursor} previous={() => { setCursors((v) => v.slice(0, -1)); setSelected('') }} next={() => { if (query.data.next_cursor) { setCursors((v) => [...v, query.data.next_cursor]); setSelected('') } }} /></>}{submission && <PlaceReview key={submission.id + ':' + submission.version} submission={submission} refresh={() => void query.refetch()} />}</section>
}
function PlaceReview({ submission, refresh }: { submission: PlaceSubmission; refresh: () => void }) {
  const { t } = useI18n(), start = useSessionTask(), client = useQueryClient(), online = useOnline()
  const [reason, setReason] = useState(''), [busy, setBusy] = useState(false), [error, setError] = useState<unknown>(), [invalid, setInvalid] = useState(false), [done, setDone] = useState(false)
  const current = useQuery({ queryKey: ['places', 'detail', submission.place_id], queryFn: ({ signal }) => request<{ place: Place }>('/places/' + encodeURIComponent(submission.place_id!), { signal }), enabled: !!submission.place_id, meta: { private: false }, retry: false, staleTime: 0, refetchOnMount: 'always' })
  const changed = !!submission.place_id && (!!current.data && current.data.place.version !== submission.base_version || current.isError)
  async function review(status: 'approved' | 'rejected') {
    if (!validReason(reason)) { setInvalid(true); return }
    const task = start(); if (!task.current()) { task.finish(); return }; setBusy(true); setError(undefined); setInvalid(false)
    try { await request('/admin/place-submissions/' + submission.id + '/review', { method: 'POST', signal: task.signal, body: JSON.stringify({ version: submission.version, status, reason: reason.trim() }) }); if (task.current()) { setDone(true); void client.invalidateQueries({ queryKey: ['place-submissions'] }); void client.invalidateQueries({ queryKey: ['places'] }) } }
    catch (value) { if (task.current()) setError(value) }
    finally { if (task.current()) setBusy(false); task.finish() }
  }
  if (done) return <p role="status" className="success-message">{t('placesReviewDone')}</p>
  const stale = error instanceof ApiError && error.code === 'stale_version'
  return <section className="surface place-submission-card place-review"><h2>{t(kindLabels[submission.kind])}</h2><span className="field-hint">{t('placesVersion', { version: submission.version })}</span><div className="place-review-panes">{submission.place_id && <section><h3>{t('placesCurrent')}</h3>{current.isPending ? <Loading /> : current.isError ? <ErrorNotice text={t('placesUnavailableCurrent')} retry={() => void current.refetch()} /> : <><h4>{current.data.place.name}</h4><PlaceInformation place={current.data.place} /></>}</section>}<section><h3>{t('placesProposal')}</h3>{submission.place ? <><h4>{submission.place.name}</h4><PlaceInformation place={submission.place} /></> : <p>{t('placesClosedHint')}</p>}</section></div><p className="place-submission-reason"><strong>{t('placesReason')} </strong>{submission.reason}</p>{changed && <ErrorNotice text={t('placesStale')} />}<fieldset disabled={busy} className="place-review-reason"><TextArea label={t('placesReviewReason')} value={reason} onChange={setReason} required maxLength={2000} rows={3} /></fieldset>{invalid && <ErrorNotice text={t('placesReviewRequired')} />}{error != null && <ErrorNotice error={error} text={stale ? t('placesStale') : undefined} />}{stale && <button className="button button-outline" onClick={() => { refresh(); void current.refetch(); setError(undefined) }}>{t('placesRefresh')}</button>}<div className="form-actions"><button className="button button-outline" disabled={busy || !online} onClick={() => void review('rejected')}>{t('placesReject')}</button><button className="button button-primary" disabled={busy || !online || changed || (!!submission.place_id && !current.data) || stale} onClick={() => void review('approved')}>{t('placesApprove')}</button></div></section>
}
