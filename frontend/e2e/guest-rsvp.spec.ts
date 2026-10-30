import { test, expect, detail, json, members, NOW } from './support/fixtures';
import type { GuestRSVP } from '../src/types';

test.use({ identity: 'admin' });
test('admin edits a guest payer and status from the player list without duplicates', async ({ page }, testInfo) => {
  const data = detail();
  let creates = 0;
  let edits = 0;
  let removals = 0;
  await page.route('**/api/sessions/session-1', route => json(route, data));
  await page.route('**/api/admin/sessions/session-1/guests', async route => {
    expect(route.request().method()).toBe('POST');
    creates++;
    expect(route.request().postDataJSON()).toEqual({ guest_name: 'Sam Guest', charged_to_user_id: members[1].id, status: 'in' });
    const guest: GuestRSVP = { ...route.request().postDataJSON(), id: 'guest-1', session_id: 'session-1', charged_to_user: members[1], rsvp_timestamp: NOW, created_at: NOW, updated_at: NOW };
    data.session.guest_rsvps = [guest];
    data.rsvp_summary.total_in = 1;
    data.rsvp_summary.spots_left = 5;
    await json(route, guest);
  });
  await page.route('**/api/admin/sessions/session-1/guests/guest-1', async route => {
    if (route.request().method() === 'DELETE') {
      removals++;
      data.session.guest_rsvps = [];
      data.rsvp_summary.total_in = 0;
      data.rsvp_summary.total_out = 0;
      data.rsvp_summary.spots_left = 6;
      await route.fulfill({ status: 204 });
      return;
    }
    expect(route.request().method()).toBe('PUT');
    const change = route.request().postDataJSON();
    expect(change.guest_name).toBe('Sam Guest');
    expect(change.charged_to_user_id).toBe(members[2].id);
    Object.assign(data.session.guest_rsvps![0], change, { charged_to_user: members[2] });
    data.rsvp_summary.total_in = change.status === 'in' ? 1 : 0;
    data.rsvp_summary.total_out = change.status === 'out' ? 1 : 0;
    data.rsvp_summary.spots_left = 6 - data.rsvp_summary.total_in;
    edits++;
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
  await page.reload();
  await page.getByRole('button', { name: "Edit Sam Guest's RSVP" }).click();
  const editor = page.getByRole('dialog', { name: 'Edit guest RSVP' });
  await expect(editor).toBeVisible();
  await expect(editor.getByLabel('Guest name', { exact: true })).toHaveValue('Sam Guest');
  await expect(editor.getByLabel('Charged to member')).toHaveValue(members[1].id);
  await expect(editor.getByLabel('Response', { exact: true })).toHaveValue('in');
  await editor.getByLabel('Charged to member').selectOption(members[2].id);
  if (process.env.GUEST_RSVP_SCREENSHOTS) {
    await page.screenshot({ path: `${process.env.GUEST_RSVP_SCREENSHOTS}/${testInfo.project.name}.png` });
  }
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await editor.getByRole('button', { name: 'Save guest RSVP' }).click();
  await expect(editor).not.toBeVisible();
  await expect(page.getByText('Charged to Casey', { exact: true })).toHaveCount(1);
  await expect(page.getByText('Charged to Blair', { exact: true })).toHaveCount(0);
  await expect(page.getByText('1 / 6 players', { exact: true })).toBeVisible();
  expect(creates).toBe(1);
  expect(edits).toBe(1);
  await page.reload();
  await expect(page.getByText('Charged to Casey', { exact: true })).toBeVisible();
  // OUT and IN each edit the existing row without re-entering name or payer.
  for (const status of ['out', 'in']) {
    await page.getByRole('button', { name: "Edit Sam Guest's RSVP" }).click();
    await expect(editor.getByLabel('Charged to member')).toHaveValue(members[2].id);
    await editor.getByLabel('Response', { exact: true }).selectOption(status);
    await editor.getByRole('button', { name: 'Save guest RSVP' }).click();
    await expect(editor).not.toBeVisible();
    await expect(page.getByText(`${status === 'in' ? 1 : 0} / 6 players`, { exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: "Edit Sam Guest's RSVP" })).toHaveCount(1);
  }
  await page.getByRole('button', { name: "Edit Sam Guest's RSVP" }).click();
  await editor.getByLabel('Response', { exact: true }).selectOption('out');
  await editor.getByRole('button', { name: 'Cancel', exact: true }).click();
  await expect(editor).not.toBeVisible();
  await expect(page.getByText('1 / 6 players', { exact: true })).toBeVisible();
  expect(edits).toBe(3);
  // Re-entering the name cannot create a duplicate and leads to the saved RSVP.
  await page.getByLabel('Player type').selectOption('guest');
  await page.getByLabel('Guest name', { exact: true }).fill('  sAM   gUEST  ');
  await expect(page.getByText('Sam Guest already has an RSVP for this session.')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Save guest RSVP' })).toBeDisabled();
  await page.getByRole('button', { name: 'Edit existing guest' }).click();
  await expect(page.getByLabel('Guest name', { exact: true })).toHaveValue('Sam Guest');
  await expect(page.getByLabel('Charged to member')).toHaveValue(members[2].id);
  expect(creates).toBe(1);

  await page.getByRole('button', { name: "Edit Sam Guest's RSVP" }).click();
  if (process.env.GUEST_RSVP_SCREENSHOTS) {
    await page.screenshot({ path: `${process.env.GUEST_RSVP_SCREENSHOTS}/${testInfo.project.name}-remove.png` });
  }
  await editor.getByRole('button', { name: 'Remove guest', exact: true }).click();
  await expect(editor).not.toBeVisible();
  await expect(page.getByRole('button', { name: "Edit Sam Guest's RSVP" })).toHaveCount(0);
  await expect(page.getByText('Charged to Casey', { exact: true })).toHaveCount(0);
  await expect(page.getByText('0 / 6 players', { exact: true })).toBeVisible();
  await expect(page.getByLabel('Guest name', { exact: true })).toHaveValue('');
  expect(removals).toBe(1);
  await page.reload();
  await expect(page.getByText('No confirmed players yet')).toBeVisible();
  await page.getByLabel('Player type').selectOption('guest');
  await page.getByLabel('Guest name', { exact: true }).fill('Sam Guest');
  await page.getByLabel('Charged to member').selectOption(members[1].id);
  await page.getByRole('button', { name: 'Save guest RSVP' }).click();
  await expect(page.getByText('1 / 6 players', { exact: true })).toBeVisible();
  expect(creates).toBe(2);
});
