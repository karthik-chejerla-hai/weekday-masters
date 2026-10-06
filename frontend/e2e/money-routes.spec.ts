import { test, expect } from './support/fixtures';

const adminRoutes = ['/admin', '/admin/members', '/admin/sessions', '/admin/sessions/session-1/settle'];
const memberRoutes = ['/dashboard', '/sessions', '/sessions/session-1', '/sessions/session-1/settlement', '/money', '/games', '/sessions/session-1/games', '/assistant'];

test.describe('Visitor route guards', () => {
  test.use({ identity: 'visitor' });
  for (const path of [...memberRoutes, ...adminRoutes, '/profile']) {
    test(`redirects ${path}`, async ({ page }) => {
      await page.goto(path);
      await expect(page).toHaveURL('/');
      await expect(page.getByRole('button', { name: 'Sign in with Google' })).toBeVisible();
      await expect(page.locator('body')).not.toContainText(/\$\d/);
    });
  }
});
for (const identity of ['pending', 'removed'] as const) {
  test.describe(`${identity} membership`, () => {
    test.use({ identity });
    test('blocks club access and allows sign out', async ({ page }) => {
      await page.goto('/money');
      await expect(page).toHaveURL('/pending');
      await expect(page.getByRole('heading', { name: identity === 'pending' ? 'Membership Pending' : 'No Longer a Member' })).toBeVisible();
      await page.getByRole('button', { name: 'Sign Out' }).click();
      await expect(page).toHaveURL('/');
      await expect(page.getByRole('button', { name: 'Sign in with Google' })).toBeVisible();
    });
  });
}
for (const path of adminRoutes) {
  test(`player cannot open ${path}`, async ({ page }) => {
    await page.goto(path);
    await expect(page).toHaveURL('/dashboard');
    await expect(page.getByRole('heading', { name: 'Ready for your next game?' })).toBeVisible();
    await expect(page.getByRole('navigation', { name: 'Admin navigation' })).toHaveCount(0);
  });
}
