import { useRef } from 'react'
import * as DropdownMenu from '@radix-ui/react-dropdown-menu'
import { DotsThreeIcon, FlagIcon, ProhibitIcon, ShieldCheckIcon } from '@phosphor-icons/react'
import { useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { getSessionEpoch } from '../api'
import { useI18n } from '../i18n'
import { useSession } from '../session'
import { useSessionTask } from '../useSessionTask'
import type { SessionTask } from '../useSessionTask'
import type { Message } from '../types'
import { useAvatarMetadata } from './Avatar'
import { useBlockAction, useReportAction, useTrust } from './Trust'
import './message-actions.css'

export function MessageActions({ message, rideId }: { message: Message; rideId: string }) {
  const { user, loading, error } = useSession()
  if (!user || loading || error) return null
  return <MessageActionControls key={user.id + ':' + getSessionEpoch() + ':' + message.id} message={message} rideId={rideId} actorId={user.id} />
}
function MessageActionControls({ message, rideId, actorId }: { message: Message; rideId: string; actorId: string }) {
  const { t } = useI18n()
  const trigger = useRef<HTMLButtonElement>(null)
  const pending = useRef<{ task: SessionTask; run: () => void } | undefined>(undefined)
  const start = useSessionTask()
  const client = useQueryClient()
  const navigate = useNavigate()
  const report = useReportAction({ focusOnOpen: true, onClose: () => trigger.current?.focus({ preventScroll: true }) })
  const block = useBlockAction({ userId: message.sender_id, name: message.sender_name })
  const avatar = useAvatarMetadata(message.sender_id, { kind: 'ride', id: rideId })
  const trust = useTrust()
  const otherRider = actorId !== message.sender_id
  const image = avatar.ready && !avatar.query.isError ? avatar.query.data?.image : undefined
  const admin = otherRider && trust.isSuccess && trust.data.is_admin
  const busy = report.busy || block.busy
  function queue(run: () => void) {
    pending.current?.task.abort(); pending.current?.task.finish()
    pending.current = { task: start(), run }
  }
  function afterClose(event: Event) {
    const next = pending.current
    pending.current = undefined
    if (!next) return
    event.preventDefault()
    // Dialogs capture the stable ellipsis as their focus return target. The
    // session task also prevents an old menu selection using a new account.
    if (next.task.current()) { trigger.current?.focus({ preventScroll: true }); next.run() }
    next.task.finish()
  }
  function manage() {
    const key = ['trust', actorId]
    if (client.getQueryState(key)?.status === 'success' && client.getQueryData<{ is_admin: boolean }>(key)?.is_admin) {
      void navigate('/admin?user=' + encodeURIComponent(message.sender_id))
    }
  }
  return <>
    <div className="message-action-owner">
      <DropdownMenu.Root>
        <DropdownMenu.Trigger asChild><button ref={trigger} type="button" className="message-action-trigger" aria-label={t('messageActions', { name: message.sender_name })} title={t('messageActionsMenu')}><DotsThreeIcon size={24} weight="bold" /></button></DropdownMenu.Trigger>
        <DropdownMenu.Portal>
          <DropdownMenu.Content className="message-menu-content" aria-label={t('messageActionsMenu')} align="end" sideOffset={8} collisionPadding={16} onCloseAutoFocus={afterClose}>
            <DropdownMenu.Group aria-label={t('messageActionMessageGroup')}>
              <DropdownMenu.Label className="message-menu-label">{t('messageActionMessageGroup')}</DropdownMenu.Label>
              <DropdownMenu.Item className="message-menu-item" disabled={busy} onSelect={() => queue(() => report.show({ targetType: 'message', targetId: message.id, label: 'reportMessage' }))}><FlagIcon size={18} />{t('reportMessage')}</DropdownMenu.Item>
            </DropdownMenu.Group>
            {otherRider && <><DropdownMenu.Separator className="message-menu-separator" /><DropdownMenu.Group aria-label={t('messageActionRiderGroup')}>
              <DropdownMenu.Label className="message-menu-label">{t('messageActionRiderGroup')}</DropdownMenu.Label>
              <DropdownMenu.Item className="message-menu-item" disabled={busy} onSelect={() => queue(() => report.show({ targetType: 'user', targetId: message.sender_id, label: 'reportRider' }))}><FlagIcon size={18} />{t('reportRider')}</DropdownMenu.Item>
              {image && <DropdownMenu.Item className="message-menu-item" disabled={busy} onSelect={() => queue(() => report.show({ targetType: 'avatar', targetId: image.id, label: 'reportAvatar' }))}><FlagIcon size={18} />{t('reportAvatar')}</DropdownMenu.Item>}
              <DropdownMenu.Item className="message-menu-item" disabled={busy || block.disabled} onSelect={() => queue(() => { void block.change() })}><ProhibitIcon size={18} />{t(block.blocked ? 'unblockRider' : 'blockRider')}</DropdownMenu.Item>
            </DropdownMenu.Group></>}
            {admin && <><DropdownMenu.Separator className="message-menu-separator" /><DropdownMenu.Item className="message-menu-item" disabled={busy} onSelect={() => queue(manage)}><ShieldCheckIcon size={18} />{t('adminManageRider')}</DropdownMenu.Item></>}
          </DropdownMenu.Content>
        </DropdownMenu.Portal>
      </DropdownMenu.Root>
    </div>
    <div className="message-action-feedback">{report.content}{block.feedback}</div>
  </>
}
