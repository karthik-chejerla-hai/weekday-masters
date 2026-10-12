import { test, expect, members, json } from './support/fixtures';

test.use({ identity: 'admin' });
const longName = 'Alexandertheverylongdisplaynamethatmustwrap';
const roster = [
  { ...members[0], role: 'admin', nickname: longName, phone_number: '', email: '' },
  members[1],
  { ...members[2], auth0_id: 'invite:casey' },
  { ...members[3], membership_status: 'removed' },
];

test('compact rows expose identity and editing by keyboard and preserve tabs and search', async ({ page }) => {
  await page.route('**/api/admin/users', route => json(route, roster));
  await page.goto('/admin/members');
  const details = page.getByRole('button', { name: `Details for ${longName}`, exact: true });
  await expect(details).toBeVisible();
  await expect(page.getByRole('button', { name: 'Push alerts and devices' })).toHaveCount(0);
  await expect(page.getByRole('button', { name: /Remove member/ })).toHaveCount(0);
  await expect(page.getByText('Alex Test', { exact: true })).not.toBeVisible();
  for (let i = 0; i < 35 && !(await details.evaluate(el => el === document.activeElement)); i++) await page.keyboard.press('Tab');
  await expect(details).toBeFocused();
  await page.keyboard.press('Enter');
  await expect(page.getByText('Alex Test', { exact: true })).toBeVisible();
  await expect(page.getByText('Not provided').filter({ visible: true })).toHaveCount(2);
  await page.getByRole('button', { name: `Edit ${longName}`, exact: true }).click();
  await expect(page.getByLabel('Full name', { exact: true })).toBeFocused();
  await expect(page.getByLabel(/^Email/)).toBeDisabled();
  await page.getByRole('button', { name: 'Cancel', exact: true }).click();
  await expect(page.getByRole('button', { name: `Edit ${longName}`, exact: true })).toBeFocused();
  await details.click();
  await page.getByRole('searchbox').fill('blair@');
  await expect(page.getByRole('button', { name: 'Details for Blair', exact: true })).toBeVisible();
  await expect(details).toHaveCount(0);
  await page.getByRole('searchbox').clear();
  await page.getByRole('tab', { name: 'Not signed in (1)' }).click();
  await page.getByRole('button', { name: 'Details for Casey' }).click();
  await expect(page.getByRole('button', { name: 'Review invitation for Casey' })).toBeVisible();
  await page.getByRole('tab', { name: 'Removed (1)' }).click();
  await page.getByRole('button', { name: 'Details for Drew' }).click();
  await expect(page.getByRole('button', { name: 'Reinstate Drew' })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
});

test('removal refusal stays visible and removed members can be restored', async ({ page }) => {
  let refuse = true;
  const member = members[1];
  await page.route('**/api/admin/users', route => json(route, [member]));
  await page.route(`**/api/admin/users/${member.id}`, route => json(route,
    refuse ? { error: 'Settle the balance before removing this member.' } : { ...member, membership_status: 'removed' }, refuse ? 400 : 200));
  await page.route(`**/api/admin/users/${member.id}/reinstate`, route => json(route, member));
  await page.goto('/admin/members');
  await page.getByRole('button', { name: 'Details for Blair' }).click();
  await page.getByRole('button', { name: 'Remove member Blair' }).click();
  await expect(page.getByRole('alert')).toContainText('Settle the balance');
  refuse = false;
  await page.getByRole('button', { name: 'Remove member Blair' }).click();
  await expect(page.getByRole('status')).toContainText('Their history is kept');
  await page.getByRole('tab', { name: 'Removed (1)' }).click();
  await page.getByRole('button', { name: 'Details for Blair' }).click();
  await page.getByRole('button', { name: 'Reinstate Blair' }).click();
  await expect(page.getByRole('status')).toContainText('back in the club');
  await expect(page.getByRole('tab', { name: 'Members (1)' })).toBeVisible();
});
