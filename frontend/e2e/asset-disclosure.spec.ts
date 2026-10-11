import { test, expect, json, position } from './support/fixtures';

const datedPosition = {
  ...position,
  assets: { ...position.assets, bank_as_of: '2026-10-04', court_credit_as_of: '2026-10-04',
    shuttle_audited_on: '2026-10-03', shuttle_stock_as_of: '2026-10-04' },
};

test('asset details open by keyboard while values and warnings stay visible', async ({ page }) => {
  await page.route('**/api/position', route => json(route, { ...datedPosition, balanced: false,
    warnings: [{ code: 'court_credit_short', message: 'Court credit is too low for the next session.' }] }));
  await page.goto('/money?tab=club');
  await expect(page.getByText('12 shuttles in the bag')).toBeVisible();
  await expect(page.getByText('Court credit is too low for the next session.')).toBeVisible();
  await expect(page.getByRole('alert')).toContainText('The books do not balance');
  const details = page.getByText('Reconciliation and audit details', { exact: true });
  await expect(page.getByText('Audited on: 3 Oct 2026')).not.toBeVisible();
  for (let i = 0; i < 30 && !(await details.evaluate(el => el === document.activeElement)); i++) await page.keyboard.press('Tab');
  await expect(details).toBeFocused();
  await page.keyboard.press('Enter');
  await expect(page.getByText('Audited on: 3 Oct 2026')).toBeVisible();
  await expect(page.getByText('Stock updated: 4 Oct 2026')).toBeVisible();
  await expect(page.getByText('On: 4 Oct 2026')).toHaveCount(2);
  await page.keyboard.press('Enter');
  await expect(page.getByText('Audited on: 3 Oct 2026')).not.toBeVisible();
  await expect(page.getByRole('alert')).toBeVisible();
  await expect(page.getByRole('button', { name: /Top up court credit|Record shuttle purchase/ })).toHaveCount(0);
  await expect(page.getByText('Balance corrections and history')).toHaveCount(0);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
});

test.describe('Admin asset actions', () => {
  test.use({ identity: 'admin' });
  test.beforeEach(async ({ page }) => {
    await page.route('**/api/admin/balance-corrections?*', route => json(route, { items: [], total: 0 }));
  });

  test('purchase forms cancel without writing and correction review remains available', async ({ page }) => {
    await page.goto('/money?tab=club');
    for (const [action, field] of [['Top up court credit', 'Amount paid to the venue'], ['Record shuttle purchase', 'Shuttles']]) {
      const opener = page.getByRole('button', { name: action, expanded: false });
      await expect(page.getByLabel(field, { exact: true })).toHaveCount(0);
      await opener.click();
      await expect(page.getByLabel(field, { exact: true })).toBeFocused();
      await page.getByRole('button', { name: 'Cancel', exact: true }).click();
      await expect(opener).toBeFocused();
    }
    await expect(page.getByLabel('Correct balance (AUD)')).not.toBeVisible();
    await page.getByText('Balance corrections and history', { exact: true }).click();
    await expect(page.getByRole('heading', { name: 'Correction audit history' })).toBeVisible();
    await page.getByLabel('Correct balance (AUD)').fill('40');
    await page.getByLabel('Reason', { exact: true }).fill('Checked with venue statement');
    await page.getByRole('button', { name: 'Review correction' }).click();
    await expect(page.getByRole('heading', { name: 'Review court credit correction' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Save correction' })).toBeVisible();
    // The fixture rejects all unhandled writes: review and Cancel must never post.
  });

  for (const kind of ['court-credit', 'shuttle-purchase']) {
    test(`${kind} preserves a failed entry and refreshes assets after retry`, async ({ page }) => {
      let fail = true;
      let reads = 0;
      const writes: unknown[] = [];
      await page.route('**/api/position', route => { reads++; return json(route, datedPosition); });
      await page.route(`**/api/admin/transactions/${kind}`, route => {
        writes.push(route.request().postDataJSON());
        return json(route, fail ? { error: 'Unavailable' } : { id: 'test-transaction' }, fail ? 503 : 200);
      });
      await page.goto('/money?tab=club');
      const action = kind === 'court-credit' ? 'Top up court credit' : 'Record shuttle purchase';
      const opener = page.getByRole('button', { name: action, expanded: false });
      await opener.click();
      const field = page.getByLabel(kind === 'court-credit' ? 'Amount paid to the venue' : 'What they cost');
      await field.fill('25.50');
      const submit = page.getByRole('region', { name: action }).getByRole('button', { name: kind === 'court-credit' ? 'Record top-up' : 'Record purchase' });
      await submit.click();
      await expect(page.getByRole('alert')).toContainText('Could not record');
      await expect(field).toHaveValue('25.50');
      const before = reads;
      fail = false;
      await submit.click();
      await expect(opener).toBeFocused();
      await expect(page.getByRole('status')).toContainText('recorded.');
      await expect.poll(() => reads).toBeGreaterThan(before);
      expect(writes).toEqual(Array(2).fill(kind === 'court-credit'
        ? { amount_cents: 2550, description: 'Venue account top-up' }
        : { amount_cents: 2550, units: 12, description: 'Shuttle purchase' }));
    });
  }
});
