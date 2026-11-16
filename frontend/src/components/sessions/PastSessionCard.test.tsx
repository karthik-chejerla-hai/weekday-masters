import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it } from 'vitest';
import PastSessionCard from './PastSessionCard';
import type { PastSession } from '../../types';

const session: PastSession = {
  session_id: 'cancelled-session', title: 'Tuesday badminton',
  ends_at: '2026-11-10T11:00:00Z', settled: false, total_cents: 0, player_count: 0,
};
function show(overrides: Partial<PastSession> = {}, isAdmin = true) {
  render(<MemoryRouter><PastSessionCard session={{ ...session, ...overrides }} isAdmin={isAdmin} /></MemoryRouter>);
}

describe('Session history state', () => {
  it.each([true, false])('shows stored cancellation without settlement or game actions (admin=%s)', (isAdmin) => {
    show({ status: 'cancelled' }, isAdmin);
    expect(screen.getByText('Cancelled')).toBeInTheDocument();
    expect(screen.getByText('No settlement required')).toBeInTheDocument();
    expect(screen.queryByText('Not settled')).not.toBeInTheDocument();
    expect(screen.queryByText('Nobody has costed this yet')).not.toBeInTheDocument();
    expect(screen.queryByRole('link')).not.toBeInTheDocument();
  });
  it.each(['open', 'closed', undefined] as const)('keeps unsettled %s sessions actionable', (status) => {
    show({ status });
    expect(screen.getByText('Not settled')).toBeInTheDocument();
    expect(screen.getByText('Nobody has costed this yet')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Settle it' })).toHaveAttribute('href', '/admin/sessions/cancelled-session/settle');
  });
  it('keeps settlement cost and split details', () => {
    show({ status: 'closed', settled: true, total_cents: 12000, player_count: 6 });
    expect(screen.getByText('$120.00')).toBeInTheDocument();
    expect(screen.getByText('6 players')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'See the split' })).toBeInTheDocument();
  });
  it('retains imported settlement details without inventing a native status', () => {
    show({ imported_date: '2024-04-02', settled: true, total_cents: 1900, player_count: 3 });
    expect(screen.getByText('3 players · Settled in Splitwise')).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Game scores' })).not.toBeInTheDocument();
    expect(screen.queryByText('Cancelled')).not.toBeInTheDocument();
  });
});
