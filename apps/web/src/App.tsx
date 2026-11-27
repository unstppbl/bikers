import { lazy, Suspense, useEffect, useRef } from 'react'
import type { ReactNode } from 'react'
import { BellIcon as Bell, CompassIcon as Compass, MonitorIcon as Monitor, MoonIcon as Moon, MotorcycleIcon as Motorbike, PlusIcon as Plus, RoadHorizonIcon as Road, SunIcon as Sun, UserCircleIcon as UserRound, UsersThreeIcon as Users, XIcon as X } from '@phosphor-icons/react'
import { Link, matchRoutes, Navigate, useLocation, useNavigationType, useRoutes } from 'react-router-dom'
import { useRegisterSW } from 'virtual:pwa-register/react'
import { bindRecorderAccount } from './route-recorder'
import { newRidePollPath, ridePollsPath } from './ride-polls'
import { useI18n } from './i18n'
import { useSession, RequireUser } from './session'
import { useTheme } from './theme'
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
const NewRidePollPage = lazy(() => import('./pages/RidePolls').then((module) => ({ default: module.NewRidePollPage })))
const RidePollPage = lazy(() => import('./pages/RidePolls').then((module) => ({ default: module.RidePollPage })))
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

// 4-pillar navigation: Rides (discover + my rides + polls), Explore (map + routes), Community (clubs + bikes), You (profile + garage + inbox + settings).
type PageSection = 'rides' | 'explore' | 'community' | 'me'

interface AppRoute { path: string; section?: PageSection; element: ReactNode }

// Pages keyed by the account start fresh when another rider signs in.
function appRoutes(userId?: string): AppRoute[] {
  return [
    { path: '/', section: 'rides', element: <BrowsePage /> },
    { path: '/my-rides', section: 'rides', element: <RequireUser><MyRidesPage /></RequireUser> },
    { path: '/rides/new', section: 'rides', element: <RequireUser><RideFormPage /></RequireUser> },
    { path: newRidePollPath, section: 'rides', element: <RequireUser><NewRidePollPage key={userId} /></RequireUser> },
    { path: ridePollsPath + '/:pollId', section: 'rides', element: <RidePollPage /> },
    { path: '/rides/:rideId/edit', section: 'rides', element: <RequireUser><RideFormPage edit /></RequireUser> },
    { path: '/rides/:rideId', section: 'rides', element: <RideDetailPage /> },
    { path: '/explore', section: 'explore', element: <Navigate to="/places" replace /> },
    { path: '/places', section: 'explore', element: <PlacesPage /> },
    { path: '/places/new', section: 'explore', element: <RequireUser><PlaceSubmissionPage key={userId} /></RequireUser> },
    { path: '/places/:placeId/suggest', section: 'explore', element: <RequireUser><PlaceSubmissionPage key={userId} /></RequireUser> },
    { path: '/places/submissions', section: 'explore', element: <RequireUser><MyPlaceSubmissionsPage key={userId} /></RequireUser> },
    { path: '/routes', section: 'explore', element: <RoutesPage /> },
    { path: '/routes/new', section: 'explore', element: <RequireUser><RouteFormPage key={userId} /></RequireUser> },
    { path: '/routes/record', section: 'explore', element: <RequireUser><TrackRecorderPage key={userId} /></RequireUser> },
    { path: '/routes/:routeId/edit', section: 'explore', element: <RequireUser><RouteFormPage key={userId} edit /></RequireUser> },
    { path: '/routes/:routeId', section: 'explore', element: <RouteDetailPage /> },
    { path: '/community', section: 'community', element: <Navigate to="/clubs" replace /> },
    { path: '/clubs', section: 'community', element: <ClubsPage /> },
    { path: '/my-clubs', section: 'community', element: <RequireUser><MyClubsPage /></RequireUser> },
    { path: '/clubs/new', section: 'community', element: <RequireUser><ClubFormPage /></RequireUser> },
    { path: '/clubs/:clubId/edit', section: 'community', element: <RequireUser><ClubFormPage edit /></RequireUser> },
    { path: '/clubs/:clubId', section: 'community', element: <ClubDetailPage /> },
    { path: '/bikes/:bikeId', section: 'community', element: <PublicBikePage /> },
    { path: '/bikes', section: 'community', element: <BikesPage /> },
    { path: '/me', section: 'me', element: <MePage /> },
    { path: '/garage', section: 'me', element: <RequireUser><GaragePage key={userId} /></RequireUser> },
    { path: '/garage/new', section: 'me', element: <RequireUser><BikeFormPage /></RequireUser> },
    { path: '/garage/:bikeId/edit', section: 'me', element: <RequireUser><BikeFormPage edit /></RequireUser> },
    { path: '/garage/:bikeId', section: 'me', element: <RequireUser><BikeOwnerPage /></RequireUser> },
    { path: '/profile', section: 'me', element: <RequireUser allowOnboarding><ProfilePage /></RequireUser> },
    { path: '/inbox', section: 'me', element: <RequireUser><InboxPage key={userId} /></RequireUser> },
    { path: '/admin', section: 'me', element: <RequireUser><AdminPage key={userId} /></RequireUser> },
    { path: '/settings', section: 'me', element: <RequireUser allowOnboarding><SettingsPage /></RequireUser> },
    { path: '/install', section: 'me', element: <InstallPage /> },
    { path: '/login', element: <LoginPage /> },
    ...RideFlowPreview ? [{ path: '/preview/ride-flow', element: <RideFlowPreview /> }] : [],
    { path: '*', element: <NotFoundPage /> },
  ]
}

