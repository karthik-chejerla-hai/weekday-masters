import { test, expect, club } from './support/fixtures';

test.describe('Public entry', () => {
  test.use({ identity: 'visitor' });
  test('renders club details and starts Google sign-in', async ({ page }) => {
    await page.goto('/');
    await expect(page).toHaveTitle(/Rally/);
    await expect(page.getByRole('heading', { name: club.name })).toBeVisible();
    await expect(page.getByText(club.venue_name)).toBeVisible();
    await page.getByRole('button', { name: 'Sign in with Google' }).click();
    await expect.poll(() => page.evaluate(() => JSON.parse(sessionStorage.getItem('e2e:login')!)))
      .toEqual({ authorizationParams: { connection: 'google-oauth2' } });
  });
  test('invitation requests account selection and retains return route', async ({ page }) => {
    await page.goto('/welcome');
    await expect(page.getByRole('heading', { name: 'Welcome to Rally' })).toBeVisible();
    await page.getByRole('button', { name: 'Sign in with Google' }).click();
    await expect.poll(() => page.evaluate(() => JSON.parse(sessionStorage.getItem('e2e:login')!)))
      .toEqual({ appState: { returnTo: '/welcome' }, authorizationParams: {
        connection: 'google-oauth2', prompt: 'select_account',
      } });
  });
});

test('member can navigate at the configured screen size', async ({ page, isMobile }) => {
  await page.goto('/dashboard');
  await expect(page.getByRole('heading', { name: 'Ready for your next game?' })).toBeVisible();
  const navigation = page.getByRole('navigation', { name: isMobile ? 'Mobile navigation' : 'Main navigation', exact: true });
  await expect(navigation).toBeVisible();
  await navigation.getByRole('link', { name: 'Sessions', exact: true }).click();
  await expect(page).toHaveURL('/sessions');
  await expect(page.getByRole('heading', { name: 'Sessions', exact: true })).toBeVisible();
  await navigation.getByRole('link', { name: 'Money', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Money', exact: true })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
});
