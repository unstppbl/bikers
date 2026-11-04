import { useCallback, useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowLeftIcon as ArrowLeft, ArrowRightIcon as ArrowRight, ArrowUpRightIcon as ArrowUpRight, CalendarDotsIcon as CalendarDays, CheckIcon as Check, GaugeIcon as Gauge, GlobeIcon as Globe, LockKeyIcon as LockKeyhole, PencilSimpleIcon as Pencil, PlusIcon as Plus, MagnifyingGlassIcon as Search, ShieldCheckIcon as ShieldCheck, TrashIcon as Trash2, WrenchIcon as Wrench } from '@phosphor-icons/react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { formatInTimeZone } from 'date-fns-tz'
import { ApiError, mutation, request } from '../api'
import { useI18n, intlLocale } from '../i18n'
import { useSession, useSessionGuard } from '../session'
import { EmptyState, ErrorNotice, Field, Loading, Pagination, ShareLink, TextArea, formatCalendarDate, useConfirmation, useDebouncedValue, useOnline } from '../ui'
import type { Bike, BikeModel, MaintenanceEntry, PublicBike } from '../types'
import { ModelReference } from '../components/ModelReference'
import { ReportButton } from '../components/Trust'
import { BikeCover, BikePhotos } from '../components/BikePhotos'

interface BikePage { bikes: Bike[]; next_cursor: string }
interface MaintenancePage { entries: MaintenanceEntry[]; next_cursor: string }
const managedBike = (bike: Bike | PublicBike, actor?: string): bike is Bike => 'can_manage' in bike && bike.can_manage && bike.owner_id === actor

function GarageBikeCard({ bike }: { bike: Bike }) {
  const { t } = useI18n()
  return <article className="surface bike-card"><Link className="bike-card-main" to={`/garage/${bike.id}`}><div className="bike-card-top"><span className="eyebrow">{bike.make}</span><span className={`status-pill ${bike.visibility === 'public' ? 'approved' : ''}`}>{bike.visibility === 'public' ? <Globe size={12} /> : <LockKeyhole size={12} />}{t(bike.visibility === 'public' ? 'bikePublic' : 'bikePrivate')}</span></div><div className="bike-identity"><strong>{bike.model}</strong><span>{bike.year}</span></div></Link><BikeCover bike={bike} variant="card" /><Link className="bike-card-bottom" to={`/garage/${bike.id}`}><span><Wrench size={15} />{t('maintenance')}</span><ArrowUpRight size={21} /></Link></article>
}

export function GaragePage() {
  const { user } = useSession()
  const { t } = useI18n()
  const [cursors, setCursors] = useState([''])
  const page = cursors.length - 1
  const query = useQuery({ queryKey: ['garage', user?.id, cursors[page]], queryFn: ({ signal }) => request<BikePage>(`/me/bikes${cursors[page] ? `?cursor=${encodeURIComponent(cursors[page])}` : ''}`, { signal }), meta: { private: true }, enabled: !!user })
  return <div className="page-content collection-page"><div className="collection-heading"><div className="page-heading"><span className="eyebrow">{t('garageEyebrow')}</span><h1>{t('garageTitle')}</h1><p>{t('garageText')}</p></div><Link className="button button-primary" to="/garage/new"><Plus size={17} />{t('addBike')}</Link></div>
    {query.isPending ? <Loading /> : query.isError ? <ErrorNotice error={query.error} retry={() => void query.refetch()} /> : !query.data.bikes.length ? <EmptyState title={t('garageEmptyTitle')} text={t('garageEmptyText')} action={<Link className="button button-outline" to="/garage/new">{t('addBike')}<ArrowRight size={17} /></Link>} /> : <div className="bike-grid">{query.data.bikes.map((bike) => <GarageBikeCard key={bike.id} bike={bike} />)}</div>}
    <Pagination page={page} hasNext={!!query.data?.next_cursor} busy={query.isFetching} previous={() => setCursors((values) => values.slice(0, -1))} next={() => { if (query.data?.next_cursor) setCursors((values) => [...values, query.data.next_cursor]) }} />
  </div>
}

