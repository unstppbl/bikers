/// <reference lib="webworker" />
import { pushNotification } from './push-notification'
import { clientsClaim } from 'workbox-core'
import { cleanupOutdatedCaches, createHandlerBoundToURL, precacheAndRoute } from 'workbox-precaching'
import { NavigationRoute, registerRoute } from 'workbox-routing'

declare const self: ServiceWorkerGlobalScope & { __WB_MANIFEST: { url: string; revision: string | null }[] }
clientsClaim()
cleanupOutdatedCaches()
precacheAndRoute(self.__WB_MANIFEST)
registerRoute(new NavigationRoute(createHandlerBoundToURL('index.html'), { denylist: [/^\/api(?:\/|$)/, /^\/media(?:\/|$)/, /^\/bike-models(?:\/|$)/] }))
self.addEventListener('message', (event) => { if (event.data?.type === 'SKIP_WAITING') void self.skipWaiting() })
self.addEventListener('push', (event) => {
  let payload: unknown
  try { payload = event.data?.json() } catch { /* Invalid payloads get a generic notice. */ }
  event.waitUntil(self.registration.showNotification('BIKERS.KZ', pushNotification(payload)))
})
self.addEventListener('notificationclick', (event) => {
  event.notification.close()
  event.waitUntil((async () => {
    const url = new URL('/inbox', self.location.origin).href
    const windows = await self.clients.matchAll({ type: 'window', includeUncontrolled: true })
    const existing = windows.find((client) => new URL(client.url).origin === self.location.origin) as WindowClient | undefined
    if (existing) { await existing.navigate(url); await existing.focus() }
    else await self.clients.openWindow(url)
  })())
})
