import { test, expect, json, position } from './support/fixtures';
import type { AssetPurchase } from '../src/types';

const purchase: AssetPurchase = {
  id: 'a8da4966-e957-4c46-8293-e37d777219b8', kind: 'shuttle_purchase',
  amount_cents: 5000, units: 12, reversed: false, actor_name: 'Alex',
  description: 'Shuttle purchase', occurred_at: '2026-10-01T06:00:00Z',
  created_at: '2026-10-02T06:00:00Z', created_by: 'member-1',
};

test('members cannot see asset purchase correction controls', async ({ page }) => {
  await page.goto('/money?tab=club');
  await expect(page.getByText('Purchase history and corrections')).toHaveCount(0);
});

test.describe('Admin purchase correction', () => {
  test.use({ identity: 'admin' });
  test('reviews and saves a linked correction, then refreshes history and assets', async ({ page }) => {
    let saved = false;
    let posts = 0;
    await page.route('**/api/admin/asset-purchases?*', route => json(route, {
      items: saved ? [{ ...purchase, id: 'replacement', amount_cents: 6000, units: 24,
        correction_reason: 'Receipt verified', corrects_transaction_id: purchase.id },
        { ...purchase, reversed: true }] : [purchase], total: saved ? 2 : 1,
    }));
    await page.route('**/api/position', route => json(route, saved ? { ...position,
      assets: { ...position.assets, bank_cents: 4000, shuttle_stock_cents: 3000, shuttle_stock_units: 24 } } : position));
    await page.route(`**/api/admin/asset-purchases/${purchase.id}/correct`, async route => {
      expect(route.request().postDataJSON()).toEqual({ amount_cents: 6000, units: 24, reason: 'Receipt verified' });
      posts++; saved = true;
      await json(route, { ...purchase, id: 'replacement', corrects_transaction_id: purchase.id });
    });
    await page.goto('/money?tab=club');
    await page.getByText('Purchase history and corrections').click();
    await page.getByRole('button', { name: 'Correct purchase', exact: true }).click();
    await page.getByLabel('Correct purchase amount (AUD)').fill('60.00');
    await page.getByLabel('Correct shuttle count').fill('24');
    await page.getByLabel('Reason', { exact: true }).fill('Receipt verified');
    await page.getByRole('button', { name: 'Review correction' }).click();
    await expect(page.getByText('Purchase: $50.00 → $60.00')).toBeVisible();
    await expect(page.getByText('Bank adjustment: -$10.00')).toBeVisible();
    expect(posts).toBe(0);
    await page.getByRole('button', { name: 'Back to edit' }).click();
    await expect(page.getByLabel('Reason', { exact: true })).toHaveValue('Receipt verified');
    await page.getByRole('button', { name: 'Review correction' }).click();
    await page.getByRole('button', { name: 'Save purchase correction', exact: true }).click();
    await expect(page.getByText('Purchase corrected. The original, reversal and replacement are retained.')).toBeVisible();
    await expect(page.getByText('24 shuttles in the bag')).toBeVisible();
    await expect(page.getByText('Reversed / corrected')).toBeVisible();
    await expect(page.getByText(`Correction of ${purchase.id}. Reason: Receipt verified`)).toBeVisible();
    expect(posts).toBe(1);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  });
});
