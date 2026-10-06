// @vitest-environment node
import { readFileSync } from 'node:fs';
import { runInNewContext } from 'node:vm';
import { expect, it, vi } from 'vitest';

function startWorker() {
  const handlers: Record<string, (event: any) => void> = {}; // eslint-disable-line @typescript-eslint/no-explicit-any
  let background: (payload: object) => unknown = () => {};
  const showNotification = vi.fn();
  const initializeApp = vi.fn();
  const client = { url: 'https://rally.test/dashboard', navigate: vi.fn(async () => {}), focus: vi.fn() };
  const source = readFileSync(new URL('../../public/firebase-messaging-sw.js', import.meta.url), 'utf8')
    .replace('/* FIREBASE_CONFIG */ {}', JSON.stringify({ apiKey: 'key', projectId: 'project', appId: 'app' }));
  runInNewContext(source, {
    URL,
    importScripts: vi.fn(),
    self: {
      addEventListener: (name: string, handler: typeof handlers[string]) => { handlers[name] = handler; },
      location: { origin: 'https://rally.test' },
      registration: { showNotification },
      clients: { matchAll: async () => [client], openWindow: vi.fn() },
    },
    firebase: { initializeApp, messaging: () => ({ onBackgroundMessage: (handler: typeof background) => { background = handler; } }) },
  });
  return { handlers, showNotification, initializeApp, client, background };
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
