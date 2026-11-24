import { useEffect, useId, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowLeftIcon, ArrowRightIcon, CalendarCheckIcon, CalendarDotsIcon, PlusIcon, XIcon } from '@phosphor-icons/react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { ApiError, mutation, request } from '../api'
import { cityLabel, cityTooLong } from '../cities'
import { CitySelector, useCityOptions } from '../components/CitySelector'
import { ShareMenu } from '../components/RideShare'
import { useI18n } from '../i18n'
import { ridePollPath, ridePollsPath } from '../ride-polls'
import type { RidePoll, RidePollResponse } from '../ride-polls'
import { localRideTime, rideTimeToInstant } from '../ride-time'
import { useSession, useSessionGuard } from '../session'
import { ErrorNotice, Field, Loading, formatRideDateTime, useConfirmation, useOnline } from '../ui'
import './ride-polls.css'

export function NewRidePollPage() {
  const { t } = useI18n()
  const { user } = useSession()
  const captureSession = useSessionGuard()
  const navigate = useNavigate()
  const online = useOnline()
  const proposalHintId = useId()
  const [city, setCity] = useState(user?.profile.city ?? '')
  const [description, setDescription] = useState('')
  const [options, setOptions] = useState(['', ''])
  const timeInputs = useRef<HTMLFieldSetElement>(null)
  const [busy, setBusy] = useState(false)
  const inFlight = useRef(false)
  const [validation, setValidation] = useState('')
  const [error, setError] = useState<unknown>()
  const [openedAt] = useState(Date.now)
  useEffect(() => {
    if (options.length === 3) timeInputs.current?.querySelectorAll('input').item(2)?.focus()
  }, [options.length])

  async function create(event: FormEvent) {
    event.preventDefault()
    if (inFlight.current || !online) return
    setValidation('')
    setError(undefined)
    const proposal = description.trim()
    if (!city.trim() || cityTooLong(city.trim()) || !proposal) {
      setValidation(t('pollInvalidProposal'))
      return
    }
    if (Array.from(proposal).length > 500 || new TextEncoder().encode(proposal).length > 1000) {
      setValidation(t('pollProposalTooLong'))
      return
    }
    const instants = options.map((value) => rideTimeToInstant(value))
    const now = Date.now()
    if (instants.some((instant) => !(instant.getTime() > now) || instant.getUTCFullYear() > 2100) ||
        new Set(instants.map((instant) => instant.getTime())).size !== instants.length) {
      setValidation(t('pollInvalidTimes'))
      return
    }
    const isCurrent = captureSession()
    inFlight.current = true
    setBusy(true)
    try {
      const { poll } = await mutation<RidePollResponse>(ridePollsPath, 'POST', {
        city: city.trim(), description: proposal, options: instants.map((instant) => instant.toISOString()),
      })
      if (isCurrent()) void navigate(ridePollPath(poll.id), { replace: true })
    } catch (failure) {
      if (isCurrent()) setError(failure)
    } finally {
      inFlight.current = false
      if (isCurrent()) setBusy(false)
    }
  }

  return <div className="page-content form-page">
    <Link className="back-link" to="/rides/new"><ArrowLeftIcon size={17} />{t('newRide')}</Link>
    <div className="page-heading"><h1>{t('pollCreateHeading')}</h1><p>{t('pollCreateIntro')}</p></div>
    <form className="surface ride-poll-panel" onSubmit={(event) => void create(event)} noValidate>
      <CitySelector label={t('city')} value={city} onChange={setCity} required disabled={busy} />
      <div className="ride-poll-proposal-field">
        <Field label={t('pollProposal')} required>
          <textarea value={description} onChange={(event) => setDescription(event.target.value)} required disabled={busy} maxLength={1000} rows={4} aria-describedby={proposalHintId} />
        </Field>
        <p className="field-hint" id={proposalHintId}>{t('pollProposalHelp')}</p>
      </div>
      <fieldset ref={timeInputs} className="ride-poll-times" disabled={busy}>
        <legend>{t('pollDepartureOptions')}</legend>
        <p className="field-hint">{t('timezoneHint')}</p>
        {options.map((value, index) => <Field key={index} label={t('pollOptionNumber', { number: index + 1 })} type="datetime-local" value={value}
          min={localRideTime(new Date(openedAt + 60_000).toISOString())} max="2100-12-31T23:59" required
          onChange={(event) => setOptions((current) => current.map((time, position) => position === index ? event.target.value : time))} />)}
      </fieldset>
      {options.length === 2
        ? <button className="text-button" type="button" disabled={busy} onClick={() => setOptions((current) => [...current, ''])}><PlusIcon size={17} />{t('pollAddOption')}</button>
        : <button className="text-button" type="button" disabled={busy} onClick={() => setOptions((current) => current.slice(0, 2))}><XIcon size={17} />{t('pollRemoveOption')}</button>}
      {validation && <ErrorNotice text={validation} />}
      {error != null && <ErrorNotice error={error} />}
      <div className="ride-poll-actions"><button className="button button-primary" type="submit" disabled={busy || !online}>{t(busy ? 'saving' : 'pollCreate')}<ArrowRightIcon size={17} /></button></div>
    </form>
  </div>
}

