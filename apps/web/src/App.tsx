import { lazy, Suspense, useEffect, useId, useRef, useState } from 'react'
import { ArrowUpRight, Bell, LogOut, Menu, Motorbike, Plus, Settings, ShieldCheck, UserRound, Users, Wrench, X, Zap } from 'lucide-react'
import { Link, NavLink, Route, Routes, useLocation, useNavigate, useNavigationType } from 'react-router-dom'
import { useRegisterSW } from 'virtual:pwa-register/react'
import { mutation } from './api'
import { useI18n } from './i18n'
import { useSession, useSessionGuard, RequireUser } from './session'
import { ErrorNotice, Loading, useOnline } from './ui'
import { LoginPage, ProfilePage } from './pages/Auth'
import { BrowsePage, RideFormPage } from './pages/Rides'
import { RideDetailPage } from './pages/RideDetail'
import { Avatar } from './components/Avatar'
import { useTrust } from './components/Trust'
import type { Locale } from './types'

const GaragePage = lazy(() => import('./pages/Garage').then((module) => ({ default: module.GaragePage })))
const BikeFormPage = lazy(() => import('./pages/Garage').then((module) => ({ default: module.BikeFormPage })))
const BikeOwnerPage = lazy(() => import('./pages/Garage').then((module) => ({ default: module.BikeOwnerPage })))
const PublicBikePage = lazy(() => import('./pages/Garage').then((module) => ({ default: module.PublicBikePage })))
const ClubsPage = lazy(() => import('./pages/Clubs').then((module) => ({ default: module.ClubsPage })))
const ClubFormPage = lazy(() => import('./pages/Clubs').then((module) => ({ default: module.ClubFormPage })))
const ClubDetailPage = lazy(() => import('./pages/Clubs').then((module) => ({ default: module.ClubDetailPage })))
const InboxPage = lazy(() => import('./pages/Inbox').then((module) => ({ default: module.InboxPage })))
const AdminPage = lazy(() => import('./pages/Admin').then((module) => ({ default: module.AdminPage })))
const SettingsPage = lazy(() => import('./pages/Settings').then((module) => ({ default: module.SettingsPage })))
const InstallPage = lazy(() => import('./pages/Install').then((module) => ({ default: module.InstallPage })))
const RideFlowPreview = import.meta.env.DEV
  ? lazy(() => import('./pages/RideFlowPreview').then((module) => ({ default: module.RideFlowPreview })))
  : null

