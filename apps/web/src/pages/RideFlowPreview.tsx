import { useState } from 'react'
import { ArrowLeft } from 'lucide-react'
import { Link } from 'react-router-dom'
import { BikersRideDemo } from '../components/ride-demo/BikersRideDemo'
import { rideDemoCopy } from '../components/ride-demo/copy'
import { StillContext } from '../components/ride-demo/kit/reduced-motion'
import { useI18n } from '../i18n'
import './ride-flow-preview.css'

// Authored product illustration. This route is included in development only.
export function RideFlowPreview() {
  const { locale, t } = useI18n()
  const [staticPreview, setStaticPreview] = useState(false)
  const copy = rideDemoCopy[locale]
  const motionLabel = { en: 'Reduced motion preview', ru: 'Просмотр без анимации', kk: 'Қозғалыссыз алдын ала қарау' }[locale]
  return <div className="page-content ride-flow-preview">
    <div className="ride-flow-preview-layout">
      <div className="ride-flow-preview-copy">
        <Link className="back-link" to="/"><ArrowLeft size={18} />{t('back')}</Link>
        <span className="eyebrow">{t('community')}</span>
        <h1>{t('howItWorks')}</h1>
        <p>{copy.summary}</p>
        <label className="ride-flow-motion-option"><input type="checkbox" checked={staticPreview} onChange={(event) => setStaticPreview(event.target.checked)} />{motionLabel}</label>
        <ol>{(['One', 'Two', 'Three'] as const).map((step, index) => <li key={step}><span>{index + 1}</span><div><strong>{t(`step${step}`)}</strong><p>{t(`step${step}Text`)}</p></div></li>)}</ol>
      </div>
      <div className="ride-flow-preview-stage"><StillContext.Provider value={staticPreview}><BikersRideDemo locale={locale} /></StillContext.Provider></div>
    </div>
  </div>
}
