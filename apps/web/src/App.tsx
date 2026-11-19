import { lazy, Suspense, useEffect, useRef } from 'react'
import type { ReactNode } from 'react'
import { BellIcon as Bell, MapTrifoldIcon as MapIcon, MotorcycleIcon as Motorbike, PlusIcon as Plus, RoadHorizonIcon as Road, UserCircleIcon as UserRound, UsersThreeIcon as Users, XIcon as X } from '@phosphor-icons/react'
import { Link, matchRoutes, useLocation, useNavigationType, useRoutes } from 'react-router-dom'
import { useRegisterSW } from 'virtual:pwa-register/react'
import { bindRecorderAccount } from './route-recorder'
import { useI18n } from './i18n'
import { useSession, RequireUser } from './session'
import { Loading, useOnline } from './ui'
import { useRevealFocus } from './useRevealFocus'
import { LoginPage, ProfilePage } from './pages/Auth'
import { BrowsePage, RideFormPage } from './pages/Rides'
import { RideDetailPage } from './pages/RideDetail'
import { MyRidesPage } from './pages/RideList'
import { MePage } from './pages/Me'
import { Avatar } from './components/Avatar'
import { NotificationBadge, useUnreadNotifications } from './components/NotificationBadge'
import type { Locale } from './types'

const RoutesPage = lazy(() => import('./pages/Routes').then((module) => ({ default: module.RoutesPage })))
const RouteDetailPage = lazy(() => import('./pages/Routes').then((module) => ({ default: module.RouteDetailPage })))
const RouteFormPage = lazy(() => import('./pages/Routes').then((module) => ({ default: module.RouteFormPage })))
const TrackRecorderPage = lazy(() => import('./pages/Routes').then((module) => ({ default: module.TrackRecorderPage })))

const PlacesPage = lazy(() => import('./pages/Places').then((module) => ({ default: module.PlacesPage })))
const PlaceSubmissionPage = lazy(() => import('./pages/PlaceSubmissions').then((module) => ({ default: module.PlaceSubmissionPage })))
const MyPlaceSubmissionsPage = lazy(() => import('./pages/PlaceSubmissions').then((module) => ({ default: module.MyPlaceSubmissionsPage })))
const GaragePage = lazy(() => import('./pages/Garage').then((module) => ({ default: module.GaragePage })))
const BikeFormPage = lazy(() => import('./pages/Garage').then((module) => ({ default: module.BikeFormPage })))
const BikeOwnerPage = lazy(() => import('./pages/Garage').then((module) => ({ default: module.BikeOwnerPage })))
const PublicBikePage = lazy(() => import('./pages/Garage').then((module) => ({ default: module.PublicBikePage })))
const BikesPage = lazy(() => import('./pages/Bikes').then((module) => ({ default: module.BikesPage })))
const ClubsPage = lazy(() => import('./pages/Clubs').then((module) => ({ default: module.ClubsPage })))
const MyClubsPage = lazy(() => import('./pages/Clubs').then((module) => ({ default: module.MyClubsPage })))
const ClubFormPage = lazy(() => import('./pages/Clubs').then((module) => ({ default: module.ClubFormPage })))
const ClubDetailPage = lazy(() => import('./pages/Clubs').then((module) => ({ default: module.ClubDetailPage })))
const InboxPage = lazy(() => import('./pages/Inbox').then((module) => ({ default: module.InboxPage })))
const AdminPage = lazy(() => import('./pages/Admin').then((module) => ({ default: module.AdminPage })))
const SettingsPage = lazy(() => import('./pages/Settings').then((module) => ({ default: module.SettingsPage })))
const InstallPage = lazy(() => import('./pages/Install').then((module) => ({ default: module.InstallPage })))
const RideFlowPreview = import.meta.env.DEV
  ? lazy(() => import('./pages/RideFlowPreview').then((module) => ({ default: module.RideFlowPreview })))
  : null

// The part of the app a page belongs to. The top bar links Map, Routes and Motorcycles separately. The phone tab bar
// groups Routes under Map, and keeps Me current on Motorcycles and the other pages reached from the Me page.
type PageSection ='rides' | 'create' | 'places' | 'routes' | 'clubs' | 'bikes' | 'me'
type TabSection = 'rides' | 'map' | 'create' | 'clubs' | 'me'
const sectionTab: Record<PageSection, TabSection> = { rides: 'rides', create: 'create', places: 'map', routes: 'map', clubs: 'clubs', bikes: 'me', me: 'me' }

interface AppRoute { path: string; section?: PageSection; element: ReactNode }