export function RidePollPage() {
  const { pollId = '' } = useParams()
  const { user, loading, error, refresh } = useSession()
  const { t } = useI18n()
  const client = useQueryClient()
  const path = ridePollPath(pollId)
  const queryKey = ['ride-poll', pollId, user?.id ?? 'guest']
  const query = useQuery({
    queryKey,
    queryFn: ({ signal }) => request<RidePollResponse>(path, { signal }),
    enabled: !!pollId && !loading && !error,
    retry: false, meta: { private: true },
    refetchInterval: (current) => current.state.data?.poll.can_manage && !current.state.data.poll.selected_option_id ? 30_000 : false,
  })
  async function update(response: RidePollResponse) {
    await client.cancelQueries({ queryKey, exact: true })
    client.setQueryData(queryKey, response)
  }
  return <div className="page-content form-page">
    <Link className="back-link" to="/"><ArrowLeftIcon size={17} />{t('back')}</Link>
    {error ? <ErrorNotice text={t('serviceUnavailable')} retry={() => void refresh()} />
      : loading || query.isPending ? <Loading />
      : query.isError ? <ErrorNotice error={query.error} retry={() => void query.refetch()} />
      : <PollOptions key={pollId + ':' + (user?.id ?? 'guest')} poll={query.data.poll} path={path} update={update} refresh={() => void query.refetch()} />}
  </div>
}

