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
  await expect(page.getByRole('button', { name: /Top up court credit|Record shuttle purchase|Audit bank balance/ })).toHaveCount(0);
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
    await page.getByRole('button', { name: 'Audit bank balance' }).click();
    await expect(page.getByLabel('Asset', { exact: true })).toHaveValue('bank');
    await expect(page.getByRole('heading', { name: 'Correction audit history' })).toBeVisible();
    await page.getByLabel('Correct balance (AUD)').fill('40');
    await page.getByLabel('Reason', { exact: true }).fill('Checked with venue statement');
    await page.getByRole('button', { name: 'Review correction' }).click();
    await expect(page.getByRole('heading', { name: 'Review bank correction' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Save correction' })).toBeVisible();
    await page.getByRole('button', { name: 'Cancel', exact: true }).click();
    await expect(page.getByRole('button', { name: 'Audit bank balance' })).toBeFocused();
    // The fixture rejects all unhandled writes: review and Cancel must never post.
  });

  test('card actions share one form area and align on desktop', async ({ page }, testInfo) => {
    await page.goto('/money?tab=club');
    const labels = ['Top up court credit', 'Record shuttle purchase', 'Audit bank balance'];
    const buttons = labels.map(name => page.getByRole('button', { name, exact: true }));
    if (testInfo.project.name === 'chromium') {
      const boxes = await Promise.all(buttons.map(button => button.boundingBox()));
      expect(Math.max(...boxes.map(box => box!.y)) - Math.min(...boxes.map(box => box!.y))).toBeLessThan(2);
    }
    for (let i = 0; i < buttons.length; i++) {
      await buttons[i].click();
      await expect(page.getByRole('region', { name: labels[i], exact: true })).toBeVisible();
      await expect(page.getByRole('button', { name: 'Cancel', exact: true })).toHaveCount(1);
    }
    await expect(page.getByLabel('Amount paid to the venue')).toHaveCount(0);
    await expect(page.getByLabel('Shuttles', { exact: true })).toHaveCount(0);
    await buttons[2].click();
    await expect(page.getByRole('region', { name: 'Asset form', exact: true })).toHaveCount(0);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  });

  test('bank audit keeps review and retry, blocks switching during save, then closes', async ({ page }) => {
    let fail = true;
    let finish: (() => void) | undefined;
    const pending = new Promise<void>(resolve => { finish = resolve; });
    const writes: Array<Record<string, unknown>> = [];
    await page.route('**/api/admin/balance-corrections', async route => {
      writes.push(route.request().postDataJSON());
      if (fail) return json(route, { message: 'Please retry' }, 503);
      await pending;
      return json(route, { request_id: writes[0].request_id });
    });
    await page.goto('/money?tab=club');
    const opener = page.getByRole('button', { name: 'Audit bank balance' });
    await opener.click();
    await page.getByLabel('Correct balance (AUD)').fill('40');
    await page.getByLabel('Reason', { exact: true }).fill('Verified bank statement');
    await page.getByRole('button', { name: 'Review correction' }).click();
    await page.getByRole('button', { name: 'Save correction' }).click();
    await expect(page.getByRole('alert')).toContainText('Please retry');
    fail = false;
    await page.getByRole('button', { name: 'Save correction' }).click();
    await expect(page.getByRole('button', { name: 'Top up court credit', exact: true })).toBeDisabled();
    await expect(page.getByRole('button', { name: 'Cancel', exact: true })).toBeDisabled();
    finish!();
    await expect(opener).toBeFocused();
    await expect(page.getByRole('status')).toHaveText('Balance correction recorded.');
    await expect(page.getByLabel('Correct balance (AUD)')).toHaveCount(0);
    expect(writes).toHaveLength(2);
    expect(writes[1]).toEqual(writes[0]);
    expect(writes[0]).toMatchObject({ account_kind: 'bank', expected_cents: 5000, target_cents: 4000, reason: 'Verified bank statement' });
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
