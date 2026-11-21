import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import AssetPurchaseForms from './AssetPurchaseForms';
import { api } from '../../services/api';
import type { ClubPosition } from '../../types';

vi.mock('../../services/api', () => ({
  api: {
    recordCourtCredit: vi.fn(),
    recordShuttlePurchase: vi.fn(),
    listBalanceCorrections: vi.fn(),
    correctAssetBalance: vi.fn(),
  },
}));

const position: ClubPosition = {
  assets_pending: false,
  assets: {
    bank_cents: 40000, court_credit_cents: 15000, shuttle_stock_cents: 8740, shuttle_stock_units: 21,
    total_cents: 63740, bank_as_of: null, court_credit_as_of: null, shuttle_stock_as_of: null, shuttle_audited_on: null,
  },
  liabilities: { player_balances_cents: 63740 },
  surplus_cents: 0,
  balanced: true,
  warnings: [],
};

const uuid = expect.stringMatching(/^[0-9a-f-]{36}$/);

function renderForms(onRecorded = vi.fn()) {
  render(<AssetPurchaseForms position={position} revision={0} onRecorded={onRecorded} />);
  return { onRecorded };
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.recordCourtCredit).mockResolvedValue({} as never);
  vi.mocked(api.recordShuttlePurchase).mockResolvedValue({} as never);
});

// Both forms follow the same rule: one request ID per submission, resent on a
// retry so a reply lost after the write cannot record the purchase twice, and a
// new one for the next purchase.
describe.each([
  {
    name: 'court credit', open: 'Record court credit purchase', failure: /Could not record that top-up/,
    method: () => vi.mocked(api.recordCourtCredit), args: [10000, 'Venue account top-up', undefined, uuid],
    amountLabel: 'Amount paid to the venue',
  },
  {
    name: 'shuttle', open: 'Record shuttle purchase', failure: /Could not record that purchase/,
    method: () => vi.mocked(api.recordShuttlePurchase), args: [12, 5000, 'Shuttle purchase', undefined, uuid],
    amountLabel: 'What they cost',
  },
])('$name purchase form', ({ open, failure, method, args, amountLabel }) => {
  const sentIds = () => method().mock.calls.map((call) => call[call.length - 1]);

  it('sends a request ID with the purchase', async () => {
    const user = userEvent.setup();
    const { onRecorded } = renderForms();

    await user.click(screen.getByRole('button', { name: open }));
    await user.click(screen.getByRole('button', { name: 'Record purchase' }));

    await waitFor(() => expect(onRecorded).toHaveBeenCalled());
    expect(method()).toHaveBeenCalledWith(...args);
  });

  it('reuses the request ID when a failed submission is retried', async () => {
    method().mockRejectedValueOnce(new Error('network'));
    const user = userEvent.setup();
    const { onRecorded } = renderForms();

    await user.click(screen.getByRole('button', { name: open }));
    await user.click(screen.getByRole('button', { name: 'Record purchase' }));
    await screen.findByText(failure);
    await user.click(screen.getByRole('button', { name: 'Record purchase' }));

    await waitFor(() => expect(onRecorded).toHaveBeenCalled());
    const [first, retry] = sentIds();
    expect(first).toEqual(uuid);
    expect(retry).toBe(first);
  });

  it('mints a new request ID when the amount is edited after a failure', async () => {
    method().mockRejectedValueOnce(new Error('network'));
    const user = userEvent.setup();
    renderForms();

    await user.click(screen.getByRole('button', { name: open }));
    await user.click(screen.getByRole('button', { name: 'Record purchase' }));
    await screen.findByText(failure);
    await user.clear(screen.getByLabelText(amountLabel));
    await user.type(screen.getByLabelText(amountLabel), '75.00');
    await user.click(screen.getByRole('button', { name: 'Record purchase' }));

    await waitFor(() => expect(method()).toHaveBeenCalledTimes(2));
    const [first, second] = sentIds();
    expect(second).not.toBe(first);
  });

  it('mints a new request ID for the next purchase', async () => {
    const user = userEvent.setup();
    const { onRecorded } = renderForms();

    for (let i = 1; i <= 2; i++) {
      await user.click(screen.getByRole('button', { name: open }));
      await user.click(screen.getByRole('button', { name: 'Record purchase' }));
      await waitFor(() => expect(onRecorded).toHaveBeenCalledTimes(i));
    }

    const [first, second] = sentIds();
    expect(second).toEqual(uuid);
    expect(second).not.toBe(first);
  });
});
