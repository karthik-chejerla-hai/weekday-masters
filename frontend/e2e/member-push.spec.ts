import { test, expect, members, preferences, NOW, json } from './support/fixtures';

test.use({ identity: 'admin' });
test('admin can see device setup without loading every member detail', async ({ page }) => {
  await page.route('**/api/admin/users', route => json(route, members.map(member => ({ ...member, push_device_count: member.id === members[0].id ? 1 : 0 }))));
  await page.route('**/api/admin/users/*/push-notifications', route => json(route, {
    preferences: { ...preferences, push_enabled: true, push_session_reminders: true },
    devices: [{ id: 'device-1', device_name: '', created_at: NOW, last_registered_at: NOW }],
  }));
  const detailRequests: string[] = [];
  page.on('request', request => { if (request.url().includes('/push-notifications')) detailRequests.push(request.url()); });
  await page.goto('/admin/members');
  await expect(page.getByRole('img', { name: 'Push set up · 1 device' })).toBeVisible();
  expect(detailRequests).toHaveLength(0);
  await page.getByRole('button', { name: 'Details for Alex', exact: true }).click();
  const alex = page.getByRole('region', { name: 'Push alerts for Alex', exact: true });
  await alex.getByRole('button', { name: 'Push alerts and devices' }).click();
  await expect(alex.getByText('Push set up')).toBeVisible();
  await expect(alex.getByText('1 registered device')).toBeVisible();
  await expect(alex.getByText('Unnamed browser 1')).toBeVisible();
  await expect(alex.getByText('All supported push alerts are included on registered devices.')).toBeVisible();
  await expect(page.getByRole('region', { name: 'Push alerts for Blair' })).toHaveCount(0);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
});

test('missing summaries and failed detail reads stay unknown rather than claiming no devices', async ({ page }) => {
  await page.route('**/api/admin/users', route => json(route, members));
  await page.route('**/api/admin/users/*/push-notifications', route => route.fulfill({ status: 503, json: { error: 'Unavailable' } }));
  await page.goto('/admin/members');
  await expect(page.getByRole('img', { name: 'Push status unavailable' }).first()).toBeVisible();
  await expect(page.getByRole('img', { name: 'No push devices' })).toHaveCount(0);
  await page.getByRole('button', { name: 'Details for Alex', exact: true }).click();
  const alex = page.getByRole('region', { name: 'Push alerts for Alex', exact: true });
  await alex.getByRole('button', { name: 'Push alerts and devices' }).click();
  await expect(alex.getByRole('alert')).toContainText('Could not load push settings');
  await expect(alex.getByText('No push devices')).toHaveCount(0);
});
