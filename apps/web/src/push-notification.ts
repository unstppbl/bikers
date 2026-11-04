const genericBodies = ['You have a new update', 'У вас новое обновление', 'Сізде жаңа жаңарту бар']

// The lock screen and click destination never use arbitrary server-provided text or URLs.
export function pushNotification(value: unknown) {
  const payload = value && typeof value === 'object' ? value : {}
  const body = 'body' in payload && typeof payload.body === 'string' && genericBodies.includes(payload.body) ? payload.body : genericBodies[0]
  const id = 'notification_id' in payload && typeof payload.notification_id === 'string' && /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(payload.notification_id) ? payload.notification_id : 'updates'
  return { body, icon: '/app-icon-192.png', badge: '/app-icon-192.png', tag: `bikerskz-${id}`, data: { url: '/inbox' } }
}
