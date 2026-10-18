// Register before importing FCM, which installs its own click handler.
self.addEventListener('notificationclick', (event) => {
  event.stopImmediatePropagation();
  event.notification.close();
  const data = event.notification.data?.FCM_MSG?.data || event.notification.data || {};
  const id = '[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}';
  const uuid = new RegExp(`^${id}$`);
  const resource = new RegExp(`^/(?:sessions/${id}(?:/settlement(?:\\?settlement_id=${id})?)?|notifications/${id})$`);
  const safePath = typeof data.destination_path === 'string' &&
    (['/money', '/notifications', '/dashboard'].includes(data.destination_path) || resource.test(data.destination_path));
  const path = safePath ? data.destination_path
    : uuid.test(data.session_id || '') ? `/sessions/${data.session_id}${['session_charge', 'session_charge_reversed'].includes(data.type) ? `/settlement${uuid.test(data.settlement_id || '') ? `?settlement_id=${data.settlement_id}` : ''}` : ''}`
    : ['balance_low', 'balance_negative'].includes(data.type) ? '/money'
    : uuid.test(data.notification_id || '') ? `/notifications/${data.notification_id}` : '/notifications';
  const url = new URL(path, self.location.origin).href;
  event.waitUntil((async () => {
    const windows = await self.clients.matchAll({ type: 'window', includeUncontrolled: true });
    for (const client of windows) {
      if (new URL(client.url).origin === self.location.origin && 'navigate' in client) {
        await client.navigate(url);
        return client.focus();
      }
    }
    return self.clients.openWindow(url);
  })());
});

self.addEventListener('install', (event) => event.waitUntil(self.skipWaiting()));

importScripts('https://www.gstatic.com/firebasejs/10.7.0/firebase-app-compat.js');
importScripts('https://www.gstatic.com/firebasejs/10.7.0/firebase-messaging-compat.js');

const firebaseConfig = /* FIREBASE_CONFIG */ {};
if (firebaseConfig.apiKey && firebaseConfig.projectId && firebaseConfig.appId) {
  firebase.initializeApp(firebaseConfig);
  const messaging = firebase.messaging();
  messaging.onBackgroundMessage((payload) => {
    // FCM already displays notification payloads. Display data-only messages once.
    if (payload.notification) return;
    return self.registration.showNotification(payload.data?.title || 'Rally', {
      body: payload.data?.body || '',
      icon: '/icons/icon-192x192.svg',
      data: payload.data,
    });
  });
}
