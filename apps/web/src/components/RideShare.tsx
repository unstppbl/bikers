import { useState } from 'react'
import { PaperPlaneTiltIcon, ShareNetworkIcon } from '@phosphor-icons/react'
import { useI18n } from '../i18n'
import { ShareLink } from '../ui'

export function RideShare({ rideId }: { rideId: string }) {
  const { t } = useI18n()
  return <ShareMenu path={`/rides/${rideId}`} label={t('rideShare')} />
}

export function ShareMenu({ path, label }: { path: string; label: string }) {
  const { t } = useI18n()
  const [failed, setFailed] = useState(false)
  const url = new URL(path, window.location.origin).href
  const telegram = new URL('https://t.me/share/url')
  telegram.searchParams.set('url', url)
  const canShare = typeof navigator.share === 'function' && (!navigator.canShare || navigator.canShare({ url }))

  async function share() {
    setFailed(false)
    try {
      await navigator.share({ url })
    } catch (error) {
      if (!(error instanceof DOMException && error.name === 'AbortError')) setFailed(true)
    }
  }

  return <details className="ride-share">
    <summary className="button button-outline"><ShareNetworkIcon size={17} />{label}</summary>
    <div className="ride-share-options">
      {canShare && <button type="button" className="button button-outline" onClick={() => void share()}>{t('rideShareApps')}</button>}
      <a className="button button-outline" href={telegram.href} target="_blank" rel="noopener noreferrer"><PaperPlaneTiltIcon size={17} />{t('rideShareTelegram')}</a>
      <ShareLink path={path} />
      {failed && <p className="field-hint" role="status">{t('rideShareFailed')}</p>}
    </div>
  </details>
}
