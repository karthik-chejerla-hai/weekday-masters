import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import Money from './Money';
import { useAuth } from '../context/useAuth';
import { api } from '../services/api';
import type { LedgerEntryView, PlayerBalance } from '../types';

vi.mock('../context/useAuth', () => ({ useAuth: vi.fn() }));
vi.mock('../services/api', () => ({
  api: {
    listBalances: vi.fn(),
    getMyBalance: vi.fn(),
    getMyEntries: vi.fn(),
    getClub: vi.fn(),
    recordTopup: vi.fn(),
  },
}));

const balances: PlayerBalance[] = [
  { user_id: 'u1', name: 'Karthik', balance_cents: 4250 },
  { user_id: 'u2', name: 'Priya', balance_cents: 3100 },
  { user_id: 'u3', name: 'Jono', balance_cents: -825 },
];

const entries: LedgerEntryView[] = [
  {
    id: 'e1',
    occurred_at: '2026-08-25T21:15:00+10:00',
    kind: 'session_settlement',
    description: 'Tuesday session',
    amount_cents: -2790,
    balance_after_cents: 4250,
    reversed: false,
  },
  {
    id: 'e2',
    occurred_at: '2026-08-19T09:02:00+10:00',
    kind: 'player_topup',
    description: 'Bank transfer',
    amount_cents: 5000,
    balance_after_cents: 7040,
    reversed: false,
  },
];

function mockAuth({ isAdmin = false } = {}) {
  vi.mocked(useAuth).mockReturnValue({
    user: { id: 'u1', name: 'Karthik' },
    isAdmin,
    isApproved: true,
  } as unknown as ReturnType<typeof useAuth>);
}

function renderPage() {
  return render(
    <MemoryRouter>
      <Money />
    </MemoryRouter>
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.listBalances).mockResolvedValue(balances);
  vi.mocked(api.getMyBalance).mockResolvedValue({ balance_cents: 4250, state: 'ok' });
  vi.mocked(api.getMyEntries).mockResolvedValue({ items: entries, total: 2 });
});

describe('Money', () => {
  it('shows every member’s balance, not just the caller’s', async () => {
    mockAuth();
    renderPage();

    await waitFor(() => expect(screen.getByText('Karthik')).toBeInTheDocument());
    expect(screen.getByText('Priya')).toBeInTheDocument();
    expect(screen.getByText('Jono')).toBeInTheDocument();
    expect(screen.getByText('$31.00')).toBeInTheDocument();
  });

  it('shows a debt as a negative amount', async () => {
    mockAuth();
    renderPage();

    await waitFor(() => expect(screen.getByText('-$8.25')).toBeInTheDocument());
  });

  it('says how many members owe the club', async () => {
    mockAuth();
    renderPage();

    await waitFor(() => expect(screen.getByText('1 member owes the club')).toBeInTheDocument());
  });

  it('shows the caller’s own history with its running balance', async () => {
    mockAuth();
    const user = userEvent.setup();
    renderPage();

    await waitFor(() => expect(screen.getByText('Balances')).toBeInTheDocument());
    await user.click(screen.getByRole('tab', { name: 'My ledger' }));

    expect(screen.getByText('Session')).toBeInTheDocument();
    expect(screen.getByText('Top-up')).toBeInTheDocument();
    expect(screen.getByText('+$50.00')).toBeInTheDocument();
    expect(screen.getByText('-$27.90')).toBeInTheDocument();
  });

  it('hides the top-up form from members who are not admins', async () => {
    mockAuth({ isAdmin: false });
    renderPage();

    await waitFor(() => expect(screen.getByText('Karthik')).toBeInTheDocument());
    expect(screen.queryByText('Record a top-up')).not.toBeInTheDocument();
  });

  it('offers the top-up form to an admin', async () => {
    mockAuth({ isAdmin: true });
    vi.mocked(api.getClub).mockResolvedValue({
      id: 'c1',
      name: 'Rally',
      venue_name: '',
      venue_address: '',
      created_at: '',
      updated_at: '',
      low_balance_threshold_cents: 2000,
    });
    renderPage();

    await waitFor(() => expect(screen.getByText('Record a top-up')).toBeInTheDocument());
  });

  it('reports a load failure instead of showing an empty ledger', async () => {
    mockAuth();
    vi.mocked(api.listBalances).mockRejectedValue(new Error('network'));
    renderPage();

    await waitFor(() =>
      expect(screen.getByText(/Could not load balances/)).toBeInTheDocument()
    );
  });
});
