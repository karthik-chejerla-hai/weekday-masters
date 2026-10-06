import { initializeApp, FirebaseApp } from 'firebase/app';
import { getMessaging, getToken, onMessage, isSupported, Messaging } from 'firebase/messaging';

// Firebase configuration from environment variables
const firebaseConfig = {
  apiKey: import.meta.env.VITE_FIREBASE_API_KEY,
  authDomain: import.meta.env.VITE_FIREBASE_AUTH_DOMAIN,
  projectId: import.meta.env.VITE_FIREBASE_PROJECT_ID,
  storageBucket: import.meta.env.VITE_FIREBASE_STORAGE_BUCKET,
  messagingSenderId: import.meta.env.VITE_FIREBASE_MESSAGING_SENDER_ID,
  appId: import.meta.env.VITE_FIREBASE_APP_ID
};

let app: FirebaseApp | null = null;
let messaging: Messaging | null = null;
let workerRegistration: ServiceWorkerRegistration | null = null;
let initialization: Promise<Messaging | null> | null = null;

// Check if Firebase is configured
export const isFirebaseConfigured = (): boolean => {
  return !!(
    firebaseConfig.apiKey &&
    firebaseConfig.projectId &&
    firebaseConfig.messagingSenderId &&
    firebaseConfig.appId &&
    import.meta.env.VITE_FIREBASE_VAPID_KEY
  );
};

// Initialize Firebase app
export const initializeFirebase = (): FirebaseApp | null => {
  if (!isFirebaseConfigured()) {
    console.log('Firebase not configured - push notifications disabled');
    return null;
  }

  if (!app) {
    try {
      app = initializeApp(firebaseConfig);
      console.log('Firebase initialized');
    } catch (error) {
      console.error('Failed to initialize Firebase:', error);
      return null;
    }
  }

  return app;
};

// Use a separate scope so FCM cannot replace the PWA's root worker.
export const initializeMessaging = (): Promise<Messaging | null> => {
  if (initialization) return initialization;
  initialization = (async () => {
    try {
      if (!isFirebaseConfigured() || !(await isSupported())) return null;
      const firebaseApp = initializeFirebase();
      if (!firebaseApp) return null;
      const registration = await navigator.serviceWorker.register('/firebase-messaging-sw.js', {
        scope: '/firebase-cloud-messaging-push-scope',
        updateViaCache: 'none',
      });
      // navigator.serviceWorker.ready may refer to the unrelated PWA worker.
      await waitForActiveWorker(registration);
      workerRegistration = registration;
      messaging = getMessaging(firebaseApp);
      return messaging;
    } catch (error) {
      console.error('Failed to initialize Firebase Messaging:', error);
      return null;
    }
  })().then((result) => {
    if (!result) initialization = null;
    return result;
  });
  return initialization;
};

function waitForActiveWorker(registration: ServiceWorkerRegistration): Promise<void> {
  if (registration.active) return Promise.resolve();
  return new Promise((resolve, reject) => {
    const worker = registration.installing || registration.waiting;
    if (!worker) return reject(new Error('Push worker is unavailable'));
    const finish = (error?: Error) => {
      clearTimeout(timer);
      worker.removeEventListener('statechange', check);
      if (error) reject(error); else resolve();
    };
    const check = () => {
      if (worker.state === 'activated') finish();
      else if (worker.state === 'redundant') finish(new Error('Push worker installation failed'));
    };
    const timer = setTimeout(() => finish(new Error('Push worker activation timed out')), 15000);
    worker.addEventListener('statechange', check);
    check();
  });
}

// Request notification permission and get FCM token
export const requestNotificationPermission = async (): Promise<string | null> => {
  try {
    // Request permission
    const permission = Notification.permission === 'granted'
      ? 'granted' : await Notification.requestPermission();
    if (permission !== 'granted') {
      console.log('Notification permission denied');
      return null;
    }

    // Initialize messaging if needed
    const msg = await initializeMessaging();
    if (!msg || !workerRegistration) return null;

    // Get VAPID key from environment
    const vapidKey = import.meta.env.VITE_FIREBASE_VAPID_KEY;
    if (!vapidKey) {
      console.error('VAPID key not configured');
      return null;
    }

    // Get FCM token
    const token = await getToken(msg, { vapidKey, serviceWorkerRegistration: workerRegistration });
    console.log('FCM token obtained');

    return token;
  } catch (error) {
    console.error('Failed to get notification permission:', error);
    return null;
  }
};

// Set up foreground message handler
export const onForegroundMessage = (
  callback: (payload: { title?: string; body?: string; data?: Record<string, string> }) => void
): (() => void) => {
  let cancelled = false;
  let unsubscribe: (() => void) | undefined;
  initializeMessaging().then((msg) => {
    if (!msg || cancelled) return;
    unsubscribe = onMessage(msg, (payload) => callback({
      title: payload.notification?.title,
      body: payload.notification?.body,
      data: payload.data,
    }));
  });
  return () => { cancelled = true; unsubscribe?.(); };
};

// Check current notification permission status
export const getNotificationPermission = (): NotificationPermission | 'unsupported' => {
  if (!('Notification' in window)) {
    return 'unsupported';
  }
  return Notification.permission;
};
