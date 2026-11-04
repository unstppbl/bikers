import { useQuery } from '@tanstack/react-query'
import { ApiError, request } from '../api'
import { useI18n } from '../i18n'
import { useSession } from '../session'

export function useUnreadNotifications() {
  const { user, loading, error } = useSession()
  const query = useQuery({
    queryKey: ['notification-count', user?.id],
    queryFn: async ({ signal }) => {
      const value = await request<{ unread_count: unknown }>('/me/notifications/unread-count', { signal })
      if (!Number.isSafeInteger(value.unread_count) || Number(value.unread_count) < 0) throw new ApiError(502, 'request_failed')
      return Number(value.unread_count)
    },
    enabled: !!user && !loading && !error,
    meta: { private: true },
    retry: false,
    staleTime: 15_000,
    refetchInterval: 30_000,
  })
  return { ...query, data: user && !loading && !error ? query.data : undefined }
}

export function NotificationBadge({ count }: { count: number }) {
  const { t } = useI18n()
  return count > 0 ? <span className="notification-badge" aria-label={t('unreadUpdates', { count })}>{count > 99 ? '99+' : count}</span> : null
}