export function BikeFormPage({ edit = false }: { edit?: boolean }) {
  const { bikeId = '' } = useParams()
  const { user } = useSession()
  const { t } = useI18n()
  const query = useQuery({ queryKey: ['bike', bikeId, user?.id], queryFn: ({ signal }) => request<{ bike: Bike | PublicBike }>(`/bikes/${bikeId}`, { signal }), meta: { private: true }, enabled: edit && !!user, refetchOnWindowFocus: false, staleTime: 0 })
  if (edit && (query.isPending || query.isFetching)) return <Loading />
  if (edit && query.isError) return <div className="page-content"><ErrorNotice error={query.error} retry={() => void query.refetch()} /></div>
  const bike = query.data?.bike
  if (edit && (!bike || !managedBike(bike, user?.id))) return <div className="page-content"><ErrorNotice text={t('noAccess')} /></div>
  return <BikeForm key={`${user?.id}-${edit && bike && 'version' in bike ? `${bike.id}-${bike.version}` : 'new'}`} bike={edit && bike && managedBike(bike, user?.id) ? bike : undefined} reload={() => void query.refetch()} />
}

function BikeForm({ bike, reload }: { bike?: Bike; reload: () => void }) {
  const { t } = useI18n()
  const { user } = useSession()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const online = useOnline()
  const captureSession = useSessionGuard()
  const [form, setForm] = useState({ make: bike?.make ?? '', model: bike?.model ?? '', model_id: bike?.model_id ?? '', year: bike ? String(bike.year) : '' })
  const [search, setSearch] = useState('')
  const debounced = useDebouncedValue(search)
  const models = useQuery({ queryKey: ['bike-models', debounced], queryFn: ({ signal }) => request<{ models: BikeModel[] }>(`/bike-models?query=${encodeURIComponent(debounced.trim())}`, { signal }), enabled: !!debounced.trim(), retry: false })
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<unknown>()
  const [validation, setValidation] = useState('')
  async function submit(event: FormEvent) {
    const currentSession = captureSession()
    event.preventDefault(); setError(undefined); setValidation('')
    if (!form.make.trim() || !form.model.trim() || !form.year) { setValidation(t('requiredFields')); return }
    const year = Number(form.year)
    const currentYear = Number(formatInTimeZone(new Date(), 'Asia/Almaty', 'yyyy'))
    if (!Number.isInteger(year) || year < 1885 || year > currentYear + 1) { setValidation(t('invalidYear')); return }
    setBusy(true)
    try {
      const result = await mutation<{ bike: Bike }>(bike ? `/me/bikes/${bike.id}` : '/me/bikes', bike ? 'PATCH' : 'POST', { make: form.make.trim(), model: form.model.trim(), model_id: form.model_id, year, ...(bike ? { version: bike.version } : { visibility: 'private' }) })
      if (!currentSession()) return
      await queryClient.invalidateQueries({ queryKey: ['garage', user?.id] })
      if (!currentSession()) return
      queryClient.setQueryData(['bike', result.bike.id, user?.id], result)
      void navigate(`/garage/${result.bike.id}`)
    } catch (value) { setError(value) }
    finally { setBusy(false) }
  }
  return <div className="page-content form-page"><Link className="back-link" to={bike ? `/garage/${bike.id}` : '/garage'}><ArrowLeft size={17} />{t('backToGarage')}</Link><div className="page-heading"><span className="eyebrow">{t('garageEyebrow')}</span><h1>{t(bike ? 'editBikeTitle' : 'addBikeTitle')}</h1><p>{t('bikeFormText')}</p></div>
    <section className="surface model-lookup"><h2><Search size={19} />{t('modelLookup')}</h2><p>{t('modelSearchHint')}</p><Field label={t('modelLookup')} type="search" value={search} onChange={(event) => setSearch(event.target.value)} placeholder={t('modelSearchPlaceholder')} maxLength={100} />{debounced.trim() && (models.isPending ? <Loading /> : models.isError ? <ErrorNotice error={models.error} /> : models.data?.models.length ? <div className="model-results">{models.data.models.map((model) => <button type="button" key={model.id} onClick={() => { setForm((current) => ({ ...current, make: model.make, model: model.model, model_id: model.id })); setSearch('') }}><strong>{model.make} {model.model}</strong><ArrowRight size={15} /></button>)}</div> : <p className="small-empty">{t('noModels')}</p>)}</section>
    <form className="surface form-section" onSubmit={(event) => void submit(event)} noValidate>{form.model_id && <p className="selected-model"><Check size={16} />{t('modelSelected')}: {form.make} {form.model}</p>}<div className="form-grid"><Field label={t('make')} value={form.make} onChange={(event) => setForm({ ...form, make: event.target.value, model_id: '' })} placeholder={t('makePlaceholder')} maxLength={100} required disabled={busy} /><Field label={t('model')} value={form.model} onChange={(event) => setForm({ ...form, model: event.target.value, model_id: '' })} placeholder={t('modelPlaceholder')} maxLength={100} required disabled={busy} /></div><Field label={t('modelYear')} type="number" inputMode="numeric" min="1885" value={form.year} onChange={(event) => setForm({ ...form, year: event.target.value })} required disabled={busy} /><ModelReference bike={{ model_id: form.model_id, make: form.make, model: form.model, year: Number(form.year) }} variant="form" showUnavailable /><div className="inline-note"><LockKeyhole size={20} /><p>{t(bike?.visibility === 'public' ? 'publicBikeNote' : 'privateBikeNote')}</p></div>{validation && <ErrorNotice text={validation} />}{error != null && <ErrorNotice error={error} />}{error instanceof ApiError && error.code === 'stale_version' && <button className="text-button" type="button" onClick={reload}>{t('reloadLatest')}</button>}<div className="form-actions"><Link className="button button-outline" to={bike ? `/garage/${bike.id}` : '/garage'}>{t('cancel')}</Link><button className="button button-primary" type="submit" disabled={busy || !online}>{busy ? t('saving') : t('saveBike')}<ArrowRight size={17} /></button></div></form>
  </div>
}

