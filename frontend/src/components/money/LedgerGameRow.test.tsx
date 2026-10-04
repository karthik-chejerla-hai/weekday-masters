import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';
import type { LedgerActivityView, LedgerGameView } from '../../types';
import LedgerList from './LedgerList';

const game: LedgerGameView = {
  session_id: 's1', title: 'Game - 22/09', played_date: '2026-09-22', date_basis: 'title', source: 'splitwise',
  total_charged_cents: 1900, reversed: false, source_count: 2,
  shares: [
    { id: 'p1', user_id: 'u1', member_name: 'Alice', inactive: false, charge_cents: 700, paid_cents: 0, amount_cents: -700, balance_after_cents: 9300 },
    { id: 'p2', user_id: 'u2', member_name: 'Bob', inactive: false, charge_cents: 700, paid_cents: 0, amount_cents: -700, balance_after_cents: -700 },
    { id: 'p3', member_name: 'Former', inactive: true, charge_cents: 500, paid_cents: 0, amount_cents: -500, balance_after_cents: 0 },
  ],
};
const activity = (value = game): LedgerActivityView => ({ id: 'a1', occurred_at: '2026-09-23T00:00:00+10:00', game: value });

describe('Grouped game ledger', () => {
  it('shows one collapsed game, then all shares and balances on expansion', async () => {
    const user = userEvent.setup();
    render(<LedgerList entries={[activity()]} showMember userId="u1" />);
    expect(screen.getAllByText('Game - 22/09')).toHaveLength(1);
    expect(screen.getAllByRole('listitem')).toHaveLength(1);
    expect(screen.getByText('$19.00')).toBeInTheDocument();
    expect(screen.getByText('Total charged')).toBeInTheDocument();
    expect(screen.queryByRole('table')).not.toBeInTheDocument();
    const button = screen.getByRole('button', { name: 'Show shares for Game - 22/09' });
    expect(button).toHaveAttribute('aria-expanded', 'false');
    await user.click(button);
    expect(button).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByText('Played 22 Sep 2026 · Recorded 23 Sep 2026')).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Balance after' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Share' })).toBeInTheDocument();
    expect(within(screen.getByRole('row', { name: /Alice/ })).getByText('$93.00')).toBeInTheDocument();
    expect(within(screen.getByRole('row', { name: /Former/ })).getByText('Inactive')).toBeInTheDocument();
    expect(screen.getByText(/Includes the regular and extra-hour charges/)).toBeInTheDocument();
    await user.click(button);
    expect(screen.queryByRole('table')).not.toBeInTheDocument();
  });

  it('keeps all shares visible in mine mode and supports keyboard expansion', async () => {
    const user = userEvent.setup();
    render(<LedgerList entries={[activity()]} userId="u1" />);
    expect(screen.getByText('Your share $7.00')).toBeInTheDocument();
    await user.tab();
    await user.keyboard('{Enter}');
    expect(screen.getByRole('row', { name: /Bob/ })).toBeInTheDocument();
    expect(screen.getByRole('row', { name: /Former/ })).toBeInTheDocument();
    await user.keyboard(' ');
    expect(screen.queryByRole('table')).not.toBeInTheDocument();
  });

  it('separates a payer’s full payment from their own share and net balance change', async () => {
    const value: LedgerGameView = { ...game, source_count: 1, total_charged_cents: 900, shares: [
      { ...game.shares[0], charge_cents: 300, paid_cents: 900, amount_cents: 600, balance_after_cents: 9900 },
      { ...game.shares[1], charge_cents: 600, amount_cents: -600, balance_after_cents: -1300 },
    ] };
    const user = userEvent.setup();
    render(<LedgerList entries={[activity(value)]} userId="u1" />);
    expect(screen.getByText('Your share $3.00')).toBeInTheDocument();
    await user.click(screen.getByRole('button'));
    expect(screen.getByText('Paid $9.00 for the group. Balance change: +$6.00.')).toBeInTheDocument();
    expect(within(screen.getByRole('row', { name: /Alice/ })).getByText('$3.00')).toBeInTheDocument();
  });

  it('shows native guest charges and zero shares without a Splitwise badge', async () => {
    const value: LedgerGameView = { ...game, source: undefined, reversed: true, source_count: 1, shares: [
      { ...game.shares[0], guest_names: ['Guest One'] }, { ...game.shares[1], charge_cents: 0 },
    ] };
    const user = userEvent.setup(); render(<LedgerList entries={[activity(value)]} />);
    await user.click(screen.getByRole('button'));
    expect(screen.getByText('Includes guest: Guest One')).toBeInTheDocument();
    expect(within(screen.getByRole('row', { name: /Bob/ })).getByText('$0.00')).toBeInTheDocument();
    expect(screen.queryByText('Source: Splitwise')).not.toBeInTheDocument();
    expect(screen.getByText('Reversed')).toBeInTheDocument();
  });
});
