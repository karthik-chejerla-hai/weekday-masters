import { test, expect, detail, json, member, NOW, session } from './support/fixtures';
import type { RSVP, SelectableRSVPStatus } from '../src/types';

for (const full of [false, true]) {
  test(full ? 'full session joins waitlist and retains queue position' : 'member joins and leaves a session, including after reload', async ({ page }) => {
    const data = detail();
    if (full) Object.assign(data.rsvp_summary, { total_in: 6, spots_left: 0 });
    await page.route('**/api/sessions/session-1', route => json(route, data));
    const writes: SelectableRSVPStatus[] = [];
    await page.route('**/api/sessions/session-1/rsvp', async route => {
      expect(route.request().method()).toBe('POST');
      const { status } = route.request().postDataJSON();
      writes.push(status);
      const rsvp: RSVP = { id: 'rsvp-1', session_id: 'session-1', user_id: member.id,
        status: full && status === 'in' ? 'waitlisted' : status, user: member,
        waitlist_position: full && status === 'in' ? 2 : undefined,
        rsvp_timestamp: NOW, created_at: NOW, updated_at: NOW, is_late_rsvp: false, added_by_admin: false };
      data.session.rsvps = [rsvp];
      data.rsvp_summary.total_in = full ? 6 : status === 'in' ? 1 : 0;
      data.rsvp_summary.spots_left = full ? 0 : 6 - data.rsvp_summary.total_in;
      data.rsvp_summary.total_waitlisted = full && status === 'in' ? 1 : 0;
      await json(route, rsvp);
    });
    await page.goto('/dashboard');
    await page.getByRole('link', { name: 'Friday badminton, Friday, 16 October 2026' }).click();
    await expect(page.getByRole('heading', { name: 'Friday badminton' })).toBeVisible();
    await page.getByRole('button', { name: full ? 'Join Waitlist' : "I'm In", exact: true }).click();
    await expect(page.getByText(full ? "You're on the waitlist at position #2" : '1 / 6 players', { exact: true })).toBeVisible();
    await page.reload();
    await expect(page.getByText(full ? "You're on the waitlist at position #2" : '1 / 6 players', { exact: true })).toBeVisible();
    await page.getByRole('button', { name: "Can't Make It", exact: true }).click();
    await expect.poll(() => writes).toEqual(['in', 'out']);
    await expect(page.getByText(full ? "You're on the waitlist at position #2" : '1 / 6 players', { exact: true })).toHaveCount(0);
  });
}

test('past deadline prevents changes for a confirmed member', async ({ page }) => {
  const data = detail({ rsvp_deadline: '2026-10-06T00:00:00Z', rsvps: [{
    id: 'rsvp-1', session_id: 'session-1', user_id: member.id, status: 'in', user: member,
    rsvp_timestamp: NOW, created_at: NOW, updated_at: NOW, is_late_rsvp: false, added_by_admin: false,
  }] });
  await page.route('**/api/sessions/session-1', route => json(route, data));
  await page.goto('/sessions/session-1');
  await expect(page.getByText('You confirmed attendance and cannot change your RSVP.')).toBeVisible();
  await expect(page.getByRole('button', { name: "Can't Make It" })).toHaveCount(0);
});

test('cancelled session gives its reason and has no RSVP controls', async ({ page }) => {
  await page.route('**/api/sessions/session-1', route => json(route,
    detail({ status: 'cancelled', cancellation_reason: 'The hall is closed for repairs.' })));
  await page.goto('/sessions/session-1');
  await expect(page.getByRole('status')).toHaveText('The hall is closed for repairs.');
  await expect(page.getByRole('heading', { name: 'Your RSVP' })).toHaveCount(0);
});

test('missing session offers a working return to the schedule', async ({ page }) => {
  await page.route('**/api/sessions/missing', route => json(route, { error: 'Not found' }, 404));
  await page.goto('/sessions/missing');
  await expect(page.getByText('Session not found')).toBeVisible();
  await page.getByRole('button', { name: 'Back to Sessions' }).click();
  await expect(page).toHaveURL('/sessions');
  await expect(page.getByRole('heading', { name: 'Sessions', exact: true })).toBeVisible();
});

