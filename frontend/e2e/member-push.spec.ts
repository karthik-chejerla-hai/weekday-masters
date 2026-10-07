import { test, expect, members, preferences, NOW, json } from './support/fixtures';

test.use({ identity: 'admin' });
test('admin can see each member’s saved push choices and devices', async ({ page }) => {
  await page.route('**/api/admin/users', route => json(route, members));
  await page.route('**/api/admin/users/*/push-notifications', route => json(route, {
    preferences: { ...preferences, push_enabled: true, push_session_reminders: true },
    devices: [{ id: 'device-1', device_name: '', created_at: NOW, last_registered_at: NOW }],
  }));
  await page.goto('/admin/members');
  const alex = page.getByRole('region', { name: 'Push alerts for Alex', exact: true });
  await alex.getByRole('button', { name: 'Push alerts and devices' }).click();
  await expect(alex.getByText('Account push: On')).toBeVisible();
  await expect(alex.getByText('1 registered device')).toBeVisible();
  await expect(alex.getByText('Unnamed browser 1')).toBeVisible();
  await expect(alex.getByText('Balance alerts')).toBeVisible();
  await expect(page.getByRole('region', { name: 'Push alerts for Blair' }).getByRole('button')).toHaveAttribute('aria-expanded', 'false');
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
});
