import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import ImportedBreakdown from './ImportedBreakdown';
import PositionPanel from '../money/PositionPanel';
import type { ClubPosition } from '../../types';

describe('Imported history', () => {
  it('shows original shares, inactive participants and a payer credit without invented rates', () => {
    render(<ImportedBreakdown session={{
      played_date: '2024-04-02', date_basis: 'title', total_cents: 1500,
      lines: [
        { name: 'Alice', user_id: 'a', inactive: false, charge_cents: 500, paid_cents: 1500, net_cents: 1000 },
        { name: 'Former', inactive: true, charge_cents: 1000, paid_cents: 0, net_cents: -1000 },
      ],
      sources: [{ description: 'Game - 2 Apr', recorded_date: '2024-04-03', cost_cents: 1500 }],
    }} />);
    expect(screen.getByText('Inactive')).toBeInTheDocument();
    expect(screen.getByText('$5.00')).toBeInTheDocument();
    expect(screen.getByText('$10.00')).toBeInTheDocument();
    expect(screen.getByText(/Paid \$15.00 for the group. Balance change: \+\$10.00/)).toBeInTheDocument();
    expect(screen.getByText(/Played Tuesday 2 April 2024/)).toBeInTheDocument();
    expect(screen.queryByText(/Costed at/)).not.toBeInTheDocument();
  });

  it('does not present unverified asset zeros as the actual club position', () => {
    render(<PositionPanel position={{ assets_pending: true } as ClubPosition} />);
    expect(screen.getByText('Club assets need review')).toBeInTheDocument();
    expect(screen.queryByText(/The books balance/)).not.toBeInTheDocument();
  });
});
