import { test, expect } from '@playwright/test';

/**
 * The money screens sit behind Auth0, and this suite has no way to mint a
 * session — there is no test tenant and no auth mocking in the project. So what
 * is verifiable from a browser is the guard: an unauthenticated visitor must
 * never see a balance, and must be sent to the landing page rather than shown an
 * empty shell of the real screen.
 *
 * The authenticated flow — top-up, settle, history, balance — is covered end to
 * end in backend/internal/handlers/{ledger,settlement}_test.go, which mount the
 * real handlers over a real database and assert the whole sequence at the HTTP
 * level. What is missing here is only the browser rendering on top of it.
 */
test.describe('Money routes are closed to visitors', () => {
  for (const path of ['/money', '/sessions', '/sessions/some-id/settlement']) {
    test(`redirects ${path} to the landing page`, async ({ page }) => {
      await page.goto(path);

      await expect(page).toHaveURL('/');
      await expect(page.getByRole('button', { name: /sign in/i })).toBeVisible();
    });
  }

  test('never renders a balance to an unauthenticated visitor', async ({ page }) => {
    await page.goto('/money');
    await expect(page).toHaveURL('/');

    // No dollar amount should appear anywhere on the landing page.
    await expect(page.locator('body')).not.toContainText(/\$\d/);
  });

  test('the admin settlement form is closed too', async ({ page }) => {
    await page.goto('/admin/sessions/some-id/settle');

    await expect(page).toHaveURL('/');
    await expect(page.locator('body')).not.toContainText(/Settle this session/i);
  });
});
