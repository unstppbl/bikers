import { GasPumpIcon, WrenchIcon, TireIcon, MotorcycleIcon, CarSimpleIcon, UsersThreeIcon, ArrowSquareOutIcon } from '@phosphor-icons/react'
import { Link } from 'react-router-dom'
import { safeExternalUrl } from '../../api'
import { intlLocale, useI18n } from '../../i18n'
import { categoryLabels } from '../../places'
import { useSession } from '../../session'
import type { Place, PlaceCategory, PlaceInput } from '../../types'
const icons = { fuel: GasPumpIcon, workshop: WrenchIcon, tires: TireIcon, motorcycle_shop: MotorcycleIcon, parking: CarSimpleIcon, meeting_spot: UsersThreeIcon }
export function PlaceCategoryIcon({ category, size = 20 }: { category: PlaceCategory; size?: number }) { const Icon = icons[category]; return <Icon size={size} aria-hidden="true" /> }
export function PlaceInformation({ place }: { place: PlaceInput }) {
  const { t } = useI18n(), website = safeExternalUrl(place.website)
  return <><p className="place-category"><PlaceCategoryIcon category={place.category} />{t(categoryLabels[place.category])}</p><p>{[place.city, place.address].filter(Boolean).join(' · ')}</p>{place.description && <p className="preserve-lines">{place.description}</p>}<dl className="place-facts"><div><dt>{t('placesLatitude')}</dt><dd>{place.lat.toFixed(5)}</dd></div><div><dt>{t('placesLongitude')}</dt><dd>{place.lon.toFixed(5)}</dd></div>{place.opening_hours && <div><dt>{t('placesHours')}</dt><dd>{place.opening_hours}</dd></div>}{place.phone && <div><dt>{t('placesPhone')}</dt><dd>{place.phone}</dd></div>}{website && <div><dt>{t('placesWebsite')}</dt><dd><a href={website} target="_blank" rel="noopener noreferrer">{t('placesWebsite')}<ArrowSquareOutIcon size={16} /></a></dd></div>}</dl></>
}
export function PlaceDetails({ place }: { place: Place }) {
  const { t, locale } = useI18n(), { user, error } = useSession(), source = safeExternalUrl(place.source_url)
  return <article className="surface place-details" aria-label={t('placesDetails')}><h2>{place.name}</h2><PlaceInformation place={place} /><a className="button button-outline" href={'https://www.google.com/maps/dir/?api=1&destination=' + place.lat + ',' + place.lon + '&travelmode=driving'} target="_blank" rel="noopener noreferrer">{t('placesNavigate')}<ArrowSquareOutIcon size={16} /></a><p className="field-hint">{t('placesUnverified')}</p><p className="place-source">{t('placesSource')}: {place.source === 'community' && <>{t('placesCommunity')}{place.source_ref && ' · '}</>}{(place.source === 'osm' || place.source_ref) && <a href={source || 'https://www.openstreetmap.org/copyright'} target="_blank" rel="noopener noreferrer">OpenStreetMap</a>}{place.source_date && <> · {t('placesSourceDate')}: <time dateTime={place.source_date}>{place.source_date}</time></>}<br />{t('placesUpdated')}: <time dateTime={place.updated_at}>{new Date(place.updated_at).toLocaleDateString(intlLocale(locale))}</time></p><div className="place-actions">{user && !error ? <><Link className="text-button" to={'/places/' + place.id + '/suggest?kind=correction'}>{t('placesCorrection')}</Link><Link className="text-button" to={'/places/' + place.id + '/suggest?kind=closed'}>{t('placesClosed')}</Link></> : <Link to={'/login?next=' + encodeURIComponent('/places/' + place.id + '/suggest?kind=correction')}>{t('placesLogin')}</Link>}</div></article>
}
