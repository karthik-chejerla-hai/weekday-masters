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
    getLedgerActivity: vi.fn(),
    getClub: vi.fn(),
    getClubPosition: vi.fn(),
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
    kind: 'session_settlement', category: 'session', member_name: 'Karthik', inactive: false,
    description: 'Tuesday session',
    amount_cents: -2790,
    balance_after_cents: 4250,
    reversed: false,
  },
  {
    id: 'e2',
    occurred_at: '2026-08-19T09:02:00+10:00',
    kind: 'player_topup', category: 'topup', member_name: 'Karthik', inactive: false,
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
  vi.mocked(api.getLedgerActivity).mockResolvedValue({ items: entries.map((entry) => ({ id: entry.id, occurred_at: entry.occurred_at, entry })), total: 2 });
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
    await user.click(screen.getByRole('tab', { name: 'Ledger' }));

    await waitFor(() => expect(screen.getByText('Tuesday session')).toBeInTheDocument());
    expect(screen.getByRole('img', { name: 'Top-up' })).toBeInTheDocument();
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

describe('Money — club assets', () => {
  const position = {
    assets: {
      bank_cents: 18500,
      court_credit_cents: 1700,
      shuttle_stock_cents: 3750,
      shuttle_stock_units: 9,
      total_cents: 23950,
      bank_as_of: "2026-10-04", court_credit_as_of: "2026-10-04",
      shuttle_audited_on: "2026-10-03", shuttle_stock_as_of: "2026-10-04",
    },
    liabilities: { player_balances_cents: 23950 },
    surplus_cents: 0,
    balanced: true,
    warnings: [],
  };

  function mockAdmin() {
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
  }

  it('shares the club assets tab with members who are not admins', async () => {
    mockAuth({ isAdmin: false });
    renderPage();

    await waitFor(() => expect(screen.getByText('Karthik')).toBeInTheDocument());
    expect(screen.getByRole('tab', { name: 'Club assets' })).toBeInTheDocument();
  });

  it('shows assets and audit dates to a member but hides admin forms', async () => {
    mockAuth();
    vi.mocked(api.getClubPosition).mockResolvedValue(position);
    const user = userEvent.setup();
    renderPage();
    await user.click(await screen.findByRole('tab', { name: 'Club assets' }));
    expect(screen.getByText('$185.00')).toBeInTheDocument();
    expect(screen.getByText('Audited on: 3 Oct 2026')).toBeInTheDocument();
    expect(screen.getByText('Stock updated: 4 Oct 2026')).toBeInTheDocument();
    expect(screen.getAllByText('On: 4 Oct 2026')).toHaveLength(2);
    expect(screen.queryByText('Top up court credit')).not.toBeInTheDocument();
    expect(screen.queryByText('Record shuttles bought')).not.toBeInTheDocument();
  });

  it('keeps the asset tab visible when an admin enters member preview', async () => {
    mockAdmin();
    vi.mocked(api.getClubPosition).mockResolvedValue(position);
    const user = userEvent.setup();
    const view = renderPage();
    await user.click(await screen.findByRole('tab', { name: 'Club assets' }));
    expect(screen.getByText('Top up court credit')).toBeInTheDocument();
    expect(screen.getByText('Record shuttles bought')).toBeInTheDocument();
    mockAuth();
    view.rerender(<MemoryRouter><Money /></MemoryRouter>);
    expect(screen.getByRole('tab', { name: 'Club assets' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByText('$185.00')).toBeInTheDocument();
    expect(screen.queryByText('Top up court credit')).not.toBeInTheDocument();
    expect(screen.queryByText('Record shuttles bought')).not.toBeInTheDocument();
  });

  it('shows a clear error and retry when assets cannot load', async () => {
    mockAuth();
    vi.mocked(api.getClubPosition).mockRejectedValueOnce(new Error('network')).mockResolvedValueOnce(position);
    const user = userEvent.setup(); renderPage();
    await user.click(await screen.findByRole('tab', { name: 'Club assets' }));
    expect(screen.getByRole('alert')).toHaveTextContent('Could not load club assets');
    await user.click(screen.getByRole('button', { name: 'Try again' }));
    expect(await screen.findByText('$185.00')).toBeInTheDocument();
  });

  // The point of three asset lines rather than one total: only the first is cash.
  it('shows the three places club money sits', async () => {
    mockAdmin();
    vi.mocked(api.getClubPosition).mockResolvedValue(position);
    const user = userEvent.setup();
    renderPage();

    await waitFor(() => expect(screen.getByRole('tab', { name: 'Club assets' })).toBeInTheDocument());
    await user.click(screen.getByRole('tab', { name: 'Club assets' }));

    expect(screen.getByText('Bank Account balance')).toBeInTheDocument();
    expect(screen.getByText('$185.00')).toBeInTheDocument();
    expect(screen.getByText('Unused Court Credit')).toBeInTheDocument();
    expect(screen.getByText('$17.00')).toBeInTheDocument();
    expect(screen.getByText('Shuttles available')).toBeInTheDocument();
    expect(screen.getByText('9 shuttles in the bag')).toBeInTheDocument();
    expect(screen.getByText(/The books balance/)).toBeInTheDocument();
  });

  it('surfaces a warning that the club cannot pay for the next session', async () => {
    mockAdmin();
    vi.mocked(api.getClubPosition).mockResolvedValue({
      ...position,
      warnings: [
        {
          code: 'court_credit_short',
          message: 'Court credit covers $17.00; the next session needs $60.00.',
          next_session_id: 's1',
        },
      ],
    });
    const user = userEvent.setup();
    renderPage();

    await waitFor(() => expect(screen.getByRole('tab', { name: 'Club assets' })).toBeInTheDocument());
    await user.click(screen.getByRole('tab', { name: 'Club assets' }));

    expect(screen.getByText(/the next session needs \$60\.00/)).toBeInTheDocument();
  });

  it('says plainly when the books do not balance', async () => {
    mockAdmin();
    vi.mocked(api.getClubPosition).mockResolvedValue({ ...position, balanced: false });
    const user = userEvent.setup();
    renderPage();

    await waitFor(() => expect(screen.getByRole('tab', { name: 'Club assets' })).toBeInTheDocument());
    await user.click(screen.getByRole('tab', { name: 'Club assets' }));

    expect(screen.getByText(/The books do not balance/)).toBeInTheDocument();
  });
});
