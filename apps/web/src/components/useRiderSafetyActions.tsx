import * as DropdownMenu from '@radix-ui/react-dropdown-menu'
import { FlagIcon, ProhibitIcon, ShieldCheckIcon } from '@phosphor-icons/react'
import { useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { useI18n } from '../i18n'
import { useAvatarMetadata } from './Avatar'
import type { AvatarScope } from './Avatar'
import { useBlockAction, useTrust } from './Trust'
import type { useReportAction } from './Trust'
import type { SelectRiderAction } from './RiderActionMenu'

export function useRiderSafetyActions({ userId, name, actorId, scope, report }: {
  userId: string; name: string; actorId: string; scope: AvatarScope
  report: ReturnType<typeof useReportAction>
}) {
  const { t } = useI18n()
  const client = useQueryClient()
  const navigate = useNavigate()
  const block = useBlockAction({ userId, name })
  const avatar = useAvatarMetadata(userId, scope)
  const trust = useTrust()
  const otherRider = actorId !== userId
  const image = avatar.ready && !avatar.query.isError ? avatar.query.data?.image : undefined
  const admin = otherRider && trust.isSuccess && trust.data.is_admin
  const busy = report.busy || block.busy
  function manage() {
    const key = ['trust', actorId]
    if (client.getQueryState(key)?.status === 'success' && client.getQueryData<{ is_admin: boolean }>(key)?.is_admin) {
      void navigate('/admin?user=' + encodeURIComponent(userId))
    }
  }
  function items(select: SelectRiderAction) {
    if (!otherRider) return null
    return <><DropdownMenu.Group aria-label={t('messageActionRiderGroup')}>
      <DropdownMenu.Label className="message-menu-label">{t('messageActionRiderGroup')}</DropdownMenu.Label>
      <DropdownMenu.Item className="message-menu-item" disabled={busy} onSelect={() => select(() => report.show({ targetType: 'user', targetId: userId, label: 'reportRider' }))}><FlagIcon size={18} />{t('reportRider')}</DropdownMenu.Item>
      {image && <DropdownMenu.Item className="message-menu-item" disabled={busy} onSelect={() => select(() => report.show({ targetType: 'avatar', targetId: image.id, label: 'reportAvatar' }))}><FlagIcon size={18} />{t('reportAvatar')}</DropdownMenu.Item>}
      <DropdownMenu.Item className="message-menu-item" disabled={busy || block.disabled} onSelect={() => select(() => { void block.change() })}><ProhibitIcon size={18} />{t(block.blocked ? 'unblockRider' : 'blockRider')}</DropdownMenu.Item>
    </DropdownMenu.Group>{admin && <><DropdownMenu.Separator className="message-menu-separator" /><DropdownMenu.Item className="message-menu-item" disabled={busy} onSelect={() => select(manage)}><ShieldCheckIcon size={18} />{t('adminManageRider')}</DropdownMenu.Item></>}</>
  }
  return { otherRider, busy, items, feedback: block.feedback }
}
