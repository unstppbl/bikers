import { useState } from 'react'
import type { ReactNode } from 'react'
import { BellIcon as Bell, CalendarDotsIcon as CalendarDays, CaretRightIcon as ChevronRight, DownloadSimpleIcon as Download, GearSixIcon as Settings, MapTrifoldIcon as MapIcon, MotorcycleIcon as Motorbike, PathIcon, PencilSimpleIcon as Pencil, ShieldCheckIcon as ShieldCheck, SignOutIcon as LogOut, UsersThreeIcon as Users, WrenchIcon as Wrench } from '@phosphor-icons/react'
import { Link, useNavigate } from 'react-router-dom'
import { mutation } from '../api'
import { cityLabel } from '../cities'
import { Avatar } from '../components/Avatar'
import { useCityOptions } from '../components/CitySelector'
import { NotificationBadge, useUnreadNotifications } from '../components/NotificationBadge'
import { CommunityBadge, useTrust } from '../components/Trust'
import { useI18n } from '../i18n'
import { useSession, useSessionGuard } from '../session'
import { ErrorNotice, Loading, useOnline } from '../ui'

function HubLink({ to, icon, label, extra }: { to: string; icon: ReactNode; label: string; extra?: ReactNode }) {
  return <li><Link className="hub-link" to={to}><span className="hub-link-icon">{icon}</span><span className="hub-link-label">{label}</span>{extra}<ChevronRight className="hub-link-arrow" size={18} /></Link></li>
}

// The phone "Me" tab gathers personal pages, the catalogue and account actions in one place.
export function MePage() {
  const { t, locale } = useI18n()
  const { user, loading, error, clear, refresh } = useSession()
  const trust = useTrust()
  const unread = useUnreadNotifications()
  const unreadCount = user && !unread.isError ? unread.data ?? 0 : 0
  const cities = useCityOptions()
  const captureSession = useSessionGuard()
  const online = useOnline()
  const navigate = useNavigate()
  const [signingOut, setSigningOut] = useState(false)
  const [logoutError, setLogoutError] = useState<unknown>()
  async function signOut() {
    const currentSession = captureSession()
    setSigningOut(true)
    setLogoutError(undefined)
    try { await mutation('/session', 'DELETE'); if (!currentSession()) return; clear(); void navigate('/') }
    catch (value) { setLogoutError(value) }
    finally { setSigningOut(false) }
  }
  if (loading) return <Loading />
  if (error) return <div className="page-content"><ErrorNotice text={t('serviceUnavailable')} retry={() => void refresh()} /></div>
  const explore = <section className="hub-section" aria-labelledby="hub-explore"><h2 id="hub-explore">{t('navExplore')}</h2><ul className="hub-list">
    <HubLink to="/routes" icon={<PathIcon size={22} />} label={t('routesTitle')} />
    <HubLink to="/bikes" icon={<Motorbike size={22} />} label={t('bikeCatalogue')} />
    <HubLink to="/places" icon={<MapIcon size={22} />} label={t('placesTitle')} />
    <HubLink to="/install" icon={<Download size={22} />} label={t('installApp')} />
  </ul></section>
  if (!user) return <div className="page-content me-page">
    <div className="page-heading"><h1>{t('meGuestTitle')}</h1><p>{t('meGuestText')}</p></div>
    <Link className="button button-primary me-sign-in" to="/login?next=%2Fme">{t('signIn')}</Link>
    {explore}
  </div>
  return <div className="page-content me-page">
    <header className="me-profile">
      <Avatar userId={user.id} name={user.profile.display_name} className="avatar-large" />
      <div className="me-profile-copy"><h1>{user.profile.display_name}</h1>{user.profile.city && <span className="me-profile-city">{cityLabel(user.profile.city, locale, cities.data?.cities ?? [])}</span>}<CommunityBadge userId={user.id} /></div>
      <Link className="button button-outline me-edit" to="/profile"><Pencil size={17} />{t('meEditProfile')}</Link>
    </header>
    <div className="hub-columns">
      <section className="hub-section" aria-labelledby="hub-yours"><h2 id="hub-yours">{t('navYourSpace')}</h2><ul className="hub-list">
        <HubLink to="/my-rides" icon={<CalendarDays size={22} />} label={t('myRides')} />
        <HubLink to="/garage" icon={<Wrench size={22} />} label={t('garage')} />
        <HubLink to="/my-clubs" icon={<Users size={22} />} label={t('myClubs')} />
        <HubLink to="/inbox" icon={<Bell size={22} />} label={t('inbox')} extra={<NotificationBadge count={unreadCount} />} />
      </ul></section>
      {explore}
      <section className="hub-section" aria-labelledby="hub-account"><h2 id="hub-account">{t('navAccount')}</h2><ul className="hub-list">
        <HubLink to="/settings" icon={<Settings size={22} />} label={t('settings')} />
        {trust.data?.is_admin && <HubLink to="/admin" icon={<ShieldCheck size={22} />} label={t('admin')} />}
        <li><button className="hub-link hub-sign-out" onClick={() => void signOut()} disabled={signingOut || !online}><span className="hub-link-icon"><LogOut size={22} /></span><span className="hub-link-label">{t('signOut')}</span></button></li>
      </ul>{logoutError != null && <ErrorNotice error={logoutError} />}</section>
    </div>
  </div>
}