export function App() {
  const { t, locale, setLocale } = useI18n()
  const { user, clear } = useSession()
  const trust = useTrust()
  const captureSession = useSessionGuard()
  const [menuOpen, setMenuOpen] = useState(false)
  const [logoutError, setLogoutError] = useState<unknown>()
  const [signingOut, setSigningOut] = useState(false)
  const online = useOnline()
  const navigate = useNavigate()
  const location = useLocation()
  const navigationType = useNavigationType()
  const navigationId = useId()
  const mainRef = useRef<HTMLElement>(null)
  const sidebarRef = useRef<HTMLElement>(null)
  const menuTriggerRef = useRef<HTMLButtonElement>(null)
  const menuCloseRef = useRef<HTMLButtonElement>(null)
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
  useEffect(() => {
    if (!menuOpen) return
    const previousOverflow = document.body.style.overflow
    const trigger = menuTriggerRef.current
    document.body.style.overflow = 'hidden'
    menuCloseRef.current?.focus()
    function keydown(event: KeyboardEvent) {
      if (event.key === 'Escape') { event.preventDefault(); setMenuOpen(false); return }
      if (event.key !== 'Tab') return
      const controls = sidebarRef.current?.querySelectorAll<HTMLElement>('a[href],button:not(:disabled),[tabindex]:not([tabindex="-1"])')
      if (!controls?.length) return
      const first = controls[0], last = controls[controls.length - 1]
      if (!sidebarRef.current?.contains(document.activeElement)) { event.preventDefault(); first.focus() }
      else if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus() }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
    }
    const close = () => setMenuOpen(false)
    const resize = () => { if (window.innerWidth > 760) close() }
    document.addEventListener('keydown', keydown)
    window.addEventListener('popstate', close)
    window.addEventListener('resize', resize)
    return () => {
      document.removeEventListener('keydown', keydown)
      window.removeEventListener('popstate', close)
      window.removeEventListener('resize', resize)
      document.body.style.overflow = previousOverflow
      if (trigger?.isConnected) trigger.focus({ preventScroll: true })
    }
  }, [menuOpen])
  async function signOut() {
    const currentSession = captureSession()
    setSigningOut(true)
    setLogoutError(undefined)
    try { await mutation('/session', 'DELETE'); if (!currentSession()) return; clear(); setMenuOpen(false); void navigate('/') }
    catch (error) { setLogoutError(error) }
    finally { setSigningOut(false) }
  }
  return <div className="app-shell">
    <aside id={navigationId} ref={sidebarRef} className={`sidebar ${menuOpen ? 'sidebar-open' : ''}`} aria-label={t('menu')} role={menuOpen ? 'dialog' : undefined} aria-modal={menuOpen || undefined}>
      <Link className="wordmark" to="/" onClick={() => setMenuOpen(false)}>BIKERS<span>.KZ</span><span className="wordmark-mark"><Zap size={19} fill="currentColor" /></span></Link>
      <button ref={menuCloseRef} className="icon-button sidebar-close" aria-label={t('close')} onClick={() => setMenuOpen(false)}><X size={23} /></button>
      <div className="sidebar-label">{t('community')}</div>
      <nav className="side-navigation">
        <NavLink to="/" end onClick={() => setMenuOpen(false)}><Motorbike size={20} />{t('rides')}<ArrowUpRight className="nav-arrow" size={17} /></NavLink>
        <NavLink to="/garage" onClick={() => setMenuOpen(false)}><Wrench size={20} />{t('garage')}</NavLink>
        <NavLink to="/clubs" onClick={() => setMenuOpen(false)}><Users size={20} />{t('clubs')}</NavLink>
        <NavLink to="/inbox" onClick={() => setMenuOpen(false)}><Bell size={20} />{t('inbox')}</NavLink>
        {trust.data?.is_admin && <NavLink to="/admin" onClick={() => setMenuOpen(false)}><ShieldCheck size={20} />{t('admin')}</NavLink>}
        <NavLink to="/profile" onClick={() => setMenuOpen(false)}><UserRound size={20} />{t('profile')}</NavLink>
        <NavLink to="/settings" onClick={() => setMenuOpen(false)}><Settings size={20} />{t('settings')}</NavLink>
      </nav>
      <div className="sidebar-bottom"><span className="small-marker" /><p>{t('together')}</p><span className="sidebar-caption">{t('country')}<span>↗</span></span></div>
      {user && <button className="sidebar-logout" onClick={() => void signOut()} disabled={signingOut || !online}><LogOut size={17} />{t('signOut')}</button>}
    </aside>
    {menuOpen && <button aria-label={t('close')} aria-hidden="true" tabIndex={-1} className="menu-scrim" onClick={() => setMenuOpen(false)} />}
    <div className="workspace" inert={menuOpen} aria-hidden={menuOpen || undefined}>
      <header className="topbar">
        <div className="mobile-brand"><button ref={menuTriggerRef} className="icon-button" aria-label={t('menu')} aria-controls={navigationId} aria-expanded={menuOpen} aria-haspopup="dialog" onClick={() => setMenuOpen(true)}><Menu size={22} /></button><Link to="/">BIKERS<span>.KZ</span></Link></div>
        <span className="topbar-country"><span className="status-dot" />{t('country')}<span className="topbar-divider">/</span>{t('community')}</span>
        <div className="topbar-actions"><label className="locale-select"><span className="sr-only">{t('locale')}</span><select aria-label={t('locale')} value={locale} onChange={(event) => setLocale(event.target.value as Locale)}><option value="en">EN</option><option value="ru">RU</option><option value="kk">KZ</option></select></label>
          {user ? <Link to="/profile" className="avatar-link" aria-label={t('profile')} title={user.profile.display_name}><Avatar userId={user.id} name={user.profile.display_name} className="avatar-dark" /></Link> : <Link className="topbar-login" to="/login">{t('signIn')}<ArrowUpRight size={16} /></Link>}
          <Link className="button button-dark topbar-create" to="/rides/new"><Plus size={17} />{t('newRide')}</Link>
        </div>
      </header>
      {!online && <div className="connection-banner" role="status">{t('offline')}</div>}
      {logoutError != null && <ErrorNotice error={logoutError} />}
      {needRefresh && <div className="update-banner" role="status"><span>{t('updateReady')}</span><button className="text-button" onClick={() => void updateServiceWorker(true)}>{t('updateNow')}</button><button className="icon-button" aria-label={t('later')} onClick={() => setNeedRefresh(false)}><X size={17} /></button></div>}
      <main id="main-content" ref={mainRef} tabIndex={-1}>
        <Suspense fallback={<Loading />}><Routes>
          <Route path="/" element={<BrowsePage />} />
          <Route path="/login" element={<LoginPage />} />
          <Route path="/profile" element={<RequireUser allowOnboarding><ProfilePage /></RequireUser>} />
          <Route path="/inbox" element={<RequireUser><InboxPage key={user?.id} /></RequireUser>} />
          <Route path="/admin" element={<RequireUser><AdminPage key={user?.id} /></RequireUser>} />
          <Route path="/settings" element={<RequireUser allowOnboarding><SettingsPage /></RequireUser>} />
          <Route path="/install" element={<InstallPage />} />
          {RideFlowPreview && <Route path="/preview/ride-flow" element={<RideFlowPreview />} />}
          <Route path="/rides/new" element={<RequireUser><RideFormPage /></RequireUser>} />
          <Route path="/rides/:rideId/edit" element={<RequireUser><RideFormPage edit /></RequireUser>} />
          <Route path="/rides/:rideId" element={<RideDetailPage />} />
          <Route path="/garage" element={<RequireUser><GaragePage key={user?.id} /></RequireUser>} />
          <Route path="/garage/new" element={<RequireUser><BikeFormPage /></RequireUser>} />
          <Route path="/garage/:bikeId/edit" element={<RequireUser><BikeFormPage edit /></RequireUser>} />
          <Route path="/garage/:bikeId" element={<RequireUser><BikeOwnerPage /></RequireUser>} />
          <Route path="/bikes/:bikeId" element={<PublicBikePage />} />
          <Route path="/clubs" element={<ClubsPage />} />
          <Route path="/clubs/new" element={<RequireUser><ClubFormPage /></RequireUser>} />
          <Route path="/clubs/:clubId/edit" element={<RequireUser><ClubFormPage edit /></RequireUser>} />
          <Route path="/clubs/:clubId" element={<ClubDetailPage />} />
          <Route path="*" element={<div className="page-content"><h1>{t('notFound')}</h1><Link to="/">{t('back')}</Link></div>} />
        </Routes></Suspense>
      </main>
      <footer className="app-footer"><span>BIKERS.KZ</span><span>{t('sidebarNote')}</span></footer>
    </div>
  </div>
}
