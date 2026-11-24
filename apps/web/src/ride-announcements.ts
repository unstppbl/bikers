import { queryOptions } from '@tanstack/react-query'
import { request } from './api'
import type { Announcement } from './types'

export interface AnnouncementPage { announcements: Announcement[]; next_cursor: string }

export function announcementPageQuery(rideId: string, actor: string | undefined, before = '') {
  return queryOptions({
    queryKey: ['announcements', rideId, actor, before],
    meta: { private: true },
    retry: false,
    refetchInterval: 15_000,
    queryFn: async ({ signal }) => {
      const value = await request<AnnouncementPage>(`/rides/${rideId}/announcements${before ? `?before=${encodeURIComponent(before)}` : ''}`, { signal })
      const announcements = value.announcements.map(({ id, sequence, owner_id, owner_name, body, created_at, client_announcement_id }) => ({ id, sequence, owner_id, owner_name, body, created_at, client_announcement_id }))
      return { announcements, next_cursor: value.next_cursor }
    },
  })
}