export function BikeOwnerPage() {
  const { bikeId = '' } = useParams()
  const { user } = useSession()
  const { t } = useI18n()
  const query = useQuery({ queryKey: ['bike', bikeId, user?.id], queryFn: ({ signal }) => request<{ bike: Bike | PublicBike }>(`/bikes/${bikeId}`, { signal }), meta: { private: true }, enabled: !!user })
  if (query.isPending) return <Loading />
  if (query.isError) return <div className="page-content"><Link className="back-link" to="/garage"><ArrowLeft size={17} />{t('backToGarage')}</Link><ErrorNotice error={query.error} retry={() => void query.refetch()} /></div>
  if (!managedBike(query.data.bike, user?.id)) return <div className="page-content"><ErrorNotice text={t('noAccess')} /></div>
  return <BikeOwnerContent key={`${bikeId}-${user?.id}`} bike={query.data.bike} />
}

function BikeOwnerContent({ bike }: { bike: Bike }) {
  const { confirm, dialog } = useConfirmation()
  const { t } = useI18n()
  const { user } = useSession()
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const online = useOnline()
  const captureSession = useSessionGuard()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<unknown>()
  async function visibility() {
    const currentSession = captureSession()
    const publish = bike.visibility === 'private'
    if (!await confirm({ message: t(publish ? 'shareBikeConfirm' : 'hideBikeConfirm'), action: t(publish ? 'shareBike' : 'hideBike') }) || !currentSession()) return
    setBusy(true); setError(undefined)
    try { const result = await mutation<{ bike: Bike }>(`/me/bikes/${bike.id}`, 'PATCH', { version: bike.version, visibility: publish ? 'public' : 'private' }); if (!currentSession()) return; queryClient.setQueryData(['bike', bike.id, user?.id], result); await queryClient.invalidateQueries({ queryKey: ['garage', user?.id] }); await queryClient.invalidateQueries({ queryKey: ['public-bike', bike.id] }) }
    catch (value) { setError(value) } finally { setBusy(false) }
  }
  async function remove() {
    const currentSession = captureSession()
    if (!await confirm({ message: t('deleteBikeConfirm'), action: t('deleteBike'), destructive: true }) || !currentSession()) return
    setBusy(true); setError(undefined)
    try { await mutation(`/me/bikes/${bike.id}`, 'DELETE', { version: bike.version }); if (!currentSession()) return; queryClient.removeQueries({ queryKey: ['maintenance', bike.id] }); queryClient.removeQueries({ queryKey: ['bike-photos', bike.id] }); queryClient.removeQueries({ queryKey: ['bike', bike.id] }); queryClient.removeQueries({ queryKey: ['public-bike', bike.id] }); await queryClient.invalidateQueries({ queryKey: ['garage', user?.id] }); if (!currentSession()) return; void navigate('/garage') }
    catch (value) { setError(value) } finally { setBusy(false) }
  }
  return <div className="page-content bike-detail-page">{dialog}<Link className="back-link" to="/garage"><ArrowLeft size={17} />{t('backToGarage')}</Link><div className="collection-heading"><div className="page-heading"><span className="eyebrow">{bike.make} · {bike.year}</span><h1>{bike.model}</h1><p>{t(bike.visibility === 'public' ? 'publicBikeNote' : 'privateBikeNote')}</p></div><Link className="button button-outline" to={`/garage/${bike.id}/edit`}><Pencil size={16} />{t('editBike')}</Link></div>
    <BikeCover bike={bike} />
    <section className="surface visibility-card"><div className="panel-title"><span className="panel-icon">{bike.visibility === 'public' ? <Globe size={20} /> : <LockKeyhole size={20} />}</span><div><h2>{t('bikeVisibility')}</h2><p>{t(bike.visibility === 'public' ? 'bikePublic' : 'bikePrivate')}</p></div></div><div className="visibility-actions"><button className="button button-outline" onClick={() => void visibility()} disabled={busy || !online}>{t(bike.visibility === 'private' ? 'shareBike' : 'hideBike')}</button>{bike.visibility === 'public' && <><Link className="inline-link" to={`/bikes/${bike.id}`}>{t('bikePublic')}<ArrowUpRight size={14} /></Link><ShareLink path={`/bikes/${bike.id}`} /></>}</div></section>
    {error != null && <ErrorNotice error={error} retry={error instanceof ApiError && error.code === 'stale_version' ? () => void queryClient.invalidateQueries({ queryKey: ['bike', bike.id] }) : undefined} />}
    <BikePhotos bike={bike} owner />
    <MaintenancePanel bike={bike} />
    <div className="ride-lower-actions"><button className="text-button danger-text" onClick={() => void remove()} disabled={busy || !online}><Trash2 size={16} />{t('deleteBike')}</button></div>
  </div>
}

