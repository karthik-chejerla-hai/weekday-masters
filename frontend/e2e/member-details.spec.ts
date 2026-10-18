import { test, expect, members, preferences, json } from './support/fixtures';

test.use({ identity: 'admin' });
const longName = 'Alexandertheverylongdisplaynamethatmustwrap';
const roster = [
  { ...members[0], role: 'admin', nickname: longName, phone_number: '', email: '' },
  members[1],
  { ...members[2], auth0_id: 'invite:casey' },
  { ...members[3], membership_status: 'removed' },
];

test('compact rows expose identity and editing by keyboard and preserve tabs and search', async ({ page }) => {
  const alex = { ...roster[0], push_device_count: 1 };
  const rosterWithPushCounts = [alex, { ...roster[1], push_device_count: 0 }, roster[2], roster[3]];
  await page.route('**/api/admin/users', route => json(route, rosterWithPushCounts));
  await page.route(`**/api/admin/users/${alex.id}`, async route => {
    if (route.request().method() !== 'PUT') return route.continue();
    return json(route, { ...alex, ...route.request().postDataJSON(), push_device_count: 1 });
  });
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
  await expect(page.getByRole('img', { name: 'Push set up · 1 device', exact: true })).toBeVisible();
  await page.getByRole('button', { name: `Edit ${longName}`, exact: true }).click();
  await page.getByLabel('Nickname').fill('Alex saved');
  await page.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Details for Alex saved', exact: true })).toBeVisible();
  await expect(page.getByRole('img', { name: 'Push set up · 1 device', exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Details for Alex saved', exact: true }).click();
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


test('closed rows show saved push states and detail refresh updates the icon', async ({ page }) => {
  const rosterWithDeviceCounts = members.map((member) => ({
    ...member,
    push_device_count: member.id === members[0].id ? 1 : member.id === members[1].id ? 0 : undefined,
  }));
  await page.route('**/api/admin/users', route => json(route, rosterWithDeviceCounts));
  let registeredDevices = 1;
  await page.route('**/api/admin/users/*/push-notifications', route => {
    const id = route.request().url().split('/').slice(-2)[0];
    if (id === members[3].id) return json(route, { error: 'Unavailable' }, 503);
    return json(route, {
      preferences: id === members[2].id ? null : { ...preferences, push_enabled: false },
      devices: id === members[0].id && registeredDevices > 0
        ? [{ id: 'device-1', device_name: 'Alex browser', created_at: '2026-10-07T01:00:00Z', last_registered_at: '2026-10-07T01:00:00Z' }]
        : [],
    });
  });
  await page.goto('/admin/members');
  await expect(page.getByRole('img', { name: 'Push set up · 1 device', exact: true })).toHaveCount(1);
  await expect(page.getByRole('img', { name: 'No push devices', exact: true })).toHaveCount(1);
  await expect(page.getByRole('img', { name: 'Push status unavailable', exact: true })).toHaveCount(2);
  await expect(page.getByRole('button', { name: 'Push alerts and devices' })).toHaveCount(0);
  await page.getByRole('button', { name: 'Details for Alex', exact: true }).click();
  await page.getByRole('button', { name: 'Push alerts and devices', exact: true }).click();
  await expect(page.getByText('Account push is off. These choices apply when the member turns it on.')).toBeVisible();
  await expect(page.getByText('1 registered device', { exact: true })).toBeVisible();
  await expect(page.getByText('Alex browser', { exact: true })).toBeVisible();
  registeredDevices = 0;
  await page.getByRole('button', { name: 'Refresh', exact: true }).click();
  await expect(page.getByText('No registered devices. Push alerts cannot reach this member.')).toBeVisible();
  await expect(page.getByText('0 registered devices', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Details for Alex', exact: true }).click();
  await expect(page.getByRole('img', { name: 'No push devices', exact: true })).toHaveCount(2);
  await page.getByRole('button', { name: 'Details for Drew', exact: true }).click();
  await page.getByRole('button', { name: 'Push alerts and devices', exact: true }).click();
  await expect(page.getByRole('alert')).toContainText('Could not load push settings');
  await expect(page.getByRole('button', { name: 'Details for Drew', exact: true })
    .getByRole('img', { name: 'Push status unavailable', exact: true })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
});
