import { test, expect, json } from './support/fixtures';

const roll = [
  { user_id: 'member-1', name: 'Alex', balance_cents: 5000 },
  { user_id: 'member-2', name: 'Blair', balance_cents: 3200 },
  { user_id: 'member-3', name: 'Casey', balance_cents: 1500 },
  { user_id: 'member-4', name: 'Drew', balance_cents: -400 },
];
const input = {
  description: 'Dinner after Friday badminton', occurred_on: '2026-10-07', total_cents: 8400,
  payer_user_id: 'member-1', split_method: 'equal',
  participants: [{ user_id: 'member-1' }, { user_id: 'member-2' }, { user_id: 'member-3' }],
};
const preview = {
  input, payer_name: 'Alex', fingerprint: 'a'.repeat(64),
  lines: [
    { user_id: 'member-1', name: 'Alex', share_cents: 2800, net_cents: 5600 },
    { user_id: 'member-2', name: 'Blair', share_cents: 2800, net_cents: -2800 },
    { user_id: 'member-3', name: 'Casey', share_cents: 2800, net_cents: -2800 },
  ],
};
const row = (id: string, category: string, description: string, amount: number, balance: number) => ({
  id, occurred_at: '2026-10-07T01:00:00Z',
  entry: { id, occurred_at: '2026-10-07T01:00:00Z', kind: 'shared_expense', description, category,
    member_name: 'Alex', user_id: 'member-1', inactive: false, amount_cents: amount, balance_after_cents: balance, reversed: false },
});

// Set SCREENSHOT_DIR to keep pictures of each step, for example for a pull request.
async function snap(page: import('@playwright/test').Page, project: string, name: string) {
  const dir = process.env.SCREENSHOT_DIR;
  // A full-page capture repeats the sticky mobile header mid-image, so mobile keeps to the viewport.
  if (dir) await page.screenshot({ path: `${dir}/${project}-${name}.png`, fullPage: project === 'chromium' });
}

test('a member has no shared expense action and cannot open the form', async ({ page }) => {
  await page.goto('/money');
  await expect(page.getByText(/Alex/).first()).toBeVisible();
  await expect(page.getByRole('link', { name: 'Record expense' })).toHaveCount(0);
  await page.goto('/admin/expenses/new');
  await expect(page.getByRole('heading', { name: 'Record shared expense' })).toHaveCount(0);
});

test.describe('Admin shared expense', () => {
  test.use({ identity: 'admin' });

  test('records a dinner paid by a member, then reverses it', async ({ page }, testInfo) => {
    const project = testInfo.project.name;
    const writes: unknown[] = [];
    await page.route('**/api/accounts', route => json(route, { items: roll }));
    await page.route('**/api/admin/expenses/preview', route => {
      writes.push(['preview', route.request().postDataJSON()]);
      return json(route, preview);
    });
    await page.route('**/api/admin/expenses', route => {
      writes.push(['confirm', route.request().postDataJSON()]);
      return json(route, { id: 'transaction-9', kind: 'shared_expense' }, 201);
    });
    await page.route('**/api/admin/transactions/transaction-9/reverse', route => {
      writes.push(['reverse', route.request().postDataJSON()]);
      return json(route, { id: 'transaction-10', kind: 'reversal' }, 201);
    });

    await page.goto('/money');
    await snap(page, project, '1-money-record-expense');
    await page.getByRole('link', { name: 'Record expense' }).click();
    await expect(page.getByRole('heading', { name: 'Record shared expense' })).toBeVisible();

    await page.getByLabel('Description').fill('Dinner after Friday badminton');
    await page.getByLabel('Total ($)').fill('84.00');
    await page.getByLabel('Paid by').selectOption('member-1');
    await expect(page.getByRole('checkbox', { name: 'Alex' })).toBeChecked();
    await page.getByRole('checkbox', { name: 'Blair' }).check();
    await page.getByRole('checkbox', { name: 'Casey' }).check();
    await snap(page, project, '2-form');

    await page.getByRole('button', { name: 'Preview expense' }).click();
    const review = page.getByRole('region', { name: 'Expense preview' });
    await expect(review).toContainText('Paid by Alex');
    await expect(review).toContainText('Alex is credited $56.00');
    await expect(review.getByRole('button', { name: 'Confirm expense' })).toBeInViewport();
    await snap(page, project, '3-preview');

    await review.getByRole('button', { name: 'Confirm expense' }).click();
    await expect(page.getByText('Expense recorded')).toBeVisible();
    await snap(page, project, '4-recorded');

    await page.getByRole('button', { name: 'Reverse this expense' }).click();
    await expect(page.getByText('Expense reversed')).toBeVisible();
    expect(writes).toEqual([
      ['preview', input],
      ['confirm', { ...input, expected_preview: preview.fingerprint }],
      ['reverse', { description: 'Reversal of Dinner after Friday badminton' }],
    ]);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  });

  test('offers amounts or shares for each selected member', async ({ page }, testInfo) => {
    await page.route('**/api/accounts', route => json(route, { items: roll }));
    await page.goto('/admin/expenses/new');
    await page.getByLabel('Description').fill('Coffee after training');
    await page.getByLabel('Total ($)').fill('19.50');
    await page.getByRole('checkbox', { name: 'Blair' }).check();
    await page.getByRole('checkbox', { name: 'Drew' }).check();

    await page.getByLabel('Split').selectOption('exact');
    await page.getByLabel('Amount for Blair').fill('12');
    await page.getByLabel('Amount for Drew').fill('7');
    await expect(page.getByText('$0.50 still to assign.')).toBeVisible();
    await snap(page, testInfo.project.name, '5-exact-amounts');
    await page.getByRole('button', { name: 'Preview expense' }).click();
    await expect(page.getByRole('alert')).toContainText('The amounts must add up to the total.');

    await page.getByLabel('Split').selectOption('shares');
    await expect(page.getByLabel('Shares for Blair')).toHaveValue('1');
    await page.getByLabel('Shares for Blair').fill('2');
    await snap(page, testInfo.project.name, '6-shares');
    // The fixture rejects unhandled writes: a refused form must never post.
  });

  test('the ledger marks coffee and meals with their own icons', async ({ page }, testInfo) => {
    await page.route('**/api/accounts/activity?*', route => json(route, { total: 3, items: [
      row('e1', 'coffee', 'Coffee after training', -650, 4350),
      row('e2', 'food', 'Dinner after Friday badminton', 5600, 5000),
      row('e3', 'other', "Friday at Joe's", -1200, -600),
    ] }));
    await page.goto('/money?tab=ledger');
    await expect(page.getByRole('img', { name: 'Coffee' })).toBeVisible();
    await expect(page.getByRole('img', { name: 'Food and drink' })).toBeVisible();
    await expect(page.getByRole('img', { name: 'Other transaction' })).toBeVisible();
    await snap(page, testInfo.project.name, '7-ledger-icons');
  });
});
