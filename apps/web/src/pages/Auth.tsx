import { CitySelector } from '../components/CitySelector'
import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { ArrowLeft, ArrowRight, Check, LockKeyhole, Send, ShieldCheck } from 'lucide-react'
import { Link, Navigate, useNavigate, useSearchParams } from 'react-router-dom'
import { mutation } from '../api'
import { AvatarEditor } from '../components/Avatar'
import { CommunityBadge, ManageUserLink } from '../components/Trust'
import { useI18n } from '../i18n'
import { useSession, useSessionGuard } from '../session'
import { ErrorNotice, Field, Loading, TextArea, useOnline, validNext } from '../ui'
import type { Challenge, Session, User } from '../types'

export function LoginPage() {
  const { t } = useI18n()
  const { user, loading, accept, error: sessionError, refresh } = useSession()
  const captureSession = useSessionGuard()
  const [searchParams] = useSearchParams()
  const next = validNext(searchParams.get('next'))
  const navigate = useNavigate()
  const online = useOnline()
  const [phone, setPhone] = useState('')
  const [code, setCode] = useState('')
  const [challenge, setChallenge] = useState<Challenge>()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<unknown>()
  const [validation, setValidation] = useState('')
  const [now, setNow] = useState(Date.now)
  useEffect(() => {
    if (!challenge) return
    const timer = window.setInterval(() => setNow(Date.now()), 1000)
    return () => window.clearInterval(timer)
  }, [challenge])
  const resendSeconds = challenge ? Math.max(0, Math.ceil((Date.parse(challenge.resend_after) - now) / 1000)) : 0
  const expired = challenge && Date.parse(challenge.expires_at) <= now
  const normalizedPhone = phone.replace(/[\s()-]/g, '')

  async function sendCode() {
    const currentSession = captureSession()
    setValidation(''); setError(undefined)
    if (!/^\+[1-9]\d{7,14}$/.test(normalizedPhone)) { setValidation(t('invalidPhone')); return }
    setBusy(true)
    try {
      const value = await mutation<Challenge>('/auth/challenges', 'POST', { phone: normalizedPhone })
      if (!currentSession()) return
      setCode(''); setChallenge(value)
    } catch (value) { setError(value) }
    finally { setBusy(false) }
  }

  async function submit(event: FormEvent) {
    const currentSession = captureSession()
    event.preventDefault()
    if (!challenge) { await sendCode(); return }
    setValidation(''); setError(undefined)
    if (!/^\d{6}$/.test(code)) { setValidation(t('invalidCode')); return }
    setBusy(true)
    try {
      const value = await mutation<Session>('/auth/challenges/verify', 'POST', { challenge_id: challenge.challenge_id, code })
      if (!currentSession()) { await refresh(); return }
      accept(value)
      void navigate(value.user?.onboarding_complete ? next : `/profile?next=${encodeURIComponent(next)}`, { replace: true })
    } catch (value) { setError(value) }
    finally { setBusy(false) }
  }

  if (loading) return <Loading />
  if (sessionError) return <div className="page-content"><ErrorNotice text={t('serviceUnavailable')} retry={() => void refresh()} /></div>
  if (user) return <Navigate to={user.onboarding_complete ? next : `/profile?next=${encodeURIComponent(next)}`} replace />
  return <div className="auth-page page-content">
    <Link className="back-link" to="/"><ArrowLeft size={17} />{t('back')}</Link>
    <div className="auth-grid">
      <div className="auth-intro"><span className="eyebrow">{t('loginEyebrow')}</span><h1>{t('loginTitle')}</h1><p>{t('loginText')}</p><div className="auth-art" aria-hidden="true"><div className="road-ring ring-one" /><div className="road-ring ring-two" /><div className="road-ring ring-three" /><span>↗</span></div><div className="auth-intro-foot"><span className="small-marker" />{t('country')}<span>BIKERS.KZ</span></div></div>
      <section className="auth-card" aria-labelledby="auth-heading">
        <div className="form-icon"><Send size={26} strokeWidth={1.6} /></div><h2 id="auth-heading">{challenge ? t('codeTitle') : t('signIn')}</h2><p className="muted">{challenge ? t('codeText', { phone: normalizedPhone }) : t('phoneHint')}</p>
        <form onSubmit={(event) => void submit(event)} noValidate>
          {!challenge ? <Field label={t('phone')} type="tel" autoComplete="tel" inputMode="tel" value={phone} onChange={(event) => setPhone(event.target.value)} placeholder={t('phonePlaceholder')} required autoFocus disabled={busy} /> : <Field label={t('code')} type="text" inputMode="numeric" autoComplete="one-time-code" value={code} onChange={(event) => setCode(event.target.value.replace(/\D/g, '').slice(0, 6))} placeholder="000000" className="otp-input" maxLength={6} required autoFocus disabled={busy || !!expired} />}
          {validation && <ErrorNotice text={validation} />}{error != null && <ErrorNotice error={error} />}{expired && <ErrorNotice text={t('expiredCode')} />}
          <button className="button button-primary button-wide" disabled={busy || !online || !!expired} type="submit">{busy ? (challenge ? t('verifying') : t('sending')) : (challenge ? t('verify') : t('sendCode'))}<ArrowRight size={18} /></button>
          {challenge && <div className="auth-secondary"><button type="button" className="text-button" disabled={busy || resendSeconds > 0 || !online} onClick={() => void sendCode()}>{resendSeconds > 0 ? t('resendIn', { seconds: resendSeconds }) : t('resend')}</button><button type="button" className="text-button muted" disabled={busy} onClick={() => { setChallenge(undefined); setCode(''); setError(undefined); setValidation('') }}>{t('changeNumber')}</button></div>}
        </form>
        <div className="form-footnote"><LockKeyhole size={16} /><p>{t('telegramOnly')}</p></div>
      </section>
    </div>
  </div>
}

