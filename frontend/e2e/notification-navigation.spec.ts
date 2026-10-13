import { test, expect, NOW, json } from './support/fixtures';
const id = '11111111-1111-4111-8111-111111111111';
const notice = { id, user_id: 'member-1', notification_type: 'admin_announcement', title: 'Court change', body: 'We play at court four.', data: '{}', push_sent: false, email_sent: false, created_at: NOW, destination_path: `/notifications/${id}` };
test('history entry opens the exact announcement', async ({ page }) => {
 await page.route('**/api/users/me/notifications/history?*', route => json(route, [notice]));
 await page.route(`**/api/notifications/${id}`, route => json(route, notice));
 await page.route(`**/api/notifications/${id}/read`, route => json(route, { message: 'read' }));
 await page.goto('/dashboard');
 await page.getByRole('link', { name: 'Notifications', exact: true }).click();
 await page.getByRole('link', { name: /Court change/ }).click();
 await expect(page).toHaveURL(new RegExp(`/notifications/${id}$`));
 await expect(page.getByRole('heading', { name: 'Court change' })).toBeVisible();
 await expect(page.getByText('We play at court four.')).toBeVisible();
});
test('missing own notification displays useful feedback', async ({ page }) => {
 await page.route(`**/api/notifications/${id}`, route => json(route, { error: 'Notification not found' }, 404));
 await page.goto(`/notifications/${id}`);
 await expect(page.getByRole('alert')).toHaveText('This notification is unavailable for your account.');
 await expect(page.getByRole('link', { name: 'All notifications' })).toBeVisible();
});
test.describe('signed-out return', () => {
 test.use({ identity: 'visitor' });
 test('preserves split destination into OAuth appState', async ({ page }) => {
  await page.goto(`/sessions/${id}/settlement`);
  await page.getByRole('button', { name: 'Sign in with Google' }).click();
  const options = await page.evaluate(() => JSON.parse(sessionStorage.getItem('e2e:login') || '{}'));
  expect(options.appState.returnTo).toBe(`/sessions/${id}/settlement`);
 });
});
test.describe('membership gate', () => {
 test.use({ identity: 'pending' });
 test('pending members cannot open notification content', async ({ page }) => {
  await page.goto(`/notifications/${id}`);
  await expect(page).toHaveURL(/\/pending$/);
  await expect(page.getByText('We play at court four.')).toHaveCount(0);
 });
});
