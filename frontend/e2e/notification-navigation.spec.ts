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

test('a reversed charge history entry opens its original split rather than the replacement', async ({ page }) => {
 const settlement = '22222222-2222-4222-8222-222222222222';
 const path = `/sessions/${id}/settlement?settlement_id=${settlement}`;
 const receipt = { ...notice, notification_type: 'session_charge_reversed', title: 'Session charge reversed', body: 'The original charge was reversed.', data: JSON.stringify({ session_id: id, settlement_id: settlement }), destination_path: path };
 await page.route(`**/api/notifications/${id}/read`, route => json(route, { message: 'read' }));
 await page.route('**/api/users/me/notifications/history?*', route => json(route, [receipt]));
 await page.route(`**/api/sessions/${id}/settlement?*`, route => {
  expect(new URL(route.request().url()).searchParams.get('settlement_id')).toBe(settlement);
  return json(route, { settlement_id: settlement, session: { id, title: 'Original split', starts_at: NOW, ends_at: NOW },
   rates: { base_hours: 2, base_rate_cents: 2000, extra_hours: 0, extra_rate_cents: 0, shuttles_per_hour: 1 },
   bands: { base: { hours: 2, court_cents: 4000, shuttle_units: 0, shuttle_cents: 0, total_cents: 4000, heads: 1 }, extra: null },
   totals: { court_cents: 4000, shuttle_cents: 0, shuttle_units: 0, charged_cents: 4000, surplus_cents: 0 },
   lines: [{ user_id: 'member-1', name: 'Alex', in_base: true, in_extra: false, comped: false, amount_cents: 4000 }],
   settled_at: NOW, reversed_at: NOW });
 });
 await page.goto('/notifications');
 await page.getByRole('link', { name: /Session charge reversed/ }).click();
 await expect(page).toHaveURL(new RegExp(`/sessions/${id}/settlement\\?settlement_id=${settlement}$`));
 await expect(page.getByRole('heading', { name: 'Original split' })).toBeVisible();
 await expect(page.getByText(/This settlement was reversed/)).toBeVisible();
});

test.describe('signed-out historical receipt', () => {
 test.use({ identity: 'visitor' });
 test('keeps settlement identity into OAuth appState', async ({ page }) => {
  const path = `/sessions/${id}/settlement?settlement_id=22222222-2222-4222-8222-222222222222`;
  await page.goto(path); await page.getByRole('button', { name: 'Sign in with Google' }).click();
  const options = await page.evaluate(() => JSON.parse(sessionStorage.getItem('e2e:login') || '{}'));
  expect(options.appState.returnTo).toBe(path);
 });
});