export function ProfilePage() {
  const { user, updateUser } = useSession()
  if (!user) return null
  return <ProfileForm key={user.id} user={user} updateUser={updateUser} />
}

function ProfileForm({ user, updateUser }: { user: User; updateUser: (user: User) => void }) {
  const { t, locale } = useI18n()
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const online = useOnline()
  const captureSession = useSessionGuard()
  const [form, setForm] = useState({ display_name: user.profile.display_name ?? '', city: user.profile.city ?? '', languages: user.profile.languages?.length ? user.profile.languages : [locale], interests: user.profile.interests ?? '' })
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<unknown>()
  const [validation, setValidation] = useState('')
  const [saved, setSaved] = useState(false)
  async function submit(event: FormEvent) {
    const currentSession = captureSession()
    event.preventDefault(); setError(undefined); setValidation(''); setSaved(false)
    if (!form.display_name.trim() || !form.city.trim()) { setValidation(t('requiredFields')); return }
    if (!form.languages.length) { setValidation(t('requiredLanguage')); return }
    setBusy(true)
    try {
      const result = await mutation<{ user: User }>('/me/profile', 'PATCH', { ...form, display_name: form.display_name.trim(), city: form.city.trim(), interests: form.interests.trim(), locale })
      if (!currentSession()) return
      updateUser(result.user)
      if (!user.onboarding_complete) void navigate(validNext(searchParams.get('next')), { replace: true })
      else setSaved(true)
    } catch (value) { setError(value) }
    finally { setBusy(false) }
  }
  return <div className="page-content narrow-page"><Link className="back-link" to="/"><ArrowLeft size={17} />{t('back')}</Link><div className="page-heading"><span className="eyebrow">{t('profileEyebrow')}</span><h1>{t(user.onboarding_complete ? 'profileEditTitle' : 'profileTitle')}</h1><p>{t('profileText')}</p></div>
    <CommunityBadge userId={user.id} /><ManageUserLink userId={user.id} />{user.onboarding_complete && <AvatarEditor />}<form className="surface profile-form" onSubmit={(event) => void submit(event)} noValidate><div className="form-grid"><Field label={t('displayName')} value={form.display_name} onChange={(event) => setForm({ ...form, display_name: event.target.value })} autoComplete="nickname" placeholder={t('namePlaceholder')} maxLength={80} required disabled={busy} /><CitySelector label={t('city')} value={form.city} onChange={(city) => setForm({ ...form, city })} placeholder={t('cityPlaceholder')} maxLength={100} required disabled={busy} /></div>
      <fieldset className="language-fieldset"><legend>{t('languages')}</legend><div className="language-options">{([['en', 'English'], ['ru', 'Русский'], ['kk', 'Қазақша']] as const).map(([value, label]) => <label key={value}><input type="checkbox" checked={form.languages.includes(value)} disabled={busy} onChange={(event) => setForm({ ...form, languages: event.target.checked ? [...form.languages, value] : form.languages.filter((item) => item !== value) })} /><span>{label}</span></label>)}</div></fieldset>
      <TextArea label={`${t('interests')} · ${t('optional')}`} value={form.interests} onChange={(interests) => setForm({ ...form, interests })} placeholder={t('interestsPlaceholder')} maxLength={500} rows={3} />
      <div className="inline-note"><ShieldCheck size={20} /><p>{t('publicProfileNote')}</p></div>
      {validation && <ErrorNotice text={validation} />}{error != null && <ErrorNotice error={error} />}{saved && <p className="success-message" role="status"><Check size={18} />{t('saved')}</p>}
      <div className="form-actions"><button className="button button-primary" type="submit" disabled={busy || !online}>{busy ? t('saving') : t(user.onboarding_complete ? 'saveProfile' : 'continueRides')}<ArrowRight size={18} /></button></div>
    </form>
  </div>
}
