import { useRef } from 'react'
import type { ReactNode, RefObject } from 'react'
import * as DropdownMenu from '@radix-ui/react-dropdown-menu'
import { DotsThreeIcon } from '@phosphor-icons/react'
import { useSessionTask } from '../useSessionTask'
import type { SessionTask } from '../useSessionTask'
import './message-actions.css'

export type SelectRiderAction = (run: () => void) => void

// Keep workflow owners outside this transient menu. Run selected actions only
// after its focus scope closes, so confirmations capture the persistent trigger.
export function RiderActionMenu({ label, title, trigger, children }: {
  label: string; title: string; trigger: RefObject<HTMLButtonElement | null>
  children: (select: SelectRiderAction) => ReactNode
}) {
  const pending = useRef<{ task: SessionTask; run: () => void } | undefined>(undefined)
  const start = useSessionTask()
  function select(run: () => void) {
    pending.current?.task.abort(); pending.current?.task.finish()
    pending.current = { task: start(), run }
  }
  function afterClose(event: Event) {
    const next = pending.current
    pending.current = undefined
    if (!next) return
    event.preventDefault()
    if (next.task.current()) { trigger.current?.focus({ preventScroll: true }); next.run() }
    next.task.finish()
  }
  // This render prop only constructs items. The ref-reading select callback
  // runs later from their onSelect handlers, never during rendering.
  // eslint-disable-next-line react-hooks/refs
  const content = children(select)
  return <DropdownMenu.Root>
    <DropdownMenu.Trigger asChild><button ref={trigger} type="button" className="message-action-trigger" aria-label={label} title={title}><DotsThreeIcon size={24} weight="bold" /></button></DropdownMenu.Trigger>
    <DropdownMenu.Portal><DropdownMenu.Content className="message-menu-content" align="end" sideOffset={8} collisionPadding={16} onCloseAutoFocus={afterClose}>{content}</DropdownMenu.Content></DropdownMenu.Portal>
  </DropdownMenu.Root>
}
