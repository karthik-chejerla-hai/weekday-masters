// @vitest-environment node
import { readFileSync } from 'node:fs';
import { runInNewContext } from 'node:vm';
import { expect, it, vi } from 'vitest';

function startWorker() {
  const handlers: Record<string, (event: any) => void> = {}; // eslint-disable-line @typescript-eslint/no-explicit-any
  let background: (payload: object) => unknown = () => {};
  const showNotification = vi.fn();
  const initializeApp = vi.fn();
  const fetch = vi.fn().mockResolvedValue({ ok: true });
  const client = { url: 'https://rally.test/dashboard', navigate: vi.fn(async () => {}), focus: vi.fn() };
  const source = readFileSync(new URL('../../public/firebase-messaging-sw.js', import.meta.url), 'utf8')
    .replace('/* FIREBASE_CONFIG */ {}', JSON.stringify({ apiKey: 'key', projectId: 'project', appId: 'app' }));
  runInNewContext(source, {
    URL,
    fetch,
    importScripts: vi.fn(),
    self: {
      addEventListener: (name: string, handler: typeof handlers[string]) => { handlers[name] = handler; },
      location: { origin: 'https://rally.test' },
      registration: { showNotification },
      clients: { matchAll: async () => [client], openWindow: vi.fn() },
    },
    firebase: { initializeApp, messaging: () => ({ onBackgroundMessage: (handler: typeof background) => { background = handler; } }) },
  });
  return { handlers, showNotification, initializeApp, client, background, fetch };
}

it('initializes on a cold worker start without a page configuration message', () => {
  const worker = startWorker();
  expect(worker.initializeApp).toHaveBeenCalledWith({ apiKey: 'key', projectId: 'project', appId: 'app' });
  worker.background({ notification: { title: 'Reminder' } });
  expect(worker.showNotification).not.toHaveBeenCalled();
  worker.background({ data: { title: 'Data reminder', body: 'Play' } });
  expect(worker.showNotification).toHaveBeenCalledTimes(1);
});

it('opens the session in an existing window for an FCM notification', async () => {
  const worker = startWorker();
  let completion: Promise<unknown> | undefined;
  worker.handlers.notificationclick({
    stopImmediatePropagation: vi.fn(),
    notification: { close: vi.fn(), data: { FCM_MSG: { data: { session_id: 'session-1' } } } },
    waitUntil: (promise: Promise<unknown>) => { completion = promise; },
  });
  await completion;
  expect(worker.client.navigate).toHaveBeenCalledWith('https://rally.test/sessions/session-1');
  expect(worker.client.focus).toHaveBeenCalled();
});

it.each(['balance_low', 'balance_negative'])('opens Money for a %s background alert', async (type) => {
  const worker = startWorker();
  let completion: Promise<unknown> | undefined;
  worker.handlers.notificationclick({
    stopImmediatePropagation: vi.fn(),
    notification: { close: vi.fn(), data: { FCM_MSG: { data: { type, balance_cents: '100' } } } },
    waitUntil: (promise: Promise<unknown>) => { completion = promise; },
  });
  await completion;
  expect(worker.client.navigate).toHaveBeenCalledWith('https://rally.test/money');
});


it('confirms a background receipt without an open page or login token', async () => {
  const worker = startWorker();
  await worker.background({ notification: { title: 'Low balance' }, data: { notification_id: 'notice-1', receipt_token: 'capability' } });
  expect(worker.fetch).toHaveBeenCalledWith('/api/notifications/notice-1/push-receipt', {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ token: 'capability' }), credentials: 'omit',
  });
  expect(worker.showNotification).not.toHaveBeenCalled();
});

it('does not confirm a data-only notification if display fails', async () => {
  const worker = startWorker();
  worker.showNotification.mockRejectedValueOnce(new Error('permission denied'));
  await expect(worker.background({ data: { notification_id: 'notice-1', receipt_token: 'capability' } })).rejects.toThrow('permission denied');
  expect(worker.fetch).not.toHaveBeenCalled();
});

it('leaves fallback eligible when the receipt request fails', async () => {
  const worker = startWorker();
  worker.fetch.mockRejectedValueOnce(new Error('offline'));
  await expect(worker.background({ notification: { title: 'Low' }, data: { notification_id: 'notice-1', receipt_token: 'capability' } })).resolves.toBeUndefined();
});