for (const path of ['/dashboard', '/sessions']) {
  test(`session card supports keyboard navigation from ${path}`, async ({ page }) => {
    await page.goto(path);
    const link = page.getByRole('link', { name: 'Friday badminton, Friday, 16 October 2026', exact: true });
    await expect(link).toBeVisible();
    await expect(page.getByText('Open session for details')).toHaveCount(0);
    if (path === '/dashboard') {
      await expect(page.getByRole('link', { name: 'Details', exact: true })).toHaveCount(0);
    }
    // Tab through the actual page to prove the card participates in tab order.
    for (let i = 0; i < 30 && !(await link.evaluate(el => el === document.activeElement)); i++) {
      await page.keyboard.press('Tab');
    }
    await expect(link).toBeFocused();
    expect(await link.evaluate(el => getComputedStyle(el).boxShadow)).not.toBe('none');
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL('/sessions/session-1');
    await expect(page.getByRole('heading', { name: 'Your RSVP' })).toBeVisible();
  });
}

test('compact cards show my response and a long confirmed roster without overflow', async ({ page }) => {
  const rsvps: RSVP[] = Array.from({ length: 7 }, (_, i) => ({
    id: `r${i}`, session_id: session.id, user_id: i === 6 ? member.id : `player-${i}`,
    status: i === 6 ? 'waitlisted' : 'in', waitlist_position: i === 6 ? 1 : undefined,
    user: { ...member, id: `player-${i}`, nickname: `LongPlayerName${i}WithNoSpacesForLayoutChecks` },
    rsvp_timestamp: NOW, created_at: NOW, updated_at: NOW, is_late_rsvp: false, added_by_admin: false,
  }));
  await page.route(/\/api\/sessions(?:\?.*)?$/, route => json(route, [{ ...session, rsvps }]));
  await page.goto('/sessions');
  await expect(page.getByText('Your RSVP: Waitlisted (#1)')).toBeVisible();
  await expect(page.getByText('Full · 1 waiting')).toBeVisible();
  await expect(page.getByRole('list', { name: 'Player responses' }).getByRole('listitem')).toHaveCount(7);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
});

test('cards retain main RSVP pills with avatars and coloured borders', async ({ page }) => {
  const rsvps: RSVP[] = (['in', 'maybe', 'out', 'waitlisted'] as const).map((status, i) => ({
    id: `r${i}`, session_id: session.id, user_id: `player-${i}`, status,
    user: { ...member, id: `player-${i}`, nickname: ['Alex', 'Blair', 'Casey', 'Drew'][i] },
    rsvp_timestamp: NOW, created_at: NOW, updated_at: NOW, is_late_rsvp: false, added_by_admin: false,
  }));
  await page.route(/\/api\/sessions(?:\?.*)?$/, route => json(route, [{ ...session, rsvps }]));
  await page.goto('/dashboard');
  const responses = page.getByRole('list', { name: 'Player responses' });
  for (const [name, label, colour] of [
    ['Alex', 'Confirmed', 'rgb(134, 239, 172)'], ['Blair', 'Tentative', 'rgb(252, 211, 77)'],
    ['Casey', 'Not attending', 'rgb(252, 165, 165)'], ['Drew', 'Waitlisted', 'rgb(252, 211, 77)'],
  ]) {
    const chip = responses.getByRole('listitem', { name: `${name}, ${label}`, exact: true });
    await expect(chip.getByText(name, { exact: true })).toBeVisible();
    await expect(chip.locator('[aria-hidden="true"]').first()).toBeVisible();
    await expect(chip).toHaveCSS('border-color', colour);
  }
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
});

test('upcoming attendance is labelled and results remain a secondary keyboard link', async ({ page }) => {
  await page.goto('/sessions/session-1');
  await expect(page.getByText('Current response: Not responded')).toBeVisible();
  for (const name of ["I'm In", 'Maybe', "Can't Make It"]) {
    const button = page.getByRole('button', { name, exact: true });
    await expect(button.getByText(name, { exact: true })).toBeVisible();
    await expect(button).toHaveAttribute('aria-pressed', 'false');
  }
  const results = page.getByRole('link', { name: 'Game scores and results' });
  await expect(results).not.toHaveClass(/btn-primary/);
  for (let i = 0; i < 30 && !(await results.evaluate(el => el === document.activeElement)); i++) await page.keyboard.press('Tab');
  await expect(results).toBeFocused();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL('/sessions/session-1/games');
});
