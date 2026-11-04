import { CitySelector, useCityOptions } from '../components/CitySelector'
import { cityLabel } from '../cities'
import { useCallback, useState } from 'react'
import type { FormEvent } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowLeftIcon as ArrowLeft, ArrowRightIcon as ArrowRight, ArrowUpRightIcon as ArrowUpRight, CheckIcon as Check, ClockIcon as Clock3, DoorOpenIcon as DoorOpen, LinkSimpleIcon as Link2, LockKeyIcon as LockKeyhole, MapPinIcon as MapPin, PencilSimpleIcon as Pencil, PlusIcon as Plus, ShieldCheckIcon as ShieldCheck, UsersThreeIcon as Users, XIcon as X } from '@phosphor-icons/react'
import { Link, Navigate, useLocation, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { ApiError, mutation, request } from '../api'
import { useI18n } from '../i18n'
import { useSession, useSessionGuard } from '../session'
import { EmptyState, ErrorNotice, Field, Loading, Pagination, ShareLink, TextArea, dateParts, initials, useConfirmation, useOnline } from '../ui'
import type { Club, ClubMember, ClubSummary, Ride, RidePage } from '../types'
import { Avatar } from '../components/Avatar'
import { CommunityBadge, ReportButton, UserSafety } from '../components/Trust'
import { ClubEmblem, ClubImageEditor, ClubImageReport } from '../components/ClubImage'

interface ClubPage { clubs: ClubSummary[]; next_cursor: string }
const fullClub = (club?: ClubSummary | Club): club is Club => !!club && 'owner_id' in club && 'version' in club && 'member_count' in club
const publicClub = (club: ClubSummary): ClubSummary => ({ id: club.id, name: club.name, city: club.city, description: club.description, status: club.status, can_manage: false })

export function ClubsPage() { return <ClubCollection personal={false} /> }
export function MyClubsPage() { return <ClubCollection personal /> }

const personalClubViews = ['organizing', 'joined', 'pending'] as const
type ClubView = 'discover' | typeof personalClubViews[number]
interface ClubFilters { q: string; city: string }
interface ClubPaging { scope: string; cursors: string[] }

function ClubCollection({ personal }: { personal: boolean }) {
  const { user, loading, error, refresh } = useSession()
  const { t } = useI18n()
  const [params] = useSearchParams()
  const location = useLocation()
  const selected = params.get('view')
  const view: ClubView = personal ? personalClubViews.find((value) => value === selected) ?? 'organizing' : 'discover'
  if (loading) return <Loading />
  if (error) return <div className="page-content"><ErrorNotice text={t('serviceUnavailable')} retry={() => void refresh()} /></div>
  if (personal && !user) return <Navigate to={`/login?next=${encodeURIComponent(location.pathname + location.search)}`} replace />
  function tab(value: ClubView) {
    const next = new URLSearchParams(params)
    next.delete('cursor')
    if (value === 'discover') next.delete('view')
    else next.set('view', value)
    return { pathname: value === 'discover' ? '/clubs' : '/my-clubs', search: next.toString() }
  }
  return <div className="page-content collection-page"><div className="collection-heading"><div className="page-heading"><span className="eyebrow">{t('clubsEyebrow')}</span><h1>{t(personal ? 'myClubs' : 'clubsTitle')}</h1><p>{t(personal ? 'myClubsText' : 'clubsText')}</p></div><Link className="button button-primary" to="/clubs/new"><Plus size={17} />{t('createClub')}</Link></div>
    <nav className="collection-tabs" aria-label={t('clubs')}><Link className={!personal ? 'active' : ''} aria-current={!personal ? 'page' : undefined} to={tab('discover')}>{t('allClubs')}</Link><Link className={personal ? 'active' : ''} aria-current={personal ? 'page' : undefined} to={tab('organizing')}>{t('myClubs')}</Link></nav>
    {personal && <nav className="collection-tabs ride-view-tabs" aria-label={t('myClubs')}>{personalClubViews.map((value) => <Link key={value} className={view === value ? 'active' : ''} aria-current={view === value ? 'page' : undefined} to={tab(value)}>{t(value)}</Link>)}</nav>}
    <ClubList view={view} />
  </div>
}

function ClubList({ view }: { view: ClubView }) {
  const { user } = useSession()
  const { t } = useI18n()
  const [params] = useSearchParams()
  const location = useLocation()
  const navigate = useNavigate()
  const filters: ClubFilters = { q: params.get('q') ?? '', city: params.get('city') ?? '' }
  const cursor = params.get('cursor') ?? ''
  const scope = JSON.stringify([user?.id ?? 'guest', view, filters.q, filters.city])
  const stored = (location.state as { clubPaging?: ClubPaging } | null)?.clubPaging
  const staleCursor = !!cursor && !!stored && stored.scope !== scope
  const cursors = stored?.scope === scope && stored.cursors.at(-1) === cursor ? stored.cursors : cursor ? ['', cursor] : ['']
  const page = cursors.length - 1
  const queryParams = new URLSearchParams({ view })
  for (const [key, value] of Object.entries(filters)) if (value) queryParams.set(key, value)
  if (cursor) queryParams.set('cursor', cursor)
  const query = useQuery({ queryKey: ['clubs', user?.id ?? 'guest', view, filters.q, filters.city, cursor], enabled: !staleCursor,
    queryFn: async ({ signal }) => {
      const result = await request<ClubPage>(`/clubs?${queryParams}`, { signal })
      return user ? result : { ...result, clubs: result.clubs.map(publicClub) }
    }, meta: { private: true } })
  function goToCursor(nextCursors: string[]) {
    const next = new URLSearchParams(params)
    const current = nextCursors.at(-1) ?? ''
    if (current) next.set('cursor', current)
    else next.delete('cursor')
    void navigate({ pathname: location.pathname, search: next.toString() }, { state: { clubPaging: { scope, cursors: nextCursors } } })
  }
  if (staleCursor) {
    const next = new URLSearchParams(params); next.delete('cursor')
    return <Navigate to={{ pathname: location.pathname, search: next.toString() }} replace />
  }
  const hasFilters = Object.values(filters).some(Boolean)
  return <section className="club-list-section" aria-label={t('clubs')}>
    <ClubSearch key={JSON.stringify(filters)} filters={filters} />
    <div className="section-header"><h2>{t(view === 'discover' ? 'allClubs' : view)}</h2>{query.data && <span className="count-pill">{t('clubsOnPage', { count: query.data.clubs.length })}</span>}</div>
    {query.isPending ? <Loading /> : query.isError ? <><ErrorNotice error={query.error} retry={() => void query.refetch()} />{cursor && <button type="button" className="text-button" onClick={() => goToCursor([''])}>{t('firstClubPage')}</button>}</> : !query.data.clubs.length ? <EmptyState title={t(hasFilters || cursor ? 'noResults' : 'clubsEmptyTitle')} text={t(hasFilters || cursor ? 'clubFiltersEmptyText' : view === 'discover' ? 'clubsEmptyText' : `${view}ClubsEmpty`)} action={<Link className="button button-outline" to={view === 'organizing' || view === 'discover' ? '/clubs/new' : '/clubs'}>{t(view === 'organizing' || view === 'discover' ? 'createClub' : 'allClubs')}<ArrowRight size={17} /></Link>} /> : <div className="club-grid">{query.data.clubs.map((club) => <ClubCard key={club.id} club={club} />)}</div>}
    {!query.isError && <Pagination page={page} hasNext={!!query.data?.next_cursor} busy={query.isFetching} previous={() => goToCursor(cursors.slice(0, -1))} next={() => { if (query.data?.next_cursor) goToCursor([...cursors, query.data.next_cursor]) }} />}
  </section>
}

function ClubSearch({ filters }: { filters: ClubFilters }) {
  const { t } = useI18n()
  const [params, setParams] = useSearchParams()
  const [draft, setDraft] = useState(filters)
  function apply(values: ClubFilters) {
    const next = new URLSearchParams(params)
    next.delete('cursor')
    for (const [key, value] of Object.entries(values)) {
      if (value.trim()) next.set(key, value.trim())
      else next.delete(key)
    }
    setParams(next)
  }
  function submit(event: FormEvent) { event.preventDefault(); apply(draft) }
  return <form className="ride-filters club-filters" aria-label={t('clubFilters')} onSubmit={submit}>
    <div className="ride-filter-fields"><Field label={t('clubSearch')} type="search" value={draft.q} onChange={(event) => setDraft({ ...draft, q: event.target.value })} maxLength={200} placeholder={t('clubSearchPlaceholder')} /><CitySelector value={draft.city} onChange={(city) => setDraft({ ...draft, city })} label={t('city')} placeholder={t('allCities')} hint="" maxLength={160} /></div>
    <div className="ride-filter-actions"><button type="submit" className="button button-dark">{t('applyFilters')}</button><button type="button" className="text-button" onClick={() => { const empty = { q: '', city: '' }; setDraft(empty); apply(empty) }}>{t('clearFilters')}</button></div>
  </form>
}

function ClubCard({ club }: { club: ClubSummary }) {
  const { t, locale } = useI18n()
  const cities = useCityOptions()
  return <Link className="surface club-card" to={`/clubs/${club.id}`}><div className="club-card-top"><ClubEmblem club={club} /><ArrowUpRight size={23} /></div><span className="city-label"><MapPin size={14} />{cityLabel(club.city, locale, cities.data?.cities ?? [])}</span><h2>{club.name}</h2><p>{club.description}</p><div className="club-card-bottom"><span><ShieldCheck size={14} />{t('clubApprovalNote')}</span>{club.status === 'closed' ? <span className="status-pill cancelled">{t('clubClosed')}</span> : club.can_manage ? <span className="status-pill">{t('clubOwner')}</span> : club.membership_state === 'approved' ? <span className="status-pill approved">{t('clubMember')}</span> : club.membership_state === 'requested' ? <span className="status-pill pending">{t('requested')}</span> : null}</div></Link>
}

export function ClubFormPage({ edit = false }: { edit?: boolean }) {
  const { clubId = '' } = useParams()
  const { user } = useSession()
  const { t } = useI18n()
  const query = useQuery({ queryKey: ['club', clubId, user?.id ?? 'guest'], queryFn: ({ signal }) => request<{ club: ClubSummary | Club }>(`/clubs/${clubId}`, { signal }), enabled: edit && !!user, meta: { private: true }, refetchOnWindowFocus: false, staleTime: 0 })
  if (edit && (query.isPending || query.isFetching)) return <Loading />
  if (edit && query.isError) return <div className="page-content"><ErrorNotice error={query.error} retry={() => void query.refetch()} /></div>
  if (edit && (!fullClub(query.data?.club) || !query.data.club.can_manage || query.data.club.status !== 'active')) return <div className="page-content"><ErrorNotice text={t('noAccess')} /></div>
  const club = edit && fullClub(query.data?.club) ? query.data.club : undefined
  return <ClubForm key={`${user?.id}-${club ? `${club.id}-${club.version}` : 'new'}`} club={club} reload={() => void query.refetch()} />
}

function ClubForm({ club, reload }: { club?: Club; reload: () => void }) {
  const { t } = useI18n()
  const { user } = useSession()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const online = useOnline()
  const captureSession = useSessionGuard()
  const [form, setForm] = useState({ name: club?.name ?? '', city: club?.city ?? user?.profile.city ?? '', description: club?.description ?? '' })
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<unknown>()
  const [validation, setValidation] = useState('')
  async function submit(event: FormEvent) {
    const currentSession = captureSession()
    event.preventDefault(); setError(undefined); setValidation('')
    if (!form.name.trim() || !form.city.trim() || !form.description.trim()) { setValidation(t('requiredFields')); return }
    setBusy(true)
    try { const result = await mutation<{ club: Club }>(club ? `/clubs/${club.id}` : '/clubs', club ? 'PATCH' : 'POST', { name: form.name.trim(), city: form.city.trim(), description: form.description.trim(), ...(club ? { version: club.version } : {}) }); if (!currentSession()) return; await queryClient.invalidateQueries({ queryKey: ['clubs'] }); if (!currentSession()) return; queryClient.setQueryData(['club', result.club.id, user?.id], result); void navigate(`/clubs/${result.club.id}`) }
    catch (value) { setError(value) } finally { setBusy(false) }
  }
  return <div className="page-content form-page"><Link className="back-link" to={club ? `/clubs/${club.id}` : '/clubs'}><ArrowLeft size={17} />{t('backToClubs')}</Link><div className="page-heading"><span className="eyebrow">{t('clubsEyebrow')}</span><h1>{t(club ? 'clubEditTitle' : 'clubCreateTitle')}</h1><p>{t('clubFormText')}</p></div><form className="surface form-section" onSubmit={(event) => void submit(event)} noValidate><Field label={t('clubName')} value={form.name} onChange={(event) => setForm({ ...form, name: event.target.value })} maxLength={160} placeholder={t('clubNamePlaceholder')} required disabled={busy} /><CitySelector label={t('city')} value={form.city} onChange={(city) => setForm({ ...form, city })} maxLength={160} placeholder={t('cityPlaceholder')} required disabled={busy} /><TextArea label={t('clubDescription')} value={form.description} onChange={(description) => setForm({ ...form, description })} maxLength={2000} placeholder={t('clubDescriptionPlaceholder')} required /><div className="inline-note"><ShieldCheck size={20} /><p>{t('clubPreviewPrivacy')}</p></div>{validation && <ErrorNotice text={validation} />}{error != null && <ErrorNotice error={error} />}{error instanceof ApiError && error.code === 'stale_version' && <button className="text-button" type="button" onClick={reload}>{t('reloadLatest')}</button>}<div className="form-actions"><Link className="button button-outline" to={club ? `/clubs/${club.id}` : '/clubs'}>{t('cancel')}</Link><button className="button button-primary" type="submit" disabled={busy || !online}>{busy ? t('saving') : t(club ? 'saveClub' : 'createClub')}<ArrowRight size={17} /></button></div></form></div>
}

export function ClubDetailPage() {
  const { clubId = '' } = useParams()
  const { user, loading, error, refresh } = useSession()
  const { t } = useI18n()
  if (error) return <div className="page-content"><ErrorNotice text={t('serviceUnavailable')} retry={() => void refresh()} /></div>
  return <ClubDetail key={`${clubId}-${user?.id ?? 'guest'}`} clubId={clubId} sessionLoading={loading} />
}

function ClubDetail({ clubId, sessionLoading }: { clubId: string; sessionLoading: boolean }) {
  const { confirm, dialog } = useConfirmation()
  const { user } = useSession()
  const { t } = useI18n()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const online = useOnline()
  const captureSession = useSessionGuard()
  const [blocked, setBlocked] = useState(false)
  const [working, setWorking] = useState('')
  const [error, setError] = useState<unknown>()
  const key = ['club', clubId, user?.id ?? 'guest']
  const query = useQuery({
    queryKey: key, enabled: !sessionLoading, meta: { private: true }, refetchInterval: user ? 5000 : false,
    queryFn: async ({ signal }) => {
      const response = await request<{ club: Club | ClubSummary }>(`/clubs/${clubId}`, { signal })
      const result = user ? response : { club: publicClub(response.club) }
      const previous = queryClient.getQueryData<{ club: Club | ClubSummary }>(['club', clubId, user?.id ?? 'guest'])?.club
      if (!fullClub(result.club) || result.club.membership_state !== 'approved' || (previous?.can_manage && !result.club.can_manage)) {
        void queryClient.cancelQueries({ queryKey: ['club-members', clubId] })
        queryClient.removeQueries({ queryKey: ['club-members', clubId] })
        void queryClient.cancelQueries({ queryKey: ['club-image', clubId], predicate: (item) => item.queryKey[3] === 'owner' })
        queryClient.removeQueries({ queryKey: ['club-image', clubId], predicate: (item) => item.queryKey[3] === 'owner' })
      }
      setBlocked(false)
      return result
    },
  })
  const club = query.data?.club
  const canRoster = !!user && fullClub(club) && club.membership_state === 'approved' && !blocked && !query.isError
  const canManage = canRoster && club.can_manage === true && club.status === 'active'
  const accessLost = useCallback(() => {
    setBlocked(true)
    queryClient.setQueryData<{ club: ClubSummary | Club }>(['club', clubId, user?.id ?? 'guest'], (value) => value ? { club: publicClub(value.club) } : undefined)
    void queryClient.cancelQueries({ queryKey: ['club-members', clubId] })
    queryClient.removeQueries({ queryKey: ['club-members', clubId] })
    void queryClient.invalidateQueries({ queryKey: ['club', clubId] })
  }, [queryClient, clubId, user?.id])
  const members = useQuery({ queryKey: ['club-members', clubId, user?.id, canManage ? 'owner' : 'member'], queryFn: async ({ signal }) => {
    try { return await request<{ members: ClubMember[] }>(`/clubs/${clubId}/members`, { signal }) }
    catch (error) { if (error instanceof ApiError && [401, 403, 404].includes(error.status)) accessLost(); throw error }
  }, enabled: canRoster, meta: { private: true }, refetchInterval: canRoster ? 5000 : false })
  async function refresh() { await queryClient.invalidateQueries({ queryKey: ['club', clubId] }); await queryClient.invalidateQueries({ queryKey: ['club-members', clubId] }); await queryClient.invalidateQueries({ queryKey: ['clubs'] }) }
  async function membership(leave = false) {
    const currentSession = captureSession()
    if (!user) { void navigate(`/login?next=${encodeURIComponent(`/clubs/${clubId}`)}`); return }
    if (!user.onboarding_complete && !leave) { void navigate(`/profile?next=${encodeURIComponent(`/clubs/${clubId}`)}`); return }
    if (leave && !await confirm({ message: t('leaveClubConfirm'), action: t('leaveClub') })) return
    if (!currentSession()) return
    setWorking('membership'); setError(undefined)
    try { await mutation(`/clubs/${clubId}/membership`, leave ? 'DELETE' : 'PUT'); if (!currentSession()) return; if (leave) accessLost(); await refresh() }
    catch (value) { setError(value) } finally { setWorking('') }
  }
  async function decide(member: ClubMember, decision: 'approve' | 'reject' | 'remove') {
    const currentSession = captureSession()
    if (decision === 'remove' && !await confirm({ message: t('removeMemberConfirm'), action: t('remove'), destructive: true })) return
    if (!currentSession()) return
    setWorking(member.user_id); setError(undefined)
    try { await mutation(`/clubs/${clubId}/members/${member.user_id}/decision`, 'POST', { decision }); if (!currentSession()) return; await refresh() }
    catch (value) { setError(value); if (value instanceof ApiError && value.code === 'membership_changed') await refresh() }
    finally { setWorking('') }
  }
  async function closeClub() {
    const currentSession = captureSession()
    if (!fullClub(club) || !await confirm({ message: t('closeClubConfirm'), action: t('closeClub'), destructive: true }) || !currentSession()) return
    setWorking('close'); setError(undefined)
    try { const result = await mutation<{ club: Club | ClubSummary }>(`/clubs/${clubId}/close`, 'POST', { version: club.version }); if (!currentSession()) return; queryClient.setQueryData(key, result); queryClient.removeQueries({ queryKey: ['club-members', clubId] }); queryClient.removeQueries({ queryKey: ['club-image', clubId], predicate: (item) => item.queryKey[3] === 'owner' }); await refresh() }
    catch (value) { setError(value) } finally { setWorking('') }
  }
  if (sessionLoading || query.isPending) return <Loading />
  if (!club) return <div className="page-content"><ErrorNotice error={query.error} retry={() => void query.refetch()} /></div>
  const state = club.membership_state
  const approvedMembers = members.data?.members.filter((member) => member.state === 'approved') ?? []
  const requestedMembers = members.data?.members.filter((member) => member.state === 'requested') ?? []
  const canLeave = !!user && (state === 'approved' || state === 'requested') && !(club.status === 'active' && fullClub(club) && club.owner_id === user.id)
  return <div className="page-content club-detail-page">{dialog}<Link className="back-link" to="/clubs"><ArrowLeft size={17} />{t('backToClubs')}</Link><div className="collection-heading"><div className="club-detail-identity"><ClubEmblem club={club} variant="detail" ownerView={canManage} /><div className="page-heading"><span className="eyebrow">{t('clubPublicPreview')}</span><h1>{club.name}</h1><span className="city-label"><MapPin size={15} />{club.city}{club.status === 'closed' && <span className="status-pill cancelled">{t('clubClosed')}</span>}{canManage && <span className="status-pill">{t('clubOwner')}</span>}</span></div></div>{canManage && <Link className="button button-outline" to={`/clubs/${club.id}/edit`}><Pencil size={16} />{t('editClub')}</Link>}</div><p className="ride-description club-description">{club.description}</p><div className="content-safety"><ShareLink path={`/clubs/${club.id}`} /><ReportButton targetType="club" targetId={club.id} /><ClubImageReport club={club} /></div>
    {query.isError && <ErrorNotice error={query.error} retry={() => void query.refetch()} />}{error != null && <ErrorNotice error={error} retry={() => void refresh()} />}{club.status === 'closed' && <div className="cancelled-banner" role="status"><LockKeyhole size={18} />{t('clubClosedText')}</div>}
    {canManage && fullClub(club) && <ClubImageEditor club={club} />}
    {canRoster ? <section className="surface club-roster"><div className="panel-title"><span className="panel-icon"><Users size={20} /></span><h2>{t('clubMembers')}</h2>{fullClub(club) && <span className="count-pill">{club.member_count}</span>}</div>{members.isPending ? <Loading /> : members.isError ? <ErrorNotice error={members.error} retry={() => void members.refetch()} /> : <>
      {canManage && <div className="request-list"><h3>{t('clubJoinRequests')}<span>{requestedMembers.length}</span></h3>{requestedMembers.length ? requestedMembers.map((member) => <div className="participant request-participant" key={member.user_id}><span className="avatar">{initials(member.display_name)}</span><div className="participant-name"><strong>{member.display_name}</strong><span>{t('requested')}</span></div><div className="decision-buttons"><button className="icon-button approve-button" aria-label={`${t('approve')} ${member.display_name}`} onClick={() => void decide(member, 'approve')} disabled={!!working || !online}><Check size={18} /></button><button className="icon-button" aria-label={`${t('reject')} ${member.display_name}`} onClick={() => void decide(member, 'reject')} disabled={!!working || !online}><X size={18} /></button></div></div>) : <p className="small-empty">{t('noClubRequests')}</p>}</div>}
      <div className="club-members-list">{approvedMembers.length ? approvedMembers.map((member) => <div className="participant" key={member.user_id}><Avatar userId={member.user_id} name={member.display_name} scope={{ kind: 'club', id: club.id }} className={member.role === 'owner' ? 'avatar-orange' : ''} /><div className="participant-name"><strong>{member.display_name}{member.user_id === user?.id && <span className="self-label"> · {t('you')}</span>}</strong><CommunityBadge userId={member.user_id} /><UserSafety userId={member.user_id} name={member.display_name} scope={{ kind: 'club', id: club.id }} /><span>{t(member.role === 'owner' ? 'clubOwner' : 'clubMember')}</span></div>{canManage && member.role !== 'owner' && <button className="icon-button remove-button" aria-label={`${t('remove')} ${member.display_name}`} disabled={!!working || !online} onClick={() => void decide(member, 'remove')}><X size={15} /></button>}</div>) : <p className="small-empty">{t('noClubMembers')}</p>}</div>
    </>}</section> : <section className="surface club-access-card"><span className="access-icon">{state === 'requested' ? <Clock3 size={26} /> : <LockKeyhole size={26} />}</span><div><h2>{t(state === 'requested' ? 'clubPendingTitle' : state === 'rejected' ? 'clubRejected' : state === 'removed' ? 'clubRemoved' : 'clubMembership')}</h2><p>{t(state === 'requested' ? 'clubPendingText' : 'clubPreviewPrivacy')}</p></div>{club.status === 'active' && state !== 'requested' && state !== 'approved' && !query.isError && !blocked && <button className="button button-primary" disabled={!!working || !online} onClick={() => void membership()}>{t(user ? 'joinClub' : 'signInJoinClub')}<ArrowRight size={17} /></button>}</section>}
    {canManage && fullClub(club) && <ClubOwnerTools key={`${club.id}-${club.version}`} club={club} members={approvedMembers} close={closeClub} busy={!!working} refresh={refresh} />}
    <ClubRides club={club} canManage={canManage} />
    {canLeave && <div className="ride-lower-actions"><button className="text-button" onClick={() => void membership(true)} disabled={!!working || !online}><DoorOpen size={16} />{t('leaveClub')}</button></div>}
  </div>
}

function ClubOwnerTools({ club, members, close, busy, refresh }: { club: Club; members: ClubMember[]; close: () => Promise<void>; busy: boolean; refresh: () => Promise<void> }) {
  const { confirm, dialog } = useConfirmation()
  const { user } = useSession()
  const { t } = useI18n()
  const queryClient = useQueryClient()
  const online = useOnline()
  const captureSession = useSessionGuard()
  const [target, setTarget] = useState('')
  const [transferring, setTransferring] = useState(false)
  const [error, setError] = useState<unknown>()
  const candidates = members.filter((member) => member.user_id !== user?.id && member.role !== 'owner')
  async function transfer(event: FormEvent) {
    const currentSession = captureSession()
    event.preventDefault()
    const member = candidates.find((candidate) => candidate.user_id === target)
    if (!member || !await confirm({ message: t('transferConfirm', { name: member.display_name }), action: t('ownerTransfer') }) || !currentSession()) return
    setTransferring(true); setError(undefined)
    try { const result = await mutation<{ club: Club }>(`/clubs/${club.id}/ownership`, 'PUT', { version: club.version, user_id: target }); if (!currentSession()) return; queryClient.removeQueries({ queryKey: ['club-members', club.id] }); queryClient.setQueryData(['club', club.id, user?.id], result); queryClient.removeQueries({ queryKey: ['club-image', club.id], predicate: (item) => item.queryKey[3] === 'owner' }); await refresh() }
    catch (value) { setError(value) } finally { setTransferring(false) }
  }
  return <section className="surface owner-tools">{dialog}<div className="panel-title"><span className="panel-icon"><ShieldCheck size={20} /></span><h2>{t('clubOwnerTools')}</h2></div><p className="section-intro">{t('errorClubOwnerLeave')}</p><form className="ownership-form" onSubmit={(event) => void transfer(event)}><div><h3>{t('ownerTransfer')}</h3><p>{t('ownerTransferText')}</p></div>{candidates.length ? <><Field label={t('chooseNewOwner')}><select value={target} onChange={(event) => setTarget(event.target.value)} disabled={busy || transferring}><option value="">{t('chooseNewOwner')}</option>{candidates.map((member) => <option key={member.user_id} value={member.user_id}>{member.display_name}</option>)}</select></Field><button className="button button-outline" type="submit" disabled={!target || busy || transferring || !online}>{transferring ? t('saving') : t('ownerTransfer')}<ArrowRight size={16} /></button></> : <p className="small-empty">{t('noTransferMembers')}</p>}</form>{error != null && <ErrorNotice error={error} retry={() => void refresh()} />}<div className="close-club"><button className="text-button danger-text" onClick={() => void close()} disabled={busy || transferring || !online}><X size={16} />{t('closeClub')}</button></div></section>
}

export function rideIdFromLink(value: string, origin: string) {
  const pattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i
  const trimmed = value.trim()
  if (pattern.test(trimmed)) return trimmed
  try { const url = new URL(trimmed, origin); const id = url.pathname.match(/^\/rides\/([^/]+)\/?$/)?.[1]; return url.origin === origin && id && pattern.test(id) ? id : undefined } catch { return undefined }
}

function ClubRides({ club, canManage }: { club: ClubSummary; canManage: boolean }) {
  const { confirm, dialog } = useConfirmation()
  const { user } = useSession()
  const { t, locale } = useI18n()
  const queryClient = useQueryClient()
  const online = useOnline()
  const captureSession = useSessionGuard()
  const [selected, setSelected] = useState('')
  const [busy, setBusy] = useState('')
  const [error, setError] = useState<unknown>()
  const [validation, setValidation] = useState('')
  const [ownedCursors, setOwnedCursors] = useState([''])
  const ownedPage = ownedCursors.length - 1
  const ownedParams = new URLSearchParams({ view: 'organizing' })
  if (ownedCursors[ownedPage]) ownedParams.set('cursor', ownedCursors[ownedPage])
  const query = useQuery({ queryKey: ['club-rides', club.id], queryFn: ({ signal }) => request<{ rides: Ride[] }>(`/clubs/${club.id}/rides`, { signal }) })
  const owned = useQuery({ queryKey: ['linkable-rides', user?.id, ownedCursors[ownedPage]], queryFn: ({ signal }) => request<RidePage>(`/rides?${ownedParams}`, { signal }), enabled: canManage, meta: { private: true } })
  const choices = owned.data?.rides.filter((ride) => ride.can_manage && ride.status === 'published') ?? []
  async function link(event: FormEvent) {
    const currentSession = captureSession()
    event.preventDefault(); setError(undefined); setValidation('')
    const id = rideIdFromLink(selected, window.location.origin)
    if (!id) { setValidation(t('invalidRideLink')); return }
    setBusy('link')
    try { await mutation(`/clubs/${club.id}/rides/${id}`, 'PUT'); if (!currentSession()) return; setSelected(''); await queryClient.invalidateQueries({ queryKey: ['club-rides', club.id] }) }
    catch (value) { setError(value) } finally { setBusy('') }
  }
  async function unlink(ride: Ride) {
    const currentSession = captureSession()
    if (!await confirm({ message: t('unlinkConfirm'), action: t('removeLink') }) || !currentSession()) return
    setBusy(ride.id); setError(undefined)
    try { await mutation(`/clubs/${club.id}/rides/${ride.id}`, 'DELETE'); if (!currentSession()) return; await queryClient.invalidateQueries({ queryKey: ['club-rides', club.id] }) }
    catch (value) { setError(value) } finally { setBusy('') }
  }
  return <section className="club-rides-section">{dialog}<div className="section-header"><div><span className="eyebrow">{t('rides')}</span><h2>{t('linkedRides')}</h2></div></div><p className="section-intro">{t('linkedRidesText')}</p>{canManage && <form className="surface link-ride-form" onSubmit={(event) => void link(event)}><h3><Link2 size={18} />{t('linkRide')}</h3><p>{t('linkRideText')}</p>{owned.isPending && <Loading />}{owned.isError && <ErrorNotice error={owned.error} retry={() => void owned.refetch()} />}{choices.length > 0 && <Field label={t('chooseRide')}><select value={choices.some((ride) => ride.id === selected) ? selected : ''} onChange={(event) => setSelected(event.target.value)} disabled={!!busy}><option value="">{t('chooseRide')}</option>{choices.map((ride) => <option key={ride.id} value={ride.id}>{ride.city} · {ride.date} · {ride.description}</option>)}</select></Field>}<Pagination page={ownedPage} busy={owned.isFetching} hasNext={!!owned.data?.next_cursor} previous={() => setOwnedCursors((values) => values.slice(0, -1))} next={() => { if (owned.data?.next_cursor) setOwnedCursors((values) => [...values, owned.data.next_cursor]) }} /><div className="link-ride-controls"><Field label={t('rideLinkOrId')} value={selected} onChange={(event) => setSelected(event.target.value)} disabled={!!busy} /><button className="button button-outline" type="submit" disabled={!selected.trim() || !!busy || !online}>{t('linkRide')}<Plus size={16} /></button></div>{validation && <ErrorNotice text={validation} />}</form>}{error != null && <ErrorNotice error={error} />}
    {query.isPending ? <Loading /> : query.isError ? <ErrorNotice error={query.error} retry={() => void query.refetch()} /> : !query.data.rides.length ? <p className="small-empty surface linked-empty">{t('noLinkedRides')}</p> : <div className="ride-list">{query.data.rides.map((ride) => { const date = dateParts(ride.date, locale); return <article className="linked-ride" key={ride.id}><Link className="ride-card" to={`/rides/${ride.id}`}><div className="date-block"><strong>{date.day}</strong><span>{date.month}</span></div><div className="ride-card-copy"><span className="city-label"><MapPin size={14} />{ride.city}</span><h3>{ride.description}</h3><span className="card-policy"><ShieldCheck size={14} />{t('organizerApproval')}</span></div><ArrowUpRight size={21} /></Link>{canManage && <button className="text-button danger-text" disabled={!!busy || !online} onClick={() => void unlink(ride)}>{t('removeLink')}</button>}</article> })}</div>}
  </section>
}
