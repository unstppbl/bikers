import { useRef } from 'react'
import * as DropdownMenu from '@radix-ui/react-dropdown-menu'
import { FlagIcon } from '@phosphor-icons/react'
import { getSessionEpoch } from '../api'
import { useI18n } from '../i18n'
import { useSession } from '../session'
import type { Message } from '../types'
import { useReportAction } from './Trust'
import { RiderActionMenu } from './RiderActionMenu'
import { useRiderSafetyActions } from './useRiderSafetyActions'
import './message-actions.css'

export function MessageActions({ message, rideId }: { message: Message; rideId: string }) {
  const { user, loading, error } = useSession()
  if (!user || loading || error) return null
  return <MessageActionControls key={user.id + ':' + getSessionEpoch() + ':' + message.id} message={message} rideId={rideId} actorId={user.id} />
}
function MessageActionControls({ message, rideId, actorId }: { message: Message; rideId: string; actorId: string }) {
  const { t } = useI18n()
  const trigger = useRef<HTMLButtonElement>(null)
  const report = useReportAction({ focusOnOpen: true, onClose: () => trigger.current?.focus({ preventScroll: true }) })
  const safety = useRiderSafetyActions({ userId: message.sender_id, name: message.sender_name, actorId, scope: { kind: 'ride', id: rideId }, report })
  return <>
    <div className="message-action-owner"><RiderActionMenu trigger={trigger} label={t('messageActions', { name: message.sender_name })} title={t('messageActionsMenu')}>{(select) => <>
      <DropdownMenu.Group aria-label={t('messageActionMessageGroup')}>
        <DropdownMenu.Label className="message-menu-label">{t('messageActionMessageGroup')}</DropdownMenu.Label>
        <DropdownMenu.Item className="message-menu-item" disabled={safety.busy} onSelect={() => select(() => report.show({ targetType: 'message', targetId: message.id, label: 'reportMessage' }))}><FlagIcon size={18} />{t('reportMessage')}</DropdownMenu.Item>
      </DropdownMenu.Group>
      {safety.otherRider && <><DropdownMenu.Separator className="message-menu-separator" />{safety.items(select)}</>}
    </>}</RiderActionMenu></div>
    <div className="message-action-feedback">{report.content}{safety.feedback}</div>
  </>
}
