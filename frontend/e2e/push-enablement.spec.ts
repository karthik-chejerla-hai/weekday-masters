import { test, expect, json } from './support/fixtures';
import type { Page } from '@playwright/test';
type PushEnrollment = { id: string; installation_id: string | null; generation: number; device_name: string; state: 'active' | 'revoked' | 'invalidated' };

async function fakeFirebase(page: Page, granted = false) {
 await page.route('**/src/services/firebase.ts', route => route.fulfill({
  contentType: 'application/javascript', body: `let permission = '${granted ? 'granted' : 'default'}';
   export const isFirebaseConfigured = () => true;
   export const getNotificationPermission = () => permission;
   export const requestNotificationPermission = async () => { permission = 'granted'; return 'test-device-token'; };
   export const getExistingPushToken = async () => permission === 'granted' ? 'test-device-token' : null;
   export const deletePushToken = async () => {};
   export const onForegroundMessage = () => () => {};`,
 }));
}

test('explicit setup recovers from failed registration and disable survives reload', async ({ page }) => {
 await fakeFirebase(page);
 let fail = true; let device: PushEnrollment | null = null; const enrollments: object[] = []; const refreshes: object[] = [];
 await page.route('**/api/users/me/push-devices', route => json(route, device ? [device] : []));
 await page.route('**/api/users/me/push-devices/enroll', async route => {
  const input = route.request().postDataJSON(); enrollments.push(input);
  if (fail) return json(route, { error: 'Setup unavailable' }, 503);
  device = { id: '11111111-1111-4111-8111-111111111111', installation_id: input.installation_id, generation: device ? device.generation + 1 : 1, device_name: 'This browser', state: 'active' };
  await json(route, device);
 });
 await page.route('**/api/users/me/push-devices/*/registration', async route => { refreshes.push(route.request().postDataJSON()); await json(route, device); });
 await page.route('**/api/users/me/push-devices/adopt', route => json(route, { error: 'No legacy token' }, 404));
 await page.route('**/api/users/me/push-devices/11111111-1111-4111-8111-111111111111', async route => {
  if (device) device = { ...device, state: 'revoked', generation: device.generation + 1 };
  await route.fulfill({ status: 204 });
 });
 await page.goto('/dashboard');
 const setup = page.getByRole('region', { name: 'Push notification setup' });
 await setup.getByRole('button', { name: 'Enable on this device' }).click();
 await expect(setup.getByText('Push alerts need attention')).toBeVisible();
 await setup.getByRole('link', { name: 'Notification settings' }).click();
 await expect(page.getByText('Push is not set up on this device')).toBeVisible();
 fail = false; await page.getByRole('button', { name: 'Enable on this device' }).click();
 await expect(page.getByRole('button', { name: 'Disable on this device' })).toBeVisible();
 expect(enrollments).toHaveLength(2);await expect(page.getByRole('switch')).toHaveCount(0);
 await page.getByRole('button', { name: 'Disable on this device' }).click();
 await expect(page.getByText('Push disabled on this device.')).toBeVisible();
 const count = refreshes.length; await page.reload();
 await expect(page.getByRole('button', { name: 'Enable on this device' })).toBeVisible();
 expect(refreshes).toHaveLength(count); expect(enrollments).toHaveLength(2);
});

test('existing registered token is silently retained and adopted regardless of old flags', async ({ page }) => {
 await fakeFirebase(page, true); let device: PushEnrollment = { id: '11111111-1111-4111-8111-111111111111', installation_id: null, generation: 1, device_name: 'Legacy phone', state: 'active' };let adopted = 0;
 await page.route('**/api/users/me/push-devices', route => json(route, [device]));
 await page.route('**/api/users/me/push-devices/adopt', async route => {
  const input = route.request().postDataJSON();expect(input.token).toBe('test-device-token');device = { ...device, installation_id: input.installation_id };adopted++;await json(route, device);
 });
 await page.route('**/api/users/me/push-devices/*/registration', route => json(route, device));
 await page.goto('/profile');
 await expect(page.getByRole('button', { name: 'Disable on this device' })).toBeVisible();expect(adopted).toBe(1);
 await expect(page.getByRole('switch')).toHaveCount(0);
 await page.reload();await expect(page.getByRole('button', { name: 'Disable on this device' })).toBeVisible();expect(adopted).toBe(1);
});

test.describe('admin self-test', () => {
 test.use({ identity: 'admin' });
 test('remains available after existing device adoption', async ({ page }) => {
  await fakeFirebase(page, true);let device: PushEnrollment = { id: '11111111-1111-4111-8111-111111111111', installation_id: null, generation: 1, device_name: 'Admin phone', state: 'active' };
  await page.route('**/api/users/me/push-devices', route => json(route, [device]));
  await page.route('**/api/users/me/push-devices/adopt', async route => { device = { ...device, installation_id: route.request().postDataJSON().installation_id };await json(route, device); });
  await page.route('**/api/users/me/push-devices/*/registration', route => json(route, device));
  await page.route('**/api/admin/notifications/test-push', route => json(route, { accepted_devices: 1, attempted_devices: 1 }));
  await page.goto('/profile');await page.getByRole('button', { name: 'Send test notification' }).click();
  await expect(page.getByText('Test notification accepted by 1 of 1 registered devices.')).toBeVisible();
 });
});

for (const identity of ['member', 'pending'] as const) {
 test.describe(`first legacy account switch to ${identity}`, () => {
  test.use({ identity });
  test('retires foreign unadopted token before completing sign-in without enrolling', async ({ page }) => {
   await page.route('**/src/services/firebase.ts', route => route.fulfill({ contentType: 'application/javascript', body: `let token = 'former-legacy-token';
    export const isFirebaseConfigured = () => true;
    export const getNotificationPermission = () => 'granted';
    export const requestNotificationPermission = async () => token;
    export const getExistingPushToken = async () => token;
    export const deletePushToken = async () => { token = 'new-browser-address'; window.__legacyDeleted = true; };
    export const onForegroundMessage = () => () => {};` }));
   let retireCalls = 0;
   await page.route('**/api/push-devices/legacy-revoke', async route => {
    expect(route.request().postDataJSON()).toEqual({ token: 'former-legacy-token' });
    const pending = await page.evaluate(() => JSON.parse(localStorage.getItem('rally:push-installation:v2') || '{}'));
    expect(pending.legacyCleanupToken).toBe('former-legacy-token');retireCalls++;await json(route, { cleanup_required: true });
   });
   await page.route('**/api/users/me/push-devices/adopt', route => json(route, { error: 'No legacy token' }, 404));
   await page.goto(identity === 'pending' ? '/' : '/profile');
   if (identity === 'pending') await expect(page).toHaveURL(/\/pending$/);
   else await expect(page.getByRole('button', { name: 'Enable on this device' })).toBeVisible();
   expect(retireCalls).toBe(1);
   expect(await page.evaluate(() => (window as unknown as { __legacyDeleted?: boolean }).__legacyDeleted)).toBe(true);
   const saved = await page.evaluate(() => JSON.parse(localStorage.getItem('rally:push-installation:v2') || '{}'));
   expect(saved.owner).toBe('member-1');expect(saved.device).toBeUndefined();expect(saved.legacyCleanupToken).toBeUndefined();
  });
 });
}
