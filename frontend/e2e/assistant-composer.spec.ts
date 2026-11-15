import { test, expect, json } from './support/fixtures';

for (const identity of ['member', 'admin'] as const) {
  test.describe(`${identity} composer`, () => {
    test.use({ identity });
    test('examples stay editable until explicit Send and provider failures keep the text', async ({ page }) => {
      await page.route('**/api/assistant/status', route => json(route, { enabled: true }));
      let calls = 0;
      await page.route('**/api/assistant/messages', async route => {
        calls++;
        await json(route, { message: 'Assistant is temporarily unavailable. Try again.' }, 503);
      });
      await page.goto('/assistant');
      await expect(page.getByRole('heading', { level: 1, name: 'Ask Rally' })).toBeVisible();
      await page.getByRole('button', { name: 'Record a score', exact: true }).click();
      const input = page.getByLabel('Your message');
      await expect(input).toBeFocused();
      await expect(input).toHaveValue('Help me record a doubles game score.');
      expect(calls).toBe(0);
      await input.fill('Help me record the last game.');
      await page.getByRole('button', { name: 'Send message', exact: true }).click();
      await expect(page.getByRole('alert')).toContainText('temporarily unavailable');
      expect(calls).toBe(1);
      await expect(input).toHaveValue('Help me record the last game.');
      await expect(page.getByRole('link', { name: 'Choose a session to expense' })).toHaveCount(identity === 'admin' ? 1 : 0);
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    });
  });
}
