import { useEffect, useId, useRef, useState } from 'react'
import * as DropdownMenu from '@radix-ui/react-dropdown-menu'
import { CheckIcon, UsersThreeIcon, UserMinusIcon, XIcon } from '@phosphor-icons/react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ApiError, getSessionEpoch, request } from '../api'
import { useI18n } from '../i18n'
import { useSession, useSessionGuard } from '../session'
import { useSessionTask } from '../useSessionTask'
import type { SessionTask } from '../useSessionTask'
import { ErrorNotice, Loading, useConfirmation, useOnline } from '../ui'
import type { Participant, Ride } from '../types'
import { Avatar } from './Avatar'
import { CommunityBadge, useReportAction } from './Trust'
import { RiderActionMenu } from './RiderActionMenu'
import type { SelectRiderAction } from './RiderActionMenu'
import { useRiderSafetyActions } from './useRiderSafetyActions'
import './participant-actions.css'

type Decision = 'approve' | 'reject' | 'remove'
interface Props { ride: Ride; onAccessLost: () => void }

export function ParticipantsPanel(props: Props) {
  const { user, loading, error } = useSession()
  if (!user || loading || error) return null
  return <ParticipantRoster key={user.id + ':' + getSessionEpoch() + ':' + props.ride.id} {...props} actorId={user.id} />
}
function ParticipantRoster({ ride, onAccessLost, actorId }: Props & { actorId: string }) {
  const { t } = useI18n()
  const client = useQueryClient()
  const online = useOnline()
  const start = useSessionTask()
  const captureSession = useSessionGuard()
  const { confirm, dialog } = useConfirmation()
  const [working, setWorking] = useState('')
  const [actionError, setActionError] = useState<unknown>()
  const heading = useRef<HTMLHeadingElement>(null)
  const headingId = useId()
  const active = useRef<SessionTask | undefined>(undefined)
  const latestRide = useRef(ride)
  const focusAfterDecision = useRef(false)
  useEffect(() => { latestRide.current = ride }, [ride])
  const key = ['participants', ride.id, actorId]
  const query = useQuery({ queryKey: key, queryFn: ({ signal }) => request<{ participants: Participant[] }>('/rides/' + ride.id + '/participants', { signal }), meta: { private: true } })
  useEffect(() => { if (query.error instanceof ApiError && [401, 403].includes(query.error.status)) onAccessLost() }, [query.error, onAccessLost])
  useEffect(() => {
    if (!focusAfterDecision.current || query.isFetching) return
    focusAfterDecision.current = false
    if (document.activeElement === document.body || !document.activeElement?.isConnected) heading.current?.focus({ preventScroll: true })
  }, [query.data, query.isFetching])
  const participants = query.data?.participants ?? []
  const requested = participants.filter((person) => person.state === 'requested')
  const approved = participants.filter((person) => person.state === 'approved')

  function allowed(task: SessionTask, person: Participant, decision: Decision) {
    if (!task.current() || navigator.onLine === false) return false
    const cached = client.getQueryState<{ ride: Ride }>(['ride', ride.id, actorId])
    const currentRide = cached?.data?.ride
    if (!cached || !currentRide?.can_manage || currentRide.organizer_id !== actorId ||
        !latestRide.current.can_manage || latestRide.current.organizer_id !== actorId) {
      setActionError(new ApiError(403, 'forbidden')); onAccessLost(); return false
    }
    const roster = client.getQueryState<{ participants: Participant[] }>(key)
    const current = roster?.data?.participants.find((p) => p.user_id === person.user_id)
    if (cached.status !== 'success' || cached.isInvalidated || cached.fetchStatus !== 'idle' ||
        roster?.status !== 'success' || roster.isInvalidated || roster.fetchStatus !== 'idle' ||
        !current || current.user_id === actorId || current.user_id === currentRide.organizer_id ||
        current.state !== (decision === 'remove' ? 'approved' : 'requested') ||
        (decision !== 'remove' && (currentRide.status === 'cancelled' || latestRide.current.status === 'cancelled'))) {
      setActionError(new ApiError(409, 'participation_changed'))
      focusAfterDecision.current = true
      void client.invalidateQueries({ queryKey: ['participants', ride.id] })
      void client.invalidateQueries({ queryKey: ['ride', ride.id] })
      return false
    }
    return true
  }
  async function decide(person: Participant, decision: Decision) {
    if (active.current) return
    const currentSession = captureSession()
    const sessionCurrent = () => currentSession() && client.getQueryState(['session'])?.status === 'success'
    const task = start()
    active.current = task
    let requestStarted = false
    let reconciled = false
    if (task.current()) { setWorking(person.user_id); setActionError(undefined) }
    try {
      if (!allowed(task, person, decision)) return
      if (decision === 'remove' && !await confirm({ message: t('removeConfirm'), action: t('remove'), destructive: true })) return
      // Confirmation and menu dismissal are asynchronous: never act on the
      // captured role or row state without checking the current session/cache.
      if (!allowed(task, person, decision)) return
      requestStarted = true
      await request('/rides/' + ride.id + '/participants/' + person.user_id + '/decision', {
        method: 'POST', signal: task.signal, body: JSON.stringify({ decision }),
      })
      if (sessionCurrent()) {
        reconciled = true
        focusAfterDecision.current = true
        await Promise.all([client.invalidateQueries({ queryKey: ['participants', ride.id] }), client.invalidateQueries({ queryKey: ['ride', ride.id] })])
      }
    } catch (error) {
      if (task.current()) {
        setActionError(error)
        if (error instanceof ApiError && error.code === 'participation_changed') { focusAfterDecision.current = true; void client.invalidateQueries({ queryKey: ['participants', ride.id] }) }
        if (error instanceof ApiError && [401, 403].includes(error.status)) onAccessLost()
      }
    } finally {
      // A trust hint can remove this panel after the server commits but before
      // its response arrives. Reconcile only the original authenticated session.
      if (requestStarted && !reconciled && !task.current() && sessionCurrent()) {
        await Promise.all([client.invalidateQueries({ queryKey: ['participants', ride.id] }), client.invalidateQueries({ queryKey: ['ride', ride.id] })])
      }
      if (task.current()) setWorking('')
      if (active.current === task) active.current = undefined
      task.finish()
    }
  }
  const managed = !!ride.can_manage && ride.organizer_id === actorId
  return <section className="surface participants-card" aria-labelledby={headingId}>{dialog}
    <div className="panel-title"><span className="panel-icon"><UsersThreeIcon size={20} /></span><h2 id={headingId} ref={heading} tabIndex={-1}>{t('participants')}</h2>{query.data && <span className="count-pill">{approved.length}</span>}</div>
    {query.isPending ? <Loading /> : query.isError ? <ErrorNotice error={query.error} retry={() => void query.refetch()} /> : <>
      {managed && <div className="request-list"><h3>{t('joinRequests')}<span>{requested.length}</span></h3>{requested.length ? requested.map((person) => <div className="participant request-participant participant-with-actions" key={person.user_id}>
        <span className="avatar">{person.display_name.trim().slice(0, 1)}</span><div className="participant-name"><strong>{person.display_name}</strong><span>{t('requested')}</span></div>
        {person.user_id !== actorId && person.user_id !== ride.organizer_id && <PendingActions person={person} disabled={!!working || !online || query.isFetching || ride.status === 'cancelled'} decide={decide} />}
      </div>) : <p className="small-empty">{t('noRequests')}</p>}</div>}
      <div className="approved-list">{approved.length ? approved.map((person) => <div className="participant participant-with-actions" key={person.user_id}>
        <Avatar userId={person.user_id} name={person.display_name} scope={{ kind: 'ride', id: ride.id }} className={person.user_id === ride.organizer_id ? 'avatar-orange' : ''} /><div className="participant-name"><strong>{person.display_name}{person.user_id === actorId && <span className="self-label"> · {t('you')}</span>}</strong><CommunityBadge userId={person.user_id} /><span>{person.user_id === ride.organizer_id ? t('organizer') : t('approved')}</span></div>
        {person.user_id !== actorId && <ApprovedActions person={person} rideId={ride.id} actorId={actorId} canRemove={managed && person.user_id !== ride.organizer_id} disabled={!!working || !online || query.isFetching} decide={decide} />}
      </div>) : <p className="small-empty">{t('noParticipants')}</p>}</div>
    </>}
    {actionError != null && <ErrorNotice error={actionError} />}
  </section>
}
interface ActionsProps { person: Participant; disabled: boolean; decide: (person: Participant, decision: Decision) => Promise<void> }
function DecisionItems({ person, disabled, decide, select, pending }: ActionsProps & { select: SelectRiderAction; pending: boolean }) {
  const { t } = useI18n()
  return <DropdownMenu.Group aria-label={t('participantDecisionGroup')}><DropdownMenu.Label className="message-menu-label">{t('participantDecisionGroup')}</DropdownMenu.Label>
    {pending ? <><DropdownMenu.Item className="message-menu-item" disabled={disabled} onSelect={() => select(() => { void decide(person, 'approve') })}><CheckIcon size={18} />{t('approve')}</DropdownMenu.Item><DropdownMenu.Item className="message-menu-item" disabled={disabled} onSelect={() => select(() => { void decide(person, 'reject') })}><XIcon size={18} />{t('reject')}</DropdownMenu.Item></> : <DropdownMenu.Item className="message-menu-item" disabled={disabled} onSelect={() => select(() => { void decide(person, 'remove') })}><UserMinusIcon size={18} />{t('remove')}</DropdownMenu.Item>}
  </DropdownMenu.Group>
}
function PendingActions(props: ActionsProps) {
  const { t } = useI18n()
  const trigger = useRef<HTMLButtonElement>(null)
  return <div className="participant-action-owner"><RiderActionMenu trigger={trigger} label={t('participantActions', { name: props.person.display_name })} title={t('participantActionsMenu')}>{(select) => <DecisionItems {...props} select={select} pending />}</RiderActionMenu></div>
}
function ApprovedActions({ person, rideId, actorId, canRemove, disabled, decide }: ActionsProps & { rideId: string; actorId: string; canRemove: boolean }) {
  const { t } = useI18n()
  const trigger = useRef<HTMLButtonElement>(null)
  const report = useReportAction({ focusOnOpen: true, onClose: () => trigger.current?.focus({ preventScroll: true }) })
  const safety = useRiderSafetyActions({ userId: person.user_id, name: person.display_name, actorId, scope: { kind: 'ride', id: rideId }, report })
  return <><div className="participant-action-owner"><RiderActionMenu trigger={trigger} label={t('participantActions', { name: person.display_name })} title={t('participantActionsMenu')}>{(select) => <>
    {canRemove && <><DecisionItems person={person} disabled={disabled || safety.busy} decide={decide} select={select} pending={false} /><DropdownMenu.Separator className="message-menu-separator" /></>}
    {safety.items(select)}
  </>}</RiderActionMenu></div><div className="participant-action-feedback">{report.content}{safety.feedback}</div></>
}
