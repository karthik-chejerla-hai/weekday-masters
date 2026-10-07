import { test, expect, json, preferences } from './support/fixtures';

test('player registers this device and can retry a failed token save', async ({ page }, testInfo) => {
  // Keep the real notification service and API client; replace only the external
  // Firebase boundary. No browser test contacts Firebase or registers a real token.
  await page.route('**/src/services/firebase.ts', route => route.fulfill({
    contentType: 'application/javascript',
    body: `let permission = 'default';
      export const isFirebaseConfigured = () => true;
      export const getNotificationPermission = () => permission;
      export const requestNotificationPermission = async () => { permission = 'granted'; return 'test-device-token'; };
      export const onForegroundMessage = () => () => {};`,
  }));
  const saved = { ...preferences, push_enabled: true, push_balance_alerts: true };
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
  await page.goto('/sessions');
  await expect(page.getByRole('region', { name: 'Push notification setup' })).toBeVisible();
  expect(tokens).toHaveLength(0);
  if (process.env.CAPTURE_REVIEW) await page.screenshot({ path: `../docs/review/push-setup-prompt-${testInfo.project.name}.png` });
  await page.getByRole('link', { name: 'Set up push alerts' }).click();
  await expect(page.getByText('Push alerts are not ready on this device')).toBeVisible();
  await expect(page.getByRole('switch', { name: 'Balance Alerts', exact: true })).toHaveCount(0);
  await page.getByRole('button', { name: 'Set up this device' }).click();
  await expect(page.getByText('Could not complete device setup. Check your connection and browser permission, then try again.')).toBeVisible();
  await expect(page.getByText('This device is registered', { exact: true })).toHaveCount(0);
  if (process.env.CAPTURE_REVIEW) await page.locator('#push-notifications').screenshot({ path: `../docs/review/push-setup-incomplete-${testInfo.project.name}.png` });
  failRegistration = false;
  await page.getByRole('button', { name: 'Set up this device' }).click();
  await expect(page.getByText('This device is registered', { exact: true })).toBeVisible();
  await expect(page.getByRole('switch', { name: 'Balance Alerts', exact: true })).toHaveAttribute('aria-checked', 'true');
  expect(tokens).toContainEqual(expect.objectContaining({ token: 'test-device-token' }));
  if (process.env.CAPTURE_REVIEW) await page.locator('#push-notifications').screenshot({ path: `../docs/review/push-setup-ready-${testInfo.project.name}.png` });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
});
