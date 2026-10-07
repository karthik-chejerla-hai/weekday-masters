import { test, expect } from './support/fixtures';

test('an expired saved session shows normal sign-in and allows a new login', async ({ page }) => {
  await page.addInitScript(() => sessionStorage.setItem('e2e:token-error', 'missing_refresh_token'));
  let profileRequests = 0;
  page.on('request', request => {
    if (request.url().endsWith('/api/auth/callback')) profileRequests += 1;
  });
  await page.goto('/');
  await expect(page.getByRole('button', { name: 'Sign in with Google' })).toBeVisible();
  await expect(page.getByRole('alert')).toHaveCount(0);
  expect(profileRequests).toBe(0);
  await page.getByRole('button', { name: 'Sign in with Google' }).click();
  await expect.poll(() => page.evaluate(() => sessionStorage.getItem('e2e:login')))
    .toBe(JSON.stringify({ authorizationParams: { connection: 'google-oauth2' } }));
});
