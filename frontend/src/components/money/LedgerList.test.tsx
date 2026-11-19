import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import LedgerList from './LedgerList';
import type { LedgerActivityView } from '../../types';

function entry(id: string, category: string, description: string): LedgerActivityView {
  return {
    id, occurred_at: '2026-11-18T02:00:00Z',
    entry: { id, occurred_at: '2026-11-18T02:00:00Z', kind: 'shared_expense', description, category, member_name: 'Priya', inactive: false, amount_cents: -450, balance_after_cents: 1550, reversed: false },
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
