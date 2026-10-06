import { test, expect, json } from './support/fixtures';

test('profile update survives reload and recovers from a failed save', async ({ page, app }) => {
  const user = { ...app.user };
  let fail = true;
  const writes: unknown[] = [];
  await page.route('**/api/auth/callback', route => json(route, { user }));
  await page.route('**/api/users/me', async route => {
    if (route.request().method() === 'PUT') {
      writes.push(route.request().postDataJSON());
      if (fail) return json(route, { error: 'Profile save unavailable. Try again.' }, 503);
      Object.assign(user, route.request().postDataJSON());
    }
    await json(route, user);
  });
  await page.goto('/profile');
  await page.getByLabel('Nickname', { exact: true }).fill('Ace');
  await page.getByPlaceholder('Enter your phone number').fill('0400000000');
  await page.getByRole('button', { name: 'Save Changes' }).click();
  await expect(page.getByText('Profile save unavailable. Try again.')).toBeVisible();
  await expect(page.getByLabel('Nickname', { exact: true })).toHaveValue('Ace');
  fail = false;
  await page.getByRole('button', { name: 'Save Changes' }).click();
  await expect(page.getByText('Profile updated successfully!')).toBeVisible();
  expect(writes).toEqual(Array(2).fill({ nickname: 'Ace', phone_number: '0400000000' }));
  await page.reload();
  await expect(page.getByLabel('Nickname', { exact: true })).toHaveValue('Ace');
  await expect(page.getByRole('heading', { name: 'Ace', exact: true })).toBeVisible();
});

test.describe('Administrator presentation', () => {
  test.use({ identity: 'admin' });
  test('member preview hides and restores admin actions', async ({ page }) => {
    await page.goto('/profile');
    const tools = page.getByRole('region', { name: 'Club admin' });
    await tools.getByRole('button', { name: /Preview as member/ }).click();
    await expect(tools).toHaveCount(0);
    await expect(page.getByRole('navigation', { name: 'Admin navigation' })).toHaveCount(0);
    await page.getByRole('button', { name: /Exit preview/ }).click();
    await expect(tools.getByRole('link', { name: /Manage members/ })).toBeVisible();
  });
});