function MaintenancePanel({ bike }: { bike: Bike }) {
  const { confirm, dialog } = useConfirmation()
  const { user } = useSession()
  const { t, locale } = useI18n()
  const queryClient = useQueryClient()
  const online = useOnline()
  const captureSession = useSessionGuard()
  const [cursors, setCursors] = useState([''])
  const [editor, setEditor] = useState<MaintenanceEntry | 'new' | null>(null)
  const [deleting, setDeleting] = useState('')
  const [error, setError] = useState<unknown>()
  const page = cursors.length - 1
  const query = useQuery({ queryKey: ['maintenance', bike.id, user?.id, cursors[page]], queryFn: ({ signal }) => request<MaintenancePage>(`/me/bikes/${bike.id}/maintenance${cursors[page] ? `?cursor=${encodeURIComponent(cursors[page])}` : ''}`, { signal }), meta: { private: true } })
  const reload = useCallback(async () => { setEditor(null); setCursors(['']); await queryClient.invalidateQueries({ queryKey: ['maintenance', bike.id] }) }, [queryClient, bike.id])
  useEffect(() => {
    if (query.error instanceof ApiError && [401, 403, 404].includes(query.error.status)) {
      queryClient.removeQueries({ queryKey: ['maintenance', bike.id] })
      void queryClient.invalidateQueries({ queryKey: ['bike', bike.id] })
    }
  }, [query.error, queryClient, bike.id])
  async function remove(entry: MaintenanceEntry) {
    const currentSession = captureSession()
    if (!await confirm({ message: t('deleteMaintenanceConfirm'), action: t('deleteMaintenance'), destructive: true }) || !currentSession()) return
    setDeleting(entry.id); setError(undefined)
    try { await mutation(`/me/bikes/${bike.id}/maintenance/${entry.id}`, 'DELETE', { version: entry.version }); if (!currentSession()) return; await reload() }
    catch (value) { setError(value) } finally { setDeleting('') }
  }
  return <section className="maintenance-section">{dialog}<div className="section-header"><div><span className="eyebrow">{t('privateRecords')}</span><h2>{t('maintenance')}</h2></div><button className="button button-primary" disabled={!online} onClick={() => setEditor('new')}><Plus size={16} />{t('addMaintenance')}</button></div><p className="section-intro">{t('maintenanceText')}</p>
    {editor && <MaintenanceForm key={editor === 'new' ? 'new' : `${editor.id}-${editor.version}`} bike={bike} entry={editor === 'new' ? undefined : editor} close={() => setEditor(null)} saved={reload} />}
    {error != null && <ErrorNotice error={error} retry={() => void reload()} />}
    {query.isPending ? <Loading /> : query.isError ? <ErrorNotice error={query.error} retry={() => void query.refetch()} /> : !query.data.entries.length ? <EmptyState title={t('noMaintenanceTitle')} text={t('noMaintenanceText')} /> : <div className="maintenance-list">{query.data.entries.map((entry) => <article key={entry.id} className="surface maintenance-entry"><div className="maintenance-meta"><span><CalendarDays size={15} />{formatCalendarDate(entry.service_date, locale)}</span><span><Gauge size={15} />{entry.mileage_km.toLocaleString(intlLocale(locale))} {t('kilometres')}</span><span className="private-label"><LockKeyhole size={12} />{t('privateRecords')}</span></div><p>{entry.work_performed}</p><div className="entry-actions"><button className="text-button" onClick={() => setEditor(entry)} disabled={!!deleting || !online}><Pencil size={14} />{t('edit')}</button><button className="text-button danger-text" onClick={() => void remove(entry)} disabled={!!deleting || !online}><Trash2 size={14} />{t('deleteMaintenance')}</button></div></article>)}</div>}
    <Pagination page={page} hasNext={!!query.data?.next_cursor} busy={query.isFetching} previous={() => setCursors((values) => values.slice(0, -1))} next={() => { if (query.data?.next_cursor) setCursors((values) => [...values, query.data.next_cursor]) }} />
  </section>
}

