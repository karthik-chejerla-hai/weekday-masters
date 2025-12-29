import { initializeApp, FirebaseApp } from 'firebase/app';
import { getMessaging, getToken, onMessage, Messaging } from 'firebase/messaging';

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

// Check if Firebase is configured
export const isFirebaseConfigured = (): boolean => {
  return !!(
    firebaseConfig.apiKey &&
    firebaseConfig.projectId &&
    firebaseConfig.messagingSenderId
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

// Initialize Firebase Messaging
export const initializeMessaging = async (): Promise<Messaging | null> => {
  if (!isFirebaseConfigured()) {
    return null;
  }

  // Check if browser supports notifications
  if (!('Notification' in window)) {
    console.log('This browser does not support notifications');
    return null;
  }

  if (!('serviceWorker' in navigator)) {
    console.log('This browser does not support service workers');
    return null;
  }

  if (!app) {
    app = initializeFirebase();
    if (!app) return null;
  }

  if (!messaging) {
    try {
      messaging = getMessaging(app);

      // Register the service worker and send Firebase config to it
      const registration = await navigator.serviceWorker.register('/firebase-messaging-sw.js');

      // Wait for service worker to be ready
      await navigator.serviceWorker.ready;

      // Send Firebase config to service worker
      if (registration.active) {
        registration.active.postMessage({
          type: 'FIREBASE_CONFIG',
          config: firebaseConfig
        });
      }

      console.log('Firebase Messaging initialized');
    } catch (error) {
      console.error('Failed to initialize Firebase Messaging:', error);
      return null;
    }
  }

  return messaging;
};

// Request notification permission and get FCM token
export const requestNotificationPermission = async (): Promise<string | null> => {
  try {
    // Request permission
    const permission = await Notification.requestPermission();
    if (permission !== 'granted') {
      console.log('Notification permission denied');
      return null;
    }

    // Initialize messaging if needed
    const msg = await initializeMessaging();
    if (!msg) return null;

    // Get VAPID key from environment
    const vapidKey = import.meta.env.VITE_FIREBASE_VAPID_KEY;
    if (!vapidKey) {
      console.error('VAPID key not configured');
      return null;
    }

    // Get FCM token
    const token = await getToken(msg, { vapidKey });
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
  if (!messaging) {
    initializeMessaging().then((msg) => {
      if (msg) {
        onMessage(msg, (payload) => {
          callback({
            title: payload.notification?.title,
            body: payload.notification?.body,
            data: payload.data
          });
        });
      }
    });
    return () => {};
  }

  const unsubscribe = onMessage(messaging, (payload) => {
    callback({
      title: payload.notification?.title,
      body: payload.notification?.body,
      data: payload.data
    });
  });

  return unsubscribe;
};

// Check current notification permission status
export const getNotificationPermission = (): NotificationPermission | 'unsupported' => {
  if (!('Notification' in window)) {
    return 'unsupported';
  }
  return Notification.permission;
};
