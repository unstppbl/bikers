import { createContext, useCallback, useContext, useEffect } from 'react'
import type { ReactNode } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Navigate, useLocation } from 'react-router-dom'
import { getSessionEpoch, request, setCsrfToken } from './api'
import type { Session, User } from './types'
import { useI18n } from './i18n'
import { Loading } from './ui'

interface SessionContextValue {
  user: User | null
  loading: boolean
  error: Error | null
  accept: (session: Session) => void
  updateUser: (user: User) => void
  clear: () => void
  refresh: () => Promise<unknown>
}
const SessionContext = createContext<SessionContextValue | null>(null)

export function SessionProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient()
  const session = useQuery({
    queryKey: ['session'],
    queryFn: async () => {
      const value = await request<Session>('/session')
      setCsrfToken(value.csrf_token)
      const previousActor = queryClient.getQueryData<Session>(['session'])?.user?.id
      if (!value.user || previousActor !== value.user.id) {
        const privateQuery = (query: { queryKey: readonly unknown[]; meta?: Record<string, unknown> }) => query.meta?.private === true || ['preferences', 'messages', 'participants', 'garage', 'bike', 'public-bike', 'public-bikes', 'bike-photos', 'maintenance', 'club', 'clubs', 'club-image', 'club-members', 'linkable-rides'].includes(String(query.queryKey[0])) || (query.queryKey[0] === 'ride' && query.queryKey[2] !== 'guest') || (query.queryKey[0] === 'rides' && query.queryKey[1] !== 'guest')
        void queryClient.cancelQueries({ predicate: privateQuery })
        queryClient.removeQueries({ predicate: privateQuery })
      }
      return value
    },
    retry: false,
    staleTime: 30_000,
  })
  const clear = useCallback(() => {
    setCsrfToken()
    void queryClient.cancelQueries()
    queryClient.removeQueries({ predicate: (query) => query.queryKey[0] !== 'session' })
    queryClient.setQueryData<Session>(['session'], { user: null })
  }, [queryClient])
  const accept = useCallback((value: Session) => {
    setCsrfToken(value.csrf_token)
    queryClient.removeQueries({ predicate: (query) => query.queryKey[0] !== 'session' })
    queryClient.setQueryData(['session'], value)
  }, [queryClient])
  const updateUser = useCallback((user: User) => {
    queryClient.setQueryData<Session>(['session'], (value) => ({ ...value, user }))
  }, [queryClient])
  useEffect(() => {
    window.addEventListener('bikerskz:session-expired', clear)
    return () => window.removeEventListener('bikerskz:session-expired', clear)
  }, [clear])
  return <SessionContext.Provider value={{
    user: session.data?.user ?? null,
    loading: session.isPending,
    error: session.error,
    accept, updateUser, clear, refresh: session.refetch,
  }}>{children}</SessionContext.Provider>
}

export function useSession() {
  const context = useContext(SessionContext)
  if (!context) throw new Error('SessionProvider missing')
  return context
}

// CSRF is regenerated for each authenticated session. Together with the actor,
// it binds an async completion to its original in-memory authentication epoch.
export function useSessionGuard() {
  const queryClient = useQueryClient()
  return useCallback(() => {
    const original = queryClient.getQueryData<Session>(['session'])
    const actor = original?.user?.id
    const csrf = original?.csrf_token
    const epoch = getSessionEpoch()
    return () => {
      const current = queryClient.getQueryData<Session>(['session'])
      return epoch === getSessionEpoch() && actor === current?.user?.id && csrf === current?.csrf_token
    }
  }, [queryClient])
}

export function RequireUser({ children, allowOnboarding = false }: { children: ReactNode; allowOnboarding?: boolean }) {
  const { user, loading, error, refresh } = useSession()
  const location = useLocation()
  const { t } = useI18n()
  if (loading) return <Loading label={t('loadingSession')} />
  if (error) return <div className="error-banner" role="alert"><span>{t('serviceUnavailable')}</span><button className="text-button" onClick={() => void refresh()}>{t('retry')}</button></div>
  const next = encodeURIComponent(location.pathname + location.search)
  if (!user) return <Navigate to={`/login?next=${next}`} replace />
  if (!allowOnboarding && !user.onboarding_complete) return <Navigate to={`/profile?next=${next}`} replace />
  return children
}