// Pages keyed by the account start fresh when another rider signs in.
function appRoutes(userId?: string): AppRoute[] {
  return [
    { path: '/', section: 'rides', element: <BrowsePage /> },
    { path: '/me', section: 'me', element: <MePage /> },
    { path: '/my-rides', section: 'me', element: <RequireUser><MyRidesPage /></RequireUser> },
    { path: '/login', element: <LoginPage /> },
    { path: '/profile', section: 'me', element: <RequireUser allowOnboarding><ProfilePage /></RequireUser> },
    { path: '/inbox', section: 'me', element: <RequireUser><InboxPage key={userId} /></RequireUser> },
    { path: '/routes', section: 'routes', element: <RoutesPage /> },
    { path: '/routes/new', section: 'routes', element: <RequireUser><RouteFormPage key={userId} /></RequireUser> },
    { path: '/routes/record', section: 'routes', element: <RequireUser><TrackRecorderPage key={userId} /></RequireUser> },
    { path: '/routes/:routeId/edit', section: 'routes', element: <RequireUser><RouteFormPage key={userId} edit /></RequireUser> },
    { path: '/routes/:routeId', section: 'routes', element: <RouteDetailPage /> },
    { path: '/places', section: 'places', element: <PlacesPage /> },
    { path: '/places/new', section: 'places', element: <RequireUser><PlaceSubmissionPage key={userId} /></RequireUser> },
    { path: '/places/:placeId/suggest', section: 'places', element: <RequireUser><PlaceSubmissionPage key={userId} /></RequireUser> },
    { path: '/places/submissions', section: 'places', element: <RequireUser><MyPlaceSubmissionsPage key={userId} /></RequireUser> },
    { path: '/admin', section: 'me', element: <RequireUser><AdminPage key={userId} /></RequireUser> },
    { path: '/settings', section: 'me', element: <RequireUser allowOnboarding><SettingsPage /></RequireUser> },
    { path: '/install', section: 'me', element: <InstallPage /> },
    ...RideFlowPreview ? [{ path: '/preview/ride-flow', element: <RideFlowPreview /> }] : [],
    { path: '/rides/new', section: 'create', element: <RequireUser><RideFormPage /></RequireUser> },
    { path: '/rides/:rideId/edit', section: 'rides', element: <RequireUser><RideFormPage edit /></RequireUser> },
    { path: '/rides/:rideId', section: 'rides', element: <RideDetailPage /> },
    { path: '/garage', section: 'me', element: <RequireUser><GaragePage key={userId} /></RequireUser> },
    { path: '/garage/new', section: 'me', element: <RequireUser><BikeFormPage /></RequireUser> },
    { path: '/garage/:bikeId/edit', section: 'me', element: <RequireUser><BikeFormPage edit /></RequireUser> },
    { path: '/garage/:bikeId', section: 'me', element: <RequireUser><BikeOwnerPage /></RequireUser> },
    { path: '/bikes/:bikeId', section: 'bikes', element: <PublicBikePage /> },
    { path: '/bikes', section: 'bikes', element: <BikesPage /> },
    { path: '/clubs', section: 'clubs', element: <ClubsPage /> },
    { path: '/my-clubs', section: 'me', element: <RequireUser><MyClubsPage /></RequireUser> },
    { path: '/clubs/new', section: 'clubs', element: <RequireUser><ClubFormPage /></RequireUser> },
    { path: '/clubs/:clubId/edit', section: 'clubs', element: <RequireUser><ClubFormPage edit /></RequireUser> },
    { path: '/clubs/:clubId', section: 'clubs', element: <ClubDetailPage /> },
    { path: '*', element: <NotFoundPage /> },
  ]
}

// The section of the route that renders the path, so unknown paths, nested ones included, select nothing.
export function pageSection(pathname: string) {
  return matchRoutes(appRoutes(), pathname)?.[0]?.route.section
}

function NotFoundPage() {
  const { t } = useI18n()
  return <div className="page-content not-found-page"><Motorbike size={64} /><div className="page-heading"><h1>{t('notFound')}</h1></div><Link className="button button-primary" to="/">{t('back')}</Link></div>
}

