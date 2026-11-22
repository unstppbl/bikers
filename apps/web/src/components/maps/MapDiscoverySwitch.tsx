import { Link, useLocation } from 'react-router-dom'
import { MapPinIcon, PathIcon } from '@phosphor-icons/react'
import { useI18n } from '../../i18n'
import './map-discovery-switch.css'

export function MapDiscoverySwitch({ current, city }: { current: 'places' | 'routes'; city: string }) {
  const { t } = useI18n()
  const location = useLocation()
  const search = city.trim() ? '?' + new URLSearchParams({ city: city.trim() }) : ''
  return <nav className="map-discovery-switch" aria-label={t('routesMapDiscovery')}>
    <Link to={'/places' + (current === 'places' ? location.search : search)} aria-current={current === 'places' ? 'page' : undefined}><MapPinIcon size={18} aria-hidden="true" />{t('routesMapPlaces')}</Link>
    <Link to={'/routes' + (current === 'routes' ? location.search : search)} aria-current={current === 'routes' ? 'page' : undefined}><PathIcon size={18} aria-hidden="true" />{t('routesTitle')}</Link>
  </nav>
}