function MaintenanceForm({ bike, entry, close, saved }: { bike: Bike; entry?: MaintenanceEntry; close: () => void; saved: () => Promise<void> }) {
  const { t } = useI18n()
  const online = useOnline()
  const captureSession = useSessionGuard()
  const [form, setForm] = useState({ service_date: entry?.service_date ?? '', mileage_km: entry ? String(entry.mileage_km) : '', work_performed: entry?.work_performed ?? '' })
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<unknown>()
  const [validation, setValidation] = useState('')
  async function submit(event: FormEvent) {
    const currentSession = captureSession()
    event.preventDefault(); setError(undefined); setValidation('')
    if (!form.service_date || !form.mileage_km || !form.work_performed.trim()) { setValidation(t('requiredFields')); return }
    const date = new Date(`${form.service_date}T12:00:00Z`)
    const today = formatInTimeZone(new Date(), 'Asia/Almaty', 'yyyy-MM-dd')
    if (!/^\d{4}-\d{2}-\d{2}$/.test(form.service_date) || Number.isNaN(date.getTime()) || date.toISOString().slice(0, 10) !== form.service_date || form.service_date > today) { setValidation(t('invalidServiceDate')); return }
    const mileage = Number(form.mileage_km)
    if (!Number.isInteger(mileage) || mileage < 0 || mileage > 10_000_000) { setValidation(t('invalidMileage')); return }
    setBusy(true)
    try { await mutation(entry ? `/me/bikes/${bike.id}/maintenance/${entry.id}` : `/me/bikes/${bike.id}/maintenance`, entry ? 'PATCH' : 'POST', { service_date: form.service_date, mileage_km: mileage, work_performed: form.work_performed.trim(), ...(entry ? { version: entry.version } : {}) }); if (!currentSession()) return; await saved() }
    catch (value) { setError(value) } finally { setBusy(false) }
  }
  return <form className="surface form-section maintenance-form" onSubmit={(event) => void submit(event)} noValidate><div className="panel-title"><span className="panel-icon"><Wrench size={20} /></span><h2>{t(entry ? 'editMaintenance' : 'addMaintenance')}</h2></div><div className="form-grid"><Field label={t('serviceDate')} type="date" value={form.service_date} onChange={(event) => setForm({ ...form, service_date: event.target.value })} required disabled={busy} /><Field label={t('mileageKm')} type="number" inputMode="numeric" min="0" max="10000000" value={form.mileage_km} onChange={(event) => setForm({ ...form, mileage_km: event.target.value })} required disabled={busy} /></div><TextArea label={t('workPerformed')} value={form.work_performed} onChange={(work_performed) => setForm({ ...form, work_performed })} placeholder={t('workPlaceholder')} maxLength={4000} required rows={3} />{validation && <ErrorNotice text={validation} />}{error != null && <ErrorNotice error={error} />}{error instanceof ApiError && error.code === 'stale_version' && <button className="text-button" type="button" onClick={() => void saved()}>{t('reloadLatest')}</button>}<div className="form-actions"><button className="button button-outline" type="button" onClick={close} disabled={busy}>{t('cancel')}</button><button className="button button-primary" type="submit" disabled={busy || !online}>{busy ? t('saving') : t('saveMaintenance')}<Check size={17} /></button></div></form>
}