export function App() {
  const { t, locale, setLocale } = useI18n()
  const { user, loading: sessionLoading } = useSession()
  useEffect(() => { if (!sessionLoading) bindRecorderAccount(user?.id ?? null) }, [user?.id, sessionLoading])
  const unread = useUnreadNotifications()
  const unreadCount = user && !unread.isError ? unread.data ?? 0 : 0
  const online = useOnline()
  const location = useLocation()
  const mapWorkspace = /^\/places\/?$/.test(location.pathname)
  const navigationType = useNavigationType()
  const mainRef = useRef<HTMLElement>(null)
  const { topbarRef, tabBarRef } = useRevealFocus()
  const previousLocation = useRef(location)
  const { needRefresh: [needRefresh, setNeedRefresh], updateServiceWorker } = useRegisterSW()
  useEffect(() => {
    const previous = previousLocation.current
    previousLocation.current = location
    // Preserve browser Back/Forward restoration and same-page native anchors.
    if (previous === location || navigationType === 'POP') return
    if (previous.pathname === location.pathname && (previous.search !== location.search || location.hash)) return
    const target = location.hash ? document.getElementById(location.hash.slice(1)) : null
    if (target) {
      target.focus({ preventScroll: true })
      target.scrollIntoView()
      return
    }
    window.scrollTo(0, 0)
    mainRef.current?.focus({ preventScroll: true })
  }, [location, navigationType])
  const routes = appRoutes(user?.id)
  const page = useRoutes(routes)
  const section = matchRoutes(routes, location)?.[0]?.route.section
  const current = (name: PageSection) => section === name ? 'page' as const : undefined
  const currentTab = (name: TabSection) => section && sectionTab[section] === name ? 'page' as const : undefined
  // The current tab also fills its icon, so it is not shown by colour alone. The create plate gets a rim instead.
  const tabIcon = (name: TabSection) => currentTab(name) ? 'fill' as const : 'regular' as const
  return <div className={`app-shell${mapWorkspace ? ' app-shell-map' : ''}`}>
    <a className="skip-link" href="#main-content">{t('skipToContent')}</a>
    <header className="topbar" ref={topbarRef}>
      <div className="topbar-inner">
        <Link className="wordmark" to="/" aria-label="BIKERS.KZ"><span>BIKERS</span><span className="wordmark-plate">KZ</span></Link>
        <nav className="primary-nav" aria-label={t('navMain')}>
          <Link aria-current={current('rides')} to="/">{t('rides')}</Link>
          <Link aria-current={current('places')} to="/places">{t('placesTitle')}</Link>
          <Link aria-current={current('routes')} to="/routes">{t('routesTitle')}</Link>
          <Link aria-current={current('clubs')} to="/clubs">{t('clubs')}</Link>
          <Link aria-current={current('bikes')} to="/bikes">{t('bikeCatalogue')}</Link>
        </nav>
        <div className="topbar-actions">
          <label className="locale-select"><span className="sr-only">{t('locale')}</span><select aria-label={t('locale')} value={locale} onChange={(event) => setLocale(event.target.value as Locale)}><option value="en">EN</option><option value="ru">RU</option><option value="kk">KZ</option></select></label>
          {user && <Link to="/inbox" className="icon-button topbar-inbox" aria-label={unreadCount ? t('notificationUnreadLabel', { count: unreadCount }) : t('inbox')}><Bell size={22} /><span aria-hidden="true"><NotificationBadge count={unreadCount} /></span></Link>}
          {user ? <Link to="/me" className="avatar-link" aria-label={t('meHubLabel')} title={user.profile.display_name}><Avatar userId={user.id} name={user.profile.display_name} /></Link> : <Link className="topbar-login" to="/login">{t('signIn')}</Link>}
          <Link className="button button-primary topbar-create" to="/rides/new" aria-label={t('newRideLabel')}><Plus size={18} weight="bold" />{t('newRideShort')}</Link>
        </div>
      </div>
    </header>
    {!online && <div className="connection-banner" role="status">{t('offline')}</div>}
    {needRefresh && <div className="update-banner" role="status"><span>{t('updateReady')}</span><button className="text-button" onClick={() => void updateServiceWorker(true)}>{t('updateNow')}</button><button className="icon-button" aria-label={t('later')} onClick={() => setNeedRefresh(false)}><X size={17} /></button></div>}
    <main id="main-content" ref={mainRef} tabIndex={-1}>
      <Suspense fallback={<Loading />}>{page}</Suspense>
    </main>
    <footer className="app-footer"><Link className="wordmark" to="/" aria-label="BIKERS.KZ"><span>BIKERS</span><span className="wordmark-plate">KZ</span></Link><span>{t('sidebarNote')}</span></footer>
    <nav className="tab-bar" ref={tabBarRef} aria-label={t('navTabs')}>
      <Link className="tab-link" aria-current={currentTab('rides')} to="/"><Road size={24} weight={tabIcon('rides')} /><span>{t('rides')}</span></Link>
      <Link className="tab-link" aria-current={currentTab('map')} to="/places"><MapIcon size={24} weight={tabIcon('map')} /><span>{t('placesTitle')}</span></Link>
      <Link className="tab-link tab-create" aria-current={currentTab('create')} to="/rides/new" aria-label={t('newRideLabel')}><span className="tab-create-plate"><Plus size={24} weight="bold" /></span><span>{t('newRideShort')}</span></Link>
      <Link className="tab-link" aria-current={currentTab('clubs')} to="/clubs"><Users size={24} weight={tabIcon('clubs')} /><span>{t('clubs')}</span></Link>
      <Link className="tab-link" aria-current={currentTab('me')} to="/me"><span className="tab-me-icon"><UserRound size={24} weight={tabIcon('me')} />{unreadCount > 0 && <span className="tab-dot" aria-hidden="true" />}</span><span>{t('tabMe')}</span></Link>
    </nav>
  </div>
}
