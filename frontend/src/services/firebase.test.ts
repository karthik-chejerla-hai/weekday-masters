import { beforeEach, expect, it, vi } from 'vitest';

const sdk = vi.hoisted(() => ({
  initializeApp: vi.fn(() => ({})),
  getMessaging: vi.fn(() => ({})),
  isSupported: vi.fn(async () => true),
  getToken: vi.fn(async () => 'device-token'),
  onMessage: vi.fn(() => vi.fn()),
}));
vi.mock('firebase/app', () => ({ initializeApp: sdk.initializeApp }));
vi.mock('firebase/messaging', () => sdk);

beforeEach(() => {
  vi.resetModules();
  vi.clearAllMocks();
  vi.stubEnv('VITE_FIREBASE_API_KEY', 'public-key');
  vi.stubEnv('VITE_FIREBASE_PROJECT_ID', 'project');
  vi.stubEnv('VITE_FIREBASE_MESSAGING_SENDER_ID', 'sender');
  vi.stubEnv('VITE_FIREBASE_APP_ID', 'app');
  vi.stubEnv('VITE_FIREBASE_VAPID_KEY', 'vapid');
  vi.stubGlobal('Notification', { permission: 'granted', requestPermission: vi.fn() });
});

it('uses the dedicated active registration when getting a token', async () => {
  const registration = { active: {} };
  const register = vi.fn(async () => registration);
  Object.defineProperty(navigator, 'serviceWorker', { configurable: true, value: { register } });
  const { requestNotificationPermission } = await import('./firebase');
  expect(await requestNotificationPermission()).toBe('device-token');
  expect(register).toHaveBeenCalledWith('/firebase-messaging-sw.js', {
    scope: '/firebase-cloud-messaging-push-scope', updateViaCache: 'none',
  });
  expect(sdk.getToken).toHaveBeenCalledWith(expect.anything(), {
    vapidKey: 'vapid', serviceWorkerRegistration: registration,
  });
  expect(Notification.requestPermission).not.toHaveBeenCalled();
});

it('waits for its installing worker even when the PWA worker is ready', async () => {
  const worker = new EventTarget() as EventTarget & { state: string };
  worker.state = 'installing';
  const registration = { installing: worker, active: null };
  Object.defineProperty(navigator, 'serviceWorker', { configurable: true, value: {
    register: vi.fn(async () => registration), ready: Promise.resolve({ active: {} }),
  } });
  const { requestNotificationPermission } = await import('./firebase');
  const token = requestNotificationPermission();
  await vi.waitFor(() => expect(sdk.initializeApp).toHaveBeenCalled());
  expect(sdk.getToken).not.toHaveBeenCalled();
  worker.state = 'activated';
  worker.dispatchEvent(new Event('statechange'));
  expect(await token).toBe('device-token');
});

it('retries initialization after a failed worker registration', async () => {
  const register = vi.fn().mockRejectedValueOnce(new Error('offline')).mockResolvedValue({ active: {} });
  Object.defineProperty(navigator, 'serviceWorker', { configurable: true, value: { register } });
  const { requestNotificationPermission } = await import('./firebase');
  expect(await requestNotificationPermission()).toBeNull();
  expect(await requestNotificationPermission()).toBe('device-token');
  expect(register).toHaveBeenCalledTimes(2);
});

it('does not attach a late foreground listener after unmount', async () => {
  Object.defineProperty(navigator, 'serviceWorker', { configurable: true, value: {
    register: vi.fn(async () => ({ active: {} })),
  } });
  const { onForegroundMessage, initializeMessaging } = await import('./firebase');
  const cleanup = onForegroundMessage(vi.fn());
  cleanup();
  await initializeMessaging();
  expect(sdk.onMessage).not.toHaveBeenCalled();
});
