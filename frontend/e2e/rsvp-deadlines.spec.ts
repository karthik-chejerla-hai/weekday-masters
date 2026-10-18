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

for (const [timeFormat, expected] of [['12h', '6:00 PM'], ['24h', '18:00']] as const) {
  test(`session cards show the RSVP deadline time in ${timeFormat} format`, async ({ page }) => {
    await page.route('**/api/club', route => json(route, { ...club, time_format: timeFormat }));
    await page.route(/\/api\/sessions(?:\?.*)?$/, route => json(route, [{ ...session,
      title: 'Tuesday badminton', session_date: '2026-10-13', start_time: '20:00', end_time: '22:00',
      starts_at: '2026-10-13T09:00:00Z', ends_at: '2026-10-13T11:00:00Z',
      rsvp_deadline: '2026-10-11T07:00:00Z',
    }]));
    await page.goto('/sessions');
    await expect(page.getByText(`RSVP by Sun, 11 Oct at ${expected}`, { exact: true })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    if (process.env.RSVP_SCREENSHOTS_DIR && timeFormat === '12h') {
      await page.screenshot({ path: `${process.env.RSVP_SCREENSHOTS_DIR}/card-time-${test.info().project.name}.png`, fullPage: true });
    }
  });
}

test('a venue-only save does not restore a stale deadline default', async ({ page }) => {
  await page.route('**/api/admin/join-requests', route => json(route, []));
  let currentHours = 50;
  await page.route('**/api/club', route => json(route, { ...club, rsvp_deadline_hours: currentHours }));
  let submitted: Record<string, unknown> | undefined;
  await page.route('**/api/admin/club', async route => {
    submitted = route.request().postDataJSON();
    await json(route, { ...club, ...submitted, rsvp_deadline_hours: currentHours });
  });
  await page.goto('/admin');
  const input = page.getByLabel('Default RSVP deadline (hours before start)');
  await expect(input).toHaveValue('50');
  // Another admin changes the saved default while this page remains open.
  currentHours = 48;
  await page.getByPlaceholder('e.g., Sydney Olympic Park Sports Centre').fill('New venue');
  await page.getByRole('button', { name: 'Save Club Settings' }).click();
  await expect(page.getByText('Club settings saved!')).toBeVisible();
  expect(submitted).toMatchObject({ venue_name: 'New venue' });
  expect(submitted).not.toHaveProperty('rsvp_deadline_hours');
  await expect(input).toHaveValue('48');
});

test('deadline edits reject the Sydney daylight saving gap', async ({ page }) => {
  await page.route(/\/api\/sessions(?:\?.*)?$/, route => json(route, [{ ...session,
    session_date: '2027-10-05', starts_at: '2027-10-05T09:00:00Z',
  }]));
  let saves = 0;
  await page.route('**/api/admin/sessions/session-1', async route => { saves++; await json(route, session); });
  await page.goto('/admin/sessions');
  await page.getByRole('button', { name: 'Edit RSVP deadline' }).click();
  const input = page.getByLabel('RSVP deadline (Sydney time)');
  await input.fill('2027-10-03T02:30');
  await page.getByRole('button', { name: 'Save deadline' }).click();
  await expect(page.getByRole('alert')).toHaveText('Choose a valid Sydney date and time. This clock time does not exist.');
  await expect(input).toHaveValue('2027-10-03T02:30');
  expect(saves).toBe(0);
  if (process.env.RSVP_SCREENSHOTS_DIR) {
    await page.screenshot({ path: `${process.env.RSVP_SCREENSHOTS_DIR}/sydney-gap-${test.info().project.name}.png`, fullPage: true });
  }
});
