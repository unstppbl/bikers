import { useCallback, useEffect, useRef } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { getSessionEpoch } from './api'
import { useSession, useSessionGuard } from './session'
import type { Session } from './types'

export interface SessionTask { signal: AbortSignal; abort: () => void; current: () => boolean; finish: () => void }

// Each authenticated operation owns a cancellation signal and may only publish
// a result while its original actor and in-memory session are still current.
export function useSessionTask() {
  const { user, error } = useSession()
  const epoch = getSessionEpoch()
  const capture = useSessionGuard()
  const queryClient = useQueryClient()
  const pending = useRef(new Set<AbortController>())
  useEffect(() => {
    const controllers = pending.current
    return () => { controllers.forEach((controller) => controller.abort()); controllers.clear() }
  }, [user?.id, epoch, error])
  return useCallback(() => {
    const controller = new AbortController()
    const currentSession = capture()
    const expectedActor = user?.id
    if (!expectedActor || queryClient.getQueryData<Session>(['session'])?.user?.id !== expectedActor || queryClient.getQueryState(['session'])?.status !== 'success') controller.abort()
    pending.current.add(controller)
    return {
      signal: controller.signal,
      abort: () => controller.abort(),
      current: () => !controller.signal.aborted && currentSession() && queryClient.getQueryData<Session>(['session'])?.user?.id === expectedActor && queryClient.getQueryState(['session'])?.status === 'success',
      finish: () => pending.current.delete(controller),
    }
  }, [capture, queryClient, user?.id])
}
