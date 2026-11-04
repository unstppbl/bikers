import { useState } from 'react'
import type { FormEvent } from 'react'
import { useQuery } from '@tanstack/react-query'
import { ArrowUpRightIcon as ArrowUpRight, WrenchIcon as Wrench } from '@phosphor-icons/react'
import { Link, Navigate, useLocation, useNavigate, useSearchParams } from 'react-router-dom'
import { request } from '../api'
import { BikeCover } from '../components/BikePhotos'
import { useI18n } from '../i18n'
import { useSession } from '../session'
import type { PublicBike } from '../types'
import { EmptyState, ErrorNotice, Field, Loading, Pagination } from '../ui'

interface BikePage { bikes: PublicBike[]; next_cursor: string }
interface BikePaging { scope: string; cursors: string[] }

// Catalogue responses always retain the public projection, including for owners.
const publicBike = ({ id, make, model, model_id, year }: PublicBike): PublicBike => ({ id, make, model, model_id, year })

export function BikesPage() {
  const { loading, error, refresh } = useSession()
  const { t } = useI18n()
  if (loading) return <Loading />
  if (error) return <div className="page-content"><ErrorNotice text={t('serviceUnavailable')} retry={() => void refresh()} /></div>
  return <div className="page-content collection-page"><div className="collection-heading"><div className="page-heading"><span className="eyebrow">{t('bikeCatalogueEyebrow')}</span><h1>{t('bikeCatalogueTitle')}</h1><p>{t('bikeCatalogueText')}</p></div><Link className="button button-outline" to="/garage"><Wrench size={17} />{t('garage')}</Link></div><BikeList /></div>
}

function BikeList() {
  const { user } = useSession()
  const { t } = useI18n()
  const [params] = useSearchParams()
  const location = useLocation()
  const navigate = useNavigate()
  const q = params.get('q') ?? ''
  const cursor = params.get('cursor') ?? ''
  const scope = JSON.stringify([user?.id ?? 'guest', q])
  const stored = (location.state as { bikePaging?: BikePaging } | null)?.bikePaging
  const staleCursor = !!cursor && !!stored && stored.scope !== scope
  const cursors = stored?.scope === scope && stored.cursors.at(-1) === cursor ? stored.cursors : cursor ? ['', cursor] : ['']
  const page = cursors.length - 1
  const queryParams = new URLSearchParams()
  if (q) queryParams.set('q', q)
  if (cursor) queryParams.set('cursor', cursor)
  const query = useQuery({
    queryKey: ['public-bikes', user?.id ?? 'guest', q, cursor],
    enabled: !staleCursor,
    queryFn: async ({ signal }) => {
      const result = await request<BikePage>(`/bikes${queryParams.size ? `?${queryParams}` : ''}`, { signal })
      return { bikes: result.bikes.map(publicBike), next_cursor: result.next_cursor }
    },
    meta: { private: true },
  })
  function goToCursor(nextCursors: string[]) {
    const next = new URLSearchParams(params)
    const current = nextCursors.at(-1) ?? ''
    if (current) next.set('cursor', current)
    else next.delete('cursor')
    void navigate({ pathname: location.pathname, search: next.toString() }, { state: { bikePaging: { scope, cursors: nextCursors } } })
  }
  if (staleCursor) {
    const next = new URLSearchParams(params); next.delete('cursor')
    return <Navigate to={{ pathname: location.pathname, search: next.toString() }} replace />
  }
  return <section aria-label={t('bikeCatalogue')}>
    <BikeSearch key={q} query={q} />
    <div className="section-header"><h2>{t('publishedBikes')}</h2>{query.data && <span className="count-pill">{t('bikesOnPage', { count: query.data.bikes.length })}</span>}</div>
    {query.isPending ? <Loading /> : query.isError ? <><ErrorNotice error={query.error} retry={() => void query.refetch()} />{cursor && <button type="button" className="text-button" onClick={() => goToCursor([''])}>{t('firstBikePage')}</button>}</> : !query.data.bikes.length ? <EmptyState title={t(q || cursor ? 'noResults' : 'bikeCatalogueEmptyTitle')} text={t(q || cursor ? 'bikeCatalogueFilteredEmpty' : 'bikeCatalogueEmptyText')} action={<Link className="button button-outline" to={q || cursor ? '/bikes' : '/garage'}>{t(q || cursor ? 'clearFilters' : 'garage')}<ArrowUpRight size={17} /></Link>} /> : <div className="bike-grid">{query.data.bikes.map((bike) => <PublicBikeCard key={bike.id} bike={bike} />)}</div>}
    {!query.isError && <Pagination page={page} hasNext={!!query.data?.next_cursor} busy={query.isFetching} previous={() => goToCursor(cursors.slice(0, -1))} next={() => { if (query.data?.next_cursor) goToCursor([...cursors, query.data.next_cursor]) }} />}
  </section>
}

function BikeSearch({ query }: { query: string }) {
  const { t } = useI18n()
  const [params, setParams] = useSearchParams()
  const [draft, setDraft] = useState(query)
  function apply(value: string) {
    const next = new URLSearchParams(params)
    next.delete('cursor')
    if (value.trim()) next.set('q', value.trim())
    else next.delete('q')
    setParams(next)
  }
  function submit(event: FormEvent) { event.preventDefault(); apply(draft) }
  return <form className="ride-filters bike-catalogue-filters" aria-label={t('bikeFilters')} onSubmit={submit}>
    <div className="ride-filter-fields"><Field label={t('bikeSearch')} type="search" value={draft} onChange={(event) => setDraft(event.target.value)} maxLength={200} placeholder={t('bikeSearchPlaceholder')} /></div>
    <div className="ride-filter-actions"><button className="button button-dark" type="submit">{t('applyFilters')}</button><button className="text-button" type="button" onClick={() => { setDraft(''); apply('') }}>{t('clearFilters')}</button></div>
  </form>
}

function PublicBikeCard({ bike }: { bike: PublicBike }) {
  const { t } = useI18n()
  return <article className="surface bike-card catalogue-bike-card"><Link className="bike-card-main" to={`/bikes/${bike.id}`}><div className="bike-card-top"><span className="eyebrow">{bike.make}</span></div><div className="bike-identity"><h2>{bike.model}</h2><span>{bike.year}</span></div></Link><BikeCover bike={bike} variant="card" /><Link className="bike-card-bottom" to={`/bikes/${bike.id}`}><span>{t('viewMotorcycle')}</span><ArrowUpRight size={21} /></Link></article>
}
