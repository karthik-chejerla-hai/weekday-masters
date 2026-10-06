import { test, expect, detail, json, member, NOW } from './support/fixtures';
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
    await page.getByRole('link', { name: 'Details', exact: true }).click();
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