function PollOptions({ poll, path, update, refresh }: {
  poll: RidePoll
  path: string
  update: (response: RidePollResponse) => Promise<void>
  refresh: () => void
}) {
  const { t, locale } = useI18n()
  const cities = useCityOptions()
  const online = useOnline()
  const captureSession = useSessionGuard()
  const { confirm, dialog } = useConfirmation()
  const [now, setNow] = useState(Date.now)
  const [changedVote, setChangedVote] = useState<string[] | null>(null)
  const [hostChoice, setHostChoice] = useState('')
  const [busy, setBusy] = useState(false)
  const inFlight = useRef(false)
  const [error, setError] = useState<unknown>()
  const [saved, setSaved] = useState<'pollVoteSaved' | 'pollVoteCleared'>()
  const chosen = poll.options.find((option) => option.id === poll.selected_option_id)
  const future = poll.options.filter((option) => Date.parse(option.starts_at) > now)
  const draft = (changedVote ?? poll.my_option_ids).filter((id) => future.some((option) => option.id === id))
  const dirty = draft.length !== poll.my_option_ids.length || draft.some((id) => !poll.my_option_ids.includes(id))
  const hasHostChoice = future.some((option) => option.id === hostChoice)

  useEffect(() => {
    const clock = window.setInterval(() => setNow(Date.now()), 30_000)
    return () => window.clearInterval(clock)
  }, [])

  async function save(action: 'vote' | 'selection', optionIDs: string[]) {
    if (inFlight.current || !online || chosen) return
    const isCurrent = captureSession()
    inFlight.current = true
    try {
      if (action === 'selection' && !await confirm({ action: t('pollConfirmTime'), message: t('pollConfirmTitle') })) return
      if (!isCurrent()) return
      setBusy(true)
      setError(undefined)
      setSaved(undefined)
      const body = action === 'vote' ? { option_ids: optionIDs } : { option_id: optionIDs[0] }
      const response = await mutation<RidePollResponse>(path + '/' + action, 'PUT', body)
      if (!isCurrent()) return
      await update(response)
      if (isCurrent() && action === 'vote') {
        setChangedVote(null)
        setSaved(optionIDs.length ? 'pollVoteSaved' : 'pollVoteCleared')
      }
    } catch (failure) {
      if (isCurrent()) {
        setError(failure)
        if (failure instanceof ApiError && failure.code === 'poll_closed') refresh()
      }
    } finally {
      inFlight.current = false
      if (isCurrent()) setBusy(false)
    }
  }

  function submit(event: FormEvent) {
    event.preventDefault()
    if (poll.can_manage ? hasHostChoice : dirty && !!draft.length) {
      void save(poll.can_manage ? 'selection' : 'vote', poll.can_manage ? [hostChoice] : draft)
    }
  }

  return <>
    <div className="page-heading"><h1>{t('pollHeading')}</h1><p>{cityLabel(poll.city, locale, cities.data?.cities ?? [])}</p></div>
    <div className="surface ride-poll-panel">
      <p className="ride-poll-proposal">{poll.description}</p>
      <ShareMenu path={path} label={t('pollShare')} />
    </div>
    {chosen && <section className="surface ride-poll-panel ride-poll-selected" aria-labelledby="poll-confirmed-heading">
      <h2 id="poll-confirmed-heading"><CalendarCheckIcon size={24} />{t('pollSelectedHeading')}</h2>
      <time dateTime={chosen.starts_at}>{formatRideDateTime(chosen.starts_at, poll.timezone, locale)}</time>
      <p className="field-hint">{t('timezoneHint')}</p>
      <p>{t('pollSelectedHint')}</p>
      {poll.can_manage && Date.parse(chosen.starts_at) > now && <div className="ride-poll-actions"><Link className="button button-primary" to={'/rides/new?' + new URLSearchParams({ poll: poll.id })}>{t('pollContinue')}<ArrowRightIcon size={17} /></Link></div>}
    </section>}
    <form className="surface ride-poll-panel" onSubmit={submit}>
      <fieldset className="ride-poll-times" disabled={busy || !!chosen}>
        <legend>{t('pollDepartureOptions')}</legend>
        <p className="field-hint">{t('timezoneHint')}</p>
        {!chosen && <p>{t(poll.can_manage ? 'pollHostPrompt' : 'pollVotePrompt')}</p>}
        {poll.options.map((option) => {
          const passed = Date.parse(option.starts_at) <= now
          const checked = poll.can_manage ? hostChoice === option.id : draft.includes(option.id)
          const OptionRow = chosen ? 'div' : 'label'
          return <OptionRow className="ride-poll-option" key={option.id} data-selected={chosen?.id === option.id || undefined}>
            {chosen ? <CalendarDotsIcon size={20} aria-hidden="true" /> : <input type={poll.can_manage ? 'radio' : 'checkbox'} name="ride-poll-option" checked={checked} disabled={passed}
              onChange={(event) => {
                setSaved(undefined)
                if (poll.can_manage) setHostChoice(option.id)
                else setChangedVote(event.target.checked ? [...draft, option.id] : draft.filter((id) => id !== option.id))
              }} />}
            <span><time dateTime={option.starts_at}>{formatRideDateTime(option.starts_at, poll.timezone, locale)}</time>
              <span className="field-hint">{t('pollVoteCount', { count: option.votes })}{passed && ' · ' + t('pollPastOption')}</span></span>
          </OptionRow>
        })}
      </fieldset>
      {!chosen && !future.length && <p className="field-hint">{t('pollNoFutureTimes')}</p>}
      {!chosen && future.length > 0 && <div className="ride-poll-actions">
        {poll.can_manage ? <button className="button button-primary" type="submit" disabled={busy || !online || !hasHostChoice}>{t('pollConfirmTime')}</button>
          : <>
            <button className="button button-primary" type="submit" disabled={busy || !online || !dirty || !draft.length}>{t('pollSaveVote')}</button>
            {!!poll.my_option_ids.length && <button className="text-button" type="button" disabled={busy || !online} onClick={() => void save('vote', [])}>{t('pollClearVote')}</button>}
          </>}
      </div>}
      {saved && <p className="success-message" role="status">{t(saved)}</p>}
      {error != null && <ErrorNotice error={error} />}
    </form>
    {dialog}
  </>
}
