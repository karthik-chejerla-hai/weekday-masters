// Register before importing FCM, which installs its own click handler.
self.addEventListener('notificationclick', (event) => {
  event.stopImmediatePropagation();
  event.notification.close();
  const data = event.notification.data?.FCM_MSG?.data || event.notification.data || {};
  const path = data.session_id
    ? `/sessions/${encodeURIComponent(data.session_id)}`
    : data.type?.startsWith('balance_') ? '/money' : '/dashboard';
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
