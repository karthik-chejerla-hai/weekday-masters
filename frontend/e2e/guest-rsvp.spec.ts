import { test, expect, detail, json, members, NOW } from './support/fixtures';
import type { GuestRSVP } from '../src/types';

test.use({ identity: 'admin' });
test('admin adds a guest, sees them after reload, and changes them to OUT', async ({ page }, testInfo) => {
  const data = detail();
  await page.route('**/api/sessions/session-1', route => json(route, data));
  await page.route('**/api/admin/sessions/session-1/guests', async route => {
    expect(route.request().method()).toBe('POST');
    expect(route.request().postDataJSON()).toEqual({ guest_name: 'Sam Guest', charged_to_user_id: members[1].id, status: 'in' });
    const guest: GuestRSVP = { ...route.request().postDataJSON(), id: 'guest-1', session_id: 'session-1', charged_to_user: members[1], rsvp_timestamp: NOW, created_at: NOW, updated_at: NOW };
    data.session.guest_rsvps = [guest];
    data.rsvp_summary.total_in = 1;
    data.rsvp_summary.spots_left = 5;
    await json(route, guest);
  });
  await page.route('**/api/admin/sessions/session-1/guests/guest-1', async route => {
    expect(route.request().method()).toBe('PUT');
    expect(route.request().postDataJSON().status).toBe('out');
    Object.assign(data.session.guest_rsvps![0], route.request().postDataJSON());
    data.rsvp_summary.total_in = 0;
    data.rsvp_summary.total_out = 1;
    data.rsvp_summary.spots_left = 6;
    await json(route, data.session.guest_rsvps![0]);
  });
  await page.goto('/sessions/session-1');
  await page.getByLabel('Player type').selectOption('guest');
  await page.getByLabel('Guest name', { exact: true }).fill('Sam Guest');
  await page.getByLabel('Charged to member').selectOption(members[1].id);
  await expect(page.getByLabel('Response', { exact: true })).toHaveValue('in');
  await page.getByRole('button', { name: 'Save guest RSVP' }).click();
  await expect(page.getByText("Sam Guest's RSVP saved as In.")).toBeVisible();
  await expect(page.getByText('Charged to Blair')).toBeVisible();
  await expect(page.getByText('1 / 6 players', { exact: true })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  if (process.env.GUEST_RSVP_SCREENSHOTS) {
    await page.evaluate(() => window.scrollTo(0, 0));
    await page.screenshot({ path: `${process.env.GUEST_RSVP_SCREENSHOTS}/${testInfo.project.name}.png`, fullPage: true });
  }
  await page.reload();
  await expect(page.getByText('Charged to Blair')).toBeVisible();
  await page.getByLabel('Player type').selectOption('guest');
  await page.getByLabel('Guest RSVP', { exact: true }).selectOption('guest-1');
  await page.getByLabel('Response', { exact: true }).selectOption('out');
  await page.getByRole('button', { name: 'Save guest RSVP' }).click();
  await expect(page.getByText("Sam Guest's RSVP saved as Out.")).toBeVisible();
  await expect(page.getByText('0 / 6 players', { exact: true })).toBeVisible();
  await expect(page.getByText('No confirmed players yet')).toBeVisible();
});
