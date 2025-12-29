// Firebase messaging service worker for handling background push notifications
// This file must be in the public folder and served from the root

importScripts('https://www.gstatic.com/firebasejs/10.7.0/firebase-app-compat.js');
importScripts('https://www.gstatic.com/firebasejs/10.7.0/firebase-messaging-compat.js');

// Firebase config will be injected at runtime via message from main app
let firebaseConfig = null;

// Listen for config message from main app
self.addEventListener('message', (event) => {
  if (event.data && event.data.type === 'FIREBASE_CONFIG') {
    firebaseConfig = event.data.config;
    initializeFirebase();
  }
});

function initializeFirebase() {
  if (!firebaseConfig) {
    console.log('Firebase config not available yet');
    return;
  }

  try {
    firebase.initializeApp(firebaseConfig);
    const messaging = firebase.messaging();

    // Handle background messages
    messaging.onBackgroundMessage((payload) => {
      console.log('Received background message:', payload);

      const notificationTitle = payload.notification?.title || 'Weekday Masters';
      const notificationOptions = {
        body: payload.notification?.body || '',
        icon: '/icons/icon-192x192.svg',
        badge: '/icons/icon-192x192.svg',
        data: payload.data,
        tag: payload.data?.type || 'default',
        requireInteraction: true
      };

      self.registration.showNotification(notificationTitle, notificationOptions);
    });

    console.log('Firebase messaging initialized in service worker');
  } catch (error) {
    console.error('Failed to initialize Firebase in service worker:', error);
  }
}

// Handle notification click
self.addEventListener('notificationclick', (event) => {
  console.log('Notification clicked:', event);
  event.notification.close();

  const data = event.notification.data;
  let url = '/dashboard';

  // Navigate to specific page based on notification type
  if (data?.session_id) {
    url = `/sessions/${data.session_id}`;
  } else if (data?.type === 'admin_announcement') {
    url = '/dashboard';
  }

  event.waitUntil(
    clients.matchAll({ type: 'window', includeUncontrolled: true }).then((clientList) => {
      // Check if there's already a window open
      for (const client of clientList) {
        if (client.url.includes(self.location.origin) && 'focus' in client) {
          client.postMessage({ type: 'NOTIFICATION_CLICK', data });
          return client.focus();
        }
      }
      // Open new window if none exists
      return clients.openWindow(url);
    })
  );
});