// The section of the route that renders the path, so unknown paths, nested ones included, select nothing.
export function pageSection(pathname: string) {
  return matchRoutes(appRoutes(), pathname)?.[0]?.route.section
}

function NotFoundPage() {
  const { t } = useI18n()
  return <div className="page-content not-found-page"><Motorbike size={56} /><div className="page-heading"><h1>{t('notFound')}</h1></div><Link className="button button-primary" to="/">{t('back')}</Link></div>
}

export function App() {
  const { t, locale, setLocale } = useI18n()
  const { mode, cycleMode } = useTheme()
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
  const tabIcon = (name: PageSection) => section === name ? 'fill' as const : 'regular' as const
  const themeLabel = t(mode === 'light' ? 'themeLight' : mode === 'dark' ? 'themeDark' : 'themeAuto')
  const ThemeIcon = mode === 'light' ? Sun : mode === 'dark' ? Moon : Monitor
  return <div className={`app-shell${mapWorkspace ? ' app-shell-map' : ''}`}>
    <a className="skip-link" href="#main-content">{t('skipToContent')}</a>
    <header className="topbar" ref={topbarRef}>
      <div className="topbar-inner">
        <Link className="wordmark" to="/" aria-label="BIKERS.KZ"><span>BIKERS</span><span className="wordmark-plate">KZ</span></Link>
        <nav className="primary-nav" aria-label={t('navMain')}>
          <Link aria-current={current('rides')} to="/">{t('rides')}</Link>
          <Link aria-current={current('explore')} to="/places">{t('tabExplore')}</Link>
          <Link aria-current={current('community')} to="/clubs">{t('tabCommunity')}</Link>
          <Link aria-current={current('me')} to="/me">{t('tabMe')}</Link>
        </nav>
        <div className="topbar-actions">
          <button type="button" className="icon-button topbar-theme" onClick={cycleMode} aria-label={t('themeToggle', { mode: themeLabel })} title={themeLabel}><ThemeIcon size={20} /></button>
          <label className="locale-select"><span className="sr-only">{t('locale')}</span><select aria-label={t('locale')} value={locale} onChange={(event) => setLocale(event.target.value as Locale)}><option value="en">EN</option><option value="ru">RU</option><option value="kk">KZ</option></select></label>
          {user && <Link to="/inbox" className="icon-button topbar-inbox" aria-label={unreadCount ? t('notificationUnreadLabel', { count: unreadCount }) : t('inbox')}><Bell size={20} /><span aria-hidden="true"><NotificationBadge count={unreadCount} /></span></Link>}
          {user ? <Link to="/me" className="avatar-link topbar-avatar" aria-label={t('meHubLabel')} title={user.profile.display_name}><Avatar userId={user.id} name={user.profile.display_name} /></Link> : <Link className="topbar-login" to="/login">{t('signIn')}</Link>}
          <Link className="button button-primary topbar-create" to="/rides/new" aria-label={t('newRideLabel')}><Plus size={17} weight="bold" />{t('newRideShort')}</Link>
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
      <Link className="tab-link" aria-current={current('rides')} to="/"><Road size={22} weight={tabIcon('rides')} /><span>{t('rides')}</span></Link>
      <Link className="tab-link" aria-current={current('explore')} to="/places"><Compass size={22} weight={tabIcon('explore')} /><span>{t('tabExplore')}</span></Link>
      <Link className="tab-link" aria-current={current('community')} to="/clubs"><Users size={22} weight={tabIcon('community')} /><span>{t('tabCommunity')}</span></Link>
      <Link className="tab-link" aria-current={current('me')} to="/me"><span className="tab-me-icon"><UserRound size={22} weight={tabIcon('me')} />{unreadCount > 0 && <span className="tab-dot" aria-hidden="true" />}</span><span>{t('tabMe')}</span></Link>
    </nav>
  </div>
}
