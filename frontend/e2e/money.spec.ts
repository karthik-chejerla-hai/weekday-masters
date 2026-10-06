import { test, expect, json, member, position } from './support/fixtures';

test('member can read balances, filter the ledger and recover failed reads', async ({ page }) => {
  let failLedger = true;
  let failAssets = true;
  const filters: string[] = [];
  await page.route('**/api/accounts/activity?*', route => {
    const query = new URL(route.request().url()).searchParams;
    filters.push(`${query.get('scope')}:${query.get('type')}:${query.get('offset')}`);
    return json(route, failLedger ? { error: 'Unavailable' } : { items: [], total: 0 }, failLedger ? 503 : 200);
  });
  await page.route('**/api/position', route => json(route,
    failAssets ? { error: 'Unavailable' } : position, failAssets ? 503 : 200));
  await page.goto('/money');
  await expect(page.getByRole('heading', { name: 'Money', exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Record top-up' })).toHaveCount(0);
  await page.getByRole('tab', { name: 'Ledger', exact: true }).click();
  await expect(page.getByRole('alert')).toContainText('Could not load transactions');
  failLedger = false;
  await page.getByRole('button', { name: 'Try again' }).click();
  await expect(page.getByText('No transactions match these filters.')).toBeVisible();
  await page.getByText('Show my transactions only', { exact: true }).click();
  await expect(page.getByRole('switch', { name: 'Show my transactions only' })).not.toBeChecked();
  await expect.poll(() => filters[filters.length - 1]).toBe('all:all:0');
  await page.getByText('Top-ups only', { exact: true }).click();
  await expect(page.getByRole('switch', { name: 'Top-ups only' })).toBeChecked();
  await expect.poll(() => filters[filters.length - 1]).toBe('all:topup:0');
  expect(filters[0]).toBe('mine:all:0');
  await page.getByRole('tab', { name: 'Club assets' }).click();
  await expect(page.getByRole('alert')).toContainText('Could not load club assets');
  failAssets = false;
  await page.getByRole('button', { name: 'Try again' }).click();
  const cards = page.getByRole('region', { name: 'Club asset cards' });
  await expect(cards.getByText('$50.00', { exact: true })).toBeVisible();
  await expect(cards.getByText('12 shuttles in the bag')).toBeVisible();
  await expect(page.getByRole('button', { name: /Record purchase|Record top-up/i })).toHaveCount(0);
});

test.describe('Admin top-ups', () => {
  test.use({ identity: 'admin' });
  test('validates input, preserves a failed entry, posts cents and refreshes balances', async ({ page }) => {
    let balance = 5000;
    let fail = true;
    const writes: unknown[] = [];
    await page.route('**/api/accounts', route => json(route, {
      items: [{ user_id: member.id, name: 'Alex', balance_cents: balance }],
    }));
    await page.route('**/api/accounts/me', route => json(route, { balance_cents: balance, state: 'ok' }));
    await page.route('**/api/admin/transactions/topup', route => {
      expect(route.request().method()).toBe('POST');
      const body = route.request().postDataJSON();
      writes.push(body);
      if (fail) return json(route, { error: 'Unavailable' }, 503);
      balance += body.amount_cents;
      return json(route, { id: 'transaction-1', kind: 'topup' });
    });
    await page.goto('/money');
    const submit = page.getByRole('button', { name: 'Record top-up' });
    await expect(submit).toBeDisabled();
    await page.getByLabel('Member', { exact: true }).selectOption(member.id);
    await page.getByLabel('Amount', { exact: true }).fill('0');
    await expect(submit).toBeDisabled();
    await page.getByLabel('Amount', { exact: true }).fill('12.34');
    await page.getByLabel('Note', { exact: true }).fill('Test bank transfer');
    await submit.click();
    await expect(page.getByText('Could not record that top-up. Check the amount and try again.')).toBeVisible();
    await expect(page.getByLabel('Amount', { exact: true })).toHaveValue('12.34');
    fail = false;
    await submit.click();
    await expect(page.getByLabel('Amount', { exact: true })).toHaveValue('');
    await expect(page.getByRole('main')).toContainText('$62.34');
    expect(writes).toEqual(Array(2).fill({ user_id: member.id, amount_cents: 1234, description: 'Test bank transfer' }));
    await page.reload();
    await expect(page.getByRole('main')).toContainText('$62.34');
  });
});
