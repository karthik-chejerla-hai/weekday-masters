import { test, expect, club, json, session } from './support/fixtures';

// Sydney controls must work even when the admin's browser uses another zone.
test.use({ identity: 'admin', timezoneId: 'UTC' });

test('admin changes the default in hours and sees the effect on future sessions', async ({ page }) => {
  await page.route('**/api/admin/join-requests', route => json(route, []));
  await page.route('**/api/club', route => json(route, { ...club, rsvp_deadline_hours: 50 }));
  let hours: number | undefined;
  await page.route('**/api/admin/club', async route => {
    hours = route.request().postDataJSON().rsvp_deadline_hours;
    await json(route, { ...club, rsvp_deadline_hours: hours });
  });
  await page.goto('/admin');
  const input = page.getByLabel('Default RSVP deadline (hours before start)');
  await expect(input).toHaveValue('50');
  await expect(page.getByText(/including custom deadlines/)).toBeVisible();
  await input.fill('48');
  if (process.env.RSVP_SCREENSHOTS_DIR) {
    await input.scrollIntoViewIfNeeded();
    await page.screenshot({ path: `${process.env.RSVP_SCREENSHOTS_DIR}/default-${test.info().project.name}.png`, fullPage: true });
  }
  await page.getByRole('button', { name: 'Save Club Settings' }).click();
  await expect(page.getByText('Club settings saved!')).toBeVisible();
  expect(hours).toBe(48);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
});

test('admin changes one session deadline in Sydney time and can retry a failed save', async ({ page }) => {
  let calls = 0;
  await page.route('**/api/admin/sessions/session-1', async route => {
    expect(route.request().postDataJSON()).toEqual({ rsvp_deadline: '2026-10-14T07:00:00.000Z' });
    calls++;
    await json(route, calls === 1 ? { error: 'Save failed. Try again.' } : { ...session, rsvp_deadline: '2026-10-14T07:00:00.000Z' }, calls === 1 ? 500 : 200);
  });
  await page.goto('/admin/sessions');
  await page.getByRole('button', { name: 'Edit RSVP deadline' }).click();
  const input = page.getByLabel('RSVP deadline (Sydney time)');
  await expect(input).toHaveValue('2026-10-13T23:59');
  await input.fill('2026-10-14T18:00');
  if (process.env.RSVP_SCREENSHOTS_DIR) {
    await page.screenshot({ path: `${process.env.RSVP_SCREENSHOTS_DIR}/session-${test.info().project.name}.png`, fullPage: true });
  }
  await page.getByRole('button', { name: 'Save deadline' }).click();
  await expect(page.getByRole('alert')).toHaveText('Save failed. Try again.');
  await expect(input).toHaveValue('2026-10-14T18:00');
  await page.getByRole('button', { name: 'Save deadline' }).click();
  await expect(input).toHaveCount(0);
  await expect(page.getByText('RSVP closes: 14 Oct 2026 at 18:00 (Sydney)')).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
});