export function PublicBikePage() {
  const { bikeId = '' } = useParams()
  const { user, loading, error, refresh } = useSession()
  const { t } = useI18n()
  const query = useQuery({ queryKey: ['public-bike', bikeId, user?.id ?? 'guest'], queryFn: async ({ signal }) => {
    const result = await request<{ bike: PublicBike | Bike }>(`/bikes/${bikeId}`, { signal })
    if (user) return result
    const { id, make, model, model_id, year } = result.bike
    return { bike: { id, make, model, model_id, year } }
  }, enabled: !loading && !error, meta: { private: true } })
  if (error) return <div className="page-content"><ErrorNotice text={t('serviceUnavailable')} retry={() => void refresh()} /></div>
  if (query.isPending) return <Loading />
  if (query.isError) return <div className="page-content"><ErrorNotice text={t('publicBikeUnavailable')} /></div>
  const bike = query.data.bike
  return <div className="page-content narrow-page"><Link className="back-link" to="/"><ArrowLeft size={17} />{t('back')}</Link><section className="surface public-bike-card"><span className="eyebrow">{t(managedBike(bike, user?.id) && bike.visibility === 'private' ? 'bikePrivate' : 'publicBikeEyebrow')}</span><div className="public-bike-year">{bike.year}</div><h1>{bike.make}<br /><span>{bike.model}</span></h1><BikeCover bike={bike} /><BikePhotos bike={bike} /><ReportButton targetType="bike" targetId={bike.id} /><div className="inline-note"><ShieldCheck size={20} /><p>{t('publicBikePrivacy')}</p></div>{managedBike(bike, user?.id) && <Link className="button button-outline" to={`/garage/${bike.id}`}>{t('garage')}<ArrowRight size={17} /></Link>}</section></div>
}
