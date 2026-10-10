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

test.describe('admin push test', () => {
  test.use({ identity: 'admin' });

  test('sends only after the admin device is registered', async ({ page }) => {
    await page.route('**/src/services/firebase.ts', route => route.fulfill({
      contentType: 'application/javascript',
      body: `export const isFirebaseConfigured = () => true;
        export const getNotificationPermission = () => 'granted';
        export const requestNotificationPermission = async () => 'admin-device-token';
        export const onForegroundMessage = () => () => {};`,
    }));

    await page.route('**/api/users/me/notifications', route => json(route, {
      ...preferences,
      push_enabled: true,
    }));
    await page.route('**/api/users/me/push-tokens', route => json(route, { message: 'Registered' }));

    const tests: unknown[] = [];
    await page.route('**/api/admin/notifications/test-push', async route => {
      tests.push(route.request().postDataJSON());
      await json(route, { accepted_devices: 1, attempted_devices: 1 });
    });

    await page.goto('/profile');
    const button = page.getByRole('button', { name: 'Send test notification' });
    await expect(button).toBeVisible();
    await button.click();

    await expect(page.getByText('Test notification accepted by 1 of 1 registered device.')).toBeVisible();
    expect(tests).toHaveLength(1);
  });
});
