import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { api } from '../../services/api';
import LedgerList from './LedgerList';
import type { LedgerActivityView } from '../../types';

vi.mock('../../services/api', () => ({ api: { reverseTransaction: vi.fn() } }));
beforeEach(() => { vi.resetAllMocks(); });

function entry(id: string, category: string, description: string, overrides: Record<string, unknown> = {}): LedgerActivityView {
  return {
    id, occurred_at: '2026-11-18T02:00:00Z',
    entry: { id, transaction_id: `txn-${id}`, occurred_at: '2026-11-18T02:00:00Z', kind: 'shared_expense', description, category, member_name: 'Priya', inactive: false, amount_cents: -450, balance_after_cents: 1550, reversed: false, ...overrides },
  } as LedgerActivityView;
}

describe('LedgerList shared expenses', () => {
  it('shows a cup for coffee, cutlery for a meal and a receipt otherwise', () => {
    render(<LedgerList entries={[entry('a', 'coffee', 'Coffee after training'), entry('b', 'food', 'Team dinner'), entry('c', 'other', "Friday at Joe's")]} />);
    expect(screen.getByRole('img', { name: 'Coffee' })).toBeInTheDocument();
    expect(screen.getByRole('img', { name: 'Food and drink' })).toBeInTheDocument();
    expect(screen.getByRole('img', { name: 'Other transaction' })).toBeInTheDocument();
  });
});

describe('LedgerList reversal', () => {
  const rows = [
    entry('a', 'coffee', 'Coffee after training'),
    entry('b', 'food', 'Team dinner', { reversed: true }),
    entry('c', 'topup', 'Bank transfer', { kind: 'player_topup' }),
    entry('d', 'reversal', 'Reversal of Team dinner', { kind: 'reversal' }),
  ];

  it('offers no reverse action to a member', () => {
    render(<LedgerList entries={rows} />);
    expect(screen.queryByRole('button', { name: /Reverse/ })).not.toBeInTheDocument();
  });

  it('offers it to an admin only on a shared expense that is still live', () => {
    render(<LedgerList entries={rows} onReversed={() => {}} />);
    expect(screen.getAllByRole('button', { name: /Reverse/ })).toHaveLength(1);
    expect(screen.getByRole('button', { name: 'Reverse Coffee after training' })).toBeInTheDocument();
  });

  it('asks first, and cancelling reverses nothing', async () => {
    const user = userEvent.setup();
    const onReversed = vi.fn();
    render(<LedgerList entries={rows} onReversed={onReversed} />);
    await user.click(screen.getByRole('button', { name: 'Reverse Coffee after training' }));
    expect(screen.getByRole('group', { name: 'Reverse Coffee after training?' })).toHaveTextContent('every member who shared it');
    await user.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(api.reverseTransaction).not.toHaveBeenCalled();
    expect(onReversed).not.toHaveBeenCalled();
    expect(screen.getByRole('button', { name: 'Reverse Coffee after training' })).toBeInTheDocument();
  });

  it('reverses the whole transaction once and reports it', async () => {
    const user = userEvent.setup();
    const onReversed = vi.fn();
    let finish: (value: unknown) => void = () => {};
    vi.mocked(api.reverseTransaction).mockReturnValue(new Promise((resolve) => { finish = resolve; }) as never);
    render(<LedgerList entries={rows} onReversed={onReversed} />);
    await user.click(screen.getByRole('button', { name: 'Reverse Coffee after training' }));
    await user.click(screen.getByRole('button', { name: 'Reverse expense' }));
    expect(screen.getByRole('button', { name: 'Reversing…' })).toBeDisabled();
    finish({ id: 'r1' });
    await vi.waitFor(() => expect(onReversed).toHaveBeenCalledTimes(1));
    expect(api.reverseTransaction).toHaveBeenCalledTimes(1);
    expect(api.reverseTransaction).toHaveBeenCalledWith('txn-a', 'Reversal of Coffee after training');
  });

  it('keeps the question open and shows the reason when the server refuses', async () => {
    const user = userEvent.setup();
    const onReversed = vi.fn();
    vi.mocked(api.reverseTransaction).mockRejectedValue({ response: { data: { code: 'transaction_already_reversed', message: 'This transaction has already been reversed.' } } });
    render(<LedgerList entries={rows} onReversed={onReversed} />);
    await user.click(screen.getByRole('button', { name: 'Reverse Coffee after training' }));
    await user.click(screen.getByRole('button', { name: 'Reverse expense' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('already been reversed');
    expect(onReversed).not.toHaveBeenCalled();
  });
});
