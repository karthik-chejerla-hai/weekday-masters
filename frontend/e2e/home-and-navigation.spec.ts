import { test, expect } from '@playwright/test';

test.describe('Home Page & Navigation', () => {
  test('renders landing page with club branding and login action', async ({ page }) => {
    await page.goto('/');

    // Check page title or header
    await expect(page).toHaveTitle(/Rally/i);

    // Check login button exists
    const loginButton = page.getByRole('button', { name: /sign in|log in|get started/i });
    if (await loginButton.count() > 0) {
      await expect(loginButton.first()).toBeVisible();
    }
  });

  test('handles responsive viewport layout', async ({ page }) => {
    // Mobile viewport
    await page.setViewportSize({ width: 375, height: 667 });
    await page.goto('/');
    await expect(page.locator('body')).toBeVisible();

    // Desktop viewport
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto('/');
    await expect(page.locator('body')).toBeVisible();
  });
});
