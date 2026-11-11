import type { QueryClient } from '@tanstack/react-query'

// Active observers must drop old history and cursors before fetching filtered data.
export async function resetTrustQueries(client: QueryClient) {
  const predicate = (query: { queryKey: readonly unknown[] }) => ['blocks', 'messages', 'ride-album', 'avatar', 'announcements', 'ride-reminders', 'participants', 'club-members', 'badge', 'bike-photos', 'club-image', 'ride', 'rides', 'club', 'clubs', 'public-bike', 'community-routes', 'community-route', 'route-conditions', 'route-collections', 'recorded-tracks', 'recorded-track'].includes(String(query.queryKey[0]))
  await client.cancelQueries({ predicate })
  await Promise.all([client.resetQueries({ predicate }), client.invalidateQueries({ queryKey: ['trust'] })])
}
