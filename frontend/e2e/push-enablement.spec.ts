import { test, expect, json, preferences } from './support/fixtures';

test('member explicitly enables push and can recover from a failed token save', async ({ page }, testInfo) => {
  // Keep Rally's notification service and API client; replace only Firebase.
  // Browser tests never request real permission or register a real device token.
  await page.route('**/src/services/firebase.ts', route => route.fulfill({
    contentType: 'application/javascript',
    body: `let permission = 'default';
      export const isFirebaseConfigured = () => true;
      export const getNotificationPermission = () => permission;
      export const requestNotificationPermission = async () => { permission = 'granted'; return 'test-device-token'; };
      export const onForegroundMessage = () => () => {};`,
  }));

  const saved = { ...preferences };
  await page.route('**/api/users/me/notifications', async route => {
    if (route.request().method() === 'PUT') Object.assign(saved, route.request().postDataJSON());
    await json(route, saved);
  });

  let failRegistration = true;
  const tokens: unknown[] = [];
  await page.route('**/api/users/me/push-tokens', async route => {
    tokens.push(route.request().postDataJSON());
    await json(route, failRegistration ? { error: 'Registration unavailable' } : { message: 'Registered' }, failRegistration ? 503 : 200);
  });

  await page.goto('/dashboard');
  const setup = page.getByRole('region', { name: 'Push notification setup' });
  await expect(setup).toBeVisible();
  await expect(setup.getByText('Stay in the Rally loop')).toBeVisible();
  if (process.env.CAPTURE_REVIEW) await page.screenshot({ path: testInfo.outputPath('push-enablement.png'), fullPage: true });
  expect(tokens).toHaveLength(0);

  await setup.getByRole('button', { name: 'Enable on this device' }).click();
  await expect(setup.getByText('Push alerts need attention')).toBeVisible();
  await setup.getByRole('link', { name: 'Notification settings' }).click();

  await expect(page.getByText('This device still needs push setup')).toBeVisible();
  await expect(page.getByRole('switch', { name: 'Balance Alerts', exact: true })).toHaveCount(0);
  failRegistration = false;
  await page.getByRole('button', { name: 'Enable on this device' }).click();

  await expect(page.getByText('This device is registered for push notifications')).toBeVisible();
  await expect(page.getByRole('switch', { name: 'Balance Alerts', exact: true })).toBeVisible();
  expect(tokens).toContainEqual(expect.objectContaining({ token: 'test-device-token' }));
  expect(saved.push_enabled).toBe(true);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
});
