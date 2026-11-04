import { useEffect, useState } from 'react'
import { ArrowLeftIcon as ArrowLeft, ArrowUpRightIcon as ArrowUpRight, CheckIcon as Check, DownloadSimpleIcon as Download, MonitorIcon as Monitor, DeviceMobileIcon as Smartphone } from '@phosphor-icons/react'
import { Link } from 'react-router-dom'
import { useI18n } from '../i18n'
import { ErrorNotice } from '../ui'

interface InstallPrompt extends Event {
  prompt(): Promise<void>
  userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>
}

export function isInstalled() {
  return window.matchMedia?.('(display-mode: standalone)').matches === true || (navigator as Navigator & { standalone?: boolean }).standalone === true
}

export function InstallPage() {
  const { t } = useI18n()
  const [installed, setInstalled] = useState(isInstalled)
  const [prompt, setPrompt] = useState<InstallPrompt | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<unknown>()
  useEffect(() => {
    const beforeInstall = (event: Event) => { event.preventDefault(); setPrompt(event as InstallPrompt) }
    const completed = () => { setInstalled(true); setPrompt(null) }
    window.addEventListener('beforeinstallprompt', beforeInstall)
    window.addEventListener('appinstalled', completed)
    return () => { window.removeEventListener('beforeinstallprompt', beforeInstall); window.removeEventListener('appinstalled', completed) }
  }, [])
  async function install() {
    if (!prompt || busy) return
    setBusy(true); setError(undefined)
    try { await prompt.prompt(); const choice = await prompt.userChoice; if (choice.outcome === 'accepted') setInstalled(true); setPrompt(null) }
    catch (value) { setError(value) }
    finally { setBusy(false) }
  }
  const guides = [
    { title: 'installIOS', steps: ['installIOSOne', 'installIOSTwo', 'installIOSThree'], url: 'https://support.apple.com/guide/iphone/open-as-web-app-iphea86e5236/ios', icon: Smartphone },
    { title: 'installAndroid', steps: ['installAndroidOne', 'installAndroidTwo', 'installAndroidThree'], url: 'https://support.google.com/chrome/answer/9658361?co=GENIE.Platform%3DAndroid', icon: Smartphone },
    { title: 'installDesktop', steps: ['installDesktopOne', 'installDesktopTwo', 'installDesktopThree'], url: 'https://support.google.com/chrome/answer/9658361?co=GENIE.Platform%3DDesktop', icon: Monitor },
  ] as const
  return <div className="page-content install-page"><Link className="back-link" to="/"><ArrowLeft size={17} />{t('back')}</Link><div className="page-heading"><span className="eyebrow">{t('installEyebrow')}</span><h1>{t('installTitle')}</h1><p>{t('installText')}</p></div><section className="surface install-status"><span className="panel-icon">{installed ? <Check size={24} /> : <Download size={24} />}</span><div><h2>{t(installed ? 'installReady' : 'installApp')}</h2><p>{t('installOnline')}</p>{!installed && !prompt && <p>{t('installNoPrompt')}</p>}</div>{!installed && prompt && <button className="button button-primary" disabled={busy} onClick={() => void install()}><Download size={16} />{t(busy ? 'installBusy' : 'installNow')}</button>}</section>{error != null && <ErrorNotice error={error} />}<div className="install-guides">{guides.map((guide) => <section key={guide.title} className="surface install-guide"><span className="panel-icon"><guide.icon size={23} /></span><h2>{t(guide.title)}</h2><ol>{guide.steps.map((step) => <li key={step}>{t(step)}</li>)}</ol><a className="inline-link" href={guide.url} target="_blank" rel="noopener noreferrer">{t('installBrowserHelp')}<ArrowUpRight size={14} /></a></section>)}</div></div>
}
